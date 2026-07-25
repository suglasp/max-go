// saveload_test.go : testing unit for saveload.go
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package saveload

import (
	"bytes"
	"testing"

	"github.com/suglasp/max-go/internal/assets"
	"github.com/suglasp/max-go/internal/game"
	"github.com/suglasp/max-go/internal/sim"
	"github.com/suglasp/max-go/internal/world"
)

// landMap builds a synthetic all land map for round trip tests.
func landMap(w, h int) *world.Map {
	m := &world.Map{}
	m.W, m.H = w, h
	m.Surface = make([]uint8, w*h)
	for i := range m.Surface {
		m.Surface[i] = world.SurfaceLand
	}
	return m
}

// newTestGame builds a tiny all land map with a couple of units for round trip
// tests, without touching real game data.
func newTestGame(t *testing.T) *sim.Game {
	t.Helper()
	m := landMap(16, 16)
	p := &game.StartParams{}
	p.Teams[0].Type = game.TeamPlayer
	p.Teams[1].Type = game.TeamComputer
	g := sim.New(p, m)

	tank := game.NewUnitInfo(assets.Tank, 0, 1, 2)
	tank.Flags = game.FlagMobileLandUnit | game.FlagSelectable
	tank.Values = &game.UnitValues{}
	tank.Values.Set(game.AttribHits, 24)
	tank.Values.Set(game.AttribSpeed, 8)
	tank.InitState()
	tank.Hits = 18 // damaged, to prove runtime state survives
	g.Units.Deploy(tank, 3, 4)

	gen := game.NewUnitInfo(assets.PowGen, 0, 2, 0)
	gen.Flags = game.FlagStationary
	gen.Values = &game.UnitValues{}
	gen.Values.Set(game.AttribHits, 40)
	gen.InitState()
	g.Units.Deploy(gen, 8, 8)

	g.State.TurnCounter = 5
	g.State.ActiveTeam = 0
	g.Gold[0] = 120
	return g
}

func TestSaveLoadRoundTrip(t *testing.T) {
	g := newTestGame(t)
	game.RandomizerSetSeed(1234)
	for i := 0; i < 7; i++ {
		game.RandomizerGetNext() // advance the RNG so its mid stream state must survive
	}
	wantRNG := game.RandomizerState()

	var buf bytes.Buffer
	if err := Save(&buf, Capture(g, "GREEN_1.WRL")); err != nil {
		t.Fatalf("save: %v", err)
	}
	snap, err := Load(&buf)
	if err != nil {
		t.Fatalf("load: %v", err)
	}

	if snap.MapName != "GREEN_1.WRL" || snap.Turn != 5 || snap.Gold[0] != 120 {
		t.Errorf("header mismatch: %+v", snap)
	}
	if snap.RNG != wantRNG {
		t.Errorf("RNG not preserved: got %v want %v", snap.RNG, wantRNG)
	}

	g2 := snap.Restore(landMap(16, 16))
	if got := len(g2.Units.Units()); got != 2 {
		t.Fatalf("restored unit count = %d, want 2", got)
	}
	if g2.State.TurnCounter != 5 || g2.Gold[0] != 120 {
		t.Errorf("restored turn/gold wrong: turn=%d gold=%d", g2.State.TurnCounter, g2.Gold[0])
	}

	var tank *game.UnitInfo
	for _, u := range g2.Units.Units() {
		if u.Type == assets.Tank {
			tank = u
		}
	}
	if tank == nil {
		t.Fatal("tank not restored")
	}
	if tank.GridX != 3 || tank.GridY != 4 || tank.Hits != 18 || tank.Angle != 2 {
		t.Errorf("tank state wrong: %+v", tank)
	}
	if tank.Values == nil || tank.Values.Get(game.AttribSpeed) != 8 {
		t.Errorf("tank values not restored: %+v", tank.Values)
	}

	// The restored RNG must match what was saved.
	game.RandomizerSetState(snap.RNG)
	if game.RandomizerState() != wantRNG {
		t.Error("RNG state mismatch after restore")
	}
}

func TestLoadRejectsBadMagic(t *testing.T) {
	if _, err := Load(bytes.NewReader([]byte("not a save file at all"))); err == nil {
		t.Fatal("expected error on bad magic")
	}
}

// makeUnit builds and deploys a statted unit for a round trip test.
func makeUnit(g *sim.Game, ut assets.UnitType, team, id, gx, gy int, flags game.Flags) *game.UnitInfo {
	u := game.NewUnitInfo(ut, team, id, 0)
	u.Flags = flags
	u.Values = &game.UnitValues{}
	u.Values.Set(game.AttribHits, 24)
	u.InitState()
	g.Units.Deploy(u, gx, gy)
	return u
}

// TestSaveRestoreCarriedPassengers checks passengers stored in Carried rather
// than the unit registry. They must remain linked to their carrier and stay off
// the map after a save and load.
func TestSaveRestoreCarriedPassengers(t *testing.T) {
	m := landMap(16, 16)
	p := &game.StartParams{}
	p.Teams[0].Type = game.TeamPlayer
	g := sim.New(p, m)

	transport := makeUnit(g, assets.ClnTrans, 0, 1, 5, 5, game.FlagMobileLandUnit|game.FlagSelectable)
	// Board two passengers, mirroring LoadUnit: off the registry, only in Carried.
	for _, spec := range []struct {
		ut assets.UnitType
		id int
	}{{assets.Tank, 10}, {assets.Infantry, 11}} {
		pas := game.NewUnitInfo(spec.ut, 0, spec.id, 0)
		pas.Flags = game.FlagMobileLandUnit
		pas.Values = &game.UnitValues{}
		pas.Values.Set(game.AttribHits, 24)
		pas.InitState()
		pas.Parent = transport
		transport.Carried = append(transport.Carried, pas)
	}

	var buf bytes.Buffer
	if err := Save(&buf, Capture(g, "GREEN_1.WRL")); err != nil {
		t.Fatalf("save: %v", err)
	}
	snap, err := Load(&buf)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	g2 := snap.Restore(landMap(16, 16))

	onMap := g2.Units.Units()
	if len(onMap) != 1 {
		t.Fatalf("on-map unit count = %d, want 1 (the transport; passengers are aboard)", len(onMap))
	}
	tr := onMap[0]
	if tr.Type != assets.ClnTrans {
		t.Fatalf("on-map unit is %v, want the transport", tr.Type)
	}
	if len(tr.Carried) != 2 {
		t.Fatalf("restored transport carries %d, want 2", len(tr.Carried))
	}
	types := map[assets.UnitType]bool{}
	for _, c := range tr.Carried {
		if c.Parent != tr {
			t.Errorf("carried %v Parent not re-linked to the transport", c.Type)
		}
		types[c.Type] = true
	}
	if !types[assets.Tank] || !types[assets.Infantry] {
		t.Errorf("carried types wrong: %v", types)
	}
}

// TestSaveRestoreAttackTargetPursuit: a unit's move to attack AttackTarget pointer
// round trips by ID; a dangling target (not in the snapshot) restores to nil.
func TestSaveRestoreAttackTargetPursuit(t *testing.T) {
	m := landMap(16, 16)
	p := &game.StartParams{}
	p.Teams[0].Type = game.TeamPlayer
	p.Teams[1].Type = game.TeamComputer
	g := sim.New(p, m)

	attacker := makeUnit(g, assets.Tank, 0, 1, 3, 3, game.FlagMobileLandUnit|game.FlagSelectable)
	target := makeUnit(g, assets.Tank, 1, 2, 9, 9, game.FlagMobileLandUnit|game.FlagSelectable)
	attacker.AttackTarget = target

	var buf bytes.Buffer
	if err := Save(&buf, Capture(g, "GREEN_1.WRL")); err != nil {
		t.Fatalf("save: %v", err)
	}
	snap, err := Load(&buf)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	g2 := snap.Restore(landMap(16, 16))

	var atk *game.UnitInfo
	for _, u := range g2.Units.Units() {
		if u.ID == 1 {
			atk = u
		}
	}
	if atk == nil {
		t.Fatal("attacker not restored")
	}
	if atk.AttackTarget == nil || atk.AttackTarget.ID != 2 {
		t.Errorf("AttackTarget not re-linked by ID: %v", atk.AttackTarget)
	}

	// Dangling: an AttackTargetID with no matching unit restores to nil (no panic).
	snap2 := Capture(g, "GREEN_1.WRL")
	for i := range snap2.Units {
		if snap2.Units[i].ID == 1 {
			snap2.Units[i].AttackTargetID = 9999
		}
	}
	g3 := snap2.Restore(landMap(16, 16))
	for _, u := range g3.Units.Units() {
		if u.ID == 1 && u.AttackTarget != nil {
			t.Errorf("dangling AttackTarget should restore to nil, got ID %d", u.AttackTarget.ID)
		}
	}
}

// TestSaveRestoreBuildOrderResumes checks that a constructor's active product, progress, builder link, and build pose survive a round trip.
func TestSaveRestoreBuildOrderResumes(t *testing.T) {
	m := landMap(16, 16)
	p := &game.StartParams{}
	p.Teams[0].Type = game.TeamPlayer
	g := sim.New(p, m)

	con := makeUnit(g, assets.Constrct, 0, 1, 5, 5,
		game.FlagMobileLandUnit|game.FlagConstructorUnit|game.FlagSelectable)
	product := game.NewUnitInfo(assets.PowGen, 0, 0, 0)
	product.Flags = game.FlagStationary
	pv := &game.UnitValues{}
	pv.Set(game.AttribHits, 40)
	pv.Set(game.AttribTurns, 4) // a 4-turn build
	product.Values = pv
	o := g.StartBuild(con, product, 5, 5, 1)
	o.Progress = 2 // halfway
	o.Started = true
	if !con.Building {
		t.Fatal("setup: constructor should be in the build pose after StartBuild")
	}

	var buf bytes.Buffer
	if err := Save(&buf, Capture(g, "GREEN_1.WRL")); err != nil {
		t.Fatalf("save: %v", err)
	}
	snap, err := Load(&buf)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	g2 := snap.Restore(landMap(16, 16))

	orders := g2.BuildOrders()
	if len(orders) != 1 {
		t.Fatalf("restored build orders = %d, want 1", len(orders))
	}
	ro := orders[0]
	if ro.Builder == nil || ro.Builder.ID != 1 {
		t.Errorf("build order builder not re-linked by ID: %v", ro.Builder)
	}
	if ro.Product == nil || ro.Product.Type != assets.PowGen {
		t.Errorf("build order product wrong: %v", ro.Product)
	}
	if ro.GridX != 5 || ro.GridY != 5 || ro.BuildRate != 1 || ro.Progress != 2 || !ro.Started {
		t.Errorf("build order state wrong: %+v", ro)
	}
	if ro.TotalTurns != 4 {
		t.Errorf("TotalTurns re-derived wrong: %d (want 4)", ro.TotalTurns)
	}
	if ro.Builder != nil && !ro.Builder.Building {
		t.Error("restored constructor not re-posed (Building=false)")
	}
}

// TestSaveRestoreFactoryQueue checks that a factory retains its queue, repeat flag, and active head order.
func TestSaveRestoreFactoryQueue(t *testing.T) {
	m := landMap(16, 16)
	p := &game.StartParams{}
	p.Teams[0].Type = game.TeamPlayer
	g := sim.New(p, m)
	g.MakeUnit = func(team int, ut assets.UnitType, flags game.Flags) *game.UnitInfo {
		u := game.NewUnitInfo(ut, team, 0, 0)
		u.Flags = flags
		u.Values = &game.UnitValues{}
		u.Values.Set(game.AttribTurns, 3)
		u.Values.Set(game.AttribHits, 10)
		return u
	}
	factory := makeUnit(g, assets.LandPlt, 0, 1, 5, 5, game.FlagStationary|game.FlagBuilding|game.FlagSelectable)
	factory.Storage = 1000
	g.SetFactoryQueue(factory, []assets.UnitType{assets.Tank, assets.Scout}, 2, true)

	var buf bytes.Buffer
	if err := Save(&buf, Capture(g, "GREEN_1.WRL")); err != nil {
		t.Fatalf("save: %v", err)
	}
	snap, err := Load(&buf)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	g2 := snap.Restore(landMap(16, 16))

	var rf *game.UnitInfo
	for _, u := range g2.Units.Units() {
		if u.ID == 1 {
			rf = u
		}
	}
	if rf == nil {
		t.Fatal("factory not restored")
	}
	q, repeat := g2.FactoryQueue(rf)
	if len(q) != 2 || q[0] != assets.Tank || q[1] != assets.Scout {
		t.Errorf("restored queue = %v, want [Tank Scout]", q)
	}
	if !repeat {
		t.Error("RepeatBuild not restored")
	}
	if rf.BuildRate != 2 {
		t.Errorf("restored BuildRate = %d, want 2", rf.BuildRate)
	}
	if got := len(g2.BuildOrders()); got != 1 {
		t.Errorf("restored build orders = %d, want 1 (the in-flight head)", got)
	}
}

// TestSaveRestorePendingMove checks that an over-budget move resumes toward its saved destination on the next turn.
func TestSaveRestorePendingMove(t *testing.T) {
	m := landMap(16, 16)
	p := &game.StartParams{}
	p.Teams[0].Type = game.TeamPlayer
	g := sim.New(p, m)

	tank := makeUnit(g, assets.Tank, 0, 1, 2, 2, game.FlagMobileLandUnit|game.FlagSelectable)
	g.SetPendingDestination(tank, 12, 13) // an over budget move queued to (12,13)
	if len(g.PendingRoutes()) != 1 {
		t.Fatal("setup: expected a queued route")
	}

	var buf bytes.Buffer
	if err := Save(&buf, Capture(g, "GREEN_1.WRL")); err != nil {
		t.Fatalf("save: %v", err)
	}
	snap, err := Load(&buf)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	g2 := snap.Restore(landMap(16, 16))

	routes := g2.PendingRoutes()
	if len(routes) != 1 {
		t.Fatalf("restored pending routes = %d, want 1", len(routes))
	}
	r := routes[0]
	if r.Unit == nil || r.Unit.ID != 1 {
		t.Errorf("pending route unit not re-linked by ID: %v", r.Unit)
	}
	last := r.Steps[len(r.Steps)-1].To
	if len(r.Steps) == 0 || last.X != 12 || last.Y != 13 {
		t.Errorf("pending route destination wrong: %+v", r.Steps)
	}
}

// TestSaveRestoreBuildPath checks that an engineer retains the remaining cells and next index of an active line build.
func TestSaveRestoreBuildPath(t *testing.T) {
	m := landMap(16, 16)
	p := &game.StartParams{}
	p.Teams[0].Type = game.TeamPlayer
	g := sim.New(p, m)

	eng := makeUnit(g, assets.Engineer, 0, 1, 3, 3,
		game.FlagMobileLandUnit|game.FlagConstructorUnit|game.FlagSelectable)
	vals := &game.UnitValues{}
	vals.Set(game.AttribTurns, 1)
	cells := [][2]int{{3, 3}, {4, 3}, {5, 3}}
	g.RestoreBuildPath(eng, assets.Road, game.FlagStationary|game.FlagGroundCover, vals, 1, cells, 1)
	if len(g.BuildPathStates()) != 1 {
		t.Fatal("setup: expected a build path")
	}

	var buf bytes.Buffer
	if err := Save(&buf, Capture(g, "GREEN_1.WRL")); err != nil {
		t.Fatalf("save: %v", err)
	}
	snap, err := Load(&buf)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	g2 := snap.Restore(landMap(16, 16))

	states := g2.BuildPathStates()
	if len(states) != 1 {
		t.Fatalf("restored build paths = %d, want 1", len(states))
	}
	bp := states[0]
	if bp.Builder == nil || bp.Builder.ID != 1 {
		t.Errorf("build path builder not re-linked by ID: %v", bp.Builder)
	}
	if bp.Product != assets.Road || bp.Rate != 1 || bp.Idx != 1 {
		t.Errorf("build path state wrong: %+v", bp)
	}
	if len(bp.Cells) != 3 || bp.Cells[2][0] != 5 || bp.Cells[2][1] != 3 {
		t.Errorf("build path cells wrong: %v", bp.Cells)
	}
}

// TestSaveRestoreSpottedMemory checks that the AI retains a fogged enemy's last-seen cell when visibility is rebuilt.
func TestSaveRestoreSpottedMemory(t *testing.T) {
	m := landMap(16, 16)
	p := &game.StartParams{}
	p.Teams[0].Type = game.TeamPlayer
	p.Teams[1].Type = game.TeamComputer
	g := sim.New(p, m)

	enemy := makeUnit(g, assets.Tank, 0, 5, 2, 2, game.FlagMobileLandUnit|game.FlagSelectable)
	g.AddSpotted(1, enemy, 7, 7, false) // team 1 last saw it at (7,7), now fogged

	var buf bytes.Buffer
	if err := Save(&buf, Capture(g, "GREEN_1.WRL")); err != nil {
		t.Fatalf("save: %v", err)
	}
	snap, err := Load(&buf)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	g2 := snap.Restore(landMap(16, 16))

	var en2 *game.UnitInfo
	for _, u := range g2.Units.Units() {
		if u.ID == 5 {
			en2 = u
		}
	}
	if en2 == nil {
		t.Fatal("enemy not restored")
	}
	if !g2.SpottedByTeam(en2, 1) {
		t.Error("team 1 forgot the spotted enemy on load")
	}
	x, y, ok := g2.LastSeen(1, en2)
	if !ok || x != 7 || y != 7 {
		t.Errorf("frozen last-seen wrong: (%d,%d) ok=%v, want (7,7)", x, y, ok)
	}
}

// TestSaveRestoreNextBuiltAndRubble locks two M-P1b-3 fixes: the built unit ID
// counter survives (so post load IDs do not collide with saved ones) and the map's
// debris piles survive (Restore otherwise starts on a clean field, losing them).
func TestSaveRestoreNextBuiltAndRubble(t *testing.T) {
	m := landMap(16, 16)
	p := &game.StartParams{}
	p.Teams[0].Type = game.TeamPlayer
	g := sim.New(p, m)
	g.SetNextBuilt(42)
	g.RestoreRubble([]sim.RubbleCell{{X: 5, Y: 5, Large: true}, {X: 6, Y: 6, Large: false}})

	var buf bytes.Buffer
	if err := Save(&buf, Capture(g, "GREEN_1.WRL")); err != nil {
		t.Fatalf("save: %v", err)
	}
	snap, err := Load(&buf)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	g2 := snap.Restore(landMap(16, 16))

	if g2.NextBuilt() != 42 {
		t.Errorf("NextBuilt = %d, want 42", g2.NextBuilt())
	}
	if large, ok := g2.RubbleAt(5, 5); !ok || !large {
		t.Errorf("large pile at (5,5) lost: large=%v ok=%v", large, ok)
	}
	if large, ok := g2.RubbleAt(6, 6); !ok || large {
		t.Errorf("small pile at (6,6) wrong: large=%v ok=%v", large, ok)
	}
	if _, ok := g2.RubbleAt(0, 0); ok {
		t.Error("phantom pile at (0,0)")
	}
}

// TestSaveRestoreClearJob locks M-P1b-3: a bulldozer's in progress clear (the turns
// left + the hazard markers) survives a save + load, re linked by ID.
func TestSaveRestoreClearJob(t *testing.T) {
	m := landMap(16, 16)
	p := &game.StartParams{}
	p.Teams[0].Type = game.TeamPlayer
	g := sim.New(p, m)

	dozer := makeUnit(g, assets.Engineer, 0, 1, 5, 5, game.FlagMobileLandUnit|game.FlagSelectable)
	cones := makeUnit(g, assets.SmlCones, 0, 2, 5, 5, game.FlagStationary|game.FlagGroundCover)
	tape := makeUnit(g, assets.SmlTape, 0, 3, 5, 5, game.FlagStationary|game.FlagGroundCover)
	g.RestoreClearJob(dozer, 5, 5, 1, 3, []*game.UnitInfo{cones, tape})
	if !g.IsClearingRubble(dozer) {
		t.Fatal("setup: expected a clear job")
	}

	var buf bytes.Buffer
	if err := Save(&buf, Capture(g, "GREEN_1.WRL")); err != nil {
		t.Fatalf("save: %v", err)
	}
	snap, err := Load(&buf)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	g2 := snap.Restore(landMap(16, 16))

	jobs := g2.ClearJobStates()
	if len(jobs) != 1 {
		t.Fatalf("restored clear jobs = %d, want 1", len(jobs))
	}
	j := jobs[0]
	if j.Bulldozer == nil || j.Bulldozer.ID != 1 {
		t.Errorf("clear job dozer not re-linked: %v", j.Bulldozer)
	}
	if j.CellX != 5 || j.CellY != 5 || j.Footprint != 1 || j.TurnsLeft != 3 {
		t.Errorf("clear job state wrong: %+v", j)
	}
	if len(j.Markers) != 2 {
		t.Errorf("clear job markers not re-linked: %d, want 2", len(j.Markers))
	}
}

// TestSaveRestoreBuildReadyParkedBuilder checks a parked builder outside the
// unit registry. Saving and loading must preserve its link to the finished
// structure while keeping it off the grid.
func TestSaveRestoreBuildReadyParkedBuilder(t *testing.T) {
	m := landMap(16, 16)
	p := &game.StartParams{}
	p.Teams[0].Type = game.TeamPlayer
	g := sim.New(p, m)

	// The parked constructor: created but NOT deployed (off the registry, like the
	// live state after finishBuild's g.Units.Remove).
	con := game.NewUnitInfo(assets.Constrct, 0, 1, 0)
	con.Flags = game.FlagMobileLandUnit | game.FlagConstructorUnit
	con.Values = &game.UnitValues{}
	con.Values.Set(game.AttribHits, 30)
	con.InitState()
	// The finished structure it guards (on the map, hidden until the builder leaves).
	structure := makeUnit(g, assets.PowGen, 0, 2, 7, 7, game.FlagStationary)
	structure.HiddenUntilExpand = true
	g.RestoreBuildReady(con, structure, assets.PowGen, 7, 7, 1)

	var buf bytes.Buffer
	if err := Save(&buf, Capture(g, "GREEN_1.WRL")); err != nil {
		t.Fatalf("save: %v", err)
	}
	snap, err := Load(&buf)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	g2 := snap.Restore(landMap(16, 16))

	states := g2.BuildReadyStates()
	if len(states) != 1 {
		t.Fatalf("parked build-ready builder lost on save: %d states, want 1", len(states))
	}
	br := states[0]
	if br.Builder == nil || br.Builder.ID != 1 {
		t.Errorf("parked builder not re-linked by ID: %v", br.Builder)
	}
	if br.Building == nil || br.Building.ID != 2 {
		t.Errorf("finished structure not re-linked: %v", br.Building)
	}
	if br.Product != assets.PowGen || br.SiteX != 7 || br.SiteY != 7 || br.Footprint != 1 {
		t.Errorf("build-ready record wrong: %+v", br)
	}
	// The parked builder is now IN the registry after load (finishBuild keeps it in
	// Cells+ByID live; a builder saved off the registry is re added on restore) and
	// reads as build ready off that same instance.
	rb := g2.Units.ByID.Get(1)
	if rb == nil {
		t.Fatal("parked builder missing from the registry after load")
	}
	if !g2.IsBuildReady(rb) {
		t.Error("the registry builder does not read as build-ready (g.buildReady keyed by a different pointer)")
	}
	// The structure's hidden flag round trips.
	for _, u := range g2.Units.Units() {
		if u.ID == 2 && !u.HiddenUntilExpand {
			t.Error("structure HiddenUntilExpand not restored")
		}
	}
}

// TestRestoreBuildReadySelectableAndUnitReady checks that a parked builder restores as UNIT_READY in Cells and ByID, allowing BLDMRK exit selection.
func TestRestoreBuildReadySelectableAndUnitReady(t *testing.T) {
	m := landMap(16, 16)
	p := &game.StartParams{}
	p.Teams[0].Type = game.TeamPlayer
	g := sim.New(p, m)

	con := game.NewUnitInfo(assets.Constrct, 0, 1, 0)
	con.Flags = game.FlagMobileLandUnit | game.FlagConstructorUnit
	con.Values = &game.UnitValues{}
	con.Values.Set(game.AttribHits, 30)
	con.Values.Set(game.AttribSpeed, 8)
	con.InitState()
	// Model the LIVE parked state: the builder is IN the registry (finishBuild keeps
	// it in Cells+ByID) - so Capture saves it via the main unit list AND BuildReadyStates
	// (the double capture that used to mint an orphan pointer on restore).
	g.Units.Deploy(con, 7, 7)
	structure := makeUnit(g, assets.PowGen, 0, 2, 7, 7, game.FlagStationary)
	structure.HiddenUntilExpand = true
	g.RestoreBuildReady(con, structure, assets.PowGen, 7, 7, 1)

	var buf bytes.Buffer
	if err := Save(&buf, Capture(g, "GREEN_1.WRL")); err != nil {
		t.Fatalf("save: %v", err)
	}
	snap, err := Load(&buf)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	g2 := snap.Restore(landMap(16, 16))

	states := g2.BuildReadyStates()
	if len(states) != 1 {
		t.Fatalf("build-ready state lost: %d, want 1", len(states))
	}
	b := states[0].Builder

	// Finished, awaiting exit - not still mid construction.
	if b.Order != game.OrderBuild || b.State != game.OrderStateUnitReady {
		t.Errorf("restored builder Order/State = %v/%v, want OrderBuild/UnitReady", b.Order, b.State)
	}
	// Selectable: it must be in the cell hash at its parked site (a click finds it).
	found := false
	for _, u := range g2.Units.Cells.At(7, 7) {
		if u == b {
			found = true
		}
	}
	if !found {
		t.Error("restored build-ready builder not in the cell hash at its site - a click can't select it")
	}
	// Findable by ID (the exit / re link paths resolve it).
	if g2.Units.ByID.Get(1) != b {
		t.Error("restored build-ready builder not in the ID registry")
	}
	// The BLDMRK exit ring resolves, so the player can drive it off.
	if len(g2.ExitMarkers(b)) == 0 {
		t.Error("no BLDMRK exit markers for the restored build-ready builder")
	}
}

// TestSaveRestoreAutoSurvey checks that the v12 Surveyor auto-survey flag survives a round trip.
func TestSaveRestoreAutoSurvey(t *testing.T) {
	g := newTestGame(t)
	sur := game.NewUnitInfo(assets.Surveyor, 0, 7, 0)
	sur.Flags = game.FlagMobileLandUnit | game.FlagSelectable
	sur.Values = &game.UnitValues{}
	sur.Values.Set(game.AttribSpeed, 6)
	sur.InitState()
	sur.AutoSurvey = true
	g.Units.Deploy(sur, 5, 5)

	var buf bytes.Buffer
	if err := Save(&buf, Capture(g, "GREEN_1.WRL")); err != nil {
		t.Fatalf("save: %v", err)
	}
	snap, err := Load(&buf)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	g2 := snap.Restore(landMap(16, 16))

	var got *game.UnitInfo
	for _, u := range g2.Units.Units() {
		if u.Type == assets.Surveyor {
			got = u
		}
	}
	if got == nil {
		t.Fatal("surveyor not restored")
	}
	if !got.AutoSurvey {
		t.Error("AutoSurvey flag not round-tripped (should stay on after load)")
	}
}

// TestSaveRestoreAlienBuilding checks that an alien stationary building stays outside the four playable-team networks and restores without joining a complex.
func TestSaveRestoreAlienBuilding(t *testing.T) {
	g := newTestGame(t)
	// An alien derelict artifact building owned by the fixed 5th (alien) team.
	alien := makeUnit(g, assets.RecCentr, game.AlienTeam, 9, 10, 10, game.FlagStationary|game.FlagSelectable)
	_ = alien

	var buf bytes.Buffer
	if err := Save(&buf, Capture(g, "GREEN_1.WRL")); err != nil {
		t.Fatalf("save: %v", err)
	}
	snap, err := Load(&buf)
	if err != nil {
		t.Fatalf("load: %v", err)
	}

	// Alien team index 4 lies outside the playable-team network array.
	g2 := snap.Restore(landMap(16, 16))

	var got *game.UnitInfo
	for _, u := range g2.Units.Units() {
		if u.Type == assets.RecCentr {
			got = u
		}
	}
	if got == nil {
		t.Fatal("alien building not restored")
	}
	if got.Team != game.AlienTeam {
		t.Errorf("alien building team = %d, want %d (AlienTeam)", got.Team, game.AlienTeam)
	}
	// The alien building must NOT have joined a complex (aliens have no economy).
	if g2.ComplexOf(got) != nil {
		t.Error("alien building wrongly joined a complex on restore")
	}
}

// TestSaveRestoreSpottedMine checks the v21 sticky stealth mark: an enemy mine the
// player already uncovered is still uncovered after a reload, and one nobody found
// stays hidden.
func TestSaveRestoreSpottedMine(t *testing.T) {
	g := newTestGame(t)

	found := game.NewUnitInfo(assets.LandMine, 1, 41, 0)
	found.Flags = game.FlagStationary
	found.Values = &game.UnitValues{}
	found.SpotBy(0) // a surveyor walked past it
	g.Units.Deploy(found, 5, 5)

	hidden := game.NewUnitInfo(assets.LandMine, 1, 42, 0)
	hidden.Flags = game.FlagStationary
	hidden.Values = &game.UnitValues{}
	g.Units.Deploy(hidden, 9, 9)

	var buf bytes.Buffer
	if err := Save(&buf, Capture(g, "GREEN_1.WRL")); err != nil {
		t.Fatalf("save: %v", err)
	}
	snap, err := Load(&buf)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	g2 := snap.Restore(landMap(16, 16))

	for _, u := range g2.Units.Units() {
		switch u.ID {
		case 41:
			if !u.SpottedByTeam(0) {
				t.Error("an uncovered mine went hidden again across a save; the mark must ride the save")
			}
		case 42:
			if u.SpottedByTeam(0) {
				t.Error("a mine nobody found came back uncovered")
			}
		}
	}
}
