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

	"github.com/suglasp/max-go/internal/sim"
)

// tickCommandoEvents drains resolved infiltrations. It reports an enemy capture, disable, or exposed infiltrator to the victim's player team. The attacker receives feedback from its action.
func (s *Screen) tickCommandoEvents() {
	player := s.game.State.PlayerTeam

	for _, e := range s.game.DrainCommandoEvents() {
		if e.Victim == nil || e.VictimTeam != player {
			continue
		}

		name := displayName(e.Victim.Type)

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
			// told what was tried on which unit, which is the warning that an enemy commando
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
