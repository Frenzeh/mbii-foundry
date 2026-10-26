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
// Release selection follows the running build's channel. Prerelease builds may
// advance through prereleases or to stable; stable builds ignore prereleases.
// Drafts, malformed versions, and releases whose GitHub prerelease flag
// disagrees with their SemVer tag are never eligible.

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
	updateAPIURL   = "https://api.github.com/repos/" + updateOwner + "/" + updateRepo + "/releases?per_page=100"
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
	Draft       bool           `json:"draft"`

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
	uc.checkAsync(false, onDone)
}

// ForceCheckAsync refreshes ignoring the 6h cache. Used by the
// toolbar's "Check for updates now" action so users who don't want
// to wait on the cache can poke the server themselves.
func (uc *UpdateChecker) ForceCheckAsync(onDone func(*UpdateInfo)) {
	uc.checkAsync(true, onDone)
}

func (uc *UpdateChecker) checkAsync(force bool, onDone func(*UpdateInfo)) {
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

		// Skip the fetch if the cache is fresh, unless the caller
		// explicitly asked to bypass the cache. The UI still gets the
		// callback so its banner can render from the cached info.
		if !force {
			if cached := uc.Latest(); cached != nil &&
				time.Since(cached.CheckedAt) < updateCacheTTL {
				if onDone != nil {
					onDone(cached)
				}
				return
			}
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
		info.CheckedAt = time.Now()

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
func AssetForPlatform(info *UpdateInfo) *ReleaseAsset {
	if info == nil {
		return nil
	}
	match := platformAssetSuffix()
	if match == "" {
		return nil
	}
	for i := range info.Assets {
		name := strings.ToLower(info.Assets[i].Name)
		if strings.Contains(name, match) && !strings.HasSuffix(name, ".manifest.json") {
			return &info.Assets[i]
		}
	}
	return nil
}

func (uc *UpdateChecker) AssetForThisPlatform() *ReleaseAsset {
	return AssetForPlatform(uc.Latest())
}

// platformAssetSuffix returns the substring the release-workflow puts
// in asset names for the current runtime (e.g. "linux-amd64",
// "macos-universal"). Kept in sync with .github/workflows/release.yml.
func platformAssetSuffix() string {
	switch runtime.GOOS {
	case "darwin":
		return "macos-universal"
	case "linux":
		return "linux-" + runtime.GOARCH
	case "windows":
		return "windows-" + runtime.GOARCH
	default:
		return ""
	}
}

// cachePath is the JSON file we persist the last check into.
func (uc *UpdateChecker) cachePath() string {
	if uc.configDir == "" {
		return ""
	}
	return filepath.Join(uc.configDir, "update_cache.json")
}

func (uc *UpdateChecker) loadCache() {
	p := uc.cachePath()
	if p == "" {
		return
	}
	data, err := os.ReadFile(p)
	if err != nil {
		return
	}
	var info UpdateInfo
	if err := json.Unmarshal(data, &info); err != nil {
		return
	}
	// A cache created by an older binary must not keep advertising that
	// binary's update after the newly installed version restarts.
	if info.CurrentVer != AppVersion {
		return
	}
	candidate, ok := eligibleReleaseVersion(&info)
	current, currentOK := parseVersion(AppVersion)
	if !ok || !currentOK || (!current.isPrerelease() && candidate.isPrerelease()) {
		return
	}
	info.IsNewer = compareVersions(candidate, current) > 0
	uc.mu.Lock()
	uc.info = &info
	uc.mu.Unlock()
}

func (uc *UpdateChecker) saveCache() {
	p := uc.cachePath()
	if p == "" {
		return
	}
	info := uc.Latest()
	if info == nil {
		return
	}
	data, err := json.MarshalIndent(info, "", "  ")
	if err != nil {
		return
	}
	_ = os.MkdirAll(uc.configDir, 0755)
	_ = os.WriteFile(p, data, 0644)
}

// fetchLatestRelease lists GitHub releases rather than using /releases/latest,
// because GitHub excludes prereleases from that endpoint. The token-less rate
// limit (60 req/hr/IP) is fine for end-user checks; the six-hour cache limits
// each installation to about four requests per day.
func fetchLatestRelease() (*UpdateInfo, error) {
	return fetchLatestReleaseFrom(
		&http.Client{Timeout: 15 * time.Second},
		updateAPIURL,
		AppVersion,
	)
}

func fetchLatestReleaseFrom(client *http.Client, url, currentVersion string) (*UpdateInfo, error) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "mbii-foundry-updater/1.0")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	const maxReleaseResponseBytes = 2 << 20
	body, readErr := io.ReadAll(io.LimitReader(resp.Body, maxReleaseResponseBytes+1))
	closeErr := resp.Body.Close()
	if readErr != nil {
		return nil, fmt.Errorf("read GitHub response: %w", readErr)
	}
	if closeErr != nil {
		return nil, fmt.Errorf("close GitHub response: %w", closeErr)
	}
	if resp.StatusCode != http.StatusOK {
		if len(body) > 512 {
			body = body[:512]
		}
		return nil, fmt.Errorf("github returned %s: %s", resp.Status, string(body))
	}
	if len(body) > maxReleaseResponseBytes {
		return nil, fmt.Errorf("GitHub release response exceeds %d-byte limit", maxReleaseResponseBytes)
	}

	var releases []UpdateInfo
	if err := json.Unmarshal(body, &releases); err != nil {
		return nil, err
	}
	return selectLatestEligibleRelease(releases, currentVersion)
}

// versionNewer reports whether latestTag is a strictly newer SemVer than
// currentVer. Invalid versions never win.
func versionNewer(latestTag, currentVer string) bool {
	latest, latestOK := parseVersion(latestTag)
	current, currentOK := parseVersion(currentVer)
	return latestOK && currentOK && compareVersions(latest, current) > 0
}

type semanticVersion struct {
	major      uint64
	minor      uint64
	patch      uint64
	prerelease []string
}

func (v semanticVersion) isPrerelease() bool {
	return len(v.prerelease) != 0
}

var versionRe = regexp.MustCompile(`^v?(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(?:-([0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*))?(?:\+[0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*)?$`)

func parseVersion(raw string) (semanticVersion, bool) {
	match := versionRe.FindStringSubmatch(strings.TrimSpace(raw))
	if match == nil {
		return semanticVersion{}, false
	}
	major, err := strconv.ParseUint(match[1], 10, 64)
	if err != nil {
		return semanticVersion{}, false
	}
	minor, err := strconv.ParseUint(match[2], 10, 64)
	if err != nil {
		return semanticVersion{}, false
	}
	patch, err := strconv.ParseUint(match[3], 10, 64)
	if err != nil {
		return semanticVersion{}, false
	}
	var prerelease []string
	if match[4] != "" {
		prerelease = strings.Split(match[4], ".")
		for _, identifier := range prerelease {
			if isNumericIdentifier(identifier) && len(identifier) > 1 && identifier[0] == '0' {
				return semanticVersion{}, false
			}
		}
	}
	return semanticVersion{
		major:      major,
		minor:      minor,
		patch:      patch,
		prerelease: prerelease,
	}, true
}

func selectLatestEligibleRelease(releases []UpdateInfo, currentVersion string) (*UpdateInfo, error) {
	current, ok := parseVersion(currentVersion)
	if !ok {
		return nil, fmt.Errorf("invalid current application version %q", currentVersion)
	}

	bestIndex := -1
	var best semanticVersion
	for i := range releases {
		if releases[i].Draft {
			continue
		}
		candidate, valid := eligibleReleaseVersion(&releases[i])
		if !valid {
			continue
		}
		if !current.isPrerelease() && candidate.isPrerelease() {
			continue
		}
		if bestIndex == -1 || compareVersions(candidate, best) > 0 {
			bestIndex = i
			best = candidate
		}
	}
	if bestIndex == -1 {
		return nil, errors.New("no eligible release found for the current update channel")
	}

	selected := releases[bestIndex]
	selected.CurrentVer = currentVersion
	selected.IsNewer = compareVersions(best, current) > 0
	return &selected, nil
}

func eligibleReleaseVersion(release *UpdateInfo) (semanticVersion, bool) {
	if release == nil || release.Draft {
		return semanticVersion{}, false
	}
	version, ok := parseVersion(release.TagName)
	if !ok || release.Prerelease != version.isPrerelease() {
		return semanticVersion{}, false
	}
	return version, true
}

func compareVersions(left, right semanticVersion) int {
	if result := compareUint64(left.major, right.major); result != 0 {
		return result
	}
	if result := compareUint64(left.minor, right.minor); result != 0 {
		return result
	}
	if result := compareUint64(left.patch, right.patch); result != 0 {
		return result
	}
	if !left.isPrerelease() && !right.isPrerelease() {
		return 0
	}
	if !left.isPrerelease() {
		return 1
	}
	if !right.isPrerelease() {
		return -1
	}
	for i := 0; i < len(left.prerelease) && i < len(right.prerelease); i++ {
		if result := comparePrereleaseIdentifier(left.prerelease[i], right.prerelease[i]); result != 0 {
			return result
		}
	}
	switch {
	case len(left.prerelease) < len(right.prerelease):
		return -1
	case len(left.prerelease) > len(right.prerelease):
		return 1
	default:
		return 0
	}
}

func compareUint64(left, right uint64) int {
	switch {
	case left < right:
		return -1
	case left > right:
		return 1
	default:
		return 0
	}
}

func comparePrereleaseIdentifier(left, right string) int {
	leftNumeric := isNumericIdentifier(left)
	rightNumeric := isNumericIdentifier(right)
	if leftNumeric && rightNumeric {
		if len(left) < len(right) {
			return -1
		}
		if len(left) > len(right) {
			return 1
		}
	}
	if leftNumeric != rightNumeric {
		if leftNumeric {
			return -1
		}
		return 1
	}
	return strings.Compare(left, right)
}

func isNumericIdentifier(value string) bool {
	if value == "" {
		return false
	}
	for _, r := range value {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}
