package main

import (
	"archive/zip"
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
	FullPath    string // Absolute path on disk (for loose files)
	PK3Path     string // Path to PK3 file (empty if loose file)
	Size        int64
	ModTime     time.Time
	IsDirectory bool
}

// VirtualFileSystem manages the merged view of assets
type VirtualFileSystem struct {
	Index       map[string]*AssetSource
	Directories map[string][]*AssetSource // Map dir path to contents
	Sources     []string                  // List of loaded PK3s/folders

	GamedataPath string
	TextAssets   string

	mu sync.RWMutex
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
	vfs.mu.Unlock()
	return nil
}

func (vfs *VirtualFileSystem) findPK3s(root string) []string {
	var pk3s []string

	// Check subfolders that game typically loads
	searchPaths := []string{
		root,
		filepath.Join(root, "MBII"),
		filepath.Join(root, "MBIITest"),
		filepath.Join(root, "base"),
	}

	seen := make(map[string]bool)

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
		// Sort alphabetically (engine load order)
		sort.Strings(dirPK3s)
		pk3s = append(pk3s, dirPK3s...)
	}

	return pk3s
}

func (vfs *VirtualFileSystem) indexPK3(path string) {
	r, err := zip.OpenReader(path)
	if err != nil {
		LogError("Failed to open PK3 %s: %v", path, err)
		return
	}
	defer r.Close()

	for _, f := range r.File {
		if strings.HasSuffix(f.Name, "/") {
			continue
		} // Skip dir entries

		logicalPath := strings.ReplaceAll(f.Name, "\\", "/")

		vfs.Index[logicalPath] = &AssetSource{
			Path:        logicalPath,
			PK3Path:     path,
			Size:        int64(f.UncompressedSize64),
			ModTime:     f.Modified,
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
		info, _ := d.Info()

		// If duplicate, overwrite (TextAssets overrides PK3s)
		vfs.Index[logicalPath] = &AssetSource{
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

	if _, exists := vfs.Directories[dir]; !exists {
		vfs.Directories[dir] = []*AssetSource{}
	}

	parent := filepath.Dir(dir)
	if parent == "." {
		parent = ""
	}
	parent = strings.ReplaceAll(parent, "\\", "/")

	if parent != dir {
		// Add this dir as a child of parent
		// Check if already exists
		exists := false
		if children, ok := vfs.Directories[parent]; ok {
			for _, child := range children {
				if child.Path == dir {
					exists = true
					break
				}
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
}

// ReadFile opens a file from the VFS (PK3 or local)
func (vfs *VirtualFileSystem) ReadFile(path string) (io.ReadCloser, error) {
	vfs.mu.RLock()
	source, ok := vfs.Index[path]
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
			if strings.ReplaceAll(f.Name, "\\", "/") == path {
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

type pk3ReadCloser struct {
	rc io.ReadCloser
	zr *zip.ReadCloser
}

func (p *pk3ReadCloser) Read(b []byte) (int, error) { return p.rc.Read(b) }
func (p *pk3ReadCloser) Close() error {
	p.rc.Close()
	return p.zr.Close()
}
