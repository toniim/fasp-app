//go:build darwin

#import <CoreGraphics/CoreGraphics.h>
#import <Foundation/Foundation.h>
#import <ImageIO/ImageIO.h>
#import <Vision/Vision.h>
#include <stdlib.h>
#include <string.h>

#include "ocr_darwin.h"

static char *copyMessage(NSString *msg) {
    return strdup(msg.UTF8String ?: "unknown error");
}

static char *copyError(NSString *what, NSError *err) {
    if (err == nil) return copyMessage(what);
    return copyMessage([NSString stringWithFormat:@"%@: %@", what, err.localizedDescription]);
}

static VNRecognizeTextRequest *newRequest(void) {
    VNRecognizeTextRequest *req = [[VNRecognizeTextRequest alloc] init];
    req.recognitionLevel = VNRequestTextRecognitionLevelAccurate;
    req.usesLanguageCorrection = YES;
    return req;
}

// matchSupported maps a requested tag ("vi-VN") to one Vision supports
// ("vi-VT"): exact match first (case-insensitive), then the same primary
// language subtag. Returns nil when nothing fits.
static NSString *matchSupported(NSString *want, NSArray<NSString *> *supported) {
    for (NSString *s in supported) {
        if ([s caseInsensitiveCompare:want] == NSOrderedSame) return s;
    }
    NSString *primary = [[want componentsSeparatedByString:@"-"] firstObject];
    for (NSString *s in supported) {
        NSString *sp = [[s componentsSeparatedByString:@"-"] firstObject];
        if ([sp caseInsensitiveCompare:primary] == NSOrderedSame) return s;
    }
    return nil;
}

int fasp_ocr_recognize(const void *data, int size, const char *languages,
                       fasp_ocr_line **outLines, int *outCount,
                       int *outWidth, int *outHeight, char **errOut) {
    @autoreleasepool {
        CFDataRef cfData = CFDataCreate(kCFAllocatorDefault, data, size);
        if (cfData == NULL) {
            *errOut = copyMessage(@"out of memory");
            return -1;
        }
        CGImageSourceRef src = CGImageSourceCreateWithData(cfData, NULL);
        CFRelease(cfData);
        if (src == NULL) {
            *errOut = copyMessage(@"cannot decode image");
            return -1;
        }
        CGImageRef image = CGImageSourceCreateImageAtIndex(src, 0, NULL);
        CFRelease(src);
        if (image == NULL) {
            *errOut = copyMessage(@"cannot decode image");
            return -1;
        }
        *outWidth = (int)CGImageGetWidth(image);
        *outHeight = (int)CGImageGetHeight(image);

        VNRecognizeTextRequest *req = newRequest();

        NSString *langStr = languages ? [NSString stringWithUTF8String:languages] : @"";
        NSMutableArray<NSString *> *wanted = [NSMutableArray array];
        for (NSString *l in [langStr componentsSeparatedByString:@","]) {
            NSString *t = [l stringByTrimmingCharactersInSet:NSCharacterSet.whitespaceCharacterSet];
            if (t.length > 0) [wanted addObject:t];
        }
        if (wanted.count > 0) {
            NSError *langErr = nil;
            NSArray<NSString *> *supported = [req supportedRecognitionLanguagesAndReturnError:&langErr];
            NSMutableArray<NSString *> *use = [NSMutableArray array];
            for (NSString *w in wanted) {
                NSString *m = matchSupported(w, supported ?: @[]);
                if (m != nil && ![use containsObject:m]) [use addObject:m];
            }
            // Unsupported languages are ignored rather than failing: the engine
            // then falls back to its default set.
            if (use.count > 0) req.recognitionLanguages = use;
        } else if (@available(macOS 13.0, *)) {
            req.automaticallyDetectsLanguage = YES;
        }

        VNImageRequestHandler *handler = [[VNImageRequestHandler alloc] initWithCGImage:image options:@{}];
        NSError *err = nil;
        BOOL ok = [handler performRequests:@[req] error:&err];
        CGImageRelease(image);
        if (!ok) {
            *errOut = copyError(@"text recognition failed", err);
            return -1;
        }

        NSArray<VNRecognizedTextObservation *> *obs = req.results ?: @[];
        fasp_ocr_line *lines = calloc(obs.count > 0 ? obs.count : 1, sizeof(fasp_ocr_line));
        if (lines == NULL) {
            *errOut = copyMessage(@"out of memory");
            return -1;
        }
        int n = 0;
        for (VNRecognizedTextObservation *o in obs) {
            VNRecognizedText *top = [[o topCandidates:1] firstObject];
            if (top == nil) continue;
            lines[n].text = strdup(top.string.UTF8String ?: "");
            lines[n].x = o.boundingBox.origin.x;
            lines[n].y = o.boundingBox.origin.y;
            lines[n].w = o.boundingBox.size.width;
            lines[n].h = o.boundingBox.size.height;
            n++;
        }
        *outLines = lines;
        *outCount = n;
        return 0;
    }
}

void fasp_ocr_free_lines(fasp_ocr_line *lines, int count) {
    if (lines == NULL) return;
    for (int i = 0; i < count; i++) free(lines[i].text);
    free(lines);
}

int fasp_ocr_supported_languages(char **out, char **errOut) {
    @autoreleasepool {
        VNRecognizeTextRequest *req = newRequest();
        NSError *err = nil;
        NSArray<NSString *> *langs = [req supportedRecognitionLanguagesAndReturnError:&err];
        if (langs == nil) {
            *errOut = copyError(@"cannot list languages", err);
            return -1;
        }
        *out = copyMessage([langs componentsJoinedByString:@"\n"]);
        return 0;
    }
}
