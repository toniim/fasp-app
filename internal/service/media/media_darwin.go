//go:build darwin

package media

/*
#cgo CFLAGS: -fobjc-arc
#cgo LDFLAGS: -framework AVFoundation -framework CoreMedia -framework CoreVideo -framework Accelerate -framework Foundation
#include <stdlib.h>
#include "media_darwin.h"
*/
import "C"

import (
	"errors"
	"runtime/cgo"
	"time"
	"unsafe"
)

func probeNative(path string) (time.Duration, int, int, error) {
	cpath := C.CString(path)
	defer C.free(unsafe.Pointer(cpath))
	var sec C.double
	var w, h C.int
	var cerr *C.char
	if C.fasp_media_probe(cpath, &sec, &w, &h, &cerr) != 0 {
		return 0, 0, 0, takeError(cerr)
	}
	return time.Duration(float64(sec) * float64(time.Second)), int(w), int(h), nil
}

func trimNative(in, out string, start, end time.Duration) error {
	cin, cout := C.CString(in), C.CString(out)
	defer C.free(unsafe.Pointer(cin))
	defer C.free(unsafe.Pointer(cout))
	var cerr *C.char
	if C.fasp_media_trim(cin, cout, C.int64_t(start), C.int64_t(end), &cerr) != 0 {
		return takeError(cerr)
	}
	return nil
}

type decodeState struct {
	fn  func(i int, bgra []byte) error
	err error
}

// decodeFrames calls fn with count BGRA frames of size w x h, sampled at
// start + k/fps. The slice is only valid during the call.
func decodeFrames(in string, start time.Duration, count int, fps float64, w, h int, fn func(i int, bgra []byte) error) error {
	st := &decodeState{fn: fn}
	handle := cgo.NewHandle(st)
	defer handle.Delete()

	cin := C.CString(in)
	defer C.free(unsafe.Pointer(cin))
	var cerr *C.char
	switch C.fasp_media_decode(cin, C.int64_t(start), C.int(count), C.double(fps), C.int(w), C.int(h),
		C.uintptr_t(handle), &cerr) {
	case 0:
		return nil
	case 1:
		return st.err
	}
	return takeError(cerr)
}

// faspMediaFrame hands one decoded frame to the Go callback. A non-zero
// return stops the decode; the error stays in the state.
//
//export faspMediaFrame
func faspMediaFrame(ctx C.uintptr_t, idx C.int, data *C.uint8_t, w, h C.int) C.int {
	st := cgo.Handle(ctx).Value().(*decodeState)
	if err := st.fn(int(idx), unsafe.Slice((*byte)(unsafe.Pointer(data)), int(w)*int(h)*4)); err != nil {
		st.err = err
		return 1
	}
	return 0
}

func takeError(cerr *C.char) error {
	if cerr == nil {
		return errors.New("unknown error")
	}
	defer C.free(unsafe.Pointer(cerr))
	return errors.New(C.GoString(cerr))
}
