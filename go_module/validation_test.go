package main

import (
	"fmt"
	"strings"
	"testing"

	"github.com/Frenzeh/mbii-foundry/parsers"
)

func TestValidateBlockSizesUsesExactEngineBoundaries(t *testing.T) {
	validator := NewValidator()
	tests := []struct {
		name       string
		atLimit    string
		overLimit  string
		issueToken string
	}{
		{
			name:       "file",
			atLimit:    strings.Repeat("A", parsers.MBCHMaxFileBytes-1),
			overLimit:  strings.Repeat("A", parsers.MBCHMaxFileBytes),
			issueToken: "File exceeds absolute engine capacity",
		},
		{
			name:       "ClassInfo",
			atLimit:    "ClassInfo{" + strings.Repeat("A", parsers.ClassInfoMaxPayload) + "}",
			overLimit:  "ClassInfo{" + strings.Repeat("A", parsers.ClassInfoMaxPayload+1) + "}",
			issueToken: "ClassInfo exceeds engine payload limit",
		},
		{
			name:       "WeaponInfo",
			atLimit:    "WeaponInfo0{" + strings.Repeat("A", parsers.WeaponInfoMaxPayload) + "}",
			overLimit:  "WeaponInfo0{" + strings.Repeat("A", parsers.WeaponInfoMaxPayload+1) + "}",
			issueToken: "WeaponInfo[0] exceeds engine payload limit",
		},
		{
			name:       "ForceInfo",
			atLimit:    "ForceInfo0{" + strings.Repeat("A", parsers.ForceInfoMaxPayload) + "}",
			overLimit:  "ForceInfo0{" + strings.Repeat("A", parsers.ForceInfoMaxPayload+1) + "}",
			issueToken: "ForceInfo[0] exceeds engine payload limit",
		},
		{
			name:       "paired value",
			atLimit:    "ClassInfo{\nkey \"" + strings.Repeat("A", parsers.PairedValueMaxPayload) + "\"\n}",
			overLimit:  "ClassInfo{\nkey \"" + strings.Repeat("A", parsers.PairedValueMaxPayload+1) + "\"\n}",
			issueToken: "Paired value exceeds engine payload limit",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if issueContains(validator.ValidateBlockSizes(test.atLimit), test.issueToken) {
				t.Fatalf("boundary input unexpectedly exceeded %s limit", test.name)
			}
			issues := validator.ValidateBlockSizes(test.overLimit)
			if !issueContains(issues, test.issueToken) {
				t.Fatalf("over-limit input did not report %q: %v", test.issueToken, issues)
			}
		})
	}
}

func TestValidateBlockSizesSurfacesAssessmentDiagnostics(t *testing.T) {
	validator := NewValidator()
	longKey := strings.Repeat("k", parsers.KeyMaxPayload)
	issues := validator.ValidateBlockSizes(fmt.Sprintf("ClassInfo{\n%s value\n}", longKey))
	if !issueContains(issues, fmt.Sprintf("key exceeds %d bytes", parsers.KeyMaxPayload)) {
		t.Fatalf("key diagnostic was not surfaced: %v", issues)
	}

	issues = validator.ValidateBlockSizes("ClassInfo{\nname \"unterminated\n}")
	if !issueContains(issues, "Parser diagnostic:") {
		t.Fatalf("lexical diagnostic was not surfaced: %v", issues)
	}
}

func TestValidateCharacterAvoidsClassAndSubstringHeuristics(t *testing.T) {
	validator := NewValidator()
	char := parsers.NewMBCHCharacter()
	char.Name = "No heuristic warnings"
	char.MBClass = "MB_CLASS_JEDI"
	char.Weapons = "WP_SABER_STAFF"
	char.Attributes = "NOT_MB_ATT_RESOURCE_STABILITY_SUFFIX"

	issues := validator.ValidateCharacter(char)
	for _, issue := range issues {
		if strings.Contains(issue, "Force Users") || strings.Contains(issue, "Saber") || strings.Contains(issue, "Stability") {
			t.Fatalf("unsupported heuristic warning returned: %q", issue)
		}
	}
}

func TestValidateCharacterEnforcesPointbuyLayoutLimits(t *testing.T) {
	validator := NewValidator()
	char := parsers.NewMBCHCharacter()
	char.Name = "Pointbuy"
	char.MBClass = "MB_CLASS_SOLDIER"
	char.HasCustomSpec = parsers.PointbuyMaxArchetypes + 1
	if issues := validator.ValidateCharacter(char); !issueContains(issues, "maximum 3 archetypes") {
		t.Fatalf("archetype limit was not surfaced: %v", issues)
	}

	char.HasCustomSpec = 1
	char.CustomSkills[parsers.PointbuySlotsPerArchetype] = "MB_ATT_HEALTH"
	if issues := validator.ValidateCharacter(char); !issueContains(issues, "maximum 15 slots per archetype, 45 total") {
		t.Fatalf("inactive slot limit was not surfaced: %v", issues)
	}
}

func TestBufferGaugeSurfacesAssessmentDiagnosticsAndExactLimits(t *testing.T) {
	char := parsers.NewMBCHCharacter()
	char.Name = "unterminated\""
	char.MBClass = "MB_CLASS_SOLDIER"

	status := CalculateBufferStatus(char)
	if !status.IsExceeded || len(status.Diagnostics) == 0 || !strings.Contains(status.WarningMsg, "Parser diagnostic:") {
		t.Fatalf("assessment diagnostics were not surfaced: %+v", status)
	}

	label := bufferGaugeLabel(BufferStatus{})
	for _, limit := range []int{
		parsers.MBCHMaxFileBytes - 1,
		parsers.ClassInfoMaxPayload,
		parsers.WeaponInfoMaxPayload,
		parsers.ForceInfoMaxPayload,
		parsers.PairedValueMaxPayload,
		parsers.KeyMaxPayload,
	} {
		if !strings.Contains(label, fmt.Sprint(limit)) {
			t.Errorf("buffer label %q does not surface exact limit %d", label, limit)
		}
	}
}

func TestBufferGaugePreservesAdvisoryThreshold(t *testing.T) {
	char := parsers.NewMBCHCharacter()
	char.Name = "Threshold"
	char.MBClass = "MB_CLASS_SOLDIER"
	char.ExtraFields = map[string]string{
		"large1": strings.Repeat("a", 1800),
		"large2": strings.Repeat("b", 1800),
		"large3": strings.Repeat("c", 1800),
		"large4": strings.Repeat("d", 1800),
	}
	status := CalculateBufferStatus(char)
	if status.IsExceeded {
		t.Fatalf("advisory threshold must not be treated as the engine limit: %+v", status)
	}
	if !status.IsWarning {
		t.Fatalf("ClassInfo at the established advisory threshold should warn: %+v", status)
	}
}

func TestValidateVehicle(t *testing.T) {
	issues := NewValidator().ValidateVehicle(&parsers.VehicleData{Name: "TestVeh"})
	if len(issues) != 1 || !strings.Contains(issues[0], "Vehicle type") {
		t.Fatalf("missing vehicle type issue = %v", issues)
	}
}

func issueContains(issues []string, token string) bool {
	for _, issue := range issues {
		if strings.Contains(issue, token) {
			return true
		}
	}
	return false
}
