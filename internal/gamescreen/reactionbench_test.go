// reactionbench_test.go : testing unit for reactionbench.go
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package gamescreen

import (
	"testing"

	"github.com/suglasp/max-go/internal/assets"
	"github.com/suglasp/max-go/internal/game"
	"github.com/suglasp/max-go/internal/sim"
)

// reactionBenchScreen deploys the reaction fire bench headless on a 112x112 all land map.
func reactionBenchScreen(t *testing.T) (*Screen, *sim.Game) {
	t.Helper()
	s, g := newTestScreen(t, 112)
	s.DeployReactionFireBench()
	if len(s.bench.stops) != 2 {
		t.Fatalf("%d stops, want 2 (near and far pair)", len(s.bench.stops))
	}
	return s, g
}

// benchTurrets returns the bench's player Gun Turrets in stop order (near first).
func benchTurrets(s *Screen) []*game.UnitInfo {
	var out []*game.UnitInfo
	for _, stop := range s.bench.stops {
		for _, u := range s.game.Units.Units() {
			if u.Type == assets.GunTurrt && u.GridY == stop.Cell.Y && abs(u.GridX-stop.Cell.X) < 12 {
				out = append(out, u)
			}
		}
	}
	return out
}

// TestReactionFireBenchFights runs both fights headless: each Tank drives in, each Gun Turret fires back, and
// the near turret answers first.
func TestReactionFireBenchFights(t *testing.T) {
	s, g := reactionBenchScreen(t)
	turrets := benchTurrets(s)
	if len(turrets) != 2 {
		t.Fatalf("%d turrets found, want 2", len(turrets))
	}

	var order []*game.UnitInfo
	for i := 0; i < 20000 && len(order) < 2; i++ {
		g.Tick()
		for _, r := range g.DrainReactions() {
			if len(order) == 0 || order[len(order)-1] != r.Reactor {
				order = append(order, r.Reactor)
			}
		}
	}
	if len(order) < 2 {
		t.Fatalf("only %d turret(s) fired back", len(order))
	}
	if order[0] != turrets[0] || order[1] != turrets[1] {
		t.Errorf("the far turret fired first")
	}
}

// TestReactionFireBenchVoice checks the H1 gate on the bench: with the camera on the near pair and nothing
// selected, only the far turret's reaction speaks; both show the banner.
func TestReactionFireBenchVoice(t *testing.T) {
	s, g := reactionBenchScreen(t)
	var voices []string
	var clk uint64 = 100000
	s.announcer.clock = func() uint64 { return clk }
	s.announcer.voiceFn = func(tag string) { voices = append(voices, tag) }
	s.clearSelected()

	banners := 0
	for i := 0; i < 20000 && banners < 2; i++ {
		clk += 20
		g.Tick()
		before := len(s.banner.msgLog)
		s.tickReactions()
		for _, e := range s.banner.msgLog[before:] {
			if e.text == "Gun Turret reacts to enemy Tank" {
				banners++
			}
		}
		if banners == 1 && len(voices) != 0 {
			t.Fatalf("the near (on screen) reaction spoke: %v", voices)
		}
	}
	if banners < 2 {
		t.Fatalf("%d reaction banners, want one per pair", banners)
	}
	if len(voices) != 1 {
		t.Errorf("voices %v, want exactly the far reaction's line", voices)
	}
}

// TestReactionFireBenchSnow places both pairs on the bench's real map, SNOW_1: each on its own land strip at its
// own compass anchor, far enough apart that the far pair is off screen.
func TestReactionFireBenchSnow(t *testing.T) {
	s := benchScreen(t, "SNOW_1.WRL")
	s.DeployReactionFireBench()
	if len(s.bench.stops) != 2 {
		t.Fatalf("%d stops on SNOW_1, want 2", len(s.bench.stops))
	}
	checkBenchStopsApart(t, s.bench.stops)
	for _, stop := range s.bench.stops {
		t.Logf("%s %s at %v", stop.Compass, stop.Title, stop.Cell)
	}
}
