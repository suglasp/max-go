// upgrades.go : the unit upgrades a computer team wants to buy with its gold.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
// C++ ported : aiplayer.cpp - AiPlayer::ChooseStrategicBuildOrders / ChooseUpgrade
package ai

import (
	"github.com/suglasp/max-go/internal/assets"
	"github.com/suglasp/max-go/internal/game"
	"github.com/suglasp/max-go/internal/sim"
)

// UpgradeOrder is one attribute of one unit type the computer team wants to buy with gold. The game layer prices and applies the wish list.
//
// C++ ported : aiplayer.hpp - struct BuildOrder (unit_type + primary_attribute)
type UpgradeOrder struct {
	Type   assets.UnitType
	Attrib game.Attrib
}

// strategyUpgradeOrders is the wish list per strategy: the units that strategy leans
// on and the attribute each one wants raised. These are the original's landing time
// lists, the short form it rolls the mission supplies against (its longer in game
// lists add the support units it can only buy once factories stand).
//
// C++ ported : aiplayer.cpp - AiPlayer::ChooseStrategicBuildOrders(mode = true)
var strategyUpgradeOrders = map[Strategy][]UpgradeOrder{
	StrategyDefensive: {
		{assets.AntiMssl, game.AttribRange},
		{assets.AntiMssl, game.AttribAttack},
	},
	StrategyMissiles: {
		{assets.MisslLch, game.AttribRange},
	},
	StrategyAir: {
		{assets.Fighter, game.AttribAttack},
		{assets.Fighter, game.AttribRange},
		{assets.Bomber, game.AttribAttack},
	},
	StrategySea: {
		{assets.Battlshp, game.AttribAttack},
		{assets.Battlshp, game.AttribArmor},
		{assets.Battlshp, game.AttribHits},
		{assets.MsslBoat, game.AttribRange},
	},
	StrategyScoutHorde: {
		{assets.Scout, game.AttribAttack},
		{assets.Scout, game.AttribArmor},
		{assets.Scout, game.AttribHits},
		{assets.Scout, game.AttribSpeed},
	},
	StrategyTankHorde: {
		{assets.Scout, game.AttribSpeed},
		{assets.Tank, game.AttribAttack},
		{assets.Tank, game.AttribRounds},
		{assets.Tank, game.AttribHits},
		{assets.Tank, game.AttribArmor},
		{assets.Tank, game.AttribSpeed},
	},
	StrategyFastAttack: {
		{assets.Scout, game.AttribScan},
		{assets.Artillry, game.AttribAttack},
		{assets.Artillry, game.AttribRounds},
		{assets.Artillry, game.AttribRange},
	},
	StrategyCombinedArms: {
		{assets.MisslLch, game.AttribRange},
		{assets.Tank, game.AttribAttack},
		{assets.Tank, game.AttribArmor},
		{assets.Tank, game.AttribHits},
	},
	StrategyEspionage: {
		{assets.Commando, game.AttribScan},
		{assets.Commando, game.AttribSpeed},
		{assets.MisslLch, game.AttribRange},
	},
}

// strategyTurnUpgradeOrders is the LONGER wish list the team buys from once it is on the
// ground and its factories stand: the landing list above plus the support units a team can
// only field later. The original keeps the two lists in one function behind a flag, and
// hands the long one to the in game buy loop.
//
// C++ ported : aiplayer.cpp - AiPlayer::ChooseStrategicBuildOrders(mode = false)
var strategyTurnUpgradeOrders = map[Strategy][]UpgradeOrder{
	StrategyDefensive: {
		{assets.AntiMssl, game.AttribRange},
		{assets.AntiMssl, game.AttribAttack},
		{assets.Radar, game.AttribScan},
		{assets.GunTurrt, game.AttribAttack},
		{assets.GunTurrt, game.AttribRounds},
		{assets.GunTurrt, game.AttribArmor},
		{assets.GunTurrt, game.AttribHits},
		{assets.GunTurrt, game.AttribRange},
		{assets.AntiAir, game.AttribRounds},
		{assets.AntiAir, game.AttribRange},
		{assets.AntiAir, game.AttribAttack},
	},
	StrategyMissiles: {
		{assets.MisslLch, game.AttribRange},
		{assets.Scanner, game.AttribScan},
		{assets.MisslLch, game.AttribAttack},
		{assets.Scanner, game.AttribSpeed},
		{assets.MisslLch, game.AttribSpeed},
		{assets.MsslBoat, game.AttribRange},
		{assets.MsslBoat, game.AttribAttack},
		{assets.MsslBoat, game.AttribSpeed},
		{assets.FastBoat, game.AttribScan},
		{assets.FastBoat, game.AttribSpeed},
	},
	StrategyAir: {
		{assets.Fighter, game.AttribAttack},
		{assets.Fighter, game.AttribRange},
		{assets.Bomber, game.AttribAttack},
		{assets.Fighter, game.AttribRounds},
		{assets.Fighter, game.AttribHits},
		{assets.Fighter, game.AttribArmor},
		{assets.Awac, game.AttribScan},
		{assets.Bomber, game.AttribRounds},
		{assets.Bomber, game.AttribHits},
		{assets.Bomber, game.AttribArmor},
	},
	StrategySea: {
		{assets.Battlshp, game.AttribAttack},
		{assets.Battlshp, game.AttribArmor},
		{assets.Battlshp, game.AttribHits},
		{assets.MsslBoat, game.AttribRange},
		{assets.Submarne, game.AttribAttack},
		{assets.Submarne, game.AttribScan},
		{assets.Battlshp, game.AttribRounds},
		{assets.MsslBoat, game.AttribAttack},
		{assets.FastBoat, game.AttribAttack},
		{assets.FastBoat, game.AttribRounds},
		{assets.FastBoat, game.AttribRange},
		{assets.FastBoat, game.AttribSpeed},
		{assets.FastBoat, game.AttribScan},
		{assets.Corvette, game.AttribAttack},
		{assets.Corvette, game.AttribRounds},
		{assets.Corvette, game.AttribSpeed},
		{assets.Corvette, game.AttribScan},
	},
	StrategyScoutHorde: {
		{assets.Scout, game.AttribAttack},
		{assets.Scout, game.AttribArmor},
		{assets.Scout, game.AttribHits},
		{assets.Scout, game.AttribSpeed},
	},
	StrategyTankHorde: {
		{assets.Scout, game.AttribSpeed},
		{assets.Tank, game.AttribAttack},
		{assets.Tank, game.AttribRounds},
		{assets.Tank, game.AttribHits},
		{assets.Tank, game.AttribArmor},
		{assets.Tank, game.AttribSpeed},
		{assets.Tank, game.AttribRange},
	},
	StrategyFastAttack: {
		{assets.Scout, game.AttribScan},
		{assets.Artillry, game.AttribAttack},
		{assets.Artillry, game.AttribRounds},
		{assets.Artillry, game.AttribRange},
		{assets.Artillry, game.AttribSpeed},
		{assets.Scout, game.AttribAttack},
		{assets.Scout, game.AttribSpeed},
		{assets.SpFlak, game.AttribAttack},
		{assets.SpFlak, game.AttribRounds},
		{assets.SpFlak, game.AttribRange},
		{assets.SpFlak, game.AttribSpeed},
		{assets.Bomber, game.AttribAttack},
		{assets.FastBoat, game.AttribAttack},
		{assets.FastBoat, game.AttribRounds},
		{assets.FastBoat, game.AttribRange},
		{assets.FastBoat, game.AttribSpeed},
		{assets.FastBoat, game.AttribScan},
		{assets.Corvette, game.AttribRange},
		{assets.Corvette, game.AttribRounds},
		{assets.Corvette, game.AttribAttack},
		{assets.Corvette, game.AttribSpeed},
		{assets.Corvette, game.AttribScan},
	},
	StrategyCombinedArms: {
		{assets.MisslLch, game.AttribRange},
		{assets.Tank, game.AttribAttack},
		{assets.Tank, game.AttribArmor},
		{assets.Tank, game.AttribHits},
		{assets.Scanner, game.AttribScan},
		{assets.Scanner, game.AttribSpeed},
		{assets.MisslLch, game.AttribAttack},
		{assets.MisslLch, game.AttribSpeed},
		{assets.Tank, game.AttribRounds},
		{assets.Tank, game.AttribSpeed},
	},
	StrategyEspionage: {
		{assets.Commando, game.AttribScan},
		{assets.Commando, game.AttribSpeed},
		{assets.MisslLch, game.AttribRange},
		{assets.Submarne, game.AttribAttack},
		{assets.Submarne, game.AttribScan},
		{assets.Scanner, game.AttribScan},
		{assets.Awac, game.AttribScan},
		{assets.MsslBoat, game.AttribRange},
		{assets.ClnTrans, game.AttribSpeed},
	},
}

// InitialUpgradeOrders is the upgrade wish list a computer team lands with: what its
// chosen strategy wants raised, cheapest first once the game layer prices it. A human
// or empty team wants nothing. The strategies are settled at AI init (the same place
// the clans are resolved), so this is answerable before a single unit deploys - which
// is when the original spends the team's landing gold.
//
// C++ ported : aiplayer.cpp - AiPlayer::ChooseInitialUpgrades (its build order half)
func (a *AI) InitialUpgradeOrders(g *sim.Game, team int) []UpgradeOrder {
	return a.upgradeOrders(g, team, strategyUpgradeOrders)
}

// TurnUpgradeOrders is the wish list a computer team buys from DURING a game, off the gold
// its refineries earn it. It is the longer of the two lists: once the team is established it
// wants its support units raised as well, not just the handful it landed with.
//
// C++ ported : aiplayer.cpp - the ChooseUpgrade call in AiPlayer::BeginTurn (:2635-2650)
func (a *AI) TurnUpgradeOrders(g *sim.Game, team int) []UpgradeOrder {
	return a.upgradeOrders(g, team, strategyTurnUpgradeOrders)
}

// upgradeOrders looks the team's strategy up in one of the two wish list tables.
func (a *AI) upgradeOrders(g *sim.Game, team int, table map[Strategy][]UpgradeOrder) []UpgradeOrder {
	if g == nil || team < 0 || team >= game.TeamCount {
		return nil
	}

	if g.State.Teams[team].Type != game.TeamComputer {
		return nil
	}

	if !a.inited || a.initedGame != g {
		a.init(g)
	}

	// A strategy still Random means the map was too degenerate to score one, so fall
	// back on the clan's default plan - the same resolution BeginTurn does.
	strategy := a.players[team].GetStrategy()
	if strategy == StrategyRandom {
		strategy = strategyForClan(a.players[team].clan)
	}

	return table[strategy]
}
