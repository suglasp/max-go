// navalfirebench_test.go : testing unit for navalfirebench.go
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package gamescreen

import (
	"path/filepath"
	"testing"

	"github.com/suglasp/max-go/internal/combat"
	"github.com/suglasp/max-go/internal/drawmap"
	"github.com/suglasp/max-go/internal/game"
	"github.com/suglasp/max-go/internal/sim"
	"github.com/suglasp/max-go/internal/testutil"
	"github.com/suglasp/max-go/internal/world"
)

// benchScreen builds a headless Screen on the retail map file (MAX_DATA_DIR), or skips the test.
func benchScreen(t *testing.T, file string) *Screen {
	t.Helper()
	dir := testutil.DataDir(t)
	m, err := world.LoadMap(filepath.Join(dir, file))
	if err != nil {
		t.Skipf("%s: %v", file, err)
	}
	p := &game.StartParams{}
	p.Teams[0].Type = game.TeamPlayer
	g := sim.New(p, m)
	return &Screen{game: g, wmap: m, r: drawmap.New(m)}
}

// checkBenchStopsApart fails when two stops share a compass anchor or lie close enough to share the screen.
func checkBenchStopsApart(t *testing.T, stops []benchStop) {
	t.Helper()
	seen := map[string]bool{}
	for i, a := range stops {
		if a.Compass == "" || seen[a.Compass] {
			t.Errorf("stop %d %s: compass %q missing or used twice", i+1, a.Title, a.Compass)
		}
		seen[a.Compass] = true
		for _, b := range stops[i+1:] {
			if d := max(abs(a.Cell.X-b.Cell.X), abs(a.Cell.Y-b.Cell.Y)); d < benchStopsApart {
				t.Errorf("%s %v and %s %v are %d cells apart, want %d or more", a.Title, a.Cell, b.Title, b.Cell, d, benchStopsApart)
			}
		}
	}
}

// benchStopsApart is the least distance (cells, larger axis) between two stops: a zoom 32 view is at most 20
// cells wide, so a neighbour stays off screen.
const benchStopsApart = 20

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}

// TestNavalFireBenchLanes runs the bench on SNOW_1: eight lanes, each its own stop at its own compass anchor and
// far apart, each with the wanted line of fire to its enemy: blocked across land, or to a Tank inland; clear
// along coast, on a diagonal that only touches land at a corner, on open water, past a friendly ship, under a
// bridge, or to a Tank on the shore (which the Corvette may also target).
func TestNavalFireBenchLanes(t *testing.T) {
	s := benchScreen(t, "SNOW_1.WRL")
	s.DeployNavalFireBench()

	stops := s.bench.stops
	if len(stops) != 8 {
		t.Fatalf("%d lanes placed, want 8", len(stops))
	}
	checkBenchStopsApart(t, stops)

	for _, stop := range stops {
		shooter := stop.Select
		var target *game.UnitInfo
		for _, u := range s.game.Units.Units() {
			if u.Team == 1 && (target == nil || dist2(u, shooter) < dist2(target, shooter)) {
				target = u
			}
		}
		want := stop.Title != "H9 BLOCKED" && stop.Title != "H20 INLAND"
		if !combat.CanTarget(shooter, target, s.game.Grid.Surf) {
			t.Errorf("%s %s: the shooter cannot target the %v at all", stop.Compass, stop.Title, target.Type)
		}
		if got := combat.WithinAttackRange(shooter, target.GridX, target.GridY, navalFireBenchRange, s.game.Grid.Surf); got != want {
			t.Errorf("%s %s: (%d,%d) -> (%d,%d) line of fire clear = %v, want %v", stop.Compass, stop.Title,
				shooter.GridX, shooter.GridY, target.GridX, target.GridY, got, want)
		}
		t.Logf("%s %s: (%d,%d) -> (%d,%d)", stop.Compass, stop.Title, shooter.GridX, shooter.GridY, target.GridX, target.GridY)
	}
}

func dist2(a, b *game.UnitInfo) int {
	dx, dy := a.GridX-b.GridX, a.GridY-b.GridY
	return dx*dx + dy*dy
}
