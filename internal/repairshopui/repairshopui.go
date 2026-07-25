// repairshopui.go : the shared Repair Shop screen contract (DTOs + backend interface).
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
//
// Package repairshopui defines the contract between the menu that draws the Repair Shop and the gamescreen that supplies simulation state and operations.
// Both packages alias these types, avoiding a dependency between them or a field-by-field adapter.
//
// C++ ported : repairshopmenu.cpp - RepairShopMenu (the screen's data contract).
package repairshopui

import "github.com/suglasp/max-go/internal/gfx"

// RepairUnitInfo contains the display values and available operations for one serviced unit.
type RepairUnitInfo struct {
	Name      string
	Portrait  *gfx.Buffer // Prerendered team-colored icon, or nil.
	Hits      int
	MaxHits   int
	Ammo      int
	MaxAmmo   int // Zero for a unit without an attack.
	Cargo     int
	MaxCargo  int
	ShowCargo bool // Draw cargo instead of ammo.

	CanActivate bool
	CanReload   bool
	CanRepair   bool
	CanUpgrade  bool
}

// RepairShopInfo is the screen snapshot returned after each operation so material counts and available actions stay current.
type RepairShopInfo struct {
	IsHangar        bool // Use the four-slot 2x2 layout.
	IsShop          bool // Show per-slot service actions.
	ShowMaterials   bool // Show the material bar and bulk actions.
	ShowActivateAll bool // Hide for AIRTRANS.
	RawMaterials    int  // Current complex raw material.
	RawCapacity     int  // Material-bar scale captured on entry.
	Units           []RepairUnitInfo
}

// RepairShopOps applies menu actions to simulation state. Indices refer to the current snapshot's Units slice.
// Activate remains on screen because the simulation has no stored-bay model for vanilla's deploy-and-close behavior.
type RepairShopOps interface {
	Snapshot() RepairShopInfo
	Activate(i int)
	Reload(i int)
	Repair(i int)
	Upgrade(i int)
	ActivateAll()
	ReloadAll()
	RepairAll()
	UpgradeAll()
}
