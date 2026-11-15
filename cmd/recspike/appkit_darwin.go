//go:build darwin

package main

/*
#cgo LDFLAGS: -framework AppKit
#include <stdbool.h>

// From AppKit's NSApplication.h; the cgo preamble is C, not Objective-C.
extern bool NSApplicationLoad(void);

static void faspLoadAppKit(void) { NSApplicationLoad(); }
*/
import "C"

// ScreenCaptureKit's window filter needs a window-server connection, which a
// plain command-line process does not have until AppKit is loaded (the app
// gets one from NSApplication). Without it, -mode window aborts with
// "CGS_REQUIRE_INIT".
func init() { C.faspLoadAppKit() }
