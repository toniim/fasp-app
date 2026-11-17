//go:build darwin

package ocr

/*
#cgo CFLAGS: -fobjc-arc
#cgo LDFLAGS: -framework Vision -framework Foundation -framework CoreGraphics -framework ImageIO
#include <stdlib.h>
#include "ocr_darwin.h"
*/
import "C"

import (
	"errors"
	"fmt"
	"strings"
	"unsafe"
)

// Recognize runs Vision text recognition on a PNG. languages are BCP 47 tags
// such as "en-US" or "vi-VN"; empty lets Vision detect the language.
func Recognize(png []byte, languages []string) (*Result, error) {
	if len(png) == 0 {
		return nil, errors.New("ocr: empty image")
	}
	cLangs := C.CString(strings.Join(languages, ","))
	defer C.free(unsafe.Pointer(cLangs))

	var (
		lines         *C.fasp_ocr_line
		count         C.int
		width, height C.int
		cerr          *C.char
	)
	if C.fasp_ocr_recognize(unsafe.Pointer(&png[0]), C.int(len(png)), cLangs, &lines, &count, &width, &height, &cerr) != 0 {
		return nil, cError("ocr", cerr)
	}
	defer C.fasp_ocr_free_lines(lines, count)

	out := make([]Line, 0, int(count))
	for _, l := range unsafe.Slice(lines, int(count)) {
		x, y, w, h := boxFromNormalized(float64(l.x), float64(l.y), float64(l.w), float64(l.h), int(width), int(height))
		out = append(out, Line{Text: C.GoString(l.text), X: x, Y: y, W: w, H: h})
	}
	return buildResult(out), nil
}

// SupportedLanguages lists the languages Vision can recognise.
func SupportedLanguages() ([]string, error) {
	var out, cerr *C.char
	if C.fasp_ocr_supported_languages(&out, &cerr) != 0 {
		return nil, cError("ocr", cerr)
	}
	defer C.free(unsafe.Pointer(out))
	s := C.GoString(out)
	if s == "" {
		return nil, nil
	}
	return strings.Split(s, "\n"), nil
}

func cError(prefix string, cerr *C.char) error {
	msg := "unknown error"
	if cerr != nil {
		msg = C.GoString(cerr)
		C.free(unsafe.Pointer(cerr))
	}
	return fmt.Errorf("%s: %s", prefix, msg)
}
