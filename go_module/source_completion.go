package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/Frenzeh/mbii-foundry/parsers"
)

type sourceFieldKind uint8

const (
	sourceFieldText sourceFieldKind = iota
	sourceFieldAsset
	sourceFieldClass
	sourceFieldWeapon
	sourceFieldAttribute
	sourceFieldForcePower
	sourceFieldClassFlag
	sourceFieldSaberStyle
	sourceFieldHeldFlag
	sourceFieldSpecial
	sourceFieldHoldable
	sourceFieldPowerup
	sourceFieldBoolean
	sourceFieldVehicleType
)

type sourceFieldSpec struct {
	name   string
	detail string
	kind   sourceFieldKind
	list   bool
	ranked bool
	enums  []string
}

type sourceScopeSpecs struct {
	fields   []sourceFieldSpec
	patterns []*regexp.Regexp
}

func sourceCompletions(language *SourceLanguage, text string, caret int, format SourceFormat) []SourceCompletion {
	if !sourceFormatSupported(format) || sourceCaretInComment(text, caret) {
		return nil
	}
	caret = sourceClampOffset(text, caret)
	context := SourceContextAt(text, caret, format)
	specs := sourceSpecsFor(format, context.Block)
	if context.AtKey {
		items := make([]SourceCompletion, 0, len(specs.fields))
		for _, field := range specs.fields {
			if !sourcePrefixMatch(field.name, context.Prefix) {
				continue
			}
			items = append(items, SourceCompletion{
				Label: field.name, InsertText: field.name, Detail: field.detail, Replace: context.Replace,
			})
		}
		return sourceSortCompletions(items)
	}
	if context.Field == "" {
		return nil
	}
	spec, ok := sourceFindFieldSpec(specs, context.Field)
	if !ok {
		return nil
	}

	if spec.ranked && sourceCaretIsRank(text, caret, context.Field) {
		return sourceRankCompletions(text, caret, context, spec)
	}

	values := sourceValuesForSpec(spec, false)
	if spec.kind == sourceFieldAsset && language != nil && language.assetSuggest != nil {
		values = language.assetSuggest(format, context.Field, context.Prefix, 50)
	}
	items := make([]SourceCompletion, 0, len(values))
	seen := make(map[string]bool, len(values))
	for _, value := range values {
		if value == "" || seen[strings.ToLower(value)] || !sourcePrefixMatch(value, context.Prefix) {
			continue
		}
		seen[strings.ToLower(value)] = true
		insert := value
		if spec.kind == sourceFieldAsset && !context.InString {
			insert = `"` + value + `"`
		}
		items = append(items, SourceCompletion{
			Label: value, InsertText: insert, Detail: sourceValueDetail(spec, value), Replace: context.Replace,
		})
	}
	return sourceSortCompletions(items)
}

func sourceRankCompletions(text string, caret int, context SourceContext, spec sourceFieldSpec) []SourceCompletion {
	identifier := sourceListIdentifierBeforeRank(text, caret)
	maximum := 0
	detail := "Rank"
	switch spec.kind {
	case sourceFieldAttribute:
		for _, attribute := range GetAllAttributes() {
			if strings.EqualFold(attribute.ID, identifier) {
				maximum = attribute.MaxLevel
				detail = "Rank for " + attribute.Name
				break
			}
		}
	case sourceFieldForcePower:
		maximum = 3
		detail = "Force power rank"
	}
	if maximum <= 0 {
		return nil // The runtime catalog does not establish this rank bound.
	}
	items := make([]SourceCompletion, 0, maximum+1)
	for rank := 0; rank <= maximum; rank++ {
		value := strconv.Itoa(rank)
		if sourcePrefixMatch(value, context.Prefix) {
			items = append(items, SourceCompletion{Label: value, InsertText: value, Detail: detail, Replace: context.Replace})
		}
	}
	return items
}

func sourceValuesForSpec(spec sourceFieldSpec, all bool) []string {
	switch spec.kind {
	case sourceFieldClass:
		classes := GetClasses()
		if all {
			classes = GetAllClasses()
		}
		values := make([]string, 0, len(classes))
		for _, class := range classes {
			values = append(values, class.ID)
		}
		return values
	case sourceFieldForcePower:
		return append([]string(nil), KnownForcePowerTargets...)
	case sourceFieldWeapon:
		weapons := GetWeapons()
		if all {
			weapons = GetAllWeapons()
		}
		values := make([]string, 0, len(weapons))
		for _, weapon := range weapons {
			values = append(values, weapon.ID)
		}
		return values
	case sourceFieldAttribute:
		attributes := GetAttributes()
		if all {
			attributes = GetAllAttributes()
		}
		values := make([]string, 0, len(attributes))
		for _, attribute := range attributes {
			values = append(values, attribute.ID)
		}
		return values
	case sourceFieldClassFlag:
		values := make([]string, 0, len(GetClassFlags()))
		for _, flag := range GetClassFlags() {
			values = append(values, flag.ID)
		}
		return values
	case sourceFieldSaberStyle:
		values := make([]string, 0, len(GetSaberStyles()))
		for _, style := range GetSaberStyles() {
			values = append(values, style.ID)
		}
		return values
	case sourceFieldHeldFlag:
		values := make([]string, 0, len(KnownHeldFlags))
		for _, flag := range KnownHeldFlags {
			values = append(values, flag.ID)
		}
		return values
	case sourceFieldSpecial:
		return append([]string(nil), KnownSpecialActions...)
	case sourceFieldHoldable:
		values := make([]string, 0, len(KnownHoldables))
		for _, holdable := range KnownHoldables {
			values = append(values, holdable.ID)
		}
		return values
	case sourceFieldBoolean:
		return []string{"0", "1"}
	case sourceFieldVehicleType:
		return []string{"VH_FIGHTER", "VH_SPEEDER", "VH_WALKER"}
	case sourceFieldPowerup:
		return append([]string(nil), sourceSchemaEnums[sourceFieldPowerup]...)
	}
	return append([]string(nil), spec.enums...)
}

func sourceValueDetail(spec sourceFieldSpec, value string) string {
	switch spec.kind {
	case sourceFieldClass:
		for _, class := range GetAllClasses() {
			if strings.EqualFold(class.ID, value) {
				return sourceFirstNonEmpty(class.Description, class.Name, spec.detail)
			}
		}
	case sourceFieldWeapon:
		for _, weapon := range GetAllWeapons() {
			if strings.EqualFold(weapon.ID, value) {
				return sourceFirstNonEmpty(weapon.Description, weapon.Name, spec.detail)
			}
		}
	case sourceFieldAttribute:
		for _, attribute := range GetAllAttributes() {
			if strings.EqualFold(attribute.ID, value) {
				return sourceFirstNonEmpty(attribute.Description, attribute.Name, spec.detail)
			}
		}
	case sourceFieldClassFlag:
		for _, flag := range GetClassFlags() {
			if strings.EqualFold(flag.ID, value) {
				return sourceFirstNonEmpty(flag.Description, flag.Name, spec.detail)
			}
		}
	case sourceFieldSaberStyle:
		for _, style := range GetSaberStyles() {
			if strings.EqualFold(style.ID, value) {
				return sourceFirstNonEmpty(style.Description, style.Name, spec.detail)
			}
		}
	case sourceFieldHeldFlag:
		for _, flag := range KnownHeldFlags {
			if strings.EqualFold(flag.ID, value) {
				return sourceFirstNonEmpty(flag.Tooltip, flag.Name, spec.detail)
			}
		}
	}
	if summary := sourceDefinitionSummary(value); summary != "" {
		return summary
	}
	return spec.detail
}

func sourceFirstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func sourceDefinitionSummary(key string) string {
	definition, ok := GetDefinition(key)
	if !ok {
		return ""
	}
	for _, line := range strings.Split(definition, "\n") {
		line = strings.TrimSpace(strings.TrimLeft(line, "#"))
		if line == "" || strings.HasPrefix(line, "---") {
			continue
		}
		if len(line) > 180 {
			line = line[:177] + "..."
		}
		return line
	}
	return ""
}

func sourceSortCompletions(items []SourceCompletion) []SourceCompletion {
	sort.SliceStable(items, func(i, j int) bool {
		return strings.ToLower(items[i].Label) < strings.ToLower(items[j].Label)
	})
	return items
}

func sourcePrefixMatch(value, prefix string) bool {
	return strings.HasPrefix(strings.ToLower(value), strings.ToLower(strings.TrimSpace(prefix)))
}

func sourceCaretInComment(text string, caret int) bool {
	caret = sourceClampOffset(text, caret)
	for _, token := range sourceLex(text) {
		if token.kind == sourceLexComment && caret >= token.start && caret < token.end {
			return true
		}
	}
	return false
}

func sourceCaretIsRank(text string, caret int, field string) bool {
	if !sourceRankedField(field) {
		return false
	}
	caret = sourceClampOffset(text, caret)
	lineStart := strings.LastIndex(text[:caret], "\n") + 1
	segment := text[lineStart:caret]
	if pipe := strings.LastIndex(segment, "|"); pipe >= 0 {
		segment = segment[pipe+1:]
	}
	return strings.Contains(segment, ",")
}

func sourceListIdentifierBeforeRank(text string, caret int) string {
	caret = sourceClampOffset(text, caret)
	lineStart := strings.LastIndex(text[:caret], "\n") + 1
	segment := text[lineStart:caret]
	if pipe := strings.LastIndex(segment, "|"); pipe >= 0 {
		segment = segment[pipe+1:]
	}
	if comma := strings.LastIndex(segment, ","); comma >= 0 {
		segment = segment[:comma]
	}
	fields := strings.Fields(segment)
	if len(fields) == 0 {
		return ""
	}
	return strings.Trim(fields[len(fields)-1], `"`)
}

func sourceFindFieldSpec(specs sourceScopeSpecs, field string) (sourceFieldSpec, bool) {
	for _, spec := range specs.fields {
		if strings.EqualFold(spec.name, field) {
			return spec, true
		}
	}
	for _, pattern := range specs.patterns {
		if pattern.MatchString(field) {
			return sourcePatternFieldSpec(field), true
		}
	}
	return sourceFieldSpec{}, false
}

func sourcePatternFieldSpec(field string) sourceFieldSpec {
	lower := strings.ToLower(field)
	spec := sourceFieldSpec{name: field, detail: "Supported patterned field"}
	switch {
	case strings.HasPrefix(lower, "wp_") && strings.HasSuffix(lower, "flags"):
		spec.kind, spec.list = sourceFieldHeldFlag, true
	case strings.HasPrefix(lower, "c_att_skill_"):
		spec.kind = sourceFieldAttribute
	case strings.HasPrefix(lower, "class"), strings.HasPrefix(lower, "subclass"):
		spec.kind = sourceFieldClass
	}
	return spec
}

func sourceSpecsFor(format SourceFormat, block string) sourceScopeSpecs {
	scope := sourceNormalizeScope(format, block)
	switch format {
	case SourceFormatMBCH:
		return sourceMBCHSpecs(scope)
	case SourceFormatSAB:
		if scope == "Saber" {
			return sourceSABSpecs()
		}
	case SourceFormatVEH:
		if scope == "Vehicle" {
			return sourceVEHSpecs()
		}
	case SourceFormatSiege:
		return sourceSiegeSpecs(scope)
	case SourceFormatMBTC:
		return sourceMBTCSpecs(scope)
	}
	return sourceScopeSpecs{}
}

func sourceNormalizeScope(format SourceFormat, block string) string {
	lower := strings.ToLower(block)
	switch format {
	case SourceFormatMBCH:
		switch {
		case lower == "classinfo":
			return "ClassInfo"
		case strings.HasPrefix(lower, "weaponinfo"):
			return "WeaponInfo"
		case strings.HasPrefix(lower, "forceinfo"):
			return "ForceInfo"
		}
		return "Global"
	case SourceFormatSAB:
		if block != "" {
			return "Saber"
		}
	case SourceFormatVEH:
		if block != "" {
			return "Vehicle"
		}
	case SourceFormatSiege:
		switch {
		case block == "":
			return "Global"
		case lower == "teams":
			return "Teams"
		case strings.HasPrefix(lower, "objective"):
			return "Objective"
		case lower == "helpicons" || lower == "automap" || lower == "levelshotdesc":
			return "Opaque"
		default:
			return "Team"
		}
	case SourceFormatMBTC:
		switch {
		case block == "":
			return "Global"
		case lower == "classes":
			return "Classes"
		case strings.HasPrefix(lower, "subclassesforclass"):
			return "Subclasses"
		default:
			return "Opaque"
		}
	}
	return "Global"
}

var (
	sourceSchemaOnce   sync.Once
	sourceSchemaDefs   map[string]sourceScopeSpecs
	sourceSchemaEnums  map[sourceFieldKind][]string
	sourceSchemaLoaded bool
)

func sourceMBCHSpecs(scope string) sourceScopeSpecs {
	sourceSchemaOnce.Do(loadSourceSchema)
	if specs, ok := sourceSchemaDefs[scope]; ok && len(specs.fields) > 0 {
		return specs
	}
	return sourceMBCHFallbackSpecs(scope)
}

func loadSourceSchema() {
	sourceSchemaDefs = make(map[string]sourceScopeSpecs)
	sourceSchemaEnums = make(map[sourceFieldKind][]string)
	schemaPath := sourceSchemaPath()
	data, err := os.ReadFile(schemaPath)
	if err != nil {
		return
	}
	var root struct {
		Definitions map[string]struct {
			Properties map[string]struct {
				Description string          `json:"description"`
				Enum        []string        `json:"enum"`
				Type        json.RawMessage `json:"type"`
			} `json:"properties"`
			PatternProperties map[string]json.RawMessage `json:"patternProperties"`
		} `json:"definitions"`
		Properties map[string]struct {
			Description string `json:"description"`
		} `json:"properties"`
	}
	if json.Unmarshal(data, &root) != nil {
		return
	}
	sourceSchemaLoaded = true
	for scope, definition := range root.Definitions {
		specs := sourceScopeSpecs{}
		for name, property := range definition.Properties {
			if strings.HasPrefix(name, "_section_") {
				continue
			}
			spec := sourceFieldSpec{name: name, detail: property.Description, enums: property.Enum}
			sourceApplyFieldSemantics(&spec)
			specs.fields = append(specs.fields, spec)
		}
		for pattern := range definition.PatternProperties {
			if compiled, compileErr := regexp.Compile(pattern); compileErr == nil {
				specs.patterns = append(specs.patterns, compiled)
			}
		}
		if scope == "ClassInfo" {
			specs.patterns = append(specs.patterns, regexp.MustCompile(`(?i)^WP_[A-Z0-9_]+Flags$`))
		}
		sort.Slice(specs.fields, func(i, j int) bool {
			return strings.ToLower(specs.fields[i].name) < strings.ToLower(specs.fields[j].name)
		})
		sourceSchemaDefs[scope] = specs
	}
	global := sourceScopeSpecs{fields: []sourceFieldSpec{
		{name: "ClassInfo", detail: "Main character definition block"},
		{name: "WeaponInfo0", detail: "Weapon override block"},
		{name: "ForceInfo0", detail: "Force power override block"},
		{name: "description", detail: root.Properties["description"].Description},
	}}
	sourceSchemaDefs["Global"] = global
	enumDir := filepath.Join(filepath.Dir(schemaPath), "enums")
	sourceSchemaEnums[sourceFieldPowerup] = sourceReadEnumKeys(filepath.Join(enumDir, "powerups.json"), "powerups")
}

func sourceReadEnumKeys(path, property string) []string {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var root map[string]json.RawMessage
	if json.Unmarshal(data, &root) != nil {
		return nil
	}
	var entries map[string]json.RawMessage
	if json.Unmarshal(root[property], &entries) != nil {
		return nil
	}
	values := make([]string, 0, len(entries))
	for value := range entries {
		upper := strings.ToUpper(value)
		if strings.Contains(upper, "_NUM_") || strings.HasSuffix(upper, "_MAX") || strings.HasSuffix(upper, "_FIRST") {
			continue
		}
		values = append(values, value)
	}
	sort.Strings(values)
	return values
}

func sourceSchemaPath() string {
	var candidates []string
	if cwd, err := os.Getwd(); err == nil {
		candidates = append(candidates,
			filepath.Join(cwd, "..", "schemas", "mbch_schema.json"),
			filepath.Join(cwd, "schemas", "mbch_schema.json"),
		)
	}
	if executable, err := os.Executable(); err == nil {
		dir := filepath.Dir(executable)
		candidates = append(candidates,
			filepath.Join(dir, "..", "schemas", "mbch_schema.json"),
			filepath.Join(dir, "..", "Resources", "schemas", "mbch_schema.json"),
			filepath.Join(dir, "schemas", "mbch_schema.json"),
		)
	}
	for _, candidate := range candidates {
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			return candidate
		}
	}
	return ""
}

func sourceApplyFieldSemantics(spec *sourceFieldSpec) {
	lower := strings.ToLower(spec.name)
	switch {
	case lower == "mbclass":
		spec.kind = sourceFieldClass
	case lower == "weapons" || lower == "weapontoreplace" || lower == "weaponbasedoff":
		spec.kind = sourceFieldWeapon
		spec.list = lower == "weapons"
	case lower == "attributes":
		spec.kind, spec.list, spec.ranked = sourceFieldAttribute, true, true
	case lower == "forcepowers":
		spec.kind, spec.list, spec.ranked = sourceFieldForcePower, true, true
	case lower == "forcetoreplace":
		spec.kind = sourceFieldForcePower
	case lower == "classflags":
		spec.kind, spec.list = sourceFieldClassFlag, true
	case lower == "saberstyle" || lower == "singlebladestyle":
		spec.kind, spec.list = sourceFieldSaberStyle, lower == "saberstyle"
	case lower == "holdables":
		spec.kind, spec.list = sourceFieldHoldable, true
	case lower == "powerups":
		spec.kind, spec.list = sourceFieldPowerup, true
	case lower == "special1" || lower == "special2" || lower == "special3" || lower == "special4":
		spec.kind = sourceFieldSpecial
	case strings.HasPrefix(lower, "wp_") && strings.HasSuffix(lower, "flags"):
		spec.kind, spec.list = sourceFieldHeldFlag, true
	case strings.HasPrefix(lower, "c_att_skill_"):
		spec.kind = sourceFieldAttribute
	case sourceLooksLikeAssetField(lower):
		spec.kind = sourceFieldAsset
	case strings.Contains(strings.ToLower(spec.detail), "(0/1)"):
		spec.kind = sourceFieldBoolean
	}
}

func sourceLooksLikeAssetField(lower string) bool {
	for _, fragment := range []string{"model", "skin", "shader", "icon", "sound", "effect", "mapgraphic", "useteam"} {
		if strings.Contains(lower, fragment) {
			return true
		}
	}
	return false
}

func sourceMBCHFallbackSpecs(scope string) sourceScopeSpecs {
	var fields []sourceFieldSpec
	switch scope {
	case "ClassInfo":
		fields = []sourceFieldSpec{
			{name: "name", detail: "Internal class name"},
			{name: "MBClass", detail: "Base class identifier", kind: sourceFieldClass},
			{name: "model", detail: "Player model", kind: sourceFieldAsset},
			{name: "skin", detail: "Player skin", kind: sourceFieldAsset},
			{name: "uishader", detail: "Portrait shader", kind: sourceFieldAsset},
			{name: "soundset", detail: "Sound set", kind: sourceFieldAsset},
			{name: "weapons", detail: "Pipe-separated weapons", kind: sourceFieldWeapon, list: true},
			{name: "attributes", detail: "Pipe-separated ranked attributes", kind: sourceFieldAttribute, list: true, ranked: true},
			{name: "forcepowers", detail: "Pipe-separated ranked powers", kind: sourceFieldForcePower, list: true, ranked: true},
			{name: "saberstyle", detail: "Pipe-separated saber styles", kind: sourceFieldSaberStyle, list: true},
			{name: "classflags", detail: "Pipe-separated class flags", kind: sourceFieldClassFlag, list: true},
		}
	case "WeaponInfo":
		fields = []sourceFieldSpec{
			{name: "WeaponToReplace", detail: "Weapon type to replace", kind: sourceFieldWeapon},
			{name: "WeaponBasedOff", detail: "Base weapon behavior", kind: sourceFieldWeapon},
			{name: "NewWorldModel", detail: "World model path", kind: sourceFieldAsset},
			{name: "NewViewModel", detail: "View model path", kind: sourceFieldAsset},
			{name: "Icon", detail: "HUD icon", kind: sourceFieldAsset},
		}
	case "ForceInfo":
		fields = []sourceFieldSpec{
			{name: "ForceToReplace", detail: "Force power to replace", kind: sourceFieldForcePower},
			{name: "Icon", detail: "Force power icon", kind: sourceFieldAsset},
			{name: "StartSound", detail: "Activation sound", kind: sourceFieldAsset},
			{name: "LoopSound", detail: "Loop sound", kind: sourceFieldAsset},
		}
	case "Global":
		fields = []sourceFieldSpec{{name: "ClassInfo", detail: "Main character block"}, {name: "description", detail: "Character description"}}
	}
	return sourceScopeSpecs{fields: fields, patterns: []*regexp.Regexp{regexp.MustCompile(`(?i)^WP_[A-Z0-9_]+Flags$`)}}
}

func sourceSABSpecs() sourceScopeSpecs {
	fields := []sourceFieldSpec{
		{name: "name", detail: "Display name"}, {name: "saberType", detail: "Saber type"},
		{name: "saberModel", detail: "Saber model", kind: sourceFieldAsset}, {name: "customSkin", detail: "Custom skin", kind: sourceFieldAsset},
		{name: "numBlades", detail: "Number of blades"}, {name: "saberColor", detail: "Blade color"},
		{name: "saberLength", detail: "Blade length"}, {name: "saberRadius", detail: "Blade radius"},
		{name: "saberStyle", detail: "Saber style", kind: sourceFieldSaberStyle}, {name: "singleBladeStyle", detail: "Single-blade style", kind: sourceFieldSaberStyle},
		{name: "maxChain", detail: "Maximum attack chain"}, {name: "lockBonus", detail: "Saber-lock bonus"},
		{name: "parryBonus", detail: "Parry bonus"}, {name: "breakParryBonus", detail: "Break-parry bonus"},
		{name: "disarmBonus", detail: "Disarm bonus"}, {name: "trailStyle", detail: "Blade trail style"},
		{name: "moveSpeedScale", detail: "Movement speed scale"}, {name: "animSpeedScale", detail: "Animation speed scale"},
		{name: "damageScale", detail: "Damage scale"}, {name: "knockbackScale", detail: "Knockback scale"},
	}
	for _, name := range []string{"soundOn", "soundOff", "soundLoop", "spinSound", "swingSound1", "swingSound2", "swingSound3", "fallSound1", "fallSound2", "fallSound3", "hitSound1", "hitSound2", "hitSound3", "blockSound1", "blockSound2", "blockSound3", "bounceSound1", "bounceSound2", "bounceSound3"} {
		fields = append(fields, sourceFieldSpec{name: name, detail: "Sound asset", kind: sourceFieldAsset})
	}
	for _, name := range []string{"noWallMarks", "noDlight", "noBlade", "noClashFlare", "noDismemberment", "noIdleEffect", "alwaysBlock", "noManualDeactivate", "transitionDamage", "notInOpen", "notInMP", "noCartwheels", "throwable", "disarmable", "blasterBlocking", "onInWater", "bounceOnWalls", "twoHanded", "useGoreConfig", "useGoreConfig2", "noDismemberment2", "noBladeEffects", "noBladeEffects2"} {
		fields = append(fields, sourceFieldSpec{name: name, detail: "Boolean saber flag", kind: sourceFieldBoolean})
	}
	for _, name := range []string{"blockEffect", "hitPersonEffect", "bladeEffect", "hitOtherEffect", "g2MarksShader", "g2WeaponMarkShader"} {
		fields = append(fields, sourceFieldSpec{name: name, detail: "Visual asset", kind: sourceFieldAsset})
	}
	for _, name := range []string{"slapAnim", "readyAnim", "jumpAtkUpMove", "jumpAtkFwdMove", "lungeAtkMove"} {
		fields = append(fields, sourceFieldSpec{name: name, detail: "Animation override"})
	}
	return sourceScopeSpecs{fields: fields, patterns: []*regexp.Regexp{regexp.MustCompile(`(?i)^saber(color|length|radius)[1-8]$`)}}
}

func sourceVEHSpecs() sourceScopeSpecs {
	return sourceScopeSpecs{fields: []sourceFieldSpec{
		{name: "name", detail: "Vehicle name"}, {name: "type", detail: "Vehicle type", kind: sourceFieldVehicleType},
		{name: "model", detail: "Vehicle model", kind: sourceFieldAsset}, {name: "skin", detail: "Vehicle skin", kind: sourceFieldAsset},
		{name: "speedMax", detail: "Maximum speed"}, {name: "turboSpeed", detail: "Turbo speed"},
		{name: "acceleration", detail: "Acceleration"}, {name: "decelIdle", detail: "Idle deceleration"},
		{name: "strafePerc", detail: "Strafe percentage"}, {name: "bankingSpeed", detail: "Banking speed"},
		{name: "rollLimit", detail: "Roll limit"}, {name: "pitchLimit", detail: "Pitch limit"},
		{name: "braking", detail: "Braking"}, {name: "mouseYaw", detail: "Mouse yaw scale"},
		{name: "mousePitch", detail: "Mouse pitch scale"}, {name: "customGravity", detail: "Custom gravity"},
		{name: "armor", detail: "Armor"}, {name: "shields", detail: "Shields"},
		{name: "weapons", detail: "Comma- or pipe-separated weapons", kind: sourceFieldWeapon, list: true},
	}}
}

func sourceSiegeSpecs(scope string) sourceScopeSpecs {
	switch scope {
	case "Global":
		return sourceScopeSpecs{fields: []sourceFieldSpec{
			{name: "missionname", detail: "Mission display name"}, {name: "mapgraphic", detail: "Map graphic", kind: sourceFieldAsset},
			{name: "radartopleft", detail: "Radar upper-left coordinates"}, {name: "radarbottomright", detail: "Radar lower-right coordinates"},
			{name: "mbmodesallowed", detail: "Allowed MB modes"}, {name: "roundbegin_target", detail: "Round begin target"},
			{name: "Teams", detail: "Team mapping block"}, {name: "HelpIcons", detail: "Help icon block"},
			{name: "AutoMap", detail: "Automap block"}, {name: "LevelshotDesc", detail: "Levelshot description block"},
		}}
	case "Teams":
		return sourceScopeSpecs{fields: []sourceFieldSpec{{name: "team1", detail: "First team block name"}, {name: "team2", detail: "Second team block name"}}}
	case "Team":
		return sourceScopeSpecs{fields: []sourceFieldSpec{
			{name: "useTeam", detail: "Relative .mbtc team reference", kind: sourceFieldAsset},
			{name: "teamIcon", detail: "Team icon", kind: sourceFieldAsset}, {name: "teamColorOn", detail: "Active team color"},
			{name: "teamColorOff", detail: "Inactive team color"}, {name: "requiredObjectives", detail: "Required objective count"},
			{name: "timed", detail: "Timed team flag", kind: sourceFieldBoolean}, {name: "attackers", detail: "Attacking team flag", kind: sourceFieldBoolean},
			{name: "wonround", detail: "Won-round message"}, {name: "lostround", detail: "Lost-round message"},
			{name: "roundover_sound_wewon", detail: "Victory sound", kind: sourceFieldAsset}, {name: "roundover_sound_welost", detail: "Defeat sound", kind: sourceFieldAsset},
			{name: "roundover_target", detail: "Round-over target"}, {name: "briefing", detail: "Team briefing"},
			{name: "FriendlyShader", detail: "Friendly-player shader", kind: sourceFieldAsset},
			{name: "FlagShader", detail: "Team flag shader", kind: sourceFieldAsset},
		}}
	case "Objective":
		return sourceScopeSpecs{fields: []sourceFieldSpec{
			{name: "goalname", detail: "Objective display name"}, {name: "final", detail: "Final objective flag", kind: sourceFieldBoolean},
			{name: "objdesc", detail: "Objective description"}, {name: "objgfx", detail: "Objective graphic", kind: sourceFieldAsset},
			{name: "mapicon", detail: "Map icon", kind: sourceFieldAsset}, {name: "litmapicon", detail: "Active map icon", kind: sourceFieldAsset},
			{name: "donemapicon", detail: "Completed map icon", kind: sourceFieldAsset}, {name: "mappos", detail: "Map position"},
			{name: "message_team1", detail: "Team 1 message"}, {name: "message_team2", detail: "Team 2 message"},
			{name: "sound_team1", detail: "Team 1 sound", kind: sourceFieldAsset}, {name: "sound_team2", detail: "Team 2 sound", kind: sourceFieldAsset},
			{name: "target", detail: "Objective target"},
		}}
	}
	return sourceScopeSpecs{}
}

func sourceMBTCSpecs(scope string) sourceScopeSpecs {
	switch scope {
	case "Global":
		fields := []sourceFieldSpec{
			{name: "name", detail: "Required team name"}, {name: "TimePeriod", detail: "Team time period"},
			{name: "EUAllowed", detail: "EU permission flag", kind: sourceFieldBoolean},
			{name: "FriendlyShader", detail: "Friendly-player shader", kind: sourceFieldAsset},
			{name: "ClassesAllowed", detail: "Legacy class mask"}, {name: "Classes", detail: "Class slots block"},
		}
		for i := 1; i <= 6; i++ {
			fields = append(fields, sourceFieldSpec{name: "SubclassesForClass" + strconv.Itoa(i), detail: "Subclass slots block"})
		}
		return sourceScopeSpecs{fields: fields}
	case "Classes":
		return sourceScopeSpecs{patterns: []*regexp.Regexp{regexp.MustCompile(`(?i)^class[1-6]$`)}, fields: []sourceFieldSpec{
			{name: "class1", detail: "Class identifier", kind: sourceFieldClass}, {name: "class2", detail: "Class identifier", kind: sourceFieldClass},
			{name: "class3", detail: "Class identifier", kind: sourceFieldClass}, {name: "class4", detail: "Class identifier", kind: sourceFieldClass},
			{name: "class5", detail: "Class identifier", kind: sourceFieldClass}, {name: "class6", detail: "Class identifier", kind: sourceFieldClass},
		}}
	case "Subclasses":
		fields := make([]sourceFieldSpec, 0, parsers.MBTCMaxSubclasses)
		for i := 1; i <= parsers.MBTCMaxSubclasses; i++ {
			fields = append(fields, sourceFieldSpec{name: "Subclass" + strconv.Itoa(i), detail: "Subclass identifier", kind: sourceFieldClass})
		}
		return sourceScopeSpecs{fields: fields, patterns: []*regexp.Regexp{regexp.MustCompile(`(?i)^subclass([1-9]|[1-3][0-9]|4[01])$`)}}
	}
	return sourceScopeSpecs{}
}

type sourceFieldOccurrence struct {
	block      string
	field      string
	fieldRange SourceRange
	value      string
	valueRange SourceRange
}

func sourceFieldOccurrences(text string, format SourceFormat) []sourceFieldOccurrence {
	tokens := sourceLex(text)
	var occurrences []sourceFieldOccurrence
	lineStart := 0
	for lineStart <= len(text) {
		lineEnd := len(text)
		if at := strings.Index(text[lineStart:], "\n"); at >= 0 {
			lineEnd = lineStart + at
		}
		lineTokens := make([]sourceLexToken, 0, 4)
		for _, token := range tokens {
			if token.end <= lineStart || token.start >= lineEnd || token.kind == sourceLexWhitespace || token.kind == sourceLexComment {
				continue
			}
			lineTokens = append(lineTokens, token)
		}
		if len(lineTokens) >= 2 && lineTokens[1].kind != sourceLexOpen && lineTokens[0].kind != sourceLexOpen && lineTokens[0].kind != sourceLexClose {
			fieldToken, valueToken := lineTokens[0], lineTokens[1]
			if valueToken.kind != sourceLexClose {
				valueStart, valueEnd := sourceValueInterior(valueToken)
				occurrences = append(occurrences, sourceFieldOccurrence{
					block: sourceBlockAt(tokens, fieldToken.start+1), field: sourceTokenValue(fieldToken),
					fieldRange: SourceRange{Start: fieldToken.start, End: fieldToken.end},
					value:      text[valueStart:valueEnd], valueRange: SourceRange{Start: valueStart, End: valueEnd},
				})
			}
		}
		if lineEnd == len(text) {
			break
		}
		lineStart = lineEnd + 1
	}
	return occurrences
}

func sourceSemanticWarnings(text string, format SourceFormat) []SourceDiagnostic {
	var diagnostics []SourceDiagnostic
	for _, occurrence := range sourceFieldOccurrences(text, format) {
		specs := sourceSpecsFor(format, occurrence.block)
		spec, known := sourceFindFieldSpec(specs, occurrence.field)
		if !known {
			if sourceNormalizeScope(format, occurrence.block) != "Opaque" && (format != SourceFormatMBCH || sourceSchemaLoaded) {
				diagnostics = append(diagnostics, sourceDiagnostic(text, occurrence.fieldRange, SourceWarning,
					"unknown field preserved: "+occurrence.field))
			}
			continue
		}
		knownValues := sourceValuesForSpec(spec, true)
		if len(knownValues) == 0 || spec.kind == sourceFieldAsset || spec.kind == sourceFieldBoolean {
			continue
		}
		knownSet := make(map[string]bool, len(knownValues))
		for _, value := range knownValues {
			knownSet[strings.ToLower(value)] = true
		}
		parts := []string{occurrence.value}
		if spec.list || strings.Contains(occurrence.value, "|") {
			parts = strings.Split(occurrence.value, "|")
		}
		searchStart := occurrence.valueRange.Start
		for _, part := range parts {
			trimmed := strings.TrimSpace(part)
			identifier, rank := sourceParseRank(trimmed)
			if spec.ranked && strings.Contains(trimmed, ",") {
				rankText := strings.TrimSpace(strings.SplitN(trimmed, ",", 2)[1])
				if rank == nil {
					at := strings.LastIndex(text[searchStart:occurrence.valueRange.End], rankText)
					rangeHint := occurrence.valueRange
					if at >= 0 {
						rangeHint = SourceRange{Start: searchStart + at, End: searchStart + at + len(rankText)}
					}
					diagnostics = append(diagnostics, sourceDiagnostic(text, rangeHint, SourceWarning,
						"invalid rank preserved for "+occurrence.field+": "+rankText))
				} else if maximum := sourceKnownRankMaximum(spec, identifier); maximum > 0 && (*rank < 0 || *rank > maximum) {
					diagnostics = append(diagnostics, sourceDiagnostic(text, occurrence.valueRange, SourceWarning,
						"rank outside known catalog range preserved for "+identifier+": "+strconv.Itoa(*rank)))
				}
			}
			if identifier == "" || knownSet[strings.ToLower(identifier)] {
				searchStart += len(part) + 1
				continue
			}
			at := strings.Index(text[searchStart:occurrence.valueRange.End], identifier)
			rangeHint := occurrence.valueRange
			if at >= 0 {
				rangeHint = SourceRange{Start: searchStart + at, End: searchStart + at + len(identifier)}
			}
			diagnostics = append(diagnostics, sourceDiagnostic(text, rangeHint, SourceWarning,
				"unknown value preserved for "+occurrence.field+": "+identifier))
			searchStart += len(part) + 1
		}
	}
	return diagnostics
}

func sourceKnownRankMaximum(spec sourceFieldSpec, identifier string) int {
	switch spec.kind {
	case sourceFieldAttribute:
		for _, attribute := range GetAllAttributes() {
			if strings.EqualFold(attribute.ID, identifier) {
				return attribute.MaxLevel
			}
		}
	case sourceFieldForcePower:
		return 3
	}
	return 0
}
