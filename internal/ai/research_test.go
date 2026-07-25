// research_test.go : tests the AI's research topic choice and centre allocation.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package ai

import (
	"testing"

	"github.com/suglasp/max-go/internal/access"
	"github.com/suglasp/max-go/internal/assets"
	"github.com/suglasp/max-go/internal/game"
	"github.com/suglasp/max-go/internal/sim"
	"github.com/suglasp/max-go/internal/units"
)

// TestBestResearchTopicWeights pins the weighting the original applies when it picks a
// topic: Range counts half its turns, Speed double, Shots five times over at level 0,
// and Build Cost is never chosen.
func TestBestResearchTopicWeights(t *testing.T) {
	tests := []struct {
		name  string
		turns [game.ResearchTopicCount]int
		want  int
	}{
		{"an empty board falls back to Attack", [game.ResearchTopicCount]int{}, game.ResearchAttack},
		{
			"the fewest turns wins",
			[game.ResearchTopicCount]int{100, 90, 200, 80, 300, 300, 300, 1},
			game.ResearchArmor,
		},
		{
			"Range wins on half its turns",
			[game.ResearchTopicCount]int{100, 100, 150, 100, 100, 100, 100, 100},
			game.ResearchRange,
		},
		{
			"Speed loses on double its turns",
			[game.ResearchTopicCount]int{100, 100, 400, 90, 100, 60, 100, 100},
			game.ResearchArmor,
		},
		{
			"Shots at level 0 counts five times over",
			[game.ResearchTopicCount]int{100, 30, 400, 200, 200, 200, 200, 1},
			game.ResearchAttack,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var topics [game.ResearchTopicCount]game.ResearchTopic
			for i, turns := range tc.turns {
				topics[i].TurnsToComplete = turns
				topics[i].Allocation = 1
			}

			if got := bestResearchTopic(&topics); got != tc.want {
				t.Fatalf("bestResearchTopic = %d, want %d", got, tc.want)
			}
		})
	}
}

// TestShotsWinsAsItsLevelApproachesTheNextShot checks the Shots scaling: the same turn
// count is discounted as the level climbs towards the next multiple of five, so the AI
// that never picked Shots at level 0 does pick it at level 4.
func TestShotsWinsAsItsLevelApproachesTheNextShot(t *testing.T) {
	var topics [game.ResearchTopicCount]game.ResearchTopic
	for i := range topics {
		topics[i].TurnsToComplete = 100
		topics[i].Allocation = 1
	}

	topics[game.ResearchShots].TurnsToComplete = 60
	topics[game.ResearchRange].TurnsToComplete = 200 // keep the halved topic out of the way

	if got := bestResearchTopic(&topics); got == game.ResearchShots {
		t.Fatal("Shots picked at level 0, where 60 turns count as 300")
	}

	topics[game.ResearchShots].Level = 4 // 5 - 4%5 = 1: the turns now count at face value

	if got := bestResearchTopic(&topics); got != game.ResearchShots {
		t.Fatalf("bestResearchTopic = %d at Shots level 4, want Shots (%d)", got, game.ResearchShots)
	}
}

// TestAIAllocatesItsResearchCenters checks that a computer team allocates running research centres and ignores switched-off centres.
func TestAIAllocatesItsResearchCenters(t *testing.T) {
	const w, h = 12, 12
	mgr := units.NewManager()
	grid := &access.Grid{W: w, H: h, Surf: allLand{}, Units: mgr.Cells}

	p := &game.StartParams{}
	p.Teams[0].Type = game.TeamPlayer
	p.Teams[1].Type = game.TeamComputer
	g := sim.NewWithGrid(game.NewGameState(p), mgr, grid, w, h)

	stationary(1, 1, assets.Research, 4, 4, mgr)
	stationary(1, 2, assets.Research, 6, 4, mgr)
	off := stationary(1, 3, assets.Research, 8, 4, mgr)
	off.Order = game.OrderPowerOff // the player switched this one off: it does not research

	g.AI = New(testUnitFactory)
	g.BeginTeamTurn(1)

	topics := &g.State.Teams[1].Research
	allocated, on := 0, -1

	for i := range topics {
		allocated += topics[i].Allocation
		if topics[i].Allocation > 0 {
			on = i
		}
	}

	if allocated != 2 {
		t.Fatalf("allocation = %d centres, want 2 (the third is switched off)", allocated)
	}

	if on != game.ResearchAttack {
		t.Fatalf("topic = %d, want Attack (%d) on an untouched board", on, game.ResearchAttack)
	}

	if topics[on].TurnsToComplete <= 0 {
		t.Fatal("the chosen topic has no turns to complete: it will never level up")
	}
}

// TestAIResearchLeavesTheHumanAlone checks the allocator only ever touches its own
// team: the player's research pool is theirs to spend.
func TestAIResearchLeavesTheHumanAlone(t *testing.T) {
	const w, h = 12, 12
	mgr := units.NewManager()
	grid := &access.Grid{W: w, H: h, Surf: allLand{}, Units: mgr.Cells}

	p := &game.StartParams{}
	p.Teams[0].Type = game.TeamPlayer
	p.Teams[1].Type = game.TeamComputer
	g := sim.NewWithGrid(game.NewGameState(p), mgr, grid, w, h)

	stationary(0, 1, assets.Research, 4, 4, mgr) // the human's centre, unallocated
	stationary(1, 2, assets.Research, 8, 8, mgr)

	g.AI = New(testUnitFactory)
	g.BeginTeamTurn(1)

	for i := range g.State.Teams[0].Research {
		if g.State.Teams[0].Research[i].Allocation != 0 {
			t.Fatalf("the AI allocated the human's topic %d", i)
		}
	}
}
