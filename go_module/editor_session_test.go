package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/test"
)

func useEditorTestApp(t *testing.T) {
	t.Helper()
	app := test.NewApp()
	app.Settings().SetTheme(&FoundryTheme{})
	t.Cleanup(app.Quit)
}

const multiSabFixture = `saber_a
{
	name			saber_a
	saberType		SABER_SINGLE
	numblades		1
}

saber_b
{
	name			saber_b
	saberType		SABER_STAFF
	numblades		2
}
`

// blockFor extracts the named top-level block's raw text (header line
// through matching close brace).
func blockFor(t *testing.T, src, name string) string {
	t.Helper()
	needle := name + "\n{"
	i := strings.Index(src, needle)
	if i < 0 {
		t.Fatalf("block %q not found in source", name)
	}
	depth := 0
	for j := i + len(name); j < len(src); j++ {
		switch src[j] {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return src[i : j+1]
			}
		}
	}
	t.Fatalf("block %q never closes", name)
	return ""
}

func writeSab(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "sabers.sab")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	return path
}

const realVEHHandlingFixture = `STAP_theed
{
	name		STAP_theed
	type		VH_SPEEDER
	speedMax	450
	turboSpeed	950
	acceleration	20
	decelIdle	10
	strafePerc	1.0
	braking		10
	customShader	vehicle/stap
}

wingmate
{
	speedMax	725
	acceleration	15
}
`

func writeReadOnlyVeh(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "STAP_theed.veh")
	if err := os.WriteFile(path, []byte(content), 0444); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestSABDefinitionNavigationPreservesSiblings(t *testing.T) {
	useEditorTestApp(t)

	e := NewSABEditor(&App{})
	if err := e.LoadFile(writeSab(t, multiSabFixture)); err != nil {
		t.Fatal(err)
	}
	names := e.DefinitionNames()
	if len(names) != 2 || names[0] != "saber_a" || names[1] != "saber_b" {
		t.Fatalf("expected [saber_a saber_b], got %v", names)
	}
	if e.SelectedDefinition() != 0 {
		t.Fatalf("should start on definition 0, got %d", e.SelectedDefinition())
	}

	// Edit definition A's name via the model, then switch to B.
	e.saber.Name = "saber_a_renamed"

	if err := e.SelectDefinition(1); err != nil {
		t.Fatal(err)
	}
	if e.SelectedDefinition() != 1 {
		t.Fatalf("selection should be 1, got %d", e.SelectedDefinition())
	}
	if e.saber.Name != "saber_b" {
		t.Fatalf("definition B should drive the form, got %q", e.saber.Name)
	}

	// The switch itself is undoable as one step.
	if !e.Undo() {
		t.Fatal("definition switch must be undoable")
	}
	if e.SelectedDefinition() != 0 || e.saber.Name != "saber_a_renamed" {
		t.Fatalf("undo must restore definition A with its edits (def=%d name=%q)",
			e.SelectedDefinition(), e.saber.Name)
	}

	if err := e.SelectDefinition(1); err != nil {
		t.Fatal(err)
	}
	current := e.CurrentSource()
	// Sibling preservation: block A in the generated document still
	// contains the rename, byte-for-byte as the AST round-trip kept it.
	// The definition identifier itself was renamed, so locate the
	// preserved sibling under its new header.
	aBlock := blockFor(t, current, "saber_a_renamed")
	if !strings.Contains(aBlock, "saber_a_renamed") {
		t.Fatalf("sibling edits must survive definition switches, block A:\n%s", aBlock)
	}
	bBlock := blockFor(t, current, "saber_b")
	if !strings.Contains(bBlock, "saber_b") || !strings.Contains(bBlock, "SABER_STAFF") {
		t.Fatalf("block B content should be intact:\n%s", bBlock)
	}

	// Range check errors, nothing mutated.
	if err := e.SelectDefinition(5); err == nil {
		t.Fatal("out-of-range definition must error")
	}
}

func TestVEHRealHandlingKeysPopulateFormFromReadOnlySource(t *testing.T) {
	useEditorTestApp(t)

	e := NewVEHEditor(&App{})
	if err := e.LoadFile(writeReadOnlyVeh(t, realVEHHandlingFixture)); err != nil {
		t.Fatal(err)
	}
	if e.speedEntry.Text != "450.0" || e.accelEntry.Text != "20.0" || e.decelEntry.Text != "10.0" {
		t.Fatalf("form shows speed=%q acceleration=%q deceleration=%q, want 450.0/20.0/10.0",
			e.speedEntry.Text, e.accelEntry.Text, e.decelEntry.Text)
	}
	source := e.CurrentSource()
	for _, want := range []string{"customShader\tvehicle/stap", "wingmate\n{\n\tspeedMax\t725\n\tacceleration\t15\n}"} {
		if !strings.Contains(source, want) {
			t.Errorf("loaded source lost %q:\n%s", want, source)
		}
	}
}

func TestSABApplySourceTextInMemoryAndUndo(t *testing.T) {
	useEditorTestApp(t)

	e := NewSABEditor(&App{})
	if err := e.LoadFile(writeSab(t, multiSabFixture)); err != nil {
		t.Fatal(err)
	}
	before := e.CurrentSource()

	// A failed parse must leave the editor untouched (text stays
	// correctable in the panel).
	if err := e.ApplySourceText("this is not {{{ a saber"); err == nil {
		t.Fatal("garbage source must fail to apply")
	}
	if e.CurrentSource() != before {
		t.Fatal("failed apply must not mutate the working state")
	}

	// Valid source applies in-memory: dirty, undoable, path untouched.
	applied := strings.Replace(multiSabFixture, "SABER_SINGLE", "SABER_STAFF", 1)
	if err := e.ApplySourceText(applied); err != nil {
		t.Fatal(err)
	}
	if e.currentPath == "" {
		t.Fatal("in-memory apply must preserve document identity")
	}
	if !e.IsDirty() {
		t.Fatal("applied state must be dirty")
	}
	if !strings.Contains(e.CurrentSource(), "SABER_STAFF") {
		t.Fatal("applied source should drive the working model")
	}
	if !e.Undo() {
		t.Fatal("apply must be undoable")
	}
	if e.CurrentSource() != before {
		t.Fatal("undo must restore the pre-apply state")
	}
}

func TestMBCHModelProjectionsStaySynchronizedAcrossReplacementPaths(t *testing.T) {
	useEditorTestApp(t)

	const loaded = `ClassInfo
{
	name		"v9_Vader"
	MBClass		MB_CLASS_SITH
	maxhealth	75
	maxarmor	70
}
`
	path := filepath.Join(t.TempDir(), "v9_Vader.mbch")
	if err := os.WriteFile(path, []byte(loaded), 0444); err != nil {
		t.Fatal(err)
	}

	e := NewMBCHEditor(&App{})
	stableModel := e.character
	assertProjection := func(name, class, health, armor string) {
		t.Helper()
		wantStats := "HP " + health + "   ARMOR " + armor
		if e.summary.nameLbl.Text != name || e.summary.classLbl.Text != class || e.summary.statsLbl.Text != wantStats {
			t.Fatalf("summary = %q/%q/%q, want %q/%q/%q",
				e.summary.nameLbl.Text, e.summary.classLbl.Text, e.summary.statsLbl.Text,
				name, class, wantStats)
		}
		if e.nameEntry.Text != name || e.healthEntry.Text != health || e.armorEntry.Text != armor {
			t.Fatalf("form = %q/HP %q/ARMOR %q, want %q/HP %q/ARMOR %q",
				e.nameEntry.Text, e.healthEntry.Text, e.armorEntry.Text,
				name, health, armor)
		}
		if e.character != stableModel || e.summary.character != stableModel || e.customSkillsUI.character != stableModel {
			t.Fatal("editor and model-backed child components no longer share the stable character model")
		}
	}

	if err := e.LoadFile(path); err != nil {
		t.Fatal(err)
	}
	assertProjection("v9_Vader", "SITH", "75", "70")

	applied := strings.NewReplacer(
		"v9_Vader", "source_applied",
		"MB_CLASS_SITH", "MB_CLASS_JEDI",
		"maxhealth\t75", "maxhealth\t88",
		"maxarmor\t70", "maxarmor\t22",
	).Replace(loaded)
	if err := e.ApplySourceText(applied); err != nil {
		t.Fatal(err)
	}
	assertProjection("source_applied", "JEDI", "88", "22")

	if !e.Undo() {
		t.Fatal("source apply must be undoable")
	}
	assertProjection("v9_Vader", "SITH", "75", "70")

	jsonPath := filepath.Join(t.TempDir(), "imported.json")
	const imported = `{"Name":"json_imported","MBClass":"MB_CLASS_SOLDIER","MaxHealth":125,"MaxArmor":45}`
	if err := os.WriteFile(jsonPath, []byte(imported), 0644); err != nil {
		t.Fatal(err)
	}
	if err := e.ImportJSON(jsonPath); err != nil {
		t.Fatal(err)
	}
	assertProjection("json_imported", "SOLDIER", "125", "45")

	if !e.Undo() {
		t.Fatal("JSON import must be undoable")
	}
	assertProjection("v9_Vader", "SITH", "75", "70")
}

func TestMBCHPrepareSaveDerivesNameWithoutMutatingModel(t *testing.T) {
	useEditorTestApp(t)

	e := NewMBCHEditor(&App{})
	e.character.Name = ""
	target := filepath.Join(t.TempDir(), "jedi_master.mbch")

	candidate, err := e.PrepareSave(target)
	if err != nil {
		t.Fatal(err)
	}
	// Candidate carries the derived name; the model does not.
	if !strings.Contains(candidate, "jedi_master") {
		t.Fatal("candidate bytes should contain the filename-derived name")
	}
	if e.character.Name != "" {
		t.Fatalf("PrepareSave must not mutate the model, name=%q", e.character.Name)
	}

	if err := e.CommitSave(target, candidate); err != nil {
		t.Fatal(err)
	}
	// Only after approved commit is the name applied and bytes exact.
	if e.character.Name != "jedi_master" {
		t.Fatalf("commit should apply the approved name, got %q", e.character.Name)
	}
	onDisk, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if string(onDisk) != candidate {
		t.Fatal("written bytes must equal reviewed candidate")
	}
	if e.IsDirty() {
		t.Fatal("committed save must be clean")
	}
}

func TestMBCHCommitSaveAbortsWhenBackupFails(t *testing.T) {
	useEditorTestApp(t)

	dir := t.TempDir()
	target := filepath.Join(dir, "vet.mbch")
	original := "MBCH\n{\n\tname\told\n}\n"
	if err := os.WriteFile(target, []byte(original), 0644); err != nil {
		t.Fatal(err)
	}

	e := NewMBCHEditor(&App{})
	// No config directory → CreateBackup errors → save must abort.
	e.fileManager = NewFileManager("")
	e.character.Name = "new"
	candidate, err := e.PrepareSave(target)
	if err != nil {
		t.Fatal(err)
	}

	if err := e.CommitSave(target, candidate); err == nil {
		t.Fatal("save must be refused when the backup cannot be created")
	}
	onDisk, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if string(onDisk) != original {
		t.Fatalf("failed save must not touch the original file, got:\n%s", onDisk)
	}
	if e.currentPath == target {
		t.Fatal("failed save must not adopt the path")
	}
}

func TestSiegeValidateRealChecks(t *testing.T) {
	useEditorTestApp(t)

	e := NewSiegeEditor(&App{})
	// Missing team blocks subsume their child-field validation; a fresh
	// document must report both required teams and the mission title.
	issues := e.Validate()
	if len(issues) == 0 {
		t.Fatal("empty siege document must produce validation issues")
	}
	joined := strings.Join(issues, "\n")
	for _, want := range []string{"team1", "team2", "missionname"} {
		if !strings.Contains(joined, want) {
			t.Errorf("validation should flag %q, got:\n%s", want, joined)
		}
	}
}

func TestSiegeSetOnSourceChangedFires(t *testing.T) {
	useEditorTestApp(t)

	e := NewSiegeEditor(&App{})
	fired := 0
	e.SetOnSourceChanged(func() { fired++ })
	e.missionNameEntry.SetText("Hoth Assault")
	e.session.flushPending() // cancel the coalescing timer before teardown
	if fired == 0 {
		t.Fatal("SetOnSourceChanged must fire on edits, not be a stub")
	}
}

func TestSABSetOnSourceChangedFires(t *testing.T) {
	useEditorTestApp(t)

	e := NewSABEditor(&App{})
	fired := 0
	e.SetOnSourceChanged(func() { fired++ })
	e.nameEntry.SetText("Darksaber Mk II")
	e.session.flushPending() // cancel the coalescing timer before teardown
	if fired == 0 {
		t.Fatal("SetOnSourceChanged must fire on edits, not be a stub")
	}
}

func TestRecoveryStoreRoundtrip(t *testing.T) {
	store := NewRecoveryStore(filepath.Join(t.TempDir(), "recovery"))
	entry := &RecoveryEntry{
		Title:        "Untitled character",
		Format:       ".mbch",
		OriginalPath: "/original/vet.mbch",
		Baseline:     "MBCH\n{\n}\n",
		HasBaseline:  true,
		Working:      "MBCH\n{\n\tname\thalf-done\n}\n",
		HasDraft:     true,
		Draft:        "MBCH\n{",
		SavedAt:      time.Now(),
	}
	if err := store.Save(entry); err != nil {
		t.Fatal(err)
	}
	if entry.ID == "" {
		t.Fatal("Save must assign an ID")
	}
	got, err := store.List()
	if err != nil || len(got) != 1 {
		t.Fatalf("expected 1 entry, got %d err=%v", len(got), err)
	}
	if got[0].Working != entry.Working || got[0].Draft != entry.Draft || !got[0].HasDraft {
		t.Fatalf("recovered entry mismatched: %+v", got[0])
	}

	// Remove drops exactly one entry.
	if err := store.Remove(entry.ID); err != nil {
		t.Fatal(err)
	}
	got, _ = store.List()
	if len(got) != 0 {
		t.Fatalf("expected empty store after Remove, got %d", len(got))
	}

	// Save two, then Clear drops everything.
	if err := store.Save(&RecoveryEntry{Format: ".mbch", Working: "a", SavedAt: time.Now()}); err != nil {
		t.Fatal(err)
	}
	if err := store.Save(&RecoveryEntry{Format: ".sab", Working: "b", SavedAt: time.Now()}); err != nil {
		t.Fatal(err)
	}
	if err := store.Clear(); err != nil {
		t.Fatal(err)
	}
	if got, _ = store.List(); len(got) != 0 {
		t.Fatalf("Clear must empty the store, got %d", len(got))
	}

	// Disabled store degrades to a no-op.
	disabled := NewRecoveryStore("")
	if got, _ := disabled.List(); got != nil {
		t.Fatal("disabled store must list nothing")
	}
}

func TestRecoveryStoreRejectsUnsafeNamesSymlinksCorruptAndOversize(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "recovery")
	store := NewRecoveryStore(dir)
	if err := store.Save(&RecoveryEntry{
		ID:      "../../escaped",
		Format:  ".mbch",
		Working: "MBCH\n{\n}\n",
	}); err == nil {
		t.Fatal("traversal recovery ID must be rejected")
	}
	if _, err := os.Stat(filepath.Join(root, "escaped.json")); !os.IsNotExist(err) {
		t.Fatalf("unsafe ID created a file outside recovery dir: %v", err)
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(dir, "0123456789abcdef.json"),
		[]byte(`{"id":"../../victim","format":".mbch","working":"x"}`), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "1111111111111111.json"), []byte(`{"id":`), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "2222222222222222.json"),
		[]byte(strings.Repeat("x", maxRecoveryFileBytes+1)), 0600); err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(root, "outside.json")
	if err := os.WriteFile(outside, []byte(`{"id":"3333333333333333","format":".mbch"}`), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(dir, "3333333333333333.json")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}

	entries, err := store.List()
	if err == nil {
		t.Fatal("unsafe/corrupt/oversize recovery files must surface warnings")
	}
	if len(entries) != 0 {
		t.Fatalf("unsafe entries must not be loaded: %+v", entries)
	}
	if err := store.Remove("../../victim"); err == nil {
		t.Fatal("Remove must reject an unsafe ID")
	}
	if data, err := os.ReadFile(outside); err != nil || !strings.Contains(string(data), "3333333333333333") {
		t.Fatal("recovery scan/remove followed or altered a symlink target")
	}
}

func newRecoveryTestApp(t *testing.T, recoveryDir, configDir string) *App {
	t.Helper()
	fyneApp := test.NewApp()
	fyneApp.Settings().SetTheme(&FoundryTheme{})
	t.Cleanup(fyneApp.Quit)
	app := &App{
		fyneApp:      fyneApp,
		mainWindow:   fyneApp.NewWindow("recovery"),
		docTabs:      container.NewDocTabs(),
		editors:      make(map[*container.TabItem]Editor),
		sourceDrafts: NewSourceDraftStore(),
		recovery:     NewRecoveryStore(recoveryDir),
		fileManager:  NewFileManager(configDir),
	}
	app.docTabs.OnSelected = func(tab *container.TabItem) {
		if editor, ok := app.editors[tab]; ok {
			app.setSourceEditorForAll(editor)
		}
	}
	return app
}

func TestRecoveryChangedOriginalRestoresDetachedWithDraftAndSelection(t *testing.T) {
	root := t.TempDir()
	originalPath := filepath.Join(root, "sabers.sab")
	if err := os.WriteFile(originalPath, []byte(strings.Replace(multiSabFixture, "SABER_SINGLE", "SABER_STAFF", 1)), 0644); err != nil {
		t.Fatal(err)
	}
	app := newRecoveryTestApp(t, filepath.Join(root, "recovery"), filepath.Join(root, "config"))
	entry := &RecoveryEntry{
		ID:            "4444444444444444",
		Title:         "sabers.sab",
		Format:        ".sab",
		OriginalPath:  originalPath,
		Baseline:      multiSabFixture,
		HasBaseline:   true,
		Working:       strings.Replace(multiSabFixture, "saber_a", "saber_recovered", 1),
		Draft:         "saber_b\n{",
		DraftParseErr: "unbalanced braces",
		DraftEditMode: true,
		HasDraft:      true,
		SelectedDef:   1,
		SavedAt:       time.Now(),
	}
	if err := app.restoreRecoveredEntryError(entry); err != nil {
		t.Fatal(err)
	}
	if len(app.editors) != 1 {
		t.Fatalf("expected one recovered editor, got %d", len(app.editors))
	}
	for _, editor := range app.editors {
		sessionEditor := editor.(SessionEditor)
		if editor.GetCurrentPath() != "" || sessionEditor.Session().HasOriginal {
			t.Fatal("changed original must restore as detached untitled work")
		}
		if sessionEditor.Session().RecoveryID != entry.ID {
			t.Fatalf("recovery ID changed: %q", sessionEditor.Session().RecoveryID)
		}
		if sessionEditor.SelectedDefinition() != 1 {
			t.Fatalf("definition selection was not restored through SelectDefinition: %d", sessionEditor.SelectedDefinition())
		}
		draft, ok := app.sourceDrafts.Get(editor)
		if !ok || draft.Text != entry.Draft || draft.ParseErr != entry.DraftParseErr || !draft.EditMode {
			t.Fatalf("draft was not restored exactly: %+v ok=%v", draft, ok)
		}
	}
}

func TestRecoveryApplyFailureStaysUnresolved(t *testing.T) {
	root := t.TempDir()
	originalPath := filepath.Join(root, "sabers.sab")
	if err := os.WriteFile(originalPath, []byte(multiSabFixture), 0644); err != nil {
		t.Fatal(err)
	}
	recoveryDir := filepath.Join(root, "recovery")
	app := newRecoveryTestApp(t, recoveryDir, filepath.Join(root, "config"))
	entry := &RecoveryEntry{
		ID:           "5555555555555555",
		Title:        "sabers.sab",
		Format:       ".sab",
		OriginalPath: originalPath,
		Baseline:     multiSabFixture,
		HasBaseline:  true,
		Working:      "not a valid {{{ saber",
		SavedAt:      time.Now(),
	}
	if err := app.recovery.Save(entry); err != nil {
		t.Fatal(err)
	}
	if err := app.restoreRecoveredEntryError(entry); err == nil {
		t.Fatal("invalid recovered working state must fail restoration")
	}
	if len(app.editors) != 0 {
		t.Fatal("failed recovery must not create a misleading restored tab")
	}
	entries, err := app.recovery.List()
	if err != nil || len(entries) != 1 || entries[0].ID != entry.ID {
		t.Fatalf("failed recovery must remain unresolved on disk: entries=%+v err=%v", entries, err)
	}
}

func TestSaveDestinationSnapshotDetectsCreatedDeletedChangedAndSymlink(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "target.sab")
	missing, err := snapshotSaveDestination(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("created"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := verifySaveDestination(path, missing); err == nil {
		t.Fatal("destination creation after review must abort")
	}
	original, err := snapshotSaveDestination(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("changed"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := verifySaveDestination(path, original); err == nil {
		t.Fatal("destination byte changes after review must abort")
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := verifySaveDestination(path, original); err == nil {
		t.Fatal("destination deletion after review must abort")
	}
	outside := filepath.Join(dir, "outside")
	if err := os.WriteFile(outside, []byte("outside"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, path); err == nil {
		if _, err := snapshotSaveDestination(path); err == nil {
			t.Fatal("symlink destination must be rejected")
		}
	}
}

func TestPublishCandidateRequiresAndChecksBackup(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "target.veh")
	if err := os.WriteFile(path, []byte("original"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := publishEditorCandidate(nil, path, "candidate"); err == nil {
		t.Fatal("replacement without backup storage must fail")
	}
	if got, _ := os.ReadFile(path); string(got) != "original" {
		t.Fatal("failed backup changed destination")
	}
	if err := publishEditorCandidate(NewFileManager(filepath.Join(dir, "config")), path, "candidate"); err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(path); string(got) != "candidate" {
		t.Fatalf("published bytes differ from candidate: %q", got)
	}
}

func TestMBCHPrepareSaveDoesNotLeakPathSpecificName(t *testing.T) {
	useEditorTestApp(t)
	editor := NewMBCHEditor(&App{})
	editor.character.Name = ""
	first, err := editor.PrepareSave(filepath.Join(t.TempDir(), "first.mbch"))
	if err != nil {
		t.Fatal(err)
	}
	secondPath := filepath.Join(t.TempDir(), "second.mbch")
	second, err := editor.PrepareSave(secondPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(first, "first") || strings.Contains(second, "first") || !strings.Contains(second, "second") {
		t.Fatal("path-specific candidate name leaked across PrepareSave calls")
	}
	if err := editor.CommitSave(secondPath, second); err != nil {
		t.Fatal(err)
	}
	if editor.character.Name != "second" {
		t.Fatalf("published candidate name not adopted: %q", editor.character.Name)
	}
}

func TestSourcePanelSharedDraftSurvivesPassiveMirrors(t *testing.T) {
	useEditorTestApp(t)
	app := &App{sourceDrafts: NewSourceDraftStore()}
	editor := NewSABEditor(app)
	if err := editor.LoadFile(writeSab(t, multiSabFixture)); err != nil {
		t.Fatal(err)
	}
	primary := NewSourcePanel(app)
	mirror := NewSourcePanel(app)
	primary.SetActiveEditor(editor)
	mirror.SetActiveEditor(editor)
	primary.setEditMode(true)
	primary.editor.SetText("saber_a\n{")
	mirror.refreshFromProvider()
	draft, ok := app.sourceDrafts.Get(editor)
	if !ok || draft.Text != "saber_a\n{" || draft.ParseErr == "" || !draft.EditMode {
		t.Fatalf("shared invalid draft not retained exactly: %+v ok=%v", draft, ok)
	}
	if mirror.editor.Text != draft.Text || mirror.lastParseErr != draft.ParseErr || !mirror.inEditMode {
		t.Fatalf("mirror did not restore document-local draft state: text=%q err=%q edit=%v",
			mirror.editor.Text, mirror.lastParseErr, mirror.inEditMode)
	}
	primary.renderFromProvider(editor.GenerateSource())
	if _, ok := app.sourceDrafts.Get(editor); !ok {
		t.Fatal("passive render deleted the shared draft")
	}
	primary.SetActiveEditor(nil)
	primary.SetActiveEditor(editor)
	if primary.editor.Text != draft.Text || primary.lastParseErr != draft.ParseErr {
		t.Fatal("tab switch did not restore exact draft and diagnostics")
	}
	if err := parseSourceForEditor(nil, "x\n{\n}\n"); err == nil {
		t.Fatal("nil parser/editor must not classify source as valid")
	}
	primary.updateByteCount("")
	if primary.byteCount.Text != "" {
		t.Fatalf("blank source must have blank byte count, got %q", primary.byteCount.Text)
	}
	primary.editorRef = &MBCHEditor{}
	primary.updateByteCount(strings.Repeat("x", 16384))
	if !strings.Contains(primary.byteCount.Text, "16383") || !strings.Contains(primary.byteCount.Text, "limit") {
		t.Fatalf("MBCH exact rejection boundary not shown from parser assessment: %q", primary.byteCount.Text)
	}
	primary.editorRef = editor
	primary.updateByteCount(strings.Repeat("x", 16384))
	if strings.Contains(primary.byteCount.Text, "16383") {
		t.Fatalf("non-MBCH source inherited MBCH capacity: %q", primary.byteCount.Text)
	}
}

func TestDiagnosticRootsAndSecretsSanitizedAtPublicationBoundary(t *testing.T) {
	app := &App{
		config: AppConfig{
			GitHubToken:    "secret-current",
			GamedataPath:   "/private/game",
			TextAssetsPath: "/private/textassets",
			MD3ViewPath:    "/private/tools/md3view",
			KnownModpacks:  []*Modpack{{Path: "/private/project"}},
		},
		legacyTokenPending: "secret-legacy",
	}
	raw := "secret-current secret-legacy /private/game/base /private/textassets/MBII /private/tools/md3view /private/project/file"
	safe := SanitizeDiagnosticText(raw,
		[]string{app.config.GitHubToken, app.legacyTokenPending},
		app.diagnosticRoots())
	for _, forbidden := range []string{"secret-current", "secret-legacy", "/private/game", "/private/textassets", "/private/tools", "/private/project"} {
		if strings.Contains(safe, forbidden) {
			t.Fatalf("diagnostic publication leaked %q: %s", forbidden, safe)
		}
	}
}

func TestRecoveryCaptureIncludesCleanEditorDraftAndKeepsStableID(t *testing.T) {
	root := t.TempDir()
	app := newRecoveryTestApp(t, filepath.Join(root, "recovery"), filepath.Join(root, "config"))
	editor := NewSABEditor(app)
	if err := editor.LoadFile(writeSab(t, multiSabFixture)); err != nil {
		t.Fatal(err)
	}
	if editor.IsDirty() {
		t.Fatal("loaded editor should start clean")
	}
	invalid := SourceDraft{Text: "saber_a\n{", ParseErr: "unbalanced braces", EditMode: true}
	app.sourceDrafts.Set(editor, invalid)
	app.captureRecoveryEntry(editor)
	firstID := editor.Session().RecoveryID
	if !validRecoveryID(firstID) {
		t.Fatalf("capture assigned invalid recovery ID %q", firstID)
	}
	valid := SourceDraft{Text: multiSabFixture, EditMode: false}
	app.sourceDrafts.Set(editor, valid)
	app.captureRecoveryEntry(editor)
	if editor.Session().RecoveryID != firstID {
		t.Fatal("repeat capture changed the document recovery ID")
	}
	entries, err := app.recovery.List()
	if err != nil || len(entries) != 1 {
		t.Fatalf("expected one stable recovery entry, got %+v err=%v", entries, err)
	}
	if !entries[0].HasDraft || entries[0].Draft != valid.Text || entries[0].DraftEditMode {
		t.Fatalf("valid source draft was not captured exactly: %+v", entries[0])
	}
}

func TestAllSessionEditorsPublishExactCandidates(t *testing.T) {
	useEditorTestApp(t)
	root := t.TempDir()
	app := &App{fileManager: NewFileManager(filepath.Join(root, "config"))}

	mbch := NewMBCHEditor(app)
	mbch.nameEntry.Text = "character"
	sab := NewSABEditor(app)
	sab.nameEntry.Text = "saber"
	veh := NewVEHEditor(app)
	veh.nameEntry.Text = "vehicle"
	siege := NewSiegeEditor(app)
	siege.missionNameEntry.Text = "mission"

	editors := []struct {
		name string
		ext  string
		edit SessionEditor
	}{
		{name: "MBCH", ext: ".mbch", edit: mbch},
		{name: "SAB", ext: ".sab", edit: sab},
		{name: "VEH", ext: ".veh", edit: veh},
		{name: "Siege", ext: ".siege", edit: siege},
	}
	for _, item := range editors {
		t.Run(item.name, func(t *testing.T) {
			item.edit.CurrentSource()
			path := filepath.Join(root, strings.ToLower(item.name)+item.ext)
			candidate, err := item.edit.PrepareSave(path)
			if err != nil {
				t.Fatal(err)
			}
			if err := item.edit.CommitSave(path, candidate); err != nil {
				t.Fatal(err)
			}
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if string(data) != candidate {
				t.Fatal("published bytes differ from reviewed candidate")
			}
			session := item.edit.Session()
			if session.Path != path || session.OriginalBytes != candidate || item.edit.IsDirty() {
				t.Fatalf("successful commit did not update path/baseline/clean state: %+v", session)
			}
		})
	}
}

func TestReviewedPublicationRejectsMutationAfterApprovalCheck(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "reviewed.sab")
	if err := os.WriteFile(path, []byte("reviewed bytes"), 0640); err != nil {
		t.Fatal(err)
	}
	reviewed, err := snapshotSaveDestination(path)
	if err != nil {
		t.Fatal(err)
	}
	backupPath, err := publishEditorCandidateReviewedWithHook(
		NewFileManager(filepath.Join(root, "config")),
		path,
		"approved candidate",
		reviewed,
		func() {
			if writeErr := os.WriteFile(path, []byte("external mutation"), 0640); writeErr != nil {
				t.Fatalf("inject destination mutation: %v", writeErr)
			}
		},
	)
	if err == nil {
		t.Fatal("mutation between approval verification and publication must abort")
	}
	got, readErr := os.ReadFile(path)
	if readErr != nil || string(got) != "external mutation" {
		t.Fatalf("reviewed save overwrote external mutation: %q err=%v", got, readErr)
	}
	backup, readErr := os.ReadFile(backupPath)
	if readErr != nil || string(backup) != reviewed.Bytes {
		t.Fatalf("backup must contain exactly reviewed bytes: %q err=%v", backup, readErr)
	}
}

func TestReviewedCommitKeepsEditorStateUntilPublicationSucceeds(t *testing.T) {
	useEditorTestApp(t)
	root := t.TempDir()
	app := &App{fileManager: NewFileManager(filepath.Join(root, "config"))}
	editor := NewSABEditor(app)
	path := filepath.Join(root, "state.sab")
	if err := os.WriteFile(path, []byte(multiSabFixture), 0644); err != nil {
		t.Fatal(err)
	}
	if err := editor.LoadFile(path); err != nil {
		t.Fatal(err)
	}
	reviewed, err := snapshotSaveDestination(path)
	if err != nil {
		t.Fatal(err)
	}
	baselineBefore := editor.Session().OriginalBytes
	sourceBefore := editor.docSource
	editor.nameEntry.SetText("approved_name")
	editor.CurrentSource()
	candidate, err := editor.PrepareSave(path)
	if err != nil {
		t.Fatal(err)
	}

	err = app.commitReviewedSaveWithHook(editor, path, candidate, reviewed, func() {
		if writeErr := os.WriteFile(path, []byte("external mutation"), 0644); writeErr != nil {
			t.Fatalf("inject destination mutation: %v", writeErr)
		}
	})
	if err == nil {
		t.Fatal("reviewed commit must reject a destination mutation at publication")
	}
	if editor.Session().OriginalBytes != baselineBefore || editor.docSource != sourceBefore {
		t.Fatal("failed publication advanced editor baseline/source state")
	}
	if !editor.IsDirty() {
		t.Fatal("failed publication marked the editor clean")
	}
	got, readErr := os.ReadFile(path)
	if readErr != nil || string(got) != "external mutation" {
		t.Fatalf("failed publication overwrote destination mutation: %q err=%v", got, readErr)
	}
}

func TestReviewedCommitReportsPostPublicationFailureWithoutAdvancingEditorState(t *testing.T) {
	useEditorTestApp(t)
	root := t.TempDir()
	app := &App{fileManager: NewFileManager(filepath.Join(root, "config"))}
	editor := NewSABEditor(app)
	path := filepath.Join(root, "state.sab")
	if err := os.WriteFile(path, []byte(multiSabFixture), 0644); err != nil {
		t.Fatal(err)
	}
	if err := editor.LoadFile(path); err != nil {
		t.Fatal(err)
	}
	reviewed, err := snapshotSaveDestination(path)
	if err != nil {
		t.Fatal(err)
	}
	baselineBefore := editor.Session().OriginalBytes
	sourceBefore := editor.docSource
	editor.nameEntry.SetText("approved_name")
	editor.CurrentSource()
	candidate, err := editor.PrepareSave(path)
	if err != nil {
		t.Fatal(err)
	}

	var holdPath, backupPath string
	err = app.commitReviewedSaveWithHooks(editor, path, candidate, reviewed, savePublicationHooks{
		afterCandidatePublish: func(hold, backup string) {
			holdPath, backupPath = hold, backup
			if writeErr := os.WriteFile(backup, []byte("corrupt backup"), 0600); writeErr != nil {
				t.Fatalf("corrupt final backup proof: %v", writeErr)
			}
		},
	})
	if err == nil || !saveWasPublished(err) {
		t.Fatalf("post-publication failure lost publication outcome: %v", err)
	}
	assertFileBytes(t, path, candidate)
	if editor.Session().OriginalBytes != baselineBefore || editor.docSource != sourceBefore {
		t.Fatal("post-publication failure advanced editor baseline/source state")
	}
	if !editor.IsDirty() {
		t.Fatal("post-publication failure marked the editor clean")
	}
	if !strings.Contains(editor.lastError, "Candidate was published") {
		t.Fatalf("editor falsely reported no write: %q", editor.lastError)
	}
	for _, recoveryPath := range []string{holdPath, backupPath} {
		if recoveryPath == "" || !strings.Contains(err.Error(), recoveryPath) {
			t.Fatalf("error is missing recovery path %q: %v", recoveryPath, err)
		}
	}
}

func TestSABAdvancedControlsFoldBothDirections(t *testing.T) {
	useEditorTestApp(t)
	e := NewSABEditor(&App{})
	e.saber.SlapAnim = "BOTH_A7_SLAP_R"
	e.saber.ReadyAnim = "BOTH_SABERFAST_STANCE"
	e.saber.JumpAtkUpMove = "LS_A_JUMP_T__B_"
	e.saber.JumpAtkFwdMove = "LS_A_FLIP_STAB"
	e.saber.LungeAtkMove = "LS_A_LUNGE"
	e.saber.G2MarksShader = "gfx/damage/saberglowmark"
	e.saber.G2WeaponMarkShader = "gfx/damage/saberburnmark"
	e.loading = true
	e.updateUI()
	e.loading = false

	gotEntries := []string{
		e.slapAnimEntry.Text,
		e.readyAnimEntry.Text,
		e.jumpAtkUpMoveEntry.Text,
		e.jumpAtkFwdMoveEntry.Text,
		e.lungeAtkMoveEntry.Text,
		e.g2MarksShaderEntry.Text,
		e.g2WeaponMarkShaderEntry.Text,
	}
	wantEntries := []string{
		e.saber.SlapAnim,
		e.saber.ReadyAnim,
		e.saber.JumpAtkUpMove,
		e.saber.JumpAtkFwdMove,
		e.saber.LungeAtkMove,
		e.saber.G2MarksShader,
		e.saber.G2WeaponMarkShader,
	}
	for i := range wantEntries {
		if gotEntries[i] != wantEntries[i] {
			t.Fatalf("model-to-UI field %d mismatch: got %q want %q", i, gotEntries[i], wantEntries[i])
		}
	}

	e.slapAnimEntry.Text = "slap_ui"
	e.readyAnimEntry.Text = "ready_ui"
	e.jumpAtkUpMoveEntry.Text = "jump_up_ui"
	e.jumpAtkFwdMoveEntry.Text = "jump_fwd_ui"
	e.lungeAtkMoveEntry.Text = "lunge_ui"
	e.g2MarksShaderEntry.Text = "shader/g2_marks_ui"
	e.g2WeaponMarkShaderEntry.Text = "shader/g2_weapon_ui"
	e.updateSaberFromUI()

	gotModel := []string{
		e.saber.SlapAnim,
		e.saber.ReadyAnim,
		e.saber.JumpAtkUpMove,
		e.saber.JumpAtkFwdMove,
		e.saber.LungeAtkMove,
		e.saber.G2MarksShader,
		e.saber.G2WeaponMarkShader,
	}
	wantModel := []string{
		"slap_ui",
		"ready_ui",
		"jump_up_ui",
		"jump_fwd_ui",
		"lunge_ui",
		"shader/g2_marks_ui",
		"shader/g2_weapon_ui",
	}
	for i := range wantModel {
		if gotModel[i] != wantModel[i] {
			t.Fatalf("UI-to-model field %d mismatch: got %q want %q", i, gotModel[i], wantModel[i])
		}
	}
}

func TestRecoveryAutosaveIncludesDetachedEditor(t *testing.T) {
	root := t.TempDir()
	app := newRecoveryTestApp(t, filepath.Join(root, "recovery"), filepath.Join(root, "config"))
	editor := NewSABEditor(app)
	if err := editor.LoadFile(writeSab(t, multiSabFixture)); err != nil {
		t.Fatal(err)
	}
	editor.nameEntry.SetText("detached_dirty")
	editor.session.flushPending()
	app.detachedEditors = map[Editor]string{editor: "Detached Saber"}

	app.autosaveRecoveryPass()
	entries, err := app.recovery.List()
	if err != nil || len(entries) != 1 {
		t.Fatalf("detached dirty editor was not autosaved: entries=%+v err=%v", entries, err)
	}
	if entries[0].Title != filepath.Base(editor.GetCurrentPath()) || !strings.Contains(entries[0].Working, "detached_dirty") {
		t.Fatalf("detached recovery snapshot did not contain live editor state: %+v", entries[0])
	}
}

func findTooltipButton(t *testing.T, root fyne.CanvasObject, label string) *TooltipButton {
	t.Helper()
	seen := make(map[fyne.CanvasObject]bool)
	var visit func(fyne.CanvasObject) *TooltipButton
	visit = func(object fyne.CanvasObject) *TooltipButton {
		if object == nil || seen[object] {
			return nil
		}
		seen[object] = true
		if button, ok := object.(*TooltipButton); ok && button.Text == label {
			return button
		}
		var children []fyne.CanvasObject
		switch value := object.(type) {
		case *fyne.Container:
			children = value.Objects
		case fyne.Widget:
			children = test.WidgetRenderer(value).Objects()
		}
		for _, child := range children {
			if found := visit(child); found != nil {
				return found
			}
		}
		return nil
	}
	button := visit(root)
	if button == nil {
		t.Fatalf("tooltip button %q not found", label)
	}
	return button
}

func tornOutWindow(t *testing.T, app *App) fyne.Window {
	t.Helper()
	var found fyne.Window
	for _, window := range app.fyneApp.Driver().AllWindows() {
		if window != app.mainWindow && strings.HasSuffix(window.Title(), " — MBII Foundry") {
			found = window
		}
	}
	if found != nil {
		return found
	}
	t.Fatal("torn-out editor window not found")
	return nil
}

func TestTornOutReattachPreservesAndConfirmedDiscardRemoves(t *testing.T) {
	root := t.TempDir()
	app := newRecoveryTestApp(t, filepath.Join(root, "recovery"), filepath.Join(root, "config"))
	editor := NewSABEditor(app)
	if err := editor.LoadFile(writeSab(t, multiSabFixture)); err != nil {
		t.Fatal(err)
	}
	editor.nameEntry.SetText("unsaved_torn_out")
	editor.session.flushPending()
	app.sourceDrafts.Set(editor, SourceDraft{Text: "unsaved source", EditMode: true})
	app.captureRecoveryEntry(editor)
	tab := container.NewTabItem("Torn Saber", editor.GetContent())
	app.editors[tab] = editor
	app.docTabs.Append(tab)
	app.docTabs.Select(tab)

	app.popOutCurrentTab()
	firstWindow := tornOutWindow(t, app)
	findTooltipButton(t, firstWindow.Content(), "Reattach tab").OnTapped()
	if len(app.detachedEditors) != 0 || len(app.editors) != 1 {
		t.Fatalf("reattach did not restore exactly one tab: detached=%d attached=%d", len(app.detachedEditors), len(app.editors))
	}
	if !app.sourceDrafts.Has(editor) {
		t.Fatal("reattach discarded the editor source draft")
	}
	if entries, _ := app.recovery.List(); len(entries) != 1 {
		t.Fatal("reattach removed the recovery snapshot")
	}

	app.popOutCurrentTab()
	secondWindow := tornOutWindow(t, app)
	app.confirmDetachedEditorClose(editor, secondWindow, func() {
		app.discardDetachedEditor(editor)
		secondWindow.Close()
	})
	confirm := secondWindow.Canvas().Overlays().Top()
	if confirm == nil {
		t.Fatal("dirty torn-out close did not request discard confirmation")
	}
	test.Tap(saveDialogButton(t, confirm, "Yes"))
	if len(app.detachedEditors) != 0 || len(app.editors) != 0 {
		t.Fatalf("confirmed discard reattached or retained editor: detached=%d attached=%d", len(app.detachedEditors), len(app.editors))
	}
	if app.sourceDrafts.Has(editor) {
		t.Fatal("confirmed discard retained the source draft")
	}
	if entries, _ := app.recovery.List(); len(entries) != 0 {
		t.Fatal("confirmed discard retained the recovery snapshot")
	}
}
