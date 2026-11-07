//go:build darwin

#import <CoreGraphics/CoreGraphics.h>
#import <Foundation/Foundation.h>
#import <ImageIO/ImageIO.h>
#import <ScreenCaptureKit/ScreenCaptureKit.h>
#import <UniformTypeIdentifiers/UniformTypeIdentifiers.h>
#include <stdlib.h>
#include <string.h>

#include "capture_darwin.h"

static const int64_t kCaptureTimeout = 5 * NSEC_PER_SEC;

// cursorDisplayID returns the display under the mouse cursor, or the main
// display when it cannot be determined.
static CGDirectDisplayID cursorDisplayID(void) {
    CGEventRef ev = CGEventCreate(NULL);
    if (ev != NULL) {
        CGPoint p = CGEventGetLocation(ev);
        CFRelease(ev);
        CGDirectDisplayID under;
        uint32_t found = 0;
        if (CGGetDisplaysWithPoint(p, 1, &under, &found) == kCGErrorSuccess && found > 0) {
            return under;
        }
    }
    return CGMainDisplayID();
}

static char *copyMessage(NSString *msg) {
    return strdup(msg.UTF8String ?: "unknown error");
}

static char *copyError(NSString *what, NSError *err) {
    if (err == nil) return copyMessage(what);
    return copyMessage([NSString stringWithFormat:@"%@: %@", what, err.localizedDescription]);
}

// fasp_capture_cursor_display captures the display under the cursor as PNG.
// On success it returns 0 and sets *out (malloc'd, caller frees) and the
// image size. On failure it returns -1 and sets *errOut (malloc'd).
int fasp_capture_cursor_display(void **out, int *outLen, int *width, int *height, char **errOut) {
    @autoreleasepool {
        CGDirectDisplayID displayID = cursorDisplayID();

        // 1. Find the SCDisplay for that display.
        __block SCShareableContent *content = nil;
        __block NSError *contentErr = nil;
        dispatch_semaphore_t sem = dispatch_semaphore_create(0);
        [SCShareableContent getShareableContentWithCompletionHandler:^(SCShareableContent *c, NSError *e) {
            content = c;
            contentErr = e;
            dispatch_semaphore_signal(sem);
        }];
        if (dispatch_semaphore_wait(sem, dispatch_time(DISPATCH_TIME_NOW, kCaptureTimeout)) != 0) {
            *errOut = copyMessage(@"timed out listing displays");
            return -1;
        }
        if (content == nil) {
            *errOut = copyError(@"failed to list displays", contentErr);
            return -1;
        }

        SCDisplay *display = nil;
        for (SCDisplay *d in content.displays) {
            if (d.displayID == displayID) {
                display = d;
                break;
            }
        }
        if (display == nil) display = content.displays.firstObject;
        if (display == nil) {
            *errOut = copyMessage(@"no display found");
            return -1;
        }

        // 2. Capture it at its physical pixel size. SCStreamConfiguration
        // defaults to the point size, which halves a Retina capture.
        size_t pixelWidth = display.width;
        size_t pixelHeight = display.height;
        CGDisplayModeRef mode = CGDisplayCopyDisplayMode(display.displayID);
        if (mode != NULL) {
            pixelWidth = CGDisplayModeGetPixelWidth(mode);
            pixelHeight = CGDisplayModeGetPixelHeight(mode);
            CGDisplayModeRelease(mode);
        }

        SCContentFilter *filter = [[SCContentFilter alloc] initWithDisplay:display excludingWindows:@[]];
        SCStreamConfiguration *config = [[SCStreamConfiguration alloc] init];
        config.width = pixelWidth;
        config.height = pixelHeight;
        config.showsCursor = NO;

        __block CGImageRef image = NULL;
        __block NSError *captureErr = nil;
        sem = dispatch_semaphore_create(0);
        [SCScreenshotManager captureImageWithFilter:filter
                                      configuration:config
                                  completionHandler:^(CGImageRef img, NSError *e) {
            if (img != NULL) image = CGImageRetain(img);
            captureErr = e;
            dispatch_semaphore_signal(sem);
        }];
        if (dispatch_semaphore_wait(sem, dispatch_time(DISPATCH_TIME_NOW, kCaptureTimeout)) != 0) {
            // The handler may still fire later and retain an image; that leak
            // only happens on a timeout, which already means something is wrong.
            *errOut = copyMessage(@"timed out capturing the screen");
            return -1;
        }
        if (image == NULL) {
            *errOut = copyError(@"failed to capture the screen", captureErr);
            return -1;
        }

        // 3. Encode as PNG in memory.
        NSMutableData *png = [NSMutableData data];
        CGImageDestinationRef dest = CGImageDestinationCreateWithData(
            (__bridge CFMutableDataRef)png, (__bridge CFStringRef)UTTypePNG.identifier, 1, NULL);
        if (dest == NULL) {
            CGImageRelease(image);
            *errOut = copyMessage(@"failed to create PNG encoder");
            return -1;
        }
        CGImageDestinationAddImage(dest, image, NULL);
        bool ok = CGImageDestinationFinalize(dest);
        CFRelease(dest);
        *width = (int)CGImageGetWidth(image);
        *height = (int)CGImageGetHeight(image);
        CGImageRelease(image);
        if (!ok) {
            *errOut = copyMessage(@"failed to encode PNG");
            return -1;
        }

        void *buf = malloc(png.length);
        if (buf == NULL) {
            *errOut = copyMessage(@"out of memory");
            return -1;
        }
        memcpy(buf, png.bytes, png.length);
        *out = buf;
        *outLen = (int)png.length;
        return 0;
    }
}
