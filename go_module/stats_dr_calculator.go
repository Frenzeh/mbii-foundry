package main

import (
	"fmt"
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"github.com/Frenzeh/mbii-foundry/parsers"
)

// DefensiveMatrix contains exact configured pools plus compatibility fields
// for the existing UI. Damage-reduction mechanics are deliberately not
// inferred from class names or attribute strings.
type DefensiveMatrix struct {
	MaxHealth   int
	MaxArmor    int
	ExtraLives  int
	TotalLives  int
	RawPool     int
	TotalRawEHP int

	HasMagPlating  bool
	MagPlatingDR   float64
	HasBlastArmour bool
	BlastArmourDR  float64
	HasCortosis    int
	HasBeskar      int
	HasEnvProt     int
	HasSBDBattery  bool
	SBDBatteryDR   float64
	HasStability   bool
	StabilityDR    float64

	// Until engine formulas are mechanically grounded, channel-specific
	// values remain equal to the configured raw pool across lives.
	EnergyEHP    int
	ExplosiveEHP int
	MeleeEHP     int

	EvidenceStatus string
	SummaryTags    []string
}

// CalculateDefensiveMatrix performs only arithmetic supported directly by
// parsed fields. It never assigns damage reduction based on a class or on
// substring matches in the attribute list.
func CalculateDefensiveMatrix(char *parsers.MBCHCharacter) DefensiveMatrix {
	if char == nil {
		return DefensiveMatrix{
			EvidenceStatus: "Unverified",
			SummaryTags:    []string{"Unverified: damage-reduction mechanics are not calculated"},
		}
	}

	dm := DefensiveMatrix{
		MaxHealth:      char.MaxHealth,
		MaxArmor:       char.MaxArmor,
		ExtraLives:     char.ExtraLives,
		EvidenceStatus: "Unverified",
		SummaryTags:    []string{"Unverified: damage-reduction mechanics are not calculated"},
	}
	if dm.ExtraLives < 0 {
		dm.ExtraLives = 0
	}
	dm.TotalLives = dm.ExtraLives + 1
	dm.RawPool = dm.MaxHealth + dm.MaxArmor
	dm.TotalRawEHP = dm.RawPool * dm.TotalLives
	dm.EnergyEHP = dm.TotalRawEHP
	dm.ExplosiveEHP = dm.TotalRawEHP
	dm.MeleeEHP = dm.TotalRawEHP
	return dm
}

// BuildDefensiveMatrixWidget shows configured pools and explicitly marks
// unsupported channel-specific mechanics as Unverified.
func BuildDefensiveMatrixWidget(dm DefensiveMatrix) fyne.CanvasObject {
	titleLbl := widget.NewLabelWithStyle("CONFIGURED DEFENSIVE POOLS", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})

	// These figures are direct arithmetic over configured fields, not an
	// assertion about the engine's damage-reduction formulas.
	rawSummary := fmt.Sprintf("HP: %d  |  Armor: %d  |  Lives: %d  |  Configured pool: %d",
		dm.MaxHealth, dm.MaxArmor, dm.TotalLives, dm.TotalRawEHP)
	rawLbl := widget.NewLabelWithStyle(rawSummary, fyne.TextAlignLeading, fyne.TextStyle{Monospace: true})

	energyPill := createStatBadge("Energy EHP: Unverified", false,
		color.RGBA{R: 40, G: 140, B: 240, A: 255})
	expPill := createStatBadge("Explosive EHP: Unverified", false,
		color.RGBA{R: 240, G: 120, B: 40, A: 255})
	meleePill := createStatBadge("Melee EHP: Unverified", false,
		color.RGBA{R: 180, G: 60, B: 200, A: 255})
	ehpRow := container.NewHBox(energyPill, expPill, meleePill)

	conditionalFootnote := widget.NewLabelWithStyle(
		"Unverified: no class- or attribute-based damage-reduction estimates are applied.",
		fyne.TextAlignLeading,
		fyne.TextStyle{Italic: true},
	)

	content := container.NewVBox(
		container.NewHBox(widget.NewIcon(theme.RadioButtonCheckedIcon()), titleLbl),
		rawLbl,
		ehpRow,
		conditionalFootnote,
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
