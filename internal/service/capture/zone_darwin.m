//go:build darwin

#import <AppKit/AppKit.h>
#import <CoreGraphics/CoreGraphics.h>
#import <Foundation/Foundation.h>
#include <math.h>
#include <stdlib.h>
#include <string.h>

#include "capture_darwin.h"

// Freeze-zone overlay: the display under the cursor is captured first, then
// shown frozen and dimmed in a borderless window above everything. The user
// drags a rectangle that is shown undimmed. All AppKit work happens on the
// main thread; fasp_zone_select blocks its (non-main) caller until the user
// finishes.
//
// The view is flipped, so every coordinate below is in points with the origin
// at the top-left of the display (the same orientation as Quartz global
// space, which is why no bottom-left conversion is needed).

typedef void (^ZoneFinish)(int status, NSRect sel);

@interface FaspZoneWindow : NSWindow
@end

@implementation FaspZoneWindow
// A borderless window refuses key status by default, and Esc/Enter need it.
- (BOOL)canBecomeKeyWindow { return YES; }
- (BOOL)canBecomeMainWindow { return YES; }
@end

@interface FaspZoneView : NSView
@property(nonatomic, strong) NSImage *image;
@property(nonatomic) CGFloat scale; // image pixels per point
@property(nonatomic) int minPixels;
@property(nonatomic, copy) ZoneFinish finish;
// Windows on screen when it was frozen, front-most first, as rects in view
// points clipped to the display. A click picks the one under the cursor.
@property(nonatomic, strong) NSArray<NSValue *> *windowRects;
@end

@implementation FaspZoneView {
    BOOL _dragging;
    BOOL _finished;
    BOOL _hasMouse;
    NSPoint _start;
    NSPoint _cur;
    NSPoint _mouse;
}

- (BOOL)isFlipped { return YES; }
- (BOOL)acceptsFirstResponder { return YES; }
- (BOOL)acceptsFirstMouse:(NSEvent *)event { return YES; }

- (void)resetCursorRects {
    [self addCursorRect:self.bounds cursor:[NSCursor crosshairCursor]];
}

- (void)updateTrackingAreas {
    for (NSTrackingArea *a in self.trackingAreas) [self removeTrackingArea:a];
    [self addTrackingArea:[[NSTrackingArea alloc]
        initWithRect:NSZeroRect
             options:NSTrackingMouseMoved | NSTrackingActiveAlways | NSTrackingInVisibleRect
               owner:self
            userInfo:nil]];
    [super updateTrackingAreas];
}

- (BOOL)largeEnough:(NSRect)r {
    return r.size.width * self.scale >= self.minPixels && r.size.height * self.scale >= self.minPixels;
}

// windowAt returns the front-most window under p, or NSZeroRect when there is
// none or it is too small to capture (a window behind it is not chosen: it is
// not what the user sees at p).
- (NSRect)windowAt:(NSPoint)p {
    for (NSValue *v in self.windowRects) {
        NSRect r = v.rectValue;
        if (NSPointInRect(p, r)) return [self largeEnough:r] ? r : NSZeroRect;
    }
    return NSZeroRect;
}

- (NSRect)selection {
    NSRect r = NSMakeRect(MIN(_start.x, _cur.x), MIN(_start.y, _cur.y),
                          fabs(_cur.x - _start.x), fabs(_cur.y - _start.y));
    return NSIntersectionRect(r, self.bounds);
}

- (void)complete:(int)status rect:(NSRect)r {
    if (_finished) return;
    _finished = YES;
    NSWindow *w = self.window;
    ZoneFinish cb = self.finish;
    self.finish = nil;
    // Tear the window down outside the event handler that triggered it.
    dispatch_async(dispatch_get_main_queue(), ^{
        [w orderOut:nil];
        [w close];
        if (cb) cb(status, r);
    });
}

- (NSPoint)local:(NSEvent *)e {
    return [self convertPoint:e.locationInWindow fromView:nil];
}

- (void)mouseMoved:(NSEvent *)e {
    _mouse = [self local:e];
    _hasMouse = YES;
    [self setNeedsDisplay:YES];
}

- (void)mouseDown:(NSEvent *)e {
    _start = _cur = _mouse = [self local:e];
    _hasMouse = YES;
    _dragging = YES;
    [self setNeedsDisplay:YES];
}

- (void)mouseDragged:(NSEvent *)e {
    if (!_dragging) return;
    _cur = _mouse = [self local:e];
    [self setNeedsDisplay:YES];
}

- (void)mouseUp:(NSEvent *)e {
    if (!_dragging) return;
    _cur = [self local:e];
    _dragging = NO;
    NSRect r = [self selection];
    if ([self largeEnough:r]) {
        [self complete:0 rect:r];
        return;
    }
    // A click (or a tiny drag) takes the window under it; on the desktop it
    // keeps waiting.
    NSRect w = [self windowAt:_start];
    if (!NSIsEmptyRect(w)) {
        [self complete:0 rect:w];
    } else {
        [self setNeedsDisplay:YES];
    }
}

- (void)rightMouseUp:(NSEvent *)e {
    if (_dragging) { // right-click while dragging drops the selection
        _dragging = NO;
        [self setNeedsDisplay:YES];
    } else {
        [self complete:1 rect:NSZeroRect];
    }
}

- (void)keyDown:(NSEvent *)e {
    switch (e.keyCode) {
        case 53: // Esc
            [self complete:1 rect:NSZeroRect];
            break;
        case 36: // Return
        case 76: // keypad Enter
            [self complete:0 rect:self.bounds];
            break;
        default:
            break; // swallow everything else (no beep)
    }
}

- (void)drawImageIn:(NSRect)r {
    [self.image drawInRect:r
                  fromRect:NSZeroRect
                 operation:NSCompositingOperationCopy
                  fraction:1
            respectFlipped:YES
                     hints:nil];
}

- (NSColor *)accent {
    return [NSColor colorWithSRGBRed:0 green:0x78 / 255.0 blue:0xD4 / 255.0 alpha:1];
}

// badge draws text on a dark pill. With above it sits on top of (x, y), else
// it is centred horizontally on x with its top at y.
- (void)badge:(NSString *)text x:(CGFloat)x y:(CGFloat)y above:(BOOL)above {
    NSDictionary *attrs = @{
        NSFontAttributeName: [NSFont systemFontOfSize:13 weight:NSFontWeightSemibold],
        NSForegroundColorAttributeName: [NSColor whiteColor],
    };
    NSSize ts = [text sizeWithAttributes:attrs];
    CGFloat padX = 8, padY = 4;
    NSSize bs = NSMakeSize(ts.width + 2 * padX, ts.height + 2 * padY);
    NSRect r;
    if (above) {
        r = NSMakeRect(x, y - bs.height - 6, bs.width, bs.height);
        if (r.origin.y < 0) r.origin.y = y + 6; // no room above: put it inside
    } else {
        r = NSMakeRect(x - bs.width / 2, y, bs.width, bs.height);
    }
    r.origin.x = MAX(0, MIN(r.origin.x, self.bounds.size.width - bs.width));
    [[NSColor colorWithWhite:0.125 alpha:0.92] setFill];
    [[NSBezierPath bezierPathWithRoundedRect:r xRadius:4 yRadius:4] fill];
    [text drawAtPoint:NSMakePoint(r.origin.x + padX, r.origin.y + padY) withAttributes:attrs];
}

- (void)drawRect:(NSRect)dirty {
    NSRect b = self.bounds;
    [self drawImageIn:b];
    [[NSColor colorWithWhite:0 alpha:0.45] setFill];
    NSRectFillUsingOperation(b, NSCompositingOperationSourceOver);

    if (_dragging) {
        NSRect r = [self selection];
        if (r.size.width > 0 && r.size.height > 0) [self showZone:r];
    } else {
        if (_hasMouse) {
            // The window a click would take, then crosshair guides for
            // precise aiming of a drag.
            NSRect w = [self windowAt:_mouse];
            if (!NSIsEmptyRect(w)) [self showZone:w];
            [[self accent] setFill];
            NSRectFill(NSMakeRect(_mouse.x, 0, 1, b.size.height));
            NSRectFill(NSMakeRect(0, _mouse.y, b.size.width, 1));
        }
        [self badge:@"Drag to select  ·  Click: window  ·  Enter: whole screen  ·  Esc: cancel"
                  x:b.size.width / 2 y:24 above:NO];
    }
}

// showZone draws r undimmed with a border and its size in pixels.
- (void)showZone:(NSRect)r {
    [NSGraphicsContext saveGraphicsState];
    NSRectClip(r);
    [self drawImageIn:self.bounds];
    [NSGraphicsContext restoreGraphicsState];

    // Border just outside the zone so it never covers its pixels.
    NSBezierPath *border = [NSBezierPath bezierPathWithRect:NSInsetRect(r, -1, -1)];
    border.lineWidth = 2;
    [[self accent] setStroke];
    [border stroke];

    NSString *label = [NSString stringWithFormat:@"%ld × %ld",
                       lround(r.size.width * self.scale), lround(r.size.height * self.scale)];
    [self badge:label x:r.origin.x y:r.origin.y above:YES];
}
@end

// Only touched on the main thread.
static FaspZoneWindow *gZoneWindow;

static NSScreen *screenForDisplay(CGDirectDisplayID id) {
    for (NSScreen *s in [NSScreen screens]) {
        NSNumber *n = s.deviceDescription[@"NSScreenNumber"];
        if (n != nil && n.unsignedIntValue == id) return s;
    }
    return nil;
}

// windowRectsOn lists the normal windows on screen (layer 0, not fully
// transparent), front-most first, clipped to display (global Quartz points)
// and moved to display-local points. Taken right after the freeze, so it
// matches the frozen picture; the overlay itself is not up yet.
static NSArray<NSValue *> *windowRectsOn(CGRect display) {
    NSMutableArray<NSValue *> *out = [NSMutableArray array];
    CFArrayRef list = CGWindowListCopyWindowInfo(
        kCGWindowListOptionOnScreenOnly | kCGWindowListExcludeDesktopElements, kCGNullWindowID);
    if (list == NULL) return out;
    for (NSDictionary *info in (__bridge NSArray *)list) {
        if ([info[(__bridge NSString *)kCGWindowLayer] intValue] != 0) continue; // menu bar, Dock, overlays
        NSNumber *alpha = info[(__bridge NSString *)kCGWindowAlpha];
        if (alpha != nil && alpha.doubleValue <= 0) continue;
        NSDictionary *boundsDict = info[(__bridge NSString *)kCGWindowBounds];
        CGRect r;
        if (boundsDict == nil || !CGRectMakeWithDictionaryRepresentation((__bridge CFDictionaryRef)boundsDict, &r)) continue;
        r = CGRectIntersection(r, display);
        if (CGRectIsEmpty(r)) continue;
        r.origin.x -= display.origin.x;
        r.origin.y -= display.origin.y;
        [out addObject:[NSValue valueWithRect:NSRectFromCGRect(r)]];
    }
    CFRelease(list);
    return out;
}

// showOverlay runs on the main thread. It returns an error message, or nil
// once the overlay is up; finish is then called exactly once.
static NSString *showOverlay(CGImageRef image, CGDirectDisplayID displayID, NSArray<NSValue *> *windowRects,
                             int minPixels, ZoneFinish finish) {
    if (gZoneWindow != nil) return @"a zone selection is already open";
    NSScreen *screen = screenForDisplay(displayID);
    if (screen == nil) return @"could not find the screen under the cursor";

    NSRect frame = screen.frame;
    FaspZoneWindow *win = [[FaspZoneWindow alloc] initWithContentRect:frame
                                                            styleMask:NSWindowStyleMaskBorderless
                                                              backing:NSBackingStoreBuffered
                                                                defer:NO
                                                               screen:screen];
    win.releasedWhenClosed = NO;
    win.level = CGShieldingWindowLevel();
    win.opaque = YES;
    win.hasShadow = NO;
    win.backgroundColor = [NSColor blackColor];
    win.acceptsMouseMovedEvents = YES;
    win.collectionBehavior = NSWindowCollectionBehaviorCanJoinAllSpaces |
                             NSWindowCollectionBehaviorFullScreenAuxiliary |
                             NSWindowCollectionBehaviorIgnoresCycle;

    FaspZoneView *view = [[FaspZoneView alloc] initWithFrame:NSMakeRect(0, 0, frame.size.width, frame.size.height)];
    view.image = [[NSImage alloc] initWithCGImage:image size:frame.size];
    view.scale = frame.size.width > 0 ? (CGFloat)CGImageGetWidth(image) / frame.size.width : 1;
    view.minPixels = minPixels;
    view.windowRects = windowRects;
    // The overlay takes the focus for Esc and Enter. On cancel the app the
    // user was in gets it back, so the screen stays as it was; a selection
    // goes on to the editor, which takes the focus anyway.
    NSRunningApplication *previous = NSWorkspace.sharedWorkspace.frontmostApplication;
    view.finish = ^(int status, NSRect sel) {
        gZoneWindow = nil;
        if (status == 1 && previous != nil && ![previous isEqual:NSRunningApplication.currentApplication]) {
            if (@available(macOS 14.0, *)) {
                [NSApp yieldActivationToApplication:previous];
                [previous activateWithOptions:0];
            } else {
                [previous activateWithOptions:0];
            }
        }
        finish(status, sel);
    };
    win.contentView = view;
    gZoneWindow = win;

    if (@available(macOS 14.0, *)) {
        [NSApp activate];
    } else {
        [NSApp activateIgnoringOtherApps:YES];
    }
    [win setFrame:frame display:YES];
    [win makeKeyAndOrderFront:nil];
    [win makeFirstResponder:view];
    return nil;
}

int fasp_zone_select(int wantImage, int minPixels, fasp_zone_result *out) {
    memset(out, 0, sizeof(*out));
    if ([NSThread isMainThread]) {
        out->err = strdup("zone selection must not run on the main thread");
        return -1;
    }
    @autoreleasepool {
        CGImageRef image = NULL;
        CGDirectDisplayID displayID = 0;
        if (fasp_capture_cursor_image(&image, &displayID, &out->err) != 0) {
            return -1;
        }
        CGRect bounds = CGDisplayBounds(displayID);
        out->boundsX = bounds.origin.x;
        out->boundsY = bounds.origin.y;
        out->boundsW = bounds.size.width;
        out->boundsH = bounds.size.height;
        out->imgW = (int)CGImageGetWidth(image);
        out->imgH = (int)CGImageGetHeight(image);
        NSArray<NSValue *> *windowRects = windowRectsOn(bounds);

        __block int status = -1;
        __block NSRect sel = NSZeroRect;
        __block NSString *failure = nil;
        dispatch_semaphore_t done = dispatch_semaphore_create(0);
        dispatch_async(dispatch_get_main_queue(), ^{
            failure = showOverlay(image, displayID, windowRects, minPixels, ^(int st, NSRect r) {
                status = st;
                sel = r;
                dispatch_semaphore_signal(done);
            });
            if (failure != nil) dispatch_semaphore_signal(done);
        });
        dispatch_semaphore_wait(done, DISPATCH_TIME_FOREVER);

        if (failure != nil) {
            out->err = strdup(failure.UTF8String);
            CGImageRelease(image);
            return -1;
        }
        if (status == 0) {
            out->selX = sel.origin.x;
            out->selY = sel.origin.y;
            out->selW = sel.size.width;
            out->selH = sel.size.height;
            if (wantImage) {
                out->image = image; // the caller crops it, then releases it
                return status;
            }
        }
        CGImageRelease(image);
        return status;
    }
}
