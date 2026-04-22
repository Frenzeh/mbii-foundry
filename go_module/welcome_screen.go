package main

import (
	"path/filepath"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

type WelcomeScreen struct {
	app *App
}

func NewWelcomeScreen(app *App) *WelcomeScreen {
	return &WelcomeScreen{app: app}
}

func (w *WelcomeScreen) GetContent() fyne.CanvasObject {
	title := widget.NewLabel("FA Creator")
	title.TextStyle = fyne.TextStyle{Bold: true, Monospace: true}
	title.Alignment = fyne.TextAlignCenter

	subtitle := widget.NewLabel("Movie Battles II Content Editor")
	subtitle.Alignment = fyne.TextAlignCenter

	// Action Buttons
	newCharBtn := widget.NewButtonWithIcon("New Character", theme.ContentAddIcon(), func() {
		w.app.createNewFile("Character", NewMBCHEditor(w.app))
	})
	
	newSaberBtn := widget.NewButtonWithIcon("New Saber", theme.ContentAddIcon(), func() {
		w.app.createNewFile("Saber", NewSABEditor(w.app))
	})
	
	newVehBtn := widget.NewButtonWithIcon("New Vehicle", theme.ContentAddIcon(), func() {
		w.app.createNewFile("Vehicle", NewVEHEditor(w.app))
	})
	
	openBtn := widget.NewButtonWithIcon("Open File...", theme.FolderOpenIcon(), func() {
		w.app.openFile()
	})

	// Recent Files List
	recentLabel := widget.NewLabel("Recent Files")
	recentLabel.TextStyle = fyne.TextStyle{Bold: true}
	
	recentList := container.NewVBox()
	
	recentFiles := w.app.fileManager.GetRecentFiles()
	
	if len(recentFiles) > 0 {
		for _, rf := range recentFiles {
			path := rf.Path
			btn := widget.NewButtonWithIcon(filepath.Base(path), theme.FileIcon(), func() {
				w.app.openFileFromPath(path)
			})
			recentList.Add(btn)
		}
	} else {
		recentList.Add(widget.NewLabel("No recent files."))
	}

	// Layout
	actions := container.NewVBox(
		newCharBtn,
		newSaberBtn,
		newVehBtn,
		widget.NewSeparator(),
		openBtn,
	)

	leftCol := container.NewVBox(title, subtitle, widget.NewSeparator(), actions)
	rightCol := container.NewVBox(recentLabel, recentList)

	return container.NewCenter(container.NewHBox(
		container.NewPadded(leftCol),
		widget.NewSeparator(),
		container.NewPadded(rightCol),
	))
}
