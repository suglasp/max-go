// projectile.go : resolves missile and torpedo travel, damage, and explosion events.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
//
// C++ ported : unitinfo.cpp - Attack / projectile travel and resolveProjectileHit
package sim

import (
	"math"

	"github.com/suglasp/max-go/internal/assets"
	"github.com/suglasp/max-go/internal/combat"
	"github.com/suglasp/max-go/internal/game"
)

// Missile and torpedo attacks create lightweight simulation records that travel over several ticks. Damage resolves on arrival; the gamescreen
// draws the projectile and trail without adding it to the unit registry, access grid, visibility, or AI state.
//
// C++ ported : unitinfo.cpp - UnitInfo::Attack (FIRES_MISSILES spawn) + airpath.cpp
// Paths_DrawMissile / Paths_FinishMove.

// projectileSpeed is the missile flight speed in pixels per Tick (faster than a
// driving unit; the map tile is 64px).
const projectileSpeed = 24

// firesMissile reports whether a unit launches a travelling projectile on attack
// (vs an instant tracer hit). The set mirrors the original's FIRES_MISSILES / MaxR's
// Rocket/Torpedo muzzle types: the air units + rocket launchers + missile crawler +
// missile boat fire rockets; the submarine and corvette fire torpedoes. Alien ball and missile attacks resolve without projectile travel.
func firesMissile(t assets.UnitType) bool {
	switch t {
	case assets.Bomber, assets.Fighter, // air (Rocket)
		assets.RocktLch, assets.MisslLch, assets.MsslBoat, // rocket launchers
		assets.Submarne, assets.Corvette: // torpedoes
		return true
	}
	return false
}

// projectileSprite returns the flying missile sprite + its trail particle for a
// firer: a Torpedo + bubble trail for sub/corvette, else a Rocket + smoke trail.
func projectileSprite(t assets.UnitType) (missile, trail assets.UnitType) {
	if t == assets.Submarne || t == assets.Corvette {
		return assets.Torpedo, assets.TrpBuble
	}
	return assets.Rocket, assets.RktSmoke
}

// Projectile is a missile in flight from Attacker toward Target, resolved (damage +
// explosion) on arrival. It is a lightweight sim record, not a deployed MISSILE_UNIT.
type Projectile struct {
	Attacker         *game.UnitInfo
	Target           *game.UnitInfo
	X, Y             int             // current pixel position (tile*64 space, like UnitInfo.X/Y)
	TargetX, TargetY int             // target pixel centre (the heading; also where the trail points back from)
	Missile          assets.UnitType // the flying sprite (Rocket / Torpedo)
	Trail            assets.UnitType // the trail particle (RktSmoke / TrpBuble)
	Angle            uint8           // 16-step firing heading (0..15), for the directional missile frame
}

// ProjectileImpact is a missile arrival the in game screen draws as an explosion +
// blast/hit sound at the target's cell. C++: Paths_FinishMove -> parent->Attack.
type ProjectileImpact struct {
	Attacker  *game.UnitInfo // the firer, so the screen can report a hit the player made
	Target    *game.UnitInfo
	Destroyed bool
	Damage    int // hit points the missile took off the target
}

// launchProjectile fires a travelling missile from attacker at target (already
// validated by CanAttack): the shot is spent now and the damage resolves when the
// missile arrives (advanceProjectiles). C++: UnitInfo::Attack spawning a MISSILE_UNIT.
func (g *Game) launchProjectile(attacker, target *game.UnitInfo) {
	attacker.Ammo--
	attacker.Shots--
	attacker.SpendShotSpeed() // firing eats into the shared move/fire budget
	missile, trail := projectileSprite(attacker.Type)
	g.projectiles = append(g.projectiles, &Projectile{
		Attacker: attacker,
		Target:   target,
		X:        attacker.X,
		Y:        attacker.Y,
		TargetX:  target.X,
		TargetY:  target.Y,
		Missile:  missile,
		Trail:    trail,
		// 16-step firing heading so the directional missile sprite (16 frames) noses
		// the right way; from the pixel delta (finer than the cell delta).
		Angle: uint8(game.FiringAngle(target.X-attacker.X, target.Y-attacker.Y)),
	})
}

// advanceProjectiles flies every in flight missile one step toward its target; on
// arrival it resolves the hit (combat.Apply, DestroyUnit if killed) and queues a
// ProjectileImpact for the in game screen. Called from Tick.
func (g *Game) advanceProjectiles() {
	if len(g.projectiles) == 0 {
		return
	}
	kept := g.projectiles[:0]
	for _, p := range g.projectiles {
		if !p.advance(projectileSpeed) {
			kept = append(kept, p)
			continue
		}
		g.resolveProjectileHit(p)
	}
	g.projectiles = kept
}

// resolveProjectileHit applies a just arrived missile's damage. A weapon with a
// blast radius (ATTRIB_ATTACK_RADIUS - in retail data ONLY the Rocket Launcher,
// radius 2) splashes every valid class unit around the impact, friend or foe;
// any other missile hits only its primary target. Each affected unit surfaces as
// a ProjectileImpact the in game screen turns into a blast / hit bloom.
//
// C++ ported : unitinfo.cpp - UnitInfo::Attack (the ATTACK_RADIUS / single target branch).
func (g *Game) resolveProjectileHit(p *Projectile) {
	radius := 0
	if p.Attacker != nil && p.Attacker.Values != nil {
		radius = p.Attacker.Values.Get(game.AttribAttackRadius)
	}
	if radius > 0 && p.Target != nil {
		// Splash centred on the target's cell (where the missile lands).
		g.attackArea(p.Attacker, p.Target.GridX, p.Target.GridY, radius)
		return
	}
	// Single target (it may have died to an earlier missile this frame, in which
	// case the blast is cosmetic only).
	t := p.Target
	impact := ProjectileImpact{Attacker: p.Attacker, Target: t}
	if t != nil && t.Hits > 0 {
		res := combat.Apply(p.Attacker, t, 0)
		impact.Destroyed = res.Destroyed
		impact.Damage = res.Damage
		if res.Destroyed {
			g.DestroyUnit(t)
		}
		g.RefreshVisibility()
	}
	g.projectileImpacts = append(g.projectileImpacts, impact)
}

// attackArea applies a blast radius weapon's damage to every targetable class unit
// within radius cells of (cx,cy) - friend OR foe (splash is indiscriminate
// collateral) - each surfacing as a ProjectileImpact for the renderer. The zone is
// the circular ZoneWalker region (dx*dx+dy*dy <= radius*radius) and the damage falls
// off with distance from the centre via the attackPotential spread (a unit at the
// impact takes full damage, the edge a fraction). Ground cover (slabs/tape/rubble)
// and the firer itself are skipped, and each multi cell unit is hit at most once.
//
// C++ ported : unitinfo.cpp - UnitInfo::AttackAreaTargets (ZoneWalker over ATTACK_RADIUS).
func (g *Game) attackArea(attacker *game.UnitInfo, cx, cy, radius int) {
	type victim struct {
		u    *game.UnitInfo
		dist int
	}
	var victims []victim
	seen := map[*game.UnitInfo]bool{}
	r2 := radius * radius
	for dy := -radius; dy <= radius; dy++ {
		for dx := -radius; dx <= radius; dx++ {
			if dx*dx+dy*dy > r2 {
				continue
			}
			for _, u := range g.Units.Cells.At(cx+dx, cy+dy) {
				if u == attacker || seen[u] || u.Hits <= 0 || u.Flags.Has(game.FlagGroundCover) {
					continue
				}
				if !combat.CanTarget(attacker, u, g.Grid.Surf) {
					continue
				}
				seen[u] = true
				victims = append(victims, victim{u, projApproxDistance(dx, dy)})
			}
		}
	}
	for _, v := range victims {
		impact := ProjectileImpact{Attacker: attacker, Target: v.u}
		res := combat.Apply(attacker, v.u, v.dist)
		impact.Destroyed = res.Destroyed
		impact.Damage = res.Damage
		if res.Destroyed {
			g.DestroyUnit(v.u)
		}
		g.projectileImpacts = append(g.projectileImpacts, impact)
	}
	g.RefreshVisibility()
}

// projApproxDistance is the octile metric (2*long + short) the area damage spread
// uses as the attackPotential. C++ ported : access.cpp - Access_GetApproximateDistance.
func projApproxDistance(dx, dy int) int {
	if dx < 0 {
		dx = -dx
	}
	if dy < 0 {
		dy = -dy
	}
	if dx >= dy {
		return 2*dx + dy
	}
	return 2*dy + dx
}

// advance moves the projectile up to speed pixels toward its target, reporting
// arrival (within one step).
func (p *Projectile) advance(speed int) bool {
	dx, dy := p.TargetX-p.X, p.TargetY-p.Y
	dist := int(math.Hypot(float64(dx), float64(dy)))
	if dist <= speed || dist == 0 {
		p.X, p.Y = p.TargetX, p.TargetY
		return true
	}
	p.X += dx * speed / dist
	p.Y += dy * speed / dist
	return false
}

// ActiveProjectiles returns the missiles in flight, for the in game renderer.
func (g *Game) ActiveProjectiles() []*Projectile {
	return g.projectiles
}

// DrainProjectileImpacts returns the missile arrivals since the last call and clears
// the queue; the in game screen drains them each frame for the explosion + sound.
func (g *Game) DrainProjectileImpacts() []ProjectileImpact {
	if len(g.projectileImpacts) == 0 {
		return nil
	}
	out := g.projectileImpacts
	g.projectileImpacts = nil
	return out
}
