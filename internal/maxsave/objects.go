// objects.go : the M.A.X. save object graph - shared object reader that reconstructs UnitInfo/UnitValues/Complex from the V70 save file stream.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
//
// C++ ported : unitvalues.cpp / complex.cpp / groundpath.cpp / airpath.cpp / builderpath.cpp / unitinfo.cpp - *::FileLoad
//
// The V70 unit graph uses six FileObject classes. Each FileLoad consumes the same bytes as its C++ counterpart while retaining only fields used by the port.

package maxsave

// Original unit flag bits (enums.hpp) used for the heuristic that picks the path type.
const (
	flagMobileAirUnit = 0x40
)

// UnitValues is a unit type's attribute set (per team base/current tables).
// C++ ported : unitvalues.cpp - UnitValues::FileLoad
type UnitValues struct {
	Turns, Hits, Armor, Attack, Speed, Range, Rounds uint16
	MoveAndFire                                      uint8
	Scan, Storage, Ammo, AttackRadius, AgentAdjust   uint16
	Version                                          uint16
	InUse                                            bool
}

func (v *UnitValues) FileLoad(r *Reader) {
	v.Turns, v.Hits, v.Armor, v.Attack = r.U16(), r.U16(), r.U16(), r.U16()
	v.Speed, v.Range, v.Rounds = r.U16(), r.U16(), r.U16()
	v.MoveAndFire = r.U8()
	v.Scan, v.Storage, v.Ammo, v.AttackRadius, v.AgentAdjust = r.U16(), r.U16(), r.U16(), r.U16(), r.U16()
	v.Version = r.U16()
	v.InUse = r.U8() != 0 // V70: legacy_units_built byte
}

func allocUnitValues() FileObject {
	return &UnitValues{}
}

// Complex is a building network's shared material/power/worker pool.
// C++ ported : complex.cpp - Complex::FileLoad
type Complex struct {
	Material, Fuel, Gold, Power, Workers, Buildings, ID int16
}

func (c *Complex) FileLoad(r *Reader) {
	c.Material, c.Fuel, c.Gold = r.I16(), r.I16(), r.I16()
	c.Power, c.Workers, c.Buildings, c.ID = r.I16(), r.I16(), r.I16(), r.I16()
}

func allocComplex() FileObject {
	return &Complex{}
}

// GroundPath consumes a land or sea unit's queued route without retaining its steps. C++ ported: groundpath.cpp, GroundPath::FileLoad.
type GroundPath struct{}

func (p *GroundPath) FileLoad(r *Reader) {
	r.Skip(2 + 2 + 2) // m_end_x i16, m_end_y i16, m_step_index u16 (V70)
	for n := r.Count(); n > 0; n-- {
		r.Skip(2) // PathStep{int8 x, int8 y}
	}
}

// AirPath is an air unit's straight line route (fixed size).
// C++ ported : airpath.cpp - AirPath::FileLoad
type AirPath struct{}

func (p *AirPath) FileLoad(r *Reader) {
	// m_length i16, m_angle i8, m_start_x i16, m_start_y i16, m_end_x i16,
	// m_end_y i16, m_step_x i32, m_step_y i32, m_delta_x i32, m_delta_y i32.
	r.Skip(2 + 1 + 2 + 2 + 2 + 2 + 4 + 4 + 4 + 4)
}

// BuilderPath is a constructor's build direction (fixed size).
// C++ ported : builderpath.cpp - BuilderPath::FileLoad
type BuilderPath struct{}

func (p *BuilderPath) FileLoad(r *Reader) { r.Skip(2 + 2) } // m_direction_x/y i16

// The stored class index of each of the six V70 FileObject classes. The original keeps
// its registered classes in an array SORTED BY CLASS NAME and writes each object's
// position in that array (1 based), so the indices are simply the class names in
// alphabetical order. Reading them means the file itself says which class follows,
// instead of the port guessing from the unit's flags.
//
// C++ ported : registerarray.hpp - RegisterArray::Insert (a SortedArray keyed by the
// class name, re-stamping every type index on each insert), read back by
// smartfile.cpp - SmartFileReader::ReadObject
const (
	typeAirPath     = 1
	typeBuilderPath = 2
	typeComplex     = 3
	typeGroundPath  = 4
	typeUnitInfo    = 5
	typeUnitValues  = 6
)

// classRegistry maps each stored class index to the reader for that class.
func classRegistry() map[uint32]Allocator {
	return map[uint32]Allocator{
		typeAirPath:     func() FileObject { return &AirPath{} },
		typeBuilderPath: func() FileObject { return &BuilderPath{} },
		typeComplex:     allocComplex,
		typeGroundPath:  func() FileObject { return &GroundPath{} },
		typeUnitInfo:    allocUnit,
		typeUnitValues:  allocUnitValues,
	}
}

// pathAllocFor is the FALLBACK reader for a unit's path when the stored class index is
// not one the registry knows: an air unit flies a straight AirPath, everything else
// walks a GroundPath. A builder's BuilderPath is a third layout entirely (4 bytes, not
// a step list), which is why guessing from the flags alone used to misread the record
// and drag every later unit in the file out of alignment.
func pathAllocFor(flags uint32) Allocator {
	if flags&flagMobileAirUnit != 0 {
		return func() FileObject { return &AirPath{} }
	}

	return func() FileObject { return &GroundPath{} }
}

// Unit retains the UnitInfo fields needed to restore a mission. FileLoad consumes the rest of the record to keep the stream aligned.
//
// C++ ported : unitinfo.cpp - UnitInfo::FileLoad (V70 path)
type Unit struct {
	Type    uint16
	ID      uint16
	Flags   uint32
	X, Y    uint16
	GridX   int16
	GridY   int16
	Name    string
	Team    uint8
	Angle   uint8
	Hits    uint8
	Speed   uint8
	Storage int16 // Current raw material, fuel, gold, or mines carried.
	Orders  uint8
	State   uint8
	// A mining station's scripted extraction: what it pulls per turn of each ore, and the
	// deposit caps under it. A mission AUTHORS these - they are not derivable from the
	// cargo map alone, because the split between the three ores is the allocation the
	// mission starts you on (the Allocate screen's sliders).
	TotalMining   uint8
	RawMining     uint8
	FuelMining    uint8
	GoldMining    uint8
	RawMiningMax  uint8
	GoldMiningMax uint8
	FuelMiningMax uint8
	RepeatBuild   uint8    // Factory repeat-build flag.
	BuildRate     uint16   // Factory production multiplier.
	BuildList     []uint16 // Queued ResourceIDs in production order.
	Complex       *Complex // Shared building-network pool, or nil.
}

func (u *Unit) FileLoad(r *Reader) {
	u.Type = r.U16()
	u.ID = r.U16()
	u.Flags = r.U32()
	u.X = r.U16()
	u.Y = r.U16()
	u.GridX = r.I16()
	u.GridY = r.I16()
	if n := r.U16(); n > 0 { // name: u16 length + bytes
		u.Name = string(r.Bytes(int(n)))
	}
	r.Skip(4) // shadow_offset (Point i16,i16)
	u.Team = r.U8()
	r.Skip(2) // unit_id, brightness
	u.Angle = r.U8()
	r.Skip(5 + 5)   // visible_to_team[5], spotted_by_team[5]
	r.Skip(4)       // max_velocity, velocity, sound, scaler_adjust
	r.Skip(16 + 16) // sprite_bounds, shadow_bounds (Rect = 4*i32)
	r.Skip(3)       // turret_angle, turret_offset_x, turret_offset_y
	r.Skip(8 * 2)   // total_images..image_index_max (8 * i16)
	u.Orders = r.U8()
	u.State = r.U8()
	r.Skip(3)     // prior_orders, prior_state, laying_state
	r.Skip(2 + 2) // V70: target_grid_x, target_grid_y (i16)
	r.Skip(1)     // build_time (u8)
	// The mining station's extraction block. MIND THE ORDER: the two LAST caps are gold
	// then fuel, NOT fuel then gold like the three rates above them. These eight bytes used
	// to be skipped wholesale, which landed every mission's mining station with a zeroed
	// allocation AND zeroed deposit caps - so a training mission opened mining nothing.
	u.TotalMining = r.U8()
	u.RawMining = r.U8()
	u.FuelMining = r.U8()
	u.GoldMining = r.U8()
	u.RawMiningMax = r.U8()
	u.GoldMiningMax = r.U8()
	u.FuelMiningMax = r.U8()
	u.Hits = r.U8()     // V70: hits is a byte
	u.Speed = r.U8()    // V70: speed is a byte
	r.Skip(2)           // shots, move_and_fire
	u.Storage = r.I16() // current cargo the unit carries
	// V70: experience/transfer_cargo/stealth_dice_roll are derived, not read.
	r.Skip(4)              // ammo, targeting_mode, enter_mode, cursor
	r.Skip(1)              // V70: recoil_delay (i8)
	r.Skip(2)              // delayed_reaction, damaged_this_turn
	r.Skip(2)              // research_topic, moved
	r.Skip(4)              // bobbed, shake_effect_state, engine, weapon
	r.Skip(4)              // V70: comm, fuel_distance, move_fraction, energized
	u.RepeatBuild = r.U8() // factory repeat build flag
	u.BuildRate = r.U16()  // factory build rate multiplier (0 is clamped up to 1)
	r.Skip(2)              // disabled_reaction_fire, auto_survey (bool)
	r.Skip(4)              // ai_state_bits (u32)

	r.ReadObjectTyped(pathAllocFor(u.Flags))               // path: ground, air OR a builder's
	r.Skip(2)                                              // connectors (u16)
	r.ReadObjectAs(allocUnitValues)                        // base_values
	u.Complex, _ = r.ReadObjectAs(allocComplex).(*Complex) // complex (shared pool)
	r.ReadObjectAs(allocUnit)                              // parent
	r.ReadObjectAs(allocUnit)                              // enemy_unit

	for n := r.Count(); n > 0; n-- { // build_list: ResourceID (u16) per queued unit
		u.BuildList = append(u.BuildList, r.U16())
	}
}

func allocUnit() FileObject {
	return &Unit{}
}
