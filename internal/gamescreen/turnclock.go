// turnclock.go : implements the per turn time limit.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package gamescreen

import (
	"fmt"

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
	s.resumeTurnClock(before, platform.Ticks())
}

// resumeTurnClock shifts turnStart forward by the modal's [before, after] wall
// clock span, so that paused interval is not charged against the turn budget.
func (s *Screen) resumeTurnClock(before, after uint64) {
	if after > before {
		s.turnFlow.turnStart += after - before
	}
}

// shouldWarnTimeLow reports whether the "time low" voice should fire, latched to
// once per turn (keyed on turnStart). Fires when remaining drops to <=20s from a
// budget over 20s. C++: GameManager_IsTurnTimerActive latch.
func (s *Screen) shouldWarnTimeLow(remaining int) bool {
	if s.turnFlow.turnSeconds <= 20 || remaining > 20 || s.turnFlow.timeWarnedTurn == s.turnFlow.turnStart {
		return false
	}

	s.turnFlow.timeWarnedTurn = s.turnFlow.turnStart
	return true
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
	// Announcer warns once at 20s left. Racing the End Turn GRACE period (the
	// opponents are done and the clock was cut to it) is its own, more urgent line:
	// "You are about to run out of time!" instead of "20 seconds left".
	// C++: GameManager_DrawTurnTimer (V_F273, and V_F275 on the paused/waiting clock).
	if s.turnFlow.handover == -1 && !s.gameOver && s.siteSelect == nil && s.camera.landingStep == 0 && !s.debugMenu.dbgMenuOpen && !s.orderInput.popupOpen && !s.turnFlow.aiTurn && s.shouldWarnTimeLow(s.turnRemaining()) {
		if s.turnFlow.graceArmed {
			s.playVoice(vsGraceWarn)
		} else {
			s.playVoice(vsTimeWarn)
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
