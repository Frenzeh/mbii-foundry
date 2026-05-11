package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"image"
	"image/png"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/Frenzeh/mbii-foundry/parsers"
)

// MBClasses is now managed via data_loader.go and GetClasses()

var SaberStyles = []string{"SS_NONE", "SS_FAST", "SS_MEDIUM", "SS_STRONG", "SS_DESANN", "SS_TAVION", "SS_DUAL", "SS_STAFF"}
var SaberColors = []string{"0 - Red", "1 - Orange", "2 - Yellow", "3 - Green", "4 - Cyan", "5 - Blue", "6 - Purple", "7 - White", "8 - Black"}
var ClassFlags = []string{
	"CFL_STRONGAGAINSTPHYSICAL",
	"CFL_STATVIEWER",
	"CFL_HEAVYMELEE",
	"CFL_REALTD",
	"CFL_HASQ3",
	"CFL_FASTHACKING",
	"CFL_NOPICKUPS",
	"CFL_BPFREEJUMPS",
	"CFL_SEEING_STEALTH",
	"CFL_AKIMBOPISTOL3",
	"CFL_THERMALROCKETS",
	"CFL_INSTAGIB",
	"CFL_ACIDBLOOD",
	"CFL_DISMEMBERFRAGILE",
	"CFL_BLOODYMELEE",
	"CFL_BUBBLESHIELD",
	"CFL_NO_FUEL_USE",
	"CFL_NO_JETPACK_OVERHEAT",
	"CFL_NO_JETPACK_COOLDOWN",
	"CFL_DISRUPTOR_WALLS",
	"CFL_KILLTEAMONDEATH",
	"CFL_NOLOCATIONALDAMAGE",
	"CFL_RUNFASTMELEE",
	"CFL_NODISMEMBER",
}

// Removed "CFL_SINGLE_ROCKET", "CFL_CUSTOMSKEL", "CFL_EXTRAFLAMEDAMAGE", "CFL_ICETHROWER", "CFL_MIRALUKA", "CFL_FORCEBLINDING", "CFL_SHOTGUN", "CFL_CONCUSSIONRIFLE", "CFL_DEADLYSIGHT", "CFL_WFLAMETHROWER", "CFL_SELFDESTRUCT" as they were commented out or conditional defines in bg_saga.h

type MBCHEditor struct {
	character       *parsers.MBCHCharacter
	currentPath     string
	container       *fyne.Container
	fileManager     *FileManager
	lastError       string
	onHover         func(string, string)
	isDirty         bool
	onDirtyChanged  func(bool)
	onSourceChanged func()
	// loading silences markDirty during programmatic SetText/SetSelected
	// in updateUI (file load + revert + external sync). Without it, every
	// widget reset fires its OnChanged → markDirty, so a freshly-loaded
	// file is "dirty" the moment it opens.
	loading bool

	nameEntry   *ValidatedEntry
	classPicker *ClassIconPicker // replaces the previous widget.Select
	modelEntry  *ValidatedEntry
	skinEntry        *ValidatedEntry
	uiShaderEntry    *ValidatedEntry
	soundsetEntry    *ValidatedEntry
	iconPreview      *canvas.Image // Portrait of the current model+skin (or explicit UI shader); raster-friendly, fills its container
	portraitSource   *widget.Label // Shows whether the portrait is "auto" or an "override" so authors can tell quickly
	weaponsEntry     *widget.Entry
	attributesEntry  *widget.Entry
	forcePowersEntry *widget.Entry
	healthEntry      *ValidatedEntry
	armorEntry       *ValidatedEntry
	forcePoolEntry   *ValidatedEntry
	forceRegenEntry  *ValidatedEntry
	speedEntry       *ValidatedEntry
	apMultEntry      *ValidatedEntry
	bpMultEntry      *ValidatedEntry
	csMultEntry      *ValidatedEntry
	asMultEntry      *ValidatedEntry
	saber1Entry      *ValidatedEntry
	saber2Entry      *ValidatedEntry
	saberColorSelect *widget.Select
	classLimitEntry  *ValidatedEntry
	respawnTimeEntry *ValidatedEntry
	extraLivesEntry  *ValidatedEntry
	isCustomCheck    *widget.Check
	mbPointsEntry    *ValidatedEntry
	descriptionEntry *ValidatedEntry
	sourceView       *widget.RichText // Correct type

	pointBuyUI      *PointBuyUI
	weaponInfoUI    *WeaponInfoUI
	forceInfoUI     *ForceInfoUI
	weaponFlagsUI   *WeaponFlagsEditor  // WP_*Flags HELD_* grid (separate from WeaponInfoUI overrides)
	skinVariantsUI  *SkinVariantsEditor // model_N / skin_N / uishader_N tuples + RGB overrides
	assetBrowser   *AssetBrowser
	iconResolver   *IconResolver
	holocronClient *HolocronClient
	app            *App
	attrGrid       *AttributeGrid
	weaponGrid     *WeaponGrid // New
	holdableGrid   *HoldableGrid

	// New MultiSelect Widgets
	saberStyleSelect *MultiSelectWidget
	classFlagsSelect *MultiSelectWidget
}

func NewMBCHEditor(app *App) *MBCHEditor {
	tCtor := time.Now()
	e := &MBCHEditor{
		character:    parsers.NewMBCHCharacter(),
		fileManager:  app.fileManager, // Use shared manager
		app:          app,
		assetBrowser: app.assetBrowser,
	}
	tA := time.Now()
	e.pointBuyUI = NewPointBuyUI(e)
	LogInfo("NewMBCHEditor: NewPointBuyUI took %s", time.Since(tA))
	tB := time.Now()
	e.weaponInfoUI = NewWeaponInfoUI(e)
	LogInfo("NewMBCHEditor: NewWeaponInfoUI took %s", time.Since(tB))
	tC := time.Now()
	e.forceInfoUI = NewForceInfoUI(e)
	LogInfo("NewMBCHEditor: NewForceInfoUI took %s", time.Since(tC))
	// Initialize onHover to a no-op so Select/Entry OnChanged handlers
	// that fire during LoadFile → updateUI don't hit a nil-deref before
	// the app has called SetOnHover. SetOnHover later replaces this
	// with the real showHoverTooltip callback.
	e.onHover = func(string, string) {}
	tUI := time.Now()
	e.createUI()
	LogInfo("NewMBCHEditor: createUI took %s (total ctor %s)",
		time.Since(tUI), time.Since(tCtor))
	return e
}

func (e *MBCHEditor) SetOnHover(f func(string, string)) {
	e.onHover = func(key, context string) {
		LogInfo("MBCHEditor: onHover triggered for key='%s'", key)
		if f != nil {
			f(key, context)
		}
	}
}

// interact is the sticky-context channel: every OnFocus / OnChanged
// / class-pick / saber-pick call in the editor routes through here,
// so the info panel pins that field as the current work surface.
// Transient hovers (grid rows, pick cards) still go through
// e.onHover, which in turn hits the app's hover-context path —
// MouseOut reverts to the sticky view.
//
// Guarded on e.app because NewMBCHEditor is sometimes called during
// tests / CLI contexts where app is nil.
func (e *MBCHEditor) interact(key, context string) {
	if e.app == nil {
		return
	}
	e.app.showStickyContext(key, context)
}
func (e *MBCHEditor) SetAssetBrowser(ab *AssetBrowser) {
	e.assetBrowser = ab
	// Always build the resolver — its alias tables are VFS-independent
	// and drive the embedded-icon lookup. The VFS is optional backup
	// for IDs that aren't in the alias tables. Refresh both grids so
	// icon columns repopulate now that the resolver is in place.
	var vfs *VirtualFileSystem
	if ab != nil {
		vfs = ab.vfs
	}
	e.iconResolver = NewIconResolver(vfs)
	// Run inline. Earlier wrapper used fyne.Do to "defer to next
	// tick"; on Fyne v2.7.1 from the main thread that DEADLOCKS —
	// fyne.Do waits for queue drain but main is the only thread
	// that drains the queue. Sample showed every thread parked in
	// pthread_cond_wait. Inline refresh is slow but won't hang.
	t0 := time.Now()
	if e.attrGrid != nil {
		e.attrGrid.Refresh()
	}
	if e.weaponGrid != nil {
		e.weaponGrid.Refresh()
	}
	if e.holdableGrid != nil {
		e.holdableGrid.Refresh()
	}
	if e.iconPreview != nil {
		e.updateIconPreview()
	}
	LogInfo("MBCHEditor.SetAssetBrowser: post-load refresh took %s",
		time.Since(t0))
}
func (e *MBCHEditor) SetHolocronClient(client *HolocronClient) { e.holocronClient = client }
func (e *MBCHEditor) SetOnDirtyChanged(f func(bool))           { e.onDirtyChanged = f }
func (e *MBCHEditor) IsDirty() bool                            { return e.isDirty }
func (e *MBCHEditor) MarkClean() {
	e.isDirty = false
	if e.onDirtyChanged != nil {
		e.onDirtyChanged(false)
	}
}

// SourceProvider implementation — lets the right-pane live-source view
// render this editor's current state.
func (e *MBCHEditor) GenerateSource() string {
	e.updateCharacterFromUI()
	content, err := parsers.GenerateMBCH(e.character)
	if err != nil {
		return "// generate error: " + err.Error()
	}
	return content
}
func (e *MBCHEditor) SetOnSourceChanged(f func()) { e.onSourceChanged = f }

func (e *MBCHEditor) markDirty() {
	if e.loading {
		return
	}
	if !e.isDirty {
		e.isDirty = true
		if e.onDirtyChanged != nil {
			e.onDirtyChanged(true)
		}
	}
	if e.onSourceChanged != nil {
		e.onSourceChanged()
	}
}

func (e *MBCHEditor) attachParser(entry *widget.Entry) {
	entry.OnChanged = func(s string) {
		// User is actively typing/editing. That's an interaction —
		// the token they just wrote is the thing they're working on,
		// so pin it as sticky (not a transient hover).
		tokens := strings.Split(s, "|")
		if len(tokens) > 0 {
			last := strings.TrimSpace(tokens[len(tokens)-1])
			parts := strings.Split(last, ",")
			key := strings.TrimSpace(parts[0])
			if len(key) < 3 {
				return
			}
			context := ""
			if len(parts) > 1 {
				context = "Level " + strings.TrimSpace(parts[1])
			}
			e.interact(key, context)
		}
	}
}

func (e *MBCHEditor) launchModelPreview(modelName string) {
	if e.app.config.MD3ViewPath == "" {
		dialog.ShowConfirm("Setup Required",
			"Model preview requires MD3View.\n\nWould you like to configure it now?\n\n(You can download it from Preferences)",
			func(ok bool) {
				if ok {
					e.app.showPreferences()
				}
			}, e.app.mainWindow)
		return
	}

	// Construct model path: models/players/{modelName}/model.glm
	// We need to find this file in the VFS or Gamedata
	relPath := fmt.Sprintf("models/players/%s/model.glm", modelName)

	// Find absolute path if possible
	fullPath := ""
	if e.app.assetBrowser != nil && e.app.assetBrowser.vfs != nil {
		if src, ok := e.app.assetBrowser.vfs.Index[relPath]; ok {
			fullPath = src.FullPath
			if src.PK3Path != "" {
				// If it's in a PK3, we can't easily pass it to md3view unless we extract it
				// or md3view supports pk3s (usually it assumes extracted folder structure)
				dialog.ShowInformation("Packed Asset", "This model is inside a PK3. MD3View may not load it correctly unless extracted.", e.app.mainWindow)
				// Best effort: pass the relative path and hope md3view finds it in base
				fullPath = relPath
			}
		}
	}

	if fullPath == "" {
		// Fallback to constructing it relative to gamedata
		fullPath = filepath.Join(e.app.config.GamedataPath, "base", relPath)
	}

	LogInfo("Previewing: %s (Path: %s)", modelName, fullPath)

	cmd := exec.Command(e.app.config.MD3ViewPath, fullPath)
	// Set Dir to Gamedata so it finds textures
	if e.app.config.GamedataPath != "" {
		cmd.Dir = filepath.Join(e.app.config.GamedataPath, "base")
	}

	err := cmd.Start()
	if err != nil {
		dialog.ShowError(fmt.Errorf("Failed to launch md3view: %v", err), e.app.mainWindow)
	}
}

func (e *MBCHEditor) createUI() {
	noOpVal := func(s string) error { return nil }

	e.nameEntry = NewValidatedEntry(noOpVal)
	e.nameEntry.SetPlaceHolder("e.g. my_jedi_master")
	e.nameEntry.OnChanged = func(s string) { e.markDirty() }
	e.nameEntry.OnFocus = func() { e.interact("name", "") }

	// Class picker — visual icon-card row replacing the old flat
	// widget.Select dropdown. "Pick a class" is fundamentally a
	// visual act (players recognize SBD or Wookie by the icon
	// before the name), so the picker leads with the art.
	e.classPicker = NewClassIconPicker(func(id string) {
		e.character.MBClass = id
		e.markDirty()
		e.interact(id, "Class Definition")
		e.updateIconPreview()
	})
	if e.app != nil {
		e.classPicker.SetHoverHandlers(e.app.showHoverContext, e.app.clearHoverContext)
	}

	e.modelEntry = NewValidatedEntry(noOpVal)
	e.modelEntry.SetPlaceHolder("e.g. cultist")
	e.modelEntry.OnChanged = func(s string) { e.markDirty(); e.updateIconPreview() }
	e.modelEntry.OnFocus = func() { e.interact("model", "") }

	previewBtn := NewTooltipButton("", theme.VisibilityIcon(), func() { e.launchModelPreview(e.modelEntry.Text) }, "Preview Model (requires md3view)")
	browseModelBtn := NewTooltipButton("", theme.FolderOpenIcon(), func() {
		if e.app != nil {
			e.app.showFilePickerForEntry(&e.modelEntry.Entry, "Select Model", AssetTypeModel)
		}
	}, "Browse for Model")

	e.skinEntry = NewValidatedEntry(noOpVal)
	e.skinEntry.OnChanged = func(s string) { e.markDirty(); e.updateIconPreview() }
	e.skinEntry.OnFocus = func() { e.interact("skin", "") }

	e.uiShaderEntry = NewValidatedEntry(noOpVal)
	e.uiShaderEntry.OnChanged = func(s string) { e.markDirty(); e.updateIconPreview() }
	e.uiShaderEntry.OnFocus = func() { e.interact("uishader", "") }

	e.soundsetEntry = NewValidatedEntry(noOpVal)
	e.soundsetEntry.OnChanged = func(s string) { e.markDirty() }
	e.soundsetEntry.OnFocus = func() { e.interact("soundset", "") }

	browseIconBtn := NewTooltipButton("", theme.FolderOpenIcon(), func() {
		if e.app != nil {
			e.app.showFilePickerForEntry(&e.uiShaderEntry.Entry, "Select UI Shader", AssetTypeIcon)
		}
	}, "Browse for Icon")

	// Icon Preview — the character's portrait, pulled from the VFS
	// (not extracted; there are thousands of player-model variants,
	// each their own mb2_icon_<skin>.tga inside its model folder).
	// AssetBrowser.LoadIconResource already caches decoded PNGs to
	// $TMPDIR/mbii-fa-cache/ so subsequent renders are free.
	//
	// Resolution priority (see IconResolver.ResolveClassIcon):
	//   1. UI Shader field non-empty → treat as explicit override
	//      (the author pointed at a specific shader/path)
	//   2. else → convention: models/players/<model>/mb2_icon_<skin>
	//
	// The portraitSource label mirrors this so authors can see at a
	// glance whether the rendered portrait is from their override or
	// the convention — useful when the icon "looks wrong" and you
	// need to know where to intervene.
	//
	// iconPreview stays a widget.Icon for SetResource compatibility,
	// but the layout that embeds it wraps it in the same canvas.Image
	// pipeline the other icon slots use — see the Portrait form row
	// below where updateIconPreview hands off to a refreshable
	// canvas.Image.
	e.iconPreview = canvas.NewImageFromResource(theme.FileImageIcon())
	e.iconPreview.FillMode = canvas.ImageFillContain
	e.iconPreview.ScaleMode = canvas.ImageScaleSmooth
	e.iconPreview.SetMinSize(fyne.NewSize(64, 64))
	e.portraitSource = widget.NewLabel("")
	e.portraitSource.TextStyle = fyne.TextStyle{Italic: true}

	e.weaponsEntry = NewMultiLineInputEntry()
	e.attributesEntry = NewMultiLineInputEntry()
	e.forcePowersEntry = NewMultiLineInputEntry()
	e.weaponsEntry.SetMinRowsVisible(3)
	e.attributesEntry.SetMinRowsVisible(3)
	e.forcePowersEntry.SetMinRowsVisible(3)
	e.attachParser(e.weaponsEntry)
	e.attachParser(e.attributesEntry)
	e.attachParser(e.forcePowersEntry)
	e.weaponsEntry.SetPlaceHolder("WP_SABER|WP_MELEE")
	e.attributesEntry.SetPlaceHolder("MB_ATT_PUSH,3|MB_ATT_PULL,3")
	e.forcePowersEntry.SetPlaceHolder("FP_PUSH,3|FP_PULL,3")

	// Hover dispatcher — wraps e.onHover lazily so grids built here
	// see the live callback set by SetOnHover later, not the no-op
	// captured at construction time. Without this indirection, the
	// grids snapshot the placeholder onHover and never see the real
	// info-panel callback the App wires in afterwards.
	hoverFn := func(k, c string) {
		if e.onHover != nil {
			e.onHover(k, c)
		}
	}

	// Initialize Attribute Grid. Pair hover/unhover so attribute
	// previews revert to the last-interacted field on mouse-out.
	e.attrGrid = NewAttributeGrid("", func(s string) {
		e.attributesEntry.SetText(s)
		e.markDirty()
	}, hoverFn, e.resolveIconResource)
	if e.app != nil {
		e.attrGrid.SetOnUnhover(e.app.clearHoverContext)
		// (i) button click → pin sidebar via showStickyContext. Click
		// path is intentionally separate from hover so clicks always
		// work even when the hover toggle is off (the default).
		e.attrGrid.SetOnClickInfo(e.app.showStickyContext)
	}

	// Initialize Weapon Grid. onHover fires on row-enter (transient
	// info-panel display), onUnhover on row-leave (revert to
	// sticky). Without the leave-side wire the panel would freeze
	// on the last-hovered weapon.
	e.weaponGrid = NewWeaponGrid("", func(s string) {
		e.weaponsEntry.SetText(s)
		e.markDirty()
	}, hoverFn, e.resolveWeaponIconResource)
	if e.app != nil {
		e.weaponGrid.SetOnUnhover(e.app.clearHoverContext)
		// Click on a weapon icon pins the sidebar via sticky context,
		// matching the attribute grid's (i) → showStickyContext route.
		e.weaponGrid.SetOnClickInfo(e.app.showStickyContext)
	}

	// Cross-tab integration — the Inventory card uses these bridges
	// to render level pills bound to the paired MB_ATT_* and to show
	// flag-count + override-exists badges, so users see one weapon's
	// full configuration in one place rather than hopping tabs.
	e.weaponGrid.SetAttributeBridge(
		func(attID string) int {
			if e.attrGrid == nil {
				return 0
			}
			return e.attrGrid.values[attID]
		},
		func(attID string, n int) {
			if e.attrGrid == nil {
				return
			}
			if n == 0 {
				delete(e.attrGrid.values, attID)
			} else {
				e.attrGrid.values[attID] = n
			}
			e.attrGrid.TriggerChange()
			e.attrGrid.Refresh()
			e.markDirty()
		},
	)
	e.weaponGrid.SetFlagsBridge(
		func(wpID string) int {
			// HELD_* flag fields live in ExtraFields keyed by
			// "WP_NameFlags" — count pipe-separated entries.
			if e.character == nil || e.character.ExtraFields == nil {
				return 0
			}
			val := e.character.ExtraFields[wpID+"Flags"]
			if val == "" {
				return 0
			}
			return len(strings.Split(val, "|"))
		},
		func(wpID string) {
			// Tab navigation handled by App via showFlagsTab — wired
			// later if/when the App provides a hook. For now this is a
			// silent no-op so the badge still gets a click handler and
			// the user gets visual feedback that something would happen.
			_ = wpID
		},
	)
	e.weaponGrid.SetOverrideBridge(
		func(wpID string) bool {
			if e.character == nil {
				return false
			}
			for _, wi := range e.character.WeaponOverrides {
				if wi.WeaponToReplace == wpID {
					return true
				}
			}
			return false
		},
		func(wpID string) {
			// Same TODO as the flags jump — needs an App-level hook.
			_ = wpID
		},
	)

	// Holdables grid — circle-badge picker for HI_* inventory items
	// (medpac, cloak, binoculars, sentry, eweb, …). Stored in the
	// same `attributes` pipe-string as MB_ATT_*; on change we splice
	// the HI_* slice into the existing attribute string so the two
	// grids don't trample each other's tokens.
	e.holdableGrid = NewHoldableGrid("", func(holdablesStr string) {
		// Strip existing HI_* from attributesEntry, append fresh slice.
		var keep []string
		for _, tok := range strings.Split(e.attributesEntry.Text, "|") {
			tok = strings.TrimSpace(tok)
			if tok == "" || strings.HasPrefix(tok, "HI_") {
				continue
			}
			keep = append(keep, tok)
		}
		if holdablesStr != "" {
			keep = append(keep, strings.Split(holdablesStr, "|")...)
		}
		e.attributesEntry.SetText(strings.Join(keep, "|"))
		e.markDirty()
	}, hoverFn)
	if e.app != nil {
		e.holdableGrid.SetOnUnhover(e.app.clearHoverContext)
	}

	// Text -> Grid binding. OnChanged reflects active typing — treat
	// the token the user just finished as a sticky interaction.
	e.attributesEntry.OnChanged = func(s string) {
		tokens := strings.Split(s, "|")
		if len(tokens) > 0 {
			last := strings.TrimSpace(tokens[len(tokens)-1])
			parts := strings.Split(last, ",")
			key := strings.TrimSpace(parts[0])
			if len(key) >= 3 {
				context := ""
				if len(parts) > 1 {
					context = "Level " + strings.TrimSpace(parts[1])
				}
				e.interact(key, context)
			}
		}
		e.attrGrid.values = parseAttributesString(s)
		e.markDirty()
	}

	e.weaponsEntry.OnChanged = func(s string) {
		e.weaponGrid.parseString(s)
		e.markDirty()
	}

	e.healthEntry = NewValidatedEntry(func(s string) error {
		if _, err := strconv.Atoi(s); err != nil {
			return fmt.Errorf("must be an integer")
		}
		return nil
	})
	e.healthEntry.SetText("100")
	e.healthEntry.OnChanged = func(s string) { e.markDirty() }
	e.healthEntry.OnFocus = func() { e.interact("maxhealth", "") }

	e.armorEntry = NewValidatedEntry(func(s string) error {
		if _, err := strconv.Atoi(s); err != nil {
			return fmt.Errorf("must be an integer")
		}
		return nil
	})
	e.armorEntry.SetText("0")
	e.armorEntry.OnChanged = func(s string) { e.markDirty() }
	e.armorEntry.OnFocus = func() { e.interact("maxarmor", "") }

	e.forcePoolEntry = NewValidatedEntry(func(s string) error {
		if _, err := strconv.Atoi(s); err != nil {
			return fmt.Errorf("must be an integer")
		}
		return nil
	})
	e.forcePoolEntry.SetText("0")
	e.forcePoolEntry.OnChanged = func(s string) { e.markDirty() }
	e.forcePoolEntry.OnFocus = func() { e.interact("forcepool", "") }

	e.forceRegenEntry = NewValidatedEntry(func(s string) error {
		if _, err := strconv.ParseFloat(s, 64); err != nil {
			return fmt.Errorf("must be a float")
		}
		return nil
	})
	e.forceRegenEntry.SetText("1.0")
	e.forceRegenEntry.OnChanged = func(s string) { e.markDirty() }
	e.forceRegenEntry.OnFocus = func() { e.interact("forceregen", "") } // Mapped to glossary key? Glossary has 'rateOfFire', 'speed'. 'forceregen' might be missing. I'll check.

	e.speedEntry = NewValidatedEntry(func(s string) error {
		if _, err := strconv.ParseFloat(s, 64); err != nil {
			return fmt.Errorf("must be a float")
		}
		return nil
	})
	e.speedEntry.SetText("1.0")
	e.speedEntry.OnChanged = func(s string) { e.markDirty() }
	e.speedEntry.OnFocus = func() { e.interact("speed", "") }

	e.apMultEntry = NewValidatedEntry(func(s string) error {
		if _, err := strconv.ParseFloat(s, 64); err != nil {
			return fmt.Errorf("must be a float")
		}
		return nil
	})
	e.apMultEntry.SetText("1.0")
	e.apMultEntry.OnChanged = func(s string) { e.markDirty() }
	e.apMultEntry.OnFocus = func() { e.interact("MB_ATT_AP_MULTIPLIER", "") }

	e.bpMultEntry = NewValidatedEntry(func(s string) error {
		if _, err := strconv.ParseFloat(s, 64); err != nil {
			return fmt.Errorf("must be a float")
		}
		return nil
	})
	e.bpMultEntry.SetText("1.0")
	e.bpMultEntry.OnChanged = func(s string) { e.markDirty() }
	e.bpMultEntry.OnFocus = func() { e.interact("MB_ATT_BP_MULTIPLIER", "") }

	e.csMultEntry = NewValidatedEntry(func(s string) error {
		if _, err := strconv.ParseFloat(s, 64); err != nil {
			return fmt.Errorf("must be a float")
		}
		return nil
	})
	e.csMultEntry.SetText("1.0")
	e.csMultEntry.OnChanged = func(s string) { e.markDirty() }
	e.csMultEntry.OnFocus = func() { e.interact("MB_ATT_CS_MULTIPLIER", "") }

	e.asMultEntry = NewValidatedEntry(func(s string) error {
		if _, err := strconv.ParseFloat(s, 64); err != nil {
			return fmt.Errorf("must be a float")
		}
		return nil
	})
	e.asMultEntry.SetText("1.0")
	e.asMultEntry.OnChanged = func(s string) { e.markDirty() }
	e.asMultEntry.OnFocus = func() { e.interact("MB_ATT_AS_MULTIPLIER", "") }

	e.saber1Entry = NewValidatedEntry(noOpVal)
	e.saber1Entry.OnChanged = func(s string) { e.markDirty() }
	e.saber1Entry.OnFocus = func() { e.interact("WP_SABER", "Saber 1 Hilt") }

	e.saber2Entry = NewValidatedEntry(noOpVal)
	e.saber2Entry.OnChanged = func(s string) { e.markDirty() }
	e.saber2Entry.OnFocus = func() { e.interact("WP_SABER", "Saber 2 Hilt") }

	e.saberColorSelect = widget.NewSelect(SaberColors, func(s string) {
		parts := strings.Split(s, " - ")
		if len(parts) > 0 {
			if v, err := strconv.Atoi(parts[0]); err == nil {
				e.character.SaberColor = v
			}
		}
	})
	e.saberColorSelect.SetSelected("0 - Red")

	// MultiSelect for Saber Style - Pass OnHover
	var saberStyleOptions []string
	for _, s := range GetSaberStyles() {
		saberStyleOptions = append(saberStyleOptions, s.ID)
	}

	e.saberStyleSelect = NewMultiSelectWidget(saberStyleOptions, "", func(s string) {
		e.character.SaberStyle = s
		e.markDirty()
	}, func(opt string) {
		e.onHover(opt, "")
	})

	e.classLimitEntry = NewValidatedEntry(func(s string) error {
		if _, err := strconv.Atoi(s); err != nil {
			return fmt.Errorf("must be an integer")
		}
		return nil
	})
	e.classLimitEntry.SetText("-1")
	e.classLimitEntry.OnFocus = func() { e.interact("classNumberLimit", "") }

	e.respawnTimeEntry = NewValidatedEntry(func(s string) error {
		if _, err := strconv.Atoi(s); err != nil {
			return fmt.Errorf("must be an integer")
		}
		return nil
	})
	e.respawnTimeEntry.SetText("0")
	e.respawnTimeEntry.OnFocus = func() { e.interact("respawnCustomTime", "") }

	e.extraLivesEntry = NewValidatedEntry(func(s string) error {
		if _, err := strconv.Atoi(s); err != nil {
			return fmt.Errorf("must be an integer")
		}
		return nil
	})
	e.extraLivesEntry.SetText("0")
	e.extraLivesEntry.OnFocus = func() { e.interact("extralives", "") }

	e.isCustomCheck = widget.NewCheck("Enable Custom Build", nil)
	e.mbPointsEntry = NewValidatedEntry(func(s string) error {
		if _, err := strconv.Atoi(s); err != nil {
			return fmt.Errorf("must be an integer")
		}
		return nil
	})
	e.mbPointsEntry.SetText("0")
	e.mbPointsEntry.OnFocus = func() { e.interact("mbPoints", "") }
	e.isCustomCheck.OnChanged = func(b bool) {
		if b {
			e.character.IsCustomBuild = 1
		} else {
			e.character.IsCustomBuild = 0
		}
		// Mirror to the Point Buy tab's checkbox so its display stays in
		// sync — otherwise the two views can disagree and confuse the
		// tester (also why their data went missing on save previously).
		if e.pointBuyUI != nil && e.pointBuyUI.customBuildCheck != nil {
			e.pointBuyUI.customBuildCheck.SetChecked(b)
		}
		e.interact("isCustomBuild", "")
		e.markDirty()
	}
	// Live OnChanged so typing here updates the model immediately (and
	// mirrors the Point Buy entry). Without this the field was only read
	// at save time, and any value entered in the Point Buy tab got
	// overwritten by whatever stale text this entry still showed.
	e.mbPointsEntry.Entry.OnChanged = func(s string) {
		if err := e.mbPointsEntry.validator(s); err != nil {
			e.mbPointsEntry.SetValidationError(err)
			return
		}
		e.mbPointsEntry.SetValidationError(nil)
		n, err := strconv.Atoi(s)
		if err != nil {
			return
		}
		e.character.MBPoints = n
		if e.pointBuyUI != nil && e.pointBuyUI.mbPointsEntry != nil && e.pointBuyUI.mbPointsEntry.Text != s {
			e.pointBuyUI.mbPointsEntry.SetText(s)
		}
		e.markDirty()
	}

	e.descriptionEntry = NewValidatedEntry(noOpVal)
	e.descriptionEntry.MultiLine = true
	e.descriptionEntry.Wrapping = fyne.TextWrapWord
	e.descriptionEntry.SetMinRowsVisible(10)
	e.descriptionEntry.OnFocus = func() { e.interact("description", "") }
	e.descriptionEntry.OnChanged = func(s string) { e.markDirty() }

	// MultiSelect for Class Flags
	var classFlagOptions []string
	for _, f := range GetClassFlags() {
		classFlagOptions = append(classFlagOptions, f.ID)
	}

	e.classFlagsSelect = NewMultiSelectWidget(classFlagOptions, "", func(s string) {
		e.character.ClassFlags = s
		e.markDirty()
	}, func(opt string) {
		e.onHover(opt, "")
	})

	// Profile form. Class picker gets its own row because it's a
	// visual strip, not a single inline input. Icon preview (64px)
	// is a peer to the Name field on the right so the current
	// character's portrait is visible while editing.
	profileForm := widget.NewForm(
		widget.NewFormItem("Name", e.nameEntry),
		widget.NewFormItem("MB Class", e.classPicker),
		widget.NewFormItem("Portrait",
			container.NewHBox(
				container.NewGridWrap(fyne.NewSize(64, 64), e.iconPreview),
				e.portraitSource,
			),
		),
		widget.NewFormItem("Model", container.NewBorder(nil, nil, nil, container.NewHBox(browseModelBtn, previewBtn), e.modelEntry)),
		widget.NewFormItem("Skin", e.skinEntry),
		widget.NewFormItem("UI Shader", container.NewBorder(nil, nil, nil, browseIconBtn, e.uiShaderEntry)),
		widget.NewFormItem("Sound Set", e.soundsetEntry),
	)

	// Add Focus Listeners for Profile fields (requires simple wrapper or changing NewEntry to NewValidatedEntry for consistency if we want OnFocus)
	// For now, let's just make them NewValidatedEntry with always-true validation to get OnFocus
	// Wait, standard Entry doesn't have OnFocus exposed easily.
	// But ValidatedEntry embeds Entry. We need to override FocusGained.

	limitsForm := widget.NewForm(widget.NewFormItem("Class Limit", e.classLimitEntry), widget.NewFormItem("Respawn Time", e.respawnTimeEntry), widget.NewFormItem("Extra Lives", e.extraLivesEntry))
	customBuildForm := widget.NewForm(widget.NewFormItem("", e.isCustomCheck), widget.NewFormItem("MB Points", e.mbPointsEntry))
	// Accordion instead of a stack of cards — users can collapse
	// sections they aren't editing, making the scrollable area much
	// shorter when you only care about a few fields. First item
	// starts open so the common-case "edit name/class" flow is
	// friction-free.
	profileAccordion := widget.NewAccordion(
		widget.NewAccordionItem("Identity", profileForm),
		widget.NewAccordionItem("Game Limits", limitsForm),
		widget.NewAccordionItem("Custom Build", customBuildForm),
	)
	profileAccordion.MultiOpen = true
	profileAccordion.Open(0)

	// Description gets its own pane below the form accordion with a
	// draggable splitter rail between them. Long descriptions on
	// custom-build classes (the in-game help text + tier walkthroughs)
	// can be 30+ lines — keeping description in the accordion forced
	// users to scroll the whole tab. The VSplit lets them drag the
	// description up to take over the tab when they're focused on
	// that field, then drag it back down to edit identity again.
	descLabel := widget.NewLabelWithStyle("Description",
		fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	descPane := container.NewBorder(descLabel, nil, nil, nil,
		container.NewVScroll(e.descriptionEntry))
	profileSplit := container.NewVSplit(
		container.NewVScroll(profileAccordion),
		descPane,
	)
	// Bias toward identity at first — description gets ~25% of height
	// until the user drags the rail.
	profileSplit.SetOffset(0.7)
	profileTab := profileSplit

	statsForm := widget.NewForm(widget.NewFormItem("Max Health", e.healthEntry), widget.NewFormItem("Max Armor", e.armorEntry), widget.NewFormItem("Force Pool", e.forcePoolEntry), widget.NewFormItem("Force Regen", e.forceRegenEntry), widget.NewFormItem("Speed", e.speedEntry))

	// Wrap grids in scroll containers
	weaponScroll := container.NewVScroll(e.weaponGrid.GetContent())
	attrScroll := container.NewVScroll(e.attrGrid.GetContent())

	// Removed Accordion for Grids - Moved to Tabs

	equipForm := widget.NewForm(widget.NewFormItem("Weapons", e.weaponsEntry), widget.NewFormItem("Attributes (Raw)", e.attributesEntry), widget.NewFormItem("Force Powers", e.forcePowersEntry))

	saberForm := widget.NewForm(
		widget.NewFormItem("Saber 1", e.saber1Entry),
		widget.NewFormItem("Saber 2", e.saber2Entry),
		widget.NewFormItem("Color", e.saberColorSelect),
		widget.NewFormItem("Styles", e.saberStyleSelect), // Use MultiSelect
	)
	advForm := widget.NewForm(
		widget.NewFormItem("AP Mult", e.apMultEntry),
		widget.NewFormItem("BP Mult", e.bpMultEntry),
		widget.NewFormItem("CS Mult", e.csMultEntry),
		widget.NewFormItem("AS Mult", e.asMultEntry),
		widget.NewFormItem("Class Flags", e.classFlagsSelect), // Use MultiSelect
	)
	// Merge all Stats & Sabers sections into one accordion so the layout
	// is consistent across tabs and users can collapse any section they
	// don't care about in a given editing session.
	statsAccordion := widget.NewAccordion(
		widget.NewAccordionItem("Vital Statistics", statsForm),
		widget.NewAccordionItem("Raw Data", equipForm),
		widget.NewAccordionItem("Saber Configuration", saberForm),
		widget.NewAccordionItem("Advanced Multipliers & Flags", advForm),
	)
	statsAccordion.MultiOpen = true
	statsAccordion.Open(0) // Vital Statistics open by default

	loadoutTab := container.NewVBox(statsAccordion)

	weaponTab := e.weaponInfoUI.GetContent()
	forceTab := e.forceInfoUI.GetContent()
	pointBuyTab := e.pointBuyUI.GetContent()

	// Weapon flags editor — WP_*Flags HELD_* modifiers. Own tab so
	// the checkbox grid has breathing room; "Weapon Mods" already
	// has WeaponInfo blocks (different concept: model/sound/ammo
	// overrides). Mixing the two in one tab would confuse both.
	if e.weaponFlagsUI == nil {
		e.weaponFlagsUI = NewWeaponFlagsEditor(e)
	}
	flagsTab := container.NewVScroll(e.weaponFlagsUI.GetContent())

	// Skin variants editor — model_N / skin_N / uishader_N tuples
	// for multi-skin characters (Rebel trooper has 20 variants for
	// different unit types, Luke has era variants, etc.). Currently
	// riding through ExtraFields; this panel makes them first-class.
	if e.skinVariantsUI == nil {
		e.skinVariantsUI = NewSkinVariantsEditor(e)
	}
	skinsTab := container.NewVScroll(e.skinVariantsUI.GetContent())

	// Source View — kept allocated so the updateSourceView helper
	// stays safe to call, but no longer rendered as a tab: the right-
	// pane live Source panel (with syntax highlighting + validated
	// editing) has fully replaced it.
	e.sourceView = widget.NewRichTextFromMarkdown("")

	// Every tab's content goes through wrapForTab — bi-directional
	// container.NewScroll — so a tab with wide content (simulator
	// rank-pill rows, skin-variant cards with long path fields,
	// point-buy slot forms, etc.) can't push the whole app window
	// past the user's screen width. Scroll caps MinSize at
	// scrollbar metrics; wide content just scrolls internally.
	//
	// This is the same pattern that fixed the sidebar-rail pin on
	// the info panel — applied here defensively across every tab
	// because any future "add a wider widget" change would
	// otherwise silently re-introduce the over-sized-window bug.
	// Class Scalars form — the static apMultiplier / bpMultiplier /
	// csMultiplier / asMultiplier / forceRegen / speed float fields
	// from the ClassInfo block. Surfaced inside the attribute grid's
	// new Resources section (NOT pinned above the grid) so they group
	// with everything else that tweaks pools/regen instead of floating
	// as a sticky banner. The grid pulls this builder via the bridge
	// hook below.
	classScalarsBuilder := func() fyne.CanvasObject {
		header := widget.NewLabelWithStyle("Class Scalars",
			fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
		sub := widget.NewLabelWithStyle(
			"Static float fields on the ClassInfo block (1.0 = neutral). Distinct from the MB_ATT_*_MULTIPLIER point-buy primitives that live in the Point Buy tab.",
			fyne.TextAlignLeading, fyne.TextStyle{Italic: true})
		grid := container.New(layout.NewGridLayoutWithColumns(3),
			widget.NewForm(widget.NewFormItem("AP Mult", e.apMultEntry)),
			widget.NewForm(widget.NewFormItem("BP Mult", e.bpMultEntry)),
			widget.NewForm(widget.NewFormItem("CS Mult", e.csMultEntry)),
			widget.NewForm(widget.NewFormItem("AS Mult", e.asMultEntry)),
			widget.NewForm(widget.NewFormItem("Force Regen", e.forceRegenEntry)),
			widget.NewForm(widget.NewFormItem("Speed", e.speedEntry)),
		)
		body := container.NewVBox(header, sub, grid)
		return NewTilePanel(body, TileOpts{FillAlpha: 18, StrokeAlpha: 70, Padded: true})
	}
	e.attrGrid.SetClassScalarsBuilder(classScalarsBuilder)

	// Inventory tab body — weapons + holdables in a SINGLE scroll
	// (no sticky split). User feedback: the VSplit kept holdables
	// pinned to the bottom of the viewport, which felt floaty and
	// stole vertical space from the weapon grid. Now everything
	// scrolls together: weapon family sections, then a separator,
	// then the Holdables family sections at the bottom of the same
	// scrollable column. WeaponGrid's existing scroll is replaced
	// with a flat content stack so the outer VScroll wrapping
	// inventoryBody handles all scrolling.
	holdablesHeader := widget.NewLabelWithStyle("Holdables (Inventory items)",
		fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	holdablesSub := widget.NewLabelWithStyle(
		"HI_* items the class can spawn with. Click a circle to toggle on/off.",
		fyne.TextAlignLeading, fyne.TextStyle{Italic: true})
	inventoryBody := container.NewVBox(
		e.weaponGrid.GetContent(),
		widget.NewSeparator(),
		container.NewPadded(container.NewVBox(holdablesHeader, holdablesSub)),
		e.holdableGrid.GetContent(),
	)

	tabs := container.NewAppTabs(
		container.NewTabItem("Profile", wrapForTab(profileTab)),
		container.NewTabItem("Attributes", wrapForTab(e.attrGrid.GetContent())),
		container.NewTabItem("Inventory", wrapForTab(inventoryBody)),
		container.NewTabItem("Flags", wrapForTab(e.weaponFlagsUI.GetContent())),
		container.NewTabItem("Skins", wrapForTab(e.skinVariantsUI.GetContent())),
		container.NewTabItem("Stats & Sabers", wrapForTab(loadoutTab)),
		container.NewTabItem("Weapon Mods", wrapForTab(weaponTab)),
		container.NewTabItem("Force Mods", wrapForTab(forceTab)),
		container.NewTabItem("Point Buy", wrapForTab(pointBuyTab)),
	)
	// wrapForTab wraps what used to be wrapped with VScroll/…Scroll
	// variants — previously inconsistent, some tabs missed the wrap
	// and propagated MinSize upward.
	_ = attrScroll
	_ = weaponScroll
	_ = flagsTab
	_ = skinsTab

	e.container = container.NewMax(tabs)
}

// wrapForTab wraps a tab's content in a bi-directional Scroll with
// a sensible minimum so the MBCH editor can never demand a window
// wider/taller than the user's screen. Unlike VScroll (which
// inherits content's MinSize.Width), container.NewScroll caps at
// the scroll's explicit MinSize in both dimensions.
func wrapForTab(content fyne.CanvasObject) fyne.CanvasObject {
	s := container.NewScroll(content)
	s.SetMinSize(fyne.NewSize(320, 280))
	return s
}

func (e *MBCHEditor) updateSourceView() {
	e.updateCharacterFromUI()

	content, err := parsers.GenerateMBCH(e.character)
	if err != nil {
		e.sourceView.ParseMarkdown("Error: " + err.Error())
		return
	}

	// Check 8192 character limit (CRITICAL for MBCH files)
	charCount := len(content)
	if charCount > 8192 {
		e.sourceView.ParseMarkdown(fmt.Sprintf("**⚠️ ERROR: File exceeds 8192 character limit! (%d chars)**\n\nReduce attributes or remove overrides to fix.\n\n---\n\n```\n%s\n```", charCount, content))
		return
	} else if charCount > 7500 {
		e.sourceView.ParseMarkdown(fmt.Sprintf("**⚠️ WARNING: Approaching 8192 character limit (%d/8192)**\n\n---\n\n```\n%s\n```", charCount, content))
		return
	}

	highlighter := NewSyntaxHighlighter()
	e.sourceView.Segments = highlighter.Highlight(content).Segments
	e.sourceView.Refresh()
}

// GetCharacterCount returns the current character count of the generated file
func (e *MBCHEditor) GetCharacterCount() int {
	e.updateCharacterFromUI()
	content, err := parsers.GenerateMBCH(e.character)
	if err != nil {
		return 0
	}
	return len(content)
}

func (e *MBCHEditor) WriteContent(w io.Writer) {
	e.updateCharacterFromUI()
	content, _ := parsers.GenerateMBCH(e.character)
	w.Write([]byte(content))
}

func (e *MBCHEditor) GetContent() fyne.CanvasObject { return e.container }
func (e *MBCHEditor) GetCurrentPath() string        { return e.currentPath }
func (e *MBCHEditor) GetRecentFiles() []RecentFile  { return e.fileManager.GetRecentFiles() }

func (e *MBCHEditor) LoadFile(path string) error {
	LogInfo("Loading file: %s", path)

	var content []byte
	var err error
	var fromVFS bool

	// 1. Try OS File
	file, osErr := os.Open(path)
	if osErr == nil {
		defer file.Close()
		content, err = io.ReadAll(file)
	} else {
		// 2. Try VFS
		if e.assetBrowser != nil && e.assetBrowser.vfs != nil {
			rc, vfsErr := e.assetBrowser.vfs.ReadFile(path)
			if vfsErr == nil {
				defer rc.Close()
				content, err = io.ReadAll(rc)
				fromVFS = true
			} else {
				err = vfsErr // Return VFS error if both fail
			}
		} else {
			err = osErr
		}
	}

	if err != nil {
		e.lastError = fmt.Sprintf("Failed to read file: %v", err)
		return err
	}

	// Use the parser!
	LogInfo("Parsing content...")
	char, err := parsers.ParseMBCH(string(content))
	if err != nil {
		LogInfo("Parser Error: %v", err)
		dialog.ShowError(fmt.Errorf("Error parsing file: %v\nProceeding with partial data.", err), fyne.CurrentApp().Driver().AllWindows()[0])
		return err
	}

	LogInfo("Parsed Character: Name='%s', Class='%s'", char.Name, char.MBClass)

	e.character = char

	if fromVFS {
		e.currentPath = "" // Read-only / New file state
	} else {
		e.currentPath = path
	}

	e.updateUI()
	// updateUI silences markDirty via e.loading, but in case any later
	// callback path slipped through (e.g. an attribute-grid rebuild
	// that fires OnChanged outside the guard), reset to clean here so
	// the file opens in a "no unsaved changes" state.
	e.MarkClean()
	if e.fileManager != nil && !fromVFS {
		e.fileManager.AddRecentFile(path)
	}
	e.lastError = ""
	return nil
}

func (e *MBCHEditor) SaveToWriter(w io.Writer) error {
	e.updateCharacterFromUI()
	content, err := parsers.GenerateMBCH(e.character)
	if err != nil {
		e.lastError = fmt.Sprintf("Failed to generate content: %v", err)
		return err
	}
	if len(content) > 8192 {
		e.lastError = fmt.Sprintf("File exceeds 8192 character limit (%d chars)", len(content))
		return fmt.Errorf("file exceeds 8192 character limit (%d chars) - reduce attributes or remove overrides", len(content))
	}

	_, err = w.Write([]byte(content))
	return err
}

func (e *MBCHEditor) SaveFile(path string) error {
	if e.fileManager != nil {
		e.fileManager.CreateBackup(path)
	}

	// Engine requires a non-empty `name` field — BG_SiegeParseClassFile
	// hard-errors with "Siege class without name entry" when it's blank
	// (bg_saga.c:2341). Files match the engine class by filename anyway,
	// so derive name from the basename when the user left it empty.
	if strings.TrimSpace(e.character.Name) == "" {
		base := filepath.Base(path)
		e.character.Name = strings.TrimSuffix(base, filepath.Ext(base))
		if e.nameEntry != nil {
			e.nameEntry.SetText(e.character.Name)
		}
	}

	file, err := os.Create(path)
	if err != nil {
		e.lastError = fmt.Sprintf("Failed to create file: %v", err)
		return err
	}
	defer file.Close()

	if err := e.SaveToWriter(file); err != nil {
		return err
	}

	e.currentPath = path
	e.MarkClean()
	return nil
}

func (e *MBCHEditor) SetCurrentPath(path string) {
	e.currentPath = path
}

func (e *MBCHEditor) ExportJSON(path string) error {
	e.updateCharacterFromUI()
	data, _ := json.MarshalIndent(e.character, "", "  ")
	return os.WriteFile(path, data, 0644)
}

func (e *MBCHEditor) ImportJSON(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	char := parsers.NewMBCHCharacter()
	json.Unmarshal(data, char)
	e.character = char
	e.updateUI()
	return nil
}

func (e *MBCHEditor) Validate() []string {
	e.updateCharacterFromUI()
	v := NewValidator()
	issues := v.ValidateCharacter(e.character)
	// Per-block byte-budget warnings — re-render the file once and
	// scan the result for block sizes. Cheap (single allocation) and
	// catches over-stuffed ClassInfo / WeaponInfoN / ForceInfoN before
	// the engine truncates them silently at load time.
	if rendered, err := parsers.GenerateMBCH(e.character); err == nil {
		issues = append(issues, v.ValidateBlockSizes(rendered)...)
	}
	return issues
}

func (e *MBCHEditor) updateUI() {
	LogInfo("Updating UI for Character: %s", e.character.Name)
	// Silence dirty-tracking during programmatic SetText/SetSelected
	// fan-out below — those fire OnChanged on every widget that gets
	// touched and would otherwise mark the just-loaded file dirty.
	e.loading = true
	defer func() {
		e.loading = false
		// Re-render source for the source panel after UI sync, and
		// ensure dirty stays clean post-load. SaveFile / explicit edits
		// re-mark dirty as needed.
		if e.onSourceChanged != nil {
			e.onSourceChanged()
		}
	}()
	// Standard updates
	e.nameEntry.SetText(e.character.Name)
	e.classPicker.SetSelected(e.character.MBClass)
	e.modelEntry.SetText(e.character.Model)
	e.skinEntry.SetText(e.character.Skin)
	e.uiShaderEntry.SetText(e.character.UIShader)
	e.soundsetEntry.SetText(e.character.Soundset)
	e.weaponsEntry.SetText(e.character.Weapons)
	e.attributesEntry.SetText(e.character.Attributes)
	e.forcePowersEntry.SetText(e.character.ForcePowers)
	e.saberStyleSelect.SetSelected(e.character.SaberStyle) // Use MultiSelect
	e.classFlagsSelect.SetSelected(e.character.ClassFlags) // Use MultiSelect
	e.healthEntry.SetText(strconv.Itoa(e.character.MaxHealth))
	e.armorEntry.SetText(strconv.Itoa(e.character.MaxArmor))
	e.forcePoolEntry.SetText(strconv.Itoa(e.character.ForcePool))
	e.forceRegenEntry.SetText(fmt.Sprintf("%.1f", e.character.ForceRegen))
	e.speedEntry.SetText(fmt.Sprintf("%.1f", e.character.Speed))
	e.apMultEntry.SetText(fmt.Sprintf("%.1f", e.character.APMultiplier))
	e.bpMultEntry.SetText(fmt.Sprintf("%.1f", e.character.BPMultiplier))
	e.csMultEntry.SetText(fmt.Sprintf("%.1f", e.character.CSMultiplier))
	e.asMultEntry.SetText(fmt.Sprintf("%.1f", e.character.ASMultiplier))
	e.saber1Entry.SetText(e.character.Saber1)
	e.saber2Entry.SetText(e.character.Saber2)
	e.saberColorSelect.SetSelected(SaberColors[e.character.SaberColor])
	e.classLimitEntry.SetText(strconv.Itoa(e.character.ClassNumberLimit))
	e.respawnTimeEntry.SetText(strconv.Itoa(e.character.RespawnCustomTime))
	e.extraLivesEntry.SetText(strconv.Itoa(e.character.ExtraLives))
	e.isCustomCheck.SetChecked(e.character.IsCustomBuild == 1)
	e.mbPointsEntry.SetText(strconv.Itoa(e.character.MBPoints))
	e.descriptionEntry.SetText(e.character.Description)

	// Run inline. fyne.Do from the main thread deadlocks on Fyne
	// v2.7.1 — the dispatch queue waits for main to drain but main
	// IS the caller. Sample dump confirmed every thread parked in
	// __psynch_cvwait. Heavy widget rebuild on the click stack is
	// slow but won't hang.
	e.attrGrid.values = parseAttributesString(e.character.Attributes)
	e.weaponGrid.parseString(e.character.Weapons)
	if e.holdableGrid != nil {
		e.holdableGrid.values = parseHoldablesString(e.character.Attributes)
	}
	t0 := time.Now()
	e.pointBuyUI.UpdateUI()
	if e.weaponFlagsUI != nil {
		e.weaponFlagsUI.Refresh()
	}
	if e.skinVariantsUI != nil {
		e.skinVariantsUI.Refresh()
	}
	e.weaponInfoUI.UpdateUI()
	e.forceInfoUI.UpdateUI()
	e.attrGrid.Refresh()
	e.weaponGrid.Refresh()
	if e.holdableGrid != nil {
		e.holdableGrid.Refresh()
	}
	LogInfo("MBCHEditor.updateUI: refresh took %s", time.Since(t0))
}

func (e *MBCHEditor) updateCharacterFromUI() {
	// Restore from Grid if text is missing but grid has data (Fix for tab switch clearing)
	if e.attributesEntry.Text == "" && len(e.attrGrid.values) > 0 {
		e.attrGrid.TriggerChange()
	}
	if e.weaponsEntry.Text == "" && len(e.weaponGrid.selected) > 0 {
		e.weaponGrid.TriggerChange()
	}

	// Basic
	e.character.Name = e.nameEntry.Text
	e.character.MBClass = e.classPicker.Selected()
	e.character.Model = e.modelEntry.Text
	e.character.Skin = e.skinEntry.Text
	e.character.UIShader = e.uiShaderEntry.Text
	e.character.Soundset = e.soundsetEntry.Text
	e.character.Weapons = e.weaponsEntry.Text
	e.character.Attributes = e.attributesEntry.Text
	e.character.ForcePowers = e.forcePowersEntry.Text
	e.character.SaberStyle = e.saberStyleSelect.GetSelected() // Get from MultiSelect
	e.character.ClassFlags = e.classFlagsSelect.GetSelected() // Get from MultiSelect

	// Numerical parsing. Upper bounds are deliberately permissive — the
	// engine stores these as plain int/float with no hard cap (bg_saga.c
	// atoi/atof), so the old 999 / 10.0 ceilings were UI inventions that
	// silently truncated values testers set higher.
	e.character.MaxHealth = parseEntryInt(e.healthEntry, 0, 999999)
	e.character.MaxArmor = parseEntryInt(e.armorEntry, 0, 999999)
	e.character.ForcePool = parseEntryInt(e.forcePoolEntry, 0, 999999)
	e.character.ForceRegen = parseEntryFloat(e.forceRegenEntry, 0.0, 1000.0)
	e.character.Speed = parseEntryFloat(e.speedEntry, 0.0, 1000.0)
	e.character.APMultiplier = parseEntryFloat(e.apMultEntry, 0.0, 1000.0)
	e.character.BPMultiplier = parseEntryFloat(e.bpMultEntry, 0.0, 1000.0)
	e.character.CSMultiplier = parseEntryFloat(e.csMultEntry, 0.0, 1000.0)
	e.character.ASMultiplier = parseEntryFloat(e.asMultEntry, 0.0, 1000.0)

	e.character.Saber1 = e.saber1Entry.Text
	e.character.Saber2 = e.saber2Entry.Text
	e.character.ClassNumberLimit = parseEntryInt(e.classLimitEntry, -1, 99)
	e.character.RespawnCustomTime = parseEntryInt(e.respawnTimeEntry, 0, 999)
	e.character.ExtraLives = parseEntryInt(e.extraLivesEntry, 0, 99)
	if e.isCustomCheck.Checked {
		e.character.IsCustomBuild = 1
	} else {
		e.character.IsCustomBuild = 0
	}
	e.character.MBPoints = parseEntryInt(e.mbPointsEntry, 0, 999)
	e.character.Description = e.descriptionEntry.Text
}

// NewValidatedEntry creates a new Entry with a custom validation function.
// The validator returns an error if the input is invalid.
type ValidatedEntry struct {
	widget.Entry
	validator func(string) error
	OnFocus   func()
}

func NewValidatedEntry(validator func(string) error) *ValidatedEntry {
	entry := &ValidatedEntry{validator: validator}
	entry.ExtendBaseWidget(entry)
	entry.OnChanged = func(s string) {
		if err := entry.validator(s); err != nil {
			entry.SetValidationError(err)
		} else {
			entry.SetValidationError(nil)
		}
	}
	return entry
}

func (e *ValidatedEntry) FocusGained() {
	LogInfo("ValidatedEntry: FocusGained")
	e.Entry.FocusGained()
	if e.OnFocus != nil {
		e.OnFocus()
	}
}

// parseEntryInt safely parses an int from an entry, clamping to min/max.
func parseEntryInt(entry *ValidatedEntry, min, max int) int {
	val, err := strconv.Atoi(entry.Text)
	if err != nil {
		return min // Default to min on error
	}
	if val < min {
		return min
	}
	if val > max {
		return max
	}
	return val
}

// parseEntryFloat safely parses a float from an entry, clamping to min/max.
func parseEntryFloat(entry *ValidatedEntry, min, max float64) float64 {
	val, err := strconv.ParseFloat(entry.Text, 64)
	if err != nil {
		return min // Default to min on error
	}
	if val < min {
		return min
	}
	if val > max {
		return max
	}
	return val
}

// modelPortraitFallbackCache memoizes the result of the VFS index
// scan for `models/players/<model>/mb2_icon_*`. The scan walks
// every entry in the VFS index (tens of thousands of paths) so we
// pay it at most once per model per session. Empty-string results
// are cached too so we don't re-scan known-missing models.
var (
	modelPortraitFallbackCache   = map[string]string{}
	modelPortraitFallbackCacheMu sync.RWMutex
)

// lookupModelPortraitFallback finds any `models/players/<model>/mb2_icon_*`
// asset in the VFS, with caching. Returns "" when nothing matches.
func lookupModelPortraitFallback(model string, vfs *VirtualFileSystem) string {
	if model == "" || vfs == nil {
		return ""
	}
	key := strings.ToLower(model)
	modelPortraitFallbackCacheMu.RLock()
	if v, ok := modelPortraitFallbackCache[key]; ok {
		modelPortraitFallbackCacheMu.RUnlock()
		return v
	}
	modelPortraitFallbackCacheMu.RUnlock()

	dirPrefix := "models/players/" + key + "/mb2_icon_"
	var found string
	vfs.mu.RLock()
	for k := range vfs.Index {
		if strings.HasPrefix(k, dirPrefix) {
			ext := filepath.Ext(k)
			if ext == ".tga" || ext == ".png" || ext == ".jpg" || ext == ".jpeg" {
				found = k
				break
			}
		}
	}
	vfs.mu.RUnlock()

	modelPortraitFallbackCacheMu.Lock()
	modelPortraitFallbackCache[key] = found
	modelPortraitFallbackCacheMu.Unlock()
	return found
}

func (e *MBCHEditor) updateIconPreview() {
	if e.iconPreview == nil {
		return
	}

	model := e.modelEntry.Text
	skin := e.skinEntry.Text
	uishader := e.uiShaderEntry.Text

	// Source label reflects the FILE's intent (override vs auto)
	// independent of whether we can resolve a texture for it.
	source := "auto"
	if uishader != "" && uishader != "default" {
		source = "override"
	}

	// setMissing renders the boxicon placeholder and clears the
	// source label entirely. Earlier versions wrote diagnostic text
	// like "override · no image found" — useful while debugging,
	// noisy in normal use. The placeholder image speaks for itself.
	setMissing := func(_ string) {
		if e.portraitSource != nil {
			e.portraitSource.SetText("")
		}
		if res := loadBoxiconResource("box"); res != nil {
			e.iconPreview.Resource = res
		} else {
			e.iconPreview.Resource = theme.FileImageIcon()
		}
		e.iconPreview.Refresh()
	}

	if e.iconResolver == nil || e.assetBrowser == nil {
		// Likely transient — SetAssetBrowser wires the resolvers a
		// moment after the editor is constructed, and updateUI may
		// fire between those two points during a Recent-file open.
		// Render the placeholder so the user isn't staring at the
		// generic file icon, and label the state honestly.
		setMissing("loading…")
		LogInfo("updateIconPreview: resolver not ready (model=%q skin=%q uishader=%q)",
			model, skin, uishader)
		return
	}

	// Walk the candidate list — author's `uishader` first, then the
	// `mb2_icon_<skin>` / `icon_<skin>` / bare-skin / `mb2_icon_default`
	// fallbacks. Each goes through LoadIconResource which probes
	// embedded HUD → shader-resolved texture → direct extension.
	// First non-nil wins.
	candidates := e.iconResolver.ResolveClassIconCandidates(model, skin, uishader)
	for _, candidate := range candidates {
		if res := e.assetBrowser.LoadIconResource(candidate); res != nil {
			if e.portraitSource != nil {
				e.portraitSource.SetText(source)
			}
			e.iconPreview.Resource = res
			e.iconPreview.Refresh()
			return
		}
	}

	// LAST-RESORT: scan the VFS for any `models/players/<model>/mb2_icon_*`
	// — the file may ship a skin-specific portrait under a name we
	// can't predict (jedi_zf has mb2_icon_legends1.jpg, not
	// mb2_icon_<skin> or mb2_icon_default).
	//
	// The scan iterates the entire VFS index (50k+ entries on a
	// fully-loaded MBII install). It runs at most once per model
	// per session — modelPortraitFallbackCache memoizes the result
	// so subsequent updateIconPreview calls for the same model
	// don't re-scan. updateIconPreview fires 3+ times during file
	// load (one per OnChanged on model/skin/uishader entries); the
	// cache is the difference between a 100-300ms hitch each time
	// vs an instant lookup.
	if model != "" && e.assetBrowser != nil && e.assetBrowser.vfs != nil {
		fallbackPath := lookupModelPortraitFallback(model, e.assetBrowser.vfs)
		if fallbackPath != "" {
			if res := e.assetBrowser.LoadIconResource(fallbackPath); res != nil {
				if e.portraitSource != nil {
					e.portraitSource.SetText(source)
				}
				e.iconPreview.Resource = res
				e.iconPreview.Refresh()
				return
			}
		}
	}

	LogInfo("updateIconPreview: no candidate resolved (model=%q skin=%q uishader=%q tried=%d)",
		model, skin, uishader, len(candidates))
	setMissing("no image found")
}

func (e *MBCHEditor) resolveIconResource(id string) fyne.Resource {
	// Compute the path. Prefer the resolver (handles both alias lookup
	// and VFS-backed candidate fallback); if unavailable, do the alias
	// table lookup ourselves so embedded icons still render without a
	// populated VFS.
	var path string
	if e.iconResolver != nil {
		path = e.iconResolver.ResolveAttributeIcon(id)
	} else if alias, ok := attributeIconAliases[id]; ok {
		path = "gfx/menus/alpha/" + alias
	}
	if path != "" {
		// Embedded first — LoadGameIcon keys on basename and doesn't need
		// the AssetBrowser / VFS to be initialized.
		if img, ok := LoadGameIcon(nil, path); ok {
			return staticPNGResource(filepath.Base(path)+".png", img)
		}
		// VFS fallback — only if the AssetBrowser is connected.
		if e.assetBrowser != nil {
			if res := e.assetBrowser.LoadIconResource(path); res != nil {
				return res
			}
		}
	}
	// Boxicon fallback — when neither MBII HUD nor VFS has an icon
	// for this attribute, pick one by keyword match against the
	// attribute's ID + name. Better than rendering an empty 24px slot
	// — the row gets a glyph that hints at the attribute's archetype
	// (heart for health, shield for defense, etc.) so the eye can
	// still scan the grid quickly.
	displayName := ""
	for _, a := range MBIIAttributes {
		if a.ID == id {
			displayName = a.Name
			break
		}
	}
	return FallbackIconForAttribute(id, displayName)
}

// resolveWeaponIconResource mirrors resolveIconResource but uses the
// weapon-specific path pattern (gfx/hud/w_icon_*). Embedded PNGs in
// assets/icons/weapons/ take priority over VFS — see game_icon.go's
// LoadGameIcon for the lookup order. The wrapper first tries the
// embedded set directly so the grid keeps rendering even when the
// user hasn't loaded any PK3s (IconResolver requires a populated
// VFS index for the check-exists path). VFS is the fallback for any
// weapon we haven't extracted yet.
func (e *MBCHEditor) resolveWeaponIconResource(id string) fyne.Resource {
	// Authoritative basename comes from weaponIconAliases — MBII's HUD
	// icon filenames rarely match the WP_* suffix verbatim
	// (WP_BRYAR_PISTOL → w_icon_blaster_pistol, WP_THROWER →
	// w_icon_cr-24_flamerifle, etc.). The naive lowercase-suffix path
	// previously skipped the alias lookup entirely, so embedded
	// resolution failed for ~70% of weapons even though the PNGs ship
	// in assets/icons/weapons/.
	var basenames []string
	if alias, ok := weaponIconAliases[id]; ok && alias != "" {
		basenames = append(basenames, alias)
	}
	// Naive fallback for any new WP_* not in the alias table yet.
	suffix := strings.ToLower(strings.TrimPrefix(id, "WP_"))
	if suffix != "" {
		basenames = append(basenames, "w_icon_"+suffix)
	}
	for _, b := range basenames {
		base := "gfx/hud/" + b
		if img, ok := LoadGameIcon(nil, base); ok {
			// Re-encode to PNG bytes so Fyne accepts it as a resource.
			// LoadGameIcon caches → at most one decode+encode per WP_*.
			return staticPNGResource(filepath.Base(base)+".png", img)
		}
	}
	if e.iconResolver == nil || e.assetBrowser == nil {
		return nil
	}
	path := e.iconResolver.ResolveWeaponIcon(id)
	if path == "" {
		return nil
	}
	return e.assetBrowser.LoadIconResource(path)
}

// staticPNGResource encodes an image.Image to PNG bytes and wraps it
// in a fyne.StaticResource. Small helper because we need this twice —
// the weapon and future class icon paths — and the ceremony around
// encoding-to-bytes-for-Fyne is enough to factor out.
func staticPNGResource(name string, img image.Image) fyne.Resource {
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil
	}
	return fyne.NewStaticResource(name, buf.Bytes())
}
