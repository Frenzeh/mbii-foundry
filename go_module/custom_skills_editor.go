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

// LoadFromCharacter extracts all c_att_* fields into structured skill items
func (cse *CustomSkillsEditor) LoadFromCharacter(char *parsers.MBCHCharacter) {
	cse.character = char
	cse.skills = nil

	if char == nil || char.ExtraFields == nil {
		cse.rebuildUI()
		return
	}

	for i := 0; i < 20; i++ {
		suffix := strconv.Itoa(i)
		skillVal, hasSkill := char.ExtraFields["c_att_skill_"+suffix]
		nameVal, hasName := char.ExtraFields["c_att_names_"+suffix]
		rankVal, hasRank := char.ExtraFields["c_att_ranks_"+suffix]
		descVal, hasDesc := char.ExtraFields["c_att_descs_"+suffix]

		if hasSkill || hasName || hasRank || hasDesc {
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

// SyncToCharacter writes all skill items back into char.ExtraFields
func (cse *CustomSkillsEditor) SyncToCharacter() {
	if cse.character == nil {
		return
	}
	if cse.character.ExtraFields == nil {
		cse.character.ExtraFields = make(map[string]string)
	}

	// 1. Clean up existing c_att_* keys
	for i := 0; i < 20; i++ {
		suffix := strconv.Itoa(i)
		delete(cse.character.ExtraFields, "c_att_skill_"+suffix)
		delete(cse.character.ExtraFields, "c_att_names_"+suffix)
		delete(cse.character.ExtraFields, "c_att_ranks_"+suffix)
		delete(cse.character.ExtraFields, "c_att_descs_"+suffix)
	}

	// 2. Write active skills in sequential index order
	for idx, item := range cse.skills {
		if strings.TrimSpace(item.Skill) == "" && strings.TrimSpace(item.Name) == "" {
			continue
		}
		suffix := strconv.Itoa(idx)
		if item.Skill != "" {
			cse.character.ExtraFields["c_att_skill_"+suffix] = item.Skill
		}
		if item.Name != "" {
			cse.character.ExtraFields["c_att_names_"+suffix] = item.Name
		}
		if item.Ranks != "" {
			cse.character.ExtraFields["c_att_ranks_"+suffix] = item.Ranks
		}
		if item.Descs != "" {
			cse.character.ExtraFields["c_att_descs_"+suffix] = item.Descs
		}
	}

	if cse.onChange != nil {
		cse.onChange()
	}
}

func (cse *CustomSkillsEditor) BuildUI() fyne.CanvasObject {
	headerLbl := widget.NewLabelWithStyle("Custom Build Skills (c_att_skill_*)", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	infoLbl := widget.NewLabel("Configure character-specific custom build trees, rank costs, and level descriptions.")

	addBtn := widget.NewButtonWithIcon("Add Custom Skill", theme.ContentAddIcon(), func() {
		cse.skills = append(cse.skills, &CustomSkillItem{
			Index: len(cse.skills),
			Skill: "MB_ATT_INVALID",
			Name:  "^7New Skill",
			Ranks: "1,1",
			Descs: "^7Level 1: Skill effect\n^7Level 2: Enhanced effect",
		})
		cse.SyncToCharacter()
		cse.rebuildUI()
	})
	addBtn.Importance = widget.HighImportance

	topBar := container.NewHBox(headerLbl, addBtn)

	scroll := container.NewVScroll(cse.cardList)
	scroll.SetMinSize(fyne.NewSize(600, 360))

	return container.NewBorder(
		container.NewVBox(topBar, infoLbl, widget.NewSeparator()),
		nil, nil, nil,
		scroll,
	)
}

func (cse *CustomSkillsEditor) rebuildUI() {
	cse.cardList.Objects = nil

	if len(cse.skills) == 0 {
		emptyMsg := widget.NewLabelWithStyle("No custom skills configured for this character.", fyne.TextAlignCenter, fyne.TextStyle{Italic: true})
		cse.cardList.Add(container.NewPadded(emptyMsg))
		cse.cardList.Refresh()
		return
	}

	for i, item := range cse.skills {
		idx := i
		skillRef := item

		titleLbl := widget.NewLabelWithStyle(fmt.Sprintf("Custom Skill #%d", idx), fyne.TextAlignLeading, fyne.TextStyle{Bold: true})

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
			container.NewGridWithColumns(2,
				container.NewVBox(widget.NewLabel("Display Name:"), nameEntry),
				container.NewVBox(widget.NewLabel("Skill Enum / ID:"), skillEntry),
			),
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
