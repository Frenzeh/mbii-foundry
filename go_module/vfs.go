package main

import (
	"archive/zip"
	"image"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// AssetSource represents where an asset comes from
type AssetSource struct {
	Path        string // Logical path (e.g. "models/players/kyle/model.glm")
	EntryName   string // Exact entry name inside PK3 archive (e.g. "Models/Players/Kyle/model.glm")
	FullPath    string // Absolute path on disk (for loose files)
	PK3Path     string // Path to PK3 file (empty if loose file)
	Size        int64
	ModTime     time.Time
	CRC32       uint32 // PK3 content identity, including replacements with the same timestamp
	IsDirectory bool
}

// VirtualFileSystem manages the merged view of assets
type VirtualFileSystem struct {
	Index       map[string]*AssetSource
	Directories map[string][]*AssetSource // Map dir path to contents
	Sources     []string                  // List of loaded PK3s/folders

	GamedataPath string
	TextAssets   string

	mu         sync.RWMutex
	generation uint64
	gameIcons  map[string]image.Image
}

func NewVirtualFileSystem(gamedata, textAssets string) *VirtualFileSystem {
	vfs := &VirtualFileSystem{
		Index:        make(map[string]*AssetSource),
		Directories:  make(map[string][]*AssetSource),
		GamedataPath: gamedata,
		TextAssets:   textAssets,
	}
	return vfs
}

// Refresh rescans all assets
func (vfs *VirtualFileSystem) Refresh() error {
	// Build the new index OUTSIDE the lock — scanning 80+ PK3s
	// takes seconds, and holding the write lock for that duration
	// blocked every concurrent RLock (icon lookups, shader
	// resolver, portrait fallback) → main thread froze for the
	// entire scan. Now we operate on a local view, swap with a
	// short-held write lock at the end.
	staging := &VirtualFileSystem{
		Index:        make(map[string]*AssetSource),
		Directories:  make(map[string][]*AssetSource),
		Sources:      []string{},
		GamedataPath: vfs.GamedataPath,
		TextAssets:   vfs.TextAssets,
	}

	if staging.GamedataPath != "" {
		pk3s := staging.findPK3s(staging.GamedataPath)
		for _, pk3 := range pk3s {
			staging.indexPK3(pk3)
			staging.Sources = append(staging.Sources, pk3)
		}
	}
	if staging.TextAssets != "" {
		staging.indexDirectory(staging.TextAssets)
		staging.Sources = append(staging.Sources, staging.TextAssets)
	}

	// Rebuild directory structure from staging index.
	for path, source := range staging.Index {
		dir := filepath.Dir(path)
		if dir == "." {
			dir = ""
		}
		dir = strings.ReplaceAll(dir, "\\", "/")
		staging.Directories[dir] = append(staging.Directories[dir], source)
		staging.ensureParentDirs(dir)
	}

	// Atomic swap. Lock held for microseconds — pointer reassignment.
	vfs.mu.Lock()
	vfs.Index = staging.Index
	vfs.Directories = staging.Directories
	vfs.Sources = staging.Sources
	vfs.generation++
	vfs.gameIcons = nil
	vfs.mu.Unlock()
	return nil
}

func (vfs *VirtualFileSystem) findPK3s(root string) []string {
	var pk3s []string
	seen := make(map[string]bool)

	// In Quake 3 / OpenJK, load order determines asset precedence:
	// 1. base/ (JKA stock base assets)
	// 2. MBII/ (Public official release for standard players)
	// 3. MBIITest/ (Beta tester / QA builds)
	// 4. MBIIRelease/ (Dev release staging)
	// 5. Dev sandboxes & custom profile folders (e.g. ProfileFrenzy, Profile*)
	priorityDirs := []string{
		filepath.Join(root, "base"),
		filepath.Join(root, "MBII"),
		filepath.Join(root, "MBIITest"),
		filepath.Join(root, "MBIIRelease"),
		root,
	}

	searchPaths := make([]string, 0, len(priorityDirs)+10)
	for _, p := range priorityDirs {
		searchPaths = append(searchPaths, p)
	}

	// Discover any dev/profile subdirectories (e.g. ProfileFrenzy, custom mods)
	if entries, err := os.ReadDir(root); err == nil {
		for _, e := range entries {
			if e.IsDir() {
				name := e.Name()
				if name != "base" && name != "MBII" && name != "MBIITest" && name != "MBIIRelease" {
					searchPaths = append(searchPaths, filepath.Join(root, name))
				}
			}
		}
	}

	for _, dir := range searchPaths {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}

		var dirPK3s []string
		for _, e := range entries {
			if !e.IsDir() && strings.HasSuffix(strings.ToLower(e.Name()), ".pk3") {
				path := filepath.Join(dir, e.Name())
				if !seen[path] {
					dirPK3s = append(dirPK3s, path)
					seen[path] = true
				}
			}
		}
		// Sort alphabetically within each folder (engine load order)
		sort.Strings(dirPK3s)
		pk3s = append(pk3s, dirPK3s...)
	}

	return pk3s
}

func (vfs *VirtualFileSystem) indexPK3(path string) {
	r, err := zip.OpenReader(path)
	if err != nil {
		return
	}
	defer r.Close()

	for _, f := range r.File {
		if strings.HasSuffix(f.Name, "/") {
			continue
		} // Skip dir entries

		logicalPath := strings.ReplaceAll(f.Name, "\\", "/")
		normalizedKey := strings.ToLower(logicalPath)

		vfs.Index[normalizedKey] = &AssetSource{
			Path:        logicalPath,
			EntryName:   f.Name,
			PK3Path:     path,
			Size:        int64(f.UncompressedSize64),
			ModTime:     f.Modified,
			CRC32:       f.CRC32,
			IsDirectory: false,
		}
	}
}

func (vfs *VirtualFileSystem) indexDirectory(root string) {
	// Check if this looks like the TextAssets root (contains MBAssets3, etc.)
	// or just a standard base folder.
	subDirs, err := os.ReadDir(root)
	if err != nil {
		return
	}

	isTextAssetsRoot := false
	for _, d := range subDirs {
		if d.IsDir() && (d.Name() == "MBAssets3" || d.Name() == "MB_Effects") {
			isTextAssetsRoot = true
			break
		}
	}

	if isTextAssetsRoot {
		// Index each known subfolder as a root
		for _, d := range subDirs {
			if !d.IsDir() || strings.HasPrefix(d.Name(), ".") {
				continue
			}

			// These folders act like unzipped PK3s
			subRoot := filepath.Join(root, d.Name())
			// Inside MBAssets3, "ext_data" is at the top.
			// So logical path of "TextAssets/MBAssets3/ext_data/x" is "ext_data/x".
			vfs.walkAndIndex(subRoot)
		}
	} else {
		// Treat as a single base folder (legacy behavior)
		vfs.walkAndIndex(root)
	}
}

func (vfs *VirtualFileSystem) walkAndIndex(root string) {
	fs.WalkDir(os.DirFS(root), ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		if strings.HasPrefix(d.Name(), ".") {
			return nil
		}

		logicalPath := strings.ReplaceAll(path, "\\", "/")
		normalizedKey := strings.ToLower(logicalPath)
		info, _ := d.Info()

		// If duplicate, overwrite (TextAssets overrides PK3s)
		vfs.Index[normalizedKey] = &AssetSource{
			Path:        logicalPath,
			FullPath:    filepath.Join(root, path),
			Size:        info.Size(),
			ModTime:     info.ModTime(),
			IsDirectory: false,
		}
		return nil
	})
}

func (vfs *VirtualFileSystem) ensureParentDirs(dir string) {
	if dir == "" || dir == "." {
		return
	}

	parent := filepath.Dir(dir)
	if parent == "." {
		parent = ""
	}
	parent = strings.ReplaceAll(parent, "\\", "/")

	// Check if parent already knows about this dir
	exists := false
	for _, child := range vfs.Directories[parent] {
		if child.Path == dir {
			exists = true
			break
		}
	}

	if !exists {
		dirEntry := &AssetSource{
			Path:        dir,
			IsDirectory: true,
		}
		vfs.Directories[parent] = append(vfs.Directories[parent], dirEntry)
	}

	vfs.ensureParentDirs(parent)
}

// Lookup returns the current winning source without exposing the mutable index.
func (vfs *VirtualFileSystem) Lookup(path string) *AssetSource {
	norm := strings.ToLower(strings.ReplaceAll(path, "\\", "/"))
	vfs.mu.RLock()
	defer vfs.mu.RUnlock()
	return vfs.Index[norm]
}

// ReadFile opens a file from the VFS (PK3 or local)
func (vfs *VirtualFileSystem) ReadFile(path string) (io.ReadCloser, error) {
	norm := strings.ToLower(strings.ReplaceAll(path, "\\", "/"))
	vfs.mu.RLock()
	source, ok := vfs.Index[norm]
	vfs.mu.RUnlock()

	if !ok {
		return nil, os.ErrNotExist
	}

	if source.PK3Path != "" {
		// Extract from PK3
		r, err := zip.OpenReader(source.PK3Path)
		if err != nil {
			return nil, err
		}

		// Find file
		for _, f := range r.File {
			if f.Name == source.EntryName || strings.EqualFold(strings.ReplaceAll(f.Name, "\\", "/"), norm) {
				rc, err := f.Open()
				if err != nil {
					r.Close()
					return nil, err
				}
				// Wrapper to close zip reader when file closed
				return &pk3ReadCloser{rc: rc, zr: r}, nil
			}
		}
		r.Close()
		return nil, os.ErrNotExist
	}

	// Local file
	return os.Open(source.FullPath)
}

// Search finds files matching a pattern
func (vfs *VirtualFileSystem) Search(pattern string) []*AssetSource {
	vfs.mu.RLock()
	defer vfs.mu.RUnlock()

	var results []*AssetSource
	pattern = strings.ToLower(pattern)

	for path, source := range vfs.Index {
		if strings.Contains(strings.ToLower(path), pattern) {
			results = append(results, source)
		}
	}

	// Sort results
	sort.Slice(results, func(i, j int) bool {
		return results[i].Path < results[j].Path
	})

	return results
}

// Suggest returns up to `max` indexed paths whose lowercase form contains
// the query AND passes the optional accept() filter. Used by the inline
// path-autocomplete on model/skin/uishader/etc. entries — the result is
// PK3-aware because the index already merges loose files and PK3
// contents (see indexPK3 / indexDirectory).
//
// `accept` may be nil to disable type filtering. When non-nil, returning
// false on a path skips it (e.g. caller wants only `.skin` files, or
// only paths under `models/players/`).
//
// Results are deduplicated by path: if the same logical asset exists in
// both a loose file and a PK3, the loose version wins (alphabetic
// tiebreak via sort).
func (vfs *VirtualFileSystem) Suggest(query string, accept func(string) bool, max int) []string {
	vfs.mu.RLock()
	defer vfs.mu.RUnlock()

	if max <= 0 {
		max = 50
	}

	q := strings.ToLower(strings.TrimSpace(query))
	seen := make(map[string]bool)
	matches := make([]string, 0, max)

	for path := range vfs.Index {
		if seen[path] {
			continue
		}
		if q != "" && !strings.Contains(strings.ToLower(path), q) {
			continue
		}
		if accept != nil && !accept(path) {
			continue
		}
		seen[path] = true
		matches = append(matches, path)
	}

	sort.Strings(matches)
	if len(matches) > max {
		matches = matches[:max]
	}
	return matches
}

type pk3ReadCloser struct {
	rc io.ReadCloser
	zr *zip.ReadCloser
}

func (p *pk3ReadCloser) Read(b []byte) (int, error) { return p.rc.Read(b) }
func (p *pk3ReadCloser) Close() error {
	p.rc.Close()
	return p.zr.Close()
}
