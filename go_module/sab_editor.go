package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strconv"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"image/color"

	"github.com/Frenzeh/mbii-foundry/parsers"
)

var SaberTypes = []string{"SABER_SINGLE", "SABER_STAFF"}
var BladeColors = []string{"red", "orange", "yellow", "green", "blue", "purple", "silver", "random"}

type SABEditor struct {
	saber       *parsers.SaberData
	currentPath string
	container   *fyne.Container
	fileManager *FileManager
	lastError   string

	nameEntry         *widget.Entry
	fullNameEntry     *widget.Entry
	typeSelect        *widget.Select
	modelEntry        *widget.Entry
	customSkinEntry   *widget.Entry
	numBladesEntry    *widget.Entry
	blade1ColorSelect *widget.Select
	blade1LengthEntry *widget.Entry
	blade1RadiusEntry *widget.Entry
	bladePreview      *canvas.Rectangle
	soundOnEntry      *widget.Entry
	soundOffEntry     *widget.Entry
	soundLoopEntry    *widget.Entry
	spinSoundEntry    *widget.Entry
	swingSound1Entry  *widget.Entry
	swingSound2Entry  *widget.Entry
	swingSound3Entry  *widget.Entry
	fallSound1Entry   *widget.Entry
	fallSound2Entry   *widget.Entry
	fallSound3Entry   *widget.Entry
	hitSound1Entry    *widget.Entry
	hitSound2Entry    *widget.Entry
	hitSound3Entry    *widget.Entry
	blockSound1Entry  *widget.Entry
	blockSound2Entry  *widget.Entry
	blockSound3Entry  *widget.Entry
	bounceSound1Entry *widget.Entry
	bounceSound2Entry *widget.Entry
	bounceSound3Entry *widget.Entry

	styleEntry              *widget.Entry
	singleBladeStyleEntry   *widget.Entry
	maxChainEntry           *widget.Entry
	lockBonusEntry          *widget.Entry
	parryBonusEntry         *widget.Entry
	breakParryEntry         *widget.Entry
	disarmBonusEntry        *widget.Entry
	moveSpeedEntry          *widget.Entry
	animSpeedEntry          *widget.Entry
	damageScaleEntry        *widget.Entry
	knockbackEntry          *widget.Entry
	trailStyleEntry         *widget.Entry
	blockEffectEntry        *widget.Entry
	hitPersonEffectEntry    *widget.Entry
	bladeEffectEntry        *widget.Entry
	hitOtherEffectEntry     *widget.Entry
	g2MarksShaderEntry      *widget.Entry
	g2WeaponMarkShaderEntry *widget.Entry

	noWallMarksCheck        *widget.Check
	noDlightCheck           *widget.Check
	noBladeCheck            *widget.Check
	noClashFlareCheck       *widget.Check
	noDismembermentCheck    *widget.Check
	noIdleEffectCheck       *widget.Check
	alwaysBlockCheck        *widget.Check
	noManualDeactivateCheck *widget.Check
	transitionDamageCheck   *widget.Check
	notInOpenCheck          *widget.Check
	notInMPCheck            *widget.Check
	noCartwheelsCheck       *widget.Check
	throwableCheck          *widget.Check
	disarmableCheck         *widget.Check
	blasterBlockingCheck    *widget.Check
	onInWaterCheck          *widget.Check
	bounceOnWallsCheck      *widget.Check
	twoHandedCheck          *widget.Check
	useGoreConfigCheck      *widget.Check
	useGoreConfig2Check     *widget.Check
	noDismemberment2Check   *widget.Check
	noBladeEffectsCheck     *widget.Check
	noBladeEffects2Check    *widget.Check

	slapAnimEntry       *widget.Entry
	readyAnimEntry      *widget.Entry
	jumpAtkUpMoveEntry  *widget.Entry
	jumpAtkFwdMoveEntry *widget.Entry
	lungeAtkMoveEntry   *widget.Entry

	swingSoundsGroup  *widget.Form
	fallSoundsGroup   *widget.Form
	hitSoundsGroup    *widget.Form
	blockSoundsGroup  *widget.Form
	bounceSoundsGroup *widget.Form

	holocronClient *HolocronClient
	assetBrowser   *AssetBrowser
	app            *App
	sourceView     *widget.RichText

	// Dirty tracking for unsaved changes
	isDirty        bool
	onDirtyChanged func(bool)
}

func NewSABEditor(app *App) *SABEditor {
	e := &SABEditor{
		saber:       parsers.NewSaberData(),
		fileManager: app.fileManager,
		app:         app,
	}
	e.createUI()
	return e
}

func (e *SABEditor) SetOnHover(f func(string, string))        {}
func (e *SABEditor) SetAssetBrowser(ab *AssetBrowser)         { e.assetBrowser = ab }
func (e *SABEditor) SetHolocronClient(client *HolocronClient) { e.holocronClient = client }

// Dirty tracking methods
func (e *SABEditor) SetOnDirtyChanged(f func(bool)) { e.onDirtyChanged = f }
func (e *SABEditor) IsDirty() bool                  { return e.isDirty }
func (e *SABEditor) MarkClean() {
	e.isDirty = false
	if e.onDirtyChanged != nil {
		e.onDirtyChanged(false)
	}
}

func (e *SABEditor) markDirty() {
	if !e.isDirty {
		e.isDirty = true
		if e.onDirtyChanged != nil {
			e.onDirtyChanged(true)
		}
	}
}

// Helper to browse assets
func (e *SABEditor) browseAsset(entry *widget.Entry, assetType AssetType) {
	// Ideally use e.app.showFilePickerForEntry if app ref was passed, but SABEditor structure doesn't enforce it yet.
	// Or generic custom picker if assetBrowser is set.
	if e.assetBrowser == nil {
		return
	}

	// win := fyne.CurrentApp().Driver().AllWindows()[0] // Not needed

	filePickerWindow := fyne.CurrentApp().NewWindow("Select Asset")
	filePickerWindow.Resize(fyne.NewSize(900, 600))

	// Use shared browser logic?
	// Ideally re-use the CustomFilePicker logic
	cfp := NewCustomFilePicker(filePickerWindow, e.assetBrowser)
	cfp.Show(func(path string) {
		if path != "" {
			entry.SetText(path)
		}
	})
}

func (e *SABEditor) createUI() {
	e.nameEntry = widget.NewEntry()
	e.fullNameEntry = widget.NewEntry()
	e.typeSelect = widget.NewSelect(SaberTypes, func(s string) { e.saber.SaberType = s; e.markDirty() })
	e.modelEntry = widget.NewEntry()
	e.customSkinEntry = widget.NewEntry()
	e.numBladesEntry = widget.NewEntry()

	// Wire up dirty tracking for identity fields
	e.nameEntry.OnChanged = func(s string) { e.markDirty() }
	e.fullNameEntry.OnChanged = func(s string) { e.markDirty() }
	e.modelEntry.OnChanged = func(s string) { e.markDirty() }
	e.customSkinEntry.OnChanged = func(s string) { e.markDirty() }
	e.numBladesEntry.OnChanged = func(s string) { e.markDirty() }

	browseModelBtn := widget.NewButtonWithIcon("", theme.FolderOpenIcon(), func() { e.browseAsset(e.modelEntry, AssetTypeModel) })

	identityForm := widget.NewForm(
		widget.NewFormItem("Saber Name", e.nameEntry),
		widget.NewFormItem("Display Name", e.fullNameEntry),
		widget.NewFormItem("Saber Type", e.typeSelect),
		widget.NewFormItem("Model", container.NewBorder(nil, nil, nil, browseModelBtn, e.modelEntry)),
		widget.NewFormItem("Custom Skin", e.customSkinEntry),
		widget.NewFormItem("Num Blades", e.numBladesEntry),
	)

	e.bladePreview = canvas.NewRectangle(color.RGBA{0, 0, 255, 255})
	e.bladePreview.SetMinSize(fyne.NewSize(200, 20))
	e.blade1ColorSelect = widget.NewSelect(BladeColors, func(s string) {
		if len(e.saber.Blades) > 0 {
			e.saber.Blades[0].Color = s
			e.updateBladePreview(s)
			e.markDirty()
		}
	})
	e.blade1LengthEntry = widget.NewEntry()
	e.blade1RadiusEntry = widget.NewEntry()
	e.blade1LengthEntry.OnChanged = func(s string) { e.markDirty() }
	e.blade1RadiusEntry.OnChanged = func(s string) { e.markDirty() }
	bladeGrid := container.NewGridWithColumns(2, widget.NewFormItem("Color", e.blade1ColorSelect).Widget, widget.NewFormItem("Preview", e.bladePreview).Widget, widget.NewFormItem("Length", e.blade1LengthEntry).Widget, widget.NewFormItem("Radius", e.blade1RadiusEntry).Widget)

	e.soundOnEntry = widget.NewEntry()
	e.soundOffEntry = widget.NewEntry()
	e.soundLoopEntry = widget.NewEntry()
	e.spinSoundEntry = widget.NewEntry()
	e.soundOnEntry.OnChanged = func(s string) { e.markDirty() }
	e.soundOffEntry.OnChanged = func(s string) { e.markDirty() }
	e.soundLoopEntry.OnChanged = func(s string) { e.markDirty() }
	e.spinSoundEntry.OnChanged = func(s string) { e.markDirty() }

	browseSound := func(entry *widget.Entry) *widget.Button {
		return widget.NewButtonWithIcon("", theme.FolderOpenIcon(), func() { e.browseAsset(entry, AssetTypeSound) })
	}

	soundsForm := widget.NewForm(
		widget.NewFormItem("On", container.NewBorder(nil, nil, nil, browseSound(e.soundOnEntry), e.soundOnEntry)),
		widget.NewFormItem("Off", container.NewBorder(nil, nil, nil, browseSound(e.soundOffEntry), e.soundOffEntry)),
		widget.NewFormItem("Loop", container.NewBorder(nil, nil, nil, browseSound(e.soundLoopEntry), e.soundLoopEntry)),
		widget.NewFormItem("Spin", container.NewBorder(nil, nil, nil, browseSound(e.spinSoundEntry), e.spinSoundEntry)),
	)

	e.swingSound1Entry = widget.NewEntry()
	e.swingSound2Entry = widget.NewEntry()
	e.swingSound3Entry = widget.NewEntry()
	e.swingSound1Entry.OnChanged = func(s string) { e.markDirty() }
	e.swingSound2Entry.OnChanged = func(s string) { e.markDirty() }
	e.swingSound3Entry.OnChanged = func(s string) { e.markDirty() }
	e.swingSoundsGroup = widget.NewForm(widget.NewFormItem("Swing 1", e.swingSound1Entry), widget.NewFormItem("Swing 2", e.swingSound2Entry), widget.NewFormItem("Swing 3", e.swingSound3Entry))

	e.fallSound1Entry = widget.NewEntry()
	e.fallSound2Entry = widget.NewEntry()
	e.fallSound3Entry = widget.NewEntry()
	e.fallSound1Entry.OnChanged = func(s string) { e.markDirty() }
	e.fallSound2Entry.OnChanged = func(s string) { e.markDirty() }
	e.fallSound3Entry.OnChanged = func(s string) { e.markDirty() }
	e.fallSoundsGroup = widget.NewForm(widget.NewFormItem("Fall 1", e.fallSound1Entry), widget.NewFormItem("Fall 2", e.fallSound2Entry), widget.NewFormItem("Fall 3", e.fallSound3Entry))

	e.hitSound1Entry = widget.NewEntry()
	e.hitSound2Entry = widget.NewEntry()
	e.hitSound3Entry = widget.NewEntry()
	e.hitSound1Entry.OnChanged = func(s string) { e.markDirty() }
	e.hitSound2Entry.OnChanged = func(s string) { e.markDirty() }
	e.hitSound3Entry.OnChanged = func(s string) { e.markDirty() }
	e.hitSoundsGroup = widget.NewForm(widget.NewFormItem("Hit 1", e.hitSound1Entry), widget.NewFormItem("Hit 2", e.hitSound2Entry), widget.NewFormItem("Hit 3", e.hitSound3Entry))

	e.blockSound1Entry = widget.NewEntry()
	e.blockSound2Entry = widget.NewEntry()
	e.blockSound3Entry = widget.NewEntry()
	e.blockSound1Entry.OnChanged = func(s string) { e.markDirty() }
	e.blockSound2Entry.OnChanged = func(s string) { e.markDirty() }
	e.blockSound3Entry.OnChanged = func(s string) { e.markDirty() }
	e.blockSoundsGroup = widget.NewForm(widget.NewFormItem("Block 1", e.blockSound1Entry), widget.NewFormItem("Block 2", e.blockSound2Entry), widget.NewFormItem("Block 3", e.blockSound3Entry))

	e.bounceSound1Entry = widget.NewEntry()
	e.bounceSound2Entry = widget.NewEntry()
	e.bounceSound3Entry = widget.NewEntry()
	e.bounceSound1Entry.OnChanged = func(s string) { e.markDirty() }
	e.bounceSound2Entry.OnChanged = func(s string) { e.markDirty() }
	e.bounceSound3Entry.OnChanged = func(s string) { e.markDirty() }
	e.bounceSoundsGroup = widget.NewForm(widget.NewFormItem("Bounce 1", e.bounceSound1Entry), widget.NewFormItem("Bounce 2", e.bounceSound2Entry), widget.NewFormItem("Bounce 3", e.bounceSound3Entry))

	e.styleEntry = widget.NewEntry()
	e.singleBladeStyleEntry = widget.NewEntry()
	e.maxChainEntry = widget.NewEntry()
	e.lockBonusEntry = widget.NewEntry()
	e.parryBonusEntry = widget.NewEntry()
	e.breakParryEntry = widget.NewEntry()
	e.disarmBonusEntry = widget.NewEntry()
	e.styleEntry.OnChanged = func(s string) { e.markDirty() }
	e.singleBladeStyleEntry.OnChanged = func(s string) { e.markDirty() }
	e.maxChainEntry.OnChanged = func(s string) { e.markDirty() }
	e.lockBonusEntry.OnChanged = func(s string) { e.markDirty() }
	e.parryBonusEntry.OnChanged = func(s string) { e.markDirty() }
	e.breakParryEntry.OnChanged = func(s string) { e.markDirty() }
	e.disarmBonusEntry.OnChanged = func(s string) { e.markDirty() }
	combatForm := widget.NewForm(widget.NewFormItem("Saber Style", e.styleEntry), widget.NewFormItem("Single Blade Style", e.singleBladeStyleEntry), widget.NewFormItem("Max Chain", e.maxChainEntry), widget.NewFormItem("Lock Bonus", e.lockBonusEntry), widget.NewFormItem("Parry Bonus", e.parryBonusEntry), widget.NewFormItem("Break Parry Bonus", e.breakParryEntry), widget.NewFormItem("Disarm Bonus", e.disarmBonusEntry))

	e.moveSpeedEntry = widget.NewEntry()
	e.animSpeedEntry = widget.NewEntry()
	e.damageScaleEntry = widget.NewEntry()
	e.knockbackEntry = widget.NewEntry()
	e.moveSpeedEntry.OnChanged = func(s string) { e.markDirty() }
	e.animSpeedEntry.OnChanged = func(s string) { e.markDirty() }
	e.damageScaleEntry.OnChanged = func(s string) { e.markDirty() }
	e.knockbackEntry.OnChanged = func(s string) { e.markDirty() }
	speedDamageForm := widget.NewForm(widget.NewFormItem("Move Speed Scale", e.moveSpeedEntry), widget.NewFormItem("Anim Speed Scale", e.animSpeedEntry), widget.NewFormItem("Damage Scale", e.damageScaleEntry), widget.NewFormItem("Knockback Scale", e.knockbackEntry))

	e.trailStyleEntry = widget.NewEntry()
	e.blockEffectEntry = widget.NewEntry()
	e.hitPersonEffectEntry = widget.NewEntry()
	e.bladeEffectEntry = widget.NewEntry()
	e.hitOtherEffectEntry = widget.NewEntry()
	e.g2MarksShaderEntry = widget.NewEntry()
	e.g2WeaponMarkShaderEntry = widget.NewEntry()
	e.trailStyleEntry.OnChanged = func(s string) { e.markDirty() }
	e.blockEffectEntry.OnChanged = func(s string) { e.markDirty() }
	e.hitPersonEffectEntry.OnChanged = func(s string) { e.markDirty() }
	e.bladeEffectEntry.OnChanged = func(s string) { e.markDirty() }
	e.hitOtherEffectEntry.OnChanged = func(s string) { e.markDirty() }
	e.g2MarksShaderEntry.OnChanged = func(s string) { e.markDirty() }
	e.g2WeaponMarkShaderEntry.OnChanged = func(s string) { e.markDirty() }
	effectsForm := widget.NewForm(widget.NewFormItem("Trail Style", e.trailStyleEntry), widget.NewFormItem("Block Effect", e.blockEffectEntry), widget.NewFormItem("Hit Person Effect", e.hitPersonEffectEntry), widget.NewFormItem("Hit Other Effect", e.hitOtherEffectEntry), widget.NewFormItem("Blade Effect", e.bladeEffectEntry), widget.NewFormItem("G2 Marks Shader", e.g2MarksShaderEntry), widget.NewFormItem("G2 Weapon Mark Shader", e.g2WeaponMarkShaderEntry))

	e.noWallMarksCheck = widget.NewCheck("No Wall Marks", func(b bool) { e.saber.NoWallMarks = b; e.markDirty() })
	e.noDlightCheck = widget.NewCheck("No Dynamic Light", func(b bool) { e.saber.NoDlight = b; e.markDirty() })
	e.noBladeCheck = widget.NewCheck("No Blade", func(b bool) { e.saber.NoBlade = b; e.markDirty() })
	e.noClashFlareCheck = widget.NewCheck("No Clash Flare", func(b bool) { e.saber.NoClashFlare = b; e.markDirty() })
	e.noDismembermentCheck = widget.NewCheck("No Dismemberment", func(b bool) { e.saber.NoDismemberment = b; e.markDirty() })
	e.noIdleEffectCheck = widget.NewCheck("No Idle Effect", func(b bool) { e.saber.NoIdleEffect = b; e.markDirty() })
	e.alwaysBlockCheck = widget.NewCheck("Always Block", func(b bool) { e.saber.AlwaysBlock = b; e.markDirty() })
	e.noManualDeactivateCheck = widget.NewCheck("No Manual Deactivate", func(b bool) { e.saber.NoManualDeactivate = b; e.markDirty() })
	e.transitionDamageCheck = widget.NewCheck("Transition Damage", func(b bool) { e.saber.TransitionDamage = b; e.markDirty() })
	e.notInOpenCheck = widget.NewCheck("Not In Open", func(b bool) { e.saber.NotInOpen = b; e.markDirty() })
	e.notInMPCheck = widget.NewCheck("Not In MP", func(b bool) { e.saber.NotInMP = b; e.markDirty() })
	e.noCartwheelsCheck = widget.NewCheck("No Cartwheels", func(b bool) { e.saber.NoCartwheels = b; e.markDirty() })
	e.throwableCheck = widget.NewCheck("Throwable", func(b bool) { e.saber.Throwable = b; e.markDirty() })
	e.disarmableCheck = widget.NewCheck("Disarmable", func(b bool) { e.saber.Disarmable = b; e.markDirty() })
	e.blasterBlockingCheck = widget.NewCheck("Blaster Blocking", func(b bool) { e.saber.BlasterBlocking = b; e.markDirty() })
	e.onInWaterCheck = widget.NewCheck("On In Water", func(b bool) { e.saber.OnInWater = b; e.markDirty() })
	e.bounceOnWallsCheck = widget.NewCheck("Bounce On Walls", func(b bool) { e.saber.BounceOnWalls = b; e.markDirty() })
	e.twoHandedCheck = widget.NewCheck("Two Handed", func(b bool) { e.saber.TwoHanded = b; e.markDirty() })
	e.useGoreConfigCheck = widget.NewCheck("Use Gore Config", func(b bool) { e.saber.UseGoreConfig = b; e.markDirty() })
	e.useGoreConfig2Check = widget.NewCheck("Use Gore Config 2", func(b bool) { e.saber.UseGoreConfig2 = b; e.markDirty() })
	e.noDismemberment2Check = widget.NewCheck("No Dismemberment 2", func(b bool) { e.saber.NoDismemberment2 = b; e.markDirty() })
	e.noBladeEffectsCheck = widget.NewCheck("No Blade Effects", func(b bool) { e.saber.NoBladeEffects = b; e.markDirty() })
	e.noBladeEffects2Check = widget.NewCheck("No Blade Effects 2", func(b bool) { e.saber.NoBladeEffects2 = b; e.markDirty() })

	behaviorFlagsContainer := container.NewVBox(
		e.noWallMarksCheck, e.noDlightCheck, e.noBladeCheck, e.noClashFlareCheck, e.noDismembermentCheck, e.noIdleEffectCheck, e.alwaysBlockCheck, e.noManualDeactivateCheck, e.transitionDamageCheck,
		e.notInOpenCheck, e.notInMPCheck, e.noCartwheelsCheck, e.throwableCheck, e.disarmableCheck, e.blasterBlockingCheck, e.onInWaterCheck, e.bounceOnWallsCheck, e.twoHandedCheck,
		e.useGoreConfigCheck, e.useGoreConfig2Check, e.noDismemberment2Check, e.noBladeEffectsCheck, e.noBladeEffects2Check,
	)

	e.slapAnimEntry = widget.NewEntry()
	e.readyAnimEntry = widget.NewEntry()
	e.jumpAtkUpMoveEntry = widget.NewEntry()
	e.jumpAtkFwdMoveEntry = widget.NewEntry()
	e.lungeAtkMoveEntry = widget.NewEntry()
	e.slapAnimEntry.OnChanged = func(s string) { e.markDirty() }
	e.readyAnimEntry.OnChanged = func(s string) { e.markDirty() }
	e.jumpAtkUpMoveEntry.OnChanged = func(s string) { e.markDirty() }
	e.jumpAtkFwdMoveEntry.OnChanged = func(s string) { e.markDirty() }
	e.lungeAtkMoveEntry.OnChanged = func(s string) { e.markDirty() }
	animForm := widget.NewForm(widget.NewFormItem("Slap Animation", e.slapAnimEntry), widget.NewFormItem("Ready Animation", e.readyAnimEntry), widget.NewFormItem("Jump Attack Up", e.jumpAtkUpMoveEntry), widget.NewFormItem("Jump Attack Forward", e.jumpAtkFwdMoveEntry), widget.NewFormItem("Lunge Attack", e.lungeAtkMoveEntry))

	// Source View
	e.sourceView = widget.NewRichTextFromMarkdown("Loading...")
	sourceTab := container.NewMax(container.NewScroll(e.sourceView))

	tabs := container.NewAppTabs(
		container.NewTabItem("Identity", container.NewVScroll(identityForm)),
		container.NewTabItem("Blades", container.NewVScroll(bladeGrid)),
		container.NewTabItem("Sounds", container.NewVScroll(container.NewVBox(soundsForm, widget.NewCard("Swing", "", e.swingSoundsGroup), widget.NewCard("Fall", "", e.fallSoundsGroup), widget.NewCard("Hit", "", e.hitSoundsGroup), widget.NewCard("Block", "", e.blockSoundsGroup), widget.NewCard("Bounce", "", e.bounceSoundsGroup)))),
		container.NewTabItem("Combat", container.NewVScroll(container.NewVBox(combatForm, speedDamageForm, animForm))),
		container.NewTabItem("Flags", container.NewVScroll(behaviorFlagsContainer)),
		container.NewTabItem("Effects", container.NewVScroll(effectsForm)),
		container.NewTabItem("Source", sourceTab),
	)

	tabs.OnSelected = func(tab *container.TabItem) {
		if tab.Text == "Source" {
			e.updateSourceView()
		}
	}

	e.container = container.NewMax(tabs)
}

func (e *SABEditor) updateSourceView() {
	e.updateSaberFromUI()
	content, err := parsers.GenerateSAB(e.saber)
	if err != nil {
		e.sourceView.ParseMarkdown("Error: " + err.Error())
		return
	}
	highlighter := NewSyntaxHighlighter()
	e.sourceView.Segments = highlighter.Highlight(content).Segments
	e.sourceView.Refresh()
}

func (e *SABEditor) updateBladePreview(colorName string) {
	switch colorName {
	case "red":
		e.bladePreview.FillColor = color.RGBA{R: 255, A: 255}
	case "orange":
		e.bladePreview.FillColor = color.RGBA{R: 255, G: 165, A: 255}
	case "yellow":
		e.bladePreview.FillColor = color.RGBA{R: 255, G: 255, A: 255}
	case "green":
		e.bladePreview.FillColor = color.RGBA{G: 255, A: 255}
	case "blue":
		e.bladePreview.FillColor = color.RGBA{B: 255, A: 255}
	case "purple":
		e.bladePreview.FillColor = color.RGBA{R: 128, B: 128, A: 255}
	case "silver":
		e.bladePreview.FillColor = color.RGBA{R: 192, G: 192, B: 192, A: 255}
	default:
		e.bladePreview.FillColor = color.RGBA{A: 255}
	}
	e.bladePreview.Refresh()
}

func (e *SABEditor) GetContent() fyne.CanvasObject { return e.container }
func (e *SABEditor) GetCurrentPath() string        { return e.currentPath }
func (e *SABEditor) GetRecentFiles() []RecentFile  { return e.fileManager.GetRecentFiles() }

func (e *SABEditor) NewSaber() {
	e.saber = parsers.NewSaberData()
	e.currentPath = ""
	e.updateUI()
}

func (e *SABEditor) LoadFile(path string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	content, err := os.ReadFile(path)
	if err != nil {
		e.lastError = fmt.Sprintf("Failed to read file: %v", err)
		return err
	}

	saber, err := parsers.ParseSAB(string(content))
	if err != nil {
		return err
	}

	e.saber = saber
	e.currentPath = path
	e.updateUI()
	if e.fileManager != nil {
		e.fileManager.AddRecentFile(path)
	}
	e.lastError = ""
	e.MarkClean() // Freshly loaded file has no unsaved changes
	return nil
}

func (e *SABEditor) SaveToWriter(w io.Writer) error {
	e.updateSaberFromUI()
	content, err := parsers.GenerateSAB(e.saber)
	if err != nil {
		return err
	}
	_, err = w.Write([]byte(content))
	return err
}

func (e *SABEditor) SaveFile(path string) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()

	if err := e.SaveToWriter(file); err != nil {
		return err
	}

	e.currentPath = path
	e.lastError = ""
	if e.fileManager != nil {
		e.fileManager.AddRecentFile(path)
	}
	e.MarkClean() // File saved, no unsaved changes
	return nil
}

func (e *SABEditor) SetCurrentPath(path string) {
	e.currentPath = path
}

func (e *SABEditor) WriteContent(w io.Writer) {
	e.SaveToWriter(w)
}

func (e *SABEditor) Validate() []string {
	e.updateSaberFromUI()
	// Note: Validator will need update to support parsers.SaberData too
	return []string{}
}

func (e *SABEditor) ExportJSON(path string) error {
	e.updateSaberFromUI()
	data, err := json.MarshalIndent(e.saber, "", "  ")
	if err != nil {
		e.lastError = fmt.Sprintf("Failed to marshal JSON: %v", err)
		return err
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		e.lastError = fmt.Sprintf("Failed to write JSON: %v", err)
		return err
	}
	e.lastError = ""
	return nil
}

func (e *SABEditor) ImportJSON(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		e.lastError = fmt.Sprintf("Failed to read JSON: %v", err)
		return err
	}
	saber := parsers.NewSaberData()
	if err := json.Unmarshal(data, saber); err != nil {
		e.lastError = fmt.Sprintf("Failed to parse JSON: %v", err)
		return err
	}
	e.saber = saber
	e.updateUI()
	e.lastError = ""
	return nil
}

func (e *SABEditor) LoadTemplate(template string) {
	e.saber = parsers.NewSaberData()
	switch template {
	case "single":
		e.saber.SaberType = "SABER_SINGLE"
	case "staff":
		e.saber.SaberType = "SABER_STAFF"
		e.saber.NumBlades = 2
		e.saber.Blades = append(e.saber.Blades, parsers.BladeInfo{Color: "blue", Length: 32.0, Radius: 3.0})
	case "darksaber":
		e.saber.Name = "darksaber"
		e.saber.Blades[0].Color = "silver"
		e.saber.Blades[0].Length = 28.0
		e.saber.Blades[0].Radius = 2.5
		e.saber.SaberFlagMap["darksaber"] = true
	}
	e.updateUI()
}

func (e *SABEditor) GetLastError() string { return e.lastError }

func (e *SABEditor) updateUI() {
	e.nameEntry.SetText(e.saber.Name)
	e.fullNameEntry.SetText(e.saber.FullName)
	e.typeSelect.SetSelected(e.saber.SaberType)
	e.modelEntry.SetText(e.saber.SaberModel)
	e.customSkinEntry.SetText(e.saber.CustomSkin)
	e.numBladesEntry.SetText(strconv.Itoa(e.saber.NumBlades))
	if len(e.saber.Blades) > 0 {
		e.blade1ColorSelect.SetSelected(e.saber.Blades[0].Color)
		e.blade1LengthEntry.SetText(fmt.Sprintf("%.1f", e.saber.Blades[0].Length))
		e.blade1RadiusEntry.SetText(fmt.Sprintf("%.1f", e.saber.Blades[0].Radius))
	}
	e.soundOnEntry.SetText(e.saber.SoundOn)
	e.soundOffEntry.SetText(e.saber.SoundOff)
	e.soundLoopEntry.SetText(e.saber.SoundLoop)
	e.spinSoundEntry.SetText(e.saber.SpinSound)
	e.swingSound1Entry.SetText(e.saber.SwingSound1)
	e.swingSound2Entry.SetText(e.saber.SwingSound2)
	e.swingSound3Entry.SetText(e.saber.SwingSound3)
	e.fallSound1Entry.SetText(e.saber.FallSound1)
	e.fallSound2Entry.SetText(e.saber.FallSound2)
	e.fallSound3Entry.SetText(e.saber.FallSound3)
	e.hitSound1Entry.SetText(e.saber.HitSound1)
	e.hitSound2Entry.SetText(e.saber.HitSound2)
	e.hitSound3Entry.SetText(e.saber.HitSound3)
	e.blockSound1Entry.SetText(e.saber.BlockSound1)
	e.blockSound2Entry.SetText(e.saber.BlockSound2)
	e.blockSound3Entry.SetText(e.saber.BlockSound3)
	e.bounceSound1Entry.SetText(e.saber.BounceSound1)
	e.bounceSound2Entry.SetText(e.saber.BounceSound2)
	e.bounceSound3Entry.SetText(e.saber.BounceSound3)
	e.styleEntry.SetText(e.saber.SaberStyle)
	e.singleBladeStyleEntry.SetText(e.saber.SingleBladeStyle)
	e.maxChainEntry.SetText(strconv.Itoa(e.saber.MaxChain))
	e.lockBonusEntry.SetText(strconv.Itoa(e.saber.LockBonus))
	e.parryBonusEntry.SetText(strconv.Itoa(e.saber.ParryBonus))
	e.breakParryEntry.SetText(strconv.Itoa(e.saber.BreakParryBonus))
	e.disarmBonusEntry.SetText(strconv.Itoa(e.saber.DisarmBonus))
	e.moveSpeedEntry.SetText(fmt.Sprintf("%.2f", e.saber.MoveSpeedScale))
	e.animSpeedEntry.SetText(fmt.Sprintf("%.2f", e.saber.AnimSpeedScale))
	e.damageScaleEntry.SetText(fmt.Sprintf("%.2f", e.saber.DamageScale))
	e.knockbackEntry.SetText(fmt.Sprintf("%.2f", e.saber.KnockbackScale))
	e.trailStyleEntry.SetText(strconv.Itoa(e.saber.TrailStyle))
	e.blockEffectEntry.SetText(e.saber.BlockEffect)
	e.hitPersonEffectEntry.SetText(e.saber.HitPersonEffect)
	e.bladeEffectEntry.SetText(e.saber.BladeEffect)
	e.hitOtherEffectEntry.SetText(e.saber.HitOtherEffect)
	e.g2MarksShaderEntry.SetText(e.saber.G2MarksShader)
	e.g2WeaponMarkShaderEntry.SetText(e.saber.G2WeaponMarkShader)
	e.noWallMarksCheck.SetChecked(e.saber.NoWallMarks)
	e.noDlightCheck.SetChecked(e.saber.NoDlight)
	e.noBladeCheck.SetChecked(e.saber.NoBlade)
	e.noClashFlareCheck.SetChecked(e.saber.NoClashFlare)
	e.noDismembermentCheck.SetChecked(e.saber.NoDismemberment)
	e.noIdleEffectCheck.SetChecked(e.saber.NoIdleEffect)
	e.alwaysBlockCheck.SetChecked(e.saber.AlwaysBlock)
	e.noManualDeactivateCheck.SetChecked(e.saber.NoManualDeactivate)
	e.transitionDamageCheck.SetChecked(e.saber.TransitionDamage)
	e.notInOpenCheck.SetChecked(e.saber.NotInOpen)
	e.notInMPCheck.SetChecked(e.saber.NotInMP)
	e.noCartwheelsCheck.SetChecked(e.saber.NoCartwheels)
	e.throwableCheck.SetChecked(e.saber.Throwable)
	e.disarmableCheck.SetChecked(e.saber.Disarmable)
	e.blasterBlockingCheck.SetChecked(e.saber.BlasterBlocking)
	e.onInWaterCheck.SetChecked(e.saber.OnInWater)
	e.bounceOnWallsCheck.SetChecked(e.saber.BounceOnWalls)
	e.twoHandedCheck.SetChecked(e.saber.TwoHanded)
	e.useGoreConfigCheck.SetChecked(e.saber.UseGoreConfig)
	e.useGoreConfig2Check.SetChecked(e.saber.UseGoreConfig2)
	e.noDismemberment2Check.SetChecked(e.saber.NoDismemberment2)
	e.noBladeEffectsCheck.SetChecked(e.saber.NoBladeEffects)
	e.noBladeEffects2Check.SetChecked(e.saber.NoBladeEffects2)
}

func (e *SABEditor) updateSaberFromUI() {
	e.saber.Name = e.nameEntry.Text
	e.saber.FullName = e.fullNameEntry.Text
	e.saber.SaberType = e.typeSelect.Selected
	e.saber.SaberModel = e.modelEntry.Text
	e.saber.CustomSkin = e.customSkinEntry.Text
	e.saber.NumBlades, _ = strconv.Atoi(e.numBladesEntry.Text)
	if len(e.saber.Blades) > 0 {
		e.saber.Blades[0].Color = e.blade1ColorSelect.Selected
		e.saber.Blades[0].Length, _ = strconv.ParseFloat(e.blade1LengthEntry.Text, 64)
		e.saber.Blades[0].Radius, _ = strconv.ParseFloat(e.blade1RadiusEntry.Text, 64)
	}
	e.saber.SoundOn = e.soundOnEntry.Text
	e.saber.SoundOff = e.soundOffEntry.Text
	e.saber.SoundLoop = e.soundLoopEntry.Text
	e.saber.SpinSound = e.spinSoundEntry.Text
	e.saber.SwingSound1 = e.swingSound1Entry.Text
	e.saber.SwingSound2 = e.swingSound2Entry.Text
	e.saber.SwingSound3 = e.swingSound3Entry.Text
	e.saber.FallSound1 = e.fallSound1Entry.Text
	e.saber.FallSound2 = e.fallSound2Entry.Text
	e.saber.FallSound3 = e.fallSound3Entry.Text
	e.saber.HitSound1 = e.hitSound1Entry.Text
	e.saber.HitSound2 = e.hitSound2Entry.Text
	e.saber.HitSound3 = e.hitSound3Entry.Text
	e.saber.BlockSound1 = e.blockSound1Entry.Text
	e.saber.BlockSound2 = e.blockSound2Entry.Text
	e.saber.BlockSound3 = e.blockSound3Entry.Text
	e.saber.BounceSound1 = e.bounceSound1Entry.Text
	e.saber.BounceSound2 = e.bounceSound2Entry.Text
	e.saber.BounceSound3 = e.bounceSound3Entry.Text
	e.saber.SaberStyle = e.styleEntry.Text
	e.saber.SingleBladeStyle = e.singleBladeStyleEntry.Text
	e.saber.MaxChain, _ = strconv.Atoi(e.maxChainEntry.Text)
	e.saber.LockBonus, _ = strconv.Atoi(e.lockBonusEntry.Text)
	e.saber.ParryBonus, _ = strconv.Atoi(e.parryBonusEntry.Text)
	e.saber.BreakParryBonus, _ = strconv.Atoi(e.breakParryEntry.Text)
	e.saber.DisarmBonus, _ = strconv.Atoi(e.disarmBonusEntry.Text)
	e.saber.MoveSpeedScale, _ = strconv.ParseFloat(e.moveSpeedEntry.Text, 64)
	e.saber.AnimSpeedScale, _ = strconv.ParseFloat(e.animSpeedEntry.Text, 64)
	e.saber.DamageScale, _ = strconv.ParseFloat(e.damageScaleEntry.Text, 64)
	e.saber.KnockbackScale, _ = strconv.ParseFloat(e.knockbackEntry.Text, 64)
	e.saber.TrailStyle, _ = strconv.Atoi(e.trailStyleEntry.Text)
	e.saber.BlockEffect = e.blockEffectEntry.Text
	e.saber.HitPersonEffect = e.hitPersonEffectEntry.Text
	e.saber.BladeEffect = e.bladeEffectEntry.Text
	e.saber.HitOtherEffect = e.hitOtherEffectEntry.Text
	e.saber.G2MarksShader = e.g2MarksShaderEntry.Text
	e.saber.G2WeaponMarkShader = e.g2WeaponMarkShaderEntry.Text
	e.saber.NoWallMarks = e.noWallMarksCheck.Checked
	e.saber.NoDlight = e.noDlightCheck.Checked
	e.saber.NoBlade = e.noBladeCheck.Checked
	e.saber.NoClashFlare = e.noClashFlareCheck.Checked
	e.saber.NoDismemberment = e.noDismembermentCheck.Checked
	e.saber.NoIdleEffect = e.noIdleEffectCheck.Checked
	e.saber.AlwaysBlock = e.alwaysBlockCheck.Checked
	e.saber.NoManualDeactivate = e.noManualDeactivateCheck.Checked
	e.saber.TransitionDamage = e.transitionDamageCheck.Checked
	e.saber.NotInOpen = e.notInOpenCheck.Checked
	e.saber.NotInMP = e.notInMPCheck.Checked
	e.saber.NoCartwheels = e.noCartwheelsCheck.Checked
	e.saber.Throwable = e.throwableCheck.Checked
	e.saber.Disarmable = e.disarmableCheck.Checked
	e.saber.BlasterBlocking = e.blasterBlockingCheck.Checked
	e.saber.OnInWater = e.onInWaterCheck.Checked
	e.saber.BounceOnWalls = e.bounceOnWallsCheck.Checked
	e.saber.TwoHanded = e.twoHandedCheck.Checked
	e.saber.UseGoreConfig = e.useGoreConfigCheck.Checked
	e.saber.UseGoreConfig2 = e.useGoreConfig2Check.Checked
	e.saber.NoDismemberment2 = e.noDismemberment2Check.Checked
	e.saber.NoBladeEffects = e.noBladeEffectsCheck.Checked
	e.saber.NoBladeEffects2 = e.noBladeEffects2Check.Checked
}
