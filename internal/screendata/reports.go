// reports.go : carries the Reports screen data transfer objects.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
// C++ ported : reportmenu.cpp / reportstats.cpp - the ReportMenu data snapshot

// Package screendata holds data transfer objects exchanged across the gamescreen/menus boundary. Both packages import this leaf package, so the
// host can pass values through without converters.
package screendata

import "github.com/suglasp/max-go/internal/gfx"

// ReportStatRow is one line of a Units row's stat block: the "cur/base" readout,
// its label, and a run of gauge icons. The host fills these in because the values
// come from the sim (and, for a building, from its whole complex); the menus
// package only draws them. Four rows fit the block, and a row with Base 0 is left
// blank.
//
// C++ ported : reportstats.cpp - one ReportStats_DrawRowEx call
type ReportStatRow struct {
	Label     string      // HITS, AMMO, CARGO, SPEED, SHOTS, POWER, USAGE, TOTAL, WORK
	Cur, Base int         // the readout, and what it is measured against
	Factor    int         // value units per gauge icon
	Full      *gfx.Buffer // filled icon; the host picks the HITS colour variant
	Empty     *gfx.Buffer // depleted icon, drawn for the remainder up to Base
	Line      bool        // hairline under the row
}

// ReportRow is one unit's row in the Units report, with the category flags the
// filter checkboxes test.
type ReportRow struct {
	UnitID        uint16          // the unit behind the row: clicking it selects that unit on the map
	Name          string          // display name, "Mk <roman> <type> <index>"
	Coords        string          // "x,y" grid position
	Status        string          // order/status text (Awaiting / Sentry / Power On / Building ...)
	Icon          *gfx.Buffer     // 32x32 team coloured icon, or nil
	Stats         []ReportStatRow // the stat block, at most four rows
	Hits, MaxHits int
	TeamColor     int
	Air           bool
	Land          bool
	Sea           bool
	Stationary    bool
	Producer      bool
	Combat        bool
}

// ReportTeam is one team's line for the Score header (and Casualties columns).
type ReportTeam struct {
	Name         string
	Color        int
	Points       int
	Ecospheres   int
	ScoreHistory [50]int16 // ring buffer: index 49 = newest turn
}

// ReportMessage is one Messages tab row: an optional icon (the unit the message
// refers to, or a turn banner icon), an optional red grid coordinate, and the
// text (may wrap to two lines). C++: a MessageLogEntry rendered with
// ReportStats_DrawListItemIcon + the coordinate + the message string.
type ReportMessage struct {
	Icon   *gfx.Buffer
	Coords string
	Text   string
}

// ReportCasualty is one row of the Casualties tab: a unit type somebody has lost,
// and how many of it each team lost (Counts is aligned with the Teams slice, one
// entry per column). The host leaves out any type nobody has lost.
//
// C++ ported : reportmenu.cpp - ReportMenu::DrawCasualties (a row per unit_types
// entry, a column per team, each cell CTInfo.casualties[type])
type ReportCasualty struct {
	Icon   *gfx.Buffer
	Name   string
	Counts []int
}

// ReportData is the whole screen snapshot the host passes in.
type ReportData struct {
	Units      []ReportRow
	Teams      []ReportTeam     // teams in play, for the Score + Casualties tabs
	Messages   []ReportMessage  // game message log (newest last) for the Messages tab
	Casualties []ReportCasualty // per unit type losses, one column per team
}
