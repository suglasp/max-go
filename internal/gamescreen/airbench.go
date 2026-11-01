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
	// Use PlaceBuilding so each pad receives its slab.
	bx, by := originX+4, originY+4
	s.benchPlaceBuilding(assets.LandPad, player, bx, by, false)
	s.benchPlaceBuilding(assets.LandPad, player, originX+1, originY+1, false)

	// Player bomber flying over the pad (InitState sets AirAlt = full while hovering).
	bomber := s.deployUnit(assets.Bomber, air, player, 0, bx, by)

	// A friendly tank tests routing around the landed bomber. A nearby enemy tank shows the grounded plane is vulnerable.
	s.deployUnit(assets.Tank, mobile|game.FlagTurretSprite, player, 2, originX+1, originY+4)
	s.deployUnit(assets.Tank, mobile|game.FlagTurretSprite, enemy, 6, originX+7, originY+4)

	// The southern enemy complex is a large generator, three storage units, and a mining station linked by connectors.
	// Large buildings use FlagBuilding and a 2x2 footprint. Same-team edge-adjacent structures automatically rig connector arms.
	// Use PlaceBuilding for slab-backed structures; connectors do not lay slabs.
	row := originY + 8
	s.benchPlaceBuilding(assets.PowerStn, enemy, originX+1, row, true) // large generator (2x2)
	s.benchPlaceConnector(enemy, originX+3, row)                       // connector bridging generator -> storage
	s.benchPlaceBuilding(assets.ADump, enemy, originX+4, row, false)   // storage unit (1x1)
	s.benchPlaceBuilding(assets.ADump, enemy, originX+5, row, false)   // storage unit (1x1)
	s.benchPlaceBuilding(assets.ADump, enemy, originX+6, row, false)   // storage unit (1x1)
	s.benchPlaceConnector(enemy, originX+7, row)                       // connector bridging storage -> mine
	s.benchPlaceBuilding(assets.MiningSt, enemy, originX+8, row, true) // mining station / mine (2x2)

	s.freezeBenchTeam(enemy)
	s.game.State.ActiveTeam = player
	s.game.State.PlayerTeam = player

	s.selected = bomber
	s.showNames = true
	s.fogShow = false
	s.r.SetZoom(airBenchZoom)
	s.startSingleBench("Air bench: attack the enemy tank -> a ROCKET flies + explodes on arrival. Scroll SOUTH to the enemy complex (generator/storage/mine) and attack a BUILDING - the bomber should fire in place, not fly over. Click a LANDING PAD to fly onto it + auto-land.", game.Point{X: originX + 5, Y: originY + 6}, bomber)
	dbg.Printf("air bench: land block (%d,%d) deployed=%d", originX, originY, len(s.game.Units.Units()))
}
