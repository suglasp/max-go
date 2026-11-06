// power_test.go : tests that the AI switches its own buildings on.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package ai

import (
	"testing"

	"github.com/suglasp/max-go/internal/access"
	"github.com/suglasp/max-go/internal/assets"
	"github.com/suglasp/max-go/internal/economy"
	"github.com/suglasp/max-go/internal/game"
	"github.com/suglasp/max-go/internal/resfile"
	"github.com/suglasp/max-go/internal/sim"
	"github.com/suglasp/max-go/internal/testutil"
	"github.com/suglasp/max-go/internal/units"
)

// Mines and plants run from the start; gold, science, and Eco Spheres wait until the team has the required fuel reserve.
func TestAIWantsPowered(t *testing.T) {
	for _, tc := range []struct {
		unit  assets.UnitType
		fuel  int
		want  bool
		about string
	}{
		{assets.MiningSt, 0, true, "a mine runs on nothing"},
		{assets.LandPlt, 0, true, "a plant is dark until switched on, and an idle one draws no power"},
		{assets.AirPlt, 0, true, "same for the air plant"},
		{assets.Shipyard, 0, true, "same for the shipyard"},
		{assets.Research, 0, false, "science waits for a fuel reserve"},
		{assets.Research, aiFuelReserve, true, "science runs once the fuel is banked"},
		{assets.CommTwr, aiFuelReserve, true, "so does the gold refinery"},
		{assets.Greenhse, aiFuelReserve, true, "so do the Eco Spheres"},
		{assets.Depot, 100, false, "a depot needs no power to hold units"},
		{assets.ADump, 100, false, "nor does a dump"},
	} {
		if got := aiWantsPowered(tc.unit, tc.fuel); got != tc.want {
			t.Errorf("%s: aiWantsPowered(%s, fuel %d) = %v, want %v", tc.about, tc.unit.Tag(), tc.fuel, got, tc.want)
		}
	}
}

// TestAIBuildsAnArmy checks that the AI powers completed factories and produces units from real ATTRIBS data.
func TestAIBuildsAnArmy(t *testing.T) {
	dir := testutil.DataDir(t)

	lib, err := resfile.OpenDir(dir)
	if err != nil {
		t.Fatal(err)
	}

	raw, err := lib.Read("ATTRIBS")
	if err != nil {
		t.Fatal(err)
	}

	attribs, err := assets.ParseAttribs(raw)
	if err != nil {
		t.Fatal(err)
	}

	const (
		w, h  = 60, 60
		turns = 60
		team  = 1
	)

	mgr := units.NewManager()
	grid := &access.Grid{W: w, H: h, Surf: allLand{}, Units: mgr.Cells}

	p := &game.StartParams{}
	p.Teams[0].Type = game.TeamPlayer
	p.Teams[team].Type = game.TeamComputer
	g := sim.NewWithGrid(game.NewGameState(p), mgr, grid, w, h)

	seq := 0
	makeUnit := func(owner int, ty assets.UnitType, flags game.Flags) *game.UnitInfo {
		seq++
		u := game.NewUnitInfo(ty, owner, seq, 0)
		u.Flags = flags
		u.Values = game.NewUnitValues(attribs[ty])
		u.InitState()

		return u
	}

	brain := New(makeUnit)
	brain.SetEnabled(true)
	g.MakeUnit = makeUnit
	g.AI = brain
	g.MiningDifficulty = 4
	g.Resources = economy.NewResourceMap(w, h)
	economy.AllocateResources(g.Resources, w, h, []game.Point{{X: 12, Y: 12}, {X: 46, Y: 46}}, game.NewRandomizer(7))

	// The team's landed base: the two start buildings running, and the founders with
	// the raw they carry (start-base-founder-cargo).
	for _, b := range []struct {
		t     assets.UnitType
		flags game.Flags
		x, y  int
	}{
		{assets.MiningSt, game.FlagStationary | game.FlagRequiresSlab | game.FlagSelectable | game.FlagBuilding, 46, 46},
		{assets.PowGen, game.FlagStationary | game.FlagRequiresSlab | game.FlagSelectable, 48, 46},
	} {
		u := makeUnit(team, b.t, b.flags)
		u.Order = game.OrderPowerOn
		u.State = game.OrderStateExecutingOrder
		g.PlaceBuilding(u, b.x, b.y)
	}

	const mobile = game.FlagMobileLandUnit | game.FlagSelectable

	for _, m := range []struct {
		t       assets.UnitType
		flags   game.Flags
		storage int
		x, y    int
	}{
		{assets.Constrct, mobile | game.FlagConstructorUnit, 40, 44, 44},
		{assets.Engineer, mobile | game.FlagConstructorUnit, 20, 45, 44},
		{assets.Surveyor, mobile, 0, 46, 44},
		{assets.Scout, mobile, 0, 47, 44},
	} {
		u := makeUnit(team, m.t, m.flags)
		u.Storage = m.storage
		g.Units.Deploy(u, m.x, m.y)
	}

	for turn := 1; turn <= turns; turn++ {
		g.BeginTeamTurn(team)

		// Stand in for the frames a turn spends on screen: Tick is what frees a plant
		// once its output has driven off the footprint (sweepFactoryReady).
		for f := 0; f < 60; f++ {
			g.Tick()
		}

		g.EndTeamTurn(team)
	}

	combat, dark, plants := 0, 0, 0

	for _, u := range g.Units.Units() {
		if u.Team != team {
			continue
		}

		switch {
		case isFactory(u.Type):
			plants++

			if !economy.IsActive(u) {
				dark++
			}
		case u.Flags.Has(game.FlagMobileLandUnit) || u.Flags.Has(game.FlagMobileAirUnit) || u.Flags.Has(game.FlagMobileSeaUnit):
			if u.Values != nil && u.Values.Get(game.AttribAttack) > 0 {
				combat++
			}
		}
	}

	// The bench's yardstick: how big an army the base actually fields. About 7 combat units
	// from 2 plants before the pacing work (a light plant and a training hall were never
	// built at all, every plant was capped at one, and the build rate was pinned to x1);
	// about 10 from 3 after it. The ceiling now is mining INCOME, not the build rate: the
	// base spends its material as fast as it digs it.
	t.Logf("after %d turns: %d factories, %d combat units", turns, plants, combat)

	if plants == 0 {
		t.Fatalf("the team raised no factory in %d turns; the bench cannot say anything about production", turns)
	}

	if dark > 0 {
		t.Errorf("%d of the team's %d factories are still powered off: the AI never pressed Start", dark, plants)
	}

	// It landed with a Scout (unarmed) and nothing else that shoots, so every armed
	// unit standing here came out of a factory.
	if combat == 0 {
		t.Errorf("the team fielded NO combat unit in %d turns from %d factories (it is hoarding, not fighting)", turns, plants)
	}
}
