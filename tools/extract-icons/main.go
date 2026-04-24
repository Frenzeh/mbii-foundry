package main

// One-shot extractor that pulls HUD/attribute/force icons out of the
// user's MBII PK3s, resizes them to a uniform thumbnail size, encodes
// as PNG, and writes them into go_module/assets/icons/ under a name
// that matches the ID the live app will look up (see game_icon.go).
//
// Run this from a machine that has the full MBII install checked out
// or synced. The resulting PNGs are committed into the Foundry repo
// and embedded into the binary via //go:embed — testers then get the
// icons without needing to have the game's PK3s available.
//
// Usage:
//   go run ./tools/extract-icons \
//     -pk3-dir=/Users/pj/Library/CloudStorage/SynologyDrive-mcp5/MBII_GameData/MBII \
//     -out=/Users/pj/mbii-foundry/go_module/assets/icons
//
// Idempotent — rerun anytime the source PK3s change.

import (
	"archive/zip"
	"bytes"
	"flag"
	"fmt"
	"image"
	"image/png"
	"io"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/ftrvxmtrx/tga"
	"github.com/nfnt/resize"
)

// prefixRules maps a source prefix inside PK3s to the output
// subdirectory we want the resulting PNGs dropped into. The extractor
// writes the filename as filepath.Base(source) with .png extension.
var prefixRules = []struct {
	src, dst string
}{
	{"gfx/hud/w_icon_", "weapons"},                // in-HUD weapon icons
	{"gfx/menus/alpha/icon_stats_", "attributes"}, // MBII's class-builder attribute icons
	{"gfx/menus/alpha/icon_stat_", "attributes"},  // alternate singular form
	{"gfx/hud/chk_", "attributes"},                // legacy checkbox icons
	{"gfx/hud/i_icon_", "attributes"},             // alternate attribute icon pattern
	// Force power icons — MBII's shaders/fp_icons.shader points at
	// gfx/mp/new_f_icon_* (push, pull, speed, jump, sight, heal,
	// absorb, protect, mind_trick, drain, grip, rage, lightning, etc)
	// plus the standalone force_blind / force_destruction / deadly_sight
	// textures and the forcepowers/ level-specific variants.
	{"gfx/mp/new_f_icon_", "force"},
	{"gfx/mp/force_", "force"},
	{"gfx/mp/deadly_sight", "force"},
	{"gfx/forcepowers/", "force"},
	{"gfx/2d/forcepowers/", "force"},
	{"gfx/menus/forcepowers/", "force"},
	{"gfx/menus/classes/", "classes"}, // class builder class icons
}

const thumbMax = 96 // px — small enough to keep binary size modest

func main() {
	pk3Dir := flag.String("pk3-dir", "", "directory of MBII .pk3 files to scan")
	outDir := flag.String("out", "", "output icons directory (e.g. go_module/assets/icons)")
	verbose := flag.Bool("v", false, "verbose logging")
	flag.Parse()

	if *pk3Dir == "" || *outDir == "" {
		log.Fatal("both -pk3-dir and -out are required")
	}

	pk3s, err := filepath.Glob(filepath.Join(*pk3Dir, "*.pk3"))
	if err != nil {
		log.Fatalf("glob %s: %v", *pk3Dir, err)
	}
	// Engine load order: alphabetical. Later PK3s override earlier.
	sort.Strings(pk3s)

	// Extracted filenames so we know what we already wrote. Later
	// PK3s override earlier ones per engine rules, so walk in order
	// and let the last writer win by overwriting.
	extracted := map[string]int{}

	for _, pk3 := range pk3s {
		if err := scanPK3(pk3, *outDir, extracted, *verbose); err != nil {
			log.Printf("skip %s: %v", pk3, err)
		}
	}

	writeManifest(*outDir, extracted)

	total := 0
	for _, n := range extracted {
		total += n
	}
	log.Printf("Extracted %d icons across %d PK3s → %s", total, len(extracted), *outDir)
}

func scanPK3(path, outDir string, extracted map[string]int, verbose bool) error {
	r, err := zip.OpenReader(path)
	if err != nil {
		return err
	}
	defer r.Close()

	count := 0
	for _, f := range r.File {
		name := strings.ToLower(strings.ReplaceAll(f.Name, "\\", "/"))
		if !hasImageExt(name) {
			continue
		}
		rule, ok := matchRule(name)
		if !ok {
			continue
		}
		if err := extractOne(f, rule.dst, outDir); err != nil {
			if verbose {
				log.Printf("  skip %s: %v", f.Name, err)
			}
			continue
		}
		count++
	}
	extracted[filepath.Base(path)] += count
	if verbose || count > 0 {
		log.Printf("%s: %d icons", filepath.Base(path), count)
	}
	return nil
}

func matchRule(lowerName string) (struct{ src, dst string }, bool) {
	for _, rule := range prefixRules {
		if strings.HasPrefix(lowerName, rule.src) {
			return rule, true
		}
	}
	return struct{ src, dst string }{}, false
}

func hasImageExt(lowerName string) bool {
	switch filepath.Ext(lowerName) {
	case ".tga", ".png", ".jpg", ".jpeg":
		return true
	}
	return false
}

func extractOne(f *zip.File, dstSubdir, outRoot string) error {
	rc, err := f.Open()
	if err != nil {
		return err
	}
	defer rc.Close()
	data, err := io.ReadAll(rc)
	if err != nil {
		return err
	}

	img, err := decode(f.Name, data)
	if err != nil {
		return fmt.Errorf("decode: %w", err)
	}
	// Uniform thumbnail keeps the binary small + renders consistently
	// across weapon and attribute grids regardless of source resolution.
	img = resize.Thumbnail(thumbMax, thumbMax, img, resize.Lanczos3)

	baseName := strings.TrimSuffix(filepath.Base(f.Name), filepath.Ext(f.Name))
	outPath := filepath.Join(outRoot, dstSubdir, strings.ToLower(baseName)+".png")
	if err := os.MkdirAll(filepath.Dir(outPath), 0755); err != nil {
		return err
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return err
	}
	return os.WriteFile(outPath, buf.Bytes(), 0644)
}

func decode(name string, data []byte) (image.Image, error) {
	switch strings.ToLower(filepath.Ext(name)) {
	case ".tga":
		return tga.Decode(bytes.NewReader(data))
	case ".png":
		img, err := png.Decode(bytes.NewReader(data))
		return img, err
	case ".jpg", ".jpeg":
		img, _, err := image.Decode(bytes.NewReader(data))
		return img, err
	}
	return nil, fmt.Errorf("unknown ext")
}

// writeManifest dumps a list of source → filename mappings so future
// devs can tell which PK3 contributed which icon. Debug aid only;
// the runtime loader just uses filename-to-ID matching.
func writeManifest(outDir string, byPK3 map[string]int) {
	var lines []string
	keys := make([]string, 0, len(byPK3))
	for k := range byPK3 {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		lines = append(lines, fmt.Sprintf("%s: %d icons", k, byPK3[k]))
	}
	_ = os.WriteFile(filepath.Join(outDir, "EXTRACTION_LOG.txt"),
		[]byte(strings.Join(lines, "\n")+"\n"), 0644)
}
