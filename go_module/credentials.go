package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/Frenzeh/mbii-foundry/safeio"
	"github.com/zalando/go-keyring"
)

const (
	keyringService = "mbii-foundry"
	keyringAccount = "github-token"

	// MaxTokenSize is the maximum allowed size for a GitHub token.
	// This prevents hitting backend command-size limits (e.g. macOS security tool limits)
	MaxTokenSize = 2048
)

var ErrSetDataTooBig = errors.New("credential data exceeds maximum allowed size")

// LoadGitHubToken retrieves the GitHub token from the native OS credential store.
// Returns an error if the token is not found or cannot be retrieved.
func LoadGitHubToken() (string, error) {
	token, err := keyring.Get(keyringService, keyringAccount)
	if err != nil {
		if errors.Is(err, keyring.ErrNotFound) {
			return "", nil // Returning empty string when not found is standard
		}
		return "", err
	}
	return token, nil
}

// SaveGitHubToken saves the GitHub token to the native OS credential store.
func SaveGitHubToken(token string) error {
	if len(token) > MaxTokenSize {
		return ErrSetDataTooBig
	}
	if token == "" {
		// If empty, we can just delete it from the keyring
		err := keyring.Delete(keyringService, keyringAccount)
		if err != nil && !errors.Is(err, keyring.ErrNotFound) {
			return err
		}
		return nil
	}

	err := keyring.Set(keyringService, keyringAccount, token)
	if err != nil {
		return fmt.Errorf("failed to save token to keyring: %w", err)
	}

	return nil
}

// MigrateLegacyGitHubToken moves an existing plaintext token into the native
// credential store and verifies that it can be read back before the caller
// removes the plaintext source.
func MigrateLegacyGitHubToken(legacyToken string) error {
	_, err := migrateLegacyGitHubToken(legacyToken, LoadGitHubToken, SaveGitHubToken)
	return err
}

func migrateLegacyGitHubToken(
	legacyToken string,
	read func() (string, error),
	write func(string) error,
) (string, error) {
	if legacyToken == "" {
		return "", nil
	}

	existing, err := read()
	if err != nil {
		return "", fmt.Errorf("failed to read native credential before migration: %w", err)
	}
	if existing != "" {
		return existing, nil
	}
	if err := write(legacyToken); err != nil {
		return "", err
	}

	published, err := read()
	if err != nil {
		return "", fmt.Errorf("native credential was written but could not be verified: %w", err)
	}
	if published != legacyToken {
		return "", fmt.Errorf("native credential publication could not be verified")
	}
	return published, nil
}

func legacyConfigToken(data []byte) (string, error) {
	var legacy struct {
		GitHubToken string `json:"github_token"`
	}
	if err := json.Unmarshal(data, &legacy); err != nil {
		return "", err
	}
	return legacy.GitHubToken, nil
}

// readRetainedConfigFile rejects links and verifies that the opened descriptor
// still identifies the regular file inspected before opening it.
func readRetainedConfigFile(path string) ([]byte, os.FileInfo, error) {
	info, err := os.Lstat(path)
	if os.IsNotExist(err) {
		return nil, nil, nil
	}
	if err != nil {
		return nil, nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, nil, fmt.Errorf("retained configuration is not a regular file")
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, nil, err
	}
	opened, statErr := file.Stat()
	if statErr != nil || !os.SameFile(info, opened) {
		return nil, nil, errors.Join(
			fmt.Errorf("retained configuration changed while opening"),
			statErr,
			file.Close(),
		)
	}
	data, readErr := io.ReadAll(file)
	if err := errors.Join(readErr, file.Close()); err != nil {
		return nil, nil, err
	}
	return data, opened, nil
}

// retainedConfig is the pre-rebrand backup exactly as inspected.
type retainedConfig struct {
	path     string
	data     []byte
	info     os.FileInfo
	token    string
	recorded bool
}

// inspectRetainedConfig never opens the native store. Nil means no known backup
// plaintext; unreadable, linked or malformed backups are left alone.
func inspectRetainedConfig(configPath string) *retainedConfig {
	path, err := legacyConfigCandidate(configPath)
	if err != nil || path == "" {
		return nil
	}
	if info, err := os.Lstat(filepath.Dir(path)); err != nil || !info.IsDir() {
		return nil
	}
	data, info, err := readRetainedConfigFile(path)
	if err != nil || info == nil {
		return nil
	}
	token, err := legacyConfigToken(data)
	if err != nil || token == "" {
		return nil
	}
	return &retainedConfig{path: path, data: data, info: info, token: token,
		recorded: legacyConfigSourceRecorded(configPath, path)}
}

// removeRetainedCredential deletes only credential fields, keeping every other
// backup value, and only while the backup is still the inspected file and bytes.
func removeRetainedCredential(retained *retainedConfig) error {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(retained.data, &fields); err != nil {
		return err
	}
	for name := range fields {
		// encoding/json matched the legacy field case-insensitively.
		if strings.EqualFold(name, "github_token") {
			delete(fields, name)
		}
	}
	sanitized, err := json.MarshalIndent(fields, "", "  ")
	if err != nil {
		return err
	}
	current, info, err := readRetainedConfigFile(retained.path)
	if err != nil {
		return err
	}
	if info == nil || !os.SameFile(info, retained.info) || !bytes.Equal(current, retained.data) {
		return errors.New("the backup changed after it was inspected")
	}
	return safeio.WriteFile(retained.path, sanitized, retained.info.Mode().Perm())
}

func retainedCredentialWarning(path string) string {
	return "The pre-rebrand configuration backup at " + path + " still contains a plaintext GitHub credential and was left unchanged. Connect GitHub removes it only after verifying secure storage and that the backup belongs to this installation; otherwise review that file manually. Local editing needs no account."
}

// cleanRetainedCredential runs only after verified native storage. The backup is
// never migrated, so it cannot revive a replaced or deleted credential. It is
// cleaned only when its migration was recorded or its token matches the active
// or verified credential; anything else stays unchanged and visible.
func cleanRetainedCredential(retained *retainedConfig, pending, verified string) string {
	if retained == nil {
		return ""
	}
	if verified == "" || !(retained.recorded || retained.token == pending || retained.token == verified) {
		return retainedCredentialWarning(retained.path)
	}
	if err := removeRetainedCredential(retained); err != nil {
		return "Credential secured, but its plaintext copy in the pre-rebrand configuration backup at " + retained.path + " could not be removed (" + err.Error() + "). The backup was left unchanged; retry Connect GitHub. Local editing remains available."
	}
	return ""
}

// loadLegacyCredentialState records active plaintext awaiting migration and
// reports known backup plaintext. Nil data means no active configuration exists.
func (a *App) loadLegacyCredentialState(data []byte) error {
	if data != nil {
		token, err := legacyConfigToken(data)
		if err != nil {
			return err
		}
		a.legacyTokenPending = token
	}
	if retained := inspectRetainedConfig(a.configPath); retained != nil {
		a.credentialWarning = retainedCredentialWarning(retained.path)
	}
	return nil
}
