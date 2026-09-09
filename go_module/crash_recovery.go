package main

// Crash recovery: periodic in-memory snapshots of open documents,
// persisted into an isolated directory under the app config dir. If
// Foundry crashes (or the process dies mid-edit), the next launch
// offers to restore the recovered working state.
//
// Safety rules:
//   - Snapshots are written atomically (safeio) and never touch the
//     original document files — restoring opens an editor with the
//     recovered working state; nothing is written back to any path
//     without an explicit user save.
//   - Untitled documents recover just like path-bound ones; their
import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"

	"github.com/Frenzeh/mbii-foundry/safeio"
)

// RecoveryEntry is one recovered document snapshot.
type RecoveryEntry struct {
	ID            string    `json:"id"`
	Title         string    `json:"title"`
	Format        string    `json:"format"` // ".mbch", ".sab", ".veh", ".siege"
	OriginalPath  string    `json:"original_path,omitempty"`
	Baseline      string    `json:"baseline,omitempty"` // bytes on disk at open/save time
	HasBaseline   bool      `json:"has_baseline"`
	Working       string    `json:"working"` // canonical current working state
	Draft         string    `json:"draft,omitempty"`
	DraftParseErr string    `json:"draft_parse_error,omitempty"`
	DraftEditMode bool      `json:"draft_edit_mode"`
	HasDraft      bool      `json:"has_draft"`
	SelectedDef   int       `json:"selected_def"`
	SavedAt       time.Time `json:"saved_at"`
}

const (
	recoveryIDHexLen     = 16
	maxRecoveryEntries   = 128
	maxRecoveryFileBytes = 8 << 20
	maxRecoverySelection = 4096
)

// RecoveryStore persists RecoveryEntries as individual JSON files.
// A nil/empty dir disables the store entirely (config dir unavailable).
type RecoveryStore struct {
	dir string
}

func NewRecoveryStore(dir string) *RecoveryStore {
	return &RecoveryStore{dir: dir}
}

// Available reports whether snapshots can be persisted.
func (r *RecoveryStore) Available() bool { return r != nil && r.dir != "" }

func validRecoveryID(id string) bool {
	if len(id) != recoveryIDHexLen || id != strings.ToLower(id) {
		return false
	}
	_, err := hex.DecodeString(id)
	return err == nil
}

func validRecoveryFormat(format string) bool {
	switch format {
	case ".mbch", ".sab", ".veh", ".siege":
		return true
	default:
		return false
	}
}

func (r *RecoveryStore) entryPath(id string) (string, error) {
	if !validRecoveryID(id) {
		return "", fmt.Errorf("invalid recovery ID %q", id)
	}
	return filepath.Join(r.dir, id+".json"), nil
}

func (r *RecoveryStore) ensureDir() error {
	if err := os.MkdirAll(r.dir, 0700); err != nil {
		return fmt.Errorf("recovery dir: %w", err)
	}
	info, err := os.Lstat(r.dir)
	if err != nil {
		return fmt.Errorf("recovery dir: %w", err)
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return fmt.Errorf("recovery dir is not a real directory")
	}
	return nil
}

// Save atomically writes one bounded entry under a validated fixed-width
// lowercase-hex filename.
func (r *RecoveryStore) Save(entry *RecoveryEntry) error {
	if !r.Available() {
		return nil
	}
	if entry == nil {
		return fmt.Errorf("nil recovery entry")
	}
	if entry.ID == "" {
		entry.ID = newRecoveryID()
	}
	if !validRecoveryID(entry.ID) {
		return fmt.Errorf("invalid recovery ID %q", entry.ID)
	}
	if !validRecoveryFormat(entry.Format) {
		return fmt.Errorf("unsupported recovery format %q", entry.Format)
	}
	if entry.SelectedDef < 0 || entry.SelectedDef > maxRecoverySelection {
		return fmt.Errorf("invalid selected definition %d", entry.SelectedDef)
	}
	if err := r.ensureDir(); err != nil {
		return err
	}
	data, err := json.MarshalIndent(entry, "", "  ")
	if err != nil {
		return err
	}
	if len(data) > maxRecoveryFileBytes {
		return fmt.Errorf("recovery snapshot exceeds %d-byte limit", maxRecoveryFileBytes)
	}
	path, err := r.entryPath(entry.ID)
	if err != nil {
		return err
	}
	return safeio.WriteFile(path, data, 0600)
}

type recoveryWarnings struct {
	problems []string
}

func (w *recoveryWarnings) add(format string, args ...interface{}) {
	w.problems = append(w.problems, fmt.Sprintf(format, args...))
}

func (w *recoveryWarnings) err() error {
	if len(w.problems) == 0 {
		return nil
	}
	return fmt.Errorf("%d recovery warning(s): %s", len(w.problems), strings.Join(w.problems, "; "))
}

// List returns valid entries sorted by save time. Unsafe, corrupt, or
// oversized files are skipped and reported while other work remains
// recoverable.
func (r *RecoveryStore) List() ([]*RecoveryEntry, error) {
	if !r.Available() {
		return nil, nil
	}
	info, err := os.Lstat(r.dir)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("inspect recovery dir: %w", err)
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return nil, fmt.Errorf("recovery dir is not a real directory")
	}
	dir, err := os.Open(r.dir)
	if err != nil {
		return nil, fmt.Errorf("open recovery dir: %w", err)
	}
	defer dir.Close()
	items, err := dir.ReadDir(maxRecoveryEntries + 1)
	if err != nil && err != io.EOF {
		return nil, fmt.Errorf("read recovery dir: %w", err)
	}

	warnings := &recoveryWarnings{}
	if len(items) > maxRecoveryEntries {
		warnings.add("directory entry limit %d reached; excess entries were not read", maxRecoveryEntries)
		items = items[:maxRecoveryEntries]
	}
	entries := make([]*RecoveryEntry, 0, len(items))
	for _, item := range items {
		name := item.Name()
		if filepath.Ext(name) != ".json" {
			continue
		}
		fileID := strings.TrimSuffix(name, ".json")
		if !validRecoveryID(fileID) {
			warnings.add("%s has an invalid filename", name)
			continue
		}
		path := filepath.Join(r.dir, name)
		lstat, err := os.Lstat(path)
		if err != nil {
			warnings.add("%s cannot be inspected: %v", name, err)
			continue
		}
		if lstat.Mode()&os.ModeSymlink != 0 || !lstat.Mode().IsRegular() {
			warnings.add("%s is not a regular file", name)
			continue
		}
		if lstat.Size() > maxRecoveryFileBytes {
			warnings.add("%s exceeds the %d-byte limit", name, maxRecoveryFileBytes)
			continue
		}
		file, err := os.Open(path)
		if err != nil {
			warnings.add("%s cannot be opened: %v", name, err)
			continue
		}
		opened, statErr := file.Stat()
		if statErr != nil || !opened.Mode().IsRegular() || !os.SameFile(lstat, opened) {
			file.Close()
			warnings.add("%s changed or became unsafe while opening", name)
			continue
		}
		data, readErr := io.ReadAll(io.LimitReader(file, maxRecoveryFileBytes+1))
		closeErr := file.Close()
		if readErr != nil {
			warnings.add("%s cannot be read: %v", name, readErr)
			continue
		}
		if closeErr != nil {
			warnings.add("%s cannot be closed: %v", name, closeErr)
			continue
		}
		if len(data) > maxRecoveryFileBytes {
			warnings.add("%s exceeds the %d-byte limit", name, maxRecoveryFileBytes)
			continue
		}
		var entry RecoveryEntry
		if err := json.Unmarshal(data, &entry); err != nil {
			warnings.add("%s is corrupt: %v", name, err)
			continue
		}
		if entry.ID != fileID || !validRecoveryID(entry.ID) {
			warnings.add("%s contains a mismatched or invalid ID", name)
			continue
		}
		if !validRecoveryFormat(entry.Format) {
			warnings.add("%s contains unsupported format %q", name, entry.Format)
			continue
		}
		if entry.SelectedDef < 0 || entry.SelectedDef > maxRecoverySelection {
			warnings.add("%s contains invalid definition selection %d", name, entry.SelectedDef)
			continue
		}
		entries = append(entries, &entry)
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].SavedAt.Before(entries[j].SavedAt)
	})
	return entries, warnings.err()
}

// Remove drops one entry (document saved cleanly or discarded).
func (r *RecoveryStore) Remove(id string) error {
	if !r.Available() || id == "" {
		return nil
	}
	path, err := r.entryPath(id)
	if err != nil {
		return err
	}
	err = os.Remove(path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// Clear drops recovery JSON entries without following symlinks and
// without materializing an unbounded directory listing.
func (r *RecoveryStore) Clear() error {
	if !r.Available() {
		return nil
	}
	info, err := os.Lstat(r.dir)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return fmt.Errorf("recovery dir is not a real directory")
	}
	dir, err := os.Open(r.dir)
	if err != nil {
		return err
	}
	defer dir.Close()
	var problems []string
	for {
		items, readErr := dir.ReadDir(maxRecoveryEntries)
		for _, item := range items {
			if filepath.Ext(item.Name()) != ".json" {
				continue
			}
			if err := os.Remove(filepath.Join(r.dir, item.Name())); err != nil && !os.IsNotExist(err) {
				problems = append(problems, fmt.Sprintf("%s: %v", item.Name(), err))
			}
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			problems = append(problems, readErr.Error())
			break
		}
	}
	if len(problems) > 0 {
		return fmt.Errorf("clear recovery data: %s", strings.Join(problems, "; "))
	}
	return nil
}

func newRecoveryID() string {
	buf := make([]byte, recoveryIDHexLen/2)
	if _, err := rand.Read(buf); err == nil {
		return hex.EncodeToString(buf)
	}
	// Preserve the fixed validated representation even if the platform
	// CSPRNG is temporarily unavailable.
	sum := sha256.Sum256([]byte(fmt.Sprintf("%d-%p", time.Now().UnixNano(), &buf)))
	return hex.EncodeToString(sum[:recoveryIDHexLen/2])
}

// captureRecoveryEntry snapshots one open editor into the store.
// Editors keep a stable RecoveryID in their session so repeated
// autosaves overwrite the same file instead of accumulating.
func (a *App) captureRecoveryEntry(ed Editor) {
	if !a.recovery.Available() || ed == nil {
		return
	}
	se, ok := ed.(SessionEditor)
	if !ok {
		return
	}
	draft, hasDraft := a.sourceDrafts.Get(ed)
	if !ed.IsDirty() && !hasDraft {
		return
	}
	sess := se.Session()
	if !validRecoveryID(sess.RecoveryID) {
		sess.RecoveryID = newRecoveryID()
	}
	entry := &RecoveryEntry{
		ID:           sess.RecoveryID,
		Format:       editorFormatFor(ed),
		OriginalPath: ed.GetCurrentPath(),
		Baseline:     sess.OriginalBytes,
		HasBaseline:  sess.HasOriginal,
		Working:      se.CurrentSource(),
		SelectedDef:  se.SelectedDefinition(),
		SavedAt:      time.Now(),
	}
	switch entry.Format {
	case ".mbch":
		entry.Title = "Untitled character"
	default:
		entry.Title = "Untitled document"
	}
	if p := ed.GetCurrentPath(); p != "" {
		entry.Title = filepath.Base(p)
	}
	if hasDraft {
		entry.Draft = draft.Text
		entry.DraftParseErr = draft.ParseErr
		entry.DraftEditMode = draft.EditMode
		entry.HasDraft = true
	}
	if err := a.recovery.Save(entry); err != nil {
		LogError("recovery snapshot failed for %s: %v", entry.Title, err)
	}
}

// autosaveRecoveryPass snapshots every dirty editor. Runs on the UI
// thread (rendering touches widgets).
func (a *App) autosaveRecoveryPass() {
	for _, ed := range a.editors {
		if ed == nil {
			continue
		}
		draftPending := a.sourceDrafts.Has(ed)
		if ed.IsDirty() || draftPending {
			a.captureRecoveryEntry(ed)
		}
	}
	for ed := range a.detachedEditors {
		if ed == nil {
			continue
		}
		draftPending := a.sourceDrafts.Has(ed)
		if ed.IsDirty() || draftPending {
			a.captureRecoveryEntry(ed)
		}
	}
}

func (a *App) startRecoveryAutosave() {
	if !a.recovery.Available() {
		return
	}
	go func() {
		ticker := time.NewTicker(15 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			fyne.Do(a.autosaveRecoveryPass)
		}
	}()
}

// removeEditorRecovery drops the snapshot for a closed/saved editor.
func (a *App) removeEditorRecovery(ed Editor) {
	if !a.recovery.Available() || ed == nil {
		return
	}
	if se, ok := ed.(SessionEditor); ok {
		if err := a.recovery.Remove(se.Session().RecoveryID); err != nil {
			LogError("recovery cleanup failed: %v", err)
		}
	}
}

// offerSessionRecovery checks for leftover snapshots from a previous
// run and offers to restore them. Declining clears the store.
func (a *App) offerSessionRecovery() {
	entries, listWarning := a.recovery.List()
	if len(entries) == 0 {
		if listWarning != nil {
			a.surfaceRecoveryError(fmt.Errorf("No safe recovery snapshots could be loaded: %w", listWarning))
		}
		return
	}
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		when := entry.SavedAt.Format("15:04")
		names = append(names, fmt.Sprintf("  • %s (%s, saved %s)", entry.Title, entry.Format, when))
	}
	msg := fmt.Sprintf("Found unsaved work from a previous session:\n\n%s\n\nRestore %d document(s)?",
		strings.Join(names, "\n"), len(entries))
	if listWarning != nil {
		LogError("recovery scan warning: %v", listWarning)
		msg += "\n\nSome recovery files were skipped for safety:\n" + listWarning.Error()
	}
	dialog.ShowConfirm("Recover Unsaved Work", msg, func(restore bool) {
		if !restore {
			if err := a.recovery.Clear(); err != nil {
				a.surfaceRecoveryError(fmt.Errorf("Recovered session data could not be fully discarded: %w", err))
				return
			}
			a.updateStatus("Discarded recovered session data")
			return
		}
		restored := 0
		var failures []string
		for _, entry := range entries {
			if err := a.restoreRecoveredEntryError(entry); err != nil {
				failures = append(failures, fmt.Sprintf("%s: %v", entry.Title, err))
				continue
			}
			restored++
		}
		if len(failures) > 0 {
			a.surfaceRecoveryError(fmt.Errorf("%d recovery item(s) remain unresolved: %s",
				len(failures), strings.Join(failures, "; ")))
		}
		a.updateStatus(fmt.Sprintf("Restored %d recovered document(s); %d unresolved", restored, len(failures)))
	}, a.mainWindow)
}

func (a *App) surfaceRecoveryError(err error) {
	if err == nil {
		return
	}
	LogError("%v", err)
	a.updateStatus(err.Error())
	if a.mainWindow != nil {
		dialog.ShowError(err, a.mainWindow)
	}
}

// restoreRecoveredEntry reopens one snapshot and reports success. The
// error-bearing helper is used by the recovery dialog so unresolved work
// is surfaced rather than counted as restored.
func (a *App) restoreRecoveredEntry(entry *RecoveryEntry) bool {
	if err := a.restoreRecoveredEntryError(entry); err != nil {
		LogError("recovery restore failed: %v", err)
		return false
	}
	return true
}

func (a *App) restoreRecoveredEntryError(entry *RecoveryEntry) error {
	if entry == nil || !validRecoveryID(entry.ID) {
		return fmt.Errorf("invalid recovery entry identity")
	}
	ed := newEditorForFormat(a, entry.Format)
	if ed == nil {
		return fmt.Errorf("unsupported recovery format %q", entry.Format)
	}
	se, ok := ed.(SessionEditor)
	if !ok {
		return fmt.Errorf("editor for %s is not session-aware", entry.Format)
	}

	baselineIntact := false
	if entry.HasBaseline && entry.OriginalPath != "" {
		original, err := snapshotSaveDestination(entry.OriginalPath)
		if err == nil && original.Exists && original.Bytes == entry.Baseline {
			if err := ed.LoadFile(entry.OriginalPath); err == nil {
				baselineIntact = true
			}
		}
	}
	if !baselineIntact {
		// Missing, changed, symlinked, or unreadable originals are always
		// detached. A later Save therefore routes through Save As and can
		// never overwrite the recorded original path.
		ed.SetCurrentPath("")
		se.Session().ClearBaseline()
	}

	if err := se.ApplySourceText(entry.Working); err != nil {
		return fmt.Errorf("apply recovered working source: %w", err)
	}
	if err := se.SelectDefinition(entry.SelectedDef); err != nil {
		return fmt.Errorf("restore definition selection %d: %w", entry.SelectedDef, err)
	}

	se.Session().RecoveryID = entry.ID
	if a.sourceDrafts == nil {
		a.sourceDrafts = NewSourceDraftStore()
	}
	if entry.HasDraft {
		a.sourceDrafts.Set(ed, SourceDraft{
			Text:     entry.Draft,
			ParseErr: entry.DraftParseErr,
			EditMode: entry.DraftEditMode,
		})
	}

	title := entry.Title
	if !baselineIntact {
		title = "Recovered " + title
	}
	a.createNewFile(title, ed)
	if tab := a.tabForEditor(ed); tab != nil {
		tab.Text = title
		if a.docTabs != nil {
			a.docTabs.Refresh()
		}
	}
	return nil
}

// newEditorForFormat builds a bare editor for a recovered document.
func newEditorForFormat(a *App, format string) Editor {
	switch format {
	case ".mbch":
		return NewMBCHEditor(a)
	case ".sab":
		return NewSABEditor(a)
	case ".veh":
		return NewVEHEditor(a)
	case ".siege":
		return NewSiegeEditor(a)
	}
	return nil
}

func editorFormatFor(ed Editor) string {
	switch ed.(type) {
	case *MBCHEditor:
		return ".mbch"
	case *SABEditor:
		return ".sab"
	case *VEHEditor:
		return ".veh"
	case *SiegeEditor:
		return ".siege"
	}
	return ""
}

func (a *App) tabForEditor(ed Editor) *container.TabItem {
	for tab, e := range a.editors {
		if e == ed {
			return tab
		}
	}
	return nil
}
