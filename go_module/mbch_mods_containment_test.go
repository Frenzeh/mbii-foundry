package main

import (
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"

	"github.com/Frenzeh/mbii-foundry/parsers"
)

const modContainmentFixture = `ClassInfo
{
	name "Responsive Mods"
}
WeaponInfo0
{
	WeaponToReplace WP_BLASTER
	WeaponBasedOff WP_PISTOL
	WeaponName "First override"
}
WeaponInfo1
{
	WeaponToReplace WP_REPEATER
	WeaponBasedOff WP_BLASTER
	NewWorldModel "models/weapons2/repeater/world.glm"
	NewViewModel "models/weapons2/repeater/view.glm"
	Icon "gfx/hud/w_icon_repeater"
	WeaponName "Second override"
	MuzzleEffect "repeater/muzzle"
	AltMuzzleEffect "repeater/alt_muzzle"
	MissileEffect "repeater/shot"
	AltMissileEffect "repeater/alt_shot"
	FlashSound0 "sound/weapons/repeater/fire.wav"
	AltFlashSound0 "sound/weapons/repeater/alt_fire.wav"
	ChargeSound "sound/weapons/repeater/charge.wav"
	customAmmo +0010
	clipSize +0007
	reloadTimeModifier +1.2300e+00
	futureWeaponField "keep exact"
}
ForceInfo0
{
	ForceToReplace FP_PUSH
	ForcePowerName "First force override"
}
ForceInfo1
{
	ForceToReplace FP_LIGHTNING
	Icon "gfx/hud/f_lightning"
	ForcePowerName "Second force override"
	StartSound "sound/weapons/force/lightning/start.wav"
	LoopSound "sound/weapons/force/lightning/loop.wav"
	futureForceField "keep exact"
}
`

func TestMBCHModEditorsReflowWithoutChangingSelectionOrSource(t *testing.T) {
	usePublicCatalogs(t)
	useEditorTestApp(t)

	t.Run("weapon overrides", func(t *testing.T) {
		editor := newModContainmentEditor(t)
		ui := NewWeaponInfoUI(editor)
		ui.weaponList.Select(1)
		assertModEditorReflow(t, editor, ui.GetContent(), ui.weaponList, ui.detailPane,
			func() int { return ui.currentWeaponIndex })
	})

	t.Run("force overrides", func(t *testing.T) {
		editor := newModContainmentEditor(t)
		ui := NewForceInfoUI(editor)
		ui.forceList.Select(1)
		assertModEditorReflow(t, editor, ui.GetContent(), ui.forceList, ui.detailPane,
			func() int { return ui.currentForceIndex })
	})
}

func TestMBCHModEditorsKeepDeepDetailsEditableAtNarrowWidth(t *testing.T) {
	usePublicCatalogs(t)
	useEditorTestApp(t)

	t.Run("weapon reload", func(t *testing.T) {
		editor := newModContainmentEditor(t)
		ui := NewWeaponInfoUI(editor)
		ui.weaponList.Select(1)
		resetModContainmentSession(t, editor)

		window := fyne.CurrentApp().NewWindow("narrow weapon mod details")
		window.SetPadded(false)
		window.SetContent(ui.GetContent())
		window.Show()
		defer window.Close()
		window.Resize(fyne.NewSize(360, 720))

		detailScroll := ui.detailPane.Objects[1].(*container.Scroll)
		accordion := detailScroll.Content.(*widget.Accordion)
		accordion.Open(3)
		detailScroll.ScrollToBottom()
		window.Canvas().Capture()
		assertModControlInViewport(t, detailScroll, ui.reloadTimeModifierEntry)

		ui.reloadTimeModifierEntry.SetText("0.625")
		if got := editor.character.WeaponOverrides[1].ReloadTimeModifier; got != 0.625 {
			t.Fatalf("narrow reload edit = %g, want 0.625", got)
		}
		if !editor.IsDirty() {
			t.Fatal("narrow weapon detail edit did not reach the native dirty callback")
		}
	})

	t.Run("force loop sound", func(t *testing.T) {
		editor := newModContainmentEditor(t)
		ui := NewForceInfoUI(editor)
		ui.forceList.Select(1)
		resetModContainmentSession(t, editor)

		window := fyne.CurrentApp().NewWindow("narrow force mod details")
		window.SetPadded(false)
		window.SetContent(ui.GetContent())
		window.Show()
		defer window.Close()
		window.Resize(fyne.NewSize(360, 720))

		detailScroll := ui.detailPane.Objects[1].(*container.Scroll)
		detailScroll.ScrollToBottom()
		window.Canvas().Capture()
		assertModControlInViewport(t, detailScroll, ui.loopSoundEntry)

		const changed = "sound/weapons/force/lightning/cafe_beta_loop.wav"
		ui.loopSoundEntry.SetText(changed)
		if got := editor.character.ForceOverrides[1].LoopSound; got != changed {
			t.Fatalf("narrow loop-sound edit = %q, want %q", got, changed)
		}
		if !editor.IsDirty() {
			t.Fatal("narrow force detail edit did not reach the native dirty callback")
		}
	})
}

func newModContainmentEditor(t *testing.T) *MBCHEditor {
	t.Helper()
	character, err := parsers.ParseMBCH(modContainmentFixture)
	if err != nil {
		t.Fatal(err)
	}
	return &MBCHEditor{character: character, sourceSubs: &sourceListeners{}}
}

func resetModContainmentSession(t *testing.T, editor *MBCHEditor) {
	t.Helper()
	render := func() string {
		source, _ := parsers.GenerateMBCH(editor.character)
		return source
	}
	editor.session = NewDocumentSession(render, func(string) error { return nil })
	editor.session.SetOnDirtyChange(editor.setDirtyState)
	editor.session.Reset()
	t.Cleanup(editor.session.flushPending)
}

func assertModEditorReflow(t *testing.T, editor *MBCHEditor, content fyne.CanvasObject,
	selector *widget.List, details fyne.CanvasObject, selectedIndex func() int,
) {
	t.Helper()
	before, err := parsers.GenerateMBCH(editor.character)
	if err != nil {
		t.Fatal(err)
	}
	window := fyne.CurrentApp().NewWindow("mod editor containment")
	window.SetPadded(false)
	window.SetContent(content)
	window.Show()
	defer window.Close()

	for _, width := range []float32{360, 600, 900, 600, 360} {
		window.Resize(fyne.NewSize(width, 720))
		window.Canvas().Capture()
		if min := content.MinSize().Width; min > 360 {
			t.Fatalf("%.0fpx resize left a %.1fpx mod-editor minimum", width, min)
		}
		if selector.Size().Width <= 0 || selector.Size().Height < 80 {
			t.Fatalf("%.0fpx selector viewport is not usable: %v", width, selector.Size())
		}
		if details.Size().Width <= 0 || details.Size().Height < 300 {
			t.Fatalf("%.0fpx detail viewport is not usable: %v", width, details.Size())
		}
		selectorPosition, selectorFound := objectPositionWithin(content, selector)
		detailPosition, detailFound := objectPositionWithin(content, details)
		if !selectorFound || !detailFound {
			t.Fatal("selector or detail surface is not reachable from the mod-editor content")
		}
		if width == 360 && detailPosition.Y <= selectorPosition.Y {
			t.Fatalf("narrow mod editor did not stack details below selector: selector=%v details=%v", selectorPosition, detailPosition)
		}
		if width == 900 && detailPosition.X <= selectorPosition.X {
			t.Fatalf("wide mod editor did not place details beside selector: selector=%v details=%v", selectorPosition, detailPosition)
		}
		if selectedIndex() != 1 {
			t.Fatalf("%.0fpx resize changed selected override to %d", width, selectedIndex())
		}
		after, err := parsers.GenerateMBCH(editor.character)
		if err != nil {
			t.Fatal(err)
		}
		if after != before {
			t.Fatalf("%.0fpx resize changed retained source bytes", width)
		}
		if editor.IsDirty() {
			t.Fatalf("%.0fpx resize dirtied the editor", width)
		}
	}
}

func assertModControlInViewport(t *testing.T, viewport *container.Scroll, control fyne.CanvasObject) {
	t.Helper()
	position, found := objectPositionWithin(viewport, control)
	size := control.Size()
	if !found || !control.Visible() || size.Width <= 0 || size.Height <= 0 ||
		position.X < -1 || position.Y < -1 ||
		position.X+size.Width > viewport.Size().Width+1 ||
		position.Y+size.Height > viewport.Size().Height+1 {
		t.Fatalf("deep detail control is not reachable in its viewport: found=%v visible=%v position=%v size=%v viewport=%v",
			found, control.Visible(), position, size, viewport.Size())
	}
}
