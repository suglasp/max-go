// building_hum_test.go : tests building selection ambient hum sound triggering.
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
	"github.com/suglasp/max-go/internal/world"
)

// TestPassiveStoreSilentOnSelect verifies that a
// powered, connected passive store (material / fuel / gold) must be SILENT when
// selected - it draws no power and has no select hum clip, so it must not loop the
// generic process hum (nor, as the data bug did, its suffix-15 HIT clip / the gold
// refinery hum). A running building with a real power clip (the mining station)
// still hums. A factory is a special consumer: it draws power (and so runs its
// plant hum) ONLY while actually producing, so a powered but HALTED / empty queue
// factory is silent.
func TestPassiveStoreSilentOnSelect(t *testing.T) {
	m := &world.Map{Surface: make([]uint8, 8*8)}
	m.W, m.H = 8, 8
	for i := range m.Surface {
		m.Surface[i] = world.SurfaceLand
	}
	p := &game.StartParams{}
	p.Teams[0].Type = game.TeamPlayer
	g := sim.New(p, m)
	s := &Screen{game: g, wmap: m}

	mk := func(ty assets.UnitType, gx, gy int, big bool) *game.UnitInfo {
		fl := game.FlagStationary | game.FlagSelectable | game.FlagRequiresSlab
		if big {
			fl |= game.FlagBuilding
		}
		u := &game.UnitInfo{
			Type:   ty,
			Team:   0,
			Flags:  fl,
			Values: &game.UnitValues{},
			Order:  game.OrderPowerOn,
			State:  game.OrderStateExecutingOrder,
		}
		u.Values.Set(game.AttribStorage, 50)
		g.PlaceBuilding(u, gx, gy)
		return u
	}

	// A connected base. Note the complex has 0 fuel here: a running building must
	// still hum (the hum follows the powered on state, not the fuel pool).
	mine := mk(assets.MiningSt, 4, 4, true)     // MSTAT17 + powered on -> hums even at 0 fuel
	gen := mk(assets.PowGen, 6, 4, false)       // POWGN17 -> hums
	idlePlant := mk(assets.LandPlt, 4, 7, true) // powered but NOT producing (empty queue) -> silent, engine off
	busyPlant := mk(assets.AirPlt, 2, 7, true)  // powered AND producing (queue + rate) -> hums generic plant
	radar := mk(assets.Radar, 0, 4, false)      // no power draw but hums (RADAR13 dish) - bug I noticed
	hab := mk(assets.Habitat, 0, 6, true)       // no power draw but hums (general MASTR17) - bug I noticed
	fuel := mk(assets.FDump, 4, 6, false)       // passive store -> silent
	mat := mk(assets.ADump, 5, 6, false)        // passive store -> silent
	gold := mk(assets.GoldSm, 6, 6, false)      // passive store -> silent

	// A producing factory draws power only while it is actually building, so the
	// running plant hum follows the production state, not merely the powered on state.
	busyPlant.BuildQueue = []assets.UnitType{assets.Scout}
	busyPlant.BuildRate = 1

	for _, store := range []struct {
		name string
		u    *game.UnitInfo
	}{{"fuel tank", fuel}, {"material store", mat}, {"gold vault", gold}, {"idle factory", idlePlant}} {
		if tag, _ := s.ambientSfxTags(store.u, nil); tag != "" {
			t.Errorf("powered %s should be SILENT on select, got tag %q", store.name, tag)
		}
	}
	for _, run := range []struct {
		name string
		u    *game.UnitInfo
	}{{"mining station", mine}, {"power generator", gen}, {"producing plant", busyPlant}, {"radar", radar}, {"habitat", hab}} {
		if tag, _ := s.ambientSfxTags(run.u, nil); tag == "" {
			t.Errorf("powered %s should hum on select even at 0 fuel", run.name)
		}
	}
}
