// turnclock.go : implements the per turn time limit.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package gamescreen

import (
	"fmt"

	"github.com/suglasp/max-go/internal/dbg"
	"github.com/suglasp/max-go/internal/platform"
)

// turnRemaining is the seconds left in the active team's turn, clamped at zero;
// meaningful only while a turn is interactive. C++: GameManager_TurnTimerValue.
func (s *Screen) turnRemaining() int {
	now := platform.Ticks()

	if s.debugMenu.dbgMenuOpen {
		now = s.debugMenu.dbgMenuOpenedTick // the F2 menu pauses the clock
	}

	return s.remainingAt(now)
}

// remainingAt is turnRemaining against an explicit clock, so the countdown can be
// tested without waiting on real time.
func (s *Screen) remainingAt(now uint64) int {
	if s.turnFlow.turnSeconds <= 0 {
		return 0
	}

	if now < s.turnFlow.turnStart {
		return s.turnFlow.turnSeconds
	}

	elapsed := int((now - s.turnFlow.turnStart) / 1000)

	if r := s.turnFlow.turnSeconds - elapsed; r > 0 {
		return r
	}

	return 0
}

// withTurnClockPaused runs a blocking modal without its real time counting
// against the turn timer: shifts turnStart forward by however long it took, so a
// long pause does not instantly expire the turn on dismiss.
func (s *Screen) withTurnClockPaused(fn func()) {
	before := platform.Ticks()
	s.turnFlow.wdModalDepth.Add(1) // mark a blocking modal open so the stall watchdog ignores the parked loop
	fn()
	s.turnFlow.wdModalDepth.Add(-1)
	// A clock restarted inside the modal (an End Turn Time switch) is credited only from that restart.
	s.resumeTurnClock(max(before, s.turnFlow.turnStart), platform.Ticks())
	/* s.resumeTurnClock(before, platform.Ticks()): note: unused code in Go port, credited a clock restarted inside the modal twice (N25) */
}

// resumeTurnClock shifts turnStart forward by the modal's [before, after] wall
// clock span, so that paused interval is not charged against the turn budget.
func (s *Screen) resumeTurnClock(before, after uint64) {
	if after > before {
		s.turnFlow.turnStart += after - before
		s.logTurnClock(fmt.Sprintf("modal closed, %dms paused time credited", after-before))
	}
}

// logTurnClock writes one debug line each time the turn clock changes: why, which turn, the running budget, the
// time left, the full budget and the End Turn grace. The trail shows which change put a value on the clock.
func (s *Screen) logTurnClock(reason string) {
	turn := s.currentTurn()
	dbg.Printf("turn clock: %s (turn %d, team %d): budget %ds, left %ds, full %ds, end turn %ds, grace armed %v",
		reason, turn.round, turn.team, s.turnFlow.turnSeconds, s.turnRemaining(), s.turnFlow.fullTurnSeconds,
		s.turnFlow.endTurnSeconds, s.turnFlow.graceArmed)
}

// turnID names one interactive turn: the round and the team playing it. Round numbers start at 1, so the zero
// value means no turn.
type turnID struct {
	round int
	team  int
}

// currentTurn returns the turn being played, or the zero turnID before a game is attached.
func (s *Screen) currentTurn() turnID {
	if s.game == nil || s.game.State == nil {
		return turnID{}
	}
	return turnID{round: s.game.State.TurnCounter, team: s.game.State.ActiveTeam}
}

// shouldWarnTimeLow reports whether the "time low" voice should fire: once per turn, the first time the clock
// shows 20s or less on a configured timer over 20s.
// The latch is keyed on the turn, not on turnStart, so a modal pause or the End Turn grace (which both move
// turnStart) never fire it again.
//
// C++ ported : game_manager.cpp - GameManager_DrawTurnTimer (the GameManager_IsTurnTimerActive latch, re
// armed at turn start only)
func (s *Screen) shouldWarnTimeLow(remaining int) bool {
	turn := s.currentTurn()
	if s.turnFlow.fullTurnSeconds <= 20 || remaining > 20 || s.turnFlow.timeWarnedTurn == turn {
		return false
	}

	s.turnFlow.timeWarnedTurn = turn
	return true
}

// timeWarnVoice picks the "time low" line: "20 seconds to end turn" when the clock ticks through 20s,
// "about to run out of time" when it jumped below 20s (the End Turn grace cut it short). A team that already
// ended its turn hears neither. C++ ported : game_manager.cpp - GameManager_DrawTurnTimer (V_F273 / V_F275).
func (s *Screen) timeWarnVoice(remaining int) (voiceState, bool) {
	if remaining == 20 {
		return vsTimeWarn, true
	}
	if s.game != nil && s.game.State != nil {
		if team := s.game.State.ActiveTeam; team >= 0 && team < len(s.game.State.Teams) && s.game.State.Teams[team].FinishedTurn {
			return 0, false
		}
	}
	return vsGraceWarn, true
}

// formatClock renders a whole second duration as mm:ss, matching the
// original turn timer "%2.2i:%2.2i" layout.
func formatClock(sec int) string {
	return fmt.Sprintf("%02d:%02d", sec/60, sec%60)
}

// defaultTurnSeconds is the per turn budget until the game config's turn timer
// value is wired in (carryover).
const defaultTurnSeconds = 180

// tickTurnClock warns once when the turn time runs low, arms the End Turn grace period
// of simultaneous play, and ends the turn automatically when the clock reaches zero.
// C++: GameManager_DrawTurnTimer + GameManager_HandleTurnTimer.
func (s *Screen) tickTurnClock() {
	// Announcer warns once per turn at 20s left; a clock the End Turn grace cut straight below 20s gets the more
	// urgent line.
	// C++: GameManager_DrawTurnTimer (V_F273 at exactly 20s, else V_F275).
	if remaining := s.turnRemaining(); s.turnFlow.handover == -1 && !s.gameOver && s.siteSelect == nil && s.camera.landingStep == 0 && !s.debugMenu.dbgMenuOpen && !s.orderInput.popupOpen && !s.turnFlow.aiTurn && s.shouldWarnTimeLow(remaining) {
		if voice, ok := s.timeWarnVoice(remaining); ok {
			s.playVoice(voice)
		}
	}

	// Simultaneous play: once the opponents are done acting, the player has the End
	// Turn grace period left, and no more. C++: GameManager_HandleTurnTimer.
	s.armEndTurnGrace()

	// A turn whose wall clock budget runs out ends automatically. The order popup
	// holds the auto commit: the round must not end and clear the selection while
	// the player is mid pick (the commit fires once the popup closes).
	if s.turnFlow.turnSeconds > 0 && s.turnFlow.handover == -1 && !s.gameOver && s.siteSelect == nil && s.camera.landingStep == 0 && !s.debugMenu.dbgMenuOpen && !s.orderInput.popupOpen && !s.turnFlow.aiTurn && s.turnRemaining() == 0 {
		s.endTurn()
	}
}
