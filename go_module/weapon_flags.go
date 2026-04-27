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

	// Group flags by family — Reload/Damage/Status/CC/Disarm/
	// Movement/Utility — so authors can scan a section instead of a
	// 31-checkbox wall. Title row of each flag now leads with the
	// HELD_* enum (monospace) so the wiki ID is the primary
	// identifier; the short Name + the longer Tooltip render
	// underneath at smaller sizes.
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
		grid := container.NewGridWithColumns(2)
		for _, f := range flags {
			flag := f // capture
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

// buildHeldFlagCell renders one flag row inside a family group:
//   [✓] [glyph] HELD_ALTRELOAD       ← enum ID, monospace bold
//               Mag reload             ← short label, italic
//               Magazine-based …       ← description, dim
// Glyph is a 28px boxicon picked from the family (Reload→refresh,
// Damage→bolt, Status→flame, CC→wave, Disarm→swap, Movement→
// footstep, Utility→star). Pure decoration but gives each row a
// stronger visual identity than a wall of text + checkbox.
// The card is wrapped in a TilePanel that lights up the family
// color when the flag is active, so checked rows pop visually.
func buildHeldFlagCell(flag HeldFlag, active map[string]bool,
	ch *parsers.MBCHCharacter, flagsKey string, wfe *WeaponFlagsEditor) fyne.CanvasObject {
	checked := active[flag.ID]

	check := widget.NewCheck("", func(on bool) {
		set := parseFlags(ch.ExtraFields[flagsKey])
		if on {
			set[flag.ID] = true
		} else {
			delete(set, flag.ID)
		}
		ch.ExtraFields[flagsKey] = serializeFlags(set)
		// If the user unchecked everything, drop the field entirely
		// so round-trip save doesn't emit an empty WP_*Flags line.
		if ch.ExtraFields[flagsKey] == "" {
			delete(ch.ExtraFields, flagsKey)
		}
		wfe.editor.markDirty()
		// Inline refresh — fyne.Do from main thread deadlocked Fyne
		// v2.7.1's dispatch queue. Tree rebuild during the OnChanged
		// callback works in practice on this version.
		wfe.Refresh()
	})
	check.Checked = checked

	// Family glyph — 28px boxicon resource. None for "Other".
	var glyph fyne.CanvasObject = container.NewGridWrap(fyne.NewSize(28, 28))
	if name := heldFlagFamilyIcon(flag.Family); name != "" {
		if res := loadBoxiconResource(name); res != nil {
			glyph = NewRasterIconFromResource(res, 28, 28)
		}
	}

	idLbl := widget.NewLabelWithStyle(flag.ID,
		fyne.TextAlignLeading, fyne.TextStyle{Bold: true, Monospace: true})
	nameLbl := widget.NewLabelWithStyle(flag.Name,
		fyne.TextAlignLeading, fyne.TextStyle{Italic: true})
	descLbl := widget.NewLabel(flag.Tooltip)
	descLbl.Wrapping = fyne.TextWrapWord
	textStack := container.NewVBox(idLbl, nameLbl, descLbl)

	body := container.NewBorder(nil, nil,
		container.NewHBox(check, glyph),
		nil, textStack,
	)

	// Active state lights the cell with the family color so the user
	// can scan which flags are on in the wall of options. Inactive
	// stays low-contrast.
	fillA, strokeA := uint8(8), uint8(35)
	if checked {
		fillA, strokeA = 28, 110
	}
	return NewTilePanel(body, TileOpts{
		AccentColor: heldFlagFamilyAccent(flag.Family),
		FillAlpha:   fillA,
		StrokeAlpha: strokeA,
		Padded:      true,
	})
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
