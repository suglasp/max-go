// briefing.go : implements typewriter-style mission stories over an ENDGAME backdrop.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
// C++: menu.cpp - menu_draw_mission_story_screen; text.cpp - Text_TypeWriter_TextBoxMultiLineWrapText
package menus

import (
	"fmt"
	"os"
	"strings"

	"github.com/suglasp/max-go/internal/fonts"
	"github.com/suglasp/max-go/internal/game"
	"github.com/suglasp/max-go/internal/gfx"
	"github.com/suglasp/max-go/internal/missionfile"
	"github.com/suglasp/max-go/internal/platform"
	"github.com/suglasp/max-go/internal/resfile"
)

// The briefing text box geometry and the reveal pace, from the original
// (menu.cpp: a 600x400 box at 20,20; 10 ms per character).
const (
	briefingTextX  = 20
	briefingTextY  = 20
	briefingTextW  = 600
	briefingTextH  = 400
	briefingCharMS = 10
)

// campaignIntroFallback is the campaign opening briefing the original shows
// before campaign mission 1 (the Secret Council text). Lang file key: campaign_intro.
const campaignIntroFallback = "We are the Secret Council of Freedom, an organization of M.A.X. Commanders " +
	"who are devoted to making a place for mankind as an equal member of the Concord. As the members of " +
	"the Concord have grown more decadent, they have left more analytical and decision-making positions " +
	"in human hands. At last we are in a position to act. Unfortunately, only four Clans have thrown " +
	"their support to our efforts: Crimson Path, Musashi, Sacred Eight and Von Griffin. The other four " +
	"Clans prefer to continue to submit themselves to the whims of our Concord masters.\n\n" +
	"As long as the Concord has the ability to destroy the Great Ships that contain the majority of our " +
	"population and all of our main manufacturing facilities, they have hostages that we cannot endanger. " +
	"For centuries we have sought some way to safeguard the Great Ships. At last we think we have the way " +
	"to at least give them a fighting chance.\n\n" +
	"The Council has received secret survey reports that there is knowledge of ancient artifacts unknown " +
	"to the Concord on various planets that are up for dispute. Taking over this knowledge will give us " +
	"an edge that will allow us to protect the Great Ships and negotiate with the Concord from a position " +
	"of strength."

// RunCampaignIntro shows the campaign opening briefing. Only the first campaign
// mission carries one; the caller plays the story video (MAXMVE2) after it.
func RunCampaignIntro(ctx *Context) error {
	return RunMissionBriefing(ctx, ctx.text("campaign_intro", campaignIntroFallback))
}

// RunCampaignVictory shows the campaign ending story after the final mission is
// won. The text is the shipped WIN<index>.CAM narrative (only WIN9.CAM exists in
// retail, so only the last mission has one); a missing file means no ending
// screen, matching the original showing a victory story only when the mission
// carries one. The win music (WINR_MSC, already started at game over) is
// restarted so it plays under the story.
// C++: menu.cpp - menu_wrap_up_game victory branch / menu_draw_mission_story_screen
func RunCampaignVictory(ctx *Context, dir string, index int) error {
	path := resfile.DataPath(dir, fmt.Sprintf("WIN%d.CAM", index))

	if _, err := os.Stat(path); err != nil {
		return nil // no WIN<N>.CAM for this mission: no ending, like a mission with no victory info
	}

	text, err := missionfile.ReadDescription(path)

	if err != nil || strings.TrimSpace(text) == "" {
		return nil
	}

	// Stop the current music so the win jingle restarts under the story.
	if ctx.Audio != nil {
		ctx.Audio.StopMusic()
		ctx.Audio.PlayMusic("WINR_MSC", false)
	}

	return RunMissionBriefing(ctx, text)
}

// RunMissionBriefing shows a mission story screen: a random ENDGAME1..9
// backdrop, the briefing text revealed typewriter style (bright silver,
// vertically centered in the box), then an OK button; Return/Escape closes.
// Empty text shows nothing, like the original skipping the window - the
// caller still plays its story video.
// C++: menu.cpp - menu_draw_mission_story_screen
func RunMissionBriefing(ctx *Context, text string) error {
	if strings.TrimSpace(text) == "" {
		return nil
	}

	// A random backdrop, the original's pick (Randomizer_Generate(9)).
	bgTag := fmt.Sprintf("ENDGAME%d", 1+game.RandomizerGenerate(9))
	bg, err := ctx.loadBig(bgTag)

	if err != nil {
		return err
	}

	ctx.Plat.SetPalette(&bg.Palette)
	ctx.drawBigAt(bg, 0, 0)

	f := ctx.Sys.Fonts().Get(fonts.Font1)
	buf := ctx.Sys.Buf(mainWindow)
	lines := briefingLines(f, text)
	lineHeight := f.Height + 1

	if maxRows := briefingTextH / lineHeight; len(lines) > maxRows {
		lines = lines[:maxRows]
	}

	total := 0

	for _, line := range lines {
		total += len(line)
	}

	// The block of rows is vertically centered in the box, like the original.
	startY := briefingTextY + (briefingTextH+1-len(lines)*lineHeight)/2

	ctx.Sys.MouseShow()
	defer ctx.Sys.MouseHide()
	ctx.Sys.FlushInput()

	// The reveal: one more character every briefingCharMS. A normal typed key, ESC,
	// or a left click finishes it at once. Only the F keys are left alone (the
	// The original speeds up on any input, while this input path leaves the F keys alone.)
	// The skipping key is flushed below, so the ESC that finished the text does not
	// also dismiss the OK plate: leaving takes a second press.
	prevButtons := ctx.Plat.Mouse().Buttons
	start := platform.Ticks()
	shown := 0

	for shown < total {
		key := ctx.Sys.GetInput()

		cur := ctx.Plat.Mouse().Buttons
		clicked := cur&platform.MousePressLeft != 0 && prevButtons&platform.MousePressLeft == 0
		prevButtons = cur

		n := int(platform.Ticks()-start) / briefingCharMS

		if n > total || briefingSkipKey(key) || clicked {
			n = total
		}

		if n != shown {
			shown = n
			ctx.Sys.Lock()
			drawBriefingText(buf, f, lines, startY, shown)
			ctx.Sys.Unlock()
		}
	}

	// The OK plate; Return or Escape (also the button's rvalue) closes. It clicks
	// like the other menu buttons.
	ctx.Sys.Fonts().SetFont(fonts.Font5)
	bid, _ := ctx.plateButton(mainWindow, 293, 458, "ENDOK_U", "ENDOK_D",
		ctx.text("5653", "OK"), platform.KeyEscape)
	ctx.registerButtonSfx(bid, "MBUTT0") // the small menu button blip, as on the in game Files/Prefs
	ctx.Sys.FlushInput()

	for {
		key := ctx.Sys.GetInput()

		if key == platform.KeyReturn || key == platform.KeyEscape {
			break
		}
	}

	ctx.Sys.DeleteButton(bid)
	return nil
}

// briefingSkipKey reports whether a key finishes the typewriter reveal at once.
// A normal typed key (any printable ASCII), Return, or ESC does. ESC only finishes
// the text: it takes a second press, once the OK plate is up, to leave the screen.
// The F keys are the one departure from the original, which speeds up on any input
// at all; they are left alone for their in game functions.
//
// C++ ported : text.cpp - Text_TypeWriter_TextBox zeroes the character delay on any
// input during the reveal, ESC included.
func briefingSkipKey(key int) bool {
	return key == platform.KeyReturn || key == platform.KeyEscape || (key >= ' ' && key <= '~')
}

// briefingLines wraps the story text for the briefing box: paragraphs split on
// newlines, each wrapped to the box width, one blank row between paragraphs.
func briefingLines(f *fonts.Font, text string) []string {
	var lines []string

	for _, para := range strings.Split(text, "\n") {
		para = strings.TrimSpace(para)

		if para == "" {
			if len(lines) > 0 && lines[len(lines)-1] != "" {
				lines = append(lines, "")
			}

			continue
		}

		lines = append(lines, wrapLines(f, para, briefingTextW, 0)...)
	}

	for len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}

	return lines
}

// drawBriefingText draws the first shown characters of the wrapped briefing
// rows, the reveal state of the typewriter. Called with the screen lock held.
func drawBriefingText(buf *gfx.Buffer, f *fonts.Font, lines []string, startY, shown int) {
	lineHeight := f.Height + 1
	y := startY
	left := shown

	for _, line := range lines {
		if left <= 0 {
			break
		}

		draw := line

		if len(draw) > left {
			draw = draw[:left]
		}

		left -= len(line)

		if draw != "" && y >= 0 && y+lineHeight+1 <= buf.H {
			f.BlitShadowed(buf.Sub(briefingTextX, y, buf.W-briefingTextX, lineHeight+1),
				draw, briefingTextW+1, fonts.ColorBrightSilver)
		}

		y += lineHeight
	}
}
