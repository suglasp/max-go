// popup_test.go : tests popup composition and unit-order actions.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package gamescreen

import (
	"strings"
	"testing"

	"github.com/suglasp/max-go/internal/access"
	"github.com/suglasp/max-go/internal/assets"
	"github.com/suglasp/max-go/internal/game"
	"github.com/suglasp/max-go/internal/sim"
	"github.com/suglasp/max-go/internal/units"
)

// TestPopupItems checks the per type popup composition: X-Fer only for
// cargo carrying units, no Stop on buildings (they get Power On/Off), and the
// mobile only / building only commons gating. A bare Screen suffices, lang and
// attribs are nil safe and HasOrder ranges the (nil) order slice.
func TestPopupItems(t *testing.T) {
	s := &Screen{game: &sim.Game{}}

	labels := func(u *game.UnitInfo) []string {
		var out []string
		for _, it := range s.popupItemsFor(u) {
			out = append(out, it.label)
		}
		return out
	}
	has := func(ls []string, want string) bool {
		for _, l := range ls {
			if l == want {
				return true
			}
		}
		return false
	}

	tests := []struct {
		name    string
		unit    *game.UnitInfo
		want    []string // labels that must be present
		notWant []string // labels that must be absent
	}{
		{
			name:    "tank carries nothing",
			unit:    &game.UnitInfo{Type: assets.Tank, Flags: game.FlagMobileLandUnit | game.FlagSelectable},
			want:    []string{"Sentry", "Done", "Enter"},
			notWant: []string{"X-Fer", "Build", "Remove", "Power On", "Power Off"},
		},
		{
			name:    "constructor carries raw + builds",
			unit:    &game.UnitInfo{Type: assets.Constrct, Flags: game.FlagMobileLandUnit | game.FlagConstructorUnit | game.FlagSelectable},
			want:    []string{"Build", "X-Fer", "Sentry", "Done"},
			notWant: []string{"Power Off", "Remove"},
		},
		{
			name:    "power station powered",
			unit:    &game.UnitInfo{Type: assets.PowerStn, Flags: game.FlagBuilding | game.FlagStationary, Order: game.OrderPowerOn},
			want:    []string{"Power Off", "Remove"},
			notWant: []string{"Stop", "Power On", "Sentry", "Enter", "Allocate"},
		},
		{
			name:    "power station off",
			unit:    &game.UnitInfo{Type: assets.PowerStn, Flags: game.FlagBuilding | game.FlagStationary, Order: game.OrderPowerOff},
			want:    []string{"Power On"},
			notWant: []string{"Stop", "Power Off"},
		},
		{
			name:    "mining station powered",
			unit:    &game.UnitInfo{Type: assets.MiningSt, Flags: game.FlagBuilding | game.FlagStationary, Order: game.OrderPowerOn},
			want:    []string{"Allocate", "Power Off", "X-Fer", "Remove"}, // mining stores raw -> X-Fer
			notWant: []string{"Stop", "Power On"},
		},
		{
			name:    "material dump has no power toggle",
			unit:    &game.UnitInfo{Type: assets.ADump, Flags: game.FlagBuilding | game.FlagStationary, Order: game.OrderPowerOn},
			want:    []string{"X-Fer", "Remove"}, // ADump stores raw
			notWant: []string{"Stop", "Power On", "Power Off", "Allocate"},
		},
		{
			name:    "gold refinery powered shows buy upgrade, not the generic upgrade",
			unit:    &game.UnitInfo{Type: assets.CommTwr, Flags: game.FlagBuilding | game.FlagStationary, Order: game.OrderPowerOn},
			want:    []string{"Buy Upgrade", "Power Off"},
			notWant: []string{"Upgrade"}, // the generic per unit label must be gone
		},
	}

	for _, tc := range tests {
		ls := labels(tc.unit)
		for _, w := range tc.want {
			if !has(ls, w) {
				t.Errorf("%s: missing %q in %v", tc.name, w, ls)
			}
		}
		for _, n := range tc.notWant {
			if has(ls, n) {
				t.Errorf("%s: unexpected %q in %v", tc.name, n, ls)
			}
		}
	}
}

// TestPopupNoDuplicateUpgrade verifies that only the Gold Refinery receives one Buy Upgrade action. Upgrades apply immediately, so units receive no in-place Upgrade action.
func TestPopupNoDuplicateUpgrade(t *testing.T) {
	s := &Screen{game: &sim.Game{}}
	count := func(u *game.UnitInfo, label string) int {
		n := 0
		for _, it := range s.popupItemsFor(u) {
			if it.label == label {
				n++
			}
		}
		return n
	}

	refinery := &game.UnitInfo{Type: assets.CommTwr, Flags: game.FlagBuilding | game.FlagStationary, Order: game.OrderPowerOn}
	if got := count(refinery, "Buy Upgrade"); got != 1 {
		t.Errorf("Gold Refinery has %d Buy Upgrade buttons, want exactly 1", got)
	}
	if got := count(refinery, "Upgrade"); got != 0 {
		t.Errorf("Gold Refinery still shows %d generic Upgrade buttons, want 0", got)
	}

	// A normal upgradable unit (Tank has hits/armor) must show NO upgrade button now.
	tank := &game.UnitInfo{Type: assets.Tank, Flags: game.FlagMobileLandUnit | game.FlagSelectable}
	if got := count(tank, "Upgrade") + count(tank, "Buy Upgrade"); got != 0 {
		t.Errorf("a Tank shows %d upgrade buttons, want 0 (the port auto-applies upgrades)", got)
	}
}

// TestPopupBuilderBuildStop checks a builder's lead order tracks its build
// state: Build when idle, Stop while a build order is in progress.
func TestPopupBuilderBuildStop(t *testing.T) {
	s := &Screen{game: &sim.Game{}}
	has := func(u *game.UnitInfo, want string) bool {
		for _, it := range s.popupItemsFor(u) {
			if it.label == want {
				return true
			}
		}
		return false
	}

	con := &game.UnitInfo{Type: assets.Constrct, Flags: game.FlagMobileLandUnit | game.FlagConstructorUnit | game.FlagSelectable}
	if !has(con, "Build") || has(con, "Stop") {
		t.Error("an idle builder should show Build, not Stop")
	}

	product := &game.UnitInfo{Type: assets.MiningSt, Flags: game.FlagBuilding}
	s.game.StartBuild(con, product, 5, 5, 1)
	if !has(con, "Stop") || has(con, "Build") {
		t.Error("a building builder should show Stop, not Build")
	}
}

// TestPopupAttackOrder checks the Attack popup order gating: Attack is registered
// for every MILITARY unit unconditionally (the original's InitMilitary), so it stays visible
// even when the unit is out of ammo/shots (the button only toggles the targeting
// cursor). An unarmed unit never offers it.
func TestPopupAttackOrder(t *testing.T) {
	s := &Screen{game: &sim.Game{}}
	has := func(u *game.UnitInfo, want string) bool {
		for _, it := range s.popupItemsFor(u) {
			if it.label == want {
				return true
			}
		}
		return false
	}

	// An armed tank with ammo + shots: Attack present.
	tank := &game.UnitInfo{Type: assets.Tank, Flags: game.FlagMobileLandUnit | game.FlagSelectable, Ammo: 6, Shots: 1}
	if !has(tank, "Attack") {
		t.Error("an armed unit with ammo should offer Attack")
	}
	// Same tank out of ammo AND shots: Attack STILL present (military button is
	// unconditional; the fire path raises the "no shots" banner). This is finding F.
	tank.Ammo, tank.Shots = 0, 0
	if !has(tank, "Attack") {
		t.Error("a military unit out of ammo/shots must STILL offer Attack (klei InitMilitary is unconditional)")
	}
	// An unarmed unit (surveyor) never offers Attack, even with ammo set.
	surv := &game.UnitInfo{Type: assets.Surveyor, Flags: game.FlagMobileLandUnit | game.FlagSelectable, Ammo: 6, Shots: 1}
	if has(surv, "Attack") {
		t.Error("an unarmed unit must not offer Attack")
	}

	// armAttack arms the target pick and selects the attacker.
	tank.Ammo = 6
	s.armAttack(tank)
	if !s.orderInput.attackArmed || s.selected != tank {
		t.Error("armAttack should set attackArmed and select the attacker")
	}
	s.cancelAttack()
	if s.orderInput.attackArmed {
		t.Error("cancelAttack should disarm")
	}
}

// TestPopupParity verifies the button for button parity pass (SPEC-popup-menu parity):
// each representative type yields the same ordered button set as its original
// UnitsManager_Popup_Init* callback + InitCommons. Units carry nil Values so the
// ammo gated Manual is skipped, keeping the sets to the structural buttons. Tags are
// the vanilla order tags (popupItem.tex).
func TestPopupParity(t *testing.T) {
	s := &Screen{game: &sim.Game{}}
	tags := func(u *game.UnitInfo) []string {
		var out []string
		for _, it := range s.popupItemsFor(u) {
			out = append(out, it.tex)
		}
		return out
	}
	eq := func(got, want []string) bool {
		if len(got) != len(want) {
			return false
		}
		for i := range got {
			if got[i] != want[i] {
				return false
			}
		}
		return true
	}

	// turretValues gives a defense structure a real attack value (so isDefenseBuilding
	// recognizes it and offers Sentry) while keeping base ammo 0 (so the ammo gated
	// Manual toggle stays off, matching the vanilla turret popup).
	turretValues := func() *game.UnitValues {
		v := &game.UnitValues{}
		v.Set(game.AttribAttack, 10)
		return v
	}

	cases := []struct {
		name string
		unit *game.UnitInfo
		want []string
	}{
		{
			// InitBuilders (Build) + commons; unarmed, stores raw. No UPGRD, no MANUAL,
			// no power. Matches the vanilla 5-button Constructor capture.
			name: "constructor",
			unit: &game.UnitInfo{Type: assets.Constrct, Flags: game.FlagMobileLandUnit | game.FlagConstructorUnit | game.FlagSelectable},
			want: []string{"BUILD", "XFER", "ENTER", "SNTRY", "DONE"},
		},
		{
			name: "engineer",
			unit: &game.UnitInfo{Type: assets.Engineer, Flags: game.FlagMobileLandUnit | game.FlagConstructorUnit | game.FlagSelectable},
			want: []string{"BUILD", "XFER", "ENTER", "SNTRY", "DONE"},
		},
		{
			// InitMilitary (Attack, unconditional) + commons; carries nothing.
			name: "tank",
			unit: &game.UnitInfo{Type: assets.Tank, Flags: game.FlagMobileLandUnit | game.FlagSelectable},
			want: []string{"ATTACK", "ENTER", "SNTRY", "DONE"},
		},
		{
			// A stationary military unit: Attack (unconditional) + defense Sentry + Remove.
			name: "gun turret",
			unit: &game.UnitInfo{Type: assets.GunTurrt, Flags: game.FlagStationary | game.FlagSelectable, Values: turretValues()},
			want: []string{"ATTACK", "SNTRY", "REMVE"},
		},
		{
			// InitFactories: Build (empty queue) + commons. A factory draws its build
			// material straight from the complex pool, not a CargoType bin, so the
			// original's InitCommons cargo gate does NOT offer it X-Fer (user 2026-07-26
			// live check); Remove closes it. NO power toggle (finding C) - asserted
			// separately below.
			name: "factory",
			unit: &game.UnitInfo{Type: assets.LandPlt, Flags: game.FlagBuilding | game.FlagStationary, Order: game.OrderPowerOn},
			want: []string{"BUILD", "REMVE"},
		},
	}

	for _, tc := range cases {
		if got := tags(tc.unit); !eq(got, tc.want) {
			t.Errorf("%s popup = %v, want %v", tc.name, got, tc.want)
		}
	}

	// A military unit keeps Attack even with 0 shots (finding F); a factory never
	// shows a power label (finding C).
	turret := &game.UnitInfo{Type: assets.GunTurrt, Flags: game.FlagStationary | game.FlagSelectable, Values: turretValues(), Shots: 0}
	if got := tags(turret); len(got) == 0 || got[0] != "ATTACK" {
		t.Errorf("a 0-shot turret must still lead with ATTACK, got %v", got)
	}
	factory := &game.UnitInfo{Type: assets.LandPlt, Flags: game.FlagBuilding | game.FlagStationary, Order: game.OrderPowerOn}
	for _, it := range s.popupItemsFor(factory) {
		if it.label == "Power On" || it.label == "Power Off" {
			t.Errorf("a factory popup must not show a power toggle, got %q", it.label)
		}
	}
}

// TestPopupFactoryStartStopLabels verifies the vanilla factory toggle naming: a running
// factory shows "Stop" (the original's bcf7 / StopBuild), a player halted one shows
// "Start" (the original's cced / StartBuild) - never a "Halt"/"Resume" rename. Both
// keep the queue; only one shows at a time. Nil lang -> the capitalized fallbacks.
func TestPopupFactoryStartStopLabels(t *testing.T) {
	s := &Screen{game: &sim.Game{}}
	labels := func(u *game.UnitInfo) []string {
		var out []string
		for _, it := range s.popupItemsFor(u) {
			out = append(out, it.label)
		}
		return out
	}
	has := func(ls []string, want string) bool {
		for _, l := range ls {
			if l == want {
				return true
			}
		}
		return false
	}

	// Running factory (queue + rate): Stop, never Start/Halt/Resume.
	running := &game.UnitInfo{Type: assets.LandPlt, Flags: game.FlagBuilding | game.FlagStationary, Order: game.OrderPowerOn, BuildQueue: []assets.UnitType{assets.Tank}, BuildRate: 1}
	ls := labels(running)
	if !has(ls, "Stop") || has(ls, "Start") || has(ls, "Halt") || has(ls, "Resume") {
		t.Errorf("a running factory should show Stop (vanilla), got %v", ls)
	}
	// Player halted factory (queue but rate 0 -> ProdPaused): Start, never Stop/Halt/Resume.
	halted := &game.UnitInfo{Type: assets.LandPlt, Flags: game.FlagBuilding | game.FlagStationary, Order: game.OrderPowerOn, BuildQueue: []assets.UnitType{assets.Tank}, BuildRate: 0}
	hs := labels(halted)
	if !has(hs, "Start") || has(hs, "Stop") || has(hs, "Halt") || has(hs, "Resume") {
		t.Errorf("a halted factory should show Start (vanilla), got %v", hs)
	}
}

// TestPopupCommonsBusyOrderGate checks finding E: a builder mid build (ORDER_BUILD)
// drops X-Fer and Enter from the commons tail, matching the original's InitCommons.
func TestPopupCommonsBusyOrderGate(t *testing.T) {
	s := &Screen{game: &sim.Game{}}
	has := func(u *game.UnitInfo, want string) bool {
		for _, it := range s.popupItemsFor(u) {
			if it.tex == want {
				return true
			}
		}
		return false
	}

	con := &game.UnitInfo{Type: assets.Constrct, Flags: game.FlagMobileLandUnit | game.FlagConstructorUnit | game.FlagSelectable}
	// Idle: X-Fer + Enter present.
	if !has(con, "XFER") || !has(con, "ENTER") {
		t.Error("an idle constructor should offer X-Fer and Enter")
	}
	// Mid build: both suppressed (the builder branch shows Stop instead).
	con.Order = game.OrderBuild
	if has(con, "XFER") || has(con, "ENTER") {
		t.Error("a mid-build constructor must not offer X-Fer or Enter (klei InitCommons order gate)")
	}
}

// TestPopupInfiltrateOrder checks the commando's Disable/Steal popup orders: a
// commando with a shot offers both; any other unit never does; a commando out of
// shots offers neither. armInfiltrate arms the right mode.
func TestPopupInfiltrateOrder(t *testing.T) {
	s := &Screen{game: &sim.Game{}}
	labels := func(u *game.UnitInfo) []string {
		var out []string
		for _, it := range s.popupItemsFor(u) {
			out = append(out, it.label)
		}
		return out
	}
	has := func(ls []string, want string) bool {
		for _, l := range ls {
			if l == want {
				return true
			}
		}
		return false
	}

	commando := &game.UnitInfo{Type: assets.Commando, Flags: game.FlagMobileLandUnit | game.FlagSelectable, Shots: 1}
	ls := labels(commando)
	if !has(ls, "Disable") || !has(ls, "Steal") {
		t.Errorf("a commando with a shot should offer Disable + Steal, got %v", ls)
	}
	// A tank (not a commando) never offers them.
	tank := &game.UnitInfo{Type: assets.Tank, Flags: game.FlagMobileLandUnit | game.FlagSelectable, Shots: 1}
	if tls := labels(tank); has(tls, "Disable") || has(tls, "Steal") {
		t.Errorf("a non-commando must not offer Disable/Steal, got %v", tls)
	}
	// Out of shots: neither.
	commando.Shots = 0
	if cls := labels(commando); has(cls, "Disable") || has(cls, "Steal") {
		t.Errorf("an out-of-shots commando must not offer Disable/Steal, got %v", cls)
	}

	// armInfiltrate arms the chosen mode and selects the commando.
	commando.Shots = 1
	s.armInfiltrate(commando, true)
	if !s.orderInput.infiltrateArmed || !s.orderInput.infiltrateSteal || s.selected != commando {
		t.Error("armInfiltrate(steal) should arm steal mode and select the commando")
	}
	s.cancelInfiltrate()
	if s.orderInput.infiltrateArmed {
		t.Error("cancelInfiltrate should disarm")
	}
}

// TestPopupReloadOrder checks the Supply Truck's Reload popup order: a truck with
// material offers it; an empty truck and any other unit never do. armReload arms the
// pick and selects the supplier.
func TestPopupReloadOrder(t *testing.T) {
	s := &Screen{game: &sim.Game{}}
	has := func(u *game.UnitInfo, want string) bool {
		for _, it := range s.popupItemsFor(u) {
			if it.label == want {
				return true
			}
		}
		return false
	}

	truck := &game.UnitInfo{Type: assets.SplyTrck, Flags: game.FlagMobileLandUnit | game.FlagSelectable, Storage: 5}
	if !has(truck, "Reload") {
		t.Error("a Supply Truck with material should offer Reload")
	}
	// Empty truck: no Reload.
	truck.Storage = 0
	if has(truck, "Reload") {
		t.Error("an empty Supply Truck must not offer Reload")
	}
	// A tank (not a supply truck) never offers Reload.
	tank := &game.UnitInfo{Type: assets.Tank, Flags: game.FlagMobileLandUnit | game.FlagSelectable, Storage: 5}
	if has(tank, "Reload") {
		t.Error("a non-reloader must not offer Reload")
	}

	// armReload arms the pick and selects the supplier.
	truck.Storage = 5
	s.armReload(truck)
	if !s.orderInput.reloadArmed || s.selected != truck {
		t.Error("armReload should set reloadArmed and select the supplier")
	}
	s.cancelReload()
	if s.orderInput.reloadArmed {
		t.Error("cancelReload should disarm")
	}
}

// TestPopupRepairOrder checks the mobile Repair unit gains a Repair popup item only
// when it carries material, and that armRepair/cancelRepair arm and disarm the pick.
func TestPopupRepairOrder(t *testing.T) {
	s := &Screen{game: &sim.Game{}}
	has := func(u *game.UnitInfo, want string) bool {
		for _, it := range s.popupItemsFor(u) {
			if it.label == want {
				return true
			}
		}
		return false
	}

	rep := &game.UnitInfo{Type: assets.Repair, Flags: game.FlagMobileLandUnit | game.FlagSelectable, Storage: 5}
	if !has(rep, "Repair") {
		t.Error("a Repair unit with material should offer Repair")
	}
	// Empty Repair unit: no Repair.
	rep.Storage = 0
	if has(rep, "Repair") {
		t.Error("an empty Repair unit must not offer Repair")
	}
	// A tank (not a repairer) never offers Repair.
	tank := &game.UnitInfo{Type: assets.Tank, Flags: game.FlagMobileLandUnit | game.FlagSelectable, Storage: 5}
	if has(tank, "Repair") {
		t.Error("a non-repairer must not offer Repair")
	}

	// armRepair arms the pick and selects the repairer.
	rep.Storage = 5
	s.armRepair(rep)
	if !s.orderInput.repairArmed || s.selected != rep {
		t.Error("armRepair should set repairArmed and select the repairer")
	}
	s.cancelRepair()
	if s.orderInput.repairArmed {
		t.Error("cancelRepair should disarm")
	}
}

// TestCommandoRankLabel checks the commando veterancy rank label across the
// experience tiers (with nil lang -> the fallback rank names) and that any
// other unit yields no label. Raw experience values map through GetExperience
// (floor(sqrt(raw*10))-2) to ranks 0/1/3/6/11.
func TestCommandoRankLabel(t *testing.T) {
	s := &Screen{game: &sim.Game{}}
	cmd := func(exp int) *game.UnitInfo {
		return &game.UnitInfo{Type: assets.Commando, Flags: game.FlagMobileLandUnit | game.FlagSelectable, Experience: exp}
	}
	cases := []struct {
		exp  int
		want string
	}{
		{0, "Rookie"},       // rank 0
		{1, "Average (+1)"}, // rank 1
		{3, "Veteran (+3)"}, // rank 3
		{7, "Crack (+6)"},   // rank 6
		{17, "Elite (+11)"}, // rank 11
	}
	for _, c := range cases {
		if got := s.commandoRankLabel(cmd(c.exp)); got != c.want {
			t.Errorf("commandoRankLabel(exp=%d) = %q, want %q", c.exp, got, c.want)
		}
	}
	// A tank (not a commando) has no rank label.
	tank := &game.UnitInfo{Type: assets.Tank, Flags: game.FlagMobileLandUnit, Experience: 17}
	if got := s.commandoRankLabel(tank); got != "" {
		t.Errorf("commandoRankLabel(tank) = %q, want \"\"", got)
	}
}

// TestMineBlastSpawnsEffect checks that a mine detonation (sim) drains into a
// visible explosion via tickMineBlasts - the missing effect/sound on a mine hit.
func TestMineBlastSpawnsEffect(t *testing.T) {
	const w, h = 8, 8
	mgr := units.NewManager()
	grid := &access.Grid{W: w, H: h, Surf: buildExitSurf{}, Units: mgr.Cells}
	p := &game.StartParams{}
	p.Teams[0].Type = game.TeamPlayer
	g := sim.NewWithGrid(game.NewGameState(p), mgr, grid, w, h)
	s := &Screen{game: g} // effectsOff defaults false -> explosions are recorded

	layer := game.NewUnitInfo(assets.MineLayr, 0, 1, 0)
	layer.Flags = game.FlagMobileLandUnit | game.FlagSelectable
	layer.Values = &game.UnitValues{}
	layer.Values.Set(game.AttribStorage, 1)
	layer.InitState()
	mgr.Deploy(layer, 4, 4)
	if !g.LayMine(layer) {
		t.Fatal("LayMine failed")
	}

	enemy := game.NewUnitInfo(assets.Tank, 1, 50, 0)
	enemy.Flags = game.FlagMobileLandUnit | game.FlagSelectable
	enemy.Values = &game.UnitValues{}
	enemy.Values.Set(game.AttribHits, 30)
	enemy.Values.Set(game.AttribArmor, 4)
	enemy.Hits = 30
	mgr.Deploy(enemy, 4, 4)

	g.Tick() // detonateMines records a blast
	before := len(s.effects.explosions)
	s.tickMineBlasts()
	if len(s.effects.explosions) <= before {
		t.Error("a mine detonation spawned no explosion effect")
	}
}

// TestProjectileImpactSpawnsEffect: a bomber's rocket, on arrival, spawns the
// explosion effect over the destroyed target (mirrors TestMineBlastSpawnsEffect).
func TestProjectileImpactSpawnsEffect(t *testing.T) {
	const w, h = 16, 8
	mgr := units.NewManager()
	grid := &access.Grid{W: w, H: h, Surf: buildExitSurf{}, Units: mgr.Cells}
	p := &game.StartParams{}
	p.Teams[0].Type = game.TeamPlayer
	g := sim.NewWithGrid(game.NewGameState(p), mgr, grid, w, h)
	s := &Screen{game: g} // effectsOff defaults false; sfxFn nil -> sounds do nothing

	bomber := game.NewUnitInfo(assets.Bomber, 0, 1, 0)
	bomber.Flags = game.FlagMobileAirUnit | game.FlagHovering | game.FlagSelectable
	bomber.Values = &game.UnitValues{}
	bomber.Values.Set(game.AttribAttack, 20)
	bomber.Values.Set(game.AttribRange, 8)
	bomber.Values.Set(game.AttribRounds, 1)
	bomber.Values.Set(game.AttribAmmo, 4)
	bomber.Values.Set(game.AttribHits, 16)
	bomber.InitState()
	mgr.Deploy(bomber, 2, 4)

	enemy := game.NewUnitInfo(assets.Tank, 1, 2, 0)
	enemy.Flags = game.FlagMobileLandUnit | game.FlagSelectable
	enemy.Values = &game.UnitValues{}
	enemy.Values.Set(game.AttribHits, 6)
	enemy.Hits = 6
	mgr.Deploy(enemy, 7, 4)

	if _, ok := g.Attack(bomber, 7, 4); !ok {
		t.Fatal("bomber attack not ok")
	}
	for i := 0; i < 1000 && len(g.ActiveProjectiles()) > 0; i++ {
		g.Tick()
	}
	before := len(s.effects.explosions)
	s.tickProjectiles()
	if len(s.effects.explosions) <= before {
		t.Error("a missile impact spawned no explosion effect")
	}
}

// TestMissileHitMessageOnImpact: a player missile reports its hit when it lands, with the real damage, and
// not at launch (it used to read "hit for 0" while the missile was still in flight).
func TestMissileHitMessageOnImpact(t *testing.T) {
	const w, h = 16, 8
	mgr := units.NewManager()
	grid := &access.Grid{W: w, H: h, Surf: buildExitSurf{}, Units: mgr.Cells}
	p := &game.StartParams{}
	p.Teams[0].Type = game.TeamPlayer
	g := sim.NewWithGrid(game.NewGameState(p), mgr, grid, w, h)
	s := &Screen{game: g}

	bomber := game.NewUnitInfo(assets.Bomber, 0, 1, 0)
	bomber.Flags = game.FlagMobileAirUnit | game.FlagHovering | game.FlagSelectable
	bomber.Values = &game.UnitValues{}
	bomber.Values.Set(game.AttribAttack, 20)
	bomber.Values.Set(game.AttribRange, 8)
	bomber.Values.Set(game.AttribRounds, 1)
	bomber.Values.Set(game.AttribAmmo, 4)
	bomber.Values.Set(game.AttribHits, 16)
	bomber.InitState()
	mgr.Deploy(bomber, 2, 4)

	enemy := game.NewUnitInfo(assets.Tank, 1, 2, 0)
	enemy.Flags = game.FlagMobileLandUnit | game.FlagSelectable
	enemy.Values = &game.UnitValues{}
	enemy.Values.Set(game.AttribHits, 30)
	enemy.Hits = 30
	mgr.Deploy(enemy, 7, 4)

	hitLines := func() []string {
		var out []string
		for _, e := range s.banner.msgLog {
			if strings.Contains(e.text, "hit for") {
				out = append(out, e.text)
			}
		}
		return out
	}

	s.attackWith(bomber, 7, 4, false)
	if len(g.ActiveProjectiles()) == 0 {
		t.Fatal("the bomber launched no missile")
	}
	if got := hitLines(); len(got) != 0 {
		t.Fatalf("hit message at launch: %q", got)
	}

	for i := 0; i < 1000 && len(g.ActiveProjectiles()) > 0; i++ {
		g.Tick()
	}
	s.tickProjectiles()

	got := hitLines()
	if len(got) != 1 || !strings.Contains(got[0], "hit for 20 (10/30") {
		t.Errorf("hit message on impact = %q, want one line with \"hit for 20 (10/30\"", got)
	}
}

// TestCasualtySpawnsExplosion: a unit killed by an instant attack (the path the AI
// uses) is blasted by tickCasualties, not silently removed.
func TestCasualtySpawnsExplosion(t *testing.T) {
	const w, h = 16, 8
	mgr := units.NewManager()
	grid := &access.Grid{W: w, H: h, Surf: buildExitSurf{}, Units: mgr.Cells}
	p := &game.StartParams{}
	p.Teams[0].Type = game.TeamPlayer
	g := sim.NewWithGrid(game.NewGameState(p), mgr, grid, w, h)
	s := &Screen{game: g}

	gun := game.NewUnitInfo(assets.Tank, 1, 1, 0) // a team-1 (AI) gun
	gun.Flags = game.FlagMobileLandUnit | game.FlagSelectable | game.FlagTurretSprite
	gun.Values = &game.UnitValues{}
	gun.Values.Set(game.AttribAttack, 60)
	gun.Values.Set(game.AttribRange, 5)
	gun.Values.Set(game.AttribRounds, 1)
	gun.Values.Set(game.AttribAmmo, 4)
	gun.Values.Set(game.AttribHits, 30)
	gun.InitState()
	mgr.Deploy(gun, 2, 4)

	victim := game.NewUnitInfo(assets.Tank, 0, 2, 0) // the player's unit
	victim.Flags = game.FlagMobileLandUnit | game.FlagSelectable
	victim.Values = &game.UnitValues{}
	victim.Values.Set(game.AttribHits, 6)
	victim.Hits = 6
	mgr.Deploy(victim, 4, 4)

	if _, ok := g.Attack(gun, 4, 4); !ok {
		t.Fatal("attack not ok")
	}
	before := len(s.effects.explosions)
	s.tickCasualties()
	if len(s.effects.explosions) <= before {
		t.Error("an instant-attack kill spawned no explosion effect")
	}
}

// TestPopupMineLayer checks a mine layer's popup: a loaded layer on an empty cell
// offers Place Mine; once a mine is on its cell it offers Remove Mine instead; any
// other unit offers neither. Needs a real sim.Game (CanLayMine reads the cell hash).
func TestPopupMineLayer(t *testing.T) {
	const w, h = 8, 8
	mgr := units.NewManager()
	grid := &access.Grid{W: w, H: h, Surf: buildExitSurf{}, Units: mgr.Cells}
	p := &game.StartParams{}
	p.Teams[0].Type = game.TeamPlayer
	g := sim.NewWithGrid(game.NewGameState(p), mgr, grid, w, h)
	s := &Screen{game: g}

	layer := game.NewUnitInfo(assets.MineLayr, 0, 1, 0)
	layer.Flags = game.FlagMobileLandUnit | game.FlagSelectable
	layer.Values = &game.UnitValues{}
	layer.Values.Set(game.AttribStorage, 2)
	layer.InitState() // Storage = 2 (loaded)
	mgr.Deploy(layer, 4, 4)

	has := func(u *game.UnitInfo, want string) bool {
		for _, it := range s.popupItemsFor(u) {
			if it.label == want {
				return true
			}
		}
		return false
	}

	// Loaded layer on an empty cell: Place Mine, not Remove Mine.
	if !has(layer, "Place Mine") {
		t.Error("a loaded mine layer should offer Place Mine")
	}
	if has(layer, "Remove Mine") {
		t.Error("no mine on the cell yet -> should not offer Remove Mine")
	}

	// After laying, the cell has the layer's mine: Remove Mine, not Place Mine.
	if !g.LayMine(layer) {
		t.Fatal("LayMine returned false")
	}
	if !has(layer, "Remove Mine") {
		t.Error("with its own mine on the cell -> should offer Remove Mine")
	}
	if has(layer, "Place Mine") {
		t.Error("a mine already on the cell -> should not offer Place Mine")
	}

	// A tank (not a mine layer) offers neither.
	tank := &game.UnitInfo{Type: assets.Tank, Flags: game.FlagMobileLandUnit | game.FlagSelectable}
	if has(tank, "Place Mine") || has(tank, "Remove Mine") {
		t.Error("a non-layer must not offer mine orders")
	}
}

// TestPopupBulldozerClear: a bulldozer parked on a rubble pile offers Remove
// (clear); while clearing it offers Stop; off a pile it offers neither.
func TestPopupBulldozerClear(t *testing.T) {
	const w, h = 8, 8
	mgr := units.NewManager()
	grid := &access.Grid{W: w, H: h, Surf: buildExitSurf{}, Units: mgr.Cells}
	p := &game.StartParams{}
	p.Teams[0].Type = game.TeamPlayer
	g := sim.NewWithGrid(game.NewGameState(p), mgr, grid, w, h)
	s := &Screen{game: g}

	has := func(u *game.UnitInfo, want string) bool {
		for _, it := range s.popupItemsFor(u) {
			if it.label == want {
				return true
			}
		}
		return false
	}

	// Leave a small pile at (4,4) by destroying a unit there, then park a dozer on it.
	wreck := game.NewUnitInfo(assets.Tank, 0, 1, 0)
	wreck.Flags = game.FlagMobileLandUnit | game.FlagSelectable
	mgr.Deploy(wreck, 4, 4)
	g.DestroyUnit(wreck)

	dozer := game.NewUnitInfo(assets.Bulldozr, 0, 2, 0)
	dozer.Flags = game.FlagMobileLandUnit | game.FlagSelectable
	dozer.Values = &game.UnitValues{}
	mgr.Deploy(dozer, 4, 4)

	// On a pile, not yet clearing: Remove, not Stop.
	if !has(dozer, "Remove") {
		t.Error("a bulldozer on a rubble pile should offer Remove")
	}
	if has(dozer, "Stop") {
		t.Error("an idle bulldozer should not offer Stop")
	}

	// While clearing: Stop, not Remove.
	if !g.StartClearRubble(dozer) {
		t.Fatal("StartClearRubble returned false")
	}
	if !has(dozer, "Stop") {
		t.Error("a clearing bulldozer should offer Stop")
	}
	if has(dozer, "Remove") {
		t.Error("a clearing bulldozer should not offer Remove again")
	}

	// A bulldozer off any pile offers neither.
	g.StopClearRubble(dozer)
	dry := game.NewUnitInfo(assets.Bulldozr, 0, 3, 0)
	dry.Flags = game.FlagMobileLandUnit | game.FlagSelectable
	dry.Values = &game.UnitValues{}
	mgr.Deploy(dry, 1, 1)
	if has(dry, "Remove") || has(dry, "Stop") {
		t.Error("a bulldozer not on a pile must offer neither Remove nor Stop")
	}
}

// TestPopupAirNoLandTakeOff: Land and Take Off are NOT popup items (vanilla drives
// both with movement - click a pad to land, click a land cell to rise + move). The air
// unit popup, flying or grounded, offers NEITHER.
func TestPopupAirNoLandTakeOff(t *testing.T) {
	const w, h = 8, 8
	mgr := units.NewManager()
	grid := &access.Grid{W: w, H: h, Surf: buildExitSurf{}, Units: mgr.Cells}
	p := &game.StartParams{}
	p.Teams[0].Type = game.TeamPlayer
	g := sim.NewWithGrid(game.NewGameState(p), mgr, grid, w, h)
	s := &Screen{game: g}

	has := func(u *game.UnitInfo, want string) bool {
		for _, it := range s.popupItemsFor(u) {
			if it.label == want {
				return true
			}
		}
		return false
	}

	// A landing pad under the flying bomber - even over a pad, no Land popup item.
	landpad := game.NewUnitInfo(assets.LandPad, 0, 9, 0)
	landpad.Flags = game.FlagStationary
	landpad.Hits = 1
	mgr.Deploy(landpad, 4, 4)

	flying := game.NewUnitInfo(assets.Bomber, 0, 1, 0)
	flying.Flags = game.FlagMobileAirUnit | game.FlagHovering | game.FlagSelectable
	mgr.Deploy(flying, 4, 4)
	if has(flying, "Land") || has(flying, "Take Off") {
		t.Error("a flying air unit must not offer Land/Take Off popup items (movement-driven)")
	}

	grounded := game.NewUnitInfo(assets.Bomber, 0, 2, 0)
	grounded.Flags = game.FlagMobileAirUnit | game.FlagSelectable // no Hovering = landed
	mgr.Deploy(grounded, 6, 6)
	if has(grounded, "Take Off") || has(grounded, "Land") {
		t.Error("a grounded air unit must not offer Take Off/Land popup items (it rises on a move)")
	}
}

// TestSentryOnlyWhileAwaiting verifies that Sentry appears only for a mobile unit waiting for orders or already
// on guard. Builders in ORDER_BUILD, a clearing Bulldozer, and a Commando do not receive it; mobile units keep Done.
func TestSentryOnlyWhileAwaiting(t *testing.T) {
	s := &Screen{game: &sim.Game{}}
	has := func(u *game.UnitInfo, label string) bool {
		for _, it := range s.popupItemsFor(u) {
			if it.label == label {
				return true
			}
		}
		return false
	}
	unit := func(tp assets.UnitType, order game.Order, state game.OrderState) *game.UnitInfo {
		u := game.NewUnitInfo(tp, 0, 1, 0)
		u.Flags = game.FlagMobileLandUnit | game.FlagSelectable
		if tp == assets.Constrct || tp == assets.Engineer {
			u.Flags |= game.FlagConstructorUnit
		}
		u.Values = &game.UnitValues{}
		u.Order, u.State = order, state
		return u
	}

	for _, tc := range []struct {
		name   string
		u      *game.UnitInfo
		sentry bool
	}{
		{"idle Constructor", unit(assets.Constrct, game.OrderAwait, game.OrderStateExecutingOrder), true},
		{"parked Constructor", unit(assets.Constrct, game.OrderBuild, game.OrderStateUnitReady), false},
		{"building Constructor", unit(assets.Constrct, game.OrderBuild, game.OrderStateBuildInProgress), false},
		{"idle Engineer", unit(assets.Engineer, game.OrderAwait, game.OrderStateExecutingOrder), true},
		{"parked Engineer", unit(assets.Engineer, game.OrderBuild, game.OrderStateUnitReady), false},
		{"Tank on guard", unit(assets.Tank, game.OrderSentry, game.OrderStateExecutingOrder), true},
		{"clearing Bulldozer", unit(assets.Bulldozr, game.OrderClear, game.OrderStateExecutingOrder), false},
		{"Commando", unit(assets.Commando, game.OrderAwait, game.OrderStateExecutingOrder), false},
	} {
		if got := has(tc.u, "Sentry"); got != tc.sentry {
			t.Errorf("%s: Sentry offered = %v, want %v", tc.name, got, tc.sentry)
		}
		if !has(tc.u, "Done") {
			t.Errorf("%s: no Done button", tc.name)
		}
	}
}
