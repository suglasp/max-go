// deploy.go : places a unit on the map with its sprite frames and combat stats wired up.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package gamescreen

import (
	"github.com/suglasp/max-go/internal/assets"
	"github.com/suglasp/max-go/internal/game"
)

// deployAt builds the next demo unit (cycling type/team/facing) and places
// it at the given cell, wiring its sprite frame bases from the type's data.
func (s *Screen) deployAt(cellX, cellY int) *game.UnitInfo {
	def := debugUnitTypes[s.spawnSeq%len(debugUnitTypes)]
	team := s.spawnSeq % 5
	angle := uint8(s.spawnSeq % game.AngleCount)

	return s.deployUnit(def.t, def.flags, team, angle, cellX, cellY)
}

// amphibious reports whether a unit type is a genuine dual flag amphibian
// (MOBILE_LAND_UNIT | MOBILE_SEA_UNIT): the Personnel Carrier. Surveyor/
// Constructor/Engineer/Scout drive onto water via a land_type override instead.
// C++ ported : the MOBILE_LAND_UNIT | MOBILE_SEA_UNIT unit flagging.
func amphibious(t assets.UnitType) bool {
	return t == assets.ClnTrans
}

// deployUnit places one unit of an explicit type/flags/team/facing at a cell,
// wired with the demo combat/scan/speed stats so movement, fog and fire all
// work. It is the parametric core shared by the U spawn demo and -random start.
func (s *Screen) deployUnit(t assets.UnitType, flags game.Flags, team int, angle uint8, cellX, cellY int) *game.UnitInfo {
	u := game.NewUnitInfo(t, team, s.spawnSeq+1, angle)

	if amphibious(t) { // ORed here, the one chokepoint every mobile spawn passes through
		flags |= game.FlagMobileSeaUnit
	}

	if game.CanHeadRotateType(t) {
		flags |= game.FlagTurretSprite | game.FlagCanHeadRotate
	}

	u.Flags = flags
	u.Values = s.deployValues(team, t)
	u.InitState() // Hits/Ammo/Shots/Speed from the values above
	u.SetAngle(angle)
	u.TurretAngle = angle

	if flags.Has(game.FlagConstructorUnit) {
		u.Storage = u.Values.Get(game.AttribStorage) // start full of raw material to spend on construction
	}

	s.game.Units.Deploy(u, cellX, cellY)
	s.spawnSeq++
	return u
}

// deployValues is a deployed unit's stats: with real ATTRIBS loaded, the same team and clan adjusted values a
// factory produced unit gets, so it starts with its TRUE stats; in a headless bench or test with no ATTRIBS,
// placeholder stats so the turn loop and fire still work.
// C++: a deployed and a produced unit share UnitsManager_BaseUnitValues.
func (s *Screen) deployValues(team int, t assets.UnitType) *game.UnitValues {
	if int(t) >= 0 && int(t) < len(s.buildUI.attribs) {
		return s.teamValues(team, t)
	}

	v := &game.UnitValues{}
	v.Set(game.AttribScan, 4)
	v.Set(game.AttribSpeed, 8)
	v.Set(game.AttribAttack, 12)
	v.Set(game.AttribArmor, 4)
	v.Set(game.AttribHits, 24)
	v.Set(game.AttribRounds, 1)
	v.Set(game.AttribRange, 5)
	v.Set(game.AttribAmmo, 6)
	v.Set(game.AttribStorage, 40)

	return v
}

// assignSprite fills a unit's sprite frame bases from its type's sheet, so units
// the sim creates internally get the right partition without headless sprite knowledge.
func (s *Screen) assignSprite(u *game.UnitInfo) {
	if set := s.sprites.Set(u.Type); set != nil {
		u.ImageBase = set.Frame.ImageBase
		u.TurretImageBase = set.Frame.TurretImageBase
	}

	if u.Type == assets.MiningSt { // 16 body frames are the eight clan emblems, not an animation
		u.ImageBase = clanEmblemFrame(s.teamClan(u.Team))
	}
}
