package main

import (
	"errors"
	"strings"
	"testing"

	"github.com/zalando/go-keyring"
)

func TestOversizedCredentialPreservesStoredValue(t *testing.T) {
	keyring.MockInit()
	t.Cleanup(keyring.MockInit)
	if err := SaveGitHubToken("existing-fixture"); err != nil { t.Fatal(err) }
	if err := SaveGitHubToken(strings.Repeat("x", MaxTokenSize+1)); !errors.Is(err, ErrSetDataTooBig) { t.Fatal("oversized credential was not rejected") }
	got, err := LoadGitHubToken()
	if err != nil || got != "existing-fixture" { t.Fatal("rejected credential changed existing storage") }
}

func TestLegacyMigrationCannotReplaceNewerCredential(t *testing.T) {
	keyring.MockInit()
	t.Cleanup(keyring.MockInit)
	if err := SaveGitHubToken("newer-fixture"); err != nil { t.Fatal(err) }
	if err := MigrateLegacyGitHubToken("older-fixture"); err != nil { t.Fatal(err) }
	got, err := LoadGitHubToken()
	if err != nil || got != "newer-fixture" { t.Fatal("stale plaintext migration overwrote newer native credential") }
}

func TestLegacyMigrationRequiresVerifiedNativePublication(t *testing.T) {
	const legacy = "legacy-fixture"
	stored := ""
	readErr := errors.New("verification unavailable")
	reads := 0

	_, err := migrateLegacyGitHubToken(
		legacy,
		func() (string, error) {
			reads++
			if reads == 1 {
				return "", nil
			}
			if readErr != nil {
				return "", readErr
			}
			return stored, nil
		},
		func(token string) error {
			stored = token
			return nil
		},
	)
	if err == nil {
		t.Fatal("unverified native publication was reported as complete")
	}
	if stored != legacy {
		t.Fatal("fixture did not reach the native provider before verification failed")
	}

	readErr = nil
	got, err := migrateLegacyGitHubToken(
		legacy,
		func() (string, error) { return stored, nil },
		func(string) error {
			t.Fatal("retry overwrote the credential that was already published")
			return nil
		},
	)
	if err != nil || got != legacy {
		t.Fatalf("retry did not recognize verified native publication: token=%q err=%v", got, err)
	}
}

func TestLegacyMigrationProviderFailureDoesNotPublish(t *testing.T) {
	writeErr := errors.New("native provider locked")
	writes := 0
	_, err := migrateLegacyGitHubToken(
		"legacy-fixture",
		func() (string, error) { return "", nil },
		func(string) error {
			writes++
			return writeErr
		},
	)
	if !errors.Is(err, writeErr) || writes != 1 {
		t.Fatalf("provider failure was not returned intact: writes=%d err=%v", writes, err)
	}
}

func TestEmptyLegacyMigrationDoesNotOpenNativeProvider(t *testing.T) {
	token, err := migrateLegacyGitHubToken(
		"",
		func() (string, error) {
			t.Fatal("empty migration opened native provider")
			return "", nil
		},
		func(string) error {
			t.Fatal("empty migration wrote native provider")
			return nil
		},
	)
	if err != nil || token != "" {
		t.Fatalf("empty migration returned token=%q err=%v", token, err)
	}
}
