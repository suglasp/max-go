// cheats.go : handles typed [MAX...] cheat codes.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
// C++ ported : game_manager.cpp - GameManager_ProcessTextInput / GameManager_ProcessCheatCodes
package gamescreen

import "github.com/suglasp/max-go/internal/platform"

// cheatInputMax caps the typed buffer, matching the original's 30 character limit.
const cheatInputMax = 30

// armCheatInput seeds the typed cheat buffer with the opening bracket and echoes it
// in the message banner. C++ ported : game_manager.cpp - GNW_KB_KEY_LEFTBRACKET case.
func (s *Screen) armCheatInput() {
	s.cheatInput = "["
	s.showBanner(s.cheatInput)
}

// cheatKeyPressed consumes one key while the cheat buffer is armed: printable keys
// append uppercased, Backspace trims, Enter matches + clears, Esc cancels. Keys the
// original never routed into its text handler (F keys, arrows, Alt combos) return
// false so normal hotkey handling proceeds.
// C++ ported : game_manager.cpp - GameManager_ProcessTextInput
func (s *Screen) cheatKeyPressed(key int) bool {
	switch {
	case key == platform.KeyReturn:
		// Drop the echo banner first (the original clears its message box before
		// matching), so a matched code's confirmation message survives.
		s.banner.msg = ""
		s.runCheatCode()
		s.cheatInput = ""

	case key == platform.KeyEscape:
		s.cheatInput = ""
		s.banner.msg = ""

	case key == platform.KeyBackspace:
		s.cheatInput = s.cheatInput[:len(s.cheatInput)-1]
		if s.cheatInput == "" {
			s.banner.msg = "" // trimmed back to nothing: disarm and drop the echo
		} else {
			s.showBanner(s.cheatInput)
		}

	case key >= ' ' && key <= '~':
		if len(s.cheatInput) < cheatInputMax {
			// The original upshifts every typed character, so lowercase typing matches
			// the uppercase code table.
			if key >= 'a' && key <= 'z' {
				key -= 'a' - 'A'
			}

			s.cheatInput += string(rune(key))
		}

		s.showBanner(s.cheatInput)

	default:
		return false
	}

	return true
}

// runCheatCode matches the finished buffer against the five vanilla codes, applies the
// effect and confirms it with a banner (a port choice; the original's single player path is
// silent). The reveal codes flip the same flags the F2 debug menu uses; the fill and
// super charge codes reuse the sim cheat helpers. A miss is a silent no op, and the
// original's multiplayer cheater punishment is not ported (the port is single player).
// C++ ported : game_manager.cpp - GameManager_ProcessCheatCodes / GameManager_CheatCodes
func (s *Screen) runCheatCode() {
	switch s.cheatInput {
	case "[MAXSPY]":
		// Toggle the omniscient reveal: every enemy unit visible (fog of war off).
		s.fogShow = !s.fogShow
		if s.fogShow {
			s.showMessage("Cheat disabled: fog of war restored.")
		} else {
			s.showMessage("Cheat enabled: all enemy units revealed.")
		}

	case "[MAXSURVEY]":
		// Toggle the full map resource survey overlay.
		s.maxSurvey = !s.maxSurvey
		if s.maxSurvey {
			s.showMessage("Cheat enabled: all resource deposits revealed.")
		} else {
			s.showMessage("Cheat disabled: resource deposits hidden again.")
		}

	case "[MAXSTORAGE]":
		s.game.CheatFillStorage(s.game.State.PlayerTeam)
		s.showMessage("Cheat: all units' cargo filled.")

	case "[MAXAMMO]":
		s.game.CheatFillAmmo(s.game.State.PlayerTeam)
		s.showMessage("Cheat: all units' ammo refilled.")

	case "[MAXSUPER]":
		if s.selected == nil {
			s.showMessage("Select a unit to super charge.")
			return
		}

		s.game.CheatSuperUnit(s.selected)
		s.showMessage("Cheat: unit super charged.")
	}
}
