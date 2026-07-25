// reloadworld_test.go : tests loading a save that references another world map.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package gamescreen

import (
	"fmt"
	"testing"

	"github.com/suglasp/max-go/internal/drawmap"
	"github.com/suglasp/max-go/internal/world"
)

// flatWorld builds a small all land map for the reload tests.
func flatWorld(w, h int) *world.Map {
	m := &world.Map{Surface: make([]uint8, w*h)}
	m.W, m.H = w, h
	for i := range m.Surface {
		m.Surface[i] = world.SurfaceLand
	}
	return m
}

// TestReloadWorldSwapsToSaveMap: reloadWorld keeps the current world when the maps match
// or no reloader is wired, and swaps to the save's own map (updating the renderer and the
// tracked name) when they differ. This is the fix for a base saved on one map being
// restored onto the wrong terrain during an in game load.
func TestReloadWorldSwapsToSaveMap(t *testing.T) {
	current := flatWorld(16, 16)
	saved := flatWorld(20, 20) // distinct dimensions so we can tell them apart

	s := &Screen{wmap: current, mapName: "crater_4.wrl", r: drawmap.New(current)}

	// Same map: no reload, world unchanged.
	if got := s.reloadWorld("crater_4.wrl"); got != current {
		t.Error("a matching map should keep the current world")
	}

	// Different map but no reloader wired: keep the current world, do not touch the name.
	if got := s.reloadWorld("crater_1.wrl"); got != current {
		t.Error("with no reloader the current world must be kept")
	}
	if s.mapName != "crater_4.wrl" {
		t.Errorf("mapName changed on a no-op reload: %q", s.mapName)
	}

	// Different map with a reloader: swap to the save's world.
	s.hooks.worldReloader = func(name string) (*world.Map, error) {
		if name != "crater_1.wrl" {
			t.Fatalf("reloader asked for %q, want crater_1.wrl", name)
		}
		return saved, nil
	}

	got := s.reloadWorld("crater_1.wrl")
	if got != saved {
		t.Fatal("reloadWorld did not return the reloaded world")
	}
	if s.wmap != saved {
		t.Error("s.wmap was not swapped to the reloaded world")
	}
	if s.mapName != "crater_1.wrl" {
		t.Errorf("mapName = %q, want crater_1.wrl", s.mapName)
	}

	// A reloader error keeps the current world.
	s2 := &Screen{
		wmap:    current,
		mapName: "crater_4.wrl",
		r:       drawmap.New(current),
		hooks:   screenHooks{worldReloader: func(string) (*world.Map, error) { return nil, fmt.Errorf("boom") }},
	}
	if got := s2.reloadWorld("crater_1.wrl"); got != current {
		t.Error("a reload failure must keep the current world")
	}
	if s2.mapName != "crater_4.wrl" {
		t.Errorf("mapName changed after a failed reload: %q", s2.mapName)
	}
}
