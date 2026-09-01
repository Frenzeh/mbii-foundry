package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// MBTCRoster holds team composition data for a .mbtc file.
type MBTCRoster struct {
	Name            string
	ClassesAllowed  int
	RebelClasses    [6]string
	ImperialClasses [6]string
}

// ClassBitDefinition defines the bit value and names for the ClassesAllowed mask.
type ClassBitDefinition struct {
	Bit      int
	Rebel    string
	Imperial string
}

var MBTCClassBits = []ClassBitDefinition{
	{Bit: 1, Rebel: "Soldier", Imperial: "Trooper"},
	{Bit: 2, Rebel: "Commander", Imperial: "Elite Trooper"},
	{Bit: 4, Rebel: "Jedi", Imperial: "Sith"},
	{Bit: 8, Rebel: "Hero", Imperial: "Bounty Hunter"},
	{Bit: 16, Rebel: "Wookiee", Imperial: "Droideka"},
	{Bit: 32, Rebel: "Clonetrooper", Imperial: "Mandalorian"},
	{Bit: 64, Rebel: "ARC Trooper", Imperial: "SBD"},
}

// MBTCComposerDialog provides an interactive editor for .mbtc team files.
type MBTCComposerDialog struct {
	app        *App
	roster     MBTCRoster
	dialogWin  fyne.Window
	maskLabel  *widget.Label
	allCheck   *widget.Check
	bitChecks  []*widget.Check
	rebelSlots [6]*widget.Entry
	impSlots   [6]*widget.Entry
}

func OpenMBTCComposer(app *App) {
	composer := &MBTCComposerDialog{
		app:    app,
		roster: MBTCRoster{},
	}
	composer.show()
}

func (c *MBTCComposerDialog) show() {
	c.dialogWin = c.app.fyneApp.NewWindow("Team Composer (.mbtc)")
	c.dialogWin.Resize(fyne.NewSize(750, 650))

	// ClassesAllowed Header & Checkboxes
	c.maskLabel = widget.NewLabelWithStyle("ClassesAllowed = 0 (All Classes)", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})

	c.allCheck = widget.NewCheck("All Classes (no restriction)", func(checked bool) {
		if checked {
			c.roster.ClassesAllowed = 0
			for _, chk := range c.bitChecks {
				chk.SetChecked(false)
			}
		}
		c.updateMaskLabel()
	})
	c.allCheck.Checked = true

	var bitWidgets []fyne.CanvasObject
	for _, bitDef := range MBTCClassBits {
		b := bitDef
		chk := widget.NewCheck(fmt.Sprintf("%s / %s (bit %d)", b.Rebel, b.Imperial, b.Bit), func(checked bool) {
			if checked {
				c.allCheck.SetChecked(false)
				c.roster.ClassesAllowed |= b.Bit
			} else {
				c.roster.ClassesAllowed &^= b.Bit
				if c.roster.ClassesAllowed == 0 {
					c.allCheck.SetChecked(true)
				}
			}
			c.updateMaskLabel()
		})
		c.bitChecks = append(c.bitChecks, chk)
		bitWidgets = append(bitWidgets, chk)
	}

	bitsGrid := container.NewGridWithColumns(2, bitWidgets...)
	maskBox := container.NewVBox(
		c.maskLabel,
		c.allCheck,
		bitsGrid,
	)

	// Team Slots (6 per team)
	var rebelRows []fyne.CanvasObject
	for i := 0; i < 6; i++ {
		slotIdx := i
		c.rebelSlots[i] = NewInputEntry()
		c.rebelSlots[i].SetPlaceHolder(fmt.Sprintf("Slot %d (e.g. mb2_rebel_soldier)", i+1))
		c.rebelSlots[i].OnChanged = func(s string) {
			c.roster.RebelClasses[slotIdx] = s
		}
		rebelRows = append(rebelRows, widget.NewFormItem(fmt.Sprintf("Slot %d", i+1), c.rebelSlots[i]).Widget)
	}

	var impRows []fyne.CanvasObject
	for i := 0; i < 6; i++ {
		slotIdx := i
		c.impSlots[i] = NewInputEntry()
		c.impSlots[i].SetPlaceHolder(fmt.Sprintf("Slot %d (e.g. mb2_imp_trooper)", i+1))
		c.impSlots[i].OnChanged = func(s string) {
			c.roster.ImperialClasses[slotIdx] = s
		}
		impRows = append(impRows, widget.NewFormItem(fmt.Sprintf("Slot %d", i+1), c.impSlots[i]).Widget)
	}

	rebelCard := widget.NewCard("Rebel / Light Side", "Up to 6 class slots", container.NewVBox(rebelRows...))
	impCard := widget.NewCard("Imperial / Dark Side", "Up to 6 class slots", container.NewVBox(impRows...))

	teamsSplit := container.NewGridWithColumns(2, rebelCard, impCard)

	// Action buttons
	openBtn := widget.NewButtonWithIcon("Open .mbtc", theme.FolderOpenIcon(), c.openFile)
	saveBtn := widget.NewButtonWithIcon("Save .mbtc", theme.DocumentSaveIcon(), c.saveFile)

	bottomBar := container.NewHBox(
		openBtn,
		saveBtn,
	)

	content := container.NewVScroll(container.NewVBox(
		widget.NewCard("Classes Allowed Mask", "", maskBox),
		teamsSplit,
		bottomBar,
	))

	c.dialogWin.SetContent(content)
	c.dialogWin.Show()
}

func (c *MBTCComposerDialog) updateMaskLabel() {
	if c.roster.ClassesAllowed == 0 {
		c.maskLabel.SetText("ClassesAllowed = 0 (All Classes)")
	} else {
		c.maskLabel.SetText(fmt.Sprintf("ClassesAllowed = %d", c.roster.ClassesAllowed))
	}
}

func (c *MBTCComposerDialog) openFile() {
	dialog.ShowFileOpen(func(reader fyne.URIReadCloser, err error) {
		if err != nil || reader == nil {
			return
		}
		defer reader.Close()

		data, readErr := os.ReadFile(reader.URI().Path())
		if readErr != nil {
			dialog.ShowError(readErr, c.dialogWin)
			return
		}

		c.parseMBTC(string(data))
	}, c.dialogWin)
}

func (c *MBTCComposerDialog) parseMBTC(content string) {
	lines := strings.Split(content, "\n")
	rebIdx := 0
	impIdx := 0
	isImperial := false

	for _, rawLine := range lines {
		line := strings.TrimSpace(rawLine)
		if strings.Contains(strings.ToLower(line), "imperial") {
			isImperial = true
			continue
		}
		if strings.HasPrefix(line, "//") || line == "" {
			continue
		}

		parts := strings.Fields(line)
		if len(parts) >= 2 {
			key := strings.ToLower(parts[0])
			val := strings.Trim(parts[1], "\"")

			if key == "classesallowed" {
				mask, _ := strconv.Atoi(val)
				c.roster.ClassesAllowed = mask
				c.allCheck.SetChecked(mask == 0)
				for idx, b := range MBTCClassBits {
					if idx < len(c.bitChecks) {
						c.bitChecks[idx].SetChecked((mask & b.Bit) != 0)
					}
				}
				c.updateMaskLabel()
			} else if strings.HasPrefix(key, "class") {
				if isImperial {
					if impIdx < 6 {
						c.roster.ImperialClasses[impIdx] = val
						c.impSlots[impIdx].SetText(val)
						impIdx++
					}
				} else {
					if rebIdx < 6 {
						c.roster.RebelClasses[rebIdx] = val
						c.rebelSlots[rebIdx].SetText(val)
						rebIdx++
					}
				}
			}
		}
	}
}

func (c *MBTCComposerDialog) saveFile() {
	dialog.ShowFileSave(func(writer fyne.URIWriteCloser, err error) {
		if err != nil || writer == nil {
			return
		}
		defer writer.Close()

		var sb strings.Builder
		sb.WriteString("// MBII Team Configuration File\n\n")
		sb.WriteString(fmt.Sprintf("ClassesAllowed\t%d\n\n", c.roster.ClassesAllowed))

		sb.WriteString("// Rebel Team\n")
		for i, cls := range c.roster.RebelClasses {
			if strings.TrimSpace(cls) != "" {
				sb.WriteString(fmt.Sprintf("class%d\t\t\"%s\"\n", i+1, strings.TrimSpace(cls)))
			}
		}

		sb.WriteString("\n// Imperial Team\n")
		for i, cls := range c.roster.ImperialClasses {
			if strings.TrimSpace(cls) != "" {
				sb.WriteString(fmt.Sprintf("class%d\t\t\"%s\"\n", i+1, strings.TrimSpace(cls)))
			}
		}

		os.WriteFile(writer.URI().Path(), []byte(sb.String()), 0644)
		dialog.ShowInformation("Saved", fmt.Sprintf("Saved .mbtc to %s", filepath.Base(writer.URI().Path())), c.dialogWin)
	}, c.dialogWin)
}
