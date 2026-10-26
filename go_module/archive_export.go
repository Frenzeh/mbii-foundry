package main

import (
	"archive/zip"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/Frenzeh/mbii-foundry/safeio"
)

type ExportEntry struct {
	SourcePath  string
	ArchivePath string
}

func BuildExportManifest(source, destination string) ([]ExportEntry, error) {
	var manifest []ExportEntry
	absSource, err := filepath.Abs(source)
	if err != nil {
		return nil, fmt.Errorf("get absolute source: %w", err)
	}
	absDest, err := filepath.Abs(destination)
	if err != nil {
		return nil, fmt.Errorf("get absolute dest: %w", err)
	}

	err = filepath.WalkDir(absSource, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		isRoot := path == absSource
		if path == absDest {
			return nil
		}

		name := d.Name()
		// Allow hidden root, but prune hidden descendants
		if !isRoot && strings.HasPrefix(name, ".") {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		info, err := d.Info()
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("symlink not allowed: %s", path)
		}
		if d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(absSource, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if strings.HasPrefix(rel, "../") || rel == ".." || filepath.IsAbs(rel) {
			return fmt.Errorf("path escapes root: %s", rel)
		}
		manifest = append(manifest, ExportEntry{
			SourcePath:  path,
			ArchivePath: rel,
		})
		return nil
	})
	return manifest, err
}

func WritePK3(source, destination string, manifest []ExportEntry) error {
	if len(manifest) == 0 {
		return errors.New("cannot write PK3 with an empty manifest")
	}

	absSource, err := filepath.Abs(source)
	if err != nil {
		return fmt.Errorf("invalid source: %w", err)
	}
	sourceInfo, err := os.Lstat(absSource)
	if err != nil {
		return fmt.Errorf("inspect source root: %w", err)
	}
	if sourceInfo.Mode()&os.ModeSymlink != 0 || !sourceInfo.IsDir() {
		return fmt.Errorf("source root must be a real directory: %s", absSource)
	}
	absDest, err := filepath.Abs(destination)
	if err != nil {
		return fmt.Errorf("invalid destination: %w", err)
	}

	type validatedEntry struct {
		source  string
		archive string
	}
	entries := make([]validatedEntry, 0, len(manifest))
	seen := make(map[string]string, len(manifest))
	for _, entry := range manifest {
		archivePath, err := validateStrictSlashPath(entry.ArchivePath)
		if err != nil {
			return fmt.Errorf("invalid archive path %q: %w", entry.ArchivePath, err)
		}
		key := archiveEntryKey(archivePath)
		if previous, ok := seen[key]; ok {
			return fmt.Errorf("duplicate archive path (case/normalization alias): %q aliases %q", archivePath, previous)
		}
		seen[key] = archivePath

		sourcePath, err := sourcePathWithinRoot(absSource, entry.SourcePath)
		if err != nil {
			return err
		}
		entryAbs := filepath.Join(absSource, filepath.FromSlash(sourcePath))
		if entryAbs == absDest {
			return fmt.Errorf("cannot include output file in archive: %s", sourcePath)
		}
		entries = append(entries, validatedEntry{source: sourcePath, archive: archivePath})
	}

	return safeio.AtomicWrite(destination, 0644, func(w io.Writer) (writeErr error) {
		root, err := os.OpenRoot(absSource)
		if err != nil {
			return fmt.Errorf("open source root: %w", err)
		}
		defer func() {
			if err := root.Close(); err != nil {
				writeErr = errors.Join(writeErr, fmt.Errorf("close source root: %w", err))
			}
		}()

		zw := zip.NewWriter(w)
		defer func() {
			if err := zw.Close(); err != nil {
				writeErr = errors.Join(writeErr, fmt.Errorf("close zip archive: %w", err))
			}
		}()

		var destinationInfo fs.FileInfo
		if info, err := os.Stat(absDest); err == nil {
			destinationInfo = info
		} else if !os.IsNotExist(err) {
			return fmt.Errorf("stat destination: %w", err)
		}

		for _, entry := range entries {
			if err := validateRootFile(root, entry.source, destinationInfo); err != nil {
				return err
			}
			if err := writePK3Entry(root, zw, entry.source, entry.archive); err != nil {
				return err
			}
		}
		return nil
	})
}

// validateStrictSlashPath accepts only a canonical, relative slash path.
// Rejecting dot components even when path.Clean would remove them prevents two
// spellings from identifying the same archive member.
func validateStrictSlashPath(name string) (string, error) {
	if name == "" {
		return "", errors.New("path is empty")
	}
	if strings.ContainsRune(name, '\\') {
		return "", errors.New("backslashes are not allowed")
	}
	for _, r := range name {
		if r < 0x20 || r == 0x7f {
			return "", errors.New("control characters are not allowed")
		}
	}
	if path.IsAbs(name) || hasWindowsDrivePrefix(name) {
		return "", errors.New("absolute paths are not allowed")
	}
	for _, component := range strings.Split(name, "/") {
		if component == "" || component == "." || component == ".." {
			return "", fmt.Errorf("invalid path component %q", component)
		}
	}
	if clean := path.Clean(name); clean != name {
		return "", fmt.Errorf("path is not canonical (clean form %q)", clean)
	}
	return name, nil
}

func hasWindowsDrivePrefix(name string) bool {
	if len(name) < 2 || name[1] != ':' {
		return false
	}
	first := name[0]
	return first >= 'a' && first <= 'z' || first >= 'A' && first <= 'Z'
}

func validateArchiveMemberPath(name string, directory bool) (string, error) {
	canonical := name
	if directory && strings.HasSuffix(canonical, "/") {
		canonical = strings.TrimSuffix(canonical, "/")
		if strings.HasSuffix(canonical, "/") {
			return "", errors.New("repeated trailing slashes are not allowed")
		}
	} else if !directory && strings.HasSuffix(canonical, "/") {
		return "", errors.New("regular file path has a trailing slash")
	}
	return validateStrictSlashPath(canonical)
}

func sourcePathWithinRoot(absRoot, sourcePath string) (string, error) {
	if sourcePath == "" {
		return "", errors.New("invalid source path: path is empty")
	}
	if filepath.IsAbs(sourcePath) {
		if filepath.Clean(sourcePath) != sourcePath {
			return "", fmt.Errorf("invalid source path %q: path is not canonical", sourcePath)
		}
		rel, err := filepath.Rel(absRoot, sourcePath)
		if err != nil {
			return "", fmt.Errorf("make source path relative: %w", err)
		}
		sourcePath = filepath.ToSlash(rel)
	}
	clean, err := validateStrictSlashPath(sourcePath)
	if err != nil {
		return "", fmt.Errorf("invalid source path %q: %w", sourcePath, err)
	}
	return clean, nil
}

func validateRootFile(root *os.Root, sourcePath string, destinationInfo fs.FileInfo) error {
	var prefix string
	for _, component := range strings.Split(sourcePath, "/") {
		if prefix == "" {
			prefix = component
		} else {
			prefix += "/" + component
		}
		info, err := root.Lstat(prefix)
		if err != nil {
			return fmt.Errorf("lstat source %s: %w", sourcePath, err)
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("symlink not allowed in source path: %s", sourcePath)
		}
		if prefix != sourcePath && !info.IsDir() {
			return fmt.Errorf("non-directory source path component: %s", prefix)
		}
		if prefix == sourcePath {
			if !info.Mode().IsRegular() {
				return fmt.Errorf("non-regular file not allowed: %s", sourcePath)
			}
			if destinationInfo != nil && os.SameFile(info, destinationInfo) {
				return fmt.Errorf("cannot include output file in archive: %s", sourcePath)
			}
		}
	}
	return nil
}

func writePK3Entry(root *os.Root, zw *zip.Writer, sourcePath, archivePath string) (err error) {
	f, err := root.Open(sourcePath)
	if err != nil {
		return fmt.Errorf("open source %s: %w", sourcePath, err)
	}
	defer func() {
		if closeErr := f.Close(); closeErr != nil {
			err = errors.Join(err, fmt.Errorf("close source %s: %w", sourcePath, closeErr))
		}
	}()

	info, err := f.Stat()
	if err != nil {
		return fmt.Errorf("stat source %s: %w", sourcePath, err)
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("non-regular file not allowed: %s", sourcePath)
	}
	header, err := zip.FileInfoHeader(info)
	if err != nil {
		return fmt.Errorf("create zip header for %s: %w", sourcePath, err)
	}
	header.Name = archivePath
	header.Method = zip.Deflate
	writer, err := zw.CreateHeader(header)
	if err != nil {
		return fmt.Errorf("create zip entry %s: %w", archivePath, err)
	}
	if _, err := io.Copy(writer, f); err != nil {
		return fmt.Errorf("write zip entry %s: %w", archivePath, err)
	}
	return nil
}
