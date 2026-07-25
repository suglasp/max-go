// selection.go : keeps the selected unit and its group in step, so neither goes stale.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package gamescreen

import (
	"fmt"

	"github.com/suglasp/max-go/internal/assets"
	"github.com/suglasp/max-go/internal/dbg"
	"github.com/suglasp/max-go/internal/game"
	"github.com/suglasp/max-go/internal/sim"
)

// containsUnit reports whether u is in us (used to decide whether a click on a
// cell should cycle the selection to the next stacked unit).
func containsUnit(us []*game.UnitInfo, u *game.UnitInfo) bool {
	for _, e := range us {
		if e == u {
			return true
		}
	}

	return false
}

// selectionSet returns the active selection with its leader first. It returns one unit for a single selection and nil when nothing is selected.
func (s *Screen) selectionSet() []*game.UnitInfo {
	if len(s.selection) > 0 {
		return s.selection
	}

	if s.selected != nil {
		return []*game.UnitInfo{s.selected}
	}

	return nil
}

// setSelected makes u the sole selection and clears any group. C++: GameManager_ClickUnit calls ClearUnitList.
func (s *Screen) setSelected(u *game.UnitInfo) {
	s.selected = u
	s.selection = nil
}

// clearSelected drops the whole selection (single or group).
func (s *Screen) clearSelected() {
	s.selected = nil
	s.selection = nil
}

// forgetSelectedUnit removes a destroyed unit from the selection. The next group member becomes leader, or selection clears.
func (s *Screen) forgetSelectedUnit(v *game.UnitInfo) {
	if v == nil || (s.selected != v && len(s.selection) == 0) {
		return
	}

	var rest []*game.UnitInfo

	for _, u := range s.selection {
		if u != v {
			rest = append(rest, u)
		}
	}

	if s.selected == v { // promote a new leader when the dead unit led the group
		if len(rest) > 0 {
			s.selected = rest[0]
		} else {
			s.selected = nil
		}
	}

	if len(rest) >= 2 {
		s.selection = rest
	} else {
		s.selection = nil // a group of one is not a group
	}
}

// setSelectionSet makes units the active deduplicated group with its leader first. One unit becomes a single selection and an empty set clears it.
func (s *Screen) setSelectionSet(units []*game.UnitInfo, leader *game.UnitInfo) {
	seen := make(map[*game.UnitInfo]bool)
	set := make([]*game.UnitInfo, 0, len(units))

	if leader != nil {
		set = append(set, leader)
		seen[leader] = true
	}

	for _, u := range units {
		if u == nil || seen[u] {
			continue
		}

		seen[u] = true
		set = append(set, u)

		if len(set) >= maxGroupSize {
			break
		}
	}

	switch len(set) {
	case 0:
		s.clearSelected()
	case 1:
		s.selectUnit(set[0]) // a group of one is not a group
	default:
		s.selected = set[0]
		s.selection = set
		s.unitStatusVoice(set[0]) // announce the leader only
	}
}

// selectUnit makes u the selected unit, announces it, and reports disabled turns. A plain selection clears any active group.
func (s *Screen) selectUnit(u *game.UnitInfo) {
	s.selected = u
	s.selection = nil
	s.unitStatusVoice(u) // "Ready!" (or the unit's status/order line)

	if n := s.game.DisabledTurns(u); n > 0 {
		s.showMessage(fmt.Sprintf("Disabled (%d turns).", n)) // C++: GameManager_GetUnitStatusMessage
	} else {
		s.buildStatusMessage(u) // a working builder reports what it is raising + turns left
	}

	dbg.Printf("select %s (%d,%d) team %d", displayName(u.Type), u.GridX, u.GridY, u.Team)
}

// buildStatusMessage reports the builder/factory status line on select: a
// producing factory or mid construction builder names its product + turns left; a
// STOPped factory reports what it was making; a bulldozer reports clear turns.
// A parked build ready or idle/non builder says nothing.
// C++ ported : game_manager.cpp - GameManager_DrawBuilderUnitStatusMessage.
func (s *Screen) buildStatusMessage(u *game.UnitInfo) {
	if u == nil || u.Team != s.game.State.PlayerTeam {
		return
	}

	// A bulldozer clearing rubble: "Number of turns to clear site: N." C++: _(af4a).
	if sim.IsBulldozer(u) {
		if n, ok := s.game.ClearJobTurnsLeft(u); ok {
			s.showMessage(fmt.Sprintf("Number of turns to clear site: %d.", n))
		}

		return
	}

	// A running research centre reports the team's soonest advance (allocation is
	// pooled, so this is the team's next level, not strictly this centre's topic).
	if u.Type == assets.Research && u.Order == game.OrderPowerOn {
		if topic, turns, ok := s.soonestResearch(u.Team); ok {
			name := s.lang.Get(researchTopicLabels[topic].key, researchTopicLabels[topic].fallback)
			s.showMessage(fmt.Sprintf("Researching %s: %d turns to next level.", name, turns))
		}

		return
	}

	o := s.game.BuildOrderFor(u)

	if o == nil || o.Product == nil || o.Done() {
		return
	}

	name := displayName(o.Product.Type)

	switch {
	case u.Building || s.game.FactoryProducing(u): // builder in build pose or a factory mid production
		s.showMessage(fmt.Sprintf("Currently building: %s. Turns to completion: %d.", name, o.TurnsLeft()))

	case s.game.ProductionState(u) == sim.ProdPaused: // STOPped factory: report the paused form
		s.showMessage(fmt.Sprintf("Was building %s, with %d turns to completion.", name, o.TotalTurns-o.Progress))
	}
}

// resolveOccupantSelect resolves a click on an occupied cell. Build-ready builders take priority, stable ordering matches the cursor, repeated clicks cycle occupants, and passable structures receive move orders.
// C++ ported : access.cpp - Access_GetSelectableUnit.
func (s *Screen) resolveOccupantSelect(cellX, cellY int) bool {
	if here := s.game.Units.Cells.At(cellX, cellY); len(here) > 0 {
		// A parked build ready builder shares its finished building's cell; prefer it
		// so exit markers show. Own team only (an enemy's fogged builder never wins
		// over the visible building). C++ ported : access.cpp - Access_GetSelectableUnit.
		for _, e := range here {
			if e.Team == s.game.State.ActiveTeam && s.game.IsBuildReady(e) {
				s.selectUnit(e)

				return true
			}
		}

		// The cell's selectable occupants, in the one stable order the cursor also uses,
		// so the hover readout and this click always name the same unit.
		occ := s.selectableOccupants(cellX, cellY)

		if len(occ) > 0 {
			// Click cycle: a cell the selected unit already sits on advances to the
			// NEXT stacked occupant. C++: Access_GetSelectableUnit reorders the cell hash.
			if s.selected != nil && containsUnit(occ, s.selected) {
				s.selectUnit(cycleNext(occ, s.selected, 1))

				return true
			}

			mobilePick, structPick, floorPick := pickOccupant(occ)

			if structPick == nil {
				structPick = floorPick // only a floor structure here: it is the pick
			}

			if mobilePick != nil {
				s.selectUnit(mobilePick)

				return true
			}

			// Only a structure occupies the cell: drive onto it if passable (GO cursor),
			// else select it. Air units never "drive onto" a structure, so they select it.
			// C++: GameManager_GetCursor.
			if structPick != nil {
				// EXCEPTION: a flying air unit clicking a friendly intact LANDING PAD moves
				// onto it, the popup then offers Land. C++: ORDER_MOVE then ORDER_LAND.
				if sel := s.selected; sel != nil && sel.Flags.Has(game.FlagMobileAirUnit) && canTakeMoveOrder(sel) &&
					sel.Team == s.game.State.ActiveTeam && structPick.Type == assets.LandPad &&
					structPick.Team == sel.Team && structPick.Hits > 0 {
					s.orderMoveTo(sel, cellX, cellY)

					return true
				}

				if sel := s.selected; canTakeMoveOrder(sel) && !sel.Flags.Has(game.FlagMobileAirUnit) && sel.Team == s.game.State.ActiveTeam && s.game.CanEnter(sel, cellX, cellY) {
					s.orderMoveTo(sel, cellX, cellY)

					return true
				}

				s.selectUnit(structPick)

				return true
			}
		}
	}

	return false
}
