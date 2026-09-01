package main

import (
	"fmt"
	"image/color"
	"strconv"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"github.com/Frenzeh/mbii-foundry/parsers"
)

// DefensiveMatrix holds computed health, armor, damage reduction, and effective HP metrics.
type DefensiveMatrix struct {
	MaxHealth   int
	MaxArmor    int
	ExtraLives  int
	TotalLives  int
	RawPool     int
	TotalRawEHP int

	// Active Damage Reduction Modifiers
	HasMagPlating   bool
	MagPlatingDR    float64 // 0.40 vs energy/blasters
	HasBlastArmour  bool
	BlastArmourDR   float64 // 0.40 vs heavy explosives, 0.30 vs splash, 0.20 vs melee
	HasCortosis     int     // 0 = none, 1 = armor absorbs saber, 2 = 50% drain
	HasBeskar       int     // 0 = none, 1..3 = beskar tier
	HasEnvProt      int     // 0 = none, 1..3 = environmental hazard protection
	HasSBDBattery   bool
	SBDBatteryDR    float64 // 0.25 - 0.40 depending on battery
	HasStability    bool
	StabilityDR     float64 // 0.10 - 0.20

	// Effective HP vs specific damage channels
	EnergyEHP    int
	ExplosiveEHP int
	MeleeEHP     int

	SummaryTags []string
}

// CalculateDefensiveMatrix computes all defensive stats and DR multipliers for a character.
func CalculateDefensiveMatrix(char *parsers.MBCHCharacter) DefensiveMatrix {
	dm := DefensiveMatrix{
		MaxHealth:  char.MaxHealth,
		MaxArmor:   char.MaxArmor,
		ExtraLives: 0,
	}

	if dm.MaxHealth <= 0 {
		dm.MaxHealth = 100
	}

	// Parse reinforcements / extra lives
	if val, ok := char.ExtraFields["reinforcements"]; ok {
		if n, err := strconv.Atoi(strings.TrimSpace(val)); err == nil && n > 0 {
			dm.ExtraLives = n
		}
	}
	dm.TotalLives = dm.ExtraLives + 1
	dm.RawPool = dm.MaxHealth + dm.MaxArmor
	dm.TotalRawEHP = dm.RawPool * dm.TotalLives

	// Parse attributes for DR modifiers
	attrs := parseAttributesMap(char.Attributes)

	// 1. Magnetic Plating (MB_ATT_MAGNETIC_PLATING) -> 40% DR vs Energy / Blasters
	if level, ok := attrs["MB_ATT_MAGNETIC_PLATING"]; ok && level > 0 {
		dm.HasMagPlating = true
		dm.MagPlatingDR = 0.40
		dm.SummaryTags = append(dm.SummaryTags, "🛡️ Mag Plating (40% Energy DR)")
	}

	// 2. Blast Armour (MB_ATT_BLAST_ARMOUR) -> 40% Heavy Explosive, 30% Splash, 20% Melee
	if level, ok := attrs["MB_ATT_BLAST_ARMOUR"]; ok && level > 0 {
		dm.HasBlastArmour = true
		dm.BlastArmourDR = 0.40
		dm.SummaryTags = append(dm.SummaryTags, "💣 Blast Armour (40% Exp / 20% Melee DR)")
	}

	// 3. Cortosis (MB_ATT_CORTOSIS)
	if level, ok := attrs["MB_ATT_CORTOSIS"]; ok && level > 0 {
		dm.HasCortosis = level
		dm.SummaryTags = append(dm.SummaryTags, fmt.Sprintf("⚡ Cortosis L%d (Saber Resistance)", level))
	}

	// 4. Beskar (MB_ATT_BESKAR)
	if level, ok := attrs["MB_ATT_BESKAR"]; ok && level > 0 {
		dm.HasBeskar = level
		dm.SummaryTags = append(dm.SummaryTags, fmt.Sprintf("🪖 Beskar L%d", level))
	}

	// 5. Environmental Protection (MB_ATT_ENV_PROT)
	if level, ok := attrs["MB_ATT_ENV_PROT"]; ok && level > 0 {
		dm.HasEnvProt = level
		dm.SummaryTags = append(dm.SummaryTags, fmt.Sprintf("🧪 Env Prot L%d", level))
	}

	// 6. SBD Class Battery DR
	if char.MBClass == "MB_CLASS_SBD" {
		dm.HasSBDBattery = true
		dm.SBDBatteryDR = 0.35 // nominal active battery DR
		dm.SummaryTags = append(dm.SummaryTags, "🔋 SBD Battery DR (25–40%)")
	}

	// 7. Shocktrooper Stability DR
	if char.MBClass == "MB_CLASS_SHOCKTROOPER" || strings.Contains(char.Attributes, "MB_ATT_RESOURCE_STABILITY") {
		dm.HasStability = true
		dm.StabilityDR = 0.15
		dm.SummaryTags = append(dm.SummaryTags, "⚡ Stability DR (10–20%)")
	}

	// Compute Channel-Specific EHP
	// EHP = Raw Pool / (1 - DR) * TotalLives
	totalBasePool := float64(dm.RawPool)

	// Energy EHP
	energyDR := 0.0
	if dm.HasMagPlating {
		energyDR += dm.MagPlatingDR
	}
	if dm.HasSBDBattery {
		energyDR += dm.SBDBatteryDR
	}
	if dm.HasStability {
		energyDR += dm.StabilityDR
	}
	if energyDR > 0.85 {
		energyDR = 0.85 // Engine safety cap
	}
	if energyDR > 0 {
		dm.EnergyEHP = int(totalBasePool / (1.0 - energyDR) * float64(dm.TotalLives))
	} else {
		dm.EnergyEHP = dm.TotalRawEHP
	}

	// Explosive EHP
	expDR := 0.0
	if dm.HasBlastArmour {
		expDR += dm.BlastArmourDR
	}
	if dm.HasSBDBattery {
		expDR += 0.20
	}
	if expDR > 0.85 {
		expDR = 0.85
	}
	if expDR > 0 {
		dm.ExplosiveEHP = int(totalBasePool / (1.0 - expDR) * float64(dm.TotalLives))
	} else {
		dm.ExplosiveEHP = dm.TotalRawEHP
	}

	// Melee EHP
	meleeDR := 0.0
	if dm.HasBlastArmour {
		meleeDR += 0.20
	}
	if meleeDR > 0 {
		dm.MeleeEHP = int(totalBasePool / (1.0 - meleeDR) * float64(dm.TotalLives))
	} else {
		dm.MeleeEHP = dm.TotalRawEHP
	}

	return dm
}

func parseAttributesMap(attrString string) map[string]int {
	res := make(map[string]int)
	if attrString == "" {
		return res
	}
	parts := strings.Split(attrString, "|")
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		kv := strings.Split(p, ",")
		key := strings.TrimSpace(kv[0])
		val := 1
		if len(kv) > 1 {
			if n, err := strconv.Atoi(strings.TrimSpace(kv[1])); err == nil {
				val = n
			}
		}
		res[key] = val
	}
	return res
}

// BuildDefensiveMatrixWidget builds a stylized card showcasing the character's EHP and DR breakdown.
func BuildDefensiveMatrixWidget(dm DefensiveMatrix) fyne.CanvasObject {
	titleLbl := widget.NewLabelWithStyle("DEFENSIVE MATRIX & EFFECTIVE HP (EHP)", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})

	// Row 1: Raw Health/Armor & Lives
	rawSummary := fmt.Sprintf("HP: %d  |  Armor: %d  |  Lives: %d  |  Raw Total: %d HP",
		dm.MaxHealth, dm.MaxArmor, dm.TotalLives, dm.TotalRawEHP)
	rawLbl := widget.NewLabelWithStyle(rawSummary, fyne.TextAlignLeading, fyne.TextStyle{Monospace: true})

	// Row 2: EHP Metrics
	energyPill := createStatBadge(fmt.Sprintf("⚡ Energy EHP: %d", dm.EnergyEHP),
		dm.EnergyEHP > dm.TotalRawEHP, color.RGBA{R: 40, G: 140, B: 240, A: 255})

	expPill := createStatBadge(fmt.Sprintf("💣 Explosive EHP: %d", dm.ExplosiveEHP),
		dm.ExplosiveEHP > dm.TotalRawEHP, color.RGBA{R: 240, G: 120, B: 40, A: 255})

	meleePill := createStatBadge(fmt.Sprintf("🥊 Melee EHP: %d", dm.MeleeEHP),
		dm.MeleeEHP > dm.TotalRawEHP, color.RGBA{R: 180, G: 60, B: 200, A: 255})

	ehpRow := container.NewHBox(energyPill, expPill, meleePill)

	// Row 3: Active Modifiers / Tags
	var tagsBox *fyne.Container
	if len(dm.SummaryTags) > 0 {
		var tagObjects []fyne.CanvasObject
		for _, tag := range dm.SummaryTags {
			lbl := widget.NewLabelWithStyle(tag, fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
			tagObjects = append(tagObjects, container.NewPadded(lbl))
		}
		tagsBox = container.NewHBox(tagObjects...)
	} else {
		tagsBox = container.NewHBox(widget.NewLabelWithStyle("No active damage reduction attributes", fyne.TextAlignLeading, fyne.TextStyle{Italic: true}))
	}

	content := container.NewVBox(
		container.NewHBox(widget.NewIcon(theme.RadioButtonCheckedIcon()), titleLbl),
		rawLbl,
		ehpRow,
		tagsBox,
	)

	bg := canvas.NewRectangle(color.RGBA{R: 28, G: 32, B: 42, A: 255})
	bg.CornerRadius = 6

	return container.NewStack(bg, container.NewPadded(content))
}

func createStatBadge(text string, boosted bool, accent color.RGBA) fyne.CanvasObject {
	lbl := widget.NewLabelWithStyle(text, fyne.TextAlignCenter, fyne.TextStyle{Bold: true})
	bgColor := color.RGBA{R: 40, G: 45, B: 55, A: 255}
	if boosted {
		bgColor = color.RGBA{R: accent.R / 4, G: accent.G / 4, B: accent.B / 4, A: 255}
	}
	bg := canvas.NewRectangle(bgColor)
	bg.CornerRadius = 4
	bg.StrokeColor = accent
	bg.StrokeWidth = 1
	return container.NewStack(bg, container.NewPadded(lbl))
}
