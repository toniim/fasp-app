//go:build windows

package clipboard

import (
	"encoding/binary"
	"unicode/utf16"
)

// dropFiles builds a CF_HDROP payload: a DROPFILES header followed by the
// wide-character paths, each NUL-terminated, plus a final NUL.
func dropFiles(paths []string) []byte {
	const header = 20 // sizeof(DROPFILES): pFiles, pt.x, pt.y, fNC, fWide
	var chars []uint16
	for _, p := range paths {
		chars = append(chars, utf16.Encode([]rune(p))...)
		chars = append(chars, 0)
	}
	chars = append(chars, 0)

	buf := make([]byte, header+len(chars)*2)
	binary.LittleEndian.PutUint32(buf[0:], header) // pFiles: offset of the list
	binary.LittleEndian.PutUint32(buf[16:], 1)     // fWide: UTF-16 paths
	for i, c := range chars {
		binary.LittleEndian.PutUint16(buf[header+i*2:], c)
	}
	return buf
}
