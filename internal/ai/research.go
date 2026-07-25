// research.go : the AI's research topic choice.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
// C++ ported : aiplayer.cpp - AiPlayer::DetermineResearchProjects
package ai

import (
	"github.com/suglasp/max-go/internal/assets"
	"github.com/suglasp/max-go/internal/dbg"
	"github.com/suglasp/max-go/internal/game"
	"github.com/suglasp/max-go/internal/sim"
)

// determineResearchProjects assigns all running research centres to the topic that finishes soonest. It runs at turn start before task planning;
// syncResearchOrders then powers the required centres and advanceResearch applies progress. Research allocation is pooled per team, so every centre
// is assigned to the selected topic.
//
// C++ ported : aiplayer.cpp - AiPlayer::DetermineResearchProjects
func (p *AiPlayer) determineResearchProjects() {
	if p.ctx == nil || p.ctx.Game == nil {
		return
	}

	g := p.ctx.Game
	if p.team < 0 || p.team >= game.TeamCount {
		return
	}

	centers := runningResearchCenters(g, p.team)
	topics := &g.State.Teams[p.team].Research
	best := bestResearchTopic(topics)

	for i := range topics {
		n := 0
		if i == best {
			n = centers
		}

		if topics[i].Allocation == n {
			continue
		}

		// SetAllocation keeps the turns to complete consistent: a topic dropped part way
		// through keeps its remaining turns and simply stops counting down, exactly like
		// the original's ResearchMenu_UpdateResearchProgress(team, topic, -1) per centre.
		topics[i].SetAllocation(game.ResearchFactors[i], n)
	}

	if centers > 0 {
		dbg.Logf(dbg.AI, "team %d researches topic %d with %d center(s)", p.team, best, centers)
	}
}

// runningResearchCenters counts the team's finished research centres that are not
// switched off. An idle one still counts: the allocation is what powers it back on.
func runningResearchCenters(g *sim.Game, team int) int {
	n := 0

	for _, u := range g.Units.Units() {
		if u.Team != team || u.Type != assets.Research || u.Building {
			continue
		}

		if u.Order != game.OrderPowerOff {
			n++
		}
	}

	return n
}

// bestResearchTopic is the topic the team should research next: the one with the
// fewest turns left, weighted the way the original weights them. Range counts half
// its turns (the AI prizes reach), Speed twice (it is the least valued), and Shots
// is scaled down as the level approaches the next multiple of five, so the AI
// eventually goes for the extra shot rather than never affording it. Build Cost is
// never researched by the computer, and Attack is the fallback.
//
// A topic nobody has started reads zero turns and therefore wins, which is what makes
// the computer seed each topic with one turn of work before it settles on one - the
// original behaves the same way.
//
// C++ ported : aiplayer.cpp - AiPlayer::DetermineResearchProjects (the topic scan)
func bestResearchTopic(topics *[game.ResearchTopicCount]game.ResearchTopic) int {
	best := game.ResearchAttack
	bestTurns := topics[best].TurnsToComplete

	for topic := game.ResearchShots; topic < game.ResearchCost; topic++ {
		turns := topics[topic].TurnsToComplete

		switch topic {
		case game.ResearchRange:
			turns /= 2
		case game.ResearchShots:
			turns *= 5 - topics[topic].Level%5
		case game.ResearchSpeed:
			turns *= 2
		}

		if turns < bestTurns {
			best, bestTurns = topic, turns
		}
	}

	return best
}
