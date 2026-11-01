// levelupbench.go : provides the research level up bench.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package gamescreen

import (
	"github.com/suglasp/max-go/internal/assets"
	"github.com/suglasp/max-go/internal/dbg"
	"github.com/suglasp/max-go/internal/game"
)

// levelUpBenchZoom frames the small unit cluster clearly.
const levelUpBenchZoom = 48

// levelUpResearchLevel is the preset Cost and HITS research level (instead of
// grinding turns): high enough that the build turn drop and the HITS rise are obvious.
const levelUpResearchLevel = 5

// DeployLevelUpBench lays out the -emulate levelup bench so research upgrades can be
// checked by hand WITHOUT grinding turns. The PLAYER team has Cost + HITS research preset
// to level 5; an ENEMY team beside it stays at level 0 as the control. Each team gets a
// FULL tank and a half DAMAGED tank, deployed at level 0 then derived again through the real
// upgrade path (upgradeUnitValues) so the player tanks show the scaled result:
//   - COST: select the player constructor -> popup Build -> the build menu turns are LOWER
//     than the base (printed to the log); the enemy's would build at the base rate.
//   - HITS: the player's max HITS rises (e.g. 30 -> 45); a FULL tank stays full (45/45),
//     a damaged tank keeps its absolute missing HP gap (e.g. 15/30 -> 30/45, NOT 22/45).
//     Compare the player tanks against the enemy (base 30) tanks. C++: n/a (debug helper
//     for ResearchMenu_ApplyUpgrades COST + UnitInfo::UpgradeInt).
func (s *Screen) DeployLevelUpBench() {
	originX, originY, ok := s.snowBlock(10, 8)

	if !ok {
		originX, originY = 4, 4
	}

	const player, enemy = 0, 1
	mobile := game.FlagMobileLandUnit | game.FlagSelectable
	constr := game.FlagMobileLandUnit | game.FlagConstructorUnit | game.FlagSelectable

	// deployPair drops a full + a half damaged tank of a team at a row, returning the full one.
	deployPair := func(team, row int) (full, damaged *game.UnitInfo) {
		full = s.deployUnit(assets.Tank, mobile|game.FlagTurretSprite, team, 0, originX+2, row)
		damaged = s.deployUnit(assets.Tank, mobile|game.FlagTurretSprite, team, 0, originX+4, row)

		if damaged.Values != nil {
			damaged.Hits = damaged.Values.Get(game.AttribHits) / 2 // half HP -> a clear gap
		}

		return full, damaged
	}

	// Both teams start at research level 0, so the pairs deploy at the base max HITS.
	pFull, pDamaged := deployPair(player, originY+2)
	eFull, eDamaged := deployPair(enemy, originY+5)
	con := s.deployUnit(assets.Constrct, constr, player, 0, originX+6, originY+2)

	// Raise ONLY the player team's Cost + HITS research, then derive its units again through
	// the real upgrade path so a full tank stays full and a damaged tank keeps its gap.
	s.game.State.Teams[player].Research[game.ResearchCost].Level = levelUpResearchLevel
	s.game.State.Teams[player].Research[game.ResearchHits].Level = levelUpResearchLevel

	for _, u := range []*game.UnitInfo{pFull, pDamaged, con} {
		s.upgradeUnitValues(player, u)
	}

	s.game.State.ActiveTeam = player
	s.game.State.PlayerTeam = player
	s.selected = con
	s.showNames = true
	s.fogShow = false
	s.r.SetZoom(levelUpBenchZoom)
	s.startSingleBench("Level-up bench: player has Cost+Hits research at level 5 (enemy at 0). Build menu turns are reduced; the player tanks show higher max HITS - a full tank stays full, a damaged tank keeps its missing-HP gap. Compare vs the enemy tanks.", game.Point{X: originX + 4, Y: originY + 4}, con)

	// Log the numbers so the effect is checkable against the panel readout.
	dbg.Printf("levelup bench: player Cost+Hits research = %d (enemy 0)", levelUpResearchLevel)
	dbg.Printf("levelup bench HITS: player full %d/%d, player damaged %d/%d | enemy full %d/%d, enemy damaged %d/%d", pFull.Hits, pFull.Values.Get(game.AttribHits), pDamaged.Hits, pDamaged.Values.Get(game.AttribHits), eFull.Hits, eFull.Values.Get(game.AttribHits), eDamaged.Hits, eDamaged.Values.Get(game.AttribHits))

	for _, t := range []assets.UnitType{assets.Tank, assets.LightPlt, assets.MiningSt} {
		base := s.baseValues(t).Get(game.AttribTurns)
		reduced := s.teamValues(player, t).Get(game.AttribTurns)
		dbg.Printf("levelup bench COST: %s build turns base=%d -> Cost-level-%d=%d", t.Tag(), base, levelUpResearchLevel, reduced)
	}
}
