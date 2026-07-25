// buildexit_test.go : tests buildexit.go.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package gamescreen

import (
	"testing"

	"github.com/suglasp/max-go/internal/access"
	"github.com/suglasp/max-go/internal/assets"
	"github.com/suglasp/max-go/internal/economy"
	"github.com/suglasp/max-go/internal/game"
	"github.com/suglasp/max-go/internal/sim"
	"github.com/suglasp/max-go/internal/units"
	"github.com/suglasp/max-go/internal/world"
)

type buildExitSurf struct{}

func (buildExitSurf) SurfaceAt(x, y int) uint8 { return world.SurfaceLand }

// finishedConstructor builds a headless game with a player constructor that has
// just finished a 2x2 building, leaving it parked build ready on the site.
func finishedConstructor(t *testing.T) (*sim.Game, *game.UnitInfo, int, int) {
	t.Helper()
	const w, h = 24, 24
	mgr := units.NewManager()
	grid := &access.Grid{W: w, H: h, Surf: buildExitSurf{}, Units: mgr.Cells}
	p := &game.StartParams{}
	p.Teams[0].Type = game.TeamPlayer
	g := sim.NewWithGrid(game.NewGameState(p), mgr, grid, w, h)
	g.Resources = economy.NewResourceMap(w, h)

	c := game.NewUnitInfo(assets.Constrct, 0, 1, 0)
	c.Flags = game.FlagMobileLandUnit | game.FlagConstructorUnit | game.FlagSelectable
	c.Values = &game.UnitValues{}
	c.Values.Set(game.AttribSpeed, 16)
	c.Values.Set(game.AttribHits, 30)
	c.InitState()
	c.Speed = 16
	c.Storage = 40
	mgr.Deploy(c, 7, 8)

	prod := game.NewUnitInfo(assets.MiningSt, 0, 0, 0)
	prod.Flags = game.FlagBuilding | game.FlagStationary | game.FlagSelectable
	prod.Values = &game.UnitValues{}
	prod.Values.Set(game.AttribTurns, 1)
	prod.Values.Set(game.AttribHits, 50)

	siteX, siteY := 8, 8
	g.StartBuild(c, prod, siteX, siteY, 1)
	g.BeginTeamTurn(0) // finishes the build; the player builder parks build ready

	if !g.IsBuildReady(c) {
		t.Fatal("setup: constructor is not build-ready after finishing")
	}
	return g, c, siteX, siteY
}

// TestBuildFinishHidesStructureUntilBuilderLeaves: a just finished structure stays
// hidden (HiddenUntilExpand) behind its parked builder, which keeps its build pose
// (Building, footprint reserved) instead of snapping to the corner cell; once the
// builder drives off the footprint the structure is revealed to unfold.
func TestBuildFinishHidesStructureUntilBuilderLeaves(t *testing.T) {
	g, c, _, _ := finishedConstructor(t)

	br := g.BuildReadyInfo(c)
	if br == nil || br.Building == nil {
		t.Fatal("no build-ready record / structure for the finished constructor")
	}
	bld := br.Building
	if !bld.HiddenUntilExpand {
		t.Error("finished structure should be hidden until the builder clears the site")
	}
	if !c.Building || c.BuildFootprint != 2 {
		t.Errorf("parked builder dropped its build pose: Building=%v footprint=%d (want true / 2)", c.Building, c.BuildFootprint)
	}

	// Drive it off a marker, then let the move resolve: the builder leaves the
	// footprint, dropping its pose, and the structure is revealed to unfold.
	s := &Screen{game: g}
	s.selected = c
	markers := g.ExitMarkers(c)
	if len(markers) == 0 {
		t.Fatal("no exit markers available")
	}
	s.clickCell(markers[0][0], markers[0][1])
	g.ResolveMovement()

	if bld.HiddenUntilExpand {
		t.Error("structure still hidden after the builder cleared the site")
	}
	if c.Building {
		t.Error("builder still in the build pose after driving off")
	}
}

// TestBuildReadySelectionSurvivesNewTurn verifies that
// a builder left parked build ready across a turn boundary lost its BLDMRK exit
// markers (and the marker click stopped working) until it was selected again, because
// the deselect at the start of turn (beginNextTeam) niled the selection. The parked
// builder of the team whose turn is starting must stay selected so its markers stay
// live and the click still drives it off.
func TestBuildReadySelectionSurvivesNewTurn(t *testing.T) {
	g, c, _, _ := finishedConstructor(t)
	s := &Screen{game: g}
	s.selected = c

	// The human team's turn begins again: the parked build ready builder stays selected.
	s.clearSelectionForNewTurn(0)
	if s.selected != c {
		t.Fatalf("a build-ready builder must stay selected across a turn start, got %v", s.selected)
	}

	// Its markers are still live and a marker click still drives it off.
	markers := g.ExitMarkers(c)
	if len(markers) == 0 {
		t.Fatal("no exit markers for the still-selected build-ready builder after the turn start")
	}
	s.clickCell(markers[0][0], markers[0][1])
	if g.IsBuildReady(c) {
		t.Error("the marker click did not drive the builder off after the turn start")
	}
	if !g.HasOrder(c) {
		t.Error("the marker click did not issue the drive-off move order after the turn start")
	}
}

// TestNewTurnKeepsOwnSelectionClearsOthers: at the start of the player's turn the
// selection persists for any of their own LIVE units/buildings (vanilla keeps the
// unit you were working with across End Turn), and clears only when it is not
// theirs or it died during the opponents' turns.
func TestNewTurnKeepsOwnSelectionClearsOthers(t *testing.T) {
	g, c, _, _ := finishedConstructor(t)
	s := &Screen{game: g}

	// A plain (not build ready) unit of the player's own team that is still alive stays selected.
	tank := game.NewUnitInfo(assets.Tank, 0, 2, 0)
	tank.Flags = game.FlagMobileLandUnit | game.FlagSelectable
	tank.Values = &game.UnitValues{}
	g.Units.Deploy(tank, 4, 4)
	s.selected = tank
	s.clearSelectionForNewTurn(0)
	if s.selected != tank {
		t.Errorf("an own-team unit must stay selected across the turn start, got %v", s.selected)
	}

	// A selection that died during the opponents' turns (gone from the unit hash)
	// is a stale pointer and must clear even though it is the player's own.
	dead := game.NewUnitInfo(assets.Tank, 0, 3, 0)
	dead.Flags = game.FlagMobileLandUnit | game.FlagSelectable
	dead.Values = &game.UnitValues{}
	s.selected = dead // never deployed -> not live in g.Units
	s.clearSelectionForNewTurn(0)
	if s.selected != nil {
		t.Errorf("a dead/stale own-team selection must clear at the turn start, got %v", s.selected)
	}

	// A build ready builder of a DIFFERENT team than the one starting is deselected.
	s.selected = c
	s.clearSelectionForNewTurn(1)
	if s.selected != nil {
		t.Errorf("a unit of another team must clear when team 1 starts, got %v", s.selected)
	}
}

// TestSelectBuildReadyPlaysCompletionVoice: selecting a build READY builder (its
// structure just finished, BLDMRK markers showing) speaks a build complete line
// ("Building finished!"), matching the original's ORDER_STATE_BUILDING_READY status voice. The
// voice belongs to the FINISH moment (markers up, builder selected), not the later
// roll off.
func TestSelectBuildReadyPlaysCompletionVoice(t *testing.T) {
	g, c, _, _ := finishedConstructor(t)
	s := &Screen{game: g}
	var voices []string
	s.announcer.voiceFn = func(tag string) { voices = append(voices, tag) }
	s.sound.sfxFn = func(string, string, float64) {} // swallow the select blip

	// Selecting the build ready constructor announces completion.
	s.selectUnit(c)
	found := false
	for _, v := range voices {
		switch v {
		case "V_F162", "V_F163", "V_F164", "V_F165":
			found = true
		}
	}
	if !found {
		t.Errorf("selecting a build-ready builder should speak a build-complete voice, got %v", voices)
	}
}

// TestRollOffSpeaksUnitCompleted: driving the finished builder off its marker speaks
// the "Unit completed" line (vsUnitComplete: V_F166/169/...), distinct from the "Unit
// ready!" announce at finish. Verified in vanilla: finish -> "Unit ready", drive off ->
// "Unit completed" (two voice events). C++: ORDER_STATE_UNIT_READY drop voice.
func TestRollOffSpeaksUnitCompleted(t *testing.T) {
	g, c, _, _ := finishedConstructor(t)
	s := &Screen{game: g}
	var voices []string
	s.announcer.voiceFn = func(tag string) { voices = append(voices, tag) }
	s.sound.sfxFn = func(string, string, float64) {}
	s.selected = c // already selected (finish already announced); the drive off itself speaks

	markers := g.ExitMarkers(c)
	if len(markers) == 0 {
		t.Fatal("no exit markers available")
	}
	s.clickCell(markers[0][0], markers[0][1])

	found := false
	for _, v := range voices {
		switch v {
		case "V_F166", "V_F169", "V_F215", "V_F216", "V_F217":
			found = true
		}
	}
	if !found {
		t.Errorf("driving the finished builder off should speak a unit-completed voice, got %v", voices)
	}
}

// TestClickMarkerExitsBuilder: clicking an exit marker drives the build ready
// constructor off the site; clicking off the markers does not move it.
func TestClickMarkerExitsBuilder(t *testing.T) {
	g, c, siteX, siteY := finishedConstructor(t)
	s := &Screen{game: g}
	s.selected = c

	// A click on an empty cell that is not a marker must not move the parked builder.
	s.clickCell(0, 0)
	if !g.IsBuildReady(c) {
		t.Fatal("clicking off the markers cleared build-ready (should be ignored)")
	}
	if g.HasOrder(c) {
		t.Fatal("clicking off the markers issued a move order")
	}

	// A click on a real marker drives it off and clears the build ready state.
	markers := g.ExitMarkers(c)
	if len(markers) == 0 {
		t.Fatal("no exit markers available")
	}
	m := markers[0]
	s.clickCell(m[0], m[1])
	if g.IsBuildReady(c) {
		t.Error("clicking a marker did not clear build-ready")
	}
	if !g.HasOrder(c) {
		t.Error("clicking a marker did not issue the drive-off move order")
	}
	g.ResolveMovement()
	onSite := c.GridX >= siteX && c.GridX <= siteX+1 && c.GridY >= siteY && c.GridY <= siteY+1
	if onSite {
		t.Errorf("constructor still on the footprint at (%d,%d) after exiting", c.GridX, c.GridY)
	}
}
