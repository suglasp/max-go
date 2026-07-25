// options.go : implements the in game options overlay.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package gamescreen

import "github.com/suglasp/max-go/internal/dbg"

// SetOptionsFunc installs the hook opened by O for in-game Preferences. A nil hook leaves O inactive.
func (s *Screen) SetOptionsFunc(fn func() error) {
	s.hooks.optionsFn = fn
}

// openOptions runs the in game Preferences screen via the installed hook.
//
// C++: OptionsMenu(team, PREFSPIC).Run(), opened from the in game menu
// (game_manager.cpp).
func (s *Screen) openOptions() {
	if s.hooks.optionsFn == nil {
		return
	}

	if err := s.hooks.optionsFn(); err != nil {
		dbg.Printf("options: %v", err)
	}
}
