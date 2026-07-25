// world.go : the M.A.X. world map loader: reads .WRL files in preview (minimap+palette) and full (tiles+surfaces) tiers.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
//
// Note: Package world reads .WRL map files, in two tiers like the original:
// a lightweight preview (minimap + palette, for the planet selection
// menu) and the full map load (tile graphics + surface types, for the
// tactical screen). Format: docs/formats/wrl.md.
package world

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"

	"github.com/suglasp/max-go/internal/gfx"
)

// TileSize is the pixel size of a map tile (GFX_MAP_TILE_SIZE).
const TileSize = 64

// Surface types of map cells (bit flags; units carry a mask of the
// surfaces they can occupy).
//
// C++: SURFACE_TYPE_* (enums.hpp)
const (
	SurfaceNone  uint8 = 0x0
	SurfaceLand  uint8 = 0x1
	SurfaceWater uint8 = 0x2
	SurfaceCoast uint8 = 0x4
	SurfaceAir   uint8 = 0x8
)

// passData maps the raw pass table byte (0..3) of the file to a
// surface type.
//
// C++: PassData (world.cpp, LoadTilesAndMetadata)
var passData = [4]uint8{SurfaceLand, SurfaceWater, SurfaceCoast, SurfaceAir}

// The palette entries a loaded map installs into the system palette;
// everything outside [TerrainPaletteFirst, TerrainPaletteLast] keeps
// the current (COLOR.PAL / menu) colors.
//
// C++: LoadTilesAndMetadata palette loop [64*3, 160*3)
const (
	TerrainPaletteFirst = 64
	TerrainPaletteLast  = 159
)

// Preview is the lightweight part of a .WRL file: enough to show the
// world in the planet selection menu.
//
// C++: Menu_LoadPlanetMinimap (menu.cpp), World preview mode
type Preview struct {
	Magic   [5]byte
	W, H    int     // map size in cells
	Minimap []uint8 // W*H palette indices, stored as W strips of H bytes
	// Palette in 6-bit components (the file stores 8-bit, the engine
	// divides by 4).
	Palette gfx.Palette
}

// LoadPreview reads the preview portion of a .WRL file.
func LoadPreview(path string) (*Preview, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return ParsePreview(data)
}

// ParsePreview parses the preview portion of .WRL data.
func ParsePreview(data []byte) (*Preview, error) {
	p := &Preview{}
	if len(data) < 9 {
		return nil, fmt.Errorf("world: header truncated (%d bytes)", len(data))
	}
	copy(p.Magic[:], data[:5])
	p.W = int(int16(binary.LittleEndian.Uint16(data[5:7])))
	p.H = int(int16(binary.LittleEndian.Uint16(data[7:9])))
	if p.W <= 0 || p.H <= 0 || p.W > 256 || p.H > 256 {
		return nil, fmt.Errorf("world: implausible map size %dx%d", p.W, p.H)
	}
	pos := 9

	cells := p.W * p.H
	if len(data) < pos+cells {
		return nil, fmt.Errorf("world: minimap truncated")
	}
	p.Minimap = data[pos : pos+cells]
	pos += cells

	// Skip the tile index layer (int16 per cell).
	pos += cells * 2
	if len(data) < pos+2 {
		return nil, fmt.Errorf("world: tile count truncated")
	}
	tileCount := int(int16(binary.LittleEndian.Uint16(data[pos:])))
	pos += 2
	if tileCount < 0 {
		return nil, fmt.Errorf("world: negative tile count")
	}

	// Skip the tile pixel data.
	pos += tileCount * TileSize * TileSize
	if len(data) < pos+gfx.PaletteSize*gfx.PaletteStride {
		return nil, fmt.Errorf("world: palette truncated (tiles=%d)", tileCount)
	}
	for i := 0; i < gfx.PaletteSize*gfx.PaletteStride; i++ {
		p.Palette[i] = data[pos+i] / 4
	}
	return p, nil
}

// MinimapBuffer wraps the minimap bytes as a buffer W wide, exactly
// how the original treats them (the file stores column strips; the
// engine reads them into a buffer ordered by row unchanged, and the
// retail 112x112 maps render correctly that way).
func (p *Preview) MinimapBuffer() *gfx.Buffer {
	return gfx.FromPix(p.Minimap, p.W, p.H)
}

// Map is a fully loaded .WRL world: everything the tactical renderer
// and the passability checks need. Only one Map is loaded at a time
// (1-2 MB); the 24 previews stay cheap.
//
// C++: class World in full load mode (world.cpp, LoadFullMap)
type Map struct {
	Preview
	TileCount int
	TileIDs   []uint16 // W*H tile index per cell, row major
	Tiles     []uint8  // TileCount consecutive 64x64 pixel blocks
	PassTable []uint8  // per tile surface type, already mapped to Surface*
	Surface   []uint8  // W*H surface type per cell
	// PaletteRaw is the untouched 8-bit palette exactly as stored in the file
	// (768 bytes). Preview.Palette holds the same colors downsampled to 6-bit,
	// which loses the low two bits, so only these raw bytes reproduce the file
	// unchanged. EncodeMap writes them (see writer.go).
	PaletteRaw []byte
}

// LoadMap reads a full .WRL file and applies the known retail data
// bug fixes (keyed by file hash, so modified files are left alone).
func LoadMap(path string) (*Map, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	m, err := ParseMap(data)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", filepath.Base(path), err)
	}
	sum := sha256.Sum256(data)
	m.applyBugFixes(hex.EncodeToString(sum[:]))
	return m, nil
}

// ParseMap parses a complete .WRL file.
//
// C++: World::LoadFullMap + LoadTilesAndMetadata + LoadMapTiles
// (world.cpp); the low resolution 32x32 downsampling mode
// (ResourceManager_DisableEnhancedGraphics) is not ported.
func ParseMap(data []byte) (*Map, error) {
	pre, err := ParsePreview(data)
	if err != nil {
		return nil, err
	}
	m := &Map{Preview: *pre}
	cells := m.W * m.H
	pos := 9 + cells // past header and minimap

	// Tile index layer: one uint16 per cell.
	if len(data) < pos+cells*2 {
		return nil, fmt.Errorf("world: tile ids truncated")
	}
	m.TileIDs = make([]uint16, cells)
	for i := range m.TileIDs {
		m.TileIDs[i] = binary.LittleEndian.Uint16(data[pos+i*2:])
	}
	pos += cells * 2

	m.TileCount = int(binary.LittleEndian.Uint16(data[pos:]))
	pos += 2

	// Tile pixel data: TileCount blocks of 64x64 indices.
	want := m.TileCount * TileSize * TileSize
	if len(data) < pos+want {
		return nil, fmt.Errorf("world: tile pixels truncated (%d tiles)", m.TileCount)
	}
	m.Tiles = data[pos : pos+want]
	pos += want

	// Palette: ParsePreview already stored it downsampled to 6-bit; keep a copy
	// of the untouched 8-bit bytes too, so EncodeMap can reproduce the file
	// exactly (the retail palettes are not all multiples of 4, so the 6-bit
	// copy alone cannot).
	palLen := gfx.PaletteSize * gfx.PaletteStride
	m.PaletteRaw = append([]byte(nil), data[pos:pos+palLen]...)
	pos += palLen

	// Pass table: one surface byte per tile.
	if len(data) < pos+m.TileCount {
		return nil, fmt.Errorf("world: pass table truncated")
	}
	m.PassTable = make([]uint8, m.TileCount)
	for i, v := range data[pos : pos+m.TileCount] {
		if int(v) >= len(passData) {
			return nil, fmt.Errorf("world: pass table value %d out of range", v)
		}
		m.PassTable[i] = passData[v]
	}

	// Surface map for each cell, validating the tile ids on the way.
	//
	// C++: LoadTilesAndMetadata surface map loop
	m.Surface = make([]uint8, cells)
	for i, id := range m.TileIDs {
		if int(id) >= m.TileCount {
			return nil, fmt.Errorf("world: cell %d tile id %d out of range (%d tiles)",
				i, id, m.TileCount)
		}
		m.Surface[i] = m.PassTable[id]
	}
	return m, nil
}

// Tile returns a view of tile id i as a 64x64 buffer.
func (m *Map) Tile(i int) *gfx.Buffer {
	return gfx.FromPix(m.Tiles[i*TileSize*TileSize:(i+1)*TileSize*TileSize], TileSize, TileSize)
}

// SurfaceAt returns the surface type at a grid cell, SurfaceNone
// outside the map.
//
// C++: World::GetSurfaceType (world.cpp)
func (m *Map) SurfaceAt(x, y int) uint8 {
	if x < 0 || x >= m.W || y < 0 || y >= m.H {
		return SurfaceNone
	}
	return m.Surface[y*m.W+x]
}

// ApplyTerrainPalette installs the map's terrain palette entries
// (64..159) into a system palette, leaving the rest untouched.
//
// C++: LoadTilesAndMetadata palette install (world.cpp)
func (m *Map) ApplyTerrainPalette(dst *gfx.Palette) {
	for i := TerrainPaletteFirst; i <= TerrainPaletteLast; i++ {
		r, g, b := m.Palette.RGB(i)
		dst.SetRGB(i, r, g, b)
	}
}

// Known retail data errors, patched like the C++ does, but only when
// the file hash matches the original reference, so edited files are left
// alone. The C++ SNOW_5 fix (injecting 4 KB of replacement art for a
// corrupt tile) is not ported.
//
// C++: World::ApplyBugFixes (world.cpp)
const (
	hashGreen4 = "dbcfc4495334640776e6a0b1776651f904583aa87f193a43436e6b1f04635241"
	hashSnow3  = "a625d409ca1d5cd24cdeed165813e5b79c538b123491791016372492f5106801"
)

// applyBugFixes patches known bad retail map data identified by file hash.
func (m *Map) applyBugFixes(fileHash string) {
	switch fileHash {
	case hashGreen4:
		// Tile 190's pass table says water; it is land. Only the
		// surface map is corrected, exactly like the C++.
		for i, id := range m.TileIDs {
			if id == 190 {
				m.Surface[i] = SurfaceLand
			}
		}
	case hashSnow3:
		// Wrong tile placed at grid (57,57); the C++ swaps the tile id
		// after the surface map is built, so the surface stays as it is.
		m.TileIDs[m.W*56+56] = 55
	}
}
