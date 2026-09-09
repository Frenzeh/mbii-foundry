package parsers

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
)

// Canonical .mbtc (siege team) document model.
//
// Engine consumer: BG_SiegeParseTeamFile (game/bg_saga.c:3253). One .mbtc
// file describes ONE team. The engine reads, via SGPV/SGVG:
//
//	name              (required — ERR_DROP "Siege team with no name definition")
//	TimePeriod        (int, default MB_TIME_NOTIMESET)
//	EUAllowed         (int, default false)
//	FriendlyShader    (cgame only)
//	Classes           { class1..class6 }  — contiguous; engine stops at the
//	                  first missing classN (MAX_SIEGE_CLASSES_PER_TEAM = 6,
//	                  bg_saga.h:17)
//	SubclassesForClass<N> { Subclass1..Subclass41 } — MAX_SIEGE_SUBCLASSES
//	                  = 42 (bg_saga.h:18), engine loop c < 42, 1-based
//
// `ClassesAllowed` appears in shipped .mbtc files but the current engine
// never reads it (bg_saga.h:476 marks it "// Old Class limits system").
// We preserve it — and any other unmodeled key or group — verbatim so a
// round-trip never deletes content we do not model.
//
// Team identity comes from the FILE (each team is its own file,
// referenced from maps/*.siege or the Legends team lists), never from
// comment markers such as "// Imperial".

const (
	MBTCMaxClasses    = 6  // MAX_SIEGE_CLASSES_PER_TEAM (bg_saga.h:17)
	MBTCMaxSubclasses = 41 // usable SubclassN slots (bg_saga.h:18, 1-based, c < 42)

	// MAX_TEAM_FILE_LEN (bg_saga.c:3252); the reader rejects
	// len >= 4096 (bg_saga.c:3261), so usable content is <= 4095 bytes.
	MBTCMaxFileBytes = 4096
)

type mbtcBaseline struct {
	name              string
	timePeriod        int
	timePeriodSet     bool
	euAllowed         int
	euAllowedSet      bool
	friendlyShader    string
	classesAllowed    int
	classesAllowedSet bool
	classes           [MBTCMaxClasses]string
	classIssues       []string
	subclasses        [MBTCMaxClasses][]string
	subclassIssues    [MBTCMaxClasses][]string
	extraFields       []MBTCExtraField
}

// MBTCExtraField is an unmodeled top-level paired value preserved in
// document order. Quoted records whether the value was written quoted.
type MBTCExtraField struct {
	Key    string
	Value  string
	Quoted bool
}

// MBTCExtraGroup is an unmodeled top-level group preserved as raw body
// text (content between its braces, inner braces included).
type MBTCExtraGroup struct {
	Name string
	Body string
}

// MBTCTeam is the parsed model of one .mbtc file. ParseMBTC always
// returns a fresh value: parsing resets the model by construction, so no
// caller-side reset is needed (or possible to forget).
//
// ctx retains the source document's AST. Serialize syncs only the
// modeled fields onto that AST, so comments, formatting, unknown keys
// and groups, duplicate (dead) entries, and authored numbering all
// survive a round trip; a no-op edit renders byte-identical output.
type MBTCTeam struct {
	ctx                  *sourceContext // original document; nil for teams never parsed
	baseline             *mbtcBaseline  // parsed modeled state; gates changed-field-only sync
	loadedClassIssues    []string
	loadedSubclassIssues [MBTCMaxClasses][]string
	Name                 string
	TimePeriod           int
	TimePeriodSet        bool
	EUAllowed            int
	EUAllowedSet         bool
	FriendlyShader       string

	// Legacy mask. Present in shipped files, unread by the current
	// engine (bg_saga.h:476). Preserved when set, dropped when absent.
	ClassesAllowed    int
	ClassesAllowedSet bool

	Classes    [MBTCMaxClasses]string
	Subclasses [MBTCMaxClasses][]string

	ExtraFields []MBTCExtraField
	ExtraGroups []MBTCExtraGroup

	Diagnostics []string
}

// ParseMBTC parses .mbtc source into a fresh MBTCTeam. Malformed numeric
// fields produce diagnostics (and stay unset) instead of silently
// half-applying; the function itself never fails — problems are reported
// on the returned model so callers can show them next to the document.
func ParseMBTC(source string) *MBTCTeam {
	t := &MBTCTeam{} // fresh model = parse resets state by construction
	tokens, _ := Lex(source)
	doc := parseAST(tokens)
	t.ctx = &sourceContext{doc: doc}

	numeric := func(key, raw string) (int, bool) {
		v, err := strconv.Atoi(strings.TrimSpace(raw))
		if err != nil {
			t.Diagnostics = append(t.Diagnostics,
				fmt.Sprintf("%s: %q is not an integer — ignored", key, raw))
			return 0, false
		}
		return v, true
	}

	// SGPV examines one key/value pair per line, ignores everything after
	// that pair, and stops a value at `//` even inside quotes. Reuse the
	// package's engine-faithful pair walker rather than treating every
	// string token as another top-level key.
	seenKeys := make(map[string]bool)
	walkSGPVPairs(doc.Nodes, func(_ int, authoredKey string, valIdx int, val string, hasVal bool) bool {
		if !hasVal {
			return true // group name or malformed bare key
		}
		key := strings.ToLower(authoredKey)
		if seenKeys[key] {
			t.Diagnostics = append(t.Diagnostics,
				fmt.Sprintf("duplicate top-level %q ignored — the engine uses the first occurrence", authoredKey))
			return true
		}
		seenKeys[key] = true

		switch key {
		case "name":
			t.Name = val
		case "timeperiod":
			if v, ok := numeric("TimePeriod", val); ok {
				t.TimePeriod, t.TimePeriodSet = v, true
			}
		case "euallowed":
			if v, ok := numeric("EUAllowed", val); ok {
				t.EUAllowed, t.EUAllowedSet = v, true
			}
		case "friendlyshader":
			t.FriendlyShader = val
		case "classesallowed":
			if v, ok := numeric("ClassesAllowed", val); ok {
				t.ClassesAllowed, t.ClassesAllowedSet = v, true
			}
		default:
			quoted := false
			if valIdx >= 0 {
				if valueToken, ok := doc.Nodes[valIdx].(*ASTToken); ok {
					quoted = strings.HasPrefix(valueToken.Text, "\"")
				}
			}
			t.ExtraFields = append(t.ExtraFields, MBTCExtraField{
				Key: authoredKey, Value: val, Quoted: quoted,
			})
		}
		return true
	})

	// Groups are first-effective per name, mirroring
	// BG_SiegeGetValueGroup. Duplicate groups remain verbatim dead text.
	seenGroups := make(map[string]bool)
	for _, node := range doc.Nodes {
		b, ok := node.(*ASTBlock)
		if !ok || b.NameToken == nil {
			continue
		}
		name := strings.ToLower(unquote(b.NameToken.Text))
		modeledGroup := name == "classes" || strings.HasPrefix(name, "subclassesforclass")
		if seenGroups[name] && modeledGroup {
			t.Diagnostics = append(t.Diagnostics,
				fmt.Sprintf("duplicate group %q ignored — the engine uses the first occurrence", unquote(b.NameToken.Text)))
			continue
		}

		switch {
		case name == "classes":
			seenGroups[name] = true
			t.parseClassesGroup(b)
		case strings.HasPrefix(name, "subclassesforclass"):
			seenGroups[name] = true
			t.parseSubclassesGroup(b)
		default:
			t.ExtraGroups = append(t.ExtraGroups, MBTCExtraGroup{
				Name: unquote(b.NameToken.Text),
				Body: blockBodyText(b),
			})
		}
	}

	// Contiguity: the engine stops at the first missing classN, so any
	// later classes are silently dead in-game.
	firstGap := -1
	for i := 0; i < MBTCMaxClasses; i++ {
		if t.Classes[i] == "" {
			firstGap = i
			break
		}
	}
	if firstGap >= 0 {
		for i := firstGap + 1; i < MBTCMaxClasses; i++ {
			if t.Classes[i] != "" {
				t.Diagnostics = append(t.Diagnostics,
					fmt.Sprintf("class%d is empty but class%d is set — the engine stops reading Classes at the first empty slot, so class%d will not load",
						firstGap+1, i+1, i+1))
			}
		}
	}
	if t.Name == "" {
		t.Diagnostics = append(t.Diagnostics,
			"no name field — the engine errors out (ERR_DROP) loading a team without name")
	}

	b := &mbtcBaseline{
		name:              t.Name,
		timePeriod:        t.TimePeriod,
		timePeriodSet:     t.TimePeriodSet,
		euAllowed:         t.EUAllowed,
		euAllowedSet:      t.EUAllowedSet,
		friendlyShader:    t.FriendlyShader,
		classesAllowed:    t.ClassesAllowed,
		classesAllowedSet: t.ClassesAllowedSet,
		classes:           t.Classes,
		classIssues:       append([]string(nil), t.loadedClassIssues...),
		subclassIssues:    t.loadedSubclassIssues,
	}
	for i := range t.Subclasses {
		b.subclasses[i] = append([]string(nil), t.Subclasses[i]...)
	}
	b.extraFields = append([]MBTCExtraField(nil), t.ExtraFields...)
	t.baseline = b
	return t
}

func (t *MBTCTeam) parseClassesGroup(b *ASTBlock) {
	seen := make(map[int]bool)
	walkSGPVPairs(b.Children, func(_ int, key string, _ int, value string, hasValue bool) bool {
		if !hasValue {
			return true
		}
		lower := strings.ToLower(key)
		if !strings.HasPrefix(lower, "class") {
			return true
		}
		slot, err := strconv.Atoi(strings.TrimPrefix(lower, "class"))
		if err != nil || slot < 1 {
			return true
		}
		if seen[slot] {
			return true
		}
		seen[slot] = true
		if slot > MBTCMaxClasses {
			issue := fmt.Sprintf("class%d ignored by the engine (max %d classes per team)", slot, MBTCMaxClasses)
			t.Diagnostics = append(t.Diagnostics, issue)
			t.loadedClassIssues = append(t.loadedClassIssues, issue)
			return true
		}
		t.Classes[slot-1] = value
		return true
	})
}

func (t *MBTCTeam) parseSubclassesGroup(b *ASTBlock) {
	suffix := strings.TrimPrefix(strings.ToLower(unquote(b.NameToken.Text)), "subclassesforclass")
	idx, err := strconv.Atoi(suffix)
	if err != nil || idx < 1 || idx > MBTCMaxClasses {
		t.Diagnostics = append(t.Diagnostics,
			fmt.Sprintf("group %q does not match SubclassesForClass1..%d — preserved verbatim", unquote(b.NameToken.Text), MBTCMaxClasses))
		t.ExtraGroups = append(t.ExtraGroups, MBTCExtraGroup{
			Name: unquote(b.NameToken.Text),
			Body: blockBodyText(b),
		})
		return
	}

	authored := make(map[int]string)
	maxSlot := 0
	walkSGPVPairs(b.Children, func(_ int, key string, _ int, value string, hasValue bool) bool {
		if !hasValue {
			return true
		}
		lower := strings.ToLower(key)
		if !strings.HasPrefix(lower, "subclass") {
			return true
		}
		slot, convErr := strconv.Atoi(strings.TrimPrefix(lower, "subclass"))
		if convErr != nil || slot < 1 {
			return true
		}
		if _, exists := authored[slot]; exists {
			return true // first-effective occurrence wins
		}
		authored[slot] = value
		if slot > maxSlot {
			maxSlot = slot
		}
		return true
	})

	if maxSlot > MBTCMaxSubclasses {
		issue := fmt.Sprintf("SubclassesForClass%d has Subclass%d — the engine reads at most %d contiguous entries",
			idx, maxSlot, MBTCMaxSubclasses)
		t.Diagnostics = append(t.Diagnostics, issue)
		t.loadedSubclassIssues[idx-1] = append(t.loadedSubclassIssues[idx-1], issue)
	}
	limit := maxSlot
	if limit > MBTCMaxSubclasses {
		limit = MBTCMaxSubclasses
	}
	for slot := 1; slot <= limit; slot++ {
		value, ok := authored[slot]
		if !ok {
			issue := fmt.Sprintf("SubclassesForClass%d: Subclass%d missing — the engine stops reading at the first gap, later subclasses will not load", idx, slot)
			t.Diagnostics = append(t.Diagnostics, issue)
			t.loadedSubclassIssues[idx-1] = append(t.loadedSubclassIssues[idx-1], issue)
			break
		}
		t.Subclasses[idx-1] = append(t.Subclasses[idx-1], value)
	}
}

// blockBodyText reconstructs a block's inner text (outer braces
// excluded, inner braces and comments included) for verbatim
// preservation of unmodeled groups.
func blockBodyText(b *ASTBlock) string {
	var sb strings.Builder
	for _, child := range b.Children {
		if tok, ok := child.(*ASTToken); ok && tok.Type == TokenWhitespace && sb.Len() == 0 {
			continue // leading whitespace only
		}
		sb.WriteString(child.String())
	}
	return sb.String()
}

// Serialize renders the team. For teams parsed from text, the original
// document AST is retained and ONLY modeled fields are synced onto it —
// comments, formatting, unmodeled keys/groups, duplicate (dead)
// entries, and authored numbering survive, and a no-op edit renders
// byte-identical output. The reconstruct path exists solely for teams
// composed fresh in the UI (no source document to preserve).
func (t *MBTCTeam) Serialize() string {
	if t.ctx != nil && t.ctx.doc != nil {
		// Sync into a private deep clone: the retained baseline
		// (t.ctx.doc) must stay pristine so repeated serializations are
		// stable and a fresh parse of the original source still renders
		// the original bytes.
		doc := cloneASTDocument(t.ctx.doc)
		syncMBTCToAST(t, doc)
		return doc.String()
	}
	return t.reconstruct()
}

func (t *MBTCTeam) reconstruct() string {
	var sb strings.Builder
	sb.WriteString("//Siege team definition file.\n")
	sb.WriteString("// Generated by MBII Foundry\n\n")

	// Values are written as plain quoted strings. %q is deliberately
	// avoided: it emits backslash/uX escapes the SGPV consumer cannot
	// parse. Validate() rejects values that cannot be represented, so
	// plain quoting here is always engine-true.
	sb.WriteString(fmt.Sprintf("name\t\t\t\"%s\"\n", t.Name))
	if t.TimePeriodSet {
		sb.WriteString(fmt.Sprintf("TimePeriod\t%d\n", t.TimePeriod))
	}
	if t.EUAllowedSet {
		sb.WriteString(fmt.Sprintf("EUAllowed\t%d\n", t.EUAllowed))
	}
	if t.FriendlyShader != "" {
		sb.WriteString(fmt.Sprintf("FriendlyShader\t\"%s\"\n", t.FriendlyShader))
	}
	if t.ClassesAllowedSet {
		// Legacy mask — unread by the current engine, preserved so the
		// file keeps working with anything that still expects it.
		sb.WriteString(fmt.Sprintf("ClassesAllowed\t%d\n", t.ClassesAllowed))
	}
	for _, f := range t.ExtraFields {
		if f.Quoted {
			sb.WriteString(fmt.Sprintf("%s\t\"%s\"\n", f.Key, f.Value))
		} else {
			sb.WriteString(fmt.Sprintf("%s\t%s\n", f.Key, f.Value))
		}
	}

	if t.hasClasses() {
		sb.WriteString("\nClasses\n{\n")
		for i, cls := range t.Classes {
			if cls != "" {
				sb.WriteString(fmt.Sprintf("\tclass%d\t\t\"%s\"\n", i+1, cls))
			}
		}
		sb.WriteString("}\n")
	}

	for i, subs := range t.Subclasses {
		if len(subs) == 0 {
			continue
		}
		fmt.Fprintf(&sb, "\nSubclassesForClass%d\n{\n", i+1)
		for c, sub := range subs {
			sb.WriteString(fmt.Sprintf("\tSubclass%d\t\"%s\"\n", c+1, sub))
		}
		sb.WriteString("}\n")
	}

	for _, g := range t.ExtraGroups {
		sb.WriteString("\n" + g.Name + "\n{\n" + g.Body + "\n}\n")
	}
	return sb.String()
}

// syncMBTCToAST writes only modeled fields whose values changed from the
// parsed baseline. Unknown fields/groups are never rewritten. Clearing a
// modeled value removes every duplicate occurrence so a formerly dead
// duplicate cannot become effective.
func syncMBTCToAST(t *MBTCTeam, doc *ASTDocument) {
	base := t.baseline
	if base == nil {
		return
	}

	if t.Name != base.name {
		if t.Name == "" {
			removeDocTopLevelField(doc, "name")
		} else {
			setDocTopLevelField(doc, "name", t.Name)
		}
	}

	type topField struct {
		key         string
		set         bool
		baselineSet bool
		value       string
		baseline    string
	}
	for _, f := range []topField{
		{key: "TimePeriod", set: t.TimePeriodSet, baselineSet: base.timePeriodSet, value: strconv.Itoa(t.TimePeriod), baseline: strconv.Itoa(base.timePeriod)},
		{key: "EUAllowed", set: t.EUAllowedSet, baselineSet: base.euAllowedSet, value: strconv.Itoa(t.EUAllowed), baseline: strconv.Itoa(base.euAllowed)},
		{key: "FriendlyShader", set: t.FriendlyShader != "", baselineSet: base.friendlyShader != "", value: t.FriendlyShader, baseline: base.friendlyShader},
		{key: "ClassesAllowed", set: t.ClassesAllowedSet, baselineSet: base.classesAllowedSet, value: strconv.Itoa(t.ClassesAllowed), baseline: strconv.Itoa(base.classesAllowed)},
	} {
		if f.set == f.baselineSet && (!f.set || f.value == f.baseline) {
			continue
		}
		if f.set {
			setDocTopLevelField(doc, f.key, f.value)
		} else {
			removeDocTopLevelField(doc, f.key)
		}
	}
	for _, extra := range t.ExtraFields {
		changed := true
		for _, original := range base.extraFields {
			if strings.EqualFold(extra.Key, original.Key) {
				changed = extra.Value != original.Value
				break
			}
		}
		if changed {
			setDocTopLevelField(doc, extra.Key, extra.Value)
		}
	}

	t.syncClasses(doc)
	t.syncSubclasses(doc)
}

func (t *MBTCTeam) syncClasses(doc *ASTDocument) {
	var classes *ASTBlock
	for _, node := range doc.Nodes {
		if b, ok := node.(*ASTBlock); ok && b.NameToken != nil &&
			strings.EqualFold(unquote(b.NameToken.Text), "classes") {
			classes = b
			break
		}
	}

	changed := false
	for i := range t.Classes {
		if t.Classes[i] != t.baseline.classes[i] {
			changed = true
			break
		}
	}
	if !changed {
		return
	}
	if classes == nil {
		if !t.hasClasses() {
			return
		}
		classes = &ASTBlock{
			NameToken:  &ASTToken{Type: TokenString, Text: "Classes"},
			Preamble:   []ASTNode{&ASTToken{Type: TokenWhitespace, Text: "\n\n"}},
			OpenBrace:  &ASTToken{Type: TokenBraceOpen, Text: "{"},
			Children:   []ASTNode{&ASTToken{Type: TokenWhitespace, Text: "\n"}},
			CloseBrace: &ASTToken{Type: TokenBraceClose, Text: "}"},
		}
		doc.Nodes = append(doc.Nodes, classes)
	}
	removeKeys := make(map[string]bool)
	walkSGPVPairs(classes.Children, func(_ int, key string, _ int, _ string, hasValue bool) bool {
		if !hasValue {
			return true
		}
		lower := strings.ToLower(key)
		if !strings.HasPrefix(lower, "class") {
			return true
		}
		slot, err := strconv.Atoi(strings.TrimPrefix(lower, "class"))
		if err == nil && slot > MBTCMaxClasses {
			removeKeys[lower] = true
		}
		return true
	})
	for key := range removeKeys {
		removeEngineBlockField(classes, key)
	}
	for i, cls := range t.Classes {
		if cls == t.baseline.classes[i] {
			continue
		}
		key := fmt.Sprintf("class%d", i+1)
		if cls == "" {
			removeEngineBlockField(classes, key)
			continue
		}
		setEngineBlockField(classes, key, cls, strings.ContainsAny(cls, " \t"))
	}
}

func stringSlicesEqual(a, b []string) bool {
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

func (t *MBTCTeam) syncSubclasses(doc *ASTDocument) {
	for i, subs := range t.Subclasses {
		if stringSlicesEqual(subs, t.baseline.subclasses[i]) {
			continue
		}

		groupName := fmt.Sprintf("SubclassesForClass%d", i+1)
		var group *ASTBlock
		for _, node := range doc.Nodes {
			if b, ok := node.(*ASTBlock); ok && b.NameToken != nil &&
				strings.EqualFold(unquote(b.NameToken.Text), groupName) {
				group = b
				break
			}
		}
		if group == nil {
			if len(subs) == 0 {
				continue
			}
			group = &ASTBlock{
				NameToken:  &ASTToken{Type: TokenString, Text: groupName},
				Preamble:   []ASTNode{&ASTToken{Type: TokenWhitespace, Text: "\n\n"}},
				OpenBrace:  &ASTToken{Type: TokenBraceOpen, Text: "{"},
				Children:   []ASTNode{&ASTToken{Type: TokenWhitespace, Text: "\n"}},
				CloseBrace: &ASTToken{Type: TokenBraceClose, Text: "}"},
			}
			doc.Nodes = append(doc.Nodes, group)
		}

		// Preserve the placement of retained slots, but remove every
		// occurrence beyond the new contiguous extent (including authored
		// Subclass42+ and duplicates). When cleared this removes all
		// modeled subclass entries from the effective group.
		removeKeys := make(map[string]bool)
		walkSGPVPairs(group.Children, func(_ int, key string, _ int, _ string, hasValue bool) bool {
			if !hasValue {
				return true
			}
			lower := strings.ToLower(key)
			if !strings.HasPrefix(lower, "subclass") {
				return true
			}
			slot, err := strconv.Atoi(strings.TrimPrefix(lower, "subclass"))
			if err == nil && slot > len(subs) {
				removeKeys[lower] = true
			}
			return true
		})
		for key := range removeKeys {
			removeEngineBlockField(group, key)
		}
		for slot, sub := range subs {
			setEngineBlockField(group, fmt.Sprintf("Subclass%d", slot+1), sub, strings.ContainsAny(sub, " \t"))
		}
	}
}

func (t *MBTCTeam) hasClasses() bool {
	for _, cls := range t.Classes {
		if cls != "" {
			return true
		}
	}
	return false
}

// Validate checks the current candidate against the file size, required
// values, contiguous slots, and representability rules enforced by
// BG_SiegeParseTeamFile.
func (t *MBTCTeam) Validate() []string {
	var issues []string
	badChars := func(label, value string) {
		if strings.ContainsAny(value, "\"\x00") {
			issues = append(issues, fmt.Sprintf("%s contains a quote or NUL — the SGPV consumer has no escape syntax", label))
		}
		if strings.Contains(value, "//") {
			issues = append(issues, fmt.Sprintf("%s contains \"//\" — the engine would truncate it as a comment", label))
		}
	}

	if strings.TrimSpace(t.Name) == "" {
		issues = append(issues, "name is required — the engine drops the team with ERR_DROP (bg_saga.c:3270)")
	} else {
		badChars("name", t.Name)
	}
	if !t.hasClasses() {
		issues = append(issues, "at least one class slot must be filled — the engine drops teams with no allowable classes (bg_saga.c:3312)")
	}
	if t.baseline != nil && t.Classes == t.baseline.classes {
		issues = append(issues, t.baseline.classIssues...)
	}

	firstGap := -1
	for i, cls := range t.Classes {
		if cls == "" {
			if firstGap < 0 {
				firstGap = i
			}
			continue
		}
		badChars(fmt.Sprintf("class%d", i+1), cls)
		if firstGap >= 0 {
			issues = append(issues, fmt.Sprintf("class%d is empty but class%d is set — the engine stops reading Classes at the first gap, so class%d will not load", firstGap+1, i+1, i+1))
		}
	}

	for classIdx, subs := range t.Subclasses {
		if len(subs) > MBTCMaxSubclasses {
			issues = append(issues, fmt.Sprintf("SubclassesForClass%d has %d entries — the engine reads at most %d", classIdx+1, len(subs), MBTCMaxSubclasses))
		}
		firstSubGap := -1
		for subIdx, sub := range subs {
			if sub == "" {
				if firstSubGap < 0 {
					firstSubGap = subIdx
				}
				continue
			}
			badChars(fmt.Sprintf("SubclassesForClass%d Subclass%d", classIdx+1, subIdx+1), sub)
			if firstSubGap >= 0 {
				issues = append(issues, fmt.Sprintf("SubclassesForClass%d: Subclass%d is empty but Subclass%d is set — the engine stops at the first gap",
					classIdx+1, firstSubGap+1, subIdx+1))
			}
		}
		if t.baseline != nil && stringSlicesEqual(subs, t.baseline.subclasses[classIdx]) {
			issues = append(issues, t.baseline.subclassIssues[classIdx]...)
		}
	}
	candidate := t.Serialize()
	issues = append(issues, LexDiagnostics(candidate)...)
	tokens, _ := Lex(candidate)
	doc := parseAST(tokens)
	var checkBlocks func([]ASTNode)
	checkBlocks = func(nodes []ASTNode) {
		for _, node := range nodes {
			switch n := node.(type) {
			case *ASTBlock:
				if n.CloseBrace == nil {
					issues = append(issues, fmt.Sprintf("group %s is missing its closing brace", unquote(n.NameToken.Text)))
				}
				checkBlocks(n.Children)
			case *ASTToken:
				if n.Type == TokenBraceOpen || n.Type == TokenBraceClose {
					issues = append(issues, "stray brace outside a named group")
				}
			}
		}
	}
	checkBlocks(doc.Nodes)
	if size := len(candidate); size >= MBTCMaxFileBytes {
		issues = append(issues, fmt.Sprintf("team file is %d bytes — the engine rejects files of %d bytes or more (bg_saga.c:3261)", size, MBTCMaxFileBytes))
	}
	return issues
}

// ─────────────────────────────────────────────────────────────────────
// Canonical bulk field editing.
//
// EditConfigField replaces the first-effective occurrence of a key using
// each format's real document model (shared AST in this package), so a
// bulk edit can never truncate a quoted multiword value, strip inline
// comments, renumber or drop sibling definitions, or insert at the wrong
// scope. The result carries the re-serialized document, which the caller
// validates (ValidateSerialized) before any write.
// ─────────────────────────────────────────────────────────────────────

// ConfigFormat selects the document model used for a bulk edit.
type ConfigFormat int

const (
	FormatMBCH ConfigFormat = iota
	FormatSAB
	FormatVEH
	FormatSIEGE
	FormatMBTC
)

// FormatFromPath maps a file extension to its document model.
func FormatFromPath(path string) (ConfigFormat, bool) {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".mbch":
		return FormatMBCH, true
	case ".sab":
		return FormatSAB, true
	case ".veh":
		return FormatVEH, true
	case ".siege":
		return FormatSIEGE, true
	case ".mbtc":
		return FormatMBTC, true
	}
	return 0, false
}

// FieldEditResult describes one planned field edit.
type FieldEditResult struct {
	OldValue string // "" when the key was absent
	Found    bool
	Scope    string // human-readable effective location
	// Definitions counts top-level definitions in the source document
	// (SAB/VEH hold one per block; MBCH is 1). Applied edits touch only
	// the first definition; siblings pass through untouched.
	Definitions int
	Content     string // re-serialized document
}

// EditConfigField plans and applies key=value to source according to the
// format's effective-placement rules:
//
//   - MBCH: fields live in the first ClassInfo group (the engine extracts
//     it once via BG_SiegeGetValueGroup and reads everything else from
//     there); only "description" is a top-level key (bg_saga.c:2382).
//     Keys that exist only inside WeaponInfo/ForceInfo overrides are
//     refused — per-weapon edits belong to the override editors.
//   - SAB/VEH: the first top-level definition; sibling definitions and
//     comments are preserved untouched.
//   - SIEGE: top-level paired values first (file-level SGPV skips
//     groups), then the first occurrence inside any group, replaced in
//     place so its scope is preserved. Missing keys are refused — a
//     .siege key without its owning group is meaningless to the engine.
//   - MBTC: the canonical MBTCTeam model.
func EditConfigField(format ConfigFormat, source, key, value string) (*FieldEditResult, error) {
	if err := validateFieldValue(value); err != nil {
		return nil, err
	}
	key = strings.TrimSpace(key)
	if key == "" {
		return nil, fmt.Errorf("empty field key")
	}

	switch format {
	case FormatMBCH:
		return editMBCHField(source, key, value)
	case FormatSAB:
		return editFirstDefinitionField(source, key, value, "saber")
	case FormatVEH:
		return editFirstDefinitionField(source, key, value, "vehicle")
	case FormatSIEGE:
		return editSiegeField(source, key, value)
	case FormatMBTC:
		return editMBTCField(source, key, value)
	}
	return nil, fmt.Errorf("unsupported format %d", format)
}

// validateFieldValue rejects values the SGPV consumer cannot represent:
// quotes (no escape support) and inline comments (terminate the value).
func validateFieldValue(value string) error {
	if strings.Contains(value, "\"") {
		return fmt.Errorf("value contains a double quote — the engine has no escape syntax, quoted values end at the next quote")
	}
	if strings.Contains(value, "//") {
		return fmt.Errorf("value contains \"//\" — the engine treats it as a comment and would truncate the value")
	}
	if strings.ContainsAny(value, "\n\r") {
		return fmt.Errorf("value contains a line break — bulk values must be single-line")
	}
	return nil
}

// ── MBCH ─────────────────────────────────────────────────────────────

func editMBCHField(source, key, value string) (*FieldEditResult, error) {
	tokens, _ := Lex(source)
	doc := parseAST(tokens)

	keyLower := strings.ToLower(key)
	res := &FieldEditResult{Definitions: 1}

	if keyLower == "description" {
		old, found := getDocTopLevelField(doc, "description")
		res.OldValue, res.Found, res.Scope = old, found, "top level"
		setDocTopLevelField(doc, "description", value)
		res.Content = doc.String()
		return res, nil
	}

	var classInfo *ASTBlock
	for _, node := range doc.Nodes {
		if b, ok := node.(*ASTBlock); ok && b.NameToken != nil &&
			strings.ToLower(unquote(b.NameToken.Text)) == "classinfo" {
			classInfo = b
			break
		}
	}
	if classInfo == nil {
		if where, ok := findOverrideScope(doc, keyLower); ok {
			return nil, fmt.Errorf("key %q only exists inside %s — per-override edits belong to the Weapon/Force editor, bulk would be ambiguous", key, where)
		}
		return nil, fmt.Errorf("no ClassInfo group in document — cannot place %q", key)
	}

	res.OldValue, res.Found = getEngineBlockField(classInfo, key)
	res.Scope = "ClassInfo"

	if !res.Found {
		if where, ok := findOverrideScope(doc, keyLower); ok {
			return nil, fmt.Errorf("key %q only exists inside %s — per-override edits belong to the Weapon/Force editor, bulk would be ambiguous", key, where)
		}
	}

	setFieldPreservingStyle(classInfo, key, value)
	res.Content = doc.String()
	return res, nil
}

// findOverrideScope reports whether key appears inside a WeaponInfo/
// ForceInfo override block, naming the first such block.
func findOverrideScope(doc *ASTDocument, keyLower string) (string, bool) {
	for _, node := range doc.Nodes {
		b, ok := node.(*ASTBlock)
		if !ok || b.NameToken == nil {
			continue
		}
		name := strings.ToLower(unquote(b.NameToken.Text))
		if strings.HasPrefix(name, "weaponinfo") || strings.HasPrefix(name, "forceinfo") {
			if _, found := getEngineBlockField(b, keyLower); found {
				return unquote(b.NameToken.Text), true
			}
		}
	}
	return "", false
}

// ── SAB / VEH (first definition) ─────────────────────────────────────

func editFirstDefinitionField(source, key, value, kind string) (*FieldEditResult, error) {
	tokens, _ := Lex(source)
	doc := parseAST(tokens)

	var blocks []*ASTBlock
	for _, node := range doc.Nodes {
		if b, ok := node.(*ASTBlock); ok && b.NameToken != nil {
			blocks = append(blocks, b)
		}
	}
	if len(blocks) == 0 {
		return nil, fmt.Errorf("no %s definition in document", kind)
	}

	first := blocks[0]
	res := &FieldEditResult{Definitions: len(blocks)}
	res.OldValue, res.Found = getEngineBlockField(first, key)
	name := unquote(first.NameToken.Text)
	if res.Definitions > 1 {
		res.Scope = fmt.Sprintf("definition 1 of %d (%s)", res.Definitions, name)
	} else {
		res.Scope = fmt.Sprintf("definition (%s)", name)
	}
	setFieldPreservingStyle(first, key, value)
	res.Content = doc.String()
	return res, nil
}

// ── SIEGE ────────────────────────────────────────────────────────────

func editSiegeField(source, key, value string) (*FieldEditResult, error) {
	tokens, _ := Lex(source)
	doc := parseAST(tokens)
	res := &FieldEditResult{Definitions: 1}

	old, found := getDocTopLevelField(doc, key)
	if found {
		res.OldValue, res.Found, res.Scope = old, true, "top level"
		setDocTopLevelField(doc, key, value)
		res.Content = doc.String()
		return res, nil
	}

	// First group occurrence in document order — replace in place so the
	// key keeps its owning scope (team block, objective, ...).
	if b, path := findGroupedField(doc, strings.ToLower(key)); b != nil {
		res.OldValue, res.Found = getEngineBlockField(b, key)
		res.Scope = "group " + path
		setFieldPreservingStyle(b, key, value)
		res.Content = doc.String()
		return res, nil
	}

	return nil, fmt.Errorf("key %q not found — .siege fields live inside their owning group (team/objective); add them in the Siege Editor instead of bulk-inserting at file scope", key)
}

// findGroupedField locates the first block (depth-first) containing key.
func findGroupedField(doc *ASTDocument, keyLower string) (*ASTBlock, string) {
	var walk func(nodes []ASTNode, prefix string) (*ASTBlock, string)
	walk = func(nodes []ASTNode, prefix string) (*ASTBlock, string) {
		for _, node := range nodes {
			b, ok := node.(*ASTBlock)
			if !ok || b.NameToken == nil {
				continue
			}
			name := unquote(b.NameToken.Text)
			path := name
			if prefix != "" {
				path = prefix + " > " + name
			}
			if _, found := getEngineBlockField(b, keyLower); found {
				return b, path
			}
			if hit, inner := walk(b.Children, path); hit != nil {
				return hit, inner
			}
		}
		return nil, ""
	}
	return walk(doc.Nodes, "")
}

// ── MBTC ─────────────────────────────────────────────────────────────

func editMBTCField(source, key, value string) (*FieldEditResult, error) {
	team := ParseMBTC(source) // parse resets the model
	res := &FieldEditResult{Definitions: 1, Scope: "team"}

	keyLower := strings.ToLower(strings.TrimSpace(key))
	switch keyLower {
	case "name":
		res.OldValue, res.Found = team.Name, team.Name != ""
		team.Name = value
	case "timeperiod":
		v, err := strconv.Atoi(strings.TrimSpace(value))
		if err != nil {
			return nil, fmt.Errorf("TimePeriod must be an integer, got %q", value)
		}
		if team.TimePeriodSet {
			res.OldValue, res.Found = strconv.Itoa(team.TimePeriod), true
		}
		team.TimePeriod, team.TimePeriodSet = v, true
	case "euallowed":
		v, err := strconv.Atoi(strings.TrimSpace(value))
		if err != nil {
			return nil, fmt.Errorf("EUAllowed must be an integer, got %q", value)
		}
		if team.EUAllowedSet {
			res.OldValue, res.Found = strconv.Itoa(team.EUAllowed), true
		}
		team.EUAllowed, team.EUAllowedSet = v, true
	case "friendlyshader":
		res.OldValue, res.Found = team.FriendlyShader, team.FriendlyShader != ""
		team.FriendlyShader = value
	case "classesallowed":
		v, err := strconv.Atoi(strings.TrimSpace(value))
		if err != nil {
			return nil, fmt.Errorf("ClassesAllowed must be an integer, got %q", value)
		}
		if team.ClassesAllowedSet {
			res.OldValue, res.Found = strconv.Itoa(team.ClassesAllowed), true
		}
		team.ClassesAllowed, team.ClassesAllowedSet = v, true
	default:
		if isClassKey(keyLower) {
			return nil, fmt.Errorf("class slots are edited per team (class1..class6 in the Team Composer) — bulk class renames would silently rewire rosters")
		}
		// Unmodeled field: replace in place, preserving order and style.
		replaced := false
		for i := range team.ExtraFields {
			if strings.ToLower(team.ExtraFields[i].Key) == keyLower {
				res.OldValue, res.Found = team.ExtraFields[i].Value, true
				team.ExtraFields[i].Value = value
				replaced = true
				break
			}
		}
		if !replaced {
			res.Found = false
			team.ExtraFields = append(team.ExtraFields, MBTCExtraField{
				Key:    key,
				Value:  value,
				Quoted: strings.ContainsAny(value, " \t"),
			})
		}
	}
	res.Content = team.Serialize()
	return res, nil
}

func isClassKey(keyLower string) bool {
	if !strings.HasPrefix(keyLower, "class") && !strings.HasPrefix(keyLower, "subclass") {
		return false
	}
	suffix := strings.TrimPrefix(strings.TrimPrefix(keyLower, "subclass"), "class")
	if suffix == "esallowed" { // classesallowed handled earlier
		return false
	}
	_, err := strconv.Atoi(suffix)
	return err == nil
}

// ── shared AST helpers ───────────────────────────────────────────────

// getEngineBlockField returns the first value SGPV can actually see:
// one pair per line, with `//` truncation and first-occurrence semantics.
func getEngineBlockField(block *ASTBlock, key string) (string, bool) {
	var value string
	found := false
	walkSGPVPairs(block.Children, func(_ int, candidate string, _ int, candidateValue string, hasValue bool) bool {
		if hasValue && strings.EqualFold(candidate, key) {
			value, found = candidateValue, true
			return false
		}
		return true
	})
	return value, found
}

func setEngineBlockField(block *ASTBlock, key, value string, useQuotes bool) {
	target := value
	if useQuotes {
		target = "\"" + value + "\""
	} else if value == "" {
		target = "\"\""
	}
	valueIdx := -1
	walkSGPVPairs(block.Children, func(_ int, candidate string, candidateValueIdx int, _ string, hasValue bool) bool {
		if hasValue && strings.EqualFold(candidate, key) {
			valueIdx = candidateValueIdx
			return false
		}
		return true
	})
	if valueIdx >= 0 {
		if token, ok := block.Children[valueIdx].(*ASTToken); ok &&
			truncateSGPV(unquote(token.Text)) != value {
			token.Text = target
		}
		return
	}
	if len(block.Children) > 0 {
		if whitespace, ok := block.Children[len(block.Children)-1].(*ASTToken); !ok ||
			whitespace.Type != TokenWhitespace || !strings.HasSuffix(whitespace.Text, "\n") {
			block.Children = append(block.Children, &ASTToken{Type: TokenWhitespace, Text: "\n"})
		}
	}
	block.Children = append(block.Children,
		&ASTToken{Type: TokenWhitespace, Text: "\t"},
		&ASTToken{Type: TokenString, Text: key},
		&ASTToken{Type: TokenWhitespace, Text: "\t\t"},
		&ASTToken{Type: TokenString, Text: target},
		&ASTToken{Type: TokenWhitespace, Text: "\n"},
	)
}

func removeEngineBlockField(block *ASTBlock, key string) {
	for {
		keyIdx, valueIdx := -1, -1
		walkSGPVPairs(block.Children, func(candidateKeyIdx int, candidate string, candidateValueIdx int, _ string, hasValue bool) bool {
			if hasValue && strings.EqualFold(candidate, key) {
				keyIdx, valueIdx = candidateKeyIdx, candidateValueIdx
				return false
			}
			return true
		})
		if keyIdx < 0 {
			return
		}
		block.Children = append(block.Children[:keyIdx], block.Children[valueIdx+1:]...)
	}
}

func setFieldPreservingStyle(block *ASTBlock, key, value string) {
	setEngineBlockField(block, key, value, strings.ContainsAny(value, " \t"))
}

// getDocTopLevelField returns the first top-level paired value.
func getDocTopLevelField(doc *ASTDocument, key string) (string, bool) {
	var value string
	found := false
	walkSGPVPairs(doc.Nodes, func(_ int, candidate string, _ int, candidateValue string, hasValue bool) bool {
		if hasValue && strings.EqualFold(candidate, key) {
			value, found = candidateValue, true
			return false
		}
		return true
	})
	return value, found
}

// setDocTopLevelField rewrites the first top-level occurrence of key, or
// appends one. A value token already holding the target text is never
// rewritten, so unchanged fields render byte-identically.
func setDocTopLevelField(doc *ASTDocument, key, value string) {
	target := quoteFor(value)
	valueIdx := -1
	walkSGPVPairs(doc.Nodes, func(_ int, candidate string, candidateValueIdx int, _ string, hasValue bool) bool {
		if hasValue && strings.EqualFold(candidate, key) {
			valueIdx = candidateValueIdx
			return false
		}
		return true
	})
	if valueIdx >= 0 {
		if valueToken, ok := doc.Nodes[valueIdx].(*ASTToken); ok &&
			truncateSGPV(unquote(valueToken.Text)) != value {
			valueToken.Text = target
		}
		return
	}
	doc.Nodes = append(doc.Nodes,
		&ASTToken{Type: TokenWhitespace, Text: "\n"},
		&ASTToken{Type: TokenString, Text: key},
		&ASTToken{Type: TokenWhitespace, Text: "\t"},
		&ASTToken{Type: TokenString, Text: target},
		&ASTToken{Type: TokenWhitespace, Text: "\n"},
	)
}

// removeDocTopLevelField deletes every real top-level pair for key. SGPV
// uses the first occurrence, so removing only that occurrence would make
// a previously dead duplicate unexpectedly effective.
func removeDocTopLevelField(doc *ASTDocument, key string) {
	for {
		keyIdx, valueIdx := -1, -1
		walkSGPVPairs(doc.Nodes, func(candidateKeyIdx int, candidate string, candidateValueIdx int, _ string, hasValue bool) bool {
			if hasValue && strings.EqualFold(candidate, key) {
				keyIdx, valueIdx = candidateKeyIdx, candidateValueIdx
				return false
			}
			return true
		})
		if keyIdx < 0 {
			return
		}
		doc.Nodes = append(doc.Nodes[:keyIdx], doc.Nodes[valueIdx+1:]...)
	}
}

func quoteFor(value string) string {
	if strings.ContainsAny(value, " \t") {
		return fmt.Sprintf("\"%s\"", value)
	}
	return value
}

// ValidateSerialized re-parses content with the format's canonical
// consumer and confirms the edited key now holds value. Beyond the AST
// echo it enforces consumer semantics: engine buffer budgets for MBCH
// (bg_saga.h:165-171 via AssessMBCHSourceBuffers), team-file size and
// roster rules for MBTC (bg_saga.c:3252-3332), and definition sanity
// for SAB/VEH mirroring what the editors and engine require. Call this
// before any write; a failure means the game would reject or misread
// the file.
func ValidateSerialized(format ConfigFormat, content, key, value string) error {
	switch format {
	case FormatMBCH:
		return validateMBCHSerialized(content, key, value)
	case FormatSAB:
		if err := validateFirstDefinitionSerialized(content, key, value); err != nil {
			return err
		}
		return validateSABSemantics(content)
	case FormatVEH:
		if err := validateFirstDefinitionSerialized(content, key, value); err != nil {
			return err
		}
		return validateVEHSemantics(content)
	case FormatSIEGE:
		tokens, _ := Lex(content)
		doc := parseAST(tokens)
		if old, found := getDocTopLevelField(doc, key); found {
			if old != value {
				return fmt.Errorf("validation failed: %q is %q after edit", key, old)
			}
			return nil
		}
		if b, _ := findGroupedField(doc, strings.ToLower(key)); b != nil {
			if old, _ := getEngineBlockField(b, key); old != value {
				return fmt.Errorf("validation failed: %q is %q after edit", key, old)
			}
			return nil
		}
		return fmt.Errorf("validation failed: %q missing after edit", key)
	case FormatMBTC:
		team := ParseMBTC(content)
		keyLower := strings.ToLower(key)
		var got string
		var ok bool
		switch keyLower {
		case "name":
			got, ok = team.Name, true
		case "timeperiod":
			got, ok = strconv.Itoa(team.TimePeriod), team.TimePeriodSet
		case "euallowed":
			got, ok = strconv.Itoa(team.EUAllowed), team.EUAllowedSet
		case "friendlyshader":
			got, ok = team.FriendlyShader, team.FriendlyShader != ""
		case "classesallowed":
			got, ok = strconv.Itoa(team.ClassesAllowed), team.ClassesAllowedSet
		default:
			for _, f := range team.ExtraFields {
				if strings.ToLower(f.Key) == keyLower {
					got, ok = f.Value, true
					break
				}
			}
		}
		if !ok || got != value {
			return fmt.Errorf("validation failed: %q is %q after edit", key, got)
		}
		if len(content) >= MBTCMaxFileBytes {
			return fmt.Errorf("team file %d bytes — the engine rejects files of %d bytes or more (bg_saga.c:3261)", len(content), MBTCMaxFileBytes)
		}
		if issues := team.Validate(); len(issues) > 0 {
			return fmt.Errorf("team rules: %s", strings.Join(issues, "; "))
		}
		return nil
	}
	return fmt.Errorf("unsupported format %d", format)
}

func validateMBCHSerialized(content, key, value string) error {
	char, err := ParseMBCH(content)
	if err != nil {
		return fmt.Errorf("re-parse failed: %w", err)
	}
	_ = char

	tokens, _ := Lex(content)
	doc := parseAST(tokens)
	keyLower := strings.ToLower(key)
	var got string
	var ok bool
	if keyLower == "description" {
		got, ok = getDocTopLevelField(doc, "description")
	} else {
		for _, node := range doc.Nodes {
			if b, isBlock := node.(*ASTBlock); isBlock && b.NameToken != nil &&
				strings.ToLower(unquote(b.NameToken.Text)) == "classinfo" {
				got, ok = getEngineBlockField(b, key)
				break
			}
		}
	}
	if !ok || got != value {
		return fmt.Errorf("validation failed: %q is %q after edit", key, got)
	}

	bs, err := AssessMBCHSourceBuffers(content)
	if err != nil {
		return err
	}
	var problems []string
	if bs.TotalFileBytes >= MBCHMaxFileBytes {
		problems = append(problems, fmt.Sprintf("file %d/%d bytes (engine rejects at >=%d)", bs.TotalFileBytes, MBCHMaxFileBytes-1, MBCHMaxFileBytes))
	}
	if bs.ClassInfoBytes > ClassInfoMaxPayload {
		problems = append(problems, fmt.Sprintf("ClassInfo %d/%d bytes", bs.ClassInfoBytes, ClassInfoMaxPayload))
	}
	for i, w := range bs.WeaponInfoBytes {
		if w > WeaponInfoMaxPayload {
			problems = append(problems, fmt.Sprintf("WeaponInfo#%d %d/%d bytes", i, w, WeaponInfoMaxPayload))
		}
	}
	for i, f := range bs.ForceInfoBytes {
		if f > ForceInfoMaxBytes-1 {
			problems = append(problems, fmt.Sprintf("ForceInfo#%d %d/%d bytes", i, f, ForceInfoMaxBytes-1))
		}
	}
	if bs.MaxPairedValueBytes > PairedValueMaxPayload {
		problems = append(problems, fmt.Sprintf("paired value %d/%d bytes", bs.MaxPairedValueBytes, PairedValueMaxPayload))
	}
	// Lexical diagnostics are the single responsibility of
	// AssessMBCHSourceBuffers (assessment.go); they are deliberately NOT
	// duplicated here. Budget enforcement above uses the shared
	// constants from limits.go.
	if len(problems) > 0 {
		return fmt.Errorf("engine budget check: %s", strings.Join(problems, "; "))
	}
	return nil
}

func validateFirstDefinitionSerialized(content, key, value string) error {
	tokens, _ := Lex(content)
	doc := parseAST(tokens)
	for _, node := range doc.Nodes {
		b, ok := node.(*ASTBlock)
		if !ok || b.NameToken == nil {
			continue
		}
		if old, found := getEngineBlockField(b, key); !found || old != value {
			return fmt.Errorf("validation failed: first definition has %q = %q (found=%v) after edit", key, old, found)
		}
		return nil
	}
	return fmt.Errorf("validation failed: no definition after edit")
}

// validateSABSemantics checks what a saber consumer requires beyond the
// edited key: a real blade layout and a saber type (mirrors the editor's
// own validation and BG_ParseSaberFile expectations).
func validateSABSemantics(content string) error {
	saber, err := ParseSAB(content)
	if err != nil {
		return fmt.Errorf("re-parse failed: %w", err)
	}
	if saber.NumBlades < 1 {
		return fmt.Errorf("semantic validation: numBlades %d — a saber needs at least one blade", saber.NumBlades)
	}
	if len(saber.Blades) == 0 {
		return fmt.Errorf("semantic validation: no blade configuration")
	}
	if saber.SaberType == "" {
		return fmt.Errorf("semantic validation: saberType is required")
	}
	return nil
}

// validateVEHSemantics checks what a vehicle consumer requires beyond
// the edited key: identity fields a nameless vehicle cannot carry.
func validateVEHSemantics(content string) error {
	veh, err := ParseVEH(content)
	if err != nil {
		return fmt.Errorf("re-parse failed: %w", err)
	}
	if veh.Name == "" {
		return fmt.Errorf("semantic validation: vehicle name is required")
	}
	if veh.Type == "" {
		return fmt.Errorf("semantic validation: vehicle type is required")
	}
	return nil
}
