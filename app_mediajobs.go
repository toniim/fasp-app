package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/heytonyne/fasp/internal/service/media"
)

// Job states of a MediaJobEvent.
const (
	jobRunning   = "running"
	jobDone      = "done"
	jobError     = "error"
	jobCancelled = "cancelled"
)

// MediaJobEvent is the payload of the "media:job" event. Total is 0 while the
// job cannot tell its progress (a trim); Path is the finished file.
type MediaJobEvent struct {
	ID        string `json:"id"`
	Kind      string `json:"kind"` // trim | gif
	State     string `json:"state"`
	Done      int    `json:"done"`
	Total     int    `json:"total"`
	Path      string `json:"path"`
	SizeBytes int64  `json:"size_bytes"`
	Error     string `json:"error"`
}

// mediaJob is one trim or GIF export running off the UI thread.
type mediaJob struct {
	id     string
	cancel context.CancelFunc
	// dir is a temporary folder holding the job's output; it is removed when
	// the job is released.
	dir string
}

// mediaJobs runs the long media jobs and reports their progress as events.
type mediaJobs struct {
	send func(MediaJobEvent)

	mu   sync.Mutex
	jobs map[string]*mediaJob
	// last is the latest event of each job, so a window that opens after a
	// short job has already finished can still learn how it ended.
	last map[string]MediaJobEvent
}

func newMediaJobs(send func(MediaJobEvent)) *mediaJobs {
	return &mediaJobs{send: send, jobs: map[string]*mediaJob{}, last: map[string]MediaJobEvent{}}
}

// emit records the event and sends it to the frontend.
func (m *mediaJobs) emit(e MediaJobEvent) {
	m.mu.Lock()
	m.last[e.ID] = e
	m.mu.Unlock()
	m.send(e)
}

// status returns the latest event of a job. Once the job has ended the record
// is dropped after this read.
func (m *mediaJobs) status(id string) (MediaJobEvent, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	e, ok := m.last[id]
	if ok && e.State != jobRunning {
		delete(m.last, id)
	}
	return e, ok
}

// jobFunc does the work of a job. It reports progress through the callback and
// returns the finished file. It must stop soon after ctx is cancelled.
type jobFunc func(ctx context.Context, dir string, progress func(done, total int)) (string, error)

// start runs fn in the background and returns the job id. The job's events
// follow: running (with progress), then done, error or cancelled.
func (m *mediaJobs) start(kind string, fn jobFunc) (string, error) {
	id, err := newRandomID()
	if err != nil {
		return "", err
	}
	dir, err := os.MkdirTemp("", "fasp-"+kind+"-")
	if err != nil {
		return "", err
	}
	ctx, cancel := context.WithCancel(context.Background())
	job := &mediaJob{id: id, cancel: cancel, dir: dir}
	m.mu.Lock()
	m.jobs[id] = job
	m.mu.Unlock()

	go func() {
		m.emit(MediaJobEvent{ID: id, Kind: kind, State: jobRunning})
		lastPercent := -1
		progress := func(done, total int) {
			// One event per percent is plenty for a progress bar.
			if total > 0 && done*100/total == lastPercent {
				return
			}
			if total > 0 {
				lastPercent = done * 100 / total
			}
			m.emit(MediaJobEvent{ID: id, Kind: kind, State: jobRunning, Done: done, Total: total})
		}
		path, err := fn(ctx, dir, progress)
		switch {
		case ctx.Err() != nil:
			m.release(id)
			m.emit(MediaJobEvent{ID: id, Kind: kind, State: jobCancelled})
		case err != nil:
			m.release(id)
			m.emit(MediaJobEvent{ID: id, Kind: kind, State: jobError, Error: err.Error()})
		default:
			var size int64
			if st, statErr := os.Stat(path); statErr == nil {
				size = st.Size()
			}
			if kind == "trim" {
				// The trimmed file is already at its destination.
				m.release(id)
			}
			m.emit(MediaJobEvent{ID: id, Kind: kind, State: jobDone, Path: path, SizeBytes: size})
		}
	}()
	return id, nil
}

// cancelJob asks a running job to stop. It is a no-op for a finished job.
func (m *mediaJobs) cancelJob(id string) {
	m.mu.Lock()
	job := m.jobs[id]
	m.mu.Unlock()
	if job != nil {
		job.cancel()
	}
}

// release forgets a job and deletes its temporary output.
func (m *mediaJobs) release(id string) {
	m.mu.Lock()
	job := m.jobs[id]
	delete(m.jobs, id)
	m.mu.Unlock()
	if job == nil {
		return
	}
	job.cancel()
	if err := os.RemoveAll(job.dir); err != nil {
		println("[error] remove job folder:", err.Error())
	}
}

// dir returns the temporary folder of a job that is still held.
func (m *mediaJobs) dir(id string) (string, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	job, ok := m.jobs[id]
	if !ok {
		return "", false
	}
	return job.dir, true
}

// releaseAll stops every job and deletes their output (the app is quitting).
func (m *mediaJobs) releaseAll() {
	m.mu.Lock()
	ids := make([]string, 0, len(m.jobs))
	for id := range m.jobs {
		ids = append(ids, id)
	}
	m.mu.Unlock()
	for _, id := range ids {
		m.release(id)
	}
}

// rangeOf converts the trim handles (milliseconds) to the media package's
// durations and rejects values that cannot be a range.
func rangeOf(startMs, endMs int64) (start, end time.Duration, err error) {
	if startMs < 0 || endMs < 0 {
		return 0, 0, errors.New("the trim range cannot be negative")
	}
	start, end = time.Duration(startMs)*time.Millisecond, time.Duration(endMs)*time.Millisecond
	if end != 0 && end <= start {
		return 0, 0, errors.New("the end of the range must be after its start")
	}
	return start, end, nil
}

// gifOptions validates the GIF choices from the export dialog.
func gifOptions(startMs, endMs int64, fps, width int) (media.GIFOptions, error) {
	start, end, err := rangeOf(startMs, endMs)
	if err != nil {
		return media.GIFOptions{}, err
	}
	if fps < 1 || fps > 30 {
		return media.GIFOptions{}, fmt.Errorf("frames per second must be between 1 and 30, got %d", fps)
	}
	if width < 0 || (width != 0 && width < 16) || width > 4096 {
		return media.GIFOptions{}, fmt.Errorf("invalid GIF width %d", width)
	}
	return media.GIFOptions{Start: start, End: end, FPS: fps, Width: width}, nil
}

// EstimateGIF predicts the size in bytes of the GIF StartGIFExport would write
// for these choices.
func (a *App) EstimateGIF(src string, startMs, endMs int64, fps, width int) (int64, error) {
	opt, err := gifOptions(startMs, endMs, fps, width)
	if err != nil {
		return 0, err
	}
	return media.EstimateGIFSize(src, opt)
}

// StartGIFExport starts exporting a GIF of the chosen range in the background
// and returns the job id. Progress and the result arrive as "media:job" events;
// the finished GIF waits in a temporary file until SaveGIFAs, QuickSaveGIF or
// ReleaseMediaJob.
func (a *App) StartGIFExport(src string, startMs, endMs int64, fps, width int) (string, error) {
	opt, err := gifOptions(startMs, endMs, fps, width)
	if err != nil {
		return "", err
	}
	if _, err := os.Stat(src); err != nil {
		return "", err
	}
	name := strings.TrimSuffix(filepath.Base(src), filepath.Ext(src)) + ".gif"
	return a.jobs.start("gif", func(ctx context.Context, dir string, progress func(done, total int)) (string, error) {
		out := filepath.Join(dir, name)
		if err := media.ExportGIFContext(ctx, src, out, opt, progress); err != nil {
			return "", err
		}
		return out, nil
	})
}

// SaveTrimmed asks where to save the trimmed video, then trims in the
// background. It returns the job id, or "" when the dialog was cancelled. A
// trim cannot be interrupted once it started, so cancelling lets it finish and
// then drops the result.
func (a *App) SaveTrimmed(src string, startMs, endMs int64) (string, error) {
	start, end, err := rangeOf(startMs, endMs)
	if err != nil {
		return "", err
	}
	if _, err := os.Stat(src); err != nil {
		return "", err
	}
	name := strings.TrimSuffix(filepath.Base(src), filepath.Ext(src)) + " (trimmed).mp4"
	dst, err := a.pickSavePath("Save Trimmed Video", name, "MP4 Video (*.mp4)", "*.mp4")
	if err != nil || dst == "" {
		return "", err
	}
	if !isVideo(dst) {
		dst += ".mp4"
	}
	if same, _ := sameFile(src, dst); same {
		return "", errors.New("choose a different file name: the trimmed video cannot replace the original")
	}
	return a.jobs.start("trim", a.trimJob(src, dst, start, end))
}

// trimJob trims src into a temporary file first and moves it to dst only when
// the job was not cancelled, so a cancelled trim leaves nothing at dst.
func (a *App) trimJob(src, dst string, start, end time.Duration) jobFunc {
	return func(ctx context.Context, dir string, progress func(done, total int)) (string, error) {
		progress(0, 0)
		out := filepath.Join(dir, "trimmed.mp4")
		if err := media.Trim(src, out, start, end); err != nil {
			return "", err
		}
		if err := ctx.Err(); err != nil {
			return "", err
		}
		if err := copyFile(out, dst); err != nil {
			return "", fmt.Errorf("failed to save %s: %w", dst, err)
		}
		a.recordRecent(dst)
		return dst, nil
	}
}

// GetMediaJob returns the latest state of a job, or nil when it is unknown. The
// player asks once when it opens a job's dialog, in case the job ended before
// the dialog was listening for events.
func (a *App) GetMediaJob(id string) *MediaJobEvent {
	if e, ok := a.jobs.status(id); ok {
		return &e
	}
	return nil
}

// CancelMediaJob stops a running trim or GIF export.
func (a *App) CancelMediaJob(id string) {
	a.jobs.cancelJob(id)
}

// ReleaseMediaJob deletes the temporary output of a finished job once the
// player no longer needs it.
func (a *App) ReleaseMediaJob(id string) {
	a.jobs.release(id)
}

// jobResult returns the finished file of a GIF job.
func (a *App) jobResult(id, path string) error {
	dir, ok := a.jobs.dir(id)
	if !ok {
		return errors.New("this export is no longer available")
	}
	// The path comes from the frontend: it must be the job's own file.
	if filepath.Dir(filepath.Clean(path)) != filepath.Clean(dir) {
		return errors.New("this file does not belong to the export")
	}
	return nil
}

// SaveGIFAs asks where to save a finished GIF and copies it there. It returns
// the saved path, or "" when the dialog was cancelled.
func (a *App) SaveGIFAs(id, path string) (string, error) {
	if err := a.jobResult(id, path); err != nil {
		return "", err
	}
	dst, err := a.pickSavePath("Save GIF", filepath.Base(path), "GIF Image (*.gif)", "*.gif")
	if err != nil || dst == "" {
		return "", err
	}
	if !strings.EqualFold(filepath.Ext(dst), ".gif") {
		dst += ".gif"
	}
	if err := copyFile(path, dst); err != nil {
		return "", fmt.Errorf("failed to save %s: %w", dst, err)
	}
	a.recordRecent(dst)
	return dst, nil
}

// QuickSaveGIF saves a finished GIF into the default folder, never replacing a
// file that is already there, and returns the path.
func (a *App) QuickSaveGIF(id, path string) (string, error) {
	if err := a.jobResult(id, path); err != nil {
		return "", err
	}
	dir, err := a.quickSaveFolder()
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("failed to create %s: %w", dir, err)
	}
	dst := uniquePath(dir, filepath.Base(path))
	if err := copyFile(path, dst); err != nil {
		return "", fmt.Errorf("failed to save %s: %w", dst, err)
	}
	a.recordRecent(dst)
	return dst, nil
}

// quickSaveFolder is the configured default folder, else the platform default.
func (a *App) quickSaveFolder() (string, error) {
	s, err := a.settingsService.GetAll()
	if err != nil {
		return "", fmt.Errorf("failed to read settings: %w", err)
	}
	if s.DefaultSavePath != "" {
		return s.DefaultSavePath, nil
	}
	dir, err := a.fileService.GetDefaultSavePath()
	if err != nil {
		return "", fmt.Errorf("failed to resolve default save folder: %w", err)
	}
	return dir, nil
}

// uniquePath returns dir/name, or dir/name_1, name_2... when it already exists.
func uniquePath(dir, name string) string {
	ext := filepath.Ext(name)
	base := strings.TrimSuffix(name, ext)
	path := filepath.Join(dir, name)
	for i := 1; ; i++ {
		if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
			return path
		}
		path = filepath.Join(dir, fmt.Sprintf("%s_%d%s", base, i, ext))
	}
}

// pickSavePath shows a save dialog and returns the chosen path, or "" when it
// was cancelled.
func (a *App) pickSavePath(message, filename, filterName, pattern string) (string, error) {
	dialog := a.app.Dialog.SaveFile().
		SetMessage(message).
		SetFilename(filename).
		AddFilter(filterName, pattern)
	if a.window.IsVisible() {
		dialog.AttachToWindow(a.window)
	}
	return dialog.PromptForSingleSelection()
}
