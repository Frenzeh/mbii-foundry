package main

import (
	"bytes"
	"errors"
	"net/http"
	"runtime"
	"strings"
	"testing"
)

func TestFetchLatestReleaseIsBoundedAndChecksResponseClose(t *testing.T) {
	valid := []byte(`{"tag_name":"v1.2.3","assets":[],"future_github_field":true}`)
	info, err := fetchLatestReleaseFrom(
		clientReturning(http.StatusOK, valid, nil),
		"https://api.invalid/releases/latest",
	)
	if err != nil {
		t.Fatalf("valid GitHub response rejected: %v", err)
	}
	if info.TagName != "v1.2.3" {
		t.Fatalf("tag = %q", info.TagName)
	}

	if _, err := fetchLatestReleaseFrom(
		clientReturning(http.StatusOK, valid, errors.New("close failed")),
		"https://api.invalid/releases/latest",
	); err == nil || !strings.Contains(err.Error(), "close GitHub response") {
		t.Fatalf("response close failure not propagated: %v", err)
	}

	const maxReleaseResponseBytes = 2 << 20
	oversized := bytes.Repeat([]byte(" "), maxReleaseResponseBytes+1)
	if _, err := fetchLatestReleaseFrom(
		clientReturning(http.StatusOK, oversized, nil),
		"https://api.invalid/releases/latest",
	); err == nil || !strings.Contains(err.Error(), "exceeds") {
		t.Fatalf("oversized response accepted: %v", err)
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
