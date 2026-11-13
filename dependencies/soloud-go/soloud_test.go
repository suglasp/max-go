// soloud_test.go : tests the SoLoud binding.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package soloud

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// testLibPath is the library the smoke test loads: SOLOUD_DLL if set, otherwise the build the
// repo ships in ../soloud-bin under its DefaultLibName.
func testLibPath() string {
	if path := os.Getenv("SOLOUD_DLL"); path != "" {
		return path
	}
	name := DefaultLibName()

	return filepath.Join("..", "soloud-bin", name)
}

// TestSmoke exercises the binding against the shipped library with the silent NOSOUND backend.
// It covers integer, float32, float64, bool, string, and []byte argument and return paths.
func TestSmoke(t *testing.T) {
	dll := testLibPath()
	if err := LoadPath(dll); err != nil {
		t.Fatalf("LoadPath(%q): %v", dll, err)
	}

	engine := NewSoloud()
	defer engine.Destroy()
	if err := engine.InitEx(SOLOUD_CLIP_ROUNDOFF, SOLOUD_NOSOUND, SOLOUD_AUTO, SOLOUD_AUTO, 2); err != nil {
		t.Fatalf("InitEx: %v", err)
	}
	defer engine.Deinit()

	if v := engine.GetVersion(); v != 202002 {
		t.Errorf("GetVersion() = %d, want 202002", v)
	}
	if b := engine.GetBackendString(); b == "" {
		t.Error("GetBackendString() is empty")
	} else {
		t.Logf("backend: %s, samplerate: %d", b, engine.GetBackendSamplerate())
	}

	// float32 round trip through the C API.
	engine.SetGlobalVolume(0.5)
	if v := engine.GetGlobalVolume(); v < 0.49 || v > 0.51 {
		t.Errorf("GetGlobalVolume() = %v, want 0.5", v)
	}
	engine.SetGlobalVolume(1)

	// string + play + voice handle queries.
	speech := NewSpeech()
	defer speech.Destroy()
	if err := speech.SetText("smoke test"); err != nil {
		t.Fatalf("SetText: %v", err)
	}
	h := engine.Play(speech)
	if !engine.IsValidVoiceHandle(h) {
		t.Fatalf("Play returned invalid voice handle %d", h)
	}
	if n := engine.GetActiveVoiceCount(); n != 1 {
		t.Errorf("GetActiveVoiceCount() = %d, want 1", n)
	}

	// float64 round trip.
	engine.SetLoopPoint(h, 1.5)
	if lp := engine.GetLoopPoint(h); lp != 1.5 {
		t.Errorf("GetLoopPoint() = %v, want 1.5", lp)
	}
	engine.StopAll()

	// []byte path + error code mapping: garbage data must fail to load.
	wav := NewWav()
	defer wav.Destroy()
	err := wav.LoadMem([]byte{1, 2, 3, 4, 5, 6, 7, 8})
	var serr SoloudError
	if !errors.As(err, &serr) {
		t.Errorf("LoadMem(garbage) = %v, want a SoloudError", err)
	}

	// Error string lookup through the engine.
	if msg := engine.GetErrorString(FILE_NOT_FOUND); msg == "" {
		t.Error("GetErrorString(FILE_NOT_FOUND) is empty")
	}

	// Filter creation, parameter introspection and float returns.
	echo := NewEchoFilter()
	defer echo.Destroy()
	if err := echo.SetParamsEx(0.3, 0.7, 0.1); err != nil {
		t.Errorf("EchoFilter.SetParamsEx: %v", err)
	}
	if n := echo.GetParamCount(); n != 4 {
		t.Errorf("EchoFilter.GetParamCount() = %d, want 4", n)
	}
	if name := echo.GetParamName(ECHOFILTER_DELAY); name == "" {
		t.Error("EchoFilter.GetParamName(DELAY) is empty")
	}
}
