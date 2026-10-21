package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

const snapshotFormatVersion = 2

type SourceMetadata struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
}

type SourceRange struct {
	Declaration string `json:"declaration"`
	Path        string `json:"path"`
	StartLine   int    `json:"start_line"`
	EndLine     int    `json:"end_line"`
}

type SourceConfiguration struct {
	DefinesPath   string `json:"defines_path"`
	DefinesSHA256 string `json:"defines_sha256"`
	Preprocessor  string `json:"preprocessor"`
	Scope         string `json:"scope"`
}

type EnumValue struct {
	Name  string `json:"name"`
	Value int64  `json:"value"`
}

type EnumMetadata struct {
	Name   string      `json:"name"`
	Values []EnumValue `json:"values"`
}

type FieldBounds struct {
	Minimum any `json:"minimum,omitempty"`
	Maximum any `json:"maximum,omitempty"`
}

type FieldMetadata struct {
	Key            string       `json:"key"`
	Type           any          `json:"type"`
	Default        any          `json:"default,omitempty"`
	Bounds         *FieldBounds `json:"bounds,omitempty"`
	EvidenceStatus string       `json:"evidence_status"`
}

type EngineSnapshot struct {
	FormatVersion     int                 `json:"format_version"`
	EvidenceStatus    string              `json:"evidence_status"`
	UnavailableReason string              `json:"unavailable_reason,omitempty"`
	Revision          string              `json:"revision"`
	Sources           []SourceMetadata    `json:"sources"`
	Configuration     SourceConfiguration `json:"configuration"`
	FieldsScope       string              `json:"fields_scope"`
	SourceRanges      []SourceRange       `json:"source_ranges"`
	Enums             []EnumMetadata      `json:"enums"`
	Fields            []FieldMetadata     `json:"fields"`
}

type enumDeclaration struct {
	name      string
	body      string
	startLine int
	endLine   int
}

type enumSpec struct {
	Name    string
	File    string
	Catalog string
	Key     string
	Title   string
}

var enumSpecs = []enumSpec{
	{"classes_t", "bg_public.h", "classes.json", "classes", "MBII Classes Enum Catalog"},
	{"MBAttribute_t", "bg_public.h", "attributes.json", "enumValues", "MBII Character Attribute Enum Catalog"},
	{"weapon_t", "bg_weapons.h", "weapons.json", "weapons", "MBII Weapons Enum Catalog"},
	{"forcePowers_t", "q_shared.h", "force_powers.json", "force_powers", "MBII Force Powers Enum Catalog"},
	{"saber_styles_t", "q_shared.h", "saber_styles.json", "saber_styles", "MBII Saber Styles Enum Catalog"},
	{"holdable_t", "bg_public.h", "holdables.json", "holdables", "MBII Holdables and Special Actions Enum Catalog"},
	{"gEntityActionSkill_t", "bg_public.h", "holdables.json", "extended_abilities", "MBII Holdables and Special Actions Enum Catalog"},
	{"powerup_t", "bg_public.h", "powerups.json", "powerups", "MBII Powerups Enum Catalog"},
	{"siegeClassFlags_t", "bg_saga.h", "class_flags.json", "class_flags", "MBII Class and Held Flag Enum Catalog"},
	{"siegeWeaponFlags_t", "bg_saga.h", "class_flags.json", "weapon_held_flags", "MBII Class and Held Flag Enum Catalog"},
}

var cIntegerSuffix = regexp.MustCompile(`(?i)(0[xX][0-9a-f]+|[0-9]+)[uUlL]+\b`)

func main() {
	verify := flag.Bool("verify", false, "compare the committed snapshot with MBII_ENGINE_SRC instead of writing it")
	snapshotPath := flag.String("snapshot", "", "snapshot path (default: <repository>/go_module/testdata/engine_snapshot.json)")
	schemaPath := flag.String("schema", "", "editor schema path used to annotate parser-key metadata")
	updateSchemas := flag.Bool("update-schemas", false, "also regenerate enum catalogs and the MBClass schema constraint")
	flag.Parse()
	if *verify && *updateSchemas {
		fatal(errors.New("-verify cannot be combined with -update-schemas"))
	}

	repoRoot, err := findRepositoryRoot()
	if err != nil {
		fatal(err)
	}
	if *snapshotPath == "" {
		*snapshotPath = filepath.Join(repoRoot, "go_module", "testdata", "engine_snapshot.json")
	}
	if *schemaPath == "" {
		*schemaPath = filepath.Join(repoRoot, "schemas", "mbch_schema.json")
	}

	engineRoot := strings.TrimSpace(os.Getenv("MBII_ENGINE_SRC"))
	if engineRoot == "" {
		mode := "generation"
		if *verify {
			mode = "explicit verification"
		}
		fatal(fmt.Errorf("%s requires MBII_ENGINE_SRC to name an engine checkout", mode))
	}

	snapshot, err := deriveSnapshot(engineRoot, *schemaPath)
	if err != nil {
		fatal(err)
	}
	encoded, err := marshalSnapshot(snapshot)
	if err != nil {
		fatal(err)
	}
	var schemaUpdates map[string][]byte
	if *updateSchemas {
		schemaUpdates, err = deriveSchemaUpdates(snapshot, *schemaPath)
		if err != nil {
			fatal(err)
		}
	}

	if *verify {
		committed, err := os.ReadFile(*snapshotPath)
		if err != nil {
			fatal(fmt.Errorf("read committed snapshot %s: %w", *snapshotPath, err))
		}
		var existing EngineSnapshot
		if err := json.Unmarshal(committed, &existing); err != nil {
			fatal(fmt.Errorf("parse committed snapshot %s: %w", *snapshotPath, err))
		}
		existingEncoded, err := marshalSnapshot(existing)
		if err != nil {
			fatal(err)
		}
		if !bytes.Equal(existingEncoded, encoded) {
			fatal(fmt.Errorf("engine metadata mismatch for revision %s: regenerate %s with the same MBII_ENGINE_SRC checkout", snapshot.Revision, *snapshotPath))
		}
		fmt.Printf("verified %s against engine revision %s\n", *snapshotPath, snapshot.Revision)
		return
	}

	if err := os.MkdirAll(filepath.Dir(*snapshotPath), 0o755); err != nil {
		fatal(fmt.Errorf("create snapshot directory: %w", err))
	}
	if err := os.WriteFile(*snapshotPath, encoded, 0o644); err != nil {
		fatal(fmt.Errorf("write snapshot: %w", err))
	}
	for path, content := range schemaUpdates {
		if err := os.WriteFile(path, content, 0o644); err != nil {
			fatal(fmt.Errorf("write schema %s: %w", path, err))
		}
	}
	fmt.Printf("generated %s from engine revision %s\n", *snapshotPath, snapshot.Revision)
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "engine snapshot:", err)
	os.Exit(1)
}

func findRepositoryRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if isDir(filepath.Join(dir, "go_module")) && isDir(filepath.Join(dir, "schemas")) {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", errors.New("repository root not found (expected go_module/ and schemas/)")
		}
		dir = parent
	}
}

func isDir(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

func deriveSnapshot(engineRoot, schemaPath string) (EngineSnapshot, error) {
	root, err := filepath.Abs(engineRoot)
	if err != nil {
		return EngineSnapshot{}, fmt.Errorf("resolve MBII_ENGINE_SRC: %w", err)
	}
	root, err = filepath.EvalSymlinks(root)
	if err != nil {
		return EngineSnapshot{}, fmt.Errorf("resolve MBII_ENGINE_SRC symlinks: %w", err)
	}
	revision, gitRoot, err := engineRevision(root)
	if err != nil {
		return EngineSnapshot{}, err
	}
	type sourceFile struct {
		path         string
		clean        string
		declarations map[string]enumDeclaration
	}
	files := make(map[string]sourceFile)
	basenames := []string{"mb_defines.h", "bg_saga.c"}
	for _, spec := range enumSpecs {
		basenames = append(basenames, spec.File)
	}
	sort.Strings(basenames)
	var sources []SourceMetadata
	for _, base := range basenames {
		if _, loaded := files[base]; loaded {
			continue
		}
		path, err := resolveEngineFile(root, base)
		if err != nil {
			return EngineSnapshot{}, err
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return EngineSnapshot{}, fmt.Errorf("read %s: %w", path, err)
		}
		relative, err := filepath.Rel(gitRoot, path)
		if err != nil {
			return EngineSnapshot{}, err
		}
		relative = filepath.ToSlash(relative)
		if err := verifySourceAtRevision(gitRoot, revision, relative, content); err != nil {
			return EngineSnapshot{}, err
		}
		clean, err := stripCComments(string(content))
		if err != nil {
			return EngineSnapshot{}, fmt.Errorf("strip comments from %s: %w", relative, err)
		}
		var declarations map[string]enumDeclaration
		if base != "mb_defines.h" && base != "bg_saga.c" {
			declarations, err = findEnumDeclarations(clean)
			if err != nil {
				return EngineSnapshot{}, fmt.Errorf("parse enums in %s: %w", relative, err)
			}
		}
		files[base] = sourceFile{path: relative, clean: clean, declarations: declarations}
		sources = append(sources, SourceMetadata{Path: relative, SHA256: sha256Hex(content)})
	}
	var enumSource strings.Builder
	var ranges []SourceRange
	for _, spec := range enumSpecs {
		source := files[spec.File]
		decl, ok := source.declarations[spec.Name]
		if !ok {
			return EngineSnapshot{}, fmt.Errorf("required enum declaration %s not found in %s", spec.Name, source.path)
		}
		fmt.Fprintf(&enumSource, "typedef enum {%s} %s;\n", decl.body, spec.Name)
		ranges = append(ranges, SourceRange{Declaration: spec.Name, Path: source.path, StartLine: decl.startLine, EndLine: decl.endLine})
	}
	preprocessed, err := preprocessEnums(files["mb_defines.h"].clean, enumSource.String())
	if err != nil {
		return EngineSnapshot{}, err
	}
	activeDeclarations, err := findEnumDeclarations(preprocessed)
	if err != nil {
		return EngineSnapshot{}, fmt.Errorf("parse preprocessed enums: %w", err)
	}
	var enums []EnumMetadata
	for _, spec := range enumSpecs {
		active, ok := activeDeclarations[spec.Name]
		if !ok {
			return EngineSnapshot{}, fmt.Errorf("preprocessor omitted enum declaration %s", spec.Name)
		}
		values, err := parseEnumBody(active.body)
		if err != nil {
			return EngineSnapshot{}, fmt.Errorf("parse enum %s: %w", spec.Name, err)
		}
		enums = append(enums, EnumMetadata{Name: spec.Name, Values: values})
	}
	fieldKeys, err := extractParserFieldKeys(files["bg_saga.c"].clean)
	if err != nil {
		return EngineSnapshot{}, fmt.Errorf("extract parser fields: %w", err)
	}
	fields, err := annotateFields(fieldKeys, schemaPath)
	if err != nil {
		return EngineSnapshot{}, err
	}
	configuration := SourceConfiguration{
		DefinesPath:  files["mb_defines.h"].path,
		Preprocessor: "cc -std=c99 -E -P -undef -nostdinc -x c -",
		Scope:        "Isolated enum declarations with mb_defines.h source defaults only; no other includes or build flags. Not evidence of a compiled or deployed configuration, parser acceptance, or gameplay behavior.",
	}
	for _, source := range sources {
		if source.Path == configuration.DefinesPath {
			configuration.DefinesSHA256 = source.SHA256
		}
	}
	return EngineSnapshot{
		FormatVersion:  snapshotFormatVersion,
		EvidenceStatus: "Verified",
		Revision:       revision,
		Sources:        sources,
		Configuration:  configuration,
		FieldsScope:    "Parser-key occurrences before preprocessing; inactive branches may be present. Types, defaults, and bounds are unverified editor annotations.",
		SourceRanges:   ranges,
		Enums:          enums,
		Fields:         fields,
	}, nil
}

func resolveEngineFile(root, base string) (string, error) {
	candidates := []string{
		filepath.Join(root, base),
		filepath.Join(root, "game", base),
		filepath.Join(root, "codemp", "game", base),
	}
	for _, candidate := range candidates {
		info, err := os.Stat(candidate)
		if err == nil && !info.IsDir() {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("%s not found under MBII_ENGINE_SRC %s (checked root, game/, and codemp/game/)", base, root)
}

func engineRevision(root string) (revision, gitRoot string, err error) {
	cmd := exec.Command("git", "-C", root, "rev-parse", "--show-toplevel", "HEAD")
	out, cmdErr := cmd.Output()
	if cmdErr != nil {
		return "", "", fmt.Errorf("derive engine revision from %s: %w", root, cmdErr)
	}
	parts := strings.Fields(string(out))
	if len(parts) != 2 {
		return "", "", fmt.Errorf("unexpected git rev-parse output for %s", root)
	}
	gitRoot = parts[0]
	revision = parts[1]
	if ok, _ := regexp.MatchString(`^[0-9a-fA-F]{40,64}$`, revision); !ok {
		return "", "", fmt.Errorf("git returned invalid revision %q", revision)
	}
	status, err := exec.Command("git", "--no-optional-locks", "-C", gitRoot, "status", "--porcelain=v1", "--untracked-files=no").Output()
	if err != nil {
		return "", "", fmt.Errorf("check engine source provenance: %w", err)
	}
	if len(bytes.TrimSpace(status)) != 0 {
		return "", "", fmt.Errorf("engine source checkout does not match revision %s; tracked files are modified", revision)
	}
	return strings.ToLower(revision), gitRoot, nil
}

func verifySourceAtRevision(gitRoot, revision, relativePath string, workingBytes []byte) error {
	object := revision + ":" + relativePath
	cmd := exec.Command("git", "-C", gitRoot, "cat-file", "blob", object)
	revisionBytes, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("read committed engine source %s at revision %s: %w", relativePath, revision, err)
	}
	if !bytes.Equal(workingBytes, revisionBytes) {
		return fmt.Errorf("engine source %s does not match revision %s; MBII_ENGINE_SRC must contain the exact committed HEAD bytes", relativePath, revision)
	}
	return nil
}

func sha256Hex(content []byte) string {
	sum := sha256.Sum256(content)
	return fmt.Sprintf("%x", sum[:])
}

// preprocessEnums uses the native C preprocessor without building the engine.
// Only the verified feature header and isolated declarations are inputs.
func preprocessEnums(defines, declarations string) (string, error) {
	cleanDefines, err := stripCComments(defines)
	if err != nil {
		return "", fmt.Errorf("strip feature-header comments: %w", err)
	}
	if regexp.MustCompile(`(?m)^\s*#\s*(?:include|include_next|import)\b`).MatchString(cleanDefines + "\n" + declarations) {
		return "", errors.New("isolated enum preprocessing does not support additional includes; their source provenance is unavailable")
	}
	cmd := exec.Command("cc", "-std=c99", "-E", "-P", "-undef", "-nostdinc", "-x", "c", "-")
	cmd.Stdin = strings.NewReader(cleanDefines + "\n" + declarations)
	var diagnostics bytes.Buffer
	cmd.Stderr = &diagnostics
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("preprocess source-default enums (requires cc): %w\n%s", err, diagnostics.String())
	}
	return string(output), nil
}

// stripCComments replaces comment bytes with spaces while preserving newlines
// and byte offsets. String and character literals are left intact.
func stripCComments(source string) (string, error) {
	out := []byte(source)
	const (
		normal = iota
		lineComment
		blockComment
		stringLiteral
		charLiteral
	)
	state := normal
	escaped := false
	for i := 0; i < len(out); i++ {
		c := out[i]
		switch state {
		case normal:
			switch {
			case c == '/' && i+1 < len(out) && out[i+1] == '/':
				out[i], out[i+1] = ' ', ' '
				i++
				state = lineComment
			case c == '/' && i+1 < len(out) && out[i+1] == '*':
				out[i], out[i+1] = ' ', ' '
				i++
				state = blockComment
			case c == '"':
				state = stringLiteral
				escaped = false
			case c == '\'':
				state = charLiteral
				escaped = false
			}
		case lineComment:
			if c == '\n' {
				state = normal
			} else {
				out[i] = ' '
			}
		case blockComment:
			if c == '*' && i+1 < len(out) && out[i+1] == '/' {
				out[i], out[i+1] = ' ', ' '
				i++
				state = normal
			} else if c != '\n' {
				out[i] = ' '
			}
		case stringLiteral, charLiteral:
			if escaped {
				escaped = false
				continue
			}
			if c == '\\' {
				escaped = true
				continue
			}
			if (state == stringLiteral && c == '"') || (state == charLiteral && c == '\'') {
				state = normal
			}
		}
	}
	if state == blockComment {
		return "", errors.New("unterminated block comment")
	}
	return string(out), nil
}

func findEnumDeclarations(source string) (map[string]enumDeclaration, error) {
	startRE := regexp.MustCompile(`\b(?:typedef\s+)?enum(?:\s+[A-Za-z_]\w*)?\s*\{`)
	result := make(map[string]enumDeclaration)
	for _, loc := range startRE.FindAllStringIndex(source, -1) {
		open := strings.LastIndex(source[loc[0]:loc[1]], "{") + loc[0]
		close, err := matchingDelimiter(source, open, '{', '}')
		if err != nil {
			return nil, err
		}
		semi := strings.IndexByte(source[close:], ';')
		if semi < 0 {
			return nil, fmt.Errorf("enum at line %d has no terminating semicolon", lineAt(source, loc[0]))
		}
		semi += close
		header := source[loc[0]:open]
		tail := strings.TrimSpace(source[close+1 : semi])
		name := ""
		if strings.HasPrefix(strings.TrimSpace(header), "typedef") {
			fields := strings.Fields(tail)
			if len(fields) > 0 {
				name = fields[0]
			}
		} else {
			fields := strings.Fields(header)
			if len(fields) >= 2 {
				name = fields[1]
			}
		}
		// The engine declares powerup_t and holdable_t as an anonymous enum
		// immediately followed by a typedef int. Require that exact adjacency.
		if name == "" && strings.TrimSpace(header) == "enum" && tail == "" {
			alias := regexp.MustCompile(`^\s*typedef\s+int\s+([A-Za-z_]\w*)\s*;`).FindStringSubmatchIndex(source[semi+1:])
			if alias != nil {
				name = source[semi+1+alias[2] : semi+1+alias[3]]
				semi += alias[1]
			}
		}
		if !regexp.MustCompile(`^[A-Za-z_]\w*$`).MatchString(name) {
			continue
		}
		if _, duplicate := result[name]; duplicate {
			return nil, fmt.Errorf("duplicate enum declaration %s", name)
		}
		result[name] = enumDeclaration{
			name:      name,
			body:      source[open+1 : close],
			startLine: lineAt(source, loc[0]),
			endLine:   lineAt(source, semi),
		}
	}
	return result, nil
}

func matchingDelimiter(source string, open int, left, right byte) (int, error) {
	depth := 0
	inString, inChar, escaped := false, false, false
	for i := open; i < len(source); i++ {
		c := source[i]
		if escaped {
			escaped = false
			continue
		}
		if (inString || inChar) && c == '\\' {
			escaped = true
			continue
		}
		if !inChar && c == '"' {
			inString = !inString
			continue
		}
		if !inString && c == '\'' {
			inChar = !inChar
			continue
		}
		if inString || inChar {
			continue
		}
		if c == left {
			depth++
		} else if c == right {
			depth--
			if depth == 0 {
				return i, nil
			}
		}
	}
	return 0, fmt.Errorf("unclosed %q at line %d", left, lineAt(source, open))
}

func lineAt(source string, offset int) int {
	return 1 + strings.Count(source[:offset], "\n")
}

func parseEnumBody(body string) ([]EnumValue, error) {
	parts, err := splitTopLevel(body, ',')
	if err != nil {
		return nil, err
	}
	values := make(map[string]int64)
	ordered := make([]EnumValue, 0, len(parts))
	var next int64
	for _, raw := range parts {
		item := strings.TrimSpace(raw)
		if item == "" {
			continue
		}
		name, expression, hasExpression := strings.Cut(item, "=")
		name = strings.TrimSpace(name)
		if !regexp.MustCompile(`^[A-Za-z_]\w*$`).MatchString(name) {
			return nil, fmt.Errorf("unsupported enum item %q", item)
		}
		if _, duplicate := values[name]; duplicate {
			return nil, fmt.Errorf("duplicate enum member %s", name)
		}
		value := next
		if hasExpression {
			value, err = evalIntegerExpression(expression, values)
			if err != nil {
				return nil, fmt.Errorf("%s = %s: %w", name, strings.TrimSpace(expression), err)
			}
		}
		values[name] = value
		ordered = append(ordered, EnumValue{Name: name, Value: value})
		next = value + 1
	}
	if len(ordered) == 0 {
		return nil, errors.New("enum has no members")
	}
	return ordered, nil
}

func splitTopLevel(source string, delimiter byte) ([]string, error) {
	var parts []string
	start, depth := 0, 0
	inString, inChar, escaped := false, false, false
	for i := 0; i < len(source); i++ {
		c := source[i]
		if escaped {
			escaped = false
			continue
		}
		if (inString || inChar) && c == '\\' {
			escaped = true
			continue
		}
		if !inChar && c == '"' {
			inString = !inString
			continue
		}
		if !inString && c == '\'' {
			inChar = !inChar
			continue
		}
		if inString || inChar {
			continue
		}
		switch c {
		case '(', '[', '{':
			depth++
		case ')', ']', '}':
			depth--
			if depth < 0 {
				return nil, errors.New("unbalanced enum expression")
			}
		default:
			if c == delimiter && depth == 0 {
				parts = append(parts, source[start:i])
				start = i + 1
			}
		}
	}
	if depth != 0 || inString || inChar {
		return nil, errors.New("unterminated enum expression")
	}
	parts = append(parts, source[start:])
	return parts, nil
}

func evalIntegerExpression(expression string, values map[string]int64) (int64, error) {
	expression = cIntegerSuffix.ReplaceAllString(expression, "$1")
	expr, err := parser.ParseExpr(strings.TrimSpace(expression))
	if err != nil {
		return 0, err
	}
	var eval func(ast.Expr) (int64, error)
	eval = func(node ast.Expr) (int64, error) {
		switch n := node.(type) {
		case *ast.BasicLit:
			if n.Kind != token.INT {
				return 0, fmt.Errorf("non-integer literal %s", n.Value)
			}
			return strconv.ParseInt(n.Value, 0, 64)
		case *ast.Ident:
			value, ok := values[n.Name]
			if !ok {
				return 0, fmt.Errorf("unknown enum reference %s", n.Name)
			}
			return value, nil
		case *ast.ParenExpr:
			return eval(n.X)
		case *ast.UnaryExpr:
			value, err := eval(n.X)
			if err != nil {
				return 0, err
			}
			switch n.Op {
			case token.ADD:
				return value, nil
			case token.SUB:
				return -value, nil
			case token.XOR:
				return ^value, nil
			default:
				return 0, fmt.Errorf("unsupported unary operator %s", n.Op)
			}
		case *ast.BinaryExpr:
			left, err := eval(n.X)
			if err != nil {
				return 0, err
			}
			right, err := eval(n.Y)
			if err != nil {
				return 0, err
			}
			switch n.Op {
			case token.ADD:
				return left + right, nil
			case token.SUB:
				return left - right, nil
			case token.MUL:
				return left * right, nil
			case token.QUO:
				if right == 0 {
					return 0, errors.New("division by zero")
				}
				return left / right, nil
			case token.REM:
				if right == 0 {
					return 0, errors.New("division by zero")
				}
				return left % right, nil
			case token.SHL:
				return left << uint(right), nil
			case token.SHR:
				return left >> uint(right), nil
			case token.OR:
				return left | right, nil
			case token.AND:
				return left & right, nil
			case token.XOR:
				return left ^ right, nil
			default:
				return 0, fmt.Errorf("unsupported binary operator %s", n.Op)
			}
		default:
			return 0, fmt.Errorf("unsupported expression %T", node)
		}
	}
	return eval(expr)
}

func extractParserFieldKeys(source string) ([]string, error) {
	callRE := regexp.MustCompile(`\b(?:SGPV|BG_SiegeGetPairedValue)\s*\(`)
	seen := make(map[string]bool)
	for _, loc := range callRE.FindAllStringIndex(source, -1) {
		open := strings.LastIndex(source[loc[0]:loc[1]], "(") + loc[0]
		close, err := matchingDelimiter(source, open, '(', ')')
		if err != nil {
			return nil, err
		}
		key, ok := firstCStringLiteral(source[open+1 : close])
		if ok && key != "" {
			seen[key] = true
		}
	}
	keys := make([]string, 0, len(seen))
	for key := range seen {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys, nil
}

func firstCStringLiteral(source string) (string, bool) {
	for i := 0; i < len(source); i++ {
		if source[i] != '"' {
			continue
		}
		start := i + 1
		escaped := false
		for i = start; i < len(source); i++ {
			if escaped {
				escaped = false
				continue
			}
			if source[i] == '\\' {
				escaped = true
				continue
			}
			if source[i] == '"' {
				literal, err := strconv.Unquote(source[start-1 : i+1])
				return literal, err == nil
			}
		}
		return "", false
	}
	return "", false
}

func annotateFields(keys []string, schemaPath string) ([]FieldMetadata, error) {
	schemaMetadata, err := loadSchemaFieldMetadata(schemaPath)
	if err != nil {
		return nil, err
	}
	fields := make([]FieldMetadata, 0, len(keys))
	for _, key := range keys {
		field := FieldMetadata{Key: key, Type: "unknown", EvidenceStatus: "SourceKeyVerified"}
		if schema, ok := schemaMetadata[key]; ok {
			field.Type = schema.Type
			field.Default = schema.Default
			field.Bounds = schema.Bounds
			field.EvidenceStatus = "SourceKeyVerified_MetadataUnverified"
		}
		fields = append(fields, field)
	}
	return fields, nil
}

func loadSchemaFieldMetadata(path string) (map[string]FieldMetadata, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read schema metadata %s: %w", path, err)
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
		return nil, fmt.Errorf("parse schema metadata %s: %w", path, err)
	}
	result := make(map[string]FieldMetadata)
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
			if _, alreadyAnnotated := result[key]; alreadyAnnotated {
				continue
			}
			property := definition.Properties[key]
			bounds := (*FieldBounds)(nil)
			if property.Minimum != nil || property.Maximum != nil {
				bounds = &FieldBounds{Minimum: property.Minimum, Maximum: property.Maximum}
			}
			result[key] = FieldMetadata{Type: property.Type, Default: property.Default, Bounds: bounds}
		}
	}
	return result, nil
}

func marshalSnapshot(snapshot EngineSnapshot) ([]byte, error) {
	if snapshot.FormatVersion != snapshotFormatVersion {
		return nil, fmt.Errorf("unsupported snapshot format_version %d", snapshot.FormatVersion)
	}
	encoded, err := json.MarshalIndent(snapshot, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(encoded, '\n'), nil
}

func deriveSchemaUpdates(snapshot EngineSnapshot, schemaPath string) (map[string][]byte, error) {
	type catalogEngine struct {
		Revision      string              `json:"revision"`
		SourcePath    string              `json:"source_path"`
		SourceSHA256  string              `json:"source_sha256"`
		SourceRanges  []SourceRange       `json:"source_ranges"`
		Configuration SourceConfiguration `json:"configuration"`
	}
	type catalogValue struct {
		Value          int64  `json:"value"`
		EvidenceStatus string `json:"evidence_status"`
	}
	enums := make(map[string][]EnumValue)
	for _, enum := range snapshot.Enums {
		enums[enum.Name] = enum.Values
	}
	catalogs := make(map[string]map[string]any)
	var classNames []string
	for _, spec := range enumSpecs {
		values, ok := enums[spec.Name]
		if !ok {
			return nil, fmt.Errorf("schema generation requires enum %s", spec.Name)
		}
		catalog, exists := catalogs[spec.Catalog]
		if !exists {
			catalog = map[string]any{
				"$schema":         "http://json-schema.org/draft-07/schema#",
				"$id":             spec.Catalog,
				"title":           spec.Title,
				"version":         "3.0.0",
				"evidence_status": "SourceDefaultsEnumValuesVerified",
				"engine":          &catalogEngine{Revision: snapshot.Revision, Configuration: snapshot.Configuration},
				"description":     "Mechanically derived active enum identifiers and integer values, including sentinel/count markers, under the recorded source-default configuration. Parser acceptance, gameplay mechanics, rank bounds, and deployed feature configuration are not established by an enum declaration.",
			}
			catalogs[spec.Catalog] = catalog
		}
		metadata := catalog["engine"].(*catalogEngine)
		foundRange := false
		for _, sourceRange := range snapshot.SourceRanges {
			if sourceRange.Declaration == spec.Name {
				if metadata.SourcePath != "" && metadata.SourcePath != sourceRange.Path {
					return nil, fmt.Errorf("catalog %s spans unexpected source files", spec.Catalog)
				}
				metadata.SourcePath = sourceRange.Path
				metadata.SourceRanges = append(metadata.SourceRanges, sourceRange)
				foundRange = true
			}
		}
		for _, source := range snapshot.Sources {
			if source.Path == metadata.SourcePath {
				metadata.SourceSHA256 = source.SHA256
			}
		}
		if metadata.SourceSHA256 == "" || !foundRange {
			return nil, fmt.Errorf("schema generation requires source provenance for %s", spec.Name)
		}
		if spec.Name == "MBAttribute_t" {
			numeric := make(map[string]int64, len(values))
			var names []string
			for _, value := range values {
				numeric[value.Name] = value.Value
				if value.Name != "MB_ATT_INVALID" && value.Name != "NO_OF_MB_ATTRIBUTES" {
					names = append(names, value.Name)
				}
			}
			catalog[spec.Key], catalog["allAttributeIds"] = numeric, names
		} else {
			entries := make(map[string]catalogValue, len(values))
			for _, value := range values {
				entries[value.Name] = catalogValue{Value: value.Value, EvidenceStatus: "SourceDefaultsVerified"}
				if spec.Name == "classes_t" && value.Name != "MB_CLASS_NOCLASS" && value.Name != "NO_OF_CLASSES" {
					classNames = append(classNames, value.Name)
				}
			}
			catalog[spec.Key] = entries
		}
		if spec.Catalog == "class_flags.json" {
			catalog["value_semantics"] = "Enum bit indices, not scalar masks. HELD indices address the engine's two-word held-flag storage; indices at or above 32 belong to the second word."
		}
	}
	updates := make(map[string][]byte)
	for filename, catalog := range catalogs {
		content, err := json.MarshalIndent(catalog, "", "  ")
		if err != nil {
			return nil, err
		}
		updates[filepath.Join(filepath.Dir(schemaPath), "enums", filename)] = append(content, '\n')
	}
	if len(classNames) == 0 {
		return nil, errors.New("schema generation requires selectable classes")
	}
	schema, err := os.ReadFile(schemaPath)
	if err != nil {
		return nil, fmt.Errorf("read schema %s: %w", schemaPath, err)
	}
	// Decode the property to locate its exact bytes, preserving all unrelated
	// schema formatting and annotations rather than reserializing the document.
	property := json.RawMessage(schema)
	for _, key := range []string{"definitions", "ClassInfo", "properties", "MBClass"} {
		var object map[string]json.RawMessage
		if err := json.Unmarshal(property, &object); err != nil {
			return nil, fmt.Errorf("decode schema property %s: %w", key, err)
		}
		property = object[key]
		if len(property) == 0 {
			return nil, fmt.Errorf("schema is missing %s", key)
		}
	}
	var classProperty map[string]any
	if err := json.Unmarshal(property, &classProperty); err != nil {
		return nil, err
	}
	if classProperty["type"] != "string" || bytes.Count(schema, property) != 1 {
		return nil, errors.New("schema MBClass must be an unambiguous string property")
	}
	classProperty["description"] = "Base class identifier in the recorded source-default enum; does not establish deployed availability or gameplay behavior."
	classProperty["evidenceStatus"] = "SourceDefaultsEnumMembershipVerified"
	classProperty["engineRevision"] = snapshot.Revision
	classProperty["sourceConfiguration"] = snapshot.Configuration
	classProperty["enum"] = classNames
	replacement, err := json.MarshalIndent(classProperty, "        ", "  ")
	if err != nil {
		return nil, err
	}
	updates[schemaPath] = bytes.Replace(schema, property, replacement, 1)
	return updates, nil
}
