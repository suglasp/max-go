# Audio files: .MSC music, .WAV effects, sound resources

## Sound resources are filename references

Audio entries in MAX.RES (e.g. `MAIN_MSC`,
`MBUTT0`,
`NDONE0`,
`KCARG0`) contain no audio data.
Each is a tiny NUL terminated file name pointing at a loose file in the game data directory:

| Resource | Content (file name) | Used for                  |
|----------|---------------------|---------------------------|
| MAIN_MSC | `main_msc.msc`      | main menu music           |
| MBUTT0   | `mengens4.wav`      | large menu button click   |
| NDONE0   | `menu38.wav`        | done/exit button click    |
| KCARG0   | `kbuy0.wav`         | portrait click            |

File name lookup should be case insensitive for Linux installs.

## .MSC: music

Despite the extension,
these are renamed RIFF/WAV files,
not MIDI or a custom SoLoud format.
Verified across the retail tracks in the 0.7.2 data set:

| Field | Value |
| --- | --- |
| Container | RIFF/WAVE |
| `fmt` audio format | `1` (PCM) |
| Channels | `2` (stereo) |
| Sample rate | `22050` Hz |
| Bits per sample | `16` |
| Typical size | 14 to 18 MB |

Because the files are large,
they must be streamed from disk.
Do not decode and cache a whole music track the way short SFX/voice clips are cached.

### Music resource tags

Reference resource IDs from `resourcetable.hpp`:

| Resource | ID | Use / notes |
| --- | ---: | --- |
| `SNOW_MSC` | `0x6A1` | Snow world music. |
| `CRTR_MSC` | `0x6A2` | Crater world music. |
| `GREN_MSC` | `0x6A3` | Green world music. |
| `DSRT_MSC` | `0x6A4` | Desert world music. |
| `MAIN_MSC` | `0x6A5` | Main menu music. |
| `BKG1_MSC`..`BKG9_MSC` | `0x6A6`..`0x6AE` | In game shuffled background playlist. |
| `CRGO_MSC` | `0x6AF` | Cargo/custom start music. |
| `CRDT_MSC` | `0x6B0` | Credits music. |
| `WINR_MSC` | `0x6B1` | Win screen music. |
| `LOSE_MSC` | `0x6B2` | Lose/death screen music. |

The RES entry body is still just a loose filename reference such as `main_msc.msc`;
the audio bytes live in the game data directory.

### Reference playback behaviour

`SoundManager::PlayMusic(id,
shuffle)` creates a music job with centered pan and the music volume group.
If `shuffle` is true and the starting resource is outside `MAIN_MSC..BKG9_MSC`,
it resets the background playlist state. `UpdateMusic` starts another background track when the current one finishes;
the menu track is not shuffled and is restarted/looped as a single track.

The original engine additionally keeps loop points for in game music (`SoundManager::LoadLoopPoints`);
the menu track simply loops whole.
Loop point support is deferred until in game music lands.

### Current Go behaviour / limits

The Go `audio.Manager` resolves a music tag through `MAX.RES`/`PATCHES.RES` to a loose `.MSC` path,
then calls the selected backend's `PlayMusic(path,
loop,
volume)`.

| Backend | Music path |
| --- | --- |
| Ebiten/Oto | Opens the file, decodes WAV through `ebiten/v2/audio/wav`, and wraps it in `audio.NewInfiniteLoop` when `loop` is true. |
| miniaudio | Loads the path as a stream backed sound and sets looping on that sound. |
| SoLoud | Loads the path as a `WavStream` and sets looping on the playing handle. |

Current limits:

- This project loops the selected track when the caller asks for looping.
- The reference shuffled `BKG1_MSC`..`BKG9_MSC` playlist handoff is not documented as
fully ported here;
check the gamescreen/menu caller before assuming shuffle.
- No per-track loop point table is parsed in Go. If the reference implementation's in-game loop points
are ported later,
they should sit above the backend and not change the `.MSC` file parser: the file itself is plain RIFF/WAV.

## .WAV: sound effects and voice

Standard RIFF/WAV,
PCM 16-bit,
mono,
22050 Hz.
Short enough to decode fully and cache.

### `smpl` loop points in held sound effects

Some held sound effects carry an optional RIFF `smpl` chunk.
This project only needs the first `SampleLoop` entry:

| Field | Offset in SampleLoop | Meaning |
|-------|----------------------|---------|
| start | +8                   | first frame of the loop region |
| end   | +12                  | loop end boundary from the data file |

In the reference miniaudio path (`sound_manager.cpp`),
ordinary unit SFX play from the start once and then loop using the internal loop points.
Building/work sounds are the special case: they are range restricted immediately to the running loop region.

Backend note for this project: not every backend can loop a subrange directly,
so the manager can slice an in memory WAV holding only the running region and send that to `PlaySfxHeldMem`.
Authored `smpl` data is the only trusted source for that slice.

Important guardrail: do not synthesize loop points for WAV files that have no `smpl` chunk. `APC1.WAV` is a known no-`smpl` example;
a heuristic tail trim was tested and produced a corrupted,
chainsaw sounding loop.
Sounds without `smpl` must fall back to whole clip behavior unless a real per file rule is verified.

Observed examples from the 0.7.2 data set:

| File | Loop start | Loop end boundary | Tail frames after end |
|------|------------|-------------------|-----------------------|
| ENGIN1.WAV | 2449 | 64895 | 1153 |
| SCAN1.WAV  | 375  | 28873 | 439  |
| APC1.WAV   | none | none  | n/a  |

## Mixing groups (sound_manager.cpp)

Three volume groups,
each with a 0..100 level and an enable flag in max-go.json: music,
sfx (positional panning in game),
voice (a new voice line interrupts the previous one).
