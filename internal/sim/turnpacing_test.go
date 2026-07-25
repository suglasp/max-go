// turnpacing_test.go : tests once per turn economy/build advance pacing guards.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package sim

import (
	"testing"

	"github.com/suglasp/max-go/internal/assets"
	"github.com/suglasp/max-go/internal/economy"
	"github.com/suglasp/max-go/internal/game"
)

// TestAdvanceBuildsOwnerGate verifies that AdvanceBuilds ignores team 0's construction during other teams' turns and advances it once during the owning team's turn.
func TestAdvanceBuildsOwnerGate(t *testing.T) {
	g := newGame(t, 24, 24)
	g.Resources = economy.NewResourceMap(24, 24)

	// A team-0 complex with ample pooled material so the build never stalls.
	st := game.NewUnitInfo(assets.PowerStn, 0, 1, 0)
	st.Flags = game.FlagBuilding | game.FlagStationary | game.FlagSelectable
	st.Values = &game.UnitValues{}
	st.Values.Set(game.AttribHits, 50)
	c := g.PlaceBuilding(st, 5, 5)
	if c == nil {
		t.Fatal("no complex formed")
	}
	c.Material = 100000

	// A team-0 engineer standing right next to the complex (so builderAtSite is
	// satisfied at once and the build commits on the owner's first AdvanceBuilds).
	eng := game.NewUnitInfo(assets.Engineer, 0, 2, 0)
	eng.Flags = game.FlagMobileLandUnit | game.FlagConstructorUnit | game.FlagSelectable
	eng.Values = &game.UnitValues{}
	eng.InitState()
	eng.Storage = 100000
	g.Units.Deploy(eng, 7, 5)

	// A long build (100 turns) so it never finishes during the measurement.
	prod := game.NewUnitInfo(assets.Radar, 0, 3, 0)
	prod.Flags = game.FlagStationary | game.FlagSelectable
	prod.Values = &game.UnitValues{}
	prod.Values.Set(game.AttribTurns, 100)
	prod.Values.Set(game.AttribHits, 20)
	o := g.StartBuild(eng, prod, 7, 5, 1)

	// --- Leak probe: call AdvanceBuilds for three OTHER teams. ---
	before := o.Progress
	g.AdvanceBuilds(1)
	g.AdvanceBuilds(2)
	g.AdvanceBuilds(3)
	leaked := o.Progress - before
	t.Logf("after AdvanceBuilds(1..3) for OTHER teams: team-0 order progress %d -> %d (leaked %d)",
		before, o.Progress, leaked)
	if leaked != 0 {
		t.Errorf("OWNER-GATE LEAK: a team-0 build advanced %d turns on other teams' AdvanceBuilds calls", leaked)
	}

	// --- Owner probe: the owning team's call must advance exactly one turn. ---
	before = o.Progress
	g.AdvanceBuilds(0)
	t.Logf("after one AdvanceBuilds(0) for the OWNING team: progress %d -> %d", before, o.Progress)
	if o.Progress != before+1 {
		t.Errorf("owning team's AdvanceBuilds advanced %d turns, want 1", o.Progress-before)
	}

	// A four-turn build must complete after exactly four turns for its owning team, regardless of team count.
	// BeginTeamTurn runs the single economy and build pass; EndTeamTurn does not advance either system.
	for _, teams := range []int{2, 4} {
		g2 := newGame(t, 24, 24)
		g2.Resources = economy.NewResourceMap(24, 24)
		st2 := game.NewUnitInfo(assets.PowerStn, 0, 1, 0)
		st2.Flags = game.FlagBuilding | game.FlagStationary | game.FlagSelectable
		st2.Values = &game.UnitValues{}
		st2.Values.Set(game.AttribHits, 50)
		c2 := g2.PlaceBuilding(st2, 5, 5)
		c2.Material = 100000
		eng2 := game.NewUnitInfo(assets.Engineer, 0, 2, 0)
		eng2.Flags = game.FlagMobileLandUnit | game.FlagConstructorUnit | game.FlagSelectable
		eng2.Values = &game.UnitValues{}
		eng2.InitState()
		eng2.Storage = 100000
		g2.Units.Deploy(eng2, 7, 5)
		prod2 := game.NewUnitInfo(assets.Radar, 0, 3, 0)
		prod2.Flags = game.FlagStationary | game.FlagSelectable
		prod2.Values = &game.UnitValues{}
		prod2.Values.Set(game.AttribTurns, 4)
		prod2.Values.Set(game.AttribHits, 20)
		g2.StartBuild(eng2, prod2, 7, 5, 1)

		ownerTurns := 0
		for len(g2.builds) > 0 && ownerTurns < 20 {
			for team := 0; team < teams; team++ {
				g2.BeginTeamTurn(team) // the one canonical economy + build pass
				if team == 0 {
					ownerTurns++
				}
				g2.EndTeamTurn(team) // no second build pass here (the dedupe)
			}
		}
		t.Logf("N=%d teams: 4-turn build completed after %d of the owner's turns", teams, ownerTurns)
		if ownerTurns != 4 {
			t.Errorf("N=%d teams: build completed after %d owner-turns, want 4 (rate must be 1 turn/owner-round, team-count independent)", teams, ownerTurns)
		}
	}
}

// TestEconomyRateTeamCountIndependent is the companion check:
// it is not only builds - the doubled RunEconomy mis paced MINING income too. A
// team-0 mining station must accrue the SAME pooled material per owner round
// regardless of how many teams play (RunEconomy is team scoped, so other teams'
// turn start passes never touch team 0's complex). This mirrors
// TestHeadlessEconomyAccumulates (16 raw/turn, no fuel deposit) across 2 vs 4 teams.
func TestEconomyRateTeamCountIndependent(t *testing.T) {
	const rounds = 3
	mat := func(teams int) int {
		g := newGame(t, 16, 16)
		g.Resources = economy.NewResourceMap(16, 16)
		g.Resources.SetDeposit(5, 5, economy.CargoRaw, 10)
		g.Resources.SetDeposit(6, 5, economy.CargoRaw, 10)

		st := stationaryBuilding(assets.MiningSt, 0, 5, 5)
		st.GridX, st.GridY = 5, 5
		g.Units.Deploy(st, 5, 5)
		g.Resources.SetInitialMining(st)

		gen := game.NewUnitInfo(assets.PowGen, 0, 2, 0)
		gen.Flags = game.FlagStationary
		gen.Order = game.OrderPowerOn
		gen.State = game.OrderStateExecutingOrder
		gen.Values = &game.UnitValues{}
		g.Units.Deploy(gen, 7, 5)

		c := economy.NewComplex(1)
		c.Team = 0
		c.Grow(st)
		c.Grow(gen)
		c.Fuel = 50 // buffer so the generator never browns out over the few rounds
		g.Complexes = append(g.Complexes, c)

		// A Storage Unit: the point here is the mining RATE, and the mine alone holds only
		// 25, which would cap the count before the rounds are up.
		addStore(g, c, assets.ADump, 9, 5)

		for r := 0; r < rounds; r++ {
			for team := 0; team < teams; team++ {
				g.BeginTeamTurn(team) // RunEconomy(team) is team scoped: only team 0 mines into c
				g.EndTeamTurn(team)
			}
		}
		return c.Material
	}

	two, four := mat(2), mat(4)
	t.Logf("team-0 pooled material after %d rounds: 2-team=%d, 4-team=%d", rounds, two, four)
	if two != four {
		t.Errorf("mining income is team-count dependent: 2-team=%d vs 4-team=%d (RunEconomy must run once per OWNER-round)", two, four)
	}
	if want := 16 * rounds; two != want {
		t.Errorf("team-0 material after %d rounds = %d, want %d (16 raw/turn, once per owner-round)", rounds, two, want)
	}
}
