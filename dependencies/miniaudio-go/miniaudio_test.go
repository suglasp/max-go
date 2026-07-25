// miniaudio_test.go : tests the miniaudio binding.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package miniaudio

import (
	"os"
	"testing"
)

// loadForTest loads the wrapper library and skips the test when it is not built.
// Set MINIAUDIO_DLL to override the default bin/ location.
func loadForTest(t *testing.T) {
	t.Helper()
	path := os.Getenv("MINIAUDIO_DLL")
	if path == "" {
		path = "bin/" + defaultLibName()
	}
	if _, err := os.Stat(path); err != nil {
		t.Skipf("wrapper library not built (%s); run capi/build_windows.sh or build_linux.sh", path)
	}
	if err := LoadPath(path); err != nil {
		t.Fatalf("LoadPath(%q): %v", path, err)
	}
}

func TestVersion(t *testing.T) {
	loadForTest(t)
	if v := Version(); v == "" {
		t.Fatal("Version() returned empty string")
	} else {
		t.Logf("miniaudio version %s", v)
	}
}

func TestNullEngine(t *testing.T) {
	loadForTest(t)
	eng, err := NewNullEngine()
	if err != nil {
		t.Fatalf("NewNullEngine: %v", err)
	}
	defer eng.Destroy()

	if got := eng.SampleRate(); got != 48000 {
		t.Errorf("SampleRate = %d, want 48000", got)
	}
	if got := eng.Channels(); got != 2 {
		t.Errorf("Channels = %d, want 2", got)
	}
}

func TestStreamWrite(t *testing.T) {
	loadForTest(t)
	// Opening a Stream needs a real playback device. Headless hosts have none.
	const capacity = 4096
	s, err := NewStream(FormatF32, 2, 48000, capacity)
	if err != nil {
		t.Skipf("no playback device available: %v", err)
	}
	defer s.Destroy()

	before := s.AvailableWrite()
	if before == 0 {
		t.Fatal("AvailableWrite returned 0 on a fresh stream")
	}

	frames := uint32(512)
	buf := make([]float32, frames*2) // 2 channels, silence
	n := s.WriteFloat32(buf)
	if n != frames {
		t.Errorf("WriteFloat32 queued %d frames, want %d", n, frames)
	}
	if after := s.AvailableWrite(); after >= before {
		t.Errorf("AvailableWrite did not decrease after write: before=%d after=%d", before, after)
	}
}
