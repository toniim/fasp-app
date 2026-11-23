//go:build !windows && !darwin

package main

import "github.com/wailsapp/wails/v3/pkg/application"

// focusWebView is only needed on Windows and macOS; see focus_windows.go
// and focus_darwin.go.
func focusWebView(*application.WebviewWindow) {}
