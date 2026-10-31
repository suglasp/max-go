// sound_test.go : testing unit for sound.go
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package units

import (
	"testing"

	"github.com/suglasp/max-go/internal/assets"
	"github.com/suglasp/max-go/internal/game"
)

// TestSfxTagFor_KnownUnits verifies that well known (unit, SfxType) pairs
// return the expected resource tag and that the generic fallback kicks in when
// no entry specific to the unit is registered.
func TestSfxTagFor_KnownUnits(t *testing.T) {
	cases := []struct {
		ut   assets.UnitType
		sfx  game.SfxType
		want string
	}{
		// Tank has specific drive, stop, fire, hit, expload.
		{assets.Tank, game.SfxDrive, "TANK5"},
		{assets.Tank, game.SfxStop, "TANK7"},
		{assets.Tank, game.SfxFire, "TANK14"},
		{assets.Tank, game.SfxHit, "TANK15"},
		{assets.Tank, game.SfxExpload, "TANK16"},
		{assets.Tank, game.SfxIdle, "TANK1"},

		// Scout has water variants.
		{assets.Scout, game.SfxWaterDrive, "SCOUT6"},
		{assets.Scout, game.SfxWaterStop, "SCOUT8"},

		// Fixed gun turret: fire overridden, turret uses generic.
		{assets.GunTurrt, game.SfxFire, "FGUN14"},
		{assets.GunTurrt, game.SfxTurret, "GEN_TRRT"},

		// Juggernaught has naval drive/stop.
		{assets.Juggrnt, game.SfxDrive, "JUGGR5"},
		{assets.Juggrnt, game.SfxStop, "JUGGR7"},

		// Sea based units: FastBoat uses ESCRT prefix.
		{assets.FastBoat, game.SfxIdle, "ESCRT2"},
		{assets.FastBoat, game.SfxDrive, "ESCRT6"},

		// Submarine has water idle/drive/stop.
		{assets.Submarne, game.SfxIdle, "SUB2"},
		{assets.Submarne, game.SfxDrive, "SUB6"},

		// Fighter has land/take.
		{assets.Fighter, game.SfxLand, "FIGHT19"},
		{assets.Fighter, game.SfxTake, "FIGHT20"},

		// Constructor has water idle and building sound.
		{assets.Constrct, game.SfxWaterIdle, "CONST2"},
		{assets.Constrct, game.SfxBuilding, "CONST10"},

		// Alien tank.
		{assets.AlnTank, game.SfxFire, "ALNTK14"},
	}
	for _, tc := range cases {
		got := SfxTagFor(tc.ut, tc.sfx)
		if got != tc.want {
			t.Errorf("SfxTagFor(%v, %v) = %q; want %q", tc.ut, tc.sfx, got, tc.want)
		}
	}
}

// TestSfxTagFor_GenericFallback checks that SFX kinds with no unit override
// fall through to the generic default tag.
func TestSfxTagFor_GenericFallback(t *testing.T) {
	// Buildings with no drive entry should fall through.
	got := SfxTagFor(assets.Barracks, game.SfxFire)
	if got != "GEN_FIRE" {
		t.Errorf("SfxTagFor(Barracks, SfxFire) = %q; want GEN_FIRE", got)
	}
	// Generic drive for a unit that doesn't override it.
	got = SfxTagFor(assets.Radar, game.SfxDrive)
	if got != "GEN_DRVE" {
		t.Errorf("SfxTagFor(Radar, SfxDrive) = %q; want GEN_DRVE", got)
	}
	// Water generic idle.
	got = SfxTagFor(assets.Barracks, game.SfxWaterIdle)
	if got != "WGN_IDLE" {
		t.Errorf("SfxTagFor(Barracks, SfxWaterIdle) = %q; want WGN_IDLE", got)
	}
}

// TestSfxDefaultTag checks the generic fallback accessor used when a clip
// specific to the unit is absent from the archive (the audio player falls
// back to this tag). It must match the generic tag SfxTagFor returns for an
// unmapped unit.
func TestSfxDefaultTag(t *testing.T) {
	cases := []struct {
		sfx  game.SfxType
		want string
	}{
		{game.SfxFire, "GEN_FIRE"},
		{game.SfxHit, "GEN_HIT"},
		{game.SfxExpload, "GEN_XPLD"},
		{game.SfxDrive, "GEN_DRVE"},
		{game.SfxStop, "GEN_STOP"},
	}
	for _, tc := range cases {
		if got := SfxDefaultTag(tc.sfx); got != tc.want {
			t.Errorf("SfxDefaultTag(%v) = %q; want %q", tc.sfx, got, tc.want)
		}
		// A unit with no specific entry for this kind returns the same default.
		if got := SfxTagFor(assets.Barracks, tc.sfx); got != tc.want {
			t.Errorf("SfxTagFor(Barracks, %v) = %q; want default %q", tc.sfx, got, tc.want)
		}
	}
}

// TestSfxTagFor_InvalidType checks that a unit type out of bounds returns the
// generic default rather than panicking.
func TestSfxTagFor_InvalidType(t *testing.T) {
	got := SfxTagFor(assets.UnitType(0xFF), game.SfxFire)
	if got != "GEN_FIRE" {
		t.Errorf("SfxTagFor(0xFF, SfxFire) = %q; want GEN_FIRE", got)
	}
}

// TestSfxTagFor_InvalidSfxType checks that an unknown SfxType returns empty
// (no generic registered for it).
func TestSfxTagFor_InvalidSfxType(t *testing.T) {
	got := SfxTagFor(assets.Tank, game.SfxInvalid)
	if got != "" {
		t.Errorf("SfxTagFor(Tank, SfxInvalid) = %q; want empty string", got)
	}
}

// TestHasWaterMovementSfx checks the gate driven by data that the in game screen
// uses to swap a unit's movement sound to its water variant on open water. It
// must be true for exactly the land units that carry their own SfxWater* clips,
// the four amphibious types with both flags set AND the Surveyor (the land_type
// special case that regressed: it crosses water but was playing its land clip),
// and false for a unit that is purely land and one that is purely sea (whose
// drive sound lives under the plain keys, not SfxWater*).
func TestHasWaterMovementSfx(t *testing.T) {
	cases := []struct {
		ut   assets.UnitType
		want bool
	}{
		{assets.Constrct, true}, // amphibious, both flags set
		{assets.Engineer, true},
		{assets.Scout, true},
		{assets.ClnTrans, true},  // Personnel Carrier
		{assets.Surveyor, true},  // land_type special case - the target of the fix
		{assets.Tank, false},     // pure land
		{assets.Submarne, false}, // pure sea: drive sound is under plain SfxDrive
		{assets.FastBoat, false}, // pure sea
		{assets.Scanner, false},  // land recon, no water clips
		{assets.UnitType(0xFF), false},
	}
	for _, tc := range cases {
		if got := HasWaterMovementSfx(tc.ut); got != tc.want {
			t.Errorf("HasWaterMovementSfx(%v) = %v; want %v", tc.ut, got, tc.want)
		}
	}
}

/* // note: ported but unused code in Go port
// TestHasPowerSfx checks the gate the in game screen uses to decide whether a
// selected building hums: true only for buildings whose data ships a real clip
// for the start of power consumption (17). The passive stores ship only
// hit/explode clips (15/16), so they must be false - the regression where their
// "power start" pointed at the suffix 15 HIT clip made a selected fuel tank or
// storage loop a sound like a firing impact.
func TestHasPowerSfx(t *testing.T) {
	cases := []struct {
		ut   assets.UnitType
		want bool
	}{
		{assets.PowerStn, true}, // POWST17
		{assets.PowGen, true},   // POWGN17
		{assets.MiningSt, true}, // MSTAT17
		{assets.Barracks, true}, // BARRA17
		{assets.Hangar, true},   // HANGR17
		{assets.Dock, true},     // DOCK17
		{assets.GoldSm, false},  // gold VAULT: a passive store - silent (GOLDR17 is the refinery hum, not the vault's)
		{assets.ADump, false},   // material store: only SSTOR15/16 ship (no power clip)
		{assets.FDump, false},   // fuel tank: only SFUEL15/16
		{assets.LandPad, false}, // only LPAD15/16
		{assets.Depot, false},   // only DEPOT15/16
		{assets.Habitat, true},  // DORMI17 running hum (falls back to MASTR17) - habitat hums on select
		{assets.Block, false},   // only BLOCK16
		{assets.Bridge, false},  // only BRIDG16
		{assets.Radar, true},    // RADAR13 reused as the dish running hum - radar hums on select
		{assets.UnitType(0xFF), false},
	}
	for _, tc := range cases {
		if got := HasPowerSfx(tc.ut); got != tc.want {
			t.Errorf("HasPowerSfx(%v) = %v; want %v", tc.ut, got, tc.want)
		}
	}
}
*/

// TestStorageHitNotPowerSfx locks the data fix: the three passive stores
// (material/fuel/gold) map their suffix 15/16 clips to HIT/EXPLODE (their real
// purpose) and define NO power start hum, so all three are silent on select -
// the regression looped the impact sound (material/fuel) or the refinery hum
// (gold).
func TestStorageHitNotPowerSfx(t *testing.T) {
	for _, ut := range []assets.UnitType{assets.ADump, assets.FDump, assets.GoldSm} {
		if _, ok := unitSfx[ut][game.SfxPowerStart]; ok {
			t.Errorf("%v should not map SfxPowerStart (a passive store has no select hum)", ut)
		}
		if unitSfx[ut][game.SfxHit] == "" {
			t.Errorf("%v should map SfxHit to its suffix 15 clip", ut)
		}
	}
	if unitSfx[assets.ADump][game.SfxHit] != "SSTOR15" {
		t.Errorf("ADump SfxHit = %q; want SSTOR15", unitSfx[assets.ADump][game.SfxHit])
	}
	if unitSfx[assets.FDump][game.SfxHit] != "SFUEL15" {
		t.Errorf("FDump SfxHit = %q; want SFUEL15", unitSfx[assets.FDump][game.SfxHit])
	}
	if unitSfx[assets.GoldSm][game.SfxHit] != "GOLDR15" {
		t.Errorf("GoldSm SfxHit = %q; want GOLDR15", unitSfx[assets.GoldSm][game.SfxHit])
	}
}

// TestUsesWaterMovementSfxSlot locks the surface to sound slot decision: a
// land unit capable of water (the four amphibians with both flags set, the
// Surveyor, the Personnel Carrier) uses its water clip on open water and the
// plain clip on land/coast; a unit with no water clip never uses it.
func TestUsesWaterMovementSfxSlot(t *testing.T) {
	cases := []struct {
		ut      assets.UnitType
		onWater bool
		want    bool
	}{
		{assets.Surveyor, true, true},   // water slot on water
		{assets.Surveyor, false, false}, // plain on land
		{assets.Scout, true, true},
		{assets.Scout, false, false},
		{assets.Engineer, true, true},
		{assets.ClnTrans, true, true},   // water clip on water (same rule as the rest)
		{assets.ClnTrans, false, false}, // plain clip on land
		{assets.Tank, true, false},      // no water clip at all
		{assets.Submarne, true, false},  // pure sea
	}
	for _, tc := range cases {
		if got := UsesWaterMovementSfxSlot(tc.ut, tc.onWater); got != tc.want {
			t.Errorf("UsesWaterMovementSfxSlot(%v, onWater=%v) = %v; want %v", tc.ut, tc.onWater, got, tc.want)
		}
	}
}

/* // note: unused code in Go port
// TestSfxPan verifies the stereo pan formula: centre of view => 0, left edge
// => -1, right edge => +1, clamped once out of bounds.
func TestSfxPan(t *testing.T) {
	cases := []struct {
		unitScreenX, viewW int
		want               float64
	}{
		{320, 640, 0.0},   // centre
		{0, 640, -1.0},    // left edge
		{640, 640, 1.0},   // right edge
		{-100, 640, -1.0}, // off screen left, clamped
		{800, 640, 1.0},   // off screen right, clamped
		{160, 640, -0.5},  // quarter left
		{480, 640, 0.5},   // quarter right
		{0, 0, 0.0},       // degenerate: no view
	}
	for _, tc := range cases {
		got := SfxPan(tc.unitScreenX, tc.viewW)
		const eps = 1e-9
		if got < tc.want-eps || got > tc.want+eps {
			t.Errorf("SfxPan(%d, %d) = %v; want %v", tc.unitScreenX, tc.viewW, got, tc.want)
		}
	}
}
*/

// TestSfxPosition checks klei's positional sound model: the pan widens to +/-0.5 at 28 cells off the view
// centre, the volume fades over 112 cells, and a zoom below 32 px per cell fades it down to half at zoom 4.
func TestSfxPosition(t *testing.T) {
	cases := []struct {
		name          string
		dx, dy, zoom  int
		reversed      bool
		wantPan, wVol float64
	}{
		{"centre", 0, 0, 64, false, 0, 1},
		{"14 right", 14, 0, 64, false, 0.25, 1 - 14.0/112},
		{"28 right", 28, 0, 64, false, 0.5, 1 - 28.0/112},
		{"40 right clamps the pan", 40, 0, 64, false, 0.5, 1 - 40.0/112},
		{"28 left", -28, 0, 64, false, -0.5, 1 - 28.0/112},
		{"reversed", 28, 0, 64, true, -0.5, 1 - 28.0/112},
		{"56 below", 0, 56, 64, false, 0, 0.5},
		{"112 off: silent", 112, 0, 64, false, 0.5, 0},
		{"130 off: still silent", 0, -130, 64, false, 0, 0},
		{"zoom 32: no fade", 0, 0, 32, false, 0, 1},
		{"zoom 4: half", 0, 0, 4, false, 0, 0.5},
		{"zoom 18: three quarters", 0, 0, 18, false, 0, 0.75},
	}
	const eps = 1e-9
	for _, tc := range cases {
		pan, vol := SfxPosition(tc.dx, tc.dy, tc.zoom, tc.reversed)
		if pan < tc.wantPan-eps || pan > tc.wantPan+eps || vol < tc.wVol-eps || vol > tc.wVol+eps {
			t.Errorf("%s: SfxPosition(%d, %d, %d, %v) = (%v, %v); want (%v, %v)", tc.name, tc.dx, tc.dy, tc.zoom, tc.reversed, pan, vol, tc.wantPan, tc.wVol)
		}
	}
}
