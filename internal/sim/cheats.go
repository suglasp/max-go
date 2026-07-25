// cheats.go : implements M.A.X. typed developer cheats.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
//
// [MAXSPY] and [MAXSURVEY] are the F2 menu's "Fog of war" and "Unlock resource map" toggles.
// This file implements the remaining three effects for the F2 menu and vanilla typed-code entry in gamescreen/cheats.go.
// The multiplayer anti-cheat flag and PunishCheater are not ported because networking is out of scope.
package sim

import "github.com/suglasp/max-go/internal/game"

// CheatFillStorage fills every team unit's cargo to ATTRIB_STORAGE capacity. Units without cargo stay at 0.
// C++ ported : game_manager.cpp - GameManager_ProcessCheatCodes CHEAT_CODE_MAXSTORAGE.
func (g *Game) CheatFillStorage(team int) {
	for _, u := range g.Units.Units() {
		if u.Team == team && u.Values != nil {
			u.Storage = u.Values.Get(game.AttribStorage)
		}
	}
}

// CheatFillAmmo fills every team unit's ammo to ATTRIB_AMMO. Units without weapons stay at 0.
// C++ ported : game_manager.cpp - GameManager_ProcessCheatCodes CHEAT_CODE_MAXAMMO.
func (g *Game) CheatFillAmmo(team int) {
	for _, u := range g.Units.Units() {
		if u.Team == team && u.Values != nil {
			u.Ammo = u.Values.Get(game.AttribAmmo)
		}
	}
}

// superChargedAttribs are the combat stats [MAXSUPER] raises to at least double.
var superChargedAttribs = []game.Attrib{
	game.AttribAttack, game.AttribArmor, game.AttribHits,
	game.AttribScan, game.AttribRange, game.AttribSpeed, game.AttribRounds,
}

// CheatSuperUnit raises one unit's attack, armor, hits, scan, range, speed, and rounds to at least twice its current base value without changing sibling units.
// It refreshes live hit points to the new maximum. Vanilla doubles team-base values; this uses the unit's values, which match for un-upgraded units.
// C++ ported : game_manager.cpp - GameManager_ProcessCheatCodes CHEAT_CODE_MAXSUPER.
func (g *Game) CheatSuperUnit(u *game.UnitInfo) {
	if u == nil || u.Values == nil {
		return
	}

	base := u.Values
	v := base.Clone()
	v.SetVersion(30) // the original bumps the boosted set to a high upgrade version
	v.MarkInUse()

	for _, a := range superChargedAttribs {
		want := 2 * base.Get(a)
		if want > 0xFFFF {
			want = 0xFFFF // the attribute store is 16 bit; do not let it wrap
		}

		if v.Get(a) < want {
			v.Set(a, want)
		}
	}

	u.Values = v
	u.Hits = v.Get(game.AttribHits)
}
