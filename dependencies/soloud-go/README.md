# SoLoud Go port (no CGO)

A Go port of the SoLoud audio engine C API (`include/soloud_c.h`),
SoLoud (c) 2013-2020 Jari Komppa, http://soloud audio.com

This binding does **not** use CGO. It loads the SoLoud shared library at
runtime (`bin/soloud_x64.dll` on Windows) and binds every function of the
C API through [github.com/ebitengine/purego](https://github.com/ebitengine/purego),
a pure Go FFI library. A plain `go build` is all you need - no C compiler.

## Quick start

```go
package main

import (
    "log"
    "time"

    "soloud"
)

func main() {
    // Load the shared library first (soloud.Load() searches PATH for the
    // platform default name instead).
    if err := soloud.LoadPath(`..\bin\soloud_x64.dll`); err != nil {
        log.Fatal(err)
    }

    engine := soloud.NewSoloud()
    defer engine.Destroy()
    if err := engine.Init(); err != nil {
        log.Fatal(err)
    }
    defer engine.Deinit()

    speech := soloud.NewSpeech()
    defer speech.Destroy()
    speech.SetText("Hello from Go")
    engine.Play(speech)

    for engine.GetActiveVoiceCount() > 0 {
        time.Sleep(50 * time.Millisecond)
    }
}
```

Try it: `go run ./example -dll ..\bin\soloud_x64.dll`

Run the smoke test (uses the silent NoSound backend): `go test ./...`

## What is ported

Everything in `soloud_c.h`, wrapped in Go types in the style of the other
glue bindings (soloud.cs, soloud.py, ...):

| C API class | Go type | Constructor |
|---|---|---|
| `Soloud` | `Soloud` | `NewSoloud()` |
| `Bus` | `Bus` | `NewBus()` |
| `Wav`, `WavStream` | `Wav`, `WavStream` | `NewWav()`, `NewWavStream()` |
| `Speech`, `Vizsn` | `Speech`, `Vizsn` | `NewSpeech()`, `NewVizsn()` |
| `Sfxr`, `Noise`, `Monotone`, `Openmpt`, `Queue`, `TedSid`, `Vic` | same names | `New...()` |
| `BassboostFilter` ... `WaveShaperFilter` (all 10 filters) | same names | `New...()` |

All `SOLOUD_ENUMS` constants are in `enums.go` with their original names
(`SOLOUD_CLIP_ROUNDOFF`, `SFXR_COIN`, `BIQUADRESONANTFILTER_LOWPASS`, ...).

Conventions used by the port:

- C functions become methods: `Soloud_playClocked` -> `(*Soloud).PlayClocked`,
  `Wav_load` -> `(*Wav).Load`. The `...Ex` variants are kept, since Go has no
  default arguments either.
- C `int` result codes become Go `error` (`nil` on success, otherwise a
  `SoloudError` such as `FILE_LOAD_FAILED`).
- C `int` booleans (`aPaused`, `aLoop`, `aEnable`, ...) become Go `bool`.
- `AudioSource*` parameters take the `soloud.AudioSource` interface, which all
  playable types (including `Bus`) implement; `Filter*` parameters take the
  `soloud.Filter` interface.
- `loadMem`-style functions take `[]byte` and always pass `aCopy = true` to
  SoLoud, so the native side never holds on to Go managed memory.
- `calcFFT`/`getWave` return a copied `[]float32` of 256 values.
- Objects are created with `New...()` and freed with `Destroy()`; there are no
  finalizers, freeing is your responsibility (same as in the C API).
- `File*` and `AudioCollider*`/`AudioAttenuator*` parameters are opaque native
  pointers (`uintptr`); the C API offers no way to construct them.

## Platform notes

- Windows: needs `soloud_x64.dll` (64-bit Go). The 32-bit `soloud_x86.dll`
  cannot be used - purego does not support windows/386.
- The DLL picks its backend at `Init()` time (SDL2 if `SDL2.dll` is found,
  otherwise WinMM, etc.). Keep `SDL2.dll` next to the SoLoud DLL or let it
  fall back.
- Linux/macOS: build SoLoud as a shared library (`libsoloud.so` /
  `libsoloud.dylib`) and pass its path to `soloud.LoadPath`.
- `soloud.LoadPath`/`soloud.Load` must be called once before any other
  function of the package.
