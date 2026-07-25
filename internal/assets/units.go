// units.go : defines the M.A.X. unit roster and resource tags.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package assets

// UnitType indexes the game's unit roster.
// The order is fixed by the original MANDER.H and shared by the ATTRIBS table,
// the ResourceID enum and the save formats.
//
// C++: ResourceID UNIT_START..UNIT_END (resourcetable.hpp)
type UnitType int

// UnitCount is the number of unit types (COMMTWR .. WALDO).
const UnitCount = 93

// Unit type constants, in canonical order.
const (
	CommTwr UnitType = iota
	PowerStn
	PowGen
	Barracks
	ShieldGn
	Radar
	ADump
	FDump
	GoldSm
	Depot
	Hangar
	Dock
	Cnct4W
	LrgRuble
	SmlRuble
	LrgTape
	SmlTape
	LrgSlab
	SmlSlab
	LrgCones
	SmlCones
	Road
	LandPad
	Shipyard
	LightPlt
	LandPlt
	SuprtPlt
	AirPlt
	Habitat
	Research
	Greenhse
	RecCentr
	TrainHal
	WtrPltfm
	GunTurrt
	AntiAir
	ArtyTrrt
	AntiMssl
	Block
	Bridge
	MiningSt
	LandMine
	SeaMine
	LndExpld
	AirExpld
	SeaExpld
	BldExpld
	HitExpld
	Master
	Constrct
	Scout
	Tank
	Artillry
	RocktLch
	MisslLch
	SpFlak
	MineLayr
	Surveyor
	Scanner
	SplyTrck
	GoldTrck
	Engineer
	Bulldozr
	Repair
	FuelTrck
	ClnTrans
	Commando
	Infantry
	FastBoat
	Corvette
	Battlshp
	Submarne
	SeaTrans
	MsslBoat
	SeaMnLyr
	CargoShp
	Fighter
	Bomber
	AirTrans
	Awac
	Juggrnt
	AlnTank
	AlnAsGun
	AlnPlane
	Rocket
	Torpedo
	AlnMissl
	AlnTBall
	AlnABall
	RktSmoke
	TrpBuble
	Harvster
	Waldo
)

// UnitTags are the resource tag names of the unit types, in canonical
// order (also their identifier in scripts and debug output).
var UnitTags = [UnitCount]string{
	"COMMTWR", "POWERSTN", "POWGEN", "BARRACKS", "SHIELDGN", "RADAR",
	"ADUMP", "FDUMP", "GOLDSM", "DEPOT", "HANGAR", "DOCK", "CNCT_4W",
	"LRGRUBLE", "SMLRUBLE", "LRGTAPE", "SMLTAPE", "LRGSLAB", "SMLSLAB",
	"LRGCONES", "SMLCONES", "ROAD", "LANDPAD", "SHIPYARD", "LIGHTPLT",
	"LANDPLT", "SUPRTPLT", "AIRPLT", "HABITAT", "RESEARCH", "GREENHSE",
	"RECCENTR", "TRAINHAL", "WTRPLTFM", "GUNTURRT", "ANTIAIR", "ARTYTRRT",
	"ANTIMSSL", "BLOCK", "BRIDGE", "MININGST", "LANDMINE", "SEAMINE",
	"LNDEXPLD", "AIREXPLD", "SEAEXPLD", "BLDEXPLD", "HITEXPLD", "MASTER",
	"CONSTRCT", "SCOUT", "TANK", "ARTILLRY", "ROCKTLCH", "MISSLLCH",
	"SP_FLAK", "MINELAYR", "SURVEYOR", "SCANNER", "SPLYTRCK", "GOLDTRCK",
	"ENGINEER", "BULLDOZR", "REPAIR", "FUELTRCK", "CLNTRANS", "COMMANDO",
	"INFANTRY", "FASTBOAT", "CORVETTE", "BATTLSHP", "SUBMARNE", "SEATRANS",
	"MSSLBOAT", "SEAMNLYR", "CARGOSHP", "FIGHTER", "BOMBER", "AIRTRANS",
	"AWAC", "JUGGRNT", "ALNTANK", "ALNASGUN", "ALNPLANE", "ROCKET",
	"TORPEDO", "ALNMISSL", "ALNTBALL", "ALNABALL", "RKTSMOKE", "TRPBUBLE",
	"HARVSTER", "WALDO",
}

// Tag returns the unit's resource tag name.
func (u UnitType) Tag() string {
	if u < 0 || int(u) >= UnitCount {
		return "INVALID"
	}

	return UnitTags[u]
}
