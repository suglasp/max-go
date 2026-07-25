// main_test.go : checks that max-settings exposes every configurable field.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package main

import (
	"reflect"
	"strings"
	"testing"

	"github.com/suglasp/max-go/internal/settings"
)

// settingsLeafNames returns the JSON path of every scalar leaf in Settings.
// Keybind rows come from the loaded map rather than the struct shape, so this
// walk skips them.
func settingsLeafNames() []string {
	var out []string

	var walk func(prefix string, rt reflect.Type)
	walk = func(prefix string, rt reflect.Type) {
		for i := 0; i < rt.NumField(); i++ {
			sf := rt.Field(i)
			tag := sf.Tag.Get("json")
			if tag == "" || tag == "-" {
				continue // unexported / untagged, e.g. the -datadir override
			}

			name := strings.Split(tag, ",")[0]
			if prefix != "" {
				name = prefix + "." + name
			}

			switch sf.Type.Kind() {
			case reflect.Struct:
				walk(name, sf.Type)
			case reflect.Map:
				// The loaded file supplies keybind rows dynamically.
			default:
				out = append(out, name)
			}
		}
	}

	walk("", reflect.TypeOf(settings.Settings{}))

	return out
}

// TestSettingFieldsCoverAllSettings requires an editor row for every setting.
func TestSettingFieldsCoverAllSettings(t *testing.T) {
	have := map[string]bool{}
	for _, f := range settingFields(settings.Default()) {
		have[f.name] = true
	}

	for _, want := range settingsLeafNames() {
		if !have[want] {
			t.Errorf("setting %q has no max-settings field row", want)
		}
	}
}
