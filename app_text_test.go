package main

import (
	"bytes"
	"errors"
	"image"
	"image/draw"
	"image/png"
	"strings"
	"testing"

	"github.com/heytonyne/fasp/internal/service/ocr"
	"github.com/heytonyne/fasp/internal/service/settings"
	"golang.org/x/image/font"
	"golang.org/x/image/font/basicfont"
	"golang.org/x/image/math/fixed"
)

// textPNG draws black text on white, scaled up so an OCR engine can read it.
func textPNG(t *testing.T, lines []string) []byte {
	t.Helper()
	const cellW, cellH, pad, scale = 7, 18, 4, 6
	maxLen := 0
	for _, l := range lines {
		maxLen = max(maxLen, len(l))
	}
	small := image.NewRGBA(image.Rect(0, 0, maxLen*cellW+2*pad, len(lines)*cellH+2*pad))
	draw.Draw(small, small.Bounds(), image.White, image.Point{}, draw.Src)
	d := &font.Drawer{Dst: small, Src: image.Black, Face: basicfont.Face7x13}
	for i, l := range lines {
		d.Dot = fixed.P(pad, pad+13+i*cellH)
		d.DrawString(l)
	}
	big := image.NewRGBA(image.Rect(0, 0, small.Bounds().Dx()*scale, small.Bounds().Dy()*scale))
	for y := 0; y < big.Bounds().Dy(); y++ {
		for x := 0; x < big.Bounds().Dx(); x++ {
			big.Set(x, y, small.At(x/scale, y/scale))
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, big); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// textApp is an App whose settings live in a scratch folder and whose text
// clipboard is captured.
func textApp(t *testing.T) (*App, *[]string) {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("APPDATA", dir)
	var copied []string
	a := NewApp()
	a.settingsService = settings.New()
	a.copyTextFn = func(s string) error { copied = append(copied, s); return nil }
	return a, &copied
}

func TestCopyTextCopiesRecognisedLines(t *testing.T) {
	a, copied := textApp(t)
	res, err := a.CopyText(textPNG(t, []string{"Hello Fasp 2026", "Second line of text"}))
	if errors.Is(errors.Unwrap(err), ocr.ErrNoLanguage) || (err != nil && strings.Contains(err.Error(), "No OCR language")) {
		t.Skipf("no OCR language on this machine: %v", err)
	}
	if err != nil {
		t.Fatalf("CopyText: %v", err)
	}
	got := strings.Join(strings.Fields(strings.ToLower(res.Text)), " ")
	if !strings.Contains(got, "hello fasp 2026") || !strings.Contains(got, "second line of text") {
		t.Fatalf("text = %q", res.Text)
	}
	if res.Lines < 2 {
		t.Errorf("lines = %d, want at least 2", res.Lines)
	}
	if len(*copied) != 1 || (*copied)[0] != res.Text {
		t.Errorf("clipboard = %q, want the recognised text once", *copied)
	}
}

func TestCopyTextOfBlankImageLeavesClipboardAlone(t *testing.T) {
	a, copied := textApp(t)
	blank := image.NewRGBA(image.Rect(0, 0, 200, 100))
	draw.Draw(blank, blank.Bounds(), image.White, image.Point{}, draw.Src)
	var buf bytes.Buffer
	if err := png.Encode(&buf, blank); err != nil {
		t.Fatal(err)
	}
	res, err := a.CopyText(buf.Bytes())
	if err != nil && strings.Contains(err.Error(), "OCR language") {
		t.Skipf("no OCR language on this machine: %v", err)
	}
	if err != nil {
		t.Fatalf("CopyText: %v", err)
	}
	if res.Lines != 0 || res.Text != "" {
		t.Errorf("blank image gave %+v", res)
	}
	if len(*copied) != 0 {
		t.Errorf("clipboard was written for an image without text: %q", *copied)
	}
}

func TestOCRErrorMessages(t *testing.T) {
	if msg := ocrError(ocr.ErrNoLanguage).Error(); !strings.Contains(msg, "OCR language") {
		t.Errorf("no-language message = %q", msg)
	}
	other := errors.New("boom")
	if err := ocrError(other); !errors.Is(err, other) {
		t.Errorf("other errors must stay unwrappable, got %v", err)
	}
}

func TestOCRLanguageSettingIsAHint(t *testing.T) {
	a, _ := textApp(t)
	if got := a.ocrLanguages(); got != nil {
		t.Errorf("default language hint = %v, want automatic (nil)", got)
	}
	if err := a.settingsService.Set("ocr_language", "vi-VN"); err != nil {
		t.Fatal(err)
	}
	if got := a.ocrLanguages(); len(got) != 1 || got[0] != "vi-VN" {
		t.Errorf("language hint = %v, want [vi-VN]", got)
	}
	if err := a.settingsService.Set("ocr_language", 5); err == nil {
		t.Error("a non-string language must be rejected")
	}
}

func TestTextSummary(t *testing.T) {
	if got := textSummary(1); got != "Copied 1 line of text" {
		t.Errorf("got %q", got)
	}
	if got := textSummary(3); got != "Copied 3 lines of text" {
		t.Errorf("got %q", got)
	}
}
