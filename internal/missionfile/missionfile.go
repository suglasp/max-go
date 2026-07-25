// missionfile.go : reads the fixed SmartFile V70 header used by retail M.A.X. save, scenario, campaign, training, and demo files.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package missionfile

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
)

const (
	smartFileV70 = 70
	nameLen      = 30
	teamCount    = 4
	slotCount    = 5
)

// Category is the retail save_game_type translated to a mission category.
type Category int

const (
	CategoryCustom Category = iota
	CategoryTraining
	CategoryCampaign
	CategoryHotSeat
	CategoryMulti
	CategoryDemo
	CategoryScenario
	CategoryMultiPlayerScenario
	CategoryCount
)

// Info contains the SmartFile V70 header fields needed to list a mission or save in a menu. It does not contain game state.
type Info struct {
	Version      uint16
	SaveType     uint8
	Category     Category
	Name         string
	WorldIndex   uint8
	MissionIndex uint16
	TeamNames    [teamCount]string
	TeamType     [slotCount]uint8
	TeamClan     [slotCount]uint8
	RNGSeed      uint32
}

// ReadInfo decodes the fixed SmartFile V70 header from r.
//
// C++: SaveLoad_GetSaveFileInfoV70 (saveload.cpp)
func ReadInfo(r io.Reader) (*Info, error) {
	var info Info

	if err := read(r, &info.Version); err != nil {
		return nil, fmt.Errorf("missionfile: read version: %w", err)
	}
	if info.Version != smartFileV70 {
		return nil, fmt.Errorf("missionfile: invalid SmartFile version %d, want %d", info.Version, smartFileV70)
	}

	if err := read(r, &info.SaveType); err != nil {
		return nil, fmt.Errorf("missionfile: read save type: %w", err)
	}
	info.Category = translateSaveFileCategory(info.SaveType)
	if info.Category >= CategoryCount {
		return nil, fmt.Errorf("missionfile: invalid save file category for type %d", info.SaveType)
	}

	name, err := readName(r)
	if err != nil {
		return nil, fmt.Errorf("missionfile: read save name: %w", err)
	}
	info.Name = name

	if err := read(r, &info.WorldIndex); err != nil {
		return nil, fmt.Errorf("missionfile: read world index: %w", err)
	}
	if err := read(r, &info.MissionIndex); err != nil {
		return nil, fmt.Errorf("missionfile: read mission index: %w", err)
	}

	for i := range info.TeamNames {
		name, err := readName(r)
		if err != nil {
			return nil, fmt.Errorf("missionfile: read team name %d: %w", i, err)
		}
		info.TeamNames[i] = name
	}
	if err := read(r, &info.TeamType); err != nil {
		return nil, fmt.Errorf("missionfile: read team types: %w", err)
	}
	if err := read(r, &info.TeamClan); err != nil {
		return nil, fmt.Errorf("missionfile: read team clans: %w", err)
	}
	if err := read(r, &info.RNGSeed); err != nil {
		return nil, fmt.Errorf("missionfile: read rng seed: %w", err)
	}

	return &info, nil
}

// C++: SaveLoad_TranslateSaveFileCategory (saveload.cpp)
func translateSaveFileCategory(saveType uint8) Category {
	const (
		gameTypeCustom              = 0x0
		gameTypeTraining            = 0x1
		gameTypeCampaign            = 0x2
		gameTypeHotSeat             = 0x3
		gameTypeMulti               = 0x4
		gameTypeDemo                = 0x5
		gameTypeScenario            = 0x8
		gameTypeMultiPlayerScenario = 0x9
	)

	switch saveType {
	case gameTypeCustom:
		return CategoryCustom
	case gameTypeTraining:
		return CategoryTraining
	case gameTypeCampaign:
		return CategoryCampaign
	case gameTypeHotSeat:
		return CategoryHotSeat
	case gameTypeMulti:
		return CategoryMulti
	case gameTypeDemo:
		return CategoryDemo
	case gameTypeScenario:
		return CategoryScenario
	case gameTypeMultiPlayerScenario:
		return CategoryMultiPlayerScenario
	default:
		return CategoryCount
	}
}

// read decodes one little endian binary value from a mission stream.
func read(r io.Reader, data any) error {
	return binary.Read(r, binary.LittleEndian, data)
}

// readName reads a fixed mission string and trims trailing zero padding.
func readName(r io.Reader) (string, error) {
	var raw [nameLen]byte
	if err := read(r, &raw); err != nil {
		return "", err
	}
	if i := bytes.IndexByte(raw[:], 0); i >= 0 {
		return string(raw[:i]), nil
	}
	return string(raw[:]), nil
}
