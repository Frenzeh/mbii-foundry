package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/Frenzeh/mbii-foundry/parsers"
)

type snapshotSource struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
}

type snapshotRange struct {
	Declaration string `json:"declaration"`
	Path        string `json:"path"`
	StartLine   int    `json:"start_line"`
	EndLine     int    `json:"end_line"`
}

type snapshotEnumValue struct {
	Name  string `json:"name"`
	Value int64  `json:"value"`
}

type snapshotEnum struct {
	Name   string              `json:"name"`
	Values []snapshotEnumValue `json:"values"`
}

type snapshotBounds struct {
	Minimum any `json:"minimum,omitempty"`
	Maximum any `json:"maximum,omitempty"`
}

type snapshotField struct {
	Key            string          `json:"key"`
	Type           any             `json:"type"`
	Default        any             `json:"default,omitempty"`
	Bounds         *snapshotBounds `json:"bounds,omitempty"`
	EvidenceStatus string          `json:"evidence_status"`
}

type engineSnapshot struct {
	FormatVersion     int              `json:"format_version"`
	EvidenceStatus    string           `json:"evidence_status"`
	UnavailableReason string           `json:"unavailable_reason,omitempty"`
	Revision          string           `json:"revision"`
	Sources           []snapshotSource `json:"sources"`
	SourceRanges      []snapshotRange  `json:"source_ranges"`
	Enums             []snapshotEnum   `json:"enums"`
	Fields            []snapshotField  `json:"fields"`
}

func TestEngineSnapshotMetadata(t *testing.T) {
	snapshotPath := filepath.Join("testdata", "engine_snapshot.json")
	content, err := os.ReadFile(snapshotPath)
	if err != nil {
		t.Fatalf("read derived engine metadata: %v", err)
	}
	var snapshot engineSnapshot
	if err := json.Unmarshal(content, &snapshot); err != nil {
		t.Fatalf("parse derived engine metadata: %v", err)
	}
	if snapshot.FormatVersion != 1 {
		t.Fatalf("snapshot format_version = %d, want 1", snapshot.FormatVersion)
	}
	canonical, err := json.MarshalIndent(snapshot, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	canonical = append(canonical, '\n')
	if !bytes.Equal(content, canonical) {
		t.Fatal("engine snapshot is not in deterministic canonical JSON form")
	}
	assertSortedUniqueFields(t, snapshot.Fields)

	switch snapshot.EvidenceStatus {
	case "Verified":
		assertVerifiedSnapshot(t, snapshot)
	case "Unverified":
		assertUnverifiedSnapshot(t, snapshot)
	default:
		t.Fatalf("unsupported snapshot evidence_status %q", snapshot.EvidenceStatus)
	}
}

func TestEngineSnapshotMatchesSchemaMetadata(t *testing.T) {
	snapshot := readEngineSnapshot(t)

	schemaFields := readSchemaFields(t)
	if snapshot.EvidenceStatus == "Unverified" && len(snapshot.Fields) != len(schemaFields) {
		t.Fatalf("unverified snapshot/schema field count differs: snapshot=%d schema=%d", len(snapshot.Fields), len(schemaFields))
	}
	for _, field := range snapshot.Fields {
		schemaField, ok := schemaFields[field.Key]
		if !ok {
			if field.Type != "unknown" || field.EvidenceStatus != "KeyVerified" {
				t.Errorf("engine-only field %q has unsupported metadata claims: %+v", field.Key, field)
			}
			continue
		}
		if !reflect.DeepEqual(field.Type, schemaField.Type) || !reflect.DeepEqual(field.Default, schemaField.Default) || !reflect.DeepEqual(field.Bounds, schemaField.Bounds) {
			t.Errorf("snapshot metadata drift for %q: snapshot=%+v schema=%+v", field.Key, field, schemaField)
		}
		delete(schemaFields, field.Key)
	}
	if snapshot.EvidenceStatus == "Unverified" && len(schemaFields) != 0 {
		keys := make([]string, 0, len(schemaFields))
		for key := range schemaFields {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		t.Fatalf("schema fields missing from unverified snapshot: %v", keys)
	}
}

func TestSchemaUsesSharedPointbuyBounds(t *testing.T) {
	content, err := os.ReadFile(filepath.Join("..", "schemas", "mbch_schema.json"))
	if err != nil {
		t.Fatal(err)
	}
	var root struct {
		Definitions struct {
			ClassInfo struct {
				Properties map[string]struct {
					Minimum *int `json:"minimum"`
					Maximum *int `json:"maximum"`
				} `json:"properties"`
				PatternProperties map[string]json.RawMessage `json:"patternProperties"`
			} `json:"ClassInfo"`
		} `json:"definitions"`
	}
	if err := json.Unmarshal(content, &root); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"hasCustomSpec", "defaultSpec"} {
		property := root.Definitions.ClassInfo.Properties[key]
		if property.Minimum == nil || *property.Minimum != 0 || property.Maximum == nil || *property.Maximum != parsers.PointbuyMaxArchetypes {
			t.Errorf("%s bounds = %v..%v, want 0..%d", key, property.Minimum, property.Maximum, parsers.PointbuyMaxArchetypes)
		}
	}
	pattern := fmt.Sprintf(`^c_att_(skill|names|ranks|descs)_([0-9]|[1-3][0-9]|4[0-%d])$`, parsers.PointbuyMaxTotalSlots-41)
	if _, ok := root.Definitions.ClassInfo.PatternProperties[pattern]; !ok {
		t.Errorf("schema is missing exact custom-slot pattern %q", pattern)
	}
}

func TestExplicitEngineVerification(t *testing.T) {
	explicit := os.Getenv("MBII_VERIFY_ENGINE") == "1"
	engineRoot := strings.TrimSpace(os.Getenv("MBII_ENGINE_SRC"))
	if !explicit && engineRoot == "" {
		return
	}

	cmd := exec.Command("go", "run", filepath.Join("..", "tools", "generate_snapshot.go"),
		"-verify",
		"-snapshot", filepath.Join("testdata", "engine_snapshot.json"),
		"-schema", filepath.Join("..", "schemas", "mbch_schema.json"),
	)
	cmd.Env = os.Environ()
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("explicit engine verification failed: %v\n%s", err, output)
	}
	t.Log(strings.TrimSpace(string(output)))
}

func TestSnapshotGeneratorParsesEnumsWithoutCopyingSource(t *testing.T) {
	engineRoot := t.TempDir()
	gameDir := filepath.Join(engineRoot, "game")
	if err := os.MkdirAll(gameDir, 0o755); err != nil {
		t.Fatal(err)
	}
	header := `/* DO_NOT_COPY_THIS_COMMENT */
typedef enum {
	MB_CLASS_NOCLASS = 0,
	MB_CLASS_IMPLICIT,
	MB_CLASS_HEX = 0x10U,
	MB_CLASS_REFERENCE = MB_CLASS_IMPLICIT + 4,
} classes_t;

typedef enum attribute_tag {
	MB_ATT_INVALID = 0,
	MB_ATT_IMPLICIT,
	MB_ATT_SHIFTED = (1 << 4),
	MB_ATT_AFTER_SHIFT,
} MBAttribute_t;
`
	saga := `// SGPV(cI, "commentedOut", value);
void parse_fixture(void) {
	SGPV(cI,
		"maxhealth", value);
	BG_SiegeGetPairedValue(cI, "name", value);
}
`
	if err := os.WriteFile(filepath.Join(gameDir, "bg_public.h"), []byte(header), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(gameDir, "bg_saga.c"), []byte(saga), 0o644); err != nil {
		t.Fatal(err)
	}

	runGit := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = engineRoot
		if output, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, output)
		}
	}
	runGit("init", "-q")
	runGit("add", ".")
	runGit("-c", "user.name=Fixture", "-c", "user.email=fixture@example.invalid", "-c", "commit.gpgsign=false", "commit", "-qm", "fixture")

	outputPath := filepath.Join(t.TempDir(), "engine_snapshot.json")
	t.Setenv("MBII_ENGINE_SRC", engineRoot)
	cmd := exec.Command("go", "run", filepath.Join("..", "tools", "generate_snapshot.go"),
		"-snapshot", outputPath,
		"-schema", filepath.Join("..", "schemas", "mbch_schema.json"),
	)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("generate synthetic snapshot: %v\n%s", err, output)
	}
	content, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(content, []byte("DO_NOT_COPY_THIS_COMMENT")) || bytes.Contains(content, []byte("typedef enum")) {
		t.Fatal("derived metadata copied C source text")
	}
	var snapshot engineSnapshot
	if err := json.Unmarshal(content, &snapshot); err != nil {
		t.Fatal(err)
	}
	if snapshot.EvidenceStatus != "Verified" || snapshot.Revision == "" {
		t.Fatalf("synthetic checkout provenance was not recorded: %+v", snapshot)
	}
	expected := map[string][]snapshotEnumValue{
		"classes_t": {
			{Name: "MB_CLASS_NOCLASS", Value: 0},
			{Name: "MB_CLASS_IMPLICIT", Value: 1},
			{Name: "MB_CLASS_HEX", Value: 16},
			{Name: "MB_CLASS_REFERENCE", Value: 5},
		},
		"MBAttribute_t": {
			{Name: "MB_ATT_INVALID", Value: 0},
			{Name: "MB_ATT_IMPLICIT", Value: 1},
			{Name: "MB_ATT_SHIFTED", Value: 16},
			{Name: "MB_ATT_AFTER_SHIFT", Value: 17},
		},
	}
	for _, enum := range snapshot.Enums {
		if !reflect.DeepEqual(enum.Values, expected[enum.Name]) {
			t.Errorf("%s values/order = %+v, want %+v", enum.Name, enum.Values, expected[enum.Name])
		}
		delete(expected, enum.Name)
	}
	if len(expected) != 0 {
		t.Fatalf("generator omitted enum declarations: %v", expected)
	}

	if err := os.WriteFile(filepath.Join(gameDir, "bg_public.h"), []byte(header+"\n/* changed after HEAD without changing derived metadata */\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	dirtyOutputPath := filepath.Join(t.TempDir(), "dirty_engine_snapshot.json")
	for _, test := range []struct {
		name string
		args []string
	}{
		{
			name: "generation",
			args: []string{
				"-snapshot", dirtyOutputPath,
				"-schema", filepath.Join("..", "schemas", "mbch_schema.json"),
			},
		},
		{
			name: "verification",
			args: []string{
				"-verify",
				"-snapshot", outputPath,
				"-schema", filepath.Join("..", "schemas", "mbch_schema.json"),
			},
		},
	} {
		t.Run("rejects modified tracked source during "+test.name, func(t *testing.T) {
			cmd := exec.Command("go", append([]string{"run", filepath.Join("..", "tools", "generate_snapshot.go")}, test.args...)...)
			output, err := cmd.CombinedOutput()
			if err == nil {
				t.Fatalf("%s succeeded against source modified after HEAD:\n%s", test.name, output)
			}
			if !bytes.Contains(output, []byte("does not match revision")) {
				t.Fatalf("%s failed without provenance mismatch diagnostic: %v\n%s", test.name, err, output)
			}
			if bytes.Contains(output, []byte("generated ")) || bytes.Contains(output, []byte("verified ")) {
				t.Fatalf("%s claimed HEAD provenance for modified source:\n%s", test.name, output)
			}
		})
	}
	if _, err := os.Stat(dirtyOutputPath); !os.IsNotExist(err) {
		t.Fatalf("failed generation wrote dirty snapshot %s: %v", dirtyOutputPath, err)
	}
}

func assertSortedUniqueFields(t *testing.T, fields []snapshotField) {
	t.Helper()
	last := ""
	for i, field := range fields {
		if field.Key == "" || field.Type == nil || field.EvidenceStatus == "" {
			t.Fatalf("field %d has incomplete metadata: %+v", i, field)
		}
		if i > 0 && field.Key <= last {
			t.Fatalf("fields are not strictly sorted and unique: %q after %q", field.Key, last)
		}
		last = field.Key
	}
}

func assertUnverifiedSnapshot(t *testing.T, snapshot engineSnapshot) {
	t.Helper()
	if snapshot.UnavailableReason == "" {
		t.Fatal("Unverified snapshot must explain the unavailable evidence")
	}
	if snapshot.Revision != "" || len(snapshot.Sources) != 0 || len(snapshot.SourceRanges) != 0 || len(snapshot.Enums) != 0 {
		t.Fatal("Unverified snapshot must not fabricate revision, hashes, ranges, or enum values")
	}
	for _, field := range snapshot.Fields {
		if field.EvidenceStatus != "Unverified" {
			t.Fatalf("unverified snapshot field %q claims %q evidence", field.Key, field.EvidenceStatus)
		}
	}
}

func assertVerifiedSnapshot(t *testing.T, snapshot engineSnapshot) {
	t.Helper()
	if !regexp.MustCompile(`^[0-9a-f]{40,64}$`).MatchString(snapshot.Revision) {
		t.Fatalf("invalid verified revision %q", snapshot.Revision)
	}
	if len(snapshot.Sources) != 2 {
		t.Fatalf("verified snapshot has %d source hashes, want 2", len(snapshot.Sources))
	}
	sourcePaths := make(map[string]bool)
	for _, source := range snapshot.Sources {
		if filepath.IsAbs(source.Path) || !regexp.MustCompile(`^[0-9a-f]{64}$`).MatchString(source.SHA256) {
			t.Fatalf("invalid source metadata: %+v", source)
		}
		sourcePaths[source.Path] = true
	}
	for _, sourceRange := range snapshot.SourceRanges {
		if !sourcePaths[sourceRange.Path] || sourceRange.Declaration == "" || sourceRange.StartLine < 1 || sourceRange.EndLine < sourceRange.StartLine {
			t.Fatalf("invalid source range: %+v", sourceRange)
		}
	}
	if len(snapshot.Enums) != 2 {
		t.Fatalf("verified snapshot has %d enums, want classes_t and MBAttribute_t", len(snapshot.Enums))
	}
	for _, enum := range snapshot.Enums {
		if enum.Name != "classes_t" && enum.Name != "MBAttribute_t" {
			t.Fatalf("unexpected verified enum %q", enum.Name)
		}
		if len(enum.Values) == 0 {
			t.Fatalf("verified enum %q is empty", enum.Name)
		}
		seen := make(map[string]bool)
		for _, value := range enum.Values {
			if value.Name == "" || seen[value.Name] {
				t.Fatalf("duplicate/empty member in %q: %q", enum.Name, value.Name)
			}
			seen[value.Name] = true
		}
	}
	assertVerifiedEnumCatalogParity(t, snapshot)
}

func assertVerifiedEnumCatalogParity(t *testing.T, snapshot engineSnapshot) {
	t.Helper()
	engine := make(map[string]map[string]int64)
	engineOrder := make(map[string][]string)
	for _, enum := range snapshot.Enums {
		values := make(map[string]int64, len(enum.Values))
		names := make([]string, 0, len(enum.Values))
		for _, value := range enum.Values {
			values[value.Name] = value.Value
			names = append(names, value.Name)
		}
		engine[enum.Name] = values
		engineOrder[enum.Name] = names
	}

	type catalogEngine struct {
		Revision     string `json:"revision"`
		SourcePath   string `json:"source_path"`
		SourceSHA256 string `json:"source_sha256"`
	}
	assertCatalogSource := func(name string, metadata catalogEngine) {
		t.Helper()
		if metadata.Revision != snapshot.Revision {
			t.Errorf("%s revision=%q, snapshot=%q", name, metadata.Revision, snapshot.Revision)
		}
		for _, source := range snapshot.Sources {
			if source.Path == metadata.SourcePath {
				if source.SHA256 != metadata.SourceSHA256 {
					t.Errorf("%s source hash=%q, snapshot=%q", name, metadata.SourceSHA256, source.SHA256)
				}
				return
			}
		}
		t.Errorf("%s source path %q is absent from snapshot", name, metadata.SourcePath)
	}

	classContent, err := os.ReadFile(filepath.Join("..", "schemas", "enums", "classes.json"))
	if err != nil {
		t.Fatal(err)
	}
	var classCatalog struct {
		EvidenceStatus string        `json:"evidence_status"`
		Engine         catalogEngine `json:"engine"`
		Classes        map[string]struct {
			Value int64 `json:"value"`
		} `json:"classes"`
	}
	if err := json.Unmarshal(classContent, &classCatalog); err != nil {
		t.Fatal(err)
	}
	if classCatalog.EvidenceStatus != "EnumValuesVerified" {
		t.Errorf("class catalog evidence=%q, want EnumValuesVerified", classCatalog.EvidenceStatus)
	}
	assertCatalogSource("class catalog", classCatalog.Engine)
	classes := engine["classes_t"]
	if len(classCatalog.Classes) != len(classes) {
		t.Errorf("class catalog has %d values, engine has %d", len(classCatalog.Classes), len(classes))
	}
	for name, entry := range classCatalog.Classes {
		value, ok := classes[name]
		if !ok {
			t.Errorf("class catalog entry %q is absent from verified engine enum", name)
		} else if value != entry.Value {
			t.Errorf("class catalog numeric drift for %q: catalog=%d engine=%d", name, entry.Value, value)
		}
	}
	for name := range classes {
		if _, ok := classCatalog.Classes[name]; !ok {
			t.Errorf("verified engine class %q is absent from class catalog", name)
		}
	}

	attributeContent, err := os.ReadFile(filepath.Join("..", "schemas", "enums", "attributes.json"))
	if err != nil {
		t.Fatal(err)
	}
	var attributeCatalog struct {
		EvidenceStatus  string           `json:"evidence_status"`
		Engine          catalogEngine    `json:"engine"`
		EnumValues      map[string]int64 `json:"enumValues"`
		AllAttributeIDs []string         `json:"allAttributeIds"`
	}
	if err := json.Unmarshal(attributeContent, &attributeCatalog); err != nil {
		t.Fatal(err)
	}
	if attributeCatalog.EvidenceStatus != "EnumValuesVerified" {
		t.Errorf("attribute catalog evidence=%q, want EnumValuesVerified", attributeCatalog.EvidenceStatus)
	}
	assertCatalogSource("attribute catalog", attributeCatalog.Engine)
	attributes := engine["MBAttribute_t"]
	if !reflect.DeepEqual(attributeCatalog.EnumValues, attributes) {
		t.Error("attribute catalog names/numeric values do not exactly match verified MBAttribute_t")
	}
	selectable := make([]string, 0, len(engineOrder["MBAttribute_t"])-2)
	for _, name := range engineOrder["MBAttribute_t"] {
		if name != "MB_ATT_INVALID" && name != "NO_OF_MB_ATTRIBUTES" {
			selectable = append(selectable, name)
		}
	}
	if !reflect.DeepEqual(attributeCatalog.AllAttributeIDs, selectable) {
		t.Error("attribute catalog selectable identifiers do not exactly match verified MBAttribute_t order")
	}
}

func readEngineSnapshot(t *testing.T) engineSnapshot {
	t.Helper()
	content, err := os.ReadFile(filepath.Join("testdata", "engine_snapshot.json"))
	if err != nil {
		t.Fatal(err)
	}
	var snapshot engineSnapshot
	if err := json.Unmarshal(content, &snapshot); err != nil {
		t.Fatal(err)
	}
	return snapshot
}

func readSchemaFields(t *testing.T) map[string]snapshotField {
	t.Helper()
	content, err := os.ReadFile(filepath.Join("..", "schemas", "mbch_schema.json"))
	if err != nil {
		t.Fatal(err)
	}
	var root struct {
		Definitions map[string]struct {
			Properties map[string]struct {
				Type    any `json:"type"`
				Default any `json:"default"`
				Minimum any `json:"minimum"`
				Maximum any `json:"maximum"`
			} `json:"properties"`
		} `json:"definitions"`
	}
	if err := json.Unmarshal(content, &root); err != nil {
		t.Fatal(err)
	}
	result := make(map[string]snapshotField)
	definitionNames := make([]string, 0, len(root.Definitions))
	for name := range root.Definitions {
		definitionNames = append(definitionNames, name)
	}
	sort.Strings(definitionNames)
	for _, definitionName := range definitionNames {
		definition := root.Definitions[definitionName]
		propertyNames := make([]string, 0, len(definition.Properties))
		for key := range definition.Properties {
			propertyNames = append(propertyNames, key)
		}
		sort.Strings(propertyNames)
		for _, key := range propertyNames {
			if strings.HasPrefix(key, "_section_") {
				continue
			}
			if _, exists := result[key]; exists {
				continue
			}
			property := definition.Properties[key]
			var bounds *snapshotBounds
			if property.Minimum != nil || property.Maximum != nil {
				bounds = &snapshotBounds{Minimum: property.Minimum, Maximum: property.Maximum}
			}
			result[key] = snapshotField{
				Key:            key,
				Type:           property.Type,
				Default:        property.Default,
				Bounds:         bounds,
				EvidenceStatus: "Unverified",
			}
		}
	}
	return result
}
