// reports_test.go : tests report filtering, paging, and status colors.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package menus

import (
	"testing"

	"github.com/suglasp/max-go/internal/screendata"
)

// TestReportRowMatches covers the Units filter predicate: location filters are
// inclusive, attribute filters restrictive.
func TestReportRowMatches(t *testing.T) {
	tank := screendata.ReportRow{Land: true, Combat: true, Hits: 10, MaxHits: 24}            // damaged combat land unit
	gen := screendata.ReportRow{Stationary: true, Hits: 40, MaxHits: 40}                     // intact building
	factory := screendata.ReportRow{Stationary: true, Producer: true, Hits: 50, MaxHits: 50} // producer building
	jet := screendata.ReportRow{Air: true, Combat: true, Hits: 8, MaxHits: 8}                // intact combat air unit

	// Default filters: all four locations on, attributes off - everything shows.
	allLoc := func(r screendata.ReportRow) bool {
		return reportRowMatches(r, true, true, true, true, false, false, false, false)
	}
	for _, r := range []screendata.ReportRow{tank, gen, factory, jet} {
		if !allLoc(r) {
			t.Errorf("default filters should show %+v", r)
		}
	}

	// Location is inclusive: with only Air on, the land tank is hidden, the jet shows.
	if reportRowMatches(tank, true, false, false, false, false, false, false, false) {
		t.Error("Air-only should hide a land unit")
	}
	if !reportRowMatches(jet, true, false, false, false, false, false, false, false) {
		t.Error("Air-only should show an air unit")
	}

	// Production restricts to producers.
	if reportRowMatches(gen, true, true, true, true, true, false, false, false) {
		t.Error("Production filter should hide a non-producer building")
	}
	if !reportRowMatches(factory, true, true, true, true, true, false, false, false) {
		t.Error("Production filter should show a producer")
	}

	// Combat restricts to attackers; Damaged to under max hits.
	if reportRowMatches(gen, true, true, true, true, false, true, false, false) {
		t.Error("Combat filter should hide a non-combat building")
	}
	if reportRowMatches(jet, true, true, true, true, false, false, true, false) {
		t.Error("Damaged filter should hide a full-health unit")
	}
	if !reportRowMatches(tank, true, true, true, true, false, true, true, false) {
		t.Error("Combat+Damaged should show a damaged combat unit")
	}

	// Stealthy excludes everything (no stealth model yet).
	if reportRowMatches(tank, true, true, true, true, false, false, false, true) {
		t.Error("Stealthy filter should currently exclude all units")
	}
}

// TestReportRowAtSelectsTheClickedUnit checks the Units list click: the row under the
// cursor resolves to its unit, so the caller can select it on the map. The other tabs
// are readouts and never resolve a unit.
func TestReportRowAtSelectsTheClickedUnit(t *testing.T) {
	m := &reportMenu{
		activeType:  reportUnits,
		rowsPerPage: rpListH / rpRowH,
		filters: []reportFilter{
			{"", true, 0}, {"", true, 0}, {"", true, 0}, {"", true, 0},
			{"", false, 0}, {"", false, 0}, {"", false, 0}, {"", false, 0},
		},
		data: screendata.ReportData{Units: []screendata.ReportRow{
			{UnitID: 11, Land: true},
			{UnitID: 22, Land: true},
			{UnitID: 33, Land: true},
		}},
	}

	// A click inside the second row hands back the second unit.
	if id, ok := m.rowAt(rpListY + rpRowH + 5); !ok || id != 22 {
		t.Errorf("click on row 2 = (%d, %v), want (22, true)", id, ok)
	}

	// Scrolled down by one, the same row is now the third unit.
	m.scroll = 1
	if id, ok := m.rowAt(rpListY + rpRowH + 5); !ok || id != 33 {
		t.Errorf("scrolled click on row 2 = (%d, %v), want (33, true)", id, ok)
	}

	// A row past the end of the list resolves nothing.
	if _, ok := m.rowAt(rpListY + 7*rpRowH); ok {
		t.Error("a click on an empty row resolved a unit")
	}

	// The other tabs never resolve a unit.
	m.scroll = 0
	m.activeType = reportScore
	if _, ok := m.rowAt(rpListY + 5); ok {
		t.Error("a click on the Score tab resolved a unit")
	}
}
