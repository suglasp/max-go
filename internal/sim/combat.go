// combat.go : resolves attacks and answers what a unit may target.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
//
// C++ ported : game_manager.cpp - attack resolution and target selection
package sim

import (
	"github.com/suglasp/max-go/internal/access"
	"github.com/suglasp/max-go/internal/assets"
	"github.com/suglasp/max-go/internal/combat"
	"github.com/suglasp/max-go/internal/game"
)

// Attack fires one shot from attacker at the unit occupying (cx, cy), resolving
// target/range/ammo/damage through the combat layer. A destroyed target is removed and
// visibility refreshed. Reports the result and whether a shot was fired.
// C++: ORDER_FIRE then UnitInfo::Attack, reduced to the single target case.
func (g *Game) Attack(attacker *game.UnitInfo, cx, cy int) (combat.Result, bool) {
	return g.attackAt(attacker, cx, cy, false)
}

// ForceAttack is Attack with the FORCE ATTACK flag: it can fire on a friendly
// unit (the player's SHIFT/CTRL force fire), bypassing the same team refusal. The
// weapon class/surface/range/ammo gates still apply. C++: the CURSOR_ATTACK_SHIFT
// force target path through UnitsManager_Popup_OnClick_Attack.
func (g *Game) ForceAttack(attacker *game.UnitInfo, cx, cy int) (combat.Result, bool) {
	return g.attackAt(attacker, cx, cy, true)
}

// attackAt resolves and fires the attack at (cx, cy). force allows a friendly
// target (force fire); otherwise only an enemy at the cell is engaged.
func (g *Game) attackAt(attacker *game.UnitInfo, cx, cy int, force bool) (combat.Result, bool) {
	if g.IsDisabled(attacker) {
		return combat.Result{}, false // a disabled unit cannot fire
	}
	target := g.attackTargetAt(attacker, cx, cy, force)
	if target == nil {
		return combat.Result{}, false
	}
	// A missile firer launches a travelling ROCKET/TORPEDO: the shot is spent now
	// but the damage resolves when it arrives (advanceProjectiles), so this returns
	// "fired, zero immediate damage" and the explosion is deferred to the impact.
	if firesMissile(attacker.Type) {
		if !combat.CanAttackForce(attacker, target, g.Grid.Surf, force) {
			return combat.Result{}, false
		}
		attacker.AimTurretAt(game.TargetAngle(cx-attacker.GridX, cy-attacker.GridY))
		g.launchProjectile(attacker, target)
		return combat.Result{}, true
	}
	res, ok := combat.AttackForce(attacker, target, g.Grid.Surf, force)
	if ok {
		attacker.AimTurretAt(game.TargetAngle(cx-attacker.GridX, cy-attacker.GridY))
	}
	if ok && res.Destroyed {
		g.cancel(target)
		g.DestroyUnit(target) // removes the unit and (for a building) its foundation slab
		// Queue the kill so the in game screen blasts it - this fires for an AI / sim side
		// instant attack too (the AI calls Attack directly), not just the player's click.
		g.casualties = append(g.casualties, Casualty{Victim: target})
		g.RefreshVisibility()
	} else if ok && res.Damage > 0 {
		// A surviving hit is queued for the impact bloom, hit sound, and "under attack!" banner.
		g.damage = append(g.damage, Damage{Victim: target, Attacker: attacker, Amount: res.Damage})
	}
	return res, ok
}

// Damage is a non-fatal instant attack hit. The screen drains it each frame for impact, sound, banner, and status voice; missile and mine hits use
// their own events, while kills queue a Casualty.
// C++ ported : game_manager.cpp - GameManager_NotifyEvent case 1 (the unit->hits
// branch: the fd20 "%s under attack!" banner plus the V_M229 status voice).
type Damage struct {
	Victim   *game.UnitInfo // unit that was hit and survived
	Attacker *game.UnitInfo // unit that fired the shot
	Amount   int            // hit points removed
}

// DrainDamage returns the non fatal instant attack hits since the last call and
// clears the queue.
func (g *Game) DrainDamage() []Damage {
	if len(g.damage) == 0 {
		return nil
	}
	out := g.damage
	g.damage = nil
	return out
}

// Casualty is a unit killed by an instant (non missile) attack. The screen drains
// these each frame (DrainCasualties) for the death blast + sound + voice, so an AI
// kill on the player animates like the player's own. (Mine/missile kills carry their
// own events.) C++: units_manager.cpp spawns the explosion sprite on any death.
type Casualty struct {
	Victim *game.UnitInfo // unit killed by the attack
}

// DrainCasualties returns the instant attack kills since the last call and clears the
// queue; the in game screen drains them each frame for the death blast + sound.
func (g *Game) DrainCasualties() []Casualty {
	if len(g.casualties) == 0 {
		return nil
	}
	out := g.casualties
	g.casualties = nil
	return out
}

// CanAttack reports whether attacker could fire at an enemy in cell (cx, cy)
// right now, without firing; the in game screen uses it to light up the attack
// cursor over a valid target.
func (g *Game) CanAttack(attacker *game.UnitInfo, cx, cy int) bool {
	target := g.topTarget(attacker, cx, cy)
	return target != nil && combat.CanAttack(attacker, target, g.Grid.Surf)
}

// CanForceAttack reports whether attacker could FORCE FIRE on a unit in (cx, cy)
// right now - the same as CanAttack but allowing a friendly target (the player's
// SHIFT/CTRL force attack). The in game screen uses it to light the attack cursor
// over a friendly while the force modifier is held.
func (g *Game) CanForceAttack(attacker *game.UnitInfo, cx, cy int) bool {
	target := g.topAnyTarget(attacker, cx, cy)
	return target != nil && combat.CanAttackForce(attacker, target, g.Grid.Surf, true)
}

// CanFarTarget reports whether (cx, cy) holds a valid enemy that attacker could fire on
// but is currently OUT of range, with attacker mobile enough to close. The screen lights
// the PTR_FTRG cursor as a hint to drive within range (vanilla never fires empty ground).
// A stationary turret, an unarmed/no ammo unit, or an in range target is not a far target.
// C++ ported : game_manager.cpp - GameManager_GetMilitaryCursor (CURSOR_FAR_TARGET).
func (g *Game) CanFarTarget(attacker *game.UnitInfo, cx, cy int) bool {
	if attacker == nil || attacker.Values == nil {
		return false
	}
	// Only a mobile attacker gets the far target cursor; a stationary weapon that
	// cannot close on the target reads as a plain no go.
	if attacker.Flags.Has(game.FlagStationary) {
		return false
	}
	if !g.UnitArmed(attacker) || attacker.Ammo <= 0 {
		return false
	}
	target := g.topTarget(attacker, cx, cy)
	if target == nil || !combat.CanTarget(attacker, target, g.Grid.Surf) {
		return false
	}
	// A far target is a valid enemy out of current weapon range; in range it is the
	// normal CURSOR_ENEMY instead.
	return !combat.InRange(attacker, target.GridX, target.GridY, attacker.Values.Get(game.AttribRange))
}

// CanTargetUnit reports whether attacker's weapon may fire on target by type and
// surface, ignoring range and ammo, i.e. "is this a unit attacker could ever
// shoot". The AI uses it to pursue only enemies it can actually engage (an
// anti air unit chases aircraft, not tanks). C++: Access_IsValidAttackTarget.
func (g *Game) CanTargetUnit(attacker, target *game.UnitInfo) bool {
	if attacker == nil || target == nil {
		return false
	}
	return combat.CanTarget(attacker, target, g.Grid.Surf)
}

// UnitArmed reports whether a unit has a weapon (can fire on some target class
// at all). C++: a unit with a non empty Access_GetValidAttackTargetTypes set.
func (g *Game) UnitArmed(u *game.UnitInfo) bool {
	return u != nil && combat.ValidTargetTypes(u.Type) != 0
}

// CanEnter reports whether u could move into cell (cx, cy): terrain passable for its
// movement type and not blocked by a same class unit. Ignores path length and budget
// (callers handle those); answers "is this cell go able", which the move cursor needs.
// C++: Access_IsAccessible (access.cpp).
func (g *Game) CanEnter(u *game.UnitInfo, cx, cy int) bool {
	if cx < 0 || cy < 0 || cx >= g.W || cy >= g.H {
		return false
	}
	d := access.UnitDesc{Type: u.Type, Flags: u.Flags, LandType: access.LandTypeForUnit(u.Type, u.Flags)}
	return g.Grid.IsAccessible(d, u.Team, cx, cy, access.SameClassBlocks).Passable()
}

// topTarget returns the enemy unit at (cx, cy) the attacker may fire on, or nil. A
// CONNECTOR (Cnct4W) can share a cell with a real structure, so prefer a real unit or
// building and fall back to the connector only when it is alone (else the connector
// would hide the building behind it, leaving it un attackable).
func (g *Game) topTarget(attacker *game.UnitInfo, cx, cy int) *game.UnitInfo {
	var connector *game.UnitInfo
	for _, u := range g.Units.Cells.At(cx, cy) {
		if u.Team == attacker.Team || !targetableUnit(u) {
			continue
		}
		if u.Type == assets.Cnct4W {
			if connector == nil {
				connector = u
			}
			continue
		}
		return u
	}
	return connector
}

// targetableUnit reports whether u can be a weapon target at all: NOT ground cover
// (slab/road) nor a BRIDGE (a FlagStationary prop you drive across). An alien bridge
// would otherwise read as an enemy target (red cursor, damageless shots).
// C++: a terrain prop is never an Access attack target.
func targetableUnit(u *game.UnitInfo) bool {
	// Waldo / Harvster are inert gag props (solid 1x1 obstacles), never weapon targets.
	if u.Type == assets.Waldo || u.Type == assets.Harvster {
		return false
	}
	return !u.Flags.Has(game.FlagGroundCover) && u.Type != assets.Bridge
}

// topAnyTarget returns the unit at (cx, cy) the attacker may FORCE fire on - like
// topTarget but INCLUDING a same team unit (a friendly), excluding the attacker
// itself and inert ground cover. Same connector rule: prefer a real structure or
// unit sharing the cell, falling back to a connector only when it is alone.
func (g *Game) topAnyTarget(attacker *game.UnitInfo, cx, cy int) *game.UnitInfo {
	var connector *game.UnitInfo
	for _, u := range g.Units.Cells.At(cx, cy) {
		if u == attacker || !targetableUnit(u) {
			continue
		}
		if u.Type == assets.Cnct4W {
			if connector == nil {
				connector = u
			}
			continue
		}
		return u
	}
	return connector
}

// attackTargetAt resolves the unit the attacker fires on at (cx, cy): any unit
// (including a friendly) when force is set, otherwise only an enemy.
func (g *Game) attackTargetAt(attacker *game.UnitInfo, cx, cy int, force bool) *game.UnitInfo {
	if force {
		return g.topAnyTarget(attacker, cx, cy)
	}
	return g.topTarget(attacker, cx, cy)
}
