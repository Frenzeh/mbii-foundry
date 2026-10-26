package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/storage"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/Frenzeh/mbii-foundry/parsers"
)

// BulkFieldResult records the per-file outcome of a bulk operation.
// Rows are produced identically by Preview and Apply: Apply re-runs the
// same plan function and refuses to write files whose content changed
// since the preview (stale detection), so what the preview showed is
// exactly what Apply writes.
type BulkFieldResult struct {
	Path       string
	OldValue   string
	OldFound   bool // distinguishes an authored empty/false value from absence
	NewValue   string
	Scope      string // effective location, e.g. "ClassInfo", "group RedTeam"
	Defs       int    // top-level definitions in the document
	Err        error
	BackupPath string

	// Batch recovery outcomes are explicit so a consumer never has to
	// infer "rollback failed" or "nothing was written" from Err text.
	RolledBack       bool
	RollbackConflict bool
	RollbackFailed   bool
	NoRollbackNeeded bool
}

// bulkPlan is the prepared write for one file, captured during preview.
type bulkPlan struct {
	source  []byte // exact document bytes the plan was computed from
	key     string // exact normalized parsed key
	value   string // exact (untrimmed) replacement value
	result  BulkFieldResult
	content string // exact planned replacement document
}

// bulkParameters pins everything a preview depends on. Any change to the
// key, the value, or the file selection invalidates the plan: Apply
// never writes content previewed for different parameters.
type bulkParameters struct {
	key       string
	value     string
	selection map[string]bool
}

func (p *bulkParameters) matches(key, value string, selection map[string]bool) bool {
	if p.key != key || p.value != value {
		return false
	}
	if len(p.selection) != len(selection) {
		return false
	}
	// Compare the CURRENT selection against the CAPTURED one — the loop
	// must read p.selection, not the map being iterated.
	for k, v := range selection {
		pv, ok := p.selection[k]
		if !ok || pv != v {
			return false
		}
	}
	return true
}

var bulkFileExtensions = []string{".mbch", ".sab", ".veh", ".siege", ".mbtc"}

// bulkFileIssue keeps a discovery/load failure attached to the exact path
// that caused it. Loader failures are rendered in a dedicated report instead
// of silently dropping files from the batch.
type bulkFileIssue struct {
	Path string
	Err  error
}

// bulkFileDiscovery is the complete, uncapped result of one picker action.
// Files contains canonical paths for supported regular files only.
type bulkFileDiscovery struct {
	Files       []string
	Unsupported []string
	Symlinks    []string
	Duplicates  []string
	Errors      []bulkFileIssue
}

func isBulkFile(path string) bool {
	_, ok := parsers.FormatFromPath(path)
	return ok
}

// canonicalBulkPath returns a stable absolute path for de-duplication.
func canonicalBulkPath(path string) (string, error) {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	return filepath.EvalSymlinks(filepath.Clean(absolute))
}

func pathWithin(root, path string) bool {
	relative, err := filepath.Rel(root, path)
	if err != nil {
		return false
	}
	return relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator))
}

// discoverBulkFiles checks paths chosen individually. An explicitly chosen
// symlink is resolved to its canonical target; folder discovery is stricter
// and never follows symlinks at all.
func discoverBulkFiles(paths []string) bulkFileDiscovery {
	var result bulkFileDiscovery
	seen := make(map[string]bool)
	for _, path := range paths {
		canonical, err := canonicalBulkPath(path)
		if err != nil {
			result.Errors = append(result.Errors, bulkFileIssue{Path: path, Err: err})
			continue
		}
		info, err := os.Stat(canonical)
		if err != nil {
			result.Errors = append(result.Errors, bulkFileIssue{Path: path, Err: err})
			continue
		}
		if !info.Mode().IsRegular() {
			result.Errors = append(result.Errors, bulkFileIssue{Path: path, Err: fmt.Errorf("not a regular file")})
			continue
		}
		if !isBulkFile(canonical) {
			result.Unsupported = append(result.Unsupported, canonical)
			continue
		}
		if seen[canonical] {
			result.Duplicates = append(result.Duplicates, canonical)
			continue
		}
		seen[canonical] = true
		result.Files = append(result.Files, canonical)
	}
	return result
}

// scanBulkFolder recursively discovers supported files beneath root. WalkDir
// does not follow directory symlinks; explicit Lstat checks also skip file
// symlinks, and the canonical containment check guards against replacement
// races that could otherwise escape the chosen root.
func scanBulkFolder(root string) bulkFileDiscovery {
	var result bulkFileDiscovery
	absolute, err := filepath.Abs(root)
	if err != nil {
		result.Errors = append(result.Errors, bulkFileIssue{Path: root, Err: err})
		return result
	}
	absolute = filepath.Clean(absolute)
	rootInfo, err := os.Lstat(absolute)
	if err != nil {
		result.Errors = append(result.Errors, bulkFileIssue{Path: absolute, Err: err})
		return result
	}
	if rootInfo.Mode()&os.ModeSymlink != 0 {
		result.Symlinks = append(result.Symlinks, absolute)
		return result
	}
	if !rootInfo.IsDir() {
		result.Errors = append(result.Errors, bulkFileIssue{Path: absolute, Err: fmt.Errorf("not a folder")})
		return result
	}
	canonicalRoot, err := canonicalBulkPath(absolute)
	if err != nil {
		result.Errors = append(result.Errors, bulkFileIssue{Path: absolute, Err: err})
		return result
	}

	seen := make(map[string]bool)
	err = filepath.WalkDir(absolute, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			result.Errors = append(result.Errors, bulkFileIssue{Path: path, Err: walkErr})
			if entry != nil && entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if path == absolute {
			return nil
		}
		if entry.Type()&os.ModeSymlink != 0 {
			result.Symlinks = append(result.Symlinks, path)
			return nil
		}
		if entry.IsDir() {
			return nil
		}
		info, infoErr := entry.Info()
		if infoErr != nil {
			result.Errors = append(result.Errors, bulkFileIssue{Path: path, Err: infoErr})
			return nil
		}
		if !info.Mode().IsRegular() {
			result.Errors = append(result.Errors, bulkFileIssue{Path: path, Err: fmt.Errorf("not a regular file")})
			return nil
		}
		if !isBulkFile(path) {
			result.Unsupported = append(result.Unsupported, path)
			return nil
		}
		canonical, canonicalErr := canonicalBulkPath(path)
		if canonicalErr != nil {
			result.Errors = append(result.Errors, bulkFileIssue{Path: path, Err: canonicalErr})
			return nil
		}
		if !pathWithin(canonicalRoot, canonical) {
			result.Symlinks = append(result.Symlinks, path)
			return nil
		}
		if seen[canonical] {
			result.Duplicates = append(result.Duplicates, canonical)
			return nil
		}
		seen[canonical] = true
		result.Files = append(result.Files, canonical)
		return nil
	})
	if err != nil {
		result.Errors = append(result.Errors, bulkFileIssue{Path: absolute, Err: err})
	}
	return result
}

// parseBulkFile proves that a supported candidate can be read and parsed by
// its exact format before it enters the selectable batch.
func parseBulkFile(path string) error {
	content, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if diagnostics := parsers.LexDiagnostics(string(content)); len(diagnostics) > 0 {
		return fmt.Errorf("%s", strings.Join(diagnostics, "; "))
	}
	format, ok := parsers.FormatFromPath(path)
	if !ok {
		return fmt.Errorf("unsupported file type %q", filepath.Ext(path))
	}
	switch format {
	case parsers.FormatMBCH:
		_, err = parsers.ParseMBCH(string(content))
	case parsers.FormatSAB:
		_, err = parsers.ParseSAB(string(content))
	case parsers.FormatVEH:
		_, err = parsers.ParseVEH(string(content))
	case parsers.FormatSIEGE:
		_, err = parsers.ParseSiege(string(content))
	case parsers.FormatMBTC:
		team := parsers.ParseMBTC(string(content))
		if len(team.Diagnostics) > 0 {
			err = fmt.Errorf("%s", strings.Join(team.Diagnostics, "; "))
		}
	}
	return err
}

// BulkEditor provides parsed-key bulk preview and validated writes
// across multiple MBII config files (.mbch, .sab, .veh, .siege, .mbtc).
type BulkEditor struct {
	container *container.Split

	app *App // owning application: window + FileManager; both required

	files     []string
	selection map[string]bool

	fileList        *widget.List
	fieldEntry      *widget.Entry // free-form parsed key (case-insensitive match)
	valueEntry      *widget.Entry
	previewList     *widget.List
	previewData     []BulkFieldResult
	plans           map[string]*bulkPlan // path → planned write from the last preview
	params          *bulkParameters      // parameters the plans were computed for
	applyBtn        *widget.Button
	previewBtn      *widget.Button
	addFileBtn      *widget.Button
	addFolderBtn    *widget.Button
	removeBtn       *widget.Button
	clearBtn        *widget.Button
	statusLabel     *widget.Label
	loadReportLabel *widget.Label

	// Tests use this to inject a destination mutation after the early stale
	// check but before the guarded publication check.
	beforePublication func(string)

	// Tests use this to inject a deterministic post-publication proof failure.
	afterCandidatePublish func(path, holdPath, backupPath string)
}

// NewBulkEditor requires the owning app: dialogs target
// app.mainWindow (never an enumerated window) and pre-edit backups go
// through the accepted FileManager backup store.
func NewBulkEditor(app *App) *BulkEditor {
	be := &BulkEditor{
		app:       app,
		selection: make(map[string]bool),
		files:     []string{},
		plans:     make(map[string]*bulkPlan),
	}
	be.createUI()
	be.wirePreviewInvalidation()
	return be
}

// parent returns the owning window. There is deliberately no fallback:
// an unnamed window would misroute dialogs, so without a wired app the
// editor only reports through the status label.
func (be *BulkEditor) parent() fyne.Window {
	if be.app == nil {
		return nil
	}
	return be.app.mainWindow
}

func (be *BulkEditor) showError(err error) {
	if win := be.parent(); win != nil {
		dialog.ShowError(err, win)
		return
	}
	be.statusLabel.SetText(err.Error())
}

// wirePreviewInvalidation makes every input that a preview depends on
// invalidate the current plan: Apply must never consume content that was
// computed for a different key, value, or selection.
func (be *BulkEditor) wirePreviewInvalidation() {
	be.fieldEntry.OnChanged = func(string) { be.invalidatePreview() }
	be.valueEntry.OnChanged = func(string) { be.invalidatePreview() }
}

// invalidatePreview drops the plans and disables Apply. The rendered
// rows stay visible (marked stale by the status line) so the user can
// see what will be recomputed.
func (be *BulkEditor) invalidatePreview() {
	if be.params == nil {
		return
	}
	be.params = nil
	be.plans = make(map[string]*bulkPlan)
	be.applyBtn.Disable()
	be.statusLabel.SetText("Preview is stale — the field, value, or selection changed. Run Preview again.")
}

func (be *BulkEditor) createUI() {
	be.fileList = widget.NewList(
		func() int { return len(be.files) },
		func() fyne.CanvasObject {
			return container.NewHBox(widget.NewCheck("", nil), widget.NewLabel("File"))
		},
		func(id widget.ListItemID, obj fyne.CanvasObject) {
			path := be.files[id]
			check := obj.(*fyne.Container).Objects[0].(*widget.Check)
			check.Checked = be.selection[path]
			check.OnChanged = func(b bool) {
				be.selection[path] = b
				be.invalidatePreview() // selection is part of the plan
			}

			label := obj.(*fyne.Container).Objects[1].(*widget.Label)
			label.SetText(path)
		},
	)

	be.addFileBtn = widget.NewButtonWithIcon("Add File…", theme.FileIcon(), be.showAddFilePicker)
	be.addFolderBtn = widget.NewButtonWithIcon("Add Folder…", theme.FolderOpenIcon(), be.showAddFolderPicker)
	selectAll := widget.NewButton("Select All", func() {
		for _, p := range be.files {
			be.selection[p] = true
		}
		be.fileList.Refresh()
		be.invalidatePreview()
	})
	selectAll.Importance = widget.LowImportance
	selectNone := widget.NewButton("Select None", func() {
		for _, p := range be.files {
			be.selection[p] = false
		}
		be.fileList.Refresh()
		be.invalidatePreview()
	})
	selectNone.Importance = widget.LowImportance
	be.removeBtn = widget.NewButton("Remove Selected", be.removeSelected)
	be.removeBtn.Importance = widget.LowImportance
	be.clearBtn = widget.NewButton("Clear Batch", be.clearBatch)
	be.clearBtn.Importance = widget.LowImportance

	be.fieldEntry = NewInputEntry()
	be.fieldEntry.SetPlaceHolder("Field key (e.g. maxhealth, speedMax, model)")
	be.valueEntry = NewInputEntry()
	be.valueEntry.SetPlaceHolder("New value")
	be.statusLabel = widget.NewLabel("")
	be.statusLabel.Wrapping = fyne.TextWrapWord

	// Preview list shows per-file current→new
	be.previewList = widget.NewList(
		func() int { return len(be.previewData) },
		func() fyne.CanvasObject {
			return container.NewHBox(
				widget.NewIcon(theme.ConfirmIcon()),
				widget.NewLabel("filename"),
				widget.NewLabel("old → new"),
			)
		},
		func(id widget.ListItemID, obj fyne.CanvasObject) {
			r := be.previewData[id]
			icon := obj.(*fyne.Container).Objects[0].(*widget.Icon)
			nameLabel := obj.(*fyne.Container).Objects[1].(*widget.Label)
			detailLabel := obj.(*fyne.Container).Objects[2].(*widget.Label)

			nameLabel.SetText(filepath.Base(r.Path))
			switch {
			case r.RollbackFailed:
				icon.SetResource(theme.ErrorIcon())
				detailLabel.SetText(r.Err.Error())
			case r.RollbackConflict:
				icon.SetResource(theme.WarningIcon())
				detail := fmt.Sprintf("changed after apply — newer content kept; pre-edit original in %s", filepath.Base(r.BackupPath))
				if r.Err != nil {
					detail += "; triggering error: " + r.Err.Error()
				}
				detailLabel.SetText(detail)
			case r.RolledBack:
				icon.SetResource(theme.WarningIcon())
				detail := fmt.Sprintf("written then rolled back — %s → %s", r.OldValue, r.NewValue)
				if r.Err != nil {
					detail += "; triggering error: " + r.Err.Error()
				}
				detailLabel.SetText(detail)
			case r.Err != nil:
				icon.SetResource(theme.ErrorIcon())
				detailLabel.SetText(r.Err.Error())
			case !r.OldFound:
				icon.SetResource(theme.ContentAddIcon())
				detailLabel.SetText(fmt.Sprintf("(missing) → %s  [%s]", r.NewValue, r.Scope))
			default:
				icon.SetResource(theme.ConfirmIcon())
				detailLabel.SetText(fmt.Sprintf("%s → %s  [%s]", r.OldValue, r.NewValue, r.Scope))
			}
		},
	)

	be.previewBtn = widget.NewButtonWithIcon("Preview", theme.VisibilityIcon(), be.runPreview)
	be.applyBtn = widget.NewButtonWithIcon("Apply to Selected", theme.ConfirmIcon(), be.applyChanges)
	be.applyBtn.Importance = widget.HighImportance
	be.applyBtn.Disable() // enabled only after a complete, current preview

	controls := container.NewVBox(
		widget.NewLabelWithStyle("Bulk Operations", fyne.TextAlignCenter, fyne.TextStyle{Bold: true}),
		widget.NewLabel("Field (case-insensitive):"),
		be.fieldEntry,
		widget.NewLabel("Value:"),
		be.valueEntry,
		container.NewHBox(be.previewBtn, be.applyBtn),
		widget.NewSeparator(),
		widget.NewLabel("Preview:"),
	)

	be.loadReportLabel = widget.NewLabel("No files loaded.")
	be.loadReportLabel.Wrapping = fyne.TextWrapWord
	reportScroll := container.NewVScroll(be.loadReportLabel)
	reportScroll.SetMinSize(fyne.NewSize(0, 96))
	fileControls := container.NewVBox(
		container.NewHBox(be.addFileBtn, be.addFolderBtn),
		widget.NewLabel("Add Folder scans recursively. Symlinks are skipped and never followed."),
		container.NewHBox(selectAll, selectNone),
		container.NewHBox(be.removeBtn, be.clearBtn),
		reportScroll,
	)
	leftPane := container.NewBorder(fileControls, nil, nil, nil, be.fileList)
	rightPane := container.NewBorder(controls, be.statusLabel, nil, nil, be.previewList)

	be.container = container.NewHSplit(container.NewPadded(leftPane), container.NewPadded(rightPane))
	be.container.SetOffset(0.4)
}

func (be *BulkEditor) GetContent() fyne.CanvasObject { return be.container }

func (be *BulkEditor) LoadFiles(paths []string) {
	result := discoverBulkFiles(paths)
	parseBulkDiscovery(&result)
	be.applyBulkDiscovery(result, true)
}

func parseBulkDiscovery(result *bulkFileDiscovery) {
	files := result.Files[:0]
	for _, path := range result.Files {
		if err := parseBulkFile(path); err != nil {
			result.Errors = append(result.Errors, bulkFileIssue{Path: path, Err: err})
			continue
		}
		files = append(files, path)
	}
	result.Files = files
}

func (be *BulkEditor) applyBulkDiscovery(result bulkFileDiscovery, replace bool) {
	if replace {
		be.files = nil
		be.selection = make(map[string]bool)
	}
	existing := make(map[string]bool, len(be.files))
	for _, path := range be.files {
		existing[path] = true
	}
	added := make([]string, 0, len(result.Files))
	for _, path := range result.Files {
		if existing[path] {
			result.Duplicates = append(result.Duplicates, path)
			continue
		}
		existing[path] = true
		be.files = append(be.files, path)
		be.selection[path] = true
		added = append(added, path)
	}
	result.Files = added
	be.resetPreview()
	be.fileList.Refresh()
	be.loadReportLabel.SetText(formatBulkLoadReport(result, len(be.files)))
}

func (be *BulkEditor) resetPreview() {
	be.previewData = nil
	be.plans = make(map[string]*bulkPlan)
	be.params = nil
	be.applyBtn.Disable()
	be.previewList.Refresh()
}

func (be *BulkEditor) removeSelected() {
	kept := be.files[:0]
	removed := 0
	for _, path := range be.files {
		if be.selection[path] {
			delete(be.selection, path)
			removed++
			continue
		}
		kept = append(kept, path)
	}
	be.files = kept
	be.resetPreview()
	be.fileList.Refresh()
	be.loadReportLabel.SetText(fmt.Sprintf("Removed %d selected file(s). %d file(s) remain in the batch.", removed, len(be.files)))
}

func (be *BulkEditor) clearBatch() {
	be.files = nil
	be.selection = make(map[string]bool)
	be.resetPreview()
	be.fileList.Refresh()
	be.loadReportLabel.SetText("Batch cleared.")
}

func (be *BulkEditor) showAddFilePicker() {
	parent := be.parent()
	if parent == nil {
		be.showError(fmt.Errorf("bulk file picker requires the owning application window"))
		return
	}
	picker := dialog.NewFileOpen(func(reader fyne.URIReadCloser, err error) {
		if err != nil {
			be.showError(err)
			return
		}
		if reader == nil {
			return
		}
		path := reader.URI().Path()
		_ = reader.Close()
		result := discoverBulkFiles([]string{path})
		parseBulkDiscovery(&result)
		be.applyBulkDiscovery(result, false)
	}, parent)
	picker.SetFilter(storage.NewExtensionFileFilter(bulkFileExtensions))
	picker.SetTitleText("Add an MBII config file")
	picker.Show()
}

func (be *BulkEditor) showAddFolderPicker() {
	parent := be.parent()
	if parent == nil {
		be.showError(fmt.Errorf("bulk folder picker requires the owning application window"))
		return
	}
	picker := dialog.NewFolderOpen(func(uri fyne.ListableURI, err error) {
		if err != nil {
			be.showError(err)
			return
		}
		if uri == nil {
			return
		}
		be.statusLabel.SetText("Scanning folder recursively…")
		go func(path string) {
			result := scanBulkFolder(path)
			parseBulkDiscovery(&result)
			fyne.Do(func() {
				be.applyBulkDiscovery(result, false)
				be.statusLabel.SetText("")
			})
		}(uri.Path())
	}, parent)
	picker.SetTitleText("Add MBII config files from folder (recursive)")
	picker.Show()
}

func formatBulkLoadReport(result bulkFileDiscovery, batchSize int) string {
	var report strings.Builder
	fmt.Fprintf(&report, "Batch: %d parsed supported file(s). Added %d.", batchSize, len(result.Files))
	writePaths := func(title string, paths []string) {
		if len(paths) == 0 {
			return
		}
		fmt.Fprintf(&report, "\n%s (%d):", title, len(paths))
		for _, path := range paths {
			fmt.Fprintf(&report, "\n• %s", path)
		}
	}
	writePaths("Skipped unsupported files", result.Unsupported)
	writePaths("Skipped symlinks (not followed)", result.Symlinks)
	writePaths("Skipped duplicates", result.Duplicates)
	if len(result.Errors) > 0 {
		fmt.Fprintf(&report, "\nLoad errors (%d):", len(result.Errors))
		for _, issue := range result.Errors {
			fmt.Fprintf(&report, "\n• %s: %v", issue.Path, issue.Err)
		}
	}
	return report.String()
}

// selectedFiles returns the paths the user has checked.
func (be *BulkEditor) selectedFiles() []string {
	var sel []string
	for _, p := range be.files {
		if be.selection[p] {
			sel = append(sel, p)
		}
	}
	return sel
}

// planFile computes the planned replacement for one file using the
// canonical parser (parsers.EditConfigField) plus the consumer
// validation gate (parsers.ValidateSerialized). Both Preview and Apply
// go through this function, so the preview rows are the write plan.
func planFile(path, key, val string) (*bulkPlan, error) {
	format, ok := parsers.FormatFromPath(path)
	if !ok {
		return nil, fmt.Errorf("unsupported file type: %s", filepath.Ext(path))
	}
	source, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read: %w", err)
	}
	edit, err := parsers.EditConfigField(format, string(source), key, val)
	if err != nil {
		return nil, err
	}
	if err := parsers.ValidateSerialized(format, edit.Content, key, val); err != nil {
		return nil, fmt.Errorf("consumer validation: %w", err)
	}
	return &bulkPlan{
		source:  source,
		key:     key,
		value:   val,
		content: edit.Content,
		result: BulkFieldResult{
			Path:     path,
			OldValue: edit.OldValue,
			OldFound: edit.Found,
			NewValue: val,
			Scope:    edit.Scope,
			Defs:     edit.Definitions,
		},
	}, nil
}

// runPreview plans the edit for every selected file. Any plan failure
// keeps Apply disabled — a partial preview must never enable a partial
// write of stale intent.
func (be *BulkEditor) runPreview() {
	be.applyBtn.Disable()
	be.plans = make(map[string]*bulkPlan)
	be.params = nil
	be.previewData = nil
	be.previewList.Refresh()

	rawKey, val := be.fieldEntry.Text, be.valueEntry.Text
	key := strings.TrimSpace(rawKey)
	if key == "" {
		be.statusLabel.SetText("Enter a field key to preview.")
		return
	}
	if val == "" {
		be.statusLabel.SetText("Enter a new value.")
		return
	}
	sel := be.selectedFiles()
	if len(sel) == 0 {
		be.statusLabel.SetText("Select at least one file.")
		return
	}

	be.plans = make(map[string]*bulkPlan, len(sel))
	planFailures := 0
	for _, path := range sel {
		plan, err := planFile(path, key, val)
		if err != nil {
			be.previewData = append(be.previewData, BulkFieldResult{Path: path, NewValue: val, Err: err})
			planFailures++
			continue
		}
		be.plans[path] = plan
		be.previewData = append(be.previewData, plan.result)
	}

	selCopy := make(map[string]bool, len(be.selection))
	for k, v := range be.selection {
		selCopy[k] = v
	}
	be.params = &bulkParameters{key: rawKey, value: val, selection: selCopy}
	be.previewList.Refresh()

	if planFailures > 0 {
		be.statusLabel.SetText(fmt.Sprintf("Preview: %d of %d files plannable — failures are shown and Apply stays disabled until Preview succeeds for all selected files.", len(sel)-planFailures, len(sel)))
		return
	}
	be.applyBtn.Enable()
	be.statusLabel.SetText(fmt.Sprintf("Preview: %d files plannable. Review before applying.", len(be.previewData)))
}

// applyChanges writes the planned changes with full batch recovery:
//
//  1. Parameter match — key/value/selection must still be exactly what
//     the preview was computed for.
//  2. Stale detection — a file whose content changed since preview is
//     refused (never written on a stale basis).
//  3. Backup — each original goes through the accepted FileManager
//     CreateBackup (unique, collision-resistant names), so originals of
//     consecutive batches are all retained.
//  4. Atomic write per file via safeio.
//  5. Automatic rollback — if any file fails after another was written,
//     every file written in this batch is restored from its in-memory
//     original, UNLESS it changed after Apply: a conflict preserves the
//     newer bytes and names the backup instead of clobbering them.
func (be *BulkEditor) applyChanges() {
	rawKey, val := be.fieldEntry.Text, be.valueEntry.Text
	key := strings.TrimSpace(rawKey)
	if key == "" || val == "" {
		be.statusLabel.SetText("Field and value are required.")
		return
	}
	sel := be.selectedFiles()
	if len(sel) == 0 {
		be.statusLabel.SetText("No files selected.")
		return
	}
	if be.params == nil {
		be.statusLabel.SetText("Apply refused: no current preview. Run Preview again.")
		return
	}
	if !be.params.matches(rawKey, val, be.selection) {
		be.invalidatePreview()
		be.statusLabel.SetText("Apply refused: the preview was computed for a different field, value, or selection. Run Preview again.")
		return
	}
	if be.app == nil || be.app.fileManager == nil {
		be.statusLabel.SetText("Apply unavailable: no FileManager configured for pre-edit backups.")
		return
	}

	be.previewData = nil
	written := make([]string, 0, len(sel)) // paths written so far this batch
	planned := make(map[string]string, len(sel))
	plannedModes := make(map[string]os.FileMode, len(sel))
	originals := make(map[string][]byte, len(sel))
	backups := make(map[string]string, len(sel))
	failed, applied := 0, 0
	batchFailed := false

	for _, path := range sel {
		plan, ok := be.plans[path]
		if !ok {
			be.previewData = append(be.previewData, BulkFieldResult{
				Path: path, Err: fmt.Errorf("no valid plan from preview — run Preview again"), NoRollbackNeeded: true,
			})
			failed++
			batchFailed = true
			continue
		}
		if plan.key != key || plan.value != val {
			be.previewData = append(be.previewData, BulkFieldResult{
				Path: path, Err: fmt.Errorf("captured plan parameters differ — run Preview again"), NoRollbackNeeded: true,
			})
			failed++
			batchFailed = true
			continue
		}

		reviewed, err := snapshotSaveDestination(path)
		if err != nil {
			be.previewData = append(be.previewData, BulkFieldResult{
				Path: path, Err: fmt.Errorf("read: %w", err), NoRollbackNeeded: true,
			})
			failed++
			batchFailed = true
			continue
		}
		if !reviewed.Exists || reviewed.Bytes != string(plan.source) {
			be.previewData = append(be.previewData, BulkFieldResult{
				Path: path, Err: fmt.Errorf("changed since preview — re-run Preview to include it"), NoRollbackNeeded: true,
			})
			failed++
			batchFailed = true
			continue
		}
		if bytes.Equal(plan.source, []byte(plan.content)) {
			row := plan.result
			row.NoRollbackNeeded = true
			be.previewData = append(be.previewData, row)
			continue
		}

		if batchFailed {
			be.previewData = append(be.previewData, BulkFieldResult{
				Path: path, Err: fmt.Errorf("skipped — batch already failed"), NoRollbackNeeded: true,
			})
			failed++
			continue
		}

		result, err := publishEditorCandidateReviewedWithHooks(
			be.app.fileManager,
			path,
			plan.content,
			reviewed,
			savePublicationHooks{
				beforeDestinationMove: func() {
					if be.beforePublication != nil {
						be.beforePublication(path)
					}
				},
				afterCandidatePublish: func(holdPath, backupPath string) {
					if be.afterCandidatePublish != nil {
						be.afterCandidatePublish(path, holdPath, backupPath)
					}
				},
			},
		)
		if err != nil && (strings.Contains(err.Error(), "destination changed") ||
			strings.Contains(err.Error(), "destination was created") ||
			strings.Contains(err.Error(), "destination was deleted")) {
			err = fmt.Errorf("changed since preview during apply — external update preserved: %w", err)
		}
		if result.BackupPath != "" {
			backups[path] = result.BackupPath
		}
		if err != nil {
			row := BulkFieldResult{
				Path:             path,
				BackupPath:       result.BackupPath,
				Err:              err,
				NoRollbackNeeded: !result.Published,
			}
			if result.Published {
				originals[path] = plan.source
				planned[path] = plan.content
				plannedModes[path] = reviewed.Mode
				written = append(written, path)
				applied++
			}
			be.previewData = append(be.previewData, row)
			failed++
			batchFailed = true
			continue
		}
		originals[path] = plan.source
		planned[path] = plan.content
		plannedModes[path] = reviewed.Mode
		written = append(written, path)
		applied++
		row := plan.result
		row.BackupPath = result.BackupPath
		be.previewData = append(be.previewData, row)
	}

	// Automatic rollback of everything this batch wrote.
	rolledBack, conflicts, restoreFailures, rollbackFinalizationFailures := 0, 0, 0, 0
	if batchFailed && len(written) > 0 {
		for i := len(written) - 1; i >= 0; i-- {
			path := written[i]
			reviewed, err := snapshotSaveDestination(path)
			if err == nil && (!reviewed.Exists ||
				reviewed.Bytes != planned[path] ||
				reviewed.Mode != plannedModes[path]) {
				err = fmt.Errorf("destination changed after apply")
			}
			rollbackResult := savePublicationResult{}
			if err == nil {
				rollbackResult, err = publishEditorCandidateReviewedWithHooks(
					be.app.fileManager,
					path,
					string(originals[path]),
					reviewed,
					savePublicationHooks{},
				)
			}
			if err == nil || rollbackResult.Published {
				rolledBack++
				newFailure := false
				be.markRow(path, func(r *BulkFieldResult) {
					r.RolledBack = true
					r.BackupPath = backups[path]
					if err != nil {
						rollbackFinalizationFailures++
						newFailure = r.Err == nil
						if r.Err == nil {
							r.Err = fmt.Errorf("original was rolled back, but rollback finalization failed: %w", err)
						} else {
							r.Err = fmt.Errorf("%v; original was rolled back, but rollback finalization failed: %w", r.Err, err)
						}
					}
				})
				if newFailure {
					failed++
				}
				continue
			}
			if strings.Contains(err.Error(), "destination changed") ||
				strings.Contains(err.Error(), "destination was deleted") {
				conflicts++
				be.markRow(path, func(r *BulkFieldResult) {
					r.RollbackConflict = true
					r.BackupPath = backups[path]
				})
				continue
			}
			restoreFailures++
			failed++
			be.markRow(path, func(r *BulkFieldResult) {
				r.RollbackFailed = true
				r.BackupPath = backups[path]
				r.Err = fmt.Errorf("ROLLBACK FAILED (%v) — restore manually from %s", err, backups[path])
			})
		}
	}

	be.previewList.Refresh()
	be.applyBtn.Disable()
	be.plans = make(map[string]*bulkPlan) // plans consumed; require re-preview
	be.params = nil

	backupNote := ""
	if len(backups) > 0 {
		backupNote = " Backups retained in the FileManager backup store."
	}
	switch {
	case failed == 0:
		be.statusLabel.SetText(fmt.Sprintf("Applied %q = %q to %d files.%s", key, val, applied, backupNote))
	case restoreFailures > 0:
		be.statusLabel.SetText(fmt.Sprintf("Batch FAILED: %d file(s) could not be rolled back and %d rollback finalization step(s) left recovery artifacts — inspect the named paths. (%d candidate(s) published before failure, %d conflicts preserved.)", restoreFailures, rollbackFinalizationFailures, applied, conflicts))
	case rollbackFinalizationFailures > 0:
		be.statusLabel.SetText(fmt.Sprintf("Batch FAILED: rolled back %d written file(s), but %d rollback finalization step(s) left recovery artifacts; inspect the named paths. (%d conflicts preserved.)", rolledBack, rollbackFinalizationFailures, conflicts))
	case conflicts > 0:
		be.statusLabel.SetText(fmt.Sprintf("Batch failed: %d of %d file(s) reported errors — rolled back %d file(s); %d changed after apply and kept newer content (originals in backups).", failed, len(sel), rolledBack, conflicts))
	case rolledBack > 0:
		be.statusLabel.SetText(fmt.Sprintf("Batch failed: %d of %d file(s) reported errors — rolled back %d written file(s) to originals. Fix the failing files and re-preview.", failed, len(sel), rolledBack))
	default:
		be.statusLabel.SetText(fmt.Sprintf("Applied to %d files, %d failed. Nothing needed rollback. Check preview for details.", applied, failed))
	}

	if failed > 0 {
		var sb strings.Builder
		for _, r := range be.previewData {
			if r.Err != nil {
				fmt.Fprintf(&sb, "✗ %s: %s\n", filepath.Base(r.Path), r.Err)
			}
		}
		be.showError(fmt.Errorf("%d of %d files had errors:\n%s", failed, len(sel), sb.String()))
	}
}

// markRow applies fn to the recorded result row for path, if present.
func (be *BulkEditor) markRow(path string, fn func(*BulkFieldResult)) {
	for i := range be.previewData {
		if be.previewData[i].Path != path {
			continue
		}
		fn(&be.previewData[i])
		return
	}
}
