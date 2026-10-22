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

type contributionSubmitter interface {
	SwitchToUpstream() error
	CreateBranch(string) error
	StageAndCommit(string) error
	PushCurrentBranch() error
	OpenPullRequest(string, string) (string, error)
}

// Both submission modes share the same publication consent and error handling.
func submitContribution(manager contributionSubmitter, branchName, title, description string, createPR bool) (string, error) {
	if branchName != "" {
		// Preserve the existing new-branch workflow: if local edits cannot move
		// to upstream, branch from the current HEAD instead.
		_ = manager.SwitchToUpstream()
		if err := manager.CreateBranch(branchName); err != nil {
			return "", err
		}
	}
	if err := manager.StageAndCommit(title); err != nil {
		return "", err
	}
	if err := manager.PushCurrentBranch(); err != nil {
		return "", err
	}
	if !createPR {
		return "Changes pushed successfully. No pull request was requested.", nil
	}
	url, err := manager.OpenPullRequest(title, description)
	if err != nil {
		return "", fmt.Errorf("changes were pushed, but the pull request could not be opened: %w", err)
	}
	return "Changes pushed successfully. Pull request ready:\n\n" + url, nil
}

func (a *App) showSubmissionWizard() {
	if a.config.GitHubToken == "" {
		a.showCredentialConnect(a.showSubmissionWizard)
		return
	}
	manager := a.githubManager
	if manager == nil {
		dialog.ShowInformation("Git Not Configured", "Please setup your workspace first.", a.mainWindow)
		return
	}

	// 1. Check Status
	changes, err := manager.GetStatus()
	if err != nil {
		dialog.ShowError(fmt.Errorf("Failed to check status: %v", err), a.mainWindow)
		return
	}

	if len(changes) == 0 {
		dialog.ShowInformation("No Changes", "You have no modified files to submit.", a.mainWindow)
		return
	}

	// 2. Prepare Form
	currentBranch, err := manager.GetCurrentBranch()
	if err != nil {
		dialog.ShowError(err, a.mainWindow)
		return
	}
	isAdvancedUser := currentBranch != "master" && currentBranch != manager.UpstreamBranch

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

	var submitBtn *widget.Button
	submitBtn = widget.NewButtonWithIcon("Submit Contribution", theme.MailSendIcon(), func() {
		title, description := titleEntry.Text, descEntry.Text
		createPR := prCheck.Checked
		if title == "" {
			dialog.ShowError(fmt.Errorf("Please provide a title/message."), a.mainWindow)
			return
		}
		branchName := ""
		if modeSelect.Selected != "Update Current Branch" {
			timestamp := time.Now().Format("20060102-150405")
			safeTitle := strings.ReplaceAll(strings.ToLower(title), " ", "-")
			branchName = fmt.Sprintf("contrib/%s-%s", safeTitle, timestamp)
		}

		submitBtn.Disable()
		progress := dialog.NewProgressInfinite("Submitting...", "Processing changes...", a.mainWindow)
		progress.Show()
		go func() {
			resultMsg, err := submitContribution(manager, branchName, title, description, createPR)
			fyne.Do(func() {
				progress.Hide()
				submitBtn.Enable()
				if err != nil {
					dialog.ShowError(err, a.mainWindow)
				} else {
					dialog.ShowInformation("Success!", resultMsg, a.mainWindow)
					if subWindow != nil {
						subWindow.Close()
					}
				}
			})
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
