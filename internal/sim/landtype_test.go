// landtype_test.go : testing unit for movement.go with the reference unit land types (access.LandTypeForUnit)
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package sim

import (
	"testing"

	"github.com/suglasp/max-go/internal/assets"
	"github.com/suglasp/max-go/internal/game"
	"github.com/suglasp/max-go/internal/world"
)

// beachSurf is water on the left, a coast column at x == 5 and land to the right.
type beachSurf struct{}

// SurfaceAt returns water, coast or land by column.
func (beachSurf) SurfaceAt(x, y int) uint8 {
	switch {
	case x < 5:
		return world.SurfaceWater
	case x == 5:
		return world.SurfaceCoast
	default:
		return world.SurfaceLand
	}
}

// TestSeaTransportSailsOntoTheCoast checks that a Sea Transport ordered to a coast cell beside land arrives there.
func TestSeaTransportSailsOntoTheCoast(t *testing.T) {
	g := newGameSurf(12, 6, beachSurf{})

	ship := game.NewUnitInfo(assets.SeaTrans, 0, 1, 0)
	ship.Flags = game.ProducedFlags(assets.SeaTrans)
	ship.Values = &game.UnitValues{}
	ship.Values.Set(game.AttribSpeed, 12)
	ship.Values.Set(game.AttribHits, 30)
	ship.InitState()
	g.Units.Deploy(ship, 1, 2)

	if !g.OrderMove(ship, 5, 2) {
		t.Fatal("OrderMove refused a coast cell for the Sea Transport")
	}

	g.ResolveMovement()

	if ship.GridX != 5 || ship.GridY != 2 {
		t.Errorf("Sea Transport ended at (%d,%d), want the coast cell (5,2)", ship.GridX, ship.GridY)
	}
}
