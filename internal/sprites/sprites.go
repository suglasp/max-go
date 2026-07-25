// sprites.go : the M.A.X. image/sprite decoder - decodes simple and RLE compressed large images from mainly 'MAX.RES'.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
//
// C++ ported : resource_manager.hpp - ImageSimpleHeader, ImageBigHeader image decoding
//
// Package sprites decodes raw and RLE-compressed images stored in *.RES resource files, including large images with embedded palettes.
// Formats are documented in docs/formats/images.md.
package sprites

import (
	"encoding/binary"
	"fmt"

	"github.com/suglasp/max-go/internal/gfx"
)

// Simple is an uncompressed image with an eight-byte header followed by W*H palette indices. The original ImageSimpleHeader names the first pixel as transparent_color.
//
// C++: struct ImageSimpleHeader (resource_manager.hpp)
type Simple struct {
	W, H     int     // Image dimensions in pixels.
	ULX, ULY int     // Draw offset subtracted from the destination point.
	Pix      []uint8 // Palette indices in row-major order.
}

// DecodeSimple parses a simple image resource.
func DecodeSimple(data []byte) (*Simple, error) {
	if len(data) < 8 {
		return nil, fmt.Errorf("sprites: simple image header truncated (%d bytes)", len(data))
	}
	s := &Simple{
		W:   int(int16(binary.LittleEndian.Uint16(data[0:2]))),
		H:   int(int16(binary.LittleEndian.Uint16(data[2:4]))),
		ULX: int(int16(binary.LittleEndian.Uint16(data[4:6]))),
		ULY: int(int16(binary.LittleEndian.Uint16(data[6:8]))),
	}
	if s.W <= 0 || s.H <= 0 {
		return nil, fmt.Errorf("sprites: bad simple image size %dx%d", s.W, s.H)
	}
	if len(data) < 8+s.W*s.H {
		return nil, fmt.Errorf("sprites: simple image pixel data truncated: have %d, want %d",
			len(data)-8, s.W*s.H)
	}
	s.Pix = data[8 : 8+s.W*s.H]
	return s, nil
}

// Buffer wraps the pixels as a gfx.Buffer (shared, not copied).
func (s *Simple) Buffer() *gfx.Buffer {
	return gfx.FromPix(s.Pix, s.W, s.H)
}

// TransparentColor returns the first pixel, which the original blend routine treats as the image's transparent color.
//
// C++: Image::Blend (image.cpp)
func (s *Simple) TransparentColor() uint8 {
	return s.Pix[0]
}

// Draw blits the image at (x-ULX, y-ULY) and clips it to dst. When transparent is true, palette index 0 is skipped according to the GNW convention.
//
// C++: WindowManager_DecodeSimpleImage (window_manager.cpp)
func (s *Simple) Draw(dst *gfx.Buffer, x, y int, transparent bool) {
	x -= s.ULX
	y -= s.ULY

	src := gfx.NewRect(0, 0, s.W-1, s.H-1)
	pos := gfx.NewRect(x, y, x+s.W-1, y+s.H-1)
	clipped, ok := pos.Intersect(gfx.NewRect(0, 0, dst.W-1, dst.H-1))
	if !ok {
		return
	}
	src.ULX += clipped.ULX - pos.ULX
	src.ULY += clipped.ULY - pos.ULY
	src.LRX -= pos.LRX - clipped.LRX
	src.LRY -= pos.LRY - clipped.LRY

	from := s.Buffer().SubRect(src)
	to := dst.SubRect(clipped)
	if transparent {
		to.TransCopy(from)
	} else {
		to.Copy(from)
	}
}

// Big is an RLE-compressed full-screen image with its own 256-color palette. Its pixel stream begins at offset 776, where the original header declares transparent_color.
//
// C++: struct ImageBigHeader (resource_manager.hpp)
type Big struct {
	ULX, ULY int         // Image origin offsets.
	W, H     int         // Image dimensions in pixels.
	Palette  gfx.Palette // Palette converted from 8-bit components to the engine's 6-bit range.
	Pix      []uint8     // Decoded pixels in row-major order.
}

const bigHeaderSize = 8 + gfx.PaletteSize*gfx.PaletteStride // 776

// DecodeBig parses and decompresses a big image resource.
//
// C++: WindowManager_DecodeBigImage (window_manager.cpp)
func DecodeBig(data []byte) (*Big, error) {
	if len(data) < bigHeaderSize {
		return nil, fmt.Errorf("sprites: big image header truncated (%d bytes)", len(data))
	}
	b := &Big{
		ULX: int(int16(binary.LittleEndian.Uint16(data[0:2]))),
		ULY: int(int16(binary.LittleEndian.Uint16(data[2:4]))),
		W:   int(int16(binary.LittleEndian.Uint16(data[4:6]))),
		H:   int(int16(binary.LittleEndian.Uint16(data[6:8]))),
	}
	if b.W <= 0 || b.H <= 0 {
		return nil, fmt.Errorf("sprites: bad big image size %dx%d", b.W, b.H)
	}
	for i := 0; i < gfx.PaletteSize*gfx.PaletteStride; i++ {
		b.Palette[i] = data[8+i] / 4
	}

	b.Pix = make([]uint8, b.W*b.H)
	stream := data[bigHeaderSize:]
	pos := 0
	for row := 0; row < b.H; row++ {
		line := b.Pix[row*b.W : (row+1)*b.W]
		for x := 0; x < b.W; {
			if pos+2 > len(stream) {
				return nil, fmt.Errorf("sprites: big image RLE truncated at row %d", row)
			}
			opt := int(int16(binary.LittleEndian.Uint16(stream[pos:])))
			pos += 2
			switch {
			case opt > 0: // literal run
				if x+opt > b.W || pos+opt > len(stream) {
					return nil, fmt.Errorf("sprites: big image literal run overflow at row %d", row)
				}
				copy(line[x:x+opt], stream[pos:pos+opt])
				pos += opt
				x += opt
			case opt < 0: // repeat run
				opt = -opt
				if x+opt > b.W || pos >= len(stream) {
					return nil, fmt.Errorf("sprites: big image repeat run overflow at row %d", row)
				}
				c := stream[pos]
				pos++
				for i := x; i < x+opt; i++ {
					line[i] = c
				}
				x += opt
			default:
				return nil, fmt.Errorf("sprites: big image zero-length run at row %d", row)
			}
		}
	}
	return b, nil
}

// Buffer wraps the decoded pixels as a gfx.Buffer (shared, not
// copied).
func (b *Big) Buffer() *gfx.Buffer {
	return gfx.FromPix(b.Pix, b.W, b.H)
}
