package parsers

import (
	"strings"
	"testing"
)

func TestNoOpLexicalPreservation(t *testing.T) {
	input := `// A test file
saber_a {
	name "Alpha"
	customSkin "red"
	customSkin "blue" // duplicate
}
saber_b {
	name "Beta"
}
`
	saber, err := ParseSABDefinition(input, 0)
	if err != nil {
		t.Fatal(err)
	}

	output, err := GenerateSAB(saber)
	if err != nil {
		t.Fatal(err)
	}

	if output != input {
		t.Errorf("GenerateSAB modified input without edits.\nExpected:\n%s\nGot:\n%s", input, output)
	}
}

func TestEditSelectedDefinition(t *testing.T) {
	input := `saber_a { name "Alpha" }
saber_b { name "Beta" }`

	saber, err := ParseSABDefinition(input, 1)
	if err != nil {
		t.Fatal(err)
	}

	saber.FullName = "Beta Modified"
	output, err := GenerateSAB(saber)
	if err != nil {
		t.Fatal(err)
	}

	// The exact formatting might differ based on setFieldValue, but saber_a MUST remain untouched.
	if !strings.Contains(output, `saber_a { name "Alpha" }`) {
		t.Errorf("saber_a was modified or lost!\nGot:\n%s", output)
	}
	if !strings.Contains(output, `"Beta Modified"`) {
		t.Errorf("saber_b was not modified correctly!\nGot:\n%s", output)
	}
}

func TestUnknownSiblingSurvival(t *testing.T) {
	input := `veh_a { type VH_SPEEDER
	unknown_field "value"
	// a comment
}
veh_b { type VH_FIGHTER }`

	veh, err := ParseVEHDefinition(input, 0)
	if err != nil {
		t.Fatal(err)
	}

	veh.SpeedMax = 100
	output, err := GenerateVEH(veh)
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(output, `unknown_field "value"`) {
		t.Errorf("unknown_field was lost!\nGot:\n%s", output)
	}
	if !strings.Contains(output, `// a comment`) {
		t.Errorf("comment was lost!\nGot:\n%s", output)
	}
	if !strings.Contains(output, `veh_b { type VH_FIGHTER }`) {
		t.Errorf("veh_b was lost!\nGot:\n%s", output)
	}
	if !strings.Contains(output, "speedMax\t\t100.0") {
		t.Errorf("speedMax 100.0 was not added!\nGot:\n%s", output)
	}
}

func TestVEHEngineHandlingKeysParseAndSync(t *testing.T) {
	input := `STAP_theed
{
	name		STAP_theed
	speedMax	450
	turboSpeed	950
	acceleration	20
	decelIdle	10
	mystery		preserve-me
}

wingmate
{
	speedMax	725
	acceleration	15
}`

	veh, err := ParseVEHDefinition(input, 0)
	if err != nil {
		t.Fatal(err)
	}
	if veh.SpeedMax != 450 || veh.Accel != 20 || veh.Decel != 10 {
		t.Fatalf("engine handling keys parsed as speed=%g accel=%g decel=%g, want 450/20/10",
			veh.SpeedMax, veh.Accel, veh.Decel)
	}

	veh.SpeedMax = 475
	veh.Accel = 25
	veh.Decel = 12
	output, err := GenerateVEH(veh)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"speedMax\t475.0", "acceleration\t25.0", "decelIdle\t12.0", "mystery\t\tpreserve-me"} {
		if !strings.Contains(output, want) {
			t.Errorf("generated source missing %q:\n%s", want, output)
		}
	}
	for _, wrong := range []string{"\tspeed\t", "\taccel\t", "\tdecel\t"} {
		if strings.Contains(output, wrong) {
			t.Errorf("generated source contains non-engine key %q:\n%s", wrong, output)
		}
	}
	if !strings.Contains(output, "wingmate\n{\n\tspeedMax\t725\n\tacceleration\t15\n}") {
		t.Errorf("unselected sibling definition changed or was lost:\n%s", output)
	}
}

func TestMalformedSyntax(t *testing.T) {
	// Unterminated quote: the SGPV lexer scans quoted values to the
	// NEXT quote or EOF, across newlines (bg_saga.c:279-295 — multiline
	// quoted values are engine-legal). A file whose quote never closes
	// must still parse best-effort and round-trip byte-exact on a
	// no-op save.
	input := "saber_a {\n\tname \"Alpha\n}\n"
	saber, err := ParseSABDefinition(input, 0)
	if err != nil {
		t.Fatal(err)
	}
	output, err := GenerateSAB(saber)
	if err != nil {
		t.Fatal(err)
	}
	if output != input {
		t.Errorf("Malformed syntax not preserved.\nExpected:\n%q\nGot:\n%q", input, output)
	}
}
