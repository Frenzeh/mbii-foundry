//go:build linux

package main

// Linux auto-installer.
//
// Release artifact is a tar.gz containing mbii-foundry + docs + data
// directories. Process:
//  1. Download the tar.gz and verify the Ed25519-signed manifest
//     (downloadToTemp).
//  2. Extract with the Go stdlib. untarGz rejects path traversal,
//     symlinks, hardlinks, device/fifo members, duplicate or
//     case/normalization-aliased names, oversized members, and
//     truncated member payloads.
//  3. Before mutating the install, build complete merged replacements for all
//     four required resource directories in an exclusive transaction directory
//     on the installation filesystem.
//  4. Atomically swap the binary and each resource directory while retaining
//     every previous object. Any swap, durability, or relaunch failure rolls all
//     prior swaps back. Rollback failures preserve and report recovery paths.
//
// A release missing data, definitions, schemas, or templates is rejected before
// the binary changes. Existing extra resource files are preserved by staging a
// merge, but no installed tree is ever updated in place.

import (
	"archive/tar"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/Frenzeh/mbii-foundry/updatemanifest"
)

var linuxReleaseDataDirs = []string{"data", "definitions", "schemas", "templates"}

type linuxResourceSwap struct {
	name      string
	target    string
	staged    string
	backup    string
	oldMoved  bool
	installed bool
}

func installUpdatePlatform(asset *ReleaseAsset, manifest UpdateManifest, progress func(UpdateProgress)) error {
	if !strings.HasSuffix(strings.ToLower(asset.Name), ".tar.gz") {
		return fmt.Errorf("expected .tar.gz asset for Linux, got %s", asset.Name)
	}
	archivePath, cleanup, err := downloadToTemp(asset, manifest, progress)
	if err != nil {
		return err
	}
	defer cleanup()

	if progress != nil {
		progress(UpdateProgress{Stage: "extracting", Percent: -1, Message: "Extracting update…"})
	}
	extractDir, err := os.MkdirTemp(filepath.Dir(archivePath), "extracted-*")
	if err != nil {
		return fmt.Errorf("create extract dir: %w", err)
	}
	defer os.RemoveAll(extractDir)
	if err := untarGz(archivePath, extractDir); err != nil {
		return fmt.Errorf("extract tarball: %w", err)
	}
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("locate running binary: %w", err)
	}
	return installLinuxPayload(extractDir, exe, relaunchAndExit, progress)
}

func installLinuxPayload(
	extractDir, exe string,
	relaunch func(string, ...string) error,
	progress func(UpdateProgress),
) (resultErr error) {
	newBin, err := findBareBinaryLinux(extractDir, "mbii-foundry")
	if err != nil {
		return err
	}
	installDir := filepath.Dir(exe)
	transactionDir, err := os.MkdirTemp(installDir, ".foundry-transaction-*")
	if err != nil {
		return fmt.Errorf("create install transaction: %w", err)
	}
	preserveTransaction := false
	defer func() {
		if !preserveTransaction {
			if err := os.RemoveAll(transactionDir); err != nil {
				resultErr = errors.Join(resultErr, fmt.Errorf("remove install transaction %s: %w", transactionDir, err))
			}
		}
	}()

	swaps, err := stageLinuxResourceDirs(extractDir, installDir, transactionDir, progress)
	if err != nil {
		return err
	}
	if progress != nil {
		progress(UpdateProgress{Stage: "installing", Percent: -1, Message: "Swapping binary and resources…"})
	}
	execBackup, err := copyFileExecLinux(newBin, exe)
	if err != nil {
		return err
	}

	rollback := func(cause error) error {
		err, preserve := rollbackLinuxInstall(swaps, execBackup, exe, transactionDir, cause)
		preserveTransaction = preserve
		return err
	}
	for i := range swaps {
		if err := applyLinuxResourceSwap(&swaps[i]); err != nil {
			return rollback(err)
		}
	}
	if err := syncDirectory(installDir); err != nil {
		return rollback(fmt.Errorf("sync installation directory: %w", err))
	}
	if err := relaunch(exe); err != nil {
		return rollback(fmt.Errorf("relaunch updated executable: %w", err))
	}

	if execBackup != "" {
		if err := os.Remove(execBackup); err != nil {
			preserveTransaction = true
			return fmt.Errorf("update relaunched but remove binary backup %s: %w", execBackup, err)
		}
	}
	return nil
}

func stageLinuxResourceDirs(extractDir, installDir, transactionDir string, progress func(UpdateProgress)) ([]linuxResourceSwap, error) {
	swaps := make([]linuxResourceSwap, 0, len(linuxReleaseDataDirs))
	for _, name := range linuxReleaseDataDirs {
		source := filepath.Join(extractDir, name)
		info, err := os.Lstat(source)
		if err != nil {
			if os.IsNotExist(err) {
				return nil, fmt.Errorf("release is missing required resource directory %s", name)
			}
			return nil, fmt.Errorf("inspect release resource directory %s: %w", name, err)
		}
		if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
			return nil, fmt.Errorf("release resource path %s is not a real directory", name)
		}
		target := filepath.Join(installDir, name)
		if current, err := os.Lstat(target); err == nil {
			if current.Mode()&os.ModeSymlink != 0 || !current.IsDir() {
				return nil, fmt.Errorf("installed resource path %s is not a real directory", target)
			}
		} else if !os.IsNotExist(err) {
			return nil, fmt.Errorf("inspect installed resource directory %s: %w", target, err)
		}
	}

	for _, name := range linuxReleaseDataDirs {
		if progress != nil {
			progress(UpdateProgress{
				Stage:   "installing",
				Percent: -1,
				Message: fmt.Sprintf("Staging %s/…", name),
			})
		}
		target := filepath.Join(installDir, name)
		staged := filepath.Join(transactionDir, "staged-"+name)
		backup := filepath.Join(transactionDir, "backup-"+name)
		if err := os.Mkdir(staged, 0755); err != nil {
			return nil, fmt.Errorf("create staged %s directory: %w", name, err)
		}
		if _, err := os.Lstat(target); err == nil {
			if err := copyTreeInto(target, staged); err != nil {
				return nil, fmt.Errorf("stage installed %s: %w", name, err)
			}
		}
		if err := copyTreeInto(filepath.Join(extractDir, name), staged); err != nil {
			return nil, fmt.Errorf("stage release %s: %w", name, err)
		}
		if err := syncTree(staged); err != nil {
			return nil, fmt.Errorf("sync staged %s: %w", name, err)
		}
		swaps = append(swaps, linuxResourceSwap{
			name: name, target: target, staged: staged, backup: backup,
		})
	}
	if err := syncDirectory(transactionDir); err != nil {
		return nil, fmt.Errorf("sync install transaction: %w", err)
	}
	return swaps, nil
}

func applyLinuxResourceSwap(swap *linuxResourceSwap) error {
	if _, err := os.Lstat(swap.target); err == nil {
		if err := os.Rename(swap.target, swap.backup); err != nil {
			return fmt.Errorf("backup installed %s at %s: %w", swap.name, swap.backup, err)
		}
		swap.oldMoved = true
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("inspect installed %s: %w", swap.name, err)
	}
	if err := os.Rename(swap.staged, swap.target); err != nil {
		return fmt.Errorf("install staged %s: %w", swap.name, err)
	}
	swap.installed = true
	return nil
}

func rollbackLinuxInstall(swaps []linuxResourceSwap, execBackup, exe, transactionDir string, cause error) (error, bool) {
	var rollbackErrors []error
	for i := len(swaps) - 1; i >= 0; i-- {
		swap := &swaps[i]
		currentMoved := !swap.installed
		if swap.installed {
			if err := os.Rename(swap.target, swap.staged); err != nil {
				rollbackErrors = append(rollbackErrors, fmt.Errorf("move new %s out of install slot: %w", swap.name, err))
			} else {
				currentMoved = true
			}
		}
		if swap.oldMoved && currentMoved {
			if err := os.Rename(swap.backup, swap.target); err != nil {
				rollbackErrors = append(rollbackErrors, fmt.Errorf("restore %s from backup %s: %w", swap.name, swap.backup, err))
			}
		}
	}
	retainedExecBackup := ""
	if execBackup == "" {
		if err := os.Remove(exe); err != nil && !os.IsNotExist(err) {
			rollbackErrors = append(rollbackErrors, fmt.Errorf("remove newly installed binary %s: %w", exe, err))
		}
	} else if err := os.Rename(execBackup, exe); err != nil {
		retainedExecBackup = execBackup
		rollbackErrors = append(rollbackErrors, fmt.Errorf("restore binary from backup %s: %w", execBackup, err))
	}
	if err := syncDirectory(filepath.Dir(exe)); err != nil {
		rollbackErrors = append(rollbackErrors, fmt.Errorf("sync rollback: %w", err))
	}
	if len(rollbackErrors) == 0 {
		return fmt.Errorf("%v (previous binary and resources restored)", cause), false
	}
	var resourceBackups []string
	for _, swap := range swaps {
		if swap.oldMoved {
			if _, err := os.Lstat(swap.backup); err == nil {
				resourceBackups = append(resourceBackups, swap.backup)
			}
		}
	}
	return errors.Join(
		fmt.Errorf(
			"%v; ROLLBACK FAILED — transaction retained at %s; retained resource backups: %s; retained binary backup: %s",
			cause, transactionDir, strings.Join(resourceBackups, ", "), retainedExecBackup,
		),
		errors.Join(rollbackErrors...),
	), true
}

func copyTreeInto(source, destination string) error {
	return filepath.WalkDir(source, func(current string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(source, current)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		target := filepath.Join(destination, rel)
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("symlink not allowed while staging: %s", current)
		}
		if entry.IsDir() {
			if existing, err := os.Lstat(target); err == nil && !existing.IsDir() {
				if err := os.RemoveAll(target); err != nil {
					return err
				}
			}
			return os.MkdirAll(target, info.Mode().Perm()|0700)
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("non-regular resource file not allowed: %s", current)
		}
		if existing, err := os.Lstat(target); err == nil && existing.IsDir() {
			if err := os.RemoveAll(target); err != nil {
				return err
			}
		}
		if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
			return err
		}
		return copyFileLinuxStaged(current, target, info.Mode().Perm())
	})
}

func copyFileLinuxStaged(source, destination string, mode os.FileMode) (resultErr error) {
	in, err := os.Open(source)
	if err != nil {
		return err
	}
	defer func() {
		resultErr = errors.Join(resultErr, wrapError("close staged source", in.Close()))
	}()
	out, err := os.OpenFile(destination, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(out, in)
	syncErr := out.Sync()
	closeErr := out.Close()
	return errors.Join(copyErr, syncErr, closeErr)
}

func syncTree(root string) error {
	return filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil || !entry.IsDir() {
			return err
		}
		return syncDirectory(path)
	})
}

func syncDirectory(path string) error {
	dir, err := os.Open(path)
	if err != nil {
		return err
	}
	syncErr := dir.Sync()
	closeErr := dir.Close()
	return errors.Join(syncErr, closeErr)
}

// untarGz extracts archivePath into destDir with strict validation:
//
//   - no path traversal (".." components, absolute paths);
//   - no symlinks, hardlinks, or device/fifo members;
//   - duplicate entries rejected under case-folding + NFC
//     normalization (defense for case-insensitive extraction targets);
//   - per-member and total size caps;
//   - every member read to its full declared size — a truncated
//     member payload is an error, never tolerated.
func untarGz(archivePath, destDir string) (resultErr error) {
	f, err := os.Open(archivePath)
	if err != nil {
		return err
	}
	gz, err := gzip.NewReader(f)
	if err != nil {
		return errors.Join(err, wrapError("close tarball", f.Close()))
	}
	defer func() {
		resultErr = errors.Join(
			resultErr,
			wrapError("close gzip stream", gz.Close()),
			wrapError("close tarball", f.Close()),
		)
	}()
	tr := tar.NewReader(gz)

	const maxArchiveEntries = 1 << 16
	var totalExtracted int64
	entryCount := 0
	seen := make(map[string]bool)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			if _, err := io.Copy(io.Discard, gz); err != nil {
				return fmt.Errorf("finish gzip stream: %w", err)
			}
			return nil
		}
		if err != nil {
			return err
		}
		entryCount++
		if entryCount > maxArchiveEntries {
			return fmt.Errorf("archive has too many entries (more than %d)", maxArchiveEntries)
		}

		isDir := hdr.Typeflag == tar.TypeDir
		canonicalName, err := validateArchiveMemberPath(hdr.Name, isDir)
		if err != nil {
			return fmt.Errorf("unsafe path in tar %q: %w", hdr.Name, err)
		}
		key := archiveEntryKey(canonicalName)
		if seen[key] {
			return fmt.Errorf("duplicate entry in archive (case/normalization alias): %s", hdr.Name)
		}
		seen[key] = true
		if hdr.Mode&07000 != 0 {
			return fmt.Errorf("unsafe file mode in tar: %s (%#o)", hdr.Name, hdr.Mode)
		}
		if hdr.Size < 0 || hdr.Size > updatemanifest.MaxArtifactBytes {
			return fmt.Errorf("tar member %s size invalid or too large: %d", hdr.Name, hdr.Size)
		}
		if hdr.Size > updatemanifest.MaxArtifactBytes-totalExtracted {
			return fmt.Errorf("tar extraction exceeded total limit of %d bytes", updatemanifest.MaxArtifactBytes)
		}

		target := filepath.Join(destDir, filepath.FromSlash(canonicalName))
		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, os.FileMode(hdr.Mode).Perm()|0700); err != nil {
				return fmt.Errorf("create directory %s: %w", hdr.Name, err)
			}
		case tar.TypeReg, tar.TypeRegA:
			if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
				return fmt.Errorf("create parent dir for %s: %w", hdr.Name, err)
			}
			out, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_EXCL, os.FileMode(hdr.Mode).Perm())
			if err != nil {
				return fmt.Errorf("create tar member %s: %w", hdr.Name, err)
			}
			n, copyErr := io.Copy(out, io.LimitReader(tr, hdr.Size+1))
			syncErr := out.Sync()
			closeErr := out.Close()
			if errors.Is(copyErr, io.ErrUnexpectedEOF) || errors.Is(copyErr, io.EOF) {
				return errors.Join(
					fmt.Errorf("tar member %s truncated: read %d of %d bytes: %w", hdr.Name, n, hdr.Size, copyErr),
					wrapError("sync "+hdr.Name, syncErr),
					wrapError("close "+hdr.Name, closeErr),
				)
			}
			if copyErr != nil || syncErr != nil || closeErr != nil {
				return errors.Join(
					wrapError("extract "+hdr.Name, copyErr),
					wrapError("sync "+hdr.Name, syncErr),
					wrapError("close "+hdr.Name, closeErr),
				)
			}
			if n != hdr.Size {
				return fmt.Errorf("tar member %s truncated: read %d of %d bytes", hdr.Name, n, hdr.Size)
			}
			totalExtracted += n
		case tar.TypeSymlink, tar.TypeLink:
			return fmt.Errorf("links not allowed in archive: %s", hdr.Name)
		default:
			return fmt.Errorf("unsupported tar member type %q: %s", string(rune(hdr.Typeflag)), hdr.Name)
		}
	}
}

func findBareBinaryLinux(root, name string) (string, error) {
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

// copyFileExecLinux copies src → dst preserving exec bits and returns
// the path of the exclusive backup it created ("" on first install).
//
// The backup name is reserved with os.CreateTemp and the old binary is
// renamed ONTO the reserved placeholder in a single atomic operation —
// the reservation is never deleted first, so no other process can
// claim the name between reservation and use, and the rename can never
// clobber a foreign file. All rollback renames are error-checked.
func copyFileExecLinux(src, dst string) (string, error) {
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
