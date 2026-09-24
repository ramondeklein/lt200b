package main

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/color"
	"testing"
)

func TestPackBitsLittleEndian(t *testing.T) {
	bits := []byte{1, 0, 1, 1, 0, 0, 0, 1, 1, 1, 1, 1, 0, 0, 0, 0, 1}
	got := packBits(bits)
	want := []byte{0x8D, 0x0F, 0x01}
	if !bytes.Equal(got, want) {
		t.Fatalf("packBits = %x, want %x", got, want)
	}
}

func TestSplitChunksAppendsMagicToLast(t *testing.T) {
	data := make([]byte, 600)
	for i := range data {
		data[i] = byte(i)
	}
	chunks, err := splitChunks(data, 500)
	if err != nil {
		t.Fatal(err)
	}
	if len(chunks) != 2 {
		t.Fatalf("got %d chunks", len(chunks))
	}
	if chunks[0][0] != 0 || len(chunks[0]) != 501 || !bytes.Equal(chunks[0][1:], data[:500]) {
		t.Fatalf("first chunk = %d bytes, index %d", len(chunks[0]), chunks[0][0])
	}
	if chunks[1][0] != 1 || !bytes.Equal(chunks[1][1:len(chunks[1])-2], data[500:]) {
		t.Fatal("second chunk payload mismatch")
	}
	if chunks[1][len(chunks[1])-2] != 0x12 || chunks[1][len(chunks[1])-1] != 0x34 {
		t.Fatal("last chunk missing magic")
	}
}

func TestCreateJobSolidBlack(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 8, 4))
	for y := 0; y < 4; y++ {
		for x := 0; x < 8; x++ {
			img.Set(x, y, color.Black)
		}
	}
	chunks, err := createJob(img)
	if err != nil {
		t.Fatal(err)
	}
	body := reconstructBody(t, chunks)

	// Rotated 4x8, scaled to 32x128. Protocol dimensions are swapped: 128 x 32.
	const packedLen = 32 * 128 / 8
	if len(body) != 6+12+packedLen+2+2+2 {
		t.Fatalf("body length %d", len(body))
	}
	if !bytes.Equal(body[:6], []byte{0x1B, 0x73, 0x9A, 0x02, 0x00, 0x00}) {
		t.Fatalf("start = %x", body[:6])
	}
	if !bytes.Equal(body[6:10], []byte{0x1B, 0x44, 0x01, 0x02}) {
		t.Fatalf("print header = %x", body[6:10])
	}
	if binary.LittleEndian.Uint32(body[10:14]) != 128 || binary.LittleEndian.Uint32(body[14:18]) != 32 {
		t.Fatalf("dimensions %d x %d", binary.LittleEndian.Uint32(body[10:14]), binary.LittleEndian.Uint32(body[14:18]))
	}
	for _, b := range body[18 : 18+packedLen] {
		if b != 0xFF {
			t.Fatal("expected solid black pixels")
		}
	}
	tail := body[18+packedLen:]
	if !bytes.Equal(tail, []byte{0x1B, 0x45, 0x1B, 0x41, 0x1B, 0x51}) {
		t.Fatalf("tail = %x", tail)
	}
}

func TestCreateTextImage(t *testing.T) {
	img, err := createTextImage("Hi", 32)
	if err != nil {
		t.Fatal(err)
	}
	if img.Bounds().Dy() != textImageHeight || img.Bounds().Dx() < 1 {
		t.Fatalf("size %v", img.Bounds())
	}
	ink := false
	b := img.Bounds()
	for y := b.Min.Y; y < b.Max.Y && !ink; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			if luma(img.At(x, y)) < 128 {
				ink = true
				break
			}
		}
	}
	if !ink {
		t.Fatal("text image has no dark pixels")
	}
}

func TestMatchPrinter(t *testing.T) {
	if !matchPrinter("38:44:BE:61:DD:F2", "689C0EDC-37F8-9D34-5029-18341A41F5A5", "Letratag3844BE61DDF2", true) {
		t.Fatal("macOS name match failed")
	}
	if matchPrinter("38:44:BE:61:DD:F2", "689C0EDC-37F8-9D34-5029-18341A41F5A5", "LetratagDC5475123456", true) {
		t.Fatal("macOS matched a different MAC")
	}
	if !matchPrinter("38:44:be:61:dd:f2", "38:44:BE:61:DD:F2", "", false) {
		t.Fatal("address match failed")
	}
	if !isMACAddress("38-44-be-61-dd-f2") || isMACAddress("689C0EDC-37F8-9D34-5029-18341A41F5A5") {
		t.Fatal("MAC detection failed")
	}
}

func reconstructBody(t *testing.T, chunks [][]byte) []byte {
	t.Helper()
	if len(chunks) < 2 {
		t.Fatal("expected a header and at least one body chunk")
	}
	header := chunks[0]
	if len(header) != 9 || !bytes.Equal(header[:4], []byte{0xFF, 0xF0, 0x12, 0x34}) {
		t.Fatalf("header %x", header)
	}
	var sum byte
	for _, b := range header[:8] {
		sum += b
	}
	if header[8] != sum {
		t.Fatalf("checksum %x, want %x", header[8], sum)
	}

	var body []byte
	for i, chunk := range chunks[1:] {
		if int(chunk[0]) != i {
			t.Fatalf("chunk index %d, want %d", chunk[0], i)
		}
		payload := chunk[1:]
		if i == len(chunks)-2 {
			if len(payload) < 2 || payload[len(payload)-2] != 0x12 || payload[len(payload)-1] != 0x34 {
				t.Fatal("missing trailing magic")
			}
			payload = payload[:len(payload)-2]
		}
		body = append(body, payload...)
	}
	if binary.LittleEndian.Uint32(header[4:8]) != uint32(len(body)) {
		t.Fatalf("header length %d, body %d", binary.LittleEndian.Uint32(header[4:8]), len(body))
	}
	return body
}
