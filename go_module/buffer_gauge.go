package main

import (
	"fmt"
	"image/color"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
	"github.com/Frenzeh/mbii-foundry/parsers"
)

// WarnThresholdBytes is the established editor warning threshold for the
// ClassInfo payload. It is intentionally advisory; the engine limit remains
// parsers.ClassInfoMaxPayload.
const WarnThresholdBytes = parsers.ClassInfoMaxPayload - 1000

// BufferStatus calculates character and block lengths against engine memory buffers.
type BufferStatus struct {
	TotalFileLen  int
	ClassInfoLen  int
	MaxWeaponLen  int
	MaxForceLen   int
	MaxPairedLen  int
	IsExceeded    bool
	IsWarning     bool
	WarningMsg    string
	Diagnostics   []string
	GenerationErr error
	AssessmentErr error
}

// CalculateBufferStatus measures character byte usage for engine limits.
func CalculateBufferStatus(char *parsers.MBCHCharacter) BufferStatus {
	if char == nil {
		return BufferStatus{
			IsExceeded: true,
			WarningMsg: "Character is required",
		}
	}

	content, err := parsers.GenerateMBCH(char)
	if err != nil {
		return BufferStatus{
			IsExceeded:    true,
			WarningMsg:    fmt.Sprintf("GenerateMBCH failed: %v", err),
			GenerationErr: err,
		}
	}

	assessment, err := parsers.AssessMBCHSourceBuffers(content)
	if err != nil {
		return BufferStatus{
			IsExceeded:    true,
			WarningMsg:    fmt.Sprintf("AssessMBCHSourceBuffers failed: %v", err),
			AssessmentErr: err,
		}
	}

	status := BufferStatus{
		TotalFileLen: assessment.TotalFileBytes,
		ClassInfoLen: assessment.ClassInfoBytes,
		MaxPairedLen: assessment.MaxPairedValueBytes,
		Diagnostics:  append([]string(nil), assessment.Diagnostics...),
	}
	for _, size := range assessment.WeaponInfoBytes {
		if size > status.MaxWeaponLen {
			status.MaxWeaponLen = size
		}
	}
	for _, size := range assessment.ForceInfoBytes {
		if size > status.MaxForceLen {
			status.MaxForceLen = size
		}
	}

	warnings := make([]string, 0, len(assessment.Diagnostics)+5)
	for _, diagnostic := range assessment.Diagnostics {
		warnings = append(warnings, fmt.Sprintf("Parser diagnostic: %s", diagnostic))
	}
	if status.TotalFileLen >= parsers.MBCHMaxFileBytes {
		warnings = append(warnings, fmt.Sprintf(
			"File exceeds engine payload limit (%d/%d bytes; buffer %d)",
			status.TotalFileLen, parsers.MBCHMaxFileBytes-1, parsers.MBCHMaxFileBytes,
		))
	}
	if status.ClassInfoLen > parsers.ClassInfoMaxPayload {
		warnings = append(warnings, fmt.Sprintf(
			"ClassInfo exceeds engine payload limit (%d/%d bytes; buffer %d)",
			status.ClassInfoLen, parsers.ClassInfoMaxPayload, parsers.ClassInfoMaxBytes,
		))
	}
	if status.MaxWeaponLen > parsers.WeaponInfoMaxPayload {
		warnings = append(warnings, fmt.Sprintf(
			"WeaponInfo exceeds engine payload limit (%d/%d bytes; buffer %d)",
			status.MaxWeaponLen, parsers.WeaponInfoMaxPayload, parsers.WeaponInfoMaxBytes,
		))
	}
	if status.MaxForceLen > parsers.ForceInfoMaxPayload {
		warnings = append(warnings, fmt.Sprintf(
			"ForceInfo exceeds engine payload limit (%d/%d bytes; buffer %d)",
			status.MaxForceLen, parsers.ForceInfoMaxPayload, parsers.ForceInfoMaxBytes,
		))
	}
	if status.MaxPairedLen > parsers.PairedValueMaxPayload {
		warnings = append(warnings, fmt.Sprintf(
			"Paired value exceeds engine payload limit (%d/%d bytes; buffer %d)",
			status.MaxPairedLen, parsers.PairedValueMaxPayload, parsers.PairedValueMaxBytes,
		))
	}
	if len(warnings) > 0 {
		status.IsExceeded = true
		status.WarningMsg = strings.Join(warnings, "; ")
	} else if status.ClassInfoLen >= WarnThresholdBytes {
		status.IsWarning = true
		status.WarningMsg = fmt.Sprintf(
			"ClassInfo approaching editor warning threshold (%d/%d bytes; engine payload limit %d)",
			status.ClassInfoLen, WarnThresholdBytes, parsers.ClassInfoMaxPayload,
		)
	}
	return status
}

// BufferGaugeWidget displays a real-time memory meter in the status bar.
type BufferGaugeWidget struct {
	container *fyne.Container
	label     *widget.Label
	badge     *canvas.Rectangle
}

func NewBufferGaugeWidget() *BufferGaugeWidget {
	badge := canvas.NewRectangle(color.RGBA{R: 50, G: 160, B: 50, A: 255})
	badge.SetMinSize(fyne.NewSize(12, 12))
	badge.CornerRadius = 6

	label := widget.NewLabelWithStyle(bufferGaugeLabel(BufferStatus{}), fyne.TextAlignLeading, fyne.TextStyle{Monospace: true})

	cnt := container.NewHBox(
		badge,
		label,
	)

	return &BufferGaugeWidget{
		container: cnt,
		label:     label,
		badge:     badge,
	}
}

func (bg *BufferGaugeWidget) Update(char *parsers.MBCHCharacter) {
	status := CalculateBufferStatus(char)

	bg.label.SetText(bufferGaugeLabel(status))

	if status.IsExceeded {
		bg.badge.FillColor = color.RGBA{R: 220, G: 40, B: 40, A: 255} // Red
		bg.label.TextStyle = fyne.TextStyle{Bold: true, Monospace: true}
	} else if status.IsWarning {
		bg.badge.FillColor = color.RGBA{R: 220, G: 160, B: 30, A: 255} // Amber
		bg.label.TextStyle = fyne.TextStyle{Bold: false, Monospace: true}
	} else {
		bg.badge.FillColor = color.RGBA{R: 40, G: 180, B: 40, A: 255} // Green
		bg.label.TextStyle = fyne.TextStyle{Bold: false, Monospace: true}
	}

	bg.badge.Refresh()
	bg.label.Refresh()
}

func bufferGaugeLabel(status BufferStatus) string {
	return fmt.Sprintf(
		"File %d/%d · Class %d/%d · Weapon %d/%d · Force %d/%d · Value %d/%d · Key max %d B",
		status.TotalFileLen, parsers.MBCHMaxFileBytes-1,
		status.ClassInfoLen, parsers.ClassInfoMaxPayload,
		status.MaxWeaponLen, parsers.WeaponInfoMaxPayload,
		status.MaxForceLen, parsers.ForceInfoMaxPayload,
		status.MaxPairedLen, parsers.PairedValueMaxPayload,
		parsers.KeyMaxPayload,
	)
}

func (bg *BufferGaugeWidget) GetContent() fyne.CanvasObject {
	return bg.container
}
