// soloud_test.go : soloud test package
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package soloud

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	soloudlib "soloud"

	"github.com/suglasp/max-go/internal/settings"
	"github.com/suglasp/max-go/internal/testutil"
)

// testDLL locates the platform-specific SoLoud library used by smoke tests.
func testDLL(t *testing.T) string {
	t.Helper()

	path := filepath.Join("..", "..", "..", "dependencies", "soloud-bin", soloudlib.DefaultLibName())
	if _, err := os.Stat(path); err != nil {
		t.Skipf("SoLoud DLL not available: %v", err)
	}

	return path
}

func TestMissingDLLReturnsError(t *testing.T) {
	be, err := NewWithBackend(filepath.Join(t.TempDir(), "missing-soloud.dll"), "")

	if err == nil {
		be.Close()
		t.Skip("SoLoud was already loaded in this process")
	}
	if be != nil {
		t.Error("a failed load returned a backend")
	}
	if !strings.Contains(err.Error(), "missing-soloud.dll") {
		t.Errorf("error %q does not name the missing library", err)
	}
}

func TestNoSoundBackendSmoke(t *testing.T) {
	dir := t.TempDir()
	wavPath := filepath.Join(dir, "tone.wav")

	if err := os.WriteFile(wavPath, testutil.SilentWAV(), 0o644); err != nil {
		t.Fatal(err)
	}

	be, err := NewNoSound(testDLL(t))
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

// TestBackendIDKnowsEverySettingsBackend checks that every SoLoud backend the settings offer maps to its own SoLoud
// constant: only "auto" may resolve to SOLOUD_AUTO.
func TestBackendIDKnowsEverySettingsBackend(t *testing.T) {
	for _, name := range settings.SoLoudBackends {
		got := BackendID(name)
		if (got == soloudlib.SOLOUD_AUTO) != (name == "auto") {
			t.Errorf("BackendID(%q) = %d, the settings list and BackendID disagree", name, got)
		}
	}
}
