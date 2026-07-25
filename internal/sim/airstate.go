// airstate.go : manages air-unit landing, takeoff, and altitude changes.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
//
// C++ ported : unitinfo.cpp - UnitInfo::Land / UnitInfo::Take
package sim

import (
	"github.com/suglasp/max-go/internal/assets"
	"github.com/suglasp/max-go/internal/game"
)

// Air units fly while FlagHovering is set. Landing clears it, so a plane blocks ground and sea movers and becomes a ground target.
// Taking off sets FlagHovering again. The simulation changes state immediately; the game screen adds the AirAlt animation and land or takeoff sound.
//
// C++ ported : unitinfo.cpp - UnitInfo::Land / UnitInfo::Take (clear/set HOVERING).

// IsAirUnit reports whether u is a mobile air unit.
func IsAirUnit(u *game.UnitInfo) bool {
	return u != nil && u.Flags.Has(game.FlagMobileAirUnit)
}

// IsHovering reports whether an air unit is currently flying (vs landed).
func IsHovering(u *game.UnitInfo) bool {
	return u != nil && u.Flags.Has(game.FlagHovering)
}

// CanTakeOff reports whether u is a landed air unit that can take off.
func (g *Game) CanTakeOff(u *game.UnitInfo) bool {
	return IsAirUnit(u) && !IsHovering(u)
}

// CanLand reports whether u is a flying air unit over a cell it can land on.
// This port only lands air units on a friendly LANDPAD.
//
// Vanilla M.A.X. also lets a plane land on open ground, where it becomes a vulnerable ground unit. A landing pad is only the rearm and refuel point.
// This port deliberately requires a friendly pad. Allowing open-ground landings requires solid LAND with no building, ground or sea unit, or landed plane.
// C++ ported : unitinfo.cpp - UnitInfo::Land (set down on the current cell).
func (g *Game) CanLand(u *game.UnitInfo) bool {
	if !IsAirUnit(u) || !IsHovering(u) {
		return false
	}
	return g.landingPadAt(u.GridX, u.GridY, u.Team) != nil
}

// landingPadAt returns the team's intact landing pad on (x, y), or nil.
func (g *Game) landingPadAt(x, y, team int) *game.UnitInfo {
	for _, e := range g.Units.Cells.At(x, y) {
		if e.Type == assets.LandPad && e.Team == team && e.Hits > 0 {
			return e
		}
	}
	return nil
}

// TakeOff sets FlagHovering, so a landed air unit flies over everything and is no longer a ground target.
// C++ ported : unitinfo.cpp - Take.
func (g *Game) TakeOff(u *game.UnitInfo) bool {
	if !g.CanTakeOff(u) {
		return false
	}
	u.Flags |= game.FlagHovering
	g.RefreshVisibility()
	return true
}

// LandAir clears FlagHovering and lands an air unit on its own cell, where it blocks ground and sea movers and is vulnerable as a ground unit.
// The name avoids Game.Land, which handles colony landing. C++ ported : unitinfo.cpp - Land.
func (g *Game) LandAir(u *game.UnitInfo) bool {
	if !g.CanLand(u) {
		return false
	}
	u.Flags &^= game.FlagHovering
	g.RefreshVisibility()
	return true
}
