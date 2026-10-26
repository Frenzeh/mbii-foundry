package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
)

// Retained-backup warnings and cleanup failures both tell the user this.
const auditRetainedUnchanged = "left unchanged"

// auditCredentialHome isolates os.UserConfigDir in a synthetic home.
func auditCredentialHome(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("APPDATA", filepath.Join(home, "AppData", "Roaming"))
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, "config"))
	base, err := os.UserConfigDir()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(base, 0700); err != nil {
		t.Fatal(err)
	}
	themeColor, density, variant := CurrentThemeColor, CurrentDensityScale, CurrentColorVariant
	t.Cleanup(func() {
		CurrentThemeColor, CurrentDensityScale, CurrentColorVariant = themeColor, density, variant
	})
	return base
}

func auditWriteCredentialFixture(t *testing.T, path string, data []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
}

func auditCredentialJSON(t *testing.T, fields map[string]string) []byte {
	t.Helper()
	data, err := json.Marshal(fields)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func auditCredentialBytes(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

// auditLoadCredentialApp starts the real configuration load, including migration.
func auditLoadCredentialApp(t *testing.T, ui fyne.App) *App {
	t.Helper()
	a := &App{fyneApp: ui}
	a.loadConfig()
	if a.configLoadError != nil {
		t.Fatalf("configuration failed to load: %v", a.configLoadError)
	}
	return a
}

func assertAuditCredentialBytes(t *testing.T, path string, expected []byte) {
	t.Helper()
	if !bytes.Equal(auditCredentialBytes(t, path), expected) {
		t.Fatalf("configuration changed unexpectedly: %s", path)
	}
}

// assertAuditSanitizedBackup requires every nonsecret backup value to survive.
func assertAuditSanitizedBackup(t *testing.T, path string, original []byte) {
	t.Helper()
	var expected, actual map[string]json.RawMessage
	if err := json.Unmarshal(original, &expected); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(auditCredentialBytes(t, path), &actual); err != nil {
		t.Fatal(err)
	}
	for key := range expected {
		if strings.EqualFold(key, "github_token") {
			delete(expected, key)
		}
	}
	if len(actual) != len(expected) {
		t.Fatalf("retained configuration lost nonsecret fields or kept a credential: got %d fields, want %d", len(actual), len(expected))
	}
	for key, value := range expected {
		var want, got bytes.Buffer
		if err := json.Compact(&want, value); err != nil {
			t.Fatal(err)
		}
		if err := json.Compact(&got, actual[key]); err != nil {
			t.Fatalf("nonsecret field %s disappeared: %v", key, err)
		}
		if !bytes.Equal(want.Bytes(), got.Bytes()) {
			t.Fatalf("nonsecret backup field changed: %s", key)
		}
	}
}

func TestAuditRebrandCredentialCleanupRequiresVerifiedPublication(t *testing.T) {
	base := auditCredentialHome(t)
	ui := test.NewApp()
	defer ui.Quit()
	store := &testCredentialStore{}
	installTestCredentialStore(t, store)
	reads := 0
	readNativeCredential = func() (string, error) {
		reads++
		if reads == 2 {
			return "", errors.New("fixture verification unavailable")
		}
		return store.read()
	}
	original := []byte(`{"github_token":"audit-legacy-secret","GITHUB_TOKEN":"audit-legacy-secret","primary_color":"blue","github_user":"fixture-user","backup_only":{"large_integer":9007199254740993,"list":[true,"keep"]}}`)
	oldPath := filepath.Join(base, "mbii-fa-creator", "config.json")
	auditWriteCredentialFixture(t, oldPath, original)
	favorites := []byte(`["fixture/character.mbch"]`)
	oldFavorites := filepath.Join(filepath.Dir(oldPath), "favorites.json")
	auditWriteCredentialFixture(t, oldFavorites, favorites)
	a := auditLoadCredentialApp(t, ui)
	if reads != 0 {
		t.Fatal("loading legacy configuration opened the native credential store")
	}
	assertAuditCredentialBytes(t, oldPath, original)
	assertAuditCredentialBytes(t, a.configPath, original)
	if err := a.persistConfig(); err == nil {
		t.Fatal("preferences could erase the only active credential before native publication")
	}
	if err := awaitCredentialAction(t, a.loadCredentials); err == nil {
		t.Fatal("unverified native publication allowed plaintext cleanup")
	}
	assertAuditCredentialBytes(t, oldPath, original)
	assertAuditCredentialBytes(t, a.configPath, original)
	if !strings.Contains(strings.ToLower(a.credentialWarning), "retry") {
		t.Fatal("failed migration did not expose a retryable warning")
	}
	if err := a.persistConfig(); err == nil {
		t.Fatal("failed verification let preferences erase active plaintext")
	}
	readNativeCredential = store.read
	a.config.PrimaryColor = "gold"
	if err := awaitCredentialAction(t, a.loadCredentials); err != nil {
		t.Fatal(err)
	}
	assertAuditSanitizedBackup(t, oldPath, original)
	if bytes.Contains(auditCredentialBytes(t, a.configPath), []byte("audit-legacy-secret")) {
		t.Fatal("active configuration retained plaintext after verified publication")
	}
	if store.token != "audit-legacy-secret" || a.config.GitHubToken != "audit-legacy-secret" {
		t.Fatal("credential migration lost the verified native credential")
	}
	if a.credentialWarning != "" {
		t.Fatalf("completed cleanup still reports plaintext: %q", a.credentialWarning)
	}
	assertAuditCredentialBytes(t, oldFavorites, favorites)
	assertAuditCredentialBytes(t, filepath.Join(filepath.Dir(a.configPath), "favorites.json"), favorites)
	restarted := auditLoadCredentialApp(t, ui)
	if restarted.credentialWarning != "" || restarted.legacyTokenPending != "" || restarted.config.PrimaryColor != "gold" {
		t.Fatal("completed cleanup reappeared after restart or lost active preferences")
	}
}

func TestAuditRetainedCredentialCleanupEligibility(t *testing.T) {
	for _, tc := range []struct {
		name                                 string
		recorded                             bool
		active, native, retained, wantNative string
		clean                                bool
	}{
		{"recorded verified replacement", true, "legacy-secret", "replacement-secret", "legacy-secret", "replacement-secret", true},
		{"recorded retained only", true, "", "current-secret", "older-secret", "current-secret", true},
		{"recorded without native credential", true, "", "", "older-secret", "", false},
		{"markerless matching active", false, "same-secret", "replacement-secret", "same-secret", "replacement-secret", true},
		{"markerless matching native", false, "", "same-secret", "same-secret", "same-secret", true},
		{"markerless differing native", false, "", "replacement-secret", "older-secret", "replacement-secret", false},
		{"markerless without native credential", false, "", "", "older-secret", "", false},
		{"markerless differing active", false, "active-secret", "", "older-secret", "active-secret", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			base := auditCredentialHome(t)
			ui := test.NewApp()
			defer ui.Quit()
			store := &testCredentialStore{token: tc.native}
			installTestCredentialStore(t, store)
			reads := 0
			readNativeCredential = func() (string, error) { reads++; return store.read() }
			oldPath := filepath.Join(base, "mbii-fa-creator", "config.json")
			activePath := filepath.Join(base, "mbii-foundry", "config.json")
			original := auditCredentialJSON(t, map[string]string{"github_token": tc.retained, "backup_only": "keep"})
			auditWriteCredentialFixture(t, oldPath, original)
			if tc.recorded {
				auditLoadCredentialApp(t, ui) // the real migration records its source
			}
			active := map[string]string{"primary_color": "gold"}
			if tc.active != "" {
				active["github_token"] = tc.active
			}
			auditWriteCredentialFixture(t, activePath, auditCredentialJSON(t, active))
			a := auditLoadCredentialApp(t, ui)
			if reads != 0 || a.credentialWarning == "" {
				t.Fatalf("known plaintext was hidden or inspected through the native store: reads=%d", reads)
			}
			if err := awaitCredentialAction(t, a.loadCredentials); err != nil {
				t.Fatal(err)
			}
			if store.token != tc.wantNative {
				t.Fatal("retained backup changed or revived the native credential")
			}
			if bytes.Contains(auditCredentialBytes(t, activePath), []byte("github_token")) {
				t.Fatal("verified Connect left active plaintext behind")
			}
			if tc.clean {
				assertAuditSanitizedBackup(t, oldPath, original)
				if a.credentialWarning != "" {
					t.Fatalf("completed cleanup still warns: %q", a.credentialWarning)
				}
				return
			}
			assertAuditCredentialBytes(t, oldPath, original)
			if !strings.Contains(a.credentialWarning, auditRetainedUnchanged) {
				t.Fatalf("unchanged backup plaintext was not reported: %q", a.credentialWarning)
			}
		})
	}
}

func TestAuditRetainedCredentialFailureStaysVisibleWithoutReviving(t *testing.T) {
	base := auditCredentialHome(t)
	ui := test.NewApp()
	defer ui.Quit()
	store := &testCredentialStore{}
	installTestCredentialStore(t, store)
	original := []byte(`{"github_token":"audit-retry-secret","backup_only":"keep"}`)
	oldPath := filepath.Join(base, "mbii-fa-creator", "config.json")
	auditWriteCredentialFixture(t, oldPath, original)
	a := auditLoadCredentialApp(t, ui)
	moved := oldPath + ".moved"
	replacement := []byte(`{"github_token":"audit-retry-secret","replacement_only":true}`)
	reads := 0
	readNativeCredential = func() (string, error) {
		reads++
		if reads == 2 { // the backup is replaced while publication is verified
			if err := os.Rename(oldPath, moved); err != nil {
				return "", err
			}
			if err := os.WriteFile(oldPath, replacement, 0600); err != nil {
				return "", err
			}
		}
		return store.read()
	}
	if err := awaitCredentialAction(t, a.loadCredentials); err != nil {
		t.Fatalf("verified credential was not made available: %v", err)
	}
	readNativeCredential = store.read
	assertAuditCredentialBytes(t, moved, original)
	assertAuditCredentialBytes(t, oldPath, replacement)
	if store.token != "audit-retry-secret" || bytes.Contains(auditCredentialBytes(t, a.configPath), []byte("audit-retry-secret")) {
		t.Fatal("verified migration did not secure and remove active plaintext")
	}
	if !strings.Contains(a.credentialWarning, auditRetainedUnchanged) || !strings.Contains(a.credentialWarning, "retry") {
		t.Fatalf("changed backup was not reported as retryable: %q", a.credentialWarning)
	}
	if err := a.persistConfig(); err != nil {
		t.Fatalf("retained backup blocked ordinary preferences: %v", err)
	}
	if err := awaitCredentialAction(t, func(done func(error)) { a.storeCredential("", done) }); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(a.credentialWarning, auditRetainedUnchanged) {
		t.Fatal("credential removal hid retained plaintext")
	}
	restarted := auditLoadCredentialApp(t, ui)
	if !strings.Contains(restarted.credentialWarning, auditRetainedUnchanged) {
		t.Fatal("restart hid retained plaintext")
	}
	if err := awaitCredentialAction(t, restarted.loadCredentials); err != nil {
		t.Fatal(err)
	}
	if store.token != "" || restarted.config.GitHubToken != "" {
		t.Fatal("retained backup revived a deleted credential")
	}
	assertAuditCredentialBytes(t, oldPath, replacement)
	if err := awaitCredentialAction(t, func(done func(error)) { restarted.storeCredential("audit-new-secret", done) }); err != nil {
		t.Fatal(err)
	}
	if err := awaitCredentialAction(t, restarted.loadCredentials); err != nil {
		t.Fatal(err)
	}
	assertAuditSanitizedBackup(t, oldPath, replacement)
	assertAuditCredentialBytes(t, moved, original)
	if store.token != "audit-new-secret" || restarted.credentialWarning != "" {
		t.Fatal("verified replacement did not complete recorded backup cleanup")
	}
}

func TestAuditRecordedRetainedCredentialWithoutActiveConfig(t *testing.T) {
	base := auditCredentialHome(t)
	ui := test.NewApp()
	defer ui.Quit()
	store := &testCredentialStore{}
	installTestCredentialStore(t, store)
	reads := 0
	readNativeCredential = func() (string, error) { reads++; return store.read() }
	original := []byte(`{"github_token":"audit-orphan-secret","backup_only":"keep"}`)
	oldPath := filepath.Join(base, "mbii-fa-creator", "config.json")
	auditWriteCredentialFixture(t, oldPath, original)
	migrated := auditLoadCredentialApp(t, ui)
	if err := os.Remove(migrated.configPath); err != nil {
		t.Fatal(err)
	}
	a := auditLoadCredentialApp(t, ui)
	if reads != 0 || a.legacyTokenPending != "" || !strings.Contains(a.credentialWarning, auditRetainedUnchanged) {
		t.Fatalf("missing active configuration hid backup plaintext or opened the native store: reads=%d warning=%q", reads, a.credentialWarning)
	}
	if err := a.persistConfig(); err != nil {
		t.Fatalf("retained-only backup blocked ordinary preferences: %v", err)
	}
	if err := awaitCredentialAction(t, a.loadCredentials); err != nil {
		t.Fatal(err)
	}
	if store.token != "" || a.config.GitHubToken != "" {
		t.Fatal("unverified backup plaintext was published as the native credential")
	}
	assertAuditCredentialBytes(t, oldPath, original)
	if !strings.Contains(a.credentialWarning, auditRetainedUnchanged) {
		t.Fatal("Connect without a native credential hid backup plaintext")
	}
	if err := awaitCredentialAction(t, func(done func(error)) { a.storeCredential("audit-current-secret", done) }); err != nil {
		t.Fatal(err)
	}
	assertAuditCredentialBytes(t, oldPath, original)
	if !strings.Contains(a.credentialWarning, auditRetainedUnchanged) {
		t.Fatal("unverified replacement implied backup cleanup")
	}
	if err := awaitCredentialAction(t, a.loadCredentials); err != nil {
		t.Fatal(err)
	}
	assertAuditSanitizedBackup(t, oldPath, original)
	if store.token != "audit-current-secret" || a.credentialWarning != "" {
		t.Fatal("verified Connect did not finish recorded backup cleanup")
	}
}

func TestAuditRetainedCredentialMetadataCannotNominateOtherFiles(t *testing.T) {
	base := auditCredentialHome(t)
	ui := test.NewApp()
	defer ui.Quit()
	store := &testCredentialStore{}
	installTestCredentialStore(t, store)
	oldPath := filepath.Join(base, "mbii-fa-creator", "config.json")
	auditWriteCredentialFixture(t, oldPath, []byte(`{"github_token":"audit-migrated-secret"}`))
	a := auditLoadCredentialApp(t, ui)
	retained := []byte(`{"github_token":"audit-retained-secret","backup_only":"keep"}`)
	auditWriteCredentialFixture(t, oldPath, retained)
	unrelated := filepath.Join(t.TempDir(), "unrelated.json")
	unrelatedData := []byte(`{"github_token":"audit-migrated-secret","keep":true}`)
	auditWriteCredentialFixture(t, unrelated, unrelatedData)
	auditWriteCredentialFixture(t, filepath.Join(filepath.Dir(a.configPath), ".legacy-config-source"), []byte(unrelated))
	restarted := auditLoadCredentialApp(t, ui)
	if err := awaitCredentialAction(t, restarted.loadCredentials); err != nil {
		t.Fatal(err)
	}
	if store.token != "audit-migrated-secret" {
		t.Fatal("active legacy credential was not migrated")
	}
	assertAuditCredentialBytes(t, unrelated, unrelatedData)
	assertAuditCredentialBytes(t, oldPath, retained)
	if !strings.Contains(restarted.credentialWarning, auditRetainedUnchanged) {
		t.Fatal("backup plaintext without proven provenance was not reported")
	}
}

func TestAuditRetainedCredentialLinksAreNeverFollowed(t *testing.T) {
	for _, linked := range []string{"file", "directory"} {
		t.Run(linked, func(t *testing.T) {
			base := auditCredentialHome(t)
			ui := test.NewApp()
			defer ui.Quit()
			store := &testCredentialStore{}
			installTestCredentialStore(t, store)
			oldDir := filepath.Join(base, "mbii-fa-creator")
			oldPath := filepath.Join(oldDir, "config.json")
			auditWriteCredentialFixture(t, oldPath, []byte(`{"github_token":"audit-link-secret"}`))
			a := auditLoadCredentialApp(t, ui)
			outsideDir := t.TempDir()
			outside := filepath.Join(outsideDir, "config.json")
			outsideData := []byte(`{"github_token":"audit-link-secret","outside":true}`)
			auditWriteCredentialFixture(t, outside, outsideData)
			link, target := oldPath, outside
			if linked == "directory" {
				link, target = oldDir, outsideDir
			}
			if err := os.RemoveAll(link); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(target, link); err != nil {
				t.Skipf("symlink unavailable: %v", err)
			}
			if err := awaitCredentialAction(t, a.loadCredentials); err != nil {
				t.Fatal(err)
			}
			assertAuditCredentialBytes(t, outside, outsideData)
			if info, err := os.Lstat(link); err != nil || info.Mode()&os.ModeSymlink == 0 {
				t.Fatal("credential cleanup replaced a linked backup")
			}
			if store.token != "audit-link-secret" {
				t.Fatal("active legacy credential was not migrated")
			}
		})
	}
}
