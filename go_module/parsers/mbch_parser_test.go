package parsers

import (
	"os"
	"path/filepath"
	"testing"
)

// externalTextAssetsRoot gates real-data coverage before any filesystem access.
// A path alone never opts an ordinary test run into a developer's checkout.
func externalTextAssetsRoot(t *testing.T) string {
	t.Helper()
	if testing.Short() {
		t.Skip("real TextAssets integration is disabled in short mode")
	}
	if os.Getenv("MBII_TEST_REAL_DATA") != "1" {
		t.Skip("set MBII_TEST_REAL_DATA=1 and MBII_TEXTASSETS to run real TextAssets integration")
	}
	root := os.Getenv("MBII_TEXTASSETS")
	if root == "" {
		t.Fatal("MBII_TEXTASSETS is required when MBII_TEST_REAL_DATA=1")
	}
	return root
}

func TestParseRealFiles(t *testing.T) {
	textAssetsDir := externalTextAssetsRoot(t)

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
