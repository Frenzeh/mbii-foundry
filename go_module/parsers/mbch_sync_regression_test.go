package parsers

import (
	"strings"
	"testing"
)

// Deleting an override in the model removes its WeaponInfo block; the
// surviving block keeps its ORIGINAL numbering (never renumbered) and
// the gap is diagnosed.
func TestMBCHOverrideDeletionKeepsIdentity(t *testing.T) {
	input := `ClassInfo
{
	name	"GapTest"
}

WeaponInfo0
{
	WeaponToReplace		WP_BLASTER
}

WeaponInfo1
{
	WeaponToReplace		WP_BOWCASTER
}
`
	char, err := ParseMBCH(input)
	if err != nil {
		t.Fatal(err)
	}
	if len(char.WeaponOverrides) != 2 {
		t.Fatalf("expected 2 overrides, got %d", len(char.WeaponOverrides))
	}
	// Delete the first override.
	char.WeaponOverrides = char.WeaponOverrides[1:]

	out, err := GenerateMBCH(char)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out, "WP_BLASTER") {
		t.Errorf("deleted override block survived:\n%s", out)
	}
	if !strings.Contains(out, "WeaponInfo1") {
		t.Errorf("surviving override lost its original identity:\n%s", out)
	}
	if !strings.Contains(out, "WP_BOWCASTER") {
		t.Errorf("surviving override content lost:\n%s", out)
	}
	foundDiag := false
	for _, d := range char.Diags {
		if strings.Contains(d, "weaponinfo0 missing") {
			foundDiag = true
		}
	}
	if !foundDiag {
		t.Errorf("missing non-contiguity diagnostic: %v", char.Diags)
	}
}

// A freshly added override takes an index after all surviving blocks so
// deletions + additions stay gap-free.
func TestMBCHOverrideAdditionAppendsAfterMax(t *testing.T) {
	input := "ClassInfo {\n\tname \"AddTest\"\n}\nWeaponInfo2 {\n\tWeaponToReplace WP_BLASTER\n}\n"
	char, err := ParseMBCH(input)
	if err != nil {
		t.Fatal(err)
	}
	char.WeaponOverrides = append(char.WeaponOverrides, WeaponInfo{
		WeaponToReplace: "WP_BOWCASTER",
	})
	out, err := GenerateMBCH(char)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "WeaponInfo3") {
		t.Errorf("new override did not append after max existing index:\n%s", out)
	}
	// Regeneration must be stable: second generate is a fixed point.
	out2, err := GenerateMBCH(char)
	if err != nil {
		t.Fatal(err)
	}
	if out != out2 {
		t.Errorf("GenerateMBCH not idempotent:\n--- first ---\n%s\n--- second ---\n%s", out, out2)
	}
}

// Deleting an extra field or rank attribute removes its line.
func TestMBCHExtraAndRankDeletion(t *testing.T) {
	input := `ClassInfo {
	name	"DelTest"
	customField "keepme"
	goneField "value"
	rank_A	1
	rank_B	2
}`
	char, err := ParseMBCH(input)
	if err != nil {
		t.Fatal(err)
	}
	delete(char.ExtraFields, "goneField")
	delete(char.RankAttributes, "rank_B")

	out, err := GenerateMBCH(char)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out, "goneField") || strings.Contains(out, "rank_B") {
		t.Errorf("deleted fields survived:\n%s", out)
	}
	for _, want := range []string{"customField \"keepme\"", "rank_A\t1", "name"} {
		if !strings.Contains(out, want) {
			t.Errorf("kept field lost: %q in\n%s", want, out)
		}
	}
}

// GenerateMBCH must not mutate the retained AST baseline: repeated
// generation from the same model is stable and a deletion followed by a
// regenerate from the ORIGINAL model still yields the original file.
func TestGenerateMBCHDoesNotMutateBaseline(t *testing.T) {
	input := `ClassInfo
{
	name	"Pure"
	weaponfield	"x"
}

WeaponInfo0
{
	WeaponToReplace		WP_BLASTER
	customAmmo	10
}
`
	char, err := ParseMBCH(input)
	if err != nil {
		t.Fatal(err)
	}

	first, err := GenerateMBCH(char)
	if err != nil {
		t.Fatal(err)
	}
	if first != input {
		t.Fatalf("no-op generation changed the file:\n--- want ---\n%s\n--- got ---\n%s", input, first)
	}

	// Pure model churn: the AST baseline never learns about intermediate
	// states, so undoing the model edit restores the original bytes.
	char.ExtraFields["weaponfield"] = "changed"
	changed, err := GenerateMBCH(char)
	if !strings.Contains(changed, "changed") || strings.Contains(changed, `"changed"`) {
		t.Fatalf("model edit not rendered as unquoted value:\n%s", changed)
	}
	delete(char.ExtraFields, "weaponfield")
	withoutField, err := GenerateMBCH(char)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(withoutField, "weaponfield") {
		t.Fatalf("removed field survived:\n%s", withoutField)
	}
	char.ExtraFields["weaponfield"] = "x"
	restored, err := GenerateMBCH(char)
	if err != nil {
		t.Fatal(err)
	}
	if restored != input {
		t.Errorf("baseline was mutated by intermediate generations:\n--- want ---\n%s\n--- got ---\n%s", input, restored)
	}

	// Deleting and re-adding an override loses its AST identity (the
	// model holds no name), so the re-created block uses canonical
	// formatting — but the content must be correct and generation must
	// be idempotent from there on.
	char.WeaponOverrides = nil
	deleted, err := GenerateMBCH(char)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(deleted, "WeaponInfo0") {
		t.Fatalf("deletion not applied:\n%s", deleted)
	}
	char.WeaponOverrides = append(char.WeaponOverrides, WeaponInfo{
		WeaponToReplace: "WP_BLASTER",
		CustomAmmo:      10,
	})
	recreated, err := GenerateMBCH(char)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(recreated, "WeaponInfo0") || !strings.Contains(recreated, "WP_BLASTER") {
		t.Errorf("re-created override wrong:\n%s", recreated)
	}
	again, err := GenerateMBCH(char)
	if err != nil {
		t.Fatal(err)
	}
	if recreated != again {
		t.Errorf("generation not idempotent after re-creation:\n--- a ---\n%s\n--- b ---\n%s", recreated, again)
	}
}

// A description spanning multiple lines survives parse → generate →
// re-parse with its interior intact and never comes back doubled.
func TestMBCHMultilineDescriptionRoundTrip(t *testing.T) {
	input := "ClassInfo {\n\tname \"M\"\n}\n" +
		"description \"Line one\nLine two\n\tLine three\"\n"
	char, err := ParseMBCH(input)
	if err != nil {
		t.Fatal(err)
	}
	want := "Line one\nLine two\n\tLine three"
	if char.Description != want {
		t.Fatalf("multiline description mangled on parse: %q", char.Description)
	}
	out, err := GenerateMBCH(char)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(out, `description`) != 1 {
		t.Errorf("description duplicated:\n%s", out)
	}
	if !strings.Contains(out, `description "Line one\nLine two`) && !strings.Contains(out, "Line one\nLine two") {
		t.Errorf("description lost:\n%q", out)
	}
	reparsed, err := ParseMBCH(out)
	if err != nil {
		t.Fatalf("re-parse failed: %v\noutput:\n%s", err, out)
	}
	if reparsed.Description != want {
		t.Errorf("multiline description lost on round trip: %q", reparsed.Description)
	}
}

// A stray description inside ClassInfo is not treated as an extra, so
// saving never emits it twice.
func TestMBCHDescriptionNotDuplicatedIntoClassInfoExtras(t *testing.T) {
	input := `description "Outer"
ClassInfo {
	name	"D"
	description "Inner"
}`
	char, err := ParseMBCH(input)
	if err != nil {
		t.Fatal(err)
	}
	if _, dup := char.ExtraFields["description"]; dup {
		t.Fatalf("in-block description leaked into ExtraFields: %v", char.ExtraFields)
	}
	out, err := GenerateMBCH(char)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(out, "description") != 2 {
		t.Errorf("description lines not preserved exactly:\n%s", out)
	}
}

// Typed WeaponInfo fields survive a parse → generate round trip through
// the AST path instead of vanishing (they used to be typed at parse,
// never synced, and never stored as extras).
func TestMBCHTypedWeaponFieldsRoundTrip(t *testing.T) {
	input := `ClassInfo { name "T" }
WeaponInfo0 {
	WeaponToReplace	WP_BLASTER
	Missile3Effect	"mfx3"
	AltMissileEffect3 "amfx3"
	PowerupShotEffect "pse"
	PowerupShotEffect3 "pse3"
	AltChargeSound "acs"
	PrimHitSound "phs"
	AltHitSound "ahs"
}`
	char, err := ParseMBCH(input)
	if err != nil {
		t.Fatal(err)
	}
	wi := char.WeaponOverrides[0]
	if wi.Missile3Effect != "mfx3" || wi.PowerupShotEffect3 != "pse3" || wi.AltHitSound != "ahs" {
		t.Fatalf("typed weapon fields not parsed: %+v", wi)
	}
	for k := range wi.ExtraFields {
		t.Fatalf("typed field %s leaked into extras", k)
	}
	out, err := GenerateMBCH(char)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"Missile3Effect", "AltMissileEffect3", "PowerupShotEffect", "PowerupShotEffect3", "AltChargeSound", "PrimHitSound", "AltHitSound"} {
		if !strings.Contains(out, want) {
			t.Errorf("typed field %s lost on save:\n%s", want, out)
		}
	}
}

// Assessment measures payloads the way BG_SiegeGetValueGroup copies
// them: comments and whitespace included, outer braces excluded.
func TestAssessmentIncludesCommentsInBudget(t *testing.T) {
	src := `// leading comment
ClassInfo {
	// a raw comment line the engine still copies
	name	"Budget"
	another	"value"
}`
	res, err := AssessMBCHSourceBuffers(src)
	if err != nil {
		t.Fatal(err)
	}
	// Interior = "\n\t// a raw comment line the engine still copies\n\tname...." —
	// everything between the braces, comment included.
	if res.ClassInfoBytes == 0 {
		t.Fatal("ClassInfo payload not measured")
	}
	noComment := strings.Replace(src, "\t// a raw comment line the engine still copies\n", "", 1)
	resNoComment, _ := AssessMBCHSourceBuffers(noComment)
	diffWith := res.ClassInfoBytes - resNoComment.ClassInfoBytes
	commentLen := len("\t// a raw comment line the engine still copies\n")
	if diffWith != commentLen {
		t.Errorf("comment bytes not counted in budget: delta %d, want %d", diffWith, commentLen)
	}
}

// The paired-value scan must not read value tokens as keys: on
// `key1 val1 key2 val2` only key1 is effective (SGPV skips to EOL), so
// the long val2 must not count and val1 must.
func TestAssessmentPairedValueScanFirstEffective(t *testing.T) {
	src := "ClassInfo {\n\tbig1 " + strings.Repeat("a", 100) + " big2 " + strings.Repeat("b", 2000) + "\n}"
	res, err := AssessMBCHSourceBuffers(src)
	if err != nil {
		t.Fatal(err)
	}
	// Engine reads big1 = <100 a's>; "big2 ..." is dead text on the
	// same line.
	if res.MaxPairedValueBytes != 100 {
		t.Errorf("MaxPairedValueBytes = %d, want 100 (value token misread as key?)", res.MaxPairedValueBytes)
	}
}

// Quoted values keep their interior only: SGPV copies between the
// quotes, so a 2047-byte interior fits SIEGE_PARSE_BUF_LEN while the
// raw token is 2049.
func TestAssessmentQuotedValueInteriorAccounting(t *testing.T) {
	interior := strings.Repeat("x", 2047)
	src := "ClassInfo {\n\tbig \"" + interior + "\"\n}"
	res, err := AssessMBCHSourceBuffers(src)
	if err != nil {
		t.Fatal(err)
	}
	if res.MaxPairedValueBytes != 2047 {
		t.Errorf("MaxPairedValueBytes = %d, want 2047 (quotes must not count)", res.MaxPairedValueBytes)
	}
}

// Blade pruning: shrinking a saber removes the numbered blade lines.
func TestSABBladePrune(t *testing.T) {
	input := `saber_a
{
	numBlades		3
	saberColor		orange
	saberLength		32.0
	saberRadius		3.0
	saberColor2		blue
	saberLength2	24.0
	saberRadius2	2.0
	saberColor3		green
	saberLength3	40.0
	saberRadius3	4.0
}`
	saber, err := ParseSABDefinition(input, 0)
	if err != nil {
		t.Fatal(err)
	}
	saber.Blades = saber.Blades[:1]
	saber.NumBlades = 1
	out, err := GenerateSAB(saber)
	if err != nil {
		t.Fatal(err)
	}
	for _, gone := range []string{"saberColor2", "saberLength2", "saberRadius2", "saberColor3", "saberLength3", "saberRadius3"} {
		if strings.Contains(out, gone) {
			t.Errorf("pruned blade key %s survived:\n%s", gone, out)
		}
	}
	if !strings.Contains(out, "saberColor\t\torange") {
		t.Errorf("blade 1 content lost:\n%s", out)
	}
}

// VEH extras: edits render; deleted extras disappear; unknown keys
// survive no-op saves.
func TestVEHExtraFieldSync(t *testing.T) {
	input := `veh_a { name "Speeder" type VH_SPEEDER customGravity 0.5 mystery "42" }`
	veh, err := ParseVEHDefinition(input, 0)
	if err != nil {
		t.Fatal(err)
	}
	out, err := GenerateVEH(veh)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "mystery") || !strings.Contains(out, "customGravity") {
		t.Errorf("no-op save lost extras:\n%s", out)
	}
	delete(veh.ExtraFields, "mystery")
	veh.CustomGravity = 0.9
	out, err = GenerateVEH(veh)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out, "mystery") {
		t.Errorf("deleted extra survived: %s", out)
	}
	if !strings.Contains(out, "customGravity") || !strings.Contains(out, "0.9") {
		t.Errorf("typed edit not applied: %s", out)
	}
}
