package parsers

import (
	"reflect"
	"strconv"
	"strings"
	"testing"
)

func TestAuditMBCHRankAttributesHaveOneOwner(t *testing.T) {
	input := `ClassInfo
{
	name "Ranked"
	rankHealth 100,125 // keep the effective rank comment
	RANKHEALTH 900,950 // retain this shadowed spelling
	rankArmor 0,50
	rank_custom 1,3
	customField "keep me"
}
`
	character, err := ParseMBCH(input)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{"rankHealth": "100,125", "rankArmor": "0,50", "rank_custom": "1,3"}
	if !reflect.DeepEqual(character.RankAttributes, want) {
		t.Fatalf("rank ownership: got %#v, want %#v", character.RankAttributes, want)
	}
	for key := range character.ExtraFields {
		if strings.HasPrefix(strings.ToLower(key), "rank") {
			t.Fatalf("rank field %q has a competing ExtraFields owner", key)
		}
	}
	noOp, err := GenerateMBCH(character)
	if err != nil || noOp != input {
		t.Fatalf("rank no-op changed retained text: %v\n%s", err, noOp)
	}

	character.RankAttributes["rankHealth"] = "100,150"
	// A stale projection must not overwrite the canonical map's edit.
	character.ExtraFields["rankHealth"] = "100,125"
	output, err := GenerateMBCH(character)
	if err != nil {
		t.Fatal(err)
	}
	expected := strings.Replace(input, "rankHealth 100,125", "rankHealth 100,150", 1)
	if output != expected {
		t.Fatalf("rank edit changed more than the effective value:\n%s", output)
	}
	roundTrip, err := ParseMBCH(output)
	if err != nil || roundTrip.RankAttributes["rankHealth"] != "100,150" {
		t.Fatalf("rank edit did not reach the consumer: %#v, %v", roundTrip, err)
	}

	delete(character.RankAttributes, "rankHealth")
	output, err = GenerateMBCH(character)
	if err != nil {
		t.Fatal(err)
	}
	roundTrip, err = ParseMBCH(output)
	if err != nil {
		t.Fatal(err)
	}
	for key := range roundTrip.RankAttributes {
		if strings.EqualFold(key, "rankHealth") {
			t.Fatalf("deleting the canonical rank revived a duplicate: %#v", roundTrip.RankAttributes)
		}
	}
	if roundTrip.RankAttributes["rankArmor"] != "0,50" || roundTrip.ExtraFields["customField"] != "keep me" {
		t.Fatalf("rank deletion lost neighboring fields: %#v", roundTrip)
	}

	fresh := NewMBCHCharacter()
	fresh.Name = "Fresh ranked"
	fresh.RankAttributes["rankHealth"] = "100,150"
	fresh.ExtraFields["rankHealth"] = "stale"
	output, err = GenerateMBCH(fresh)
	if err != nil {
		t.Fatal(err)
	}
	roundTrip, err = ParseMBCH(output)
	if err != nil || roundTrip.RankAttributes["rankHealth"] != "100,150" {
		t.Fatalf("fresh generation lost canonical ranks: %#v, %v", roundTrip, err)
	}
}

func auditVehicleNumbers(vehicle *VehicleData) [12]float64 {
	return [12]float64{
		vehicle.SpeedMax, vehicle.TurboSpeed, vehicle.Accel, vehicle.Decel,
		vehicle.StrafePerc, vehicle.BankingSpeed, vehicle.RollLimit, vehicle.PitchLimit,
		vehicle.Braking, vehicle.MouseYaw, vehicle.MousePitch, vehicle.CustomGravity,
	}
}

func TestAuditVEHNumericNoOpAndEditedPrecision(t *testing.T) {
	const sibling = `wingmate
{
	name wingmate
	type VH_FIGHTER
	speedMax 725.125
}
`
	input := `// authored vehicle precision
runner
{
	name runner
	type VH_SPEEDER
	speedMax 12.345678901234567 // preserve this comment
	SPEEDMAX 999.0
	turboSpeed 1.2345678901234567e+03
	acceleration 0.12345678901234567
	decelIdle 10.000
	strafePerc .12500
	bankingSpeed 2.718281828459045
	rollLimit -12.345678901234567
	pitchLimit +45.12500
	braking 0.00012500
	mouseYaw 1.4142135623730951
	mousePitch 0.3333333333333333
	customGravity -0.00012500
	unknownField "keep me"
}
` + sibling
	vehicle, err := ParseVEH(input)
	if err != nil {
		t.Fatal(err)
	}
	noOp, err := GenerateVEH(vehicle)
	if err != nil || noOp != input {
		t.Fatalf("numeric no-op changed authored bytes: %v\n%s", err, noOp)
	}

	setNumbers := func(vehicle *VehicleData) {
		vehicle.SpeedMax = 123.12345678901235
		vehicle.TurboSpeed = 987.9876543210987
		vehicle.Accel = 0.12345678901234567
		vehicle.Decel = 10.987654321098765
		vehicle.StrafePerc = 0.23456789012345678
		vehicle.BankingSpeed = 2.345678901234568
		vehicle.RollLimit = -12.987654321098765
		vehicle.PitchLimit = 45.987654321098766
		vehicle.Braking = 0.00012345678901234567
		vehicle.MouseYaw = 1.2345678901234567
		vehicle.MousePitch = 0.3456789012345679
		vehicle.CustomGravity = -0.0009876543210987654
	}
	for _, mode := range []string{"retained", "fresh"} {
		t.Run(mode, func(t *testing.T) {
			current := NewVehicleData()
			current.Name = "runner"
			if mode == "retained" {
				current, err = ParseVEH(input)
				if err != nil {
					t.Fatal(err)
				}
			}
			setNumbers(current)
			output, err := GenerateVEH(current)
			if err != nil {
				t.Fatal(err)
			}
			roundTrip, err := ParseVEH(output)
			if err != nil {
				t.Fatal(err)
			}
			if got, want := auditVehicleNumbers(roundTrip), auditVehicleNumbers(current); got != want {
				t.Fatalf("fractional edit changed numeric values:\ngot  %v\nwant %v\n%s", got, want, output)
			}
			if mode == "retained" {
				for _, retained := range []string{sibling, "// preserve this comment", "SPEEDMAX 999.0", `unknownField "keep me"`} {
					if !strings.Contains(output, retained) {
						t.Fatalf("numeric edit lost retained text %q:\n%s", retained, output)
					}
				}
			}
		})
	}
}

func TestAuditAnonymousDefinitionSlotsPreserveSnapshotFields(t *testing.T) {
	vehicle := NewVehicleData()
	vehicle.Type = "VH_FIGHTER"
	vehicle.SpeedMax = 123.45678901234567
	output, err := GenerateVEH(vehicle)
	if err != nil {
		t.Fatal(err)
	}
	output += "named\n{\n\tname named\n\ttype VH_SPEEDER\n}\n"
	names, err := DefinitionNames(output)
	if err != nil || !reflect.DeepEqual(names, []string{"", "named"}) {
		t.Fatalf("anonymous definition shifted indices: %#v, %v", names, err)
	}
	unnamed, err := ParseVEHDefinition(output, 0)
	if err != nil || unnamed.Name != "" || unnamed.Type != vehicle.Type || unnamed.SpeedMax != vehicle.SpeedMax {
		t.Fatalf("unnamed snapshot swallowed subsequent fields: %#v, %v", unnamed, err)
	}
	named, err := ParseVEHDefinition(output, 1)
	if err != nil || named.Name != "named" {
		t.Fatalf("named sibling no longer matches its slot: %#v, %v", named, err)
	}

	saber := NewSaberData()
	saber.FullName = "Unassigned Saber"
	saber.MoveSpeedScale = 1.2345678901234567
	output, err = GenerateSAB(saber)
	if err != nil {
		t.Fatal(err)
	}
	names, err = DefinitionNames(output)
	if err != nil || !reflect.DeepEqual(names, []string{""}) {
		t.Fatalf("anonymous saber slot missing: %#v, %v", names, err)
	}
	restored, err := ParseSABDefinition(output, 0)
	if err != nil || restored.Name != "" || restored.FullName != saber.FullName || restored.MoveSpeedScale != saber.MoveSpeedScale {
		t.Fatalf("unnamed saber snapshot lost form fields: %#v, %v", restored, err)
	}
}

const auditSABBulkFixture = `// retain header
bulk_saber
{
	name "First" name "Effective"
	numBlades 2
	g2MarksShader "gfx/old" G2MARKSSHADER "gfx/effective" // keep scalar comment
	moveSpeedScale 1.25 moveSpeedScale 1.5
	noWallMarks 2 // keep flag comment
	NOWALLMARKS 0
	unknownField "keep me"
}
sibling_saber { name "Sibling" }
`

func TestAuditSABBulkUsesEffectiveScalarAndFlagValues(t *testing.T) {
	for _, test := range []struct {
		key, old, value string
		read            func(*SaberData) string
	}{
		{"name", "Effective", "New Label", func(s *SaberData) string { return s.FullName }},
		{"g2MarksShader", "gfx/effective", "gfx/edited", func(s *SaberData) string { return s.G2MarksShader }},
		{"moveSpeedScale", "1.5", "1.2345678901234567", func(s *SaberData) string { return strconv.FormatFloat(s.MoveSpeedScale, 'g', -1, 64) }},
	} {
		t.Run(test.key, func(t *testing.T) {
			edit, err := EditConfigField(FormatSAB, auditSABBulkFixture, test.key, test.value)
			if err != nil {
				t.Fatal(err)
			}
			if !edit.Found || edit.OldValue != test.old || edit.Definitions != 2 {
				t.Fatalf("bulk metadata describes a shadowed value: %#v", edit)
			}
			if err := ValidateSerialized(FormatSAB, edit.Content, test.key, test.value); err != nil {
				t.Fatal(err)
			}
			saber, err := ParseSAB(edit.Content)
			if err != nil || test.read(saber) != test.value {
				t.Fatalf("successful bulk edit did not reach the consumer: %#v, %v", saber, err)
			}
			for _, retained := range []string{`name "First"`, `g2MarksShader "gfx/old"`, "// keep scalar comment", `unknownField "keep me"`, `sibling_saber { name "Sibling" }`} {
				if !strings.Contains(edit.Content, retained) {
					t.Fatalf("scalar edit lost %q:\n%s", retained, edit.Content)
				}
			}
		})
	}
	if err := ValidateSerialized(FormatSAB, auditSABBulkFixture, "g2MarksShader", "gfx/old"); err == nil {
		t.Fatal("validation accepted the shadowed scalar instead of the effective value")
	}
	if err := ValidateSerialized(FormatSAB, auditSABBulkFixture, "noWallMarks", "0"); err == nil {
		t.Fatal("validation ignored an earlier non-zero flag occurrence")
	}

	edit, err := EditConfigField(FormatSAB, auditSABBulkFixture, "noWallMarks", "0")
	if err != nil {
		t.Fatal(err)
	}
	if edit.OldValue != "1" || !edit.Found {
		t.Fatalf("flag metadata did not report its accumulated value: %#v", edit)
	}
	if err := ValidateSerialized(FormatSAB, edit.Content, "noWallMarks", "0"); err != nil {
		t.Fatal(err)
	}
	saber, err := ParseSAB(edit.Content)
	if err != nil || saber.NoWallMarks {
		t.Fatalf("bulk clear left an effective true flag: %#v, %v", saber, err)
	}
	for _, retained := range []string{"// keep flag comment", "// keep scalar comment", `unknownField "keep me"`, `sibling_saber { name "Sibling" }`} {
		if !strings.Contains(edit.Content, retained) {
			t.Fatalf("flag clear lost %q:\n%s", retained, edit.Content)
		}
	}
	edit, err = EditConfigField(FormatSAB, edit.Content, "noDlight", "2")
	if err != nil {
		t.Fatal(err)
	}
	if err := ValidateSerialized(FormatSAB, edit.Content, "noDlight", "2"); err != nil {
		t.Fatal(err)
	}
	saber, err = ParseSAB(edit.Content)
	if err != nil || !saber.NoDlight {
		t.Fatalf("non-zero bulk flag did not reach the consumer: %#v, %v", saber, err)
	}
}

const auditSABBladeFixture = `blade_saber
{
	numBlades 3
	saberColor1 red // keep the numbered color
	saberColor2 yellow
	saberColor blue // later global wins
	saberLength1 20.00
	saberLength2 24.00
	saberLength 32.00
	saberRadius1 1.00
	saberRadius2 2.00
	saberRadius 3.00
	unknownField "keep me"
}
sibling_saber { name "Sibling" }
`

func TestAuditSABBladeEditsFollowGlobalAssignments(t *testing.T) {
	for index, name := range []string{"first", "second", "third"} {
		t.Run(name, func(t *testing.T) {
			saber, err := ParseSAB(auditSABBladeFixture)
			if err != nil {
				t.Fatal(err)
			}
			noOp, err := GenerateSAB(saber)
			if err != nil || noOp != auditSABBladeFixture {
				t.Fatalf("blade no-op changed retained bytes: %v\n%s", err, noOp)
			}
			saber.Blades[index] = BladeInfo{Color: "orange", Length: 41.125, Radius: 2.875}
			output, err := GenerateSAB(saber)
			if err != nil {
				t.Fatal(err)
			}
			roundTrip, err := ParseSAB(output)
			if err != nil || !reflect.DeepEqual(roundTrip.Blades, saber.Blades) {
				t.Fatalf("global assignment defeated the blade edit: %#v, %v\n%s", roundTrip, err, output)
			}
			for _, retained := range []string{"saberColor1 red // keep the numbered color", "saberColor blue // later global wins", "saberLength2 24.00", `unknownField "keep me"`, `sibling_saber { name "Sibling" }`} {
				if !strings.Contains(output, retained) {
					t.Fatalf("blade edit lost %q:\n%s", retained, output)
				}
			}
		})
	}
}

func TestAuditSABNumberedBulkEditsReachActualBlades(t *testing.T) {
	for _, test := range []struct {
		key, old, value string
		index           int
		apply           func(*BladeInfo)
	}{
		{"saberColor1", "blue", "orange", 0, func(b *BladeInfo) { b.Color = "orange" }},
		{"saberLength2", "32", "41.1250", 1, func(b *BladeInfo) { b.Length = 41.125 }},
		{"saberRadius3", "3", "2.8750", 2, func(b *BladeInfo) { b.Radius = 2.875 }},
	} {
		t.Run(test.key, func(t *testing.T) {
			before, err := ParseSAB(auditSABBladeFixture)
			if err != nil {
				t.Fatal(err)
			}
			test.apply(&before.Blades[test.index])
			edit, err := EditConfigField(FormatSAB, auditSABBladeFixture, test.key, test.value)
			if err != nil {
				t.Fatal(err)
			}
			if !edit.Found || edit.OldValue != test.old {
				t.Fatalf("numbered metadata ignored global assignments: %#v", edit)
			}
			if err := ValidateSerialized(FormatSAB, edit.Content, test.key, test.value); err != nil {
				t.Fatal(err)
			}
			after, err := ParseSAB(edit.Content)
			if err != nil || !reflect.DeepEqual(after.Blades, before.Blades) {
				t.Fatalf("numbered bulk edit changed the wrong blade: %#v, %v\n%s", after, err, edit.Content)
			}
		})
	}

	// A global applied before blade creation does not initialize later blades.
	input := "ordered {\n\tsaberLength 48.5\n\tnumBlades 3\n}\n"
	if err := ValidateSerialized(FormatSAB, input, "saberLength2", "48.5"); err == nil {
		t.Fatal("validation echoed a global that never reached the second blade")
	}
	edit, err := EditConfigField(FormatSAB, input, "saberLength2", "36.125")
	if err != nil {
		t.Fatal(err)
	}
	if edit.OldValue != "32" {
		t.Fatalf("bulk metadata did not use the modeled blade: %#v", edit)
	}
	after, err := ParseSAB(edit.Content)
	if err != nil || after.Blades[0].Length != 48.5 || after.Blades[1].Length != 36.125 || after.Blades[2].Length != 32 {
		t.Fatalf("source-ordered blade creation was not retained: %#v, %v", after, err)
	}
}

func TestAuditSiegeTeamsResolveIndependentlyOfOrder(t *testing.T) {
	const teams = "Teams\n{\n\tteam1 Blue\n\tteam2 Red\n}\n"
	const blue = `Blue
{
	UseTeam "blue_roster"
	RequiredObjectives 1
	briefing "Advance"
	Objective1
	{
		goalname "beacon"
		objdesc "Reach the beacon"
	}
}
`
	const red = `Red
{
	// keep the untouched team
	UseTeam "red_roster"
	briefing "Hold position"
}
`
	const raw = "Ambient // keep this raw block\n{\n\tmode quiet\n}\n"
	for _, order := range []struct{ name, body string }{
		{"before", teams + blue + red},
		{"between", blue + teams + red},
		{"after", blue + red + teams},
	} {
		t.Run(order.name, func(t *testing.T) {
			input := "missionname \"Order Test\"\n" + order.body + raw
			siege, err := ParseSiege(input)
			if err != nil {
				t.Fatal(err)
			}
			if siege.Team1 == nil || siege.Team2 == nil || siege.Team1.Name != "Blue" || siege.Team2.Name != "Red" || siege.Team1.UseTeam != "blue_roster" || len(siege.Team1.Objectives) != 1 {
				t.Fatalf("Teams order lost named bodies: %#v", siege)
			}
			noOp, err := GenerateSiege(siege)
			if err != nil || noOp != input {
				t.Fatalf("Teams order changed no-op bytes: %v\n%s", err, noOp)
			}
			siege.Team1.RequiredObjectives = 2
			siege.Team1.Objectives[0].ObjDesc = "Activate the beacon"
			output, err := GenerateSiege(siege)
			if err != nil {
				t.Fatal(err)
			}
			after, err := ParseSiege(output)
			if err != nil || after.Team1 == nil || after.Team2 == nil {
				t.Fatalf("generation removed a resolved association: %#v, %v", after, err)
			}
			if after.TeamsMap["team1"] != "Blue" || after.TeamsMap["team2"] != "Red" || after.Team1.RequiredObjectives != 2 || after.Team1.Objectives[0].ObjDesc != "Activate the beacon" {
				t.Fatalf("ordered team edit did not reach its consumer: %#v", after)
			}
			if !strings.Contains(output, red) || !strings.Contains(output, raw) {
				t.Fatalf("team edit changed its untouched sibling or raw block:\n%s", output)
			}
		})
	}
}

func TestAuditMBCHAssessmentCountsRetainedNestedAndTopLevelBytes(t *testing.T) {
	description := "Patrol café\n  keep the outer lane clear"
	classInterior := "\n\tname \"Pilot\"\n\tExtras // keep café and preamble bytes\n\t{\n\t\tvalue retained\n\t\tInner \t{\n\t\t\tflag 1\n\t\t}\n\t}\n"
	weaponInterior := "\n\tWeaponToReplace WP_MELEE\n\tAttachment\n\t// keep attachment spacing\n\t{\n\t\tmode quiet\n\t}\n"
	forceInterior := "\n\tForceToReplace FP_PUSH\n\t{\n\t\tvalue retained\n\t}\n"
	input := "description \"" + description + "\"\nClassInfo {" + classInterior + "}\n" +
		"ClassInfo {\n\tname \"Shadowed\"\n}\n" +
		"WeaponInfo0 {" + weaponInterior + "}\nForceInfo0 {" + forceInterior + "}\n"
	character, err := ParseMBCH(input)
	if err != nil {
		t.Fatal(err)
	}
	output, err := GenerateMBCH(character)
	if err != nil || output != input {
		t.Fatalf("assessment fixture lost retained source: %v\n%s", err, output)
	}
	assessment, err := AssessMBCHSourceBuffers(output)
	if err != nil {
		t.Fatal(err)
	}
	if assessment.TotalFileBytes != len(input) || assessment.ClassInfoBytes != len(classInterior) ||
		!reflect.DeepEqual(assessment.WeaponInfoBytes, []int{len(weaponInterior)}) ||
		!reflect.DeepEqual(assessment.ForceInfoBytes, []int{len(forceInterior)}) ||
		assessment.MaxPairedValueBytes != len(description) {
		t.Fatalf("retained byte counts are not exact: %#v; want file=%d class=%d weapon=%d force=%d paired=%d",
			assessment, len(input), len(classInterior), len(weaponInterior), len(forceInterior), len(description))
	}
	if len(assessment.Diagnostics) != 0 {
		t.Fatalf("ordinary retained groups produced diagnostics: %v", assessment.Diagnostics)
	}

	interior := "\n\tname \"Pilot\"\n\tNested // an unfinished draft\n\t{\n\t\tkey value\n"
	assessment, err = AssessMBCHSourceBuffers("ClassInfo {" + interior)
	if err != nil || assessment.ClassInfoBytes != len(interior) || !strings.Contains(strings.Join(assessment.Diagnostics, "; "), "unbalanced braces") {
		t.Fatalf("unfinished nested text was not measured exactly: %#v, %v", assessment, err)
	}
}

func TestAuditMBCHBulkConsumesAssessmentDiagnostics(t *testing.T) {
	for _, test := range []struct{ name, source, diagnostic string }{
		{"unfinished group", "ClassInfo {\n\tname \"Pilot\"\n\tmaxhealth 100\n", "unbalanced braces"},
		{"unfinished description", "ClassInfo {\n\tname \"Pilot\"\n\tmaxhealth 100\n}\ndescription \"An unfinished note", "unterminated quote"},
	} {
		t.Run(test.name, func(t *testing.T) {
			edit, err := EditConfigField(FormatMBCH, test.source, "maxhealth", "125")
			if err != nil {
				t.Fatal(err)
			}
			err = ValidateSerialized(FormatMBCH, edit.Content, "maxhealth", "125")
			if err == nil || !strings.Contains(err.Error(), test.diagnostic) {
				t.Fatalf("bulk validation ignored assessment diagnostic %q: %v", test.diagnostic, err)
			}
		})
	}
}

func TestAuditMBCHAssessmentReadsScalarValuesBeforeRetainedGroups(t *testing.T) {
	label := "Ordinary retained label"
	classInterior := "\n\tname \"Pilot\"\n"
	for _, scope := range []string{"top level", "ClassInfo"} {
		t.Run(scope, func(t *testing.T) {
			interior := classInterior
			prefix := ""
			pair := "\"" + label + "\" // retain the value comment\n\t{\n\t\tmode quiet\n\t}\n"
			if scope == "top level" {
				prefix = "description " + pair
			} else {
				interior += "\tcustomField " + pair
			}
			input := prefix + "ClassInfo {" + interior + "}\n"
			assessment, err := AssessMBCHSourceBuffers(input)
			if err != nil {
				t.Fatal(err)
			}
			if len(assessment.Diagnostics) != 0 {
				t.Fatalf("a lexical value became a false bare-key diagnostic: %v", assessment.Diagnostics)
			}
			if assessment.MaxPairedValueBytes != len(label) || assessment.TotalFileBytes != len(input) || assessment.ClassInfoBytes != len(interior) {
				t.Fatalf("value/group context changed exact byte accounting: %#v; want paired=%d file=%d class=%d",
					assessment, len(label), len(input), len(interior))
			}
		})
	}
}

func TestAuditMBCHAssessmentRetainsGenuineBareScalarDiagnostics(t *testing.T) {
	for _, test := range []struct{ name, source, key string }{
		{
			"bare scalar after a retained group",
			"ClassInfo {\n\tname \"Pilot\"\n\tNested { mode quiet }\n\tmaxhealth\n}\n",
			"maxhealth",
		},
		{
			"empty scalar before a retained group",
			"ClassInfo {\n\tname \"Pilot\"\n\tcustomField \"\"\n\t{\n\t\tmode quiet\n\t}\n}\n",
			"customField",
		},
		{
			"bare top-level scalar",
			"ClassInfo {\n\tname \"Pilot\"\n}\ndescription",
			"description",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			assessment, err := AssessMBCHSourceBuffers(test.source)
			if err != nil {
				t.Fatal(err)
			}
			diagnostic := "bare recognized value or empty key: " + test.key
			if !strings.Contains(strings.Join(assessment.Diagnostics, "; "), diagnostic) {
				t.Fatalf("genuine bare/empty scalar diagnostic disappeared: %#v", assessment)
			}
			if err := ValidateSerialized(FormatMBCH, test.source, "name", "Pilot"); err == nil || !strings.Contains(err.Error(), diagnostic) {
				t.Fatalf("bulk validation ignored genuine scalar diagnostic %q: %v", diagnostic, err)
			}
		})
	}
}
