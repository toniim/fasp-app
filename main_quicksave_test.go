package main

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/heytonyne/fasp/internal/service/file"
	"github.com/heytonyne/fasp/internal/service/settings"
)

// TestQuickSaveImageDefaultFolder checks that quick save resolves a platform
// default folder on its own (no settings) and writes a real PNG there.
func TestQuickSaveImageDefaultFolder(t *testing.T) {
	app := &App{
		fileService:     file.New(context.Background()),
		settingsService: settings.New(),
	}

	var buf bytes.Buffer
	img := image.NewRGBA(image.Rect(0, 0, 2, 2))
	img.Set(0, 0, color.RGBA{R: 255, A: 255})
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("encode png: %v", err)
	}

	path, err := app.QuickSaveImage(buf.Bytes())
	if err != nil {
		t.Fatalf("QuickSaveImage: %v", err)
	}
	defer os.Remove(path)

	if _, err := os.Stat(path); err != nil {
		t.Fatalf("saved file missing: %v", err)
	}
	if filepath.Ext(path) != ".png" {
		t.Fatalf("unexpected extension: %s", path)
	}

	// Saving again must not collide (timestamped name).
	path2, err := app.QuickSaveImage(buf.Bytes())
	if err != nil {
		t.Fatalf("second QuickSaveImage: %v", err)
	}
	defer os.Remove(path2)
	if path == path2 {
		t.Fatalf("duplicate filename: %s", path)
	}
	t.Logf("saved %s", path)
}
