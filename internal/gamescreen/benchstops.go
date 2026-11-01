// benchstops.go : provides the scenario stops of the -emulate debug benches (compass anchors, Tab, banners).
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package gamescreen

import (
	"fmt"
	"strings"

	"github.com/suglasp/max-go/internal/dbg"
	"github.com/suglasp/max-go/internal/game"
	"github.com/suglasp/max-go/internal/platform"
)

// benchBannerMS is how long a bench banner holds (a normal banner holds 2.5 s); Tab shows it again.
const benchBannerMS = 15000

// benchAnchorRadius is how far from its compass anchor a scenario may be placed, so it stays in that region.
const benchAnchorRadius = 12

// benchStop is one scenario of a bench: where the camera goes, what is selected and what the banner says.
type benchStop struct {
	Compass string         // the anchor's wind direction ("north east"), empty for a single stop bench
	Title   string         // the scenario ("H13 Air Transport loads a parked builder")
	Text    string         // what to do there and what to see
	Cell    game.Point     // the cell the camera centres on
	Select  *game.UnitInfo // the unit selected on arrival, or nil
}

// benchState holds the bench's stops, the current one and the compass anchors already used, plus the banner
// on show so tickBenchBanner can bring it back once after another banner (a turn start alert) displaced it.
type benchState struct {
	stops       []benchStop
	index       int
	claimed     map[int]bool
	bannerText  string
	bannerUntil uint64
	restored    bool
	popup       string               // the start popup, opened on the first frame
	intro       string               // the bench wide line shown above stop 1's banner
	frozen      [game.TeamCount]bool // teams locked still by the bench: their turn passes at once
}

// benchAnchor is one of the nine compass points a bench scenario can be placed near.
type benchAnchor struct {
	Name string
	Cell game.Point
}

// benchCompassNames lists the anchor names by row (north to south) and column (west to east).
var benchCompassNames = [3][3]string{
	{"north west", "north", "north east"},
	{"west", "centre", "east"},
	{"south west", "south", "south east"},
}

// benchAnchors returns the nine compass anchors in reading order: a 3x3 grid at 1/6, 1/2 and 5/6 of the map.
//
// C++: n/a (debug helper).
func (s *Screen) benchAnchors() []benchAnchor {
	out := make([]benchAnchor, 0, 9)

	for row := 0; row < 3; row++ {
		for col := 0; col < 3; col++ {
			cell := game.Point{X: s.wmap.W * (1 + 2*col) / 6, Y: s.wmap.H * (1 + 2*row) / 6}
			out = append(out, benchAnchor{Name: benchCompassNames[row][col], Cell: cell})
		}
	}

	return out
}

// claimAnchor offers each free anchor to fits in reading order and claims the first one fits accepts; fits
// places the scenario near that anchor. It returns the anchor's name, or false when no free anchor fits.
//
// C++: n/a (debug helper).
func (s *Screen) claimAnchor(fits func(anchor game.Point) bool) (string, bool) {
	if s.bench.claimed == nil {
		s.bench.claimed = map[int]bool{}
	}

	for i, a := range s.benchAnchors() {
		if s.bench.claimed[i] || !fits(a.Cell) {
			continue
		}

		s.bench.claimed[i] = true

		return a.Name, true
	}

	return "", false
}

// addBenchStop appends a scenario stop; Tab visits the stops in this order.
func (s *Screen) addBenchStop(stop benchStop) {
	s.bench.stops = append(s.bench.stops, stop)
}

// benchBanner is the banner line of stop i: "<n>/<total> <COMPASS>: <Title> - <Text>", the count and the
// compass left out for a single stop bench.
func (s *Screen) benchBanner(i int) string {
	stop := s.bench.stops[i]
	text := stop.Title

	if stop.Compass != "" {
		text = strings.ToUpper(stop.Compass) + ": " + text
	}

	if n := len(s.bench.stops); n > 1 {
		text = fmt.Sprintf("%d/%d %s", i+1, n, text)
	}

	if stop.Text != "" {
		text += " - " + stop.Text
	}

	return text
}

// gotoBenchStop moves to stop i (wrapping both ways): the camera centres on it, its unit is selected and its
// banner shows. intro, when set, goes on a line above the banner.
//
// C++: n/a (debug helper).
func (s *Screen) gotoBenchStop(i int, intro string) {
	n := len(s.bench.stops)

	if n == 0 {
		return
	}

	i = (i%n + n) % n
	s.bench.index = i
	stop := s.bench.stops[i]

	vw, vh := viewSize(MainMap)
	s.camera.scrollX, s.camera.scrollY = centerScroll(stop.Cell.X, stop.Cell.Y, s.r.Zoom(), vw, vh)
	s.camera.scrollX, s.camera.scrollY = s.r.ClampScroll(s.camera.scrollX, s.camera.scrollY, vw, vh)

	if stop.Select != nil && stop.Select.Hits > 0 && s.game.Units.ByID.Get(stop.Select.ID) != nil {
		s.setSelected(stop.Select)
	}

	text := s.benchBanner(i)

	if intro != "" {
		s.logMessage(intro)
		s.logMessage(text)
		text = intro + "\n" + text
	} else {
		s.logMessage(text)
	}

	s.showBenchBanner(text)
	dbg.Printf("bench stop %d/%d %q at (%d,%d)", i+1, n, stop.Compass, stop.Cell.X, stop.Cell.Y)
}

// startBench opens the bench. First the start popup (startBenchNamed); after its OK the bench goes to stop 1,
// with intro (the bench wide checks) above that stop's banner.
//
// C++: n/a (debug helper).
func (s *Screen) startBench(intro string) {
	s.startBenchNamed(benchName(intro), intro)
}

// startSingleBench opens a one scenario bench: after the start popup the camera centres on cell, sel is
// selected and the banner shows title (Tab shows it again).
//
// C++: n/a (debug helper).
func (s *Screen) startSingleBench(title string, cell game.Point, sel *game.UnitInfo) {
	s.addBenchStop(benchStop{Title: title, Cell: cell, Select: sel})
	s.startBenchNamed(benchName(title), "")
}

// startBenchNamed queues the start popup, the general guide for whoever runs the bench (which bench, its keys,
// End Turn), for the first frame; tickBenchBanner opens it and, after OK, goes to stop 1 (openBenchStart).
// Without a message dialog (headless tests) the bench goes to stop 1 at once.
func (s *Screen) startBenchNamed(name, intro string) {
	if len(s.bench.stops) > 1 {
		intro = strings.TrimSpace(intro + " Tab = next scenario, Shift+Tab = previous.")
	}

	s.bench.intro = intro

	if s.hooks.messageFn == nil {
		s.openBenchStart()
		return
	}

	s.bench.popup = s.benchPopupText(name)
}

// openBenchStart goes to stop 1 with the intro above its banner, or shows just the intro when the bench has
// no stops.
func (s *Screen) openBenchStart() {
	if len(s.bench.stops) == 0 {
		if s.bench.intro != "" {
			s.showBenchBanner(s.bench.intro)
			s.logMessage(s.bench.intro)
		}

		return
	}

	s.gotoBenchStop(0, s.bench.intro)
}

// freezeBenchTeam locks a bench's enemy still: it plays as a human team (so the AI never moves it), and its
// turn passes at once (beginNextTeam), so End Turn comes straight back to the player with no hot seat banner.
func (s *Screen) freezeBenchTeam(team int) {
	s.game.State.Teams[team].Type = game.TeamPlayer
	s.bench.frozen[team] = true
}

// benchFrozen reports whether team is a bench's frozen enemy.
func (s *Screen) benchFrozen(team int) bool {
	return team >= 0 && team < game.TeamCount && s.bench.frozen[team]
}

// benchName is a bench's name from the start of its intro or banner: the text before the first ':' or '.',
// without a "(...)" note.
func benchName(text string) string {
	if i := strings.IndexAny(text, ":."); i >= 0 {
		text = text[:i]
	}

	if i := strings.Index(text, "("); i >= 0 {
		text = text[:i]
	}

	return strings.TrimSpace(text)
}

// benchPopupText is the start popup: the general guide for a bench, its name, its keys and what End Turn does.
func (s *Screen) benchPopupText(name string) string {
	if name == "" {
		name = "debug bench"
	}

	keys := "After OK the camera goes to the scenario and its banner says what to check and what you should see. Tab shows the banner again."

	if len(s.bench.stops) > 1 {
		keys = "After OK the camera goes to scenario 1; its banner says what to check and what you should see. Tab = next scenario, Shift+Tab = previous scenario."
	}

	turn := "End Turn starts the next turn."

	for _, frozen := range s.bench.frozen {
		if frozen {
			turn = "The enemy is frozen: it never moves or plays, and End Turn comes straight back to you."
			break
		}
	}

	return "This is a test bench: " + name + ".\n\n" + keys + "\n\n" + turn + "\n\nEvery banner is also kept in Reports - Messages."
}

// stepBenchStop is the Tab key: the next stop, or the previous one with Shift held. Outside a bench (no stops)
// it does nothing.
func (s *Screen) stepBenchStop() {
	if len(s.bench.stops) == 0 {
		return
	}

	step := 1

	if s.shiftHeld() {
		step = -1
	}

	s.gotoBenchStop(s.bench.index+step, "")
}

// showBenchBanner shows a bench banner for benchBannerMS and remembers it for tickBenchBanner.
func (s *Screen) showBenchBanner(text string) {
	s.setBannerFor(text, bannerTintDark, benchBannerMS)
	s.bench.bannerText = text
	s.bench.bannerUntil = platform.Ticks() + benchBannerMS
	s.bench.restored = false
}

// tickBenchBanner brings the bench banner back once, for a full hold, when another banner displaced it (the
// turn start "Enemy detected!" alert lands on the bench's first frame) and has since run out.
func (s *Screen) tickBenchBanner() {
	if s.bench.popup != "" { // the start popup first; after its OK, stop 1 and its banner
		popup := s.bench.popup
		s.bench.popup = ""

		if s.sys != nil {
			s.draw(s.sys.Buf(bgWindow)) // the map behind the popup (it opens before the first frame is drawn)
		}

		s.openMenuScreen(func() { s.hooks.messageFn(popup) }, false)
		s.openBenchStart()

		return
	}

	text := s.bench.bannerText

	if text == "" || s.banner.msg == text {
		return
	}

	now := platform.Ticks()

	if s.bench.restored || now >= s.bench.bannerUntil {
		s.bench.bannerText = ""
		return
	}

	if s.banner.msg != "" && now < s.banner.msgExpire {
		return // the other banner is still up: wait for it
	}

	s.setBannerFor(text, bannerTintDark, benchBannerMS)
	s.bench.bannerUntil = now + benchBannerMS
	s.bench.restored = true
}
