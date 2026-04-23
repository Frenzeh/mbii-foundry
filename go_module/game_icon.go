package main

// Helpers for rendering MBII in-game icons inside Foundry widgets.
// The game ships every HUD icon as a .tga inside one of its PK3s;
// Foundry has already indexed those via VirtualFileSystem, so we can
// pull the bytes out and hand them to Fyne as an image.Image.
//
// Usage pattern matches the welcome screen's logo handling (see
// welcome_screen.go): decode once into image.Image, then wrap in a
// canvas.Image inside a GridWrapLayout so the image gets a concrete
// Resize call — MinSize alone isn't enough for Fyne to always paint
// the raster at the size you expect.
//
// The user's complaint was that the weapon cards were showing
// emojis (💣 Pulse Grenade, 🔫 T-21…) where real w_icon_*.tga assets
// exist in the MBII PK3s. This file is the shared surface that
// weapon_grid / attribute_grid / anywhere-else pulls from so the
// rendering pipeline is identical everywhere.

import (
	"bytes"
	"image"
	"image/jpeg"
	"image/png"
	"io"
	"path/filepath"
	"strings"
	"sync"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/ftrvxmtrx/tga"
)

// gameIconCache memoizes decoded game icons so the asset doesn't get
// re-decoded every time the weapon grid refreshes (which happens on
// every search keystroke). Keyed by the base path sans extension —
// that's what IconResolver returns. Value is nil when a base path
// resolved but nothing decoded, so we remember the miss and don't
// re-try. Separate from "not yet looked up" which is an absent key.
var (
	gameIconCache   = map[string]image.Image{}
	gameIconCacheMu sync.RWMutex
)

// LoadGameIcon decodes a game icon, preferring the PNGs embedded
// directly in the Foundry binary (assets/icons, populated by
// tools/extract-icons) and falling back to the user's VFS (PK3 or
// loose file) for anything not embedded.
//
// The embedded lookup keys on the BASENAME of basePath — so
// "gfx/hud/w_icon_a280" resolves to embedded "weapons/w_icon_a280.png"
// via filename convention. This means the embedded set can live in
// categorized subdirs without callers needing to know which subdir.
//
// Returns (nil, false) when neither source has the asset. Cached so
// subsequent lookups are free.
func LoadGameIcon(vfs *VirtualFileSystem, basePath string) (image.Image, bool) {
	if basePath == "" {
		return nil, false
	}
	key := strings.ToLower(basePath)

	gameIconCacheMu.RLock()
	if cached, ok := gameIconCache[key]; ok {
		gameIconCacheMu.RUnlock()
		return cached, cached != nil
	}
	gameIconCacheMu.RUnlock()

	// Embedded lookup first — no I/O, no PK3 presence required.
	img := loadEmbeddedIcon(key)
	if img == nil && vfs != nil {
		img = decodeGameIcon(vfs, key)
	}

	gameIconCacheMu.Lock()
	gameIconCache[key] = img
	gameIconCacheMu.Unlock()

	return img, img != nil
}

// loadEmbeddedIcon searches the embedded icon FS for a PNG whose
// filename (stripped of ext) matches basePath's basename. The
// embedded tree is flat per-category, so we walk all category
// subdirectories. embed.FS lookups are cheap (in-memory).
func loadEmbeddedIcon(basePath string) image.Image {
	wanted := filepath.Base(basePath) + ".png"
	wanted = strings.ToLower(wanted)

	// Known subdirs — kept in sync with tools/extract-icons'
	// prefixRules so the runtime lookup matches what the extractor
	// writes. Order is irrelevant since basenames are unique.
	for _, dir := range []string{"weapons", "attributes", "force"} {
		path := "assets/icons/" + dir + "/" + wanted
		data, err := embedIcons.ReadFile(path)
		if err != nil {
			continue
		}
		img, err := png.Decode(bytes.NewReader(data))
		if err != nil {
			continue
		}
		return img
	}
	return nil
}

// decodeGameIcon walks the extension preference list and returns the
// first that decodes successfully.
func decodeGameIcon(vfs *VirtualFileSystem, basePath string) image.Image {
	for _, ext := range []string{".tga", ".png", ".jpg", ".jpeg"} {
		full := basePath + ext
		if _, ok := vfs.Index[full]; !ok {
			continue
		}
		rc, err := vfs.ReadFile(full)
		if err != nil {
			continue
		}
		data, err := io.ReadAll(rc)
		rc.Close()
		if err != nil || len(data) == 0 {
			continue
		}
		img := decodeByExt(ext, data)
		if img != nil {
			return img
		}
	}
	return nil
}

// decodeByExt dispatches to the right decoder. We avoid image.Decode
// because Foundry imports github.com/ftrvxmtrx/tga which registers
// TGA with an empty magic string — image.Decode then tries TGA first
// on every input and mis-parses PNG/JPG files. Explicit dispatch is
// how welcome_screen.go dodges the same bug.
func decodeByExt(ext string, data []byte) image.Image {
	switch strings.ToLower(ext) {
	case ".tga":
		img, _ := tga.Decode(bytes.NewReader(data))
		return img
	case ".png":
		img, _ := png.Decode(bytes.NewReader(data))
		return img
	case ".jpg", ".jpeg":
		img, _ := jpeg.Decode(bytes.NewReader(data))
		return img
	}
	return nil
}

// NewGameIconCanvas returns a sized Fyne CanvasObject rendering the
// game icon at basePath. Falls back to Fyne's generic file-image
// theme icon when no real asset is found — better than an empty box
// because the UI still conveys "this is an icon slot" visually.
// width/height are logical pixels; the image fills that box using
// ImageFillContain so non-square source TGAs don't distort.
func NewGameIconCanvas(vfs *VirtualFileSystem, basePath string, width, height float32) fyne.CanvasObject {
	if img, ok := LoadGameIcon(vfs, basePath); ok {
		ci := canvas.NewImageFromImage(img)
		ci.FillMode = canvas.ImageFillContain
		ci.ScaleMode = canvas.ImageScaleSmooth
		ci.SetMinSize(fyne.NewSize(width, height))
		return container.New(layout.NewGridWrapLayout(fyne.NewSize(width, height)), ci)
	}
	// Fallback — generic "image" icon in the theme palette so users
	// can see which entries are missing art at a glance rather than
	// the whole row collapsing.
	fb := widget.NewIcon(theme.FileImageIcon())
	return container.New(layout.NewGridWrapLayout(fyne.NewSize(width, height)), fb)
}

// stripLeadingNonWord strips any leading runes that aren't letters
// or digits, plus the whitespace immediately following them. Used to
// clean "💣 Pulse Grenade" → "Pulse Grenade" at load time so the
// real game icon (rendered via NewGameIconCanvas alongside) isn't
// competing with a decorative emoji inside the label.
func stripLeadingNonWord(s string) string {
	for i, r := range s {
		if isWordRune(r) {
			return strings.TrimLeft(s[i:], " \t")
		}
	}
	return s
}

func isWordRune(r rune) bool {
	return (r >= 'a' && r <= 'z') ||
		(r >= 'A' && r <= 'Z') ||
		(r >= '0' && r <= '9') ||
		r == '(' || r == '[' // preserve bracketed names like "(Old) Bryar"
}

// hasImageExtension is a tiny helper used by future callers that want
// to sanity-check a user-supplied icon path points at an image asset.
// Kept here alongside the other icon helpers so related logic stays
// discoverable.
func hasImageExtension(path string) bool {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".tga", ".png", ".jpg", ".jpeg":
		return true
	}
	return false
}
