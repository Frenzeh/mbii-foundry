package main

import (
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// AssetHealthStatus represents the validation state of a game asset reference.
type AssetHealthStatus int

const (
	AssetHealthUnknown AssetHealthStatus = iota
	AssetHealthVerified
	AssetHealthAbsent
	AssetHealthUnreadable
	AssetHealthFallback
)

// AssetHealthResult contains the status and diagnostic message.
type AssetHealthResult struct {
	Status        AssetHealthStatus
	FailureStage  string // e.g. "open", "read", "close", "decode"
	Message       string
	Suggestions   []string
	Provenance    string // e.g. "MBII/zz_MBModels.pk3"
	Fallback      string // e.g. "models/players/kyle/mb2_icon_default"
	PrimaryFailed string // e.g. path of the failed primary
	DecodeErr     error
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

	var provenance string
	if hasGlm {
		if src, ok := vfs.Index[glmPath]; ok {
			provenance = src.PK3Path
			if provenance == "" {
				provenance = src.FullPath
			}
		}
	}

	if !hasGlm {
		suggestions := findModelSuggestions(modelLower, vfs)
		return AssetHealthResult{
			Status:      AssetHealthAbsent,
			Message:     fmt.Sprintf("Model '%s' not found in any PK3 archive.", model),
			Suggestions: suggestions,
		}
	}

	if !hasSkin1 && !hasSkin2 {
		return AssetHealthResult{
			Status:     AssetHealthAbsent,
			Message:    fmt.Sprintf("Skin '%s' not found for model '%s'.", skin, model),
			Provenance: provenance,
		}
	}

	var skinProvenance string
	if hasSkin1 {
		if src, ok := vfs.Index[skinPath1]; ok {
			skinProvenance = src.PK3Path
			if skinProvenance == "" {
				skinProvenance = src.FullPath
			}
		}
	} else if hasSkin2 {
		if src, ok := vfs.Index[skinPath2]; ok {
			skinProvenance = src.PK3Path
			if skinProvenance == "" {
				skinProvenance = src.FullPath
			}
		}
	}
	if skinProvenance != "" && skinProvenance != provenance {
		provenance = fmt.Sprintf("%s (model) / %s (skin)", provenance, skinProvenance)
	}

	return AssetHealthResult{
		Status:     AssetHealthVerified,
		Message:    "Model and skin assets found in VFS.",
		Provenance: provenance,
	}
}

var testCheckGenericAssetHealthRCHook func(io.ReadCloser) io.ReadCloser

// CheckGenericAssetHealth verifies if a generic texture, sound, or file exists in the VFS.
func CheckGenericAssetHealth(path string, vfs *VirtualFileSystem) AssetHealthResult {
	if vfs == nil || strings.TrimSpace(path) == "" {
		return AssetHealthResult{Status: AssetHealthUnknown, Message: "Empty path or no VFS"}
	}

	norm := strings.ToLower(strings.TrimSpace(strings.ReplaceAll(path, "\\", "/")))

	vfs.mu.RLock()
	// Direct match
	src, ok := vfs.Index[norm]
	var matchedExt string
	var matchPath = norm
	if !ok && filepath.Ext(norm) == "" {
		for _, ext := range []string{".tga", ".jpg", ".png", ".wav", ".mp3", ".efx"} {
			if s, ok2 := vfs.Index[norm+ext]; ok2 {
				src = s
				ok = true
				matchedExt = ext
				matchPath = norm + ext
				break
			}
		}
	}
	var prov string
	if ok {
		prov = src.PK3Path
		if prov == "" {
			prov = src.FullPath
		}
	}
	vfs.mu.RUnlock()

	if !ok {
		return AssetHealthResult{
			Status:       AssetHealthAbsent,
			FailureStage: "missing",
			Message:      fmt.Sprintf("Asset '%s' not found in loaded packages.", path),
		}
	}

	res := AssetHealthResult{Status: AssetHealthVerified, Provenance: prov}
	if matchedExt == "" {
		res.Message = "File verified in PK3 archives."
	} else {
		res.Message = fmt.Sprintf("File verified as %s in PK3 archives.", matchedExt)
	}

	ext := filepath.Ext(matchPath)
	if ext == ".tga" || ext == ".jpg" || ext == ".png" {
		rc, err := vfs.ReadFile(matchPath)
		if err != nil {
			res.Status = AssetHealthUnreadable
			res.FailureStage = "open"
			res.DecodeErr = err
			res.Message = "File found but failed to open."
		} else {
			if testCheckGenericAssetHealthRCHook != nil {
				rc = testCheckGenericAssetHealthRCHook(rc)
			}
			data, readErr := readRasterBytes(rc)
			closeErr := rc.Close()

			if readErr != nil {
				res.Status = AssetHealthUnreadable
				res.FailureStage = "read"
				res.DecodeErr = readErr
				res.Message = "File found but failed to read."
			} else if closeErr != nil {
				res.Status = AssetHealthUnreadable
				res.FailureStage = "close"
				res.DecodeErr = closeErr
				res.Message = "File found but failed to close."
			} else {
				if _, decErr := decodeByExt(ext, data); decErr != nil {
					res.Status = AssetHealthUnreadable
					res.FailureStage = "decode"
					res.DecodeErr = decErr
					res.Message = "File found but failed to decode."
				}
			}
		}
	}

	return res
}

// CheckIconAssetHealth uses the IconResolver to find and decode the actual UI shader icon.
// A successful fallback keeps the selected asset's provenance while carrying the
// primary candidate's structured failure diagnostics.
func CheckIconAssetHealth(ir *IconResolver, model, skin, shader string) AssetHealthResult {
	if ir == nil || ir.vfs == nil {
		return AssetHealthResult{Status: AssetHealthUnknown, Message: "No IconResolver or VFS"}
	}
	candidates := ir.ResolveClassIconCandidates(model, skin, shader)
	if len(candidates) == 0 {
		return AssetHealthResult{Status: AssetHealthAbsent, FailureStage: "missing", Message: "No valid icon candidates resolved."}
	}

	primaryPath := candidates[0]
	primary := CheckGenericAssetHealth(primaryPath, ir.vfs)
	if primary.Status == AssetHealthVerified {
		return primary
	}

	for _, candidate := range candidates[1:] {
		resolved := CheckGenericAssetHealth(candidate, ir.vfs)
		if resolved.Status != AssetHealthVerified {
			continue
		}

		// Keep the winning fallback's provenance, but report why the
		// primary candidate was skipped. Never substitute the failing
		// fallback's path or error for the primary diagnostic.
		resolved.Status = AssetHealthFallback
		resolved.Fallback = candidate
		resolved.PrimaryFailed = primaryPath
		resolved.FailureStage = primary.FailureStage
		resolved.DecodeErr = primary.DecodeErr
		if primary.Status == AssetHealthAbsent {
			resolved.Message = fmt.Sprintf("Primary '%s' was absent; resolved via fallback.", primaryPath)
		} else {
			resolved.Message = fmt.Sprintf("Primary '%s' failed at %s; resolved via fallback.", primaryPath, primary.FailureStage)
		}
		return resolved
	}

	// No fallback was selected. Return the primary candidate's exact
	// absent/open/read/close/decode result rather than blaming whichever
	// later candidate happened to fail last.
	primary.PrimaryFailed = primaryPath
	primary.Fallback = ""
	return primary
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

	switch result.Status {
	case AssetHealthVerified:
		icon = theme.ConfirmIcon()
	case AssetHealthAbsent:
		icon = theme.WarningIcon()
	case AssetHealthUnreadable, AssetHealthFallback:
		icon = theme.ErrorIcon()
	default:
		icon = theme.QuestionIcon()
	}

	btn := widget.NewButtonWithIcon("", icon, func() {
		details := result.Message
		if result.Provenance != "" {
			details += "\n\nSource: " + result.Provenance
		}
		if result.PrimaryFailed != "" {
			details += "\n\nPrimary Failed Path: " + result.PrimaryFailed
			if result.FailureStage != "" {
				details += fmt.Sprintf(" (Stage: %s)", result.FailureStage)
			}
		}
		if result.Fallback != "" {
			details += "\n\nFallback Selected: " + result.Fallback
		}
		if result.DecodeErr != nil {
			details += "\n\nError: " + result.DecodeErr.Error()
		}
		if len(result.Suggestions) > 0 {
			details += "\n\nSuggestions: " + strings.Join(result.Suggestions, ", ")
		}

		windows := fyne.CurrentApp().Driver().AllWindows()
		if len(windows) > 0 {
			dialog.ShowInformation("Asset Diagnostics", details, windows[0])
		}
	})
	btn.Importance = widget.LowImportance

	return container.NewGridWrap(fyne.NewSize(32, 32), btn)
}
