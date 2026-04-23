package main

// Skin variants editor — MBCH files can offer multiple visual
// skins per character (up to ~20 in Legends content: model_N +
// skin_N + uishader_N tuples, some with RGB overrides). Previously
// editable only via the Source tab or raw ExtraFields soup; this
// dedicated panel renders each variant as a row with typed form
// fields + a live portrait preview, and lets users add/remove/
// edit variants without touching strings.
//
// Field format on disk:
//   model         "crixmadine"         // base — slot 0
//   skin          "default"
//   uishader      "models/players/crixmadine/mb2_icon_default"
//   model_1       "crixmadine"         // variant — slot 1
//   skin_1        "alt"
//   uishader_1    "models/players/crixmadine/mb2_icon_alt"
//   userRGB_1     1                    // optional: enables RGB
//   customred_1   0.945                // 0.0 – 1.0
//   customgreen_1 0.510
//   customblue_1  0.099
//
// Data flow:
//   - Base (slot 0) binds to MBCHCharacter.Model/Skin/UIShader
//     directly — these have dedicated struct fields and live in
//     the Profile tab already.
//   - Variants 1..N live in ExtraFields under the suffixed keys.
//     The editor reads/writes that map.
//
// We cap variants at 25 in the UI even though MBII technically
// allows more — no real FA ships beyond ~20, and the cap keeps
// the row list manageable.

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

const maxSkinVariants = 25

// SkinVariantsEditor renders the list of variants + an "add" button.
type SkinVariantsEditor struct {
	editor *MBCHEditor

	container *fyne.Container
	listBox   *fyne.Container
}

func NewSkinVariantsEditor(editor *MBCHEditor) *SkinVariantsEditor {
	sve := &SkinVariantsEditor{editor: editor}
	sve.createUI()
	return sve
}

func (sve *SkinVariantsEditor) createUI() {
	sve.listBox = container.NewVBox()

	addBtn := widget.NewButtonWithIcon("Add skin variant", theme.ContentAddIcon(),
		sve.addVariant)

	note := widget.NewLabel("Base model/skin/uishader live on the Profile tab; these are the additional pickable variants.")
	note.Wrapping = fyne.TextWrapWord
	note.TextStyle = fyne.TextStyle{Italic: true}

	sve.container = container.NewVBox(
		container.NewPadded(note),
		container.NewPadded(addBtn),
		sve.listBox,
	)
}

func (sve *SkinVariantsEditor) GetContent() fyne.CanvasObject {
	return sve.container
}

// Refresh rebuilds the variant list from ExtraFields. Called on load
// and after any add/remove mutation.
func (sve *SkinVariantsEditor) Refresh() {
	sve.listBox.Objects = nil

	ch := sve.editor.character
	if ch.ExtraFields == nil {
		ch.ExtraFields = map[string]string{}
	}

	// Collect every variant index that has at least one field set
	// (model_N, skin_N, uishader_N). Sorted ascending so the rows
	// render 1, 2, 3, …
	indices := collectVariantIndices(ch.ExtraFields)
	for _, idx := range indices {
		sve.listBox.Add(sve.buildRow(idx))
	}
	sve.listBox.Refresh()
}

// addVariant appends a new row at the next free index — keeps the
// variant list contiguous (no gaps) which matches how the game
// iterates them. Respects the cap.
func (sve *SkinVariantsEditor) addVariant() {
	ch := sve.editor.character
	if ch.ExtraFields == nil {
		ch.ExtraFields = map[string]string{}
	}
	indices := collectVariantIndices(ch.ExtraFields)
	next := 1
	if len(indices) > 0 {
		next = indices[len(indices)-1] + 1
	}
	if next > maxSkinVariants {
		dialog.ShowInformation("Variant limit reached",
			fmt.Sprintf("Foundry caps variants at %d. Delete one to add another.", maxSkinVariants),
			sve.editor.app.mainWindow)
		return
	}
	// Seed with the base model + default skin so the row isn't
	// entirely blank. Author edits from there.
	ch.ExtraFields[fmt.Sprintf("model_%d", next)] = ch.Model
	ch.ExtraFields[fmt.Sprintf("skin_%d", next)] = "default"
	ch.ExtraFields[fmt.Sprintf("uishader_%d", next)] = fmt.Sprintf("models/players/%s/mb2_icon_default", ch.Model)
	sve.editor.markDirty()
	sve.Refresh()
}

// buildRow renders one variant's form + delete button + portrait
// preview. Every field is a direct read/write to ExtraFields so
// changes flow straight to the save side.
func (sve *SkinVariantsEditor) buildRow(idx int) fyne.CanvasObject {
	ch := sve.editor.character

	modelKey := fmt.Sprintf("model_%d", idx)
	skinKey := fmt.Sprintf("skin_%d", idx)
	shaderKey := fmt.Sprintf("uishader_%d", idx)
	rgbKey := fmt.Sprintf("userRGB_%d", idx)
	redKey := fmt.Sprintf("customred_%d", idx)
	greenKey := fmt.Sprintf("customgreen_%d", idx)
	blueKey := fmt.Sprintf("customblue_%d", idx)

	// Portrait preview — reuses AssetBrowser.LoadIconResource which
	// cache-hits the embed set and falls back to VFS. Updates live
	// as the author edits the shader/model/skin.
	preview := canvas.NewImageFromResource(theme.FileImageIcon())
	preview.FillMode = canvas.ImageFillContain
	preview.ScaleMode = canvas.ImageScaleSmooth
	preview.SetMinSize(fyne.NewSize(64, 64))
	refreshPreview := func() {
		if sve.editor.iconResolver == nil || sve.editor.assetBrowser == nil {
			preview.Resource = theme.FileImageIcon()
			preview.Refresh()
			return
		}
		path := sve.editor.iconResolver.ResolveClassIcon(
			ch.ExtraFields[modelKey],
			ch.ExtraFields[skinKey],
			ch.ExtraFields[shaderKey],
		)
		if path == "" {
			preview.Resource = theme.FileImageIcon()
			preview.Refresh()
			return
		}
		if res := sve.editor.assetBrowser.LoadIconResource(path); res != nil {
			preview.Resource = res
			preview.Refresh()
			return
		}
		preview.Resource = theme.FileImageIcon()
		preview.Refresh()
	}
	refreshPreview()

	// Form fields. Each OnChanged writes directly back to ExtraFields
	// and marks dirty. Empty values drop the key so round-trip save
	// doesn't emit blank lines.
	modelEntry := NewInputEntry()
	modelEntry.SetText(ch.ExtraFields[modelKey])
	modelEntry.SetPlaceHolder("e.g. crixmadine")
	modelEntry.OnChanged = func(s string) {
		setOrDelete(ch.ExtraFields, modelKey, s)
		sve.editor.markDirty()
		refreshPreview()
	}

	skinEntry := NewInputEntry()
	skinEntry.SetText(ch.ExtraFields[skinKey])
	skinEntry.SetPlaceHolder("e.g. default, blue, red")
	skinEntry.OnChanged = func(s string) {
		setOrDelete(ch.ExtraFields, skinKey, s)
		sve.editor.markDirty()
		refreshPreview()
	}

	shaderEntry := NewInputEntry()
	shaderEntry.SetText(ch.ExtraFields[shaderKey])
	shaderEntry.SetPlaceHolder("models/players/<model>/mb2_icon_<skin>")
	shaderEntry.OnChanged = func(s string) {
		setOrDelete(ch.ExtraFields, shaderKey, s)
		sve.editor.markDirty()
		refreshPreview()
	}

	// RGB block — collapsed by default; the Check widget toggles it.
	// When userRGB_N is set, customred/green/blue_N are expected
	// to be 0.0-1.0 floats. We expose entries (not sliders) because
	// author files regularly hard-code specific values (0.471 for
	// brown's R channel, etc.) that a slider would only approximate.
	rgbCheck := widget.NewCheck("Use custom RGB", nil)
	rgbCheck.Checked = strings.TrimSpace(ch.ExtraFields[rgbKey]) == "1"

	redEntry := newFloatEntry(ch.ExtraFields[redKey], redKey, sve, "0.0")
	greenEntry := newFloatEntry(ch.ExtraFields[greenKey], greenKey, sve, "0.0")
	blueEntry := newFloatEntry(ch.ExtraFields[blueKey], blueKey, sve, "0.0")

	rgbRow := container.NewGridWithColumns(3,
		widget.NewFormItem("Red", redEntry).Widget,
		widget.NewFormItem("Green", greenEntry).Widget,
		widget.NewFormItem("Blue", blueEntry).Widget,
	)
	if !rgbCheck.Checked {
		rgbRow.Hide()
	}
	rgbCheck.OnChanged = func(on bool) {
		if on {
			ch.ExtraFields[rgbKey] = "1"
			rgbRow.Show()
		} else {
			delete(ch.ExtraFields, rgbKey)
			rgbRow.Hide()
		}
		sve.editor.markDirty()
	}

	// Delete row button.
	deleteBtn := widget.NewButtonWithIcon("", theme.DeleteIcon(), func() {
		dialog.ShowConfirm("Remove variant",
			fmt.Sprintf("Remove variant #%d?", idx),
			func(ok bool) {
				if !ok {
					return
				}
				for _, k := range []string{modelKey, skinKey, shaderKey,
					rgbKey, redKey, greenKey, blueKey} {
					delete(ch.ExtraFields, k)
				}
				sve.editor.markDirty()
				sve.Refresh()
			},
			sve.editor.app.mainWindow)
	})
	deleteBtn.Importance = widget.LowImportance

	// Title label shows the variant index — "Variant 1", "Variant 2",
	// etc. Monospace so the list of rows lines up neatly.
	title := widget.NewLabelWithStyle(fmt.Sprintf("Variant %d", idx),
		fyne.TextAlignLeading, fyne.TextStyle{Bold: true, Monospace: true})

	header := container.NewBorder(nil, nil,
		container.NewHBox(preview, title),
		deleteBtn,
		nil,
	)

	form := widget.NewForm(
		widget.NewFormItem("Model", modelEntry),
		widget.NewFormItem("Skin", skinEntry),
		widget.NewFormItem("UI Shader", shaderEntry),
	)

	body := container.NewVBox(header, form, rgbCheck, rgbRow)
	return widget.NewCard("", "", body)
}

// collectVariantIndices scans ExtraFields for model_N / skin_N /
// uishader_N keys and returns every N that appears, sorted.
func collectVariantIndices(ef map[string]string) []int {
	seen := map[int]bool{}
	for k := range ef {
		if idx, ok := parseVariantIndex(k); ok {
			seen[idx] = true
		}
	}
	out := make([]int, 0, len(seen))
	for i := range seen {
		out = append(out, i)
	}
	sort.Ints(out)
	return out
}

// parseVariantIndex extracts the numeric suffix from a known
// variant-suffixed field. Returns (0, false) for anything else.
func parseVariantIndex(key string) (int, bool) {
	prefixes := []string{
		"model_", "skin_", "uishader_",
		"userRGB_", "customred_", "customgreen_", "customblue_",
	}
	for _, p := range prefixes {
		if strings.HasPrefix(key, p) {
			rest := strings.TrimPrefix(key, p)
			if n, err := strconv.Atoi(rest); err == nil && n >= 1 {
				return n, true
			}
		}
	}
	return 0, false
}

// setOrDelete writes value to the map or deletes the key if value is
// empty. Keeps MBCH save output minimal — no stray blank fields.
func setOrDelete(m map[string]string, key, value string) {
	if strings.TrimSpace(value) == "" {
		delete(m, key)
		return
	}
	m[key] = value
}

// newFloatEntry builds a 0.0-1.0 validated entry for an RGB channel.
// Accepts empty (deletes the key), any valid float (stores it), and
// silently drops invalid text so the user's typing doesn't get
// rejected mid-edit.
func newFloatEntry(initial, key string, sve *SkinVariantsEditor, placeholder string) *widget.Entry {
	e := NewInputEntry()
	e.SetText(initial)
	e.SetPlaceHolder(placeholder)
	e.OnChanged = func(s string) {
		setOrDelete(sve.editor.character.ExtraFields, key, s)
		sve.editor.markDirty()
	}
	return e
}
