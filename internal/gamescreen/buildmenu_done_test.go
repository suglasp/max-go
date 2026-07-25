// buildmenu_done_test.go : tests build/done event delivery and the materialise animation on the game screen.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package gamescreen

import (
	"strings"
	"testing"

	"github.com/suglasp/max-go/internal/access"
	"github.com/suglasp/max-go/internal/assets"
	"github.com/suglasp/max-go/internal/economy"
	"github.com/suglasp/max-go/internal/game"
	"github.com/suglasp/max-go/internal/screendata"
	"github.com/suglasp/max-go/internal/sim"
	"github.com/suglasp/max-go/internal/units"
	"github.com/suglasp/max-go/internal/voice"
	"github.com/suglasp/max-go/internal/world"
)

// spoke reports whether tag is among the played voice tags.
func spoke(played []string, tag string) bool {
	for _, t := range played {
		if t == tag {
			return true
		}
	}
	return false
}

// TestBuildPlacementPromptSilentInNormalGame verifies the strict original behaviour: the
// V_F050 "Position tape..." prompt is a cue only for TRAINING (buildmenu.cpp:931), so a
// normal game stays SILENT on the build box - no prompt for a constructor's
// SELECT_SITE, an engineer's path pick, or an immediate 1x1 build.
func TestBuildPlacementPromptSilentInNormalGame(t *testing.T) {
	capture := func(s *Screen) *[]string {
		var played []string
		s.announcer.voiceFn = func(tag string) { played = append(played, tag) }
		s.announcer.clock = func() uint64 { return 1000 }
		return &played
	}

	con, _ := builderScreen(t, assets.Constrct, 8, 8, -1, -1,
		screendata.BuildSelection{Type: assets.MiningSt, Rate: 1, Path: false})
	conPlayed := capture(con)
	con.openBuild()
	if spoke(*conPlayed, voice.BuildPlacementPrompt) {
		t.Errorf("a normal-game constructor SELECT_SITE must NOT speak the training prompt, got %v", *conPlayed)
	}

	eng, _ := builderScreen(t, assets.Engineer, 8, 8, -1, -1,
		screendata.BuildSelection{Type: assets.ADump, Rate: 1, Path: false})
	engPlayed := capture(eng)
	eng.openBuild()
	if spoke(*engPlayed, voice.BuildPlacementPrompt) {
		t.Errorf("an immediate 1x1 build must NOT speak the placement prompt, got %v", *engPlayed)
	}

	path, _ := builderScreen(t, assets.Engineer, 8, 8, -1, -1,
		screendata.BuildSelection{Type: assets.Road, Rate: 1, Path: true})
	pathPlayed := capture(path)
	path.openBuild()
	if spoke(*pathPlayed, voice.BuildPlacementPrompt) {
		t.Errorf("a normal-game engineer path build must NOT speak the training prompt, got %v", *pathPlayed)
	}
}

// mixedSurf is land everywhere except a single configurable water cell, so a test
// can stand a builder on valid land or on an invalid (water) square.
type mixedSurf struct{ waterX, waterY int }

func (m mixedSurf) SurfaceAt(x, y int) uint8 {
	// The one water cell makes a land building's site check fail there.
	if x == m.waterX && y == m.waterY {
		return world.SurfaceWater
	}
	return world.SurfaceLand
}

// builderScreen builds a headless Screen with a player builder of the given type
// at (gx,gy), and a stub Construction Menu hook that returns the chosen build.
// waterX/waterY mark a single invalid (water) cell; pass (-1,-1) for all land.
func builderScreen(t *testing.T, builder assets.UnitType, gx, gy, waterX, waterY int, sel screendata.BuildSelection) (*Screen, *game.UnitInfo) {
	t.Helper()
	const w, h = 24, 24
	mgr := units.NewManager()
	grid := &access.Grid{W: w, H: h, Surf: mixedSurf{waterX, waterY}, Units: mgr.Cells}
	p := &game.StartParams{}
	p.Teams[0].Type = game.TeamPlayer
	g := sim.NewWithGrid(game.NewGameState(p), mgr, grid, w, h)
	g.State.ActiveTeam = 0
	g.Resources = economy.NewResourceMap(w, h)

	b := game.NewUnitInfo(builder, 0, 1, 0)
	b.Flags = game.FlagMobileLandUnit | game.FlagConstructorUnit | game.FlagSelectable
	b.Values = &game.UnitValues{}
	b.Values.Set(game.AttribHits, 30)
	b.Values.Set(game.AttribSpeed, 16)
	b.InitState()
	b.Storage = 40
	mgr.Deploy(b, gx, gy)

	s := &Screen{game: g}
	s.selected = b
	// Stub the host Construction Menu hook: openBuild calls this for the choice.
	s.hooks.buildFn = func(_ assets.UnitType, _, _ int) (screendata.BuildSelection, bool, error) {
		return sel, true, nil
	}
	return s, b
}

// TestEngineerDoneBuildsImmediately: an engineer's Done on valid land starts a
// build at once (one item where it stands) and leaves NO placement cursor armed -
// buildIdx back to 0, buildPath false. C++: MobileBuildMenu::Build else branch.
func TestEngineerDoneBuildsImmediately(t *testing.T) {
	s, eng := builderScreen(t, assets.Engineer, 8, 8, -1, -1,
		screendata.BuildSelection{Type: assets.ADump, Rate: 1, Path: false})

	s.openBuild()

	if s.game.BuildOrderFor(eng) == nil {
		t.Fatal("engineer Done did not start an immediate build on valid land")
	}
	if s.buildUI.buildIdx != 0 {
		t.Errorf("buildIdx = %d after immediate build, want 0 (no placement cursor armed)", s.buildUI.buildIdx)
	}
	if s.buildUI.buildPath {
		t.Error("buildPath set after a Done build (should never path-build on Done)")
	}
}

// TestEngineerDoneInvalidSquareAborts: an engineer's Done on an invalid square does
// NOT start a build and does NOT leave a placement cursor armed (buildIdx 0, not 1).
// buildIdx must return to zero so the cursor does not remain in site-selection mode.
func TestEngineerDoneInvalidSquareAborts(t *testing.T) {
	// The engineer stands on the one water cell, so the land building cannot be laid.
	s, eng := builderScreen(t, assets.Engineer, 8, 8, 8, 8,
		screendata.BuildSelection{Type: assets.ADump, Rate: 1, Path: false})

	s.openBuild()

	if s.game.BuildOrderFor(eng) != nil {
		t.Fatal("engineer started a build on an invalid square")
	}
	if s.buildUI.buildIdx != 0 {
		t.Errorf("buildIdx = %d after an invalid-square Done, want 0 (no placement cursor armed)", s.buildUI.buildIdx)
	}
	if s.buildUI.buildPath {
		t.Error("buildPath set after an invalid-square Done")
	}
}

// TestConstructorDoneArmsSelectSite: a constructor's Done arms SELECT_SITE (the
// next map click positions the 2x2 footprint), so buildIdx stays 1 and no build
// starts yet. C++: MobileBuildMenu::Build BUILDING flag branch.
func TestConstructorDoneArmsSelectSite(t *testing.T) {
	s, con := builderScreen(t, assets.Constrct, 8, 8, -1, -1,
		screendata.BuildSelection{Type: assets.MiningSt, Rate: 1, Path: false})

	s.openBuild()

	if s.game.BuildOrderFor(con) != nil {
		t.Fatal("constructor Done built immediately, want SELECT_SITE (wait for a click)")
	}
	if s.buildUI.buildIdx != 1 {
		t.Errorf("buildIdx = %d after constructor Done, want 1 (SELECT_SITE armed)", s.buildUI.buildIdx)
	}
}

// TestBuildRefusalNamesTheSurface: the original refuses a 1x1 build with one generic line
// for every reason, which leaves the player guessing - and the map's art does not tell you
// what the pass table calls a tile (a shoreline tile can be drawn as water meeting land
// and still be classified LAND, so a Bridge refuses it). When the SURFACE is the blocker
// the port now says so and names both surfaces.
func TestBuildRefusalNamesTheSurface(t *testing.T) {
	s, g := newClickScreen(t)

	engineer := mkUnit(g, assets.Engineer, 1, 5, 5, game.FlagMobileLandUnit|game.FlagSelectable|game.FlagConstructorUnit)

	// A Bridge on a LAND cell: the surface is the reason, so it must be spelled out.
	bridgeFlags := game.FlagStationary | game.FlagSelectable
	bridge := access.UnitDesc{Type: assets.Bridge, Flags: bridgeFlags, LandType: access.LandTypeForUnit(assets.Bridge, bridgeFlags)}

	msg := s.buildRefusalText(bridge, engineer)
	for _, want := range []string{"Bridge", "water", "coastal shallows", "land"} {
		if !strings.Contains(msg, want) {
			t.Errorf("the refusal %q does not mention %q", msg, want)
		}
	}

	// A type the cell's surface DOES accept falls back to the original's generic line.
	dumpFlags := game.FlagStationary | game.FlagSelectable
	dump := access.UnitDesc{Type: assets.ADump, Flags: dumpFlags, LandType: access.LandTypeForUnit(assets.ADump, dumpFlags)}

	if got := s.buildRefusalText(dump, engineer); !strings.Contains(got, "invalid square") {
		t.Errorf("a non-surface refusal = %q, want the original's generic line", got)
	}
}

// TestSurfaceWords spells a surface mask out for the player.
func TestSurfaceWords(t *testing.T) {
	if got := surfaceWords(world.SurfaceWater | world.SurfaceCoast); got != "coastal shallows or water" {
		t.Errorf("water+coast = %q", got)
	}
	if got := surfaceWords(world.SurfaceLand); got != "land" {
		t.Errorf("land = %q", got)
	}
	if got := surfaceWords(0); got != "nothing" {
		t.Errorf("empty mask = %q", got)
	}
}
