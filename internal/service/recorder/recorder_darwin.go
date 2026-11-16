//go:build darwin

package recorder

/*
#cgo CFLAGS: -fobjc-arc
#cgo LDFLAGS: -framework ScreenCaptureKit -framework AVFoundation -framework CoreMedia -framework CoreVideo -framework CoreAudio -framework CoreGraphics -framework Foundation
#include <stdlib.h>
#include "recorder_darwin.h"
*/
import "C"

import (
	"encoding/json"
	"errors"
	"fmt"
	"runtime/cgo"
	"sync"
	"time"
	"unsafe"
)

// pumpInterval is how often mixed audio is handed to the encoder.
const pumpInterval = 100 * time.Millisecond

type macRecording struct {
	handle unsafe.Pointer
	ctx    cgo.Handle
	origin int64 // host-clock ns of timeline zero

	mix  *mixer
	srcs [2]*audioSource // indexed by FASP_SRC_*, nil when not recorded

	ended      chan struct{} // closed when the recording ends, for any reason
	endOnce    sync.Once
	waiterDone chan struct{}
	pumpStop   chan struct{}
	pumpDone   chan struct{}
	pumpErr    error

	stopOnce sync.Once
	stats    Stats
	err      error
}

// Start begins recording with ScreenCaptureKit. It needs the Screen Recording
// permission, like screenshots do, and the Microphone permission when the
// microphone is recorded.
func Start(opts Options) (Recording, error) {
	if err := opts.normalize(); err != nil {
		return nil, err
	}
	r := &macRecording{
		ended:      make(chan struct{}),
		waiterDone: make(chan struct{}),
		pumpStop:   make(chan struct{}),
		pumpDone:   make(chan struct{}),
	}
	if opts.SystemAudio || opts.Microphone {
		r.mix = &mixer{}
		if opts.SystemAudio {
			r.srcs[C.FASP_SRC_SYSTEM] = newAudioSource("system audio", opts.SystemVolume)
		}
		if opts.Microphone {
			r.srcs[C.FASP_SRC_MIC] = newAudioSource("microphone", opts.MicVolume)
		}
		for _, s := range r.srcs {
			if s != nil {
				r.mix.sources = append(r.mix.sources, s)
			}
		}
	}
	// Audio callbacks find the recording through this handle; they can start
	// before fasp_rec_start returns, so it is fully set up first.
	r.ctx = cgo.NewHandle(r)

	path := C.CString(opts.Output)
	defer C.free(unsafe.Pointer(path))
	micID := C.CString(opts.MicDevice)
	defer C.free(unsafe.Pointer(micID))
	o := C.fasp_rec_opts{
		windowID: C.uint32_t(opts.Window),
		x:        C.double(opts.Region.X), y: C.double(opts.Region.Y),
		w: C.double(opts.Region.Width), h: C.double(opts.Region.Height),
		fps: C.int(opts.FPS), bitrate: C.int(opts.Bitrate),
		micID: micID,
		ctx:   C.uintptr_t(r.ctx),
		path:  path,
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
	if opts.SystemAudio {
		o.systemAudio = 1
	}
	if opts.Microphone {
		o.microphone = 1
	}
	var origin C.int64_t
	var cerr *C.char
	h := C.fasp_rec_start(&o, &origin, &cerr)
	if h == nil {
		r.ctx.Delete()
		return nil, takeError(cerr)
	}
	r.handle, r.origin = h, int64(origin)

	go r.waitEnd()
	go r.pump()
	return r, nil
}

// waitEnd closes ended when the stream stops on its own (the window closed).
func (r *macRecording) waitEnd() {
	defer close(r.waiterDone)
	if C.fasp_rec_wait_end(r.handle) == 1 {
		r.endOnce.Do(func() { close(r.ended) })
	}
}

// pump mixes audio a little behind real time, so late packets still make it.
func (r *macRecording) pump() {
	defer close(r.pumpDone)
	if r.mix == nil {
		return
	}
	t := time.NewTicker(pumpInterval)
	defer t.Stop()
	for {
		select {
		case <-r.pumpStop:
			return
		case <-t.C:
		}
		if err := r.writeAudio(r.frameAt(int64(C.fasp_rec_now_ns())) - audioLatency); err != nil {
			r.pumpErr = err
			return
		}
	}
}

// frameAt converts a host time to a timeline frame.
func (r *macRecording) frameAt(ns int64) int64 {
	return (ns - r.origin) * audioRate / int64(time.Second)
}

// writeAudio mixes timeline frames up to upTo and appends them to the file.
func (r *macRecording) writeAudio(upTo int64) error {
	pcm, start := r.mix.take(upTo)
	if len(pcm) == 0 {
		return nil
	}
	var cerr *C.char
	if C.fasp_rec_write_audio(r.handle, (*C.int16_t)(unsafe.Pointer(&pcm[0])), C.int(len(pcm)/audioChannels),
		C.int64_t(start), &cerr) != 0 {
		return takeError(cerr)
	}
	return nil
}

// faspRecAudio receives one audio buffer as interleaved stereo float. relNs
// is its capture time relative to timeline zero: the first buffers can arrive
// before fasp_rec_start returns, so the C side, which already knows the
// origin, does the subtraction.
//
//export faspRecAudio
func faspRecAudio(ctx C.uintptr_t, source C.int, relNs C.int64_t, data *C.float, frames C.int) {
	r, ok := cgo.Handle(ctx).Value().(*macRecording)
	if !ok || int(source) < 0 || int(source) >= len(r.srcs) || r.srcs[source] == nil || frames <= 0 {
		return
	}
	n := int(frames)
	pcm := make([]float32, n*audioChannels)
	copy(pcm, unsafe.Slice((*float32)(unsafe.Pointer(data)), n*audioChannels))
	r.srcs[source].push(int64(relNs)*audioRate/int64(time.Second), pcm, int64(n))
}

// Done is closed when the recorded window closes, or after Stop.
func (r *macRecording) Done() <-chan struct{} { return r.ended }

func (r *macRecording) Stop() (Stats, error) {
	r.stopOnce.Do(func() {
		defer r.endOnce.Do(func() { close(r.ended) })
		// Release the waiter and the pump before the recorder is torn down.
		C.fasp_rec_wake(r.handle)
		<-r.waiterDone
		close(r.pumpStop)
		<-r.pumpDone

		// Stop every source first so all audio up to now is in, then flush
		// the tail without waiting for latency.
		var endNs C.int64_t
		C.fasp_rec_halt(r.handle, &endNs)
		r.ctx.Delete()
		end := r.frameAt(int64(endNs))
		var audioErr error
		if r.mix != nil {
			for i, src := range r.srcs {
				if src == nil {
					continue
				}
				st := src.finish(end)
				if e := C.fasp_rec_source_error(r.handle, C.int(i)); e != nil {
					st.Err = C.GoString(e)
					C.free(unsafe.Pointer(e))
				}
				r.stats.Audio = append(r.stats.Audio, st)
			}
			audioErr = r.pumpErr
			if audioErr == nil {
				audioErr = r.writeAudio(end)
			}
		}

		var st C.fasp_rec_stats
		var cerr *C.char
		if C.fasp_rec_finish(r.handle, endNs, &st, &cerr) != 0 {
			r.err = takeError(cerr)
		} else if audioErr != nil {
			r.err = fmt.Errorf("audio: %w", audioErr)
		}
		r.stats.Width, r.stats.Height = int(st.width), int(st.height)
		r.stats.Frames, r.stats.Captured, r.stats.Dropped = int(st.frames), int(st.captured), int(st.dropped)
		r.stats.Duration = time.Duration(int64(endNs) - r.origin)
		r.stats.Pipeline = "ScreenCaptureKit + AVAssetWriter H.264 (VFR)"
		if r.mix != nil {
			r.stats.Pipeline += " + AAC"
		}
		r.stats.EndReason = EndStopped
		if st.windowClosed != 0 {
			r.stats.EndReason = EndWindowClosed
		}
		if n := int(st.audioDropped); n > 0 && len(r.stats.Audio) > 0 {
			r.stats.Audio[0].Err = joinErr(r.stats.Audio[0].Err,
				fmt.Sprintf("%d audio buffers were dropped by the writer", n))
		}
	})
	return r.stats, r.err
}

func joinErr(a, b string) string {
	if a == "" {
		return b
	}
	return a + "; " + b
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

// ListMicrophones returns the audio input devices. Listing does not need the
// Microphone permission; recording does.
func ListMicrophones() ([]AudioDevice, error) {
	var out, cerr *C.char
	if C.fasp_rec_list_mics(&out, &cerr) != 0 {
		return nil, takeError(cerr)
	}
	defer C.free(unsafe.Pointer(out))
	var ds []AudioDevice
	if err := json.Unmarshal([]byte(C.GoString(out)), &ds); err != nil {
		return nil, err
	}
	return ds, nil
}

func takeError(cerr *C.char) error {
	if cerr == nil {
		return errors.New("unknown error")
	}
	defer C.free(unsafe.Pointer(cerr))
	return errors.New(C.GoString(cerr))
}
