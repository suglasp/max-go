// t4_transport_test.go : testing unit for transport.go loading rules
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package gamescreen

import (
	"testing"

	"github.com/suglasp/max-go/internal/assets"
	"github.com/suglasp/max-go/internal/game"
	"github.com/suglasp/max-go/internal/platform"
	"github.com/suglasp/max-go/internal/sim"
)

// t4Carrier deploys an awake transport or storage building with the given bay capacity.
func t4Carrier(g *sim.Game, typ assets.UnitType, id, gridX, gridY int, flags game.Flags, capacity int) *game.UnitInfo {
	u := mkUnit(g, typ, id, gridX, gridY, flags)
	u.Values.Set(game.AttribStorage, capacity)
	u.Hits = 10
	u.Order = game.OrderAwait

	return u
}

// t4Passenger deploys a healthy mobile land unit with movement points left.
func t4Passenger(g *sim.Game, typ assets.UnitType, id, gridX, gridY int) *game.UnitInfo {
	u := mkUnit(g, typ, id, gridX, gridY, game.FlagMobileLandUnit|game.FlagSelectable)
	u.Values.Set(game.AttribHits, 10)
	u.Values.Set(game.AttribSpeed, 8)
	u.InitState()
	u.Hits = 10
	u.Order = game.OrderAwait

	return u
}

// t4ForceShrinkStep makes the next tickLoads call advance the shrink animation.
func t4ForceShrinkStep(s *Screen) {
	s.effects.loadStamp = 1 << 63 // far in the future: now minus stamp wraps to a huge value
}

// TestT4ArmLoad pins which units can arm a load pick: mobile and not disabled.
func TestT4ArmLoad(t *testing.T) {
	cases := []struct {
		name  string
		setup func(g *sim.Game) *game.UnitInfo
		armed bool
	}{
		{"mobile land unit arms", func(g *sim.Game) *game.UnitInfo { return t4Passenger(g, assets.Infantry, 1, 5, 5) }, true},
		{"building cannot be stored", func(g *sim.Game) *game.UnitInfo {
			return mkUnit(g, assets.Depot, 1, 5, 5, game.FlagStationary|game.FlagBuilding|game.FlagSelectable)
		}, false},
		{"disabled unit is refused", func(g *sim.Game) *game.UnitInfo {
			u := t4Passenger(g, assets.Infantry, 1, 5, 5)
			u.Order = game.OrderDisable

			return u
		}, false},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			s, g := newClickScreen(t)
			u := c.setup(g)
			s.armLoad(u)

			if s.orderInput.loadArmed != c.armed {
				t.Fatalf("loadArmed = %v, want %v", s.orderInput.loadArmed, c.armed)
			}

			if c.armed && s.orderInput.loadPassenger != u {
				t.Error("the armed pick must remember the passenger")
			}

			if !c.armed && s.orderInput.loadPassenger != nil {
				t.Error("a refused arm must leave no passenger behind")
			}
		})
	}
}

// TestT4ArmLoadReplacesOtherPicks pins that arming one transport pick disarms the others.
func TestT4ArmLoadReplacesOtherPicks(t *testing.T) {
	s, g := newClickScreen(t)
	depot := t4Carrier(g, assets.Depot, 1, 8, 8, game.FlagStationary|game.FlagBuilding|game.FlagSelectable, 4)
	tank := t4Passenger(g, assets.Tank, 2, 5, 5)

	s.armLoadInto(depot)

	if s.effects.loadInto != depot {
		t.Fatal("armLoadInto should arm the building")
	}

	s.armLoad(tank)

	if s.effects.loadInto != nil {
		t.Error("arming a unit load must clear the building load pick")
	}

	if !s.orderInput.loadArmed {
		t.Error("the unit load pick should be armed")
	}
}

// TestT4ArmLoadInto pins that a bay with room arms the pick and a full bay refuses it.
func TestT4ArmLoadInto(t *testing.T) {
	flags := game.FlagStationary | game.FlagBuilding | game.FlagSelectable

	t.Run("room left arms", func(t *testing.T) {
		s, g := newClickScreen(t)
		depot := t4Carrier(g, assets.Depot, 1, 8, 8, flags, 2)
		depot.Carried = append(depot.Carried, t4Passenger(g, assets.Tank, 2, 1, 1))
		s.armLoadInto(depot)

		if s.effects.loadInto != depot {
			t.Error("a depot with one free slot should arm the pick")
		}
	})

	t.Run("full bay refuses", func(t *testing.T) {
		s, g := newClickScreen(t)
		depot := t4Carrier(g, assets.Depot, 1, 8, 8, flags, 1)
		depot.Carried = append(depot.Carried, t4Passenger(g, assets.Tank, 2, 1, 1))
		s.armLoadInto(depot)

		if s.effects.loadInto != nil {
			t.Error("a full depot must not arm the pick")
		}

		if s.banner.msg == "" {
			t.Error("a full bay should say why")
		}
	})
}

// TestT4BeginLoad pins that a far unit is sent to a cell beside the carrier and a near one is not.
func TestT4BeginLoad(t *testing.T) {
	t.Run("far unit drives over", func(t *testing.T) {
		s, g := newClickScreen(t)
		apc := t4Carrier(g, assets.ClnTrans, 1, 5, 5, game.FlagMobileLandUnit|game.FlagSelectable, 2)
		squad := t4Passenger(g, assets.Infantry, 2, 10, 10)
		s.beginLoad(squad, apc)

		if s.effects.loading[squad] == nil || s.effects.loading[squad].building != apc {
			t.Fatal("beginLoad should register the load job")
		}

		if !s.isMoving(squad) {
			t.Error("a unit away from the carrier must be sent driving to it")
		}
	})

	t.Run("adjacent unit stays put", func(t *testing.T) {
		s, g := newClickScreen(t)
		apc := t4Carrier(g, assets.ClnTrans, 1, 5, 5, game.FlagMobileLandUnit|game.FlagSelectable, 2)
		squad := t4Passenger(g, assets.Infantry, 2, 6, 5)
		s.beginLoad(squad, apc)

		if s.effects.loading[squad] == nil {
			t.Fatal("beginLoad should register the load job")
		}

		if s.isMoving(squad) {
			t.Error("an adjacent unit must not get a move order")
		}
	})
}

// TestT4TickLoadsBoardsAfterShrink pins the shrink then store sequence for an adjacent unit.
func TestT4TickLoadsBoardsAfterShrink(t *testing.T) {
	s, g := newClickScreen(t)
	apc := t4Carrier(g, assets.ClnTrans, 1, 5, 5, game.FlagMobileLandUnit|game.FlagSelectable, 2)
	squad := t4Passenger(g, assets.Infantry, 2, 6, 5)
	s.beginLoad(squad, apc)

	s.tickLoads() // adjacent: the suck up starts

	if !s.effects.loading[squad].shrinking {
		t.Fatal("an adjacent unit should start shrinking on the first tick")
	}

	if len(apc.Carried) != 0 {
		t.Fatal("the unit must not board before it has shrunk")
	}

	for step := 1; step <= loadShrinkMax; step++ {
		t4ForceShrinkStep(s)
		s.tickLoads()

		if int(squad.Unfold) != step {
			t.Fatalf("after %d steps Unfold = %d, want %d", step, squad.Unfold, step)
		}

		if len(apc.Carried) != 0 {
			t.Fatalf("boarded early at step %d", step)
		}
	}

	t4ForceShrinkStep(s)
	s.tickLoads()

	if len(apc.Carried) != 1 || apc.Carried[0] != squad || squad.Parent != apc {
		t.Fatal("the fully shrunk unit should be stored in the carrier")
	}

	if squad.Unfold != 0 {
		t.Errorf("Unfold = %d after boarding, want it reset to 0", squad.Unfold)
	}

	if len(s.effects.loading) != 0 {
		t.Error("the finished job must be removed")
	}

	if len(g.Units.Cells.At(6, 5)) != 0 {
		t.Error("the boarded unit must have left the map")
	}
}

// TestT4TickLoadsWaitsForClock pins that the shrink only advances on the expand clock.
func TestT4TickLoadsWaitsForClock(t *testing.T) {
	s, g := newClickScreen(t)
	apc := t4Carrier(g, assets.ClnTrans, 1, 5, 5, game.FlagMobileLandUnit|game.FlagSelectable, 2)
	squad := t4Passenger(g, assets.Infantry, 2, 6, 5)
	s.beginLoad(squad, apc)
	s.tickLoads()

	s.effects.loadStamp = platform.Ticks() // a frame was just consumed: none has elapsed
	s.tickLoads()

	if squad.Unfold != 0 {
		t.Errorf("Unfold = %d without an elapsed frame, want 0", squad.Unfold)
	}
}

// TestT4TickLoadsDropsDeadAndStranded pins when a load job is abandoned.
func TestT4TickLoadsDropsDeadAndStranded(t *testing.T) {
	t.Run("passenger destroyed", func(t *testing.T) {
		s, g := newClickScreen(t)
		apc := t4Carrier(g, assets.ClnTrans, 1, 5, 5, game.FlagMobileLandUnit|game.FlagSelectable, 2)
		squad := t4Passenger(g, assets.Infantry, 2, 6, 5)
		s.beginLoad(squad, apc)
		squad.Hits = 0
		s.tickLoads()

		if len(s.effects.loading) != 0 {
			t.Error("a dead passenger must drop its job")
		}
	})

	t.Run("carrier destroyed", func(t *testing.T) {
		s, g := newClickScreen(t)
		apc := t4Carrier(g, assets.ClnTrans, 1, 5, 5, game.FlagMobileLandUnit|game.FlagSelectable, 2)
		apc.Hits = 0
		squad := t4Passenger(g, assets.Infantry, 2, 6, 5)
		s.beginLoad(squad, apc)
		s.tickLoads()

		if len(s.effects.loading) != 0 {
			t.Error("a dead carrier must drop the job")
		}
	})

	t.Run("far unit with no route is dropped", func(t *testing.T) {
		s, g := newClickScreen(t)
		apc := t4Carrier(g, assets.ClnTrans, 1, 5, 5, game.FlagMobileLandUnit|game.FlagSelectable, 2)
		squad := t4Passenger(g, assets.Infantry, 2, 12, 12)
		s.effects.loading = map[*game.UnitInfo]*loadJob{squad: {building: apc}} // never ordered to move
		s.tickLoads()

		if len(s.effects.loading) != 0 {
			t.Error("a unit that is neither adjacent nor travelling cannot reach the carrier")
		}
	})

	t.Run("far unit still driving keeps its job", func(t *testing.T) {
		s, g := newClickScreen(t)
		apc := t4Carrier(g, assets.ClnTrans, 1, 5, 5, game.FlagMobileLandUnit|game.FlagSelectable, 2)
		squad := t4Passenger(g, assets.Infantry, 2, 12, 12)
		s.beginLoad(squad, apc)
		s.tickLoads()

		if s.effects.loading[squad] == nil {
			t.Error("a travelling unit keeps its job")
		}
	})
}

// TestT4TickLoadsEmptyIsNoop pins that an empty loading set does nothing.
func TestT4TickLoadsEmptyIsNoop(t *testing.T) {
	s, _ := newClickScreen(t)
	s.tickLoads()

	if len(s.effects.loading) != 0 {
		t.Error("nothing to load must stay nothing")
	}
}

// TestT4LoadPickAcceptance pins the click on a transport while a load pick is armed.
func TestT4LoadPickAcceptance(t *testing.T) {
	land := game.FlagMobileLandUnit | game.FlagSelectable
	store := game.FlagStationary | game.FlagBuilding | game.FlagSelectable
	cases := []struct {
		name      string
		carrier   assets.UnitType
		flags     game.Flags
		capacity  int
		passenger assets.UnitType
		passX     int
		full      bool
		loads     bool
	}{
		{"apc takes infantry", assets.ClnTrans, land, 2, assets.Infantry, 6, false, true},
		{"apc takes commando", assets.ClnTrans, land, 2, assets.Commando, 6, false, true},
		{"apc refuses tank", assets.ClnTrans, land, 2, assets.Tank, 6, false, false},
		{"barracks refuses tank", assets.Barracks, store, 4, assets.Tank, 7, false, false},
		{"barracks takes infantry", assets.Barracks, store, 4, assets.Infantry, 7, false, true},
		{"sea transport takes tank", assets.SeaTrans, game.FlagMobileSeaUnit | game.FlagSelectable, 4, assets.Tank, 6, false, true},
		{"full apc refuses", assets.ClnTrans, land, 1, assets.Infantry, 6, true, false},
		{"not adjacent refuses", assets.ClnTrans, land, 2, assets.Infantry, 9, false, false},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			s, g := newClickScreen(t)
			carrier := t4Carrier(g, c.carrier, 1, 5, 5, c.flags, c.capacity)

			if c.full {
				carrier.Carried = append(carrier.Carried, t4Passenger(g, assets.Commando, 3, 1, 1))
			}

			passenger := t4Passenger(g, c.passenger, 2, c.passX, 5)
			s.armLoad(passenger)

			if !s.tryTransportTarget(5, 5) {
				t.Fatal("the click must be consumed")
			}

			if s.transportArmed() {
				t.Error("the pick must disarm after the click")
			}

			loading := s.effects.loading[passenger] != nil

			if loading != c.loads {
				t.Fatalf("load job started = %v, want %v", loading, c.loads)
			}

			if c.loads && s.selected != carrier {
				t.Error("a started load selects the carrier")
			}

			if !c.loads && s.banner.msg == "" {
				t.Error("a refused load should say why")
			}
		})
	}
}

// TestT4LoadPickNoContainer pins that clicking an empty cell refuses and disarms.
func TestT4LoadPickNoContainer(t *testing.T) {
	s, g := newClickScreen(t)
	squad := t4Passenger(g, assets.Infantry, 2, 6, 5)
	s.armLoad(squad)

	if !s.tryTransportTarget(5, 5) {
		t.Fatal("the click must be consumed")
	}

	if len(s.effects.loading) != 0 || s.transportArmed() {
		t.Error("no container means no job and a disarmed pick")
	}
}

// TestT4TryTransportTargetUnarmed pins that an unarmed screen does not consume clicks.
func TestT4TryTransportTargetUnarmed(t *testing.T) {
	s, _ := newClickScreen(t)

	if s.tryTransportTarget(5, 5) {
		t.Error("nothing armed: the click must pass through")
	}
}

// t4Engineer deploys an engineer with some hit points for the builder at work checks.
func t4Engineer(g *sim.Game, id, gridX, gridY int) *game.UnitInfo {
	flags := game.FlagMobileLandUnit | game.FlagSelectable | game.FlagConstructorUnit
	u := mkUnit(g, assets.Engineer, id, gridX, gridY, flags)
	u.Values.Set(game.AttribHits, 10)
	u.Hits = 10

	return u
}

// TestT4BuilderAtWorkIsNeverLoaded pins that a builder in its build pose, or a bulldozer clearing, is not stored.
func TestT4BuilderAtWorkIsNeverLoaded(t *testing.T) {
	setups := []struct {
		name string
		work func(u *game.UnitInfo)
	}{
		{"engineer in build pose", func(u *game.UnitInfo) { u.StartBuildPose(u.GridX, u.GridY, 1) }},
		{"engineer on a build order", func(u *game.UnitInfo) { u.Order = game.OrderBuild }},
		{"engineer clearing rubble", func(u *game.UnitInfo) { u.Order = game.OrderClear }},
	}

	for _, c := range setups {
		t.Run(c.name+" refused by unit pick", func(t *testing.T) {
			s, g := newClickScreen(t)
			boat := t4Carrier(g, assets.SeaTrans, 1, 5, 5, game.FlagMobileSeaUnit|game.FlagSelectable, 4)
			eng := t4Engineer(g, 2, 6, 5)
			c.work(eng)

			s.armLoad(eng)

			if !s.tryTransportTarget(5, 5) {
				t.Fatal("the click must be consumed")
			}

			if s.effects.loading[eng] != nil || len(boat.Carried) != 0 {
				t.Error("a builder at work must not be loaded")
			}
		})

		t.Run(c.name+" selected by building load", func(t *testing.T) {
			s, g := newClickScreen(t)
			depot := t4Carrier(g, assets.Depot, 1, 8, 8, game.FlagStationary|game.FlagBuilding|game.FlagSelectable, 4)
			eng := t4Engineer(g, 2, 6, 5)
			c.work(eng)

			s.armLoadInto(depot)

			if !s.tryTransportTarget(6, 5) {
				t.Fatal("the click must be consumed")
			}

			if s.effects.loading[eng] != nil {
				t.Error("a builder at work must not start loading")
			}

			if s.selected != eng {
				t.Error("the Load action selects a unit it cannot take")
			}
		})
	}

	t.Run("control: idle engineer is loaded", func(t *testing.T) {
		s, g := newClickScreen(t)
		boat := t4Carrier(g, assets.SeaTrans, 1, 5, 5, game.FlagMobileSeaUnit|game.FlagSelectable, 4)
		eng := t4Engineer(g, 2, 6, 5)
		eng.Order = game.OrderAwait
		s.armLoad(eng)
		s.tryTransportTarget(5, 5)

		if s.effects.loading[eng] == nil || s.effects.loading[eng].building != boat {
			t.Error("an idle engineer beside a sea transport should be loaded")
		}
	})
}

// TestT4LoadIntoPick pins the building initiated Load pick.
func TestT4LoadIntoPick(t *testing.T) {
	depotFlags := game.FlagStationary | game.FlagBuilding | game.FlagSelectable

	t.Run("empty cell refuses", func(t *testing.T) {
		s, g := newClickScreen(t)
		depot := t4Carrier(g, assets.Depot, 1, 8, 8, depotFlags, 4)
		s.armLoadInto(depot)

		if !s.tryTransportTarget(2, 2) {
			t.Fatal("the click must be consumed")
		}

		if len(s.effects.loading) != 0 || s.transportArmed() {
			t.Error("no unit means no job and a disarmed pick")
		}
	})

	t.Run("disabled unit refused", func(t *testing.T) {
		s, g := newClickScreen(t)
		depot := t4Carrier(g, assets.Depot, 1, 8, 8, depotFlags, 4)
		tank := t4Passenger(g, assets.Tank, 2, 2, 2)
		tank.Order = game.OrderDisable
		s.armLoadInto(depot)
		s.tryTransportTarget(2, 2)

		if len(s.effects.loading) != 0 {
			t.Error("a disabled unit must not be loaded")
		}
	})

	t.Run("wrong class is selected not loaded", func(t *testing.T) {
		s, g := newClickScreen(t)
		depot := t4Carrier(g, assets.Depot, 1, 8, 8, depotFlags, 4)
		squad := t4Passenger(g, assets.Infantry, 2, 2, 2) // a depot takes vehicles only
		s.armLoadInto(depot)
		s.tryTransportTarget(2, 2)

		if len(s.effects.loading) != 0 {
			t.Error("infantry must not enter a depot")
		}

		if s.selected != squad {
			t.Error("the refused unit is selected instead")
		}
	})

	t.Run("vehicle drives over and loads", func(t *testing.T) {
		s, g := newClickScreen(t)
		depot := t4Carrier(g, assets.Depot, 1, 8, 8, depotFlags, 4)
		tank := t4Passenger(g, assets.Tank, 2, 2, 2)
		s.armLoadInto(depot)
		s.tryTransportTarget(2, 2)

		if s.effects.loading[tank] == nil || s.effects.loading[tank].building != depot {
			t.Fatal("a vehicle should start loading into the depot")
		}

		if !s.isMoving(tank) {
			t.Error("a far vehicle drives to the depot first")
		}

		if s.selected != depot {
			t.Error("a started load selects the depot")
		}
	})
}
