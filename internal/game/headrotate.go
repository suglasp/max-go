// headrotate.go : identifies units with independently rotating turrets.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
// C++ ported : unitinfo.cpp - turret angle update
package game

import "github.com/suglasp/max-go/internal/assets"

// CanHeadRotateType reports whether a unit type has an independently rotating head or gun layer.
// Other armed units aim by turning their whole body.
func CanHeadRotateType(t assets.UnitType) bool {
	switch t {
	case assets.Tank, assets.GunTurrt, assets.AntiAir, assets.ArtyTrrt, assets.AntiMssl:
		return true
	}
	return false
}
