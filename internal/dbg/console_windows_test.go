// console_windows_test.go : tests that the console rebind never closes a std handle it replaced.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package dbg

import (
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"golang.org/x/sys/windows"
)

// TestSwapStdioKeepsReplacedFiles stands a file handle in for a std handle freed by FreeConsole. After the swap
// and a few garbage collections that handle must still name its own file: a dropped stdout file would close it
// from its finalizer, and Windows would hand the value to the next object it opens.
func TestSwapStdioKeepsReplacedFiles(t *testing.T) {
	dir := t.TempDir()
	savedOut, savedErr := os.Stdout, os.Stderr
	keptBefore := len(replacedStdio)

	t.Cleanup(func() {
		os.Stdout, os.Stderr = savedOut, savedErr
		log.SetOutput(savedErr)

		// The test handle is the only one that may be closed, so the temp folder can be removed.
		if len(replacedStdio) > keptBefore {
			replacedStdio[keptBefore].Close()
			replacedStdio = replacedStdio[:keptBefore]
		}
	})

	source, err := os.Create(filepath.Join(dir, "stdout.txt"))
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	process := windows.CurrentProcess()
	var handle windows.Handle
	err = windows.DuplicateHandle(process, windows.Handle(source.Fd()), process, &handle, 0, false, windows.DUPLICATE_SAME_ACCESS)
	source.Close()
	if err != nil {
		t.Fatalf("DuplicateHandle: %v", err)
	}

	stand := os.NewFile(uintptr(handle), "/dev/stdout")
	os.Stdout, os.Stderr = stand, stand
	stand = nil

	out, err := os.Create(filepath.Join(dir, "console.txt"))
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	defer out.Close()

	swapStdio(out)

	for range 5 {
		runtime.GC()
		time.Sleep(10 * time.Millisecond)
	}

	path := make([]uint16, windows.MAX_PATH)
	n, err := windows.GetFinalPathNameByHandle(handle, &path[0], uint32(len(path)), 0)
	if err != nil {
		t.Fatalf("the replaced std handle was closed: %v", err)
	}

	if name := windows.UTF16ToString(path[:n]); !strings.HasSuffix(name, `\stdout.txt`) {
		t.Fatalf("the replaced std handle now names %q, it was closed and reused", name)
	}
}
