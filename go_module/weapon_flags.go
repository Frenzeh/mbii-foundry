package main

// Weapon "held" flags editor — per-weapon HELD_* modifiers that MBII
// supports on any weapon the class can carry. Stored in the MBCH as
// `WP_NameFlags HELD_ALTRELOAD|HELD_STUN` type fields. The parser
// stashes these into ExtraFields since there's no dedicated struct
// field for them.
//
// UI: one row per weapon-flags field, with a grid of labeled
// checkboxes below the weapon's name. Toggling a checkbox rewrites
// the ExtraFields entry in canonical form (sorted, pipe-separated).
// Users can add rows for any weapon — the wiki explicitly notes
// "you can add weaponflag fields without actually granting the
// character the weapon", so we don't restrict the picker to the
// character's current weapon list.

import (
	"fmt"
	"image/color"
	"sort"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/Frenzeh/mbii-foundry/parsers"
)

// HeldFlag describes one HELD_* option: its enum name + a compact
// human-readable tooltip label. Sourced from the wiki's "weapon
// overrides HELD_" table.
type HeldFlag struct {
	ID      string // HELD_*
	Name    string // Short label shown next to the checkbox
	Tooltip string // What the flag does — shown on hover
	Family  string // Grouping bucket: "Reload", "Damage", "Status",
	// "CC", "Disarm", "Movement", "Utility", "R22"
}

// KnownHeldFlags lists every HELD_* the wiki documents. Kept in
// source order so the editor shows them in a logical grouping
// (reload/regen, damage mods, status effects, movement, etc.)
// rather than alphabetical, which would split related flags apart.
var KnownHeldFlags = []HeldFlag{
	{"HELD_ALTRELOAD", "Mag reload", "Magazine-based reload like WESTAR-M5", "Reload"},
	{"HELD_AMMOREGEN", "Ammo regen", "Regenerates ammo while held", "Reload"},
	{"HELD_HIGHDAMAGE", "2× damage", "100% more damage; also applies to force drains", "Damage"},
	{"HELD_LOWDAMAGE", "½ damage", "50% less damage", "Damage"},
	{"HELD_EXPLOSIVE", "Explosive", "Hit effects become an AoE explosion", "Damage"},
	{"HELD_DISRUPTIFY", "Disintegrates", "Targets vaporize on death", "Damage"},
	{"HELD_IGNOREBLOCK", "No block", "Ignores Blaster Defense", "Damage"},
	{"HELD_FLAME", "Ignites", "Brief burn effect on hit", "Status"},
	{"HELD_FREEZE", "Freezes", "Brief freeze on hit", "Status"},
	{"HELD_POISON", "Poisons", "Poison dart effect on hit (doesn't stack)", "Status"},
	{"HELD_PULSE", "Shocks", "Pulse grenade effect on hit (half drain)", "Status"},
	{"HELD_SONIC", "Stuns", "Brief sonic stun on hit", "Status"},
	{"HELD_STUN", "Staggers", "Gunbash-style stagger", "Status"},
	{"HELD_KNOCKBACK", "Pushes", "Knockback like Force Push 1", "CC"},
	{"HELD_KNOCKDOWN", "Trips", "Target knocked down on hit", "CC"},
	{"HELD_KNOCKDOWNRESISTANCE", "KD resist", "User resists incoming knockdowns", "CC"},
	{"HELD_HEAL", "Heal on hold", "User regenerates HP while active", "Utility"},
	{"HELD_SPEED", "+15% move", "User moves 15% faster while held", "Movement"},
	{"HELD_SLOW", "−15% move", "User moves 15% slower while held", "Movement"},
	{"HELD_SLOWPROJ", "−75% velocity", "Projectile moves at 25% speed", "Movement"},
	{"HELD_TRACKING", "Tracks", "Hit targets are visible to the user for 45s", "Utility"},
	{"HELD_LIFT", "Lifts", "R22.0.00: knocks target into the air on hit", "CC"},
	{"HELD_SLIPPERY", "Slippery", "R22.0.00: target slides/loses footing on hit", "CC"},
	{"HELD_DISARM", "Disarms", "R22.0.00: target's currently held weapon is dropped", "Disarm"},
	{"HELD_NODISARM", "Disarm-immune", "R22.0.00: weapon cannot be disarmed off the user", "Disarm"},
	{"HELD_PULL", "Pulls", "R22.0.00: drags hit target toward the firer", "CC"},
	{"HELD_CRIPPLE", "Cripples", "R22.0.00: brief slow-and-stagger debuff (movement + actions)", "CC"},
	{"HELD_FORCEFOCUS", "Force focus", "R22.0.00: hits restore Force Pool to the firer", "Utility"},
	{"HELD_LIFESTEAL", "Lifesteal", "R22.0.00: portion of damage dealt heals the firer", "Utility"},
	{"HELD_FLASH", "Flashes", "R22.0.00: Flashbang-style blind on hit", "Status"},
	{"HELD_BACTA", "Bacta heal", "R22.0.00: hit allies (or self) receive bacta-style heal-over-time", "Utility"},
}

// WeaponFlagTargets is the list of WP_* IDs that accept flags in
// practice — the live weapon enum minus sentinels + level objects.
// Sourced from MBIIWeapons (the canonical weapon catalog), unioned
// with weaponIconAliases (catches any WP_* that has an icon mapping
// but isn't yet in MBIIWeapons). Earlier this list was built only
// from weaponIconAliases — a subset — so weapons present in MBIIWeapons
// but lacking a custom HUD icon (e.g. WP_BLASTER_PISTOL) were silently
// missing from the Flags and Weapon Mods pickers.
var WeaponFlagTargets = func() []string {
	seen := map[string]bool{}
	for _, w := range MBIIWeapons {
		if w.ID == "" || w.ID == "WP_NONE" || w.Hidden {
			continue
		}
		seen[w.ID] = true
	}
	for id := range weaponIconAliases {
		if id == "" || id == "WP_NONE" {
			continue
		}
		seen[id] = true
	}
	out := make([]string, 0, len(seen))
	for id := range seen {
		out = append(out, id)
	}
	sort.Strings(out)
	return out
}()

// wpFlagsFieldName returns the canonical ExtraFields key for a
// weapon's flag field — the MBCH format uses "WP_NameFlags" where
// Name is title-cased with underscores preserved (the wiki's
// convention, e.g. WP_T21 → WP_T21Flags, WP_CLONE_PISTOL →
// WP_ClonePistolFlags). We don't normalize case at save time — the
// parser is case-insensitive on the key anyway — so feed the wiki's
// canonical form so diffs stay clean.
func wpFlagsFieldName(wpID string) string {
	suffix := strings.TrimPrefix(wpID, "WP_")
	return "WP_" + titleCaseFlagSuffix(suffix) + "Flags"
}

// titleCaseFlagSuffix turns "CLONE_PISTOL" into "ClonePistol" —
// matches MBII's wiki convention for WP_*Flags field names. Kept
// separate from general string.Title because we need underscore-
// separated-words collapsed, which Title doesn't do.
func titleCaseFlagSuffix(s string) string {
	words := strings.Split(strings.ToLower(s), "_")
	for i, w := range words {
		if w == "" {
			continue
		}
		words[i] = strings.ToUpper(w[:1]) + w[1:]
	}
	return strings.Join(words, "")
}

// parseFlags parses the pipe-separated CSV ("HELD_STUN|HELD_FLAME")
// into a set. Tolerates whitespace and empty segments so manual
// hand-edits don't break the UI.
func parseFlags(csv string) map[string]bool {
	out := map[string]bool{}
	for _, part := range strings.Split(csv, "|") {
		part = strings.TrimSpace(part)
		if part != "" {
			out[part] = true
		}
	}
	return out
}

// serializeFlags returns the pipe-separated canonical form with
// flags sorted alphabetically — deterministic output so round-trip
// save-reopen-save doesn't produce churn in diffs.
func serializeFlags(set map[string]bool) string {
	keys := make([]string, 0, len(set))
	for k, on := range set {
		if on {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)
	return strings.Join(keys, "|")
}

// WeaponFlagsEditor is the composite widget that renders a vertical
// list of weapon-flag rows plus an "add weapon flags" button. Owns
// a pointer to the editor's character so it can read + write
// ExtraFields directly; all mutations mark the editor dirty.
type WeaponFlagsEditor struct {
	editor *MBCHEditor

	container *fyne.Container
	listBox   *fyne.Container
}

func NewWeaponFlagsEditor(editor *MBCHEditor) *WeaponFlagsEditor {
	wfe := &WeaponFlagsEditor{editor: editor}
	wfe.createUI()
	return wfe
}

func (wfe *WeaponFlagsEditor) createUI() {
	wfe.listBox = container.NewVBox()

	addBtn := widget.NewButtonWithIcon("Add Weapon Flags...", theme.ContentAddIcon(), func() {
		wfe.showAddDialog()
	})

	classFlagsTile := wfe.buildClassFlagsSection()

	weaponFlagsHeader := widget.NewLabelWithStyle("Weapon Held Flags (WP_*Flags)", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	weaponFlagsSub := widget.NewLabelWithStyle("Per-weapon combat modifiers (HELD_*) applied when holding the specific weapon.", fyne.TextAlignLeading, fyne.TextStyle{Italic: true})

	wfe.container = container.NewVBox(
		classFlagsTile,
		widget.NewSeparator(),
		container.NewBorder(nil, nil, container.NewVBox(weaponFlagsHeader, weaponFlagsSub), addBtn),
		wfe.listBox,
	)
}

// GetContent returns the root widget for embedding in a tab.
func (wfe *WeaponFlagsEditor) GetContent() fyne.CanvasObject {
	return wfe.container
}

// Refresh rebuilds the row list from the character's ExtraFields and ClassFlags.
func (wfe *WeaponFlagsEditor) Refresh() {
	if wfe.container == nil {
		wfe.createUI()
	}
	wfe.createUI()

	ch := wfe.editor.character
	if ch.ExtraFields == nil {
		ch.ExtraFields = map[string]string{}
	}

	keys := make([]string, 0, len(ch.ExtraFields))
	for k := range ch.ExtraFields {
		if isWeaponFlagsField(k) {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)

	for _, k := range keys {
		wfe.listBox.Add(wfe.buildRow(k))
	}
	if wfe.container != nil {
		wfe.container.Refresh()
	}
}

func buildCheckmarkPip(checked bool, accent color.Color) fyne.CanvasObject {
	if checked {
		circle := canvas.NewCircle(accent)
		icon := widget.NewIcon(theme.ConfirmIcon())
		return container.NewStack(
			container.NewGridWrap(fyne.NewSize(22, 22), circle),
			container.NewGridWrap(fyne.NewSize(18, 18), icon),
		)
	}
	ring := canvas.NewCircle(color.NRGBA{R: 70, G: 75, B: 85, A: 255})
	inner := canvas.NewCircle(color.NRGBA{R: 28, G: 30, B: 36, A: 255})
	return container.NewStack(
		container.NewGridWrap(fyne.NewSize(22, 22), ring),
		container.NewGridWrap(fyne.NewSize(16, 16), inner),
	)
}

func (wfe *WeaponFlagsEditor) buildClassFlagsSection() fyne.CanvasObject {
	ch := wfe.editor.character
	activeFlags := map[string]bool{}
	if ch.ClassFlags != "" {
		for _, f := range strings.Split(ch.ClassFlags, "|") {
			activeFlags[strings.TrimSpace(f)] = true
		}
	}

	header := widget.NewLabelWithStyle("Class Flags (CFL_*)", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	sub := widget.NewLabelWithStyle("Inherent character traits, physical attributes, and passive immunities.", fyne.TextAlignLeading, fyne.TextStyle{Italic: true})

	grid := container.NewGridWrap(fyne.NewSize(275, 54))
	for _, flag := range GetClassFlags() {
		f := flag
		checked := activeFlags[f.ID]
		accent := color.NRGBA{R: 100, G: 190, B: 240, A: 255}

		toggle := func() {
			current := map[string]bool{}
			if ch.ClassFlags != "" {
				for _, cf := range strings.Split(ch.ClassFlags, "|") {
					current[strings.TrimSpace(cf)] = true
				}
			}
			if checked {
				delete(current, f.ID)
			} else {
				current[f.ID] = true
			}
			var list []string
			for k, on := range current {
				if on {
					list = append(list, k)
				}
			}
			sort.Strings(list)
			ch.ClassFlags = strings.Join(list, "|")
			if wfe.editor.classFlagsSelect != nil {
				wfe.editor.classFlagsSelect.SetSelected(ch.ClassFlags)
			}
			wfe.editor.markDirty()
			wfe.Refresh()
		}

		pip := buildCheckmarkPip(checked, accent)

		idLbl := widget.NewLabelWithStyle(f.ID,
			fyne.TextAlignLeading, fyne.TextStyle{Bold: true, Monospace: true})
		nameLbl := widget.NewLabelWithStyle(f.Name,
			fyne.TextAlignLeading, fyne.TextStyle{Italic: true})
		descLbl := widget.NewLabel(f.Description)
		descLbl.Wrapping = fyne.TextWrapWord

		textStack := container.NewVBox(
			container.NewHBox(idLbl, nameLbl),
			descLbl,
		)

		rowContent := container.NewBorder(nil, nil,
			container.NewCenter(pip),
			nil,
			textStack,
		)

		fillA, strokeA := uint8(8), uint8(35)
		if checked {
			fillA, strokeA = 26, 100
		}

		tile := NewTilePanel(rowContent, TileOpts{
			AccentColor: accent,
			FillAlpha:   fillA,
			StrokeAlpha: strokeA,
			Padded:      true,
		})

		grid.Add(newClickableCell(tile, toggle))
	}

	return NewTilePanel(
		container.NewVBox(header, sub, grid),
		TileOpts{
			AccentColor: color.NRGBA{R: 100, G: 190, B: 240, A: 255},
			FillAlpha:   14,
			StrokeAlpha: 50,
			Padded:      true,
		},
	)
}

// buildRow renders one weapon-flags row.
func (wfe *WeaponFlagsEditor) buildRow(flagsKey string) fyne.CanvasObject {
	ch := wfe.editor.character
	wpID := weaponIDFromFlagsKey(flagsKey)

	// Header: icon + weapon name + delete button.
	title := widget.NewLabelWithStyle(flagsKey,
		fyne.TextAlignLeading, fyne.TextStyle{Bold: true, Monospace: true})

	var iconObj fyne.CanvasObject = layout_newSpacer()
	if alias, ok := weaponIconAliases[wpID]; ok && alias != "" {
		if img, ok2 := LoadGameIcon(nil, "gfx/hud/"+alias); ok2 {
			iconObj = NewRasterIconFromResource(
				staticPNGResource(alias+".png", img), 28, 28,
			)
		}
	}

	deleteBtn := widget.NewButtonWithIcon("", theme.DeleteIcon(), func() {
		delete(ch.ExtraFields, flagsKey)
		wfe.editor.markDirty()
		wfe.Refresh()
	})
	deleteBtn.Importance = widget.LowImportance

	header := container.NewBorder(nil, nil,
		container.NewHBox(iconObj, title),
		deleteBtn,
		nil,
	)

	active := parseFlags(ch.ExtraFields[flagsKey])

	familyOrder := []string{"Reload", "Damage", "Status", "CC", "Disarm", "Movement", "Utility"}
	byFamily := map[string][]HeldFlag{}
	for _, f := range KnownHeldFlags {
		fam := f.Family
		if fam == "" {
			fam = "Utility"
		}
		byFamily[fam] = append(byFamily[fam], f)
	}

	body := container.NewVBox(header)
	for _, fam := range familyOrder {
		flags, ok := byFamily[fam]
		if !ok || len(flags) == 0 {
			continue
		}
		famHeader := widget.NewLabelWithStyle(
			fmt.Sprintf("%s  ·  %d", fam, len(flags)),
			fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
		grid := container.NewGridWrap(fyne.NewSize(275, 56))
		for _, f := range flags {
			flag := f
			grid.Add(buildHeldFlagCell(flag, active, ch, flagsKey, wfe))
		}
		body.Add(NewTilePanel(
			container.NewVBox(famHeader, grid),
			TileOpts{
				AccentColor: heldFlagFamilyAccent(fam),
				FillAlpha:   18,
				StrokeAlpha: 60,
				Padded:      true,
			},
		))
	}

	card := widget.NewCard("", "", body)
	return card
}

// buildHeldFlagCell renders one flag row inside a family group.
func buildHeldFlagCell(flag HeldFlag, active map[string]bool,
	ch *parsers.MBCHCharacter, flagsKey string, wfe *WeaponFlagsEditor) fyne.CanvasObject {
	checked := active[flag.ID]
	accent := heldFlagFamilyAccent(flag.Family)

	toggle := func() {
		set := parseFlags(ch.ExtraFields[flagsKey])
		if checked {
			delete(set, flag.ID)
		} else {
			set[flag.ID] = true
		}
		ch.ExtraFields[flagsKey] = serializeFlags(set)
		if ch.ExtraFields[flagsKey] == "" {
			delete(ch.ExtraFields, flagsKey)
		}
		wfe.editor.markDirty()
		wfe.Refresh()
	}

	pip := buildCheckmarkPip(checked, accent)

	var glyph fyne.CanvasObject = container.NewGridWrap(fyne.NewSize(24, 24))
	if name := heldFlagFamilyIcon(flag.Family); name != "" {
		if res := loadBoxiconResource(name); res != nil {
			glyph = NewRasterIconFromResource(res, 24, 24)
		}
	}

	idLbl := widget.NewLabelWithStyle(flag.ID,
		fyne.TextAlignLeading, fyne.TextStyle{Bold: true, Monospace: true})
	nameLbl := widget.NewLabelWithStyle(flag.Name,
		fyne.TextAlignLeading, fyne.TextStyle{Italic: true})
	descLbl := widget.NewLabel(flag.Tooltip)
	descLbl.Wrapping = fyne.TextWrapWord
	textStack := container.NewVBox(
		container.NewHBox(idLbl, nameLbl),
		descLbl,
	)

	body := container.NewBorder(nil, nil,
		container.NewHBox(container.NewCenter(pip), container.NewCenter(glyph)),
		nil, textStack,
	)

	fillA, strokeA := uint8(8), uint8(35)
	if checked {
		fillA, strokeA = 28, 110
	}
	tile := NewTilePanel(body, TileOpts{
		AccentColor: accent,
		FillAlpha:   fillA,
		StrokeAlpha: strokeA,
		Padded:      true,
	})

	return newClickableCell(tile, toggle)
}

// heldFlagFamilyIcon picks the boxicon basename that visually
// represents the family. None for "Other".
func heldFlagFamilyIcon(family string) string {
	switch family {
	case "Reload":
		return "refresh"
	case "Damage":
		return "bolt"
	case "Status":
		return "flame"
	case "CC":
		return "wave"
	case "Disarm":
		return "swap"
	case "Movement":
		return "footstep"
	case "Utility":
		return "star"
	}
	return ""
}

// heldFlagFamilyAccent assigns a per-family accent color so each
// section reads distinctly without the eye having to parse 30
// checkbox labels uniformly. Tuned for the dark theme.
func heldFlagFamilyAccent(family string) color.Color {
	switch family {
	case "Reload":
		return color.NRGBA{R: 110, G: 200, B: 220, A: 255}
	case "Damage":
		return color.NRGBA{R: 220, G: 110, B: 110, A: 255}
	case "Status":
		return color.NRGBA{R: 220, G: 180, B: 100, A: 255}
	case "CC":
		return color.NRGBA{R: 200, G: 130, B: 200, A: 255}
	case "Disarm":
		return color.NRGBA{R: 230, G: 130, B: 90, A: 255}
	case "Movement":
		return color.NRGBA{R: 140, G: 200, B: 140, A: 255}
	case "Utility":
		return color.NRGBA{R: 160, G: 180, B: 220, A: 255}
	}
	return color.NRGBA{R: 160, G: 160, B: 170, A: 255}
}

// showAddDialog prompts the user to pick a weapon to add flags for.
// Creates an empty WP_*Flags entry so the row appears in the list,
// with no flags checked.
func (wfe *WeaponFlagsEditor) showAddDialog() {
	// Offer only weapons that don't already have a flags row.
	existing := map[string]bool{}
	for k := range wfe.editor.character.ExtraFields {
		if isWeaponFlagsField(k) {
			existing[weaponIDFromFlagsKey(k)] = true
		}
	}
	var options []string
	for _, wp := range WeaponFlagTargets {
		if !existing[wp] {
			options = append(options, wp)
		}
	}
	if len(options) == 0 {
		dialog.ShowInformation("No weapons to add",
			"Every live weapon already has a flags row. Delete one to re-add.",
			wfe.editor.app.mainWindow)
		return
	}

	wpNames := map[string]string{}
	for _, w := range MBIIWeapons {
		wpNames[w.ID] = w.Name
	}

	var d dialog.Dialog
	grid := container.NewGridWrap(fyne.NewSize(140, 75))

	populateGrid := func(filter string) {
		grid.Objects = nil
		filter = strings.ToLower(strings.TrimSpace(filter))
		for _, wp := range options {
			name := wpNames[wp]
			if name == "" {
				name = strings.TrimPrefix(wp, "WP_")
			}
			if filter != "" && !strings.Contains(strings.ToLower(wp), filter) && !strings.Contains(strings.ToLower(name), filter) {
				continue
			}

			var iconObj fyne.CanvasObject
			if alias, ok := weaponIconAliases[wp]; ok && alias != "" {
				if img, ok2 := LoadGameIcon(nil, "gfx/hud/"+alias); ok2 {
					iconObj = NewRasterIconFromResource(
						staticPNGResource(alias+".png", img), 28, 28,
					)
				}
			}
			if iconObj == nil {
				iconObj = widget.NewIcon(theme.FileImageIcon())
			}

			lblTitle := canvas.NewText(name, color.NRGBA{R: 240, G: 240, B: 245, A: 255})
			lblTitle.TextSize = 11
			lblTitle.TextStyle = fyne.TextStyle{Bold: true}
			lblTitle.Alignment = fyne.TextAlignCenter

			lblID := canvas.NewText(wp, color.NRGBA{R: 140, G: 140, B: 150, A: 255})
			lblID.TextSize = 9
			lblID.TextStyle = fyne.TextStyle{Monospace: true}
			lblID.Alignment = fyne.TextAlignCenter

			cardBg := canvas.NewRectangle(color.NRGBA{R: 35, G: 40, B: 48, A: 240})
			cardBg.StrokeColor = color.NRGBA{R: 70, G: 80, B: 95, A: 255}
			cardBg.StrokeWidth = 1
			cardBg.CornerRadius = 6

			cardContent := container.NewVBox(
				container.NewCenter(iconObj),
				lblTitle,
				lblID,
			)

			targetWP := wp
			clickable := newClickableCell(container.NewStack(cardBg, container.NewPadded(cardContent)), func() {
				key := wpFlagsFieldName(targetWP)
				if wfe.editor.character.ExtraFields == nil {
					wfe.editor.character.ExtraFields = map[string]string{}
				}
				wfe.editor.character.ExtraFields[key] = ""
				wfe.editor.markDirty()
				wfe.Refresh()
				if d != nil {
					d.Hide()
				}
			})

			grid.Add(clickable)
		}
		grid.Refresh()
	}

	searchEntry := NewInputEntry()
	searchEntry.SetPlaceHolder("Filter weapons…")
	searchEntry.OnChanged = func(s string) {
		populateGrid(s)
	}

	populateGrid("")

	scroll := container.NewVScroll(grid)
	scroll.SetMinSize(fyne.NewSize(460, 320))

	dlgContent := container.NewBorder(
		container.NewVBox(
			widget.NewLabelWithStyle("Select a weapon to customize weapon flags:", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
			searchEntry,
		),
		nil, nil, nil,
		scroll,
	)

	d = dialog.NewCustom("Add Weapon Flags", "Cancel", dlgContent, wfe.editor.app.mainWindow)
	d.Resize(fyne.NewSize(480, 380))
	d.Show()
}

// isWeaponFlagsField recognizes any ExtraFields key that's a
// WP_*Flags entry. Uses a lenient substring check since MBCH is
// case-insensitive on keys (the wiki uses "WP_T21Flags" but a
// hand-written file might have "wp_t21flags").
func isWeaponFlagsField(key string) bool {
	lower := strings.ToLower(key)
	return strings.HasPrefix(lower, "wp_") && strings.HasSuffix(lower, "flags")
}

// weaponIDFromFlagsKey turns "WP_T21Flags" → "WP_T21". Used for
// looking up the weapon's icon + display name when rendering a row.
// Returns the original key on failure so the row still shows rather
// than silently dropping an unrecognized field.
func weaponIDFromFlagsKey(key string) string {
	if !isWeaponFlagsField(key) {
		return key
	}
	stripped := key[:len(key)-len("Flags")]
	// The MBCH convention is "WP_<TitleCasedName>Flags". We don't
	// reliably recover the original UPPER_UNDERSCORE enum from the
	// title-cased form (WP_ClonePistol → WP_CLONE_PISTOL needs
	// knowledge of where words split). Cross-reference against
	// weaponIconAliases so we pick the canonical enum form when
	// possible; fall back to the title-cased id upcase for display.
	for id := range weaponIconAliases {
		if strings.EqualFold(stripped, idToFlagsStem(id)) {
			return id
		}
	}
	return strings.ToUpper(stripped)
}

// idToFlagsStem turns "WP_CLONE_PISTOL" → "WP_ClonePistol" so we can
// match WP_ClonePistolFlags back to the canonical enum ID.
func idToFlagsStem(id string) string {
	return "WP_" + titleCaseFlagSuffix(strings.TrimPrefix(id, "WP_"))
}

// layout_newSpacer is a tiny helper so buildRow stays readable —
// Fyne's layout.NewSpacer() is just noise inline with a big icon
// resolution block.
func layout_newSpacer() fyne.CanvasObject {
	return widget.NewLabel("")
}
