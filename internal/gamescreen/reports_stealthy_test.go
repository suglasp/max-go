// reports_stealthy_test.go : covers the Stealthy flag the Units report rows carry.
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

// TestReportDataFlagsStealthUnits checks that reportData marks the Submarine and the
// Commando rows as stealthy and leaves a Tank row alone.
func TestReportDataFlagsStealthUnits(t *testing.T) {
	s, g := newTestScreen(t, 8)
	t4ArtlessScreen(s)

	mkUnit(g, assets.Submarne, 0, 1, 1, 1, game.FlagSelectable|game.FlagMobileSeaUnit)
	mkUnit(g, assets.Commando, 0, 2, 2, 1, game.FlagSelectable|game.FlagMobileLandUnit)
	mkUnit(g, assets.Tank, 0, 3, 3, 1, game.FlagSelectable|game.FlagMobileLandUnit)

	want := map[uint16]bool{1: true, 2: true, 3: false}
	rows := s.reportData().Units

	if len(rows) != len(want) {
		t.Fatalf("report has %d rows, want %d", len(rows), len(want))
	}

	for _, r := range rows {
		if r.Stealthy != want[r.UnitID] {
			t.Errorf("unit %d (%s): Stealthy = %v, want %v", r.UnitID, r.Name, r.Stealthy, want[r.UnitID])
		}
	}
}
