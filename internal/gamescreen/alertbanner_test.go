// alertbanner_test.go : tests the 3-tint message banner plumbing (dark / world / red).
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package gamescreen

import "testing"

// TestAlertBanner verifies the banner tint plumbing through bannerText: a red
// alert marks the banner red, a world tint info message marks it type 1, and a
// normal banner is dark. The LUTs themselves are covered by the gfx tints; this
// locks the type plumbing.
func TestAlertBanner(t *testing.T) {
	s := &Screen{turnFlow: turnFlowState{handover: -1}} // no game over / site select / handover banner in play

	s.showAlertBanner("Too close to the enemy.")
	text, ok, tint := s.bannerText()
	if !ok || text != "Too close to the enemy." || tint != bannerTintRed {
		t.Fatalf("after showAlertBanner: bannerText = (%q, %v, %v), want a red alert", text, ok, tint)
	}

	// A world tint info message is type 1 (enemy spotted / status).
	s.showInfo("Enemy detected!")
	if text, ok, tint = s.bannerText(); !ok || text != "Enemy detected!" || tint != bannerTintWorld {
		t.Errorf("after showInfo: bannerText = (%q, %v, %v), want a world tint", text, ok, tint)
	}

	// A normal banner is dark (type 0).
	s.showBanner("Unit ready.")
	if text, ok, tint = s.bannerText(); !ok || text != "Unit ready." || tint != bannerTintDark {
		t.Errorf("after showBanner: bannerText = (%q, %v, %v), want a dark banner", text, ok, tint)
	}
}

// TestShowInfoLines: the multi line power shutdown banner joins its lines with a
// newline and shows as a world tint (type 1) banner.
func TestShowInfoLines(t *testing.T) {
	s := &Screen{turnFlow: turnFlowState{handover: -1}}

	s.showInfoLines([]string{"Research Centre shut down.", "Eco-sphere shut down."})
	text, ok, tint := s.bannerText()
	if !ok || tint != bannerTintWorld {
		t.Fatalf("showInfoLines should show a world-tint banner, got ok=%v tint=%v", ok, tint)
	}
	if text != "Research Centre shut down.\nEco-sphere shut down." {
		t.Errorf("banner text = %q, want the two lines joined by a newline", text)
	}

	// An empty list shows nothing.
	s2 := &Screen{turnFlow: turnFlowState{handover: -1}}
	s2.showInfoLines(nil)
	if _, ok, _ := s2.bannerText(); ok {
		t.Error("showInfoLines(nil) must not raise a banner")
	}
}

// TestWorldTintParams verifies the type-1 LUT levels aware of world type: red 63 always, with
// green/blue 0 on a SNOW world (a tint shaded red) and 63 elsewhere. C++:
// World::GetWorldTintParameters.
func TestWorldTintParams(t *testing.T) {
	snow := &Screen{mapName: "SNOW_3.WRL"}
	if r, g, b := snow.worldTintParams(); r != 63 || g != 0 || b != 0 {
		t.Errorf("snow world tint = (%d,%d,%d), want (63,0,0)", r, g, b)
	}
	green := &Screen{mapName: "GREEN_1.WRL"}
	if r, g, b := green.worldTintParams(); r != 63 || g != 63 || b != 63 {
		t.Errorf("non-snow world tint = (%d,%d,%d), want (63,63,63)", r, g, b)
	}
}
