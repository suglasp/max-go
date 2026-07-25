// scoregraph.go : stores per-turn samples for the end-game score graph.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package sim

import "github.com/suglasp/max-go/internal/game"

// SnapshotScoreHistory appends the current team_points for every team into
// the 50-entry ring buffer (shift left, newest value at index 49).
// Called once per complete round, just before the turn counter advances.
//
// C++ ported : game_manager.cpp - GameManager_UpdateScoreGraph
func (g *Game) SnapshotScoreHistory() {
	for team := 0; team < game.TeamCount; team++ {
		copy(g.ScoreHistory[team][:], g.ScoreHistory[team][1:])
		g.ScoreHistory[team][49] = int16(g.TeamPoints[team])
	}
}

// AdvanceTurn snapshots the score ring buffer then rolls the turn counter.
// Replace direct g.State.AdvanceTurn() calls with this so history stays in sync.
func (g *Game) AdvanceTurn() {
	g.SnapshotScoreHistory()
	g.State.AdvanceTurn()
}
