// navalfirebench_test.go : testing unit for navalfirebench.go
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package gamescreen

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/suglasp/max-go/internal/assets"
	"github.com/suglasp/max-go/internal/combat"
	"github.com/suglasp/max-go/internal/game"
	"github.com/suglasp/max-go/internal/world"
)

// TestNavalFireBenchLanes checks the bench finds its three lanes on SNOW_1 and each gives the wanted line of
// fire: blocked across land, clear on open water and clear past a friendly ship (units never block).
func TestNavalFireBenchLanes(t *testing.T) {
	dir := os.Getenv("MAX_DATA_DIR")
	if dir == "" {
		t.Skip("game data not found (set MAX_DATA_DIR)")
	}
	m, err := world.LoadMap(filepath.Join(dir, "SNOW_1.WRL"))
	if err != nil {
		t.Skipf("SNOW_1.WRL: %v", err)
	}
	s := &Screen{wmap: m}
	originX, originY, ok := s.snowBlock(8, 8)
	if !ok {
		originX, originY = m.W/2, m.H/2
	}
	center := game.Point{X: originX + 4, Y: originY + 4}
	taken := map[game.Point]bool{}

	blocked, ok := s.findNavalLane(center, taken, true)
	if !ok {
		t.Fatal("no water lane across land on SNOW_1")
	}
	clear, ok := s.findNavalLane(blocked.From, taken, false)
	if !ok {
		t.Fatal("no open water lane on SNOW_1")
	}
	friendly, ok := s.findNavalLane(blocked.From, taken, false)
	if !ok {
		t.Fatal("no second open water lane on SNOW_1")
	}

	t.Logf("lanes: blocked %v, clear %v, friendly %v", blocked, clear, friendly)
	for _, c := range []struct {
		name   string
		lane   navalLane
		wanted bool
	}{
		{"blocked", blocked, false},
		{"clear", clear, true},
		{"friendly", friendly, true},
	} {
		u := game.NewUnitInfo(assets.Corvette, 0, 0, 0)
		u.SetPosition(c.lane.From.X, c.lane.From.Y)
		if got := combat.WithinAttackRange(u, c.lane.To.X, c.lane.To.Y, navalFireBenchRange, m); got != c.wanted {
			t.Errorf("%s lane %v: line of fire clear = %v, want %v", c.name, c.lane, got, c.wanted)
		}
		if n := len(c.lane.cells()) - 1; n > navalFireBenchRange {
			t.Errorf("%s lane %v: %d cells long, past the bench range %d", c.name, c.lane, n, navalFireBenchRange)
		}
	}
}
