// spotted_center_test.go : tests the F1 camera jump to the latest spotted enemy.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package gamescreen

import (
	"testing"

	"github.com/suglasp/max-go/internal/assets"
	"github.com/suglasp/max-go/internal/game"
)

// TestF1CentersOnLastSpottedEnemy: checkEnemyDetected latches the spotted enemy's cell,
// lastSpottedTarget returns it (falling back to a visible enemy before any latch), and
// centerOnLastSpotted scrolls the view onto that cell.
func TestF1CentersOnLastSpottedEnemy(t *testing.T) {
	s, g := newClickScreen(t)
	s.announcer.voiceFn = func(string) {}
	s.sound.sfxFn = func(string, string, float64) {}
	s.announcer.clock = func() uint64 { return 0 }

	// A player scout with scan coverage over a hostile cell several tiles away.
	scout := mkUnit(g, assets.Tank, 1, 5, 5, game.FlagMobileLandUnit|game.FlagSelectable)
	scout.Values.Set(game.AttribScan, 6)

	enemy := game.NewUnitInfo(assets.Tank, 1, 2, 0) // team 1 = enemy
	enemy.Flags = game.FlagMobileLandUnit | game.FlagSelectable
	enemy.Values = &game.UnitValues{}
	enemy.Order = game.OrderAwait // non idle, so visibility keys on coverage
	g.Units.Deploy(enemy, 9, 5)
	g.Vis.Rebuild(g.Units.Units())

	// Fallback before any latch: a currently visible enemy is a valid F1 target.
	if x, y, ok := s.lastSpottedTarget(); !ok || x != 9 || y != 5 {
		t.Errorf("fallback target = (%d,%d) ok=%v; want the visible enemy (9,5)", x, y, ok)
	}

	s.checkEnemyDetected() // latches the spotted enemy cell
	if !s.announcer.hasSpottedEnemy {
		t.Fatal("checkEnemyDetected did not latch a spotted enemy")
	}
	if x, y, ok := s.lastSpottedTarget(); !ok || x != 9 || y != 5 {
		t.Errorf("latched target = (%d,%d) ok=%v; want (9,5)", x, y, ok)
	}

	// F1 recentres the view onto that cell (the deterministic centerScroll result).
	vw, vh := viewSize(MainMap)
	wantX, wantY := centerScroll(9, 5, s.r.Zoom(), vw, vh)
	wantX, wantY = s.r.ClampScroll(wantX, wantY, vw, vh)
	s.camera.scrollX, s.camera.scrollY = -1, -1
	s.centerOnLastSpotted()
	if s.camera.scrollX != wantX || s.camera.scrollY != wantY {
		t.Errorf("F1 scrolled to (%d,%d); want (%d,%d)", s.camera.scrollX, s.camera.scrollY, wantX, wantY)
	}
}
