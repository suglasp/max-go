// gaugestagger_test.go : tests that a report row's stat gauges stay level despite the uneven pixel split across its four lines.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package menus

import "testing"

// reportGaugeBox is the icon box of report stat line row, from the same calls drawStatBlock makes for a row.
func reportGaugeBox(row int) int {
	top, bottom := statLineSpan(0, rpRowBodyH, row)

	return statGaugeBoxH(bottom - top)
}

// A report row must draw its icons on one level. The reference calculation uses an icon box one pixel shorter than the panel
// (`reportstats.cpp: height - 1 - image->height`).
func TestReportGaugeRowsDoNotStagger(t *testing.T) {
	// Sprite heights as MAX.RES actually stores them.
	const (
		hitsH  = 9  // SI_HITSB 6x9
		powerH = 7  // SI_POWER 7x7
		workH  = 9  // SI_WORK 8x9
		rawH   = 10 // SI_RAW 7x10
		ammoH  = 7  // SI_AMMO 5x7
		speedH = 7  // SI_SPEED 7x7
	)

	// A power station reads HITS / POWER / TOTAL / USAGE, and every one of them holds
	// more than five icons, so all four are past the stagger threshold.
	for _, tc := range []struct {
		name  string
		row   int
		iconH int
		count int
	}{
		{"hits", 0, hitsH, 14},
		{"power", 1, powerH, 6},
		{"total", 2, powerH, 14},
		{"usage", 3, powerH, 9},
		{"ammo", 1, ammoH, 10},
		{"speed", 2, speedH, 8},
		{"cargo raw", 1, rawH, 15},
		{"complex pool", 2, workH, 25},
	} {
		if got := gaugeStagger(reportGaugeBox(tc.row), tc.iconH, tc.count); got != 0 {
			t.Errorf("%s: stagger = %d, want 0 (the row should be level)", tc.name, got)
		}
	}
}

// The panel keeps its stagger: an 18px row has the headroom the report's rows do not.
func TestPanelGaugeStillStaggers(t *testing.T) {
	if got := gaugeStagger(gaugeRowH, 7, 6); got != 2 {
		t.Errorf("panel stagger = %d, want 2", got)
	}

	// Under six icons never staggers, however much room the row has.
	if got := gaugeStagger(gaugeRowH, 7, 5); got != 0 {
		t.Errorf("stagger with 5 icons = %d, want 0", got)
	}

	// An icon taller than its box would step upward; it clamps flat instead.
	if got := gaugeStagger(8, 20, 10); got != 0 {
		t.Errorf("stagger with an oversized icon = %d, want 0", got)
	}
}
