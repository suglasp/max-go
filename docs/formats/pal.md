# Palettes: COLOR.PAL, INTROPAL.PAL, and where the screen palette comes from

The game is fully 8-bit palettized.
This file documents the COLOR.PAL file format,
the plain 256-color palettes used by intro/movie resources,
and the runtime palette conventions shared by all the other formats.
All multi byte integers are little endian.

## Palette sources at runtime

One 256-entry system palette drives the display,
and its content changes per screen:

1. COLOR.PAL is loaded at startup by `Color_Init` (color.cpp). It is the
default game palette and the source of the lookup tables below.
2. INTROPAL.PAL and movie palette chunks are plain 768-byte, 256-entry
palettes.
They do not carry COLOR.PAL's RGB555 lookup table or optional intensity table.
3. Big images (menu backgrounds: MAINPIC, PLANETSE, ...) embed their own
768-byte palette (8-bit components,
divided by 4 on load) and install it when drawn.
Every menu screen swaps the system palette this way (`WindowManager_SwapSystemPalette`).
4. .WRL maps embed a palette (8-bit, /4) for in game terrain. For the
planet selection previews the world palette is not installed: each entry is remapped to the nearest color of the current menu palette (`Color_MapColor` full scan) and the minimap pixels are recolored through that table.

All stored palettes use 6-bit components (0..63) after load.
The display path multiplies by 4,
so full red shows as 252,
not 255 (this project keeps this faithful,
see `gfx.Palette.RGBA8`).

## Layout

| Offset | Size   | Field           | Meaning                                  |
|--------|--------|-----------------|------------------------------------------|
| 0      | 768    | palette         | 256 entries x 3 bytes (R, G, B)          |
| 768    | 32768  | rgb555 index    | RGB555 value -> nearest palette index     |
| 33536  | 4      | tag (optional)  | `'NEWC'` (0x4E455743) if a table follows |
| 33540  | 65536  | intensity table | `[color][level]` -> palette index         |

## Palette

Components are 6-bit (0..63),
VGA DAC style.
Entry 0 is black,
entry 1 is full red.

Two index ranges are special (animated/reserved): 9..31 and 96..127 are used for color cycling (water animation rotates entries by mutating the palette,
no pixel work) and team colors.
Nearest color searches (`Color_MapColor`) skip them unless a full scan is requested.

### Index conventions used by the UI

Well known palette indices the menu code relies on (stable across the menu palettes):

| Index | Meaning                                            |
|-------|----------------------------------------------------|
| 0     | black, also the transparent color in sprite blits  |
| 1     | `COLOR_RED`, selection boxes                       |
| 2     | `COLOR_GREEN`, lime green menu display text        |
| 2/3, 9..31, 64..159 | menu/simple image transparent key candidates when they appear as the image corner color; index 17 is a common case |
| 165/175/199 | `Fonts_GoldColor` base/bevel/shadow (button captions) |
| 5/56/199    | `Fonts_DarkOrageColor`, pressed button captions  |
| 162/172/199 | `Fonts_BrightSilverColor`                  |
| 255   | white, plain dialog text                           |

Font colors are triples (base,
inner bevel,
drop shadow) consumed by the `Text_TextLine` bevel+shadow pass (`fonts.BlitShadowed`).

### Transparent keys

The final transparent pixel value used by `gfx.TransCopy`,
transparent windows,
GNW buttons and scaled transparent blits is palette index 0.
Most decoded buffers therefore have to be normalized before they are drawn transparently.

Simple images carry their transparent key implicitly at the first pixel: the original `ImageSimpleHeader` overlaps its `transparent_color` field with the start of pixel data,
so `sprites.Simple.TransparentColor()` is the top left pixel.
The Go helper `gfx.Buffer.KeyToZero(key)` remaps every pixel with that key to 0,
after which a normal transparent blit skips it.
This is why black corners usually mean "the key was not normalized" rather than "black should be treated as transparent".

Menu/button helpers are intentionally more selective. `menus.fuchsiaKey` treats dynamic/menu fuchsia key slots (`2`,
`3`,
`9..31`,
and `64..159`) as keyed corner colors and leaves stable bevel colors (for example 217/218 on solid plates) opaque.
Palette index 17 sits in that keyed range,
so an image whose corner pixel is 17 must be run through `KeyToZero(17)` before a `TransCopy` (or a GNW button with the `ButtonTransparent` flag set,
which routes `blitButton` through `TransCopy` the same way).

## RGB555 index table

32768 bytes.
Index with `r<<10 | g<<5 | b` where r,
g,
b are 5-bit components (6-bit palette values shifted right once).
Each byte is the palette index whose color is nearest to that RGB value.
Used by `Color_RGB2Color`.

## Intensity table (optional)

Present only when the `'NEWC'` tag follows the RGB555 table. 256x256 bytes: `table[color][level]` is the palette index of `color` scaled to intensity `level` (callers pass `intensity/512` as level,
e.g.
GNW's `lighten_buf` uses intensity `0x12600`,
level 147).

The retail 1.0.4 COLOR.PAL is 33,536 bytes: palette plus RGB555 table only,
no intensity table.
The original engine then leaves the table zeroed (every lookup returns palette index 0).
The port's `ColorIntensity` (which would return the input color unchanged without a table, to avoid blacked out
lighten effects) sits in an unused code note block in `internal/gfx/palette.go`: no screen calls it.

## Plain 768-byte palettes

Several palette sources other than COLOR.PAL are just 256 RGB triplets with no extra tables. `INTROPAL.PAL`,
embedded big image palettes,
MVE palette chunks,
and WRL terrain palettes all use this shape.

The stored component range depends on the container:

| Source | Stored range | Go conversion |
|--------|--------------|---------------|
| `INTROPAL.PAL` / big images / WRL palette | 0..255 | divide by 4 into 0..63 |
| FLC `COLOR_256` chunk | 0..255 | shift right 2 into 0..63 |
| FLC `COLOR_64` chunk | 0..63 | copy unchanged |
| MVE palette chunk | 0..63 in the current decoder path | copy unchanged |

Current Go behaviour: `gfx.ParsePalFile` parses only the COLOR.PAL shaped file with lookup tables.
Plain palettes are decoded at their call sites (image/resource conversion,
world loading,
FLC/MVE playback) because those containers decide whether the bytes are 8-bit or already 6-bit.

## Generated tables (not in the file)

`color.cpp` derives further 256-byte recolor tables from the palette at runtime (ported as `Palette.IntensityTable1/2/3`):

1. Table1 blends each entry towards a target RGB with weights
`factor1:(factor1+factor2)`,
snapping to the nearest palette entry by full search.
2. Table2 tints towards a hue while keeping brightness (used for team
color tinting),
snapping via `MapColor` (restricted scan).
3. Table3 scales brightness per channel by `level/factor` with a floor
of `level/4` (used for darkening,
e.g. fog of war).
