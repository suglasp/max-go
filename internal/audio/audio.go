// audio.go : Package audio is the port of sound_manager.cpp.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
//
// Note: sound_manager manages music, sound effects, and voice with volumes grouped by
// channel, on top of a pluggable Backend. The Ebitengine backend lives in ebiten.go; a
// SoLoud backend is selectable through settings.json audio.backend.
//
// Sound resources in MAX.RES are tiny filename references (e.g. MBUTT0 maps to "mengens4.wav").
// The actual audio lives in loose files in the game data directory.
// Music (*.MSC) files are renamed RIFF/WAV, PCM16 stereo at 22050Hz (docs/formats/msc.md).
package audio

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/suglasp/max-go/internal/dbg"
	"github.com/suglasp/max-go/internal/resfile"
	"github.com/suglasp/max-go/internal/settings"
)

// Backend is the low-level audio device. Volumes range from 0 to 1 and pan ranges from -1 (left) to +1 (right).
type Backend interface {
	PlayMusic(path string, loop bool, volume float64) error // PlayMusic starts or replaces the streamed music track
	StopMusic()                                             // StopMusic stops the current music track
	SetMusicVolume(volume float64)                          // SetMusicVolume adjusts a running music track
	PlaySfx(path string, volume, pan float64) error         // PlaySfx plays one short sound effect, optionally panned
	PlayVoice(path string, volume float64) error            // PlayVoice starts a voice line, replacing any current voice
	VoicePlaying() bool                                     // VoicePlaying reports whether the voice channel is active
	StopAll()                                               // StopAll silences every channel owned by the backend
	Close() error                                           // Close releases backend resources
}

// MoviePCMBackend optionally streams decoded MVE audio through a backend-owned ring buffer. Other backends use the Manager's Oto movie player.
type MoviePCMBackend interface {
	StartPCM(format PCMFormat, volume float64) error // StartPCM prepares raw movie PCM playback
	QueuePCM(samples []int16) error                  // QueuePCM appends signed 16-bit movie samples to the raw PCM stream
	StopPCM()                                        // StopPCM stops raw movie PCM playback
}

// HeldSfxBackend optionally keeps a sound-effect handle so drive sounds and idle loops can be stopped early. Other backends fall back to one PlaySfx call.
//
// C++ ported : sound_manager.cpp - SoundManager's persistent per unit sfx channel m_sfx, freed by FreeSfx.
type HeldSfxBackend interface {
	// PlaySfxHeld starts a sound effect (looped when loop is set) and returns a
	// handle to stop it early.
	PlaySfxHeld(path string, volume, pan float64, loop bool) (SfxHandle, error)
}

// MemHeldSfxBackend is an optional Backend capability: play a held sound from an
// in memory WAV buffer. The Manager uses it to loop only a sound's running region
// (sliced at the smpl loop point) so a building hums without replaying its power
// up. Backends without it loop the whole clip via PlaySfxHeld.
type MemHeldSfxBackend interface {
	// PlaySfxHeldMem plays a WAV held in memory (looped when loop is set) and
	// returns a handle to stop it early.
	PlaySfxHeldMem(wav []byte, volume, pan float64, loop bool) (SfxHandle, error)
}

// IntroLoopSfxBackend is an optional Backend capability: play a held sound whose
// intro [0, end) plays once then loops [loopStart, end) seamlessly in a SINGLE
// voice (no intro/loop player swap), matching the original's smpl loop point.
// introLoopWAV holds samples [0, loopEnd); loopStart is where the loop begins.
// Backends that cannot set a loop point fall back to the Manager's two voice swap,
// which gaps on backends whose new voice does not start instantly (Oto, SoLoud).
type IntroLoopSfxBackend interface {
	PlaySfxHeldMemIntroLoop(introLoopWAV []byte, loopStart time.Duration, volume, pan float64) (SfxHandle, error)
}

// SfxHandle is a playing sound effect that can be stopped before it ends, and
// moved in the stereo field and faded while it plays (its unit or the view moved).
type SfxHandle interface {
	Stop()                    // Stop ends the sound and releases it.
	SetPan(pan float64)       // SetPan moves a held sound in stereo, pan -1 (left) .. +1 (right).
	SetVolume(volume float64) // SetVolume sets a held sound's final volume, 0..1.
}

// noopHandle is the SfxHandle returned for a fallback single play on a backend
// that cannot hold a sound: it represents a clip that is already playing and
// cannot be interrupted, so Stop does nothing (the clip plays to its end).
type noopHandle struct{}

func (noopHandle) Stop() {
	// Stop does nothing because a fallback clip cannot be interrupted.
}

func (noopHandle) SetPan(float64) {
	// SetPan does nothing because a fallback clip cannot be panned after it starts.
}

func (noopHandle) SetVolume(float64) {
	// SetVolume does nothing because a fallback clip cannot be faded after it starts.
}

// PCMFormat describes decoded signed 16-bit PCM streamed by the movie player.
type PCMFormat struct {
	Rate     int
	Channels int
	Bits     int
}

// Manager coordinates the audio backend, resource lookup, settings, and active sound handles.
//
// C++ ported : sound_manager.cpp - SoundManager, reduced (no async job queue,
// since loads are fast enough synchronously); the shuffle playlist follows
// with the game screen chunk.
type Manager struct {
	be      Backend
	lib     *resfile.Library
	dataDir string
	cfg     settings.Audio

	musicTag      string // the track currently playing ("" = none)
	lastMusicTag  string // desired track remembered while stopped, to resume after the player turns music back on
	lastMusicLoop bool

	moviePCMMu sync.Mutex      // guards moviePCM/moviePCMBE (decode goroutine vs main goroutine)
	moviePCM   *moviePCMPlayer // streams movie audio through Oto regardless of backend (ebiten / soloud fallback)
	moviePCMBE MoviePCMBackend // native movie audio path (miniaudio); nil when using Oto

	sfxHeld    map[int]SfxHandle // active held sound effects (e.g. engine drive) by handle id, so one can be stopped early
	nextHeldID int               // next sfxHeld handle id to hand out
}

// New creates the manager. cfg supplies the volume/enable settings.
func New(be Backend, lib *resfile.Library, dataDir string, cfg settings.Audio) *Manager {
	return &Manager{be: be, lib: lib, dataDir: dataDir, cfg: cfg}
}

// resolvePath turns a sound resource tag into the path of the loose
// audio file it references.
func (m *Manager) resolvePath(tag string) (string, error) {
	// 1. MAX.RES maps the tag to a clip filename; load that file from the data dir.
	if name, err := m.lib.ReadString(tag); err == nil {
		if name = strings.TrimSpace(name); name != "" {
			if p, ok := m.findDataFile(name); ok {
				return p, nil
			}
		}
	}

	// 2. Fallback: a loose <tag>.WAV in the data dir. The port ships building hum and
	//    other clips as standalone WAVs (e.g. MASTR17.WAV, HVP17.WAV) that have no
	//    MAX.RES string entry, so resolve them by the tag name directly.
	if p, ok := m.findDataFile(tag + ".WAV"); ok {
		return p, nil
	}

	return "", fmt.Errorf("audio: %s: no resource string and no %s.WAV in %s", tag, tag, m.dataDir)
}

// findDataFile returns the path to name, searching the game data dir
// (MAX_DATA_DIR) and the directory the binary resides in (loose clips shipped next
// to max-go, e.g. MASTR17.WAV). Each dir is matched by exact case first, then
// without regard to case (for Linux). Returns the path and whether it was found.
func (m *Manager) findDataFile(name string) (string, bool) {
	for _, dir := range m.searchDirs() {
		if dir == "" {
			continue
		}

		exact := filepath.Join(dir, name)
		if _, err := os.Stat(exact); err == nil {
			return exact, true
		}

		list, err := os.ReadDir(dir)
		if err != nil {
			continue
		}

		for _, e := range list {
			if !e.IsDir() && strings.EqualFold(e.Name(), name) {
				return filepath.Join(dir, e.Name()), true
			}
		}
	}

	return "", false
}

// searchDirs lists the directories sound clips load from: the game data dir, then
// the directory the binary lives in (its loose WAVs). C++/project rule: data is
// found under MAX_DATA_DIR or next to the executable.
func (m *Manager) searchDirs() []string {
	dirs := []string{m.dataDir}

	if base, err := settings.BaseDir(); err == nil && base != m.dataDir {
		dirs = append(dirs, base)
	}

	return dirs
}

// vol converts a settings percentage into the backend's 0..1 volume scale.
func vol(level int) float64 {
	return float64(level) / 100
}

// PlayMusic starts the music track referenced by tag, looping when
// loop is set. A second call with the same tag keeps the current
// playback.
//
// C++ ported : sound_manager.cpp - SoundManager::PlayMusic
func (m *Manager) PlayMusic(tag string, loop bool) {
	// Remember the intended track even while music is gated, so turning it back on
	// (or raising the volume from 0) resumes it via SetVolumes. Without this, music
	// disabled at game start leaves nothing to resume.
	m.lastMusicTag, m.lastMusicLoop = tag, loop
	if !m.cfg.MusicEnabled || m.cfg.MusicVolume == 0 {
		return
	}

	if m.musicTag == tag {
		return
	}

	path, err := m.resolvePath(tag)
	if err != nil {
		dbg.Printf("music %s: %v", tag, err)
		return
	}

	m.be.StopMusic()
	if err := m.be.PlayMusic(path, loop, vol(m.cfg.MusicVolume)); err != nil {
		m.disableAudio("music", tag, err)
		return
	}

	m.musicTag = tag

	dbg.Printf("music (%s) : playing (vol %d)", tag, m.cfg.MusicVolume)
}

// StopMusic stops the current track.
func (m *Manager) StopMusic() {
	m.be.StopMusic()
	m.musicTag = ""
}

// StartMoviePCM prepares decoded movie (MVE) audio playback: a native
// MoviePCMBackend (miniaudio) plays through its own ring buffer, otherwise it
// falls back to the Manager's Oto player. A failure to open the movie device is
// not fatal and must not silence music, sfx, or voice.
func (m *Manager) StartMoviePCM(format PCMFormat) {
	if !m.cfg.SfxEnabled || m.cfg.SfxVolume == 0 {
		return
	}

	m.moviePCMMu.Lock()
	defer m.moviePCMMu.Unlock()

	m.stopMoviePCMLocked()
	if be, ok := m.be.(MoviePCMBackend); ok {
		if err := be.StartPCM(format, vol(m.cfg.SfxVolume)); err != nil {
			dbg.Printf("movie pcm (backend): %v; movie audio disabled", err)
			return
		}
		m.moviePCMBE = be
		return
	}

	player, err := newMoviePCMPlayer(format, vol(m.cfg.SfxVolume))
	if err != nil {
		dbg.Printf("movie pcm: %v; movie audio disabled", err)
		return
	}

	m.moviePCM = player
}

// QueueMoviePCM queues decoded movie PCM on whichever movie path is active.
func (m *Manager) QueueMoviePCM(samples []int16) {
	if len(samples) == 0 || !m.cfg.SfxEnabled || m.cfg.SfxVolume == 0 {
		return
	}

	m.moviePCMMu.Lock()
	be := m.moviePCMBE
	player := m.moviePCM
	m.moviePCMMu.Unlock()

	if be != nil {
		if err := be.QueuePCM(samples); err != nil {
			dbg.Printf("movie pcm (backend) queue: %v", err)
		}
		return
	}

	player.queue(samples)
}

// StopMoviePCM stops and releases movie PCM playback.
func (m *Manager) StopMoviePCM() {
	m.moviePCMMu.Lock()
	defer m.moviePCMMu.Unlock()

	m.stopMoviePCMLocked()
}

// stopMoviePCMLocked stops whichever movie path is active; m.moviePCMMu is held.
func (m *Manager) stopMoviePCMLocked() {
	if m.moviePCMBE != nil {
		m.moviePCMBE.StopPCM()
		m.moviePCMBE = nil
	}

	if m.moviePCM != nil {
		m.moviePCM.stop()
		m.moviePCM = nil
	}
}

// PlaySfx plays a sound effect (centered).
//
// C++ ported : sound_manager.cpp - SoundManager::PlaySfx
func (m *Manager) PlaySfx(tag string) {
	m.PlaySfxPannedFallback(tag, "", 0)
}

/* // note: unused code in Go port
// PlaySfxPanned was only called by PlaySfx with pan 0; PlaySfx calls PlaySfxPannedFallback directly.
// PlaySfxPanned plays a sound effect with stereo panning.
func (m *Manager) PlaySfxPanned(tag string, pan float64) {
	m.PlaySfxPannedFallback(tag, "", pan)
}
*/

// PlaySfxPannedFallback plays the first of primary/fallback that resolves to a
// real file, at the given pan, mirroring the C++ fallback to the default sound
// when a unit specific clip is missing. Empty tags and a fallback equal to the
// primary are skipped.
//
// C++ ported : sound_manager.cpp - SoundManager::PlaySfx with the resolved SoundEffectInfo.
func (m *Manager) PlaySfxPannedFallback(primary, fallback string, pan float64) {
	m.PlaySfxPositional(primary, fallback, pan, 1)
}

// PlaySfxPositional is PlaySfxPannedFallback with a volume factor on top of the
// SFX group volume: the positional fade of a unit's sound (its distance from the
// view centre and the zoom).
//
// C++ ported : sound_manager.cpp - SoundManager::PlaySfx (volume_1 = volume_2 * attenuation)
func (m *Manager) PlaySfxPositional(primary, fallback string, pan, volScale float64) {
	if !m.cfg.SfxEnabled || m.cfg.SfxVolume == 0 {
		return
	}

	tried := ""
	for _, tag := range [2]string{primary, fallback} {
		if tag == "" || tag == tried {
			continue
		}

		tried = tag
		path, err := m.resolvePath(tag)
		if err != nil {
			continue // not in this archive; try the fallback
		}

		if err := m.be.PlaySfx(path, vol(m.cfg.SfxVolume)*volScale, pan); err != nil {
			// A play error here is a backend/device failure (a missing or
			// undecodable clip is caught by resolvePath above), so fail over to
			// silence rather than erroring every later sound.
			m.disableAudio("sfx", tag, err)
		}

		return
	}

	dbg.Printf("sfx (%s/%s) : no resolvable clip", primary, fallback)
}

// PlaySfxHeld starts a panned sound effect and returns a handle id to stop it
// early (0 = nothing started). volScale multiplies the SFX group volume; loop
// repeats the clip (idle hum) versus playing once (drive sound). Plays the first
// of primary/fallback that resolves. A backend that cannot hold a handle falls
// back to a single fire and forget play, still tracked by an inert handle so the
// caller does not retrigger and stack it.
//
// C++ ported : sound_manager.cpp - SoundManager's persistent per unit sfx channel, freed by FreeSfx.
func (m *Manager) PlaySfxHeld(primary, fallback string, pan, volScale float64, loop bool) int {
	if !m.cfg.SfxEnabled || m.cfg.SfxVolume == 0 {
		return 0
	}

	v := vol(m.cfg.SfxVolume) * volScale
	held, canHold := m.be.(HeldSfxBackend)
	tried := ""

	for _, tag := range [2]string{primary, fallback} {
		if tag == "" || tag == tried {
			continue
		}

		tried = tag
		path, err := m.resolvePath(tag)
		if err != nil {
			continue // not in this archive; try the fallback
		}

		if !canHold {
			// No handle support: play a single fire and forget clip but still hand
			// back an inert handle id, so the caller records it and does NOT retrigger
			// every frame (which would stack overlapping copies). The clip plays to
			// its end and cannot be cut short or looped.
			if err := m.be.PlaySfx(path, v, pan); err != nil {
				m.disableAudio("sfx", tag, err)
				return 0
			}

			if m.sfxHeld == nil {
				m.sfxHeld = map[int]SfxHandle{}
			}

			m.nextHeldID++
			m.sfxHeld[m.nextHeldID] = noopHandle{}

			return m.nextHeldID
		}

		// A looping sound with a smpl loop region (power up plus running, e.g.
		// HVP17/POWGN17) loops only the running region so the power up is not
		// replayed. Slice the WAV at its loop point and loop the slice in memory;
		// fall back to looping the whole clip when there is no region or the backend
		// can't play from memory.
		var handle SfxHandle
		var err2 error

		if loop {
			if mem, ok := m.be.(MemHeldSfxBackend); ok {
				if raw, rerr := os.ReadFile(path); rerr == nil {
					if slice := sliceFromLoop(raw); slice != nil {
						dbg.Printf("sfx (%s) : looping running region (%d-byte slice of %d)", tag, len(slice), len(raw))
						handle, err2 = mem.PlaySfxHeldMem(slice, v, pan, true)
					}
				}
			}
		}

		if handle == nil && err2 == nil {
			handle, err2 = held.PlaySfxHeld(path, v, pan, loop)
		}

		if err2 != nil {
			m.disableAudio("sfx", tag, err2)

			return 0
		}

		if m.sfxHeld == nil {
			m.sfxHeld = map[int]SfxHandle{}
		}

		m.nextHeldID++
		m.sfxHeld[m.nextHeldID] = handle

		return m.nextHeldID
	}

	dbg.Printf("sfx (%s/%s) : held, no resolvable clip", primary, fallback)

	return 0
}

// StopSfxHeld stops and releases the held sound effect with the given handle id
// (a 0 id or an unknown id does nothing).
func (m *Manager) StopSfxHeld(id int) {
	if id == 0 {
		return
	}

	if handle, ok := m.sfxHeld[id]; ok {
		handle.Stop()
		delete(m.sfxHeld, id)
	}
}

// SetSfxHeldPan pans the live held sound again with the given handle id so a
// moving source (a unit driving across the screen) tracks the stereo field. A
// 0 or unknown id does nothing, and neither does a backend that does not pan
// (the handle ignores it).
func (m *Manager) SetSfxHeldPan(id int, pan float64) {
	if id == 0 {
		return
	}

	if handle, ok := m.sfxHeld[id]; ok {
		handle.SetPan(pan)
	}
}

// SetSfxHeldVolume fades the live held sound with the given handle id to the SFX
// group volume times volScale, the same product PlaySfxHeld starts it with. A 0 or
// unknown id does nothing.
//
// C++ ported : sound_manager.cpp - SoundManager::UpdateSfxPosition (ma_sound_set_volume)
func (m *Manager) SetSfxHeldVolume(id int, volScale float64) {
	if id == 0 {
		return
	}

	if handle, ok := m.sfxHeld[id]; ok {
		handle.SetVolume(vol(m.cfg.SfxVolume) * volScale)
	}
}

// PlayVoice plays a voice line, interrupting a running one.
//
// C++ ported : sound_manager.cpp - SoundManager::PlayVoice
func (m *Manager) PlayVoice(tag string) {
	if !m.cfg.VoiceEnabled || m.cfg.VoiceVolume == 0 {
		return
	}

	path, err := m.resolvePath(tag)
	if err != nil {
		dbg.Printf("voice %s: %v", tag, err)
		return
	}

	if err := m.be.PlayVoice(path, vol(m.cfg.VoiceVolume)); err != nil {
		m.disableAudio("voice", tag, err)
	}
}

// VoicePlaying reports whether a voice line is still audible, so the caller can
// serialise announcer lines one after another instead of cutting them off.
func (m *Manager) VoicePlaying() bool {
	return m.be != nil && m.be.VoicePlaying()
}

// SetVolumes applies changed audio settings, starting or stopping music live so
// the options menu's Disable Music toggle works both ways: disabling stops the
// track (remembered as lastMusicTag), turning it back on resumes it.
func (m *Manager) SetVolumes(cfg settings.Audio) {
	m.cfg = cfg

	switch {
	case !cfg.MusicEnabled || cfg.MusicVolume == 0:
		m.StopMusic() // keeps lastMusicTag so it can be resumed

	case m.musicTag == "" && m.lastMusicTag != "":
		m.PlayMusic(m.lastMusicTag, m.lastMusicLoop) // resume after being turned back on

	default:
		m.be.SetMusicVolume(vol(cfg.MusicVolume))
	}

	// Disabling SFX or setting volume to zero stops active idle and drive loops.
	// SfxHandle cannot change volume during playback, so later triggers start with
	// the new setting.
	if !cfg.SfxEnabled || cfg.SfxVolume == 0 {
		m.stopAllHeldSfx()
	}
}

// stopAllHeldSfx stops and releases every active held sound effect.
func (m *Manager) stopAllHeldSfx() {
	for id, handle := range m.sfxHeld {
		handle.Stop()

		delete(m.sfxHeld, id)
	}
}

// Close stops everything and releases the backend.
func (m *Manager) Close() {
	m.StopMoviePCM()
	m.be.StopAll()
	m.be.Close()
}

// disableAudio tears down a failing backend and replaces it with silence so
// gameplay can continue after an audio device or decoder error.
func (m *Manager) disableAudio(kind, tag string, err error) {
	dbg.Printf("%s %s: %v; disabling audio", kind, tag, err)
	m.be.StopAll()

	if closeErr := m.be.Close(); closeErr != nil {
		dbg.Printf("audio close after failure: %v", closeErr)
	}

	m.be = NewSilentBackend()
	m.cfg.MusicEnabled = false
	m.cfg.SfxEnabled = false
	m.cfg.VoiceEnabled = false
	m.musicTag = ""
	m.sfxHeld = nil // StopAll above already closed the held players
}
