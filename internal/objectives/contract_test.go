// contract_test.go : testing unit for the objectives against the real sim.Game
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package objectives_test

import (
	"testing"

	"github.com/suglasp/max-go/internal/assets"
	"github.com/suglasp/max-go/internal/game"
	"github.com/suglasp/max-go/internal/objectives"
	"github.com/suglasp/max-go/internal/sim"
	"github.com/suglasp/max-go/internal/winloss"
	"github.com/suglasp/max-go/internal/world"
)

const (
	teamRed   = 0
	teamGreen = 1
)

// realGame builds a small all land game in which both teams own an armed scout, so each
// team passes the liveness preamble and the test is about the objective alone.
func realGame(t *testing.T) *sim.Game {
	t.Helper()

	m := &world.Map{}
	m.W, m.H = 16, 16
	m.Surface = make([]uint8, m.W*m.H)

	for i := range m.Surface {
		m.Surface[i] = world.SurfaceLand
	}

	p := &game.StartParams{}
	p.Teams[teamRed].Type = game.TeamPlayer
	p.Teams[teamGreen].Type = game.TeamComputer
	g := sim.New(p, m)

	guardRed := deploy(g, teamRed, assets.Scout, 100, 0, 0, game.FlagMobileLandUnit|game.FlagSelectable)
	guardRed.Ammo = 1
	guardGreen := deploy(g, teamGreen, assets.Scout, 101, 15, 15, game.FlagMobileLandUnit|game.FlagSelectable)
	guardGreen.Ammo = 1

	return g
}

// deploy puts a unit of a type on the board with the flags a real one would carry.
func deploy(g *sim.Game, team int, t assets.UnitType, id, cellX, cellY int, flags game.Flags) *game.UnitInfo {
	u := game.NewUnitInfo(t, team, id, 0)
	u.Flags = flags
	u.Values = &game.UnitValues{}
	u.InitState()
	g.Units.Deploy(u, cellX, cellY)

	return u
}

// building deploys a stationary building with an order and the state of a running one.
func building(g *sim.Game, team int, t assets.UnitType, id, cellX, cellY int, order game.Order) *game.UnitInfo {
	u := deploy(g, team, t, id, cellX, cellY, game.FlagStationary|game.FlagSelectable)
	u.Order = order
	u.State = game.OrderStateExecutingOrder

	return u
}

// verdict asks the real game what the mission says about the red team.
func verdict(t *testing.T, g *sim.Game, category game.MissionCategory, index int) winloss.TeamStatus {
	t.Helper()

	cond := objectives.Bind(objectives.For(category, index), g)
	if cond == nil {
		t.Fatalf("mission %d/%d has no bound objective", category, index)
	}

	status, handled := cond(teamRed)
	if !handled {
		t.Fatalf("mission %d/%d did not handle the red team", category, index)
	}

	return status
}

// TestRealGameSatisfiesTheQuery checks at compile time that the real game answers the objectives' board.
func TestRealGameSatisfiesTheQuery(t *testing.T) {
	var _ objectives.Query = (*sim.Game)(nil)

	g := realGame(t)

	if got := verdict(t, g, game.MissionTraining, 8); got != winloss.StatusPending {
		t.Errorf("a plain board = %v, want Pending", got)
	}
}

// TestTrainingMiningCapacityOnRealGame checks lesson 4: a powered station with capacity 22 wins, 16 does not.
func TestTrainingMiningCapacityOnRealGame(t *testing.T) {
	g := realGame(t)
	deploy(g, teamRed, assets.Constrct, 1, 2, 2, game.FlagMobileLandUnit|game.FlagSelectable)
	station := building(g, teamRed, assets.MiningSt, 2, 5, 5, game.OrderPowerOn)
	station.RawMiningMax = 16

	if got := verdict(t, g, game.MissionTraining, 4); got != winloss.StatusPending {
		t.Fatalf("capacity 16 = %v, want Pending", got)
	}

	station.RawMiningMax = 22

	if got := verdict(t, g, game.MissionTraining, 4); got != winloss.StatusWon {
		t.Fatalf("capacity 22 = %v, want Won", got)
	}

	station.Order = game.OrderPowerOff

	if got := verdict(t, g, game.MissionTraining, 4); got != winloss.StatusPending {
		t.Fatalf("capacity 22 but unpowered = %v, want Pending", got)
	}
}

// TestTrainingTurretUnderConstructionOnRealGame checks lesson 6: the turret on an engineer's order wins.
func TestTrainingTurretUnderConstructionOnRealGame(t *testing.T) {
	g := realGame(t)

	if got := verdict(t, g, game.MissionTraining, 6); got != winloss.StatusPending {
		t.Fatalf("nothing under construction = %v, want Pending", got)
	}

	engineer := deploy(g, teamRed, assets.Engineer, 1, 2, 2, game.FlagMobileLandUnit|game.FlagConstructorUnit|game.FlagSelectable)
	turret := game.NewUnitInfo(assets.GunTurrt, teamRed, 2, 0)
	turret.Flags = game.FlagStationary
	g.StartBuild(engineer, turret, 3, 2, 1)

	if got := verdict(t, g, game.MissionTraining, 6); got != winloss.StatusWon {
		t.Fatalf("turret on order = %v, want Won", got)
	}
}

// TestTrainingRunningPlantsOnRealGame checks lesson 7: two plants running on a fuel surplus win.
func TestTrainingRunningPlantsOnRealGame(t *testing.T) {
	g := realGame(t)
	building(g, teamRed, assets.LightPlt, 1, 2, 2, game.OrderPowerOn)
	building(g, teamRed, assets.LandPlt, 2, 6, 2, game.OrderPowerOn)
	building(g, teamRed, assets.PowerStn, 3, 10, 2, game.OrderPowerOn) // burns 6 fuel a turn
	mine := building(g, teamRed, assets.MiningSt, 4, 2, 8, game.OrderPowerOn)

	mine.FuelMining = 6 // exactly what the station burns: no surplus

	if got := verdict(t, g, game.MissionTraining, 7); got != winloss.StatusPending {
		t.Fatalf("no fuel surplus = %v, want Pending", got)
	}

	mine.FuelMining = 8

	if got := verdict(t, g, game.MissionTraining, 7); got != winloss.StatusWon {
		t.Fatalf("a fuel surplus = %v, want Won", got)
	}
}

// TestTrainingUpgradeOnRealGame checks lesson 9: a tank past Mk I wins.
func TestTrainingUpgradeOnRealGame(t *testing.T) {
	g := realGame(t)
	tank := deploy(g, teamRed, assets.Tank, 1, 2, 2, game.FlagMobileLandUnit|game.FlagSelectable)
	tank.Values.SetVersion(1)

	if got := verdict(t, g, game.MissionTraining, 9); got != winloss.StatusPending {
		t.Fatalf("a Mk I tank = %v, want Pending", got)
	}

	tank.Values.SetVersion(2)

	if got := verdict(t, g, game.MissionTraining, 9); got != winloss.StatusWon {
		t.Fatalf("an upgraded tank = %v, want Won", got)
	}
}

// TestTrainingPointsOnRealGame checks lesson 10: a point on the board wins.
func TestTrainingPointsOnRealGame(t *testing.T) {
	g := realGame(t)

	if got := verdict(t, g, game.MissionTraining, 10); got != winloss.StatusPending {
		t.Fatalf("no points = %v, want Pending", got)
	}

	g.TeamPoints[teamRed] = 1

	if got := verdict(t, g, game.MissionTraining, 10); got != winloss.StatusWon {
		t.Fatalf("one point = %v, want Won", got)
	}
}

// TestTrainingExperienceOnRealGame checks lesson 11: a blooded commando wins, and with no hall the lesson is lost.
func TestTrainingExperienceOnRealGame(t *testing.T) {
	g := realGame(t)

	if got := verdict(t, g, game.MissionTraining, 11); got != winloss.StatusLost {
		t.Fatalf("no hall, housing or mine = %v, want Lost", got)
	}

	building(g, teamRed, assets.TrainHal, 1, 2, 2, game.OrderPowerOn)
	building(g, teamRed, assets.Habitat, 2, 6, 2, game.OrderPowerOn)
	building(g, teamRed, assets.MiningSt, 3, 10, 2, game.OrderPowerOn)

	if got := verdict(t, g, game.MissionTraining, 11); got != winloss.StatusPending {
		t.Fatalf("the hall, housing and mine = %v, want Pending", got)
	}

	cmd := deploy(g, teamRed, assets.Commando, 4, 2, 8, game.FlagMobileLandUnit|game.FlagSelectable)
	cmd.Experience = 1

	if got := verdict(t, g, game.MissionTraining, 11); got != winloss.StatusWon {
		t.Fatalf("a blooded commando = %v, want Won", got)
	}
}

// TestTrainingMinesOnRealGame checks lesson 12: our laid mine plus a lost enemy mine wins.
func TestTrainingMinesOnRealGame(t *testing.T) {
	g := realGame(t)
	deploy(g, teamRed, assets.LandMine, 1, 2, 2, game.FlagStationary)

	if got := verdict(t, g, game.MissionTraining, 12); got != winloss.StatusPending {
		t.Fatalf("only half done = %v, want Pending", got)
	}

	g.TeamStats[teamGreen].Casualties = map[assets.UnitType]int{assets.LandMine: 1}

	if got := verdict(t, g, game.MissionTraining, 12); got != winloss.StatusWon {
		t.Fatalf("mine laid and enemy mine cleared = %v, want Won", got)
	}
}

// TestTrainingTankLossOnRealGame checks lesson 13: a tank lost to the repair lesson loses it.
func TestTrainingTankLossOnRealGame(t *testing.T) {
	g := realGame(t)

	for i := 0; i < 2; i++ {
		deploy(g, teamRed, assets.Tank, 1+i, 2+i*2, 2, game.FlagMobileLandUnit|game.FlagSelectable)
	}

	if got := verdict(t, g, game.MissionTraining, 13); got != winloss.StatusPending {
		t.Fatalf("two tanks = %v, want Pending", got)
	}

	g.TeamStats[teamRed].Casualties = map[assets.UnitType]int{assets.Tank: 1}

	if got := verdict(t, g, game.MissionTraining, 13); got != winloss.StatusLost {
		t.Fatalf("a tank lost = %v, want Lost", got)
	}
}

// TestTrainingResearchOnRealGame checks lesson 15: armor research at level 1 wins.
func TestTrainingResearchOnRealGame(t *testing.T) {
	g := realGame(t)
	building(g, teamRed, assets.Research, 1, 2, 2, game.OrderPowerOn)

	if got := verdict(t, g, game.MissionTraining, 15); got != winloss.StatusPending {
		t.Fatalf("research still working = %v, want Pending", got)
	}

	g.State.Teams[teamRed].Research[game.ResearchArmor].Level = 1

	if got := verdict(t, g, game.MissionTraining, 15); got != winloss.StatusWon {
		t.Fatalf("armor research in = %v, want Won", got)
	}
}

// TestCampaignTurnLimitOnRealGame checks campaign 2: surviving to the turn cap wins, losing the lab loses.
func TestCampaignTurnLimitOnRealGame(t *testing.T) {
	g := realGame(t)
	g.VictoryType = winloss.VictoryDuration
	g.VictoryLimit = 20
	g.State.TurnCounter = 5

	if got := verdict(t, g, game.MissionCampaign, 2); got != winloss.StatusPending {
		t.Fatalf("turn 5 of 20 = %v, want Pending", got)
	}

	g.State.TurnCounter = 20

	if got := verdict(t, g, game.MissionCampaign, 2); got != winloss.StatusWon {
		t.Fatalf("turn 20 of 20 = %v, want Won", got)
	}

	g.TeamStats[teamRed].Casualties = map[assets.UnitType]int{assets.Research: 1}

	if got := verdict(t, g, game.MissionCampaign, 2); got != winloss.StatusLost {
		t.Fatalf("lab destroyed = %v, want Lost", got)
	}
}
