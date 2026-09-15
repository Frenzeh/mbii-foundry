package main

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"image/color"
	"io"
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
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/storage"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/Frenzeh/mbii-foundry/parsers"
)

const (
	// AppVersion MUST track the latest shipped release tag (without the
	// leading "v"). The UpdateChecker compares this string against the
	// GitHub Releases API's tag_name to decide whether to show the Home
	// screen's "new version available" banner. Bump this before tagging
	// a release — if they drift, testers get a stale banner or none at
	// all.
	AppVersion = "0.16.1-alpha"
	AppName    = "MBII Foundry"
)

type App struct {
	fyneApp    fyne.App
	mainWindow fyne.Window

	config                  AppConfig
	configPath              string
	legacyTokenPending      string
	configLoadError         error
	configWarning           string
	credentialWarning       string
	legacyCredentialSecured bool
	credentialBusy          bool

	docTabs         *container.DocTabs
	editors         map[*container.TabItem]Editor
	detachedEditors map[Editor]string // torn-out editors remain live until reattached or discarded

	assetBrowser       *AssetBrowser
	infoPanel          *InfoPanel
	infoPanelMirrors   []*InfoPanel // pop-out windows hosting fresh InfoPanel instances
	sourcePanel        *SourcePanel
	sourcePanelMirrors []*SourcePanel  // pop-out windows tracking the same active editor
	activityBar        *SidebarHeader  // top-of-sidebar horizontal activity switcher (legacy field name)
	sidebarHost        *fyne.Container // swap target for the active activity's content

	// Session infrastructure — shared per-document draft store for the
	// source panels, and the crash-recovery snapshot store.
	sourceDrafts *SourceDraftStore
	recovery     *RecoveryStore
	bulkEditor   *BulkEditor

	fileManager    *FileManager
	githubManager  *GitHubManager
	modpackManager *ModpackManager

	statusLabel    *widget.Label
	bufferGauge    *BufferGaugeWidget
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

	// Update Checker — hits GitHub Releases in the background on
	// startup + caches result for 6h in $CONFIG/update_cache.json.
	// WelcomeScreen reads this to decide whether to render the "new
	// version available" banner.
	updateChecker *UpdateChecker
}

var CurrentThemeColor color.Color = color.RGBA{R: 0, G: 128, B: 255, A: 255} // Default Blue

// CurrentColorVariant is the dark/light mode selector the custom
// FoundryTheme respects. Historically the theme forced VariantDark to
// dodge Fyne's cross-platform OS-reporting inconsistency; we still
// ignore the OS signal, but we let the user pick via Preferences.
// Updated from config at startup and whenever the user flips the
// toggle — SetTheme() on the fyne.App refreshes render.
var CurrentColorVariant fyne.ThemeVariant = theme.VariantDark

type AppConfig struct {
	GamedataPath    string       `json:"gamedata_path"`
	TextAssetsPath  string       `json:"text_assets_path"`
	MD3ViewPath     string       `json:"md3view_path"`
	LastOpenDir     string       `json:"last_open_dir"`
	WindowWidth     float32      `json:"window_width"`
	WindowHeight    float32      `json:"window_height"`
	RecentFiles     []RecentFile `json:"recent_files"`
	Theme           string       `json:"theme"`
	PrimaryColor    string       `json:"primary_color"` // Accent color: blue/red/gold/green/orange/purple
	ColorVariant    string       `json:"color_variant"` // "dark" | "light" — foundry defaults to dark if unset
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

	// Developer-fields visibility. When true, the editor surfaces
	// schema fields marked `"dev": true` (engine-internal bitmasks,
	// jetpack offsets/angles, saberDamageStyle, etc.) instead of
	// hiding them. Default false — keeps the loadout-editor surface
	// uncluttered for the common case while letting power users
	// flip everything on from View → Show Developer Fields.
	ShowDeveloperFields bool `json:"show_developer_fields"`

	// Density scales the theme's internal padding so the UI reads
	// tighter or airier per taste. "comfortable" = 1.0× (default),
	// "compact" = 0.8×, "spacious" = 1.25×. Applied via FoundryTheme
	// .Size overrides for padding-related size names.
	Density string `json:"density"`

	// Pinned folder paths — shown as quick-select shortcuts in file pickers
	// and path-entry fields so users don't have to navigate to the same
	// location repeatedly. Most-recently-pinned first. Max 12 entries.
	FavoritePaths []string `json:"favorite_paths"`

	// GitHub Config
	GitHubToken string `json:"-"`
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
	// Ignore whatever variant Fyne passes — on Windows/Linux the OS
	// report often disagrees with the user's intent. We source the
	// variant from CurrentColorVariant (set by config + Preferences
	// toggle) so the experience is consistent cross-platform while
	// still respecting the user's choice.
	variant = CurrentColorVariant

	if name == theme.ColorNamePrimary {
		return CurrentThemeColor
	}

	// Surface Shifting for Backgrounds — base color depends on mode,
	// then gets a subtle accent tint so switching accent colors
	// (blue/red/gold/…) gently recolors the whole chrome.
	if name == theme.ColorNameBackground || name == theme.ColorNameInputBackground || name == theme.ColorNameOverlayBackground {
		var base color.Color
		if variant == theme.VariantLight {
			// Light base: paper white for inputs, warm off-white for
			// the main background. Matches the dark-mode approach of
			// "a touch darker" for inputs than general background,
			// just flipped.
			if name == theme.ColorNameInputBackground {
				base = color.RGBA{R: 252, G: 252, B: 252, A: 255}
			} else {
				base = color.RGBA{R: 240, G: 240, B: 240, A: 255}
			}
			// Lighter tint ratio — 4% on white quickly looks garish,
			// 2% is just enough to carry the accent.
			return blendColors(base, CurrentThemeColor, 0.02)
		}
		// Dark base.
		if name == theme.ColorNameInputBackground {
			base = color.RGBA{R: 15, G: 15, B: 15, A: 255}
		} else {
			base = color.RGBA{R: 28, G: 28, B: 28, A: 255}
		}
		return blendColors(base, CurrentThemeColor, 0.04)
	}

	// Mode-specific surface overrides. Fyne's default palette gives
	// us generic grey for buttons, hover, selections, separators —
	// which looks out of place next to our accent-driven chrome. The
	// overrides keep those surfaces on-brand: tinted neutrals for
	// resting states, alpha-accent for interactive states.
	if variant == theme.VariantLight {
		switch name {
		case theme.ColorNameButton:
			// Soft card surface that reads as raised on the off-white
			// background but doesn't compete with foreground text.
			return blendColors(color.RGBA{R: 230, G: 230, B: 230, A: 255}, CurrentThemeColor, 0.04)
		case theme.ColorNameDisabled:
			return color.NRGBA{R: 150, G: 150, B: 150, A: 255}
		case theme.ColorNameDisabledButton:
			return color.NRGBA{R: 220, G: 220, B: 220, A: 255}
		case theme.ColorNameForeground:
			// Near-black for readable body text. Default dark text can
			// be too soft against warm off-white.
			return color.NRGBA{R: 30, G: 30, B: 30, A: 255}
		case theme.ColorNamePlaceHolder:
			return color.NRGBA{R: 120, G: 120, B: 120, A: 255}
		case theme.ColorNameHover:
			return tintWithAlpha(CurrentThemeColor, 40)
		case theme.ColorNamePressed:
			return tintWithAlpha(CurrentThemeColor, 80)
		case theme.ColorNameFocus:
			return tintWithAlpha(CurrentThemeColor, 100)
		case theme.ColorNameSelection:
			return tintWithAlpha(CurrentThemeColor, 70)
		case theme.ColorNameSeparator:
			return color.NRGBA{R: 200, G: 200, B: 200, A: 200}
		case theme.ColorNameInputBorder:
			return color.NRGBA{R: 170, G: 170, B: 170, A: 220}
		case theme.ColorNameScrollBar:
			return tintWithAlpha(CurrentThemeColor, 70)
		case theme.ColorNameShadow:
			// Strong enough to separate a modal from the workspace,
			// still softer than dark mode against the light canvas.
			return color.NRGBA{R: 0, G: 0, B: 0, A: 70}
		case theme.ColorNameMenuBackground:
			return blendColors(color.RGBA{R: 246, G: 246, B: 246, A: 255}, CurrentThemeColor, 0.02)
		case theme.ColorNameHeaderBackground:
			return blendColors(color.RGBA{R: 235, G: 235, B: 235, A: 255}, CurrentThemeColor, 0.04)
		}
		// Syntax palette recalibrated for readability on light
		// backgrounds — darker saturations so tokens don't wash out.
		switch name {
		case ColorNameSyntaxComment:
			return color.NRGBA{R: 130, G: 130, B: 130, A: 255}
		case ColorNameSyntaxString:
			return color.NRGBA{R: 60, G: 135, B: 55, A: 255}
		case ColorNameSyntaxNumber:
			return color.NRGBA{R: 40, G: 90, B: 180, A: 255}
		case ColorNameSyntaxConst:
			return CurrentThemeColor
		case ColorNameSyntaxHeader:
			return color.NRGBA{R: 160, G: 100, B: 30, A: 255}
		case ColorNameSyntaxPunct:
			return color.NRGBA{R: 110, G: 110, B: 110, A: 255}
		}
		return theme.DefaultTheme().Color(name, variant)
	}

	// Dark variant.
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
		// Modal dialogs need a clear task boundary over the dense
		// three-pane workspace. This remains below the old heavy 55%
		// treatment, while providing enough dimming to quiet toolbars
		// and pane outlines behind a dialog.
		return color.NRGBA{R: 0, G: 0, B: 0, A: 105}
	case theme.ColorNameMenuBackground:
		return blendColors(color.RGBA{R: 32, G: 32, B: 32, A: 255}, CurrentThemeColor, 0.05)
	case theme.ColorNameHeaderBackground:
		return blendColors(color.RGBA{R: 22, G: 22, B: 22, A: 255}, CurrentThemeColor, 0.08)
	}

	// Syntax highlight palette used by the source panel's RichText
	// view. Colors are tuned for readability against the 28/28/28
	// base with accent tint.
	switch name {
	case ColorNameSyntaxComment:
		return color.NRGBA{R: 110, G: 110, B: 110, A: 255} // dim grey
	case ColorNameSyntaxString:
		return color.NRGBA{R: 160, G: 205, B: 130, A: 255} // muted green
	case ColorNameSyntaxNumber:
		return color.NRGBA{R: 125, G: 175, B: 230, A: 255} // soft blue
	case ColorNameSyntaxConst:
		// Enum constants take the theme accent — switching theme
		// (e.g. Sith → Jedi) repaints every MB_CLASS_*, WP_*, FP_*
		// token to match.
		return CurrentThemeColor
	case ColorNameSyntaxHeader:
		return color.NRGBA{R: 235, G: 190, B: 110, A: 255} // amber
	case ColorNameSyntaxPunct:
		return color.NRGBA{R: 150, G: 150, B: 150, A: 255} // dim grey
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

// densityScale returns the multiplier the user's density preference
// applies to padding/inner-padding theme sizes. Text size isn't
// touched — only spacing — so the app breathes wider without
// becoming grade-school chunky.
var CurrentDensityScale float32 = 1.0

func (h FoundryTheme) Size(name fyne.ThemeSizeName) float32 {
	base := theme.DefaultTheme().Size(name)
	switch name {
	case theme.SizeNamePadding, theme.SizeNameInnerPadding, theme.SizeNameInlineIcon:
		return base * CurrentDensityScale
	}
	return base
}

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
	if a.fyneApp != nil {
		a.fyneApp.Settings().SetTheme(&FoundryTheme{}) // Refresh theme
	}
}

// applyDensity wires AppConfig.Density through to the theme-size
// scale the FoundryTheme.Size overrides read. Called at startup
// AND whenever the user flips the Preferences selector. Unknown
// values fall back to "comfortable" rather than error — the config
// may have been migrated from an older layout.
func (a *App) applyDensity(density string) {
	switch density {
	case "compact":
		CurrentDensityScale = 0.8
	case "spacious":
		CurrentDensityScale = 1.25
	default:
		CurrentDensityScale = 1.0
	}
	a.config.Density = density
	if a.fyneApp != nil {
		a.fyneApp.Settings().SetTheme(a.fyneApp.Settings().Theme())
	}
}

// applyColorVariant wires AppConfig.ColorVariant through to the
// package-global the FoundryTheme reads. Call after loading config
// and whenever the Preferences toggle flips. Empty or unknown values
// default to dark — matches the historical behavior so existing
// configs keep rendering as they always did.
func (a *App) applyColorVariant(variant string) {
	switch strings.ToLower(strings.TrimSpace(variant)) {
	case "light":
		CurrentColorVariant = theme.VariantLight
	default:
		CurrentColorVariant = theme.VariantDark
	}
	if a.fyneApp != nil {
		a.fyneApp.Settings().SetTheme(&FoundryTheme{})
	}
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

	application := &App{
		editors:        make(map[*container.TabItem]Editor),
		holocronClient: NewHolocronClient(),
	}

	application.fyneApp = app.NewWithID("com.frenzeh.mbii-foundry")
	application.fyneApp.Settings().SetTheme(&FoundryTheme{})

	application.loadConfig()
	configDirectory := ""
	if application.configPath != "" {
		configDirectory = filepath.Dir(application.configPath)
	}
	application.fileManager = NewFileManager(configDirectory)
	application.updateChecker = NewUpdateChecker(configDirectory)
	// Crash recovery lives in an isolated directory under the config
	// dir — never next to user documents, prefs or backups. An unavailable
	// config directory disables persistence rather than using ./recovery.
	recoveryDirectory := ""
	if configDirectory != "" {
		recoveryDirectory = filepath.Join(configDirectory, "recovery")
	}
	application.recovery = NewRecoveryStore(recoveryDirectory)
	application.sourceDrafts = NewSourceDraftStore()

	// Kick off the version check in the background. CheckAsync uses the
	// 6h cache first and only hits the network when the cache is stale,
	// so relaunches within the same session are free. When the result
	// lands, nudge the Home/welcome tab so the banner picks it up — if
	// the user is already mid-edit on another tab, nothing visibly
	// changes until they navigate back to Home.
	application.updateChecker.CheckAsync(func(info *UpdateInfo) {
		if info == nil || !info.IsNewer {
			return
		}
		fyne.Do(application.refreshWelcomeBanner)
	})

	// Dev-only: start background check for local Holocron server.
	// Guarded at creation via NewHolocronClient; nil for regular users so
	// this goroutine noops. See holocron_client.go for context.
	if application.holocronClient != nil {
		go application.monitorHolocronStatus()
	}

	application.mainWindow = application.fyneApp.NewWindow(fmt.Sprintf("%s - MBII Content Editor", AppName))
	// Restore last-session window size if persisted; else default
	// 1400x900. Sub-1200x600 is treated as corrupt config (Fyne
	// rounding glitches at very small initial sizes) and ignored.
	w, h := application.config.WindowWidth, application.config.WindowHeight
	if w < 1200 || h < 600 {
		w, h = 1400, 900
	}
	application.mainWindow.Resize(fyne.NewSize(w, h))

	application.setupUI()
	application.setupShortcuts()

	// Offer to restore documents left unsaved by a previous crash or
	// forced quit, then start the periodic autosave for dirty docs.
	application.offerSessionRecovery()
	application.startRecoveryAutosave()

	// Check for first run / missing configuration
	application.checkFirstRun()

	// Persist split-dragged offsets + final window size on close, and
	// guard against silent data loss: if any open editor is dirty,
	// prompt before exiting. Without this guard the user could close
	// the app via Cmd+Q and lose every dirty tab without warning;
	// per-tab close prompts didn't fire on app exit.
	application.mainWindow.SetCloseIntercept(func() {
		application.persistSplitOffsets()
		application.persistWindowSize()
		// Snapshot every dirty document first so even a confirmed quit
		// is recoverable on next launch (crash recovery, not discard).
		application.autosaveRecoveryPass()
		dirtyTabs := []string{}
		for tab, ed := range application.editors {
			if ed != nil && (ed.IsDirty() || application.sourceDrafts.Has(ed)) {
				dirtyTabs = append(dirtyTabs, tab.Text)
			}
		}
		for ed, title := range application.detachedEditors {
			if ed != nil && (ed.IsDirty() || application.sourceDrafts.Has(ed)) {
				dirtyTabs = append(dirtyTabs, title)
			}
		}
		if len(dirtyTabs) > 0 {
			msg := fmt.Sprintf("These tabs have unsaved changes:\n\n  • %s\n\nQuit anyway?\n\n"+
				"(A recovery snapshot of unsaved work will be kept and offered on next launch.)",
				strings.Join(dirtyTabs, "\n  • "))
			dialog.ShowConfirm("Unsaved Changes", msg, func(confirmed bool) {
				if confirmed {
					application.mainWindow.Close()
				}
			}, application.mainWindow)
			return
		}
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

	a.assetBrowser = NewAssetBrowser(a.config.GamedataPath, a.config.TextAssetsPath, func() {
		for _, ed := range a.editors {
			if mbchEd, ok := ed.(*MBCHEditor); ok {
				mbchEd.SetAssetBrowser(a.assetBrowser)
			}
		}
	})
	// Shared draft store must exist before the panels are built. Preserve
	// an existing store when setupUI is rebuilt in tests or app lifecycle.
	if a.sourceDrafts == nil {
		a.sourceDrafts = NewSourceDraftStore()
	}
	a.infoPanel = NewInfoPanel()
	a.infoPanel.SetHolocronClient(a.holocronClient)
	a.infoPanel.SetOnPopOut(a.popOutInfoPanel)
	if a.sourcePanel != nil {
		a.sourcePanel.Close()
	}
	a.sourcePanel = NewSourcePanel(a)
	a.sourcePanel.SetOnPopOut(a.popOutSourcePanel)

	a.modpackManager = NewModpackManager(a)
	// Bulk editor (B): dialogs route through app.mainWindow and
	// pre-edit backups through app.fileManager.
	a.bulkEditor = NewBulkEditor(a)

	// DocTabs setup
	a.docTabs = container.NewDocTabs()
	a.docTabs.OnClosed = a.closeTab
	a.docTabs.OnSelected = func(tab *container.TabItem) {
		// When the user switches tabs, point the live source panel
		// at the newly-active editor. Home/welcome tabs aren't
		// editors and show a placeholder in the source pane.
		if editor, ok := a.editors[tab]; ok {
			a.setSourceEditorForAll(editor)
		} else {
			a.setSourceEditorForAll(nil)
		}
	}
	a.docTabs.SetTabLocation(container.TabLocationTop)

	// Starts empty — no "Ready" default. The status bar is there for
	// meaningful messages ("Opened X", "Saved Y") not for a permanent
	// "Ready" word that says nothing. updateMainLayout collapses the
	// bar when the label is empty so the chrome disappears with it.
	a.statusLabel = widget.NewLabel("")
	a.statusLabel.TextStyle = fyne.TextStyle{Italic: true}
	a.bufferGauge = NewBufferGaugeWidget()

	// Dev-mode status icon. Only displayed when MBII_FOUNDRY_DEV is set;
	// updateMainLayout hides the whole label/icon pair for regular users.
	a.holocronStatus = widget.NewIcon(theme.CancelIcon())

	// Double-click-to-open for assets in the sidebar browser. Both
	// filesystem and PK3-embedded entries are supported — the latter
	// route through openFileFromAsset which extracts the entry to a
	// temp file and clears currentPath so the user's next Save prompts
	// Save-As rather than trying to write back into the archive.
	a.assetBrowser.SetOnAssetDouble(func(asset *AssetEntry) {
		a.openFileFromAsset(asset)
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
		{ID: "bulkedit", Label: "Bulk Edit", Tooltip: "Batch-edit fields across many files",
			Icon: theme.ContentPasteIcon(), Content: a.bulkEditor.GetContent()},
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
	hasStatus := a.statusLabel.Text != "" || a.holocronClient != nil || a.bufferGauge != nil
	if hasStatus {
		statusBarItems := []fyne.CanvasObject{
			a.statusLabel,
			layout.NewSpacer(),
		}
		if a.bufferGauge != nil {
			statusBarItems = append(statusBarItems, a.bufferGauge.GetContent())
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
			sidebarOff = 0.30
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
			offset = 0.70
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

// toggleDeveloperFields flips ShowDeveloperFields and notifies every
// open MBCH editor so it can show/hide dev-flagged UI sections.
// Persists across sessions via the standard saveConfig.
//
// The fan-out walks `editors` rather than holding a single active-
// editor reference because multiple MBCH files can be open
// simultaneously in tabs; flipping the toggle should update them all.
func (a *App) toggleDeveloperFields() {
	a.config.ShowDeveloperFields = !a.config.ShowDeveloperFields
	a.saveConfig()
	for _, ed := range a.editors {
		if me, ok := ed.(*MBCHEditor); ok {
			me.applyDeveloperVisibility(a.config.ShowDeveloperFields)
		}
	}
}

// showStickyContext is called when the user INTERACTS with a field —
// clicks/focuses an entry, picks a class card, selects a weapon, etc.
// The info panel saves this key as its "sticky" view: transient
// hovers can overlay it, but mouse-out reverts here. Think of it as
// the panel's home base — "what am I currently editing?"
func (a *App) showStickyContext(key, context string) {
	if key == "" || a.infoPanel == nil {
		return
	}
	if a.config.HoverTooltipsDisabled {
		return
	}
	a.infoPanel.ShowSticky(key, context)
	for _, m := range a.infoPanelMirrors {
		m.ShowSticky(key, context)
	}
}

// showHoverContext is called on mouseover of hoverable targets (grid
// rows, pick cards, etc.). Renders the target's info without
// mutating sticky state — a subsequent clearHoverContext reverts.
//
// Previously this was showHoverTooltip, which ALSO spawned a small
// popup next to the focused widget. The popup duplicated the info
// panel's content and felt redundant once the panel itself became
// the primary context surface. Popup removed.
func (a *App) showHoverContext(key, context string) {
	if key == "" || a.infoPanel == nil {
		return
	}
	if a.config.HoverTooltipsDisabled {
		return
	}
	a.infoPanel.ShowHover(key, context)
	for _, m := range a.infoPanelMirrors {
		m.ShowHover(key, context)
	}
}

// clearHoverContext reverts the panel to its sticky state after a
// hover target's MouseOut. Cheap no-op when the panel isn't
// currently showing a hover — InfoPanel.ClearHover guards on its
// own showingHover flag.
func (a *App) clearHoverContext() {
	if a.infoPanel == nil {
		return
	}
	a.infoPanel.ClearHover()
	for _, m := range a.infoPanelMirrors {
		m.ClearHover()
	}
}

func (a *App) discardDetachedEditor(ed Editor) {
	a.sourceDrafts.Delete(ed)
	a.removeEditorRecovery(ed)
}

func (a *App) confirmDetachedEditorClose(ed Editor, win fyne.Window, closeWindow func()) {
	hasDraft := a.sourceDrafts.Has(ed)
	if ed.IsDirty() || hasDraft {
		message := "This file has unsaved changes. Close window and discard?"
		if hasDraft && !ed.IsDirty() {
			message = "This file has unapplied source edits. Close window and discard?"
		}
		dialog.ShowConfirm("Unsaved Changes", message, func(confirmed bool) {
			if confirmed {
				closeWindow()
			}
		}, win)
		return
	}
	closeWindow()
}

// popOutCurrentTab tears the active editor tab out of docTabs and rehosts it
// in its own window. Torn-out editors remain part of the live session until
// the user explicitly reattaches or confirms discard.
func (a *App) popOutCurrentTab() {
	if a.fyneApp == nil || a.docTabs == nil {
		return
	}
	tab := a.docTabs.Selected()
	if tab == nil {
		return
	}
	ed, ok := a.editors[tab]
	if !ok {
		return
	}

	title := tab.Text
	a.docTabs.Remove(tab)
	delete(a.editors, tab)
	if a.detachedEditors == nil {
		a.detachedEditors = make(map[Editor]string)
	}
	a.detachedEditors[ed] = title

	win := a.fyneApp.NewWindow(title + " — MBII Foundry")
	reattachRequested := false
	discardRequested := false
	closeWithoutIntercept := func() {
		win.SetCloseIntercept(nil)
		win.Close()
	}
	bar := reattachBar("Reattach tab", func() {
		reattachRequested = true
		closeWithoutIntercept()
	})
	win.SetContent(container.NewBorder(bar, nil, nil, nil, ed.GetContent()))
	win.Resize(fyne.NewSize(1100, 800))

	reattach := func() {
		newTab := container.NewTabItem(title, ed.GetContent())
		a.editors[newTab] = ed
		a.docTabs.Append(newTab)
		a.docTabs.Select(newTab)
	}

	win.SetCloseIntercept(func() {
		a.confirmDetachedEditorClose(ed, win, func() {
			discardRequested = true
			closeWithoutIntercept()
		})
	})
	win.SetOnClosed(func() {
		delete(a.detachedEditors, ed)
		if reattachRequested {
			reattach()
			return
		}
		if discardRequested {
			a.discardDetachedEditor(ed)
		}
	})
	win.Show()
}

// setSourceEditorForAll points the primary source panel and every
// mirror at the same Editor so a pop-out window mirrors the main
// source view as the user switches files.
func (a *App) setSourceEditorForAll(ed Editor) {
	if a.sourcePanel != nil {
		a.sourcePanel.SetActiveEditor(ed)
	}
	for _, m := range a.sourcePanelMirrors {
		m.SetActiveEditor(ed)
	}
}

// reattachBar builds the standard top-of-window strip every pop-out
// uses for the "send back to main window" affordance. A single
// LowImportance button labelled "Reattach" runs the supplied callback
// then closes its own window. Keeps reattach UX consistent across
// info-panel / source-panel / tab-tear-off pop-outs. Tooltip clarifies
// the closing-window behaviour so users know their work isn't lost.
func reattachBar(label string, action func()) fyne.CanvasObject {
	btn := NewTooltipButton(label, theme.NavigateBackIcon(), action,
		"Close this window and return the panel to the main app (state preserved)")
	btn.Importance = widget.LowImportance
	return container.NewPadded(container.NewHBox(btn, layout.NewSpacer()))
}

// popOutSourcePanel opens the source panel in a fresh window. New
// instance of SourcePanel registered as a mirror so the broadcast
// pipeline keeps it in sync with whatever editor is active in the
// main window. The window header carries a Reattach button that
// removes the mirror + closes the window in one click; on plain
// close (without Reattach) the mirror is also unregistered.
func (a *App) popOutSourcePanel() {
	if a.fyneApp == nil {
		return
	}
	win := a.fyneApp.NewWindow("Source — MBII Foundry")
	mirror := NewSourcePanel(a)
	mirror.SetOnPopOut(nil) // suppress chained pop-outs
	a.sourcePanelMirrors = append(a.sourcePanelMirrors, mirror)
	if a.sourcePanel != nil && a.sourcePanel.editorRef != nil {
		mirror.SetActiveEditor(a.sourcePanel.editorRef)
	}
	bar := reattachBar("Reattach to main window", func() { win.Close() })
	win.SetContent(container.NewBorder(bar, nil, nil, nil, mirror.GetContent()))
	win.Resize(fyne.NewSize(560, 720))

	// Collapse the docked source rail in the main window — the panel
	// is now living in its own window, so leaving the rail expanded
	// just shows an empty stub. Snapshot the prior visible state so
	// reattach restores it.
	wasVisible := a.config.SourcePanelVisible
	if wasVisible {
		a.config.SourcePanelVisible = false
		a.updateMainLayout()
		a.mainWindow.Content().Refresh()
	}

	win.SetOnClosed(func() {
		mirror.Close()
		out := a.sourcePanelMirrors[:0]
		for _, m := range a.sourcePanelMirrors {
			if m != mirror {
				out = append(out, m)
			}
		}
		a.sourcePanelMirrors = out
		// Restore the rail iff it was visible at pop-out time. Don't
		// re-show it if the user had it hidden before — that would be
		// a state-change surprise.
		if wasVisible && !a.config.SourcePanelVisible {
			a.config.SourcePanelVisible = true
			a.updateMainLayout()
			a.mainWindow.Content().Refresh()
		}
	})
	win.Show()
}

// popOutInfoPanel opens the info panel in a fresh window — the dual-
// monitor workflow. Each pop-out is a fully independent InfoPanel
// instance registered as a "mirror" on the App; show/hover/clear
// pipelines broadcast updates to every mirror so the new window
// stays in sync with what the user is doing in the main window. On
// close, the mirror is unregistered so we don't leak event delivery
// to a dead window.
func (a *App) popOutInfoPanel() {
	if a.fyneApp == nil {
		return
	}
	win := a.fyneApp.NewWindow("Info — MBII Foundry")
	mirror := NewInfoPanel()
	mirror.SetHolocronClient(a.holocronClient)
	// New mirrors don't need their own pop-out button; suppress the
	// callback so the icon does nothing rather than spawning a chain.
	mirror.SetOnPopOut(nil)
	a.infoPanelMirrors = append(a.infoPanelMirrors, mirror)

	// Seed the mirror with whatever the primary panel is currently
	// showing — without this the new window opens on the welcome
	// copy and the user has to mouse around to populate it.
	if a.infoPanel.stickyKey != "" {
		mirror.ShowSticky(a.infoPanel.stickyKey, a.infoPanel.stickyContext)
	}

	bar := reattachBar("Reattach to main window", func() { win.Close() })
	win.SetContent(container.NewBorder(bar, nil, nil, nil, mirror.GetContent()))
	win.Resize(fyne.NewSize(420, 720))

	// Collapse the docked sidebar — the info panel lives there and is
	// now in its own window. Snapshot prior state for restore.
	wasVisible := a.sidebarVisible
	if wasVisible {
		a.sidebarVisible = false
		a.config.SidebarVisible = false
		a.updateMainLayout()
		a.mainWindow.Content().Refresh()
	}

	win.SetOnClosed(func() {
		out := a.infoPanelMirrors[:0]
		for _, m := range a.infoPanelMirrors {
			if m != mirror {
				out = append(out, m)
			}
		}
		a.infoPanelMirrors = out
		// Restore the sidebar iff it was visible at pop-out time.
		if wasVisible && !a.sidebarVisible {
			a.sidebarVisible = true
			a.config.SidebarVisible = true
			a.updateMainLayout()
			a.mainWindow.Content().Refresh()
		}
	})
	win.Show()
}

// showHoverTooltip is kept as a thin shim for existing editors that
// invoke the old name via SetOnHover(a.showHoverTooltip). Treating
// every such call as a hover-style update (not sticky) matches the
// historical behavior — these sites used to produce the transient
// popup, not a panel commit. Sticky updates happen at explicit
// interaction sites that have been migrated to showStickyContext.
func (a *App) showHoverTooltip(key, context string) {
	a.showHoverContext(key, context)
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

// setupShortcuts wires keyboard shortcuts + a macOS-style main menu.
// Until this landed the README and welcome screen advertised Cmd+N /
// Cmd+O / Cmd+S etc. but none were actually bound — typing them did
// nothing. This brings the app to parity with the docs and gives Mac
// users the system-bar menu they expect.
func (a *App) setupShortcuts() {
	if a.mainWindow == nil {
		return
	}
	c := a.mainWindow.Canvas()
	add := func(key fyne.KeyName, mods fyne.KeyModifier, fn func()) {
		c.AddShortcut(&desktop.CustomShortcut{KeyName: key, Modifier: mods}, func(_ fyne.Shortcut) { fn() })
	}
	mod := fyne.KeyModifierShortcutDefault // Cmd on macOS, Ctrl elsewhere

	// File ops.
	add(fyne.KeyN, mod, func() { a.createNewFile("Character", NewMBCHEditor(a)) })
	add(fyne.KeyO, mod, func() { a.openFile() })
	add(fyne.KeyS, mod, func() { a.saveFile() })
	add(fyne.KeyS, mod|fyne.KeyModifierShift, func() { a.saveFileAs() })
	add(fyne.KeyW, mod, func() {
		if t := a.docTabs.Selected(); t != nil {
			a.closeTab(t)
		}
	})

	// Validate.
	add(fyne.KeyR, mod, func() { a.validateFile() })

	// Preferences (Cmd+,).
	add(fyne.KeyComma, mod, func() { a.showPreferences() })

	// Tab navigation — Cmd+1..9.
	for i := 1; i <= 9; i++ {
		idx := i
		var key fyne.KeyName
		switch i {
		case 1:
			key = fyne.Key1
		case 2:
			key = fyne.Key2
		case 3:
			key = fyne.Key3
		case 4:
			key = fyne.Key4
		case 5:
			key = fyne.Key5
		case 6:
			key = fyne.Key6
		case 7:
			key = fyne.Key7
		case 8:
			key = fyne.Key8
		case 9:
			key = fyne.Key9
		}
		add(key, mod, func() {
			if a.docTabs == nil || idx > len(a.docTabs.Items) {
				return
			}
			a.docTabs.Select(a.docTabs.Items[idx-1])
		})
	}

	// Undo / redo across form edits, source applies and JSON imports.
	add(fyne.KeyZ, mod, func() { a.undoActiveEditor() })
	add(fyne.KeyZ, mod|fyne.KeyModifierShift, func() { a.redoActiveEditor() })
	add(fyne.KeyY, mod, func() { a.redoActiveEditor() })

	// Validate.
	add(fyne.KeyR, mod, func() { a.validateFile() })

	// macOS main menu — gives Cmd-anything a system-bar entry so the
	// shortcuts above also show up where Mac users look for them.
	fileMenu := fyne.NewMenu("File",
		fyne.NewMenuItem("New Character", func() { a.createNewFile("Character", NewMBCHEditor(a)) }),
		fyne.NewMenuItem("Open File…", func() { a.openFile() }),
		fyne.NewMenuItemSeparator(),
		fyne.NewMenuItem("Save", func() { a.saveFile() }),
		fyne.NewMenuItem("Save As…", func() { a.saveFileAs() }),
		fyne.NewMenuItemSeparator(),
		fyne.NewMenuItem("Validate", func() { a.validateFile() }),
		fyne.NewMenuItem("Validate Folder…", func() { a.validateFolder() }),
		fyne.NewMenuItemSeparator(),
		fyne.NewMenuItem("Close Tab", func() {
			if t := a.docTabs.Selected(); t != nil {
				a.closeTab(t)
			}
		}),
	)
	editMenu := fyne.NewMenu("Edit",
		fyne.NewMenuItem("Undo", func() { a.undoActiveEditor() }),
		fyne.NewMenuItem("Redo", func() { a.redoActiveEditor() }),
		fyne.NewMenuItemSeparator(),
		fyne.NewMenuItem("Preferences…", func() { a.showPreferences() }),
	)
	devFieldsItem := fyne.NewMenuItem("Show Developer Fields", func() {
		a.toggleDeveloperFields()
	})
	devFieldsItem.Checked = a.config.ShowDeveloperFields
	viewMenu := fyne.NewMenu("View",
		fyne.NewMenuItemSeparator(),
		devFieldsItem,
		fyne.NewMenuItemSeparator(),
		fyne.NewMenuItem("Pop Out Current Tab", func() { a.popOutCurrentTab() }),
		fyne.NewMenuItem("Pop Out Info Panel", func() { a.popOutInfoPanel() }),
		fyne.NewMenuItem("Pop Out Source Panel", func() { a.popOutSourcePanel() }),
	)
	toolsMenu := fyne.NewMenu("Tools",
		fyne.NewMenuItem("Team Composer (.mbtc)…", func() { OpenMBTCComposer(a) }),
		fyne.NewMenuItem("Compare Characters (.mbch)…", func() {
			if tab := a.docTabs.Selected(); tab != nil {
				if ed, ok := a.editors[tab]; ok {
					if mbchEd, ok := ed.(*MBCHEditor); ok {
						ShowCharacterDiffDialog(a.mainWindow, mbchEd.character, "Current Character")
					}
				}
			}
		}),
		fyne.NewMenuItemSeparator(),
		fyne.NewMenuItem("Test in MBII (Live Hot-Test)", func() {
			if tab := a.docTabs.Selected(); tab != nil {
				if ed, ok := a.editors[tab]; ok {
					if mbchEd, ok := ed.(*MBCHEditor); ok {
						LaunchInGameTest(a.mainWindow, mbchEd.character, a.config.GamedataPath)
					}
				}
			}
		}),
		fyne.NewMenuItemSeparator(),
		fyne.NewMenuItem("Validate Character", func() { a.validateFile() }),
		fyne.NewMenuItem("Validate Folder…", func() { a.validateFolder() }),
	)
	helpMenu := fyne.NewMenu("Help",
		fyne.NewMenuItem("About MBII Foundry", func() { a.showAbout() }),
		fyne.NewMenuItem("Debug Logs", func() { a.showLogs() }),
		fyne.NewMenuItem("Icon Inventory…", func() { a.showIconInventory() }),
		fyne.NewMenuItem("Check for Updates", func() { a.checkForUpdatesNow() }),
	)
	a.mainWindow.SetMainMenu(fyne.NewMainMenu(fileMenu, editMenu, viewMenu, toolsMenu, helpMenu))
}

// persistWindowSize captures the main window's final dimensions and
// writes them to config so the next launch reopens at the same size.
// Called from the close intercept; AppConfig.WindowWidth/Height were
// declared but never actually written before this.
func (a *App) persistWindowSize() {
	if a.mainWindow == nil {
		return
	}
	sz := a.mainWindow.Canvas().Size()
	if sz.Width >= 600 && sz.Height >= 400 {
		a.config.WindowWidth = sz.Width
		a.config.WindowHeight = sz.Height
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
				a.setSourceEditorForAll(editor)
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
		t1 := time.Now()
		ed.SetAssetBrowser(a.assetBrowser)
		LogInfo("createNewFile[%s]: SetAssetBrowser took %s", title, time.Since(t1))
		t2 := time.Now()
		ed.SetOnHover(a.showHoverTooltip)
		ed.SetHolocronClient(a.holocronClient)
		LogInfo("createNewFile[%s]: SetOnHover+Holocron took %s", title, time.Since(t2))

		t3 := time.Now()
		tab := container.NewTabItem("Untitled "+title, ed.GetContent())
		LogInfo("createNewFile[%s]: GetContent+TabItem took %s", title, time.Since(t3))
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
			a.setSourceEditorForAll(ed)
		}

		// Dirty handler — Editor interface already requires
		// SetOnDirtyChanged, so the four-way type-switch this
		// previously did was redundant.
		ed.SetOnDirtyChanged(func(isDirty bool) {
			a.updateTabTitle(tab, isDirty)
		})
	}
}

func (a *App) openFileFromPath(filePath string) *container.TabItem {
	// Earlier versions wrapped this body in fyne.Do to "defer onto
	// the next event loop tick" hoping to avoid the spinny wheel.
	// On Fyne v2.7.1 / macOS, fyne.Do called from the main thread
	// blocks waiting for the dispatch queue to drain — but the queue
	// can't drain because main IS waiting. Classic deadlock; sample
	// dump showed every thread parked in __psynch_cvwait. The wheel
	// is preferable to a hang. Run synchronously like the original.
	a.updateStatus("Opening " + filepath.Base(filePath) + "…")
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

	tStart := time.Now()
	tCtor := tStart
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
		return nil
	}
	LogInfo("openFileFromPath[%s]: ctor took %s", title, time.Since(tCtor))

	if editor != nil {
		tLoad := time.Now()
		err := editor.LoadFile(filePath)
		LogInfo("openFileFromPath[%s]: LoadFile took %s", title, time.Since(tLoad))
		if err != nil {
			ShowError(fmt.Errorf("Failed to load file: %v", err), a.mainWindow)
			return nil
		}
		// Add to recent files centrally
		a.fileManager.AddRecentFile(filePath)

		// Reuse logic
		tCreate := time.Now()
		a.createNewFile(title, editor)
		tab := a.docTabs.Selected()
		tab.Text = title
		a.docTabs.Refresh()
		LogInfo("openFileFromPath[%s]: createNewFile+tab took %s", title, time.Since(tCreate))

		a.updateStatus(fmt.Sprintf("Opened %s", title))
		LogInfo("openFileFromPath[%s]: TOTAL %s", title, time.Since(tStart))
		return tab
	}
	return nil
}

// refreshWelcomeBanner rebuilds the Home tab so the update banner picks
// up the latest UpdateChecker result. Called from the CheckAsync
// completion callback; no-op if the Home tab isn't currently in the
// tab strip (the user has it closed and opened some editors).
//
// We rebuild the whole welcome tab rather than mutating the existing
// one because the banner is baked into WelcomeScreen.GetContent at
// construction — cheaper in code to rebuild than to plumb a live
// reference back through every render path.
func (a *App) refreshWelcomeBanner() {
	if a.docTabs == nil {
		return
	}
	for _, tab := range a.docTabs.Items {
		if tab.Text != "Home" {
			continue
		}
		ws := NewWelcomeScreen(a)
		tab.Content = ws.GetContent()
		a.docTabs.Refresh()
		return
	}
}

// openFileFromAsset opens a file represented by an AssetEntry. Handles
// both filesystem-backed entries (PK3Source empty) and PK3-embedded
// entries (PK3Source is the on-disk .pk3, Path is the logical path
// inside that archive). For PK3-backed files the entry bytes are
// extracted to a temp file which the editor then reads through its
// normal LoadFile path; currentPath is cleared afterwards so the user's
// next Save forces Save-As — writing back into a .pk3 in place isn't
// something Foundry supports and shouldn't silently overwrite.
//
// VFS-backed entries (PK3Source == "VFS") are paths that live in the
// merged virtual file system; the editors' LoadFile already falls back
// to VFS when os.Open fails, so we route those straight through the
// normal path-based open.
func (a *App) openFileFromAsset(asset *AssetEntry) {
	if asset == nil || asset.IsDir || asset.Path == "" {
		return
	}

	if asset.PK3Source == "" || asset.PK3Source == "VFS" {
		a.openFileFromPath(asset.Path)
		return
	}

	reader, err := zip.OpenReader(asset.PK3Source)
	if err != nil {
		ShowError(fmt.Errorf("couldn't open %s: %w", filepath.Base(asset.PK3Source), err), a.mainWindow)
		return
	}
	defer reader.Close()

	wanted := strings.ReplaceAll(asset.Path, "\\", "/")
	var zf *zip.File
	for _, f := range reader.File {
		if strings.ReplaceAll(f.Name, "\\", "/") == wanted {
			zf = f
			break
		}
	}
	if zf == nil {
		ShowError(fmt.Errorf("%s not found inside %s", asset.Path, filepath.Base(asset.PK3Source)), a.mainWindow)
		return
	}

	rc, err := zf.Open()
	if err != nil {
		ShowError(fmt.Errorf("couldn't read %s: %w", asset.Path, err), a.mainWindow)
		return
	}
	data, err := io.ReadAll(rc)
	rc.Close()
	if err != nil {
		ShowError(fmt.Errorf("couldn't read %s: %w", asset.Path, err), a.mainWindow)
		return
	}

	tmp, err := os.CreateTemp("", "foundry-pk3-*"+filepath.Ext(asset.Name))
	if err != nil {
		ShowError(fmt.Errorf("couldn't create temp file: %w", err), a.mainWindow)
		return
	}
	tmpPath := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmpPath)
		ShowError(fmt.Errorf("couldn't write temp file: %w", err), a.mainWindow)
		return
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpPath)
		ShowError(fmt.Errorf("couldn't finish temporary document: %w", err), a.mainWindow)
		return
	}
	defer os.Remove(tmpPath)

	tab := a.openFileFromPath(tmpPath)
	if tab == nil {
		return
	}
	ed, ok := a.editors[tab]
	if !ok {
		return
	}
	// Clear the synthetic temp path so the user's next Save falls through
	// to Save-As and lands somewhere they actually expect. Rename the tab
	// to the PK3-source-qualified form so the read-only origin is clear.
	ed.SetCurrentPath("")
	tab.Text = asset.Name + "  ·  " + filepath.Base(asset.PK3Source)
	a.docTabs.Refresh()
	a.updateStatus(fmt.Sprintf("Opened %s (from %s — Save As to persist)",
		asset.Name, filepath.Base(asset.PK3Source)))
}

func (a *App) closeTab(tab *container.TabItem) {
	editor, ok := a.editors[tab]
	if !ok {
		a.removeTab(tab)
		return
	}
	// Dirty includes invalid unapplied source drafts: half-typed
	// source that doesn't parse is still user work. Never implicitly
	// discard — prompt, and keep the draft if the user cancels.
	hasDraft := a.sourceDrafts.Has(editor)
	if editor.IsDirty() || hasDraft {
		msg := "This file has unsaved changes. Close anyway?"
		if hasDraft && !editor.IsDirty() {
			msg = "This file has unapplied edits in the source panel. Close and discard them?"
		}
		dialog.ShowConfirm("Unsaved Changes",
			msg,
			func(confirmed bool) {
				if confirmed {
					a.removeTab(tab)
				}
			}, a.mainWindow)
		return
	}
	a.removeTab(tab)
}

func (a *App) removeTab(tab *container.TabItem) {
	editor, wasEditor := a.editors[tab]
	delete(a.editors, tab)
	if wasEditor && editor != nil {
		// The user explicitly closed this document — drop its retained
		// draft and crash-recovery snapshot with it.
		a.sourceDrafts.Delete(editor)
		a.removeEditorRecovery(editor)
		if se, ok := editor.(SessionEditor); ok {
			se.Session().RecoveryID = ""
		}
	}
	// Clear the live source panel — whatever was being tracked is
	// about to be torn down (or has been), so leaving the panel
	// pointed at it risks stale refreshes.
	if a.sourcePanel != nil {
		a.setSourceEditorForAll(nil)
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

		// Pop out current editor tab into its own window — completes
		// the dual-monitor workflow alongside info-panel + source-panel
		// pop-outs. Dirty-state is preserved; closing the popped-out
		// window reattaches the tab rather than destroying unsaved work.
		btn(theme.WindowMaximizeIcon(), func() { a.popOutCurrentTab() }, "Pop out tab"),
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
		btn(theme.ViewRefreshIcon(), func() { a.checkForUpdatesNow() }, "Check for updates"),
		btn(theme.HelpIcon(), func() { a.showAbout() }, "About MBII Foundry"),
	)

	return container.NewHBox(items...)
}

// undoActiveEditor / redoActiveEditor route Cmd+Z / Cmd+Shift+Z (and
// the Edit menu) to the selected document's session history. History
// spans form edits, in-memory source applies, JSON imports and
// definition switches; undoing past a save correctly re-marks the
// document dirty against its saved baseline.
func (a *App) undoActiveEditor() {
	if se, ok := a.activeSessionEditor(); ok {
		if !se.Undo() {
			a.updateStatus("Nothing to undo")
		}
	}
}

func (a *App) redoActiveEditor() {
	if se, ok := a.activeSessionEditor(); ok {
		if !se.Redo() {
			a.updateStatus("Nothing to redo")
		}
	}
}

func (a *App) activeSessionEditor() (SessionEditor, bool) {
	if a.docTabs == nil {
		return nil, false
	}
	tab := a.docTabs.Selected()
	if tab == nil {
		return nil, false
	}
	ed, ok := a.editors[tab]
	if !ok {
		return nil, false
	}
	se, ok := ed.(SessionEditor)
	return se, ok
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

	// Editor.Validate() is on the interface; the four-way type switch
	// was redundant. MBCH alone reports a character count for the
	// 8192-byte cap warning — keep that as a narrow type assertion.
	issues := editor.Validate()
	var charCount int
	if mbch, ok := editor.(*MBCHEditor); ok {
		charCount = mbch.GetCharacterCount()
		// R22.0.00 raised the cap from 8192 to 16384. Engine rejects len >= 16384.
		if charCount >= 16384 {
			issues = append([]string{fmt.Sprintf("CRITICAL: File exceeds 16384 character limit (%d chars)", charCount)}, issues...)
		} else if charCount > 15000 {
			issues = append([]string{fmt.Sprintf("Warning: Approaching 16384 character limit (%d/16384)", charCount)}, issues...)
		}
	}

	if len(issues) == 0 {
		msg := "✓ No issues found!"
		if charCount > 0 {
			msg += fmt.Sprintf("\n\nCharacter count: %d/16384", charCount)
		}
		dialog.ShowInformation("Validation Passed", msg, a.mainWindow)
	} else {
		msg := fmt.Sprintf("Found %d issue(s):\n\n• %s", len(issues), strings.Join(issues, "\n• "))
		if charCount > 0 {
			msg += fmt.Sprintf("\n\nCharacter count: %d/16384", charCount)
		}
		dialog.ShowInformation("Validation Results", msg, a.mainWindow)
	}
}

// validateFolder runs the MBCH parser against every .mbch file under a
// directory the user picks and reports per-file results. Useful for
// catching schema drift after an MBII patch: point it at the patched
// TextAssets character/ folder and any newly-introduced field that
// the parser can't handle (or any file whose required fields went
// missing) shows up immediately.
//
// Hidden / dotfile-leading entries are skipped. Walks recursively so
// the user can pick the parent of subfoldered class dirs (h*, e*, etc.).
func (a *App) validateFolder() {
	dialog.ShowFolderOpen(func(uri fyne.ListableURI, err error) {
		if err != nil || uri == nil {
			return
		}
		root := uri.Path()
		var (
			scanned int
			ok      int
			fails   []string
		)
		walkErr := filepath.Walk(root, func(path string, info os.FileInfo, walkErr error) error {
			if walkErr != nil {
				return nil // keep walking; report at end
			}
			if info.IsDir() {
				if strings.HasPrefix(info.Name(), ".") && path != root {
					return filepath.SkipDir
				}
				return nil
			}
			if !strings.EqualFold(filepath.Ext(info.Name()), ".mbch") {
				return nil
			}
			scanned++
			data, readErr := os.ReadFile(path)
			if readErr != nil {
				fails = append(fails, fmt.Sprintf("%s — read error: %v", info.Name(), readErr))
				return nil
			}
			char, parseErr := parsers.ParseMBCH(string(data))
			if parseErr != nil {
				fails = append(fails, fmt.Sprintf("%s — parse error: %v", info.Name(), parseErr))
				return nil
			}
			// Light validation that mirrors the engine's hard
			// requirements (BG_SiegeParseClassFile). Don't run the
			// editor's full Validate() — that's UI-state-dependent.
			if strings.TrimSpace(char.Name) == "" {
				fails = append(fails, fmt.Sprintf("%s — missing required `name`", info.Name()))
				return nil
			}
			ok++
			return nil
		})

		summary := fmt.Sprintf("Scanned %d .mbch file(s) under %s\n  ✓ %d ok\n  ✗ %d failed",
			scanned, root, ok, len(fails))
		if walkErr != nil {
			summary += fmt.Sprintf("\n\nWalk warning: %v", walkErr)
		}
		if len(fails) > 0 {
			summary += "\n\nFailures:\n  • " + strings.Join(fails, "\n  • ")
		}
		dialog.ShowInformation("Validate Folder Results", summary, a.mainWindow)
	}, a.mainWindow)
}

func (a *App) diagnosticRoots() []DiagnosticRoot {
	roots := []DiagnosticRoot{
		{Path: a.config.GamedataPath, Label: "<GameData>"},
		{Path: a.config.TextAssetsPath, Label: "<TextAssets>"},
		{Path: a.config.MD3ViewPath, Label: "<MD3View>"},
	}
	if document := a.currentEditorPath(); document != "" {
		roots = append(roots, DiagnosticRoot{Path: filepath.Dir(document), Label: "<DocumentRoot>"})
	}
	for i, project := range a.config.KnownModpacks {
		if project != nil && project.Path != "" {
			roots = append(roots, DiagnosticRoot{
				Path:  project.Path,
				Label: fmt.Sprintf("<ProjectRoot%d>", i+1),
			})
		}
	}
	return roots
}

func (a *App) showLogs() {
	// Use platform-appropriate temp directory (works on Windows, macOS, Linux).
	logPath := filepath.Join(os.TempDir(), "mbii-foundry.log")
	content, err := os.ReadFile(logPath)
	text := ""
	if err != nil {
		text = "Could not read log file: " + err.Error()
	} else {
		text = string(content)
	}
	// Sanitize the fully assembled diagnostic payload at its final UI
	// publication boundary. Raw log/config text is never placed in a
	// copyable widget first.
	text = SanitizeDiagnosticText(text,
		[]string{a.config.GitHubToken, a.legacyTokenPending},
		a.diagnosticRoots())

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
	filePickerWindow.Resize(fyne.NewSize(1200, 780))

	pickerBrowser := NewAssetBrowser(a.config.GamedataPath, a.config.TextAssetsPath, nil)
	cfp := NewCustomFilePicker(filePickerWindow, pickerBrowser)

	// Route every picker selection through openFileFromAsset. That
	// covers filesystem entries (normal path open) *and* PK3-embedded
	// entries (extract to temp, load, clear currentPath) with the
	// same editor/source-panel wiring createNewFile does — previously
	// this flow registered the editors map after Append and never
	// called SetActiveEditor, which left the Source panel stuck on
	// its "Select a file…" placeholder after a successful open.
	cfp.Show(func(asset *AssetEntry) {
		a.openFileFromAsset(asset)
	})
}

// isTempPath reports whether p sits under the OS temp directory.
// Used by saveFile to refuse writing back to a path that originated
// from applyEdits' parser round-trip (the .txt-in-temp bug).
func isTempPath(p string) bool {
	if p == "" {
		return false
	}
	abs, err := filepath.Abs(p)
	if err != nil {
		return false
	}
	return strings.HasPrefix(strings.ToLower(abs), strings.ToLower(os.TempDir()))
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

	// Editor.GetCurrentPath() is already on the interface; the four
	// per-type checks were redundant.
	path := editor.GetCurrentPath()

	// Treat OS temp dir as "no path" — applyEdits used to leak temp
	// file paths into the editor's currentPath, which made Save write
	// back to AppData\Local\Temp. The applyEdits leak is fixed, but
	// guard here too: if the editor is somehow pointed at a temp file,
	// route through Save As so the user gets a real destination
	// instead of clobbering an ephemeral file.
	if path == "" || isTempPath(path) {
		a.saveFileAs()
		return
	}

	// Review then write: the dialog shows the exact candidate bytes
	// against the on-disk original; cancelling leaves baseline, path
	// and draft state untouched.
	a.saveWithReview(editor, tab, path)
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

	defaultPath := editor.GetCurrentPath()
	if defaultPath == "" {
		defaultPath = "Untitled" + expectedExt
	}
	a.showSaveAsReviewDialog(editor, tab, defaultPath, expectedExt)
}

func (a *App) loadConfig() {
	appConfigDir, directoryErr := AppConfigDir()
	if directoryErr != nil {
		a.configWarning = "Configuration migration could not finish: " + directoryErr.Error()
	}
	if appConfigDir == "" {
		a.configLoadError = fmt.Errorf("configuration directory unavailable: %v", directoryErr)
		a.configWarning = "Preferences, favorites and crash recovery cannot be persisted. Local file editing remains available."
		a.config.SidebarVisible = true
		return
	}
	a.configPath = filepath.Join(appConfigDir, "config.json")

	// Set defaults
	a.config.SidebarVisible = true // Default to true

	data, err := os.ReadFile(a.configPath)
	if err == nil {
		var legacy struct {
			GitHubToken string `json:"github_token"`
		}
		if err := json.Unmarshal(data, &a.config); err != nil {
			a.configLoadError = fmt.Errorf("configuration is invalid; original preserved: %w", err)
		} else if err := json.Unmarshal(data, &legacy); err != nil {
			a.configLoadError = err
		} else {
			a.legacyTokenPending = legacy.GitHubToken
		}
	} else if !os.IsNotExist(err) {
		a.configLoadError = err
	}
	if a.configLoadError != nil {
		a.configWarning = "The saved configuration could not be read and will not be overwritten. Repair or restore it before saving Preferences."
	}
	if a.legacyTokenPending != "" {
		a.credentialWarning = "A legacy credential is preserved in the original configuration. Use Connect GitHub to migrate it securely; local editing needs no account."
	}

	// Balance the nested three-pane layout: the sidebar receives 30%
	// of the left/editor region and the source panel takes the final
	// 30% of the window. This keeps the editor dominant while leaving
	// both utility panes readable.
	if a.config.SidebarOffset == 0 || a.config.SidebarOffset >= 0.6 {
		a.config.SidebarOffset = 0.30
	}

	// Source panel defaults on for first-launch users. We default to
	// visible iff no saved offset exists (= config was never written
	// with these keys), so existing users who turned it off keep that.
	if a.config.SourcePanelOffset == 0 {
		a.config.SourcePanelVisible = true
		a.config.SourcePanelOffset = 0.70
	}

	// Apply Theme
	if a.config.PrimaryColor == "" {
		a.config.PrimaryColor = "blue"
	}
	a.applyColorVariant(a.config.ColorVariant) // before applyThemeColor so a single SetTheme covers both
	a.applyThemeColor(a.config.PrimaryColor)
	a.applyDensity(a.config.Density) // theme-size scale; "" falls back to comfortable inside applyDensity
}

// currentEditorPath returns the file path of the currently-active
// editor, or "" if no editor tab is focused. Used by the source
// panel's Apply flow to restore the original path after reusing
// LoadFile with a temp file.
func (a *App) currentEditorPath() string {
	if a == nil || a.docTabs == nil {
		return ""
	}
	tab := a.docTabs.Selected()
	if tab == nil {
		return ""
	}
	editor, ok := a.editors[tab]
	if !ok || editor == nil {
		return ""
	}
	return editor.GetCurrentPath()
}

func (a *App) updateStatus(msg string) {
	if a == nil || a.statusLabel == nil {
		return
	}
	wasEmpty := a.statusLabel.Text == ""
	a.statusLabel.SetText(fmt.Sprintf("[%s] %s", time.Now().Format("15:04:05"), msg))
	// If the status bar was collapsed (empty text), rebuild once so it
	// appears. Most updates only mutate the label.
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

	tooltipsCheck := widget.NewCheck("Show info tooltips on hover", nil)
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

	// Dark/light mode selector. We ignore the OS-reported variant (too
	// inconsistent cross-platform) and honor whatever the user picks
	// here. "Dark" is the historical default; "Light" flipped on adds
	// the light-variant palette branches in FoundryTheme.Color.
	modeSelect := widget.NewSelect([]string{"Dark", "Light"}, nil)
	switch strings.ToLower(a.config.ColorVariant) {
	case "light":
		modeSelect.SetSelected("Light")
	default:
		modeSelect.SetSelected("Dark")
	}

	// Density picker — scales theme padding. "Comfortable" is the
	// default and tracks classic Fyne sizing; "Compact" tightens
	// everything for a busier IDE feel; "Spacious" loosens it so long
	// reading sessions in the info panel don't feel cramped.
	densitySelect := widget.NewSelect([]string{"Compact", "Comfortable", "Spacious"}, nil)
	switch strings.ToLower(a.config.Density) {
	case "compact":
		densitySelect.SetSelected("Compact")
	case "spacious":
		densitySelect.SetSelected("Spacious")
	default:
		densitySelect.SetSelected("Comfortable")
	}

	// Form rows, grouped with visible section headers instead of empty-
	// label + separator spacer rows. The spacer rows rendered as ugly
	// dark bands across the dialog — an accidental consequence of the
	// dark theme + Fyne's form row padding.
	coreForm := widget.NewForm(
		widget.NewFormItem("Color Mode", modeSelect),
		widget.NewFormItem("Theme Color", themeSelect),
		widget.NewFormItem("Density", densitySelect),
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
						uri.Close()
					}
				}, a.mainWindow)
			}, "Select the md3view executable for model previews"), md3viewEntry)),
	)

	githubForm := widget.NewForm(
		widget.NewFormItem("Native credential store", widget.NewButton("Connect / manage GitHub access", func() { a.showCredentialConnect(nil) })),
	)

	configNotice := widget.NewLabel(strings.TrimSpace(a.configWarning + "\n" + a.credentialWarning))
	configNotice.Wrapping = fyne.TextWrapWord
	if configNotice.Text == "" {
		configNotice.Hide()
	}
	form := container.NewVBox(
		configNotice,
		sectionHeading("GENERAL"),
		coreForm,
		Gap(SpaceMD),

		sectionHeading("PATHS"),
		pathsForm,
		container.NewPadded(downloadMD3Btn),
		Gap(SpaceMD),

		sectionHeading("OPTIONAL CONTRIBUTION ACCESS"),
		widget.NewLabel("Local editing needs no account. Tokens are stored in the OS credential store."),
		githubForm,
		Gap(SpaceMD),

		sectionHeading("DATA"),
		container.NewPadded(container.NewVBox(updateEnumBtn, updateStatusLabel)),
	)

	prefsError := widget.NewLabel("")
	prefsError.Wrapping = fyne.TextWrapWord
	var prefsDlg *dialog.CustomDialog
	savePreferences := func() {
		if gamedataEntry.Text != "" {
			if err := ValidateGamedataPath(gamedataEntry.Text); err != nil {
				prefsError.SetText(err.Error())
				return
			}
		}
		previous := a.config
		a.config.GamedataPath = gamedataEntry.Text
		a.config.TextAssetsPath = textAssetsEntry.Text
		a.config.MD3ViewPath = md3viewEntry.Text
		a.config.HoverTooltipsDisabled = !tooltipsCheck.Checked
		a.config.Density = strings.ToLower(densitySelect.Selected)

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
		// Color mode — applied before applyThemeColor so the
		// single SetTheme triggered by that call picks up both.
		switch strings.ToLower(modeSelect.Selected) {
		case "light":
			a.config.ColorVariant = "light"
		default:
			a.config.ColorVariant = "dark"
		}
		if err := a.persistConfig(); err != nil {
			a.config = previous
			prefsError.SetText("Preferences were not saved: " + err.Error())
			return
		}
		a.applyColorVariant(a.config.ColorVariant)
		a.applyThemeColor(a.config.PrimaryColor)
		a.applyDensity(a.config.Density)
		a.githubManager = NewGitHubManager(a.config.GitHubToken, a.config.TextAssetsPath)

		// Refresh components
		if a.assetBrowser != nil {
			a.assetBrowser.SetPaths(a.config.GamedataPath, a.config.TextAssetsPath)
		}
		prefsDlg.Hide()
	}
	saveButton := widget.NewButton("Save", savePreferences)
	saveButton.Importance = widget.HighImportance
	prefsDlg = dialog.NewCustom("Preferences", "Cancel", container.NewBorder(nil,
		container.NewVBox(prefsError, saveButton), nil, nil, container.NewVScroll(form)), a.mainWindow)
	// Resize before showing — default width crams long path fields
	// into a sliver you can't read. 720x560 gives every form row a
	// comfortable full-width input and leaves headroom for the theme
	// picker + data-update sub-forms to breathe.
	prefsDlg.Resize(fyne.NewSize(720, 560))
	prefsDlg.Show()
}

func (a *App) saveConfig() bool {
	if err := a.persistConfig(); err != nil {
		LogError("Could not save preferences; previous configuration preserved")
		if a.mainWindow != nil {
			dialog.ShowError(fmt.Errorf("preferences were not saved: %w", err), a.mainWindow)
		}
		return false
	}
	return true
}

// checkForUpdatesNow is the toolbar action. Forces a fresh GitHub
// check (bypasses the 6h cache), tells the user what it found, and
// — if a newer release exists — rebuilds the Home tab so the
// footer callout becomes visible without them having to relaunch.
//
// Progress feedback is lightweight: a modal progress dialog while
// the HTTP request is in flight (typically <1s), replaced by a
// result dialog. Using the dialog package rather than an inline
// status pill keeps the action discoverable even when the user
// isn't already looking at Home.
func (a *App) checkForUpdatesNow() {
	if a.updateChecker == nil {
		dialog.ShowInformation("Updates",
			"The update checker isn't initialized — this usually means the "+
				"build is running in a stripped-down dev mode.",
			a.mainWindow)
		return
	}

	progress := dialog.NewCustomWithoutButtons("Checking for updates",
		container.NewPadded(container.NewVBox(
			widget.NewLabel("Contacting GitHub…"),
			widget.NewProgressBarInfinite(),
		)), a.mainWindow)
	progress.Resize(fyne.NewSize(360, 120))
	progress.Show()

	a.updateChecker.ForceCheckAsync(func(info *UpdateInfo) {
		fyne.Do(func() {
			progress.Hide()

			if info == nil {
				dialog.ShowError(
					fmt.Errorf("couldn't reach GitHub. Check your internet "+
						"connection and try again."),
					a.mainWindow)
				return
			}
			if !info.IsNewer {
				dialog.ShowInformation("You're up to date",
					fmt.Sprintf("Foundry v%s is the latest release.", AppVersion),
					a.mainWindow)
				return
			}
			// New version — rebuild Home so the footer callout shows it
			// immediately, then confirm with a dialog that points the
			// user at Home if they're on a different tab.
			a.refreshWelcomeBanner()
			dialog.ShowInformation("Update available",
				fmt.Sprintf("Foundry %s is out. Head to the Home tab "+
					"to install it.", info.TagName),
				a.mainWindow)
		})
	})
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

// buildPK3 publishes only a validated manifest through the transactional writer.
func (a *App) buildPK3(source, dest string) error {
	manifest, err := BuildExportManifest(source, dest)
	if err != nil {
		return err
	}
	if err := WritePK3(source, dest, manifest); err != nil {
		return err
	}
	a.updateStatus(fmt.Sprintf("Built %s (%d files)", filepath.Base(dest), len(manifest)))
	return nil
}

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
	notice := strings.TrimSpace(a.configWarning + "\n" + a.credentialWarning)
	if a.config.SetupWizardSeen && notice == "" {
		return
	}
	go func() {
		time.Sleep(500 * time.Millisecond)
		fyne.Do(func() {
			if a.config.SetupWizardSeen {
				// Onboarding already completed — a warning alone is
				// informational.
				dialog.ShowInformation("Local editing remains available", notice, a.mainWindow)
				return
			}
			// First run: onboarding must never be suppressed by the
			// presence of a warning. The notice is embedded inline in
			// the wizard so both surfaces appear together.
			a.showSetupWizardWithWarning(notice)
		})
	}()
}

func (a *App) showSetupWizard() {
	a.showSetupWizardWithWarning("")
}

type setupPathValidation struct {
	GameDataPath   string
	TextAssetsPath string
	GameDataErr    error
	TextAssetsErr  error
	CanUse         bool
}

// validateSetupPaths evaluates both optional setup fields independently.
// At least one path must be supplied to use the primary action, and every
// supplied path must validate. Empty fields are not errors.
func validateSetupPaths(gamedata, textAssets string) setupPathValidation {
	state := setupPathValidation{
		GameDataPath:   strings.TrimSpace(gamedata),
		TextAssetsPath: strings.TrimSpace(textAssets),
	}
	if state.GameDataPath != "" {
		state.GameDataErr = ValidateGamedataPath(state.GameDataPath)
	}
	if state.TextAssetsPath != "" {
		state.TextAssetsErr = ValidateTextAssetsPath(state.TextAssetsPath)
	}
	hasPath := state.GameDataPath != "" || state.TextAssetsPath != ""
	state.CanUse = hasPath && state.GameDataErr == nil && state.TextAssetsErr == nil
	return state
}

func (s setupPathValidation) status() (fyne.Resource, string) {
	var problems []string
	if s.GameDataErr != nil {
		problems = append(problems, "GameData: "+s.GameDataErr.Error())
	}
	if s.TextAssetsErr != nil {
		problems = append(problems, "TextAssets: "+s.TextAssetsErr.Error())
	}
	if len(problems) > 0 {
		return theme.WarningIcon(), strings.Join(problems, "  ")
	}
	var ready []string
	if s.GameDataPath != "" {
		ready = append(ready, "GameData")
	}
	if s.TextAssetsPath != "" {
		ready = append(ready, "TextAssets")
	}
	if len(ready) == 0 {
		return nil, ""
	}
	return theme.ConfirmIcon(), strings.Join(ready, " and ") + " ready."
}

// boundedSetupStatus keeps filesystem diagnostics useful without allowing a
// deeply nested absolute path to consume the dialog and displace its footer.
// Preserve both ends so users can identify the root and the failing leaf.
func boundedSetupStatus(message string) string {
	const maxRunes = 96
	runes := []rune(message)
	if len(runes) <= maxRunes {
		return message
	}
	const suffixRunes = 28
	return string(runes[:maxRunes-suffixRunes-1]) + "…" + string(runes[len(runes)-suffixRunes:])
}

type setupPathDetection struct {
	GameDataPath   string
	TextAssetsPath string
	Notes          []string
	FoundAny       bool
}

// detectSetupPaths applies independent detector results without making
// GameData a prerequisite for TextAssets discovery.
func detectSetupPaths(
	currentGameData, currentTextAssets string,
	findGameData func() string,
	findTextAssets func(string) string,
) setupPathDetection {
	result := setupPathDetection{
		GameDataPath:   currentGameData,
		TextAssetsPath: currentTextAssets,
	}
	if found := findGameData(); found != "" {
		result.FoundAny = true
		if ShouldReplaceGamedataPath(result.GameDataPath, found) {
			result.GameDataPath = found
			result.Notes = append(result.Notes, "GameData found")
		} else {
			result.Notes = append(result.Notes, "existing GameData kept")
		}
	}
	if found := findTextAssets(result.GameDataPath); found != "" {
		result.FoundAny = true
		if ShouldFillTextAssetsPath(result.TextAssetsPath, found) {
			result.TextAssetsPath = found
			result.Notes = append(result.Notes, "TextAssets found")
		}
	}
	return result
}

// showSetupWizardWithWarning opens a compact first-run surface. Core setup
// deliberately fits without scrolling at the supported window sizes: one
// concise introduction, one detection action, two path rows, and a conventional
// footer. Asset folders are optional, so dismissing the dialog is explicit.
func (a *App) showSetupWizardWithWarning(notice string) {
	gamedataEntry := NewInputEntry()
	gamedataEntry.SetPlaceHolder("Path to the Jedi Academy GameData folder")
	gamedataEntry.SetText(a.config.GamedataPath)

	textAssetsEntry := NewInputEntry()
	textAssetsEntry.SetPlaceHolder("Optional path to a TextAssets checkout")
	textAssetsEntry.SetText(a.config.TextAssetsPath)

	var d *dialog.CustomDialog
	var saveButton *widget.Button

	statusIcon := widget.NewIcon(theme.InfoIcon())
	statusLabel := widget.NewLabel("")
	// Break long absolute paths inside the allocated center column. Border
	// gives the label the full remaining row width; HBox sized to the label's
	// minimum content width and could collapse it to a one-character column.
	statusLabel.Wrapping = fyne.TextWrapBreak
	statusRow := container.NewBorder(
		nil, nil,
		container.NewGridWrap(fyne.NewSize(20, 20), statusIcon),
		nil,
		statusLabel,
	)
	statusRow.Hide()

	setStatus := func(icon fyne.Resource, message string) {
		statusIcon.SetResource(icon)
		statusLabel.SetText(boundedSetupStatus(message))
		statusRow.Show()
		statusRow.Refresh()
	}
	validateAndShow := func() setupPathValidation {
		state := validateSetupPaths(gamedataEntry.Text, textAssetsEntry.Text)
		icon, message := state.status()
		if message == "" {
			statusLabel.SetText("")
			statusRow.Hide()
		} else {
			setStatus(icon, message)
		}
		if saveButton != nil {
			if state.CanUse {
				saveButton.Enable()
			} else {
				saveButton.Disable()
			}
		}
		return state
	}
	gamedataEntry.OnChanged = func(string) { validateAndShow() }
	textAssetsEntry.OnChanged = func(string) { validateAndShow() }

	browseGamedata := func() {
		picker := dialog.NewFolderOpen(func(uri fyne.ListableURI, err error) {
			if err != nil {
				setStatus(theme.WarningIcon(), "Could not open that folder: "+err.Error())
				return
			}
			if uri != nil {
				gamedataEntry.SetText(uri.Path())
			}
		}, a.mainWindow)
		if parents := CommonGamedataParents(); len(parents) > 0 {
			if lister, err := storage.ListerForURI(storage.NewFileURI(parents[0])); err == nil {
				picker.SetLocation(lister)
			}
		}
		picker.SetConfirmText("Choose GameData")
		picker.Show()
	}

	browseTextAssets := func() {
		picker := dialog.NewFolderOpen(func(uri fyne.ListableURI, err error) {
			if err != nil {
				setStatus(theme.WarningIcon(), "Could not open that folder: "+err.Error())
				return
			}
			if uri != nil {
				textAssetsEntry.SetText(uri.Path())
			}
		}, a.mainWindow)
		picker.SetConfirmText("Choose TextAssets")
		picker.Show()
	}

	pathRow := func(label string, entry *widget.Entry, browse func()) fyne.CanvasObject {
		title := widget.NewLabelWithStyle(label, fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
		browseButton := widget.NewButtonWithIcon("Browse", theme.FolderOpenIcon(), browse)
		browseButton.Importance = widget.LowImportance
		field := container.NewBorder(nil, nil, nil, browseButton, entry)
		return container.NewVBox(title, field)
	}

	autoDetectBtn := widget.NewButtonWithIcon("Find installation", theme.SearchIcon(), func() {
		detected := detectSetupPaths(
			gamedataEntry.Text,
			textAssetsEntry.Text,
			DetectGamedataPath,
			func(effectiveGameData string) string {
				if effectiveGameData != "" {
					if paired := DetectTextAssetsPath(effectiveGameData); paired != "" {
						return paired
					}
				}
				return DetectTextAssetsStandalone(effectiveGameData)
			},
		)
		if detected.GameDataPath != gamedataEntry.Text {
			gamedataEntry.SetText(detected.GameDataPath)
		}
		if detected.TextAssetsPath != textAssetsEntry.Text {
			textAssetsEntry.SetText(detected.TextAssetsPath)
		}

		state := validateAndShow()
		if !detected.FoundAny && !state.CanUse {
			setStatus(theme.InfoIcon(), "Nothing was found automatically. Choose either folder below, or skip setup.")
			return
		}
		if len(detected.Notes) > 0 && state.CanUse {
			setStatus(theme.ConfirmIcon(), strings.Join(detected.Notes, "; ")+".")
		}
	})

	heading := canvas.NewText("Connect local assets", theme.ForegroundColor())
	heading.TextSize = SizeHeading
	heading.TextStyle = fyne.TextStyle{Bold: true}

	intro := widget.NewLabel("Foundry edits local files without an account. Add GameData for portraits and asset browsing, or skip setup and start editing now.")
	intro.Wrapping = fyne.TextWrapWord

	detectHint := widget.NewLabelWithStyle(
		"Search common Steam, retail, GOG, Wine, and OpenJK locations.",
		fyne.TextAlignLeading,
		fyne.TextStyle{Italic: true},
	)
	detectHint.Wrapping = fyne.TextWrapWord
	detectRow := container.NewBorder(nil, nil, nil, autoDetectBtn, detectHint)

	fields := container.NewVBox(
		pathRow("GameData — portraits and asset browsing", gamedataEntry, browseGamedata),
		Gap(SpaceXS),
		pathRow("TextAssets — optional loose-file overrides", textAssetsEntry, browseTextAssets),
	)

	contentObjects := []fyne.CanvasObject{heading, intro, Gap(SpaceSM)}
	if notice != "" {
		warningIcon := widget.NewIcon(theme.WarningIcon())
		warningText := widget.NewLabel(notice)
		warningText.Wrapping = fyne.TextWrapWord
		warning := NewTilePanel(
			container.NewBorder(nil, nil, warningIcon, nil, warningText),
			TileOpts{AccentColor: color.NRGBA{R: 220, G: 160, B: 70, A: 255}, FillAlpha: 16, StrokeAlpha: 60, Padded: true},
		)
		contentObjects = append(contentObjects, warning, Gap(SpaceSM))
	}
	contentObjects = append(contentObjects,
		detectRow,
		NewAccentRule(),
		fields,
		Gap(SpaceSM),
		statusRow,
	)
	content := container.NewVBox(contentObjects...)

	saveSetup := func() {
		state := validateAndShow()
		if !state.CanUse {
			return
		}
		previous := a.config
		a.config.GamedataPath = state.GameDataPath
		a.config.TextAssetsPath = state.TextAssetsPath
		a.config.SetupWizardSeen = true
		if err := a.persistConfig(); err != nil {
			a.config = previous
			setStatus(theme.WarningIcon(), "Configuration was not saved: "+err.Error())
			return
		}
		if a.assetBrowser != nil {
			a.assetBrowser.SetPaths(a.config.GamedataPath, a.config.TextAssetsPath)
		}
		d.Hide()
		a.updateStatus("Local asset folders configured")
	}
	continueWithoutAssets := func() {
		previous := a.config.SetupWizardSeen
		a.config.SetupWizardSeen = true
		if err := a.persistConfig(); err != nil {
			a.config.SetupWizardSeen = previous
			setStatus(theme.WarningIcon(), "Configuration was not saved: "+err.Error())
			return
		}
		d.Hide()
		a.updateStatus("Local editing ready; asset folders can be configured in Preferences")
	}

	skipButton := widget.NewButton("Skip for now", continueWithoutAssets)
	skipButton.Importance = widget.LowImportance
	saveButton = widget.NewButtonWithIcon("Use these folders", theme.ConfirmIcon(), saveSetup)
	saveButton.Importance = widget.HighImportance
	validateAndShow()

	d = dialog.NewCustomWithoutButtons("Local Setup", container.NewPadded(content), a.mainWindow)
	actions := container.NewBorder(nil, nil, layout.NewSpacer(), nil, container.NewHBox(skipButton, saveButton))
	d.SetButtons([]fyne.CanvasObject{actions})
	d.Resize(a.boundedDialogSize(fyne.NewSize(760, 520)))
	d.Show()
}

// boundedDialogSize clamps a desired dialog size to the parent
// window's canvas minus chrome margins.
func (a *App) boundedDialogSize(want fyne.Size) fyne.Size {
	if a == nil || a.mainWindow == nil {
		return want
	}
	c := a.mainWindow.Canvas()
	if c == nil {
		return want
	}
	maxW := c.Size().Width - 48
	maxH := c.Size().Height - 48
	if maxW < 320 {
		maxW = 320
	}
	if maxH < 240 {
		maxH = 240
	}
	w, h := want.Width, want.Height
	if w > maxW {
		w = maxW
	}
	if h > maxH {
		h = maxH
	}
	return fyne.NewSize(w, h)
}

// showFilePickerForEntry opens a file picker for an Entry widget.
func (a *App) showFilePickerForEntry(entry *widget.Entry, title string, filter AssetType) {
	filePickerWindow := a.fyneApp.NewWindow(title)
	filePickerWindow.Resize(fyne.NewSize(1200, 780))

	pickerBrowser := NewAssetBrowser(a.config.GamedataPath, a.config.TextAssetsPath, nil)

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

	cfp.Show(func(asset *AssetEntry) {
		if asset != nil && asset.Path != "" {
			filePath := asset.Path
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
