# miniaudio - no CGO Go binding

A Go binding for the [miniaudio](https://miniaud.io) audio engine that uses **no
CGO**. It loads a shared library at runtime and binds it through
[purego](https://github.com/ebitengine/purego), the same approach as the SoLoud
go-port in this workspace.

## Why there's a C wrapper (and no prebuilt download)

miniaudio is distributed only as a single C header and ships **no official
binaries** - it is meant to be compiled into your application. Its public API also
works on large, platform specific structs that are awkward to reproduce from a
language without CGO. So this binding does **not** bind `ma_*` directly. Instead:

```
capi/ma_capi.c   #define MINIAUDIO_IMPLEMENTATION + a thin flat "maw_*" C API
capi/ma_capi.h   opaque handle prototypes (engine, sound, group, stream, context)
        |  compiled together into one shared library
        v
go-port/bin/miniaudio_x64.dll   (Windows)
go-port/bin/libminiaudio_x64.so (Linux)
        |  loaded + bound at runtime via purego
        v
package miniaudio (this directory)
```

The wrapper hides every struct behind a `void*` handle and exposes only scalar /
buffer arguments, which is what makes a clean FFI binding possible. The audio
data callback lives entirely in C, so **no host language callback runs on the
realtime audio thread**.

## Build the shared library

miniaudio has no third party dependencies; it loads the OS audio backends
dynamically at runtime.

Windows (MinGW-w64, e.g. w64devkit):

```sh
# from extra-src/miniaudio/capi
CC="G:/data/max-go/tools/w64devkit/bin/gcc.exe" sh build_windows.sh
# -> ../go-port/bin/miniaudio_x64.dll
```

Linux (gcc/clang):

```sh
# from extra-src/miniaudio/capi, on a Linux host
sh build_linux.sh
# -> ../go-port/bin/libminiaudio_x64.so
```

## Usage

```go
if err := miniaudio.LoadPath(`bin\miniaudio_x64.dll`); err != nil {
    log.Fatal(err)
}
eng, _ := miniaudio.NewEngine()
defer eng.Destroy()

snd, _ := eng.NewSoundFromFile("explosion.wav", 0)
defer snd.Destroy()
snd.Start()
```

Push PCM streaming (e.g. decoded movie/music audio), without an engine:

```go
st, _ := miniaudio.NewStream(miniaudio.FormatS16, 2, 22050, 8192)
defer st.Destroy()
st.Start()
for chunk := range pcmChunks {       // []int16, interleaved stereo
    for st.WriteInt16(chunk) == 0 {  // ring buffer full; wait and retry
        time.Sleep(time.Millisecond)
    }
}
```

## Run

```sh
cd go-port
go build ./... && go vet ./... && go test ./...           # tests use a silent null engine
go run ./example -dll bin/miniaudio_x64.dll -wav some.wav  # audible
```

## License / attribution

This is a tribute binding. miniaudio (c) David Reid is dual licensed public domain /
MIT-0; see the upstream header. This binding follows the same no CGO philosophy as
the workspace's SoLoud port.
