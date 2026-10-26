package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"testing"
)

// TestDevFieldDriftAgainstSchema is the contract between devFieldsRegistry
// (the runtime Go list that drives the Advanced (Developer) Fields card)
// and schemas/mbch_schema.json (the documentation-of-record). If they
// disagree, the runtime form will be missing keys the schema marks dev
// — or include stale ones the schema has unmarked. Failing the build
// is the only way to keep them honest: there is no compiler-level link
// between them.
//
// Drift sources we have seen on other projects: someone adds a new
// dev-flagged field to the schema without touching the Go registry,
// or vice-versa during refactors. This test catches both directions
// of drift with one diff.
func TestDevFieldDriftAgainstSchema(t *testing.T) {
	// Locate the schema relative to the package dir. `go test` sets
	// cwd to the package directory; schemas/ lives one level up.
	schemaPath := filepath.Join("..", "schemas", "mbch_schema.json")
	data, err := os.ReadFile(schemaPath)
	if err != nil {
		t.Fatalf("read schema (%s): %v", schemaPath, err)
	}

	// Walk only ClassInfo for now — dev fields live there. If a future
	// dev field gets added to WeaponInfo/ForceInfo, expand this.
	var root struct {
		Definitions struct {
			ClassInfo struct {
				Properties map[string]struct {
					Dev *bool `json:"dev,omitempty"`
				} `json:"properties"`
			} `json:"ClassInfo"`
		} `json:"definitions"`
	}
	if err := json.Unmarshal(data, &root); err != nil {
		t.Fatalf("parse schema: %v", err)
	}

	var schemaDevKeys []string
	for name, prop := range root.Definitions.ClassInfo.Properties {
		if prop.Dev != nil && *prop.Dev {
			schemaDevKeys = append(schemaDevKeys, name)
		}
	}
	sort.Strings(schemaDevKeys)

	goDevKeys := devFieldKeysSorted()

	if !equalStringSlices(schemaDevKeys, goDevKeys) {
		t.Errorf("dev-field drift between schema and Go registry:\n"+
			"  schema (%d): %v\n"+
			"  Go registry (%d): %v\n"+
			"Fix: edit both schemas/mbch_schema.json and go_module/dev_fields.go.",
			len(schemaDevKeys), schemaDevKeys,
			len(goDevKeys), goDevKeys,
		)
	}
}

func equalStringSlices(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
