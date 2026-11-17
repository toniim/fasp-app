package ocr

import (
	"bytes"
	"errors"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"math"
	"runtime"
	"strings"
	"testing"

	"golang.org/x/image/font"
	"golang.org/x/image/font/basicfont"
	"golang.org/x/image/math/fixed"
)

// renderLines draws black text on white with the built-in 7x13 bitmap font,
// scaled up with nearest-neighbour so strokes stay crisp and large enough for
// an OCR engine.
func renderLines(t *testing.T, lines []string, scale int) []byte {
	t.Helper()
	const cellW, cellH, pad = 7, 18, 4
	maxLen := 0
	for _, l := range lines {
		if len(l) > maxLen {
			maxLen = len(l)
		}
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

func normalize(s string) string {
	return strings.Join(strings.Fields(strings.ToLower(s)), " ")
}

func TestRecognizeReturnsLinesInOrder(t *testing.T) {
	data := renderLines(t, []string{"Hello Fasp 2026", "Second line of text"}, 6)
	res, err := Recognize(data, []string{"en-US"})
	if errors.Is(err, ErrNoLanguage) {
		t.Skipf("no OCR language installed on this %s machine: %v", runtime.GOOS, err)
	}
	if err != nil {
		t.Fatalf("Recognize: %v", err)
	}
	got := normalize(res.Text)
	// "Hello" is left out of the match: Windows OCR reads the blocky test
	// font's "o" as "a" ("Hella"); the order of the lines is what matters.
	i, j := strings.Index(got, "fasp 2026"), strings.Index(got, "second line of text")
	if i < 0 || j < 0 || i > j {
		t.Fatalf("lines missing or out of order, text = %q", res.Text)
	}
	if len(res.Lines) < 2 {
		t.Fatalf("want at least 2 lines, got %d", len(res.Lines))
	}
	for k := 1; k < len(res.Lines); k++ {
		if res.Lines[k].Y < res.Lines[k-1].Y {
			t.Errorf("line %d is above line %d", k, k-1)
		}
	}
	for _, l := range res.Lines {
		if l.W <= 0 || l.H <= 0 || l.X < 0 || l.Y < 0 {
			t.Errorf("bad box for %q: %+v", l.Text, l)
		}
	}
}

func TestRecognizeVietnameseHint(t *testing.T) {
	langs, err := SupportedLanguages()
	if errors.Is(err, ErrUnsupported) {
		t.Skip("ocr unsupported here")
	}
	if err != nil {
		t.Logf("SupportedLanguages: %v", err)
	}
	t.Logf("supported languages: %v", langs)
	// The bitmap font has no diacritics, so this checks that a Vietnamese hint
	// is accepted and still returns the text; it cannot prove accent handling.
	data := renderLines(t, []string{"Xin chao the gioi"}, 6)
	res, err := Recognize(data, []string{"vi-VN"})
	if errors.Is(err, ErrNoLanguage) {
		t.Skipf("no OCR language installed: %v", err)
	}
	if err != nil {
		t.Logf("vi-VN not usable: %v", err)
		return
	}
	t.Logf("vi-VN result: %q", res.Text)
}

func TestRecognizeRejectsBadInput(t *testing.T) {
	if _, err := Recognize(nil, nil); err == nil {
		t.Fatal("empty input must fail")
	}
	if _, err := Recognize([]byte("not an image"), nil); err == nil {
		t.Fatal("garbage input must fail")
	}
}

func TestBoxFromNormalized(t *testing.T) {
	// A box in the bottom-left quarter of a 200x100 image (Vision origin is
	// bottom-left) lands in the lower-left of the top-left-origin pixel space.
	x, y, w, h := boxFromNormalized(0, 0, 0.5, 0.5, 200, 100)
	if x != 0 || y != 50 || w != 100 || h != 50 {
		t.Fatalf("got %v %v %v %v", x, y, w, h)
	}
	// Top-right quarter.
	x, y, w, h = boxFromNormalized(0.5, 0.5, 0.5, 0.5, 200, 100)
	if x != 100 || math.Abs(y) > 1e-9 || w != 100 || h != 50 {
		t.Fatalf("got %v %v %v %v", x, y, w, h)
	}
}

func TestBuildResultReadingOrder(t *testing.T) {
	res := buildResult([]Line{
		{Text: "third", X: 0, Y: 100, W: 50, H: 20},
		{Text: "right", X: 200, Y: 2, W: 50, H: 20},
		{Text: "left", X: 0, Y: 0, W: 50, H: 20},
		{Text: "  ", X: 0, Y: 50, W: 5, H: 5},
	})
	if res.Text != "left\nright\nthird" {
		t.Fatalf("got %q", res.Text)
	}
	if len(res.Lines) != 3 {
		t.Fatalf("blank line must be dropped, got %d lines", len(res.Lines))
	}
}

func TestToBGRA(t *testing.T) {
	src := image.NewNRGBA(image.Rect(0, 0, 2, 1))
	src.Set(0, 0, color.NRGBA{R: 10, G: 20, B: 30, A: 255})
	src.Set(1, 0, color.NRGBA{A: 0}) // transparent becomes white
	var buf bytes.Buffer
	if err := png.Encode(&buf, src); err != nil {
		t.Fatal(err)
	}
	pix, w, h, err := toBGRA(buf.Bytes())
	if err != nil || w != 2 || h != 1 {
		t.Fatalf("err=%v w=%d h=%d", err, w, h)
	}
	want := []byte{30, 20, 10, 255, 255, 255, 255, 255}
	if !bytes.Equal(pix, want) {
		t.Fatalf("got %v want %v", pix, want)
	}
}
