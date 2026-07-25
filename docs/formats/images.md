# Image resources (simple and big)

Two of the three image formats stored in MAX.RES (multi frame unit sprites are documented separately in sprites.md).
All integers are little endian.
Pixels are 8-bit palette indices.

A shared quirk: the C headers (`resource_manager.hpp`) declare a final `transparent_color` byte on both structs,
but the pixel data starts at that offset.
The field is really the first byte of the image data,
not a header field.

## Simple image

Raw,
uncompressed.
Used for buttons,
icons and other UI art.

| Offset | Size | Type  | Field | Meaning                          |
|--------|------|-------|-------|----------------------------------|
| 0      | 2    | int16 | width |                                  |
| 2      | 2    | int16 | height|                                  |
| 4      | 2    | int16 | ulx   | hotspot X, subtracted on draw    |
| 6      | 2    | int16 | uly   | hotspot Y, subtracted on draw    |
| 8      | w*h  | u8[]  | pixels| row major palette indices        |

The resource size is exactly `8 + width*height` (verified: BLANK_UP is 5808 = 8 + 200x29).

## Simple image transparency

There are two related rules that are easy to confuse:

1. GNW transparent blits skip palette index 0.
2. MAX simple image resources store their per image transparent key as the first
pixel of the pixel data (`transparent_color` in the reference struct).

In this project,
callers that need keyed transparency decode the image into a `gfx.Buffer`,
read `buf.At(0,
0)` or `Simple.TransparentColor()`,
call `KeyToZero(key)`,
then draw with `TransCopy` (or,
for a GNW button face,
pass the `ButtonTransparent` flag - `internal/gnw/button.go` - so `blitButton` calls `TransCopy` for that button).
This maps the resource's native key to the GNW transparent index before the blit.
Without that step,
keyed corner pixels render as solid colours.
This is the rule behind the UI/bar art fixes for resources such as scrollbar bars,
stat icons,
resource overlay markers,
and beveled buttons.

Common keyed UI art uses a fuchsia/bright key colour.
The exact palette index is varies per resource;
index 17 is one important case,
but the authoritative key is always the first pixel of the decoded simple image.
See `docs/formats/pal.md` for the palette index notes and `gfx.Buffer.KeyToZero` for the project helper.

When one image is *blended* over another (`Image::Blend`,
used by button states),
The reference implementation also treats the first pixel of the source image as the transparent colour.

## Big image

RLE compressed full screen art (menu backgrounds,
logos) carrying its own palette.

| Offset | Size | Type    | Field   | Meaning                          |
|--------|------|---------|---------|----------------------------------|
| 0      | 2    | int16   | ulx     | default draw position X          |
| 2      | 2    | int16   | uly     | default draw position Y          |
| 4      | 2    | int16   | width   |                                  |
| 6      | 2    | int16   | height  |                                  |
| 8      | 768  | u8[768] | palette | 256xRGB, 8-bit components        |
| 776    | ...  |         | stream  | RLE pixel stream                 |

The embedded palette uses 8-bit components (0..255);
the engine divides by 4 when installing it as the 6-bit system palette (`WindowManager_SwapSystemPalette`).

### RLE stream

Decoded row by row;
runs never cross a row boundary:

```
repeat until row has width pixels:
    opt = int16 (LE)
    opt > 0:  literal run: copy the next opt bytes
    opt < 0:  repeat run:  read 1 byte, write it -opt times
    opt == 0: invalid (would loop forever in the original decoder)
```

`C++: WindowManager_DecodeBigImage (window_manager.cpp)`

## Known examples (retail 1.0.4)

- `MAINPIC`: 640x480 big image, main menu background.
- `ILOGO`: 640x480 big image, Interplay logo; overridden by PATCHES.RES.
- `BLANK_UP` / `BLANK_DN`: 200x29 simple images, large menu button
up/down faces.
