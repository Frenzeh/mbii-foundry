package main

import (
	"testing"

	"github.com/Frenzeh/mbii-foundry/parsers"
)

func TestPointbuySlotModesUseExactTokens(t *testing.T) {
	if got := detectSlotMode("MB_ATT_INVALID", "-1"); got != "Header" {
		t.Fatalf("exact header token mode = %q, want Header", got)
	}
	if got := detectSlotMode("PREFIX_MB_ATT_INVALID", "-1"); got != "Skill" {
		t.Fatalf("lookalike attribute must not be treated as header, got %q", got)
	}
	if got := detectSlotMode("", "-1"); got != "Empty" {
		t.Fatalf("empty skill mode = %q, want Empty", got)
	}
}

func TestPointbuySharedLayoutLimits(t *testing.T) {
	if parsers.PointbuyMaxTotalSlots != parsers.PointbuyMaxArchetypes*parsers.PointbuySlotsPerArchetype {
		t.Fatalf("total slot limit %d is inconsistent with %d archetypes * %d slots",
			parsers.PointbuyMaxTotalSlots,
			parsers.PointbuyMaxArchetypes,
			parsers.PointbuySlotsPerArchetype,
		)
	}
}
