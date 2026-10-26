package tray

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
)

const iconSize = 32

var (
	colorWorking = color.NRGBA{R: 0x2e, G: 0xb8, B: 0x4b, A: 0xff}
	colorResting = color.NRGBA{R: 0x8a, G: 0x8a, B: 0x8a, A: 0xff}
)

// icon returns the tray icon: ICO on Windows, PNG elsewhere.
func icon(working bool) []byte {
	c := colorResting
	if working {
		c = colorWorking
	}
	return encodeIcon(dotPNG(c))
}

// dotPNG draws a filled circle on a transparent background.
func dotPNG(c color.NRGBA) []byte {
	img := image.NewNRGBA(image.Rect(0, 0, iconSize, iconSize))
	const r = iconSize/2 - 2
	for y := 0; y < iconSize; y++ {
		for x := 0; x < iconSize; x++ {
			dx, dy := 2*x+1-iconSize, 2*y+1-iconSize // doubled, centred coordinates
			if dx*dx+dy*dy <= 4*r*r {
				img.SetNRGBA(x, y, c)
			}
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		panic(err) // encoding to memory cannot fail
	}
	return buf.Bytes()
}
