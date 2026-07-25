// live_probe_test.go : provides an opt in check of registry and home directory discovery.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package bootstrap

import (
	"os"
	"testing"
)

// TestLiveProbe runs the real host prober and logs what it finds. It is read
// only and opt in: set MAX_BOOTSTRAP_LIVE=1 to run it (a machine without an
// installed M.A.X. legitimately reports not found, so it never fails).
func TestLiveProbe(t *testing.T) {
	if os.Getenv("MAX_BOOTSTRAP_LIVE") == "" {
		t.Skip("set MAX_BOOTSTRAP_LIVE=1 to probe the real host")
	}

	p := NewProber()
	t.Logf("gog dir:     %q", p.GOGDir())
	t.Logf("steam roots: %q", p.SteamRoots())

	inst, err := Locate(p)
	t.Logf("locate:      %+v err=%v", inst, err)
}
