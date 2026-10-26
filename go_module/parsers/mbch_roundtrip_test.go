package parsers

import (
	"strings"
	"testing"
)

// sampleMBCH is a minimal but realistic character file used for self-contained
// round-trip tests. Kept in-code so tests don't depend on an external
// TextAssets checkout (unlike TestParseRealFiles).
const sampleMBCH = `// Siege class def file.

ClassInfo
{
	name			"Test Jedi"
	weapons			WP_SABER|WP_MELEE
	saber1			single_1
	sabercolor		1
	saberstyle		SS_MEDIUM|SS_STRONG
	attributes		MB_ATT_SABER_OFFENSE|MB_ATT_SABER_DEFENSE|MB_ATT_DODGE
	forcepowers		FP_PUSH|FP_PULL|FP_HEAL
	forcepool		100
	maxhealth		100
	maxarmor		0
	model			"kyle"
	skin			"default"
	uishader		"models/players/kyle/mb2_icon_default"
	MBClass			MB_CLASS_JEDI
}

description	"A test Jedi for round-trip parsing."
`

func TestParseMBCHBasic(t *testing.T) {
	char, err := ParseMBCH(sampleMBCH)
	if err != nil {
		t.Fatalf("ParseMBCH failed: %v", err)
	}
	if char == nil {
		t.Fatal("ParseMBCH returned nil character")
	}
}

func TestGenerateMBCH(t *testing.T) {
	char, err := ParseMBCH(sampleMBCH)
	if err != nil {
		t.Fatalf("ParseMBCH failed: %v", err)
	}
	out, err := GenerateMBCH(char)
	if err != nil {
		t.Fatalf("GenerateMBCH failed: %v", err)
	}
	if out == "" {
		t.Fatal("GenerateMBCH returned empty string")
	}
	// The generated output must contain the canonical ClassInfo marker so
	// the game can find it.
	if !strings.Contains(out, "ClassInfo") {
		t.Errorf("generated MBCH missing ClassInfo block; got:\n%s", out)
	}
}

func TestRoundTripPreservesClassName(t *testing.T) {
	// A minimal invariant: the class name survives a parse -> generate cycle.
	// Many other fields are lossy in the current parser (known; tracked as
	// future work in docs/ROADMAP.md), but the class name must persist.
	char, err := ParseMBCH(sampleMBCH)
	if err != nil {
		t.Fatalf("ParseMBCH failed: %v", err)
	}
	out, err := GenerateMBCH(char)
	if err != nil {
		t.Fatalf("GenerateMBCH failed: %v", err)
	}
	char2, err := ParseMBCH(out)
	if err != nil {
		t.Fatalf("re-parse failed: %v\ngenerated output was:\n%s", err, out)
	}
	if char.Name != char2.Name {
		t.Errorf("name not preserved: original=%q, after round-trip=%q", char.Name, char2.Name)
	}
}

func TestParseMBCHEmpty(t *testing.T) {
	// Parsing an empty file shouldn't crash. It may return an empty
	// character or an error — both are acceptable; the test just guards
	// against a panic.
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("ParseMBCH panicked on empty input: %v", r)
		}
	}()
	_, _ = ParseMBCH("")
}

func TestParseMBCHMalformed(t *testing.T) {
	// Missing closing brace — parser should return an error, not panic.
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("ParseMBCH panicked on malformed input: %v", r)
		}
	}()
	_, _ = ParseMBCH(`ClassInfo {
	name	"Broken"
`)
}
