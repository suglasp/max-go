# Audio backends: MVE movie sound and held loops

This note documents how two audio concerns behave **per audio backend** (`ebiten`/Oto,
`soloud`,
`miniaudio`):

1. **MVE movie sound** - how decoded movie (Interplay MVE) PCM reaches the device.
2. **Held loops** - the selected unit idle hum / building running hum, looped
without a gap at the loop boundary.

## 1. MVE movie sound

Movie audio is decoded by the MVE player and pushed to the audio layer as a stream of small signed 16 bit PCM chunks.
Two delivery paths exist;
the Manager picks one when the movie starts (`internal/audio/audio.go`: `StartMoviePCM` / `QueueMoviePCM` / `StopMoviePCM`).
A backend that implements the optional `MoviePCMBackend` interface (`StartPCM` / `QueuePCM` / `StopPCM`) gets the native path;
everything else uses the Manager's own Oto streaming player (`internal/audio/moviepcm.go`,
`moviePCMPlayer`).

### ebiten (Oto)
Movie PCM streams through the Manager's `moviePCMPlayer` (`internal/audio/moviepcm.go`): a continuous `pcmStream` fed to an Oto player with a 250 ms buffer.
The `EbitenBackend` itself does not implement `MoviePCMBackend`,
so this is the path taken.

### soloud
SoLoud has no continuous ring buffer streaming primitive (only `Queue`,
which crackles on the steady stream of tiny per frame movie chunks),
so it deliberately does **not** implement `MoviePCMBackend`.
Movie audio therefore **falls back to the same Oto `moviePCMPlayer`** as the ebiten backend,
even when SoLoud is the configured backend for everything else (a deliberate fallback,
noted in `MoviePCMBackend`'s doc comment in `internal/audio/audio.go`).

### miniaudio
miniaudio implements `MoviePCMBackend` natively (`internal/audio/miniaudio/miniaudio.go`: `StartPCM` / `QueuePCM` / `StopPCM`),
using a real ring buffer device (`ma.Stream`) sized for ~2 s of audio with a preroll cushion so it rides out decode jitter.
This is the native movie path;
the Oto fallback is unused when miniaudio is selected.

## 2. Held loops (idle / running hum)

The held idle/running hum is started **once** and looped in a single player (`internal/gamescreen/sfx.go`: `updateAmbientSfx`),
so a clean loop needs the looped data itself to end on real audio.

### ebiten (Oto)
The game's sound clips are 22050 Hz;
the Ebitengine audio context mixes at 44100 Hz,
so `wav.DecodeWithSampleRate` **resamples**.
Draining the decoded stream with `io.ReadAll` keeps the resampler's **trailing zero padding past `stream.Length()`** (~223 ms of silence for ENGIN1's loop region),
which a looped clip would replay as an audible gap. `internal/audio/ebiten.go` therefore decodes through:

- **`decodePCM(stream)`** - reads the decoded stream and trims the result to
`stream.Length()`,
dropping the resampler's silent tail so a looped clip wraps cleanly.
Used by `pcm` (single shot SFX,
whole file held loops,
the cached decode),
`PlaySfxHeldMem` (the sliced running region held loop: building hums,
drive sounds) and `PlaySfxHeldMemIntroLoop` (the single voice intro->loop idle hum,
`eaudio.NewInfiniteLoopWithIntro`).
- **`loopReader` /
ewLoopReader`** - the held loop primitive: it fills every
buffer with no short read or crossfade,
unlike `NewInfiniteLoop` which blends the loop joint and can buzz on short mono SFX.
With trimmed data,
`loopReader` wraps gaplessly.

The music path is unaffected because it loops on `stream.Length()` (`eaudio.NewInfiniteLoop(stream,
stream.Length())`),
never on an `io.ReadAll` tail.
Regression test: `internal/audio/decodepcm_test.go` (`TestDecodePCMTrimsResamplerTail`) builds a synthetic 22050 Hz tone,
decodes it to 44100 Hz,
and asserts `decodePCM` drops the silent resampler tail and that the trimmed clip ends on real audio (a clean wrap).

### soloud
SoLoud decodes WAVs natively in the DLL (`Wav.LoadMem` in `internal/audio/soloud/soloud.go`: `PlaySfxHeldMem` / `PlaySfxHeldMemIntroLoop`,
looping via `engine.SetLooping` and `engine.SetLoopPoint`).
It does not use ebiten's resampler,
so the trimmed tail concern does not apply;
miniaudio is the gapless reference backend if a SoLoud loop gap is ever suspected.

### miniaudio
miniaudio loops a held clip with `NewSoundFromMemory` + `SetLooping` (`internal/audio/miniaudio/miniaudio.go`: `PlaySfxHeldMem`),
decoding natively and looping the data source continuously in its own callback.
It is gapless by construction.

## Related setting

`audio.backend` accepts a `"default"` value in `max-go.json`,
resolved to the native ebiten/Oto backend (`internal/settings/settings.go`: `BackendDefault`;
`cmd/max-go/main.go`:
ewAudioBackend` fall through).
It is the value written to a fresh `max-go.json`,
so the file itself shows that the default backend is in use.
