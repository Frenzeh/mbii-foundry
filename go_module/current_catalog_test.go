package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"

	"github.com/Frenzeh/mbii-foundry/parsers"
)

// Keep these tests independent of optional local visibility overlays.
func usePublicCatalogs(t *testing.T) {
	t.Helper()
	attrs, weapons, classes := LoadedAttributes, LoadedWeapons, LoadedClasses
	flags, styles, glossary, path := LoadedClassFlags, LoadedSaberStyles, LoadedGlossary, DataPath
	hiddenAttrs, hiddenWeapons, hiddenClasses := hiddenAttributeIDs, hiddenWeaponIDs, hiddenClassIDs
	t.Cleanup(func() {
		LoadedAttributes, LoadedWeapons, LoadedClasses = attrs, weapons, classes
		LoadedClassFlags, LoadedSaberStyles, LoadedGlossary, DataPath = flags, styles, glossary, path
		hiddenAttributeIDs, hiddenWeaponIDs, hiddenClassIDs = hiddenAttrs, hiddenWeapons, hiddenClasses
	})
	LoadedAttributes, LoadedWeapons, LoadedClasses = nil, nil, nil
	hiddenClassIDs = map[string]bool{"MB_CLASS_NOCLASS": true, "MB_CLASS_OBSERVER": true}
	hiddenWeaponIDs = map[string]bool{"WP_NONE": true}
	hiddenAttributeIDs = map[string]bool{
		"MB_ATT_INVALID": true, "MB_ATT_FP_FINAL": true,
		"MB_RES_ENERGY": true, "MB_RES_STAMINA": true, "MB_RES_RAGE": true, "MB_RES_BATTERY": true,
	}
}

func currentCatalogAttribute(t *testing.T, id string) AttributeDef {
	t.Helper()
	for _, attr := range GetAllAttributes() {
		if attr.ID == id {
			return attr
		}
	}
	t.Fatalf("missing attribute %s", id)
	return AttributeDef{}
}

func hasCatalogOption(options []string, id string) bool {
	for _, option := range options {
		if option == id {
			return true
		}
	}
	return false
}

func TestCurrentCatalogPublicDataLoads(t *testing.T) {
	usePublicCatalogs(t)
	dataPath := filepath.Join("..", "data")
	readCatalog := func(name string, target any) {
		t.Helper()
		data, err := os.ReadFile(filepath.Join(dataPath, name))
		if err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(data, target); err != nil {
			t.Fatalf("%s cannot be loaded by its runtime type: %v", name, err)
		}
	}
	var attrs []AttributeDef
	var classes []ClassDef
	var weapons []WeaponDef
	readCatalog("attributes.json", &attrs)
	readCatalog("classes.json", &classes)
	readCatalog("weapons.json", &weapons)
	attrIDs := make(map[string]bool)
	for _, attr := range attrs {
		attrIDs[attr.ID] = true
	}
	for _, attr := range MBIIAttributes {
		if !attrIDs[attr.ID] {
			t.Errorf("standalone attribute %s is missing from shipped data", attr.ID)
		}
	}
	for _, id := range append(append([]string(nil), KnownForcePowerTargets...), KnownSpecialActions...) {
		if !attrIDs[id] {
			t.Errorf("selectable power/action %s is missing from shipped data", id)
		}
	}
	classIDs := make(map[string]bool)
	for _, class := range classes {
		classIDs[class.ID] = true
	}
	for _, class := range MBIIClasses {
		if !classIDs[class.ID] {
			t.Errorf("standalone class %s is missing from shipped data", class.ID)
		}
	}
	weaponIDs := make(map[string]bool)
	for _, weapon := range weapons {
		weaponIDs[weapon.ID] = true
	}
	for _, weapon := range MBIIWeapons {
		if !weaponIDs[weapon.ID] {
			t.Errorf("standalone weapon %s is missing from shipped data", weapon.ID)
		}
	}
	if err := LoadExternalData(dataPath); err != nil {
		t.Fatal(err)
	}
	cycler := currentCatalogAttribute(t, "MB_ATT_CYCLER")
	if cycler.Name != "Tusken Cycler Tuning" || cycler.Levels["1"].Effect == "" {
		t.Fatalf("external metadata was not merged: %+v", cycler)
	}
	if got := currentCatalogAttribute(t, "MB_ATT_CYBERNETICS").MaxLevel; got != 0 {
		t.Fatalf("undocumented gameplay rank limit was invented: %d", got)
	}
	if got := currentCatalogAttribute(t, "MB_ATT_UGL_BURST").MaxLevel; got != 21 {
		t.Fatalf("documented UGL burst count limit = %d, want 21", got)
	}
	for _, retired := range []string{"MB_ATT_FA3_FLECHETE", "MB_ATT_LASERCOVER", "MB_ATT_IMP_PISTOL", "MB_ATT_QUICKTOSS", "MB_ATT_THERMAL", "FP_ATTUNEMENT", "FP_MINDTRICK", "FP_SABER_THROW"} {
		if attrIDs[retired] {
			t.Errorf("retired identifier remains selectable: %s", retired)
		}
	}
}

func TestCurrentCatalogSelectionPreservesAuthoredValues(t *testing.T) {
	usePublicCatalogs(t)
	useEditorTestApp(t)
	const original = "FUTURE_ATTRIBUTE,0|MB_ATT_STAMINA|X_CUSTOM,007|MB_ATT_FA3_FLECHETTE,1|HI_MEDPAC|MB_ATT_TEMP_CLOAK,0"
	ch, err := parsers.ParseMBCH("ClassInfo\n{\nname \"Catalog regression\"\nMBClass MB_CLASS_FUTURE\nattributes \"" + original + "\"\nfutureField \"opaque value\"\n}\n")
	if err != nil {
		t.Fatal(err)
	}
	picker := NewClassIconPicker(func(id string) { ch.MBClass = id })
	picker.SetSelected(ch.MBClass)
	if picker.Selected() != "MB_CLASS_FUTURE" {
		t.Fatal("loading an unknown class changed its identifier")
	}
	for _, id := range []string{"MB_CLASS_SHOCKTROOPER", "MB_CLASS_OPERATIVE", "MB_CLASS_ENFORCER", "MB_CLASS_SPY", "MB_CLASS_ASTRODROID", "MB_CLASS_ASTROMECH", "MB_CLASS_JETTROOPER", "MB_CLASS_ASSASSINDROID", "MB_CLASS_PATROLIAN"} {
		found := false
		for _, card := range picker.cards {
			if card.def.ID == id {
				test.Tap(card)
				found = true
				break
			}
		}
		if !found || picker.Selected() != id || ch.MBClass != id {
			t.Fatalf("class card did not select canonical identifier %s", id)
		}
	}
	ag := &AttributeGrid{onChange: func(s string) { ch.Attributes = s }}
	ag.setRaw(ch.Attributes)
	if ag.values["MB_ATT_STAMINA"] != 3 {
		t.Fatal("bare attributes must display the engine default level 3")
	}
	control := ag.createAttributeItem(currentCatalogAttribute(t, "MB_ATT_CYBERNETICS")).(*AttributeToggleWidget)
	if control.levelEntry == nil {
		t.Fatal("undocumented ranks must use a raw numeric control, not guessed rank pills")
	}
	control.levelEntry.SetText("4")
	want := original + "|MB_ATT_CYBERNETICS,4"
	if ch.Attributes != want {
		t.Fatalf("unrelated raw tokens changed:\n got %q\nwant %q", ch.Attributes, want)
	}
	control.levelEntry.SetText("256")
	if ch.Attributes != want || control.levelEntry.Validate() == nil {
		t.Fatal("out-of-byte-range input must not change the authored attributes")
	}
	control.levelEntry.SetText("0")
	if ch.Attributes != original {
		t.Fatalf("removing one attribute changed unrelated tokens: %q", ch.Attributes)
	}
	control.levelEntry.SetText("255")
	want = original + "|MB_ATT_CYBERNETICS,255"
	if ch.Attributes != want {
		t.Fatalf("valid parser-boundary value was not selected: %q", ch.Attributes)
	}
	flechette := ag.createAttributeItem(currentCatalogAttribute(t, "MB_ATT_FA3_FLECHETTE")).(*AttributeToggleWidget)
	flechette.levelEntry.SetText("2")
	want = strings.Replace(want, "MB_ATT_FA3_FLECHETTE,1", "MB_ATT_FA3_FLECHETTE,2", 1)
	if ch.Attributes != want {
		t.Fatalf("canonical flechette selection changed unrelated tokens: %q", ch.Attributes)
	}
	source, err := parsers.GenerateMBCH(ch)
	if err != nil {
		t.Fatal(err)
	}
	reopened, err := parsers.ParseMBCH(source)
	if err != nil {
		t.Fatal(err)
	}
	if reopened.MBClass != ch.MBClass || reopened.Attributes != want || reopened.ExtraFields["futureField"] != "opaque value" {
		t.Fatalf("catalog selection did not survive round-trip without unrelated loss: %+v", reopened)
	}
}

func TestCurrentCatalogSharedWeaponRankSurvivesRemoval(t *testing.T) {
	usePublicCatalogs(t)
	useEditorTestApp(t)
	for _, tc := range []struct {
		name, first, second, attribute string
		numeric                        bool
	}{
		{"numeric", "WP_WELD_PULSE", "WP_WELD_BEAM", "MB_ATT_WELDING_LASER", true},
		{"pills", "WP_BRYAR_PISTOL", "WP_BLASTER_PISTOL", "MB_ATT_PISTOL", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			const unrelated = "MB_ATT_STAMINA|X_CUSTOM,007|FUTURE_ATTRIBUTE,0|HI_MEDPAC"
			original := unrelated + "|" + tc.attribute + ",2"
			e := NewMBCHEditor(&App{})
			source := "ClassInfo\n{\nname \"Shared weapon rank\"\nweapons \"" + tc.first + "|" + tc.second + "\"\nattributes \"" + original + "\"\n}\n"
			if err := e.ApplySourceText(source); err != nil {
				t.Fatal(err)
			}
			removeWeapon := func(id string) {
				t.Helper()
				for _, weapon := range GetWeapons() {
					if weapon.ID != id {
						continue
					}
					control := e.weaponGrid.buildLevelPills(id, CanonicalAttributeFor(id), weapon)
					if tc.numeric {
						row := control.(*fyne.Container).Objects[0].(*fyne.Container)
						entry := row.Objects[1].(*fyne.Container).Objects[0].(*widget.Entry)
						if entry.Text != "2" {
							t.Fatalf("%s displayed rank %q, want 2", id, entry.Text)
						}
						entry.SetText("0")
					} else {
						if saveDialogButton(t, control, "2").Importance != widget.HighImportance {
							t.Fatalf("%s did not display rank 2 as selected", id)
						}
						test.Tap(saveDialogButton(t, control, "Off"))
					}
					return
				}
				t.Fatalf("shared-rank weapon %s is not selectable in the public catalog", id)
			}
			assertSource := func(wantWeapons, wantAttributes string) {
				t.Helper()
				reopened, err := parsers.ParseMBCH(e.CurrentSource())
				if err != nil {
					t.Fatal(err)
				}
				if reopened.Weapons != wantWeapons {
					t.Fatalf("weapons after removal = %q, want %q", reopened.Weapons, wantWeapons)
				}
				if reopened.Attributes != wantAttributes {
					t.Fatalf("attributes after removal = %q, want %q", reopened.Attributes, wantAttributes)
				}
			}

			removeWeapon(tc.first)
			assertSource(tc.second, original)
			removeWeapon(tc.second)
			assertSource("", unrelated)
		})
	}
}

func TestCurrentCatalogSpecialActionSelection(t *testing.T) {
	usePublicCatalogs(t)
	useEditorTestApp(t)
	ch := parsers.NewMBCHCharacter()
	ch.ExtraFields = map[string]string{"special1": "EAS_FUTURE", "special2": "EAS_OTHER_FUTURE", "futureField": "keep"}
	e := &MBCHEditor{character: ch, loading: true, devFieldEntries: make(map[string]*widget.Entry)}
	card := e.buildDevFieldsCard()
	e.populateDevFieldsFromCharacter()
	form := card.Content.(*widget.Form)
	var action *widget.SelectEntry
	for _, item := range form.Items {
		if item.Text == "Special 1" {
			action, _ = item.Widget.(*widget.SelectEntry)
		}
	}
	if action == nil || action.Text != "EAS_FUTURE" {
		t.Fatal("special-action picker must retain an unknown loaded action")
	}
	if !hasCatalogOption(KnownSpecialActions, "EAS_HI_OVERCLOCK") {
		t.Fatal("Overclock is not offered by the action catalog")
	}
	action.SetText("EAS_HI_OVERCLOCK")
	if ch.ExtraFields["special1"] != "EAS_HI_OVERCLOCK" || ch.ExtraFields["special2"] != "EAS_OTHER_FUTURE" || ch.ExtraFields["futureField"] != "keep" {
		t.Fatalf("action selection changed unrelated fields: %v", ch.ExtraFields)
	}
	if RequiredAttributeForEAS(action.Text) != "MB_ATT_CYBERNETICS" {
		t.Fatal("Overclock must reference the current engine attribute")
	}
}

func TestCurrentCatalogOverrideSelectionPreservesUnknowns(t *testing.T) {
	usePublicCatalogs(t)
	useEditorTestApp(t)
	ch := parsers.NewMBCHCharacter()
	ch.ForceOverrides = []parsers.ForceInfo{{ForceToReplace: "FP_FUTURE", ExtraFields: map[string]string{"gripDamage": "0", "futureForceField": "keep"}}}
	ch.WeaponOverrides = []parsers.WeaponInfo{{WeaponToReplace: "WP_FUTURE", WeaponBasedOff: "WP_FUTURE_BASE", ExtraFields: map[string]string{"primFireEnabled": "0", "damageMod": "1.25", "futureWeaponField": "keep"}}}
	e := &MBCHEditor{character: ch, loading: true}
	force := NewForceInfoUI(e)
	force.currentForceIndex = 0
	force.loadForceDetails(0)
	force.forcePowerNameEntry.SetText("Changed display name")
	if ch.ForceOverrides[0].ForceToReplace != "FP_FUTURE" {
		t.Fatal("editing force metadata discarded the unknown target")
	}
	for _, id := range []string{"FP_SUMMON", "FP_BATTLEMED", "FP_DOMINATION", "FP_DARKRAGE"} {
		force.forceToReplaceSelect.SetSelected(id)
		if ch.ForceOverrides[0].ForceToReplace != id {
			t.Fatalf("force picker did not emit %s", id)
		}
	}
	if hasCatalogOption(KnownForcePowerTargets, "FP_REPULSE") {
		t.Fatal("an action-only identifier leaked into the Force target catalog")
	}
	weapon := NewWeaponInfoUI(e)
	weapon.currentWeaponIndex = 0
	weapon.loadWeaponDetails(0)
	weapon.weaponNameEntry.SetText("Changed weapon display name")
	if ch.WeaponOverrides[0].WeaponToReplace != "WP_FUTURE" || ch.WeaponOverrides[0].WeaponBasedOff != "WP_FUTURE_BASE" {
		t.Fatal("editing weapon metadata discarded unknown targets")
	}
	weapon.weaponToReplaceSelect.SetSelected("WP_WELD_PULSE")
	if ch.WeaponOverrides[0].WeaponToReplace != "WP_WELD_PULSE" || ch.WeaponOverrides[0].WeaponBasedOff != "WP_FUTURE_BASE" {
		t.Fatal("weapon selection did not preserve the unrelated base identifier")
	}
	if ch.ForceOverrides[0].ExtraFields["gripDamage"] != "0" || ch.ForceOverrides[0].ExtraFields["futureForceField"] != "keep" || ch.WeaponOverrides[0].ExtraFields["primFireEnabled"] != "0" || ch.WeaponOverrides[0].ExtraFields["damageMod"] != "1.25" || ch.WeaponOverrides[0].ExtraFields["futureWeaponField"] != "keep" {
		t.Fatal("override form selection discarded raw runtime fields")
	}
}

func TestCurrentCatalogDomainsAndVisibility(t *testing.T) {
	usePublicCatalogs(t)
	hiddenClassIDs["MB_CLASS_OPERATIVE"] = true
	hiddenWeaponIDs["WP_WELD_PULSE"] = true
	hiddenAttributeIDs["MB_ATT_CYBERNETICS"] = true
	hiddenAttributeIDs["FP_SUMMON"] = true
	checkVisible := func() {
		t.Helper()
		if hasCatalogOption(forcePowerOptions(), "FP_SUMMON") {
			t.Error("hidden force target leaked into picker")
		}
		for _, class := range GetClasses() {
			if class.ID == "MB_CLASS_OPERATIVE" || class.ID == "MB_CLASS_OBSERVER" {
				t.Errorf("hidden class leaked into picker: %s", class.ID)
			}
		}
		for _, attr := range GetAttributes() {
			if attr.ID == "MB_ATT_CYBERNETICS" {
				t.Error("hidden attribute leaked into picker")
			}
		}
		for _, id := range append(weaponPickOptions(), weaponFlagTargets()...) {
			if id == "WP_WELD_PULSE" || id == "WP_EMPLACED_GUN" || id == "WP_TURRET" || id == "WP_NONE" {
				t.Errorf("hidden or nonplayer weapon leaked into picker: %s", id)
			}
		}
	}
	checkVisible()
	if err := LoadExternalData(filepath.Join("..", "data")); err != nil {
		t.Fatal(err)
	}
	checkVisible()
	for _, id := range pointBuyAttrOptions() {
		if !isAttributeProper(id) {
			t.Errorf("non-attribute identifier leaked into point-buy skills: %s", id)
		}
	}
	for _, id := range KnownSpecialActions {
		if !strings.HasPrefix(id, "EAS_") || id == "EAS_MAX" {
			t.Errorf("invalid action catalog member: %s", id)
		}
	}
	for weapon, key := range map[string]string{"WP_CYCLER": "WP_cyclerFlags", "WP_FA3_FLECHETTE": "WP_fa3Flags", "WP_BRYAR_PISTOL": "WP_BlasterPistolFlags"} {
		if WeaponFlagKeyForWeapon(weapon) != key || weaponIDFromFlagsKey(key) != weapon {
			t.Errorf("weapon flag field does not round-trip: %s / %s", weapon, key)
		}
	}
}
