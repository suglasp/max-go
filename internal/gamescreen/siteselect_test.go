// siteselect_test.go : tests siteselect.go.
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
	"github.com/suglasp/max-go/internal/units"
)

// TestPhasePrecedence checks the derived turn phase: game over wins over site
// select, which wins over landing, which wins over a hand over banner, with
// normal play as the default.
func TestPhasePrecedence(t *testing.T) {
	s := &Screen{turnFlow: turnFlowState{handover: -1}}
	if got := s.phase(); got != phaseInGame {
		t.Errorf("idle screen phase = %d, want phaseInGame", got)
	}
	s.turnFlow.handover = 1
	if got := s.phase(); got != phaseHandover {
		t.Errorf("handover phase = %d, want phaseHandover", got)
	}
	s.camera.landingStep = 3
	if got := s.phase(); got != phaseLanding {
		t.Errorf("landing phase = %d, want phaseLanding (over handover)", got)
	}
	s.siteSelect = &siteSelectState{}
	if got := s.phase(); got != phaseSiteSelect {
		t.Errorf("site-select phase = %d, want phaseSiteSelect (over landing)", got)
	}
	s.camera.leavingStart = 1
	if got := s.phase(); got != phaseLeaving {
		t.Errorf("leaving phase = %d, want phaseLeaving (over site-select)", got)
	}
	s.gameOver = true
	if got := s.phase(); got != phaseGameOver {
		t.Errorf("game-over phase = %d, want phaseGameOver (highest)", got)
	}
}

// TestTooCloseToEnemy checks the per axis exclude range landing check: a site
// within exclude_range on BOTH axes of an enemy start is rejected; one beyond on
// either axis is allowed.
/* // note: unused code in Go port
// The test of tooCloseToEnemy, commented out with it: the computer teams now land after the human (N1), so no enemy start exists while the human picks.
func TestTooCloseToEnemy(t *testing.T) {
	s := &Screen{siteSelect: &siteSelectState{enemy: []game.Point{{X: 50, Y: 50}}}}
	cases := []struct {
		x, y int
		want bool
	}{
		{50, 50, true},                                                      // same cell
		{50 + landingExcludeRange, 50, true},                                // on the exclude edge (<=)
		{50 + landingExcludeRange + 1, 50, false},                           // one past on X -> clear
		{50, 50 + landingExcludeRange + 1, false},                           // one past on Y -> clear
		{50 + landingExcludeRange + 1, 50 + landingExcludeRange + 1, false}, // both past
		{10, 10, false},                                                     // far away
	}
	for _, c := range cases {
		if got := s.tooCloseToEnemy(c.x, c.y); got != c.want {
			t.Errorf("tooCloseToEnemy(%d,%d) = %v, want %v", c.x, c.y, got, c.want)
		}
	}
}
*/

// TestSiteSelectRingCenterMatchesUnitDrawPosition checks that the proximity
// ring the confirm animation grows is centred on the SAME screen pixel the
// MASTER lander itself is drawn at (units.SpriteCache.DrawUnit's sx = u.X*
// zoom/tile - scrollX, where u.X comes from UnitInfo.SetPosition's gridX*
// tile + tile/2). A prior version centred the ring one whole tile further
// right/down ("footprint centre (2x2 -> +1 tile)"), which visibly detached
// the ring from the lander and from wherever the player actually clicked -
// GameManager_DrawProximityZones centres on GameManager_SelectedUnit
// ->x/y, the same footprint-agnostic gridX*tile+tile/2 point every unit uses
// (unitinfo.cpp:1276), never a footprint-shifted point.
func TestSiteSelectRingCenterMatchesUnitDrawPosition(t *testing.T) {
	const tile = 64
	cellX, cellY := 5, 7
	zoom := 32
	scrollX, scrollY := 20, 10

	gotX, gotY := siteSelectRingCenter(cellX, cellY, zoom, scrollX, scrollY)

	// The MASTER lander's own draw position, independently derived the way
	// SetPosition + DrawUnit compute it (gridX*tile + tile/2, then *zoom/tile).
	unitX := cellX*tile + tile/2
	unitY := cellY*tile + tile/2
	wantX := unitX*zoom/tile - scrollX
	wantY := unitY*zoom/tile - scrollY

	if gotX != wantX || gotY != wantY {
		t.Errorf("siteSelectRingCenter(%d,%d,zoom=%d,scroll=%d,%d) = (%d,%d), want (%d,%d) (the MASTER lander's own drawn position)",
			cellX, cellY, zoom, scrollX, scrollY, gotX, gotY, wantX, wantY)
	}
}

// TestConfirmSite checks that confirming a landing site removes the MASTER
// marker, clears the site select state, and invokes the deploy closure with the
// chosen candidate cell.
func TestConfirmSite(t *testing.T) {
	s := &Screen{game: &sim.Game{Units: units.NewManager()}}
	master := game.NewUnitInfo(assets.Master, 0, 1, 0)
	s.game.Units.Deploy(master, 7, 9)

	var gotX, gotY int
	called := false
	s.siteSelect = &siteSelectState{cx: 7, cy: 9, master: master, confirm: func(x, y int) {
		called, gotX, gotY = true, x, y
	}}

	// confirmSite starts the ring grow intro: it commits but defers the deploy.
	s.confirmSite()
	if s.siteSelect == nil || s.siteSelect.introStart == 0 {
		t.Fatal("confirmSite should start the intro (introStart set), not deploy immediately")
	}
	if called {
		t.Error("deploy closure must not run until the intro finishes")
	}

	// finishSiteIntro completes it: marker removed, state cleared, base deployed.
	s.finishSiteIntro()
	if s.siteSelect != nil {
		t.Error("siteSelect should be nil after the intro finishes")
	}
	if !called || gotX != 7 || gotY != 9 {
		t.Errorf("confirm called=%v at (%d,%d), want true at (7,9)", called, gotX, gotY)
	}
	if len(s.game.Units.Cells.At(7, 9)) != 0 {
		t.Error("MASTER marker should be removed from the cell after the intro")
	}
}
