package main

import (
	"fmt"
	"image/color"
	"path/filepath"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// AssetHealthStatus represents the validation state of a game asset reference.
type AssetHealthStatus int

const (
	AssetHealthUnknown AssetHealthStatus = iota
	AssetHealthVerified
	AssetHealthMissing
)

// AssetHealthResult contains the status and diagnostic message.
type AssetHealthResult struct {
	Status      AssetHealthStatus
	Message     string
	Suggestions []string
}

// CheckModelAssetHealth verifies if a player model and skin exist in the VFS.
func CheckModelAssetHealth(model, skin string, vfs *VirtualFileSystem) AssetHealthResult {
	if vfs == nil || model == "" {
		return AssetHealthResult{Status: AssetHealthUnknown, Message: "No VFS index loaded"}
	}

	modelLower := strings.ToLower(strings.TrimSpace(model))
	skinLower := strings.ToLower(strings.TrimSpace(skin))
	if skinLower == "" {
		skinLower = "default"
	}

	// 1. Check if model folder or .glm exists
	glmPath := fmt.Sprintf("models/players/%s/model.glm", modelLower)
	skinPath1 := fmt.Sprintf("models/players/%s/model_%s.skin", modelLower, skinLower)
	skinPath2 := fmt.Sprintf("models/players/%s/%s.skin", modelLower, skinLower)

	vfs.mu.RLock()
	defer vfs.mu.RUnlock()

	_, hasGlm := vfs.Index[glmPath]
	_, hasSkin1 := vfs.Index[skinPath1]
	_, hasSkin2 := vfs.Index[skinPath2]

	if !hasGlm {
		// Model folder missing
		suggestions := findModelSuggestions(modelLower, vfs)
		return AssetHealthResult{
			Status:      AssetHealthMissing,
			Message:     fmt.Sprintf("Model '%s' not found in any PK3 archive.", model),
			Suggestions: suggestions,
		}
	}

	if !hasSkin1 && !hasSkin2 && skinLower != "default" {
		return AssetHealthResult{
			Status:  AssetHealthMissing,
			Message: fmt.Sprintf("Skin '%s' not found for model '%s'.", skin, model),
		}
	}

	return AssetHealthResult{
		Status:  AssetHealthVerified,
		Message: fmt.Sprintf("Model '%s' and skin '%s' verified in PK3 archives.", model, skin),
	}
}

// CheckGenericAssetHealth verifies if a generic texture, sound, or file exists in the VFS.
func CheckGenericAssetHealth(path string, vfs *VirtualFileSystem) AssetHealthResult {
	if vfs == nil || strings.TrimSpace(path) == "" {
		return AssetHealthResult{Status: AssetHealthUnknown, Message: "Empty path or no VFS"}
	}

	norm := strings.ToLower(strings.TrimSpace(strings.ReplaceAll(path, "\\", "/")))

	vfs.mu.RLock()
	defer vfs.mu.RUnlock()

	// Direct match
	if _, ok := vfs.Index[norm]; ok {
		return AssetHealthResult{Status: AssetHealthVerified, Message: "File verified in PK3 archives."}
	}

	// Extension probes (.tga, .jpg, .png, .wav, .mp3)
	if filepath.Ext(norm) == "" {
		for _, ext := range []string{".tga", ".jpg", ".png", ".wav", ".mp3", ".efx"} {
			if _, ok := vfs.Index[norm+ext]; ok {
				return AssetHealthResult{Status: AssetHealthVerified, Message: "File verified in PK3 archives."}
			}
		}
	}

	return AssetHealthResult{
		Status:  AssetHealthMissing,
		Message: fmt.Sprintf("Asset '%s' not found in loaded packages.", path),
	}
}

func findModelSuggestions(target string, vfs *VirtualFileSystem) []string {
	var suggestions []string
	seen := make(map[string]bool)

	for k := range vfs.Index {
		if strings.HasPrefix(k, "models/players/") {
			rel := strings.TrimPrefix(k, "models/players/")
			parts := strings.Split(rel, "/")
			if len(parts) > 1 {
				m := parts[0]
				if !seen[m] && (strings.Contains(m, target) || strings.Contains(target, m)) {
					seen[m] = true
					suggestions = append(suggestions, m)
					if len(suggestions) >= 3 {
						break
					}
				}
			}
		}
	}
	return suggestions
}

// BuildAssetHealthBadge creates a small reactive status badge for inline UI placement.
func BuildAssetHealthBadge(result AssetHealthResult) fyne.CanvasObject {
	var icon fyne.Resource
	var iconColor color.RGBA

	switch result.Status {
	case AssetHealthVerified:
		icon = theme.ConfirmIcon()
		iconColor = color.RGBA{R: 50, G: 200, B: 80, A: 255}
	case AssetHealthMissing:
		icon = theme.WarningIcon()
		iconColor = color.RGBA{R: 240, G: 80, B: 60, A: 255}
	default:
		icon = theme.QuestionIcon()
		iconColor = color.RGBA{R: 150, G: 150, B: 160, A: 255}
	}

	img := widget.NewIcon(icon)
	bg := canvas.NewRectangle(color.RGBA{R: iconColor.R / 5, G: iconColor.G / 5, B: iconColor.B / 5, A: 200})
	bg.CornerRadius = 3

	tooltip := result.Message
	if len(result.Suggestions) > 0 {
		tooltip += " Suggestions: " + strings.Join(result.Suggestions, ", ")
	}

	badgeStack := container.NewStack(bg, container.NewPadded(img))
	return container.NewGridWrap(fyne.NewSize(24, 24), badgeStack)
}
