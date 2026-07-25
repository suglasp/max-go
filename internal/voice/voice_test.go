// voice_test.go : testing unit for voice.go
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package voice

import "testing"

func TestVoiceGroups(t *testing.T) {
	tests := []struct {
		name string
		got  []string
		want []string
	}{
		{"select", Select(), []string{"V_F001", "V_F004", "V_F005", "V_F006"}},
		{"select sentry", SelectSentry(), []string{"V_F001", "V_F004", "V_F005", "V_F006", "V_F158"}},
		{"move ack", MoveAck(), []string{"V_F001", "V_F004", "V_F005", "V_F006"}},
		{"combat", Combat(), []string{"V_F196", "V_F198"}},
		{"reaction fire", ReactionFire(), []string{"V_F250", "V_F251"}},
		{"status yellow", StatusYellow(), []string{"V_F150", "V_F151"}},
		{"status red", StatusRed(), []string{"V_F154", "V_F155"}},
		{"under attack", UnderAttack(), []string{"V_F229", "V_F230", "V_F231", "V_F232", "V_F255", "V_F256"}},
		{"unit destroyed", UnitDestroyed(), []string{"V_F234", "V_F236"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if len(tt.got) != len(tt.want) {
				t.Fatalf("len = %d, want %d (%v)", len(tt.got), len(tt.want), tt.got)
			}
			for i := range tt.want {
				if tt.got[i] != tt.want[i] {
					t.Fatalf("[%d] = %q, want %q (all=%v)", i, tt.got[i], tt.want[i], tt.got)
				}
			}
		})
	}
}

func TestVoiceGroupsReturnCopies(t *testing.T) {
	got := Select()
	got[0] = "changed"
	if again := Select()[0]; again != SelectReadyA {
		t.Fatalf("Select()[0] = %q after caller mutation, want %q", again, SelectReadyA)
	}

	got = SelectSentry()
	got[len(got)-1] = "changed"
	if again := SelectSentry()[len(SelectSentry())-1]; again != OnSentry {
		t.Fatalf("SelectSentry last = %q after caller mutation, want %q", again, OnSentry)
	}
}
