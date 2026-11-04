//go:build windows

// console_windows.go : attaches or allocates a Windows console for debug output.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package dbg

import (
	"log"
	"os"

	"golang.org/x/sys/windows"
)

var (
	kernel32         = windows.NewLazySystemDLL("kernel32.dll")
	user32           = windows.NewLazySystemDLL("user32.dll")
	getConsoleWindow = kernel32.NewProc("GetConsoleWindow")
	allocConsole     = kernel32.NewProc("AllocConsole")
	showWindow       = user32.NewProc("ShowWindow")
)

const swShow = 5 // SW_SHOW

// QuickEdit Mode lets a click-drag in the console window select text, which
// pauses the WHOLE console (any WriteConsole call blocks) until Enter or a
// right click releases it. dbg.emit writes every log line to this console
// synchronously from the game loop's own goroutine, so a stray click here
// freezes the entire game - this was caught live via runStallWatchdog's
// goroutine dump (a multi second stall inside dbg.emit -> WriteConsole).
const (
	enableQuickEditMode = 0x0040
	enableExtendedFlags = 0x0080
)

// ShowConsole makes debugging output visible on Windows. Ebitengine
// hides the console window when the game owns it (double click
// launch); this shows it again. If the process has no console at all
// (for example, an -H=windowsgui build), one is allocated and
// stdout/stderr are rebound to it. Started from a terminal, output is
// already visible and the existing console window is left alone.
//
// Call it after ebiten.RunGame has begun (e.g. on the first Update),
// otherwise Ebitengine's own console hiding runs afterwards and undoes
// it.
func ShowConsole() {
	hwnd, _, _ := getConsoleWindow.Call()
	if hwnd == 0 {
		if r, _, err := allocConsole.Call(); r == 0 {
			log.Printf("dbg: AllocConsole failed: %v", err)
			return
		}
		rebindStdio()
		hwnd, _, _ = getConsoleWindow.Call()
		if hwnd == 0 {
			return
		}
	}
	showWindow.Call(hwnd, swShow)
	disableQuickEdit()
}

// disableQuickEdit turns off the console's QuickEdit Mode so a click into
// the window can never again pause WriteConsole and freeze the game loop.
func disableQuickEdit() {
	in, err := windows.GetStdHandle(windows.STD_INPUT_HANDLE)
	if err != nil {
		return
	}
	var mode uint32
	if err := windows.GetConsoleMode(in, &mode); err != nil {
		return
	}
	mode = (mode &^ enableQuickEditMode) | enableExtendedFlags
	_ = windows.SetConsoleMode(in, mode)
}

// rebindStdio points os.Stdout/os.Stderr and the log package at the
// freshly allocated console.
func rebindStdio() {
	out, err := os.OpenFile("CONOUT$", os.O_WRONLY, 0)
	if err != nil {
		return
	}
	swapStdio(out)
}

// replacedStdio holds the std files that swapStdio replaced, for the whole run. FreeConsole already closed their
// handles and Windows reuses those values, so a collected file would close a handle that is no longer its own
// (a runtime semaphore makes the game die at start). These files are never closed.
var replacedStdio []*os.File

// swapStdio points os.Stdout, os.Stderr and the log package at out, keeping the replaced files referenced.
func swapStdio(out *os.File) {
	replacedStdio = append(replacedStdio, os.Stdout, os.Stderr)
	os.Stdout = out
	os.Stderr = out
	log.SetOutput(out)
}
