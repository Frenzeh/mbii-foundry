//go:build darwin

package main

// Mac auto-installer.
//
// Release artifact is a .zip containing "MBII Foundry.app". Process:
//  1. Download the zip and verify the Ed25519-signed manifest
//     (version, platform, architecture, length, sha256 digest) —
//     downloadToTemp does both.
//  2. Extract with the Go stdlib. unzipGo rejects path traversal,
//     symlinks, setuid/setgid modes, oversized entries, and duplicate
//     entries under case/normalization aliasing, and verifies every
//     member's byte count and CRC32 — a truncated or forged payload
//     fails the extraction outright.
//  3. Verify the new bundle structurally: codesign --verify proves
//     the signature CI applied is intact, and a Mach-O header parse
//     proves the main executable carries BOTH arm64 and x86_64
//     slices.
//  4. Swap atomically on the installation filesystem: exclusive
//     MkdirTemp staging and backup directories (no PID-suffixed,
//     predictable names), rename-only installation, and every
//     rollback rename error-checked — a failed restore is reported
//     loudly with the backup location instead of being ignored.
//  5. Relaunch the new binary and exit.
//
// Trust reality: a successful install proves (a) the artifact matches
// the publisher-signed manifest and (b) the new bundle is internally
// consistent. It does NOT confer Apple Developer ID trust or
// notarization — Gatekeeper still governs the relaunch, and releases
// built without Apple signing credentials are ad-hoc signed. Every
// structural failure (broken signature, missing architecture slice,
// truncated payload) fails closed and restores the previous bundle.

import (
	"archive/zip"
	"debug/macho"
	"errors"
	"fmt"
	"hash/crc32"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/Frenzeh/mbii-foundry/updatemanifest"
)

func installUpdatePlatform(asset *ReleaseAsset, manifest UpdateManifest, progress func(UpdateProgress)) error {
	if !strings.HasSuffix(strings.ToLower(asset.Name), ".zip") {
		return fmt.Errorf("expected .zip asset for macOS, got %s", asset.Name)
	}

	archivePath, cleanup, err := downloadToTemp(asset, manifest, progress)
	if err != nil {
		return err
	}
	defer cleanup()

	if progress != nil {
		progress(UpdateProgress{Stage: "extracting", Percent: -1, Message: "Extracting update…"})
	}

	currentBundle := appBundleContainingExe()
	if currentBundle == "" {
		// Not running inside a bundle — fall back to raw-binary swap.
		extractDir, err := os.MkdirTemp("", "foundry-extract-*")
		if err != nil {
			return fmt.Errorf("create extract dir: %w", err)
		}
		defer os.RemoveAll(extractDir)
		if err := unzipGo(archivePath, extractDir); err != nil {
			return fmt.Errorf("extract failed: %w", err)
		}
		exe, err := os.Executable()
		if err != nil {
			return fmt.Errorf("locate running binary: %w", err)
		}
		return installRawBinaryDarwin(extractDir, exe, relaunchAndExit, progress)
	}

	// Stage on the SAME installation filesystem so the final swap is an
	// atomic rename. os.MkdirTemp creates exclusive directory names —
	// no PID-suffixed, recycled-PID collisions — and the parent is the
	// install directory, so staging, backup, and target share one device.
	installParent := filepath.Dir(currentBundle)
	extractDir, err := os.MkdirTemp(installParent, ".foundry-staging-*")
	if err != nil {
		return fmt.Errorf("create staging dir on installation filesystem: %w", err)
	}
	defer os.RemoveAll(extractDir)

	if err := unzipGo(archivePath, extractDir); err != nil {
		return fmt.Errorf("extract failed: %w", err)
	}

	newAppPath, err := findAppBundle(extractDir)
	if err != nil {
		return fmt.Errorf("no .app bundle found in archive")
	}

	// Structural integrity: codesign --verify proves the bundle's
	// signature — applied by CI as Developer ID when credentials exist,
	// ad-hoc otherwise — is intact after extraction. This is integrity,
	// NOT Apple trust: passing implies nothing about notarization or a
	// Gatekeeper assessment.
	if out, err := exec.Command("codesign", "--verify", "--deep", "--strict", newAppPath).CombinedOutput(); err != nil {
		return fmt.Errorf("new app code signature invalid (structural check only — implies no Apple Developer ID trust and no notarization): %s: %w", strings.TrimSpace(string(out)), err)
	}

	// The main executable must be a universal binary carrying both
	// arm64 and x86_64 slices — verified by parsing Mach-O headers.
	newExe := filepath.Join(newAppPath, "Contents", "MacOS", "mbii-foundry")
	if err := verifyUniversalBinary(newExe); err != nil {
		return err
	}

	if progress != nil {
		progress(UpdateProgress{Stage: "installing", Percent: -1, Message: "Swapping app bundle…"})
	}

	// Exclusive backup directory on the same filesystem.
	backupDir, err := os.MkdirTemp(installParent, ".foundry-backup-*")
	if err != nil {
		return fmt.Errorf("create backup dir: %w", err)
	}
	backup := filepath.Join(backupDir, filepath.Base(currentBundle))
	if err := os.Rename(currentBundle, backup); err != nil {
		os.RemoveAll(backupDir)
		return fmt.Errorf("move old bundle aside: %w", err)
	}

	installErr := func() error {
		if err := os.Rename(newAppPath, currentBundle); err != nil {
			return fmt.Errorf("install new bundle: %w", err)
		}
		installedExe := filepath.Join(currentBundle, "Contents", "MacOS", "mbii-foundry")
		if _, err := os.Stat(installedExe); err != nil {
			return fmt.Errorf("new binary missing after install: %w", err)
		}
		return relaunchAndExit(installedExe)
	}()

	if installErr != nil {
		if rbErr := rollbackBundleSwap(currentBundle, newAppPath, backup); rbErr != nil {
			return fmt.Errorf("%v; ROLLBACK FAILED — previous bundle preserved at %s: %v", installErr, backup, rbErr)
		}
		if err := os.RemoveAll(backupDir); err != nil {
			return fmt.Errorf("%v (rolled back, but remove empty backup directory %s: %w)", installErr, backupDir, err)
		}
		return fmt.Errorf("%v (rolled back to previous bundle)", installErr)
	}
	if err := os.RemoveAll(backupDir); err != nil {
		return fmt.Errorf("new bundle relaunched, but remove previous bundle backup %s: %w", backup, err)
	}
	return nil
}

// rollbackBundleSwap undoes a partially-applied bundle swap: the
// half-installed bundle moves back to its staging location and the
// backup is restored into the installation slot. Both renames are
// error-checked; the first failure is returned so the caller can tell
// the user where the previous bundle lives.
func rollbackBundleSwap(currentBundle, stagedBundle, backup string) error {
	if err := os.Rename(currentBundle, stagedBundle); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("move new bundle back to staging: %w", err)
	}
	if err := os.Rename(backup, currentBundle); err != nil {
		return fmt.Errorf("restore previous bundle: %w", err)
	}
	return nil
}

// installRawBinaryDarwin handles the "not running inside a .app"
// case — swap the bare executable file in place. relaunch is injected
// so tests can simulate launch failure and verify the restore path.
// On a relaunch failure the previous binary is restored from the
// backup copyFileExec created; a failed restore is reported loudly
// with the backup location.
func installRawBinaryDarwin(extractDir, exePath string, relaunch func(string, ...string) error, progress func(UpdateProgress)) error {
	newBin, err := findBareBinary(extractDir, "mbii-foundry")
	if err != nil {
		return err
	}
	if progress != nil {
		progress(UpdateProgress{Stage: "installing", Percent: -1, Message: "Swapping binary…"})
	}
	backup, err := copyFileExec(newBin, exePath)
	if err != nil {
		return err
	}
	if err := relaunch(exePath); err != nil {
		if backup == "" {
			if removeErr := os.Remove(exePath); removeErr != nil && !os.IsNotExist(removeErr) {
				return fmt.Errorf("relaunch failed: %v; ROLLBACK FAILED — remove new binary %s: %v", err, exePath, removeErr)
			}
			return fmt.Errorf("relaunch failed (new first-install binary removed): %w", err)
		}
		if rbErr := os.Rename(backup, exePath); rbErr != nil {
			return fmt.Errorf("relaunch failed: %v; RESTORE FAILED — previous binary preserved at %s: %v", err, backup, rbErr)
		}
		return fmt.Errorf("relaunch failed (previous binary restored): %w", err)
	}
	if backup != "" {
		if err := os.Remove(backup); err != nil {
			return fmt.Errorf("new binary relaunched, but remove previous binary backup %s: %w", backup, err)
		}
	}
	return nil
}

// CPU types from mach/machine.h.
const (
	cpuArchABI64 = 0x01000000
	cpuTypeX8664 = 7 | cpuArchABI64
	cpuTypeARM64 = 12 | cpuArchABI64
)

// verifyUniversalBinary proves the executable is a Mach-O carrying
// both arm64 and x86_64 slices, which is what our release pipeline
// (lipo -create of the two GOARCH builds) is required to ship.
func verifyUniversalBinary(exePath string) error {
	archs, err := machoArchs(exePath)
	if err != nil {
		return fmt.Errorf("inspect new executable %s: %w", exePath, err)
	}
	var missing []string
	for _, want := range []string{"arm64", "x86_64"} {
		if !archs[want] {
			missing = append(missing, want)
		}
	}
	if len(missing) > 0 || len(archs) != 2 {
		return fmt.Errorf("new executable %s is not an exact arm64+x86_64 universal binary: found %v, missing %v",
			exePath, archNames(archs), missing)
	}
	return nil
}

// machoArchs reports which architectures the Mach-O file at path
// contains: one entry for a single-architecture binary, one per slice
// for a fat/universal binary. It parses headers only — nothing is
// mapped or executed.
func machoArchs(path string) (map[string]bool, error) {
	fat, err := macho.OpenFat(path)
	if err == nil {
		archs := make(map[string]bool, len(fat.Arches))
		for _, architecture := range fat.Arches {
			tableCPU := uint32(architecture.FatArchHeader.Cpu)
			sliceCPU := uint32(architecture.File.Cpu)
			if tableCPU != sliceCPU {
				closeErr := fat.Close()
				return nil, errors.Join(
					fmt.Errorf("fat table CPU %#x does not match embedded Mach-O CPU %#x", tableCPU, sliceCPU),
					wrapError("close Mach-O", closeErr),
				)
			}
			name := machoArchName(tableCPU)
			if name == "" {
				name = fmt.Sprintf("cpu-type-%#x", tableCPU)
			}
			if archs[name] {
				closeErr := fat.Close()
				return nil, errors.Join(
					fmt.Errorf("duplicate Mach-O architecture %s", name),
					wrapError("close Mach-O", closeErr),
				)
			}
			archs[name] = true
		}
		if err := fat.Close(); err != nil {
			return nil, fmt.Errorf("close Mach-O: %w", err)
		}
		return archs, nil
	}
	if !errors.Is(err, macho.ErrNotFat) {
		return nil, fmt.Errorf("parse universal Mach-O: %w", err)
	}

	thin, err := macho.Open(path)
	if err != nil {
		return nil, fmt.Errorf("parse Mach-O: %w", err)
	}
	name := machoArchName(uint32(thin.Cpu))
	if name == "" {
		name = fmt.Sprintf("cpu-type-%#x", uint32(thin.Cpu))
	}
	if err := thin.Close(); err != nil {
		return nil, fmt.Errorf("close Mach-O: %w", err)
	}
	return map[string]bool{name: true}, nil
}

func machoArchName(cputype uint32) string {
	switch cputype {
	case cpuTypeX8664:
		return "x86_64"
	case cpuTypeARM64:
		return "arm64"
	}
	return ""
}

func archNames(archs map[string]bool) []string {
	names := make([]string, 0, len(archs))
	for name := range archs {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// unzipGo extracts archivePath into destDir with strict validation:
//
//   - no path traversal ("..", absolute paths) and no backslash or
//     control-character names;
//   - no symlinks and no setuid/setgid/sticky modes;
//   - duplicate entries rejected under case-folding + NFC
//     normalization, so "Icon.png", "icon.PNG", and an NFD-spelled
//     variant cannot alias each other on case-insensitive filesystems;
//   - per-entry and total size caps;
//   - every member is read to full EOF, its byte count checked against
//     the declared size, and its CRC32 verified — a truncated or
//     corrupt payload fails the extraction instead of being tolerated.
func unzipGo(archivePath, destDir string) (resultErr error) {
	r, err := zip.OpenReader(archivePath)
	if err != nil {
		return err
	}
	defer func() {
		resultErr = errors.Join(resultErr, wrapError("close zip archive", r.Close()))
	}()

	const maxEntries = 1 << 16
	if len(r.File) > maxEntries {
		return fmt.Errorf("archive has too many entries (%d > %d)", len(r.File), maxEntries)
	}
	var totalExtracted uint64
	seen := make(map[string]bool, len(r.File))
	canonicalNames := make(map[*zip.File]string, len(r.File))

	for _, f := range r.File {
		isDir := f.FileInfo().IsDir()
		canonicalName, err := validateArchiveMemberPath(f.Name, isDir)
		if err != nil {
			return fmt.Errorf("illegal path in zip %q: %w", f.Name, err)
		}
		canonicalNames[f] = canonicalName
		key := archiveEntryKey(canonicalName)
		if seen[key] {
			return fmt.Errorf("duplicate entry in archive (case/normalization alias): %s", f.Name)
		}
		seen[key] = true

		mode := f.Mode()
		if mode&(os.ModeSymlink|os.ModeSetuid|os.ModeSetgid|os.ModeSticky) != 0 ||
			(!isDir && !mode.IsRegular()) {
			return fmt.Errorf("unsafe file mode in archive: %s (%s)", f.Name, mode)
		}
		if f.UncompressedSize64 > uint64(updatemanifest.MaxArtifactBytes) {
			return fmt.Errorf("file too large in zip: %s (%d bytes)", f.Name, f.UncompressedSize64)
		}
		if f.UncompressedSize64 > uint64(updatemanifest.MaxArtifactBytes)-totalExtracted {
			return fmt.Errorf("archive extraction too large (exceeds %d bytes)", updatemanifest.MaxArtifactBytes)
		}
		totalExtracted += f.UncompressedSize64
	}

	for _, f := range r.File {
		fpath := filepath.Join(destDir, filepath.FromSlash(canonicalNames[f]))
		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(fpath, f.Mode().Perm()|0700); err != nil {
				return fmt.Errorf("create dir %s: %w", f.Name, err)
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(fpath), 0755); err != nil {
			return fmt.Errorf("create parent dir for %s: %w", f.Name, err)
		}
		if err := extractZipFile(f, fpath); err != nil {
			return err
		}
	}
	return nil
}

// extractZipFile writes one member and verifies its complete byte count and
// CRC32. Both archive and destination handles are closed explicitly.
func extractZipFile(f *zip.File, fpath string) error {
	outFile, err := os.OpenFile(fpath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, f.Mode().Perm())
	if err != nil {
		return fmt.Errorf("create %s: %w", f.Name, err)
	}
	rc, err := f.Open()
	if err != nil {
		return errors.Join(
			fmt.Errorf("open %s: %w", f.Name, err),
			wrapError("close "+f.Name, outFile.Close()),
		)
	}

	crc := crc32.NewIEEE()
	n, copyErr := io.Copy(io.MultiWriter(outFile, crc), rc)
	readCloseErr := rc.Close()
	syncErr := outFile.Sync()
	writeCloseErr := outFile.Close()
	if copyErr != nil || readCloseErr != nil || syncErr != nil || writeCloseErr != nil {
		return errors.Join(
			wrapError("extract "+f.Name, copyErr),
			wrapError("close compressed "+f.Name, readCloseErr),
			wrapError("sync "+f.Name, syncErr),
			wrapError("close "+f.Name, writeCloseErr),
		)
	}
	if uint64(n) != f.UncompressedSize64 {
		return fmt.Errorf("extract %s: truncated payload: read %d bytes, header declares %d",
			f.Name, n, f.UncompressedSize64)
	}
	if crc.Sum32() != f.CRC32 {
		return fmt.Errorf("extract %s: CRC32 mismatch (archive corrupt or forged)", f.Name)
	}
	return nil
}

func findAppBundle(root string) (string, error) {
	var found string
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() && filepath.Ext(path) == ".app" {
			found = path
			return filepath.SkipDir
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	if found == "" {
		return "", errors.New("no .app bundle in archive")
	}
	return found, nil
}

func findBareBinary(root, name string) (string, error) {
	var found string
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && filepath.Base(path) == name {
			found = path
			return filepath.SkipAll
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	if found == "" {
		return "", fmt.Errorf("no %s in archive", name)
	}
	return found, nil
}

// copyFileExec copies src → dst preserving exec bits and returns the
// path of the exclusive backup it created ("" on first install, when
// no previous binary existed). os.Rename would fail across devices on
// weird /tmp setups, hence copy + atomic rename on the destination
// filesystem.
//
// The backup name is reserved with os.CreateTemp and the old binary is
// renamed ONTO the reserved placeholder in a single atomic operation —
// the reservation is never deleted first, so no other process can
// claim the name between reservation and use, and the rename can never
// clobber a foreign file.
func copyFileExec(src, dst string) (string, error) {
	in, err := os.Open(src)
	if err != nil {
		return "", err
	}
	info, err := in.Stat()
	if err != nil {
		return "", errors.Join(err, wrapError("close new executable", in.Close()))
	}
	if !info.Mode().IsRegular() {
		return "", errors.Join(
			fmt.Errorf("new executable is not a regular file: %s", src),
			wrapError("close new executable", in.Close()),
		)
	}

	dstDir := filepath.Dir(dst)
	tmpFile, err := os.CreateTemp(dstDir, ".foundry-newexe-*")
	if err != nil {
		return "", errors.Join(err, wrapError("close new executable", in.Close()))
	}
	tmpDst := tmpFile.Name()
	if err := tmpFile.Chmod(info.Mode().Perm() | 0111); err != nil {
		return "", errors.Join(
			err,
			wrapError("close staged executable", tmpFile.Close()),
			wrapError("close new executable", in.Close()),
			wrapError("remove staged executable", os.Remove(tmpDst)),
		)
	}
	_, copyErr := io.Copy(tmpFile, in)
	syncErr := tmpFile.Sync()
	tmpCloseErr := tmpFile.Close()
	inCloseErr := in.Close()
	if copyErr != nil || syncErr != nil || tmpCloseErr != nil || inCloseErr != nil {
		return "", errors.Join(
			wrapError("copy new executable", copyErr),
			wrapError("sync staged executable", syncErr),
			wrapError("close staged executable", tmpCloseErr),
			wrapError("close new executable", inCloseErr),
			wrapError("remove staged executable", os.Remove(tmpDst)),
		)
	}

	backupFile, err := os.CreateTemp(dstDir, ".foundry-oldexe-*")
	if err != nil {
		return "", errors.Join(err, wrapError("remove staged executable", os.Remove(tmpDst)))
	}
	backupDst := backupFile.Name()
	if err := backupFile.Close(); err != nil {
		return "", errors.Join(
			fmt.Errorf("close binary backup reservation: %w", err),
			wrapError("remove backup reservation", os.Remove(backupDst)),
			wrapError("remove staged executable", os.Remove(tmpDst)),
		)
	}
	if err := os.Rename(dst, backupDst); err != nil {
		removeBackupErr := os.Remove(backupDst)
		if !os.IsNotExist(err) {
			return "", errors.Join(
				fmt.Errorf("backup existing executable: %w", err),
				wrapError("remove backup reservation", removeBackupErr),
				wrapError("remove staged executable", os.Remove(tmpDst)),
			)
		}
		if removeBackupErr != nil && !os.IsNotExist(removeBackupErr) {
			return "", errors.Join(
				fmt.Errorf("release unused backup reservation: %w", removeBackupErr),
				wrapError("remove staged executable", os.Remove(tmpDst)),
			)
		}
		backupDst = ""
	}
	if err := os.Rename(tmpDst, dst); err != nil {
		removeErr := os.Remove(tmpDst)
		if backupDst != "" {
			if rbErr := os.Rename(backupDst, dst); rbErr != nil {
				return "", errors.Join(
					fmt.Errorf("swap failed: %w; RESTORE FAILED — previous binary preserved at %s", err, backupDst),
					rbErr,
					wrapError("remove staged executable", removeErr),
				)
			}
			return "", errors.Join(
				fmt.Errorf("swap executable (previous binary restored): %w", err),
				wrapError("remove staged executable", removeErr),
			)
		}
		return "", errors.Join(fmt.Errorf("swap executable: %w", err), wrapError("remove staged executable", removeErr))
	}
	return backupDst, nil
}
