package main

import (
	"archive/zip"
	"bytes"
	"image"
	"image/jpeg"
	"os"
	"path/filepath"
	"testing"
)

func TestJPEGAndExtractionErrors(t *testing.T) {
	if !pk3PathLess("a.pk3", "Z.pk3") || !pk3PathLess("Z.pk3", "_override.pk3") || pk3PathLess("a.pk3", "A.pk3") {
		t.Fatal("PK3 precedence must use engine ASCII uppercase folding")
	}
	var encoded bytes.Buffer
	if err := jpeg.Encode(&encoded, image.NewRGBA(image.Rect(0, 0, 2, 2)), nil); err != nil {
		t.Fatal(err)
	}
	if _, err := decode("icon.jpg", encoded.Bytes()); err != nil {
		t.Fatalf("declared JPEG support: %v", err)
	}

	root := t.TempDir()
	pk3 := filepath.Join(root, "icons.pk3")
	var packed bytes.Buffer
	writer := zip.NewWriter(&packed)
	entry, err := writer.Create("gfx/hud/w_icon_test.jpg")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := entry.Write([]byte("invalid image")); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(pk3, packed.Bytes(), 0600); err != nil {
		t.Fatal(err)
	}
	if err := scanPK3(pk3, filepath.Join(root, "out"), map[string]int{}, false); err == nil {
		t.Fatal("unreadable matching artwork must fail even without -v")
	}
	if err := scanPK3(filepath.Join(root, "missing.pk3"), root, map[string]int{}, false); err == nil {
		t.Fatal("missing PK3 must fail")
	}
	if err := writeManifest(filepath.Join(root, "missing"), map[string]int{}); err == nil {
		t.Fatal("manifest write failure must propagate")
	}
}
