# FLI/FLC Animations

M.A.X. stores unit portraits and small UI animations as Autodesk Animator FLC files.
All retail files observed in the data directory are 8-bit palettized FLC streams (`type = 0xAF12`) with little endian fields.
The decoder also accepts FLI headers (`0xAF11`) for tooling/debug use.

## File Header

The fixed header is 128 bytes.

| Offset | Size | Field | Meaning |
| --- | ---: | --- | --- |
| 0x00 | 4 | size | Total file size in bytes |
| 0x04 | 2 | type | `0xAF12` FLC, `0xAF11` FLI |
| 0x06 | 2 | frames | Number of playable frames |
| 0x08 | 2 | width | Frame width in pixels |
| 0x0A | 2 | height | Frame height in pixels |
| 0x0C | 2 | depth | Bits per pixel; M.A.X. uses 8 |
| 0x0E | 2 | flags | Format flags, unused by the Go decoder |
| 0x10 | 4 | speed | Frame delay; FLC stores milliseconds |
| 0x14 | 2 | reserved1 | Unused |
| 0x16 | 4 | created | Timestamp/metadata |
| 0x1A | 4 | creator | Metadata |
| 0x1E | 4 | updated | Timestamp/metadata |
| 0x22 | 4 | updater | Metadata |
| 0x26 | 2 | aspect_dx | Pixel aspect metadata |
| 0x28 | 2 | aspect_dy | Pixel aspect metadata |
| 0x2A | 38 | reserved2 | Unused |
| 0x50 | 4 | oframe1 | Offset of the first playable frame |
| 0x54 | 4 | oframe2 | Offset of the second frame or metadata |
| 0x58 | 40 | reserved3 | Unused |

Retail FLC files have a prefix chunk at offset `0x80` with frame type `0xF100`;
the in game stream begins at `oframe1`.
This matches `flicsmgr_read` in the C++ reference.

## Frame Header

Each playable frame starts with a 16-byte header.

| Offset | Size | Field | Meaning |
| --- | ---: | --- | --- |
| 0x00 | 4 | size | Frame chunk size including this header |
| 0x04 | 2 | type | `0xF1FA` frame; `0xF100` accepted for FLI compatibility |
| 0x06 | 2 | chunks | Number of subchunks |
| 0x08 | 8 | reserved | Unused |

The frame payload is a sequence of subchunks.
Each subchunk has `{size u32,
type u16}` followed by `size - 6` bytes of data.

## Subchunks

| Type | Name | Meaning |
| ---: | --- | --- |
| 4 | COLOR_256 | Palette packet data with 8-bit RGB components; the game shifts each component right by 2 into its 6-bit system palette |
| 7 | DELTA_FLC | Word oriented delta compression used by animated frames |
| 11 | COLOR_64 | Palette packet data already in 6-bit RGB components |
| 12 | DELTA_FLI | Byte oriented FLI delta compression; implemented for compatibility |
| 13 | BLACK | Fill the frame with palette index 0 |
| 15 | BYTE_RUN | Per line RLE used by key frames |
| 16 | LITERAL | Raw `width * height` palette indices |
| 18 | PSTAMP | Thumbnail/preview metadata; present in retail files and ignored for playback |

Retail scan result: the loose `.FLC` files use `PSTAMP`,
`COLOR_256`,
`BYTE_RUN`,
and `DELTA_FLC` in the playable stream.

## Palette

The active palette is persistent across frames. `COLOR_256` and `COLOR_64` chunks contain packets:

| Size | Field | Meaning |
| ---: | --- | --- |
| 2 | packet_count | Number of palette packets |
| 1 | skip | Entries to skip from the current palette index |
| 1 | count | Entries to update; 0 means 256 |
| 3 * count | RGB | New palette entries |

The Go decoder returns palettes as 768 bytes of 6-bit RGB components (`0..63`) so they can be applied directly to `internal/gfx.Palette`.

## Timing

For FLC files,
`speed` is treated as milliseconds per frame.
For FLI compatibility,
`speed` is converted from 1/70-second ticks to milliseconds.
The in game UI often advances FLC portraits at fixed rates such as 12.5 or 15 FPS,
so callers may override the returned `DelayMS` where a particular UI requires C++-exact timing.

## Current Go behaviour / limits

`internal/flic.Open` is a streaming decoder.
It keeps the previous frame pixels and palette internally,
then returns a copy of the decoded 8-bit pixel buffer and the current 6-bit palette for each `NextFrame` call.
This matches the way FLC delta frames build on prior frames.

Implemented chunk types are the ones needed by the retail portrait/UI animations plus FLI compatibility: `COLOR_256`,
`COLOR_64`,
`DELTA_FLC`,
`DELTA_FLI`,
`BLACK`,
`BYTE_RUN`,
`LITERAL`,
and ignored `PSTAMP`.
Unknown chunk types are fatal so unsupported retail or modded files fail loudly instead of rendering corrupted frames.

The decoder validates positive geometry,
8-bit depth,
chunk sizes,
palette packet bounds,
row/column bounds in delta chunks,
and literal/byte run payload lengths.
It does not implement Autodesk metadata beyond the frame offset and timing fields,
and it does not do playback scheduling itself;
menu/widgets own frame pacing.
