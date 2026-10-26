package main

import (
	"fmt"
	"testing"

	"fyne.io/fyne/v2/test"
	"github.com/Frenzeh/mbii-foundry/parsers"
)

func TestCustomSkillsEditorClearedCardKeepsSlot(t *testing.T) {
	e := loadAuditMBCH(t, auditMBCHCanonicalFixture)
	want := parseAuditMBCH(t, auditMBCHCanonicalFixture)
	cse := e.customSkillsUI
	cse.BuildUI()
	retained := cse.skills[0]
	initialCount := len(cse.skills)
	entries := auditMBCHEntries(auditMBCHSkillCard(t, e, 0))
	if len(entries) != 4 {
		t.Fatalf("custom-skill entries = %d, want four editable fields", len(entries))
	}
	for _, entry := range entries {
		entry.SetText("")
	}
	want.CustomSkills[0], want.CustomNames[0], want.CustomRanks[0], want.CustomDescs[0] = "", "", "", ""
	assertAuditMBCHSkills(t, e.character, want)
	if len(cse.skills) != initialCount || cse.skills[0] != retained {
		t.Fatal("clearing fields removed the retained custom-skill card")
	}

	test.Tap(cse.addButton)
	if len(cse.skills) != initialCount+1 {
		t.Fatalf("custom-skill cards after Add = %d, want %d", len(cse.skills), initialCount+1)
	}
	added := cse.skills[len(cse.skills)-1]
	if retained.Index != 0 || added.Index != 1 {
		t.Fatalf("retained/new canonical slots = %d/%d, want distinct slots 0/1", retained.Index, added.Index)
	}

	for _, edit := range []struct {
		index int
		name  string
		skill string
		ranks string
		descs string
	}{
		{0, "Restored Push", "MB_ATT_PUSH", "2,6,12", "Earlier card edited after Add"},
		{1, "Added Pull", "MB_ATT_PULL", "3,7", "New card kept separately"},
	} {
		entries = auditMBCHEntries(auditMBCHSkillCard(t, e, edit.index))
		if len(entries) != 4 {
			t.Fatalf("slot %d entries = %d, want four editable fields", edit.index, len(entries))
		}
		entries[0].SetText(edit.name)
		entries[1].SetText(edit.skill)
		entries[2].SetText(edit.ranks)
		entries[3].SetText(edit.descs)
		want.CustomNames[edit.index] = edit.name
		want.CustomSkills[edit.index] = edit.skill
		want.CustomRanks[edit.index] = edit.ranks
		want.CustomDescs[edit.index] = edit.descs
	}
	assertAuditMBCHSkills(t, e.character, want)
	assertAuditMBCHSkills(t, parseAuditMBCH(t, e.GenerateSource()), want)
}

func TestCustomSkillsEditorCapacityAndDeletedSlotReuse(t *testing.T) {
	for specs := 1; specs <= parsers.PointbuyMaxArchetypes; specs++ {
		t.Run(fmt.Sprintf("%d_archetypes", specs), func(t *testing.T) {
			useEditorTestApp(t)
			capacity := specs * parsers.PointbuySlotsPerArchetype
			character := &parsers.MBCHCharacter{HasCustomSpec: specs}
			for slot := range capacity - 1 {
				character.CustomSkills[slot] = "MB_ATT_PUSH"
				character.CustomNames[slot] = fmt.Sprintf("Loaded skill %d", slot)
				character.CustomRanks[slot] = "1,3"
				character.CustomDescs[slot] = "Loaded description"
			}
			cse := NewCustomSkillsEditor(character, nil)
			cse.BuildUI()
			retained := cse.skills[0]
			entries := auditMBCHEntries(cse.cardList.Objects[0])
			if len(entries) != 4 {
				t.Fatalf("custom-skill entries = %d, want four editable fields", len(entries))
			}
			for _, entry := range entries {
				entry.SetText("")
			}

			test.Tap(cse.addButton)
			if len(cse.skills) != capacity || cse.skills[0] != retained {
				t.Fatalf("Add failed to retain the empty card within the %d-slot capacity", capacity)
			}
			if added := cse.skills[len(cse.skills)-1]; added.Index != capacity-1 {
				t.Fatalf("new canonical slot = %d, want last free slot %d", added.Index, capacity-1)
			}
			if !cse.addButton.Disabled() || cse.nextFreeSlot() != -1 {
				t.Fatal("an empty retained card did not count toward archetype capacity")
			}
			full := *character
			test.Tap(cse.addButton)
			if len(cse.skills) != capacity {
				t.Fatalf("Add exceeded the %d-slot capacity", capacity)
			}
			assertAuditMBCHSkills(t, character, &full)

			test.Tap(auditMBCHButton(t, cse.cardList.Objects[0], ""))
			if cse.addButton.Disabled() || cse.nextFreeSlot() != 0 {
				t.Fatal("deleting the empty card did not release its canonical slot")
			}
			assertAuditMBCHSkills(t, character, &full)
			test.Tap(cse.addButton)
			if len(cse.skills) != capacity {
				t.Fatalf("custom-skill cards after reusing slot 0 = %d, want %d", len(cse.skills), capacity)
			}
			replacement := cse.skills[len(cse.skills)-1]
			if replacement.Index != 0 || replacement == retained || !cse.addButton.Disabled() {
				t.Fatal("Add did not reuse the deleted card's slot and restore the capacity limit")
			}
			entries = auditMBCHEntries(cse.cardList.Objects[len(cse.skills)-1])
			if len(entries) != 4 {
				t.Fatalf("replacement entries = %d, want four editable fields", len(entries))
			}
			entries[0].SetText("Reused slot")
			entries[1].SetText("MB_ATT_DASH")
			entries[2].SetText("2,5")
			entries[3].SetText("Replacement description")
			full.CustomNames[0] = "Reused slot"
			full.CustomSkills[0] = "MB_ATT_DASH"
			full.CustomRanks[0] = "2,5"
			full.CustomDescs[0] = "Replacement description"
			assertAuditMBCHSkills(t, character, &full)
		})
	}
}
