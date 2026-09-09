//go:build darwin

package main

import (
	"bytes"
	"compress/flate"
	"encoding/binary"
	"errors"
	"fmt"
	"hash/crc32"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"
)

// ── raw zip fixture builder ─────────────────────────────────────────
//
// Hand-assembles zip archives so tests can forge headers independently
// of payload reality: declared sizes and CRCs can lie about the data,
// which is exactly the corruption the strict extractor must catch.
type zipEntryFixture struct {
	name   string
	data   []byte // store: raw content; deflate: compressed stream
	mode   uint32 // unix mode stored in external attrs
	method uint16 // 0 = store (default), 8 = deflate

	// Overrides: what the headers DECLARE (defaults derived from data).
	csize       *uint64 // compressed size
	usize       *uint64 // uncompressed size
	declaredCRC *uint32
}

// deflateFixture compresses raw with compress/flate for method-8 entries.
func deflateFixture(t *testing.T, raw []byte) []byte {
	t.Helper()
	var comp bytes.Buffer
	fw, err := flate.NewWriter(&comp, flate.DefaultCompression)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := fw.Write(raw); err != nil {
		t.Fatal(err)
	}
	if err := fw.Close(); err != nil {
		t.Fatal(err)
	}
	return comp.Bytes()
}

func buildRawZip(t *testing.T, entries []zipEntryFixture) string {
	t.Helper()
	var buf bytes.Buffer
	type cdRecord struct{ hdr []byte }
	var cds []cdRecord

	for _, e := range entries {
		offset := uint32(buf.Len())
		crc := crc32.ChecksumIEEE(e.data)
		if e.declaredCRC != nil {
			crc = *e.declaredCRC
		}
		usize := uint64(len(e.data))
		if e.usize != nil {
			usize = *e.usize
		}
		csize := uint64(len(e.data))
		if e.csize != nil {
			csize = *e.csize
		}
		nameBytes := []byte(e.name)

		var lh bytes.Buffer
		binary.Write(&lh, binary.LittleEndian, uint32(0x04034b50))
		binary.Write(&lh, binary.LittleEndian, uint16(20)) // version needed
		binary.Write(&lh, binary.LittleEndian, uint16(0))  // flags
		binary.Write(&lh, binary.LittleEndian, e.method)   // method
		binary.Write(&lh, binary.LittleEndian, uint16(0))  // mod time
		binary.Write(&lh, binary.LittleEndian, uint16(0))  // mod date
		binary.Write(&lh, binary.LittleEndian, crc)
		binary.Write(&lh, binary.LittleEndian, uint32(csize)) // compressed
		binary.Write(&lh, binary.LittleEndian, uint32(usize)) // uncompressed
		binary.Write(&lh, binary.LittleEndian, uint16(len(nameBytes)))
		binary.Write(&lh, binary.LittleEndian, uint16(0)) // extra len
		lh.Write(nameBytes)
		buf.Write(lh.Bytes())
		buf.Write(e.data)

		var ch bytes.Buffer
		binary.Write(&ch, binary.LittleEndian, uint32(0x02014b50))
		binary.Write(&ch, binary.LittleEndian, uint16(0x031E)) // made by: unix
		binary.Write(&ch, binary.LittleEndian, uint16(20))     // version needed
		binary.Write(&ch, binary.LittleEndian, uint16(0))      // flags
		binary.Write(&ch, binary.LittleEndian, e.method)       // method
		binary.Write(&ch, binary.LittleEndian, uint16(0))      // time
		binary.Write(&ch, binary.LittleEndian, uint16(0))      // date
		binary.Write(&ch, binary.LittleEndian, crc)
		binary.Write(&ch, binary.LittleEndian, uint32(csize)) // compressed
		binary.Write(&ch, binary.LittleEndian, uint32(usize)) // uncompressed
		binary.Write(&ch, binary.LittleEndian, uint16(len(nameBytes)))
		binary.Write(&ch, binary.LittleEndian, uint16(0)) // extra len
		binary.Write(&ch, binary.LittleEndian, uint16(0)) // comment len
		binary.Write(&ch, binary.LittleEndian, uint16(0)) // disk start
		binary.Write(&ch, binary.LittleEndian, uint16(0)) // internal attrs
		binary.Write(&ch, binary.LittleEndian, e.mode<<16)
		binary.Write(&ch, binary.LittleEndian, offset)
		ch.Write(nameBytes)
		cds = append(cds, cdRecord{hdr: ch.Bytes()})
	}

	cdStart := uint32(buf.Len())
	for _, cd := range cds {
		buf.Write(cd.hdr)
	}
	cdSize := uint32(buf.Len()) - cdStart

	var eocd bytes.Buffer
	binary.Write(&eocd, binary.LittleEndian, uint32(0x06054b50))
	binary.Write(&eocd, binary.LittleEndian, uint16(0)) // disk
	binary.Write(&eocd, binary.LittleEndian, uint16(0)) // cd disk
	binary.Write(&eocd, binary.LittleEndian, uint16(len(cds)))
	binary.Write(&eocd, binary.LittleEndian, uint16(len(cds)))
	binary.Write(&eocd, binary.LittleEndian, cdSize)
	binary.Write(&eocd, binary.LittleEndian, cdStart)
	binary.Write(&eocd, binary.LittleEndian, uint16(0)) // comment len

	path := filepath.Join(t.TempDir(), "fixture.zip")
	if err := os.WriteFile(path, append(buf.Bytes(), eocd.Bytes()...), 0644); err != nil {
		t.Fatal(err)
	}
	return path
}

func u64p(v uint64) *uint64 { return &v }
func u32p(v uint32) *uint32 { return &v }

func expectExtractError(t *testing.T, zipPath, destDir, wantSubstr string) {
	t.Helper()
	err := unzipGo(zipPath, destDir)
	if err == nil {
		t.Fatalf("expected extraction failure containing %q, got nil", wantSubstr)
	}
	if !strings.Contains(err.Error(), wantSubstr) {
		t.Errorf("expected error containing %q, got: %v", wantSubstr, err)
	}
}

// ── unzipGo strict validation ───────────────────────────────────────

func TestUnzipGoStrictValidation(t *testing.T) {
	t.Run("ValidRoundTrip", func(t *testing.T) {
		tmp := t.TempDir()
		dest := filepath.Join(tmp, "dest")
		zipPath := buildRawZip(t, []zipEntryFixture{
			{name: "Contents/", mode: 040755},
			{name: "Contents/MacOS/", mode: 040755},
			{name: "Contents/MacOS/mbii-foundry", data: []byte("\xcf\xfa\xed\xfe binary"), mode: 0100755},
			{name: "Contents/Info.plist", data: []byte("<plist/>"), mode: 0100644},
		})
		if err := unzipGo(zipPath, dest); err != nil {
			t.Fatalf("valid archive rejected: %v", err)
		}
		got, err := os.ReadFile(filepath.Join(dest, "Contents/MacOS/mbii-foundry"))
		if err != nil {
			t.Fatal(err)
		}
		if string(got) != "\xcf\xfa\xed\xfe binary" {
			t.Errorf("content mismatch: %q", got)
		}
		info, err := os.Stat(filepath.Join(dest, "Contents/MacOS/mbii-foundry"))
		if err != nil {
			t.Fatal(err)
		}
		if info.Mode().Perm()&0111 == 0 {
			t.Errorf("exec bit not preserved: %v", info.Mode())
		}
	})

	t.Run("TraversalRejected", func(t *testing.T) {
		tmp := t.TempDir()
		zipPath := buildRawZip(t, []zipEntryFixture{
			{name: "../evil.txt", data: []byte("pwned")},
		})
		expectExtractError(t, zipPath, filepath.Join(tmp, "dest"), "illegal path")
	})

	t.Run("NonCanonicalAndPlatformAmbiguousPathsRejected", func(t *testing.T) {
		for _, name := range []string{
			"",
			"/absolute",
			"./file",
			"a/./file",
			"C:/Windows/system.ini",
			"a/../file",
			"a/../../escape",
			"a//file",
			"a///",
			`a\file`,
			"a/\x01file",
		} {
			t.Run(fmt.Sprintf("%q", name), func(t *testing.T) {
				zipPath := buildRawZip(t, []zipEntryFixture{{name: name, data: []byte("x")}})
				expectExtractError(t, zipPath, filepath.Join(t.TempDir(), "dest"), "illegal path")
			})
		}
	})

	t.Run("DuplicateExactRejected", func(t *testing.T) {
		tmp := t.TempDir()
		zipPath := buildRawZip(t, []zipEntryFixture{
			{name: "same.txt", data: []byte("first")},
			{name: "same.txt", data: []byte("second")},
		})
		expectExtractError(t, zipPath, filepath.Join(tmp, "dest"), "duplicate entry")
	})

	t.Run("DuplicateCaseAliasRejected", func(t *testing.T) {
		// On APFS (case-insensitive by default) these two entries would
		// extract onto one path — the second silently clobbering the
		// first. Must be rejected at prevalidation.
		tmp := t.TempDir()
		zipPath := buildRawZip(t, []zipEntryFixture{
			{name: "Readme.txt", data: []byte("benign")},
			{name: "readme.txt", data: []byte("clobber")},
		})
		expectExtractError(t, zipPath, filepath.Join(tmp, "dest"), "duplicate entry")
	})

	t.Run("DuplicateNormalizationAliasRejected", func(t *testing.T) {
		tmp := t.TempDir()
		zipPath := buildRawZip(t, []zipEntryFixture{
			{name: "Caf\u00e9.txt", data: []byte("NFC")},  // é composed
			{name: "Cafe\u0301.txt", data: []byte("NFD")}, // e + combining acute
		})
		expectExtractError(t, zipPath, filepath.Join(tmp, "dest"), "duplicate entry")
	})

	t.Run("SymlinkRejected", func(t *testing.T) {
		tmp := t.TempDir()
		zipPath := buildRawZip(t, []zipEntryFixture{
			{name: "link", data: []byte("/etc/passwd"), mode: 0xA1FF}, // S_IFLNK|0777
		})
		expectExtractError(t, zipPath, filepath.Join(tmp, "dest"), "unsafe file mode")
	})

	t.Run("SetuidRejected", func(t *testing.T) {
		tmp := t.TempDir()
		zipPath := buildRawZip(t, []zipEntryFixture{
			{name: "rootish", data: []byte("x"), mode: 010755 | 0x800}, // S_ISUID
		})
		expectExtractError(t, zipPath, filepath.Join(tmp, "dest"), "unsafe file mode")
	})

	t.Run("BackslashNameRejected", func(t *testing.T) {
		tmp := t.TempDir()
		zipPath := buildRawZip(t, []zipEntryFixture{
			{name: `dir\evil.txt`, data: []byte("x")},
		})
		expectExtractError(t, zipPath, filepath.Join(tmp, "dest"), "backslash")
	})

	t.Run("OversizeDeclaredRejected", func(t *testing.T) {
		tmp := t.TempDir()
		zipPath := buildRawZip(t, []zipEntryFixture{
			{name: "huge.bin", data: []byte("x"), usize: u64p(1<<30 + 1)},
		})
		expectExtractError(t, zipPath, filepath.Join(tmp, "dest"), "too large")
	})

	t.Run("TotalOversizeRejected", func(t *testing.T) {
		tmp := t.TempDir()
		zipPath := buildRawZip(t, []zipEntryFixture{
			{name: "a.bin", usize: u64p(600 << 20)},
			{name: "b.bin", usize: u64p(600 << 20)},
		})
		expectExtractError(t, zipPath, filepath.Join(tmp, "dest"), "extraction too large")
	})

	t.Run("CRCMismatchRejected", func(t *testing.T) {
		// Headers claim a CRC the payload doesn't produce — a forged or
		// corrupted archive must fail even though sizes line up.
		tmp := t.TempDir()
		zipPath := buildRawZip(t, []zipEntryFixture{
			{name: "forged.bin", data: []byte("0123456789"), declaredCRC: u32p(0xDEADBEEF)},
		})
		// Go's archive/zip verifies stored-entry CRCs itself ("checksum
		// error"); the extractor's own CRC check is the second layer.
		// Either rejection proves a forged CRC cannot slip through.
		err := unzipGo(zipPath, filepath.Join(tmp, "dest"))
		if err == nil {
			t.Fatalf("forged CRC accepted")
		}
		if !strings.Contains(err.Error(), "CRC32 mismatch") && !strings.Contains(err.Error(), "checksum error") {
			t.Errorf("expected CRC rejection, got: %v", err)
		}
	})

	t.Run("TruncatedDeflateRejected", func(t *testing.T) {
		// Deflate stream inflates to 10 bytes while headers declare 200
		// uncompressed. Go's hardened zip reader rejects the lying size
		// itself (premature EOF is never tolerated at either layer).
		tmp := t.TempDir()
		comp := deflateFixture(t, []byte("0123456789"))
		zipPath := buildRawZip(t, []zipEntryFixture{
			{name: "trunc.bin", data: comp, method: 8, usize: u64p(200), csize: u64p(uint64(len(comp)))},
		})
		err := unzipGo(zipPath, filepath.Join(tmp, "dest"))
		if err == nil {
			t.Fatalf("lying deflate size accepted")
		}
		if !strings.Contains(err.Error(), "truncated payload") && !strings.Contains(err.Error(), "EOF") {
			t.Errorf("expected truncation rejection, got: %v", err)
		}
	})

	t.Run("TruncatedStoredPayloadRejected", func(t *testing.T) {
		// Stored member declares 200 uncompressed bytes but only 10
		// exist. Go's hardened zip reader enforces the declared size
		// itself ("unexpected EOF"); the extractor's own byte-count
		// check ("truncated payload: read N of M") is the second layer
		// for toolchains whose stdlib does not. Either rejection proves
		// premature EOF cannot slip through.
		tmp := t.TempDir()
		zipPath := buildRawZip(t, []zipEntryFixture{
			{name: "trunc.bin", data: []byte("0123456789"), usize: u64p(200), csize: u64p(10)},
		})
		err := unzipGo(zipPath, filepath.Join(tmp, "dest"))
		if err == nil {
			t.Fatalf("truncated stored member accepted")
		}
		if !strings.Contains(err.Error(), "truncated payload") && !strings.Contains(err.Error(), "EOF") {
			t.Errorf("expected truncation rejection, got: %v", err)
		}
	})

	t.Run("MkdirAllFailurePropagates", func(t *testing.T) {
		tmp := t.TempDir()
		dest := filepath.Join(tmp, "dest")
		if err := os.MkdirAll(dest, 0755); err != nil {
			t.Fatal(err)
		}
		// A FILE occupies the path where the archive needs a directory.
		if err := os.WriteFile(filepath.Join(dest, "blocker"), []byte("in the way"), 0644); err != nil {
			t.Fatal(err)
		}
		zipPath := buildRawZip(t, []zipEntryFixture{
			{name: "blocker/inner.txt", data: []byte("x")},
		})
		err := unzipGo(zipPath, dest)
		if err == nil {
			t.Fatalf("expected mkdir failure to propagate, got nil")
		}
		if !strings.Contains(err.Error(), "blocker/inner.txt") {
			t.Errorf("expected error to name the failing entry, got: %v", err)
		}
	})
}

// ── Mach-O architecture verification ────────────────────────────────

func machoSingleArchHeader(cputype uint32) []byte {
	hdr := make([]byte, 32)
	binary.LittleEndian.PutUint32(hdr[0:4], 0xfeedfacf) // MH_MAGIC_64
	binary.LittleEndian.PutUint32(hdr[4:8], cputype)
	return hdr
}

func writeFatBinary(t *testing.T, slices map[string]uint32) string {
	t.Helper()
	names := make([]string, 0, len(slices))
	for name := range slices {
		names = append(names, name)
	}
	sort.Strings(names)

	var buf bytes.Buffer
	binary.Write(&buf, binary.BigEndian, uint32(0xcafebabe)) // FAT_MAGIC
	binary.Write(&buf, binary.BigEndian, uint32(len(slices)))
	offset := uint32(8 + 20*len(slices))
	for _, name := range names {
		binary.Write(&buf, binary.BigEndian, slices[name])
		binary.Write(&buf, binary.BigEndian, uint32(0))  // cpusubtype
		binary.Write(&buf, binary.BigEndian, offset)     // offset
		binary.Write(&buf, binary.BigEndian, uint32(32)) // size
		binary.Write(&buf, binary.BigEndian, uint32(12)) // align
		offset += 32
	}
	for _, name := range names {
		buf.Write(machoSingleArchHeader(slices[name]))
	}

	path := filepath.Join(t.TempDir(), "universal")
	if err := os.WriteFile(path, buf.Bytes(), 0755); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestMachoArchsHostTestBinary(t *testing.T) {
	exe, err := os.Executable()
	if err != nil {
		t.Skip("no executable path")
	}
	archs, err := machoArchs(exe)
	if err != nil {
		t.Fatalf("parse host binary: %v", err)
	}
	want := runtime.GOARCH
	if want == "amd64" {
		want = "x86_64"
	}
	if len(archs) != 1 || !archs[want] {
		t.Errorf("host binary should contain exactly %q, got %v", want, archs)
	}
}

func TestVerifyUniversalBinaryAcceptsFat(t *testing.T) {
	path := writeFatBinary(t, map[string]uint32{
		"arm64":  12 | 0x01000000,
		"x86_64": 7 | 0x01000000,
	})
	if err := verifyUniversalBinary(path); err != nil {
		t.Errorf("universal binary rejected: %v", err)
	}
}

func TestVerifyUniversalBinaryRejectsUnexpectedThirdArchitecture(t *testing.T) {
	path := writeFatBinary(t, map[string]uint32{
		"arm64":  12 | 0x01000000,
		"x86_64": 7 | 0x01000000,
		"other":  18 | 0x01000000,
	})
	err := verifyUniversalBinary(path)
	if err == nil || !strings.Contains(err.Error(), "exact") {
		t.Fatalf("universal binary with unexpected architecture accepted: %v", err)
	}
}

func TestMachoArchsRejectsFatTableSliceMismatch(t *testing.T) {
	var buf bytes.Buffer
	binary.Write(&buf, binary.BigEndian, uint32(0xcafebabe))
	binary.Write(&buf, binary.BigEndian, uint32(1))
	binary.Write(&buf, binary.BigEndian, uint32(cpuTypeARM64))
	binary.Write(&buf, binary.BigEndian, uint32(0))
	binary.Write(&buf, binary.BigEndian, uint32(28))
	binary.Write(&buf, binary.BigEndian, uint32(32))
	binary.Write(&buf, binary.BigEndian, uint32(2))
	buf.Write(machoSingleArchHeader(cpuTypeX8664))
	path := filepath.Join(t.TempDir(), "mismatched-fat")
	if err := os.WriteFile(path, buf.Bytes(), 0755); err != nil {
		t.Fatal(err)
	}
	if _, err := machoArchs(path); err == nil {
		t.Fatal("fat table architecture accepted despite mismatched slice header")
	}
}

func TestVerifyUniversalBinaryRejectsSingleArch(t *testing.T) {
	path := filepath.Join(t.TempDir(), "single")
	if err := os.WriteFile(path, machoSingleArchHeader(12|0x01000000), 0755); err != nil {
		t.Fatal(err)
	}
	err := verifyUniversalBinary(path)
	if err == nil {
		t.Fatalf("arm64-only binary accepted as universal")
	}
	if !strings.Contains(err.Error(), "x86_64") {
		t.Errorf("error should name the missing slice, got: %v", err)
	}
}

func TestMachoArchsGarbageRejected(t *testing.T) {
	path := filepath.Join(t.TempDir(), "garbage")
	if err := os.WriteFile(path, []byte("#!/bin/sh\necho not macho\n"), 0755); err != nil {
		t.Fatal(err)
	}
	if _, err := machoArchs(path); err == nil {
		t.Fatalf("shell script accepted as Mach-O")
	}
}

// ── copyFileExec: swap, backup reservation, first install ───────────

func TestCopyFileExecSwapKeepsExclusiveBackup(t *testing.T) {
	tmp := t.TempDir()
	src := filepath.Join(tmp, "new-binary")
	dst := filepath.Join(tmp, "existing-binary")
	if err := os.WriteFile(src, []byte("new content"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dst, []byte("original content"), 0755); err != nil {
		t.Fatal(err)
	}

	backup, err := copyFileExec(src, dst)
	if err != nil {
		t.Fatalf("copyFileExec: %v", err)
	}
	if backup == "" {
		t.Fatalf("expected backup path on swap, got empty")
	}
	backupData, err := os.ReadFile(backup)
	if err != nil {
		t.Fatalf("read backup %s: %v", backup, err)
	}
	if string(backupData) != "original content" {
		t.Errorf("backup content mismatch: %q", backupData)
	}
	got, err := os.ReadFile(dst)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "new content" {
		t.Errorf("dst content mismatch: %q", got)
	}
	if !strings.Contains(filepath.Base(backup), ".foundry-oldexe-") {
		t.Errorf("backup name should use the exclusive reservation prefix, got %q", backup)
	}
}

func TestCopyFileExecFirstInstall(t *testing.T) {
	tmp := t.TempDir()
	src := filepath.Join(tmp, "new-binary")
	dst := filepath.Join(tmp, "fresh-binary")
	if err := os.WriteFile(src, []byte("fresh"), 0755); err != nil {
		t.Fatal(err)
	}

	backup, err := copyFileExec(src, dst)
	if err != nil {
		t.Fatalf("copyFileExec: %v", err)
	}
	if backup != "" {
		t.Errorf("first install must not report a backup, got %q", backup)
	}
	info, err := os.Stat(dst)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm()&0111 == 0 {
		t.Errorf("exec bit missing: %v", info.Mode())
	}
}

// ── installRawBinaryDarwin: restore on relaunch failure ─────────────

func newExtractDirWithBinary(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "mbii-foundry"), []byte(content), 0755); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestInstallRawBinaryDarwinRestoresOnRelaunchFailure(t *testing.T) {
	exe := filepath.Join(t.TempDir(), "app")
	if err := os.WriteFile(exe, []byte("original"), 0755); err != nil {
		t.Fatal(err)
	}
	extractDir := newExtractDirWithBinary(t, "updated")

	failRelaunch := func(string, ...string) error { return errors.New("exec format error") }

	err := installRawBinaryDarwin(extractDir, exe, failRelaunch, nil)
	if err == nil {
		t.Fatalf("expected relaunch failure to surface")
	}
	if !strings.Contains(err.Error(), "previous binary restored") {
		t.Errorf("error should report the restore, got: %v", err)
	}
	got, readErr := os.ReadFile(exe)
	if readErr != nil {
		t.Fatalf("restored binary unreadable: %v", readErr)
	}
	if string(got) != "original" {
		t.Errorf("binary was not restored: %q", got)
	}
}

func TestInstallRawBinaryDarwinFirstInstallRelaunchFailure(t *testing.T) {
	exe := filepath.Join(t.TempDir(), "app") // does not exist yet
	extractDir := newExtractDirWithBinary(t, "updated")

	failRelaunch := func(string, ...string) error { return errors.New("boom") }

	err := installRawBinaryDarwin(extractDir, exe, failRelaunch, nil)
	if err == nil {
		t.Fatalf("expected relaunch failure to surface")
	}
	if !strings.Contains(err.Error(), "first-install binary removed") {
		t.Errorf("error should report removal of the new binary, got: %v", err)
	}
	if _, statErr := os.Stat(exe); !os.IsNotExist(statErr) {
		t.Errorf("new binary remains after failed first-install relaunch: %v", statErr)
	}
}

func TestInstallRawBinaryDarwinSuccessSwapsContent(t *testing.T) {
	exe := filepath.Join(t.TempDir(), "app")
	if err := os.WriteFile(exe, []byte("original"), 0755); err != nil {
		t.Fatal(err)
	}
	extractDir := newExtractDirWithBinary(t, "updated")

	okRelaunch := func(string, ...string) error { return nil }

	if err := installRawBinaryDarwin(extractDir, exe, okRelaunch, nil); err != nil {
		t.Fatalf("expected success, got %v", err)
	}
	got, err := os.ReadFile(exe)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "updated" {
		t.Errorf("binary not swapped: %q", got)
	}
	matches, err := filepath.Glob(filepath.Join(filepath.Dir(exe), ".foundry-oldexe-*"))
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 0 {
		t.Errorf("successful install left binary backups: %v", matches)
	}
}

// ── rollbackBundleSwap ──────────────────────────────────────────────

func TestRollbackBundleSwapRestoresOldBundle(t *testing.T) {
	tmp := t.TempDir()
	currentBundle := filepath.Join(tmp, "MBII Foundry.app")
	stagedBundle := filepath.Join(tmp, "staged.app")
	backup := filepath.Join(tmp, "backup", "MBII Foundry.app")

	if err := os.MkdirAll(filepath.Join(currentBundle, "Contents"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(currentBundle, "Contents", "new"), []byte("half-installed"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(backup, "Contents"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(backup, "Contents", "old"), []byte("previous"), 0644); err != nil {
		t.Fatal(err)
	}

	if err := rollbackBundleSwap(currentBundle, stagedBundle, backup); err != nil {
		t.Fatalf("rollback failed: %v", err)
	}
	// Old bundle is back in the installation slot.
	if _, err := os.Stat(filepath.Join(currentBundle, "Contents", "old")); err != nil {
		t.Errorf("old bundle not restored: %v", err)
	}
	// Half-installed bundle moved back to staging.
	if _, err := os.Stat(filepath.Join(stagedBundle, "Contents", "new")); err != nil {
		t.Errorf("half-installed bundle not moved back to staging: %v", err)
	}
}

func TestRollbackBundleSwapReportsFailedRestore(t *testing.T) {
	tmp := t.TempDir()
	currentBundle := filepath.Join(tmp, "MBII Foundry.app")
	if err := os.MkdirAll(currentBundle, 0755); err != nil {
		t.Fatal(err)
	}
	// Backup does not exist — the restore must fail loudly, never silently.
	err := rollbackBundleSwap(currentBundle, filepath.Join(tmp, "staged.app"), filepath.Join(tmp, "missing-backup"))
	if err == nil {
		t.Fatalf("missing backup must fail the rollback")
	}
	if !strings.Contains(err.Error(), "restore previous bundle") {
		t.Errorf("error should name the failed restore, got: %v", err)
	}
}

func TestRollbackBundleSwapReportsFailedMoveBack(t *testing.T) {
	tmp := t.TempDir()
	currentBundle := filepath.Join(tmp, "MBII Foundry.app")
	stagedBundle := filepath.Join(tmp, "staged.app")
	backup := filepath.Join(tmp, "backup", "MBII Foundry.app")

	if err := os.MkdirAll(currentBundle, 0755); err != nil {
		t.Fatal(err)
	}
	// Occupied non-empty staging target blocks the move-back.
	if err := os.MkdirAll(filepath.Join(stagedBundle, "kept"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(backup, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(backup, "marker"), []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}

	err := rollbackBundleSwap(currentBundle, stagedBundle, backup)
	if err == nil {
		t.Fatalf("blocked move-back must fail the rollback")
	}
	if !strings.Contains(err.Error(), "move new bundle back to staging") {
		t.Errorf("error should name the failed step, got: %v", err)
	}
}

// Compile-time guard: the injected relaunch must match relaunchAndExit.
var _ func(string, ...string) error = relaunchAndExit
