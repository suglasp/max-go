// cursor.go : implements the game screen cursor logic.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
// C++ ported : mouse.c - cursor state resolution; interface.c - click dispatch
package gamescreen

import (
	"sort"

	"github.com/suglasp/max-go/internal/assets"
	"github.com/suglasp/max-go/internal/cursors"
	"github.com/suglasp/max-go/internal/game"
	"github.com/suglasp/max-go/internal/platform"
)

// newCursorManager loads the game's cursor catalog and returns a Manager
// (Hand selected). A partial or empty catalog is tolerated: cursors that
// did not load are skipped at draw time, leaving the gnw default arrow.
func newCursorManager(lib cursors.Reader) *cursors.Manager {
	cat, _ := cursors.LoadCatalog(lib) // partial catalog is fine; nil safe below

	return cursors.NewManager(cat)
}

// updateCursor selects the context cursor for the current pointer
// position, advances any animation, and installs it as the gnw software
// cursor. Called once per idle frame, before the screen is composited.
//
// C++: Cursor_SetCursor driven by the window / unit under the pointer
// (game_manager.cpp + cursor.cpp).
func (s *Screen) updateCursor() {
	if s.cursorMgr == nil {
		return
	}

	now := platform.Ticks()

	if s.cursorStamp != 0 {
		s.cursorMgr.Tick(int(now - s.cursorStamp))
	}

	s.cursorStamp = now

	s.installCursor(s.cursorContext())
}

// installCursor selects id and installs its current frame as the gnw
// software cursor (does nothing when the catalog is unavailable).
func (s *Screen) installCursor(id cursors.ID) {
	if s.cursorMgr == nil {
		return
	}

	s.cursorMgr.Set(id)

	if buf := s.cursorMgr.Frame(); buf != nil {
		hx, hy := s.cursorMgr.Hotspot()
		s.sys.SetCursor(buf, hx, hy, s.cursorMgr.Transparent())
	}
}

// cursorContext maps what is under the pointer to a cursor ID. It ports the
// part of GameManager_GetCursor (game_manager.cpp) the port needs so far:
// scroll arrows at the screen edges, then, for a selected active team unit,
// the enemy cursor over an attackable target, the friend cursor over a
// friendly unit, and the go / no go cursors over reachable or blocked terrain.
// The hand is the fallback. An armed X-Fer pick shows the transfer pointer
// over a valid target. Load, reload and way cursors are still to do.
func (s *Screen) cursorContext() cursors.ID {
	m := s.plat.Mouse()

	// Scroll arrows where edge scroll fires (screen edges), as in vanilla. When the
	// view is already clamped at the map boundary in that direction the arrow becomes
	// a NO cursor, signalling the view can't go further.
	if dx, dy := edgeScroll(m.X, m.Y, 1); dx != 0 || dy != 0 {
		return s.scrollEdgeCursor(dx, dy)
	}

	// Landing site selection: the green "land here" pointer over the map while
	// picking; the hand once the site is confirmed and the descent plays. C++:
	// the UNIT_GO / CURSOR_HAND of GameManager_SelectSite.
	if s.siteSelect != nil {
		if s.siteSelect.introStart != 0 {
			return cursors.Hand
		}

		if _, _, overMap := s.mapCell(m.X, m.Y); overMap {
			return cursors.UnitGo
		}

		return cursors.Hand
	}

	cellX, cellY, overMap := s.mapCell(m.X, m.Y)

	if !overMap {
		return cursors.Hand // over the side/top panels
	}

	// During the opponent's (computer's) turn the player cannot issue orders, but
	// can still inspect: hovering over a unit or building it can see shows the
	// friend pointer (right click opens its Status), and every other map cell shows
	// the no go cursor. The view still scrolls; the edge arrows are handled above.
	// C++: the cursor is FRND_PTR over a unit, NO_PTR otherwise, while another team
	// takes its turn.
	if s.turnFlow.aiTurn {
		return s.spectatorCursor(cellX, cellY)
	}

	if id, armed := s.cursorForArmedOrder(cellX, cellY); armed {
		return id
	}

	// Build placement mode: the map crosshair (MAP_PTR) marks the site to pick.
	// C++: the SELECT_SITE cursor over the map.
	if s.buildUI.buildIdx > 0 {
		return cursors.Map
	}

	sel := s.selected

	// A build ready builder (just finished a structure) shows the activate pointer
	// over each of its BLDMRK exit markers. Clicking one drives it off. Off the
	// markers it is the plain hand (the parked builder takes no other order). C++:
	// the ACTIVATE cursor over a build marker (game_manager.cpp).
	if sel != nil && s.game.IsBuildReady(sel) {
		for _, c := range s.game.ExitMarkers(sel) {
			if c[0] == cellX && c[1] == cellY {
				return cursors.Activate
			}
		}

		return cursors.Hand
	}

	// A builder working its site is busy (it cannot be ordered), so the pointer
	// is the plain map hand, not a move/no go cursor. C++: GameManager_GetCursor
	// over a unit in ORDER_BUILD.
	if sel != nil && sel.Building {
		return cursors.Hand
	}

	hasSelection := sel != nil && sel.Team == s.game.State.ActiveTeam && s.buildUI.buildIdx == 0

	// FORCE ATTACK (SHIFT/CTRL held): a selected armed unit lights the enemy reticle
	// over ANY unit it could fire on - friendly units included - so the player can
	// force fire by clicking the target directly, without arming Attack from the popup
	// (vanilla has no Attack order). The normal enemy attack cursor is handled by
	// pickCursor below. C++: GameManager_GetMilitaryCursor CURSOR_ATTACK_SHIFT/CTRL.
	if hasSelection && s.forceModifierHeld() && s.game.CanForceAttack(sel, cellX, cellY) {
		return cursors.Enemy
	}

	t := cellCursorTarget{occupant: s.topUnitAt(cellX, cellY)}

	if hasSelection {
		t.canAttack = s.game.CanAttack(sel, cellX, cellY)
		// canEnter reflects terrain reachability only, NOT the budget for each turn: a
		// unit out of moves still shows the GO cursor, because clicking now queues
		// the route for next turn (the green/blue arrows). Vanilla confirmed to
		// keep the go here cursor at 0 moves; it just reroutes or defers the move.
		t.canEnter = s.game.CanEnter(sel, cellX, cellY)
		t.canFarTarget = s.game.CanFarTarget(sel, cellX, cellY)
	}

	return pickCursor(t, sel, s.game.State.ActiveTeam, hasSelection)
}

// cursorForArmedOrder returns the pointer for whichever order the player has armed from
// the popup (X-Fer, steal/disable, reload, repair, load, attack), and armed=false when
// none is. An armed order narrows the whole map to two answers: the order's own pointer
// over a valid target, and no go everywhere else.
func (s *Screen) cursorForArmedOrder(cellX, cellY int) (id cursors.ID, armed bool) {
	// X-Fer armed (popup -> X-Fer -> pick a target): the transfer pointer shows
	// ONLY over a valid adjacent transfer target (a matching cargo unit or a
	// connected complex building); every other cell is no go. C++: the FRND_XFR
	// cursor of the ORDER_TRANSFER target pick.
	if s.orderInput.xferArmed && s.orderInput.xferSource != nil {
		if s.transferTargetValid(cellX, cellY) {
			return cursors.Transfer, true
		}

		return cursors.UnitNoGo, true
	}

	// Commando Disable/Steal armed: the steal/disable pointer over a valid adjacent
	// enemy, no go elsewhere. C++: the CURSOR_STEAL/CURSOR_DISABLE picks in
	// GameManager_GetMilitaryCursor.
	if s.orderInput.infiltrateArmed && s.selected != nil {
		if s.orderInput.infiltrateSteal {
			if s.game.CanStealAt(s.selected, cellX, cellY) {
				return cursors.Steal, true
			}
		} else if s.game.CanDisableAt(s.selected, cellX, cellY) {
			return cursors.Disable, true
		}

		return cursors.UnitNoGo, true
	}

	// Supply Truck Reload armed: the reload pointer (PTR_RLD) over a valid adjacent
	// reloadable friendly, no go elsewhere. C++: the CURSOR_RELOAD pick in
	// GameManager_GetMilitaryCursor.
	if s.orderInput.reloadArmed && s.selected != nil {
		if s.game.CanReloadAt(s.selected, cellX, cellY) {
			return cursors.Reload, true
		}

		return cursors.UnitNoGo, true
	}

	// Mobile Repair unit armed: the repair pointer (FRND_FIX) over a valid adjacent
	// damaged friendly, no go elsewhere. C++: the CURSOR_REPAIR pick in
	// GameManager_GetMilitaryCursor.
	if s.orderInput.repairArmed && s.selected != nil {
		if s.game.CanRepairAt(s.selected, cellX, cellY) {
			return cursors.Repair, true
		}

		return cursors.UnitNoGo, true
	}

	// Enter/load armed: the friendly load pointer (FRND_LOD) over an adjacent
	// transport or storage building that can receive the unit, no go elsewhere.
	// C++: the CURSOR_UNIT / load pick in GameManager_GetCursor.
	if s.orderInput.loadArmed && s.orderInput.loadPassenger != nil {
		if s.game.CanLoad(s.pickFriendlyUnit(cellX, cellY), s.orderInput.loadPassenger) {
			return cursors.Load, true
		}

		return cursors.UnitNoGo, true
	}

	// Building "Load" armed: the friendly load pointer over any friendly unit this
	// Depot/Hangar/Dock can take (it paths in if not adjacent), no go elsewhere.
	if s.effects.loadInto != nil {
		friend := s.pickFriendlyUnit(cellX, cellY)

		if s.game.CanContain(s.effects.loadInto, friend) {
			return cursors.Load, true
		}

		// A friendly unit it cannot take (a builder at work, a full hold, the wrong class) gets the select
		// pointer: a click only selects it. C++: GameManager_GetCursor falls through to CURSOR_FRIEND.
		if friend != nil && friend != s.effects.loadInto {
			return cursors.Friend, true
		}

		return cursors.UnitNoGo, true
	}

	// Attack target pick armed (popup Attack): the enemy pointer (ENMY_PTR) over a
	// valid target, no go elsewhere. With SHIFT/CTRL held (FORCE ATTACK) a friendly
	// under the pointer is a valid target too. C++: the CURSOR_ATTACK / the
	// CURSOR_ATTACK_SHIFT force target pick in GameManager_GetMilitaryCursor.
	if s.orderInput.attackArmed && s.selected != nil {
		if s.forceModifierHeld() && s.game.CanForceAttack(s.selected, cellX, cellY) {
			return cursors.Enemy, true
		}

		if s.game.CanAttack(s.selected, cellX, cellY) {
			return cursors.Enemy, true
		}

		return cursors.UnitNoGo, true
	}

	return cursors.Hidden, false
}

// cellCursorTarget is what the pointer is over, resolved from screen state.
type cellCursorTarget struct {
	occupant     *game.UnitInfo // topmost selectable unit in the cell, or nil
	canAttack    bool           // the selected unit can fire on this cell
	canEnter     bool           // the selected unit can move into this cell (terrain + budget)
	canFarTarget bool           // the cell holds a valid enemy out of range (mobile attacker can close in)
}

// spectatorCursor is the cursor over a map cell during the opponent's (computer's)
// turn: the friend pointer over a unit/building the player can see (right click
// inspects it), and the no go pointer everywhere else (no orders can be issued).
// C++: FRND_PTR over a unit, NO_PTR otherwise, while another team takes its turn.
func (s *Screen) spectatorCursor(cellX, cellY int) cursors.ID {
	if u := s.topUnitAt(cellX, cellY); u != nil && s.unitVisibleToPlayer(u) {
		return cursors.Friend
	}

	return cursors.UnitNoGo
}

// pickCursor is the pure cursor decision over the map, ported from
// GameManager_GetCursor. With a controllable unit selected it favours the
// action under the pointer: attack, move, friend or no go. With nothing
// selected it follows the engine's null selection branch: friend over a
// friendly unit, no go over everything else (not the hand, so the cursor
// still reflects the terrain). The hand is only for off map and UI, which
// the caller handles.
func pickCursor(t cellCursorTarget, sel *game.UnitInfo, activeTeam int, hasSelection bool) cursors.ID {
	if !hasSelection {
		if t.occupant != nil && t.occupant.Team == activeTeam {
			return cursors.Friend
		}

		return cursors.UnitNoGo
	}

	if t.canAttack {
		return cursors.Enemy
	}

	if t.occupant != nil {
		// A FRIENDLY passable structure (own connector / landing pad / a building an air
		// unit can fly over) reads as drivable terrain: show the move cursor when the
		// selected unit can enter the cell. The same team guard matters for AIR units,
		// whose canEnter is true over ANY cell - without it an out of range ENEMY building
		// would wrongly show the green "move here" cursor instead of the attack / far
		// target cursor in the switch below. Ground units gate naturally (canEnter is
		// false into a building), so this only changed the air case. Drivable neutral
		// terrain (road / bridge / slab) is ground cover, excluded from topUnitAt, so it
		// arrives here as a nil occupant and the bottom canEnter branch handles it.
		// C++: GameManager_GetCursor shows UNIT_GO over own drivable terrain, the military
		// cursor over an enemy.
		if t.occupant.Flags.Has(game.FlagStationary) && t.canEnter && t.occupant.Team == sel.Team {
			return cursors.UnitGo
		}

		switch {
		case t.occupant == sel:
			return cursors.UnitNoGo // cannot move onto itself

		case t.occupant.Team == sel.Team:
			return cursors.Friend

		case t.canFarTarget:
			// A valid enemy out of range: the far target pointer hints "move closer to
			// fire" rather than a dead no go. C++: CURSOR_FAR_TARGET (GetMilitaryCursor).
			return cursors.FarTarget

		default:
			return cursors.UnitNoGo // an enemy out of attack range blocks the cell
		}
	}

	if t.canEnter {
		return cursors.UnitGo
	}

	return cursors.UnitNoGo
}

// topUnitAt returns the unit a click would select at (cellX, cellY): the topmost
// unit that is not ground cover, preferring a MOBILE unit over a stationary structure that
// shares its cell (a unit parked on a road/bridge wins over the road). nil when the
// cell holds only ground cover / a slab. Mirrors clickCell's selection rule and is
// also used for right click Status - both want the unit over the structure it sits
// on. C++ ported : access.cpp - Access_GetSelectableUnit (a ground cover / idle
// structure yields to the unit).

// selectableOccupant reports whether a cell occupant e is a valid SELECTION /
// cursor target for the human player. It is the one rule shared by topUnitAt and
// clickCell so the cursor and the click agree: a target must be a fully
// materialised unit that is not ground cover, that the player can actually SEE. Not a target:
// ground cover (slab/road), a unit still HIDDEN inside a building or SCALING UP
// (a parked factory output / unfolding structure), an undetected stealth unit
// (mine/sub/commando), and - while fog is shown - any unit the player cannot
// currently see (own units are always visible; the F2 omniscient reveal lifts the
// fog gate). C++ ported : access.cpp - Access_GetSelectableUnit (visibility gate).
func (s *Screen) selectableOccupant(e *game.UnitInfo) bool {
	if e.Flags.Has(game.FlagGroundCover) || e.HiddenUntilExpand || e.Unfold != 0 {
		return false
	}

	// A BRIDGE is drivable TERRAIN, not a selectable/attackable unit. It is FlagStationary
	// (not FlagGroundCover), so the flag check above misses it - and an ALIEN owned bridge
	// would then show a red attack cursor and silently absorb shots (no damage, no hit).
	// Excluding it lets a unit drive ACROSS the bridge instead. C++: a BRIDGE is a terrain
	// prop, never an Access_GetSelectableUnit / attack target.
	if e.Type == assets.Bridge {
		return false
	}

	// Waldo / Harvster are inert gag props: solid 1x1 obstacles you cannot select
	// (nor attack, see sim.targetableUnit), only walk around.
	if e.Type == assets.Waldo || e.Type == assets.Harvster {
		return false
	}

	if s.stealthHidden(e) {
		return false
	}

	// While the fog overlay is drawn, you cannot select what you cannot see; the
	// omniscient debug reveal (fogShow off) makes everything selectable.
	if s.fogShow && !s.unitVisibleToPlayer(e) {
		return false
	}

	return true
}

// selectableOccupants returns the units a click on this cell could land on, in a STABLE
// order: the active team's own units first, then by unit id. Cells.At hands back arrival
// order, and that order shifts every time a unit drives onto the cell, so without this
// two stacked units resolve one way for the hover readout and another for the click.
func (s *Screen) selectableOccupants(cellX, cellY int) []*game.UnitInfo {
	var occ []*game.UnitInfo

	for _, e := range s.game.Units.Cells.At(cellX, cellY) {
		if s.selectableOccupant(e) { // drops ground cover, hidden/unfolding units, and what the player cannot see
			occ = append(occ, e)
		}
	}

	own := s.game.State.ActiveTeam
	sort.SliceStable(occ, func(i, j int) bool {
		if a, b := occ[i].Team == own, occ[j].Team == own; a != b {
			return a // your own unit before an enemy's: an enemy cannot be acted on anyway
		}

		return occ[i].ID < occ[j].ID
	})

	return occ
}

// pickOccupant names the ONE unit a cell resolves to: a mobile unit beats a structure,
// and a real structure beats the floor it stands on (a water platform). The cursor and
// the click both go through it, so they can never name different units.
// C++ ported : access.cpp - Access_GetSelectableUnit.
func pickOccupant(occ []*game.UnitInfo) (mobile, structure, floor *game.UnitInfo) {
	for _, e := range occ {
		switch {
		case !e.Flags.Has(game.FlagStationary):
			if mobile == nil {
				mobile = e
			}

		case groundCoverLayer(e): // a water platform / flat bridge: the floor
			if floor == nil {
				floor = e
			}

		default: // a real structure standing on the cell
			if structure == nil {
				structure = e
			}
		}
	}

	return mobile, structure, floor
}

func (s *Screen) topUnitAt(cellX, cellY int) *game.UnitInfo {
	mobile, structure, floor := pickOccupant(s.selectableOccupants(cellX, cellY))

	switch {
	case mobile != nil:
		return mobile // a unit standing on a structure wins over the structure
	case structure != nil:
		return structure
	case floor != nil:
		return floor // only the floor is here
	}

	// No unit is hashed on this cell: fall back to a unit drawn centred on a 2x2
	// work site whose footprint covers it (build pose / clearing), so a click where
	// such a unit VISUALLY sits selects it. nil for a bare cell or one with only ground cover.
	return s.siteUnitAt(cellX, cellY)
}

// siteUnitAt finds a unit drawn CENTRED on a multi cell work site whose reserved
// footprint contains (cellX, cellY): a constructor in its build pose (Building, the
// 2x2/1x1 BuildSite) or a bulldozer clearing a building pile (Clearing, the
// ClearSite). Such a unit is hashed on its anchor cell only and sits off the other
// site cells, so a click on a site cell holding no unit should still find it.
// Returns nil when no site covers the cell. C++: the ORDER_BUILD / clear site is
// reserved across its footprint (units_manager.cpp).
func (s *Screen) siteUnitAt(cellX, cellY int) *game.UnitInfo {
	for _, u := range s.game.Units.Units() {
		switch {
		case u.Clearing:
			// A bulldozer's clearing site (the building pile it sits centred on).
			if ax, ay, fp, ok := s.game.ClearSite(u); ok && inFootprint(cellX, cellY, ax, ay, fp) {
				return u
			}

		case u.Building:
			// A constructor/engineer's reserved build footprint.
			if inFootprint(cellX, cellY, u.BuildSiteX, u.BuildSiteY, u.BuildFootprint) {
				return u
			}
		}
	}

	return nil
}

// scrollEdgeCursor resolves the edge scroll cursor for direction (dx, dy) against
// the live camera: a NO cursor when the view is fully clamped at that edge/corner,
// otherwise the arrow for the component(s) that can still scroll.
func (s *Screen) scrollEdgeCursor(dx, dy int) cursors.ID {
	vw, vh := viewSize(MainMap)
	mw, mh := s.r.MapPixels()
	maxX, maxY := mw-vw, mh-vh

	if maxX < 0 {
		maxX = 0
	}

	if maxY < 0 {
		maxY = 0
	}

	return scrollEdgeCursorID(s.camera.scrollX, s.camera.scrollY, maxX, maxY, dx, dy)
}

// scrollEdgeCursorID is the pure edge scroll cursor decision: it zeroes any axis
// already clamped at its scroll limit, then returns the NO cursor if nothing can
// move that way, or the arrow for the components that can still scroll. C++:
// game_manager.cpp - the edge scroll cursor reflects whether the view can still move.
func scrollEdgeCursorID(scrollX, scrollY, maxX, maxY, dx, dy int) cursors.ID {
	// Clamped left/right: drop the horizontal component.
	if (dx < 0 && scrollX <= 0) || (dx > 0 && scrollX >= maxX) {
		dx = 0
	}

	// Clamped top/bottom: drop the vertical component.
	if (dy < 0 && scrollY <= 0) || (dy > 0 && scrollY >= maxY) {
		dy = 0
	}

	// Fully clamped at this edge/corner: no further scroll, show the NO cursor.
	if dx == 0 && dy == 0 {
		return cursors.UnitNoGo
	}

	return scrollArrow(dx, dy)
}

// scrollArrow returns the directional scroll cursor for an edge scroll
// delta (dx, dy each in {-1, 0, 1}).
func scrollArrow(dx, dy int) cursors.ID {
	switch {
	case dy < 0 && dx == 0:
		return cursors.ArrowN

	case dy < 0 && dx > 0:
		return cursors.ArrowNE

	case dy == 0 && dx > 0:
		return cursors.ArrowE

	case dy > 0 && dx > 0:
		return cursors.ArrowSE

	case dy > 0 && dx == 0:
		return cursors.ArrowS

	case dy > 0 && dx < 0:
		return cursors.ArrowSW

	case dy == 0 && dx < 0:
		return cursors.ArrowW

	default: // dy < 0 && dx < 0
		return cursors.ArrowNW
	}
}
