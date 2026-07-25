// v70body.go : the M.A.X. V70 body reader - parses the variable length mission body, after the SmartFile header.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package missionfile

import (
	"encoding/binary"
	"fmt"
	"io"
)

// The V70 body follows the fixed header parsed by ReadInfo. This file reads its terrain and cargo maps.
// Unit, team, and heat-map data depend on the SmartFile shared-object graph.
// See docs/formats/v70-savegame.md.
//
// C++: the start of SaveLoad_LoadFormatV70 (saveload.cpp) up to the two map reads.

// v70BodyPrefix is the number of bytes between the 170-byte fixed header and the surface map.
// opponent(i8) + turn_timer(u16) + endturn(u16) + play_mode(i8) = 6, then SaveLoad_LoadOptions = 12 * int32 = 48.
const v70BodyPrefix = 6 + 12*4

// CargoKind is the resource type held in a cargo map cell.
type CargoKind uint8

const (
	CargoNone CargoKind = iota
	CargoRaw
	CargoFuel
	CargoGold
)

// Cargo map cell bit layout, matching internal/economy and CARGO_*:
// - the low 5 bits are the deposit amount
// - one bit flags the resource type
// - the high bits (cleared here) hold per team surveyed flags
const (
	cargoAmountMask = 0x1F
	cargoFuelBit    = 0x20
	cargoGoldBit    = 0x40
	cargoRawBit     = 0x80
	cargoAmountCap  = 16
)

// MissionMap contains the row-major terrain and raw cargo cells decoded from a V70 mission.
type MissionMap struct {
	W, H    int
	Surface []uint8  // world.Surface* value per cell.
	Cargo   []uint16 // Raw cargo value per cell; decode with DepositAt.
}

// DepositAt returns a cell's resource type and its game-capped amount. Off-map and empty cells return CargoNone.
func (m *MissionMap) DepositAt(x, y int) (CargoKind, int) {
	if x < 0 || y < 0 || x >= m.W || y >= m.H {
		return CargoNone, 0
	}
	c := m.Cargo[y*m.W+x]
	amount := int(c & cargoAmountMask)
	if amount > cargoAmountCap {
		amount = cargoAmountCap
	}
	switch {
	case c&cargoGoldBit != 0:
		return CargoGold, amount
	case c&cargoFuelBit != 0:
		return CargoFuel, amount
	case c&cargoRawBit != 0:
		return CargoRaw, amount
	default:
		return CargoNone, 0
	}
}

// ReadMissionMap reads the surface and resource maps from a V70 stream. The file does not store map dimensions, so w and h come from the mission's .WRL file.
//
// C++: SaveLoad_LoadFormatV70: the surface map + ResourceManager_CargoMap reads.
func ReadMissionMap(r io.Reader, w, h int) (*MissionMap, error) {
	if w <= 0 || h <= 0 {
		return nil, fmt.Errorf("missionfile: invalid map size %dx%d", w, h)
	}
	if _, err := ReadInfo(r); err != nil { // consumes + validates the 170-byte header
		return nil, err
	}
	if _, err := io.CopyN(io.Discard, r, v70BodyPrefix); err != nil {
		return nil, fmt.Errorf("missionfile: skip options prefix: %w", err)
	}

	n := w * h
	surface := make([]uint8, n)
	if _, err := io.ReadFull(r, surface); err != nil {
		return nil, fmt.Errorf("missionfile: read surface map: %w", err)
	}
	rawCargo := make([]byte, n*2)
	if _, err := io.ReadFull(r, rawCargo); err != nil {
		return nil, fmt.Errorf("missionfile: read cargo map: %w", err)
	}
	cargo := make([]uint16, n)
	for i := range cargo {
		cargo[i] = binary.LittleEndian.Uint16(rawCargo[i*2:])
	}
	return &MissionMap{W: w, H: h, Surface: surface, Cargo: cargo}, nil
}

/* // note: unused code in Go port
// ReadMissionMapFile is ReadMissionMap for a file path.
func ReadMissionMapFile(path string, w, h int) (*MissionMap, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return ReadMissionMap(f, w, h)
}
*/
