package main

import (
	"encoding/json"
	"fmt"
	"image/color"
	"net/url"
	"os"
	"path/filepath"
	"runtime/debug"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/canvas"
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
	sourcePanel  *SourcePanel
	activityBar  *SidebarHeader // top-of-sidebar horizontal activity switcher (legacy field name)
	sidebarHost  *fyne.Container // swap target for the active activity's content

	fileManager   *FileManager
	githubManager *GitHubManager

	modpackManager *ModpackManager

	statusLabel    *widget.Label
	split          *container.Split   // Reference to split layout
	sideTabs       *container.AppTabs // Reference to sidebar
	sidebarVisible bool

	// Live refs to the current HSplits so we can read back the user's
	// drag offset before the next updateMainLayout rebuild wipes them.
	// Without this, dragging the divider had no persistent effect: every
	// rebuild called SetOffset with the stale config value.
	sidebarSplit *container.Split
	sourceSplit  *container.Split

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

	// Right-side live source panel. Remembered across sessions.
	SourcePanelVisible bool    `json:"source_panel_visible"`
	SourcePanelOffset  float32 `json:"source_panel_offset"` // 0 = collapsed, 1 = full

	// Left-side activity bar: the currently-selected activity id
	// (files/library/modpacks/workspace). Empty = use the default.
	ActiveActivity string `json:"active_activity"`

	// Hover tooltips for enum values. Default is on; users who find
	// them distracting can flip this off in Preferences.
	HoverTooltipsDisabled bool `json:"hover_tooltips_disabled"`

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
	// Force dark mode on every platform regardless of what the OS
	// reports. macOS respects system light/dark, but on Windows and
	// Linux the Fyne default often reports VariantLight and the app
	// rendered as a washed-out pale UI. The whole design is dark-
	// native; collapsing both variants to the dark path keeps the
	// experience consistent cross-platform.
	variant = theme.VariantDark

	if name == theme.ColorNamePrimary {
		return CurrentThemeColor
	}

	// Surface Shifting for Backgrounds
	if name == theme.ColorNameBackground || name == theme.ColorNameInputBackground || name == theme.ColorNameOverlayBackground {
		var base color.Color

		// Dark-only base colors now (variant was pinned to dark above).
		if name == theme.ColorNameInputBackground {
			base = color.RGBA{R: 15, G: 15, B: 15, A: 255}
		} else {
			base = color.RGBA{R: 28, G: 28, B: 28, A: 255}
		}

		// Tint the background slightly with the primary accent (5%)
		// This creates the "Surface Shift" effect (e.g. reddish dark mode for Sith)
		return blendColors(base, CurrentThemeColor, 0.04)
	}

	// Dark variant surface overrides. Fyne's default dark theme gives
	// us generic grey for buttons, hover, selections, separators —
	// which looks out of place next to our accent-driven chrome. These
	// overrides keep those surfaces on-brand: tinted darks for resting
	// states, alpha-accent for interactive states.
	if variant == theme.VariantDark {
		switch name {
		case theme.ColorNameButton:
			// Default button background: subtle raised surface with a
			// whisper of accent. Used by widget.Button without LowImp.
			return blendColors(color.RGBA{R: 40, G: 40, B: 40, A: 255}, CurrentThemeColor, 0.06)
		case theme.ColorNameDisabled:
			return color.NRGBA{R: 120, G: 120, B: 120, A: 255}
		case theme.ColorNameDisabledButton:
			return color.NRGBA{R: 32, G: 32, B: 32, A: 255}
		case theme.ColorNameHover:
			// Accent tint at low alpha — our standard interactive hint.
			return tintWithAlpha(CurrentThemeColor, 45)
		case theme.ColorNamePressed:
			return tintWithAlpha(CurrentThemeColor, 90)
		case theme.ColorNameFocus:
			return tintWithAlpha(CurrentThemeColor, 110)
		case theme.ColorNameSelection:
			return tintWithAlpha(CurrentThemeColor, 80)
		case theme.ColorNameSeparator:
			// Thin lines between list items / form rows. Slightly
			// warmer than pure grey so they harmonize with the
			// accent-tinted base.
			return color.NRGBA{R: 60, G: 60, B: 60, A: 160}
		case theme.ColorNameInputBorder:
			return color.NRGBA{R: 80, G: 80, B: 80, A: 200}
		case theme.ColorNameScrollBar:
			return tintWithAlpha(CurrentThemeColor, 80)
		case theme.ColorNameShadow:
			return color.NRGBA{R: 0, G: 0, B: 0, A: 140}
		case theme.ColorNameMenuBackground:
			return blendColors(color.RGBA{R: 32, G: 32, B: 32, A: 255}, CurrentThemeColor, 0.05)
		case theme.ColorNameHeaderBackground:
			return blendColors(color.RGBA{R: 22, G: 22, B: 22, A: 255}, CurrentThemeColor, 0.08)
		}

		// Syntax highlight palette used by the source panel's
		// RichText view. Colors are tuned for readability against the
		// 28/28/28 base with accent tint.
		switch name {
		case ColorNameSyntaxComment:
			return color.NRGBA{R: 110, G: 110, B: 110, A: 255} // dim grey
		case ColorNameSyntaxString:
			return color.NRGBA{R: 160, G: 205, B: 130, A: 255} // muted green
		case ColorNameSyntaxNumber:
			return color.NRGBA{R: 125, G: 175, B: 230, A: 255} // soft blue
		case ColorNameSyntaxConst:
			// Enum constants take the theme accent — switching
			// theme (e.g. Sith → Jedi) repaints every MB_CLASS_*,
			// WP_*, FP_* token to match.
			return CurrentThemeColor
		case ColorNameSyntaxHeader:
			return color.NRGBA{R: 235, G: 190, B: 110, A: 255} // amber
		case ColorNameSyntaxPunct:
			return color.NRGBA{R: 150, G: 150, B: 150, A: 255} // dim grey
		}
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
	// Monospace code always gets Hack.
	if style.Monospace {
		if embedFont != nil {
			return fyne.NewStaticResource("font.ttf", embedFont)
		}
		return theme.DefaultTheme().Font(style)
	}
	// Regular UI text gets Jost at an appropriate weight.
	// Bold text picks the Bold face; other text uses Regular. This is
	// more targeted than "everything non-mono = display font" — avoids
	// the over-application of display type to running body content.
	switch {
	case style.Bold && embedJostBold != nil:
		return fyne.NewStaticResource("jost-bold.ttf", embedJostBold)
	case embedJostRegular != nil:
		return fyne.NewStaticResource("jost-regular.ttf", embedJostRegular)
	}
	return theme.DefaultTheme().Font(style)
}

// DisplayFontResource returns a SemiBold display face for use with
// canvas.Text when we want a size-driven hero moment. Nil-safe.
func DisplayFontResource() fyne.Resource {
	if embedJostSemibold != nil {
		return fyne.NewStaticResource("jost-semibold.ttf", embedJostSemibold)
	}
	return nil
}

// MonoFontResource returns the bundled monospace font (Hack). Nil-safe.
func MonoFontResource() fyne.Resource {
	if embedFont != nil {
		return fyne.NewStaticResource("font.ttf", embedFont)
	}
	return nil
}
func (h FoundryTheme) Icon(name fyne.ThemeIconName) fyne.Resource {
	return theme.DefaultTheme().Icon(name)
}
func (h FoundryTheme) Size(name fyne.ThemeSizeName) float32 { return theme.DefaultTheme().Size(name) }

// Panel-push icons wrapped as themed resources so Fyne tints them
// with the current theme's foreground color automatically.
func PanelCollapseLeftIcon() fyne.Resource {
	return theme.NewThemedResource(fyne.NewStaticResource("panel-collapse-left.svg", embedPanelCollapseLeft))
}
func PanelExpandLeftIcon() fyne.Resource {
	return theme.NewThemedResource(fyne.NewStaticResource("panel-expand-left.svg", embedPanelExpandLeft))
}
func PanelCollapseRightIcon() fyne.Resource {
	return theme.NewThemedResource(fyne.NewStaticResource("panel-collapse-right.svg", embedPanelCollapseRight))
}
func PanelExpandRightIcon() fyne.Resource {
	return theme.NewThemedResource(fyne.NewStaticResource("panel-expand-right.svg", embedPanelExpandRight))
}

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

	// Persist whatever the user dragged the split dividers to as the
	// window closes — no other code path captures a last-minute drag.
	application.mainWindow.SetCloseIntercept(func() {
		application.persistSplitOffsets()
		application.mainWindow.Close()
	})

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
	a.sourcePanel = NewSourcePanel(a)

	a.modpackManager = NewModpackManager(a)

	// DocTabs setup
	a.docTabs = container.NewDocTabs()
	a.docTabs.OnClosed = a.closeTab
	a.docTabs.OnSelected = func(tab *container.TabItem) {
		// When the user switches tabs, point the live source panel
		// at the newly-active editor. Home/welcome tabs aren't
		// editors and show a placeholder in the source pane.
		if editor, ok := a.editors[tab]; ok {
			a.sourcePanel.SetActiveEditor(editor)
		} else {
			a.sourcePanel.SetActiveEditor(nil)
		}
	}
	a.docTabs.SetTabLocation(container.TabLocationTop)

	// Starts empty — no "Ready" default. The status bar is there for
	// meaningful messages ("Opened X", "Saved Y") not for a permanent
	// "Ready" word that says nothing. updateMainLayout collapses the
	// bar when the label is empty so the chrome disappears with it.
	a.statusLabel = widget.NewLabel("")
	a.statusLabel.TextStyle = fyne.TextStyle{Italic: true}

	// Dev-mode status icon. Only displayed when MBII_FOUNDRY_DEV is set;
	// updateMainLayout hides the whole label/icon pair for regular users.
	a.holocronStatus = widget.NewIcon(theme.CancelIcon())

	// Double-click-to-open for file-system assets. Active both in the
	// sidebar asset browser (when "Files" activity is selected) and in
	// any future pop-out dialog.
	a.assetBrowser.SetOnAssetDouble(func(asset *AssetEntry) {
		if asset == nil || asset.IsDir || asset.Path == "" || asset.PK3Source != "" {
			return
		}
		a.openFileFromPath(asset.Path)
	})

	// Activity bar: VS-Code-style vertical icon strip on the far left.
	// Each icon owns the left sidebar's content — click to switch. Gives
	// depth-heavy apps a clean way to expose every workflow without
	// cramming them all into simultaneous panels.
	// Activity items for the sidebar's horizontal header. Each pill is
	// icon+label; the active pill is full-opacity, inactive ones are
	// dimmed. The user picks which activity is showing by clicking the
	// pill — the previous vertical icon strip was non-obvious, so the
	// switcher moved into the sidebar's own header where the pill-to-
	// content relationship is direct.
	activities := []*ActivityItem{
		{ID: "files", Label: "Files", Tooltip: "Browse assets and TextAssets",
			Icon: theme.FolderIcon(), Content: a.assetBrowser.GetContent()},
		{ID: "library", Label: "Library", Tooltip: "Enum reference and glossary",
			Icon: theme.ListIcon(), Content: a.infoPanel.GetContent()},
		{ID: "modpacks", Label: "Modpacks", Tooltip: "Bundle changes into pk3s",
			Icon: theme.StorageIcon(), Content: a.modpackManager.GetContent()},
	}

	// Host that holds the currently-active activity's content. Swapped
	// by the sidebar header's onSelect below.
	a.sidebarHost = container.NewStack()

	a.activityBar = NewSidebarHeader(activities,
		func(it *ActivityItem) {
			a.sidebarHost.Objects = []fyne.CanvasObject{it.Content}
			a.sidebarHost.Refresh()
			a.config.ActiveActivity = it.ID
			a.saveConfig()
		},
		func() { // collapse toggle on the right edge of the header
			a.toggleSidebar()
		})

	// Choose initial activity: persisted preference → default to Files.
	initialActivity := a.config.ActiveActivity
	if initialActivity == "" {
		initialActivity = "files"
	}
	a.activityBar.SetActive(initialActivity)

	// sideTabs kept only as a type-compatibility placeholder — the
	// legacy AppTabs reference isn't used in the new layout but other
	// code paths may still hold pointers.
	a.sideTabs = container.NewAppTabs()
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
	// Before tearing down the existing layout, pull whatever the user
	// just dragged the dividers to back into config. Otherwise the next
	// SetOffset below would snap them to the stale saved value and it'd
	// look like dragging does nothing.
	a.persistSplitOffsets()

	// Keep the activity bar's collapse icon in sync with the current
	// sidebar state. The toggle lives on the activity bar now (bottom
	// of the left strip) — the old status-bar toggle is gone.
	if a.activityBar != nil {
		a.activityBar.SetCollapsed(!a.sidebarVisible)
	}

	// StatusBar container. Only built when there's something to show —
	// an empty statusLabel gets collapsed entirely so no "Ready" dead
	// space sits at the bottom of the window. Holocron indicator only
	// surfaces in dev mode.
	var statusBar fyne.CanvasObject
	hasStatus := a.statusLabel.Text != "" || a.holocronClient != nil
	if hasStatus {
		statusBarItems := []fyne.CanvasObject{
			a.statusLabel,
			layout.NewSpacer(),
		}
		if a.holocronClient != nil {
			statusBarItems = append(statusBarItems,
				widget.NewSeparator(),
				widget.NewLabel("Holocron:"),
				a.holocronStatus,
			)
		}
		statusBar = container.NewHBox(statusBarItems...)
	}

	// Compose the center: [sidebar | docTabs+edge-rails | source-panel]
	// Each side pane is either shown fully (split) OR collapsed to a
	// narrow edge rail that peeks on the side of the docTabs area.
	// Activity switching happens in the sidebar's own header.
	a.sidebarSplit = nil
	a.sourceSplit = nil

	// Start with docTabs, then attach edge rails if either panel is
	// collapsed. Rails sit as Border sides so docTabs still fills the
	// remaining area.
	docArea := fyne.CanvasObject(a.docTabs)
	var leftRail, rightRail fyne.CanvasObject
	if !a.sidebarVisible {
		leftRail = collapsedEdgeRail(PanelExpandLeftIcon(), a.toggleSidebar, "Show sidebar")
	}
	if !a.config.SourcePanelVisible {
		rightRail = collapsedEdgeRail(PanelExpandRightIcon(), a.toggleSourcePanel, "Show source panel")
	}
	if leftRail != nil || rightRail != nil {
		docArea = container.NewBorder(nil, nil, leftRail, rightRail, docArea)
	}
	var centerContent fyne.CanvasObject = docArea

	// Sidebar: [SidebarHeader] / [active-activity content]. Only built
	// when visible — when collapsed, the leftRail above offers the
	// expand affordance instead.
	if a.sidebarVisible && a.sidebarHost != nil && a.activityBar != nil {
		sidebarPanel := container.NewBorder(a.activityBar, nil, nil, nil, a.sidebarHost)
		sidebarSplit := container.NewHSplit(sidebarPanel, centerContent)
		sidebarOff := a.config.SidebarOffset
		if sidebarOff <= 0 || sidebarOff >= 1 {
			sidebarOff = 0.25
		}
		sidebarSplit.SetOffset(float64(sidebarOff))
		a.sidebarSplit = sidebarSplit
		centerContent = sidebarSplit
	}

	// Source panel on the right. Collapsed → rightRail above handles
	// the expand affordance.
	if a.config.SourcePanelVisible && a.sourcePanel != nil {
		sourceSplit := container.NewHSplit(centerContent, a.sourcePanel.GetContent())
		offset := a.config.SourcePanelOffset
		if offset <= 0 || offset >= 1 {
			offset = 0.65
		}
		sourceSplit.SetOffset(float64(offset))
		a.sourceSplit = sourceSplit
		centerContent = sourceSplit
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

// showHoverTooltip is called from editors when the user hovers or
// focuses a field with a known key. Pops up a small transient panel
// anchored to the currently-focused widget so the user's eye doesn't
// have to travel across the screen. Disabled entirely if the user
// flipped HoverTooltipsDisabled in Preferences.
func (a *App) showHoverTooltip(key, context string) {
	if key == "" || a.infoPanel == nil {
		return
	}
	if a.config.HoverTooltipsDisabled {
		return
	}
	// Reuse InfoPanel's lookup pipeline — same resolution logic as the
	// full Library, just rendered into a transient popup.
	a.infoPanel.ShowInfo(key, context)
	// Tiny delay lets ShowInfo's markdown settle before we snapshot
	// it into the popup.
	go func() {
		time.Sleep(30 * time.Millisecond)
		fyne.Do(func() {
			a.renderHoverPopup(key, context)
		})
	}()
}

// Track the current hover popup so successive hovers replace it
// instead of stacking.
var currentHoverPopup *widget.PopUp

func (a *App) renderHoverPopup(key, context string) {
	// Dismiss any previous popup so hovers don't stack.
	if currentHoverPopup != nil {
		currentHoverPopup.Hide()
		currentHoverPopup = nil
	}

	title := widget.NewLabelWithStyle(key, fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	if context != "" {
		title.SetText(key + " · " + context)
	}

	// Pull the rendered markdown from the InfoPanel (already resolved
	// by ShowInfo above). We reuse it rather than re-resolving.
	content := widget.NewRichText(a.infoPanel.content.Segments...)
	content.Wrapping = fyne.TextWrapWord
	// Smaller default — less disruptive than the old 380×220.
	scroll := container.NewVScroll(content)
	scroll.SetMinSize(fyne.NewSize(320, 140))

	card := container.NewBorder(title, nil, nil, nil, scroll)
	pop := widget.NewPopUp(card, a.mainWindow.Canvas())
	pop.Resize(fyne.NewSize(340, 180))

	// Anchor near the focused widget when possible — puts the tooltip
	// next to the thing the user is editing instead of the screen
	// corner. Fall back to bottom-right only when no focused widget
	// is findable (Home tab, etc.).
	canvas := a.mainWindow.Canvas()
	pos := a.tooltipAnchor(canvas)
	pop.ShowAtPosition(pos)
	currentHoverPopup = pop
}

// tooltipAnchor returns a good position for a hover popup. Tries the
// currently-focused widget's bottom-right corner first; falls back to
// the canvas bottom-right if there's no focused object.
func (a *App) tooltipAnchor(canvas fyne.Canvas) fyne.Position {
	const (
		popupW = float32(340)
		popupH = float32(180)
	)
	canvasSize := canvas.Size()
	fallback := fyne.NewPos(canvasSize.Width-popupW-20, canvasSize.Height-popupH-20)

	focused := canvas.Focused()
	if focused == nil {
		return fallback
	}
	// Focusable may or may not also be a CanvasObject. Try to extract
	// position via fyne.CanvasObject interface.
	obj, ok := focused.(fyne.CanvasObject)
	if !ok {
		return fallback
	}
	absPos := fyne.CurrentApp().Driver().AbsolutePositionForObject(obj)
	size := obj.Size()

	// Prefer: below and slightly right of the focused widget. If that
	// would push offscreen, fall back to left or above.
	x := absPos.X + size.Width + 10
	y := absPos.Y
	if x+popupW > canvasSize.Width {
		// Not enough room to the right — try below.
		x = absPos.X
		y = absPos.Y + size.Height + 6
	}
	if y+popupH > canvasSize.Height {
		// Try above.
		y = absPos.Y - popupH - 6
	}
	if x < 0 || y < 0 {
		return fallback
	}
	return fyne.NewPos(x, y)
}

// showLibraryModal opens the InfoPanel in a full-size dialog so users
// can browse every known enum without dedicating sidebar space to it.
func (a *App) showLibraryModal() {
	if a.infoPanel == nil {
		return
	}
	content := a.infoPanel.GetContent()
	win := a.fyneApp.NewWindow("Reference Library")
	win.SetContent(content)
	win.Resize(fyne.NewSize(700, 700))
	win.Show()
}

// collapsedEdgeRail returns a CollapsedRail widget — a thin vertical
// strip that stands in for a collapsed panel. Hovering the whole
// strip fills it with the accent tint (so the user can see the
// panel's full extent before committing to restore it); clicking
// anywhere on the strip restores the panel. The arrow icon sits at
// the top as a visual hint of the direction.
func collapsedEdgeRail(icon fyne.Resource, onTap func(), tooltip string) fyne.CanvasObject {
	return NewCollapsedRail(icon, onTap, tooltip)
}

// sectionHeading renders a bold small-caps label + thin accent rule,
// used to group form rows in modal dialogs (Preferences, etc.).
// Replaces the ugly empty-label + separator spacer rows that rendered
// as dark bands on the dark theme.
func sectionHeading(title string) fyne.CanvasObject {
	label := canvas.NewText(title, theme.PlaceHolderColor())
	label.TextSize = SizeSmall
	label.TextStyle = fyne.TextStyle{Bold: true}

	return container.NewVBox(label, NewAccentRule())
}

// persistSplitOffsets snapshots whatever offsets the user dragged the
// split dividers to and writes them into config. Called right before
// any rebuild that would otherwise discard the in-memory splits, and
// on save/close so the offsets survive across sessions. Cheap — just
// field reads and a small float write.
func (a *App) persistSplitOffsets() {
	changed := false
	if a.sidebarSplit != nil {
		off := float32(a.sidebarSplit.Offset)
		if off > 0 && off < 1 && off != a.config.SidebarOffset {
			a.config.SidebarOffset = off
			changed = true
		}
	}
	if a.sourceSplit != nil {
		off := float32(a.sourceSplit.Offset)
		if off > 0 && off < 1 && off != a.config.SourcePanelOffset {
			a.config.SourcePanelOffset = off
			changed = true
		}
	}
	if changed {
		a.saveConfig()
	}
}

func (a *App) toggleSourcePanel() {
	a.config.SourcePanelVisible = !a.config.SourcePanelVisible
	if a.config.SourcePanelOffset <= 0 {
		a.config.SourcePanelOffset = 0.6
	}
	a.saveConfig()
	a.updateMainLayout()
	a.mainWindow.Content().Refresh()
	// Refresh the source panel to sync to the currently-active tab.
	if a.config.SourcePanelVisible && a.sourcePanel != nil {
		if tab := a.docTabs.Selected(); tab != nil {
			if editor, ok := a.editors[tab]; ok {
				a.sourcePanel.SetActiveEditor(editor)
			}
		}
	}
}

func (a *App) createNewFile(title string, editor interface{}) {
	// Check if we should replace the "Start" tab if it's the only one
	if len(a.docTabs.Items) == 1 && a.docTabs.Items[0].Text == "Home" {
		a.docTabs.Remove(a.docTabs.Items[0])
	}

	if ed, ok := editor.(Editor); ok {
		ed.SetAssetBrowser(a.assetBrowser)
		ed.SetOnHover(a.showHoverTooltip)
		ed.SetHolocronClient(a.holocronClient)

		tab := container.NewTabItem("Untitled "+title, ed.GetContent())
		// Register the mapping BEFORE Append AND Select. Append can
		// auto-select the new tab when it's the only one (e.g. right
		// after the Home tab was removed), firing OnSelected before any
		// later bookkeeping runs. If the map isn't populated yet, the
		// source panel sees nil and gets stuck on its "Select a file"
		// placeholder. Registering first makes both auto-select on
		// Append and explicit Select resolve the editor correctly.
		a.editors[tab] = ed
		a.docTabs.Append(tab)
		a.docTabs.Select(tab)
		// Belt-and-suspenders: explicitly point the source panel at the
		// new editor. Fyne's DocTabs.OnSelected firing across
		// remove/append/select is historically inconsistent; doing the
		// wire-up directly is cheap and guarantees the live-source view
		// shows content on the first open.
		if a.sourcePanel != nil {
			a.sourcePanel.SetActiveEditor(ed)
		}

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
	// Catch panics from editor init / LoadFile / updateUI so the app
	// stays running instead of hard-crashing. Rare-path code in the
	// editors (e.g. a new field, a missing enum) can nil-deref —
	// without this, users get silent CTD. With it, they get a dialog
	// with the path + error and can carry on.
	defer func() {
		if r := recover(); r != nil {
			stack := string(debug.Stack())
			LogError("PANIC opening %s: %v\n%s", filePath, r, stack)
			dialog.ShowError(fmt.Errorf(
				"Failed to open %s\n\nInternal error: %v\n\n"+
					"Full stack trace written to the log "+
					"(Debug Logs in toolbar).\nPlease file an issue "+
					"with the log attached.",
				filepath.Base(filePath), r),
				a.mainWindow)
		}
	}()

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
	// Clear the live source panel — whatever was being tracked is
	// about to be torn down (or has been), so leaving the panel
	// pointed at it risks stale refreshes.
	if a.sourcePanel != nil {
		a.sourcePanel.SetActiveEditor(nil)
	}

	// If no tabs remain, the user closed the last editor (or Home).
	// Re-add the welcome screen so the app never ends up on a blank
	// document area with no way back.
	if len(a.docTabs.Items) == 0 {
		welcomeScreen := NewWelcomeScreen(a)
		welcomeTab := container.NewTabItem("Home", welcomeScreen.GetContent())
		welcomeTab.Icon = theme.HomeIcon()
		a.docTabs.Append(welcomeTab)
		a.docTabs.Select(welcomeTab)
		// Force the DocTabs to repaint — Append inside an OnClosed
		// callback can otherwise leave the tab strip stale for a tick
		// and the user sees a blank content area.
		a.docTabs.Refresh()
	}
}

func (a *App) createToolbar() fyne.CanvasObject {
	// Toolbar buttons use ToolbarButton (icon-only, subtle accent
	// hover) instead of widget.Button's grey Material hover which was
	// a visual mismatch with the dark/accent design.
	btn := func(icon fyne.Resource, action func(), tooltip string) *ToolbarButton {
		return NewToolbarButton("", icon, action, tooltip)
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
	)

	// No panel toggles in the toolbar. When a panel is collapsed its
	// expand button shows up as an edge rail on the main area (see
	// collapsedEdgeRail in updateMainLayout) — that's a natural place
	// to click "bring the panel back" and keeps the toolbar focused
	// on file ops + app-level controls.
	items = append(items,
		// Tools & View (Right). The Library moved to the sidebar
		// header so reference docs are one click away without needing
		// a modal.
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

	entry := NewMultiLineInputEntry()
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
			editor.SetOnHover(a.showHoverTooltip)
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

	// Set default sidebar offset if not configured. New activity-bar
	// layout puts the sidebar on the left; 0.25 = quarter for sidebar,
	// three-quarters for the editor. (Old layout used 0.8 with the
	// sidebar on the right; we overwrite stale values from that era.)
	if a.config.SidebarOffset == 0 || a.config.SidebarOffset >= 0.6 {
		a.config.SidebarOffset = 0.25
	}

	// Source panel defaults on for first-launch users. We default to
	// visible iff no saved offset exists (= config was never written
	// with these keys), so existing users who turned it off keep that.
	if a.config.SourcePanelOffset == 0 {
		a.config.SourcePanelVisible = true
		a.config.SourcePanelOffset = 0.65
	}

	// Apply Theme
	if a.config.PrimaryColor == "" {
		a.config.PrimaryColor = "blue"
	}
	a.applyThemeColor(a.config.PrimaryColor)
}

// currentEditorPath returns the file path of the currently-active
// editor, or "" if no editor tab is focused. Used by the source
// panel's Apply flow to restore the original path after reusing
// LoadFile with a temp file.
func (a *App) currentEditorPath() string {
	tab := a.docTabs.Selected()
	if tab == nil {
		return ""
	}
	editor, ok := a.editors[tab]
	if !ok {
		return ""
	}
	return editor.GetCurrentPath()
}

func (a *App) updateStatus(msg string) {
	wasEmpty := a.statusLabel.Text == ""
	a.statusLabel.SetText(fmt.Sprintf("[%s] %s", time.Now().Format("15:04:05"), msg))
	// If the status bar was collapsed (empty text), we need to rebuild
	// the layout so it appears. Only rebuilds on the transition — most
	// updates just mutate the label in place.
	if wasEmpty && a.mainWindow != nil {
		a.updateMainLayout()
	}
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
	gamedataEntry := NewInputEntry()
	gamedataEntry.SetText(a.config.GamedataPath)

	textAssetsEntry := NewInputEntry()
	textAssetsEntry.SetText(a.config.TextAssetsPath)

	md3viewEntry := NewInputEntry()
	md3viewEntry.SetText(a.config.MD3ViewPath)

	themeSelect := widget.NewSelect([]string{"Blue (Jedi)", "Red (Sith)", "Gold (Foundry)", "Green (Console)", "Orange (Rebel)", "Purple (Mace)"}, nil)

	tooltipsCheck := widget.NewCheck("Show info tooltips on hover", func(on bool) {
		a.config.HoverTooltipsDisabled = !on
		a.saveConfig()
	})
	tooltipsCheck.Checked = !a.config.HoverTooltipsDisabled
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

	// Secondary buttons (download, update, get token) — LowImportance
	// so they don't compete with the primary Save action. Previous
	// version used plain widget.NewButton which rendered as Material-
	// style grey boxes, a jarring mismatch with the rest of the flat
	// red-accent design language.
	downloadMD3Btn := widget.NewButton("Download MD3View", func() {
		if u, err := url.Parse("https://github.com/JACoders/md3view/releases"); err == nil {
			a.fyneApp.OpenURL(u)
		}
	})
	downloadMD3Btn.Importance = widget.LowImportance

	tokenEntry := NewPasswordInputEntry()
	tokenEntry.SetText(a.config.GitHubToken)
	tokenEntry.OnChanged = func(s string) {
		a.config.GitHubToken = s
		if a.config.TextAssetsPath != "" {
			a.githubManager = NewGitHubManager(s, a.config.TextAssetsPath)
		}
	}
	getTokenBtn := widget.NewButton("Get Token", func() {
		if u, err := url.Parse("https://github.com/settings/tokens/new?scopes=repo&description=FA%20Creator"); err == nil {
			a.fyneApp.OpenURL(u)
		}
	})
	getTokenBtn.Importance = widget.LowImportance

	updateEnumBtn := NewTooltipButton("Update Data from GitHub", nil, func() {
		updateStatusLabel.SetText("Updating…")
		go func() {
			result, err := UpdateDataFromGitHub()
			if err != nil {
				updateStatusLabel.SetText("⚠ " + result)
			} else {
				updateStatusLabel.SetText("✓ " + result)
			}
		}()
	}, "Download latest enum definitions from GitHub")
	updateEnumBtn.Importance = widget.LowImportance

	// Form rows, grouped with visible section headers instead of empty-
	// label + separator spacer rows. The spacer rows rendered as ugly
	// dark bands across the dialog — an accidental consequence of the
	// dark theme + Fyne's form row padding.
	coreForm := widget.NewForm(
		widget.NewFormItem("Theme Color", themeSelect),
		widget.NewFormItem("Info Tooltips", tooltipsCheck),
	)

	pathsForm := widget.NewForm(
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
	)

	githubForm := widget.NewForm(
		widget.NewFormItem("GitHub Token", container.NewBorder(nil, nil, nil, getTokenBtn, tokenEntry)),
	)

	form := container.NewVBox(
		sectionHeading("GENERAL"),
		coreForm,
		Gap(SpaceMD),

		sectionHeading("PATHS"),
		pathsForm,
		container.NewPadded(downloadMD3Btn),
		Gap(SpaceMD),

		sectionHeading("GITHUB ACCESS"),
		githubForm,
		Gap(SpaceMD),

		sectionHeading("DATA"),
		container.NewPadded(container.NewVBox(updateEnumBtn, updateStatusLabel)),
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

	gamedataEntry := NewInputEntry()
	gamedataEntry.PlaceHolder = "e.g. C:\\Program Files (x86)\\LucasArts\\Star Wars Jedi Knight Jedi Academy\\GameData"
	gamedataEntry.SetText(a.config.GamedataPath)

	textAssetsEntry := NewInputEntry()
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
