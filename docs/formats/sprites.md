# Multi frame sprites (unit bodies, turrets, shadows)

Unit graphics in `MAX.RES` are stored as multi frame sprite resources: one resource holds every animation/facing frame of a unit's body (or turret,
or shadow).
The body and its shadow are two separate resources,
e.g. `BOMBER` (pixels) and `S_BOMBER` (silhouette).
Shadow tags are the body tag prefixed with `S_` and truncated to 8 characters (`ANTIAIR` -> `S_ANTIAI`).

All integers are little endian.
Offsets are byte offsets from the start of the resource data.

Decoder: `internal/sprites/multi.go` (`DecodeMultiSprite` / `DecodeMultiShadow`).
C++ reference: `ImageMultiHeader` / `ImageMultiFrameHeader` (`resource_manager.hpp`),
decoded by `Gfx_DecodeSprite` / `Gfx_DecodeShadow` (`gfx.cpp`).

## Container layout

| Offset | Size        | Field        | Meaning                                  |
|--------|-------------|--------------|------------------------------------------|
| 0      | 2 (u16)     | image_count  | number of frames                         |
| 2      | 4xcount (i32)| frame_offset[]| byte offset to each frame header        |
| ...    | varies      | frame data   | the frame headers + row data, in any order|

`image_count` is typically 6/8/10 for buildings and 8,
16,
24,
32,
38... for mobile units (8 facing angles x animation passes;
turrets and firing frames add more).

## Frame header (`ImageMultiFrameHeader`)

At `frame_offset[i]`:

| Offset | Size    | Field  | Meaning                                  |
|--------|---------|--------|------------------------------------------|
| +0     | 2 (i16) | width  | frame width in pixels                    |
| +2     | 2 (i16) | height | frame height in pixels                   |
| +4     | 2 (i16) | hotx   | hotspot X (draw anchor)                  |
| +6     | 2 (i16) | hoty   | hotspot Y (draw anchor)                  |
| +8     | 4xheight (u32)| row_offset[]| absolute offset of each row's run data|

The hotspot is the unit's logical centre: a frame is drawn so that `(hotx,
hoty)` lands on the target screen point (top left corner = `point - hotspot`).
A frame may be empty (`width` or `height` == 0),
used for a missing facing.

`row_offset[y]` points (from the start of the resource,
not the frame) at the run length data for row `y`.

## Row run length data

### Sprite rows (with pixels): `DecodeMultiSprite`

A row is a sequence of spans,
each:

| Size | Field            | Meaning                                |
|------|------------------|----------------------------------------|
| 1    | transparent_count| transparent pixels to skip first       |
| 1    | pixel_count      | number of literal pixels that follow   |
| pixel_count | pixels    | palette indices, drawn opaque          |

Spans repeat until a byte `0xFF` is read where a `transparent_count` would begin,
which ends the row.
A fully transparent row is a single `0xFF`.
Skipped pixels are transparent (not written).

### Shadow rows (silhouette only): `DecodeMultiShadow`

Identical container and frame header,
but shadow spans carry no pixel bytes:

| Size | Field            | Meaning                                |
|------|------------------|----------------------------------------|
| 1    | transparent_count| transparent pixels to skip             |
| 1    | shadow_count     | length of the shadow (silhouette) run  |

again terminated by `0xFF`.
The shadow stores only *shape*.
When drawn,
each silhouette pixel darkens the destination pixel through a brightness reduced palette LUT (`Gfx_DecodeShadow`: `dst = darken[dst]`).
The LUT itself is built from the palette and is not yet ported (the decoder exposes the silhouette mask;
the caller supplies the darkening table).

## Team colours

Unit ownership is shown by remapping a fixed colorable band of the palette per team.
The band is palette indices `32..39` and `48..63` (`40..47` is left alone);
each team maps that band onto an eight entry ramp:

| Team     | Ramp (8 indices)                       |
|----------|----------------------------------------|
| Red      | 56 57 58 59 60 61 62 63                |
| Green    | 32 33 34 35 36 37 38 39 (the band's own, identity) |
| Blue     | 48 49 50 51 52 53 54 55                |
| Gray     | 255 161 172 169 216 213 212 207        |
| Derelict | 216 215 214 213 212 211 210 209        |

`TeamColorTable(team)` returns the full 256-entry LUT (identity outside the band).
It is passed to `MultiFrame.Draw` as the per pixel remap.
C++: `ResourceManager_BuildColorTables`,
`ResourceManager_Team*ColorIndices`.

## Unit frame layout (`D_<unit>` data resource)

A unit type's sprite sheet (the `<unit>` tag,
e.g. `TANK`) packs its body,
turret and firing frames back to back.
How those frames are partitioned is described by a small companion resource named `D_` + the first six characters of the unit tag (`D_TANK`,
`D_AWAC`,
`D_CONSTR`...).
Many units share template data resources whose names are irregular (`D_MOBILE`,
`D_FIXED`,
`D_LRGBLD`,
...);
a unit with no data resource uses the default layout (body frames starting at 0,
no turret/firing/connector).

| Offset | Type | Field |
|--------|------|-------|
| 0  | int8 | `image_base` (first body frame) |
| 1  | int8 | `image_count` (body frames, one per facing) |
| 2  | int8 | `turret_image_base` |
| 3  | int8 | `turret_image_count` |
| 4  | int8 | `firing_image_base` |
| 5  | int8 | `firing_image_count` |
| 6  | int8 | `connector_image_base` |
| 7  | int8 | `connector_image_count` |
| 8.. | int8 x16 | eight `(x, y)` turret mount offsets, one per facing (present only for turreted units, i.e. a 24-byte resource) |

All fields are signed bytes.
A unit's body frame for facing `a` is `image_base + a`;
its turret frame is `turret_image_base + turret_angle`,
drawn at the body position plus the facing's mount offset.
Example: `D_TANK` (24 bytes): body `0..7`,
turret `8..15`,
firing `16..23`,
with mount offsets `(0,7) (-7,7) (-8,0) (-8,-7) (0,-8) (6,-8) (7,0) (7,6)`.

C++: `struct FrameInfo` (unit.hpp),
`Sprites::DecodeFrameInfo` (installer/sprites.cpp).
Ported in `internal/game/frameinfo.go`.

## Soldier walk cycle (infantry / commando)

Most mobile units have exactly eight body frames,
one per facing,
and stand still on the map.
The two foot soldiers,
infantry and commando,
are different: their body sheets hold a long walk cycle plus firing and death frames.
The walk frames are interleaved by facing,
so frame `image_base + angle + 8*phase` is facing `angle` at walk phase `phase`:

```
phase 0 (standing):  frames  0.. 7   (facing 0..7)
phase 1:             frames  8..15
phase 2:             frames 16..23
   ...
phase 12:            frames 96..103
```

A marching soldier advances one phase per step,
wrapping back to phase 0 (the standing frame for the facing) when the next phase would exceed `image_index_max`.
For these two types the original hardcodes `image_index_max = 103` (`UnitInfo::Init`,
unitinfo.cpp) rather than deriving it from `image_count`: the frames above 103 are the firing and death animations,
which the walk must not run into.
With `image_base = 0` that gives 13 walk phases (a standing phase plus 12 stride frames).

> Note: `image_count` is a signed byte, and these sheets have 200+
> frames, so `D_INFANT`/`D_COMMAN` decode `image_count` as a negative
> number (e.g. -56). That value is unused for soldiers precisely because
> `image_index_max` is the hardcoded 103; the renderer special cases the
> two soldier types before any `image_count`-based branch.

In this project the phase is a per-unit counter (`UnitInfo.WalkFrame`,
advanced by `AdvanceWalk`);
the frame is computed by `soldierWalkFrame` in `internal/units/sprite.go`.
The cycle runs only while the soldier is executing a move order (matching the original,
which advances the frame inside `UnitInfo::Move`).
C++: the `COMMANDO`/`INFANTRY` `image_index + 8` branch in `UnitInfo::Move` (unitinfo.cpp:1806).

Inspect it with `max-go.exe -emulate soldier`: both soldier types in all eight facings on snow,
marching in place.

## Amphibious water sprite (surface frame swap)

A few land units can drive onto water and carry a second body facing set for it: a boat / pontoon form drawn instead of the land vehicle while the unit stands on a full open water cell.
The original picks it in `UnitInfo::SetSpriteFrameForTerrain` (unitinfo.cpp),
which adds 8 to `image_base` on a `SURFACE_TYPE_WATER` cell (inverted for the personnel carrier).
Coast / shore,
a bridge,
and a water platform all read as land,
so only a full water cell swaps.

The two facing sets sit back to back in the sheet: frames `0..7` are the land facings,
frames `8..15` the water facings.
For a builder,
whose sheet also holds a build pose,
the pose set is a parallel copy offset by +16,
so the water build pose follows the same +8 rule on top of it:

| Frames | Pose |
|--------|------|
| 0..7   | land drive (one per facing) |
| 8..15  | water drive |
| 16..23 | land build pose (drive frame + 16) |
| 24..31 | water build pose (water drive + 16) |

Which retail units actually carry a distinct water set was settled by decoding the sheets (a per facing pixel compare of frame
` against frame
+8`):

| Unit | Sheet | Water set (8..15) |
|------|-------|-------------------|
| Personnel Carrier (`CLNTRANS`) | 16 frames | yes - **inverted**: land set at 8, boat at 0 |
| Surveyor (`SURVEYOR`) | 16 frames | yes - 0..7 legs out land pose, 8..15 folded float pose |
| Engineer (`ENGINEER`) | 32 frames | yes - 8..15 a wider pontoon hull; 24..31 the water build pose |
| Scout (`SCOUT`) | 32 frames | yes - 8..15 a distinct water hull (no build pose) |
| Constructor (`CONSTRCT`) | 40 frames | **no** - frames 8..15 are a byte for byte copy of the land set, and 24..31 are a different build animation phase, not a water variant; the Constructor stays on its land look on water |

So the swap is enabled per type (`isAmphibious` in `internal/units/sprite.go`): the Personnel Carrier,
Surveyor,
Engineer and Scout swap;
the Constructor does not.
The driving swap is `amphibBase` (land 0 / water 8,
inverted for the carrier);
the build pose over water is `amphibBodyBase`,
used only by the engineer (the one amphibious builder).

> Note: the Surveyor / Scout water frames differ in size from their land
> frames per facing (the unit folds up), so the swap is visible; the
> Constructor's identical copies are why it was excluded - swapping would
> do nothing and it also misread the build pose.

C++: `UnitInfo::SetSpriteFrameForTerrain` (the `AMPHIBIOUS` `image_base` 0/8 swap),
driven from `UnitsManager_ActivateEngineer` and the move / transport unload paths.

## Scaled drawing and shadows

`MultiFrame.DrawScaled` / `DrawShadowScaled` blit a frame scaled by
um/den` screen pixels per 64-pixel tile (the map zoom over the tile size),
anchored so the hotspot lands on the unit's cell centre,
using the same nearest neighbour step as the terrain tiles (`((src-1)<<16)/(dst-1) + 8`).
A ground shadow replaces each covered destination pixel with `darken[dest]`,
where the darkening LUT (`gfx.Palette.DarkeningTable`) maps every colour to the nearest palette index of itself at roughly half brightness (each 6-bit component floored at 7,
scaled by 31/63).
A hovering air unit's shadow is offset one tile down right of the body.
C++: `Gfx_DecodeSprite` / `Gfx_DecodeShadow` and the `ResourceManager_DarkeningColorIndexTable` loop (world.cpp).

## Verification

- `internal/sprites/multi_test.go`: SHA-256 goldens of `BOMBER` frame 0
pixels and `S_BOMBER` frame 0 silhouette,
frame counts / dimensions / hotspots,
the team table band remap,
and synthetic draw/clip/shadow cases.
- `max-go.exe -emulate sprite:<TAG>` renders a sprite cycling through its
frames once per team colour.

## Not ported yet (carry over)

- Sprite rescaling for the low resolution (`DisableEnhancedGraphics`)
mode (`Gfx_RescaleSprite`).

## Shipped since this doc was written

- The brightness / darkening / tint colour index tables built from the
palette (`Gfx_DecodeShadow` darkening,
unit brightness levels): see `gfx.Palette.DarkeningTable`/`RedTintTable`/`BrightnessTable` (`internal/gfx/palette.go`),
wired in by `units.NewSpriteCache`/ `SetPalette` (`internal/units/sprite.go`).
- Per zoom scaled drawing: `MultiFrame.DrawScaled`/`DrawScaledCentered`/
`DrawShadowScaled` (`internal/sprites/multi.go`),
consumed by `units.SpriteCache.DrawUnit`/`DrawFrameAtPixel`/`DrawEffect` (`internal/units/sprite.go`) for tactical unit rendering.
