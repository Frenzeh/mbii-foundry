package main

import (
	"archive/zip"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"
)

func TestModpackManagerSaveLoadProjects(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.json")
	if err := os.WriteFile(configPath, []byte("{}"), 0644); err != nil {
		t.Fatal(err)
	}

	mm := &ModpackManager{
		app:      &App{configPath: configPath},
		projects: []*Modpack{},
	}

	mm.projects = append(mm.projects, &Modpack{
		Name:    "TestMod",
		Version: "1.0",
		Path:    filepath.Join(dir, "TestMod"),
	})

	if err := mm.saveProjects(); err != nil {
		t.Fatalf("saveProjects: %v", err)
	}

	// Verify the file was written
	projectsPath := filepath.Join(dir, "fa_projects.json")
	data, err := os.ReadFile(projectsPath)
	if err != nil {
		t.Fatalf("read saved projects: %v", err)
	}

	var loaded []*Modpack
	if err := json.Unmarshal(data, &loaded); err != nil {
		t.Fatalf("unmarshal projects: %v", err)
	}
	if len(loaded) != 1 {
		t.Fatalf("expected 1 project, got %d", len(loaded))
	}
	if loaded[0].Name != "TestMod" {
		t.Errorf("project name: got %q, want TestMod", loaded[0].Name)
	}
}

func TestModpackManagerSaveProjectsRequiresConfigPath(t *testing.T) {
	mm := &ModpackManager{
		app:      &App{configPath: ""},
		projects: []*Modpack{},
	}
	if err := mm.saveProjects(); err == nil {
		t.Error("expected error when configPath is empty")
	}
}

func TestModpackManagerMalformedMetadataBlocksPersistence(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.json")
	projectsPath := filepath.Join(dir, "fa_projects.json")
	malformed := []byte(`[{"name":"kept"`)
	if err := os.WriteFile(projectsPath, malformed, 0644); err != nil {
		t.Fatal(err)
	}
	kept := &Modpack{Name: "in-memory"}
	mm := &ModpackManager{
		app:      &App{configPath: configPath},
		projects: []*Modpack{kept},
	}

	err := mm.loadProjects()
	if err == nil || !strings.Contains(err.Error(), "decode projects") {
		t.Fatalf("malformed metadata error not surfaced: %v", err)
	}
	if len(mm.projects) != 1 || mm.projects[0] != kept {
		t.Fatal("failed load changed the in-memory project list")
	}
	mm.projects = append(mm.projects, &Modpack{Name: "must not persist"})
	if err := mm.saveProjects(); err == nil || !strings.Contains(err.Error(), "original preserved") {
		t.Fatalf("persistence was not blocked after corrupt load: %v", err)
	}
	if got, err := os.ReadFile(projectsPath); err != nil || string(got) != string(malformed) {
		t.Fatalf("malformed metadata was overwritten: %q, %v", got, err)
	}
}

func TestNewModpackManagerSurfacesMetadataCorruption(t *testing.T) {
	ui := test.NewApp()
	defer ui.Quit()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "fa_projects.json"), []byte(`{"broken":`), 0644); err != nil {
		t.Fatal(err)
	}
	status := widget.NewLabel("ready")
	mm := NewModpackManager(&App{
		fyneApp:     ui,
		mainWindow:  ui.NewWindow("modpacks"),
		configPath:  filepath.Join(dir, "config.json"),
		statusLabel: status,
	})

	if mm.loadErr == nil {
		t.Fatal("constructor discarded the metadata load error")
	}
	if !strings.Contains(status.Text, "Modpack metadata could not be loaded") ||
		!strings.Contains(status.Text, "decode projects") {
		t.Fatalf("metadata corruption was not surfaced to the user: %q", status.Text)
	}
}

func TestModpackManagerReadFailureBlocksPersistence(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.json")
	projectsPath := filepath.Join(dir, "fa_projects.json")
	if err := os.Mkdir(projectsPath, 0755); err != nil {
		t.Fatal(err)
	}
	mm := &ModpackManager{app: &App{configPath: configPath}}

	err := mm.loadProjects()
	if err == nil || !strings.Contains(err.Error(), "read projects") {
		t.Fatalf("metadata read error not surfaced: %v", err)
	}
	if err := mm.saveProjects(); err == nil {
		t.Fatal("persistence was not blocked after metadata read failure")
	}
	info, err := os.Stat(projectsPath)
	if err != nil || !info.IsDir() {
		t.Fatalf("failed persistence changed corrupt metadata destination: %v", err)
	}
}

func TestModpackConfirmRemoveRollsBackExactListOnSaveFailure(t *testing.T) {
	mm, _ := newTestModpackManager(t)
	first := &Modpack{Name: "first"}
	target := &Modpack{Name: "target"}
	last := &Modpack{Name: "last"}
	original := []*Modpack{first, target, last}
	mm.projects = original
	mm.app.configPath = "" // inject persistence failure

	mm.confirmRemove(target)
	confirm := mm.app.mainWindow.Canvas().Overlays().Top()
	if confirm == nil {
		t.Fatal("remove confirmation was not shown")
	}
	yes := saveDialogButton(t, confirm, "Yes")
	if yes == nil {
		t.Fatal("remove confirmation Yes button not found")
	}
	test.Tap(yes)

	if len(mm.projects) != len(original) {
		t.Fatalf("rollback length = %d, want %d", len(mm.projects), len(original))
	}
	for i := range original {
		if mm.projects[i] != original[i] {
			t.Fatalf("rollback changed project %d: got %p, want %p", i, mm.projects[i], original[i])
		}
	}
}

func TestModpackCreateProjectScaffolding(t *testing.T) {
	dir := t.TempDir()
	projectDir := filepath.Join(dir, "MyMod")

	// Verify the expected directories would be created
	dirs := []string{
		"ext_data/mb2/character",
		"ext_data/sabers",
		"ext_data/vehicles",
		"maps",
		"shaders",
		"models/players",
		"gfx/hud",
	}

	for _, d := range dirs {
		full := filepath.Join(projectDir, d)
		if err := os.MkdirAll(full, 0755); err != nil {
			t.Fatalf("mkdir %s: %v", d, err)
		}
	}

	// Verify all were created
	for _, d := range dirs {
		full := filepath.Join(projectDir, d)
		info, err := os.Stat(full)
		if err != nil {
			t.Errorf("expected %s to exist: %v", d, err)
			continue
		}
		if !info.IsDir() {
			t.Errorf("expected %s to be a directory", d)
		}
	}
}

func TestModpackManifestPreviewNonEmpty(t *testing.T) {
	dir := t.TempDir()

	// Create some files in the project
	charDir := filepath.Join(dir, "ext_data/mb2/character")
	os.MkdirAll(charDir, 0755)
	os.WriteFile(filepath.Join(charDir, "soldier.mbch"), []byte("classinfo\n{\n}\n"), 0644)
	os.WriteFile(filepath.Join(charDir, "jedi.mbch"), []byte("classinfo\n{\n}\n"), 0644)

	manifest, err := BuildExportManifest(dir, "")
	if err != nil {
		t.Fatalf("BuildExportManifest: %v", err)
	}
	if len(manifest) != 2 {
		t.Errorf("expected 2 manifest entries, got %d", len(manifest))
	}

	// Verify archive paths
	for _, entry := range manifest {
		if !filepath.IsAbs(entry.SourcePath) {
			t.Errorf("SourcePath should be absolute: %s", entry.SourcePath)
		}
		if filepath.IsAbs(entry.ArchivePath) {
			t.Errorf("ArchivePath should be relative: %s", entry.ArchivePath)
		}
	}
}

func TestModpackManifestPreviewEmpty(t *testing.T) {
	dir := t.TempDir()
	manifest, err := BuildExportManifest(dir, "")
	if err != nil {
		t.Fatalf("BuildExportManifest: %v", err)
	}
	if len(manifest) != 0 {
		t.Errorf("expected 0 entries for empty dir, got %d", len(manifest))
	}
}

// newTestModpackManager builds a manager wired to a throwaway app and
// config directory, with UI present so list mutations are legal.
func newTestModpackManager(t *testing.T) (*ModpackManager, string) {
	t.Helper()
	app := test.NewApp()
	t.Cleanup(app.Quit)
	win := app.NewWindow("modpacks")
	configDir := t.TempDir()
	mm := NewModpackManager(&App{
		fyneApp:     app,
		mainWindow:  win,
		configPath:  filepath.Join(configDir, "config.json"),
		statusLabel: widget.NewLabel("ready"), // non-empty skips the layout rebuild in updateStatus
	})
	return mm, configDir
}

func TestModpackCreateProjectKeepsScaffoldAndNoEntryWhenPathUnusable(t *testing.T) {
	mm, _ := newTestModpackManager(t)
	dir := t.TempDir()
	blocker := filepath.Join(dir, "blocker")
	if err := os.WriteFile(blocker, []byte("a file"), 0644); err != nil {
		t.Fatal(err)
	}

	// A path through a regular file cannot host the scaffold.
	mm.createProject("Broken", filepath.Join(blocker, "sub", "mod"), "tester")

	if len(mm.projects) != 0 {
		t.Fatalf("failed create left a project entry: %d", len(mm.projects))
	}
	if _, err := os.Stat(filepath.Join(blocker, "sub", "mod", "ext_data")); err == nil {
		t.Fatal("scaffold leaked under an unusable path")
	}
	// Nothing persisted.
	if _, err := os.Stat(filepath.Join(filepath.Dir(mm.app.configPath), "fa_projects.json")); !os.IsNotExist(err) {
		t.Fatalf("failed create wrote a project list: %v", err)
	}
}

func TestModpackCreateProjectRollsBackOwnedDirsAndPreservesExisting(t *testing.T) {
	mm, _ := newTestModpackManager(t)
	dir := t.TempDir()

	// User content: a pre-existing scaffold dir and a directory made
	// unusable for later scaffolding (models is a FILE, so
	// models/players cannot be created).
	userDir := filepath.Join(dir, "maps")
	if err := os.MkdirAll(userDir, 0755); err != nil {
		t.Fatal(err)
	}
	userFile := filepath.Join(userDir, "mymap.bsp")
	if err := os.WriteFile(userFile, []byte("user map"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "models"), []byte("a file"), 0644); err != nil {
		t.Fatal(err)
	}

	mm.createProject("Keeper", dir, "tester")

	if len(mm.projects) != 0 {
		t.Fatalf("failed create left a project entry: %d", len(mm.projects))
	}
	// Pre-existing directory and its user content untouched.
	if got, err := os.ReadFile(userFile); err != nil || string(got) != "user map" {
		t.Fatalf("pre-existing user content damaged: %v %q", err, got)
	}
	// Earlier-created scaffold directories belonged to this failed
	// attempt and are rolled back; the pre-existing maps directory is
	// never a rollback candidate.
	if _, err := os.Stat(filepath.Join(dir, "ext_data", "mb2", "character")); !os.IsNotExist(err) {
		t.Fatalf("owned scaffold should be rolled back, got: %v", err)
	}
	if _, err := os.Stat(userDir); err != nil {
		t.Fatalf("pre-existing dir deleted: %v", err)
	}
}

func TestModpackCreateProjectRollsBackScaffoldOnSaveFailure(t *testing.T) {
	mm, _ := newTestModpackManager(t)
	mm.app.configPath = "" // metadata persistence impossible

	dir := t.TempDir()
	target := filepath.Join(dir, "MyMod")
	mm.createProject("MyMod", target, "tester")

	if len(mm.projects) != 0 {
		t.Fatalf("phantom entry kept after metadata failure: %d", len(mm.projects))
	}
	// Every directory in the scaffold was created by this attempt, so a
	// metadata failure rolls it all back.
	if _, err := os.Stat(target); !os.IsNotExist(err) {
		t.Fatalf("owned scaffold should be removed after metadata failure: %v", err)
	}
}

func TestModpackCreateProjectDuplicateGuard(t *testing.T) {
	mm, configDir := newTestModpackManager(t)
	dir := t.TempDir()

	mm.projects = append(mm.projects, &Modpack{Name: "Existing", Path: dir})
	mm.createProject("Second", dir, "tester")

	if len(mm.projects) != 1 {
		t.Fatalf("duplicate folder tracked twice: %d entries", len(mm.projects))
	}
	_ = configDir
}

func TestModpackImportRevertsOnSaveFailure(t *testing.T) {
	mm, _ := newTestModpackManager(t)
	mm.app.configPath = ""

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "keep.txt"), []byte("content"), 0644); err != nil {
		t.Fatal(err)
	}

	mm.importProjectAt(dir)

	if len(mm.projects) != 0 {
		t.Fatalf("import kept a phantom entry: %d", len(mm.projects))
	}
	if _, err := os.Stat(filepath.Join(dir, "keep.txt")); err != nil {
		t.Fatalf("import touched the folder: %v", err)
	}
}

func TestModpackExportSourceArchiveIsZipWithManifest(t *testing.T) {
	mm, _ := newTestModpackManager(t)
	project := t.TempDir()
	sub := filepath.Join(project, "ext_data", "mb2", "character")
	if err := os.MkdirAll(sub, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sub, "hero.mbch"), []byte("ClassInfo{}"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(project, ".hidden"), []byte("secret"), 0644); err != nil {
		t.Fatal(err)
	}
	p := &Modpack{Name: "Arc", Path: project}

	dest := filepath.Join(t.TempDir(), "arc.zip")
	count, err := mm.exportSourceArchive(p, dest)
	if err != nil {
		t.Fatalf("export: %v", err)
	}

	zr, err := zip.OpenReader(dest)
	if err != nil {
		t.Fatalf("output is not a readable zip: %v", err)
	}
	defer zr.Close()
	if count != 1 || len(zr.File) != 1 {
		t.Fatalf("archived %d (want 1), zip has %d entries", count, len(zr.File))
	}
	if zr.File[0].Name != "ext_data/mb2/character/hero.mbch" {
		t.Fatalf("archive entry: %q", zr.File[0].Name)
	}
	rc, err := zr.File[0].Open()
	if err != nil {
		t.Fatal(err)
	}
	defer rc.Close()
	body, _ := io.ReadAll(rc)
	if string(body) != "ClassInfo{}" {
		t.Fatalf("archive content: %q", body)
	}
}

func TestModpackBuildPK3AtExcludesOutputInsideProject(t *testing.T) {
	mm, _ := newTestModpackManager(t)
	project := t.TempDir()
	if err := os.WriteFile(filepath.Join(project, "map.info"), []byte("info"), 0644); err != nil {
		t.Fatal(err)
	}
	p := &Modpack{Name: "Built", Path: project}

	// Destination INSIDE the project — the self-output exclusion must hold.
	dest := filepath.Join(project, "Built.pk3")
	count, _, err := mm.buildPK3At(p, dest)
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if count != 1 {
		t.Fatalf("count: %d, want only map.info", count)
	}

	zr, err := zip.OpenReader(dest)
	if err != nil {
		t.Fatal(err)
	}
	defer zr.Close()
	for _, f := range zr.File {
		if f.Name == "Built.pk3" {
			t.Fatal("the output archived itself")
		}
	}
}

func TestModpackBuildPK3ChooserJourney(t *testing.T) {
	mm, configDir := newTestModpackManager(t)
	project := t.TempDir()
	if err := os.WriteFile(filepath.Join(project, "readme.txt"), []byte("hi"), 0644); err != nil {
		t.Fatal(err)
	}
	mm.app.config.LastOpenDir = configDir // chooser defaults into the temp workspace
	p := &Modpack{Name: "Jour", Path: project}

	mm.buildPK3Dialog(p)

	chooser := mm.app.mainWindow.Canvas().Overlays().Top()
	if chooser == nil {
		t.Fatal("build must open the non-writing destination chooser")
	}
	saveBtn := saveDialogButton(t, chooser, "Save")
	if saveBtn == nil {
		t.Fatal("chooser Save button not found")
	}
	test.Tap(saveBtn)

	dest := filepath.Join(configDir, "Jour.pk3")
	if _, err := os.Stat(dest); err != nil {
		t.Fatalf("chooser journey did not write the pk3 at the seeded destination: %v", err)
	}
}

func TestModpackBuildPK3AtRefusesEmptyManifest(t *testing.T) {
	mm, _ := newTestModpackManager(t)
	project := t.TempDir() // nothing inside
	p := &Modpack{Name: "Empty", Path: project}

	dest := filepath.Join(t.TempDir(), "out.pk3")
	if _, _, err := mm.buildPK3At(p, dest); err == nil {
		t.Fatal("empty project must refuse the build")
	}
	if _, err := os.Stat(dest); !os.IsNotExist(err) {
		t.Fatalf("empty build wrote a file anyway: %v", err)
	}
}

func TestModpackExportSourceArchiveRefusesEmpty(t *testing.T) {
	mm, _ := newTestModpackManager(t)
	project := t.TempDir()
	p := &Modpack{Name: "EmptyZip", Path: project}

	dest := filepath.Join(t.TempDir(), "out.zip")
	if _, err := mm.exportSourceArchive(p, dest); err == nil {
		t.Fatal("empty project must refuse the export")
	}
	if _, err := os.Stat(dest); !os.IsNotExist(err) {
		t.Fatalf("empty export wrote a file anyway: %v", err)
	}
}

func TestModpackBuildWarningSurvivesSuccessLine(t *testing.T) {
	mm, _ := newTestModpackManager(t)
	project := t.TempDir()
	if err := os.WriteFile(filepath.Join(project, "f.txt"), []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}
	p := &Modpack{Name: "Warned", Path: project}
	dest := filepath.Join(t.TempDir(), "w.pk3")

	// Metadata persistence impossible → build succeeds WITH a warning.
	mm.app.configPath = ""
	count, warning, err := mm.buildPK3At(p, dest)
	if err != nil {
		t.Fatalf("build should succeed: %v", err)
	}
	if count != 1 || warning == "" || !strings.Contains(warning, "project list not updated") {
		t.Fatalf("warning must name the stale metadata: count=%d warning=%q", count, warning)
	}
}

func TestModpackEmptyArchiveGuardPreservesExistingDestination(t *testing.T) {
	mm, _ := newTestModpackManager(t)
	p := &Modpack{Name: "Empty", Path: t.TempDir()}
	dest := filepath.Join(t.TempDir(), "keep.zip")
	if err := os.WriteFile(dest, []byte("existing"), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := mm.exportSourceArchive(p, dest); err == nil {
		t.Fatal("empty export must fail")
	}
	if got, err := os.ReadFile(dest); err != nil || string(got) != "existing" {
		t.Fatalf("empty export damaged existing destination: %q, %v", got, err)
	}
}

func TestModpackShareChooserWritesSourceArchive(t *testing.T) {
	mm, configDir := newTestModpackManager(t)
	project := t.TempDir()
	if err := os.WriteFile(filepath.Join(project, "source.txt"), []byte("source"), 0644); err != nil {
		t.Fatal(err)
	}
	mm.app.config.LastOpenDir = configDir
	p := &Modpack{Name: "Shared", Path: project}

	mm.shareProject(p)
	chooser := mm.app.mainWindow.Canvas().Overlays().Top()
	if chooser == nil {
		t.Fatal("share must open the non-writing destination chooser")
	}
	saveBtn := saveDialogButton(t, chooser, "Save")
	if saveBtn == nil {
		t.Fatal("share chooser Save button not found")
	}
	test.Tap(saveBtn)

	dest := filepath.Join(configDir, "Shared.zip")
	zr, err := zip.OpenReader(dest)
	if err != nil {
		t.Fatalf("share did not produce a real zip: %v", err)
	}
	defer zr.Close()
	if len(zr.File) != 1 || zr.File[0].Name != "source.txt" {
		t.Fatalf("shared archive contents: %#v", zr.File)
	}
}
