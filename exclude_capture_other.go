//go:build !windows

package main

// setExcludedFromCapture is a no-op on macOS: the ScreenCaptureKit filter
// already leaves Fasp's own windows out of recordings.
func setExcludedFromCapture(bool) {}
