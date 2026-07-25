// icon.go : the M.A.X. platform icon loader - sets the Ebitengine window icon from the embedded 'max-go.ico' PNG frames.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package platform

import (
	"image"
	"image/color"

	"github.com/suglasp/max-go/internal/icon"
)

// windowIcons returns the embedded max-go.ico frames, or procedural reticles when the icon cannot be decoded. The OS selects the best size.
func windowIcons() []image.Image {
	if imgs := icon.Images(); len(imgs) > 0 {
		return imgs
	}
	return []image.Image{drawMaxIcon(64), drawMaxIcon(32), drawMaxIcon(16)}
}

var (
	iconBack  = color.RGBA{R: 0x10, G: 0x14, B: 0x18, A: 0xFF} // dark slate field
	iconGreen = color.RGBA{R: 0x24, G: 0xD8, B: 0x24, A: 0xFF} // M.A.X. UI lime
)

// drawMaxIcon renders a square fallback icon with a dark field, green corner brackets, and a center dot.
func drawMaxIcon(size int) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, size, size))
	fillRect(img, 0, 0, size, size, iconBack)

	t := size / 10 // stroke thickness
	if t < 1 {
		t = 1
	}
	m := size / 8     // inset margin from the edge
	arm := size/3 + 1 // bracket arm length

	// Four corner brackets.
	fillRect(img, m, m, m+arm, m+t, iconGreen) // top left
	fillRect(img, m, m, m+t, m+arm, iconGreen)
	fillRect(img, size-m-arm, m, size-m, m+t, iconGreen) // top right
	fillRect(img, size-m-t, m, size-m, m+arm, iconGreen)
	fillRect(img, m, size-m-t, m+arm, size-m, iconGreen) // bottom left
	fillRect(img, m, size-m-arm, m+t, size-m, iconGreen)
	fillRect(img, size-m-arm, size-m-t, size-m, size-m, iconGreen) // bottom right
	fillRect(img, size-m-t, size-m-arm, size-m, size-m, iconGreen)

	// Centre dot.
	c := size / 2
	fillRect(img, c-t, c-t, c+t, c+t, iconGreen)
	return img
}

// fillRect paints a solid rectangle [x0,x1) x [y0,y1), clipped to the image.
func fillRect(img *image.RGBA, x0, y0, x1, y1 int, c color.RGBA) {
	b := img.Bounds()
	if x0 < b.Min.X {
		x0 = b.Min.X
	}
	if y0 < b.Min.Y {
		y0 = b.Min.Y
	}
	if x1 > b.Max.X {
		x1 = b.Max.X
	}
	if y1 > b.Max.Y {
		y1 = b.Max.Y
	}
	for y := y0; y < y1; y++ {
		for x := x0; x < x1; x++ {
			img.SetRGBA(x, y, c)
		}
	}
}
