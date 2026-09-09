package main

import (
	"archive/zip"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"

	"github.com/Frenzeh/mbii-foundry/parsers"
)

// realMBTCFixture mirrors the shipped engine format (e.g. PB3_B.mbtc):
// flat paired values, a Classes group, per-class subclass groups,
// comments, a legacy mask, and unmodeled content.
const realMBTCFixture = `//Siege team definition file.
// header comment must survive

name "Wacky_B"

ClassesAllowed 6
TimePeriod 5
EUAllowed 0

MyFlag 1

Classes
{
class1	    	"PB3_B_Stormurai"
class2		"PB3_B_Inferno"
}

SubclassesForClass1
{
	Subclass1	"PB3_B_Wesker"
	Subclass2   	"PB3_B_BD"
}

SubclassesForClass2
{
	Subclass1	"PB3_B_Reelo"
}

CustomGroup
{
	inner "value"
}
`

// newTestComposer builds a composer whose widgets exist (parse/render
// touch them) without showing a real window.
func newTestComposer(t *testing.T) *MBTCComposerDialog {
	t.Helper()
	c := &MBTCComposerDialog{subClassIdx: 0}
	c.nameEntry = widget.NewEntry()
	c.timeEntry = widget.NewEntry()
	c.euCheck = widget.NewCheck("", nil)
	c.shaderEntry = widget.NewEntry()
	c.classesAllowed = widget.NewEntry()
	for i := 0; i < parsers.MBTCMaxClasses; i++ {
		c.classSlots[i] = widget.NewEntry()
	}
	c.subEntry = widget.NewMultiLineEntry()
	c.subClassSelect = widget.NewSelect([]string{"class1", "class2", "class3", "class4", "class5", "class6"}, nil)
	c.statusLabel = widget.NewLabel("")
	return c
}

func TestParseMBTCReadsEngineFormat(t *testing.T) {
	c := newTestComposer(t)
	c.parseMBTC(realMBTCFixture)

	if c.roster.Name != "Wacky_B" {
		t.Errorf("name: %q", c.roster.Name)
	}
	if !c.roster.TimePeriodSet || c.roster.TimePeriod != 5 {
		t.Errorf("TimePeriod: set=%v value=%d", c.roster.TimePeriodSet, c.roster.TimePeriod)
	}
	if !c.roster.EUAllowedSet || c.roster.EUAllowed != 0 {
		t.Errorf("EUAllowed: set=%v value=%d", c.roster.EUAllowedSet, c.roster.EUAllowed)
	}
	if !c.roster.ClassesAllowedSet || c.roster.ClassesAllowed != 6 {
		t.Errorf("ClassesAllowed legacy: set=%v value=%d", c.roster.ClassesAllowedSet, c.roster.ClassesAllowed)
	}
	if c.roster.Classes[0] != "PB3_B_Stormurai" || c.roster.Classes[1] != "PB3_B_Inferno" {
		t.Errorf("classes: %q / %q", c.roster.Classes[0], c.roster.Classes[1])
	}
	if len(c.roster.Subclasses[0]) != 2 || c.roster.Subclasses[0][0] != "PB3_B_Wesker" {
		t.Errorf("subclasses for class1: %q", c.roster.Subclasses[0])
	}
}

func TestParseMBTCFirstEffectiveWins(t *testing.T) {
	// SGPV answers with the FIRST occurrence; a later duplicate is dead
	// text and must not overwrite the model.
	content := `name "First"
name "Second"
ClassesAllowed 7
ClassesAllowed 9
`
	team := parsers.ParseMBTC(content)
	if team.Name != "First" {
		t.Fatalf("name: %q, want first-effective First", team.Name)
	}
	if team.ClassesAllowed != 7 {
		t.Fatalf("ClassesAllowed: %d, want first-effective 7", team.ClassesAllowed)
	}
	joined := strings.Join(team.Diagnostics, "; ")
	if !strings.Contains(joined, "duplicate") {
		t.Errorf("duplicates should be diagnosed: %q", joined)
	}
}

func TestParseMBTCCommentMarkersDoNotSplitTeams(t *testing.T) {
	content := "// Imperial\n// Dark Side roster below\n" + realMBTCFixture
	team := parsers.ParseMBTC(content)

	if team.Name != "Wacky_B" {
		t.Fatalf("name: %q", team.Name)
	}
	if team.Classes[0] != "PB3_B_Stormurai" || team.Classes[1] != "PB3_B_Inferno" {
		t.Fatalf("comment marker displaced classes: %q %q", team.Classes[0], team.Classes[1])
	}
	if len(team.Subclasses[0]) != 2 {
		t.Fatalf("subclass group scrambled: %q", team.Subclasses[0])
	}
}

func TestParseMBTCMalformedNumericsAreClean(t *testing.T) {
	content := `name "Broken"
TimePeriod abc
EUAllowed ""
ClassesAllowed 3.5
`
	team := parsers.ParseMBTC(content)

	if team.TimePeriodSet || team.EUAllowedSet || team.ClassesAllowedSet {
		t.Fatalf("malformed numerics must stay unset: %+v", team)
	}
	joined := strings.Join(team.Diagnostics, "; ")
	for _, key := range []string{"TimePeriod", "EUAllowed", "ClassesAllowed"} {
		if !strings.Contains(joined, key) {
			t.Errorf("no diagnostic for malformed %s: %q", key, joined)
		}
	}
}

func TestParseMBTCResetsPriorState(t *testing.T) {
	c := newTestComposer(t)
	c.parseMBTC(realMBTCFixture)
	if c.roster.Name == "" {
		t.Fatal("fixture should populate the roster first")
	}

	c.parseMBTC("name \"Other\"\n")
	if c.roster.Name != "Other" {
		t.Fatalf("name: %q", c.roster.Name)
	}
	if c.roster.Classes[0] != "" || len(c.roster.Subclasses[0]) != 0 {
		t.Fatalf("previous roster leaked through parse: %+v", c.roster)
	}
	if c.roster.TimePeriodSet || c.roster.ClassesAllowedSet {
		t.Fatalf("previous scalar state leaked: %+v", c.roster)
	}
}

// R2: serialization must be non-destructive — a no-op renders the exact
// original bytes, and an edit preserves every unmodeled token.
func TestMBTCSerializeNoOpIsByteIdentical(t *testing.T) {
	team := parsers.ParseMBTC(realMBTCFixture)
	if got := team.Serialize(); got != realMBTCFixture {
		t.Fatalf("no-op serialize changed the document:\n--- got ---\n%s\n--- want ---\n%s", got, realMBTCFixture)
	}
}

func TestMBTCSyncPreservesUnmodeledOnEdit(t *testing.T) {
	team := parsers.ParseMBTC(realMBTCFixture)
	team.Name = "Edited_B"
	team.Classes[1] = "New_Guy"
	out := team.Serialize()

	for _, keep := range []string{
		"// header comment must survive",
		"MyFlag 1",
		"CustomGroup",
		`inner "value"`,
		"ClassesAllowed 6",
		"TimePeriod 5",
		"EUAllowed 0",
		`class1	    	"PB3_B_Stormurai"`, // original formatting untouched
		"PB3_B_Wesker",
		"PB3_B_Reelo",
	} {
		if !strings.Contains(out, keep) {
			t.Errorf("unmodeled/modeled content lost on edit: %q", keep)
		}
	}
	if !strings.Contains(out, "Edited_B") || !strings.Contains(out, "New_Guy") {
		t.Errorf("edit not applied:\n%s", out)
	}
	// The replaced value must not leak the old one.
	if strings.Contains(out, "PB3_B_Inferno") {
		t.Errorf("old class2 value survived the edit:\n%s", out)
	}
}

func TestMBTCSyncShrinksSubclassGhosts(t *testing.T) {
	team := parsers.ParseMBTC(realMBTCFixture) // class1 has 2 subclasses
	team.Subclasses[0] = team.Subclasses[0][:1]
	out := team.Serialize()

	if strings.Contains(out, "Subclass2") && strings.Contains(out, "PB3_B_BD") {
		t.Errorf("shrunk subclass entry survived as a ghost:\n%s", out)
	}
	reparsed := parsers.ParseMBTC(out)
	if len(reparsed.Subclasses[0]) != 1 {
		t.Fatalf("shrunk list did not round-trip: %q", reparsed.Subclasses[0])
	}
}

func TestMBTCNewTeamSerializeStillWorks(t *testing.T) {
	// Teams composed fresh in the UI have no source AST; the canonical
	// writer must still produce a valid engine file.
	team := parsers.MBTCTeam{Name: "Fresh", TimePeriod: 4, TimePeriodSet: true}
	team.Classes[0] = "mb2_rebel_soldier"
	team.Subclasses[0] = []string{"mb2_rebel_soldier2"}
	out := team.Serialize()

	reparsed := parsers.ParseMBTC(out)
	if reparsed.Name != "Fresh" || reparsed.Classes[0] != "mb2_rebel_soldier" || len(reparsed.Subclasses[0]) != 1 {
		t.Fatalf("fresh-team writer produced an unparseable file:\n%s", out)
	}
}

// R3: validation judges the CURRENT candidate — never stale parse
// diagnostics, never letting a corrected load stay blocked.
func TestMBTCValidateJudgesCurrentCandidate(t *testing.T) {
	// Fresh empty team must be BLOCKED (engine ERR_DROPs).
	empty := parsers.MBTCTeam{}
	if issues := empty.Validate(); len(issues) == 0 {
		t.Fatal("empty team must fail validation")
	}

	// A corrected candidate from an invalid file must PASS: stale
	// diagnostics must not poison it.
	bad := parsers.ParseMBTC("") // diagnostics: no name
	if len(bad.Diagnostics) == 0 {
		t.Fatal("empty source should carry load diagnostics")
	}
	bad.Name = "Fixed"
	bad.Classes[0] = "mb2_rebel_soldier"
	if issues := bad.Validate(); len(issues) != 0 {
		t.Fatalf("corrected candidate must validate clean: %q", strings.Join(issues, "; "))
	}
}

func TestMBTCValidateEngineRules(t *testing.T) {
	// Gap: engine stops at the first empty slot.
	gappy := parsers.MBTCTeam{Name: "G"}
	gappy.Classes[0] = "a"
	gappy.Classes[2] = "c"
	issues := gappy.Validate()
	if !contains(issues, "class2 is empty") {
		t.Fatalf("gap not diagnosed: %q", strings.Join(issues, "; "))
	}

	// Subclass cap: engine reads at most 41.
	over := parsers.MBTCTeam{Name: "O"}
	over.Classes[0] = "a"
	for i := 0; i < 42; i++ {
		over.Subclasses[0] = append(over.Subclasses[0], "s")
	}
	if !contains(over.Validate(), "at most 41") {
		t.Fatalf("subclass cap not enforced: %q", strings.Join(over.Validate(), "; "))
	}

	// Quotes and NUL are unrepresentable for SGPV.
	quoted := parsers.MBTCTeam{Name: "Q"}
	quoted.Classes[0] = `bad"name`
	if !contains(quoted.Validate(), "quote or NUL") {
		t.Fatalf("quote not rejected: %q", strings.Join(quoted.Validate(), "; "))
	}

	// Spaces ARE representable — the engine reads quoted values.
	spaced := parsers.MBTCTeam{Name: "S"}
	spaced.Classes[0] = "My Soldier"
	if issues := spaced.Validate(); len(issues) != 0 {
		t.Fatalf("spaces must be allowed in quoted refs: %q", strings.Join(issues, "; "))
	}
	// And serialize quoted so the value survives SGPV.
	if !strings.Contains(spaced.Serialize(), `"My Soldier"`) {
		t.Fatalf("spaced ref not quoted: %s", spaced.Serialize())
	}
}

func contains(list []string, sub string) bool {
	for _, s := range list {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}

// R4: reference resolution scans VFS content (loose AND PK3) with the
// canonical parser, no cap, and reports unavailability explicitly.
func TestValidateClassRefsVFSLooseAndPK3(t *testing.T) {
	app := &App{}
	gamedata := t.TempDir()
	textassets := t.TempDir()
	// The VFS indexes loose files under TextAssets and PK3s under
	// gamedata — place content where the indexer actually looks.
	charDir := filepath.Join(textassets, "ext_data", "mb2", "character")
	if err := os.MkdirAll(charDir, 0755); err != nil {
		t.Fatal(err)
	}
	// Ref by stem (loose).
	if err := os.WriteFile(filepath.Join(charDir, "mb2_rebel_soldier.mbch"), []byte("ClassInfo\n{\n\tname\t\t\"soldier\"\n}\n"), 0644); err != nil {
		t.Fatal(err)
	}
	// Ref by parsed `name` field, different from the stem.
	if err := os.WriteFile(filepath.Join(charDir, "h10_obi.mbch"), []byte("ClassInfo\n{\n\tname\t\t\"sith_lord\"\n}\n"), 0644); err != nil {
		t.Fatal(err)
	}
	// Ref inside a PK3.
	pk3Path := filepath.Join(gamedata, "z_assets.pk3")
	pk3, err := os.Create(pk3Path)
	if err != nil {
		t.Fatal(err)
	}
	zw := zip.NewWriter(pk3)
	entry, err := zw.Create("ext_data/mb2/character/pk3_hero.mbch")
	if err != nil {
		t.Fatal(err)
	}
	entry.Write([]byte("ClassInfo\n{\n\tname\t\t\"pk3_hero\"\n}\n"))
	zw.Close()
	pk3.Close()

	app.config.GamedataPath = gamedata
	app.config.TextAssetsPath = textassets
	c := newTestComposer(t)
	c.app = app

	names, err := c.classRefIndex()
	if err != nil {
		t.Fatalf("ref index should build: %v", err)
	}
	for _, ref := range []string{"mb2_rebel_soldier", "soldier", "sith_lord", "pk3_hero"} {
		if !names[ref] {
			t.Errorf("reference %q not resolved from VFS (loose+PK3)", ref)
		}
	}

	// The composer's validate surfaces missing refs as warnings, not
	// hard failures, and unavailable roots as an explicit unknown.
	c.roster = parsers.MBTCTeam{Name: "T"}
	c.roster.Classes[0] = "mb2_ghost"
	c.nameEntry.SetText("T")
	c.classSlots[0].SetText("mb2_ghost")
	warnings, err := c.validate()
	if err != nil {
		t.Fatalf("missing refs are warnings, not errors: %v", err)
	}
	if len(warnings) != 1 || !strings.Contains(warnings[0], "mb2_ghost") {
		t.Fatalf("missing ref warning wrong: %q", warnings)
	}

	emptyApp := &App{}
	c2 := newTestComposer(t)
	c2.app = emptyApp
	c2.roster = parsers.MBTCTeam{Name: "T2"}
	c2.roster.Classes[0] = "anything"
	_, err = c2.classRefIndex()
	if err == nil || !strings.Contains(err.Error(), "no gamedata") {
		t.Fatalf("unavailable roots must be an explicit unknown: %v", err)
	}
}

// R1: the subclass selector must edit the slot it names, and switching
// between slots must preserve both sides.
func TestMBTCSubclassSwitchingEditsCorrectSlots(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	c := &MBTCComposerDialog{app: &App{fyneApp: app, mainWindow: app.NewWindow("composer")}}
	c.show() // the REAL construction path — order bugs would panic here

	c.nameEntry.SetText("Switcher")
	c.classSlots[0].SetText("class_a")
	c.classSlots[1].SetText("class_b")

	// Edit class1's subclasses.
	c.subEntry.SetText("sub_a1\nsub_a2")

	// Switch to class2 (user action): the select callback must store
	// class1's list and load class2's (empty) one.
	c.subClassSelect.SetSelected("class2")
	if c.subClassIdx != 1 {
		t.Fatalf("selection did not switch slots: idx=%d", c.subClassIdx)
	}
	if c.subEntry.Text != "" {
		t.Fatalf("class2 editor should start empty, got %q", c.subEntry.Text)
	}
	c.subEntry.SetText("sub_b1")

	// Switch back: class1's edits must be intact, then class2's again.
	c.subClassSelect.SetSelected("class1")
	if got := strings.Join(c.roster.Subclasses[0], ","); got != "sub_a1,sub_a2" {
		t.Fatalf("class1 subclasses lost across switch: %q", got)
	}
	c.subClassSelect.SetSelected("class2")
	if got := strings.Join(c.roster.Subclasses[1], ","); got != "sub_b1" {
		t.Fatalf("class2 subclasses lost across switch: %q", got)
	}

	// Save and reload: values must round-trip through the file.
	output, err := c.serialize()
	if err != nil {
		t.Fatalf("serialize: %v", err)
	}
	reparsed := parsers.ParseMBTC(output)
	if reparsed.Name != "Switcher" ||
		len(reparsed.Subclasses[0]) != 2 || reparsed.Subclasses[0][0] != "sub_a1" ||
		len(reparsed.Subclasses[1]) != 1 || reparsed.Subclasses[1][0] != "sub_b1" {
		t.Fatalf("save/reload mismatch:\n%s", output)
	}
}

// R3: a fresh empty team must not save; a corrected load must.
func TestMBTCComposeEmptyTeamBlocked(t *testing.T) {
	c := newTestComposer(t)
	c.roster = parsers.MBTCTeam{}
	if _, err := c.validate(); err == nil {
		t.Fatal("an empty candidate must be blocked (engine ERR_DROPs: no name, no classes)")
	}
}

func TestMBTCSerializeRejectsInvalidInputInsteadOfFallback(t *testing.T) {
	c := newTestComposer(t)
	// Non-numeric TimePeriod: serialize must return the error, not
	// silently serialize a half-collected roster.
	c.nameEntry.SetText("Okay")
	c.timeEntry.SetText("five")
	if _, err := c.serialize(); err == nil || !strings.Contains(err.Error(), "TimePeriod") {
		t.Fatalf("serialize must surface the collection error: %v", err)
	}
}

// R2 blocker: Serialize must clone the retained baseline. A mutated
// baseline would (a) make a fresh parse of the same source render
// differently and (b) compound changes across repeated serializations.
func TestMBTCSerializeCloneLeavesBaselinePristine(t *testing.T) {
	first := parsers.ParseMBTC(realMBTCFixture)
	first.Name = "Mutated"
	first.Classes[0] = "Replaced"

	// A second team parsed from the SAME source must still render the
	// original bytes — proving the first team's sync never touched a
	// shared baseline.
	fresh := parsers.ParseMBTC(realMBTCFixture)
	if got := fresh.Serialize(); got != realMBTCFixture {
		t.Fatalf("baseline was mutated by another team's serialize:\n%s", got)
	}

	// Repeated serialization of one team is stable.
	once := first.Serialize()
	twice := first.Serialize()
	if once != twice {
		t.Fatalf("serialize is not idempotent:\n--once--\n%s\n--twice--\n%s", once, twice)
	}
	if !strings.Contains(once, "Mutated") {
		t.Fatalf("edit lost across serializations:\n%s", once)
	}
}

// R2 blocker: clearing all classes must strip ghost class1..class6 from
// the first-effective Classes group so validation refuses the save;
// unknown keys inside the group survive.
func TestMBTCClearClassesRemovesGhostSlotsAndBlocksSave(t *testing.T) {
	content := `name "Ghosted"

Classes
{
	class1	"old_one"
	class2	"old_two"
	weirdkey "keep me"
}
`
	team := parsers.ParseMBTC(content)
	for i := range team.Classes {
		team.Classes[i] = ""
	}
	out := team.Serialize()

	if strings.Contains(out, "old_one") || strings.Contains(out, "old_two") {
		t.Fatalf("ghost class slots survived clearing:\n%s", out)
	}
	if !strings.Contains(out, `weirdkey "keep me"`) {
		t.Fatalf("unknown key inside Classes was deleted:\n%s", out)
	}

	// The consumer gate must now see "no classes" and refuse.
	reparsed := parsers.ParseMBTC(out)
	if issues := reparsed.Validate(); !contains(issues, "at least one class slot") {
		t.Fatalf("validation must refuse a team without classes: %q", strings.Join(issues, "; "))
	}
	if err := parsers.ValidateSerialized(parsers.FormatMBTC, out, "name", "Ghosted"); err == nil ||
		!strings.Contains(err.Error(), "at least one class slot") {
		t.Fatalf("ValidateSerialized must refuse the classless team: %v", err)
	}
}

// R2 blocker: appended missing modeled fields must appear in a FIXED
// sequence — map iteration order would flip bytes between runs.
func TestMBTCSyncAppendsMissingFieldsInFixedOrder(t *testing.T) {
	src := "name \"Ordered\"\n\nClasses\n{\n\tclass1\t\"c1\"\n}\n"
	var reference string
	for i := 0; i < 25; i++ { // 25 rounds: Go map randomization would flip a map-order implementation
		team := parsers.ParseMBTC(src)
		team.TimePeriod, team.TimePeriodSet = 5, true
		team.EUAllowed, team.EUAllowedSet = 1, true
		team.ClassesAllowed, team.ClassesAllowedSet = 6, true
		out := team.Serialize()

		tp := strings.Index(out, "TimePeriod\t5")
		eu := strings.Index(out, "EUAllowed\t1")
		ca := strings.Index(out, "ClassesAllowed\t6")
		if tp == -1 || eu == -1 || ca == -1 {
			t.Fatalf("missing fields not appended:\n%s", out)
		}
		if !(tp < eu && eu < ca) {
			t.Fatalf("append order is not the fixed TimePeriod<EUAllowed<ClassesAllowed sequence:\n%s", out)
		}
		if reference == "" {
			reference = out
		} else if out != reference {
			t.Fatalf("byte output flipped between runs:\n--a--\n%s\n--b--\n%s", reference, out)
		}
	}
}

func TestMBTCNoOpPreservesEngineCommentAndDeadSameLineTokens(t *testing.T) {
	source := "name \"Team//dead\" FriendlyShader should_not_be_a_key\n" +
		"Classes\n{\n\tclass1 \"soldier//dead\" class2 should_not_be_a_key\n}\n"
	team := parsers.ParseMBTC(source)
	if team.Name != "Team" || team.Classes[0] != "soldier" {
		t.Fatalf("engine comment semantics not modeled: name=%q class=%q", team.Name, team.Classes[0])
	}
	if team.FriendlyShader != "" || team.Classes[1] != "" {
		t.Fatalf("dead same-line tokens were parsed as fields: shader=%q class2=%q", team.FriendlyShader, team.Classes[1])
	}
	if got := team.Serialize(); got != source {
		t.Fatalf("no-op changed engine-truncated draft:\n%s", got)
	}
}

func TestMBTCClearRemovesAllDuplicateOccurrences(t *testing.T) {
	source := `name "Duplicates"
TimePeriod 1 // keep-time-comment
TimePeriod 2
Classes
{
	class1 one // keep-class-comment
	class1 dead_one
}
SubclassesForClass1
{
	Subclass1 sub // keep-subclass-comment
	Subclass1 dead_sub
}
`
	team := parsers.ParseMBTC(source)
	team.TimePeriod, team.TimePeriodSet = 0, false
	team.Classes[0] = ""
	team.Subclasses[0] = nil
	out := team.Serialize()
	if strings.Contains(strings.ToLower(out), "timeperiod") ||
		strings.Contains(out, "class1 one") || strings.Contains(out, "class1 dead_one") ||
		strings.Contains(out, "Subclass1 sub") || strings.Contains(out, "Subclass1 dead_sub") {
		t.Fatalf("cleared duplicate modeled fields survived:\n%s", out)
	}
	for _, comment := range []string{"keep-time-comment", "keep-class-comment", "keep-subclass-comment"} {
		if !strings.Contains(out, comment) {
			t.Fatalf("clearing modeled fields deleted comment %q:\n%s", comment, out)
		}
	}
}

func TestMBTCAuthoredSubclassGapBlocksUntilNormalized(t *testing.T) {
	source := `name "Gap"
Classes
{
	class1 c
}
SubclassesForClass1
{
	Subclass1 a
	Subclass3 dead
}
`
	team := parsers.ParseMBTC(source)
	if issues := team.Validate(); !contains(issues, "Subclass2 missing") {
		t.Fatalf("authored subclass gap must block unchanged output: %q", strings.Join(issues, "; "))
	}
	team.Subclasses[0] = []string{"a", "b"}
	if issues := team.Validate(); len(issues) != 0 {
		t.Fatalf("normalized candidate should validate: %q", strings.Join(issues, "; "))
	}
	out := team.Serialize()
	if strings.Contains(out, "Subclass3") || !strings.Contains(out, "Subclass2") {
		t.Fatalf("normalized subclass output retained a dead tail:\n%s", out)
	}
}

func TestMBTCValidateRejectsEngineFileLimit(t *testing.T) {
	team := parsers.MBTCTeam{Name: strings.Repeat("n", parsers.MBTCMaxFileBytes)}
	team.Classes[0] = "c"
	if issues := team.Validate(); !contains(issues, "team file is") {
		t.Fatalf("engine file limit missing: %q", strings.Join(issues, "; "))
	}
}

func TestMBTCComposerPreservesExplicitEUAllowedZero(t *testing.T) {
	source := "name \"Zero\"\nEUAllowed 0\nClasses\n{\n\tclass1 c\n}\n"
	c := newTestComposer(t)
	c.parseMBTC(source)
	out, err := c.serialize()
	if err != nil {
		t.Fatal(err)
	}
	if out != source {
		t.Fatalf("explicit false value was treated as absent:\n%s", out)
	}
}

func TestMBTCCommitSaveBacksUpChangedExistingFileOnly(t *testing.T) {
	configDir := t.TempDir()
	path := filepath.Join(t.TempDir(), "team.mbtc")
	if err := os.WriteFile(path, []byte("old"), 0644); err != nil {
		t.Fatal(err)
	}
	c := &MBTCComposerDialog{app: &App{fileManager: NewFileManager(configDir)}}
	if err := c.commitSave(path, []byte("new")); err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(path); string(got) != "new" {
		t.Fatalf("saved bytes = %q", got)
	}
	entries, err := os.ReadDir(filepath.Join(configDir, "backups"))
	if err != nil || len(entries) != 1 {
		t.Fatalf("changed save backups = %d, err=%v", len(entries), err)
	}
	if err := c.commitSave(path, []byte("new")); err != nil {
		t.Fatal(err)
	}
	entries, err = os.ReadDir(filepath.Join(configDir, "backups"))
	if err != nil || len(entries) != 1 {
		t.Fatalf("unchanged save created another backup: %d, err=%v", len(entries), err)
	}
}

func TestMBTCCommitSaveRefusesUnbackedOverwrite(t *testing.T) {
	path := filepath.Join(t.TempDir(), "team.mbtc")
	if err := os.WriteFile(path, []byte("old"), 0644); err != nil {
		t.Fatal(err)
	}
	c := &MBTCComposerDialog{app: &App{}}
	if err := c.commitSave(path, []byte("new")); err == nil {
		t.Fatal("existing file overwrite must require a checked backup")
	}
	if got, _ := os.ReadFile(path); string(got) != "old" {
		t.Fatalf("failed backup path still overwrote file: %q", got)
	}
}

func TestMBTCCommitSaveRejectsChangeSinceOpenBaseline(t *testing.T) {
	configDir := t.TempDir()
	path := filepath.Join(t.TempDir(), "team.mbtc")
	original := []byte("name original\n")
	if err := os.WriteFile(path, original, 0644); err != nil {
		t.Fatal(err)
	}
	baseline, err := snapshotSaveDestination(path)
	if err != nil {
		t.Fatal(err)
	}
	c := &MBTCComposerDialog{
		app:          &App{fileManager: NewFileManager(configDir)},
		filePath:     path,
		baselinePath: path,
		baseline:     baseline,
		baselineSet:  true,
	}
	external := []byte("name external\n")
	if err := os.WriteFile(path, external, 0644); err != nil {
		t.Fatal(err)
	}

	if err := c.commitSave(path, []byte("name candidate\n")); err == nil {
		t.Fatal("save accepted a destination changed while open")
	}
	if got, err := os.ReadFile(path); err != nil || string(got) != string(external) {
		t.Fatalf("external change was overwritten: %q, %v", got, err)
	}
	if c.baseline.Bytes != string(original) || c.baselinePath != path {
		t.Fatal("failed save changed the loaded baseline")
	}
	if _, err := os.Stat(filepath.Join(configDir, "backups")); !os.IsNotExist(err) {
		t.Fatalf("stale loaded file was backed up instead of rejected: %v", err)
	}
}

func TestMBTCCommitSaveRejectsDeletionSinceOpenBaseline(t *testing.T) {
	path := filepath.Join(t.TempDir(), "team.mbtc")
	if err := os.WriteFile(path, []byte("original"), 0644); err != nil {
		t.Fatal(err)
	}
	baseline, err := snapshotSaveDestination(path)
	if err != nil {
		t.Fatal(err)
	}
	c := &MBTCComposerDialog{
		app:          &App{fileManager: NewFileManager(t.TempDir())},
		filePath:     path,
		baselinePath: path,
		baseline:     baseline,
		baselineSet:  true,
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}

	if err := c.commitSave(path, []byte("candidate")); err == nil {
		t.Fatal("save recreated a destination deleted while open")
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("deleted destination was recreated: %v", err)
	}
}

func TestMBTCCommitSaveRejectsMutationDuringPublication(t *testing.T) {
	configDir := t.TempDir()
	path := filepath.Join(t.TempDir(), "team.mbtc")
	original := []byte("original")
	if err := os.WriteFile(path, original, 0644); err != nil {
		t.Fatal(err)
	}
	baseline, err := snapshotSaveDestination(path)
	if err != nil {
		t.Fatal(err)
	}
	c := &MBTCComposerDialog{
		app:          &App{fileManager: NewFileManager(configDir)},
		filePath:     path,
		baselinePath: path,
		baseline:     baseline,
		baselineSet:  true,
	}
	external := []byte("external")
	c.beforePublication = func(gotPath string) {
		if gotPath != path {
			t.Fatalf("publication hook path = %q, want %q", gotPath, path)
		}
		if err := os.WriteFile(path, external, 0644); err != nil {
			t.Fatal(err)
		}
		c.beforePublication = nil
	}

	if err := c.commitSave(path, []byte("candidate")); err == nil {
		t.Fatal("save accepted mutation between review and publication")
	}
	if got, err := os.ReadFile(path); err != nil || string(got) != string(external) {
		t.Fatalf("publication overwrote external bytes: %q, %v", got, err)
	}
	entries, err := os.ReadDir(filepath.Join(configDir, "backups"))
	if err != nil || len(entries) != 1 {
		t.Fatalf("expected one verified original backup: %d, %v", len(entries), err)
	}
	backup, err := os.ReadFile(filepath.Join(configDir, "backups", entries[0].Name()))
	if err != nil || string(backup) != string(original) {
		t.Fatalf("backup bytes = %q, %v; want exact open baseline", backup, err)
	}
}

func TestMBTCCommitSaveReportsPostPublicationFailureWithoutAdvancingBaseline(t *testing.T) {
	configDir := t.TempDir()
	path := filepath.Join(t.TempDir(), "team.mbtc")
	original := []byte("name original\n")
	if err := os.WriteFile(path, original, 0644); err != nil {
		t.Fatal(err)
	}
	baseline, err := snapshotSaveDestination(path)
	if err != nil {
		t.Fatal(err)
	}
	c := &MBTCComposerDialog{
		app:          &App{fileManager: NewFileManager(configDir)},
		filePath:     path,
		baselinePath: path,
		baseline:     baseline,
		baselineSet:  true,
	}

	var holdPath, backupPath string
	c.afterCandidatePublish = func(gotPath, hold, backup string) {
		if gotPath != path {
			t.Fatalf("publication hook path = %q, want %q", gotPath, path)
		}
		holdPath, backupPath = hold, backup
		if writeErr := os.WriteFile(backup, []byte("corrupt backup"), 0600); writeErr != nil {
			t.Fatalf("corrupt final backup proof: %v", writeErr)
		}
	}
	candidate := []byte("name candidate\n")
	err = c.commitSave(path, candidate)
	if err == nil || !saveWasPublished(err) {
		t.Fatalf("post-publication failure lost publication outcome: %v", err)
	}
	if got, readErr := os.ReadFile(path); readErr != nil || string(got) != string(candidate) {
		t.Fatalf("published candidate bytes = %q, %v", got, readErr)
	}
	if c.baseline != baseline || c.baselinePath != path || !c.baselineSet {
		t.Fatal("post-publication failure advanced the composer saved baseline")
	}
	for _, recoveryPath := range []string{holdPath, backupPath} {
		if recoveryPath == "" || !strings.Contains(err.Error(), recoveryPath) {
			t.Fatalf("error is missing recovery path %q: %v", recoveryPath, err)
		}
	}
	if !strings.Contains(err.Error(), "candidate bytes were published") {
		t.Fatalf("composer falsely reported no write: %v", err)
	}
}

func TestMBTCCommitSaveAsNewDestinationIsNoClobber(t *testing.T) {
	path := filepath.Join(t.TempDir(), "new-team.mbtc")
	c := &MBTCComposerDialog{}
	external := []byte("created externally")
	c.beforePublication = func(string) {
		if err := os.WriteFile(path, external, 0644); err != nil {
			t.Fatal(err)
		}
		c.beforePublication = nil
	}

	if err := c.commitSave(path, []byte("candidate")); err == nil {
		t.Fatal("Save As overwrote a destination created during publication")
	}
	if got, err := os.ReadFile(path); err != nil || string(got) != string(external) {
		t.Fatalf("Save As clobbered external destination: %q, %v", got, err)
	}
	if c.baselineSet || c.filePath != "" {
		t.Fatal("failed Save As changed composer destination state")
	}
}

func TestMBTCAuthoredClassBeyondEngineCapBlocksUntilRosterEdit(t *testing.T) {
	source := `name "TooMany"
Classes
{
	class1 c
	class7 dead
}
`
	team := parsers.ParseMBTC(source)
	if issues := team.Validate(); !contains(issues, "class7") {
		t.Fatalf("authored class beyond cap must block unchanged output: %q", strings.Join(issues, "; "))
	}
	team.Classes[0] = "replacement"
	if issues := team.Validate(); len(issues) != 0 {
		t.Fatalf("edited capped roster should validate: %q", strings.Join(issues, "; "))
	}
	if out := team.Serialize(); strings.Contains(out, "class7") {
		t.Fatalf("edited roster retained out-of-range class:\n%s", out)
	}
}

func TestMBTCClassRefsUseExistingAppVFSWithoutConfiguredRoots(t *testing.T) {
	root := t.TempDir()
	charDir := filepath.Join(root, "ext_data", "mb2", "character")
	if err := os.MkdirAll(charDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(charDir, "winner.mbch"), []byte("ClassInfo\n{\n\tname canonical_winner\n}\n"), 0644); err != nil {
		t.Fatal(err)
	}
	vfs := NewVirtualFileSystem("", root)
	if err := vfs.Refresh(); err != nil {
		t.Fatal(err)
	}
	c := newTestComposer(t)
	c.app = &App{assetBrowser: &AssetBrowser{vfs: vfs}}

	refs, err := c.classRefIndex()
	if err != nil {
		t.Fatalf("existing app VFS should be sufficient: %v", err)
	}
	if !refs["winner"] || !refs["canonical_winner"] {
		t.Fatalf("stem and canonical parsed name missing from app VFS refs: %#v", refs)
	}
}

func TestMBTCValidateRejectsMalformedQuotedDraft(t *testing.T) {
	source := "name Team\nClasses\n{\n\tclass1 c\n}\nUnknown \"unterminated"
	team := parsers.ParseMBTC(source)
	if issues := team.Validate(); !contains(issues, "unexpected EOF while looking for endquote") {
		t.Fatalf("engine quote failure was not validated: %q", strings.Join(issues, "; "))
	}
	if got := team.Serialize(); got != source {
		t.Fatalf("validation path must not rewrite malformed draft:\n%s", got)
	}
}
