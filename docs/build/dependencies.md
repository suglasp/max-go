# Build Dependencies

This repository keeps the SoLoud and miniaudio integrations self contained under `dependencies/` so a fresh checkout does not need a sibling source folder.

## Layout

| Path | Purpose |
|------|---------|
| `dependencies/soloud-go/` | Local Go module used by `replace soloud => ./dependencies/soloud-go` in `go.mod`. |
| `dependencies/soloud-bin/` | Native SoLoud shared libraries and license/readme files copied next to built binaries. |
| `dependencies/miniaudio-go/` | Local Go module used by `replace miniaudio => ./dependencies/miniaudio-go` in `go.mod`. |
| `dependencies/miniaudio-bin/` | Native miniaudio wrapper shared libraries and license copied next to built binaries. |

The old workspace local path `../soloud_20200207 - goport/go-port` is not required for normal builds.

## Build Scripts

There are two steps: build (compiles to `build\`) and stage (copies the runnable set into `bin\`).

Windows:

```bat
build-windows.cmd
stage-windows.cmd
```

Linux/macOS style shells:

```bash
./build-linux.sh
./stage-linux.sh
```

Both build scripts build:

- `cmd/max-go`
- `cmd/max-restool`
- `cmd/max-convert`
- `cmd/max-settings`

The build scripts write CLI help text files and copy the native libraries from `dependencies/soloud-bin/` and `dependencies/miniaudio-bin/`,
`README.md`,
`lang-en-us.ini`,
and `max-go.json` into `build\`.
The stage scripts then copy the built executables,
help text,
native libraries,
`README.md`,
`lang-en-us.ini`,
and (only if `bin\max-go.json` doesn't already exist,
so a live settings file is never clobbered) `max-go.json` from `build\` into `bin\` - `bin\` is the folder to actually run from.

To build artifacts without launching the game:

```bat
set MAX_GO_BUILD_ONLY=1
build-windows.cmd
```

```bash
MAX_GO_BUILD_ONLY=1 ./build-linux.sh
```

## Notes

- `dependencies/soloud-go/` and `dependencies/miniaudio-go/` contain only the Go
bindings (no CGO).
They are source code and must stay in sync with the native library ABI in the matching `*-bin/` folder.
- `dependencies/soloud-bin/` contains the SoLoud binaries. On Windows,
`soloud_x64.dll`;
on Linux,
`libsoloud_x64.so`.
- `dependencies/miniaudio-bin/` contains the miniaudio wrapper binaries. On
Windows,
`miniaudio_x64.dll`;
on Linux,
`libminiaudio_x64.so`.
- `max-go.json` selects the audio backend: `ebiten` (native Ebitengine/Oto),
`soloud`,
or `miniaudio`.
The game falls back to Ebitengine when the chosen native backend is unavailable.
- Movie (MVE) audio routing per backend:

  | Backend | Music / SFX / voice | Movie (MVE) PCM |
  | --- | --- | --- |
  | `ebiten` | Ebitengine/Oto | Ebitengine/Oto |
  | `miniaudio` | miniaudio | miniaudio (native) |
  | `soloud` | SoLoud | Ebitengine/Oto fallback |

The `miniaudio` backend plays movie PCM natively through a continuous `ma_pcm_rb` ring buffer with a ~400ms preroll cushion (and a ~2s ring),
which rides out frame decode hitches and plays cleanly.
The `soloud` backend falls back to Ebitengine/Oto for movie audio only: SoLoud's sole streaming primitive is `Queue`,
a chain of discrete one shot sources rather than a continuous ring buffer,
so streamed movie PCM both underruns (silence on an empty queue) and clicks at per chunk resample boundaries.
Deepening a buffer cannot fix that;
native SoLoud movie playback would require a custom ring buffer `AudioSource` in the SoLoud C wrapper plus a DLL rebuild,
which buys nothing over the clean Oto fallback and is deliberately not done.
Result: all three backends deliver glitch free movie audio.
