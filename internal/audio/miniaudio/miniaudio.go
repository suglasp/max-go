// miniaudio.go : Package miniaudio implements audio.Backend (and audio.MoviePCMBackend) using the local miniaudio purego binding, with no CGO.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
//
// Note: Unlike SoLoud, miniaudio has a continuous ring buffer source, so it plays streamed movie (MVE) PCM gaplessly.
// This backend handles movie audio itself (no fallback).
package miniaudio

import (
	"fmt"
	"math"
	"sync"

	ma "miniaudio"

	"github.com/suglasp/max-go/internal/audio"
	"github.com/suglasp/max-go/internal/dbg"
)

// Backend plays game music, sound effects, voice, and movie PCM through miniaudio.
type Backend struct {
	mu     sync.Mutex
	engine *ma.Engine

	music *ma.Sound
	voice *ma.Sound
	sfx   []*ma.Sound            // active effects played once, swept when finished
	held  map[*ma.Sound]struct{} // held effects (drive once / idle looped), stopped via their handle

	stream        *ma.Stream // movie PCM ring buffer device
	streamStarted bool       // device started once preroll is buffered
	streamCap     uint32     // ring buffer capacity in frames (empty AvailableWrite)
	streamPreroll uint32     // frames to buffer before starting the device
}

var (
	_ audio.Backend           = (*Backend)(nil)
	_ audio.MoviePCMBackend   = (*Backend)(nil)
	_ audio.HeldSfxBackend    = (*Backend)(nil)
	_ audio.MemHeldSfxBackend = (*Backend)(nil)
)

// New loads the miniaudio shared library and initializes the default device.
func New(dllPath string) (*Backend, error) {
	return newBackend(dllPath, false)
}

// NewNull loads the miniaudio shared library and initializes a deviceless
// engine (ma.NewNullEngine), mirroring SoLoud's NewNoSound: sounds load and
// play logically with no real audio hardware, so callers (headless tests,
// CI) can exercise the backend without a playback device.
func NewNull(dllPath string) (*Backend, error) {
	return newBackend(dllPath, true)
}

// newBackend loads the shared library and initializes the engine, real or null.
func newBackend(dllPath string, null bool) (*Backend, error) {
	if dllPath != "" {
		if err := ma.LoadPath(dllPath); err != nil {
			return nil, err
		}
	} else if err := ma.Load(); err != nil {
		return nil, err
	}

	var engine *ma.Engine
	var err error
	if null {
		engine, err = ma.NewNullEngine()
	} else {
		engine, err = ma.NewEngine()
	}
	if err != nil {
		return nil, fmt.Errorf("audio/miniaudio: init: %w", err)
	}

	dbg.Printf("miniaudio: version %s engine rate=%d channels=%d", ma.Version(), engine.SampleRate(), engine.Channels())

	return &Backend{engine: engine}, nil
}

// PlayMusic plays a music file. It loads the whole file (flag 0) rather than
// streaming (ma.SoundFlagStream): the streamed path starts without error but
// produces no audible output through this binding, whereas a sound loaded fully
// plays reliably (the same path the working SFX use). The .MSC tracks are WAVs
// of a few MB, so loading one at a time is cheap.
func (b *Backend) PlayMusic(path string, loop bool, volume float64) error {
	snd, err := b.engine.NewSoundFromFile(path, 0)
	if err != nil {
		return fmt.Errorf("audio/miniaudio: load music %s: %w", path, err)
	}
	snd.SetLooping(loop)
	snd.SetVolume(clampVolume(volume))

	b.mu.Lock()
	defer b.mu.Unlock()

	if b.engine == nil {
		snd.Destroy()
		return fmt.Errorf("audio/miniaudio: backend closed")
	}

	b.stopMusicLocked()
	b.music = snd

	return snd.Start()
}

// StopMusic stops and releases the current music stream.
func (b *Backend) StopMusic() {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.stopMusicLocked()
}

// stopMusicLocked stops and destroys the music stream while b.mu is held.
func (b *Backend) stopMusicLocked() {
	if b.music != nil {
		b.music.Stop()
		b.music.Destroy()
		b.music = nil
	}
}

// SetMusicVolume adjusts the running music voice.
func (b *Backend) SetMusicVolume(volume float64) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.music != nil {
		b.music.SetVolume(clampVolume(volume))
	}
}

// PlaySfx plays a short sound effect, optionally panned. Overlapping plays mix.
func (b *Backend) PlaySfx(path string, volume, pan float64) error {
	snd, err := b.engine.NewSoundFromFile(path, 0)
	if err != nil {
		return fmt.Errorf("audio/miniaudio: load sfx %s: %w", path, err)
	}

	snd.SetVolume(clampVolume(volume))
	snd.SetPan(clampPan(pan))
	if err := snd.Start(); err != nil {
		snd.Destroy()
		return err
	}

	b.mu.Lock()
	defer b.mu.Unlock()

	if b.engine == nil {
		snd.Stop()
		snd.Destroy()
		return fmt.Errorf("audio/miniaudio: backend closed")
	}

	// Sweep out finished effects before retaining the new one.
	kept := b.sfx[:0]
	for _, s := range b.sfx {
		if s.IsPlaying() {
			kept = append(kept, s)
		} else {
			s.Destroy()
		}
	}

	b.sfx = append(kept, snd)

	return nil
}

// PlaySfxHeld plays a sound effect on a stoppable handle, looping it when loop
// is set (the idle hum of the selected unit) and playing it once otherwise (the
// engine drive sound, cut on arrival); implements audio.HeldSfxBackend.
func (b *Backend) PlaySfxHeld(path string, volume, pan float64, loop bool) (audio.SfxHandle, error) {
	snd, err := b.engine.NewSoundFromFile(path, 0)

	if err != nil {
		return nil, fmt.Errorf("audio/miniaudio: load sfx %s: %w", path, err)
	}

	snd.SetVolume(clampVolume(volume))
	snd.SetPan(clampPan(pan))
	snd.SetLooping(loop)

	if err := snd.Start(); err != nil {
		snd.Destroy()
		return nil, err
	}

	b.mu.Lock()
	defer b.mu.Unlock()

	if b.engine == nil {
		snd.Stop()
		snd.Destroy()
		return nil, fmt.Errorf("audio/miniaudio: backend closed")
	}

	if b.held == nil {
		b.held = map[*ma.Sound]struct{}{}
	}
	b.held[snd] = struct{}{}

	return &heldSound{b: b, snd: snd}, nil
}

// PlaySfxHeldMem plays a held sound from a WAV held in memory (looped when
// loop is set): used to loop only a sound's running region. Implements
// audio.MemHeldSfxBackend.
func (b *Backend) PlaySfxHeldMem(wav []byte, volume, pan float64, loop bool) (audio.SfxHandle, error) {
	snd, err := b.engine.NewSoundFromMemory(wav, 0)

	if err != nil {
		return nil, fmt.Errorf("audio/miniaudio: load held mem sfx: %w", err)
	}

	snd.SetVolume(clampVolume(volume))
	snd.SetPan(clampPan(pan))
	snd.SetLooping(loop)

	if err := snd.Start(); err != nil {
		snd.Destroy()
		return nil, err
	}

	b.mu.Lock()
	defer b.mu.Unlock()

	if b.engine == nil {
		snd.Stop()
		snd.Destroy()
		return nil, fmt.Errorf("audio/miniaudio: backend closed")
	}

	if b.held == nil {
		b.held = map[*ma.Sound]struct{}{}
	}
	b.held[snd] = struct{}{}

	return &heldSound{b: b, snd: snd}, nil
}

// heldSound is a stoppable held sound effect owned by a Backend.
type heldSound struct {
	b   *Backend
	snd *ma.Sound
}

// SetPan moves the held sound in stereo while it plays (a moving unit's drive
// sound). Does nothing once the sound has stopped.
func (h *heldSound) SetPan(pan float64) {
	h.b.mu.Lock()
	defer h.b.mu.Unlock()

	if h.snd != nil {
		h.snd.SetPan(clampPan(pan))
	}
}

// SetVolume fades the held sound while it plays. Does nothing once it has stopped.
func (h *heldSound) SetVolume(volume float64) {
	h.b.mu.Lock()
	defer h.b.mu.Unlock()

	if h.snd != nil {
		h.snd.SetVolume(clampVolume(volume))
	}
}

// Stop ends the sound and releases it (idempotent).
func (h *heldSound) Stop() {
	h.b.mu.Lock()
	defer h.b.mu.Unlock()

	if h.snd == nil {
		return
	}

	if _, ok := h.b.held[h.snd]; ok {
		h.snd.Stop()
		h.snd.Destroy()
		delete(h.b.held, h.snd)
	}

	h.snd = nil
}

// PlayVoice plays a voice line, interrupting the previous one.
func (b *Backend) PlayVoice(path string, volume float64) error {
	snd, err := b.engine.NewSoundFromFile(path, 0)
	if err != nil {
		return fmt.Errorf("audio/miniaudio: load voice %s: %w", path, err)
	}

	snd.SetVolume(clampVolume(volume))

	b.mu.Lock()
	defer b.mu.Unlock()

	if b.engine == nil {
		snd.Destroy()
		return fmt.Errorf("audio/miniaudio: backend closed")
	}

	b.stopVoiceLocked()
	b.voice = snd

	return snd.Start()
}

// VoicePlaying reports whether a voice line is still active.
func (b *Backend) VoicePlaying() bool {
	b.mu.Lock()
	defer b.mu.Unlock()

	return b.voice != nil && b.voice.IsPlaying()
}

// stopVoiceLocked stops and destroys the current voice sample while b.mu is held.
func (b *Backend) stopVoiceLocked() {
	if b.voice != nil {
		b.voice.Stop()
		b.voice.Destroy()
		b.voice = nil
	}
}

// StartPCM opens a ring buffer device for decoded movie audio (MoviePCMBackend).
func (b *Backend) StartPCM(format audio.PCMFormat, volume float64) error {
	if format.Bits != 16 || (format.Channels != 1 && format.Channels != 2) || format.Rate <= 0 {
		return fmt.Errorf("audio/miniaudio: unsupported PCM format %+v", format)
	}

	// Keep about two seconds in the ring buffer to absorb decoder, GC, and disk-read stalls. The device runs at the movie rate, so it needs no resampling.
	capacity := uint32(format.Rate) * 2

	stream, err := ma.NewStream(ma.FormatS16, uint32(format.Channels), uint32(format.Rate), capacity)
	if err != nil {
		return fmt.Errorf("audio/miniaudio: movie stream: %w", err)
	}

	if err := stream.SetVolume(clampVolume(volume)); err != nil {
		stream.Destroy()

		return err
	}

	b.mu.Lock()
	defer b.mu.Unlock()

	b.stopStreamLocked()
	b.stream = stream
	// Queue about 400 ms before starting the device. This avoids initial underruns while keeping the audio/video offset small.
	// streamCap records the actual empty capacity because miniaudio may round the requested size.
	b.streamStarted = false
	b.streamCap = stream.AvailableWrite()
	b.streamPreroll = uint32(format.Rate) * 2 / 5

	dbg.Printf("miniaudio: movie pcm rate=%d channels=%d preroll=%d cap=%d (frames)", format.Rate, format.Channels, b.streamPreroll, b.streamCap)

	return nil
}

// QueuePCM pushes one decoded movie PCM chunk into the ring buffer, starting the
// device once enough audio is buffered (preroll).
func (b *Backend) QueuePCM(samples []int16) error {
	if len(samples) == 0 {
		return nil
	}

	b.mu.Lock()
	defer b.mu.Unlock()

	if b.stream == nil {
		return nil
	}

	// WriteInt16 returns the frames queued; a short write means the ring buffer
	// is briefly full and the overflow is dropped (not fatal for movie audio).
	b.stream.WriteInt16(samples)
	if !b.streamStarted && b.streamCap-b.stream.AvailableWrite() >= b.streamPreroll {
		if err := b.stream.Start(); err != nil {
			return fmt.Errorf("audio/miniaudio: movie stream start: %w", err)
		}

		b.streamStarted = true
	}

	return nil
}

// StopPCM stops and releases the movie PCM stream.
func (b *Backend) StopPCM() {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.stopStreamLocked()
}

// stopStreamLocked stops and destroys the movie stream while b.mu is held.
func (b *Backend) stopStreamLocked() {
	if b.stream != nil {
		b.stream.Stop()
		b.stream.Destroy()
		b.stream = nil
	}

	b.streamStarted = false
}

// StopAll silences music, voice, sound effects, and movie PCM.
func (b *Backend) StopAll() {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.stopMusicLocked()
	b.stopVoiceLocked()

	for _, s := range b.sfx {
		s.Stop()
		s.Destroy()
	}
	b.sfx = nil

	for s := range b.held {
		s.Stop()
		s.Destroy()
	}
	b.held = nil

	b.stopStreamLocked()
}

// Close releases all miniaudio resources owned by the backend.
func (b *Backend) Close() error {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.engine == nil {
		return nil
	}

	b.stopMusicLocked()
	b.stopVoiceLocked()

	for _, s := range b.sfx {
		s.Destroy()
	}
	b.sfx = nil

	for s := range b.held {
		s.Destroy()
	}

	b.held = nil
	b.stopStreamLocked()
	b.engine.Destroy()
	b.engine = nil

	return nil
}

// clampVolume normalizes a 0..1 volume to miniaudio's float32 scale.
func clampVolume(v float64) float32 {
	if math.IsNaN(v) || v < 0 {
		return 0
	}

	if v > 1 {
		return 1
	}

	return float32(v)
}

// clampPan normalizes a stereo pan to [-1, 1].
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
