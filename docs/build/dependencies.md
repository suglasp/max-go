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

There are two steps: build (compiles to a per platform folder under `build\`) and stage (copies the runnable set into `bin\`).

Each platform keeps its own output folder, so a Windows and a Linux build can sit side by side.
Both scripts also build what they can for the other platform:

| Script | Native | Cross built |
|------|---------|---------|
| `build-windows.cmd` | `build\windows-x64\`, game + utilities | `build\linux-x64\`, game + utilities |
| `build-linux.sh` | `build/linux-x64/`, game + utilities | `build/windows-x64/`, game + utilities |

Ebitengine 2.10 is pure Go on both platforms, so each side cross-builds the other with `CGO_ENABLED=0`.
On Linux it drives X11 and loads OpenGL at run time, so the Linux binary needs only libc plus the X11 and GL libraries of the desktop.
The three utilities are pure Go on both platforms and always cross build.
Help text comes from running each binary, so a cross-built folder has none.

`build-linux.sh` under MSYS or Git Bash on Windows builds Windows natively and skips the cross step.
The stage scripts read the folder for the platform they run on and copy it into `bin\`, which is the native folder you actually run from.

### Release archives

Each build packs what it has into `release/`, named after the first line of the `version` file:

| Archive | From |
|------|---------|
| `max-go-<version>-windows-x64.zip` | `build/windows-x64/` |
| `max-go-<version>-linux-x64.tar.zst` | `build/linux-x64/` |

An archive is written only when its folder contains the game binary.
On Linux, packing uses `zip` and `zstd`; a missing tool is reported and skipped.
On Windows the Linux tarball is written by `pack-linux-tar.py` instead of `tar.exe`, so it needs `python` on PATH.
`tar.exe` reads permissions from the filesystem. NTFS has no execute bit, so its archives unpack binaries without executable permissions.
The packer sets `0755` on the executables and the shared libraries and `0644` on everything else, with uid and gid `0`.
Set `MAX_GO_BUILD_ONLY=1` to build without packing.

The native output folder is cleared before its build so stale files do not enter an archive.
The cross-built folder preserves files the running script cannot regenerate; only the other platform's audio libraries are cleared.

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
and `max-go.json` into the platform folder, and pack `LANG.RES` (UI strings + unit descriptions) from `langres/` with the freshly built `max-restool`.
The stage scripts then copy the built executables,
help text,
native libraries,
`README.md`,
`max-lavender.png`,
`LANG.RES`,
and (only if `bin\max-go.json` doesn't already exist,
so a live settings file is never clobbered) `max-go.json` from `build\windows-x64\` or `build/linux-x64/` into `bin\` - `bin\` is the folder to actually run from.

To build the artifacts without packing release archives:

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
