//go:build !windows

package main

import "github.com/wailsapp/wails/v3/pkg/application"

// setExcludedFromCapture is a no-op on macOS: the ScreenCaptureKit filter
// already leaves Fasp's own windows out of recordings.
func setExcludedFromCapture(*application.WebviewWindow, bool) {}
