//go:build darwin

package clipboard

/*
#cgo CFLAGS: -x objective-c -fobjc-arc
#cgo LDFLAGS: -framework AppKit
#import <AppKit/AppKit.h>

static int fasp_copy_png(const void *bytes, int length) {
    @autoreleasepool {
        NSData *data = [NSData dataWithBytes:bytes length:(NSUInteger)length];
        NSPasteboard *pb = [NSPasteboard generalPasteboard];
        [pb clearContents];
        return [pb setData:data forType:NSPasteboardTypePNG] ? 1 : 0;
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
