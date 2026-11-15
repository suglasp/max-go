// upgradable.go : the candidate sets and attribute rows the upgrade lists are built from.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
//
// C++ ported : abstractupgrademenu.cpp - AbstractUpgradeMenu::IsBuildable
package game

import "github.com/suglasp/max-go/internal/assets"

// UpgradableBuildingTypes lists the stationary types a team can build and so upgrade: the Constructor and
// Engineer rosters in the manual order, then the two mines, which are upgradable because their deployer is
// buildable. The in game build roster (gamescreen buildableTypes) is the prefix without the mines.
//
// C++ ported : abstractupgrademenu.cpp - AbstractUpgradeMenu::IsBuildable (mines follow MINELAYR / SEAMNLYR)
func UpgradableBuildingTypes() []assets.UnitType {
	return []assets.UnitType{
		assets.MiningSt, assets.PowerStn, assets.LightPlt, assets.LandPlt, assets.AirPlt, assets.Shipyard,
		assets.CommTwr, assets.Depot, assets.Hangar, assets.Dock, assets.Habitat, assets.Research,
		assets.Greenhse, assets.TrainHal, assets.Barracks,
		assets.PowGen, assets.ADump, assets.FDump, assets.GoldSm, assets.Radar, assets.GunTurrt,
		assets.AntiAir, assets.ArtyTrrt, assets.AntiMssl, assets.Block, assets.Road, assets.WtrPltfm,
		assets.Bridge,
		assets.LandMine, assets.SeaMine,
	}
}

// UpgradableAttribs lists the upgradable attributes a unit type offers, in the menu row order (attack
// family, then armor, hits and scan, then speed), gating the attack family on a nonzero attack value.
//
// C++ ported : abstractupgrademenu.cpp - AddUpgradeMilitary / AddUpgradeGeneric / AddUpgradeMobile
func UpgradableAttribs(base *UnitValues) []Attrib {
	var a []Attrib

	if base.Get(AttribAttack) > 0 {
		a = append(a, AttribAttack)

		if base.Get(AttribRounds) > 0 {
			a = append(a, AttribRounds)
		}

		if base.Get(AttribRange) > 0 {
			a = append(a, AttribRange)
		}

		if base.Get(AttribAmmo) > 0 {
			a = append(a, AttribAmmo)
		}
	}

	if base.Get(AttribArmor) > 0 {
		a = append(a, AttribArmor)
	}

	if base.Get(AttribHits) > 0 {
		a = append(a, AttribHits)
	}

	if base.Get(AttribScan) > 0 {
		a = append(a, AttribScan)
	}

	if base.Get(AttribSpeed) > 0 {
		a = append(a, AttribSpeed)
	}

	return a
}
