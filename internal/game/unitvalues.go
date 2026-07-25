// unitvalues.go : stores base and upgraded attributes for each unit type.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
// C++ ported : unitvalues.cpp - UnitValues per-type stat tables
package game

import "github.com/suglasp/max-go/internal/assets"

// Attrib indexes a unit's upgradeable attributes; the first eight coincide with the research
// topics (RESEARCH_TOPIC_*), so research and upgrades share the index. Values are wire stable.
//
// C++ ported : unitvalues.hpp - ATTRIB_* enum
type Attrib int

const (
	AttribAttack       Attrib = 0 // RESEARCH_TOPIC_ATTACK
	AttribRounds       Attrib = 1 // RESEARCH_TOPIC_SHOTS
	AttribRange        Attrib = 2 // RESEARCH_TOPIC_RANGE
	AttribArmor        Attrib = 3 // RESEARCH_TOPIC_ARMOR
	AttribHits         Attrib = 4 // RESEARCH_TOPIC_HITS
	AttribSpeed        Attrib = 5 // RESEARCH_TOPIC_SPEED
	AttribScan         Attrib = 6 // RESEARCH_TOPIC_SCAN
	AttribTurns        Attrib = 7 // RESEARCH_TOPIC_COST
	AttribAmmo         Attrib = 8
	AttribMoveAndFire  Attrib = 9
	AttribFuel         Attrib = 10
	AttribStorage      Attrib = 11
	AttribAttackRadius Attrib = 12
	AttribAgentAdjust  Attrib = 13
	AttribCount        Attrib = 14
)

// ResearchTopicCount is how many attributes are researchable (AttribAttack..AttribTurns).
//
// C++ ported : unitvalues.hpp - RESEARCH_TOPIC_COUNT
const ResearchTopicCount = 8

// UnitValues is one unit type's attributes for one team: base stats plus research and upgrade increments, with a version for improvements.
//
// C++ ported : unitvalues.cpp - UnitValues
type UnitValues struct {
	turns, hits, armor, attack        uint16
	speed, fuel, range_, rounds       uint16
	moveAndFire                       uint16
	scan, storage, ammo, attackRadius uint16
	agentAdjust                       uint16
	version                           uint16
	inUse                             bool
}

// NewUnitValues builds the base UnitValues for a unit type from its ATTRIBS row. Version starts at 1.
func NewUnitValues(a assets.UnitAttributes) *UnitValues {
	var moveAndFire uint16
	if a.MoveAndFire {
		moveAndFire = 1
	}
	return &UnitValues{
		turns:        uint16(a.TurnsToBuild),
		hits:         uint16(a.HitPoints),
		armor:        uint16(a.Armor),
		attack:       uint16(a.Attack),
		speed:        uint16(a.Speed),
		fuel:         uint16(a.Fuel),
		range_:       uint16(a.Range),
		rounds:       uint16(a.Rounds),
		moveAndFire:  moveAndFire,
		scan:         uint16(a.Scan),
		storage:      uint16(a.Storage),
		ammo:         uint16(a.Ammo),
		attackRadius: uint16(a.Area),
		version:      1,
	}
}

// addr returns a pointer to the attribute's storage; an unknown Attrib panics (a programmer error).
func (v *UnitValues) addr(a Attrib) *uint16 {
	switch a {
	case AttribAttack:
		return &v.attack
	case AttribRounds:
		return &v.rounds
	case AttribRange:
		return &v.range_
	case AttribArmor:
		return &v.armor
	case AttribHits:
		return &v.hits
	case AttribSpeed:
		return &v.speed
	case AttribScan:
		return &v.scan
	case AttribTurns:
		return &v.turns
	case AttribAmmo:
		return &v.ammo
	case AttribMoveAndFire:
		return &v.moveAndFire
	case AttribFuel:
		return &v.fuel
	case AttribStorage:
		return &v.storage
	case AttribAttackRadius:
		return &v.attackRadius
	case AttribAgentAdjust:
		return &v.agentAdjust
	default:
		panic("game: UnitValues attribute out of range")
	}
}

// Get returns the value of one attribute.
//
// C++ ported : unitvalues.cpp - UnitValues::GetAttribute
func (v *UnitValues) Get(a Attrib) int {
	return int(*v.addr(a))
}

// Set replaces one attribute.
//
// C++ ported : unitvalues.cpp - UnitValues::SetAttribute
func (v *UnitValues) Set(a Attrib, value int) {
	*v.addr(a) = uint16(value)
}

// Add increments one attribute by value (the upgrade primitive).
//
// C++ ported : unitvalues.cpp - UnitValues::AddAttribute
func (v *UnitValues) Add(a Attrib, value int) {
	*v.addr(a) = uint16(int(*v.addr(a)) + value)
}

// Version returns the upgrade version.
//
// C++ ported : unitvalues.cpp - UnitValues::GetVersion
func (v *UnitValues) Version() int {
	return int(v.version)
}

// SetVersion sets the upgrade version.
//
// C++ ported : unitvalues.cpp - UnitValues::SetVersion
func (v *UnitValues) SetVersion(value int) {
	v.version = uint16(value)
}

// UpdateVersion bumps the version, but only while the set is in use.
//
// C++ ported : unitvalues.cpp - UnitValues::UpdateVersion
func (v *UnitValues) UpdateVersion() {
	if v.inUse {
		v.version++
	}
}

// MarkInUse marks this slot as in use, enabling UpdateVersion.
//
// C++ ported : unitvalues.cpp - UnitValues::MarkAsInUse
func (v *UnitValues) MarkInUse() {
	v.inUse = true
}

// MarkNotInUse marks this slot as not in use.
//
// C++ ported : unitvalues.cpp - UnitValues::MarkAsNotInUse
func (v *UnitValues) MarkNotInUse() {
	v.inUse = false
}

// InUse reports whether the set is marked as belonging to a unit in service.
func (v *UnitValues) InUse() bool {
	return v.inUse
}

// Clone returns an independent copy (each in service unit captures the team values at its version).
func (v *UnitValues) Clone() *UnitValues {
	c := *v
	return &c
}

// Equal mirrors the original operator== and compares every attribute except fuel, plus version and in-use state.
//
// C++ ported : unitvalues.cpp - UnitValues::operator==
func (v *UnitValues) Equal(o *UnitValues) bool {
	return v.turns == o.turns && v.hits == o.hits && v.armor == o.armor &&
		v.attack == o.attack && v.speed == o.speed && v.range_ == o.range_ &&
		v.rounds == o.rounds && v.moveAndFire == o.moveAndFire && v.scan == o.scan &&
		v.storage == o.storage && v.ammo == o.ammo && v.attackRadius == o.attackRadius &&
		v.version == o.version && v.inUse == o.inUse && v.agentAdjust == o.agentAdjust
}
