package main

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"fyne.io/fyne/v2/test"
)

func awaitCredentialAction(t *testing.T, action func(func(error))) error {
	t.Helper()
	done := make(chan error, 1)
	action(func(err error) { done <- err })
	select {
	case err := <-done:
		return err
	case <-time.After(5 * time.Second):
		t.Fatal("credential operation did not finish")
		return nil
	}
}

type testCredentialStore struct {
	token    string
	readErr  error
	writeErr error
}

func (s *testCredentialStore) read() (string, error) {
	if s.readErr != nil {
		return "", s.readErr
	}
	return s.token, nil
}

func (s *testCredentialStore) write(token string) error {
	if s.writeErr != nil {
		return s.writeErr
	}
	s.token = token
	return nil
}

func installTestCredentialStore(t *testing.T, store *testCredentialStore) {
	t.Helper()
	originalReader := readNativeCredential
	originalWriter := writeNativeCredential
	readNativeCredential = store.read
	writeNativeCredential = store.write
	t.Cleanup(func() {
		readNativeCredential = originalReader
		writeNativeCredential = originalWriter
	})
}

func TestConfigMigrationRetainsLegacyUntilSecurePublication(t *testing.T) {
	ui := test.NewApp()
	defer ui.Quit()
	store := &testCredentialStore{readErr: errors.New("credential store locked")}
	installTestCredentialStore(t, store)
	path := filepath.Join(t.TempDir(), "config.json")
	original := []byte(`{"github_token":"legacy-fixture-secret","primary_color":"blue"}`)
	if err := os.WriteFile(path, original, 0600); err != nil {
		t.Fatal(err)
	}
	a := &App{configPath: path, legacyTokenPending: "legacy-fixture-secret", config: AppConfig{PrimaryColor: "gold"}}
	if err := awaitCredentialAction(t, a.loadCredentials); err == nil {
		t.Fatal("locked store allowed legacy credential removal")
	}
	got, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(got, original) {
		t.Fatalf("failed migration changed original: %v", err)
	}
	store.readErr = nil
	if err := awaitCredentialAction(t, a.loadCredentials); err != nil {
		t.Fatal(err)
	}
	got, err = os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(got, []byte("legacy-fixture-secret")) || bytes.Contains(got, []byte("github_token")) {
		t.Fatal("successful migration persisted plaintext credential")
	}
	if store.token != "legacy-fixture-secret" {
		t.Fatal("successful publication lost migrated credential")
	}
}

func TestConfigPublicationFailureRemainsRetryable(t *testing.T) {
	ui := test.NewApp()
	defer ui.Quit()
	store := &testCredentialStore{}
	installTestCredentialStore(t, store)
	root := t.TempDir()
	originalPath := filepath.Join(root, "original.json")
	path := filepath.Join(root, "config.json")
	original := []byte(`{"github_token":"retry-fixture-secret"}`)
	if err := os.WriteFile(originalPath, original, 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(originalPath, path); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	a := &App{configPath: path, legacyTokenPending: "retry-fixture-secret"}
	if err := awaitCredentialAction(t, a.loadCredentials); err == nil {
		t.Fatal("unsafe destination accepted")
	}
	got, err := os.ReadFile(originalPath)
	if err != nil || !bytes.Equal(got, original) {
		t.Fatal("publication failure changed original")
	}
	if store.token != "retry-fixture-secret" {
		t.Fatal("credential was not safely retained after publication failure")
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(originalPath, path); err != nil {
		t.Fatal(err)
	}
	if err := a.persistConfig(); err != nil {
		t.Fatal(err)
	}
	got, err = os.ReadFile(path)
	if err != nil || bytes.Contains(got, []byte("retry-fixture-secret")) {
		t.Fatal("retry did not remove migrated plaintext")
	}
}

func TestExplicitCredentialDeletionCannotReviveLegacyOnRetry(t *testing.T) {
	ui := test.NewApp()
	defer ui.Quit()
	store := &testCredentialStore{}
	installTestCredentialStore(t, store)
	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, []byte(`{"github_token":"obsolete-fixture-secret"}`), 0600); err != nil {
		t.Fatal(err)
	}
	a := &App{configPath: path, legacyTokenPending: "obsolete-fixture-secret"}
	if err := awaitCredentialAction(t, func(done func(error)) { a.storeCredential("replacement-fixture-secret", done) }); err != nil {
		t.Fatal(err)
	}
	if store.token != "replacement-fixture-secret" {
		t.Fatal("explicit replacement did not supersede the legacy credential")
	}
	if err := awaitCredentialAction(t, func(done func(error)) { a.storeCredential("", done) }); err != nil {
		t.Fatal(err)
	}
	retained := path + ".retained"
	if err := os.Rename(path, retained); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(retained, path); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	if err := a.persistConfig(); err == nil {
		t.Fatal("unsafe publication unexpectedly succeeded")
	}
	if store.token != "" {
		t.Fatal("failed preferences publication revived a deleted credential")
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(retained, path); err != nil {
		t.Fatal(err)
	}
	if err := a.persistConfig(); err != nil {
		t.Fatal(err)
	}
	if store.token != "" {
		t.Fatal("preferences retry revived a deleted credential")
	}
}

func TestMalformedConfigurationCannotBeOverwritten(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	original := []byte(`{"primary_color":`)
	if err := os.WriteFile(path, original, 0600); err != nil {
		t.Fatal(err)
	}
	parseErr := errors.New("configuration is invalid; original preserved")
	a := &App{
		configPath:      path,
		configLoadError: parseErr,
		config:          AppConfig{PrimaryColor: "gold"},
	}
	if err := a.persistConfig(); !errors.Is(err, parseErr) {
		t.Fatalf("malformed configuration guard returned %v, want %v", err, parseErr)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, original) {
		t.Fatal("preferences save replaced malformed original configuration")
	}
}

func TestPendingPlaintextCredentialBlocksUnsecuredPreferencesWrite(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	original := []byte(`{"github_token":"legacy-fixture"}`)
	if err := os.WriteFile(path, original, 0600); err != nil {
		t.Fatal(err)
	}
	a := &App{configPath: path, legacyTokenPending: "legacy-fixture"}
	if err := a.persistConfig(); err == nil {
		t.Fatal("preferences write removed an unsecured legacy credential")
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, original) {
		t.Fatal("blocked preferences write changed the original configuration")
	}
}

func TestPendingCredentialProviderDoesNotBlockLocalEditing(t *testing.T) {
	ui := test.NewApp()
	defer ui.Quit()
	originalReader := readNativeCredential
	defer func() { readNativeCredential = originalReader }()
	started, release := make(chan struct{}), make(chan struct{})
	readNativeCredential = func() (string, error) { close(started); <-release; return "stored-fixture", nil }
	a := &App{}
	returned, done := make(chan struct{}), make(chan error, 1)
	go func() { a.loadCredentials(func(err error) { done <- err }); close(returned) }()
	<-started
	select {
	case <-returned:
	case <-time.After(2 * time.Second):
		close(release)
		<-done
		t.Fatal("native credential provider blocked the calling UI action")
	}
	entry := NewInputEntry()
	changed := false
	entry.OnChanged = func(string) { changed = true }
	test.Type(entry, "local draft remains editable")
	if !changed || entry.Text != "local draft remains editable" {
		close(release)
		<-done
		t.Fatal("local editor interaction did not complete while credential provider was pending")
	}
	if err := awaitCredentialAction(t, a.loadCredentials); err == nil {
		close(release)
		<-done
		t.Fatal("second provider operation was started while one was pending")
	}
	close(release)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if a.config.GitHubToken != "stored-fixture" {
		t.Fatal("completed credential was not usable by contributions")
	}
	readNativeCredential = func() (string, error) { return "", errors.New("locked fixture store") }
	if err := awaitCredentialAction(t, a.loadCredentials); err == nil {
		t.Fatal("native provider failure was hidden")
	}
	if a.config.GitHubToken != "stored-fixture" {
		t.Fatal("failed reload discarded a usable in-memory credential")
	}
}
