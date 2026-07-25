// mines.go : deploys, detects, and removes mines.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
//
// C++ ported : unitinfo.cpp - OrderLayMine / OrderClear mine handling
package sim

import (
	"github.com/suglasp/max-go/internal/assets"
	"github.com/suglasp/max-go/internal/combat"
	"github.com/suglasp/max-go/internal/game"
)

// mineAttackDamage / mineHitPoints are the FALLBACK stats for a laid mine, used only
// when no unit factory is wired (a headless test). A real game mints the mine through
// Game.MakeUnit, so it carries its true LANDMINE / SEAMINE row from the ATTRIBS table
// (attack 28, hits 2) plus the team's upgrades, exactly like a factory built unit.
const (
	mineAttackDamage = 28
	mineHitPoints    = 2
)

// Mine laying / removal: a mine layer (MineLayr land / SeaMnLyr sea) carries a
// store of mines (UnitInfo.Storage, capped at ATTRIB_STORAGE) and lays one on its
// own cell or picks its own back up. A laid mine is stealth: it hides from the enemy
// until a mine DETECTOR walks up to it, and it is passable, so an enemy that cannot
// see it drives on and sets it off. Resolved instantly on the popup action because
// the simulation has no per-frame ORDER_STATE machine, matching Attack.
//
// A mine's blast has no splash: its ATTRIB_AREA is 0 and the original's explosion is
// cosmetic, the damage coming from the mine's own point blank shot at whoever stepped
// on it.
//
// C++ ported : unitinfo.cpp - UnitInfo::PlaceMine / UnitInfo::PickUpMine.

// mineDetectRadius is how far a detector uncovers mines: one cell, so the 3x3 it
// stands in the middle of. Not its scan range - a surveyor sees far but only sniffs
// out a mine it is standing next to.
//
// C++ ported : access.cpp - Access_UpdateMapStatus (Survey_SurveyArea(unit, 1))
const mineDetectRadius = 1

// IsMineLayer reports whether u is a mine layer (land or sea).
func IsMineLayer(u *game.UnitInfo) bool {
	return u != nil && (u.Type == assets.MineLayr || u.Type == assets.SeaMnLyr)
}

// IsMineDetector reports whether u uncovers enemy mines around itself. The original's
// four, and the manual's ("Invisible to enemy units except surveyors, mine layers, and
// infiltrators"): the SURVEYOR, both mine LAYERS, and the COMMANDO. A scanner does not
// find mines however good its radar is, and neither does infantry.
//
// C++ ported : access.cpp - Access_UpdateMapStatus (the SURVEYOR / MINELAYR / SEAMNLYR / COMMANDO gate)
func IsMineDetector(u *game.UnitInfo) bool {
	if u == nil || u.Order == game.OrderDisable {
		return false
	}

	switch u.Type {
	case assets.Surveyor, assets.MineLayr, assets.SeaMnLyr, assets.Commando:
		return true
	default:
		return false
	}
}

// IsMine reports whether u is a laid mine (land or sea).
func IsMine(u *game.UnitInfo) bool {
	return u != nil && (u.Type == assets.LandMine || u.Type == assets.SeaMine)
}

// sweepMineDetectors uncovers enemy mines next to a team's mine detectors. UnitInfo.SpotBy keeps a mine visible after discovery; RefreshVisibility
// runs this sweep as a detector moves.
//
// C++ ported : survey.cpp - Survey_SurveyArea (Access_GetEnemyMineOnSentry -> SpotByTeam)
func (g *Game) sweepMineDetectors() {
	var detectors, mines []*game.UnitInfo

	for _, u := range g.Units.Units() {
		switch {
		case IsMine(u):
			mines = append(mines, u)
		case IsMineDetector(u):
			detectors = append(detectors, u)
		}
	}

	if len(mines) == 0 || len(detectors) == 0 {
		return
	}

	for _, d := range detectors {
		for _, m := range mines {
			if m.Team == d.Team || m.SpottedByTeam(d.Team) {
				continue
			}

			if abs(m.GridX-d.GridX) <= mineDetectRadius && abs(m.GridY-d.GridY) <= mineDetectRadius {
				m.SpotBy(d.Team)
				g.minesFound = append(g.minesFound, MineFound{Mine: m, Finder: d, Team: d.Team})
			}
		}
	}
}

// MineFound is an enemy mine a team's detector has just uncovered. The in game screen
// drains these to tell the player: an uncovered mine is a single dark sprite that appears
// somewhere along a driving surveyor's flank, which is easy to miss entirely, and the whole
// point of finding one is to NOT drive into it.
type MineFound struct {
	Mine   *game.UnitInfo
	Finder *game.UnitInfo
	Team   int
}

// DrainMinesFound returns the mines uncovered since the last call and clears the queue.
func (g *Game) DrainMinesFound() []MineFound {
	if len(g.minesFound) == 0 {
		return nil
	}

	out := g.minesFound
	g.minesFound = nil

	return out
}

// mineTypeFor is the mine a layer lays: SEAMINE for a sea layer, else LANDMINE.
func mineTypeFor(layer *game.UnitInfo) assets.UnitType {
	if layer.Type == assets.SeaMnLyr {
		return assets.SeaMine
	}
	return assets.LandMine
}

// MineCapacity is the layer's maximum mine store (ATTRIB_STORAGE), 0 if unknown.
func MineCapacity(u *game.UnitInfo) int {
	if u == nil || u.Values == nil {
		return 0
	}
	return u.Values.Get(game.AttribStorage)
}

// mineOnCell returns the team's mine of mineType on (cx, cy), or nil.
func (g *Game) mineOnCell(mineType assets.UnitType, team, cx, cy int) *game.UnitInfo {
	for _, u := range g.Units.Cells.At(cx, cy) {
		if u.Type == mineType && u.Team == team {
			return u
		}
	}
	return nil
}

// CanLayMine reports whether the layer can drop a mine on its cell right now: it
// is a mine layer with a mine in store and no mine of its type already there.
func (g *Game) CanLayMine(layer *game.UnitInfo) bool {
	if !IsMineLayer(layer) || layer.Storage <= 0 {
		return false
	}
	return g.mineOnCell(mineTypeFor(layer), layer.Team, layer.GridX, layer.GridY) == nil
}

// CanRemoveMine reports whether the layer can pick its own mine back up from its
// cell: it has room in store and one of its mines is on the cell.
func (g *Game) CanRemoveMine(layer *game.UnitInfo) bool {
	if !IsMineLayer(layer) || layer.Storage >= MineCapacity(layer) {
		return false
	}
	return g.mineOnCell(mineTypeFor(layer), layer.Team, layer.GridX, layer.GridY) != nil
}

// LayMine drops a team owned mine on the layer's cell and spends one from store.
// C++: UnitInfo::PlaceMine.
func (g *Game) LayMine(layer *game.UnitInfo) bool {
	if !g.CanLayMine(layer) {
		return false
	}

	mine := g.newMine(mineTypeFor(layer), layer.Team)
	g.Units.Deploy(mine, layer.GridX, layer.GridY)
	layer.Storage--
	g.RefreshVisibility()

	return true
}

// newMine mints the mine a layer drops. The unit factory gives it the REAL row from the
// ATTRIBS table, so the blast hits for the mine's own ATTRIB_ATTACK (28 in retail data,
// against the victim's armor) and the team's upgrades ride along - the same stats path a
// factory built unit takes. A game with no factory wired, i.e. a headless test, falls back
// to the retail figures by hand.
func (g *Game) newMine(t assets.UnitType, team int) *game.UnitInfo {
	if g.MakeUnit != nil {
		if mine := g.MakeUnit(team, t, game.FlagStationary); mine != nil && mine.Values != nil && mine.Values.Get(game.AttribAttack) > 0 {
			if mine.Hits <= 0 {
				mine.Hits = mine.Values.Get(game.AttribHits)
			}

			return mine
		}
	}

	g.nextBuilt++
	mine := game.NewUnitInfo(t, team, 20000+g.nextBuilt, 0)
	mine.Flags = game.FlagStationary
	mine.Values = &game.UnitValues{}
	mine.Values.Set(game.AttribAttack, mineAttackDamage)
	mine.Values.Set(game.AttribHits, mineHitPoints)
	mine.Hits = mineHitPoints

	return mine
}

// detonateMines explodes any laid mine that an ENEMY land/sea mobile unit has
// driven onto: the mine attacks that unit (its mineAttackDamage, minus the unit's
// armor) and is consumed. Mines are stealth + passable (access lets the enemy enter,
// vis hides the mine), so this is the "stepped on a hidden mine" trigger. Air units
// flying over do not set off a ground/sea mine. Called from Tick after movement.
//
// C++ ported : access.cpp - Access_TriggerEnemyMine (the mine's AttackUnit on entry).
func (g *Game) detonateMines() {
	for _, m := range g.Units.Units() {
		if m.Type != assets.LandMine && m.Type != assets.SeaMine {
			continue
		}
		var victim *game.UnitInfo
		for _, u := range g.Units.Cells.At(m.GridX, m.GridY) {
			if u.Team == m.Team {
				continue
			}
			if u.Flags.Has(game.FlagMobileLandUnit) || u.Flags.Has(game.FlagMobileSeaUnit) {
				victim = u // a land/sea mobile enemy triggers the mine
				break
			}
		}
		if victim == nil {
			continue
		}
		res := combat.Apply(m, victim, 0) // mine attack - victim armor (floor 1)
		g.Units.Remove(m)                 // the mine is consumed by the blast
		// Record the blast so the in game screen can show the explosion + sound on
		// the victim's cell (the mine's cell); drained by DrainMineBlasts each frame.
		g.mineBlasts = append(g.mineBlasts, MineBlast{Victim: victim, Destroyed: res.Destroyed})
		if res.Destroyed {
			g.DestroyUnit(victim)
		}
		g.RefreshVisibility()
	}
}

// MineBlast is a mine detonation the in game screen draws as an explosion + plays
// the blast sound at the victim's cell. Destroyed reports whether the victim died
// (a death blast + rubble) versus survived (an impact bloom). Victim's GridX/GridY
// remain valid for anchoring even after it is removed.
type MineBlast struct {
	Victim    *game.UnitInfo
	Destroyed bool
}

// DrainMineBlasts returns the mine detonations since the last call and clears the
// queue; the in game screen drains them each frame to spawn the explosion + sound.
func (g *Game) DrainMineBlasts() []MineBlast {
	if len(g.mineBlasts) == 0 {
		return nil
	}
	out := g.mineBlasts
	g.mineBlasts = nil
	return out
}

// RemoveMine picks the layer's own mine back off its cell into store. A picked up
// mine is removed cleanly (no rubble, unlike a destroyed unit). C++: UnitInfo::PickUpMine.
func (g *Game) RemoveMine(layer *game.UnitInfo) bool {
	if !g.CanRemoveMine(layer) {
		return false
	}
	mine := g.mineOnCell(mineTypeFor(layer), layer.Team, layer.GridX, layer.GridY)
	if mine == nil {
		return false
	}
	g.Units.Remove(mine)
	layer.Storage++
	g.RefreshVisibility()
	return true
}
