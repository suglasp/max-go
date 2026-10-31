// audio_test.go : testing unit for audio.go
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package audio

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/hajimehoshi/ebiten/v2/audio/wav"

	"github.com/suglasp/max-go/internal/resfile"
	"github.com/suglasp/max-go/internal/settings"
	"github.com/suglasp/max-go/internal/testutil"
)

// fakeBackend records calls for Manager tests.
type fakeBackend struct {
	musicPath string
	musicLoop bool
	musicVol  float64
	sfxPaths  []string
	sfxVols   []float64
	voicePath string
	stops     int
}

func (f *fakeBackend) PlayMusic(path string, loop bool, volume float64) error {
	f.musicPath, f.musicLoop, f.musicVol = path, loop, volume
	return nil
}
func (f *fakeBackend) StopMusic()             { f.stops++; f.musicPath = "" }
func (f *fakeBackend) SetMusicVolume(float64) {}
func (f *fakeBackend) PlaySfx(path string, volume, _ float64) error {
	f.sfxPaths = append(f.sfxPaths, path)
	f.sfxVols = append(f.sfxVols, volume)
	return nil
}
func (f *fakeBackend) PlayVoice(path string, _ float64) error {
	f.voicePath = path
	return nil
}
func (f *fakeBackend) VoicePlaying() bool { return false }
func (f *fakeBackend) StopAll()           {}
func (f *fakeBackend) Close() error       { return nil }

type failingBackend struct {
	fakeBackend
	closed bool
}

func (f *failingBackend) PlaySfx(string, float64, float64) error {
	return errors.New("device unavailable")
}

func (f *failingBackend) Close() error {
	f.closed = true
	return nil
}

func newManager(t *testing.T, cfg settings.Audio) (*Manager, *fakeBackend) {
	t.Helper()
	dir := testutil.DataDir(t)
	lib, err := resfile.OpenDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { lib.Close() })
	be := &fakeBackend{}
	return New(be, lib, dir, cfg), be
}

func enabledCfg() settings.Audio {
	return settings.Audio{
		MusicEnabled: true, SfxEnabled: true, VoiceEnabled: true,
		MusicVolume: 80, SfxVolume: 100, VoiceVolume: 50,
	}
}

func TestResolveSoundResources(t *testing.T) {
	m, _ := newManager(t, enabledCfg())

	// Sound resources are filename references to loose files.
	tests := map[string]string{
		"MAIN_MSC": "main_msc.msc",
		"MBUTT0":   "mengens4.wav",
		"NDONE0":   "menu38.wav",
	}
	for tag, want := range tests {
		path, err := m.resolvePath(tag)
		if err != nil {
			t.Fatalf("%s: %v", tag, err)
		}
		if !strings.EqualFold(strings.TrimSuffix(path[strings.LastIndexByte(path, '\\')+1:], ""), want) {
			t.Errorf("%s resolved to %s, want file %s", tag, path, want)
		}
		if _, err := os.Stat(path); err != nil {
			t.Errorf("%s: referenced file missing: %v", tag, err)
		}
	}
}

func TestManagerGating(t *testing.T) {
	m, be := newManager(t, enabledCfg())

	m.PlayMusic("MAIN_MSC", true)
	if be.musicPath == "" || !be.musicLoop || be.musicVol != 0.8 {
		t.Errorf("music call = %q loop=%v vol=%v", be.musicPath, be.musicLoop, be.musicVol)
	}
	// Same tag again: no restart.
	prevStops := be.stops
	m.PlayMusic("MAIN_MSC", true)
	if be.stops != prevStops {
		t.Error("same track restarted")
	}

	m.PlaySfx("MBUTT0")
	if len(be.sfxPaths) != 1 {
		t.Errorf("sfx calls = %v", be.sfxPaths)
	}

	// Disabled groups stay silent.
	cfg := enabledCfg()
	cfg.SfxEnabled = false
	cfg.MusicEnabled = false
	m2, be2 := newManager(t, cfg)
	m2.PlaySfx("MBUTT0")
	m2.PlayMusic("MAIN_MSC", true)
	if len(be2.sfxPaths) != 0 || be2.musicPath != "" {
		t.Error("disabled audio still played")
	}

	// Zero volume behaves like disabled.
	cfg = enabledCfg()
	cfg.VoiceVolume = 0
	m3, be3 := newManager(t, cfg)
	m3.PlayVoice("MBUTT0")
	if be3.voicePath != "" {
		t.Error("zero-volume voice still played")
	}
}

// TestManagerMusicToggle checks that the options menu stops music when disabled and resumes the same track when re-enabled.
func TestManagerMusicToggle(t *testing.T) {
	cfg := enabledCfg()
	m, be := newManager(t, cfg)

	m.PlayMusic("MAIN_MSC", true)
	if be.musicPath == "" {
		t.Fatal("music did not start")
	}

	// Disabling music stops the current track.
	cfg.MusicEnabled = false
	m.SetVolumes(cfg)
	if be.musicPath != "" {
		t.Error("music not stopped when disabled")
	}

	// Re-enabling music resumes the remembered track.
	cfg.MusicEnabled = true
	m.SetVolumes(cfg)
	if be.musicPath == "" || !be.musicLoop {
		t.Errorf("music not resumed on re-enable: path=%q loop=%v", be.musicPath, be.musicLoop)
	}
}

// TestManagerMusicResumeAfterDisabledStart checks that PlayMusic remembers a request made while music is disabled, allowing SetVolumes to start it later.
func TestManagerMusicResumeAfterDisabledStart(t *testing.T) {
	cfg := enabledCfg()
	cfg.MusicEnabled = false // music off when the track is first requested
	m, be := newManager(t, cfg)

	m.PlayMusic("MAIN_MSC", true) // gated: nothing should play yet
	if be.musicPath != "" {
		t.Fatal("music played while disabled")
	}

	cfg.MusicEnabled = true
	m.SetVolumes(cfg)
	if be.musicPath == "" || !be.musicLoop {
		t.Errorf("music not started on enable after a disabled start: path=%q loop=%v", be.musicPath, be.musicLoop)
	}
}

// Movie PCM no longer goes through the configured Backend (it streams through the
// Manager's Oto player; see moviePCMPlayer), so the only thing worth asserting
// here regardless of device is that disabled SFX never opens a movie player.
func TestManagerMoviePCMSkippedWhenSfxDisabled(t *testing.T) {
	cfg := enabledCfg()
	cfg.SfxEnabled = false
	m, _ := newManager(t, cfg)
	m.StartMoviePCM(PCMFormat{Rate: 22050, Channels: 2, Bits: 16})
	m.QueueMoviePCM([]int16{1, 2})
	m.StopMoviePCM()
	if m.moviePCM != nil {
		t.Fatal("disabled sfx still started movie PCM")
	}
}

func TestManagerDisablesAudioOnBackendError(t *testing.T) {
	dir := testutil.DataDir(t)
	lib, err := resfile.OpenDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { lib.Close() })

	be := &failingBackend{}
	m := New(be, lib, dir, enabledCfg())
	m.PlaySfx("MBUTT0")

	if !be.closed {
		t.Fatal("failing backend was not closed")
	}
	if _, ok := m.be.(*SilentBackend); !ok {
		t.Fatalf("backend = %T, want *SilentBackend", m.be)
	}
	if m.cfg.MusicEnabled || m.cfg.SfxEnabled || m.cfg.VoiceEnabled {
		t.Fatal("audio groups were not disabled")
	}
}

// heldFakeBackend adds HeldSfxBackend support to fakeBackend and records each returned handle.
type heldFakeBackend struct {
	fakeBackend
	held []*fakeSfxHandle
	mem  []*fakeSfxHandle
}

type fakeSfxHandle struct {
	path    string
	wav     []byte
	loop    bool
	stopped bool
	pan     float64
	volume  float64
}

func (f *fakeSfxHandle) Stop()               { f.stopped = true }
func (f *fakeSfxHandle) SetPan(p float64)    { f.pan = p }
func (f *fakeSfxHandle) SetVolume(v float64) { f.volume = v }

func (f *heldFakeBackend) PlaySfxHeld(path string, _, _ float64, loop bool) (SfxHandle, error) {
	h := &fakeSfxHandle{path: path, loop: loop}
	f.held = append(f.held, h)
	return h, nil
}

func (f *heldFakeBackend) PlaySfxHeldMem(wav []byte, _, _ float64, loop bool) (SfxHandle, error) {
	h := &fakeSfxHandle{path: "mem", wav: append([]byte(nil), wav...), loop: loop}
	f.mem = append(f.mem, h)
	return h, nil
}

type fakeTimer struct {
	stopped bool
}

func (t *fakeTimer) Stop() bool {
	t.stopped = true
	return true
}

// introLoopFakeBackend adds the IntroLoopSfxBackend capability so the single
// voice intro plus loop path (no swap) can be exercised.
type introLoopFakeBackend struct {
	heldFakeBackend
	introLoop []*fakeIntroLoop
}

type fakeIntroLoop struct {
	wav       []byte
	loopStart time.Duration
	stopped   bool
	pan       float64
	volume    float64
}

func (f *fakeIntroLoop) Stop()               { f.stopped = true }
func (f *fakeIntroLoop) SetPan(p float64)    { f.pan = p }
func (f *fakeIntroLoop) SetVolume(v float64) { f.volume = v }

func (f *introLoopFakeBackend) PlaySfxHeldMemIntroLoop(wav []byte, loopStart time.Duration, _, _ float64) (SfxHandle, error) {
	h := &fakeIntroLoop{wav: append([]byte(nil), wav...), loopStart: loopStart}
	f.introLoop = append(f.introLoop, h)
	return h, nil
}

// TestManagerSfxIntroThenLoopSingleVoice checks that a backend with native loop points receives one [0, loopEnd) WAV and its loop-start offset.
func TestManagerSfxIntroThenLoopSingleVoice(t *testing.T) {
	lib, err := resfile.OpenDir(testutil.DataDir(t))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { lib.Close() })
	dir := t.TempDir()
	// 1000 frames, loop [200, 900): a 100-frame tail after the loop end.
	if err := os.WriteFile(filepath.Join(dir, "CUSTOM.WAV"), buildWAVWithLoop(1000, 200, 900, true), 0o644); err != nil {
		t.Fatal(err)
	}

	be := &introLoopFakeBackend{}
	m := New(be, lib, dir, enabledCfg())
	id := m.PlaySfxIntroThenLoop("CUSTOM", "", 0.25, 0.5)
	if id == 0 {
		t.Fatal("intro-loop returned no handle")
	}
	// The native loop path does not create separate intro and loop voices.
	if len(be.held) != 0 || len(be.mem) != 0 {
		t.Errorf("single-voice path should not use the swap: held=%d mem=%d", len(be.held), len(be.mem))
	}
	if len(be.introLoop) != 1 {
		t.Fatalf("intro-loop calls = %d, want 1", len(be.introLoop))
	}
	il := be.introLoop[0]
	if want := time.Duration(200) * time.Second / 22050; il.loopStart != want {
		t.Errorf("loopStart = %v, want %v", il.loopStart, want)
	}
	// The intro loop WAV excludes the tail after the loop: [0,900) = 900 frames.
	w, ok := parseWAV(il.wav)
	if !ok {
		t.Fatal("intro-loop WAV not parseable")
	}
	if frames := (w.dataEnd - w.dataStart) / w.blockAlign; frames != 900 {
		t.Errorf("intro-loop WAV frames = %d, want 900 (tail dropped)", frames)
	}
	// StopSfxHeld stops the single voice.
	m.StopSfxHeld(id)
	if !il.stopped {
		t.Error("StopSfxHeld did not stop the intro-loop voice")
	}
}

// TestManagerSfxHeld checks stoppable handles and the single-play fallback used by backends without held-sound support.
func TestManagerSfxHeld(t *testing.T) {
	dir := testutil.DataDir(t)
	lib, err := resfile.OpenDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { lib.Close() })

	// A held-capable backend returns a live, nonzero handle.
	be := &heldFakeBackend{}
	m := New(be, lib, dir, enabledCfg())
	id := m.PlaySfxHeld("MBUTT0", "", 0, 1.0, true)
	if id == 0 {
		t.Fatal("held-capable backend returned no handle")
	}
	if len(be.held) != 1 || be.held[0].stopped {
		t.Fatalf("held sound not started: %+v", be.held)
	}
	if !be.held[0].loop {
		t.Error("loop flag not passed through to backend")
	}
	if len(be.sfxPaths) != 0 {
		t.Errorf("a backend that can hold sounds should not fire and forget: %v", be.sfxPaths)
	}
	m.StopSfxHeld(id)
	if !be.held[0].stopped {
		t.Error("StopSfxHeld did not stop the sound")
	}
	m.StopSfxHeld(id) // an id already used does nothing
	m.StopSfxHeld(0)  // a zero id does nothing

	// A plain backend plays once and returns an inert handle so callers do not retrigger the sound every frame.
	plain, beP := newManager(t, enabledCfg())
	pid := plain.PlaySfxHeld("MBUTT0", "", 0, 1.0, false)
	if pid == 0 {
		t.Error("plain fallback returned no handle (would trigger it again and stack)")
	}
	if len(beP.sfxPaths) != 1 {
		t.Errorf("plain fallback fire and forget calls = %v", beP.sfxPaths)
	}
	plain.StopSfxHeld(pid) // does nothing, but must not panic

	// Disabled sfx: nothing starts.
	cfg := enabledCfg()
	cfg.SfxEnabled = false
	be2 := &heldFakeBackend{}
	m2 := New(be2, lib, dir, cfg)
	if id := m2.PlaySfxHeld("MBUTT0", "", 0, 1.0, true); id != 0 || len(be2.held) != 0 {
		t.Error("disabled sfx still started a held sound")
	}
}

// TestManagerSetSfxHeldPan checks that a moving source can update a held sound's pan and that zero or unknown handles do nothing.
func TestManagerSetSfxHeldPan(t *testing.T) {
	dir := testutil.DataDir(t)
	lib, err := resfile.OpenDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { lib.Close() })

	be := &heldFakeBackend{}
	m := New(be, lib, dir, enabledCfg())
	id := m.PlaySfxHeld("MBUTT0", "", 0, 1.0, false)
	if id == 0 || len(be.held) != 1 {
		t.Fatalf("held sound not started: id=%d held=%d", id, len(be.held))
	}

	// Pan the live sound again: the backend handle records the new value.
	m.SetSfxHeldPan(id, 0.75)
	if be.held[0].pan != 0.75 {
		t.Errorf("SetSfxHeldPan did not forward pan: got %v, want 0.75", be.held[0].pan)
	}
	m.SetSfxHeldPan(id, -0.5)
	if be.held[0].pan != -0.5 {
		t.Errorf("SetSfxHeldPan did not update pan: got %v, want -0.5", be.held[0].pan)
	}

	// An unknown or zero id must not panic.
	m.SetSfxHeldPan(id+1234, 1.0)
	m.SetSfxHeldPan(0, 1.0)
}

// TestManagerPositionalSfxVolume checks the positional fade on top of the SFX group volume: a one shot
// positional play and a live held sound both land at group volume x volScale.
func TestManagerPositionalSfxVolume(t *testing.T) {
	dir := testutil.DataDir(t)
	lib, err := resfile.OpenDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { lib.Close() })

	cfg := enabledCfg()
	cfg.SfxVolume = 50
	be := &heldFakeBackend{}
	m := New(be, lib, dir, cfg)
	group := vol(cfg.SfxVolume)

	m.PlaySfxPositional("MBUTT0", "", 0.25, 0.5)
	if len(be.sfxVols) != 1 || be.sfxVols[0] != group*0.5 {
		t.Errorf("one shot volume = %v, want %v", be.sfxVols, group*0.5)
	}

	id := m.PlaySfxHeld("MBUTT0", "", 0, 1.0, true)
	if id == 0 || len(be.held) != 1 {
		t.Fatalf("held sound not started: id=%d held=%d", id, len(be.held))
	}
	m.SetSfxHeldVolume(id, 0.25)
	if be.held[0].volume != group*0.25 {
		t.Errorf("held volume = %v, want %v", be.held[0].volume, group*0.25)
	}
	m.SetSfxHeldVolume(id+99, 1) // an unknown id does nothing
	m.SetSfxHeldVolume(0, 1)
}

// TestIntroLoopHandleKeepsVolume checks that a fade set during the intro voice carries over to the loop voice.
func TestIntroLoopHandleKeepsVolume(t *testing.T) {
	intro, loop := &fakeSfxHandle{volume: 1}, &fakeSfxHandle{volume: 1}
	h := &introLoopHandle{current: intro}
	h.SetVolume(0.3)
	if intro.volume != 0.3 {
		t.Errorf("intro voice volume = %v, want 0.3", intro.volume)
	}
	h.swap(loop)
	if loop.volume != 0.3 {
		t.Errorf("loop voice volume after the swap = %v, want 0.3", loop.volume)
	}

	untouched := &fakeSfxHandle{volume: 1}
	(&introLoopHandle{current: &fakeSfxHandle{}}).swap(untouched)
	if untouched.volume != 1 {
		t.Errorf("a never faded handle changed the loop voice volume to %v", untouched.volume)
	}
}

func TestManagerSfxIntroThenLoop(t *testing.T) {
	lib, err := resfile.OpenDir(testutil.DataDir(t))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { lib.Close() })
	dir := t.TempDir()
	path := filepath.Join(dir, "CUSTOM.WAV")
	if err := os.WriteFile(path, buildWAV(22050, 11025, true), 0o644); err != nil {
		t.Fatal(err)
	}

	origAfter := afterSfxIntro
	var scheduled func()
	var delay time.Duration
	afterSfxIntro = func(d time.Duration, f func()) sfxTimer {
		delay = d
		scheduled = f
		return &fakeTimer{}
	}
	defer func() { afterSfxIntro = origAfter }()

	be := &heldFakeBackend{}
	m := New(be, lib, dir, enabledCfg())
	id := m.PlaySfxIntroThenLoop("CUSTOM", "", 0.25, 0.5)
	if id == 0 {
		t.Fatal("intro-loop returned no handle")
	}
	if len(be.held) != 1 || be.held[0].loop {
		t.Fatalf("intro phase = %+v, want one non-looped held sound", be.held)
	}
	if be.held[0].path != path {
		t.Fatalf("intro path = %q, want %q", be.held[0].path, path)
	}
	if len(be.mem) != 0 {
		t.Fatalf("loop started before timer: %+v", be.mem)
	}
	if delay != time.Second {
		t.Fatalf("intro delay = %v, want 1s", delay)
	}
	if scheduled == nil {
		t.Fatal("intro-loop did not schedule loop phase")
	}

	scheduled()
	if !be.held[0].stopped {
		t.Fatal("intro handle was not stopped when loop phase started")
	}
	if len(be.mem) != 1 || !be.mem[0].loop {
		t.Fatalf("loop phase = %+v, want one looped memory sound", be.mem)
	}
	m.StopSfxHeld(id)
	if !be.mem[0].stopped {
		t.Fatal("StopSfxHeld did not stop the loop phase")
	}
}

func TestManagerSfxIntroThenLoopUsesLoopEndBoundary(t *testing.T) {
	lib, err := resfile.OpenDir(testutil.DataDir(t))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { lib.Close() })
	dir := t.TempDir()
	path := filepath.Join(dir, "CUSTOM.WAV")
	if err := os.WriteFile(path, buildWAVWithLoop(24000, 11025, 22050, true), 0o644); err != nil {
		t.Fatal(err)
	}

	origAfter := afterSfxIntro
	var scheduled func()
	var delay time.Duration
	afterSfxIntro = func(d time.Duration, f func()) sfxTimer {
		delay = d
		scheduled = f
		return &fakeTimer{}
	}
	defer func() { afterSfxIntro = origAfter }()

	be := &heldFakeBackend{}
	m := New(be, lib, dir, enabledCfg())
	id := m.PlaySfxIntroThenLoop("CUSTOM", "", 0, 1)
	if id == 0 {
		t.Fatal("intro-loop returned no handle")
	}
	if delay != time.Second {
		t.Fatalf("intro delay = %v, want 1s at smpl loop end, not full file duration", delay)
	}
	scheduled()
	if len(be.mem) != 1 {
		t.Fatalf("loop phase count = %d, want 1", len(be.mem))
	}
	info, ok := parseWAV(be.mem[0].wav)
	if !ok {
		t.Fatal("loop WAV does not parse")
	}
	if got, want := (info.dataEnd-info.dataStart)/info.blockAlign, 11025; got != want {
		t.Fatalf("loop WAV frames = %d, want %d", got, want)
	}
}

func TestManagerSfxIntroThenLoopStopBeforeLoop(t *testing.T) {
	lib, err := resfile.OpenDir(testutil.DataDir(t))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { lib.Close() })
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "CUSTOM.WAV"), buildWAV(22050, 11025, true), 0o644); err != nil {
		t.Fatal(err)
	}

	origAfter := afterSfxIntro
	var scheduled func()
	afterSfxIntro = func(_ time.Duration, f func()) sfxTimer {
		scheduled = f
		return &fakeTimer{}
	}
	defer func() { afterSfxIntro = origAfter }()

	be := &heldFakeBackend{}
	m := New(be, lib, dir, enabledCfg())
	id := m.PlaySfxIntroThenLoop("CUSTOM", "", 0, 1)
	if id == 0 {
		t.Fatal("intro-loop returned no handle")
	}
	m.StopSfxHeld(id)
	if !be.held[0].stopped {
		t.Fatal("StopSfxHeld did not stop the intro phase")
	}
	scheduled()
	if len(be.mem) != 1 {
		t.Fatalf("timer should create and immediately stop one loop handle, got %d", len(be.mem))
	}
	if !be.mem[0].stopped {
		t.Fatal("late loop handle was not stopped after pre-timer Stop")
	}
}

func TestRealFilesDecode(t *testing.T) {
	m, _ := newManager(t, enabledCfg())

	// The referenced files must be decodable RIFF/WAV (the .MSC music
	// too: it is just a renamed WAV).
	for _, tag := range []string{"MAIN_MSC", "MBUTT0"} {
		path, err := m.resolvePath(tag)
		if err != nil {
			t.Fatal(err)
		}
		f, err := os.Open(path)
		if err != nil {
			t.Fatal(err)
		}
		stream, err := wav.DecodeWithSampleRate(sampleRate, f)
		if err != nil {
			f.Close()
			t.Fatalf("%s (%s): %v", tag, path, err)
		}
		if stream.Length() <= 0 {
			t.Errorf("%s: empty audio stream", tag)
		}
		f.Close()
	}
}
