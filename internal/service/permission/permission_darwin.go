//go:build darwin

package permission

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework ApplicationServices -framework AVFoundation -framework CoreGraphics
#import <ApplicationServices/ApplicationServices.h>
#import <AVFoundation/AVFoundation.h>
#import <CoreGraphics/CoreGraphics.h>

bool checkAccessibilityPermission() {
    NSDictionary *options = @{(__bridge id)kAXTrustedCheckOptionPrompt: @NO};
    return AXIsProcessTrustedWithOptions((__bridge CFDictionaryRef)options);
}

void requestAccessibilityPermission() {
    NSDictionary *options = @{(__bridge id)kAXTrustedCheckOptionPrompt: @YES};
    AXIsProcessTrustedWithOptions((__bridge CFDictionaryRef)options);
}

bool checkScreenRecordingPermission() {
    // Check Screen Recording permission using CGPreflightScreenCaptureAccess
    // This is the recommended way on macOS 10.15+
    //
    // Returns:
    // - true: permission granted
    // - false: permission denied or not determined

    printf("[DEBUG C] Checking Screen Recording permission...\n");

    // CGPreflightScreenCaptureAccess checks without prompting

    bool hasPermission = false;
	if (@available(macOS 10.15, *)) {
        hasPermission = CGPreflightScreenCaptureAccess();
    }

    printf("[DEBUG C] Screen Recording Permission: %s\n",
           hasPermission ? "✅ GRANTED" : "❌ DENIED");


    return hasPermission;
}

void openScreenRecordingSettings() {
    // Open System Settings to Screen Recording page
    // This is the most reliable way to guide users
    CFStringRef urlString = CFStringCreateWithCString(NULL,
        "x-apple.systempreferences:com.apple.preference.security?Privacy_ScreenCapture",
        kCFStringEncodingUTF8);

    if (urlString != NULL) {
        CFURLRef url = CFURLCreateWithString(NULL, urlString, NULL);
        if (url != NULL) {
            LSOpenCFURLRef(url, NULL);
            CFRelease(url);
        }
        CFRelease(urlString);
    }
}

bool requestScreenRecordingPermission(void) {
    if (@available(macOS 10.15, *)) {
        // This triggers registration + may prompt / guide; returns current result
        bool granted = CGRequestScreenCaptureAccess();

        // Even if it returns false, user might need manual toggle in Settings
        if (!granted) {
            openScreenRecordingSettings();
        }
        return granted;
    }

    // On < 10.15: no Screen Recording permission model like this
    return true;
}
*/
import "C"

type serviceImpl struct{}

// New creates a new permission service instance
func New() Service {
	return &serviceImpl{}
}

// CheckAccessibility checks if the app has Accessibility permissions
func (s *serviceImpl) CheckAccessibility() (bool, error) {
	println("[DEBUG Go] Checking Accessibility permission...")
	result := bool(C.checkAccessibilityPermission())
	println("[DEBUG Go] Accessibility permission:", result)
	return result, nil
}

// RequestAccessibility requests Accessibility permissions (opens System Settings)
func (s *serviceImpl) RequestAccessibility() error {
	println("[INFO Go] Requesting Accessibility permission (opening System Settings)...")
	C.requestAccessibilityPermission()
	return nil
}

// CheckScreenRecording checks if the app has Screen Recording permissions
func (s *serviceImpl) CheckScreenRecording() (bool, error) {
	println("[DEBUG Go] Checking Screen Recording permission...")
	result := bool(C.checkScreenRecordingPermission())
	println("[DEBUG Go] Screen Recording permission:", result)
	return result, nil
}

// RequestScreenRecording requests Screen Recording permissions (opens System Settings)
func (s *serviceImpl) RequestScreenRecording() error {
	println("[INFO Go] Requesting Screen Recording permission (opening System Settings)...")
	C.requestScreenRecordingPermission()
	return nil
}
