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
	"sort"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// HeldFlag describes one HELD_* option: its enum name + a compact
// human-readable tooltip label. Sourced from the wiki's "weapon
// overrides HELD_" table.
type HeldFlag struct {
	ID      string // HELD_*
	Name    string // Short label shown next to the checkbox
	Tooltip string // What the flag does — shown on hover
}

// KnownHeldFlags lists every HELD_* the wiki documents. Kept in
// source order so the editor shows them in a logical grouping
// (reload/regen, damage mods, status effects, movement, etc.)
// rather than alphabetical, which would split related flags apart.
var KnownHeldFlags = []HeldFlag{
	{"HELD_ALTRELOAD", "Mag reload", "Magazine-based reload like WESTAR-M5"},
	{"HELD_AMMOREGEN", "Ammo regen", "Regenerates ammo while held"},
	{"HELD_HIGHDAMAGE", "2× damage", "100% more damage; also applies to force drains"},
	{"HELD_LOWDAMAGE", "½ damage", "50% less damage"},
	{"HELD_EXPLOSIVE", "Explosive", "Hit effects become an AoE explosion"},
	{"HELD_DISRUPTIFY", "Disintegrates", "Targets vaporize on death"},
	{"HELD_FLAME", "Ignites", "Brief burn effect on hit"},
	{"HELD_FREEZE", "Freezes", "Brief freeze on hit"},
	{"HELD_POISON", "Poisons", "Poison dart effect on hit (doesn't stack)"},
	{"HELD_PULSE", "Shocks", "Pulse grenade effect on hit (half drain)"},
	{"HELD_SONIC", "Stuns", "Brief sonic stun on hit"},
	{"HELD_STUN", "Staggers", "Gunbash-style stagger"},
	{"HELD_KNOCKBACK", "Pushes", "Knockback like Force Push 1"},
	{"HELD_KNOCKDOWN", "Trips", "Target knocked down on hit"},
	{"HELD_KNOCKDOWNRESISTANCE", "KD resist", "User resists incoming knockdowns"},
	{"HELD_IGNOREBLOCK", "No block", "Ignores Blaster Defense"},
	{"HELD_HEAL", "Heal on hold", "User regenerates HP while active"},
	{"HELD_SPEED", "+15% move", "User moves 15% faster while held"},
	{"HELD_SLOW", "−15% move", "User moves 15% slower while held"},
	{"HELD_SLOWPROJ", "−75% velocity", "Projectile moves at 25% speed"},
	{"HELD_TRACKING", "Tracks", "Hit targets are visible to the user for 45s"},
}

// WeaponFlagTargets is the list of WP_* IDs that accept flags in
// practice — the live weapon enum minus sentinels + level objects.
// Built at package init from weaponIconAliases so it stays in sync
// with the icon-coverage set. The flags UI uses this for its "add
// weapon" picker.
var WeaponFlagTargets = func() []string {
	out := make([]string, 0, len(weaponIconAliases))
	for id := range weaponIconAliases {
		if id == "WP_NONE" {
			continue
		}
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

	addBtn := widget.NewButtonWithIcon("Add weapon flags", theme.ContentAddIcon(), func() {
		wfe.showAddDialog()
	})

	wfe.container = container.NewVBox(
		container.NewPadded(addBtn),
		wfe.listBox,
	)
}

// GetContent returns the root widget for embedding in a tab.
func (wfe *WeaponFlagsEditor) GetContent() fyne.CanvasObject {
	return wfe.container
}

// Refresh rebuilds the row list from the character's ExtraFields.
// Called on file load and after add/remove mutations.
func (wfe *WeaponFlagsEditor) Refresh() {
	wfe.listBox.Objects = nil

	ch := wfe.editor.character
	if ch.ExtraFields == nil {
		ch.ExtraFields = map[string]string{}
	}

	// Collect every key that looks like a WP_*Flags field. Sorted
	// so the list is stable across refreshes (Go map iteration is
	// random otherwise).
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
	wfe.listBox.Refresh()
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

	// Active-flag set for this weapon.
	active := parseFlags(ch.ExtraFields[flagsKey])

	// Checkbox grid — 3 columns keeps each row readable without
	// making the editor a mile long. The visual weight of each
	// checkbox + label line is about the same as one text-entry
	// row, so 3 across fits nicely alongside the weapon grid.
	grid := container.NewGridWithColumns(3)
	for _, f := range KnownHeldFlags {
		flag := f // capture
		check := widget.NewCheck(flag.Name, func(on bool) {
			set := parseFlags(ch.ExtraFields[flagsKey])
			if on {
				set[flag.ID] = true
			} else {
				delete(set, flag.ID)
			}
			ch.ExtraFields[flagsKey] = serializeFlags(set)
			// If the user unchecked everything, drop the field
			// entirely so round-trip save doesn't emit an empty
			// WP_*Flags line.
			if ch.ExtraFields[flagsKey] == "" {
				delete(ch.ExtraFields, flagsKey)
			}
			wfe.editor.markDirty()
		})
		check.Checked = active[flag.ID]
		// Tooltip-ish hint: append effect summary in a dim label
		// next to the checkbox so users don't have to memorize the
		// HELD_* enum's semantics.
		hint := widget.NewLabel(flag.Tooltip)
		hint.TextStyle = fyne.TextStyle{Italic: true}
		row := container.NewBorder(nil, nil, check, nil, hint)
		grid.Add(row)
	}

	card := widget.NewCard("", "", container.NewVBox(header, grid))
	return card
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

	sel := widget.NewSelect(options, nil)
	sel.PlaceHolder = "WP_..."
	dialog.ShowCustomConfirm("Add weapon flags", "Add", "Cancel",
		container.NewVBox(widget.NewLabel("Which weapon?"), sel),
		func(ok bool) {
			if !ok || sel.Selected == "" {
				return
			}
			key := wpFlagsFieldName(sel.Selected)
			if wfe.editor.character.ExtraFields == nil {
				wfe.editor.character.ExtraFields = map[string]string{}
			}
			// Empty flag set by default — the row appears so the
			// user can start checking boxes. Delete-empty logic
			// above strips the field on save if no flags get
			// picked, so this doesn't pollute the MBCH.
			wfe.editor.character.ExtraFields[key] = ""
			wfe.editor.markDirty()
			wfe.Refresh()
		},
		wfe.editor.app.mainWindow,
	)
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
