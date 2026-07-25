// buffer.go : defines 8-bit indexed pixel buffers and subviews.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
// C++ ported : grbuf.c, rect.c, color.cpp - GNW graphics buffer and palette
package gfx

// Buffer is an 8-bit indexed pixel surface. Pixel (x, y) lives at
// Pix[y*Stride+x]. A Buffer may be a view into a larger buffer
// (Stride > W), which is how the original code passes a sub rectangle
// via a pointer plus its "full" pitch.
//
// Like the C primitives, drawing does not clip: coordinates outside
// the buffer panic (instead of corrupting memory as the original
// would). Callers clip with Rect.Intersect first.
type Buffer struct {
	W, H   int
	Stride int
	Pix    []uint8
}

// New allocates a wxh buffer.
func New(w, h int) *Buffer {
	if w < 0 || h < 0 {
		panic("gfx: negative buffer size")
	}
	return &Buffer{W: w, H: h, Stride: w, Pix: make([]uint8, w*h)}
}

// FromPix wraps existing pixel data (len must be at least w*h).
func FromPix(pix []uint8, w, h int) *Buffer {
	if len(pix) < w*h {
		panic("gfx: pixel slice too short")
	}
	return &Buffer{W: w, H: h, Stride: w, Pix: pix}
}

// Sub returns a wxh view at (x, y) sharing b's pixels. Drawing into
// the view draws into b.
func (b *Buffer) Sub(x, y, w, h int) *Buffer {
	if x < 0 || y < 0 || w < 0 || h < 0 || x+w > b.W || y+h > b.H {
		panic("gfx: Sub out of bounds")
	}
	return &Buffer{W: w, H: h, Stride: b.Stride, Pix: b.Pix[y*b.Stride+x:]}
}

// SubRect returns the view covering r, corners inclusive.
func (b *Buffer) SubRect(r Rect) *Buffer {
	return b.Sub(r.ULX, r.ULY, r.Width()+1, r.Height()+1)
}

// At returns the pixel at (x, y).
func (b *Buffer) At(x, y int) uint8 {
	if x < 0 || y < 0 || x >= b.W || y >= b.H {
		panic("gfx: At out of bounds")
	}
	return b.Pix[y*b.Stride+x]
}

// Set writes the pixel at (x, y).
func (b *Buffer) Set(x, y int, c uint8) {
	if x < 0 || y < 0 || x >= b.W || y >= b.H {
		panic("gfx: Set out of bounds")
	}
	b.Pix[y*b.Stride+x] = c
}

// Row returns the pixels of row y as a slice of length W.
func (b *Buffer) Row(y int) []uint8 {
	return b.Pix[y*b.Stride : y*b.Stride+b.W]
}

// Clone returns a compact copy (Stride == W) of the buffer.
func (b *Buffer) Clone() *Buffer {
	c := New(b.W, b.H)
	for y := 0; y < b.H; y++ {
		copy(c.Row(y), b.Row(y))
	}
	return c
}
