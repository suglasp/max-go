//go:build !windows

// console_other.go : keeps terminal output unchanged on non-Windows systems.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package dbg

// ShowConsole does nothing outside Windows: output goes to the terminal
// the game was started from.
func ShowConsole() {}
