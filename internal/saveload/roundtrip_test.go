// roundtrip_test.go : testing unit for the full Capture -> Save -> Load -> Restore -> Capture chain
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package saveload

import (
	"bytes"
	"fmt"
	"reflect"
	"sort"
	"testing"

	"github.com/suglasp/max-go/internal/assets"
	"github.com/suglasp/max-go/internal/economy"
	"github.com/suglasp/max-go/internal/game"
	"github.com/suglasp/max-go/internal/sim"
	"github.com/suglasp/max-go/internal/winloss"
)

// richMapName is the map file name the rich fixture is captured under.
const richMapName = "GREEN_1.WRL"

// newRichGame builds a game that holds a value in every field the snapshot persists.
func newRichGame(t *testing.T) *sim.Game {
	t.Helper()
	params := &game.StartParams{}
	params.Teams[0].Type = game.TeamPlayer
	params.Teams[1].Type = game.TeamComputer
	g := sim.New(params, landMap(24, 24))
	g.MakeUnit = func(team int, unitType assets.UnitType, flags game.Flags) *game.UnitInfo {
		u := game.NewUnitInfo(unitType, team, 0, 0)
		u.Flags = flags
		u.Values = &game.UnitValues{}
		u.Values.Set(game.AttribTurns, 3)
		u.Values.Set(game.AttribHits, 10)
		return u
	}
	mobile := game.FlagMobileLandUnit | game.FlagSelectable

	// A transport with a carried passenger (off the registry).
	transport := makeUnit(g, assets.ClnTrans, 0, 1, 5, 5, mobile)
	passenger := game.NewUnitInfo(assets.Tank, 0, 10, 0)
	passenger.Flags = mobile
	passenger.Values = &game.UnitValues{}
	passenger.Values.Set(game.AttribHits, 24)
	passenger.InitState()
	passenger.Parent = transport
	transport.Carried = append(transport.Carried, passenger)

	// An attacker with a target, a prior order, veterancy and a held route.
	attacker := makeUnit(g, assets.Tank, 0, 2, 3, 3, mobile)
	enemy := makeUnit(g, assets.Tank, 1, 3, 9, 9, mobile)
	attacker.AttackTarget = enemy
	attacker.PriorOrder = game.OrderSentry
	attacker.PriorState = game.OrderStateExecutingOrder
	attacker.Experience = 4
	attacker.DisabledReactionFire = true
	attacker.SpottedBy = 2
	attacker.Angle, attacker.TurretAngle, attacker.TurretTarget = 3, 5, 6
	g.SetHeldDestination(attacker, 8, 3)

	// A tank with an over budget move.
	mover := makeUnit(g, assets.Tank, 0, 4, 2, 2, mobile)
	g.SetPendingDestination(mover, 12, 13)

	// A constructor with a build order in flight.
	constructor := makeUnit(g, assets.Constrct, 0, 5, 5, 8, mobile|game.FlagConstructorUnit)
	product := game.NewUnitInfo(assets.PowGen, 0, 0, 0)
	product.Flags = game.FlagStationary
	product.Values = &game.UnitValues{}
	product.Values.Set(game.AttribHits, 40)
	product.Values.Set(game.AttribTurns, 4)
	order := g.StartBuild(constructor, product, 5, 8, 1)
	order.Progress = 2
	order.Started = true

	// An engineer laying a road line.
	engineer := makeUnit(g, assets.Engineer, 0, 6, 3, 12, mobile|game.FlagConstructorUnit)
	pathValues := &game.UnitValues{}
	pathValues.Set(game.AttribTurns, 1)
	g.RestoreBuildPath(engineer, assets.Road, game.FlagStationary|game.FlagGroundCover, pathValues, 1,
		[][2]int{{3, 12}, {4, 12}, {5, 12}}, 1)

	// A bulldozer with a clear job and two hazard markers.
	dozer := makeUnit(g, assets.Engineer, 0, 7, 10, 3, mobile)
	cones := makeUnit(g, assets.SmlCones, 0, 8, 10, 3, game.FlagStationary|game.FlagGroundCover)
	tape := makeUnit(g, assets.SmlTape, 0, 9, 10, 3, game.FlagStationary|game.FlagGroundCover)
	g.RestoreClearJob(dozer, 10, 3, 1, 3, []*game.UnitInfo{cones, tape})

	// A factory with a repeating queue.
	factory := makeUnit(g, assets.LandPlt, 0, 11, 14, 6, game.FlagStationary|game.FlagBuilding|game.FlagSelectable)
	factory.Storage = 1000
	factory.LastBuildRate = 2
	g.SetFactoryQueue(factory, []assets.UnitType{assets.Tank, assets.Scout}, 2, true)
	// A neighbouring generator keeps the factory powered: Restore idles a running consumer in a complex with no power source.
	makeUnit(g, assets.PowGen, 0, 19, 13, 6, game.FlagStationary)

	// A builder parked on the site of the structure it just finished (still on the registry, as in play).
	parked := game.NewUnitInfo(assets.Constrct, 0, 12, 0)
	parked.Flags = mobile | game.FlagConstructorUnit
	parked.Values = &game.UnitValues{}
	parked.Values.Set(game.AttribHits, 30)
	parked.InitState()
	g.Units.Deploy(parked, 17, 17)
	structure := makeUnit(g, assets.PowGen, 0, 13, 17, 17, game.FlagStationary)
	structure.HiddenUntilExpand = true
	g.RestoreBuildReady(parked, structure, assets.PowGen, 17, 17, 1)

	// A disabled commando and a raised bridge.
	commando := makeUnit(g, assets.Commando, 1, 14, 20, 20, mobile)
	commando.Order = game.OrderDisable
	g.RestoreDisabled(commando, 3)
	bridge := makeUnit(g, assets.Bridge, 0, 17, 21, 2, game.FlagStationary)
	bridge.BridgeRaise = 2

	// An adjacent generator and eco sphere form one complex with pooled stores.
	generator := game.NewUnitInfo(assets.PowGen, 0, 15, 0)
	generator.Flags = game.FlagStationary
	generator.Order = game.OrderPowerOn
	generator.State = game.OrderStateExecutingOrder
	generator.Values = &game.UnitValues{}
	generator.Values.Set(game.AttribHits, 40)
	g.PlaceBuilding(generator, 1, 18)
	eco := game.NewUnitInfo(assets.Greenhse, 0, 16, 0)
	eco.Flags = game.FlagBuilding | game.FlagStationary | game.FlagSelectable
	eco.Order = game.OrderPowerOn
	eco.State = game.OrderStateExecutingOrder
	eco.Values = &game.UnitValues{}
	eco.Values.Set(game.AttribHits, 20)
	g.PlaceBuilding(eco, 2, 18)
	complexOfEco := g.ComplexOf(eco)
	if complexOfEco == nil {
		t.Fatal("setup: the generator and eco sphere did not form a complex")
	}
	complexOfEco.Material, complexOfEco.Fuel, complexOfEco.Gold = 77, 50, 9

	// Fog memory: a scout explores a corner and walks home.
	scout := game.NewUnitInfo(assets.Scout, 0, 18, 0)
	scout.Flags = mobile
	scout.Values = &game.UnitValues{}
	scout.Values.Set(game.AttribScan, 3)
	scout.InitState()
	g.Units.Deploy(scout, 20, 12)
	g.RefreshVisibility()
	g.Units.Move(scout, 2, 20)
	g.RefreshVisibility()

	// Field state: spotted memory, rubble, the unit ID counter, the deposit map, loss memory.
	g.AddSpotted(1, attacker, 7, 7, false)
	g.RestoreRubble([]sim.RubbleCell{{X: 12, Y: 12, Large: true, Material: 30}, {X: 13, Y: 12, Material: 8}})
	g.SetNextBuilt(42)
	g.Resources = economy.NewResourceMap(24, 24)
	g.Resources.SetDeposit(5, 6, economy.CargoRaw, 14)
	g.Resources.SetDeposit(6, 6, economy.CargoFuel, 9)
	g.Resources.MarkSurveyed(5, 6, 0)
	g.RestoreLossSites(0, []sim.LossSite{{X: 4, Y: 5, Turn: 3}})
	g.RestoreLossSites(1, []sim.LossSite{{X: 15, Y: 16, Turn: 4}, {X: 16, Y: 16, Turn: 5}})

	// Match state: turn, scores, research, upgrades, clan, victory rule, AI plan, mission.
	g.State.TurnCounter = 9
	g.State.ActiveTeam = 1
	g.State.PlayerTeam = 1
	g.State.Phase = game.PhaseInGame
	g.State.PlayMode = game.PlaySimultaneous
	g.State.Difficulty = 3
	g.State.AIWinGoal[1] = 2
	g.State.AIStrategy[1] = 3
	g.State.Teams[0].FinishedTurn = true
	g.State.Teams[1].Eliminated = true
	g.State.Teams[0].Clan = 5
	g.State.Teams[0].Research[game.ResearchAttack] = game.ResearchTopic{Level: 2, TurnsToComplete: 6, Allocation: 3}
	armor := [game.AttribCount]int{}
	armor[game.AttribArmor] = 4
	g.State.Teams[0].Upgrades = map[assets.UnitType]*[game.AttribCount]int{assets.Tank: &armor}
	g.Gold = [game.TeamCount]int{120, 45}
	g.TeamPoints = [game.TeamCount]int{300, 150}
	g.TeamStats[0] = sim.TeamStats{
		FactoriesBuilt: 3, MinesBuilt: 2, BuildingsBuilt: 9, BuildingsLost: 1, UnitsBuilt: 14,
		UnitsLost: 5, GoldOnUpgrades: 120,
		Casualties: map[assets.UnitType]int{assets.Tank: 3, assets.Scout: 2},
	}
	g.MiningDifficulty = 2
	g.VictoryType = winloss.VictoryScore
	g.VictoryLimit = 50
	g.ScoreHistory[0][49] = 7
	g.ScoreHistory[1][0] = 3
	g.Mission = game.MissionTraining
	g.MissionIndex = 4

	// The random generator sits mid stream.
	game.RandomizerSetSeed(777)
	for i := 0; i < 11; i++ {
		game.RandomizerGetNext()
	}
	return g
}

// sortSnapshotLists orders every slice field of the snapshot by its printed form, because the unit registry iterates in random order.
func sortSnapshotLists(s *Snapshot) {
	value := reflect.ValueOf(s).Elem()
	for i := 0; i < value.NumField(); i++ {
		field := value.Field(i)
		if field.Kind() != reflect.Slice || field.Len() < 2 {
			continue
		}
		keys := make([]string, field.Len())
		order := make([]int, field.Len())
		for j := range keys {
			keys[j] = fmt.Sprintf("%+v", field.Index(j).Interface())
			order[j] = j
		}
		sort.SliceStable(order, func(a, b int) bool { return keys[order[a]] < keys[order[b]] })
		sorted := reflect.MakeSlice(field.Type(), field.Len(), field.Len())
		for j, from := range order {
			sorted.Index(j).Set(field.Index(from))
		}
		field.Set(sorted)
	}
}

// dropEmptyComplexes removes complexes with empty stores. Restore rebuilds a complex around every stationary unit by adjacency, so a fixture building placed without a complex gains an empty one, by design.
func dropEmptyComplexes(s *Snapshot) {
	kept := s.Complexes[:0]
	for _, c := range s.Complexes {
		if c.Material != 0 || c.Fuel != 0 || c.Gold != 0 {
			kept = append(kept, c)
		}
	}
	s.Complexes = kept
}

// saveAndRestore pushes a game through Capture, Save, Load and Restore and returns the snapshots taken before and after the trip.
func saveAndRestore(t *testing.T, g *sim.Game) (before, after *Snapshot) {
	t.Helper()
	before = Capture(g, richMapName)
	var buf bytes.Buffer
	if err := Save(&buf, before); err != nil {
		t.Fatalf("save: %v", err)
	}
	loaded, err := Load(&buf)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	restored := loaded.Restore(landMap(24, 24))
	after = Capture(restored, richMapName)
	dropEmptyComplexes(before)
	dropEmptyComplexes(after)
	sortSnapshotLists(before)
	sortSnapshotLists(after)
	return before, after
}

// describeDifference prints how two snapshot fields differ, listing only the differing elements for slices.
func describeDifference(before, after reflect.Value) string {
	if before.Kind() != reflect.Slice {
		return fmt.Sprintf("\n    before: %+v\n    after:  %+v", before.Interface(), after.Interface())
	}
	text := fmt.Sprintf("\n    lengths: before %d, after %d", before.Len(), after.Len())
	for i := 0; i < before.Len() && i < after.Len(); i++ {
		if !reflect.DeepEqual(before.Index(i).Interface(), after.Index(i).Interface()) {
			text += fmt.Sprintf("\n    [%d] before: %+v\n    [%d] after:  %+v", i, before.Index(i).Interface(), i, after.Index(i).Interface())
		}
	}
	return text
}

// TestSaveRestoreCaptureIsStable checks that a restored game captures to the same snapshot, field by field.
func TestSaveRestoreCaptureIsStable(t *testing.T) {
	for run := 0; run < 3; run++ {
		before, after := saveAndRestore(t, newRichGame(t))
		beforeValue := reflect.ValueOf(before).Elem()
		afterValue := reflect.ValueOf(after).Elem()
		for i := 0; i < beforeValue.NumField(); i++ {
			name := beforeValue.Type().Field(i).Name
			if !reflect.DeepEqual(beforeValue.Field(i).Interface(), afterValue.Field(i).Interface()) {
				t.Errorf("Snapshot.%s changed across save/load (run %d):%s",
					name, run, describeDifference(beforeValue.Field(i), afterValue.Field(i)))
			}
		}
	}
}

// TestRoundTripFixtureFillsEverySnapshotField checks that the rich fixture sets every snapshot field. A new Snapshot field fails here until the fixture and the round trip cover it.
func TestRoundTripFixtureFillsEverySnapshotField(t *testing.T) {
	// The game screen fills these after Capture, so Capture itself never sets them.
	screenFilled := map[string]bool{
		"Name": true, "Bookmarks": true, "Groups": true, "Displays": true,
		"TurnSeconds": true, "MissionTitle": true, "MissionBrief": true, "SpawnSeq": true,
		"TurnTimer": true, "EndTurn": true, // v26, the screen's own timers
	}
	snap := Capture(newRichGame(t), richMapName)
	value := reflect.ValueOf(snap).Elem()
	for i := 0; i < value.NumField(); i++ {
		name := value.Type().Field(i).Name
		isZero := value.Field(i).IsZero()
		switch {
		case screenFilled[name] && !isZero:
			t.Errorf("Snapshot.%s is set by Capture; drop it from the screen filled list", name)
		case !screenFilled[name] && isZero:
			t.Errorf("Snapshot.%s is zero in the rich fixture: extend newRichGame so a lost field is noticed", name)
		}
	}
}
