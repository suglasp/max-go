// frameinfo.go : decodes unit sprite frame ranges and turret offsets.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
// C++ ported : unitinfo.cpp / resource_manager.hpp - image frame index tables
package game

// FrameInfo describes a unit type's body, turret, firing, and connector frame ranges, plus turret mount offsets for each facing.
// Each type stores it in a small "D_<unit>" resource.
//
// C++ ported : unit.hpp - FrameInfo
type FrameInfo struct {
	ImageBase, ImageCount             int
	TurretImageBase, TurretImageCount int
	FiringImageBase, FiringImageCount int
	ConnectorImageBase                int
	ConnectorImageCount               int
	AngleOffsets                      [8]Point
	HasData                           bool
}

// DecodeFrameInfo parses a unit's D_<unit> resource.
// It reads eight signed base and count bytes for body, turret, firing, and connector frames, then optionally sixteen turret offsets. Short data returns a zero value with HasData false.
//
// C++ ported : installer/sprites.cpp - Sprites::DecodeFrameInfo
func DecodeFrameInfo(data []byte) FrameInfo {
	var fi FrameInfo
	if len(data) < 8 {
		return fi
	}
	s8 := func(i int) int { return int(int8(data[i])) }
	fi.ImageBase = s8(0)
	fi.ImageCount = s8(1)
	fi.TurretImageBase = s8(2)
	fi.TurretImageCount = s8(3)
	fi.FiringImageBase = s8(4)
	fi.FiringImageCount = s8(5)
	fi.ConnectorImageBase = s8(6)
	fi.ConnectorImageCount = s8(7)
	fi.HasData = true
	if len(data) >= 8+16 {
		for i := 0; i < 8; i++ {
			fi.AngleOffsets[i] = Point{X: s8(8 + i*2), Y: s8(8 + i*2 + 1)}
		}
	}
	return fi
}
