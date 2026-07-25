// attachservices_test.go : tests the gameservice attachment wiring on GameScreen initialization.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package gamescreen

import (
	"testing"

	"github.com/suglasp/max-go/internal/ai"
	"github.com/suglasp/max-go/internal/assets"
	"github.com/suglasp/max-go/internal/game"
	"github.com/suglasp/max-go/internal/sim"
)

// TestAttachGameServicesRewiresAfterLoad verifies that a restored sim.Game receives new AI and MakeUnit services so computer turns and factory production remain active.
func TestAttachGameServicesRewiresAfterLoad(t *testing.T) {
	makeUnit := func(team int, ty assets.UnitType, fl game.Flags) *game.UnitInfo {
		return game.NewUnitInfo(ty, team, 1, 0)
	}
	s := &Screen{
		ai:       ai.New(makeUnit),
		makeUnit: makeUnit,
		// a fresh game, exactly as snap.Restore yields it: AI + MakeUnit unset.
		game: &sim.Game{},
	}
	if s.game.AI != nil || s.game.MakeUnit != nil {
		t.Fatal("precondition: a freshly restored game must have no AI/MakeUnit")
	}

	s.attachGameServices()

	if s.game.AI == nil {
		t.Error("attachGameServices left AI nil; computer teams would sit idle after a load")
	}
	if s.game.MakeUnit == nil {
		t.Error("attachGameServices left MakeUnit nil; factory production would break after a load")
	}
}

// TestAttachGameServicesAppliesLoadedDifficulty verifies the reload difficulty fix: a
// load restores the opponent tier onto the game state, and attach must apply it to
// the freshly built AI so the Goal / Reports readout is right immediately (the
// reported bug: a game started at Expert, saved, quit and reloaded showed Average).
func TestAttachGameServicesAppliesLoadedDifficulty(t *testing.T) {
	makeUnit := func(team int, ty assets.UnitType, fl game.Flags) *game.UnitInfo {
		return game.NewUnitInfo(ty, team, 1, 0)
	}
	p := &game.StartParams{}
	p.Teams[0].Type = game.TeamPlayer
	p.Teams[1].Type = game.TeamComputer
	st := game.NewGameState(p)
	st.Difficulty = ai.OpponentExpert // as a loaded save carries it

	a := ai.New(makeUnit)
	s := &Screen{ai: a, makeUnit: makeUnit, game: &sim.Game{State: st}}
	t.Cleanup(func() { a.SetDifficulty(ai.OpponentAverage) }) // do not leak the tier into other tests

	s.attachGameServices()

	if got := s.ai.Difficulty(); got != ai.OpponentExpert {
		t.Errorf("after attach (load), AI difficulty = %d, want Expert(%d) from the restored game state", got, ai.OpponentExpert)
	}
}
