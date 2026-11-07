// complex.go : pools resources and power across connected buildings.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
//
// C++ ported : complex.cpp - Complex / ComplexManager
package economy

import (
	"github.com/suglasp/max-go/internal/assets"
	"github.com/suglasp/max-go/internal/game"
)

// Complex is a connected network of a team's stationary buildings that pool
// material/fuel/gold plus power and worker balances, drawn on each turn. Unlike
// the C++ Complex (which reaches into the global stationary list) membership is
// explicit (Members), keeping the package self contained and testable.
// C++: class Complex (complex.hpp / complex.cpp).
type Complex struct {
	ID        int
	Team      int
	Material  int
	Fuel      int
	Gold      int
	Buildings int

	Members []*game.UnitInfo

	lastShed     []*game.UnitInfo // consumers shed/held for POWER last reconcile (banner); transient, not serialized
	lastLifeShed []*game.UnitInfo // consumers powered off for WORKERS last reconcile (banner); transient, not serialized
}

// NewComplex returns an empty complex with the given id.
func NewComplex(id int) *Complex {
	return &Complex{ID: id}
}

// Grow adds a building to the complex.
//
// C++: Complex::Grow (complex.cpp): here it also records membership.
func (c *Complex) Grow(u *game.UnitInfo) {
	c.Members = append(c.Members, u)
	c.Buildings++
}

// Shrink removes a building from the complex, returning whether the complex is
// now empty (the caller should then dispose of it).
//
// C++: Complex::Shrink (complex.cpp).
func (c *Complex) Shrink(u *game.UnitInfo) (empty bool) {
	kept := c.Members[:0]
	for _, m := range c.Members {
		if m != u {
			kept = append(kept, m)
		}
	}
	c.Members = kept
	c.Buildings--
	return c.Buildings == 0
}

// active reports whether a member building counts toward production this turn
// (not powered off, disabled or idle).
//
// C++: the order != POWER_OFF/DISABLE/IDLE guard in Complex::GetCargo*.
func active(u *game.UnitInfo) bool {
	return u.Order != game.OrderPowerOff && u.Order != game.OrderDisable && u.Order != game.OrderIdle
}

// IsActive reports whether a building is operational (powered on, not off,
// disabled or idle): the same predicate the complex uses to count production.
// Used by the UI to decide e.g. whether a selected building hums its running
// sound.
func IsActive(u *game.UnitInfo) bool {
	return active(u)
}

// CargoMining reports the resources the complex's mining stations currently
// extract (materials) and the deposits still available to them (capacity).
//
// C++: Complex::GetCargoMining (complex.cpp).
func (c *Complex) CargoMining(rmap *ResourceMap) (materials, capacity Cargo) {
	for _, u := range c.Members {
		if u.Type != assets.MiningSt || !active(u) {
			continue
		}
		materials.Gold += u.GoldMining
		materials.Raw += u.RawMining
		materials.Fuel += u.FuelMining
		materials.FreeCapacity += 16 - (u.GoldMining + u.RawMining + u.FuelMining)

		if rmap != nil {
			raw, gold, fuel := rmap.ResourcesInArea(u.GridX, u.GridY, 1, 16, u.Team, true)
			capacity.Raw += raw
			capacity.Fuel += fuel
			capacity.Gold += gold
		}
	}
	return
}

// CargoMinable reports the complex's net production capacity headroom; the
// extra each member could still produce beyond its current mining draw.
//
// C++: Complex::GetCargoMinable (complex.cpp).
func (c *Complex) CargoMinable(difficultyFactor int) (capacity Cargo) {
	for _, u := range c.Members {
		if !active(u) {
			continue
		}
		cargo := NetProduction(u, difficultyFactor)
		if u.Type == assets.MiningSt {
			cargo.Gold -= u.GoldMining
			cargo.Raw -= u.RawMining
			cargo.Fuel -= u.FuelMining
		}
		capacity.Gold -= cargo.Gold
		capacity.Raw -= cargo.Raw
		capacity.Fuel -= cargo.Fuel
	}
	return
}

// NetTurn reports the complex's net per turn production (mining minus
// consumption) per cargo: the "+N / turn" figure the allocation screen shows.
//
// C++: the production - usage net in AllocMenu::DrawCargoBars (allocmenu.cpp).
func (c *Complex) NetTurn(difficultyFactor int) (net Cargo) {
	for _, u := range c.Members {
		if active(u) {
			net.Add(NetProduction(u, difficultyFactor))
		}
	}
	return
}

// CargoInfo reports the total stored inventory and storage capacity across all
// member buildings.
//
// C++: Complex::GetCargoInfo (complex.cpp).
func (c *Complex) CargoInfo() (materials, capacity Cargo) {
	for _, u := range c.Members {
		materials.Add(Inventory(u))
		capacity.Add(Capacity(u))
	}
	return
}

// transferCargo moves up to *amount into (or, if negative, out of) one unit's
// storage, respecting its capacity; *amount is updated with the remainder.
//
// C++: Complex::TransferCargo (complex.cpp).
func transferCargo(u *game.UnitInfo, amount *int) {
	if *amount >= 0 {
		free := 0
		if u.Values != nil {
			free = u.Values.Get(game.AttribStorage) - u.Storage
		}
		if *amount <= free {
			u.Storage += *amount
			*amount = 0
		} else {
			u.Storage += free
			*amount -= free
		}
	} else if -*amount <= u.Storage {
		u.Storage += *amount
		*amount = 0
	} else {
		*amount += u.Storage
		u.Storage = 0
	}
}

// Transfer adds raw/fuel/gold to the complex store and distributes the amounts
// into the matching cargo type member buildings until consumed or full.
//
// C++: Complex::Transfer (complex.cpp).
func (c *Complex) Transfer(raw, fuel, gold int) {
	c.Gold += gold
	c.Fuel += fuel
	c.Material += raw
	c.CapStores() // a delivery into a full base spills, exactly like a full turn of mining
	for _, u := range c.Members {
		if raw == 0 && fuel == 0 && gold == 0 {
			break
		}
		switch CargoTypeOf(u.Type) {
		case CargoRaw:
			transferCargo(u, &raw)
		case CargoFuel:
			transferCargo(u, &fuel)
		case CargoGold:
			transferCargo(u, &gold)
		}
	}
}

// BalanceTurn applies one turn of the complex economy: active members' net
// material/fuel/gold accumulate into the shared stores; power and workers are
// recomputed as the live surplus/deficit. Returns the summed net production.
// difficultyFactor scales mining (4 human/easy, 5/6 master/god).
// C++: per turn sum of Cargo_GetNetProduction over the members (production_manager
// turn end cycle), stores grow as Cargo_UpdateResourceLevels.
func (c *Complex) BalanceTurn(difficultyFactor int) Cargo {
	// Reconcile power first: optimize generators, shed lowest priority consumers
	// until demand fits, leaving an active set the running generators can cover.
	c.OptimizeProduction()

	// Sum net production of active members that are not held. A held factory
	// (PowerStarved) produces/consumes nothing; a shed aux/mine is powered OFF, so
	// not active. Generators always burn fuel into the net - they never gate on fuel
	// stock, allowing a drained base to keep mining fuel and recover from zero.
	var net Cargo
	for _, u := range c.Members {
		if active(u) && !u.PowerStarved {
			net.Add(NetProduction(u, difficultyFactor))
		}
	}

	// Apply the net, clamping stores at zero (a fuel deficit empties the tank but
	// never goes negative; more fuel mining clears the shortage).
	c.Material = max(0, c.Material+net.Raw)
	c.Fuel = max(0, c.Fuel+net.Fuel)
	c.Gold = max(0, c.Gold+net.Gold)
	c.CapStores()    // a base cannot hold more than its storage: the rest spills
	c.redistribute() // mirror the pooled stores into the member buildings' cargo bars
	return net
}

// RefineGold returns the credits produced by powered Gold Refineries this turn.
// Each refinery converts up to GoldConsumptionRate from the gold available at
// the start of the turn. BalanceTurn applies the matching gold draw, so this
// function does not modify the pool.
// C++ ported : unitinfo.cpp - UnitInfo::UpdateProduction (COMMTWR && ORDER_POWER_ON:
// team_gold += 5) + cargo.cpp Cargo_GetGoldConsumptionRate (the matching -5 draw).
func (c *Complex) RefineGold(availableGold int) int {
	credit := 0
	for _, u := range c.Members {
		if u.Type != assets.CommTwr || !active(u) || u.PowerStarved {
			continue
		}
		want := GoldConsumptionRate(u.Type)
		if want > availableGold {
			want = availableGold
		}
		if want <= 0 {
			continue
		}
		availableGold -= want
		credit += want
	}
	return credit
}

// CapStores clamps the pooled stores to what the complex can actually HOLD: the summed
// storage capacity of its buildings. Anything over that is WASTED, which is the original's
// behaviour - it keeps no pool at all. There the stock lives in the buildings, each capped
// at its own ATTRIB_STORAGE, and the complex total is rebuilt from their inventory every
// turn, so a mine that out produces the base's storage simply spills its output on the
// ground. The cure, then as now, is to build storage units.
//
// The spill is silent: no message tells the player, as in the original.
//
// C++ ported : access.cpp - Access_UpdateResourcesTotal (the per turn rebuild from
// inventory) + unitinfo.cpp - UnitInfo::UpdateProduction (storage = min(pool, capacity)).
func (c *Complex) CapStores() {
	_, capacity := c.CargoInfo()

	c.Material = min(c.Material, capacity.Raw)
	c.Fuel = min(c.Fuel, capacity.Fuel)
	c.Gold = min(c.Gold, capacity.Gold)
}

// redistribute fills the member buildings' cargo bars from the pooled raw/fuel/gold each
// turn so the on screen storage gauges reflect what the base holds. The pool is capped at
// capacity (CapStores) before this runs, so the bars and the pool agree: what the gauges
// show IS what the base has.
// C++: Complex::Transfer distributes cargo into buildings.
func (c *Complex) redistribute() {
	raw, fuel, gold := c.Material, c.Fuel, c.Gold
	for _, u := range c.Members {
		switch CargoTypeOf(u.Type) {
		case CargoRaw:
			u.Storage = 0
			transferCargo(u, &raw)
		case CargoFuel:
			u.Storage = 0
			transferCargo(u, &fuel)
		case CargoGold:
			u.Storage = 0
			transferCargo(u, &gold)
		}
	}
}

/* // note: unused code in Go port
// maxInt repeated the builtin max, which the economy now calls.
// maxInt returns the larger of two ints (stores clamp at zero).
func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
*/

/* // note: unused code in Go port
// minInt repeated the builtin min, which the economy now calls.
// minInt returns the smaller of two ints (stores clamp at capacity).
func minInt(a, b int) int {
	if a < b {
		return a
	}

	return b
}
*/

/* // note: unused code in Go port
// CanPower is only read by tests; the live power on gate is StartShortage, which shares hasPowerSupply.
// CanPower reports whether the complex can run unit u: u makes its own power or
// needs none, or the complex has a generator AND a fuel source (fuel in stock OR a
// member mining fuel). A base on a raw+fuel deposit sustains itself even at 0 stock
// (generators ignore fuel STOCK, the per turn balance refills from mining). Only a
// generator with NO fuel at all, or an isolated complex with no generator, bounces
// "not enough power". C++: the power/no fuel check in ValidateAuxilaryIndustry.
func (c *Complex) CanPower(u *game.UnitInfo) bool {
	if isGenerator(u.Type) || PowerConsumptionRate(u.Type) == 0 {
		return true
	}
	return c.hasPowerSupply()
}
*/

// isGenerator reports whether a type produces power (a power generator or station).
func isGenerator(t assets.UnitType) bool {
	return t == assets.PowGen || t == assets.PowerStn
}

// hasPowerSupply reports whether the complex contains a generator AND a fuel
// source for it (fuel in stock OR a member mining its own fuel). Shared by
// CanPower and StartShortage.
func (c *Complex) hasPowerSupply() bool {
	hasGenerator := false
	hasFuel := c.Fuel > 0
	for _, m := range c.Members {
		if isGenerator(m.Type) {
			hasGenerator = true
		}
		if m.FuelMining > 0 {
			hasFuel = true // mines its own fuel: sustains itself even at 0 stock
		}
	}
	return hasGenerator && hasFuel
}

// Shortage names the resource that blocks a building from powering on, or
// ShortageNone when it can start. It is what ComposeIndustryMessage
// _(961d) "Cannot turn %s on, %s needed" reports as the 2nd %s.
type Shortage int

const (
	ShortageNone Shortage = iota
	ShortagePower
	ShortageWorkers
)

// workerSurplus is the complex's FREE worker count right now: active habitats house
// minus active personnel consumers draw, computed LIVE from members so a just destroyed habitat lowers it immediately. A
// consumer powered ON but inactive is not counted, so surplus >= its need means it
// can be staffed.
func (c *Complex) workerSurplus() int {
	w := 0
	for _, u := range c.Members {
		if active(u) {
			w += -LifeConsumptionRate(u.Type) // habitat houses (+), consumer draws (-)
		}
	}
	return w
}

// StartShortage reports which resource (if any) prevents powering building u on:
// ShortagePower (power consumer, no generator/fuel), ShortageWorkers (personnel
// consumer, no spare worker via the LIVE workerSurplus), else ShortageNone.
// C++ ported : production_manager.cpp ValidateAuxilaryIndustry - the power vs life
// deficiency that bounces ORDER_POWER_ON.
func (c *Complex) StartShortage(u *game.UnitInfo) Shortage {
	if isGenerator(u.Type) {
		return ShortageNone
	}
	if PowerConsumptionRate(u.Type) > 0 && !c.hasPowerSupply() {
		return ShortagePower
	}
	if need := LifeConsumptionRate(u.Type); need > 0 && c.workerSurplus() < need {
		return ShortageWorkers
	}
	return ShortageNone
}

/* // note: unused code in Go port
// HasActiveGenerator is only read by a test; the live checks use hasGeneratorMember (the optimizer powers idle generators off).
// HasActiveGenerator reports whether any active member produces power, so the
// complex can run its power consuming buildings. A detached complex with no
// generator cannot power its consumers at all.
func (c *Complex) HasActiveGenerator() bool {
	for _, u := range c.Members {
		if active(u) && isGenerator(u.Type) {
			return true
		}
	}
	return false
}
*/

// hasGeneratorMember reports whether the complex has a generator at all (on OR off,
// but not disabled) - a power SOURCE it can switch on. Distinct from
// HasActiveGenerator: the optimizer powers idle generators off, so "active" no longer
// means "can be powered"; only "no generator member at all" means detached/unpowerable.
func (c *Complex) hasGeneratorMember() bool {
	for _, u := range c.Members {
		if isGenerator(u.Type) && u.Order != game.OrderDisable {
			return true
		}
	}
	return false
}

// IdleUnpowered powers OFF a complex's power consuming buildings when it has NO
// generator at all (a structure sited off base with no connector), so they sit idle
// instead of "running" ungated. A complex WITH a generator (even one the optimizer
// idled) is left alone: a real deficit there is handled per consumer in BalanceTurn,
// and generators never gate on fuel STOCK. Stores and generators are never idled, and
// it only forces OFF, never back ON (the player powers it on once connected). Called
// each turn from BalanceTurn and once at scenario setup (Game.ReconcilePower).
// C++ ported : units_manager.cpp - ProcessOrderPowerOn bounces an unpowerable building.
func (c *Complex) IdleUnpowered() {
	if c.hasGeneratorMember() {
		return // has a power source (active or auto idled): leave its members alone
	}
	for _, u := range c.Members {
		// Skip generators and buildings that draw no power (stores need none).
		if isGenerator(u.Type) || PowerConsumptionRate(u.Type) <= 0 {
			continue
		}
		// Idle a running consumer; a player off / disabled / already idle one stays.
		if u.Order == game.OrderPowerOn {
			u.Order = game.OrderPowerOff
			u.State = game.OrderStateExecutingOrder
		}
	}
}

// factoryProducing reports whether a factory is actively building - the only state
// in which it draws power. The engine charges upkeep only under ORDER_BUILD; here a
// positive BuildRate (zeroed when paused or the queue empties) is the equivalent.
func factoryProducing(u *game.UnitInfo) bool {
	return u.BuildRate > 0
}

// powerDraw is a member's live power consumption this turn: 0 for a generator, a
// building that draws no power (stores), an inactive building, or an idle factory
// that is not producing; otherwise its PowerConsumptionRate. C++:
// Cargo_GetPowerConsumptionRate gated by the ORDER_BUILD producing state for factories.
func powerDraw(u *game.UnitInfo) int {
	draw := PowerConsumptionRate(u.Type)
	if draw <= 0 || !active(u) { // generators (negative), zero draw, or off
		return 0
	}
	if isFactory(u.Type) && (!factoryProducing(u) || u.PowerStarved) {
		return 0 // idle or paused, or power HELD this turn (build paused, draws nothing)
	}
	return draw
}

// consumerPowerDemand is the total power the complex's active consumers need this
// turn (a factory counts only while producing). C++: -total.power after the active
// generators' output is removed in OptimizePowerProduction.
func (c *Complex) consumerPowerDemand() int {
	demand := 0
	for _, u := range c.Members {
		demand += powerDraw(u)
	}
	return demand
}

// availableGenerators counts the complex's generators of type t that could run
// (powered on or off, but not disabled).
func (c *Complex) availableGenerators(t assets.UnitType) int {
	n := 0
	for _, u := range c.Members {
		if u.Type == t && u.Order != game.OrderDisable {
			n++
		}
	}
	return n
}

// targetGenerators returns the minimum FUEL count of active power stations and power
// generators to supply `demand`, given how many of each are available. A station makes
// 6 power for 8 fuel, a generator 1 for 2, so stations carry the bulk and generators
// fill the remainder (a station preferred once the remainder hits the crossover, 4
// power). When they cannot meet demand it returns the most they can supply, and the
// consumer shed trims the deficit.
// C++ ported : production_manager.cpp ProductionManager::OptimizePowerProduction.
func targetGenerators(demand, stationCount, genCount int) (stations, generators int) {
	const stnPower, stnFuel, genPower, genFuel = 6, 8, 1, 2
	power := demand
	if power < 0 {
		power = 0
	}
	count := (genPower * stnFuel) / genFuel // the station/generator crossover (=4)
	stations = (power + stnPower - count) / stnPower
	if stations > stationCount {
		stations = stationCount
	}
	power -= stations * stnPower
	if power < 0 {
		power = 0
	}
	generators = (power + genPower - 1) / genPower
	if generators > genCount {
		generators = genCount
	}
	power -= generators * genPower
	if power > 0 { // remainder still uncovered: top up with another station if one is free
		count2 := (power + stnPower - 1) / stnPower
		if count2 > stationCount-stations {
			count2 = stationCount - stations
		}
		stations += count2
		power = demand - stations*stnPower
		if power < 0 {
			power = 0
		}
		generators = (power + genPower - 1) / genPower
		if generators > genCount {
			generators = genCount
		}
	}
	return stations, generators
}

// optimizeGenerators powers the complex's generators on/off to the minimum fuel set
// that meets the current consumer demand. This is the automatic management the player
// relies on to conserve: drop a consumer and an idle generator powers itself off;
// power a generator off and it comes back on if its power is still needed. C++:
// OptimizePowerProduction + OptimizePowerConsumption.
func (c *Complex) optimizeGenerators() {
	stations, generators := targetGenerators(
		c.consumerPowerDemand(),
		c.availableGenerators(assets.PowerStn),
		c.availableGenerators(assets.PowGen),
	)
	c.setGeneratorActive(assets.PowerStn, stations)
	c.setGeneratorActive(assets.PowGen, generators)
}

// setGeneratorActive leaves exactly `want` of type t's generators powered on,
// PREFERRING ones already on - so a generator the player just turned off comes back
// only when genuinely needed. Disabled generators are never touched.
// C++: UpdateUnitPowerConsumption.
func (c *Complex) setGeneratorActive(t assets.UnitType, want int) {
	// First pass: keep up to want already on; power the surplus down.
	poweredOn := 0
	for _, u := range c.Members {
		if u.Type != t || u.Order == game.OrderDisable || u.Order != game.OrderPowerOn {
			continue
		}
		if poweredOn < want {
			poweredOn++ // already on and still needed: leave it running
		} else {
			u.Order = game.OrderPowerOff // surplus: power it down (saves fuel)
			u.State = game.OrderStateExecutingOrder
		}
	}
	// Second pass: still short of want -> power on the off generators to cover it.
	for _, u := range c.Members {
		if poweredOn >= want {
			break
		}
		if u.Type != t || u.Order == game.OrderDisable || u.Order == game.OrderPowerOn {
			continue
		}
		u.Order = game.OrderPowerOn
		u.State = game.OrderStateExecutingOrder
		poweredOn++
	}
}

// activeGeneratorOutput is the power the complex's currently active generators make.
func (c *Complex) activeGeneratorOutput() int {
	out := 0
	for _, u := range c.Members {
		if active(u) && isGenerator(u.Type) {
			out += -PowerConsumptionRate(u.Type)
		}
	}
	return out
}

// maxGeneratorOutput is the power the complex could make with EVERY generator on
// (the ceiling the optimizer can reach). Used by the on demand power on capacity
// check. C++: the power available after OptimizePowerProduction maxes the generators.
func (c *Complex) maxGeneratorOutput() int {
	out := 0
	for _, u := range c.Members {
		if isGenerator(u.Type) && u.Order != game.OrderDisable {
			out += -PowerConsumptionRate(u.Type)
		}
	}
	return out
}

// CanAbsorbPower reports whether the complex could power a NEW consumer drawing
// `draw` power: the maximum generator output must cover the existing demand plus
// `draw`. Generators are not gated on fuel STOCK (a base short on fuel still runs
// them), so this is a pure power capacity test. C++: ProductionManager::CheckPowerNeed.
func (c *Complex) CanAbsorbPower(draw int) bool {
	return c.consumerPowerDemand()+draw <= c.maxGeneratorOutput()
}

// shedRank gives the power down order on a shortfall (lower = shed FIRST): Research,
// Eco Sphere, Gold Refinery, Factories, Mining last. C++ ported :
// ProductionManager_OptimizeProduction (RESEARCH -> GREENHSE -> COMMTWR ->
// OptimizeIndustry -> OptimizeMiningIndustry).
func shedRank(t assets.UnitType) int {
	switch {
	case t == assets.Research:
		return 0
	case t == assets.Greenhse:
		return 1
	case t == assets.CommTwr:
		return 2
	case isFactory(t):
		return 3
	case t == assets.MiningSt:
		return 4
	default:
		return 5 // habitat/radar etc. draw no power and are never shed victims
	}
}

// lowestPriorityConsumer returns the active power drawing member the complex should
// shed first (lowest shedRank, Research before a mine), or nil when nothing draws.
func (c *Complex) lowestPriorityConsumer() *game.UnitInfo {
	var victim *game.UnitInfo
	rank := 0
	for _, u := range c.Members {
		if powerDraw(u) <= 0 {
			continue // not drawing power: off, held, zero draw, idle factory, or a generator
		}
		if r := shedRank(u.Type); victim == nil || r < rank {
			victim, rank = u, r
		}
	}
	return victim
}

// OptimizeProduction reconciles power: optimize generators, then while consumers draw
// more than they make shed the lowest priority one (Research -> Eco Sphere -> Gold
// Refinery -> Factory -> Mining) until demand fits, and reoptimize. A factory is HELD
// (PowerStarved: build pauses, resumes when power returns); an aux/mine is durably
// powered OFF (the player powers it again). Then reconciles WORKERS the same way (see
// shedLifeConsumers). Returns members shed/held for POWER (for the banner).
// C++ ported : production_manager.cpp ProductionManager_OptimizeProduction.
func (c *Complex) OptimizeProduction() []*game.UnitInfo {
	for _, u := range c.Members {
		u.PowerStarved = false // recomputed below from the live power balance
	}
	// A complex with NO generator at all (a structure sited off base) idles its
	// consumers up front, so nothing "runs" ungated.
	c.IdleUnpowered()
	c.optimizeGenerators()

	var shed []*game.UnitInfo
	for c.consumerPowerDemand() > c.activeGeneratorOutput() {
		victim := c.lowestPriorityConsumer()
		if victim == nil {
			break
		}
		if isFactory(victim.Type) {
			victim.PowerStarved = true // build holds; resumes automatically when power returns
		} else {
			victim.Order = game.OrderPowerOff // durable: the player powers it again
			victim.State = game.OrderStateExecutingOrder
		}
		shed = append(shed, victim)
	}
	c.shedLifeConsumers()  // power off personnel buildings the habitats can no longer staff
	c.optimizeGenerators() // both sheds lowered demand: power down any generator now idle
	c.lastShed = shed
	return shed
}

// shedLifeConsumers powers off worker consumers by priority until active habitats
// can staff the remaining buildings. It runs after power shedding and leaves each
// shed building off until the player restores it.
// C++ ported : production_manager.cpp ProductionManager::UpdateLifeConsumption.
func (c *Complex) shedLifeConsumers() {
	var shed []*game.UnitInfo
	for c.workerSurplus() < 0 {
		victim := c.lowestLifeConsumer()
		if victim == nil {
			break
		}
		victim.Order = game.OrderPowerOff // durable: the player powers it again after housing them
		victim.State = game.OrderStateExecutingOrder
		shed = append(shed, victim)
	}
	c.lastLifeShed = shed
}

// lowestLifeConsumer returns the active personnel (worker) consumer the complex
// should power off first when short of workers - the lowest shedRank, so research
// goes before the Eco Sphere before the training hall. Returns nil when no active
// member draws workers. C++: the UpdateUnitLifeConsumption candidate walk.
func (c *Complex) lowestLifeConsumer() *game.UnitInfo {
	var victim *game.UnitInfo
	rank := 0
	for _, u := range c.Members {
		if !active(u) || LifeConsumptionRate(u.Type) <= 0 {
			continue // not an active worker consumer (a habitat houses, mining/power draw none)
		}
		if r := shedRank(u.Type); victim == nil || r < rank {
			victim, rank = u, r
		}
	}
	return victim
}

// LastShed returns the consumers OptimizeProduction shed or held on its most recent
// run (for the "not enough power" banner). Transient - valid until the next reconcile.
func (c *Complex) LastShed() []*game.UnitInfo {
	return c.lastShed
}

// LastLifeShed returns the personnel consumers powered off for lack of workers on the
// most recent reconcile (for the "not enough workers" banner). Transient.
func (c *Complex) LastLifeShed() []*game.UnitInfo {
	return c.lastLifeShed
}

// LifeReport summarizes the complex's worker balance for debug tracing: the workers
// housed by active habitats, those drawn by active personnel consumers, and the
// resulting surplus (negative means shedLifeConsumers should power one off).
func (c *Complex) LifeReport() (housed, drawn, surplus int) {
	for _, u := range c.Members {
		if !active(u) {
			continue
		}
		switch r := LifeConsumptionRate(u.Type); {
		case r < 0:
			housed += -r // a habitat houses workers
		case r > 0:
			drawn += r // an Eco Sphere / research / training hall draws one
		}
	}
	return housed, drawn, housed - drawn
}

// PowerOnDraw is the power a building would draw once powered ON - its
// PowerConsumptionRate, except a factory draws only while producing (an idle factory
// toggled on draws nothing). Used by the on demand power on capacity check so it does
// not over reserve for an idle factory.
func PowerOnDraw(u *game.UnitInfo) int {
	draw := PowerConsumptionRate(u.Type)
	if draw <= 0 {
		return 0
	}
	if isFactory(u.Type) && !factoryProducing(u) {
		return 0
	}
	return draw
}

// isFactory reports whether a building produces mobile units (the unit plants +
// shipyard + barracks). On a power shortfall these are shed AFTER research / the
// Eco Sphere / the gold refinery, but before mining. C++: the *_PLANT / SHIPYARD /
// TRAINHAL production set.
func isFactory(t assets.UnitType) bool {
	switch t {
	case assets.Shipyard, assets.LightPlt, assets.LandPlt, assets.AirPlt, assets.TrainHal:
		return true
	}
	return false
}

/* // note: unused code in Go port
// PowerShort is only read by Game.PowerShort, which has no caller; the alert reads u.PowerStarved.
// PowerShort reports whether any of the complex's powered on consumers was starved
// for power this turn (a deficit cut it). The in game screen reads this to flash the
// "not enough power" alert. C++: the power deficit warning.
func (c *Complex) PowerShort() bool {
	for _, u := range c.Members {
		if u.PowerStarved {
			return true
		}
	}
	return false
}
*/
