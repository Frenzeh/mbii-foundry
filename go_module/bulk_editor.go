package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

type BulkEditor struct {
	container *container.Split

	files     []string
	selection map[string]bool

	fileList    *widget.List
	fieldSelect *widget.Select
	valueEntry  *widget.Entry
	applyBtn    *widget.Button
}

func NewBulkEditor() *BulkEditor {
	be := &BulkEditor{
		selection: make(map[string]bool),
		files:     []string{},
	}
	be.createUI()
	return be
}

func (be *BulkEditor) createUI() {
	be.fileList = widget.NewList(
		func() int { return len(be.files) },
		func() fyne.CanvasObject {
			return container.NewHBox(widget.NewCheck("", nil), widget.NewLabel("File"))
		},
		func(id widget.ListItemID, obj fyne.CanvasObject) {
			path := be.files[id]
			check := obj.(*fyne.Container).Objects[0].(*widget.Check)
			check.Checked = be.selection[path]
			check.OnChanged = func(b bool) { be.selection[path] = b }

			label := obj.(*fyne.Container).Objects[1].(*widget.Label)
			label.SetText(filepath.Base(path))
		},
	)

	be.fieldSelect = widget.NewSelect([]string{"MaxHealth", "MaxArmor", "Speed", "ForcePool"}, nil)
	be.valueEntry = NewInputEntry()
	be.valueEntry.SetPlaceHolder("New Value")

	be.applyBtn = widget.NewButtonWithIcon("Apply to Selected", theme.ConfirmIcon(), be.applyChanges)

	controls := container.NewVBox(
		widget.NewLabelWithStyle("Bulk Operations", fyne.TextAlignCenter, fyne.TextStyle{Bold: true}),
		widget.NewLabel("Field:"),
		be.fieldSelect,
		widget.NewLabel("Value:"),
		be.valueEntry,
		layout.NewSpacer(),
		be.applyBtn,
	)

	be.container = container.NewHSplit(container.NewPadded(be.fileList), container.NewPadded(controls))
	be.container.SetOffset(0.7)
}

func (be *BulkEditor) GetContent() fyne.CanvasObject { return be.container }

func (be *BulkEditor) LoadFiles(paths []string) {
	be.files = paths
	be.selection = make(map[string]bool)
	for _, p := range paths {
		be.selection[p] = false
	}
	be.fileList.Refresh()
}

func (be *BulkEditor) applyChanges() {
	count := 0
	for path, selected := range be.selection {
		if selected {
			if err := be.processFile(path); err == nil {
				count++
			}
		}
	}
	dialog.ShowInformation("Bulk Update", fmt.Sprintf("Updated %d files.", count), fyne.CurrentApp().Driver().AllWindows()[0])
}

func (be *BulkEditor) processFile(path string) error {
	content, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	lines := strings.Split(string(content), "\n")
	var newLines []string

	fieldMap := map[string]string{
		"MaxHealth": "maxhealth",
		"MaxArmor":  "maxarmor",
		"Speed":     "speed",
		"ForcePool": "forcepool",
	}

	targetKey := fieldMap[be.fieldSelect.Selected]
	if targetKey == "" {
		return fmt.Errorf("invalid field")
	}

	val := be.valueEntry.Text

	for _, line := range lines {
		trim := strings.TrimSpace(line)
		if strings.HasPrefix(trim, targetKey) {
			newLines = append(newLines, fmt.Sprintf("\t%s\t\t%s", targetKey, val))
			continue
		}
		newLines = append(newLines, line)
	}

	return os.WriteFile(path, []byte(strings.Join(newLines, "\n")), 0644)
}
