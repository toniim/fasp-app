//go:build darwin

#import <AVFoundation/AVFoundation.h>
#import <CoreGraphics/CoreGraphics.h>
#import <CoreMedia/CoreMedia.h>
#import <Foundation/Foundation.h>
#import <ScreenCaptureKit/ScreenCaptureKit.h>
#include <stdlib.h>
#include <string.h>
#include <unistd.h>

#include "recorder_darwin.h"

// SPIKE: ScreenCaptureKit (SCStream) -> AVAssetWriter (VideoToolbox H.264).
// macOS 15 adds SCRecordingOutput, which writes the file itself; the app
// supports macOS 14, so frames are appended to an AVAssetWriter here.

static const int64_t kTimeout = 5 * NSEC_PER_SEC;

static char *copyMsg(NSString *msg) { return strdup(msg.UTF8String ?: "unknown error"); }

static char *copyErr(NSString *what, NSError *err) {
    if (err == nil) return copyMsg(what);
    return copyMsg([NSString stringWithFormat:@"%@: %@", what, err.localizedDescription]);
}

static BOOL waitFor(dispatch_semaphore_t sem) {
    return dispatch_semaphore_wait(sem, dispatch_time(DISPATCH_TIME_NOW, kTimeout)) == 0;
}

static SCShareableContent *shareableContent(char **errOut) {
    __block SCShareableContent *content = nil;
    __block NSError *err = nil;
    dispatch_semaphore_t sem = dispatch_semaphore_create(0);
    [SCShareableContent getShareableContentExcludingDesktopWindows:YES
                                               onScreenWindowsOnly:YES
                                                 completionHandler:^(SCShareableContent *c, NSError *e) {
        content = c;
        err = e;
        dispatch_semaphore_signal(sem);
    }];
    if (!waitFor(sem)) {
        *errOut = copyMsg(@"timed out listing screen content");
        return nil;
    }
    if (content == nil) *errOut = copyErr(@"failed to list screen content (Screen Recording permission?)", err);
    return content;
}

static CGDirectDisplayID displayAtPoint(CGPoint p) {
    CGDirectDisplayID d;
    uint32_t n = 0;
    if (CGGetDisplaysWithPoint(p, 1, &d, &n) == kCGErrorSuccess && n > 0) return d;
    return CGMainDisplayID();
}

static CGPoint cursorLocation(void) {
    CGEventRef ev = CGEventCreate(NULL);
    if (ev == NULL) return CGPointZero;
    CGPoint p = CGEventGetLocation(ev);
    CFRelease(ev);
    return p;
}

@interface FaspRecorder : NSObject <SCStreamOutput, SCStreamDelegate>
@end

@implementation FaspRecorder {
@public
    SCStream *stream;
    AVAssetWriter *writer;
    AVAssetWriterInput *input;
    dispatch_queue_t queue;
    BOOL started;
    CMTime firstPTS;
    int width, height, frames, captured, dropped;
    NSError *failure;
}

- (void)stream:(SCStream *)s didOutputSampleBuffer:(CMSampleBufferRef)sb ofType:(SCStreamOutputType)type {
    if (type != SCStreamOutputTypeScreen || !CMSampleBufferIsValid(sb) || failure != nil) return;
    // SCStream also sends "idle" status buffers without an image when the
    // screen does not change; only complete frames carry pixels.
    CFArrayRef atts = CMSampleBufferGetSampleAttachmentsArray(sb, false);
    if (atts == NULL || CFArrayGetCount(atts) == 0) return;
    NSDictionary *info = (__bridge NSDictionary *)CFArrayGetValueAtIndex(atts, 0);
    NSNumber *status = info[SCStreamFrameInfoStatus];
    if (status == nil || status.integerValue != SCFrameStatusComplete) return;
    captured++;

    CMTime pts = CMSampleBufferGetPresentationTimeStamp(sb);
    if (!started) {
        if (![writer startWriting]) {
            failure = writer.error;
            return;
        }
        [writer startSessionAtSourceTime:pts];
        firstPTS = pts;
        started = YES;
    }
    if (!input.isReadyForMoreMediaData) {
        dropped++;
        return;
    }
    if ([input appendSampleBuffer:sb]) {
        frames++;
    } else {
        failure = writer.error;
    }
}

- (void)stream:(SCStream *)s didStopWithError:(NSError *)error {
    // Fires when the window closes, the display goes away or the user
    // revokes the permission. Recorded on the sample queue to stay ordered.
    dispatch_async(queue, ^{ self->failure = error; });
}
@end

void *fasp_rec_start(const fasp_rec_opts *o, char **errOut) {
    @autoreleasepool {
        SCShareableContent *content = shareableContent(errOut);
        if (content == nil) return NULL;

        // Never record Fasp's own windows (toolbar, region overlay).
        NSMutableArray<SCWindow *> *own = [NSMutableArray array];
        pid_t me = getpid();
        for (SCWindow *w in content.windows) {
            if (w.owningApplication.processID == me) [own addObject:w];
        }

        SCContentFilter *filter = nil;
        CGRect sourceRect = CGRectNull;
        CGSize points;
        if (o->mode == FASP_REC_WINDOW) {
            SCWindow *win = nil;
            for (SCWindow *w in content.windows) {
                if (w.windowID == o->windowID) {
                    win = w;
                    break;
                }
            }
            if (win == nil) {
                *errOut = copyMsg(@"window not found (closed, minimized or on another Space)");
                return NULL;
            }
            filter = [[SCContentFilter alloc] initWithDesktopIndependentWindow:win];
            points = win.frame.size;
        } else {
            CGPoint at = o->mode == FASP_REC_REGION
                ? CGPointMake(o->x + o->w / 2, o->y + o->h / 2)
                : cursorLocation();
            CGDirectDisplayID displayID = displayAtPoint(at);
            SCDisplay *display = nil;
            for (SCDisplay *d in content.displays) {
                if (d.displayID == displayID) {
                    display = d;
                    break;
                }
            }
            if (display == nil) display = content.displays.firstObject;
            if (display == nil) {
                *errOut = copyMsg(@"no display found");
                return NULL;
            }
            filter = [[SCContentFilter alloc] initWithDisplay:display excludingWindows:own];
            points = CGSizeMake(display.width, display.height);
            if (o->mode == FASP_REC_REGION) {
                CGRect bounds = CGDisplayBounds(display.displayID);
                CGRect r = CGRectMake(o->x - bounds.origin.x, o->y - bounds.origin.y, o->w, o->h);
                r = CGRectIntersection(r, CGRectMake(0, 0, bounds.size.width, bounds.size.height));
                if (CGRectIsEmpty(r)) {
                    *errOut = copyMsg(@"region is not on any display");
                    return NULL;
                }
                sourceRect = r;
                points = r.size;
            }
        }

        // Record at physical pixels (Retina = 2x points).
        CGFloat scale = filter.pointPixelScale;
        if (scale <= 0) scale = 1;
        int width = ((int)(points.width * scale)) & ~1;
        int height = ((int)(points.height * scale)) & ~1;
        if (width < 2 || height < 2) {
            *errOut = copyMsg(@"capture area is empty");
            return NULL;
        }
        int bitrate = o->bitrate;
        if (bitrate <= 0) {
            long long b = (long long)width * height * o->fps / 10;
            bitrate = (int)MAX(2000000LL, MIN(40000000LL, b));
        }

        SCStreamConfiguration *config = [[SCStreamConfiguration alloc] init];
        config.width = width;
        config.height = height;
        config.minimumFrameInterval = CMTimeMake(1, o->fps);
        config.pixelFormat = kCVPixelFormatType_32BGRA;
        config.showsCursor = o->cursor != 0;
        config.queueDepth = 6;
        if (!CGRectIsNull(sourceRect)) config.sourceRect = sourceRect;

        NSURL *url = [NSURL fileURLWithPath:[NSString stringWithUTF8String:o->path]];
        [[NSFileManager defaultManager] removeItemAtURL:url error:nil];
        NSError *err = nil;
        AVAssetWriter *writer = [AVAssetWriter assetWriterWithURL:url fileType:AVFileTypeMPEG4 error:&err];
        if (writer == nil) {
            *errOut = copyErr(@"failed to create the video file", err);
            return NULL;
        }
        NSDictionary *settings = @{
            AVVideoCodecKey : AVVideoCodecTypeH264,
            AVVideoWidthKey : @(width),
            AVVideoHeightKey : @(height),
            AVVideoCompressionPropertiesKey : @{
                AVVideoAverageBitRateKey : @(bitrate),
                AVVideoExpectedSourceFrameRateKey : @(o->fps),
                AVVideoMaxKeyFrameIntervalKey : @(o->fps * 2),
                AVVideoProfileLevelKey : AVVideoProfileLevelH264HighAutoLevel,
            },
        };
        AVAssetWriterInput *input = [AVAssetWriterInput assetWriterInputWithMediaType:AVMediaTypeVideo
                                                                       outputSettings:settings];
        input.expectsMediaDataInRealTime = YES;
        if (![writer canAddInput:input]) {
            *errOut = copyMsg(@"the H.264 encoder rejected the video settings");
            return NULL;
        }
        [writer addInput:input];

        FaspRecorder *rec = [[FaspRecorder alloc] init];
        rec->writer = writer;
        rec->input = input;
        rec->width = width;
        rec->height = height;
        rec->queue = dispatch_queue_create("fasp.recorder", DISPATCH_QUEUE_SERIAL);
        rec->stream = [[SCStream alloc] initWithFilter:filter configuration:config delegate:rec];
        if (![rec->stream addStreamOutput:rec type:SCStreamOutputTypeScreen sampleHandlerQueue:rec->queue error:&err]) {
            *errOut = copyErr(@"failed to add the stream output", err);
            return NULL;
        }

        __block NSError *startErr = nil;
        dispatch_semaphore_t sem = dispatch_semaphore_create(0);
        [rec->stream startCaptureWithCompletionHandler:^(NSError *e) {
            startErr = e;
            dispatch_semaphore_signal(sem);
        }];
        if (!waitFor(sem)) {
            *errOut = copyMsg(@"timed out starting the capture");
            return NULL;
        }
        if (startErr != nil) {
            *errOut = copyErr(@"failed to start the capture", startErr);
            return NULL;
        }
        return (__bridge_retained void *)rec;
    }
}

int fasp_rec_stop(void *handle, fasp_rec_stats *st, char **errOut) {
    @autoreleasepool {
        FaspRecorder *rec = (__bridge_transfer FaspRecorder *)handle;
        dispatch_semaphore_t sem = dispatch_semaphore_create(0);
        [rec->stream stopCaptureWithCompletionHandler:^(NSError *e) { dispatch_semaphore_signal(sem); }];
        waitFor(sem);
        dispatch_sync(rec->queue, ^{}); // let in-flight frames finish

        // SCStream only delivers frames when something changes, so the file
        // has a variable frame rate; ending the session now holds the last
        // frame until the moment the user pressed stop.
        CMTime end = CMClockGetTime(CMClockGetHostTimeClock());
        st->width = rec->width;
        st->height = rec->height;
        st->frames = rec->frames;
        st->captured = rec->captured;
        st->dropped = rec->dropped;
        st->seconds = rec->started ? CMTimeGetSeconds(CMTimeSubtract(end, rec->firstPTS)) : 0;

        if (!rec->started) {
            [rec->writer cancelWriting];
            *errOut = rec->failure ? copyErr(@"recording failed", rec->failure) : copyMsg(@"no frames were captured");
            return -1;
        }
        [rec->writer endSessionAtSourceTime:end];
        [rec->input markAsFinished];
        sem = dispatch_semaphore_create(0);
        [rec->writer finishWritingWithCompletionHandler:^{ dispatch_semaphore_signal(sem); }];
        if (!waitFor(sem)) {
            *errOut = copyMsg(@"timed out finishing the video file");
            return -1;
        }
        if (rec->writer.status != AVAssetWriterStatusCompleted) {
            *errOut = copyErr(@"failed to write the video file", rec->writer.error);
            return -1;
        }
        if (rec->failure != nil) {
            // The file is valid up to the failure (e.g. the window closed).
            *errOut = copyErr(@"recording stopped early", rec->failure);
            return -1;
        }
        return 0;
    }
}

int fasp_rec_list_windows(char **jsonOut, char **errOut) {
    @autoreleasepool {
        SCShareableContent *content = shareableContent(errOut);
        if (content == nil) return -1;
        pid_t me = getpid();
        NSMutableArray *list = [NSMutableArray array];
        for (SCWindow *w in content.windows) {
            if (w.windowLayer != 0 || !w.isOnScreen) continue; // skip menus, the Dock, overlays
            if (w.owningApplication.processID == me) continue;
            if (w.frame.size.width < 2 || w.frame.size.height < 2) continue;
            NSString *app = w.owningApplication.applicationName ?: @"";
            NSString *title = w.title ?: @"";
            if (title.length == 0 && app.length == 0) continue;
            [list addObject:@{
                @"handle" : @(w.windowID),
                @"title" : title,
                @"app" : app,
                @"width" : @((int)w.frame.size.width),
                @"height" : @((int)w.frame.size.height),
            }];
        }
        NSError *err = nil;
        NSData *json = [NSJSONSerialization dataWithJSONObject:list options:0 error:&err];
        if (json == nil) {
            *errOut = copyErr(@"failed to encode the window list", err);
            return -1;
        }
        char *buf = malloc(json.length + 1);
        memcpy(buf, json.bytes, json.length);
        buf[json.length] = 0;
        *jsonOut = buf;
        return 0;
    }
}
