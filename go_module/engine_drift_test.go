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

type snapshotConfiguration struct {
	DefinesPath   string `json:"defines_path"`
	DefinesSHA256 string `json:"defines_sha256"`
	Preprocessor  string `json:"preprocessor"`
	Scope         string `json:"scope"`
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
	FormatVersion     int                   `json:"format_version"`
	EvidenceStatus    string                `json:"evidence_status"`
	UnavailableReason string                `json:"unavailable_reason,omitempty"`
	Revision          string                `json:"revision"`
	Sources           []snapshotSource      `json:"sources"`
	Configuration     snapshotConfiguration `json:"configuration"`
	FieldsScope       string                `json:"fields_scope"`
	SourceRanges      []snapshotRange       `json:"source_ranges"`
	Enums             []snapshotEnum        `json:"enums"`
	Fields            []snapshotField       `json:"fields"`
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
	if snapshot.FormatVersion != 2 {
		t.Fatalf("snapshot format_version = %d, want 2", snapshot.FormatVersion)
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
			if field.Type != "unknown" || field.EvidenceStatus != "SourceKeyVerified" {
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

func TestSchemaClassAndAbilityAcceptancePreservesRawIdentifiers(t *testing.T) {
	schemaBytes, err := os.ReadFile(filepath.Join("..", "schemas", "mbch_schema.json"))
	if err != nil {
		t.Fatal(err)
	}
	var schema struct {
		Definitions struct {
			ClassInfo struct {
				Properties struct {
					MBClass struct {
						Type          string                `json:"type"`
						Enum          []string              `json:"enum"`
						Revision      string                `json:"engineRevision"`
						Configuration snapshotConfiguration `json:"sourceConfiguration"`
					} `json:"MBClass"`
				} `json:"properties"`
			} `json:"ClassInfo"`
		} `json:"definitions"`
	}
	if err := json.Unmarshal(schemaBytes, &schema); err != nil {
		t.Fatal(err)
	}
	constraint := schema.Definitions.ClassInfo.Properties.MBClass
	if constraint.Type != "string" || len(constraint.Enum) == 0 {
		t.Fatal("MBClass must have a nonempty string enum constraint")
	}
	snapshot := readEngineSnapshot(t)
	if constraint.Revision != snapshot.Revision || constraint.Configuration != snapshot.Configuration {
		t.Fatal("MBClass schema provenance does not match the snapshot source configuration")
	}
	var expectedClasses []string
	for _, enum := range snapshot.Enums {
		if enum.Name != "classes_t" {
			continue
		}
		for _, value := range enum.Values {
			if value.Name != "MB_CLASS_NOCLASS" && value.Name != "NO_OF_CLASSES" {
				expectedClasses = append(expectedClasses, value.Name)
			}
		}
	}
	if !reflect.DeepEqual(constraint.Enum, expectedClasses) {
		t.Errorf("MBClass validation constraint differs from active source-default classes: %v", constraint.Enum)
	}
	catalogBytes, err := os.ReadFile(filepath.Join("..", "schemas", "enums", "attributes.json"))
	if err != nil {
		t.Fatal(err)
	}
	var attributes struct {
		IDs []string `json:"allAttributeIds"`
	}
	if err := json.Unmarshal(catalogBytes, &attributes); err != nil {
		t.Fatal(err)
	}
	contains := func(ids []string, id string) bool {
		for _, candidate := range ids {
			if candidate == id {
				return true
			}
		}
		return false
	}
	for _, test := range []struct {
		class     string
		attribute string
		supported bool
	}{
		{"MB_CLASS_SHOCKTROOPER", "MB_ATT_CYBERNETICS", true},
		{"MB_CLASS_OPERATIVE", "MB_ATT_TEMP_CLOAK", true},
		{"MB_CLASS_ENFORCER", "MB_ATT_TEMP_PSHIELD", true},
		{"MB_CLASS_SPY", "MB_ATT_TEMP_GRAP_HOOKS", true},
		{"MB_CLASS_ASTRODROID", "MB_ATT_WELDER", true},
		{"MB_CLASS_ASTROMECH", "MB_ATT_MIRV", true},
		{"MB_CLASS_BATTLEDROID", "MB_ATT_ROCK_NADE", true},
		{"MB_CLASS_JETTROOPER", "MB_ATT_CYCLER", true},
		{"MB_CLASS_ASSASSINDROID", "MB_ATT_FA3_FLECHETTE", true},
		{"MB_CLASS_PATROLIAN", "MB_ATT_FP_SUMMON", true},
		{"MB_CLASS_REBMEDIC", "MB_ATT_FP_PROJECTION", false},
		{"MB_CLASS_FUTURE_UNKNOWN", "MB_ATT_FA3_FLECHETE", false},
	} {
		t.Run(test.class+"/"+test.attribute, func(t *testing.T) {
			if got := contains(constraint.Enum, test.class); got != test.supported {
				t.Errorf("class schema accepts %s = %v, want %v", test.class, got, test.supported)
			}
			if got := contains(attributes.IDs, test.attribute); got != test.supported {
				t.Errorf("attribute catalog accepts %s = %v, want %v", test.attribute, got, test.supported)
			}
			// A catalog constraint must not make the lossless raw parser discard
			// retired, misspelled, or future identifiers.
			rawAttributes := test.attribute + ",2|MB_ATT_FUTURE_UNKNOWN,0"
			source := fmt.Sprintf("ClassInfo {\nname \"Schema acceptance\"\nMBClass %s\nattributes \"%s\"\n}\n", test.class, rawAttributes)
			character, err := parsers.ParseMBCH(source)
			if err != nil {
				t.Fatal(err)
			}
			generated, err := parsers.GenerateMBCH(character)
			if err != nil {
				t.Fatal(err)
			}
			reparsed, err := parsers.ParseMBCH(generated)
			if err != nil {
				t.Fatal(err)
			}
			if reparsed.MBClass != test.class || reparsed.Attributes != rawAttributes {
				t.Fatalf("schema support changed raw round-trip identifiers: class=%q attributes=%q", reparsed.MBClass, reparsed.Attributes)
			}
		})
	}
}

func TestSchemaWeaponAbilityAndFlagDomains(t *testing.T) {
	readValues := func(filename, key string) map[string]int64 {
		t.Helper()
		content, err := os.ReadFile(filepath.Join("..", "schemas", "enums", filename))
		if err != nil {
			t.Fatal(err)
		}
		var catalog map[string]json.RawMessage
		if err := json.Unmarshal(content, &catalog); err != nil {
			t.Fatal(err)
		}
		var entries map[string]struct {
			Value int64 `json:"value"`
		}
		if err := json.Unmarshal(catalog[key], &entries); err != nil {
			t.Fatal(err)
		}
		values := make(map[string]int64)
		for name, entry := range entries {
			values[name] = entry.Value
		}
		return values
	}
	for _, test := range []struct {
		file, key string
		expected  map[string]int64
		absent    []string
	}{
		{"weapons.json", "weapons", map[string]int64{
			"WP_HEAVY_PISTOL": 11, "WP_DLT19": 12, "WP_TRAD_BOWCASTER": 13,
			"WP_PROTON_NADE": 45, "WP_GLOP_NADE": 46, "WP_WELD_PULSE": 49, "WP_WELD_BEAM": 50,
			"WP_WELDER": 59, "WP_MIRV": 60, "WP_CYCLER": 61, "WP_FA3_FLECHETTE": 62, "WP_NUM_WEAPONS": 65,
		}, []string{"WP_IMPLODER_NADE", "WP_SHIELD_NADE"}},
		{"force_powers.json", "force_powers", map[string]int64{
			"FP_SUMMON": 20, "FP_BATTLEMED": 23, "FP_DOMINATION": 24, "FP_DARKRAGE": 25,
		}, []string{"FP_PROJECTION", "FP_REPULSE", "EAS_FP_REPULSE"}},
		{"holdables.json", "holdables", map[string]int64{
			"HI_SPAWNER": 10, "HI_STIMPACK": 11, "HI_PSHIELD": 12, "HI_BASESEEKER": 13, "HI_SHOCKFIELD": 14, "HI_NUM_HOLDABLE": 15,
		}, []string{"EAS_HI_OVERCLOCK", "EAS_HI_TEMP_CLOAK"}},
		{"powerups.json", "powerups", map[string]int64{"PW_PSHIELD": 22, "PW_NUM_POWERUPS": 23}, nil},
		{"class_flags.json", "weapon_held_flags", map[string]int64{
			"HELD_FORCEFOCUS": 31, "HELD_LIFESTEAL": 32, "HELD_FLASH": 33, "HELD_BACTA": 34, "HELD_NODISARM": 35,
		}, nil},
	} {
		t.Run(test.file+"/"+test.key, func(t *testing.T) {
			values := readValues(test.file, test.key)
			for name, want := range test.expected {
				if got, ok := values[name]; !ok || got != want {
					t.Errorf("source-default enum %s = %d (present %v), want %d", name, got, ok, want)
				}
			}
			for _, name := range test.absent {
				if _, ok := values[name]; ok {
					t.Errorf("inactive/retired/wrong-domain identifier %s is present", name)
				}
			}
		})
	}
	actions := readValues("holdables.json", "extended_abilities")
	for _, name := range []string{"EAS_FP_REPULSE", "EAS_HI_OVERCLOCK", "EAS_HI_TEMP_CLOAK", "EAS_HI_TEMP_PSHIELD", "EAS_HI_TEMP_GRAPPLEHOOK"} {
		if _, ok := actions[name]; !ok {
			t.Errorf("special action %s is absent from its own domain", name)
		}
	}
	if _, ok := actions["FP_REPULSE"]; ok {
		t.Error("special-action schema invented a grantable force-power identifier")
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

func TestSnapshotGeneratorParsesSourceDefaultsWithoutCopyingSource(t *testing.T) {
	for _, enabled := range []bool{true, false} {
		t.Run(fmt.Sprintf("feature_enabled=%v", enabled), func(t *testing.T) {
			engineRoot := t.TempDir()
			gameDir := filepath.Join(engineRoot, "game")
			if err := os.MkdirAll(gameDir, 0o755); err != nil {
				t.Fatal(err)
			}
			header := `/* DO_NOT_COPY_THIS_COMMENT */
typedef enum {
	MB_CLASS_NOCLASS = 0,
	MB_CLASS_IMPLICIT,
#ifdef FIXTURE_ENABLED
	MB_CLASS_ENABLED,
#else
	MB_CLASS_DISABLED_BRANCH = 4,
#endif
#ifdef FIXTURE_NEVER_DEFINED
	MB_CLASS_INACTIVE,
#endif
	MB_CLASS_AFTER_CONDITIONAL,
	MB_CLASS_HEX = 0x10U,
	MB_CLASS_REFERENCE = MB_CLASS_IMPLICIT + 4,
} classes_t;

typedef enum attribute_tag {
	MB_ATT_INVALID = 0,
	MB_ATT_IMPLICIT,
#if defined(FIXTURE_ENABLED) && FIXTURE_LEVEL == 2
	MB_ATT_SHIFTED = (1 << 4),
#endif
	MB_ATT_AFTER_CONDITIONAL,
} MBAttribute_t;
enum { HI_NONE, HI_ITEM, HI_NUM_HOLDABLE };
typedef int holdable_t;
enum { PW_NONE, PW_ITEM, PW_NUM_POWERUPS };
typedef int powerup_t;
typedef enum { EAS_NONE, EAS_SAMPLE, EAS_MAX } gEntityActionSkill_t;
`
			defines := "#define FIXTURE_LEVEL 2\n"
			if enabled {
				defines += "#define FIXTURE_ENABLED\n"
			}
			saga := `// SGPV(cI, "commentedOut", value);
void parse_fixture(void) {
	SGPV(cI, "maxhealth", value);
	BG_SiegeGetPairedValue(cI, "name", value);
}
`
			sources := map[string]string{
				"bg_public.h": header, "bg_saga.c": saga, "mb_defines.h": defines, "unrelated.c": "int unchanged;\n",
				"bg_weapons.h": "typedef enum { WP_NONE, WP_SAMPLE, WP_NUM_WEAPONS } weapon_t;",
				"q_shared.h": "typedef enum { FP_INVALID = -1, FP_FIRST = 0, FP_HEAL = 0, NUM_FORCE_POWERS } forcePowers_t;\n" +
					"typedef enum { SS_NONE, SS_NUM_SABER_STYLES } saber_styles_t;",
				"bg_saga.h": "typedef enum { CFL_SAMPLE } siegeClassFlags_t;\n" +
					"typedef enum { HELD_SAMPLE, HELD_UPPER = 35 } siegeWeaponFlags_t;",
			}
			for name, content := range sources {
				if err := os.WriteFile(filepath.Join(gameDir, name), []byte(content), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			for _, args := range [][]string{
				{"init", "-q"},
				{"add", "."},
				{"-c", "user.name=Fixture", "-c", "user.email=fixture@example.invalid", "-c", "commit.gpgsign=false", "commit", "-qm", "fixture"},
			} {
				cmd := exec.Command("git", args...)
				cmd.Dir = engineRoot
				if output, err := cmd.CombinedOutput(); err != nil {
					t.Fatalf("git %v: %v\n%s", args, err, output)
				}
			}
			outputDir := t.TempDir()
			if err := os.MkdirAll(filepath.Join(outputDir, "enums"), 0o755); err != nil {
				t.Fatal(err)
			}
			schemaPath := filepath.Join(outputDir, "mbch_schema.json")
			schema := `{"definitions":{"ClassInfo":{"properties":{"MBClass":{"type":"string","enum":["STALE"]},"maxhealth":{"type":"integer"}}}}}`
			if err := os.WriteFile(schemaPath, []byte(schema), 0o644); err != nil {
				t.Fatal(err)
			}
			outputPath := filepath.Join(outputDir, "engine_snapshot.json")
			t.Setenv("MBII_ENGINE_SRC", engineRoot)
			generate := func(args ...string) ([]byte, error) {
				t.Helper()
				cmd := exec.Command("go", append([]string{"run", filepath.Join("..", "tools", "generate_snapshot.go")}, args...)...)
				return cmd.CombinedOutput()
			}
			output, err := generate("-update-schemas", "-snapshot", outputPath, "-schema", schemaPath)
			if err != nil {
				t.Fatalf("generate synthetic snapshot/catalogs: %v\n%s", err, output)
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
			if snapshot.EvidenceStatus != "Verified" || snapshot.Revision == "" || snapshot.Configuration.DefinesPath != "game/mb_defines.h" {
				t.Fatalf("synthetic checkout/configuration provenance was not recorded: %+v", snapshot)
			}
			classes := []snapshotEnumValue{{"MB_CLASS_NOCLASS", 0}, {"MB_CLASS_IMPLICIT", 1}}
			attributes := []snapshotEnumValue{{"MB_ATT_INVALID", 0}, {"MB_ATT_IMPLICIT", 1}}
			classAfter, attributeAfter := int64(5), int64(2)
			if enabled {
				classes = append(classes, snapshotEnumValue{"MB_CLASS_ENABLED", 2})
				attributes = append(attributes, snapshotEnumValue{"MB_ATT_SHIFTED", 16})
				classAfter, attributeAfter = 3, 17
			} else {
				classes = append(classes, snapshotEnumValue{"MB_CLASS_DISABLED_BRANCH", 4})
			}
			classes = append(classes, snapshotEnumValue{"MB_CLASS_AFTER_CONDITIONAL", classAfter}, snapshotEnumValue{"MB_CLASS_HEX", 16}, snapshotEnumValue{"MB_CLASS_REFERENCE", 5})
			attributes = append(attributes, snapshotEnumValue{"MB_ATT_AFTER_CONDITIONAL", attributeAfter})
			expected := map[string][]snapshotEnumValue{"classes_t": classes, "MBAttribute_t": attributes}
			for _, enum := range snapshot.Enums {
				if enum.Name != "classes_t" && enum.Name != "MBAttribute_t" {
					continue
				}
				if !reflect.DeepEqual(enum.Values, expected[enum.Name]) {
					t.Errorf("%s values/order = %+v, want %+v", enum.Name, enum.Values, expected[enum.Name])
				}
				delete(expected, enum.Name)
			}
			if len(expected) != 0 {
				t.Fatalf("generator omitted enum declarations: %v", expected)
			}
			assertGeneratedSchemaDefaults(t, outputDir, enabled, classAfter, attributeAfter)
			output, err = generate("-verify", "-snapshot", outputPath, "-schema", schemaPath)
			if err != nil {
				t.Fatalf("verify generated source-default snapshot: %v\n%s", err, output)
			}
			if !enabled {
				return
			}
			for _, source := range []string{"bg_public.h", "mb_defines.h", "unrelated.c"} {
				t.Run("rejects modified "+source, func(t *testing.T) {
					path := filepath.Join(gameDir, source)
					if err := os.WriteFile(path, []byte(sources[source]+"\n/* dirty metadata source */\n"), 0o644); err != nil {
						t.Fatal(err)
					}
					failedPath := filepath.Join(outputDir, "dirty_engine_snapshot.json")
					for _, args := range [][]string{
						{"-update-schemas", "-snapshot", failedPath, "-schema", schemaPath},
						{"-verify", "-snapshot", outputPath, "-schema", schemaPath},
					} {
						output, err := generate(args...)
						if err == nil || !bytes.Contains(output, []byte("does not match revision")) {
							t.Fatalf("dirty source did not fail closed: %v\n%s", err, output)
						}
					}
					if _, err := os.Stat(failedPath); !os.IsNotExist(err) {
						t.Fatalf("failed generation wrote snapshot: %v", err)
					}
					if err := os.WriteFile(path, []byte(sources[source]), 0o644); err != nil {
						t.Fatal(err)
					}
				})
			}
			if err := os.Remove(filepath.Join(gameDir, "mb_defines.h")); err != nil {
				t.Fatal(err)
			}
			output, err = generate("-snapshot", filepath.Join(outputDir, "missing.json"), "-schema", schemaPath)
			if err == nil || (!bytes.Contains(output, []byte("mb_defines.h not found")) && !bytes.Contains(output, []byte("does not match revision"))) {
				t.Fatalf("missing feature configuration did not fail closed: %v\n%s", err, output)
			}
		})
	}
}

func assertGeneratedSchemaDefaults(t *testing.T, directory string, enabled bool, classAfter, attributeAfter int64) {
	t.Helper()
	classBytes, err := os.ReadFile(filepath.Join(directory, "enums", "classes.json"))
	if err != nil {
		t.Fatal(err)
	}
	var classes struct {
		Classes map[string]struct {
			Value int64 `json:"value"`
		} `json:"classes"`
	}
	if err := json.Unmarshal(classBytes, &classes); err != nil {
		t.Fatal(err)
	}
	if got := classes.Classes["MB_CLASS_AFTER_CONDITIONAL"].Value; got != classAfter {
		t.Errorf("consumer class value after gate = %d, want %d", got, classAfter)
	}
	if _, ok := classes.Classes["MB_CLASS_ENABLED"]; ok != enabled {
		t.Errorf("consumer class catalog enabled membership = %v, want %v", ok, enabled)
	}
	if _, ok := classes.Classes["MB_CLASS_INACTIVE"]; ok {
		t.Error("consumer class catalog contains inactive identifier")
	}
	attributeBytes, err := os.ReadFile(filepath.Join(directory, "enums", "attributes.json"))
	if err != nil {
		t.Fatal(err)
	}
	var attributes struct {
		Values map[string]int64 `json:"enumValues"`
		IDs    []string         `json:"allAttributeIds"`
	}
	if err := json.Unmarshal(attributeBytes, &attributes); err != nil {
		t.Fatal(err)
	}
	if got := attributes.Values["MB_ATT_AFTER_CONDITIONAL"]; got != attributeAfter {
		t.Errorf("consumer attribute value after gate = %d, want %d", got, attributeAfter)
	}
	if _, ok := attributes.Values["MB_ATT_SHIFTED"]; ok != enabled {
		t.Errorf("consumer attribute catalog enabled membership = %v, want %v", ok, enabled)
	}
	schema, err := os.ReadFile(filepath.Join(directory, "mbch_schema.json"))
	if err != nil {
		t.Fatal(err)
	}
	var root struct {
		Definitions struct {
			ClassInfo struct {
				Properties struct {
					MBClass struct {
						Enum []string `json:"enum"`
					} `json:"MBClass"`
				} `json:"properties"`
			} `json:"ClassInfo"`
		} `json:"definitions"`
	}
	if err := json.Unmarshal(schema, &root); err != nil {
		t.Fatal(err)
	}
	seen := make(map[string]bool)
	for _, name := range root.Definitions.ClassInfo.Properties.MBClass.Enum {
		seen[name] = true
	}
	if seen["MB_CLASS_ENABLED"] != enabled || seen["MB_CLASS_DISABLED_BRANCH"] == enabled || seen["MB_CLASS_INACTIVE"] || seen["MB_CLASS_NOCLASS"] || seen["STALE"] {
		t.Fatalf("generated MBClass validation constraint does not respect gates/sentinels: %v", seen)
	}
	for _, name := range attributes.IDs {
		if name == "MB_ATT_INVALID" || (name == "MB_ATT_SHIFTED" && !enabled) {
			t.Errorf("generated selectable attributes contain %s", name)
		}
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
	if len(snapshot.Sources) != 6 {
		t.Fatalf("verified snapshot has %d source hashes, want six enum/parser/feature source files", len(snapshot.Sources))
	}
	sourcePaths := make(map[string]bool)
	for _, source := range snapshot.Sources {
		if filepath.IsAbs(source.Path) || !regexp.MustCompile(`^[0-9a-f]{64}$`).MatchString(source.SHA256) {
			t.Fatalf("invalid source metadata: %+v", source)
		}
		sourcePaths[source.Path] = true
	}
	configuration := snapshot.Configuration
	if !sourcePaths[configuration.DefinesPath] || configuration.Preprocessor != "cc -std=c99 -E -P -undef -nostdinc -x c -" ||
		!strings.Contains(configuration.Scope, "source defaults only") || !strings.Contains(snapshot.FieldsScope, "before preprocessing") {
		t.Fatalf("missing source-default/field-evidence scope: %+v / %q", configuration, snapshot.FieldsScope)
	}
	for _, source := range snapshot.Sources {
		if source.Path == configuration.DefinesPath && source.SHA256 != configuration.DefinesSHA256 {
			t.Fatal("feature configuration hash differs from verified source hash")
		}
	}
	for _, sourceRange := range snapshot.SourceRanges {
		if !sourcePaths[sourceRange.Path] || sourceRange.Declaration == "" || sourceRange.StartLine < 1 || sourceRange.EndLine < sourceRange.StartLine {
			t.Fatalf("invalid source range: %+v", sourceRange)
		}
	}
	expectedEnums := map[string]bool{
		"classes_t": true, "MBAttribute_t": true, "weapon_t": true, "forcePowers_t": true,
		"saber_styles_t": true, "holdable_t": true, "gEntityActionSkill_t": true, "powerup_t": true,
		"siegeClassFlags_t": true, "siegeWeaponFlags_t": true,
	}
	for _, enum := range snapshot.Enums {
		if !expectedEnums[enum.Name] {
			t.Fatalf("unexpected or duplicate verified enum %q", enum.Name)
		}
		delete(expectedEnums, enum.Name)
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
	if len(expectedEnums) != 0 {
		t.Fatalf("snapshot omitted required enum domains: %v", expectedEnums)
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
		Revision      string                `json:"revision"`
		SourcePath    string                `json:"source_path"`
		SourceSHA256  string                `json:"source_sha256"`
		Configuration snapshotConfiguration `json:"configuration"`
		SourceRanges  []snapshotRange       `json:"source_ranges"`
	}
	assertCatalogSource := func(name string, metadata catalogEngine) {
		t.Helper()
		if metadata.Revision != snapshot.Revision {
			t.Errorf("%s revision=%q, snapshot=%q", name, metadata.Revision, snapshot.Revision)
		}
		if metadata.Configuration != snapshot.Configuration {
			t.Errorf("%s source configuration does not match the snapshot", name)
		}
		if len(metadata.SourceRanges) == 0 {
			t.Errorf("%s has no declaration ranges", name)
		}
		for _, catalogRange := range metadata.SourceRanges {
			found := false
			for _, sourceRange := range snapshot.SourceRanges {
				if sourceRange == catalogRange {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("%s declaration range is absent from snapshot: %+v", name, catalogRange)
			}
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
	if classCatalog.EvidenceStatus != "SourceDefaultsEnumValuesVerified" {
		t.Errorf("class catalog evidence=%q, want SourceDefaultsEnumValuesVerified", classCatalog.EvidenceStatus)
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
	if attributeCatalog.EvidenceStatus != "SourceDefaultsEnumValuesVerified" {
		t.Errorf("attribute catalog evidence=%q, want SourceDefaultsEnumValuesVerified", attributeCatalog.EvidenceStatus)
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
	for _, spec := range []struct {
		file, key, declaration string
	}{
		{"weapons.json", "weapons", "weapon_t"},
		{"force_powers.json", "force_powers", "forcePowers_t"},
		{"saber_styles.json", "saber_styles", "saber_styles_t"},
		{"holdables.json", "holdables", "holdable_t"},
		{"holdables.json", "extended_abilities", "gEntityActionSkill_t"},
		{"powerups.json", "powerups", "powerup_t"},
		{"class_flags.json", "class_flags", "siegeClassFlags_t"},
		{"class_flags.json", "weapon_held_flags", "siegeWeaponFlags_t"},
	} {
		content, err := os.ReadFile(filepath.Join("..", "schemas", "enums", spec.file))
		if err != nil {
			t.Fatal(err)
		}
		var catalog map[string]json.RawMessage
		if err := json.Unmarshal(content, &catalog); err != nil {
			t.Fatal(err)
		}
		var metadata catalogEngine
		if err := json.Unmarshal(catalog["engine"], &metadata); err != nil {
			t.Fatal(err)
		}
		assertCatalogSource(spec.file, metadata)
		var entries map[string]struct {
			Value int64 `json:"value"`
		}
		if err := json.Unmarshal(catalog[spec.key], &entries); err != nil {
			t.Fatal(err)
		}
		values := make(map[string]int64)
		for name, entry := range entries {
			values[name] = entry.Value
		}
		if !reflect.DeepEqual(values, engine[spec.declaration]) {
			t.Errorf("%s/%s does not exactly match source-default %s", spec.file, spec.key, spec.declaration)
		}
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
