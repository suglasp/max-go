// crashlog_test.go : testing unit for crashlog.go.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// crashChildEnv makes the test binary act as a crashing game run.
const crashChildEnv = "MAX_GO_CRASH_CHILD_DIR"

// TestCrashLogChild is the crashing run itself; it only acts when started by
// TestCrashLogKeepsPanicTrace.
func TestCrashLogChild(t *testing.T) {
	dir := os.Getenv(crashChildEnv)
	if dir == "" {
		t.Skip("only runs as the child of TestCrashLogKeepsPanicTrace")
	}

	openCrashLog(dir)
	panic("crash log probe")
}

// TestCrashLogKeepsPanicTrace runs a child that panics with the crash log open: the
// report next to the binary must hold the panic and its stack.
func TestCrashLogKeepsPanicTrace(t *testing.T) {
	dir := t.TempDir()

	cmd := exec.Command(os.Args[0], "-test.run=^TestCrashLogChild$")
	cmd.Env = append(os.Environ(), crashChildEnv+"="+dir)
	if err := cmd.Run(); err == nil {
		t.Fatal("the crashing child exited cleanly")
	}

	data, err := os.ReadFile(filepath.Join(dir, crashLogName))
	if err != nil {
		t.Fatalf("no crash report: %v", err)
	}

	report := string(data)
	if !strings.Contains(report, "crash log probe") || !strings.Contains(report, "goroutine") {
		t.Fatalf("crash report lacks the panic trace:\n%s", report)
	}
}

// TestCrashLogRemovedAfterCleanExit checks a run that ends normally leaves no report.
func TestCrashLogRemovedAfterCleanExit(t *testing.T) {
	dir := t.TempDir()

	openCrashLog(dir)
	if _, err := os.Stat(filepath.Join(dir, crashLogName)); err != nil {
		t.Fatalf("crash report not created: %v", err)
	}

	closeCrashLog()
	if _, err := os.Stat(filepath.Join(dir, crashLogName)); !os.IsNotExist(err) {
		t.Fatalf("an empty crash report stayed behind: %v", err)
	}
}
