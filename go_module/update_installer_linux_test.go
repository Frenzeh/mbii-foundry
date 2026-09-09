//go:build linux

package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ── tar.gz fixture builders ─────────────────────────────────────────

func buildTarGz(t *testing.T, build func(tw *tar.Writer)) string {
	t.Helper()
	var raw bytes.Buffer
	tw := tar.NewWriter(&raw)
	build(tw)
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	return gzipBytes(t, raw.Bytes())
}

func gzipBytes(t *testing.T, raw []byte) string {
	t.Helper()
	var gz bytes.Buffer
	zw := gzip.NewWriter(&gz)
	if _, err := zw.Write(raw); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "fixture.tar.gz")
	if err := os.WriteFile(path, gz.Bytes(), 0644); err != nil {
		t.Fatal(err)
	}
	return path
}

func addFile(tw *tar.Writer, name, data string, mode int64) {
	hdr := &tar.Header{Name: name, Mode: mode, Size: int64(len(data)), Typeflag: tar.TypeReg}
	if err := tw.WriteHeader(hdr); err != nil {
		panic(err)
	}
	if _, err := tw.Write([]byte(data)); err != nil {
		panic(err)
	}
}

// ustarHeader hand-assembles one POSIX header block so tests can lie
// about member sizes (archive/tar refuses to write a member shorter
// than its declared Size).
func ustarHeader(name string, size int64, typeflag byte) []byte {
	hdr := make([]byte, 512)
	copy(hdr[0:100], name)
	copy(hdr[100:108], "0000644\x00")
	copy(hdr[108:116], "0000000\x00")
	copy(hdr[116:124], "0000000\x00")
	copy(hdr[124:136], fmt.Sprintf("%011o\x00", size))
	copy(hdr[136:148], "00000000000\x00")
	copy(hdr[148:156], "        ") // checksum placeholder
	hdr[156] = typeflag
	copy(hdr[257:263], "ustar\x00")
	copy(hdr[263:265], "00")
	chksum := 0
	for _, b := range hdr {
		chksum += int(b)
	}
	copy(hdr[148:156], fmt.Sprintf("%06o\x00 ", chksum))
	return hdr
}

func expectUntarError(t *testing.T, archivePath, destDir, wantSubstr string) {
	t.Helper()
	err := untarGz(archivePath, destDir)
	if err == nil {
		t.Fatalf("expected extraction failure containing %q, got nil", wantSubstr)
	}
	if !strings.Contains(err.Error(), wantSubstr) {
		t.Errorf("expected error containing %q, got: %v", wantSubstr, err)
	}
}

// ── untarGz strict validation ───────────────────────────────────────

func TestUntarGzStrictValidation(t *testing.T) {
	t.Run("ValidRoundTrip", func(t *testing.T) {
		dest := filepath.Join(t.TempDir(), "dest")
		archivePath := buildTarGz(t, func(tw *tar.Writer) {
			addFile(tw, "mbii-foundry", "ELF binary", 0755)
			addFile(tw, "definitions/MB_ATT_PUSH.md", "docs", 0644)
		})
		if err := untarGz(archivePath, dest); err != nil {
			t.Fatalf("valid archive rejected: %v", err)
		}
		got, err := os.ReadFile(filepath.Join(dest, "mbii-foundry"))
		if err != nil {
			t.Fatal(err)
		}
		if string(got) != "ELF binary" {
			t.Errorf("content mismatch: %q", got)
		}
		info, err := os.Stat(filepath.Join(dest, "mbii-foundry"))
		if err != nil {
			t.Fatal(err)
		}
		if info.Mode().Perm()&0111 == 0 {
			t.Errorf("exec bit not preserved: %v", info.Mode())
		}
	})

	t.Run("TraversalRejected", func(t *testing.T) {
		archivePath := buildTarGz(t, func(tw *tar.Writer) {
			addFile(tw, "../evil.txt", "pwned", 0644)
		})
		expectUntarError(t, archivePath, filepath.Join(t.TempDir(), "dest"), "unsafe path")
	})

	t.Run("NonCanonicalAndPlatformAmbiguousPathsRejected", func(t *testing.T) {
		for _, name := range []string{
			"/absolute",
			"./file",
			"a/./file",
			"C:/Windows/system.ini",
			"a/../file",
			"a/../../escape",
			"a//file",
			`a\file`,
			"a/\x01file",
		} {
			t.Run(fmt.Sprintf("%q", name), func(t *testing.T) {
				archivePath := buildTarGz(t, func(tw *tar.Writer) {
					addFile(tw, name, "x", 0644)
				})
				expectUntarError(t, archivePath, filepath.Join(t.TempDir(), "dest"), "unsafe path")
			})
		}
	})

	t.Run("DuplicateExactRejected", func(t *testing.T) {
		archivePath := buildTarGz(t, func(tw *tar.Writer) {
			addFile(tw, "same.txt", "first", 0644)
			addFile(tw, "same.txt", "second", 0644)
		})
		expectUntarError(t, archivePath, filepath.Join(t.TempDir(), "dest"), "duplicate entry")
	})

	t.Run("DuplicateCaseAliasRejected", func(t *testing.T) {
		// Defense for case-insensitive extraction targets (SMB/CIFS
		// mounts): case-aliased members must be rejected everywhere.
		archivePath := buildTarGz(t, func(tw *tar.Writer) {
			addFile(tw, "Readme.txt", "benign", 0644)
			addFile(tw, "readme.txt", "clobber", 0644)
		})
		expectUntarError(t, archivePath, filepath.Join(t.TempDir(), "dest"), "duplicate entry")
	})

	t.Run("DuplicateNormalizationAndUnicodeFoldAliasesRejected", func(t *testing.T) {
		for _, pair := range [][2]string{
			{"Caf\u00e9.txt", "Cafe\u0301.txt"},
			{"Stra\u00dfe.txt", "STRASSE.TXT"},
		} {
			archivePath := buildTarGz(t, func(tw *tar.Writer) {
				addFile(tw, pair[0], "first", 0644)
				addFile(tw, pair[1], "second", 0644)
			})
			expectUntarError(t, archivePath, filepath.Join(t.TempDir(), "dest"), "duplicate entry")
		}
	})

	t.Run("SymlinkRejected", func(t *testing.T) {
		archivePath := buildTarGz(t, func(tw *tar.Writer) {
			if err := tw.WriteHeader(&tar.Header{Name: "link", Typeflag: tar.TypeSymlink, Linkname: "/etc/passwd"}); err != nil {
				t.Fatal(err)
			}
		})
		expectUntarError(t, archivePath, filepath.Join(t.TempDir(), "dest"), "links not allowed")
	})

	t.Run("HardlinkRejected", func(t *testing.T) {
		archivePath := buildTarGz(t, func(tw *tar.Writer) {
			if err := tw.WriteHeader(&tar.Header{Name: "link", Typeflag: tar.TypeLink, Linkname: "target"}); err != nil {
				t.Fatal(err)
			}
		})
		expectUntarError(t, archivePath, filepath.Join(t.TempDir(), "dest"), "links not allowed")
	})

	t.Run("DeviceRejected", func(t *testing.T) {
		archivePath := buildTarGz(t, func(tw *tar.Writer) {
			if err := tw.WriteHeader(&tar.Header{Name: "zero", Typeflag: tar.TypeChar, Devmajor: 1, Devminor: 5}); err != nil {
				t.Fatal(err)
			}
		})
		expectUntarError(t, archivePath, filepath.Join(t.TempDir(), "dest"), "unsupported tar member type")
	})

	t.Run("OversizeDeclaredRejected", func(t *testing.T) {
		// Hand-built header declaring 2GB — prevalidation must reject
		// before any data is read.
		var raw bytes.Buffer
		raw.Write(ustarHeader("huge.bin", 2<<30, '0'))
		raw.Write(make([]byte, 1024)) // EOF blocks
		archivePath := gzipBytes(t, raw.Bytes())
		expectUntarError(t, archivePath, filepath.Join(t.TempDir(), "dest"), "too large")
	})

	t.Run("TruncatedMemberRejected", func(t *testing.T) {
		// Header declares 200 bytes; the stream ends after 10. The
		// member read must fail on the short count, never install a
		// partial payload.
		var raw bytes.Buffer
		raw.Write(ustarHeader("t.bin", 200, '0'))
		raw.Write([]byte("0123456789"))
		archivePath := gzipBytes(t, raw.Bytes())
		expectUntarError(t, archivePath, filepath.Join(t.TempDir(), "dest"), "truncated")
	})

	t.Run("CorruptGzipRejected", func(t *testing.T) {
		archivePath := buildTarGz(t, func(tw *tar.Writer) {
			addFile(tw, "data.bin", "payload payload payload", 0644)
		})
		data, err := os.ReadFile(archivePath)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(archivePath, data[:len(data)-12], 0644); err != nil {
			t.Fatal(err)
		}
		if err := untarGz(archivePath, filepath.Join(t.TempDir(), "dest")); err == nil {
			t.Fatalf("corrupt gzip accepted")
		}
	})

	t.Run("MkdirAllFailurePropagates", func(t *testing.T) {
		dest := filepath.Join(t.TempDir(), "dest")
		if err := os.MkdirAll(dest, 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dest, "blocker"), []byte("in the way"), 0644); err != nil {
			t.Fatal(err)
		}
		archivePath := buildTarGz(t, func(tw *tar.Writer) {
			addFile(tw, "blocker/inner.txt", "x", 0644)
		})
		err := untarGz(archivePath, dest)
		if err == nil {
			t.Fatalf("expected mkdir failure to propagate, got nil")
		}
		if !strings.Contains(err.Error(), "blocker/inner.txt") {
			t.Errorf("expected error to name the failing member, got: %v", err)
		}
	})
}

// ── copyFileExecLinux: swap, backup reservation, first install ──────

func TestCopyFileExecLinuxSwapKeepsExclusiveBackup(t *testing.T) {
	tmp := t.TempDir()
	src := filepath.Join(tmp, "new-binary")
	dst := filepath.Join(tmp, "existing-binary")
	if err := os.WriteFile(src, []byte("new content"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dst, []byte("original content"), 0755); err != nil {
		t.Fatal(err)
	}

	backup, err := copyFileExecLinux(src, dst)
	if err != nil {
		t.Fatalf("copyFileExecLinux: %v", err)
	}
	if backup == "" {
		t.Fatalf("expected backup path on swap, got empty")
	}
	backupData, err := os.ReadFile(backup)
	if err != nil {
		t.Fatalf("read backup: %v", err)
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
		t.Errorf("backup should use the exclusive reservation prefix, got %q", backup)
	}
}

func TestCopyFileExecLinuxFirstInstall(t *testing.T) {
	tmp := t.TempDir()
	src := filepath.Join(tmp, "new-binary")
	dst := filepath.Join(tmp, "fresh-binary")
	if err := os.WriteFile(src, []byte("fresh"), 0755); err != nil {
		t.Fatal(err)
	}

	backup, err := copyFileExecLinux(src, dst)
	if err != nil {
		t.Fatalf("copyFileExecLinux: %v", err)
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

// ── transactional binary + resource installation ───────────────────

func makeLinuxInstallFixture(t *testing.T) (extractDir, installDir, exe string) {
	t.Helper()
	extractDir = filepath.Join(t.TempDir(), "extract")
	installDir = t.TempDir()
	if err := os.MkdirAll(extractDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(extractDir, "mbii-foundry"), []byte("new binary"), 0755); err != nil {
		t.Fatal(err)
	}
	exe = filepath.Join(installDir, "mbii-foundry")
	if err := os.WriteFile(exe, []byte("old binary"), 0755); err != nil {
		t.Fatal(err)
	}
	for _, name := range linuxReleaseDataDirs {
		releaseDir := filepath.Join(extractDir, name)
		installedDir := filepath.Join(installDir, name)
		if err := os.MkdirAll(releaseDir, 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.MkdirAll(installedDir, 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(releaseDir, "version.txt"), []byte("new "+name), 0644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(installedDir, "version.txt"), []byte("old "+name), 0644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(installedDir, "custom.txt"), []byte("custom "+name), 0644); err != nil {
			t.Fatal(err)
		}
	}
	return extractDir, installDir, exe
}

func TestInstallLinuxPayloadRollsBackBinaryAndEveryResourceOnRelaunchFailure(t *testing.T) {
	extractDir, installDir, exe := makeLinuxInstallFixture(t)
	err := installLinuxPayload(extractDir, exe, func(string, ...string) error {
		return errors.New("simulated relaunch failure")
	}, nil)
	if err == nil || !strings.Contains(err.Error(), "binary and resources restored") {
		t.Fatalf("expected complete rollback report, got %v", err)
	}
	assertFileContent(t, exe, "old binary")
	for _, name := range linuxReleaseDataDirs {
		assertFileContent(t, filepath.Join(installDir, name, "version.txt"), "old "+name)
		assertFileContent(t, filepath.Join(installDir, name, "custom.txt"), "custom "+name)
	}
	assertNoLinuxTransactionArtifacts(t, installDir)
}

func TestInstallLinuxPayloadPublishesCompleteMergedDirectories(t *testing.T) {
	extractDir, installDir, exe := makeLinuxInstallFixture(t)
	if err := installLinuxPayload(extractDir, exe, func(string, ...string) error { return nil }, nil); err != nil {
		t.Fatalf("installLinuxPayload: %v", err)
	}
	assertFileContent(t, exe, "new binary")
	for _, name := range linuxReleaseDataDirs {
		assertFileContent(t, filepath.Join(installDir, name, "version.txt"), "new "+name)
		assertFileContent(t, filepath.Join(installDir, name, "custom.txt"), "custom "+name)
	}
	assertNoLinuxTransactionArtifacts(t, installDir)
}

func TestInstallLinuxPayloadRejectsMissingShippedDirectoryBeforeMutation(t *testing.T) {
	extractDir, installDir, exe := makeLinuxInstallFixture(t)
	if err := os.RemoveAll(filepath.Join(extractDir, "schemas")); err != nil {
		t.Fatal(err)
	}
	err := installLinuxPayload(extractDir, exe, func(string, ...string) error {
		t.Fatal("relaunch called for incomplete release")
		return nil
	}, nil)
	if err == nil || !strings.Contains(err.Error(), "missing required resource directory schemas") {
		t.Fatalf("expected missing directory rejection, got %v", err)
	}
	assertFileContent(t, exe, "old binary")
	for _, name := range linuxReleaseDataDirs {
		assertFileContent(t, filepath.Join(installDir, name, "version.txt"), "old "+name)
	}
	assertNoLinuxTransactionArtifacts(t, installDir)
}

func TestRollbackLinuxInstallReportsRetainedRecoveryPaths(t *testing.T) {
	installDir := t.TempDir()
	transactionDir := filepath.Join(installDir, ".foundry-transaction-recovery")
	if err := os.Mkdir(transactionDir, 0700); err != nil {
		t.Fatal(err)
	}
	swap := linuxResourceSwap{
		name:      "data",
		target:    filepath.Join(installDir, "missing-current"),
		staged:    filepath.Join(transactionDir, "staged-data"),
		backup:    filepath.Join(transactionDir, "backup-data"),
		oldMoved:  true,
		installed: true,
	}
	if err := os.Mkdir(swap.backup, 0700); err != nil {
		t.Fatal(err)
	}
	execBackup := filepath.Join(installDir, "binary-backup")
	if err := os.WriteFile(execBackup, []byte("old"), 0700); err != nil {
		t.Fatal(err)
	}
	exe := filepath.Join(installDir, "mbii-foundry")
	if err := os.Mkdir(exe, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(exe, "blocker"), []byte("x"), 0600); err != nil {
		t.Fatal(err)
	}
	err, preserve := rollbackLinuxInstall(
		[]linuxResourceSwap{swap},
		execBackup,
		exe,
		transactionDir,
		errors.New("install failure"),
	)
	if !preserve || err == nil {
		t.Fatalf("rollback failures must preserve recovery paths: preserve=%v err=%v", preserve, err)
	}
	for _, want := range []string{"ROLLBACK FAILED", transactionDir, swap.backup, execBackup} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("rollback error %q does not name %q", err, want)
		}
	}
}

func assertFileContent(t *testing.T, path, want string) {
	t.Helper()
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	if string(got) != want {
		t.Fatalf("%s = %q, want %q", path, got, want)
	}
}

func assertNoLinuxTransactionArtifacts(t *testing.T, installDir string) {
	t.Helper()
	for _, pattern := range []string{".foundry-transaction-*", ".foundry-oldexe-*", ".foundry-newexe-*"} {
		matches, err := filepath.Glob(filepath.Join(installDir, pattern))
		if err != nil {
			t.Fatal(err)
		}
		if len(matches) != 0 {
			t.Errorf("transaction artifacts remain for %s: %v", pattern, matches)
		}
	}
}

// Guard: the fixture builder must produce deterministic headers —
// checksum field recomputed over the same bytes twice must agree.
func TestUstarHeaderChecksumDeterministic(t *testing.T) {
	h1 := ustarHeader("a.txt", 123, '0')
	h2 := ustarHeader("a.txt", 123, '0')
	sum1 := sha256.Sum256(h1)
	sum2 := sha256.Sum256(h2)
	if hex.EncodeToString(sum1[:]) != hex.EncodeToString(sum2[:]) {
		t.Errorf("ustarHeader output not deterministic")
	}
}
