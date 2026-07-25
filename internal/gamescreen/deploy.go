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

	if int(t) >= 0 && int(t) < len(s.buildUI.attribs) {
		// Real ATTRIBS loaded: use the same team/clan adjusted values a factory
		// produced unit gets, so a deployed unit starts with its TRUE stats.
		// C++: a deployed and a produced unit share UnitsManager_BaseUnitValues.
		u.Values = s.teamValues(team, t)
	} else {
		// Headless benches/tests with no ATTRIBS: placeholder stats so the turn loop
		// and right click fire still work.
		u.Values = &game.UnitValues{}
		u.Values.Set(game.AttribScan, 4)
		u.Values.Set(game.AttribSpeed, 8)
		u.Values.Set(game.AttribAttack, 12)
		u.Values.Set(game.AttribArmor, 4)
		u.Values.Set(game.AttribHits, 24)
		u.Values.Set(game.AttribRounds, 1)
		u.Values.Set(game.AttribRange, 5)
		u.Values.Set(game.AttribAmmo, 6)
		u.Values.Set(game.AttribStorage, 40)
	}

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
