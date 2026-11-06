// t5_turnclock_test.go : testing unit for turnclock.go tickTurnClock
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package gamescreen

import (
	"runtime"
	"slices"
	"testing"

	"github.com/suglasp/max-go/internal/assets"
	"github.com/suglasp/max-go/internal/game"
	"github.com/suglasp/max-go/internal/platform"
	"github.com/suglasp/max-go/internal/sim"
)

// clockFixture is a turn based screen with a recorded voice log, a driven announcer clock and a
// tank still sliding, so a turn that ends shows up as the End Turn settle state and never as a commit.
type clockFixture struct {
	s      *Screen
	g      *sim.Game
	voices []string
	clock  uint64
}

// waitForTicks spins until the platform tick counter reaches minimum, so a turn start can be placed in the past.
func waitForTicks(minimum uint64) {
	for platform.Ticks() < minimum {
		runtime.Gosched()
	}
}

// newClockFixture builds the fixture with a 180 second configured turn on round 1.
func newClockFixture(t *testing.T) *clockFixture {
	t.Helper()

	s, g := newClickScreen(t)
	g.State.TurnCounter = 1 // round 0 is the "no turn" value, which never warns
	s.turnFlow.handover = -1
	s.SetTurnTimer(180)

	f := &clockFixture{s: s, g: g, clock: 10000}
	s.announcer.voiceFn = func(tag string) { f.voices = append(f.voices, tag) }
	s.announcer.clock = func() uint64 { return f.clock }

	tank := mkUnit(g, assets.Tank, 1, 2, 2, game.FlagMobileLandUnit|game.FlagSelectable)
	tank.Values.Set(game.AttribSpeed, 4)
	tank.Speed = 4

	if !g.OrderMove(tank, 12, 2) {
		t.Fatal("OrderMove should start on open land")
	}

	g.Tick()

	if g.ActiveOrders() == 0 {
		t.Fatal("the tank should be an active mover")
	}

	return f
}

// runTurn sets the running budget so that about secondsLeft remain at the turn start.
func (f *clockFixture) runTurn(secondsLeft int) {
	f.s.turnFlow.turnSeconds = secondsLeft
	f.s.turnFlow.turnStart = platform.Ticks()
}

// expire puts the running turn past its budget (needs a one second budget and enough ticks).
func (f *clockFixture) expire() {
	waitForTicks(1600)
	f.s.turnFlow.turnSeconds = 1
	f.s.turnFlow.turnStart = platform.Ticks() - 1500
}

// spoke counts the lines played from a tag list.
func (f *clockFixture) spoke(tags []string) int {
	n := 0

	for _, v := range f.voices {
		if slices.Contains(tags, v) {
			n++
		}
	}

	return n
}

// TestTickTurnClockWarnsOncePerTurn pins that the time low line plays once per turn and again on the next turn.
func TestTickTurnClockWarnsOncePerTurn(t *testing.T) {
	f := newClockFixture(t)
	warn := voiceLines[vsTimeWarn].tags

	f.runTurn(20)
	f.s.tickTurnClock()

	if got := f.spoke(warn); got != 1 {
		t.Fatalf("first tick at 20s played %d warnings, want 1 (voices %v)", got, f.voices)
	}

	// The same turn keeps ticking well past the voice cooldown: no second warning.
	f.clock += 5000
	f.s.tickTurnClock()
	f.clock += 5000
	f.s.tickTurnClock()

	if got := f.spoke(warn); got != 1 {
		t.Errorf("the same turn warned %d times, want it latched at 1", got)
	}

	// A modal pause moves turnStart but not the turn: still no new warning.
	f.s.resumeTurnClock(platform.Ticks(), platform.Ticks()+500)
	f.clock += 5000
	f.s.tickTurnClock()

	if got := f.spoke(warn); got != 1 {
		t.Errorf("a modal pause re armed the warning: %d plays, want 1", got)
	}

	// The next round re arms it.
	f.g.State.TurnCounter = 2
	f.runTurn(20)
	f.clock += 5000
	f.s.tickTurnClock()

	if got := f.spoke(warn); got != 2 {
		t.Errorf("the next turn played %d warnings in total, want 2", got)
	}
}

// TestTickTurnClockWarningVariants pins which line plays when the clock is cut below 20s and when no warning is due.
func TestTickTurnClockWarningVariants(t *testing.T) {
	t.Run("clock cut below 20s plays the urgent line", func(t *testing.T) {
		f := newClockFixture(t)
		f.runTurn(15)
		f.s.tickTurnClock()

		if got := f.spoke(voiceLines[vsGraceWarn].tags); got != 1 {
			t.Errorf("urgent line played %d times, want 1 (voices %v)", got, f.voices)
		}

		if got := f.spoke(voiceLines[vsTimeWarn].tags); got != 0 {
			t.Errorf("the 20s line played %d times for a cut clock, want 0", got)
		}
	})

	t.Run("a team that ended its turn hears nothing", func(t *testing.T) {
		f := newClockFixture(t)
		f.g.State.Teams[0].FinishedTurn = true
		f.runTurn(15)
		f.s.tickTurnClock()

		if len(f.voices) != 0 {
			t.Errorf("a finished team heard %v, want silence", f.voices)
		}
	})

	t.Run("plenty of time is silent", func(t *testing.T) {
		f := newClockFixture(t)
		f.runTurn(100)
		f.s.tickTurnClock()

		if len(f.voices) != 0 {
			t.Errorf("100s left played %v, want silence", f.voices)
		}
	})

	t.Run("a configured timer of 20s or less never warns", func(t *testing.T) {
		f := newClockFixture(t)
		f.s.turnFlow.fullTurnSeconds = 20
		f.runTurn(15)
		f.s.tickTurnClock()

		if len(f.voices) != 0 {
			t.Errorf("a 20s timer played %v, want silence", f.voices)
		}
	})
}

// TestTickTurnClockOutOfTime pins that a spent budget ends the turn and a running one does not.
func TestTickTurnClockOutOfTime(t *testing.T) {
	t.Run("time left keeps the turn open", func(t *testing.T) {
		f := newClockFixture(t)
		f.runTurn(60)
		f.s.tickTurnClock()

		if f.s.turnFlow.endTurnSettling {
			t.Error("the turn ended with 60s left")
		}
	})

	t.Run("counts down while running", func(t *testing.T) {
		f := newClockFixture(t)
		waitForTicks(1600)
		f.s.turnFlow.turnSeconds = 5
		f.s.turnFlow.turnStart = platform.Ticks() - 1500

		if got := f.s.turnRemaining(); got != 4 {
			t.Errorf("remaining after 1.5s of 5 = %d, want 4", got)
		}
	})

	t.Run("a spent budget ends the turn", func(t *testing.T) {
		f := newClockFixture(t)
		f.expire()
		f.s.tickTurnClock()

		if !f.s.turnFlow.endTurnSettling {
			t.Error("the spent budget did not end the turn")
		}
	})

	t.Run("no limit never ends the turn", func(t *testing.T) {
		f := newClockFixture(t)
		f.expire()
		f.s.turnFlow.turnSeconds = 0
		f.s.tickTurnClock()

		if f.s.turnFlow.endTurnSettling {
			t.Error("a game without a turn limit ended its turn")
		}
	})
}

// TestTickTurnClockHoldsUnderModal pins that a modal pause credits its span so the turn does not end on dismiss.
func TestTickTurnClockHoldsUnderModal(t *testing.T) {
	f := newClockFixture(t)
	f.expire()

	// The same state without the pause ends the turn.
	if f.s.turnRemaining() != 0 {
		t.Fatal("test setup: the budget should be spent")
	}

	// A modal that lasted 10 seconds credits that span back to the clock.
	before := platform.Ticks()
	f.s.resumeTurnClock(before, before+10000)
	f.s.tickTurnClock()

	if f.s.turnFlow.endTurnSettling {
		t.Error("the turn ended although a long modal pause was credited")
	}

	if got := f.s.turnRemaining(); got == 0 {
		t.Error("the clock stayed spent after the pause credit")
	}
}

// TestTickTurnClockGates pins every state in which the clock neither warns nor ends the turn.
func TestTickTurnClockGates(t *testing.T) {
	cases := []struct {
		name  string
		apply func(f *clockFixture)
	}{
		{"hot seat pass banner", func(f *clockFixture) { f.s.turnFlow.handover = 2 }},
		{"game over", func(f *clockFixture) { f.s.gameOver = true }},
		{"site selection", func(f *clockFixture) { f.s.siteSelect = &siteSelectState{} }},
		{"landing animation", func(f *clockFixture) { f.s.camera.landingStep = 1 }},
		{"debug menu", func(f *clockFixture) { f.s.debugMenu.dbgMenuOpen = true }},
		{"order popup", func(f *clockFixture) { f.s.orderInput.popupOpen = true }},
		{"computer turn", func(f *clockFixture) { f.s.turnFlow.aiTurn = true }},
	}

	for _, tc := range cases {
		t.Run(tc.name+" holds the end", func(t *testing.T) {
			f := newClockFixture(t)
			f.expire()
			tc.apply(f)
			f.s.turnFlow.endTurnSettling = false
			f.s.tickTurnClock()

			if f.s.turnFlow.endTurnSettling {
				t.Error("the turn ended inside a gated state")
			}
		})

		t.Run(tc.name+" holds the warning", func(t *testing.T) {
			f := newClockFixture(t)
			f.runTurn(20)
			tc.apply(f)
			f.s.tickTurnClock()

			if len(f.voices) != 0 {
				t.Errorf("warned inside a gated state: %v", f.voices)
			}
		})
	}
}
