//go:build linux

package main

// Linux auto-installer.
//
// Release artifact is a tar.gz containing mbii-foundry + docs + data
// directories. Process:
//   1. Download the tar.gz.
//   2. Extract into a temp dir.
//   3. Copy the new binary over the running executable. Linux is
//      happy to let us overwrite a running ELF because the kernel
//      keeps the inode around until the process exits — the in-
//      memory copy keeps running, and the next exec picks up the
//      new bytes.
//   4. chmod +x and relaunch the new binary, exit the old one.
//
// The data/definitions/schemas/templates directories that ship in
// the tarball are intentionally NOT copied here — Foundry reads those
// from its install dir, and the user may have their own tree next to
// the binary. If the binary is the only file in the install dir we
// leave the shipped-data handling to a future pass (mention it in
// the commit so it's on the radar).

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

func installUpdatePlatform(asset *ReleaseAsset, progress func(UpdateProgress)) error {
	if !strings.HasSuffix(strings.ToLower(asset.Name), ".tar.gz") {
		return fmt.Errorf("expected .tar.gz asset for Linux, got %s", asset.Name)
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
	if err := untarGz(archivePath, extractDir); err != nil {
		return fmt.Errorf("extract tarball: %w", err)
	}

	newBin, err := findBareBinaryLinux(extractDir, "mbii-foundry")
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

	if err := copyFileExecLinux(newBin, exe); err != nil {
		return err
	}
	if err := os.Chmod(exe, 0755); err != nil {
		return fmt.Errorf("chmod %s: %w", exe, err)
	}
	return relaunchAndExit(exe)
}

func untarGz(archivePath, destDir string) error {
	f, err := os.Open(archivePath)
	if err != nil {
		return err
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		return err
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		target := filepath.Join(destDir, hdr.Name)
		// Defense against path-traversal in crafted archives.
		rel, err := filepath.Rel(destDir, target)
		if err != nil || strings.HasPrefix(rel, "..") {
			return fmt.Errorf("unsafe path in tar: %s", hdr.Name)
		}
		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, os.FileMode(hdr.Mode)|0700); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
				return err
			}
			out, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, os.FileMode(hdr.Mode))
			if err != nil {
				return err
			}
			if _, err := io.Copy(out, tr); err != nil {
				out.Close()
				return err
			}
			out.Close()
		case tar.TypeSymlink:
			// Ignored — Foundry's Linux tarball shouldn't contain any.
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

func copyFileExecLinux(src, dst string) error {
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
