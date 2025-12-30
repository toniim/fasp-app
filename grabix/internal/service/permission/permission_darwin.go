//go:build darwin

package permission

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework ApplicationServices
#import <ApplicationServices/ApplicationServices.h>

bool checkAccessibilityPermission() {
    NSDictionary *options = @{(__bridge id)kAXTrustedCheckOptionPrompt: @NO};
    return AXIsProcessTrustedWithOptions((__bridge CFDictionaryRef)options);
}

void requestAccessibilityPermission() {
    NSDictionary *options = @{(__bridge id)kAXTrustedCheckOptionPrompt: @YES};
    AXIsProcessTrustedWithOptions((__bridge CFDictionaryRef)options);
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

