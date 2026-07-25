# Economy logic

How resources,
mining,
power,
and the per-turn balance work in this project.
This documents the code in `internal/economy` (plus the sim hooks in `internal/sim`),
ported from the C++ `cargo.cpp`,
`complex.cpp`,
`resource_allocator.cpp`,
`production_manager.cpp` and `game_manager.cpp`.

## Resources

Three materials,
the `economy.CargoType` enum:

- **raw** (materials) - builds units and structures.
- **fuel** - burned by power generators to produce power.
- **gold** - credits; spent on purchases and upgrades.

A `Cargo` value carries all of these plus `Power`,
`Life` (workers) and `FreeCapacity`,
used as a per turn delta or a stored total.

## Deposits and the resource map

`ResourceMap` (`resourcemap.go`) holds one deposit per cell: a type and an amount (stored capped at 31,
read capped at 16). `SetDeposit` / `DepositAt` are the accessors;
it also tracks per team survey state (the in game survey overlay reads that).

At game start the deposits are scattered by the **ResourceAllocator** (`resource_allocator.go`,
a faithful port of `resource_allocator.cpp` + `GameManager_PopulateMapWithResources`):

1. A **normal** background level of each material on its own interleaved grid
(raw on even/even cells,
gold on odd/even,
fuel on odd/odd).
2. **Mixed separation** then **concentration** scatter richer lodes.
3. Each team's **landing spot is boosted** so a base lands on usable raw/fuel.
4. Zones below `min_resources` are cleared.

It is deterministic for a given `game.Randomizer` seed (test `TestAllocateResources`).
The tunables (`ResourceParams`) default to the original's shipped `settings.ini` values (`max_resources` 20,
`min_resources` 9,
the per material normal/concentrate bands,
etc.). `AllocateResources` is wired into the `-random` quick start (and the custom game landing) with the team start points so the boost lands where the bases do.

## Complexes

A `Complex` is a connected group of a team's buildings that share one pool of stores (`Material`,
`Fuel`,
`Gold`,
`Power`,
`Workers`).
Buildings join a complex by edge adjacency (the build `Network`);
a connector building can bridge two.
The complex is the unit of the per turn balance.

## Mining

A mining station extracts raw/fuel/gold each turn from the deposits under its footprint (`RawMining` / `FuelMining` / `GoldMining`,
with `TotalMining` capped at 16),
set by `SetInitialMining` when it is placed.
The yield is scaled by a difficulty factor: 4 for a human or easy AI,
5 for master,
6 for god.
A mining station only mines while powered on.

## Per unit production and consumption

`NetProduction(unit,
difficulty)` is one building's per turn contribution:

- **Mining station** (powered): produces scaled raw/fuel/gold, minus its own
upkeep.
- **Power generator / station** (powered): produces power, burns fuel
(`FuelConsumptionRate`: PowGen 2,
PowerStn 6;
power output: PowGen 1,
PowerStn 6).
- **Factory** (Shipyard/LightPlt/LandPlt/AirPlt/TrainHal) draws raw + power
**while building** a unit (`ORDER_BUILD`,
not yet `UNIT_READY`),
at its `BuildRate` (1/2/4);
idle or finished it draws nothing.
- **Other powered buildings** (CommTwr, Research, Greenhse, Habitat) draw their
upkeep (power,
gold for the comm tower,
workers/life).

The rate tables live in `cargo.go` (`RawConsumptionRate`,
`FuelConsumptionRate`,
`PowerConsumptionRate`,
`LifeConsumptionRate`,
`GoldConsumptionRate`).

## The per turn balance

`Complex.BalanceTurn(difficulty)` runs once per turn per complex (from `sim.RunEconomy`,
called in `EndTeamTurn`).
The order matters:

1. **Auto power generators** - if the complex's consumers draw more power than its active
generators supply,
idle generators are switched on (PowGen before PowerStn) until the balance is met.
This is the forced minimum generator rule (C++ `ProductionManager::SatisfyPowerDemand`): you cannot leave the base's only generator off while a building needs it.
2. **Generators fill a power budget**, but only while the start of turn fuel
store covers their burn.
A generator starved of fuel supplies no power.
3. **Consumers run in priority** - mining stations first (essential), then the
rest - and only if the power budget covers their draw.
A consumer with no power available **browns out**: it produces nothing that turn.
4. The net raw/fuel/gold accumulate into the complex stores; `Power` is the live
leftover budget (not accumulated);
`Workers` is the life balance.

So a building needs power to run,
and power needs fuel:

```
fuel  ->  generator  ->  power budget  ->  mining / factories run
```

Pull the fuel (or the generator) and the chain breaks: the generator cannot run,
the budget is empty,
and the consumers brown out.
The `-random` base lands with a starting supply (50 raw,
30 fuel per complex) so it has build material and the generator has runway while the mining station ramps up its own fuel income.

## Faithfulness and deferred behavior

What is modeled matches the original in spirit and in the rate tables.
Still simplified,
pending the full `production_manager.cpp` port (M6 scale):

- **No shutoff cascade.** When a complex runs a sustained material/power deficit
the original's `ValidateAuxilaryIndustry` powers off the least essential buildings with a warning message;
here a consumer short of power simply browns out for the turn and recovers when power returns.
- **Turn boundary forced generator.** The forced minimum generator is enforced in
`BalanceTurn` (once per turn),
not as an immediate bounce back with the stop/start sound and message when you click power off.
- **Start of turn fuel.** Generators gate on the fuel store at the start of the
turn;
this turn's mining fuel income helps the next turn (avoids a circular dependency).
- **Factory unit production** is not yet wired to the build flow, so the factory
build draw only fires once that lands.

## Source map

| Topic | Go | C++ |
|-------|----|-----|
| Cargo, rate tables, NetProduction | `internal/economy/cargo.go` | `cargo.cpp` |
| Complex, BalanceTurn, power balance | `internal/economy/complex.go` | `complex.cpp`, `production_manager.cpp` |
| Resource map | `internal/economy/resourcemap.go` | resource map in `resource_manager.cpp` |
| Deposit placement | `internal/economy/resource_allocator.go` | `resource_allocator.cpp`, `game_manager.cpp` |
| Per turn hooks, mining init | `internal/sim/sim.go` (`RunEconomy`, `PlaceBuilding`) | `game_manager.cpp` |
