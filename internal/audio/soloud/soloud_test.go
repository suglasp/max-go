// soloud_test.go : soloud test package
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package soloud

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"strings"
	"testing"

	soloudlib "soloud"

	"github.com/suglasp/max-go/internal/settings"
)

// testDLL locates the platform-specific SoLoud library used by smoke tests.
func testDLL(t *testing.T) string {
	t.Helper()

	/* // note: unused code in Go port
	// the per platform name table; the binding's DefaultLibName gives the same names.
	// Linux is the default below.
	name := "libsoloud_x64.so"

	// Windows
	if runtime.GOOS == "windows" {
		name = "soloud_x64.dll"

		//if runtime.GOARCH == "386" {
		//	name = "soloud_x86.dll"
		//}
	} else if runtime.GOOS == "darwin" {
		// macOS
		name = "libsoloud_x64.dylib"
	}
	*/

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

	if err := os.WriteFile(wavPath, testWAV(), 0o644); err != nil {
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

	memHeld, err := be.PlaySfxHeldMem(testWAV(), 0.5, 0.25, true)
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

func testWAV() []byte {
	const (
		sampleRate    = 22050
		channels      = 1
		bitsPerSample = 16
		samples       = 22050
	)

	// Build a minimal WAVE/RIFF header.
	dataBytes := (samples * channels * bitsPerSample / 8)
	out := make([]byte, 44+dataBytes)
	copy(out[0:4], "RIFF")
	binary.LittleEndian.PutUint32(out[4:8], uint32(len(out)-8))
	copy(out[8:12], "WAVE")
	copy(out[12:16], "fmt ")
	binary.LittleEndian.PutUint32(out[16:20], 16)
	binary.LittleEndian.PutUint16(out[20:22], 1)
	binary.LittleEndian.PutUint16(out[22:24], channels)
	binary.LittleEndian.PutUint32(out[24:28], sampleRate)
	binary.LittleEndian.PutUint32(out[28:32], sampleRate*channels*bitsPerSample/8)
	binary.LittleEndian.PutUint16(out[32:34], channels*bitsPerSample/8)
	binary.LittleEndian.PutUint16(out[34:36], bitsPerSample)
	copy(out[36:40], "data")
	binary.LittleEndian.PutUint32(out[40:44], uint32(dataBytes))

	return out
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
