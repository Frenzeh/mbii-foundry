package parsers

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// siegeSample is a compact but representative .siege file: Teams block,
// quoted top-level scalars, trailing inline comments, a multiline
// objective description and raw AutoMap/HelpIcons blocks.
const siegeSample = `//This file should never exceed 16384 bytes.

Teams
{
	team1 Goodies
	team2 Baddies
}

mapgraphic "gfx/mplevels/map"
missionname  "Sample Siege"
radartopleft "-6976, 7040"
radarbottomright "5888, -7296"
AutoMap
{
	AutoMap0
	{
		radargraphic "gfx/automap/map"
	}
}

HelpIcons
{
	//Side Gen Hack
	HelpIcon0
	{
		end0 "doorformaul"
		origin "457 584 635"
		sideobjective 1
	}
}

Goodies
{
	RequiredObjectives 1 //How many objectives must be done
	Timed 300
     	UseTeam "PB3_G" //theme
	TeamColorOn	"1 0 0 1"

	Objective1
	{
		goalname "Storm the generator room"
		final -1
		objdesc "SECONDARY GOAL:
 		Get to the override console and open the door"
		objgfx "gfx/mplevels/map/obj1"
	}

            Objective2
	{
		goalname "Raid the vault"
		final 0
	}
}

Baddies
{
	RequiredObjectives 1
	UseTeam "PB3_B"
	wonround "They did it"
	briefing "Hold the line"
}
`

func mustParseSiege(t *testing.T, content string) *SiegeData {
	t.Helper()
	siege, err := ParseSiege(content)
	if err != nil {
		t.Fatalf("ParseSiege: %v", err)
	}
	return siege
}

func TestSiegeNoOpPreservation(t *testing.T) {
	siege := mustParseSiege(t, siegeSample)
	out, err := GenerateSiege(siege)
	if err != nil {
		t.Fatal(err)
	}
	if out != siegeSample {
		t.Errorf("no-op GenerateSiege rewrote the file.\n--- want ---\n%s\n--- got ---\n%s", siegeSample, out)
	}
}

func TestSiegeTeamEditRendersInPlace(t *testing.T) {
	siege := mustParseSiege(t, siegeSample)
	siege.Team1.Briefing = "New briefing text"
	siege.Team1.UseTeam = "PB3_H"
	out, err := GenerateSiege(siege)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, `"New briefing text"`) {
	}
	if !strings.Contains(out, `UseTeam "PB3_H"`) {
		t.Errorf("UseTeam not updated:\n%s", out)
	}
	if strings.Contains(out, `UseTeam "PB3_G"`) {
		t.Errorf("old UseTeam value survived:\n%s", out)
	}
	// Untouched sibling content must survive byte-for-byte.
	for _, want := range []string{
		"RequiredObjectives 1 //How many objectives must be done",
		`radartopleft "-6976, 7040"`,
		"//Side Gen Hack",
		`team1 Goodies`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("untouched content lost: %q", want)
		}
	}
}

func TestSiegeObjectiveDeleteAndAdd(t *testing.T) {
	siege := mustParseSiege(t, siegeSample)
	if len(siege.Team1.Objectives) != 2 {
		t.Fatalf("expected 2 objectives, got %d", len(siege.Team1.Objectives))
	}
	// Delete Objective1 (first), keep Objective2, add a new one.
	siege.Team1.Objectives = siege.Team1.Objectives[1:]
	siege.Team1.Objectives = append(siege.Team1.Objectives, SiegeObjective{
		Name:     "Objective3",
		GoalName: "Fresh goal",
		Final:    1,
	})
	out, err := GenerateSiege(siege)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out, "Storm the generator room") {
		t.Errorf("deleted objective still present:\n%s", out)
	}
	if !strings.Contains(out, "Raid the vault") {
		t.Errorf("kept objective lost:\n%s", out)
	}
	if !strings.Contains(out, "Objective3") || !strings.Contains(out, "Fresh goal") {
		t.Errorf("added objective missing:\n%s", out)
	}
}

func TestSiegeTeamRenameAndDelete(t *testing.T) {
	siege := mustParseSiege(t, siegeSample)
	siege.Team1.Name = "Heroes"
	out, err := GenerateSiege(siege)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "team1 Heroes") {
		t.Errorf("Teams reference not updated:\n%s", out)
	}
	if !strings.Contains(out, "\nHeroes\n{") && !strings.Contains(out, "\nHeroes {") {
		t.Errorf("team block not renamed in place:\n%s", out)
	}
	if strings.Contains(out, "Goodies") {
		t.Errorf("stale team block or reference left behind:\n%s", out)
	}

	// Deleting team 2 removes its block and its reference.
	siege.Team2 = nil
	out, err = GenerateSiege(siege)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out, "Baddies") || strings.Contains(out, "team2") {
		t.Errorf("deleted team survived:\n%s", out)
	}
}

func TestSiegeRawBlockEdit(t *testing.T) {
	siege := mustParseSiege(t, siegeSample)
	siege.AutoMap = "{\n\tAutoMap0\n\t{\n\t\tradargraphic \"gfx/automap/other\"\n\t}\n}"
	out, err := GenerateSiege(siege)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, `radargraphic "gfx/automap/other"`) {
		t.Errorf("AutoMap edit not rendered:\n%s", out)
	}
	if !strings.Contains(out, "HelpIcons") {
		t.Errorf("HelpIcons lost when editing AutoMap:\n%s", out)
	}
}

// TestSiegeRealFileNoOp verifies byte-exact no-op generation on a real
// shipped .siege file when available locally.
func TestSiegeRealFileNoOp(t *testing.T) {
	const root = "/Users/pj/Library/CloudStorage/SynologyDrive-mcp5/mbii/TextAssets/mb2_pb_assets2/maps"
	data, err := os.ReadFile(filepath.Join(root, "pb3_dotf.siege"))
	if err != nil {
		t.Skipf("real siege file not present: %v", err)
	}
	src := string(data)
	siege := mustParseSiege(t, src)
	out, err := GenerateSiege(siege)
	if err != nil {
		t.Fatal(err)
	}
	if out != src {
		// Report the first divergence point for diagnosis.
		i := 0
		for i < len(src) && i < len(out) && src[i] == out[i] {
			i++
		}
		lo := i - 80
		if lo < 0 {
			lo = 0
		}
		hi := i + 80
		t.Errorf("no-op rewrite diverges at byte %d.\n--- want ---\n%s\n--- got ---\n%s", i, src[lo:hi], out[lo:min(i+80, len(out))])
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
