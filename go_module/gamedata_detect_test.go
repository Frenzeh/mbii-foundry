package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// mustMakeDir creates dir (and parents) or fails the test.
func mustMakeDir(t *testing.T, dir string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
}

// mustWriteFile writes content or fails the test.
func mustWriteFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}

func TestValidateGamedataPathRequiresBothBaseAndMBII(t *testing.T) {
	dir := t.TempDir()

	// Both present → valid.
	full := filepath.Join(dir, "full")
	mustMakeDir(t, filepath.Join(full, "base"))
	mustMakeDir(t, filepath.Join(full, "MBII"))
	if err := ValidateGamedataPath(full); err != nil {
		t.Fatalf("base+MBII should validate: %v", err)
	}

	// Dev/beta folder variants satisfy the MBII requirement.
	for _, variant := range []string{"MBIITest", "MBIIRelease"} {
		p := filepath.Join(dir, strings.ToLower(variant))
		mustMakeDir(t, filepath.Join(p, "base"))
		mustMakeDir(t, filepath.Join(p, variant))
		if err := ValidateGamedataPath(p); err != nil {
			t.Fatalf("base+%s should validate: %v", variant, err)
		}
	}

	// base/ only → rejected, and the error names the missing MBII side.
	baseOnly := filepath.Join(dir, "baseonly")
	mustMakeDir(t, filepath.Join(baseOnly, "base"))
	err := ValidateGamedataPath(baseOnly)
	if err == nil {
		t.Fatal("base-only must not validate — MBII is required")
	}
	if !strings.Contains(err.Error(), "MBII") {
		t.Errorf("error should mention missing MBII, got: %v", err)
	}

	// MBII/ only → rejected, error names the missing base side.
	mbiiOnly := filepath.Join(dir, "mbiionly")
	mustMakeDir(t, filepath.Join(mbiiOnly, "MBII"))
	err = ValidateGamedataPath(mbiiOnly)
	if err == nil {
		t.Fatal("MBII-only must not validate — base is required")
	}
	if !strings.Contains(err.Error(), "base") {
		t.Errorf("error should mention missing base, got: %v", err)
	}

	// A folder with loose pk3s but no base/ + MBII/ is NOT a GameData
	// folder (the old loose-pk3 fallback is gone).
	pk3s := filepath.Join(dir, "pk3s")
	mustMakeDir(t, pk3s)
	mustWriteFile(t, filepath.Join(pk3s, "z_assets.pk3"), "x")
	if err := ValidateGamedataPath(pk3s); err == nil {
		t.Fatal("loose-pk3 folder must not validate")
	}

	// A same-named FILE must not pass either check — base and MBII
	// have to be real directories.
	fileFake := filepath.Join(dir, "filefake")
	mustMakeDir(t, fileFake)
	mustMakeDir(t, filepath.Join(fileFake, "MBII"))
	mustWriteFile(t, filepath.Join(fileFake, "base"), "x")
	err = ValidateGamedataPath(fileFake)
	if err == nil {
		t.Fatal("file named 'base' must not satisfy the base/ requirement")
	}
	if !strings.Contains(err.Error(), "base") {
		t.Errorf("error should name the missing base dir, got: %v", err)
	}

	fileFake2 := filepath.Join(dir, "filefake2")
	mustMakeDir(t, fileFake2)
	mustMakeDir(t, filepath.Join(fileFake2, "base"))
	mustWriteFile(t, filepath.Join(fileFake2, "MBII"), "x")
	err = ValidateGamedataPath(fileFake2)
	if err == nil {
		t.Fatal("file named 'MBII' must not satisfy the MBII/ requirement")
	}
	if !strings.Contains(err.Error(), "MBII") {
		t.Errorf("error should name the missing MBII dir, got: %v", err)
	}

	if err := ValidateGamedataPath(""); err == nil {
		t.Fatal("empty path must not validate")
	}
	if err := ValidateGamedataPath(filepath.Join(dir, "missing")); err == nil {
		t.Fatal("missing folder must not validate")
	}
	fileTarget := filepath.Join(dir, "afile")
	mustWriteFile(t, fileTarget, "x")
	if err := ValidateGamedataPath(fileTarget); err == nil {
		t.Fatal("file target must not validate")
	}
}

func TestSteamLibraryPathsFromVDF(t *testing.T) {
	dir := t.TempDir()
	vdf := filepath.Join(dir, "libraryfolders.vdf")
	content := "\"libraryfolders\"\n{\n\t\"0\"\n\t{\n\t\t\"path\"\t\t\"C:\\\\Program Files (x86)\\\\Steam\"\n\t}\n\t\"1\"\n\t{\n\t\t\"path\"\t\t\"D:\\\\SteamLibrary\"\n\t}\n}"
	mustWriteFile(t, vdf, content)
	libs := steamLibraryPathsFromVDF(vdf)
	if len(libs) != 2 {
		t.Fatalf("expected 2 libraries, got %d: %v", len(libs), libs)
	}
	want0 := `C:\Program Files (x86)\Steam`
	if libs[0] != want0 {
		t.Errorf("escaped backslash should unescape to %q, got %q", want0, libs[0])
	}
	if libs[1] != `D:\SteamLibrary` {
		t.Errorf("second library mismatched: %q", libs[1])
	}

	// Missing file → nil.
	if got := steamLibraryPathsFromVDF(filepath.Join(dir, "nope.vdf")); got != nil {
		t.Errorf("missing vdf should yield nil, got %v", got)
	}

	// Oversized file treated as corrupt → nil (bounded reads).
	huge := filepath.Join(dir, "huge.vdf")
	big := make([]byte, 2<<20)
	for i := range big {
		big[i] = 'x'
	}
	mustWriteFile(t, huge, string(big))
	if got := steamLibraryPathsFromVDF(huge); got != nil {
		t.Errorf("oversized vdf should yield nil, got %v", got)
	}
}

// TestValidateTextAssetsPathStructure verifies structural TextAssets
// validation: recognized asset packages or a real git checkout pass;
// empty and random folders fail.
func TestValidateTextAssetsPathStructure(t *testing.T) {
	dir := t.TempDir()

	// Recognized asset packages.
	for _, child := range []string{"ext_data", "models", "maps"} {
		p := filepath.Join(dir, "ta_"+child)
		mustMakeDir(t, filepath.Join(p, child))
		if err := ValidateTextAssetsPath(p); err != nil {
			t.Fatalf("%s/ should satisfy TextAssets validation: %v", child, err)
		}
	}

	// Git checkout marker with content.
	gitRepo := filepath.Join(dir, "ta_git")
	mustMakeDir(t, filepath.Join(gitRepo, ".git", "objects"))
	if err := ValidateTextAssetsPath(gitRepo); err != nil {
		t.Fatalf("non-empty .git should satisfy TextAssets validation: %v", err)
	}

	// Empty .git marker (no content) → rejected.
	emptyGit := filepath.Join(dir, "ta_emptygit")
	mustMakeDir(t, filepath.Join(emptyGit, ".git"))
	if err := ValidateTextAssetsPath(emptyGit); err == nil {
		t.Fatal("empty .git must not satisfy TextAssets validation")
	}

	// Random/empty folder → rejected.
	random := filepath.Join(dir, "ta_random")
	mustMakeDir(t, random)
	if err := ValidateTextAssetsPath(random); err == nil {
		t.Fatal("empty folder must not satisfy TextAssets validation")
	}
	mustMakeDir(t, filepath.Join(random, "docs"))
	mustWriteFile(t, filepath.Join(random, "readme.md"), "hello")
	if err := ValidateTextAssetsPath(random); err == nil {
		t.Fatal("folder without recognized content must not validate")
	}

	if err := ValidateTextAssetsPath(""); err == nil {
		t.Fatal("empty path must not validate")
	}
}

// TestDetectTextAssetsPairedSiblings verifies paired TextAssets
// detection stays bounded to sibling relationships of the GameData
// folder and refuses empty/random siblings.
func TestDetectTextAssetsPairedSiblings(t *testing.T) {
	dir := t.TempDir()

	// GameData with an ../mbii/TextAssets sibling.
	gd := filepath.Join(dir, "GameData")
	mustMakeDir(t, gd)
	paired := filepath.Join(dir, "mbii", "TextAssets")
	mustMakeDir(t, filepath.Join(paired, "ext_data"))
	if got := DetectTextAssetsPath(gd); got != paired {
		t.Errorf("expected sibling mbii/TextAssets %q, got %q", paired, got)
	}

	// Direct ../TextAssets sibling.
	dir2 := t.TempDir()
	gd2 := filepath.Join(dir2, "GameData")
	mustMakeDir(t, gd2)
	direct := filepath.Join(dir2, "TextAssets")
	mustMakeDir(t, filepath.Join(direct, "models"))
	if got := DetectTextAssetsPath(gd2); got != direct {
		t.Errorf("expected sibling TextAssets %q, got %q", direct, got)
	}

	// An empty sibling with the right name must NOT autofill.
	dir3 := t.TempDir()
	gd3 := filepath.Join(dir3, "GameData")
	mustMakeDir(t, gd3)
	mustMakeDir(t, filepath.Join(dir3, "TextAssets")) // empty — noise
	if got := DetectTextAssetsPath(gd3); got != "" {
		t.Errorf("empty sibling must not be detected, got %q", got)
	}

	// No sibling → empty, not a walk.
	dir4 := t.TempDir()
	gd4 := filepath.Join(dir4, "GameData")
	mustMakeDir(t, gd4)
	mustMakeDir(t, filepath.Join(dir4, "somewhere", "TextAssets"))
	if got := DetectTextAssetsPath(gd4); got != "" {
		t.Errorf("unrelated TextAssets must not be found, got %q", got)
	}
	if got := DetectTextAssetsPath(""); got != "" {
		t.Errorf("empty gamedata must yield empty, got %q", got)
	}
}

func TestShouldReplaceGamedataPathNeverClobbersValid(t *testing.T) {
	valid := ""
	{
		dir := t.TempDir()
		p := filepath.Join(dir, "gd")
		mustMakeDir(t, filepath.Join(p, "base"))
		mustMakeDir(t, filepath.Join(p, "MBII"))
		valid = p
	}
	if !ShouldReplaceGamedataPath("", valid) {
		t.Error("empty current should be filled")
	}
	if ShouldReplaceGamedataPath(valid, "anything") {
		t.Error("valid current field must never be overwritten")
	}
	if !ShouldReplaceGamedataPath("/definitely/not/real", valid) {
		t.Error("invalid current should be replaced by a detected path")
	}
	if ShouldReplaceGamedataPath("", "") {
		t.Error("empty candidate must never replace anything")
	}
}

func TestShouldFillTextAssetsPathOnlyWhenEmpty(t *testing.T) {
	if !ShouldFillTextAssetsPath("", "/some/TextAssets") {
		t.Error("empty field should be filled")
	}
	if ShouldFillTextAssetsPath("/existing", "/some/TextAssets") {
		t.Error("existing TextAssets value must never be overwritten")
	}
	if ShouldFillTextAssetsPath("", "") {
		t.Error("empty candidate must not be written")
	}
}
