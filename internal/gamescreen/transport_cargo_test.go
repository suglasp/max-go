// transport_cargo_test.go : tests transport cargo capacity and multiple unit load sequencing.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package gamescreen

import (
	"testing"

	"github.com/suglasp/max-go/internal/assets"
	"github.com/suglasp/max-go/internal/game"
	"github.com/suglasp/max-go/internal/sim"
)

// TestDeployUnitTransportCapacityFromAttribs verifies that
// a demo/bench spawned transport (deployUnit) showed its UNIT capacity gauge
// as 0/40 (a flat placeholder) instead of its real ATTRIBS capacity. deployUnit
// must read the real Storage so a personnel/sea carrier reads 6 and an air
// transport 3 - the value the left bar "Units" gauge shows.
func TestDeployUnitTransportCapacityFromAttribs(t *testing.T) {
	s, _ := newTestScreen(t, 16)

	// Minimal ATTRIBS table with the real carrier capacities.
	attribs := make([]assets.UnitAttributes, assets.UnitCount)
	attribs[assets.ClnTrans] = assets.UnitAttributes{Storage: 6}
	attribs[assets.AirTrans] = assets.UnitAttributes{Storage: 3}
	s.SetAttribs(attribs)

	cln := s.deployUnit(assets.ClnTrans, game.FlagMobileLandUnit|game.FlagSelectable, 0, 2, 5, 5)
	if got := sim.TransportCapacity(cln); got != 6 {
		t.Errorf("ClnTrans capacity = %d, want 6 (real ATTRIBS, not the 40 placeholder)", got)
	}
	air := s.deployUnit(assets.AirTrans, game.FlagMobileAirUnit|game.FlagSelectable, 0, 2, 8, 8)
	if got := sim.TransportCapacity(air); got != 3 {
		t.Errorf("AirTrans capacity = %d, want 3", got)
	}

	// Headless fallback: with no ATTRIBS loaded the demo default (40) still applies,
	// so a transport is at least placeable (not 0-capacity).
	s2, _ := newTestScreen(t, 16)
	u := s2.deployUnit(assets.ClnTrans, game.FlagMobileLandUnit|game.FlagSelectable, 0, 2, 5, 5)
	if got := sim.TransportCapacity(u); got != 40 {
		t.Errorf("ClnTrans capacity with no ATTRIBS = %d, want the 40 headless fallback", got)
	}
}
