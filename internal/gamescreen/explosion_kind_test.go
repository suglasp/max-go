// explosion_kind_test.go : tests the destruction blast sheet choice in effects.go.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package gamescreen

import (
	"testing"

	"github.com/suglasp/max-go/internal/assets"
	"github.com/suglasp/max-go/internal/game"
)

// TestSpawnExplosionSheetByUnitKind: the blast a dying unit queues depends on its class, as in the reference (building, land first, then air, then sea, else land).
func TestSpawnExplosionSheetByUnitKind(t *testing.T) {
	tests := []struct {
		name  string
		unit  assets.UnitType
		flags game.Flags
		want  assets.UnitType
		dur   uint64
	}{
		{"aircraft", assets.Fighter, game.FlagMobileAirUnit, assets.AirExpld, smallExplosionMS},
		{"ship", assets.Corvette, game.FlagMobileSeaUnit, assets.SeaExpld, smallExplosionMS},
		{"land unit", assets.Tank, game.FlagMobileLandUnit, assets.LndExpld, smallExplosionMS},
		{"building", assets.PowerStn, game.FlagStationary | game.FlagBuilding, assets.BldExpld, bldExplosionMS},
		{"amphibious land and sea", assets.Scout, game.FlagMobileLandUnit | game.FlagMobileSeaUnit, assets.LndExpld, smallExplosionMS},
		{"land and air", assets.AirTrans, game.FlagMobileLandUnit | game.FlagMobileAirUnit, assets.LndExpld, smallExplosionMS},
		{"air and sea", assets.AirTrans, game.FlagMobileAirUnit | game.FlagMobileSeaUnit, assets.AirExpld, smallExplosionMS},
		{"no class flag", assets.Tank, 0, assets.LndExpld, smallExplosionMS},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s := &Screen{}
			u := game.NewUnitInfo(tc.unit, 0, 1, 0)
			u.Flags = tc.flags
			u.SetPosition(4, 4)

			s.spawnExplosion(u)

			if len(s.effects.explosions) != 1 {
				t.Fatalf("explosions = %d, want 1", len(s.effects.explosions))
			}

			e := s.effects.explosions[0]
			if e.sprite != tc.want {
				t.Errorf("sprite = %v, want %v", e.sprite, tc.want)
			}

			if e.dur != tc.dur {
				t.Errorf("dur = %d, want %d", e.dur, tc.dur)
			}
		})
	}
}
