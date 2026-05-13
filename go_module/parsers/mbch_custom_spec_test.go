package parsers

import "testing"

// TestCustomSpecExtendedRoundTrip pins the three custom-build fields
// promoted from ExtraFields catch-all to first-class struct slots:
//   * IsOnlyOneSpec   (bg_saga.c:2367)
//   * DefaultSpec     (bg_saga.c:2370)
//   * CustomSpecDescs (bg_saga.c:2375)
//
// Before promotion, CustomSpecDescs was dropped silently and the other
// two leaked into the alphabetical ExtraFields tail. The round-trip
// here is the regression guard.
func TestCustomSpecExtendedRoundTrip(t *testing.T) {
	in := NewMBCHCharacter()
	in.Name = "h_test"
	in.IsCustomBuild = 1
	in.MBPoints = 60
	in.HasCustomSpec = 3
	in.IsOnlyOneSpec = 1
	in.DefaultSpec = 2
	in.CustomSpecNames = [3]string{"Gunner", "Saberist", "Medic"}
	in.CustomSpecIcons = [3]string{"gfx/icon_gun", "gfx/icon_saber", "gfx/icon_medic"}
	in.CustomSpecDescs = [3]string{"Ranged DPS", "Saber duelist", "Battlefield support"}

	out, err := GenerateMBCH(in)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}

	back, err := ParseMBCH(out)
	if err != nil {
		t.Fatalf("re-parse: %v", err)
	}

	if back.IsOnlyOneSpec != 1 {
		t.Errorf("IsOnlyOneSpec lost: got %d, want 1", back.IsOnlyOneSpec)
	}
	if back.DefaultSpec != 2 {
		t.Errorf("DefaultSpec lost: got %d, want 2", back.DefaultSpec)
	}
	for i, want := range in.CustomSpecNames {
		if back.CustomSpecNames[i] != want {
			t.Errorf("CustomSpecNames[%d]: got %q, want %q (spec_3 used to silently fall through to ExtraFields when bounds were idx<3)", i, back.CustomSpecNames[i], want)
		}
	}
	for i, want := range in.CustomSpecIcons {
		if back.CustomSpecIcons[i] != want {
			t.Errorf("CustomSpecIcons[%d]: got %q, want %q", i, back.CustomSpecIcons[i], want)
		}
	}
	for i, want := range in.CustomSpecDescs {
		if back.CustomSpecDescs[i] != want {
			t.Errorf("CustomSpecDescs[%d]: got %q, want %q", i, back.CustomSpecDescs[i], want)
		}
	}

	// Also make sure none of these landed in ExtraFields — that would
	// mean the parser hit the default case instead of the dedicated
	// branches and the writer would emit them twice.
	for _, k := range []string{"isOnlyOneSpec", "defaultSpec",
		"customSpecName_1", "customSpecName_2", "customSpecName_3",
		"customSpecIcon_1", "customSpecIcon_2", "customSpecIcon_3",
		"customSpecDesc_1", "customSpecDesc_2", "customSpecDesc_3"} {
		if v, leaked := back.ExtraFields[k]; leaked {
			t.Errorf("%s leaked to ExtraFields with value %q — promotion regression", k, v)
		}
	}
}
