// Package ocr recognises text in images with the operating system's own OCR
// engine: Vision on macOS, Windows.Media.Ocr on Windows. Nothing is bundled and
// nothing leaves the machine.
package ocr

import (
	"bytes"
	"errors"
	"image"
	"image/draw"
	"image/png"
	"sort"
	"strings"
)

// ErrUnsupported is returned on platforms without an OCR engine.
var ErrUnsupported = errors.New("ocr is not supported on this platform")

// ErrNoLanguage is returned when the OS has no OCR language installed.
var ErrNoLanguage = errors.New("no OCR language is installed")

// Line is one recognised line of text. The box is in image pixels with the
// origin at the top-left corner.
type Line struct {
	Text       string
	X, Y, W, H float64
}

// Result holds the recognised lines in reading order.
type Result struct {
	Lines []Line
	// Text is the lines joined with "\n".
	Text string
}

// boxFromNormalized converts a Vision-style box (normalized 0..1, origin at the
// bottom-left) to image pixels with a top-left origin.
func boxFromNormalized(nx, ny, nw, nh float64, imgW, imgH int) (x, y, w, h float64) {
	W, H := float64(imgW), float64(imgH)
	return nx * W, (1 - ny - nh) * H, nw * W, nh * H
}

// buildResult drops empty lines, puts the rest in reading order and joins the
// text. Engines return lines in their own order, which is not guaranteed to
// match what a person reads, so the order is fixed here for both platforms.
func buildResult(lines []Line) *Result {
	kept := make([]Line, 0, len(lines))
	for _, l := range lines {
		if strings.TrimSpace(l.Text) != "" {
			kept = append(kept, l)
		}
	}
	sortReadingOrder(kept)
	texts := make([]string, len(kept))
	for i, l := range kept {
		texts[i] = l.Text
	}
	return &Result{Lines: kept, Text: strings.Join(texts, "\n")}
}

// sortReadingOrder orders lines top-to-bottom, and left-to-right among lines
// that sit on the same visual row (their vertical centres fall within the
// first line's box of that row).
func sortReadingOrder(lines []Line) {
	sort.SliceStable(lines, func(i, j int) bool { return lines[i].Y < lines[j].Y })
	for start := 0; start < len(lines); {
		ref := lines[start]
		end := start + 1
		for end < len(lines) && lines[end].Y+lines[end].H/2 <= ref.Y+ref.H {
			end++
		}
		row := lines[start:end]
		sort.SliceStable(row, func(i, j int) bool { return row[i].X < row[j].X })
		start = end
	}
}

// toBGRA decodes a PNG into tightly packed opaque BGRA pixels. Transparent
// areas are composited over white because OCR engines expect an opaque image.
func toBGRA(pngData []byte) (pix []byte, w, h int, err error) {
	img, err := png.Decode(bytes.NewReader(pngData))
	if err != nil {
		return nil, 0, 0, err
	}
	b := img.Bounds()
	w, h = b.Dx(), b.Dy()
	if w == 0 || h == 0 {
		return nil, 0, 0, errors.New("image is empty")
	}
	rgba := image.NewRGBA(image.Rect(0, 0, w, h))
	draw.Draw(rgba, rgba.Bounds(), image.White, image.Point{}, draw.Src)
	draw.Draw(rgba, rgba.Bounds(), img, b.Min, draw.Over)
	pix = make([]byte, w*h*4)
	for i := 0; i < w*h*4; i += 4 {
		pix[i], pix[i+1], pix[i+2], pix[i+3] = rgba.Pix[i+2], rgba.Pix[i+1], rgba.Pix[i], 0xFF
	}
	return pix, w, h, nil
}
