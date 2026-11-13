// research_gate_test.go : tests research lab power gate behavior blocking research progress.
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
	"github.com/suglasp/max-go/internal/sim"
	"github.com/suglasp/max-go/internal/units"
)

// newResearchScreen builds a Screen with an initialised game state (sim.Game.State
// is a pointer; a zero Game would nil dereference on Teams access).
func newResearchScreen() *Screen {
	p := &game.StartParams{}
	p.Teams[0].Type = game.TeamPlayer
	return &Screen{game: &sim.Game{Units: units.NewManager(), State: game.NewGameState(p)}}
}

// countOrder reports how many of the units carry the given order.
func countOrder(us []*game.UnitInfo, o game.Order) int {
	n := 0
	for _, u := range us {
		if u.Order == o {
			n++
		}
	}
	return n
}

// TestSyncResearchOrdersRunsAllocatedRestIdle: with three powered on centres and
// one lab allocated, exactly one runs (the lowest ID, for a stable choice) and the
// rest idle so the economy skips their power draw.
func TestSyncResearchOrdersRunsAllocatedRestIdle(t *testing.T) {
	s := newResearchScreen()
	a := mkBuilding(assets.Research, 0, 1, -1, -1, game.FlagStationary|game.FlagBuilding|game.FlagSelectable, game.OrderPowerOn, 0)
	b := mkBuilding(assets.Research, 0, 2, -1, -1, game.FlagStationary|game.FlagBuilding|game.FlagSelectable, game.OrderPowerOn, 0)
	c := mkBuilding(assets.Research, 0, 3, -1, -1, game.FlagStationary|game.FlagBuilding|game.FlagSelectable, game.OrderPowerOn, 0)
	s.game.Units.Deploy(a, 1, 1)
	s.game.Units.Deploy(b, 2, 1)
	s.game.Units.Deploy(c, 3, 1)
	s.game.State.Teams[0].Research[game.ResearchAttack].Allocation = 1

	s.syncResearchOrders(0)

	all := []*game.UnitInfo{a, b, c}
	if got := countOrder(all, game.OrderPowerOn); got != 1 {
		t.Fatalf("running centres = %d, want 1", got)
	}
	if a.Order != game.OrderPowerOn {
		t.Errorf("lowest-ID centre should run, got order %v", a.Order)
	}
	if b.Order != game.OrderIdle || c.Order != game.OrderIdle {
		t.Errorf("excess centres should idle: b=%v c=%v", b.Order, c.Order)
	}
	// economy.IsActive (the power draw / hum gate) tracks the run state.
	if !economy.IsActive(a) {
		t.Error("running centre should be economy-active (drawing power)")
	}
	if economy.IsActive(b) {
		t.Error("idle centre should NOT be economy-active (no power draw)")
	}
}

// TestSyncResearchOrdersZeroAllocationIdlesAll: a powered on centre with no topic
// allocated sits idle (by design: idle until allocated).
func TestSyncResearchOrdersZeroAllocationIdlesAll(t *testing.T) {
	s := newResearchScreen()
	a := mkBuilding(assets.Research, 0, 1, -1, -1, game.FlagStationary|game.FlagBuilding|game.FlagSelectable, game.OrderPowerOn, 0)
	b := mkBuilding(assets.Research, 0, 2, -1, -1, game.FlagStationary|game.FlagBuilding|game.FlagSelectable, game.OrderPowerOn, 0)
	s.game.Units.Deploy(a, 1, 1)
	s.game.Units.Deploy(b, 2, 1)

	s.syncResearchOrders(0) // no allocation set

	if a.Order != game.OrderIdle || b.Order != game.OrderIdle {
		t.Errorf("unallocated centres should idle: a=%v b=%v", a.Order, b.Order)
	}
	if economy.IsActive(a) || economy.IsActive(b) {
		t.Error("unallocated centres must not draw power")
	}
}

// TestSyncResearchOrdersForceOffExcluded: a centre the player force powered off is
// not an available lab - it stays off and is not counted, while the available one
// runs to cover the allocation.
func TestSyncResearchOrdersForceOffExcluded(t *testing.T) {
	s := newResearchScreen()
	off := mkBuilding(assets.Research, 0, 1, -1, -1, game.FlagStationary|game.FlagBuilding|game.FlagSelectable, game.OrderPowerOff, 0) // force off (also the freshly built default)
	on := mkBuilding(assets.Research, 0, 2, -1, -1, game.FlagStationary|game.FlagBuilding|game.FlagSelectable, game.OrderPowerOn, 0)
	s.game.Units.Deploy(off, 1, 1)
	s.game.Units.Deploy(on, 2, 1)
	s.game.State.Teams[0].Research[game.ResearchHits].Allocation = 1

	if got := s.activeResearchCenters(0); got != 1 {
		t.Fatalf("activeResearchCenters = %d, want 1 (force-off excluded)", got)
	}

	s.syncResearchOrders(0)

	if off.Order != game.OrderPowerOff {
		t.Errorf("force-off centre should stay off, got %v", off.Order)
	}
	if on.Order != game.OrderPowerOn {
		t.Errorf("available centre should run to cover the lab, got %v", on.Order)
	}
}

// TestSyncResearchOrdersAllocationClampedToCenters: allocating more labs than the
// available centres runs them all (no out of range), none left idle.
func TestSyncResearchOrdersAllocationClampedToCenters(t *testing.T) {
	s := newResearchScreen()
	a := mkBuilding(assets.Research, 0, 1, -1, -1, game.FlagStationary|game.FlagBuilding|game.FlagSelectable, game.OrderPowerOn, 0)
	b := mkBuilding(assets.Research, 0, 2, -1, -1, game.FlagStationary|game.FlagBuilding|game.FlagSelectable, game.OrderPowerOn, 0)
	s.game.Units.Deploy(a, 1, 1)
	s.game.Units.Deploy(b, 2, 1)
	s.game.State.Teams[0].Research[game.ResearchAttack].Allocation = 3
	s.game.State.Teams[0].Research[game.ResearchHits].Allocation = 2 // 5 > 2 centres

	s.syncResearchOrders(0)

	if a.Order != game.OrderPowerOn || b.Order != game.OrderPowerOn {
		t.Errorf("over-allocation should run every centre: a=%v b=%v", a.Order, b.Order)
	}
}

// TestSyncResearchOrdersWritesTheClampBack: losing centres re-caps the STORED
// pool (highest index topics shrink first, like the research menu frees), so a
// team cannot keep researching on ghost centres it no longer has.
func TestSyncResearchOrdersWritesTheClampBack(t *testing.T) {
	s := newResearchScreen()
	a := mkBuilding(assets.Research, 0, 1, -1, -1, game.FlagStationary|game.FlagBuilding|game.FlagSelectable, game.OrderPowerOn, 0)
	b := mkBuilding(assets.Research, 0, 2, -1, -1, game.FlagStationary|game.FlagBuilding|game.FlagSelectable, game.OrderPowerOn, 0)
	s.game.Units.Deploy(a, 1, 1)
	s.game.Units.Deploy(b, 2, 1)
	research := &s.game.State.Teams[0].Research
	research[game.ResearchAttack].SetAllocation(game.ResearchFactors[game.ResearchAttack], 2)
	research[game.ResearchCost].SetAllocation(game.ResearchFactors[game.ResearchCost], 1)

	s.syncResearchOrders(0) // 3 committed, only 2 centres survive

	if got := research[game.ResearchCost].Allocation; got != 0 {
		t.Errorf("Cost allocation = %d, want 0 (the highest index topic shrinks first)", got)
	}
	if got := research[game.ResearchAttack].Allocation; got != 2 {
		t.Errorf("Attack allocation = %d, want the kept 2", got)
	}
	if got := research[game.ResearchCost].TurnsToComplete; got != 0 {
		t.Errorf("a zeroed topic keeps TurnsToComplete %d, want 0 (SetAllocation consistency)", got)
	}
}

// TestUpgradeUnitValuesBumpsMkOnRealChange: the GetBestValues version gate -
// a re-derive that changes nothing keeps the unit's "Mk" mark; a real change
// (research, gold, a depot upgrade) bumps it exactly once.
func TestUpgradeUnitValuesBumpsMkOnRealChange(t *testing.T) {
	s := newResearchScreen()
	u := &game.UnitInfo{ID: 1, Type: assets.Tank, Team: 0, Flags: game.FlagMobileLandUnit | game.FlagSelectable}
	u.Values = s.teamValues(0, u.Type)
	s.game.Units.Deploy(u, 1, 1)
	base := u.Values.Version()

	s.upgradeUnitValues(0, u) // nothing changed
	if got := u.Values.Version(); got != base {
		t.Errorf("no change re-derive moved the version %d -> %d", base, got)
	}

	s.game.State.Teams[0].Research[game.ResearchHits].Level = 1 // hits gain 10%: a real change
	s.upgradeUnitValues(0, u)
	if got := u.Values.Version(); got != base+1 {
		t.Errorf("version after a real change = %d, want %d", got, base+1)
	}

	s.upgradeUnitValues(0, u) // steady state again
	if got := u.Values.Version(); got != base+1 {
		t.Errorf("steady re-derive bumped the version to %d", got)
	}
}
