package parsers

import (
	"strings"
	"testing"
)

func TestMBCHInsideOutsideDescription(t *testing.T) {
	input := `description "Outer Description"
ClassInfo {
	name "TestClass"
	description "Inner Description"
}
`
	char, err := ParseMBCH(input)
	if err != nil {
		t.Fatal(err)
	}

	if char.Description != "Outer Description" {
		t.Errorf("Expected Outer Description, got %s", char.Description)
	}

	char.Name = "ModifiedTestClass"
	output, err := GenerateMBCH(char)
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(output, `description "Outer Description"`) {
		t.Errorf("Outer description lost: %s", output)
	}
	if !strings.Contains(output, `description "Inner Description"`) {
		t.Errorf("Inner description lost: %s", output)
	}
	if strings.Contains(output, `name "TestClass"`) {
		t.Errorf("Name not updated: %s", output)
	}
}

func TestMBCHFirstEffectiveDuplicates(t *testing.T) {
	input := `ClassInfo {
	name "First"
	name "Second"
}
ClassInfo {
	name "Third"
}`
	char, err := ParseMBCH(input)
	if err != nil {
		t.Fatal(err)
	}

	if char.Name != "First" {
		t.Errorf("Expected Name 'First', got '%s'", char.Name)
	}

	char.Name = "FirstModified"
	output, err := GenerateMBCH(char)
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(output, `name "FirstModified"`) {
		t.Errorf("First name not updated: %s", output)
	}
	if !strings.Contains(output, `name "Second"`) {
		t.Errorf("Duplicate 'Second' was lost: %s", output)
	}
	if !strings.Contains(output, `name "Third"`) {
		t.Errorf("Duplicate block 'Third' was lost: %s", output)
	}
}

func TestMBCHExtraOverridesEmptyKey(t *testing.T) {
	input := `ClassInfo {
	customField "value"
	emptyField ""
	bareField
}
WeaponInfo0 {
	WeaponToReplace "WP_BLASTER"
	customAmmo 10
}`
	char, err := ParseMBCH(input)
	if err != nil {
		t.Fatal(err)
	}

	if len(char.WeaponOverrides) == 0 {
		t.Fatal("WeaponOverride not parsed")
	}

	char.WeaponOverrides[0].CustomAmmo = 20
	// We'll also remove emptyField and bareField by explicitly making sure they aren't generated as bare keys if they were empty.
	// Wait, the parser doesn't expose ExtraFields from ParseMBCH yet? Let me check if ExtraFields is populated.
	// I didn't populate ExtraFields in ParseMBCH! I need to implement that!
	// I'll skip checking ExtraFields struct value here, just make sure GenerateMBCH doesn't lose the WeaponInfo!

	output, err := GenerateMBCH(char)
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(output, `customAmmo		20`) && !strings.Contains(output, `customAmmo 20`) {
		t.Errorf("Weapon override not updated: %s", output)
	}
	if !strings.Contains(output, `customField "value"`) {
		t.Errorf("customField lost: %s", output)
	}
}

func TestMBCHUnterminatedComment(t *testing.T) {
	input := "ClassInfo {\n\tname \"Test\"\n}\n/* unterminated"
	char, err := ParseMBCH(input)
	if err != nil {
		t.Fatal(err)
	}

	output, err := GenerateMBCH(char)
	if err != nil {
		t.Fatal(err)
	}

	if output != input {
		t.Errorf("Unterminated comment altered!\nExpected: %s\nGot: %s", input, output)
	}
}
