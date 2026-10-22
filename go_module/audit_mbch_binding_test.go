package main

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"
	"github.com/Frenzeh/mbii-foundry/parsers"
)

const auditMBCHCanonicalFixture = `ClassInfo
{
	name "Canonical skills"
	MBClass MB_CLASS_JEDI
	isCustomBuild 1
	mbPoints 100
	hasCustomSpec 3
	c_att_skill_0 MB_ATT_PUSH
	c_att_names_0 "^3Push"
	c_att_ranks_0 0,4,10
	c_att_descs_0 "Push description"
	c_att_skill_2 MB_ATT_DASH
	c_att_names_2 "^2Dash"
	c_att_ranks_2 2,5
	c_att_descs_2 "Dash description"
	c_att_ranks_6 7,11
	c_att_descs_14 "Description-only slot"
	c_att_skill_17 MB_ATT_PULL
	c_att_names_17 "^5Pull"
	c_att_ranks_17 1,3,7
	c_att_skill_44 MB_ATT_STAMINA
	c_att_names_44 "Last slot"
	c_att_ranks_44 3,6
}
`

func loadAuditMBCH(t *testing.T, source string) *MBCHEditor {
	t.Helper()
	useEditorTestApp(t)
	path := filepath.Join(t.TempDir(), "fixture.mbch")
	if err := os.WriteFile(path, []byte(source), 0644); err != nil {
		t.Fatal(err)
	}
	e := NewMBCHEditor(nil)
	t.Cleanup(e.session.flushPending)
	if err := e.LoadFile(path); err != nil {
		t.Fatal(err)
	}
	return e
}

func parseAuditMBCH(t *testing.T, source string) *parsers.MBCHCharacter {
	t.Helper()
	ch, err := parsers.ParseMBCH(source)
	if err != nil {
		t.Fatal(err)
	}
	return ch
}

func walkAuditMBCH(root fyne.CanvasObject, visit func(fyne.CanvasObject)) {
	visit(root)
	if c, ok := root.(*fyne.Container); ok {
		for _, child := range c.Objects {
			walkAuditMBCH(child, visit)
		}
	}
}

func auditMBCHEntries(root fyne.CanvasObject) []*widget.Entry {
	var entries []*widget.Entry
	walkAuditMBCH(root, func(obj fyne.CanvasObject) {
		if entry, ok := obj.(*widget.Entry); ok {
			entries = append(entries, entry)
		}
	})
	return entries
}

func auditMBCHButton(t *testing.T, root fyne.CanvasObject, text string) *widget.Button {
	t.Helper()
	var found *widget.Button
	walkAuditMBCH(root, func(obj fyne.CanvasObject) {
		if button, ok := obj.(*widget.Button); ok && button.Text == text {
			found = button
		}
	})
	if found == nil {
		t.Fatalf("button %q missing from %T", text, root)
	}
	return found
}

func auditMBCHSkillCard(t *testing.T, e *MBCHEditor, index int) fyne.CanvasObject {
	t.Helper()
	for i, skill := range e.customSkillsUI.skills {
		if skill.Index == index {
			return e.customSkillsUI.cardList.Objects[i]
		}
	}
	t.Fatalf("canonical custom-skill slot %d is absent from the form", index)
	return nil
}

func assertAuditMBCHSkills(t *testing.T, got, want *parsers.MBCHCharacter) {
	t.Helper()
	if got.CustomSkills != want.CustomSkills || got.CustomNames != want.CustomNames ||
		got.CustomRanks != want.CustomRanks || got.CustomDescs != want.CustomDescs {
		t.Fatalf("canonical skill arrays changed unexpectedly:\n got skills=%q names=%q ranks=%q descs=%q\nwant skills=%q names=%q ranks=%q descs=%q",
			got.CustomSkills, got.CustomNames, got.CustomRanks, got.CustomDescs,
			want.CustomSkills, want.CustomNames, want.CustomRanks, want.CustomDescs)
	}
	for key := range got.ExtraFields {
		if strings.HasPrefix(strings.ToLower(key), "c_att_") {
			t.Fatalf("canonical custom-skill key %q acquired competing extra-field ownership", key)
		}
	}
}

func TestAuditMBCHCustomSkillsCanonicalRoundTrip(t *testing.T) {
	e := loadAuditMBCH(t, auditMBCHCanonicalFixture)
	want := parseAuditMBCH(t, auditMBCHCanonicalFixture)
	assertAuditMBCHSkills(t, parseAuditMBCH(t, e.GenerateSource()), want)
	if len(e.customSkillsUI.skills) != 6 {
		t.Fatalf("loaded custom-skill cards = %d, want six occupied canonical slots", len(e.customSkillsUI.skills))
	}
	entries := auditMBCHEntries(auditMBCHSkillCard(t, e, 0))
	if len(entries) != 4 || entries[0].Text != "^3Push" || entries[2].Text != "0,4,10" {
		t.Fatal("parsed canonical skill did not populate its editable form")
	}
	entries[0].SetText("^2Edited Push")
	want.CustomNames[0] = "^2Edited Push"
	assertAuditMBCHSkills(t, parseAuditMBCH(t, e.GenerateSource()), want)
	if !e.IsDirty() || e.pointBuyUI.slotNames[0].Text != want.CustomNames[0] {
		t.Fatal("custom-skill form edit did not dirty the document and update Point Buy")
	}
	if !e.Undo() || e.IsDirty() || auditMBCHEntries(auditMBCHSkillCard(t, e, 0))[0].Text != "^3Push" {
		t.Fatal("custom-skill edit did not undo to the loaded clean form")
	}
	if !e.Redo() || !e.IsDirty() {
		t.Fatal("custom-skill edit did not redo")
	}
	assertAuditMBCHSkills(t, parseAuditMBCH(t, e.GenerateSource()), want)

	e.pointBuyUI.slotNames[17].SetText("Point Buy edit")
	want.CustomNames[17] = "Point Buy edit"
	if auditMBCHEntries(auditMBCHSkillCard(t, e, 17))[0].Text != want.CustomNames[17] {
		t.Fatal("Point Buy edit left the Custom Skills projection stale")
	}
	auditMBCHEntries(auditMBCHSkillCard(t, e, 0))[2].SetText("0,7,13")
	want.CustomRanks[0] = "0,7,13"
	assertAuditMBCHSkills(t, parseAuditMBCH(t, e.GenerateSource()), want)
}

func TestAuditMBCHCustomSkillAddDeletePreserveSlotIndices(t *testing.T) {
	e := loadAuditMBCH(t, auditMBCHCanonicalFixture)
	want := parseAuditMBCH(t, auditMBCHCanonicalFixture)
	customForm := e.customSkillsUI.BuildUI()
	test.Tap(auditMBCHButton(t, auditMBCHSkillCard(t, e, 2), ""))
	want.CustomSkills[2], want.CustomNames[2], want.CustomRanks[2], want.CustomDescs[2] = "", "", "", ""
	assertAuditMBCHSkills(t, parseAuditMBCH(t, e.GenerateSource()), want)
	e.session.flushPending()

	test.Tap(auditMBCHButton(t, customForm, "Add Custom Skill"))
	added := parseAuditMBCH(t, e.GenerateSource())
	if added.CustomSkills[1] == "" || added.CustomNames[1] == "" {
		t.Fatal("Add did not use the first empty canonical slot")
	}
	want.CustomSkills[1] = added.CustomSkills[1]
	want.CustomNames[1] = added.CustomNames[1]
	want.CustomRanks[1] = added.CustomRanks[1]
	want.CustomDescs[1] = added.CustomDescs[1]
	assertAuditMBCHSkills(t, added, want)
	if !e.Undo() || e.character.CustomSkills[1] != "" || e.character.CustomSkills[2] != "" {
		t.Fatal("Add did not undo independently without moving the deleted slot")
	}
	if !e.Undo() || e.IsDirty() {
		t.Fatal("Delete did not undo to the clean loaded document")
	}
	assertAuditMBCHSkills(t, parseAuditMBCH(t, e.GenerateSource()), parseAuditMBCH(t, auditMBCHCanonicalFixture))
}

func TestAuditMBCHFrontAndMiddleSkillReplacementPreservesDocumentHistory(t *testing.T) {
	for _, tc := range []struct {
		name  string
		index int
	}{
		{"front", 0},
		{"middle", 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e := loadAuditMBCH(t, auditMBCHCanonicalFixture)
			initial := parseAuditMBCH(t, auditMBCHCanonicalFixture)
			check := func(want *parsers.MBCHCharacter) {
				t.Helper()
				assertAuditMBCHSkills(t, parseAuditMBCH(t, e.GenerateSource()), want)
			}
			test.Tap(auditMBCHButton(t, auditMBCHSkillCard(t, e, tc.index), ""))
			deleted := *initial
			deleted.CustomSkills[tc.index], deleted.CustomNames[tc.index] = "", ""
			deleted.CustomRanks[tc.index], deleted.CustomDescs[tc.index] = "", ""
			check(&deleted)
			e.session.flushPending()

			e.pointBuyUI.slotNames[17].SetText("Survivor after deletion")
			e.pointBuyUI.slotRanks[17].SetText("2,6,12")
			e.pointBuyUI.slotDescs[17].SetText("Retained slot description")
			edited := deleted
			edited.CustomNames[17] = "Survivor after deletion"
			edited.CustomRanks[17] = "2,6,12"
			edited.CustomDescs[17] = "Retained slot description"
			check(&edited)
			e.session.flushPending()
			for _, want := range []*parsers.MBCHCharacter{&deleted, initial} {
				if !e.Undo() {
					t.Fatal("survivor edit and deletion must undo independently")
				}
				check(want)
			}
			if e.IsDirty() {
				t.Fatal("undoing deletion did not restore the clean parsed document")
			}
			for _, want := range []*parsers.MBCHCharacter{&deleted, &edited} {
				if !e.Redo() {
					t.Fatal("deletion and survivor edit must redo independently")
				}
				check(want)
			}

			// Reinserting as a header moves Name from the skill form's second
			// row to its only row; changing back reintroduces the other fields.
			e.pointBuyUI.slotModes[tc.index].SetSelected("Header")
			e.pointBuyUI.slotNames[tc.index].SetText("Reinserted section")
			header := edited
			header.CustomSkills[tc.index] = "MB_ATT_INVALID"
			header.CustomNames[tc.index] = "Reinserted section"
			header.CustomRanks[tc.index] = "-1"
			check(&header)
			e.session.flushPending()

			e.pointBuyUI.slotModes[tc.index].SetSelected("Skill")
			auditMBCHEntries(auditMBCHSkillCard(t, e, tc.index))[1].SetText(initial.CustomSkills[tc.index])
			e.pointBuyUI.slotNames[tc.index].SetText("Reinserted skill")
			e.pointBuyUI.slotRanks[tc.index].SetText("1,4,9")
			e.pointBuyUI.slotDescs[tc.index].SetText("Reinserted description")
			reinserted := header
			reinserted.CustomSkills[tc.index] = initial.CustomSkills[tc.index]
			reinserted.CustomNames[tc.index] = "Reinserted skill"
			reinserted.CustomRanks[tc.index] = "1,4,9"
			reinserted.CustomDescs[tc.index] = "Reinserted description"
			check(&reinserted)
			e.session.flushPending()
			for _, want := range []*parsers.MBCHCharacter{&header, &edited} {
				if !e.Undo() {
					t.Fatal("reinserted skill and header must undo independently")
				}
				check(want)
			}
			for _, want := range []*parsers.MBCHCharacter{&header, &reinserted} {
				if !e.Redo() {
					t.Fatal("header and reinserted skill must redo independently")
				}
				check(want)
			}
		})
	}
}

func TestAuditMBCHRankModifierFormOwnsParsedValue(t *testing.T) {
	const source = "ClassInfo\n{\nname \"Ranks\"\nisCustomBuild 1\nrankHealth 100,120,150\n}\n"
	e := loadAuditMBCH(t, source)
	entries := auditMBCHEntries(e.pointBuyUI.rankAttrContainer)
	if len(entries) != 1 || entries[0].Text != "100,120,150" {
		t.Fatal("parsed rankHealth did not populate the rank-modifier form")
	}
	entries[0].SetText("100,125,175")
	got := parseAuditMBCH(t, e.GenerateSource())
	if got.RankAttributes["rankHealth"] != "100,125,175" || !e.IsDirty() {
		t.Fatalf("rank edit did not reach canonical serialized state: %+v", got.RankAttributes)
	}
	for key := range got.ExtraFields {
		if strings.EqualFold(key, "rankHealth") {
			t.Fatalf("rank form has competing extra-field owner %q", key)
		}
	}
	if !e.Undo() || e.IsDirty() || e.character.RankAttributes["rankHealth"] != "100,120,150" {
		t.Fatal("rank modifier did not undo to the clean parsed value")
	}
}

func TestAuditMBCHScalarMutationsRecordDirtySourceAndHistory(t *testing.T) {
	const source = `ClassInfo
{
	name "Scalars"
	sabercolor 2
	classNumberLimit 4
	respawnCustomTime 7
	extralives 1
	forcepowers FP_PUSH,3
}
`
	cases := []struct {
		name  string
		edit  func(*MBCHEditor)
		value func(*parsers.MBCHCharacter) string
		want  string
	}{
		{"saber color", func(e *MBCHEditor) { e.saberColorSelect.SetSelected("6 - Purple") }, func(ch *parsers.MBCHCharacter) string { return strconv.Itoa(ch.SaberColor) }, "6"},
		{"class limit", func(e *MBCHEditor) { e.classLimitEntry.SetText("5") }, func(ch *parsers.MBCHCharacter) string { return strconv.Itoa(ch.ClassNumberLimit) }, "5"},
		{"respawn time", func(e *MBCHEditor) { e.respawnTimeEntry.SetText("12") }, func(ch *parsers.MBCHCharacter) string { return strconv.Itoa(ch.RespawnCustomTime) }, "12"},
		{"extra lives", func(e *MBCHEditor) { e.extraLivesEntry.SetText("3") }, func(ch *parsers.MBCHCharacter) string { return strconv.Itoa(ch.ExtraLives) }, "3"},
		{"clear force powers", func(e *MBCHEditor) { e.forcePowersEntry.SetText("") }, func(ch *parsers.MBCHCharacter) string { return ch.ForcePowers }, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := loadAuditMBCH(t, source)
			before := tc.value(e.character)
			notifications := 0
			e.SetOnSourceChanged(func() { notifications++ })
			tc.edit(e)
			if !e.IsDirty() || !e.Session().DerivedDirty() || notifications != 1 {
				t.Fatalf("one scalar edit: dirty=%v, source notifications=%d", e.IsDirty(), notifications)
			}
			if got := tc.value(parseAuditMBCH(t, e.GenerateSource())); got != tc.want {
				t.Fatalf("serialized scalar = %q, want %q", got, tc.want)
			}
			if !e.Undo() || e.IsDirty() || tc.value(parseAuditMBCH(t, e.GenerateSource())) != before {
				t.Fatal("scalar edit did not undo to the clean parsed value")
			}
			if !e.Redo() || !e.IsDirty() || tc.value(parseAuditMBCH(t, e.GenerateSource())) != tc.want {
				t.Fatal("scalar edit did not redo to the edited value")
			}
		})
	}
}

func TestAuditMBCHLoadingCallbacksDoNotCreateUserEdits(t *testing.T) {
	e := loadAuditMBCH(t, "ClassInfo\n{\nname \"Loaded\"\nsabercolor 2\nclassNumberLimit 4\nrespawnCustomTime 7\nextralives 1\n}\n")
	changes := 0
	e.SetOnDirtyChanged(func(bool) { changes++ })
	e.loading = true
	e.saberColorSelect.SetSelected("8 - Black")
	e.classLimitEntry.SetText("20")
	e.respawnTimeEntry.SetText("30")
	e.extraLivesEntry.SetText("4")
	if e.IsDirty() || changes != 0 {
		t.Fatal("programmatic population recorded a user edit")
	}
	e.loading = false
	e.updateUI()
	got := parseAuditMBCH(t, e.GenerateSource())
	if got.SaberColor != 2 || got.ClassNumberLimit != 4 || got.RespawnCustomTime != 7 || got.ExtraLives != 1 || e.CanUndo() || e.IsDirty() {
		t.Fatal("loading callbacks changed canonical values or created history")
	}
}

func TestAuditMBCHFreshFirstEditHasRestorableAnchor(t *testing.T) {
	useEditorTestApp(t)
	e := NewMBCHEditor(nil)
	t.Cleanup(e.session.flushPending)
	// Do not read Source/Session before this edit: that used to hide the late-anchor bug.
	e.nameEntry.SetText("First edit")
	if !e.IsDirty() || !e.Undo() || e.IsDirty() {
		t.Fatal("fresh first edit did not undo to the unnamed clean document")
	}
	got := parseAuditMBCH(t, e.GenerateSource())
	if got.Name != "" || got.MaxHealth != 100 || got.ForceRegen != 1 || got.Speed != 1 ||
		got.APMultiplier != 1 || got.BPMultiplier != 1 || got.CSMultiplier != 1 || got.ASMultiplier != 1 {
		t.Fatalf("fresh undo lost populated defaults: %+v", got)
	}
	if !e.Redo() || !e.IsDirty() || parseAuditMBCH(t, e.GenerateSource()).Name != "First edit" {
		t.Fatal("fresh first edit did not redo")
	}
}

func TestAuditMBCHFormRetainsAuthoredFloatPrecision(t *testing.T) {
	const source = `ClassInfo
{
	name "Precision"
	forceregen 1.2345678901234567
	speed 0.0000001234567890123456
	APmultiplier 0.10000000000000002
	BPmultiplier 2.718281828459045
	CSmultiplier 3.141592653589793
	ASmultiplier 0.9876543210987654
}
`
	e := loadAuditMBCH(t, source)
	floats := func(ch *parsers.MBCHCharacter) [6]float64 {
		return [6]float64{ch.ForceRegen, ch.Speed, ch.APMultiplier, ch.BPMultiplier, ch.CSMultiplier, ch.ASMultiplier}
	}
	want := floats(parseAuditMBCH(t, source))
	entries := []*ValidatedEntry{e.forceRegenEntry, e.speedEntry, e.apMultEntry, e.bpMultEntry, e.csMultEntry, e.asMultEntry}
	for i, entry := range entries {
		value, err := strconv.ParseFloat(entry.Text, 64)
		if err != nil || value != want[i] {
			t.Fatalf("float control %d = %q, want roundtrip-safe %g", i, entry.Text, want[i])
		}
	}
	if got := floats(parseAuditMBCH(t, e.GenerateSource())); got != want {
		t.Fatalf("no-op form serialization rounded floats: got %v, want %v", got, want)
	}
	e.nameEntry.SetText("Unrelated edit")
	if got := floats(parseAuditMBCH(t, e.GenerateSource())); got != want {
		t.Fatalf("unrelated form edit rounded floats: got %v, want %v", got, want)
	}
	if !e.Undo() || e.IsDirty() || floats(parseAuditMBCH(t, e.GenerateSource())) != want || !e.Redo() || floats(parseAuditMBCH(t, e.GenerateSource())) != want {
		t.Fatal("float precision did not survive undo/redo")
	}
}

func TestAuditMBCHRawAndGridProjectionsShareHistory(t *testing.T) {
	const source = "ClassInfo\n{\nname \"Inventory\"\nweapons WP_BLASTER_PISTOL\nattributes MB_ATT_PUSH,1|HI_MEDPAC,1|CUSTOM_KEEP,9\n}\n"
	e := loadAuditMBCH(t, source)
	notifications := 0
	e.SetOnSourceChanged(func() { notifications++ })
	const raw = "MB_ATT_PUSH,2|HI_BINOCULARS,1|HI_UNKNOWN,3|CUSTOM_KEEP,9"
	e.attributesEntry.SetText(raw)
	if notifications != 1 || e.holdableGrid.values["HI_BINOCULARS"] != 1 || e.holdableGrid.values["HI_UNKNOWN"] != 3 || e.holdableGrid.values["HI_MEDPAC"] != 0 {
		t.Fatal("raw typing failed to synchronize holdables exactly once")
	}
	e.session.flushPending()
	notifications = 0
	e.holdableGrid.setLevel("HI_CLOAK", 1)
	if notifications != 1 {
		t.Fatalf("one holdable edit recursively published %d source changes", notifications)
	}
	attrs := parseAttributesString(parseAuditMBCH(t, e.GenerateSource()).Attributes)
	if attrs["HI_BINOCULARS"] != 1 || attrs["HI_UNKNOWN"] != 3 || attrs["HI_CLOAK"] != 1 || attrs["HI_MEDPAC"] != 0 || attrs["CUSTOM_KEEP"] != 9 {
		t.Fatalf("holdable edit used stale inventory or discarded unrelated tokens: %+v", attrs)
	}
	e.session.flushPending()
	notifications = 0
	e.attrGrid.updateValue("MB_ATT_PUSH", 3)
	if notifications != 1 || e.holdableGrid.values["HI_BINOCULARS"] != 1 || e.holdableGrid.values["HI_CLOAK"] != 1 {
		t.Fatal("attribute-grid edit failed to retain synchronized holdables")
	}
	if !e.Undo() || e.attrGrid.values["MB_ATT_PUSH"] != 2 || e.holdableGrid.values["HI_CLOAK"] != 1 {
		t.Fatal("attribute-grid undo did not synchronize projections")
	}
	if !e.Undo() || e.holdableGrid.values["HI_CLOAK"] != 0 || e.holdableGrid.values["HI_BINOCULARS"] != 1 {
		t.Fatal("holdable undo did not restore the authoritative raw edit")
	}
	if !e.Undo() || e.IsDirty() || e.holdableGrid.values["HI_MEDPAC"] != 1 || e.holdableGrid.values["HI_BINOCULARS"] != 0 {
		t.Fatal("raw undo did not restore clean loaded inventory")
	}
	if !e.Redo() || !e.Redo() || !e.Redo() || e.attrGrid.values["MB_ATT_PUSH"] != 3 || e.holdableGrid.values["HI_CLOAK"] != 1 {
		t.Fatal("raw/grid projection history did not redo")
	}
	e.attributesEntry.SetText("")
	e.weaponsEntry.SetText("")
	got := parseAuditMBCH(t, e.GenerateSource())
	if got.Attributes != "" || got.Weapons != "" || len(e.holdableGrid.values) != 0 || len(e.attrGrid.values) != 0 || len(e.weaponGrid.selected) != 0 {
		t.Fatal("explicit raw clearing resurrected stale grid data")
	}
}

func auditMBCHRankDots(t *testing.T, root fyne.CanvasObject) *RankDots {
	t.Helper()
	var found *RankDots
	walkAuditMBCH(root, func(obj fyne.CanvasObject) {
		if dots, ok := obj.(*RankDots); ok {
			found = dots
		}
	})
	if found == nil {
		t.Fatal("simulator skill card has no rank selector")
	}
	return found
}

func TestAuditMBCHConfiguredCostPreviewIsConsistent(t *testing.T) {
	const source = `ClassInfo
{
	name "Configured costs"
	isCustomBuild 1
	mbPoints 12
	hasCustomSpec 2
	customSpecName_1 "Gunner"
	customSpecName_2 "Scout"
	c_att_skill_0 MB_ATT_PISTOL
	c_att_names_0 "Pistol"
	c_att_ranks_0 2,4,8
	c_att_skill_1 MB_ATT_DASH
	c_att_ranks_1 1,3
	c_att_skill_2 MB_ATT_INVALID
	c_att_names_2 "Header"
	c_att_ranks_2 -1
	c_att_ranks_8 99
	c_att_skill_15 MB_ATT_PUSH
	c_att_ranks_15 0,3,5
}
`
	e := loadAuditMBCH(t, source)
	initial := e.GenerateSource()
	p, sim := e.pointBuyUI, e.pointBuyUI.simulator
	if !strings.Contains(p.budgetLabel.Text, "18 / 12") || !strings.Contains(p.budgetLabel.Text, "Unverified") || p.slotMaxCost[0].Text != "max 14" {
		t.Fatalf("configured maximum includes empty slots or differs from cumulative costs: %q / %q", p.budgetLabel.Text, p.slotMaxCost[0].Text)
	}
	pick := func(row, rank int) {
		t.Helper()
		dots := auditMBCHRankDots(t, sim.slotBox.Objects[row])
		test.WidgetRenderer(dots)
		test.Tap(dots.content.Objects[rank-1].(*rankDot))
	}
	pick(0, 2)
	pick(1, 2)
	if sim.budgetSpent.Text != "10" {
		t.Fatalf("selected ranks cost %q, want cumulative 6 + 4 = 10", sim.budgetSpent.Text)
	}
	pick(0, 3)
	if sim.purchased[0] != 2 || sim.budgetSpent.Text != "10" {
		t.Fatal("affordability used only the final rank's cost rather than cumulative configured costs")
	}
	if e.IsDirty() || e.GenerateSource() != initial {
		t.Fatal("simulator purchases mutated the authored document")
	}

	p.mbPointsEntry.SetText("18")
	sim.Refresh()
	pick(0, 3)
	if sim.budgetSpent.Text != "18" || !strings.Contains(p.budgetLabel.Text, "18 / 18") {
		t.Fatal("simulator maximum and editor maximum use different configured-cost semantics")
	}
	test.Tap(sim.archetypeBar.Objects[1].(*widget.Button))
	pick(0, 3)
	if sim.budgetSpent.Text != "8" || sim.purchased[15] != 3 || sim.purchased[0] != 0 {
		t.Fatal("archetype preview did not use its own configured costs and reset purchases")
	}
	p.mbPointsEntry.SetText("0")
	sim.Refresh()
	test.Tap(sim.resetBtn)
	pick(0, 1)
	pick(0, 2)
	if sim.purchased[15] != 1 || sim.budgetSpent.Text != "0" {
		t.Fatal("zero configured budget must admit only zero-cost selections, not invent unlimited purchases")
	}
}

func TestAuditMBCHComparisonPropagatesUnverifiedEvidence(t *testing.T) {
	useEditorTestApp(t)
	a := parseAuditMBCH(t, "ClassInfo\n{\nname A\nmaxhealth 100\nmaxarmor 50\nextralives 1\n}\n")
	b := parseAuditMBCH(t, "ClassInfo\n{\nname B\nmaxhealth 80\nmaxarmor 20\n}\n")
	stats := buildCharacterDiffStats(a, b)
	rows := map[string][]string{}
	for _, obj := range stats.Objects {
		row, ok := obj.(*fyne.Container)
		if !ok || len(row.Objects) != 3 {
			continue
		}
		label := row.Objects[0].(*widget.Label).Text
		for _, col := range row.Objects[1:] {
			walkAuditMBCH(col, func(child fyne.CanvasObject) {
				if value, ok := child.(*widget.Label); ok {
					rows[label] = append(rows[label], value.Text)
				}
			})
		}
	}
	for _, label := range []string{"Energy EHP:", "Explosive EHP:"} {
		values := rows[label]
		if len(values) != 2 || values[0] != CalculateDefensiveMatrix(a).EvidenceStatus || values[1] != CalculateDefensiveMatrix(b).EvidenceStatus || values[0] != "Unverified" {
			t.Fatalf("comparison %s labels configured raw pools as verified gameplay stats: %q", label, values)
		}
	}
	if values := rows["Configured pool across lives:"]; len(values) != 2 || values[0] != "300" || values[1] != "100" {
		t.Fatalf("comparison lost supported configured-pool arithmetic: %q", values)
	}
}

func TestAuditMBCHClickableCellHasVisibleKeyboardFocus(t *testing.T) {
	useEditorTestApp(t)
	taps := 0
	cell := newClickableCell(widget.NewLabel("Inventory item"), func() { taps++ })
	renderer := test.WidgetRenderer(cell)
	visibleOutline := func() bool {
		found := false
		for _, obj := range renderer.Objects() {
			walkAuditMBCH(obj, func(child fyne.CanvasObject) {
				if rect, ok := child.(*canvas.Rectangle); ok && rect.Visible() && rect.StrokeWidth > 0 && rect.StrokeColor != nil {
					_, _, _, alpha := rect.StrokeColor.RGBA()
					found = found || alpha > 0
				}
			})
		}
		return found
	}
	if visibleOutline() {
		t.Fatal("unfocused cell unexpectedly has a focus outline")
	}
	cell.FocusGained()
	if !visibleOutline() || taps != 0 {
		t.Fatal("keyboard focus must become visible without activating the cell")
	}
	cell.TypedRune(' ')
	cell.TypedKey(&fyne.KeyEvent{Name: fyne.KeyReturn})
	if taps != 2 {
		t.Fatalf("Space and Enter activated the focused cell %d times, want two", taps)
	}
	cell.FocusLost()
	if visibleOutline() || taps != 2 {
		t.Fatal("focus loss did not remove the outline without changing selection")
	}
}
