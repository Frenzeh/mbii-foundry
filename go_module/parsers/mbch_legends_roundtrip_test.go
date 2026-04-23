package parsers

// Legends round-trip regression tests.
//
// These tests load real MBCH files from the user's local TextAssets
// checkout, parse → generate → re-parse, and assert that every
// significant field survives the round trip. The goal isn't byte-
// exact output (formatting drift is fine) — it's data fidelity: if
// a tester edits a file in Foundry, saves it, reloads it, they
// should see exactly the same loadout.
//
// Files are looked up relative to the user's standard gamedata
// location. The test skips (not fails) when the files aren't
// present, so CI on a clean checkout still passes.

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
)

// legendsCharRoot is the filesystem path the tests check for real
// MBCH files. Skips gracefully on machines that don't have MBII
// installed at this path (CI, fresh clones, etc.).
const legendsCharRoot = "/Users/pj/Library/CloudStorage/SynologyDrive-mcp5/mbii/TextAssets/z_MBLegends/ext_data/mb2/character"

// roundTripCases lists legends files chosen to exercise the parser
// across its complexity spectrum:
//   - monkey lizard: simple + uses c_att_descs_N (description
//     fields per point-buy slot).
//   - h10_Obi: force-user + hasCustomSpec 3 (archetype system).
//   - h3_CloneCom: hasCustomSpec 3 with 45+ c_att_skill slots.
//   - h1_Talz: custom weapons with HELD_* flags.
//   - h1_Rebel: 20-variant skin list with RGB overrides.
var roundTripCases = []string{
	"v2_KMonkeyL.mbch",
	"h10_Obi.mbch",
	"h3_CloneCom.mbch",
	"h1_Talz.mbch",
	"h1_Rebel.mbch",
}

func TestRoundTripLegendsCharacters(t *testing.T) {
	if _, err := os.Stat(legendsCharRoot); err != nil {
		t.Skipf("legends character root not present (%v) — skipping round-trip test", err)
	}

	for _, fname := range roundTripCases {
		fname := fname
		t.Run(fname, func(t *testing.T) {
			path := filepath.Join(legendsCharRoot, fname)
			data, err := os.ReadFile(path)
			if err != nil {
				t.Skipf("%s not readable: %v", fname, err)
				return
			}

			orig, err := ParseMBCH(string(data))
			if err != nil {
				t.Fatalf("first parse failed: %v", err)
			}

			regenerated, err := GenerateMBCH(orig)
			if err != nil {
				t.Fatalf("generate failed: %v", err)
			}

			reparsed, err := ParseMBCH(regenerated)
			if err != nil {
				t.Fatalf("re-parse failed: %v\nregenerated output:\n%s", err, regenerated)
			}

			diffs := diffMBCH(orig, reparsed)
			if len(diffs) > 0 {
				t.Errorf("%s round-trip lost %d field(s):\n  %s",
					fname, len(diffs), strings.Join(diffs, "\n  "))
			}
		})
	}
}

// diffMBCH reports every field whose value differs between two
// MBCHCharacter instances — returns a list of human-readable
// differences, one per field. Fields we consider insignificant
// (auto-generated ExtraFields ordering, comment-only formatting)
// are excluded from the comparison; see skippedExtraField below.
func diffMBCH(a, b *MBCHCharacter) []string {
	var diffs []string

	// Scalar struct fields — use reflection so we don't miss new
	// additions as the struct grows.
	va := reflect.ValueOf(*a)
	vb := reflect.ValueOf(*b)
	ta := va.Type()
	for i := 0; i < va.NumField(); i++ {
		name := ta.Field(i).Name
		// Skip internal maps (diffed separately) and anything that
		// doesn't round-trip by design (see note on ExtraFields).
		switch name {
		case "ExtraFields", "RankAttributes",
			"CustomSkills", "CustomNames", "CustomRanks", "CustomDescs",
			"CustomSpecNames", "CustomSpecIcons",
			"WeaponOverrides", "ForceOverrides":
			continue
		}
		av := va.Field(i).Interface()
		bv := vb.Field(i).Interface()
		if !reflect.DeepEqual(av, bv) {
			diffs = append(diffs, fmt.Sprintf("%s: %v → %v", name, av, bv))
		}
	}

	// Slot arrays.
	for i := 0; i < 45; i++ {
		if a.CustomSkills[i] != b.CustomSkills[i] {
			diffs = append(diffs, fmt.Sprintf("CustomSkills[%d]: %q → %q", i, a.CustomSkills[i], b.CustomSkills[i]))
		}
		if a.CustomNames[i] != b.CustomNames[i] {
			diffs = append(diffs, fmt.Sprintf("CustomNames[%d]: %q → %q", i, a.CustomNames[i], b.CustomNames[i]))
		}
		if a.CustomRanks[i] != b.CustomRanks[i] {
			diffs = append(diffs, fmt.Sprintf("CustomRanks[%d]: %q → %q", i, a.CustomRanks[i], b.CustomRanks[i]))
		}
		if a.CustomDescs[i] != b.CustomDescs[i] {
			diffs = append(diffs, fmt.Sprintf("CustomDescs[%d]: %q → %q", i, a.CustomDescs[i], b.CustomDescs[i]))
		}
	}
	for i := 0; i < 3; i++ {
		if a.CustomSpecNames[i] != b.CustomSpecNames[i] {
			diffs = append(diffs, fmt.Sprintf("CustomSpecNames[%d]: %q → %q", i, a.CustomSpecNames[i], b.CustomSpecNames[i]))
		}
		if a.CustomSpecIcons[i] != b.CustomSpecIcons[i] {
			diffs = append(diffs, fmt.Sprintf("CustomSpecIcons[%d]: %q → %q", i, a.CustomSpecIcons[i], b.CustomSpecIcons[i]))
		}
	}

	// Maps.
	diffs = append(diffs, diffStringMap("RankAttributes", a.RankAttributes, b.RankAttributes)...)
	diffs = append(diffs, diffExtraFields(a.ExtraFields, b.ExtraFields)...)

	return diffs
}

// diffStringMap compares two string→string maps, returning human-
// readable diff strings. Used for RankAttributes (unambiguous set).
func diffStringMap(label string, a, b map[string]string) []string {
	var diffs []string
	keys := map[string]bool{}
	for k := range a {
		keys[k] = true
	}
	for k := range b {
		keys[k] = true
	}
	sorted := make([]string, 0, len(keys))
	for k := range keys {
		sorted = append(sorted, k)
	}
	sort.Strings(sorted)
	for _, k := range sorted {
		if a[k] != b[k] {
			diffs = append(diffs, fmt.Sprintf("%s[%s]: %q → %q", label, k, a[k], b[k]))
		}
	}
	return diffs
}

// diffExtraFields skips fields known to change representation
// intentionally (quoted/unquoted, whitespace normalization, etc.)
// — those aren't round-trip losses worth surfacing.
func diffExtraFields(a, b map[string]string) []string {
	var diffs []string
	keys := map[string]bool{}
	for k := range a {
		if !skippedExtraField(k) {
			keys[k] = true
		}
	}
	for k := range b {
		if !skippedExtraField(k) {
			keys[k] = true
		}
	}
	sorted := make([]string, 0, len(keys))
	for k := range keys {
		sorted = append(sorted, k)
	}
	sort.Strings(sorted)
	for _, k := range sorted {
		if normalizeExtra(a[k]) != normalizeExtra(b[k]) {
			diffs = append(diffs, fmt.Sprintf("ExtraFields[%s]: %q → %q", k, a[k], b[k]))
		}
	}
	return diffs
}

// skippedExtraField is the escape hatch for fields where round-trip
// difference is expected (comment-only, generated during save, etc).
// Currently empty — everything is expected to round-trip. Entries
// added here should note WHY.
func skippedExtraField(key string) bool {
	_ = key
	return false
}

// normalizeExtra tightens whitespace on values so quote/whitespace
// drift doesn't flag as a real diff. MBCH values are single-line
// strings; trimming + collapsing internal runs keeps the comparison
// meaningful.
func normalizeExtra(s string) string {
	s = strings.TrimSpace(s)
	// Collapse consecutive whitespace runs to a single space. Values
	// like descriptions often have tabs the generator normalizes to
	// spaces; we don't want that to count as a diff.
	fields := strings.Fields(s)
	return strings.Join(fields, " ")
}
