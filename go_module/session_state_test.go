package main

import (
	"fmt"
	"testing"
)

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
