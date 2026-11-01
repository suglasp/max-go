// combat.go : implements the armed player orders: attack, infiltrate, reload and repair.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package gamescreen

import (
	"fmt"

	"github.com/suglasp/max-go/internal/assets"
	"github.com/suglasp/max-go/internal/combat"
	"github.com/suglasp/max-go/internal/dbg"
	"github.com/suglasp/max-go/internal/game"
	"github.com/suglasp/max-go/internal/platform"
	"github.com/suglasp/max-go/internal/winloss"
)

// canSeeTargetAt reports whether the enemy targeted by a left click is on the player's visible heat map. CanAttack has no fog gate, so this prevents blind fire into fog.
func (s *Screen) canSeeTargetAt(attacker *game.UnitInfo, cellX, cellY int) bool {
	if !s.fogShow || s.game.Vis == nil {
		return true
	}

	for _, e := range s.game.Units.Cells.At(cellX, cellY) {
		if e == attacker || e.Team == attacker.Team {
			continue
		}

		if e.Flags.Has(game.FlagGroundCover) || e.Type == assets.Bridge || e.Type == assets.Waldo || e.Type == assets.Harvster {
			continue
		}

		return s.unitVisibleToPlayer(e)
	}

	return true
}

// armAttack arms a target pick from the popup's Attack order: the unit becomes
// selected (ENMY_PTR cursor lights valid targets) and the next left click fires.
// C++: UnitsManager_Popup_OnClick_Attack.
func (s *Screen) armAttack(u *game.UnitInfo) {
	s.orderInput.popupOpen = false
	s.setSelected(u)
	s.orderInput.attackArmed = true
	s.showMessage("Pick a target to attack (ESC cancels).")
}

// cancelAttack disarms a pending attack target pick.
func (s *Screen) cancelAttack() {
	if s.orderInput.attackArmed {
		s.orderInput.attackArmed = false
		s.showMessage("Attack cancelled.")
	}
}

// armInfiltrate arms a commando's Disable (steal=false) or Steal (steal=true)
// target pick: the next left click on an adjacent enemy resolves it.
// C++ ported : units_manager.cpp - UnitsManager_Popup_OnClick_Disable/Steal.
func (s *Screen) armInfiltrate(u *game.UnitInfo, steal bool) {
	s.orderInput.popupOpen = false
	s.setSelected(u)
	s.orderInput.infiltrateArmed = true
	s.orderInput.infiltrateSteal = steal

	if steal {
		s.showMessage("Pick an enemy unit to steal (ESC cancels).")
	} else {
		s.showMessage("Pick an enemy to disable (ESC cancels).")
	}
}

// cancelInfiltrate disarms a pending disable/steal target pick.
func (s *Screen) cancelInfiltrate() {
	if s.orderInput.infiltrateArmed {
		s.orderInput.infiltrateArmed = false
		s.showMessage("Cancelled.")
	}
}

// infiltrate resolves the armed commando action against the enemy in (cellX,
// cellY): disable shuts it down, steal captures it. It reports the outcome via a
// message + voice. C++: GameManager_DisableUnit / GameManager_StealUnit.
func (s *Screen) infiltrate(cellX, cellY int) {
	u := s.selected

	if u == nil || u.Team != s.game.State.ActiveTeam {
		return
	}

	// The sim queues a failed attempt as a CommandoExposed event. tickCommandoEvents reports it to the team whose unit the infiltrator targeted.
	var ok bool

	if s.orderInput.infiltrateSteal {
		ok, _ = s.game.Steal(u, cellX, cellY)
	} else {
		ok, _ = s.game.Disable(u, cellX, cellY)
	}

	switch {
	case ok && s.orderInput.infiltrateSteal:
		s.showMessage("Unit captured!")
		s.playVoice(vsUnitCaptured)
	case ok:
		s.showMessage("Unit disabled.")
		s.playVoice(vsUnitDisabled)
	default:
		s.showMessage("Infiltration unsuccessful!")
		s.playVoice(vsInfiltrateFailed)
	}
}

// armReload arms a Supply Truck's Reload order: the truck becomes the selection (so
// the PTR_RLD cursor lights over a reloadable friendly) and the next left click on
// an adjacent out of ammo unit refills it.
// C++ ported : units_manager.cpp - UnitsManager_Popup_InitReloaders (Reload order).
func (s *Screen) armReload(u *game.UnitInfo) {
	s.orderInput.popupOpen = false
	s.setSelected(u)
	s.orderInput.reloadArmed = true
	s.showMessage("Pick a unit to reload (ESC cancels).")
}

// cancelReload disarms a pending reload target pick.
func (s *Screen) cancelReload() {
	if s.orderInput.reloadArmed {
		s.orderInput.reloadArmed = false
		s.showMessage("Reload cancelled.")
	}
}

// reloadAt resolves the armed reload against the friendly in (cellX, cellY): the
// truck spends one material and the target's ammo refills. C++: ProcessOrderReload.
func (s *Screen) reloadAt(cellX, cellY int) {
	u := s.selected

	if u == nil || u.Team != s.game.State.ActiveTeam {
		return
	}

	if s.game.ReloadAt(u, cellX, cellY) {
		s.showMessage("Unit reloaded.")
		s.playVoice(vsReloaded)                 // "Unit reloaded" acknowledgement (V_F085/089)
		s.playSfxForUnit(u, game.SfxPowerStart) // SFX_TYPE_POWER_CONSUMPTION_START, as UnitInfo::Reload
		dbg.Printf("reload: %s rearms unit at (%d,%d); cargo left %d", displayName(u.Type), cellX, cellY, u.Storage)
	} else {
		s.showMessage("Cannot reload that unit.")
	}
}

// armRepair arms a mobile Repair unit's Repair order: the repairer becomes the
// selection (so the FRND_FIX cursor lights over a damaged friendly) and the next
// left click on an adjacent damaged unit mends it.
// C++ ported : units_manager.cpp - UnitsManager_Popup_InitRepair (Repair order).
func (s *Screen) armRepair(u *game.UnitInfo) {
	s.orderInput.popupOpen = false
	s.setSelected(u)
	s.orderInput.repairArmed = true
	s.showMessage("Pick a unit to repair (ESC cancels).")
}

// cancelRepair disarms a pending repair target pick.
func (s *Screen) cancelRepair() {
	if s.orderInput.repairArmed {
		s.orderInput.repairArmed = false
		s.showMessage("Repair cancelled.")
	}
}

// repairAt resolves the armed repair against the friendly in (cellX, cellY): the
// repairer spends material and the target's hit points mend. C++: ProcessOrderRepair.
func (s *Screen) repairAt(cellX, cellY int) {
	u := s.selected

	if u == nil || u.Team != s.game.State.ActiveTeam {
		return
	}

	if s.game.RepairAt(u, cellX, cellY) {
		s.showMessage("Unit repaired.")
		s.playVoice(vsRepaired)                 // "Unit repaired" acknowledgement (V_F210/211/219/220)
		s.playSfxForUnit(u, game.SfxPowerStart) // SFX_TYPE_POWER_CONSUMPTION_START, as UnitInfo::Repair
		dbg.Printf("repair: %s mends unit at (%d,%d); cargo left %d", displayName(u.Type), cellX, cellY, u.Storage)
	} else {
		s.showMessage("Cannot repair that unit.")
	}
}

// hitBand classifies a unit's remaining health the way the HITS gauge colours it:
// "ok" above half, "caution" above a quarter, else "critical" (the blue/yellow/red
// bands). Used to annotate combat debug/log lines. C++: the I_HITS gauge thresholds.
func hitBand(u *game.UnitInfo) string {
	max := 0

	if u.Values != nil {
		max = u.Values.Get(game.AttribHits)
	}

	switch {
	case max <= 0 || u.Hits*2 > max:
		return "ok"
	case u.Hits*4 > max:
		return "caution"
	default:
		return "critical"
	}
}

// attack fires the current selection's leader at the cell. C++: the click the target
// attack flow (GameManager left click ORDER_FIRE / the popup Attack order).
func (s *Screen) attack(cellX, cellY int, force bool) {
	s.attackWith(s.selected, cellX, cellY, force)
}

// attackWith fires u at (cellX, cellY). Group fire loops every member in range
// through here; the single unit path passes s.selected.
func (s *Screen) attackWith(u *game.UnitInfo, cellX, cellY int, force bool) {
	if u == nil || u.Team != s.game.State.ActiveTeam {
		return
	}

	// A quick attack within a just ordered move's bulb dwell cancels that deferred
	// move, so the stale move cannot fire over this attack a moment later.
	s.cancelPlanningBulb()

	// A FORCE ATTACK (SHIFT/CTRL) may fire on a friendly, so the target search drops
	// the enemy team filter; a normal attack engages only the first enemy in the cell.
	var target *game.UnitInfo

	for _, e := range s.game.Units.Cells.At(cellX, cellY) {
		if e == u || e.Flags.Has(game.FlagGroundCover) {
			continue
		}

		if force || e.Team != u.Team {
			target = e
			break
		}
	}

	projBefore := len(s.game.ActiveProjectiles())

	// Every unit fires from its current cell and holds position: air units (Fighter/
	// Bomber) hover to attack, they do not run a strafing pass over the target. The
	// drive INTO range is a separate move to attack step before this shot.
	var res combat.Result
	var ok bool

	switch {
	case force:
		res, ok = s.game.ForceAttack(u, cellX, cellY)
	default:
		res, ok = s.game.Attack(u, cellX, cellY)
	}

	if !ok {
		return
	}

	// A missile firer spent its shot but damage resolves on arrival; log the launch. Its hit message waits
	// for the impact (tickProjectiles), since res holds no damage yet.
	missile := target != nil && len(s.game.ActiveProjectiles()) > projBefore
	if missile {
		maxAmmo := 0

		if u.Values != nil {
			maxAmmo = u.Values.Get(game.AttribAmmo)
		}

		dbg.Printf("fire: %s launches missile -> %s (ammo %d/%d, shots %d)", displayName(u.Type), displayName(target.Type), u.Ammo, maxAmmo, u.Shots)
	}

	u.AimTurretAt(game.TargetAngle(cellX-u.GridX, cellY-u.GridY)) // turret rotates toward it over the next frames

	s.surfaceSubmarine(u) // an attacking or struck submarine surfaces briefly. C++: SUBMARNE image_base 8.

	if !res.Destroyed {
		s.surfaceSubmarine(target)
	}

	s.playSfxForUnit(u, game.SfxFire) // C++: SoundManager::PlaySfx(attacker, SFX_TYPE_FIRE)

	// The sim queues each impact. tickCasualties presents a kill and tickDamage presents a nonfatal hit, whether the shot came from the player or a computer team.

	// A not fatal hit reports damage here; a kill's message comes from tickCasualties.
	if !res.Destroyed && !missile {
		s.showMessage(s.hitMessage(target, res.Damage))
	}

	s.game.UpdateEliminations()

	if !s.debugMenu.endless { // the F2 "Endless mode" flag suppresses the win/loss check

		if o := s.game.Outcome(); o != winloss.Continue {
			s.enterGameOver(o)
		}
	}
}

// destroyUnit blows a unit up (footprint aware explosion + rubble) and removes
// it from the game; a destroyed building's foundation slab is cleared too, so
// only rubble remains. Clears the selection if it was the destroyed unit.
func (s *Screen) destroyUnit(u *game.UnitInfo) {
	if u != nil && u.Team == s.game.State.PlayerTeam {
		s.playVoice(vsUnitDestroyed) // "Unit destroyed!"
	}

	s.spawnExplosion(u)
	s.game.CancelOrder(u)
	s.game.DestroyUnit(u) // removes the unit and (for a building) its slab
	s.game.RefreshVisibility()

	s.forgetSelectedUnit(u)
}

// handleAttackPick resolves an armed attack pick: a left click fires on the cell, a
// right click or a click off the map cancels. SHIFT/CTRL force fires on whatever is
// under the pointer. Returns whether it consumed the click.
func (s *Screen) handleAttackPick(mouse platform.MouseState) bool {
	if s.orderInput.attackArmed && s.turnFlow.handover == -1 && !s.gameOver {
		if mouse.Buttons&platform.MouseReleaseRight != 0 {
			s.cancelAttack()
			return true
		}

		if mouse.Buttons&platform.MouseReleaseLeft != 0 {
			if cellX, cellY, ok := s.mapCell(mouse.X, mouse.Y); ok && s.selected != nil {
				force := s.forceModifierHeld() // SHIFT/CTRL: force attack whatever is under the pointer
				if force && s.game.CanForceAttack(s.selected, cellX, cellY) {
					s.attack(cellX, cellY, true)
					s.orderInput.attackArmed = false
					return true
				}

				if s.game.CanAttack(s.selected, cellX, cellY) {
					s.attack(cellX, cellY, false)
					s.orderInput.attackArmed = false
					return true
				}
			}

			s.cancelAttack()

			return true
		}
	}

	return false
}

// handleInfiltratePick resolves an armed commando Disable or Steal pick: a left click
// on an adjacent enemy carries it out. Returns whether it consumed the click.
func (s *Screen) handleInfiltratePick(mouse platform.MouseState) bool {
	if s.orderInput.infiltrateArmed && s.turnFlow.handover == -1 && !s.gameOver {
		if mouse.Buttons&platform.MouseReleaseRight != 0 {
			s.cancelInfiltrate()

			return true
		}

		if mouse.Buttons&platform.MouseReleaseLeft != 0 {
			cellX, cellY, ok := s.mapCell(mouse.X, mouse.Y)
			valid := ok && s.selected != nil
			if valid && s.orderInput.infiltrateSteal {
				valid = s.game.CanStealAt(s.selected, cellX, cellY)
			} else if valid {
				valid = s.game.CanDisableAt(s.selected, cellX, cellY)
			}

			if valid {
				s.infiltrate(cellX, cellY)
				s.orderInput.infiltrateArmed = false
				return true
			}

			s.cancelInfiltrate()

			return true
		}
	}

	return false
}

// handleReloadPick resolves an armed reload pick: a left click on an adjacent
// reloadable friendly carries it out. Returns whether it consumed the click.
func (s *Screen) handleReloadPick(mouse platform.MouseState) bool {
	if s.orderInput.reloadArmed && s.turnFlow.handover == -1 && !s.gameOver {
		if mouse.Buttons&platform.MouseReleaseRight != 0 {
			s.cancelReload()
			return true
		}

		if mouse.Buttons&platform.MouseReleaseLeft != 0 {
			if cellX, cellY, ok := s.mapCell(mouse.X, mouse.Y); ok && s.selected != nil &&
				s.game.CanReloadAt(s.selected, cellX, cellY) {
				s.reloadAt(cellX, cellY)
				s.orderInput.reloadArmed = false
				return true
			}

			s.cancelReload()
			return true
		}
	}

	return false
}

// handleRepairPick resolves an armed repair pick: a left click on an adjacent damaged
// friendly carries it out. Returns whether it consumed the click.
func (s *Screen) handleRepairPick(mouse platform.MouseState) bool {
	if s.orderInput.repairArmed && s.turnFlow.handover == -1 && !s.gameOver {
		if mouse.Buttons&platform.MouseReleaseRight != 0 {
			s.cancelRepair()

			return true
		}

		if mouse.Buttons&platform.MouseReleaseLeft != 0 {
			if cellX, cellY, ok := s.mapCell(mouse.X, mouse.Y); ok && s.selected != nil &&
				s.game.CanRepairAt(s.selected, cellX, cellY) {
				s.repairAt(cellX, cellY)
				s.orderInput.repairArmed = false

				return true
			}

			s.cancelRepair()

			return true
		}
	}

	return false
}

// clickCombat answers a left click on the map with the selected unit's weapon, in the
// order the original checks: a force attack with SHIFT/CTRL held, then a normal shot at
// an attackable enemy in range, then a move to attack on one out of range. A group fires
// or closes as a whole. Returns whether the click became an attack; anything else falls
// through to select or move. C++: GameManager left click ORDER_FIRE / ORDER_MOVE_TO_ATTACK.
func (s *Screen) clickCombat(cellX, cellY int) bool {
	if sel := s.selected; sel != nil && sel.Team == s.game.State.ActiveTeam &&
		s.forceModifierHeld() && s.game.CanForceAttack(sel, cellX, cellY) {
		s.attack(cellX, cellY, true)

		return true
	}

	// A left click on an attackable enemy in range fires (does not select it); out of
	// range falls through to select/move. canSeeTargetAt blocks a blind fire into fog.
	// C++: GameManager left click ORDER_FIRE on the enemy target cursor.
	if sel := s.selected; sel != nil && sel.Team == s.game.State.ActiveTeam &&
		s.game.CanAttack(sel, cellX, cellY) && s.canSeeTargetAt(sel, cellX, cellY) {
		if len(s.selectionSet()) > 1 {
			s.orderGroupAttack(cellX, cellY) // the whole group fires (or closes on) the target
		} else {
			s.attack(cellX, cellY, false)
		}

		return true
	}

	// Enemy out of range with a mobile armed unit selected: move to attack, driving
	// to firing range and pursuing. C++: CURSOR_FAR_TARGET -> ORDER_MOVE_TO_ATTACK.
	if sel := s.selected; sel != nil && sel.Team == s.game.State.ActiveTeam && s.game.CanFarTarget(sel, cellX, cellY) {
		if len(s.selectionSet()) > 1 {
			s.orderGroupAttack(cellX, cellY) // the whole group pursues (in range members fire)
		} else {
			s.game.OrderMoveToAttack(sel, cellX, cellY)
			s.showMessage("Moving to attack.")
		}

		dbg.Printf("move-to-attack %s (%d,%d) -> enemy (%d,%d)", displayName(sel.Type), sel.GridX, sel.GridY, cellX, cellY)

		return true
	}

	return false
}

// hitMessage is the status line for a shot that hit target for damage without destroying it.
func (s *Screen) hitMessage(target *game.UnitInfo, damage int) string {
	if target == nil {
		return fmt.Sprintf("Enemy unit hit for %d.", damage)
	}

	maxHits := 0

	if target.Values != nil {
		maxHits = target.Values.Get(game.AttribHits)
	}

	return fmt.Sprintf("%s unit hit for %d (%d/%d, %s).", s.teamName(target.Team), damage, target.Hits, maxHits, hitBand(target))
}
