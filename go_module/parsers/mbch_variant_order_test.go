package parsers

import (
	"strings"
	"testing"
)

// TestDrainVariantsOrdering covers bug #4 from the 2026-04-28 tester
// report ("model_1 isn't next to model but somewhere down in the file").
// Asserts that:
//   1. Variant keys appear in numeric order, not Go's randomized map order
//   2. model_N is emitted adjacent to `model` (and same for skin / uishader / saber)
//   3. ExtraFields not in any base group still fall through to
//      writeExtraFields's alphabetical tail
//   4. GenerateMBCH does not mutate the input character — required for the
//      legends round-trip tests, which assume orig stays unchanged
func TestDrainVariantsOrdering(t *testing.T) {
	c := NewMBCHCharacter()
	c.Name = "test"
	c.Model = "reborn"
	c.Skin = "default"
	c.UIShader = "models/players/reborn/mb2_icon_default"
	c.Saber1 = "single_1"
	// Variants injected out of order to exercise sort
	c.ExtraFields["model_2"] = "reborn_dark"
	c.ExtraFields["model_1"] = "reborn_light"
	c.ExtraFields["model_3"] = "reborn_gold"
	c.ExtraFields["skin_2"] = "blue"
	c.ExtraFields["skin_1"] = "red"
	c.ExtraFields["uishader_1"] = "models/players/reborn/mb2_icon_red"
	c.ExtraFields["saber1_1"] = "single_2"
	c.ExtraFields["customred_1"] = "0.5"
	// A non-variant ExtraField — should land in the alphabetical tail
	c.ExtraFields["unrelated_key"] = "value"

	out, err := GenerateMBCH(c)
	if err != nil {
		t.Fatalf("GenerateMBCH: %v", err)
	}

	// (1) + (2): walk output line-by-line and confirm adjacency
	lines := strings.Split(out, "\n")
	idx := make(map[string]int, len(lines))
	for i, ln := range lines {
		// pull the first token
		fields := strings.Fields(ln)
		if len(fields) == 0 {
			continue
		}
		key := fields[0]
		if _, exists := idx[key]; !exists {
			idx[key] = i
		}
	}

	requireBefore := func(early, late string) {
		t.Helper()
		ie, ok1 := idx[early]
		il, ok2 := idx[late]
		if !ok1 || !ok2 {
			t.Errorf("missing key in output: %s=%v %s=%v", early, ok1, late, ok2)
			return
		}
		if ie >= il {
			t.Errorf("expected %s (line %d) before %s (line %d)", early, ie, late, il)
		}
	}

	// model + variants in order
	requireBefore("model", "model_1")
	requireBefore("model_1", "model_2")
	requireBefore("model_2", "model_3")
	// model variants before skin
	requireBefore("model_3", "skin")
	// skin + variants
	requireBefore("skin", "skin_1")
	requireBefore("skin_1", "skin_2")
	// uishader + variants
	requireBefore("uishader", "uishader_1")
	// saber1 + variants
	requireBefore("saber1", "saber1_1")
	// customred variant should appear in its model-adjacent slot
	requireBefore("model_3", "customred_1")

	// (3): unrelated_key should be in the alphabetical tail (after specific blocks)
	if idx["unrelated_key"] < idx["model_3"] {
		t.Errorf("unrelated_key (line %d) should be after model variants (model_3 at %d)",
			idx["unrelated_key"], idx["model_3"])
	}

	// (4): input character must not be mutated
	if _, lost := c.ExtraFields["model_1"]; !lost {
		t.Error("GenerateMBCH mutated input: model_1 was drained from char.ExtraFields")
	}
	if v, ok := c.ExtraFields["unrelated_key"]; !ok || v != "value" {
		t.Error("GenerateMBCH mutated input: unrelated_key changed")
	}
}
