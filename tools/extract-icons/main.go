package main

// One-shot extractor that pulls HUD/attribute/force icons out of the
// user's MBII PK3s, resizes them to a uniform thumbnail size, encodes
// as PNG, and stages them under names the live app will look up
// (see game_icon.go). Review before copying into go_module/assets/icons/.
//
// Run this from a machine that has the full MBII install checked out
// or synced. The resulting PNGs are committed into the Foundry repo
// and embedded into the binary via //go:embed — testers then get the
// icons without needing to have the game's PK3s available.
//
// Usage from the repository root, with MBII_PK3_DIR set to the hydrated
// public-release archive directory:
//   scratch="$(mktemp -d "${TMPDIR:-/tmp}/mbii-icons.XXXXXX")"
//   (cd tools/extract-icons && go run . \
//     -pk3-dir="$MBII_PK3_DIR" -out="$scratch/icons")
//
// Use a fresh scratch directory for each refresh; integrate reviewed PNGs only.

import (
	"archive/zip"
	"bytes"
	"flag"
	"fmt"
	"image"
	"image/jpeg"
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
// writes filepath.Base(source) with .png extension, except the smoke grenade alias.
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
const smokeNadeIcon = "models/weapons2/smokenade/icon"

func pk3PathLess(a, b string) bool {
	fold := func(c byte) byte {
		if c >= 'a' && c <= 'z' {
			return c - ('a' - 'A')
		}
		if c == '\\' || c == ':' {
			return '/'
		}
		return c
	}
	for i := 0; i < len(a) && i < len(b); i++ {
		ac, bc := fold(a[i]), fold(b[i])
		if ac != bc {
			return ac < bc
		}
	}
	return len(a) < len(b)
}

func main() {
	pk3Dir := flag.String("pk3-dir", "", "directory of MBII .pk3 files to scan")
	outDir := flag.String("out", "", "output icons directory (e.g. go_module/assets/icons)")
	verbose := flag.Bool("v", false, "verbose logging")
	flag.Parse()

	if *pk3Dir == "" || *outDir == "" {
		log.Fatal("both -pk3-dir and -out are required")
	}

	entries, err := os.ReadDir(*pk3Dir)
	if err != nil {
		log.Fatalf("read %s: %v", *pk3Dir, err)
	}
	var pk3s []string
	for _, entry := range entries {
		if !entry.IsDir() && strings.EqualFold(filepath.Ext(entry.Name()), ".pk3") {
			pk3s = append(pk3s, filepath.Join(*pk3Dir, entry.Name()))
		}
	}
	if len(pk3s) == 0 {
		log.Fatalf("no PK3 files found in %s", *pk3Dir)
	}
	// Match OpenJK FS_PathCmp, including uppercase folding and separator aliases.
	sort.Slice(pk3s, func(i, j int) bool {
		return pk3PathLess(filepath.Base(pk3s[i]), filepath.Base(pk3s[j]))
	})

	// Extracted filenames so we know what we already wrote. Later
	// PK3s override earlier ones per engine rules, so walk in order
	// and let the last writer win by overwriting.
	extracted := map[string]int{}

	for _, pk3 := range pk3s {
		if err := scanPK3(pk3, *outDir, extracted, *verbose); err != nil {
			log.Fatalf("extract %s: %v", pk3, err)
		}
	}

	total := 0
	for _, n := range extracted {
		total += n
	}
	if total == 0 {
		log.Fatal("no matching icons extracted; check the PK3 directory")
	}
	if err := writeManifest(*outDir, extracted); err != nil {
		log.Fatalf("write manifest: %v", err)
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
			return fmt.Errorf("%s: %w", f.Name, err)
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
	if strings.TrimSuffix(lowerName, filepath.Ext(lowerName)) == smokeNadeIcon {
		return struct{ src, dst string }{smokeNadeIcon, "weapons"}, true
	}
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

	name := strings.ReplaceAll(f.Name, "\\", "/")
	baseName := strings.TrimSuffix(filepath.Base(name), filepath.Ext(name))
	if strings.EqualFold(strings.TrimSuffix(name, filepath.Ext(name)), smokeNadeIcon) {
		baseName = "w_icon_smoke_nade"
	}
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
		img, err := jpeg.Decode(bytes.NewReader(data))
		return img, err
	}
	return nil, fmt.Errorf("unknown ext")
}

// writeManifest dumps a list of source → filename mappings so future
// devs can tell which PK3 contributed which icon. Debug aid only;
// the runtime loader just uses filename-to-ID matching.
func writeManifest(outDir string, byPK3 map[string]int) error {
	var lines []string
	keys := make([]string, 0, len(byPK3))
	for k := range byPK3 {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		lines = append(lines, fmt.Sprintf("%s: %d icons", k, byPK3[k]))
	}
	return os.WriteFile(filepath.Join(outDir, "EXTRACTION_LOG.txt"),
		[]byte(strings.Join(lines, "\n")+"\n"), 0644)
}
