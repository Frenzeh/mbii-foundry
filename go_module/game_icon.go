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
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"path/filepath"
	"strings"
	"sync"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// gameIconCache holds only immutable embedded icons (including embedded misses).
// Runtime images belong to their VFS and are discarded when it refreshes.
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
	path := strings.ToLower(basePath)
	gameIconCacheMu.RLock()
	img, cached := gameIconCache[path]
	gameIconCacheMu.RUnlock()
	if !cached {
		img = loadEmbeddedIcon(path)
		gameIconCacheMu.Lock()
		gameIconCache[path] = img
		gameIconCacheMu.Unlock()
	}
	if img == nil && vfs != nil {
		img = vfs.loadGameIcon(path)
	}

	return img, img != nil
}

func (vfs *VirtualFileSystem) loadGameIcon(path string) image.Image {
	vfs.mu.RLock()
	img, cached := vfs.gameIcons[path]
	generation := vfs.generation
	vfs.mu.RUnlock()
	if cached {
		return img
	}

	img = decodeGameIcon(vfs, path)
	vfs.mu.Lock()
	// A scan may finish during decoding. Do not repopulate the new
	// source's cache with an image (or miss) from the previous index.
	if vfs.generation == generation {
		if vfs.gameIcons == nil {
			vfs.gameIcons = make(map[string]image.Image)
		}
		vfs.gameIcons[path] = img
	}
	vfs.mu.Unlock()
	return img
}

// loadEmbeddedIcon searches the embedded icon FS for a PNG whose
// filename (stripped of ext) matches basePath's basename. The
// embedded tree is flat per-category, so we walk all category
// subdirectories. embed.FS lookups are cheap (in-memory).
//
// Keep the dir list in sync with tools/extract-icons' prefixRules
// AND with the actual subdirs created under assets/icons/. Missing
// one here means icons silently fail to resolve even though the
// PNG is baked into the binary.
func loadEmbeddedIcon(basePath string) image.Image {
	wanted := filepath.Base(basePath) + ".png"
	wanted = strings.ToLower(wanted)

	for _, dir := range []string{"weapons", "attributes", "classes", "force"} {
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

	// Fallback: if big_bacta is not present, fall back to standard bacta
	if strings.Contains(wanted, "big_bacta") {
		fallbackWanted := strings.ReplaceAll(wanted, "big_bacta", "bacta")
		for _, dir := range []string{"weapons", "attributes", "classes", "force"} {
			path := "assets/icons/" + dir + "/" + fallbackWanted
			if data, err := embedIcons.ReadFile(path); err == nil {
				if img, err := png.Decode(bytes.NewReader(data)); err == nil {
					return img
				}
			}
		}
	}
	return nil
}

// decodeGameIcon walks the extension preference list and returns the
// first that decodes successfully.
func decodeGameIcon(vfs *VirtualFileSystem, basePath string) image.Image {
	for _, ext := range []string{".tga", ".png", ".jpg", ".jpeg"} {
		full := basePath + ext
		if vfs.Lookup(full) == nil {
			continue
		}
		rc, err := vfs.ReadFile(full)
		if err != nil {
			continue
		}
		data, readErr := readRasterBytes(rc)
		closeErr := rc.Close()
		if readErr != nil || closeErr != nil || len(data) == 0 {
			continue
		}
		img, _ := decodeByExt(ext, data)
		if img != nil {
			return img
		}
	}

	// Fallback for big_bacta -> bacta in VFS
	if strings.Contains(basePath, "big_bacta") {
		fallbackBase := strings.ReplaceAll(basePath, "big_bacta", "bacta")
		for _, ext := range []string{".tga", ".png", ".jpg", ".jpeg"} {
			full := fallbackBase + ext
			if vfs.Lookup(full) == nil {
				continue
			}
			rc, err := vfs.ReadFile(full)
			if err != nil {
				continue
			}
			data, readErr := readRasterBytes(rc)
			closeErr := rc.Close()
			if readErr != nil || closeErr != nil || len(data) == 0 {
				continue
			}
			if img, _ := decodeByExt(ext, data); img != nil {
				return img
			}
		}
	}
	return nil
}

// decodeByExt dispatches explicitly so raster resources never depend on
// image.Decode's process-global format ordering.
func decodeByExt(ext string, data []byte) (image.Image, error) {
	if len(data) > maxRasterInputSize {
		return nil, errRasterInputTooLarge
	}
	switch strings.ToLower(ext) {
	case ".tga":
		return decodeTGA(data)
	case ".png":
		cfg, err := png.DecodeConfig(bytes.NewReader(data))
		if err != nil {
			return nil, err
		}
		if _, err := checkedRasterPixelCount(cfg.Width, cfg.Height); err != nil {
			return nil, err
		}
		return png.Decode(bytes.NewReader(data))
	case ".jpg", ".jpeg":
		cfg, err := jpeg.DecodeConfig(bytes.NewReader(data))
		if err != nil {
			return nil, err
		}
		if _, err := checkedRasterPixelCount(cfg.Width, cfg.Height); err != nil {
			return nil, err
		}
		return jpeg.Decode(bytes.NewReader(data))
	}
	return nil, fmt.Errorf("unsupported image extension: %s", ext)
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
		return container.New(layout.NewGridWrapLayout(fyne.NewSize(width, height)), ci)
	}
	// Fallback — generic "image" icon in the theme palette so users
	// can see which entries are missing art at a glance rather than
	// the whole row collapsing.
	fb := widget.NewIcon(theme.FileImageIcon())
	return container.New(layout.NewGridWrapLayout(fyne.NewSize(width, height)), fb)
}

// NewRasterIconFromResource takes a PNG/JPEG fyne.Resource and returns
// a sized CanvasObject that actually RENDERS it at the requested
// size.
//
// Render path: explicit png.Decode → canvas.NewImageFromImage,
// SIZED ONLY by the parent GridWrap (no SetMinSize). Why:
//
//  1. Raster formats are decoded explicitly before constructing the canvas
//     image. This avoids process-global format sniffing and repeated decode
//     attempts during repaint.
//
//  2. canvas.NewImageFromImage keeps already-decoded pixels on the direct
//     renderer path.
//
//  3. SetMinSize on canvas.Image fired Fyne's "param mismatch" log
//     in v2.7.1 — the renderer's size negotiation dislikes a
//     min-size on an image resource of different bounds. Removing
//     SetMinSize and letting the outer GridWrapLayout dictate cell
//     dimensions resolves both: the image's natural bounds drive
//     the renderer, the GridWrap forces the cell rectangle.
//
// Decoded image.Image instances are cached by resource Name() so
// 200+ rows on a grid Refresh share one decode per unique icon.
func NewRasterIconFromResource(res fyne.Resource, width, height float32) fyne.CanvasObject {
	if res == nil {
		fb := widget.NewIcon(theme.FileImageIcon())
		return container.New(layout.NewGridWrapLayout(fyne.NewSize(width, height)), fb)
	}
	if strings.HasSuffix(strings.ToLower(res.Name()), ".svg") || (len(res.Content()) > 0 && bytes.Contains(res.Content(), []byte("<svg"))) {
		ci := canvas.NewImageFromResource(res)
		ci.FillMode = canvas.ImageFillContain
		ci.ScaleMode = canvas.ImageScaleSmooth
		return container.New(layout.NewGridWrapLayout(fyne.NewSize(width, height)), ci)
	}
	if img := decodedImageFor(res); img != nil {
		ci := canvas.NewImageFromImage(img)
		ci.FillMode = canvas.ImageFillContain
		ci.ScaleMode = canvas.ImageScaleSmooth
		return container.New(layout.NewGridWrapLayout(fyne.NewSize(width, height)), ci)
	}
	// Non-PNG/SVG. Fall back to a theme placeholder
	fb := widget.NewIcon(theme.FileImageIcon())
	return container.New(layout.NewGridWrapLayout(fyne.NewSize(width, height)), fb)
}

// setRasterPreview updates an existing portrait without sending raster bytes
// through Fyne's generic decoder (the TGA decoder claims any magic header).
// Clear every previous source so missing assets cannot retain old pixels.
func setRasterPreview(preview *canvas.Image, res fyne.Resource) {
	preview.File = ""
	preview.Resource = nil
	preview.Image = nil

	if res != nil {
		if strings.HasSuffix(strings.ToLower(res.Name()), ".svg") || (len(res.Content()) > 0 && bytes.Contains(res.Content(), []byte("<svg"))) {
			preview.Resource = res
		} else {
			preview.Image = decodedImageFor(res)
		}
	}

	if preview.Image == nil && preview.Resource == nil {
		preview.Resource = theme.AccountIcon()
	}
	preview.Refresh()
}

// decodedImageFor decodes PNG bytes from a resource into an
// image.Image, caching by resource Name(). Negative results (non-PNG
// or decode failure) are stored as nil so we don't re-attempt every
// paint. Bypasses image.Decode entirely → TGA's empty-magic
// registration can't poison this path.
var (
	decodedImageCache   = map[string]image.Image{}
	decodedImageCacheMu sync.RWMutex
)

func decodedImageFor(res fyne.Resource) image.Image {
	if res == nil {
		return nil
	}
	key := res.Name()
	decodedImageCacheMu.RLock()
	if cached, ok := decodedImageCache[key]; ok {
		decodedImageCacheMu.RUnlock()
		return cached
	}
	decodedImageCacheMu.RUnlock()

	var img image.Image
	data := res.Content()
	ext := filepath.Ext(key)
	if ext != "" {
		img, _ = decodeByExt(ext, data)
	} else {
		// Fallback for missing ext: try PNG first
		if len(data) >= 8 && bytes.HasPrefix(data, pngMagic) {
			img, _ = png.Decode(bytes.NewReader(data))
		}
	}

	decodedImageCacheMu.Lock()
	decodedImageCache[key] = img
	decodedImageCacheMu.Unlock()
	return img
}

// pngMagic is the 8-byte PNG file signature. Pulled out so the
// HasPrefix check in NewRasterIconFromResource stays a slice-compare
// instead of a string-allocation per icon.
var pngMagic = []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n'}

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
