package main

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"

	"github.com/Frenzeh/mbii-foundry/parsers"
)

// newTestBulkApp builds an App whose FileManager stores backups in an
// isolated config dir and whose window is the dialog parent.
func newTestBulkApp(app fyne.App, configDir string) *App {
	a := &App{
		fyneApp:    app,
		mainWindow: app.NewWindow("bulk owner"),
		configPath: filepath.Join(configDir, "config.json"),
	}
	a.fileManager = NewFileManager(configDir)
	a.statusLabel = widget.NewLabel("ready") // non-empty: skips layout rebuild in updateStatus
	return a
}

// writeBulkFixture writes content to dir/name and returns the path.
func writeBulkFixture(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	return path
}

const bulkMBCHFixture = `// top comment must survive
description	"A veteran pilot of the outer rim"

ClassInfo
{
	name			"veteran"
	// inline comment inside ClassInfo must survive
	maxhealth		100
	speed			1.2
}

WeaponInfo0
{
	WeaponToReplace	 WP_MELEE
	customAmmo		20
}
`

func TestBulkEditCanonicalReplacePreservesDocument(t *testing.T) {
	dir := t.TempDir()
	path := writeBulkFixture(t, dir, "vet.mbch", bulkMBCHFixture)

	plan, err := planFile(path, "maxhealth", "250")
	if err != nil {
		t.Fatalf("planFile: %v", err)
	}

	out := plan.content
	if !strings.Contains(out, "maxhealth") {
		t.Fatalf("maxhealth missing from output:\n%s", out)
	}
	if strings.Contains(out, "maxhealth\t\t100") {
		t.Fatalf("old maxhealth value survived:\n%s", out)
	}
	for _, marker := range []string{"// top comment must survive", "// inline comment inside ClassInfo must survive"} {
		if !strings.Contains(out, marker) {
			t.Errorf("comment lost after bulk edit: %q\noutput:\n%s", marker, out)
		}
	}
	if !strings.Contains(out, "WeaponInfo0") || !strings.Contains(out, "customAmmo") {
		t.Errorf("override block lost:\n%s", out)
	}
	if plan.result.OldValue != "100" || plan.result.Scope != "ClassInfo" {
		t.Errorf("result metadata: old=%q scope=%q", plan.result.OldValue, plan.result.Scope)
	}
}

func TestBulkEditMultiwordValueStaysQuoted(t *testing.T) {
	dir := t.TempDir()
	path := writeBulkFixture(t, dir, "vet.mbch", bulkMBCHFixture)

	plan, err := planFile(path, "description", "Ghost Protocol strikes at dawn")
	if err != nil {
		t.Fatalf("planFile: %v", err)
	}

	char, err := parsers.ParseMBCH(plan.content)
	if err != nil {
		t.Fatalf("re-parse: %v", err)
	}
	if char.Description != "Ghost Protocol strikes at dawn" {
		t.Fatalf("multiword value truncated: %q", char.Description)
	}
}

func TestBulkEditInsertGoesIntoClassInfoNotTopLevel(t *testing.T) {
	dir := t.TempDir()
	path := writeBulkFixture(t, dir, "vet.mbch", bulkMBCHFixture)

	plan, err := planFile(path, "maxarmor", "75")
	if err != nil {
		t.Fatalf("planFile: %v", err)
	}
	if plan.result.OldValue != "" {
		t.Fatal("maxarmor should not pre-exist")
	}
	if plan.result.Scope != "ClassInfo" {
		t.Fatalf("insert scope: %q, want ClassInfo", plan.result.Scope)
	}

	char := mustParseMBCH(t, plan.content)
	if char.MaxArmor != 75 {
		t.Fatalf("maxarmor not applied: %v", char.MaxArmor)
	}
	// Engine reads non-description fields from ClassInfo; a top-level
	// insertion would be invisible. Assert nothing precedes the
	// ClassInfo group but comments/description.
	for _, line := range strings.Split(plan.content, "\n") {
		if strings.HasPrefix(line, "ClassInfo") {
			break
		}
		if strings.HasPrefix(line, "maxarmor") {
			t.Fatalf("key inserted at top level where the engine ignores it:\n%s", plan.content)
		}
	}
}

// mustParseMBCH is a tiny test helper wrapping parsers.ParseMBCH.
func mustParseMBCH(t *testing.T, content string) *parsers.MBCHCharacter {
	t.Helper()
	char, err := parsers.ParseMBCH(content)
	if err != nil {
		t.Fatalf("ParseMBCH: %v", err)
	}
	return char
}

func TestBulkEditOverrideScopedKeyRefused(t *testing.T) {
	dir := t.TempDir()
	path := writeBulkFixture(t, dir, "vet.mbch", bulkMBCHFixture)

	if _, err := planFile(path, "customAmmo", "99"); err == nil {
		t.Fatal("bulk edit of a WeaponInfo-only key must be refused")
	} else if !strings.Contains(err.Error(), "WeaponInfo0") {
		t.Fatalf("refusal should name the owning override: %v", err)
	}
}

func TestBulkEditSABRetainsSiblingDefinitions(t *testing.T) {
	dir := t.TempDir()
	content := `saber_a
{
	name	"Alpha"
	saberType	SABER_SINGLE
}

saber_b
{
	name	"Beta"
	saberType	SABER_SINGLE
}
`
	path := writeBulkFixture(t, dir, "two.sab", content)

	plan, err := planFile(path, "name", "AlphaPrime")
	if err != nil {
		t.Fatalf("planFile: %v", err)
	}
	if plan.result.Defs != 2 {
		t.Fatalf("definitions: %d, want 2", plan.result.Defs)
	}
	if !strings.Contains(plan.result.Scope, "definition 1 of 2") {
		t.Fatalf("scope should disclose multiplicity: %q", plan.result.Scope)
	}
	if !strings.Contains(plan.content, "saber_b") || !strings.Contains(plan.content, `"Beta"`) {
		t.Fatalf("sibling definition lost:\n%s", plan.content)
	}
}

func TestBulkEditConsumerValidationRejectsOversizedValue(t *testing.T) {
	dir := t.TempDir()
	path := writeBulkFixture(t, dir, "vet.mbch", bulkMBCHFixture)

	// A 3000-byte ClassInfo value cannot fit the engine's 2048-byte
	// paired-value buffer (bg_saga.h SIEGE_PARSE_BUF_LEN); the consumer
	// gate must refuse before anything is written.
	huge := strings.Repeat("x", 3000)
	before, _ := os.ReadFile(path)
	if _, err := planFile(path, "maxarmor", huge); err == nil {
		t.Fatal("oversized value must fail consumer validation")
	}
	after, _ := os.ReadFile(path)
	if string(before) != string(after) {
		t.Fatal("rejected plan modified the file")
	}
}

func TestBulkEditLexicallyImpossibleValueRejected(t *testing.T) {
	dir := t.TempDir()
	path := writeBulkFixture(t, dir, "vet.mbch", bulkMBCHFixture)

	for _, bad := range []string{`say "hi"`, "a//b", "two\nlines"} {
		if _, err := planFile(path, "description", bad); err == nil {
			t.Errorf("value %q must be rejected (engine cannot represent it)", bad)
		}
	}
}

func TestBulkEditorBatchRollsBackOnInjectedBackupFailure(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	dir := t.TempDir()
	configDir := t.TempDir()

	orig1 := "ClassInfo\n{\n\tname\t\t\"one\"\n\tmaxhealth\t\t100\n}\n"
	orig2 := "ClassInfo\n{\n\tname\t\t\"two\"\n\tmaxhealth\t\t100\n}\n"
	orig3 := "ClassInfo\n{\n\tname\t\t\"three\"\n\tmaxhealth\t\t100\n}\n"
	first := writeBulkFixture(t, dir, "one.mbch", orig1)
	second := writeBulkFixture(t, dir, "two.mbch", orig2)
	failing := writeBulkFixture(t, dir, "three.mbch", orig3)

	be := NewBulkEditor(newTestBulkApp(app, configDir))
	be.LoadFiles([]string{first, second, failing})
	first, _ = canonicalBulkPath(first)
	second, _ = canonicalBulkPath(second)
	failing, _ = canonicalBulkPath(failing)
	be.selection[first] = true
	be.selection[second] = true
	be.selection[failing] = true
	be.fieldEntry.SetText("maxhealth")
	be.valueEntry.SetText("200")

	be.runPreview()
	if len(be.previewData) != 3 {
		t.Fatalf("preview rows: %d, want 3", len(be.previewData))
	}
	be.afterCandidatePublish = func(path, _, backupPath string) {
		if path != failing {
			return
		}
		if backupPath == "" {
			t.Fatal("existing destination publication omitted its backup path")
		}
		if err := os.WriteFile(backupPath, []byte("corrupt backup"), 0600); err != nil {
			t.Fatalf("inject backup proof failure: %v", err)
		}
		be.afterCandidatePublish = nil
	}
	be.applyChanges()

	// All-or-nothing: both earlier writes and the write whose final backup
	// proof failed must be restored byte-exact.
	for path, orig := range map[string]string{first: orig1, second: orig2, failing: orig3} {
		got, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if string(got) != orig {
			t.Errorf("%s was not restored after batch failure:\n%s", filepath.Base(path), got)
		}
	}

	// Backups live in the accepted FileManager backup store.
	backupDir := filepath.Join(configDir, "backups")
	entries, err := os.ReadDir(backupDir)
	if err != nil || len(entries) < 3 {
		t.Fatalf("expected retained FileManager backups, got %d (%v)", len(entries), err)
	}

	rows := make(map[string]BulkFieldResult)
	for _, row := range be.previewData {
		rows[row.Path] = row
	}
	for _, path := range []string{first, second} {
		if row := rows[path]; !row.RolledBack || row.Err != nil {
			t.Errorf("prior write %s was not cleanly rolled back: %+v", filepath.Base(path), row)
		}
	}
	if row := rows[failing]; row.Err == nil || !row.RolledBack {
		t.Errorf("injected backup failure outcome is incomplete: %+v", row)
	}
}

func TestBulkEditorPostPublicationFailureRollsBackCurrentAndEarlierWrites(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	dir := t.TempDir()
	configDir := t.TempDir()

	orig1 := "ClassInfo\n{\n\tname\t\t\"one\"\n\tmaxhealth\t\t100\n}\n"
	orig2 := "ClassInfo\n{\n\tname\t\t\"two\"\n\tmaxhealth\t\t100\n}\n"
	orig3 := "ClassInfo\n{\n\tname\t\t\"three\"\n\tmaxhealth\t\t100\n}\n"
	first := writeBulkFixture(t, dir, "one.mbch", orig1)
	conflicted := writeBulkFixture(t, dir, "two.mbch", orig2)
	failing := writeBulkFixture(t, dir, "three.mbch", orig3)

	be := NewBulkEditor(newTestBulkApp(app, configDir))
	be.LoadFiles([]string{first, conflicted, failing})
	first, _ = canonicalBulkPath(first)
	conflicted, _ = canonicalBulkPath(conflicted)
	failing, _ = canonicalBulkPath(failing)
	for _, path := range []string{first, conflicted, failing} {
		be.selection[path] = true
	}
	be.fieldEntry.SetText("maxhealth")
	be.valueEntry.SetText("200")
	be.runPreview()

	external := "ClassInfo\n{\n\tname\t\t\"external\"\n\tmaxhealth\t\t999\n}\n"
	be.afterCandidatePublish = func(path, holdPath, backupPath string) {
		if path != failing {
			return
		}
		if holdPath == "" || backupPath == "" {
			t.Fatalf("existing destination publication omitted recovery paths: hold=%q backup=%q", holdPath, backupPath)
		}
		if err := os.WriteFile(conflicted, []byte(external), 0644); err != nil {
			t.Fatalf("inject external update: %v", err)
		}
		if err := os.WriteFile(backupPath, []byte("corrupt backup"), 0600); err != nil {
			t.Fatalf("force final backup proof failure: %v", err)
		}
		be.afterCandidatePublish = nil
	}
	be.applyChanges()

	for path, want := range map[string]string{
		first:      orig1,
		conflicted: external,
		failing:    orig3,
	} {
		got, err := os.ReadFile(path)
		if err != nil || string(got) != want {
			t.Errorf("%s bytes = %q, err=%v; want %q", filepath.Base(path), got, err, want)
		}
	}

	rows := make(map[string]BulkFieldResult)
	for _, row := range be.previewData {
		rows[row.Path] = row
	}
	if row := rows[first]; !row.RolledBack || row.Err != nil {
		t.Errorf("earlier write was not cleanly rolled back: %+v", row)
	}
	if row := rows[conflicted]; !row.RollbackConflict || row.RolledBack {
		t.Errorf("external update was not preserved as a rollback conflict: %+v", row)
	}
	if row := rows[failing]; !row.RolledBack || row.NoRollbackNeeded || row.Err == nil ||
		!strings.Contains(row.Err.Error(), "candidate was published") ||
		!saveWasPublished(row.Err) {
		t.Errorf("published failing file was omitted from rollback: %+v", row)
	}
	failingRowID := -1
	for id, row := range be.previewData {
		if row.Path == failing {
			failingRowID = id
			break
		}
	}
	if failingRowID < 0 {
		t.Fatal("published-then-rolled-back row is missing")
	}
	rowObject := be.previewList.CreateItem()
	be.previewList.UpdateItem(failingRowID, rowObject)
	detail := rowObject.(*fyne.Container).Objects[2].(*widget.Label).Text
	if !strings.Contains(detail, "written then rolled back") ||
		!strings.Contains(detail, "triggering error") {
		t.Fatalf("published failure rollback outcome is hidden in the UI: %q", detail)
	}
	if status := be.statusLabel.Text; !strings.Contains(status, "rolled back 2 file(s)") ||
		!strings.Contains(status, "1 changed after apply") {
		t.Fatalf("batch status does not report rollback and preserved conflict: %q", status)
	}
}

func TestBulkRollbackTreatsModeMutationAsConflict(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows does not expose POSIX permission modes")
	}

	app := test.NewApp()
	defer app.Quit()
	dir := t.TempDir()
	configDir := t.TempDir()
	original := "ClassInfo\n{\n\tname\t\t\"one\"\n\tmaxhealth\t\t100\n}\n"
	failingOriginal := "ClassInfo\n{\n\tname\t\t\"two\"\n\tmaxhealth\t\t100\n}\n"
	modeChanged := writeBulkFixture(t, dir, "one.mbch", original)
	failing := writeBulkFixture(t, dir, "two.mbch", failingOriginal)
	if err := os.Chmod(modeChanged, 0644); err != nil {
		t.Fatal(err)
	}

	be := NewBulkEditor(newTestBulkApp(app, configDir))
	be.LoadFiles([]string{modeChanged, failing})
	modeChanged, _ = canonicalBulkPath(modeChanged)
	failing, _ = canonicalBulkPath(failing)
	be.selection[modeChanged] = true
	be.selection[failing] = true
	be.fieldEntry.SetText("maxhealth")
	be.valueEntry.SetText("200")
	be.runPreview()
	be.afterCandidatePublish = func(path, _, backupPath string) {
		if path != failing {
			return
		}
		if err := os.Chmod(modeChanged, 0600); err != nil {
			t.Fatalf("inject mode-only external update: %v", err)
		}
		if err := os.WriteFile(backupPath, []byte("corrupt backup"), 0600); err != nil {
			t.Fatalf("force batch failure: %v", err)
		}
		be.afterCandidatePublish = nil
	}
	be.applyChanges()

	modeChangedBytes, err := os.ReadFile(modeChanged)
	if err != nil || !strings.Contains(string(modeChangedBytes), "maxhealth\t\t200") {
		t.Fatalf("mode-changed candidate was overwritten during rollback: %q, %v", modeChangedBytes, err)
	}
	info, err := os.Stat(modeChanged)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0600 {
		t.Fatalf("external mode change was not preserved: mode=%v", info.Mode().Perm())
	}
	if got, err := os.ReadFile(failing); err != nil || string(got) != failingOriginal {
		t.Fatalf("failing file was not rolled back: %q, %v", got, err)
	}

	rows := make(map[string]BulkFieldResult)
	for _, row := range be.previewData {
		rows[row.Path] = row
	}
	if row := rows[modeChanged]; !row.RollbackConflict || row.RolledBack {
		t.Fatalf("mode mutation was not reported as a rollback conflict: %+v", row)
	}
	if row := rows[failing]; !row.RolledBack {
		t.Fatalf("published failing file was not rolled back: %+v", row)
	}
}

func TestBulkEditorConsecutiveBatchesRetainOriginals(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	configDir := t.TempDir()
	dir := t.TempDir()
	path := writeBulkFixture(t, dir, "vet.mbch", "ClassInfo\n{\n\tmaxhealth\t\t100\n}\n")

	be := NewBulkEditor(newTestBulkApp(app, configDir))
	be.LoadFiles([]string{path})
	be.selection[path] = true

	backupDir := filepath.Join(configDir, "backups")
	countBackups := func() int {
		entries, _ := os.ReadDir(backupDir)
		return len(entries)
	}

	be.fieldEntry.SetText("maxhealth")
	applyWithValue := func(value string) {
		be.valueEntry.SetText(value)
		be.runPreview()
		be.applyChanges()
	}

	applyWithValue("150")
	first := countBackups()
	if first != 1 {
		t.Fatalf("after batch 1: %d backups, want 1", first)
	}

	applyWithValue("250")
	second := countBackups()
	if second != 2 {
		t.Fatalf("after batch 2: %d backups, want 2 — batch 1's original must be retained", second)
	}

	got, _ := os.ReadFile(path)
	if !strings.Contains(string(got), "250") {
		t.Fatalf("second apply not written:\n%s", got)
	}
}

func TestBulkEditorRefusesStalePreview(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	dir := t.TempDir()
	path := writeBulkFixture(t, dir, "vet.mbch", "ClassInfo\n{\n\tmaxhealth\t\t100\n}\n")

	be := NewBulkEditor(newTestBulkApp(app, t.TempDir()))
	be.LoadFiles([]string{path})
	be.selection[path] = true
	be.fieldEntry.SetText("maxhealth")
	be.valueEntry.SetText("200")
	be.runPreview()

	other := "ClassInfo\n{\n\tmaxhealth\t\t150\n\tmaxarmor\t\t75\n}\n"
	if err := os.WriteFile(path, []byte(other), 0644); err != nil {
		t.Fatal(err)
	}

	be.applyChanges()

	if len(be.previewData) != 1 || be.previewData[0].Err == nil {
		t.Fatalf("stale apply must record an error: %+v", be.previewData)
	}
	if !strings.Contains(be.previewData[0].Err.Error(), "changed since preview") {
		t.Fatalf("stale error should explain the transition: %v", be.previewData[0].Err)
	}
	got, _ := os.ReadFile(path)
	if string(got) != other {
		t.Fatal("stale apply wrote over a file it did not preview")
	}
}

func TestBulkEditorRejectsMutationAtPublicationBoundary(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	configDir := t.TempDir()
	path := writeBulkFixture(t, t.TempDir(), "vet.mbch", "ClassInfo\n{\n\tmaxhealth\t\t100\n}\n")
	canonicalPath, err := canonicalBulkPath(path)
	if err != nil {
		t.Fatal(err)
	}

	be := NewBulkEditor(newTestBulkApp(app, configDir))
	be.LoadFiles([]string{path})
	be.selection[path] = true
	be.fieldEntry.SetText("maxhealth")
	be.valueEntry.SetText("200")
	be.runPreview()

	external := []byte("ClassInfo\n{\n\tmaxhealth\t\t999\n}\n")
	be.beforePublication = func(gotPath string) {
		if gotPath != canonicalPath {
			t.Fatalf("publication hook path = %q, want %q", gotPath, canonicalPath)
		}
		if err := os.WriteFile(path, external, 0644); err != nil {
			t.Fatal(err)
		}
		be.beforePublication = nil
	}
	be.applyChanges()

	if got, err := os.ReadFile(path); err != nil || string(got) != string(external) {
		t.Fatalf("external update was overwritten: %q, %v", got, err)
	}
	if len(be.previewData) != 1 || be.previewData[0].Err == nil ||
		!strings.Contains(be.previewData[0].Err.Error(), "changed since preview") {
		t.Fatalf("publication mutation was not rejected: %+v", be.previewData)
	}
	entries, err := os.ReadDir(filepath.Join(configDir, "backups"))
	if err != nil || len(entries) != 1 {
		t.Fatalf("rejected publication must retain one preview-original backup: %d, %v", len(entries), err)
	}
	backup, err := os.ReadFile(filepath.Join(configDir, "backups", entries[0].Name()))
	if err != nil || string(backup) != "ClassInfo\n{\n\tmaxhealth\t\t100\n}\n" {
		t.Fatalf("backup did not preserve the exact preview original: %q, %v", backup, err)
	}
}

func TestBulkEditorPreviewApplyTransition(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	dir := t.TempDir()
	path := writeBulkFixture(t, dir, "vet.mbch", "ClassInfo\n{\n\tmaxhealth\t\t100\n}\n")

	be := NewBulkEditor(newTestBulkApp(app, t.TempDir()))
	be.LoadFiles([]string{path})
	be.selection[path] = true
	be.fieldEntry.SetText("maxhealth")
	be.valueEntry.SetText("200")

	be.runPreview()
	if be.previewData[0].OldValue != "100" || be.previewData[0].NewValue != "200" {
		t.Fatalf("preview: %+v", be.previewData[0])
	}

	be.applyChanges()
	got, _ := os.ReadFile(path)
	if !strings.Contains(string(got), "200") {
		t.Fatalf("apply did not write the previewed value:\n%s", got)
	}

	// Second preview observes the applied state — the transition loop.
	be.runPreview()
	if be.previewData[0].OldValue != "200" {
		t.Fatalf("re-preview after apply: old=%q, want 200", be.previewData[0].OldValue)
	}
}

func TestBulkEditorPreviewInvalidatedByInputs(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	dir := t.TempDir()
	path := writeBulkFixture(t, dir, "vet.mbch", "ClassInfo\n{\n\tmaxhealth\t\t100\n}\n")

	be := NewBulkEditor(newTestBulkApp(app, t.TempDir()))
	be.LoadFiles([]string{path})
	be.selection[path] = true
	be.fieldEntry.SetText("maxhealth")
	be.valueEntry.SetText("200")
	be.runPreview()
	if be.applyBtn.Disabled() {
		t.Fatal("clean preview must enable Apply")
	}

	// Value change invalidates: Apply disables, plans are dropped, and a
	// forced apply refuses instead of consuming the invalidated content.
	be.valueEntry.SetText("250")
	if !be.applyBtn.Disabled() {
		t.Fatal("value change must disable Apply")
	}
	be.applyBtn.Enable() // bypass the UI guard: the parameter gate is under test
	be.applyChanges()
	got, _ := os.ReadFile(path)
	if strings.Contains(string(got), "250") {
		t.Fatalf("apply consumed invalidated plan content:\n%s", got)
	}
	if !strings.Contains(be.statusLabel.Text, "no current preview") {
		t.Fatalf("apply must refuse without a current preview, got status: %q", be.statusLabel.Text)
	}

	// Re-preview with the new value is what authorizes the write.
	be.runPreview()
	be.applyChanges()
	got, _ = os.ReadFile(path)
	if !strings.Contains(string(got), "250") {
		t.Fatalf("re-previewed apply did not write:\n%s", got)
	}
}

func TestBulkEditorFailedPreviewDisablesApply(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	dir := t.TempDir()
	good := writeBulkFixture(t, dir, "good.mbch", "ClassInfo\n{\n\tmaxhealth\t\t100\n}\n")

	be := NewBulkEditor(newTestBulkApp(app, t.TempDir()))
	be.LoadFiles([]string{good})
	be.selection[good] = true
	be.fieldEntry.SetText("maxhealth")
	be.valueEntry.SetText("200")
	be.runPreview()
	if be.applyBtn.Disabled() {
		t.Fatal("clean preview must enable Apply")
	}

	// Selection now includes a supported, parseable file whose format cannot
	// place this field — the whole preview fails and Apply must NOT stay
	// enabled from the prior pass.
	unplannable := writeBulkFixture(t, dir, "map.siege", "Teams\n{\n\tteam1\tRed\n}\n")
	be.LoadFiles([]string{good, unplannable})
	be.selection[good] = true
	be.selection[unplannable] = true
	be.fieldEntry.SetText("maxhealth")
	be.valueEntry.SetText("200")
	be.runPreview()

	if !be.applyBtn.Disabled() {
		t.Fatal("failed preview must disable Apply instead of leaving stale write access")
	}
}

func TestBulkEditorParentWindowIsRespected(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	win := app.NewWindow("test owner")
	be := NewBulkEditor(&App{fyneApp: app, mainWindow: win})
	if be.parent() != win {
		t.Fatal("dialogs must target the wired owner window")
	}
}

func TestBulkEditorRequiresAppForApply(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	dir := t.TempDir()
	path := writeBulkFixture(t, dir, "vet.mbch", "ClassInfo\n{\n\tmaxhealth\t\t100\n}\n")

	// No FileManager wired: Apply refuses instead of writing unbacked.
	be := NewBulkEditor(&App{fyneApp: app})
	be.LoadFiles([]string{path})
	be.selection[path] = true
	be.fieldEntry.SetText("maxhealth")
	be.valueEntry.SetText("200")
	be.runPreview()
	be.applyChanges()

	got, _ := os.ReadFile(path)
	if strings.Contains(string(got), "200") {
		t.Fatal("apply without backup capability must not write")
	}
	if !strings.Contains(be.statusLabel.Text, "no FileManager") {
		t.Fatalf("status must explain the refusal: %q", be.statusLabel.Text)
	}
}

func TestBulkParametersMatchesDetectsEqualCardinalitySwap(t *testing.T) {
	p := &bulkParameters{
		key:       "maxhealth",
		value:     "200",
		selection: map[string]bool{"/a.mbch": true, "/b.mbch": false},
	}
	// Same cardinality, different member: must NOT match.
	if p.matches("maxhealth", "200", map[string]bool{"/a.mbch": false, "/b.mbch": true}) {
		t.Fatal("equal-length swapped selection must not match the captured plan")
	}
	// Identical selection matches.
	if !p.matches("maxhealth", "200", map[string]bool{"/a.mbch": true, "/b.mbch": false}) {
		t.Fatal("identical selection must match")
	}
}

func TestBulkEditorRefusesSelectionSwapAfterPreview(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	dir := t.TempDir()
	a := writeBulkFixture(t, dir, "a.mbch", "ClassInfo\n{\n\tmaxhealth\t\t100\n}\n")
	b := writeBulkFixture(t, dir, "b.mbch", "ClassInfo\n{\n\tmaxhealth\t\t100\n}\n")

	be := NewBulkEditor(newTestBulkApp(app, t.TempDir()))
	be.LoadFiles([]string{a, b})
	be.selection[a] = true
	be.fieldEntry.SetText("maxhealth")
	be.valueEntry.SetText("200")
	be.runPreview()

	// Equal-cardinality selection swap after preview: A out, B in.
	be.selection[a] = false
	be.selection[b] = true
	be.applyBtn.Enable() // bypass the UI guard: the parameter gate is under test
	be.applyChanges()

	if !strings.Contains(be.statusLabel.Text, "different field, value, or selection") {
		t.Fatalf("swapped selection must be refused, got status: %q", be.statusLabel.Text)
	}
	for _, path := range []string{a, b} {
		got, _ := os.ReadFile(path)
		if strings.Contains(string(got), "200") {
			t.Fatalf("%s was written from another file's preview", filepath.Base(path))
		}
	}
}

func TestBulkPreviewDistinguishesExplicitZeroFromMissing(t *testing.T) {
	dir := t.TempDir()
	withZero := writeBulkFixture(t, dir, "zero.mbtc", "name \"Zero\"\nEUAllowed 0\nClasses\n{\n class1 c\n}\n")
	missing := writeBulkFixture(t, dir, "missing.mbtc", "name \"Missing\"\nClasses\n{\n class1 c\n}\n")

	zeroPlan, err := planFile(withZero, "EUAllowed", "1")
	if err != nil {
		t.Fatal(err)
	}
	missingPlan, err := planFile(missing, "EUAllowed", "1")
	if err != nil {
		t.Fatal(err)
	}
	if !zeroPlan.result.OldFound || zeroPlan.result.OldValue != "0" {
		t.Fatalf("explicit zero must be captured as present: %+v", zeroPlan.result)
	}
	if missingPlan.result.OldFound || missingPlan.result.OldValue != "" {
		t.Fatalf("missing field must remain distinct: %+v", missingPlan.result)
	}
}

func TestBulkNoOpNeedsNoBackup(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	configDir := t.TempDir()
	path := writeBulkFixture(t, t.TempDir(), "same.mbch", "ClassInfo\n{\n\tmaxhealth\t100\n}\n")

	be := NewBulkEditor(newTestBulkApp(app, configDir))
	be.LoadFiles([]string{path})
	be.selection[path] = true
	be.fieldEntry.SetText("maxhealth")
	be.valueEntry.SetText("100")
	be.runPreview()
	be.applyChanges()

	if len(be.previewData) != 1 || !be.previewData[0].NoRollbackNeeded || be.previewData[0].BackupPath != "" {
		t.Fatalf("no-op outcome must explicitly need no rollback or backup: %+v", be.previewData)
	}
	if _, err := os.Stat(filepath.Join(configDir, "backups")); !os.IsNotExist(err) {
		t.Fatalf("no-op unexpectedly created a backup store: %v", err)
	}
}

func TestBulkEditMBTCUnknownFieldUsesPreservedAST(t *testing.T) {
	source := "name \"Team\"\nCustomFlag old // keep comment\nClasses\n{\n\tclass1 c\n}\n"
	edit, err := parsers.EditConfigField(parsers.FormatMBTC, source, "CustomFlag", "new")
	if err != nil {
		t.Fatal(err)
	}
	if edit.OldValue != "old" || !edit.Found {
		t.Fatalf("old unknown field not captured: %+v", edit)
	}
	if !strings.Contains(edit.Content, "CustomFlag new // keep comment") {
		t.Fatalf("unknown field edit did not preserve its AST line:\n%s", edit.Content)
	}
}

func TestScanBulkFolderRecursiveAndUncapped(t *testing.T) {
	root := t.TempDir()
	nested := filepath.Join(root, "nested", "deeper")
	if err := os.MkdirAll(nested, 0755); err != nil {
		t.Fatal(err)
	}
	const fileCount = 137
	for i := range fileCount {
		name := filepath.Join(nested, fmt.Sprintf("class-%03d.mbch", i))
		if err := os.WriteFile(name, []byte("ClassInfo\n{\n\tmaxhealth\t100\n}\n"), 0644); err != nil {
			t.Fatal(err)
		}
	}
	unsupported := writeBulkFixture(t, root, "notes.txt", "not a config")

	result := scanBulkFolder(root)
	if len(result.Files) != fileCount {
		t.Fatalf("recursive scan returned %d files, want all %d", len(result.Files), fileCount)
	}
	if len(result.Unsupported) != 1 || result.Unsupported[0] != unsupported {
		t.Fatalf("unsupported report = %#v, want exact path %q", result.Unsupported, unsupported)
	}
	if len(result.Errors) != 0 {
		t.Fatalf("unexpected scan errors: %+v", result.Errors)
	}
}

func TestScanBulkFolderDoesNotFollowSymlinkEscapes(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	outsideFile := writeBulkFixture(t, outside, "outside.mbch", "ClassInfo\n{\n\tmaxhealth\t100\n}\n")
	fileLink := filepath.Join(root, "linked.mbch")
	dirLink := filepath.Join(root, "linked-folder")
	if err := os.Symlink(outsideFile, fileLink); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, dirLink); err != nil {
		t.Fatal(err)
	}

	result := scanBulkFolder(root)
	if len(result.Files) != 0 {
		t.Fatalf("symlink escape entered batch: %#v", result.Files)
	}
	if len(result.Symlinks) != 2 {
		t.Fatalf("symlink report = %#v, want file and folder links", result.Symlinks)
	}
}

func TestDiscoverBulkFilesCanonicalizesAndDeduplicates(t *testing.T) {
	root := t.TempDir()
	path := writeBulkFixture(t, root, "unit.mbch", "ClassInfo\n{\n\tmaxhealth\t100\n}\n")
	alias := filepath.Join(root, "alias.mbch")
	if err := os.Symlink(path, alias); err != nil {
		t.Fatal(err)
	}

	result := discoverBulkFiles([]string{path, filepath.Join(root, ".", "unit.mbch"), alias})
	if len(result.Files) != 1 {
		t.Fatalf("canonical discovery files = %#v, want one", result.Files)
	}
	canonical, err := canonicalBulkPath(path)
	if err != nil {
		t.Fatal(err)
	}
	if result.Files[0] != canonical || len(result.Duplicates) != 2 {
		t.Fatalf("canonical=%q duplicates=%#v", result.Files[0], result.Duplicates)
	}
}

func TestDiscoverBulkFilesReportsPathErrors(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "missing.mbch")
	result := discoverBulkFiles([]string{missing})
	if len(result.Files) != 0 || len(result.Errors) != 1 {
		t.Fatalf("missing path discovery = %+v", result)
	}
	if result.Errors[0].Path != missing {
		t.Fatalf("error lost exact path: %+v", result.Errors[0])
	}
}

func TestBulkLoaderUIStateReportsSkipsAndErrors(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	root := t.TempDir()
	good := writeBulkFixture(t, root, "good.mbch", "ClassInfo\n{\n\tmaxhealth\t100\n}\n")
	bad := writeBulkFixture(t, root, "broken.mbch", "ClassInfo\n{\n\tname\t\"unterminated\n}\n")
	unsupported := writeBulkFixture(t, root, "notes.txt", "not a config")
	canonicalGood, err := canonicalBulkPath(good)
	if err != nil {
		t.Fatal(err)
	}
	canonicalBad, err := canonicalBulkPath(bad)
	if err != nil {
		t.Fatal(err)
	}
	canonicalUnsupported, err := canonicalBulkPath(unsupported)
	if err != nil {
		t.Fatal(err)
	}

	be := NewBulkEditor(newTestBulkApp(app, t.TempDir()))
	if be.addFileBtn.Text != "Add File…" || be.addFolderBtn.Text != "Add Folder…" {
		t.Fatalf("loader controls are not clear: %q / %q", be.addFileBtn.Text, be.addFolderBtn.Text)
	}
	be.LoadFiles([]string{good, bad, unsupported})
	if len(be.files) != 1 || be.files[0] != canonicalGood {
		t.Fatalf("batch files = %#v, want only parsed supported file", be.files)
	}
	if !be.selection[canonicalGood] {
		t.Fatal("newly loaded file must be selected")
	}
	report := be.loadReportLabel.Text
	for _, want := range []string{canonicalBad, canonicalUnsupported, "Load errors (1)", "Skipped unsupported files (1)"} {
		if !strings.Contains(report, want) {
			t.Fatalf("load report %q does not contain %q", report, want)
		}
	}

	be.removeSelected()
	if len(be.files) != 0 {
		t.Fatalf("Remove Selected left files: %#v", be.files)
	}
	be.LoadFiles([]string{good})
	be.selection[canonicalGood] = false
	be.removeSelected()
	if len(be.files) != 1 {
		t.Fatal("Remove Selected removed an unselected file")
	}
	be.clearBatch()
	if len(be.files) != 0 || len(be.selection) != 0 {
		t.Fatalf("Clear Batch left state: files=%#v selection=%#v", be.files, be.selection)
	}
}

func TestBulkFileSupportSetIsExact(t *testing.T) {
	for _, extension := range []string{".mbch", ".SAB", ".veh", ".SiEgE", ".mbtc"} {
		if !isBulkFile("fixture" + extension) {
			t.Errorf("supported extension %q was rejected", extension)
		}
	}
	for _, extension := range []string{".txt", ".cfg", ".json", ".mbch.bak", ""} {
		if isBulkFile("fixture" + extension) {
			t.Errorf("unsupported extension %q was accepted", extension)
		}
	}
}
