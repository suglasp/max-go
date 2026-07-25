// context.go : the M.A.X. unit AI context for the tasks to plan against.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package ai

import "github.com/suglasp/max-go/internal/sim"

// aiContext is the live game handle the tasks plan against: the simulation to
// query and order, plus the unit factory. The factory is sim.UnitFactory, the
// same closure the game layer hands the sim for its own factory production, so
// the AI and the sim mint units through one shared type rather than two identical
// declarations. The AI decides WHAT and WHERE to build; the factory handles HOW a
// unit is minted (its base attributes + team research live in the game layer, not
// in a layer the AI imports). Game is refreshed at the start of each computer turn
// (the game object is stable, but this keeps the seam explicit).
// C++: the global UnitsManager / GameManager the tasks reach into.
type aiContext struct {
	Game     *sim.Game
	MakeUnit sim.UnitFactory
}
