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

const (
	MaxClassInfoBytes  = 8192
	MaxWeaponInfoBytes = 4096
	MaxForceInfoBytes  = 2048
	WarnThresholdBytes = 7200
)

// BufferStatus calculates character and block lengths against engine memory buffers.
type BufferStatus struct {
	ClassInfoLen  int
	MaxWeaponLen  int
	MaxForceLen   int
	AttrStringLen int
	IsExceeded    bool
	IsWarning     bool
	WarningMsg    string
}

// CalculateBufferStatus measures character byte usage for engine limits.
func CalculateBufferStatus(char *parsers.MBCHCharacter) BufferStatus {
	if char == nil {
		return BufferStatus{}
	}

	content, _ := parsers.GenerateMBCH(char)
	
	// Extract ClassInfo block
	classInfoLen := len(content)
	if idx := strings.Index(content, "WeaponInfo"); idx != -1 {
		classInfoLen = idx
	} else if idx := strings.Index(content, "description"); idx != -1 {
		classInfoLen = idx
	}

	maxWeaponLen := 0
	for _, w := range char.WeaponOverrides {
		// Approximate weapon override block length
		wLen := len(w.WeaponName) + len(w.NewWorldModel) + len(w.NewViewModel) + len(w.Icon) + len(w.MissileEffect) + len(w.FlashSound0) + 300
		if wLen > maxWeaponLen {
			maxWeaponLen = wLen
		}
	}

	maxForceLen := 0
	for _, f := range char.ForceOverrides {
		fLen := len(f.ForcePowerName) + len(f.Icon) + len(f.StartSound) + len(f.LoopSound) + 200
		if fLen > maxForceLen {
			maxForceLen = fLen
		}
	}

	attrLen := len(char.Attributes)

	status := BufferStatus{
		ClassInfoLen:  classInfoLen,
		MaxWeaponLen:  maxWeaponLen,
		MaxForceLen:   maxForceLen,
		AttrStringLen: attrLen,
	}

	if classInfoLen >= MaxClassInfoBytes {
		status.IsExceeded = true
		status.WarningMsg = fmt.Sprintf("ClassInfo buffer exceeded (%d / %d bytes)! Map will crash on load.", classInfoLen, MaxClassInfoBytes)
	} else if classInfoLen >= WarnThresholdBytes {
		status.IsWarning = true
		status.WarningMsg = fmt.Sprintf("ClassInfo approaching engine limit (%d / %d bytes).", classInfoLen, MaxClassInfoBytes)
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

	label := widget.NewLabelWithStyle("Buffer: 0 / 8192 B", fyne.TextAlignLeading, fyne.TextStyle{Monospace: true})

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

	bg.label.SetText(fmt.Sprintf("ClassInfo: %d / %d B", status.ClassInfoLen, MaxClassInfoBytes))

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

func (bg *BufferGaugeWidget) GetContent() fyne.CanvasObject {
	return bg.container
}
