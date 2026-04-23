package main

// Compact update callout that lives inside the Home footer's right
// column, next to the version. Not a top-of-screen banner — the user
// prefers the update signal to land in the same visual region as the
// current version string, so the relationship between "you are on X"
// and "Y is available" reads immediately.
//
// Factory: NewUpdateCallout(app, info) returns nil when no actionable
// update exists (info nil, not newer, or already dismissed). Callers
// can then just skip adding the widget — no empty Spacer required.

import (
	"fmt"
	"net/url"
	"runtime"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// UpdateCallout is the compact footer-mounted variant of the old
// top-banner design. It renders:
//
//	[ ↓ v0.4.0-alpha available ] [ Install ] [ Notes ] [ × ]
//
// during idle, and collapses to a status/progress row during install.
type UpdateCallout struct {
	app       *App
	info      *UpdateInfo
	dismissed bool

	statusLabel *widget.Label
	progressBar *widget.ProgressBar
	installBtn  *ToolbarButton
	notesBtn    *ToolbarButton
	dismissBtn  *ToolbarButton
	actionRow   *fyne.Container
	statusRow   *fyne.Container
	content     *fyne.Container
}

// NewUpdateCallout returns a callout widget, or nil when there's
// nothing to announce. Callers treat nil as "don't add anything to
// the layout."
func NewUpdateCallout(a *App, info *UpdateInfo) *UpdateCallout {
	if info == nil || !info.IsNewer {
		return nil
	}
	return &UpdateCallout{app: a, info: info}
}

func (uc *UpdateCallout) GetContent() fyne.CanvasObject {
	if uc.dismissed {
		return layout.NewSpacer()
	}

	headline := canvas.NewText(
		fmt.Sprintf("↓ v%s available", trimV(uc.info.TagName)),
		CurrentThemeColor,
	)
	headline.TextSize = SizeSmall
	headline.TextStyle = fyne.TextStyle{Bold: true}
	headline.Alignment = fyne.TextAlignTrailing

	released := canvas.NewText(
		"Released "+formatReleaseDate(uc.info.PublishedAt),
		theme.PlaceHolderColor(),
	)
	released.TextSize = SizeSmall
	released.TextStyle = fyne.TextStyle{Italic: true}
	released.Alignment = fyne.TextAlignTrailing

	installLabel := "Install"
	if runtime.GOOS == "windows" {
		installLabel = "Download"
	}
	uc.installBtn = NewToolbarButton(installLabel, theme.DownloadIcon(),
		uc.runInstall, "Install the newer version")
	uc.notesBtn = NewToolbarButton("Notes", theme.InfoIcon(),
		uc.openReleasePage, "Open the release page in your browser")
	uc.dismissBtn = NewToolbarButton("", theme.CancelIcon(),
		uc.dismiss, "Dismiss for this session")

	uc.actionRow = container.NewHBox(
		layout.NewSpacer(), // right-align to match the version above
		uc.installBtn, uc.notesBtn, uc.dismissBtn,
	)

	uc.statusLabel = widget.NewLabel("")
	uc.statusLabel.TextStyle = fyne.TextStyle{Italic: true}
	uc.statusLabel.Alignment = fyne.TextAlignTrailing
	uc.progressBar = widget.NewProgressBar()
	uc.statusRow = container.NewVBox(uc.statusLabel, uc.progressBar)
	uc.statusRow.Hide()

	uc.content = container.NewVBox(
		headline,
		released,
		uc.actionRow,
		uc.statusRow,
	)
	return uc.content
}

func (uc *UpdateCallout) dismiss() {
	uc.dismissed = true
	if uc.content != nil {
		uc.content.Hide()
	}
}

func (uc *UpdateCallout) openReleasePage() {
	if uc.info == nil || uc.info.HTMLURL == "" {
		return
	}
	u, err := url.Parse(uc.info.HTMLURL)
	if err != nil {
		return
	}
	_ = uc.app.fyneApp.OpenURL(u)
}

func (uc *UpdateCallout) runInstall() {
	// Windows has no supported auto-install — open the release page
	// so the user can re-download + reinstall manually.
	if runtime.GOOS == "windows" {
		uc.openReleasePage()
		return
	}

	asset := uc.app.updateChecker.AssetForThisPlatform()
	if asset == nil {
		dialog.ShowInformation("Nothing to download",
			"The latest release doesn't include a build for this platform. "+
				"You can grab source or other artifacts from the release page.",
			uc.app.mainWindow)
		return
	}

	uc.installBtn.Disable()
	uc.notesBtn.Disable()
	uc.statusLabel.SetText("Starting update…")
	uc.progressBar.SetValue(0)
	uc.actionRow.Hide()
	uc.statusRow.Show()
	uc.content.Refresh()

	go func() {
		err := InstallUpdate(asset, func(p UpdateProgress) {
			fyne.Do(func() {
				uc.statusLabel.SetText(p.Message)
				if p.Percent >= 0 {
					uc.progressBar.SetValue(p.Percent / 100)
				}
			})
		})
		if err != nil {
			fyne.Do(func() {
				uc.installBtn.Enable()
				uc.notesBtn.Enable()
				uc.statusRow.Hide()
				uc.actionRow.Show()
				uc.content.Refresh()
				dialog.ShowError(
					fmt.Errorf("Couldn't install the update.\n\n%v\n\n"+
						"You can still download it manually from the release page.", err),
					uc.app.mainWindow)
			})
			return
		}
		fyne.Do(func() {
			uc.statusLabel.SetText("Update staged — relaunching…")
		})
	}()
}

func trimV(tag string) string {
	if len(tag) > 1 && (tag[0] == 'v' || tag[0] == 'V') {
		return tag[1:]
	}
	return tag
}

func formatReleaseDate(t time.Time) string {
	if t.IsZero() {
		return "recently"
	}
	d := time.Since(t)
	switch {
	case d < 24*time.Hour:
		return "today"
	case d < 48*time.Hour:
		return "yesterday"
	case d < 7*24*time.Hour:
		return fmt.Sprintf("%d days ago", int(d.Hours()/24))
	default:
		return t.Format("Jan 2, 2006")
	}
}
