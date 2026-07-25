// portrait.go : implements the unit portrait panel.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
// C++ ported : unitinfo.cpp - portrait FLC playback in the GNW panel
package gamescreen

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"

	"github.com/suglasp/max-go/internal/assets"
	"github.com/suglasp/max-go/internal/flic"
	"github.com/suglasp/max-go/internal/game"
	"github.com/suglasp/max-go/internal/gfx"
	"github.com/suglasp/max-go/internal/platform"
)

// portraitState holds the selected unit's portrait file, player, and animation timing.
type portraitState struct {
	portraitDir   string         // directory the loose portrait FLC files live in
	portrait      *flic.Player   // looping corner portrait player
	portraitFile  *os.File       // kept open for the looping player
	portraitUnit  *game.UnitInfo // tracks the loaded selection, so it's only rebuilt on change
	portraitStamp uint64         // portrait animation timestamp
}

// portraitFrameMS is the corner portrait frame interval, about eight frames per second.
const portraitFrameMS = 125

// portraitTagExceptions maps unit tags whose portrait resource name differs from the first six tag characters.
//
// C++: the per unit flics ResourceID (units.json "flics")
var portraitTagExceptions = map[string]string{
	"SP_FLAK": "F_FLAK",
	"COMMTWR": "F_COMMT",
	"CNCT_4W": "F_CNCT4W",
}

// portraitResTag returns the F_<unit> resource tag holding a unit portrait's FLC file name.
func portraitResTag(t assets.UnitType) string {
	tag := t.Tag()

	if r, ok := portraitTagExceptions[tag]; ok {
		return r
	}

	if len(tag) > 6 {
		tag = tag[:6]
	}

	return "F_" + tag
}

// SetPortraitDir sets the directory containing loose portrait FLC files. Without it, the corner panel keeps its static FRAMEPIC backing.
func (s *Screen) SetPortraitDir(dir string) {
	s.portraitUI.portraitDir = dir
}

// updatePortrait rebuilds the corner FLC player when selection changes. Failed resource or file loading leaves the static panel visible.
func (s *Screen) updatePortrait() {
	if s.selected == s.portraitUI.portraitUnit {
		return
	}

	s.portraitUI.portraitUnit = s.selected
	s.clearPortrait()

	if s.selected == nil || s.portraitUI.portraitDir == "" || s.lib == nil {
		return
	}

	name, err := s.lib.Read(portraitResTag(s.selected.Type))

	if err != nil {
		return
	}

	file := openPortraitFile(s.portraitUI.portraitDir, portraitFileName(name))

	if file == nil {
		return
	}

	p, err := flic.NewPlayer(file, CornerFlic, true)

	if err != nil {
		file.Close()
		return
	}

	s.portraitUI.portraitFile = file
	s.portraitUI.portrait = p
	s.portraitUI.portraitStamp = 0 // force a frame on the next draw
}

// clearPortrait releases the current portrait player and its open file.
func (s *Screen) clearPortrait() {
	s.portraitUI.portrait = nil

	if s.portraitUI.portraitFile != nil {
		s.portraitUI.portraitFile.Close()
		s.portraitUI.portraitFile = nil
	}
}

// drawCornerFlic advances and draws the portrait into the corner panel, or
// restores the static FRAMEPIC backing when no portrait is loaded.
func (s *Screen) drawCornerFlic(buf *gfx.Buffer) {
	s.updatePortrait()

	if s.portraitUI.portrait == nil {
		s.restoreFrame(buf, CornerFlic)
		return
	}

	// Draw a newly loaded portrait once even when paused. Later frames advance only while playFlic is set. C++: GameManager_PlayFlic gates GameManager_AdvanceFlic.
	now := platform.Ticks()
	first := s.portraitUI.portraitStamp == 0

	if first || (s.hudArt.playFlic && now-s.portraitUI.portraitStamp >= portraitFrameMS) {
		s.portraitUI.portraitStamp = now

		if _, err := s.portraitUI.portrait.DrawNext(buf); err != nil {
			s.clearPortrait()
			s.restoreFrame(buf, CornerFlic)
		}
	}
}

// portraitFileName trims the NUL terminated file name stored in an "F_" tag.
func portraitFileName(raw []byte) string {
	if i := bytes.IndexByte(raw, 0); i >= 0 {
		raw = raw[:i]
	}

	return strings.TrimSpace(string(raw))
}

// openPortraitFile opens a loose .FLC by name, tolerating an upper or lower
// case mismatch between the stored name and the on disk file on case
// sensitive filesystems.
func openPortraitFile(dir, name string) *os.File {
	if name == "" {
		return nil
	}

	for _, n := range []string{name, strings.ToUpper(name), strings.ToLower(name)} {
		if f, err := os.Open(filepath.Join(dir, n)); err == nil {
			return f
		}
	}

	return nil
}
