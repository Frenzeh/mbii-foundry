//go:build darwin

package main

// Mac auto-installer.
//
// Release artifact is a .zip containing "MBII Foundry.app". Process:
//   1. Download the zip to a temp dir.
//   2. Unzip it via /usr/bin/unzip (preserves symlinks + extended
//      attrs reliably; archive/zip misses some of that).
//   3. Strip the com.apple.quarantine xattr off the new .app so
//      Gatekeeper doesn't re-flag it on relaunch.
//   4. ad-hoc re-sign the bundle (CI already did, but the act of
//      unzipping can invalidate it on some macOS versions).
//   5. Move the running .app out of the way, move the new one in,
//      relaunch the new app binary, exit.
//
// If the app isn't inside a .app bundle (dev run, or someone running
// the raw binary), the installer falls back to a raw-binary swap of
// the executable path.

import (
	"archive/zip"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func installUpdatePlatform(asset *ReleaseAsset, progress func(UpdateProgress)) error {
	if !strings.HasSuffix(strings.ToLower(asset.Name), ".zip") {
		return fmt.Errorf("expected .zip asset for macOS, got %s", asset.Name)
	}

	archivePath, cleanup, err := downloadToTemp(asset, progress)
	if err != nil {
		return err
	}
	defer cleanup()

	if progress != nil {
		progress(UpdateProgress{Stage: "extracting", Percent: -1, Message: "Extracting update…"})
	}
	extractDir := filepath.Join(filepath.Dir(archivePath), "extracted")
	if err := os.MkdirAll(extractDir, 0755); err != nil {
		return fmt.Errorf("create extract dir: %w", err)
	}

	if err := unzipWithSystem(archivePath, extractDir); err != nil {
		// Fall back to Go's archive/zip — works for simple payloads,
		// even if it misses some exotic bits.
		if zipErr := unzipGo(archivePath, extractDir); zipErr != nil {
			return fmt.Errorf("extract failed: unzip %v / archive/zip %v", err, zipErr)
		}
	}

	// Find the .app inside the extracted tree.
	newAppPath, err := findAppBundle(extractDir)
	if err != nil {
		// No .app — look for a bare binary instead (dev-ish release).
		return installRawBinaryDarwin(extractDir, progress)
	}

	if progress != nil {
		progress(UpdateProgress{Stage: "installing", Percent: -1, Message: "Clearing quarantine…"})
	}
	// Strip quarantine on the new bundle so the relaunched app
	// doesn't pop Gatekeeper on first run. -r recurses, -s is silent
	// on missing attrs.
	_ = exec.Command("xattr", "-dr", "com.apple.quarantine", newAppPath).Run()

	// Re-sign ad-hoc so Gatekeeper's runtime checks have a stable
	// signature to validate. The CI also signs, but the zip→unzip
	// round trip can flag bundles as modified.
	_ = exec.Command("codesign", "--force", "--deep", "--sign", "-", newAppPath).Run()

	currentBundle := appBundleContainingExe()
	if currentBundle == "" {
		// Not running inside a bundle — fall back to raw-binary swap.
		return installRawBinaryDarwin(extractDir, progress)
	}

	if progress != nil {
		progress(UpdateProgress{Stage: "installing", Percent: -1, Message: "Swapping app bundle…"})
	}

	// Move current bundle aside (in case we need to roll back) and
	// move new one into place. os.Rename across devices will fail —
	// we stay inside /Applications or wherever the app lives, so
	// same-filesystem renames are safe.
	backup := currentBundle + ".old"
	_ = os.RemoveAll(backup)
	if err := os.Rename(currentBundle, backup); err != nil {
		return fmt.Errorf("move old bundle aside: %w", err)
	}
	if err := os.Rename(newAppPath, currentBundle); err != nil {
		// Restore the old bundle if the swap failed mid-way.
		_ = os.Rename(backup, currentBundle)
		return fmt.Errorf("install new bundle: %w", err)
	}
	// Best-effort cleanup of the backup. Leave it around on failure.
	_ = os.RemoveAll(backup)

	// Relaunch the new app's main binary (not the .app path — that
	// requires /usr/bin/open which spawns a separate login-context
	// process and loses our env).
	newExe := filepath.Join(currentBundle, "Contents", "MacOS", "mbii-foundry")
	if _, err := os.Stat(newExe); err != nil {
		// Last resort: use `open` to bounce through LaunchServices.
		_ = exec.Command("open", currentBundle).Start()
	} else {
		if err := relaunchAndExit(newExe); err != nil {
			return err
		}
	}
	return nil
}

// installRawBinaryDarwin handles the "not running inside a .app"
// case — swap the bare executable file in place.
func installRawBinaryDarwin(extractDir string, progress func(UpdateProgress)) error {
	newBin, err := findBareBinary(extractDir, "mbii-foundry")
	if err != nil {
		return err
	}
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("locate running binary: %w", err)
	}
	if progress != nil {
		progress(UpdateProgress{Stage: "installing", Percent: -1, Message: "Swapping binary…"})
	}
	// macOS + Linux both allow overwriting a running executable
	// because the kernel holds the inode until the process exits.
	// os.Rename works across same-device paths.
	if err := copyFileExec(newBin, exe); err != nil {
		return err
	}
	_ = exec.Command("xattr", "-d", "com.apple.quarantine", exe).Run()
	_ = exec.Command("codesign", "--force", "--sign", "-", exe).Run()
	return relaunchAndExit(exe)
}

func unzipWithSystem(archivePath, destDir string) error {
	cmd := exec.Command("/usr/bin/unzip", "-q", "-o", archivePath, "-d", destDir)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("/usr/bin/unzip failed: %w (%s)", err, strings.TrimSpace(string(out)))
	}
	return nil
}

func unzipGo(archivePath, destDir string) error {
	r, err := zip.OpenReader(archivePath)
	if err != nil {
		return err
	}
	defer r.Close()
	for _, f := range r.File {
		target := filepath.Join(destDir, f.Name)
		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(target, f.Mode()|0111); err != nil {
				return err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
			return err
		}
		out, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
		if err != nil {
			return err
		}
		rc, err := f.Open()
		if err != nil {
			out.Close()
			return err
		}
		if _, err := io.Copy(out, rc); err != nil {
			out.Close()
			rc.Close()
			return err
		}
		out.Close()
		rc.Close()
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

// copyFileExec copies src → dst preserving exec bits. Used for the
// raw-binary swap; os.Rename would fail across devices on weird
// /tmp setups.
func copyFileExec(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	info, err := in.Stat()
	if err != nil {
		return err
	}
	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, info.Mode()|0111)
	if err != nil {
		return err
	}
	defer out.Close()
	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return nil
}
