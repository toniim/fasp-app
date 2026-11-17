//go:build !windows

package main

import "github.com/wailsapp/wails/v3/pkg/application"

// focusWebView is only needed on Windows; see focus_windows.go.
func focusWebView(*application.WebviewWindow) {}
