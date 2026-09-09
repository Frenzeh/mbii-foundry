package main

// Shared per-document session state, owned by every slice editor
// (MBCH / SAB / VEH / Siege) and consulted by the source panel, the
// tab close/quit guards, the save-review dialog and crash recovery.
//
// State model:
//
//	cleanSnapshot — the dirty anchor. Updated ONLY by Reset (file
//	  load / fresh document) and MarkClean (successful save).
//	  DerivedDirty compares the working state against it, so a
//	  debounced edit commit or a failed save can never silently flip
//	  a document back to clean. Quit/save guards therefore see every
//	  unsaved byte until it really is on disk.
//	committed — the undo-history cursor (what the stacks believe is
//	  the current state). Deliberately separate from the dirty
//	  anchor: history movement must not change cleanliness.
//	undo/redo — snapshot stacks of canonical source strings.
//
// Editing scenarios that drive the design:
//   - edit → debounce fires → still dirty (cleanSnapshot untouched)
//   - undo back to the saved state → clean again
//   - save (MarkClean) → undo past the save → dirty again
//   - failed save → cleanSnapshot unmoved → still dirty
//   - new edit after undo → redo stack truncated

import (
	"sync"
	"time"

	"fyne.io/fyne/v2"
)

// SourceDraft is the retained raw source text a user typed into the
// source panel but has not applied back to the form yet. ParseErr
// keeps the live diagnostics so re-selecting the tab restores the
// exact validation state along with the text.
type SourceDraft struct {
	Text     string
	ParseErr string
	EditMode bool
}

// SourceDraftStore keeps one draft per editor document. It is shared
// by the primary source panel and every pop-out mirror so a draft
// survives pop-out/reattach and tab switches without being dropped.
type SourceDraftStore struct {
	mu     sync.Mutex
	drafts map[Editor]SourceDraft
}

func NewSourceDraftStore() *SourceDraftStore {
	return &SourceDraftStore{drafts: make(map[Editor]SourceDraft)}
}

func (s *SourceDraftStore) Set(ed Editor, d SourceDraft) {
	if s == nil || ed == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.drafts[ed] = d
}

func (s *SourceDraftStore) Get(ed Editor) (SourceDraft, bool) {
	if s == nil || ed == nil {
		return SourceDraft{}, false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	d, ok := s.drafts[ed]
	return d, ok
}

func (s *SourceDraftStore) Delete(ed Editor) {
	if s == nil || ed == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.drafts, ed)
}

// DeleteIfMatch consumes only the draft that was actually reviewed or
// applied. A newer edit from another source-panel mirror must survive.
func (s *SourceDraftStore) DeleteIfMatch(ed Editor, want SourceDraft) bool {
	if s == nil || ed == nil {
		return false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	got, ok := s.drafts[ed]
	if !ok || got != want {
		return false
	}
	delete(s.drafts, ed)
	return true
}

func (s *SourceDraftStore) Has(ed Editor) bool {
	_, ok := s.Get(ed)
	return ok
}

// sourceListeners multiplexes source-changed push notifications to
// every panel tracking an editor. SetOnSourceChanged keeps its legacy
// single-callback semantics; AddSourceListener lets mirrors subscribe
// without stealing the primary's notification.
type sourceListeners struct {
	mu     sync.Mutex
	nextID uint64
	fns    map[uint64]func()
}

// add registers fn and returns a remove closure.
func (l *sourceListeners) add(fn func()) func() {
	l.mu.Lock()
	if l.fns == nil {
		l.fns = make(map[uint64]func())
	}
	l.nextID++
	id := l.nextID
	l.fns[id] = fn
	l.mu.Unlock()

	var once sync.Once
	return func() {
		once.Do(func() {
			l.mu.Lock()
			delete(l.fns, id)
			l.mu.Unlock()
		})
	}
}

// reset replaces all listeners with a single fn (legacy semantics).
func (l *sourceListeners) reset(fn func()) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if fn == nil {
		l.fns = nil
		return
	}
	l.nextID++
	l.fns = map[uint64]func(){l.nextID: fn}
}

func (l *sourceListeners) fire() {
	l.mu.Lock()
	fns := make([]func(), 0, len(l.fns))
	for _, fn := range l.fns {
		fns = append(fns, fn)
	}
	l.mu.Unlock()
	for _, fn := range fns {
		fn()
	}
}

// DocumentSession owns baseline/draft/history state for one document.
// render snapshots the current working state into its canonical
// source form; restore swaps a previously-snapshotted source back
// into the working model. Both run on the UI thread.
type documentSnapshot struct {
	Source      string
	SelectedDef int
}

type DocumentSession struct {
	// render returns the canonical current source of the working model
	// (UI state folded in). Called on the UI thread.
	render func() string
	// restore swaps src into the working model in-memory. Must not
	// touch path, baseline or dirty bookkeeping.
	restore func(src string) error
	// onDirtyChange reports derived dirty transitions (tab title *).
	onDirtyChange func(bool)

	// Immutable saved baseline.
	Path          string
	OriginalBytes string // exact bytes on disk at open/save time
	HasOriginal   bool

	// Selected definition for multi-block documents.
	SelectedDef int

	// Recovery identity, assigned on first autosave snapshot.
	RecoveryID string

	// Dirty anchor — see the package comment. Only Reset/MarkClean
	// write cleanSnapshot.
	cleanSnapshot string
	hasClean      bool

	// History cursor + stacks include the selected definition so an
	// undoable definition switch restores both bytes and form target.
	undo         []documentSnapshot
	redo         []documentSnapshot
	committed    string
	committedDef int

	// Debounce plumbing for form-level edits. Both fields are only
	// mutated on the UI thread (the timer callback hops through
	// fyne.Do first), so the single-shot clear is atomic with respect
	// to flushPending.
	started   bool
	timer     *time.Timer
	pendingFn func()

	maxHistory int
}

func NewDocumentSession(render func() string, restore func(src string) error) *DocumentSession {
	return &DocumentSession{
		render:     render,
		restore:    restore,
		maxHistory: 100,
	}
}

// SetOnDirtyChange wires the derived-dirty reporter.
func (s *DocumentSession) SetOnDirtyChange(f func(bool)) { s.onDirtyChange = f }

// noteUserEdit records a form-level user edit. Snapshots are
// coalesced: the first keystroke after a quiet period schedules a
// deferred commit of the pre-edit state, so a burst of keystrokes in
// one field becomes a single undo step. The first edit after a clean
// anchor immediately marks the document dirty.
func (s *DocumentSession) noteUserEdit() {
	s.init()
	if s.timer != nil || s.pendingFn != nil {
		return // coalesce into the pending commit
	}
	s.pendingFn = s.commitPending
	s.timer = time.AfterFunc(600*time.Millisecond, func() {
		// Background goroutine: hop to the UI thread before touching
		// widget-backed render state. Clear both handles atomically
		// inside the callback so a racing flushPending can't run it
		// twice.
		fyne.Do(func() {
			s.timer = nil
			fn := s.pendingFn
			s.pendingFn = nil
			if fn != nil {
				fn()
			}
		})
	})
	s.syncDirty() // first keystroke marks the tab dirty immediately
}

// commitPending pushes the history cursor if the working state
// drifted from it, invalidating redo (a new edit after undo is the
// standard redo-stack truncation point). Never touches cleanSnapshot.
func (s *DocumentSession) commitPending() {
	cur := s.render()
	if cur != s.committed || s.SelectedDef != s.committedDef {
		s.appendUndo(documentSnapshot{Source: s.committed, SelectedDef: s.committedDef})
		s.redo = nil
	}
	s.committed = cur
	s.committedDef = s.SelectedDef
}

func (s *DocumentSession) appendUndo(snapshot documentSnapshot) {
	s.undo = append(s.undo, snapshot)
	if len(s.undo) > s.maxHistory {
		s.undo = s.undo[len(s.undo)-s.maxHistory:]
	}
}

// flushPending force-runs any scheduled commit exactly once. Call
// before discrete operations (undo/redo/apply/import/definition
// switch) and on save. Safe to call when nothing is pending.
func (s *DocumentSession) flushPending() {
	if s.timer != nil {
		s.timer.Stop()
		s.timer = nil
	}
	fn := s.pendingFn
	s.pendingFn = nil
	if fn != nil {
		fn()
	}
}

// beginDiscreteChange snapshots the current state ahead of an atomic
// operation (source apply, JSON import, definition switch, template
// load). Any uncommitted form edits are committed first so the stack
// keeps both steps at the right granularity.
func (s *DocumentSession) beginDiscreteChange() {
	s.init()
	s.flushPending()
	cur := documentSnapshot{Source: s.render(), SelectedDef: s.SelectedDef}
	if cur.Source != s.committed || cur.SelectedDef != s.committedDef {
		s.appendUndo(documentSnapshot{Source: s.committed, SelectedDef: s.committedDef})
	}
	if len(s.undo) == 0 || s.undo[len(s.undo)-1] != cur {
		s.appendUndo(cur)
	}
	s.redo = nil
}

// beginDiscreteChangeFrom records a source snapshot already generated by
// the caller. Definition switches use this to avoid a second UI fold
// overwriting direct/model-level edits between generation and selection.
func (s *DocumentSession) beginDiscreteChangeFrom(source string) {
	s.init()
	s.flushPending()
	cur := documentSnapshot{Source: source, SelectedDef: s.SelectedDef}
	if cur.Source != s.committed || cur.SelectedDef != s.committedDef {
		s.appendUndo(documentSnapshot{Source: s.committed, SelectedDef: s.committedDef})
	}
	if len(s.undo) == 0 || s.undo[len(s.undo)-1] != cur {
		s.appendUndo(cur)
	}
	s.redo = nil
}

// endDiscreteChange re-anchors the history cursor after an atomic
// operation completed and re-derives dirty.
func (s *DocumentSession) endDiscreteChange() {
	s.committed = s.render()
	s.committedDef = s.SelectedDef
	s.syncDirty()
}

// Undo rolls the working state back one step. Returns true when a
// step was restored. On a failed restore the target stays on the
// stack and the working state is untouched.
func (s *DocumentSession) Undo() bool {
	s.init()
	s.flushPending()
	if len(s.undo) == 0 {
		return false
	}

	cur := documentSnapshot{Source: s.render(), SelectedDef: s.SelectedDef}
	prev := s.undo[len(s.undo)-1]
	oldSelection := s.SelectedDef
	s.SelectedDef = prev.SelectedDef
	if err := s.restore(prev.Source); err != nil {
		s.SelectedDef = oldSelection
		return false
	}

	s.redo = append(s.redo, cur)
	s.undo = s.undo[:len(s.undo)-1]
	s.committed = prev.Source
	s.committedDef = prev.SelectedDef
	s.syncDirty()
	return true
}

// Redo reapplies the most recently undone state. On a failed restore
// the target stays on the redo stack and the working state is untouched.
func (s *DocumentSession) Redo() bool {
	s.init()
	s.flushPending()
	if len(s.redo) == 0 {
		return false
	}

	cur := documentSnapshot{Source: s.render(), SelectedDef: s.SelectedDef}
	next := s.redo[len(s.redo)-1]
	oldSelection := s.SelectedDef
	s.SelectedDef = next.SelectedDef
	if err := s.restore(next.Source); err != nil {
		s.SelectedDef = oldSelection
		return false
	}

	s.appendUndo(cur)
	s.redo = s.redo[:len(s.redo)-1]
	s.committed = next.Source
	s.committedDef = next.SelectedDef
	s.syncDirty()
	return true
}

func (s *DocumentSession) CanUndo() bool {
	if len(s.undo) > 0 {
		return true
	}
	s.init()
	s.flushPending()
	return s.render() != s.committed || s.SelectedDef != s.committedDef
}

func (s *DocumentSession) CanRedo() bool { return len(s.redo) > 0 }

// MarkClean re-anchors BOTH the dirty anchor and the history cursor
// to the current state. Called after a successful save and on loads
// that complete through Reset instead.
func (s *DocumentSession) MarkClean() {
	s.init()
	s.flushPending()
	s.cleanSnapshot = s.render()
	s.hasClean = true
	s.committed = s.cleanSnapshot
	s.committedDef = s.SelectedDef
	s.syncDirty()
}

// Reset clears history and anchors the dirty anchor at the current
// state (fresh document or completed file load).
func (s *DocumentSession) Reset() {
	s.init()
	s.flushPending()
	s.undo = nil
	s.redo = nil
	s.cleanSnapshot = s.render()
	s.hasClean = true
	s.committed = s.cleanSnapshot
	s.committedDef = s.SelectedDef
	s.syncDirty()
}

func (s *DocumentSession) SetBaseline(path, original string) {
	s.Path = path
	s.OriginalBytes = original
	s.HasOriginal = true
}

// ClearBaseline detaches a recovered document from an original that is
// missing or changed. The working dirty anchor is intentionally left
// untouched; only a successful later save may establish a new baseline.
func (s *DocumentSession) ClearBaseline() {
	s.Path = ""
	s.OriginalBytes = ""
	s.HasOriginal = false
}

// DerivedDirty compares the working state against the clean anchor —
// never the history cursor. Untitled documents start anchored at
// their initial state, so the first edit marks them dirty.
func (s *DocumentSession) DerivedDirty() bool {
	s.init()
	return s.render() != s.cleanSnapshot
}

// syncDirty reports derived-dirty transitions to the editor.
func (s *DocumentSession) syncDirty() {
	if s.onDirtyChange != nil {
		s.onDirtyChange(s.DerivedDirty())
	}
}

// init lazily anchors both the dirty anchor and the history cursor at
// first observation so editors constructed before their UI exists
// still behave.
func (s *DocumentSession) init() {
	if s.started {
		return
	}
	s.started = true
	s.cleanSnapshot = s.render()
	s.hasClean = true
	s.committed = s.cleanSnapshot
	s.committedDef = s.SelectedDef
}

// SessionEditor is the full session-aware editor contract. All four
// slice editors implement it; App wiring, the source panel, the save
// review dialog and crash recovery type-assert to it.
type SessionEditor interface {
	Editor

	// Session returns the shared per-document state.
	Session() *DocumentSession

	// CurrentSource returns the canonical source of the working model
	// (UI state folded in).
	CurrentSource() string

	// ApplySourceText parses src and swaps it into the working model
	// in-memory — no temp files, no LoadFile. A failed parse leaves
	// the document untouched so the text stays correctable. Undoable.
	ApplySourceText(src string) error

	// Definition navigation for multi-block documents. Single-block
	// formats report one definition.
	DefinitionNames() []string
	SelectedDefinition() int
	SelectDefinition(index int) error

	// Save review: PrepareSave returns the exact candidate bytes that
	// would be written, without mutating any state. CommitSave writes
	// those bytes after user approval and refreshes baseline/path.
	PrepareSave(path string) (string, error)
	CommitSave(path string, candidate string) error

	// Undo/redo across form edits, source applies and JSON imports.
	Undo() bool
	Redo() bool
	CanUndo() bool
	CanRedo() bool
}

// SourceListenerProvider lets panels and mirrors subscribe to
// source-changed push notifications without stealing each other's
// callbacks (SetOnSourceChanged keeps single-callback semantics).
type SourceListenerProvider interface {
	AddSourceListener(fn func()) (remove func())
}
