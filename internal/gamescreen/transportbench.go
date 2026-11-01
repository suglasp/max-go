// transportbench.go : provides the transport emulation bench.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package gamescreen

import (
	"github.com/suglasp/max-go/internal/assets"
	"github.com/suglasp/max-go/internal/dbg"
	"github.com/suglasp/max-go/internal/game"
	"github.com/suglasp/max-go/internal/world"
)

// transportBenchZoom opens the bench large enough that the transports, their labels
// and the ejected units read clearly.
const transportBenchZoom = 48

// DeployTransport lays out the transport unload test bench for -emulate transport, on
// snow. A land transport (APC / ClnTrans) and an air transport (AirTrans) each START
// CARRYING two land units: select one, pick "Activate" to open the Repair Shop screen,
// choose a carried unit, and it ejects with the Depot style build ready exit (BLDMRK
// markers -> click one -> the unit unfolds and drives out). Two free land units sit
// beside the APC so the Enter/Load boarding flow can be tested too (board them, then
// Activate to unload). A SEA transport (pre loaded with a Corvette + Submarine) plus a
// free Corvette sit on the nearest water block for the sea load/unload flow (skipped if
// the map has no water). A lone enemy tank far off keeps the turn loop / win check
// happy. C++: n/a (a debug layout helper).
func (s *Screen) DeployTransport() {
	originX, originY, ok := s.snowBlock(10, 9)

	if !ok {
		originX, originY = 4, 4
	}

	const (
		player = 0
		enemy  = 1
	)
	land := game.FlagMobileLandUnit | game.FlagSelectable

	// The land transport, pre loaded with two land units.
	apc := s.deployUnit(assets.ClnTrans, land, player, 0, originX, originY)
	s.galleryLabels = append(s.galleryLabels, galleryLabel{name: "APC (loaded)", gx: originX, gy: originY})
	s.preloadTransport(apc, []assets.UnitType{assets.Tank, assets.Scout})

	// The air transport, pre loaded with two land units (air transports ferry GROUND
	// units, so it unloads them onto adjacent land - it is parked over snow here).
	airX, airY := originX, originY+3
	airFlags := game.FlagMobileAirUnit | game.FlagHovering | game.FlagSelectable
	air := s.deployUnit(assets.AirTrans, airFlags, player, 0, airX, airY)
	s.galleryLabels = append(s.galleryLabels, galleryLabel{name: "Air Transport (loaded)", gx: airX, gy: airY})
	s.preloadTransport(air, []assets.UnitType{assets.Tank, assets.Scout})

	// Two free land units in a column to the right: Enter/Load them aboard the APC,
	// then Activate to unload. (Move one adjacent to the APC first - boarding needs
	// adjacency.) A column keeps each name label clear of the others.
	type freeUnit struct {
		t    assets.UnitType
		dy   int
		name string
	}
	free := []freeUnit{
		{assets.Tank, 0, "Free Tank"},
		{assets.Scout, 3, "Free Scout"},
	}

	for _, f := range free {
		gx, gy := originX+4, originY+f.dy
		s.deployUnit(f.t, land, player, 0, gx, gy)
		s.galleryLabels = append(s.galleryLabels, galleryLabel{name: f.name, gx: gx, gy: gy})
	}

	// A sea transport + free sea unit on the nearest water block, so sea load/unload
	// can be tested too (skipped if the map has no water).
	s.deploySeaSection()

	// A lone enemy tank off to the side so there are two live sides (no instant win,
	// and the turn loop has another team to hand over to on End Turn).
	ex, ey := originX+7, originY+6
	s.deployUnit(assets.Tank, land|game.FlagTurretSprite, enemy, 0, ex, ey)
	s.galleryLabels = append(s.galleryLabels, galleryLabel{name: "Enemy", gx: ex, gy: ey})
	s.freezeBenchTeam(enemy)

	s.game.State.ActiveTeam = player
	s.game.State.PlayerTeam = player
	s.fogShow = false
	s.r.SetZoom(transportBenchZoom)
	s.startSingleBench("Transport bench: the APC and the Air Transport carry two units each - popup Activate, pick one, click an exit marker. Load the two free units beside the APC (move one next to it first). A loaded Sea Transport and a free Corvette sit on the nearest water.", game.Point{X: originX + 2, Y: originY + 2}, apc)
	dbg.Logf(dbg.UI, "transport bench: snow (%d,%d) APC carries %d, AirTransport carries %d", originX, originY, len(apc.Carried), len(air.Carried))
}

// preloadTransport boards land units of the given types into transport via the real
// LoadUnit path: each is briefly deployed on a free adjacent cell and sucked aboard,
// so the transport opens its Repair Shop screen already carrying them.
func (s *Screen) preloadTransport(transport *game.UnitInfo, types []assets.UnitType) {
	s.preloadTransportFlags(transport, types, game.FlagMobileLandUnit|game.FlagSelectable)
}

// preloadTransportFlags is preloadTransport with the passenger class flags spelled
// out for every supported passenger class. Each passenger leaves the map as
// it boards, so the staging cells are reused safely.
func (s *Screen) preloadTransportFlags(transport *game.UnitInfo, types []assets.UnitType, flags game.Flags) {
	gx, gy := transport.GridX, transport.GridY
	// Adjacent staging cells (positive offsets stay inside the deploy block).
	spots := [][2]int{{gx + 1, gy}, {gx, gy + 1}, {gx + 1, gy + 1}}

	for i, t := range types {
		if i >= len(spots) {
			break
		}

		u := s.deployUnit(t, flags, transport.Team, 0, spots[i][0], spots[i][1])

		if !s.game.LoadUnit(transport, u) {
			dbg.Logf(dbg.UI, "transport bench: could not board %v into %v", t, transport.Type)
		}
	}
}

// coastCell finds a WATER cell that has at least one orthogonally adjacent LAND cell
// (a shore), returning the water cell plus its adjacent land cells. A sea transport
// sits on the water and ferries LAND units that board from the shore. ok is false when
// the map has no coast (the sea section is then skipped). C++: n/a (a debug helper).
func (s *Screen) coastCell() (waterX, waterY int, land [][2]int, ok bool) {
	dirs := [][2]int{{1, 0}, {-1, 0}, {0, 1}, {0, -1}}

	for y := 2; y < s.wmap.H-2; y++ {
		for x := 2; x < s.wmap.W-2; x++ {
			if s.wmap.SurfaceAt(x, y)&world.SurfaceWater == 0 {
				continue
			}

			var shore [][2]int

			for _, d := range dirs {
				lx, ly := x+d[0], y+d[1]

				if s.wmap.SurfaceAt(lx, ly)&world.SurfaceLand != 0 {
					shore = append(shore, [2]int{lx, ly})
				}
			}

			if len(shore) > 0 {
				return x, y, shore, true
			}
		}
	}

	return 0, 0, nil, false
}

// deploySeaSection adds a SEA transport on a coast, ferrying LAND units (a sea
// transport carries land units across water; sea units like a Corvette/Submarine board
// a DOCK, never a transport). It pre loads the transport with a Tank and leaves a free
// land unit on the shore to board it, so both load (Enter / the transport's Load, from
// the adjacent shore) and unload (Activate) are testable. Skipped when the map has no
// coast. C++: access.cpp Access_GetReceiverUnit (SEATRANS takes land units).
func (s *Screen) deploySeaSection() {
	wx, wy, shore, ok := s.coastCell()

	if !ok {
		dbg.Logf(dbg.UI, "transport bench: no coast found, skipping the sea transport section")
		return
	}

	land := game.FlagMobileLandUnit | game.FlagSelectable

	// The sea transport, on the water cell. It carries LAND units, not sea units.
	seaTrans := s.deployUnit(assets.SeaTrans, game.FlagMobileSeaUnit|game.FlagSelectable, 0, 0, wx, wy)
	s.galleryLabels = append(s.galleryLabels, galleryLabel{name: "Sea Transport (loaded)", gx: wx, gy: wy})

	// Pre load one land unit by staging it on the shore (adjacent to the transport) and
	// boarding it through the real LoadUnit path, so Activate can unload it at once.
	staged := s.deployUnit(assets.Tank, land, 0, 0, shore[0][0], shore[0][1])

	if !s.game.LoadUnit(seaTrans, staged) {
		dbg.Logf(dbg.UI, "transport bench: could not board the Tank into the Sea Transport")
	}

	// A free land unit on the shore (shore[0] is free again now the staged unit boarded):
	// drive it onto the transport, then Activate to unload.
	s.deployUnit(assets.Infantry, land, 0, 0, shore[0][0], shore[0][1])
	s.galleryLabels = append(s.galleryLabels, galleryLabel{name: "Shore Infantry", gx: shore[0][0], gy: shore[0][1]})
}
