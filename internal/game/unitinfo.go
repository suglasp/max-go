// unitinfo.go : defines the runtime state and animation helpers for one unit.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
// C++ ported : unitinfo.hpp - UnitInfo struct and runtime state fields
package game

import (
	"math"

	"github.com/suglasp/max-go/internal/assets"
)

// mapTileSize is the map tile edge in pixels; a unit's pixel position is its cell centre.
//
// C++ ported : gfx.hpp - GFX_MAP_TILE_SIZE
const mapTileSize = 64

// AngleCount is the number of facing directions a unit sprite carries (N, NE, E, SE, S, SW, W, NW).
//
// C++ ported : enums.hpp - UnitAngleType
const AngleCount = 8

// AirAltMax is the drawn altitude of a hovering air unit. Its shadow sits one tile down-right; a landed unit
// is at zero and transitions glide between both states.
const AirAltMax = 8

// BridgeRaiseMax is a bridge's fully raised level: its body sheet has 5 frames, so
// the raise animation runs image_base+0 (flat) .. image_base+4 (fully up out of the
// water). C++ ported : BRIDGE image_index_max = image_count + image_base - 1 (unitinfo.cpp).
const BridgeRaiseMax = 4

// UnitInfo is one map unit's type, team, position, order, facing, and sprite-frame state.
//
// C++ ported : unitinfo.cpp - UnitInfo
type UnitInfo struct {
	ID    uint16
	Type  assets.UnitType
	Team  int
	Flags Flags

	X, Y         int // pixel position (cell center)
	GridX, GridY int // -1 until placed

	BuildToX, BuildToY int // build pose target pixel (footprint centre)

	BuildSiteX, BuildSiteY int // construction site anchor held in the cell hash while Building (whole footprint, not just the builder's cell)
	BuildFootprint         int // site size in cells (1 or 2) while Building; 0 otherwise

	Order      Order
	PriorOrder Order
	State      OrderState
	PriorState OrderState

	Angle             uint8
	TurretAngle       uint8
	TurretTarget      uint8 // desired turret facing; StepTurret rotates toward it
	SpinFrame         uint8 // spinning turret (radar dish) animation frame; AdvanceSpin cycles it
	BodyAnim          uint8 // animated building body cycle frame (mining drill, radar dish); AdvanceBodyAnim cycles it
	WalkFrame         uint8 // marching soldier (infantry/commando) walk cycle phase; AdvanceWalk cycles it
	Building          bool  // a builder (constructor/engineer) is on its site in the build pose; StartBuildPose sets it
	Clearing          bool  // a bulldozer is on a rubble site in the clearing pose; StartClearPose sets it
	Leaving           bool  // a bulldozer is sliding back out of a finished/cancelled 2x2 clear site (PositionInTape DEINIT)
	Unfold            uint8 // landing step that scales up: 0 = full size, 1..4 = drawn progressively smaller (the lander unfolding). C++: scaler_adjust
	HiddenUntilExpand bool  // a just finished building parked behind its build ready builder: not drawn until the builder clears the site, then it unfolds into place. C++: the structure is added to the map status and scaled up (EXPAND) only after UnitsManager_BuildingReady sees the constructor leave its bounds
	Surfaced          bool  // a submarine surfaced (visible sprite); false = submerged. Set when it attacks, is attacked, or is spotted. C++: the SUBMARNE image_base 0/8 state
	PowerStarved      bool  // a powered on consumer that the complex could NOT supply this turn (a power deficit cut it, factories first). Transient, set each economy BalanceTurn; gates the build + the running animation, and drives the "not enough power" alert. C++: the power deficit shutdown priority
	RawStarved        bool  // a RUNNING factory whose build could not draw raw material from the complex pool this turn. Transient, set each AdvanceBuilds; the material short twin of PowerStarved that stalls production and drives the red "needs material" banner. C++: the materials shortage hold in GameManager_ManageEconomy
	BridgeRaise       int   // a bridge's raise level: 0 = flat (ground cover, land crosses over), up to BridgeRaiseMax = fully raised (a sea unit passes under). C++: BRIDGE image_index vs image_base / IsBridgeElevated
	AirAlt            int   // an air unit's drawn altitude (shadow offset) 0..AirAltMax: AirAltMax = flying (FlagHovering), 0 = landed. The tween between landing and taking off glides it toward the FlagHovering target. C++: the HOVERING shadow_offset slide in UnitInfo::Land/Take
	Connectors        uint8 // connection direction bitmask for a structure (bit i = the connector overlay frame to draw). C++: UnitInfo::connectors

	ImageBase          int
	TurretImageBase    int
	FiringImageBase    int
	ConnectorImageBase int
	ImageIndex         int
	TurretImageIndex   int
	ImageIndexMax      int

	Brightness uint8

	Hits   int
	Values *UnitValues

	Experience int // raw veterancy points (a commando gains one per successful disable/steal); GetExperience derives the level

	Speed        int // per turn movement budget left (Order_Move)
	MoveFraction int // remainder below one movement point
	Velocity     int // pixel ramp velocity
	MaxVelocity  int // pixel ramp target velocity
	Moved        int // pixels moved into the current cell

	Shots int // per turn attack budget left (Order_Fire)
	Ammo  int // long term store that depletes per shot and refills at a depot

	DisabledReactionFire bool  // suppresses automatic reaction (opportunity) fire; player toggles it per unit ('4' button)
	AutoSurvey           bool  // human Surveyor auto drives to the nearest unsurveyed cell each turn; "Auto" popup toggle, persisted v12
	SpottedBy            uint8 // teams that have UNCOVERED this stealth unit, one bit per team. A found mine stays found even after the detector drives away. C++: UnitInfo::spotted_by_team

	Storage                           int // cargo held
	RawMining, FuelMining, GoldMining int // per turn extraction (mining station)
	RawMiningMax, FuelMiningMax       int // deposit caps
	GoldMiningMax                     int // deposit cap
	TotalMining                       int // sum of the extraction fields (<=16)

	BuildRate int // factory/builder's current build speed multiplier (1/2/4); 0 when idle

	BuildQueue  []assets.UnitType // FACTORY's ordered production list, [0] currently building, rest pending
	RepeatBuild bool              // loops BuildQueue (a finished head is appended to the tail again)

	LastBuildRate int // build speed a factory ran at when STOPPED, so START resumes at the same rate

	Carried []*UnitInfo // units a transport is ferrying (off map while aboard)
	Parent  *UnitInfo   // the transport this unit is aboard (nil on the map)

	AttackTarget *UnitInfo // enemy a move to attack order is pursuing (ORDER_MOVE_TO_ATTACK) until within weapon range
}

// NewUnitInfo constructs a unit of the given type/team/facing with the original ctor's defaults
// (unplaced at -1,-1, full brightness). The caller wires Flags, sprite bases and Values.
//
// C++ ported : unitinfo.cpp - UnitInfo::UnitInfo
func NewUnitInfo(unitType assets.UnitType, team, id int, angle uint8) *UnitInfo {
	u := &UnitInfo{
		ID:    uint16(id),
		Type:  unitType,
		Team:  team,
		GridX: -1,
		GridY: -1,
		Angle: angle & 0x07,
		// Start the turret at the spawn angle so SetAngle can rotate it with the
		// body until a target changes its aim.
		TurretAngle:  angle & 0x07,
		TurretTarget: angle & 0x07,
		Brightness:   0xFF,
	}
	u.ImageIndex = u.ImageBase + int(u.Angle)
	u.TurretImageIndex = u.TurretImageBase + int(u.Angle)
	return u
}

// SetPosition places the unit at a grid cell and recomputes its pixel centre.
//
// C++ ported : unitinfo.cpp - UnitInfo::SetPosition
func (u *UnitInfo) SetPosition(gridX, gridY int) {
	u.X = gridX*mapTileSize + mapTileSize/2
	u.Y = gridY*mapTileSize + mapTileSize/2
	u.GridX = u.X / mapTileSize
	u.GridY = u.Y / mapTileSize
	// A building's 2x2 sprite hotspot sits half a tile down right of the single cell centre.
	if u.Flags.Has(FlagBuilding) {
		u.X += mapTileSize/2 - 1
		u.Y += mapTileSize/2 - 1
	}
}

// StartBuildPose puts a mobile builder into its build pose at the footprint centre. Constructors build 2x2
// sites and Engineers build 1x1 sites.
//
// C++ ported : unitinfo.cpp - UnitInfo::StartBuilding / PositionInTape
func (u *UnitInfo) StartBuildPose(gx, gy, footprint int) {
	u.Building = true
	// Mark the builder as working (ORDER_BUILD): it is no longer idle, so it
	// holds its cells against passing units; the construction site is reserved
	// while the structure is raised. C++: StartBuilding sets ORDER_STATE_BUILD_IN_PROGRESS.
	u.Order = OrderBuild
	u.State = OrderStateBuildInProgress
	// Reserve the whole site footprint (not just the builder's corner) so units
	// route around it; the cell hash is bucketed again across these cells.
	u.BuildSiteX, u.BuildSiteY, u.BuildFootprint = gx, gy, footprint
	if footprint >= 2 {
		// Face diagonally inward toward the 2x2 centre from whichever corner the
		// builder occupies (angles NE=1, SE=3, SW=5, NW=7). C++: GetTargetUnitAngle
		// in ORDER_STATE_TAPE_POSITIONING_ENTER (top left->SE, bottom left->NE,
		// top right->SW, bottom right->NW).
		left, top := u.GridX == gx, u.GridY == gy
		switch {
		case left && top:
			u.Angle = 3 // SE
		case left && !top:
			u.Angle = 1 // NE
		case !left && top:
			u.Angle = 5 // SW
		default:
			u.Angle = 7 // NW
		}
	}
	// Target the footprint centre but keep the position; AdvanceBuildSlide glides X/Y over.
	u.BuildToX = gx*mapTileSize + footprint*mapTileSize/2
	u.BuildToY = gy*mapTileSize + footprint*mapTileSize/2
}

// AdvanceBuildSlide moves the builder step pixels toward its build site centre, reporting arrival.
//
// C++ ported : units_manager.cpp - ORDER_STATE_TAPE_POSITIONING_ENTER
func (u *UnitInfo) AdvanceBuildSlide(step int) bool {
	u.X = stepToward(u.X, u.BuildToX, step)
	u.Y = stepToward(u.Y, u.BuildToY, step)
	return u.X == u.BuildToX && u.Y == u.BuildToY
}

// stepToward moves cur toward target by at most step, without overshooting.
func stepToward(cur, target, step int) int {
	switch {
	case cur < target:
		return min(cur+step, target)
	case cur > target:
		return max(cur-step, target)
	default:
		return cur
	}
}

// EndBuildPose returns a finished or cancelled builder to its idle position and
// clears the site reservation.
func (u *UnitInfo) EndBuildPose() {
	u.Building = false
	u.BuildFootprint = 0
	if u.Order == OrderBuild {
		u.Order = OrderAwait
		u.State = OrderStateExecutingOrder
	}
}

// BeginBuildLeave ends the build pose and slides the builder back to its cell. A 2x2 builder faces outward
// before leaving.
func (u *UnitInfo) BeginBuildLeave(footprint int) {
	u.Building = false
	u.BuildFootprint = 0
	if u.Order == OrderBuild {
		u.Order = OrderAwait
		u.State = OrderStateExecutingOrder
	}
	u.Leaving = true
	u.BuildToX = u.GridX*mapTileSize + mapTileSize/2
	u.BuildToY = u.GridY*mapTileSize + mapTileSize/2
	if footprint >= 2 {
		u.Angle = (u.Angle + 4) & 7 // inward (build pose) -> outward (driving back out)
	}
}

// StartClearPose puts a bulldozer into its on site clearing pose like a constructor's build
// pose: faces diagonally inward from its 2x2 corner and targets the footprint centre, which
// AdvanceBuildSlide glides over (OrderClear set by the caller). footprint is 1 or 2.
// C++ ported : unitinfo.cpp - UnitInfo::PositionInTape (+8 working frame).
func (u *UnitInfo) StartClearPose(gx, gy, footprint int) {
	u.Clearing = true
	if footprint >= 2 {
		// Face diagonally inward toward the 2x2 centre from the corner it occupies
		// (same mapping as StartBuildPose: top left->SE, bottom left->NE, etc.).
		left, top := u.GridX == gx, u.GridY == gy
		switch {
		case left && top:
			u.Angle = 3 // SE
		case left && !top:
			u.Angle = 1 // NE
		case !left && top:
			u.Angle = 5 // SW
		default:
			u.Angle = 7 // NW
		}
	}
	u.BuildToX = gx*mapTileSize + footprint*mapTileSize/2
	u.BuildToY = gy*mapTileSize + footprint*mapTileSize/2
}

// EndClearPose clears the clearing pose so the bulldozer renders its normal driving
// sprite again (it has finished clearing or is driving off the site).
func (u *UnitInfo) EndClearPose() {
	u.Clearing = false
	u.Leaving = false
}

// BeginClearLeave ends the clearing pose and slides the bulldozer from the site centre back
// onto its OWN cell (the gamescreen slide loop clears Leaving on arrival). A 2x2 turns to
// face OUTWARD to drive out nose first; a 1x1 is already on its cell. C++ ported :
// unitinfo.cpp - PositionInTape (TAPE_POSITIONING_DEINIT).
func (u *UnitInfo) BeginClearLeave(footprint int) {
	u.Clearing = false
	u.Leaving = true
	u.BuildToX = u.GridX*mapTileSize + mapTileSize/2
	u.BuildToY = u.GridY*mapTileSize + mapTileSize/2
	if footprint >= 2 {
		u.Angle = (u.Angle + 4) & 7 // inward (clearing) -> outward (driving back out)
	}
}

// SetAngle turns the unit to face dir (0..7), updating the body image index and rotating the
// head by the same delta when the type can rotate its head independently.
//
// C++ ported : unitinfo.cpp - UnitInfo::UpdateAngle
func (u *UnitInfo) SetAngle(dir uint8) {
	diff := int(dir) - int(u.Angle)
	u.Angle = dir & 0x07
	u.ImageIndex = (u.ImageIndex &^ 0x07) | int(u.Angle)
	if u.Flags.Has(FlagCanHeadRotate) {
		u.TurretAngle = uint8((int(u.TurretAngle) + diff) & 0x07)
		u.TurretTarget = uint8((int(u.TurretTarget) + diff) & 0x07)
		u.TurretImageIndex = u.TurretImageBase + int(u.TurretAngle)
	}
}

// TurnToward rotates one eighth step toward target (shorter way) and reports whether it turned.
//
// C++ ported : paths.cpp - Paths_UpdateAngle
func (u *UnitInfo) TurnToward(target uint8) bool {
	target &= 0x07
	if u.Angle == target {
		return false
	}
	deltaA := int(target) - int(u.Angle)
	if deltaA < 0 {
		deltaA += 8
	}
	deltaB := int(u.Angle) - int(target)
	if deltaB < 0 {
		deltaB += 8
	}
	dir := 1
	if deltaA >= deltaB {
		dir = -1
	}
	u.SetAngle(uint8((int(u.Angle) + dir) & 0x07))
	return true
}

// FiringAngle returns the 16-step clockwise firing direction toward an offset. Missile sprites use the same
// 16-frame heading.
// An axis needs a 4:1 ratio (klei 23/08/26, was 2:1), so shallow angles take the in between frames.
// C++ ported : units_manager.cpp - UnitsManager_GetFiringAngle
func FiringAngle(dx, dy int) int {
	lx, ly := iabs(dx), iabs(dy)
	if dx > 0 || dy > 0 {
		switch {
		case dx >= 0 && dy >= 0: // south east
			switch {
			case ly/4 > lx || lx == 0:
				return 8
			case lx/4 > ly || ly == 0:
				return 4
			case ly/2 >= lx:
				return 7
			case lx/2 >= ly:
				return 5
			default:
				return 6
			}
		case dx > 0: // north east (dy < 0)
			switch {
			case ly/4 > lx || lx == 0:
				return 0
			case lx/4 > ly || ly == 0:
				return 4
			case ly/2 >= lx:
				return 1
			case lx/2 >= ly:
				return 3
			default:
				return 2
			}
		default: // south west (dx < 0, dy > 0)
			switch {
			case ly/4 > lx || lx == 0:
				return 8
			case lx/4 > ly || ly == 0:
				return 12
			case ly/2 >= lx:
				return 9
			case lx/2 >= ly:
				return 11
			default:
				return 10
			}
		}
	}
	// north west quadrant (dx <= 0, dy <= 0; includes the zero offset)
	switch {
	case ly/4 > lx || lx == 0:
		return 0
	case lx/4 > ly || ly == 0:
		return 12
	case ly/2 >= lx:
		return 15
	case lx/2 >= ly:
		return 13
	default:
		return 14
	}
}

// TargetAngle returns the facing (0..7, 0=north, clockwise) from a unit toward offset (dx east,
// dy south), using the original's 2:1 ratio octant test so near diagonals resolve to the diagonal.
//
// C++ ported : units_manager.cpp - UnitsManager_GetTargetAngle
func TargetAngle(dx, dy int) uint8 {
	lx, ly := iabs(dx), iabs(dy)
	switch {
	case dx <= 0 && dy <= 0: // north west quadrant (includes the zero offset)
		switch {
		case ly/2 > lx || lx == 0:
			return 0
		case lx/2 > ly || ly == 0:
			return 6
		default:
			return 7
		}
	case dx >= 0 && dy >= 0: // south east quadrant
		switch {
		case ly/2 > lx || lx == 0:
			return 4
		case lx/2 > ly || ly == 0:
			return 2
		default:
			return 3
		}
	case dx > 0: // north east quadrant
		switch {
		case ly/2 > lx || lx == 0:
			return 0
		case lx/2 > ly || ly == 0:
			return 2
		default:
			return 1
		}
	default: // south west quadrant
		switch {
		case ly/2 > lx || lx == 0:
			return 4
		case lx/2 > ly || ly == 0:
			return 6
		default:
			return 5
		}
	}
}

// AimTurret snaps the gun to face dir (0..7): a unit that rotates its head does so
// alone, while other units turn the whole body.
//
// C++ ported : units_manager.cpp - turret/body angle on fire
func (u *UnitInfo) AimTurret(dir uint8) {
	dir &= 0x07
	if u.Flags.Has(FlagCanHeadRotate) {
		u.TurretAngle = dir
		u.TurretTarget = dir
		u.TurretImageIndex = u.TurretImageBase + int(dir)
		return
	}
	u.SetAngle(dir)
}

// AimTurretAt sets the head's desired facing (0..7) without snapping; StepTurret rotates to it.
// Units without independent head rotation turn their whole body at once.
func (u *UnitInfo) AimTurretAt(dir uint8) {
	dir &= 0x07
	if u.Flags.Has(FlagCanHeadRotate) {
		u.TurretTarget = dir
		return
	}
	u.SetAngle(dir)
}

// StepTurret rotates an independent head one eighth step toward its target (shorter way)
// and reports whether it moved. Called on a timer to animate aiming.
func (u *UnitInfo) StepTurret() bool {
	if !u.Flags.Has(FlagCanHeadRotate) || u.TurretAngle == u.TurretTarget {
		return false
	}
	delta := (int(u.TurretTarget) - int(u.TurretAngle) + 8) & 0x07
	step := 1
	if delta > 4 {
		step = -1
	}
	u.TurretAngle = uint8((int(u.TurretAngle) + step) & 0x07)
	u.TurretImageIndex = u.TurretImageBase + int(u.TurretAngle)
	return true
}

// AdvanceSpin steps a spinning turret unit's dish one frame, wrapping at count; does nothing when count<=0.
//
// C++ ported : unitinfo.cpp - UnitInfo::SpinningTurretAdvanceAnimation
func (u *UnitInfo) AdvanceSpin(count int) {
	if count <= 0 {
		return
	}
	u.SpinFrame = uint8((int(u.SpinFrame) + 1) % count)
}

// AdvanceBodyAnim steps an animated building's body frame one, wrapping at count; does nothing when count<=0.
//
// C++ ported : units_manager.cpp - UnitsManager_ProcessOrderSentry (ANIMATED body advance)
func (u *UnitInfo) AdvanceBodyAnim(count int) {
	if count <= 0 {
		return
	}
	u.BodyAnim = uint8((int(u.BodyAnim) + 1) % count)
}

// AdvanceWalk steps a marching soldier's walk phase one, wrapping at phaseCount; does nothing when phaseCount<=0.
// Phase is the multiple of 8 added to image_base+angle, so phase 0 is the standing frame.
//
// C++ ported : unitinfo.cpp - UnitInfo::Move (COMMANDO/INFANTRY walk advance)
func (u *UnitInfo) AdvanceWalk(phaseCount int) {
	if phaseCount <= 0 {
		return
	}
	u.WalkFrame = uint8((int(u.WalkFrame) + 1) % phaseCount)
}

// iabs returns the absolute value of an integer.
func iabs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}

// ResetMovement refills the per turn movement budget at turn start (speed from ATTRIB_SPEED) and
// clears carry/animation state.
//
// C++ ported : unitinfo.cpp - UnitInfo::Init / RestoreOrders
func (u *UnitInfo) ResetMovement() {
	if u.Values != nil {
		u.Speed = u.Values.Get(AttribSpeed)
	}
	u.MoveFraction = 0
	u.Velocity = 0
	u.MaxVelocity = 0
	u.Moved = 0
}

// InitState seeds per unit runtime from the upgraded base values: full hits, ammo and shots.
// Call on create/deploy.
//
// C++ ported : unitinfo.cpp - UnitInfo::Init
func (u *UnitInfo) InitState() {
	if u.Values == nil {
		return
	}
	u.Hits = u.Values.Get(AttribHits)
	u.Ammo = u.Values.Get(AttribAmmo)
	u.Shots = u.Values.Get(AttribRounds)
	u.Speed = u.Values.Get(AttribSpeed)
	// A mine layer starts loaded with its full mine store (Storage = ATTRIB_STORAGE),
	// so it can immediately lay mines. C++: the mine layer's initial storage.
	if u.Type == assets.MineLayr || u.Type == assets.SeaMnLyr {
		u.Storage = u.Values.Get(AttribStorage)
	}
	// A unit spawned flying starts at full altitude so its shadow is drawn offset
	// (it flies above its shadow); the tween between landed and taking off moves AirAlt from here.
	if u.Flags.Has(FlagMobileAirUnit) && u.Flags.Has(FlagHovering) {
		u.AirAlt = AirAltMax
	}
}

// ResetCombat refills the per turn shot budget at turn start (ATTRIB_ROUNDS, capped by ammo).
//
// C++ ported : unitinfo.cpp - UnitInfo::RestoreOrders
func (u *UnitInfo) ResetCombat() {
	if u.Values == nil {
		return
	}
	rounds := u.Values.Get(AttribRounds)
	if rounds <= 0 {
		return
	}
	if rounds > u.Ammo {
		u.Shots = u.Ammo
	} else {
		u.Shots = rounds
	}
}

// MovesAndFires reports whether the unit can move and fire in the same turn for free
// (ATTRIB_MOVE_AND_FIRE), exempting it from the shared move/fire budget below.
func (u *UnitInfo) MovesAndFires() bool {
	return u.Values != nil && u.Values.Get(AttribMoveAndFire) != 0
}

// SpendShotSpeed debits the movement cost of one shot just fired: move and fire share one
// per turn budget, so each shot eats a slice of Speed (all ATTRIB_ROUNDS uses the whole bar).
// Call AFTER Shots is decremented (matches the original's post --shots run). MOVE_AND_FIRE
// exempt; Speed floors at zero. C++ ported : unitinfo.cpp - UnitInfo::UpdateInfoDisplay
func (u *UnitInfo) SpendShotSpeed() {
	if u.Values == nil || u.MovesAndFires() {
		return
	}
	rounds := u.Values.Get(AttribRounds)
	if rounds <= 0 {
		return
	}
	baseSpeed := u.Values.Get(AttribSpeed)
	u.Speed -= ((u.Shots+1)*baseSpeed)/rounds - (u.Shots*baseSpeed)/rounds
	if u.Speed < 0 {
		u.Speed = 0
	}
}

// ClampShotsToSpeed lowers Shots to what the leftover Speed can afford (the reverse of
// SpendShotSpeed): after spending movement a unit may not have the budget for all its shots.
// Call after a move step debits Speed. Never raises Shots; MOVE_AND_FIRE exempt.
// C++ ported : paths.cpp - Paths_CalculateStep (shots clamp)
func (u *UnitInfo) ClampShotsToSpeed() {
	if u.Values == nil || u.MovesAndFires() {
		return
	}
	baseSpeed := u.Values.Get(AttribSpeed)
	if baseSpeed <= 0 {
		return
	}
	allowed := (u.Values.Get(AttribRounds) * u.Speed) / baseSpeed
	if allowed < u.Shots {
		u.Shots = allowed
	}
}

// GetExperience returns the displayed veterancy level from raw experience. It drives Commando disable and
// steal success and duration.
func (u *UnitInfo) GetExperience() int {
	lvl := int(math.Floor(math.Sqrt(float64(u.Experience*10)))) - 2
	if lvl < 0 {
		return 0
	}
	return lvl
}

// BodyFrameIndex is the body frame for the current facing (image_base + angle).
func (u *UnitInfo) BodyFrameIndex() int {
	return u.ImageBase + int(u.Angle&0x07)
}

// TurretFrameIndex is the turret frame for the current turret facing (turret_image_base + turret_angle).
func (u *UnitInfo) TurretFrameIndex() int {
	return u.TurretImageBase + int(u.TurretAngle&0x07)
}

// Position returns the unit's grid cell.
func (u *UnitInfo) Position() Point {
	return Point{u.GridX, u.GridY}
}

// SpotBy records that a team has uncovered this stealth unit. The mark sticks:
// a mine a surveyor walked past stays on the map once found, even after the
// surveyor moves on.
//
// C++ ported : unitinfo.cpp - UnitInfo::SpotByTeam
func (u *UnitInfo) SpotBy(team int) {
	if team < 0 || team >= TeamSlots {
		return
	}

	u.SpottedBy |= 1 << uint(team)
}

// SpottedByTeam reports whether a team has already uncovered this stealth unit.
func (u *UnitInfo) SpottedByTeam(team int) bool {
	if team < 0 || team >= TeamSlots {
		return false
	}

	return u.SpottedBy&(1<<uint(team)) != 0
}

// SetOrder sets the current order and returns the previous one; the prior order is untouched.
//
// C++ ported : unitinfo.cpp - UnitInfo::SetOrder
func (u *UnitInfo) SetOrder(o Order) Order {
	prev := u.Order
	u.Order = o
	return prev
}

// SetPriorOrder sets the prior order and returns the previous value.
func (u *UnitInfo) SetPriorOrder(o Order) Order {
	prev := u.PriorOrder
	u.PriorOrder = o
	return prev
}

// SetOrderState sets the current order state and returns the previous one.
//
// C++ ported : unitinfo.cpp - UnitInfo::SetOrderState
func (u *UnitInfo) SetOrderState(s OrderState) OrderState {
	prev := u.State
	u.State = s
	return prev
}

// SetPriorOrderState sets the prior order state and returns the previous one.
//
// C++ ported : unitinfo.cpp - UnitInfo::SetPriorOrderState
func (u *UnitInfo) SetPriorOrderState(s OrderState) OrderState {
	prev := u.PriorState
	u.PriorState = s
	return prev
}
