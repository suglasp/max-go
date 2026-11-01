// production.go : implements the factory production panel on the game screen.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
// C++ ported : production_manager.cpp - GameManager_DrawFactoryStatus
package gamescreen

// production.go presents the sim's production transitions as messages, banners, and voice. The sim owns the production state machine and events.
//
// C++: the production messages GameManager_ManageEconomy raises after the economy /
// build step (units_manager.cpp / production_manager.cpp).

import (
	"fmt"

	"github.com/suglasp/max-go/internal/assets"
	"github.com/suglasp/max-go/internal/game"
	"github.com/suglasp/max-go/internal/sim"
)

// tickProductionEvents drains production transitions. It reports local factory changes, first Eco Sphere activation as enemy intelligence, and the last Eco Sphere stopping as a global event.
// announceBuildStarted tells the human player when a mobile builder begins a structure and how many turns it needs. Computer builds remain silent.
func (s *Screen) announceBuildStarted(builder *game.UnitInfo, product assets.UnitType, turns, cellX, cellY int) {
	// Only the local player's own construction is worth a message.
	if builder == nil || !s.isHumanTeam(builder.Team) {
		return
	}

	// A just started build always has at least one turn to go.
	if turns < 1 {
		turns = 1
	}

	// Singular / plural so a one turn build does not read "1 turns".
	var text string
	if turns == 1 {
		text = fmt.Sprintf("%s under construction: ready in 1 turn.", displayName(product))
	} else {
		text = fmt.Sprintf("%s under construction: ready in %d turns.", displayName(product), turns)
	}

	s.showBanner(text) // routine dark banner, message type 0
	s.logEvent(s.iconForType(product, builder.Team), fmt.Sprintf("%d,%d", cellX, cellY), text)
}

func (s *Screen) tickProductionEvents() {
	for _, e := range s.game.DrainProductionEvents() {
		switch e.Kind {
		case sim.ProdStopped:
			// A factory stopped: the player STOPPED it (queue KEPT) or its queue RAN OUT
			// (powered down automatically, queue empty). Distinguish the message and play
			// the engine wind down cue (the power consumption end sound the factory makes on
			// halt). Local only - an enemy factory across the map stays silent.
			if e.Unit != nil && s.isHumanTeam(e.Team) {
				if len(e.Unit.BuildQueue) == 0 {
					s.showMessage(fmt.Sprintf("%s finished its queue and powered down.", displayName(e.Unit.Type)))
				} else {
					s.showMessage(fmt.Sprintf("%s production halted.", displayName(e.Unit.Type)))
				}
				s.playPowerCue(e.Unit, game.SfxPowerEnd) // wind down cue (stoppable)
			}
		case sim.ProdStarted:
			// A factory started/resumed: player START, or a unit queued powers it on
			// automatically. Play the spin up cue on the stoppable power cue handle (NOT a
			// fire and forget one shot: SfxPowerStart is the long running hum clip, which as a
			// one shot rang on past a reselect / quit to menu). playPowerCue stops it when the
			// selection moves off the factory and on leaving the screen. Local only.
			if e.Unit != nil && s.isHumanTeam(e.Team) {
				s.showMessage(fmt.Sprintf("%s production started.", displayName(e.Unit.Type)))
				s.playPowerCue(e.Unit, game.SfxPowerStart)
			}
		case sim.ColonyStarted:
			// A team's first Eco Sphere began operating: announce it as enemy intel
			// (checkColonyStarted skips your own team and latches via colonyAnnounced).
			s.checkColonyStarted()
		case sim.ColonyStopped:
			// A team's last operating Eco Sphere stopped: a global event for all players.
			s.checkColonyStopped(e.Team)
		}
	}
}
