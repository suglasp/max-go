// build.go : carries the construction and factory build screen result objects.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
// C++ ported : the CONBUILD / FACBUILD menu results
package screendata

import "github.com/suglasp/max-go/internal/assets"

// BuildSelection is the Construction Menu's result handed back to the game: the
// chosen building, the build rate (1/2/4), and the engineer's build along a path
// flag.
type BuildSelection struct {
	Type assets.UnitType
	Rate int
	Path bool
}

// FactorySelection is the factory Production Menu's result handed back to the
// game: the committed build queue (empty = halt the factory), the build rate
// (1/2/4), and the repeat flag.
type FactorySelection struct {
	Queue  []assets.UnitType
	Rate   int
	Repeat bool
}
