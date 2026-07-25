// resourcemap_save_test.go : resource map save/load tests for the saveload package.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package saveload

import (
	"bytes"
	"encoding/binary"
	"encoding/gob"
	"testing"

	"github.com/suglasp/max-go/internal/economy"
)

// TestSaveRestoreResourceMap requires resource deposits and each team's survey
// overlay to survive a save and load. Restored mining stations must retain their
// yield, and surveyed cells must stay visible.
func TestSaveRestoreResourceMap(t *testing.T) {
	g := newTestGame(t)
	g.Resources = economy.NewResourceMap(16, 16)
	g.Resources.SetDeposit(5, 6, economy.CargoRaw, 14)
	g.Resources.SetDeposit(6, 6, economy.CargoFuel, 9)
	g.Resources.SetDeposit(7, 7, economy.CargoGold, 5)
	g.Resources.MarkSurveyed(5, 6, 0)
	g.Resources.MarkSurveyed(7, 7, 1)

	var buf bytes.Buffer
	if err := Save(&buf, Capture(g, "GREEN_1.WRL")); err != nil {
		t.Fatalf("save: %v", err)
	}
	snap, err := Load(&buf)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	m := landMap(16, 16)
	g2 := snap.Restore(m)
	if g2.Resources == nil {
		t.Fatal("resource map not restored (nil); post-load mining sites would be lost")
	}

	if ty, amt := g2.Resources.DepositAt(5, 6); ty != economy.CargoRaw || amt != 14 {
		t.Errorf("raw deposit (5,6) = %v/%d, want CargoRaw/14", ty, amt)
	}
	if ty, amt := g2.Resources.DepositAt(6, 6); ty != economy.CargoFuel || amt != 9 {
		t.Errorf("fuel deposit (6,6) = %v/%d, want CargoFuel/9", ty, amt)
	}
	if ty, amt := g2.Resources.DepositAt(7, 7); ty != economy.CargoGold || amt != 5 {
		t.Errorf("gold deposit (7,7) = %v/%d, want CargoGold/5", ty, amt)
	}
	if ty, _ := g2.Resources.DepositAt(0, 0); ty != economy.CargoNone {
		t.Errorf("empty cell (0,0) restored as %v, want CargoNone", ty)
	}

	if !g2.Resources.IsSurveyed(5, 6, 0) || g2.Resources.IsSurveyed(5, 6, 1) {
		t.Error("team-0 survey bit at (5,6) not round-tripped (or leaked to team 1)")
	}
	if !g2.Resources.IsSurveyed(7, 7, 1) {
		t.Error("team-1 survey bit at (7,7) not restored")
	}
	if g2.Resources.Surf != m {
		t.Error("restored resource map Surf not wired to the live map (isAir/Survey would misbehave)")
	}
}

// TestSaveRestoreNoResourceMap checks that a pre-landing game retains a nil resource map, matching pre-v9 saves.
func TestSaveRestoreNoResourceMap(t *testing.T) {
	g := newTestGame(t) // Resources left nil
	var buf bytes.Buffer
	if err := Save(&buf, Capture(g, "GREEN_1.WRL")); err != nil {
		t.Fatalf("save: %v", err)
	}
	snap, err := Load(&buf)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if g2 := snap.Restore(landMap(16, 16)); g2.Resources != nil {
		t.Error("a game with no resource map should restore to nil Resources")
	}
}

// writeSave hand writes a MAXGOSV stream with an arbitrary header version, to
// exercise Load's version gate independent of the current Version.
func writeSave(t *testing.T, ver uint32, s *Snapshot) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	buf.Write(magic[:])
	if err := binary.Write(&buf, binary.LittleEndian, ver); err != nil {
		t.Fatalf("write version: %v", err)
	}
	if err := gob.NewEncoder(&buf).Encode(s); err != nil {
		t.Fatalf("encode: %v", err)
	}
	return &buf
}

// TestLoadAcceptsOlderVersion checks that additive fields default correctly when an older schema version is loaded.
func TestLoadAcceptsOlderVersion(t *testing.T) {
	snap := Capture(newTestGame(t), "GREEN_1.WRL")
	snap.Version = Version - 1
	buf := writeSave(t, Version-1, snap)
	if _, err := Load(buf); err != nil {
		t.Fatalf("older-version save should load, got: %v", err)
	}
}

// TestLoadRejectsNewerVersion checks that Load refuses an unknown future schema.
func TestLoadRejectsNewerVersion(t *testing.T) {
	buf := writeSave(t, Version+1, Capture(newTestGame(t), "GREEN_1.WRL"))
	if _, err := Load(buf); err == nil {
		t.Fatal("a newer-version save must be rejected")
	}
}
