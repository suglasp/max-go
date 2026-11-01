// benchstops_test.go : testing unit for benchstops.go
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package gamescreen

import (
	"strings"
	"testing"

	"github.com/suglasp/max-go/internal/assets"
	"github.com/suglasp/max-go/internal/game"
	"github.com/suglasp/max-go/internal/platform"
	"github.com/suglasp/max-go/internal/world"
)

// TestBenchAnchors checks the nine compass anchors on a 112x112 map: reading order, names, 1/6 1/2 5/6 grid.
func TestBenchAnchors(t *testing.T) {
	m := &world.Map{}
	m.W, m.H = 112, 112
	s := &Screen{wmap: m}
	anchors := s.benchAnchors()
	want := []struct {
		name string
		x, y int
	}{
		{"north west", 18, 18}, {"north", 56, 18}, {"north east", 93, 18},
		{"west", 18, 56}, {"centre", 56, 56}, {"east", 93, 56},
		{"south west", 18, 93}, {"south", 56, 93}, {"south east", 93, 93},
	}
	if len(anchors) != len(want) {
		t.Fatalf("%d anchors, want %d", len(anchors), len(want))
	}
	for i, w := range want {
		if a := anchors[i]; a.Name != w.name || a.Cell.X != w.x || a.Cell.Y != w.y {
			t.Errorf("anchor %d = %q %v, want %q (%d,%d)", i, a.Name, a.Cell, w.name, w.x, w.y)
		}
	}
}

// TestBenchStopsTab checks Tab and Shift+Tab step through the stops and wrap both ways, each stop selecting its
// unit, centring the camera on its cell and showing its numbered compass banner; with no stops Tab does nothing.
func TestBenchStopsTab(t *testing.T) {
	s, g := newClickScreen(t)

	s.handleHotkey(platform.KeyTab, nil) // no stops: a normal game, nothing happens
	if s.selected != nil || s.banner.msg != "" {
		t.Fatal("Tab acted outside a bench")
	}

	a := mkUnit(g, assets.Tank, 1, 2, 2, game.FlagMobileLandUnit|game.FlagSelectable)
	b := mkUnit(g, assets.Scout, 2, 12, 12, game.FlagMobileLandUnit|game.FlagSelectable)
	a.Hits, b.Hits = 10, 10
	s.addBenchStop(benchStop{Compass: "north west", Title: "first", Text: "do one", Cell: game.Point{X: 2, Y: 2}, Select: a})
	s.addBenchStop(benchStop{Compass: "south east", Title: "second", Cell: game.Point{X: 12, Y: 12}, Select: b})

	s.startBench("Intro.")
	if s.selected != a || !strings.Contains(s.banner.msg, "Intro. Tab = next scenario") ||
		!strings.Contains(s.banner.msg, "1/2 NORTH WEST: first - do one") {
		t.Fatalf("start: selected %v, banner %q", s.selected, s.banner.msg)
	}

	s.handleHotkey(platform.KeyTab, nil)
	if s.selected != b || s.bench.index != 1 || s.banner.msg != "2/2 SOUTH EAST: second" {
		t.Errorf("Tab: selected %v index %d banner %q", s.selected, s.bench.index, s.banner.msg)
	}
	vw, vh := viewSize(MainMap)
	wantX, wantY := centerScroll(12, 12, s.r.Zoom(), vw, vh)
	wantX, wantY = s.r.ClampScroll(wantX, wantY, vw, vh)
	if s.camera.scrollX != wantX || s.camera.scrollY != wantY {
		t.Errorf("camera at (%d,%d), want (%d,%d)", s.camera.scrollX, s.camera.scrollY, wantX, wantY)
	}

	s.handleHotkey(platform.KeyTab, nil) // wraps to the first
	if s.bench.index != 0 || s.selected != a {
		t.Errorf("Tab did not wrap: index %d", s.bench.index)
	}

	s.orderInput.shiftMod = true
	s.handleHotkey(platform.KeyTab, nil) // Shift+Tab wraps back to the last
	if s.bench.index != 1 || s.selected != b {
		t.Errorf("Shift+Tab did not wrap back: index %d", s.bench.index)
	}
}

// TestStartSingleBench checks a one stop bench banner has no count and no compass.
func TestStartSingleBench(t *testing.T) {
	s, _ := newClickScreen(t)
	s.startSingleBench("Soldier bench: watch them walk.", game.Point{X: 4, Y: 4}, nil)
	if s.banner.msg != "Soldier bench: watch them walk." {
		t.Errorf("banner %q", s.banner.msg)
	}
}

// TestBenchBannerReturnsAfterAlert checks a bench banner displaced by another banner (the turn start alert)
// comes back once that banner has run out, and only once.
func TestBenchBannerReturnsAfterAlert(t *testing.T) {
	s, _ := newClickScreen(t)
	s.startSingleBench("Bench: what to do.", game.Point{X: 4, Y: 4}, nil)

	s.showAlertBanner("Enemy detected!")
	s.tickBenchBanner()
	if s.banner.msg != "Enemy detected!" {
		t.Fatalf("the bench banner pushed the live alert away: %q", s.banner.msg)
	}

	s.banner.msgExpire = 0 // the alert ran out
	s.tickBenchBanner()
	if s.banner.msg != "Bench: what to do." || !s.bench.restored {
		t.Fatalf("the bench banner did not come back: %q", s.banner.msg)
	}

	s.showAlertBanner("Enemy detected!")
	s.banner.msgExpire = 0
	s.tickBenchBanner()
	if s.banner.msg != "Enemy detected!" {
		t.Errorf("the bench banner came back twice: %q", s.banner.msg)
	}
}
