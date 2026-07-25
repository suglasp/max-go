// ai_upgrades.go : what a computer team buys with its landing gold.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
// C++ ported : aiplayer.cpp - AiPlayer::ChooseInitialUpgrades / ChooseUpgrade / UpgradeUnitType
package gamescreen

import (
	"github.com/suglasp/max-go/internal/ai"
	"github.com/suglasp/max-go/internal/assets"
	"github.com/suglasp/max-go/internal/dbg"
	"github.com/suglasp/max-go/internal/economy"
	"github.com/suglasp/max-go/internal/game"
)

// aiUpgradeBuyLimit caps one landing spend, so a pricing bug can never spin the buy
// loop forever. Real spends stop long before this: every step raises the attribute,
// and the next step of the same attribute costs more than the last.
const aiUpgradeBuyLimit = 200

// startFounders are the units every team is handed for free at landing: the two
// builders that found the base and the surveyor that finds it a deposit. The purchase
// screen gives the player these before it hands over the credits, so a computer team
// is not charged for its own copies either.
//
// C++ ported : missionmanager - the default mission loadout, rolled before the team
// spends its gold (aiplayer.cpp RollTeamMissionSupplies)
var startFounders = []assets.UnitType{assets.Constrct, assets.Engineer, assets.Surveyor}

// applyAIStartUpgrades spends a computer team's landing credit on strategy upgrades before deployment, so its landing force and later production
// use the improved values.
//
// The team is charged for the landing force it is handed (at the purchase screen's
// price, the founders excepted) and upgrades with what is left over, which is how the
// original splits the same pot: it buys its extra units out of team_gold and hands the
// remainder to ChooseInitialUpgrades. A game that never set a start credit (the quick
// start benches without -startcredit) buys nothing.
//
// C++ ported : aiplayer.cpp - AiPlayer::ChooseInitialUpgrades
func (s *Screen) applyAIStartUpgrades(team int, roster []unitSpec) {
	if s.ai == nil || !s.startCreditSet {
		return
	}

	orders := s.ai.InitialUpgradeOrders(s.game, team) // empty for a human team
	if len(orders) == 0 {
		return
	}

	budget := s.aiLandingBudget(roster)
	if budget <= 0 {
		return // the landing force ate the whole credit
	}

	s.game.Gold[team] = budget
	spent := s.buyUpgrades(team, orders)

	dbg.Printf("ai: team %d lands with %d credits, spends %d on upgrades", team, budget, spent)
}

// buyAITurnUpgrades spends a computer team's banked refinery gold at the start of each turn.
//
// Called at the team's turn start, BEFORE its economy and its planning run, so anything it
// builds this turn is already minted at the better values. The team's units in the field are
// re derived too, which is the same courtesy the player's own Upgrade screen does them.
//
// C++ ported : aiplayer.cpp - the ChooseUpgrade / UpgradeUnitType loop in AiPlayer::BeginTurn (:2635-2650)
func (s *Screen) buyAITurnUpgrades(team int) {
	if s.ai == nil || s.game.Gold[team] <= 0 {
		return
	}

	orders := s.ai.TurnUpgradeOrders(s.game, team) // empty for a human team
	if len(orders) == 0 {
		return
	}

	spent := s.buyUpgrades(team, orders)
	if spent == 0 {
		return // nothing on the list is affordable yet: keep saving
	}

	for _, u := range s.game.Units.Units() {
		if u.Team == team {
			s.upgradeUnitValues(team, u)
		}
	}

	dbg.Printf("ai: team %d spends %d banked gold on upgrades (%d left)", team, spent, s.game.Gold[team])
}

// aiLandingBudget is the gold a team has left after paying for the landing force it is
// handed. The three founders are free (see startFounders); everything else costs what
// the purchase screen charges the player for it.
func (s *Screen) aiLandingBudget(roster []unitSpec) int {
	budget := s.startCredit

	free := map[assets.UnitType]int{}
	for _, t := range startFounders {
		free[t]++
	}

	for _, r := range roster {
		if free[r.t] > 0 {
			free[r.t]--

			continue
		}

		budget -= s.unitCredit(r.t)
	}

	if budget < 0 {
		return 0
	}

	return budget
}

// unitCredit is what one unit costs on the purchase screen: the land factory's raw
// draw at build speed 1 times the unit's build turns.
//
// C++ ported : cargomenu.cpp - Cargo_GetRawConsumptionRate(LANDPLT, 1) * ATTRIB_TURNS
func (s *Screen) unitCredit(t assets.UnitType) int {
	return economy.RawConsumptionRate(assets.LandPlt, 1) * s.baseValues(t).Get(game.AttribTurns)
}

// buyUpgrades spends the team's gold reserve on its wish list, always taking the
// cheapest step it can still afford, until the cheapest one no longer fits. It returns
// the gold spent, which is also the score screen's "gold spent on upgrades" tally.
//
// C++ ported : aiplayer.cpp - the ChooseUpgrade / UpgradeUnitType buy loop
func (s *Screen) buyUpgrades(team int, orders []ai.UpgradeOrder) int {
	spent := 0

	for step := 0; step < aiUpgradeBuyLimit; step++ {
		gold := s.game.Gold[team]
		pick, price := -1, 0

		for i, o := range orders {
			cost := s.upgradePrice(team, o.Type, o.Attrib)
			if cost <= 0 || cost > gold {
				continue // the type has no such attribute, or the team cannot afford the step
			}

			if pick == -1 || cost < price {
				pick, price = i, cost
			}
		}

		if pick == -1 {
			break // nothing on the list is affordable any more
		}

		o := orders[pick]
		base := s.baseValues(o.Type).Get(o.Attrib)
		offset := s.game.State.Teams[team].UpgradeOffset(o.Type, o.Attrib)

		s.game.State.Teams[team].SetUpgradeOffset(o.Type, o.Attrib, offset+game.UpgradeStep(o.Attrib, base))
		s.game.Gold[team] -= price
		spent += price
	}

	if spent > 0 {
		s.game.RecordUpgradeGold(team, spent)
	}

	return spent
}

// upgradePrice is what one step of attribute a costs the team for unit type t: the
// same price the Upgrade menu shows the player, research discount included. Zero when
// the type has no such attribute to raise.
//
// C++ ported : teamunits.cpp - TeamUnits_GetUpgradeCost
func (s *Screen) upgradePrice(team int, t assets.UnitType, a game.Attrib) int {
	if a < 0 || a >= game.ResearchTopicCount {
		return 0
	}

	base := s.baseValues(t).Get(a)
	if base <= 0 {
		return 0
	}

	current := s.teamValues(team, t).Get(a)
	discount := game.UpgradeResearchFactor(s.game.State.Teams[team].Research[a].Level, base)

	return game.UpgradeCost(a, current, discount, base)
}
