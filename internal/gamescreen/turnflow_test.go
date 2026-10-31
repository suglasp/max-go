// turnflow_test.go : testing unit for turnflow.go
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package gamescreen

import (
	"testing"

	"github.com/suglasp/max-go/internal/game"
	"github.com/suglasp/max-go/internal/winloss"
)

// TestGameEndWarningDue checks the Duration "game will end in 10 turns" notice opens turn limit-9, so
// turns limit-9 to limit are the last ten (klei 23/08/26), and only for a human player in a Duration game.
func TestGameEndWarningDue(t *testing.T) {
	s := timeWarnScreen(0)
	s.game.VictoryType = winloss.VictoryDuration
	s.game.VictoryLimit = 100

	for turn, want := range map[int]bool{89: false, 90: false, 91: true, 92: false, 100: false} {
		s.game.State.TurnCounter = turn
		if got := s.gameEndWarningDue(); got != want {
			t.Errorf("turn %d of 100: warning due = %v, want %v", turn, got, want)
		}
	}

	s.game.State.TurnCounter = 91
	s.game.VictoryType = winloss.VictoryScore
	if s.gameEndWarningDue() {
		t.Error("a Score game warned about the turn limit")
	}

	s.game.VictoryType = winloss.VictoryDuration
	s.game.State.Teams[0].Type = game.TeamComputer
	if s.gameEndWarningDue() {
		t.Error("warned with no human player (attract demo / AI bench)")
	}
}
