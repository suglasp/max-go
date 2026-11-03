// crashlog.go : keeps the reason for a crash on disk when the game runs without a console.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime/debug"

	"github.com/suglasp/max-go/internal/dbg"
)

// crashLogName is the crash report written next to the binary.
const crashLogName = "max-go-crash.log"

// crashFile receives the Go runtime's panic trace and the fatalf reasons; nil
// until openCrashLog runs.
var crashFile *os.File

// openCrashLog creates max-go-crash.log next to the binary and routes the Go
// runtime's fatal panic output into it, so a crash with the console hidden still
// leaves its trace on disk. closeCrashLog removes the report again after a clean exit.
func openCrashLog(baseDir string) {
	f, err := os.Create(filepath.Join(baseDir, crashLogName))
	if err != nil {
		dbg.Printf("crash log unavailable: %v", err)
		return
	}

	if err := debug.SetCrashOutput(f, debug.CrashOptions{}); err != nil {
		dbg.Printf("crash log: %v", err)
	}

	crashFile = f
}

// closeCrashLog stops the crash routing and deletes the report when nothing was
// written to it, so only a run that died leaves a max-go-crash.log behind.
func closeCrashLog() {
	if crashFile == nil {
		return
	}

	_ = debug.SetCrashOutput(nil, debug.CrashOptions{})

	info, err := crashFile.Stat()
	path := crashFile.Name()
	crashFile.Close()
	crashFile = nil

	if err == nil && info.Size() == 0 {
		os.Remove(path)
	}
}

// fatalf ends the run on an error: the reason goes to the debug log (flushed, so
// the lines before it survive too) and to the crash report, then to stderr, and
// the process exits with status 1.
func fatalf(format string, args ...any) {
	msg := fmt.Sprintf(format, args...)

	dbg.Printf("%s", msg)
	dbg.Close()

	if crashFile != nil {
		fmt.Fprintln(crashFile, msg)
		crashFile.Close()
	}

	log.Fatal(msg)
}
