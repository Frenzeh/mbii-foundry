package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestFetchLatestReleaseIsBoundedAndChecksResponseClose(t *testing.T) {
	valid := []byte(`[{"tag_name":"v1.2.3-alpha","prerelease":true,"assets":[],"future_github_field":true}]`)
	info, err := fetchLatestReleaseFrom(
		clientReturning(http.StatusOK, valid, nil),
		"https://api.invalid/releases",
		"1.2.2-alpha",
	)
	if err != nil {
		t.Fatalf("valid GitHub response rejected: %v", err)
	}
	if info.TagName != "v1.2.3-alpha" {
		t.Fatalf("tag = %q", info.TagName)
	}

	if _, err := fetchLatestReleaseFrom(
		clientReturning(http.StatusOK, valid, errors.New("close failed")),
		"https://api.invalid/releases",
		"1.2.2-alpha",
	); err == nil || !strings.Contains(err.Error(), "close GitHub response") {
		t.Fatalf("response close failure not propagated: %v", err)
	}

	const maxReleaseResponseBytes = 2 << 20
	oversized := bytes.Repeat([]byte(" "), maxReleaseResponseBytes+1)
	if _, err := fetchLatestReleaseFrom(
		clientReturning(http.StatusOK, oversized, nil),
		"https://api.invalid/releases",
		"1.2.2-alpha",
	); err == nil || !strings.Contains(err.Error(), "exceeds") {
		t.Fatalf("oversized response accepted: %v", err)
	}
}

func TestPrereleaseChannelSelectsHighestSemVer(t *testing.T) {
	releases := []UpdateInfo{
		{TagName: "v0.16.0-alpha.2", Prerelease: true},
		{TagName: "v0.15.0-alpha", Prerelease: true},
		{TagName: "v0.16.0-alpha.10", Prerelease: true},
	}

	selected, err := selectLatestEligibleRelease(releases, "0.15.0-alpha")
	if err != nil {
		t.Fatal(err)
	}
	if selected.TagName != "v0.16.0-alpha.10" || !selected.Prerelease || !selected.IsNewer {
		t.Fatalf("selected release = %+v", selected)
	}
}

func TestReleaseSelectionExcludesDraftsAndMislabeledTags(t *testing.T) {
	releases := []UpdateInfo{
		{TagName: "v9.0.0-alpha", Prerelease: true, Draft: true},
		{TagName: "v2.0.0-alpha", Prerelease: false},
		{TagName: "v1.9.0", Prerelease: true},
		{TagName: "v1.4.0", Prerelease: false},
	}

	selected, err := selectLatestEligibleRelease(releases, "1.3.0-alpha")
	if err != nil {
		t.Fatal(err)
	}
	if selected.TagName != "v1.4.0" || selected.Prerelease {
		t.Fatalf("selected release = %+v", selected)
	}
}

func TestStableChannelIgnoresPrereleases(t *testing.T) {
	releases := []UpdateInfo{
		{TagName: "v2.0.0-alpha", Prerelease: true},
		{TagName: "v1.4.0", Prerelease: false},
	}

	selected, err := selectLatestEligibleRelease(releases, "1.4.0")
	if err != nil {
		t.Fatal(err)
	}
	if selected.TagName != "v1.4.0" || selected.IsNewer {
		t.Fatalf("stable channel selected an ineligible update: %+v", selected)
	}
}

func TestReleaseSelectionNeverDowngradesOrLoopsOnEqualVersion(t *testing.T) {
	for _, releases := range [][]UpdateInfo{
		{{TagName: "v0.15.0-alpha", Prerelease: true}},
		{{TagName: "v0.16.0-alpha", Prerelease: true}},
	} {
		selected, err := selectLatestEligibleRelease(releases, "0.16.0-alpha")
		if err != nil {
			t.Fatal(err)
		}
		if selected.IsNewer {
			t.Fatalf("non-newer release offered as update: %+v", selected)
		}
	}
}

func TestUpdateCacheFromPriorVersionCannotTriggerPostInstallLoop(t *testing.T) {
	t.Setenv("FOUNDRY_DEV_FAKE_UPDATE", "")
	dir := t.TempDir()
	stale := UpdateInfo{
		TagName:    "v0.16.0-alpha",
		Prerelease: true,
		IsNewer:    true,
		CurrentVer: "0.15.0-alpha",
		CheckedAt:  time.Now(),
	}
	data, err := json.Marshal(stale)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "update_cache.json"), data, 0600); err != nil {
		t.Fatal(err)
	}

	if cached := NewUpdateChecker(dir).Latest(); cached != nil {
		t.Fatalf("prior-version cache survived restart: %+v", cached)
	}
}

func TestVersionComparisonUsesSemVerPrereleaseOrdering(t *testing.T) {
	for _, test := range []struct {
		latest  string
		current string
		newer   bool
	}{
		{latest: "v0.16.0-alpha.10", current: "0.16.0-alpha.2", newer: true},
		{latest: "v0.16.0-beta", current: "0.16.0-alpha.10", newer: true},
		{latest: "v0.16.0", current: "0.16.0-rc.1", newer: true},
		{latest: "v0.16.0-alpha", current: "0.16.0-alpha", newer: false},
		{latest: "v0.15.0", current: "0.16.0-alpha", newer: false},
	} {
		if got := versionNewer(test.latest, test.current); got != test.newer {
			t.Errorf("versionNewer(%q, %q) = %v, want %v", test.latest, test.current, got, test.newer)
		}
	}
}

func TestPlatformAssetSuffixBindsRuntimeArchitecture(t *testing.T) {
	got := platformAssetSuffix()
	switch runtime.GOOS {
	case "darwin":
		if got != "macos-universal" {
			t.Fatalf("suffix = %q, want macos-universal", got)
		}
	case "linux", "windows":
		want := runtime.GOOS + "-" + runtime.GOARCH
		if got != want {
			t.Fatalf("suffix = %q, want %q", got, want)
		}
	default:
		if got != "" {
			t.Fatalf("unsupported platform suffix = %q", got)
		}
	}
}
