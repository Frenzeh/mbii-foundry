package main

import (
	"strings"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"github.com/Frenzeh/mbii-foundry/parsers"
)

func TestBufferGaugeCalculation(t *testing.T) {
	char := parsers.NewMBCHCharacter()
	char.Name = "Test Character"
	char.MBClass = "MB_CLASS_CLONETROOPER"
	char.MaxHealth = 100
	char.MaxArmor = 100

	status := CalculateBufferStatus(char)
	if status.ClassInfoLen <= 0 {
		t.Fatalf("expected positive ClassInfoLen, got %d", status.ClassInfoLen)
	}
	if status.IsExceeded {
		t.Fatalf("expected status not exceeded for minimal character")
	}

	// Inflate character to test warning/exceeded threshold
	char.Description = strings.Repeat("A", 8000)
	status = CalculateBufferStatus(char)
	if status.ClassInfoLen < 100 {
		t.Fatalf("expected valid measurement")
	}
}

func TestRGBPresetValues(t *testing.T) {
	if len(MBIIRGBPresets) < 8 {
		t.Fatalf("expected at least 8 RGB presets, got %d", len(MBIIRGBPresets))
	}

	// Verify Red preset
	var redPreset *RGBPreset
	for _, p := range MBIIRGBPresets {
		if p.Name == "Red" {
			redPreset = &p
			break
		}
	}
	if redPreset == nil {
		t.Fatalf("missing Red preset")
	}
	if redPreset.R != 0.709 || redPreset.G != 0.120 || redPreset.B != 0.120 {
		t.Errorf("incorrect Red floats: %f, %f, %f", redPreset.R, redPreset.G, redPreset.B)
	}

	str := FormatRGBFloat(redPreset.R)
	if str != "0.709" {
		t.Errorf("expected 0.709, got %s", str)
	}
}

func TestStandardDescriptionGeneration(t *testing.T) {
	char := parsers.NewMBCHCharacter()
	char.Name = "Commander Cody"
	char.Weapons = "WP_CLONE_PISTOL|WP_CLONE_RIFLE"
	char.Attributes = "MB_ATT_CLONERIFLE,3|MB_ATT_STAMINA,2"
	char.ExtraFields = map[string]string{
		"holdables": "HI_MEDPAC",
	}

	desc := GenerateStandardDescription(char)
	if !strings.Contains(desc, "Commander Cody") {
		t.Errorf("expected description to contain name")
	}
	if !strings.Contains(desc, "^2Weaponry:") {
		t.Errorf("expected description to contain ^2Weaponry:")
	}
	if !strings.Contains(desc, "^6Inventory:") {
		t.Errorf("expected description to contain ^6Inventory:")
	}
	if !strings.Contains(desc, "^8Attributes:") {
		t.Errorf("expected description to contain ^8Attributes:")
	}
	if !strings.Contains(desc, "Clone Rifle") {
		t.Errorf("expected Clone Rifle in description, got: %s", desc)
	}
	if !strings.Contains(desc, "Bacta Canister") {
		t.Errorf("expected Bacta Canister in description, got: %s", desc)
	}
	if !strings.Contains(desc, "DC-15A Training") {
		t.Errorf("expected DC-15A Training in description, got: %s", desc)
	}
}

func TestDescriptionAccuracy(t *testing.T) {
	char := parsers.NewMBCHCharacter()
	char.Name = "Luke Skywalker"
	char.Weapons = "WP_SABER|WP_BLASTER_PISTOL"
	char.ForcePowers = "FP_LEVITATION,3|FP_TELEPATHY,2|FP_SEE,1|FP_HEAL,2"
	char.SaberStyle = "3" // Fast (1) + Medium (2)
	char.Attributes = "MB_ATT_MAGNETIC_PLATING,1|MB_ATT_FORCEBLOCK,2"

	desc := GenerateStandardDescription(char)

	// Verify Lightsaber (NOT Saber)
	if !strings.Contains(desc, "Lightsaber") {
		t.Errorf("expected WP_SABER to produce 'Lightsaber', got: %s", desc)
	}
	// Verify Force Jump (NOT Levitation)
	if !strings.Contains(desc, "Force Jump (3)") {
		t.Errorf("expected FP_LEVITATION to produce 'Force Jump (3)', got: %s", desc)
	}
	// Verify Mind Trick (NOT Telepathy)
	if !strings.Contains(desc, "Mind Trick (2)") {
		t.Errorf("expected FP_TELEPATHY to produce 'Mind Trick (2)', got: %s", desc)
	}
	// Verify Force Sense (NOT See)
	if !strings.Contains(desc, "Force Sense (1)") {
		t.Errorf("expected FP_SEE to produce 'Force Sense (1)', got: %s", desc)
	}
	// Verify Saber Styles
	if !strings.Contains(desc, "Fast (Cyan) / Medium (Yellow)") {
		t.Errorf("expected saber styles to be listed, got: %s", desc)
	}
}


func TestRelationsMapping(t *testing.T) {
	// Test primary weapon mapping
	if att := CanonicalAttributeFor("WP_M5"); att != "MB_ATT_WESTARM5" {
		t.Errorf("expected MB_ATT_WESTARM5, got %s", att)
	}

	// Test secondary weapon attributes
	secondaries := RelatedAttributesForWeapon("WP_CLONE_RIFLE")
	if len(secondaries) < 3 {
		t.Errorf("expected at least 3 attributes for WP_CLONE_RIFLE (primary + blobs), got %d", len(secondaries))
	}

	// Test EAS item link
	if att := RequiredAttributeForEAS("EAS_HI_GRAPPLEHOOK"); att != "MB_ATT_GRAPPLE_HOOK" {
		t.Errorf("expected MB_ATT_GRAPPLE_HOOK, got %s", att)
	}
}

func TestVFSGameDataPortraits(t *testing.T) {
	testApp := test.NewApp()
	defer testApp.Quit()

	gamedata := "/Users/pj/Library/CloudStorage/SynologyDrive-mcp5/MBII_GameData"
	vfs := NewVirtualFileSystem(gamedata, "")
	if err := vfs.Refresh(); err != nil {
		t.Fatalf("failed to refresh VFS: %v", err)
	}

	if len(vfs.Index) == 0 {
		t.Skip("GameData PK3s not present, skipping integration check")
	}

	// Verify portrait candidate resolution for clonerc2
	ab := NewAssetBrowser(gamedata, "")
	ab.vfs = vfs

	res := ab.LoadIconResource("models/players/clonerc2/mb2_icon_rgb")
	if res == nil {
		t.Errorf("expected to resolve models/players/clonerc2/mb2_icon_rgb from PK3s")
	}

	// Verify portrait candidate resolution for baby_yoda
	ir := NewIconResolver(vfs)
	candidates := ir.ResolveClassIconCandidates("baby_yoda", "default", "")
	var yodaRes fyne.Resource
	for _, c := range candidates {
		if r := ab.LoadIconResource(c); r != nil {
			yodaRes = r
			break
		}
	}
	if yodaRes == nil {
		t.Errorf("expected to resolve baby_yoda portrait candidate from PK3s, tried %v", candidates)
	}

	// Verify canonical default skin resolution for clonetrooper_p1
	cloneCandidates := ir.ResolveClassIconCandidates("clonetrooper_p1", "default", "")
	var cloneRes fyne.Resource
	for _, c := range cloneCandidates {
		if r := ab.LoadIconResource(c); r != nil {
			cloneRes = r
			break
		}
	}
	if cloneRes == nil {
		t.Errorf("expected to resolve clonetrooper_p1 default portrait candidate from PK3s, tried %v", cloneCandidates)
	}

	// Verify portrait candidate resolution for jedi_zf with merc_et
	zfCandidates := ir.ResolveClassIconCandidates("jedi_zf", "merc_et", "models/players/jedi_zf/mb2_icon_merc_et")
	var zfRes fyne.Resource
	for _, c := range zfCandidates {
		if r := ab.LoadIconResource(c); r != nil {
			zfRes = r
			break
		}
	}
	if zfRes == nil {
		t.Errorf("expected to resolve jedi_zf merc_et portrait candidate from PK3s, tried %v", zfCandidates)
	}

	// Verify portrait candidate resolution for jedi_zf with head_a1
	zfHeadCandidates := ir.ResolveClassIconCandidates("jedi_zf", "head_a1", "")
	var zfHeadRes fyne.Resource
	for _, c := range zfHeadCandidates {
		if r := ab.LoadIconResource(c); r != nil {
			zfHeadRes = r
			break
		}
	}
	if zfHeadRes == nil {
		t.Errorf("expected to resolve jedi_zf head_a1 portrait candidate from PK3s, tried %v", zfHeadCandidates)
	}
}

func TestDefensiveMatrixDR(t *testing.T) {
	char := parsers.NewMBCHCharacter()
	char.MaxHealth = 100
	char.MaxArmor = 100
	char.Attributes = "MB_ATT_MAGNETIC_PLATING,1|MB_ATT_BLAST_ARMOUR,1|MB_ATT_CORTOSIS,1"
	char.ExtraFields = map[string]string{
		"reinforcements": "2", // 3 lives total
	}

	dm := CalculateDefensiveMatrix(char)

	if dm.TotalLives != 3 {
		t.Errorf("expected 3 total lives, got %d", dm.TotalLives)
	}
	if dm.RawPool != 200 {
		t.Errorf("expected RawPool 200, got %d", dm.RawPool)
	}
	if dm.TotalRawEHP != 600 {
		t.Errorf("expected TotalRawEHP 600, got %d", dm.TotalRawEHP)
	}

	// 40% Energy DR -> RawPool (200) / 0.60 * 3 lives = 1000 Energy EHP
	if dm.EnergyEHP < 990 || dm.EnergyEHP > 1010 {
		t.Errorf("expected ~1000 Energy EHP with Mag Plating, got %d", dm.EnergyEHP)
	}

	// 40% Explosive DR -> RawPool (200) / 0.60 * 3 lives = 1000 Explosive EHP
	if dm.ExplosiveEHP < 990 || dm.ExplosiveEHP > 1010 {
		t.Errorf("expected ~1000 Explosive EHP with Blast Armour, got %d", dm.ExplosiveEHP)
	}

	if dm.HasCortosis != 1 {
		t.Errorf("expected Cortosis level 1")
	}
}

func TestCustomSkillsSync(t *testing.T) {
	char := parsers.NewMBCHCharacter()
	char.ExtraFields = map[string]string{
		"c_att_skill_0": "MB_ATT_DASH",
		"c_att_names_0": "^3Dash",
		"c_att_ranks_0": "2,2",
		"c_att_descs_0": "^7Level 1: Quick Dash\n^7Level 2: Long Dash",
	}

	cse := NewCustomSkillsEditor(char, nil)
	if len(cse.skills) != 1 {
		t.Fatalf("expected 1 custom skill parsed, got %d", len(cse.skills))
	}

	skill := cse.skills[0]
	if skill.Skill != "MB_ATT_DASH" || skill.Name != "^3Dash" {
		t.Errorf("unexpected skill fields: %+v", skill)
	}

	// Add a second skill
	cse.skills = append(cse.skills, &CustomSkillItem{
		Index: 1,
		Skill: "MB_ATT_STAMINA",
		Name:  "^2Stamina Boost",
		Ranks: "1,1",
		Descs: "^7Level 1: +20 Stamina",
	})
	cse.SyncToCharacter()

	if char.ExtraFields["c_att_skill_1"] != "MB_ATT_STAMINA" {
		t.Errorf("expected c_att_skill_1 to be synced, got %s", char.ExtraFields["c_att_skill_1"])
	}
	if char.ExtraFields["c_att_names_1"] != "^2Stamina Boost" {
		t.Errorf("expected c_att_names_1 to be synced, got %s", char.ExtraFields["c_att_names_1"])
	}
}


