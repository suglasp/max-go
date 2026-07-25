// soloud.go : Package soloud implements audio.Backend using the local SoLoud purego binding
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
//
// Note: SoLoud doesn't use a ring buffer, so it handles movie audio with a fallback to Oto (Ebitengine sound backend).
package soloud

import (
	"fmt"
	"math"
	"strings"
	"sync"
	"time"

	soloudlib "soloud"

	"github.com/suglasp/max-go/internal/audio"
	"github.com/suglasp/max-go/internal/dbg"
)

const (
	defaultFlags = soloudlib.SOLOUD_CLIP_ROUNDOFF
	noVoice      = uint32(0)
)

// Backend plays game music, sound effects, and voice through SoLoud.
type Backend struct {
	mu     sync.Mutex
	engine *soloudlib.Soloud

	music       *soloudlib.WavStream
	musicHandle uint32
	voice       *soloudlib.Wav
	voiceHandle uint32
	sfx         map[string]*soloudlib.Wav
	held        map[*heldSound]struct{}
}

// NOTE: this backend intentionally has no movie PCM (StartPCM/QueuePCM) path.
// Movie audio is streamed through the Manager's Oto player instead, because
// SoLoud has no continuous ring buffer source; only its Queue, which returns
// silence the instant it momentarily empties and so crackles on the steady
// stream of small MVE chunks arriving frame by frame. See internal/audio/moviepcm.go.

var (
	_ audio.Backend             = (*Backend)(nil)
	_ audio.HeldSfxBackend      = (*Backend)(nil)
	_ audio.MemHeldSfxBackend   = (*Backend)(nil)
	_ audio.IntroLoopSfxBackend = (*Backend)(nil)
)

// New loads the SoLoud shared library and initializes the automatic backend.
func New(dllPath string) (*Backend, error) {
	return newBackend(dllPath, soloudlib.SOLOUD_AUTO)
}

// NewWithBackend loads SoLoud and initializes the named device backend (see
// BackendID); an empty or unknown name uses SoLoud's automatic choice.
func NewWithBackend(dllPath, backend string) (*Backend, error) {
	return newBackend(dllPath, BackendID(backend))
}

// BackendID maps a settings backend name to a SoLoud backend constant. Unknown
// or empty names map to SOLOUD_AUTO. The chosen backend must be one the loaded
// SoLoud build supports, else init fails and the caller falls back.
func BackendID(name string) uint32 {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "miniaudio":
		return soloudlib.SOLOUD_MINIAUDIO
	case "wasapi":
		return soloudlib.SOLOUD_WASAPI
	case "winmm":
		return soloudlib.SOLOUD_WINMM
	case "xaudio2":
		return soloudlib.SOLOUD_XAUDIO2
	case "sdl2":
		return soloudlib.SOLOUD_SDL2
	case "portaudio":
		return soloudlib.SOLOUD_PORTAUDIO
	case "openal":
		return soloudlib.SOLOUD_OPENAL
	case "alsa":
		return soloudlib.SOLOUD_ALSA
	case "coreaudio":
		return soloudlib.SOLOUD_COREAUDIO
	default:
		return soloudlib.SOLOUD_AUTO
	}
}

// NewNoSound loads SoLoud with its silent backend for tests and smoke tools.
func NewNoSound(dllPath string) (*Backend, error) {
	return newBackend(dllPath, soloudlib.SOLOUD_NOSOUND)
}

// newBackend loads the shared library and initializes SoLoud with backend.
func newBackend(dllPath string, backend uint32) (*Backend, error) {
	if dllPath != "" {
		if err := soloudlib.LoadPath(dllPath); err != nil {
			return nil, err
		}
	} else if err := soloudlib.Load(); err != nil {
		return nil, err
	}

	engine := soloudlib.NewSoloud()
	if err := engine.InitEx(defaultFlags, backend, soloudlib.SOLOUD_AUTO, soloudlib.SOLOUD_AUTO, 2); err != nil {
		engine.Destroy()
		return nil, fmt.Errorf("audio/soloud: init: %w", err)
	}
	// Log the device backend SoLoud chose and its rate/buffer: a small AUTO
	// buffer on the device backend (often miniaudio) is a known stutter source,
	// and the device sample rate matters for movie PCM resampling.
	dbg.Printf("soloud: backend %q rate=%d buffer=%d channels=%d", engine.GetBackendString(), engine.GetBackendSamplerate(), engine.GetBackendBufferSize(), engine.GetBackendChannels())
	return &Backend{engine: engine, sfx: map[string]*soloudlib.Wav{}}, nil
}

// PlayMusic streams a music file from disk.
func (b *Backend) PlayMusic(path string, loop bool, volume float64) error {
	stream := soloudlib.NewWavStream()
	if err := stream.Load(path); err != nil {
		stream.Destroy()
		return fmt.Errorf("audio/soloud: load music %s: %w", path, err)
	}
	stream.SetLooping(loop)

	b.mu.Lock()
	defer b.mu.Unlock()
	if b.engine == nil {
		stream.Destroy()
		return fmt.Errorf("audio/soloud: backend closed")
	}
	b.stopMusicLocked()
	b.music = stream
	b.musicHandle = b.engine.PlayEx(stream, clampVolume(volume), 0, false, 0)
	return nil
}

// StopMusic stops and releases the current music stream.
func (b *Backend) StopMusic() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.stopMusicLocked()
}

// stopMusicLocked stops and destroys the current music stream while b.mu is held.
func (b *Backend) stopMusicLocked() {
	if b.engine != nil && b.musicHandle != noVoice {
		b.engine.Stop(b.musicHandle)
		b.musicHandle = noVoice
	}
	if b.music != nil {
		b.music.Destroy()
		b.music = nil
	}
}

// SetMusicVolume adjusts the running music voice.
func (b *Backend) SetMusicVolume(volume float64) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.engine != nil && b.musicHandle != noVoice && b.engine.IsValidVoiceHandle(b.musicHandle) {
		b.engine.SetVolume(b.musicHandle, clampVolume(volume))
	}
}

// PlaySfx plays a cached sound effect held in memory.
func (b *Backend) PlaySfx(path string, volume, pan float64) error {
	wav, err := b.wav(path)
	if err != nil {
		return err
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.engine == nil {
		return fmt.Errorf("audio/soloud: backend closed")
	}
	b.engine.PlayEx(wav, clampVolume(volume), clampPan(pan), false, 0)
	return nil
}

// PlaySfxHeld plays a cached sound effect on a stoppable voice handle, looping
// the voice when requested. Looping is applied to the voice handle, not the
// shared Wav source, so single plays using the same cached sound are unaffected.
func (b *Backend) PlaySfxHeld(path string, volume, pan float64, loop bool) (audio.SfxHandle, error) {
	wav, err := b.wav(path)
	if err != nil {
		return nil, err
	}

	b.mu.Lock()
	defer b.mu.Unlock()
	if b.engine == nil {
		return nil, fmt.Errorf("audio/soloud: backend closed")
	}
	handle := b.engine.PlayEx(wav, clampVolume(volume), clampPan(pan), false, 0)
	if handle != noVoice {
		b.engine.SetLooping(handle, loop)
	}
	return b.newHeldSoundLocked(handle, nil), nil
}

// PlaySfxHeldMem plays a held sound from a WAV buffer held in memory. The Wav is
// owned by the returned handle and is destroyed when the handle stops.
func (b *Backend) PlaySfxHeldMem(wavBytes []byte, volume, pan float64, loop bool) (audio.SfxHandle, error) {
	wav := soloudlib.NewWav()
	if err := wav.LoadMem(wavBytes); err != nil {
		wav.Destroy()
		return nil, fmt.Errorf("audio/soloud: load held mem sfx: %w", err)
	}

	b.mu.Lock()
	defer b.mu.Unlock()
	if b.engine == nil {
		wav.Destroy()
		return nil, fmt.Errorf("audio/soloud: backend closed")
	}
	handle := b.engine.PlayEx(wav, clampVolume(volume), clampPan(pan), false, 0)
	if handle != noVoice {
		b.engine.SetLooping(handle, loop)
	}
	return b.newHeldSoundLocked(handle, wav), nil
}

// PlaySfxHeldMemIntroLoop plays [0, end) once, then uses SoLoud's native loop point to repeat [loopStart, end) on the same voice without a gap.
func (b *Backend) PlaySfxHeldMemIntroLoop(introLoopWAV []byte, loopStart time.Duration, volume, pan float64) (audio.SfxHandle, error) {
	wav := soloudlib.NewWav()
	if err := wav.LoadMem(introLoopWAV); err != nil {
		wav.Destroy()
		return nil, fmt.Errorf("audio/soloud: load intro-loop sfx: %w", err)
	}

	b.mu.Lock()
	defer b.mu.Unlock()
	if b.engine == nil {
		wav.Destroy()
		return nil, fmt.Errorf("audio/soloud: backend closed")
	}
	handle := b.engine.PlayEx(wav, clampVolume(volume), clampPan(pan), false, 0)
	if handle != noVoice {
		// Loop back to the region start (not 0): the voice plays the whole clip
		// [0, end) once, then repeats [loopStart, end) seamlessly.
		b.engine.SetLoopPoint(handle, loopStart.Seconds())
		b.engine.SetLooping(handle, true)
	}
	return b.newHeldSoundLocked(handle, wav), nil
}

func (b *Backend) newHeldSoundLocked(handle uint32, ownedWav *soloudlib.Wav) *heldSound {
	if b.held == nil {
		b.held = map[*heldSound]struct{}{}
	}
	h := &heldSound{b: b, handle: handle, ownedWav: ownedWav}
	b.held[h] = struct{}{}
	return h
}

// heldSound is a stoppable SoLoud voice. ownedWav is set only for held sounds
// loaded from memory; cached file Wavs remain owned by Backend.sfx.
type heldSound struct {
	b        *Backend
	handle   uint32
	ownedWav *soloudlib.Wav
}

// SetPan moves the held voice in stereo while it plays (a moving unit's drive
// sound). Does nothing once the voice has stopped. Wraps SoLoud's own
// Soloud::setPan.
func (h *heldSound) SetPan(pan float64) {
	if h == nil || h.b == nil {
		return
	}
	b := h.b
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.engine != nil && h.handle != noVoice {
		b.engine.SetPan(h.handle, clampPan(pan))
	}
}

// Stop ends the held voice and releases any Wav owned by this handle. It is idempotent.
func (h *heldSound) Stop() {
	if h == nil || h.b == nil {
		return
	}
	b := h.b
	b.mu.Lock()
	defer b.mu.Unlock()
	h.stopLocked()
}

func (h *heldSound) stopLocked() {
	b := h.b
	if b == nil {
		return
	}
	if b.engine != nil && h.handle != noVoice {
		b.engine.Stop(h.handle)
	}
	if h.ownedWav != nil {
		h.ownedWav.Destroy()
		h.ownedWav = nil
	}
	delete(b.held, h)
	h.handle = noVoice
	h.b = nil
}

// wav loads and caches a short sound effect sample.
func (b *Backend) wav(path string) (*soloudlib.Wav, error) {
	b.mu.Lock()
	wav, ok := b.sfx[path]
	b.mu.Unlock()
	if ok {
		return wav, nil
	}

	wav = soloudlib.NewWav()
	if err := wav.Load(path); err != nil {
		wav.Destroy()
		return nil, fmt.Errorf("audio/soloud: load %s: %w", path, err)
	}

	b.mu.Lock()
	defer b.mu.Unlock()
	if b.engine == nil {
		wav.Destroy()
		return nil, fmt.Errorf("audio/soloud: backend closed")
	}
	if existing, ok := b.sfx[path]; ok {
		wav.Destroy()
		return existing, nil
	}
	b.sfx[path] = wav
	return wav, nil
}

// PlayVoice plays a voice line, interrupting the previous one.
func (b *Backend) PlayVoice(path string, volume float64) error {
	wav := soloudlib.NewWav()
	if err := wav.Load(path); err != nil {
		wav.Destroy()
		return fmt.Errorf("audio/soloud: load voice %s: %w", path, err)
	}

	b.mu.Lock()
	defer b.mu.Unlock()
	if b.engine == nil {
		wav.Destroy()
		return fmt.Errorf("audio/soloud: backend closed")
	}
	b.stopVoiceLocked()
	b.voice = wav
	b.voiceHandle = b.engine.PlayEx(wav, clampVolume(volume), 0, false, 0)
	return nil
}

// VoicePlaying reports whether a voice line is still active.
func (b *Backend) VoicePlaying() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.engine != nil && b.voiceHandle != noVoice && b.engine.IsValidVoiceHandle(b.voiceHandle)
}

// stopVoiceLocked stops and destroys the current voice sample while b.mu is held.
func (b *Backend) stopVoiceLocked() {
	if b.engine != nil && b.voiceHandle != noVoice {
		b.engine.Stop(b.voiceHandle)
		b.voiceHandle = noVoice
	}
	if b.voice != nil {
		b.voice.Destroy()
		b.voice = nil
	}
}

// StopAll silences music, voice, and sound effects.
func (b *Backend) StopAll() {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.engine != nil {
		b.engine.StopAll()
	}
	b.stopMusicLocked()
	b.stopVoiceLocked()
	b.stopHeldLocked()
}

// Close releases all SoLoud resources owned by the backend.
func (b *Backend) Close() error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.engine == nil {
		return nil
	}
	b.engine.StopAll()
	b.stopMusicLocked()
	b.stopVoiceLocked()
	b.stopHeldLocked()
	for path, wav := range b.sfx {
		wav.Destroy()
		delete(b.sfx, path)
	}
	b.engine.Deinit()
	b.engine.Destroy()
	b.engine = nil
	return nil
}

func (b *Backend) stopHeldLocked() {
	for h := range b.held {
		h.stopLocked()
	}
	b.held = nil
}

// clampVolume normalizes user volume values to SoLoud's accepted range.
func clampVolume(v float64) float32 {
	if math.IsNaN(v) || v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return float32(v)
}

// clampPan normalizes stereo pan values to SoLoud's accepted range.
func clampPan(v float64) float32 {
	if math.IsNaN(v) {
		return 0
	}
	if v < -1 {
		return -1
	}
	if v > 1 {
		return 1
	}
	return float32(v)
}
