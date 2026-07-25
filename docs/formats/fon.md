# FONTn.FON: GNW bitmap fonts

The original game's text fonts,
stored as loose files `FONT0.FON` .. `FONT6.FON` in the game data directory.
All integers little endian.

Note: The reference implementation replaced GNW bitmap text with FreeType TTF rendering.
The `FONT_1`/`FONT_2`/`FONT_5` entries in its PATCHES.RES are TTF blobs,
not .FON data. max-go uses the original bitmap fonts and ignores those resources.

## Layout

The file is a raw dump of GNW's in memory font struct,
which is why it contains two meaningless pointer fields:

| Offset | Size | Type  | Field       | Meaning                            |
|--------|------|-------|-------------|-------------------------------------|
| 0      | 4    | int32 | glyph_count | 128 or 256                          |
| 4      | 4    | int32 | height      | glyph height in pixels              |
| 8      | 4    | int32 | spacing     | extra pixels after each glyph       |
| 12     | 8    | bytes | (junk)      | two raw pointers from the dump      |
| 20     | 8xN  |       | glyph table | per glyph: {int32 width, int32 offset} |
| 20+8xN | ...  |       | bitmap data | 1bpp glyph rows                     |

Glyph `offset` is relative to the start of the bitmap data section.
Each glyph stores `height` rows of `ceil(width/8)` bytes;
bits are MSB first (bit 7 = leftmost pixel).
Glyph index = byte value of the character (ASCII).

Loader validation in max-go:

- `glyph_count` must be 1..256.
- `height` must be 1..64.
- `spacing` must be 0..16.
- glyph width must be 0..64.
- each glyph bitmap range must stay inside the bitmap data section.

The 128-glyph fonts only cover byte values 0..127.
Drawing a byte outside a font's glyph table draws nothing and has zero width;
there is no fallback glyph.

## Retail files

| File  | Glyphs | Height | Spacing |
|-------|--------|--------|---------|
| FONT0 | 128    | 10     | 1       |
| FONT1 | 256    | 16     | 0       |
| FONT2 | 256    | 9      | 0       |
| FONT3 | 128    | 9      | 0       |
| FONT4 | 128    | 11     | 0       |
| FONT5 | 256    | 10     | 0       |
| FONT6 | 128    | 16     | 0       |

Heights match the FontMeta table in the reference `text.cpp` exactly.

## Rendering semantics (text.c)

- Only set bits are written; the background shows through.
- The color argument packs the palette index in the low byte plus
flags: outline 0x10000,
underline 0x20000,
monospace 0x40000.
- String width = sum of (glyph width + spacing).
- Monospace mode uses a fixed cell (widest glyph + spacing); max-go
centers each glyph in its cell.
- Underline paints the bottom glyph row across the string's advance
width.

Plain outline mode draws the glyph one pixel left,
right,
up,
and down in palette index 0,
then draws the foreground glyph in the low byte palette index.

`BlitShadowed` implements the classic GNW bevel/drop shadow pass used by menus.
It first draws the base text into a scratch buffer,
then scans bottom up: a base pixel whose down right neighbor is also base becomes the configured outline color there;
otherwise that down right neighbor becomes the configured shadow color.

Standard menu color triples in max-go:

| Name | Base | Outline | Shadow |
|------|------|---------|--------|
| Gold | 165 | 175 | 199 |
| Dark orange | 5 | 56 | 199 |
| Dark gray | 179 | 215 | 199 |
| Bright brown | 164 | 173 | 199 |
| Bright yellow | 162 | 8 | 199 |
| Bright silver | 162 | 172 | 199 |

## Runtime use

`fonts.LoadSet` loads exactly `FONT0.FON` through `FONT6.FON` from the selected game data directory.
Fonts are loose files,
not read through the `FONT_1` / `FONT_2` / `FONT_5` resource entries in PATCHES.RES because those entries are TTF replacements in the reference implementation.
