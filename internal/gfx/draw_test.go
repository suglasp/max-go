// draw_test.go : tests the 8-bit drawing primitives in draw.go.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package gfx

import (
	"bytes"
	"testing"
)

func TestRectOps(t *testing.T) {
	a := NewRect(0, 0, 9, 9)
	b := NewRect(5, 5, 14, 14)

	/* // note: unused code in Go port
	// the check of Rect.Union, commented out with it.
	if got := a.Union(b); got != NewRect(0, 0, 14, 14) {
		t.Errorf("Union = %+v", got)
	}
	*/
	clipped, ok := b.Intersect(a)
	if !ok || clipped != NewRect(5, 5, 9, 9) {
		t.Errorf("Intersect = %+v, %v", clipped, ok)
	}
	if _, ok := NewRect(20, 20, 30, 30).Intersect(a); ok {
		t.Error("disjoint rects reported as intersecting")
	}
	// Corners are inclusive: rects touching at one pixel intersect.
	touch, ok := NewRect(9, 9, 12, 12).Intersect(a)
	if !ok || touch != NewRect(9, 9, 9, 9) {
		t.Errorf("touching Intersect = %+v, %v", touch, ok)
	}
	if a.Width() != 9 || a.Height() != 9 {
		t.Errorf("Width/Height = %d/%d, want 9/9 (rect.c convention)", a.Width(), a.Height())
	}
	if !a.Contains(9, 9) || a.Contains(10, 9) {
		t.Error("Contains corner inclusivity wrong")
	}
}

func TestDrawLine(t *testing.T) {
	b := New(8, 8)
	b.DrawLine(1, 2, 6, 2, 7) // horizontal
	for x := 1; x <= 6; x++ {
		if b.At(x, 2) != 7 {
			t.Errorf("horizontal line missing pixel at %d", x)
		}
	}
	if b.At(0, 2) != 0 || b.At(7, 2) != 0 {
		t.Error("horizontal line endpoints not inclusive-exact")
	}

	b = New(8, 8)
	b.DrawLine(3, 1, 3, 6, 9) // vertical
	for y := 1; y <= 6; y++ {
		if b.At(3, y) != 9 {
			t.Errorf("vertical line missing pixel at %d", y)
		}
	}

	b = New(8, 8)
	b.DrawLine(0, 0, 7, 7, 5) // diagonal
	for i := 0; i < 8; i++ {
		if b.At(i, i) != 5 {
			t.Errorf("diagonal missing pixel at %d", i)
		}
	}

	b = New(8, 8)
	b.DrawLine(6, 5, 1, 1, 4) // reverse direction
	if b.At(6, 5) != 4 || b.At(1, 1) != 4 {
		t.Error("reverse line endpoints missing")
	}
}

func TestBoxesAndFill(t *testing.T) {
	b := New(10, 10)
	v := b.Sub(2, 2, 6, 6)
	v.Fill(3)
	if b.At(1, 1) != 0 || b.At(8, 8) != 0 {
		t.Error("Fill leaked outside sub view")
	}
	if b.At(2, 2) != 3 || b.At(7, 7) != 3 {
		t.Error("Fill missed sub view corners")
	}

	b = New(10, 10)
	b.DrawBox(1, 1, 8, 8, 6)
	if b.At(1, 1) != 6 || b.At(8, 8) != 6 || b.At(8, 1) != 6 || b.At(1, 8) != 6 {
		t.Error("DrawBox corners missing")
	}
	if b.At(2, 2) != 0 {
		t.Error("DrawBox filled interior")
	}

	b = New(10, 10)
	b.DrawShadedBox(1, 1, 8, 8, 6, 9)
	if b.At(4, 1) != 6 || b.At(1, 4) != 6 {
		t.Error("shaded box top/left should be color1")
	}
	if b.At(4, 8) != 9 || b.At(8, 4) != 9 {
		t.Error("shaded box bottom/right should be color2")
	}
}

func TestDrawCircle(t *testing.T) {
	b := New(16, 16)
	b.DrawCircle(8, 8, 5, 1)

	if b.At(8, 3) != 1 || b.At(8, 13) != 1 || b.At(3, 8) != 1 || b.At(13, 8) != 1 {
		t.Error("circle cardinal points missing")
	}
	// 4-way symmetry around the center.
	for y := 0; y < 16; y++ {
		for x := 0; x < 16; x++ {
			if b.At(x, y) != b.At(16-1-(16-1-x), y) { // identity sanity
				t.Fatal("impossible")
			}
			mx, my := 16-x, 16-y // mirrored across center (8,8)
			if mx < 16 && my < 16 && b.At(x, y) != b.At(mx, my) {
				t.Errorf("circle not symmetric at (%d,%d)", x, y)
			}
		}
	}
}

func TestCopies(t *testing.T) {
	src := New(3, 2)
	copy(src.Pix, []uint8{1, 0, 2, 0, 3, 0})

	dst := New(5, 5)
	dst.Fill(9)
	dst.Sub(1, 1, 3, 2).Copy(src)
	if dst.At(1, 1) != 1 || dst.At(2, 1) != 0 || dst.At(2, 2) != 3 {
		t.Error("Copy wrong pixels")
	}

	dst = New(5, 5)
	dst.Fill(9)
	dst.Sub(1, 1, 3, 2).TransCopy(src)
	if dst.At(1, 1) != 1 || dst.At(3, 1) != 2 {
		t.Error("TransCopy skipped opaque pixels")
	}
	if dst.At(2, 1) != 9 {
		t.Error("TransCopy overwrote with transparent color 0")
	}

	/* // note: unused code in Go port
	// the checks of MaskCopy and MaskTransCopy, commented out with them.
	mask := New(3, 2)
	copy(mask.Pix, []uint8{1, 1, 0, 0, 1, 1})

	dst = New(5, 5)
	dst.Fill(9)
	dst.Sub(1, 1, 3, 2).MaskCopy(src, mask)
	if dst.At(1, 1) != 1 {
		t.Error("MaskCopy dropped masked opaque pixel")
	}
	if dst.At(3, 1) != 9 { // mask 0 there
		t.Error("MaskCopy wrote where mask is zero")
	}
	if dst.At(2, 1) != 9 { // src 0 there, mask 1
		t.Error("MaskCopy wrote transparent src pixel")
	}

	dst = New(5, 5)
	dst.Fill(9)
	dst.Sub(1, 1, 3, 2).MaskTransCopy(src, mask)
	if dst.At(2, 1) != 0 { // src 0, mask 1: copied
		t.Error("MaskTransCopy must copy color 0 where mask set")
	}
	*/
}

func TestScale(t *testing.T) {
	src := New(2, 2)
	copy(src.Pix, []uint8{1, 2, 3, 4})

	dst := New(4, 4)
	dst.Scale(src)
	want := []uint8{
		1, 1, 2, 2,
		1, 1, 2, 2,
		3, 3, 4, 4,
		3, 3, 4, 4,
	}
	if !bytes.Equal(dst.Pix, want) {
		t.Errorf("Scale 2x2->4x4 =\n%v want\n%v", dst.Pix, want)
	}

	/* // note: unused code in Go port
	// the check of TransScale, commented out with it.
	src.Pix[1] = 0 // transparent
	dst = New(4, 4)
	dst.Fill(9)
	dst.TransScale(src)
	if dst.At(2, 0) != 9 || dst.At(3, 1) != 9 {
		t.Error("TransScale wrote transparent block")
	}
	if dst.At(0, 0) != 1 || dst.At(3, 3) != 4 {
		t.Error("TransScale missed opaque blocks")
	}
	*/
}

/* // note: unused code in Go port
// the test of Texture, commented out with it.
func TestTexture(t *testing.T) {
	tex := New(2, 2)
	copy(tex.Pix, []uint8{1, 2, 3, 4})

	b := New(5, 3)
	b.Texture(tex, 0, 0)
	want := []uint8{
		1, 2, 1, 2, 1,
		3, 4, 3, 4, 3,
		1, 2, 1, 2, 1,
	}
	if !bytes.Equal(b.Pix, want) {
		t.Errorf("Texture =\n%v want\n%v", b.Pix, want)
	}

	b = New(2, 2)
	b.Texture(tex, 1, 1)
	want = []uint8{4, 3, 2, 1}
	if !bytes.Equal(b.Pix, want) {
		t.Errorf("Texture offset =\n%v want\n%v", b.Pix, want)
	}
}
*/

func TestRecolorAndSwap(t *testing.T) {
	b := New(2, 2)
	copy(b.Pix, []uint8{0, 1, 2, 1})

	var table [256]uint8
	for i := range table {
		table[i] = uint8(i)
	}
	table[1] = 100
	b.Recolor(&table)
	if !bytes.Equal(b.Pix, []uint8{0, 100, 2, 100}) {
		t.Errorf("Recolor = %v", b.Pix)
	}

	/* // note: unused code in Go port
	// the check of SwapColor, commented out with it.
	b.SwapColor(100, 2)
	if !bytes.Equal(b.Pix, []uint8{0, 2, 100, 2}) {
		t.Errorf("SwapColor = %v", b.Pix)
	}
	*/
}

func TestSubViewSharesPixels(t *testing.T) {
	b := New(4, 4)
	v := b.Sub(1, 1, 2, 2)
	v.Set(0, 0, 5)
	if b.At(1, 1) != 5 {
		t.Error("Sub view does not share backing pixels")
	}
	c := b.Clone()
	c.Set(0, 0, 7)
	if b.At(0, 0) == 7 {
		t.Error("Clone shares backing pixels")
	}
}
