package main

import (
	"fmt"
	"testing"
	"time"
)

type sessionTestTimer struct {
	due  time.Duration
	fire func()
}

func (t *sessionTestTimer) Stop() bool {
	active := t.fire != nil
	t.fire = nil
	return active
}

// Fyne's headless driver executes fyne.Do inline. Own session timers here so
// a long widget operation cannot start a concurrent history/render callback.
// All session delays are the same, so scheduled timers remain in due order.
type sessionTestClock struct {
	now    time.Duration
	timers []*sessionTestTimer
}

func useSessionTestClock(t testing.TB) *sessionTestClock {
	t.Helper()
	clock := &sessionTestClock{}
	previous := scheduleSessionCommit
	scheduleSessionCommit = clock.schedule
	t.Cleanup(func() {
		for len(clock.timers) > 0 {
			clock.advance(clock.timers[0].due - clock.now)
		}
		scheduleSessionCommit = previous
	})
	return clock
}

func (c *sessionTestClock) schedule(delay time.Duration, commit func()) sessionCommitTimer {
	timer := &sessionTestTimer{due: c.now + delay, fire: commit}
	c.timers = append(c.timers, timer)
	return timer
}

func (c *sessionTestClock) advance(elapsed time.Duration) {
	target := c.now + elapsed
	for len(c.timers) > 0 && c.timers[0].due <= target {
		timer := c.timers[0]
		c.timers = c.timers[1:]
		c.now = timer.due
		fire := timer.fire
		timer.fire = nil
		if fire != nil {
			fire()
		}
	}
	c.now = target
}

func TestSessionTypingCoalescesAtScheduledBoundary(t *testing.T) {
	clock := useSessionTestClock(t)
	value := "saved"
	s := NewDocumentSession(
		func() string { return value },
		func(source string) error { value = source; return nil },
	)
	s.MarkClean()
	value = "first keystroke"
	s.noteUserEdit()
	if !s.DerivedDirty() {
		t.Fatal("pending typing must be dirty before its history commit")
	}
	clock.advance(300 * time.Millisecond)
	value = "first burst"
	s.noteUserEdit()
	clock.advance(299 * time.Millisecond)
	if s.committed != "saved" || len(s.undo) != 0 {
		t.Fatal("typing committed before the 600ms coalescing boundary")
	}
	clock.advance(time.Millisecond)
	if s.committed != "first burst" || !s.DerivedDirty() {
		t.Fatal("the first burst was not committed without changing its dirty anchor")
	}

	value = "second burst"
	s.noteUserEdit()
	clock.advance(600 * time.Millisecond)
	for _, want := range []string{"first burst", "saved"} {
		if !s.Undo() || value != want {
			t.Fatalf("Undo = %q, want %q", value, want)
		}
	}
	if s.Undo() || s.DerivedDirty() {
		t.Fatal("coalescing left extra history or failed to restore the saved anchor")
	}
	for _, want := range []string{"first burst", "second burst"} {
		if !s.Redo() || value != want {
			t.Fatalf("Redo = %q, want %q", value, want)
		}
	}
}

func TestSessionQueuedCommitCannotConsumeNewEdit(t *testing.T) {
	for _, boundary := range []string{"undo", "redo", "save", "reset", "discrete"} {
		t.Run(boundary, func(t *testing.T) {
			clock := useSessionTestClock(t)
			value := "saved"
			s := NewDocumentSession(
				func() string { return value },
				func(source string) error { value = source; return nil },
			)
			s.MarkClean()
			value = "first"
			s.noteUserEdit()
			clock.advance(600 * time.Millisecond)
			value = "pending"
			s.noteUserEdit()
			// A timer may have queued its UI callback before Stop. Retain
			// that callback to run after the history boundary and a new edit.
			queued := clock.timers[0].fire
			switch boundary {
			case "undo":
				if !s.Undo() || value != "first" {
					t.Fatal("Undo did not flush pending typing as a distinct step")
				}
			case "redo":
				if !s.Undo() || !s.Redo() || value != "pending" {
					t.Fatal("Redo did not restore the flushed pending typing")
				}
			case "save":
				s.MarkClean()
			case "reset":
				s.Reset()
			case "discrete":
				s.beginDiscreteChange()
				value = "applied"
				s.endDiscreteChange()
			}
			want := value
			value = "new typing"
			s.noteUserEdit()
			queued()
			clock.advance(599 * time.Millisecond)
			if s.committed != want {
				t.Fatal("an obsolete callback consumed the newer typing burst")
			}
			clock.advance(time.Millisecond)
			if !s.Undo() || value != want {
				t.Fatalf("Undo crossed the %s boundary: got %q, want %q", boundary, value, want)
			}
			if (boundary == "save" || boundary == "reset") && s.DerivedDirty() {
				t.Fatal("Undo to the saved/reset boundary must be clean")
			}
			if !s.Redo() || value != "new typing" || !s.DerivedDirty() {
				t.Fatal("Redo did not restore the new burst independently of the old callback")
			}
		})
	}
}

func TestSessionRetainsItsSchedulerAcrossFixtures(t *testing.T) {
	outer := useSessionTestClock(t)
	value := "saved"
	s := NewDocumentSession(
		func() string { return value },
		func(source string) error { value = source; return nil },
	)
	s.MarkClean()
	t.Run("nested clock", func(t *testing.T) {
		inner := useSessionTestClock(t)
		value = "outer edit"
		s.noteUserEdit()
		inner.advance(time.Second)
		if s.committed != "saved" {
			t.Fatal("an existing session adopted a different fixture's clock")
		}
	})
	outer.advance(600 * time.Millisecond)
	if !s.Undo() || value != "saved" || !s.Redo() || value != "outer edit" {
		t.Fatal("the owning clock did not retain the session's history boundary")
	}
}

func TestSessionEngineBasics(t *testing.T) {
	var store struct{ v string }
	store.v = "one"
	s := NewDocumentSession(
		func() string { return store.v },
		func(src string) error { store.v = src; return nil },
	)

	dirtyEvents := 0
	s.SetOnDirtyChange(func(bool) { dirtyEvents++ })

	// Fresh session anchored at initial state: clean.
	s.MarkClean()
	if s.DerivedDirty() {
		t.Fatal("fresh session must be clean")
	}

	// A form edit, flushed: one undo step to the prior state.
	store.v = "one!"
	s.flushPending()
	s.commitPending()
	if !s.DerivedDirty() {
		t.Fatal("edited state must be dirty")
	}
	if !s.CanUndo() {
		t.Fatal("committed edit must be undoable")
	}
	if !s.Undo() {
		t.Fatal("undo should restore")
	}
	if store.v != "one" {
		t.Fatalf("undo should restore prior state, got %q", store.v)
	}
	if s.DerivedDirty() {
		t.Fatal("undo back to committed state must be clean")
	}
	if !s.CanRedo() {
		t.Fatal("redo must be available after undo")
	}
	if !s.Redo() {
		t.Fatal("redo should apply")
	}
	if store.v != "one!" {
		t.Fatalf("redo should reapply edit, got %q", store.v)
	}
	if dirtyEvents == 0 {
		t.Fatal("dirty transitions should be reported")
	}

	// New edit after undo truncates the redo stack.
	store.v = "two"
	s.flushPending()
	s.commitPending()
	if s.CanRedo() {
		t.Fatal("new edit must invalidate redo")
	}
}

func TestSessionEngineRedoInvalidationAndDirtyAfterSave(t *testing.T) {
	var store struct{ v string }
	store.v = "base"
	s := NewDocumentSession(
		func() string { return store.v },
		func(src string) error { store.v = src; return nil },
	)
	s.MarkClean()

	// Discrete change (apply-like): pre-state becomes undo entry.
	s.beginDiscreteChange()
	store.v = "edited"
	s.endDiscreteChange()

	if !s.Undo() {
		t.Fatal("apply must be undoable")
	}
	if store.v != "base" {
		t.Fatalf("undo must return to pre-apply state, got %q", store.v)
	}
	if !s.Redo() {
		t.Fatal("redo must reapply")
	}
	if store.v != "edited" {
		t.Fatalf("redo must restore applied state, got %q", store.v)
	}

	// Save re-baselines: undo past the save re-dirties, redo back to
	// the saved bytes is clean again.
	s.SetBaseline("/doc", "edited")
	s.MarkClean()
	if s.DerivedDirty() {
		t.Fatal("just-saved state must be clean")
	}
	s.Undo() // back to "base" (pre-edit)
	if !s.DerivedDirty() {
		t.Fatal("undoing past a save must mark dirty again")
	}
	s.Redo() // back to "edited"
	if s.DerivedDirty() {
		t.Fatal("redo to saved bytes must be clean")
	}
}

func TestSessionEngineFailedRestoreKeepsState(t *testing.T) {
	var store struct{ v string }
	store.v = "good"
	s := NewDocumentSession(
		func() string { return store.v },
		func(src string) error { return fmt.Errorf("parse boom") },
	)
	s.MarkClean()
	s.beginDiscreteChange()
	store.v = "next"
	s.endDiscreteChange()
	if s.Undo() {
		t.Fatal("undo should report failure when restore errors")
	}
	if store.v != "next" {
		t.Fatalf("failed restore must leave state intact, got %q", store.v)
	}
}

func TestSourceDraftStorePerEditor(t *testing.T) {
	store := NewSourceDraftStore()
	if store.Has(nil) {
		t.Fatal("nil editor must never hold a draft")
	}
	sab := &SABEditor{}
	mbch := &MBCHEditor{}
	store.Set(sab, SourceDraft{Text: "saber1 {", ParseErr: "unbalanced braces", EditMode: true})
	store.Set(mbch, SourceDraft{Text: "MBCH\n{\n}"})
	d, ok := store.Get(sab)
	if !ok || d.Text != "saber1 {" || d.ParseErr != "unbalanced braces" || !d.EditMode {
		t.Fatalf("draft roundtrip failed: %+v ok=%v", d, ok)
	}
	if !store.Has(sab) || !store.Has(mbch) {
		t.Fatal("both drafts must be retained")
	}
	store.Delete(sab)
	if store.Has(sab) {
		t.Fatal("deleted draft must be gone")
	}
	if !store.Has(mbch) {
		t.Fatal("other editor's draft must survive")
	}
}

func TestSourceDraftStoreNotifiesOnlyActualDocumentChanges(t *testing.T) {
	store := NewSourceDraftStore()
	editor, other := &SABEditor{}, &SABEditor{}
	draft := SourceDraft{Text: "saber\n{", ParseErr: "unbalanced braces", EditMode: true}
	var observed SourceDraft
	var present bool
	events := 0
	remove := store.addListener(editor, func() {
		events++
		if events > 2 {
			t.Fatal("unchanged draft writes caused reentrant notifications")
		}
		observed, present = store.Get(editor)
		if present {
			// Rendering may normalize diagnostics back into the store.
			store.Set(editor, observed)
		}
	})
	store.Set(editor, draft)
	store.Set(editor, draft)
	store.Set(other, draft)
	store.Delete(other)
	if store.DeleteIfMatch(editor, SourceDraft{Text: "stale"}) {
		t.Fatal("a stale draft consumed the latest text")
	}
	if events != 1 || !present || observed != draft {
		t.Fatal("draft listeners did not observe exactly the changed document")
	}
	if !store.DeleteIfMatch(editor, draft) || present || events != 2 {
		t.Fatal("consuming the reviewed draft did not notify its removal")
	}
	store.Delete(editor)
	remove()
	if len(store.listeners) != 0 {
		t.Fatal("unsubscribing retained the document in the listener registry")
	}
	replacement := store.addListener(editor, func() { observed, present = store.Get(editor) })
	remove() // repeated disposal must not detach the new subscription
	store.Set(editor, draft)
	if !present || observed != draft || events != 2 {
		t.Fatal("a removed listener ran or displaced a replacement subscription")
	}
	store.Delete(editor)
	if present {
		t.Fatal("discarding a draft did not notify its removal")
	}
	replacement()
	if len(store.listeners) != 0 {
		t.Fatal("the replacement subscription retained its document after disposal")
	}
}

func TestSourceListenersMultiSubscribe(t *testing.T) {
	var l sourceListeners
	count := 0
	rm1 := l.add(func() { count++ })
	rm2 := l.add(func() { count += 10 })
	l.fire()
	if count != 11 {
		t.Fatalf("both listeners should fire, got %d", count)
	}
	rm1()
	count = 0
	l.fire()
	if count != 10 {
		t.Fatalf("removed listener must not fire, got %d", count)
	}
	rm2()
	count = 0
	l.fire()
	if count != 0 {
		t.Fatalf("all removed, got %d", count)
	}
	// Legacy reset semantics: single callback replaces the set.
	rm3 := l.add(func() { count++ })
	l.reset(func() { count += 100 })
	rm3()
	count = 0
	l.fire()
	if count != 100 {
		t.Fatalf("reset must leave exactly one listener, got %d", count)
	}
}

func TestSessionHistoryCappedAtMax(t *testing.T) {
	var store struct{ v string }
	store.v = "0"
	s := NewDocumentSession(
		func() string { return store.v },
		func(src string) error { store.v = src; return nil },
	)
	s.maxHistory = 10
	s.MarkClean()
	for i := 1; i <= 50; i++ {
		s.beginDiscreteChange()
		store.v = fmt.Sprintf("%d", i)
		s.endDiscreteChange()
	}
	if len(s.undo) > s.maxHistory {
		t.Fatalf("undo stack must stay capped at %d, got %d", s.maxHistory, len(s.undo))
	}
}
