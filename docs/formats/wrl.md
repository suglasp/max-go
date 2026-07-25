# .WRL: world/map files

The 24 planet maps (`SNOW_1.WRL` .. `DESERT_6.WRL`),
loose files in the game directory.
Resources `SNOW_1` etc. in MAX.RES are filename references to them (like the audio resources).
All integers little endian.

## Layout

| Offset | Size      | Field       | Meaning                                 |
|--------|-----------|-------------|------------------------------------------|
| 0      | 3         | magic       | `WRL` or `DMO` (never validated)         |
| 3      | 2         | layer count | uint16, always 1 in retail files         |
| 5      | 4xlayers  | layer dims  | per layer: int16 width, int16 height (cells) |
| 9      | w*h       | minimap     | one palette index per cell               |
| +      | w*h*2     | tile index  | uint16 tile number per cell, row major   |
| +      | 2         | tile count  | uint16 (retail: 154..411)                |
| +      | n*64*64   | tiles       | 64x64 8bpp tile pixels, n consecutive blocks |
| +      | 768       | palette     | 256xRGB, 8-bit components                |
| +      | n         | pass table  | one surface byte per tile: 0..3          |

The layout accounts for every byte of the retail files (verified for all 24: `size = 9 + cells + 2*cells + 2 + 4097*n + 768`).

The original treats bytes 0..4 as one unit ("magic + version");
the reader actually seeks to offset 3 and reads a layer count plus one dimension pair per layer.
All retail files have exactly one layer.

## Pass table and surface map

The pass table holds one byte per *tile*: 0=land,
1=water,
2=coast,
3=air(bridge).
On load it is mapped through `PassData` to the engine's bit flag surface types (`SURFACE_TYPE_LAND` 0x1,
`WATER` 0x2,
`COAST` 0x4,
`AIR` 0x8,
enums.hpp),
and a per-*cell* surface map is built by indexing it with the cell's tile id (`world.Map.Surface`).

## Palette handling

- The map palette uses 8-bit components, divided by 4 into the 6-bit
range on load.
- A loaded map installs only entries 64..159 into the system palette;
everything else keeps the current (COLOR.PAL) colors (`world.Map.ApplyTerrainPalette`).
- For the planet selection preview, the palette is instead remapped to
the nearest colors of the current menu palette (full scan `Color_MapColor`) and the minimap pixels recolored through that table.

## Writing (`world.EncodeMap` / `SaveMap`)

The map editor writes `.WRL` files with `world.EncodeMap`,
the exact inverse of `ParseMap`: a file parsed with `ParseMap` re-encodes to the identical bytes (verified byte for byte across all 24 retail maps). `SaveMap(path,
m)` encodes and writes the file.

Two details make the round trip exact:

- **Raw palette.** `ParsePreview` divides each palette byte by 4 (8-bit to
6-bit),
which drops the low two bits;
the retail palettes are not all multiples of 4,
so the 6-bit copy alone cannot reproduce the file. `ParseMap` therefore also keeps the untouched 768 bytes in `Map.PaletteRaw`,
and `EncodeMap` writes those.
- **Pass table.** `ParseMap` maps the raw 0..3 byte per tile through `PassData`
to a surface flag (`Map.PassTable`);
`EncodeMap` inverts that flag back to the raw 0..3 value.
The four values map one to one,
so the inverse is exact.

Everything else (the magic/version bytes,
`W`/`H`,
minimap,
tile index layer,
tile count,
tile pixels) is copied back unchanged.

Parse with `ParseMap`,
not `LoadMap`,
when a byte exact result matters: `LoadMap` applies the hash keyed retail fixes (below),
which move `TileIDs`/`Surface` away from the on disk bytes.

## Color cycling

Terrain animation never touches pixels;
the game rotates palette ranges (`world.ColorCycles`,
C++ `World::InitColorCycles`):

| Range   | Dir   | FPS | Effect            |
|---------|-------|-----|-------------------|
| 9..12   | left  | 9   | water caustics    |
| 13..16  | right | 6   |                   |
| 17..20  | right | 9   |                   |
| 21..24  | right | 6   |                   |
| 25..30  | right | 2   |                   |
| 31      | pulse | 6   | beacon blink (green channel 63->7->63) |
| 96..102 | right | 8   | shore/water bands |
| 103..109| right | 8   |                   |
| 110..116| right | 10  |                   |
| 117..122| right | 6   |                   |
| 123..127| right | 6   |                   |

"Right" rotation moves every color to the next higher index (the last entry wraps to the first);
the single entry cycle halves the entry's green component per step,
resetting to 63 below 8.

## Known retail data errors

The reference implementation patches three maps after loading,
guarded by the SHA-256 of the file.
This project applies the first two (`world.LoadMap`):

- `GREEN_4.WRL` (`dbcfc449...`): tile 190's pass byte says water but the
art is land,
so the surface map (not the pass table) is corrected.
- `SNOW_3.WRL` (`a625d409...`): the tile id at grid (57,57) is wrong;
replaced with 55 *after* the surface map is built (so the surface keeps the old tile's type,
like the original).
- `SNOW_5.WRL`: the reference implementation injects 4 KB of handmade replacement art for a
corrupt tile.
Not ported: one tile on one map renders as the shipped,
slightly garbled data.

## Other notes

- The minimap is stored as `width` strips of `height` bytes; the
engine reads them straight into a row major buffer (for the square retail maps this is equivalent).
- All retail maps are 112x112. Non square maps would be scaled into
the 112x112 menu slot with centering (`Menu_LoadPlanetMinimap`).
- The original's low resolution mode (`DisableEnhancedGraphics`,
32x32 downsampled tiles) is not ported.
- The cargo map (per cell resource amounts) is game state initialized
to zero at load,
not file data;
it is filled in by the economy code.
