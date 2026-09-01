package main

import (
	"fmt"
	"image/color"
	"os"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"github.com/Frenzeh/mbii-foundry/parsers"
)

// ShowCharacterDiffDialog opens a side-by-side comparison tool for two characters.
func ShowCharacterDiffDialog(parent fyne.Window, charA *parsers.MBCHCharacter, nameA string) {
	if charA == nil {
		dialog.ShowInformation("Compare Characters", "Please open or create a character first.", parent)
		return
	}

	var charB *parsers.MBCHCharacter
	var nameB string = "Select a character to compare..."

	diffContent := container.NewVBox()

	var refreshDiff func()
	refreshDiff = func() {
		diffContent.Objects = nil

		if charB == nil {
			msg := widget.NewLabelWithStyle("Click 'Browse Character B...' below to load a second character for comparison.", fyne.TextAlignCenter, fyne.TextStyle{Italic: true})
			diffContent.Add(container.NewPadded(msg))
			diffContent.Refresh()
			return
		}

		dmA := CalculateDefensiveMatrix(charA)
		dmB := CalculateDefensiveMatrix(charB)

		// 1. Header Overview
		hdrA := widget.NewLabelWithStyle(fmt.Sprintf("%s\nClass: %s\nModel: %s/%s", nameA, charA.MBClass, charA.Model, charA.Skin), fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
		hdrB := widget.NewLabelWithStyle(fmt.Sprintf("%s\nClass: %s\nModel: %s/%s", nameB, charB.MBClass, charB.Model, charB.Skin), fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
		hdrRow := container.NewGridWithColumns(2, container.NewPadded(hdrA), container.NewPadded(hdrB))

		// 2. Stats Delta Table
		statsList := container.NewVBox(
			widget.NewLabelWithStyle("Core Stats & Defensive Matrix", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
			createDiffRow("Health", fmt.Sprintf("%d", charA.MaxHealth), fmt.Sprintf("%d", charB.MaxHealth), charA.MaxHealth != charB.MaxHealth),
			createDiffRow("Armor", fmt.Sprintf("%d", charA.MaxArmor), fmt.Sprintf("%d", charB.MaxArmor), charA.MaxArmor != charB.MaxArmor),
			createDiffRow("Total Lives", fmt.Sprintf("%d", dmA.TotalLives), fmt.Sprintf("%d", dmB.TotalLives), dmA.TotalLives != dmB.TotalLives),
			createDiffRow("Raw EHP", fmt.Sprintf("%d", dmA.TotalRawEHP), fmt.Sprintf("%d", dmB.TotalRawEHP), dmA.TotalRawEHP != dmB.TotalRawEHP),
			createDiffRow("Energy EHP", fmt.Sprintf("%d", dmA.EnergyEHP), fmt.Sprintf("%d", dmB.EnergyEHP), dmA.EnergyEHP != dmB.EnergyEHP),
			createDiffRow("Explosive EHP", fmt.Sprintf("%d", dmA.ExplosiveEHP), fmt.Sprintf("%d", dmB.ExplosiveEHP), dmA.ExplosiveEHP != dmB.ExplosiveEHP),
			createDiffRow("Speed", fmt.Sprintf("%.2f", charA.Speed), fmt.Sprintf("%.2f", charB.Speed), charA.Speed != charB.Speed),
		)

		// 3. Weapons Comparison
		weapList := container.NewVBox(
			widget.NewLabelWithStyle("Weapon Arsenal", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
			createDiffRow("Weapons", strings.ReplaceAll(charA.Weapons, "|", "\n"), strings.ReplaceAll(charB.Weapons, "|", "\n"), charA.Weapons != charB.Weapons),
		)

		// 4. Attributes Comparison
		attrList := container.NewVBox(
			widget.NewLabelWithStyle("Attributes & Abilities", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
			createDiffRow("Attributes", strings.ReplaceAll(charA.Attributes, "|", "\n"), strings.ReplaceAll(charB.Attributes, "|", "\n"), charA.Attributes != charB.Attributes),
		)

		diffContent.Add(container.NewVBox(
			hdrRow,
			widget.NewSeparator(),
			statsList,
			widget.NewSeparator(),
			weapList,
			widget.NewSeparator(),
			attrList,
		))
		diffContent.Refresh()
	}

	browseBtn := widget.NewButtonWithIcon("Browse Character B (.mbch)...", theme.FileIcon(), func() {
		fd := dialog.NewFileOpen(func(reader fyne.URIReadCloser, err error) {
			if err != nil || reader == nil {
				return
			}
			defer reader.Close()

			data, readErr := os.ReadFile(reader.URI().Path())
			if readErr != nil {
				dialog.ShowError(readErr, parent)
				return
			}

			parsed, parseErr := parsers.ParseMBCH(string(data))
			if parseErr != nil {
				dialog.ShowError(parseErr, parent)
				return
			}

			charB = parsed
			nameB = reader.URI().Name()
			refreshDiff()
		}, parent)
		fd.Show()
	})
	browseBtn.Importance = widget.HighImportance

	refreshDiff()

	scroll := container.NewVScroll(diffContent)
	scroll.SetMinSize(fyne.NewSize(700, 480))

	dlg := dialog.NewCustom(
		"Side-by-Side Character Comparison & Diff",
		"Close",
		container.NewBorder(
			container.NewVBox(
				widget.NewLabelWithStyle("Compare Character Attributes, Weapons & EHP Deltas", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
				browseBtn,
				widget.NewSeparator(),
			),
			nil, nil, nil,
			scroll,
		),
		parent,
	)
	dlg.Resize(fyne.NewSize(760, 560))
	dlg.Show()
}

func createDiffRow(label, valA, valB string, isDiff bool) fyne.CanvasObject {
	lblTitle := widget.NewLabelWithStyle(label+":", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})

	lblA := widget.NewLabel(valA)
	lblB := widget.NewLabel(valB)

	colA := container.NewPadded(lblA)
	colB := container.NewPadded(lblB)

	if isDiff {
		bgA := canvas.NewRectangle(color.RGBA{R: 60, G: 30, B: 30, A: 160})
		bgA.CornerRadius = 4
		colA = container.NewStack(bgA, colA)

		bgB := canvas.NewRectangle(color.RGBA{R: 30, G: 60, B: 30, A: 160})
		bgB.CornerRadius = 4
		colB = container.NewStack(bgB, colB)
	}

	return container.NewGridWithColumns(3,
		lblTitle,
		colA,
		colB,
	)
}
