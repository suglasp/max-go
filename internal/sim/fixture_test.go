// fixture_test.go : the unit builders shared by the sim tests.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package sim

import (
	"github.com/suglasp/max-go/internal/assets"
	"github.com/suglasp/max-go/internal/game"
)

// addTankTarget deploys a ground tank for the given team (splash hits friend or foe).
func addTankTarget(g *Game, id, gx, gy, team int) *game.UnitInfo {
	u := game.NewUnitInfo(assets.Tank, team, id, 0)
	u.Flags = game.FlagMobileLandUnit | game.FlagSelectable
	u.Values = &game.UnitValues{}
	u.Values.Set(game.AttribHits, 30)
	u.Values.Set(game.AttribArmor, 2)
	u.Hits = 30
	g.Units.Deploy(u, gx, gy)
	return u
}

// addConstructor deploys a healthy mobile builder (id 1) of type t with a movement budget and its cargo at a cell.
func addConstructor(g *Game, t assets.UnitType, team, gx, gy, speed, hits, storage int) *game.UnitInfo {
	u := game.NewUnitInfo(t, team, 1, 0)
	u.Flags = game.FlagMobileLandUnit | game.FlagConstructorUnit | game.FlagSelectable
	u.Values = &game.UnitValues{}
	u.Values.Set(game.AttribSpeed, speed)
	u.Values.Set(game.AttribHits, hits)
	u.InitState() // Speed and Hits from the values
	u.Storage = storage
	g.Units.Deploy(u, gx, gy)
	return u
}
