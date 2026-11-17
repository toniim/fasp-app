#ifndef FASP_CAPTURE_DARWIN_H
#define FASP_CAPTURE_DARWIN_H

#include <CoreGraphics/CoreGraphics.h>

int fasp_capture_cursor_display(void **out, int *outLen, int *width, int *height, char **errOut);

// Captures the display under the cursor. *outImage is retained (caller
// releases); *errOut is malloc'd on failure.
int fasp_capture_cursor_image(CGImageRef *outImage, CGDirectDisplayID *outDisplay, char **errOut);

// Captures one display by id (+1 retained image; *errOut malloc'd on failure).
int fasp_capture_display_image(CGDirectDisplayID displayID, CGImageRef *outImage, char **errOut);

typedef struct {
    unsigned int id;
    // Global Quartz points, top-left origin.
    double x, y, w, h;
} fasp_display;

// Lists the active displays; returns how many were written (at most max).
int fasp_list_displays(fasp_display *out, int max);

typedef struct {
    // Selection in points, relative to the top-left of the display.
    double selX, selY, selW, selH;
    // CGDisplayBounds of the display: global Quartz points, top-left origin.
    double boundsX, boundsY, boundsW, boundsH;
    // Size of the captured image in pixels.
    int imgW, imgH;
    // The captured image (retained, release with fasp_release_image) when
    // requested.
    CGImageRef image;
    // Malloc'd message when fasp_zone_select returns -1.
    char *err;
} fasp_zone_result;

// Freezes the display under the cursor in a full-screen overlay and blocks
// until the user selects a zone. Returns 0 (selected), 1 (cancelled) or -1
// (error). Must not be called on the main thread.
int fasp_zone_select(int wantImage, int minPixels, fasp_zone_result *out);

// Crops image to a pixel rectangle (top-left origin) and encodes it as PNG
// with the image's colour profile. *out is malloc'd. Returns 0 or -1.
int fasp_crop_png(CGImageRef image, int x, int y, int w, int h, void **out, int *outLen, char **errOut);
void fasp_release_image(CGImageRef image);

#endif
