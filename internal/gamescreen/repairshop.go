// repairshop.go : implements the repair shop overlay on the game screen.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
// C++ ported : unitinfo.cpp - repair shop panel display
package gamescreen

import (
	"github.com/suglasp/max-go/internal/assets"
	"github.com/suglasp/max-go/internal/build"
	"github.com/suglasp/max-go/internal/dbg"
	"github.com/suglasp/max-go/internal/game"
	"github.com/suglasp/max-go/internal/gfx"
	"github.com/suglasp/max-go/internal/repairshopui"
	"github.com/suglasp/max-go/internal/sprites"
)

// Repair Shop host side. A depot/hangar/dock/barracks popup opens the DEPOTFRM/
// HANGRFRM screen for the friendly mobile units it services; the screen pulls
// its data and runs reload/repair/upgrade through RepairShopOps. The gamescreen
// resolves all sim state (which units, complex raw, the operations) so the menus
// package stays free of the build network dependency; the same split Transfer
// uses; cmd bridges these gamescreen DTOs onto the parallel menus types.
//
// C++: RepairShopMenu (repairshopmenu.cpp) + Access_IsHeldByUnit.

// The Repair Shop data contract (the two DTOs + the backend interface) lives in the
// shared repairshopui package, aliased here so the host side keeps its short local
// names. Because the menus screen aliases the SAME types, a value the gamescreen
// resolves is already a menus.RepairShopOps, so the host passes it straight to the
// screen with no field by field bridge.
type (
	RepairUnitInfo = repairshopui.RepairUnitInfo
	RepairShopInfo = repairshopui.RepairShopInfo
	RepairShopOps  = repairshopui.RepairShopOps
)

// SetRepairFunc installs the hook a service building's popup calls to open the
// Repair Shop screen (wired to menus.RunRepairShop via a bridge in cmd).
func (s *Screen) SetRepairFunc(fn func(ops RepairShopOps) error) {
	s.hooks.repairFn = fn
}

// isServiceBuilding reports whether a building type runs a Repair Shop screen.
// C++: the DEPOT/HANGAR/DOCK/BARRACKS cases in RepairShopMenu.
func isServiceBuilding(t assets.UnitType) bool {
	switch t {
	case assets.Depot, assets.Hangar, assets.Dock, assets.Barracks:
		return true
	}

	return false
}

// isFactory reports whether a building type is a unit factory (its popup builds
// mobile units). C++: the SHIPYARD/LIGHTPLT/LANDPLT/AIRPLT/TRAINHAL cases that
// map to UnitsManager_Popup_InitFactories (unitinfo.cpp).
func isFactory(t assets.UnitType) bool {
	switch t {
	case assets.Shipyard, assets.LightPlt, assets.LandPlt, assets.AirPlt, assets.TrainHal:
		return true
	}

	return false
}

// openRepairShop resolves the serviced units + complex raw for a building and
// runs the Repair Shop screen through the hook.
func (s *Screen) openRepairShop(building *game.UnitInfo) {
	if s.hooks.repairFn == nil || building == nil {
		return
	}

	raw, _ := s.complexRaw(building)
	ops := &screenRepairOps{s: s, building: building, team: building.Team, rawCapacity: raw}

	// The shop manages the units STORED inside the building (its Carried bay);
	// Snapshot reads them live so an eject reflects at once.
	if err := s.hooks.repairFn(ops); err != nil {
		dbg.Printf("repair shop: %v", err)
	}
}

/* // note: ported but unused code in Go port
// servicedUnits returns the friendly mobile units a building services: air units
// for a hangar, land/sea units otherwise, that touch the building's footprint.
//
// C++: the Access_IsHeldByUnit loop over MobileAirUnits / MobileLandSeaUnits.
func (s *Screen) servicedUnits(building *game.UnitInfo) []*game.UnitInfo {
	air := building.Type == assets.Hangar
	var out []*game.UnitInfo

	for _, u := range s.game.Units.Units() {
		if u == building || u.Team != building.Team || !u.Flags.Has(game.FlagSelectable) {
			continue
		}

		mobileAir := u.Flags.Has(game.FlagMobileAirUnit)
		mobileLandSea := u.Flags.Has(game.FlagMobileLandUnit) || u.Flags.Has(game.FlagMobileSeaUnit)

		if air != mobileAir || (!air && !mobileLandSea) {
			continue
		}

		if !unitsAdjacent(building, u) {
			continue
		}

		out = append(out, u)
	}

	return out
}
*/

// armoryTag maps a unit type to its "A_<abbrev>" armory portrait resource (the
// large transport/loadout artwork MAX.RES ships). Buildings, projectiles and the
// constructor/builder style units have no entry (they have no A_ image). The
// abbreviations are irregular, so the table is explicit. C++: the per unit
// GetArmoryPortrait() ResourceID.
var armoryTag = map[assets.UnitType]string{
	assets.SpFlak: "A_AA", assets.AirTrans: "A_AIRTRN", assets.AlnAsGun: "A_ALNASG",
	assets.AlnPlane: "A_ALNPLA", assets.AlnTank: "A_ALNTAN", assets.Artillry: "A_ARTY",
	assets.Awac: "A_AWAC", assets.Bomber: "A_BOMBER", assets.Bulldozr: "A_BULLDZ",
	assets.CargoShp: "A_CARGOS", assets.ClnTrans: "A_COLNST", assets.Commando: "A_COMMAN",
	assets.Constrct: "A_CONTRC", assets.Corvette: "A_CORVET", assets.Engineer: "A_ENGINR",
	assets.FastBoat: "A_ESCORT", assets.Fighter: "A_FIGHTR", assets.FuelTrck: "A_FUELTR",
	assets.GoldTrck: "A_GOLDTR", assets.Infantry: "A_INFANT", assets.Juggrnt: "A_JUGGER",
	assets.MisslLch: "A_MISSIL", assets.MineLayr: "A_MNELAY", assets.MsslBoat: "A_MSLCR",
	assets.Repair: "A_REPAIR", assets.RocktLch: "A_ROCKET", assets.Scanner: "A_SCANNR",
	assets.Scout: "A_SCOUT", assets.SeaMnLyr: "A_SEAMNL", assets.SeaTrans: "A_SEATRN",
	assets.SplyTrck: "A_SPLYTR", assets.Submarne: "A_SUB", assets.Surveyor: "A_SURVEY",
	assets.Tank: "A_TANK",
}

// armoryPortrait decodes a unit type's A_<tag> armory image (the well artwork),
// or returns nil when the type has none (constructor/builder fall back to the
// small body icon). The image is drawn with its raw indices under the shop's
// palette, like WindowManager_LoadBigImage(apply_palette=false).
func (s *Screen) armoryPortrait(t assets.UnitType) *gfx.Buffer {
	tag, ok := armoryTag[t]

	if !ok || s.lib == nil {
		return nil
	}

	data, err := s.lib.Read(tag)

	if err != nil {
		return nil
	}

	img, err := sprites.DecodeBig(data)

	if err != nil {
		return nil
	}

	return img.Buffer().Clone()
}

// complexRaw reports a building's complex POOLED raw material (what the shop
// actually spends, via drawComplexRaw -> Complex.Transfer) and whether it belongs
// to a complex at all. It reads the pool (c.Material), NOT the sum of the member
// buildings' displayed cargo gauges (CargoInfo) - the pool is the source of truth
// for building/repair, and the two can differ when the pool holds more than the
// gauges show (over capacity, or before an economy tick has redistributed it).
func (s *Screen) complexRaw(building *game.UnitInfo) (raw int, hasComplex bool) {
	c := s.game.ComplexOf(building)

	if c == nil {
		return 0, false
	}

	return c.Material, true
}

// drawComplexRaw withdraws (delta<0) or returns raw material to a building's
// complex store, distributing it across member buildings like a transfer.
func (s *Screen) drawComplexRaw(building *game.UnitInfo, delta int) {
	if c := s.game.ComplexOf(building); c != nil {
		c.Transfer(delta, 0, 0)
	}
}

// screenRepairOps is the Screen backed RepairShopOps for one shop session. units
// is the serviced set, fixed at open so the menu's indices stay valid.
type screenRepairOps struct {
	s           *Screen
	building    *game.UnitInfo
	team        int
	rawCapacity int // raw present when the screen opened (the gauge full scale)
	units       []*game.UnitInfo
}

func (o *screenRepairOps) Snapshot() RepairShopInfo {
	o.units = o.building.Carried // the stored bay, read live each refresh
	raw, hasComplex := o.s.complexRaw(o.building)
	isShop := o.building.Flags.Has(game.FlagStationary)
	info := RepairShopInfo{
		IsHangar:        o.building.Type == assets.Hangar,
		IsShop:          isShop,
		ShowMaterials:   hasComplex && isServiceBuilding(o.building.Type),
		ShowActivateAll: o.building.Type != assets.AirTrans,
		RawMaterials:    raw,
		RawCapacity:     o.rawCapacity,
	}

	for _, u := range o.units {
		info.Units = append(info.Units, o.unitInfo(u, isShop, raw))
	}

	return info
}

// unitInfo resolves one serviced unit's display + viability state.
func (o *screenRepairOps) unitInfo(u *game.UnitInfo, isShop bool, raw int) RepairUnitInfo {
	maxHits := u.Values.Get(game.AttribHits)
	maxAmmo := u.Values.Get(game.AttribAmmo)
	maxCargo := u.Values.Get(game.AttribStorage)
	combat := u.Values.Get(game.AttribAttack) > 0
	// Prefer the large A_<tag> armory artwork; fall back to the small body icon
	// for the few units with no A_ image (constructor/builder).
	portrait := o.s.armoryPortrait(u.Type)

	if portrait == nil {
		portrait = o.s.unitIcon(u)
	}

	ru := RepairUnitInfo{
		Name:     displayName(u.Type),
		Portrait: portrait,
		Hits:     u.Hits, MaxHits: maxHits,
	}

	if combat && maxAmmo > 0 {
		ru.Ammo, ru.MaxAmmo = u.Ammo, maxAmmo
	} else if maxCargo > 0 {
		ru.ShowCargo = true
		ru.Cargo, ru.MaxCargo = u.Storage, maxCargo
	}

	// IsActivateViable (repairshopmenu.cpp:657-659): a stored AIR unit needs
	// speed left to lift off; everything else activates freely.
	ru.CanActivate = !u.Flags.Has(game.FlagMobileAirUnit) || u.Speed > 0
	ru.CanReload = isShop && maxAmmo > 0 && u.Ammo < maxAmmo && raw > 0
	ru.CanRepair = isShop && u.Hits < maxHits && raw > 0
	ru.CanUpgrade = isShop && !u.Flags.Has(game.FlagRegenerating) && o.upgradeViable(u, raw)
	return ru
}

// upgradeViable reports whether a unit can be upgraded here: the complex holds at
// least the build cost quarter and the team's current baseline beats the unit's
// in service values on some attribute. C++: RepairShopMenu::IsUpgradeViable.
func (o *screenRepairOps) upgradeViable(u *game.UnitInfo, raw int) bool {
	if raw < o.upgradeCost(u) {
		return false
	}

	team := o.s.teamValues(o.team, u.Type)

	for a := 0; a < game.ResearchTopicCount; a++ {
		ga := game.Attrib(a)

		if team.Get(ga) > u.Values.Get(ga) {
			return true
		}
	}

	return false
}

// upgradeCost is the raw the shop charges to upgrade one unit: a quarter of its
// normal build cost. C++: GetNormalRateBuildCost() / 4.
func (o *screenRepairOps) upgradeCost(u *game.UnitInfo) int {
	return build.Cost(u) / 4
}

func (o *screenRepairOps) Reload(i int) {
	if i < 0 || i >= len(o.units) {
		return
	}

	u := o.units[i]
	maxAmmo := u.Values.Get(game.AttribAmmo)
	raw, _ := o.s.complexRaw(o.building)

	if maxAmmo <= 0 || u.Ammo >= maxAmmo || raw <= 0 {
		return
	}

	reload := reloadAmount(maxAmmo, u.Ammo, raw)
	u.Ammo += reload
	o.s.drawComplexRaw(o.building, -reload)
	o.s.playVoice(vsReloaded) // "Unit reloaded" (V_F085/089); the cooldown collapses ReloadAll to one cue
}

func (o *screenRepairOps) Repair(i int) {
	if i < 0 || i >= len(o.units) {
		return
	}

	u := o.units[i]
	maxHits := u.Values.Get(game.AttribHits)
	raw, _ := o.s.complexRaw(o.building)

	if u.Hits >= maxHits || raw <= 0 {
		return
	}

	heal, cost := repairHeal(maxHits, u.Hits, raw)
	u.Hits += heal
	o.s.drawComplexRaw(o.building, -cost)
	o.s.playVoice(vsRepaired) // "Unit repaired" (V_F210/211/219/220); cooldown collapses RepairAll to one cue
}

func (o *screenRepairOps) Upgrade(i int) {
	if i < 0 || i >= len(o.units) {
		return
	}

	u := o.units[i]

	if u.Flags.Has(game.FlagRegenerating) {
		return
	}

	cost := o.upgradeCost(u)
	raw, _ := o.s.complexRaw(o.building)

	if raw < cost || !o.upgradeViable(u, raw) {
		return
	}

	// Bring the unit up to the team's current upgraded baseline, scaling current HP by the
	// max HITS delta so a full unit stays full. C++: ORDER_UPGRADE / UnitInfo::UpgradeInt.
	o.s.upgradeUnitValues(o.team, u)
	o.s.drawComplexRaw(o.building, -cost)
}

func (o *screenRepairOps) Activate(i int) {
	if i < 0 || i >= len(o.units) {
		return
	}

	unit := o.units[i]

	// An AIR transport drops the unit on the ground BENEATH itself with a descend /
	// take off animation (it has no footprint to park a build ready unit on); a
	// Depot/Dock/Hangar or a land/sea transport ejects it build ready with BLDMRK exit
	// markers. C++: ProgressUnloading / Land / Take (air, unitinfo.cpp) vs the depot
	// "Activate" that deploys a stored unit back out.
	if o.building.Type == assets.AirTrans {
		// Refuse BEFORE the descent when the ground beneath cannot take the unit
		// (blocked, or terrain its move class cannot stand on - dropping a land
		// unit onto water would strand it forever). C++ ported : repairshopmenu.cpp -
		// Access_IsAccessible gates ORDER_UNLOAD, message _(eeb4).
		if !o.s.game.CanUnloadBeneath(o.building, unit) {
			o.s.showMessage(o.s.lang.Get("eeb4", "Unable to activate unit at this site."))
			return
		}
		o.s.beginAirUnload(o.building, unit)
		return
	}

	// Activate (repairshopmenu.cpp:676-689): probe for a reachable adjacent
	// spot first - refuse with the retail a1f0 line when the carrier is boxed in -
	// then arm the click to place pick ("Select an open square to place unit.").
	// The menu has already closed, so the pick happens on the map.
	if _, _, ok := o.s.game.UnloadSpot(o.building, unit); !ok {
		o.s.showInfo(o.s.lang.Get("a1f0", "Unable to activate unit at this site."))
		return
	}

	o.s.armUnload(o.building, unit)
}

func (o *screenRepairOps) ReloadAll() {
	for i := range o.units {
		o.Reload(i)
	}
}

func (o *screenRepairOps) RepairAll() {
	for i := range o.units {
		o.Repair(i)
	}
}

func (o *screenRepairOps) UpgradeAll() {
	for i := range o.units {
		o.Upgrade(i)
	}
}

func (o *screenRepairOps) ActivateAll() {
	// Only one unit can park build ready (be ejected) at a time, so "Activate All"
	// ejects the first stored unit; the player drives it out, then ejects the next.
	if len(o.units) > 0 {
		o.Activate(0)
	}
}

// reloadAmount is the shells a reload restores given the unit's full/current
// ammo and the complex raw available: one raw per shell, bounded by both. C++:
// the per turn ORDER_RELOAD material draw, simplified (exact rate carried over).
func reloadAmount(maxAmmo, ammo, raw int) int {
	if maxAmmo <= ammo || raw <= 0 {
		return 0
	}

	return min(maxAmmo-ammo, raw)
}

// repairHeal is the hits a repair restores and the raw it costs: one raw per four
// hits, bounded by the damage and the raw available. C++: the per turn
// ORDER_REPAIR material draw, simplified (exact rate carried over).
func repairHeal(maxHits, hits, raw int) (heal, cost int) {
	if maxHits <= hits || raw <= 0 {
		return 0, 0
	}

	heal = min(maxHits-hits, raw*4)
	return heal, (heal + 3) / 4
}
