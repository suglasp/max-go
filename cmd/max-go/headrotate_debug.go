// headrotate_debug.go : launches the turret rotation bench through the game screen helpers.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package main

import "github.com/suglasp/max-go/internal/gamescreen"

// runHeadrotateDebug prepares the turret/head rotation bench. The bench itself
// lives in gamescreen so it can use the same deploy helpers as the other
// in world debug layouts.
func runHeadrotateDebug(scr *gamescreen.Screen) {
	scr.DeployHeadRotateBench()
}
