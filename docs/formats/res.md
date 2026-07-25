# RES resource archive (MAX.RES, PATCHES.RES)

Interplay's resource archive format used by M.A.X.
All integers are little endian.
Data is stored uncompressed.

## Layout

```
+------------------+  offset 0
| header (12 B)    |
+------------------+  offset 12
| resource data    |  raw bytes, back to back
| ...              |
+------------------+  header.index_offset
| index table      |  header.index_size bytes (16 B per entry)
+------------------+  end of file
```

## Header (12 bytes)

Reference structure: `resource_manager.cpp` `res_header`.

| Offset | Size | Type | Field | Meaning |
| ---: | ---: | --- | --- | --- |
| `0` | `4` | `char[4]` | `id` | Magic, always `"RES0"` with no NUL terminator. |
| `4` | `4` | `int32` | `index_offset` | Absolute file offset of the index table. |
| `8` | `4` | `int32` | `index_size` | Index table size in bytes. Must be a multiple of 16. |

Number of resources = `index_size / 16`.

## Index entry (16 bytes)

Reference structure: `resource_manager.cpp` `res_index`.

| Offset | Size | Type | Field | Meaning |
| ---: | ---: | --- | --- | --- |
| `0` | `8` | `char[8]` | `tag` | Resource name, NUL terminated if shorter than 8 bytes. |
| `8` | `4` | `int32` | `data_offset` | Absolute file offset of the resource payload. |
| `12` | `4` | `int32` | `data_size` | Resource payload size in bytes. |

There is no per entry compression flag.
Data is stored exactly as the consumer expects it.
Some payload formats,
such as big images and multi frame sprites,
carry their own compression inside the resource body.

### Tag quirks

- The tag field is NUL terminated within its 8 bytes; bytes after
the first NUL are garbage and must be ignored (e.g.
PATCHES.RES contains `ILOGO\0/2`).
An 8-character tag has no terminator.
- Tags are uppercase in practice; treat lookups ignoring case.

## Reading a resource

1. Read the 12-byte header, check `id == "RES0"`.
2. Seek to `index_offset`, read `index_size` bytes, split into
16-byte entries.
3. To read a resource: find its entry by tag, seek to `data_offset`,
read `data_size` bytes.
The engine appends a NUL byte for text resources;
this project keeps the data as it is (`ReadString` trims trailing NULs instead).

The Go reader (`internal/resfile`) also validates that the index table and every entry span stay inside the archive file.
It skips empty tags and keeps the first entry if a tag appears more than once.

## Layering (how PATCHES.RES overrides MAX.RES)

The original loader (`ResourceManager_BuildResourceTable`,
`resource_manager.cpp`) loads PATCHES.RES first,
then MAX.RES,
and records each tag only on its first occurrence.
A tag present in both files therefore resolves to the PATCHES.RES copy.
Duplicate tags within a single archive behave the same way: first entry wins.

The Go `OpenDir` helper mirrors this as an ordered library: optional `PATCHES.RES` is opened first,
then required `MAX.RES`.
Lookup returns the first archive that contains the normalized tag.

## Resource IDs

The archive stores only 8-byte tags.
Numeric `ResourceID` values are a reference enum-layer convenience in `resourcetable.hpp`,
with the parallel string table in `resourcetable.cpp`.
The reference implementation resolves an archive tag to an enum by comparing the tag text against that string table after reading the index.
This project mostly uses string tags directly through `resfile.Library.Read`.

## Common image payloads

The RES archive does not type its entries.
Image consumers pick a decoder based on the known resource tag or call site.
The struct names below come from `resource_manager.hpp`.

### `ImageSimpleHeader`

Simple UI images are raw pixels after an 8-byte geometry header.

| Offset | Size | Type | Field | Meaning |
| ---: | ---: | --- | --- | --- |
| `0` | `2` | `int16` | `width` | Pixel width. |
| `2` | `2` | `int16` | `height` | Pixel height. |
| `4` | `2` | `int16` | `ulx` | Draw hotspot X; subtracted by the draw helper. |
| `6` | `2` | `int16` | `uly` | Draw hotspot Y; subtracted by the draw helper. |
| `8` | `width*height` | `uint8[]` | `pixels` | Row major palette indices. |

The reference implementation declares a packed 9-byte struct with a `transparent_color` byte at offset 8,
but the pixel data starts at that byte.
In practice the "transparent color" is the first pixel.
Go documents and decodes this as an 8-byte header in `internal/sprites.DecodeSimple`.

### `ImageBigHeader`

Big menu/background images carry their own palette and a per row RLE stream.

| Offset | Size | Type | Field | Meaning |
| ---: | ---: | --- | --- | --- |
| `0` | `2` | `int16` | `ulx` | Default draw X. |
| `2` | `2` | `int16` | `uly` | Default draw Y. |
| `4` | `2` | `int16` | `width` | Pixel width. |
| `6` | `2` | `int16` | `height` | Pixel height. |
| `8` | `768` | `uint8[256][3]` | `palette` | RGB palette with 8-bit components. |
| `776` | variable | bytes | stream | RLE image stream. |

The reference packed struct size is 777 for the same reason as SIMPLE: the declared `transparent_color` byte is the first byte of the RLE stream.
The Go decoder starts the stream at offset 776.
The embedded palette is converted to the engine's 6-bit palette range by dividing every component by 4.

The RLE stream is decoded row by row.
Each run starts with a signed little endian `int16`: positive means copy that many literal bytes;
negative means repeat the next byte `-n` times;
zero is invalid.

### `ImageMultiHeader`

Multi-frame unit sprites and shadows use a pointer-style layout in reference memory,
but the resource body stores offsets relative to the start of the resource.

| Offset | Size | Type | Field | Meaning |
| ---: | ---: | --- | --- | --- |
| `0` | `2` | `uint16` | `image_count` | Number of frames. |
| `2` | `4*image_count` | `uint32[]` | `frame_offsets` | Absolute offsets within this resource body. |

Each frame starts at its table offset:

| Offset | Size | Type | Field | Meaning |
| ---: | ---: | --- | --- | --- |
| `0` | `2` | `int16` | `width` | Frame width. |
| `2` | `2` | `int16` | `height` | Frame height. |
| `4` | `2` | `int16` | `hotx` | Draw hotspot X. |
| `6` | `2` | `int16` | `hoty` | Draw hotspot Y. |
| `8` | `4*height` | `uint32[]` | `row_offsets` | Absolute row data offsets within this resource body. |

Sprite row data is a sequence of spans terminated by byte `0xFF`.
A normal sprite span is:

```
transparent_count uint8
pixel_count       uint8
pixel bytes       uint8[pixel_count]
```

Shadow rows omit the pixel bytes:

```
transparent_count uint8
shadow_count      uint8
```

Go decodes these in `internal/sprites.DecodeMultiSprite` and `DecodeMultiShadow`.
More image specific detail is in `images.md` and `sprites.md`;
this section exists here because those payloads are the most common RES entry bodies.

## Current Go behaviour / limits

- `internal/resfile.OpenFile` validates archive bounds and reads resources by tag.
- `internal/resfile.OpenDir` matches `MAX.RES` and `PATCHES.RES` ignoring case for Linux data dirs.
- `internal/restool` can list, extract, convert, and repack uncompressed `RES0` archives. Repacking writes
the same header/index shape: header placeholder,
payloads,
index table,
then the final header.
- The archive layer does not know resource types. Callers decode text, images, sounds, tables, or filenames
from the returned bytes.
- No compressed entry flag exists in the archive format. Do not add one when packing.

## Observed data (game 1.0.4, reference implementation 0.7.2)

- `MAX.RES`: 1828 entries, index at end of file.
- `PATCHES.RES`: 5 entries: `FONT_1`, `FONT_2`, `FONT_5`, `ILOGO`,
`SC_SCHEM`.
