//go:build darwin

#import <AVFoundation/AVFoundation.h>
#import <CoreAudio/CoreAudioTypes.h>
#import <CoreGraphics/CoreGraphics.h>
#import <CoreMedia/CoreMedia.h>
#import <Foundation/Foundation.h>
#import <ScreenCaptureKit/ScreenCaptureKit.h>
#include <stdlib.h>
#include <string.h>
#include <unistd.h>

#include "recorder_darwin.h"

// ScreenCaptureKit (SCStream) -> AVAssetWriter (VideoToolbox H.264 + AAC).
// macOS 15 adds SCRecordingOutput, which writes the file itself; the app
// supports macOS 14, so samples are appended to an AVAssetWriter here.
//
// Every timestamp is on the host clock: SCStream video and audio carry host
// time, microphone buffers are converted from the capture session's clock.
// Timeline zero (the start of the file) is the host time when the recording
// starts. Audio goes to Go (faspRecAudio), is mixed there on that timeline
// (mixer.go, shared with Windows) and comes back as PCM16 through
// fasp_rec_write_audio.

// relNs is the buffer's host time minus the recording origin.
extern void faspRecAudio(uintptr_t ctx, int source, int64_t relNs, float *data, int frames);

static const int64_t kTimeout = 5 * NSEC_PER_SEC;
static const int kAudioRate = 48000;

static char *copyMsg(NSString *msg) { return strdup(msg.UTF8String ?: "unknown error"); }

static char *copyErr(NSString *what, NSError *err) {
    if (err == nil) return copyMsg(what);
    return copyMsg([NSString stringWithFormat:@"%@: %@", what, err.localizedDescription]);
}

static BOOL waitFor(dispatch_semaphore_t sem) {
    return dispatch_semaphore_wait(sem, dispatch_time(DISPATCH_TIME_NOW, kTimeout)) == 0;
}

static int64_t toNs(CMTime t) { return CMTimeConvertScale(t, NSEC_PER_SEC, kCMTimeRoundingMethod_Default).value; }

int64_t fasp_rec_now_ns(void) { return toNs(CMClockGetTime(CMClockGetHostTimeClock())); }

// shareableContent lists displays, windows and apps. onScreenOnly limits the
// windows (and the apps that own them) to those visible now.
static SCShareableContent *shareableContent(BOOL onScreenOnly, char **errOut) {
    __block SCShareableContent *content = nil;
    __block NSError *err = nil;
    dispatch_semaphore_t sem = dispatch_semaphore_create(0);
    [SCShareableContent getShareableContentExcludingDesktopWindows:YES
                                               onScreenWindowsOnly:onScreenOnly
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

// readSample returns one sample of an LPCM buffer as float.
static float readSample(const AudioBufferList *abl, const AudioStreamBasicDescription *f, BOOL planar, int ch,
                        CMItemCount frame) {
    const AudioBuffer *b;
    size_t idx;
    if (planar) {
        b = &abl->mBuffers[ch];
        idx = (size_t)frame;
    } else {
        b = &abl->mBuffers[0];
        idx = (size_t)frame * f->mChannelsPerFrame + ch;
    }
    if (f->mFormatFlags & kAudioFormatFlagIsFloat) {
        if (f->mBitsPerChannel == 32) return ((const float *)b->mData)[idx];
        if (f->mBitsPerChannel == 64) return (float)((const double *)b->mData)[idx];
    } else if (f->mBitsPerChannel == 16) {
        return ((const int16_t *)b->mData)[idx] / 32768.0f;
    } else if (f->mBitsPerChannel == 32) {
        return ((const int32_t *)b->mData)[idx] / 2147483648.0f;
    }
    return 0;
}

@interface FaspRecorder : NSObject <SCStreamOutput, SCStreamDelegate, AVCaptureAudioDataOutputSampleBufferDelegate>
@end

@implementation FaspRecorder {
@public
    uintptr_t ctx;
    BOOL windowMode;
    SCStream *stream;
    AVCaptureSession *micSession;
    AVAssetWriter *writer;
    AVAssetWriterInput *videoIn, *audioIn;
    CMAudioFormatDescriptionRef pcmFormat;
    dispatch_queue_t queue;    // video samples and every writer call
    dispatch_queue_t sysQueue; // system audio samples
    dispatch_queue_t micQueue; // microphone samples
    int64_t originNs;
    int width, height, frames, captured, dropped, audioDropped;
    int fps;
    CMSampleBufferRef lastFrame; // newest appended frame, retained
    NSError *failure;          // writer failure
    NSError *stopError;        // why the stream stopped on its own
    BOOL selfStopped, halting;
    dispatch_semaphore_t endSem;
    NSString *sourceErr[2];
}

// markStopped records that the stream ended on its own; runs on queue.
- (void)markStopped:(NSError *)error {
    if (halting || selfStopped) return;
    selfStopped = YES;
    stopError = error;
    dispatch_semaphore_signal(endSem);
}

- (void)deliverAudio:(CMSampleBufferRef)sb source:(int)src clock:(CMClockRef)clock {
    CMFormatDescriptionRef fd = CMSampleBufferGetFormatDescription(sb);
    const AudioStreamBasicDescription *f = fd ? CMAudioFormatDescriptionGetStreamBasicDescription(fd) : NULL;
    CMItemCount n = CMSampleBufferGetNumSamples(sb);
    if (f == NULL || n <= 0) return;
    if (f->mFormatID != kAudioFormatLinearPCM || f->mChannelsPerFrame < 1 || (int)f->mSampleRate != kAudioRate) {
        @synchronized(self) {
            if (sourceErr[src] == nil) {
                sourceErr[src] = [NSString stringWithFormat:@"unsupported audio format (%.0f Hz, %u channels)",
                                                            f->mSampleRate, (unsigned)f->mChannelsPerFrame];
            }
        }
        return;
    }
    BOOL planar = (f->mFormatFlags & kAudioFormatFlagIsNonInterleaved) != 0;
    size_t size = 0;
    if (CMSampleBufferGetAudioBufferListWithRetainedBlockBuffer(sb, &size, NULL, 0, NULL, NULL, 0, NULL) != noErr) return;
    AudioBufferList *abl = malloc(size);
    CMBlockBufferRef block = NULL;
    OSStatus st = CMSampleBufferGetAudioBufferListWithRetainedBlockBuffer(
        sb, NULL, abl, size, NULL, NULL, kCMSampleBufferFlag_AudioBufferList_Assure16ByteAlignment, &block);
    if (st != noErr) {
        free(abl);
        return;
    }
    int chans = (int)f->mChannelsPerFrame;
    float *out = malloc((size_t)n * 2 * sizeof(float));
    for (CMItemCount i = 0; i < n; i++) {
        out[i * 2] = readSample(abl, f, planar, 0, i);
        out[i * 2 + 1] = readSample(abl, f, planar, chans > 1 ? 1 : 0, i); // mono -> both sides
    }
    if (block) CFRelease(block);
    free(abl);

    CMTime pts = CMSampleBufferGetPresentationTimeStamp(sb);
    if (clock != NULL) pts = CMSyncConvertTime(pts, clock, CMClockGetHostTimeClock());
    faspRecAudio(ctx, src, toNs(pts) - originNs, out, (int)n);
    free(out);
}

- (void)stream:(SCStream *)s didOutputSampleBuffer:(CMSampleBufferRef)sb ofType:(SCStreamOutputType)type {
    if (!CMSampleBufferIsValid(sb)) return;
    if (type == SCStreamOutputTypeAudio) {
        [self deliverAudio:sb source:FASP_SRC_SYSTEM clock:NULL];
        return;
    }
    if (type != SCStreamOutputTypeScreen || failure != nil) return;
    // SCStream also sends status buffers without an image: "idle" when the
    // screen does not change, "stopped" when the recorded window is gone.
    CFArrayRef atts = CMSampleBufferGetSampleAttachmentsArray(sb, false);
    if (atts == NULL || CFArrayGetCount(atts) == 0) return;
    NSDictionary *info = (__bridge NSDictionary *)CFArrayGetValueAtIndex(atts, 0);
    NSNumber *status = info[SCStreamFrameInfoStatus];
    if (status != nil && status.integerValue == SCFrameStatusStopped) {
        [self markStopped:nil];
        return;
    }
    if (status == nil || status.integerValue != SCFrameStatusComplete || selfStopped || halting) return;
    captured++;
    if (!videoIn.isReadyForMoreMediaData) {
        dropped++;
        return;
    }
    // The first frame arrives a little after the file starts; show it from
    // the start rather than leaving the head of the video empty.
    CMSampleBufferRef out = frames == 0 ? [self retime:sb at:CMTimeMake(originNs, NSEC_PER_SEC)] : sb;
    if (out == NULL) return;
    if ([videoIn appendSampleBuffer:out]) {
        frames++;
        if (lastFrame) CFRelease(lastFrame);
        lastFrame = (CMSampleBufferRef)CFRetain(out);
    } else {
        failure = writer.error;
    }
    if (out != sb) CFRelease(out);
}

// retime returns a copy of a video sample at another presentation time
// (+1 retained), or NULL.
- (CMSampleBufferRef)retime:(CMSampleBufferRef)sb at:(CMTime)pts {
    CMSampleTimingInfo timing = {.duration = kCMTimeInvalid, .presentationTimeStamp = pts,
                                 .decodeTimeStamp = kCMTimeInvalid};
    CMSampleBufferRef out = NULL;
    if (CMSampleBufferCreateCopyWithNewTiming(kCFAllocatorDefault, sb, 1, &timing, &out) != noErr) return NULL;
    return out;
}

// holdLastFrame repeats the newest frame at pts, so a screen that stopped
// changing still fills the video up to the end. Runs on queue.
- (void)holdLastFrame:(CMTime)pts {
    if (lastFrame == NULL || failure != nil) return;
    if (CMTimeCompare(pts, CMSampleBufferGetPresentationTimeStamp(lastFrame)) <= 0) return;
    CMSampleBufferRef out = [self retime:lastFrame at:pts];
    if (out == NULL) return;
    if (videoIn.isReadyForMoreMediaData && [videoIn appendSampleBuffer:out]) frames++;
    CFRelease(out);
}

- (void)dealloc {
    if (lastFrame) CFRelease(lastFrame);
}

- (void)stream:(SCStream *)s didStopWithError:(NSError *)error {
    // Fires when the window closes, the display goes away or the user
    // revokes the permission. Recorded on the sample queue to stay ordered.
    dispatch_async(queue, ^{ [self markStopped:error]; });
}

- (void)captureOutput:(AVCaptureOutput *)output
    didOutputSampleBuffer:(CMSampleBufferRef)sb
           fromConnection:(AVCaptureConnection *)connection {
    [self deliverAudio:sb source:FASP_SRC_MIC clock:micSession.synchronizationClock];
}
@end

static AVCaptureDevice *microphone(const char *micID) {
    if (micID != NULL && micID[0] != 0) {
        AVCaptureDevice *d = [AVCaptureDevice deviceWithUniqueID:[NSString stringWithUTF8String:micID]];
        if (d != nil) return d;
    }
    return [AVCaptureDevice defaultDeviceWithMediaType:AVMediaTypeAudio];
}

// requireMicPermission asks once (the system prompt) and reports a refusal.
static BOOL requireMicPermission(char **errOut) {
    AVAuthorizationStatus s = [AVCaptureDevice authorizationStatusForMediaType:AVMediaTypeAudio];
    if (s == AVAuthorizationStatusNotDetermined) {
        __block BOOL granted = NO;
        dispatch_semaphore_t sem = dispatch_semaphore_create(0);
        [AVCaptureDevice requestAccessForMediaType:AVMediaTypeAudio
                                 completionHandler:^(BOOL ok) {
            granted = ok;
            dispatch_semaphore_signal(sem);
        }];
        dispatch_semaphore_wait(sem, dispatch_time(DISPATCH_TIME_NOW, 120 * NSEC_PER_SEC));
        if (granted) return YES;
    } else if (s == AVAuthorizationStatusAuthorized) {
        return YES;
    }
    *errOut = copyMsg(@"microphone permission is required. Allow Fasp in System Settings > Privacy & Security > "
                      @"Microphone");
    return NO;
}

static AVCaptureSession *startMic(FaspRecorder *rec, const char *micID, char **errOut) {
    if (!requireMicPermission(errOut)) return nil;
    AVCaptureDevice *dev = microphone(micID);
    if (dev == nil) {
        *errOut = copyMsg(@"no microphone found");
        return nil;
    }
    NSError *err = nil;
    AVCaptureDeviceInput *in = [AVCaptureDeviceInput deviceInputWithDevice:dev error:&err];
    if (in == nil) {
        *errOut = copyErr(@"failed to open the microphone", err);
        return nil;
    }
    AVCaptureSession *session = [[AVCaptureSession alloc] init];
    AVCaptureAudioDataOutput *out = [[AVCaptureAudioDataOutput alloc] init];
    // Let the session convert any device to the mixer's format.
    out.audioSettings = @{
        AVFormatIDKey : @(kAudioFormatLinearPCM),
        AVSampleRateKey : @(kAudioRate),
        AVNumberOfChannelsKey : @2,
        AVLinearPCMBitDepthKey : @32,
        AVLinearPCMIsFloatKey : @YES,
        AVLinearPCMIsNonInterleaved : @NO,
        AVLinearPCMIsBigEndianKey : @NO,
    };
    [out setSampleBufferDelegate:rec queue:rec->micQueue];
    [session beginConfiguration];
    if (![session canAddInput:in] || ![session canAddOutput:out]) {
        [session commitConfiguration];
        *errOut = copyMsg(@"the microphone cannot be recorded");
        return nil;
    }
    [session addInput:in];
    [session addOutput:out];
    [session commitConfiguration];
    rec->micSession = session; // the delegate reads its clock
    [session startRunning];
    if (!session.isRunning) {
        rec->micSession = nil;
        *errOut = copyMsg(@"failed to start the microphone");
        return nil;
    }
    return session;
}

static void stopStream(FaspRecorder *rec) {
    dispatch_semaphore_t sem = dispatch_semaphore_create(0);
    [rec->stream stopCaptureWithCompletionHandler:^(NSError *e) { dispatch_semaphore_signal(sem); }];
    waitFor(sem);
}

// abandon releases a recorder that failed to start.
static void *abandon(FaspRecorder *rec, BOOL streamStarted) {
    if (rec->micSession != nil) [rec->micSession stopRunning];
    if (streamStarted) stopStream(rec);
    if (rec->writer.status == AVAssetWriterStatusWriting) [rec->writer cancelWriting];
    if (rec->pcmFormat) CFRelease(rec->pcmFormat);
    return NULL;
}

void *fasp_rec_start(const fasp_rec_opts *o, int64_t *originNs, char **errOut) {
    @autoreleasepool {
        // All windows: Fasp's own app must be found even while its window is
        // hidden, to exclude it.
        SCShareableContent *content = shareableContent(NO, errOut);
        if (content == nil) return NULL;

        // Never record Fasp's own windows. Excluding the application (not the
        // windows listed now) also covers windows shown later, such as the
        // recording bar that appears after the region overlay hid the app.
        NSMutableArray<SCRunningApplication *> *own = [NSMutableArray array];
        pid_t me = getpid();
        for (SCRunningApplication *a in content.applications) {
            if (a.processID == me) [own addObject:a];
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
            CGPoint at = o->mode == FASP_REC_REGION ? CGPointMake(o->x + o->w / 2, o->y + o->h / 2) : cursorLocation();
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
            filter = [[SCContentFilter alloc] initWithDisplay:display excludingApplications:own exceptingWindows:@[]];
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
        if (o->systemAudio) {
            config.capturesAudio = YES;
            config.excludesCurrentProcessAudio = YES; // not the app's own player
            config.sampleRate = kAudioRate;
            config.channelCount = 2;
        }

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
        AVAssetWriterInput *videoIn = [AVAssetWriterInput assetWriterInputWithMediaType:AVMediaTypeVideo
                                                                         outputSettings:settings];
        videoIn.expectsMediaDataInRealTime = YES;
        if (![writer canAddInput:videoIn]) {
            *errOut = copyMsg(@"the H.264 encoder rejected the video settings");
            return NULL;
        }
        [writer addInput:videoIn];

        FaspRecorder *rec = [[FaspRecorder alloc] init];
        rec->ctx = o->ctx;
        rec->windowMode = o->mode == FASP_REC_WINDOW;
        rec->writer = writer;
        rec->videoIn = videoIn;
        rec->width = width;
        rec->height = height;
        rec->fps = o->fps;
        rec->endSem = dispatch_semaphore_create(0);
        rec->queue = dispatch_queue_create("fasp.recorder", DISPATCH_QUEUE_SERIAL);
        rec->sysQueue = dispatch_queue_create("fasp.recorder.system-audio", DISPATCH_QUEUE_SERIAL);
        rec->micQueue = dispatch_queue_create("fasp.recorder.microphone", DISPATCH_QUEUE_SERIAL);

        if (o->systemAudio || o->microphone) {
            // The mixer hands back 48 kHz stereo PCM16, encoded to one AAC track.
            AudioStreamBasicDescription pcm = {
                .mSampleRate = kAudioRate,
                .mFormatID = kAudioFormatLinearPCM,
                .mFormatFlags = kAudioFormatFlagIsSignedInteger | kAudioFormatFlagIsPacked,
                .mBytesPerPacket = 4,
                .mFramesPerPacket = 1,
                .mBytesPerFrame = 4,
                .mChannelsPerFrame = 2,
                .mBitsPerChannel = 16,
            };
            if (CMAudioFormatDescriptionCreate(kCFAllocatorDefault, &pcm, 0, NULL, 0, NULL, NULL, &rec->pcmFormat) !=
                noErr) {
                *errOut = copyMsg(@"failed to describe the audio format");
                return NULL;
            }
            AudioChannelLayout layout = {.mChannelLayoutTag = kAudioChannelLayoutTag_Stereo};
            NSDictionary *aac = @{
                AVFormatIDKey : @(kAudioFormatMPEG4AAC),
                AVSampleRateKey : @(kAudioRate),
                AVNumberOfChannelsKey : @2,
                AVEncoderBitRateKey : @160000,
                AVChannelLayoutKey : [NSData dataWithBytes:&layout length:sizeof(layout)],
            };
            AVAssetWriterInput *audioIn = [AVAssetWriterInput assetWriterInputWithMediaType:AVMediaTypeAudio
                                                                             outputSettings:aac
                                                                           sourceFormatHint:rec->pcmFormat];
            audioIn.expectsMediaDataInRealTime = YES;
            if (![writer canAddInput:audioIn]) {
                *errOut = copyMsg(@"the AAC encoder rejected the audio settings");
                return abandon(rec, NO);
            }
            [writer addInput:audioIn];
            rec->audioIn = audioIn;
        }

        rec->stream = [[SCStream alloc] initWithFilter:filter configuration:config delegate:rec];
        if (![rec->stream addStreamOutput:rec type:SCStreamOutputTypeScreen sampleHandlerQueue:rec->queue error:&err]) {
            *errOut = copyErr(@"failed to add the stream output", err);
            return abandon(rec, NO);
        }
        if (o->systemAudio &&
            ![rec->stream addStreamOutput:rec type:SCStreamOutputTypeAudio sampleHandlerQueue:rec->sysQueue error:&err]) {
            *errOut = copyErr(@"failed to add the system audio output", err);
            return abandon(rec, NO);
        }

        // The file starts now; every sample is placed relative to this time.
        if (![writer startWriting]) {
            *errOut = copyErr(@"failed to start the video file", writer.error);
            return abandon(rec, NO);
        }
        rec->originNs = fasp_rec_now_ns();
        [writer startSessionAtSourceTime:CMTimeMake(rec->originNs, NSEC_PER_SEC)];

        if (o->microphone && startMic(rec, o->micID, errOut) == nil) return abandon(rec, NO);

        __block NSError *startErr = nil;
        dispatch_semaphore_t sem = dispatch_semaphore_create(0);
        [rec->stream startCaptureWithCompletionHandler:^(NSError *e) {
            startErr = e;
            dispatch_semaphore_signal(sem);
        }];
        if (!waitFor(sem)) {
            *errOut = copyMsg(@"timed out starting the capture");
            return abandon(rec, YES);
        }
        if (startErr != nil) {
            *errOut = copyErr(@"failed to start the capture", startErr);
            return abandon(rec, NO);
        }
        *originNs = rec->originNs;
        return (__bridge_retained void *)rec;
    }
}

int fasp_rec_wait_end(void *handle) {
    FaspRecorder *rec = (__bridge FaspRecorder *)handle;
    dispatch_semaphore_wait(rec->endSem, DISPATCH_TIME_FOREVER);
    __block BOOL stopped = NO;
    dispatch_sync(rec->queue, ^{ stopped = rec->selfStopped; });
    return stopped ? 1 : 0;
}

void fasp_rec_wake(void *handle) {
    FaspRecorder *rec = (__bridge FaspRecorder *)handle;
    dispatch_semaphore_signal(rec->endSem);
}

int fasp_rec_write_audio(void *handle, const int16_t *pcm, int frames, int64_t startFrame, char **errOut) {
    @autoreleasepool {
        FaspRecorder *rec = (__bridge FaspRecorder *)handle;
        if (rec->audioIn == nil || frames <= 0) return 0;
        size_t bytes = (size_t)frames * 4;
        CMBlockBufferRef block = NULL;
        OSStatus st = CMBlockBufferCreateWithMemoryBlock(kCFAllocatorDefault, NULL, bytes, kCFAllocatorDefault, NULL, 0,
                                                         bytes, kCMBlockBufferAssureMemoryNowFlag, &block);
        if (st == noErr) st = CMBlockBufferReplaceDataBytes(pcm, block, 0, bytes);
        CMSampleBufferRef sb = NULL;
        if (st == noErr) {
            // Timeline frame -> host time, at audio-sample precision.
            CMTime pts = CMTimeAdd(CMTimeMake(rec->originNs, NSEC_PER_SEC), CMTimeMake(startFrame, kAudioRate));
            st = CMAudioSampleBufferCreateReadyWithPacketDescriptions(kCFAllocatorDefault, block, rec->pcmFormat,
                                                                      frames, pts, NULL, &sb);
        }
        if (block) CFRelease(block);
        if (st != noErr) {
            *errOut = copyMsg([NSString stringWithFormat:@"failed to build an audio buffer (%d)", (int)st]);
            return -1;
        }
        __block int rc = 0;
        dispatch_sync(rec->queue, ^{
            if (rec->failure != nil) return;
            if (!rec->audioIn.isReadyForMoreMediaData) {
                rec->audioDropped++;
                return;
            }
            if (![rec->audioIn appendSampleBuffer:sb]) {
                rec->failure = rec->writer.error;
                rc = -1;
            }
        });
        CFRelease(sb);
        if (rc != 0) *errOut = copyErr(@"failed to write audio", rec->failure);
        return rc;
    }
}

void fasp_rec_halt(void *handle, int64_t *endNs) {
    @autoreleasepool {
        FaspRecorder *rec = (__bridge FaspRecorder *)handle;
        dispatch_sync(rec->queue, ^{ rec->halting = YES; });
        *endNs = fasp_rec_now_ns();
        stopStream(rec);
        if (rec->micSession != nil) [rec->micSession stopRunning];
        // Let in-flight buffers finish: after this no audio callback runs.
        dispatch_sync(rec->queue, ^{});
        dispatch_sync(rec->sysQueue, ^{});
        dispatch_sync(rec->micQueue, ^{});
    }
}

char *fasp_rec_source_error(void *handle, int source) {
    FaspRecorder *rec = (__bridge FaspRecorder *)handle;
    @synchronized(rec) {
        NSString *e = rec->sourceErr[source];
        return e ? copyMsg(e) : NULL;
    }
}

int fasp_rec_finish(void *handle, int64_t endNs, fasp_rec_stats *st, char **errOut) {
    @autoreleasepool {
        FaspRecorder *rec = (__bridge_transfer FaspRecorder *)handle;
        st->width = rec->width;
        st->height = rec->height;
        st->frames = rec->frames;
        st->captured = rec->captured;
        st->dropped = rec->dropped;
        st->audioDropped = rec->audioDropped;
        // A window recording that ended on its own: the window was closed.
        // The file is complete up to that moment.
        st->windowClosed = rec->selfStopped && rec->windowMode;
        if (rec->pcmFormat) CFRelease(rec->pcmFormat);

        if (rec->frames == 0) {
            [rec->writer cancelWriting];
            *errOut = rec->failure ? copyErr(@"recording failed", rec->failure) : copyMsg(@"no frames were captured");
            return -1;
        }
        // SCStream only delivers frames when something changes, so the file
        // has a variable frame rate. Repeat the last frame one frame before
        // the end, so the video track lasts until the recording stopped.
        CMTime end = CMTimeMake(endNs, NSEC_PER_SEC);
        dispatch_sync(rec->queue, ^{ [rec holdLastFrame:CMTimeSubtract(end, CMTimeMake(1, rec->fps))]; });
        st->frames = rec->frames;
        [rec->writer endSessionAtSourceTime:end];
        [rec->videoIn markAsFinished];
        [rec->audioIn markAsFinished];
        dispatch_semaphore_t sem = dispatch_semaphore_create(0);
        [rec->writer finishWritingWithCompletionHandler:^{ dispatch_semaphore_signal(sem); }];
        if (!waitFor(sem)) {
            *errOut = copyMsg(@"timed out finishing the video file");
            return -1;
        }
        if (rec->writer.status != AVAssetWriterStatusCompleted) {
            *errOut = copyErr(@"failed to write the video file", rec->writer.error);
            return -1;
        }
        if (rec->selfStopped && !rec->windowMode) {
            // The display went away or the permission was revoked; the file
            // is valid up to that moment.
            *errOut = copyErr(@"recording stopped early", rec->stopError);
            return -1;
        }
        return 0;
    }
}

int fasp_rec_list_windows(char **jsonOut, char **errOut) {
    @autoreleasepool {
        SCShareableContent *content = shareableContent(YES, errOut);
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

int fasp_rec_list_mics(char **jsonOut, char **errOut) {
    @autoreleasepool {
        AVCaptureDeviceDiscoverySession *ds = [AVCaptureDeviceDiscoverySession
            discoverySessionWithDeviceTypes:@[ AVCaptureDeviceTypeMicrophone, AVCaptureDeviceTypeExternal ]
                                  mediaType:AVMediaTypeAudio
                                   position:AVCaptureDevicePositionUnspecified];
        NSMutableArray *list = [NSMutableArray array];
        for (AVCaptureDevice *d in ds.devices) {
            [list addObject:@{@"id" : d.uniqueID ?: @"", @"name" : d.localizedName ?: @""}];
        }
        NSError *err = nil;
        NSData *json = [NSJSONSerialization dataWithJSONObject:list options:0 error:&err];
        if (json == nil) {
            *errOut = copyErr(@"failed to encode the microphone list", err);
            return -1;
        }
        char *buf = malloc(json.length + 1);
        memcpy(buf, json.bytes, json.length);
        buf[json.length] = 0;
        *jsonOut = buf;
        return 0;
    }
}
