// loadzoom_test.go : tests zoom level restore on game save/load.
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

// TestFocusOnLoadedBaseZoomsIn verifies the load focus: from a wide view a load zooms
// partly IN to the base (landingStep>0, landingZoom=loadFocusZoom) WITHOUT unfolding again
// the already built base; a load already zoomed in just stays centred (no animation).
func TestFocusOnLoadedBaseZoomsIn(t *testing.T) {
	s, g := newClickScreen(t)
	s.r.SetZoom(16)
	mkUnit(g, assets.MiningSt, 1, 8, 8, game.FlagStationary|game.FlagBuilding|game.FlagSelectable)

	s.focusOnLoadedBase()
	if s.camera.landingStep <= 0 {
		t.Error("a load from a wide zoom should start a zoom-in (landingStep>0)")
	}
	if s.camera.landingZoom != loadFocusZoom {
		t.Errorf("landingZoom = %d, want %d", s.camera.landingZoom, loadFocusZoom)
	}
	if !s.camera.landingSmooth {
		t.Error("the post-load focus must use the smooth linear ramp (landingSmooth)")
	}
	// The linear ramp spans more than a single doubling step (so it glides, not snaps).
	if s.camera.landingStep < 4 {
		t.Errorf("smooth ramp landingStep = %d, want a multi-step glide", s.camera.landingStep)
	}
	for _, u := range g.Units.Units() {
		if u.Unfold != 0 {
			t.Errorf("load focus must NOT re-unfold %s (Unfold=%d)", u.Type.Tag(), u.Unfold)
		}
	}

	s2, g2 := newClickScreen(t)
	s2.r.SetZoom(64) // already fully zoomed in
	mkUnit(g2, assets.MiningSt, 1, 8, 8, game.FlagStationary|game.FlagBuilding|game.FlagSelectable)
	s2.focusOnLoadedBase()
	if s2.camera.landingStep != 0 {
		t.Errorf("an already zoomed-in load should not animate, landingStep=%d", s2.camera.landingStep)
	}
}

// TestCenterOnPlayerBaseIsStable verifies the "view goes all over the
// place" load zoom bug: recentring each frame must be INDEPENDENT OF ORDER. With only
// spread out mobile units (the fallback path with no buildings), the old "first unit in
// random map order" pick jumped the view every frame; the lowest ID fallback is stable.
func TestCenterOnPlayerBaseIsStable(t *testing.T) {
	s, g := newClickScreen(t)
	s.r.SetZoom(16)
	for i, c := range [][2]int{{2, 2}, {14, 14}, {2, 14}, {14, 2}} {
		mkUnit(g, assets.Tank, i+1, c[0], c[1], game.FlagMobileLandUnit|game.FlagSelectable)
	}
	s.centerOnPlayerBase()
	x1, y1 := s.camera.scrollX, s.camera.scrollY
	for i := 0; i < 12; i++ {
		s.centerOnPlayerBase()
		if s.camera.scrollX != x1 || s.camera.scrollY != y1 {
			t.Fatalf("centerOnPlayerBase not stable across calls: (%d,%d) vs (%d,%d) - random-order pick?",
				s.camera.scrollX, s.camera.scrollY, x1, y1)
		}
	}
}
