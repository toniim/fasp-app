package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"image"
	"image/png"
	"io"
	"os"
	"path/filepath"

	"golang.org/x/image/draw"
)

func main() {
	sourceLogoPath := `c:\Users\Admin\ws\fasp\logo.png`
	buildDir := `c:\Users\Admin\ws\fasp-app\build`
	frontendDir := `c:\Users\Admin\ws\fasp-app\frontend\src\assets\images`

	fmt.Printf("Reading source logo from %s...\n", sourceLogoPath)
	file, err := os.Open(sourceLogoPath)
	if err != nil {
		fmt.Printf("Error opening source logo: %v\n", err)
		os.Exit(1)
	}
	defer file.Close()

	srcImg, err := png.Decode(file)
	if err != nil {
		fmt.Printf("Error decoding source logo: %v\n", err)
		os.Exit(1)
	}

	bounds := srcImg.Bounds()
	fmt.Printf("Source logo decoded. Size: %dx%d\n", bounds.Dx(), bounds.Dy())

	// 1. Generate build/appicon.png (512x512)
	fmt.Println("Generating appicon.png...")
	appIconData, err := resizePNG(srcImg, 512)
	if err != nil {
		fmt.Printf("Error resizing to 512: %v\n", err)
		os.Exit(1)
	}
	err = os.WriteFile(filepath.Join(buildDir, "appicon.png"), appIconData, 0644)
	if err != nil {
		fmt.Printf("Error writing appicon.png: %v\n", err)
		os.Exit(1)
	}

	// 2. Generate frontend/src/assets/images/logo-universal.png (512x512)
	fmt.Println("Generating logo-universal.png...")
	err = os.WriteFile(filepath.Join(frontendDir, "logo-universal.png"), appIconData, 0644)
	if err != nil {
		fmt.Printf("Error writing logo-universal.png: %v\n", err)
		os.Exit(1)
	}

	// 3. Generate build/trayicon.png (32x32)
	fmt.Println("Generating build/trayicon.png...")
	trayIcon32Data, err := resizePNG(srcImg, 32)
	if err != nil {
		fmt.Printf("Error resizing to 32: %v\n", err)
		os.Exit(1)
	}
	err = os.WriteFile(filepath.Join(buildDir, "trayicon.png"), trayIcon32Data, 0644)
	if err != nil {
		fmt.Printf("Error writing trayicon.png: %v\n", err)
		os.Exit(1)
	}

	// 4. Generate build/darwin/trayicon.png (32x32)
	fmt.Println("Generating build/darwin/trayicon.png...")
	err = os.WriteFile(filepath.Join(buildDir, "darwin", "trayicon.png"), trayIcon32Data, 0644)
	if err != nil {
		fmt.Printf("Error writing build/darwin/trayicon.png: %v\n", err)
		os.Exit(1)
	}

	// 5. Generate build/windows/icon.ico (multi-resolution: 16, 32, 48, 64, 128, 256)
	fmt.Println("Generating build/windows/icon.ico...")
	icoSizes := []int{16, 32, 48, 64, 128, 256}
	var icoPNGs [][]byte
	for _, sz := range icoSizes {
		pngBytes, err := resizePNG(srcImg, sz)
		if err != nil {
			fmt.Printf("Error resizing for ICO size %d: %v\n", sz, err)
			os.Exit(1)
		}
		icoPNGs = append(icoPNGs, pngBytes)
	}
	icoFile, err := os.Create(filepath.Join(buildDir, "windows", "icon.ico"))
	if err != nil {
		fmt.Printf("Error creating icon.ico: %v\n", err)
		os.Exit(1)
	}
	defer icoFile.Close()
	err = writeICO(icoFile, icoPNGs)
	if err != nil {
		fmt.Printf("Error writing icon.ico: %v\n", err)
		os.Exit(1)
	}

	// 6. Generate build/windows/trayicon.ico (multi-resolution: 16, 32, 48)
	fmt.Println("Generating build/windows/trayicon.ico...")
	trayIcoSizes := []int{16, 32, 48}
	var trayIcoPNGs [][]byte
	for _, sz := range trayIcoSizes {
		pngBytes, err := resizePNG(srcImg, sz)
		if err != nil {
			fmt.Printf("Error resizing for tray ICO size %d: %v\n", sz, err)
			os.Exit(1)
		}
		trayIcoPNGs = append(trayIcoPNGs, pngBytes)
	}
	trayIcoFile, err := os.Create(filepath.Join(buildDir, "windows", "trayicon.ico"))
	if err != nil {
		fmt.Printf("Error creating trayicon.ico: %v\n", err)
		os.Exit(1)
	}
	defer trayIcoFile.Close()
	err = writeICO(trayIcoFile, trayIcoPNGs)
	if err != nil {
		fmt.Printf("Error writing trayicon.ico: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("All icons generated successfully!")
}

// resizePNG scales an image to size x size using bilinear interpolation and returns PNG bytes
func resizePNG(src image.Image, size int) ([]byte, error) {
	rect := image.Rect(0, 0, size, size)
	dst := image.NewRGBA(rect)
	draw.BiLinear.Scale(dst, rect, src, src.Bounds(), draw.Src, nil)

	var buf bytes.Buffer
	err := png.Encode(&buf, dst)
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// writeICO packages PNG-encoded image slices into a single ICO file
func writeICO(w io.Writer, pngs [][]byte) error {
	if len(pngs) == 0 {
		return fmt.Errorf("no images to write")
	}

	// Header: Reserved (2 bytes), Type (2 bytes), Count (2 bytes)
	if err := binary.Write(w, binary.LittleEndian, uint16(0)); err != nil {
		return err
	}
	if err := binary.Write(w, binary.LittleEndian, uint16(1)); err != nil {
		return err
	}
	if err := binary.Write(w, binary.LittleEndian, uint16(len(pngs))); err != nil {
		return err
	}

	// Directory entries start at offset 6 (size of header)
	// Directory entry is 16 bytes. All directory entries are written together.
	offset := uint32(6 + len(pngs)*16)

	for _, data := range pngs {
		img, err := png.Decode(bytes.NewReader(data))
		if err != nil {
			return fmt.Errorf("failed to decode PNG for size calculation: %w", err)
		}
		bounds := img.Bounds()
		width := bounds.Dx()
		height := bounds.Dy()

		var wByte, hByte byte
		if width >= 256 {
			wByte = 0
		} else {
			wByte = byte(width)
		}
		if height >= 256 {
			hByte = 0
		} else {
			hByte = byte(height)
		}

		// Write Directory Entry:
		// Width (1 byte), Height (1 byte), ColorCount (1 byte), Reserved (1 byte),
		// Planes (2 bytes), BitCount (2 bytes), BytesInRes (4 bytes), ImageOffset (4 bytes)
		if err := binary.Write(w, binary.LittleEndian, wByte); err != nil {
			return err
		}
		if err := binary.Write(w, binary.LittleEndian, hByte); err != nil {
			return err
		}
		if err := binary.Write(w, binary.LittleEndian, byte(0)); err != nil { // 0 for >= 256 colors
			return err
		}
		if err := binary.Write(w, binary.LittleEndian, byte(0)); err != nil { // Reserved must be 0
			return err
		}
		if err := binary.Write(w, binary.LittleEndian, uint16(1)); err != nil { // Planes (typically 1)
			return err
		}
		if err := binary.Write(w, binary.LittleEndian, uint16(32)); err != nil { // 32 bits/pixel (RGBA)
			return err
		}
		if err := binary.Write(w, binary.LittleEndian, uint32(len(data))); err != nil { // Bytes in resource
			return err
		}
		if err := binary.Write(w, binary.LittleEndian, offset); err != nil { // Image data offset
			return err
		}

		offset += uint32(len(data))
	}

	// Append raw PNG data bytes
	for _, data := range pngs {
		if _, err := w.Write(data); err != nil {
			return err
		}
	}

	return nil
}
