// popupitems.go : builds the order list the unit popup shows, per unit kind.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package gamescreen

import (
	"github.com/suglasp/max-go/internal/assets"
	"github.com/suglasp/max-go/internal/economy"
	"github.com/suglasp/max-go/internal/game"
	"github.com/suglasp/max-go/internal/sim"
)

// popupItemsFor assembles the context orders for a unit, faithful to the per type
// UnitsManager_Popup_Init* dispatch: mobile units get their type orders then the shared
// commons; buildings get power on/off plus their type orders then commons. Unmodelled
// orders raise a "not available" banner rather than being hidden, so the menu shape
// matches. C++: UnitsManager_Popup_Init* + InitCommons; dispatch is the UnitInfo::Init
// switch (units_manager.cpp / unitinfo.cpp).
func (s *Screen) popupItemsFor(u *game.UnitInfo) []popupItem {
	if u.Flags.Has(game.FlagStationary) {
		return s.buildingPopupItems(u)
	}

	return s.unitPopupItems(u)
}

// unitPopupItems builds the order rows for a MOBILE unit, in the order they appear on
// screen. It leads with the orders specific to what the unit is, then hands off to
// appendCommons for the ones every unit shares.
// C++ ported : units_manager.cpp - UnitsManager_Popup_InitMilitary and friends
func (s *Screen) unitPopupItems(u *game.UnitInfo) []popupItem {
	var items []popupItem

	// Continue leads for a unit that halted on spotting an enemy: it still holds the rest
	// of its drive (arrows and all), and this sends it on, re pathing from where it stands.
	// Nothing else resumes a held route, which is the point - the player decides whether to
	// press on or deal with what the unit found.
	//
	// The original discards the route at the halt (access.cpp:829 -
	// UnitEventEmergencyStop), so it cannot offer this action.
	if len(s.game.HeldRoute(u)) > 0 {
		items = append(items, popupItem{
			tex:   "MOVE",
			label: "Continue",
			action: func() {
				s.orderInput.popupOpen = false

				if !s.game.ContinueHeldRoute(u) {
					s.showMessage("No route to that destination any more.")
				}
			},
		})
	}

	// Auto is the FIRST item for a player owned Surveyor (vanilla order): it drives
	// itself toward the nearest unsurveyed cell, starting RIGHT AWAY while moves remain
	// this turn. Kept SEPARATE from Sentry (distinct vanilla buttons).
	// C++ ported : units_manager.cpp - UnitsManager_Popup_OnClick_Auto.
	if u.Type == assets.Surveyor && s.game.State != nil && u.Team == s.game.State.PlayerTeam {
		items = append(items, popupItem{
			tex:    "AUTO",
			label:  s.lang.Get("auto", "Auto"),
			active: u.AutoSurvey, // green highlight when automatic survey is on
			action: func() {
				u.AutoSurvey = !u.AutoSurvey

				// Start surveying at once while moves remain this turn.
				if u.AutoSurvey {
					s.driveAutoSurvey(u, s.game.State.PlayerTeam)
				}
			},
		})
	}

	// Builders (constructor/engineer) lead with Stop while a build is in progress,
	// Build otherwise.
	//
	// C++ ported : units_manager.cpp - UnitsManager_Popup_InitBuilders
	if rosterFor(u.Type) != nil && u.Flags.Has(game.FlagConstructorUnit) {
		if s.game.BuildOrderFor(u) != nil || s.game.BuildPathActive(u) {
			// A build in progress - or a Path (line) run between cells - is stopped here.
			items = append(items, popupItem{tex: "STOP", label: s.lang.Get("bf85", "Stop"), action: func() {
				s.orderInput.popupOpen = false
				s.game.CancelBuildPath(u) // drop any remaining line run
				s.game.CancelBuild(u)     // and the cell currently under construction
				s.showMessage("Build cancelled.")
			}})
		} else {
			items = append(items, popupItem{tex: "BUILD", label: s.lang.Get("7b6d", "Build"), action: func() {
				s.orderInput.popupOpen = false
				s.openMenuScreen(s.openBuild, true)
			}})
		}
	}

	// A transport carrying units offers Activate: opens the Repair Shop, which lists
	// the carried units so the player picks WHICH to unload; the chosen unit ejects with
	// the same build ready exit (BLDMRK markers + drive out) a Depot/Dock/Hangar uses.
	// C++: Popup_OnClick_Activate -> RepairShopMenu_Menu (units_manager.cpp).
	if sim.IsTransport(u.Type) && len(u.Carried) > 0 {
		items = append(items, popupItem{tex: "ACTVT", label: s.lang.Get("200c", "Activate"), action: func() {
			s.orderInput.popupOpen = false
			s.openMenuScreen(func() { s.openRepairShop(u) }, true)
		}})
	}

	// Load: a transport picks a friendly unit to carry (land/sea sucks it up in place;
	// air flies over, descends, picks up, rises - tryTransportTarget routes by type). The
	// item shows even on a FULL transport (so it does not vanish); armLoadInto then
	// reports it is full. C++: Popup_OnClick_Load.
	if sim.IsTransport(u.Type) {
		items = append(items, popupItem{tex: "LOAD", label: s.lang.Get("load", "Load"), action: func() { s.armLoadInto(u) }})
	}

	return s.appendCommons(u, items)
}

// buildingPopupItems builds the order rows for a STRUCTURE, switching on what the
// building does: power plants and generators, factories, the service buildings, then
// everything else. It closes with the orders every unit shares.
// C++ ported : units_manager.cpp - UnitsManager_Popup_InitFactories / InitCommons
func (s *Screen) buildingPopupItems(u *game.UnitInfo) []popupItem {
	var items []popupItem
	on := u.Order == game.OrderPowerOn

	switch {
	case u.Type == assets.PowerStn || u.Type == assets.PowGen:
		// Power generators: just power on/off. C++: InitPowerGenerators.
		items = append(items, s.powerItem(u, on))

	case u.Type == assets.MiningSt:
		// Mining station: Allocate (while powered) + power off; power on while off.
		// C++: InitMiningStation (PowerOnAllocate).
		if on {
			items = append(items, popupItem{tex: "ALLOC", label: s.lang.Get("33b7", "Allocate"), action: func() {
				s.orderInput.popupOpen = false
				s.openMenuScreen(func() { s.openAlloc(u) }, false)
			}})
			items = append(items, s.powerItem(u, true))
		} else {
			items = append(items, s.powerItem(u, false))
		}

	case u.Type == assets.Research:
		// Research centre: run state follows its topic allocation, not the raw power
		// order, so "available" (not forced off) is the on state. Offer Research (allocate)
		// whenever available - including an automatically idled centre - plus a power
		// toggle that forces it off / turns it back on. C++: InitResearch.
		available := u.Order != game.OrderPowerOff

		if available {
			items = append(items, popupItem{tex: "RSRCH", label: s.lang.Get("2937", "Research"), action: func() {
				s.orderInput.popupOpen = false
				s.openMenuScreen(s.openResearch, false)
			}})
		}

		items = append(items, s.powerItem(u, available))

	case u.Type == assets.Greenhse:
		// Eco Sphere: power on/off only. C++: InitEcoSphere.
		items = append(items, s.powerItem(u, on))

	case u.Type == assets.CommTwr:
		// Gold Refinery (COMMTWR): buy upgrade + power on/off. The Upgrade button shows
		// only on an OPERATING (powered) refinery, per the manual ("click on an operating
		// refinery"); a powered off one offers just the power toggle. C++: InitGoldRefinery.
		if on {
			items = append(items, popupItem{tex: "UPGRD", label: s.lang.Get("1da0", "Buy Upgrade"), action: func() {
				s.orderInput.popupOpen = false
				s.openMenuScreen(s.openUpgrade, false)
			}})
		}

		items = append(items, s.powerItem(u, on))

	case isServiceBuilding(u.Type):
		items = s.serviceBuildingItems(u, items)

	case isFactory(u.Type):
		items = s.factoryItems(u, items)
	}

	// Material/fuel dumps (ADUMP/FDUMP) and anything else fall through to commons
	// only: no power toggle, matching InitCommons/InitFuelSuppliers.
	return s.appendCommons(u, items)
}

// serviceBuildingItems adds a depot, hangar, dock or barracks' own orders.
func (s *Screen) serviceBuildingItems(u *game.UnitInfo, items []popupItem) []popupItem {
	// Depot/hangar/dock/barracks: Activate opens the Repair Shop, Load stores a unit.
	// C++: InitRepairShops (Activate + Load).
	items = append(items, popupItem{tex: "ACTVT", label: s.lang.Get("200c", "Activate"), action: func() {
		s.orderInput.popupOpen = false
		s.openMenuScreen(func() { s.openRepairShop(u) }, true)
	}})

	// Load: pick a friendly unit (matched by class); it paths to this building and is
	// sucked up into the bay. The item shows even when the bay is FULL (so the option
	// does not vanish); armLoadInto then reports it is full. C++: the depot/hangar/dock
	// Load order.
	if sim.IsStorageBuilding(u.Type) {
		items = append(items, popupItem{tex: "LOAD", label: s.lang.Get("load", "Load"), action: func() { s.armLoadInto(u) }})
	}

	return items
}

// factoryItems adds a unit factory's own orders: Build always, plus the Start xor Stop
// toggle once it holds a queue.
func (s *Screen) factoryItems(u *game.UnitInfo, items []popupItem) []popupItem {
	// Unit factories produce mobile units through the FACBUILD build queue. With a
	// queue they show a Start xor Stop toggle that KEEPS the queue and partial build;
	// an empty factory shows only BUILD. There is NO power toggle: vanilla
	// InitFactories registers only Build + Start/Stop (a factory draws power on its own
	// only while producing, so there is nothing to switch by hand). The message is
	// raised by tickProductionEvents, not here.
	// C++: InitFactories (Build + Start/Stop, no PowerOn/PowerOff); ManageEconomy raises the message.
	if q, _ := s.game.FactoryQueue(u); len(q) > 0 {
		// Vanilla labels: "Start" (_cced) on a halted factory resumes it, "Stop"
		// (_bcf7) on a running one halts it - only one shows at a time, both keep the
		// queue and partial build.
		// C++: InitFactories - StartBuild(_cced) on ORDER_HALT_BUILDING vs StopBuild(_bcf7) on ORDER_BUILD.
		if s.game.ProductionState(u) == sim.ProdPaused {
			// Halted by the player: Start picks production back up at the remembered rate.
			items = append(items, popupItem{tex: "START", label: s.lang.Get("cced", "Start"), action: func() {
				s.orderInput.popupOpen = false
				s.game.StartFactory(u)
			}})
		} else {
			// Running (or stalled on resources): Stop pauses it, KEEPING the whole queue.
			items = append(items, popupItem{tex: "STOP", label: s.lang.Get("bcf7", "Stop"), action: func() {
				s.orderInput.popupOpen = false
				s.game.PauseFactory(u)
			}})
		}
	}

	items = append(items, popupItem{tex: "BUILD", label: s.lang.Get("7b6d", "Build"), action: func() {
		s.orderInput.popupOpen = false
		s.openMenuScreen(func() { s.openFactoryBuild(u) }, true)
	}})

	return items
}

// busyOrder reports whether a unit is mid build or mid clear - the two order states
// the original's InitCommons excludes from the X-Fer / Enter / Stop commons buttons. A builder
// raising a structure (ORDER_BUILD) or a bulldozer clearing rubble (ORDER_CLEAR) may
// not transfer cargo or board a container until it stops.
func busyOrder(u *game.UnitInfo) bool {
	return u.Order == game.OrderBuild || u.Order == game.OrderClear
}

// appendCommons adds the shared order buttons, faithful to
// UnitsManager_Popup_InitCommons: X-Fer only for cargo carrying units (raw/fuel/
// gold), Enter for mobile units, Stop while moving, Upgrade for upgradable types,
// Sentry+Done for mobile units, and Remove for stationary buildings.
func (s *Screen) appendCommons(u *game.UnitInfo, items []popupItem) []popupItem {
	mobile := u.Flags.Has(game.FlagMobileLandUnit) ||
		u.Flags.Has(game.FlagMobileSeaUnit) ||
		u.Flags.Has(game.FlagMobileAirUnit)

	// Attack: registered for every military unit (mobile guns + defensive turrets),
	// unconditionally, exactly like the original's UnitsManager_Popup_InitMilitary - the
	// button only toggles the targeting cursor, so it stays visible even when the unit
	// has spent its shots (arming with 0 shots is inert, the fire path raises the "no
	// shots" banner). UnitArmed = combat.ValidTargetTypes(type) != 0 is the "is military"
	// proxy, so unarmed units (Constructor, Surveyor) still omit it.
	// C++ ported : units_manager.cpp - UnitsManager_Popup_InitMilitary.
	if s.game.UnitArmed(u) {
		items = append(items, popupItem{tex: "ATTACK", label: s.lang.Get("attack", "Attack"), action: func() { s.armAttack(u) }})
	}

	items = s.appendSpecialistOrders(u, items)

	// Manual: the reaction fire toggle. Any armed unit or building (base ammo > 0, not a
	// Commando) can switch its automatic opportunity fire off (fire by hand) and back on,
	// highlighted while manual. Defense turrets have ammo, so they get it alongside Sentry.
	// Toggling out of auto drops an on guard order back to awaiting (releases ORDER_SENTRY).
	// DELIBERATE DEVIATION: the original's InitCommons also gates Manual on PLAY_MODE_SIMULTANEOUS_MOVES;
	// the port supports reaction fire in turn based play too, so that condition is
	// intentionally left OUT. The ammo > 0 gate still keeps it off every
	// unarmed unit (a Constructor has 0 ammo, so it never shows Manual).
	// C++ ported : units_manager.cpp - UnitsManager_Popup_OnClick_Manual / InitCommons.
	if u.Values != nil && u.Values.Get(game.AttribAmmo) > 0 && !sim.IsCommando(u) {
		items = append(items, popupItem{
			tex:    "MANUAL",
			label:  s.lang.Get("b0b9", "Manual"),
			active: u.DisabledReactionFire, // green highlight when reaction fire is OFF
			action: func() {
				u.DisabledReactionFire = !u.DisabledReactionFire

				// Switching to manual fire releases an on guard order (C++).
				if u.Order == game.OrderSentry {
					u.Order = game.OrderAwait
					u.State = game.OrderStateExecutingOrder
				}
			},
		})
	}

	items = s.appendCargoOrders(u, items, mobile)
	items = s.appendSentryOrders(u, items, mobile)

	// Remove (self destruct): stationary buildings.
	if u.Flags.Has(game.FlagStationary) {
		items = append(items, popupItem{tex: "REMVE", label: s.lang.Get("7e56", "Remove"), action: func() {
			s.orderInput.popupOpen = false
			s.openMenuScreen(func() { s.openSelfDestruct(u) }, false)
		}})
	}

	return items
}

// appendSpecialistOrders adds the orders a unit gets from its role: the mine layer's
// place and remove, a supply truck's Reload, a repair unit's Repair, the bulldozer's
// clear and stop, and a commando's Disable and Steal.
func (s *Screen) appendSpecialistOrders(u *game.UnitInfo, items []popupItem) []popupItem {
	// Lay Mine / Remove Mine: a mine layer (MineLayr/SeaMnLyr) drops a mine on its
	// own cell (store > 0, no mine already there) or picks its own back up (room in
	// store, a mine on the cell). No target pick - it acts on its own cell.
	// C++ ported : units_manager.cpp - UnitsManager_Popup_OnClick_PlaceMine/RemoveMine.
	if sim.IsMineLayer(u) {
		if s.game.CanLayMine(u) {
			items = append(items, popupItem{tex: "LAYMNE", label: s.lang.Get("81df", "Place Mine"), action: func() {
				s.orderInput.popupOpen = false

				if s.game.LayMine(u) {
					s.showMessage("Mine placed.")
				}
			}})
		}

		if s.game.CanRemoveMine(u) {
			items = append(items, popupItem{tex: "RMVMNE", label: s.lang.Get("bfe6", "Remove Mine"), action: func() {
				s.orderInput.popupOpen = false

				if s.game.RemoveMine(u) {
					s.showMessage("Mine removed.")
					s.playVoice(vsRemovingMines) // acknowledge the pickup (laying is acknowledged on select)
				}
			}})
		}
	}

	// Reload: a Supply Truck with material left arms a friendly unit pick (the next
	// click on an adjacent unit with no ammo refills its ammo for one material).
	// C++ ported : units_manager.cpp - UnitsManager_Popup_InitReloaders (Reload order).
	if sim.IsReloader(u) && u.Storage > 0 {
		items = append(items, popupItem{tex: "RELOAD", label: s.lang.Get("9d39", "Reload"), action: func() { s.armReload(u) }})
	}

	// Repair: a mobile Repair unit with material left arms a friendly unit pick (the
	// next click on an adjacent damaged unit mends its hit points for material).
	// C++ ported : units_manager.cpp - UnitsManager_Popup_InitRepair (Repair order).
	if sim.IsRepairer(u) && u.Storage > 0 {
		items = append(items, popupItem{tex: "REPAIR", label: s.lang.Get("a1a1", "Repair"), action: func() { s.armRepair(u) }})
	}

	// Clear / Stop: a Bulldozer parked on a rubble pile clears it (over 1 turn for a
	// small pile, 4 for a building's) to reclaim the ground; while clearing it shows
	// Stop instead. No target pick - it acts on its own cell.
	// C++ ported : units_manager.cpp - UnitsManager_Popup_OnClick_StopRemove (ORDER_CLEAR).
	if sim.IsBulldozer(u) {
		if s.game.IsClearingRubble(u) {
			items = append(items, popupItem{tex: "STOP", label: s.lang.Get("bc20", "Stop"), action: func() {
				s.orderInput.popupOpen = false
				s.game.StopClearRubble(u)
				s.showMessage("Clearing stopped.")
			}})
		} else if s.game.CanClearRubble(u) {
			items = append(items, popupItem{tex: "CLEAR", label: s.lang.Get("9737", "Remove"), action: func() {
				s.orderInput.popupOpen = false

				if s.game.StartClearRubble(u) {
					// The looping working sound (BULL10) is handled by the ambient SFX
					// reconcile while the clearing dozer is selected (ambientSfxTags),
					// so it stops cleanly when the clear ends or the screen is left.
					s.showMessage("Clearing rubble...")
				}
			}})
		}
	}

	// Take Off / Land are NOT popup items - vanilla makes them MOVEMENT driven: clicking a
	// landing pad flies a plane on and lands it automatically; clicking a land cell with a
	// landed plane makes it take off on its own and move. The sound + descend/rise shadow ride the
	// Hovering transition. C++ ported : an air unit's ORDER_MOVE onto/off a pad, not a popup order.

	// Disable / Steal: a commando with a shot left arms an adjacent enemy pick.
	// Disable shuts a unit (or building) down for a few turns; Steal captures an
	// enemy mobile unit. A commando with no shots left omits both.
	// C++ ported : units_manager.cpp - UnitsManager_Popup_InitInfiltrator (Disable/Steal).
	if sim.IsCommando(u) && u.Shots > 0 {
		items = append(items, popupItem{tex: "DISBL", label: s.lang.Get("disable", "Disable"), action: func() { s.armInfiltrate(u, false) }})
		items = append(items, popupItem{tex: "STEAL", label: s.lang.Get("steal", "Steal"), action: func() { s.armInfiltrate(u, true) }})
	}

	return items
}

// appendCargoOrders adds the InitCommons trio that depends on what a unit carries and
// whether it is under way: X-Fer, Enter and Stop.
func (s *Screen) appendCargoOrders(u *game.UnitInfo, items []popupItem, mobile bool) []popupItem {
	// X-Fer: only units/buildings that store a resource (the C++ CargoType
	// raw..gold gate). A tank/scout/turret carries nothing, so no X-Fer. Excluded
	// while the unit is mid build or mid clear (the original's InitCommons drops X-Fer for
	// ORDER_BUILD / ORDER_CLEAR), so a constructor raising a building does not offer
	// it. C++ ported : units_manager.cpp - InitCommons X-Fer order gate.
	if economy.CargoTypeOf(u.Type) != economy.CargoNone && !busyOrder(u) {
		items = append(items, popupItem{tex: "XFER", label: s.lang.Get("886b", "X-Fer"), action: func() { s.armTransfer(u) }})
	}

	// Enter: board a transport OR be stored in a Depot/Hangar/Dock - any mobile unit
	// (the container's class gate decides what it accepts: depot=land, hangar=air,
	// dock=sea). A sea/air transport cannot itself enter another container, but the
	// land APC may (klei ferries a loaded APC on a Sea Transport / stores it in a
	// Depot, access.cpp:2105-2109). Also excluded mid build / mid clear, matching
	// the original's InitCommons.
	if mobile && (!sim.IsTransport(u.Type) || u.Type == assets.ClnTrans) && !busyOrder(u) {
		items = append(items, popupItem{tex: "ENTER", label: s.lang.Get("e08d", "Enter"), action: func() { s.armLoad(u) }})
	}

	// Stop: only while the unit is actually moving (C++: unit->path != null). A build /
	// clear order is not a move order (HasOrder ranges the move list only), so this
	// already stays off during a build - the builder/dozer branches own that Stop.
	if mobile && s.game.HasOrder(u) {
		items = append(items, popupItem{tex: "STOP", label: s.lang.Get("6c45", "Stop"), action: func() { s.game.CancelOrder(u) }})
	}

	return items
}

// appendSentryOrders adds the on guard toggles: Sentry and Done for a mobile unit,
// Sentry alone for a defense building.
func (s *Screen) appendSentryOrders(u *game.UnitInfo, items []popupItem, mobile bool) []popupItem {
	// NOTE: no generic per unit Upgrade button. C++ InitCommons Upgrade applies a PENDING
	// bought upgrade in place, but the port applies upgrades eagerly on purchase, so no unit
	// ever has one pending and the button is always moot. Buy Upgrade lives on the Gold
	// Refinery only (adding it here too showed it twice).
	// Sentry + Done: mobile units (the SENTRY_UNIT flag in the original).
	if mobile {
		items = append(items, popupItem{
			tex:    "SNTRY",
			label:  s.lang.Get("524e", "Sentry"),
			active: u.Order == game.OrderSentry, // green highlight follows the real order
			action: func() { s.toggleSentry(u) },
		})
		items = append(items, popupItem{tex: "DONE", label: s.lang.Get("2b7d", "Done"), action: func() {
			u.Speed = 0 // spend the rest of its turn
			s.game.CancelOrder(u)
			s.clearSelected()
		}})
	}

	// Sentry toggle for defense buildings (any stationary unit with an attack value).
	// Vanilla defaults them to on guard; the popup toggles overwatch off/on. Reuses
	// toggleSentry, safe here: CancelOrder does nothing for a building and clear on move
	// never fires. C++ ported : units_manager.cpp - UnitsManager_Popup_OnClick_Sentry.
	if isDefenseBuilding(u) {
		items = append(items, popupItem{
			tex:    "SNTRY",
			label:  s.lang.Get("524e", "Sentry"),
			active: u.Order == game.OrderSentry, // green highlight when on guard
			action: func() { s.toggleSentry(u) },
		})
	}

	return items
}
