// deployunit_ammo_test.go : tests units deployed at game start coming in at full ATTRIB_AMMO.
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

// TestDeployUnitUsesRealAmmoWhenAttribsLoaded checks the start / random / custom
// deploy path gives a unit its TRUE ammo and rounds (the same a factory produced
// unit gets), not the old placeholder few. This was the "produced Escort full,
// starting Escort few" bug: deployUnit hardcoded Ammo=6 / Rounds=1.
func TestDeployUnitUsesRealAmmoWhenAttribsLoaded(t *testing.T) {
	s := newResearchScreen()
	rows := make([]assets.UnitAttributes, assets.UnitCount)
	rows[assets.Tank] = assets.UnitAttributes{HitPoints: 30, Ammo: 9, Rounds: 3, Speed: 16, Range: 5, Attack: 12, Armor: 4, Scan: 4}
	s.SetAttribs(rows)

	u := s.deployUnit(assets.Tank, game.FlagMobileLandUnit, 0, 0, 5, 5)

	if u.Ammo != 9 {
		t.Errorf("deployed Tank Ammo = %d, want 9 (full ATTRIB_AMMO)", u.Ammo)
	}

	if u.Shots != 3 {
		t.Errorf("deployed Tank Shots = %d, want 3 (ATTRIB_ROUNDS)", u.Shots)
	}

	// Parity with the factory path: identical to the values a produced unit's
	// makeUnit assigns through teamValues + InitState.
	if want := s.teamValues(0, assets.Tank).Get(game.AttribAmmo); u.Ammo != want {
		t.Errorf("deployed Ammo %d != produced (teamValues) Ammo %d", u.Ammo, want)
	}

	if want := s.teamValues(0, assets.Tank).Get(game.AttribHits); u.Hits != want {
		t.Errorf("deployed Hits %d != produced Hits %d", u.Hits, want)
	}
}

// TestDeployUnitPlaceholderWhenNoAttribs checks the headless bench fallback still
// seeds placeholder combat stats (no ATTRIBS loaded), so the -emulate benches and
// tests with no values keep a unit that can move and fire.
func TestDeployUnitPlaceholderWhenNoAttribs(t *testing.T) {
	s := newResearchScreen() // no SetAttribs: the headless bench path
	u := s.deployUnit(assets.Tank, game.FlagMobileLandUnit, 0, 0, 5, 5)

	if u.Ammo != 6 || u.Shots != 1 {
		t.Errorf("bench Tank Ammo/Shots = %d/%d, want 6/1 (placeholder)", u.Ammo, u.Shots)
	}

	if u.Hits != 24 {
		t.Errorf("bench Tank Hits = %d, want 24 (placeholder)", u.Hits)
	}
}
