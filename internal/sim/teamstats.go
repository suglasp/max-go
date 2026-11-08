// teamstats.go : records each team's production, losses, material use, upgrades, and score.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package sim

import (
	"github.com/suglasp/max-go/internal/assets"
	"github.com/suglasp/max-go/internal/game"
)

// TeamStats accumulates the per team production and loss tallies shown on the
// end of game score screen (menu_draw_game_over_screen). Every field starts at
// zero and only ever grows over a match; they are persisted in the save so the
// score table stays correct after a load.
//
// C++ ported : ctinfo.hpp - CTInfo.stats_factories_built/mines_built/
// buildings_built/units_built/gold_spent_on_upgrades + casualties[].
type TeamStats struct {
	FactoriesBuilt int // ++ when a Shipyard/LightPlt/LandPlt/AirPlt enters play
	MinesBuilt     int // ++ when a mining station enters play
	BuildingsBuilt int // ++ when any BUILDING enters play
	UnitsBuilt     int // ++ when a mobile unit is produced or landed
	BuildingsLost  int // ++ when a BUILDING is destroyed
	UnitsLost      int // ++ when a SELECTABLE unit is destroyed (a building counts here too)
	GoldOnUpgrades int // += gold spent buying unit upgrades

	// Casualties counts the losses PER UNIT TYPE, which is what the Reports screen's
	// Casualties tab lists (one row per type anybody has lost, a column per team). The
	// score screen's two totals above are the same losses summed by flag class.
	// C++ ported : ctinfo.hpp - CTInfo.casualties[UNIT_END]
	Casualties map[assets.UnitType]int
}

// isFactoryType is the original's UnitsManager_IsFactory: only the four vehicle/ship/air
// plants count as factories for the score tally - the Barracks and Training Hall
// do NOT, even though the player build UI treats them as
// production buildings. Defined locally so sim does not import menus.
//
// C++ ported : units_manager.cpp - UnitsManager_IsFactory.
func isFactoryType(t assets.UnitType) bool {
	switch t {
	case assets.Shipyard, assets.LightPlt, assets.LandPlt, assets.AirPlt:
		return true
	default:
		return false
	}
}

// recordBuildingBuilt tallies a building entering a team's play, mirroring the
// stats bumped in UnitsManager_UpdateConnectors: any BUILDING bumps
// BuildingsBuilt; a factory also bumps FactoriesBuilt; a mining station also
// bumps MinesBuilt. A bare connector/road lacks the BUILDING flag and is not
// counted (faithful to the original); a ground cover slab is skipped (the large slab
// carries FlagBuilding only as a sprite anchor hint).
//
// C++ ported : units_manager.cpp - UnitsManager_UpdateConnectors stats block.
func (g *Game) recordBuildingBuilt(u *game.UnitInfo) {
	if u == nil || !u.Flags.Has(game.FlagBuilding) || u.Flags.Has(game.FlagGroundCover) {
		return
	}
	if u.Team < 0 || u.Team >= game.TeamCount {
		return
	}
	g.TeamStats[u.Team].BuildingsBuilt++
	if isFactoryType(u.Type) {
		g.TeamStats[u.Team].FactoriesBuilt++
	}
	if u.Type == assets.MiningSt {
		g.TeamStats[u.Team].MinesBuilt++
	}
}

// recordUnitBuilt tallies a mobile unit produced by a factory or deployed in the
// team's landing roster.
//
// C++ ported : game_manager.cpp - stats_units_built (unit completion / supply).
func (g *Game) recordUnitBuilt(team int) {
	if team < 0 || team >= game.TeamCount {
		return
	}
	g.TeamStats[team].UnitsBuilt++
}

// recordCasualty tallies a destroyed unit into its owner's loss counters, then
// recurses into any units it was carrying (a destroyed transport loses its
// passengers too - they live off the unit registry and are never destroyed
// individually). A BUILDING bumps BuildingsLost; a SELECTABLE unit bumps
// UnitsLost (a building is both, so it lands in both sums - matching the score
// screen's two casualty totals). Ground cover (slabs) is skipped.
//
// C++ ported : units_manager.cpp - ++casualties[type] (incl. carried units);
// menu.cpp - the BUILDING vs SELECTABLE casualty sums.
func (g *Game) recordCasualty(u *game.UnitInfo) {
	if u == nil || u.Flags.Has(game.FlagGroundCover) {
		return
	}
	if u.Team >= 0 && u.Team < game.TeamCount {
		if u.Flags.Has(game.FlagBuilding) {
			g.TeamStats[u.Team].BuildingsLost++
		}
		if u.Flags.Has(game.FlagSelectable) {
			g.TeamStats[u.Team].UnitsLost++
		}

		// The per type tally the Reports screen's Casualties tab reads.
		if g.TeamStats[u.Team].Casualties == nil {
			g.TeamStats[u.Team].Casualties = map[assets.UnitType]int{}
		}

		g.TeamStats[u.Team].Casualties[u.Type]++
	}
	for _, c := range u.Carried {
		g.recordCasualty(c)
	}
}

// RecordUpgradeGold adds gold spent on unit upgrades to a team's tally (the
// score screen's "Upgrades" row). A non positive delta is ignored.
//
// C++ ported : abstractupgrademenu.cpp - stats_gold_spent_on_upgrades.
func (g *Game) RecordUpgradeGold(team, delta int) {
	if delta <= 0 || team < 0 || team >= game.TeamCount {
		return
	}
	g.TeamStats[team].GoldOnUpgrades += delta
}
