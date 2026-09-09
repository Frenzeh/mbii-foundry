package main

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/Frenzeh/mbii-foundry/updatemanifest"
)

// mustSignManifest signs the manifest over the canonical, length-prefixed
// payload (updatemanifest.Payload) — the exact framing the release signer
// and the updater share.
func mustSignManifest(t *testing.T, priv ed25519.PrivateKey, m UpdateManifest) UpdateManifest {
	t.Helper()
	payload, err := updatemanifest.Payload(m.Version, m.Platform, m.Architecture, m.Length, m.Digest)
	if err != nil {
		t.Fatalf("build manifest payload: %v", err)
	}
	m.Signature = hex.EncodeToString(ed25519.Sign(priv, payload))
	return m
}

func TestInstallUpdateSecurity(t *testing.T) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}

	origPublisherKey := PublisherTrustedKey
	defer func() {
		PublisherTrustedKey = origPublisherKey
	}()
	PublisherTrustedKey = hex.EncodeToString(pub)

	assetData := []byte("hello world update payload")
	assetHash := sha256.Sum256(assetData)
	assetDigest := hex.EncodeToString(assetHash[:])

	expectedArch := runtime.GOARCH
	if runtime.GOOS == "darwin" {
		expectedArch = "universal"
	}

	manifest := mustSignManifest(t, priv, UpdateManifest{
		Version:      "v1.0.0",
		Platform:     runtime.GOOS,
		Architecture: expectedArch,
		Length:       int64(len(assetData)),
		Digest:       assetDigest,
	})

	var overrideManifestData []byte
	var overrideAssetData []byte

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, ".manifest.json") {
			if overrideManifestData != nil {
				w.Write(overrideManifestData)
			} else {
				json.NewEncoder(w).Encode(manifest)
			}
		} else {
			if overrideAssetData != nil {
				w.Write(overrideAssetData)
			} else {
				w.Write(assetData)
			}
		}
	}))
	defer ts.Close()

	ext := ".tar.gz"
	if runtime.GOOS == "darwin" || runtime.GOOS == "windows" {
		ext = ".zip"
	}
	assetName := fmt.Sprintf("release-%s%s", platformAssetSuffix(), ext)
	asset := ReleaseAsset{
		Name:               assetName,
		BrowserDownloadURL: ts.URL + "/" + assetName,
		Size:               int64(len(assetData)),
	}
	info := &UpdateInfo{
		TagName: "v1.0.0",
		Assets:  []ReleaseAsset{asset},
	}

	t.Run("NoPublisherKey", func(t *testing.T) {
		saved := PublisherTrustedKey
		PublisherTrustedKey = ""
		err := InstallUpdate(info, nil)
		PublisherTrustedKey = saved
		if err == nil || !strings.Contains(err.Error(), "no trusted publisher key") {
			t.Errorf("expected 'no trusted publisher key' error, got %v", err)
		}
	})

	t.Run("WrongDigestSameLength", func(t *testing.T) {
		// Same length, different content — exercises digest check independently of length check.
		overrideAssetData = []byte("wrong world update payload")
		_, cleanup, err := downloadToTemp(&asset, manifest, nil)
		if err == nil {
			t.Errorf("expected error for wrong digest same length")
		} else if !strings.Contains(err.Error(), "digest") {
			t.Errorf("expected digest error, got: %v", err)
		}
		if cleanup != nil {
			cleanup()
		}
		overrideAssetData = nil
	})

	t.Run("WrongKey", func(t *testing.T) {
		wrongPub, _, _ := ed25519.GenerateKey(rand.Reader)
		PublisherTrustedKey = hex.EncodeToString(wrongPub)
		err := InstallUpdate(info, nil)
		if err == nil || !strings.Contains(err.Error(), "signature verification failed") {
			t.Errorf("expected signature verification error, got %v", err)
		}
		PublisherTrustedKey = hex.EncodeToString(pub)
	})

	t.Run("WrongSignature", func(t *testing.T) {
		badManifest := manifest
		badManifest.Signature = hex.EncodeToString(make([]byte, 64))
		overrideManifestData, _ = json.Marshal(badManifest)
		err := InstallUpdate(info, nil)
		if err == nil || !strings.Contains(err.Error(), "signature verification failed") {
			t.Errorf("expected signature verification error, got %v", err)
		}
		overrideManifestData = nil
	})

	t.Run("LegacyColonPayloadRejected", func(t *testing.T) {
		// A signature over the OLD ambiguous colon-joined payload must
		// not verify against the canonical framing. This pins the fix
		// for the spoofable field-separator format: regressing either
		// side (signer or updater) to colon-joining fails here.
		legacy := fmt.Sprintf("%s:%s:%s:%d:%s",
			manifest.Version, manifest.Platform, manifest.Architecture, manifest.Length, manifest.Digest)
		badManifest := manifest
		badManifest.Signature = hex.EncodeToString(ed25519.Sign(priv, []byte(legacy)))
		overrideManifestData, _ = json.Marshal(badManifest)
		err := InstallUpdate(info, nil)
		if err == nil || !strings.Contains(err.Error(), "signature verification failed") {
			t.Errorf("expected legacy colon-joined payload signature to be rejected, got %v", err)
		}
		overrideManifestData = nil
	})

	t.Run("WrongVersion", func(t *testing.T) {
		badInfo := &UpdateInfo{
			TagName: "v2.0.0",
			Assets:  []ReleaseAsset{asset},
		}
		err := InstallUpdate(badInfo, nil)
		if err == nil || !strings.Contains(err.Error(), "version mismatch") {
			t.Errorf("expected version mismatch error, got %v", err)
		}
	})

	t.Run("WrongPlatform", func(t *testing.T) {
		badManifest := mustSignManifest(t, priv, UpdateManifest{
			Version:      "v1.0.0",
			Platform:     "wrongos",
			Architecture: expectedArch,
			Length:       int64(len(assetData)),
			Digest:       assetDigest,
		})
		overrideManifestData, _ = json.Marshal(badManifest)
		err := InstallUpdate(info, nil)
		if err == nil || !strings.Contains(err.Error(), "platform mismatch") {
			t.Errorf("expected platform mismatch error, got %v", err)
		}
		overrideManifestData = nil
	})

	t.Run("WrongArchitecture", func(t *testing.T) {
		badManifest := mustSignManifest(t, priv, UpdateManifest{
			Version:      "v1.0.0",
			Platform:     runtime.GOOS,
			Architecture: "wrongarch",
			Length:       int64(len(assetData)),
			Digest:       assetDigest,
		})
		overrideManifestData, _ = json.Marshal(badManifest)
		err := InstallUpdate(info, nil)
		if err == nil || !strings.Contains(err.Error(), "architecture mismatch") {
			t.Errorf("expected architecture mismatch error, got %v", err)
		}
		overrideManifestData = nil
	})

	t.Run("OversizedBody", func(t *testing.T) {
		overrideAssetData = append(assetData, []byte(" extra payload")...)
		_, cleanup, err := downloadToTemp(&asset, manifest, nil)
		if err == nil {
			t.Errorf("expected error for oversized body")
		}
		if cleanup != nil {
			cleanup()
		}
		overrideAssetData = nil
	})

	t.Run("TruncatedBody", func(t *testing.T) {
		overrideAssetData = assetData[:10]
		_, cleanup, err := downloadToTemp(&asset, manifest, nil)
		if err == nil {
			t.Errorf("expected error for truncated body")
		}
		if cleanup != nil {
			cleanup()
		}
		overrideAssetData = nil
	})

	t.Run("MaliciousFilename", func(t *testing.T) {
		malicious := ReleaseAsset{
			Name:               "../escape.tar.gz",
			BrowserDownloadURL: ts.URL + "/release.tar.gz",
		}
		_, _, err := downloadToTemp(&malicious, manifest, nil)
		if err == nil || !strings.Contains(err.Error(), "invalid asset name") {
			t.Errorf("expected invalid asset name error, got %v", err)
		}
	})
}

func TestFetchUpdateManifestRequiresOneBoundedExactObject(t *testing.T) {
	valid := UpdateManifest{
		Version:      "v1.0.0",
		Platform:     runtime.GOOS,
		Architecture: runtime.GOARCH,
		Length:       0,
		Digest:       "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
		Signature:    strings.Repeat("0", ed25519.SignatureSize*2),
	}
	validJSON, err := json.Marshal(valid)
	if err != nil {
		t.Fatal(err)
	}
	exactLimit := append(append([]byte(nil), validJSON...), bytes.Repeat([]byte(" "), maxManifestBytes-len(validJSON))...)

	tests := []struct {
		name string
		body []byte
		want string
	}{
		{name: "empty", body: nil, want: "empty"},
		{name: "whitespace", body: []byte(" \n\t"), want: "empty"},
		{name: "oversized", body: bytes.Repeat([]byte(" "), maxManifestBytes+1), want: "exceeds"},
		{name: "second object", body: append(append([]byte(nil), validJSON...), validJSON...), want: "trailing JSON"},
		{name: "trailing token", body: append(append([]byte(nil), validJSON...), []byte(" nope")...), want: "trailing data"},
		{name: "unknown field", body: []byte(`{"version":"v1","platform":"linux","architecture":"amd64","length":0,"digest":"e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855","signature":"00","extra":true}`), want: "unknown field"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			client := clientReturning(http.StatusOK, test.body, nil)
			if _, err := fetchUpdateManifest(client, "https://updates.invalid/manifest"); err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("expected error containing %q, got %v", test.want, err)
			}
		})
	}
	if _, err := fetchUpdateManifest(clientReturning(http.StatusOK, exactLimit, nil), "https://updates.invalid/manifest"); err != nil {
		t.Fatalf("manifest exactly at limit rejected: %v", err)
	}
}

func TestFetchAndDownloadPropagateResponseCloseErrors(t *testing.T) {
	manifest := UpdateManifest{
		Version:      "v1",
		Platform:     runtime.GOOS,
		Architecture: runtime.GOARCH,
		Length:       0,
		Digest:       "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
		Signature:    strings.Repeat("0", ed25519.SignatureSize*2),
	}
	body, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	closeFailure := errors.New("close failed")
	if _, err := fetchUpdateManifest(clientReturning(http.StatusOK, body, closeFailure), "https://updates.invalid/manifest"); err == nil || !strings.Contains(err.Error(), "close manifest response") {
		t.Fatalf("manifest response close error not propagated: %v", err)
	}

	asset := &ReleaseAsset{Name: "release.tar.gz", BrowserDownloadURL: "https://updates.invalid/release.tar.gz"}
	_, cleanup, err := downloadToTempWithClient(
		clientReturning(http.StatusOK, nil, closeFailure),
		asset,
		manifest,
		nil,
	)
	if cleanup != nil {
		cleanup()
	}
	if err == nil || !strings.Contains(err.Error(), "close download response") {
		t.Fatalf("download response close error not propagated: %v", err)
	}
}

func TestDownloadRejectsUnsafeLengthAndNonCanonicalDigestBeforeRequest(t *testing.T) {
	requests := 0
	client := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		requests++
		return nil, errors.New("must not request")
	})}
	asset := &ReleaseAsset{Name: "release.tar.gz", BrowserDownloadURL: "https://updates.invalid/release.tar.gz"}
	base := UpdateManifest{Digest: "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"}
	for _, length := range []int64{-1, updatemanifest.MaxArtifactBytes + 1, int64(^uint64(0) >> 1)} {
		manifest := base
		manifest.Length = length
		if _, _, err := downloadToTempWithClient(client, asset, manifest, nil); err == nil || !strings.Contains(err.Error(), "length") {
			t.Fatalf("unsafe length %d accepted: %v", length, err)
		}
	}
	manifest := base
	manifest.Digest = strings.ToUpper(base.Digest)
	if _, _, err := downloadToTempWithClient(client, asset, manifest, nil); err == nil || !strings.Contains(err.Error(), "canonical") {
		t.Fatalf("uppercase digest accepted: %v", err)
	}
	if requests != 0 {
		t.Fatalf("performed %d requests before manifest validation", requests)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req)
}

type closeErrorBody struct {
	io.Reader
	err error
}

func (body *closeErrorBody) Close() error {
	return body.err
}

func clientReturning(status int, body []byte, closeErr error) *http.Client {
	return &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: status,
			Status:     fmt.Sprintf("%d status", status),
			Body:       &closeErrorBody{Reader: bytes.NewReader(body), err: closeErr},
			Header:     make(http.Header),
		}, nil
	})}
}

func TestCanonicalHexRequiresExactEd25519SignatureLength(t *testing.T) {
	for _, encoded := range []string{
		strings.Repeat("00", ed25519.SignatureSize-1),
		strings.Repeat("00", ed25519.SignatureSize+1),
		strings.ToUpper(strings.Repeat("ab", ed25519.SignatureSize)),
	} {
		if _, err := decodeCanonicalHex(encoded, ed25519.SignatureSize, "manifest signature"); err == nil {
			t.Fatalf("invalid signature encoding accepted (length %d)", len(encoded))
		}
	}
	if _, err := decodeCanonicalHex(strings.Repeat("ab", ed25519.SignatureSize), ed25519.SignatureSize, "manifest signature"); err != nil {
		t.Fatalf("canonical signature rejected: %v", err)
	}
}

// TestArchiveEntryKeyAliases pins the alias detection shared by the zip
// and tar extractors: entries that would collapse onto one path on a
// case-insensitive (or normalization-insensitive) filesystem must map
// to the same collision key.
func TestArchiveEntryKeyAliases(t *testing.T) {
	aliases := [][2]string{
		{"Readme.txt", "readme.txt"},
		{"A", "a"},
		{"Caf\u00e9.png", "Cafe\u0301.png"}, // NFC vs NFD
		{"CONTENT/INFO.PLIST", "content/info.plist"},
		{"Stra\u00dfe.txt", "STRASSE.TXT"}, // full Unicode case folding
		{"content/", "CONTENT"},            // directory/file path alias
	}
	for _, pair := range aliases {
		if archiveEntryKey(pair[0]) != archiveEntryKey(pair[1]) {
			t.Errorf("expected %q and %q to be detected as aliases", pair[0], pair[1])
		}
	}
	distinct := [][2]string{
		{"readme.txt", "readme.md"},
		{"data", "datb"},
	}
	for _, pair := range distinct {
		if archiveEntryKey(pair[0]) == archiveEntryKey(pair[1]) {
			t.Errorf("distinct names %q and %q must not collide", pair[0], pair[1])
		}
	}
	// The destination-relative path form used by callers must stay stable.
	p := filepath.ToSlash(filepath.Join("a", "B.txt"))
	if archiveEntryKey(p) != archiveEntryKey("a/b.txt") {
		t.Errorf("expected separator form to be irrelevant, got %q vs %q",
			archiveEntryKey(p), archiveEntryKey("a/b.txt"))
	}
}
