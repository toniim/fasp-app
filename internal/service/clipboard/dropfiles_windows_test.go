//go:build windows

package clipboard

import (
	"encoding/binary"
	"testing"
	"unicode/utf16"
)

func TestDropFiles(t *testing.T) {
	b := dropFiles([]string{`C:\a.mp4`, `D:\bé.mp4`})
	if off := binary.LittleEndian.Uint32(b[0:]); off != 20 {
		t.Fatalf("pFiles = %d", off)
	}
	if wide := binary.LittleEndian.Uint32(b[16:]); wide != 1 {
		t.Fatalf("fWide = %d", wide)
	}
	u := make([]uint16, (len(b)-20)/2)
	for i := range u {
		u[i] = binary.LittleEndian.Uint16(b[20+i*2:])
	}
	got := string(utf16.Decode(u))
	want := `C:\a.mp4` + "\x00" + `D:\bé.mp4` + "\x00\x00"
	if got != want {
		t.Errorf("list = %q, want %q", got, want)
	}
}
