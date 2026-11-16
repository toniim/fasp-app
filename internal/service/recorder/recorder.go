// Package recorder records the screen to an H.264 MP4 file with the
// platform's native capture and encoding stack:
//
//   - Windows: Windows.Graphics.Capture (WGC) frames on a D3D11 device,
//     encoded by a Media Foundation sink writer (hardware H.264 MFT).
//   - macOS: ScreenCaptureKit (SCStream) frames, encoded by AVAssetWriter
//     (VideoToolbox).
//
// SPIKE: this package is a feasibility prototype for screen recording. See
// docs/spike-screen-recording.md for the findings and what is still missing
// before it can ship.
package recorder

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

// Mode selects what is recorded.
type Mode string

const (
	// ModeDisplay records the whole display under the mouse cursor.
	ModeDisplay Mode = "display"
	// ModeWindow records a single window, following it when it moves and
	// including it when it is covered by other windows.
	ModeWindow Mode = "window"
	// ModeRegion records a fixed rectangle of one display.
	ModeRegion Mode = "region"
)

// Rect is a rectangle on the desktop.
//
// Windows: physical pixels in virtual-screen coordinates (the app is
// per-monitor DPI aware). macOS: points in global display coordinates with
// the origin at the top-left of the main display (Quartz space).
type Rect struct {
	X, Y, Width, Height int
}

// Options configure a recording.
type Options struct {
	Mode Mode
	// Window is the platform window handle for ModeWindow: an HWND on
	// Windows, a CGWindowID on macOS. See ListWindows.
	Window uint64
	// Region is the area for ModeRegion. It must lie on a single display.
	Region Rect
	// FPS is the output frame rate. Defaults to 30.
	FPS int
	// Bitrate is the target H.264 bitrate in bits per second. Zero picks one
	// from the frame size and rate.
	Bitrate int
	// Cursor draws the mouse cursor into the video.
	Cursor bool
	// Output is the .mp4 path to write.
	Output string
	// SystemAudio records everything the computer plays (loopback of the
	// default output device).
	SystemAudio bool
	// Microphone records a microphone, mixed with system audio into one
	// AAC track.
	Microphone bool
	// MicDevice picks the microphone (see ListMicrophones); "" follows the
	// system default.
	MicDevice string
	// SystemVolume and MicVolume scale each source in the mix; 0 means 1.
	SystemVolume, MicVolume float64
	// ForceCPU disables the zero-copy GPU encode path on Windows (frames go
	// through a CPU staging copy instead). Used to compare the two paths.
	ForceCPU bool
}

// Stats describe a finished recording.
type Stats struct {
	Width, Height int
	// Frames is the number of frames written to the file.
	Frames int
	// Captured is the number of new frames delivered by the OS. A static
	// screen delivers few; the last one is repeated to keep a constant rate.
	Captured int
	// Dropped is the number of frame slots skipped because the pipeline fell
	// behind the requested rate.
	Dropped  int
	Duration time.Duration
	// Pipeline describes the encode path that was used.
	Pipeline string
	// Audio has one entry per recorded audio source.
	Audio []AudioStats
	// EndReason says why the recording ended.
	EndReason EndReason
}

// EndReason says why a recording ended.
type EndReason string

const (
	// EndStopped: Stop was called.
	EndStopped EndReason = "stopped"
	// EndWindowClosed: the recorded window was closed; the file is complete
	// up to that moment.
	EndWindowClosed EndReason = "window_closed"
)

// Recording is an in-progress recording.
type Recording interface {
	// Stop ends the recording, finalizes the file and reports what was
	// written. It is safe to call once.
	Stop() (Stats, error)
	// Done is closed when the recording ends on its own (e.g. the recorded
	// window closed) or after Stop. Call Stop to get the result either way.
	Done() <-chan struct{}
}

// AudioDevice is a microphone the user can pick.
type AudioDevice struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// Window is a top-level window that can be recorded.
type Window struct {
	Handle uint64 `json:"handle"`
	Title  string `json:"title"`
	App    string `json:"app"`
	Width  int    `json:"width"`
	Height int    `json:"height"`
}

func (o *Options) normalize() error {
	if o.FPS <= 0 {
		o.FPS = 30
	}
	if o.SystemVolume <= 0 {
		o.SystemVolume = 1
	}
	if o.MicVolume <= 0 {
		o.MicVolume = 1
	}
	if o.FPS > 120 {
		return fmt.Errorf("fps %d is too high (max 120)", o.FPS)
	}
	if o.Output == "" {
		return errors.New("output path is required")
	}
	if !strings.EqualFold(extOf(o.Output), ".mp4") {
		return fmt.Errorf("output %q must be an .mp4 file", o.Output)
	}
	switch o.Mode {
	case ModeDisplay:
	case ModeWindow:
		if o.Window == 0 {
			return errors.New("window mode needs a window handle")
		}
	case ModeRegion:
		if o.Region.Width < 2 || o.Region.Height < 2 {
			return errors.New("region must be at least 2x2")
		}
	default:
		return fmt.Errorf("unknown mode %q", o.Mode)
	}
	return nil
}

// bitrateFor picks a bitrate when none is set: ~0.1 bit per pixel per frame,
// which keeps screen text sharp, clamped to a sane range.
func bitrateFor(w, h, fps, requested int) int {
	if requested > 0 {
		return requested
	}
	b := w * h * fps / 10
	const lo, hi = 500_000, 40_000_000
	if b < lo {
		return lo
	}
	if b > hi {
		return hi
	}
	return b
}

// even rounds down to an even number: H.264 needs even frame dimensions.
func even(n int) int { return n &^ 1 }

func extOf(path string) string {
	for i := len(path) - 1; i >= 0 && path[i] != '/' && path[i] != '\\'; i-- {
		if path[i] == '.' {
			return path[i:]
		}
	}
	return ""
}
