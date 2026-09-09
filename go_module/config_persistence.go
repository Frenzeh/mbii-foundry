package main

import (
	"encoding/json"
	"fmt"

	"github.com/Frenzeh/mbii-foundry/safeio"
)

// persistConfig never removes a legacy credential until native storage succeeds.
// A failed publication keeps the pending migration retryable and the old file intact.
func (a *App) persistConfig() error {
	if a.configLoadError != nil {
		return a.configLoadError
	}
	if a.configPath == "" {
		return fmt.Errorf("configuration directory is unavailable")
	}
	if a.legacyTokenPending != "" && !a.legacyCredentialSecured {
		return fmt.Errorf("legacy credential is preserved; use Connect GitHub to finish secure migration before saving preferences")
	}
	data, err := json.MarshalIndent(a.config, "", "  ")
	if err != nil {
		return err
	}
	if err := safeio.WriteFile(a.configPath, data, 0600); err != nil {
		return err
	}
	a.legacyTokenPending = ""
	a.legacyCredentialSecured = false
	return nil
}

