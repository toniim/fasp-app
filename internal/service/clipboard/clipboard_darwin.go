//go:build darwin

package clipboard

/*
#cgo CFLAGS: -x objective-c -fobjc-arc
#cgo LDFLAGS: -framework AppKit
#import <AppKit/AppKit.h>
#include <limits.h>
#include <stdlib.h>
#include <string.h>

static int fasp_copy_png(const void *bytes, int length) {
    @autoreleasepool {
        NSData *data = [NSData dataWithBytes:bytes length:(NSUInteger)length];
        NSPasteboard *pb = [NSPasteboard generalPasteboard];
        [pb clearContents];
        return [pb setData:data forType:NSPasteboardTypePNG] ? 1 : 0;
    }
}

// Returns a malloc'd PNG buffer (caller frees) and its length, or NULL when
// the pasteboard has no image. Prefers PNG; falls back to TIFF (what Preview
// and many apps put there), converted to PNG.
static void *fasp_read_png(int *outLength) {
    @autoreleasepool {
        *outLength = 0;
        NSPasteboard *pb = [NSPasteboard generalPasteboard];
        NSData *png = [pb dataForType:NSPasteboardTypePNG];
        if (png == nil) {
            NSData *tiff = [pb dataForType:NSPasteboardTypeTIFF];
            if (tiff == nil) {
                return NULL;
            }
            NSBitmapImageRep *rep = [NSBitmapImageRep imageRepWithData:tiff];
            if (rep == nil) {
                return NULL;
            }
            png = [rep representationUsingType:NSBitmapImageFileTypePNG properties:@{}];
        }
        if (png == nil || [png length] == 0 || [png length] > INT_MAX) {
            return NULL;
        }
        void *buf = malloc([png length]);
        if (buf == NULL) {
            return NULL;
        }
        memcpy(buf, [png bytes], [png length]);
        *outLength = (int)[png length];
        return buf;
    }
}
*/
import "C"

import (
	"errors"
	"unsafe"
)

// serviceImpl implements the Service interface for macOS
type serviceImpl struct{}

// New creates a new clipboard service instance
func New() Service {
	return &serviceImpl{}
}

// CopyImage puts PNG image data on the general pasteboard.
func (s *serviceImpl) CopyImage(data []byte) error {
	if len(data) == 0 {
		return errors.New("failed to copy to clipboard: no image data")
	}
	if C.fasp_copy_png(unsafe.Pointer(&data[0]), C.int(len(data))) == 0 {
		return errors.New("failed to copy to clipboard: pasteboard rejected the image")
	}
	return nil
}

// ReadImage returns the pasteboard image as PNG bytes.
func (s *serviceImpl) ReadImage() ([]byte, error) {
	var n C.int
	buf := C.fasp_read_png(&n)
	if buf == nil {
		return nil, ErrNoImage
	}
	defer C.free(buf)
	return C.GoBytes(buf, n), nil
}
