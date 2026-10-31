// sfx_test.go : tests sfx.go.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package gamescreen

import (
	"testing"

	"github.com/suglasp/max-go/internal/access"
	"github.com/suglasp/max-go/internal/assets"
	"github.com/suglasp/max-go/internal/drawmap"
	"github.com/suglasp/max-go/internal/game"
	"github.com/suglasp/max-go/internal/sim"
	"github.com/suglasp/max-go/internal/units"
	"github.com/suglasp/max-go/internal/world"
)

// TestMovementSfxOnWaterPlatform verifies that an
// amphibious unit standing on a Water Platform (or bridge) over water is "on the
// platform" - it must use its LAND drive sound (and land sprite), not the water
// clip. The fix reads the MODIFIED surface (platform/bridge -> land) instead of
// the raw tile; here a Personnel Carrier on a platform cell stays on the land clip
// while the same carrier on bare open water uses the water clip.
func TestMovementSfxOnWaterPlatform(t *testing.T) {
	// 2x1: open water at x=0 and x=1.
	m := &world.Map{Surface: []uint8{world.SurfaceWater, world.SurfaceWater}}
	m.W, m.H = 2, 1
	mgr := units.NewManager()
	grid := &access.Grid{W: 2, H: 1, Surf: m, Units: mgr.Cells}
	p := &game.StartParams{}
	p.Teams[0].Type = game.TeamPlayer
	g := sim.NewWithGrid(game.NewGameState(p), mgr, grid, 2, 1)
	s := &Screen{game: g, wmap: m}

	// A Water Platform on the x=0 cell makes it read as land.
	platform := game.NewUnitInfo(assets.WtrPltfm, 0, 1, 0)
	platform.Flags = game.FlagStationary
	mgr.Deploy(platform, 0, 0)

	apc := game.NewUnitInfo(assets.ClnTrans, 0, 2, 0)

	// On the platform cell the carrier uses its land drive clip.
	apc.GridX, apc.GridY = 0, 0
	if got := s.movementSfx(apc, game.SfxDrive); got != game.SfxDrive {
		t.Errorf("carrier on a water platform = %v; want SfxDrive (it is on the platform)", got)
	}
	// On bare open water (no platform) it uses the water clip.
	apc.GridX, apc.GridY = 1, 0
	if got := s.movementSfx(apc, game.SfxDrive); got != game.SfxWaterDrive {
		t.Errorf("carrier on open water = %v; want SfxWaterDrive", got)
	}
}

// TestAudibleMoverFogGate verifies the movement sound visibility gate: your own units
// are always audible, a VISIBLE enemy mover is audible, and an enemy hidden in the
// fog is silent (so you no longer hear enemy units drive across the dark map).
func TestAudibleMoverFogGate(t *testing.T) {
	s, g := newClickScreen(t)
	s.fogTeam = 0

	own := mkUnit(g, assets.Tank, 1, 5, 5, game.FlagMobileLandUnit|game.FlagSelectable)
	if !s.audibleMover(own) {
		t.Error("own unit must always be audible")
	}

	// A player scanner reveals an enemy beside it; a distant enemy stays fogged.
	scout := mkUnit(g, assets.Scout, 2, 8, 8, game.FlagMobileLandUnit|game.FlagSelectable)
	scout.Values.Set(game.AttribScan, 4)
	seen := game.NewUnitInfo(assets.Tank, 1, 3, 0)
	seen.Flags = game.FlagMobileLandUnit | game.FlagSelectable
	seen.Values = &game.UnitValues{}
	seen.Order = game.OrderAwait
	g.Units.Deploy(seen, 9, 8)
	fogged := game.NewUnitInfo(assets.Tank, 1, 4, 0)
	fogged.Flags = game.FlagMobileLandUnit | game.FlagSelectable
	fogged.Values = &game.UnitValues{}
	fogged.Order = game.OrderAwait
	g.Units.Deploy(fogged, 1, 1)
	g.Vis.Rebuild(g.Units.Units())

	if !s.audibleMover(seen) {
		t.Error("a visible enemy mover must be audible")
	}
	if s.audibleMover(fogged) {
		t.Error("a fogged enemy mover must be silent")
	}
}

// TestSelectedBuilderWorkingSoundIsSelectionTied verifies a build sound
// build/clear audio ownership: the "working" loop is the selected unit's sound
// (game_manager.cpp + sound_manager.cpp), NOT a global per unit loop. An
// unselected building unit is silent; selecting an actively building one starts the loop;
// deselecting stops it. A global per-unit loop would play unselected builders and fail to stop on a selection change.
func TestSelectedBuilderWorkingSoundIsSelectionTied(t *testing.T) {
	s, g := newClickScreen(t)
	s.fogTeam = 0
	var started, stopped int
	s.sound.sfxHeldStartFn = func(primary, fallback string, pan, vol float64, loop bool) int { started++; return 55 }
	s.sound.sfxHeldStopFn = func(id int) { stopped++ }

	con := mkUnit(g, assets.Constrct, 1, 6, 6, game.FlagMobileLandUnit|game.FlagConstructorUnit|game.FlagSelectable)
	con.Building = true
	con.State = game.OrderStateBuildInProgress

	// Not selected -> silent (no global working loop).
	s.updateAmbientSfx(map[*game.UnitInfo]bool{})
	if started != 0 {
		t.Fatalf("an unselected building unit must be silent, got %d starts", started)
	}

	// Selected + actively building -> the working loop starts.
	s.selected = con
	s.updateAmbientSfx(map[*game.UnitInfo]bool{})
	if started != 1 {
		t.Fatalf("a selected actively-building unit should start the working loop, got %d", started)
	}

	// Deselecting stops the builder's working loop.
	s.selected = nil
	s.updateAmbientSfx(map[*game.UnitInfo]bool{})
	if stopped != 1 {
		t.Errorf("deselecting should stop the builder's working loop, got %d stops", stopped)
	}
}

// TestSelectedParkedBuilderPlaysNoWorkingSound: a builder that FINISHED its structure
// stays in its build pose (Building=true) but parked build READY (State BuildingReady) -
// its work is DONE, so even when selected it must NOT play the BUILDING working loop (the
// held loop path). As a mobile unit it may hum its plain IDLE sound (the intro loop path),
// which is faithful: the original picks SFX_TYPE_IDLE for a UNIT_READY builder, not SFX_TYPE_BUILDING.
func TestSelectedParkedBuilderPlaysNoWorkingSound(t *testing.T) {
	s, g := newClickScreen(t)
	s.fogTeam = 0
	var heldStarted int // the build/clear WORKING loop uses the held loop start
	s.sound.sfxHeldStartFn = func(primary, fallback string, pan, vol float64, loop bool) int { heldStarted++; return 55 }
	s.sound.sfxIntroLoopStartFn = func(primary, fallback string, pan, vol float64) int { return 56 } // the idle hum
	s.sound.sfxHeldStopFn = func(id int) {}

	con := mkUnit(g, assets.Constrct, 1, 6, 6, game.FlagMobileLandUnit|game.FlagConstructorUnit|game.FlagSelectable)
	con.Building = true
	con.State = game.OrderStateBuildingReady
	s.selected = con
	s.updateAmbientSfx(map[*game.UnitInfo]bool{})
	if heldStarted != 0 {
		t.Errorf("a parked build ready builder must NOT play the held working sound, got %d", heldStarted)
	}
}

// surfaceMapForSfx builds a tiny 3x1 world map: water at x=0, coast at x=1, land
// at x=2. movementSfx reads SurfaceAt(GridX, GridY) off it.
func surfaceMapForSfx() *world.Map {
	m := &world.Map{Surface: []uint8{world.SurfaceWater, world.SurfaceCoast, world.SurfaceLand}}
	m.W, m.H = 3, 1
	return m
}

// TestMovementSfxSurveyorWater verifies that the
// Surveyor crosses open water but kept playing its LAND drive clip. It
// carries its own SURVY2/6/8 water set, so on a FULL water cell its idle/drive/stop
// sounds must remap to the water variants; on coast/shore and land they stay the
// land clip (matching the sprite). A pure land unit on the same water cell never
// remaps (it has no water clip).
func TestMovementSfxSurveyorWater(t *testing.T) {
	s := &Screen{wmap: surfaceMapForSfx()}

	surveyor := game.NewUnitInfo(assets.Surveyor, 0, 1, 0)
	cases := []struct {
		gridX int
		land  game.SfxType
		want  game.SfxType
	}{
		{0, game.SfxDrive, game.SfxWaterDrive}, // on water: remap to the splash variant
		{0, game.SfxIdle, game.SfxWaterIdle},
		{0, game.SfxStop, game.SfxWaterStop},
		{1, game.SfxDrive, game.SfxDrive}, // coast/shore keeps the land clip
		{2, game.SfxDrive, game.SfxDrive}, // land keeps the land clip
	}
	for _, tc := range cases {
		surveyor.GridX, surveyor.GridY = tc.gridX, 0
		if got := s.movementSfx(surveyor, tc.land); got != tc.want {
			t.Errorf("movementSfx(Surveyor@x=%d, %v) = %v; want %v", tc.gridX, tc.land, got, tc.want)
		}
	}

	// A pure land Tank afloat on the same water cell must not remap: it has no
	// water clip, so it keeps its land drive sound.
	tank := game.NewUnitInfo(assets.Tank, 0, 2, 0)
	tank.GridX, tank.GridY = 0, 0
	if got := s.movementSfx(tank, game.SfxDrive); got != game.SfxDrive {
		t.Errorf("movementSfx(Tank@water, SfxDrive) = %v; want SfxDrive (no water clip)", got)
	}

	// The Personnel Carrier follows the same rule as the Surveyor: water clip on
	// open water, plain clip on coast and land.
	apc := game.NewUnitInfo(assets.ClnTrans, 0, 3, 0)
	apcCases := []struct {
		gridX int
		want  game.SfxType
	}{
		{0, game.SfxWaterDrive}, // open water: water clip (APC6)
		{1, game.SfxDrive},      // coast: plain clip (not open water)
		{2, game.SfxDrive},      // land: plain clip (APC5)
	}
	for _, tc := range apcCases {
		apc.GridX, apc.GridY = tc.gridX, 0
		if got := s.movementSfx(apc, game.SfxDrive); got != tc.want {
			t.Errorf("movementSfx(ClnTrans@x=%d, SfxDrive) = %v; want %v", tc.gridX, got, tc.want)
		}
	}
}

// TestDriveSfxReswapsOnWaterCrossing verifies the second root cause:
// the held drive clip is chosen once at move start, so a Personnel Carrier that
// started on one surface kept that clip across a land<->water boundary.
// reswapDriveSfxOnSurfaceChange must stop the old handle and restart with the
// matching variant the moment the unit's cell flips, and not churn while it stays
// on one surface. The Personnel Carrier starts on land with its plain clip (APC5)
// and must switch to its water clip (APC6) when it reaches open water - the reported
// bug was that it kept the land clip the whole way.
func TestDriveSfxReswapsOnWaterCrossing(t *testing.T) {
	// 3x1: land at x=0,1; open water at x=2.
	m := &world.Map{Surface: []uint8{world.SurfaceLand, world.SurfaceLand, world.SurfaceWater}}
	m.W, m.H = 3, 1

	var started []string      // primary tag of every held sound start, in order
	stopped := map[int]bool{} // handle ids that were stopped
	nextID := 0
	s := &Screen{wmap: m, r: drawmap.New(m)}
	s.sound.sfxHeldStartFn = func(primary, fallback string, pan, vol float64, loop bool) int {
		nextID++
		started = append(started, primary)
		return nextID
	}
	s.sound.sfxHeldStopFn = func(id int) { stopped[id] = true }
	s.sound.driveSfx = map[*game.UnitInfo]int{}
	s.sound.driveSfxType = map[*game.UnitInfo]game.SfxType{}

	apc := game.NewUnitInfo(assets.ClnTrans, 0, 1, 0)
	apc.GridX, apc.GridY = 0, 0 // start on land -> plain drive clip APC5
	s.startDriveSfx(apc)
	if len(started) != 1 || started[0] != "APC5" {
		t.Fatalf("Personnel Carrier on land = %v; want [APC5]", started)
	}
	landID := s.sound.driveSfx[apc]

	// Drives onto the open water cell: the land handle is stopped and the water clip
	// (APC6) starts.
	apc.GridX = 2
	s.reswapDriveSfxOnSurfaceChange(map[*game.UnitInfo]bool{apc: true})
	if !stopped[landID] {
		t.Error("the land drive handle should be stopped when the unit reaches water")
	}
	if len(started) != 2 || started[1] != "APC6" {
		t.Fatalf("Personnel Carrier on water = %v; want the last to be APC6 (water clip)", started)
	}
	if s.sound.driveSfxType[apc] != game.SfxWaterDrive {
		t.Errorf("driveSfxType on water = %v; want SfxWaterDrive", s.sound.driveSfxType[apc])
	}

	// Staying on water must not restart the clip every frame.
	before := len(started)
	s.reswapDriveSfxOnSurfaceChange(map[*game.UnitInfo]bool{apc: true})
	if len(started) != before {
		t.Errorf("remaining on water restarted the clip (starts %d -> %d)", before, len(started))
	}
}

func TestAmbientMobileIdleUsesIntroLoop(t *testing.T) {
	m := &world.Map{Surface: make([]uint8, 16)}
	for i := range m.Surface {
		m.Surface[i] = world.SurfaceLand
	}
	m.W, m.H = 4, 4
	s := &Screen{wmap: m, r: drawmap.New(m)}
	eng := game.NewUnitInfo(assets.Engineer, 0, 1, 0)
	eng.Flags = game.FlagMobileLandUnit
	eng.GridX, eng.GridY = 1, 1
	s.selected = eng

	var introPrimary string
	var heldCalled bool
	s.sound.sfxIntroLoopStartFn = func(primary, fallback string, pan, vol float64) int {
		introPrimary = primary
		return 11
	}
	s.sound.sfxHeldStartFn = func(primary, fallback string, pan, vol float64, loop bool) int {
		heldCalled = true
		return 12
	}

	s.updateAmbientSfx(map[*game.UnitInfo]bool{})

	if introPrimary != "ENGIN1" {
		t.Fatalf("mobile idle introloop primary = %q, want ENGIN1", introPrimary)
	}
	if heldCalled {
		t.Fatal("mobile idle should use introloop start, not immediate held loop")
	}
	if s.sound.ambientSfxID != 11 {
		t.Fatalf("ambientSfxID = %d, want introloop handle 11", s.sound.ambientSfxID)
	}
}

func TestAmbientWorkSoundUsesHeldLoop(t *testing.T) {
	m := &world.Map{Surface: make([]uint8, 16)}
	for i := range m.Surface {
		m.Surface[i] = world.SurfaceLand
	}
	m.W, m.H = 4, 4
	s := &Screen{wmap: m, r: drawmap.New(m)}
	eng := game.NewUnitInfo(assets.Engineer, 0, 1, 0)
	eng.Flags = game.FlagMobileLandUnit
	eng.GridX, eng.GridY = 1, 1
	eng.Building = true
	eng.State = game.OrderStateBuildInProgress
	s.selected = eng

	var introCalled bool
	var heldPrimary string
	s.sound.sfxIntroLoopStartFn = func(primary, fallback string, pan, vol float64) int {
		introCalled = true
		return 11
	}
	s.sound.sfxHeldStartFn = func(primary, fallback string, pan, vol float64, loop bool) int {
		heldPrimary = primary
		return 12
	}

	s.updateAmbientSfx(map[*game.UnitInfo]bool{})

	if introCalled {
		t.Fatal("building/work sound should not use introloop start")
	}
	if heldPrimary != "ENGIN10" {
		t.Fatalf("work held-loop primary = %q, want ENGIN10", heldPrimary)
	}
	if s.sound.ambientSfxID != 12 {
		t.Fatalf("ambientSfxID = %d, want held-loop handle 12", s.sound.ambientSfxID)
	}
}

// TestPositionalUnitSfx checks the klei positional model end to end on the screen side: a unit's one shot goes
// out with the pan and fade of its cell against the view centre, and after the view moves every live held sound
// (a drive sound, the ambient hum at its half volume) is panned and faded again.
func TestPositionalUnitSfx(t *testing.T) {
	s, g := newClickScreen(t)
	tank := mkUnit(g, assets.Tank, 1, 2, 3, game.FlagMobileLandUnit|game.FlagSelectable)

	var gotPan, gotVol float64
	plays := 0
	s.sound.sfxPosFn = func(_, _ string, pan, volScale float64) { gotPan, gotVol = pan, volScale; plays++ }
	s.sound.sfxFn = func(string, string, float64) { t.Error("a unit sound fell back to the plain centred hook") }

	expect := func() (float64, float64) {
		cellX, cellY := s.viewCenterCell()
		return units.SfxPosition(tank.GridX-cellX, tank.GridY-cellY, s.r.Zoom(), s.sound.channelsReversed)
	}

	s.playSfxForUnit(tank, game.SfxFire)
	wantPan, wantVol := expect()
	if plays != 1 || gotPan != wantPan || gotVol != wantVol {
		t.Fatalf("one shot = %d plays at (%v, %v), want 1 at (%v, %v)", plays, gotPan, gotVol, wantPan, wantVol)
	}

	// Two live held sounds on the tank, then the view scrolls 5 cells right and the channels are reversed.
	s.sound.driveSfx = map[*game.UnitInfo]int{tank: 9}
	s.sound.ambientSfxID, s.sound.ambientSfxUnit = 12, tank
	s.camera.scrollX += 5 * s.r.Zoom()
	s.sound.channelsReversed = true

	pans, vols := map[int]float64{}, map[int]float64{}
	s.sound.sfxHeldPanFn = func(id int, pan float64) { pans[id] = pan }
	s.sound.sfxHeldVolumeFn = func(id int, volScale float64) { vols[id] = volScale }
	s.updateHeldSfxPositions(map[*game.UnitInfo]bool{tank: true})

	wantPan, wantVol = expect()
	if pans[9] != wantPan || vols[9] != wantVol {
		t.Errorf("drive sound = (%v, %v), want (%v, %v)", pans[9], vols[9], wantPan, wantVol)
	}
	if pans[12] != wantPan || vols[12] != ambientVolumeScale*wantVol {
		t.Errorf("ambient hum = (%v, %v), want (%v, %v)", pans[12], vols[12], wantPan, ambientVolumeScale*wantVol)
	}

	// A drive sound of a unit that stopped this frame is left alone.
	delete(pans, 9)
	s.updateHeldSfxPositions(map[*game.UnitInfo]bool{})
	if _, touched := pans[9]; touched {
		t.Error("a stopped unit's drive sound was repositioned")
	}
}
