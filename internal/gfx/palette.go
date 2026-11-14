// palette.go : loads and manipulates the 256-entry palette and recolor tables.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
// C++ ported : color.cpp - palette loading and manipulation
package gfx

import (
	"encoding/binary"
	"fmt"
	"os"

	"github.com/suglasp/max-go/internal/resfile"
)

// Palette and color table handling ported from color.cpp. The format
// of COLOR.PAL is documented in docs/formats/pal.md.

const (
	// PaletteSize is the number of palette entries.
	PaletteSize = 256
	// PaletteStride is the bytes per entry (R, G, B).
	PaletteStride = 3

	rgb555Count = 1 << 15
	palFileTag  = 0x4E455743 // 'NEWC', little endian

	// PalFileBaseSize is a COLOR.PAL without the intensity table: the palette plus the RGB555 nearest index table.
	PalFileBaseSize = PaletteSize*PaletteStride + rgb555Count
	// PalFileFullSize adds the 'NEWC' tag and its 256x256 intensity table.
	PalFileFullSize = PalFileBaseSize + 4 + PaletteSize*PaletteSize
)

// GNW color indices into the system palette (color.h). Note that the
// actual palette colors do not all match their historic names.
const (
	ColorBlack  uint8 = 0x00
	ColorRed    uint8 = 0x01
	ColorGreen  uint8 = 0x02
	ColorBlue   uint8 = 0x03
	ColorYellow uint8 = 0x04
)

// Palette is a 256-entry RGB palette with 6-bit components (0..63),
// as stored in the game files.
type Palette [PaletteSize * PaletteStride]uint8

// RGB returns the 6-bit components of entry i.
func (p *Palette) RGB(i int) (r, g, b uint8) {
	return p[i*PaletteStride], p[i*PaletteStride+1], p[i*PaletteStride+2]
}

// SetRGB sets the 6-bit components of entry i.
func (p *Palette) SetRGB(i int, r, g, b uint8) {
	p[i*PaletteStride] = r
	p[i*PaletteStride+1] = g
	p[i*PaletteStride+2] = b
}

// RGBA8 returns entry i scaled to 8-bit components, multiplying by 4
// exactly like the original display path.
//
// C++: Color_SetSystemPaletteEntry (color.cpp)
func (p *Palette) RGBA8(i int) (r, g, b uint8) {
	return p[i*PaletteStride] * 4, p[i*PaletteStride+1] * 4, p[i*PaletteStride+2] * 4
}

// PalFile is the parsed content of COLOR.PAL: the game palette, an
// RGB555 to index lookup table, and an optional intensity table.
//
// C++: Color_Init (color.cpp)
type PalFile struct {
	Palette  Palette
	RGBIndex []uint8 // 32768 entries, RGB555 -> palette index
	// Intensity maps [color][level] to a palette index; nil when the
	// file has no 'NEWC' section (true for the retail COLOR.PAL).
	Intensity *[PaletteSize][PaletteSize]uint8
}

// LoadBasePalette reads COLOR.PAL from the game data folder and returns its
// palette. On a missing or invalid file it returns the zero palette and the
// error, so a caller that only wants a fallback can ignore the error.
func LoadBasePalette(dataDir string) (Palette, error) {
	data, err := os.ReadFile(resfile.DataPath(dataDir, "COLOR.PAL"))
	if err != nil {
		return Palette{}, err
	}

	pf, err := ParsePalFile(data)
	if err != nil {
		return Palette{}, err
	}

	return pf.Palette, nil
}

// ParsePalFile parses COLOR.PAL data.
func ParsePalFile(data []byte) (*PalFile, error) {
	const baseSize = PalFileBaseSize
	if len(data) < baseSize {
		return nil, fmt.Errorf("gfx: COLOR.PAL too short: %d bytes, want >= %d", len(data), baseSize)
	}
	f := &PalFile{RGBIndex: make([]uint8, rgb555Count)}
	copy(f.Palette[:], data[:PaletteSize*PaletteStride])
	copy(f.RGBIndex, data[PaletteSize*PaletteStride:baseSize])

	rest := data[baseSize:]
	if len(rest) >= 4 && binary.LittleEndian.Uint32(rest) == palFileTag {
		rest = rest[4:]
		if len(rest) < PaletteSize*PaletteSize {
			return nil, fmt.Errorf("gfx: COLOR.PAL intensity table truncated: %d bytes", len(rest))
		}
		var it [PaletteSize][PaletteSize]uint8
		for i := range it {
			copy(it[i][:], rest[i*PaletteSize:(i+1)*PaletteSize])
		}
		f.Intensity = &it
	}
	return f, nil
}

/* // note: ported but unused code in Go port
// RGB2Color maps a 15-bit RGB555 value (r<<10 | g<<5 | b, 5-bit
// components) to the nearest palette index using the file's lookup
// table.
//
// C++: Color_RGB2Color (color.cpp)
func (f *PalFile) RGB2Color(rgb555 int) uint8 {
	return f.RGBIndex[rgb555]
}

// ColorIntensity scales a color via the intensity table. When the
// file carries no intensity table the color is returned unchanged
// (deviation from the original, which would read a zeroed table and
// return black; the retail COLOR.PAL has no table).
//
// C++: Color_ColorIntensity (color.cpp)
func (f *PalFile) ColorIntensity(intensity int, color uint8) uint8 {
	if f.Intensity == nil {
		return color
	}
	return f.Intensity[color][intensity/512]
}

// LightenTable returns the recolor table used by GNW's lighten_buf
// (intensity 0x12600).
//
// C++: lighten_buf (grbuf.c)
func (f *PalFile) LightenTable() *[256]uint8 {
	var t [256]uint8
	for i := range t {
		t[i] = f.ColorIntensity(0x12600, uint8(i))
	}
	return &t
}
*/

// DarkeningTable builds the minimap fog of war recolor LUT: every palette
// entry is mapped to the nearest palette index of its own color at roughly
// half brightness (each 6-bit component clamped to a floor of 7, then scaled
// by 31/63). It dims minimap cells that are explored but not currently scanned.
// Unit ground shadows use the lighter BrightnessTable(5) mapping instead.
//
// C++: World::ApplyPaletteAndTints, the ResourceManager_DarkeningColorIndexTable
// loop (world.cpp)
func (p *Palette) DarkeningTable() *[256]uint8 {
	var t [256]uint8
	for i := 0; i < PaletteSize; i++ {
		r, g, b := p.RGB(i)
		rr := uint8((max(7, int(r)) * 31) / 63)
		gg := uint8((max(7, int(g)) * 31) / 63)
		bb := uint8((max(7, int(b)) * 31) / 63)
		t[i] = p.MapColor(rr, gg, bb, false)
	}
	return &t
}

// RedTintTable builds the message banner red alert recolor LUT: every palette
// entry is mapped to the nearest palette index of a red-tinted version of its
// color. Green and blue become zero while red is raised from a minimum value.
// factor controls the strength (31
// for most worlds, 63 for snow worlds = a gentler tint); 0 defaults to 31.
//
// C++ ported : message_manager.cpp - Color_GenerateIntensityTable3 with
// World::GetRedTintParameters (red_level 63, green_level/blue_level 0).
func (p *Palette) RedTintTable(factor int) *[256]uint8 {
	if factor <= 0 {
		factor = 31
	}
	return p.IntensityTable3(63, 0, 0, factor)
	/* // note: unused code in Go port
	// the hand inlined copy of IntensityTable3(63, 0, 0, factor) it used before.
	var t [256]uint8
	for i := 0; i < PaletteSize; i++ {
		r, _, _ := p.RGB(i)
		// red_level=63: red = (max(r, 63/4) * 63) / factor; green/blue level 0 -> 0.
		red := (max(int(r), 63/4) * 63) / factor
		t[i] = p.MapColor(uint8(red), 0, 0, false)
	}
	return &t
	*/
}

// BrightnessTable builds one level of the 7-level brightness recolor ramp:
// every palette entry is mapped to the nearest palette index of its own color
// scaled to level/7 brightness (level 0 = black, 7 = unchanged). Palette index
// 31 (the constant white slot) is preserved. Unit ground shadows use level 5
// (~71% brightness): a light dim that keeps the terrain texture
// visible under the silhouette.
//
// C++: the "Generate brightness color index table (7 levels)" loop in
// World::ApplyPaletteAndTints, indexed as BrightnessColorIndexTable[level*256]
// (world.cpp); unit shadows read level 5 in Gfx_DecodeShadow (gfx.cpp).
func (p *Palette) BrightnessTable(level int) *[256]uint8 {
	var t [256]uint8
	for i := 0; i < PaletteSize; i++ {
		if i == 31 {
			t[i] = 31
			continue
		}
		r, g, b := p.RGB(i)
		rr := uint8(int(r) * level / 7)
		gg := uint8(int(g) * level / 7)
		bb := uint8(int(b) * level / 7)
		t[i] = p.MapColor(rr, gg, bb, false)
	}
	return &t
}

// ColorDistance returns the squared euclidean distance between two
// RGB triplets.
//
// C++: Color_GetColorDistance (color.cpp)
func ColorDistance(r1, g1, b1, r2, g2, b2 uint8) int {
	dr := int(r1) - int(r2)
	dg := int(g1) - int(g2)
	db := int(b1) - int(b2)
	return dr*dr + dg*dg + db*db
}

// MapColor returns the palette index closest to (r, g, b). Unless
// fullScan is set, the animated palette ranges 9..31 and 96..127
// (color cycling, team colors) are excluded from the search.
//
// C++: Color_MapColor (color.cpp)
func (p *Palette) MapColor(r, g, b uint8, fullScan bool) uint8 {
	best := 0
	bestDist := int(^uint(0) >> 1)

	for i := 0; i < PaletteSize; i++ {
		if !fullScan && ((i >= 9 && i <= 31) || (i >= 96 && i <= 127)) {
			continue
		}
		pr, pg, pb := p.RGB(i)
		d := ColorDistance(pr, pg, pb, r, g, b)
		if d < bestDist {
			bestDist = d
			best = i
			if d == 0 {
				break
			}
		}
	}
	return uint8(best)
}

// IntensityTable1 builds a recolor table blending every palette entry
// towards (r, g, b) with weight factor1/(factor1+factor2), snapping
// each result to the nearest palette entry by full search.
//
// C++: Color_GenerateIntensityTable1 (color.cpp)
func (p *Palette) IntensityTable1(r, g, b, factor1, factor2 int) *[256]uint8 {
	var table [256]uint8
	sum := factor1 + factor2

	for i := 0; i < PaletteSize; i++ {
		pr, pg, pb := p.RGB(i)
		cr := uint8((int(pr)*factor2 + r*factor1) / sum)
		cg := uint8((int(pg)*factor2 + g*factor1) / sum)
		cb := uint8((int(pb)*factor2 + b*factor1) / sum)

		index := i
		dist := ColorDistance(pr, pg, pb, cr, cg, cb)
		for j := 0; j < PaletteSize && dist > 0; j++ {
			jr, jg, jb := p.RGB(j)
			if d := ColorDistance(jr, jg, jb, cr, cg, cb); d < dist {
				dist = d
				index = j
			}
		}
		table[i] = uint8(index)
	}
	return &table
}

// IntensityTable2 builds a tinting table that pushes every palette
// entry towards the hue (rLevel, gLevel, bLevel) while keeping its
// brightness, using MapColor for the final snap.
//
// C++: Color_GenerateIntensityTable2 (color.cpp)
func (p *Palette) IntensityTable2(rLevel, gLevel, bLevel int) *[256]uint8 {
	var table [256]uint8

	if rLevel == 0 && gLevel == 0 && bLevel == 0 {
		rLevel, gLevel, bLevel = 1, 1, 1
	}
	maxLevel := max(rLevel, max(gLevel, bLevel))

	for i := 0; i < PaletteSize; i++ {
		r, g, b := p.RGB(i)
		maxColor := (int(max(r, max(g, b))) + maxLevel) / 2
		table[i] = p.MapColor(
			uint8(maxColor*rLevel/maxLevel),
			uint8(maxColor*gLevel/maxLevel),
			uint8(maxColor*bLevel/maxLevel),
			false)
	}
	return &table
}

// IntensityTable3 builds a brightness table scaling every palette
// entry by level/factor per channel, with a floor for each channel of
// level/4.
//
// C++: Color_GenerateIntensityTable3 (color.cpp)
func (p *Palette) IntensityTable3(rLevel, gLevel, bLevel, factor int) *[256]uint8 {
	var table [256]uint8

	for i := 0; i < PaletteSize; i++ {
		r, g, b := p.RGB(i)
		table[i] = p.MapColor(
			uint8(max(int(r), rLevel/4)*rLevel/factor),
			uint8(max(int(g), gLevel/4)*gLevel/factor),
			uint8(max(int(b), bLevel/4)*bLevel/factor),
			false)
	}
	return &table
}
