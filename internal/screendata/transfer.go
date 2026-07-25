// transfer.go : carries the X-Fer screen data transfer object.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
// C++ ported : transfermenu.cpp - TransferMenu_GetUnitCargoInfo
package screendata

import (
	"github.com/suglasp/max-go/internal/economy"
	"github.com/suglasp/max-go/internal/gfx"
)

// TransferInfo carries the two units' pooled materials and capacity for the X-Fer
// dialog. The host resolves the complex pooling (TransferMenu_GetUnitCargoInfo)
// before handing it over, so the menus package stays free of the build network
// dependency.
type TransferInfo struct {
	SourceName, TargetName          string
	SourceMaterials, SourceCapacity int
	TargetMaterials, TargetCapacity int
	Cargo                           economy.CargoType
	SourceIcon, TargetIcon          *gfx.Buffer // 32x32 team coloured unit icons, or nil
}
