// cargoactions.go : holds the purchase screen state changes: buy, delete, cargo load
// and the attribute upgrade steppers.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
//
// C++: cargomenu.cpp - CargoMenu::EventHandler + CommitUpgradeChanges
package menus

import (
	"sort"

	"github.com/suglasp/max-go/internal/assets"
	"github.com/suglasp/max-go/internal/economy"
	"github.com/suglasp/max-go/internal/game"
)

// stepAttribute applies one click of the attribute stepper column to the row
// under the cursor: key 2501 buys an upgrade step, 2500 sells one back. Rows
// without a drawn stepper are left alone. C++: CargoMenu upgrade arrow handling.
func (m *cargoMenu) stepAttribute(key int) {
	ctx := m.ctx
	lo := m.lo

	if t, ok := m.selectedType(); ok {
		my := ctx.Plat.Mouse().Y
		for _, ur := range m.upRows {
			if my < ur.y || my >= ur.y+19 {
				continue
			}
			cur, base := m.values(t)
			b, c := base.Get(ur.attr), cur.Get(ur.attr)
			step := game.UpgradeStep(ur.attr, b)
			if key == 2501 { // buy one upgrade step
				cost := game.UpgradeCost(ur.attr, c, 0, b)
				if cost > 0 && cost < game.UpgradeCostLimit && cost <= lo.TeamGold {
					cur.Set(ur.attr, c+step)
					lo.TeamGold -= cost
					lo.UpgradeGold += cost // the score screen's "gold spent on upgrades" tally
				}
			} else if c-step >= b { // sell back (not below the base value)
				refund := game.UpgradeCost(ur.attr, c-step, 0, b)
				lo.TeamGold += refund
				lo.UpgradeGold -= refund
				cur.Set(ur.attr, c-step)
			}
			m.drawDetail()
			m.drawBars()
			m.updateBuyButton()
			break
		}
	}
}

// commitUpgrades turns the steppers' working values into the offsets the caller
// applies to the team: one entry per attribute a type was raised above its spec base.
// Types the player never touched contribute nothing. The list is sorted by type and
// attribute so a committed loadout is stable to compare and to log.
//
// C++ ported : cargomenu.cpp - CargoMenu::CommitUpgradeChanges
func (m *cargoMenu) commitUpgrades() []game.UnitUpgrade {
	var ups []game.UnitUpgrade

	for t, cur := range m.curVals {
		base := m.baseVals[t]

		if base == nil || cur == nil {
			continue
		}

		for a := game.Attrib(0); a < game.AttribCount; a++ {
			if off := cur.Get(a) - base.Get(a); off != 0 {
				ups = append(ups, game.UnitUpgrade{Type: t, Attrib: a, Offset: off})
			}
		}
	}

	sort.Slice(ups, func(i, j int) bool {
		if ups[i].Type != ups[j].Type {
			return ups[i].Type < ups[j].Type
		}

		return ups[i].Attrib < ups[j].Attrib
	})

	return ups
}

// values returns (and lazily builds) the cur/base value tables for a type.
// C++: UnitsManager_TeamInfo[team].team_units values.
func (m *cargoMenu) values(t assets.UnitType) (cur, base *game.UnitValues) {
	if int(t) >= len(m.at) {
		return nil, nil
	}
	if base = m.baseVals[t]; base == nil {
		base = game.NewUnitValues(m.at[t])
		m.baseVals[t] = base
	}
	if cur = m.curVals[t]; cur == nil {
		cur = base.Clone()
		m.curVals[t] = cur
	}
	return cur, base
}

// selectedType returns the unit the focused list points at, or false.
func (m *cargoMenu) selectedType() (assets.UnitType, bool) {
	if m.active == cgCargoList {
		if m.cargoSel < len(m.lo.Units) {
			return m.lo.Units[m.cargoSel], true
		}
		return 0, false
	}
	if m.buySel < len(cargoRoster) {
		return cargoRoster[m.buySel].t, true
	}
	return 0, false
}

// loadCargoTo sets the selected Purchased unit's cargo to target (rounded to a
// step of 5, clamped to its storage and to what gold allows), adjusting the
// team gold by 1 per 5 cargo. Gold trucks and units with no storage can't load.
// C++: CargoMenu::EventHandler (team_gold += old/new; cargos = value*5).
func (m *cargoMenu) loadCargoTo(target int) {
	if m.cargoSel >= len(m.lo.Units) {
		return
	}
	ut := m.lo.Units[m.cargoSel]
	if int(ut) >= len(m.at) || m.at[ut].Storage <= 0 ||
		economy.CargoTypeOf(ut) == economy.CargoNone || ut == assets.GoldTrck {
		return
	}
	st := m.at[ut].Storage
	target = max(0, min(target/5*5, st/5*5))
	cur := m.lo.Cargos[m.cargoSel]
	if cost := (target - cur) / 5; cost > m.lo.TeamGold { // can't afford the increase
		target = max(0, min(cur+m.lo.TeamGold*5, st/5*5))
	}
	if target == cur {
		return
	}
	m.lo.TeamGold -= (target - cur) / 5
	m.lo.Cargos[m.cargoSel] = target
	m.drawBars()
	m.updateBuyButton()
}

// updateBuyButton enables Buy only while the roster selection is affordable.
func (m *cargoMenu) updateBuyButton() {
	if m.buyBtn < 0 || m.buySel >= len(cargoRoster) {
		return
	}
	if cargoCanBuy(m.at, m.lo.TeamGold, cargoRoster[m.buySel].t) {
		m.ctx.Sys.EnableButton(m.buyBtn)
	} else {
		m.ctx.Sys.DisableButton(m.buyBtn)
	}
}

// updateDeleteButton enables Delete only for a removable Purchased selection.
func (m *cargoMenu) updateDeleteButton() {
	if m.deleteBtn < 0 {
		return
	}
	if m.lo.CanDelete(m.cargoSel) {
		m.ctx.Sys.EnableButton(m.deleteBtn)
	} else {
		m.ctx.Sys.DisableButton(m.deleteBtn)
	}
}

// clampScroll keeps both list selections inside their visible windows.
func (m *cargoMenu) clampScroll() {
	if m.buySel < m.buyScroll {
		m.buyScroll = m.buySel
	}
	if m.buySel >= m.buyScroll+m.buyRows {
		m.buyScroll = m.buySel - m.buyRows + 1
	}
	if m.cargoSel < m.cargoScroll {
		m.cargoScroll = m.cargoSel
	}
	if m.cargoSel >= m.cargoScroll+m.cargoRows {
		m.cargoScroll = m.cargoSel - m.cargoRows + 1
	}
}

// buySelected buys the roster selection into the loadout, spending gold.
func (m *cargoMenu) buySelected() {
	if m.buySel >= len(cargoRoster) {
		return
	}
	t := cargoRoster[m.buySel].t
	cost := cargoUnitCost(m.at, t)
	if !cargoCanBuy(m.at, m.lo.TeamGold, t) {
		return
	}
	m.lo.Units = append(m.lo.Units, t)
	m.lo.Cargos = append(m.lo.Cargos, 0)
	m.lo.TeamGold -= cost
	m.cargoSel = len(m.lo.Units) - 1
	m.clampScroll()
	m.drawCargoList()
	m.drawBars()
	m.updateBuyButton()
	m.updateDeleteButton()
}

// deleteSelected removes the Purchased selection, refunding its cost + cargo.
func (m *cargoMenu) deleteSelected() {
	// Per type minimum: never delete the last Constructor/Engineer/Surveyor.
	if !m.lo.CanDelete(m.cargoSel) {
		return
	}
	t := m.lo.Units[m.cargoSel]
	m.lo.TeamGold += cargoUnitCost(m.at, t) + m.lo.Cargos[m.cargoSel]/5
	m.lo.Units = append(m.lo.Units[:m.cargoSel], m.lo.Units[m.cargoSel+1:]...)
	m.lo.Cargos = append(m.lo.Cargos[:m.cargoSel], m.lo.Cargos[m.cargoSel+1:]...)
	if m.cargoSel >= len(m.lo.Units) && m.cargoSel > 0 {
		m.cargoSel--
	}
	m.clampScroll()
	m.drawCargoList()
	m.drawDetail()
	m.drawBars()
	m.updateBuyButton()
	m.updateDeleteButton()
}
