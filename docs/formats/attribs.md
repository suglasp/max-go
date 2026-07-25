# ATTRIBS: unit attribute table

`ATTRIBS` is the retail unit stat table stored in `MAX.RES`.
The Go loader parses it as plain text,
not as a binary structure.

The resource is read by `internal/assets.ParseAttribs`.
The reference implementation's current reference tree also contains a newer XOR obfuscated JSON resource path (`SC_ATTRI` plus `SC_SCHEA` in `attributes.cpp`),
but that is not the retail 1.0.4 table consumed by max-go.

## Layout

The file is line oriented ASCII:

- Empty lines are ignored.
- Lines whose first character after any spaces is `;` are comments.
- Every data line has exactly 13 whitespace separated decimal integers.
- Rows are bound by position, not by a unit name in the text.

There must be at least 93 data rows,
one for each unit type from `COMMTWR` through `WALDO`.
The canonical row order is `internal/assets.UnitTags`.

## Columns

| # | Go field | Meaning | Runtime attribute |
|---|----------|---------|-------------------|
| 1 | `TurnsToBuild` | Turns/cost to build. | `AttribTurns` / research cost topic |
| 2 | `HitPoints` | Maximum hit points. | `AttribHits` |
| 3 | `Armor` | Armor rating. | `AttribArmor` |
| 4 | `Attack` | Attack damage rating. | `AttribAttack` |
| 5 | `MoveAndFire` | Nonzero means moving does not consume fire budget. | `AttribMoveAndFire` |
| 6 | `Speed` | Movement points per turn. | `AttribSpeed` |
| 7 | `Fuel` | Fuel/energy reserve. | `AttribFuel` |
| 8 | `Range` | Attack range in tiles. | `AttribRange` |
| 9 | `Rounds` | Shots per turn. | `AttribRounds` |
| 10 | `Scan` | Visibility/scan range in tiles. | `AttribScan` |
| 11 | `Storage` | Cargo, mine, or transport capacity, depending on unit. | `AttribStorage` |
| 12 | `Ammo` | Ammunition capacity. | `AttribAmmo` |
| 13 | `Area` | Blast radius. Retail data uses this for rocket style splash. | `AttribAttackRadius` |

`AttribAgentAdjust` exists in the runtime `UnitValues` enum for reference-save compatibility,
but it is not present in the retail `ATTRIBS` text rows and starts at zero.

## Example

The retail TANK row is:

```text
4 24 10 16 0 6 80 4 2 4 0 14 0
```

It decodes to 4 build turns,
24 hit points,
10 armor,
16 attack,
no move and fire flag,
speed 6,
fuel 80,
range 4,
2 shots per turn,
scan 4,
no storage,
ammo 14,
and no blast radius.

## Runtime mapping

`assets.Load` reads the resource named `ATTRIBS` and stores the parsed rows as `Assets.Attribs`,
indexed by `assets.UnitType`.
When a unit is created,
`game.NewUnitValues` copies a row into the runtime `UnitValues` layout.
The first eight runtime attributes intentionally share indices with research topics:

`Attack`,
`Rounds`,
`Range`,
`Armor`,
`Hits`,
`Speed`,
`Scan`,
`Turns`.

That shared order is why research,
gold upgrades,
saved unit values,
and the base `ATTRIBS` rows must stay aligned.
