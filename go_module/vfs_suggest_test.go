package main

import (
	"archive/zip"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// TestVFSSuggestPK3Reach is the integration backbone of Phase 2 (PK3-aware
// autocomplete): build a real gamedata layout with one loose file and one
// PK3 archive, then assert that Suggest() surfaces paths from BOTH sources
// in a single result list. Regressions here would silently break inline
// autocomplete on model/skin/uishader entries — the popup would only show
// loose-file matches and miss everything inside the patch PK3s.
func TestVFSSuggestPK3Reach(t *testing.T) {
	tmp := t.TempDir()
	gamedata := filepath.Join(tmp, "gamedata")
	textAssets := filepath.Join(tmp, "textassets")
	// Loose tree under textAssets (VFS.indexDirectory walks here).
	if err := os.MkdirAll(filepath.Join(textAssets, "models", "players", "reborn_loose"), 0o755); err != nil {
		t.Fatal(err)
	}
	loose := filepath.Join(textAssets, "models", "players", "reborn_loose", "model_default.skin")
	if err := os.WriteFile(loose, []byte("loose"), 0o644); err != nil {
		t.Fatal(err)
	}

	// PK3 under gamedata (VFS.findPK3s + indexPK3 walks here). Put it
	// in MBII/ to match the engine's mount order — findPK3s looks at
	// gamedata/, gamedata/MBII/, gamedata/MBIITest/, gamedata/base/.
	if err := os.MkdirAll(filepath.Join(gamedata, "MBII"), 0o755); err != nil {
		t.Fatal(err)
	}
	pk3 := filepath.Join(gamedata, "MBII", "z_pk3_models.pk3")
	zf, err := os.Create(pk3)
	if err != nil {
		t.Fatal(err)
	}
	zw := zip.NewWriter(zf)
	w, err := zw.Create("models/players/reborn_packed/model_default.skin")
	if err != nil {
		t.Fatal(err)
	}
	w.Write([]byte("packed"))
	zw.Close()
	zf.Close()

	vfs := NewVirtualFileSystem(gamedata, textAssets)
	if err := vfs.Refresh(); err != nil {
		t.Fatalf("vfs refresh: %v", err)
	}

	// Suggest should surface both sources for the substring "reborn_"
	matches := vfs.Suggest("reborn_", nil, 50)
	if len(matches) < 2 {
		t.Fatalf("expected >=2 matches across loose + PK3, got %d: %v", len(matches), matches)
	}
	sort.Strings(matches)
	wantLoose := strings.Contains(strings.ToLower(matches[1]), "reborn_loose") ||
		strings.Contains(strings.ToLower(matches[0]), "reborn_loose")
	wantPacked := strings.Contains(strings.ToLower(matches[1]), "reborn_packed") ||
		strings.Contains(strings.ToLower(matches[0]), "reborn_packed")
	if !wantLoose {
		t.Errorf("expected a loose-file match in results, got %v", matches)
	}
	if !wantPacked {
		t.Errorf("expected a PK3-packed match in results, got %v", matches)
	}

	// accept filter — only .skin files (both happen to be, but exercises
	// the filter wiring against a non-matching extension)
	skinOnly := vfs.Suggest("reborn_", HasSuffixAny(".skin"), 50)
	for _, m := range skinOnly {
		if !strings.HasSuffix(strings.ToLower(m), ".skin") {
			t.Errorf("accept filter leaked non-.skin path: %s", m)
		}
	}

	// max cap respected
	capped := vfs.Suggest("", nil, 1)
	if len(capped) != 1 {
		t.Errorf("max=1 should cap result list, got %d", len(capped))
	}
}
