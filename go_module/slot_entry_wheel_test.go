package main

import (
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/test"
)

func TestPointBuySlotEntryRoutesWheelToPageAndArchetypeOwners(t *testing.T) {
	e := loadAuditMBCH(t, pointBuyContainmentFixture)
	p := e.pointBuyUI
	p.modeTabs.SelectIndex(0)
	p.specTabs.SelectIndex(0)

	page, ok := p.pageWheel.target.(*container.Scroll)
	if !ok || page == nil {
		t.Fatal("Point Buy page wheel route is not bound to its vertical scroll")
	}
	archetype, ok := p.windowWheel[0].target.(*container.Scroll)
	if !ok || archetype == nil {
		t.Fatal("first archetype wheel route is not bound to its slot scroll")
	}

	window := fyne.CurrentApp().NewWindow("Point Buy slot wheel routing")
	window.SetPadded(false)
	window.SetContent(archetype)
	window.Resize(fyne.NewSize(640, 460))
	window.Show()
	defer window.Close()
	window.Canvas().Capture()

	entry := p.slotNames[0]
	value := "A280 Blaster Rifle: " + strings.Repeat("precision configuration / ", 30)
	entry.SetText(value)
	window.Canvas().Focus(entry)
	entry.TypedShortcut(&fyne.ShortcutSelectAll{})
	selected, cursor := entry.SelectedText(), entry.CursorTextOffset()
	if selected != value {
		t.Fatal("slot entry did not establish the selection used by the wheel regression")
	}
	horizontal := slotEntryHorizontalViewport(t, entry)

	assertStable := func(action string) {
		t.Helper()
		if entry.Text != value || entry.SelectedText() != selected || entry.CursorTextOffset() != cursor {
			t.Fatalf("%s changed entry state: text=%q selection=%q cursor=%d", action, entry.Text, entry.SelectedText(), entry.CursorTextOffset())
		}
	}
	scrollAtEntry := func(dx, dy float32) {
		t.Helper()
		position := fyne.CurrentApp().Driver().AbsolutePositionForObject(entry).
			Add(fyne.NewPos(20, entry.Size().Height/2))
		slotEntryScrollAndWait(t, window.Canvas(), position, dx, dy, page, archetype, horizontal)
	}

	pageBefore := page.Offset
	horizontalBefore := horizontal.Offset
	scrollAtEntry(0, -40)
	if archetype.Offset.Y <= 0 || horizontal.Offset != horizontalBefore || page.Offset != pageBefore {
		t.Fatalf("vertical slot wheel routed incorrectly: page=%v archetype=%v horizontal=%v", page.Offset, archetype.Offset, horizontal.Offset)
	}
	assertStable("vertical wheel")

	archetype.ScrollToTop()
	horizontal.ScrollToTop()
	window.Canvas().Capture()
	scrollAtEntry(-40, 0)
	if horizontal.Offset.X <= 0 || archetype.Offset.Y != 0 || page.Offset != pageBefore {
		t.Fatalf("horizontal slot wheel routed incorrectly: page=%v archetype=%v horizontal=%v", page.Offset, archetype.Offset, horizontal.Offset)
	}
	assertStable("horizontal wheel")

	archetype.ScrollToTop()
	horizontal.ScrollToTop()
	window.Canvas().Capture()
	scrollAtEntry(-10, -40)
	if archetype.Offset.Y <= 0 || horizontal.Offset.X != 0 {
		t.Fatalf("vertical-dominant diagonal routed incorrectly: archetype=%v horizontal=%v", archetype.Offset, horizontal.Offset)
	}
	assertStable("vertical-dominant diagonal wheel")

	archetype.ScrollToTop()
	horizontal.ScrollToTop()
	window.Canvas().Capture()
	scrollAtEntry(-40, -10)
	if horizontal.Offset.X <= 0 || archetype.Offset.Y != 0 {
		t.Fatalf("horizontal-dominant diagonal routed incorrectly: archetype=%v horizontal=%v", archetype.Offset, horizontal.Offset)
	}
	assertStable("horizontal-dominant diagonal wheel")

	window.SetContent(page)
	window.Resize(fyne.NewSize(640, 360))
	window.Canvas().Capture()
	header := p.specNameEntries[0]
	header.SetText("Gunner " + strings.Repeat("long-range archetype / ", 20))
	headerHorizontal := slotEntryHorizontalViewport(t, header)
	headerPosition, found := pointBuyObjectPositionWithin(page.Content, header)
	if !found {
		t.Fatal("page-bound archetype header is absent from the Point Buy page")
	}
	page.ScrollToOffset(fyne.NewPos(0, fyne.Max(0, headerPosition.Y-20)))
	window.Canvas().Capture()
	pageBefore = page.Offset
	archetypeBefore := archetype.Offset
	headerHorizontalBefore := headerHorizontal.Offset
	position := fyne.CurrentApp().Driver().AbsolutePositionForObject(header).
		Add(fyne.NewPos(20, header.Size().Height/2))
	slotEntryScrollAndWait(t, window.Canvas(), position, 0, -40, page, archetype, horizontal, headerHorizontal)
	if page.Offset.Y <= pageBefore.Y || archetype.Offset != archetypeBefore || headerHorizontal.Offset != headerHorizontalBefore {
		t.Fatalf("page-bound header wheel routed incorrectly: page=%v archetype=%v horizontal=%v", page.Offset, archetype.Offset, headerHorizontal.Offset)
	}

	oldArchetype := archetype
	oldOffset := oldArchetype.Offset
	p.rebuildArchetypeTabs()
	p.specTabs.SelectIndex(0)
	rebound, ok := p.windowWheel[0].target.(*container.Scroll)
	if !ok || rebound == nil || rebound == oldArchetype {
		t.Fatal("archetype rebuild did not rebind the slot wheel route to the replacement scroll")
	}
	window.SetContent(rebound)
	window.Resize(fyne.NewSize(640, 460))
	window.Canvas().Capture()
	position = fyne.CurrentApp().Driver().AbsolutePositionForObject(entry).
		Add(fyne.NewPos(20, entry.Size().Height/2))
	slotEntryScrollAndWait(t, window.Canvas(), position, 0, -40, page, oldArchetype, rebound, horizontal, headerHorizontal)
	if rebound.Offset.Y <= 0 || oldArchetype.Offset != oldOffset {
		t.Fatalf("rebuilt slot wheel used stale owner: old=%v replacement=%v", oldArchetype.Offset, rebound.Offset)
	}
	assertStable("wheel after archetype rebuild")
}

func slotEntryHorizontalViewport(t *testing.T, entry *slotEntry) *container.Scroll {
	t.Helper()
	for _, object := range test.WidgetRenderer(entry).Objects() {
		if viewport, ok := object.(*container.Scroll); ok && viewport.Direction == container.ScrollHorizontalOnly {
			return viewport
		}
	}
	t.Fatal("slot entry has no native horizontal viewport")
	return nil
}

// Fyne 2.8's native Scroll owns an AfterFunc auto-hide timer that neither window
// Close, renderer Destroy, nor the test app's Quit joins. Keep each real pointer
// event inside its app lifetime, including failure paths, without changing the
// native scrollbar preference. The private flag is only a cleanup condition:
// routing and entry-state assertions above still observe the real widgets.
func slotEntryScrollAndWait(t *testing.T, c fyne.Canvas, position fyne.Position, dx, dy float32, owners ...*container.Scroll) {
	t.Helper()
	states := make([]reflect.Value, len(owners))
	for i, owner := range owners {
		states[i] = reflect.ValueOf(owner).Elem().FieldByName("scrolling")
		if states[i].Kind() != reflect.Bool {
			t.Fatal("Fyne Scroll timer lifecycle changed: update the wheel-test drain")
		}
	}

	app := fyne.CurrentApp()
	driver := &slotEntryWheelDriver{Driver: app.Driver(), wake: make(chan struct{}, 1)}
	fyne.SetCurrentApp(&slotEntryWheelApp{App: app, driver: driver})
	defer fyne.SetCurrentApp(app)
	defer func() {
		deadline := time.NewTimer(5 * time.Second)
		defer deadline.Stop()
		for {
			select {
			case <-deadline.C:
				panic("timed out draining Fyne native scroll auto-hide callbacks")
			default:
			}
			driver.mu.Lock()
			pending := driver.pending
			driver.pending = nil
			driver.mu.Unlock()
			for _, callback := range pending {
				callback()
			}
			if len(pending) != 0 {
				continue
			}
			scrolling := false
			for _, state := range states {
				scrolling = scrolling || state.Bool()
			}
			if !scrolling {
				driver.mu.Lock()
				if len(driver.pending) != 0 {
					driver.mu.Unlock()
					continue
				}
				driver.finished = true
				driver.mu.Unlock()
				return
			}
			select {
			case <-driver.wake:
			case <-deadline.C:
				// Fail closed: t.Fatal would let the next test replace the app
				// while a native timer can still dispatch into it.
				panic("timed out draining Fyne native scroll auto-hide callbacks")
			}
		}
	}()
	test.Scroll(c, position, dx, dy)
}

type slotEntryWheelApp struct {
	fyne.App
	driver fyne.Driver
}

func (a *slotEntryWheelApp) Driver() fyne.Driver {
	return a.driver
}

// The headless driver normally executes timer callbacks on their goroutines.
// Queue them instead so the drain observes scrolling only on the test goroutine,
// after the entire callback (including refreshBars) has returned, not merely
// after it clears scrolling. No unsafe access or timer-delay duplication is used.
type slotEntryWheelDriver struct {
	fyne.Driver
	mu       sync.Mutex
	pending  []func()
	wake     chan struct{}
	finished bool
}

func (d *slotEntryWheelDriver) DoFromGoroutine(fn func(), wait bool) {
	var done chan struct{}
	if wait {
		done = make(chan struct{})
		callback := fn
		fn = func() {
			defer close(done)
			callback()
		}
	}
	d.mu.Lock()
	if d.finished {
		d.mu.Unlock()
		d.Driver.DoFromGoroutine(fn, wait)
		return
	}
	d.pending = append(d.pending, fn)
	d.mu.Unlock()
	select {
	case d.wake <- struct{}{}:
	default:
	}
	if wait {
		<-done
	}
}
