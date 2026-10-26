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
		a.showCredentialConnect(a.showWorkspaceSetupWizard)
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

	var setupBtn *widget.Button
	setupBtn = widget.NewButton("Initialize Workspace", func() {
		targetPath := pathEntry.Text
		if targetPath == "" {
			dialog.ShowError(fmt.Errorf("Please select a destination folder."), a.mainWindow)
			return
		}

		// This worker owns its manager until successful UI-thread publication.
		manager := NewGitHubManager(a.config.GitHubToken, targetPath)
		setupBtn.Disable()
		pathEntry.Disable()
		pathSelectBtn.Disable()
		progressBar.Show()
		logLabel.SetText("Starting setup...")

		go func() {
			err := manager.SetupWorkspace(func(msg string) {
				fyne.Do(func() {
					logLabel.SetText(msg)
				})
			})
			status := ""
			if err == nil {
				fyne.Do(func() {
					logLabel.SetText("Detecting development branch...")
				})
				branch, errBranch := manager.DetectDevelopmentBranch()
				if errBranch == nil {
					status = "Tracking upstream branch: " + branch
				} else {
					status = "Warning: Defaulting to master. (" + errBranch.Error() + ")"
				}
			}

			fyne.Do(func() {
				progressBar.Hide()
				setupBtn.Enable()
				pathEntry.Enable()
				pathSelectBtn.Enable()
				if err != nil {
					dialog.ShowError(err, a.mainWindow)
					logLabel.SetText("Error: " + err.Error())
					return
				}
				a.config.TextAssetsPath = targetPath
				a.saveConfig()
				// Credentials can change in the non-modal Connect window during setup.
				if a.config.GitHubToken != "" && a.config.GitHubToken == manager.token {
					a.githubManager = manager
				} else {
					a.refreshGitHubConnection()
					if a.githubManager != nil {
						// The upstream branch is reusable; the previous account identity is not.
						a.githubManager.UpstreamBranch = manager.UpstreamBranch
					}
				}
				logLabel.SetText(status)
				dialog.ShowInformation("Success", "Workspace initialized successfully!", a.mainWindow)
				if setupWindow != nil {
					setupWindow.Close()
				}
				if a.assetBrowser != nil {
					a.assetBrowser.SetPaths(a.config.GamedataPath, targetPath)
				}
			})
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
