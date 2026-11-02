// effects.go : implements the combat visual effects.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
// C++ ported : game_manager.cpp - explosion effect spawn and animation tick
package gamescreen

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/suglasp/max-go/internal/assets"
	"github.com/suglasp/max-go/internal/dbg"
	"github.com/suglasp/max-go/internal/game"
	"github.com/suglasp/max-go/internal/gfx"
	"github.com/suglasp/max-go/internal/platform"
	"github.com/suglasp/max-go/internal/sprites"
)

// effectsState holds combat and build effects along with transport load and unload animations.
type effectsState struct {
	explosions  []explosion                 // destroyed unit's short expanding flash
	colRubble   uint8                       // rubble marker colour
	colHazard   uint8                       // "under construction" yellow/black hazard stripe colour
	bldMrk      []*sprites.MultiFrame       // BLDMRK1..5 exit markers around a just finished structure (build ready flow)
	bulbFrame   *sprites.MultiFrame         // BULB "thinking" sprite flashed over a unit while its move is deferred
	bulbUnit    *game.UnitInfo              // unit showing the bulb (nil = none)
	bulbDue     uint64                      // platform.Ticks() deadline when the deferred move starts
	bulbMoveX   int                         // deferred move target X
	bulbMoveY   int                         // deferred move target Y
	expanding   map[*game.UnitInfo]bool     // buildings playing their grow in (unfold) animation
	expandStamp uint64                      // paces the unfold steps
	airUnload   *airUnloadJob               // in progress AIR transport unload (nil when idle)
	airLoad     *airLoadJob                 // in progress AIR transport load (nil when idle)
	loadInto    *game.UnitInfo              // building popup Load armed; next click picks a unit to suck up
	loading     map[*game.UnitInfo]*loadJob // in progress suck ups
	loadStamp   uint64                      // suck up animation timestamp
}

// explosionMS is how long a primitive (with no sprite) destruction flash lives.
const explosionMS = 600

// Sprite explosion lifetimes spread each sheet evenly across its animation. Building blasts last longer than small-unit blasts.
const (
	bldExplosionMS   = 2600 // BLDEXPLD over ~2.6s
	smallExplosionMS = 1000 // LNDEXPLD over ~1s
	hitImpactMS      = 500  // HITEXPLD over ~0.5s: a quick weapon impact flash
)

// explosion is a transient destruction blast or weapon impact over a unit footprint. n is the footprint size in cells; sprite and dur select its animation.
type explosion struct {
	cellX, cellY int
	n            int
	sprite       assets.UnitType
	dur          uint64
	start        uint64
}

// spawnExplosion plays the destruction blast for a unit, using the building or small-unit sheet for its footprint. The sim records the rubble separately.
// Explosions remain active when Animate Effects is off.
//
// C++: UnitsManager spawning the explosion sprite on a kill (units_manager.cpp;
// LNDEXPLD/BLDEXPLD; the LRGRUBLE/SMLRUBLE debris is the sim's rubble record).
func (s *Screen) spawnExplosion(u *game.UnitInfo) {
	// DestroyUnit records rubble; this path only adds the blast.
	building := u.Flags.Has(game.FlagBuilding)
	n := 1
	sprite, dur := smallExplosionType, uint64(smallExplosionMS)

	if building {
		n = 2 // a 2x2 building's blast spans the footprint
		sprite, dur = bldExplosionType, bldExplosionMS
	}

	s.effects.explosions = append(s.effects.explosions, explosion{
		cellX:  u.GridX,
		cellY:  u.GridY,
		n:      n,
		sprite: sprite,
		dur:    dur,
		start:  platform.Ticks(),
	})

	s.playSfx(blastSfx) // the destruction blast
}

// spawnHitImpact records a HITEXPLD flash over a surviving target's footprint. It adds no rubble or destruction sound and remains active when Animate Effects is off.
//
// C++: the HITEXPLD sprite spawned at the impact point on a hit that is not fatal
// (units_manager.cpp weapon resolution).
func (s *Screen) spawnHitImpact(target *game.UnitInfo) {
	n := 1

	if target.Flags.Has(game.FlagBuilding) {
		n = 2
	}

	s.effects.explosions = append(s.effects.explosions, explosion{
		cellX:  target.GridX,
		cellY:  target.GridY,
		n:      n,
		sprite: hitImpactType,
		dur:    hitImpactMS,
		start:  platform.Ticks(),
	})
}

// forgetDestroyed clears selection and camera state for a unit removed through the sim's death queues.
func (s *Screen) forgetDestroyed(v *game.UnitInfo) {
	if v == nil {
		return
	}

	s.forgetSelectedUnit(v)

	if s.camera.lockUnit == v {
		s.camera.lockUnit = nil
	}

	// Keep portraitUnit until updatePortrait sees the selection change and tears down its FLC player. The stale pointer is compared but never dereferenced.
}

// tickMineBlasts presents sim mine detonations. Destroyed victims use the death blast and rubble; survivors receive a blast, sound, and damage notification.
//
// C++: the explosion sprite + SFX_TYPE_EXPLOAD spawned on a mine detonation
// (Access_TriggerEnemyMine -> the mine's AttackUnit).
func (s *Screen) tickMineBlasts() {
	for _, b := range s.game.DrainMineBlasts() {
		v := b.Victim

		if v == nil {
			continue
		}

		if b.Destroyed {
			// notifyDeath supplies the blast, sound, feedback, and sim-recorded rubble.
			s.notifyDeath(v)

			continue
		}

		// A surviving victim receives the LNDEXPLD blast rather than the smaller weapon-impact flash.
		s.effects.explosions = append(s.effects.explosions, explosion{
			cellX: v.GridX, cellY: v.GridY, n: 1,
			sprite: smallExplosionType, dur: uint64(smallExplosionMS), start: platform.Ticks(),
		})

		s.playSfx(blastSfx)
		s.notifyDamage(v) // it still took the damage: report it like any other hit
	}
}

// tickMinesFound reports enemy mines discovered by the player's detectors. It combines each sweep into one message.
func (s *Screen) tickMinesFound() {
	found := s.game.DrainMinesFound()
	if len(found) == 0 || s.game.State == nil {
		return
	}

	mine := 0

	for _, f := range found {
		if f.Team == s.game.State.PlayerTeam {
			mine++
		}
	}

	switch {
	case mine == 1:
		s.showMessage("Enemy mine detected!")
	case mine > 1:
		s.showMessage(strconv.Itoa(mine) + " enemy mines detected!")
	}
}

// projTrailSpacing is the pixel gap (in world space) between the trailing puffs.
const projTrailSpacing = 24

// drawProjectiles paints missiles over units with directional ROCKET or TORPEDO frames and trails along their flight heading.
//
// C++ ported : airpath.cpp - Paths_DrawMissile (the ROCKET/TORPEDO + RKTSMOKE/TRPBUBLE).
func (s *Screen) drawProjectiles(view *gfx.Buffer) {
	z := s.r.Zoom()

	for _, p := range s.game.ActiveProjectiles() {
		// Place trail puffs behind the missile along its heading.
		dx, dy := p.TargetX-p.X, p.TargetY-p.Y

		if d := int(math.Hypot(float64(dx), float64(dy))); d > 0 {
			for k := 1; k <= 3; k++ {
				bx := p.X - dx*projTrailSpacing*k/d
				by := p.Y - dy*projTrailSpacing*k/d
				s.sprites.DrawFrameAtPixel(view, p.Trail, 0, bx, by, z, s.camera.scrollX, s.camera.scrollY)
			}
		}

		// The missile sprite has 16 directional frames; index it by the firing heading.
		s.sprites.DrawFrameAtPixel(view, p.Missile, int(p.Angle), p.X, p.Y, z, s.camera.scrollX, s.camera.scrollY)
	}
}

// tickProjectiles presents sim missile impacts. Destroyed targets use death feedback; survivors receive a hit flash and sound. C++: Paths_FinishMove.
func (s *Screen) tickProjectiles() {
	for _, im := range s.game.DrainProjectileImpacts() {
		t := im.Target

		if t == nil {
			continue
		}

		if im.Destroyed {
			s.notifyDeath(t) // deselect + death blast + sound + the destroyed voice/message
			dbg.Printf("impact: missile destroys %s at (%d,%d)", displayName(t.Type), t.GridX, t.GridY)
		} else if t.Hits > 0 {
			s.playSfxForUnit(t, game.SfxHit)
			s.spawnHitImpact(t)
			s.notifyDamage(t)

			// The player's missile reports its hit on arrival, the same line a direct shot shows at once.
			if im.Attacker != nil && s.isHumanTeam(im.Attacker.Team) {
				s.showMessage(s.hitMessage(t, im.Damage))
			}

			maxHits := 0

			if t.Values != nil {
				maxHits = t.Values.Get(game.AttribHits)
			}

			dbg.Printf("impact: missile -> %s dmg %d (hits %d/%d, %s)", displayName(t.Type), im.Damage, t.Hits, maxHits, hitBand(t))
		}
	}
}

// tickCasualties presents sim kills from instant attacks through the common death path.
func (s *Screen) tickCasualties() {
	for _, c := range s.game.DrainCasualties() {
		if c.Victim == nil {
			continue
		}

		s.notifyDeath(c.Victim)
	}
}

// notifyDeath presents every unit death: it clears selection, plays the blast and sound, reports player losses, and reports visible enemy losses. The sim records rubble.
//
// C++ ported : units_manager.cpp - the explosion sprite spawned on any UnitInfo death,
// plus GameManager_NotifyEvent case 1 at hits 0 (the 73cc destroyed banner + V_M234).
func (s *Screen) notifyDeath(v *game.UnitInfo) {
	s.forgetDestroyed(v) // a killed unit must not stay selected/locked (panel ghost)
	s.playSfxForUnit(v, game.SfxExpload)
	s.spawnExplosion(v) // death blast (+ the generic boom)

	if v.Team == s.game.State.PlayerTeam {
		s.playVoice(vsUnitDestroyed)                    // the player just lost this unit
		s.showInfo(displayName(v.Type) + " destroyed!") // world tint event: a unit is lost (the original's type 1)
		s.latchEventCell(v.GridX, v.GridY)              // F1 jumps to where it fell

		return
	}

	if s.unitVisibleToPlayer(v) {
		// Visible enemy deaths report one-based coordinates. The language string uses C's %i, which Go formats as %d.
		//
		// C++ ported : unitinfo.cpp - the e6d7 path (a visible unit that is not the player's, at hits 0)
		format := strings.ReplaceAll(s.lang.Get("e6d7", "Enemy %s at [%d,%d] destroyed."), "%i", "%d")
		s.showMessage(fmt.Sprintf(format, displayName(v.Type), v.GridX+1, v.GridY+1))
	}
}

// tickDamage presents sim non-fatal instant hits with an impact flash, hit sound, and player warning.
//
// C++ ported : game_manager.cpp - GameManager_NotifyEvent case 1 (the surviving unit
// branch), raised on every strike, not just a fatal one.
func (s *Screen) tickDamage() {
	for _, d := range s.game.DrainDamage() {
		v := d.Victim

		if v == nil || v.Hits <= 0 {
			continue // a later shot in the same frame finished it off: the kill drain owns it
		}

		s.playSfxForUnit(v, game.SfxHit)
		s.spawnHitImpact(v) // HITEXPLD impact bloom on the struck (surviving) unit/building
		s.notifyDamage(v)   // banner + status/under attack voice when it is the player's unit
	}
}

// tickReactions reports opportunity fire. Damage uses the normal damage queues; this path handles the firing sound, banner, and announcer cue for player-involved reactions.
func (s *Screen) tickReactions() {
	for _, r := range s.game.DrainReactions() {
		if r.Reactor == nil || r.Mover == nil {
			continue
		}

		player := s.game.State.PlayerTeam
		involved := r.Reactor.Team == player || r.Mover.Team == player

		// Visible AI exchanges are audible, but unseen AI exchanges remain silent.
		if !involved && !s.unitVisibleToPlayer(r.Reactor) && !s.unitVisibleToPlayer(r.Mover) {
			continue
		}

		s.playSfxForUnit(r.Reactor, game.SfxFire)

		if !involved {
			continue // a visible AI vs AI exchange is heard, not narrated
		}

		s.showMessage(displayName(r.Reactor.Type) + " reacts to enemy " + displayName(r.Mover.Type))

		// The announcer cue is for a player reactor the player cannot see fire: not selected and off screen.
		// Its cooldown combines a multi-unit sweep.
		// C++ ported : units_manager.cpp - UnitsManager_CheckReaction (V_M250/V_F251)
		if s.isHumanTeam(r.Reactor.Team) && r.Reactor != s.selected && !s.unitInsideMapView(r.Reactor) {
			s.playVoice(vsReactionFire)
		}
	}
}

// blastSfx is the generic explosion sound (GEN_XPLD -> explmed.wav) played on
// any unit/building destruction.
const blastSfx = "GEN_XPLD"

// effect sprite resources: small unit vs building blasts, small vs large rubble.
const (
	smallExplosionType = assets.LndExpld
	bldExplosionType   = assets.BldExpld
	hitImpactType      = assets.HitExpld
	smallRubbleType    = assets.SmlRuble
	largeRubbleType    = assets.LrgRuble
)

// drawRubble paints sim-recorded rubble before units. Large rubble covers 2x2 building footprints; small rubble covers one cell. C++: drawmap.cpp ground layer.
func (s *Screen) drawRubble(view *gfx.Buffer) {
	z := s.r.Zoom()

	for _, rb := range s.game.RubbleCells() {
		n := 1
		rt := smallRubbleType

		if rb.Large {
			n = 2
			rt = largeRubbleType
		}

		if s.sprites.FrameCount(rt) > 0 {
			s.sprites.DrawEffect(view, rt, 0, rb.X, rb.Y, n, z, s.camera.scrollX, s.camera.scrollY)
			continue
		}

		px, py, ok := s.footprintCenter(rb.X, rb.Y, n, view)

		if !ok {
			continue
		}

		r := n * z / 4

		if r < 1 {
			r = 1
		}

		view.DrawBox(px-r, py-r, px+r, py+r, s.effects.colRubble)
	}
}

// drawEffects paints active explosions over units and removes expired ones. It uses explosion sprites when available and footprint-sized primitives otherwise.
func (s *Screen) drawEffects(view *gfx.Buffer) {
	z := s.r.Zoom()

	now := platform.Ticks()
	kept := s.effects.explosions[:0]

	for _, e := range s.effects.explosions {
		age := now - e.start

		if explFrames := s.sprites.FrameCount(e.sprite); explFrames > 0 {
			if age >= e.dur {
				continue // whole sheet played; drop the explosion
			}

			kept = append(kept, e)
			frame := int(age * uint64(explFrames) / e.dur) // spread the sheet across the duration

			if frame >= explFrames {
				frame = explFrames - 1
			}

			s.sprites.DrawEffect(view, e.sprite, frame, e.cellX, e.cellY, e.n, z, s.camera.scrollX, s.camera.scrollY)
			continue
		}

		// Primitive fallback (sprite missing): a flash that expands over a fixed duration.
		if age >= explosionMS {
			continue
		}

		kept = append(kept, e)
		px, py, ok := s.footprintCenter(e.cellX, e.cellY, e.n, view)

		if !ok {
			continue
		}

		maxR := e.n*z/2 + 2
		r := int(uint64(maxR)*age/explosionMS) + 1
		col := s.explosionColor(age)
		view.DrawCircle(px, py, r, col)

		if r > 2 {
			view.DrawCircle(px, py, r-2, col)
		}
	}

	s.effects.explosions = kept
}

// footprintCenter maps an anchored n-by-n footprint to its on-screen centre and reports whether that point is visible.
func (s *Screen) footprintCenter(cellX, cellY, n int, view *gfx.Buffer) (int, int, bool) {
	z := s.r.Zoom()

	px := cellX*z + n*z/2 - s.camera.scrollX
	py := cellY*z + n*z/2 - s.camera.scrollY

	if px < 0 || py < 0 || px >= view.W || py >= view.H {
		return 0, 0, false
	}

	return px, py, true
}

/* // note: unused code in Go port
// cellCenter maps a grid cell to its on screen pixel centre in the map view,
// reporting whether it lies within the view.
func (s *Screen) cellCenter(cellX, cellY int, view *gfx.Buffer) (int, int, bool) {
	z := s.r.Zoom()

	px := cellX*z + z/2 - s.camera.scrollX
	py := cellY*z + z/2 - s.camera.scrollY

	if px < 0 || py < 0 || px >= view.W || py >= view.H {
		return 0, 0, false
	}

	return px, py, true
}
*/

// explosionColor ramps a flash through white -> yellow -> orange -> red over
// its lifetime, the classic blast fade from hot to cool.
func (s *Screen) explosionColor(age uint64) uint8 {
	switch {
	case age < explosionMS/4:
		return 0xFF // white hot

	case age < explosionMS/2:
		return 0x04 // yellow

	case age < explosionMS*3/4:
		return s.colorOrange // orange

	default:
		return 0x01 // red
	}
}
