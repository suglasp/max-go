// voiceunits.go : speaks the lines about units: what a selected unit answers, what a
// damaged one reports, and the cues for spotting an enemy or a submarine.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
// C++ ported : game_manager.cpp - GameManager_PlayUnitStatusVoice + GameManager_NotifyEvent
package gamescreen

import (
	"fmt"

	"github.com/suglasp/max-go/internal/assets"
	"github.com/suglasp/max-go/internal/game"
)

// playBuildPlacementPrompt speaks the F049/F050 placement prompt when a builder arms placement in a TRAINING mission. Other mission categories stay silent.
//
// C++ ported : buildmenu.cpp:931 - PlayVoice(V_M049, V_F050) under
// mission_category == MISSION_CATEGORY_TRAINING
func (s *Screen) playBuildPlacementPrompt() {
	if s.game.Mission != game.MissionTraining {
		return
	}

	s.playVoice(vsBuildPlacement)
}

// unitStatusVoice speaks the status line for a freshly selected unit. Ammo, movement
// and damage warnings take priority; else the line reflects the unit's order or type
// (surveying, attacking, laying mines, on sentry, building, or "Ready!"). Player team's
// own selectable units only. C++ ported : game_manager.cpp - GameManager_PlayUnitStatusVoice
func (s *Screen) unitStatusVoice(u *game.UnitInfo) {
	if u == nil || u.Team != s.game.State.PlayerTeam || !u.Flags.Has(game.FlagSelectable) {
		return
	}

	// The select blip is the default in world selection sound: it plays on every own
	// unit/building select, on the SFX channel, independent of any ready/status voice
	// that may follow on the (separate) voice channel.
	s.playSelectBlip()

	// A build READY builder (structure just finished, BLDMRK markers showing) or a
	// factory with a unit ready announces completion on SELECT, matching the original's
	// ORDER_STATE_BUILDING_READY / UNIT_READY status voice. C++:
	// GameManager_PlayUnitStatusVoice ORDER_BUILD branch.
	if s.game.IsBuildReady(u) {
		if br := s.game.BuildReadyInfo(u); br != nil && br.Factory {
			s.playVoice(vsUnitComplete)
		} else {
			s.playVoice(vsBuildComplete)
		}

		return
	}

	// Only mobile units acknowledge selection; a building is otherwise silent.
	// EXCEPTION (by design, deviates from C++): a defense building ON GUARD speaks the
	// blended ready/sentry pool so a clicked turret can answer "On sentry" - the rest
	// play just the select blip.
	if !isMobileUnit(u) {
		switch {
		case isDefenseBuilding(u) && u.Order == game.OrderSentry:
			s.playVoice(vsSelectSentry)
		case s.game.FactoryProducing(u):
			s.playVoice(vsBuilding) // "Building!" - a factory mid production
		}

		return
	}

	maxHits, maxAmmo := 0, 0

	if u.Values != nil {
		maxHits = u.Values.Get(game.AttribHits)
		maxAmmo = u.Values.Get(game.AttribAmmo)
	}

	switch {
	case maxAmmo > 0 && u.Ammo == 0:
		s.playVoice(vsAmmoDepleted)
	case maxAmmo > 0 && u.Ammo*4 <= maxAmmo:
		s.playVoice(vsAmmoLow)
	case isMobileUnit(u) && u.Speed == 0:
		s.playVoice(vsMovementExhausted)
	case maxHits > 0 && u.Hits*4 <= maxHits:
		s.playVoice(vsSelectCritical) // "Status critical" / "Unit must be repaired!" / "Requesting repairs!"
	case maxHits > 0 && u.Hits*2 <= maxHits:
		s.playVoice(vsSelectDamaged) // "Status yellow" / one of the two repair calls
	default:
		s.playVoice(unitOrderVoice(u))
	}
}

// unitOrderVoice maps a healthy unit's type/order to the line it acknowledges on
// selection: a surveyor surveys, an attacker attacks, a mine layer lays mines, and
// so on; anything else is a generic "Ready!".
func unitOrderVoice(u *game.UnitInfo) voiceState {
	if u.Type == assets.Surveyor {
		return vsSurveying
	}

	switch u.Order {
	case game.OrderMoveToAttack:
		return vsAttacking
	case game.OrderLayMine:
		// The original distinguishes laying vs removing by order state; default to laying.
		return vsLayingMines
	case game.OrderClear:
		return vsClearing
	case game.OrderSentry:
		// Selecting a sentry unit answers from the blended ready/sentry pool (F158 is
		// one of several picks), not a fixed "On sentry" every time. The deterministic
		// vsOnSentry stays for the explicit Sentry toggle confirmation.
		return vsSelectSentry
	case game.OrderBuild:
		return vsBuilding
	}

	return vsSelectUnit
}

// isMobileUnit reports whether u moves under its own power (land, sea or air).
func isMobileUnit(u *game.UnitInfo) bool {
	return u.Flags.Has(game.FlagMobileLandUnit) ||
		u.Flags.Has(game.FlagMobileSeaUnit) ||
		u.Flags.Has(game.FlagMobileAirUnit)
}

// latchEventCell remembers the cell of the latest event message for the F1 key: an enemy coming into view, one of
// the player's units hit or lost, or a commando acting on one of them. Each new event moves it.
//
// C++ ported : game_manager.cpp - GameManager_NotifyEvent (GameManager_SpottedEnemyPosition)
func (s *Screen) latchEventCell(cellX, cellY int) {
	s.announcer.lastEventCellX, s.announcer.lastEventCellY = cellX, cellY
	s.announcer.hasEventCell = true
}

// notifyDamage tells the player that one of their own units takes a hit, using a banner and a
// voice line the way the original raises the two together. The banner is the type 1
// "under attack" line; the voice scales with how hurt the unit is: "Status critical" at
// a quarter hits, "Status yellow" at a half, else "Unit under attack". A killed unit is
// handled by the casualty drain instead, so this only fires while it still has hits.
// C++ ported : game_manager.cpp GameManager_NotifyEvent case 1 (fd20 "%s under attack!"
// banner plus the V_M229 status voice).
func (s *Screen) notifyDamage(u *game.UnitInfo) {
	if u == nil || u.Team != s.game.State.PlayerTeam || u.Values == nil {
		return
	}

	maxHits := u.Values.Get(game.AttribHits)

	if maxHits <= 0 || u.Hits <= 0 {
		return
	}

	// The banner names the unit that takes the hit (world tint, type 1), and F1 jumps to it.
	s.showInfo(fmt.Sprintf(s.lang.Get("fd20", "%s under attack!"), displayName(u.Type)))
	s.latchEventCell(u.GridX, u.GridY)

	// The status voice reflects the remaining hits.
	switch {
	case u.Hits*4 <= maxHits:
		s.playVoice(vsStatusRed)
	case u.Hits*2 <= maxHits:
		s.playVoice(vsStatusYellow)
	default:
		s.playVoice(vsUnderAttack)
	}
}

// checkEnemyDetected announces the first time an enemy unit becomes visible. Each
// enemy is announced once (id remembered); the state's 5s cooldown keeps a wave of
// newly revealed units to a single line.
// C++ : the enemy spotted branch of GameManager_NotifyEvent (V_F070/V_F071).
func (s *Screen) checkEnemyDetected() {
	team := s.game.State.PlayerTeam

	if s.game.Vis == nil {
		return
	}

	if s.announcer.enemySeen == nil {
		s.announcer.enemySeen = map[uint16]bool{}
	}

	newly := false

	for _, u := range s.game.Units.Units() {
		if u.Team == team || u.Team < 0 || !u.Flags.Has(game.FlagSelectable) {
			continue
		}

		// A submarine getting detected says "Submarine detected!" (checkSubmarineDetected),
		// a distinct cue from the general "Enemy detected!" - so skip subs here.
		if u.Type == assets.Submarne {
			continue
		}

		if !s.game.Vis.VisibleToTeam(u, team) || s.announcer.enemySeen[u.ID] {
			continue
		}

		s.announcer.enemySeen[u.ID] = true
		newly = true
		s.latchEventCell(u.GridX, u.GridY) // F1 jumps to where this hostile came into view
	}

	if newly {
		s.playEnemySpottedPing()      // PING.WAV alert blip on the SFX channel
		s.playVoice(vsEnemyDetected)  // "Enemy detected!" voice line
		s.showInfo("Enemy detected!") // world tint event: a new hostile came into view (the original's type 1)
	}
}

// checkSubmarineDetected announces when the player's sonar (Corvette, AWAC or Scanner)
// reveals a hostile SUBMARINE - the sub stealth equivalent of checkEnemyDetected with
// its own "Submarine detected!" cue. Announced once per detection: the latch rearms
// when the sub slips back into hiding, so finding it again reannounces.
// C++ : unitinfo.cpp - the V_M201/V_F201 branch, distinct from V_F070 enemy detected.
func (s *Screen) checkSubmarineDetected() {
	team := s.game.State.PlayerTeam

	if s.game.Vis == nil {
		return
	}

	if s.announcer.subDetected == nil {
		s.announcer.subDetected = map[uint16]bool{}
	}

	newly := false

	for _, u := range s.game.Units.Units() {
		if u.Type != assets.Submarne || u.Team == team || u.Team < 0 {
			continue
		}

		// Detected = the player's sonar covers the sub's cell (VisibleToTeam applies the
		// sea stealth gate). Rearm the latch the moment it slips back out of coverage.
		if s.game.Vis.VisibleToTeam(u, team) {
			if !s.announcer.subDetected[u.ID] {
				s.announcer.subDetected[u.ID] = true
				newly = true
			}
		} else {
			delete(s.announcer.subDetected, u.ID)
		}
	}

	if newly {
		s.playEnemySpottedPing()          // PING.WAV alert blip
		s.playVoice(vsSubmarineDetected)  // "Submarine detected!" voice line
		s.showInfo("Submarine detected!") // world tint event: a sub came into view (the original's type 1)
	}
}
