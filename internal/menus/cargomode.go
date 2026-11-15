// cargomode.go : the Purchase Menu Buy Units / Upgrades mode, its toggle, type plates and right hand list.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
//
// C++ ported : cargomenu.cpp - CargoMenu::PopulateTeamUnitsList; abstractupgrademenu.cpp - IsUnitFiltered, IsBetterUnit
package menus

import (
	"fmt"
	"sort"

	"github.com/suglasp/max-go/internal/assets"
	"github.com/suglasp/max-go/internal/economy"
	"github.com/suglasp/max-go/internal/fonts"
	"github.com/suglasp/max-go/internal/game"
	"github.com/suglasp/max-go/internal/gfx"
	"github.com/suglasp/max-go/internal/gnw"
)

// The two modes of the right hand list: the buyable roster, or every upgradable type through the plates.
const (
	cgModeBuy = iota
	cgModeUpgrades
)

// Event value and geometry of the mode toggle and the type plate row (the plates share the Upgrades menu
// event values 1003, 1005, 1007, 1009 and 1011).
const (
	cgToggleEvent = 1100
	cgToggleX     = 542
	cgToggleY     = 446
	cgPlateY      = 411
)

// rightList is the list shown in the right hand selector: the roster in Buy mode, the filtered upgradable
// types in Upgrades mode. The zero value menu is in Buy mode.
func (m *cargoMenu) rightList() []cargoRosterEntry {
	if m.mode == cgModeUpgrades {
		return m.list
	}

	return cargoRoster
}

// isRosterType reports whether t is one of the buyable roster types.
func isRosterType(t assets.UnitType) bool {
	for _, e := range cargoRoster {
		if e.t == t {
			return true
		}
	}

	return false
}

// cargoRowCost is the gold cost shown on a list row: stationary types use the Constructor rate, infantry
// and commandos the Training Hall rate, everything else the heavy plant rate.
//
// C++ ported : purchasetypeselector.cpp - PurchaseTypeSelector::Draw
func cargoRowCost(at []assets.UnitAttributes, t assets.UnitType) int {
	if int(t) < 0 || int(t) >= len(at) {
		return 0
	}

	turns := at[t].TurnsToBuild

	switch {
	case isUpgradableBuilding(t):
		return economy.RawConsumptionRate(assets.Constrct, 1) * turns
	case t == assets.Commando || t == assets.Infantry:
		return economy.RawConsumptionRate(assets.TrainHal, 1) * turns
	default:
		return economy.RawConsumptionRate(assets.LandPlt, 1) * turns
	}
}

// isUpgradableBuilding reports whether t is a stationary type from the upgradable building set.
func isUpgradableBuilding(t assets.UnitType) bool {
	for _, b := range game.UpgradableBuildingTypes() {
		if b == t {
			return true
		}
	}

	return false
}

// cargoUnitClass gives the class bits the type plates filter on.
func cargoUnitClass(t assets.UnitType) (ground, air, sea, building bool) {
	if isUpgradableBuilding(t) {
		return false, false, false, true
	}

	f := game.ProducedFlags(t)

	return f.Has(game.FlagMobileLandUnit), f.Has(game.FlagMobileAirUnit), f.Has(game.FlagMobileSeaUnit), false
}

// upgradeCandidates lists every type a team can build, in type order: the buildings (and mines) plus every
// mobile type some factory produces.
func upgradeCandidates() []assets.UnitType {
	seen := map[assets.UnitType]bool{}

	var out []assets.UnitType

	for _, t := range game.UpgradableBuildingTypes() {
		if !seen[t] {
			seen[t] = true
			out = append(out, t)
		}
	}

	for _, roster := range factoryBuild {
		for _, e := range roster {
			if !seen[e.t] {
				seen[e.t] = true
				out = append(out, e.t)
			}
		}
	}

	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })

	return out
}

// isAfter reports whether a sorts after b in the Upgrades list: land, sea, air, then stationary; attackers
// first, then fewer build turns, then the lower type id.
//
// C++ ported : abstractupgrademenu.cpp - AbstractUpgradeMenu::IsBetterUnit
func (m *cargoMenu) isAfter(a, b assets.UnitType) bool {
	aGround, aAir, aSea, _ := cargoUnitClass(a)
	bGround, bAir, bSea, _ := cargoUnitClass(b)

	if aGround != bGround {
		return !aGround
	}

	if aSea != bSea {
		return !aSea
	}

	if aAir != bAir {
		return !aAir
	}

	_, baseA := m.values(a)
	_, baseB := m.values(b)

	if baseA == nil || baseB == nil {
		return a > b
	}

	atkA, atkB := baseA.Get(game.AttribAttack) > 0, baseB.Get(game.AttribAttack) > 0

	if atkA != atkB {
		return !atkA
	}

	turnsA, turnsB := baseA.Get(game.AttribTurns), baseB.Get(game.AttribTurns)

	if turnsA != turnsB {
		return turnsA > turnsB
	}

	return a > b
}

// upgradeList builds the Upgrades mode list: the upgradable types the plates let through, sorted.
//
// C++ ported : abstractupgrademenu.cpp - AbstractUpgradeMenu::PopulateTeamUnitsList / IsUnitFiltered
func (m *cargoMenu) upgradeList() []cargoRosterEntry {
	f := upgradeFilterState

	var types []assets.UnitType

	for _, t := range upgradeCandidates() {
		_, base := m.values(t)
		if base == nil || len(game.UpgradableAttribs(base)) == 0 {
			continue
		}

		ground, air, sea, building := cargoUnitClass(t)
		if !((ground && f.ground) || (air && f.air) || (sea && f.sea) || (building && f.building)) {
			continue
		}

		if f.combat && base.Get(game.AttribAttack) <= 0 {
			continue
		}

		types = append(types, t)
	}

	for i := 0; i+1 < len(types); i++ {
		for j := i + 1; j < len(types); j++ {
			if m.isAfter(types[i], types[j]) {
				types[i], types[j] = types[j], types[i]
			}
		}
	}

	out := make([]cargoRosterEntry, len(types))
	for i, t := range types {
		out[i] = cargoRosterEntry{t, rosterName(t)}
	}

	return out
}

// setMode switches the right hand list between the roster and the upgradable types, enables the plates in
// Upgrades mode only, and keeps the selected type when the new list has it.
//
// C++ ported : cargomenu.cpp - CargoMenu::PopulateTeamUnitsList
func (m *cargoMenu) setMode(mode int) {
	prev, had := m.buySelectedType()
	m.mode = mode

	for _, p := range m.plates {
		if mode == cgModeUpgrades {
			m.ctx.Sys.EnableButton(p.bid)
		} else {
			m.ctx.Sys.DisableButton(p.bid)
		}
	}

	m.refreshList(prev, had)
}

// refreshList rebuilds the Upgrades list (when in that mode), reselects prev when it is still listed (else
// row 0) and repaints.
//
// C++ ported : unittypeselector.cpp - UnitTypeSelector::AddItems
func (m *cargoMenu) refreshList(prev assets.UnitType, had bool) {
	if m.mode == cgModeUpgrades {
		m.list = m.upgradeList()
	}

	list := m.rightList()
	m.buySel, m.buyScroll = 0, 0

	if had {
		for i, e := range list {
			if e.t == prev {
				m.buySel = i

				break
			}
		}
	}

	m.clampScroll()
	m.drawBuyList()
	m.drawCargoList()
	m.drawDetail()
	m.updateBuyButton()
}

// buySelectedType is the type under the right list selection, if any.
func (m *cargoMenu) buySelectedType() (assets.UnitType, bool) {
	list := m.rightList()
	if m.buySel >= 0 && m.buySel < len(list) {
		return list[m.buySel].t, true
	}

	return 0, false
}

// applyPlateToggle handles a press of a type plate: the plate's checked state becomes the shared filter
// flag. It reports whether key was a plate event.
func (m *cargoMenu) applyPlateToggle(key int) bool {
	p, ok := m.plates[key]
	if !ok {
		return false
	}

	*p.state = m.ctx.Sys.ButtonChecked(p.bid)

	return true
}

// buildModeWidgets creates the Buy Units / Upgrades toggle and the five type plates, returning the ids for
// the caller to free. The plates start disabled (Buy mode) and rest in the shared filter state.
//
// C++ ported : cargomenu.cpp - CargoMenu ctor (button_buy_upgrade_toggle, button_ground ... button_combat)
func (m *cargoMenu) buildModeWidgets() []gnw.ButtonID {
	ctx := m.ctx

	var buttons []gnw.ButtonID

	if b, e := m.buildToggle(); e == nil {
		m.toggleBtn = b
		ctx.registerButtonSfx(b, "KCARG0")
		buttons = append(buttons, b)
	}

	m.plates = map[int]upgradeFilterPlate{}
	state := &upgradeFilterState
	plates := []upgradeFilterButton{
		{467, "SELGRD_U", "SELGRD_D", 1003, &state.ground},
		{500, "SELAIR_U", "SELAIR_D", 1005, &state.air},
		{533, "SELSEA_U", "SELSEA_D", 1007, &state.sea},
		{565, "SELBLD_U", "SELBLD_D", 1009, &state.building},
		{598, "SELCBT_U", "SELCBT_D", 1011, &state.combat},
	}

	dxImg, dxErr := ctx.loadSimple("CRGSELDX")

	for _, fb := range plates {
		b, e := ctx.imageButton(mainWindow, fb.x, cgPlateY, fb.up, fb.down, "", 0, 0, true, fb.on)
		if e != nil {
			continue
		}

		ctx.Sys.SetButtonToggle(b)
		ctx.Sys.SetRestState(b, *fb.state)

		if dxErr == nil {
			dis := dxImg.Buffer().Clone()
			ctx.Sys.SetDisabledImages(b, dis, dis)
		}

		ctx.Sys.DisableButton(b) // Buy mode first
		ctx.registerButtonSfx(b, "KCARG0")
		buttons = append(buttons, b)
		m.plates[fb.on] = upgradeFilterPlate{bid: b, state: fb.state}
	}

	return buttons
}

// buildToggle creates the sticky CRGTOG button with its two captions: "Buy Units" on the upper half and
// "Upgrades" on the lower half, the active one dark orange. Up is Buy mode, checked is Upgrades mode.
//
// C++ ported : cargomenu.cpp - CargoMenu ctor (button_buy_upgrade_toggle SetCaption x2)
func (m *cargoMenu) buildToggle() (gnw.ButtonID, error) {
	ctx := m.ctx

	upImg, err := ctx.loadSimple("CRGTOG_U")
	if err != nil {
		return -1, err
	}

	downImg, err := ctx.loadSimple("CRGTOG_D")
	if err != nil {
		return -1, err
	}

	up, down := upImg.Buffer().Clone(), downImg.Buffer().Clone()
	f := ctx.Sys.Fonts().Font()

	caption := func(b *gfx.Buffer, text string, boxY, boxH int, color fonts.Color) {
		x := max(20+(60-f.Width(text))/2, 0)
		y := boxY + (boxH-f.Height)/2
		f.BlitShadowed(b.Sub(x, y, b.W-x, b.H-y), text, 0, color)
	}

	buyText, upText := ctx.text("834a", "Buy Units"), ctx.text("e02a", "Upgrades")
	caption(up, buyText, 0, 15, fonts.ColorDarkOrange)
	caption(down, buyText, 0, 15, fonts.ColorGold)
	caption(up, upText, 15, 16, fonts.ColorGold)
	caption(down, upText, 15, 16, fonts.ColorDarkOrange)

	b := ctx.Sys.AddButton(mainWindow, cgToggleX, cgToggleY, up.W, up.H,
		gnw.NoValue, gnw.NoValue, cgToggleEvent, gnw.NoValue, up, down, nil, gnw.ButtonSticky)
	if b < 0 {
		return -1, fmt.Errorf("menus: AddButton failed for the Buy / Upgrades toggle")
	}

	return b, nil
}
