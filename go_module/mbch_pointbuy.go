package main

import (
	"fmt"
	"strconv"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

var KnownRankAttributes = []string{
	"rankHealth", "rankArmor", "rankAP", "rankBP", "rankCS", "rankAS", "rankROF", "rankSTM",
	"rankKbTaken", "rankKbGiven", "rankDmgTaken", "rankDmgGiven", "rankBaseSpeed",
	"rankSaberDamage", "rankSaberThrowDamage", "rankModelScale", "rankROFMelee",
	"rankHealthRegenAmount", "rankHealthRegenRate", "rankHealthRegenCap",
	"rankArmourRegenAmount", "rankArmourRegenRate", "rankArmourRegenCap",
	"rankBlockRegenAmount", "rankBlockRegenRate", "rankBlockRegenCap",
	"rankResourceRegenAmount", "rankResourceRegenRate", "rankResourceRegenCap",
	"rankForcePool", "rankForceRegen",
}

type PointBuyUI struct {
	editor    *MBCHEditor
	container *fyne.Container

	skillEntries []*widget.Entry
	nameEntries  []*widget.Entry
	rankEntries  []*widget.Entry

	rankAttrContainer *fyne.Container
}

func NewPointBuyUI(editor *MBCHEditor) *PointBuyUI {
	p := &PointBuyUI{editor: editor}
	p.createUI()
	return p
}

func (p *PointBuyUI) createUI() {
	p.skillEntries = make([]*widget.Entry, 15)
	p.nameEntries = make([]*widget.Entry, 15)
	p.rankEntries = make([]*widget.Entry, 15)

	grid := container.NewGridWithColumns(3)
	grid.Add(widget.NewLabelWithStyle("Skill Name", fyne.TextAlignCenter, fyne.TextStyle{Bold: true}))
	grid.Add(widget.NewLabelWithStyle("Display Name", fyne.TextAlignCenter, fyne.TextStyle{Bold: true}))
	grid.Add(widget.NewLabelWithStyle("Rank", fyne.TextAlignCenter, fyne.TextStyle{Bold: true}))

	for i := 0; i < 15; i++ {
		skill := widget.NewEntry()
		skill.SetPlaceHolder(fmt.Sprintf("SKILL_%d", i))
		skill.OnChanged = p.makeSkillEntryOnChanged(i)
		p.skillEntries[i] = skill
		grid.Add(skill)

		name := widget.NewEntry()
		name.SetPlaceHolder("Display Name")
		name.OnChanged = p.makeNameEntryOnChanged(i)
		p.nameEntries[i] = name
		grid.Add(name)

		rank := widget.NewEntry()
		rank.SetPlaceHolder("0-5")
		rank.OnChanged = p.makeRankEntryOnChanged(i)
		p.rankEntries[i] = rank
		grid.Add(rank)
	}

	// Rank Attributes Section
	p.rankAttrContainer = container.NewVBox()

	addRankAttrBtn := widget.NewButtonWithIcon("Add Rank Attribute", theme.ContentAddIcon(), func() {
		p.showAddRankAttrDialog()
	})

	p.container = container.NewVBox(
		widget.NewCard("Custom Skills", "Define up to 15 custom skills for point buy.", container.NewVScroll(grid)),
		widget.NewCard("Rank Attributes", "Define attribute values for each rank level (e.g., rankHealth 100,120,150).", container.NewVBox(
			addRankAttrBtn,
			p.rankAttrContainer,
		)),
	)
}

func (p *PointBuyUI) showAddRankAttrDialog() {
	keySelect := widget.NewSelectEntry(KnownRankAttributes)
	keySelect.PlaceHolder = "Select or type attribute (e.g., rankHealth)"
	valueEntry := widget.NewEntry()
	valueEntry.PlaceHolder = "Comma-separated values (e.g., 100,120,150)"

	dialog.ShowCustomConfirm("Add Rank Attribute", "Add", "Cancel", container.NewVBox(
		widget.NewLabel("Attribute Name:"),
		keySelect,
		widget.NewLabel("Values (comma-separated):"),
		valueEntry,
	), func(ok bool) {
		if ok && keySelect.Text != "" && valueEntry.Text != "" {
			p.editor.character.RankAttributes[keySelect.Text] = valueEntry.Text
			p.refreshRankAttributes()
			p.editor.markDirty()
		}
	}, p.editor.app.mainWindow)
}

func (p *PointBuyUI) refreshRankAttributes() {
	p.rankAttrContainer.Objects = nil

	for key, val := range p.editor.character.RankAttributes {
		k := key // capture for closure

		entry := widget.NewEntry()
		entry.SetText(val)
		entry.OnChanged = func(s string) {
			p.editor.character.RankAttributes[k] = s
			p.editor.markDirty()
		}

		delBtn := widget.NewButtonWithIcon("", theme.DeleteIcon(), func() {
			delete(p.editor.character.RankAttributes, k)
			p.refreshRankAttributes()
			p.editor.markDirty()
		})

		row := container.NewBorder(nil, nil, widget.NewLabel(k), delBtn, entry)
		p.rankAttrContainer.Add(row)
	}
	p.rankAttrContainer.Refresh()
}

func (p *PointBuyUI) GetContent() fyne.CanvasObject {
	return p.container
}

func (p *PointBuyUI) UpdateUI() {
	for i := 0; i < 15; i++ {
		p.skillEntries[i].SetText(p.editor.character.CustomSkills[i])
		p.nameEntries[i].SetText(p.editor.character.CustomNames[i])
		p.rankEntries[i].SetText(p.editor.character.CustomRanks[i])
	}
	p.refreshRankAttributes()
}

func (p *PointBuyUI) makeSkillEntryOnChanged(index int) func(string) {
	return func(s string) {
		p.editor.character.CustomSkills[index] = strings.ToUpper(s)
	}
}

func (p *PointBuyUI) makeNameEntryOnChanged(index int) func(string) {
	return func(s string) {
		p.editor.character.CustomNames[index] = s
	}
}

func (p *PointBuyUI) makeRankEntryOnChanged(index int) func(string) {
	return func(s string) {
		if s == "" {
			p.editor.character.CustomRanks[index] = ""
			return
		}
		// Allow single integer (including -1) or comma-separated list of integers
		parts := strings.Split(s, ",")
		valid := true
		for _, part := range parts {
			part = strings.TrimSpace(part)
			if _, err := strconv.Atoi(part); err != nil {
				valid = false
				break
			}
		}

		if !valid {
			p.editor.lastError = fmt.Sprintf("Invalid rank format for skill %d. Must be integer(s) (e.g., '-1' or '0,5,10').", index)
			// Don't revert text immediately to allow user to correct it,
			// but don't save invalid state to struct if strictness is desired.
			// For now, we update the struct but leave the error in lastError so they see it.
		}

		p.editor.character.CustomRanks[index] = s
	}
}
