package main

// ShaderResolver supplies still-image previews for declared Quake-style shaders.
// It chooses the first supported non-built-in map/clampmap/animmap texture in
// each definition; it does not reproduce stage blending or engine rendering.
//
// Only the winning VFS copy of each .shader file is read. Duplicate names use
// Foundry's deterministic preview policy: later PK3 source wins, loose files
// win over PK3s, then the lexically later lowercase logical .shader path wins.
// Within a file, the last supported declaration wins. This is a preview policy,
// not verified equivalence with an engine's active profile or shader registration.

import (
	"bufio"
	"io"
	"sort"
	"strings"
	"sync"
)

// ShaderResolver caches the parsed shader-name → texture-path map.
// One instance per VFS, built by Prebuild after indexing and invalidated
// with Reset before a later background rebuild.
type ShaderResolver struct {
	vfs     *VirtualFileSystem
	mu      sync.RWMutex
	buildMu sync.Mutex
	built   bool
	shaders map[string]string // shader name (lowercased) → texture path
}

// NewShaderResolver constructs a resolver bound to a VFS. Prebuild parses
// the table on a background worker before the UI readiness notification.
func NewShaderResolver(vfs *VirtualFileSystem) *ShaderResolver {
	return &ShaderResolver{vfs: vfs}
}

// Reset invalidates the cached shader map. Call after a VFS refresh
// before Prebuild scans the new asset set.
func (sr *ShaderResolver) Reset() {
	sr.buildMu.Lock()
	defer sr.buildMu.Unlock()
	sr.mu.Lock()
	defer sr.mu.Unlock()
	sr.built = false
	sr.shaders = nil
}

// Resolve looks up a shader name and returns the primary texture path
// it points at, or "" if no .shader file declares this name (or the
// resolver isn't built yet). Lookup is case-insensitive.
//
// IMPORTANT: this method is *non-blocking*. If the resolver hasn't
// finished its initial scan, Resolve returns "" — callers degrade to
// "no shader info available, fall back to direct texture probe."
// Building the resolver synchronously on first lookup used to block
// the UI thread for several seconds on a full MBII install (hundreds
// of `.shader` files), which looked like a freeze when opening the
// first MBCH. Build is now triggered eagerly via Prebuild() from
// AssetBrowser's background indexing goroutine.
func (sr *ShaderResolver) Resolve(name string) string {
	if sr == nil || sr.vfs == nil || name == "" {
		return ""
	}
	sr.mu.RLock()
	defer sr.mu.RUnlock()
	if !sr.built {
		return ""
	}
	return sr.shaders[strings.ToLower(name)]
}

// Prebuild serializes scans and skips a completed build. Call from a
// background worker: Resolve remains non-blocking while a scan runs.
func (sr *ShaderResolver) Prebuild() {
	if sr == nil {
		return
	}
	sr.buildMu.Lock()
	defer sr.buildMu.Unlock()
	sr.build()
}

// build scans the VFS for .shader files and parses them into the
// shaders map. Invariant: every key is lowercased.
//
// Lock discipline:
//  1. Snapshot the .shader path list under vfs.mu.RLock — short
//     critical section, no I/O held.
//  2. Release vfs lock, then call vfs.ReadFile() per file. ReadFile
//     itself acquires vfs.mu.RLock — we'd deadlock if we held it
//     across the call. Recovered output is appended into a local
//     map, then assigned under sr.mu at the end.
//
// buildMu serializes builds/resets without holding sr.mu across file I/O.
func (sr *ShaderResolver) build() {
	if sr.vfs == nil {
		sr.mu.Lock()
		sr.shaders = map[string]string{}
		sr.built = true
		sr.mu.Unlock()
		return
	}

	// Avoid duplicate work — re-check under lock before scanning.
	sr.mu.Lock()
	if sr.built {
		sr.mu.Unlock()
		return
	}
	sr.mu.Unlock()

	// Snapshot logical paths and their VFS source priority. All loose files
	// form the final tier; VFS indexing has already selected their overlay.
	type shaderFile struct {
		path     string
		priority int
	}
	var shaderFiles []shaderFile
	sr.vfs.mu.RLock()
	sourceOrder := make(map[string]int, len(sr.vfs.Sources))
	for i, source := range sr.vfs.Sources {
		sourceOrder[source] = i
	}
	for path, source := range sr.vfs.Index {
		if strings.HasSuffix(path, ".shader") {
			priority := len(sr.vfs.Sources)
			if source.PK3Path != "" {
				priority = sourceOrder[source.PK3Path]
			}
			shaderFiles = append(shaderFiles, shaderFile{path: path, priority: priority})
		}
	}
	sr.vfs.mu.RUnlock()
	sort.Slice(shaderFiles, func(i, j int) bool {
		if shaderFiles[i].priority != shaderFiles[j].priority {
			return shaderFiles[i].priority < shaderFiles[j].priority
		}
		return shaderFiles[i].path < shaderFiles[j].path
	})

	// Parse outside any lock. Defensive: each ReadFile / parse is
	// wrapped in a recover() so a malformed shader file can't crash
	// the editor — worst case it skips that file.
	out := map[string]string{}
	for _, file := range shaderFiles {
		func() {
			defer func() { _ = recover() }()
			rc, err := sr.vfs.ReadFile(file.path)
			if err != nil {
				return
			}
			defer rc.Close()
			data, err := io.ReadAll(rc)
			if err != nil || len(data) == 0 {
				return
			}
			parseShaderFile(string(data), out)
		}()
	}

	sr.mu.Lock()
	sr.shaders = out
	sr.built = true
	sr.mu.Unlock()
}

// parseShaderFile mutates `out` with shadername → primary-map entries
// found in the file body. Minimal, robust against:
//   - // line comments and /* */ block comments
//   - extra whitespace / tabs between tokens
//   - nested stages ({ ... } inside { ... })
//   - missing map directive (skipped silently)
//   - case differences (every key/value lowercased)
func parseShaderFile(body string, out map[string]string) {
	// Strip block comments first — line comments handled by the
	// scanner. Block comments inside strings shouldn't exist in Q3
	// shaders so a naive replace is safe.
	body = stripShaderBlockComments(body)

	scanner := bufio.NewScanner(strings.NewReader(body))
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	state := 0 // 0 = expecting shader name, 1 = inside top-level block, 2 = inside stage
	depth := 0
	var currentName string
	var firstMap string

	for scanner.Scan() {
		line := scanner.Text()
		if i := strings.Index(line, "//"); i >= 0 {
			line = line[:i]
		}
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		switch state {
		case 0:
			// Shader name line. Could be just the name, or name + {
			// on the same line. Strip trailing { if present.
			name := strings.TrimSuffix(strings.TrimSpace(line), "{")
			name = strings.TrimSpace(name)
			if name == "" {
				continue
			}
			currentName = name
			firstMap = ""
			if strings.HasSuffix(line, "{") {
				state = 1
				depth = 1
			} else {
				state = 1
				depth = 0
			}

		case 1:
			// Top-level block body. Look for { (stage open), } (block
			// close), or top-level directives we ignore.
			if line == "{" {
				depth++
				if depth >= 2 {
					state = 2
				}
				continue
			}
			if line == "}" {
				depth--
				if depth < 0 {
					depth = 0 // clamp — malformed shader, don't corrupt next entry
				}
				if depth == 0 {
					if currentName != "" && firstMap != "" {
						out[strings.ToLower(currentName)] = strings.ToLower(firstMap)
					}
					currentName = ""
					firstMap = ""
					state = 0
				}
				continue
			}
			// Some shaders inline `q3map_*` / `surfaceparm` / `cull`
			// at the top level. Ignored.

		case 2:
			// Inside a stage. Look for `map <path>` and stage close.
			if line == "{" {
				depth++
				continue
			}
			if line == "}" {
				depth--
				if depth < 0 {
					depth = 0
				}
				if depth <= 1 {
					state = 1
				}
				continue
			}
			fields := strings.Fields(line)
			if len(fields) >= 2 && firstMap == "" {
				directive := strings.ToLower(fields[0])
				if directive == "map" || directive == "clampmap" || directive == "animmap" {
					// `map <path>`, `clampmap <path>`, or
					// `animmap <freq> <path1> <path2>...` — pick path1.
					// Skip animmap when it has fewer than 3 fields —
					// fields[1] is the FREQUENCY, not a path; storing
					// it would pollute the resolver with a numeric
					// "texture name."
					var p string
					switch {
					case directive == "animmap":
						if len(fields) < 3 {
							continue
						}
						p = fields[2]
					default:
						p = fields[1]
					}
					p = strings.TrimSpace(p)
					// Skip "$lightmap", "$whiteimage", etc.
					if p != "" && !strings.HasPrefix(p, "$") {
						firstMap = p
					}
				}
			}
		}
	}
}

// stripShaderBlockComments removes /* ... */ block comments from a
// shader file body. Minimal; handles nesting by treating only the
// first matching pair on each scan.
func stripShaderBlockComments(s string) string {
	var b strings.Builder
	for {
		i := strings.Index(s, "/*")
		if i < 0 {
			b.WriteString(s)
			break
		}
		b.WriteString(s[:i])
		j := strings.Index(s[i+2:], "*/")
		if j < 0 {
			break
		}
		s = s[i+2+j+2:]
	}
	return b.String()
}
