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

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
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

	// New MultiSelect Widgets
	saberStyleSelect *MultiSelectWidget
	classFlagsSelect *MultiSelectWidget
}

func NewMBCHEditor(app *App) *MBCHEditor {
	e := &MBCHEditor{
		character:    parsers.NewMBCHCharacter(),
		fileManager:  app.fileManager, // Use shared manager
		app:          app,
		assetBrowser: app.assetBrowser,
	}
	e.pointBuyUI = NewPointBuyUI(e)
	e.weaponInfoUI = NewWeaponInfoUI(e)
	e.forceInfoUI = NewForceInfoUI(e)
	// Initialize onHover to a no-op so Select/Entry OnChanged handlers
	// that fire during LoadFile → updateUI don't hit a nil-deref before
	// the app has called SetOnHover. SetOnHover later replaces this
	// with the real showHoverTooltip callback.
	e.onHover = func(string, string) {}
	e.createUI()
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
	if ab != nil && ab.vfs != nil {
		e.iconResolver = NewIconResolver(ab.vfs)
		if e.attrGrid != nil {
			e.attrGrid.Refresh()
		}
	}
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

	// Initialize Attribute Grid. Pair hover/unhover so attribute
	// previews revert to the last-interacted field on mouse-out.
	e.attrGrid = NewAttributeGrid("", func(s string) {
		e.attributesEntry.SetText(s)
		e.markDirty()
	}, e.onHover, e.resolveIconResource)
	if e.app != nil {
		e.attrGrid.SetOnUnhover(e.app.clearHoverContext)
	}

	// Initialize Weapon Grid. onHover fires on row-enter (transient
	// info-panel display), onUnhover on row-leave (revert to
	// sticky). Without the leave-side wire the panel would freeze
	// on the last-hovered weapon.
	e.weaponGrid = NewWeaponGrid("", func(s string) {
		e.weaponsEntry.SetText(s)
		e.markDirty()
	}, e.onHover, e.resolveWeaponIconResource)
	if e.app != nil {
		e.weaponGrid.SetOnUnhover(e.app.clearHoverContext)
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
	e.apMultEntry.OnFocus = func() { e.interact("MB_ATT_AP_MULTIPLIER", "") }

	e.bpMultEntry = NewValidatedEntry(func(s string) error {
		if _, err := strconv.ParseFloat(s, 64); err != nil {
			return fmt.Errorf("must be a float")
		}
		return nil
	})
	e.bpMultEntry.SetText("1.0")
	e.bpMultEntry.OnFocus = func() { e.interact("MB_ATT_BP_MULTIPLIER", "") }

	e.csMultEntry = NewValidatedEntry(func(s string) error {
		if _, err := strconv.ParseFloat(s, 64); err != nil {
			return fmt.Errorf("must be a float")
		}
		return nil
	})
	e.csMultEntry.SetText("1.0")
	e.csMultEntry.OnFocus = func() { e.interact("MB_ATT_CS_MULTIPLIER", "") }

	e.asMultEntry = NewValidatedEntry(func(s string) error {
		if _, err := strconv.ParseFloat(s, 64); err != nil {
			return fmt.Errorf("must be a float")
		}
		return nil
	})
	e.asMultEntry.SetText("1.0")
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

	e.isCustomCheck = widget.NewCheck("Enable Custom Build", func(b bool) {
		if b {
			e.character.IsCustomBuild = 1
		} else {
			e.character.IsCustomBuild = 0
		}
	})
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
		e.interact("isCustomBuild", "")
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
		widget.NewAccordionItem("Description", e.descriptionEntry),
	)
	profileAccordion.MultiOpen = true
	profileAccordion.Open(0)
	profileTab := container.NewVBox(profileAccordion)

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

	tabs := container.NewAppTabs(
		container.NewTabItem("Profile", container.NewVScroll(profileTab)),
		container.NewTabItem("Attributes", attrScroll),
		container.NewTabItem("Inventory", weaponScroll),
		container.NewTabItem("Flags", flagsTab),
		container.NewTabItem("Skins", skinsTab),
		container.NewTabItem("Stats & Sabers", container.NewVScroll(loadoutTab)),
		container.NewTabItem("Weapon Mods", weaponTab),
		container.NewTabItem("Force Mods", forceTab),
		container.NewTabItem("Point Buy", pointBuyTab),
	)

	e.container = container.NewMax(tabs)
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
	return v.ValidateCharacter(e.character)
}

func (e *MBCHEditor) updateUI() {
	LogInfo("Updating UI for Character: %s", e.character.Name)
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

	e.pointBuyUI.UpdateUI()
	if e.weaponFlagsUI != nil {
		e.weaponFlagsUI.Refresh()
	}
	if e.skinVariantsUI != nil {
		e.skinVariantsUI.Refresh()
	}
	e.weaponInfoUI.UpdateUI()
	e.forceInfoUI.UpdateUI()

	// Update Grids
	e.attrGrid.values = parseAttributesString(e.character.Attributes)
	e.attrGrid.Refresh()

	e.weaponGrid.parseString(e.character.Weapons)
	e.weaponGrid.Refresh()
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

	// Numerical parsing with default/clamp for safety
	e.character.MaxHealth = parseEntryInt(e.healthEntry, 1, 9999) // Limit to 9999
	e.character.MaxArmor = parseEntryInt(e.armorEntry, 0, 999)    // Limit to 999
	e.character.ForcePool = parseEntryInt(e.forcePoolEntry, 0, 999)
	e.character.ForceRegen = parseEntryFloat(e.forceRegenEntry, 0.0, 10.0)
	e.character.Speed = parseEntryFloat(e.speedEntry, 0.1, 10.0)
	e.character.APMultiplier = parseEntryFloat(e.apMultEntry, 0.0, 10.0)
	e.character.BPMultiplier = parseEntryFloat(e.bpMultEntry, 0.0, 10.0)
	e.character.CSMultiplier = parseEntryFloat(e.csMultEntry, 0.0, 10.0)
	e.character.ASMultiplier = parseEntryFloat(e.asMultEntry, 0.0, 10.0)

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

func (e *MBCHEditor) updateIconPreview() {
	if e.iconPreview == nil {
		return
	}

	model := e.modelEntry.Text
	skin := e.skinEntry.Text
	uishader := e.uiShaderEntry.Text

	// Derive the source label up front so we can set it regardless
	// of whether the load actually succeeds — the label tells the
	// author what the editor is *trying* to load, which is useful
	// even if the VFS can't find it.
	source := "auto"
	if uishader != "" && uishader != "default" {
		source = "override"
	}
	if e.portraitSource != nil {
		e.portraitSource.SetText(source)
	}

	// If we don't have an asset browser / resolver yet (editor is
	// still being built, or running in a VFS-less test context),
	// leave the placeholder in place. Nothing else to do.
	if e.iconResolver == nil || e.assetBrowser == nil {
		return
	}

	path := e.iconResolver.ResolveClassIcon(model, skin, uishader)
	if path != "" {
		if res := e.assetBrowser.LoadIconResource(path); res != nil {
			e.iconPreview.Resource = res
			e.iconPreview.Refresh()
			return
		}
	}
	// Nothing resolved — fall back to a generic placeholder so the
	// 64px slot still reads as "this is the portrait area" instead
	// of going blank.
	e.iconPreview.Resource = theme.FileImageIcon()
	e.iconPreview.Refresh()
}

func (e *MBCHEditor) resolveIconResource(id string) fyne.Resource {
	if e.iconResolver == nil || e.assetBrowser == nil {
		return nil
	}
	path := e.iconResolver.ResolveAttributeIcon(id)
	if path == "" {
		return nil
	}
	return e.assetBrowser.LoadIconResource(path)
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
	base := "gfx/hud/w_icon_" + strings.ToLower(strings.TrimPrefix(id, "WP_"))
	if img, ok := LoadGameIcon(nil, base); ok {
		// Re-encode to PNG bytes so Fyne accepts it as a resource. The
		// LoadGameIcon cache means this decode+encode round-trip happens
		// at most once per weapon per session.
		return staticPNGResource(filepath.Base(base)+".png", img)
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
