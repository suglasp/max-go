// endturntimer.go : implements the End Turn grace countdown of simultaneous play.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
//
// C++ ported : game_manager.cpp - GameManager_HandleTurnTimer
package gamescreen

import (
	"fmt"

	"github.com/suglasp/max-go/internal/dbg"
	"github.com/suglasp/max-go/internal/platform"
)

// SetEndTurnTimer sets the End Turn grace period (seconds) from the end_turn setting.
// In SIMULTANEOUS play, once every other team has finished acting, this is how long the
// player still has to wrap up before the round commits itself. Zero disables it, and a
// turn based game ignores it entirely (there, nobody else is moving while you play).
//
// C++ ported : game_manager.cpp - GameManager_HandleTurnTimer, the "endturn" setting
func (s *Screen) SetEndTurnTimer(seconds int) {
	if seconds < 0 {
		seconds = 0
	}

	s.turnFlow.endTurnSeconds = seconds
}

// armEndTurnGrace shortens the running turn to the End Turn grace period the moment the
// opponents are done: every other team has finished its turn AND has nothing still
// moving on the board. The clock is only ever SHORTENED - a grace longer than the time
// already left leaves the turn as it is - and an untimed turn gains the countdown it
// never had, which is exactly what the setting is for.
//
// The reference implementation arms the grace on frame one when computer teams queue no orders, reducing the
// fresh timer to min(endturn, remaining).
//
// C++ ported : game_manager.cpp:882 - GameManager_HandleTurnTimer
// (timer_setting = min(endturn, GameManager_TurnTimerValue) -> UpdateTurnTimer(true, ...))
func (s *Screen) armEndTurnGrace() {
	if !s.simMode() || s.turnFlow.endTurnSeconds <= 0 || s.turnFlow.graceArmed {
		return
	}

	// Only while the human is actually playing the turn.
	if !s.turnFlow.simPlanned || s.gameOver || s.turnFlow.aiTurn || s.siteSelect != nil || s.camera.landingStep > 0 {
		return
	}

	// Hold while a non blocking UI is up: arming resets the clock start, which
	// would break the debug menu's close time credit and commit under the popup.
	if s.orderInput.popupOpen || s.debugMenu.dbgMenuOpen {
		return
	}

	if s.opponentsActing() {
		if turn := s.currentTurn(); s.turnFlow.graceWaitLogged != turn {
			s.turnFlow.graceWaitLogged = turn
			s.logTurnClock("end turn grace waits for the opponents (planning or units still moving)")
		}

		return
	}

	grace := s.graceSeconds()

	// Never EXTEND the turn: a grace longer than what is left changes nothing.
	if left := s.turnRemaining(); s.turnFlow.turnSeconds > 0 && left <= grace {
		s.turnFlow.graceArmed = true // the turn already ends sooner than the grace would

		return
	}

	s.startGraceCountdown(grace, "end turn grace armed")

	s.showMessage(fmt.Sprintf("Opponents have ended their turn. %d seconds left.", grace))
	dbg.Printf("end turn grace: opponents done, human has %ds left (turn %d)", grace, s.game.State.TurnCounter)
}

// graceSeconds is the End Turn countdown length: the End Turn Time, floored at 15 seconds.
//
// The reference implementation floors the countdown at 15 seconds while computer teams still finish their
// work, so end_turn values below 15 cannot immediately end a turn.
func (s *Screen) graceSeconds() int {
	return max(s.turnFlow.endTurnSeconds, 15)
}

// startGraceCountdown puts the End Turn countdown on the clock: the running turn now ends grace seconds from now.
func (s *Screen) startGraceCountdown(grace int, reason string) {
	s.turnFlow.turnSeconds = grace
	s.turnFlow.turnStart = platform.Ticks()
	s.turnFlow.graceArmed = true
	s.turnFlow.graceOnClock = true
	s.logTurnClock(reason)
}

// switchGraceCountdown applies a new End Turn Time to a countdown that is already running: the clock switches to
// the new value at once, shorter or longer (user 2026-11-09: "use new time now"). A countdown that has not started
// yet, or a turn that already ended sooner than the grace, keeps its clock; the next countdown uses the new value.
// The reference reads the setting only when a team ends its turn; this goes past it on the player's request.
func (s *Screen) switchGraceCountdown() {
	if !s.simMode() || !s.turnFlow.graceArmed || !s.turnFlow.graceOnClock || s.turnFlow.endTurnSeconds <= 0 {
		return
	}

	grace := s.graceSeconds()
	s.startGraceCountdown(grace, "End Turn Time changed, the running countdown switches")

	if grace > 20 {
		s.turnFlow.timeWarnedTurn = turnID{} // the 0:20 warning plays again when the new countdown gets there
	}

	s.showMessage(fmt.Sprintf("End Turn Time changed. %d seconds left.", grace))
}

// opponentsActing reports whether any team other than the human still has something in
// flight this round: a turn it has not finished, or a unit still moving on the board.
// The computer teams are flagged finished when they plan at round start, but their
// units are still driving, and the player should not lose the turn while they do.
func (s *Screen) opponentsActing() bool {
	human := s.humanViewTeam()

	for t := s.game.State.FirstActiveTeam(); t != -1; t = s.game.State.NextActiveTeam(t) {
		if t != human && !s.game.State.Teams[t].FinishedTurn {
			return true
		}
	}

	for _, o := range s.game.MoveOrders() {
		if o.Unit != nil && o.Unit.Team != human {
			return true // an opponent's unit is still driving
		}
	}

	return false
}

// resetEndTurnGrace re-arms the grace for a new round: the full turn budget comes back
// and the countdown waits for the opponents to finish again.
func (s *Screen) resetEndTurnGrace() {
	s.turnFlow.graceArmed = false
	s.turnFlow.graceOnClock = false

	if s.turnFlow.fullTurnSeconds > 0 || s.turnFlow.endTurnSeconds > 0 {
		s.turnFlow.turnSeconds = s.turnFlow.fullTurnSeconds
	}
}
