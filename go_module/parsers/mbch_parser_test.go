package parsers

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseRealFiles(t *testing.T) {
	// Integration test that exercises the parser against a real TextAssets
	// checkout. Skipped when the checkout isn't available (e.g. CI running
	// off a fresh clone of just this repo). Run with `MBII_TEXTASSETS=/path`
	// or place a TextAssets checkout adjacent to this repo.
	textAssetsDir := os.Getenv("MBII_TEXTASSETS")
	if textAssetsDir == "" {
		candidates := []string{
			"../../../../TextAssets",
			"../../../../../TextAssets",
			"../../../TextAssets",
		}
		for _, c := range candidates {
			if _, err := os.Stat(c); err == nil {
				textAssetsDir = c
				break
			}
		}
	}
	if textAssetsDir == "" {
		t.Skip("TextAssets checkout not found; set MBII_TEXTASSETS or place one adjacent to this repo to run this integration test")
	}

	t.Logf("Scanning for MBCH files in %s...", textAssetsDir)

	var files []string
	err := filepath.Walk(textAssetsDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && filepath.Ext(path) == ".mbch" {
			files = append(files, path)
		}
		return nil
	})

	if err != nil {
		t.Fatalf("Failed to walk directories: %v", err)
	}

	t.Logf("Found %d MBCH files. Starting parsing stress test...", len(files))

	passed := 0
	failed := 0

	for _, file := range files {
		content, err := os.ReadFile(file)
		if err != nil {
			t.Errorf("Failed to read %s: %v", file, err)
			failed++
			continue
		}

		_, err = ParseMBCH(string(content))
		if err != nil {
			// Log only the first few failures to avoid spamming
			if failed < 10 {
				t.Logf("FAIL %s: %v", filepath.Base(file), err)
			}
			failed++
		} else {
			passed++
		}
	}

	t.Logf("Result: %d Passed, %d Failed", passed, failed)

	if failed > 0 {
		t.Logf("Success Rate: %.2f%%", float64(passed)/float64(len(files))*100)
	} else {
		t.Log("Perfect Score! 100% Parsing Success.")
	}
}
