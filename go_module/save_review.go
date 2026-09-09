package main

// Original-vs-candidate save review. Every explicit Save / Save As
// routes through here: the editor produces the exact candidate bytes
// it would write (PrepareSave — pure, no state mutation), the dialog
// shows those bytes next to the on-disk original, and only explicit
// approval commits them (CommitSave). A cancelled or failed save
// leaves baseline, path and draft state untouched.

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/storage"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"github.com/Frenzeh/mbii-foundry/parsers"
)

type saveDestinationSnapshot struct {
	Exists bool
	Bytes  string
	Mode   os.FileMode
}

// snapshotSaveDestination records exactly what occupies path. Symlinks
// and other non-regular destinations are never reviewable save targets.
func snapshotSaveDestination(path string) (saveDestinationSnapshot, error) {
	info, err := os.Lstat(path)
	if os.IsNotExist(err) {
		return saveDestinationSnapshot{}, nil
	}
	if err != nil {
		return saveDestinationSnapshot{}, fmt.Errorf("inspect destination: %w", err)
	}
	if !info.Mode().IsRegular() {
		return saveDestinationSnapshot{}, fmt.Errorf("destination is not a regular file: %s", path)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return saveDestinationSnapshot{}, fmt.Errorf("read destination: %w", err)
	}
	after, err := os.Lstat(path)
	if err != nil {
		return saveDestinationSnapshot{}, fmt.Errorf("recheck destination: %w", err)
	}
	if !after.Mode().IsRegular() || !os.SameFile(info, after) ||
		after.Size() != int64(len(data)) || after.Mode().Perm() != info.Mode().Perm() {
		return saveDestinationSnapshot{}, fmt.Errorf("destination changed while it was being read")
	}
	return saveDestinationSnapshot{Exists: true, Bytes: string(data), Mode: after.Mode().Perm()}, nil
}

func verifySaveDestination(path string, reviewed saveDestinationSnapshot) error {
	current, err := snapshotSaveDestination(path)
	if err != nil {
		return err
	}
	if current.Exists != reviewed.Exists {
		if reviewed.Exists {
			return fmt.Errorf("destination was deleted after review")
		}
		return fmt.Errorf("destination was created after review")
	}
	if current.Exists && current.Bytes != reviewed.Bytes {
		return fmt.Errorf("destination changed after review")
	}
	if current.Exists && current.Mode != reviewed.Mode {
		return fmt.Errorf("destination mode changed after review")
	}
	return nil
}

var saveDestinationLocks sync.Map // canonical destination path -> *sync.Mutex
func saveDestinationLock(path string) *sync.Mutex {
	canonical, err := filepath.Abs(path)
	if err != nil {
		canonical = filepath.Clean(path)
	}
	if realDir, err := filepath.EvalSymlinks(filepath.Dir(canonical)); err == nil {
		canonical = filepath.Join(realDir, filepath.Base(canonical))
	}
	lock, _ := saveDestinationLocks.LoadOrStore(canonical, &sync.Mutex{})
	return lock.(*sync.Mutex)
}

// stageEditorCandidate writes and syncs candidate beside path. Publication is
// deliberately separate so the destination can be secured in a verified hold
// immediately before the candidate's atomic no-replace rename.
func stageEditorCandidate(path, candidate string, mode os.FileMode) (string, error) {
	if mode == 0 {
		mode = 0644
	}
	f, err := os.CreateTemp(filepath.Dir(path), filepath.Base(path)+".*.tmp")
	if err != nil {
		return "", err
	}
	tmp := f.Name()
	ok := false
	defer func() {
		if !ok {
			f.Close()
			os.Remove(tmp)
		}
	}()
	if _, err := f.WriteString(candidate); err != nil {
		return "", err
	}
	if err := f.Chmod(mode.Perm()); err != nil {
		return "", err
	}
	if err := f.Sync(); err != nil {
		return "", err
	}
	if err := f.Close(); err != nil {
		return "", err
	}
	ok = true
	return tmp, nil
}

type savePublicationHooks struct {
	// beforeDestinationMove runs after candidate and backup preparation but
	// before an existing destination is moved. It preserves the original test
	// seam used by bulk, MBTC, and editor commit race tests.
	beforeDestinationMove func()
	// beforeCandidatePublish runs only after an existing destination has been
	// moved into a hold and verified, or after a new destination was verified
	// absent. It models the last publication race.
	beforeCandidatePublish func(holdPath, backupPath string)
	// afterCandidatePublish is a failure-injection seam for proving that the
	// reviewed hold survives until the backup is verified after publication.
	afterCandidatePublish func(holdPath, backupPath string)
}

type savePublicationResult struct {
	BackupPath string
	Published  bool
}

// savePublicationError preserves whether the candidate reached the destination
// even when a later durability proof or recovery-artifact cleanup fails.
// Wrappers may add context with %w without losing this outcome.
type savePublicationError struct {
	Published bool
	Err       error
}

func (e *savePublicationError) Error() string { return e.Err.Error() }
func (e *savePublicationError) Unwrap() error { return e.Err }

func saveWasPublished(err error) bool {
	var publicationErr *savePublicationError
	return errors.As(err, &publicationErr) && publicationErr.Published
}

// publishEditorCandidateReviewed keeps the reviewed destination snapshot bound
// to publication. Saves to the same canonical path are serialized in-process.
// Cross-process safety comes from platform-native atomic no-replace renames.
func publishEditorCandidateReviewed(fileManager *FileManager, path, candidate string, reviewed saveDestinationSnapshot) error {
	_, err := publishEditorCandidateReviewedWithBackup(fileManager, path, candidate, reviewed)
	return err
}

// publishEditorCandidateReviewedWithBackup additionally reports the exact
// verified backup created for rollback-capable callers.
func publishEditorCandidateReviewedWithBackup(fileManager *FileManager, path, candidate string, reviewed saveDestinationSnapshot) (string, error) {
	return publishEditorCandidateReviewedWithHook(fileManager, path, candidate, reviewed, nil)
}

func publishEditorCandidateReviewedWithHook(fileManager *FileManager, path, candidate string, reviewed saveDestinationSnapshot, beforeDestinationMove func()) (string, error) {
	result, err := publishEditorCandidateReviewedWithHooks(fileManager, path, candidate, reviewed, savePublicationHooks{
		beforeDestinationMove: beforeDestinationMove,
	})
	return result.BackupPath, err
}

func publishEditorCandidateReviewedWithHooks(fileManager *FileManager, path, candidate string, reviewed saveDestinationSnapshot, hooks savePublicationHooks) (result savePublicationResult, err error) {
	defer func() {
		if err != nil {
			err = &savePublicationError{Published: result.Published, Err: err}
		}
	}()

	lock := saveDestinationLock(path)
	lock.Lock()
	defer lock.Unlock()
	if err := verifySaveDestination(path, reviewed); err != nil {
		return result, err
	}
	if reviewed.Exists && reviewed.Bytes == candidate {
		return result, nil
	}

	tmp, err := stageEditorCandidate(path, candidate, reviewed.Mode)
	if err != nil {
		return result, fmt.Errorf("stage candidate: %w", err)
	}
	defer os.Remove(tmp)

	if reviewed.Exists {
		if fileManager == nil {
			return result, fmt.Errorf("backup required before replacing %s, but backup storage is unavailable", filepath.Base(path))
		}
		result.BackupPath, err = fileManager.CreateBackup(path)
		if err != nil {
			return result, fmt.Errorf("backup failed, save aborted: %w", err)
		}
		if result.BackupPath == "" {
			return result, fmt.Errorf("backup failed, save aborted: no backup was created")
		}
		if err := verifyExactSaveSnapshot(result.BackupPath, reviewed); err != nil {
			return result, fmt.Errorf("verify backup %q: %w", result.BackupPath, err)
		}
	}

	if hooks.beforeDestinationMove != nil {
		hooks.beforeDestinationMove()
	}

	if !reviewed.Exists {
		if err := verifySaveDestination(path, reviewed); err != nil {
			return result, fmt.Errorf("destination changed before publication: %w", err)
		}
		if hooks.beforeCandidatePublish != nil {
			hooks.beforeCandidatePublish("", "")
		}
		if err := renameNoReplace(tmp, path); err != nil {
			if os.IsExist(err) {
				return result, fmt.Errorf("publish candidate to %q: destination was created after final review and was preserved", path)
			}
			return result, fmt.Errorf("publish candidate to %q with atomic no-replace rename: %w", path, err)
		}
		result.Published = true
		if hooks.afterCandidatePublish != nil {
			hooks.afterCandidatePublish("", "")
		}
		return result, nil
	}

	holdPath, err := moveSaveDestinationToHold(path)
	if err != nil {
		return result, fmt.Errorf("secure reviewed destination %q before publication: %w", path, err)
	}

	// From this point a crash leaves the reviewed file in holdPath and its
	// verified backup in result.BackupPath. Never defer removal of the hold: it
	// is recovery data until publication and the final backup proof both succeed.
	if err := verifyExactSaveSnapshot(holdPath, reviewed); err != nil {
		cause := fmt.Errorf("destination changed before publication: moved destination %q does not match review: %w", holdPath, err)
		return result, restoreSaveHoldAfterFailure(path, holdPath, result.BackupPath, cause)
	}

	if hooks.beforeCandidatePublish != nil {
		hooks.beforeCandidatePublish(holdPath, result.BackupPath)
	}
	if err := renameNoReplace(tmp, path); err != nil {
		cause := fmt.Errorf("publish candidate to %q with atomic no-replace rename: %w", path, err)
		return result, restoreSaveHoldAfterFailure(path, holdPath, result.BackupPath, cause)
	}
	result.Published = true

	if hooks.afterCandidatePublish != nil {
		hooks.afterCandidatePublish(holdPath, result.BackupPath)
	}
	if err := verifyExactSaveSnapshot(result.BackupPath, reviewed); err != nil {
		return result, fmt.Errorf(
			"candidate was published to %q, but backup proof failed: %w; reviewed original retained at %q; inspect backup %q",
			path, err, holdPath, result.BackupPath,
		)
	}
	if err := os.Remove(holdPath); err != nil {
		return result, fmt.Errorf(
			"candidate was published to %q and backup verified at %q, but reviewed hold cleanup failed: %w; remove hold %q after inspection",
			path, result.BackupPath, err, holdPath,
		)
	}
	return result, nil
}

func verifyExactSaveSnapshot(path string, reviewed saveDestinationSnapshot) error {
	current, err := snapshotSaveDestination(path)
	if err != nil {
		return err
	}
	if !current.Exists {
		return fmt.Errorf("file is missing")
	}
	if current.Bytes != reviewed.Bytes {
		return fmt.Errorf("bytes do not match reviewed destination")
	}
	if current.Mode != reviewed.Mode {
		return fmt.Errorf("mode %04o does not match reviewed mode %04o", current.Mode, reviewed.Mode)
	}
	return nil
}

func moveSaveDestinationToHold(path string) (string, error) {
	var random [16]byte
	for range 100 {
		if _, err := rand.Read(random[:]); err != nil {
			return "", fmt.Errorf("generate hold name: %w", err)
		}
		holdPath := filepath.Join(
			filepath.Dir(path),
			"."+filepath.Base(path)+".reviewed-hold-"+hex.EncodeToString(random[:]),
		)
		if err := renameNoReplace(path, holdPath); err != nil {
			if os.IsExist(err) {
				continue
			}
			return "", err
		}
		return holdPath, nil
	}
	return "", fmt.Errorf("could not allocate a unique reviewed hold path")
}

func restoreSaveHoldAfterFailure(path, holdPath, backupPath string, cause error) error {
	if err := renameNoReplace(holdPath, path); err == nil {
		return fmt.Errorf("%w; moved destination restored to %q; verified backup retained at %q", cause, path, backupPath)
	} else if os.IsExist(err) {
		return fmt.Errorf(
			"%w; another writer's destination at %q was preserved; moved destination retained at %q; verified backup retained at %q",
			cause, path, holdPath, backupPath,
		)
	} else {
		return fmt.Errorf(
			"%w; automatic restore to %q failed: %v; moved destination retained at %q; verified backup retained at %q",
			cause, path, err, holdPath, backupPath,
		)
	}
}

// publishEditorCandidate is the programmatic-save boundary. Its snapshot is
// captured while holding the same per-destination lock used by reviewed saves.
func publishEditorCandidate(fileManager *FileManager, path, candidate string) error {
	lock := saveDestinationLock(path)
	lock.Lock()
	current, err := snapshotSaveDestination(path)
	lock.Unlock()
	if err != nil {
		return err
	}
	return publishEditorCandidateReviewed(fileManager, path, candidate, current)
}

// commitReviewedSave validates the still-current candidate, publishes it
// against the exact reviewed destination snapshot, and only then advances
// editor path/baseline state.
func (a *App) commitReviewedSave(se SessionEditor, path, candidate string, reviewed saveDestinationSnapshot) error {
	return a.commitReviewedSaveWithHooks(se, path, candidate, reviewed, savePublicationHooks{})
}

func (a *App) commitReviewedSaveWithHook(se SessionEditor, path, candidate string, reviewed saveDestinationSnapshot, beforeDestinationMove func()) error {
	return a.commitReviewedSaveWithHooks(se, path, candidate, reviewed, savePublicationHooks{
		beforeDestinationMove: beforeDestinationMove,
	})
}

func (a *App) commitReviewedSaveWithHooks(se SessionEditor, path, candidate string, reviewed saveDestinationSnapshot, hooks savePublicationHooks) error {
	current, err := se.PrepareSave(path)
	if err != nil {
		return err
	}
	if current != candidate {
		return fmt.Errorf("reviewed candidate no longer matches the current document")
	}

	// Prepare the only format-specific post-publication projection before
	// touching the filesystem, so no fallible work remains after publication.
	derivedMBCHName := ""
	if e, ok := se.(*MBCHEditor); ok && strings.TrimSpace(e.character.Name) == "" {
		published, err := parsers.ParseMBCH(candidate)
		if err != nil {
			return fmt.Errorf("prepare published character state: %w", err)
		}
		derivedMBCHName = published.Name
	}

	result, err := publishEditorCandidateReviewedWithHooks(a.fileManager, path, candidate, reviewed, hooks)
	if err != nil {
		message := fmt.Sprintf("Failed to save file: %v", err)
		if result.Published {
			message = fmt.Sprintf("Candidate was published, but save finalization failed: %v", err)
		}
		switch e := se.(type) {
		case *MBCHEditor:
			e.lastError = message
		case *SABEditor:
			e.lastError = message
		case *VEHEditor:
			e.lastError = message
		case *SiegeEditor:
			e.lastError = message
		}
		return err
	}

	switch e := se.(type) {
	case *MBCHEditor:
		if derivedMBCHName != "" {
			e.character.Name = derivedMBCHName
			if e.nameEntry != nil {
				e.nameEntry.Text = derivedMBCHName
			}
		}
		e.lastError = ""
	case *SABEditor:
		e.docSource = candidate
		e.lastError = ""
	case *VEHEditor:
		e.docSource = candidate
		e.lastError = ""
	case *SiegeEditor:
		e.lastError = ""
	}
	se.SetCurrentPath(path)
	se.Session().SetBaseline(path, candidate)
	se.MarkClean()
	if notifier, ok := se.(interface{ fireSourceChanged() }); ok {
		notifier.fireSourceChanged()
	}
	return nil
}

// saveWithReview is the normal Save entry point.
func (a *App) saveWithReview(ed Editor, tab *container.TabItem, path string) {
	a.saveWithReviewCompletion(ed, tab, path, nil)
}

// saveWithReviewCompletion reports the terminal result after the async
// review/apply dialogs finish. Save As uses it to keep its destination
// chooser open on cancel/failure and hide it only after CommitSave.
func (a *App) saveWithReviewCompletion(ed Editor, tab *container.TabItem, path string, done func(error)) {
	finish := func(err error) {
		if done != nil {
			done(err)
		}
	}
	if ed == nil || path == "" {
		finish(fmt.Errorf("choose an editor and destination"))
		return
	}
	se, ok := ed.(SessionEditor)
	if !ok {
		err := fmt.Errorf("save review is unavailable for editor type %T; nothing was written", ed)
		ShowError(err, a.mainWindow)
		finish(err)
		return
	}

	// A source draft is user work separate from the form model. Invalid
	// text blocks saving and remains correctable. Valid text is applied
	// only after the user explicitly approves that step.
	if draft, hasDraft := a.sourceDrafts.Get(ed); hasDraft {
		if err := parseSourceForEditor(ed, draft.Text); err != nil {
			err = fmt.Errorf("the source draft is invalid and was kept for correction: %w", err)
			ShowError(fmt.Errorf("Cannot save %s: %v", filepath.Base(path), err), a.mainWindow)
			finish(err)
			return
		}
		dialog.ShowConfirm("Apply Source Edits",
			"Apply the pending source-panel text to the form before reviewing this save?",
			func(confirmed bool) {
				if !confirmed {
					err := fmt.Errorf("save cancelled; pending source edits were kept")
					a.updateStatus(err.Error())
					finish(err)
					return
				}
				current, ok := a.sourceDrafts.Get(ed)
				if !ok || current != draft {
					err := fmt.Errorf("source draft changed before it could be applied; review the latest text and save again")
					ShowError(err, a.mainWindow)
					finish(err)
					return
				}
				if err := se.ApplySourceText(draft.Text); err != nil {
					err = fmt.Errorf("cannot apply source draft: %w", err)
					ShowError(err, a.mainWindow)
					finish(err)
					return
				}
				a.beginSaveReview(ed, se, tab, path, &draft, done)
			}, a.mainWindow)
		return
	}
	a.beginSaveReview(ed, se, tab, path, nil, done)
}

func (a *App) beginSaveReview(ed Editor, se SessionEditor, tab *container.TabItem, path string, savedDraft *SourceDraft, done func(error)) {
	// Fold live form controls into the working model before invoking the
	// UI-free/pure candidate generator.
	se.CurrentSource()
	candidate, err := se.PrepareSave(path)
	if err != nil {
		err = fmt.Errorf("cannot save %s: %w", filepath.Base(path), err)
		ShowError(err, a.mainWindow)
		if done != nil {
			done(err)
		}
		return
	}
	reviewed, err := snapshotSaveDestination(path)
	if err != nil {
		err = fmt.Errorf("cannot review %s: %w", filepath.Base(path), err)
		ShowError(err, a.mainWindow)
		if done != nil {
			done(err)
		}
		return
	}

	if reviewed.Exists && reviewed.Bytes == candidate && !ed.IsDirty() && savedDraft == nil {
		a.updateStatus(fmt.Sprintf("%s already saved — no changes", filepath.Base(path)))
		if done != nil {
			done(nil)
		}
		return
	}

	warnings := ed.Validate()
	a.showSaveReviewDialogState(ed, tab, path, reviewed, candidate, warnings, savedDraft, done)
}

func reviewedSaveCommitFailure(base string, err error) error {
	if saveWasPublished(err) {
		return fmt.Errorf("Candidate was published to %s, but save finalization failed; editor remains dirty: %w", base, err)
	}
	return fmt.Errorf("Failed to save %s: %w", base, err)
}

func (a *App) showSaveReviewDialogState(ed Editor, tab *container.TabItem, path string, reviewed saveDestinationSnapshot, candidate string, warnings []string, savedDraft *SourceDraft, done func(error)) {
	base := filepath.Base(path)

	pane := func(title, text string, show bool) fyne.CanvasObject {
		if !show {
			return container.NewVBox(widget.NewLabelWithStyle(title, fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
				widget.NewLabel("(new file)"))
		}
		grid := widget.NewTextGrid()
		grid.SetText(text)
		return container.NewBorder(
			widget.NewLabelWithStyle(fmt.Sprintf("%s — %d bytes", title, len(text)),
				fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
			nil, nil, nil, container.NewScroll(grid))
	}

	header := widget.NewLabelWithStyle("Review changes before writing "+base, fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	header.Wrapping = fyne.TextWrapWord

	content := container.NewVBox(header)
	if len(warnings) > 0 {
		shown := warnings
		note := ""
		if len(shown) > 5 {
			shown = shown[:5]
			note = fmt.Sprintf("\n… and %d more (Validate shows all)", len(warnings)-5)
		}
		warnLabel := widget.NewLabelWithStyle(
			fmt.Sprintf("⚠ %d validation warning(s):\n  • %s%s", len(warnings), joinStrings(shown, "\n  • "), note),
			fyne.TextAlignLeading, fyne.TextStyle{})
		warnLabel.Wrapping = fyne.TextWrapWord
		content.Add(container.NewVBox(warnLabel, layout.NewSpacer()))
	}
	content.Add(container.NewHSplit(
		pane("Current file", reviewed.Bytes, reviewed.Exists),
		pane("Will be written", candidate, true),
	))

	dialog.NewCustomConfirm("Save Changes", "Save", "Cancel", content, func(confirmed bool) {
		finish := func(err error) {
			if done != nil {
				done(err)
			}
		}
		fail := func(err error) {
			ShowError(err, a.mainWindow)
			finish(err)
		}
		if !confirmed {
			err := fmt.Errorf("save cancelled; %s was not written", base)
			a.updateStatus(err.Error())
			finish(err)
			return
		}
		se, ok := ed.(SessionEditor)
		if !ok {
			fail(fmt.Errorf("Failed to save %s: editor is not session-aware", base))
			return
		}
		if err := verifySaveDestination(path, reviewed); err != nil {
			fail(fmt.Errorf("Failed to save %s: %v; review again before writing", base, err))
			return
		}
		currentDraft, hasDraft := a.sourceDrafts.Get(ed)
		if savedDraft == nil {
			if hasDraft {
				fail(fmt.Errorf("Failed to save %s: a source draft was created after review; nothing was written", base))
				return
			}
		} else if !hasDraft || currentDraft != *savedDraft {
			fail(fmt.Errorf("Failed to save %s: the source draft changed after review; nothing was written", base))
			return
		}
		// Fold any form edits made while the dialog was open, then
		// regenerate through the pure candidate path for equality.
		se.CurrentSource()
		currentCandidate, err := se.PrepareSave(path)
		if err != nil {
			fail(fmt.Errorf("Failed to save %s: candidate can no longer be prepared: %v", base, err))
			return
		}
		if currentCandidate != candidate {
			fail(fmt.Errorf("Failed to save %s: document changed after review; review the latest bytes before writing", base))
			return
		}
		if err := a.commitReviewedSave(se, path, candidate, reviewed); err != nil {
			fail(reviewedSaveCommitFailure(base, err))
			return
		}
		a.afterSuccessfulReviewedSave(ed, tab, path, savedDraft)
		finish(nil)
	}, a.mainWindow).Show()
}

// afterSuccessfulSave updates tab chrome and recovery state without
// consuming any unreviewed source draft.
func (a *App) afterSuccessfulSave(ed Editor, tab *container.TabItem, path string) {
	a.afterSuccessfulReviewedSave(ed, tab, path, nil)
}

func (a *App) afterSuccessfulReviewedSave(ed Editor, tab *container.TabItem, path string, savedDraft *SourceDraft) {
	if tab != nil {
		tab.Text = filepath.Base(path)
		if a.docTabs != nil {
			a.docTabs.Refresh()
		}
	}
	if a.fileManager != nil {
		a.fileManager.AddRecentFile(path)
	}
	if savedDraft != nil {
		a.sourceDrafts.DeleteIfMatch(ed, *savedDraft)
	}
	// Only a fully clean document loses its recovery snapshot. A newer
	// mirror draft that appeared around publication remains recoverable.
	if a.sourceDrafts.Has(ed) {
		a.captureRecoveryEntry(ed)
	} else {
		a.removeEditorRecovery(ed)
	}
	a.updateStatus("Saved " + filepath.Base(path))
}

// showSaveAsReviewDialog owns the destination chooser until the async
// source-apply/save-review/commit sequence reaches a terminal result.
// Cancel or failure re-enables the same chooser for retry; only a
// successful CommitSave hides it.
func (a *App) showSaveAsReviewDialog(ed Editor, tab *container.TabItem, defaultPath, extension string) {
	directory, name := filepath.Split(defaultPath)
	if directory == "" {
		directory, _ = os.UserHomeDir()
		if directory == "" {
			directory, _ = os.Getwd()
		}
	}
	if name == "" {
		name = "Untitled" + extension
	}

	directoryEntry := widget.NewEntry()
	directoryEntry.SetText(filepath.Clean(directory))
	nameEntry := widget.NewEntry()
	nameEntry.SetText(name)
	message := widget.NewLabel("")
	message.Wrapping = fyne.TextWrapWord

	browse := widget.NewButtonWithIcon("Choose folder", theme.FolderOpenIcon(), func() {
		picker := dialog.NewFolderOpen(func(uri fyne.ListableURI, err error) {
			if err != nil {
				message.SetText(err.Error())
				return
			}
			if uri != nil {
				directoryEntry.SetText(uri.Path())
			}
		}, a.mainWindow)
		if location, err := storage.ListerForURI(storage.NewFileURI(directoryEntry.Text)); err == nil {
			picker.SetLocation(location)
		}
		picker.Show()
	})

	var chooser *dialog.CustomDialog
	var saveButton *widget.Button
	launchReview := func(path string) {
		saveButton.Disable()
		message.SetText("Review the exact pending bytes to finish saving.")
		a.saveWithReviewCompletion(ed, tab, path, func(err error) {
			if err != nil {
				message.SetText(err.Error() + "\nAdjust the destination or source and press Save to retry.")
				saveButton.Enable()
				return
			}
			chooser.Hide()
		})
	}
	saveButton = widget.NewButtonWithIcon("Save", theme.DocumentSaveIcon(), func() {
		path, err := resolveSaveDestination(directoryEntry.Text, nameEntry.Text, extension)
		if err != nil {
			message.SetText(err.Error())
			return
		}
		info, err := os.Lstat(path)
		if err == nil {
			if !info.Mode().IsRegular() {
				message.SetText("Choose a regular file destination, not a directory or symbolic link.")
				return
			}
			dialog.ShowConfirm("Replace existing file?",
				fmt.Sprintf("Review and replace %s only after validation, backup, and atomic publication?", filepath.Base(path)),
				func(confirmed bool) {
					if confirmed {
						launchReview(path)
					}
				}, a.mainWindow)
			return
		}
		if !os.IsNotExist(err) {
			message.SetText(err.Error())
			return
		}
		launchReview(path)
	})
	saveButton.Importance = widget.HighImportance
	content := container.NewVBox(
		widget.NewLabel("The destination chooser stays open until the reviewed bytes are saved successfully."),
		widget.NewForm(
			widget.NewFormItem("Folder", container.NewBorder(nil, nil, nil, browse, directoryEntry)),
			widget.NewFormItem("File name", nameEntry),
		),
		message,
		container.NewHBox(saveButton),
	)
	chooser = dialog.NewCustom("Save As", "Cancel", content, a.mainWindow)
	chooser.Resize(fyne.NewSize(680, 280))
	chooser.Show()
	a.mainWindow.Canvas().Focus(nameEntry)
}

func joinStrings(parts []string, sep string) string {
	out := ""
	for i, p := range parts {
		if i > 0 {
			out += sep
		}
		out += p
	}
	return out
}
