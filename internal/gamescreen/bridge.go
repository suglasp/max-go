// bridge.go : implements the bridge construction and rendering.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
// C++ ported : unitinfo.cpp - bridge build and tile placement
package gamescreen

// bridge.go - the bridge raise/lower animation. A bridge sits flat so land units
// cross over it; when a sea unit routes onto its cell the bridge raises out of the
// water and the unit passes underneath, then it lowers once no sea unit is there.
// This is the visual half of the mechanic - the access layer already lets a sea
// unit pass a bridge cell (treated as water) and a land unit cross it (treated as
// land). The original drives it through the ORDER_STATE_ELEVATE / ORDER_STATE_LOWER order
// states plus the GroundCover<->Stationary draw list swap that puts a raised bridge
// above the unit under it (units_manager.cpp ProcessOrderMove / unitinfogroup.cpp).

import (
	"github.com/suglasp/max-go/internal/assets"
	"github.com/suglasp/max-go/internal/game"
	"github.com/suglasp/max-go/internal/platform"
)

// bridgeStepMS is how long one raise/lower frame holds; the 5-frame bridge takes
// ~0.5s to fully raise or lower.
const bridgeStepMS = 110

// updateBridges steps every bridge's raise level one frame toward its target:
// fully raised (BridgeRaiseMax) while a sea unit is on or routing onto its cell,
// flat (0) otherwise. Throttled to bridgeStepMS so the animation reads naturally.
// Called each idle frame (player and computer turns alike, so an AI boat raises it
// too). C++: UnitsManager_ProcessOrderMove ORDER_STATE_ELEVATE / ORDER_STATE_LOWER.
func (s *Screen) updateBridges() {
	now := platform.Ticks()

	if now-s.bridgeStamp < bridgeStepMS {
		return // hold the current frame until the next step is due
	}

	s.bridgeStamp = now
	s.stepBridges()
}

// stepBridges advances every bridge one raise/lower frame toward its target. Split
// from the throttle in updateBridges so it can be driven directly in tests.
func (s *Screen) stepBridges() {
	for _, u := range s.game.Units.Units() {
		if u.Type != assets.Bridge {
			continue
		}

		// Raise toward fully up while a sea unit needs to pass, else lower toward flat.
		target := 0

		if s.bridgeShouldRaise(u.GridX, u.GridY) {
			target = game.BridgeRaiseMax
		}

		switch {
		case u.BridgeRaise < target:
			u.BridgeRaise++
		case u.BridgeRaise > target:
			u.BridgeRaise--
		}
	}
}

// bridgeShouldRaise reports whether a bridge at (cellX, cellY) should be raised: a
// unit that travels UNDER it is on the cell now, or one is routing onto it (its
// remaining path crosses the cell), so the bridge is already up by the time the unit
// arrives. C++: Access_SetUnitDestination elevates the bridge when such a unit heads
// onto its cell.
func (s *Screen) bridgeShouldRaise(cellX, cellY int) bool {
	for _, u := range s.game.Units.Cells.At(cellX, cellY) {
		if passesUnderBridge(u) {
			return true
		}
	}

	for _, o := range s.game.MoveOrders() {
		if o.Unit == nil || !passesUnderBridge(o.Unit) {
			continue // only a unit that goes under raises the bridge
		}

		for i := o.Index; i < len(o.Steps); i++ {
			if o.Steps[i].To.X == cellX && o.Steps[i].To.Y == cellY {
				return true
			}
		}
	}

	return false
}

// passesUnderBridge reports whether u travels UNDER a raised bridge rather than
// crossing OVER it. Per the original the elevate trigger is
// Access_SetUnitDestination(mode = !(unit->flags & MOBILE_SEA_UNIT)) - only a unit
// that carries MOBILE_SEA_UNIT goes under: a pure sea unit, or a genuine dual flag
// amphibian (the Scout / Personnel Carrier). The Constructor and Engineer are NOT
// MOBILE_SEA_UNIT (land builders like the Surveyor), so they cross OVER, like a pure land
// unit. Verified in vanilla: constructor + engineer over, personnel carrier under.
func passesUnderBridge(u *game.UnitInfo) bool {
	return u.Flags.Has(game.FlagMobileSeaUnit)
}

// groundCoverLayer reports whether u draws in the terrain foundation layer, UNDER
// every unit: real ground cover (slabs / roads / rubble) and a FLAT (lowered) bridge
// - you cross over a lowered bridge like a road, even while approaching it from a
// nearer row. A RAISED bridge is excluded; it draws OVER the unit passing under it. A
// built bridge is FlagStationary (not FlagGroundCover), so the bridge case is keyed on
// type, not the flag. C++: a lowered bridge sits in the GroundCover draw list, a raised
// one moves to the Stationary list (unitinfogroup.cpp).
func groundCoverLayer(u *game.UnitInfo) bool {
	if u.Type == assets.Bridge {
		return u.BridgeRaise == 0 // flat bridge is terrain; a raised one draws over
	}

	if u.Type == assets.WtrPltfm {
		// A water platform is the FLOOR over water a base/building stands on, so it draws
		// UNDER every unit - not in the unit layer, where a platform in a nearer row would
		// clip the lower part of a constructor (or any unit) standing a row behind it (the
		// reported "constructor 1/4 merged into a platform" bug). It is FlagStationary, not
		// FlagGroundCover, so this is keyed on type like the flat bridge case above.
		// C++: UnitInfo::GetDrawLayer puts WTRPLTFM at layer 2, below units (layer 6).
		return true
	}

	return u.Flags.Has(game.FlagGroundCover)
}

// isElevatedBridge reports whether u is a bridge currently raised off the water (so
// it draws ABOVE the unit passing under it rather than as flat ground cover).
func isElevatedBridge(u *game.UnitInfo) bool {
	return u.Type == assets.Bridge && u.BridgeRaise > 0
}

// sameCellRank is the stacking order of two units sharing a cell (lower is drawn
// first, UNDER). A connector ("collector") is an elevated structure: a ground / sea
// / bipedal unit drives UNDER it (so the connector draws over the unit on its
// cell), while an aircraft flies OVER it. A stationary ground object (a mine) sits
// under the unit standing on it.
//
// C++ ported : unitinfogroup.cpp - UnitInfoGroup::RenderLists draws the lists in
// order GroundCover < MobileLandSea < Stationary (connectors) < MobileAir.
func sameCellRank(u *game.UnitInfo) int {
	switch {
	case u.Flags.Has(game.FlagMobileAirUnit):
		return 3 // aircraft fly over the connector
	case u.Type == assets.Cnct4W:
		return 2 // a connector: ground / sea / bipedal units drive under it
	case !u.Flags.Has(game.FlagStationary):
		return 1 // a ground / sea / bipedal unit
	default:
		return 0 // a mine / stationary ground object under the unit on it
	}
}
