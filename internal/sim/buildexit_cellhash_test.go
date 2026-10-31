// buildexit_cellhash_test.go : tests cell hash consistency after builder exits a build site.
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

// TestBuildExitClearsStaleCellHash verifies that a constructor leaving a completed 2x2 site is removed from all four footprint cells.
// The hash must remove the posed 2x2 footprint before the builder returns to its 1x1 mobile footprint.
func TestBuildExitClearsStaleCellHash(t *testing.T) {
	g := newGame(t, 24, 24)
	g.Resources = economy.NewResourceMap(24, 24)

	c := constructorAt(g, 0, 7, 8) // adjacent to the site's west edge
	siteX, siteY := 8, 8           // 2x2 footprint (8,8)..(9,9)
	g.StartBuild(c, miningProduct(), siteX, siteY, 1)
	g.BeginTeamTurn(0) // one refilled turn finishes the one turn build

	if !g.IsBuildReady(c) {
		t.Fatal("constructor is not build-ready after finishing the structure")
	}
	markers := g.ExitMarkers(c)
	if len(markers) == 0 {
		t.Fatal("no exit markers offered around the finished footprint")
	}
	m := markers[0]
	if !g.ExitBuildSite(c, m[0], m[1]) {
		t.Fatalf("ExitBuildSite rejected a valid marker (%d,%d)", m[0], m[1])
	}
	g.ResolveMovement() // animate the drive off to completion

	// The builder has driven off: it must be hashed ONLY at its new cell, never in any
	// of the four old build site footprint cells.
	for dy := 0; dy < 2; dy++ {
		for dx := 0; dx < 2; dx++ {
			cx, cy := siteX+dx, siteY+dy
			for _, u := range g.Units.Cells.At(cx, cy) {
				if u == c {
					t.Errorf("constructor still stale-hashed at old footprint cell (%d,%d)", cx, cy)
				}
			}
		}
	}
	// Sanity: it IS hashed at its actual resting cell.
	atNew := false
	for _, u := range g.Units.Cells.At(c.GridX, c.GridY) {
		if u == c {
			atNew = true
		}
	}
	if !atNew {
		t.Errorf("constructor not hashed at its new cell (%d,%d)", c.GridX, c.GridY)
	}
}

// TestBuildExitAfterSentryEndsPose verifies that a parked builder whose order was changed by the popup (Sentry on, then off) still leaves its build pose on exit.
// A kept pose made the screen slide the sprite back onto the finished building while the unit stood beside it.
func TestBuildExitAfterSentryEndsPose(t *testing.T) {
	cases := []struct {
		name    string
		builder func(g *Game) *game.UnitInfo
		product func() *game.UnitInfo
		siteX   int
		siteY   int
	}{
		{"constructor 2x2", func(g *Game) *game.UnitInfo { return constructorAt(g, 0, 7, 8) }, miningProduct, 8, 8},
		{"engineer 1x1", func(g *Game) *game.UnitInfo { return engineerAt(g, 0, 8, 8) }, turretProduct, 8, 8},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			g := newGame(t, 24, 24)
			g.Resources = economy.NewResourceMap(24, 24)

			b := tc.builder(g)
			g.StartBuild(b, tc.product(), tc.siteX, tc.siteY, 1)
			g.BeginTeamTurn(0)
			if !g.IsBuildReady(b) {
				t.Fatal("builder is not build-ready after finishing the structure")
			}

			// The Sentry popup button pressed twice on the parked builder: on guard, then released.
			b.Order, b.State = game.OrderSentry, game.OrderStateExecutingOrder
			b.Order, b.State = game.OrderAwait, game.OrderStateExecutingOrder

			m := g.ExitMarkers(b)[0]
			if !g.ExitBuildSite(b, m[0], m[1]) {
				t.Fatalf("ExitBuildSite rejected a valid marker (%d,%d)", m[0], m[1])
			}
			g.ResolveMovement()

			if b.Building || b.BuildFootprint != 0 {
				t.Errorf("builder kept its build pose after the exit (Building=%v footprint=%d)", b.Building, b.BuildFootprint)
			}
			if b.GridX != m[0] || b.GridY != m[1] {
				t.Errorf("builder at (%d,%d), want the exit marker (%d,%d)", b.GridX, b.GridY, m[0], m[1])
			}
		})
	}
}

// engineerAt deploys a healthy mobile engineer with movement budget and cargo at a cell for the build exit tests.
func engineerAt(g *Game, team, gx, gy int) *game.UnitInfo {
	e := game.NewUnitInfo(assets.Engineer, team, 1, 0)
	e.Flags = game.FlagMobileLandUnit | game.FlagConstructorUnit | game.FlagSelectable
	e.Values = &game.UnitValues{}
	e.Values.Set(game.AttribSpeed, 16)
	e.Values.Set(game.AttribHits, 20)
	e.InitState()
	e.Speed = 16
	e.Storage = 40
	g.Units.Deploy(e, gx, gy)
	return e
}

// turretProduct builds a one turn 1x1 gun turret product, a real building an engineer parks on.
func turretProduct() *game.UnitInfo {
	prod := game.NewUnitInfo(assets.GunTurrt, 0, 0, 0)
	prod.Flags = game.FlagStationary | game.FlagSelectable
	prod.Values = &game.UnitValues{}
	prod.Values.Set(game.AttribTurns, 1) // finishes in a single turn
	prod.Values.Set(game.AttribHits, 30)
	return prod
}

// parkedBuilderCases are the two builder sizes a finished building parks on (as in step 1433).
var parkedBuilderCases = []struct {
	name    string
	builder func(g *Game) *game.UnitInfo
	product func() *game.UnitInfo
}{
	{"constructor 2x2", func(g *Game) *game.UnitInfo { return constructorAt(g, 0, 7, 8) }, miningProduct},
	{"engineer 1x1", func(g *Game) *game.UnitInfo { return engineerAt(g, 0, 8, 8) }, turretProduct},
}

// parkOnFinishedBuilding builds a one turn product at (8,8) and returns the parked builder and its hidden building.
func parkOnFinishedBuilding(t *testing.T, g *Game, builder func(g *Game) *game.UnitInfo, product func() *game.UnitInfo) (*game.UnitInfo, *game.UnitInfo) {
	t.Helper()
	b := builder(g)
	g.StartBuild(b, product(), 8, 8, 1)
	g.BeginTeamTurn(0)
	if !g.IsBuildReady(b) {
		t.Fatal("builder is not build-ready after finishing the structure")
	}
	bld := g.buildReady[b].Building
	if bld == nil || !bld.HiddenUntilExpand {
		t.Fatal("the finished building is not parked hidden behind its builder")
	}
	return b, bld
}

// TestLoadParkedBuilderReleasesBuilding checks klei 23/08/26 UnitsManager_BuildingReady: a builder stored in a
// transport no longer blocks its finished building, which unfolds at once; the builder leaves its build pose.
func TestLoadParkedBuilderReleasesBuilding(t *testing.T) {
	for _, tc := range parkedBuilderCases {
		t.Run(tc.name, func(t *testing.T) {
			g := newGame(t, 24, 24)
			g.Resources = economy.NewResourceMap(24, 24)
			b, bld := parkOnFinishedBuilding(t, g, tc.builder, tc.product)

			air := game.NewUnitInfo(assets.AirTrans, 0, 1, 0)
			air.Flags = game.FlagMobileAirUnit | game.FlagHovering | game.FlagSelectable
			air.Values = &game.UnitValues{}
			air.Values.Set(game.AttribStorage, 3)
			air.InitState()
			g.Units.Deploy(air, b.GridX, b.GridY-1)

			if !g.LoadUnit(air, b) {
				t.Fatal("the air transport refused the parked builder")
			}
			if bld.HiddenUntilExpand {
				t.Error("the finished building stayed hidden after its builder was loaded")
			}
			released := false
			for _, r := range g.ReleasedExpansions() {
				released = released || r == bld
			}
			if !released {
				t.Error("the finished building was not queued to unfold")
			}
			if g.IsBuildReady(b) || b.Building {
				t.Errorf("the loaded builder kept its build state (ready=%v pose=%v)", g.IsBuildReady(b), b.Building)
			}
			for dy := 0; dy < 2; dy++ {
				for dx := 0; dx < 2; dx++ {
					for _, u := range g.Units.Cells.At(8+dx, 8+dy) {
						if u == b {
							t.Errorf("the loaded builder is still hashed at (%d,%d)", 8+dx, 8+dy)
						}
					}
				}
			}
		})
	}
}

// TestDestroyParkedBuilderLosesBuilding checks klei UnitsManager_BuildingReady with a dead parent inside the
// bounds: the never shown building goes with its builder, slab and all, without a casualty tally or rubble.
func TestDestroyParkedBuilderLosesBuilding(t *testing.T) {
	for _, tc := range parkedBuilderCases {
		t.Run(tc.name, func(t *testing.T) {
			g := newGame(t, 24, 24)
			g.Resources = economy.NewResourceMap(24, 24)
			b, bld := parkOnFinishedBuilding(t, g, tc.builder, tc.product)
			lostBefore := g.TeamStats[0].BuildingsLost

			g.DestroyUnit(b)

			if g.Units.ByID.Get(bld.ID) != nil {
				t.Error("the hidden building survived its builder")
			}
			for _, u := range g.Units.Units() {
				if u == bld {
					t.Error("the hidden building is still in the unit list")
				}
			}
			for dy := 0; dy < 2; dy++ {
				for dx := 0; dx < 2; dx++ {
					for _, u := range g.Units.Cells.At(8+dx, 8+dy) {
						if u.Flags.Has(game.FlagGroundCover) || u == bld {
							t.Errorf("%v left on the lost site at (%d,%d)", u.Type, 8+dx, 8+dy)
						}
					}
				}
			}
			if g.TeamStats[0].BuildingsLost != lostBefore {
				t.Errorf("the never shown building counted as a lost building (%d -> %d)", lostBefore, g.TeamStats[0].BuildingsLost)
			}
		})
	}
}
