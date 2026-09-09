package main

import (
	"fmt"
	"image/color"
	"regexp"
	"strconv"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
	"github.com/Frenzeh/mbii-foundry/parsers"
)

// Canonical mappings from engine symbols to clean, natural in-game Movie Battles II display names.
var CanonicalWeaponNames = map[string]string{
	"WP_MELEE":           "Melee",
	"WP_SABER":           "Lightsaber",
	"WP_STUN_BATON":      "Stun Baton",
	"WP_BRYAR_PISTOL":    "Bryar Pistol",
	"WP_BLASTER_PISTOL":  "Blaster Pistol",
	"WP_CLONE_PISTOL":    "Clone Pistol",
	"WP_MANDO_PISTOL":    "WESTAR-34 Pistols",
	"WP_CR2":             "CR-2 Heavy Blaster",
	"WP_HEAVY_PISTOL":    "Heavy Blaster Pistol",
	"WP_BRYAR_OLD":       "Old Bryar Pistol",
	"WP_BLASTER":         "E-11 Blaster",
	"WP_A280":            "A280 Rifle",
	"WP_CLONE_RIFLE":     "Clone Rifle",
	"WP_M5":              "Westar M5",
	"WP_T21":             "T-21 Heavy Blaster",
	"WP_E_22":            "E-22 Blaster",
	"WP_DLT19":           "DLT-19 Heavy Blaster",
	"WP_DLT20A":          "DLT-20A Blaster",
	"WP_EE3":             "EE-3 Carbine",
	"WP_EE4":             "EE-4 Carbine",
	"WP_DISRUPTOR":       "Disruptor Rifle",
	"WP_BOWCASTER":       "Bowcaster",
	"WP_TRAD_BOWCASTER":  "Traditional Bowcaster",
	"WP_REPEATER":        "Heavy Repeater",
	"WP_DEMP2":           "DEMP 2",
	"WP_FLECHETTE":       "Golan Arms Flechette",
	"WP_CONCUSSION":      "Concussion Rifle",
	"WP_SHOTGUN":         "CP-50 Repeater",
	"WP_AMBAN":           "Amban Rifle",
	"WP_PROJ":            "Projectile Rifle",
	"WP_ROCKET_LAUNCHER": "PLX-1 Rocket Launcher",
	"WP_PLX1":            "PLX-1 Missile Launcher",
	"WP_THROWER":         "Flamethrower",
	"WP_MINIGUN":         "Rotary Cannon",
	"WP_SBD":             "Arm Blaster",
	"WP_UGL":             "Universal Grenade Launcher",
	"WP_MGL":             "Micro Grenade Launcher",
	"WP_EQUALIZER":       "Equalizer MiniMag",
	"WP_THERMAL":         "Thermal Detonator",
	"WP_REAL_TD":         "Thermal Detonator",
	"WP_FRAG_NADE":       "Frag Grenade",
	"WP_FIRE_NADE":       "Fire Grenade",
	"WP_PULSE_NADE":      "Pulse Grenade",
	"WP_SONIC_NADE":      "Sonic Detonator",
	"WP_CRYO_NADE":       "Cryoban Grenade",
	"WP_CONC_NADE":       "Concussion Grenade",
	"WP_TRIP_MINE":       "Trip Mine",
	"WP_DET_PACK":        "Detonation Pack",
}

var CanonicalForceNames = map[string]string{
	"FP_HEAL":          "Force Heal",
	"FP_LEVITATION":    "Force Jump",
	"FP_SPEED":         "Force Speed",
	"FP_PUSH":          "Force Push",
	"FP_PULL":          "Force Pull",
	"FP_TELEPATHY":     "Mind Trick",
	"FP_GRIP":          "Force Grip",
	"FP_LIGHTNING":     "Force Lightning",
	"FP_RAGE":          "Force Destruction / Rage",
	"FP_PROTECT":       "Force Protect",
	"FP_ABSORB":        "Force Absorb",
	"FP_TEAM_HEAL":     "Team Heal",
	"FP_TEAM_FORCE":    "Team Energize",
	"FP_DRAIN":         "Force Drain",
	"FP_SEE":           "Force Sense",
	"FP_SABER_OFFENSE": "Saber Offense",
	"FP_SABER_DEFENSE": "Saber Defense",
	"FP_SABERTHROW":    "Saber Throw",
	"FP_BLIND":         "Force Blinding",
	"FP_DESTRUCTION":   "Force Destruction",
	"FP_DEADLYSIGHT":   "Deadly Sight",
	"FP_REPULSE":       "Force Repulse",
}

var CanonicalHoldableNames = map[string]string{
	"HI_MEDPAC":     "Bacta Canister",
	"HI_MEDPAC_BIG": "Large Bacta Canister",
	"HI_BINOCULARS": "Electrobinoculars",
	"HI_SENTRY_GUN": "Sentry Gun",
	"HI_JETPACK":    "Jetpack",
	"HI_HEALTHDISP": "Health Dispenser",
	"HI_AMMODISP":   "Ammo Dispenser",
	"HI_EWEB":       "Portable E-Web",
	"HI_CLOAK":      "Cloaking Device",
	"HI_SEEKER":     "Seeker Drone",
	"HI_SHIELD":     "Portable Forcefield",
	"HI_STIMPACK":   "Stimpack",
}

var CanonicalAttributeNames = map[string]string{
	"MB_ATT_MAGNETIC_PLATING": "Magnetic Plating",
	"MB_ATT_BLAST_ARMOUR":     "Blast Armour",
	"MB_ATT_CORTOSIS":         "Cortosis",
	"MB_ATT_BESKAR":           "Beskar Armour",
	"MB_ATT_ASSEMBLE":         "Assemble",
	"MB_ATT_DODGE":            "Dodge",
	"MB_ATT_DASH":             "Dash",
	"MB_ATT_RALLY":            "Rally",
	"MB_ATT_HEALING":          "Healing",
	"MB_ATT_STAMINA":          "Stamina",
	"MB_ATT_DEXTERITY":        "Dexterity",
	"MB_ATT_RESPAWNS":         "Reinforcements",
	"MB_ATT_ARMOUR":           "Armor",
	"MB_ATT_AMMO":             "Ammo",
	"MB_ATT_POISON_DART":      "Poison Darts",
	"MB_ATT_TRACKING_DART":    "Tracking Darts",
	"MB_ATT_JETPACK":          "Jetpack",
	"MB_ATT_FUEL":             "Fuel",
	"MB_ATT_FLAMETHROWER":     "Wrist Flamethrower",
	"MB_ATT_WRISTLASER":       "Wrist Laser",
	"MB_ATT_ROCKET":           "Wrist Rocket",
	"MB_ATT_WOOKIEESTRENGTH":  "Wookiee Strength",
	"MB_ATT_WOOKIEEFURY":      "Wookiee Fury",
	"MB_ATT_SBD_BATTERY":      "Battery",
	"MB_ATT_SBD_RECHARGE":     "Recharge",
	"MB_ATT_RECHARGE":         "Armor Recharge",
	"MB_ATT_FORCEBLOCK":       "Force Block",
	"MB_ATT_DEFLECT":          "Saber Deflect",
	"MB_ATT_RADAR":            "Radar",
	"MB_ATT_GRAPPLE_HOOK":     "Grappling Hook",
	"MB_ATT_CLONERIFLE":       "DC-15A Training",
	"MB_ATT_WESTARM5":         "Westar M5 Training",
	"MB_ATT_STRONGBLOX":       "Ion / Concussion Blobs",
	"MB_ATT_CLONE_BLOBS":      "Concussion Blobs",
	"MB_ATT_ARC_RIFLE_GREN":   "Westar M5 Grenade Launcher",
	"MB_ATT_ARC_RIFLE_SCOPE":  "Westar M5 Scope",
	"MB_ATT_UGL":              "Universal Grenade Launcher",
	"MB_ATT_UGL_BURST":        "UGL (Burst)",
	"MB_ATT_UGL_IMPACT":       "UGL (Impact)",
	"MB_ATT_UGL_BURST_MIXED":  "UGL (Mixed Burst)",
	"MB_ATT_MGL":              "Micro Grenade Launcher",
	"MB_ATT_MGL_BURST":        "MGL (Burst)",
	"MB_ATT_MGL_IMPACT":       "MGL (Impact)",
	"MB_ATT_FRAGS":            "Frag Grenades",
	"MB_ATT_FIRE_GRENADES":    "Fire Grenades",
	"MB_ATT_PULSE_GRENADES":   "Pulse Grenades",
	"MB_ATT_SONIC_DETONATOR":  "Sonic Detonators",
	"MB_ATT_CRYOBAN_GRENADES": "Cryoban Grenades",
	"MB_ATT_MICRO_GRENADES":   "Concussion Grenades",
	"MB_ATT_THERMALS":         "Thermal Detonators",
	"MB_ATT_TRIP_MINES":       "Trip Mines",
	"MB_ATT_DET_PACK":         "Detonation Packs",
	"MB_ATT_DISP_HEALTH":      "Health Dispenser",
	"MB_ATT_DISP_AMMO":        "Ammo Dispenser",
	"MB_ATT_DISP_ARMOR":       "Armor Dispenser",
	"MB_ATT_SHIELD_RECHARGE":  "Shield Recharge",
	"MB_ATT_BUNNY_HOP":        "Bunny Hop",
	"MB_ATT_ANTI_MT":          "Mind Trick Immunity",
	"MB_ATT_BLAST":            "Blast Armour",
	"MB_ATT_SABER_DEFENSE":    "Saber Defense",
	"MB_ATT_SABER_OFFENSE":    "Saber Offense",
	"MB_ATT_ZOOM":             "Binoculars Zoom",
}

// GenerateStandardDescription creates a standardized, authentic MBII description
// block based on the character's active loadout.
func GenerateStandardDescription(ch *parsers.MBCHCharacter) string {
	if ch == nil {
		return ""
	}

	var sb strings.Builder

	// Header: Name
	if ch.Name != "" {
		sb.WriteString(ch.Name)
		sb.WriteString("\n\n")
	}

	// 1. Weaponry (^2)
	var weaponLines []string
	if ch.Weapons != "" {
		wpTokens := strings.Split(ch.Weapons, "|")
		for _, tok := range wpTokens {
			tok = strings.TrimSpace(tok)
			if tok == "" || tok == "0" || tok == "WP_NONE" {
				continue
			}
			name := formatWeaponDisplayName(tok)
			weaponLines = append(weaponLines, fmt.Sprintf("%s", name))
		}
	}
	if len(weaponLines) > 0 {
		sb.WriteString("^2Weaponry:\n")
		for _, line := range weaponLines {
			sb.WriteString(line)
			sb.WriteString("\n")
		}
		sb.WriteString("\n")
	}

	// 2. Force Powers (^5)
	var forceLines []string
	if ch.ForcePowers != "" {
		fpTokens := strings.Split(ch.ForcePowers, "|")
		for _, tok := range fpTokens {
			tok = strings.TrimSpace(tok)
			if tok == "" || tok == "0" {
				continue
			}
			parts := strings.Split(tok, ",")
			fpName := formatForceDisplayName(strings.TrimSpace(parts[0]))
			level := "3"
			if len(parts) > 1 {
				level = strings.TrimSpace(parts[1])
			}
			forceLines = append(forceLines, fmt.Sprintf("%s (%s)", fpName, level))
		}
	}
	if len(forceLines) > 0 {
		sb.WriteString("^5Force Powers:\n")
		for _, line := range forceLines {
			sb.WriteString(line)
			sb.WriteString("\n")
		}
		sb.WriteString("\n")
	}

	// 3. Saber Styles (if applicable)
	if ch.SaberStyle != "" && ch.SaberStyle != "0" {
		styleName := formatSaberStyles(ch.SaberStyle)
		if styleName != "" {
			sb.WriteString("^5Saber Styles:\n")
			sb.WriteString(styleName + "\n\n")
		}
	}

	// 4. Attributes (^8)
	var attrLines []string
	if ch.Attributes != "" {
		attTokens := strings.Split(ch.Attributes, "|")
		for _, tok := range attTokens {
			tok = strings.TrimSpace(tok)
			if tok == "" || tok == "0" {
				continue
			}
			parts := strings.Split(tok, ",")
			attName := formatAttrDisplayName(strings.TrimSpace(parts[0]))
			if len(parts) > 1 && strings.TrimSpace(parts[1]) != "" && strings.TrimSpace(parts[1]) != "0" {
				attrLines = append(attrLines, fmt.Sprintf("%s (%s)", attName, strings.TrimSpace(parts[1])))
			} else {
				attrLines = append(attrLines, fmt.Sprintf("%s", attName))
			}
		}
	}
	if len(attrLines) > 0 {
		sb.WriteString("^8Attributes:\n")
		for _, line := range attrLines {
			sb.WriteString(line)
			sb.WriteString("\n")
		}
		sb.WriteString("\n")
	}

	// 5. Inventory / Holdables (^6)
	var invLines []string
	holdablesVal := ""
	if ch.ExtraFields != nil {
		holdablesVal = ch.ExtraFields["holdables"]
	}
	if holdablesVal != "" {
		hTokens := strings.Split(holdablesVal, "|")
		for _, tok := range hTokens {
			tok = strings.TrimSpace(tok)
			if tok == "" || tok == "0" {
				continue
			}
			invLines = append(invLines, fmt.Sprintf("%s", formatHoldableDisplayName(tok)))
		}
	}
	if len(invLines) > 0 {
		sb.WriteString("^6Inventory:\n")
		for _, line := range invLines {
			sb.WriteString(line)
			sb.WriteString("\n")
		}
		sb.WriteString("\n")
	}

	// 6. Custom Skills / Abilities (^3)
	var skillLines []string
	if ch.ExtraFields != nil {
		for i := 0; i < 20; i++ {
			nameKey := fmt.Sprintf("c_att_names_%d", i)
			if name, ok := ch.ExtraFields[nameKey]; ok && strings.TrimSpace(name) != "" {
				skillLines = append(skillLines, name)
			}
		}
	}
	if len(skillLines) > 0 {
		sb.WriteString("^3Abilities / Custom Skills:\n")
		for _, line := range skillLines {
			sb.WriteString(line)
			sb.WriteString("\n")
		}
		sb.WriteString("\n")
	}

	// 7. Preserve trailing backstory / lore paragraph if present
	if ch.Description != "" {
		lore := extractExistingLore(ch.Description)
		if lore != "" {
			sb.WriteString(lore)
			sb.WriteString("\n")
		}
	}

	return strings.TrimSpace(sb.String())
}

func extractExistingLore(oldDesc string) string {
	lines := strings.Split(oldDesc, "\n")
	var loreLines []string
	inSection := false

	sectionHeaderRegex := regexp.MustCompile(`^(\^[0-9A-Za-z])?([A-Za-z\s/]+):`)

	for i := 1; i < len(lines); i++ {
		line := strings.TrimSpace(lines[i])
		if line == "" {
			continue
		}
		if sectionHeaderRegex.MatchString(line) {
			inSection = true
			continue
		}
		if inSection {
			// Check if this looks like a bullet or field (starts with -, *, or ends with (N))
			if strings.HasPrefix(line, "-") || strings.HasPrefix(line, "*") || strings.Contains(line, "(") {
				continue
			}
			// If it's a long sentence or paragraph, it's lore!
			if len(line) > 40 && !strings.Contains(line, "WP_") && !strings.Contains(line, "MB_ATT_") {
				inSection = false
				loreLines = append(loreLines, line)
			}
		} else {
			if len(line) > 30 {
				loreLines = append(loreLines, line)
			}
		}
	}

	return strings.Join(loreLines, "\n\n")
}

func formatSaberStyles(styleMask string) string {
	mask, err := strconv.Atoi(styleMask)
	if err != nil {
		return styleMask
	}
	var styles []string
	styleNames := []struct {
		bit  int
		name string
	}{
		{1, "Fast (Cyan)"},
		{2, "Medium (Yellow)"},
		{4, "Strong (Red)"},
		{8, "Desann (Purple)"},
		{16, "Tavion (Blue)"},
		{32, "Duals"},
		{64, "Staff"},
	}
	for _, s := range styleNames {
		if (mask & s.bit) != 0 {
			styles = append(styles, s.name)
		}
	}
	if len(styles) == 0 {
		return ""
	}
	return strings.Join(styles, " / ")
}

// Q3 color codes mapping with authentic RGB values
type Q3ColorInfo struct {
	Code  string
	Name  string
	Color color.NRGBA
}

var Q3ColorLabels = []Q3ColorInfo{
	{"1", "Red", color.NRGBA{239, 83, 80, 255}},
	{"2", "Green", color.NRGBA{102, 187, 106, 255}},
	{"3", "Yellow", color.NRGBA{255, 238, 88, 255}},
	{"4", "Blue", color.NRGBA{66, 165, 245, 255}},
	{"5", "Cyan", color.NRGBA{38, 198, 218, 255}},
	{"6", "Magenta", color.NRGBA{236, 64, 122, 255}},
	{"7", "White", color.NRGBA{255, 255, 255, 255}},
	{"8", "Orange", color.NRGBA{255, 167, 38, 255}},
	{"9", "Grey", color.NRGBA{189, 189, 189, 255}},
}

func newColorDotResource(name string, c color.NRGBA) fyne.Resource {
	hex := fmt.Sprintf("#%02X%02X%02X", c.R, c.G, c.B)
	svgStr := fmt.Sprintf(`<svg width="16" height="16" viewBox="0 0 16 16" xmlns="http://www.w3.org/2000/svg"><circle cx="8" cy="8" r="6" fill="%s"/></svg>`, hex)
	return fyne.NewStaticResource(name+".svg", []byte(svgStr))
}

// NewQ3ColorToolbar creates a toolbar with rich Q3 color swatches and Auto-Generate.
func NewQ3ColorToolbar(entry *ValidatedEntry, onAutoGenerate func()) fyne.CanvasObject {
	buttons := make([]fyne.CanvasObject, 0, len(Q3ColorLabels)+2)

	for _, c := range Q3ColorLabels {
		code := c.Code
		icon := newColorDotResource(c.Name, c.Color)
		btn := NewTooltipButton(fmt.Sprintf("^%s", code), icon, func() {
			if entry != nil {
				entry.SetText(entry.Text + fmt.Sprintf("^%s", code))
			}
		}, fmt.Sprintf("Insert ^%s (%s)", code, c.Name))
		btn.Importance = widget.LowImportance
		buttons = append(buttons, btn)
	}

	clearBtn := NewTooltipButton("Clear", nil, func() {
		if entry != nil {
			cleaned := stripQ3Colors(entry.Text)
			entry.SetText(cleaned)
		}
	}, "Remove all ^1..^9 color codes from description")
	clearBtn.Importance = widget.LowImportance
	buttons = append(buttons, clearBtn)

	autoBtn := widget.NewButton("✨ Auto-Generate", func() {
		if onAutoGenerate != nil {
			onAutoGenerate()
		}
	})
	autoBtn.Importance = widget.MediumImportance

	leftBar := container.NewHBox(buttons...)
	return container.NewBorder(nil, nil, leftBar, autoBtn, nil)
}

func stripQ3Colors(s string) string {
	for _, c := range Q3ColorLabels {
		s = strings.ReplaceAll(s, fmt.Sprintf("^%s", c.Code), "")
	}
	return s
}

// RenderQ3ColoredPreview builds a live, styled terminal preview of Q3 color-coded text.
func RenderQ3ColoredPreview(rawText string) fyne.CanvasObject {
	lines := strings.Split(rawText, "\n")
	rows := make([]fyne.CanvasObject, 0, len(lines))

	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			rows = append(rows, widget.NewLabel(""))
			continue
		}

		var spans []fyne.CanvasObject
		curColor := color.Color(color.NRGBA{235, 240, 245, 255})
		var curText strings.Builder

		flush := func() {
			if curText.Len() > 0 {
				txt := canvas.NewText(curText.String(), curColor)
				txt.TextSize = 13
				txt.TextStyle = fyne.TextStyle{Monospace: true}
				spans = append(spans, txt)
				curText.Reset()
			}
		}

		runes := []rune(line)
		for i := 0; i < len(runes); {
			if runes[i] == '^' && i+1 < len(runes) && runes[i+1] >= '0' && runes[i+1] <= '9' {
				flush()
				code := string(runes[i+1])
				for _, c := range Q3ColorLabels {
					if c.Code == code {
						curColor = c.Color
						break
					}
				}
				if code == "0" {
					curColor = color.NRGBA{120, 120, 120, 255}
				}
				i += 2
			} else {
				curText.WriteRune(runes[i])
				i++
			}
		}
		flush()

		if len(spans) == 0 {
			rows = append(rows, widget.NewLabel(""))
		} else {
			rows = append(rows, container.NewHBox(spans...))
		}
	}

	content := container.NewVBox(rows...)
	bg := canvas.NewRectangle(color.NRGBA{14, 18, 24, 255})
	card := container.NewStack(bg, container.NewPadded(content))
	return container.NewVScroll(card)
}

func formatWeaponDisplayName(id string) string {
	if name, ok := CanonicalWeaponNames[id]; ok {
		return name
	}
	// Check loaded weapons
	for _, w := range GetWeapons() {
		if w.ID == id && w.Name != "" {
			return w.Name
		}
	}
	clean := strings.TrimPrefix(id, "WP_")
	clean = strings.ReplaceAll(clean, "_", " ")
	return strings.Title(strings.ToLower(clean))
}

func formatForceDisplayName(id string) string {
	if name, ok := CanonicalForceNames[id]; ok {
		return name
	}
	clean := strings.TrimPrefix(id, "FP_")
	clean = strings.ReplaceAll(clean, "_", " ")
	return strings.Title(strings.ToLower(clean))
}

func formatHoldableDisplayName(id string) string {
	if name, ok := CanonicalHoldableNames[id]; ok {
		return name
	}
	clean := strings.TrimPrefix(id, "HI_")
	clean = strings.TrimPrefix(clean, "MB_HI_")
	clean = strings.ReplaceAll(clean, "_", " ")
	return strings.Title(strings.ToLower(clean))
}

func formatAttrDisplayName(id string) string {
	if name, ok := CanonicalAttributeNames[id]; ok {
		return name
	}
	// Check loaded attributes
	for _, a := range GetAttributes() {
		if a.ID == id && a.Name != "" {
			return a.Name
		}
	}
	clean := strings.TrimPrefix(id, "MB_ATT_")
	clean = strings.ReplaceAll(clean, "_", " ")
	return strings.Title(strings.ToLower(clean))
}
