// unitnames.go : readable display names for unit types.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package game

import "github.com/suglasp/max-go/internal/assets"

// unitDisplayNames gives readable names for unit types shown by the game screen and the menus. The tag is the fallback.
var unitDisplayNames = map[assets.UnitType]string{
	// Mobile land units.
	assets.Tank: "Tank", assets.Scout: "Scout Car", assets.Constrct: "Constructor",
	assets.Engineer: "Engineer", assets.Bulldozr: "Bulldozer",
	assets.Surveyor: "Surveyor", assets.Scanner: "Scanner",
	assets.MisslLch: "Missile Launcher", assets.Artillry: "Assault Gun",
	assets.RocktLch: "Rocket Launcher", assets.SpFlak: "SP Flak",
	assets.MineLayr: "Mine Layer", assets.SplyTrck: "Supply Truck",
	assets.GoldTrck: "Gold Truck", assets.FuelTrck: "Fuel Truck",
	assets.Repair: "Repair Unit", assets.ClnTrans: "Personnel Carrier",
	assets.Commando: "Commando", assets.Infantry: "Infantry",
	// Air units.
	assets.Awac: "AWAC", assets.Bomber: "Bomber", assets.Fighter: "Fighter",
	assets.AirTrans: "Air Transport",
	// Sea units.
	assets.FastBoat: "Escort", assets.Corvette: "Corvette",
	assets.Battlshp: "Battleship", assets.Submarne: "Submarine",
	assets.SeaTrans: "Sea Transport", assets.MsslBoat: "Missile Cruiser",
	assets.SeaMnLyr: "Sea Mine Layer", assets.CargoShp: "Cargo Ship",
	// Structures.
	assets.MiningSt: "Mining Station", assets.PowGen: "Power Generator",
	assets.PowerStn: "Power Station", assets.LandPlt: "Heavy Vehicle Plant",
	assets.LightPlt: "Light Vehicle Plant", assets.AirPlt: "Air Units Plant",
	assets.Shipyard: "Shipyard", assets.Radar: "Radar",
	assets.Barracks: "Barracks", assets.Depot: "Depot",
	assets.Habitat: "Habitat", assets.Research: "Research Centre",
	assets.ShieldGn: "Shield Generator", assets.GunTurrt: "Gun Turret",
	assets.AntiAir: "Anti Aircraft", assets.ArtyTrrt: "Artillery",
	assets.AntiMssl: "Missile Turret",
	assets.SuprtPlt: "Life Support", assets.Greenhse: "Eco-sphere",
	assets.RecCentr: "Recreation Center", assets.Hangar: "Hangar",
	assets.Dock: "Dock", assets.TrainHal: "Training Hall",
	assets.CommTwr: "Gold Refinery", assets.ADump: "Storage Unit",
	assets.FDump: "Fuel Tank", assets.GoldSm: "Gold Vault",
	assets.Cnct4W: "Connector", assets.WtrPltfm: "Water Platform",
	assets.Bridge: "Bridge", assets.Road: "Road",
	assets.LandPad: "Landing Pad", assets.Block: "Block",
	assets.LandMine: "Land Mine", assets.SeaMine: "Sea Mine",
}

// UnitDisplayName returns a readable name for a unit type, falling back to its resource tag for types
// not in the table.
func UnitDisplayName(t assets.UnitType) string {
	if n, ok := unitDisplayNames[t]; ok {
		return n
	}

	return t.Tag()
}
