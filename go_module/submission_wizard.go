package main

import (
	"fmt"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

func (a *App) showSubmissionWizard() {
	if a.githubManager == nil {
		dialog.ShowInformation("Git Not Configured", "Please setup your workspace first.", a.mainWindow)
		return
	}

	// 1. Check Status
	changes, err := a.githubManager.GetStatus()
	if err != nil {
		dialog.ShowError(fmt.Errorf("Failed to check status: %v", err), a.mainWindow)
		return
	}

	if len(changes) == 0 {
		dialog.ShowInformation("No Changes", "You have no modified files to submit.", a.mainWindow)
		return
	}

	// 2. Prepare Form
	currentBranch, _ := a.githubManager.GetCurrentBranch()
	isAdvancedUser := currentBranch != "master" && currentBranch != a.githubManager.UpstreamBranch

	// Mode Selection (if on feature branch)
	modeSelect := widget.NewSelect([]string{"Create New Request (Recommended)", "Update Current Branch"}, nil)
	if isAdvancedUser {
		modeSelect.SetSelected("Update Current Branch")
	} else {
		modeSelect.SetSelected("Create New Request (Recommended)")
		modeSelect.Disable() // No choice for master
	}

	titleEntry := NewInputEntry()
	titleEntry.PlaceHolder = "Brief summary of changes (e.g., Fix Clone Stats)"

	// Pre-fill title if updating existing branch?
	if isAdvancedUser {
		titleEntry.SetText("Update " + currentBranch)
	}

	descEntry := NewMultiLineInputEntry()
	descEntry.PlaceHolder = "Detailed description of what you changed and why..."
	descEntry.SetMinRowsVisible(5)

	prCheck := widget.NewCheck("Create Pull Request", nil)
	prCheck.Checked = true

	fileList := widget.NewList(
		func() int { return len(changes) },
		func() fyne.CanvasObject { return widget.NewLabel("File") },
		func(id widget.ListItemID, obj fyne.CanvasObject) {
			obj.(*widget.Label).SetText(changes[id])
		},
	)

	// 3. Submit Action
	var subWindow fyne.Window

	submitBtn := widget.NewButtonWithIcon("Submit Contribution", theme.MailSendIcon(), func() {
		if titleEntry.Text == "" {
			dialog.ShowError(fmt.Errorf("Please provide a title/message."), a.mainWindow)
			return
		}

		progress := dialog.NewProgressInfinite("Submitting...", "Processing changes...", a.mainWindow)
		progress.Show()

		go func() {
			var err error
			var resultMsg string

			if modeSelect.Selected == "Update Current Branch" {
				// Advanced Path: Commit -> Push -> Optional PR
				err = a.githubManager.StageAndCommit(titleEntry.Text)
				if err == nil {
					err = a.githubManager.PushCurrentBranch()
				}
				if err == nil && prCheck.Checked {
					url, prErr := a.githubManager.OpenPullRequest(titleEntry.Text, descEntry.Text)
					if prErr == nil {
						resultMsg = "Branch updated and PR created!\n" + url
					} else {
						// PR might already exist, which is fine
						resultMsg = "Branch updated successfully!"
					}
				} else if err == nil {
					resultMsg = "Branch updated successfully!"
				}
			} else {
				// Standard Path: New Branch -> PR
				timestamp := time.Now().Format("20060102-150405")
				safeTitle := strings.ReplaceAll(strings.ToLower(titleEntry.Text), " ", "-")
				branchName := fmt.Sprintf("contrib/%s-%s", safeTitle, timestamp)

				var url string
				url, err = a.githubManager.CreateContribution(branchName, titleEntry.Text, descEntry.Text)
				resultMsg = "Pull Request created successfully!\n\n" + url
			}

			progress.Hide()

			if err != nil {
				dialog.ShowError(err, a.mainWindow)
			} else {
				dialog.ShowInformation("Success!", resultMsg, a.mainWindow)
				if subWindow != nil {
					subWindow.Close()
				}
			}
		}()
	})
	submitBtn.Importance = widget.HighImportance

	// Dynamic Layout based on mode
	formContainer := container.NewVBox(
		modeSelect,
		widget.NewLabel("Title / Commit Message:"), titleEntry,
		widget.NewLabel("Description (for PR):"), descEntry,
		prCheck,
	)

	content := container.NewBorder(
		widget.NewLabelWithStyle("Review & Submit", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		container.NewVBox(submitBtn),
		nil, nil,
		container.NewVSplit(
			formContainer,
			container.NewBorder(widget.NewLabel("Modified Files:"), nil, nil, nil, fileList),
		),
	)

	subWindow = a.fyneApp.NewWindow("Submit Contribution")
	subWindow.SetContent(container.NewPadded(content))
	subWindow.Resize(fyne.NewSize(500, 600))
	subWindow.Show()
}
