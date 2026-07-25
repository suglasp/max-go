// projectile_test.go : tests projectile.go.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package sim

import (
	"testing"

	"github.com/suglasp/max-go/internal/assets"
	"github.com/suglasp/max-go/internal/game"
)

// addBomber deploys an armed air unit (Bomber) at (gx, gy) for the given team.
func addBomber(g *Game, id, gx, gy, team int) *game.UnitInfo {
	u := game.NewUnitInfo(assets.Bomber, team, id, 0)
	u.Flags = game.FlagMobileAirUnit | game.FlagHovering | game.FlagSelectable
	u.Values = &game.UnitValues{}
	u.Values.Set(game.AttribAttack, 12)
	u.Values.Set(game.AttribRange, 6)
	u.Values.Set(game.AttribRounds, 1)
	u.Values.Set(game.AttribAmmo, 4)
	u.Values.Set(game.AttribHits, 16)
	u.InitState() // Shots = Rounds, Ammo, Hits from the values
	g.Units.Deploy(u, gx, gy)
	return u
}

// addEnemyTarget deploys a team 1 ground tank to be shot at.
func addEnemyTarget(g *Game, id, gx, gy int) *game.UnitInfo {
	u := game.NewUnitInfo(assets.Tank, 1, id, 0)
	u.Flags = game.FlagMobileLandUnit | game.FlagSelectable
	u.Values = &game.UnitValues{}
	u.Values.Set(game.AttribHits, 30)
	u.Values.Set(game.AttribArmor, 2)
	u.Hits = 30
	g.Units.Deploy(u, gx, gy)
	return u
}

// TestBomberLaunchesProjectile: a missile firer's Attack spawns a flying projectile,
// spends the shot, and deals NO immediate damage (it resolves on arrival).
func TestBomberLaunchesProjectile(t *testing.T) {
	g := newGame(t, 24, 8)
	bomber := addBomber(g, 1, 2, 4, 0)
	enemy := addEnemyTarget(g, 2, 8, 4)

	res, ok := g.Attack(bomber, 8, 4)
	if !ok {
		t.Fatal("bomber Attack returned not-ok (could not fire)")
	}
	if res.Damage != 0 || res.Destroyed {
		t.Errorf("a missile launch should deal NO immediate damage, got %+v", res)
	}
	if len(g.ActiveProjectiles()) != 1 {
		t.Fatalf("expected 1 projectile in flight, got %d", len(g.ActiveProjectiles()))
	}
	if bomber.Shots != 0 {
		t.Errorf("shot not spent at launch: Shots=%d, want 0", bomber.Shots)
	}
	if enemy.Hits != 30 {
		t.Errorf("target damaged before the missile arrived: Hits=%d, want 30", enemy.Hits)
	}
}

// TestProjectileResolvesOnArrival: ticking flies the missile to its target and the
// hit (damage + an impact event) resolves only on arrival.
func TestProjectileResolvesOnArrival(t *testing.T) {
	g := newGame(t, 24, 8)
	bomber := addBomber(g, 1, 2, 4, 0)
	enemy := addEnemyTarget(g, 2, 8, 4)
	if _, ok := g.Attack(bomber, 8, 4); !ok {
		t.Fatal("attack not ok")
	}
	for i := 0; i < 1000 && len(g.ActiveProjectiles()) > 0; i++ {
		g.Tick()
	}
	if len(g.ActiveProjectiles()) != 0 {
		t.Fatal("projectile never arrived")
	}
	if enemy.Hits >= 30 {
		t.Errorf("target took no damage on impact: Hits=%d", enemy.Hits)
	}
	if imps := g.DrainProjectileImpacts(); len(imps) != 1 || imps[0].Target != enemy {
		t.Errorf("expected 1 impact on the enemy, got %+v", imps)
	}
}

// TestResolveMovementFlushesProjectiles: the headless fast forward also flies
// missiles to impact (so AI / tests resolve them).
func TestResolveMovementFlushesProjectiles(t *testing.T) {
	g := newGame(t, 24, 8)
	bomber := addBomber(g, 1, 2, 4, 0)
	enemy := addEnemyTarget(g, 2, 8, 4)
	g.Attack(bomber, 8, 4)
	g.ResolveMovement()
	if len(g.ActiveProjectiles()) != 0 {
		t.Error("ResolveMovement left a projectile in flight")
	}
	if enemy.Hits >= 30 {
		t.Error("ResolveMovement did not resolve the missile hit")
	}
}

// TestFiresMissileAndKind: the full missile firing set (land launchers, sea
// sub/corvette/boat, air) is recognised and picks the right projectile + trail.
func TestFiresMissileAndKind(t *testing.T) {
	for _, ty := range []assets.UnitType{
		assets.Bomber, assets.Fighter, // air
		assets.RocktLch, assets.MisslLch, assets.MsslBoat, // rocket launchers
		assets.Submarne, assets.Corvette, // torpedoes
	} {
		if !firesMissile(ty) {
			t.Errorf("%v should fire a travelling missile", ty)
		}
	}
	if firesMissile(assets.Tank) {
		t.Error("a Tank should NOT fire a missile (instant gun)")
	}
	if m, tr := projectileSprite(assets.Submarne); m != assets.Torpedo || tr != assets.TrpBuble {
		t.Errorf("submarine projectile = (%v,%v), want Torpedo + TrpBuble", m, tr)
	}
	if m, tr := projectileSprite(assets.RocktLch); m != assets.Rocket || tr != assets.RktSmoke {
		t.Errorf("rocket-launcher projectile = (%v,%v), want Rocket + RktSmoke", m, tr)
	}
}

// TestLauncherLaunchesProjectile: a ground rocket launcher's attack also spawns a
// travelling missile (same path as the air units).
func TestLauncherLaunchesProjectile(t *testing.T) {
	g := newGame(t, 24, 8)
	launcher := game.NewUnitInfo(assets.RocktLch, 0, 1, 0)
	launcher.Flags = game.FlagMobileLandUnit | game.FlagSelectable | game.FlagTurretSprite
	launcher.Values = &game.UnitValues{}
	launcher.Values.Set(game.AttribAttack, 14)
	launcher.Values.Set(game.AttribRange, 9)
	launcher.Values.Set(game.AttribRounds, 1)
	launcher.Values.Set(game.AttribAmmo, 4)
	launcher.Values.Set(game.AttribHits, 16)
	launcher.InitState()
	g.Units.Deploy(launcher, 2, 4)
	enemy := addEnemyTarget(g, 2, 9, 4)

	if _, ok := g.Attack(launcher, 9, 4); !ok {
		t.Fatal("rocket launcher Attack not ok")
	}
	if len(g.ActiveProjectiles()) != 1 {
		t.Fatalf("expected 1 rocket in flight, got %d", len(g.ActiveProjectiles()))
	}
	g.ResolveMovement()
	if enemy.Hits >= 30 {
		t.Error("rocket did not damage the target on arrival")
	}
}

// addTankTarget deploys a ground tank for the given team (splash hits friend or foe).
func addTankTarget(g *Game, id, gx, gy, team int) *game.UnitInfo {
	u := game.NewUnitInfo(assets.Tank, team, id, 0)
	u.Flags = game.FlagMobileLandUnit | game.FlagSelectable
	u.Values = &game.UnitValues{}
	u.Values.Set(game.AttribHits, 30)
	u.Values.Set(game.AttribArmor, 2)
	u.Hits = 30
	g.Units.Deploy(u, gx, gy)
	return u
}

// TestRocketLauncherSplashDamage: the Rocket Launcher's blast radius
// (ATTRIB_ATTACK_RADIUS=2) splashes everything in range - the aimed target, an
// adjacent enemy AND an adjacent FRIENDLY unit (indiscriminate collateral) - while
// a unit outside the radius is untouched.
func TestRocketLauncherSplashDamage(t *testing.T) {
	g := newGame(t, 24, 12)
	launcher := game.NewUnitInfo(assets.RocktLch, 0, 1, 0)
	launcher.Flags = game.FlagMobileLandUnit | game.FlagSelectable | game.FlagTurretSprite
	launcher.Values = &game.UnitValues{}
	launcher.Values.Set(game.AttribAttack, 20)
	launcher.Values.Set(game.AttribRange, 12)
	launcher.Values.Set(game.AttribRounds, 1)
	launcher.Values.Set(game.AttribAmmo, 4)
	launcher.Values.Set(game.AttribHits, 16)
	launcher.Values.Set(game.AttribAttackRadius, 2) // the Rocket Launcher's retail blast radius
	launcher.InitState()
	g.Units.Deploy(launcher, 2, 4)

	primary := addTankTarget(g, 2, 10, 4, 1)      // aimed here (dist 0)
	enemyNear := addTankTarget(g, 3, 10, 5, 1)    // dx0 dy1 -> within radius 2
	friendlyNear := addTankTarget(g, 4, 11, 4, 0) // dx1 dy0 -> friendly collateral, in radius
	outside := addTankTarget(g, 5, 14, 4, 1)      // dx4 -> beyond radius 2, untouched

	if _, ok := g.Attack(launcher, 10, 4); !ok {
		t.Fatal("rocket launcher Attack not ok")
	}
	g.ResolveMovement() // fly to impact + splash

	if primary.Hits >= 30 {
		t.Errorf("primary target took no damage: Hits=%d", primary.Hits)
	}
	if enemyNear.Hits >= 30 {
		t.Errorf("adjacent enemy took no splash: Hits=%d", enemyNear.Hits)
	}
	if friendlyNear.Hits >= 30 {
		t.Errorf("adjacent FRIENDLY took no collateral splash: Hits=%d", friendlyNear.Hits)
	}
	if outside.Hits != 30 {
		t.Errorf("unit beyond the blast radius was hit: Hits=%d, want 30", outside.Hits)
	}
}

// TestCombatKillQueuesCasualty: an instant (gun) kill queues a Casualty the in game
// screen drains to blast the dead unit - so an AI scored kill animates too.
func TestCombatKillQueuesCasualty(t *testing.T) {
	g := newGame(t, 16, 8)
	gun := game.NewUnitInfo(assets.Tank, 0, 1, 0)
	gun.Flags = game.FlagMobileLandUnit | game.FlagSelectable | game.FlagTurretSprite
	gun.Values = &game.UnitValues{}
	gun.Values.Set(game.AttribAttack, 60) // kills the weak enemy in one shot
	gun.Values.Set(game.AttribRange, 5)
	gun.Values.Set(game.AttribRounds, 1)
	gun.Values.Set(game.AttribAmmo, 4)
	gun.Values.Set(game.AttribHits, 30)
	gun.InitState()
	g.Units.Deploy(gun, 2, 4)
	enemy := addEnemyTarget(g, 2, 4, 4) // 30 hits, armor 2

	res, ok := g.Attack(gun, 4, 4)
	if !ok || !res.Destroyed {
		t.Fatalf("expected a kill, got ok=%v res=%+v", ok, res)
	}
	cas := g.DrainCasualties()
	if len(cas) != 1 || cas[0].Victim != enemy {
		t.Fatalf("expected 1 casualty (the enemy), got %+v", cas)
	}
	if len(g.DrainCasualties()) != 0 {
		t.Error("casualties not cleared after drain")
	}
}

// TestMissileKillUsesImpactNotCasualty: a missile kill carries its OWN event
// (ProjectileImpact), so it must NOT also queue a Casualty - else it double blasts.
func TestMissileKillUsesImpactNotCasualty(t *testing.T) {
	g := newGame(t, 24, 8)
	bomber := addBomber(g, 1, 2, 4, 0)
	bomber.Values.Set(game.AttribAttack, 60) // ensure the rocket kills on arrival
	enemy := addEnemyTarget(g, 2, 8, 4)
	if _, ok := g.Attack(bomber, 8, 4); !ok {
		t.Fatal("attack not ok")
	}
	if len(g.DrainCasualties()) != 0 {
		t.Error("a missile launch must not queue a Casualty (the launch is not a kill)")
	}
	g.ResolveMovement() // fly to impact + resolve
	if enemy.Hits > 0 {
		t.Fatalf("missile did not kill: enemy Hits=%d", enemy.Hits)
	}
	if n := len(g.DrainCasualties()); n != 0 {
		t.Errorf("missile kill queued %d casualties; it should use ProjectileImpact only", n)
	}
}

// TestNonMissileAttackIsInstant: a gun unit (Tank) still resolves instantly, no projectile.
func TestNonMissileAttackIsInstant(t *testing.T) {
	g := newGame(t, 16, 8)
	gun := game.NewUnitInfo(assets.Tank, 0, 1, 0)
	gun.Flags = game.FlagMobileLandUnit | game.FlagSelectable | game.FlagTurretSprite
	gun.Values = &game.UnitValues{}
	gun.Values.Set(game.AttribAttack, 12)
	gun.Values.Set(game.AttribRange, 5)
	gun.Values.Set(game.AttribRounds, 1)
	gun.Values.Set(game.AttribAmmo, 4)
	gun.Values.Set(game.AttribHits, 30)
	gun.InitState()
	g.Units.Deploy(gun, 2, 4)
	enemy := addEnemyTarget(g, 2, 4, 4)

	res, ok := g.Attack(gun, 4, 4)
	if !ok {
		t.Fatal("tank attack not ok")
	}
	if len(g.ActiveProjectiles()) != 0 {
		t.Error("a Tank must NOT launch a projectile (instant fire)")
	}
	if res.Damage == 0 && !res.Destroyed {
		t.Errorf("instant attack dealt no damage: enemy Hits=%d", enemy.Hits)
	}
}
