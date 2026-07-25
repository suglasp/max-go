// dbg_test.go : tests console gating, buffering, and log formatting.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package dbg

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDisabledIsNoop(t *testing.T) {
	if err := Init(false, "", 0); err != nil {
		t.Fatal(err)
	}
	defer Close()
	if Enabled() {
		t.Error("Enabled() = true with no outputs")
	}
	Printf("goes nowhere %d", 1) // must not panic
}

func TestLogFileBatchesUntilThreshold(t *testing.T) {
	path := filepath.Join(t.TempDir(), "max-go.log")
	if err := Init(true, path, 3); err != nil { // console on so the file is written
		t.Fatal(err)
	}
	defer Close()
	if !Enabled() {
		t.Fatal("Enabled() = false with console+log configured")
	}

	Printf("line one")
	Printf("line two")
	// Below the 3-line batch: the file has not been written yet.
	if data, _ := os.ReadFile(path); len(data) != 0 {
		t.Errorf("log flushed too early: %q", data)
	}

	Printf("line three")
	// Batch reached: all three lines written at once, in "<date>@<time> : msg".
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"line one", "line two", "line three", "@", " : "} {
		if !strings.Contains(string(data), want) {
			t.Errorf("log missing %q after flush:\n%s", want, data)
		}
	}

	Printf("line four")
	Flush() // manual flush of a partial batch
	if data, _ := os.ReadFile(path); !strings.Contains(string(data), "line four") {
		t.Error("Flush did not write pending lines")
	}
}

func TestConsoleGatesLogFile(t *testing.T) {
	// console off is the master off switch: no log file even when a path is set.
	path := filepath.Join(t.TempDir(), "max-go.log")
	if err := Init(false, path, 1); err != nil {
		t.Fatal(err)
	}
	defer Close()
	if Enabled() {
		t.Error("Enabled() = true with console off")
	}
	Printf("should not be written")
	Flush()
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("log file created despite console off: %v", err)
	}
}

func TestCloseFlushesRemainder(t *testing.T) {
	path := filepath.Join(t.TempDir(), "max-go.log")
	if err := Init(true, path, 1000); err != nil {
		t.Fatal(err)
	}
	Printf("buffered until close")
	Close()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "buffered until close") {
		t.Errorf("Close lost buffered lines: %q", data)
	}
	if Enabled() {
		t.Error("Enabled() = true after Close")
	}
}

func TestLogFormatVariants(t *testing.T) {
	path := filepath.Join(t.TempDir(), "max-go.log")
	if err := Init(true, path, 1); err != nil { // batch 1 -> each line flushes immediately
		t.Fatal(err)
	}
	defer Close()

	Logf(SFX, "looping %s", "GBOAT")     // module tag, no team
	Log("TANK", "GRAY", "fires (%d)", 8) // module + team
	Printf("untagged %d", 1)             // compatibility alias, no tags
	Log("", "", "")                      // line with only the timestamp

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimRight(string(data), "\n"), "\n")
	if len(lines) != 4 {
		t.Fatalf("want 4 lines, got %d: %q", len(lines), data)
	}
	// Every line opens with the [DD-MM-YY@HH:mm:ss] timestamp bracket.
	for i, ln := range lines {
		if !strings.HasPrefix(ln, "[") || !strings.Contains(ln, "@") {
			t.Errorf("line %d missing the timestamp bracket: %q", i, ln)
		}
	}
	// Logf: module bracket, no team, then the message.
	if !strings.Contains(lines[0], "[SFX] : looping GBOAT") {
		t.Errorf("Logf line wrong: %q", lines[0])
	}
	// Log: module then team bracket (in order), then the message.
	if !strings.Contains(lines[1], "[TANK] [GRAY] : fires (8)") {
		t.Errorf("Log module+team line wrong: %q", lines[1])
	}
	// Printf: no tag brackets, just "] : <msg>".
	if !strings.Contains(lines[2], "] : untagged 1") || strings.Contains(lines[2], "[SFX]") {
		t.Errorf("Printf untagged line wrong: %q", lines[2])
	}
	// All empty Log: a line with only the timestamp, no " : " message tail.
	if strings.Contains(lines[3], " : ") {
		t.Errorf("timestamp-only line should have no message tail: %q", lines[3])
	}
}

func TestLogFileOverwrittenPerStart(t *testing.T) {
	path := filepath.Join(t.TempDir(), "max-go.log")

	if err := Init(true, path, 1); err != nil {
		t.Fatal(err)
	}
	Printf("from previous run")
	Close()

	// Second game start truncates the previous log.
	if err := Init(true, path, 1); err != nil {
		t.Fatal(err)
	}
	Printf("fresh start")
	Close()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "from previous run") {
		t.Error("old log content survived a new start")
	}
	if !strings.Contains(string(data), "fresh start") {
		t.Errorf("new log content missing: %q", data)
	}
}
