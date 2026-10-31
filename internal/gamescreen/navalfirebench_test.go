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

// TestNavalFireBenchLanes checks the bench finds its six lanes on SNOW_1 and each gives the wanted line of fire:
// blocked across land; clear on open water, past a friendly ship or a bridge (units and bridges never block),
// along coast, and on a diagonal that only touches land at a corner.
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

	blocked, ok := s.findNavalLane(center, taken, laneLand)
	if !ok {
		t.Fatal("no water lane across land on SNOW_1")
	}
	clear, ok := s.findNavalLane(blocked.From, taken, laneOpen)
	if !ok {
		t.Fatal("no open water lane on SNOW_1")
	}
	friendly, ok := s.findNavalLane(blocked.From, taken, laneOpen)
	if !ok {
		t.Fatal("no second open water lane on SNOW_1")
	}
	coast, ok := s.findNavalLane(blocked.From, taken, laneCoast)
	if !ok {
		t.Fatal("no lane along coast on SNOW_1")
	}
	bridge, ok := s.findNavalLane(blocked.From, taken, laneOpen)
	if !ok {
		t.Fatal("no third open water lane (bridge) on SNOW_1")
	}
	diagonal, ok := s.findNavalLane(blocked.From, taken, laneDiagonal)
	if !ok {
		t.Fatal("no diagonal lane grazing land on SNOW_1")
	}
	if diagonal.To.X == diagonal.From.X || diagonal.To.Y == diagonal.From.Y {
		t.Errorf("the diagonal lane %v is not diagonal", diagonal)
	}

	t.Logf("lanes: blocked %v, clear %v, friendly %v, coast %v, bridge %v, diagonal %v", blocked, clear, friendly, coast, bridge, diagonal)
	for _, c := range []struct {
		name   string
		lane   navalLane
		wanted bool
	}{
		{"blocked", blocked, false},
		{"clear", clear, true},
		{"friendly", friendly, true},
		{"coast", coast, true},
		{"bridge", bridge, true},
		{"diagonal", diagonal, true},
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
