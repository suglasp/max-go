// commando.go : reports infiltration events to the affected team.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
//
// C++ ported : game_manager.cpp - GameManager_NotifyEvent cases 2-5 (the victim side of
// UnitsManager_CaptureUnit / UnitsManager_DisableUnit / AttemptStealthAction)
package gamescreen

import (
	"fmt"

	"github.com/suglasp/max-go/internal/game"
	"github.com/suglasp/max-go/internal/sim"
)

// tickCommandoEvents drains resolved infiltrations. It reports an enemy capture, disable, or exposed infiltrator to the victim's player team. The attacker receives feedback from its action.
func (s *Screen) tickCommandoEvents() {
	player := s.game.State.PlayerTeam

	for _, e := range s.game.DrainCommandoEvents() {
		s.updateRaidedSelection(e, player)

		if e.Victim == nil || e.VictimTeam != player {
			continue
		}

		name := displayName(e.Victim.Type)
		s.latchEventCell(e.Victim.GridX, e.Victim.GridY) // F1 jumps to the targeted unit

		switch e.Kind {
		case sim.CommandoCapture:
			// "<unit> has been captured!" + "Units have been seized by the enemy!"
			s.showInfo(fmt.Sprintf(s.lang.Get("3206", "%s has been captured!"), name))
			s.playVoice(vsUnitSeized)

		case sim.CommandoDisable:
			// "<unit> has been disabled!" + "Unit not functioning!"
			s.showInfo(fmt.Sprintf(s.lang.Get("20c3", "%s has been disabled!"), name))
			s.playVoice(vsUnitNotFunctioning)

		case sim.CommandoExposed:
			// The attempt FAILED and the infiltrator gave itself away: the target's owner is
			// told what the commando tried on which unit, which warns that an enemy commando
			// is standing next to it.
			key, fallback := "eb6c", "Attempt to disable %s!"

			if e.Steal {
				key, fallback = "f900", "Attempt to capture %s!"
			}

			s.showInfo(fmt.Sprintf(s.lang.Get(key, fallback), name))
			s.playVoice(vsInfiltrateDetected)
		}
	}
}

// updateRaidedSelection keeps the selection sane when a capture or disable lands on a selected unit: the popup closes, a unit
// the player lost leaves the selection, and with Auto Select on the player's own turn the next unit is selected.
//
// C++ ported : units_manager.cpp - UnitsManager_UpdateSabotagedUnitSelection
func (s *Screen) updateRaidedSelection(e sim.CommandoEvent, player int) {
	victim := e.Victim
	if victim == nil || (e.Kind != sim.CommandoCapture && e.Kind != sim.CommandoDisable) || !s.isSelectedUnit(victim) {
		return
	}

	s.orderInput.popupOpen = false

	ownLoss := e.VictimTeam == player && victim.Team != player
	if ownLoss {
		s.forgetSelectedUnit(victim)
	}

	canAct := s.game.State.ActiveTeam == player && !s.gameOver // a computer turn skips this, the next human turn clears the selection
	disabledOwn := e.Kind == sim.CommandoDisable && e.VictimTeam == player && s.selected == victim

	if s.autoSelect && canAct && ((ownLoss && s.selected == nil) || disabledOwn) {
		s.selectNextWithMoves()
	}
}

// isSelectedUnit reports whether u is the selected unit or a member of the selected group.
func (s *Screen) isSelectedUnit(u *game.UnitInfo) bool {
	if s.selected == u {
		return true
	}

	for _, m := range s.selection {
		if m == u {
			return true
		}
	}

	return false
}
