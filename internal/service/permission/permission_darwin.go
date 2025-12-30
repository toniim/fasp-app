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
    // Check screen recording permission by getting window list
    // Without permission, we can only see our own windows
    // With permission, we can see all windows

    CFArrayRef windowList = CGWindowListCopyWindowInfo(
        kCGWindowListOptionAll | kCGWindowListExcludeDesktopElements,
        kCGNullWindowID
    );

    if (windowList == NULL) {
        return false;
    }

    CFIndex count = CFArrayGetCount(windowList);

    // Count windows from other apps (not our own)
    int otherAppWindows = 0;
    for (CFIndex i = 0; i < count; i++) {
        CFDictionaryRef window = (CFDictionaryRef)CFArrayGetValueAtIndex(windowList, i);

        // Get window owner name
        CFStringRef ownerName = (CFStringRef)CFDictionaryGetValue(window, kCGWindowOwnerName);
        if (ownerName != NULL) {
            // Check if it's not our app (grabix or Terminal/IDE in dev mode)
            char ownerNameStr[256];
            if (CFStringGetCString(ownerName, ownerNameStr, sizeof(ownerNameStr), kCFStringEncodingUTF8)) {
                // If we can see windows from other apps, we have permission
                if (strcmp(ownerNameStr, "grabix") != 0 &&
                    strcmp(ownerNameStr, "Terminal") != 0 &&
                    strcmp(ownerNameStr, "Code") != 0 &&
                    strcmp(ownerNameStr, "Visual Studio Code") != 0) {
                    otherAppWindows++;
                }
            }
        }
    }

    CFRelease(windowList);

    // Debug logging
    printf("[DEBUG C] Total windows: %ld, Other app windows: %d\n", count, otherAppWindows);

    // If we can see windows from other apps, we have permission
    // Typically need to see at least a few windows (Finder, Dock, etc.)
    return otherAppWindows > 3;
}

void requestScreenRecordingPermission() {
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
*/
import "C"

type serviceImpl struct{}

// New creates a new permission service instance
func New() Service {
	return &serviceImpl{}
}

// CheckAccessibility checks if the app has Accessibility permissions
func (s *serviceImpl) CheckAccessibility() (bool, error) {
	result := bool(C.checkAccessibilityPermission())
	return result, nil
}

// RequestAccessibility requests Accessibility permissions (opens System Settings)
func (s *serviceImpl) RequestAccessibility() error {
	C.requestAccessibilityPermission()
	return nil
}

// CheckScreenRecording checks if the app has Screen Recording permissions
func (s *serviceImpl) CheckScreenRecording() (bool, error) {
	result := bool(C.checkScreenRecordingPermission())
	println("[DEBUG] CheckScreenRecording result:", result)
	return result, nil
}

// RequestScreenRecording requests Screen Recording permissions (opens System Settings)
func (s *serviceImpl) RequestScreenRecording() error {
	C.requestScreenRecordingPermission()
	return nil
}
