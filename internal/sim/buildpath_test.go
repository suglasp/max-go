// buildpath_test.go : tests buildpath.go.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package sim

import (
	"testing"

	"github.com/suglasp/max-go/internal/access"
	"github.com/suglasp/max-go/internal/assets"
	"github.com/suglasp/max-go/internal/game"
	"github.com/suglasp/max-go/internal/units"
	"github.com/suglasp/max-go/internal/world"
)

// TestPathCellsStraightLine verifies that the longer axis selects a horizontal or vertical inclusive line, never a diagonal.
func TestPathCellsStraightLine(t *testing.T) {
	// Vertical: the up/down delta dominates.
	v := PathCells(5, 5, 6, 9) // dy=4 > dx=1 -> vertical at x=5
	wantV := [][2]int{{5, 5}, {5, 6}, {5, 7}, {5, 8}, {5, 9}}
	if !sameCells(v, wantV) {
		t.Errorf("vertical run = %v, want %v", v, wantV)
	}
	// Horizontal: the left/right delta dominates, and it runs the negative direction.
	h := PathCells(8, 3, 4, 4) // dx=4 > dy=1 -> horizontal at y=3, leftward
	wantH := [][2]int{{8, 3}, {7, 3}, {6, 3}, {5, 3}, {4, 3}}
	if !sameCells(h, wantH) {
		t.Errorf("horizontal run = %v, want %v", h, wantH)
	}
	// A single cell (no delta) is a one element run, which StartBuildPath rejects.
	if got := PathCells(2, 2, 2, 2); len(got) != 1 {
		t.Errorf("single-cell run len = %d, want 1", len(got))
	}
}

// TestBuildPathLaysRoadRun drives the full engineer Path build headlessly: an engineer
// lays a straight vertical run of roads, one per turn, building its own cell first then
// driving cell by cell to lay the rest. After enough turns every cell of the run carries
// a road and the path run has finished.
func TestBuildPathLaysRoadRun(t *testing.T) {
	g := newGame(t, 24, 24) // all land

	eng := game.NewUnitInfo(assets.Engineer, 0, 1, 0)
	eng.Flags = game.FlagMobileLandUnit | game.FlagConstructorUnit | game.FlagSelectable
	eng.Values = &game.UnitValues{}
	eng.Values.Set(game.AttribSpeed, 8)
	eng.Values.Set(game.AttribHits, 30) // a live engineer (tickBuildPaths drops a path whose builder is dead)
	eng.InitState()
	eng.Storage = 500 // plenty of material for the whole run
	g.Units.Deploy(eng, 5, 5)

	template := game.NewUnitInfo(assets.Road, 0, 2, 0)
	template.Flags = game.FlagGroundCover
	template.Values = &game.UnitValues{}
	template.Values.Set(game.AttribTurns, 1)
	template.Values.Set(game.AttribHits, 2)

	// A 4-cell vertical run: (5,5) (5,6) (5,7) (5,8).
	if !g.StartBuildPath(eng, template, 1, 5, 8) {
		t.Fatal("StartBuildPath rejected a 4-cell run")
	}

	// Drive turns until the run is laid (bounded against a runaway). BeginTeamTurn
	// already runs the economy + a turn of construction (AdvanceBuilds) and issues
	// the next move; ResolveMovement then plays the engineer's drive to the next cell.
	for turn := 0; turn < 60 && (g.BuildPathActive(eng) || g.ActiveOrders() > 0); turn++ {
		g.BeginTeamTurn(0)
		g.ResolveMovement()
	}

	if g.BuildPathActive(eng) {
		t.Fatal("path run never finished")
	}
	want := [][2]int{{5, 5}, {5, 6}, {5, 7}, {5, 8}}
	for _, c := range want {
		if !roadAt(g, c[0], c[1]) {
			t.Errorf("no road laid at (%d,%d)", c[0], c[1])
		}
	}
}

// TestBuildPathLaysBuildingRunNoExit verifies that an engineer continues through consecutive buildings without a manual exit between sites.
// Only the final building keeps the normal parked exit. C++ ported:
// units_manager.cpp - ORDER_STATE_UNIT_READY runs UnitsManager_BuildNext when a path is set.
func TestBuildPathLaysBuildingRunNoExit(t *testing.T) {
	g := newGame(t, 24, 24)

	eng := game.NewUnitInfo(assets.Engineer, 0, 1, 0)
	eng.Flags = game.FlagMobileLandUnit | game.FlagConstructorUnit | game.FlagSelectable
	eng.Values = &game.UnitValues{}
	eng.Values.Set(game.AttribSpeed, 8)
	eng.Values.Set(game.AttribHits, 30)
	eng.InitState()
	eng.Storage = 800 // plenty for the whole run, built from its own cargo
	g.Units.Deploy(eng, 5, 5)

	// A 1x1 storage unit (Storage Unit / ADump): a real stationary building, lays a slab.
	template := game.NewUnitInfo(assets.ADump, 0, 2, 0)
	template.Flags = game.FlagStationary | game.FlagRequiresSlab | game.FlagSelectable
	template.Values = &game.UnitValues{}
	template.Values.Set(game.AttribTurns, 1)
	template.Values.Set(game.AttribHits, 20)

	if !g.StartBuildPath(eng, template, 1, 5, 7) { // a 3-cell run: (5,5) (5,6) (5,7)
		t.Fatal("StartBuildPath rejected a 3-building run")
	}

	for turn := 0; turn < 80 && (g.BuildPathActive(eng) || g.ActiveOrders() > 0); turn++ {
		g.BeginTeamTurn(0)
		g.ResolveMovement()
	}

	if g.BuildPathActive(eng) {
		t.Fatal("a building path run never finished (likely stuck on a between-cell exit)")
	}
	built := 0
	for _, u := range g.Units.Units() {
		if u.Type == assets.ADump {
			built++
		}
	}
	if built != 3 {
		t.Errorf("path laid %d storage units, want 3 (the run must auto-continue past each one)", built)
	}
}

// TestBuildPathBuildsOnArrivalSameTurn verifies that an engineer starts building on the same turn it reaches the next path cell, matching vanilla move-and-build timing.
func TestBuildPathBuildsOnArrivalSameTurn(t *testing.T) {
	g := newGame(t, 24, 24) // all land

	eng := game.NewUnitInfo(assets.Engineer, 0, 1, 0)
	eng.Flags = game.FlagMobileLandUnit | game.FlagConstructorUnit | game.FlagSelectable
	eng.Values = &game.UnitValues{}
	eng.Values.Set(game.AttribSpeed, 8)
	eng.Values.Set(game.AttribHits, 30)
	eng.InitState()
	eng.Storage = 500
	g.Units.Deploy(eng, 5, 5)

	template := game.NewUnitInfo(assets.Road, 0, 2, 0)
	template.Flags = game.FlagGroundCover
	template.Values = &game.UnitValues{}
	template.Values.Set(game.AttribTurns, 1)
	template.Values.Set(game.AttribHits, 2)

	if !g.StartBuildPath(eng, template, 1, 5, 9) { // a 5-cell vertical run
		t.Fatal("StartBuildPath rejected the run")
	}

	for turn := 0; turn < 60 && g.BuildPathActive(eng); turn++ {
		g.BeginTeamTurn(0)
		g.ResolveMovement() // plays the engineer's drive; resumePathBuildOnArrival builds on arrival
		// After the turn settles, the engineer must never be standing FREE on an
		// unfinished path cell with no build started - that is the wasted idle turn.
		if g.BuildPathActive(eng) && !g.isMoving(eng) && !g.IsBuildReady(eng) && g.BuildOrderFor(eng) == nil {
			t.Fatalf("turn %d: engineer idle on a path cell with no build started (the build deferred a turn)", turn)
		}
	}
	if g.BuildPathActive(eng) {
		t.Fatal("path run never finished")
	}
	for y := 5; y <= 9; y++ {
		if !roadAt(g, 5, y) {
			t.Errorf("no road laid at (5,%d)", y)
		}
	}
}

// TestBuildPathStopsWhenOutOfMaterial confirms a Path run abandons when the engineer
// runs out of material: it lays what it can afford, the run is dropped (the in game
// screen's stalled message tells the player), and it does not hang forever.
func TestBuildPathStopsWhenOutOfMaterial(t *testing.T) {
	g := newGame(t, 24, 24)

	eng := game.NewUnitInfo(assets.Engineer, 0, 1, 0)
	eng.Flags = game.FlagMobileLandUnit | game.FlagConstructorUnit | game.FlagSelectable
	eng.Values = &game.UnitValues{}
	eng.Values.Set(game.AttribSpeed, 8)
	eng.Values.Set(game.AttribHits, 30) // a live engineer (tickBuildPaths drops a path whose builder is dead)
	eng.InitState()
	eng.Storage = 6 // only enough for the first cell or two
	g.Units.Deploy(eng, 5, 5)

	template := game.NewUnitInfo(assets.Road, 0, 2, 0)
	template.Flags = game.FlagGroundCover
	template.Values = &game.UnitValues{}
	template.Values.Set(game.AttribTurns, 1)
	template.Values.Set(game.AttribHits, 2)

	if !g.StartBuildPath(eng, template, 1, 5, 10) { // a long 6-cell run it cannot afford
		t.Fatal("StartBuildPath rejected the run")
	}

	for turn := 0; turn < 60 && (g.BuildPathActive(eng) || g.ActiveOrders() > 0); turn++ {
		g.BeginTeamTurn(0)
		g.AdvanceBuilds(0)
		g.ResolveMovement()
	}

	if g.BuildPathActive(eng) {
		t.Error("an unaffordable path run never stopped (hung)")
	}
	// It laid at least the anchor but not the whole run.
	laid := 0
	for y := 5; y <= 10; y++ {
		if roadAt(g, 5, y) {
			laid++
		}
	}
	if laid == 0 {
		t.Error("the run laid no roads at all")
	}
	if laid == 6 {
		t.Error("the run laid the whole line despite insufficient material")
	}
}

// runPathTurns plays turns until the run and its orders are done, and reports whether
// any turn named eng among the path builders stopped short of material.
func runPathTurns(g *Game, eng *game.UnitInfo) (short bool) {
	for turn := 0; turn < 40 && (g.BuildPathActive(eng) || g.BuildOrderFor(eng) != nil); turn++ {
		g.BeginTeamTurn(0)
		for _, b := range g.LastPathShortBuilders() {
			short = short || b == eng
		}
		g.ResolveMovement()
	}
	return short
}

// TestBuildPathStopsParkedWhenNextCellUnaffordable checks a building run whose storage pays
// for one cell only: the run ends before the second cell, the engineer stays parked on the
// finished building with its exit markers, and no second site is started.
func TestBuildPathStopsParkedWhenNextCellUnaffordable(t *testing.T) {
	g := newGame(t, 24, 24)
	eng := addConstructor(g, assets.Engineer, 0, 5, 5, 8, 30, 6) // one ADump costs 2 turns x 2 raw = 4

	template := game.NewUnitInfo(assets.ADump, 0, 2, 0)
	template.Flags = game.FlagStationary | game.FlagRequiresSlab | game.FlagSelectable
	template.Values = &game.UnitValues{}
	template.Values.Set(game.AttribTurns, 2)
	template.Values.Set(game.AttribHits, 20)

	if !g.StartBuildPath(eng, template, 1, 5, 7) {
		t.Fatal("StartBuildPath rejected the run")
	}
	short := runPathTurns(g, eng)

	if g.BuildPathActive(eng) {
		t.Fatal("the run did not stop before the unaffordable cell")
	}
	if !short {
		t.Error("LastPathShortBuilders never named the engineer (no warning would show)")
	}
	if g.BuildOrderFor(eng) != nil || len(g.BuildOrders()) > 0 {
		t.Error("a second cell was started although the storage could not pay for it")
	}
	if !g.IsBuildReady(eng) {
		t.Error("the engineer was released; it must stay parked on the finished building with its exit markers")
	}
	built := 0
	for _, u := range g.Units.Units() {
		if u.Type == assets.ADump {
			built++
		}
	}
	if built != 1 {
		t.Errorf("laid %d storage units, want 1", built)
	}
}

// TestBuildPathRoadStopsInPlaceWhenUnaffordable checks a road run whose storage pays for one
// cell only: the engineer stops on the road it laid, with no half built cell left behind.
func TestBuildPathRoadStopsInPlaceWhenUnaffordable(t *testing.T) {
	g := newGame(t, 24, 24)
	eng := addConstructor(g, assets.Engineer, 0, 5, 5, 8, 30, 3) // one road costs 1 turn x 2 raw = 2

	template := game.NewUnitInfo(assets.Road, 0, 2, 0)
	template.Flags = game.FlagGroundCover
	template.Values = &game.UnitValues{}
	template.Values.Set(game.AttribTurns, 1)
	template.Values.Set(game.AttribHits, 2)

	if !g.StartBuildPath(eng, template, 1, 5, 8) {
		t.Fatal("StartBuildPath rejected the run")
	}
	short := runPathTurns(g, eng)

	if g.BuildPathActive(eng) {
		t.Fatal("the run did not stop before the unaffordable cell")
	}
	if !short {
		t.Error("LastPathShortBuilders never named the engineer (no warning would show)")
	}
	if len(g.BuildOrders()) > 0 {
		t.Error("a half built road cell was left behind")
	}
	if eng.GridX != 5 || eng.GridY != 5 {
		t.Errorf("engineer at (%d,%d), want it on its last road at (5,5)", eng.GridX, eng.GridY)
	}
	if !roadAt(g, 5, 5) || roadAt(g, 5, 6) {
		t.Error("want exactly the anchor road laid")
	}
}

// TestPlayerMoveCancelsBuildPath verifies that a player-issued move cancels the engineer's path, while the path's own orderMoveTo steps preserve it.
func TestPlayerMoveCancelsBuildPath(t *testing.T) {
	g := newGame(t, 24, 24)

	eng := game.NewUnitInfo(assets.Engineer, 0, 1, 0)
	eng.Flags = game.FlagMobileLandUnit | game.FlagConstructorUnit | game.FlagSelectable
	eng.Values = &game.UnitValues{}
	eng.Values.Set(game.AttribSpeed, 8)
	eng.Values.Set(game.AttribHits, 30)
	eng.InitState()
	eng.Storage = 500
	g.Units.Deploy(eng, 5, 5)

	template := game.NewUnitInfo(assets.Road, 0, 2, 0)
	template.Flags = game.FlagGroundCover
	template.Values = &game.UnitValues{}
	template.Values.Set(game.AttribTurns, 1)
	template.Values.Set(game.AttribHits, 2)

	if !g.StartBuildPath(eng, template, 1, 5, 10) {
		t.Fatal("StartBuildPath rejected the run")
	}
	if !g.BuildPathActive(eng) {
		t.Fatal("path not active after StartBuildPath")
	}
	// The player orders the engineer elsewhere: the Path must be dropped at once.
	g.OrderMove(eng, 15, 15)
	if g.BuildPathActive(eng) {
		t.Error("a player move did not cancel the engineer's Path build (it would loop)")
	}
}

// bridgeBandSurf is land with a central water band (rows 7-9) flanked by coast
// (rows 6,10), so a vertical run from an inland engineer crosses cells valid for
// a bridge (water/coast) while its own cell stays land.
type bridgeBandSurf struct{}

func (bridgeBandSurf) SurfaceAt(x, y int) uint8 {
	switch {
	case y >= 7 && y <= 9:
		return world.SurfaceWater
	case y == 6 || y == 10:
		return world.SurfaceCoast
	default:
		return world.SurfaceLand
	}
}

// TestBuildPathBridgeFromInlandDrivesNotTeleports verifies that an inland engineer drives to the shoreline before starting a bridge path.
// LandTypeForUnit(Bridge) accepts Water|Coast, so StartBuildPath must route to the first valid cell rather than starting a remote build pose.
func TestBuildPathBridgeFromInlandDrivesNotTeleports(t *testing.T) {
	mgr := units.NewManager()
	grid := &access.Grid{W: 24, H: 24, Surf: bridgeBandSurf{}, Units: mgr.Cells}
	p := &game.StartParams{}
	p.Teams[0].Type = game.TeamPlayer
	g := NewWithGrid(game.NewGameState(p), mgr, grid, 24, 24)

	eng := game.NewUnitInfo(assets.Engineer, 0, 1, 0)
	eng.Flags = game.FlagMobileLandUnit | game.FlagConstructorUnit | game.FlagSelectable
	eng.Values = &game.UnitValues{}
	eng.Values.Set(game.AttribSpeed, 8)
	eng.Values.Set(game.AttribHits, 30)
	eng.InitState()
	eng.Storage = 200
	g.Units.Deploy(eng, 5, 5) // inland on land

	bridge := game.NewUnitInfo(assets.Bridge, 0, 2, 0)
	bridge.Flags = game.FlagStationary
	bridge.Values = &game.UnitValues{}
	bridge.Values.Set(game.AttribTurns, 1)
	bridge.Values.Set(game.AttribHits, 2)

	// A vertical run from (5,5) down across the water band: the buildable cells start at
	// the shoreline (5,6), not the engineer's own land cell (5,5).
	if !g.StartBuildPath(eng, bridge, 1, 5, 12) {
		t.Fatal("StartBuildPath rejected the bridge run")
	}
	if g.BuildOrderFor(eng) != nil {
		t.Error("bridge path force-started a build on a cell the engineer was not on (the inland teleport bug)")
	}
	if eng.Building {
		t.Error("engineer entered a build pose at a remote cell it was not standing on")
	}
	if eng.GridX != 5 || eng.GridY != 5 {
		t.Errorf("engineer position jumped to (%d,%d); it must stay put until it drives there", eng.GridX, eng.GridY)
	}
}

// TestBuildPathWaitsForBlockingUnit verifies that an engineer waits for an occupied path cell and builds it after the blocking unit moves, matching vanilla behavior.
func TestBuildPathWaitsForBlockingUnit(t *testing.T) {
	g := newGame(t, 24, 24) // all land

	eng := game.NewUnitInfo(assets.Engineer, 0, 1, 0)
	eng.Flags = game.FlagMobileLandUnit | game.FlagConstructorUnit | game.FlagSelectable
	eng.Values = &game.UnitValues{}
	eng.Values.Set(game.AttribSpeed, 8)
	eng.Values.Set(game.AttribHits, 30)
	eng.InitState()
	eng.Storage = 500
	g.Units.Deploy(eng, 5, 5)

	template := game.NewUnitInfo(assets.Road, 0, 2, 0)
	template.Flags = game.FlagGroundCover
	template.Values = &game.UnitValues{}
	template.Values.Set(game.AttribTurns, 1)
	template.Values.Set(game.AttribHits, 2)

	if !g.StartBuildPath(eng, template, 1, 5, 8) { // run (5,5) (5,6) (5,7) (5,8)
		t.Fatal("StartBuildPath rejected the run")
	}

	// A unit drives onto the third cell of the run AFTER the path is armed, blocking it
	// (the run was planned clean, matching a unit walking into the path mid build).
	blocker := game.NewUnitInfo(assets.Tank, 0, 1, 0)
	blocker.Flags = game.FlagMobileLandUnit | game.FlagSelectable
	blocker.Values = &game.UnitValues{}
	blocker.Values.Set(game.AttribHits, 30)
	blocker.InitState()
	blocker.Order = game.OrderSentry // on guard: it blocks the cell (not idle)
	g.Units.Deploy(blocker, 5, 7)

	// The engineer lays up to the blocker, then holds: it must NOT skip past the cell.
	for turn := 0; turn < 20 && g.BuildPathActive(eng); turn++ {
		g.BeginTeamTurn(0)
		g.ResolveMovement()
	}
	if !roadAt(g, 5, 5) || !roadAt(g, 5, 6) {
		t.Error("engineer did not lay the cells before the blocker")
	}
	if roadAt(g, 5, 7) {
		t.Error("road laid on the cell a unit was standing on (5,7)")
	}
	if roadAt(g, 5, 8) {
		t.Error("engineer skipped the blocked cell and built past it at (5,8)")
	}
	if !g.BuildPathActive(eng) {
		t.Fatal("path build ended while a unit still blocked it; it should wait in place")
	}

	// The unit drives away: the engineer now builds the spot it vacated and finishes.
	g.Units.Move(blocker, 10, 10)
	for turn := 0; turn < 20 && g.BuildPathActive(eng); turn++ {
		g.BeginTeamTurn(0)
		g.ResolveMovement()
	}
	if g.BuildPathActive(eng) {
		t.Fatal("path never finished after the blocker moved away")
	}
	if !roadAt(g, 5, 7) {
		t.Error("engineer did not build the spot the unit vacated at (5,7)")
	}
	if !roadAt(g, 5, 8) {
		t.Error("engineer did not finish the run at (5,8)")
	}
}

// TestBuildPathSkipsExistingRoad verifies that a road path crossing existing road ground cover does not build the crossing cell twice.
func TestBuildPathSkipsExistingRoad(t *testing.T) {
	g := newGame(t, 24, 24) // all land

	eng := game.NewUnitInfo(assets.Engineer, 0, 1, 0)
	eng.Flags = game.FlagMobileLandUnit | game.FlagConstructorUnit | game.FlagSelectable
	eng.Values = &game.UnitValues{}
	eng.Values.Set(game.AttribSpeed, 8)
	eng.Values.Set(game.AttribHits, 30)
	eng.InitState()
	eng.Storage = 500
	g.Units.Deploy(eng, 5, 5)

	template := game.NewUnitInfo(assets.Road, 0, 2, 0)
	template.Flags = game.FlagGroundCover
	template.Values = &game.UnitValues{}
	template.Values.Set(game.AttribTurns, 1)
	template.Values.Set(game.AttribHits, 2)

	// An existing road already crosses the run at (5,7).
	existing := game.NewUnitInfo(assets.Road, 0, 0, 0)
	existing.Flags = game.FlagGroundCover
	existing.Values = &game.UnitValues{}
	existing.Values.Set(game.AttribHits, 2)
	existing.InitState()
	g.Units.Deploy(existing, 5, 7)

	if !g.StartBuildPath(eng, template, 1, 5, 8) { // run (5,5) (5,6) (5,7) (5,8)
		t.Fatal("StartBuildPath rejected the run")
	}

	for turn := 0; turn < 40 && (g.BuildPathActive(eng) || g.ActiveOrders() > 0); turn++ {
		g.BeginTeamTurn(0)
		g.ResolveMovement()
	}
	if g.BuildPathActive(eng) {
		t.Fatal("path run never finished")
	}

	// The crossing tile keeps exactly one road: no duplicate laid on top of the existing one.
	roads := 0
	for _, u := range g.Units.Units() {
		if u.Type == assets.Road && u.GridX == 5 && u.GridY == 7 {
			roads++
		}
	}
	if roads != 1 {
		t.Errorf("crossing tile (5,7) has %d roads, want 1 (no double build)", roads)
	}
	// The rest of the run is laid.
	for _, c := range [][2]int{{5, 5}, {5, 6}, {5, 8}} {
		if !roadAt(g, c[0], c[1]) {
			t.Errorf("no road laid at (%d,%d)", c[0], c[1])
		}
	}
}

// TestBuildPathKeepsCellUnitParkedOnAtArm verifies that a mobile unit occupying a path cell does not remove that cell when the path is armed.
// The engineer waits for the OrderAwait unit to move, then builds on the vacated cell.
func TestBuildPathKeepsCellUnitParkedOnAtArm(t *testing.T) {
	g := newGame(t, 24, 24) // all land

	eng := game.NewUnitInfo(assets.Engineer, 0, 1, 0)
	eng.Flags = game.FlagMobileLandUnit | game.FlagConstructorUnit | game.FlagSelectable
	eng.Values = &game.UnitValues{}
	eng.Values.Set(game.AttribSpeed, 8)
	eng.Values.Set(game.AttribHits, 30)
	eng.InitState()
	eng.Storage = 500
	g.Units.Deploy(eng, 5, 5)

	// A unit that has just finished moving remains parked on the third cell with OrderAwait rather than OrderIdle.
	blocker := game.NewUnitInfo(assets.Tank, 0, 1, 0)
	blocker.Flags = game.FlagMobileLandUnit | game.FlagSelectable
	blocker.Values = &game.UnitValues{}
	blocker.Values.Set(game.AttribHits, 30)
	blocker.InitState()
	blocker.Order = game.OrderAwait // just settled from a move: non idle, but only a transient obstacle
	g.Units.Deploy(blocker, 5, 7)

	template := game.NewUnitInfo(assets.Road, 0, 2, 0)
	template.Flags = game.FlagGroundCover
	template.Values = &game.UnitValues{}
	template.Values.Set(game.AttribTurns, 1)
	template.Values.Set(game.AttribHits, 2)

	if !g.StartBuildPath(eng, template, 1, 5, 8) { // run (5,5) (5,6) (5,7) (5,8)
		t.Fatal("StartBuildPath rejected the run")
	}
	// The cell the unit is parked on must stay in the run (not filtered out at arm time).
	bp := g.buildPaths[eng]
	if bp == nil {
		t.Fatal("no path armed")
	}
	found := false
	for _, c := range bp.Cells {
		if c[0] == 5 && c[1] == 7 {
			found = true
		}
	}
	if !found {
		t.Fatal("the parked unit's cell (5,7) was dropped from the run at arm time (the skip bug)")
	}

	// Drive up to the blocker: it holds, does not skip past.
	for turn := 0; turn < 20 && g.BuildPathActive(eng); turn++ {
		g.BeginTeamTurn(0)
		g.ResolveMovement()
		if roadAt(g, 5, 8) && !roadAt(g, 5, 7) {
			t.Fatal("engineer skipped the blocked cell (5,7) and built past it at (5,8)")
		}
	}

	// The player moves the unit off: the engineer builds the spot it vacated and finishes.
	g.Units.Move(blocker, 10, 10)
	for turn := 0; turn < 20 && g.BuildPathActive(eng); turn++ {
		g.BeginTeamTurn(0)
		g.ResolveMovement()
	}
	if g.BuildPathActive(eng) {
		t.Fatal("path never finished after the blocker moved away")
	}
	for _, c := range [][2]int{{5, 5}, {5, 6}, {5, 7}, {5, 8}} {
		if !roadAt(g, c[0], c[1]) {
			t.Errorf("no road laid at (%d,%d) (the tile a unit was parked on must still be built)", c[0], c[1])
		}
	}
}

// sameCells reports whether two cell runs are identical in order.
func sameCells(a, b [][2]int) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// roadAt reports whether a road ground cover unit sits on (x, y).
func roadAt(g *Game, x, y int) bool {
	for _, u := range g.Units.Units() {
		if u.Type == assets.Road && u.GridX == x && u.GridY == y {
			return true
		}
	}
	return false
}

// TestBuildPathBridgeSpansOpenWater pins the FULL span, which the backlog long claimed was
// impossible ("a bridge path lays what is reachable from the shore and drops the rest",
// because a built bridge cell supposedly did not read as walkable land). It works: the
// access layer's ModifiedSurface reads a BRIDGE / WTRPLTFM cell as LAND, and the engineer is
// amphibious, so it drives onto each finished segment and lays the next one, right across
// the water and out the far side.
//
// C++ ported : access.cpp - Access_GetModifiedSurfaceType + units_manager.cpp - UnitsManager_BuildNext.
func TestBuildPathBridgeSpansOpenWater(t *testing.T) {
	mgr := units.NewManager()
	grid := &access.Grid{W: 24, H: 24, Surf: bridgeBandSurf{}, Units: mgr.Cells}
	p := &game.StartParams{}
	p.Teams[0].Type = game.TeamPlayer
	g := NewWithGrid(game.NewGameState(p), mgr, grid, 24, 24)

	eng := game.NewUnitInfo(assets.Engineer, 0, 1, 0)
	eng.Flags = game.FlagMobileLandUnit | game.FlagConstructorUnit | game.FlagSelectable
	eng.Values = &game.UnitValues{}
	eng.Values.Set(game.AttribSpeed, 8)
	eng.Values.Set(game.AttribHits, 30)
	eng.InitState()
	eng.Storage = 200
	g.Units.Deploy(eng, 5, 5) // inland, on land

	bridge := game.NewUnitInfo(assets.Bridge, 0, 2, 0)
	bridge.Flags = game.FlagStationary
	bridge.Values = &game.UnitValues{}
	bridge.Values.Set(game.AttribTurns, 1)
	bridge.Values.Set(game.AttribHits, 2)

	// Run the path from the inland cell down across the water band (coast 6, water 7-9,
	// coast 10) to the far shore.
	if !g.StartBuildPath(eng, bridge, 1, 5, 12) {
		t.Fatal("StartBuildPath rejected the bridge run")
	}

	for turn := 0; turn < 25 && g.BuildPathActive(eng); turn++ {
		g.BeginTeamTurn(0)
		g.ResolveMovement()
		g.EndTeamTurn(0)
		g.AdvanceTurn()
	}

	// Every water and coast cell of the run must carry a bridge - the mid water ones above all.
	for y := 6; y <= 10; y++ {
		if !g.cellHasStructureType(5, y, assets.Bridge) {
			t.Errorf("no bridge at (5,%d): the span dropped a cell the engineer could not reach", y)
		}
	}
}
