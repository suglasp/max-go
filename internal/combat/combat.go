// combat.go : validates targets and resolves unit attacks.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
//
// C++ ported : access.cpp - attack-target half; units_manager.cpp / unitinfo.cpp - damage and attack resolution
//
// Package combat implements target validation, range checks, damage, and shot
// application from access.cpp, units_manager.cpp, and unitinfo.cpp.
//
// Like internal/vis it operates on the live unit set (game.UnitInfo),
// so it imports game/assets/world and sits above access in the import DAG.
// The surface dependent rules (naval line of sight, valid target surface) take an
// optional SurfaceSource rather than importing access,
// keeping combat usable headlessly with synthetic maps.
package combat

import (
	"github.com/suglasp/max-go/internal/assets"
	"github.com/suglasp/max-go/internal/game"
	"github.com/suglasp/max-go/internal/world"
)

// tileSize is GFX_MAP_TILE_SIZE: ranges are given in tiles but compared in
// pixels (64 px per tile), making the attack range a disc.
const tileSize = 64

// SurfaceSource yields the surface mask of a cell; combat needs it for the
// naval rules (sub/corvette can only shoot across water, and a sub/corvette
// must itself stand on water/coast). Satisfied by *world.Map and the synthetic
// test surfaces; may be nil, in which case those rules are skipped.
type SurfaceSource interface {
	SurfaceAt(x, y int) uint8
}

// ValidTargetTypes is the bitmask of target categories an attacker type may
// fire on (mobile land / sea / air, stationary). It drives target validation.
//
// C++: Access_GetValidAttackTargetTypes (access.cpp).
func ValidTargetTypes(attacker assets.UnitType) game.Flags {
	switch attacker {
	case assets.Scout, assets.Commando, assets.Infantry, assets.Corvette,
		assets.Battlshp, assets.Submarne, assets.MsslBoat, assets.Bomber,
		assets.Juggrnt, assets.AlnTank, assets.AlnAsGun, assets.Tank,
		assets.Artillry, assets.RocktLch, assets.MisslLch, assets.GunTurrt,
		assets.ArtyTrrt, assets.AntiMssl:
		return game.FlagMobileSeaUnit | game.FlagMobileLandUnit | game.FlagStationary
	case assets.FastBoat, assets.Fighter, assets.SpFlak, assets.AntiAir:
		return game.FlagMobileAirUnit
	case assets.AlnPlane:
		return game.FlagMobileAirUnit | game.FlagMobileSeaUnit | game.FlagMobileLandUnit | game.FlagStationary
	case assets.LandMine, assets.SeaMine:
		return game.FlagMobileSeaUnit | game.FlagMobileLandUnit
	default:
		return 0
	}
}

// canTargetType ports Access_IsValidAttackTargetTypeEx: the per type special
// cases (bridge/platform/mine treated as land, submarine only attackable by a
// few types, tapes never) plus the target class bitmask test.
//
// C++: Access_IsValidAttackTargetTypeEx (access.cpp).
func canTargetType(attacker, target assets.UnitType, targetFlags game.Flags) bool {
	switch target {
	case assets.Bridge, assets.WtrPltfm, assets.SeaMine:
		targetFlags = game.FlagMobileLandUnit
	case assets.Submarne:
		switch attacker {
		case assets.Bomber, assets.Submarne, assets.Corvette, assets.AlnPlane, assets.SeaMine:
		default:
			return false
		}
	case assets.LrgTape, assets.SmlTape:
		return false
	}
	return targetFlags&ValidTargetTypes(attacker) != 0
}

// CanTarget reports whether attacker may fire on target at the target's cell.
// It ports Access_IsValidAttackTarget: a ground (or hovering) attacker cannot
// be the pure air firing path; a not hovering air target is treated as a land
// target; and a naval attacker (sub/corvette) may only shoot if the target
// cell is water or coast.
//
// C++: Access_IsValidAttackTarget / Access_IsValidAttackTargetEx (access.cpp).
func CanTarget(attacker, target *game.UnitInfo, surf SurfaceSource) bool {
	// A pure flying attacker (air, not hovering) keeps the target's real class;
	// a ground or hovering attacker treats a not hovering air target as a land
	// target (a low flying plane is hittable by ground guns, a true hover unit is
	// not). C++: Access_IsValidAttackTarget.
	//
	// The original sends Fighter and Bomber attacks through the flyover sortie in
	// airpath.cpp. This implementation resolves them as single target attacks in
	// range and does not model the flyover or area bombing path.
	airAttacker := attacker.Flags.Has(game.FlagMobileAirUnit) && !attacker.Flags.Has(game.FlagHovering)

	targetFlags := target.Flags
	if !airAttacker && targetFlags.Has(game.FlagMobileAirUnit) && !targetFlags.Has(game.FlagHovering) {
		targetFlags &^= game.FlagMobileAirUnit
		targetFlags |= game.FlagMobileLandUnit
	}

	// Naval attackers can only fire when the target stands on water/coast.
	if attacker.Type == assets.Submarne || attacker.Type == assets.Corvette {
		if surf != nil {
			s := surf.SurfaceAt(target.GridX, target.GridY)
			if s&(world.SurfaceWater|world.SurfaceCoast) == 0 {
				return false
			}
		}
	}

	return canTargetType(attacker.Type, target.Type, targetFlags)
}

// InRange reports whether (gx, gy) is within attacker's attack range. The test
// is the same circular pixel test as scan range: (range*64)^2 >= dx^2+dy^2.
// WithinAttackRange adds the naval line of fire on top.
//
// C++: Access_IsWithinAttackRange (access.cpp), circular distance part.
func InRange(attacker *game.UnitInfo, gx, gy, attackRange int) bool {
	area := (attackRange * tileSize) * (attackRange * tileSize)
	dx := (gx - attacker.GridX) * tileSize
	dy := (gy - attacker.GridY) * tileSize
	return dx*dx+dy*dy <= area
}

// WithinAttackRange is InRange plus the naval line of fire: a Submarine or Corvette fires only when every
// cell between it and the target is water or coast. A nil surface skips the line, as in CanTarget.
//
// C++ ported : access.cpp - Access_IsWithinAttackRange
func WithinAttackRange(attacker *game.UnitInfo, gx, gy, attackRange int, surf SurfaceSource) bool {
	if !InRange(attacker, gx, gy, attackRange) {
		return false
	}
	if (attacker.Type != assets.Submarne && attacker.Type != assets.Corvette) || surf == nil {
		return true
	}
	return navalLineClear(attacker, gx, gy, surf)
}

// navalLineClear walks from the attacker's pixel position toward (gx, gy) one cell per step (16.16 fixed
// point) and reports whether every cell strictly between the two is water or coast. The step count is the
// longer axis in cells (klei 23/08/26 fixed it to `/ 64`, it was `* 64`).
//
// C++ ported : access.cpp - Access_IsWithinAttackRange (naval part)
func navalLineClear(attacker *game.UnitInfo, gx, gy int, surf SurfaceSource) bool {
	dx := (gx - attacker.GridX) * tileSize
	dy := (gy - attacker.GridY) * tileSize
	steps := max(abs(dx), abs(dy)) / tileSize
	if steps <= 0 {
		return true
	}
	stepX := (dx << 16) / steps
	stepY := (dy << 16) / steps
	x := stepX + attacker.X<<16
	y := stepY + attacker.Y<<16
	for steps--; steps > 0; steps-- {
		s := surf.SurfaceAt((x>>16)/tileSize, (y>>16)/tileSize)
		if s != world.SurfaceWater && s != world.SurfaceCoast {
			return false
		}
		x += stepX
		y += stepY
	}
	return true
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}

// Damage computes the hit point loss attacker inflicts on target for one shot.
// attackPotential>1 scales the attack down by (5-potential)/4 (area/multi hit
// spread); a submarine struck by a bomber/alien plane takes half; armour is
// subtracted, with a floor of 1.
//
// C++: UnitsManager_GetAttackDamage (units_manager.cpp).
func Damage(attacker, target *game.UnitInfo, attackPotential int) int {
	if attacker.Values == nil || target.Values == nil {
		return 0
	}
	attack := attacker.Values.Get(game.AttribAttack)
	armor := target.Values.Get(game.AttribArmor)

	if attackPotential > 1 {
		attack = ((5 - attackPotential) * attack) / 4
	}
	if target.Type == assets.Submarne &&
		(attacker.Type == assets.Bomber || attacker.Type == assets.AlnPlane) {
		attack /= 2
	}
	if armor >= attack {
		return 1
	}
	return attack - armor
}

// Result reports the outcome of resolving one shot.
type Result struct {
	Damage    int  // hit points actually removed from the target
	Destroyed bool // target reached 0 hits
}

// Apply removes one shot's damage from target (clamped to its remaining hits)
// and reports whether it was destroyed. It mirrors the hit point arithmetic of
// UnitInfo::AttackUnit. The caller handles explosions, AI tracking, experience,
// and messages.
//
// C++: UnitInfo::AttackUnit hit point block (unitinfo.cpp).
func Apply(attacker, target *game.UnitInfo, attackPotential int) Result {
	if target.Hits <= 0 {
		return Result{}
	}
	dmg := Damage(attacker, target, attackPotential)
	if dmg > target.Hits {
		dmg = target.Hits
	}
	target.Hits -= dmg
	return Result{Damage: dmg, Destroyed: target.Hits == 0}
}

/* // note: ported but unused code in Go port
// Attack fires one shot from attacker at target: it validates the target, the
// attacker's per turn shots and range, then spends a shot/ammo and applies the
// damage. It reports the result and whether a shot was actually fired (ok ==
// false means an invalid/out of range target or no shots/ammo left).
//
// C++: the validate -> Fire -> AttackUnit path (UnitInfo::Attack / Fire +
// units_manager.cpp), reduced to the turn based single target case.
func Attack(attacker, target *game.UnitInfo, surf SurfaceSource) (Result, bool) {
	return AttackForce(attacker, target, surf, false)
}
*/

// AttackForce fires one shot like Attack, with the force flag threaded into the
// validity check so a FORCE ATTACK can hit a friendly target. force=false is
// identical to Attack.
func AttackForce(attacker, target *game.UnitInfo, surf SurfaceSource, force bool) (Result, bool) {
	if !CanAttackForce(attacker, target, surf, force) {
		return Result{}, false
	}
	attacker.Ammo--
	attacker.Shots--
	attacker.SpendShotSpeed() // firing eats into the shared move/fire budget
	return Apply(attacker, target, 0), true
}

// CanAttack reports whether attacker could fire on target right now; a valid
// enemy target, in range, with shots and ammo left, without spending or
// applying anything. It is the nonmutating precondition of Attack, used to
// light up the attack cursor.
func CanAttack(attacker, target *game.UnitInfo, surf SurfaceSource) bool {
	return CanAttackForce(attacker, target, surf, false)
}

// CanAttackForce is CanAttack with a force flag: when force is set it skips the
// same team refusal, so a player FORCE ATTACK (SHIFT/CTRL) can fire on a friendly
// unit. With force=false it is identical to CanAttack. The weapon class, surface,
// range, shots and ammo checks still apply - you can only force fire on a target
// your weapon could ever hit, in range, with a shot to spare. C++: the
// CURSOR_ATTACK_SHIFT/CTRL force target path skips the enemy team gate.
func CanAttackForce(attacker, target *game.UnitInfo, surf SurfaceSource, force bool) bool {
	if attacker.Values == nil || attacker.Shots <= 0 || attacker.Ammo <= 0 {
		return false
	}
	// A building still hidden behind its parked builder cannot be shot (it is not on the map yet).
	if target.HiddenUntilExpand {
		return false
	}
	if !force && attacker.Team == target.Team {
		return false
	}
	return CanTarget(attacker, target, surf) &&
		WithinAttackRange(attacker, target.GridX, target.GridY, attacker.Values.Get(game.AttribRange), surf)
}
