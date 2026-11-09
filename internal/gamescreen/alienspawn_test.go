// alienspawn_test.go : tests alienspawn.go.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package gamescreen

import (
	"bytes"
	"testing"

	"github.com/suglasp/max-go/internal/assets"
	"github.com/suglasp/max-go/internal/drawmap"
	"github.com/suglasp/max-go/internal/game"
	"github.com/suglasp/max-go/internal/saveload"
	"github.com/suglasp/max-go/internal/sim"
	"github.com/suglasp/max-go/internal/world"
)

// newAlienScreen builds a headless Screen on a large all land map for the
// alien derelict spawn tests (big enough for the separation grid to step).
func newAlienScreen(t *testing.T, size int) *Screen {
	t.Helper()
	s, g := newTestScreen(t, size)
	g.State.Teams[game.AlienTeam].Type = game.TeamAlien
	return s
}

// TestAlienDensityMapping verifies the original's resource_derelicts switch (menu.cpp).
func TestAlienDensityMapping(t *testing.T) {
	for _, tc := range []struct{ level, sep, val int }{
		{0, 0, 0}, {1, 65, 20}, {2, 50, 30}, {99, 0, 0},
	} {
		if sep, val := alienDensity(tc.level); sep != tc.sep || val != tc.val {
			t.Errorf("alienDensity(%d) = %d/%d, want %d/%d", tc.level, sep, val, tc.sep, tc.val)
		}
	}
}

// TestPopulateAlienDerelictsNoneIsInert: density None (level 0 -> separation 0,
// value 0) deploys nothing, so a custom game with no derelicts is untouched.
func TestPopulateAlienDerelictsNoneIsInert(t *testing.T) {
	s := newAlienScreen(t, 64)
	before := len(s.game.Units.Units())
	sep, val := alienDensity(0)
	s.populateAlienDerelicts(sep, val, nil, game.NewRandomizer(1))
	if after := len(s.game.Units.Units()); after != before {
		t.Errorf("density None deployed %d units; it must be inert", after-before)
	}
}

// TestPopulateAlienDerelictsSpawnsAlienOwned: a Common density spawn deploys
// derelicts, every one owned by the fixed alien slot (never a playable team), and
// keeps every site clear of the team starting positions.
func TestPopulateAlienDerelictsSpawnsAlienOwned(t *testing.T) {
	s := newAlienScreen(t, 64)
	avoid := []game.Point{{X: 5, Y: 5}, {X: 58, Y: 58}}

	sep, val := alienDensity(2)
	s.populateAlienDerelicts(sep, val, avoid, game.NewRandomizer(7))

	units := s.game.Units.Units()
	if len(units) == 0 {
		t.Fatal("Common density spawned no derelicts")
	}
	buildings := 0
	for _, u := range units {
		if u.Team != game.AlienTeam {
			t.Fatalf("a derelict is owned by team %d, want the alien slot %d", u.Team, game.AlienTeam)
		}
		if u.Flags.Has(game.FlagBuilding) {
			buildings++
			// The 2x2 artifact footprint must clear every team start by >= 16 tiles.
			for _, a := range avoid {
				dx, dy := a.X-u.GridX, a.Y-u.GridY
				if dx*dx+dy*dy < 256 {
					t.Errorf("alien artifact at (%d,%d) too close to start (%d,%d)", u.GridX, u.GridY, a.X, a.Y)
				}
			}
		}
	}
	if buildings == 0 {
		t.Error("no alien artifact building was placed")
	}
}

// TestSpawnAlienBridgeRing: an artifact placed on open water is framed by
// BRIDGE props owned by the alien team (the original's water/coast bridge ring), and a land site gets
// none (no water to bridge). value 0 keeps it to just the building + bridges.
func TestSpawnAlienBridgeRing(t *testing.T) {
	const w, h = 40, 40
	water := &world.Map{Surface: make([]uint8, w*h)}
	water.W, water.H = w, h
	for i := range water.Surface {
		water.Surface[i] = world.SurfaceWater
	}
	p := &game.StartParams{}
	p.Teams[0].Type = game.TeamPlayer
	g := sim.New(p, water)
	g.State.Teams[game.AlienTeam].Type = game.TeamAlien
	s := &Screen{game: g, wmap: water, r: drawmap.New(water)}

	s.spawnAlienDerelicts(game.Point{X: 20, Y: 20}, 0, nil, game.NewRandomizer(3))

	bridges := 0
	for _, u := range s.game.Units.Units() {
		if u.Type == assets.Bridge {
			if u.Team != game.AlienTeam {
				t.Errorf("bridge owned by team %d, want the alien slot %d", u.Team, game.AlienTeam)
			}
			bridges++
		}
	}
	if bridges == 0 {
		t.Fatal("artifact on open water got no bridge ring")
	}

	// A site that is purely land has no water/coast tiles to bridge, so no bridges deploy.
	land := newAlienScreen(t, w)
	land.spawnAlienDerelicts(game.Point{X: 20, Y: 20}, 0, nil, game.NewRandomizer(3))
	for _, u := range land.game.Units.Units() {
		if u.Type == assets.Bridge {
			t.Fatalf("land site placed a bridge at (%d,%d); only water/coast tiles should be bridged", u.GridX, u.GridY)
		}
	}
}

// TestFindAlienSpotAvoidsStart: findAlienSpot moves a point that lands on a team
// start out to a valid site at least 16 tiles clear of it, in bounds.
func TestFindAlienSpotAvoidsStart(t *testing.T) {
	s := newAlienScreen(t, 64)
	avoid := []game.Point{{X: 30, Y: 30}}
	p := game.Point{X: 30, Y: 30} // starts right on the avoided spot
	s.findAlienSpot(&p, avoid)

	dx, dy := avoid[0].X-p.X, avoid[0].Y-p.Y
	if dx*dx+dy*dy < 256 {
		t.Errorf("findAlienSpot returned (%d,%d), still < 16 tiles from the start", p.X, p.Y)
	}
	if p.X < 1 || p.Y < 1 || p.X+3 >= s.wmap.W || p.Y+3 >= s.wmap.H {
		t.Errorf("findAlienSpot returned out-of-margin (%d,%d)", p.X, p.Y)
	}
}

// newMixedAlienScreen builds a headless Screen on a mostly land map with a central
// water band, so a real alien deploy lays land derelicts + an artifact building AND
// (on the water) a coastal BRIDGE ring + juggernauts - the full roster the save
// round trip must preserve.
func newMixedAlienScreen(t *testing.T, w, h int) *Screen {
	t.Helper()
	m := &world.Map{Surface: make([]uint8, w*h)}
	m.W, m.H = w, h
	for y := 0; y < h; y++ {
		surf := uint8(world.SurfaceLand)
		if y >= h/2-3 && y <= h/2+3 {
			surf = world.SurfaceWater
		}
		for x := 0; x < w; x++ {
			m.Surface[y*w+x] = surf
		}
	}
	p := &game.StartParams{}
	p.Teams[0].Type = game.TeamPlayer
	g := sim.New(p, m)
	g.State.Teams[game.AlienTeam].Type = game.TeamAlien
	return &Screen{game: g, wmap: m, r: drawmap.New(m)}
}

// alienUnitsOf collects every unit owned by the alien team (game.AlienTeam) in a game, with a
// tally of artifact buildings and BRIDGE props, for round trip comparison.
func alienUnitsOf(g *sim.Game) (total, buildings, bridges int) {
	for _, u := range g.Units.Units() {
		if u.Team != game.AlienTeam {
			continue
		}
		total++
		if u.Type == assets.Bridge {
			bridges++
		} else if u.Flags.Has(game.FlagBuilding) {
			buildings++
		}
	}
	return
}

// TestSaveRestoreRealAlienDeploy verifies that a populated derelict site survives save and load with its mobile units, artifact buildings, and coastal bridge ring intact.
// Every alien remains owned by the fixed alien slot, inert, and outside the economy network.
func TestSaveRestoreRealAlienDeploy(t *testing.T) {
	s := newMixedAlienScreen(t, 64, 64)
	avoid := []game.Point{{X: 4, Y: 4}, {X: 59, Y: 59}}

	sep, val := alienDensity(2) // Common
	s.populateAlienDerelicts(sep, val, avoid, game.NewRandomizer(11))

	wantTotal := len(s.game.Units.Units())
	wantAlien, wantBuildings, wantBridges := alienUnitsOf(s.game)
	if wantBuildings == 0 || wantBridges == 0 || wantAlien <= wantBuildings+wantBridges {
		t.Fatalf("deploy too thin to be a useful round-trip: total=%d alien=%d buildings=%d bridges=%d",
			wantTotal, wantAlien, wantBuildings, wantBridges)
	}

	var buf bytes.Buffer
	if err := saveload.Save(&buf, saveload.Capture(s.game, "GREEN_1.WRL")); err != nil {
		t.Fatalf("save: %v", err)
	}
	snap, err := saveload.Load(&buf)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	g2 := snap.Restore(s.wmap) // before the 1025 fix this panicked on the alien building

	if got := len(g2.Units.Units()); got != wantTotal {
		t.Errorf("restored unit count = %d, want %d (units dropped on round-trip)", got, wantTotal)
	}
	gotAlien, gotBuildings, gotBridges := alienUnitsOf(g2)
	if gotAlien != wantAlien || gotBuildings != wantBuildings || gotBridges != wantBridges {
		t.Errorf("restored alien roster = %d (b=%d br=%d), want %d (b=%d br=%d)",
			gotAlien, gotBuildings, gotBridges, wantAlien, wantBuildings, wantBridges)
	}
	// Every alien stays on the unclaimable slot and never joins an economy complex
	// (the faction has no economy and is inert until a player attacks it).
	for _, u := range g2.Units.Units() {
		if u.Team == game.AlienTeam && u.Flags.Has(game.FlagBuilding) && g2.ComplexOf(u) != nil {
			t.Errorf("alien building %v at (%d,%d) wrongly joined a complex on restore", u.Type, u.GridX, u.GridY)
		}
	}
}
