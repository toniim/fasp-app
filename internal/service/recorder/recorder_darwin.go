//go:build darwin

package recorder

/*
#cgo CFLAGS: -fobjc-arc
#cgo LDFLAGS: -framework ScreenCaptureKit -framework AVFoundation -framework CoreMedia -framework CoreVideo -framework CoreGraphics -framework Foundation
#include <stdlib.h>
#include "recorder_darwin.h"
*/
import "C"

import (
	"encoding/json"
	"errors"
	"sync"
	"time"
	"unsafe"
)

type macRecording struct {
	handle unsafe.Pointer
	ended  chan struct{}
	once   sync.Once
	stats  Stats
	err    error
}

// Start begins recording with ScreenCaptureKit. It needs the Screen Recording
// permission, like screenshots do.
func Start(opts Options) (Recording, error) {
	if err := opts.normalize(); err != nil {
		return nil, err
	}
	if opts.SystemAudio || opts.Microphone {
		// SPIKE: audio is prototyped on Windows only so far. Plan: SCStream
		// audio output (capturesAudio) for system audio, AVCaptureSession for
		// the microphone on macOS 14, both fed into the shared mixer.
		return nil, errors.New("audio recording is not implemented on macOS yet")
	}
	path := C.CString(opts.Output)
	defer C.free(unsafe.Pointer(path))
	o := C.fasp_rec_opts{
		windowID: C.uint32_t(opts.Window),
		x:        C.double(opts.Region.X), y: C.double(opts.Region.Y),
		w: C.double(opts.Region.Width), h: C.double(opts.Region.Height),
		fps: C.int(opts.FPS), bitrate: C.int(opts.Bitrate),
		path: path,
	}
	switch opts.Mode {
	case ModeWindow:
		o.mode = C.FASP_REC_WINDOW
	case ModeRegion:
		o.mode = C.FASP_REC_REGION
	default:
		o.mode = C.FASP_REC_DISPLAY
	}
	if opts.Cursor {
		o.cursor = 1
	}
	var cerr *C.char
	h := C.fasp_rec_start(&o, &cerr)
	if h == nil {
		return nil, takeError(cerr)
	}
	return &macRecording{handle: h, ended: make(chan struct{})}, nil
}

// Done is closed after Stop.
// TODO(macos): also close it when SCStream stops on its own (window closed).
func (r *macRecording) Done() <-chan struct{} { return r.ended }

func (r *macRecording) Stop() (Stats, error) {
	r.once.Do(func() {
		defer close(r.ended)
		var st C.fasp_rec_stats
		var cerr *C.char
		if C.fasp_rec_stop(r.handle, &st, &cerr) != 0 {
			r.err = takeError(cerr)
		}
		r.stats = Stats{
			Width: int(st.width), Height: int(st.height),
			Frames: int(st.frames), Captured: int(st.captured), Dropped: int(st.dropped),
			Duration:  time.Duration(float64(st.seconds) * float64(time.Second)),
			Pipeline:  "ScreenCaptureKit + AVAssetWriter H.264 (VFR)",
			EndReason: EndStopped,
		}
	})
	return r.stats, r.err
}

// ListWindows returns the on-screen windows ScreenCaptureKit can record.
func ListWindows() ([]Window, error) {
	var out, cerr *C.char
	if C.fasp_rec_list_windows(&out, &cerr) != 0 {
		return nil, takeError(cerr)
	}
	defer C.free(unsafe.Pointer(out))
	var ws []Window
	if err := json.Unmarshal([]byte(C.GoString(out)), &ws); err != nil {
		return nil, err
	}
	return ws, nil
}

func takeError(cerr *C.char) error {
	if cerr == nil {
		return errors.New("unknown error")
	}
	defer C.free(unsafe.Pointer(cerr))
	return errors.New(C.GoString(cerr))
}

// ListMicrophones is not implemented on macOS yet.
func ListMicrophones() ([]AudioDevice, error) {
	return nil, errors.New("not implemented on macOS yet")
}
