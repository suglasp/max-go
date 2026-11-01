// banner.go : implements the message banner and the message log.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package gamescreen

import (
	"fmt"
	"strings"

	"github.com/suglasp/max-go/internal/dbg"
	"github.com/suglasp/max-go/internal/fonts"
	"github.com/suglasp/max-go/internal/game"
	"github.com/suglasp/max-go/internal/gfx"
	"github.com/suglasp/max-go/internal/platform"
	"github.com/suglasp/max-go/internal/winloss"
)

// bannerState holds the transient message banner and the messages retained for Reports.
type bannerState struct {
	msg       string     // transient event message shown in the message banner
	msgExpire uint64     // tick the message banner clears
	msgTint   bannerTint // which of the 3 translucent banner LUTs the message uses
	msgLog    []msgEntry // Reports -> Messages tab history (newest last), capped to msgLogMax
}

// bannerText returns the active banner text, whether it is visible, and its tint. A transient message keeps its own tint; other states use the dark tint.
func (s *Screen) bannerText() (string, bool, bannerTint) {

	switch {
	case s.gameOver && s.outcome == winloss.PlayerWon:
		return fmt.Sprintf("VICTORY - %s wins!   (RETURN / ESC)", s.teamName(s.game.State.PlayerTeam)), true, bannerTintDark
	case s.gameOver:
		return "DEFEAT   (RETURN / ESC)", true, bannerTintDark
	case s.siteSelect != nil:
		if s.banner.msg != "" && platform.Ticks() < s.banner.msgExpire { // a transient warning overrides the prompt
			return s.banner.msg, true, s.banner.msgTint
		}

		return s.lang.Get("851c", "Select starting location."), true, bannerTintDark
	case s.turnFlow.handover != -1:
		if s.game.State.Teams[s.turnFlow.handover].Type == game.TeamComputer {
			return fmt.Sprintf(s.lang.Get("2ab8", "%s Computer Turn."), s.teamName(s.turnFlow.handover)), true, bannerTintDark
		}

		return fmt.Sprintf("%s Player Turn.", s.teamName(s.turnFlow.handover)), true, bannerTintDark
	case s.turnFlow.aiTurn:
		// The computer turn still needs a banner when no hot-seat handover is active. A transient message takes priority.
		if s.banner.msg != "" && platform.Ticks() < s.banner.msgExpire {
			return s.banner.msg, true, s.banner.msgTint
		}

		return fmt.Sprintf(s.lang.Get("2ab8", "%s Computer Turn."), s.teamName(s.turnFlow.aiTurnTeam)), true, bannerTintDark
	case s.banner.msg != "" && platform.Ticks() < s.banner.msgExpire:
		return s.banner.msg, true, s.banner.msgTint
	}

	return "", false, bannerTintDark
}

// showMessage displays and records a transient event banner, such as a combat result. C++: MessageManager_DrawMessage (type 1, the normal dark band).
func (s *Screen) showMessage(text string) {
	s.showBanner(text)
	s.logMessage(text)
}

// bannerTint selects one of M.A.X.'s three message-box lookup tables. The 8-bit banner recolours map pixels through the selected 256-byte intensity table.
//
// C++ ported : message_manager.cpp - MessageManager_MessageBox_BgColorArray[type]
type bannerTint int

const (
	bannerTintDark  bannerTint = iota // type 0: a neutral dark scrim (routine messages)
	bannerTintWorld                   // type 1: a world shaded tint (in world events, status, spotted enemy)
	bannerTintRed                     // type 2: a red alert (blocking errors: no material, bad placement)
)

// showAlert displays and records a red banner for a blocking error, such as insufficient material or an invalid placement. C++: MessageManager_DrawMessage with BgColorArray[2].
func (s *Screen) showAlert(text string) {
	s.setBanner(text, bannerTintRed)
	s.logMessage(text)
}

// showInfo displays and records a world-tinted banner for map events and status. C++: MessageManager_DrawMessage with BgColorArray[1].
func (s *Screen) showInfo(text string) {
	s.setBanner(text, bannerTintWorld)
	s.logMessage(text)
}

// showBanner displays a dark transient event banner without adding a log entry.
func (s *Screen) showBanner(text string) {
	s.setBanner(text, bannerTintDark)
}

// showAlertBanner displays a red transient banner without adding a log entry.
func (s *Screen) showAlertBanner(text string) {
	s.setBanner(text, bannerTintRed)
}

// showInfoLines displays and records a world-tinted multi-line banner, with one line per string. C++: ProductionManager accumulates one ComposeIndustryMessage per shut building.
func (s *Screen) showInfoLines(lines []string) {
	if len(lines) == 0 {
		return
	}

	text := strings.Join(lines, "\n")
	s.setBanner(text, bannerTintWorld)
	s.logMessage(text)
}

// setBanner stores a transient banner's text, expiry tick, and tint.
func (s *Screen) setBanner(text string, tint bannerTint) {
	s.setBannerFor(text, tint, 2500)
}

// setBannerFor is setBanner with its own hold time in milliseconds.
func (s *Screen) setBannerFor(text string, tint bannerTint, ms uint64) {
	s.banner.msg = text
	s.banner.msgExpire = platform.Ticks() + ms
	s.banner.msgTint = tint
}

// msgLogMax caps the retained message history.
const msgLogMax = 200

// msgEntry is one Reports -> Messages row with an optional unit icon, map coordinate, and message text. C++: MessageLogEntry (unit + position + text).
type msgEntry struct {
	icon   *gfx.Buffer // unit icon shown in the report
	coords string      // map coordinates tied to the message
	text   string      // report message text
}

// logMessage appends a plain text message (no icon/coords) to the history.
func (s *Screen) logMessage(text string) {
	s.logEvent(nil, "", text)
}

// logEvent appends an optional unit icon and grid coordinate to the message history, trims it to msgLogMax, and writes it to the debug log.
func (s *Screen) logEvent(icon *gfx.Buffer, coords, text string) {
	s.banner.msgLog = append(s.banner.msgLog, msgEntry{icon: icon, coords: coords, text: text})

	if len(s.banner.msgLog) > msgLogMax {
		s.banner.msgLog = s.banner.msgLog[len(s.banner.msgLog)-msgLogMax:]
	}

	if coords != "" {
		dbg.Printf("msg [%s]: %s", coords, text)
	} else {
		dbg.Printf("msg: %s", text)
	}
}

// Message banner geometry uses logical 640x480 pixels. The strip starts 10 pixels below the map and is padded by bannerPad.
const (
	bannerTop = 10
	bannerPad = 8
)

// redTintFactor returns 63 for snow worlds and 31 elsewhere. Larger factors make the red alert tint more transparent.
// C++: World::GetRedTintParameters (factor 63 for SNOW_1..6).
func (s *Screen) redTintFactor() int {
	if strings.Contains(strings.ToUpper(s.mapName), "SNOW") {
		return 63
	}

	return 31
}

// worldTintParams returns type-1 WorldTint LUT levels. Snow worlds use red-only shading; other worlds use equal red, green, and blue values.
// C++ ported : world.cpp - World::GetWorldTintParameters.
func (s *Screen) worldTintParams() (r, g, b int) {
	if strings.Contains(strings.ToUpper(s.mapName), "SNOW") {
		return 63, 0, 0
	}

	return 63, 63, 63
}

// messageColor draws white message text with a one-pixel down-right shadow at palette index 199. The outline stays white for thick and diagonal strokes.
var messageColor = fonts.Color{Base: 0xFF, Outline: 0xFF, Shadow: 199}

// drawBanner draws the active message in a tinted strip near the map's top-left corner. It wraps text and stays within the map rectangle.
// C++: MessageManager_DrawMessage / WrapText.
func (s *Screen) drawBanner(buf *gfx.Buffer) {
	text, ok, tint := s.bannerText()

	if !ok {
		return
	}

	f := s.sys.Fonts().Get(fonts.Font5)

	maxTextW := MainMap.Width() - 2*bannerPad // never reaches the side UI frame. C++: WrapText(text, mapWidth-20)

	if maxTextW < 1 {
		maxTextW = 1
	}

	lines := wrapBannerText(f, text, maxTextW)

	if len(lines) == 0 {
		return
	}

	// Size the band to the widest wrapped line and one font row per line.
	textW := 0

	for _, ln := range lines {
		if w := f.Width(ln); w > textW {
			textW = w
		}
	}

	startX := MainMap.ULX
	startY := MainMap.ULY + bannerTop
	bandW := min(textW+2*bannerPad, MainMap.Width()) // never past the map's right edge
	bandH := len(lines)*f.Height + 2*bannerPad

	if maxH := MainMap.Height() - bannerTop; bandH > maxH {
		bandH = maxH
	}

	lut := s.pal.DarkeningTable() // one of 3 message box LUTs by tint tier. C++: BgColorArray[type]

	switch tint {
	case bannerTintRed:
		lut = s.pal.RedTintTable(s.redTintFactor())
	case bannerTintWorld:
		r, g, b := s.worldTintParams()
		lut = s.pal.IntensityTable2(r, g, b)
	}

	band := buf.Sub(startX, startY, bandW, bandH)

	for y := 0; y < band.H; y++ {
		row := band.Row(y)

		for x := 0; x < band.W; x++ {
			row[x] = lut[row[x]]
		}
	}

	// Draw one shadowed white line per band row.
	tx := startX + bannerPad

	for i, ln := range lines {
		ly := startY + bannerPad + i*f.Height
		f.BlitShadowed(buf.Sub(tx, ly, bandW-bannerPad, f.Height), ln, f.Width(ln)+1, messageColor)
	}
}

// wrapBannerText wraps a banner message to width pixels and splits a word that cannot fit on one line. C++: Text_SplitText.
func wrapBannerText(f *fonts.Font, text string, width int) []string {
	if f == nil || width < 1 {
		return nil
	}

	var lines []string

	for _, segment := range strings.Split(text, "\n") { // explicit breaks first, then word wrap each
		line := ""

		for _, word := range strings.Fields(segment) {
			// Split a word that is wider than the banner.
			for f.Width(word) > width && len(word) > 1 {
				cut := len(word)

				for cut > 1 && f.Width(word[:cut]) > width {
					cut--
				}

				if line != "" {
					lines = append(lines, line)
					line = ""
				}

				lines = append(lines, word[:cut])
				word = word[cut:]
			}

			switch {
			case line == "":
				line = word
			case f.Width(line+" "+word) <= width:
				line += " " + word
			default:
				lines = append(lines, line)
				line = word
			}
		}

		if line != "" {
			lines = append(lines, line)
		}
	}

	return lines
}
