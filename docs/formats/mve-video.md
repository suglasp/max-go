# MVE video streams

Interplay MVE movies (`*.MVE`) are chunked animation streams used for intro and cutscene playback.
This page documents the container and the 8-bit video path used by the M.A.X. movie files.
All integers are little endian.

Reference implementation: `mvelib32.c` (`MVE_rmStepMovie`,

fPkDecomp`).

## File header

| Offset | Size | Type    | Field            | Meaning                         |
|--------|------|---------|------------------|---------------------------------|
| 0      | 20   | char[20]| tag              | `Interplay MVE File\x1A`         |
| 20     | 2    | uint16  | magic separator  | `0x001A`                         |
| 22     | 2    | uint16  | format version   | `0x0100`                         |
| 24     | 2    | uint16  | checksum         | `^format_version + 0x1234`       |
| 26     | 2    | uint16  | next chunk size  | byte count of the first chunk    |
| 28     | 1    | uint8   | next chunk type  | carried by the chunk header      |
| 29     | 1    | uint8   | next chunk ver   | carried by the chunk header      |

The first chunk header is embedded in the file header.
Every later chunk is stored as:

| Offset | Size | Type   | Field | Meaning                         |
|--------|------|--------|-------|---------------------------------|
| 0      | n    | bytes  | body  | opcode records                  |
| +n     | 2    | uint16 | size  | next chunk body size            |
| +      | 1    | uint8  | type  | next chunk type                 |
| +      | 1    | uint8  | ver   | next chunk version              |

## Opcode records

Each chunk body is a sequence of opcode records:

| Offset | Size | Type   | Field   | Meaning                         |
|--------|------|--------|---------|---------------------------------|
| 0      | 2    | uint16 | size    | payload size, excluding header  |
| 2      | 1    | uint8  | opcode  | operation id                    |
| 3      | 1    | uint8  | version | operation version               |
| 4      | size | bytes  | payload | opcode specific data            |

Opcode `0x01` ends the current record.
The reference decoder then loads the next chunk and continues.
Opcode `0x00` ends the stream. `MVE_rmStepMovie` returns one displayed frame when it reaches opcode `0x07`.

## Video opcodes

| Opcode | Name                  | Payload                                  |
|--------|-----------------------|------------------------------------------|
| `0x02` | create timer          | `uint32 rate`, `uint16 divider`          |
| `0x05` | alloc video buffers   | `uint16 width`, `height`, `fqty`, `hicolor` |
| `0x06` | decomp video 0        | block rectangle + raw opcode-6 data      |
| `0x07` | show video frame      | `uint16 pal_start`, `pal_count`, `flags` |
| `0x0A` | init gfx mode         | `uint16 window_width`, `window_height`, `flags` |
| `0x0B` | synth palette         | six one byte palette synthesis fields    |
| `0x0C` | load palette          | `uint16 start`, `count`, `count*3` RGB   |
| `0x0D` | load compressed palette | compressed palette stream              |
| `0x0E` | set decoding map 1    | map bytes for changed block paths        |
| `0x0F` | set decoding map 2    | packed block mode nibbles                |
| `0x10` | decomp video 1        | block rectangle + changed block data     |
| `0x11` | decomp video 2        | block rectangle + packed block data      |

The Go port targets 8-bit output: frame pixels are palette indices and palette entries are 6-bit RGB components (`0..63`).
High color paths in the C decoder are not part of the M.A.X. runtime path.

### Timer

`create timer` stores presentation timing as `rate * divider` microseconds.
A frame API should expose this as a frame delay in milliseconds;
playback and sleeping stay outside the decoder.

### Video allocation

`alloc video buffers` configures two frame buffers.
The reference stores dimensions in pixels,
and `fqty` controls row scaling for the buffer layout.
The normal 8-bit path keeps a current and previous buffer;
block modes can copy from either.

### Palette loading

`load palette` writes `count` RGB triples into the movie palette at `start`.
MVE palette bytes are already 6-bit values in the reference decoder's palette table. `show video frame` publishes the active palette range around frame presentation;
a frame returned by a decoder should mark palette changes for the caller.

`synth palette` and `load compressed palette` are present in the format but are stubbed in `mvelib32.c`;
a decoder should reject them unless a target movie is proven to need them.

## Rectangle payloads

The decompression opcodes share this prefix:

| Offset | Size | Type   | Field    | Meaning                         |
|--------|------|--------|----------|---------------------------------|
| 0      | 2    | uint16 | unused 0 | ignored by the reference        |
| 2      | 2    | uint16 | unused 1 | ignored by the reference        |
| 4      | 2    | uint16 | x        | block X coordinate              |
| 6      | 2    | uint16 | y        | block Y coordinate              |
| 8      | 2    | uint16 | width    | width in 8x8 blocks             |
| 10     | 2    | uint16 | height   | height in 8x8 blocks            |
| 12     | 2    | uint16 | flags    | bit 0 advances current/previous |
| 14     | ...  | bytes  | data     | compressed block payload        |

Coordinates and sizes are in 8x8 blocks.
The decoded pixel rectangle is `x*8`,
`y*8`,
`width*8`,
`height*8`.

## Packed block video (`0x0F` + `0x11`)

The common path for 8-bit Interplay video is:

1. `0x0F` supplies a decoding map.
2. `0x11` supplies a rectangle and byte stream.
3.
fPkDecomp` consumes one nibble from the decoding map per 8x8
block and consumes additional bytes from the rectangle stream as required by the block mode.

The decoding map packs two block modes per byte: low nibble first,
then high nibble.
Blocks are visited left to right,
top to bottom in pairs.

| Mode | Meaning                                                |
|------|--------------------------------------------------------|
| `0`  | copy 8x8 block from previous frame at same position    |
| `1`  | leave current frame block unchanged                    |
| `2`  | copy from current frame using small signed offset      |
| `3`  | copy from current frame using negated small offset     |
| `4`  | copy from previous frame using small signed offset     |
| `5`  | copy from previous frame using explicit signed offset  |
| `6`  | skip a run of blocks                                   |
| `7`  | two color pattern                                      |
| `8`  | two color pattern split by halves/quadrants            |
| `9`  | four color pattern                                     |
| `10` | four color pattern split by halves/quadrants           |
| `11` | raw 8x8 pixels, 64 bytes                               |
| `12` | four 2x2 solid color quadrants, 16 bytes               |
| `13` | four 4x4 solid color quadrants, 4 bytes                |
| `14` | whole block filled with one byte                       |
| `15` | 2-byte checker/stripe fill                             |

Modes `2..5` depend on lookup tables in `mvelib32.c` (
f_pk_lut_2`,

f_pk_lut_3`) that expand compact motion codes into signed `(x,y)` offsets.
The C code computes byte offsets as `dx + stride*dy`,
optionally plus the previous/current buffer difference.

Pattern modes use the ordering in
fFillPattern`,

fFillPattern11` and
fFillPattern12`.
Do not infer a normal row major bit order from the packed bytes;
follow the reference bit to pixel mapping.

## Frame presentation

`show video frame` is the frame boundary.
It may temporarily clear or publish a palette range before presenting,
depending on `pal_count`,
double buffering state,
and decoding map use.
For a non interactive decoder,
the practical output is a copy of the current frame buffer,
the current palette,
whether the palette changed,
any audio gathered since the previous frame,
and the delay to the next frame.

## Notes

- `decomp video 0` is the older non packed path (
fDecomp`); it is a
separate opcode stream from
fPkDecomp`.
- `decomp video 1` and changed frame presentation are marked TODO in
the C reference.
- Unknown or stubbed opcodes should fail with a clear error when a
target movie uses them;
guessing tends to desynchronize the block stream.
