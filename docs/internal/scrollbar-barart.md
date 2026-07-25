# Scrollbar Bar Art

**Status (2026-07-25): WON'T-DO.** `internal/gnw.Scrollbar`/`LimitedScrollbar` are built and bench-tested (`-emulate scrollbar`),
but vanilla never puts a scrollbar on a build/research/report list — those use up/down arrows — and the five screens that do use a real vanilla scrollbar (transfer/upgrade/cargo/repair/alloc) already have faithful,
bespoke inline sliders that predate the widget.
Wiring the Go widget into any of them would only be cosmetic churn with real regression risk;
wiring it into a list screen would invent UI vanilla does not have.
The widget and its `LoadHorizontalBar`/`LoadVerticalBar` helpers stay in the tree as a bench-tested spare part for a genuinely new scrollbar screen,
not as pending work - nothing below is "remaining." The rest of this note (the resource table and the transparent-colour gotcha) stays accurate and useful background for anyone touching this bar art family.

## Core resources

The original names these resources in `resourcetable.hpp`:

| Resource | ID | Use |
| --- | ---: | --- |
| `BARRAW` | `0x216` | Allocation screen raw material horizontal bar. |
| `BARFUEL` | `0x217` | Allocation screen fuel horizontal bar. |
| `BARGOLD` | `0x218` | Allocation screen gold horizontal bar. |
| `BARUNUS` | `0x219` | Unused/empty bar art; not used by the current C++ call sites checked here. |
| `BARTAPE` | `0x21A` | Allocation screen tape overlay. |
| `BARTAPE2` | `0x21B` | Alternate tape art; no active C++ call site found in this sweep. |
| `BARTAPE3` | `0x21C` | Alternate tape art; no active C++ call site found in this sweep. |
| `BARTAPEX` | `0x21D` | Alternate tape art; no active C++ call site found in this sweep. |
| `SMBRRAW` | `0x21E` | Small horizontal raw material transfer bar. |
| `SMBRFUEL` | `0x21F` | Small horizontal fuel transfer bar. |
| `SMBRGOLD` | `0x220` | Small horizontal gold transfer bar. |
| `VERTRAW` | `0x221` | Vertical raw material bar. |
| `VERTFUEL` | `0x222` | Vertical fuel bar. |
| `VERTGOLD` | `0x223` | Vertical gold bar. |
| `LOADBAR` | `0x224` | Loading dialog progress bar. |

## CRITICAL: palette index 17 is the transparent colour

MAX bar art resources use **palette index 17** (a fuchsia/magenta slot) as their transparent colour,
NOT index 0.
The `gnw.Scrollbar` helpers and the GNW draw path treat index `0` as transparent,
so consumers **MUST** remap index 17 -> 0 before passing a decoded bar buffer to the widget.
Failing to do so causes every index 17 pixel to render as solid fuchsia.

Call `KeyToZero` on the decoded buffer immediately after loading:

```go
bar.KeyToZero(bar.At(0, 0)) // bar.At(0,0) returns the transparent_color field (palette idx 17)
```

`bar.At(0,
0)` reads the SIMPLE image's `transparent_color` header field,
which for this entire resource family is palette index 17.
This matches the project's existing menu `fuchsiaKey` / `KeyToZero` convention.
Do NOT skip this step even for a quick test render - the fuchsia bleed is obvious and hard to debug later.

## Screen mapping

The "C++ widget" column names the algorithm used by the vanilla screen (`LimitedScrollbar`/`Scrollbar`);
the "Go widget" column is what this project actually draws with today - in every case below,
a bespoke inline slider in the menu screen's own file,
not `internal/gnw.Scrollbar`.

| Screen / call site | C++ source | C++ widget | Go widget (actual) | Orientation | Bar art | Range source |
| --- | --- | --- | --- | --- | --- | --- |
| Purchase/custom start cargo loadout | `cargomenu.cpp` constructor and `CargoMenu::Select` | `LimitedScrollbar` | Bespoke fill logic in `internal/menus/cargodraw.go`, mirroring the `LimitedScrollbar` algorithm. | Vertical | Starts as `VERTRAW`; changes to `VERTFUEL` for `FUELTRCK`, `VERTGOLD` for `GOLDTRCK`, otherwise `VERTRAW`. | Value is selected cargo divided by 5. Free capacity is selected unit storage divided by 5, except `GOLDTRCK` uses zero free capacity. Give/take clamps are adjusted per selected row. |
| Transfer dialog | `transfermenu.cpp` constructor | `LimitedScrollbar` | Bespoke fill logic in `internal/menus/transfer.go` (`drawDynamic`, same "mirrors `LimitedScrollbar`'s value math" comment) | Horizontal | `SMBRRAW`, `SMBRFUEL`, or `SMBRGOLD` from source unit cargo type. | Zero offset is negative target cargo. Free capacity is target free capacity. Give/take clamps come from target free capacity and source free capacity. |
| Allocation screen resource bars | `allocmenu.cpp` `AllocMenu::DrawCargoBars` / `CargoBar::Draw` | Draw helper, not interactive scrollbar | `internal/menus/alloc.go`'s own bar+click drawing | Horizontal | `BARRAW`, `BARFUEL`, `BARGOLD`; tape overlay `BARTAPE`. | Per resource production, balance, capacity, usage, complex cargo, and complex capacity values from the complex allocator. |
| Load/progress dialog | Unconfirmed — `LOADBAR` (`0x224`) is a real `resourcetable.hpp` ID, but no `drawloadbar.cpp` equivalent call site was found; the same caveat applies to `BARTAPE2/3/X` below. | - | None — no load/progress dialog exists in this project; a repository-wide `LOADBAR` search finds only this document. | Horizontal | `LOADBAR` | Unconfirmed |
| Upgrade gold reserve | `abstractupgrademenu.cpp` | Draw helper, not interactive scrollbar | `internal/menus/upgrade.go`'s own bar drawing | Vertical | `VERTGOLD`. | Height is `(gold_background.Height - 25) * team_gold / start_gold`. |
| Repair shop raw material reserve | `repairshopmenu.cpp` `RepairShopMenu::DrawCargoBar` | Draw helper, not interactive scrollbar | `internal/menus/repairshop.go`'s own bar drawing | Vertical | `VERTRAW`. | Height is `materials.raw * raw_material_bar.Height / raw_material_in_complex`. |

## Helper behaviour

`scrollbar.cpp` uses three drawing helpers. `LoadHorizontalBar`/`LoadVerticalBar` were ported to Go alongside the widget,
but per the WON'T-DO status above they have no production callers either - same bench-tested-spare-part status as `gnw.Scrollbar` itself (confirmed: a repo-wide search finds them called only from the widget's own test/bench code).

| Helper | Go equivalent | Notes |
| --- | --- | --- |
| `LoadHorizontalBar` | `gnw.LoadHorizontalBar` (unused in production) | Copies the left portion of the bar resource up to the filled capacity, skipping transparent pixels. |
| `LoadVerticalBar` | `gnw.LoadVerticalBar` (unused in production) | Copies the top portion of the vertical bar resource. For the interactive scrollbar, the original draws it bottom aligned inside the slider bounds after computing the filled height. |
| `LoadHorizontalTape` | Not used here (see below) | Used by allocation bars for the `BARTAPE` overlay. `internal/menus/alloc.go` draws `BARTAPE` with inline logic after keying the art, rather than through this helper. |

`LimitedScrollbar` adds clamp limits on top of `Scrollbar`: one side limits how much can be given,
and the other limits how much can be taken.
For transfer/cargo wiring,
set `ZeroOffset`,
`FreeCapacity`,
`XferGiveMax`,
`XferTakeMax`,
then `Value`,
matching the original's constructor order.
