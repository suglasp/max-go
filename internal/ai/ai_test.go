// ai_test.go : tests AI strategy, production, movement, and combat behavior.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package ai

import (
	"testing"

	"github.com/suglasp/max-go/internal/access"
	"github.com/suglasp/max-go/internal/assets"
	"github.com/suglasp/max-go/internal/build"
	"github.com/suglasp/max-go/internal/economy"
	"github.com/suglasp/max-go/internal/game"
	"github.com/suglasp/max-go/internal/sim"
	"github.com/suglasp/max-go/internal/units"
	"github.com/suglasp/max-go/internal/winloss"
	"github.com/suglasp/max-go/internal/world"
)

// allLand is a synthetic all land SurfaceSource so the hook test needs no game
// data. C++: a flat test map.
type allLand struct{}

func (allLand) SurfaceAt(x, y int) uint8 { return world.SurfaceLand }

// testUnitFactory mints a minimally statted unit for the headless AI tests (no
// ATTRIBS / research data needed).
var testFactorySeq int

func testUnitFactory(team int, t assets.UnitType, flags game.Flags) *game.UnitInfo {
	testFactorySeq++
	u := game.NewUnitInfo(t, team, testFactorySeq, 0)
	u.Flags = flags
	u.Values = &game.UnitValues{}
	u.InitState()
	return u
}

// TestAIGrowsBase checks the C2 economy task: a computer team with an idle
// constructor issues a build order on its turn (the base starts growing).
func TestAIGrowsBase(t *testing.T) {
	const w, h = 12, 12
	mgr := units.NewManager()
	grid := &access.Grid{W: w, H: h, Surf: allLand{}, Units: mgr.Cells}

	p := &game.StartParams{}
	p.Teams[0].Type = game.TeamPlayer
	p.Teams[1].Type = game.TeamComputer
	g := sim.NewWithGrid(game.NewGameState(p), mgr, grid, w, h)

	// A computer team constructor, deployed mid map with room to build around it.
	con := game.NewUnitInfo(assets.Constrct, 1, 1, 0)
	con.Flags = game.FlagConstructorUnit | game.FlagMobileLandUnit | game.FlagSelectable
	con.Values = &game.UnitValues{}
	con.InitState()
	con.Values.Set(game.AttribStorage, 40)
	con.Storage = 40 // a real constructor is loaded with raw to spend on construction
	g.Units.Deploy(con, 6, 6)

	g.AI = New(testUnitFactory)
	if got := len(g.BuildOrders()); got != 0 {
		t.Fatalf("pre-turn build orders = %d, want 0", got)
	}

	g.BeginTeamTurn(1)

	if got := len(g.BuildOrders()); got == 0 {
		t.Fatal("AI placed no build order; the base did not start growing")
	}
	if !con.Building {
		t.Error("constructor was not put into the build pose")
	}
}

// TestAIDefersUnfundedDetachedBuild verifies that an empty constructor without access to a base pool waits
// for raw material before starting a detached build.
func TestAIDefersUnfundedDetachedBuild(t *testing.T) {
	const w, h = 12, 12
	mgr := units.NewManager()
	grid := &access.Grid{W: w, H: h, Surf: allLand{}, Units: mgr.Cells}

	p := &game.StartParams{}
	p.Teams[1].Type = game.TeamComputer
	g := sim.NewWithGrid(game.NewGameState(p), mgr, grid, w, h)

	// An empty constructor (capacity 40, but 0 raw loaded) with no base to supply it.
	con := game.NewUnitInfo(assets.Constrct, 1, 1, 0)
	con.Flags = game.FlagConstructorUnit | game.FlagMobileLandUnit | game.FlagSelectable
	con.Values = &game.UnitValues{}
	con.InitState()
	con.Values.Set(game.AttribStorage, 40)
	con.Storage = 0 // empty: cannot fund a detached build, and no pool to load from
	g.Units.Deploy(con, 6, 6)

	bm := newBuildManager(g, 1)

	bm.BeginTurn()
	if got := len(g.BuildOrders()); got != 0 {
		t.Fatalf("an empty constructor with no supply started %d build(s); it should defer (would stall at empty)", got)
	}

	// Load it, like a player loading a constructor: now it can self supply and build.
	con.Storage = 40
	bm.BeginTurn()
	if got := len(g.BuildOrders()); got == 0 {
		t.Fatal("a loaded constructor placed no build order; it should now build")
	}
}

// newBuildManager wires a building manager task directly to a game so a test can
// drive its demand roster (taskManageBuildings.BeginTurn) without running the full
// BeginTeamTurn economy/visibility passes that could mutate the staged setup.
func newBuildManager(g *sim.Game, team int) *taskManageBuildings {
	return newTaskManageBuildings(team, StrategyCombinedArms, goalKill, false, &aiContext{Game: g, MakeUnit: testUnitFactory})
}

// stationary deploys a finished 1x1 stationary building for the economy demand
// tests (footprint is irrelevant to the economy snapshot, which keys off type and
// storage).
func stationary(team, id int, ut assets.UnitType, gx, gy int, mgr *units.Manager) *game.UnitInfo {
	u := game.NewUnitInfo(ut, team, id, 0)
	u.Flags = game.FlagStationary | game.FlagSelectable
	u.Values = &game.UnitValues{}
	u.InitState()
	mgr.Deploy(u, gx, gy)
	return u
}

// constructorUnit deploys an idle computer team constructor (raises 2x2 buildings).
func constructorUnit(team, id, gx, gy int, mgr *units.Manager) *game.UnitInfo {
	u := game.NewUnitInfo(assets.Constrct, team, id, 0)
	u.Flags = game.FlagConstructorUnit | game.FlagMobileLandUnit | game.FlagSelectable
	u.Values = &game.UnitValues{}
	u.InitState()
	u.Values.Set(game.AttribStorage, 40)
	u.Storage = 40 // a real constructor is loaded with raw to spend on construction
	mgr.Deploy(u, gx, gy)
	return u
}

// engineerUnit deploys an idle computer team engineer (raises 1x1 buildings).
func engineerUnit(team, id, gx, gy int, mgr *units.Manager) *game.UnitInfo {
	u := game.NewUnitInfo(assets.Engineer, team, id, 0)
	u.Flags = game.FlagConstructorUnit | game.FlagMobileLandUnit | game.FlagSelectable
	u.Values = &game.UnitValues{}
	u.InitState()
	u.Values.Set(game.AttribStorage, 40)
	u.Storage = 40 // a real engineer is loaded with raw to spend on construction
	mgr.Deploy(u, gx, gy)
	return u
}

// findOrder returns the first active build order producing the given type, or nil.
func findOrder(g *sim.Game, ut assets.UnitType) *build.BuildOrder {
	for _, o := range g.BuildOrders() {
		if o.Product != nil && o.Product.Type == ut {
			return o
		}
	}
	return nil
}

// TestAIBuildsMiningOnRichestSite checks the mining demand tier: a base with no
// mining station picks the highest yield reachable footprint, not merely the
// nearest deposit. The builder stands on the rich cluster so it builds in place
// (the distant site drive is covered by TestAIDrivesToDistantMiningSite).
func TestAIBuildsMiningOnRichestSite(t *testing.T) {
	const w, h = 20, 20
	mgr := units.NewManager()
	grid := &access.Grid{W: w, H: h, Surf: allLand{}, Units: mgr.Cells}
	p := &game.StartParams{}
	p.Teams[1].Type = game.TeamComputer
	g := sim.NewWithGrid(game.NewGameState(p), mgr, grid, w, h)

	// A poor single cell deposit and a rich 2x2 cluster. The footprint anchored at
	// (8,8) taps all four rich cells (64); the poor cell yields only 8, so the
	// manager must choose the cluster.
	res := economy.NewResourceMap(w, h)
	res.SetDeposit(12, 10, economy.CargoRaw, 8)
	for _, c := range [][2]int{{8, 8}, {9, 8}, {8, 9}, {9, 9}} {
		res.SetDeposit(c[0], c[1], economy.CargoRaw, 16)
	}
	// The team has surveyed the region (its surveyor revealed it); the AI obeys
	// resource fog, so it only ranks deposits it has seen. TestAIDoesNotMineUnsurveyed
	// covers the unsurveyed case.
	res.Survey(10, 9, 10, 1)
	g.Resources = res

	con := constructorUnit(1, 1, 9, 9, mgr) // standing on the cluster: builds in place

	newBuildManager(g, 1).BeginTurn()

	o := findOrder(g, assets.MiningSt)
	if o == nil {
		t.Fatal("no mining station queued for a base with no mine and reachable resources")
	}
	raw, gold, fuel := res.TotalResourcesInArea(o.GridX, o.GridY, 1, 1, true)
	if got := raw + gold + fuel; got < 40 {
		t.Errorf("mine sited at (%d,%d) taps only %d resource; expected the rich cluster (~64)", o.GridX, o.GridY, got)
	}
	// The builder must raise the structure from INSIDE its footprint (so the build
	// pose is a short tape slide, not a long glide in from a neighbouring cell).
	if !builderOnFootprint(con.GridX, con.GridY, o.GridX, o.GridY, 2) {
		t.Errorf("builder at (%d,%d) is not inside the mine footprint at (%d,%d): would slide in from outside",
			con.GridX, con.GridY, o.GridX, o.GridY)
	}
}

// TestAIDrivesToDistantMiningSite checks the move then build behaviour: a
// constructor heading for a deposit too far to build in place DRIVES there (an
// animated move) instead of teleporting into the build pose, and raises the
// station once it arrives. C++: TaskCreateBuilding's TaskMove then build.
func TestAIDrivesToDistantMiningSite(t *testing.T) {
	const w, h = 24, 24
	mgr := units.NewManager()
	grid := &access.Grid{W: w, H: h, Surf: allLand{}, Units: mgr.Cells}
	p := &game.StartParams{}
	p.Teams[1].Type = game.TeamComputer
	g := sim.NewWithGrid(game.NewGameState(p), mgr, grid, w, h)

	res := economy.NewResourceMap(w, h)
	for _, c := range [][2]int{{4, 4}, {5, 4}, {4, 5}, {5, 5}} {
		res.SetDeposit(c[0], c[1], economy.CargoRaw, 16)
	}
	res.Survey(4, 4, 3, 1) // the team has surveyed the cluster; the AI mines only what it has seen
	g.Resources = res

	con := constructorUnit(1, 1, 18, 18, mgr) // far from the deposit cluster
	con.Values.Set(game.AttribSpeed, 40)
	con.ResetMovement() // give it a movement budget for the drive

	bm := newBuildManager(g, 1)

	// First plan: the site is distant, so the manager sends the constructor toward
	// it rather than starting a build on the spot.
	bm.BeginTurn()
	if findOrder(g, assets.MiningSt) != nil {
		t.Fatal("manager started a distant build in place instead of driving to it")
	}
	if !g.HasOrder(con) {
		t.Fatal("manager did not send the constructor toward the distant deposit")
	}

	// Drive it in over several turns; it must arrive and raise the mine on the cluster.
	for i := 0; i < 20 && findOrder(g, assets.MiningSt) == nil; i++ {
		g.ResolveMovement()
		for _, u := range g.Units.Units() {
			if u.Team == 1 {
				u.ResetMovement()
			}
		}
		bm.BeginTurn()
	}

	o := findOrder(g, assets.MiningSt)
	if o == nil {
		t.Fatal("constructor never reached the distant deposit to build the mine")
	}
	if !con.Building {
		t.Error("constructor did not enter the build pose on arrival")
	}
	raw, gold, fuel := res.TotalResourcesInArea(o.GridX, o.GridY, 1, 1, true)
	if got := raw + gold + fuel; got < 40 {
		t.Errorf("mine sited at (%d,%d) taps only %d; expected the deposit cluster", o.GridX, o.GridY, got)
	}
}

// surveyorUnit deploys an idle computer team Surveyor (reveals resources as it
// moves) with a movement budget for the survey tests.
func surveyorUnit(team, id, gx, gy int, mgr *units.Manager) *game.UnitInfo {
	u := game.NewUnitInfo(assets.Surveyor, team, id, 0)
	u.Flags = game.FlagMobileLandUnit | game.FlagSelectable
	v := &game.UnitValues{}
	v.Set(game.AttribSpeed, 12)
	v.Set(game.AttribScan, 4)
	u.Values = v
	u.InitState()
	mgr.Deploy(u, gx, gy)
	return u
}

// bulldozerUnit deploys an idle computer team Bulldozer (clears rubble).
func bulldozerUnit(team, id, gx, gy int, mgr *units.Manager) *game.UnitInfo {
	u := game.NewUnitInfo(assets.Bulldozr, team, id, 0)
	u.Flags = game.FlagMobileLandUnit | game.FlagSelectable
	v := &game.UnitValues{}
	v.Set(game.AttribSpeed, 12)
	u.Values = v
	u.InitState()
	mgr.Deploy(u, gx, gy)
	return u
}

// TestAIDoesNotMineUnsurveyed verifies that the AI cannot site a mine on a rich deposit until the team has surveyed it.
func TestAIDoesNotMineUnsurveyed(t *testing.T) {
	const w, h = 20, 20
	mgr := units.NewManager()
	grid := &access.Grid{W: w, H: h, Surf: allLand{}, Units: mgr.Cells}
	p := &game.StartParams{}
	p.Teams[1].Type = game.TeamComputer
	g := sim.NewWithGrid(game.NewGameState(p), mgr, grid, w, h)

	// A rich 2x2 deposit cluster, left UNSURVEYED by the team.
	res := economy.NewResourceMap(w, h)
	for _, c := range [][2]int{{8, 8}, {9, 8}, {8, 9}, {9, 9}} {
		res.SetDeposit(c[0], c[1], economy.CargoRaw, 16)
	}
	g.Resources = res

	// Ground truth (all=true): the cluster really is rich - the AI just cannot see it.
	if raw, _, _ := res.TotalResourcesInArea(8, 8, 1, 1, true); raw < 40 {
		t.Fatalf("setup: cluster should tap ~64 raw, got %d", raw)
	}

	constructorUnit(1, 1, 9, 9, mgr) // standing on the rich but unsurveyed deposit

	newBuildManager(g, 1).BeginTurn()

	if o := findOrder(g, assets.MiningSt); o != nil {
		t.Errorf("AI sited a mine at (%d,%d) on UNSURVEYED ground (omniscient resource read)", o.GridX, o.GridY)
	}
}

// TestAISurveyorScoutsUnsurveyed checks taskSurvey: an idle Surveyor on a map with
// nothing surveyed drives toward unsurveyed ground (so the team discovers its
// resources), while taskSurvey leaves non surveyor units alone.
func TestAISurveyorScoutsUnsurveyed(t *testing.T) {
	const w, h = 20, 20
	mgr := units.NewManager()
	grid := &access.Grid{W: w, H: h, Surf: allLand{}, Units: mgr.Cells}
	p := &game.StartParams{}
	p.Teams[1].Type = game.TeamComputer
	g := sim.NewWithGrid(game.NewGameState(p), mgr, grid, w, h)
	g.Resources = economy.NewResourceMap(w, h) // a fresh map: nothing surveyed yet

	surveyor := surveyorUnit(1, 1, 10, 10, mgr)
	scout := scoutUnit(1, 2, 14, 14, 3, mgr) // a non Surveyor: taskSurvey must not touch it

	newTaskSurvey(1, &aiContext{Game: g, MakeUnit: testUnitFactory}).BeginTurn()

	if !g.HasOrder(surveyor) {
		t.Error("surveyor did not drive toward unsurveyed ground")
	}
	if g.HasOrder(scout) {
		t.Error("taskSurvey moved a non-surveyor unit; it should only drive the Surveyor")
	}
}

// TestAIBuildsPowerWhenShort checks the power demand tier for a CONSTRUCTOR: a
// base whose consumers outweigh its generators raises a 2x2 Power Station (the
// constructor's generator), never a 1x1 Power Generator (that is engineer only).
func TestAIBuildsPowerWhenShort(t *testing.T) {
	const w, h = 16, 16
	mgr := units.NewManager()
	grid := &access.Grid{W: w, H: h, Surf: allLand{}, Units: mgr.Cells}
	p := &game.StartParams{}
	p.Teams[1].Type = game.TeamComputer
	g := sim.NewWithGrid(game.NewGameState(p), mgr, grid, w, h)

	stationary(1, 12, assets.LandPlt, 4, 5, mgr) // three consumers, no generation: power > 0
	stationary(1, 13, assets.AirPlt, 5, 5, mgr)
	stationary(1, 14, assets.Research, 6, 5, mgr)

	con := constructorUnit(1, 1, 7, 5, mgr) // beside the base: builds connected in place

	newBuildManager(g, 1).BeginTurn()

	if !con.Building {
		t.Fatal("constructor stayed idle: the manager issued no order")
	}
	if findOrder(g, assets.PowerStn) == nil {
		t.Error("power-short base: constructor did not queue a Power Station")
	}
	if findOrder(g, assets.PowGen) != nil {
		t.Error("constructor queued a 1x1 Power Generator (engineer-only building)")
	}
}

// TestAIEngineerBuildsPowerGenerator checks the power demand tier for an ENGINEER:
// it raises a 1x1 Power Generator (its generator), not the constructor's station.
func TestAIEngineerBuildsPowerGenerator(t *testing.T) {
	const w, h = 16, 16
	mgr := units.NewManager()
	grid := &access.Grid{W: w, H: h, Surf: allLand{}, Units: mgr.Cells}
	p := &game.StartParams{}
	p.Teams[1].Type = game.TeamComputer
	g := sim.NewWithGrid(game.NewGameState(p), mgr, grid, w, h)

	stationary(1, 12, assets.LandPlt, 4, 5, mgr) // consumers, no generation: power > 0
	stationary(1, 13, assets.Research, 5, 5, mgr)

	eng := engineerUnit(1, 1, 6, 5, mgr) // beside the base: builds connected in place

	newBuildManager(g, 1).BeginTurn()

	if !eng.Building {
		t.Fatal("engineer stayed idle: the manager issued no order")
	}
	if findOrder(g, assets.PowGen) == nil {
		t.Error("power-short base: engineer did not queue a Power Generator")
	}
	if findOrder(g, assets.PowerStn) != nil {
		t.Error("engineer queued a 2x2 Power Station (constructor-only building)")
	}
}

// TestAIBuildsStorageWhenFull checks the storage demand tier: an ENGINEER (storage
// dumps are 1x1) raises a material dump when raw storage has filled to three
// quarters (a generator balances power so the power tier does not preempt it).
func TestAIBuildsStorageWhenFull(t *testing.T) {
	const w, h = 16, 16
	mgr := units.NewManager()
	grid := &access.Grid{W: w, H: h, Surf: allLand{}, Units: mgr.Cells}
	p := &game.StartParams{}
	p.Teams[1].Type = game.TeamComputer
	g := sim.NewWithGrid(game.NewGameState(p), mgr, grid, w, h)

	st := stationary(1, 10, assets.MiningSt, 4, 4, mgr)
	st.Values.Set(game.AttribStorage, 16)
	st.Storage = 12                             // 3/4 of capacity: a dump is due
	stationary(1, 11, assets.PowGen, 4, 6, mgr) // balances the mining station's power draw

	engineerUnit(1, 1, 5, 4, mgr) // beside the base: builds connected in place

	newBuildManager(g, 1).BeginTurn()

	if findOrder(g, assets.ADump) == nil {
		t.Error("raw storage at 3/4 did not trigger a material dump")
	}
}

// TestAIStopsRawStorageAtCap checks the storage cap: once a cargo type's capacity
// reaches the limit, the AI stops adding dumps even though the store is full - so a
// mining surplus does not pile on raw dumps forever. C++: EndTurn's capacity.* < 500.
func TestAIStopsRawStorageAtCap(t *testing.T) {
	const w, h = 16, 16
	mgr := units.NewManager()
	grid := &access.Grid{W: w, H: h, Surf: allLand{}, Units: mgr.Cells}
	p := &game.StartParams{}
	p.Teams[1].Type = game.TeamComputer
	g := sim.NewWithGrid(game.NewGameState(p), mgr, grid, w, h)

	// A mining station whose raw store alone already sits at the cap and is full.
	st := stationary(1, 10, assets.MiningSt, 4, 4, mgr)
	st.Values.Set(game.AttribStorage, storageCapLimit)
	st.Storage = storageCapLimit                // full, but total raw capacity is already at the cap
	stationary(1, 11, assets.PowGen, 4, 6, mgr) // balance power so that tier does not preempt

	engineerUnit(1, 1, 5, 4, mgr)

	newBuildManager(g, 1).BeginTurn()

	if findOrder(g, assets.ADump) != nil {
		t.Error("raw capacity already at the cap still triggered another material dump")
	}
}

// TestStorageCapScalesWithDifficulty verifies that a hard AI stops adding dumps at a lower per-cargo capacity
// than an average AI.
func TestStorageCapScalesWithDifficulty(t *testing.T) {
	setDifficulty(t, OpponentClueless)
	clueless := aiStorageCap
	setDifficulty(t, OpponentAverage)
	average := aiStorageCap
	setDifficulty(t, OpponentGod)
	god := aiStorageCap

	if !(clueless > average && average > god) {
		t.Fatalf("storage cap should fall as difficulty rises: clueless=%d average=%d god=%d", clueless, average, god)
	}

	// A full raw store whose capacity sits between God's cap and Average's cap: the hard
	// AI must NOT add another dump (already over its cap), the average AI still should.
	midCap := (god + average) / 2

	setDifficulty(t, OpponentGod)
	if needsStorage(midCap, midCap, true) {
		t.Errorf("God AI added a dump at capacity %d over its cap %d", midCap, aiStorageCap)
	}

	setDifficulty(t, OpponentAverage)
	if !needsStorage(midCap, midCap, true) {
		t.Errorf("Average AI did not add a dump at capacity %d under its cap %d", midCap, aiStorageCap)
	}
}

// newProduceManager builds a production task wired to the headless test context.
func newProduceManager(g *sim.Game, team int) *taskProduceUnits {
	return newTaskProduceUnits(team, StrategyCombinedArms, &aiContext{Game: g, MakeUnit: testUnitFactory})
}

// TestAIBuildsSurveyorWhenMissing checks the recon needs demand: a team with an
// idle Light Plant and no Surveyor produces one. The Surveyor is absent from balanced factory rosters, so
// this restores lost reconnaissance.
func TestAIBuildsSurveyorWhenMissing(t *testing.T) {
	const w, h = 16, 16
	mgr := units.NewManager()
	grid := &access.Grid{W: w, H: h, Surf: allLand{}, Units: mgr.Cells}
	p := &game.StartParams{}
	p.Teams[1].Type = game.TeamComputer
	g := sim.NewWithGrid(game.NewGameState(p), mgr, grid, w, h)

	stationary(1, 10, assets.LightPlt, 4, 4, mgr) // idle light plant, no Surveyor in the field

	newProduceManager(g, 1).BeginTurn()

	if findOrder(g, assets.Surveyor) == nil {
		t.Error("a team with no Surveyor did not build one (recon-needs demand)")
	}
}

// TestAIDoesNotOverbuildSurveyor checks the demand is satisfied once met: a team
// that already fields a Surveyor issues no new Surveyor order.
func TestAIDoesNotOverbuildSurveyor(t *testing.T) {
	const w, h = 16, 16
	mgr := units.NewManager()
	grid := &access.Grid{W: w, H: h, Surf: allLand{}, Units: mgr.Cells}
	p := &game.StartParams{}
	p.Teams[1].Type = game.TeamComputer
	g := sim.NewWithGrid(game.NewGameState(p), mgr, grid, w, h)

	stationary(1, 10, assets.LightPlt, 4, 4, mgr)
	surveyorUnit(1, 1, 6, 6, mgr) // the team already has its Surveyor

	newProduceManager(g, 1).BeginTurn()

	if findOrder(g, assets.Surveyor) != nil {
		t.Error("a team that already has a Surveyor built another (demand should be satisfied)")
	}
}

// TestAIBuildsScannerForRecon checks the enemy recon half of the demand: with the
// Surveyor demand already met, an idle Heavy Plant produces a Scanner (mobile radar)
// for enemy reconnaissance - the Scanner is a HEAVY unit (manual p.28), so it builds
// at the LandPlt, not the Light plant.
func TestAIBuildsScannerForRecon(t *testing.T) {
	const w, h = 16, 16
	mgr := units.NewManager()
	grid := &access.Grid{W: w, H: h, Surf: allLand{}, Units: mgr.Cells}
	p := &game.StartParams{}
	p.Teams[1].Type = game.TeamComputer
	g := sim.NewWithGrid(game.NewGameState(p), mgr, grid, w, h)

	stationary(1, 10, assets.LandPlt, 4, 4, mgr) // the Heavy plant builds the Scanner
	surveyorUnit(1, 1, 6, 6, mgr)                // Surveyor demand satisfied, so the Scanner demand is under test

	newProduceManager(g, 1).BeginTurn()

	if findOrder(g, assets.Scanner) == nil {
		t.Error("a team with no Scanner did not build one for enemy recon")
	}
}

// TestFactoryProductsMatchManual locks the AI factory roster to the manual authoritative
// placement (matching the player's menus/factorybuild.go): the Scanner is HEAVY (LandPlt),
// SP Flak is LIGHT (LightPlt), and infantry trains at the TrainHal - the Barracks is a
// service building, not a factory.
func TestFactoryProductsMatchManual(t *testing.T) {
	has := func(factory, unit assets.UnitType) bool {
		for _, prod := range factoryProducts[factory] {
			if prod.t == unit {
				return true
			}
		}
		return false
	}
	if !has(assets.LandPlt, assets.Scanner) || has(assets.LightPlt, assets.Scanner) {
		t.Error("Scanner must be under the Heavy plant (LandPlt), not the Light plant")
	}
	if !has(assets.LightPlt, assets.SpFlak) || has(assets.LandPlt, assets.SpFlak) {
		t.Error("SP Flak must be under the Light plant (LightPlt), not the Heavy plant")
	}
	if !isFactory(assets.TrainHal) || !has(assets.TrainHal, assets.Infantry) {
		t.Error("infantry must train at the TrainHal (and TrainHal must be a factory)")
	}
	if isFactory(assets.Barracks) {
		t.Error("the Barracks must NOT be a factory (it is a service building)")
	}
}

// TestChooseProductSkewsByStrategy: the production picker biases the army toward each
// strategy's signature units while never starving the others - an Air strategy fills the
// air plant mostly with the combat wing, a Tank Horde fills the heavy plant mostly with
// armour, and CombinedArms keeps an even spread (the baseline).
func TestChooseProductSkewsByStrategy(t *testing.T) {
	// Fill one factory `rounds` times, counting the chosen unit types.
	run := func(factory assets.UnitType, s Strategy, rounds int) map[assets.UnitType]int {
		counts := map[assets.UnitType]int{}
		mix := func(u assets.UnitType) int { return counts[u] }
		for i := 0; i < rounds; i++ {
			prod, ok := chooseProduct(factory, s, mix)
			if !ok {
				t.Fatalf("chooseProduct returned nothing for %v", factory)
			}
			counts[prod.t]++
		}
		return counts
	}

	// Tank Horde at the heavy plant: tanks dominate, but the other types still appear.
	c := run(assets.LandPlt, StrategyTankHorde, 40)
	if c[assets.Tank] <= c[assets.Artillry] || c[assets.Tank] <= c[assets.RocktLch] {
		t.Errorf("tank horde did not favour tanks: %v", c)
	}
	if c[assets.Artillry] == 0 || c[assets.RocktLch] == 0 {
		t.Errorf("tank horde starved a non-favoured type (it should still build some): %v", c)
	}

	// Air strategy at the air plant: fighters + bombers dominate the radar plane.
	a := run(assets.AirPlt, StrategyAir, 40)
	if a[assets.Fighter]+a[assets.Bomber] <= a[assets.Awac]*2 {
		t.Errorf("air strategy did not favour the combat air wing: %v", a)
	}

	// CombinedArms keeps the even baseline spread (no type more than one ahead).
	e := run(assets.LandPlt, StrategyCombinedArms, 40)
	lo, hi := 1<<30, 0
	for _, p := range factoryProducts[assets.LandPlt] {
		n := e[p.t]
		if n < lo {
			lo = n
		}
		if n > hi {
			hi = n
		}
	}
	if hi-lo > 1 {
		t.Errorf("combined arms should spread evenly, got spread %d..%d: %v", lo, hi, e)
	}
}

// TestAIBulldozerClearsRubble: a bulldozer standing on a base area debris pile starts
// clearing it to reclaim the ground (the bulldozer task).
func TestAIBulldozerClearsRubble(t *testing.T) {
	const w, h = 16, 16
	mgr := units.NewManager()
	grid := &access.Grid{W: w, H: h, Surf: allLand{}, Units: mgr.Cells}
	p := &game.StartParams{}
	p.Teams[1].Type = game.TeamComputer
	g := sim.NewWithGrid(game.NewGameState(p), mgr, grid, w, h)

	stationary(1, 10, assets.MiningSt, 8, 8, mgr) // base anchor (teamBaseCenter)
	wreck := armedTank(1, 2, 9, 8, mgr)
	g.DestroyUnit(wreck) // leaves a small rubble pile at (9,8)
	if _, ok := g.RubbleAt(9, 8); !ok {
		t.Fatal("setup: destroying a unit on land should leave rubble")
	}
	dozer := bulldozerUnit(1, 3, 9, 8, mgr) // standing on the pile

	newTaskBulldozer(1, &aiContext{Game: g, MakeUnit: testUnitFactory}).BeginTurn()

	if !g.IsClearingRubble(dozer) {
		t.Error("a bulldozer on a base-area rubble pile should start clearing it")
	}
}

// TestAIBuildsBulldozerForRubble: a team with base area rubble but no bulldozer has a
// Light plant build one (the obtain), so it can reclaim the ground.
func TestAIBuildsBulldozerForRubble(t *testing.T) {
	const w, h = 16, 16
	mgr := units.NewManager()
	grid := &access.Grid{W: w, H: h, Surf: allLand{}, Units: mgr.Cells}
	p := &game.StartParams{}
	p.Teams[1].Type = game.TeamComputer
	g := sim.NewWithGrid(game.NewGameState(p), mgr, grid, w, h)

	stationary(1, 10, assets.MiningSt, 8, 8, mgr) // base anchor
	stationary(1, 11, assets.LightPlt, 5, 5, mgr) // the Light plant builds the dozer
	wreck := armedTank(1, 2, 9, 8, mgr)
	g.DestroyUnit(wreck) // rubble near the base, but the team fields NO bulldozer

	newTaskBulldozer(1, &aiContext{Game: g, MakeUnit: testUnitFactory}).BeginTurn()

	if findOrder(g, assets.Bulldozr) == nil {
		t.Error("a team with base-area rubble and no bulldozer should build one")
	}
}

// TestAIBuildsConnectedToBase checks the connectivity rule: a builder standing far
// from the base on open ground does NOT scatter a detached building there - it
// drives back and raises the structure bordering the base, so it joins the complex.
func TestAIBuildsConnectedToBase(t *testing.T) {
	const w, h = 24, 24
	mgr := units.NewManager()
	grid := &access.Grid{W: w, H: h, Surf: allLand{}, Units: mgr.Cells}
	p := &game.StartParams{}
	p.Teams[1].Type = game.TeamComputer
	g := sim.NewWithGrid(game.NewGameState(p), mgr, grid, w, h)

	base := buildingUnit(1, assets.LandPlt, true) // a consumer: power > 0
	g.PlaceBuilding(base, 4, 4)
	g.ComplexOf(base).Material = 100       // a STOCKED base the engineer connects to and draws
	eng := engineerUnit(1, 1, 18, 18, mgr) // far from the base, on open ground
	eng.Values.Set(game.AttribSpeed, 40)
	eng.ResetMovement()

	bm := newBuildManager(g, 1)
	bm.BeginTurn()

	// It must not raise a detached generator out where it stands; it drives back.
	if findOrder(g, assets.PowGen) != nil {
		t.Fatal("engineer raised a DETACHED generator far from the base instead of connecting")
	}
	if !g.HasOrder(eng) {
		t.Fatal("engineer neither built nor drove back to connect to the base")
	}

	// Drive it in; the generator it finally raises must border the base complex.
	for i := 0; i < 30 && findOrder(g, assets.PowGen) == nil; i++ {
		g.ResolveMovement()
		for _, u := range g.Units.Units() {
			if u.Team == 1 {
				u.ResetMovement()
			}
		}
		bm.BeginTurn()
	}
	o := findOrder(g, assets.PowGen)
	if o == nil {
		t.Fatal("engineer never reached the base to build a connected generator")
	}
	if !bm.touchesComplex(o.GridX, o.GridY, 1) {
		t.Errorf("generator at (%d,%d) does not border the base complex", o.GridX, o.GridY)
	}
}

// TestAIProducesUnits checks the C2b production task: a computer team with an
// idle factory queues a mobile unit on its turn.
func TestAIProducesUnits(t *testing.T) {
	const w, h = 12, 12
	mgr := units.NewManager()
	grid := &access.Grid{W: w, H: h, Surf: allLand{}, Units: mgr.Cells}

	p := &game.StartParams{}
	p.Teams[0].Type = game.TeamPlayer
	p.Teams[1].Type = game.TeamComputer
	g := sim.NewWithGrid(game.NewGameState(p), mgr, grid, w, h)

	// A finished heavy vehicle plant for the computer team (a producer factory).
	plant := game.NewUnitInfo(assets.LandPlt, 1, 1, 0)
	plant.Flags = game.FlagStationary | game.FlagBuilding | game.FlagSelectable
	plant.Values = &game.UnitValues{}
	plant.InitState()
	g.Units.Deploy(plant, 5, 5)

	g.AI = New(testUnitFactory)
	g.BeginTeamTurn(1)

	// The production task should have queued a mobile land unit from the plant.
	var produced *game.UnitInfo
	for _, o := range g.BuildOrders() {
		if o.Builder == plant && o.Product != nil {
			produced = o.Product
		}
	}
	if produced == nil {
		t.Fatal("factory produced nothing")
	}
	if !produced.Flags.Has(game.FlagMobileLandUnit) {
		t.Errorf("produced unit %v is not a mobile land unit", produced.Type)
	}
}

// armedUnit builds a combat ready unit of any type/flags with explicit stats.
func armedUnit(team, id int, ut assets.UnitType, flags game.Flags, gx, gy int, mgr *units.Manager) *game.UnitInfo {
	u := game.NewUnitInfo(ut, team, id, 0)
	u.Flags = flags
	v := &game.UnitValues{}
	v.Set(game.AttribAttack, 12)
	v.Set(game.AttribRange, 4)
	v.Set(game.AttribRounds, 1)
	v.Set(game.AttribAmmo, 6)
	v.Set(game.AttribSpeed, 12)
	v.Set(game.AttribHits, 24)
	u.Values = v
	u.InitState()
	mgr.Deploy(u, gx, gy)
	return u
}

// armedTank builds a combat ready land tank with explicit stats.
func armedTank(team, id, gx, gy int, mgr *units.Manager) *game.UnitInfo {
	return armedUnit(team, id, assets.Tank, game.FlagMobileLandUnit|game.FlagSelectable|game.FlagTurretSprite, gx, gy, mgr)
}

// setMinStrikeForce overrides the AI muster threshold for one test and restores
// it afterwards, so a geometry/targeting test can exercise a single committed
// attacker without first fielding a full strike force (the muster ramp itself is
// covered by TestAIMustersBeforeCommitting).
func setMinStrikeForce(t *testing.T, n int) {
	t.Helper()
	prev := minStrikeForce
	minStrikeForce = n
	t.Cleanup(func() { minStrikeForce = prev })
}

// setDifficulty applies an AI difficulty tier for one test and restores every knob
// it touches afterwards. The tier knobs are package globals (a single global tier
// drives all AI, as in vanilla), so a test that wants a specific muster / retreat /
// temperament sets it through here rather than poking the individual vars.
func setDifficulty(t *testing.T, tier int) {
	t.Helper()
	pm, pr, pf, pd := minStrikeForce, retreatEnabled, aiDifficultyFactor, aiDefensiveOnly
	ph, pg, pk := aiHarassStart, aiGraceTurns, aiKeepReserve
	pt, pe, pl := aiTier, escortEnabled, aiLossMemoryTurns
	psc := aiStorageCap
	applyDifficulty(tier)
	t.Cleanup(func() {
		minStrikeForce, retreatEnabled, aiDifficultyFactor, aiDefensiveOnly = pm, pr, pf, pd
		aiHarassStart, aiGraceTurns, aiKeepReserve = ph, pg, pk
		aiTier, escortEnabled, aiLossMemoryTurns = pt, pe, pl
		aiStorageCap = psc
	})
}

// advancingAtTurn builds a fresh Average vs player game at the given turn with
// nAttackers spotted AI tanks and one far, spotted, out of range player tank, runs
// the computer turn, and returns how many AI units were sent to advance. The map is
// small and fully scanned so the only orders come from the attack task (no explore /
// build noise). The caller sets the difficulty tier via setDifficulty first.
func advancingAtTurn(t *testing.T, turn, nAttackers int) int {
	t.Helper()
	const w, h = 16, 16
	mgr := units.NewManager()
	grid := &access.Grid{W: w, H: h, Surf: allLand{}, Units: mgr.Cells}
	p := &game.StartParams{}
	p.Teams[0].Type = game.TeamPlayer
	p.Teams[1].Type = game.TeamComputer
	g := sim.NewWithGrid(game.NewGameState(p), mgr, grid, w, h)
	g.State.TurnCounter = turn
	g.State.Difficulty = aiTier // the tier set via setDifficulty is the game's tier the AI reads on init
	for i := 0; i < nAttackers; i++ {
		u := armedTank(1, i+1, 2+i, 2, mgr)
		seeAll(u, w)
	}
	_ = armedTank(0, 100, 14, 14, mgr) // player, spotted, far out of range
	g.AI = New(testUnitFactory)
	g.BeginTeamTurn(1)
	n := 0
	for _, u := range g.Units.Units() {
		if u.Team == 1 && g.HasOrder(u) {
			n++
		}
	}
	return n
}

// TestAIBuildPhaseHoldsThenHarasses walks an Average team through the three phases:
// it holds and builds early, sends a single light probe in the harassment window, and
// commits the whole mustered force only after the grace turn.
func TestAIBuildPhaseHoldsThenHarasses(t *testing.T) {
	setDifficulty(t, OpponentAverage) // harassStart 100, grace 200, muster 3, no reserve
	if n := advancingAtTurn(t, 10, 3); n != 0 {
		t.Errorf("build phase (turn 10): %d advanced, want 0 (hold and build)", n)
	}
	if n := advancingAtTurn(t, 120, 3); n != 1 {
		t.Errorf("harass phase (turn 120): %d advanced, want 1 light probe", n)
	}
	if n := advancingAtTurn(t, 210, 3); n != 3 {
		t.Errorf("assault phase (turn 210): %d advanced, want all 3 (mustered, no reserve)", n)
	}
}

// TestGodGraceFloor: even a fully mustered God team does not launch its full assault
// before its grace turn (the player always gets an opening); after the grace it does.
func TestGodGraceFloor(t *testing.T) {
	setDifficulty(t, OpponentGod) // harassStart 20, grace 40, muster 1
	if n := advancingAtTurn(t, 5, 1); n != 0 {
		t.Errorf("God at turn 5 (before harass/grace): %d advanced, want 0", n)
	}
	if n := advancingAtTurn(t, 50, 1); n != 1 {
		t.Errorf("God at turn 50 (past grace): %d advanced, want 1 (assault)", n)
	}
}

// TestAIRaidsKeyFacility: the target picker prefers an enemy eco sphere (a key
// facility) over a NEARER generic enemy unit, so the AI raids the economy/score.
func TestAIRaidsKeyFacility(t *testing.T) {
	const w, h = 24, 24
	mgr := units.NewManager()
	grid := &access.Grid{W: w, H: h, Surf: allLand{}, Units: mgr.Cells}
	p := &game.StartParams{}
	p.Teams[0].Type = game.TeamPlayer
	p.Teams[1].Type = game.TeamComputer
	g := sim.NewWithGrid(game.NewGameState(p), mgr, grid, w, h)

	aiTank := armedTank(1, 1, 2, 2, mgr)
	enemyTank := armedTank(0, 2, 8, 2, mgr)                            // nearer (d 6), generic
	greenhouse := buildingUnit(0, assets.Greenhse, true)               // the eco sphere (key facility)
	g.Units.Deploy(greenhouse, 14, 2)                                  // farther (d 12)
	g.AddSpotted(1, enemyTank, enemyTank.GridX, enemyTank.GridY, true) // both already spotted
	g.AddSpotted(1, greenhouse, greenhouse.GridX, greenhouse.GridY, true)

	ta := newTaskAttackEnemies(1, &aiContext{Game: g, MakeUnit: testUnitFactory})
	if got := ta.nearestEnemy(aiTank); got != greenhouse {
		t.Errorf("raid targeting picked %v, want the eco-sphere (key facility) over the nearer tank", got)
	}
}

// TestAITargetsHighestScoringEnemyTeam: in a three team game the force focus targets the
// enemy team with the higher score (g.TeamPoints), a reduced port of DetermineTargetTeam,
// even when a lower scoring enemy sits closer to the AI base.
func TestAITargetsHighestScoringEnemyTeam(t *testing.T) {
	const w, h = 24, 24
	mgr := units.NewManager()
	grid := &access.Grid{W: w, H: h, Surf: allLand{}, Units: mgr.Cells}
	p := &game.StartParams{}
	p.Teams[0].Type = game.TeamPlayer   // enemy A: low score, near the base
	p.Teams[1].Type = game.TeamComputer // the AI under test
	p.Teams[2].Type = game.TeamComputer // enemy B: high score, far off
	g := sim.NewWithGrid(game.NewGameState(p), mgr, grid, w, h)
	g.Vis = nil // bare harness: every enemy counts as spotted and visible

	deployBuilding(g, 1, 90, assets.MiningSt, 4, 4) // the base centre the force sorties from

	_ = armedTank(0, 10, 8, 8, mgr)          // enemy A, closer to the base
	farHigh := armedTank(2, 20, 20, 20, mgr) // enemy B, farther
	g.TeamPoints[0] = 1
	g.TeamPoints[2] = 5 // enemy B is winning, so the focus should target its team

	ta := newTaskAttackEnemies(1, &aiContext{Game: g, MakeUnit: testUnitFactory})
	if focus := ta.teamFocusTarget(); focus != farHigh {
		t.Errorf("focus targeted %v, want the higher scoring team 2's unit despite the nearer enemy", focus)
	}
}

// TestAIForceConvergesOnSharedObjective: once committed, every attacker drives at the ONE
// shared objective instead of each hunting its own nearest enemy. Two attackers sit beside
// two different enemies; with a focus both pick the same target, and that differs from the
// nearest one attacker would have chosen alone.
func TestAIForceConvergesOnSharedObjective(t *testing.T) {
	const w, h = 24, 24
	mgr := units.NewManager()
	grid := &access.Grid{W: w, H: h, Surf: allLand{}, Units: mgr.Cells}
	p := &game.StartParams{}
	p.Teams[0].Type = game.TeamPlayer
	p.Teams[1].Type = game.TeamComputer
	g := sim.NewWithGrid(game.NewGameState(p), mgr, grid, w, h)
	g.Vis = nil

	deployBuilding(g, 1, 90, assets.MiningSt, 2, 2) // base centre near enemy A

	tank1 := armedTank(1, 1, 3, 3, mgr)
	tank2 := armedTank(1, 2, 22, 22, mgr)
	enemyA := armedTank(0, 10, 2, 8, mgr)   // near the base: the focus
	enemyB := armedTank(0, 11, 22, 16, mgr) // far from the base

	ta := newTaskAttackEnemies(1, &aiContext{Game: g, MakeUnit: testUnitFactory})

	// Control: alone, each attacker hunts its own nearest, and they differ.
	if ta.nearestEnemy(tank1) == ta.nearestEnemy(tank2) {
		t.Fatal("setup: the two attackers should have different nearest enemies")
	}

	ta.focus = ta.teamFocusTarget()
	if ta.focus != enemyA {
		t.Fatalf("focus = %v, want enemyA (nearest the base centre)", ta.focus)
	}

	if got1, got2 := ta.chooseTarget(tank1), ta.chooseTarget(tank2); got1 != enemyA || got2 != enemyA {
		t.Errorf("force did not converge: tank1 -> %v, tank2 -> %v, want both enemyA", got1, got2)
	}
	_ = enemyB
}

// TestAIUnitSelfDefendsOverFocus: a unit an enemy can already shoot answers that gun even
// when a distant shared objective exists, so it is not marched past a live threat.
func TestAIUnitSelfDefendsOverFocus(t *testing.T) {
	const w, h = 24, 24
	mgr := units.NewManager()
	grid := &access.Grid{W: w, H: h, Surf: allLand{}, Units: mgr.Cells}
	p := &game.StartParams{}
	p.Teams[0].Type = game.TeamPlayer
	p.Teams[1].Type = game.TeamComputer
	g := sim.NewWithGrid(game.NewGameState(p), mgr, grid, w, h)
	g.Vis = nil

	deployBuilding(g, 1, 90, assets.MiningSt, 2, 2)

	aiTank := armedTank(1, 1, 5, 5, mgr)
	adjacent := armedTank(0, 10, 6, 5, mgr)                       // one cell away: in range, a live threat
	facility := deployBuilding(g, 0, 91, assets.Greenhse, 22, 22) // a key facility far off: the natural focus

	ta := newTaskAttackEnemies(1, &aiContext{Game: g, MakeUnit: testUnitFactory})
	if ta.focus = ta.teamFocusTarget(); ta.focus != facility {
		t.Fatalf("focus = %v, want the distant key facility", ta.focus)
	}

	if got := ta.chooseTarget(aiTank); got != adjacent {
		t.Errorf("self defence failed: chose %v, want the adjacent threat over the distant focus", got)
	}
}

// TestAIWeaponClassFallbackFromFocus: when the shared objective is a class this unit cannot
// hit, the unit falls back to its own nearest valid target instead of chasing something it
// can never engage. An anti air flak whose focus is a ground unit hunts the air enemy.
func TestAIWeaponClassFallbackFromFocus(t *testing.T) {
	const w, h = 24, 24
	mgr := units.NewManager()
	grid := &access.Grid{W: w, H: h, Surf: allLand{}, Units: mgr.Cells}
	p := &game.StartParams{}
	p.Teams[0].Type = game.TeamPlayer
	p.Teams[1].Type = game.TeamComputer
	g := sim.NewWithGrid(game.NewGameState(p), mgr, grid, w, h)
	g.Vis = nil

	deployBuilding(g, 1, 90, assets.MiningSt, 2, 2)

	flak := armedUnit(1, 1, assets.SpFlak, game.FlagMobileLandUnit|game.FlagSelectable, 5, 5, mgr)
	ground := armedUnit(0, 10, assets.Tank, game.FlagMobileLandUnit|game.FlagSelectable, 2, 10, mgr) // near the base: the focus, but out of the flak's range and a class it cannot hit
	air := armedUnit(0, 11, assets.Awac, game.FlagMobileAirUnit|game.FlagHovering|game.FlagSelectable, 18, 18, mgr)

	ta := newTaskAttackEnemies(1, &aiContext{Game: g, MakeUnit: testUnitFactory})
	if ta.focus = ta.teamFocusTarget(); ta.focus != ground {
		t.Fatalf("focus = %v, want the ground unit nearest the base", ta.focus)
	}

	if got := ta.chooseTarget(flak); got != air {
		t.Errorf("class fallback failed: chose %v, want the air enemy the flak can actually hit", got)
	}
}

// TestResolveWinGoalKeepsSavedGoal: a game state that already carries a committed goal
// (a loaded save; AIWinGoal stores goal+1) is honoured, NOT re-rolled - even when the
// fresh roll would pick the other goal.
func TestResolveWinGoalKeepsSavedGoal(t *testing.T) {
	prev := winGoalRoll
	defer func() { winGoalRoll = prev }()
	winGoalRoll = func() int { return 0 } // a roll would say colonise

	g := &sim.Game{State: &game.GameState{}}
	g.State.AIWinGoal[1] = int(goalKill) + 1 // the save committed team 1 to Kill

	if got := resolveWinGoal(g, 1, OpponentGod); got != goalKill {
		t.Fatalf("a saved goal must be kept: got %s, want kill", goalName(got))
	}

	if g.State.AIWinGoal[1] != int(goalKill)+1 {
		t.Errorf("the saved goal slot must be unchanged, got %d", g.State.AIWinGoal[1])
	}
}

// TestResolveWinGoalWritesBackFreshRoll: an unset slot (a new game / pre v17 save)
// rolls a goal and writes it (goal+1) back onto the game state so a save captures it.
func TestResolveWinGoalWritesBackFreshRoll(t *testing.T) {
	prev := winGoalRoll
	defer func() { winGoalRoll = prev }()
	winGoalRoll = func() int { return 0 } // a winning colonise roll

	g := &sim.Game{VictoryType: winloss.VictoryDuration, State: &game.GameState{}}

	if got := resolveWinGoal(g, 2, OpponentGod); got != goalColonize {
		t.Fatalf("an unset slot should roll: got %s, want colonize", goalName(got))
	}

	if g.State.AIWinGoal[2] != int(goalColonize)+1 {
		t.Errorf("the rolled goal must be written back as goal+1, got %d", g.State.AIWinGoal[2])
	}
}

// TestResolveStrategyKeepsSavedStrategy: a game state that already carries a committed
// strategy (a loaded save; AIStrategy stores strategy+1) is honoured, NOT overwritten by
// the freshly rolled `chosen` argument.
func TestResolveStrategyKeepsSavedStrategy(t *testing.T) {
	g := &sim.Game{State: &game.GameState{}}
	g.State.AIStrategy[1] = int(StrategySea) + 1 // the save committed team 1 to Sea

	// SelectStrategy rolled Air this session, but the save's Sea must win.
	if got := resolveStrategy(g, 1, StrategyAir); got != StrategySea {
		t.Fatalf("a saved strategy must be kept: got %s, want sea", strategyName(got))
	}

	if g.State.AIStrategy[1] != int(StrategySea)+1 {
		t.Errorf("the saved strategy slot must be unchanged, got %d", g.State.AIStrategy[1])
	}
}

// TestResolveStrategyWritesBackChosen: an unset slot (a new game / pre v18 save) keeps the
// freshly chosen strategy and writes it (strategy+1) back onto the game state so a save
// captures it.
func TestResolveStrategyWritesBackChosen(t *testing.T) {
	g := &sim.Game{State: &game.GameState{}}

	if got := resolveStrategy(g, 2, StrategyTankHorde); got != StrategyTankHorde {
		t.Fatalf("an unset slot should keep the chosen strategy: got %s, want tank_horde", strategyName(got))
	}

	if g.State.AIStrategy[2] != int(StrategyTankHorde)+1 {
		t.Errorf("the chosen strategy must be written back as strategy+1, got %d", g.State.AIStrategy[2])
	}
}

// TestChooseWinGoalScoreVictory: a score victory game pins the AI to Colonize at every
// tier (the win path IS eco spheres - a score mission's scripted goal, honoured).
func TestChooseWinGoalScoreVictory(t *testing.T) {
	g := &sim.Game{VictoryType: winloss.VictoryScore, VictoryLimit: 100}
	for tier := OpponentClueless; tier <= OpponentGod; tier++ {
		if got := chooseWinGoal(g, tier); got != goalColonize {
			t.Errorf("score victory, tier %d: goal %s, want colonize", tier, goalName(got))
		}
	}
}

// TestChooseWinGoalSkirmish: a non score game keeps the easy/average tiers on Kill
// (colonise chance 0), while a forced roll lets a top tier choose to out colonise.
func TestChooseWinGoalSkirmish(t *testing.T) {
	g := &sim.Game{VictoryType: winloss.VictoryDuration} // no score limit
	for _, tier := range []int{OpponentClueless, OpponentApprentice, OpponentAverage} {
		if got := chooseWinGoal(g, tier); got != goalKill {
			t.Errorf("skirmish, tier %d: goal %s, want kill (colonise chance 0)", tier, goalName(got))
		}
	}
	prev := winGoalRoll
	defer func() { winGoalRoll = prev }()
	winGoalRoll = func() int { return 0 } // a winning colonise roll: a God CAN out colonise
	if got := chooseWinGoal(g, OpponentGod); got != goalColonize {
		t.Errorf("God with a winning colonise roll: goal %s, want colonize", goalName(got))
	}
	winGoalRoll = func() int { return 99 } // a missed roll: even God falls back to the kill
	if got := chooseWinGoal(g, OpponentGod); got != goalKill {
		t.Errorf("God with a missed colonise roll: goal %s, want kill", goalName(got))
	}
}

// rosterHas reports whether a strategic build roster includes a unit type.
func rosterHas(orders []buildSpec, ut assets.UnitType) bool {
	for _, o := range orders {
		if o.t == ut {
			return true
		}
	}
	return false
}

// TestColonizeBuildRoster: a colonising team's strategic backbone includes the
// Eco sphere (so its constructors raise them and its score climbs); a killer's does not.
func TestColonizeBuildRoster(t *testing.T) {
	if !rosterHas(strategicBuildOrders(StrategyCombinedArms, goalColonize, false), assets.Greenhse) {
		t.Error("colonize roster has no Eco-sphere; the team would never build for score")
	}
	if rosterHas(strategicBuildOrders(StrategyCombinedArms, goalKill, false), assets.Greenhse) {
		t.Error("kill roster includes an Eco-sphere; a killer should pour constructors into the military")
	}
}

// TestCoastalBuildRoster: a coastal team's strategic backbone includes a Shipyard (so
// it fields a navy); a landlocked team's never does (an unusable dock). A landlocked
// Sea strategy team falls back to an air plant instead of a shipyard.
func TestCoastalBuildRoster(t *testing.T) {
	if !rosterHas(strategicBuildOrders(StrategyCombinedArms, goalKill, true), assets.Shipyard) {
		t.Error("coastal roster has no Shipyard; a seaside team would never field a navy")
	}
	if rosterHas(strategicBuildOrders(StrategyCombinedArms, goalKill, false), assets.Shipyard) {
		t.Error("landlocked roster includes a Shipyard; a landlocked team must not build an unusable dock")
	}
	if rosterHas(strategicBuildOrders(StrategySea, goalKill, false), assets.Shipyard) {
		t.Error("landlocked Sea-strategy roster still has a Shipyard; it should fall back to an air plant")
	}
	if !rosterHas(strategicBuildOrders(StrategySea, goalKill, false), assets.AirPlt) {
		t.Error("landlocked Sea-strategy roster has no air plant fallback")
	}
}

// TestColonizerQueuesEcoSphereAfterBackbone: once a colonising team owns its military
// backbone, the next strategic build it selects is the Eco sphere (the score race),
// where a killer would have nothing left to build.
func TestColonizerQueuesEcoSphereAfterBackbone(t *testing.T) {
	const w, h = 16, 16
	mgr := units.NewManager()
	grid := &access.Grid{W: w, H: h, Surf: allLand{}, Units: mgr.Cells}
	p := &game.StartParams{}
	p.Teams[1].Type = game.TeamComputer
	g := sim.NewWithGrid(game.NewGameState(p), mgr, grid, w, h)

	// The whole kill backbone already stands, so the only unmet colonize order is the
	// Eco sphere. The backbone is the full roster: both production lines of each kind, the
	// light plant, the depot, the lab and the training hall.
	stationary(1, 10, assets.LandPlt, 4, 4, mgr)
	stationary(1, 11, assets.AirPlt, 6, 4, mgr)
	stationary(1, 12, assets.Depot, 8, 4, mgr)
	stationary(1, 13, assets.Research, 10, 4, mgr)
	stationary(1, 14, assets.LightPlt, 12, 4, mgr)
	stationary(1, 15, assets.TrainHal, 4, 8, mgr)
	stationary(1, 16, assets.LandPlt, 6, 8, mgr) // the second heavy line
	stationary(1, 17, assets.AirPlt, 8, 8, mgr)  // the second air line

	if spec, ok := nextBuilding(g, 1, strategicBuildOrders(StrategyCombinedArms, goalColonize, false)); !ok || spec.t != assets.Greenhse {
		t.Errorf("next colonize build = %v (ok=%v), want the Eco-sphere", spec.t, ok)
	}
	if spec, ok := nextBuilding(g, 1, strategicBuildOrders(StrategyCombinedArms, goalKill, false)); ok {
		t.Errorf("a killer with its whole backbone built still wanted %v; want nothing", spec.t)
	}
}

// waterColumn is a synthetic SurfaceSource: every cell is land except a single water
// column at waterX (a simple coast for the terrain test).
type waterColumn struct{ waterX int }

func (w waterColumn) SurfaceAt(x, y int) uint8 {
	if x == w.waterX {
		return world.SurfaceWater
	}
	return world.SurfaceLand
}

// TestBaseIsCoastal: a building within the coastal scan radius of water reads as
// coastal; one far inland reads as landlocked.
func TestBaseIsCoastal(t *testing.T) {
	const w, h = 24, 24
	newGame := func(bx, by int) *sim.Game {
		mgr := units.NewManager()
		grid := &access.Grid{W: w, H: h, Surf: waterColumn{waterX: 10}, Units: mgr.Cells}
		p := &game.StartParams{}
		p.Teams[1].Type = game.TeamComputer
		g := sim.NewWithGrid(game.NewGameState(p), mgr, grid, w, h)
		stationary(1, 10, assets.MiningSt, bx, by, mgr)
		return g
	}
	if !baseIsCoastal(newGame(6, 6), 1) { // 4 cells from the water column
		t.Error("a base within the coastal radius of water should read as coastal")
	}
	if baseIsCoastal(newGame(20, 20), 1) { // far from the water column
		t.Error("a base far inland should read as landlocked")
	}
}

// TestDifficultyProfiles checks the tier table and applyDifficulty wiring: AVERAGE
// reproduces the pre difficulty constants, CLUELESS turns retreat off and goes
// defensive, GOD musters at one and mines hardest.
func TestDifficultyProfiles(t *testing.T) {
	if p := profileFor(OpponentAverage); p.muster != 3 || !p.retreat || p.miningFactor != 4 || p.defensiveOnly {
		t.Errorf("AVERAGE profile = %+v, want muster 3 / retreat / mining 4 / hunt", p)
	}
	setDifficulty(t, OpponentClueless)
	if minStrikeForce != 6 || retreatEnabled || aiDifficultyFactor != 3 || !aiDefensiveOnly {
		t.Errorf("CLUELESS knobs = muster %d retreat %v mining %d defensive %v, want 6/false/3/true",
			minStrikeForce, retreatEnabled, aiDifficultyFactor, aiDefensiveOnly)
	}
	setDifficulty(t, OpponentGod)
	if minStrikeForce != 1 || aiDifficultyFactor != 6 || aiDefensiveOnly {
		t.Errorf("GOD knobs = muster %d mining %d defensive %v, want 1/6/false",
			minStrikeForce, aiDifficultyFactor, aiDefensiveOnly)
	}
}

// TestEasyAIStaysDefensive: a CLUELESS (defensive only) team holds rather than
// marching out at a spotted, out of range enemy. The map is small and fully scanned
// so taskExplore has no frontier; the only candidate order is an attack advance,
// which the defensive tier suppresses.
func TestEasyAIStaysDefensive(t *testing.T) {
	const w, h = 14, 14
	setDifficulty(t, OpponentClueless)
	mgr := units.NewManager()
	grid := &access.Grid{W: w, H: h, Surf: allLand{}, Units: mgr.Cells}
	p := &game.StartParams{}
	p.Teams[0].Type = game.TeamPlayer
	p.Teams[1].Type = game.TeamComputer
	g := sim.NewWithGrid(game.NewGameState(p), mgr, grid, w, h)
	g.State.Difficulty = aiTier // the CLUELESS tier set above is the game's tier the AI reads on init

	aiTank := armedTank(1, 1, 5, 5, mgr)
	seeAll(aiTank, w)
	_ = armedTank(0, 2, 12, 5, mgr) // player, spotted, out of range

	g.AI = New(testUnitFactory)
	g.BeginTeamTurn(1)
	if g.HasOrder(aiTank) {
		t.Error("an Easy (defensive) AI marched out at an out-of-range enemy instead of holding")
	}
}

// (God's muster of one commit is covered by TestGodGraceFloor, which exercises it
// past the grace turn; the pre arc TestGodAICommitsImmediately was superseded by the
// build up grace - even God now waits for its grace turn before assaulting.)

// seeAll grants a unit map wide scan so the spotting gate is satisfied and the
// test can focus on movement/firing geometry rather than reconnaissance.
func seeAll(u *game.UnitInfo, mapSize int) {
	u.Values.Set(game.AttribScan, mapSize*2)
}

// TestAITargetsOnlyWhatItCanHit: a self propelled flak (mobile land, can target
// ONLY air) ignores a ground enemy but engages an air enemy. A tank (no air
// targeting) ignores a lone hovering aircraft.
func TestAITargetsOnlyWhatItCanHit(t *testing.T) {
	// Small map fully covered by the units' scan, so every enemy is spotted and
	// taskExplore has no frontier to chase - the only orders come from the attack
	// task, and one attacker is enough to commit.
	const w, h = 14, 14
	setMinStrikeForce(t, 1)
	mgr := units.NewManager()
	grid := &access.Grid{W: w, H: h, Surf: allLand{}, Units: mgr.Cells}
	p := &game.StartParams{}
	p.Teams[0].Type = game.TeamPlayer
	p.Teams[1].Type = game.TeamComputer
	g := sim.NewWithGrid(game.NewGameState(p), mgr, grid, w, h)

	flak := armedUnit(1, 1, assets.SpFlak, game.FlagMobileLandUnit|game.FlagSelectable, 5, 5, mgr)
	tank := armedUnit(1, 2, assets.Tank, game.FlagMobileLandUnit|game.FlagSelectable, 6, 5, mgr)
	seeAll(flak, w)
	seeAll(tank, w)
	_ = armedUnit(0, 3, assets.Tank, game.FlagMobileLandUnit|game.FlagSelectable, 12, 5, mgr) // ground enemy, spotted, out of range

	g.AI = New(testUnitFactory)
	g.BeginTeamTurn(1)
	if g.HasOrder(flak) {
		t.Error("flak advanced toward a ground enemy it cannot target")
	}
	if !g.HasOrder(tank) {
		t.Error("tank did not advance toward the ground enemy it can target")
	}

	// Add a hovering air enemy in range of the flak's scan: the flak can now
	// engage it, the tank still cannot.
	_ = armedUnit(0, 4, assets.Awac, game.FlagMobileAirUnit|game.FlagHovering|game.FlagSelectable, 11, 6, mgr)
	g.BeginTeamTurn(1)
	if !g.HasOrder(flak) {
		t.Error("flak did not engage an air enemy it can target")
	}
}

// TestAIAttacksEnemyInRange checks the C3 combat task: a computer tank fires on
// an enemy already within range on its turn.
func TestAIAttacksEnemyInRange(t *testing.T) {
	const w, h = 16, 16
	mgr := units.NewManager()
	grid := &access.Grid{W: w, H: h, Surf: allLand{}, Units: mgr.Cells}
	p := &game.StartParams{}
	p.Teams[0].Type = game.TeamPlayer
	p.Teams[1].Type = game.TeamComputer
	g := sim.NewWithGrid(game.NewGameState(p), mgr, grid, w, h)

	aiTank := armedTank(1, 1, 5, 5, mgr) // computer
	aiTank.Values.Set(game.AttribScan, 6)
	enemy := armedTank(0, 2, 7, 5, mgr) // player, 2 cells away (within range 4)
	startHits := enemy.Hits

	g.AI = New(testUnitFactory)
	g.BeginTeamTurn(1)

	// Defence does not wait for a muster: a lone unit still fires on a spotted
	// enemy already in range.
	if enemy.Hits >= startHits {
		t.Errorf("AI did not fire on the in-range enemy: hits %d (start %d)", enemy.Hits, startHits)
	}
}

// TestAIAdvancesTowardEnemy checks that an out of range computer unit issues a
// move order toward the enemy.
func TestAIAdvancesTowardEnemy(t *testing.T) {
	const w, h = 24, 24
	setMinStrikeForce(t, 1) // isolate the advance from the muster ramp
	mgr := units.NewManager()
	grid := &access.Grid{W: w, H: h, Surf: allLand{}, Units: mgr.Cells}
	p := &game.StartParams{}
	p.Teams[0].Type = game.TeamPlayer
	p.Teams[1].Type = game.TeamComputer
	g := sim.NewWithGrid(game.NewGameState(p), mgr, grid, w, h)

	aiTank := armedTank(1, 1, 2, 2, mgr) // computer
	seeAll(aiTank, w)                    // the enemy is spotted, so the AI may pursue it
	_ = armedTank(0, 2, 20, 20, mgr)     // player, far away (out of range)

	g.AI = New(testUnitFactory)
	g.BeginTeamTurn(1)

	if !g.HasOrder(aiTank) {
		t.Error("AI tank did not advance toward the distant enemy")
	}
}

// TestAIClosesAndEngages is the end to end C3 check: over several turns a
// computer tank advances on a stationary enemy and eventually destroys it.
func TestAIClosesAndEngages(t *testing.T) {
	const w, h = 24, 24
	mgr := units.NewManager()
	grid := &access.Grid{W: w, H: h, Surf: allLand{}, Units: mgr.Cells}
	p := &game.StartParams{}
	p.Teams[0].Type = game.TeamPlayer
	p.Teams[1].Type = game.TeamComputer
	g := sim.NewWithGrid(game.NewGameState(p), mgr, grid, w, h)

	setMinStrikeForce(t, 1)              // a single committed attacker; the muster ramp is tested separately
	aiTank := armedTank(1, 1, 2, 2, mgr) // computer, fast + armed
	seeAll(aiTank, w)                    // keeps the enemy spotted across the whole approach
	enemy := armedTank(0, 2, 16, 2, mgr) // player, stationary target far away
	start := chebyshev(aiTank.GridX, aiTank.GridY, 16, 2)
	g.AI = New(testUnitFactory)

	// Run the computer team's turns, resolving its moves each turn so the tank
	// actually closes the distance. (Only team 1 acts; team 0 holds still.)
	g.RunRounds(10, func(g *sim.Game, team int) {
		if team == 1 {
			g.ResolveMovement()
		}
	})

	if chebyshev(aiTank.GridX, aiTank.GridY, 16, 2) >= start {
		t.Errorf("AI tank made no progress: still %d cells away (start %d), enemy hits %d",
			chebyshev(aiTank.GridX, aiTank.GridY, 16, 2), start, enemy.Hits)
	}
}

// TestAIStandsOffAtRange checks the stand off positioning: a long range (range 4)
// computer unit closes on a stationary enemy only to its weapon range and fires
// from there - it never walks adjacent (the old advance to contact behaviour),
// kiting instead of meleeing.
func TestAIStandsOffAtRange(t *testing.T) {
	const w, h = 32, 32
	mgr := units.NewManager()
	grid := &access.Grid{W: w, H: h, Surf: allLand{}, Units: mgr.Cells}
	p := &game.StartParams{}
	p.Teams[0].Type = game.TeamPlayer
	p.Teams[1].Type = game.TeamComputer
	g := sim.NewWithGrid(game.NewGameState(p), mgr, grid, w, h)

	setMinStrikeForce(t, 1)               // a single committed attacker; the muster ramp is tested separately
	aiTank := armedTank(1, 1, 2, 16, mgr) // computer, range 4
	seeAll(aiTank, w)                     // keeps the enemy spotted across the whole approach
	enemy := armedTank(0, 2, 24, 16, mgr) // player, stationary, far away
	// Make the enemy durable so it survives the approach and we can observe that
	// the attacker holds at range rather than closing to melee.
	enemy.Values.Set(game.AttribHits, 400)
	enemy.Hits = 400
	startHits := enemy.Hits

	g.AI = New(testUnitFactory)

	// Run the computer's turns, resolving its moves, tracking how close the tank
	// ever gets to the (fixed) enemy cell.
	minDist := 1 << 30
	g.RunRounds(12, func(g *sim.Game, team int) {
		if team == 1 {
			g.ResolveMovement()
		}
		if d := chebyshev(aiTank.GridX, aiTank.GridY, enemy.GridX, enemy.GridY); d < minDist {
			minDist = d
		}
	})

	if minDist <= 1 {
		t.Errorf("attacker closed to melee (min distance %d); expected a stand-off at weapon range", minDist)
	}
	if minDist > 4 {
		t.Errorf("attacker never reached weapon range (min distance %d, range 4)", minDist)
	}
	if enemy.Hits >= startHits {
		t.Errorf("attacker did not fire from stand-off range: enemy hits %d (start %d)", enemy.Hits, startHits)
	}
}

// aiAirAttacker deploys a computer airborne (FlagHovering) flyer set up to run a
// fly over: range 4, MOVE_AND_FIRE so firing leaves the move budget intact for the
// overflight (real air units are move and fire), with map wide scan so its target
// is spotted. Pass hovering=false for a LANDED air unit (fires in place, no sortie).
func aiAirAttacker(team, id, gx, gy int, hovering bool, mapW int, mgr *units.Manager) *game.UnitInfo {
	flags := game.FlagMobileAirUnit | game.FlagSelectable
	if hovering {
		flags |= game.FlagHovering
	}
	u := armedUnit(team, id, assets.Bomber, flags, gx, gy, mgr)
	u.Values.Set(game.AttribMoveAndFire, 1)
	seeAll(u, mapW)
	return u
}

// TestAIAirAttackerFiresInPlace checks the air attack routing: an airborne air unit
// attacks like a land/sea unit - it fires from its current cell on an in range target
// and HOLDS position; it does not run a strafing pass over and past the target. A
// landed air unit behaves the same. (Regression guard for the plane fly past bug.)
func TestAIAirAttackerFiresInPlace(t *testing.T) {
	const w, h = 24, 24

	fireInPlace := func(t *testing.T, hovering bool) {
		setMinStrikeForce(t, 1) // a single committed attacker; the muster ramp is tested separately
		mgr := units.NewManager()
		grid := &access.Grid{W: w, H: h, Surf: allLand{}, Units: mgr.Cells}
		p := &game.StartParams{}
		p.Teams[0].Type = game.TeamPlayer
		p.Teams[1].Type = game.TeamComputer
		g := sim.NewWithGrid(game.NewGameState(p), mgr, grid, w, h)

		// Computer plane 3 cells west of the enemy, within its range 4.
		bomber := aiAirAttacker(1, 1, 5, 8, hovering, w, mgr)
		enemy := armedTank(0, 2, 8, 8, mgr)    // ground enemy 3 cells away (within range 4)
		enemy.Values.Set(game.AttribHits, 400) // durable, so it survives the shot and stays put
		enemy.Hits = 400

		g.AI = New(testUnitFactory)
		g.BeginTeamTurn(1)

		// A plane's ordnance is a launched ROCKET that explodes on arrival, so it
		// shows up as a projectile in flight - fired from its cell, no fly past move.
		if n := len(g.ActiveProjectiles()); n != 1 {
			t.Errorf("the in range plane launched no ordnance: %d projectiles in flight, want 1", n)
		}

		g.ResolveMovement()
		if bomber.GridX != 5 || bomber.GridY != 8 {
			t.Errorf("the plane drifted off its firing cell to (%d,%d); an air unit fires in place, want (5,8)",
				bomber.GridX, bomber.GridY)
		}
	}

	t.Run("airborne", func(t *testing.T) { fireInPlace(t, true) })
	t.Run("landed", func(t *testing.T) { fireInPlace(t, false) })
}

// TestAIIgnoresUnspottedEnemy verifies that a computer unit
// with no scan coverage of a distant enemy does NOT pursue it. The AI acts only
// on enemies it has actually spotted, so toggling fog off no longer reveals a
// turn-1 rush. minStrikeForce is dropped to 1 so the only thing that can hold the
// unit back is the spotting gate, not the muster ramp.
func TestAIIgnoresUnspottedEnemy(t *testing.T) {
	const w, h = 24, 24
	setMinStrikeForce(t, 1)
	mgr := units.NewManager()
	grid := &access.Grid{W: w, H: h, Surf: allLand{}, Units: mgr.Cells}
	p := &game.StartParams{}
	p.Teams[0].Type = game.TeamPlayer
	p.Teams[1].Type = game.TeamComputer
	g := sim.NewWithGrid(game.NewGameState(p), mgr, grid, w, h)

	aiTank := armedTank(1, 1, 2, 2, mgr) // computer, scan 0: sees only its own cell
	_ = armedTank(0, 2, 20, 20, mgr)     // player, far away in the fog

	g.AI = New(testUnitFactory)
	g.BeginTeamTurn(1)

	if g.HasOrder(aiTank) {
		t.Error("AI pursued an enemy it has not spotted (the omniscient rush)")
	}
}

// TestAIPursuesLastSeenPosition: once the AI has spotted an enemy that then flees
// into fog, it pursues the LAST SEEN cell (the SpottedUnit memory), not the enemy's
// true position. The enemy is spotted to the NORTH, then flees SOUTH out of scan; the
// AI tank must advance north (toward where it last saw it), proving it does not read
// the fogged enemy's real cell.
func TestAIPursuesLastSeenPosition(t *testing.T) {
	const w, h = 24, 24
	setMinStrikeForce(t, 1) // a single committed attacker; isolate from the muster ramp
	mgr := units.NewManager()
	grid := &access.Grid{W: w, H: h, Surf: allLand{}, Units: mgr.Cells}
	p := &game.StartParams{}
	p.Teams[0].Type = game.TeamPlayer
	p.Teams[1].Type = game.TeamComputer
	g := sim.NewWithGrid(game.NewGameState(p), mgr, grid, w, h)

	aiTank := armedTank(1, 1, 12, 12, mgr) // computer
	aiTank.Values.Set(game.AttribScan, 7)  // sees out 7 cells: enemy at dist 5 yes, dist 8 no
	enemy := armedTank(0, 2, 12, 7, mgr)   // player, 5 cells NORTH: spotted, out of range 4
	g.RefreshVisibility()                  // record the sighting at (12,7)
	if !g.SpottedByTeam(enemy, 1) {
		t.Fatal("setup: the enemy should be spotted to the north")
	}

	g.Units.Move(enemy, 12, 20) // flee 8 cells SOUTH into fog
	g.RefreshVisibility()
	if g.Vis.VisibleToTeam(enemy, 1) {
		t.Fatal("setup: the fled enemy should now be in fog")
	}

	g.AI = New(testUnitFactory)
	g.BeginTeamTurn(1) // pursue the last seen cell (12,7), north of the tank
	g.ResolveMovement()

	if aiTank.GridY >= 12 {
		t.Errorf("AI tank at y=%d: it did not advance NORTH toward the last-seen cell "+
			"(it should pursue (12,7), not the true position (12,20))", aiTank.GridY)
	}
}

// TestZoneWalkerCoversDisc: the ZoneWalker port visits exactly the cells of the
// Euclidean disc around its centre, in bounds, including the centre and the rim.
func TestZoneWalkerCoversDisc(t *testing.T) {
	const w, h = 20, 20
	const r = 5
	zw := newZoneWalker(game.Point{X: 10, Y: 10}, r, w, h)
	count := 0
	sawCenter, sawRim := false, false
	for {
		x, y := zw.x, zw.y
		dx, dy := x-10, y-10
		if dx*dx+dy*dy > r*r {
			t.Fatalf("zone walker yielded (%d,%d) outside the disc radius %d", x, y, r)
		}
		if x < 0 || y < 0 || x >= w || y >= h {
			t.Fatalf("zone walker yielded out-of-bounds (%d,%d)", x, y)
		}
		if x == 10 && y == 10 {
			sawCenter = true
		}
		if x == 10 && y == 10-r {
			sawRim = true
		}
		count++
		if !zw.findNext() {
			break
		}
	}
	if !sawCenter {
		t.Error("zone walker missed the centre cell")
	}
	if !sawRim {
		t.Error("zone walker missed the top rim cell")
	}
	if count < 70 { // a radius-5 lattice disc holds 81 cells
		t.Errorf("zone walker visited only %d cells; want the full disc (~81)", count)
	}
}

// TestAIZoneSearchSweepsUnexploredDisc: with mostly fogged ground, a unit told to
// zone search around a cell issues a move toward an unexplored cell of that disc.
func TestAIZoneSearchSweepsUnexploredDisc(t *testing.T) {
	const w, h = 24, 24
	mgr := units.NewManager()
	grid := &access.Grid{W: w, H: h, Surf: allLand{}, Units: mgr.Cells}
	p := &game.StartParams{}
	p.Teams[1].Type = game.TeamComputer
	g := sim.NewWithGrid(game.NewGameState(p), mgr, grid, w, h)

	hunter := armedTank(1, 1, 12, 12, mgr)
	hunter.Values.Set(game.AttribScan, 2) // only a small bubble around (12,12) is explored
	g.RefreshVisibility()

	ta := newTaskAttackEnemies(1, &aiContext{Game: g, MakeUnit: testUnitFactory})
	if !ta.zoneSearch(hunter, 12, 12) {
		t.Fatal("zone search should move the hunter toward an unexplored cell in the disc")
	}
	if !g.HasOrder(hunter) {
		t.Error("zone search issued no move order")
	}
}

// TestAIZoneSearchStopsWhenZoneExplored: once the whole disc has been scanned, the
// zone search finds nothing and stops (so the unit can fall back to generic explore).
func TestAIZoneSearchStopsWhenZoneExplored(t *testing.T) {
	const w, h = 24, 24
	mgr := units.NewManager()
	grid := &access.Grid{W: w, H: h, Surf: allLand{}, Units: mgr.Cells}
	p := &game.StartParams{}
	p.Teams[1].Type = game.TeamComputer
	g := sim.NewWithGrid(game.NewGameState(p), mgr, grid, w, h)

	hunter := armedTank(1, 1, 12, 12, mgr)
	hunter.Values.Set(game.AttribScan, w*2) // sees the whole map -> everything explored
	g.RefreshVisibility()

	ta := newTaskAttackEnemies(1, &aiContext{Game: g, MakeUnit: testUnitFactory})
	if ta.zoneSearch(hunter, 12, 12) {
		t.Error("with the whole zone explored, the zone search should find nothing and stop")
	}
	if g.HasOrder(hunter) {
		t.Error("a stopped zone search must not issue a move")
	}
}

// TestAISearchModeSelection: the RNG seam picks the zone sweep or the generic explore
// and latches it on the unit.
func TestAISearchModeSelection(t *testing.T) {
	const w, h = 16, 16
	mgr := units.NewManager()
	grid := &access.Grid{W: w, H: h, Surf: allLand{}, Units: mgr.Cells}
	p := &game.StartParams{}
	p.Teams[1].Type = game.TeamComputer
	g := sim.NewWithGrid(game.NewGameState(p), mgr, grid, w, h)
	hunter := armedTank(1, 1, 8, 8, mgr)
	hunter.Values.Set(game.AttribScan, 2)
	enemy := armedTank(0, 2, 9, 8, mgr)
	g.RefreshVisibility()

	ta := newTaskAttackEnemies(1, &aiContext{Game: g, MakeUnit: testUnitFactory})
	defer func(prev func() bool) { pickZoneSearch = prev }(pickZoneSearch)

	pickZoneSearch = func() bool { return true }
	ta.beginSearch(hunter, enemy, 8, 8)
	if ta.searchMode[hunter] != searchZone {
		t.Error("RNG true should choose the focused zone search")
	}

	delete(ta.searchMode, hunter)
	pickZoneSearch = func() bool { return false }
	ta.beginSearch(hunter, enemy, 8, 8)
	if ta.searchMode[hunter] != searchExplore {
		t.Error("RNG false should choose the generic explore search")
	}
}

// TestAIShelvesTargetAfterFailedSearch: a zone search that finds nothing reachable to
// reveal around a lost target's last seen cell shelves (abandons) that target instead
// of latching onto the search again every turn.
func TestAIShelvesTargetAfterFailedSearch(t *testing.T) {
	const w, h = 16, 16
	mgr := units.NewManager()
	grid := &access.Grid{W: w, H: h, Surf: allLand{}, Units: mgr.Cells}
	p := &game.StartParams{}
	p.Teams[1].Type = game.TeamComputer
	g := sim.NewWithGrid(game.NewGameState(p), mgr, grid, w, h)

	hunter := armedTank(1, 1, 8, 8, mgr)
	hunter.Values.Set(game.AttribScan, w*2) // sees the whole map -> everything explored
	enemy := armedTank(0, 2, 12, 8, mgr)
	g.RefreshVisibility()

	ta := newTaskAttackEnemies(1, &aiContext{Game: g, MakeUnit: testUnitFactory})
	defer func(prev func() bool) { pickZoneSearch = prev }(pickZoneSearch)
	pickZoneSearch = func() bool { return true } // force the focused zone sweep

	ta.beginSearch(hunter, enemy, 12, 8) // the whole disc is explored: the sweep finds nothing
	if _, shelved := ta.abandoned[enemy]; !shelved {
		t.Error("a zone search that finds nothing should shelve (abandon) the target")
	}
	if _, searching := ta.searchMode[hunter]; searching {
		t.Error("the hunter should not stay latched in a futile search after shelving")
	}
}

// TestAISkipsShelvedTargetUntilCooldownOrResight: once a fogged target is shelved, the
// team stops locking onto it again (no more thrash) until the cooldown elapses or it is
// seen again, whichever comes first.
func TestAISkipsShelvedTargetUntilCooldownOrResight(t *testing.T) {
	const w, h = 24, 24
	setMinStrikeForce(t, 1)
	mgr := units.NewManager()
	grid := &access.Grid{W: w, H: h, Surf: allLand{}, Units: mgr.Cells}
	p := &game.StartParams{}
	p.Teams[0].Type = game.TeamPlayer
	p.Teams[1].Type = game.TeamComputer
	g := sim.NewWithGrid(game.NewGameState(p), mgr, grid, w, h)

	hunter := armedTank(1, 1, 12, 12, mgr)
	hunter.Values.Set(game.AttribScan, 7)
	enemy := armedTank(0, 2, 12, 7, mgr) // 5 cells north: spotted
	g.RefreshVisibility()
	g.Units.Move(enemy, 12, 22) // flee far south into fog
	g.RefreshVisibility()
	if g.Vis.VisibleToTeam(enemy, 1) {
		t.Fatal("setup: the fled enemy should now be fogged")
	}

	ta := newTaskAttackEnemies(1, &aiContext{Game: g, MakeUnit: testUnitFactory})
	if ta.nearestEnemy(hunter) != enemy {
		t.Fatal("setup: the spotted fogged enemy should be a target before shelving")
	}

	// Shelve it: it is now skipped while it stays fogged within the cooldown.
	g.State.TurnCounter = 100
	ta.abandon(enemy)
	if ta.nearestEnemy(hunter) != nil {
		t.Error("a shelved fogged target should be skipped during its cooldown")
	}

	// After the cooldown elapses, the team retries it.
	g.State.TurnCounter = 100 + abandonCooldown
	if ta.nearestEnemy(hunter) != enemy {
		t.Error("after the cooldown the shelved target should be retried")
	}

	// A fresh sighting unshelves it at once.
	g.State.TurnCounter = 105
	ta.abandon(enemy)           // shelve again
	g.Units.Move(enemy, 12, 13) // back within scan of the hunter
	g.RefreshVisibility()
	if ta.nearestEnemy(hunter) != enemy {
		t.Error("a re-sighted target must be pursued again at once")
	}
	if _, shelved := ta.abandoned[enemy]; shelved {
		t.Error("re-acquiring a target should clear its shelving")
	}
}

// TestExpertAIRevealsEnemyEconomy: the EXPERT+ spot enemy cheat. An enemy Mining Station
// hidden in the fog is NOT a spotted target at AVERAGE (the team must scout it out), but
// at EXPERT+ the team is handed its location through the fog (revealedKeyFacility), so it
// becomes a target without ever having been seen. Only the economy facilities are
// revealed - a fogged enemy TANK stays unspotted at every tier.
func TestExpertAIRevealsEnemyEconomy(t *testing.T) {
	const w, h = 20, 20
	mgr := units.NewManager()
	grid := &access.Grid{W: w, H: h, Surf: allLand{}, Units: mgr.Cells}
	p := &game.StartParams{}
	p.Teams[0].Type = game.TeamPlayer
	p.Teams[1].Type = game.TeamComputer
	g := sim.NewWithGrid(game.NewGameState(p), mgr, grid, w, h)

	hunter := armedTank(1, 1, 4, 4, mgr)
	hunter.Values.Set(game.AttribScan, 2) // a small bubble: the enemy base is out of sight
	mining := stationary(0, 2, assets.MiningSt, 16, 16, mgr)
	tank := armedTank(0, 3, 16, 15, mgr) // a fogged enemy tank beside the mine
	g.RefreshVisibility()
	if g.Vis.VisibleToTeam(mining, 1) {
		t.Fatal("setup: the enemy mining station should be fogged")
	}

	ta := newTaskAttackEnemies(1, &aiContext{Game: g, MakeUnit: testUnitFactory})

	// AVERAGE (the package default tier): a never seen fogged facility is not spotted.
	if ta.spottedByTeam(mining) {
		t.Error("AVERAGE: a fogged, never-seen enemy mining station must not be spotted")
	}

	// EXPERT: the mining station is revealed through the fog; the fogged tank is not.
	setDifficulty(t, OpponentExpert)
	if !revealedKeyFacility(mining) {
		t.Error("EXPERT: revealedKeyFacility should accept a mining station")
	}
	if revealedKeyFacility(tank) {
		t.Error("EXPERT: only economy facilities are revealed, not a tank")
	}
	if !ta.spottedByTeam(mining) {
		t.Error("EXPERT: the enemy mining station should be spotted through the fog")
	}
	if ta.spottedByTeam(tank) {
		t.Error("EXPERT: a fogged, never-seen enemy tank must still be unspotted")
	}
}

// TestHarassProbePrefersLauncher: launchersFirst moves a long range launcher to the front
// of the ready list so the limited pre commitment harassment sortie sends it ahead of a
// generic tank, while the tanks keep their stable (ID) order behind it.
func TestHarassProbePrefersLauncher(t *testing.T) {
	mgr := units.NewManager()
	// Two tanks (ids 1, 3) and a rocket launcher (id 2): by ID the launcher sits in the
	// middle, so a plain ID sort would not put it first.
	tankA := armedTank(1, 1, 2, 2, mgr)
	launcher := armedTank(1, 2, 3, 3, mgr)
	launcher.Type = assets.RocktLch
	tankB := armedTank(1, 3, 4, 4, mgr)

	ta := newTaskAttackEnemies(1, nil)
	ready := []*game.UnitInfo{tankA, launcher, tankB}
	ta.launchersFirst(ready)
	if ready[0] != launcher {
		t.Errorf("launchersFirst should lead with the launcher, got %s", ready[0].Type.Tag())
	}
	if ready[1] != tankA || ready[2] != tankB {
		t.Error("launchersFirst should keep the tanks' relative order behind the launcher")
	}
}

// TestAIMustersBeforeCommitting checks the build up ramp: with the enemy spotted
// but fewer than minStrikeForce attackers fielded, the team holds near base;
// once it reaches the threshold it commits and advances. The small fully scanned
// map leaves no unexplored frontier, so a held unit issues no order at all and a
// stray order can only be the attack advance.
func TestAIMustersBeforeCommitting(t *testing.T) {
	const w, h = 10, 10
	mgr := units.NewManager()
	grid := &access.Grid{W: w, H: h, Surf: allLand{}, Units: mgr.Cells}
	p := &game.StartParams{}
	p.Teams[0].Type = game.TeamPlayer
	p.Teams[1].Type = game.TeamComputer
	g := sim.NewWithGrid(game.NewGameState(p), mgr, grid, w, h)

	// Two attackers - below the default muster threshold of three.
	a := armedTank(1, 1, 2, 2, mgr)
	b := armedTank(1, 2, 2, 3, mgr)
	seeAll(a, w)
	seeAll(b, w)
	_ = armedTank(0, 9, 8, 2, mgr) // player, spotted, out of weapon range

	g.AI = New(testUnitFactory)
	g.BeginTeamTurn(1)

	if g.HasOrder(a) || g.HasOrder(b) {
		t.Fatal("AI advanced with a sub-threshold force (no build-up)")
	}

	// Reinforce to the muster threshold: now the team commits and advances.
	c := armedTank(1, 3, 2, 4, mgr)
	seeAll(c, w)
	g.BeginTeamTurn(1)

	if !g.HasOrder(a) && !g.HasOrder(b) && !g.HasOrder(c) {
		t.Error("AI did not commit its strike force once mustered")
	}
}

// scoutUnit builds a mobile land scout with scan range and a movement budget for
// the explore tests: scan reveals fog, speed gives it a per turn move budget.
func scoutUnit(team, id, gx, gy, scan int, mgr *units.Manager) *game.UnitInfo {
	u := game.NewUnitInfo(assets.Scout, team, id, 0)
	u.Flags = game.FlagMobileLandUnit | game.FlagSelectable
	v := &game.UnitValues{}
	v.Set(game.AttribScan, scan)
	v.Set(game.AttribSpeed, 12)
	u.Values = v
	u.InitState()
	mgr.Deploy(u, gx, gy)
	return u
}

// TestAIExploresUnexploredMap checks the fog aware explore task: a computer scout
// with no enemy to fight and nothing to build issues a move toward unexplored
// territory rather than standing still.
func TestAIExploresUnexploredMap(t *testing.T) {
	const w, h = 24, 24
	mgr := units.NewManager()
	grid := &access.Grid{W: w, H: h, Surf: allLand{}, Units: mgr.Cells}
	p := &game.StartParams{}
	p.Teams[0].Type = game.TeamPlayer
	p.Teams[1].Type = game.TeamComputer
	g := sim.NewWithGrid(game.NewGameState(p), mgr, grid, w, h)

	scout := scoutUnit(1, 1, 12, 12, 2, mgr) // computer scout, mid map, no enemy anywhere

	g.AI = New(testUnitFactory)
	g.BeginTeamTurn(1)

	if !g.HasOrder(scout) {
		t.Error("scout did not move to explore an unexplored map")
	}
}

// TestAINoExploreWhenFullyExplored checks the explore task holds when there is
// nothing left to discover: on a small map the scout's own scan covers every
// cell, so no unexplored frontier remains and it issues no move.
func TestAINoExploreWhenFullyExplored(t *testing.T) {
	const w, h = 5, 5
	mgr := units.NewManager()
	grid := &access.Grid{W: w, H: h, Surf: allLand{}, Units: mgr.Cells}
	p := &game.StartParams{}
	p.Teams[0].Type = game.TeamPlayer
	p.Teams[1].Type = game.TeamComputer
	g := sim.NewWithGrid(game.NewGameState(p), mgr, grid, w, h)

	// Centre scout, scan 3: a tile radius 3 disc from (2,2) covers the whole 5x5
	// map (the far corner is sqrt(8) < 3 tiles away).
	scout := scoutUnit(1, 1, 2, 2, 3, mgr)
	g.RefreshVisibility() // stamp the scout's coverage into the team's fog memory

	if !g.Vis.Team(1).Explored(0, 0) || !g.Vis.Team(1).Explored(4, 4) {
		t.Fatal("test setup: scout scan did not cover the whole map")
	}

	g.AI = New(testUnitFactory)
	g.BeginTeamTurn(1)

	if g.HasOrder(scout) {
		t.Error("scout issued an explore move on a fully-explored map")
	}
}

// probeTask is a minimal concrete Task for exercising the manager plumbing: it
// counts how many times its lifecycle hooks fire and reschedules itself a fixed
// number of times so the drain loop has work to do.
type probeTask struct {
	TaskBase
	beginCount  int
	endCount    int
	rescheduled int
	maxRuns     int
}

func newProbeTask(team, maxRuns int) *probeTask {
	return &probeTask{TaskBase: newTaskBase(team, nil, 100), maxRuns: maxRuns}
}

func (p *probeTask) BeginTurn() {
	p.beginCount++
	if p.rescheduled < p.maxRuns {
		p.rescheduled++
		p.RemindTurnStart(false) // queue another pass
	}
}

func (p *probeTask) EndTurn() { p.endCount++ }

func TestAppendTaskSchedulesInitReminder(t *testing.T) {
	m := NewTaskManager()
	task := newProbeTask(0, 0)
	m.AppendTask(task)

	if m.TaskCount() != 1 {
		t.Fatalf("TaskCount = %d, want 1", m.TaskCount())
	}
	// Init schedules a priority turn start reminder.
	if m.RemindersCount() != 1 {
		t.Fatalf("RemindersCount after AppendTask = %d, want 1", m.RemindersCount())
	}
	if !task.isScheduledForTurnStart() {
		t.Error("task not marked scheduled for turn start")
	}
}

func TestExecuteRemindersDrains(t *testing.T) {
	m := NewTaskManager()
	task := newProbeTask(0, 0) // Init reminder only; no reschedule
	m.AppendTask(task)

	if !m.ExecuteReminders() {
		t.Fatal("ExecuteReminders reported no work, want true")
	}
	if task.beginCount != 1 {
		t.Errorf("beginCount = %d, want 1", task.beginCount)
	}
	if m.RemindersCount() != 0 {
		t.Errorf("RemindersCount after drain = %d, want 0", m.RemindersCount())
	}
	if m.ExecuteReminders() {
		t.Error("ExecuteReminders on empty queue reported work, want false")
	}
}

func TestReminderReschedulingTerminates(t *testing.T) {
	m := NewTaskManager()
	task := newProbeTask(0, 3) // reschedules itself 3 times
	m.AppendTask(task)

	m.ExecuteReminders()
	// 1 initial run + 3 reschedules = 4 BeginTurn calls, then it stops.
	if task.beginCount != 4 {
		t.Errorf("beginCount = %d, want 4", task.beginCount)
	}
	if m.RemindersCount() != 0 {
		t.Errorf("queue not empty after drain: %d", m.RemindersCount())
	}
}

func TestTaskManagerBeginEndTurnTeamScoped(t *testing.T) {
	m := NewTaskManager()
	t0 := newProbeTask(0, 0)
	t1 := newProbeTask(1, 0)
	m.AppendTask(t0)
	m.AppendTask(t1)
	m.ExecuteReminders() // clear the Init reminders

	m.BeginTurn(0)
	m.ExecuteReminders()
	if t0.beginCount != 2 { // 1 from Init drain, 1 from BeginTurn(0)
		t.Errorf("team-0 beginCount = %d, want 2", t0.beginCount)
	}
	if t1.beginCount != 1 { // only the Init drain
		t.Errorf("team-1 beginCount = %d, want 1 (BeginTurn(0) must not touch team 1)", t1.beginCount)
	}

	m.EndTurn(0)
	m.ExecuteReminders()
	if t0.endCount != 1 {
		t.Errorf("team-0 endCount = %d, want 1", t0.endCount)
	}
	if t1.endCount != 0 {
		t.Errorf("team-1 endCount = %d, want 0", t1.endCount)
	}
}

func TestRemoveSelfUnlinks(t *testing.T) {
	m := NewTaskManager()
	task := newProbeTask(0, 0)
	m.AppendTask(task)
	task.RemoveSelf()
	if m.TaskCount() != 0 {
		t.Errorf("TaskCount after RemoveSelf = %d, want 0", m.TaskCount())
	}
}

func TestStrategyForClan(t *testing.T) {
	cases := map[int]Strategy{
		clanTheChosen:    StrategyAir,
		clanCrimsonPath:  StrategySea,
		clanAyersHand:    StrategyMissiles,
		clanSacredEights: StrategyFastAttack,
		clanSevenKnights: StrategyDefensive,
		clanVonGriffin:   StrategyCombinedArms, // unmapped -> default
	}
	for clan, want := range cases {
		if got := strategyForClan(clan); got != want {
			t.Errorf("strategyForClan(%d) = %v, want %v", clan, got, want)
		}
	}
}

// TestAIHookPassesComputerTurn checks the full seam: a game wired with the AI
// runs a computer team's BeginTeamTurn without error, picks a terrain scored
// strategy, and the team is able to finish. The roll is forced to the first
// positive strategy so the outcome is deterministic. On an all land 8x8 map at the
// default Average tier the positive scores are Defensive/Missiles/Air/Sea/Tank/
// Combined; the first (lowest index) is Defensive.
func TestAIHookPassesComputerTurn(t *testing.T) {
	defer restoreStrategyRoll(strategyRoll)()
	strategyRoll = func(int) int { return 0 } // pick the first positively scored strategy

	const w, h = 8, 8
	mgr := units.NewManager()
	grid := &access.Grid{W: w, H: h, Surf: allLand{}, Units: mgr.Cells}

	p := &game.StartParams{}
	p.Teams[0].Type = game.TeamPlayer
	p.Teams[1].Type = game.TeamComputer
	p.Teams[1].Clan = clanTheChosen
	g := sim.NewWithGrid(game.NewGameState(p), mgr, grid, w, h)

	a := New(testUnitFactory)
	g.AI = a
	g.BeginTeamTurn(1)

	if got := a.players[1].GetStrategy(); got != StrategyDefensive {
		t.Errorf("computer team strategy = %v, want StrategyDefensive (first positive on all-land)", got)
	}
	if a.players[1].needInit {
		t.Error("player still needs init after BeginTurn")
	}
}

// allWater is a synthetic all water SurfaceSource: no viable landmass, so the AI
// finds no continent and falls back to the clan default.
type allWater struct{}

func (allWater) SurfaceAt(x, y int) uint8 { return world.SurfaceWater }

// restoreStrategyRoll returns the saved roll seam (used with defer).
func restoreStrategyRoll(saved func(int) int) func() { return func() { strategyRoll = saved } }

// TestSelectStrategyEmptyMapFallsBackToClan checks the degenerate map safety net: a
// map with no viable landmass leaves the strategy Random, which BeginTurn resolves
// to the team's clan default (The Chosen -> Air).
func TestSelectStrategyEmptyMapFallsBackToClan(t *testing.T) {
	const w, h = 8, 8
	mgr := units.NewManager()
	grid := &access.Grid{W: w, H: h, Surf: allWater{}, Units: mgr.Cells}

	p := &game.StartParams{}
	p.Teams[0].Type = game.TeamPlayer
	p.Teams[1].Type = game.TeamComputer
	p.Teams[1].Clan = clanTheChosen
	g := sim.NewWithGrid(game.NewGameState(p), mgr, grid, w, h)

	a := New(testUnitFactory)
	g.AI = a
	g.BeginTeamTurn(1)

	if got := a.players[1].GetStrategy(); got != StrategyAir {
		t.Errorf("water-map strategy = %v, want StrategyAir (clan fallback)", got)
	}
}

// patternSurf is a SurfaceSource built from a row pattern: '#' = land, '~' = water,
// ' ' (or any other rune) = blocked. Out of bounds reads return SurfaceNone.
type patternSurf struct {
	w, h  int
	cells []uint8
}

func newPatternSurf(rows []string) patternSurf {
	h := len(rows)
	w := 0
	for _, r := range rows {
		if len(r) > w {
			w = len(r)
		}
	}
	cells := make([]uint8, w*h)
	for y, r := range rows {
		for x := 0; x < w; x++ {
			s := world.SurfaceNone
			if x < len(r) {
				switch r[x] {
				case '#':
					s = world.SurfaceLand
				case '~':
					s = world.SurfaceWater
				}
			}
			cells[y*w+x] = s
		}
	}
	return patternSurf{w: w, h: h, cells: cells}
}

func (p patternSurf) SurfaceAt(x, y int) uint8 {
	if x < 0 || y < 0 || x >= p.w || y >= p.h {
		return world.SurfaceNone
	}
	return p.cells[y*p.w+x]
}

// gameFromSurf wraps a SurfaceSource in a minimal sim.Game for the terrain helpers.
func gameFromSurf(s access.SurfaceSource, w, h int) *sim.Game {
	mgr := units.NewManager()
	grid := &access.Grid{W: w, H: h, Surf: s, Units: mgr.Cells}
	p := &game.StartParams{}
	return sim.NewWithGrid(game.NewGameState(p), mgr, grid, w, h)
}

// TestFindContinentsCountsLandmasses checks the flood fill splits two land blocks
// across a water column into two continents, and drops a sub-4x4 sliver.
func TestFindContinentsCountsLandmasses(t *testing.T) {
	// Two 4x4 land blocks separated by a 2-wide water gap, plus a 1x1 sliver island.
	rows := []string{
		"####~~####~",
		"####~~####~",
		"####~~####~",
		"####~~####~",
		"~~~~~~~~~~~",
		"~~~~~~~~~~#",
	}
	g := gameFromSurf(newPatternSurf(rows), 11, 6)
	cs := findContinents(g)
	if len(cs) != 2 {
		t.Fatalf("findContinents = %d continents, want 2 (the sliver is not viable)", len(cs))
	}
	for _, c := range cs {
		if c.size != 16 {
			t.Errorf("continent size = %d, want 16 (a 4x4 block)", c.size)
		}
	}
}

// TestContinentCloseProximityAndIsolated checks the shape verdicts: a small island
// ringed by water is close proximity AND isolated; a small mass joined to a big one
// by a short land bridge is neither isolated (the BFS reaches the big mass).
func TestContinentCloseProximityAndIsolated(t *testing.T) {
	// A lone 4x4 island in a sea (well inside proximity_range, water all around).
	island := []string{
		"~~~~~~~~",
		"~~~~~~~~",
		"~~####~~",
		"~~####~~",
		"~~####~~",
		"~~####~~",
		"~~~~~~~~",
		"~~~~~~~~",
	}
	cs := findContinents(gameFromSurf(newPatternSurf(island), 8, 8))
	if len(cs) != 1 {
		t.Fatalf("island: %d continents, want 1", len(cs))
	}
	if !cs[0].closeProx || !cs[0].isolated {
		t.Errorf("lone island closeProx=%v isolated=%v, want both true", cs[0].closeProx, cs[0].isolated)
	}

	// Two 4x4 masses separated by a single water column: the isolation BFS crosses the
	// short gap, so neither is isolated (though both are close proximity).
	near := []string{
		"~~~~~~~~~~",
		"~####~####",
		"~####~####",
		"~####~####",
		"~####~####",
		"~~~~~~~~~~",
	}
	cs = findContinents(gameFromSurf(newPatternSurf(near), 10, 6))
	if len(cs) != 2 {
		t.Fatalf("near masses: %d continents, want 2", len(cs))
	}
	for _, c := range cs {
		if !c.closeProx {
			t.Errorf("near continent (filler %d) closeProx=false, want true", c.filler)
		}
		if c.isolated {
			t.Errorf("near continent (filler %d) isolated, want connected via the 1-cell gap", c.filler)
		}
	}
}

// TestScoreStrategiesSingleContinent checks the table for one viable big mass that
// everyone shares. The single mass path weighs the tank horde 6/12/24 by tier, which
// also suppresses the scout/defensive reassignment; Expert+ scores espionage with the
// continent count (it no longer overwrites the tank weight).
// Air/Sea stay 0 (the mass is neither cramped nor isolated).
func TestScoreStrategiesSingleContinent(t *testing.T) {
	one := []continent{{filler: 3, size: 200, ulx: 0, uly: 0, lrx: 30, lry: 30}}
	scores := scoreStrategies(one, OpponentExpert)
	if scores[StrategyScoutHorde] != 0 {
		t.Errorf("single-mass ScoutHorde = %d, want 0 (suppressed by the single-mass tank flag)", scores[StrategyScoutHorde])
	}
	if scores[StrategyTankHorde] != 6 {
		t.Errorf("single-mass Expert tank horde = %d, want 6 (the single mass weight survives)", scores[StrategyTankHorde])
	}
	if scores[StrategyEspionage] != 1 {
		t.Errorf("single-mass Expert espionage = %d, want 1 (continent count)", scores[StrategyEspionage])
	}
	if scores[StrategyCombinedArms] != 2 { // (count 1 - close 0) * 2
		t.Errorf("single-mass CombinedArms = %d, want 2", scores[StrategyCombinedArms])
	}
	if got := scoreStrategies(one, OpponentAverage)[StrategyEspionage]; got != 0 {
		t.Errorf("Average espionage = %d, want 0 (Expert and up only)", got)
	}
	if scores[StrategyAir] != 0 || scores[StrategySea] != 0 {
		t.Errorf("big single mass Air=%d Sea=%d, want both 0 (not close/isolated)", scores[StrategyAir], scores[StrategySea])
	}
}

// TestScoreStrategiesArchipelago checks the table for several small isolated islands:
// the water strategies (sea / scout horde) score positive, fast attack stays modest.
func TestScoreStrategiesArchipelago(t *testing.T) {
	islands := []continent{
		{filler: 3, size: 20, closeProx: true, isolated: true},
		{filler: 4, size: 20, closeProx: true, isolated: true},
		{filler: 5, size: 20, closeProx: true, isolated: true},
	}
	scores := scoreStrategies(islands, OpponentMaster)
	if scores[StrategySea] != 6 { // isolatedCount(3) * 2
		t.Errorf("archipelago Sea = %d, want 6", scores[StrategySea])
	}
	if scores[StrategyScoutHorde] <= 0 {
		t.Errorf("archipelago ScoutHorde = %d, want > 0", scores[StrategyScoutHorde])
	}
	if scores[StrategyFastAttack] != 0 { // count(3) - close(3)
		t.Errorf("all-close archipelago FastAttack = %d, want 0", scores[StrategyFastAttack])
	}
	if scores[StrategyCombinedArms] != 0 { // (count 3 - close 3) * 2, not count + close = 12
		t.Errorf("all-close archipelago CombinedArms = %d, want 0", scores[StrategyCombinedArms])
	}
}

// TestContinentCloseProximityUsesDoubleRange checks the doubled range: a landmass counts as close
// proximity while both bounding box sides fit in twice proximity_range (28), not once (14).
func TestContinentCloseProximityUsesDoubleRange(t *testing.T) {
	if !continentCloseProximity(continent{ulx: 0, uly: 0, lrx: 2 * proximityRange, lry: 20}) {
		t.Error("a 28x20 landmass is not close proximity, want true (twice the range)")
	}
	if continentCloseProximity(continent{ulx: 0, uly: 0, lrx: 2*proximityRange + 1, lry: 20}) {
		t.Error("a 29x20 landmass is close proximity, want false")
	}
}

// TestPickStrategyWeightedAndDiversifies checks the weighted roll, sibling
// diversification, and the empty table fallback to Random.
func TestPickStrategyWeightedAndDiversifies(t *testing.T) {
	defer restoreStrategyRoll(strategyRoll)()

	var scores [strategyCount]int
	scores[StrategyAir] = 3
	scores[StrategySea] = 5

	// Force the roll to land in the Air band (cumulative: Air covers rolls 1..3).
	strategyRoll = func(int) int { return 0 } // roll = 1 -> first positive = Air
	if got := pickStrategy(scores, nil); got != StrategyAir {
		t.Errorf("pickStrategy = %v, want StrategyAir", got)
	}

	// Diversify: with Air already taken by a sibling, only Sea remains.
	if got := pickStrategy(scores, []Strategy{StrategyAir}); got != StrategySea {
		t.Errorf("pickStrategy(decided=Air) = %v, want StrategySea", got)
	}

	// Every positive option taken -> fall back to the undiversified table (not Random).
	if got := pickStrategy(scores, []Strategy{StrategyAir, StrategySea}); got == StrategyRandom {
		t.Error("pickStrategy with all taken returned Random, want a fallback choice")
	}

	// Empty table -> Random.
	var empty [strategyCount]int
	if got := pickStrategy(empty, nil); got != StrategyRandom {
		t.Errorf("pickStrategy(empty) = %v, want StrategyRandom", got)
	}
}

// armedTurret builds a stationary armed turret unit. The type and flags let the
// test supply GunTurrt (ground) or AntiAir (air only). Stationary buildings are
// not mobile, so FlagStationary is set and no mobile flag is present.
func armedTurret(team, id int, ut assets.UnitType, flags game.Flags, gx, gy int, mgr *units.Manager) *game.UnitInfo {
	u := game.NewUnitInfo(ut, team, id, 0)
	u.Flags = flags
	v := &game.UnitValues{}
	v.Set(game.AttribAttack, 10)
	v.Set(game.AttribRange, 5) // covers any cell in a 5-cell Chebyshev radius
	v.Set(game.AttribScan, 6)  // sees its own firing range, so an in range enemy is spotted
	v.Set(game.AttribRounds, 1)
	v.Set(game.AttribAmmo, 6)
	v.Set(game.AttribSpeed, 0) // stationary: no movement budget
	v.Set(game.AttribHits, 20)
	u.Values = v
	u.InitState()
	mgr.Deploy(u, gx, gy)
	return u
}

// TestAITurretFiresOnInRangeEnemy checks that a stationary GunTurrt fires on a
// ground enemy already within its range each turn, and eventually destroys it.
func TestAITurretFiresOnInRangeEnemy(t *testing.T) {
	const w, h = 16, 16
	mgr := units.NewManager()
	grid := &access.Grid{W: w, H: h, Surf: allLand{}, Units: mgr.Cells}
	p := &game.StartParams{}
	p.Teams[0].Type = game.TeamPlayer
	p.Teams[1].Type = game.TeamComputer
	g := sim.NewWithGrid(game.NewGameState(p), mgr, grid, w, h)

	// GunTurrt flags: stationary + building + selectable. No mobile flags.
	turretFlags := game.FlagStationary | game.FlagBuilding | game.FlagSelectable
	turret := armedTurret(1, 1, assets.GunTurrt, turretFlags, 8, 8, mgr)

	// Enemy ground tank 3 cells away (within range 5).
	enemy := armedTank(0, 2, 11, 8, mgr)
	startHits := enemy.Hits

	g.AI = New(testUnitFactory)
	g.BeginTeamTurn(1)

	if enemy.Hits >= startHits {
		t.Errorf("AI GunTurrt did not fire on in-range enemy: hits %d (start %d)", enemy.Hits, startHits)
	}
	// Turret must NOT have moved (it is stationary).
	if turret.GridX != 8 || turret.GridY != 8 {
		t.Errorf("turret moved to (%d,%d), expected (8,8)", turret.GridX, turret.GridY)
	}
}

// TestAITurretDestroysEnemyOverNTurns verifies that a computer GunTurrt with an
// enemy always in range eventually destroys it.
func TestAITurretDestroysEnemyOverNTurns(t *testing.T) {
	const w, h = 16, 16
	mgr := units.NewManager()
	grid := &access.Grid{W: w, H: h, Surf: allLand{}, Units: mgr.Cells}
	p := &game.StartParams{}
	p.Teams[0].Type = game.TeamPlayer
	p.Teams[1].Type = game.TeamComputer
	g := sim.NewWithGrid(game.NewGameState(p), mgr, grid, w, h)

	turretFlags := game.FlagStationary | game.FlagBuilding | game.FlagSelectable
	_ = armedTurret(1, 1, assets.GunTurrt, turretFlags, 8, 8, mgr)
	enemy := armedTank(0, 2, 10, 8, mgr) // 2 cells away, well within range 5

	g.AI = New(testUnitFactory)

	// The turret has attack 10 vs enemy hits 24; needs at most 3 turns to kill it.
	// Run 10 turns to give ample margin.
	for turn := 0; turn < 10; turn++ {
		if enemy.Hits <= 0 {
			break
		}
		g.BeginTeamTurn(1)
	}
	if enemy.Hits > 0 {
		t.Errorf("GunTurrt failed to destroy enemy over 10 turns (hits remaining: %d)", enemy.Hits)
	}
}

// TestAIAntiAirIgnoresGroundFiresOnAir checks that an AntiAir turret ignores a
// ground tank but fires on an aircraft in range.
func TestAIAntiAirIgnoresGroundFiresOnAir(t *testing.T) {
	const w, h = 24, 24
	mgr := units.NewManager()
	grid := &access.Grid{W: w, H: h, Surf: allLand{}, Units: mgr.Cells}
	p := &game.StartParams{}
	p.Teams[0].Type = game.TeamPlayer
	p.Teams[1].Type = game.TeamComputer
	g := sim.NewWithGrid(game.NewGameState(p), mgr, grid, w, h)

	// AntiAir flags: stationary + building + selectable. The CanTargetUnit /
	// CanAttack seam (combat.CanTarget) gates anti air on air only targets.
	antiAirFlags := game.FlagStationary | game.FlagBuilding | game.FlagSelectable
	antiAir := armedTurret(1, 1, assets.AntiAir, antiAirFlags, 8, 8, mgr)

	// A ground tank: AntiAir must NOT fire at it.
	groundEnemy := armedTank(0, 2, 10, 8, mgr)
	groundStartHits := groundEnemy.Hits

	g.AI = New(testUnitFactory)
	g.BeginTeamTurn(1)

	if groundEnemy.Hits < groundStartHits {
		t.Error("AntiAir fired on a ground unit, should only target aircraft")
	}
	if antiAir.GridX != 8 || antiAir.GridY != 8 {
		t.Errorf("AntiAir moved to (%d,%d), expected (8,8)", antiAir.GridX, antiAir.GridY)
	}

	// Add an air unit within range: AntiAir should now fire on it.
	airEnemy := armedUnit(0, 3, assets.Awac, game.FlagMobileAirUnit|game.FlagHovering|game.FlagSelectable, 11, 8, mgr)
	airStartHits := airEnemy.Hits
	g.BeginTeamTurn(1)

	if airEnemy.Hits >= airStartHits {
		t.Errorf("AntiAir did not fire on air enemy: hits %d (start %d)", airEnemy.Hits, airStartHits)
	}
	if groundEnemy.Hits < groundStartHits {
		t.Error("AntiAir fired on ground unit after air target was added")
	}
}

// TestAITurretHoldsWhenNoTargetInRange confirms that a turret with no in range
// enemy stays idle (does not move, does not fire).
func TestAITurretHoldsWhenNoTargetInRange(t *testing.T) {
	const w, h = 24, 24
	mgr := units.NewManager()
	grid := &access.Grid{W: w, H: h, Surf: allLand{}, Units: mgr.Cells}
	p := &game.StartParams{}
	p.Teams[0].Type = game.TeamPlayer
	p.Teams[1].Type = game.TeamComputer
	g := sim.NewWithGrid(game.NewGameState(p), mgr, grid, w, h)

	turretFlags := game.FlagStationary | game.FlagBuilding | game.FlagSelectable
	turret := armedTurret(1, 1, assets.GunTurrt, turretFlags, 8, 8, mgr)
	startShots := turret.Shots

	// Place enemy 15 cells away, well outside range 5.
	_ = armedTank(0, 2, 23, 8, mgr)

	g.AI = New(testUnitFactory)
	g.BeginTeamTurn(1)

	// Shots should be unchanged (did not fire).
	if turret.Shots != startShots {
		t.Errorf("turret fired when no enemy in range: shots %d -> %d", startShots, turret.Shots)
	}
	// Turret must still be at its original position.
	if turret.GridX != 8 || turret.GridY != 8 {
		t.Errorf("turret moved to (%d,%d), expected (8,8)", turret.GridX, turret.GridY)
	}
}

// TestAITurretHoldsAgainstFoggedEnemy checks the turret spotting gate: an enemy
// inside the turret's weapon range but outside its scan coverage is not fired on
// - a turret never shoots blindly into the fog.
func TestAITurretHoldsAgainstFoggedEnemy(t *testing.T) {
	const w, h = 24, 24
	mgr := units.NewManager()
	grid := &access.Grid{W: w, H: h, Surf: allLand{}, Units: mgr.Cells}
	p := &game.StartParams{}
	p.Teams[0].Type = game.TeamPlayer
	p.Teams[1].Type = game.TeamComputer
	g := sim.NewWithGrid(game.NewGameState(p), mgr, grid, w, h)

	turretFlags := game.FlagStationary | game.FlagBuilding | game.FlagSelectable
	turret := armedTurret(1, 1, assets.GunTurrt, turretFlags, 8, 8, mgr)
	turret.Values.Set(game.AttribScan, 2) // sees only 2 cells, short of its range 5
	startShots := turret.Shots

	// Enemy 4 cells away: inside weapon range 5, but outside the scan-2 coverage.
	_ = armedTank(0, 2, 12, 8, mgr)

	g.AI = New(testUnitFactory)
	g.BeginTeamTurn(1)

	if turret.Shots != startShots {
		t.Errorf("turret fired at an unspotted (fogged) in-range enemy: shots %d -> %d", startShots, turret.Shots)
	}
}

// buildingUnit mints a finished stationary structure ready for g.PlaceBuilding (so
// it joins the team's complex network), 2x2 when large.
func buildingUnit(team int, ut assets.UnitType, large bool) *game.UnitInfo {
	testFactorySeq++
	u := game.NewUnitInfo(ut, team, testFactorySeq, 0)
	u.Flags = buildingFlags(large)
	u.Values = &game.UnitValues{}
	u.InitState()
	return u
}

// deployBuilding places a stationary building with an EXPLICIT id and deploys it. Tests use
// it instead of buildingUnit when they also place units with explicit ids, so the building's
// id cannot collide with the shared testFactorySeq. A collision would overwrite a unit in the
// id hash (byID is keyed by id) and silently drop one, a flaky failure that depends on run order.
func deployBuilding(g *sim.Game, team, id int, ut assets.UnitType, gx, gy int) *game.UnitInfo {
	u := game.NewUnitInfo(ut, team, id, 0)
	u.Flags = buildingFlags(true)
	u.Values = &game.UnitValues{}
	u.InitState()
	g.Units.Deploy(u, gx, gy)
	return u
}

// complexCount reports how many separate complexes a team's network holds.
func complexCount(g *sim.Game, team int) int {
	if g.Networks[team] == nil {
		return 0
	}
	return len(g.Networks[team].Complexes)
}

// connectorPending returns the connector site the manager has seeded on a builder,
// if any.
func connectorPending(bm *taskManageBuildings) (pendingBuildSite, bool) {
	for _, ps := range bm.pending {
		if ps.unitType == assets.Cnct4W {
			return ps, true
		}
	}
	return pendingBuildSite{}, false
}

// TestAINoConnectorWhenSingleComplex checks the connection assistant is inert for a
// healthy, already connected base: it seeds no connector and issues no connector
// build, so it cannot disturb the existing connectivity preference behaviour.
func TestAINoConnectorWhenSingleComplex(t *testing.T) {
	const w, h = 20, 20
	mgr := units.NewManager()
	grid := &access.Grid{W: w, H: h, Surf: allLand{}, Units: mgr.Cells}
	p := &game.StartParams{}
	p.Teams[1].Type = game.TeamComputer
	g := sim.NewWithGrid(game.NewGameState(p), mgr, grid, w, h)

	// One connected base: a 2x2 factory plus an edge adjacent mining station.
	g.PlaceBuilding(buildingUnit(1, assets.LandPlt, true), 5, 5)  // (5,5)-(6,6)
	g.PlaceBuilding(buildingUnit(1, assets.MiningSt, true), 7, 5) // (7,5)-(8,6), abuts -> same complex
	engineerUnit(1, 1, 12, 12, mgr)

	if got := complexCount(g, 1); got != 1 {
		t.Fatalf("setup: want 1 complex, got %d", got)
	}

	bm := newBuildManager(g, 1)
	bm.reconnectBuildings()
	if _, ok := connectorPending(bm); ok {
		t.Fatal("assistant seeded a connector for an already-connected base")
	}
	bm.BeginTurn()
	if findOrder(g, assets.Cnct4W) != nil {
		t.Fatal("assistant built a connector for an already-connected base")
	}
}

// TestAIConnectsDetachedComplex checks the assistant sites the first connector
// correctly: bordering the PRIMARY base (so it draws the base pool) and stepping
// toward the detached mining station.
func TestAIConnectsDetachedComplex(t *testing.T) {
	const w, h = 24, 24
	mgr := units.NewManager()
	grid := &access.Grid{W: w, H: h, Surf: allLand{}, Units: mgr.Cells}
	p := &game.StartParams{}
	p.Teams[1].Type = game.TeamComputer
	g := sim.NewWithGrid(game.NewGameState(p), mgr, grid, w, h)

	// Primary base (two abutting buildings -> unambiguously the larger complex).
	prim := buildingUnit(1, assets.LandPlt, true)
	g.PlaceBuilding(prim, 4, 4)                                   // (4,4)-(5,5)
	g.PlaceBuilding(buildingUnit(1, assets.PowerStn, true), 4, 6) // (4,6)-(5,7), abuts -> same complex
	// A detached mining station several cells east, a gap of open land between.
	mine := buildingUnit(1, assets.MiningSt, true)
	g.PlaceBuilding(mine, 12, 4) // (12,4)-(13,5)
	eng := engineerUnit(1, 1, 8, 9, mgr)
	eng.Values.Set(game.AttribSpeed, 40)
	eng.ResetMovement()

	if got := complexCount(g, 1); got != 2 {
		t.Fatalf("setup: want 2 complexes, got %d", got)
	}

	primCells := complexFootprintCells(g.ComplexOf(prim))
	mineCells := complexFootprintCells(g.ComplexOf(mine))
	gap := minPairManhattan(mineCells, primCells)

	bm := newBuildManager(g, 1)
	bm.reconnectBuildings()

	ps, ok := connectorPending(bm)
	if !ok {
		t.Fatal("no connector seeded to join the detached mining station")
	}
	// It must step toward the detached mine...
	if d := minManhattanTo(ps.x, ps.y, mineCells); d >= gap {
		t.Errorf("connector at (%d,%d) did not step toward the detached mine: dist %d >= gap %d", ps.x, ps.y, d, gap)
	}
	// ...and border the PRIMARY base so it draws the base pool and merges into it on completion.
	if minManhattanTo(ps.x, ps.y, primCells) != 1 {
		t.Errorf("connector at (%d,%d) does not border the primary base", ps.x, ps.y)
	}
}

// TestAIConnectorRunMergesComplexes is the end to end check: over several turns the
// engineer lays a run of connectors that joins the detached mine into one complex
// with the base.
func TestAIConnectorRunMergesComplexes(t *testing.T) {
	const w, h = 24, 24
	mgr := units.NewManager()
	grid := &access.Grid{W: w, H: h, Surf: allLand{}, Units: mgr.Cells}
	p := &game.StartParams{}
	p.Teams[1].Type = game.TeamComputer
	g := sim.NewWithGrid(game.NewGameState(p), mgr, grid, w, h)

	prim := buildingUnit(1, assets.LandPlt, true)
	g.PlaceBuilding(prim, 4, 4)
	g.PlaceBuilding(buildingUnit(1, assets.PowerStn, true), 4, 6)
	g.PlaceBuilding(buildingUnit(1, assets.MiningSt, true), 11, 4)
	eng := engineerUnit(1, 1, 7, 8, mgr)
	eng.Values.Set(game.AttribSpeed, 40)
	eng.ResetMovement()
	eng.Storage = 2000 // own cargo to spend on the connector run (no economy in this headless test)

	if got := complexCount(g, 1); got != 2 {
		t.Fatalf("setup: want 2 complexes, got %d", got)
	}

	bm := newBuildManager(g, 1)
	merged := false
	for i := 0; i < 400 && !merged; i++ {
		bm.reconnectBuildings()
		bm.BeginTurn()
		g.ResolveMovement()
		g.AdvanceBuilds(1)
		for _, u := range g.Units.Units() {
			if u.Team == 1 {
				u.ResetMovement()
			}
		}
		merged = complexCount(g, 1) == 1
	}
	if !merged {
		t.Fatalf("the detached mine was never connected: still %d complexes", complexCount(g, 1))
	}
	connectors, smallSlabs := 0, 0
	for _, u := range g.Units.Units() {
		if u.Team != 1 {
			continue
		}
		switch u.Type {
		case assets.Cnct4W:
			connectors++
		case assets.SmlSlab:
			smallSlabs++ // a connector wrongly carrying FlagRequiresSlab would lay one of these
		}
	}
	if smallSlabs != 0 {
		t.Errorf("connectors laid %d small slabs; a connector is a terrain special and lays none", smallSlabs)
	}
	if connectors == 0 {
		t.Error("complexes merged but no connectors were built")
	}
}

// TestAIConnectorFundedFromPrimaryPool verifies that an empty engineer completes a connector run by drawing
// pooled material from the adjacent stocked complex.
func TestAIConnectorFundedFromPrimaryPool(t *testing.T) {
	const w, h = 24, 24
	mgr := units.NewManager()
	grid := &access.Grid{W: w, H: h, Surf: allLand{}, Units: mgr.Cells}
	p := &game.StartParams{}
	p.Teams[1].Type = game.TeamComputer
	g := sim.NewWithGrid(game.NewGameState(p), mgr, grid, w, h)

	prim := buildingUnit(1, assets.LandPlt, true)
	g.PlaceBuilding(prim, 4, 4)
	g.PlaceBuilding(buildingUnit(1, assets.PowerStn, true), 4, 6)
	g.PlaceBuilding(buildingUnit(1, assets.MiningSt, true), 11, 4)

	// Stock ONLY the primary base pool; the engineer carries nothing.
	g.ComplexOf(prim).Material = 200
	startPool := g.ComplexOf(prim).Material

	eng := engineerUnit(1, 1, 7, 8, mgr)
	eng.Values.Set(game.AttribSpeed, 40)
	eng.ResetMovement()
	eng.Storage = 0 // empty: a factory fresh engineer with no cargo

	if got := complexCount(g, 1); got != 2 {
		t.Fatalf("setup: want 2 complexes, got %d", got)
	}

	bm := newBuildManager(g, 1)
	merged := false
	for i := 0; i < 400 && !merged; i++ {
		bm.reconnectBuildings()
		bm.BeginTurn()
		g.ResolveMovement()
		g.AdvanceBuilds(1)
		for _, u := range g.Units.Units() {
			if u.Team == 1 {
				u.ResetMovement()
			}
		}
		merged = complexCount(g, 1) == 1
	}
	if !merged {
		t.Fatalf("an empty-cargo engineer never merged the detached mine: still %d complexes", complexCount(g, 1))
	}
	// The connectors must have been paid for out of the primary pool (not the
	// engineer, which started and stayed at 0 cargo).
	if got := g.ComplexOf(prim).Material; got >= startPool {
		t.Errorf("primary pool did not fund the connectors: material %d, want < %d", got, startPool)
	}
}

// TestStockedComplexGate locks the material aware "connected" test that prevents the
// detached island stall: a site bordering the STOCKED base counts as suppliable, a
// site bordering an EMPTY island does not - even though both border a building
// (touchesComplex, the geometric test, sees both). The distinction is material.
func TestStockedComplexGate(t *testing.T) {
	const w, h = 24, 24
	mgr := units.NewManager()
	grid := &access.Grid{W: w, H: h, Surf: allLand{}, Units: mgr.Cells}
	p := &game.StartParams{}
	p.Teams[1].Type = game.TeamComputer
	g := sim.NewWithGrid(game.NewGameState(p), mgr, grid, w, h)

	base := buildingUnit(1, assets.LandPlt, true) // (4,4)-(5,5)
	g.PlaceBuilding(base, 4, 4)
	g.ComplexOf(base).Material = 100 // a stocked base

	island := buildingUnit(1, assets.ADump, false) // (12,12), a separate empty complex
	g.PlaceBuilding(island, 12, 12)
	g.ComplexOf(island).Material = 0

	if got := complexCount(g, 1); got != 2 {
		t.Fatalf("setup: want 2 complexes, got %d", got)
	}

	bm := newBuildManager(g, 1)
	cost := bm.buildRawCostPerTurn()

	// A 1x1 site bordering the stocked base is suppliable.
	if !bm.touchesStockedComplex(6, 5, 1, cost) {
		t.Error("a site bordering the stocked base was not recognised as stocked")
	}
	// A 1x1 site bordering the empty island is NOT - though it does border a building.
	if bm.touchesStockedComplex(13, 11, 1, cost) {
		t.Error("a site bordering an EMPTY island was wrongly treated as stocked")
	}
	if !bm.touchesComplex(13, 11, 1) {
		t.Fatal("setup: the island site should border a building (geometric test)")
	}
}

// TestRiskLevel locks the threat map per type risk weights (threatmap.cpp
// GetRiskLevel): the units a fleeing unit most wants to avoid score highest.
func TestRiskLevel(t *testing.T) {
	cases := []struct {
		ut    assets.UnitType
		flags game.Flags
		want  int
	}{
		{assets.ClnTrans, game.FlagMobileLandUnit, 7},
		{assets.Submarne, game.FlagMobileSeaUnit, 6},
		{assets.Commando, game.FlagMobileLandUnit, 5},
		{assets.Surveyor, game.FlagMobileLandUnit, 2},
		{assets.Bomber, game.FlagMobileAirUnit, 3}, // any air unit
		{assets.Tank, game.FlagMobileLandUnit, 1},  // default ground
	}
	for _, c := range cases {
		if got := riskLevel(c.ut, c.flags); got != c.want {
			t.Errorf("riskLevel(%v) = %d, want %d", c.ut, got, c.want)
		}
	}
}

// TestThreatMapStampsSpottedEnemy: the threat map sums a SPOTTED enemy's firepower
// over its weapon range disc, and is NON OMNISCIENT - a never seen enemy in the fog
// contributes nothing (the same g.SpottedByTeam gate the attack task uses).
func TestThreatMapStampsSpottedEnemy(t *testing.T) {
	const w, h = 24, 24
	mgr := units.NewManager()
	grid := &access.Grid{W: w, H: h, Surf: allLand{}, Units: mgr.Cells}
	p := &game.StartParams{}
	p.Teams[0].Type = game.TeamPlayer
	p.Teams[1].Type = game.TeamComputer
	g := sim.NewWithGrid(game.NewGameState(p), mgr, grid, w, h)

	eye := armedTank(1, 1, 12, 12, mgr) // computer, scan 6
	eye.Values.Set(game.AttribScan, 6)
	seen := armedTank(0, 2, 12, 8, mgr) // player, dist 4: SPOTTED (range 4, rounds 1)
	_ = armedTank(0, 3, 2, 2, mgr)      // player, far in the fog: NEVER seen
	g.RefreshVisibility()
	if !g.SpottedByTeam(seen, 1) {
		t.Fatal("setup: the near enemy should be spotted")
	}

	tm := buildThreatMap(g, 1)
	if tm.at(seen.GridX, seen.GridY) <= 0 {
		t.Error("threat map did not stamp the spotted enemy's own cell")
	}
	if tm.at(12, 11) <= 0 { // within the spotted enemy's range disc (dist 3 <= 4)
		t.Error("threat map did not stamp a cell inside the spotted enemy's range")
	}
	if tm.at(2, 2) != 0 {
		t.Error("threat map leaked an UNSPOTTED enemy's position (omniscience)")
	}
	if tm.at(0, 0) != 0 {
		t.Error("threat map stamped a cell out of every enemy's range")
	}
}

// TestAIRetreatsDamagedToDepot: a damaged computer tank near a friendly Depot heads
// for the depot and does NOT attack the in range enemy (retreat claims it).
func TestAIRetreatsDamagedToDepot(t *testing.T) {
	const w, h = 16, 16
	setMinStrikeForce(t, 1)
	mgr := units.NewManager()
	grid := &access.Grid{W: w, H: h, Surf: allLand{}, Units: mgr.Cells}
	p := &game.StartParams{}
	p.Teams[0].Type = game.TeamPlayer
	p.Teams[1].Type = game.TeamComputer
	g := sim.NewWithGrid(game.NewGameState(p), mgr, grid, w, h)

	_ = stationary(1, 1, assets.Depot, 4, 5, mgr) // friendly repair shop to the WEST
	tank := armedTank(1, 2, 8, 5, mgr)            // computer, damaged, enemy to the EAST
	tank.Hits = 5                                 // < HITS/2 (24/2): damaged
	seeAll(tank, w)
	enemy := armedTank(0, 3, 10, 5, mgr) // player, dist 2 (in range 4)
	startHits := enemy.Hits

	g.AI = New(testUnitFactory)
	g.BeginTeamTurn(1)

	if !g.HasOrder(tank) {
		t.Fatal("damaged tank issued no retreat order")
	}
	if enemy.Hits < startHits {
		t.Error("damaged tank attacked instead of retreating")
	}
	g.ResolveMovement()
	if tank.GridX >= 8 {
		t.Errorf("damaged tank at x=%d did not move WEST toward the depot (away from the enemy)", tank.GridX)
	}
}

// TestAIRetreatFleesWhenNoShop: a damaged unit with no reachable repair shop falls
// back to a strictly lower threat cell (away from the enemy's firing line).
func TestAIRetreatFleesWhenNoShop(t *testing.T) {
	const w, h = 24, 24
	setMinStrikeForce(t, 1)
	mgr := units.NewManager()
	grid := &access.Grid{W: w, H: h, Surf: allLand{}, Units: mgr.Cells}
	p := &game.StartParams{}
	p.Teams[0].Type = game.TeamPlayer
	p.Teams[1].Type = game.TeamComputer
	g := sim.NewWithGrid(game.NewGameState(p), mgr, grid, w, h)

	tank := armedTank(1, 1, 10, 10, mgr) // computer, damaged, no friendly building anywhere
	tank.Hits = 5
	seeAll(tank, w)
	_ = armedTank(0, 2, 14, 10, mgr) // player to the EAST, range 4 reaches (10,10)
	g.RefreshVisibility()

	tm := buildThreatMap(g, 1)
	startThreat := tm.at(tank.GridX, tank.GridY)
	if startThreat <= 0 {
		t.Fatal("setup: the damaged unit should start under enemy threat")
	}

	g.AI = New(testUnitFactory)
	g.BeginTeamTurn(1)
	if !g.HasOrder(tank) {
		t.Fatal("damaged tank with no shop issued no flee order")
	}
	g.ResolveMovement()
	if got := tm.at(tank.GridX, tank.GridY); got >= startThreat {
		t.Errorf("fled to threat %d, not below the start threat %d", got, startThreat)
	}
}

// TestAIHealthyUnitStillAttacks: retreat does not claim a full health unit - it
// still fires on an in range enemy.
func TestAIHealthyUnitStillAttacks(t *testing.T) {
	const w, h = 16, 16
	mgr := units.NewManager()
	grid := &access.Grid{W: w, H: h, Surf: allLand{}, Units: mgr.Cells}
	p := &game.StartParams{}
	p.Teams[0].Type = game.TeamPlayer
	p.Teams[1].Type = game.TeamComputer
	g := sim.NewWithGrid(game.NewGameState(p), mgr, grid, w, h)

	tank := armedTank(1, 1, 5, 5, mgr) // computer, FULL health
	tank.Values.Set(game.AttribScan, 6)
	enemy := armedTank(0, 2, 7, 5, mgr) // player, in range
	startHits := enemy.Hits

	g.AI = New(testUnitFactory)
	g.BeginTeamTurn(1)
	if enemy.Hits >= startHits {
		t.Error("a healthy unit did not attack (retreat wrongly claimed it)")
	}
}

// TestAIRetreatHoldsWithNoOptions: a damaged unit with no repair shop, no enemy and
// no buildings holds (no order, no panic) rather than wandering.
func TestAIRetreatHoldsWithNoOptions(t *testing.T) {
	const w, h = 16, 16
	mgr := units.NewManager()
	grid := &access.Grid{W: w, H: h, Surf: allLand{}, Units: mgr.Cells}
	p := &game.StartParams{}
	p.Teams[0].Type = game.TeamPlayer
	p.Teams[1].Type = game.TeamComputer
	g := sim.NewWithGrid(game.NewGameState(p), mgr, grid, w, h)

	tank := armedTank(1, 1, 8, 8, mgr) // alone: no enemy, no building
	tank.Hits = 5

	g.AI = New(testUnitFactory)
	g.BeginTeamTurn(1)
	if g.HasOrder(tank) {
		t.Error("a damaged unit with nowhere to go issued an order instead of holding")
	}
}

// TestSortApproachSafest: the caution ordering used for firing site choice puts the
// least threatened cell first, breaking ties by travel distance.
func TestSortApproachSafest(t *testing.T) {
	tm := &threatMap{w: 10, h: 10, cell: make([][]int, 10)}
	for x := range tm.cell {
		tm.cell[x] = make([]int, 10)
	}
	tm.cell[1][1] = 9 // near but dangerous
	tm.cell[5][5] = 0 // far but safe
	tm.cell[2][2] = 0 // safe and nearer than (5,5)
	cells := []sim.ApproachCell{
		{X: 1, Y: 1, D: 2},  // dist 2, threat 9
		{X: 5, Y: 5, D: 50}, // dist 50, threat 0
		{X: 2, Y: 2, D: 8},  // dist 8, threat 0
	}
	sortApproachSafest(cells, tm)
	if cells[0].X != 2 || cells[0].Y != 2 {
		t.Errorf("safest-nearest first = (%d,%d), want (2,2)", cells[0].X, cells[0].Y)
	}
	if cells[len(cells)-1].X != 1 {
		t.Errorf("most-threatened cell should sort last, got (%d,%d)", cells[len(cells)-1].X, cells[len(cells)-1].Y)
	}
}

// commandoUnit deploys an idle commando with one stealth action and movement to spend.
func commandoUnit(team, id, gx, gy int, mgr *units.Manager) *game.UnitInfo {
	u := game.NewUnitInfo(assets.Commando, team, id, 0)
	u.Flags = game.FlagMobileLandUnit | game.FlagSelectable
	v := &game.UnitValues{}
	v.Set(game.AttribRounds, 1)
	v.Set(game.AttribSpeed, 8)
	v.Set(game.AttribScan, 4)
	v.Set(game.AttribHits, 12)
	u.Values = v
	u.InitState()
	mgr.Deploy(u, gx, gy)
	return u
}

// electronicBuilding deploys an enemy electronic building (a disable/steal target)
// with an explicit id, so a test that also uses small fixed ids never collides with
// buildingUnit's global testFactorySeq when run in isolation.
func electronicBuilding(team, id int, ut assets.UnitType, large bool, gx, gy int, mgr *units.Manager) *game.UnitInfo {
	u := game.NewUnitInfo(ut, team, id, 0)
	u.Flags = buildingFlags(large) | game.FlagElectronicUnit
	u.Values = &game.UnitValues{}
	u.InitState()
	mgr.Deploy(u, gx, gy)
	return u
}

// mineLayerUnit deploys an idle land mine layer carrying a few mines (InitState loads
// it to ATTRIB_STORAGE).
func mineLayerUnit(team, id, gx, gy int, mgr *units.Manager) *game.UnitInfo {
	u := game.NewUnitInfo(assets.MineLayr, team, id, 0)
	u.Flags = game.FlagMobileLandUnit | game.FlagSelectable
	v := &game.UnitValues{}
	v.Set(game.AttribSpeed, 8)
	v.Set(game.AttribStorage, 5)
	u.Values = v
	u.InitState()
	mgr.Deploy(u, gx, gy)
	return u
}

// TestAICommandoMovesToKeyFacility: a commando prefers the enemy eco sphere (a key
// facility to disable) over a nearer electronic tank, and drives toward it when it is
// not yet adjacent.
func TestAICommandoMovesToKeyFacility(t *testing.T) {
	const w, h = 24, 24
	mgr := units.NewManager()
	grid := &access.Grid{W: w, H: h, Surf: allLand{}, Units: mgr.Cells}
	p := &game.StartParams{}
	p.Teams[0].Type = game.TeamPlayer
	p.Teams[1].Type = game.TeamComputer
	g := sim.NewWithGrid(game.NewGameState(p), mgr, grid, w, h)

	commando := commandoUnit(1, 1, 2, 2, mgr)
	nearTank := armedTank(0, 2, 6, 2, mgr) // player, nearer (d 4), electronic
	nearTank.Flags |= game.FlagElectronicUnit
	eco := electronicBuilding(0, 3, assets.Greenhse, true, 16, 16, mgr) // farther (d 14), key facility
	g.AddSpotted(1, nearTank, nearTank.GridX, nearTank.GridY, true)
	g.AddSpotted(1, eco, eco.GridX, eco.GridY, true)

	ct := newTaskCommando(1, &aiContext{Game: g, MakeUnit: testUnitFactory})
	if got := ct.bestInfiltrationTarget(commando); got != eco {
		t.Errorf("commando targeting picked %v, want the eco-sphere (key facility) over the nearer tank", got)
	}
	ct.BeginTurn()
	if !g.HasOrder(commando) {
		t.Error("a distant commando issued no approach order toward its target")
	}
}

// TestAICommandoAttemptsAdjacent: a commando standing next to an enemy electronic
// building attempts a disable, spending its shot (success is the RNG seam's business).
func TestAICommandoAttemptsAdjacent(t *testing.T) {
	const w, h = 16, 16
	mgr := units.NewManager()
	grid := &access.Grid{W: w, H: h, Surf: allLand{}, Units: mgr.Cells}
	p := &game.StartParams{}
	p.Teams[0].Type = game.TeamPlayer
	p.Teams[1].Type = game.TeamComputer
	g := sim.NewWithGrid(game.NewGameState(p), mgr, grid, w, h)

	commando := commandoUnit(1, 1, 5, 5, mgr)
	radar := electronicBuilding(0, 2, assets.Radar, false, 6, 5, mgr) // 1x1 electronic building, adjacent
	g.AddSpotted(1, radar, radar.GridX, radar.GridY, true)
	startShots := commando.Shots

	newTaskCommando(1, &aiContext{Game: g, MakeUnit: testUnitFactory}).BeginTurn()
	if commando.Shots != startShots-1 {
		t.Errorf("adjacent commando shots = %d, want %d (it should attempt a disable)", commando.Shots, startShots-1)
	}
}

// TestAICommandoIgnoresOrganicAndFogged: a commando does not target an organic enemy
// (cannot be infiltrated) nor an electronic enemy it has never spotted.
func TestAICommandoIgnoresOrganicAndFogged(t *testing.T) {
	const w, h = 24, 24
	mgr := units.NewManager()
	grid := &access.Grid{W: w, H: h, Surf: allLand{}, Units: mgr.Cells}
	p := &game.StartParams{}
	p.Teams[0].Type = game.TeamPlayer
	p.Teams[1].Type = game.TeamComputer
	g := sim.NewWithGrid(game.NewGameState(p), mgr, grid, w, h)

	commando := commandoUnit(1, 1, 5, 5, mgr)                                                            // scan 4
	infantry := armedUnit(0, 2, assets.Infantry, game.FlagMobileLandUnit|game.FlagSelectable, 6, 5, mgr) // organic, adjacent
	foggedTank := armedTank(0, 3, 20, 20, mgr)                                                           // electronic, far in fog
	foggedTank.Flags |= game.FlagElectronicUnit
	g.RefreshVisibility()
	if !g.SpottedByTeam(infantry, 1) {
		t.Fatal("setup: the adjacent organic enemy should be spotted")
	}
	if g.SpottedByTeam(foggedTank, 1) {
		t.Fatal("setup: the far electronic enemy should be fogged")
	}

	ct := newTaskCommando(1, &aiContext{Game: g, MakeUnit: testUnitFactory})
	if got := ct.bestInfiltrationTarget(commando); got != nil {
		t.Errorf("commando targeted %v; want none (organic = uninfiltrable, fogged = unseen)", got)
	}
}

// TestAILaysMineInDefenseBand: a mine layer standing on a defensive ring cell facing
// the threat lays a mine there and spends one from its store.
func TestAILaysMineInDefenseBand(t *testing.T) {
	const w, h = 24, 24
	mgr := units.NewManager()
	grid := &access.Grid{W: w, H: h, Surf: allLand{}, Units: mgr.Cells}
	p := &game.StartParams{}
	p.Teams[0].Type = game.TeamPlayer
	p.Teams[1].Type = game.TeamComputer
	g := sim.NewWithGrid(game.NewGameState(p), mgr, grid, w, h)

	_ = stationary(1, 1, assets.LandPlt, 12, 12, mgr) // base centre
	layer := mineLayerUnit(1, 2, 12, 7, mgr)          // 5 cells NORTH of centre: inside the ring
	threat := armedTank(0, 3, 12, 2, mgr)             // player to the NORTH: the threat direction
	g.AddSpotted(1, threat, threat.GridX, threat.GridY, true)
	startStore := layer.Storage

	newTaskMines(1, &aiContext{Game: g, MakeUnit: testUnitFactory}).BeginTurn()

	if layer.Storage != startStore-1 {
		t.Errorf("mine layer store = %d, want %d (it should have laid one)", layer.Storage, startStore-1)
	}
	laid := false
	for _, u := range g.Units.Units() {
		if u.Type == assets.LandMine && u.GridX == layer.GridX && u.GridY == layer.GridY {
			laid = true
		}
	}
	if !laid {
		t.Errorf("no mine was laid on the layer's cell (%d,%d)", layer.GridX, layer.GridY)
	}
}

// TestAIMinelayerDrivesToBand: a mine layer too close to the base centre (inside the
// ring) drives out to a ring cell instead of laying on the spot.
func TestAIMinelayerDrivesToBand(t *testing.T) {
	const w, h = 24, 24
	mgr := units.NewManager()
	grid := &access.Grid{W: w, H: h, Surf: allLand{}, Units: mgr.Cells}
	p := &game.StartParams{}
	p.Teams[0].Type = game.TeamPlayer
	p.Teams[1].Type = game.TeamComputer
	g := sim.NewWithGrid(game.NewGameState(p), mgr, grid, w, h)

	_ = stationary(1, 1, assets.LandPlt, 12, 12, mgr) // base centre
	layer := mineLayerUnit(1, 2, 12, 10, mgr)         // d 2 from centre: inside the inner radius
	threat := armedTank(0, 3, 12, 2, mgr)
	g.AddSpotted(1, threat, threat.GridX, threat.GridY, true)

	newTaskMines(1, &aiContext{Game: g, MakeUnit: testUnitFactory}).BeginTurn()
	if !g.HasOrder(layer) {
		t.Error("a mine layer inside the ring issued no order to drive out to a ring cell")
	}
}

// TestAttackSkipsCommandoAndMinelayer: the gun line attack task never musters or drives
// a commando / mine layer, even if a scenario hands one an attack value - they act via
// their own tasks.
func TestAttackSkipsCommandoAndMinelayer(t *testing.T) {
	const w, h = 12, 12
	mgr := units.NewManager()
	grid := &access.Grid{W: w, H: h, Surf: allLand{}, Units: mgr.Cells}
	p := &game.StartParams{}
	p.Teams[1].Type = game.TeamComputer
	g := sim.NewWithGrid(game.NewGameState(p), mgr, grid, w, h)

	commando := commandoUnit(1, 1, 3, 3, mgr)
	commando.Values.Set(game.AttribAttack, 5) // even if armed, the gun line must skip it
	layer := mineLayerUnit(1, 2, 4, 4, mgr)
	layer.Values.Set(game.AttribAttack, 5)
	tank := armedTank(1, 3, 5, 5, mgr)

	ta := newTaskAttackEnemies(1, &aiContext{Game: g, MakeUnit: testUnitFactory})
	if ta.isArmedMobile(commando) {
		t.Error("the attack task counts a commando as gun-line muster; it should be left to taskCommando")
	}
	if ta.isArmedMobile(layer) {
		t.Error("the attack task counts a mine layer as gun-line muster; it should be left to taskMines")
	}
	if !ta.isArmedMobile(tank) {
		t.Error("a tank should still be armed-mobile (sanity)")
	}
}

// TestReservedGuardsScaleWithForce: the home reserve is a quarter of the armed force
// on a reserve keeping tier, nothing when the tier overcommits, and nothing for a
// force too small to spare anyone.
func TestReservedGuardsScaleWithForce(t *testing.T) {
	const w, h = 24, 24
	mgr := units.NewManager()
	grid := &access.Grid{W: w, H: h, Surf: allLand{}, Units: mgr.Cells}
	p := &game.StartParams{}
	p.Teams[1].Type = game.TeamComputer
	g := sim.NewWithGrid(game.NewGameState(p), mgr, grid, w, h)

	_ = stationary(1, 50, assets.MiningSt, 10, 10, mgr) // a base so teamBaseCenter succeeds
	for i := 0; i < 8; i++ {
		armedTank(1, i+1, 2, 2+i, mgr)
	}

	prev := aiKeepReserve
	t.Cleanup(func() { aiKeepReserve = prev })

	aiKeepReserve = false
	if n := len(reservedGuards(g, 1)); n != 0 {
		t.Errorf("keepReserve off: reserved %d, want 0 (the easier tiers overcommit)", n)
	}
	aiKeepReserve = true
	if n := len(reservedGuards(g, 1)); n != 2 {
		t.Errorf("force 8: reserved %d, want 2 (a quarter)", n)
	}

	// A force too small to spare a quarter reserves no one (force 3 -> 3/4 = 0).
	mgr2 := units.NewManager()
	grid2 := &access.Grid{W: w, H: h, Surf: allLand{}, Units: mgr2.Cells}
	g2 := sim.NewWithGrid(game.NewGameState(p), mgr2, grid2, w, h)
	_ = stationary(1, 50, assets.MiningSt, 10, 10, mgr2)
	for i := 0; i < 3; i++ {
		armedTank(1, i+1, 2, 2+i, mgr2)
	}
	if n := len(reservedGuards(g2, 1)); n != 0 {
		t.Errorf("force 3: reserved %d, want 0 (too small to spare a quarter)", n)
	}
}

// TestAIReservesGuardsNearKeyFacility: on a reserve keeping tier the armed unit closest
// to the base is held as a guard and driven to its post by a key building, while the
// far units stay free for the assault.
func TestAIReservesGuardsNearKeyFacility(t *testing.T) {
	setDifficulty(t, OpponentMaster) // keepReserve = true
	const w, h = 24, 24
	mgr := units.NewManager()
	grid := &access.Grid{W: w, H: h, Surf: allLand{}, Units: mgr.Cells}
	p := &game.StartParams{}
	p.Teams[0].Type = game.TeamPlayer
	p.Teams[1].Type = game.TeamComputer
	g := sim.NewWithGrid(game.NewGameState(p), mgr, grid, w, h)

	_ = stationary(1, 50, assets.MiningSt, 10, 10, mgr) // the key facility (the base)
	guard := armedTank(1, 1, 6, 10, mgr)                // closest to base -> the lone reserve guard
	_ = armedTank(1, 2, 2, 2, mgr)                      // far -> free attackers
	_ = armedTank(1, 3, 2, 5, mgr)
	_ = armedTank(1, 4, 2, 18, mgr)

	reserve := reservedGuards(g, 1)
	if len(reserve) != 1 || !reserve[guard] {
		t.Fatalf("reserved %d units, want exactly the closest guard (id 1)", len(reserve))
	}

	newTaskDefenseReserve(1, &aiContext{Game: g, MakeUnit: testUnitFactory}).BeginTurn()
	if !g.HasOrder(guard) {
		t.Error("the reserve guard issued no order to take up a post by the key facility")
	}
}

// TestAttackSkipsReservedGuard: the gun line attack task leaves a reserve guard alone
// (taskDefenseReserve owns it) but still treats a non reserved armed unit as ready.
func TestAttackSkipsReservedGuard(t *testing.T) {
	setDifficulty(t, OpponentMaster) // keepReserve = true
	const w, h = 24, 24
	mgr := units.NewManager()
	grid := &access.Grid{W: w, H: h, Surf: allLand{}, Units: mgr.Cells}
	p := &game.StartParams{}
	p.Teams[1].Type = game.TeamComputer
	g := sim.NewWithGrid(game.NewGameState(p), mgr, grid, w, h)

	_ = stationary(1, 50, assets.MiningSt, 10, 10, mgr)
	guard := armedTank(1, 1, 9, 10, mgr) // closest -> reserved
	free := armedTank(1, 2, 2, 2, mgr)   // far -> free
	_ = armedTank(1, 3, 2, 5, mgr)
	_ = armedTank(1, 4, 2, 18, mgr)

	ta := newTaskAttackEnemies(1, &aiContext{Game: g, MakeUnit: testUnitFactory})
	ta.reserve = reservedGuards(g, 1)
	if ta.isReadyAttacker(guard) {
		t.Error("the attack task counts a reserve guard as a ready attacker; it should skip it")
	}
	if !ta.isReadyAttacker(free) {
		t.Error("a non-reserved armed unit should still be a ready attacker")
	}
}

// TestAIReserveInterceptsThreat: a guard on post fires on a spotted enemy that presses
// its key building (instead of holding) - the defensive engage.
func TestAIReserveInterceptsThreat(t *testing.T) {
	setDifficulty(t, OpponentMaster) // keepReserve = true
	const w, h = 24, 24
	mgr := units.NewManager()
	grid := &access.Grid{W: w, H: h, Surf: allLand{}, Units: mgr.Cells}
	p := &game.StartParams{}
	p.Teams[0].Type = game.TeamPlayer
	p.Teams[1].Type = game.TeamComputer
	g := sim.NewWithGrid(game.NewGameState(p), mgr, grid, w, h)

	_ = stationary(1, 50, assets.MiningSt, 10, 10, mgr) // the guarded key facility
	guard := armedTank(1, 1, 9, 10, mgr)                // on post, closest -> reserved
	_ = armedTank(1, 2, 2, 2, mgr)                      // far -> free
	_ = armedTank(1, 3, 2, 5, mgr)
	_ = armedTank(1, 4, 2, 18, mgr)
	enemy := armedTank(0, 99, 12, 12, mgr) // presses the facility, in the guard's range
	g.AddSpotted(1, enemy, enemy.GridX, enemy.GridY, true)

	if r := reservedGuards(g, 1); len(r) != 1 || !r[guard] {
		t.Fatalf("setup: reserved %d, want exactly the on-post guard", len(r))
	}
	startShots := guard.Shots
	newTaskDefenseReserve(1, &aiContext{Game: g, MakeUnit: testUnitFactory}).BeginTurn()
	if guard.Shots >= startShots {
		t.Errorf("guard shots = %d (was %d); it should have fired on the intruder, not held", guard.Shots, startShots)
	}
}

// scannerUnit deploys an idle computer team Scanner (a fragile mobile radar mover the
// escort task protects). armedUnit gives it stats; escortableTarget keys off the TYPE.
func scannerUnit(team, id, gx, gy int, mgr *units.Manager) *game.UnitInfo {
	return armedUnit(team, id, assets.Scanner, game.FlagMobileLandUnit|game.FlagSelectable, gx, gy, mgr)
}

// TestEscortDisabledByDefault: with escort off (the default / Average tier) no mover is
// escorted, so the assault and explore passes are unaffected by construction.
func TestEscortDisabledByDefault(t *testing.T) {
	prev := escortEnabled
	escortEnabled = false
	t.Cleanup(func() { escortEnabled = prev })

	const w, h = 24, 24
	mgr := units.NewManager()
	grid := &access.Grid{W: w, H: h, Surf: allLand{}, Units: mgr.Cells}
	p := &game.StartParams{}
	p.Teams[1].Type = game.TeamComputer
	g := sim.NewWithGrid(game.NewGameState(p), mgr, grid, w, h)

	_ = scannerUnit(1, 1, 10, 10, mgr)
	for i := 0; i < 4; i++ {
		armedTank(1, i+2, 2, 2+i, mgr)
	}
	if n := len(escortAssignments(g, 1)); n != 0 {
		t.Errorf("escort off: assigned %d escorts, want 0", n)
	}
}

// TestEscortAssignsGuardToMover: on an escort tier a fragile Scanner gets the nearest
// spare tank as a shadow, which drives to its charge, while the rest stay free to attack.
func TestEscortAssignsGuardToMover(t *testing.T) {
	setDifficulty(t, OpponentExpert) // escort on, muster 3
	const w, h = 24, 24
	mgr := units.NewManager()
	grid := &access.Grid{W: w, H: h, Surf: allLand{}, Units: mgr.Cells}
	p := &game.StartParams{}
	p.Teams[0].Type = game.TeamPlayer
	p.Teams[1].Type = game.TeamComputer
	g := sim.NewWithGrid(game.NewGameState(p), mgr, grid, w, h)

	scanner := scannerUnit(1, 1, 10, 10, mgr)
	guard := armedTank(1, 2, 6, 10, mgr) // nearest to the Scanner -> the escort
	free := armedTank(1, 3, 2, 2, mgr)   // far -> stays in the gun line
	_ = armedTank(1, 4, 2, 5, mgr)
	_ = armedTank(1, 5, 18, 18, mgr)

	a := escortAssignments(g, 1)
	if len(a) != 1 || a[guard] != scanner {
		t.Fatalf("assigned %d escorts, want exactly the nearest tank (id 2) shadowing the Scanner", len(a))
	}

	newTaskEscort(1, &aiContext{Game: g, MakeUnit: testUnitFactory}).BeginTurn()
	if !g.HasOrder(guard) {
		t.Error("the escort guard issued no order to move to its charge")
	}
	ta := newTaskAttackEnemies(1, &aiContext{Game: g, MakeUnit: testUnitFactory})
	ta.escort = escortGuards(g, 1)
	if ta.isReadyAttacker(guard) {
		t.Error("the attack task counts the escort guard as a ready attacker; it should skip it")
	}
	if !ta.isReadyAttacker(free) {
		t.Error("a non-escort armed unit should still be a ready attacker")
	}
}

// TestEscortSurplusCap: escorts come only from the SPARE force - a team is never taken
// below its assault muster, so a team with no surplus escorts nobody.
func TestEscortSurplusCap(t *testing.T) {
	setDifficulty(t, OpponentExpert) // escort on, muster 3
	const w, h = 24, 24
	p := &game.StartParams{}
	p.Teams[1].Type = game.TeamComputer

	// No surplus: a Scanner + exactly muster (3) tanks -> nobody to spare.
	mgr := units.NewManager()
	grid := &access.Grid{W: w, H: h, Surf: allLand{}, Units: mgr.Cells}
	g := sim.NewWithGrid(game.NewGameState(p), mgr, grid, w, h)
	_ = scannerUnit(1, 1, 10, 10, mgr)
	for i := 0; i < 3; i++ {
		armedTank(1, i+2, 2, 2+i, mgr)
	}
	if n := len(escortAssignments(g, 1)); n != 0 {
		t.Errorf("3 guards (= muster): assigned %d, want 0 (no spare force)", n)
	}

	// One spare: a 4th tank can be spared as the escort.
	mgr2 := units.NewManager()
	grid2 := &access.Grid{W: w, H: h, Surf: allLand{}, Units: mgr2.Cells}
	g2 := sim.NewWithGrid(game.NewGameState(p), mgr2, grid2, w, h)
	_ = scannerUnit(1, 1, 10, 10, mgr2)
	for i := 0; i < 4; i++ {
		armedTank(1, i+2, 2, 2+i, mgr2)
	}
	if n := len(escortAssignments(g2, 1)); n != 1 {
		t.Errorf("4 guards (muster + 1): assigned %d, want 1 (the one spare)", n)
	}
}

// TestExploreSkipsEscort: an escort guard that could otherwise scout (it has scan range)
// is left on its charge, while a free scan unit is still a ready explorer.
func TestExploreSkipsEscort(t *testing.T) {
	setDifficulty(t, OpponentExpert) // escort on, muster 3
	const w, h = 24, 24
	mgr := units.NewManager()
	grid := &access.Grid{W: w, H: h, Surf: allLand{}, Units: mgr.Cells}
	p := &game.StartParams{}
	p.Teams[1].Type = game.TeamComputer
	g := sim.NewWithGrid(game.NewGameState(p), mgr, grid, w, h)

	_ = scannerUnit(1, 1, 10, 10, mgr)
	guard := armedTank(1, 2, 6, 10, mgr) // nearest -> the escort
	free := armedTank(1, 3, 2, 2, mgr)
	_ = armedTank(1, 4, 2, 5, mgr)
	_ = armedTank(1, 5, 18, 18, mgr)
	guard.Values.Set(game.AttribScan, 3) // both could scout if free
	free.Values.Set(game.AttribScan, 3)

	te := newTaskExplore(1, &aiContext{Game: g, MakeUnit: testUnitFactory})
	te.escort = escortGuards(g, 1)
	if te.isReadyExplorer(guard) {
		t.Error("the explore task would pull the escort guard off its charge to scout")
	}
	if !te.isReadyExplorer(free) {
		t.Error("a free scan unit should still be a ready explorer")
	}
}

// TestEscortGuardEngagesThreatInRange: an escort on station fires on a spotted enemy that
// comes into range rather than only hugging its charge.
func TestEscortGuardEngagesThreatInRange(t *testing.T) {
	setDifficulty(t, OpponentExpert) // escort on, muster 3
	const w, h = 24, 24
	mgr := units.NewManager()
	grid := &access.Grid{W: w, H: h, Surf: allLand{}, Units: mgr.Cells}
	p := &game.StartParams{}
	p.Teams[0].Type = game.TeamPlayer
	p.Teams[1].Type = game.TeamComputer
	g := sim.NewWithGrid(game.NewGameState(p), mgr, grid, w, h)

	scanner := scannerUnit(1, 1, 10, 10, mgr)
	guard := armedTank(1, 2, 9, 10, mgr) // on station beside the Scanner (the escort)
	_ = armedTank(1, 3, 2, 2, mgr)
	_ = armedTank(1, 4, 2, 5, mgr)
	_ = armedTank(1, 5, 18, 18, mgr)
	enemy := armedTank(0, 99, 11, 12, mgr) // in the guard's weapon range
	g.AddSpotted(1, enemy, enemy.GridX, enemy.GridY, true)

	if a := escortAssignments(g, 1); len(a) != 1 || a[guard] != scanner {
		t.Fatalf("setup: want exactly the on-station tank escorting the Scanner")
	}
	startShots := guard.Shots
	newTaskEscort(1, &aiContext{Game: g, MakeUnit: testUnitFactory}).BeginTurn()
	if guard.Shots >= startShots {
		t.Errorf("escort shots = %d (was %d); it should have engaged the intruder, not just hugged its charge", guard.Shots, startShots)
	}
}

// TestEscortAirTargetGetsAirGuard: an air mover (Bomber) is shadowed by an air guard
// (Fighter), not a ground tank that cannot keep up - the movement domain match.
func TestEscortAirTargetGetsAirGuard(t *testing.T) {
	setDifficulty(t, OpponentExpert) // escort on
	setMinStrikeForce(t, 1)          // so two guards leave one to spare
	const w, h = 24, 24
	mgr := units.NewManager()
	grid := &access.Grid{W: w, H: h, Surf: allLand{}, Units: mgr.Cells}
	p := &game.StartParams{}
	p.Teams[1].Type = game.TeamComputer
	g := sim.NewWithGrid(game.NewGameState(p), mgr, grid, w, h)

	bomber := armedUnit(1, 1, assets.Bomber, game.FlagMobileAirUnit|game.FlagHovering|game.FlagSelectable, 10, 10, mgr)
	tank := armedTank(1, 2, 8, 10, mgr) // ground: wrong domain
	fighter := armedUnit(1, 3, assets.Fighter, game.FlagMobileAirUnit|game.FlagHovering|game.FlagSelectable, 9, 10, mgr)

	a := escortAssignments(g, 1)
	if len(a) != 1 || a[fighter] != bomber {
		t.Fatalf("assigned %d escorts, want the air Fighter shadowing the Bomber", len(a))
	}
	if _, used := a[tank]; used {
		t.Error("a ground tank was assigned to escort an air mover; the domains must match")
	}
}

// TestEscortPrefersDirectFireGuard: a direct fire guard (tank) is chosen to shadow a mover
// even when a long range artillery unit is nearer, so the artillery stays on the gun line
// to fire from range; artillery is only taken when nothing better is free.
func TestEscortPrefersDirectFireGuard(t *testing.T) {
	setDifficulty(t, OpponentExpert) // escort on
	setMinStrikeForce(t, 1)          // two guards leave exactly one to spare
	const w, h = 24, 24

	// A nearer rocket launcher and a farther tank: the tank (direct fire) wins.
	mgr := units.NewManager()
	grid := &access.Grid{W: w, H: h, Surf: allLand{}, Units: mgr.Cells}
	p := &game.StartParams{}
	p.Teams[1].Type = game.TeamComputer
	g := sim.NewWithGrid(game.NewGameState(p), mgr, grid, w, h)

	scanner := scannerUnit(1, 1, 10, 10, mgr)
	artillery := armedUnit(1, 2, assets.RocktLch, game.FlagMobileLandUnit|game.FlagSelectable, 8, 10, mgr) // nearer
	tank := armedTank(1, 3, 6, 10, mgr)                                                                    // farther, direct fire

	a := escortAssignments(g, 1)
	if len(a) != 1 || a[tank] != scanner {
		t.Fatalf("assigned %d escorts, want the direct-fire tank shadowing the Scanner (not the nearer artillery)", len(a))
	}
	if _, used := a[artillery]; used {
		t.Error("the long-range artillery was pulled onto escort duty while a tank was free; it should stay on the gun-line")
	}

	// Only artillery free: it is still taken as a last resort (better than no escort).
	mgr2 := units.NewManager()
	grid2 := &access.Grid{W: w, H: h, Surf: allLand{}, Units: mgr2.Cells}
	g2 := sim.NewWithGrid(game.NewGameState(p), mgr2, grid2, w, h)
	scanner2 := scannerUnit(1, 1, 10, 10, mgr2)
	rl1 := armedUnit(1, 2, assets.RocktLch, game.FlagMobileLandUnit|game.FlagSelectable, 8, 10, mgr2)
	_ = armedUnit(1, 3, assets.Artillry, game.FlagMobileLandUnit|game.FlagSelectable, 6, 10, mgr2)
	a2 := escortAssignments(g2, 1)
	if len(a2) != 1 || a2[rl1] != scanner2 {
		t.Fatalf("only artillery free: assigned %d, want the nearest artillery as a last-resort escort", len(a2))
	}
}

// TestEscortSkipsUnarmedSupportUnit: a weaponless support unit (Supply Truck) is never used
// as an escort even when a bench deploy stamped it with placeholder combat stats, so it is
// excluded by TYPE; a real combat unit shadows the mover instead. Guards the random2-bench
// case where deployUnit gives every unit a flat Attack=12.
func TestEscortSkipsUnarmedSupportUnit(t *testing.T) {
	setDifficulty(t, OpponentExpert) // escort on
	setMinStrikeForce(t, 1)          // two tanks leave one to spare
	const w, h = 24, 24
	mgr := units.NewManager()
	grid := &access.Grid{W: w, H: h, Surf: allLand{}, Units: mgr.Cells}
	p := &game.StartParams{}
	p.Teams[1].Type = game.TeamComputer
	g := sim.NewWithGrid(game.NewGameState(p), mgr, grid, w, h)

	scanner := scannerUnit(1, 1, 10, 10, mgr)
	// A supply truck with bench style placeholder combat stats (armedUnit sets Attack 12),
	// sitting NEAREST the mover - it must still be skipped.
	supply := armedUnit(1, 2, assets.SplyTrck, game.FlagMobileLandUnit|game.FlagSelectable, 9, 10, mgr)
	tank := armedTank(1, 3, 7, 10, mgr)
	_ = armedTank(1, 4, 5, 10, mgr)

	if armedMobileUnit(supply, 1) {
		t.Error("a supply truck counts as an armed mobile unit; a weaponless support type must be excluded")
	}
	a := escortAssignments(g, 1)
	if len(a) != 1 || a[tank] != scanner {
		t.Fatalf("assigned %d escorts, want a real tank shadowing the Scanner (not the nearer supply truck)", len(a))
	}
	if _, used := a[supply]; used {
		t.Error("the unarmed supply truck was assigned as an escort; it cannot fight and must be skipped")
	}
}

// TestAITargetsSofterEnemyUnderThreat: within a target class the threat map steers the
// AI to a softer, even slightly farther target instead of charging the most defended
// one. With no threat map it falls back to the nearest target.
func TestAITargetsSofterEnemyUnderThreat(t *testing.T) {
	const w, h = 32, 32
	mgr := units.NewManager()
	grid := &access.Grid{W: w, H: h, Surf: allLand{}, Units: mgr.Cells}
	p := &game.StartParams{}
	p.Teams[0].Type = game.TeamPlayer
	p.Teams[1].Type = game.TeamComputer
	g := sim.NewWithGrid(game.NewGameState(p), mgr, grid, w, h)

	aiTank := armedTank(1, 1, 12, 12, mgr)
	softer := armedTank(0, 2, 12, 19, mgr)  // FARTHER (d 7), undefended
	defended := armedTank(0, 3, 12, 6, mgr) // NEARER (d 6) but heavily defended
	guard := armedTank(0, 4, 12, 4, mgr)    // defends the nearer tank (range reaches it)
	guard.Values.Set(game.AttribRounds, 4)  // a heavy threat stamp on the defended cell
	for _, e := range []*game.UnitInfo{softer, defended, guard} {
		g.AddSpotted(1, e, e.GridX, e.GridY, true)
	}

	ta := newTaskAttackEnemies(1, &aiContext{Game: g, MakeUnit: testUnitFactory})
	if got := ta.nearestEnemy(aiTank); got != defended {
		t.Errorf("with no threat map picked %v, want the nearest (defended) tank", got)
	}
	ta.tm = buildThreatMap(g, 1)
	if got := ta.nearestEnemy(aiTank); got != softer {
		t.Errorf("threat-weighted pick = %v, want the softer (less-defended) tank", got)
	}
}

// TestSelectClanByStrategy checks the strategy->clan mapping (port of the original's
// AiPlayer::SelectTeamClan): the deterministic strategies always land on their
// clan, and the probabilistic ones stay within the expected set and 1..8.
func TestSelectClanByStrategy(t *testing.T) {
	// Deterministic strategies map to one clan every time.
	det := map[Strategy]int{
		StrategyScoutHorde: clanSacredEights,
		StrategyFastAttack: clanSacredEights,
		StrategyTankHorde:  clanMusashi,
	}
	for strat, want := range det {
		p := &AiPlayer{strategy: strat}
		for seed := uint32(1); seed <= 50; seed++ {
			if got := p.SelectClan(game.NewRandomizer(seed)); got != want {
				t.Fatalf("strategy %s seed %d: clan = %d, want %d", strategyName(strat), seed, got, want)
			}
		}
	}

	// Probabilistic strategies: the result is always a valid clan, the themed
	// clan is the common case, and Axis Inc. shows up as the 1/7 fallback.
	type spread struct {
		strat   Strategy
		themed  int
		allowed map[int]bool
	}
	for _, s := range []spread{
		{StrategyAir, clanTheChosen, map[int]bool{clanTheChosen: true, clanAxisInc: true}},
		{StrategyDefensive, clanSevenKnights, map[int]bool{clanSevenKnights: true, clanAxisInc: true}},
		{StrategyCombinedArms, clanVonGriffin, map[int]bool{clanVonGriffin: true, clanAyersHand: true, clanMusashi: true, clanAxisInc: true}},
	} {
		p := &AiPlayer{strategy: s.strat}
		themedHits, axisHits := 0, 0
		for seed := uint32(1); seed <= 400; seed++ {
			got := p.SelectClan(game.NewRandomizer(seed))
			if got < 1 || got > 8 {
				t.Fatalf("strategy %s seed %d: clan %d out of 1..8", strategyName(s.strat), seed, got)
			}
			if !s.allowed[got] {
				t.Fatalf("strategy %s seed %d: clan %d not in allowed set", strategyName(s.strat), seed, got)
			}
			if got == s.themed {
				themedHits++
			}
			if got == clanAxisInc {
				axisHits++
			}
		}
		if themedHits == 0 {
			t.Errorf("strategy %s never picked its themed clan %d", strategyName(s.strat), s.themed)
		}
		if axisHits == 0 {
			t.Errorf("strategy %s never fell back to Axis Inc.", strategyName(s.strat))
		}
	}

	// A Random/unknown strategy falls back to a uniform pick across 1..8.
	p := &AiPlayer{strategy: StrategyRandom}
	seen := map[int]bool{}
	for seed := uint32(1); seed <= 400; seed++ {
		got := p.SelectClan(game.NewRandomizer(seed))
		if got < 1 || got > 8 {
			t.Fatalf("random strategy seed %d: clan %d out of 1..8", seed, got)
		}
		seen[got] = true
	}
	if len(seen) < 6 {
		t.Errorf("random strategy only produced %d distinct clans, want a wide spread", len(seen))
	}
}

// moveDest returns the final cell of u's issued move order, or false if u has none.
func moveDest(g *sim.Game, u *game.UnitInfo) (game.Point, bool) {
	for _, mo := range g.MoveOrders() {
		if mo.Unit == u && len(mo.Steps) > 0 {
			return mo.Steps[len(mo.Steps)-1].To, true
		}
	}
	return game.Point{}, false
}

// TestOrderMoveAvoidingDetoursAroundDanger checks the danger weighted move seam. A
// unit sent straight across an open map takes the direct line with no penalty, but
// with a penalty wall over that line it bows around the penalized cells (the AI's
// route around defended ground). The move executor still gets the plain terrain
// cost, so the detour does not change the movement budget.
func TestOrderMoveAvoidingDetoursAroundDanger(t *testing.T) {
	const w, h = 20, 11
	newGame := func() (*sim.Game, *units.Manager) {
		mgr := units.NewManager()
		grid := &access.Grid{W: w, H: h, Surf: allLand{}, Units: mgr.Cells}
		p := &game.StartParams{}
		p.Teams[0].Type = game.TeamComputer
		return sim.NewWithGrid(game.NewGameState(p), mgr, grid, w, h), mgr
	}

	// A high penalty wall on the middle row between the unit and its destination.
	wall := func(x, y int) int {
		if y == 5 && x >= 6 && x <= 13 {
			return 1000
		}
		return 0
	}

	// Plain move (no penalty): the cheapest route is the straight line, so it crosses
	// the wall - the blind behaviour the fix corrects.
	gPlain, mgrPlain := newGame()
	moverPlain := armedTank(0, 1, 2, 5, mgrPlain)
	if !gPlain.OrderMove(moverPlain, 17, 5) {
		t.Fatal("plain move found no route")
	}
	crossed := false
	if _, ok := moveDest(gPlain, moverPlain); !ok {
		t.Fatal("plain move issued no order")
	}
	for _, mo := range gPlain.MoveOrders() {
		if mo.Unit != moverPlain {
			continue
		}
		for _, s := range mo.Steps {
			if wall(s.To.X, s.To.Y) > 0 {
				crossed = true
			}
		}
	}
	if !crossed {
		t.Fatal("test setup: the plain straight route did not cross the wall")
	}

	// Danger weighted move: the same route with the wall as a penalty must avoid
	// every penalized cell.
	gAvoid, mgrAvoid := newGame()
	moverAvoid := armedTank(0, 1, 2, 5, mgrAvoid)
	if !gAvoid.OrderMoveAvoiding(moverAvoid, 17, 5, wall) {
		t.Fatal("danger weighted move found no route")
	}
	if _, ok := moveDest(gAvoid, moverAvoid); !ok {
		t.Fatal("danger weighted move issued no order")
	}
	for _, mo := range gAvoid.MoveOrders() {
		if mo.Unit != moverAvoid {
			continue
		}
		for _, s := range mo.Steps {
			if wall(s.To.X, s.To.Y) > 0 {
				t.Errorf("danger weighted route stepped onto penalized cell (%d,%d)", s.To.X, s.To.Y)
			}
		}
	}
}

// TestImmediateThreatBeatsDistantFacility: an armed enemy that can already fire on
// the unit (within its weapon range) is engaged ahead of a farther key facility, so
// the attacker deals with the gun trained on it instead of pressing on and being
// whittled down. This is the counterpart to TestAIRaidsKeyFacility, where the same
// generic enemy is OUT of range and the eco sphere rightly wins.
func TestImmediateThreatBeatsDistantFacility(t *testing.T) {
	const w, h = 24, 24
	mgr := units.NewManager()
	grid := &access.Grid{W: w, H: h, Surf: allLand{}, Units: mgr.Cells}
	p := &game.StartParams{}
	p.Teams[0].Type = game.TeamPlayer
	p.Teams[1].Type = game.TeamComputer
	g := sim.NewWithGrid(game.NewGameState(p), mgr, grid, w, h)

	// buildingUnit draws its id from the shared test sequence, so derive the tanks'
	// ids from it to guarantee no collision (a shared id would overwrite the scanner
	// in the unit hash and blind the team).
	greenhouse := buildingUnit(0, assets.Greenhse, true) // key facility, farther off
	g.Units.Deploy(greenhouse, 14, 2)
	aiTank := armedTank(1, int(greenhouse.ID)+1, 2, 2, mgr)
	seeAll(aiTank, w)                                        // scan the whole map so the shooter is VISIBLE, not a fogged memory
	shooter := armedTank(0, int(greenhouse.ID)+2, 4, 2, mgr) // d 2, within its range 4: an immediate threat
	g.RefreshVisibility()                                    // record the sightings so fogged() sees the shooter in the open
	g.AddSpotted(1, shooter, shooter.GridX, shooter.GridY, true)
	g.AddSpotted(1, greenhouse, greenhouse.GridX, greenhouse.GridY, true)

	ta := newTaskAttackEnemies(1, &aiContext{Game: g, MakeUnit: testUnitFactory})
	if got := ta.nearestEnemy(aiTank); got != shooter {
		t.Errorf("targeting picked %v, want the in range shooter (immediate threat) over the distant eco sphere", got)
	}
}

// TestReconAvoidsThreatenedFrontier: a scout heads for its nearest unexplored
// frontier cell, but once a spotted armed enemy covers that cell the explore task
// sends it elsewhere and never routes it onto threatened ground - a fragile recon
// unit stops driving into a defended area.
func TestReconAvoidsThreatenedFrontier(t *testing.T) {
	const w, h = 21, 21
	newGame := func() (*sim.Game, *units.Manager) {
		mgr := units.NewManager()
		grid := &access.Grid{W: w, H: h, Surf: allLand{}, Units: mgr.Cells}
		p := &game.StartParams{}
		p.Teams[0].Type = game.TeamPlayer
		p.Teams[1].Type = game.TeamComputer
		return sim.NewWithGrid(game.NewGameState(p), mgr, grid, w, h), mgr
	}

	// Baseline with no enemy: record the frontier cell the scout drives to.
	gBase, mgrBase := newGame()
	scoutBase := scoutUnit(1, 1, 10, 10, 2, mgrBase)
	gBase.RefreshVisibility()
	newTaskExplore(1, &aiContext{Game: gBase, MakeUnit: testUnitFactory}).BeginTurn()
	base, ok := moveDest(gBase, scoutBase)
	if !ok {
		t.Fatal("baseline: scout issued no explore move")
	}

	// Same setup, but a spotted enemy now sits between the scout and that cell so its
	// weapon range disc covers it. The scout must pick a different, threat free cell.
	g, mgr := newGame()
	scout := scoutUnit(1, 1, 10, 10, 2, mgr)
	ex, ey := base.X, base.Y // one step from base back toward the scout: covers base, leaves it enterable
	if base.X > 10 {
		ex--
	} else if base.X < 10 {
		ex++
	}
	if base.Y > 10 {
		ey--
	} else if base.Y < 10 {
		ey++
	}
	enemy := armedTank(0, 2, ex, ey, mgr)
	g.AddSpotted(1, enemy, enemy.GridX, enemy.GridY, true)
	g.RefreshVisibility()

	newTaskExplore(1, &aiContext{Game: g, MakeUnit: testUnitFactory}).BeginTurn()

	tm := buildThreatMap(g, 1)
	if tm.at(base.X, base.Y) == 0 {
		t.Fatalf("test setup: enemy at (%d,%d) does not threaten the baseline cell (%d,%d)", ex, ey, base.X, base.Y)
	}
	if dest, moved := moveDest(g, scout); moved {
		if tm.at(dest.X, dest.Y) > 0 {
			t.Errorf("recon scout drove onto threatened cell (%d,%d)", dest.X, dest.Y)
		}
		if dest == base {
			t.Errorf("recon scout still chose the now-threatened baseline cell (%d,%d)", base.X, base.Y)
		}
	}
}

// TestReconAvoidsRecentLossSite: with NO enemy spotted (so the enemy firepower threat
// is zero), a fresh unit loss on the scout's chosen frontier cell still steers it away
// - the AI does not send a second unit back to where it just lost one. Once the
// difficulty scaled forget window passes, the loss no longer stamps danger.
func TestReconAvoidsRecentLossSite(t *testing.T) {
	const w, h = 21, 21
	newG := func() (*sim.Game, *units.Manager) {
		mgr := units.NewManager()
		grid := &access.Grid{W: w, H: h, Surf: allLand{}, Units: mgr.Cells}
		p := &game.StartParams{}
		p.Teams[0].Type = game.TeamPlayer
		p.Teams[1].Type = game.TeamComputer
		return sim.NewWithGrid(game.NewGameState(p), mgr, grid, w, h), mgr
	}

	// Baseline with no loss: record the frontier cell the scout drives to.
	gBase, mgrBase := newG()
	scoutBase := scoutUnit(1, 1, 10, 10, 2, mgrBase)
	gBase.RefreshVisibility()
	newTaskExplore(1, &aiContext{Game: gBase, MakeUnit: testUnitFactory}).BeginTurn()
	base, ok := moveDest(gBase, scoutBase)
	if !ok {
		t.Fatal("baseline: scout issued no explore move")
	}

	// Same setup, but the team just lost a unit on that frontier cell. No enemy is
	// spotted, so only the loss memory can steer the scout.
	g, mgr := newG()
	scout := scoutUnit(1, 1, 10, 10, 2, mgr)
	g.State.TurnCounter = 30
	g.DestroyUnit(armedTank(1, 2, base.X, base.Y, mgr)) // a team-1 unit dies right on the frontier cell
	g.RefreshVisibility()
	newTaskExplore(1, &aiContext{Game: g, MakeUnit: testUnitFactory}).BeginTurn()

	tm := buildThreatMap(g, 1)
	if tm.at(base.X, base.Y) == 0 {
		t.Fatalf("test setup: the loss at (%d,%d) did not stamp danger", base.X, base.Y)
	}
	if dest, moved := moveDest(g, scout); moved {
		if tm.at(dest.X, dest.Y) > 0 {
			t.Errorf("recon scout drove onto a loss danger cell (%d,%d)", dest.X, dest.Y)
		}
		if dest == base {
			t.Errorf("recon scout drove back to the cell where it just lost a unit (%d,%d)", base.X, base.Y)
		}
	}

	// Decay: once the forget window passes, the loss no longer stamps danger there.
	g.State.TurnCounter = 30 + aiLossMemoryTurns + 1
	if tmFaded := buildThreatMap(g, 1); tmFaded.at(base.X, base.Y) != 0 {
		t.Errorf("loss should fade after %d turns, but base is still danger-stamped (%d)", aiLossMemoryTurns, tmFaded.at(base.X, base.Y))
	}
}

// TestLossMemoryScalesWithDifficulty: a weaker tier forgets a loss site sooner than a
// stronger one (the difficulty scaled forget window), all within the 15..25 band.
func TestLossMemoryScalesWithDifficulty(t *testing.T) {
	setDifficulty(t, OpponentClueless)
	clueless := aiLossMemoryTurns
	setDifficulty(t, OpponentGod)
	god := aiLossMemoryTurns

	if !(clueless >= 15 && god <= 25 && clueless < god) {
		t.Errorf("loss memory window: Clueless=%d God=%d, want 15<=Clueless<God<=25", clueless, god)
	}
}

// TestSurveyorAvoidsRecentLossSite: like the recon scout, an idle Surveyor no longer
// drives back to a cell where the team just lost a unit. It mirrors the loss memory
// test but through taskSurvey (the surveyor's own driver), proving the danger
// avoidance now also covers the unarmed resource scout.
func TestSurveyorAvoidsRecentLossSite(t *testing.T) {
	const w, h = 21, 21
	newG := func() (*sim.Game, *units.Manager) {
		mgr := units.NewManager()
		grid := &access.Grid{W: w, H: h, Surf: allLand{}, Units: mgr.Cells}
		p := &game.StartParams{}
		p.Teams[0].Type = game.TeamPlayer
		p.Teams[1].Type = game.TeamComputer
		g := sim.NewWithGrid(game.NewGameState(p), mgr, grid, w, h)
		g.Resources = economy.NewResourceMap(w, h) // nothing surveyed: the whole map is a frontier
		return g, mgr
	}

	// Baseline with no loss: record the cell the surveyor drives to.
	gBase, mgrBase := newG()
	survBase := surveyorUnit(1, 1, 10, 10, mgrBase)
	gBase.RefreshVisibility()
	newTaskSurvey(1, &aiContext{Game: gBase, MakeUnit: testUnitFactory}).BeginTurn()
	base, ok := moveDest(gBase, survBase)
	if !ok {
		t.Fatal("baseline: surveyor issued no survey move")
	}

	// Same setup, but the team just lost a unit on that cell. No enemy is spotted, so
	// only the loss memory can steer the surveyor off it.
	g, mgr := newG()
	surv := surveyorUnit(1, 1, 10, 10, mgr)
	g.State.TurnCounter = 30
	g.DestroyUnit(armedTank(1, 2, base.X, base.Y, mgr)) // a team-1 unit dies right on the frontier cell
	g.RefreshVisibility()
	newTaskSurvey(1, &aiContext{Game: g, MakeUnit: testUnitFactory}).BeginTurn()

	tm := buildThreatMap(g, 1)
	if tm.at(base.X, base.Y) == 0 {
		t.Fatalf("test setup: the loss at (%d,%d) did not stamp danger", base.X, base.Y)
	}
	if dest, moved := moveDest(g, surv); moved {
		if tm.at(dest.X, dest.Y) > 0 {
			t.Errorf("surveyor drove onto a loss danger cell (%d,%d)", dest.X, dest.Y)
		}
		if dest == base {
			t.Errorf("surveyor drove back to the cell where the team just lost a unit (%d,%d)", base.X, base.Y)
		}
	}
}

// TestAIInitAppliesStateDifficulty: a game whose state carries an opponent tier (a
// setup choice or a loaded save) has that tier adopted by the AI on init, so a loaded
// game plays at the saved strength instead of the launch default.
func TestAIInitAppliesStateDifficulty(t *testing.T) {
	setDifficulty(t, OpponentAverage) // baseline; registers the full knob restore on cleanup
	const w, h = 12, 12
	mgr := units.NewManager()
	grid := &access.Grid{W: w, H: h, Surf: allLand{}, Units: mgr.Cells}
	p := &game.StartParams{}
	p.Teams[0].Type = game.TeamPlayer
	p.Teams[1].Type = game.TeamComputer
	g := sim.NewWithGrid(game.NewGameState(p), mgr, grid, w, h)
	g.State.Difficulty = OpponentGod // the game carries a God tier (as a loaded save would)

	a := New(testUnitFactory)
	g.AI = a
	g.BeginTeamTurn(1) // drives init, which must adopt the game's tier

	if a.Difficulty() != OpponentGod {
		t.Errorf("AI difficulty after init = %d, want God(%d) from the game state", a.Difficulty(), OpponentGod)
	}
	if minStrikeForce != profileFor(OpponentGod).muster {
		t.Errorf("God knobs not applied: minStrikeForce = %d, want %d", minStrikeForce, profileFor(OpponentGod).muster)
	}
}

// TestAILoadReappliesDifficulty: a load swaps a fresh game instance in behind the same
// AI object; the next computer turn must reinitialise against it and adopt the loaded game's tier.
func TestAILoadReappliesDifficulty(t *testing.T) {
	setDifficulty(t, OpponentAverage) // baseline + full knob restore on cleanup
	mk := func(tier int) *sim.Game {
		const w, h = 12, 12
		mgr := units.NewManager()
		grid := &access.Grid{W: w, H: h, Surf: allLand{}, Units: mgr.Cells}
		p := &game.StartParams{}
		p.Teams[0].Type = game.TeamPlayer
		p.Teams[1].Type = game.TeamComputer
		g := sim.NewWithGrid(game.NewGameState(p), mgr, grid, w, h)
		g.State.Difficulty = tier
		return g
	}

	g1 := mk(OpponentExpert)
	a := New(testUnitFactory)
	g1.AI = a
	g1.BeginTeamTurn(1)
	if a.Difficulty() != OpponentExpert {
		t.Fatalf("first game difficulty = %d, want Expert(%d)", a.Difficulty(), OpponentExpert)
	}

	g2 := mk(OpponentGod)
	g2.AI = a // the SAME AI, now wired onto a freshly loaded game instance
	g2.BeginTeamTurn(1)
	if a.Difficulty() != OpponentGod {
		t.Errorf("after a load the AI kept the old tier: difficulty = %d, want God(%d) from the loaded game", a.Difficulty(), OpponentGod)
	}
}

// TestSecondComputerTeamActs: a game with two computer teams must drive BOTH, not just
// the first. The second computer team's idle constructor issues a build order on its
// turn (the old "second team is passive" limitation is gone).
func TestSecondComputerTeamActs(t *testing.T) {
	const w, h = 12, 12
	mgr := units.NewManager()
	grid := &access.Grid{W: w, H: h, Surf: allLand{}, Units: mgr.Cells}
	p := &game.StartParams{}
	p.Teams[0].Type = game.TeamPlayer
	p.Teams[1].Type = game.TeamComputer
	p.Teams[2].Type = game.TeamComputer // a SECOND computer team
	g := sim.NewWithGrid(game.NewGameState(p), mgr, grid, w, h)

	// A loaded constructor for the SECOND computer team, room to build around it.
	con := game.NewUnitInfo(assets.Constrct, 2, 1, 0)
	con.Flags = game.FlagConstructorUnit | game.FlagMobileLandUnit | game.FlagSelectable
	con.Values = &game.UnitValues{}
	con.InitState()
	con.Values.Set(game.AttribStorage, 40)
	con.Storage = 40
	g.Units.Deploy(con, 6, 6)

	g.AI = New(testUnitFactory)
	g.BeginTeamTurn(2) // the second computer team's turn

	if len(g.BuildOrders()) == 0 {
		t.Error("the second computer team placed no build order; it is passive")
	}
}

// TestLaunchDifficultyCapturedToState: the launch path chooses a tier through
// SetDifficulty before the AI has a game handle (game state still unset). init must
// then capture that tier onto the game state, so a save of a fresh menu game carries
// the chosen difficulty and a reload plays at it (not the default).
func TestLaunchDifficultyCapturedToState(t *testing.T) {
	setDifficulty(t, OpponentAverage) // baseline + full knob restore on cleanup
	const w, h = 12, 12
	mgr := units.NewManager()
	grid := &access.Grid{W: w, H: h, Surf: allLand{}, Units: mgr.Cells}
	p := &game.StartParams{}
	p.Teams[0].Type = game.TeamPlayer
	p.Teams[1].Type = game.TeamComputer
	g := sim.NewWithGrid(game.NewGameState(p), mgr, grid, w, h)
	if g.State.Difficulty != game.DifficultyUnset {
		t.Fatalf("fresh game difficulty = %d, want Unset(%d)", g.State.Difficulty, game.DifficultyUnset)
	}

	a := New(testUnitFactory)
	a.SetDifficulty(OpponentGod) // the launch path sets the tier BEFORE the AI has a game handle
	g.AI = a
	g.BeginTeamTurn(1) // init runs and must capture the launch tier onto the state

	if g.State.Difficulty != OpponentGod {
		t.Errorf("launch tier not captured onto the game state: difficulty = %d, want God(%d) so a save carries it", g.State.Difficulty, OpponentGod)
	}
}

// TestAIBuildRateFollowsTheMaterial pins the rate gate. The AI used to commit every build at
// x1 for the whole game while a human played at x2 or x4, so it raised its base and turned out
// its army at a quarter speed. It now reads the same limit the player's build menu does: a
// builder with material to burn goes faster, a poor one still crawls at x1.
func TestAIBuildRateFollowsTheMaterial(t *testing.T) {
	const w, h = 16, 16
	mgr := units.NewManager()
	grid := &access.Grid{W: w, H: h, Surf: allLand{}, Units: mgr.Cells}
	p := &game.StartParams{}
	p.Teams[1].Type = game.TeamComputer
	g := sim.NewWithGrid(game.NewGameState(p), mgr, grid, w, h)

	builder := game.NewUnitInfo(assets.Constrct, 1, 1, 0)
	builder.Flags = game.FlagMobileLandUnit | game.FlagConstructorUnit | game.FlagSelectable
	builder.Values = &game.UnitValues{}
	builder.Values.Set(game.AttribStorage, 60)
	g.Units.Deploy(builder, 5, 5)

	product := game.NewUnitInfo(assets.LandPlt, 1, 2, 0)
	product.Values = &game.UnitValues{}
	product.Values.Set(game.AttribTurns, 8)

	builder.Storage = 4 // barely enough for one turn of work
	if rate := aiBuildRate(g, builder, product); rate != 1 {
		t.Errorf("a builder with 4 raw committed at x%d; a poor team must crawl at x1", rate)
	}

	builder.Storage = 60 // a full load
	if rate := aiBuildRate(g, builder, product); rate <= 1 {
		t.Errorf("a builder with a full load still committed at x%d; it should spend it to build faster", rate)
	}

	// It must never exceed what the player's own gate allows for the same job.
	if got, want := aiBuildRate(g, builder, product), build.MaxBuildRate(assets.Constrct, 8, 60); got != want {
		t.Errorf("the AI's rate (x%d) differs from the player's gate (x%d) for the same job", got, want)
	}
}

// laidMine deploys a mine the way the sim does: the retail LANDMINE row (attack 28, 2 hits) and
// NO weapon range, so it never reaches the threat map through the normal enemy firepower stamp -
// only through the mine layer, which is what these tests are about.
func laidMine(team, id, gx, gy int, mgr *units.Manager) *game.UnitInfo {
	u := game.NewUnitInfo(assets.LandMine, team, id, 0)
	u.Flags = game.FlagStationary
	v := &game.UnitValues{}
	v.Set(game.AttribAttack, 28)
	v.Set(game.AttribHits, 2)
	u.Values = v
	u.InitState()
	mgr.Deploy(u, gx, gy)

	return u
}

// TestThreatMapStampsKnownMines: a mine the team has UNCOVERED is danger ground on its threat
// map (and the patch around it, which the original assumes is mined too), while an enemy mine
// it has never found stays invisible to it - no omniscience.
func TestThreatMapStampsKnownMines(t *testing.T) {
	const w, h = 24, 24
	mgr := units.NewManager()
	grid := &access.Grid{W: w, H: h, Surf: allLand{}, Units: mgr.Cells}
	p := &game.StartParams{}
	p.Teams[0].Type = game.TeamPlayer
	p.Teams[1].Type = game.TeamComputer
	g := sim.NewWithGrid(game.NewGameState(p), mgr, grid, w, h)

	found := laidMine(0, 1, 10, 10, mgr)
	found.SpotBy(1) // the computer's surveyor drove past this one

	_ = laidMine(0, 2, 4, 4, mgr) // never found

	own := laidMine(1, 3, 18, 18, mgr) // the AI's OWN mine
	own.SpotBy(1)

	g.RefreshVisibility()

	tm := buildThreatMap(g, 1)

	if got := tm.at(10, 10); got < mineDangerValue {
		t.Errorf("threat at the found mine = %d, want at least %d: the AI must not drive over it", got, mineDangerValue)
	}
	if tm.at(11, 10) <= 0 {
		t.Error("the patch around a found mine should read as danger too (mines come in fields)")
	}
	if tm.at(4, 4) != 0 {
		t.Error("an enemy mine the team never found leaked onto its threat map (omniscience)")
	}
	if tm.at(18, 18) != 0 {
		t.Error("the AI flagged its OWN mine as danger; it drives over those safely")
	}
}

// TestAIRoutesAroundAKnownMine: the same threat map, driven through the route penalty the AI
// moves with - the path it takes does not cross the mine it has found.
func TestAIRoutesAroundAKnownMine(t *testing.T) {
	// A narrow corridor, so stepping around the mine costs real movement: with the mine
	// carrying no more danger than a puff of enemy fire, A* takes the straight line over it.
	const w, h = 16, 3
	mgr := units.NewManager()
	grid := &access.Grid{W: w, H: h, Surf: allLand{}, Units: mgr.Cells}
	p := &game.StartParams{}
	p.Teams[0].Type = game.TeamPlayer
	p.Teams[1].Type = game.TeamComputer
	g := sim.NewWithGrid(game.NewGameState(p), mgr, grid, w, h)

	mine := laidMine(0, 1, 8, 1, mgr) // dead ahead on the straight line east
	mine.SpotBy(1)

	tank := armedTank(1, 2, 2, 1, mgr) // computer, drives east along the middle row
	tank.Values.Set(game.AttribSpeed, 40)

	g.RefreshVisibility()
	g.BeginTeamTurn(1)
	if !g.OrderMoveAvoiding(tank, 14, 1, dangerPenalty(buildThreatMap(g, 1))) {
		t.Fatal("the computer's move was rejected")
	}

	// The whole point is the ROUTE, not where it ends up: check no step lands on the mine.
	orders := g.MoveOrders()
	if len(orders) != 1 {
		t.Fatalf("want the tank's move order, got %d orders", len(orders))
	}
	for _, st := range orders[0].RemainingSteps() {
		if st.To.X == mine.GridX && st.To.Y == mine.GridY {
			t.Fatal("the computer routed straight over a mine it had already found")
		}
	}

	// And the penalty is what does it: the mine is passable ground, so the plain route (what
	// every unpenalized AI move takes) drives right across it. Also pin the danger high enough
	// to beat a detour - at the old value a found mine cost about one extra step, which A* will
	// happily pay to keep a straight line.
	if pen := dangerPenalty(buildThreatMap(g, 1))(mine.GridX, mine.GridY); pen < mineDangerValue*dangerRouteWeight {
		t.Errorf("the route penalty on a found mine is only %d; too cheap to steer a unit around it", pen)
	}

	g.CancelOrder(tank)
	if !g.OrderMove(tank, 14, 1) {
		t.Fatal("the plain move was rejected")
	}
	crossed := false
	for _, st := range g.MoveOrders()[0].RemainingSteps() {
		if st.To.X == mine.GridX && st.To.Y == mine.GridY {
			crossed = true
		}
	}
	if !crossed {
		t.Error("setup no longer proves anything: the plain route already misses the mine, so the penalty is untested")
	}
	g.CancelOrder(tank)
	if !g.OrderMoveAvoiding(tank, 14, 1, dangerPenalty(buildThreatMap(g, 1))) {
		t.Fatal("the computer's move was rejected")
	}

	g.ResolveMovement()

	if tank.GridX != 14 || tank.GridY != 1 {
		t.Errorf("the tank ended at (%d,%d), want its destination (14,1): it should route AROUND the mine, not give up", tank.GridX, tank.GridY)
	}
}
