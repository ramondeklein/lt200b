package main

import (
	"fmt"
	"image"
	"image/color"
	"image/draw"

	"golang.org/x/image/font"
	"golang.org/x/image/font/gofont/goregular"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"
)

const textImageHeight = 64

func createTextImage(text string, fontSize int) (image.Image, error) {
	if text == "" {
		return nil, fmt.Errorf("text is empty")
	}
	if fontSize < 1 {
		return nil, fmt.Errorf("font size must be positive")
	}

	parsed, err := opentype.Parse(goregular.TTF)
	if err != nil {
		return nil, err
	}
	face, err := opentype.NewFace(parsed, &opentype.FaceOptions{
		Size:    float64(fontSize),
		DPI:     72,
		Hinting: font.HintingNone,
	})
	if err != nil {
		return nil, err
	}
	defer face.Close()

	advance := font.MeasureString(face, text)
	width := advance.Floor()
	if width < 1 {
		width = 1
	}

	img := image.NewRGBA(image.Rect(0, 0, width, textImageHeight))
	draw.Draw(img, img.Bounds(), image.NewUniform(color.White), image.Point{}, draw.Src)

	bounds, _ := font.BoundString(face, text)
	// Anchor "lm": the left-middle of the text sits at (0, 32).
	originX := -bounds.Min.X
	midY := bounds.Min.Y + (bounds.Max.Y-bounds.Min.Y)/2
	originY := fixed.I(textImageHeight/2) - midY

	drawer := &font.Drawer{
		Dst:  img,
		Src:  image.Black,
		Face: face,
		Dot:  fixed.Point26_6{X: originX, Y: originY},
	}
	drawer.DrawString(text)
	return img, nil
}
