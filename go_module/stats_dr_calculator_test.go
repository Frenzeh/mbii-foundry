package main

import (
	"testing"

	"github.com/Frenzeh/mbii-foundry/parsers"
)

func TestCalculateDefensiveMatrixUsesConfiguredPoolsOnly(t *testing.T) {
	char := &parsers.MBCHCharacter{
		MaxHealth:  100,
		MaxArmor:   50,
		MBClass:    "MB_CLASS_SBD",
		Attributes: "MB_ATT_MAGNETIC_PLATING,1|MB_ATT_RESOURCE_STABILITY,3",
		ExtraLives: 2,
	}

	dm := CalculateDefensiveMatrix(char)
	if dm.MaxHealth != 100 || dm.MaxArmor != 50 || dm.TotalLives != 3 {
		t.Fatalf("configured fields were not preserved: %+v", dm)
	}
	if dm.RawPool != 150 || dm.TotalRawEHP != 450 {
		t.Fatalf("configured pool arithmetic = %d/%d, want 150/450", dm.RawPool, dm.TotalRawEHP)
	}
	if dm.HasMagPlating || dm.HasSBDBattery || dm.HasStability {
		t.Fatalf("class/attribute heuristics must remain disabled: %+v", dm)
	}
	if dm.EnergyEHP != dm.TotalRawEHP || dm.ExplosiveEHP != dm.TotalRawEHP || dm.MeleeEHP != dm.TotalRawEHP {
		t.Fatalf("unverified channel values must not invent damage reduction: %+v", dm)
	}
	if dm.EvidenceStatus != "Unverified" {
		t.Fatalf("evidence status = %q, want Unverified", dm.EvidenceStatus)
	}
}

func TestCalculateDefensiveMatrixDoesNotInventHealthDefault(t *testing.T) {
	dm := CalculateDefensiveMatrix(&parsers.MBCHCharacter{})
	if dm.MaxHealth != 0 || dm.RawPool != 0 || dm.TotalRawEHP != 0 {
		t.Fatalf("zero configured pools must remain zero, got %+v", dm)
	}
}

func TestCalculateDefensiveMatrixPreservesExtraLivesFloor(t *testing.T) {
	dm := CalculateDefensiveMatrix(&parsers.MBCHCharacter{MaxHealth: 100, ExtraLives: -1})
	if dm.ExtraLives != 0 || dm.TotalLives != 1 || dm.TotalRawEHP != 100 {
		t.Fatalf("negative extralives must retain the established zero floor: %+v", dm)
	}
}
