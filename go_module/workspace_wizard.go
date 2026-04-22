package main

import (
	"fmt"
	"path/filepath"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

func (a *App) showWorkspaceSetupWizard() {
	if a.config.GitHubToken == "" {
		dialog.ShowInformation("GitHub Token Required", "Please configure your GitHub Token in Preferences first.", a.mainWindow)
		return
	}

	// 1. Path Selection
	pathEntry := NewInputEntry()
	if a.config.TextAssetsPath != "" {
		pathEntry.SetText(a.config.TextAssetsPath)
	} else {
		// Default to a folder next to Gamedata or in Documents
		// configDir, _ := os.UserConfigDir()
		// pathEntry.SetText(filepath.Join(configDir, "mbii-fa-creator", "TextAssets"))
	}
	pathEntry.PlaceHolder = "Select where to download TextAssets..."

	pathSelectBtn := widget.NewButtonWithIcon("", theme.FolderOpenIcon(), func() {
		dialog.ShowFolderOpen(func(uri fyne.ListableURI, err error) {
			if uri != nil {
				pathEntry.SetText(filepath.Join(uri.Path(), "TextAssets"))
			}
		}, a.mainWindow)
	})

	// 2. Status / Log
	logLabel := widget.NewLabel("Ready to initialize.")
	logLabel.Wrapping = fyne.TextWrapWord

	progressBar := widget.NewProgressBarInfinite()
	progressBar.Hide()

	// 3. Action
	var setupWindow fyne.Window

	setupBtn := widget.NewButton("Initialize Workspace", func() {
		targetPath := pathEntry.Text
		if targetPath == "" {
			dialog.ShowError(fmt.Errorf("Please select a destination folder."), a.mainWindow)
			return
		}

		// Update Config
		a.config.TextAssetsPath = targetPath
		a.saveConfig()

		// Re-init Manager with new path
		a.githubManager = NewGitHubManager(a.config.GitHubToken, targetPath)

		// Start Process
		progressBar.Show()
		logLabel.SetText("Starting setup...")

		go func() {
			err := a.githubManager.SetupWorkspace(func(msg string) {
				logLabel.SetText(msg)
			})

			if err == nil {
				logLabel.SetText("Detecting development branch...")
				branch, errBranch := a.githubManager.DetectDevelopmentBranch()
				if errBranch == nil {
					logLabel.SetText("Tracking upstream branch: " + branch)
				} else {
					logLabel.SetText("Warning: Defaulting to master. (" + errBranch.Error() + ")")
				}
			}

			progressBar.Hide()

			if err != nil {
				dialog.ShowError(err, a.mainWindow)
				logLabel.SetText("Error: " + err.Error())
			} else {
				dialog.ShowInformation("Success", "Workspace initialized successfully!", a.mainWindow)
				if setupWindow != nil {
					setupWindow.Close()
				}
				// Refresh Asset Browser
				if a.assetBrowser != nil {
					a.assetBrowser.SetPaths(a.config.GamedataPath, a.config.TextAssetsPath)
				}
			}
		}()
	})
	setupBtn.Importance = widget.HighImportance

	content := container.NewVBox(
		widget.NewRichTextFromMarkdown("# Setup Workspace\n\nThis will **Fork** and **Clone** the official MBII TextAssets repository so you can make changes and submit them."),
		widget.NewSeparator(),
		widget.NewLabel("Destination Folder:"),
		container.NewBorder(nil, nil, nil, pathSelectBtn, pathEntry),
		widget.NewSeparator(),
		logLabel,
		progressBar,
		container.NewPadded(setupBtn),
	)

	setupWindow = a.fyneApp.NewWindow("Workspace Setup")
	setupWindow.SetContent(container.NewPadded(content))
	setupWindow.Resize(fyne.NewSize(500, 300))
	setupWindow.Show()
}
