// ebiten.go : the M.A.X. Ebitengine audio backend: implements the Backend interface using Ebitengine's audio context.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package audio

import (
	"bytes"
	"fmt"
	"io"
	"math"
	"os"
	"sync"
	"sync/atomic"
	"time"

	eaudio "github.com/hajimehoshi/ebiten/v2/audio"
	"github.com/hajimehoshi/ebiten/v2/audio/wav"

	"github.com/suglasp/max-go/internal/dbg"
)

// sampleRate is the mixing rate of the Ebitengine audio context.
// The game data uses 22050 Hz; the wav decoder resamples on the fly.
const sampleRate = 44100

// The Ebitengine audio context can only be created once per process.
var (
	ctxOnce sync.Once
	ctx     *eaudio.Context
)

// context returns the audio context shared by the whole process.
func context() *eaudio.Context {
	ctxOnce.Do(func() { ctx = eaudio.NewContext(sampleRate) })
	return ctx
}

// EbitenBackend plays audio through Ebitengine's mixer (the "native"
// backend of settings.json).
type EbitenBackend struct {
	mu         sync.Mutex
	music      *eaudio.Player
	musicFile  *os.File
	voice      *eaudio.Player
	sfx        map[string][]byte           // decoded PCM cache for short sounds
	sfxPlayers []*eaudio.Player            // retained until finished; an unreferenced Player is GC'd and stops mid play
	sfxHeld    map[*eaudio.Player]struct{} // active held sfx (engine drive sounds); stopped early on Stop/StopAll
}

// NewEbitenBackend creates the native backend.
func NewEbitenBackend() *EbitenBackend {
	return &EbitenBackend{sfx: map[string][]byte{}}
}

// NewEbitenBackendChecked creates the native backend and verifies that
// Oto can create a player. This catches headless/VM setups with no
// usable audio device before gameplay tries to play a sound.
func NewEbitenBackendChecked() (*EbitenBackend, error) {
	b := NewEbitenBackend()
	if err := b.probe(); err != nil {
		return nil, err
	}
	dbg.Printf("ebiten/oto: rate=%d channels=2 (stereo)", sampleRate)
	return b, nil
}

// probe creates and closes a tiny player to verify the native audio device.
func (b *EbitenBackend) probe() error {
	player, err := context().NewPlayer(bytes.NewReader(make([]byte, 4)))
	if err != nil {
		return fmt.Errorf("audio: ebiten/oto init: %w", err)
	}
	return player.Close()
}

// PlayMusic streams a music file from disk (17 MB tracks are not loaded whole).
func (b *EbitenBackend) PlayMusic(path string, loop bool, volume float64) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	stream, err := wav.DecodeWithSampleRate(sampleRate, f)
	if err != nil {
		f.Close()
		return fmt.Errorf("audio: %s: %w", path, err)
	}

	var src io.Reader = stream
	if loop {
		src = eaudio.NewInfiniteLoop(stream, stream.Length())
	}
	player, err := context().NewPlayer(src)
	if err != nil {
		f.Close()
		return err
	}
	player.SetVolume(volume)
	player.Play()

	b.mu.Lock()
	b.stopMusicLocked()
	b.music = player
	b.musicFile = f
	b.mu.Unlock()
	return nil
}

// StopMusic stops and releases the music player.
func (b *EbitenBackend) StopMusic() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.stopMusicLocked()
}

// stopMusicLocked closes the current music player while b.mu is held.
func (b *EbitenBackend) stopMusicLocked() {
	if b.music != nil {
		b.music.Close()
		b.music = nil
	}
	if b.musicFile != nil {
		b.musicFile.Close()
		b.musicFile = nil
	}
}

// SetMusicVolume adjusts the running track.
func (b *EbitenBackend) SetMusicVolume(volume float64) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.music != nil {
		b.music.SetVolume(volume)
	}
}

// PCM loads and caches a short sound fully decoded.
func (b *EbitenBackend) pcm(path string) ([]byte, error) {
	b.mu.Lock()
	data, ok := b.sfx[path]
	b.mu.Unlock()
	if ok {
		return data, nil
	}

	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	stream, err := wav.DecodeWithSampleRate(sampleRate, f)
	if err != nil {
		return nil, fmt.Errorf("audio: %s: %w", path, err)
	}
	data, err = decodePCM(stream)
	if err != nil {
		return nil, err
	}

	b.mu.Lock()
	b.sfx[path] = data
	b.mu.Unlock()
	return data, nil
}

// PlaySfx fires a short sound; overlapping plays mix freely.
func (b *EbitenBackend) PlaySfx(path string, volume, pan float64) error {
	data, err := b.pcm(path)
	if err != nil {
		return err
	}
	data = panStereoPCM(data, pan)
	player, err := context().NewPlayer(bytes.NewReader(data))
	if err != nil {
		return err
	}
	player.SetVolume(volume)
	player.Play()
	// Retain the player: Ebitengine stops a Player as soon as it is garbage
	// collected, so a local that goes out of scope would cut the sound off
	// instantly. Keep it alive until it finishes, sweeping out done players.
	b.mu.Lock()
	kept := b.sfxPlayers[:0]
	for _, p := range b.sfxPlayers {
		if p.IsPlaying() {
			kept = append(kept, p)
		} else {
			p.Close()
		}
	}
	b.sfxPlayers = append(kept, player)
	b.mu.Unlock()
	return nil
}

// frameSize is the bytes per audio frame the Ebitengine context consumes:
// signed 16 bit little endian stereo = 2 bytes * 2 channels.
const frameSize = 4

// decodePCM reads a decoded wav stream to PCM, trimming the trailing zero
// padding ebiten's resampler emits PAST stream.Length(). io.ReadAll alone keeps
// that padding, tens to hundreds of ms of silence (223ms for a clip resampled
// from 22050 to 44100); when the clip is looped, that silence is the audible
// "stop and repeat" at the loop boundary. The music path never hits this because
// it loops on stream.Length() (eaudio.NewInfiniteLoop), not the tail read past that.
// SoLoud and miniaudio decode natively and have no such resampler tail. This
// was the real cause of the hiccup in a held loop, not the loop reader (which
// loops the bytes it is handed faithfully).
func decodePCM(stream *wav.Stream) ([]byte, error) {
	data, err := io.ReadAll(stream)
	if err != nil {
		return nil, err
	}
	// stream.Length() is the true resampled byte length (aligned to whole frames);
	// anything io.ReadAll read beyond it is the resampler's silent tail, so drop it.
	if n := int(stream.Length()); n >= 0 && n <= len(data) {
		data = data[:n]
	}
	return data, nil
}

// loopReader repeats a fixed PCM buffer forever, restarting at the end. Unlike
// eaudio.NewInfiniteLoop it does no blending at the stereo loop joint (which
// assumes a seamless stereo music loop and buzzes on the game's short mono
// sfx); it simply replays the exact bytes the single play path plays
// correctly. The loop boundary is the buffer truncated to a whole number of
// frames so the channels never drift out of phase across the wrap.
type loopReader struct {
	data []byte
	pos  int
}

func newLoopReader(data []byte) *loopReader {
	return &loopReader{data: data[:len(data)/frameSize*frameSize]}
}

func (r *loopReader) Read(p []byte) (int, error) {
	if len(r.data) == 0 {
		return 0, io.EOF
	}
	n := 0
	for n < len(p) {
		if r.pos >= len(r.data) {
			r.pos = 0
		}
		m := copy(p[n:], r.data[r.pos:])
		r.pos += m
		n += m
	}
	return n, nil
}

// panReader applies a live, updatable stereo pan to the PCM frames it reads from
// src. Unlike the single play PlaySfx path (which bakes a fixed pan into the
// buffer), a held sound can still be panned again later: the game updates the
// pan each frame as the unit moves across the screen through the handle's
// SetPan, and the audio goroutine reads the current value here. The pan is
// stored as the float64 bits in an atomic so the two goroutines never race.
type panReader struct {
	src io.Reader
	pan atomic.Uint64 // math.Float64bits of the current pan, [-1, 1]
}

// newPanReader wraps src with the given starting pan.
func newPanReader(src io.Reader, pan float64) *panReader {
	r := &panReader{src: src}
	r.pan.Store(math.Float64bits(clampPan(pan)))
	return r
}

// setPan updates the pan applied to subsequent reads.
func (r *panReader) setPan(pan float64) {
	r.pan.Store(math.Float64bits(clampPan(pan)))
}

// Read fills p from src and pans the whole frames in place.
func (r *panReader) Read(p []byte) (int, error) {
	n, err := r.src.Read(p)
	if n > 0 {
		panStereoInPlace(p[:n], math.Float64frombits(r.pan.Load()))
	}
	return n, err
}

// PlaySfxHeld plays a sound effect and returns a handle whose Stop ends it
// early. With loop set the clip repeats (the idle hum); otherwise it plays once
// (the engine drive sound, cut the moment a unit arrives). Like PlaySfx the
// player is retained, both to stop it on demand and so the GC does not cut it
// off. The pan is applied through a panReader so the handle can still be
// panned again later.
//
// C++ ported : sound_manager.cpp - SoundManager's persistent sfx channel per unit, freed by FreeSfx
func (b *EbitenBackend) PlaySfxHeld(path string, volume, pan float64, loop bool) (SfxHandle, error) {
	data, err := b.pcm(path)
	if err != nil {
		return nil, err
	}
	var src io.Reader = bytes.NewReader(data)
	if loop {
		// Loop the exact PCM (decodePCM already trimmed the resampler's silent
		// tail in pcm(), so the wrap is gapless). loopReader fills every buffer
		// fully with no short read or blend, unlike eaudio.NewInfiniteLoop which
		// crossfades the loop joint and buzzes on these short mono sfx.
		src = newLoopReader(data)
	}
	pr := newPanReader(src, pan)
	player, err := context().NewPlayer(pr)
	if err != nil {
		return nil, err
	}
	player.SetVolume(volume)
	player.Play()
	b.mu.Lock()
	if b.sfxHeld == nil {
		b.sfxHeld = map[*eaudio.Player]struct{}{}
	}
	b.sfxHeld[player] = struct{}{}
	b.mu.Unlock()
	return &ebitenSfxHandle{b: b, player: player, pan: pr}, nil
}

// PlaySfxHeldMem plays a held sound from a WAV held in memory (looped when
// loop is set): used to loop only a sound's running region (the Manager slices
// the WAV at its smpl loop point). Implements audio.MemHeldSfxBackend.
func (b *EbitenBackend) PlaySfxHeldMem(wavBytes []byte, volume, pan float64, loop bool) (SfxHandle, error) {
	stream, err := wav.DecodeWithSampleRate(sampleRate, bytes.NewReader(wavBytes))
	if err != nil {
		return nil, fmt.Errorf("audio: held mem: %w", err)
	}
	data, err := decodePCM(stream)
	if err != nil {
		return nil, err
	}
	var src io.Reader = bytes.NewReader(data)
	if loop {
		// Loop the exact PCM. decodePCM trimmed the resampler's silent tail (the
		// real cause of the loop hiccup), so loopReader wraps gaplessly. See the
		// PlaySfxHeld note on why loopReader is preferred over NewInfiniteLoop.
		src = newLoopReader(data)
	}
	pr := newPanReader(src, pan)
	player, err := context().NewPlayer(pr)
	if err != nil {
		return nil, err
	}
	player.SetVolume(volume)
	player.Play()
	b.mu.Lock()
	if b.sfxHeld == nil {
		b.sfxHeld = map[*eaudio.Player]struct{}{}
	}
	b.sfxHeld[player] = struct{}{}
	b.mu.Unlock()
	return &ebitenSfxHandle{b: b, player: player, pan: pr}, nil
}

// PlaySfxHeldMemIntroLoop plays introLoopWAV [0, end) once and then loops
// [loopStart, end) in a SINGLE Oto player via eaudio.NewInfiniteLoopWithIntro, so
// there is no gap at the intro->loop transition (the startup latency of a new
// player in the two voice swap is what made the idle hum "stop and repeat").
// Implements audio.IntroLoopSfxBackend.
func (b *EbitenBackend) PlaySfxHeldMemIntroLoop(introLoopWAV []byte, loopStart time.Duration, volume, pan float64) (SfxHandle, error) {
	stream, err := wav.DecodeWithSampleRate(sampleRate, bytes.NewReader(introLoopWAV))
	if err != nil {
		return nil, fmt.Errorf("audio: intro-loop: %w", err)
	}
	data, err := decodePCM(stream)
	if err != nil {
		return nil, err
	}
	data = data[:len(data)/frameSize*frameSize] // whole frames only, so channels stay in phase
	// Split the decoded buffer at loopStart (aligned to whole frames): [0,intro)
	// plays once, then [intro,end) loops. sampleRate is the decoded (resampled)
	// context rate.
	introBytes := int64(int(loopStart.Seconds()*float64(sampleRate)) * frameSize)
	if introBytes < 0 || introBytes > int64(len(data)) {
		introBytes = 0 // loop the whole region if the offset is out of range
	}
	loopBytes := int64(len(data)) - introBytes
	// Pan after the intro and loop reader so the live SetPan applies to both the
	// intro and the looped region (the reader emits unpanned frames; panReader
	// pans them).
	pr := newPanReader(eaudio.NewInfiniteLoopWithIntro(bytes.NewReader(data), introBytes, loopBytes), pan)
	player, err := context().NewPlayer(pr)
	if err != nil {
		return nil, err
	}
	player.SetVolume(volume)
	player.Play()
	b.mu.Lock()
	if b.sfxHeld == nil {
		b.sfxHeld = map[*eaudio.Player]struct{}{}
	}
	b.sfxHeld[player] = struct{}{}
	b.mu.Unlock()
	return &ebitenSfxHandle{b: b, player: player, pan: pr}, nil
}

// ebitenSfxHandle is a playing held sound effect owned by an EbitenBackend.
type ebitenSfxHandle struct {
	b      *EbitenBackend
	player *eaudio.Player
	pan    *panReader // the live stereo pan applied to this sound's frames
}

// SetPan moves the held sound in stereo while it plays.
func (h *ebitenSfxHandle) SetPan(pan float64) {
	if h.pan != nil {
		h.pan.setPan(pan)
	}
}

// Stop ends the sound and releases its player (idempotent).
func (h *ebitenSfxHandle) Stop() {
	h.b.mu.Lock()
	defer h.b.mu.Unlock()
	if h.player == nil {
		return
	}
	if _, live := h.b.sfxHeld[h.player]; live {
		h.player.Close()
		delete(h.b.sfxHeld, h.player)
	}
	h.player = nil
}

// PlayVoice plays a voice line, cutting off the previous one.
func (b *EbitenBackend) PlayVoice(path string, volume float64) error {
	data, err := b.pcm(path)
	if err != nil {
		return err
	}
	b.mu.Lock()
	if b.voice != nil {
		b.voice.Close()
	}
	player, err := context().NewPlayer(bytes.NewReader(data))
	if err != nil {
		b.mu.Unlock()
		return err
	}
	b.voice = player
	b.mu.Unlock()
	player.SetVolume(volume)
	player.Play()
	return nil
}

// VoicePlaying reports whether a voice line is still audible.
func (b *EbitenBackend) VoicePlaying() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.voice != nil && b.voice.IsPlaying()
}

// Movie PCM is handled by the Manager's Oto moviePCMPlayer, not the backend, so
// it plays through Oto even when the configured backend is SoLoud.

// StopAll silences everything.
func (b *EbitenBackend) StopAll() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.stopMusicLocked()
	if b.voice != nil {
		b.voice.Close()
		b.voice = nil
	}
	for _, p := range b.sfxPlayers {
		p.Close()
	}
	b.sfxPlayers = nil
	for p := range b.sfxHeld {
		p.Close()
	}
	b.sfxHeld = nil
}

// Close releases resources (the context itself is shared by the whole process).
func (b *EbitenBackend) Close() error {
	b.StopAll()
	return nil
}
