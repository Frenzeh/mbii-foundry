package main

import (
	"archive/zip"
	"bytes"
	"image/color"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	fyneTest "fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

type auditAssetFile struct {
	path string
	body string
}

func writeAuditAsset(t *testing.T, root, path, body string) string {
	t.Helper()
	full := filepath.Join(root, filepath.FromSlash(path))
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return full
}

func writeAuditPK3(t *testing.T, path string, files ...auditAssetFile) {
	t.Helper()
	var packed bytes.Buffer
	writer := zip.NewWriter(&packed)
	for _, file := range files {
		entry, err := writer.Create(file.path)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := io.WriteString(entry, file.body); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, packed.Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}
}

func auditReadAsset(t *testing.T, vfs *VirtualFileSystem, path string) string {
	t.Helper()
	reader, err := vfs.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	body, readErr := io.ReadAll(reader)
	closeErr := reader.Close()
	if readErr != nil || closeErr != nil {
		t.Fatalf("read %q: read=%v close=%v", path, readErr, closeErr)
	}
	return string(body)
}

func auditAssetBrowser(t *testing.T, gamedata, loose string) *AssetBrowser {
	t.Helper()
	vfs := NewVirtualFileSystem(gamedata, loose)
	if err := vfs.Refresh(); err != nil {
		t.Fatal(err)
	}
	browser := &AssetBrowser{
		assets: make(map[string]*AssetEntry), viewMode: ViewModeGrid,
		sortMode: SortNameAsc, iconSize: 100, vfs: vfs,
		gamedataPath: gamedata, textAssetsPath: loose,
		shaderResolver: NewShaderResolver(vfs),
	}
	browser.shaderResolver.Prebuild()
	if gamedata != "" {
		browser.pk3Files = vfs.findPK3s(gamedata)
	}
	browser.createUI()
	return browser
}

func auditAssetItem(t *testing.T, browser *AssetBrowser, path string) *GridItem {
	t.Helper()
	for _, object := range browser.grid.Objects {
		if item, ok := object.(*GridItem); ok && item.Entry != nil && item.Entry.Path == path {
			return item
		}
	}
	t.Fatalf("asset %q is not in the current grid", path)
	return nil
}

func auditAssetFind(root fyne.CanvasObject, matches func(fyne.CanvasObject) bool) fyne.CanvasObject {
	seen := make(map[fyne.CanvasObject]bool)
	var visit func(fyne.CanvasObject) fyne.CanvasObject
	visit = func(object fyne.CanvasObject) fyne.CanvasObject {
		if object == nil || seen[object] {
			return nil
		}
		seen[object] = true
		if matches(object) {
			return object
		}
		var children []fyne.CanvasObject
		switch value := object.(type) {
		case *fyne.Container:
			children = value.Objects
		case fyne.Widget:
			children = fyneTest.WidgetRenderer(value).Objects()
		}
		for _, child := range children {
			if found := visit(child); found != nil {
				return found
			}
		}
		return nil
	}
	return visit(root)
}

func auditPickerRefresh(t *testing.T, picker *CustomFilePicker) {
	t.Helper()
	object := auditAssetFind(picker.window.Content(), func(object fyne.CanvasObject) bool {
		button, ok := object.(*widget.Button)
		return ok && button.Text == "" && button.Icon != nil && button.Icon.Name() == theme.ViewRefreshIcon().Name()
	})
	if object == nil {
		t.Fatal("picker Refresh button not found")
	}
	object.(*widget.Button).Tapped(nil)
}

func auditPickerSource(t *testing.T, picker *CustomFilePicker, label string) {
	t.Helper()
	for id, source := range picker.sources {
		if source == label {
			picker.sidebar.OnSelected(id)
			return
		}
	}
	t.Fatalf("picker source %q not found", label)
}

func auditPickerBreadcrumb(t *testing.T, picker *CustomFilePicker, label string) {
	t.Helper()
	for _, object := range picker.pathBar.Objects {
		if button, ok := object.(*widget.Button); ok && button.Text == label {
			button.Tapped(nil)
			return
		}
	}
	t.Fatalf("breadcrumb %q not found", label)
}

func auditPickerUnselected(t *testing.T, picker *CustomFilePicker) {
	t.Helper()
	if picker.selectedFile != nil || !picker.selectButton.Disabled() || picker.browser.GetSelectedAsset() != nil {
		t.Fatalf("stale picker selection: file=%#v disabled=%v browser=%#v", picker.selectedFile, picker.selectButton.Disabled(), picker.browser.GetSelectedAsset())
	}
}

func TestAuditVFSPK3OrderingMatchesExtractor(t *testing.T) {
	root := t.TempDir()
	logical := "gfx/audit_order.txt"
	for _, name := range []string{"a.pk3", "Z.pk3", "_override.pk3"} {
		writeAuditPK3(t, filepath.Join(root, "MBII", name), auditAssetFile{logical, name})
	}
	vfs := NewVirtualFileSystem(root, "")
	if err := vfs.Refresh(); err != nil {
		t.Fatal(err)
	}
	winner := filepath.Join(root, "MBII", "_override.pk3")
	if source := vfs.Lookup(logical); source == nil || source.PK3Path != winner {
		t.Fatalf("indexed winner = %#v, want %s", source, winner)
	}
	if got := auditReadAsset(t, vfs, logical); got != "_override.pk3" {
		t.Fatalf("read winner = %q", got)
	}
}

func TestAuditVFSReadsIndexedLastDuplicateEntry(t *testing.T) {
	root := t.TempDir()
	archive := filepath.Join(root, "MBII", "duplicate.pk3")
	logical := "models/players/audit_duplicate/readme.txt"
	writeAuditPK3(t, archive,
		auditAssetFile{"Models/Players/Audit_Duplicate/Readme.txt", "first case variant"},
		auditAssetFile{logical, "earlier exact name"},
		auditAssetFile{logical, "last indexed content"},
	)
	vfs := NewVirtualFileSystem(root, "")
	if err := vfs.Refresh(); err != nil {
		t.Fatal(err)
	}
	if source := vfs.Lookup(logical); source == nil || source.EntryName != logical || source.Size != int64(len("last indexed content")) {
		t.Fatalf("indexed duplicate = %#v", source)
	}
	if got := auditReadAsset(t, vfs, "MODELS\\PLAYERS\\AUDIT_DUPLICATE\\README.TXT"); got != "last indexed content" {
		t.Fatalf("ReadFile disagreed with the winning index entry: %q", got)
	}
}

func TestAuditLooseVFSSelectionOpensPhysicalDocument(t *testing.T) {
	root := t.TempDir()
	loose := filepath.Join(root, "loose")
	logical := "ext_data/sabers/audit_loose.sab"
	physical := writeAuditAsset(t, loose, logical, multiSabFixture)
	app := newRecoveryTestApp(t, filepath.Join(root, "recovery"), filepath.Join(root, "config"))
	browser := auditAssetBrowser(t, "", loose)
	app.assetBrowser = browser
	browser.navigateToPath(logical)
	selected := browser.GetSelectedAsset()
	if selected == nil || selected.Path != logical || selected.PK3Source != "" {
		t.Fatalf("loose logical selection = %#v", selected)
	}
	app.openFileFromAsset(selected)
	editor := app.editors[app.docTabs.Selected()]
	if editor == nil {
		t.Fatal("opening a loose VFS selection did not create an editor")
	}
	if got := editor.GetCurrentPath(); got != physical {
		t.Fatalf("opened document path = %q, want physical path %q", got, physical)
	}
}

func TestAuditPK3SelectionOpensLastDuplicate(t *testing.T) {
	root := t.TempDir()
	archive := filepath.Join(root, "game", "MBII", "duplicate.pk3")
	logical := "ext_data/sabers/audit_packed.sab"
	first := strings.ReplaceAll(multiSabFixture, "saber_a", "first_duplicate")
	last := strings.ReplaceAll(multiSabFixture, "saber_a", "last_duplicate")
	writeAuditPK3(t, archive, auditAssetFile{logical, first}, auditAssetFile{logical, last})
	app := newRecoveryTestApp(t, filepath.Join(root, "recovery"), filepath.Join(root, "config"))
	browser := auditAssetBrowser(t, filepath.Join(root, "game"), "")
	app.assetBrowser = browser
	browser.navigateToPath(logical)
	app.openFileFromAsset(browser.GetSelectedAsset())
	editor := app.editors[app.docTabs.Selected()]
	if editor == nil {
		t.Fatal("PK3 selection did not open")
	}
	provider, ok := editor.(SourceProvider)
	if !ok || !strings.Contains(provider.GenerateSource(), "last_duplicate") || strings.Contains(provider.GenerateSource(), "first_duplicate") {
		t.Fatal("PK3 editor loaded a different duplicate than the indexed winner")
	}
	if editor.GetCurrentPath() != "" {
		t.Fatal("PK3-backed editor did not require Save As")
	}
}

func auditShaderDeclaration(name, texture string) string {
	return name + "\n{\n\t{\n\t\tmap " + texture + "\n\t}\n}\n"
}

func TestAuditDeclaredShaderPrecedesCoincidentTexture(t *testing.T) {
	useEditorTestApp(t)
	root := t.TempDir()
	name := "models/players/audit_declared/portrait"
	missing := "models/players/audit_declared/missing"
	mapped := "textures/audit_declared/actual"
	red := color.NRGBA{R: 255, A: 255}
	blue := color.NRGBA{B: 255, A: 255}
	writePortraitFixture(t, root, name+".png", red)
	writePortraitFixture(t, root, missing+".png", red)
	writePortraitFixture(t, root, mapped+".png", blue)
	writeAuditAsset(t, root, "scripts/audit_declared.shader",
		auditShaderDeclaration(name, mapped)+auditShaderDeclaration(missing, "textures/audit_declared/absent.png"))
	browser := auditAssetBrowser(t, "", root)
	assertPortraitPixel(t, browser.LoadIconResource(name), blue)
	if browser.LoadIconResource(missing) != nil {
		t.Fatal("missing declared shader texture fell back to an unrelated coincident texture")
	}
}

func TestAuditDuplicateShaderPreviewPolicy(t *testing.T) {
	root := t.TempDir()
	gamedata, loose := filepath.Join(root, "game"), filepath.Join(root, "loose")
	prefix := "models/players/audit_shader_policy/"
	writeAuditPK3(t, filepath.Join(gamedata, "MBII", "a.pk3"), auditAssetFile{
		"scripts/z_low.shader", auditShaderDeclaration(prefix+"source", "textures/low") + auditShaderDeclaration(prefix+"loose", "textures/low"),
	})
	high := filepath.Join(gamedata, "MBII", "Z.pk3")
	writeAuditPK3(t, high, auditAssetFile{
		"scripts/a_high.shader", auditShaderDeclaration(prefix+"source", "textures/high") + auditShaderDeclaration(prefix+"loose", "textures/high"),
	})
	writeAuditAsset(t, loose, "scripts/0_loose.shader",
		auditShaderDeclaration(prefix+"loose", "textures/loose")+auditShaderDeclaration(prefix+"file", "textures/earlier_file"))
	writeAuditAsset(t, loose, "scripts/z_loose.shader",
		auditShaderDeclaration(prefix+"file", "textures/later_file")+auditShaderDeclaration(prefix+"within", "textures/earlier_definition")+auditShaderDeclaration(prefix+"within", "textures/later_definition"))
	vfs := NewVirtualFileSystem(gamedata, loose)
	if err := vfs.Refresh(); err != nil {
		t.Fatal(err)
	}
	if source := vfs.Lookup("scripts/a_high.shader"); source == nil || source.PK3Path != high {
		t.Fatalf("shader-file provenance = %#v, want %s", source, high)
	}
	resolver := NewShaderResolver(vfs)
	for rebuild := range 8 {
		resolver.Reset()
		resolver.Prebuild()
		for name, want := range map[string]string{
			"source": "textures/high", "loose": "textures/loose",
			"file": "textures/later_file", "within": "textures/later_definition",
		} {
			if got := resolver.Resolve(strings.ToUpper(prefix + name)); got != want {
				t.Fatalf("rebuild %d, %s = %q, want %q", rebuild, name, got, want)
			}
		}
	}
}

func TestAuditPickerFilesystemNavigationClearsSelection(t *testing.T) {
	useEditorTestApp(t)
	root := t.TempDir()
	left := writeAuditAsset(t, root, "left/one.txt", "left")
	right := writeAuditAsset(t, root, "right/two.txt", "right")
	browser := auditAssetBrowser(t, "", "")
	window := fyne.CurrentApp().NewWindow("picker")
	defer window.Close()
	picker := NewCustomFilePicker(window, browser)
	browser.loadFS(filepath.Dir(left))
	auditAssetItem(t, browser, left).Tapped(nil)
	if picker.selectedFile == nil || picker.selectButton.Disabled() {
		t.Fatal("file tap did not enable Open")
	}
	browser.loadFS(root)
	auditPickerUnselected(t, picker)
	browser.tree.OnSelected(filepath.Dir(right))
	auditPickerUnselected(t, picker)
	auditAssetItem(t, browser, right).Tapped(nil)
	picker.goUp()
	auditPickerUnselected(t, picker)
	if browser.currentDir == nil || browser.currentDir.Path != root {
		t.Fatal("filesystem Up did not retain the filesystem backend")
	}
	browser.tree.OnSelected(filepath.Dir(left))
	auditAssetItem(t, browser, left).Tapped(nil)
	added := writeAuditAsset(t, root, "left/added.txt", "new")
	auditPickerRefresh(t, picker)
	auditPickerUnselected(t, picker)
	auditAssetItem(t, browser, added)
	auditPickerBreadcrumb(t, picker, filepath.Base(root))
	if browser.currentDir == nil || browser.currentDir.Path != root || browser.currentPK3 != "" {
		t.Fatal("filesystem breadcrumb changed backend or failed to navigate")
	}
}

func TestAuditPickerPK3NavigationUsesArchiveBackend(t *testing.T) {
	useEditorTestApp(t)
	root := t.TempDir()
	archive := filepath.Join(root, "MBII", "audit_navigation.pk3")
	file := auditAssetFile{"models/players/demo/one.txt", "one"}
	writeAuditPK3(t, archive, file, auditAssetFile{"root.txt", "root"})
	browser := auditAssetBrowser(t, root, "")
	window := fyne.CurrentApp().NewWindow("picker")
	defer window.Close()
	picker := NewCustomFilePicker(window, browser)
	auditPickerSource(t, picker, filepath.Base(archive))
	if browser.currentDir == nil || browser.currentDir.Path != "" || browser.currentPK3 != archive {
		t.Fatal("choosing a PK3 did not display its root")
	}
	auditAssetItem(t, browser, "root.txt")
	auditAssetItem(t, browser, "models").DoubleTapped(nil)
	auditAssetItem(t, browser, "models/players").Tapped(nil)
	picker.selectButton.Tapped(nil)
	auditPickerUnselected(t, picker)
	if browser.currentDir.Path != "models/players" || browser.currentPK3 != archive {
		t.Fatal("Open directory did not navigate inside the PK3")
	}
	picker.goUp()
	picker.goUp()
	if browser.currentDir.Path != "" || browser.currentPK3 != archive {
		t.Fatal("PK3 Up escaped into filesystem loading")
	}
	browser.tree.OnSelected("models/players/demo")
	auditAssetItem(t, browser, file.path).Tapped(nil)
	writeAuditPK3(t, archive, file, auditAssetFile{"models/players/demo/added.txt", "added"})
	auditPickerRefresh(t, picker)
	auditPickerUnselected(t, picker)
	if browser.currentDir == nil || browser.currentDir.Path != "models/players/demo" || browser.currentPK3 != archive {
		t.Fatal("PK3 Refresh did not preserve the archive directory")
	}
	auditAssetItem(t, browser, "models/players/demo/added.txt")
	auditPickerBreadcrumb(t, picker, "models")
	if browser.currentDir.Path != "models" || browser.currentPK3 != archive {
		t.Fatal("PK3 breadcrumb routed through the filesystem")
	}
	auditPickerBreadcrumb(t, picker, filepath.Base(archive))
	if browser.currentDir.Path != "" || browser.currentPK3 != archive {
		t.Fatal("PK3 root breadcrumb did not navigate to archive root")
	}
}

func TestAuditPickerVFSNavigationAndRefresh(t *testing.T) {
	useEditorTestApp(t)
	root := t.TempDir()
	logical := "models/players/demo/one.txt"
	writeAuditAsset(t, root, logical, "one")
	browser := auditAssetBrowser(t, "", root)
	window := fyne.CurrentApp().NewWindow("picker")
	defer window.Close()
	picker := NewCustomFilePicker(window, browser)
	auditPickerSource(t, picker, "All Game Data (Virtual)")
	auditAssetItem(t, browser, "models").Tapped(nil)
	picker.selectButton.Tapped(nil)
	if browser.currentDir == nil || browser.currentDir.Path != "models" || browser.currentPK3 != "VFS" {
		t.Fatal("Open did not enter the VFS directory")
	}
	browser.tree.OnSelected("models/players/demo")
	auditAssetItem(t, browser, logical).Tapped(nil)
	picker.goUp()
	auditPickerUnselected(t, picker)
	if browser.currentDir.Path != "models/players" || browser.currentPK3 != "VFS" {
		t.Fatal("VFS Up routed through the filesystem")
	}
	auditPickerBreadcrumb(t, picker, "models")
	writeAuditAsset(t, root, "models/added.txt", "new")
	ready := make(chan struct{})
	browser.onVFSReady = func() { close(ready) }
	auditPickerRefresh(t, picker)
	select {
	case <-ready:
	case <-time.After(5 * time.Second):
		t.Fatal("VFS Refresh did not publish a refreshed index")
	}
	if browser.currentDir == nil || browser.currentDir.Path != "models" || browser.currentPK3 != "VFS" {
		t.Fatal("VFS Refresh did not preserve the virtual directory")
	}
	auditAssetItem(t, browser, "models/added.txt")
	auditPickerUnselected(t, picker)
	auditPickerBreadcrumb(t, picker, "All Game Data")
	if browser.currentDir.Path != "" || browser.currentPK3 != "VFS" {
		t.Fatal("VFS breadcrumb did not return to the virtual root")
	}
}

func TestAuditGridItemKeyboardSelectionActivationAndFocus(t *testing.T) {
	useEditorTestApp(t)
	selected, opened := 0, 0
	item := NewGridItem("audit asset", theme.FileIcon(), func() { selected++ })
	item.OnDoubleTapped = func() { opened++ }
	fyneTest.WidgetRenderer(item)
	focusable, ok := any(item).(fyne.Focusable)
	if !ok {
		t.Fatal("asset grid item cannot receive keyboard focus")
	}
	focusable.TypedKey(&fyne.KeyEvent{Name: fyne.KeyEscape})
	focusable.TypedRune('x')
	if selected != 0 || opened != 0 {
		t.Fatal("unrelated keyboard input activated the asset")
	}
	focusable.TypedRune(' ')
	if selected != 1 || opened != 0 {
		t.Fatal("Space should select without opening")
	}
	for _, key := range []fyne.KeyName{fyne.KeyReturn, fyne.KeyEnter} {
		focusable.TypedKey(&fyne.KeyEvent{Name: key})
	}
	if selected != 3 || opened != 2 {
		t.Fatalf("keyboard selection/activation = %d/%d, want 3/2", selected, opened)
	}
	for _, selectedState := range []bool{false, true} {
		item.SetSelected(selectedState)
		fill := item.background.FillColor
		focusable.FocusGained()
		_, _, _, alpha := item.background.StrokeColor.RGBA()
		if item.background.StrokeWidth <= 0 || alpha == 0 || item.background.FillColor != fill {
			t.Fatal("focus is not visible independently of selection")
		}
		focusable.FocusLost()
		if item.background.StrokeWidth != 0 || item.background.FillColor != fill {
			t.Fatal("blur changed selection styling")
		}
	}
}

func TestAuditIconInventoryPublishesAllTiles(t *testing.T) {
	useEditorTestApp(t)
	app := &App{fyneApp: fyne.CurrentApp()}
	app.showIconInventory()
	fyne.DoAndWait(func() {})
	var window fyne.Window
	for _, candidate := range app.fyneApp.Driver().AllWindows() {
		if candidate.Title() == "Icon Inventory" {
			window = candidate
		}
	}
	if window == nil {
		t.Fatal("inventory window not opened")
	}
	defer window.Close()
	object := auditAssetFind(window.Content(), func(object fyne.CanvasObject) bool {
		_, ok := object.(*container.AppTabs)
		return ok
	})
	if object == nil {
		t.Fatal("inventory tabs not published")
	}
	for _, tab := range object.(*container.AppTabs).Items {
		dir := strings.Split(tab.Text, " (")[0]
		var count int
		if dir == "boxicons" {
			entries, err := embedBoxicons.ReadDir("assets/boxicons")
			if err != nil {
				t.Fatal(err)
			}
			for _, entry := range entries {
				if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".svg") {
					count++
				}
			}
		} else {
			entries, err := embedIcons.ReadDir("assets/icons/" + dir)
			if err != nil {
				t.Fatal(err)
			}
			for _, entry := range entries {
				if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".png") {
					count++
				}
			}
		}
		scroll := auditAssetFind(tab.Content, func(object fyne.CanvasObject) bool {
			_, ok := object.(*container.Scroll)
			return ok
		})
		if scroll == nil {
			t.Fatalf("%s inventory grid not published", dir)
		}
		grid := scroll.(*container.Scroll).Content.(*fyne.Container)
		if got := len(grid.Objects); got != count {
			t.Fatalf("%s published %d tiles, want %d embedded files", dir, got, count)
		}
	}
}
