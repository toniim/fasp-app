//go:build darwin

package clipboard

/*
#cgo CFLAGS: -x objective-c -fobjc-arc
#cgo LDFLAGS: -framework AppKit
#import <AppKit/AppKit.h>
#include <stdlib.h>

static int fasp_copy_file(const char *path) {
    @autoreleasepool {
        NSURL *url = [NSURL fileURLWithPath:[NSString stringWithUTF8String:path]];
        NSPasteboard *pb = [NSPasteboard generalPasteboard];
        [pb clearContents];
        return [pb writeObjects:@[url]] ? 1 : 0;
    }
}

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
	"os"
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

// CopyFiles puts the first file on the general pasteboard as a file URL.
func (s *serviceImpl) CopyFiles(paths []string) error {
	if len(paths) == 0 {
		return errors.New("failed to copy files: no files")
	}
	if _, err := os.Stat(paths[0]); err != nil {
		return err
	}
	cpath := C.CString(paths[0])
	defer C.free(unsafe.Pointer(cpath))
	if C.fasp_copy_file(cpath) == 0 {
		return errors.New("failed to copy files: pasteboard rejected the file")
	}
	return nil
}
