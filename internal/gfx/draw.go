// draw.go : renders lines, boxes, fills, copies, and scaling on 8-bit indexed buffers.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
// C++ ported : grbuf.c - GNW drawing primitives
package gfx

// Drawing primitives ported from GNW's grbuf.c. They follow the
// original algorithms pixel for pixel; none of them clip (see Buffer).

// DrawLine draws a line from the start (x, y) to the target (endX, endY),
// both endpoints inclusive, using GNW's Bresenham variant.
//
// C++: draw_line (grbuf.c)
func (b *Buffer) DrawLine(x, y, endX, endY int, color uint8) {
	deltaX := 2 * abs(endX-x)
	offsetX := 1
	if endX-x < 0 {
		offsetX = -1
	}
	deltaY := 2 * abs(endY-y)
	offsetY := 1
	if endY-y < 0 {
		offsetY = -1
	}

	if deltaX <= deltaY {
		accum := deltaX - deltaY>>1
		for {
			b.Set(x, y, color)
			if y == endY {
				return
			}
			if accum >= 0 {
				accum -= deltaY
				x += offsetX
			}
			accum += deltaX
			y += offsetY
		}
	} else {
		accum := deltaY - deltaX>>1
		for {
			b.Set(x, y, color)
			if x == endX {
				return
			}
			if accum >= 0 {
				accum -= deltaX
				y += offsetY
			}
			accum += deltaY
			x += offsetX
		}
	}
}

// DrawBox draws a hollow rectangle, corners inclusive.
//
// C++: draw_box (grbuf.c)
func (b *Buffer) DrawBox(ulx, uly, lrx, lry int, color uint8) {
	b.DrawLine(ulx, uly, lrx, uly, color)
	b.DrawLine(ulx, lry, lrx, lry, color)
	b.DrawLine(ulx, uly, ulx, lry, color)
	b.DrawLine(lrx, uly, lrx, lry, color)
}

// DrawShadedBox draws a hollow rectangle with the top/left edges in
// color1 and the bottom/right edges in color2 (3D bevel look).
//
// C++: draw_shaded_box (grbuf.c)
func (b *Buffer) DrawShadedBox(ulx, uly, lrx, lry int, color1, color2 uint8) {
	b.DrawLine(ulx, uly, lrx, uly, color1)
	b.DrawLine(ulx, lry, lrx, lry, color2)
	b.DrawLine(ulx, uly, ulx, lry, color1)
	b.DrawLine(lrx, uly, lrx, lry, color2)
}

// DrawCircle draws a circle outline of radius r around (x, y) using
// GNW's midpoint variant.
//
// C++: draw_circle (grbuf.c)
func (b *Buffer) DrawCircle(x, y, r int, color uint8) {
	offsetX := 0
	offsetY := r
	decision := 2 * (1 - r)

	b.Set(x, y+r, color)
	b.Set(x, y-r, color)

	for offsetY >= 0 {
		if decision >= 0 {
			if decision <= 0 {
				offsetX++
				offsetY--
				decision += 2*offsetX - 2*offsetY + 2
			} else if 2*decision-2*offsetX-1 <= 0 {
				offsetX++
				offsetY--
				decision += 2*offsetX - 2*offsetY + 2
			} else {
				offsetY--
				decision += 1 - 2*offsetY
			}
		} else if 2*decision+2*offsetY-1 <= 0 {
			offsetX++
			decision += 2*offsetX + 1
		} else {
			offsetX++
			offsetY--
			decision += 2*offsetX - 2*offsetY + 2
		}

		b.Set(x+offsetX, y+offsetY, color)
		b.Set(x+offsetX, y-offsetY, color)
		b.Set(x-offsetX, y-offsetY, color)
		b.Set(x-offsetX, y+offsetY, color)
	}
}

// DrawCircleClip draws a circle outline like DrawCircle but silently skips any
// point outside the buffer, so a radius larger than the buffer (e.g. a wide
// scan range ring over the map viewport) is safe.
func (b *Buffer) DrawCircleClip(x, y, r int, color uint8) {
	set := func(px, py int) {
		if px >= 0 && py >= 0 && px < b.W && py < b.H {
			b.Pix[py*b.Stride+px] = color
		}
	}
	// offsetX/offsetY walk one octant out from the centre; decision is the
	// midpoint error term that picks the next pixel.
	offsetX, offsetY, decision := 0, r, 2*(1-r)
	set(x, y+r)
	set(x, y-r)
	for offsetY >= 0 {
		if decision >= 0 {
			if decision <= 0 || 2*decision-2*offsetX-1 <= 0 {
				offsetX++
				offsetY--
				decision += 2*offsetX - 2*offsetY + 2
			} else {
				offsetY--
				decision += 1 - 2*offsetY
			}
		} else if 2*decision+2*offsetY-1 <= 0 {
			offsetX++
			decision += 2*offsetX + 1
		} else {
			offsetX++
			offsetY--
			decision += 2*offsetX - 2*offsetY + 2
		}
		set(x+offsetX, y+offsetY)
		set(x+offsetX, y-offsetY)
		set(x-offsetX, y-offsetY)
		set(x-offsetX, y+offsetY)
	}
}

// Fill sets every pixel of the buffer view to color.
//
// C++: buf_fill (grbuf.c)
func (b *Buffer) Fill(color uint8) {
	for y := 0; y < b.H; y++ {
		row := b.Row(y)
		for i := range row {
			row[i] = color
		}
	}
}

// Copy copies src into b at (0, 0). Position with Sub views.
//
// C++: buf_to_buf (grbuf.c)
func (b *Buffer) Copy(src *Buffer) {
	for y := 0; y < src.H; y++ {
		copy(b.Pix[y*b.Stride:], src.Row(y))
	}
}

// TransCopy copies src into b, skipping color 0 (transparent).
//
// C++: trans_buf_to_buf (grbuf.c)
func (b *Buffer) TransCopy(src *Buffer) {
	for y := 0; y < src.H; y++ {
		srow := src.Row(y)
		drow := b.Pix[y*b.Stride : y*b.Stride+src.W]
		for x, c := range srow {
			if c != 0 {
				drow[x] = c
			}
		}
	}
}

// KeyToZero remaps every pixel equal to key to color 0, so a later
// transparent blit (which skips 0) drops them. MAX's ImageSimpleHeader overlaps
// its transparent_color field onto the first pixel, so an image's transparent
// colour is its top left pixel (commonly a fuchsia palette slot); callers pass
// that as key to make the image's keyed regions transparent.
func (b *Buffer) KeyToZero(key uint8) {
	if key == 0 {
		return
	}
	for y := 0; y < b.H; y++ {
		row := b.Pix[y*b.Stride : y*b.Stride+b.W]
		for x, c := range row {
			if c == key {
				row[x] = 0
			}
		}
	}
}

// MaskCopy copies nonzero src pixels where the mask is nonzero.
//
// C++: mask_buf_to_buf (grbuf.c)
func (b *Buffer) MaskCopy(src, mask *Buffer) {
	for y := 0; y < src.H; y++ {
		srow := src.Row(y)
		mrow := mask.Row(y)
		drow := b.Pix[y*b.Stride : y*b.Stride+src.W]
		for x, c := range srow {
			if mrow[x] != 0 && c != 0 {
				drow[x] = c
			}
		}
	}
}

// MaskTransCopy copies src pixels (including color 0) where the mask
// is nonzero.
//
// C++: mask_trans_buf_to_buf (grbuf.c)
func (b *Buffer) MaskTransCopy(src, mask *Buffer) {
	for y := 0; y < src.H; y++ {
		srow := src.Row(y)
		mrow := mask.Row(y)
		drow := b.Pix[y*b.Stride : y*b.Stride+src.W]
		for x := range srow {
			if mrow[x] != 0 {
				drow[x] = srow[x]
			}
		}
	}
}

// Scale stretches src to fill b (nearest neighbor, 16.16 fixed point
// exactly as the original; the game depends on its rounding for tile
// zoom levels).
//
// C++: cscale (grbuf.c)
func (b *Buffer) Scale(src *Buffer) {
	b.scale(src, false)
}

// TransScale is Scale skipping color 0 in src.
//
// C++: trans_cscale (grbuf.c)
func (b *Buffer) TransScale(src *Buffer) {
	b.scale(src, true)
}

// scale copies src into b using nearest neighbor scaling.
func (b *Buffer) scale(src *Buffer, transparent bool) {
	my := (b.H << 16) / src.H
	mx := (b.W << 16) / src.W

	for srcy := 0; srcy < src.H; srcy++ {
		dstX0 := 0
		dstX1 := mx
		srow := src.Row(srcy)

		for srcx := 0; srcx < src.W; srcx++ {
			pixel := srow[srcx]
			if !transparent || pixel != 0 {
				for desty := my * srcy >> 16; desty < (my+my*srcy)>>16; desty++ {
					drow := b.Pix[desty*b.Stride:]
					for destx := dstX0 >> 16; destx < dstX1>>16; destx++ {
						drow[destx] = pixel
					}
				}
			}
			dstX0 += mx
			dstX1 += mx
		}
	}
}

// Texture tiles tex across the buffer view, starting at texture
// offset (tx, ty).
//
// C++: buf_texture (grbuf.c)
func (b *Buffer) Texture(tex *Buffer, tx, ty int) {
	tx %= tex.W
	ty %= tex.H

	for y := 0; y < b.H; y++ {
		drow := b.Row(y)
		trow := tex.Row(ty)
		x := 0
		texX := tx
		for x < b.W {
			n := tex.W - texX
			if b.W-x < n {
				n = b.W - x
			}
			copy(drow[x:x+n], trow[texX:texX+n])
			x += n
			texX += n
			if texX == tex.W {
				texX = 0
			}
		}
		ty++
		if ty == tex.H {
			ty = 0
		}
	}
}

// Recolor maps every pixel through the 256-entry lookup table.
//
// C++: Color_RecolorPixels (color.cpp)
func (b *Buffer) Recolor(table *[256]uint8) {
	for y := 0; y < b.H; y++ {
		row := b.Row(y)
		for x, c := range row {
			row[x] = table[c]
		}
	}
}

// SwapColor exchanges all pixels of color c1 and c2.
//
// C++: swap_color_buf (grbuf.c)
func (b *Buffer) SwapColor(c1, c2 uint8) {
	for y := 0; y < b.H; y++ {
		row := b.Row(y)
		for x, c := range row {
			switch c {
			case c1:
				row[x] = c2
			case c2:
				row[x] = c1
			}
		}
	}
}

// abs returns the absolute value of an integer.
func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}
