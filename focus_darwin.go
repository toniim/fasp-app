//go:build darwin

package main

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework AppKit
#import <AppKit/AppKit.h>

static void faspBringToFront(void *window) {
	NSWindow *win = (NSWindow *)window;
	if (![NSApp isActive]) {
		[NSApp activate];
	}
	[win makeKeyAndOrderFront:nil];
	// Activation is only a request since macOS 14 and can be refused. The
	// window still goes in front of the other apps' windows, so it is seen;
	// the first click on it makes Fasp active.
	[win orderFrontRegardless];
}
*/
import "C"

import "github.com/wailsapp/wails/v3/pkg/application"

// focusWebView brings the editor in front of the other apps' windows.
//
// Window.Focus asks to activate Fasp and orders the window front, but since
// macOS 14 the system may refuse the activation, for example after a hotkey
// capture started from another app. makeKeyAndOrderFront then orders the
// window front only among Fasp's own windows, so it stayed behind the active
// app until the user switched to Fasp.
func focusWebView(w *application.WebviewWindow) {
	application.InvokeSync(func() {
		if win := w.NativeWindow(); win != nil {
			C.faspBringToFront(win)
		}
	})
}
