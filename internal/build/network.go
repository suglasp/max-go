// network.go : groups adjacent buildings into resource sharing complexes.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package build

import (
	"github.com/suglasp/max-go/internal/economy"
	"github.com/suglasp/max-go/internal/game"
)

// adjacent reports whether two building footprints share an orthogonal edge.
// This simplified model connects touching structures without consulting the
// original directional connector bits.
//
// C++: the CONNECTOR_* adjacency that UnitsManager_UpdateConnectors wires up
// (units_manager.cpp), simplified to edge adjacency.
func adjacent(a, b *game.UnitInfo) bool {
	an, bn := Footprint(a.Flags), Footprint(b.Flags)

	for ay := a.GridY; ay < a.GridY+an; ay++ {
		for ax := a.GridX; ax < a.GridX+an; ax++ {
			for by := b.GridY; by < b.GridY+bn; by++ {
				for bx := b.GridX; bx < b.GridX+bn; bx++ {
					dx := ax - bx
					dy := ay - by

					if (dx == 0 && (dy == 1 || dy == -1)) || (dy == 0 && (dx == 1 || dx == -1)) {
						return true
					}
				}
			}
		}
	}

	return false
}

// Network tracks one team's building complexes and reshapes them as buildings
// are placed or removed. A building joins the complex with the lowest id among
// the structures it touches, merging any other touched complexes into it; an
// isolated building starts a new complex.
//
// C++: UnitInfo::AttachToPrimaryComplex / AttachComplex + TeamUnits::Create
// Complex (unitinfo.cpp / teamunits.cpp), over the simplified adjacency above.
type Network struct {
	Team      int
	Complexes []*economy.Complex
	owner     map[*game.UnitInfo]*economy.Complex
	nextID    int
}

// NewNetwork returns an empty network for a team. firstID is the id assigned to
// the first complex created (complex ids ascend from there).
func NewNetwork(team, firstID int) *Network {
	return &Network{Team: team, owner: map[*game.UnitInfo]*economy.Complex{}, nextID: firstID}
}

// ComplexOf returns the complex a placed building belongs to, or nil.
func (n *Network) ComplexOf(b *game.UnitInfo) *economy.Complex {
	return n.owner[b]
}

/* // note: unused code in Go port
// members returns the buildings already placed in the network.
func (n *Network) members() []*game.UnitInfo {
	out := make([]*game.UnitInfo, 0, len(n.owner))

	for b := range n.owner {
		out = append(out, b)
	}

	return out
}
*/

// Add places a building into the network, forming or merging complexes by
// adjacency, and returns the complex it ends up in.
func (n *Network) Add(b *game.UnitInfo) *economy.Complex {
	// Collect the distinct complexes of the buildings this one touches.
	var touched []*economy.Complex

	for existing, c := range n.owner {
		if adjacent(b, existing) && !containsComplex(touched, c) {
			touched = append(touched, c)
		}
	}

	target := lowestID(touched)
	if target == nil {
		target = economy.NewComplex(n.nextID)
		target.Team = n.Team
		n.nextID++
		n.Complexes = append(n.Complexes, target)
	}

	// Merge every other touched complex into target.
	for _, c := range touched {
		if c == target {
			continue
		}
		n.mergeInto(target, c)
	}

	target.Grow(b)
	n.owner[b] = target

	return target
}

// Remove takes a building out of the network and drops an empty complex. It does
// not split the remaining members when the removed building was their bridge.
func (n *Network) Remove(b *game.UnitInfo) {
	c := n.owner[b]

	if c == nil {
		return
	}
	delete(n.owner, b)

	if c.Shrink(b) {
		n.dropComplex(c)
	}
}

// mergeInto moves every member of src into dst and removes src.
func (n *Network) mergeInto(dst, src *economy.Complex) {
	dst.Material += src.Material
	dst.Fuel += src.Fuel
	dst.Gold += src.Gold

	for _, m := range src.Members {
		dst.Buildings++
		n.owner[m] = dst
	}

	dst.Members = append(dst.Members, src.Members...)
	src.Members = nil
	src.Buildings = 0
	n.dropComplex(src)
}

// dropComplex removes a complex from the network's list.
func (n *Network) dropComplex(c *economy.Complex) {
	kept := n.Complexes[:0]

	for _, x := range n.Complexes {
		if x != c {
			kept = append(kept, x)
		}
	}

	n.Complexes = kept
}

// lowestID returns the complex with the smallest id, or nil for an empty list.
func lowestID(cs []*economy.Complex) *economy.Complex {
	var best *economy.Complex

	for _, c := range cs {
		if best == nil || c.ID < best.ID {
			best = c
		}
	}

	return best
}

// containsComplex reports whether c is already present in the complex list.
func containsComplex(cs []*economy.Complex, c *economy.Complex) bool {
	for _, x := range cs {
		if x == c {
			return true
		}
	}

	return false
}
