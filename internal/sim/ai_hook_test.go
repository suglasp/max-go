// ai_hook_test.go : tests the AI turn hook integration in the sim turn loop.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package sim

import (
	"testing"

	"github.com/suglasp/max-go/internal/game"
)

// countingAI records which teams BeginTeamTurn handed to the AI seam.
type countingAI struct{ teams []int }

func (a *countingAI) BeginTurn(g *Game, team int) { a.teams = append(a.teams, team) }

// TestAITurnHookFiresForComputerTeamsOnly checks the Phase A AI seam: the sim
// calls TurnAI.BeginTurn when a TeamComputer team's turn starts and never for a
// human team, and not at all when no AI is wired (the default pass behavior).
func TestAITurnHookFiresForComputerTeamsOnly(t *testing.T) {
	g := newGame(t, 8, 8)
	g.State.Teams[1].Type = game.TeamComputer

	// No AI wired: BeginTeamTurn must not panic and there is nothing to assert
	// beyond it running for both team kinds.
	g.BeginTeamTurn(0)
	g.BeginTeamTurn(1)

	ai := &countingAI{}
	g.AI = ai
	g.BeginTeamTurn(0) // human team: no AI call
	g.BeginTeamTurn(1) // computer team: one AI call
	g.BeginTeamTurn(1) // again: another call

	if len(ai.teams) != 2 {
		t.Fatalf("AI hook fired %d times, want 2 (computer team only)", len(ai.teams))
	}
	for _, team := range ai.teams {
		if team != 1 {
			t.Errorf("AI hook fired for team %d, want only the computer team 1", team)
		}
	}
}
