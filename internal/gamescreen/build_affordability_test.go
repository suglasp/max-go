// build_affordability_test.go : verifies a mobile builder cannot start a build it cannot pay for.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package gamescreen

import (
	"testing"

	"github.com/suglasp/max-go/internal/assets"
	"github.com/suglasp/max-go/internal/screendata"
)

// stubbedAttribs makes an ATTRIBS slice long enough to index t, with t's build time
// set so a real raw cost is computed. The headless builderScreen leaves attribs empty,
// which would make every build cost 0 and never trip the affordability gate.
func stubbedAttribs(t assets.UnitType, turns int) []assets.UnitAttributes {
	rows := make([]assets.UnitAttributes, int(t)+1)
	rows[t].TurnsToBuild = turns
	rows[t].HitPoints = 40

	return rows
}

// TestBuilderRefusesUnaffordableBuild verifies that a mobile builder short of the building's total raw cost receives the red c529 alert and starts no build.
// C++: buildmenu.cpp - MobileBuildMenu::Build (storage < rate1*turns -> _(c529)).
func TestBuilderRefusesUnaffordableBuild(t *testing.T) {
	s, eng := builderScreen(t, assets.Engineer, 8, 8, -1, -1,
		screendata.BuildSelection{Type: assets.ADump, Rate: 1, Path: false})
	s.SetAttribs(stubbedAttribs(assets.ADump, 10)) // engineer rate1 = 2, so the full cost is 20
	eng.Storage = 10                               // only half the material it needs

	s.openBuild()

	if s.game.BuildOrderFor(eng) != nil {
		t.Fatal("engineer started a build it cannot afford; it should be refused")
	}
	if s.buildUI.buildIdx != 0 {
		t.Errorf("buildIdx = %d after a refused build, want 0 (no placement cursor armed)", s.buildUI.buildIdx)
	}
	if s.banner.msgTint != bannerTintRed {
		t.Errorf("refusal banner tint = %v, want bannerTintRed (the insufficient-material alert)", s.banner.msgTint)
	}
}

// TestBuilderStartsAffordableBuild: the same builder with storage for the whole
// building starts the build normally (the gate only blocks the unaffordable case).
func TestBuilderStartsAffordableBuild(t *testing.T) {
	s, eng := builderScreen(t, assets.Engineer, 8, 8, -1, -1,
		screendata.BuildSelection{Type: assets.ADump, Rate: 1, Path: false})
	s.SetAttribs(stubbedAttribs(assets.ADump, 10)) // full cost 20
	eng.Storage = 40                               // ample material

	s.openBuild()

	if s.game.BuildOrderFor(eng) == nil {
		t.Fatal("engineer with ample material did not start the build")
	}
}
