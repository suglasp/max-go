// fonts.go : decodes GNW bitmap fonts and manages the active font set.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
// C++ ported : text.c - GNW font/text rendering API
//
// Package fonts decodes FONT0.FON through FONT6.FON and renders them with
// text.c semantics. docs/formats/fon.md documents the file format.
//
// A later port build replaced GNW bitmap text with FreeType TTF (its
// PATCHES.RES FONT_1/2/5 entries are TTF blobs, not .FON data); this
// port sticks to the original bitmap fonts.
package fonts

import (
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
)

// Text flags packed into the color argument (text.h). The low byte is
// the palette index to draw with.
const (
	FlagOutline   = 0x10000
	FlagUnderline = 0x20000
	FlagMonospace = 0x40000
	ColorMask     = 0xFF
)

// GNW font numbers (text.h). The game data provides fonts 0..6.
const (
	Font0 = iota
	Font1
	Font2
	Font3
	Font4
	Font5
	Font6
	FontCount = 7
)

// Glyph is one character: Width columns by Font.Height rows of 1-bit
// pixels, each row packed with the most significant bit first into ceil(Width/8) bytes.
type Glyph struct {
	Width int
	rows  []byte
}

// Pixel reports whether the glyph pixel at (x, y) is set.
func (g *Glyph) Pixel(x, y int) bool {
	pitch := (g.Width + 7) / 8
	return g.rows[y*pitch+x/8]&(0x80>>(x%8)) != 0
}

// Font is a decoded GNW bitmap font.
type Font struct {
	Height  int
	Spacing int
	Glyphs  []Glyph
}

// fonHeaderSize covers count, height, spacing plus the two raw
// pointers the original engine dumped to disk along with its struct.
const fonHeaderSize = 20

// Decode parses a .FON file.
//
// The format is a memory dump of GNW's font struct: {int32 count,
// int32 height, int32 spacing, 8 junk bytes (pointers)}, then count
// entries {int32 width, int32 data_offset}, then the 1bpp glyph rows.
func Decode(data []byte) (*Font, error) {
	if len(data) < fonHeaderSize {
		return nil, fmt.Errorf("fonts: header truncated (%d bytes)", len(data))
	}
	count := int(int32(binary.LittleEndian.Uint32(data[0:4])))
	f := &Font{
		Height:  int(int32(binary.LittleEndian.Uint32(data[4:8]))),
		Spacing: int(int32(binary.LittleEndian.Uint32(data[8:12]))),
	}
	if (count <= 0) || (count > 256) || (f.Height <= 0) || (f.Height > 64) || (f.Spacing < 0) || (f.Spacing > 16) {
		return nil, fmt.Errorf("fonts: implausible header (count=%d height=%d spacing=%d)",
			count, f.Height, f.Spacing)
	}
	tableEnd := fonHeaderSize + count*8
	if len(data) < tableEnd {
		return nil, fmt.Errorf("fonts: glyph table truncated")
	}
	bitmap := data[tableEnd:]

	f.Glyphs = make([]Glyph, count)
	for i := 0; i < count; i++ {
		w := int(int32(binary.LittleEndian.Uint32(data[fonHeaderSize+i*8:])))
		off := int(int32(binary.LittleEndian.Uint32(data[fonHeaderSize+i*8+4:])))
		if w < 0 || w > 64 {
			return nil, fmt.Errorf("fonts: glyph %d has implausible width %d", i, w)
		}
		size := f.Height * ((w + 7) / 8)
		if off < 0 || off+size > len(bitmap) {
			return nil, fmt.Errorf("fonts: glyph %d data out of bounds (off=%d size=%d have=%d)",
				i, off, size, len(bitmap))
		}
		f.Glyphs[i] = Glyph{Width: w, rows: bitmap[off : off+size]}
	}
	return f, nil
}

// LoadFile decodes one .FON file from disk.
func LoadFile(path string) (*Font, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	f, err := Decode(data)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return f, nil
}

// Set holds the game's fonts with a current selection, mirroring
// text.c's Text_SetFont/Text_GetFont globals.
type Set struct {
	fonts   [FontCount]*Font
	current int
}

// LoadSet reads FONT0.FON .. FONT6.FON from the game data directory.
//
// C++: Text_Init (text.cpp), reduced to the GNW bitmap fonts
func LoadSet(dir string) (*Set, error) {
	s := &Set{}
	for i := 0; i < FontCount; i++ {
		f, err := LoadFile(filepath.Join(dir, fmt.Sprintf("FONT%d.FON", i)))
		if err != nil {
			return nil, err
		}
		s.fonts[i] = f
	}
	return s, nil
}

// SetFont selects the current font.
//
// C++: Text_SetFont (text.cpp)
func (s *Set) SetFont(n int) error {
	if n < 0 || n >= FontCount || s.fonts[n] == nil {
		return fmt.Errorf("fonts: no font %d", n)
	}
	s.current = n
	return nil
}

// GetFont returns the current font number.
//
// C++: Text_GetFont (text.cpp)
func (s *Set) GetFont() int {
	return s.current
}

// Font returns the currently selected font.
func (s *Set) Font() *Font {
	return s.fonts[s.current]
}

// Get returns font n, or nil.
func (s *Set) Get(n int) *Font {
	if n < 0 || n >= FontCount {
		return nil
	}
	return s.fonts[n]
}
