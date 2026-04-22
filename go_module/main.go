package main

import (
	"encoding/json"
	"fmt"
	"image/color"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/storage"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

const (
	AppVersion = "2.0"
	AppName    = "MBII Foundry"
)

type App struct {
	fyneApp    fyne.App
	mainWindow fyne.Window

	config     AppConfig
	configPath string

	docTabs *container.DocTabs
	editors map[*container.TabItem]Editor

	assetBrowser *AssetBrowser
	infoPanel    *InfoPanel

	fileManager   *FileManager
	githubManager *GitHubManager

	modpackManager *ModpackManager

	statusLabel    *widget.Label
	split          *container.Split   // Reference to split layout
	sideTabs       *container.AppTabs // Reference to sidebar
	sidebarVisible bool

	// Holocron Integration
	holocronClient *HolocronClient
	holocronStatus *widget.Icon // Visual indicator
}

var CurrentThemeColor color.Color = color.RGBA{R: 0, G: 128, B: 255, A: 255} // Default Blue

type AppConfig struct {
	GamedataPath    string       `json:"gamedata_path"`
	TextAssetsPath  string       `json:"text_assets_path"`
	MD3ViewPath     string       `json:"md3view_path"`
	LastOpenDir     string       `json:"last_open_dir"`
	WindowWidth     float32      `json:"window_width"`
	WindowHeight    float32      `json:"window_height"`
	RecentFiles     []RecentFile `json:"recent_files"`
	Theme           string       `json:"theme"`
	PrimaryColor    string       `json:"primary_color"` // New field
	KnownModpacks   []*Modpack   `json:"known_modpacks"`
	SidebarOffset   float32      `json:"sidebar_offset"`
	SidebarVisible  bool         `json:"sidebar_visible"`
	SetupWizardSeen bool         `json:"setup_wizard_seen"`

	// Pinned folder paths — shown as quick-select shortcuts in file pickers
	// and path-entry fields so users don't have to navigate to the same
	// location repeatedly. Most-recently-pinned first. Max 12 entries.
	FavoritePaths []string `json:"favorite_paths"`

	// GitHub Config
	GitHubToken string `json:"github_token"`
	GitHubUser  string `json:"github_user"`
}

// NewToolbarAction is a helper to create a ToolbarAction
func NewToolbarAction(icon fyne.Resource, tooltip string, action func()) *widget.ToolbarAction {
	return widget.NewToolbarAction(icon, action)
}

// FoundryTheme is the custom amber-gold sci-fi theme. Name is cosmetic;
// derived from the MBII Holocron project's original palette but unrelated
// to the dev-only Holocron network integration (see holocron_client.go).
type FoundryTheme struct{}

func (h FoundryTheme) Color(name fyne.ThemeColorName, variant fyne.ThemeVariant) color.Color {
	if name == theme.ColorNamePrimary {
		return CurrentThemeColor
	}

	// Surface Shifting for Backgrounds
	if name == theme.ColorNameBackground || name == theme.ColorNameInputBackground || name == theme.ColorNameOverlayBackground {
		var base color.Color

		if variant == theme.VariantLight {
			base = theme.DefaultTheme().Color(name, variant)
		} else {
			// Custom "Star Wars" Dark Mode Base
			if name == theme.ColorNameInputBackground {
				base = color.RGBA{R: 15, G: 15, B: 15, A: 255}
			} else {
				base = color.RGBA{R: 28, G: 28, B: 28, A: 255}
			}
		}

		// Tint the background slightly with the primary accent (5%)
		// This creates the "Surface Shift" effect (e.g. reddish dark mode for Sith)
		return blendColors(base, CurrentThemeColor, 0.04)
	}

	return theme.DefaultTheme().Color(name, variant)
}

func blendColors(c1, c2 color.Color, ratio float32) color.Color {
	r1, g1, b1, a1 := c1.RGBA()
	r2, g2, b2, a2 := c2.RGBA()

	// Fast conversion (lossy but fine for UI tinting)
	r1, g1, b1, a1 = r1>>8, g1>>8, b1>>8, a1>>8
	r2, g2, b2, a2 = r2>>8, g2>>8, b2>>8, a2>>8

	inv := 1.0 - ratio

	return color.RGBA{
		R: uint8(float32(r1)*inv + float32(r2)*ratio),
		G: uint8(float32(g1)*inv + float32(g2)*ratio),
		B: uint8(float32(b1)*inv + float32(b2)*ratio),
		A: uint8(a1), // Keep alpha of base
	}
}

func (h FoundryTheme) Font(style fyne.TextStyle) fyne.Resource {
	if embedFont != nil {
		return fyne.NewStaticResource("font.ttf", embedFont)
	}
	return theme.DefaultTheme().Font(style)
}
func (h FoundryTheme) Icon(name fyne.ThemeIconName) fyne.Resource {
	return theme.DefaultTheme().Icon(name)
}
func (h FoundryTheme) Size(name fyne.ThemeSizeName) float32 { return theme.DefaultTheme().Size(name) }

func (a *App) applyThemeColor(colorName string) {
	switch strings.ToLower(colorName) {
	case "red", "sith":
		CurrentThemeColor = color.RGBA{R: 200, G: 0, B: 0, A: 255}
	case "green", "console":
		CurrentThemeColor = color.RGBA{R: 0, G: 200, B: 0, A: 255}
	case "gold", "foundry", "holocron": // "holocron" kept as alias for pre-rebrand configs
		CurrentThemeColor = color.RGBA{R: 255, G: 215, B: 0, A: 255}
	case "blue", "jedi":
		CurrentThemeColor = color.RGBA{R: 0, G: 128, B: 255, A: 255}
	case "orange", "rebel":
		CurrentThemeColor = color.RGBA{R: 255, G: 165, B: 0, A: 255}
	case "purple", "mace":
		CurrentThemeColor = color.RGBA{R: 147, G: 112, B: 219, A: 255}
	default:
		CurrentThemeColor = color.RGBA{R: 0, G: 128, B: 255, A: 255} // Default Blue
	}
	a.fyneApp.Settings().SetTheme(&FoundryTheme{}) // Refresh theme
}

func main() {
	InitLogger()
	defer func() {
		if LogFile != nil {
			LogFile.Close()
		}
	}()

	LogInfo("Starting %s v%s", AppName, AppVersion)

	// Load Definitions
	InitDefinitions()

	// Resolve the data/ folder across install layouts. First match wins.
	// The hardcoded defaults in attribute_data.go ship bare bones ("Name
	// attribute.") — without a real data/ folder, the info panel looks
	// embarrassing. So probe every reasonable location.
	dataPath := resolveDataPath()
	if dataPath == "" {
		LogError("Could not find data/ folder. Info panel descriptions will show raw defaults until the data files are available.")
	} else {
		LogInfo("Loading data from: %s", dataPath)
		if err := LoadExternalData(dataPath); err != nil {
			LogError("Failed to load external data from %s: %v", dataPath, err)
		}
	}

	appConfigDir := AppConfigDir()

	application := &App{
		editors:        make(map[*container.TabItem]Editor),
		holocronClient: NewHolocronClient(),
		fileManager:    NewFileManager(appConfigDir),
	}

	// Initialize GitHub Manager if token exists
	if application.config.GitHubToken != "" {
		repoPath := application.config.TextAssetsPath
		if repoPath == "" {
			// Default to a subdirectory in config dir if not set?
			// Or just nil for now until setup
		}
		application.githubManager = NewGitHubManager(application.config.GitHubToken, repoPath)
	}

	// Dev-only: start background check for local Holocron server.
	// Guarded at creation via NewHolocronClient; nil for regular users so
	// this goroutine noops. See holocron_client.go for context.
	if application.holocronClient != nil {
		go application.monitorHolocronStatus()
	}

	application.fyneApp = app.NewWithID("com.frenzeh.mbii-foundry")
	application.fyneApp.Settings().SetTheme(&FoundryTheme{})
	application.mainWindow = application.fyneApp.NewWindow(fmt.Sprintf("%s - MBII Content Editor", AppName))
	application.mainWindow.Resize(fyne.NewSize(1400, 900))

	application.loadConfig()

	application.setupUI()

	// Check for first run / missing configuration
	application.checkFirstRun()

	application.mainWindow.ShowAndRun()
}

// monitorHolocronStatus polls the local Holocron server and updates the
// dev-mode status icon. Only runs when MBII_FOUNDRY_DEV is set.
func (a *App) monitorHolocronStatus() {
	if a.holocronClient == nil {
		return
	}
	for {
		wasAvailable := a.holocronClient.Available
		isAvailable := a.holocronClient.CheckAvailability()

		if wasAvailable != isAvailable && a.holocronStatus != nil {
			if isAvailable {
				a.holocronStatus.SetResource(theme.ConfirmIcon())
			} else {
				a.holocronStatus.SetResource(theme.CancelIcon())
			}
			a.holocronStatus.Refresh()
		}

		time.Sleep(5 * time.Second)
	}
}

func (a *App) setupUI() {
	a.sidebarVisible = a.config.SidebarVisible

	a.assetBrowser = NewAssetBrowser(a.config.GamedataPath, a.config.TextAssetsPath)
	a.infoPanel = NewInfoPanel()
	a.infoPanel.SetHolocronClient(a.holocronClient)

	a.modpackManager = NewModpackManager(a)

	// DocTabs setup
	a.docTabs = container.NewDocTabs()
	a.docTabs.OnClosed = a.closeTab
	a.docTabs.SetTabLocation(container.TabLocationTop)

	a.statusLabel = widget.NewLabel("Ready")
	a.statusLabel.TextStyle = fyne.TextStyle{Italic: true}

	// Dev-mode status icon. Only displayed when MBII_FOUNDRY_DEV is set;
	// updateMainLayout hides the whole label/icon pair for regular users.
	a.holocronStatus = widget.NewIcon(theme.CancelIcon())

	// Layout
	assetsTab := container.NewTabItem("Assets", a.assetBrowser.GetContent())
	assetsTab.Icon = theme.FolderOpenIcon()

	infoTab := container.NewTabItem("Info", a.infoPanel.GetContent())
	infoTab.Icon = theme.InfoIcon()

	// Info first (default)
	a.sideTabs = container.NewAppTabs(infoTab, assetsTab)
	a.sideTabs.SetTabLocation(container.TabLocationBottom)

	// Initial "Home" Tab
	welcomeScreen := NewWelcomeScreen(a)
	welcomeTab := container.NewTabItem("Home", welcomeScreen.GetContent())
	welcomeTab.Icon = theme.HomeIcon()
	a.docTabs.Append(welcomeTab)
	a.docTabs.Select(welcomeTab)

	a.updateMainLayout()
}

func (a *App) updateMainLayout() {
	// Sidebar Toggle
	toggleIcon := theme.NavigateBackIcon() // Point Left (Show)
	if a.sidebarVisible {
		toggleIcon = theme.NavigateNextIcon() // Point Right (Hide)
	}
	toggleBtn := widget.NewButtonWithIcon("", toggleIcon, func() { a.toggleSidebar() })
	toggleBtn.Importance = widget.LowImportance

	// StatusBar container. The Holocron status indicator only surfaces in
	// dev mode (set MBII_FOUNDRY_DEV=1); regular users see a clean status
	// bar with no mention of internal tooling.
	statusBarItems := []fyne.CanvasObject{
		a.statusLabel,
		layout.NewSpacer(),
		toggleBtn,
	}
	if a.holocronClient != nil {
		statusBarItems = append(statusBarItems,
			widget.NewSeparator(),
			widget.NewLabel("Holocron:"),
			a.holocronStatus,
		)
	}
	statusBar := container.NewHBox(statusBarItems...)

	var centerContent fyne.CanvasObject

	if a.sidebarVisible {
		a.split = container.NewHSplit(a.docTabs, a.sideTabs)
		a.split.SetOffset(float64(a.config.SidebarOffset)) // Use configured offset
		centerContent = a.split
	} else {
		centerContent = a.docTabs
	}

	a.mainWindow.SetContent(container.NewBorder(
		a.createToolbar(),
		statusBar,
		nil,
		nil,
		centerContent,
	))
}

func (a *App) toggleSidebar() {
	a.sidebarVisible = !a.sidebarVisible
	a.config.SidebarVisible = a.sidebarVisible
	a.saveConfig()
	a.updateMainLayout()
	a.mainWindow.Content().Refresh() // Force refresh of main window content
}

func (a *App) createNewFile(title string, editor interface{}) {
	// Check if we should replace the "Start" tab if it's the only one
	if len(a.docTabs.Items) == 1 && a.docTabs.Items[0].Text == "Home" {
		a.docTabs.Remove(a.docTabs.Items[0])
	}

	if ed, ok := editor.(Editor); ok {
		ed.SetAssetBrowser(a.assetBrowser)
		ed.SetOnHover(a.infoPanel.ShowInfo)
		ed.SetHolocronClient(a.holocronClient)

		tab := container.NewTabItem("Untitled "+title, ed.GetContent())
		a.docTabs.Append(tab)
		a.docTabs.Select(tab)
		a.editors[tab] = ed

		// Set up dirty change handler to update tab title
		if mbch, ok := ed.(*MBCHEditor); ok {
			mbch.SetOnDirtyChanged(func(isDirty bool) {
				a.updateTabTitle(tab, isDirty)
			})
		} else if sab, ok := ed.(*SABEditor); ok {
			sab.SetOnDirtyChanged(func(isDirty bool) {
				a.updateTabTitle(tab, isDirty)
			})
		} else if veh, ok := ed.(*VEHEditor); ok {
			veh.SetOnDirtyChanged(func(isDirty bool) {
				a.updateTabTitle(tab, isDirty)
			})
		} else if siege, ok := ed.(*SiegeEditor); ok {
			siege.SetOnDirtyChanged(func(isDirty bool) {
				a.updateTabTitle(tab, isDirty)
			})
		}
	}
}

func (a *App) openFileFromPath(filePath string) {
	ext := strings.ToLower(filepath.Ext(filePath))
	var editor Editor
	var title = filepath.Base(filePath)

	switch ext {
	case ".mbch":
		editor = NewMBCHEditor(a)
	case ".sab":
		editor = NewSABEditor(a)
	case ".veh":
		editor = NewVEHEditor(a)
	case ".siege":
		editor = NewSiegeEditor(a)
	default:
		dialog.ShowInformation("Unknown File Type", "Could not determine editor for this file.", a.mainWindow)
		return
	}

	if editor != nil {
		err := editor.LoadFile(filePath)
		if err != nil {
			ShowError(fmt.Errorf("Failed to load file: %v", err), a.mainWindow)
			return
		}
		// Add to recent files centrally
		a.fileManager.AddRecentFile(filePath)

		// Reuse logic
		a.createNewFile(title, editor)
		tab := a.docTabs.Selected()
		tab.Text = title
		a.docTabs.Refresh()

		a.updateStatus(fmt.Sprintf("Opened %s", title))
	}
}

func (a *App) closeTab(tab *container.TabItem) {
	// Check for unsaved changes
	if editor, ok := a.editors[tab]; ok {
		isDirty := false
		if mbch, ok := editor.(*MBCHEditor); ok {
			isDirty = mbch.IsDirty()
		} else if sab, ok := editor.(*SABEditor); ok {
			isDirty = sab.IsDirty()
		} else if veh, ok := editor.(*VEHEditor); ok {
			isDirty = veh.IsDirty()
		} else if siege, ok := editor.(*SiegeEditor); ok {
			isDirty = siege.IsDirty()
		}

		if isDirty {
			dialog.ShowConfirm("Unsaved Changes",
				"This file has unsaved changes. Close anyway?",
				func(confirmed bool) {
					if confirmed {
						a.removeTab(tab)
					}
				}, a.mainWindow)
			return
		}
	}
	a.removeTab(tab)
}

func (a *App) removeTab(tab *container.TabItem) {
	delete(a.editors, tab)
	// If no tabs left, show Welcome
	if len(a.docTabs.Items) == 0 {
		welcomeScreen := NewWelcomeScreen(a)
		welcomeTab := container.NewTabItem("Home", welcomeScreen.GetContent())
		welcomeTab.Icon = theme.HomeIcon()
		a.docTabs.Append(welcomeTab)
		a.docTabs.Select(welcomeTab)
	}
}

func (a *App) createToolbar() fyne.CanvasObject {
	// Helper for tooltip buttons in toolbar
	btn := func(icon fyne.Resource, action func(), tooltip string) *TooltipButton {
		b := NewTooltipButton("", icon, action, tooltip)
		b.Importance = widget.LowImportance
		return b
	}

	items := []fyne.CanvasObject{
		// File Operations (Left)
		btn(theme.ContentAddIcon(), func() {
			var d dialog.Dialog
			content := container.NewVBox(
				NewTooltipButton("Character (.mbch)", nil, func() { a.createNewFile("Character", NewMBCHEditor(a)); d.Hide() }, "Create a new character file (.mbch)"),
				NewTooltipButton("Saber (.sab)", nil, func() { a.createNewFile("Saber", NewSABEditor(a)); d.Hide() }, "Create a new saber file (.sab)"),
				NewTooltipButton("Vehicle (.veh)", nil, func() { a.createNewFile("Vehicle", NewVEHEditor(a)); d.Hide() }, "Create a new vehicle file (.veh)"),
				NewTooltipButton("Siege (.siege)", nil, func() { a.createNewFile("Siege", NewSiegeEditor(a)); d.Hide() }, "Create a new siege class file (.siege)"),
			)
			d = dialog.NewCustom("Create New File", "Cancel", content, a.mainWindow)
			d.Show()
		}, "Create New File"),
		btn(theme.FolderOpenIcon(), func() { a.openFile() }, "Open File"),
		btn(theme.DocumentSaveIcon(), func() { a.saveFile() }, "Save File"),

		widget.NewSeparator(),

		// Validate
		btn(theme.WarningIcon(), func() { a.validateFile() }, "Validate Current File"),
	}

	// Dev-only: maintainer "share with Holocron" button. Uploads the
	// current file to a local Holocron Ops server for review before a
	// definition change lands in the repo. Only shown when
	// MBII_FOUNDRY_DEV is set (NewHolocronClient returns non-nil).
	if a.holocronClient != nil {
		items = append(items, btn(theme.MailSendIcon(), func() { a.shareFile() }, "Share with Holocron (dev)"))
	}

	items = append(items,
		// Workspace (GitHub)
		btn(theme.StorageIcon(), func() { a.showWorkspaceSetupWizard() }, "Setup TextAssets Workspace"),
		btn(theme.DownloadIcon(), func() { a.syncWorkspace() }, "Update Assets (Sync)"),
		btn(theme.UploadIcon(), func() { a.showSubmissionWizard() }, "Submit Changes to Devs"),

		// Push to Right
		layout.NewSpacer(),

		// Tools & View (Right)
		btn(theme.SettingsIcon(), func() { a.showPreferences() }, "Preferences"),
		btn(theme.InfoIcon(), func() { a.showLogs() }, "Show Debug Logs"),
		btn(theme.HelpIcon(), func() { a.showAbout() }, "About MBII Foundry"),
	)

	return container.NewHBox(items...)
}

func (a *App) validateFile() {
	tab := a.docTabs.Selected()
	if tab == nil {
		return
	}

	editor, ok := a.editors[tab]
	if !ok {
		dialog.ShowInformation("Validation", "No editor open for validation.", a.mainWindow)
		return
	}

	var issues []string
	var charCount int

	if mbch, ok := editor.(*MBCHEditor); ok {
		issues = mbch.Validate()
		charCount = mbch.GetCharacterCount()

		// Add character limit check to validation
		if charCount > 8192 {
			issues = append([]string{fmt.Sprintf("CRITICAL: File exceeds 8192 character limit (%d chars)", charCount)}, issues...)
		} else if charCount > 7500 {
			issues = append([]string{fmt.Sprintf("Warning: Approaching 8192 character limit (%d/8192)", charCount)}, issues...)
		}
	} else if sab, ok := editor.(*SABEditor); ok {
		issues = sab.Validate()
	} else if veh, ok := editor.(*VEHEditor); ok {
		issues = veh.Validate()
	} else if siege, ok := editor.(*SiegeEditor); ok {
		issues = siege.Validate()
	} else {
		dialog.ShowInformation("Validation", "Validation not available for this file type.", a.mainWindow)
		return
	}

	if len(issues) == 0 {
		msg := "✓ No issues found!"
		if charCount > 0 {
			msg += fmt.Sprintf("\n\nCharacter count: %d/8192", charCount)
		}
		dialog.ShowInformation("Validation Passed", msg, a.mainWindow)
	} else {
		msg := fmt.Sprintf("Found %d issue(s):\n\n• %s", len(issues), strings.Join(issues, "\n• "))
		if charCount > 0 {
			msg += fmt.Sprintf("\n\nCharacter count: %d/8192", charCount)
		}
		dialog.ShowInformation("Validation Results", msg, a.mainWindow)
	}
}

func (a *App) showLogs() {
	// Use platform-appropriate temp directory (works on Windows, macOS, Linux)
	logPath := os.TempDir() + string(os.PathSeparator) + "mbii-foundry.log"
	content, err := os.ReadFile(logPath)
	text := ""
	if err != nil {
		text = "Could not read log file: " + err.Error()
	} else {
		text = string(content)
	}

	entry := widget.NewMultiLineEntry()
	entry.SetText(text)
	entry.TextStyle = fyne.TextStyle{Monospace: true}

	w := a.fyneApp.NewWindow("Debug Logs")
	w.SetContent(container.NewScroll(entry))
	w.Resize(fyne.NewSize(800, 600))
	w.Show()
}

func (a *App) shareFile() {
	if !a.holocronClient.Available {
		dialog.ShowInformation("Holocron Offline", "The Holocron system is not connected. Cannot share file.", a.mainWindow)
		return
	}

	tab := a.docTabs.Selected()
	if tab == nil {
		return
	}

	editor, ok := a.editors[tab]
	if !ok {
		return
	}

	// Currently only supporting MBCH for sharing demo
	mbchEditor, ok := editor.(*MBCHEditor)
	if !ok {
		dialog.ShowInformation("Not Supported", "Sharing is currently only supported for Character (.mbch) files.", a.mainWindow)
		return
	}

	dialog.ShowConfirm("Share to Holocron", "Upload this character to the local Holocron server for sharing?", func(b bool) {
		if b {
			var sb strings.Builder
			mbchEditor.WriteContent(&sb)
			content := sb.String()

			name := filepath.Base(mbchEditor.GetCurrentPath())
			if name == "" || name == "." {
				name = "untitled.mbch"
			}

			msg, err := a.holocronClient.ShareFile(name, content, "character")
			if err != nil {
				dialog.ShowError(err, a.mainWindow)
			} else {
				dialog.ShowInformation("Share Successful", msg, a.mainWindow)
			}
		}
	}, a.mainWindow)
}

func (a *App) openFile() {
	filePickerWindow := a.fyneApp.NewWindow("Open File")
	filePickerWindow.Resize(fyne.NewSize(900, 600))

	pickerBrowser := NewAssetBrowser(a.config.GamedataPath, a.config.TextAssetsPath)
	cfp := NewCustomFilePicker(filePickerWindow, pickerBrowser)

	cfp.Show(func(filePath string) {
		if filePath == "" {
			return
		}

		ext := strings.ToLower(filepath.Ext(filePath))

		var editor Editor
		var title string

		switch ext {
		case ".mbch":
			editor = NewMBCHEditor(a)
			title = filepath.Base(filePath)
		case ".sab":
			editor = NewSABEditor(a)
			title = filepath.Base(filePath)
		case ".veh":
			editor = NewVEHEditor(a)
			title = filepath.Base(filePath)
		case ".siege":
			editor = NewSiegeEditor(a)
			title = filepath.Base(filePath)
		default:
			dialog.ShowInformation("Unknown File Type", "Could not determine editor for this file.", a.mainWindow)
			return
		}

		if editor != nil {
			err := editor.LoadFile(filePath)
			if err != nil {
				ShowError(fmt.Errorf("Failed to load file: %v", err), a.mainWindow)
				return
			}

			editor.SetAssetBrowser(a.assetBrowser)
			editor.SetOnHover(a.infoPanel.ShowInfo)
			editor.SetHolocronClient(a.holocronClient)

			tab := container.NewTabItem(title, editor.GetContent())
			a.docTabs.Append(tab)
			a.docTabs.Select(tab)
			a.editors[tab] = editor

			a.updateStatus(fmt.Sprintf("Opened %s", title))
		}
	})
}

func (a *App) saveFile() {
	tab := a.docTabs.Selected()
	if tab == nil {
		return
	}

	editor, ok := a.editors[tab]
	if !ok {
		return
	}

	var path string
	if ed, ok := editor.(*MBCHEditor); ok {
		path = ed.GetCurrentPath()
	}
	if ed, ok := editor.(*SABEditor); ok {
		path = ed.GetCurrentPath()
	}
	if ed, ok := editor.(*VEHEditor); ok {
		path = ed.GetCurrentPath()
	}
	if ed, ok := editor.(*SiegeEditor); ok {
		path = ed.GetCurrentPath()
	}

	if path == "" {
		a.saveFileAs()
		return
	}

	err := editor.SaveFile(path)
	if err != nil {
		ShowError(err, a.mainWindow)
	} else {
		tab.Text = filepath.Base(path) // Remove * indicator
		a.docTabs.Refresh()
		a.updateStatus("Saved " + filepath.Base(path))
	}
}

func (a *App) saveFileAs() {
	tab := a.docTabs.Selected()
	if tab == nil {
		return
	}
	editor, ok := a.editors[tab]
	if !ok {
		return
	}

	// Determine the appropriate extension based on editor type
	var expectedExt string
	switch editor.(type) {
	case *MBCHEditor:
		expectedExt = ".mbch"
	case *SABEditor:
		expectedExt = ".sab"
	case *VEHEditor:
		expectedExt = ".veh"
	case *SiegeEditor:
		expectedExt = ".siege"
	}

	dialog.ShowFileSave(func(uri fyne.URIWriteCloser, err error) {
		if err != nil {
			ShowError(err, a.mainWindow)
			return
		}
		if uri == nil {
			return
		}

		// Get path and close the Fyne handle immediately so we can manage the file ourselves
		path := uri.URI().Path()
		uri.Close()

		// Auto-add extension if missing
		if expectedExt != "" && !strings.HasSuffix(strings.ToLower(path), expectedExt) {
			// Clean up the file Fyne created without extension
			os.Remove(path)
			// Update path with extension
			path = path + expectedExt
		}

		if err := editor.SaveFile(path); err != nil {
			ShowError(err, a.mainWindow)
		} else {
			// Update tab title and status
			tab.Text = filepath.Base(path)
			a.docTabs.Refresh()
			a.updateStatus("Saved to " + path)
		}
	}, a.mainWindow)
}

func (a *App) loadConfig() {
	appConfigDir := AppConfigDir()
	if appConfigDir == "" {
		LogError("Failed to resolve app config dir")
		return
	}
	a.configPath = filepath.Join(appConfigDir, "config.json")

	// Set defaults
	a.config.SidebarVisible = true // Default to true

	data, err := os.ReadFile(a.configPath)
	if err == nil {
		json.Unmarshal(data, &a.config)
	}

	// Set default sidebar offset if not configured
	if a.config.SidebarOffset == 0 {
		a.config.SidebarOffset = 0.8
	}

	// Apply Theme
	if a.config.PrimaryColor == "" {
		a.config.PrimaryColor = "blue"
	}
	a.applyThemeColor(a.config.PrimaryColor)
}

func (a *App) updateStatus(msg string) {
	a.statusLabel.SetText(fmt.Sprintf("[%s] %s", time.Now().Format("15:04:05"), msg))
}

func (a *App) updateTabTitle(tab *container.TabItem, isDirty bool) {
	baseName := strings.TrimPrefix(tab.Text, "* ")
	if isDirty {
		tab.Text = "* " + baseName
	} else {
		tab.Text = baseName
	}
	a.docTabs.Refresh()
}

func (a *App) showPreferences() {
	gamedataEntry := widget.NewEntry()
	gamedataEntry.SetText(a.config.GamedataPath)

	textAssetsEntry := widget.NewEntry()
	textAssetsEntry.SetText(a.config.TextAssetsPath)

	md3viewEntry := widget.NewEntry()
	md3viewEntry.SetText(a.config.MD3ViewPath)

	themeSelect := widget.NewSelect([]string{"Blue (Jedi)", "Red (Sith)", "Gold (Foundry)", "Green (Console)", "Orange (Rebel)", "Purple (Mace)"}, nil)
	themeSelect.SetSelected(strings.Title(a.config.PrimaryColor))
	if a.config.PrimaryColor == "blue" || a.config.PrimaryColor == "" {
		themeSelect.SetSelected("Blue (Jedi)")
	}
	if a.config.PrimaryColor == "red" {
		themeSelect.SetSelected("Red (Sith)")
	}
	if a.config.PrimaryColor == "gold" {
		themeSelect.SetSelected("Gold (Foundry)")
	}
	if a.config.PrimaryColor == "green" {
		themeSelect.SetSelected("Green (Console)")
	}
	if a.config.PrimaryColor == "orange" {
		themeSelect.SetSelected("Orange (Rebel)")
	}
	if a.config.PrimaryColor == "purple" {
		themeSelect.SetSelected("Purple (Mace)")
	}

	// GitHub Update button for data files
	updateStatusLabel := widget.NewLabel("")

	prefsBrowseGamedata := func() {
		d := dialog.NewFolderOpen(func(uri fyne.ListableURI, err error) {
			if uri != nil {
				gamedataEntry.SetText(uri.Path())
			}
		}, a.mainWindow)
		if parents := CommonGamedataParents(); len(parents) > 0 {
			if lister, err := storage.ListerForURI(storage.NewFileURI(parents[0])); err == nil {
				d.SetLocation(lister)
			}
		}
		d.Show()
	}
	prefsBrowseTextAssets := func() {
		dialog.ShowFolderOpen(func(uri fyne.ListableURI, err error) {
			if uri != nil {
				textAssetsEntry.SetText(uri.Path())
			}
		}, a.mainWindow)
	}

	form := widget.NewForm(
		widget.NewFormItem("Theme Color", themeSelect),
		widget.NewFormItem("Gamedata Path", a.NewPathEntryWithFavorites(gamedataEntry, prefsBrowseGamedata)),
		widget.NewFormItem("TextAssets Path", a.NewPathEntryWithFavorites(textAssetsEntry, prefsBrowseTextAssets)),
		widget.NewFormItem("MD3View Path", container.NewBorder(nil, nil, nil,
			NewTooltipButton("", theme.FolderOpenIcon(), func() {
				dialog.ShowFileOpen(func(uri fyne.URIReadCloser, err error) {
					if uri != nil {
						md3viewEntry.SetText(uri.URI().Path())
					}
				}, a.mainWindow)
			}, "Select the md3view executable for model previews"), md3viewEntry)),

		widget.NewFormItem("", func() fyne.CanvasObject {
			return widget.NewButton("Download MD3View (Required for Previews)", func() {
				// Link to JACoders or a reliable mirror
				if u, err := url.Parse("https://github.com/JACoders/md3view/releases"); err == nil {
					a.fyneApp.OpenURL(u)
				}
			})
		}()),

		widget.NewFormItem("", widget.NewSeparator()),

		// GitHub Config
		widget.NewFormItem("GitHub Token", func() fyne.CanvasObject {
			tokenEntry := widget.NewPasswordEntry()
			tokenEntry.SetText(a.config.GitHubToken)

			helpBtn := widget.NewButton("Get Token", func() {
				// Open browser to token creation page
				// Note: Ideally use Device Flow, but for now simple link
				// "https://github.com/settings/tokens/new?scopes=repo&description=FA%20Creator"
				if u, err := url.Parse("https://github.com/settings/tokens/new?scopes=repo&description=FA%20Creator"); err == nil {
					a.fyneApp.OpenURL(u)
				}
			})

			// On change, update config and manager
			tokenEntry.OnChanged = func(s string) {
				a.config.GitHubToken = s
				// Re-init manager
				if a.config.TextAssetsPath != "" {
					a.githubManager = NewGitHubManager(s, a.config.TextAssetsPath)
				}
			}

			return container.NewBorder(nil, nil, nil, helpBtn, tokenEntry)
		}()),

		widget.NewFormItem("", widget.NewSeparator()),

		widget.NewFormItem("Enum Data", container.NewVBox(
			NewTooltipButton("Update Data from GitHub", nil, func() {
				updateStatusLabel.SetText("Updating...")
				go func() {
					result, err := UpdateDataFromGitHub()
					if err != nil {
						updateStatusLabel.SetText("⚠ " + result)
					} else {
						updateStatusLabel.SetText("✓ " + result)
					}
				}()
			}, "Download latest enum definitions from GitHub"),
			updateStatusLabel,
		)),
	)

	dialog.ShowCustomConfirm("Preferences", "Save", "Cancel", form, func(b bool) {
		if b {
			a.config.GamedataPath = gamedataEntry.Text
			a.config.TextAssetsPath = textAssetsEntry.Text
			a.config.MD3ViewPath = md3viewEntry.Text

			// Save Theme
			switch themeSelect.Selected {
			case "Blue (Jedi)":
				a.config.PrimaryColor = "blue"
			case "Red (Sith)":
				a.config.PrimaryColor = "red"
			case "Gold (Foundry)":
				a.config.PrimaryColor = "gold"
			case "Green (Console)":
				a.config.PrimaryColor = "green"
			case "Orange (Rebel)":
				a.config.PrimaryColor = "orange"
			case "Purple (Mace)":
				a.config.PrimaryColor = "purple"
			}
			a.applyThemeColor(a.config.PrimaryColor)

			a.saveConfig()

			// Refresh components
			if a.assetBrowser != nil {
				a.assetBrowser.SetPaths(a.config.GamedataPath, a.config.TextAssetsPath)
			}
		}
	}, a.mainWindow)
}

func (a *App) saveConfig() {
	data, _ := json.MarshalIndent(a.config, "", "  ")
	os.WriteFile(a.configPath, data, 0644)
}

func (a *App) showAbout() {
	content := widget.NewRichTextFromMarkdown(`
# MBII Foundry v` + AppVersion + `

**Created by Frenzy & Pipex**

The ultimate content creation suite for Movie Battles II.

### Getting Started
1. **Configure:** Go to Settings (Gear Icon) and set your **GameData Path** and **TextAssets Path**.
2. **Browse:** Use the sidebar to explore PK3 contents, models, and icons directly.
3. **Create:** Click the **+** button to start a new Character, Saber, Vehicle, or Siege file.
4. **Edit:** Use the visual selectors and grids to build your content without syntax errors.
5. **Source:** View the generated code in the Source tab to verify or copy-paste.

### Features
*   **Visual Attributes:** Toggle attributes grid instead of typing enums.
*   **Asset Integration:** Browse and preview game assets.
*   **Force & Weapons:** Dedicated editors for complex overrides.
*   **Info Panel:** Hover any field for its enum definition and usage tips.

For support, file an issue at github.com/Frenzeh/mbii-foundry or ask in the MBII Discord.
`)

	scroll := container.NewVScroll(content)
	scroll.SetMinSize(fyne.NewSize(500, 400))

	dialog.ShowCustom("About MBII Foundry", "Close", scroll, a.mainWindow)
}

func (a *App) buildPK3(source, dest string) {}

func (a *App) syncWorkspace() {
	if a.githubManager == nil {
		dialog.ShowInformation("Not Configured", "Please setup your workspace first.", a.mainWindow)
		return
	}

	// Check for dirty state?
	clean, err := a.githubManager.IsClean()
	if err == nil && !clean {
		dialog.ShowConfirm("Unsaved Changes",
			"You have pending changes. Updating assets might cause conflicts or require a reset.\n\nIt is recommended to Submit your changes first.\n\nContinue anyway?",
			func(ok bool) {
				if ok {
					a.doSync()
				}
			}, a.mainWindow)
		return
	}

	a.doSync()
}

func (a *App) doSync() {
	progress := dialog.NewProgressInfinite("Updating...", "Pulling latest assets from official repository...", a.mainWindow)
	progress.Show()

	go func() {
		err := a.githubManager.SyncUpdates()
		progress.Hide()

		if err != nil {
			dialog.ShowError(fmt.Errorf("Update Failed: %v", err), a.mainWindow)
		} else {
			dialog.ShowInformation("Updated", "Assets are now up to date!", a.mainWindow)
			// Refresh Browser
			if a.assetBrowser != nil {
				a.assetBrowser.Refresh()
			}
		}
	}()
}

func (a *App) checkFirstRun() {
	// Defer the check to ensure window is visible first
	if !a.config.SetupWizardSeen {
		// Use lifecycle hook or simple timer to show it after startup
		go func() {
			time.Sleep(500 * time.Millisecond) // Give UI a moment to render
			a.showSetupWizard()
		}()
	}
}

func (a *App) showSetupWizard() {
	// Content
	intro := widget.NewRichTextFromMarkdown(`
# Welcome to MBII Foundry!

To enable the **Asset Browser**, **Visual Editor**, and **Model Previews**, we need to locate your Movie Battles II installation.

1. Select your **GameData** folder.
2. (Optional) Select **TextAssets** if you are a developer.
3. You can configure **MD3View** later in Preferences for 3D previews.
`)

	gamedataEntry := widget.NewEntry()
	gamedataEntry.PlaceHolder = "e.g. C:\\Program Files (x86)\\LucasArts\\Star Wars Jedi Knight Jedi Academy\\GameData"
	gamedataEntry.SetText(a.config.GamedataPath)

	textAssetsEntry := widget.NewEntry()
	textAssetsEntry.PlaceHolder = "Optional — path to your TextAssets Git checkout"
	textAssetsEntry.SetText(a.config.TextAssetsPath)

	// Inline validation indicator — tells the user whether the typed/
	// detected path actually contains base/ and MBII/ subfolders.
	statusLabel := widget.NewLabel("")
	statusLabel.Wrapping = fyne.TextWrapWord
	validateAndShow := func(path string) {
		if path == "" {
			statusLabel.SetText("")
			return
		}
		if err := ValidateGamedataPath(path); err != nil {
			statusLabel.SetText("✗ " + err.Error())
		} else {
			statusLabel.SetText("✓ Looks good — base/ and MBII/ both found.")
		}
	}
	gamedataEntry.OnChanged = validateAndShow
	validateAndShow(gamedataEntry.Text)

	// Auto-Detect Button — now covers LucasArts retail, Steam, GoG,
	// Linux, and macOS Wine/OpenJK installs via gamedata_detect.go.
	autoDetectBtn := widget.NewButton("Auto-Detect Installation", func() {
		if found := DetectGamedataPath(); found != "" {
			gamedataEntry.SetText(found)
			statusLabel.SetText("✓ Found MBII at: " + found)
		} else {
			statusLabel.SetText("✗ Auto-detect didn't find an MBII install. Paste the path above, or use Browse.")
		}
	})

	// Browse button: opens Fyne's folder picker, but starts at the
	// most likely parent directory (e.g. C:\Program Files (x86)\) so
	// the user doesn't land in their home folder and have to drill
	// down from scratch.
	browseGamedata := func() {
		d := dialog.NewFolderOpen(func(uri fyne.ListableURI, err error) {
			if uri != nil {
				gamedataEntry.SetText(uri.Path())
			}
		}, a.mainWindow)
		if parents := CommonGamedataParents(); len(parents) > 0 {
			if lister, err := storage.ListerForURI(storage.NewFileURI(parents[0])); err == nil {
				d.SetLocation(lister)
			}
		}
		d.Show()
	}

	browseTextAssets := func() {
		dialog.ShowFolderOpen(func(uri fyne.ListableURI, err error) {
			if uri != nil {
				textAssetsEntry.SetText(uri.Path())
			}
		}, a.mainWindow)
	}

	form := widget.NewForm(
		widget.NewFormItem("GameData", a.NewPathEntryWithFavorites(gamedataEntry, browseGamedata)),
		widget.NewFormItem("TextAssets", a.NewPathEntryWithFavorites(textAssetsEntry, browseTextAssets)),
	)

	hint := widget.NewLabel("💡 Tip: paste a full path directly, or ★-pin a folder once and pick it from the dropdown next time.")
	hint.Wrapping = fyne.TextWrapWord

	content := container.NewVBox(intro, autoDetectBtn, widget.NewSeparator(), form, statusLabel, hint)

	// Custom Dialog that forces a choice (mostly)
	d := dialog.NewCustomConfirm("Initial Setup", "Save & Continue", "Skip (Limited Features)", content, func(save bool) {
		if save {
			// Validate
			path := gamedataEntry.Text
			if path == "" {
				dialog.ShowError(fmt.Errorf("GameData path cannot be empty."), a.mainWindow)
				// Re-show? Complicated with async. Ideally loop or check before closing.
				// For now, if they click Save with empty, we assume they messed up but save empty (or check).
				// Better: don't close if invalid? Fyne dialogs close on callback.
				// We'll warn them.
			} else {
				// Save config
				a.config.GamedataPath = path
				a.config.TextAssetsPath = textAssetsEntry.Text
				a.config.SetupWizardSeen = true // Mark as seen
				a.saveConfig()

				// Auto-pin successfully-saved paths so they're one-click in
				// future dialogs. Does nothing on re-save of an already-pinned
				// path (move-to-front only).
				a.PinFavorite(path)
				if ta := textAssetsEntry.Text; ta != "" {
					a.PinFavorite(ta)
				}

				// Update components
				if a.assetBrowser != nil {
					a.assetBrowser.SetPaths(a.config.GamedataPath, a.config.TextAssetsPath)
				}
				dialog.ShowInformation("Setup Complete", "Configuration saved! You can change this later in Preferences.", a.mainWindow)
			}
		} else {
			a.config.SetupWizardSeen = true // Mark as seen even if skipped to avoid loop
			a.saveConfig()
			dialog.ShowInformation("Skipped", "Asset features will be limited. You can configure paths later in Preferences.", a.mainWindow)
		}
	}, a.mainWindow)

	// Resize dialog to be readable
	d.Resize(fyne.NewSize(600, 400))
	d.Show()
}

// showFilePickerForEntry opens a file picker for an Entry widget.
func (a *App) showFilePickerForEntry(entry *widget.Entry, title string, filter AssetType) {
	filePickerWindow := a.fyneApp.NewWindow(title)
	filePickerWindow.Resize(fyne.NewSize(900, 600))

	pickerBrowser := NewAssetBrowser(a.config.GamedataPath, a.config.TextAssetsPath)

	// Set initial path based on filter type
	initialPath := ""
	if a.config.GamedataPath != "" {
		switch filter {
		case AssetTypeModel:
			initialPath = filepath.Join(a.config.GamedataPath, "base", "models", "players")
		case AssetTypeIcon:
			initialPath = filepath.Join(a.config.GamedataPath, "base", "gfx", "hud")
		case AssetTypeEffect: // Assuming effects are usually in base/fx or similar
			initialPath = filepath.Join(a.config.GamedataPath, "base", "fx")
		case AssetTypeSound: // Assuming sounds are in base/sound
			initialPath = filepath.Join(a.config.GamedataPath, "base", "sound")
		}
	}

	cfp := NewCustomFilePicker(filePickerWindow, pickerBrowser)
	if initialPath != "" {
		cfp.SetInitialPath(initialPath)
	}

	cfp.Show(func(filePath string) {
		if filePath != "" {
			// Convert absolute path to relative game path if it's within gamedata
			if strings.HasPrefix(filePath, a.config.GamedataPath) {
				relativePath := strings.TrimPrefix(filePath, a.config.GamedataPath+string(os.PathSeparator))
				// Remove 'base/' if it's the first component
				if strings.HasPrefix(relativePath, "base"+string(os.PathSeparator)) {
					relativePath = strings.TrimPrefix(relativePath, "base"+string(os.PathSeparator))
				}

				// Smart parsing based on type
				if filter == AssetTypeModel {
					// models/players/X/model.glm -> X
					if strings.HasSuffix(strings.ToLower(relativePath), "model.glm") {
						entry.SetText(filepath.Base(filepath.Dir(relativePath)))
						return
					}
				} else if filter == AssetTypeSkin {
					// .../model_X.skin -> X
					base := filepath.Base(relativePath)
					lower := strings.ToLower(base)
					if strings.HasPrefix(lower, "model_") && strings.HasSuffix(lower, ".skin") {
						// Extract name between model_ and .skin
						skinName := base[6 : len(base)-5]
						entry.SetText(skinName)
						return
					}
				} else if filter == AssetTypeIcon {
					// gfx/menus/classes/X.tga -> X? No, UI Shader usually expects full path relative to base or just shader name.
					// If it's a shader, use name. If texture, use path without extension?
					// Usually for Class Icon, we want "models/players/X/icon_default" or "gfx/menus/classes/X"
					// Let's leave full path for now, minus extension?
					// entry.SetText(strings.TrimSuffix(relativePath, filepath.Ext(relativePath)))
					// No, MBII often uses full paths or specific references.
				}

				entry.SetText(relativePath)
			} else {
				entry.SetText(filePath)
			}
		}
	})
}
