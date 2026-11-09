// resumeturn_test.go : tests that a load reopens the turn slot the save was taken in.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package gamescreen

import (
	"testing"

	"github.com/suglasp/max-go/internal/game"
)

// TestLoadResumesAComputerTurn verifies that a save made during a computer turn resumes that computer turn after loading.
func TestLoadResumesAComputerTurn(t *testing.T) {
	s, g := newTestScreen(t, 16)
	g.State.Teams[0].Type = game.TeamPlayer
	g.State.Teams[1].Type = game.TeamComputer

	g.State.ActiveTeam = 1
	s.turnFlow.aiTurn = false
	s.resumeActiveTurn()
	if !s.turnFlow.aiTurn {
		t.Error("a save taken during the opponent's turn must reopen on that computer's turn, not hand control to the human")
	}

	g.State.ActiveTeam = 0
	s.resumeActiveTurn()
	if s.turnFlow.aiTurn {
		t.Error("a save taken on the human's own turn must hand control back to the human")
	}
}
