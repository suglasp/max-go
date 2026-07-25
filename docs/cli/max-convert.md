# max-convert

M.A.X. asset converter: turns game data into images,
sounds and text you can open in any ordinary viewer.

Archives themselves are [max-restool](max-restool.md)'s job.
Unpack a `.RES` with that tool first,
then point `max-convert res` at the directory.

## Usage

```text
max-convert help
max-convert res  [-format bmp|png] [-pal COLOR.PAL] [-path <dir>]
max-convert file [-format bmp|png] [-pal COLOR.PAL] <input> <outdir>
```

## Commands

```text
max-convert help
```

Prints command usage.

```text
max-convert res [-format bmp|png] [-pal COLOR.PAL] [-path <dir>]
```

Walks every file in a directory and writes each converted result **next to** the original.
The directory is `extracted` under the current folder,
which is where `max-restool` unpacks by default,
unless `-path` names another one.
So the whole job is usually two commands:

```text
max-restool -res MAX.RES
max-convert res
```

The raw files and the manifest are left untouched,
so `max-restool pack` still rebuilds a bit identical archive from the same directory.

Every file gets a line as it is handled:

```text
file ILOGO converted to ILOGO.bmp [bmp]
file REPAIR02.FLC converted to REPAIR02_0000.bmp [bmp] (+29 more)
file ILOGO.bmp already converted!
file SC_SCHEM failed to convert!
```

A file wearing an extension this tool writes (`.bmp`,
`.png`,
`.txt`,
`.wav`,
`.ttf`,
`.otf`,
`.ttc`) is output from an earlier run and is left alone.
A file nothing can be made of does not stop the walk.

The walk handles whatever it finds,
so `-path` can just as well point at a folder of loose game data holding `.MSC` music,
`.FLC` animations and `.MVE` movies.
What each kind becomes is the table under `file`,
below.

RES resources convert like this:

- **Images**. Big images use their embedded palette; simple images and multi
frame sprites are coloured with COLOR.PAL.
- **Multi frame sprites** become a single horizontal **strip** (one frame after
another);
shadow resources (`S_` prefix) render as black silhouettes.
- **Palettes** (a 256x3 table or a COLOR.PAL shaped resource) become a 256x1
colour strip,
one pixel per entry.
- **Fonts** (`FONT*`). A GNW bitmap `.FON` becomes an image of the printable
ASCII glyphs.
The TrueType and OpenType fonts in `PATCHES.RES` are already usable,
so they are written out verbatim as `.ttf`,
`.otf` or `.ttc`.
- **Text and sound filename references** (for example `MAIN_MSC` holding
`main_msc.msc`) become `.txt`.
- Anything else is left as raw bytes only.

When a palette is in play a `_palette` colour strip of it is written into the directory as well.

```text
max-convert file [-format bmp|png] [-pal COLOR.PAL] <input> <outdir>
```

Converts a single asset into `<outdir>`,
which is created if it does not exist.
The kind is worked out from the file extension,
and from the leading bytes when there is none:

| Input | Output |
| --- | --- |
| `.WAV`, `.MSC` | one `.wav` (both are RIFF/WAVE already, so this is a validated rename) |
| `.FLC`, `.FLI` | one image per frame, named `<base>_0000`, `<base>_0001`, ... |
| `.MVE` | one image per frame, plus `<base>.wav` holding the whole sound track |
| anything else | treated as a raw RES resource, as under `res` above |

## Flags

`-format` picks the image encoder and defaults to `bmp`.

BMP is written as uncompressed 24-bit and carries no alpha channel,
so a transparent pixel comes out fuchsia (255,
0,
255).
That is the same colour M.A.X. parks in its own transparent palette slot.
Use `-format png` when you need real transparency,
for instance to composite a sprite over something else.

`-pal` names the COLOR.PAL to colour simple images and multi frame sprites with.
Without it,
a `COLOR.PAL` beside the input is used if one is there,
and failing that sprites render in grayscale.
Big images,
palettes,
animations and movies carry their own colours and ignore this flag.

## Notes

Conversion is one way and for inspection only.
Nothing here reads a `.bmp` or a `.wav` back into game data.

Both subcommands take their dash style flags before their positional arguments.
