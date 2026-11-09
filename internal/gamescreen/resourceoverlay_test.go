// resourceoverlay_test.go : tests resourceoverlay.go.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package gamescreen

import (
	"testing"

	"github.com/suglasp/max-go/internal/assets"
	"github.com/suglasp/max-go/internal/economy"
	"github.com/suglasp/max-go/internal/game"
)

func TestSurveyTickResourceSurveyUnits(t *testing.T) {
	cases := []struct {
		name       string
		unitType   assets.UnitType
		wantSurvey bool
	}{
		{name: "surveyor", unitType: assets.Surveyor, wantSurvey: true},
		{name: "mine layer", unitType: assets.MineLayr, wantSurvey: true},
		{name: "sea mine layer", unitType: assets.SeaMnLyr, wantSurvey: true},
		{name: "commando", unitType: assets.Commando, wantSurvey: true},
		{name: "scout", unitType: assets.Scout, wantSurvey: false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s, g := newTestScreen(t, 16)
			g.Resources = economy.NewResourceMap(g.W, g.H)
			mkUnit(g, tc.unitType, 0, 1, 5, 5, game.FlagMobileLandUnit|game.FlagSelectable)

			s.surveyTick()

			if got := g.Resources.IsSurveyed(5, 5, 0); got != tc.wantSurvey {
				t.Fatalf("center surveyed = %v, want %v", got, tc.wantSurvey)
			}
			if got := g.Resources.IsSurveyed(4, 4, 0); got != tc.wantSurvey {
				t.Fatalf("neighbour surveyed = %v, want %v", got, tc.wantSurvey)
			}
			if g.Resources.IsSurveyed(3, 3, 0) {
				t.Fatal("surveyTick marked outside the radius-1 survey area")
			}
		})
	}
}
