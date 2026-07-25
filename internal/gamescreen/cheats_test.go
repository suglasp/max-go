// cheats_test.go : tests cheats.go.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package gamescreen

import (
	"testing"

	"github.com/suglasp/max-go/internal/access"
	"github.com/suglasp/max-go/internal/assets"
	"github.com/suglasp/max-go/internal/game"
	"github.com/suglasp/max-go/internal/platform"
	"github.com/suglasp/max-go/internal/sim"
	"github.com/suglasp/max-go/internal/units"
)

// typeCheatCode arms the buffer and feeds the rest of the code plus Enter, the way
// the run loop would key by key.
func typeCheatCode(s *Screen, code string) {
	s.armCheatInput()
	for _, ch := range code[1:] {
		s.cheatKeyPressed(int(ch))
	}
	s.cheatKeyPressed(platform.KeyReturn)
}

// TestCheatInputBuffer: '[' arms, letters append uppercased, Backspace trims (empty
// disarms), Esc cancels, the length caps at 30, and keys outside the printable range
// are not consumed.
func TestCheatInputBuffer(t *testing.T) {
	s := &Screen{}

	s.armCheatInput()
	if s.cheatInput != "[" {
		t.Fatalf("armed buffer = %q, want \"[\"", s.cheatInput)
	}

	// Lowercase typing upshifts like the original.
	for _, ch := range "max" {
		s.cheatKeyPressed(int(ch))
	}
	if s.cheatInput != "[MAX" {
		t.Errorf("buffer = %q, want \"[MAX\" (uppercased)", s.cheatInput)
	}

	// Backspace trims one character.
	s.cheatKeyPressed(platform.KeyBackspace)
	if s.cheatInput != "[MA" {
		t.Errorf("buffer after Backspace = %q, want \"[MA\"", s.cheatInput)
	}

	// An F key is not text: it must not be consumed and must leave the buffer alone.
	if s.cheatKeyPressed(platform.KeyF1) {
		t.Error("F1 was consumed by the cheat buffer; it should pass through")
	}
	if s.cheatInput != "[MA" {
		t.Errorf("buffer after F1 = %q, want \"[MA\" untouched", s.cheatInput)
	}

	// Esc cancels the whole entry.
	s.cheatKeyPressed(platform.KeyEscape)
	if s.cheatInput != "" {
		t.Errorf("buffer after Esc = %q, want empty", s.cheatInput)
	}

	// The buffer caps at 30 characters like the original.
	s.armCheatInput()
	for i := 0; i < 40; i++ {
		s.cheatKeyPressed('a')
	}
	if len(s.cheatInput) != cheatInputMax {
		t.Errorf("buffer length = %d, want the %d cap", len(s.cheatInput), cheatInputMax)
	}

	// Backspacing all the way back to empty disarms.
	s.cheatInput = "["
	s.cheatKeyPressed(platform.KeyBackspace)
	if s.cheatInput != "" {
		t.Errorf("buffer after trimming the bracket = %q, want empty (disarmed)", s.cheatInput)
	}
}

// TestCheatToggleCodes: [MAXSPY] flips the omniscient reveal and [MAXSURVEY] flips
// the resource reveal, both back and forth; a garbage code is a silent no op.
func TestCheatToggleCodes(t *testing.T) {
	s := &Screen{fogShow: true}

	typeCheatCode(s, "[maxspy]")
	if s.fogShow {
		t.Error("[MAXSPY] did not turn the reveal on (fogShow still true)")
	}
	if s.banner.msg == "" {
		t.Error("a matched cheat must confirm with a banner message")
	}
	typeCheatCode(s, "[MAXSPY]")
	if !s.fogShow {
		t.Error("a second [MAXSPY] did not toggle the reveal back off")
	}

	typeCheatCode(s, "[maxsurvey]")
	if !s.maxSurvey {
		t.Error("[MAXSURVEY] did not turn the resource reveal on")
	}
	typeCheatCode(s, "[MAXSURVEY]")
	if s.maxSurvey {
		t.Error("a second [MAXSURVEY] did not toggle the resource reveal back off")
	}

	// A miss changes nothing, clears the buffer, and shows no confirmation.
	typeCheatCode(s, "[bogus]")
	if s.fogShow != true || s.maxSurvey || s.cheatInput != "" {
		t.Errorf("a bogus code changed state: fogShow=%v maxSurvey=%v buffer=%q", s.fogShow, s.maxSurvey, s.cheatInput)
	}
	if s.banner.msg != "" {
		t.Errorf("a bogus code raised a banner %q, want silence", s.banner.msg)
	}
}

// cheatTestScreen builds a minimal live game with one drained player tank, deployed
// and selected, for the effect dispatch tests.
func cheatTestScreen(t *testing.T) (*Screen, *game.UnitInfo) {
	t.Helper()

	const w, h = 16, 8
	mgr := units.NewManager()
	grid := &access.Grid{W: w, H: h, Surf: buildExitSurf{}, Units: mgr.Cells}
	p := &game.StartParams{}
	p.Teams[0].Type = game.TeamPlayer
	g := sim.NewWithGrid(game.NewGameState(p), mgr, grid, w, h)

	tank := game.NewUnitInfo(assets.Tank, 0, 9001, 0)
	tank.Values = &game.UnitValues{}
	tank.Values.Set(game.AttribHits, 20)
	tank.Values.Set(game.AttribAttack, 10)
	tank.Values.Set(game.AttribStorage, 40)
	tank.Values.Set(game.AttribAmmo, 6)
	tank.InitState()
	tank.Storage, tank.Ammo = 0, 0 // drained, so the fill codes have work to do
	mgr.Deploy(tank, 5, 4)

	return &Screen{game: g, selected: tank}, tank
}

// TestCheatEffectCodes: the typed [MAXSTORAGE] / [MAXAMMO] / [MAXSUPER] codes reach
// the sim cheat helpers (the effects themselves are covered by the sim tests).
func TestCheatEffectCodes(t *testing.T) {
	s, tank := cheatTestScreen(t)

	typeCheatCode(s, "[maxstorage]")
	if tank.Storage != 40 {
		t.Errorf("[MAXSTORAGE] storage = %d, want the 40 cap", tank.Storage)
	}

	typeCheatCode(s, "[maxammo]")
	if tank.Ammo != 6 {
		t.Errorf("[MAXAMMO] ammo = %d, want the 6 cap", tank.Ammo)
	}

	typeCheatCode(s, "[maxsuper]")
	if got := tank.Values.Get(game.AttribAttack); got < 20 {
		t.Errorf("[MAXSUPER] attack = %d, want >= 2x base (20)", got)
	}

	// With nothing selected the super code is a harmless no op.
	s.selected = nil
	typeCheatCode(s, "[maxsuper]")
}
