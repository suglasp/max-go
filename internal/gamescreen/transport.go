// transport.go : implements the land/sea transport load/unload animation on the game screen.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
// C++ ported : unitinfo.cpp - transport load/unload visual sequence
package gamescreen

import (
	"github.com/suglasp/max-go/internal/assets"
	"github.com/suglasp/max-go/internal/game"
	"github.com/suglasp/max-go/internal/platform"
	"github.com/suglasp/max-go/internal/sim"
)

// loadJob tracks a unit travelling to a building, shrinking beside it, and entering storage.
type loadJob struct {
	building  *game.UnitInfo
	shrinking bool
}

// loadShrinkMax is the most collapsed Unfold step a unit reaches before it is
// stored (mirrors the build/exit grow's 4-step range, played in reverse).
const loadShrinkMax = 4

// transport.go wires transport and storage loading. Enter arms an adjacent carrier pick, while a building Load action picks a unit. The sim performs storage changes through sim.LoadUnit.

// armLoad arms a load pick for passenger u: the next click on an adjacent
// friendly transport boards it. Only mobile land units can board.
func (s *Screen) armLoad(u *game.UnitInfo) {
	if !(u.Flags.Has(game.FlagMobileLandUnit) || u.Flags.Has(game.FlagMobileAirUnit) || u.Flags.Has(game.FlagMobileSeaUnit)) {
		s.showMessage("This unit cannot be stored.")
		return
	}

	// klei refuses to load DISABLED units up front (_(9897), game_manager.cpp:6719).
	if s.game.IsDisabled(u) {
		s.showInfo(s.lang.Get("9897", "Unable to load disabled units."))
		return
	}

	s.cancelTransport()
	s.orderInput.loadPassenger = u
	s.orderInput.loadArmed = true
	s.showMessage("Select an adjacent transport or storage building.")
}

// armUnload arms an Activate placement pick for a stored unit. The caller has already found an adjacent free cell.
// C++ ported : repairshopmenu.cpp - Activate (_(755b)) + the ORDER_ACTIVATE click.
func (s *Screen) armUnload(carrier, unit *game.UnitInfo) {
	s.cancelTransport()
	s.orderInput.unloadFrom = carrier
	s.orderInput.unloadUnit = unit
	s.setSelected(carrier)
	s.showMessage(s.lang.Get("755b", "Select an open square to place unit."))
}

// armLoadInto arms a building Load action. The chosen friendly unit moves beside the Depot, Hangar, or Dock before entering storage.
func (s *Screen) armLoadInto(building *game.UnitInfo) {
	s.cancelTransport()

	// A full bay refuses the Load: report it clearly here (a world tint event banner,
	// the original's type 1 like the other transport notices) rather than letting the player arm
	// a pick that can only fail (the popup item stays visible so it never silently vanishes).
	if len(building.Carried) >= sim.ContainerCapacity(building) {
		s.showInfo("Cannot load - the " + displayName(building.Type) + " is full.")
		return
	}

	s.effects.loadInto = building
	s.showMessage("Select a unit to load into the " + displayName(building.Type) + ".")
}

// cancelTransport disarms any pending load / load into / unload placement pick.
func (s *Screen) cancelTransport() {
	s.orderInput.loadPassenger, s.orderInput.loadArmed = nil, false
	s.orderInput.unloadFrom, s.orderInput.unloadUnit = nil, nil
	s.effects.loadInto = nil
}

// transportArmed reports whether a load, load into or unload pick awaits a click.
func (s *Screen) transportArmed() bool {
	return s.orderInput.loadArmed || s.effects.loadInto != nil || s.orderInput.unloadUnit != nil
}

// beginLoad starts sucking unit into building: it joins the loading set and, if it
// is not already adjacent, is sent driving to a cell beside the building first.
func (s *Screen) beginLoad(unit, building *game.UnitInfo) {
	if s.effects.loading == nil {
		s.effects.loading = make(map[*game.UnitInfo]*loadJob)
	}

	s.effects.loading[unit] = &loadJob{building: building}

	if !s.game.AdjacentToContainer(building, unit) {
		if x, y, ok := s.game.LoadCell(building, unit); ok {
			s.game.OrderMove(unit, x, y)
		}
	}
}

// isMoving reports whether u currently has an active move order.
func (s *Screen) isMoving(u *game.UnitInfo) bool {
	for _, o := range s.game.MoveOrders() {
		if o.Unit == u {
			return true
		}
	}

	return false
}

// tickLoads advances building loads. Units at the building shrink on the expand clock, play the pack-up sound, and enter the bay.
func (s *Screen) tickLoads() {
	if len(s.effects.loading) == 0 {
		return
	}

	advance := false

	if now := platform.Ticks(); now-s.effects.loadStamp >= buildExpandFrameMS {
		s.effects.loadStamp = now
		advance = true
	}

	for u, job := range s.effects.loading {
		if u.Hits <= 0 || job.building.Hits <= 0 {
			delete(s.effects.loading, u) // unit or building lost mid load
			continue
		}

		if !job.shrinking {
			if s.game.AdjacentToContainer(job.building, u) {
				job.shrinking = true // arrived (or already adjacent): start the suck up
				s.playSfxForUnit(u, game.SfxShrink)
			} else if !s.isMoving(u) && !s.game.HasPendingRoute(u) {
				// No active move AND no paused (over budget) route left: it genuinely
				// cannot reach the building. A unit that just ran out of moves keeps its
				// pending route and resumes next turn, so the load is NOT dropped then.
				delete(s.effects.loading, u)
			}

			continue
		}

		if advance {
			if u.Unfold < loadShrinkMax {
				u.Unfold++
			} else {
				u.Unfold = 0
				s.game.LoadUnit(job.building, u) // fully shrunk: into the bay
				delete(s.effects.loading, u)
			}
		}
	}
}

// tryTransportTarget handles a click at (cellX, cellY) while a load/unload pick is
// armed, returning whether the click was consumed.
func (s *Screen) tryTransportTarget(cellX, cellY int) bool {
	switch {
	case s.orderInput.unloadUnit != nil:
		// Activate placement pick: the clicked cell takes the stored unit if it is
		// adjacent, free and passable for its class - UnloadUnit validates all of it.
		carrier, unit := s.orderInput.unloadFrom, s.orderInput.unloadUnit
		s.cancelTransport()

		if s.game.Units.ByID.Get(carrier.ID) != carrier {
			return true // the carrier died while the pick was armed
		}

		if !s.game.UnloadUnit(carrier, unit, cellX, cellY) {
			s.showInfo(s.lang.Get("a1f0", "Unable to activate unit at this site."))
			return true
		}

		s.setSelected(unit) // the placed unit grows in selected (tickBuildExpansions animates it)
		s.showMessage(displayName(unit.Type) + " activated.")
		return true

	case s.orderInput.loadArmed:
		passenger := s.orderInput.loadPassenger
		container := s.pickFriendlyUnit(cellX, cellY)
		s.cancelTransport()

		// A storage BUILDING (Depot/Hangar/Dock) SUCKS the unit up (animated scale down
		// in place); Enter needs the unit already adjacent.
		if container != nil && sim.IsStorageBuilding(container.Type) {
			if !s.game.CanContain(container, passenger) || !s.game.AdjacentToContainer(container, passenger) {
				s.showInfo("Cannot store there - need an adjacent friendly storage building with room.")
				return true
			}

			s.beginLoad(passenger, container)
			s.setSelected(container)
			return true
		}

		// An AIR transport flies over the passenger, descends, picks it up, and rises -
		// not the in place store the land/sea carrier uses.
		if container != nil && container.Type == assets.AirTrans {
			if !s.game.CanContain(container, passenger) {
				s.showInfo("Cannot load - the air transport has no room for that unit.")
				return true
			}

			s.beginAirLoad(container, passenger)
			return true
		}

		// A land/sea transport: the same scale down suck up a storage building plays
		// (klei ScaleUnit SHRINK for every carrier, units_manager.cpp:2927).
		if container == nil || !s.game.CanContain(container, passenger) || !s.game.AdjacentToContainer(container, passenger) {
			s.showInfo("Cannot store there - need an adjacent friendly transport or storage building with room.")
			return true
		}

		s.beginLoad(passenger, container)
		s.setSelected(container)
		return true

	case s.effects.loadInto != nil:
		// Building initiated Load: pick a friendly unit anywhere (class + capacity), it
		// paths to the building if needed and is sucked up on arrival.
		building := s.effects.loadInto
		unit := s.pickFriendlyUnit(cellX, cellY)
		s.cancelTransport()

		if unit != nil && s.game.IsDisabled(unit) {
			s.showInfo(s.lang.Get("9897", "Unable to load disabled units."))
			return true
		}

		if unit == nil {
			s.showInfo("Cannot load that unit here.")
			return true
		}

		// A friendly unit it cannot take is selected instead of loaded, as indicated by the select pointer.
		if !s.game.CanContain(building, unit) {
			s.setSelected(unit)
			return true
		}

		// An AIR transport flies to the unit (descend -> pick up -> rise); a land/sea
		// carrier or storage building sucks the unit up in place after it paths over.
		if building.Type == assets.AirTrans {
			s.beginAirLoad(building, unit)
			return true
		}

		s.beginLoad(unit, building)
		s.setSelected(building)
		s.showMessage("Loading " + displayName(unit.Type) + "...")
		return true
	}

	return false
}

// handleTransportPick resolves an armed transport load or unload pick: a left click
// names the transport to board, or the cell to unload onto. Returns whether it consumed
// the click.
func (s *Screen) handleTransportPick(mouse platform.MouseState) bool {
	if s.transportArmed() && s.turnFlow.handover == -1 && !s.gameOver {
		if mouse.Buttons&platform.MouseReleaseRight != 0 {
			s.cancelTransport()
			s.showMessage("Cancelled.")

			return true
		}

		if mouse.Buttons&platform.MouseReleaseLeft != 0 {
			if cellX, cellY, ok := s.mapCell(mouse.X, mouse.Y); ok && s.tryTransportTarget(cellX, cellY) {
				return true
			}

			s.cancelTransport()

			return true
		}
	}

	return false
}
