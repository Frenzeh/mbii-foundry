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

const snapshotFormatVersion = 1

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
	FormatVersion     int              `json:"format_version"`
	EvidenceStatus    string           `json:"evidence_status"`
	UnavailableReason string           `json:"unavailable_reason,omitempty"`
	Revision          string           `json:"revision"`
	Sources           []SourceMetadata `json:"sources"`
	SourceRanges      []SourceRange    `json:"source_ranges"`
	Enums             []EnumMetadata   `json:"enums"`
	Fields            []FieldMetadata  `json:"fields"`
}

type enumDeclaration struct {
	name      string
	body      string
	startLine int
	endLine   int
}

var cIntegerSuffix = regexp.MustCompile(`(?i)(0[xX][0-9a-f]+|[0-9]+)[uUlL]+\b`)

func main() {
	verify := flag.Bool("verify", false, "compare the committed snapshot with MBII_ENGINE_SRC instead of writing it")
	snapshotPath := flag.String("snapshot", "", "snapshot path (default: <repository>/go_module/testdata/engine_snapshot.json)")
	schemaPath := flag.String("schema", "", "schema path used only to annotate parser-key metadata")
	flag.Parse()

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
	publicPath, err := resolveEngineFile(root, "bg_public.h")
	if err != nil {
		return EngineSnapshot{}, err
	}
	sagaPath, err := resolveEngineFile(root, "bg_saga.c")
	if err != nil {
		return EngineSnapshot{}, err
	}
	publicBytes, err := os.ReadFile(publicPath)
	if err != nil {
		return EngineSnapshot{}, fmt.Errorf("read %s: %w", publicPath, err)
	}
	sagaBytes, err := os.ReadFile(sagaPath)
	if err != nil {
		return EngineSnapshot{}, fmt.Errorf("read %s: %w", sagaPath, err)
	}

	revision, gitRoot, err := engineRevision(root)
	if err != nil {
		return EngineSnapshot{}, err
	}
	publicRel, err := filepath.Rel(gitRoot, publicPath)
	if err != nil {
		return EngineSnapshot{}, err
	}
	sagaRel, err := filepath.Rel(gitRoot, sagaPath)
	if err != nil {
		return EngineSnapshot{}, err
	}
	publicRel = filepath.ToSlash(publicRel)
	sagaRel = filepath.ToSlash(sagaRel)

	if err := verifySourceAtRevision(gitRoot, revision, publicRel, publicBytes); err != nil {
		return EngineSnapshot{}, err
	}
	if err := verifySourceAtRevision(gitRoot, revision, sagaRel, sagaBytes); err != nil {
		return EngineSnapshot{}, err
	}

	cleanPublic, err := stripCComments(string(publicBytes))
	if err != nil {
		return EngineSnapshot{}, fmt.Errorf("strip comments from %s: %w", publicRel, err)
	}
	declarations, err := findEnumDeclarations(cleanPublic)
	if err != nil {
		return EngineSnapshot{}, fmt.Errorf("parse enums in %s: %w", publicRel, err)
	}

	requested := []string{"classes_t", "MBAttribute_t"}
	var enums []EnumMetadata
	var ranges []SourceRange
	for _, name := range requested {
		decl, ok := declarations[name]
		if !ok {
			return EngineSnapshot{}, fmt.Errorf("required enum declaration %s not found in %s", name, publicRel)
		}
		values, err := parseEnumBody(decl.body)
		if err != nil {
			return EngineSnapshot{}, fmt.Errorf("parse enum %s: %w", name, err)
		}
		enums = append(enums, EnumMetadata{Name: name, Values: values})
		ranges = append(ranges, SourceRange{Declaration: name, Path: publicRel, StartLine: decl.startLine, EndLine: decl.endLine})
	}

	cleanSaga, err := stripCComments(string(sagaBytes))
	if err != nil {
		return EngineSnapshot{}, fmt.Errorf("strip comments from %s: %w", sagaRel, err)
	}
	fieldKeys, err := extractParserFieldKeys(cleanSaga)
	if err != nil {
		return EngineSnapshot{}, fmt.Errorf("extract parser fields from %s: %w", sagaRel, err)
	}
	fields, err := annotateFields(fieldKeys, schemaPath)
	if err != nil {
		return EngineSnapshot{}, err
	}

	return EngineSnapshot{
		FormatVersion:  snapshotFormatVersion,
		EvidenceStatus: "Verified",
		Revision:       revision,
		Sources: []SourceMetadata{
			{Path: publicRel, SHA256: sha256Hex(publicBytes)},
			{Path: sagaRel, SHA256: sha256Hex(sagaBytes)},
		},
		SourceRanges: ranges,
		Enums:        enums,
		Fields:       fields,
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
		item := strings.TrimSpace(removePreprocessorLines(raw))
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

func removePreprocessorLines(s string) string {
	lines := strings.Split(s, "\n")
	kept := lines[:0]
	for _, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), "#") {
			continue
		}
		kept = append(kept, line)
	}
	return strings.Join(kept, "\n")
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
		field := FieldMetadata{Key: key, Type: "unknown", EvidenceStatus: "KeyVerified"}
		if schema, ok := schemaMetadata[key]; ok {
			field.Type = schema.Type
			field.Default = schema.Default
			field.Bounds = schema.Bounds
			field.EvidenceStatus = "KeyVerified_MetadataUnverified"
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
