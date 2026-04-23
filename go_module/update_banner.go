package main

// Home-screen update banner.
//
// Appears at the top of WelcomeScreen when UpdateChecker reports a
// newer release. Compact, dismissible, theme-aware. On Mac/Linux
// the primary action runs the in-place installer; on Windows it
// just opens the release page in the default browser.
//
// The banner is lazy: if no newer version exists, GetContent()
// returns an empty Spacer that collapses to zero. The Welcome
// screen always asks for it — costs nothing when there's no update.

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

// UpdateBanner renders the "new version available" strip and wires
// the Install / Later / dismiss actions.
type UpdateBanner struct {
	app         *App
	info        *UpdateInfo
	dismissed   bool
	statusLabel *widget.Label
	progressBar *widget.ProgressBar
	installBtn  *ToolbarButton
	laterBtn    *ToolbarButton
	dismissBtn  *ToolbarButton
	content     *fyne.Container
}

// NewUpdateBanner creates the banner. If info is nil or IsNewer is
// false, GetContent returns an empty spacer so the host layout
// stays flush.
func NewUpdateBanner(a *App, info *UpdateInfo) *UpdateBanner {
	return &UpdateBanner{app: a, info: info}
}

// GetContent returns the banner UI, or an empty Spacer if there's no
// update to advertise.
func (ub *UpdateBanner) GetContent() fyne.CanvasObject {
	if ub.info == nil || !ub.info.IsNewer || ub.dismissed {
		return layout.NewSpacer()
	}

	headline := canvas.NewText(
		fmt.Sprintf("FOUNDRY %s IS AVAILABLE", trimV(ub.info.TagName)),
		theme.ForegroundColor())
	headline.TextSize = SizeSubtitle
	headline.TextStyle = fyne.TextStyle{Bold: true}

	currentMeta := fmt.Sprintf("You're on %s · released %s",
		AppVersion, formatReleaseDate(ub.info.PublishedAt))
	subtitle := canvas.NewText(currentMeta, theme.PlaceHolderColor())
	subtitle.TextSize = SizeSmall

	ub.statusLabel = widget.NewLabel("")
	ub.statusLabel.TextStyle = fyne.TextStyle{Italic: true}
	ub.statusLabel.Hide()

	ub.progressBar = widget.NewProgressBar()
	ub.progressBar.Hide()

	installLabel := "Install Update"
	if runtime.GOOS == "windows" {
		installLabel = "Download"
	}
	ub.installBtn = NewToolbarButton(installLabel, theme.DownloadIcon(),
		ub.runInstall, "Install the newer version")
	ub.laterBtn = NewToolbarButton("Release Notes", theme.InfoIcon(),
		ub.openReleasePage, "Open the GitHub release page in your browser")
	ub.dismissBtn = NewToolbarButton("", theme.CancelIcon(),
		func() { ub.dismiss() }, "Dismiss for this session")

	buttonRow := container.NewHBox(ub.installBtn, ub.laterBtn)

	textCol := container.NewVBox(headline, subtitle, ub.statusLabel, ub.progressBar)

	// Accent rule bookends so the banner reads as part of the chrome
	// rather than an ad.
	ub.content = container.NewVBox(
		NewAccentRule(),
		container.NewBorder(nil, nil, nil, container.NewHBox(buttonRow, ub.dismissBtn), textCol),
		NewAccentRule(),
	)
	return ub.content
}

func (ub *UpdateBanner) dismiss() {
	ub.dismissed = true
	if ub.content != nil {
		ub.content.Hide()
	}
}

// openReleasePage pops the GitHub release URL into the system browser.
// Used both by the "Release Notes" button and as the Windows install
// fallback.
func (ub *UpdateBanner) openReleasePage() {
	if ub.info == nil || ub.info.HTMLURL == "" {
		return
	}
	u, err := url.Parse(ub.info.HTMLURL)
	if err != nil {
		return
	}
	_ = ub.app.fyneApp.OpenURL(u)
}

func (ub *UpdateBanner) runInstall() {
	// Windows has no supported auto-install — just open the release
	// page so the user can re-download + re-install manually.
	if runtime.GOOS == "windows" {
		ub.openReleasePage()
		return
	}

	asset := ub.app.updateChecker.AssetForThisPlatform()
	if asset == nil {
		dialog.ShowInformation("Nothing to download",
			"The latest release doesn't include a build for this platform yet. "+
				"You can grab source or other artifacts from the release page.",
			ub.app.mainWindow)
		return
	}

	// Lock out repeated clicks + show progress UI.
	ub.installBtn.Disable()
	ub.laterBtn.Disable()
	ub.statusLabel.SetText("Starting update…")
	ub.statusLabel.Show()
	ub.progressBar.SetValue(0)
	ub.progressBar.Show()
	ub.content.Refresh()

	// Run the installer off the UI thread; progress callbacks fyne.Do
	// back to the main loop so widget mutations stay safe.
	go func() {
		err := InstallUpdate(asset, func(p UpdateProgress) {
			fyne.Do(func() {
				ub.statusLabel.SetText(p.Message)
				if p.Percent >= 0 {
					ub.progressBar.SetValue(p.Percent / 100)
				}
			})
		})
		if err != nil {
			fyne.Do(func() {
				ub.installBtn.Enable()
				ub.laterBtn.Enable()
				ub.statusLabel.SetText("Update failed — " + err.Error())
				ub.progressBar.Hide()
				ub.content.Refresh()
				dialog.ShowError(
					fmt.Errorf("Couldn't install the update.\n\n%v\n\n"+
						"You can still download it manually from the release page.", err),
					ub.app.mainWindow)
			})
			return
		}
		// On success, InstallUpdate exits the process. If we're still
		// here, something's odd — fall through to an info dialog so
		// the user isn't left guessing.
		fyne.Do(func() {
			ub.statusLabel.SetText("Update staged — relaunching…")
		})
	}()
}

// trimV strips a leading "v" from a tag so "v0.3.0-alpha" becomes
// "0.3.0-alpha" in the banner headline.
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
