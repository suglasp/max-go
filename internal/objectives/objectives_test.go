// objectives_test.go : drives every mission objective to each of its verdicts.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package objectives

import (
	"testing"

	"github.com/suglasp/max-go/internal/assets"
	"github.com/suglasp/max-go/internal/economy"
	"github.com/suglasp/max-go/internal/game"
	"github.com/suglasp/max-go/internal/winloss"
)

// board is a made up battlefield an objective can be asked about, so each mission's rule
// can be driven to Won, Lost and Pending without standing a real game up.
type board struct {
	units       map[int]map[assets.UnitType]int // team -> type -> count in service
	casualties  map[int]map[assets.UnitType]int // team -> type -> lost
	cargo       map[int]map[economy.CargoType]int
	building    map[int]map[assets.UnitType]int // under construction
	running     map[int]map[assets.UnitType]int // powered buildings
	mining      map[int]map[economy.CargoType]int
	consumption map[int]map[economy.CargoType]int
	capacity    map[int]int
	damaged     map[int]map[assets.UnitType]int
	reduced     map[int]map[assets.UnitType]int
	research    map[int]map[int]int
	marked      map[int]map[assets.UnitType]int // highest Mk level fielded
	experienced map[int]map[assets.UnitType]bool
	points      map[int]int
	turn        int
	turnLimit   int
	dead        map[int]bool // the team can no longer fight, produce or build
}

func newBoard() *board {
	return &board{
		units:       map[int]map[assets.UnitType]int{},
		casualties:  map[int]map[assets.UnitType]int{},
		cargo:       map[int]map[economy.CargoType]int{},
		building:    map[int]map[assets.UnitType]int{},
		running:     map[int]map[assets.UnitType]int{},
		mining:      map[int]map[economy.CargoType]int{},
		consumption: map[int]map[economy.CargoType]int{},
		capacity:    map[int]int{},
		damaged:     map[int]map[assets.UnitType]int{},
		reduced:     map[int]map[assets.UnitType]int{},
		research:    map[int]map[int]int{},
		marked:      map[int]map[assets.UnitType]int{},
		experienced: map[int]map[assets.UnitType]bool{},
		points:      map[int]int{},
		dead:        map[int]bool{},
	}
}

// give puts n units of a type on a team's side of the board.
func (b *board) give(team int, t assets.UnitType, n int) *board {
	if b.units[team] == nil {
		b.units[team] = map[assets.UnitType]int{}
	}

	b.units[team][t] = n

	return b
}

func (b *board) lost(team int, t assets.UnitType, n int) *board {
	if b.casualties[team] == nil {
		b.casualties[team] = map[assets.UnitType]int{}
	}

	b.casualties[team][t] = n

	return b
}

func (b *board) store(team int, c economy.CargoType, n int) *board {
	if b.cargo[team] == nil {
		b.cargo[team] = map[economy.CargoType]int{}
	}

	b.cargo[team][c] = n

	return b
}

func (b *board) kill(team int) *board {
	b.dead[team] = true

	return b
}

func (b *board) HasAttackPower(team int, _ game.UnitClass) bool { return !b.dead[team] }
func (b *board) HasProductionInfrastructure(team int) bool      { return !b.dead[team] }
func (b *board) HasConstructionInfrastructure(team int) bool    { return !b.dead[team] }
func (b *board) CountReadyUnits(team int, t assets.UnitType) int {
	return b.units[team][t]
}
func (b *board) CountCasualties(team int, t assets.UnitType) int { return b.casualties[team][t] }
func (b *board) StoredCargo(team int, c economy.CargoType) int   { return b.cargo[team][c] }
func (b *board) RawMiningCapacity(team int) int                   { return b.capacity[team] }
func (b *board) TotalMining(team int, c economy.CargoType) int   { return b.mining[team][c] }
func (b *board) TotalConsumption(team int, c economy.CargoType) int {
	return b.consumption[team][c]
}
func (b *board) RunningBuildings(team int, t assets.UnitType) int { return b.running[team][t] }
func (b *board) UnitsBeingConstructed(team int, t assets.UnitType) int {
	return b.building[team][t]
}
func (b *board) HasUnitAboveMarkLevel(team int, t assets.UnitType, mark int) bool {
	return b.marked[team][t] > mark
}
func (b *board) HasUnitExperience(team int, t assets.UnitType) bool {
	return b.experienced[team][t]
}
func (b *board) CountDamagedUnits(team int, t assets.UnitType) int { return b.damaged[team][t] }
func (b *board) CountUnitsWithReducedAttrib(team int, t assets.UnitType, _ game.Attrib) int {
	return b.reduced[team][t]
}
func (b *board) ResearchLevel(team, topic int) int { return b.research[team][topic] }
func (b *board) Points(team int) int               { return b.points[team] }
func (b *board) TurnValue() int                    { return b.turn }
func (b *board) TurnVictoryLimit() int             { return b.turnLimit }

// alive gives a team the bare minimum that keeps it in the fight, so a test can be about
// the objective rather than about the liveness preamble.
func (b *board) alive(team int) *board {
	b.dead[team] = false

	return b
}

// TestForKeysOffTheMissionIndex checks the lookup: the retail header's mission_index is
// the mission's identity, a 0 means no scripted objective, and a category with no table
// (a multiplayer scenario) never has one.
func TestForKeysOffTheMissionIndex(t *testing.T) {
	if For(game.MissionCampaign, 1) == nil {
		t.Fatal("campaign 1 has no objective")
	}

	if For(game.MissionTraining, 15) == nil {
		t.Fatal("training 15 has no objective")
	}

	if For(game.MissionScenario, 18) == nil {
		t.Fatal("scenario 18 has no objective")
	}

	if For(game.MissionCampaign, 0) != nil {
		t.Error("mission_index 0 must mean the generic rules")
	}

	if For(game.MissionCampaign, 6) != nil {
		t.Error("campaign 6 carries no bespoke objective in the original either")
	}

	if For(game.MissionMultiScenario, 1) != nil {
		t.Error("a multiplayer scenario has no scripted objective")
	}

	if For(game.MissionNone, 1) != nil {
		t.Error("a skirmish has no scripted objective")
	}
}

// TestLivenessPreamble pins the shape every bespoke objective shares: a team that can no
// longer fight, produce or build is LOST whatever the objective says, and a surviving
// OPPONENT is generic (it plays for points like anyone else).
func TestLivenessPreamble(t *testing.T) {
	b := newBoard().alive(teamRed).alive(teamGreen)
	b.give(teamGreen, assets.Greenhse, 1) // campaign 4: still pending

	if got := razeTheEcoSpheres(b, teamRed); got != winloss.StatusPending {
		t.Fatalf("a live player mid mission = %v, want Pending", got)
	}

	if got := razeTheEcoSpheres(b, teamGreen); got != winloss.StatusGeneric {
		t.Fatalf("a live opponent = %v, want Generic", got)
	}

	b.kill(teamRed)

	if got := razeTheEcoSpheres(b, teamRed); got != winloss.StatusLost {
		t.Fatalf("a player who cannot fight, build or produce = %v, want Lost", got)
	}

	b.kill(teamGreen)

	if got := razeTheEcoSpheres(b, teamGreen); got != winloss.StatusLost {
		t.Fatalf("a beaten opponent = %v, want Lost", got)
	}
}

// TestCampaign1 is the headline: Islands in the Sun is won by destroying the enemy's eco
// spheres, and LOST by destroying the research centre you were sent to capture - so
// killing everything the enemy owns loses the mission rather than winning it.
func TestCampaign1(t *testing.T) {
	b := newBoard().alive(teamRed).alive(teamGreen)
	b.give(teamGreen, assets.Research, 1).give(teamGreen, assets.Greenhse, 2)

	if got := campaign1(b, teamRed); got != winloss.StatusPending {
		t.Fatalf("eco spheres still standing = %v, want Pending", got)
	}

	b.give(teamGreen, assets.Greenhse, 0)

	if got := campaign1(b, teamRed); got != winloss.StatusWon {
		t.Fatalf("eco spheres gone, lab intact = %v, want Won", got)
	}

	b.give(teamGreen, assets.Research, 0)

	if got := campaign1(b, teamRed); got != winloss.StatusLost {
		t.Fatalf("the lab was destroyed too = %v, want Lost", got)
	}
}

// TestCampaign2 checks Heart of the Matter: survive to the turn limit with the research
// centre intact.
func TestCampaign2(t *testing.T) {
	b := newBoard().alive(teamRed)
	b.turn, b.turnLimit = 10, 30

	if got := campaign2(b, teamRed); got != winloss.StatusPending {
		t.Fatalf("turn 10 of 30 = %v, want Pending", got)
	}

	b.turn = 30

	if got := campaign2(b, teamRed); got != winloss.StatusWon {
		t.Fatalf("the turn limit reached = %v, want Won", got)
	}

	b.lost(teamRed, assets.Research, 1)

	if got := campaign2(b, teamRed); got != winloss.StatusLost {
		t.Fatalf("the research centre was lost = %v, want Lost", got)
	}
}

// TestCampaign3 checks Element of Import: come away with two supply trucks, and do not
// destroy the trucks (or lose the commando) trying.
func TestCampaign3(t *testing.T) {
	b := newBoard().alive(teamRed)
	b.give(teamGreen, assets.SplyTrck, 2).give(teamRed, assets.Commando, 1)

	if got := campaign3(b, teamRed); got != winloss.StatusPending {
		t.Fatalf("the trucks are still the enemy's = %v, want Pending", got)
	}

	b.give(teamRed, assets.SplyTrck, 2).give(teamGreen, assets.SplyTrck, 0)

	if got := campaign3(b, teamRed); got != winloss.StatusWon {
		t.Fatalf("two trucks taken = %v, want Won", got)
	}

	b.give(teamRed, assets.SplyTrck, 1).give(teamGreen, assets.SplyTrck, 0)

	if got := campaign3(b, teamRed); got != winloss.StatusLost {
		t.Fatalf("only one truck left in the world = %v, want Lost", got)
	}

	b.give(teamGreen, assets.SplyTrck, 1).give(teamRed, assets.Commando, 0)

	if got := campaign3(b, teamRed); got != winloss.StatusLost {
		t.Fatalf("no commando left to take them = %v, want Lost", got)
	}
}

// TestCampaign8 checks Bright Hope: capture four alien units before the clock runs out.
func TestCampaign8(t *testing.T) {
	b := newBoard().alive(teamRed)
	b.turn, b.turnLimit = 10, 100
	b.give(teamGreen, assets.AlnTank, 2).give(teamGreen, assets.AlnPlane, 2)

	if got := campaign8(b, teamRed); got != winloss.StatusPending {
		t.Fatalf("the aliens are still the enemy's = %v, want Pending", got)
	}

	b.give(teamRed, assets.AlnTank, 2).give(teamRed, assets.AlnPlane, 2)
	b.give(teamGreen, assets.AlnTank, 0).give(teamGreen, assets.AlnPlane, 0)

	if got := campaign8(b, teamRed); got != winloss.StatusGeneric {
		t.Fatalf("four aliens captured = %v, want Generic (the objective is met)", got)
	}

	b.give(teamRed, assets.AlnPlane, 0) // three left: two destroyed somewhere

	if got := campaign8(b, teamRed); got != winloss.StatusLost {
		t.Fatalf("too few aliens left in the world = %v, want Lost", got)
	}

	b.give(teamGreen, assets.AlnPlane, 2) // four exist again, but two are still the enemy's
	b.turn = 100

	if got := campaign8(b, teamRed); got != winloss.StatusLost {
		t.Fatalf("out of time with the aliens still out there = %v, want Lost", got)
	}
}

// TestScenarioObjectives covers One Bullet (get ahead on points) and Covert (raze the
// research centres).
func TestScenarioObjectives(t *testing.T) {
	b := newBoard().alive(teamRed)
	b.points[teamRed], b.points[teamGreen] = 3, 5

	if got := scenario9(b, teamRed); got != winloss.StatusPending {
		t.Fatalf("behind on points = %v, want Pending", got)
	}

	b.points[teamRed] = 6

	if got := scenario9(b, teamRed); got != winloss.StatusWon {
		t.Fatalf("ahead on points = %v, want Won", got)
	}

	c := newBoard().alive(teamRed)
	c.give(teamGreen, assets.Research, 1)

	if got := scenario18(c, teamRed); got != winloss.StatusPending {
		t.Fatalf("a research centre still stands = %v, want Pending", got)
	}

	c.give(teamGreen, assets.Research, 0)

	if got := scenario18(c, teamRed); got != winloss.StatusWon {
		t.Fatalf("the research centres are gone = %v, want Won", got)
	}
}

// TestTrainingObjectives walks each of the fifteen lessons to its win, and to its loss
// where it has one. A training mission IS its objective, so this is the whole category.
func TestTrainingObjectives(t *testing.T) {
	tests := []struct {
		name  string
		o     Objective
		setup func(b *board)
		want  winloss.TeamStatus
	}{
		{"1 pending: nothing stored yet", training1, func(b *board) {
			b.give(teamRed, assets.MiningSt, 1).give(teamRed, assets.Engineer, 1)
		}, winloss.StatusPending},
		{"1 won: raw and fuel in store with somewhere to put them", training1, func(b *board) {
			b.give(teamRed, assets.MiningSt, 1).give(teamRed, assets.Engineer, 1).give(teamRed, assets.ADump, 1)
			b.store(teamRed, economy.CargoRaw, 5).store(teamRed, economy.CargoFuel, 5)
		}, winloss.StatusWon},
		{"1 lost: the mining station is gone", training1, func(b *board) {
			b.give(teamRed, assets.Engineer, 1)
		}, winloss.StatusLost},

		{"2 won: a plant and a turret", training2, func(b *board) {
			b.give(teamRed, assets.LightPlt, 1).give(teamRed, assets.GunTurrt, 1)
		}, winloss.StatusWon},
		{"2 pending: a builder and something to build with", training2, func(b *board) {
			b.give(teamRed, assets.Engineer, 1).give(teamRed, assets.Constrct, 1)
		}, winloss.StatusPending},
		{"2 lost: nothing left to build with", training2, func(b *board) {}, winloss.StatusLost},

		{"3 won: a heavy plant and three scouts", training3, func(b *board) {
			b.give(teamRed, assets.LandPlt, 1).give(teamRed, assets.Scout, 3)
		}, winloss.StatusWon},
		{"3 pending: on the way", training3, func(b *board) {
			b.give(teamRed, assets.LightPlt, 1).give(teamRed, assets.MiningSt, 1).give(teamRed, assets.Constrct, 1)
		}, winloss.StatusPending},

		{"4 won: mining capacity reached", training4, func(b *board) {
			b.capacity[teamRed] = 22
		}, winloss.StatusWon},
		{"4 pending: still surveying", training4, func(b *board) {
			b.capacity[teamRed] = 16
			b.give(teamRed, assets.Constrct, 1).give(teamRed, assets.MiningSt, 1)
		}, winloss.StatusPending},

		{"5 won: the enemy mine is rubble", training5, func(b *board) {}, winloss.StatusWon},
		{"5 pending: it still stands", training5, func(b *board) {
			b.give(teamGreen, assets.MiningSt, 1)
		}, winloss.StatusPending},

		{"6 won: the turret is going up", training6, func(b *board) {
			b.building[teamRed] = map[assets.UnitType]int{assets.GunTurrt: 1}
		}, winloss.StatusWon},

		{"7 won: two plants running on a fuel surplus", training7, func(b *board) {
			b.running[teamRed] = map[assets.UnitType]int{assets.LightPlt: 1, assets.LandPlt: 1}
			b.mining[teamRed] = map[economy.CargoType]int{economy.CargoFuel: 6}
			b.consumption[teamRed] = map[economy.CargoType]int{economy.CargoFuel: 4}
		}, winloss.StatusWon},
		{"7 lost: no plants and no way to mine", training7, func(b *board) {}, winloss.StatusLost},

		{"8 won: gold in store", training8, func(b *board) {
			b.store(teamRed, economy.CargoGold, 1)
		}, winloss.StatusWon},
		{"8 pending: no gold yet", training8, func(b *board) {}, winloss.StatusPending},

		{"9 won: a tank above Mk I", training9, func(b *board) {
			b.marked[teamRed] = map[assets.UnitType]int{assets.Tank: 2}
		}, winloss.StatusWon},
		{"9 pending: still Mk I", training9, func(b *board) {
			b.marked[teamRed] = map[assets.UnitType]int{assets.Tank: 1}
		}, winloss.StatusPending},

		{"10 won: a point on the board", training10, func(b *board) {
			b.points[teamRed] = 1
		}, winloss.StatusWon},
		{"10 pending: no points yet", training10, func(b *board) {}, winloss.StatusPending},

		{"11 won: a blooded commando", training11, func(b *board) {
			b.experienced[teamRed] = map[assets.UnitType]bool{assets.Commando: true}
		}, winloss.StatusWon},
		{"11 pending: the hall, the housing and the mine still stand", training11, func(b *board) {
			b.give(teamRed, assets.TrainHal, 1).give(teamRed, assets.Habitat, 1).give(teamRed, assets.MiningSt, 1)
		}, winloss.StatusPending},
		{"11 lost: no way to train one", training11, func(b *board) {}, winloss.StatusLost},

		{"12 won: a mine laid and an enemy mine cleared", training12, func(b *board) {
			b.give(teamRed, assets.LandMine, 1).lost(teamGreen, assets.LandMine, 1)
		}, winloss.StatusWon},
		{"12 pending: only half done", training12, func(b *board) {
			b.give(teamRed, assets.LandMine, 1)
		}, winloss.StatusPending},

		{"13 won: three tanks, none damaged", training13, func(b *board) {
			b.give(teamRed, assets.Tank, 3)
		}, winloss.StatusWon},
		{"13 pending: one still damaged", training13, func(b *board) {
			b.give(teamRed, assets.Tank, 3)
			b.damaged[teamRed] = map[assets.UnitType]int{assets.Tank: 1}
		}, winloss.StatusPending},
		{"13 lost: a tank died instead of being repaired", training13, func(b *board) {
			b.give(teamRed, assets.Tank, 2).lost(teamRed, assets.Tank, 1)
		}, winloss.StatusLost},

		{"14 won: three tanks, all loaded", training14, func(b *board) {
			b.give(teamRed, assets.Tank, 3)
		}, winloss.StatusWon},
		{"14 pending: one still short of ammo", training14, func(b *board) {
			b.give(teamRed, assets.Tank, 3)
			b.reduced[teamRed] = map[assets.UnitType]int{assets.Tank: 1}
		}, winloss.StatusPending},
		{"14 lost: a tank died instead of being reloaded", training14, func(b *board) {
			b.give(teamRed, assets.Tank, 2).lost(teamRed, assets.Tank, 1)
		}, winloss.StatusLost},

		{"15 won: armor research is in", training15, func(b *board) {
			b.research[teamRed] = map[int]int{game.ResearchArmor: 1}
		}, winloss.StatusWon},
		{"15 pending: the centre is still working", training15, func(b *board) {
			b.give(teamRed, assets.Research, 1)
		}, winloss.StatusPending},
		{"15 lost: the research centre is gone", training15, func(b *board) {}, winloss.StatusLost},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			b := newBoard().alive(teamRed).alive(teamGreen)
			tc.setup(b)

			if got := tc.o(b, teamRed); got != tc.want {
				t.Fatalf("%s = %v, want %v", tc.name, got, tc.want)
			}
		})
	}
}

// TestTraining6Pending covers the other half of Material Transfer: nothing under
// construction yet.
func TestTraining6Pending(t *testing.T) {
	b := newBoard().alive(teamRed)

	if got := training6(b, teamRed); got != winloss.StatusPending {
		t.Fatalf("no turret under construction = %v, want Pending", got)
	}
}

// TestBindGivesTheSimItsHook checks the adapter: an objective becomes the condition
// function the sim's win/loss evaluation takes, and a nil objective binds to nothing (so
// the game keeps the generic rules).
func TestBindGivesTheSimItsHook(t *testing.T) {
	b := newBoard().alive(teamRed).alive(teamGreen)
	b.give(teamGreen, assets.Greenhse, 1)

	fn := Bind(For(game.MissionCampaign, 4), b)
	if fn == nil {
		t.Fatal("campaign 4 bound to no condition function")
	}

	status, ok := fn(teamRed)
	if !ok || status != winloss.StatusPending {
		t.Fatalf("bound condition = %v/%v, want Pending/true", status, ok)
	}

	if Bind(nil, b) != nil {
		t.Error("a mission with no objective must bind to nil, leaving the generic rules")
	}
}
