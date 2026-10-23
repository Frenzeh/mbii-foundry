package main

import (
	"path"
	"sort"
	"strings"
	"unicode"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

type sourceAssetKind uint8

const (
	sourceAssetUnknown sourceAssetKind = iota
	sourceAssetPlayerModel
	sourceAssetPlayerSkin
	sourceAssetModel
	sourceAssetSkin
	sourceAssetImage
	sourceAssetSound
	sourceAssetSoundset
	sourceAssetEffect
	sourceAssetTeam
)

func (a *App) sourceAssetSuggestions(format SourceFormat, field, prefix string, limit int) []string {
	if a == nil || a.assetBrowser == nil || a.assetBrowser.vfs == nil {
		return nil
	}
	kind := sourceAssetKindForField(format, field)
	if kind == sourceAssetUnknown {
		return nil
	}
	if limit <= 0 {
		limit = 50
	}
	// Filter normalized references while iterating, then sort only matches;
	// Complete is synchronous and the merged index can be very large.
	prefix = strings.ToLower(strings.TrimSpace(prefix))
	seen := make(map[string]bool)
	values := make([]string, 0, limit)
	collect := func(candidate string, declaredShader bool) {
		value, ok := sourceAssetReference(format, field, candidate, declaredShader)
		if !ok {
			return
		}
		key := strings.ToLower(value)
		if seen[key] || prefix != "" && !strings.HasPrefix(key, prefix) {
			return
		}
		seen[key] = true
		values = append(values, value)
	}
	vfs := a.assetBrowser.vfs
	vfs.mu.RLock()
	for _, source := range vfs.Index {
		collect(source.Path, false)
	}
	vfs.mu.RUnlock()
	if kind == sourceAssetImage {
		a.assetBrowser.shaderResolver.visitDeclared(func(name string) { collect(name, true) })
	}
	sort.Slice(values, func(i, j int) bool { return strings.ToLower(values[i]) < strings.ToLower(values[j]) })
	if len(values) > limit {
		values = values[:limit]
	}
	return values
}

func (a *App) showSourceAssetPicker(code *SourceCodeEditor, format SourceFormat) {
	if a == nil || code == nil || a.fyneApp == nil || a.assetBrowser == nil || a.assetBrowser.vfs == nil {
		return
	}
	target, ok := captureSourceAssetTarget(code, format)
	if !ok {
		if a.mainWindow != nil {
			dialog.ShowInformation("Insert Asset", "Place the caret in an asset field value first.", a.mainWindow)
		}
		return
	}

	pickerWindow := a.fyneApp.NewWindow("Insert Asset")
	pickerBrowser := sourceAssetBrowserView(a.assetBrowser)
	picker := NewVirtualFilePicker(pickerWindow, pickerBrowser)
	picker.SetInitialPath(sourceAssetInitialPath(target.kind))

	if target.kind == sourceAssetImage {
		addSourceShaderChooser(pickerWindow, pickerBrowser, target)
	}
	picker.Show(func(asset *AssetEntry) {
		if asset == nil || asset.IsDir || pickerBrowser.currentPK3 != "VFS" {
			return
		}
		reference, valid := sourceAssetReference(target.format, target.field, asset.Path, false)
		if !valid {
			if a.mainWindow != nil {
				dialog.ShowInformation("Unsupported Asset", "That file is not valid for "+target.field+".", a.mainWindow)
			}
			return
		}
		if target.kind == sourceAssetImage && a.mainWindow != nil {
			showSourceAssetPreview(a.mainWindow, pickerBrowser, reference, func(confirmed bool) {
				target.apply(reference, confirmed)
			})
			return
		}
		target.apply(reference, true)
	})
}

func sourceAssetBrowserView(existing *AssetBrowser) *AssetBrowser {
	viewMode := existing.viewMode
	if viewMode == "" {
		viewMode = ViewModeGrid
	}
	sortMode := existing.sortMode
	if sortMode == "" {
		sortMode = SortNameAsc
	}
	iconSize := existing.iconSize
	if iconSize <= 0 {
		iconSize = 100
	}
	view := &AssetBrowser{
		assets:         make(map[string]*AssetEntry),
		rootEntries:    []*AssetEntry{},
		viewMode:       viewMode,
		sortMode:       sortMode,
		iconSize:       iconSize,
		gamedataPath:   existing.gamedataPath,
		textAssetsPath: existing.textAssetsPath,
		vfs:            existing.vfs,
		shaderResolver: existing.shaderResolver,
		virtualOnly:    true,
	}
	view.createUI()
	return view
}

func addSourceShaderChooser(window fyne.Window, browser *AssetBrowser, target sourceAssetTarget) {
	if browser.shaderResolver == nil {
		return
	}
	names := browser.shaderResolver.Suggest("", 0)
	if len(names) == 0 {
		return
	}
	known := make(map[string]string, len(names))
	for _, name := range names {
		known[strings.ToLower(name)] = name
	}
	entry := widget.NewSelectEntry(names)
	entry.SetPlaceHolder("Declared shader name")
	preview := container.NewStack(widget.NewLabel("Choose a shader to preview"))
	selectedShader := func() (string, bool) {
		name, ok := known[strings.ToLower(strings.TrimSpace(entry.Text))]
		if !ok {
			return "", false
		}
		return sourceAssetReference(target.format, target.field, name, true)
	}
	previewButton := widget.NewButtonWithIcon("Preview", theme.VisibilityIcon(), func() {
		name, ok := selectedShader()
		if !ok {
			return
		}
		preview.Objects = []fyne.CanvasObject{sourceAssetPreviewObject(browser, name)}
		preview.Refresh()
	})
	insertButton := widget.NewButtonWithIcon("Insert Shader", theme.ConfirmIcon(), func() {
		name, ok := selectedShader()
		if !ok || !target.apply(name, true) {
			return
		}
		window.Close()
	})
	insertButton.Importance = widget.HighImportance
	shaderBar := container.NewBorder(
		widget.NewLabelWithStyle("Declared shader", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		nil, nil, container.NewHBox(previewButton, insertButton),
		container.NewBorder(nil, nil, nil, preview, entry),
	)
	window.SetContent(container.NewBorder(shaderBar, nil, nil, nil, window.Content()))
}

func showSourceAssetPreview(parent fyne.Window, browser *AssetBrowser, reference string, selected func(bool)) {
	dialog.ShowCustomConfirm(
		"Insert Asset", "Insert", "Cancel",
		container.NewBorder(widget.NewLabel(reference), nil, nil, nil, sourceAssetPreviewObject(browser, reference)),
		selected, parent,
	)
}

func sourceAssetPreviewObject(browser *AssetBrowser, reference string) fyne.CanvasObject {
	resource := browser.LoadIconResource(reference)
	if resource == nil {
		label := widget.NewLabel("No still-image preview is available for this reference.")
		label.Wrapping = fyne.TextWrapWord
		return label
	}
	image := canvas.NewImageFromResource(resource)
	image.FillMode = canvas.ImageFillContain
	image.SetMinSize(fyne.NewSize(256, 192))
	return image
}

type sourceAssetTarget struct {
	code     *SourceCodeEditor
	snapshot string
	format   SourceFormat
	field    string
	kind     sourceAssetKind
	replace  SourceRange
	inString bool
}

func captureSourceAssetTarget(code *SourceCodeEditor, format SourceFormat) (sourceAssetTarget, bool) {
	if code == nil || code.IsReadOnly() {
		return sourceAssetTarget{}, false
	}
	text := code.Text()
	context := SourceContextAt(text, code.CursorOffset(), format)
	kind := sourceAssetKindForField(format, context.Field)
	if context.AtKey || context.Field == "" || kind == sourceAssetUnknown {
		return sourceAssetTarget{}, false
	}
	start, end := code.Selection()
	if start < 0 {
		start = 0
	}
	if end > len(text) {
		end = len(text)
	}
	if start > end {
		start, end = end, start
	}
	// An empty quoted value has its closing quote after the replacement range.
	if start == end && context.Replace.Start == context.Replace.End && !context.InString {
		lineEnd := len(text)
		if offset := strings.IndexByte(text[context.Replace.End:], '\n'); offset >= 0 {
			lineEnd = context.Replace.End + offset
		}
		if strings.TrimSpace(text[context.Replace.End:lineEnd]) != "" {
			return sourceAssetTarget{}, false
		}
	}
	inString := context.InString && start >= context.Replace.Start && end <= context.Replace.End
	if start == end {
		start, end = context.Replace.Start, context.Replace.End
		inString = context.InString
	}
	return sourceAssetTarget{
		code: code, snapshot: text, format: format, field: context.Field, kind: kind,
		replace: SourceRange{Start: start, End: end}, inString: inString,
	}, true
}

func (target sourceAssetTarget) apply(reference string, confirmed bool) bool {
	if !confirmed || target.code == nil || target.code.IsReadOnly() || target.code.Text() != target.snapshot {
		return false
	}
	reference, ok := validVirtualAssetPath(reference)
	if !ok {
		return false
	}
	insert := reference
	if !target.inString {
		insert = `"` + reference + `"`
	}
	target.code.ReplaceRange(target.replace.Start, target.replace.End, insert)
	return true
}

func sourceAssetKindForField(format SourceFormat, field string) sourceAssetKind {
	lower := strings.ToLower(strings.TrimSpace(field))
	switch {
	case format == SourceFormatMBCH && (lower == "model" || strings.HasPrefix(lower, "model_")):
		return sourceAssetPlayerModel
	case format == SourceFormatMBCH && (lower == "skin" || strings.HasPrefix(lower, "skin_")):
		return sourceAssetPlayerSkin
	case format == SourceFormatMBCH && lower == "soundset":
		return sourceAssetSoundset
	case lower == "useteam":
		return sourceAssetTeam
	case strings.Contains(lower, "sound"):
		return sourceAssetSound
	case strings.Contains(lower, "effect"):
		return sourceAssetEffect
	case strings.Contains(lower, "shader") || strings.Contains(lower, "icon") || strings.Contains(lower, "mapgraphic") || strings.Contains(lower, "objgfx"):
		return sourceAssetImage
	case strings.Contains(lower, "model"):
		return sourceAssetModel
	case strings.Contains(lower, "skin"):
		return sourceAssetSkin
	default:
		return sourceAssetUnknown
	}
}

func sourceAssetPathFilter(kind sourceAssetKind) func(string) bool {
	return func(candidate string) bool {
		logical, ok := validVirtualAssetPath(candidate)
		if !ok {
			return false
		}
		lower := strings.ToLower(logical)
		ext := strings.ToLower(path.Ext(lower))
		switch kind {
		case sourceAssetPlayerModel:
			return strings.HasPrefix(lower, "models/players/") && path.Base(lower) == "model.glm"
		case sourceAssetPlayerSkin:
			base := path.Base(lower)
			return strings.HasPrefix(lower, "models/players/") && strings.HasPrefix(base, "model_") && ext == ".skin"
		case sourceAssetModel:
			return ext == ".glm" || ext == ".md3"
		case sourceAssetSkin:
			return ext == ".skin"
		case sourceAssetImage:
			return ext == ".tga" || ext == ".png" || ext == ".jpg" || ext == ".jpeg"
		case sourceAssetSound:
			return ext == ".wav" || ext == ".mp3" || ext == ".ogg"
		case sourceAssetSoundset:
			parts := strings.Split(lower, "/")
			return len(parts) >= 4 && parts[0] == "sound" && parts[1] == "chars" && parts[2] != "" && (ext == ".wav" || ext == ".mp3" || ext == ".ogg")
		case sourceAssetEffect:
			return ext == ".efx"
		case sourceAssetTeam:
			return ext == ".mbtc"
		default:
			return false
		}
	}
}

func sourceAssetReference(format SourceFormat, field, candidate string, declaredShader bool) (string, bool) {
	logical, ok := validVirtualAssetPath(candidate)
	if !ok {
		return "", false
	}
	kind := sourceAssetKindForField(format, field)
	if kind == sourceAssetUnknown {
		return "", false
	}
	if declaredShader {
		if kind != sourceAssetImage {
			return "", false
		}
		return logical, true
	}
	if !sourceAssetPathFilter(kind)(logical) {
		return "", false
	}
	switch kind {
	case sourceAssetPlayerModel:
		parts := strings.Split(logical, "/")
		if len(parts) != 4 {
			return "", false
		}
		return parts[2], true
	case sourceAssetPlayerSkin:
		parts := strings.Split(logical, "/")
		if len(parts) != 4 {
			return "", false
		}
		base := path.Base(logical)
		skin := base[len("model_") : len(base)-len(".skin")]
		return skin, skin != ""
	case sourceAssetSoundset:
		parts := strings.Split(logical, "/")
		if len(parts) < 4 {
			return "", false
		}
		return parts[2], true
	case sourceAssetImage:
		return strings.TrimSuffix(logical, path.Ext(logical)), true
	case sourceAssetTeam:
		return path.Base(logical), true
	default:
		return logical, true
	}
}

func validVirtualAssetPath(candidate string) (string, bool) {
	if candidate == "" || candidate != strings.TrimSpace(candidate) {
		return "", false
	}
	value := strings.ReplaceAll(candidate, "\\", "/")
	if strings.HasPrefix(value, "/") || len(value) >= 2 && value[1] == ':' {
		return "", false
	}
	for _, r := range value {
		if r == '"' || r == 0 || r == '\n' || r == '\r' || unicode.IsControl(r) {
			return "", false
		}
	}
	parts := strings.Split(value, "/")
	for _, part := range parts {
		if part == "" || part == "." || part == ".." {
			return "", false
		}
	}
	if len(parts) > 1 {
		switch strings.ToLower(parts[0]) {
		case "base", "mbii", "mbiitest", "mbiirelease":
			parts = parts[1:]
		}
	}
	value = path.Clean(strings.Join(parts, "/"))
	if value == "." || value == "" || strings.HasPrefix(value, "../") {
		return "", false
	}
	return value, true
}

func sourceAssetInitialPath(kind sourceAssetKind) string {
	switch kind {
	case sourceAssetPlayerModel, sourceAssetPlayerSkin:
		return "models/players"
	case sourceAssetModel, sourceAssetSkin:
		return "models"
	case sourceAssetImage:
		return "gfx"
	case sourceAssetSound:
		return "sound"
	case sourceAssetSoundset:
		return "sound/chars"
	case sourceAssetEffect:
		return "effects"
	case sourceAssetTeam:
		return "ext_data"
	default:
		return ""
	}
}
