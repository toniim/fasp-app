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
    // Without permission, we get limited/no window info
    // With permission, we can see all windows with full details

    CFArrayRef windowList = CGWindowListCopyWindowInfo(
        kCGWindowListOptionAll | kCGWindowListExcludeDesktopElements,
        kCGNullWindowID
    );

    if (windowList == NULL) {
        printf("[DEBUG C] Failed to get window list - NO PERMISSION\n");
        return false;
    }

    CFIndex count = CFArrayGetCount(windowList);
    printf("[DEBUG C] Total windows visible: %ld\n", count);

    // Count windows from other apps (not our own)
    int otherAppWindows = 0;
    int ourAppWindows = 0;
    int windowsWithoutOwner = 0;

    // Print first 10 windows for debugging
    printf("[DEBUG C] First 10 windows:\n");

    for (CFIndex i = 0; i < count; i++) {
        CFDictionaryRef window = (CFDictionaryRef)CFArrayGetValueAtIndex(windowList, i);

        // Get window owner name
        CFStringRef ownerName = (CFStringRef)CFDictionaryGetValue(window, kCGWindowOwnerName);
        if (ownerName != NULL) {
            char ownerNameStr[256];
            if (CFStringGetCString(ownerName, ownerNameStr, sizeof(ownerNameStr), kCFStringEncodingUTF8)) {
                // Print first 10 for debugging
                if (i < 10) {
                    printf("[DEBUG C]   Window %ld: %s\n", i, ownerNameStr);
                }

                // Check if it's our own app or dev environment
                bool isOurApp = (strcmp(ownerNameStr, "grabix") == 0 ||
                                strcmp(ownerNameStr, "Grabix") == 0 ||
                                strcmp(ownerNameStr, "Terminal") == 0 ||
                                strcmp(ownerNameStr, "Code") == 0 ||
                                strcmp(ownerNameStr, "Visual Studio Code") == 0 ||
                                strcmp(ownerNameStr, "Cursor") == 0 ||
                                strcmp(ownerNameStr, "iTerm2") == 0 ||
                                strcmp(ownerNameStr, "iTerm") == 0);

                if (isOurApp) {
                    ourAppWindows++;
                } else {
                    otherAppWindows++;
                }
            }
        } else {
            windowsWithoutOwner++;
            if (i < 10) {
                printf("[DEBUG C]   Window %ld: (no owner)\n", i);
            }
        }
    }

    CFRelease(windowList);

    printf("[DEBUG C] Summary:\n");
    printf("[DEBUG C]   Total windows: %ld\n", count);
    printf("[DEBUG C]   Our app windows: %d\n", ourAppWindows);
    printf("[DEBUG C]   Other app windows: %d\n", otherAppWindows);
    printf("[DEBUG C]   Windows without owner: %d\n", windowsWithoutOwner);

    // Permission check logic:
    // Key indicator: Can we see windows from OTHER apps?
    // Without permission: we can only see our own windows (or very limited list)
    // With permission: we can see windows from Finder, Safari, Chrome, etc.
    //
    // Even if user has minimal apps open, they should have at least:
    // - Finder (always running)
    // - WindowServer, Dock (system)
    // So if otherAppWindows == 0, we likely don't have permission
    bool hasPermission = (otherAppWindows > 0);

    printf("[DEBUG C] Screen Recording Permission: %s\n",
           hasPermission ? "✅ GRANTED" : "❌ DENIED");
    printf("[DEBUG C] Reason: %s\n",
           hasPermission ? "Can see windows from other apps" : "Can only see our own windows");

    return hasPermission;
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
