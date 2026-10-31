// unitinfo_test.go : tests unit positioning, facing, and animation state.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package game

import "testing"

func TestSetPositionCentersInCell(t *testing.T) {
	u := NewUnitInfo(0, 0, 1, 0)
	if u.GridX != -1 || u.GridY != -1 {
		t.Fatalf("fresh unit grid = %d,%d, want -1,-1", u.GridX, u.GridY)
	}
	u.SetPosition(3, 5)
	if u.X != 3*64+32 || u.Y != 5*64+32 {
		t.Errorf("pixel center = %d,%d, want %d,%d", u.X, u.Y, 3*64+32, 5*64+32)
	}
	if u.GridX != 3 || u.GridY != 5 {
		t.Errorf("grid = %d,%d, want 3,5", u.GridX, u.GridY)
	}
}

// TestTurretStartsAlignedWithBody checks that a new unit's turret matches its
// body. SetAngle keeps both aligned until aiming rotates the turret separately.
func TestTurretStartsAlignedWithBody(t *testing.T) {
	u := NewUnitInfo(0, 0, 1, 3) // spawned facing SE
	u.Flags |= FlagCanHeadRotate
	if u.TurretAngle != 3 {
		t.Errorf("spawn turret angle = %d, want 3 (aligned with the body)", u.TurretAngle)
	}
	u.SetAngle(5) // drive toward SW: the turret follows the body
	if u.Angle != 5 || u.TurretAngle != 5 {
		t.Errorf("after drive body/turret = %d/%d, want 5/5", u.Angle, u.TurretAngle)
	}
}

// TestStartBuildPose checks that a 2x2 constructor moves from its corner to the
// footprint center. A 1x1 engineer stays centered on its cell, and EndBuildPose
// clears the build state.
func TestStartBuildPose(t *testing.T) {
	con := &UnitInfo{Flags: FlagConstructorUnit}
	con.SetPosition(5, 7) // corner cell of the 2x2 site
	con.StartBuildPose(5, 7, 2)
	if !con.Building {
		t.Error("constructor: Building not set")
	}
	if con.BuildToX != 5*64+64 || con.BuildToY != 7*64+64 {
		t.Errorf("constructor target = %d,%d, want %d,%d", con.BuildToX, con.BuildToY, 5*64+64, 7*64+64)
	}
	if con.X != 5*64+32 { // starts at the corner cell centre, not snapped to the site centre
		t.Errorf("constructor start X = %d, want corner-cell centre %d", con.X, 5*64+32)
	}
	for i := 0; i < 16 && !con.AdvanceBuildSlide(8); i++ {
	}
	if con.X != con.BuildToX || con.Y != con.BuildToY {
		t.Errorf("after slide = %d,%d, want %d,%d", con.X, con.Y, con.BuildToX, con.BuildToY)
	}

	eng := &UnitInfo{Flags: FlagConstructorUnit}
	eng.SetPosition(3, 4)
	eng.StartBuildPose(3, 4, 1) // 1x1 on its own cell -> target is the current centre
	if eng.BuildToX != eng.X || eng.BuildToY != eng.Y {
		t.Errorf("engineer should not slide: target %d,%d vs pos %d,%d", eng.BuildToX, eng.BuildToY, eng.X, eng.Y)
	}
	eng.EndBuildPose()
	if eng.Building {
		t.Error("EndBuildPose did not clear Building")
	}
}

func TestBodyFrameFollowsAngle(t *testing.T) {
	u := NewUnitInfo(0, 0, 1, 0)
	u.ImageBase = 16
	for dir := uint8(0); dir < AngleCount; dir++ {
		u.SetAngle(dir)
		if u.Angle != dir {
			t.Errorf("angle = %d, want %d", u.Angle, dir)
		}
		if got := u.BodyFrameIndex(); got != 16+int(dir) {
			t.Errorf("body frame for dir %d = %d, want %d", dir, got, 16+int(dir))
		}
	}
	// Direction wraps to the low three bits.
	u.SetAngle(9)
	if u.Angle != 1 {
		t.Errorf("angle 9 wrapped to %d, want 1", u.Angle)
	}
}

func TestTurretRotatesWithBodyOnlyWhenFlagged(t *testing.T) {
	// No head rotate flag: turret angle stays put as the body turns.
	plain := NewUnitInfo(0, 0, 1, 0)
	plain.SetAngle(3)
	if plain.TurretAngle != 0 {
		t.Errorf("turret turned without CAN_HEAD_ROTATE: %d", plain.TurretAngle)
	}

	// With the head rotate flag the turret rotates by the same delta as the body.
	turreted := NewUnitInfo(0, 0, 2, 2) // facing E, turret aligned at 2
	turreted.Flags = FlagCanHeadRotate
	turreted.TurretAngle = 2
	turreted.TurretImageBase = 40
	turreted.SetAngle(4) // body turns +2 (E -> S)
	if turreted.TurretAngle != 4 {
		t.Errorf("turret angle = %d, want 4 (rotated by +2)", turreted.TurretAngle)
	}
	if got := turreted.TurretFrameIndex(); got != 44 {
		t.Errorf("turret frame = %d, want 44", got)
	}
}

func TestTargetAngle(t *testing.T) {
	// The eight unit offsets resolve to the matching facing (0=N clockwise).
	for _, c := range []struct {
		dx, dy int
		want   uint8
	}{
		{0, -1, 0}, {1, -1, 1}, {1, 0, 2}, {1, 1, 3},
		{0, 1, 4}, {-1, 1, 5}, {-1, 0, 6}, {-1, -1, 7},
		{0, 0, 0}, // no offset -> north
	} {
		if got := TargetAngle(c.dx, c.dy); got != c.want {
			t.Errorf("TargetAngle(%d,%d) = %d, want %d", c.dx, c.dy, got, c.want)
		}
	}
	// A shallow offset (>2:1) snaps to the nearer cardinal, not the diagonal.
	if got := TargetAngle(4, 1); got != 2 { // mostly east
		t.Errorf("TargetAngle(4,1) = %d, want 2 (E)", got)
	}
	if got := TargetAngle(1, 4); got != 4 { // mostly south
		t.Errorf("TargetAngle(1,4) = %d, want 4 (S)", got)
	}
}

func TestFiringAngle(t *testing.T) {
	// Sixteen headings, 0=N clockwise; an axis needs a 4:1 ratio (klei 23/08/26), so 4:1 itself takes the in between frame.
	for _, c := range []struct {
		dx, dy int
		want   int
	}{
		{0, -5, 0}, {5, 0, 4}, {0, 5, 8}, {-5, 0, 12}, {0, 0, 0},
		{3, -3, 2}, {3, 3, 6}, {-3, 3, 10}, {-3, -3, 14},
		{1, -4, 1}, {4, -1, 3}, {4, 1, 5}, {1, 4, 7}, // the 2:1 rule gave 0, 4, 4, 8
		{-1, 4, 9}, {-4, 1, 11}, {-4, -1, 13}, {-1, -4, 15}, // the 2:1 rule gave 8, 12, 12, 0
		{12, -2, 4}, {2, 12, 8}, {-12, 2, 12}, {-2, -12, 0}, // past 4:1 snaps to the axis
	} {
		if got := FiringAngle(c.dx, c.dy); got != c.want {
			t.Errorf("FiringAngle(%d,%d) = %d, want %d", c.dx, c.dy, got, c.want)
		}
	}
}

func TestAimTurret(t *testing.T) {
	// A turreted unit rotates only its turret; the body keeps its facing.
	turreted := NewUnitInfo(0, 0, 1, 0)
	turreted.Flags = FlagCanHeadRotate
	turreted.TurretImageBase = 40
	turreted.AimTurret(3)
	if turreted.TurretAngle != 3 || turreted.TurretFrameIndex() != 43 {
		t.Errorf("turret angle/frame = %d/%d, want 3/43", turreted.TurretAngle, turreted.TurretFrameIndex())
	}
	if turreted.Angle != 0 {
		t.Errorf("body turned while aiming turret: %d", turreted.Angle)
	}
	// A unit without a turret aims by turning its whole body.
	plain := NewUnitInfo(0, 0, 1, 0)
	plain.AimTurret(5)
	if plain.Angle != 5 {
		t.Errorf("body angle = %d, want 5", plain.Angle)
	}
	// A unit may have turret/firing art but no independent head rotation; it also
	// aims by turning its body.
	bodyTurner := NewUnitInfo(0, 0, 1, 0)
	bodyTurner.Flags = FlagTurretSprite
	bodyTurner.AimTurret(4)
	if bodyTurner.Angle != 4 || bodyTurner.TurretAngle != 0 {
		t.Errorf("turret-sprite-only aim body/turret = %d/%d, want 4/0", bodyTurner.Angle, bodyTurner.TurretAngle)
	}
}

func TestStepTurret(t *testing.T) {
	u := NewUnitInfo(0, 0, 1, 0)
	u.Flags = FlagCanHeadRotate
	u.TurretImageBase = 40
	// Aim sets the target without snapping.
	u.AimTurretAt(3)
	if u.TurretTarget != 3 || u.TurretAngle != 0 {
		t.Fatalf("after AimTurretAt(3): angle=%d target=%d, want 0/3", u.TurretAngle, u.TurretTarget)
	}
	// Rotates the short way (0->1->2->3) one step per call, then stops.
	for want := uint8(1); want <= 3; want++ {
		if !u.StepTurret() || u.TurretAngle != want {
			t.Fatalf("step toward 3: angle=%d, want %d", u.TurretAngle, want)
		}
	}
	if u.StepTurret() {
		t.Error("StepTurret moved after reaching target")
	}
	if u.TurretFrameIndex() != 43 {
		t.Errorf("turret frame = %d, want 43", u.TurretFrameIndex())
	}
	// Target 0 from 3 goes the short way backward (3->2->1->0).
	u.AimTurretAt(0)
	for _, want := range []uint8{2, 1, 0} {
		if !u.StepTurret() || u.TurretAngle != want {
			t.Fatalf("step toward 0: angle=%d, want %d", u.TurretAngle, want)
		}
	}
	// A unit without a turret aims by turning its body, never animating the turret.
	plain := NewUnitInfo(0, 0, 1, 0)
	plain.AimTurretAt(5)
	if plain.Angle != 5 || plain.StepTurret() {
		t.Errorf("plain unit: angle=%d stepped=%v, want 5/false", plain.Angle, plain.StepTurret())
	}
}

func TestAdvanceSpin(t *testing.T) {
	// A spinning dish cycles 0..count-1 and wraps back to 0. The caller gates
	// which units spin, so AdvanceSpin steps unconditionally for count > 0.
	u := NewUnitInfo(0, 0, 1, 0)
	for want := uint8(1); want <= 3; want++ {
		u.AdvanceSpin(4)
		if u.SpinFrame != want {
			t.Fatalf("spin frame = %d, want %d", u.SpinFrame, want)
		}
	}
	u.AdvanceSpin(4) // 3 -> wrap to 0
	if u.SpinFrame != 0 {
		t.Errorf("spin frame after wrap = %d, want 0", u.SpinFrame)
	}
	// A count that is not positive does nothing (avoids a division by zero for a unit
	// whose data declares no turret frames).
	u.AdvanceSpin(0)
	if u.SpinFrame != 0 {
		t.Errorf("AdvanceSpin(0) moved the frame to %d", u.SpinFrame)
	}
}

func TestAdvanceWalk(t *testing.T) {
	// A marching soldier cycles its walk phase 0..count-1 and wraps to 0.
	u := NewUnitInfo(0, 0, 1, 0)
	for want := uint8(1); want <= 3; want++ {
		u.AdvanceWalk(4)
		if u.WalkFrame != want {
			t.Fatalf("walk frame = %d, want %d", u.WalkFrame, want)
		}
	}
	u.AdvanceWalk(4) // 3 -> wrap to 0
	if u.WalkFrame != 0 {
		t.Errorf("walk frame after wrap = %d, want 0", u.WalkFrame)
	}
	// A non-positive count does nothing; a non-soldier reports zero phases.
	u.AdvanceWalk(0)
	if u.WalkFrame != 0 {
		t.Errorf("AdvanceWalk(0) moved the frame to %d", u.WalkFrame)
	}
}

func TestOrderStateSettersReturnPrior(t *testing.T) {
	u := NewUnitInfo(0, 0, 1, 0)
	u.SetOrder(OrderMove)
	if prev := u.SetOrder(OrderFire); prev != OrderMove {
		t.Errorf("SetOrder returned %d, want %d", prev, OrderMove)
	}
	if u.Order != OrderFire {
		t.Errorf("order = %d, want %d", u.Order, OrderFire)
	}
	u.SetOrderState(OrderStateInProgress)
	if prev := u.SetOrderState(OrderStateDestroy); prev != OrderStateInProgress {
		t.Errorf("SetOrderState returned %d, want %d", prev, OrderStateInProgress)
	}
}
