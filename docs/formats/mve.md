# MVE: Interplay movie files

Interplay MVE files are chunked 8-bit palettized movies.
M.A.X. uses them for intro/cutscene playback (`MAXINT.MVE`,
`MAXMVE1.MVE`,
`MAXMVE2.MVE`).
All integers are little endian.

## Header

| Offset | Size | Type | Field | Meaning |
|--------|------|------|-------|---------|
| 0      | 20   | char | tag   | `Interplay MVE File\x1a\x00` |
| 20     | 2    | u16  | separator | `0x001a` |
| 22     | 2    | u16  | version | `0x0100` |
| 24     | 2    | u16  | checksum | `(^version + 0x1234) & 0xffff` |
| 26     | 2    | u16  | next_size | byte size of the first record |
| 28     | 1    | u8   | next_opcode | record type |
| 29     | 1    | u8   | next_version | record version |

After the file header,
each record is stored as
ext_size` bytes of opcode stream followed by the next 4-byte record control block.
This matches `ioNextRecord` in `mvelib32.c`.

## Opcode stream

Each opcode inside a record has:

| Offset | Size | Type | Field | Meaning |
|--------|------|------|-------|---------|
| +0     | 2    | u16  | size | payload byte size |
| +2     | 1    | u8   | opcode | operation |
| +3     | 1    | u8   | version | operation version |
| +4     | size | u8[] | payload | operation data |

M.A.X. retail movies use these opcodes:

| Opcode | Meaning |
|--------|---------|
| `0x00` | end of stream |
| `0x01` | end of record; load the next record |
| `0x02` | timer: `u32 rate`, `u16 divider`; delay is `rate*divider/1000` ms |
| `0x03` | audio allocation; version 1 includes flags, sample rate and buffer size |
| `0x04` | audio sync marker |
| `0x05` | video allocation; width/height are counts of 8x8 blocks |
| `0x07` | show current video frame |
| `0x08` | audio frame |
| `0x09` | silence audio frame |
| `0x0a` | initialize graphics mode |
| `0x0c` | load palette: `u16 start`, `u16 count`, then `count*3` RGB bytes |
| `0x0f` | packed video decoding map; two 4-bit block modes per byte |
| `0x11` | packed video frame data |
| `0x13` | ignored by the reference decoder for these movies |
| `0x15` | ignored by the reference decoder for these movies |

Unsupported opcodes are decoder errors.

## Video

The retail M.A.X. movies allocate `54x40` blocks,
producing a `432x320` 8-bit frame.
Opcode `0x11` payload:

| Offset | Size | Type | Field | Meaning |
|--------|------|------|-------|---------|
| 0      | 2    | u16 | unused0 | ignored |
| 2      | 2    | u16 | unused1 | ignored |
| 4      | 2    | u16 | x | block X |
| 6      | 2    | u16 | y | block Y |
| 8      | 2    | u16 | width | block count |
| 10     | 2    | u16 | height | block count |
| 12     | 2    | u16 | flags | bit 0 swaps current/previous buffers |
| 14     | ...  | u8[] | data | block payloads consumed by the map |

The decoding map (`0x0f`) stores one byte for every two blocks: low nibble is the first block,
high nibble is the second.
M.A.X. uses block modes `0..5` and `7..15`;
mode `6` was not observed.
The Go decoder ports the packed 8-bit path from
fPkDecomp`.

## Audio

The retail files use version-1 audio allocation with flags `43679`: stereo,
16-bit,
compressed,
sample rate 22050 Hz.
Audio frame payload:

| Offset | Size | Type | Field | Meaning |
|--------|------|------|-------|---------|
| 0      | 2    | u16 | id | ignored by current decoder |
| 2      | 2    | u16 | mask | track mask; M.A.X. uses track bit 1 |
| 4      | 2    | u16 | length | decompressed byte length |
| 6      | ...  | u8[] | data | compressed deltas or raw PCM |

Compressed 16-bit audio stores initial sample values followed by 8-bit delta codes mapped through the Interplay `snd_8to16` table.
Stereo streams alternate left/right deltas.
