// produced.go : assigns runtime flags to factory-built units.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
//
// C++ ported : production_manager.cpp - produced unit tracking
package game

import "github.com/suglasp/max-go/internal/assets"

// ProducedFlags returns the movement and rendering flags for a factory-built mobile unit. The simulation build queue and AI production share this mapping.
// Unit types here must match the AI factoryProducts lists in internal/ai/buildunits.go and the menu factory rosters.
//
// C++: a produced unit's flags come from its unit data row (the MOBILE / LAND /
// SEA / AIR class), stamped on creation (units_manager.cpp).
func ProducedFlags(t assets.UnitType) Flags {
	const (
		land = FlagMobileLandUnit | FlagSelectable
		air  = FlagMobileAirUnit | FlagHovering | FlagSelectable
		sea  = FlagMobileSeaUnit | FlagSelectable
	)
	switch t {
	// The Tank is the only produced mobile unit with an independently rotating head.
	case assets.Tank:
		return land | FlagTurretSprite | FlagCanHeadRotate
	case assets.Artillry, assets.RocktLch, assets.MisslLch, assets.SpFlak:
		return land
	// A mobile radar (Scanner) carries a spinning dish.
	case assets.Scanner:
		return land | FlagSpinningTurret
	// Mobile builders need the construction flag; galleryFlags omits it because static gallery units never build.
	case assets.Constrct, assets.Engineer:
		return land | FlagConstructorUnit
	// Other ground units: scouts, support trucks, mine layer, repair, bulldozer, the
	// personnel carrier, infantry.
	case assets.Scout, assets.Surveyor, assets.SplyTrck, assets.GoldTrck, assets.FuelTrck,
		assets.MineLayr, assets.Repair, assets.Bulldozr, assets.ClnTrans,
		assets.Infantry, assets.Commando:
		return land
	// Air: the AWAC's radar dish spins; the rest are plain flyers.
	case assets.Awac:
		return air | FlagSpinningTurret
	case assets.Bomber, assets.Fighter, assets.AirTrans:
		return air
	// Naval sprites cover the whole unit, so they have no separate turret flag.
	case assets.FastBoat, assets.Corvette, assets.Battlshp, assets.Submarne,
		assets.SeaTrans, assets.MsslBoat, assets.SeaMnLyr, assets.CargoShp:
		return sea
	default:
		return land
	}
}
