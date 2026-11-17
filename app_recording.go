package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/heytonyne/fasp/internal/model"
	"github.com/heytonyne/fasp/internal/service/recorder"
	"github.com/heytonyne/fasp/internal/service/shell"
)

// RecordingRequest is what the "Record screen" dialog asks for.
type RecordingRequest struct {
	Mode        string            `json:"mode"`   // display | window | region
	Window      uint64            `json:"window"` // handle from ListWindows
	Region      *model.CropRegion `json:"region"` // from SelectRecordingRegion
	SystemAudio bool              `json:"system_audio"`
	Microphone  bool              `json:"microphone"`
	MicDevice   string            `json:"mic_device"`
	FPS         int               `json:"fps"`
}

// RecordingStatus is sent with the "recording:started" event.
type RecordingStatus struct {
	Mode      string `json:"mode"`
	StartedAt int64  `json:"started_at"` // Unix milliseconds
}

// RecordingResult describes a finished recording ("recording:finished").
type RecordingResult struct {
	Path      string   `json:"path"`
	URL       string   `json:"url"` // for the in-app player
	Seconds   float64  `json:"seconds"`
	Width     int      `json:"width"`
	Height    int      `json:"height"`
	SizeBytes int64    `json:"size_bytes"`
	EndReason string   `json:"end_reason"`
	Warnings  []string `json:"warnings"`
	Error     string   `json:"error"`
}

// recordingState is the one recording that can run at a time.
type recordingState struct {
	mu   sync.Mutex
	rec  recorder.Recording
	path string
	// finishing guards finishRecording against Stop racing the auto-stop
	// watcher for the same recording.
	finishing sync.Once
}

// ListWindows returns the windows that can be recorded, front-most first,
// without Fasp's own windows.
func (a *App) ListWindows() ([]recorder.Window, error) {
	return recorder.ListWindows()
}

// ListMicrophones returns the microphones that can be recorded.
func (a *App) ListMicrophones() ([]recorder.AudioDevice, error) {
	return recorder.ListMicrophones()
}

// IsRecording reports whether a recording is running.
func (a *App) IsRecording() bool {
	a.recording.mu.Lock()
	defer a.recording.mu.Unlock()
	return a.recording.rec != nil
}

// StartRecording starts recording to the quick-save folder. While it runs the
// editor window turns into a small always-on-top bar with a Stop button that
// is hidden from the recording itself.
func (a *App) StartRecording(req RecordingRequest) error {
	if err := a.requireScreenRecording(); err != nil {
		return err
	}
	st := &a.recording
	st.mu.Lock()
	defer st.mu.Unlock()
	if st.rec != nil {
		return errors.New("a recording is already running")
	}

	opts := recorder.Options{
		Mode:        recorder.Mode(req.Mode),
		Window:      req.Window,
		FPS:         req.FPS,
		Cursor:      true,
		SystemAudio: req.SystemAudio,
		Microphone:  req.Microphone,
		MicDevice:   req.MicDevice,
	}
	if req.Region != nil {
		opts.Region = recorder.Rect{X: req.Region.X, Y: req.Region.Y, Width: req.Region.Width, Height: req.Region.Height}
	}
	path, err := a.recordingPath()
	if err != nil {
		return err
	}
	opts.Output = path

	// Hide our own window from capture before the first frame is taken.
	setExcludedFromCapture(a.window, true)
	rec, err := recorder.Start(opts)
	if err != nil {
		setExcludedFromCapture(a.window, false)
		return err
	}
	st.rec, st.path, st.finishing = rec, path, sync.Once{}
	a.enterRecordingBar()
	a.emit("recording:started", RecordingStatus{Mode: req.Mode, StartedAt: time.Now().UnixMilli()})

	go func() {
		<-rec.Done() // stopped by the user, or the recorded window closed
		a.finishRecording(rec)
	}()
	return nil
}

// StopRecording stops the running recording. The result arrives with the
// "recording:finished" event.
func (a *App) StopRecording() {
	a.recording.mu.Lock()
	rec := a.recording.rec
	a.recording.mu.Unlock()
	if rec != nil {
		a.finishRecording(rec)
	}
}

func (a *App) finishRecording(rec recorder.Recording) {
	st := &a.recording
	st.mu.Lock()
	if st.rec != rec {
		st.mu.Unlock()
		return
	}
	once, path := &st.finishing, st.path
	st.mu.Unlock()

	once.Do(func() {
		stats, err := rec.Stop()
		st.mu.Lock()
		st.rec = nil
		st.mu.Unlock()

		res := RecordingResult{
			Path: path, Seconds: stats.Duration.Seconds(), Width: stats.Width, Height: stats.Height,
			EndReason: string(stats.EndReason), Warnings: recordingWarnings(stats),
		}
		if err != nil {
			res.Error = err.Error()
		}
		if v, vErr := a.videoInfo(path, res.Warnings); vErr == nil && v.SizeBytes > 0 {
			res.SizeBytes, res.URL = v.SizeBytes, v.URL
			a.recordRecent(path)
		}
		setExcludedFromCapture(a.window, false)
		a.exitRecordingBar()
		a.emit("recording:finished", res)
	})
}

func recordingWarnings(st recorder.Stats) []string {
	var w []string
	if st.EndReason == recorder.EndWindowClosed {
		w = append(w, "The recorded window was closed, so the recording stopped.")
	}
	for _, a := range st.Audio {
		if a.Err != "" {
			w = append(w, fmt.Sprintf("%s stopped: %s", a.Name, a.Err))
		}
		if a.Reopens > 0 {
			w = append(w, fmt.Sprintf("%s device changed during the recording (%d×); the gap is silent.", a.Name, a.Reopens))
		}
	}
	return w
}

// recordingPath is a new file in the quick-save folder.
func (a *App) recordingPath() (string, error) {
	settings, err := a.settingsService.GetAll()
	if err != nil {
		return "", err
	}
	dir := settings.DefaultSavePath
	if dir == "" {
		if dir, err = a.fileService.GetDefaultSavePath(); err != nil {
			return "", fmt.Errorf("failed to resolve the save folder: %w", err)
		}
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	base := filepath.Join(dir, "Recording_"+time.Now().Format("20060102_150405"))
	path := base + ".mp4"
	for i := 1; ; i++ {
		if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
			return path, nil
		}
		path = fmt.Sprintf("%s_%d.mp4", base, i)
	}
}

// OpenPath opens a file with its default app (e.g. a recording in the video
// player).
func (a *App) OpenPath(path string) error {
	return shell.Open(path)
}

// RevealPath opens the folder that contains path.
func (a *App) RevealPath(path string) error {
	return shell.Open(filepath.Dir(path))
}

const (
	recordingBarWidth  = 300
	recordingBarHeight = 110 // includes the title bar
)

// enterRecordingBar shrinks the editor into a small always-on-top control bar
// at the top centre of the screen.
func (a *App) enterRecordingBar() {
	a.saveWindowState()
	a.recordingBar.Store(true)
	a.window.UnMaximise()
	a.window.SetSize(recordingBarWidth, recordingBarHeight)
	a.window.SetAlwaysOnTop(true)
	// The position is relative to the screen the window is on, as before.
	if s, err := a.window.GetScreen(); err == nil && s != nil {
		a.window.SetRelativePosition((s.Size.Width-recordingBarWidth)/2, 12)
	}
	a.window.Show()
}

// exitRecordingBar restores the editor window.
func (a *App) exitRecordingBar() {
	a.recordingBar.Store(false)
	a.window.SetAlwaysOnTop(false)
	a.window.Center()
	a.ShowWindow()
}

// toggleRecording is the "record" hotkey: stop when recording, otherwise open
// the recording dialog.
func (a *App) toggleRecording() {
	if a.IsRecording() {
		a.StopRecording()
		return
	}
	a.emit("open:record")
	a.ShowWindow()
}
