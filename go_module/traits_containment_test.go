package main

import (
	"strings"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"

	"github.com/Frenzeh/mbii-foundry/parsers"
)

func TestTraitsFlagsReflowWithoutMutation(t *testing.T) {
	useEditorTestApp(t)
	usePublicCatalogs(t)

	LoadedClassFlags = []ClassFlagDef{{
		ID:          "CFL_RESPONSIVE_TRAIT",
		Name:        "Responsive inherent character trait",
		Description: strings.Repeat("This complete inherent trait description remains visible while the editor narrows. ", 4),
	}}
	flagsKey := "WP_" + strings.Repeat("CUSTOM_WEAPON_IDENTIFIER_", 5) + "Flags"
	character := parsers.NewMBCHCharacter()
	character.ClassFlags = LoadedClassFlags[0].ID
	character.ExtraFields = map[string]string{flagsKey: "HELD_STUN"}
	editor := &MBCHEditor{character: character}
	flags := NewWeaponFlagsEditor(editor)
	flags.Refresh()
	root := flags.GetContent()

	if got := root.MinSize().Width; got > 360 {
		t.Fatalf("flags minimum width = %.1f, want at most 360", got)
	}
	originalClassFlags := character.ClassFlags
	originalHeldFlags := character.ExtraFields[flagsKey]
	var firstNarrowHeight float32
	for pass, width := range []float32{360, 760, 360} {
		height := layoutTraitsObject(root, width)
		if pass == 0 {
			firstNarrowHeight = height
		} else if pass == 2 && height != firstNarrowHeight {
			t.Fatalf("flags narrow height changed after wide reflow: first %.1f, second %.1f", firstNarrowHeight, height)
		}

		for _, text := range []string{
			"Inherent character traits, physical attributes, and passive immunities.",
			"Per-weapon combat modifiers (HELD_*) applied when holding the specific weapon.",
			flagsKey,
		} {
			assertTraitsObjectContained(t, root, findTraitsLabel(t, root, text), width)
		}
		assertTraitsObjectContained(t, root, saveDialogButton(t, root, "Add Weapon Flags..."), width)
	}

	if len(flags.listBox.Objects) != 1 {
		t.Fatalf("weapon flag rows = %d, want 1", len(flags.listBox.Objects))
	}
	if got := flags.listBox.Objects[0].Size().Height; got <= 88 {
		t.Fatalf("wrapped weapon flag card retained a fixed-height row: %.1f", got)
	}
	if character.ClassFlags != originalClassFlags || character.ExtraFields[flagsKey] != originalHeldFlags {
		t.Fatalf("layout reflow mutated source state: class=%q held=%q", character.ClassFlags, character.ExtraFields[flagsKey])
	}
}

func TestDefensiveMatrixReflowsMetricsAndPills(t *testing.T) {
	useEditorTestApp(t)

	dm := CalculateDefensiveMatrix(&parsers.MBCHCharacter{
		MaxHealth:  100,
		MaxArmor:   50,
		ExtraLives: 2,
	})
	root := BuildDefensiveMatrixWidget(dm)
	if got := root.MinSize().Width; got > 360 {
		t.Fatalf("defensive matrix minimum width = %.1f, want at most 360", got)
	}

	raw := findTraitsLabel(t, root, "HP: 100  |  Armor: 50  |  Lives: 3  |  Configured pool: 450")
	footnote := findTraitsLabel(t, root, "Unverified: no class- or attribute-based damage-reduction estimates are applied.")
	pills := []*widget.Label{
		findTraitsLabel(t, root, "Energy EHP: Unverified"),
		findTraitsLabel(t, root, "Explosive EHP: Unverified"),
		findTraitsLabel(t, root, "Melee EHP: Unverified"),
	}

	var firstNarrowHeight float32
	for pass, width := range []float32{360, 960, 360} {
		height := layoutTraitsObject(root, width)
		if pass == 0 {
			firstNarrowHeight = height
		} else if pass == 2 && height != firstNarrowHeight {
			t.Fatalf("stats narrow height changed after wide reflow: first %.1f, second %.1f", firstNarrowHeight, height)
		}
		assertTraitsObjectContained(t, root, raw, width)
		assertTraitsObjectContained(t, root, footnote, width)
		for _, pill := range pills {
			assertTraitsObjectContained(t, root, pill, width)
		}
		assertTraitsObjectsDoNotOverlap(t, root, pills)
	}

	if dm.TotalRawEHP != 450 || dm.EnergyEHP != 450 || dm.ExplosiveEHP != 450 || dm.MeleeEHP != 450 {
		t.Fatalf("layout changed defensive calculations: %+v", dm)
	}
}

func TestWeaponCardReflowsAndKeepsLevelAndHoverActions(t *testing.T) {
	useEditorTestApp(t)
	usePublicCatalogs(t)

	var weapon WeaponDef
	for _, candidate := range GetWeapons() {
		if candidate.ID == "WP_BRYAR_PISTOL" {
			weapon = candidate
			break
		}
	}
	if weapon.ID == "" {
		t.Fatal("public Bryar pistol weapon is unavailable")
	}
	weapon.Name = strings.Repeat("Long inventory weapon name ", 7)

	var serialized string
	level := 2
	hovers, leaves := 0, 0
	grid := NewWeaponGrid(weapon.ID, func(value string) {
		serialized = value
	}, func(string, string) {
		hovers++
	}, nil)
	grid.SetOnUnhover(func() { leaves++ })
	grid.SetAttributeBridge(func(string) int { return level }, func(_ string, value int) { level = value })
	grid.SetFlagsBridge(func(string) int { return 2 }, func(string) {})
	grid.SetOverrideBridge(func(string) bool { return true }, func(string) {})

	card := grid.buildCard(weapon)
	if got := card.MinSize().Width; got > 360 {
		t.Fatalf("weapon card minimum width = %.1f, want at most 360", got)
	}
	name := findTraitsLabel(t, card, weapon.Name)
	caption := findTraitsLabel(t, card, "paired: "+CanonicalAttributeFor(weapon.ID))
	controls := []*widget.Button{
		saveDialogButton(t, card, "Off"),
		saveDialogButton(t, card, "1"),
		saveDialogButton(t, card, "2"),
		saveDialogButton(t, card, "3"),
	}
	flagsBadge := findTraitsTooltipButton(t, card, "F·2")
	overrideBadge := findTraitsTooltipButton(t, card, "M")

	var firstNarrowHeight, wideHeight float32
	for pass, width := range []float32{360, 900, 360} {
		height := layoutTraitsObject(card, width)
		switch pass {
		case 0:
			firstNarrowHeight = height
		case 1:
			wideHeight = height
		case 2:
			if height != firstNarrowHeight {
				t.Fatalf("weapon card narrow height changed after wide reflow: first %.1f, second %.1f", firstNarrowHeight, height)
			}
		}
		assertTraitsObjectContained(t, card, name, width)
		assertTraitsObjectContained(t, card, caption, width)
		for _, control := range controls {
			assertTraitsObjectContained(t, card, control, width)
		}
		assertTraitsObjectContained(t, card, flagsBadge, width)
		assertTraitsObjectContained(t, card, overrideBadge, width)
		assertTraitsButtonsDoNotOverlap(t, card, controls)
		if traitsObjectsOverlap(card, flagsBadge, overrideBadge) {
			t.Fatal("weapon metadata actions overlap")
		}
	}
	if firstNarrowHeight <= wideHeight {
		t.Fatalf("long weapon name did not grow the narrow card: narrow %.1f, wide %.1f", firstNarrowHeight, wideHeight)
	}
	if firstNarrowHeight <= 88 {
		t.Fatalf("weapon card retained a fixed 88px row height: %.1f", firstNarrowHeight)
	}

	hover, ok := card.(*HoverContainer)
	if !ok {
		t.Fatalf("weapon card lost hover surface: %T", card)
	}
	hover.MouseIn(&desktop.MouseEvent{})
	hover.MouseOut()
	if hovers != 1 || leaves != 1 {
		t.Fatalf("hover lifecycle = %d/%d, want 1/1", hovers, leaves)
	}

	test.Tap(controls[3])
	if level != 3 || serialized != weapon.ID {
		t.Fatalf("level action produced level=%d weapons=%q, want 3/%q", level, serialized, weapon.ID)
	}
	selected := grid.buildLevelPills(weapon.ID, CanonicalAttributeFor(weapon.ID), weapon)
	if button := saveDialogButton(t, selected, "3"); button.Importance != widget.HighImportance {
		t.Fatal("selected level lost its visible selected state")
	}
}

func layoutTraitsObject(root fyne.CanvasObject, width float32) float32 {
	root.Resize(fyne.NewSize(width, 10000))
	height := root.MinSize().Height
	root.Resize(fyne.NewSize(width, height))
	return height
}

func findTraitsLabel(t *testing.T, root fyne.CanvasObject, text string) *widget.Label {
	t.Helper()
	if label := findTraitsLabelOrNil(root, text); label != nil {
		return label
	}
	t.Fatalf("label %q not found", text)
	return nil
}

func findTraitsLabelOrNil(root fyne.CanvasObject, text string) *widget.Label {
	if label, ok := root.(*widget.Label); ok && label.Text == text {
		return label
	}
	for _, child := range classFlagChildren(root) {
		if label := findTraitsLabelOrNil(child, text); label != nil {
			return label
		}
	}
	return nil
}

func findTraitsTooltipButton(t *testing.T, root fyne.CanvasObject, text string) *TooltipButton {
	t.Helper()
	if button := findTraitsTooltipButtonOrNil(root, text); button != nil {
		return button
	}
	t.Fatalf("tooltip button %q not found", text)
	return nil
}

func findTraitsTooltipButtonOrNil(root fyne.CanvasObject, text string) *TooltipButton {
	if button, ok := root.(*TooltipButton); ok && button.Text == text {
		return button
	}
	for _, child := range classFlagChildren(root) {
		if button := findTraitsTooltipButtonOrNil(child, text); button != nil {
			return button
		}
	}
	return nil
}

func assertTraitsObjectContained(t *testing.T, root, object fyne.CanvasObject, width float32) {
	t.Helper()
	position, ok := classFlagObjectPosition(root, object)
	if !ok {
		t.Fatalf("%.0fpx object %T is outside its surface", width, object)
	}
	if position.X < 0 || position.Y < 0 ||
		position.X+object.Size().Width > root.Size().Width+1 ||
		position.Y+object.Size().Height > root.Size().Height+1 {
		t.Fatalf("%.0fpx object %T is clipped: position=%v size=%v root=%v",
			width, object, position, object.Size(), root.Size())
	}
}

func assertTraitsObjectsDoNotOverlap(t *testing.T, root fyne.CanvasObject, objects []*widget.Label) {
	t.Helper()
	for i := 0; i < len(objects); i++ {
		for j := i + 1; j < len(objects); j++ {
			if traitsObjectsOverlap(root, objects[i], objects[j]) {
				t.Fatalf("stat pills %q and %q overlap", objects[i].Text, objects[j].Text)
			}
		}
	}
}

func assertTraitsButtonsDoNotOverlap(t *testing.T, root fyne.CanvasObject, buttons []*widget.Button) {
	t.Helper()
	for i := 0; i < len(buttons); i++ {
		for j := i + 1; j < len(buttons); j++ {
			if traitsObjectsOverlap(root, buttons[i], buttons[j]) {
				t.Fatalf("level controls %q and %q overlap", buttons[i].Text, buttons[j].Text)
			}
		}
	}
}

func traitsObjectsOverlap(root, first, second fyne.CanvasObject) bool {
	firstPosition, firstOK := classFlagObjectPosition(root, first)
	secondPosition, secondOK := classFlagObjectPosition(root, second)
	if !firstOK || !secondOK {
		return true
	}
	return firstPosition.X < secondPosition.X+second.Size().Width &&
		secondPosition.X < firstPosition.X+first.Size().Width &&
		firstPosition.Y < secondPosition.Y+second.Size().Height &&
		secondPosition.Y < firstPosition.Y+first.Size().Height
}
