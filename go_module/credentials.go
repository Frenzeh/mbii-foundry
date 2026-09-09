package main

import (
	"errors"
	"fmt"

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
