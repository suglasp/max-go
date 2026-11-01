// sound.go : the M.A.X. unit sound triggers - maps unit events (select, move, attack, death) to audio resource tags.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
//
// C++ ported : unitinfo.cpp - unit sound trigger logic
package units

// sound.go: per unit SFX tag table reconstructed from the resource tag suffix
// convention (1 idle, 2 idle water, 5/6 drive, 7/8 stop, 9 transform, 10 build,
// 13 turret, 14 fire, 15 hit, 16 explode, 17 power start, 18 power end, 19 land,
// 20 take) and the WAV files in the game data dir. Generic GEN_*/WGN_* defaults
// fill any (unit, sfx) pair with no override.
//
// C++ ported : units.cpp ParseSoundEffects (SC_UNITS JSON), sound_manager.cpp SoundManager::PlaySfx.

import (
	"github.com/suglasp/max-go/internal/assets"
	"github.com/suglasp/max-go/internal/game"
)

// sfxDefaults maps each SfxType to its generic GEN_*/WGN_* fallback tag, used
// when a unit has no clip of its own.
//
// C++ ported : units.cpp - default_sound_effects block in SC_UNITS JSON.
var sfxDefaults = map[game.SfxType]string{
	game.SfxIdle:       "GEN_IDLE",
	game.SfxWaterIdle:  "WGN_IDLE",
	game.SfxDrive:      "GEN_DRVE",
	game.SfxWaterDrive: "WGN_DRVE",
	game.SfxStop:       "GEN_STOP",
	game.SfxWaterStop:  "WGN_STOP",
	game.SfxTransform:  "GEN_XFRM",
	game.SfxBuilding:   "GEN_BLDG",
	game.SfxShrink:     "GEN_SHNK",
	game.SfxExpand:     "GEN_XPND",
	game.SfxTurret:     "GEN_TRRT",
	game.SfxFire:       "GEN_FIRE",
	game.SfxHit:        "GEN_HIT",
	game.SfxExpload:    "GEN_XPLD",
	game.SfxPowerStart: "MASTR17", // the general building running/power hum (MASTR17.WAV)
	game.SfxPowerEnd:   "GEN_PRCE",
	game.SfxLand:       "GEN_LAND",
	game.SfxTake:       "GEN_TAKE",
}

// unitSfx maps (unit type, SFX kind) to a resource tag; omitted entries fall
// through to sfxDefaults. Tag suffix encodes the SFX kind (see file header).
//
// C++ ported : units.cpp - unit_sound_effects entries in SC_UNITS JSON.
var unitSfx = map[assets.UnitType]map[game.SfxType]string{
	// --- Stationary buildings --------------------------------------------------

	assets.CommTwr: {
		game.SfxPowerStart: "MONOP17",
		game.SfxPowerEnd:   "MONOP18",
	},
	assets.PowerStn: {
		game.SfxPowerStart: "POWST17",
		game.SfxPowerEnd:   "POWST18",
	},
	assets.PowGen: {
		game.SfxPowerStart: "POWGN17",
		game.SfxPowerEnd:   "POWGN18",
	},
	assets.Barracks: {
		game.SfxPowerStart: "BARRA17",
		game.SfxPowerEnd:   "BARRA18",
	},
	assets.GoldSm: {
		// Gold vault: a passive store like the material/fuel stores - silent on select
		// (no select hum). The GOLDR17/18 power clips belong to the gold refinery sound
		// set, not a vault's hum; map only its hit/explode combat sounds here. The
		// original silences a building whose power sound is the generic default (sound_manager.cpp).
		game.SfxHit:     "GOLDR15",
		game.SfxExpload: "GOLDR16",
	},
	assets.Radar: {
		game.SfxIdle:       "RADAR13", // the dish rotate hum (suffix 13 = turret)
		game.SfxPowerStart: "RADAR13", // running hum on select: reuse the dish rotate clip (no RADAR17 ships)
		game.SfxExpload:    "RADAR16", // no own power end clip ships (only 13/16)
	},
	assets.ADump: {
		// Material storage: only the hit/explode clips ship (no power clip), so it is
		// silent on select; SSTOR15/16 are the suffix 15 (hit) / suffix 16 (explode)
		// sounds - mapping them to SfxPowerStart looped the HIT sound when selected.
		game.SfxHit:     "SSTOR15",
		game.SfxExpload: "SSTOR16",
	},
	assets.FDump: {
		game.SfxHit:     "SFUEL15",
		game.SfxExpload: "SFUEL16",
	},
	assets.Depot: {
		game.SfxHit:     "DEPOT15", // no power clip ships (only 15/16)
		game.SfxExpload: "DEPOT16",
	},
	assets.Hangar: {
		game.SfxPowerStart: "HANGR17",
		game.SfxPowerEnd:   "HANGR18",
	},
	assets.Dock: {
		game.SfxPowerStart: "DOCK17",
		game.SfxPowerEnd:   "DOCK18",
	},
	assets.LandPad: {
		game.SfxHit:     "LPAD15", // no power clip ships (only 15/16)
		game.SfxExpload: "LPAD16",
	},
	assets.LightPlt: {
		game.SfxPowerStart: "LVP17",
		game.SfxPowerEnd:   "LVP18",
	},
	assets.LandPlt: {
		game.SfxHit:        "HVP15",
		game.SfxExpload:    "HVP16",
		game.SfxPowerStart: "HVP17", // Heavy Vehicle Plant has its own running hum (HVP17.WAV)
		game.SfxPowerEnd:   "HVP18",
	},
	assets.AirPlt: {
		game.SfxPowerStart: "AUNIT17",
		game.SfxPowerEnd:   "AUNIT18",
	},
	assets.Habitat: {
		game.SfxHit:        "DORMI15", // hit/explode combat clips
		game.SfxExpload:    "DORMI16",
		game.SfxPowerStart: "DORMI17", // running hum on select (no DORMI17 ships -> falls back to the general MASTR17)
	},
	assets.Research: {
		game.SfxPowerStart: "RESEAR17",
		game.SfxPowerEnd:   "RESEAR18",
	},
	assets.Greenhse: {
		game.SfxHit:        "ECOSP15",
		game.SfxExpload:    "ECOSP16",
		game.SfxPowerStart: "HVP17", // Eco Sphere (colony building) shares the heavy plant running hum HVP17 - confirmed against vanilla
		game.SfxPowerEnd:   "HVP18",
	},
	assets.TrainHal: {
		game.SfxPowerStart: "TRAIN17",
		game.SfxPowerEnd:   "TRAIN18",
	},
	assets.MiningSt: {
		game.SfxPowerStart: "MSTAT17",
		game.SfxPowerEnd:   "MSTAT18",
	},
	assets.Block: {
		game.SfxExpload: "BLOCK16", // only the explode clip ships
	},
	assets.Bridge: {
		game.SfxExpload: "BRIDG16", // only the explode clip ships
	},

	// Fixed gun turrets (fire only; turret / idle use generics).
	assets.GunTurrt: {
		game.SfxFire: "FGUN14",
	},
	assets.AntiAir: {
		game.SfxFire: "FANTI14",
	},
	assets.ArtyTrrt: {
		game.SfxFire: "FARTY14",
	},
	assets.AntiMssl: {
		game.SfxFire: "FROCK14",
	},

	// --- Mobile land units -----------------------------------------------------

	assets.Master: {
		game.SfxIdle:      "MASTR1",
		game.SfxDrive:     "MASTR5",
		game.SfxStop:      "MASTR7",
		game.SfxTransform: "MASTR9",
		game.SfxHit:       "MASTR15",
		game.SfxExpload:   "MASTR16",
	},
	assets.Constrct: {
		game.SfxIdle:       "CONST1",
		game.SfxWaterIdle:  "CONST2",
		game.SfxDrive:      "CONST5",
		game.SfxWaterDrive: "CONST6",
		game.SfxStop:       "CONST7",
		game.SfxWaterStop:  "CONST8",
		game.SfxBuilding:   "CONST10",
		game.SfxHit:        "CONST15",
		game.SfxExpload:    "CONST16",
	},
	assets.Scout: {
		game.SfxIdle:       "SCOUT1",
		game.SfxWaterIdle:  "SCOUT2",
		game.SfxDrive:      "SCOUT5",
		game.SfxWaterDrive: "SCOUT6",
		game.SfxStop:       "SCOUT7",
		game.SfxWaterStop:  "SCOUT8",
		game.SfxFire:       "SCOUT14",
		game.SfxHit:        "SCOUT15",
		game.SfxExpload:    "SCOUT16",
	},
	assets.Tank: {
		game.SfxIdle:    "TANK1",
		game.SfxDrive:   "TANK5",
		game.SfxStop:    "TANK7",
		game.SfxFire:    "TANK14",
		game.SfxHit:     "TANK15",
		game.SfxExpload: "TANK16",
	},
	assets.Artillry: {
		game.SfxIdle:    "ASGUN1",
		game.SfxDrive:   "ASGUN5",
		game.SfxStop:    "ASGUN7",
		game.SfxFire:    "ASGUN14",
		game.SfxHit:     "ASGUN15",
		game.SfxExpload: "ASGUN16",
	},
	assets.RocktLch: {
		game.SfxIdle:    "RLNCH1",
		game.SfxDrive:   "RLNCH5",
		game.SfxStop:    "RLNCH7",
		game.SfxFire:    "RLNCH14",
		game.SfxHit:     "RLNCH15",
		game.SfxExpload: "RLNCH16",
	},
	assets.MisslLch: {
		game.SfxIdle:    "MSLNC1",
		game.SfxDrive:   "MSLNC5",
		game.SfxStop:    "MSLNC7",
		game.SfxFire:    "MSLNC14",
		game.SfxHit:     "MSLNC15",
		game.SfxExpload: "MSLNC16",
	},
	assets.SpFlak: {
		game.SfxIdle:    "MANTI1",
		game.SfxDrive:   "MANTI5",
		game.SfxStop:    "MANTI7",
		game.SfxFire:    "MANTI14",
		game.SfxHit:     "MANTI15",
		game.SfxExpload: "MANTI16",
	},
	assets.MineLayr: {
		game.SfxIdle:       "MLAYER1",
		game.SfxDrive:      "MLAYER5",
		game.SfxStop:       "MLAYER7",
		game.SfxHit:        "MLAYER15",
		game.SfxExpload:    "MLAYER16",
		game.SfxPowerStart: "MLAYER17",
		game.SfxPowerEnd:   "MLAYER18",
	},
	assets.Surveyor: {
		game.SfxIdle:       "SURVY1",
		game.SfxWaterIdle:  "SURVY2",
		game.SfxDrive:      "SURVY5",
		game.SfxWaterDrive: "SURVY6",
		game.SfxStop:       "SURVY7",
		game.SfxWaterStop:  "SURVY8",
		game.SfxHit:        "SURVY15",
		game.SfxExpload:    "SURVY16",
	},
	assets.Scanner: {
		game.SfxIdle:    "SCAN1",
		game.SfxDrive:   "SCAN5",
		game.SfxStop:    "SCAN7",
		game.SfxHit:     "SCAN15",
		game.SfxExpload: "SCAN16",
	},
	assets.SplyTrck: {
		game.SfxIdle:       "MTRUK1",
		game.SfxDrive:      "MTRUK5",
		game.SfxStop:       "MTRUK7",
		game.SfxHit:        "MTRUK15",
		game.SfxExpload:    "MTRUK16",
		game.SfxPowerStart: "MTRUK17",
	},
	assets.GoldTrck: {
		game.SfxIdle:       "GTRUK1",
		game.SfxDrive:      "GTRUK5",
		game.SfxStop:       "GTRUK7",
		game.SfxHit:        "GTRUK15",
		game.SfxExpload:    "GTRUK16",
		game.SfxPowerStart: "GTRUK17",
	},
	assets.Engineer: {
		game.SfxIdle:       "ENGIN1",
		game.SfxWaterIdle:  "ENGIN2",
		game.SfxDrive:      "ENGIN5",
		game.SfxWaterDrive: "ENGIN6",
		game.SfxStop:       "ENGIN7",
		game.SfxWaterStop:  "ENGIN8",
		game.SfxBuilding:   "ENGIN10",
		game.SfxHit:        "ENGIN15",
		game.SfxExpload:    "ENGIN16",
	},
	assets.Bulldozr: {
		game.SfxIdle:     "BULL1",
		game.SfxDrive:    "BULL5",
		game.SfxStop:     "BULL7",
		game.SfxBuilding: "BULL10",
		game.SfxHit:      "BULL15",
		game.SfxExpload:  "BULL16",
	},
	assets.Repair: {
		game.SfxIdle:       "REPAIR1",
		game.SfxDrive:      "REPAIR5",
		game.SfxStop:       "REPAIR7",
		game.SfxHit:        "REPAIR15",
		game.SfxExpload:    "REPAIR16",
		game.SfxPowerStart: "REPAIR17",
	},
	assets.FuelTrck: {
		game.SfxIdle:       "FTRUK1",
		game.SfxDrive:      "FTRUK5",
		game.SfxStop:       "FTRUK7",
		game.SfxHit:        "FTRUK15",
		game.SfxExpload:    "FTRUK16",
		game.SfxPowerStart: "FTRUK17",
	},
	assets.ClnTrans: {
		game.SfxIdle:       "APC1",
		game.SfxWaterIdle:  "APC2",
		game.SfxDrive:      "APC5",
		game.SfxWaterDrive: "APC6",
		game.SfxStop:       "APC7",
		game.SfxWaterStop:  "APC8",
		game.SfxHit:        "APC15",
		game.SfxExpload:    "APC16",
		game.SfxPowerStart: "APC17",
		game.SfxPowerEnd:   "APC18",
	},
	assets.Commando: {
		game.SfxDrive:      "INFIL5",
		game.SfxFire:       "INFIL14",
		game.SfxHit:        "INFIL15",
		game.SfxExpload:    "INFIL16",
		game.SfxPowerStart: "INFIL17",
	},
	assets.Infantry: {
		game.SfxDrive:   "INFAN5",
		game.SfxFire:    "INFAN14",
		game.SfxHit:     "INFAN15",
		game.SfxExpload: "INFAN16",
	},

	// --- Alien land units ------------------------------------------------------

	assets.AlnTank: {
		game.SfxIdle:    "ALNTK1",
		game.SfxDrive:   "ALNTK5",
		game.SfxStop:    "ALNTK7",
		game.SfxFire:    "ALNTK14",
		game.SfxHit:     "ALNTK15",
		game.SfxExpload: "ALNTK16",
	},
	assets.AlnAsGun: {
		game.SfxIdle:    "ALNAG1",
		game.SfxDrive:   "ALNAG5",
		game.SfxStop:    "ALNAG7",
		game.SfxFire:    "ALNAG14",
		game.SfxHit:     "ALNAG15",
		game.SfxExpload: "ALNAG16",
	},

	// --- Naval units -----------------------------------------------------------

	assets.FastBoat: {
		game.SfxIdle:    "ESCRT2",
		game.SfxDrive:   "ESCRT6",
		game.SfxStop:    "ESCRT8",
		game.SfxFire:    "ESCRT14",
		game.SfxHit:     "ESCRT15",
		game.SfxExpload: "ESCRT16",
	},
	assets.Corvette: {
		game.SfxIdle:    "CORVT2",
		game.SfxDrive:   "CORVT6",
		game.SfxStop:    "CORVT8",
		game.SfxFire:    "CORVT14",
		game.SfxHit:     "CORVT15",
		game.SfxExpload: "CORVT16",
	},
	assets.Battlshp: {
		game.SfxIdle:    "GUNBT2",
		game.SfxDrive:   "GUNBT6",
		game.SfxStop:    "GUNBT8",
		game.SfxFire:    "GUNBT14",
		game.SfxHit:     "GUNBT15",
		game.SfxExpload: "GUNBT16",
	},
	assets.Submarne: {
		game.SfxIdle:    "SUB2",
		game.SfxDrive:   "SUB6",
		game.SfxStop:    "SUB8",
		game.SfxFire:    "SUB14",
		game.SfxHit:     "SUB15",
		game.SfxExpload: "SUB16",
	},
	assets.SeaTrans: {
		game.SfxIdle:       "STRANS2",
		game.SfxDrive:      "STRANS6",
		game.SfxStop:       "STRANS8",
		game.SfxHit:        "STRANS15",
		game.SfxExpload:    "STRANS16",
		game.SfxPowerStart: "STRANS17",
		game.SfxPowerEnd:   "STRANS18",
	},
	assets.MsslBoat: {
		game.SfxIdle:    "MSLCR2",
		game.SfxDrive:   "MSLCR6",
		game.SfxStop:    "MSLCR8",
		game.SfxFire:    "MSLCR14",
		game.SfxHit:     "MSLCR15",
		game.SfxExpload: "MSLCR16",
	},
	assets.SeaMnLyr: {
		game.SfxIdle:       "SMINE2",
		game.SfxDrive:      "SMINE6",
		game.SfxStop:       "SMINE8",
		game.SfxHit:        "SMINE15",
		game.SfxExpload:    "SMINE16",
		game.SfxPowerStart: "SMINE17",
		game.SfxPowerEnd:   "SMINE18",
	},
	assets.CargoShp: {
		game.SfxIdle:       "CSHIP2",
		game.SfxDrive:      "CSHIP6",
		game.SfxStop:       "CSHIP8",
		game.SfxHit:        "CSHIP15",
		game.SfxExpload:    "CSHIP16",
		game.SfxPowerStart: "CSHIP17",
	},

	// --- Air units -------------------------------------------------------------

	assets.Fighter: {
		game.SfxIdle:    "FIGHT1",
		game.SfxDrive:   "FIGHT5",
		game.SfxStop:    "FIGHT7",
		game.SfxFire:    "FIGHT14",
		game.SfxHit:     "FIGHT15",
		game.SfxExpload: "FIGHT16",
		game.SfxLand:    "FIGHT19",
		game.SfxTake:    "FIGHT20",
	},
	assets.Bomber: {
		game.SfxIdle:    "ATACK1",
		game.SfxDrive:   "ATACK5",
		game.SfxStop:    "ATACK7",
		game.SfxFire:    "ATACK14",
		game.SfxHit:     "ATACK15",
		game.SfxExpload: "ATACK16",
		game.SfxLand:    "ATACK19",
		game.SfxTake:    "ATACK20",
	},
	assets.AirTrans: {
		game.SfxIdle:       "ATRANS1",
		game.SfxDrive:      "ATRANS5",
		game.SfxStop:       "ATRANS7",
		game.SfxHit:        "ATRANS15",
		game.SfxExpload:    "ATRANS16",
		game.SfxPowerStart: "ATRANS17",
		game.SfxPowerEnd:   "ATRANS18",
		game.SfxLand:       "ATRANS19",
		game.SfxTake:       "ATRANS20",
	},
	assets.Awac: {
		game.SfxIdle:    "AWAC1",
		game.SfxDrive:   "AWAC5",
		game.SfxStop:    "AWAC7",
		game.SfxFire:    "AWAC14",
		game.SfxHit:     "AWAC15",
		game.SfxExpload: "AWAC16",
		game.SfxLand:    "AWAC19",
		game.SfxTake:    "AWAC20",
	},
	assets.AlnPlane: {
		game.SfxIdle:    "ALNPL1",
		game.SfxDrive:   "ALNPL5",
		game.SfxStop:    "ALNPL7",
		game.SfxFire:    "ALNPL14",
		game.SfxHit:     "ALNPL15",
		game.SfxExpload: "ALNPL16",
		game.SfxLand:    "ALNPL19",
		game.SfxTake:    "ALNPL20",
	},

	// --- Juggernauts -----------------------------------------------------------

	assets.Juggrnt: {
		game.SfxIdle:    "JUGGR1",
		game.SfxDrive:   "JUGGR5",
		game.SfxStop:    "JUGGR7",
		game.SfxFire:    "JUGGR14",
		game.SfxHit:     "JUGGR15",
		game.SfxExpload: "JUGGR16",
	},
}

// SfxTagFor returns the resource tag for (unit type, SFX kind), preferring the
// unit table then the generic default. Empty when neither is set.
//
// C++ ported : sound_manager.cpp/unit.cpp - SoundManager::PlaySfx, base_unit.GetSoundEffect.
func SfxTagFor(ut assets.UnitType, sfx game.SfxType) string {
	if ut >= 0 && int(ut) < assets.UnitCount {
		if m, ok := unitSfx[ut]; ok {
			if tag, ok2 := m[sfx]; ok2 {
				return tag
			}
		}
	}
	if def, ok := sfxDefaults[sfx]; ok {
		return def
	}
	return ""
}

// HasWaterMovementSfx reports whether a unit defines its OWN water variant move
// clips (e.g. SURVY2/6/8): the amphibious types plus the Surveyor. A pure sea
// unit returns false since its drive clip already IS its water clip under the
// plain keys and must not be remapped. Checks the unit table only, not WGN_*.
//
// C++ ported : sound_manager.cpp - SoundManager::PlaySfx swaps IDLE/DRIVE/STOP afloat.
func HasWaterMovementSfx(ut assets.UnitType) bool {
	if ut >= 0 && int(ut) < assets.UnitCount {
		if m, ok := unitSfx[ut]; ok {
			_, ok = m[game.SfxWaterDrive]
			return ok
		}
	}
	return false
}

/* // note: ported but unused code in Go port
// HasPowerSfx reports whether a building defines its OWN power start clip (the
// looping running hum). Passive stores/pads ship none and return false so they
// stay silent on select rather than looping a generic or hit clip. Unit table
// only, never the GEN_PRCS fallback.
//
// C++ ported : sound_manager.cpp - SoundManager::PlaySfx SFX_TYPE_POWER_CONSUMPTION_START.
func HasPowerSfx(ut assets.UnitType) bool {
	if ut >= 0 && int(ut) < assets.UnitCount {
		if m, ok := unitSfx[ut]; ok {
			_, ok = m[game.SfxPowerStart]
			return ok
		}
	}
	return false
}
*/

// UsesWaterMovementSfxSlot reports whether a unit on the given surface plays its
// water variant move clip (SfxWater* slot) rather than the plain land clip. True
// only for a water capable land unit afloat on open water.
//
// C++ ported : sound_manager.cpp - SoundManager::PlaySfx remaps DRIVE/IDLE/STOP afloat.
func UsesWaterMovementSfxSlot(ut assets.UnitType, onWater bool) bool {
	return HasWaterMovementSfx(ut) && onWater
}

// SfxDefaultTag returns just the generic GEN_*/WGN_* fallback for an SFX kind
// ("" when none). The player falls back here when a unit specific tag names a
// clip absent from the loaded archive, matching the C++ file_exists check.
//
// C++ ported : units.cpp ParseSoundEffects - default_sound_effects block.
func SfxDefaultTag(sfx game.SfxType) string {
	return sfxDefaults[sfx]
}

/* // note: unused code in Go port
// SfxPan maps a unit's on screen pixel X in a viewW wide map view to a stereo
// pan in [-1, 1] (0 center). Replaced by SfxPosition (the grid model) in step 1446.
func SfxPan(unitScreenX, viewW int) float64 {
	if viewW <= 0 {
		return 0
	}
	// Normalise: 0 = far left, 0.5 = centre, 1 = far right; then shift to [-1, 1].
	pan := 2.0*float64(unitScreenX)/float64(viewW) - 1.0
	if pan < -1 {
		pan = -1
	}
	if pan > 1 {
		pan = 1
	}
	return pan
}
*/

// Positional sound constants, in grid cells (sound_manager.cpp).
const (
	sfxPanRange         = 28  // offset from the view centre at which the pan stops widening
	sfxPanDivisor       = 56  // pan = offset / 56, so at most +/-0.5
	sfxAttenuationRange = 112 // SOUND_MANAGER_ATTENUATION_RANGE: silent this far from the view centre
	sfxZoomFull         = 32  // at or above this zoom (px per cell) there is no zoom fade
	sfxZoomMin          = 4   // the smallest zoom, where the zoom fade halves the volume
)

// SfxPosition returns the stereo pan (-0.5 left .. +0.5 right) and the volume factor (0..1) of a sound whose
// unit sits dx, dy cells from the view centre at the given zoom (px per cell). The pan widens up to 28 cells
// off centre; the volume fades over a fixed 112 cells and drops to half when zoomed all the way out.
// reversed swaps left and right (settings channels_reversed).
//
// C++ ported : sound_manager.cpp - SoundManager::GetPanning / GetDistanceAttenuation / GetZoomAttenuation
func SfxPosition(dx, dy, zoom int, reversed bool) (pan, volScale float64) {
	distX, distY := dx, dy
	if distX < 0 {
		distX = -distX
	}
	if distY < 0 {
		distY = -distY
	}

	panDist := min(distX, sfxPanRange)
	if dx < 0 {
		panDist = -panDist
	}
	pan = float64(panDist) / sfxPanDivisor
	if reversed {
		pan = -pan
	}

	dist := min(max(distX, distY), sfxAttenuationRange)
	volScale = 1 - float64(dist)/sfxAttenuationRange

	if zoom < sfxZoomFull {
		zoom = max(zoom, sfxZoomMin)
		volScale *= 0.5 + 0.5*float64(zoom-sfxZoomMin)/float64(sfxZoomFull-sfxZoomMin)
	}

	return pan, volScale
}
