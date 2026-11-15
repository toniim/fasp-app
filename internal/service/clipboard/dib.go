package clipboard

import (
	"encoding/binary"
	"errors"
	"fmt"
	"image"
	"math/bits"
)

const (
	bitmapInfoHeaderSize = 40

	biRGB       = 0
	biBitfields = 3

	// Largest decoded image accepted from the clipboard, in pixels. Guards
	// against a hostile or corrupt header asking for gigabytes.
	maxDIBPixels = 1 << 28
)

// dibToImage decodes a packed CF_DIB/CF_DIBV5 block (header, optional colour
// masks, pixel rows) into an NRGBA image. It handles BI_RGB 24/32 bpp and
// BI_BITFIELDS 32 bpp, bottom-up and top-down. Palette and 16 bpp formats are
// rejected.
func dibToImage(dib []byte) (*image.NRGBA, error) {
	le := binary.LittleEndian
	if len(dib) < bitmapInfoHeaderSize {
		return nil, errors.New("dib: block shorter than header")
	}
	hdrSize := int(le.Uint32(dib[0:]))
	if hdrSize < bitmapInfoHeaderSize || hdrSize > len(dib) {
		return nil, fmt.Errorf("dib: bad header size %d", hdrSize)
	}
	w := int(int32(le.Uint32(dib[4:])))
	rawH := int(int32(le.Uint32(dib[8:])))
	bpp := int(le.Uint16(dib[14:]))
	comp := le.Uint32(dib[16:])
	clrUsed := int(le.Uint32(dib[32:]))

	h, topDown := rawH, false
	if rawH < 0 {
		h, topDown = -rawH, true
	}
	if w <= 0 || h <= 0 || w*h > maxDIBPixels {
		return nil, fmt.Errorf("dib: bad size %dx%d", w, rawH)
	}
	if le.Uint16(dib[12:]) != 1 {
		return nil, errors.New("dib: planes != 1")
	}
	if bpp != 24 && bpp != 32 {
		return nil, fmt.Errorf("dib: unsupported bit depth %d", bpp)
	}

	// Channel masks, default to BGRA byte order (BI_RGB 32 bpp).
	rMask, gMask, bMask, aMask := uint32(0x00ff0000), uint32(0x0000ff00), uint32(0x000000ff), uint32(0)
	off := hdrSize
	switch comp {
	case biRGB:
		if bpp == 32 {
			aMask = 0xff000000
		}
		// Pixel data follows the header (and any colour table, unused here).
		off += clrUsed * 4
	case biBitfields:
		if bpp != 32 {
			return nil, errors.New("dib: bitfields only supported at 32 bpp")
		}
		if hdrSize == bitmapInfoHeaderSize {
			// Masks sit right after the 40-byte header.
			if len(dib) < off+12 {
				return nil, errors.New("dib: truncated colour masks")
			}
			rMask, gMask, bMask = le.Uint32(dib[off:]), le.Uint32(dib[off+4:]), le.Uint32(dib[off+8:])
			off += 12
		} else {
			// V4/V5 headers carry the masks inside the header.
			if hdrSize < 56 {
				return nil, fmt.Errorf("dib: bad header size %d for bitfields", hdrSize)
			}
			rMask, gMask, bMask, aMask = le.Uint32(dib[40:]), le.Uint32(dib[44:]), le.Uint32(dib[48:]), le.Uint32(dib[52:])
		}
	default:
		return nil, fmt.Errorf("dib: unsupported compression %d", comp)
	}

	stride := ((w*bpp + 31) / 32) * 4
	if off < 0 || len(dib)-off < stride*h {
		return nil, errors.New("dib: truncated pixel data")
	}
	pix := dib[off:]

	rs, gs, bs, as := maskShift(rMask), maskShift(gMask), maskShift(bMask), maskShift(aMask)
	rm, gm, bm, am := maskMax(rMask), maskMax(gMask), maskMax(bMask), maskMax(aMask)
	// A 32 bpp bitmap without an alpha mask (plain BI_RGB, or a zero alpha
	// mask) has no usable alpha: treat it as opaque.
	hasAlpha := aMask != 0

	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		srcY := h - 1 - y
		if topDown {
			srcY = y
		}
		row := pix[srcY*stride:]
		dst := img.Pix[y*img.Stride:]
		for x := 0; x < w; x++ {
			var v uint32
			if bpp == 32 {
				v = le.Uint32(row[x*4:])
			} else {
				v = uint32(row[x*3]) | uint32(row[x*3+1])<<8 | uint32(row[x*3+2])<<16
			}
			dst[x*4+0] = scaleChannel(v, rs, rm)
			dst[x*4+1] = scaleChannel(v, gs, gm)
			dst[x*4+2] = scaleChannel(v, bs, bm)
			if hasAlpha {
				dst[x*4+3] = scaleChannel(v, as, am)
			} else {
				dst[x*4+3] = 0xff
			}
		}
	}
	// Many apps leave the unused fourth byte of 32 bpp BI_RGB at zero. An
	// image that is fully transparent everywhere is never what was copied, so
	// treat all-zero alpha as opaque.
	if bpp == 32 && comp == biRGB && !hasVisibleAlpha(img) {
		for i := 3; i < len(img.Pix); i += 4 {
			img.Pix[i] = 0xff
		}
	}
	return img, nil
}

func hasVisibleAlpha(img *image.NRGBA) bool {
	for i := 3; i < len(img.Pix); i += 4 {
		if img.Pix[i] != 0 {
			return true
		}
	}
	return false
}

func maskShift(m uint32) uint {
	if m == 0 {
		return 0
	}
	return uint(bits.TrailingZeros32(m))
}

// maskMax is the largest value the mask can hold once shifted down.
func maskMax(m uint32) uint32 {
	if m == 0 {
		return 0
	}
	return m >> maskShift(m)
}

func scaleChannel(v uint32, shift uint, max uint32) byte {
	if max == 0 {
		return 0
	}
	c := (v >> shift) & max
	if max == 0xff {
		return byte(c)
	}
	return byte((c*255 + max/2) / max)
}

// imageToDIB encodes an image as a packed CF_DIB block: a BITMAPINFOHEADER
// followed by bottom-up 32 bpp BGRA rows, the layout Windows apps read from
// the clipboard.
func imageToDIB(img image.Image) []byte {
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	out := make([]byte, bitmapInfoHeaderSize+w*h*4)

	le := binary.LittleEndian
	le.PutUint32(out[0:], bitmapInfoHeaderSize) // biSize
	le.PutUint32(out[4:], uint32(w))            // biWidth
	le.PutUint32(out[8:], uint32(h))            // biHeight > 0: bottom-up
	le.PutUint16(out[12:], 1)                   // biPlanes
	le.PutUint16(out[14:], 32)                  // biBitCount
	le.PutUint32(out[16:], 0)                   // biCompression = BI_RGB
	le.PutUint32(out[20:], uint32(w*h*4))       // biSizeImage

	pix := out[bitmapInfoHeaderSize:]
	for y := 0; y < h; y++ {
		row := pix[(h-1-y)*w*4:]
		for x := 0; x < w; x++ {
			r, g, bl, a := img.At(b.Min.X+x, b.Min.Y+y).RGBA()
			row[x*4+0] = byte(bl >> 8)
			row[x*4+1] = byte(g >> 8)
			row[x*4+2] = byte(r >> 8)
			row[x*4+3] = byte(a >> 8)
		}
	}
	return out
}
