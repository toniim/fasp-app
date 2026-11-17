// Package media trims recordings and exports them as GIFs. Decoding and
// trimming use the platform's media stack (AVFoundation on macOS, Media
// Foundation on Windows); GIF encoding is pure Go.
package media

import (
	"bufio"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"time"
)

// ErrUnsupported is returned on platforms without a media backend.
var ErrUnsupported = errors.New("media: not supported on this platform")

const (
	defaultGIFFPS   = 12
	maxGIFFPS       = 30
	defaultGIFWidth = 960
	maxGIFWidth     = 4096
	minGIFWidth     = 2
)

// GIFOptions selects the part of a recording and the look of the GIF.
type GIFOptions struct {
	Start, End time.Duration // End 0 = to the end of the recording
	FPS        int           // 0 = 12, at most 30
	Width      int           // 0 = source width capped at 960; height keeps the aspect ratio
}

// gifPlan is GIFOptions resolved against a concrete recording.
type gifPlan struct {
	start    time.Duration
	count    int // frames to emit
	fps      int
	w, h     int
	duration time.Duration // length of the exported range
}

// Probe returns the length and the video size of a recording.
func Probe(path string) (duration time.Duration, width, height int, err error) {
	if _, err := os.Stat(path); err != nil {
		return 0, 0, 0, err
	}
	return probeNative(path)
}

// Trim writes the part of in between start and end to out as an MP4. The
// video and the audio are cut at the same times, so they stay in sync. End 0
// (or past the end) means the end of the recording.
func Trim(in, out string, start, end time.Duration) error {
	dur, _, _, err := Probe(in)
	if err != nil {
		return err
	}
	if end <= 0 || end > dur {
		end = dur
	}
	if start < 0 {
		start = 0
	}
	if end-start < 10*time.Millisecond {
		return fmt.Errorf("media: empty range %v-%v of a %v recording", start, end, dur)
	}
	return writeAtomically(in, out, func(tmp string) error { return trimNative(in, tmp, start, end) })
}

// ExportGIF writes a looping GIF. progress, when set, is called after each
// frame with the frames done and the frames in total.
func ExportGIF(in, out string, opt GIFOptions, progress func(done, total int)) error {
	p, err := planGIF(in, opt)
	if err != nil {
		return err
	}
	pal, err := samplePalette(in, p)
	if err != nil {
		return err
	}
	return writeAtomically(in, out, func(tmp string) (err error) {
		f, err := os.Create(tmp)
		if err != nil {
			return err
		}
		defer func() {
			if cerr := f.Close(); err == nil {
				err = cerr
			}
		}()
		bw := bufio.NewWriterSize(f, 1<<20)
		enc, err := newGIFEncoder(bw, p.w, p.h, p.fps, pal)
		if err != nil {
			return err
		}
		err = decodeFrames(in, p.start, p.count, float64(p.fps), p.w, p.h, func(i int, bgra []byte) error {
			if _, err := enc.addFrame(bgra); err != nil {
				return err
			}
			if progress != nil {
				progress(i+1, p.count)
			}
			return nil
		})
		if err != nil {
			return err
		}
		if err := enc.close(); err != nil {
			return err
		}
		return bw.Flush()
	})
}

// estimateWindowFrames is how many consecutive frames each estimate sample
// encodes: the first is a full frame, the rest are the deltas that make up
// most of a real GIF.
const (
	estimateWindowFrames = 3
	estimateWindows      = 3
)

// EstimateGIFSize predicts the size of the GIF ExportGIF would write, in
// bytes. It encodes a few short windows of the real recording instead of
// guessing from the pixel count, because the size depends on how much of the
// screen changes.
func EstimateGIFSize(in string, opt GIFOptions) (int64, error) {
	p, err := planGIF(in, opt)
	if err != nil {
		return 0, err
	}
	win := estimateWindowFrames
	if p.count < win {
		win = p.count
	}
	nWin := estimateWindows
	if p.count < win*nWin {
		nWin = 1
	}

	// Decode every window first: the palette is built from all of them, as
	// the export builds it from frames across the whole range.
	var windows [][][]byte
	hist := &histogram{}
	for k := 0; k < nWin; k++ {
		first := 0
		if nWin > 1 {
			// Windows at the start, middle and end, never past the last frame.
			first = (p.count - win) * k / (nWin - 1)
		}
		var frames [][]byte
		at := p.start + time.Duration(float64(first)/float64(p.fps)*float64(time.Second))
		err := decodeFrames(in, at, win, float64(p.fps), p.w, p.h, func(_ int, bgra []byte) error {
			frames = append(frames, append([]byte(nil), bgra...))
			hist.add(bgra)
			return nil
		})
		if err != nil {
			return 0, err
		}
		windows = append(windows, frames)
	}
	pal := hist.palette()

	var full, delta float64
	var nFull, nDelta int
	for _, frames := range windows {
		enc, err := newGIFEncoder(discard{}, p.w, p.h, p.fps, pal)
		if err != nil {
			return 0, err
		}
		for i, f := range frames {
			n, err := enc.addFrame(f)
			if err != nil {
				return 0, err
			}
			if i == 0 {
				full += float64(n)
				nFull++
			} else {
				delta += float64(n)
				nDelta++
			}
		}
	}
	if nFull == 0 {
		return 0, errors.New("media: no frames to estimate from")
	}
	est := full / float64(nFull)
	if nDelta > 0 {
		est += delta / float64(nDelta) * float64(p.count-1)
	}
	return int64(est) + gifOverhead(), nil
}

// discard is a writer that drops everything; estimates only need the sizes
// the encoder reports.
type discard struct{}

func (discard) Write(b []byte) (int, error) { return len(b), nil }

// planGIF resolves opt against the recording.
func planGIF(in string, opt GIFOptions) (gifPlan, error) {
	dur, sw, sh, err := Probe(in)
	if err != nil {
		return gifPlan{}, err
	}
	if sw <= 0 || sh <= 0 {
		return gifPlan{}, errors.New("media: recording has no video size")
	}
	start, end := opt.Start, opt.End
	if start < 0 {
		start = 0
	}
	if end <= 0 || end > dur {
		end = dur
	}
	if end-start < time.Millisecond {
		return gifPlan{}, fmt.Errorf("media: empty range %v-%v of a %v recording", start, end, dur)
	}
	fps := opt.FPS
	switch {
	case fps <= 0:
		fps = defaultGIFFPS
	case fps > maxGIFFPS:
		fps = maxGIFFPS
	}
	w := opt.Width
	if w <= 0 {
		w = sw
		if w > defaultGIFWidth {
			w = defaultGIFWidth
		}
	}
	w = min(max(w, minGIFWidth), maxGIFWidth)
	h := int(math.Round(float64(sh) * float64(w) / float64(sw)))
	if h < 1 {
		h = 1
	}
	span := end - start
	count := int(math.Round(span.Seconds() * float64(fps)))
	if count < 1 {
		count = 1
	}
	return gifPlan{start: start, count: count, fps: fps, w: w, h: h, duration: span}, nil
}

// samplePalette builds the one palette every frame of the GIF uses, from a
// handful of small frames spread over the range. Streaming the export needs
// the palette before the first frame is written.
func samplePalette(in string, p gifPlan) (*palette, error) {
	const samples, sampleWidth = 16, 192
	n := min(samples, p.count)
	sw := min(sampleWidth, p.w)
	sh := max(1, int(math.Round(float64(p.h)*float64(sw)/float64(p.w))))
	hist := &histogram{}
	err := decodeFrames(in, p.start, n, float64(n)/p.duration.Seconds(), sw, sh, func(_ int, bgra []byte) error {
		hist.add(bgra)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return hist.palette(), nil
}

// writeAtomically runs fn against a temporary file next to out and moves it
// into place, so a failed or cancelled run never leaves a broken file at out.
func writeAtomically(in, out string, fn func(tmp string) error) error {
	if same, err := sameFile(in, out); err != nil {
		return err
	} else if same {
		return errors.New("media: output must differ from the input")
	}
	// The container is chosen from the extension by some platform writers.
	tmp := out + ".part" + filepath.Ext(out)
	if err := os.Remove(tmp); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if err := fn(tmp); err != nil {
		os.Remove(tmp)
		return err
	}
	if err := os.Rename(tmp, out); err != nil {
		os.Remove(tmp)
		return err
	}
	return nil
}

func sameFile(a, b string) (bool, error) {
	ia, err := os.Stat(a)
	if err != nil {
		return false, err
	}
	ib, err := os.Stat(b)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return os.SameFile(ia, ib), nil
}
