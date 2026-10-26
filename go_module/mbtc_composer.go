package main

import (
	"fmt"
	"io"
	"path/filepath"
	"strconv"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/Frenzeh/mbii-foundry/parsers"
)

// MBTCComposerDialog provides an interactive editor for .mbtc team files.
//
// One .mbtc file is ONE team (BG_SiegeParseTeamFile, bg_saga.c:3253):
// name (required), TimePeriod, EUAllowed, FriendlyShader, a Classes
// group with class1..class6, and per-class SubclassesForClassN groups.
// The old rebel/imperial two-team model with "// Imperial" comment
// detection modeled nothing the engine reads and is gone. `ClassesAllowed`
// is legacy (bg_saga.h:476) — preserved, clearly labeled, not advertised
// as live.
type MBTCComposerDialog struct {
	app      *App
	roster   parsers.MBTCTeam
	filePath string // current file, empty for unsaved
	lastDest string // last successfully chosen destination (seeds the chooser)

	baselinePath string
	baseline     saveDestinationSnapshot
	baselineSet  bool

	dialogWin   fyne.Window
	statusLabel *widget.Label

	nameEntry      *widget.Entry
	timeEntry      *widget.Entry
	euCheck        *widget.Check
	shaderEntry    *widget.Entry
	classesAllowed *widget.Entry
	classSlots     [parsers.MBTCMaxClasses]*widget.Entry
	subClassSelect *widget.Select
	subEntry       *widget.Entry
	subClassIdx    int // 0-based class the subclass editor shows

	// syncingUI suppresses widget callbacks while the model is being
	// pushed INTO the UI (render, programmatic selection) so programmatic
	// changes are never stored back over freshly loaded data.
	syncingUI bool
	euPresent bool // preserves an authored EUAllowed 0 distinct from absence

	// Class-reference resolution over the VFS (PK3-aware), built lazily
	// and cached for the dialog's lifetime.
	refIndex     map[string]bool // canonical class names/stems found
	refBuilt     bool
	refAvailable bool
	refErr       error

	// Tests inject a filesystem change after review/baseline capture but
	// before the guarded final publication check.
	beforePublication func(string)
	// Tests inject a failure after candidate publication but before final proof.
	afterCandidatePublish func(path, holdPath, backupPath string)
}

func OpenMBTCComposer(app *App) {
	composer := &MBTCComposerDialog{
		app: app,
	}
	composer.show()
}

func (c *MBTCComposerDialog) show() {
	c.dialogWin = c.app.fyneApp.NewWindow("Team Composer (.mbtc)")
	c.dialogWin.Resize(fyne.NewSize(760, 720))
	c.subClassIdx = 0

	c.nameEntry = NewInputEntry()
	c.nameEntry.SetPlaceHolder("Team name written to the `name` key (required)")

	c.timeEntry = NewInputEntry()
	c.timeEntry.SetPlaceHolder("e.g. 5 (TimePeriod, optional)")

	c.euCheck = widget.NewCheck("EUAllowed (1 = EU content allowed)", func(bool) {
		if !c.syncingUI {
			c.euPresent = true
		}
	})

	c.shaderEntry = NewInputEntry()
	c.shaderEntry.SetPlaceHolder("shader path (FriendlyShader, optional)")

	c.classesAllowed = NewInputEntry()
	c.classesAllowed.SetPlaceHolder("legacy mask — unread by the current engine, preserved")

	teamBox := container.NewVBox(
		widget.NewForm(
			widget.NewFormItem("Name", c.nameEntry),
			widget.NewFormItem("TimePeriod", c.timeEntry),
			widget.NewFormItem("FriendlyShader", c.shaderEntry),
			widget.NewFormItem("ClassesAllowed (legacy)", c.classesAllowed),
		),
		c.euCheck,
	)

	// Classes: class1..class6, exactly the slots the engine reads
	// (contiguous — it stops at the first empty slot).
	var classRows []fyne.CanvasObject
	for i := 0; i < parsers.MBTCMaxClasses; i++ {
		c.classSlots[i] = NewInputEntry()
		c.classSlots[i].SetPlaceHolder(fmt.Sprintf("class%d (e.g. mb2_rebel_soldier)", i+1))
		label := widget.NewLabel(fmt.Sprintf("class%d", i+1))
		label.TextStyle.Monospace = true
		classRows = append(classRows, container.NewBorder(nil, nil, label, nil, c.classSlots[i]))
	}
	classesCard := widget.NewCard("Classes", "Slots 1..6 — the engine stops at the first empty slot", container.NewVBox(classRows...))

	// Subclasses: per-class list, one reference per line. subEntry is
	// constructed BEFORE the select widget and the selection callback is
	// wired to actually switch slots — every choice edits its own class.
	c.subEntry = widget.NewMultiLineEntry()
	c.subEntry.SetPlaceHolder("One subclass reference per line (Subclass1, Subclass2, …; max 41)")

	c.subClassSelect = widget.NewSelect([]string{"class1", "class2", "class3", "class4", "class5", "class6"}, func(s string) {
		if c.syncingUI {
			return // programmatic selection during render/open — not a user switch
		}
		c.storeSubEditor() // persist the slot being left
		idx := subSlotIndex(s)
		if idx < 0 {
			return
		}
		c.subClassIdx = idx
		c.loadSubEditor()
	})

	subCard := widget.NewCard("Subclasses", "Per-class alternates (SubclassesForClassN)", container.NewBorder(nil, nil, nil, nil,
		container.NewVBox(c.subClassSelect, c.subEntry)))

	openBtn := widget.NewButtonWithIcon("Open .mbtc", theme.FolderOpenIcon(), c.openFile)
	saveBtn := widget.NewButtonWithIcon("Save .mbtc", theme.DocumentSaveIcon(), c.saveFile)
	saveBtn.Importance = widget.HighImportance
	resetBtn := widget.NewButtonWithIcon("Reset", theme.ContentClearIcon(), c.resetForm)
	resetBtn.Importance = widget.LowImportance

	c.statusLabel = widget.NewLabel("")
	c.statusLabel.Wrapping = fyne.TextWrapWord

	content := container.NewVScroll(container.NewVBox(
		widget.NewCard("Team", "One .mbtc file = one siege team", teamBox),
		classesCard,
		subCard,
		container.NewHBox(openBtn, saveBtn, resetBtn),
		c.statusLabel,
	))

	c.dialogWin.SetContent(content)

	// Initial selection is programmatic: guard it so it neither stores
	// into the (empty) roster nor double-loads.
	c.syncingUI = true
	c.subClassSelect.SetSelected("class1")
	c.syncingUI = false
	c.loadSubEditor()

	c.dialogWin.Show()
}

// subSlotIndex maps "classN" to the 0-based roster slot.
func subSlotIndex(s string) int {
	n, err := strconv.Atoi(strings.TrimPrefix(strings.ToLower(s), "class"))
	if err != nil || n < 1 || n > parsers.MBTCMaxClasses {
		return -1
	}
	return n - 1
}

// loadSubEditor pushes roster.Subclasses[subClassIdx] into the editor.
func (c *MBTCComposerDialog) loadSubEditor() {
	if c.subEntry == nil || c.subClassIdx < 0 || c.subClassIdx >= parsers.MBTCMaxClasses {
		return
	}
	wasSyncing := c.syncingUI
	c.syncingUI = true
	c.subEntry.SetText(strings.Join(c.roster.Subclasses[c.subClassIdx], "\n"))
	c.syncingUI = wasSyncing
}

// storeSubEditor collects the multiline editor into the roster. Blank
// lines are dropped; the engine stores an ordered list, so contiguity is
// the serialization contract.
func (c *MBTCComposerDialog) storeSubEditor() {
	if c.syncingUI || c.subEntry == nil || c.subClassIdx < 0 || c.subClassIdx >= parsers.MBTCMaxClasses {
		return
	}
	var subs []string
	for _, line := range strings.Split(c.subEntry.Text, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			subs = append(subs, line)
		}
	}
	c.roster.Subclasses[c.subClassIdx] = subs
}

// collectUI reads every widget into c.roster (excluding the subclass
// editor, which is stored on switch — storeSubEditor is called here too).
func (c *MBTCComposerDialog) collectUI() error {
	c.roster.Name = strings.TrimSpace(c.nameEntry.Text)
	c.roster.FriendlyShader = strings.TrimSpace(c.shaderEntry.Text)

	if v := strings.TrimSpace(c.timeEntry.Text); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			return fmt.Errorf("TimePeriod must be an integer, got %q", v)
		}
		c.roster.TimePeriod, c.roster.TimePeriodSet = n, true
	} else {
		c.roster.TimePeriod, c.roster.TimePeriodSet = 0, false
	}

	if c.euCheck.Checked {
		c.roster.EUAllowed, c.roster.EUAllowedSet = 1, true
	} else if c.euPresent {
		c.roster.EUAllowed, c.roster.EUAllowedSet = 0, true
	} else {
		c.roster.EUAllowed, c.roster.EUAllowedSet = 0, false
	}

	if v := strings.TrimSpace(c.classesAllowed.Text); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			return fmt.Errorf("ClassesAllowed must be an integer, got %q", v)
		}
		c.roster.ClassesAllowed, c.roster.ClassesAllowedSet = n, true
	} else {
		c.roster.ClassesAllowed, c.roster.ClassesAllowedSet = 0, false
	}

	for i := range parsers.MBTCMaxClasses {
		if c.classSlots[i] == nil {
			return fmt.Errorf("class%d editor is unavailable", i+1)
		}
		c.roster.Classes[i] = strings.TrimSpace(c.classSlots[i].Text)
	}
	c.storeSubEditor()
	return nil
}

// render pushes the roster into the widgets after load/reset. All
// programmatic writes run under syncingUI so entry callbacks and the
// select callback never store staged UI text back over the fresh model.
func (c *MBTCComposerDialog) render() {
	if c.nameEntry == nil || c.timeEntry == nil || c.euCheck == nil ||
		c.shaderEntry == nil || c.classesAllowed == nil ||
		c.subClassSelect == nil || c.subEntry == nil {
		return
	}
	for _, slot := range c.classSlots {
		if slot == nil {
			return
		}
	}
	wasSyncing := c.syncingUI
	c.syncingUI = true
	defer func() { c.syncingUI = wasSyncing }()
	c.nameEntry.SetText(c.roster.Name)
	if c.roster.TimePeriodSet {
		c.timeEntry.SetText(strconv.Itoa(c.roster.TimePeriod))
	} else {
		c.timeEntry.SetText("")
	}
	c.euPresent = c.roster.EUAllowedSet
	c.euCheck.SetChecked(c.roster.EUAllowedSet && c.roster.EUAllowed != 0)
	c.shaderEntry.SetText(c.roster.FriendlyShader)
	if c.roster.ClassesAllowedSet {
		c.classesAllowed.SetText(strconv.Itoa(c.roster.ClassesAllowed))
	} else {
		c.classesAllowed.SetText("")
	}
	for i := range parsers.MBTCMaxClasses {
		c.classSlots[i].SetText(c.roster.Classes[i])
	}
	c.subClassIdx = 0
	c.subClassSelect.SetSelected("class1")
	c.loadSubEditor()
}

// resetForm returns to the empty state.
func (c *MBTCComposerDialog) resetForm() {
	c.roster = parsers.MBTCTeam{}
	c.filePath = ""
	c.baselinePath = ""
	c.baseline = saveDestinationSnapshot{}
	c.baselineSet = false
	c.render()
	c.statusLabel.SetText("Form reset to empty state.")
}

func (c *MBTCComposerDialog) openFile() {
	dialog.ShowFileOpen(func(reader fyne.URIReadCloser, err error) {
		if err != nil || reader == nil {
			return
		}
		defer reader.Close()

		data, readErr := io.ReadAll(reader)
		if readErr != nil {
			dialog.ShowError(readErr, c.dialogWin)
			return
		}
		path := reader.URI().Path()
		baseline, baselineErr := snapshotSaveDestination(path)
		if baselineErr != nil {
			dialog.ShowError(baselineErr, c.dialogWin)
			return
		}
		if !baseline.Exists || baseline.Bytes != string(data) {
			dialog.ShowError(fmt.Errorf("file changed while it was being opened; open it again"), c.dialogWin)
			return
		}

		c.filePath = path
		c.lastDest = path
		c.baselinePath = path
		c.baseline = baseline
		c.baselineSet = true
		c.parseMBTC(string(data))
	}, c.dialogWin)
}

// parseMBTC parses .mbtc content. The canonical parser returns a fresh
// model (and retains the source AST for non-destructive serialization),
// so parsing itself resets all prior roster state.
func (c *MBTCComposerDialog) parseMBTC(content string) {
	c.roster = *parsers.ParseMBTC(content)
	c.render()
	if diags := c.roster.Diagnostics; len(diags) > 0 {
		c.statusLabel.SetText("Loaded with warnings: " + strings.Join(diags, "; "))
		return
	}
	c.statusLabel.SetText(fmt.Sprintf("Loaded %s", filepath.Base(c.filePath)))
}

// validate checks the CURRENT candidate (widgets as they stand) against
// the engine rules, then reports class-reference resolution as
// non-blocking warnings. It never consults stale parse diagnostics:
// loading an invalid file blocks that file's original state, but the
// moment the user corrects the fields the candidate is judged on its own.
func (c *MBTCComposerDialog) validate() (warnings []string, err error) {
	if err := c.collectUI(); err != nil {
		return nil, err
	}
	if issues := c.roster.Validate(); len(issues) > 0 {
		return nil, fmt.Errorf("%s", strings.Join(issues, "; "))
	}

	var refs []string
	for _, cls := range c.roster.Classes {
		if cls != "" {
			refs = append(refs, cls)
		}
	}
	for _, subs := range c.roster.Subclasses {
		refs = append(refs, subs...)
	}
	if len(refs) == 0 {
		return nil, nil
	}

	names, rErr := c.classRefIndex()
	if rErr != nil {
		// Resolution unavailable — an explicit unknown, never a false
		// verified or false missing.
		return []string{fmt.Sprintf("class references not checked: %v", rErr)}, nil
	}
	var missing []string
	for _, ref := range refs {
		if !names[strings.ToLower(ref)] {
			missing = append(missing, ref)
		}
	}
	if len(missing) > 0 {
		return []string{fmt.Sprintf("%d class reference(s) not found in the scanned project/VFS content: %s",
			len(missing), strings.Join(missing, ", "))}, nil
	}
	return nil, nil
}

// classRefIndex lazily builds (and caches) the set of resolvable class
// references: every .mbch the VFS indexes under the configured roots —
// loose files AND PK3 contents — matched by file stem and by the
// canonical parsed `name` field. No line parsing, no file-count cap.
func (c *MBTCComposerDialog) classRefIndex() (map[string]bool, error) {
	if c.refBuilt {
		if c.refErr != nil {
			return nil, c.refErr
		}
		return c.refIndex, nil
	}
	c.refBuilt = true

	if c.app == nil {
		c.refErr = fmt.Errorf("application VFS unavailable")
		return nil, c.refErr
	}

	var vfs *VirtualFileSystem
	if c.app.assetBrowser != nil {
		vfs = c.app.assetBrowser.vfs
	}
	if vfs == nil {
		if c.app.config.GamedataPath == "" && c.app.config.TextAssetsPath == "" {
			c.refErr = fmt.Errorf("no gamedata/TextAssets root configured")
			return nil, c.refErr
		}
		vfs = NewVirtualFileSystem(c.app.config.GamedataPath, c.app.config.TextAssetsPath)
		if err := vfs.Refresh(); err != nil {
			c.refErr = fmt.Errorf("asset scan failed: %w", err)
			return nil, c.refErr
		}
	}

	names := make(map[string]bool)
	var incomplete []string
	for _, source := range vfs.Search("ext_data/mb2/character") {
		if !strings.EqualFold(filepath.Ext(source.Path), ".mbch") {
			continue
		}
		names[strings.ToLower(strings.TrimSuffix(filepath.Base(source.Path), filepath.Ext(source.Path)))] = true

		// Read through the VFS by logical path, so the same winning loose
		// or PK3 content the app uses supplies the canonical Name.
		rc, err := vfs.ReadFile(source.Path)
		if err != nil {
			incomplete = append(incomplete, source.Path)
			continue
		}
		data, readErr := readAllLimited(rc, parsers.MBCHMaxFileBytes)
		closeErr := rc.Close()
		if readErr != nil || closeErr != nil {
			incomplete = append(incomplete, source.Path)
			continue
		}
		char, parseErr := parsers.ParseMBCH(string(data))
		if parseErr != nil {
			incomplete = append(incomplete, source.Path)
			continue
		}
		if name := strings.TrimSpace(char.Name); name != "" {
			names[strings.ToLower(name)] = true
		}
	}
	if len(incomplete) > 0 {
		c.refErr = fmt.Errorf("class reference index incomplete: %d winning .mbch file(s) unreadable", len(incomplete))
		return nil, c.refErr
	}
	if len(names) == 0 {
		c.refErr = fmt.Errorf("no .mbch files found in the app VFS")
		return nil, c.refErr
	}
	c.refIndex = names
	c.refAvailable = true
	return names, nil
}

// readAllLimited reads r up to limit bytes; larger files fail rather
// than being silently truncated (a truncated parse could produce a
// wrong class name).
func readAllLimited(r io.Reader, limit int64) ([]byte, error) {
	data, err := io.ReadAll(io.LimitReader(r, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > limit {
		return nil, fmt.Errorf("file exceeds scan limit of %d bytes", limit)
	}
	return data, nil
}

func (c *MBTCComposerDialog) saveFile() {
	refWarnings, err := c.validate()
	if err != nil {
		c.statusLabel.SetText("Validation: " + err.Error())
		dialog.ShowError(err, c.dialogWin)
		return
	}

	output, err := c.serialize()
	if err != nil {
		// Keep the correctable input in the UI; nothing is written.
		c.statusLabel.SetText("Not saved: " + err.Error())
		dialog.ShowError(err, c.dialogWin)
		return
	}
	// Engine gate: BG_SiegeParseTeamFile refuses len >= MAX_TEAM_FILE_LEN.
	if len(output) >= parsers.MBTCMaxFileBytes {
		msg := fmt.Sprintf("not saved: team file is %d bytes — the engine rejects files of %d bytes or more (bg_saga.c:3261)", len(output), parsers.MBTCMaxFileBytes)
		c.statusLabel.SetText(msg)
		dialog.ShowError(fmt.Errorf("%s", msg), c.dialogWin)
		return
	}

	if c.filePath != "" {
		if err := c.commitSave(c.filePath, []byte(output)); err != nil {
			if saveWasPublished(err) {
				c.statusLabel.SetText("Published, but save finalization failed: " + err.Error())
			} else {
				c.statusLabel.SetText("Not saved: " + err.Error())
			}
			dialog.ShowError(err, c.dialogWin)
			return
		}
		c.lastDest = c.filePath
		c.setSavedStatus(filepath.Base(c.filePath), refWarnings)
		return
	}

	// No existing file — choose a destination without opening it. The
	// chooser keeps its folder/name entries across failed attempts, so
	// retry/cancel never loses the chosen destination or name.
	ShowSavePathDialog(c.dialogWin, "Save Team Configuration", c.defaultSavePath(), ".mbtc", func(path string) error {
		if err := c.commitSave(path, []byte(output)); err != nil {
			return err
		}
		c.filePath = path
		c.lastDest = path
		c.setSavedStatus(filepath.Base(path), refWarnings)
		return nil
	})
}

// commitSave is the checked write boundary used by both Save and Save As.
// A loaded file must still match its open baseline. A Save As destination
// is snapshotted before publication, including nonexistence, so creation or
// replacement races never clobber another writer.
func (c *MBTCComposerDialog) commitSave(path string, data []byte) error {
	reviewed, err := snapshotSaveDestination(path)
	if err != nil {
		return err
	}
	if c.baselineSet && sameDestinationPath(path, c.baselinePath) {
		reviewed = c.baseline
	}

	var fileManager *FileManager
	if c.app != nil {
		fileManager = c.app.fileManager
	}
	result, err := publishEditorCandidateReviewedWithHooks(
		fileManager,
		path,
		string(data),
		reviewed,
		savePublicationHooks{
			beforeDestinationMove: func() {
				if c.beforePublication != nil {
					c.beforePublication(path)
				}
			},
			afterCandidatePublish: func(holdPath, backupPath string) {
				if c.afterCandidatePublish != nil {
					c.afterCandidatePublish(path, holdPath, backupPath)
				}
			},
		},
	)
	if err != nil {
		if result.Published {
			return fmt.Errorf("candidate bytes were published, but save finalization failed; editor remains dirty and recovery paths follow: %w", err)
		}
		return err
	}
	published, snapshotErr := snapshotSaveDestination(path)
	if snapshotErr != nil {
		return fmt.Errorf("save published, but establishing the new baseline failed: %w", snapshotErr)
	}
	c.baselinePath = path
	c.baseline = published
	c.baselineSet = true
	return nil
}

func sameDestinationPath(left, right string) bool {
	leftAbs, leftErr := filepath.Abs(left)
	rightAbs, rightErr := filepath.Abs(right)
	if leftErr == nil && rightErr == nil {
		return leftAbs == rightAbs
	}
	return filepath.Clean(left) == filepath.Clean(right)
}

// setSavedStatus reports a successful save WITHOUT discarding reference
// warnings — "Saved to X" must not erase the resolution note the user
// still needs to see.
func (c *MBTCComposerDialog) setSavedStatus(name string, warnings []string) {
	msg := "Saved to " + name
	if len(warnings) > 0 {
		msg += " — " + strings.Join(warnings, "; ")
	}
	c.statusLabel.SetText(msg)
}

// defaultSavePath seeds the chooser with the last successful
// destination, falling back to a default name in the last-used directory.
func (c *MBTCComposerDialog) defaultSavePath() string {
	if c.lastDest != "" {
		return c.lastDest
	}
	return "Untitled.mbtc"
}

// serialize collects the UI and renders the .mbtc text. An inconsistent
// form is an error, never a silent fallback: the caller keeps the
// correctable input on screen.
func (c *MBTCComposerDialog) serialize() (string, error) {
	if err := c.collectUI(); err != nil {
		return "", err
	}
	return c.roster.Serialize(), nil
}
