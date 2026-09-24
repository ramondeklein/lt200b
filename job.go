package main

import (
	"encoding/binary"
	"fmt"
	"image"
	"image/color"

	xdraw "golang.org/x/image/draw"
)

const (
	printWidth = 32
	chunkSize  = 500
)

// createJob builds the GATT writes for one label. Each returned slice is one write.
func createJob(img image.Image) ([][]byte, error) {
	packed, width, height, err := prepareImage(img)
	if err != nil {
		return nil, err
	}
	if width*height != len(packed)*8 {
		return nil, fmt.Errorf("data does not match dimensions (%d*%d!=%d)", width, height, len(packed)*8)
	}

	body := make([]byte, 0, 16+len(packed))
	body = append(body, startJob()...)
	body = append(body, printData(packed, width, height)...)
	body = append(body, 0x1B, 0x45) // form feed
	body = append(body, 0x1B, 0x41) // status
	body = append(body, 0x1B, 0x51) // end

	chunks, err := splitChunks(body, chunkSize)
	if err != nil {
		return nil, err
	}
	return append([][]byte{headerBytes(len(body))}, chunks...), nil
}

// prepareImage converts img the way the printer expects: 1-bit, rotated
// clockwise, scaled to 32 pixels across the tape. The returned width and
// height are the values the print-data command carries. The Python client
// returns the resized dimensions swapped, and the printer was built against
// that layout.
func prepareImage(img image.Image) (packed []byte, width, height int, err error) {
	bounds := img.Bounds()
	srcW, srcH := bounds.Dx(), bounds.Dy()
	if srcW == 0 || srcH == 0 {
		return nil, 0, 0, fmt.Errorf("image is empty")
	}

	// Rotate -90 degrees (clockwise) with expand. new(x, y) = old(y, srcH-1-x).
	rotated := image.NewGray(image.Rect(0, 0, srcH, srcW))
	for y := 0; y < srcH; y++ {
		for x := 0; x < srcW; x++ {
			nx := srcH - 1 - y
			ny := x
			if luma(img.At(bounds.Min.X+x, bounds.Min.Y+y)) < 128 {
				rotated.SetGray(nx, ny, color.Gray{Y: 0})
			} else {
				rotated.SetGray(nx, ny, color.Gray{Y: 255})
			}
		}
	}

	rotW, rotH := rotated.Bounds().Dx(), rotated.Bounds().Dy()
	scaledH := int(64 * float64(rotH) / float64(rotW))
	if scaledH < 1 {
		return nil, 0, 0, fmt.Errorf("image is too small to print")
	}
	scaled := image.NewGray(image.Rect(0, 0, printWidth, scaledH))
	xdraw.CatmullRom.Scale(scaled, scaled.Bounds(), rotated, rotated.Bounds(), xdraw.Over, nil)

	bits := make([]byte, 0, printWidth*scaledH)
	for y := 0; y < scaledH; y++ {
		for x := 0; x < printWidth; x++ {
			if scaled.GrayAt(x, y).Y < 128 {
				bits = append(bits, 1)
			} else {
				bits = append(bits, 0)
			}
		}
	}
	return packBits(bits), scaledH, printWidth, nil
}

func luma(c color.Color) uint32 {
	r, g, b, _ := c.RGBA()
	// Same integer coefficients Pillow uses for RGB to luminance.
	r8, g8, b8 := r>>8, g>>8, b>>8
	return (r8*19595 + g8*38470 + b8*7471 + 0x8000) >> 16
}

// packBits packs 0/1 pixels least-significant-bit first, matching numpy.packbits(bitorder="little").
func packBits(bits []byte) []byte {
	out := make([]byte, (len(bits)+7)/8)
	for i, bit := range bits {
		if bit != 0 {
			out[i/8] |= 1 << (uint(i) % 8)
		}
	}
	return out
}

func headerBytes(length int) []byte {
	header := []byte{0xFF, 0xF0, 0x12, 0x34, 0, 0, 0, 0}
	binary.LittleEndian.PutUint32(header[4:8], uint32(length))
	var sum byte
	for _, b := range header {
		sum += b
	}
	return append(header, sum)
}

func startJob() []byte {
	return []byte{0x1B, 0x73, 0x9A, 0x02, 0x00, 0x00}
}

func printData(data []byte, width, height int) []byte {
	out := []byte{0x1B, 0x44, 0x01, 0x02, 0, 0, 0, 0, 0, 0, 0, 0}
	binary.LittleEndian.PutUint32(out[4:8], uint32(width))
	binary.LittleEndian.PutUint32(out[8:12], uint32(height))
	return append(out, data...)
}

func splitChunks(data []byte, size int) ([][]byte, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("print job is empty")
	}
	var chunks [][]byte
	for i := 0; i < len(data); i += size {
		index := i / size
		if index > 0xFF {
			return nil, fmt.Errorf("print job is too long")
		}
		end := i + size
		if end > len(data) {
			end = len(data)
		}
		chunk := make([]byte, 0, 1+end-i+2)
		chunk = append(chunk, byte(index))
		chunk = append(chunk, data[i:end]...)
		chunks = append(chunks, chunk)
	}
	last := len(chunks) - 1
	chunks[last] = append(chunks[last], 0x12, 0x34)
	return chunks, nil
}
