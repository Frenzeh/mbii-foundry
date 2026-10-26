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

// CustomSkillItem represents a single custom build skill (c_att_skill_X)
type CustomSkillItem struct {
	Index int
	Skill string
	Name  string
	Ranks string
	Descs string
}

// CustomSkillsEditor provides a visual editor for Custom Build skill trees in MBCH characters.
type CustomSkillsEditor struct {
	character *parsers.MBCHCharacter
	skills    []*CustomSkillItem
	addButton *widget.Button
	cardList  *fyne.Container
	onChange  func()
}

func NewCustomSkillsEditor(char *parsers.MBCHCharacter, onChange func()) *CustomSkillsEditor {
	cse := &CustomSkillsEditor{
		character: char,
		cardList:  container.NewVBox(),
		onChange:  onChange,
	}
	cse.LoadFromCharacter(char)
	return cse
}

// LoadFromCharacter projects the canonical arrays without moving slot indices.
func (cse *CustomSkillsEditor) LoadFromCharacter(char *parsers.MBCHCharacter) {
	cse.character = char
	cse.skills = nil

	if char == nil {
		cse.rebuildUI()
		return
	}

	for i := range char.CustomSkills {
		skillVal := char.CustomSkills[i]
		nameVal := char.CustomNames[i]
		rankVal := char.CustomRanks[i]
		descVal := char.CustomDescs[i]

		if skillVal != "" || nameVal != "" || rankVal != "" || descVal != "" {
			cse.skills = append(cse.skills, &CustomSkillItem{
				Index: i,
				Skill: skillVal,
				Name:  nameVal,
				Ranks: rankVal,
				Descs: descVal,
			})
		}
	}

	cse.rebuildUI()
}

// SyncToCharacter writes the projected slots back to their canonical indices.
func (cse *CustomSkillsEditor) SyncToCharacter() {
	if cse.character == nil {
		return
	}
	cse.character.CustomSkills = [parsers.PointbuyMaxTotalSlots]string{}
	cse.character.CustomNames = [parsers.PointbuyMaxTotalSlots]string{}
	cse.character.CustomRanks = [parsers.PointbuyMaxTotalSlots]string{}
	cse.character.CustomDescs = [parsers.PointbuyMaxTotalSlots]string{}
	for _, item := range cse.skills {
		if item.Index < 0 || item.Index >= parsers.PointbuyMaxTotalSlots {
			continue
		}
		cse.character.CustomSkills[item.Index] = item.Skill
		cse.character.CustomNames[item.Index] = item.Name
		cse.character.CustomRanks[item.Index] = item.Ranks
		cse.character.CustomDescs[item.Index] = item.Descs
	}

	if cse.onChange != nil {
		cse.onChange()
	}
}

func (cse *CustomSkillsEditor) nextFreeSlot() int {
	if cse.character == nil {
		return -1
	}
	specs := cse.character.HasCustomSpec
	if specs < 1 {
		specs = 1
	}
	if specs > parsers.PointbuyMaxArchetypes {
		specs = parsers.PointbuyMaxArchetypes
	}
	// Empty cards retain their slots until explicitly deleted.
	var reserved [parsers.PointbuyMaxTotalSlots]bool
	for _, item := range cse.skills {
		if item.Index >= 0 && item.Index < len(reserved) {
			reserved[item.Index] = true
		}
	}
	for i := range specs * parsers.PointbuySlotsPerArchetype {
		if !reserved[i] && cse.character.CustomSkills[i] == "" && cse.character.CustomNames[i] == "" &&
			cse.character.CustomRanks[i] == "" && cse.character.CustomDescs[i] == "" {
			return i
		}
	}
	return -1
}

func (cse *CustomSkillsEditor) BuildUI() fyne.CanvasObject {
	headerLbl := widget.NewLabelWithStyle("Custom Build Skills (c_att_skill_*)", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	headerLbl.Wrapping = fyne.TextWrapWord
	infoLbl := widget.NewLabel("Configure character-specific custom build trees, rank costs, and level descriptions.")
	infoLbl.Wrapping = fyne.TextWrapWord

	addBtn := widget.NewButtonWithIcon("Add Custom Skill", theme.ContentAddIcon(), func() {
		slot := cse.nextFreeSlot()
		if slot < 0 {
			return
		}
		cse.skills = append(cse.skills, &CustomSkillItem{
			Index: slot,
			Skill: "MB_ATT_INVALID",
			Name:  "^7New Skill",
			Ranks: "1,1",
			Descs: "^7Level 1: Skill effect\n^7Level 2: Enhanced effect",
		})
		cse.SyncToCharacter()
		cse.rebuildUI()
	})
	addBtn.Importance = widget.HighImportance
	cse.addButton = addBtn
	cse.rebuildUI()

	// The MBCH tab owns scrolling for the full form.
	return container.NewVBox(
		headerLbl,
		infoLbl,
		container.NewHBox(addBtn),
		widget.NewSeparator(),
		cse.cardList,
	)
}

func (cse *CustomSkillsEditor) rebuildUI() {
	if cse.addButton != nil {
		if cse.nextFreeSlot() < 0 {
			cse.addButton.Disable()
		} else {
			cse.addButton.Enable()
		}
	}
	cse.cardList.Objects = nil

	if len(cse.skills) == 0 {
		emptyMsg := widget.NewLabelWithStyle("No custom skills configured for this character.", fyne.TextAlignCenter, fyne.TextStyle{Italic: true})
		emptyMsg.Wrapping = fyne.TextWrapWord
		cse.cardList.Add(container.NewPadded(emptyMsg))
		cse.cardList.Refresh()
		return
	}

	for i, item := range cse.skills {
		idx := i
		skillRef := item

		titleLbl := widget.NewLabelWithStyle(fmt.Sprintf("Custom Skill #%d", item.Index), fyne.TextAlignLeading, fyne.TextStyle{Bold: true})

		nameEntry := widget.NewEntry()
		nameEntry.SetText(skillRef.Name)
		nameEntry.SetPlaceHolder("^7Display Name (e.g. ^3Dash)")
		nameEntry.OnChanged = func(s string) {
			skillRef.Name = s
			cse.SyncToCharacter()
		}

		skillEntry := widget.NewEntry()
		skillEntry.SetText(skillRef.Skill)
		skillEntry.SetPlaceHolder("MB_ATT_* or custom ID")
		skillEntry.OnChanged = func(s string) {
			skillRef.Skill = s
			cse.SyncToCharacter()
		}

		ranksEntry := widget.NewEntry()
		ranksEntry.SetText(skillRef.Ranks)
		ranksEntry.SetPlaceHolder("Rank costs (e.g. 1,2,3)")
		ranksEntry.OnChanged = func(s string) {
			skillRef.Ranks = s
			cse.SyncToCharacter()
		}

		descEntry := widget.NewMultiLineEntry()
		descEntry.Wrapping = fyne.TextWrapWord
		descEntry.SetText(skillRef.Descs)
		descEntry.SetPlaceHolder("Level descriptions (newline separated)")
		descEntry.SetMinRowsVisible(3)
		descEntry.OnChanged = func(s string) {
			skillRef.Descs = s
			cse.SyncToCharacter()
		}

		deleteBtn := widget.NewButtonWithIcon("", theme.DeleteIcon(), func() {
			cse.skills = append(cse.skills[:idx], cse.skills[idx+1:]...)
			cse.SyncToCharacter()
			cse.rebuildUI()
		})
		deleteBtn.Importance = widget.DangerImportance

		form := container.NewVBox(
			container.NewBorder(nil, nil, titleLbl, deleteBtn),
			container.NewVBox(widget.NewLabel("Display Name:"), nameEntry),
			container.NewVBox(widget.NewLabel("Skill Enum / ID:"), skillEntry),
			container.NewVBox(widget.NewLabel("Rank Costs:"), ranksEntry),
			container.NewVBox(widget.NewLabel("Rank Descriptions:"), descEntry),
		)

		bg := canvas.NewRectangle(color.RGBA{R: 32, G: 36, B: 46, A: 255})
		bg.CornerRadius = 6

		card := container.NewStack(bg, container.NewPadded(form))
		cse.cardList.Add(card)
	}

	cse.cardList.Refresh()
}
