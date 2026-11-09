// powerbounce_test.go : tests powerbounce.go.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package gamescreen

import (
	"testing"

	"github.com/suglasp/max-go/internal/assets"
	"github.com/suglasp/max-go/internal/economy"
	"github.com/suglasp/max-go/internal/game"
)

// TestPowerBounce verifies Bug 2.b: powering on a building that cannot
// be powered (a structure off base with no connected generator) must play a BRIEF
// rev up that is CUT by the wind down, then show the vanilla "needs power" message,
// not two overlapping single plays plus the wrong message. The brief then cut and
// deferred message run on the frame loop (tickPowerBounce).
func TestPowerBounce(t *testing.T) {
	s, g := newTestScreen(t, 16)

	var startCalled, stopCalled bool
	s.sound.sfxHeldStartFn = func(primary, fallback string, pan, vol float64, loop bool) int {
		startCalled = true
		return 7 // a nonzero handle
	}
	s.sound.sfxHeldStopFn = func(id int) { stopCalled = true }

	// A detached shipyard: its own complex, no generator -> cannot be powered.
	yard := mkBuilding(assets.Shipyard, 0, 1, 5, 5, game.FlagStationary|game.FlagBuilding, game.OrderPowerOff, 0)
	yard.Flags |= game.FlagSelectable
	g.PlaceBuilding(yard, 5, 5)

	// Power it on -> it bounces: stays OFF, arms the bounce, NO message yet.
	s.setPower(yard, true)
	if yard.Order != game.OrderPowerOff {
		t.Fatalf("an unpowerable building must stay OrderPowerOff, got %v", yard.Order)
	}
	if s.sound.powerBounceUnit != yard {
		t.Fatalf("setPower must arm the bounce for the unit, got %v", s.sound.powerBounceUnit)
	}
	if s.banner.msg != "" {
		t.Errorf("the bounce message must be deferred (shown by tickPowerBounce), got %q", s.banner.msg)
	}

	// Before the due tick: nothing fires (the due time is in the future).
	s.tickPowerBounce()
	if s.sound.powerBounceUnit == nil {
		t.Error("tickPowerBounce must not fire before the rev up window elapses")
	}

	// At or after the due tick: cut the rev up, show the message, clear the bounce.
	s.sound.powerBounceDue = 0 // due now
	s.tickPowerBounce()
	if s.sound.powerBounceUnit != nil {
		t.Error("tickPowerBounce must clear the bounce after firing")
	}
	if startCalled && !stopCalled {
		t.Error("tickPowerBounce must cut (stop) the held rev up it started")
	}
	if want := "Cannot turn " + displayName(yard.Type) + " on, power needed."; s.banner.msg != want {
		t.Errorf("bounce message = %q, want %q", s.banner.msg, want)
	}
}

// TestPowerBounceMessageNamesWorkers checks the bounce message names the DEFICIENT
// resource: a worker short bounce reads "workers needed", not "power needed".
func TestPowerBounceMessageNamesWorkers(t *testing.T) {
	s, _ := newTestScreen(t, 16)

	gh := mkBuilding(assets.Greenhse, 0, 1, 5, 5, game.FlagStationary|game.FlagBuilding, game.OrderPowerOff, 0)
	s.beginPowerBounce(gh, economy.ShortageWorkers)
	s.sound.powerBounceDue = 0 // due now
	s.tickPowerBounce()
	if want := "Cannot turn " + displayName(gh.Type) + " on, workers needed."; s.banner.msg != want {
		t.Errorf("worker short bounce message = %q, want %q", s.banner.msg, want)
	}
}

// TestPowerOnPowerableBuilding is the control: a building whose complex HAS a
// generator (and fuel) powers on normally - no bounce armed.
func TestPowerOnPowerableBuilding(t *testing.T) {
	s, g := newTestScreen(t, 16)

	// A power station + an edge adjacent factory: one complex with a generator.
	gen := mkBuilding(assets.PowerStn, 0, 1, 8, 8, game.FlagStationary|game.FlagBuilding, game.OrderPowerOn, 0)
	factory := mkBuilding(assets.LandPlt, 0, 1, 10, 8, game.FlagStationary|game.FlagBuilding, game.OrderPowerOff, 0)
	c := g.PlaceBuilding(gen, 8, 8)
	g.PlaceBuilding(factory, 10, 8) // merges into the generator's complex
	c.Fuel = 50                     // fuel so the generator can supply power

	s.setPower(factory, true)
	if factory.Order != game.OrderPowerOn {
		t.Errorf("a powerable factory must turn on, got %v", factory.Order)
	}
	if s.sound.powerBounceUnit != nil {
		t.Error("a powerable building must not arm a bounce")
	}
}

// TestPowerOnBouncesAtCapacity: a complex whose lone generator is already committed
// to a mining station bounces a power on of another consumer, even though the
// complex HAS a generator and fuel (so it is not the no power supply bounce, but the
// new capacity bounce via PowerOnConsumer/CanAbsorbPower).
func TestPowerOnBouncesAtCapacity(t *testing.T) {
	s, g := newTestScreen(t, 16)
	s.sound.sfxHeldStartFn = func(primary, fallback string, pan, vol float64, loop bool) int { return 7 }
	s.sound.sfxHeldStopFn = func(id int) {}

	// One small generator powering a mine, plus a gold refinery to power on. Chain
	// them into one complex (2x2 buildings two cells apart are edge adjacent).
	gen := mkBuilding(assets.PowGen, 0, 1, 6, 8, game.FlagStationary|game.FlagBuilding, game.OrderPowerOn, 0)
	mine := mkBuilding(assets.MiningSt, 0, 1, 8, 8, game.FlagStationary|game.FlagBuilding, game.OrderPowerOn, 0)
	refinery := mkBuilding(assets.CommTwr, 0, 1, 10, 8, game.FlagStationary|game.FlagBuilding, game.OrderPowerOff, 0)
	g.PlaceBuilding(gen, 6, 8)
	g.PlaceBuilding(mine, 8, 8)
	g.PlaceBuilding(refinery, 10, 8)
	for _, cx := range g.Complexes {
		if cx.Team == 0 {
			cx.Fuel = 50 // a power supply exists (so it is the CAPACITY bounce, not no fuel)
		}
	}

	s.setPower(refinery, true)
	if refinery.Order != game.OrderPowerOff {
		t.Fatalf("a capacity-bounced consumer must stay off, got %v", refinery.Order)
	}
	if s.sound.powerBounceUnit != refinery {
		t.Errorf("setPower must arm the bounce for the capacity-short consumer, got %v", s.sound.powerBounceUnit)
	}
}

/* // note: unused code in Go port
// replaced by the shared mkBuilding(t, team, 1, gx, gy, game.FlagStationary|game.FlagBuilding, order, storage) in fixture_test.go
// building makes a stationary building of a type/team at a cell (gamescreen test helper).
func building(t assets.UnitType, team, gx, gy int, order game.Order, storage int) *game.UnitInfo {
	u := game.NewUnitInfo(t, team, 1, 0)
	u.Flags = game.FlagStationary | game.FlagBuilding
	u.GridX, u.GridY = gx, gy
	u.Order = order
	u.State = game.OrderStateExecutingOrder
	u.Values = &game.UnitValues{}
	u.Values.Set(game.AttribStorage, storage)
	return u
}
*/
