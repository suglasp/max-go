// draw.go : renders bitmap glyphs and clipped strings into 8-bit buffers.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package fonts

import "github.com/suglasp/max-go/internal/gfx"

// Text rendering with text.c semantics: glyphs are drawn into an
// 8-bit buffer at its origin (callers position text with Sub views,
// as the C code does with pointer arithmetic), only set bits are
// written (background stays), and the color int carries flags.

// glyph returns the glyph for byte c, or nil when the font does not
// cover it (fonts 0/3/4/6 only have 128 glyphs).
func (f *Font) glyph(c byte) *Glyph {
	if int(c) >= len(f.Glyphs) {
		return nil
	}
	return &f.Glyphs[c]
}

// GlyphWidth returns the width of character c used for advance.
//
// C++: Text_GetGlyphWidth
func (f *Font) GlyphWidth(c byte) int {
	if g := f.glyph(c); g != nil {
		return g.Width
	}
	return 0
}

// Width returns the pixel width of str: glyph widths plus spacing
// after every character.
//
// C++: Text_GetWidth
func (f *Font) Width(str string) int {
	w := 0
	for i := 0; i < len(str); i++ {
		w += f.GlyphWidth(str[i]) + f.Spacing
	}
	return w
}

// MonoCell returns the cell width used in monospace mode: the widest
// glyph plus spacing.
func (f *Font) MonoCell() int {
	max := 0
	for i := range f.Glyphs {
		if f.Glyphs[i].Width > max {
			max = f.Glyphs[i].Width
		}
	}
	return max + f.Spacing
}

/* // note: ported but unused code in Go port
// every message box passes monospace = false; the reference never reaches it either.
// MonoWidth returns the pixel width of str in monospace mode.
//
// C++: Text_GetMonospaceWidth
func (f *Font) MonoWidth(str string) int {
	return f.MonoCell() * len(str)
}
*/

/* // note: ported but unused code in Go port
// no caller sizes a text buffer this way; the reference never calls Text_GetSize either.
// Size returns Width(str) * Height, the buffer size needed for the
// string.
//
// C++: Text_GetSize
func (f *Font) Size(str string) int {
	return f.Width(str) * f.Height
}
*/

// Blit renders str into dst starting at the buffer origin, drawing at
// most maxWidth pixels (and never outside dst). color packs the
// palette index (low byte) and Flag* bits.
//
// C++: Text_Blit (text.cpp)
func (f *Font) Blit(dst *gfx.Buffer, str string, maxWidth int, color int) {
	if maxWidth <= 0 || maxWidth > dst.W {
		maxWidth = dst.W
	}
	fg := uint8(color & ColorMask)
	mono := color&FlagMonospace != 0
	cell := 0
	if mono {
		cell = f.MonoCell()
	}

	x := 0
	for i := 0; i < len(str); i++ {
		g := f.glyph(str[i])
		if g == nil {
			continue
		}
		advance := g.Width + f.Spacing
		gx := x
		if mono {
			advance = cell
			gx = x + (cell-g.Width)/2
		}
		if gx+g.Width > maxWidth {
			break
		}
		if color&FlagOutline != 0 {
			drawGlyph(dst, g, gx-1, 0, f.Height, 0)
			drawGlyph(dst, g, gx+1, 0, f.Height, 0)
			drawGlyph(dst, g, gx, -1, f.Height, 0)
			drawGlyph(dst, g, gx, 1, f.Height, 0)
		}
		drawGlyph(dst, g, gx, 0, f.Height, fg)
		x += advance
	}

	if color&FlagUnderline != 0 && x > 0 && f.Height-1 < dst.H {
		row := dst.Row(f.Height - 1)
		for i := 0; i < x && i < len(row); i++ {
			row[i] = fg
		}
	}
}

// Color is a GNW font color triple: the text body, the inner bevel
// painted on text pixels whose neighbor down and to the right is also
// text, and the drop shadow painted where that neighbor is background.
//
// C++: struct FontColor (fonts.hpp)
type Color struct {
	Base, Outline, Shadow uint8
}

// Standard menu text colors (fonts.cpp).
var (
	ColorGold       = Color{165, 175, 199}
	ColorDarkOrange = Color{5, 56, 199}
	/* // note: ported but unused code in Go port
	// a disabled caption in vanilla is a blank plate (DOS check 2026-11-07), so the reference default is unused.
	ColorDarkGray     = Color{179, 215, 199}
	*/
	ColorBrightBrown  = Color{164, 173, 199}
	ColorBrightYellow = Color{162, 8, 199}
	ColorBrightSilver = Color{162, 172, 199}
)

// BlitShadowed renders str into dst with the classic GNW bevel and
// drop shadow effect. The pass runs from the bottom up so shadows
// cascade exactly like the original.
//
// C++: Text_TextLine (text.cpp)
func (f *Font) BlitShadowed(dst *gfx.Buffer, str string, maxWidth int, color Color) {
	tw := f.Width(str) + 1
	th := f.Height + 1
	scratch := gfx.New(tw, th)
	f.Blit(scratch.Sub(0, 0, tw, f.Height), str, maxWidth, int(color.Base))

	for y := th - 2; y >= 0; y-- {
		row := scratch.Row(y)
		next := scratch.Row(y + 1)
		for x := 0; x < tw-1; x++ {
			if row[x] == color.Base {
				if next[x+1] == color.Base {
					next[x+1] = color.Outline
				} else {
					next[x+1] = color.Shadow
				}
			}
		}
	}

	w := min(tw, dst.W)
	h := min(th, dst.H)
	dst.Sub(0, 0, w, h).TransCopy(scratch.Sub(0, 0, w, h))
}

// drawGlyph writes the set bits of g at (ox, oy), clipped to dst.
func drawGlyph(dst *gfx.Buffer, g *Glyph, ox, oy, height int, color uint8) {
	for y := 0; y < height; y++ {
		dy := y + oy
		if dy < 0 || dy >= dst.H {
			continue
		}
		row := dst.Row(dy)
		for x := 0; x < g.Width; x++ {
			dx := x + ox
			if dx < 0 || dx >= len(row) {
				continue
			}
			if g.Pixel(x, y) {
				row[dx] = color
			}
		}
	}
}
