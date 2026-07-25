// writer.go : encodes a world.Map back into .WRL bytes (the exact inverse of ParseMap) for the map editor.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
//
// Note: the original game never writes .WRL files; this is a port addition for
// the -mapeditor map/scenario editor. Format: docs/formats/wrl.md.
package world

import (
	"encoding/binary"
	"fmt"
	"os"

	"github.com/suglasp/max-go/internal/gfx"
)

// passValue inverts passData: an engine surface flag back to the raw 0..3 pass
// table byte the file stores. passData maps four distinct raw values to four
// distinct flags, so the reverse is unambiguous.
func passValue(surface uint8) (byte, bool) {
	for raw, s := range passData {
		if s == surface {
			return byte(raw), true
		}
	}
	return 0, false
}

// EncodeMap serialises a Map back into .WRL file bytes. It is the exact inverse
// of ParseMap: for a file parsed with ParseMap, EncodeMap reproduces the
// original bytes. The map editor uses it to save edited and newly created
// worlds. Parse with ParseMap, not LoadMap: LoadMap applies the hash keyed
// retail data fixes, which would change TileIDs/Surface away from the file.
func EncodeMap(m *Map) ([]byte, error) {
	cells := m.W * m.H
	palLen := gfx.PaletteSize * gfx.PaletteStride

	switch {
	case m.W <= 0 || m.H <= 0:
		return nil, fmt.Errorf("world: cannot encode a %dx%d map", m.W, m.H)
	case len(m.Minimap) != cells:
		return nil, fmt.Errorf("world: minimap has %d bytes, want %d", len(m.Minimap), cells)
	case len(m.TileIDs) != cells:
		return nil, fmt.Errorf("world: tile ids have %d entries, want %d", len(m.TileIDs), cells)
	case m.TileCount < 0:
		return nil, fmt.Errorf("world: negative tile count %d", m.TileCount)
	case len(m.Tiles) != m.TileCount*TileSize*TileSize:
		return nil, fmt.Errorf("world: tile pixels have %d bytes, want %d", len(m.Tiles), m.TileCount*TileSize*TileSize)
	case len(m.PaletteRaw) != palLen:
		return nil, fmt.Errorf("world: raw palette has %d bytes, want %d", len(m.PaletteRaw), palLen)
	case len(m.PassTable) != m.TileCount:
		return nil, fmt.Errorf("world: pass table has %d entries, want %d", len(m.PassTable), m.TileCount)
	}

	size := 9 + cells + cells*2 + 2 + len(m.Tiles) + palLen + m.TileCount
	out := make([]byte, size)
	pos := 0

	// Header: the 5 magic/version bytes, then the single layer's int16 W and H.
	copy(out[pos:], m.Magic[:])
	pos += 5
	binary.LittleEndian.PutUint16(out[pos:], uint16(m.W))
	pos += 2
	binary.LittleEndian.PutUint16(out[pos:], uint16(m.H))
	pos += 2

	// Minimap: one palette index per cell, verbatim.
	copy(out[pos:], m.Minimap)
	pos += cells

	// Tile index layer: one uint16 per cell.
	for _, id := range m.TileIDs {
		binary.LittleEndian.PutUint16(out[pos:], id)
		pos += 2
	}

	binary.LittleEndian.PutUint16(out[pos:], uint16(m.TileCount))
	pos += 2

	// Tile pixels then the raw 8-bit palette, both verbatim.
	copy(out[pos:], m.Tiles)
	pos += len(m.Tiles)
	copy(out[pos:], m.PaletteRaw)
	pos += palLen

	// Pass table: one raw 0..3 byte per tile, inverted from the surface flag.
	for i, surface := range m.PassTable {
		raw, ok := passValue(surface)
		if !ok {
			return nil, fmt.Errorf("world: pass table entry %d has unmappable surface 0x%x", i, surface)
		}
		out[pos] = raw
		pos++
	}

	return out, nil
}

// SaveMap encodes m and writes it to a .WRL file at path. It is the write side
// counterpart of LoadMap, minus the retail data fixes (a map edited in memory
// carries none, so SaveMap is a plain encode).
func SaveMap(path string, m *Map) error {
	data, err := EncodeMap(m)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}
