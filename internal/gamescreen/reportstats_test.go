// reportstats_test.go : covers the Units report stat block: which rows a unit gets,
// the gauge factor cap, and the display name.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package gamescreen

import (
	"testing"

	"github.com/suglasp/max-go/internal/assets"
	"github.com/suglasp/max-go/internal/game"
	"github.com/suglasp/max-go/internal/screendata"
)

// statLabels lists the non empty rows of a block, in order, so a test can say what
// the unit is expected to show without caring about the blank slots.
func statLabels(rows []screendata.ReportStatRow) []string {
	var out []string

	for _, r := range rows {
		if r.Base > 0 {
			out = append(out, r.Label)
		}
	}

	return out
}

// findStat returns the row with the given label.
func findStat(t *testing.T, rows []screendata.ReportStatRow, label string) screendata.ReportStatRow {
	t.Helper()

	for _, r := range rows {
		if r.Label == label && r.Base > 0 {
			return r
		}
	}

	t.Fatalf("no %q row in the stat block (got %v)", label, statLabels(rows))

	return screendata.ReportStatRow{}
}

// TestReportStatsArmedUnitShowsAmmoAndShots verifies that an armed unit reports ammo and shots, using the row that carries cargo for an unarmed unit.
func TestReportStatsArmedUnitShowsAmmoAndShots(t *testing.T) {
	s, g := newClickScreen(t)
	g.State.PlayerTeam = 0

	u := mkUnit(g, assets.Tank, 51, 4, 4, game.FlagMobileLandUnit|game.FlagSelectable)
	u.Values.Set(game.AttribHits, 24)
	u.Values.Set(game.AttribAttack, 10)
	u.Values.Set(game.AttribAmmo, 14)
	u.Values.Set(game.AttribRounds, 2)
	u.Values.Set(game.AttribSpeed, 6)
	u.Hits, u.Ammo, u.Shots, u.Speed = 24, 14, 2, 6

	rows := s.reportStats(u)

	if got := statLabels(rows); len(got) != 4 {
		t.Fatalf("an armed unit fills all four rows, got %v", got)
	}

	ammo := findStat(t, rows, s.lang.Get("3347", "AMMO"))
	if ammo.Cur != 14 || ammo.Base != 14 {
		t.Errorf("ammo row = %d/%d, want 14/14", ammo.Cur, ammo.Base)
	}

	// Shots are capped by the ammo actually carried, not the rounds alone.
	shots := findStat(t, rows, s.lang.Get("27e1", "SHOTS"))
	if shots.Base != 2 {
		t.Errorf("shots base = %d, want 2 (min of rounds and ammo)", shots.Base)
	}

	// The hits row separates itself from the next with a hairline; the last row does not.
	if !rows[0].Line {
		t.Error("the hits row must draw its hairline")
	}
}

// TestReportStatsUnarmedUnitShowsCargo verifies that an unarmed unit shows cargo in the ammo row.
func TestReportStatsUnarmedUnitShowsCargo(t *testing.T) {
	s, g := newClickScreen(t)
	g.State.PlayerTeam = 0

	u := mkUnit(g, assets.Constrct, 51, 4, 4, game.FlagMobileLandUnit|game.FlagSelectable)
	u.Values.Set(game.AttribHits, 24)
	u.Values.Set(game.AttribAttack, 0)
	u.Values.Set(game.AttribStorage, 60)
	u.Values.Set(game.AttribSpeed, 6)
	u.Hits, u.Storage, u.Speed = 24, 60, 6

	rows := s.reportStats(u)

	cargo := findStat(t, rows, s.lang.Get("199c", "CARGO"))
	if cargo.Cur != 60 || cargo.Base != 60 {
		t.Errorf("cargo row = %d/%d, want 60/60", cargo.Cur, cargo.Base)
	}

	// A 60 point hold at one icon per point would be 60 icons. The factor groups them
	// so the gauge stays inside its column.
	if icons := (cargo.Base + cargo.Factor - 1) / cargo.Factor; icons > reportIconCap {
		t.Errorf("cargo gauge would draw %d icons, over the %d cap", icons, reportIconCap)
	}

	for _, label := range statLabels(rows) {
		if label == s.lang.Get("3347", "AMMO") {
			t.Error("an unarmed unit must not show an ammo row")
		}
	}
}

// TestReportStatsFactorCapsTheGauge: a huge stat still fits, because the factor is
// rescaled until the icon count is under the cap.
func TestReportStatsFactorCapsTheGauge(t *testing.T) {
	s, g := newClickScreen(t)
	g.State.PlayerTeam = 0

	u := mkUnit(g, assets.Tank, 51, 4, 4, game.FlagMobileLandUnit|game.FlagSelectable)
	u.Values.Set(game.AttribHits, 500) // far more than the cap of icons
	u.Hits = 500

	rows := s.reportStats(u)
	hits := findStat(t, rows, s.lang.Get("891f", "HITS"))

	if icons := (hits.Base + hits.Factor - 1) / hits.Factor; icons > reportIconCap {
		t.Errorf("hits gauge would draw %d icons, over the %d cap", icons, reportIconCap)
	}
}

// TestHitsGaugeTierFollowsDamage: the hits icon changes colour as the unit is beaten
// down, which is why the host picks the sprite rather than the menu. Blue while it is
// over half, yellow past that, red once it is down to a quarter.
func TestHitsGaugeTierFollowsDamage(t *testing.T) {
	for _, tc := range []struct {
		name     string
		cur, max int
		want     int
	}{
		{"undamaged", 20, 20, 0},
		{"just over half", 11, 20, 0},
		{"exactly half", 10, 20, 1},
		{"just over a quarter", 6, 20, 1},
		{"exactly a quarter", 5, 20, 2},
		{"nearly dead", 1, 20, 2},
	} {
		if got := hitsGaugeTier(tc.cur, tc.max); got != tc.want {
			t.Errorf("%s (%d/%d): tier %d, want %d", tc.name, tc.cur, tc.max, got, tc.want)
		}
	}
}

// TestReportDisplayName: the report names a unit the way the original does, with the
// mark it was built at in roman numerals and its number among its own kind.
func TestReportDisplayName(t *testing.T) {
	_, g := newClickScreen(t)

	u := mkUnit(g, assets.Tank, 51, 4, 4, game.FlagMobileLandUnit|game.FlagSelectable)
	u.Values.SetVersion(1)

	if got, want := reportDisplayName(u, 3), "Mk I "+displayName(assets.Tank)+" 3"; got != want {
		t.Errorf("display name = %q, want %q", got, want)
	}

	u.Values.SetVersion(4)

	if got, want := reportDisplayName(u, 1), "Mk IV "+displayName(assets.Tank)+" 1"; got != want {
		t.Errorf("an upgraded unit = %q, want %q", got, want)
	}
}

// TestRomanNumeral covers the mark levels a unit can actually reach.
func TestRomanNumeral(t *testing.T) {
	for _, tc := range []struct {
		n    int
		want string
	}{
		{1, "I"}, {2, "II"}, {3, "III"}, {4, "IV"}, {5, "V"},
		{9, "IX"}, {10, "X"}, {14, "XIV"}, {40, "XL"},
	} {
		if got := romanNumeral(tc.n); got != tc.want {
			t.Errorf("romanNumeral(%d) = %q, want %q", tc.n, got, tc.want)
		}
	}
}
