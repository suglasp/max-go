// cargo_test.go : tests starting loadouts, purchases, and protected units.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package menus

import (
	"testing"

	"github.com/suglasp/max-go/internal/assets"
	"github.com/suglasp/max-go/internal/economy"
	"github.com/suglasp/max-go/internal/fonts"
	"github.com/suglasp/max-go/internal/game"
	"github.com/suglasp/max-go/internal/gfx"
	"github.com/suglasp/max-go/internal/gnw"
	"github.com/suglasp/max-go/internal/platform"
	"github.com/suglasp/max-go/internal/resfile"
	"github.com/suglasp/max-go/internal/testutil"
	"github.com/suglasp/max-go/internal/units"
)

// TestNewCargoLoadoutFreeMinimums: a fresh loadout starts with the free minimum
// set (Constructor + Engineer + Surveyor), not charged against the starting gold.
func TestNewCargoLoadoutFreeMinimums(t *testing.T) {
	lo := NewCargoLoadout(0, 250)
	if lo.TeamGold != 250 {
		t.Errorf("the minimum set must be free, gold=%d want 250", lo.TeamGold)
	}
	if len(lo.Units) != len(cargoMinimumTypes) {
		t.Fatalf("default loadout has %d units, want %d", len(lo.Units), len(cargoMinimumTypes))
	}
	for _, want := range cargoMinimumTypes {
		found := false
		for _, u := range lo.Units {
			if u == want {
				found = true
			}
		}
		if !found {
			t.Errorf("default loadout missing the minimum type %v", want)
		}
	}
	if len(lo.Cargos) != len(lo.Units) {
		t.Errorf("Cargos length %d should match Units %d", len(lo.Cargos), len(lo.Units))
	}
}

// TestNewCargoLoadoutSeedsBuilderCargo: the free Constructor and Engineer open the
// purchase screen already loaded with their default raw materials (the same amounts
// they land with in a game that never shows this screen), and that load is free, so
// the Credit bar still reads the full starting gold.
func TestNewCargoLoadoutSeedsBuilderCargo(t *testing.T) {
	lo := NewCargoLoadout(0, 250)

	for i, u := range lo.Units {
		want := economy.StartBuilderCargo(u)
		if lo.Cargos[i] != want {
			t.Errorf("%v starts with cargo %d, want %d", u, lo.Cargos[i], want)
		}
	}

	if lo.Cargos[0] != economy.StartConstructorCargo || lo.Cargos[1] != economy.StartEngineerCargo {
		t.Errorf("builder cargo = %d / %d, want %d / %d", lo.Cargos[0], lo.Cargos[1],
			economy.StartConstructorCargo, economy.StartEngineerCargo)
	}

	if lo.TeamGold != 250 {
		t.Errorf("the seeded cargo must be free, gold=%d want 250", lo.TeamGold)
	}
}

// TestCargoCanDeletePerTypeMinimum: the last Constructor/Engineer/Surveyor cannot be
// deleted, but a bought duplicate (or any not minimum type) can.
func TestCargoCanDeletePerTypeMinimum(t *testing.T) {
	lo := NewCargoLoadout(0, 250)
	for i := range lo.Units {
		if lo.CanDelete(i) {
			t.Errorf("the last %v must not be deletable", lo.Units[i])
		}
	}

	// Buying a second Constructor makes either copy removable, but one must remain.
	lo.Units = append(lo.Units, assets.Constrct)
	lo.Cargos = append(lo.Cargos, 0)
	deletable := 0
	for i := range lo.Units {
		if lo.Units[i] == assets.Constrct && lo.CanDelete(i) {
			deletable++
		}
	}
	if deletable != 2 {
		t.Errorf("with 2 Constructors both should be deletable down to 1, got %d", deletable)
	}

	// A type that is not a minimum (Tank) is always deletable.
	lo.Units = append(lo.Units, assets.Tank)
	lo.Cargos = append(lo.Cargos, 0)
	if !lo.CanDelete(len(lo.Units) - 1) {
		t.Error("a Tank (non-minimum type) should be deletable")
	}
	// Out of range index is not deletable (no panic).
	if lo.CanDelete(-1) || lo.CanDelete(len(lo.Units)) {
		t.Error("an out-of-range index must not be deletable")
	}
}

// TestCargoUnitCost checks the loadout cost is the land factory raw rate (3 at
// build speed 1) times the unit's build turns, and that an out of range type is
// free rather than panicking.
func TestCargoUnitCost(t *testing.T) {
	at := make([]assets.UnitAttributes, int(assets.Tank)+1)
	at[assets.Tank] = assets.UnitAttributes{TurnsToBuild: 4}
	at[assets.Constrct] = assets.UnitAttributes{TurnsToBuild: 8}

	if got := cargoUnitCost(at, assets.Tank); got != 12 { // 3 * 4
		t.Errorf("Tank cost = %d, want 12", got)
	}
	if got := cargoUnitCost(at, assets.Constrct); got != 24 { // 3 * 8
		t.Errorf("Constructor cost = %d, want 24", got)
	}
	if got := cargoUnitCost(at, assets.UnitType(len(at)+5)); got != 0 {
		t.Errorf("out-of-range cost = %d, want 0", got)
	}
}

func TestCargoCanBuy(t *testing.T) {
	at := make([]assets.UnitAttributes, int(assets.Tank)+1)
	at[assets.Tank] = assets.UnitAttributes{TurnsToBuild: 4} // 12 credits
	if !cargoCanBuy(at, 12, assets.Tank) {
		t.Error("exact credit should be enough to buy")
	}
	if cargoCanBuy(at, 11, assets.Tank) {
		t.Error("insufficient credit should disable Buy")
	}
}

// TestCargoRosterNames checks every roster entry has a name and rosterName
// resolves a known type and falls back to the tag for an unknown one.
func TestCargoRosterNames(t *testing.T) {
	for _, e := range cargoRoster {
		if e.name == "" {
			t.Errorf("roster entry %v has no name", e.t)
		}
	}
	if got := rosterName(assets.Tank); got != "Tank" {
		t.Errorf("rosterName(Tank) = %q, want Tank", got)
	}
	// A type not in the roster falls back to its tag (not empty), not a crash.
	if got := rosterName(assets.PowGen); got == "" {
		t.Error("rosterName(PowGen) returned empty")
	}
}

// newTestCargoMenu builds a cargoMenu on a headless gnw System with the
// CARGOPIC art loaded, mirroring RunCargo's setup minus buttons and palette.
// Without buttons the Buy/Delete ids stay -1, which the update methods no-op on.
func newTestCargoMenu(t *testing.T, lo *CargoLoadout) *cargoMenu {
	t.Helper()
	dir := testutil.DataDir(t)
	lib, err := resfile.OpenDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { lib.Close() })
	a, err := assets.Load(lib, dir, "")
	if err != nil {
		t.Fatal(err)
	}
	fs, err := fonts.LoadSet(dir)
	if err != nil {
		t.Fatal(err)
	}
	screen := gfx.New(platform.ScreenWidth, platform.ScreenHeight)
	sys := gnw.New(screen, fs)
	ctx := &Context{Sys: sys, Lib: lib, Assets: a, DataDir: dir}

	pic, err := ctx.loadBig("CARGOPIC")
	if err != nil {
		t.Fatal(err)
	}
	ctx.drawBigAt(pic, 0, 0)
	sys.Fonts().SetFont(fonts.Font5)

	m := &cargoMenu{
		ctx: ctx, lo: lo, active: cgBuyList,
		buyRows: cgBuyH / cgRowH, cargoRows: cgCargoH / cgRowH,
		pic: pic, f5: sys.Fonts().Get(fonts.Font5), buf: sys.Buf(mainWindow),
		sc: units.NewSpriteCache(lib, &pic.Palette), at: a.Attribs,
		baseVals: map[assets.UnitType]*game.UnitValues{},
		curVals:  map[assets.UnitType]*game.UnitValues{},
		/* statIcons, portraitCache: note: unused code in Go port, the caches moved to Context */
		buyBtn: -1, deleteBtn: -1,
	}
	m.entry = *lo
	m.entry.Units = append([]assets.UnitType(nil), lo.Units...)
	m.entry.Cargos = append([]int(nil), lo.Cargos...)
	return m
}

// TestCargoCommitUpgrades checks that an attribute step bought on the purchase screen
// leaves the menu as a committed offset (with its gold tally), so the caller can apply
// it to the team before it lands. An untouched type contributes nothing.
func TestCargoCommitUpgrades(t *testing.T) {
	lo := NewCargoLoadout(0, 250)
	m := newTestCargoMenu(t, lo)

	cur, base := m.values(assets.Tank)
	if cur == nil || base == nil {
		t.Fatal("no values for the Tank")
	}

	step := game.UpgradeStep(game.AttribArmor, base.Get(game.AttribArmor))
	cur.Set(game.AttribArmor, base.Get(game.AttribArmor)+step)

	// A type merely LOOKED at (values built, nothing raised) must not commit an offset.
	m.values(assets.Scout)

	ups := m.commitUpgrades()
	if len(ups) != 1 {
		t.Fatalf("committed %d upgrades, want 1: %+v", len(ups), ups)
	}

	if ups[0].Type != assets.Tank || ups[0].Attrib != game.AttribArmor || ups[0].Offset != step {
		t.Errorf("committed %+v, want Tank armor +%d", ups[0], step)
	}
}

// TestCargoBuyLoadDelete verifies that buying spends gold, cargo costs one gold per five units, and deleting refunds the unit and its cargo.
func TestCargoBuyLoadDelete(t *testing.T) {
	lo := NewCargoLoadout(0, 250)
	m := newTestCargoMenu(t, lo)

	// Buy the roster's Tank row.
	for i, e := range cargoRoster {
		if e.t == assets.Tank {
			m.buySel = i
		}
	}
	cost := cargoUnitCost(m.at, assets.Tank)
	if cost <= 0 || cost > lo.TeamGold {
		t.Fatalf("tank cost %d not buyable with %d gold", cost, lo.TeamGold)
	}
	before := lo.TeamGold
	m.buySelected()
	if len(lo.Units) != len(cargoMinimumTypes)+1 || lo.Units[len(lo.Units)-1] != assets.Tank {
		t.Fatalf("tank not appended: %v", lo.Units)
	}
	if lo.TeamGold != before-cost {
		t.Errorf("gold after buy = %d, want %d", lo.TeamGold, before-cost)
	}
	if m.cargoSel != len(lo.Units)-1 {
		t.Errorf("cargoSel = %d, want the new unit %d", m.cargoSel, len(lo.Units)-1)
	}

	// The Tank and Surveyor have no storage, so buy a Supply Truck and load ten cargo into it.
	for i, e := range cargoRoster {
		if e.t == assets.SplyTrck {
			m.buySel = i
		}
	}
	m.buySelected()
	goldBefore := lo.TeamGold
	m.loadCargoTo(10)
	if lo.Cargos[m.cargoSel] != 10 {
		t.Fatalf("truck cargo = %d, want 10", lo.Cargos[m.cargoSel])
	}
	if lo.TeamGold != goldBefore-2 {
		t.Errorf("gold after 10 cargo = %d, want %d (1 per 5)", lo.TeamGold, goldBefore-2)
	}

	// Delete the truck: refunds its cost plus cargo/5.
	truckCost := cargoUnitCost(m.at, assets.SplyTrck)
	goldBefore = lo.TeamGold
	unitsBefore := len(lo.Units)
	m.deleteSelected()
	if len(lo.Units) != unitsBefore-1 {
		t.Fatalf("truck not deleted: %v", lo.Units)
	}
	if lo.TeamGold != goldBefore+truckCost+2 {
		t.Errorf("gold after delete = %d, want %d", lo.TeamGold, goldBefore+truckCost+2)
	}
}

// TestCargoDisabledFacesHaveNoCaption checks the disabled Buy face is the plain CRGBUY_X art, with no caption.
func TestCargoDisabledFacesHaveNoCaption(t *testing.T) {
	m := newTestCargoMenu(t, NewCargoLoadout(0, 250))
	ctx := m.ctx

	bid, err := ctx.cargoPlateButtonWithDisabled(mainWindow, 590, 386, "CRGBUY_U", "CRGBUY_D", "CRGBUY_X", "Buy", 1102)
	if err != nil {
		t.Fatal(err)
	}

	ctx.Sys.DisableButton(bid)

	art, err := ctx.loadSimple("CRGBUY_X")
	if err != nil {
		t.Fatal(err)
	}

	want := art.Buffer()
	got := ctx.Sys.Buf(mainWindow)

	for y := 0; y < want.H; y++ {
		for x := 0; x < want.W; x++ {
			if w := want.At(x, y); w != 0 && got.At(590+x, 386+y) != w {
				t.Fatalf("disabled Buy face differs from CRGBUY_X at %d,%d: got %d want %d", x, y, got.At(590+x, 386+y), w)
			}
		}
	}
}

// TestRepairShopDisabledCaptionGray checks the Repair Shop disabled face keeps its caption in dark gray.
func TestRepairShopDisabledCaptionGray(t *testing.T) {
	m := newTestCargoMenu(t, NewCargoLoadout(0, 250))
	ctx := m.ctx
	const x, y = 100, 100

	bid, err := ctx.captionButton(x, y, "DPTMNUUP", "DPTMNUDN", "DPTMNUDN", "Repair", 1)
	if err != nil {
		t.Fatal(err)
	}

	ctx.Sys.DisableButton(bid)

	// bake renders the disabled art with the caption colour, as the button does.
	bake := func(color fonts.Color) *gfx.Buffer {
		img, err := ctx.loadSimple("DPTMNUDN")
		if err != nil {
			t.Fatal(err)
		}

		b := img.Buffer().Clone()
		keyButtonCorner(b)
		f := ctx.Sys.Fonts().Font()
		tx, ty := max((b.W-f.Width("Repair"))/2, 0), max((b.H-f.Height)/2, 0)
		f.BlitShadowed(b.Sub(tx, ty, b.W-tx, b.H-ty), "Repair", 0, color)

		return b
	}

	matches := func(want *gfx.Buffer) bool {
		got := ctx.Sys.Buf(mainWindow)

		for yy := 0; yy < want.H; yy++ {
			for xx := 0; xx < want.W; xx++ {
				if w := want.At(xx, yy); w != 0 && got.At(x+xx, y+yy) != w {
					return false
				}
			}
		}

		return true
	}

	if !matches(bake(fonts.ColorDarkGray)) {
		t.Error("the disabled caption is not dark gray")
	}

	if matches(bake(fonts.ColorDarkOrange)) {
		t.Error("the disabled caption is still dark orange")
	}
}

// TestCargoRowCost checks the row cost rule per unit class.
func TestCargoRowCost(t *testing.T) {
	at := make([]assets.UnitAttributes, int(assets.Commando)+int(assets.Tank)+int(assets.PowGen)+8)
	for _, ty := range []assets.UnitType{assets.PowGen, assets.Infantry, assets.Tank} {
		at[ty] = assets.UnitAttributes{TurnsToBuild: 10}
	}

	cases := []struct {
		t    assets.UnitType
		want int
	}{
		{assets.PowGen, 20},   // stationary: Constructor rate 2
		{assets.Infantry, 10}, // Training Hall rate 1
		{assets.Tank, 30},     // heavy plant rate 3
	}

	for _, c := range cases {
		if got := cargoRowCost(at, c.t); got != c.want {
			t.Errorf("cargoRowCost(%v) = %d, want %d", c.t, got, c.want)
		}
	}
}

// resetUpgradeFilter restores the shared filter state after a test.
func resetUpgradeFilter(t *testing.T) {
	t.Helper()

	saved := upgradeFilterState
	t.Cleanup(func() { upgradeFilterState = saved })

	upgradeFilterState = upgradeFilter{ground: true, air: true, sea: true, building: true}
}

// indexOfType returns the row of type ty in list, or -1.
func indexOfType(list []cargoRosterEntry, ty assets.UnitType) int {
	for i, e := range list {
		if e.t == ty {
			return i
		}
	}

	return -1
}

// TestCargoUpgradesMode checks the Upgrades list content and order, the plates, the selection rule on a
// switch, and that Buy only works for a roster type.
func TestCargoUpgradesMode(t *testing.T) {
	resetUpgradeFilter(t)

	lo := NewCargoLoadout(0, 250)
	m := newTestCargoMenu(t, lo)

	if len(m.rightList()) != len(cargoRoster) {
		t.Fatalf("Buy mode list has %d rows, want the roster %d", len(m.rightList()), len(cargoRoster))
	}

	m.buySel = indexOfType(cargoRoster, assets.Tank)
	m.setMode(cgModeUpgrades)

	list := m.rightList()
	tank, corvette := indexOfType(list, assets.Tank), indexOfType(list, assets.Corvette)
	fighter, powGen := indexOfType(list, assets.Fighter), indexOfType(list, assets.PowGen)
	surveyor := indexOfType(list, assets.Surveyor)

	if tank < 0 || corvette < 0 || fighter < 0 || powGen < 0 || surveyor < 0 {
		t.Fatalf("Upgrades list misses a type: tank %d corvette %d fighter %d powgen %d surveyor %d", tank, corvette, fighter, powGen, surveyor)
	}

	if !(tank < surveyor && surveyor < corvette && corvette < fighter && fighter < powGen) {
		t.Errorf("order wrong: tank %d surveyor %d corvette %d fighter %d powgen %d", tank, surveyor, corvette, fighter, powGen)
	}

	if got, _ := m.buySelectedType(); got != assets.Tank {
		t.Errorf("the Tank selection was not kept on the switch, got %v", got)
	}

	// A plate off removes its class.
	upgradeFilterState.air = false
	m.refreshList(assets.Tank, true)

	if indexOfType(m.rightList(), assets.Fighter) >= 0 {
		t.Error("the Air plate is off but the Fighter is still listed")
	}

	// Buy is refused for a non roster type.
	m.buySel = indexOfType(m.rightList(), assets.PowGen)
	before := len(lo.Units)
	m.buySelected()

	if len(lo.Units) != before {
		t.Error("Buy bought a building")
	}

	// Back to Buy: the roster again; a type that is not in it resets the selection to row 0.
	m.setMode(cgModeBuy)

	if len(m.rightList()) != len(cargoRoster) || m.buySel != 0 {
		t.Errorf("Buy mode: %d rows, selection %d, want the roster and row 0", len(m.rightList()), m.buySel)
	}
}

// TestCargoModeWidgets checks the plates start disabled in Buy mode and the toggle and plates drive the
// shared filter.
func TestCargoModeWidgets(t *testing.T) {
	resetUpgradeFilter(t)

	m := newTestCargoMenu(t, NewCargoLoadout(0, 250))
	m.buildModeWidgets()

	if len(m.plates) != 5 {
		t.Fatalf("%d plates built, want 5", len(m.plates))
	}

	air := m.plates[1005].bid
	if !m.ctx.Sys.ButtonDisabled(air) {
		t.Error("the plates must be disabled in Buy mode")
	}

	m.setMode(cgModeUpgrades)

	if m.ctx.Sys.ButtonDisabled(air) {
		t.Error("the plates must be enabled in Upgrades mode")
	}

	press := func(x, y int) {
		m.ctx.Sys.CheckButtons(platform.MouseState{X: x, Y: y, Buttons: platform.MousePressLeft})
		m.ctx.Sys.CheckButtons(platform.MouseState{X: x, Y: y, Buttons: platform.MouseReleaseLeft})
	}

	press(500+4, cgPlateY+4)

	if !m.applyPlateToggle(1005) || upgradeFilterState.air {
		t.Error("a click on the Air plate did not turn the shared filter off")
	}

	press(cgToggleX+4, cgToggleY+4)

	if !m.ctx.Sys.ButtonChecked(m.toggleBtn) {
		t.Error("the toggle did not latch to Upgrades")
	}
}
