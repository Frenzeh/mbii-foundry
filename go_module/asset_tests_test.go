package main

import (
	"bytes"
	"errors"
	"image"
	"image/color"
	"image/png"
	"io"
	"os"
	"testing"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/theme"
)

func TestTileCardKeyboardFocus(t *testing.T) {
	tapped := false
	card := NewTileCard("Test", "Sub", nil, func() {
		tapped = true
	})

	card.TypedRune(' ')
	if !tapped {
		t.Errorf("TileCard did not activate on Space key")
	}

	tapped = false
	card.TypedKey(&fyne.KeyEvent{Name: fyne.KeyReturn})
	if !tapped {
		t.Errorf("TileCard did not activate on Enter key")
	}

	test.WidgetRenderer(card)
	card.FocusGained()
	activeColor := card.bg.FillColor

	card.FocusLost()
	restingColor := card.bg.FillColor

	if activeColor == restingColor {
		t.Errorf("TileCard background color did not change on FocusGained")
	}
}

func TestRasterFormatsDoNotClaimPNG(t *testing.T) {
	want := color.NRGBA{R: 0x12, G: 0x34, B: 0x56, A: 0xff}
	source := image.NewNRGBA(image.Rect(0, 0, 1, 1))
	source.SetNRGBA(0, 0, want)
	var encoded bytes.Buffer
	if err := png.Encode(&encoded, source); err != nil {
		t.Fatal(err)
	}

	decoded, format, err := image.Decode(bytes.NewReader(encoded.Bytes()))
	if err != nil {
		t.Fatalf("standard PNG detection was intercepted: %v", err)
	}
	if format != "png" {
		t.Fatalf("image format = %q, want png", format)
	}
	if got := color.NRGBAModel.Convert(decoded.At(0, 0)); got != want {
		t.Fatalf("decoded pixel = %v, want %v", got, want)
	}
}

func TestDecodeTGARLE(t *testing.T) {
	encoded := make([]byte, 18, 22)
	encoded[2] = 10 // RLE true color
	encoded[12] = 2
	encoded[14] = 1
	encoded[16] = 24
	encoded[17] = 0x20
	encoded = append(encoded, 0x81, 0x56, 0x34, 0x12)

	decoded, err := decodeTGA(encoded)
	if err != nil {
		t.Fatal(err)
	}
	want := color.NRGBA{R: 0x12, G: 0x34, B: 0x56, A: 0xff}
	for x := range 2 {
		if got := color.NRGBAModel.Convert(decoded.At(x, 0)); got != want {
			t.Fatalf("pixel %d = %v, want %v", x, got, want)
		}
	}
}

func TestDecodeTGARejectsCompactHugeRLEWithoutAllocation(t *testing.T) {
	encoded := make([]byte, 18, 22)
	encoded[2] = 10 // RLE true color
	encoded[12], encoded[13] = 0xff, 0xff
	encoded[14], encoded[15] = 0xff, 0xff
	encoded[16] = 24
	encoded = append(encoded, 0x80, 0, 0, 0)

	var decodeErr error
	allocations := testing.AllocsPerRun(100, func() {
		_, decodeErr = decodeTGA(encoded)
	})
	if !errors.Is(decodeErr, errRasterTooLarge) {
		t.Fatalf("huge compact RLE error = %v, want %v", decodeErr, errRasterTooLarge)
	}
	if allocations != 0 {
		t.Fatalf("huge compact RLE rejection allocated %v objects, want 0", allocations)
	}
}
func TestAssetHealthDiagnosticTransitions(t *testing.T) {
	vfs := NewVirtualFileSystem("", "")

	// Create mock index for testing provenance and diagnostics
	vfs.Index = map[string]*AssetSource{
		"models/players/testmodel/model.glm": {
			PK3Path: "MBII/zz_MBModels.pk3",
		},
		"models/players/testmodel/model_default.skin": {
			PK3Path: "MBII/zz_MBModels.pk3",
		},
	}

	res := CheckModelAssetHealth("testmodel", "default", vfs)
	if res.Status != AssetHealthVerified {
		t.Errorf("Expected AssetHealthVerified, got %v", res.Status)
	}
	if res.Provenance != "MBII/zz_MBModels.pk3" {
		t.Errorf("Expected Provenance to be MBII/zz_MBModels.pk3, got %v", res.Provenance)
	}

	resMissing := CheckModelAssetHealth("missingmodel", "default", vfs)
	if resMissing.Status != AssetHealthAbsent {
		t.Errorf("Expected AssetHealthAbsent, got %v", resMissing.Status)
	}

	// Test actual icon resolver with fallback
	vfs.Index["models/players/testmodel/mb2_icon_default.tga"] = &AssetSource{
		PK3Path: "MBII/zz_MBModels.pk3",
	}
	resolver := NewIconResolver(vfs)
	_ = CheckIconAssetHealth(resolver, "testmodel", "default", "")
	// For primary-corrupt -> fallback-good:
	// We make primary icon unreadable, and fallback icon exist.
	tmpBad, _ := os.CreateTemp("", "bad.tga")
	defer os.Remove(tmpBad.Name())
	tmpBad.Write([]byte("not an image"))
	tmpBad.Close()

	tmpGood, err := os.CreateTemp("", "good.tga")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpGood.Name())

	// Include the standard 26-byte TGA footer: the previous 21-byte
	// fixture represented valid 1.0 data but was rejected by the decoder
	// contract used at the time because it always sought the 2.0 footer.
	validTGA := make([]byte, 18+3+26)
	validTGA[2] = 2  // uncompressed true color
	validTGA[12] = 1 // width
	validTGA[14] = 1 // height
	validTGA[16] = 24
	validTGA[17] = 0x20 // top-left origin
	validTGA[20] = 0xff // red in BGR order
	copy(validTGA[29:], []byte("TRUEVISION-XFILE.\x00"))
	if _, err := tmpGood.Write(validTGA); err != nil {
		t.Fatal(err)
	}
	if err := tmpGood.Close(); err != nil {
		t.Fatal(err)
	}

	// 1. primary-corrupt -> fallback-good
	vfs.Index["models/players/testmodel/mb2_icon_default.tga"] = &AssetSource{FullPath: tmpBad.Name()}
	vfs.Index["models/players/testmodel/icon_default.tga"] = &AssetSource{FullPath: tmpGood.Name()}
	iconFallbackRes := CheckIconAssetHealth(resolver, "testmodel", "default", "")

	if iconFallbackRes.Status != AssetHealthFallback {
		t.Errorf("Expected AssetHealthFallback, got %v", iconFallbackRes.Status)
	}
	if iconFallbackRes.Fallback != "models/players/testmodel/icon_default" && iconFallbackRes.Fallback != "models/players/testmodel/icon_default.tga" {
		t.Errorf("Expected Fallback to be the second candidate, got %v", iconFallbackRes.Fallback)
	}
	if iconFallbackRes.DecodeErr == nil {
		t.Errorf("Expected DecodeErr from primary to be retained")
	}
	if iconFallbackRes.FailureStage != "decode" {
		t.Errorf("Expected FailureStage 'decode', got '%v'", iconFallbackRes.FailureStage)
	}
	if iconFallbackRes.PrimaryFailed != "models/players/testmodel/mb2_icon_default" {
		t.Errorf("Expected primary candidate path, got %q", iconFallbackRes.PrimaryFailed)
	}
	if iconFallbackRes.Provenance != tmpGood.Name() {
		t.Errorf("Expected fallback provenance %q, got %q", tmpGood.Name(), iconFallbackRes.Provenance)
	}

	// 2. all-failed
	vfs.Index["models/players/testmodel/icon_default.tga"] = &AssetSource{FullPath: tmpBad.Name()} // make fallback bad too
	iconAllBadRes := CheckIconAssetHealth(resolver, "testmodel", "default", "")

	if iconAllBadRes.Status != AssetHealthUnreadable {
		t.Errorf("Expected AssetHealthUnreadable, got %v", iconAllBadRes.Status)
	}
	if iconAllBadRes.Fallback != "" {
		t.Errorf("Expected Fallback to be empty when all fail, got %v", iconAllBadRes.Fallback)
	}
	if iconAllBadRes.DecodeErr == nil {
		t.Errorf("Expected DecodeErr to be retained")
	}
	if iconAllBadRes.FailureStage != "decode" {
		t.Errorf("Expected FailureStage 'decode' when all fail due to decode, got '%v'", iconAllBadRes.FailureStage)
	}

	// 3. unreadable open-vs-decode
	vfs.Index["test_missing_on_disk.png"] = &AssetSource{FullPath: "/path/that/definitely/does/not/exist.png"}
	missingOnDiskRes := CheckGenericAssetHealth("test_missing_on_disk.png", vfs)
	if missingOnDiskRes.Status != AssetHealthUnreadable || missingOnDiskRes.FailureStage != "open" {
		t.Errorf("Expected open failure, got %v %v", missingOnDiskRes.Status, missingOnDiskRes.FailureStage)
	}

	vfs.Index["test_decode_fail.png"] = &AssetSource{FullPath: tmpBad.Name()}
	decodeFailRes := CheckGenericAssetHealth("test_decode_fail.png", vfs)
	if decodeFailRes.Status != AssetHealthUnreadable || decodeFailRes.FailureStage != "decode" {
		t.Errorf("Expected decode failure, got %v %v", decodeFailRes.Status, decodeFailRes.FailureStage)
	}

	// 4. primary-absent -> fallback-good
	delete(vfs.Index, "models/players/testmodel/mb2_icon_default.tga") // primary is absent
	vfs.Index["models/players/testmodel/icon_default.tga"] = &AssetSource{FullPath: tmpGood.Name()}
	absentFallbackRes := CheckIconAssetHealth(resolver, "testmodel", "default", "")
	if absentFallbackRes.Status != AssetHealthFallback {
		t.Errorf("Expected AssetHealthFallback for absent primary, got %v", absentFallbackRes.Status)
	}
	if absentFallbackRes.FailureStage != "missing" {
		t.Errorf("Expected FailureStage 'missing', got '%v'", absentFallbackRes.FailureStage)
	}
	if absentFallbackRes.PrimaryFailed != "models/players/testmodel/mb2_icon_default" && absentFallbackRes.PrimaryFailed != "models/players/testmodel/mb2_icon_default.tga" {
		t.Errorf("Expected absent primary path to be tracked, got '%v'", absentFallbackRes.PrimaryFailed)
	}
	if absentFallbackRes.Provenance != tmpGood.Name() {
		t.Errorf("Expected absent-primary fallback provenance %q, got %q", tmpGood.Name(), absentFallbackRes.Provenance)
	}

	// 5. primary-open-failure -> fallback-good retains the exact error.
	vfs.Index["models/players/testmodel/mb2_icon_default.tga"] = &AssetSource{FullPath: "/path/that/definitely/does/not/exist.tga"}
	openFallbackRes := CheckIconAssetHealth(resolver, "testmodel", "default", "")
	if openFallbackRes.Status != AssetHealthFallback ||
		openFallbackRes.FailureStage != "open" ||
		openFallbackRes.PrimaryFailed != "models/players/testmodel/mb2_icon_default" ||
		!errors.Is(openFallbackRes.DecodeErr, os.ErrNotExist) {
		t.Errorf("Expected exact primary open failure with fallback, got %#v", openFallbackRes)
	}
	if openFallbackRes.Provenance != tmpGood.Name() {
		t.Errorf("Expected open-failure fallback provenance %q, got %q", tmpGood.Name(), openFallbackRes.Provenance)
	}

	// 6. all absent reports the primary, not one of the fallback candidates.
	delete(vfs.Index, "models/players/testmodel/mb2_icon_default.tga")
	delete(vfs.Index, "models/players/testmodel/icon_default.tga")
	allAbsentRes := CheckIconAssetHealth(resolver, "testmodel", "default", "")
	if allAbsentRes.Status != AssetHealthAbsent ||
		allAbsentRes.FailureStage != "missing" ||
		allAbsentRes.PrimaryFailed != "models/players/testmodel/mb2_icon_default" {
		t.Errorf("Expected exact absent primary result, got %#v", allAbsentRes)
	}

	// 7. real read-error fixture.
	vfs.Index["test_read_error.png"] = &AssetSource{FullPath: tmpGood.Name()}
	testCheckGenericAssetHealthRCHook = func(rc io.ReadCloser) io.ReadCloser {
		return &mockErrorReader{ReadCloser: rc}
	}
	readFailRes := CheckGenericAssetHealth("test_read_error.png", vfs)
	testCheckGenericAssetHealthRCHook = nil
	if readFailRes.Status != AssetHealthUnreadable ||
		readFailRes.FailureStage != "read" ||
		!errors.Is(readFailRes.DecodeErr, os.ErrInvalid) {
		t.Errorf("Expected exact read failure, got %#v", readFailRes)
	}

	// 8. real close-error fixture
	vfs.Index["test_close_error.png"] = &AssetSource{FullPath: tmpGood.Name()}
	testCheckGenericAssetHealthRCHook = func(rc io.ReadCloser) io.ReadCloser {
		return &mockErrorCloser{ReadCloser: rc}
	}

	closeFailRes := CheckGenericAssetHealth("test_close_error.png", vfs)
	testCheckGenericAssetHealthRCHook = nil

	if closeFailRes.Status != AssetHealthUnreadable || closeFailRes.FailureStage != "close" {
		t.Errorf("Expected close failure, got %v %v", closeFailRes.Status, closeFailRes.FailureStage)
	}
}

type mockErrorCloser struct {
	io.ReadCloser
}

func (m *mockErrorCloser) Close() error {
	m.ReadCloser.Close()
	return os.ErrClosed
}

type mockErrorReader struct {
	io.ReadCloser
}

func (m *mockErrorReader) Read([]byte) (int, error) {
	return 0, os.ErrInvalid
}

func TestSkinVariantTransitions(t *testing.T) {
	app := test.NewApp()
	app.Settings().SetTheme(theme.DefaultTheme())
	defer app.Quit()
	win := app.NewWindow("Test")

	foundryApp := &App{mainWindow: win}
	editor := NewMBCHEditor(foundryApp)

	// Use the bound entries so the character and document session remain
	// synchronized when add/remove operations take undo snapshots.
	editor.modelEntry.SetText("base_model")
	editor.skinEntry.SetText("default")
	if editor.character.ExtraFields == nil {
		editor.character.ExtraFields = make(map[string]string)
	}

	sve := NewSkinVariantsEditor(editor)
	sve.Refresh()

	// Add variant 1
	sve.addVariant()
	if editor.character.ExtraFields["model_1"] != "base_model" {
		t.Errorf("Variant 1 model not seeded correctly")
	}

	// Edit variant 1.
	editor.character.ExtraFields["model_1"] = "new_model"
	sve.Refresh()

	// Add variant 2.
	sve.addVariant()
	if editor.character.ExtraFields["model_2"] != "base_model" {
		t.Errorf("Variant 2 model not seeded correctly")
	}

	// Removing fields leaves a sparse set with variant 2 still present.
	for _, prefix := range []string{"model_", "skin_", "uishader_"} {
		delete(editor.character.ExtraFields, prefix+"1")
	}
	sve.Refresh()
	if editor.character.ExtraFields["model_2"] != "base_model" {
		t.Errorf("Variant 2 lost after clearing variant 1")
	}

	// Adding fills the gap without overwriting the later variant.
	sve.addVariant()
	if editor.character.ExtraFields["model_1"] != "base_model" {
		t.Errorf("Sparse variant slot 1 was not reused")
	}
	if editor.character.ExtraFields["model_2"] != "base_model" {
		t.Errorf("Variant 2 lost while filling sparse slot 1")
	}

	sve.removeVariant(1)
	if _, exists := editor.character.ExtraFields["model_1"]; exists {
		t.Errorf("Removed variant 1 still exists")
	}
	if editor.character.ExtraFields["model_2"] != "base_model" {
		t.Errorf("Variant 2 lost after removing variant 1")
	}
}

func TestClickableCellKeyboardFocus(t *testing.T) {
	tapped := false
	cell := newClickableCell(canvas.NewRectangle(color.Black), func() {
		tapped = true
	})

	cell.TypedRune(' ')
	if !tapped {
		t.Errorf("clickableCell did not activate on Space")
	}

	tapped = false
	cell.TypedKey(&fyne.KeyEvent{Name: fyne.KeyReturn})
	if !tapped {
		t.Errorf("clickableCell did not activate on Enter")
	}
}

func TestAssetHealthDiagnosticContention(t *testing.T) {
	vfs := NewVirtualFileSystem("", "")

	// Create a dummy file to force a real disk read (which takes non-zero time)
	tmpFile, err := os.CreateTemp("", "mock_tga_*.tga")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpFile.Name())
	// Write a chunk of data so the read isn't instant, ensuring the window for contention exists
	tmpFile.Write(make([]byte, 1024*1024)) // 1MB
	tmpFile.Close()

	vfs.Index = map[string]*AssetSource{
		"test/icon.tga": {
			FullPath: tmpFile.Name(),
		},
	}

	// If CheckGenericAssetHealth holds an RLock across the ReadFile/Decode operation,
	// and a WLock is requested during that time, Go's RWMutex will block new RLocks.
	// Because vfs.ReadFile() ALSO requests an RLock internally, it would deadlock!
	// This test asserts that the lock is released properly before I/O.

	done := make(chan bool)
	go func() {
		CheckGenericAssetHealth("test/icon.tga", vfs)
		done <- true
	}()

	go func() {
		// Attempt to acquire WLock while the read is happening
		time.Sleep(1 * time.Millisecond)
		vfs.mu.Lock()
		vfs.mu.Unlock()
	}()

	select {
	case <-done:
		// Passed! No deadlock and no extreme lock contention.
	case <-time.After(2 * time.Second):
		t.Fatal("Deadlock detected! RLock was held during I/O causing contention with writers.")
	}
}
