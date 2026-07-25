// unitflags.go : defines wire-stable unit capability and category bits.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
// C++ ported : enums.hpp - UnitFlags bit definitions
package game

// Flags are the per unit type capability/category bits (wire stable). They drive categorization,
// rendering and behaviour; a unit instance inherits its type's flags.
//
// C++ ported : enums.hpp - unit flag enum
type Flags uint32

// UnitClass groups units as land/sea, air, or stationary. Scripted mission objectives evaluate units by these families.
//
// C++ ported : the MOBILE_LAND_UNIT|MOBILE_SEA_UNIT / MOBILE_AIR_UNIT / STATIONARY unit
// flags, which the original reuses as its MAX_UNIT_CLASS script enum (enums.hpp)
type UnitClass int

const (
	ClassMobileLandSea UnitClass = iota
	ClassMobileAir
	ClassStationary
)

// InClass reports whether the unit belongs to the given family.
func (u *UnitInfo) InClass(c UnitClass) bool {
	switch c {
	case ClassMobileLandSea:
		return u.Flags.Has(FlagMobileLandUnit) || u.Flags.Has(FlagMobileSeaUnit)
	case ClassMobileAir:
		return u.Flags.Has(FlagMobileAirUnit)
	case ClassStationary:
		return u.Flags.Has(FlagStationary)
	}

	return false
}

const (
	FlagGroundCover      Flags = 0x00000001
	FlagExploding        Flags = 0x00000002
	FlagAnimated         Flags = 0x00000004
	FlagConnectorUnit    Flags = 0x00000008
	FlagBuilding         Flags = 0x00000010
	FlagMissileUnit      Flags = 0x00000020
	FlagMobileAirUnit    Flags = 0x00000040
	FlagMobileSeaUnit    Flags = 0x00000080
	FlagMobileLandUnit   Flags = 0x00000100
	FlagStationary       Flags = 0x00000200
	FlagHashTeamGray     Flags = 0x00000400
	FlagHashTeamBlue     Flags = 0x00000800
	FlagHashTeamGreen    Flags = 0x00001000
	FlagHashTeamRed      Flags = 0x00002000
	FlagUpgradable       Flags = 0x00004000
	FlagHashTeamDerelict Flags = 0x00008000
	FlagHovering         Flags = 0x00010000
	FlagHasFiringSprite  Flags = 0x00020000
	FlagFiresMissiles    Flags = 0x00040000
	FlagConstructorUnit  Flags = 0x00080000
	FlagCanHeadRotate    Flags = 0x00100000
	FlagElectronicUnit   Flags = 0x00200000
	FlagSelectable       Flags = 0x00400000
	FlagStandalone       Flags = 0x00800000
	FlagRequiresSlab     Flags = 0x01000000
	FlagTurretSprite     Flags = 0x02000000
	FlagSentryUnit       Flags = 0x04000000
	FlagSpinningTurret   Flags = 0x08000000
	FlagRegenerating     Flags = 0x10000000
)

// Has reports whether all of the given bits are set.
func (f Flags) Has(bits Flags) bool {
	return f&bits == bits
}
