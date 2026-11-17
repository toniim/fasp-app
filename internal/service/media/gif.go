package media

import (
	"compress/lzw"
	"errors"
	"image"
	"io"
	"math"
	"sort"
)

// A streaming GIF writer. image/gif only encodes a complete GIF held in
// memory, which for a 10 s 1080p-source recording is more than needed: this
// writer emits each frame as it arrives and keeps one reference frame.
//
// Frames use one global palette of 255 colours; index 255 is transparent.
// After the first frame only the pixels that changed are written (cropped to
// their bounding box, the rest transparent), which is what keeps screen
// recordings small: most of a screen does not change between frames.

const (
	gifColors      = 255 // palette entries that are real colours
	gifTransparent = 255
	// deltaTolerance is the per-channel difference below which a pixel counts
	// as unchanged. Lossy video leaves a little noise on a still screen, and
	// rewriting it would cost more than it shows.
	deltaTolerance = 3
)

// gifOverhead is the header: signature, screen descriptor, the 768-byte
// colour table, the loop extension and the trailer.
func gifOverhead() int64 { return 6 + 7 + 768 + 19 + 1 }

type palette struct {
	rgb [256][3]uint8
}

// histogram counts pixels in 5-bit-per-channel bins and remembers the exact
// colour sum of each bin, so palette entries are the real average colour:
// flat UI colours come out exact instead of snapped to the bin.
type histogram struct {
	n   [1 << 15]uint32
	sum [3][1 << 15]uint64 // r, g, b
}

func binOf(r, g, b uint8) int { return int(r>>3)<<10 | int(g>>3)<<5 | int(b>>3) }

// add counts the pixels of a BGRA buffer.
func (h *histogram) add(bgra []byte) {
	for i := 0; i+3 < len(bgra); i += 4 {
		b, g, r := bgra[i], bgra[i+1], bgra[i+2]
		k := binOf(r, g, b)
		h.n[k]++
		h.sum[0][k] += uint64(r)
		h.sum[1][k] += uint64(g)
		h.sum[2][k] += uint64(b)
	}
}

type colorBox struct {
	bins  []int
	count uint64
}

// palette runs median cut over the bins: the box holding the most pixels is
// split along its widest colour axis at the pixel-count median, until there
// are gifColors boxes or every box is a single bin.
func (h *histogram) palette() *palette {
	var all []int
	var total uint64
	for k := range h.n {
		if h.n[k] > 0 {
			all = append(all, k)
			total += uint64(h.n[k])
		}
	}
	boxes := []colorBox{{bins: all, count: total}}
	for len(boxes) < gifColors {
		best := -1
		for i, b := range boxes {
			if len(b.bins) > 1 && (best < 0 || b.count > boxes[best].count) {
				best = i
			}
		}
		if best < 0 {
			break
		}
		lo, hi := splitBox(h, boxes[best])
		boxes[best] = lo
		boxes = append(boxes, hi)
	}
	p := &palette{}
	for i, b := range boxes {
		if b.count == 0 {
			continue
		}
		var s [3]uint64
		for _, k := range b.bins {
			for c := 0; c < 3; c++ {
				s[c] += h.sum[c][k]
			}
		}
		for c := 0; c < 3; c++ {
			p.rgb[i][c] = uint8((s[c] + b.count/2) / b.count)
		}
	}
	return p
}

func splitBox(h *histogram, b colorBox) (colorBox, colorBox) {
	var minV, maxV [3]int
	for c := range minV {
		minV[c], maxV[c] = 31, 0
	}
	comp := func(k, c int) int {
		switch c {
		case 0:
			return k >> 10
		case 1:
			return k >> 5 & 31
		}
		return k & 31
	}
	for _, k := range b.bins {
		for c := 0; c < 3; c++ {
			v := comp(k, c)
			minV[c], maxV[c] = min(minV[c], v), max(maxV[c], v)
		}
	}
	axis := 0
	for c := 1; c < 3; c++ {
		if maxV[c]-minV[c] > maxV[axis]-minV[axis] {
			axis = c
		}
	}
	sort.Slice(b.bins, func(i, j int) bool { return comp(b.bins[i], axis) < comp(b.bins[j], axis) })
	var acc uint64
	cut := 0
	for i, k := range b.bins[:len(b.bins)-1] {
		acc += uint64(h.n[k])
		cut = i + 1
		if acc*2 >= b.count {
			break
		}
	}
	lo := colorBox{bins: b.bins[:cut]}
	hi := colorBox{bins: b.bins[cut:]}
	for _, k := range lo.bins {
		lo.count += uint64(h.n[k])
	}
	hi.count = b.count - lo.count
	return lo, hi
}

// quantizer maps colours to the nearest palette entry. Searching 255 entries
// per pixel is too slow for a 1080p frame, so results are cached: exact colours
// (flat areas, which must map the same way every time) in a direct-mapped
// table, and dither-adjusted colours by their 5-bit bin.
type quantizer struct {
	pal   *palette
	exact [1 << 16]uint32 // idx<<24 | rgb; all ones = empty
	bin   [1 << 15]int16  // -1 = not computed
}

func newQuantizer(p *palette) *quantizer {
	q := &quantizer{pal: p}
	for i := range q.exact {
		q.exact[i] = 0xFFFFFFFF
	}
	for i := range q.bin {
		q.bin[i] = -1
	}
	return q
}

func (q *quantizer) nearest(r, g, b int) uint8 {
	best, bestD := 0, math.MaxInt
	for i := 0; i < gifColors; i++ {
		c := q.pal.rgb[i]
		dr, dg, db := r-int(c[0]), g-int(c[1]), b-int(c[2])
		if d := 2*dr*dr + 4*dg*dg + 3*db*db; d < bestD {
			best, bestD = i, d
		}
	}
	return uint8(best)
}

// exactIndex is for an unadjusted source colour.
func (q *quantizer) exactIndex(r, g, b uint8) uint8 {
	key := uint32(r)<<16 | uint32(g)<<8 | uint32(b)
	slot := (key * 2654435761) >> 16
	if e := q.exact[slot]; e != 0xFFFFFFFF && e&0xFFFFFF == key {
		return uint8(e >> 24)
	}
	idx := q.nearest(int(r), int(g), int(b))
	q.exact[slot] = uint32(idx)<<24 | key
	return idx
}

// binIndex is for a colour already moved by the dither error.
func (q *quantizer) binIndex(r, g, b uint8) uint8 {
	k := binOf(r, g, b)
	if v := q.bin[k]; v >= 0 {
		return uint8(v)
	}
	// Search from the bin's centre so every colour of the bin gets one answer.
	idx := q.nearest(int(r>>3)<<3|4, int(g>>3)<<3|4, int(b>>3)<<3|4)
	q.bin[k] = int16(idx)
	return idx
}

type gifEncoder struct {
	w             *countWriter
	width, height int
	fps           int
	q             *quantizer

	ref      []byte // BGRA of what the viewer sees after the frames so far
	frame    int
	errCur   []int32 // Floyd-Steinberg error, 3 channels per pixel, current row
	errNext  []int32
	pix      []byte
	lzwBlock blockWriter
}

type countWriter struct {
	w io.Writer
	n int64
}

func (c *countWriter) Write(b []byte) (int, error) {
	n, err := c.w.Write(b)
	c.n += int64(n)
	return n, err
}

// newGIFEncoder writes the GIF header to w. width and height are the size of
// every BGRA frame passed to addFrame.
func newGIFEncoder(w io.Writer, width, height, fps int, pal *palette) (*gifEncoder, error) {
	if width < 1 || height < 1 || width > 0xFFFF || height > 0xFFFF {
		return nil, errors.New("media: bad GIF size")
	}
	e := &gifEncoder{
		w: &countWriter{w: w}, width: width, height: height, fps: fps,
		q:       newQuantizer(pal),
		errCur:  make([]int32, (width+2)*3),
		errNext: make([]int32, (width+2)*3),
	}
	hdr := make([]byte, 0, 6+7+768+19)
	hdr = append(hdr, "GIF89a"...)
	// Logical screen: global colour table of 256 entries, 8 bits per channel.
	hdr = append(hdr, byte(width), byte(width>>8), byte(height), byte(height>>8), 0xF7, 0, 0)
	for _, c := range pal.rgb {
		hdr = append(hdr, c[0], c[1], c[2])
	}
	// Netscape extension: loop forever.
	hdr = append(hdr, 0x21, 0xFF, 0x0B)
	hdr = append(hdr, "NETSCAPE2.0"...)
	hdr = append(hdr, 0x03, 0x01, 0x00, 0x00, 0x00)
	if _, err := e.w.Write(hdr); err != nil {
		return nil, err
	}
	return e, nil
}

// delay returns the delay of frame k in 1/100 s. GIF cannot hold 1/12 s, so
// the rounding is spread over the frames and the total length stays right.
func (e *gifEncoder) delay(k int) int {
	r := func(k int) int { return int(math.Round(float64(k) * 100 / float64(e.fps))) }
	return max(r(k+1)-r(k), 1)
}

func unchanged(a, b []byte) bool {
	for c := 0; c < 3; c++ {
		d := int(a[c]) - int(b[c])
		if d > deltaTolerance || d < -deltaTolerance {
			return false
		}
	}
	return true
}

// addFrame appends one BGRA frame and returns the bytes it took.
func (e *gifEncoder) addFrame(bgra []byte) (int64, error) {
	if len(bgra) != e.width*e.height*4 {
		return 0, errors.New("media: frame size does not match the GIF size")
	}
	start := e.w.n

	// Bounding box of the pixels that differ from what is on screen.
	// A literal, not image.Rect: Rect would swap the inverted corners.
	bounds := image.Rectangle{Min: image.Point{X: e.width, Y: e.height}}
	if e.ref == nil {
		bounds = image.Rect(0, 0, e.width, e.height)
	} else {
		for y := 0; y < e.height; y++ {
			row := y * e.width * 4
			for x := 0; x < e.width; x++ {
				i := row + x*4
				if !unchanged(bgra[i:i+4], e.ref[i:i+4]) {
					bounds.Min.X, bounds.Max.X = min(bounds.Min.X, x), max(bounds.Max.X, x+1)
					bounds.Min.Y, bounds.Max.Y = min(bounds.Min.Y, y), max(bounds.Max.Y, y+1)
				}
			}
		}
	}
	var pix []byte
	if bounds.Empty() {
		// Nothing moved: a one-pixel transparent frame still keeps the timing.
		bounds = image.Rect(0, 0, 1, 1)
		pix = []byte{gifTransparent}
	} else {
		pix = e.dither(bgra, bounds)
	}

	bw, bh := bounds.Dx(), bounds.Dy()
	d := e.delay(e.frame)
	gce := []byte{0x21, 0xF9, 0x04, 0x05 /* keep, transparent */, byte(d), byte(d >> 8), gifTransparent, 0}
	desc := []byte{0x2C,
		byte(bounds.Min.X), byte(bounds.Min.X >> 8), byte(bounds.Min.Y), byte(bounds.Min.Y >> 8),
		byte(bw), byte(bw >> 8), byte(bh), byte(bh >> 8), 0, 8 /* LZW minimum code size */}
	if _, err := e.w.Write(append(gce, desc...)); err != nil {
		return 0, err
	}
	e.lzwBlock.w = e.w
	zw := lzw.NewWriter(&e.lzwBlock, lzw.LSB, 8)
	if _, err := zw.Write(pix); err != nil {
		return 0, err
	}
	if err := zw.Close(); err != nil {
		return 0, err
	}
	if err := e.lzwBlock.finish(); err != nil {
		return 0, err
	}
	e.frame++
	return e.w.n - start, nil
}

// dither quantises the changed pixels inside r with Floyd-Steinberg error
// diffusion and marks the others transparent. image/draw.FloydSteinberg
// searches the whole palette for every pixel, which is far too slow for
// 1080p, hence the cached quantizer and this own loop.
func (e *gifEncoder) dither(bgra []byte, r image.Rectangle) []byte {
	w, h := r.Dx(), r.Dy()
	if cap(e.pix) < w*h {
		e.pix = make([]byte, w*h)
	}
	out := e.pix[:w*h]
	first := e.ref == nil
	if first {
		e.ref = make([]byte, len(bgra))
	}
	clear(e.errCur)
	clear(e.errNext)

	for y := 0; y < h; y++ {
		sy := r.Min.Y + y
		for x := 0; x < w; x++ {
			sx := r.Min.X + x
			i := (sy*e.width + sx) * 4
			px := bgra[i : i+4]
			if !first && unchanged(px, e.ref[i:i+4]) {
				out[y*w+x] = gifTransparent
				continue
			}
			ec := e.errCur[(x+1)*3 : (x+1)*3+3]
			cr, cg, cb := int(px[2]), int(px[1]), int(px[0])
			ar, ag, ab := clamp8(cr+int(ec[0])/16), clamp8(cg+int(ec[1])/16), clamp8(cb+int(ec[2])/16)
			var idx uint8
			if ar == cr && ag == cg && ab == cb {
				idx = e.q.exactIndex(uint8(ar), uint8(ag), uint8(ab))
			} else {
				idx = e.q.binIndex(uint8(ar), uint8(ag), uint8(ab))
			}
			out[y*w+x] = idx
			copy(e.ref[i:i+4], px)

			p := e.q.pal.rgb[idx]
			er, eg, eb := ar-int(p[0]), ag-int(p[1]), ab-int(p[2])
			spread(e.errCur, e.errNext, x+1, er, eg, eb)
		}
		e.errCur, e.errNext = e.errNext, e.errCur
		clear(e.errNext)
	}
	return out
}

// spread distributes a pixel's quantisation error to its neighbours with the
// Floyd-Steinberg weights 7, 3, 5, 1 (out of 16). Row buffers are offset by
// one pixel so the left and right neighbours need no edge checks.
func spread(cur, next []int32, x, er, eg, eb int) {
	for c, e := range [3]int{er, eg, eb} {
		cur[(x+1)*3+c] += int32(e * 7)
		next[(x-1)*3+c] += int32(e * 3)
		next[x*3+c] += int32(e * 5)
		next[(x+1)*3+c] += int32(e)
	}
}

func clamp8(v int) int {
	if v < 0 {
		return 0
	}
	if v > 255 {
		return 255
	}
	return v
}

// close writes the trailer.
func (e *gifEncoder) close() error {
	_, err := e.w.Write([]byte{0x3B})
	return err
}

// blockWriter wraps data in GIF sub-blocks (at most 255 bytes each).
type blockWriter struct {
	w   io.Writer
	buf [256]byte // buf[0] is the length byte
	n   int
	err error
}

func (b *blockWriter) Write(p []byte) (int, error) {
	total := len(p)
	for len(p) > 0 && b.err == nil {
		c := copy(b.buf[1+b.n:256], p)
		b.n += c
		p = p[c:]
		if b.n == 255 {
			b.flush()
		}
	}
	return total, b.err
}

func (b *blockWriter) flush() {
	if b.n == 0 || b.err != nil {
		return
	}
	b.buf[0] = byte(b.n)
	_, b.err = b.w.Write(b.buf[:1+b.n])
	b.n = 0
}

// finish writes the pending bytes and the zero-length block that ends the
// image data.
func (b *blockWriter) finish() error {
	b.flush()
	if b.err == nil {
		_, b.err = b.w.Write([]byte{0})
	}
	err := b.err
	b.err = nil
	return err
}
