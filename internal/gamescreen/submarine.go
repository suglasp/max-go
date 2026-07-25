// submarine.go : implements the submarine stealth and surface rendering.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
// C++ ported : unitinfo.cpp - submarine stealth/surface state
package gamescreen

// submarine.go - the submarine surface/submerge state. A submarine renders
// submerged by default and only surfaces (shows its visible sprite) when it
// attacks, is attacked, or is spotted by an enemy. Per the original's source the only
// unit that detects a submerged submarine is the CORVETTE (sonar) - the heat
// map's sea stealth qualifier is `unit->GetUnitType() == CORVETTE`
// (resource_manager.cpp ResourceManager_InitHeatMaps); the AWAC does NOT spot
// submarines. Surfacing on spot is Access_DrawUnit -> SpotByTeam (access.cpp:703,
// unitinfo.cpp:2792); the combat surfacing mirrors a sub revealing itself to fire.

import (
	"github.com/suglasp/max-go/internal/assets"
	"github.com/suglasp/max-go/internal/game"
)

// subCombatSurfaceMS is how long a submarine stays surfaced after a combat event
// (firing or being hit) before it can submerge again.
const subCombatSurfaceMS = 3000

// surfaceSubmarine surfaces a submarine after a combat event and holds it up for
// a short window (it has broken the surface to attack or has been struck). A
// Does nothing for any other unit type.
func (s *Screen) surfaceSubmarine(u *game.UnitInfo) {
	if u == nil || u.Type != assets.Submarne {
		return
	}

	if s.announcer.subSurfacedUntil == nil {
		s.announcer.subSurfacedUntil = map[uint16]uint64{}
	}

	u.Surfaced = true
	s.announcer.subSurfacedUntil[u.ID] = s.now() + subCombatSurfaceMS
}

// updateSubmarines recomputes every submarine's surfaced state each frame: a
// submarine surfaces while an enemy can detect it (an enemy Corvette's sonar
// covers its cell) or within the brief window after a combat event, and
// resubmerges otherwise. C++: Access_DrawUnit spots a submerged unit when an
// enemy heat map's stealth sea covers it; UnitInfo::Draw resets image_base to 0
// (submerged) once no enemy sees it.
func (s *Screen) updateSubmarines() {
	now := s.now()

	for _, u := range s.game.Units.Units() {
		if u.Type != assets.Submarne {
			continue
		}

		recentCombat := s.announcer.subSurfacedUntil != nil && now < s.announcer.subSurfacedUntil[u.ID]
		u.Surfaced = recentCombat || s.subSpottedByEnemy(u)
	}
}

// subSpottedByEnemy reports whether any rival team currently detects the unit:
// its submerged cell is under that team's sonar coverage. VisibleToTeam already
// applies the sea stealth gate (only a Corvette feeds it) and the idle unit
// exemption, so an idle submarine stays hidden. C++: the StealthSea check in
// Access_DrawUnit.
func (s *Screen) subSpottedByEnemy(u *game.UnitInfo) bool {
	for t := 0; t < game.TeamCount; t++ {
		if t == u.Team || s.game.State.Teams[t].Type == game.TeamNone {
			continue
		}

		if s.game.Vis.VisibleToTeam(u, t) {
			return true
		}
	}

	return false
}
