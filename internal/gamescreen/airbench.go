// airbench.go : provides the air unit benchmark harness.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package gamescreen

import (
	"github.com/suglasp/max-go/internal/assets"
	"github.com/suglasp/max-go/internal/dbg"
	"github.com/suglasp/max-go/internal/game"
)

// airBenchZoom frames the takeoff and landing bench.
const airBenchZoom = 48

// DeployAirBench lays out the -emulate air test bench on snow with a player BOMBER over open ground and a nearby player TANK.
// Land makes the bomber descend, block ground units, and become a ground target. Take Off returns it to the air.
// An enemy tank shows that a grounded plane is vulnerable. A southern enemy complex verifies the attack and far-target cursors over structures.
// C++: n/a (a debug layout helper for ORDER_TAKE_OFF / ORDER_LAND).
func (s *Screen) DeployAirBench() {
	// Prefer a block tall enough for the tank test and southern complex. Fall back to the original 9x9, then a fixed corner.
	originX, originY, ok := s.snowBlock(12, 12)

	if !ok {
		originX, originY, ok = s.snowBlock(9, 9)
	}

	if !ok {
		originX, originY = 4, 4
	}

	const (
		player = 0
		enemy  = 1
	)
	mobile := game.FlagMobileLandUnit | game.FlagSelectable
	air := game.FlagMobileAirUnit | game.FlagHovering | game.FlagSelectable

	// Two player LANDING PADs, one under the bomber and one to fly to.
	pad := game.FlagStationary | game.FlagSelectable
	bx, by := originX+4, originY+4
	s.deployUnit(assets.LandPad, pad, player, 0, bx, by)
	s.deployUnit(assets.LandPad, pad, player, 0, originX+1, originY+1)

	// Player bomber flying over the pad (InitState sets AirAlt = full while hovering).
	bomber := s.deployUnit(assets.Bomber, air, player, 0, bx, by)

	// A friendly tank tests routing around the landed bomber. A nearby enemy tank shows the grounded plane is vulnerable.
	s.deployUnit(assets.Tank, mobile|game.FlagTurretSprite, player, 2, originX+1, originY+4)
	s.deployUnit(assets.Tank, mobile|game.FlagTurretSprite, enemy, 6, originX+7, originY+4)

	// The southern enemy complex is a large generator, three storage units, and a mining station linked by connectors.
	// Large buildings use FlagBuilding and a 2x2 footprint. Same-team edge-adjacent structures automatically rig connector arms.
	bld2x2 := game.FlagStationary | game.FlagBuilding | game.FlagSelectable // large 2x2 structure
	small := game.FlagStationary | game.FlagSelectable                      // 1x1 structure (storage / connector)
	row := originY + 8
	s.deployUnit(assets.PowerStn, bld2x2, enemy, 0, originX+1, row) // large generator (2x2)
	s.deployUnit(assets.Cnct4W, small, enemy, 0, originX+3, row)    // connector bridging generator -> storage
	s.deployUnit(assets.ADump, small, enemy, 0, originX+4, row)     // storage unit (1x1)
	s.deployUnit(assets.ADump, small, enemy, 0, originX+5, row)     // storage unit (1x1)
	s.deployUnit(assets.ADump, small, enemy, 0, originX+6, row)     // storage unit (1x1)
	s.deployUnit(assets.Cnct4W, small, enemy, 0, originX+7, row)    // connector bridging storage -> mine
	s.deployUnit(assets.MiningSt, bld2x2, enemy, 0, originX+8, row) // mining station / mine (2x2)

	s.game.State.Teams[enemy].Type = game.TeamPlayer
	s.game.State.ActiveTeam = player
	s.game.State.PlayerTeam = player

	s.selected = bomber
	s.showNames = true
	s.fogShow = false
	s.r.SetZoom(airBenchZoom)
	vw, vh := viewSize(MainMap)
	s.camera.scrollX, s.camera.scrollY = centerScroll(originX+5, originY+6, s.r.Zoom(), vw, vh)
	s.camera.scrollX, s.camera.scrollY = s.r.ClampScroll(s.camera.scrollX, s.camera.scrollY, vw, vh)
	s.showMessage("Air bench: attack the enemy tank -> a ROCKET flies + explodes on arrival. Scroll SOUTH to the enemy complex (generator/storage/mine) and attack a BUILDING - the bomber should fire in place, not fly over. Click a LANDING PAD to fly onto it + auto-land.")
	dbg.Printf("air bench: land block (%d,%d) deployed=%d", originX, originY, len(s.game.Units.Units()))
}
