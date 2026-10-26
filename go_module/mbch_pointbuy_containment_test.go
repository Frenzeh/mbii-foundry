package main

import (
	"strings"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/test"
	"github.com/Frenzeh/mbii-foundry/parsers"
)

const pointBuyContainmentFixture = `ClassInfo
{
	name "Responsive point buy"
	isCustomBuild 1
	mbPoints 100
	hasCustomSpec 3
	customSpecName_1 "Gunner"
	customSpecIcon_1 "gfx/menus/alpha/icon_weap_accuracy"
	customSpecDesc_1 "Long-range weapons and configurable support equipment"
	customSpecName_2 "Scout"
	customSpecIcon_2 "gfx/menus/alpha/icon_force_seeing"
	customSpecDesc_2 "Mobility, reconnaissance, and utility"
	customSpecName_3 "Vanguard"
	customSpecIcon_3 "gfx/menus/alpha/icon_weap_heavy"
	customSpecDesc_3 "Front-line durability and heavy weapons"
	c_att_skill_0 MB_ATT_PISTOL
	c_att_names_0 "A280 Blaster Rifle: precision configuration"
	c_att_ranks_0 0,4,10
	c_att_descs_0 "Adds secondary fire without changing the canonical slot"
	c_att_skill_15 MB_ATT_PUSH
	c_att_names_15 "Force Push: field control"
	c_att_ranks_15 1,3,7
	c_att_descs_15 "Configured independently in the second archetype window"
	c_att_skill_30 MB_ATT_STAMINA
	c_att_names_30 "Stamina: vanguard endurance"
	c_att_ranks_30 2,5,9
	c_att_descs_30 "Configured independently in the third archetype window"
	rankHealth 100,120,150
	rankArmourRegenAmount 0,2,4
	rankResourceRegenAmount 1,3,6
}
`

func TestPointBuyContainmentPreservesAllArchetypesAndHistory(t *testing.T) {
	e := loadAuditMBCH(t, pointBuyContainmentFixture)
	p := e.pointBuyUI
	cleanSource := e.GenerateSource()
	if e.IsDirty() || e.CanUndo() || e.CanRedo() {
		t.Fatal("loaded Point Buy fixture did not begin with clean history")
	}

	const editedName = "Stamina: vanguard endurance after responsive edit"
	p.slotNames[30].SetText(editedName)
	e.session.flushPending()
	editedSource := e.GenerateSource()
	if !e.IsDirty() || !e.CanUndo() || e.CanRedo() || editedSource == cleanSource {
		t.Fatal("Point Buy edit did not establish dirty undoable state before resizing")
	}

	viewport := container.NewVScroll(p.GetContent())
	window := fyne.CurrentApp().NewWindow("point buy containment")
	window.SetPadded(false)
	window.SetContent(viewport)
	window.Show()
	defer window.Close()

	for _, width := range []float32{360, 600, 900, 600, 360} {
		window.Resize(fyne.NewSize(width, 900))
		window.Canvas().Capture()

		if p.GetContent().MinSize().Width > 360+1 {
			t.Fatalf("Point Buy minimum pins the %.0fpx viewport: %v", width, p.GetContent().MinSize())
		}
		if p.GetContent().Size().Width > viewport.Size().Width+1 {
			t.Fatalf("Point Buy content escaped its %.0fpx viewport: content=%v viewport=%v", width, p.GetContent().Size(), viewport.Size())
		}
		for _, section := range p.container.Objects {
			if section.MinSize().Width > 360+1 || section.Size().Width > p.container.Size().Width+1 {
				t.Fatalf("Point Buy section pins or escapes %.0fpx: minimum=%v size=%v root=%v", width, section.MinSize(), section.Size(), p.container.Size())
			}
		}

		p.modeTabs.SelectIndex(0)
		for spec := 0; spec < parsers.PointbuyMaxArchetypes; spec++ {
			p.specTabs.SelectIndex(spec)
			window.Canvas().Capture()
			base := spec * parsers.PointbuySlotsPerArchetype
			row := p.slotRows[base]
			if row.MinSize().Width > 360+1 {
				t.Fatalf("archetype %d slot row pins the %.0fpx viewport: %v", spec+1, width, row.MinSize())
			}
			for _, control := range []fyne.CanvasObject{
				p.slotModes[base], p.slotSkills[base], p.slotNames[base],
				p.slotRanks[base], p.slotDescs[base], p.slotMaxCost[base],
			} {
				assertPointBuyObjectContained(t, row, control)
			}
			for _, entry := range []*slotEntry{
				p.specNameEntries[spec], p.specIconEntries[spec], p.specDescEntries[spec],
			} {
				assertPointBuyObjectContained(t, p.specTabs.Items[spec].Content, entry)
			}
		}

		p.modeTabs.SelectIndex(1)
		window.Canvas().Capture()
		if p.simulator.container.MinSize().Width > 360+1 {
			t.Fatalf("hidden simulator pins the %.0fpx Point Buy viewport: %v", width, p.simulator.container.MinSize())
		}
		assertPointBuyObjectContained(t, p.simulator.container, p.simulator.resetBtn)
		for _, button := range p.simulator.archetypeBar.Objects {
			assertPointBuyObjectContained(t, p.simulator.container, button)
		}

		if got := e.GenerateSource(); got != editedSource || !e.IsDirty() || !e.CanUndo() || e.CanRedo() {
			t.Fatalf("resize or tab selection changed source/history at %.0fpx", width)
		}
		if p.slotNames[30].Text != editedName ||
			p.specIconEntries[0].Text != "gfx/menus/alpha/icon_weap_accuracy" ||
			p.slotDescs[15].Text != "Configured independently in the second archetype window" {
			t.Fatalf("resize to %.0fpx truncated or replaced an authored Point Buy value", width)
		}
		ranks := auditMBCHSlotEntries(p.rankAttrContainer)
		if len(ranks) != 3 {
			t.Fatalf("resize to %.0fpx lost rank-modifier editors: got %d", width, len(ranks))
		}
		for _, entry := range ranks {
			assertPointBuyObjectContained(t, p.rankAttrContainer, entry)
			if !strings.Contains(entry.Text, ",") {
				t.Fatalf("resize to %.0fpx replaced rank values with %q", width, entry.Text)
			}
		}
	}

	if !e.Undo() || e.IsDirty() || e.GenerateSource() != cleanSource {
		t.Fatal("responsive Point Buy edit did not undo to the clean parsed document")
	}
	if !e.Redo() || !e.IsDirty() || e.GenerateSource() != editedSource {
		t.Fatal("responsive Point Buy edit did not redo after the resize cycle")
	}
}

func assertPointBuyObjectContained(t *testing.T, root, object fyne.CanvasObject) {
	t.Helper()
	position, found := pointBuyObjectPositionWithin(root, object)
	if !found || !object.Visible() || object.Size().Width <= 0 || object.Size().Height <= 0 ||
		position.X < -1 || position.Y < -1 ||
		position.X+object.Size().Width > root.Size().Width+1 ||
		position.Y+object.Size().Height > root.Size().Height+1 {
		t.Fatalf("%T escapes %T: found=%v visible=%v position=%v size=%v bounds=%v",
			object, root, found, object.Visible(), position, object.Size(), root.Size())
	}
}

func pointBuyObjectPositionWithin(root, target fyne.CanvasObject) (fyne.Position, bool) {
	if root == target {
		return fyne.NewPos(0, 0), true
	}
	for _, child := range pointBuyChildObjects(root) {
		position, found := pointBuyObjectPositionWithin(child, target)
		if found {
			return position.Add(child.Position()), true
		}
	}
	return fyne.Position{}, false
}

func pointBuyChildObjects(root fyne.CanvasObject) []fyne.CanvasObject {
	switch object := root.(type) {
	case *fyne.Container:
		return object.Objects
	case fyne.Widget:
		return test.WidgetRenderer(object).Objects()
	default:
		return nil
	}
}
