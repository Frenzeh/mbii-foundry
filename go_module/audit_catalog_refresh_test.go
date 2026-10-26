package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const auditInstalledAttributes = `[
	{"id":"MB_ATT_PISTOL","name":"Installed pistol","max_level":5,"description":"Installed rank documentation","levels":{"1":{"effect":"Retain installed level documentation"}}},
	{"id":"MB_ATT_LOCAL_ONLY","name":"Installed custom attribute","max_level":2}
]`

const auditInstalledWeapons = `[
	{"id":"WP_BLASTER","name":"Installed blaster","description":"Installed weapon documentation"},
	{"id":"WP_LOCAL_ONLY","name":"Installed custom weapon"}
]`

const auditRefreshedAttributes = `[
	{"id":"MB_ATT_PISTOL","name":"✨ Refreshed Pistol","max_level":0,"future_metadata":true},
	{"id":"MB_ATT_FP_HEAL","name":"Refreshed Heal","category":"General","max_level":0},
	{"id":"MB_ATT_CYBERNETICS","name":"Cybernetics","max_level":0},
	{"id":"MB_ATT_INVALID","name":"Sentinel","max_level":0},
	{"id":"MB_ATT_PUBLIC_CUSTOM","name":"Public custom attribute","max_level":7}
]`

const auditRefreshedWeapons = `[
	{"id":"WP_BLASTER","name":"💥 Refreshed Blaster"},
	{"id":"WP_NONE","name":"Sentinel"},
	{"id":"WP_PUBLIC_CUSTOM","name":"Public custom weapon"}
]`

const auditPreviousCatalog = `{"attributes":[{"id":"MB_ATT_PISTOL","name":"Previously cached pistol","max_level":2}],"weapons":[{"id":"WP_BLASTER","name":"Previously cached blaster"}]}`

func auditWriteCatalogFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
}

func auditUseCatalogTransport(t *testing.T, transport roundTripFunc) {
	t.Helper()
	previous := http.DefaultTransport
	http.DefaultTransport = transport
	t.Cleanup(func() { http.DefaultTransport = previous })
}

func auditCatalogEnvironment(t *testing.T) (installed, cachePath string) {
	t.Helper()
	usePublicCatalogs(t)
	LoadedClassFlags, LoadedSaberStyles, LoadedGlossary = nil, nil, nil
	root := t.TempDir()
	t.Setenv("HOME", filepath.Join(root, "home"))
	t.Setenv("XDG_CACHE_HOME", filepath.Join(root, "cache"))
	t.Setenv("LocalAppData", filepath.Join(root, "local-app-data"))
	base, err := os.UserCacheDir()
	if err != nil {
		t.Fatal(err)
	}
	cachePath = filepath.Join(base, "mbii-foundry", "catalog.json")
	installed = filepath.Join(root, "Foundry.app", "Contents", "Resources", "data")
	auditWriteCatalogFile(t, filepath.Join(installed, "attributes.json"), auditInstalledAttributes)
	auditWriteCatalogFile(t, filepath.Join(installed, "weapons.json"), auditInstalledWeapons)
	auditUseCatalogTransport(t, func(req *http.Request) (*http.Response, error) {
		t.Errorf("unexpected HTTP request: %s", req.URL)
		return nil, errors.New("external HTTP is disabled in catalog tests")
	})
	return installed, cachePath
}

func auditCatalogReplies(t *testing.T, attrs, weapons string, weaponStatus int, closeErr error) {
	t.Helper()
	auditUseCatalogTransport(t, func(req *http.Request) (*http.Response, error) {
		body := ""
		status := http.StatusOK
		var bodyCloseErr error
		switch req.URL.String() {
		case "https://raw.githubusercontent.com/Frenzeh/mbii-foundry/main/data/attributes.json":
			body = attrs
		case "https://raw.githubusercontent.com/Frenzeh/mbii-foundry/main/data/weapons.json":
			body = weapons
			status = weaponStatus
			bodyCloseErr = closeErr
		default:
			t.Errorf("catalog request left its HTTPS main-branch source: %s", req.URL)
			return nil, errors.New("unexpected catalog destination")
		}
		return &http.Response{
			StatusCode: status,
			Header:     make(http.Header),
			Body:       &closeErrorBody{Reader: strings.NewReader(body), err: bodyCloseErr},
			Request:    req,
		}, nil
	})
}

func auditCatalogSnapshot(t *testing.T) []byte {
	t.Helper()
	data, err := json.Marshal([]any{GetAllAttributes(), GetAllWeapons(), GetAllClasses(), GetClassFlags(), GetSaberStyles(), GetGlossary()})
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func auditAssertInstalledCatalogUnchanged(t *testing.T, installed string) {
	t.Helper()
	for name, want := range map[string]string{"attributes.json": auditInstalledAttributes, "weapons.json": auditInstalledWeapons} {
		got, err := os.ReadFile(filepath.Join(installed, name))
		if err != nil {
			t.Fatal(err)
		}
		if string(got) != want {
			t.Errorf("manual refresh modified installed resource %s", name)
		}
	}
}

func auditCatalogWeapon(t *testing.T, id string) WeaponDef {
	t.Helper()
	for _, weapon := range GetAllWeapons() {
		if weapon.ID == id {
			return weapon
		}
	}
	t.Fatalf("missing weapon %s", id)
	return WeaponDef{}
}

func TestAuditCatalogRefreshPersistsWithoutPublishingOrChangingResources(t *testing.T) {
	installed, cachePath := auditCatalogEnvironment(t)
	auditWriteCatalogFile(t, cachePath, auditPreviousCatalog)
	if err := LoadExternalData(installed); err != nil {
		t.Fatal(err)
	}
	before := auditCatalogSnapshot(t)
	auditCatalogReplies(t, auditRefreshedAttributes, auditRefreshedWeapons, http.StatusOK, nil)
	if _, err := UpdateDataFromGitHub(); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, auditCatalogSnapshot(t)) {
		t.Fatal("background HTTP/cache preparation published shared catalog state")
	}
	auditAssertInstalledCatalogUnchanged(t, installed)

	data, err := os.ReadFile(cachePath)
	if err != nil {
		t.Fatalf("successful manual refresh did not persist a user cache: %v", err)
	}
	var cached struct {
		Attributes []AttributeDef `json:"attributes"`
		Weapons    []WeaponDef    `json:"weapons"`
	}
	if err := json.Unmarshal(data, &cached); err != nil {
		t.Fatal(err)
	}
	if len(cached.Attributes) != 5 || len(cached.Weapons) != 3 || cached.Attributes[0].MaxLevel != 0 {
		t.Fatalf("cache did not retain the fetched catalog pair independently of installed overrides: %+v", cached)
	}
	if err := LoadExternalData(installed); err != nil {
		t.Fatal(err)
	}
	pistol := currentCatalogAttribute(t, "MB_ATT_PISTOL")
	if pistol.Name != "Refreshed Pistol" || pistol.MaxLevel != 5 || pistol.Description != "Installed rank documentation" || pistol.Levels["1"].Effect != "Retain installed level documentation" {
		t.Fatalf("cached refresh bypassed the startup metadata/rank merge: %+v", pistol)
	}
	if weapon := auditCatalogWeapon(t, "WP_BLASTER"); weapon.Name != "Refreshed Blaster" || weapon.Description != "Installed weapon documentation" {
		t.Fatalf("cached refresh bypassed the startup weapon merge/name policy: %+v", weapon)
	}
	heal := currentCatalogAttribute(t, "MB_ATT_FP_HEAL")
	if heal.Category != "Force" || heal.MaxLevel != 3 {
		t.Fatalf("cached refresh bypassed categorization or embedded rank fallback: %+v", heal)
	}
	if got := currentCatalogAttribute(t, "MB_ATT_CYBERNETICS").MaxLevel; got != 0 {
		t.Fatalf("refresh invented an undocumented rank limit: %d", got)
	}
	if !currentCatalogAttribute(t, "MB_ATT_INVALID").Hidden || !auditCatalogWeapon(t, "WP_NONE").Hidden {
		t.Fatal("cached sentinels were not marked hidden")
	}
	publicAttributeFound := false
	for _, attr := range GetAttributes() {
		if attr.ID == "MB_ATT_INVALID" {
			t.Fatal("cached sentinel leaked into public attribute pickers")
		}
		if attr.ID == "MB_ATT_PUBLIC_CUSTOM" {
			publicAttributeFound = true
		}
	}
	publicWeaponFound := false
	for _, weapon := range GetWeapons() {
		if weapon.ID == "WP_NONE" {
			t.Fatal("cached sentinel leaked into public weapon pickers")
		}
		if weapon.ID == "WP_PUBLIC_CUSTOM" {
			publicWeaponFound = true
		}
	}
	if !publicAttributeFound || !publicWeaponFound {
		t.Fatal("refreshed public entries were hidden from default pickers")
	}
	currentCatalogAttribute(t, "MB_ATT_LOCAL_ONLY")
	currentCatalogAttribute(t, "MB_ATT_PUBLIC_CUSTOM")
	currentCatalogAttribute(t, "MB_ATT_STAMINA")
	auditCatalogWeapon(t, "WP_LOCAL_ONLY")
	auditCatalogWeapon(t, "WP_PUBLIC_CUSTOM")
	auditCatalogWeapon(t, "WP_MELEE")

	// Startup must also consume the cache when no resource directory was found.
	if err := LoadExternalData(""); err != nil {
		t.Fatal(err)
	}
	pistol = currentCatalogAttribute(t, "MB_ATT_PISTOL")
	if pistol.Name != "Refreshed Pistol" || pistol.MaxLevel != 3 {
		t.Fatalf("cache-only startup lost refreshed metadata or embedded rank fallback: %+v", pistol)
	}
	currentCatalogAttribute(t, "MB_ATT_STAMINA")
	auditCatalogWeapon(t, "WP_MELEE")
	if !currentCatalogAttribute(t, "MB_ATT_INVALID").Hidden || !auditCatalogWeapon(t, "WP_NONE").Hidden {
		t.Fatal("cache-only startup lost visibility policy")
	}
	auditAssertInstalledCatalogUnchanged(t, installed)
}

func TestAuditCatalogRefreshFailurePreservesCatalogPair(t *testing.T) {
	many := make([]AttributeDef, 4097)
	for i := range many {
		many[i].ID = fmt.Sprintf("MB_ATT_SYNTHETIC_%d", i)
	}
	tooMany, err := json.Marshal(many)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name, attrs, weapons, want string
		status                     int
		closeErr                   error
	}{
		{name: "second request fails", want: "weapons.json: HTTP 503", status: http.StatusServiceUnavailable},
		{name: "null attributes", attrs: "null", want: "attributes must contain"},
		{name: "empty weapons", weapons: "[]", want: "weapons must contain"},
		{name: "wrong top level", attrs: `{}`, want: "attributes parse"},
		{name: "wrong field type", attrs: `[{"id":"MB_ATT_PISTOL","max_level":"3"}]`, want: "attributes parse"},
		{name: "missing identifier", attrs: `[{"name":"No identifier"}]`, want: "identifier"},
		{name: "invalid identifier", attrs: `[{"id":"two words"}]`, want: "identifier"},
		{name: "duplicate attributes", attrs: `[{"id":"MB_ATT_PISTOL"},{"id":"MB_ATT_PISTOL"}]`, want: "duplicate"},
		{name: "duplicate weapons", weapons: `[{"id":"WP_BLASTER"},{"id":"WP_BLASTER"}]`, want: "duplicate"},
		{name: "negative rank", attrs: `[{"id":"MB_ATT_PISTOL","max_level":-1}]`, want: "outside 0–255"},
		{name: "rank exceeds byte", attrs: `[{"id":"MB_ATT_PISTOL","max_level":256}]`, want: "outside 0–255"},
		{name: "too many entries", attrs: string(tooMany), want: "attributes must contain"},
		{name: "bounded response", attrs: strings.Repeat(" ", (4<<20)+1), want: "exceeds scan limit"},
		{name: "response close fails", want: "close failed", closeErr: errors.New("close failed")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			installed, cachePath := auditCatalogEnvironment(t)
			auditWriteCatalogFile(t, cachePath, auditPreviousCatalog)
			if err := LoadExternalData(installed); err != nil {
				t.Fatal(err)
			}
			before := auditCatalogSnapshot(t)
			attrs, weapons, status := tc.attrs, tc.weapons, tc.status
			if attrs == "" {
				attrs = auditRefreshedAttributes
			}
			if weapons == "" {
				weapons = auditRefreshedWeapons
			}
			if status == 0 {
				status = http.StatusOK
			}
			auditCatalogReplies(t, attrs, weapons, status, tc.closeErr)
			if _, err := UpdateDataFromGitHub(); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("refresh failure was not reported accurately: got %v, want %q", err, tc.want)
			}
			if !bytes.Equal(before, auditCatalogSnapshot(t)) {
				t.Fatal("failed refresh published part of a catalog generation")
			}
			data, err := os.ReadFile(cachePath)
			if err != nil || string(data) != auditPreviousCatalog {
				t.Fatalf("failed refresh replaced the previous catalog pair: %q, %v", data, err)
			}
			auditAssertInstalledCatalogUnchanged(t, installed)
		})
	}
}

func TestAuditCatalogRefreshReportsCacheWriteFailure(t *testing.T) {
	for _, failure := range []string{"cache parent is a file", "cache destination is a directory", "cache directory unavailable"} {
		t.Run(failure, func(t *testing.T) {
			installed, cachePath := auditCatalogEnvironment(t)
			if err := LoadExternalData(installed); err != nil {
				t.Fatal(err)
			}
			before := auditCatalogSnapshot(t)
			sentinel := "preserve this cache location"
			var retainedPath string
			switch failure {
			case "cache parent is a file":
				retainedPath = filepath.Dir(cachePath)
				auditWriteCatalogFile(t, retainedPath, sentinel)
			case "cache destination is a directory":
				retainedPath = filepath.Join(cachePath, "retained.txt")
				auditWriteCatalogFile(t, retainedPath, sentinel)
			case "cache directory unavailable":
				t.Setenv("HOME", "")
				t.Setenv("XDG_CACHE_HOME", "")
				t.Setenv("LocalAppData", "")
			}
			auditCatalogReplies(t, auditRefreshedAttributes, auditRefreshedWeapons, http.StatusOK, nil)
			if _, err := UpdateDataFromGitHub(); err == nil || !strings.Contains(err.Error(), "not saved") {
				t.Fatalf("cache persistence failure was reported as a successful update: %v", err)
			}
			if !bytes.Equal(before, auditCatalogSnapshot(t)) {
				t.Fatal("cache write failure still published the downloaded catalogs")
			}
			if retainedPath != "" {
				data, err := os.ReadFile(retainedPath)
				if err != nil || string(data) != sentinel {
					t.Fatalf("failed refresh modified the existing cache location: %q, %v", data, err)
				}
			}
			auditAssertInstalledCatalogUnchanged(t, installed)
		})
	}
}

func TestAuditCatalogStartupRejectsInvalidCacheAsAWhole(t *testing.T) {
	for _, tc := range []struct{ name, cache string }{
		{"truncated", `{"attributes":[`},
		{"null pair", `{"attributes":null,"weapons":null}`},
		{"invalid second catalog", `{"attributes":[{"id":"MB_ATT_PISTOL","name":"Must not publish"}],"weapons":{}}`},
		{"invalid rank", `{"attributes":[{"id":"MB_ATT_PISTOL","max_level":256}],"weapons":[{"id":"WP_BLASTER"}]}`},
		{"duplicate IDs", `{"attributes":[{"id":"MB_ATT_PISTOL"},{"id":"MB_ATT_PISTOL"}],"weapons":[{"id":"WP_BLASTER"}]}`},
		{"bounded cache", strings.Repeat(" ", (8<<20)+1)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			installed, cachePath := auditCatalogEnvironment(t)
			if err := LoadExternalData(installed); err != nil {
				t.Fatal(err)
			}
			before := auditCatalogSnapshot(t)
			auditWriteCatalogFile(t, cachePath, tc.cache)
			if err := LoadExternalData(installed); err != nil {
				t.Fatalf("invalid optional cache prevented fallback startup: %v", err)
			}
			if !bytes.Equal(before, auditCatalogSnapshot(t)) {
				t.Fatal("invalid cache replaced or partially overlaid installed/embedded catalogs")
			}
			data, err := os.ReadFile(cachePath)
			if err != nil || string(data) != tc.cache {
				t.Fatalf("startup rewrote the optional cache: %v", err)
			}
			auditAssertInstalledCatalogUnchanged(t, installed)
		})
	}
}

func TestAuditCatalogFetchDoesNotFollowRedirects(t *testing.T) {
	auditCatalogEnvironment(t)
	requests := 0
	auditUseCatalogTransport(t, func(req *http.Request) (*http.Response, error) {
		requests++
		return &http.Response{
			StatusCode: http.StatusFound,
			Header:     http.Header{"Location": []string{req.URL.String() + "?redirected=1"}},
			Body:       &closeErrorBody{Reader: strings.NewReader(""), err: nil},
			Request:    req,
		}, nil
	})
	if _, err := FetchDataFromGitHub("attributes.json"); err == nil || !strings.Contains(err.Error(), "redirect") {
		t.Fatalf("catalog redirect did not fail: %v", err)
	}
	if requests != 1 {
		t.Fatalf("followed a redirect away from the fixed main-branch URL: %d requests", requests)
	}
}
