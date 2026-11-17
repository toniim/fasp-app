package media

import (
	"bytes"
	"image/color"
	"image/gif"
	"testing"
)

// solid returns a BGRA frame of one colour.
func solid(w, h int, r, g, b uint8) []byte {
	px := make([]byte, w*h*4)
	for i := 0; i < len(px); i += 4 {
		px[i], px[i+1], px[i+2], px[i+3] = b, g, r, 255
	}
	return px
}

// The writer is pure Go, so its contract is checked on every platform with the
// stdlib decoder: frames, timing, colours, and delta frames compositing back
// to the source.
func TestGIFEncoderRoundTrip(t *testing.T) {
	const w, h = 32, 24
	frames := [][]byte{solid(w, h, 30, 30, 30), solid(w, h, 30, 30, 30), solid(w, h, 30, 30, 30)}
	// Frame 1 changes one block; frame 2 is identical to frame 1.
	for y := 4; y < 8; y++ {
		for x := 10; x < 14; x++ {
			i := (y*w + x) * 4
			frames[1][i], frames[1][i+1], frames[1][i+2] = 0, 0, 255 // red (BGRA)
		}
	}
	copy(frames[2], frames[1])

	hist := &histogram{}
	for _, f := range frames {
		hist.add(f)
	}
	var buf bytes.Buffer
	enc, err := newGIFEncoder(&buf, w, h, 12, hist.palette())
	if err != nil {
		t.Fatal(err)
	}
	var sizes []int64
	for _, f := range frames {
		n, err := enc.addFrame(f)
		if err != nil {
			t.Fatal(err)
		}
		sizes = append(sizes, n)
	}
	if err := enc.close(); err != nil {
		t.Fatal(err)
	}

	g, err := gif.DecodeAll(&buf)
	if err != nil {
		t.Fatalf("not a valid GIF: %v", err)
	}
	if len(g.Image) != 3 || g.Config.Width != w || g.Config.Height != h {
		t.Fatalf("%d frames of %dx%d, want 3 of %dx%d", len(g.Image), g.Config.Width, g.Config.Height, w, h)
	}
	if b := g.Image[1].Bounds(); b.Dx() != 4 || b.Dy() != 4 {
		t.Errorf("delta frame covers %v, want only the changed 4x4 block", b)
	}
	if b := g.Image[2].Bounds(); b.Dx() != 1 || b.Dy() != 1 {
		t.Errorf("identical frame covers %v, want a 1x1 placeholder", b)
	}
	if sizes[1] >= sizes[0] {
		t.Errorf("delta frame took %d bytes, full frame %d", sizes[1], sizes[0])
	}
	// 12 fps is 8.33 centiseconds: the delays must add up to 25, not 24.
	total := 0
	for _, d := range g.Delay {
		total += d
	}
	if total != 25 {
		t.Errorf("delays %v add up to %d, want 25", g.Delay, total)
	}
	// Flat colours are exact, not dithered or snapped to a bin.
	if c := color.RGBAModel.Convert(g.Image[0].At(0, 0)).(color.RGBA); c.R != 30 || c.G != 30 || c.B != 30 {
		t.Errorf("flat colour came out as %v, want 30,30,30", c)
	}
	if c := color.RGBAModel.Convert(g.Image[1].At(11, 5)).(color.RGBA); c.R != 255 || c.G != 0 || c.B != 0 {
		t.Errorf("changed block came out as %v, want red", c)
	}
}

func TestPaletteCoversManyColours(t *testing.T) {
	// 4096 distinct colours must reduce to at most 255 entries without panicking,
	// and an image of them must still encode.
	const w, h = 64, 64
	px := make([]byte, w*h*4)
	for i := 0; i < w*h; i++ {
		px[i*4], px[i*4+1], px[i*4+2], px[i*4+3] = byte(i%16*16), byte(i/16%16*16), byte(i/256%16*16), 255
	}
	hist := &histogram{}
	hist.add(px)
	var buf bytes.Buffer
	enc, err := newGIFEncoder(&buf, w, h, 10, hist.palette())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := enc.addFrame(px); err != nil {
		t.Fatal(err)
	}
	if err := enc.close(); err != nil {
		t.Fatal(err)
	}
	if _, err := gif.DecodeAll(&buf); err != nil {
		t.Fatalf("not a valid GIF: %v", err)
	}
}
