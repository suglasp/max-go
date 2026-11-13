// sim_test.go : tests the headless turn loop and game-state initialization.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package sim

import (
	"testing"

	"github.com/suglasp/max-go/internal/access"
	"github.com/suglasp/max-go/internal/assets"
	"github.com/suglasp/max-go/internal/build"
	"github.com/suglasp/max-go/internal/economy"
	"github.com/suglasp/max-go/internal/game"
	"github.com/suglasp/max-go/internal/units"
	"github.com/suglasp/max-go/internal/vis"
	"github.com/suglasp/max-go/internal/winloss"
	"github.com/suglasp/max-go/internal/world"
)

// TestBuilderAtSite verifies the gate that holds a mobile constructor's build
// until it reaches the site: a far constructor does not progress, an adjacent
// one does, and a stationary factory (no constructor flag) is always ready.
func TestBuilderAtSite(t *testing.T) {
	product := game.NewUnitInfo(assets.MiningSt, 0, 0, 0)
	product.Flags = game.FlagBuilding // 2x2 footprint at site (5,5)..(6,6)

	order := func(x, y int, constructor bool) *build.BuildOrder {
		b := game.NewUnitInfo(assets.Constrct, 0, 0, 0)
		b.GridX, b.GridY = x, y
		if constructor {
			b.Flags = game.FlagConstructorUnit
		}
		return &build.BuildOrder{Builder: b, Product: product, GridX: 5, GridY: 5}
	}

	if builderAtSite(order(0, 0, true)) {
		t.Error("constructor far from the site should not be at site")
	}
	if !builderAtSite(order(4, 5, true)) {
		t.Error("constructor one cell left of the footprint should be adjacent")
	}
	if !builderAtSite(order(7, 7, true)) {
		t.Error("constructor at the diagonal corner should be adjacent")
	}
	if !builderAtSite(order(0, 0, false)) {
		t.Error("a non-constructor builder (factory) is always ready")
	}
	if !builderAtSite(&build.BuildOrder{Product: product, GridX: 5, GridY: 5}) {
		t.Error("an order with no builder is always ready")
	}
}

// TestBuildDrawsFromAdjacentComplex checks that an AI builder with EMPTY cargo can
// still build next to the base: the order draws from the adjacent complex's pooled
// material (the auto resupply stand in for the cargo truck), and AdvanceBuilds
// reports a stall only when no material is reachable at all. Only a COMPUTER team's
// builder is auto supplied this way - a player builder spends its own cargo.
func TestBuildDrawsFromAdjacentComplex(t *testing.T) {
	g := newGame(t, 24, 24)
	g.State.Teams[0].Type = game.TeamComputer // AI builder: auto resupplied from the base

	// A power station forms a complex; give it pooled material.
	st := game.NewUnitInfo(assets.PowerStn, 0, 1, 0)
	st.Flags = game.FlagBuilding | game.FlagStationary | game.FlagSelectable
	st.Values = &game.UnitValues{}
	st.Values.Set(game.AttribHits, 50)
	c := g.PlaceBuilding(st, 5, 5) // covers (5,5)..(6,6)
	if c == nil {
		t.Fatal("no complex formed")
	}
	c.Material = 50

	// An engineer with NO cargo, adjacent to the station's east edge.
	eng := game.NewUnitInfo(assets.Engineer, 0, 2, 0)
	eng.Flags = game.FlagMobileLandUnit | game.FlagConstructorUnit | game.FlagSelectable
	eng.Values = &game.UnitValues{}
	eng.InitState()
	eng.Storage = 0
	g.Units.Deploy(eng, 7, 5)

	conn := game.NewUnitInfo(assets.Cnct4W, 0, 3, 0)
	conn.Flags = game.FlagStationary | game.FlagSelectable
	conn.Values = &game.UnitValues{}
	conn.Values.Set(game.AttribTurns, 1)
	conn.Values.Set(game.AttribHits, 10)
	g.StartBuild(eng, conn, 7, 5, 1)

	matBefore := c.Material
	stalled := g.AdvanceBuilds(0)
	if len(stalled) != 0 {
		t.Errorf("build stalled despite an adjacent complex with material: %v", stalled)
	}
	if c.Material >= matBefore {
		t.Errorf("complex material not drawn: %d -> %d", matBefore, c.Material)
	}
	if len(g.builds) != 0 {
		t.Errorf("1-turn connector did not finish (progress stuck)")
	}
}

// TestBuildSpendsOwnCargoNotEmptyComplex reproduces the "builds forever" bug: an
// engineer with ample cargo builds a structure next to a freshly raised lone
// storage building whose complex holds NO material. The build must spend the
// engineer's OWN storage (the original UnitInfo::UpdateProduction) and finish - it must NOT
// draw the adjacent 0-material complex and stall indefinitely.
func TestBuildSpendsOwnCargoNotEmptyComplex(t *testing.T) {
	g := newGame(t, 24, 24)

	// A just built lone storage forms its own complex with no pooled material.
	dump := game.NewUnitInfo(assets.ADump, 0, 1, 0)
	dump.Flags = game.FlagStationary | game.FlagSelectable
	dump.Values = &game.UnitValues{}
	dump.Values.Set(game.AttribHits, 30)
	c := g.PlaceBuilding(dump, 5, 5)
	if c == nil {
		t.Fatal("no complex formed for the lone storage")
	}
	c.Material = 0 // a fresh storage building pools nothing

	// An engineer with plenty of its own cargo, standing right next to the storage.
	eng := game.NewUnitInfo(assets.Engineer, 0, 2, 0)
	eng.Flags = game.FlagMobileLandUnit | game.FlagConstructorUnit | game.FlagSelectable
	eng.Values = &game.UnitValues{}
	eng.InitState()
	eng.Storage = 200
	g.Units.Deploy(eng, 6, 5)

	// Build a 1-turn structure on the engineer's own cell (adjacent to the storage).
	radar := game.NewUnitInfo(assets.Radar, 0, 3, 0)
	radar.Flags = game.FlagStationary | game.FlagSelectable
	radar.Values = &game.UnitValues{}
	radar.Values.Set(game.AttribTurns, 1)
	radar.Values.Set(game.AttribHits, 20)
	g.StartBuild(eng, radar, 6, 5, 1)

	storeBefore := eng.Storage
	stalled := g.AdvanceBuilds(0)
	if len(stalled) != 0 {
		t.Errorf("build stalled despite the engineer's own ample cargo: %v", stalled)
	}
	if len(g.builds) != 0 {
		t.Error("1-turn build did not finish: the engineer is stuck building forever")
	}
	if eng.Storage >= storeBefore {
		t.Errorf("build did not draw the engineer's own cargo: %d -> %d", storeBefore, eng.Storage)
	}
	if c.Material != 0 {
		t.Errorf("the empty adjacent complex was drawn (%d): the build should use the builder's cargo", c.Material)
	}
}

// TestBuildDrawsRichestAdjacentComplex checks that a build bordering BOTH an empty
// detached island (scanned first) and the stocked base draws the base, not the
// island. complexAdjacentTo returns the richest adjacent complex, so an empty cargo
// builder at the bridging cell is funded by the base pool instead of stalling on the
// 0-material island it also touches. AI team builder (only the AI auto draws the pool).
func TestBuildDrawsRichestAdjacentComplex(t *testing.T) {
	g := newGame(t, 24, 24)
	g.State.Teams[0].Type = game.TeamComputer // AI builder: auto resupplied from the base

	// An EMPTY island building edge adjacent above the build cell (scanned first).
	island := game.NewUnitInfo(assets.ADump, 0, 1, 0)
	island.Flags = game.FlagStationary | game.FlagSelectable
	island.Values = &game.UnitValues{}
	island.Values.Set(game.AttribHits, 10)
	ic := g.PlaceBuilding(island, 7, 4)
	if ic == nil {
		t.Fatal("no complex formed for the island")
	}
	ic.Material = 0

	// A STOCKED building edge adjacent below the build cell (a separate complex).
	stocked := game.NewUnitInfo(assets.Radar, 0, 2, 0)
	stocked.Flags = game.FlagStationary | game.FlagSelectable
	stocked.Values = &game.UnitValues{}
	stocked.Values.Set(game.AttribHits, 10)
	sc := g.PlaceBuilding(stocked, 7, 6)
	if sc == nil || sc == ic {
		t.Fatal("the stocked building must form a separate complex")
	}
	sc.Material = 50

	// An empty cargo engineer raising a 2-turn connector at the cell between them.
	eng := game.NewUnitInfo(assets.Engineer, 0, 3, 0)
	eng.Flags = game.FlagMobileLandUnit | game.FlagConstructorUnit | game.FlagSelectable
	eng.Values = &game.UnitValues{}
	eng.InitState()
	eng.Storage = 0
	g.Units.Deploy(eng, 7, 5)

	conn := game.NewUnitInfo(assets.Cnct4W, 0, 4, 0)
	conn.Flags = game.FlagStationary | game.FlagSelectable
	conn.Values = &game.UnitValues{}
	conn.Values.Set(game.AttribTurns, 2) // 2 turns: assert mid build, before any merge
	conn.Values.Set(game.AttribHits, 10)
	g.StartBuild(eng, conn, 7, 5, 1)

	stalled := g.AdvanceBuilds(0)
	if len(stalled) != 0 {
		t.Errorf("build stalled though the stocked base is adjacent: %v", stalled)
	}
	if sc.Material >= 50 {
		t.Errorf("the stocked base pool was not drawn: material still %d", sc.Material)
	}
	if ic.Material != 0 {
		t.Errorf("the empty island was drawn (%d): the build should pick the richest pool", ic.Material)
	}
}

// TestBuildTopsUpConnectedBuilderCargo checks the AI auto resupply: a connected
// COMPUTER team builder that started EMPTY has its own cargo topped up from the base
// pool each turn (the cargo truck stand in) and spends it to build. Material only
// moves pool -> builder (conserved). A player builder does NOT get this - it spends
// its own load and the player refills it manually.
func TestBuildTopsUpConnectedBuilderCargo(t *testing.T) {
	g := newGame(t, 24, 24)
	g.State.Teams[0].Type = game.TeamComputer // AI builder: auto resupplied from the base

	st := game.NewUnitInfo(assets.PowerStn, 0, 1, 0)
	st.Flags = game.FlagBuilding | game.FlagStationary | game.FlagSelectable
	st.Values = &game.UnitValues{}
	st.Values.Set(game.AttribHits, 50)
	c := g.PlaceBuilding(st, 5, 5) // covers (5,5)..(6,6)
	if c == nil {
		t.Fatal("no complex formed")
	}
	c.Material = 100

	// A constructor with capacity 60 but EMPTY, standing right next to the base.
	con := game.NewUnitInfo(assets.Constrct, 0, 2, 0)
	con.Flags = game.FlagMobileLandUnit | game.FlagConstructorUnit | game.FlagSelectable
	con.Values = &game.UnitValues{}
	con.Values.Set(game.AttribStorage, 60)
	con.Storage = 0
	g.Units.Deploy(con, 7, 5)

	// A 2-turn structure on the constructor's own cell (adjacent to the base).
	prod := game.NewUnitInfo(assets.Radar, 0, 3, 0)
	prod.Flags = game.FlagStationary | game.FlagSelectable
	prod.Values = &game.UnitValues{}
	prod.Values.Set(game.AttribTurns, 2) // mid build after one advance
	prod.Values.Set(game.AttribHits, 20)
	g.StartBuild(con, prod, 7, 5, 1)

	stalled := g.AdvanceBuilds(0)
	if len(stalled) != 0 {
		t.Errorf("connected build stalled: %v", stalled)
	}
	if len(g.builds) != 1 {
		t.Fatalf("2-turn build should still be in progress, have %d orders", len(g.builds))
	}
	// The builder was topped up from the pool: its bar shows material, not 0.
	if con.Storage <= 0 || con.Storage >= 60 {
		t.Errorf("connected builder cargo not topped up + spent: storage %d, want 0 < s < 60", con.Storage)
	}
	// Conserved + not duplicated: the pool drop equals the builder's cargo PLUS what the
	// build consumed (which must be a positive amount, proving material actually moved).
	consumed := (100 - c.Material) - con.Storage
	if consumed <= 0 {
		t.Errorf("build consumed no material or it was duplicated: pool drop %d, builder cargo %d", 100-c.Material, con.Storage)
	}
}

// TestPlayerBuilderSpendsOwnNotConnectedPool verifies that a player constructor consumes only its loaded cargo and stalls when empty without drawing from an adjacent complex.
// C++: UnitInfo::UpdateProduction (storage -= rate, no auto refill).
func TestPlayerBuilderSpendsOwnNotConnectedPool(t *testing.T) {
	g := newGame(t, 24, 24) // team 0 = TeamPlayer

	st := game.NewUnitInfo(assets.PowerStn, 0, 1, 0)
	st.Flags = game.FlagBuilding | game.FlagStationary | game.FlagSelectable
	st.Values = &game.UnitValues{}
	st.Values.Set(game.AttribHits, 50)
	c := g.PlaceBuilding(st, 5, 5)
	if c == nil {
		t.Fatal("no complex formed")
	}
	c.Material = 100 // a well stocked base pool right next to the builder

	// A constructor with a small load (6) next to the base, on a long multi turn build.
	con := game.NewUnitInfo(assets.Constrct, 0, 2, 0)
	con.Flags = game.FlagMobileLandUnit | game.FlagConstructorUnit | game.FlagSelectable
	con.Values = &game.UnitValues{}
	con.Values.Set(game.AttribStorage, 60)
	con.Storage = 6
	g.Units.Deploy(con, 7, 5)

	bld := game.NewUnitInfo(assets.Radar, 0, 3, 0)
	bld.Flags = game.FlagStationary | game.FlagSelectable
	bld.Values = &game.UnitValues{}
	bld.Values.Set(game.AttribTurns, 20) // long build: its cost outlasts the 6-unit load
	bld.Values.Set(game.AttribHits, 20)
	g.StartBuild(con, bld, 7, 5, 1)

	// Run several turns: the builder spends its own cargo down and then stalls.
	var stalled []*game.UnitInfo
	for turn := 0; turn < 6; turn++ {
		stalled = g.AdvanceBuilds(0)
	}

	if con.Storage != 0 {
		t.Errorf("player builder storage = %d, want 0 (spent its whole load, no auto refill)", con.Storage)
	}
	if c.Material != 100 {
		t.Errorf("the connected pool was drawn (%d); a player builder must spend only its own cargo", c.Material)
	}
	if len(g.builds) == 0 {
		t.Error("the build finished though the builder ran out of cargo; it should stall until refilled")
	}
	if len(stalled) == 0 {
		t.Error("an out-of-cargo player builder should report a stall")
	}
}

// TestHeadlessConstructorBuild reproduces the in game constructor flow: a mobile
// constructor with material walks to a site and, once adjacent, raises a 2x2
// building over several turns until it deploys.
func TestHeadlessConstructorBuild(t *testing.T) {
	g := newGame(t, 24, 24)
	g.Resources = economy.NewResourceMap(24, 24)

	c := game.NewUnitInfo(assets.Constrct, 0, 1, 0)
	c.Flags = game.FlagMobileLandUnit | game.FlagConstructorUnit | game.FlagSelectable
	c.Values = &game.UnitValues{}
	c.Values.Set(game.AttribSpeed, 16)
	c.InitState()
	c.Speed = 16
	c.Storage = 40
	g.Units.Deploy(c, 5, 5)

	prod := game.NewUnitInfo(assets.MiningSt, 0, 0, 0)
	prod.Flags = game.FlagBuilding | game.FlagStationary | game.FlagSelectable
	prod.Values = &game.UnitValues{}
	prod.Values.Set(game.AttribTurns, 4)
	prod.Values.Set(game.AttribHits, 50)

	siteX, siteY := 8, 8 // 2x2 footprint covers (8,8)..(9,9)
	g.StartBuild(c, prod, siteX, siteY, 1)
	g.OrderMove(c, 7, 8) // adjacent cell left of the footprint
	g.ResolveMovement()

	if len(g.builds) == 0 {
		t.Fatal("build order vanished before construction")
	}
	if !builderAtSite(g.builds[0]) {
		t.Fatalf("constructor not adjacent to site after moving: at (%d,%d)", c.GridX, c.GridY)
	}

	// First turn latches the build (constructor is adjacent).
	g.BeginTeamTurn(0)
	g.EndTeamTurn(0)
	g.AdvanceTurn()
	if len(g.builds) > 0 && !g.builds[0].Started {
		t.Fatal("build did not commit after the constructor reached the site")
	}

	// Committed build must finish even if the constructor is nudged off its spot.
	g.Units.Move(c, 2, 2)
	for turn := 0; turn < 8 && len(g.builds) > 0; turn++ {
		g.BeginTeamTurn(0)
		g.EndTeamTurn(0)
		g.AdvanceTurn()
	}

	if len(g.builds) != 0 {
		o := g.builds[0]
		t.Errorf("build never finished: %d order(s) left, progress %d/%d, pool %d",
			len(g.builds), o.Progress, o.TotalTurns, c.Storage)
	}
	at := g.Units.Cells.At(siteX, siteY)
	found := false
	for _, u := range at {
		if u.Type == assets.MiningSt {
			found = true
		}
	}
	if !found {
		t.Errorf("no mining station deployed at site (%d,%d): %v", siteX, siteY, at)
	}
}

/* // note: unused code in Go port
// addConstructor(g, assets.Constrct, team, gx, gy, 16, 30, 40) builds the same unit, see fixture_test.go
// constructorAt deploys a healthy mobile constructor with movement budget and
// cargo at a cell for the build exit tests.
func constructorAt(g *Game, team, gx, gy int) *game.UnitInfo {
	c := game.NewUnitInfo(assets.Constrct, team, 1, 0)
	c.Flags = game.FlagMobileLandUnit | game.FlagConstructorUnit | game.FlagSelectable
	c.Values = &game.UnitValues{}
	c.Values.Set(game.AttribSpeed, 16)
	c.Values.Set(game.AttribHits, 30)
	c.InitState()
	c.Speed = 16
	c.Storage = 40
	g.Units.Deploy(c, gx, gy)
	return c
}
*/

// miningProduct builds a one turn 2x2 mining station product.
func miningProduct() *game.UnitInfo {
	prod := game.NewUnitInfo(assets.MiningSt, 0, 0, 0)
	prod.Flags = game.FlagBuilding | game.FlagStationary | game.FlagSelectable
	prod.Values = &game.UnitValues{}
	prod.Values.Set(game.AttribTurns, 1) // finishes in a single turn
	prod.Values.Set(game.AttribHits, 50)
	return prod
}

// TestBuilderBuildReadyAndExit checks the faithful build finish flow for a human
// player: the builder does NOT teleport or auto move off; it parks build ready on
// the finished site, offers exit markers around the footprint, and only drives off
// (animated) when the player clicks one (ExitBuildSite).
func TestBuilderBuildReadyAndExit(t *testing.T) {
	g := newGame(t, 24, 24)
	g.Resources = economy.NewResourceMap(24, 24)

	c := addConstructor(g, assets.Constrct, 0, 7, 8, 16, 30, 40) // adjacent to the site's west edge
	siteX, siteY := 8, 8                                         // 2x2 footprint (8,8)..(9,9)
	g.StartBuild(c, miningProduct(), siteX, siteY, 1)

	// One refilled turn latches the build (constructor is adjacent) and finishes it.
	g.BeginTeamTurn(0)

	if len(g.builds) != 0 {
		t.Fatalf("one-turn build did not finish: %d order(s) left", len(g.builds))
	}
	if !cellHasType(g, siteX, siteY, assets.MiningSt) {
		t.Fatalf("no mining station deployed at site (%d,%d)", siteX, siteY)
	}
	// A human builder waits build ready: parked on the site, NO auto move.
	if !g.IsBuildReady(c) {
		t.Fatal("constructor is not build-ready after finishing the structure")
	}
	// The finished building starts collapsed (Unfold>0) until the builder clears it.
	building := cellUnitOfType(g, siteX, siteY, assets.MiningSt)
	if building == nil {
		t.Fatal("could not find the finished building")
	}
	if building.Unfold == 0 {
		t.Error("finished building should start collapsed (Unfold>0), unfolding when the builder leaves")
	}
	if g.ActiveOrders() != 0 {
		t.Errorf("player builder auto-moved (%d orders); it should wait for a marker click", g.ActiveOrders())
	}
	markers := g.ExitMarkers(c)
	if len(markers) == 0 {
		t.Fatal("no exit markers offered around the finished footprint")
	}
	// Every marker must be a free, accessible cell off the footprint.
	for _, m := range markers {
		onSite := m[0] >= siteX && m[0] <= siteX+1 && m[1] >= siteY && m[1] <= siteY+1
		if onSite {
			t.Errorf("exit marker (%d,%d) is on the building footprint", m[0], m[1])
		}
	}

	// A non marker cell is rejected; a marker cell drives the builder off.
	if g.ExitBuildSite(c, siteX, siteY) {
		t.Error("ExitBuildSite accepted a cell that is not an exit marker")
	}
	m := markers[0]
	if !g.ExitBuildSite(c, m[0], m[1]) {
		t.Fatalf("ExitBuildSite rejected a valid marker (%d,%d)", m[0], m[1])
	}
	if g.IsBuildReady(c) {
		t.Error("constructor still build-ready after choosing an exit")
	}
	g.ResolveMovement()
	if c.GridX != m[0] || c.GridY != m[1] {
		t.Errorf("constructor at (%d,%d), want exit marker (%d,%d)", c.GridX, c.GridY, m[0], m[1])
	}
	// The builder has cleared the footprint: the building is released to unfold.
	released := g.ReleasedExpansions()
	found := false
	for _, b := range released {
		if b == building {
			found = true
		}
	}
	if !found {
		t.Error("building not released to unfold after the builder drove off the footprint")
	}
}

// cellUnitOfType returns the unit of the given type at (gx, gy), or nil.
func cellUnitOfType(g *Game, gx, gy int, typ assets.UnitType) *game.UnitInfo {
	for _, u := range g.Units.Cells.At(gx, gy) {
		if u.Type == typ {
			return u
		}
	}
	return nil
}

// TestComputerBuilderAutoExits checks that a COMPUTER team's builder, with no
// player to click a marker, drives off the finished site on its own at the start
// of its next turn (it never teleports and never stays stuck on the structure).
func TestComputerBuilderAutoExits(t *testing.T) {
	p := &game.StartParams{}
	p.Teams[0].Type = game.TeamComputer
	mgr := units.NewManager()
	grid := &access.Grid{W: 24, H: 24, Surf: allLand{}, Units: mgr.Cells}
	g := NewWithGrid(game.NewGameState(p), mgr, grid, 24, 24)
	g.Resources = economy.NewResourceMap(24, 24)

	c := addConstructor(g, assets.Constrct, 0, 7, 8, 16, 30, 40)
	siteX, siteY := 8, 8
	g.StartBuild(c, miningProduct(), siteX, siteY, 1)

	g.BeginTeamTurn(0) // finishes the build; computer builder queues an auto exit
	if !cellHasType(g, siteX, siteY, assets.MiningSt) {
		t.Fatalf("no mining station deployed at site (%d,%d)", siteX, siteY)
	}
	// The computer builder drives off on its own: an active order this turn (it did
	// not wait for a marker click and did not teleport).
	if !g.HasOrder(c) {
		t.Fatal("computer builder did not auto-issue its drive-off order")
	}
	g.ResolveMovement()
	onSite := c.GridX >= siteX && c.GridX <= siteX+1 && c.GridY >= siteY && c.GridY <= siteY+1
	if onSite {
		t.Errorf("computer constructor still on the footprint at (%d,%d)", c.GridX, c.GridY)
	}
	if g.IsBuildReady(c) {
		t.Error("computer builder still build-ready after auto-exiting")
	}
}

// cellHasType reports whether a unit of the given type occupies cell (gx, gy).
func cellHasType(g *Game, gx, gy int, typ assets.UnitType) bool {
	for _, u := range g.Units.Cells.At(gx, gy) {
		if u.Type == typ {
			return true
		}
	}
	return false
}

// addScout deploys a selectable land unit with a scan range for an arbitrary
// team (no movement budget needed for visibility tests).
func addScout(g *Game, team, scan, gx, gy int) *game.UnitInfo {
	u := game.NewUnitInfo(assets.Tank, team, 0, 2)
	u.Flags = game.FlagMobileLandUnit | game.FlagSelectable
	u.Values = &game.UnitValues{}
	u.Values.Set(game.AttribScan, scan)
	g.Units.Deploy(u, gx, gy)
	return u
}

// allLand is a synthetic SurfaceSource: every cell is land, so a land unit
// can route freely. Lets the harness run with no game data.
type allLand struct{}

func (allLand) SurfaceAt(x, y int) uint8 { return world.SurfaceLand }

// newGame builds a headless game on a wxh all land map with one human team.
func newGame(t *testing.T, w, h int) *Game {
	t.Helper()
	mgr := units.NewManager()
	grid := &access.Grid{W: w, H: h, Surf: allLand{}, Units: mgr.Cells}
	p := &game.StartParams{}
	p.Teams[0].Type = game.TeamPlayer
	return NewWithGrid(game.NewGameState(p), mgr, grid, w, h)
}

// addTank deploys a land tank with the given per turn speed budget.
func addTank(g *Game, id, speed, gx, gy int) *game.UnitInfo {
	u := game.NewUnitInfo(assets.Tank, 0, id, 2) // facing east
	u.Flags = game.FlagMobileLandUnit | game.FlagSelectable
	u.Values = &game.UnitValues{}
	u.Values.Set(game.AttribSpeed, speed)
	g.Units.Deploy(u, gx, gy)
	return u
}

// TestHeadlessTenRoundsRefillsBudget verifies that a tank with a one-cell budget advances exactly one cell per turn across ten rounds.
func TestHeadlessTenRoundsRefillsBudget(t *testing.T) {
	g := newGame(t, 32, 4)
	tank := addTank(g, 1, 1, 1, 1) // speed 1 = one Clear cell per turn

	g.RunRounds(10, func(g *Game, team int) {
		g.OrderMove(tank, 30, 1) // far east; recomputed from current cell
	})

	if g.State.TurnCounter != 11 {
		t.Errorf("turn counter = %d, want 11 after 10 rounds", g.State.TurnCounter)
	}
	if tank.GridX != 11 || tank.GridY != 1 {
		t.Errorf("tank at (%d,%d), want (11,1); one cell per refilled turn", tank.GridX, tank.GridY)
	}
	if g.ActiveOrders() != 0 {
		t.Errorf("active orders = %d, want 0 after movement resolved", g.ActiveOrders())
	}
	// The cell hash must track the move.
	if got := g.Units.Cells.At(11, 1); len(got) != 1 || got[0] != tank {
		t.Error("tank not indexed at its final cell")
	}
}

// TestHeadlessFullBudgetInOneTurn confirms a larger budget spends multiple
// cells in a single turn.
func TestHeadlessFullBudgetInOneTurn(t *testing.T) {
	g := newGame(t, 16, 4)
	tank := addTank(g, 1, 4, 1, 1) // 4 points = up to four Clear cells

	g.BeginTeamTurn(0)
	g.OrderMove(tank, 1+5, 1) // ask for five cells; budget caps at four
	g.ResolveMovement()

	if tank.GridX != 5 {
		t.Errorf("tank at grid x %d, want 5 (four cells then out of budget)", tank.GridX)
	}
	if tank.Speed != 0 {
		t.Errorf("speed = %d, want 0 spent", tank.Speed)
	}
}

// TestHeadlessQueuedRouteResumesNextTurn verifies that a move which runs out of
// movement points mid path is not discarded: the remainder is queued and the
// unit auto continues on its next turn once its budget refills.
func TestHeadlessQueuedRouteResumesNextTurn(t *testing.T) {
	g := newGame(t, 16, 4)
	tank := addTank(g, 1, 4, 1, 1) // 4 points; ask for six cells east

	g.BeginTeamTurn(0)
	g.OrderMove(tank, 1+6, 1) // to x=7; budget caps at four cells this turn
	g.ResolveMovement()

	if tank.GridX != 5 {
		t.Fatalf("turn 1: tank at x=%d, want 5 (four cells then out of budget)", tank.GridX)
	}
	pr := g.PendingRoutes()
	if len(pr) != 1 || pr[0].Unit != tank {
		t.Fatalf("turn 1: want the unreached remainder queued (1 route for the tank), got %d", len(pr))
	}

	// Next turn: no new order is issued. BeginTeamTurn must re issue the queued
	// route and the tank continues toward x=7 on its refilled budget.
	g.BeginTeamTurn(0)
	g.ResolveMovement()

	if tank.GridX != 7 {
		t.Errorf("turn 2: tank at x=%d, want 7 (queued route resumed automatically)", tank.GridX)
	}
	if n := len(g.PendingRoutes()); n != 0 {
		t.Errorf("turn 2: queued route should clear once completed, have %d", n)
	}
}

// TestHeadlessCancelClearsQueuedRoute confirms a stopped unit does not resume.
func TestHeadlessCancelClearsQueuedRoute(t *testing.T) {
	g := newGame(t, 16, 4)
	tank := addTank(g, 1, 4, 1, 1)

	g.BeginTeamTurn(0)
	g.OrderMove(tank, 1+6, 1)
	g.ResolveMovement()
	if len(g.PendingRoutes()) != 1 {
		t.Fatalf("setup: expected a queued route")
	}

	g.CancelOrder(tank) // popup Stop
	if n := len(g.PendingRoutes()); n != 0 {
		t.Fatalf("CancelOrder must drop the queued route, have %d", n)
	}
	g.BeginTeamTurn(0)
	g.ResolveMovement()
	if tank.GridX != 5 {
		t.Errorf("tank moved to x=%d after cancel; want it to stay at 5", tank.GridX)
	}
}

// TestHeadlessTwoTeamsAlternate checks the active team rotation across a turn.
func TestHeadlessTwoTeamsAlternate(t *testing.T) {
	mgr := units.NewManager()
	grid := &access.Grid{W: 8, H: 8, Surf: allLand{}, Units: mgr.Cells}
	p := &game.StartParams{}
	p.Teams[0].Type = game.TeamPlayer
	p.Teams[1].Type = game.TeamComputer
	g := NewWithGrid(game.NewGameState(p), mgr, grid, 8, 8)

	var order []int
	g.RunRounds(1, func(g *Game, team int) { order = append(order, team) })

	if len(order) != 2 || order[0] != 0 || order[1] != 1 {
		t.Errorf("team order = %v, want [0 1]", order)
	}
	if g.State.TurnCounter != 2 {
		t.Errorf("turn counter = %d, want 2", g.State.TurnCounter)
	}
}

// addGunTank deploys a combat ready tank (attack/armor/hits/rounds/range/ammo)
// for the given team at a cell.
func addGunTank(g *Game, id, team, gx, gy int) *game.UnitInfo {
	u := game.NewUnitInfo(assets.Tank, team, id, 2)
	u.Flags = game.FlagMobileLandUnit | game.FlagSelectable
	u.Values = &game.UnitValues{}
	u.Values.Set(game.AttribAttack, 18)
	u.Values.Set(game.AttribArmor, 16)
	u.Values.Set(game.AttribHits, 36)
	u.Values.Set(game.AttribRounds, 1)
	u.Values.Set(game.AttribRange, 4)
	u.Values.Set(game.AttribAmmo, 6)
	u.InitState()
	g.Units.Deploy(u, gx, gy)
	return u
}

// TestHeadlessCombatTrade checks the turn loop wires combat: a team turn
// refills shots, an attack removes hits and spends a shot, and a killing blow
// removes the target from the registry.
func TestHeadlessCombatTrade(t *testing.T) {
	g := newGame(t, 8, 8)
	a := addGunTank(g, 1, 0, 2, 2)
	b := addGunTank(g, 2, 1, 3, 2)

	g.BeginTeamTurn(0)
	if a.Shots != 1 {
		t.Fatalf("attacker shots after refill = %d, want 1", a.Shots)
	}
	res, ok := g.Attack(a, 3, 2)
	if !ok || res.Damage != 2 || res.Destroyed {
		t.Fatalf("attack = %+v ok=%v, want damage 2", res, ok)
	}
	if b.Hits != 34 || a.Shots != 0 {
		t.Errorf("b.Hits=%d a.Shots=%d, want 34/0", b.Hits, a.Shots)
	}

	// A second shot this turn is blocked (out of shots).
	if _, ok := g.Attack(a, 3, 2); ok {
		t.Error("second attack should be blocked with no shots left")
	}

	// Wound b to near death, refresh A's turn, finish it off.
	b.Hits = 1
	g.BeginTeamTurn(0)
	res, ok = g.Attack(a, 3, 2)
	if !ok || !res.Destroyed {
		t.Fatalf("killing blow = %+v ok=%v", res, ok)
	}
	if got := g.Units.Cells.At(3, 2); len(got) != 0 {
		t.Errorf("destroyed unit still in cell hash: %d units", len(got))
	}
	for _, u := range g.Units.Units() {
		if u == b {
			t.Error("destroyed unit still in registry")
		}
	}
}

func TestAttackAimsTurretWithoutTurningBody(t *testing.T) {
	g := newGame(t, 8, 8)
	a := addGunTank(g, 1, 0, 2, 4)
	a.Flags |= game.FlagTurretSprite | game.FlagCanHeadRotate
	a.SetAngle(2) // hull driving/facing east
	a.TurretAngle = 2
	a.TurretTarget = 2
	b := addGunTank(g, 2, 1, 2, 2)

	g.BeginTeamTurn(0)
	if _, ok := g.Attack(a, b.GridX, b.GridY); !ok {
		t.Fatal("attack did not fire")
	}
	if a.Angle != 2 {
		t.Errorf("body angle = %d, want 2 (still facing driving direction)", a.Angle)
	}
	if a.TurretTarget != 0 {
		t.Errorf("turret target = %d, want 0 (north toward target)", a.TurretTarget)
	}
	if !a.StepTurret() || a.TurretAngle != 1 {
		t.Errorf("first turret step angle = %d, want 1", a.TurretAngle)
	}
}

func TestAttackTurnsWholeBodyWhenHeadCannotRotate(t *testing.T) {
	g := newGame(t, 8, 8)
	a := addGunTank(g, 1, 0, 2, 4)
	a.Type = assets.RocktLch
	a.Flags = game.FlagMobileLandUnit | game.FlagSelectable
	a.SetAngle(2) // facing east
	b := addGunTank(g, 2, 1, 2, 2)

	g.BeginTeamTurn(0)
	if _, ok := g.Attack(a, b.GridX, b.GridY); !ok {
		t.Fatal("attack did not fire")
	}
	if a.Angle != 0 {
		t.Errorf("body angle = %d, want 0 (whole vehicle turns north to fire)", a.Angle)
	}
	if a.StepTurret() {
		t.Error("rocket launcher should not run independent head rotation")
	}
}

// TestHeadlessEconomyAccumulates checks the turn loop runs the economy: a
// powered mining station complex grows its material store each team turn.
// greenhouse deploys a Greenhouse (Eco Sphere) for a team, powered on or off. id
// must be unique within the game (the unit hash is keyed by ID).
func greenhouse(t *testing.T, g *Game, team, id, gx, gy int, powered bool) *game.UnitInfo {
	t.Helper()
	u := game.NewUnitInfo(assets.Greenhse, team, id, 0)
	u.Flags = game.FlagBuilding | game.FlagStationary
	u.GridX, u.GridY = gx, gy
	if powered {
		u.Order = game.OrderPowerOn
	} else {
		u.Order = game.OrderPowerOff
	}
	u.State = game.OrderStateExecutingOrder
	u.Values = &game.UnitValues{}
	g.Units.Deploy(u, gx, gy)
	return u
}

// TestEcoSphereScoreAccrual checks that each operating Eco Sphere earns the team
// one point per turn (the score victory metric), and a powered off one earns none.
func TestEcoSphereScoreAccrual(t *testing.T) {
	g := newGame(t, 16, 16)
	greenhouse(t, g, 0, 1, 4, 4, true)  // operating: +1/turn
	greenhouse(t, g, 0, 2, 6, 4, true)  // operating: +1/turn
	greenhouse(t, g, 0, 3, 8, 4, false) // powered off: +0

	for turn := 0; turn < 3; turn++ {
		g.EndTeamTurn(0)
	}
	if g.TeamPoints[0] != 6 { // 2 operating * 3 turns
		t.Errorf("TeamPoints[0] = %d, want 6 (2 eco-spheres * 3 turns)", g.TeamPoints[0])
	}
}

// TestScoreVictoryFires checks that once score tracking accrues past the limit,
// the score victory condition ends the game (it could not fire while TeamPoints
// stayed 0). C++: VICTORY_TYPE_SCORE + team_points.
func TestScoreVictoryFires(t *testing.T) {
	g := newGame(t, 16, 16)
	g.State.Teams[1].Type = game.TeamComputer
	g.VictoryType = winloss.VictoryScore
	g.VictoryLimit = 3
	greenhouse(t, g, 0, 1, 4, 4, true)
	// Both sides need something they can still fight with: the default win/loss rule is the
	// original's liveness test, and a team whose whole holding is one eco sphere is beaten
	// before the score can ever accrue (no gun, no plant, no builder).
	addGunTank(g, 3, 0, 5, 5)   // the player's own defender
	addGunTank(g, 2, 1, 10, 10) // enemy unit (id=2, team=1) so the game isn't won by elimination first

	if o := g.Outcome(); o != winloss.Continue {
		t.Fatalf("game ended before any score accrued: %v", o)
	}
	for turn := 0; turn < 3; turn++ { // reach the 3-point limit
		g.EndTeamTurn(0)
	}
	if g.TeamPoints[0] < g.VictoryLimit {
		t.Fatalf("score %d did not reach limit %d", g.TeamPoints[0], g.VictoryLimit)
	}
	if o := g.Outcome(); o != winloss.PlayerWon {
		t.Errorf("Outcome at score limit = %v, want PlayerWon", o)
	}
}

func TestHeadlessEconomyAccumulates(t *testing.T) {
	g := newGame(t, 16, 16)
	g.Resources = economy.NewResourceMap(16, 16)
	g.Resources.SetDeposit(5, 5, economy.CargoRaw, 10)
	g.Resources.SetDeposit(6, 5, economy.CargoRaw, 10)

	st := stationaryBuilding(assets.MiningSt, 0, 5, 5)
	st.GridX, st.GridY = 5, 5
	g.Units.Deploy(st, 5, 5)
	g.Resources.SetInitialMining(st)

	// A power generator powers the mining station (it browns out without one).
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
	c.Fuel = 50 // a starting fuel buffer for the generator to burn
	g.Complexes = append(g.Complexes, c)

	// A Storage Unit, or the mine's own 25 would cap the run at 25 and spill the rest.
	addStore(g, c, assets.ADump, 9, 5)

	for turn := 0; turn < 3; turn++ {
		g.BeginTeamTurn(0)
		g.EndTeamTurn(0)
		g.AdvanceTurn()
	}
	// No fuel deposit here, so raw fills the full 16 cap: 16/turn over 3 turns, and the
	// base (25 + 50) can hold all of it.
	if c.Material != 48 {
		t.Errorf("complex material after 3 turns = %d, want 48 (16/turn)", c.Material)
	}
}

// TestEconomySustainsWithFuelDeposit verifies that AutoBalanceFuel shifts mining toward the six-fuel demand of active Power Stations.
// The setup mirrors random2 with two mining stations and two power stations.
func TestEconomySustainsWithFuelDeposit(t *testing.T) {
	g := newGame(t, 16, 16)
	g.Resources = economy.NewResourceMap(16, 16)
	// Generous raw + fuel deposits across the mining stations' areas.
	for x := 4; x <= 9; x++ {
		for y := 4; y <= 7; y++ {
			g.Resources.SetDeposit(x, y, economy.CargoRaw, 16)
		}
	}
	g.Resources.SetDeposit(5, 5, economy.CargoFuel, 16)
	g.Resources.SetDeposit(8, 5, economy.CargoFuel, 16)

	c := economy.NewComplex(1)
	c.Team = 0

	mine := func(gx, gy int) {
		st := stationaryBuilding(assets.MiningSt, 0, gx, gy)
		g.Units.Deploy(st, gx, gy)
		g.Resources.SetInitialMining(st)
		c.Grow(st)
	}
	powerStation := func(gx, gy int) {
		ps := game.NewUnitInfo(assets.PowerStn, 0, 2, 0)
		ps.Flags = game.FlagBuilding | game.FlagStationary
		ps.Order = game.OrderPowerOn
		ps.State = game.OrderStateExecutingOrder
		ps.Values = &game.UnitValues{}
		g.Units.Deploy(ps, gx, gy)
		c.Grow(ps)
	}
	mine(5, 5)
	mine(8, 5)
	powerStation(5, 7)
	powerStation(8, 7)

	// A Fuel Tank, so the base can BANK fuel: a power station stores none, and without
	// somewhere to put it the surplus spills every turn (CapStores).
	addStore(g, c, assets.FDump, 11, 7)

	c.Fuel = 8 // a small buffer: the stations must replenish it to keep the base alive
	g.Complexes = append(g.Complexes, c)

	startFuel := c.Fuel
	_, capacity := c.CargoInfo()

	for turn := 0; turn < 30; turn++ {
		before := c.Material
		g.BeginTeamTurn(0)
		g.EndTeamTurn(0)
		g.AdvanceTurn()

		// The base must keep EARNING: material rises until the stores are full, and once
		// full it must HOLD there (a death spiral would drain it back toward zero).
		if c.Material <= before && c.Material < capacity.Raw {
			t.Fatalf("turn %d: material stalled at %d of %d (the base stopped producing - death spiral)",
				turn, c.Material, capacity.Raw)
		}
	}

	if c.Fuel < startFuel {
		t.Errorf("fuel collapsed from %d to %d over 30 turns (death spiral not prevented)", startFuel, c.Fuel)
	}
}

// retailStorage is ATTRIB_STORAGE for the buildings these fixtures raise, straight from the
// retail ATTRIBS table. It matters because a base can only HOLD what its buildings hold
// (Complex.CapStores): a fixture that mines more than its storage must raise a Storage Unit,
// exactly as a player must. Everything else (power, factories, habitat) stores nothing.
var retailStorage = map[assets.UnitType]int{
	assets.MiningSt: 25,
	assets.ADump:    50, // Storage Unit (raw)
	assets.FDump:    50, // Fuel Tank
	assets.GoldSm:   50, // Gold Vault
}

// stationaryBuilding makes a powered, active building of a type at a cell, with its real
// storage capacity.
func stationaryBuilding(typ assets.UnitType, team, gx, gy int) *game.UnitInfo {
	u := game.NewUnitInfo(typ, team, 0, 0)
	u.Flags = game.FlagBuilding | game.FlagStationary
	u.Order = game.OrderPowerOn
	u.State = game.OrderStateExecutingOrder
	u.Values = &game.UnitValues{}
	u.Values.Set(game.AttribStorage, retailStorage[typ])

	return u
}

// addStore raises a powered storage building into a complex. A base that mines more than it
// can hold spills the difference, so a fixture that wants to bank material needs one.
func addStore(g *Game, c *economy.Complex, typ assets.UnitType, gx, gy int) *game.UnitInfo {
	u := stationaryBuilding(typ, c.Team, gx, gy)
	g.Units.Deploy(u, gx, gy)
	c.Grow(u)

	return u
}

// TestHeadlessBuildBaseAndTank verifies that an adjacent mining station funds a factory in the same complex until it deploys a tank.
func TestHeadlessBuildBaseAndTank(t *testing.T) {
	g := newGame(t, 16, 16)
	g.Resources = economy.NewResourceMap(16, 16)
	g.Resources.SetDeposit(2, 2, economy.CargoRaw, 16)
	g.Resources.SetDeposit(3, 2, economy.CargoRaw, 16)

	// A mining station at (2,2) and a factory abutting it at (4,2) -> one base.
	mine := stationaryBuilding(assets.MiningSt, 0, 2, 2)
	cm := g.PlaceBuilding(mine, 2, 2)
	factory := stationaryBuilding(assets.LandPlt, 0, 4, 2)
	cf := g.PlaceBuilding(factory, 4, 2)
	if cm != cf {
		t.Fatal("mining station and adjacent factory should share one complex")
	}
	if cf.Buildings != 2 {
		t.Fatalf("base complex buildings = %d, want 2", cf.Buildings)
	}

	// A power generator joins the complex so the mining station has power to run
	// (mining browns out without it).
	gen := stationaryBuilding(assets.PowGen, 0, 2, 4)
	gen.Flags = game.FlagStationary // 1x1
	g.PlaceBuilding(gen, 2, 4)
	// A SECOND generator: one PowGen supplies 1 power and every consumer (mining,
	// factory) draws 1, and mining is served first - so a single generator cannot also
	// power the factory; it would be starved (the power deficit rule). Two cover both.
	gen2 := stationaryBuilding(assets.PowGen, 0, 3, 4)
	gen2.Flags = game.FlagStationary // 1x1
	g.PlaceBuilding(gen2, 3, 4)
	for _, cx := range g.Complexes { // a starting fuel buffer for the generators
		if cx.Team == 0 {
			cx.Fuel = 50
		}
	}

	// Order the factory to build a tank (5 turns at rate 1). With the factory output
	// flow, a finished unit PARKS on the plant build ready and waits for its exit
	// (rather than teleporting onto a deploy cell).
	tank := game.NewUnitInfo(assets.Tank, 0, 0, 0)
	tank.Flags = game.FlagMobileLandUnit | game.FlagSelectable
	tank.Values = &game.UnitValues{}
	tank.Values.Set(game.AttribTurns, 5)
	tank.Values.Set(game.AttribHits, 36)
	tank.Values.Set(game.AttribSpeed, 8)
	g.StartBuild(factory, tank, factory.GridX, factory.GridY, 1)

	before := len(g.Units.Units())
	for turn := 0; turn < 8 && g.BuildReadyInfo(factory) == nil; turn++ { // mining funds + completes
		g.BeginTeamTurn(0)
		g.EndTeamTurn(0)
		g.AdvanceTurn()
	}

	if len(g.builds) != 0 {
		t.Errorf("build order still active after the build: %d", len(g.builds))
	}
	if got := len(g.Units.Units()); got != before+1 {
		t.Errorf("unit count = %d, want %d (a tank was produced)", got, before+1)
	}
	br := g.BuildReadyInfo(factory)
	if br == nil || br.Product != assets.Tank || !br.Factory {
		t.Fatalf("factory output not parked build-ready: %+v", br)
	}
	if tank.Hits != 36 {
		t.Errorf("produced tank hits = %d, want 36 (full stats)", tank.Hits)
	}
	if tank.GridX != factory.GridX || tank.GridY != factory.GridY {
		t.Errorf("parked tank at (%d,%d), want the factory cell (%d,%d)", tank.GridX, tank.GridY, factory.GridX, factory.GridY)
	}
}

// TestFactoryStarvedByPowerDeficit: one generator powers the mining station but not
// ALSO the factory (factories are cut FIRST on a power deficit), so the factory's
// build HOLDS and the factory reads PowerStarved; adding a second generator clears the deficit and
// the build resumes. (The world rule the bench's single tiny generator should show.)
func TestFactoryStarvedByPowerDeficit(t *testing.T) {
	g := newGame(t, 16, 16)
	g.Resources = economy.NewResourceMap(16, 16)
	g.Resources.SetDeposit(2, 2, economy.CargoRaw, 16)
	mine := stationaryBuilding(assets.MiningSt, 0, 2, 2)
	g.PlaceBuilding(mine, 2, 2)
	factory := stationaryBuilding(assets.LandPlt, 0, 4, 2)
	g.PlaceBuilding(factory, 4, 2)
	gen := stationaryBuilding(assets.PowGen, 0, 2, 4)
	gen.Flags = game.FlagStationary
	g.PlaceBuilding(gen, 2, 4)
	for _, cx := range g.Complexes {
		if cx.Team == 0 {
			cx.Fuel = 50
			cx.Material = 100
		}
	}
	tank := game.NewUnitInfo(assets.Tank, 0, 0, 0)
	tank.Flags = game.FlagMobileLandUnit | game.FlagSelectable
	tank.Values = &game.UnitValues{}
	tank.Values.Set(game.AttribTurns, 5)
	tank.Values.Set(game.AttribHits, 36)
	tank.Values.Set(game.AttribSpeed, 8)
	g.StartBuild(factory, tank, factory.GridX, factory.GridY, 1)
	factory.BuildRate = 1 // a producing factory draws power (SetFactoryQueue would set this)

	// One generator (1 power): mining is served first, so the factory is STARVED.
	for turn := 0; turn < 6; turn++ {
		g.BeginTeamTurn(0)
		g.EndTeamTurn(0)
		g.AdvanceTurn()
	}
	if !factory.PowerStarved {
		t.Error("factory should be power-starved (one generator cannot power mining + factory)")
	}
	if g.BuildReadyInfo(factory) != nil {
		t.Error("a power-starved factory must NOT finish its build")
	}

	// Add a second generator -> deficit cleared -> the factory resumes + finishes.
	gen2 := stationaryBuilding(assets.PowGen, 0, 3, 4)
	gen2.Flags = game.FlagStationary
	g.PlaceBuilding(gen2, 3, 4)
	for turn := 0; turn < 8 && g.BuildReadyInfo(factory) == nil; turn++ {
		g.BeginTeamTurn(0)
		g.EndTeamTurn(0)
		g.AdvanceTurn()
	}
	if factory.PowerStarved {
		t.Error("factory should no longer be starved once a second generator is added")
	}
	if g.BuildReadyInfo(factory) == nil {
		t.Error("factory should finish the tank once it has power")
	}
}

/* // note: unused code in Go port
// tested Land, commented out with it; TestDeployStartBaseOneComplex covers the live landing.
// TestHeadlessLanding verifies that a colony landing creates a powered mining complex and slab, sets team gold, and places loadout units on reachable cells.
func TestHeadlessLanding(t *testing.T) {
	g := newGame(t, 24, 24)
	g.Resources = economy.NewResourceMap(24, 24)
	g.Resources.SetDeposit(10, 10, economy.CargoRaw, 16)
	g.Resources.SetDeposit(11, 10, economy.CargoRaw, 16)

	mine := game.NewUnitInfo(assets.MiningSt, 0, 0, 0)
	mine.Flags = game.FlagBuilding | game.FlagStationary | game.FlagRequiresSlab
	mine.Values = &game.UnitValues{}
	pg := game.NewUnitInfo(assets.PowGen, 0, 0, 0)
	pg.Flags = game.FlagBuilding | game.FlagStationary | game.FlagRequiresSlab
	pg.Values = &game.UnitValues{}

	con := game.NewUnitInfo(assets.Constrct, 0, 0, 0)
	con.Flags = game.FlagMobileLandUnit | game.FlagSelectable
	con.Values = &game.UnitValues{}
	scout := game.NewUnitInfo(assets.Scout, 0, 0, 0)
	scout.Flags = game.FlagMobileLandUnit | game.FlagSelectable
	scout.Values = &game.UnitValues{}

	c := g.Land(0, 10, 10, 150, mine, pg, []*game.UnitInfo{con, scout})

	// One complex with the mining station and the power generator.
	if c.Buildings != 2 {
		t.Errorf("starting complex buildings = %d, want 2 (mine + power gen)", c.Buildings)
	}
	if g.Networks[0] == nil || g.Networks[0].ComplexOf(mine) != g.Networks[0].ComplexOf(pg) {
		t.Error("mine and power generator should share one complex")
	}
	if c.Gold != 150 || g.Gold[0] != 150 {
		t.Errorf("gold = complex %d team %d, want 150/150", c.Gold, g.Gold[0])
	}
	// The mining station mined material (16 raw, no fuel deposit).
	if mine.RawMining != 16 {
		t.Errorf("mine raw mining = %d, want 16", mine.RawMining)
	}
	// A large slab was laid under the mining station.
	slabs := 0
	for _, u := range g.Units.Cells.At(10, 10) {
		if u.Type == assets.LrgSlab {
			slabs++
		}
	}
	if slabs != 1 {
		t.Errorf("large slabs under mining station = %d, want 1", slabs)
	}
	// Both loadout units were placed on the map.
	if con.GridX < 0 || scout.GridX < 0 {
		t.Errorf("loadout units not placed: con(%d,%d) scout(%d,%d)", con.GridX, con.GridY, scout.GridX, scout.GridY)
	}
	if con.Position() == scout.Position() {
		t.Error("loadout units should land on distinct cells")
	}
}
*/

// TestHeadlessPlayToVictory verifies that play continues with two active teams and ends in victory after the player destroys the enemy's final unit.
func TestHeadlessPlayToVictory(t *testing.T) {
	mgr := units.NewManager()
	grid := &access.Grid{W: 8, H: 8, Surf: allLand{}, Units: mgr.Cells}
	p := &game.StartParams{}
	p.Teams[0].Type = game.TeamPlayer
	p.Teams[1].Type = game.TeamComputer
	g := NewWithGrid(game.NewGameState(p), mgr, grid, 8, 8)

	a := addGunTank(g, 1, 0, 2, 2)
	enemy := addGunTank(g, 2, 1, 3, 2)
	enemy.Hits = 1 // one shot finishes it

	if got := g.Outcome(); got != winloss.Continue {
		t.Fatalf("outcome with both teams in play = %d, want Continue", got)
	}

	g.BeginTeamTurn(0)
	res, ok := g.Attack(a, 3, 2)
	if !ok || !res.Destroyed {
		t.Fatalf("killing blow = %+v ok=%v", res, ok)
	}
	g.EndTeamTurn(0)

	if !g.State.Teams[1].Eliminated {
		t.Error("team 1 should be eliminated after losing its last unit")
	}
	if got := g.Status(); got.TeamsInPlay != 1 || got.TeamRank[0] != 1 {
		t.Errorf("status = in-play %d rank[0] %d, want 1/1", got.TeamsInPlay, got.TeamRank[0])
	}
	if got := g.Outcome(); got != winloss.PlayerWon {
		t.Errorf("outcome after wiping the enemy = %d, want PlayerWon", got)
	}
}

// mixedSurf is a synthetic SurfaceSource with a water column at x == waterX.
type mixedSurf struct{ waterX int }

func (m mixedSurf) SurfaceAt(x, y int) uint8 {
	if x == m.waterX {
		return world.SurfaceWater
	}
	return world.SurfaceLand
}

// newMixedGame builds a headless game where column waterX is open water and
// all other columns are land.
func newMixedGame(t *testing.T, w, h, waterX int) *Game {
	t.Helper()
	mgr := units.NewManager()
	surf := mixedSurf{waterX: waterX}
	grid := &access.Grid{W: w, H: h, Surf: surf, Units: mgr.Cells}
	p := &game.StartParams{}
	p.Teams[0].Type = game.TeamPlayer
	return NewWithGrid(game.NewGameState(p), mgr, grid, w, h)
}

// TestSurveyorCrossesWater verifies that a Surveyor carrying only FlagMobileLandUnit can cross open water at Clear cost without FlagMobileSeaUnit.
//
// C++ reference: access.cpp line 121 (unit_type != SURVEYOR excludes the
// +8 water cost) and the Surveyor's land_type entry including water.
func TestSurveyorCrossesWater(t *testing.T) {
	// 5-wide map: columns 0-1 land, column 2 water, columns 3-4 land.
	g := newMixedGame(t, 5, 3, 2)

	surv := game.NewUnitInfo(assets.Surveyor, 0, 1, 0)
	surv.Flags = game.FlagMobileLandUnit | game.FlagSelectable
	surv.Values = &game.UnitValues{}
	surv.Values.Set(game.AttribSpeed, 8)
	g.Units.Deploy(surv, 0, 1) // start on land (col 0)

	// CanEnter must be true for the water cell and the far side land cell.
	if !g.CanEnter(surv, 2, 1) {
		t.Error("Surveyor should be able to enter water cell (2,1)")
	}
	if !g.CanEnter(surv, 4, 1) {
		t.Error("Surveyor should be able to enter land cell (4,1) past water")
	}

	// A pure land Tank on the same map must NOT be able to enter the water cell.
	tank := game.NewUnitInfo(assets.Tank, 0, 2, 0)
	tank.Flags = game.FlagMobileLandUnit | game.FlagSelectable
	tank.Values = &game.UnitValues{}
	g.Units.Deploy(tank, 1, 1)
	if g.CanEnter(tank, 2, 1) {
		t.Error("Tank must not enter the water cell")
	}
}

// TestHeadlessVisibilityTracksMovement checks the turn loop keeps the team
// view in sync: an enemy outside scan range is invisible, then becomes visible
// once a scout drives within range, and the cell behind it stays explored.
func TestHeadlessVisibilityTracksMovement(t *testing.T) {
	g := newGame(t, 32, 4)
	scout := addTank(g, 1, 8, 1, 1)      // a fast mover on team 0
	scout.Values.Set(game.AttribScan, 2) // scan radius 2 tiles
	enemy := addScout(g, 1, 0, 10, 1)    // stationary enemy far east
	enemy.Flags |= game.FlagMobileLandUnit

	g.BeginTeamTurn(0)
	if g.Vis.VisibleToTeam(enemy, 0) {
		t.Fatal("enemy should be invisible before the scout approaches")
	}

	g.OrderMove(scout, 9, 1) // drive next to the enemy
	g.ResolveMovement()

	if scout.GridX != 9 {
		t.Fatalf("scout at x=%d, want 9", scout.GridX)
	}
	if !g.Vis.VisibleToTeam(enemy, 0) {
		t.Error("enemy should be visible once the scout is within scan range")
	}
	if got := g.Vis.Team(0).FogAt(1, 1); got != vis.FogExplored {
		t.Errorf("scout's starting cell fog = %d, want FogExplored", got)
	}
}
