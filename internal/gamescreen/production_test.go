// production_test.go : tests production.go.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package gamescreen

import (
	"fmt"
	"testing"

	"github.com/suglasp/max-go/internal/assets"
	"github.com/suglasp/max-go/internal/game"
	"github.com/suglasp/max-go/internal/sim"
	"github.com/suglasp/max-go/internal/units"
)

// TestFactoryStopStartNotifies verifies the powered production gamescreen half: the popup
// STOP pauses a factory (KEEPING its queue) and START resumes it, and the event drain
// turns each transition into the owner's local message. A factory owned by another team
// is NOT announced to the human.
func TestFactoryStopStartNotifies(t *testing.T) {
	s, g := newTestScreen(t, 16)

	// A player factory with a one unit queue, powered and running.
	f := mkUnit(g, assets.LandPlt, 0, 1, 4, 4, game.FlagStationary|game.FlagBuilding|game.FlagSelectable)
	f.Order = game.OrderPowerOn
	f.State = game.OrderStateExecutingOrder
	f.BuildQueue = []assets.UnitType{assets.Tank}
	f.BuildRate = 1
	if got := g.ProductionState(f); got != sim.ProdRunning {
		t.Fatalf("running factory state = %v, want ProdRunning", got)
	}

	// STOP: pauses (rate 0, queue KEPT) and the drain raises the local "halted" message.
	g.PauseFactory(f)
	if f.BuildRate != 0 || len(f.BuildQueue) != 1 {
		t.Fatalf("after STOP: rate=%d queue=%v, want 0 / [Tank] kept", f.BuildRate, f.BuildQueue)
	}
	if got := g.ProductionState(f); got != sim.ProdPaused {
		t.Fatalf("stopped factory state = %v, want ProdPaused", got)
	}
	s.tickProductionEvents()
	if want := fmt.Sprintf("%s production halted.", displayName(assets.LandPlt)); s.banner.msg != want {
		t.Fatalf("STOP message = %q, want %q", s.banner.msg, want)
	}

	// START: resumes at the remembered rate and the drain raises the "started" message.
	g.StartFactory(f)
	if f.BuildRate != 1 {
		t.Fatalf("after START: rate=%d, want 1 (remembered)", f.BuildRate)
	}
	s.tickProductionEvents()
	if want := fmt.Sprintf("%s production started.", displayName(assets.LandPlt)); s.banner.msg != want {
		t.Fatalf("START message = %q, want %q", s.banner.msg, want)
	}

	// Another team's factory stop is NOT shown to the human player.
	ef := game.NewUnitInfo(assets.AirPlt, 1, 2, 0)
	ef.Flags = game.FlagStationary | game.FlagBuilding | game.FlagSelectable
	ef.Values = &game.UnitValues{}
	ef.Order = game.OrderPowerOn
	ef.BuildQueue = []assets.UnitType{assets.Fighter}
	ef.BuildRate = 1
	g.Units.Deploy(ef, 8, 8)

	s.banner.msg = "untouched"
	g.PauseFactory(ef)
	s.tickProductionEvents()
	if s.banner.msg != "untouched" {
		t.Errorf("an enemy factory stop should not message the human, got %q", s.banner.msg)
	}
}

// TestFactoryStartStopSfx verifies the factory engine cue: STOP plays the wind down
// (SfxPowerEnd, HVP18) and START plays the spin up (SfxPowerStart, HVP17), both on a
// STOPPABLE power cue handle (not a fire and forget one shot). The cue is stopped when the
// selection moves off the factory and when the screen is left, so the long hum clip never
// rings on past a reselect or a quit to menu.
func TestFactoryStartStopSfx(t *testing.T) {
	s, g := newTestScreen(t, 16)
	var started []string
	var stopped []int
	next := 0
	s.sound.sfxHeldStartFn = func(primary, fallback string, pan, vol float64, loop bool) int {
		started = append(started, primary)
		next++
		return next
	}
	s.sound.sfxHeldStopFn = func(id int) { stopped = append(stopped, id) }

	f := mkUnit(g, assets.LandPlt, 0, 1, 4, 4, game.FlagStationary|game.FlagBuilding|game.FlagSelectable)
	f.Order = game.OrderPowerOn
	f.State = game.OrderStateExecutingOrder
	f.BuildQueue = []assets.UnitType{assets.Tank}
	f.BuildRate = 1
	s.selected = f

	// STOP -> wind down cue (HVP18) on a stoppable handle.
	g.PauseFactory(f)
	s.tickProductionEvents()
	wantStop := units.SfxTagFor(assets.LandPlt, game.SfxPowerEnd)
	if len(started) != 1 || started[0] != wantStop {
		t.Fatalf("stopping a factory played %v, want the wind-down %q", started, wantStop)
	}
	if s.sound.powerCueSfx == 0 || s.sound.powerCueUnit != f {
		t.Fatalf("the stop cue should be tracked on a handle for the factory, got id=%d unit=%v", s.sound.powerCueSfx, s.sound.powerCueUnit)
	}

	// START -> the previous cue is stopped, then the spin up (HVP17) plays on a new handle.
	g.StartFactory(f)
	s.tickProductionEvents()
	wantStart := units.SfxTagFor(assets.LandPlt, game.SfxPowerStart)
	if len(started) != 2 || started[1] != wantStart {
		t.Fatalf("starting a factory played %v, want the spin-up %q second", started, wantStart)
	}
	if len(stopped) != 1 {
		t.Errorf("starting should stop the still-ringing stop cue, stopped=%v", stopped)
	}

	// Selecting another unit stops the lingering cue (no ring on to a new building).
	s.selected = nil
	s.updateAmbientSfx(map[*game.UnitInfo]bool{})
	if s.sound.powerCueSfx != 0 {
		t.Errorf("selecting away from the factory should stop its power cue, id=%d", s.sound.powerCueSfx)
	}

	// And leaving the screen stops any cue (quit to menu).
	g.StartFactory(f) // does nothing (already running); force a fresh cue instead:
	s.playPowerCue(f, game.SfxPowerStart)
	if s.sound.powerCueSfx == 0 {
		t.Fatal("expected an active power cue before stopUnitSfx")
	}
	s.stopUnitSfx()
	if s.sound.powerCueSfx != 0 {
		t.Errorf("stopUnitSfx (quit to menu) should stop the power cue, id=%d", s.sound.powerCueSfx)
	}
}

// TestColonyStoppedAnnounceGlobal verifies the global colony-stopped event: it posts a banner
// for any team (including via the drain) and resets the colony latch so a restart
// announces the start again.
func TestColonyStoppedAnnounceGlobal(t *testing.T) {
	s, _ := newTestScreen(t, 16)
	s.announcer.colonyAnnounced[1] = true // pretend team 1's colony start was announced

	s.checkColonyStopped(1)
	if s.announcer.colonyAnnounced[1] {
		t.Error("checkColonyStopped should reset the colony latch (so a restart re-announces)")
	}
	if want := fmt.Sprintf("%s team colony stopped!", s.teamName(1)); s.banner.msg != want {
		t.Fatalf("colony-stopped banner = %q, want %q", s.banner.msg, want)
	}
}
