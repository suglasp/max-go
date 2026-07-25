// timer.go : the M.A.X. platform timer - millisecond resolution tick counter for animation and input timing.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
//
// C++ ported : timer.c - GNW timer API
package platform

import "time"

// Timer helpers follow timer.c's millisecond semantics.

var timerStart = time.Now()

// Ticks returns milliseconds since program start.
//
// C++: timer_get (timer.c)
func Ticks() uint64 {
	return uint64(time.Since(timerStart) / time.Millisecond)
}

// Elapsed returns milliseconds passed since the given Ticks stamp, or
// 0 if the stamp lies in the future.
//
// C++: timer_elapsed_time (timer.c)
func Elapsed(stamp uint64) uint64 {
	now := Ticks()
	if now > stamp {
		return now - stamp
	}
	return 0
}

/* // note: ported but unused code in Go port
// Wait sleeps for the given number of milliseconds.
//
// C++: timer_wait (timer.c)
func Wait(ms uint32) {
	time.Sleep(time.Duration(ms) * time.Millisecond)
}
*/
