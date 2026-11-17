//go:build darwin

#import <Accelerate/Accelerate.h>
#import <AVFoundation/AVFoundation.h>
#import <CoreMedia/CoreMedia.h>
#import <CoreVideo/CoreVideo.h>
#import <Foundation/Foundation.h>
#include <stdlib.h>
#include <string.h>

#include "media_darwin.h"

// AVAssetReader decodes frames for the GIF export; AVAssetExportSession cuts
// the trims. The synchronous AVAsset accessors (tracks, duration) are used on
// purpose: the files are local, and the async loaders need a run loop for no
// gain. They are deprecated on the newest SDK but still work on macOS 14+.
#pragma clang diagnostic ignored "-Wdeprecated-declarations"

extern int faspMediaFrame(uintptr_t ctx, int idx, uint8_t *data, int w, int h);

static char *copyMsg(NSString *msg) { return strdup(msg.UTF8String ?: "unknown error"); }

static char *copyErr(NSString *what, NSError *err) {
    if (err == nil) return copyMsg(what);
    return copyMsg([NSString stringWithFormat:@"%@: %@", what, err.localizedDescription]);
}

static AVURLAsset *openAsset(const char *path) {
    NSURL *url = [NSURL fileURLWithPath:[NSString stringWithUTF8String:path]];
    return [AVURLAsset URLAssetWithURL:url options:nil];
}

int fasp_media_probe(const char *path, double *durationSec, int *w, int *h, char **errOut) {
    @autoreleasepool {
        AVURLAsset *asset = openAsset(path);
        if (!asset.isReadable) {
            *errOut = copyMsg(@"cannot read the media file");
            return -1;
        }
        NSArray<AVAssetTrack *> *video = [asset tracksWithMediaType:AVMediaTypeVideo];
        if (video.count == 0) {
            *errOut = copyMsg(@"no video track");
            return -1;
        }
        Float64 d = CMTimeGetSeconds(asset.duration);
        if (!isfinite(d) || d <= 0) {
            *errOut = copyMsg(@"unknown duration");
            return -1;
        }
        CGSize s = video[0].naturalSize;
        *durationSec = d;
        *w = (int)lround(s.width);
        *h = (int)lround(s.height);
        return 0;
    }
}

int fasp_media_decode(const char *path, int64_t startNs, int count, double fps, int outW, int outH,
                      uintptr_t ctx, char **errOut) {
    @autoreleasepool {
        AVURLAsset *asset = openAsset(path);
        AVAssetTrack *track = [asset tracksWithMediaType:AVMediaTypeVideo].firstObject;
        if (track == nil) {
            *errOut = copyMsg(@"no video track");
            return -1;
        }
        NSError *err = nil;
        AVAssetReader *reader = [AVAssetReader assetReaderWithAsset:asset error:&err];
        if (reader == nil) {
            *errOut = copyErr(@"open reader", err);
            return -1;
        }
        AVAssetReaderTrackOutput *out = [AVAssetReaderTrackOutput
            assetReaderTrackOutputWithTrack:track
                             outputSettings:@{(id)kCVPixelBufferPixelFormatTypeKey : @(kCVPixelFormatType_32BGRA)}];
        out.alwaysCopiesSampleData = NO;
        [reader addOutput:out];
        // The reader starts at the keyframe before startNs and drops what it
        // decodes before the range, so a mid-GOP start costs one GOP of decode.
        reader.timeRange = CMTimeRangeMake(CMTimeMake(startNs, 1000000000), kCMTimePositiveInfinity);
        if (![reader startReading]) {
            *errOut = copyErr(@"start reading", reader.error);
            return -1;
        }

        uint8_t *buf = malloc((size_t)outW * outH * 4);
        if (buf == NULL) {
            [reader cancelReading];
            *errOut = copyMsg(@"out of memory");
            return -1;
        }

        // Screen recordings are variable frame rate: a still screen has few
        // frames. Each sample time takes the last frame at or before it, so a
        // still screen repeats its frame, and the frame after is read ahead
        // to know where the current one ends.
        const double t0 = (double)startNs / 1e9, step = 1.0 / fps;
        CMSampleBufferRef cur = NULL, next = [out copyNextSampleBuffer];
        int rc = 0;
        for (int k = 0; k < count && rc == 0; k++) {
            double t = t0 + k * step;
            while (next != NULL && CMTimeGetSeconds(CMSampleBufferGetPresentationTimeStamp(next)) <= t + 0.0005) {
                if (cur != NULL) CFRelease(cur);
                cur = next;
                next = [out copyNextSampleBuffer];
            }
            if (cur == NULL) { // the first frame comes after the requested start
                cur = next;
                next = cur != NULL ? [out copyNextSampleBuffer] : NULL;
            }
            CVImageBufferRef img = cur != NULL ? CMSampleBufferGetImageBuffer(cur) : NULL;
            if (img == NULL) {
                *errOut = copyErr(@"no video frame to read", reader.error);
                rc = -1;
                break;
            }
            CVPixelBufferLockBaseAddress(img, kCVPixelBufferLock_ReadOnly);
            vImage_Buffer src = {CVPixelBufferGetBaseAddress(img), CVPixelBufferGetHeight(img),
                                 CVPixelBufferGetWidth(img), CVPixelBufferGetBytesPerRow(img)};
            vImage_Buffer dst = {buf, (vImagePixelCount)outH, (vImagePixelCount)outW, (size_t)outW * 4};
            vImage_Error ve = vImageScale_ARGB8888(&src, &dst, NULL, kvImageHighQualityResampling);
            CVPixelBufferUnlockBaseAddress(img, kCVPixelBufferLock_ReadOnly);
            if (ve != kvImageNoError) {
                *errOut = copyMsg([NSString stringWithFormat:@"scale frame: vImage error %ld", (long)ve]);
                rc = -1;
                break;
            }
            if (faspMediaFrame(ctx, k, buf, outW, outH) != 0) rc = 1;
        }
        if (cur != NULL) CFRelease(cur);
        if (next != NULL) CFRelease(next);
        free(buf);
        [reader cancelReading];
        return rc;
    }
}

// startOnSyncSample reports whether a cut at startNs begins exactly on a
// keyframe. Only then can the video be copied without re-encoding.
static BOOL startOnSyncSample(AVURLAsset *asset, int64_t startNs) {
    AVAssetTrack *video = [asset tracksWithMediaType:AVMediaTypeVideo].firstObject;
    if (video == nil) return NO;
    AVSampleCursor *c = [video makeSampleCursorWithPresentationTimeStamp:CMTimeMake(startNs, 1000000000)];
    if (c == nil) return NO;
    double dt = fabs(CMTimeGetSeconds(c.presentationTimeStamp) - (double)startNs / 1e9);
    return dt < 0.0005 && c.currentSampleSyncInfo.sampleIsFullSync;
}

static BOOL runExport(AVURLAsset *asset, NSString *preset, NSURL *outURL, CMTimeRange range, NSString **why) {
    if (![[AVAssetExportSession exportPresetsCompatibleWithAsset:asset] containsObject:preset]) {
        *why = [NSString stringWithFormat:@"export preset %@ is not available for this file", preset];
        return NO;
    }
    [[NSFileManager defaultManager] removeItemAtURL:outURL error:nil];
    AVAssetExportSession *s = [[AVAssetExportSession alloc] initWithAsset:asset presetName:preset];
    if (s == nil) {
        *why = @"cannot create the export session";
        return NO;
    }
    s.outputURL = outURL;
    s.outputFileType = AVFileTypeMPEG4;
    s.timeRange = range;
    dispatch_semaphore_t sem = dispatch_semaphore_create(0);
    [s exportAsynchronouslyWithCompletionHandler:^{
        dispatch_semaphore_signal(sem);
    }];
    dispatch_semaphore_wait(sem, DISPATCH_TIME_FOREVER);
    if (s.status != AVAssetExportSessionStatusCompleted) {
        *why = [NSString stringWithFormat:@"export: %@", s.error.localizedDescription ?: @"failed"];
        return NO;
    }
    return YES;
}

int fasp_media_trim(const char *in, const char *out, int64_t startNs, int64_t endNs, char **errOut) {
    @autoreleasepool {
        AVURLAsset *asset = openAsset(in);
        NSURL *outURL = [NSURL fileURLWithPath:[NSString stringWithUTF8String:out]];
        CMTimeRange range = CMTimeRangeFromTimeToTime(CMTimeMake(startNs, 1000000000), CMTimeMake(endNs, 1000000000));
        NSString *why = nil;

        // Passthrough keeps the quality and takes no time, but it can only
        // start on a keyframe; any other start is re-encoded to stay exact.
        if (startOnSyncSample(asset, startNs) &&
            runExport(asset, AVAssetExportPresetPassthrough, outURL, range, &why)) {
            return 0;
        }
        if (runExport(asset, AVAssetExportPresetHighestQuality, outURL, range, &why)) return 0;
        *errOut = copyMsg(why ?: @"export failed");
        return -1;
    }
}
