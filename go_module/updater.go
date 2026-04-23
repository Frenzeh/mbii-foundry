package main

// Update-checker + installer dispatcher.
//
// Flow:
//   1. Startup kicks off UpdateChecker.CheckAsync() in a goroutine so
//      the window opens instantly and the network call never blocks.
//   2. Result is cached in $CONFIG/update_cache.json for 6h so we
//      don't spam the GitHub API every relaunch.
//   3. If a newer version is available, the Home screen's banner
//      reads the cached result and offers to install (Mac/Linux) or
//      open the release page (Windows).
//
// Version comparison is intentionally lenient — GitHub release tags
// have looked like "v0.2.0-alpha", "v0.3.0-alpha" etc., and semver
// parsers from the stdlib choke on the -alpha suffix. We parse
// "major.minor.patch" out of whatever prefix/suffix is present and
// compare numerically. Pre-release suffix on current-but-not-latest
// still counts as "update available" so testers get the stable bump.

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	updateOwner    = "Frenzeh"
	updateRepo     = "mbii-foundry"
	updateCacheTTL = 6 * time.Hour
	updateAPIURL   = "https://api.github.com/repos/" + updateOwner + "/" + updateRepo + "/releases/latest"
)

// UpdateInfo is the subset of the GitHub release response we care
// about, plus a couple of derived fields.
type UpdateInfo struct {
	TagName     string         `json:"tag_name"`
	Name        string         `json:"name"`
	HTMLURL     string         `json:"html_url"`
	PublishedAt time.Time      `json:"published_at"`
	Assets      []ReleaseAsset `json:"assets"`
	Prerelease  bool           `json:"prerelease"`

	// Derived at check-time, persisted in cache.
	IsNewer    bool      `json:"is_newer"`
	CheckedAt  time.Time `json:"checked_at"`
	CurrentVer string    `json:"current_ver"`
}

type ReleaseAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
	Size               int64  `json:"size"`
}

// UpdateChecker runs in the background, persists the latest result,
// and exposes Latest() to UI code. Thread-safe.
type UpdateChecker struct {
	configDir string

	mu      sync.RWMutex
	info    *UpdateInfo
	running bool
}

func NewUpdateChecker(configDir string) *UpdateChecker {
	uc := &UpdateChecker{configDir: configDir}
	uc.loadCache() // best-effort; stale cache is fine, CheckAsync refreshes
	// Dev-only preview: set FOUNDRY_DEV_FAKE_UPDATE=1 to populate the
	// checker with a synthetic "new release available" so the Home
	// footer callout renders without needing to ship a real release.
	// Useful when iterating on the UI; disabled in regular runs.
	if os.Getenv("FOUNDRY_DEV_FAKE_UPDATE") == "1" {
		uc.info = &UpdateInfo{
			TagName:     "v99.0.0-dev",
			Name:        "Fake Release for UI Preview",
			HTMLURL:     "https://github.com/" + updateOwner + "/" + updateRepo + "/releases",
			PublishedAt: time.Now().Add(-2 * time.Hour),
			IsNewer:     true,
			CurrentVer:  AppVersion,
			CheckedAt:   time.Now(),
		}
	}
	return uc
}

// CheckAsync refreshes the cached update info in the background.
// Callback fires on the calling goroutine — wrap it in fyne.Do if
// it touches UI.
func (uc *UpdateChecker) CheckAsync(onDone func(*UpdateInfo)) {
	uc.mu.Lock()
	if uc.running {
		uc.mu.Unlock()
		return
	}
	uc.running = true
	uc.mu.Unlock()

	go func() {
		defer func() {
			uc.mu.Lock()
			uc.running = false
			uc.mu.Unlock()
		}()

		// Skip the fetch if the cache is fresh. The UI still gets the
		// callback so its banner can render from the cached info.
		if cached := uc.Latest(); cached != nil &&
			time.Since(cached.CheckedAt) < updateCacheTTL {
			if onDone != nil {
				onDone(cached)
			}
			return
		}

		info, err := fetchLatestRelease()
		if err != nil {
			LogInfo("Update check failed: %v", err)
			// Keep whatever cache we had — no-network is a non-event
			// for the banner (it just stays hidden).
			if onDone != nil {
				onDone(uc.Latest())
			}
			return
		}
		info.CurrentVer = AppVersion
		info.CheckedAt = time.Now()
		info.IsNewer = versionNewer(info.TagName, AppVersion)

		uc.mu.Lock()
		uc.info = info
		uc.mu.Unlock()
		uc.saveCache()

		if onDone != nil {
			onDone(info)
		}
	}()
}

// Latest returns the most recent cached UpdateInfo, or nil if nothing's
// been fetched yet.
func (uc *UpdateChecker) Latest() *UpdateInfo {
	uc.mu.RLock()
	defer uc.mu.RUnlock()
	return uc.info
}

// AssetForThisPlatform picks the release asset matching the current
// GOOS/GOARCH from the release's asset list. Returns nil if no match.
func (uc *UpdateChecker) AssetForThisPlatform() *ReleaseAsset {
	info := uc.Latest()
	if info == nil {
		return nil
	}
	match := platformAssetSuffix()
	if match == "" {
		return nil
	}
	for i := range info.Assets {
		if strings.Contains(strings.ToLower(info.Assets[i].Name), match) {
			return &info.Assets[i]
		}
	}
	return nil
}

// platformAssetSuffix returns the substring the release-workflow puts
// in asset names for the current runtime (e.g. "linux-amd64",
// "macos-universal"). Kept in sync with .github/workflows/release.yml.
func platformAssetSuffix() string {
	switch runtime.GOOS {
	case "darwin":
		return "macos-universal"
	case "linux":
		return "linux-amd64"
	case "windows":
		return "windows-amd64"
	}
	return ""
}

// cachePath is the JSON file we persist the last check into.
func (uc *UpdateChecker) cachePath() string {
	return filepath.Join(uc.configDir, "update_cache.json")
}

func (uc *UpdateChecker) loadCache() {
	data, err := os.ReadFile(uc.cachePath())
	if err != nil {
		return
	}
	var info UpdateInfo
	if err := json.Unmarshal(data, &info); err != nil {
		return
	}
	uc.mu.Lock()
	uc.info = &info
	uc.mu.Unlock()
}

func (uc *UpdateChecker) saveCache() {
	info := uc.Latest()
	if info == nil {
		return
	}
	data, err := json.MarshalIndent(info, "", "  ")
	if err != nil {
		return
	}
	_ = os.MkdirAll(uc.configDir, 0755)
	_ = os.WriteFile(uc.cachePath(), data, 0644)
}

// fetchLatestRelease hits the GitHub API. The token-less rate limit
// (60 req/hr/IP) is fine for end-user update checks — cached 6h means
// about 4 hits per day per machine.
func fetchLatestRelease() (*UpdateInfo, error) {
	req, err := http.NewRequest(http.MethodGet, updateAPIURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "mbii-foundry-updater/1.0")

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return nil, fmt.Errorf("github returned %s: %s", resp.Status, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	var info UpdateInfo
	if err := json.Unmarshal(body, &info); err != nil {
		return nil, err
	}
	if info.TagName == "" {
		return nil, errors.New("empty tag in release response")
	}
	return &info, nil
}

// versionNewer reports whether latestTag is newer than currentVer.
// Both strings are parsed leniently — "v0.3.0-alpha" and "0.3.0"
// both yield (0, 3, 0) for comparison. Pre-release suffixes break
// ties so that "0.3.0" beats "0.3.0-alpha" (stable > prerelease of
// same triplet), but a higher triplet always wins outright.
func versionNewer(latestTag, currentVer string) bool {
	la, lb, lc, lPre := parseVersion(latestTag)
	ca, cb, cc, cPre := parseVersion(currentVer)

	if la != ca {
		return la > ca
	}
	if lb != cb {
		return lb > cb
	}
	if lc != cc {
		return lc > cc
	}
	// Same triplet: stable (no prerelease) beats prerelease.
	if lPre == "" && cPre != "" {
		return true
	}
	if lPre != "" && cPre == "" {
		return false
	}
	// Both prerelease or both stable: alphabetical on the prerelease
	// suffix is a reasonable-enough ordering for our use (alpha < beta
	// < rc alphabetically).
	return lPre > cPre
}

var versionRe = regexp.MustCompile(`(\d+)\.(\d+)\.(\d+)(?:[.-]?(.*))?`)

func parseVersion(v string) (int, int, int, string) {
	v = strings.TrimSpace(v)
	v = strings.TrimPrefix(v, "v")
	m := versionRe.FindStringSubmatch(v)
	if m == nil {
		// Give up — treat unparseable versions as 0.0.0 so the remote
		// wins iff it has any recognizable triplet.
		return 0, 0, 0, v
	}
	major, _ := strconv.Atoi(m[1])
	minor, _ := strconv.Atoi(m[2])
	patch, _ := strconv.Atoi(m[3])
	pre := ""
	if len(m) > 4 {
		pre = m[4]
	}
	return major, minor, patch, pre
}
