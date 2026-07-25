// flow.go : coordinates transitions among the top-level menu screens.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package menus

import (
	"github.com/suglasp/max-go/internal/dbg"
	"github.com/suglasp/max-go/internal/game"
	"github.com/suglasp/max-go/internal/settings"
)

// CustomGameFlow runs game options, planet selection, and player or clan assignment for a custom or hot-seat game. Cancelling a screen returns to the previous one.
// It returns assembled StartParams, or nil when the player backs out entirely, and saves confirmed options into cfg.
//
// C++: menu_custom_game_menu (menu.cpp)
func CustomGameFlow(ctx *Context, cfg *settings.Settings, isSinglePlayer bool) (*game.StartParams, error) {
	params := &game.StartParams{}

	enterOptions := true
	enterPlanet := true

	for {

		if enterOptions {
			done, err := GameConfig(ctx, &cfg.Game, GameConfigModeCustom)
			if err != nil {
				return nil, err
			}

			if !done {
				return nil, nil
			}

			enterOptions = false
		}

		if enterPlanet {
			world, err := PlanetSelect(ctx, cfg.Game.World)

			if err != nil {
				return nil, err
			}

			if world < 0 {
				enterOptions = true

				continue
			}

			cfg.Game.World = world
			enterPlanet = false
		}

		// Every team starts from the player's default clan.
		// C++: menu_custom_game_menu clan reset
		for i := range params.Teams {
			params.Teams[i].Clan = cfg.Player.Clan
		}

		ok, err := ChoosePlayer(ctx, params, isSinglePlayer, cfg.Player.Name)
		if err != nil {
			return nil, err
		}

		if !ok {
			enterPlanet = true
			continue
		}

		params.World = cfg.Game.World
		params.Options = cfg.Game

		for i, t := range params.Teams {
			dbg.Printf("team %d: %s clan=%d name=%q", i, t.Type, t.Clan, t.Name)
		}

		return params, nil
	}
}
