// miniaudio_test.go : miniaudio backend test package
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package miniaudio

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	ma "miniaudio"

	"github.com/suglasp/max-go/internal/audio"
	"github.com/suglasp/max-go/internal/testutil"
)

// testDLL locates the platform-specific miniaudio library used by smoke tests.
func testDLL(t *testing.T) string {
	t.Helper()

	path := filepath.Join("..", "..", "..", "dependencies", "miniaudio-bin", ma.DefaultLibName())
	if _, err := os.Stat(path); err != nil {
		t.Skipf("miniaudio library not available: %v", err)
	}

	return path
}

func TestMissingDLLReturnsError(t *testing.T) {
	be, err := New(filepath.Join(t.TempDir(), "missing-miniaudio.dll"))

	if err == nil {
		be.Close()
		t.Skip("miniaudio was already loaded in this process")
	}
	if be != nil {
		t.Error("a failed load returned a backend")
	}
	if !strings.Contains(err.Error(), "missing-miniaudio.dll") {
		t.Errorf("error %q does not name the missing library", err)
	}
}

func TestNullEngineBackendSmoke(t *testing.T) {
	dir := t.TempDir()
	wavPath := filepath.Join(dir, "tone.wav")

	if err := os.WriteFile(wavPath, testutil.SilentWAV(), 0o644); err != nil {
		t.Fatal(err)
	}

	be, err := NewNull(testDLL(t))
	if err != nil {
		t.Fatal(err)
	}
	defer be.Close()

	if err := be.PlaySfx(wavPath, 0.5, -0.25); err != nil {
		t.Fatal(err)
	}

	held, err := be.PlaySfxHeld(wavPath, 0.5, -0.25, true)
	if err != nil {
		t.Fatal(err)
	}
	held.SetPan(0.25)
	held.Stop()

	memHeld, err := be.PlaySfxHeldMem(testutil.SilentWAV(), 0.5, 0.25, true)
	if err != nil {
		t.Fatal(err)
	}
	memHeld.Stop()

	if err := be.PlayVoice(wavPath, 0.5); err != nil {
		t.Fatal(err)
	}

	if !be.VoicePlaying() {
		t.Fatal("VoicePlaying = false after PlayVoice")
	}

	be.SetMusicVolume(0.25)
	if err := be.PlayMusic(wavPath, true, 0.25); err != nil {
		t.Fatal(err)
	}

	be.StopMusic()
	be.StopAll()
}

func TestNullEngineMoviePCMSmoke(t *testing.T) {
	be, err := NewNull(testDLL(t))
	if err != nil {
		t.Fatal(err)
	}
	defer be.Close()

	var _ audio.MoviePCMBackend = be

	if err := be.StartPCM(audio.PCMFormat{Bits: 16, Channels: 2, Rate: 22050}, 0.5); err != nil {
		t.Fatal(err)
	}

	samples := make([]int16, 512)
	if err := be.QueuePCM(samples); err != nil {
		t.Fatal(err)
	}

	be.StopPCM()
}
