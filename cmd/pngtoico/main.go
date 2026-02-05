// PNG to ICO converter with proper resizing
// Run: go run ./cmd/pngtoico <input.png> <output.ico>

package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"image"
	"image/png"
	"os"

	"golang.org/x/image/draw"
)

func main() {
	if len(os.Args) < 3 {
		fmt.Println("Usage: pngtoico <input.png> <output.ico>")
		os.Exit(1)
	}

	inputPath := os.Args[1]
	outputPath := os.Args[2]

	// Read and decode PNG
	pngFile, err := os.Open(inputPath)
	if err != nil {
		fmt.Printf("Error opening PNG: %v\n", err)
		os.Exit(1)
	}
	defer pngFile.Close()

	srcImg, err := png.Decode(pngFile)
	if err != nil {
		fmt.Printf("Error decoding PNG: %v\n", err)
		os.Exit(1)
	}

	// Create multiple sizes for proper Windows icon
	sizes := []int{256, 48, 32, 16}
	var images [][]byte
	var dimensions []int

	for _, size := range sizes {
		// Resize image
		resized := resizeImage(srcImg, size, size)

		// Encode to PNG
		var buf bytes.Buffer
		if err := png.Encode(&buf, resized); err != nil {
			fmt.Printf("Error encoding PNG for size %d: %v\n", size, err)
			os.Exit(1)
		}

		images = append(images, buf.Bytes())
		dimensions = append(dimensions, size)
	}

	// Create ICO file with multiple images
	icoData := createMultiICO(images, dimensions)

	// Write ICO file
	if err := os.WriteFile(outputPath, icoData, 0644); err != nil {
		fmt.Printf("Error writing ICO: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Created %s with sizes: %v\n", outputPath, sizes)
}

// resizeImage resizes an image to the specified dimensions
func resizeImage(src image.Image, width, height int) image.Image {
	dst := image.NewRGBA(image.Rect(0, 0, width, height))
	draw.CatmullRom.Scale(dst, dst.Bounds(), src, src.Bounds(), draw.Over, nil)
	return dst
}

// createMultiICO creates an ICO file with multiple image sizes
func createMultiICO(images [][]byte, dimensions []int) []byte {
	var buf bytes.Buffer

	numImages := len(images)

	// ICO Header (6 bytes)
	binary.Write(&buf, binary.LittleEndian, uint16(0))           // Reserved
	binary.Write(&buf, binary.LittleEndian, uint16(1))           // Type: 1 = ICO
	binary.Write(&buf, binary.LittleEndian, uint16(numImages))   // Number of images

	// Calculate offset for first image (after header + all directory entries)
	// Header = 6 bytes, each directory entry = 16 bytes
	offset := uint32(6 + numImages*16)

	// Write directory entries
	for i, imgData := range images {
		size := dimensions[i]

		// Width (0 means 256)
		if size >= 256 {
			buf.WriteByte(0)
		} else {
			buf.WriteByte(byte(size))
		}

		// Height (0 means 256)
		if size >= 256 {
			buf.WriteByte(0)
		} else {
			buf.WriteByte(byte(size))
		}

		buf.WriteByte(0)                                              // Color palette
		buf.WriteByte(0)                                              // Reserved
		binary.Write(&buf, binary.LittleEndian, uint16(1))            // Color planes
		binary.Write(&buf, binary.LittleEndian, uint16(32))           // Bits per pixel
		binary.Write(&buf, binary.LittleEndian, uint32(len(imgData))) // Size of image data
		binary.Write(&buf, binary.LittleEndian, offset)               // Offset to image data

		offset += uint32(len(imgData))
	}

	// Write image data
	for _, imgData := range images {
		buf.Write(imgData)
	}

	return buf.Bytes()
}
