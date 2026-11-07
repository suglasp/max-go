// multi.go : the M.A.X. multi frame unit sprite decoder - decodes ImageMultiHeader sprites with directional frame strips.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
//
// C++ ported : resource_manager.hpp - ImageMultiHeader
package sprites

import (
	"encoding/binary"
	"fmt"

	"github.com/suglasp/max-go/internal/gfx"
)

// Multi holds the frames decoded from a MULTI sprite resource. The format starts with a frame count and offset table, followed by each frame's header and run-length encoded rows.
// It is used for both unit sprites, whose runs contain palette indices, and shadows, whose runs describe only a silhouette.
//
// C++: struct ImageMultiHeader (resource_manager.hpp)
type Multi struct {
	Frames []MultiFrame // Frames decoded in resource order.
}

// MultiFrame holds one decoded frame. Opaque distinguishes written pixels from transparent gaps; shadow frames use only Opaque and leave Pix zeroed.
//
// C++: struct ImageMultiFrameHeader (resource_manager.hpp)
type MultiFrame struct {
	W, H       int     // Frame dimensions in pixels.
	HotX, HotY int     // Draw anchor subtracted from the destination point.
	Pix        []uint8 // Palette indices in row-major order.
	Opaque     []bool  // Marks pixels written by encoded runs.
}

/* // note: unused code in Go port
// no caller measures a frame's opaque box; the draws work from W, H and the hotspot.
// ContentBounds returns the inclusive bounds of the opaque pixels. It reports false for an empty frame, letting callers ignore transparent padding when scaling icons.
func (f *MultiFrame) ContentBounds() (minX, minY, maxX, maxY int, ok bool) {
	minX, minY, maxX, maxY = f.W, f.H, -1, -1
	for y := 0; y < f.H; y++ {
		row := y * f.W
		for x := 0; x < f.W; x++ {
			if f.Opaque[row+x] {
				if x < minX {
					minX = x
				}
				if x > maxX {
					maxX = x
				}
				if y < minY {
					minY = y
				}
				if y > maxY {
					maxY = y
				}
			}
		}
	}
	return minX, minY, maxX, maxY, maxX >= 0
}
*/

// rowDelimiter ends a frame row in both the sprite and shadow encodings.
const rowDelimiter = 0xFF

// DecodeMultiSprite parses a multi-frame sprite. Each row contains [transparent_count u8][pixel_count u8][pixels...] spans terminated by 0xFF.
//
// C++: Gfx_DecodeSprite (gfx.cpp)
func DecodeMultiSprite(data []byte) (*Multi, error) {
	return decodeMulti(data, false)
}

// DecodeMultiShadow parses a multi-frame shadow. Its rows contain [transparent_count u8][shadow_count u8] spans terminated by 0xFF, with no pixel payload.
//
// C++: Gfx_DecodeShadow (gfx.cpp)
func DecodeMultiShadow(data []byte) (*Multi, error) {
	return decodeMulti(data, true)
}

// decodeMulti decodes the MULTI sprite format, optionally treating it as shadows.
func decodeMulti(data []byte, shadow bool) (*Multi, error) {
	if len(data) < 2 {
		return nil, fmt.Errorf("sprites: multi header truncated (%d bytes)", len(data))
	}
	count := int(binary.LittleEndian.Uint16(data[0:2]))
	if count <= 0 {
		return nil, fmt.Errorf("sprites: multi image_count is %d", count)
	}
	if 2+count*4 > len(data) {
		return nil, fmt.Errorf("sprites: multi frame table truncated (count %d)", count)
	}
	m := &Multi{Frames: make([]MultiFrame, count)}
	for i := 0; i < count; i++ {
		off := int(int32(binary.LittleEndian.Uint32(data[2+i*4:])))
		if err := decodeMultiFrame(data, off, shadow, &m.Frames[i]); err != nil {
			return nil, fmt.Errorf("sprites: multi frame %d: %w", i, err)
		}
	}
	return m, nil
}

// decodeMultiFrame reads an ImageMultiFrameHeader and its table of absolute row offsets. Width, height, and hotspot values are signed 16-bit integers.
func decodeMultiFrame(data []byte, off int, shadow bool, fr *MultiFrame) error {
	if off < 0 || off+8 > len(data) {
		return fmt.Errorf("header offset %d out of range", off)
	}
	fr.W = int(int16(binary.LittleEndian.Uint16(data[off:])))
	fr.H = int(int16(binary.LittleEndian.Uint16(data[off+2:])))
	fr.HotX = int(int16(binary.LittleEndian.Uint16(data[off+4:])))
	fr.HotY = int(int16(binary.LittleEndian.Uint16(data[off+6:])))
	if fr.W < 0 || fr.H < 0 {
		return fmt.Errorf("bad frame size %dx%d", fr.W, fr.H)
	}
	fr.Pix = make([]uint8, fr.W*fr.H)
	fr.Opaque = make([]bool, fr.W*fr.H)
	if fr.W == 0 || fr.H == 0 {
		return nil // empty frame (e.g. a missing animation angle)
	}
	rowsBase := off + 8
	if rowsBase+fr.H*4 > len(data) {
		return fmt.Errorf("row offset table truncated")
	}
	for y := 0; y < fr.H; y++ {
		rowOff := int(binary.LittleEndian.Uint32(data[rowsBase+y*4:]))
		if err := decodeRow(data, rowOff, shadow, fr, y); err != nil {
			return fmt.Errorf("row %d: %w", y, err)
		}
	}
	return nil
}

// decodeRow expands one encoded row into fr. Sprite spans carry literal palette indices after their counts, while shadow spans carry no pixel bytes.
func decodeRow(data []byte, off int, shadow bool, fr *MultiFrame, y int) error {
	x := 0
	for {
		if off >= len(data) {
			return fmt.Errorf("row data overruns resource")
		}
		if data[off] == rowDelimiter {
			return nil
		}
		if off+2 > len(data) {
			return fmt.Errorf("row span header truncated")
		}
		transparent := int(data[off])
		opaque := int(data[off+1])
		off += 2
		x += transparent
		end := x + opaque
		if end > fr.W {
			return fmt.Errorf("row span exceeds width %d", fr.W)
		}
		if !shadow {
			if off+opaque > len(data) {
				return fmt.Errorf("row pixel run overruns resource")
			}
		}
		base := y * fr.W
		for i := 0; i < opaque; i++ {
			fr.Opaque[base+x+i] = true
			if !shadow {
				fr.Pix[base+x+i] = data[off+i]
			}
		}
		if !shadow {
			off += opaque
		}
		x = end
	}
}

/* // note: unused code in Go port
// no caller wraps a frame as a buffer; frames draw through Draw and DrawScaled.
// Buffer wraps the frame pixels as a gfx.Buffer (shared, not copied).
// Transparent pixels read back as 0; use Opaque to distinguish them.
func (f *MultiFrame) Buffer() *gfx.Buffer {
	return gfx.FromPix(f.Pix, f.W, f.H)
}
*/

// Draw blits the frame into dst with its hotspot at (x, y): the
// upper left corner lands at (x-HotX, y-HotY). Transparent pixels are
// skipped. When remap is not nil each pixel is mapped through it; the
// team color table for unit ownership, or a brightness/tint table.
//
// C++: Gfx_DecodeSprite (gfx.cpp), unscaled path
func (f *MultiFrame) Draw(dst *gfx.Buffer, x, y int, remap *[256]uint8) {
	x -= f.HotX
	y -= f.HotY
	for srcY := 0; srcY < f.H; srcY++ {
		dy := y + srcY
		if dy < 0 || dy >= dst.H {
			continue
		}
		base := srcY * f.W
		for srcX := 0; srcX < f.W; srcX++ {
			if !f.Opaque[base+srcX] {
				continue
			}
			dx := x + srcX
			if dx < 0 || dx >= dst.W {
				continue
			}
			c := f.Pix[base+srcX]
			if remap != nil {
				c = remap[c]
			}
			dst.Set(dx, dy, c)
		}
	}
}

// scaleStep is the nearest neighbor sampling step (8.16 fixed point) that
// maps dstDim output pixels back onto srcDim source pixels, with the
// original's +8 bias so the last output pixel lands on the source edge.
// It matches drawmap's tile scaling and the engine's Gfx_ScalingFactor.
//
// C++: Gfx_DecodeSpriteSetup scaling factor formula (gfx.cpp)
func scaleStep(srcDim, dstDim int) int {
	if dstDim <= 1 {
		return 0
	}
	return ((srcDim-1)<<16)/(dstDim-1) + 8
}

// DrawScaledCentered draws the frame scaled by num/den with its geometric
// centre placed at (x, y), ignoring the baked hotspot. Effect sprites
// (explosions) carry hotspots for each frame that track a blast origin off
// center as the blast expands, not the visual centre, so anchoring by the
// hotspot throws a blast spanning multiple cells off its footprint; centring
// by dimensions keeps it on the cell/building it destroyed.
func (f *MultiFrame) DrawScaledCentered(dst *gfx.Buffer, x, y, num, den int, remap *[256]uint8) {
	dw := scaleDim(f.W, num, den)
	dh := scaleDim(f.H, num, den)
	if dw <= 0 || dh <= 0 {
		return
	}
	f.blitScaled(dst, x-dw/2, y-dh/2, dw, dh, scaleStep(f.W, dw), scaleStep(f.H, dh), remap)
}

// blitScaled copies the frame's opaque pixels into dst nearest neighbour scaled to
// dw x dh at origin (ox, oy), stepping the source by stepX/stepY (16.16 fixed
// point). remap, when not nil, recolours each pixel. It is the shared inner blit
// of DrawScaled and DrawScaledCentered, which differ only in how they place the
// origin (baked hotspot vs geometric centre).
func (f *MultiFrame) blitScaled(dst *gfx.Buffer, ox, oy, dw, dh, stepX, stepY int, remap *[256]uint8) {
	for dy := 0; dy < dh; dy++ {
		ty := oy + dy
		if ty < 0 || ty >= dst.H {
			continue
		}
		srcY := (dy * stepY) >> 16
		srow := srcY * f.W
		for dx := 0; dx < dw; dx++ {
			srcX := (dx * stepX) >> 16
			if !f.Opaque[srow+srcX] {
				continue
			}
			tx := ox + dx
			if tx < 0 || tx >= dst.W {
				continue
			}
			c := f.Pix[srow+srcX]
			if remap != nil {
				c = remap[c]
			}
			dst.Set(tx, ty, c)
		}
	}
}

// DrawScaled blits the frame scaled by num/den (screen pixels per
// GFX_MAP_TILE_SIZE source pixels: i.e. the map zoom over 64) with its
// hotspot at (x, y): the scaled upper left lands at (x-HotX*num/den,
// y-HotY*num/den). Transparent pixels are skipped; remap, when not nil,
// recolors each pixel (the team color table, or a brightness/tint LUT).
// At num == den this is the exact 1:1 Draw.
//
// C++: Gfx_DecodeSprite, the scaled blit path (gfx.cpp)
func (f *MultiFrame) DrawScaled(dst *gfx.Buffer, x, y, num, den int, remap *[256]uint8) {
	if num == den {
		f.Draw(dst, x, y, remap)
		return
	}
	dw := scaleDim(f.W, num, den)
	dh := scaleDim(f.H, num, den)
	if dw <= 0 || dh <= 0 {
		return
	}
	f.blitScaled(dst, x-(f.HotX*num)/den, y-(f.HotY*num)/den, dw, dh, scaleStep(f.W, dw), scaleStep(f.H, dh), remap)
}

// DrawShadowScaled darkens dst under the frame's silhouette scaled by
// num/den, the shadow counterpart of DrawScaled (see DrawShadow).
//
// C++: Gfx_DecodeShadow, the scaled blit path (gfx.cpp)
func (f *MultiFrame) DrawShadowScaled(dst *gfx.Buffer, x, y, num, den int, darken *[256]uint8) {
	if num == den {
		f.DrawShadow(dst, x, y, darken)
		return
	}
	dw := scaleDim(f.W, num, den)
	dh := scaleDim(f.H, num, den)
	if dw <= 0 || dh <= 0 {
		return
	}
	ox := x - (f.HotX*num)/den
	oy := y - (f.HotY*num)/den
	stepX := scaleStep(f.W, dw)
	stepY := scaleStep(f.H, dh)
	for dy := 0; dy < dh; dy++ {
		ty := oy + dy
		if ty < 0 || ty >= dst.H {
			continue
		}
		srcY := (dy * stepY) >> 16
		srow := srcY * f.W
		for dx := 0; dx < dw; dx++ {
			srcX := (dx * stepX) >> 16
			if !f.Opaque[srow+srcX] {
				continue
			}
			tx := ox + dx
			if tx < 0 || tx >= dst.W {
				continue
			}
			dst.Set(tx, ty, darken[dst.At(tx, ty)])
		}
	}
}

// scaleDim scales a source dimension by num/den, rounding up so a
// source that is not empty never collapses to zero output pixels.
func scaleDim(src, num, den int) int {
	if src <= 0 || num <= 0 || den <= 0 {
		return 0
	}
	d := (src*num + den - 1) / den
	if d < 1 {
		d = 1
	}
	return d
}

// DrawShadow darkens dst under the frame's silhouette: every opaque pixel
// replaces the destination pixel with darken[dest]. The shadow sprite
// supplies only the shape; the darkening LUT (a palette mapping with
// reduced brightness) is supplied by the caller.
//
// C++: Gfx_DecodeShadow (gfx.cpp): window_buffer[i] = index_table[window_buffer[i]]
func (f *MultiFrame) DrawShadow(dst *gfx.Buffer, x, y int, darken *[256]uint8) {
	x -= f.HotX
	y -= f.HotY
	for srcY := 0; srcY < f.H; srcY++ {
		dy := y + srcY
		if dy < 0 || dy >= dst.H {
			continue
		}
		base := srcY * f.W
		for srcX := 0; srcX < f.W; srcX++ {
			if !f.Opaque[base+srcX] {
				continue
			}
			dx := x + srcX
			if dx < 0 || dx >= dst.W {
				continue
			}
			dst.Set(dx, dy, darken[dst.At(dx, dy)])
		}
	}
}
