// transfer.go : implements the resource transfer overlay.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
// C++ ported : unitinfo.cpp - transfer panel and execute transfer
package gamescreen

import (
	"github.com/suglasp/max-go/internal/build"
	"github.com/suglasp/max-go/internal/dbg"
	"github.com/suglasp/max-go/internal/economy"
	"github.com/suglasp/max-go/internal/game"
	"github.com/suglasp/max-go/internal/gfx"
	"github.com/suglasp/max-go/internal/platform"
	"github.com/suglasp/max-go/internal/screendata"
)

// TransferInfo now lives in internal/screendata, shared with the menus renderer.
// The gamescreen resolves complex pooling (TransferMenu_GetUnitCargoInfo) before
// filling one in, so the menus package stays free of the build network dependency.

// unitIcon renders a 32x32 team coloured icon for a unit (its resting body
// frame, scaled to fit) for the Transfer screen's icon wells.
//
// C++: ReportStats_DrawListItemIcon (reportstats.cpp).
func (s *Screen) unitIcon(u *game.UnitInfo) *gfx.Buffer {
	const iconSize = 32
	buf := gfx.New(iconSize, iconSize)
	s.sprites.DrawIcon(buf, u.Type, u.Team, iconSize/2, iconSize/2)
	return buf
}

// SetTransferFunc installs the hook the X-Fer order calls to run the Transfer
// screen (wired to menus.RunTransfer). It returns the signed amount the player
// chose to move from source to target (0 = cancelled).
func (s *Screen) SetTransferFunc(fn func(info screendata.TransferInfo) (int, error)) {
	s.hooks.transferFn = fn
}

// armTransfer arms an X-Fer target pick for the given source unit: the next
// click on an adjacent friendly unit with matching cargo opens the Transfer
// screen. C++: issuing ORDER_TRANSFER then GameManager_SelectNextUnit's pick.
func (s *Screen) armTransfer(u *game.UnitInfo) {
	if economy.CargoTypeOf(u.Type) == economy.CargoNone {
		s.showMessage("This unit carries no cargo to transfer.")
		return
	}

	s.orderInput.xferSource = u
	s.orderInput.xferArmed = true
	s.showMessage("Select an adjacent unit to exchange cargo with.")
}

// cancelTransfer disarms a pending X-Fer pick (right click / ESC / invalid pick).
func (s *Screen) cancelTransfer() {
	s.orderInput.xferSource = nil
	s.orderInput.xferArmed = false
}

// tryTransferTarget handles a click in (cellX, cellY) while an X-Fer pick is armed.
// It opens the Transfer screen when the cell holds a valid adjacent friendly
// unit of matching cargo type, and reports whether the click was consumed.
func (s *Screen) tryTransferTarget(cellX, cellY int) bool {
	if !s.orderInput.xferArmed || s.orderInput.xferSource == nil {
		return false
	}

	src := s.orderInput.xferSource
	dst := s.pickFriendlyUnit(cellX, cellY)
	s.cancelTransfer()

	if dst == nil || dst == src {
		s.showMessage("No unit there - transfer cancelled.")
		return true
	}

	if !s.transferReachable(src, dst) {
		s.showMessage("Move next to it or a connected structure to transfer.")
		return true
	}

	if !s.canExchangeCargo(src, dst) {
		s.showMessage("That target stores no matching cargo to exchange.")
		return true
	}

	s.openMenuScreen(func() { s.openTransfer(src, dst) }, false)
	return true
}

// transferTargetValid reports whether the armed X-Fer source can transfer with a
// unit/building at (cellX, cellY): a reachable active team unit of matching cargo,
// or a reachable complex building that pools the source's cargo type. The cursor
// uses it to show the transfer pointer only over a valid target.
func (s *Screen) transferTargetValid(cellX, cellY int) bool {
	src := s.orderInput.xferSource

	if src == nil {
		return false
	}

	dst := s.pickFriendlyUnit(cellX, cellY)

	if dst == nil || dst == src || !s.transferReachable(src, dst) {
		return false
	}

	return s.canExchangeCargo(src, dst)
}

// transferReachable reports whether src and dst are close enough to exchange cargo.
// A connected complex acts as ONE transfer endpoint: a mobile unit standing next to
// ANY building of a complex is "next to" the whole pool, so it can transfer with a
// building anywhere in that complex (manual p.64: "The unit does not have to be
// adjacent to the Material Storage Unit ... as long as it is adjacent to any
// structure that is ultimately connected to the storage unit."). Two mobile units,
// or two buildings, need direct adjacency. Applied symmetrically so the order of
// selection (the truck or the storage building first) does not matter.
//
// C++ ported : game_manager.cpp - GameManager_IsValidTransferTarget.
func (s *Screen) transferReachable(src, dst *game.UnitInfo) bool {
	if unitsAdjacent(src, dst) {
		return true
	}

	// A mobile unit reaches a building whose complex it stands beside: src mobile vs
	// dst's complex, or dst mobile vs src's complex.
	if !src.Flags.Has(game.FlagStationary) {
		if c := s.game.ComplexOf(dst); c != nil && s.adjacentToComplex(src, c) {
			return true
		}
	}

	if !dst.Flags.Has(game.FlagStationary) {
		if c := s.game.ComplexOf(src); c != nil && s.adjacentToComplex(dst, c) {
			return true
		}
	}

	return false
}

// adjacentToComplex reports whether u is adjacent to any building of complex c.
func (s *Screen) adjacentToComplex(u *game.UnitInfo, c *economy.Complex) bool {
	for _, m := range c.Members {
		if unitsAdjacent(u, m) {
			return true
		}
	}

	return false
}

// canExchangeCargo reports whether src may X-Fer cargo with the adjacent dst. It
// ports the CURSOR_ARROW_NE transfer test:
//   - PATH A: src and dst carry the SAME cargo kind and dst is not mid build/clear
//     (a truck<->truck swap, or a truck refilling from a same type storer); OR
//   - PATH B: a MOBILE src draws from a STATIONARY building whose complex pools
//     that cargo; any connected building represents the whole base store.
//
// A stationary source (a building) is barred from path B: a complex already pools
// its buildings automatically, so a mining station cannot push raw into an
// unrelated building (e.g. a power plant) it merely sits beside.
//
// C++ ported : game_manager.cpp - GameManager_GetCursor (CURSOR_ARROW_NE) +
// GameManager_GetUnitWithCargoType.
func (s *Screen) canExchangeCargo(src, dst *game.UnitInfo) bool {
	ctype := economy.CargoTypeOf(src.Type)

	if ctype == economy.CargoNone {
		return false
	}

	// Path A: same cargo kind, and the target is not in the middle of a build/clear.
	if economy.CargoTypeOf(dst.Type) == ctype && dst.Order != game.OrderBuild && dst.Order != game.OrderClear {
		return true
	}

	// Path B: a mobile unit refills from a stationary building whose complex stores
	// the cargo (the connectors pipe it across the base's connected buildings).
	if !src.Flags.Has(game.FlagStationary) && dst.Flags.Has(game.FlagStationary) {
		if c := s.game.ComplexOf(dst); c != nil {
			_, caps := c.CargoInfo()
			return caps.Get(ctype) != 0
		}
	}

	return false
}

// pickFriendlyUnit returns the topmost active team, non ground cover unit in a
// cell, or nil.
func (s *Screen) pickFriendlyUnit(cellX, cellY int) *game.UnitInfo {
	var pick *game.UnitInfo

	for _, e := range s.game.Units.Cells.At(cellX, cellY) {
		if e.Team == s.game.State.ActiveTeam && !e.Flags.Has(game.FlagGroundCover) {
			pick = e
		}
	}

	return pick
}

// openTransfer resolves both units' cargo sides, runs the Transfer screen and
// applies the chosen exchange. C++: TransferMenu_Menu + the storage write back.
func (s *Screen) openTransfer(src, dst *game.UnitInfo) {
	if s.hooks.transferFn == nil {
		return
	}

	ctype := economy.CargoTypeOf(src.Type)
	srcMat, srcCap, srcCx := s.cargoSide(src, dst, ctype)
	dstMat, dstCap, dstCx := s.cargoSide(dst, src, ctype)
	info := screendata.TransferInfo{
		SourceName:      displayName(src.Type),
		TargetName:      displayName(dst.Type),
		SourceMaterials: srcMat, SourceCapacity: srcCap,
		TargetMaterials: dstMat, TargetCapacity: dstCap,
		Cargo:      ctype,
		SourceIcon: s.unitIcon(src),
		TargetIcon: s.unitIcon(dst),
	}

	amount, err := s.hooks.transferFn(info)

	if err != nil {
		dbg.Printf("transfer: %v", err)
		return
	}

	if amount == 0 {
		return
	}

	// amount > 0 means the source gives to the target.
	s.adjustStored(src, srcCx, ctype, -amount)
	s.adjustStored(dst, dstCx, ctype, amount)
	s.playVoice(vsTransferComplete) // a committed X-Fer acknowledges "transfer complete" (V_F224)
	// Log the cargo move (giver -> receiver : N type) for debugging the economy.
	giver, receiver, n := src, dst, amount

	if amount < 0 {
		giver, receiver, n = dst, src, -amount
	}

	dbg.Printf("transfer: %s -> %s : %d %s", displayName(giver.Type), displayName(receiver.Type), n, ctype)
}

// cargoSide reports a unit's transfer materials and capacity for cargo type
// ctype, and the complex to write back to (nil = the unit's own storage). A
// building (or a unit) in a complex the other side does not share reports its
// complex's pooled cargo of ctype, so a complex building represents the
// whole connected base's store of the source's cargo.
//
// C++: TransferMenu_GetUnitCargoInfo (transfermenu.cpp).
func (s *Screen) cargoSide(u, other *game.UnitInfo, ctype economy.CargoType) (materials, capacity int, c *economy.Complex) {
	cu := s.game.ComplexOf(u)

	if cu != nil && cu != s.game.ComplexOf(other) {
		mats, caps := cu.CargoInfo()
		return mats.Get(ctype), caps.Get(ctype), cu
	}

	cap := 0

	if u.Values != nil {
		cap = u.Values.Get(game.AttribStorage)
	}

	return u.Storage, cap, nil
}

// adjustStored moves delta into (delta>0) or out of (delta<0) a unit's storage
// of cargo type ctype, distributing across the complex when one was resolved.
func (s *Screen) adjustStored(u *game.UnitInfo, c *economy.Complex, ctype economy.CargoType, delta int) {
	if c != nil {
		switch ctype {
		case economy.CargoRaw:
			c.Transfer(delta, 0, 0)
		case economy.CargoFuel:
			c.Transfer(0, delta, 0)
		case economy.CargoGold:
			c.Transfer(0, 0, delta)
		}

		return
	}

	cap := 0

	if u.Values != nil {
		cap = u.Values.Get(game.AttribStorage)
	}

	u.Storage = clampStore(u.Storage+delta, cap)
}

// clampStore clamps a storage value to [0, capacity].
func clampStore(v, cap int) int {
	if v < 0 {
		return 0
	}

	if v > cap {
		return cap
	}

	return v
}

// unitsAdjacent reports whether two units touch (their footprints are within
// one cell), the proximity the X-Fer order requires.
func unitsAdjacent(a, b *game.UnitInfo) bool {
	ax0, ay0, an := a.GridX, a.GridY, build.Footprint(a.Flags)
	bx0, by0, bn := b.GridX, b.GridY, build.Footprint(b.Flags)
	// Expand a's box by one cell and test intersection with b's box.
	return ax0-1 <= bx0+bn-1 && bx0 <= ax0+an && ay0-1 <= by0+bn-1 && by0 <= ay0+an
}

/* // note: unused code in Go port
// footprint repeated build.Footprint, which the adjacency check now calls.
// footprint returns a unit's cell span (2 for 2x2 buildings, else 1).
func footprint(u *game.UnitInfo) int {
	if u.Flags.Has(game.FlagBuilding) {
		return 2
	}

	return 1
}
*/

// handleXferPick resolves an armed X-Fer pick: a left click picks the adjacent unit to
// trade cargo with, a right click or a click off the map cancels. Returns whether it
// consumed the click, and it is checked before the normal click handlers.
func (s *Screen) handleXferPick(mouse platform.MouseState) bool {
	if s.orderInput.xferArmed && s.turnFlow.handover == -1 && !s.gameOver {
		if mouse.Buttons&platform.MouseReleaseRight != 0 {
			s.cancelTransfer()
			s.showMessage("Transfer cancelled.")
			return true
		}

		if mouse.Buttons&platform.MouseReleaseLeft != 0 {
			if cellX, cellY, ok := s.mapCell(mouse.X, mouse.Y); ok && s.tryTransferTarget(cellX, cellY) {
				return true
			}

			s.cancelTransfer()

			return true
		}
	}

	return false
}
