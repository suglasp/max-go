// rubble_test.go : tests rubble.go.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package sim

import (
	"testing"

	"github.com/suglasp/max-go/internal/assets"
	"github.com/suglasp/max-go/internal/game"
)

// addBulldozer deploys a player Bulldozer at (gx, gy).
func addBulldozer(g *Game, id, gx, gy int) *game.UnitInfo {
	u := game.NewUnitInfo(assets.Bulldozr, 0, id, 0)
	u.Flags = game.FlagMobileLandUnit | game.FlagSelectable
	u.Values = &game.UnitValues{}
	u.Values.Set(game.AttribSpeed, 8)
	u.Speed = 8
	g.Units.Deploy(u, gx, gy)
	return u
}

// addBuilding deploys a player building (2x2, leaves a large pile when destroyed).
func addBuilding(g *Game, id, gx, gy int) *game.UnitInfo {
	u := game.NewUnitInfo(assets.PowerStn, 0, id, 0)
	u.Flags = game.FlagStationary | game.FlagBuilding | game.FlagSelectable
	u.Values = &game.UnitValues{}
	g.Units.Deploy(u, gx, gy)
	return u
}

// TestDestroyLeavesRubble: a destroyed building leaves a large pile, a destroyed
// mobile unit a small one; ground cover and mines leave none.
func TestDestroyLeavesRubble(t *testing.T) {
	g := newGame(t, 16, 16)

	bld := addBuilding(g, 1, 4, 4)
	g.DestroyUnit(bld)
	if large, ok := g.RubbleAt(4, 4); !ok || !large {
		t.Errorf("destroyed building: RubbleAt(4,4) = (large=%v, ok=%v), want (true, true)", large, ok)
	}

	tank := addTank(g, 2, 1, 8, 8)
	g.DestroyUnit(tank)
	if large, ok := g.RubbleAt(8, 8); !ok || large {
		t.Errorf("destroyed tank: RubbleAt(8,8) = (large=%v, ok=%v), want (false, true)", large, ok)
	}

	// A mine leaves no rubble (it is consumed, not wrecked).
	mine := game.NewUnitInfo(assets.LandMine, 0, 3, 0)
	mine.Flags = game.FlagStationary
	g.Units.Deploy(mine, 2, 2)
	g.DestroyUnit(mine)
	if _, ok := g.RubbleAt(2, 2); ok {
		t.Error("a destroyed mine left rubble; it must not")
	}

	if len(g.RubbleCells()) != 2 {
		t.Errorf("RubbleCells = %d, want 2 (building + tank)", len(g.RubbleCells()))
	}
}

// TestBuildClearsFootprintRubble: finishing a structure over a debris pile auto clears
// it (the original Access_DestroyUtilities on build completion) - a 1x1 over a small pile, a
// 2x2 over a large pile, and a 1x1 on a cell a large pile covers from its up/left anchor.
func TestBuildClearsFootprintRubble(t *testing.T) {
	g := newGame(t, 24, 24)

	// (1) a 1x1 building over a small pile clears it.
	g.DestroyUnit(addTank(g, 1, 1, 4, 4)) // small pile at (4,4)
	if _, ok := g.RubbleAt(4, 4); !ok {
		t.Fatal("setup: no small pile at (4,4)")
	}
	radar := game.NewUnitInfo(assets.Radar, 0, 10, 0)
	radar.Flags = game.FlagStationary | game.FlagSelectable
	g.clearFootprintRubble(radar, 4, 4)
	if _, ok := g.RubbleAt(4, 4); ok {
		t.Error("a 1x1 building over a small pile should clear it")
	}

	// (2) a 2x2 building over a large 2x2 pile clears the whole pile.
	g.DestroyUnit(addBuilding(g, 2, 9, 9)) // large pile anchor (9,9), covers 9..10
	if large, ok := g.RubbleAt(9, 9); !ok || !large {
		t.Fatal("setup: no large pile at (9,9)")
	}
	power := game.NewUnitInfo(assets.PowerStn, 0, 11, 0)
	power.Flags = game.FlagStationary | game.FlagBuilding | game.FlagSelectable
	g.clearFootprintRubble(power, 9, 9)
	if _, ok := g.RubbleAt(9, 9); ok {
		t.Error("a 2x2 building over a large pile should clear it")
	}

	// (3) a 1x1 building on a cell a large pile covers from its up/left anchor.
	g.DestroyUnit(addBuilding(g, 3, 16, 16)) // large pile anchor (16,16), covers 16..17
	radar2 := game.NewUnitInfo(assets.Radar, 0, 12, 0)
	radar2.Flags = game.FlagStationary | game.FlagSelectable
	g.clearFootprintRubble(radar2, 17, 17) // covered by the pile anchored up/left at (16,16)
	if _, ok := g.RubbleAt(16, 16); ok {
		t.Error("a 1x1 build on a cell a large pile covers should clear the whole pile")
	}
}

// TestConnectorDoesNotClearRubble: an engineer laying a connector (CNCT_4W) over a
// pile leaves it - the original sets skip_rubble_removal for a connector.
func TestConnectorDoesNotClearRubble(t *testing.T) {
	g := newGame(t, 16, 16)
	g.DestroyUnit(addTank(g, 1, 1, 6, 6)) // small pile at (6,6)
	conn := game.NewUnitInfo(assets.Cnct4W, 0, 10, 0)
	conn.Flags = game.FlagStationary | game.FlagGroundCover
	g.clearFootprintRubble(conn, 6, 6)
	if _, ok := g.RubbleAt(6, 6); !ok {
		t.Error("a connector must NOT clear the rubble under it (skip_rubble_removal)")
	}
}

// TestClearSmallRubble: a bulldozer on a small pile clears it in 1 turn.
func TestClearSmallRubble(t *testing.T) {
	g := newGame(t, 16, 16)
	tank := addTank(g, 1, 1, 6, 6)
	g.DestroyUnit(tank) // small pile at (6,6)

	dozer := addBulldozer(g, 2, 6, 6)
	if !g.CanClearRubble(dozer) {
		t.Fatal("CanClearRubble = false for a bulldozer parked on a pile")
	}
	if !g.StartClearRubble(dozer) {
		t.Fatal("StartClearRubble returned false")
	}
	if !g.IsClearingRubble(dozer) || dozer.Order != game.OrderClear {
		t.Errorf("after StartClearRubble: clearing=%v order=%v", g.IsClearingRubble(dozer), dozer.Order)
	}
	if g.ClearTurnsLeft(dozer) != clearSmallTurns {
		t.Errorf("ClearTurnsLeft = %d, want %d", g.ClearTurnsLeft(dozer), clearSmallTurns)
	}

	g.tickClearing(0) // one turn of work
	if _, ok := g.RubbleAt(6, 6); ok {
		t.Error("small pile not cleared after 1 turn")
	}
	// Done clearing: freed to a normal idle order (faithful - the bulldozer just
	// becomes a free unit, NO exit markers; the original SpawnNewUnit sets ORDER_AWAIT).
	if g.IsClearingRubble(dozer) || dozer.Clearing {
		t.Error("bulldozer still clearing after the small pile was cleared")
	}
	if g.IsBuildReady(dozer) {
		t.Error("a cleared bulldozer must NOT use the build-ready exit markers")
	}
	if dozer.Order != game.OrderAwait {
		t.Errorf("bulldozer not freed after clearing: order=%v, want OrderAwait", dozer.Order)
	}
}

// TestBulldozerReclaimsRubbleMaterial: clearing a pile credits its salvaged raw
// (half the destroyed unit's build cost) into the bulldozer's own cargo.
func TestBulldozerReclaimsRubbleMaterial(t *testing.T) {
	g := newGame(t, 16, 16)
	tank := addTank(g, 1, 1, 6, 6)
	tank.Values.Set(game.AttribTurns, 10) // gives the unit a non zero build cost
	want := rubbleSalvage(tank)
	if want <= 0 {
		t.Fatalf("setup: expected positive salvage, got %d", want)
	}
	g.DestroyUnit(tank)

	dozer := addBulldozer(g, 2, 6, 6)
	dozer.Values.Set(game.AttribStorage, want+100) // ample capacity
	dozer.Storage = 0
	if !g.StartClearRubble(dozer) {
		t.Fatal("StartClearRubble returned false")
	}
	g.tickClearing(0) // the small pile clears in one turn
	if dozer.Storage != want {
		t.Errorf("dozer reclaimed %d material, want %d", dozer.Storage, want)
	}
}

// TestBulldozerReclaimClampsToCapacity: the reclaim never overflows the dozer's
// ATTRIB_STORAGE; the remainder is lost when it is full.
func TestBulldozerReclaimClampsToCapacity(t *testing.T) {
	g := newGame(t, 16, 16)
	tank := addTank(g, 1, 1, 5, 5)
	tank.Values.Set(game.AttribTurns, 10)
	want := rubbleSalvage(tank)
	g.DestroyUnit(tank)

	dozer := addBulldozer(g, 2, 5, 5)
	capacity := want / 2 // too small to hold the whole pile
	dozer.Values.Set(game.AttribStorage, capacity)
	dozer.Storage = 0
	if !g.StartClearRubble(dozer) {
		t.Fatal("StartClearRubble returned false")
	}
	g.tickClearing(0)
	if dozer.Storage != capacity {
		t.Errorf("reclaim clamp: storage=%d, want capacity %d", dozer.Storage, capacity)
	}
}

// TestRubbleMaterialRoundTrips: the pile's salvage value survives the save/restore
// snapshot (RubbleCells -> RestoreRubble), so a pile cleared after a load still pays.
func TestRubbleMaterialRoundTrips(t *testing.T) {
	g := newGame(t, 16, 16)
	tank := addTank(g, 1, 1, 7, 7)
	tank.Values.Set(game.AttribTurns, 10)
	g.DestroyUnit(tank)

	var mat int
	for _, c := range g.RubbleCells() {
		if c.X == 7 && c.Y == 7 {
			mat = c.Material
		}
	}
	if mat <= 0 {
		t.Fatalf("rubble material not captured in the snapshot: %d", mat)
	}

	g2 := newGame(t, 16, 16)
	g2.RestoreRubble(g.RubbleCells())
	dozer := addBulldozer(g2, 2, 7, 7)
	dozer.Values.Set(game.AttribStorage, mat+100)
	dozer.Storage = 0
	if !g2.StartClearRubble(dozer) {
		t.Fatal("StartClearRubble returned false after restore")
	}
	g2.tickClearing(0)
	if dozer.Storage != mat {
		t.Errorf("after restore+clear, dozer storage=%d, want %d", dozer.Storage, mat)
	}
}

// TestBulldozerKeepsSpeedOnClearFinish verifies that a bulldozer retains its turn-start movement budget when clearing completes, allowing it to leave immediately.
func TestBulldozerKeepsSpeedOnClearFinish(t *testing.T) {
	g := newGame(t, 16, 16)
	tank := addTank(g, 1, 1, 6, 6)
	tank.Values.Set(game.AttribTurns, 10)
	g.DestroyUnit(tank) // small pile at (6,6), clears in one turn

	dozer := addBulldozer(g, 2, 6, 6)
	if !g.StartClearRubble(dozer) {
		t.Fatal("StartClearRubble returned false")
	}
	// BeginTeamTurn refills the movement budget before tickClearing; simulate that refill.
	budget := dozer.Values.Get(game.AttribSpeed)
	dozer.Speed = budget
	if budget <= 0 {
		t.Fatal("setup: dozer has no movement budget")
	}

	g.tickClearing(0) // the small pile finishes this turn

	if g.IsClearingRubble(dozer) {
		t.Fatal("dozer still clearing after the pile finished")
	}
	if dozer.Speed != budget {
		t.Errorf("finished dozer Speed = %d, want its refilled budget %d (it must keep movement to drive off)", dozer.Speed, budget)
	}
}

// TestClearLargeRubble: a building pile takes 4 turns and occupies the bulldozer.
func TestClearLargeRubble(t *testing.T) {
	g := newGame(t, 16, 16)
	bld := addBuilding(g, 1, 5, 5)
	g.DestroyUnit(bld) // large pile at (5,5)

	dozer := addBulldozer(g, 2, 5, 5)
	if !g.StartClearRubble(dozer) {
		t.Fatal("StartClearRubble returned false on a large pile")
	}
	if g.ClearTurnsLeft(dozer) != clearLargeTurns {
		t.Fatalf("ClearTurnsLeft = %d, want %d", g.ClearTurnsLeft(dozer), clearLargeTurns)
	}

	// A clearing bulldozer cannot move and gets no budget.
	if g.OrderMove(dozer, 1, 1) {
		t.Error("a clearing bulldozer accepted a move order")
	}
	for i := 0; i < clearLargeTurns-1; i++ {
		g.tickClearing(0)
		if dozer.Speed != 0 {
			t.Errorf("turn %d: clearing bulldozer Speed = %d, want 0 (occupied)", i, dozer.Speed)
		}
		if _, ok := g.RubbleAt(5, 5); !ok {
			t.Errorf("large pile cleared early (after %d turns)", i+1)
		}
	}
	g.tickClearing(0) // the 4th turn finishes it
	if _, ok := g.RubbleAt(5, 5); ok {
		t.Error("large pile not cleared after 4 turns")
	}
	if g.IsClearingRubble(dozer) {
		t.Error("bulldozer still busy after the large pile was cleared")
	}
	// A 2x2 clear ends with the leave slide armed (PositionInTape DEINIT) and the
	// dozer freed; no build ready exit markers.
	if g.IsBuildReady(dozer) {
		t.Error("a cleared bulldozer must NOT use the build-ready exit markers")
	}
	if !dozer.Leaving || dozer.Order != game.OrderAwait {
		t.Errorf("2x2 clear did not arm the leave slide + free the dozer: leaving=%v order=%v", dozer.Leaving, dozer.Order)
	}
}

// TestClearGatingAndStop: only a bulldozer on a pile can clear; Stop cancels.
func TestClearGatingAndStop(t *testing.T) {
	g := newGame(t, 16, 16)

	// A non bulldozer never clears, even standing on rubble.
	tank := addTank(g, 1, 1, 3, 3)
	g.DestroyUnit(tank)
	other := addTank(g, 2, 1, 3, 3)
	if IsBulldozer(other) || g.CanClearRubble(other) || g.StartClearRubble(other) {
		t.Error("a non-bulldozer cleared rubble")
	}
	g.Units.Remove(other)

	// A bulldozer off any pile cannot clear.
	dry := addBulldozer(g, 3, 10, 10)
	if g.CanClearRubble(dry) || g.StartClearRubble(dry) {
		t.Error("a bulldozer not on a pile cleared rubble")
	}

	// Stop cancels a clear in progress, removes the cones+tape, and leaves the pile.
	dozer := addBulldozer(g, 4, 3, 3)
	if !g.StartClearRubble(dozer) {
		t.Fatal("StartClearRubble returned false")
	}
	if markerCount(g) == 0 {
		t.Error("StartClearRubble laid no cones/tape markers")
	}
	g.StopClearRubble(dozer)
	if g.IsClearingRubble(dozer) || dozer.Order != game.OrderAwait {
		t.Errorf("StopClearRubble did not free the bulldozer: clearing=%v order=%v", g.IsClearingRubble(dozer), dozer.Order)
	}
	if markerCount(g) != 0 {
		t.Errorf("Stop left %d cones/tape markers; want 0", markerCount(g))
	}
	if _, ok := g.RubbleAt(3, 3); !ok {
		t.Error("Stop removed the pile; it must remain")
	}
}

// markerCount counts the live clear site cones+tape units.
func markerCount(g *Game) int {
	n := 0
	for _, u := range g.Units.Units() {
		switch u.Type {
		case assets.LrgTape, assets.SmlTape, assets.LrgCones, assets.SmlCones:
			n++
		}
	}
	return n
}

// TestClearSiteCoversFootprintAndBlocks: a bulldozer on any cell of a 2x2 building
// pile can clear it; it snaps to the anchor, lays cones+tape over the footprint
// that block other movers, and both clear when the job finishes.
func TestClearSiteCoversFootprintAndBlocks(t *testing.T) {
	g := newGame(t, 16, 16)
	bld := addBuilding(g, 1, 5, 5) // 2x2 anchored at (5,5), covers (5,5)..(6,6)
	g.DestroyUnit(bld)

	// Park the dozer on a NON anchor footprint cell (6,5); it can still clear.
	dozer := addBulldozer(g, 2, 6, 5)
	if !g.CanClearRubble(dozer) {
		t.Fatal("a bulldozer on a 2x2 pile's non-anchor cell cannot clear")
	}
	if !g.StartClearRubble(dozer) {
		t.Fatal("StartClearRubble returned false")
	}
	if dozer.GridX != 6 || dozer.GridY != 5 {
		t.Errorf("bulldozer should keep its own cell while clearing: at (%d,%d), want (6,5)", dozer.GridX, dozer.GridY)
	}
	// It keeps its own cell but is posed to GLIDE from there to the 2x2 centre (the
	// middle of the four cells), driven per frame by the gamescreen loop. Drive the
	// slide here to confirm it reaches the centre, starting from its current cell.
	const tile = 64
	wantCenter := 5*tile + tile // anchor*tile + footprint*tile/2 (footprint 2)
	if !dozer.Clearing || dozer.BuildToX != wantCenter || dozer.BuildToY != wantCenter {
		t.Errorf("clear pose not set toward the 2x2 centre: clearing=%v target=(%d,%d) want (%d,%d)", dozer.Clearing, dozer.BuildToX, dozer.BuildToY, wantCenter, wantCenter)
	}
	for i := 0; i < 16 && !dozer.AdvanceBuildSlide(8); i++ {
	}
	if dozer.X != wantCenter || dozer.Y != wantCenter {
		t.Errorf("bulldozer did not glide to the 2x2 centre: at (%d,%d), want (%d,%d)", dozer.X, dozer.Y, wantCenter, wantCenter)
	}
	if markerCount(g) != 2 {
		t.Fatalf("clear site laid %d markers, want 2 (cones+tape)", markerCount(g))
	}

	// The cones/tape block another mover across the whole 2x2 footprint - a cell
	// the bulldozer is NOT on (6,6) is blocked by the markers.
	enemy := addTank(g, 9, 1, 0, 0)
	if g.CanEnter(enemy, 6, 6) {
		t.Error("a clear-site footprint cell is not blocked; units should route around")
	}

	// Finishing the job removes the pile AND the markers, freeing the cell.
	for i := 0; i < clearLargeTurns; i++ {
		g.tickClearing(0)
	}
	if _, ok := g.RubbleAt(5, 5); ok {
		t.Error("pile not cleared after the full job")
	}
	if markerCount(g) != 0 {
		t.Errorf("%d cones/tape markers left after clearing; want 0", markerCount(g))
	}
	if !g.CanEnter(enemy, 6, 6) {
		t.Error("the cleared cell is still blocked")
	}
	// Faithful finish: the dozer is freed (no exit markers) with the leave slide
	// armed back toward its own cell; advancing the slide returns it to its cell.
	if g.IsBuildReady(dozer) {
		t.Error("a cleared bulldozer must NOT use the build-ready exit markers")
	}
	if !dozer.Leaving || dozer.Order != game.OrderAwait {
		t.Fatalf("clear did not arm the leave slide + free the dozer: leaving=%v order=%v", dozer.Leaving, dozer.Order)
	}
	for i := 0; i < 16 && !dozer.AdvanceBuildSlide(8); i++ {
	}
	if dozer.X != dozer.GridX*tile+tile/2 || dozer.Y != dozer.GridY*tile+tile/2 {
		t.Errorf("after the leave slide, dozer draw pos = (%d,%d), want its own cell centre (%d,%d)", dozer.X, dozer.Y, dozer.GridX*tile+tile/2, dozer.GridY*tile+tile/2)
	}
}
