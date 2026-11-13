// reports.go : implements the in game reports overlay.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package gamescreen

import (
	"fmt"
	"sort"

	"github.com/suglasp/max-go/internal/assets"
	"github.com/suglasp/max-go/internal/dbg"
	"github.com/suglasp/max-go/internal/game"
	"github.com/suglasp/max-go/internal/gfx"
	"github.com/suglasp/max-go/internal/screendata"
)

// Reports uses screendata snapshots of player units and team scores for the REP_FRME menu. C++: ReportMenu.

// SetReportsFunc installs the hook the REPORTS button calls to open the Reports
// screen (wired in cmd to menus.RunReports with the resolved data). The hook hands back
// the id of a unit the player clicked in the list, or 0 if they just closed the screen.
func (s *Screen) SetReportsFunc(fn func(data screendata.ReportData) (uint16, error)) {
	s.hooks.reportsFn = fn
}

// openReports runs the report hook. Selecting a Units-row entry selects that unit and centres the map on it.
//
// C++ ported : reportmenu.cpp - ReportMenu::SelectUnit (the row click selects the unit
// and moves the view to it)
func (s *Screen) openReports() {
	if s.hooks.reportsFn == nil {
		return
	}

	id, err := s.hooks.reportsFn(s.reportData())
	if err != nil {
		dbg.Printf("reports: %v", err)

		return
	}

	if id == 0 {
		return
	}

	for _, u := range s.game.Units.Units() {
		if u.ID != id || u.Team != s.game.State.PlayerTeam {
			continue
		}

		s.selectUnit(u)
		s.centerOnSelected()

		return
	}
}

// SetHelpFunc installs the context help hook (wired to menus.Help).
func (s *Screen) SetHelpFunc(fn func(section string)) {
	s.hooks.helpFn = fn
}

// SetVoiceFunc installs the announcer voice hook (wired to audio.PlayVoice).
func (s *Screen) SetVoiceFunc(fn func(tag string)) {
	s.announcer.voiceFn = fn
}

// SetVoicePlayingFunc installs the "is a voice still audible" query (wired to
// audio.VoicePlaying), so the voice queue plays the next line only once the
// previous one has finished. Nil falls back to a fixed clip length spacing.
func (s *Screen) SetVoicePlayingFunc(fn func() bool) {
	s.announcer.voicePlayingFn = fn
}

// SetMusicFunc installs the music hook (wired to audio.PlayMusic).
func (s *Screen) SetMusicFunc(fn func(tag string, loop bool)) {
	s.hooks.musicFn = fn
}

// SetNoMovie suppresses win/lose mission movie playback (wired from -nomovie).
func (s *Screen) SetNoMovie(skip bool) {
	s.noMovie = skip
}

// SetMovieFunc installs the hook that plays an MVE by resource tag: the landing cinematic when the player
// confirms the landing site.
func (s *Screen) SetMovieFunc(fn func(tag string) error) {
	s.hooks.movieFn = fn
}

/* // note: unported C++ code (mission victory / defeat story screen)
// nothing feeds a per mission victory movie; vanilla has none.
// SetWinMovieTag sets the MVE resource tag played on victory (mission specific;
// empty = no movie).
func (s *Screen) SetWinMovieTag(tag string) {
	s.winMovieTag = tag
}
*/

/* // note: unported C++ code (mission victory / defeat story screen)
// nothing feeds a per mission defeat movie; vanilla has none.
// SetLoseMovieTag sets the MVE resource tag played on defeat (mission specific;
// empty = no movie).
func (s *Screen) SetLoseMovieTag(tag string) {
	s.loseMovieTag = tag
}
*/

// SetLandingMovieTag sets the intro cinematic played when the player confirms the
// landing site (the custom game MAXMVE1; empty = none), through the movie hook.
func (s *Screen) SetLandingMovieTag(tag string) {
	s.landingMovieTag = tag
}

// colonyVoice maps a team to its "<colour> team colony started!" voice resource
// (loose F0##.WAV via the V_F### redirect): red/green/blue/gray = team 0..3.
var colonyVoice = [game.TeamCount]string{"V_F098", "V_F103", "V_F108", "V_F113"}

// checkColonyStarted announces another team's first operating Eco Sphere with a colour-specific voice, banner, and message. The player's own colony is not announced.
//
// C++ ported : unitinfo.cpp UnitInfo::UpdateProduction - the GREENHSE colony rating
// voice (V_M098/F098 + team*2), gated GameManager_PlayerTeam != team.
func (s *Screen) checkColonyStarted() {
	for t := 0; t < game.TeamCount; t++ {
		// Skip your own colony (announced only for other/enemy teams), already announced
		// teams, and teams with no working Eco Sphere yet.
		if t == s.game.State.PlayerTeam || s.announcer.colonyAnnounced[t] || s.workingEcospheres(t) == 0 {
			continue
		}

		s.announcer.colonyAnnounced[t] = true

		if s.announcer.voiceFn != nil {
			s.announcer.voiceFn(colonyVoice[t])
		}

		msg := fmt.Sprintf("%s team colony started!", s.teamName(t))
		s.logEvent(s.iconForType(assets.Greenhse, t), "", msg)
		s.showBanner(msg) // the player sees it too (intel that an opponent is colonizing)
	}
}

// checkColonyStopped announces when a team's last operating Eco Sphere stops. The sim emits the edge once, and clearing the latch allows a later start announcement.
//
// C++ ported : the colony rating loss when a team's GREENHSE production ends
// (unitinfo.cpp UnitInfo::UpdateProduction / the economy report).
func (s *Screen) checkColonyStopped(team int) {
	if team < 0 || team >= game.TeamCount {
		return
	}

	s.announcer.colonyAnnounced[team] = false // a restart will announce again the colony start
	msg := fmt.Sprintf("%s team colony stopped!", s.teamName(team))
	s.logEvent(s.iconForType(assets.Greenhse, team), "", msg)
	s.showBanner(msg)
}

// openHelp shows the help blurb for a screen section via the hook.
func (s *Screen) openHelp(section string) {
	if s.hooks.helpFn != nil {
		s.hooks.helpFn(section)
	}
}

// producerTypes are the unit types the Production filter keeps: the factories
// that build units plus the mobile builders. C++: the "production" active_units set.
var producerTypes = map[assets.UnitType]bool{
	assets.LandPlt: true, assets.AirPlt: true, assets.Shipyard: true,
	assets.LightPlt: true, assets.TrainHal: true,
	assets.Constrct: true, assets.Engineer: true,
}

// reportData builds the Reports snapshot: the player's units as rows + a team
// score line per team in play.
func (s *Screen) reportData() screendata.ReportData {
	team := s.game.State.PlayerTeam
	var mine []*game.UnitInfo

	for _, u := range s.game.Units.Units() {
		if u.Team == team && u.Flags.Has(game.FlagSelectable) {
			mine = append(mine, u)
		}
	}

	// Group the list by unit type, and order each type by the id it was built with.
	// Units() hands back map order, so without this the report shuffles between
	// openings. C++: ReportMenu::AddUnits sorts on (unit type, unit id).
	sort.Slice(mine, func(a, b int) bool {
		if mine[a].Type != mine[b].Type {
			return mine[a].Type < mine[b].Type
		}

		return mine[a].ID < mine[b].ID
	})

	var rows []screendata.ReportRow
	seen := make(map[assets.UnitType]int) // how many of each type named so far

	for _, u := range mine {
		maxHits, combat := 0, false

		if u.Values != nil {
			maxHits = u.Values.Get(game.AttribHits)
			combat = u.Values.Get(game.AttribAttack) > 0
		}

		seen[u.Type]++

		rows = append(rows, screendata.ReportRow{
			UnitID: u.ID,
			Name:   reportDisplayName(u, seen[u.Type]),
			// The original counts grid cells from one, so a unit in the top left
			// corner reads 1,1 rather than 0,0.
			Coords: fmt.Sprintf("%d,%d", u.GridX+1, u.GridY+1),
			Status: s.unitStatusText(u),
			Icon:   s.unitIcon(u),
			Stats:  s.reportStats(u),
			Hits:   u.Hits, MaxHits: maxHits, /* TeamColor: 0xFF: note: unused code in Go port, see ReportRow */
			Air:        u.Flags.Has(game.FlagMobileAirUnit),
			Land:       u.Flags.Has(game.FlagMobileLandUnit),
			Sea:        u.Flags.Has(game.FlagMobileSeaUnit),
			Stationary: u.Flags.Has(game.FlagStationary),
			Producer:   producerTypes[u.Type],
			Combat:     combat,
			// The reference keeps only these two types when the Stealthy box is ticked.
			Stealthy: u.Type == assets.Submarne || u.Type == assets.Commando,
		})
	}

	st := s.game.Status()

	var (
		teams     []screendata.ReportTeam
		teamOrder []int // the team indices behind the Teams slice, for the casualty columns
	)

	for t := 0; t < game.TeamCount; t++ {
		if st.TeamType[t] == game.TeamNone {
			continue
		}

		teams = append(teams, screendata.ReportTeam{
			Name:         s.teamName(t),
			Color:        reportTeamColor(t),
			Points:       s.game.TeamPoints[t],
			Ecospheres:   s.workingEcospheres(t),
			ScoreHistory: s.game.ScoreHistory[t],
		})
		teamOrder = append(teamOrder, t)
	}

	// Messages NEWEST FIRST: the log is kept newest last, but the Messages tab opens at
	// the top of the list, so a just posted event (a build finish, an auto powered
	// generator, a shed warning) must sit at the top to be seen without scrolling down a
	// long history. C++: ReportStats lists the most recent message first.
	var msgs []screendata.ReportMessage

	for i := len(s.banner.msgLog) - 1; i >= 0; i-- {
		m := s.banner.msgLog[i]
		msgs = append(msgs, screendata.ReportMessage{Icon: m.icon, Coords: m.coords, Text: m.text})
	}

	return screendata.ReportData{Units: rows, Teams: teams, Messages: msgs, Casualties: s.casualtyRows(teamOrder)}
}

// casualtyRows builds the Casualties tab: one row per unit type ANY team has lost, with
// that type's icon, its name, and the count each team lost of it (one entry per column,
// in the same order as the Teams slice). A type nobody has lost is left out entirely, so
// an early game report is short and a long war fills the page.
//
// C++ ported : reportmenu.cpp - ReportMenu::DrawCasualties + the unit_types list in
// UpdateStatistics (a type is listed only when some team's casualties[type] is nonzero)
func (s *Screen) casualtyRows(teamOrder []int) []screendata.ReportCasualty {
	lost := map[assets.UnitType]bool{}

	for _, t := range teamOrder {
		for unitType, n := range s.game.TeamStats[t].Casualties {
			if n > 0 {
				lost[unitType] = true
			}
		}
	}

	types := make([]assets.UnitType, 0, len(lost))
	for unitType := range lost {
		types = append(types, unitType)
	}

	sort.Slice(types, func(a, b int) bool { return types[a] < types[b] })

	rows := make([]screendata.ReportCasualty, 0, len(types))

	for _, unitType := range types {
		counts := make([]int, len(teamOrder))

		for i, t := range teamOrder {
			counts[i] = s.game.TeamStats[t].Casualties[unitType]
		}

		rows = append(rows, screendata.ReportCasualty{
			// The list icon is drawn in the PLAYER's colours, like every other report row.
			Icon:   s.iconForType(unitType, s.game.State.PlayerTeam),
			Name:   displayName(unitType),
			Counts: counts,
		})
	}

	return rows
}

// iconForType renders a 32x32 team coloured icon for a unit type (for message
// rows that refer to a unit).
func (s *Screen) iconForType(t assets.UnitType, team int) *gfx.Buffer {
	const sz = 32
	buf := gfx.New(sz, sz)

	if s.sprites == nil {
		return buf // no sprite cache (headless): a blank icon
	}

	s.sprites.DrawIcon(buf, t, team, sz/2, sz/2)
	return buf
}

// reportTeamColor is the palette index the Reports screen prints a team's name
// in. C++: ReportMenu_TeamColors (red/green/blue/grey).
func reportTeamColor(team int) int {
	switch team {
	case 0:
		return int(gfx.ColorRed) // red (0x01)
	case 1:
		return int(gfx.ColorGreen) // green (0x02)
	case 2:
		return int(gfx.ColorBlue) // blue (0x03) - was 0x04 (yellow) by mistake
	default:
		return 0xA9 // grey
	}
}

// workingEcospheres counts a team's operational Eco Spheres: powered Greenhouse
// (GREENHSE) buildings. C++: ReportMenu::GetWorkingEcoSphereCount (GREENHSE with
// ORDER_POWER_ON). The Eco Sphere is the colonization building; its working
// count drives the colony rating (count*10 + team points).
func (s *Screen) workingEcospheres(team int) int {
	n := 0

	for _, u := range s.game.Units.Units() {
		if u.Team == team && u.Type == assets.Greenhse && u.Order == game.OrderPowerOn {
			n++
		}
	}

	return n
}

// unitStatusText is the status the Units report shows for one unit. A unit that halted on
// spotting an enemy still carries OrderMove (it is en route, just standing), so on the plain
// order it would read "Moving" while parked in the open. It reads "Halted" instead, which is
// what the player needs to see in the list to find the units waiting on a decision.
func (s *Screen) unitStatusText(u *game.UnitInfo) string {
	if len(s.game.HeldRoute(u)) > 0 {
		return "Halted"
	}

	return orderStatusText(u.Order)
}

// orderStatusText maps a unit order to the short status the Units report shows.
// C++: the order/state description in ReportMenu unit rows.
func orderStatusText(o game.Order) string {
	switch o {
	case game.OrderPowerOn:
		return "Power On"
	case game.OrderSentry:
		return "Sentry"
	case game.OrderBuild:
		return "Building"
	case game.OrderMove:
		return "Moving"
	default:
		return "Awaiting"
	}
}
