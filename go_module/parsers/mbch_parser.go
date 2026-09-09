package parsers

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// writeExtraFields emits ExtraFields map entries in sorted-key order
// so GenerateMBCH output is deterministic. Go's map iteration is
// intentionally randomized; without sorting, fields like primGore /
// altGore swap positions between ticks in the live source panel and
// round-trip diffs jitter line-by-line.
func writeExtraFields(sb *strings.Builder, fields map[string]string) {
	if len(fields) == 0 {
		return
	}
	keys := make([]string, 0, len(fields))
	for k := range fields {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		v := fields[k]
		if strings.Contains(v, " ") {
			fmt.Fprintf(sb, "\t%s\t\t\t\"%s\"\n", k, v)
		} else {
			fmt.Fprintf(sb, "\t%s\t\t\t%s\n", k, v)
		}
	}
}

// drainVariants pulls every key from `fields` matching `<base>_N` (for any
// integer N) in ascending N order, emits each adjacent to its base field
// (so model_1 / skin_2 / uishader_3 don't get exiled to the bottom of the
// block via writeExtraFields's alphabetical dump), and removes them from
// the map. Returns nothing — side effects on `sb` and `fields`.
func drainVariants(sb *strings.Builder, fields map[string]string, base string) {
	if len(fields) == 0 {
		return
	}
	prefix := base + "_"
	type pair struct {
		idx int
		key string
	}
	var found []pair
	for k := range fields {
		if !strings.HasPrefix(k, prefix) {
			continue
		}
		n, err := strconv.Atoi(strings.TrimPrefix(k, prefix))
		if err != nil {
			continue
		}
		found = append(found, pair{n, k})
	}
	sort.Slice(found, func(i, j int) bool { return found[i].idx < found[j].idx })
	for _, p := range found {
		v := fields[p.key]
		if strings.Contains(v, " ") {
			fmt.Fprintf(sb, "\t%s\t\t\"%s\"\n", p.key, v)
		} else {
			fmt.Fprintf(sb, "\t%s\t\t%s\n", p.key, v)
		}
		delete(fields, p.key)
	}
}

// WeaponInfo represents a weapon override block in an MBCH file.
// astName is the parse-time block identity (e.g. "weaponinfo2",
// lowercase): override blocks are keyed by NAME, never by slice index —
// files may carry non-contiguous numbering and the engine's
// ParseWeaponOverrides loop (bg_saga.c:2126-2253) stops scanning at the
// first gap, so renumbering would silently change what the game loads.
type WeaponInfo struct {
	astName string
	WeaponToReplace    string
	WeaponBasedOff     string
	NewWorldModel      string
	NewViewModel       string
	Icon               string
	WeaponName         string
	MuzzleEffect       string
	AltMuzzleEffect    string
	MissileEffect      string
	AltMissileEffect   string
	Missile3Effect     string
	AltMissileEffect3  string
	PowerupShotEffect  string
	PowerupShotEffect3 string
	FlashSound0        string
	AltFlashSound0     string
	ChargeSound        string
	AltChargeSound     string
	PrimHitSound       string
	AltHitSound        string
	CustomAmmo         int
	ClipSize           int
	ReloadTimeModifier float64
	ExtraFields        map[string]string
}

// ForceInfo represents a force power override block. astName mirrors
// WeaponInfo.astName (e.g. "forceinfo1").
type ForceInfo struct {
	astName string
	ForceToReplace string
	Icon           string
	ForcePowerName string
	StartSound     string
	LoopSound      string
	ExtraFields    map[string]string
}

// MBCHCharacter represents the parsed data of a .mbch file
type MBCHCharacter struct {
	ctx *sourceContext
	Name              string
	MBClass           string
	Model             string
	Skin              string
	UIShader          string
	Soundset          string
	Weapons           string
	Attributes        string
	ForcePowers       string
	SaberStyle        string
	ClassFlags        string
	MaxHealth         int
	MaxArmor          int
	ForcePool         int
	ForceRegen        float64
	Speed             float64
	APMultiplier      float64
	BPMultiplier      float64
	CSMultiplier      float64
	ASMultiplier      float64
	Saber1            string
	Saber2            string
	SaberColor        int
	Saber2Color       int
	ClassNumberLimit  int
	RespawnCustomTime int
	ExtraLives        int
	IsCustomBuild     int
	MBPoints          int
	Description       string
	ExtraFields       map[string]string

	// Diags collects non-fatal sync findings (e.g. non-contiguous
	// WeaponInfo<N> numbering, which makes the engine stop scanning at
	// the gap and ignore every later override). Non-destructive: the
	// file is never renumbered. Editors surface these to the user.
	Diags []string
	// Point-buy slots. Fixed capacity of 45 accommodates the
	// Legends 2.0 archetype system: up to 3 archetypes × 15 slots
	// each (spec1 uses 0-14, spec2 uses 15-29, spec3 uses 30-44).
	// Slots beyond `15 * HasCustomSpec` are ignored at serialize
	// time. Zero-valued entries don't emit. Descs added per the
	// monkey-lizard pattern (c_att_descs_N optional description
	// strings that render in the in-game loadout menu).
	CustomSkills   [45]string
	CustomNames    [45]string
	CustomRanks    [45]string
	CustomDescs    [45]string
	RankAttributes map[string]string

	// Archetype ("customSpec") system. hasCustomSpec declares how
	// many archetypes the character offers (1-3); per-archetype
	// name + icon + desc describe the in-game tab. When HasCustomSpec
	// > 1 each archetype gets its own 15-slot window in CustomSkills.
	// HasCustomSpec == 0 or 1 means "single spec" — treat CustomSkills
	// as one 15-slot build.
	//
	// IsOnlyOneSpec / DefaultSpec are first-class so the writer can
	// emit them adjacent to isCustomBuild + mbPoints + hasCustomSpec
	// (otherwise they leak into the alphabetical ExtraFields tail
	// and read oddly in the generated file).
	HasCustomSpec   int
	IsOnlyOneSpec   int       // bg_saga.c:2367 — point investment cannot span specs
	DefaultSpec     int       // bg_saga.c:2370 — initial visible spec tab (1-3)
	CustomSpecNames [3]string
	CustomSpecIcons [3]string
	CustomSpecDescs [3]string // bg_saga.c:2375 — tooltip-style spec description

	WeaponOverrides []WeaponInfo
	ForceOverrides  []ForceInfo
}

// NewMBCHCharacter creates a default character
func NewMBCHCharacter() *MBCHCharacter {
	return &MBCHCharacter{
		MaxHealth:        100,
		ForceRegen:       1.0,
		Speed:            1.0,
		APMultiplier:     1.0,
		BPMultiplier:     1.0,
		CSMultiplier:     1.0,
		ASMultiplier:     1.0,
		ClassNumberLimit: -1,
		ExtraFields:      make(map[string]string),
		RankAttributes:   make(map[string]string),
		WeaponOverrides:  []WeaponInfo{},
		ForceOverrides:   []ForceInfo{},
	}
}

// ParseMBCH parses the content of an MBCH file










// GenerateMBCH generates the string content for an MBCH file
func GenerateMBCH(char *MBCHCharacter) (string, error) {
	if char.ctx != nil && char.ctx.doc != nil {
		doc := cloneASTDocument(char.ctx.doc)
		syncMBCHToAST(char, doc)
		diagnoseOverrideGaps(char)
		return doc.String(), nil
	}

	var sb strings.Builder
	// Work on a shallow copy of ExtraFields so drainVariants doesn't
	// mutate the caller's character — round-trip tests (and any caller
	// that re-uses the struct after generating) depended on the input
	// surviving unchanged.
	extras := make(map[string]string, len(char.ExtraFields))
	for k, v := range char.ExtraFields {
		extras[k] = v
	}

	fmt.Fprintf(&sb, "// %s\n\nClassInfo\n{\n", char.Name)
	fmt.Fprintf(&sb, "\tname\t\t\t\"%s\"\n", char.Name)
	if char.MBClass != "" {
		fmt.Fprintf(&sb, "\tMBClass\t\t\t%s\n", char.MBClass)
	}
	if char.Model != "" {
		fmt.Fprintf(&sb, "\tmodel\t\t\t\"%s\"\n", char.Model)
	}
	drainVariants(&sb, extras, "model")
	drainVariants(&sb, extras, "customred")
	drainVariants(&sb, extras, "customgreen")
	drainVariants(&sb, extras, "customblue")
	drainVariants(&sb, extras, "userRGB")
	if char.Skin != "" {
		fmt.Fprintf(&sb, "\tskin\t\t\t\"%s\"\n", char.Skin)
	}
	drainVariants(&sb, extras, "skin")
	if char.UIShader != "" {
		fmt.Fprintf(&sb, "\tuishader\t\t\"%s\"\n", char.UIShader)
	}
	drainVariants(&sb, extras, "uishader")
	if char.Soundset != "" {
		fmt.Fprintf(&sb, "\tsoundset\t\t\"%s\"\n", char.Soundset)
	}
	if char.Weapons != "" {
		fmt.Fprintf(&sb, "\tweapons\t\t\t%s\n", char.Weapons)
	}
	if char.Attributes != "" {
		fmt.Fprintf(&sb, "\tattributes\t\t%s\n", char.Attributes)
	}
	if char.ForcePowers != "" {
		fmt.Fprintf(&sb, "\tforcepowers\t\t%s\n", char.ForcePowers)
	}
	if char.SaberStyle != "" {
		fmt.Fprintf(&sb, "\tsaberstyle\t\t%s\n", char.SaberStyle)
	}
	if char.ClassFlags != "" {
		fmt.Fprintf(&sb, "\tclassflags\t\t%s\n", char.ClassFlags)
	}
	fmt.Fprintf(&sb, "\tmaxhealth\t\t%d\n", char.MaxHealth)
	if char.MaxArmor > 0 {
		fmt.Fprintf(&sb, "\tmaxarmor\t\t%d\n", char.MaxArmor)
	}
	if char.ForcePool > 0 {
		fmt.Fprintf(&sb, "\tforcepool\t\t%d\n", char.ForcePool)
	}
	if char.ForceRegen != 1.0 {
		fmt.Fprintf(&sb, "\tforceregen\t\t%g\n", char.ForceRegen)
	}
	if char.Speed != 1.0 {
		fmt.Fprintf(&sb, "\tspeed\t\t\t%g\n", char.Speed)
	}
	if char.APMultiplier != 1.0 {
		fmt.Fprintf(&sb, "\tAPmultiplier\t\t%g\n", char.APMultiplier)
	}
	if char.BPMultiplier != 1.0 {
		fmt.Fprintf(&sb, "\tBPmultiplier\t\t%g\n", char.BPMultiplier)
	}
	if char.CSMultiplier != 1.0 {
		fmt.Fprintf(&sb, "\tCSmultiplier\t\t%g\n", char.CSMultiplier)
	}
	if char.ASMultiplier != 1.0 {
		fmt.Fprintf(&sb, "\tASMultiplier\t\t%g\n", char.ASMultiplier)
	}
	if char.Saber1 != "" {
		fmt.Fprintf(&sb, "\tsaber1\t\t\t%s\n", char.Saber1)
	}
	drainVariants(&sb, extras, "saber1")
	if char.Saber2 != "" {
		fmt.Fprintf(&sb, "\tsaber2\t\t\t%s\n", char.Saber2)
	}
	drainVariants(&sb, extras, "saber2")
	if char.SaberColor != 0 {
		fmt.Fprintf(&sb, "\tsabercolor\t\t%d\n", char.SaberColor)
	}
	drainVariants(&sb, extras, "sabercolor")
	if char.Saber2Color != 0 {
		fmt.Fprintf(&sb, "\tsaber2color\t\t%d\n", char.Saber2Color)
	}
	drainVariants(&sb, extras, "saber2color")
	drainVariants(&sb, extras, "saberstyle")
	if char.ClassNumberLimit != -1 {
		fmt.Fprintf(&sb, "\tclassNumberLimit\t%d\n", char.ClassNumberLimit)
	}
	if char.RespawnCustomTime > 0 {
		fmt.Fprintf(&sb, "\trespawnCustomTime\t%d\n", char.RespawnCustomTime)
	}
	if char.ExtraLives > 0 {
		fmt.Fprintf(&sb, "\textralives\t\t%d\n", char.ExtraLives)
	}

	if char.IsCustomBuild == 1 {
		fmt.Fprintf(&sb, "\tisCustomBuild\t\t1\n")
		fmt.Fprintf(&sb, "\tmbPoints\t\t%d\n", char.MBPoints)
		// Emit isOnlyOneSpec / defaultSpec adjacent to the custom-build
		// header — both are engine-parsed (bg_saga.c:2367,2370) and were
		// previously round-tripping through the alphabetical ExtraFields
		// tail, which scattered them away from the rest of the custom-
		// build block in the saved file.
		if char.IsOnlyOneSpec != 0 {
			fmt.Fprintf(&sb, "\tisOnlyOneSpec\t\t%d\n", char.IsOnlyOneSpec)
		}
		if char.DefaultSpec != 0 {
			fmt.Fprintf(&sb, "\tdefaultSpec\t\t%d\n", char.DefaultSpec)
		}

		// Archetype ("customSpec") header. hasCustomSpec = number of
		// archetypes (1-3). When > 1, customSpecName_N/customSpecIcon_N
		// /customSpecDesc_N define each archetype's in-menu tab.
		// Wiki uses 1-based indexing (customSpecName_1 is the first
		// spec) so we emit with +1 offset and skip empty slots.
		if char.HasCustomSpec > 1 {
			fmt.Fprintf(&sb, "\thasCustomSpec\t\t%d\n", char.HasCustomSpec)
			for i := 0; i < char.HasCustomSpec && i < 3; i++ {
				if name := char.CustomSpecNames[i]; name != "" {
					fmt.Fprintf(&sb, "\tcustomSpecName_%d\t\"%s\"\n", i+1, name)
				}
				if icon := char.CustomSpecIcons[i]; icon != "" {
					fmt.Fprintf(&sb, "\tcustomSpecIcon_%d\t\"%s\"\n", i+1, icon)
				}
				if desc := char.CustomSpecDescs[i]; desc != "" {
					fmt.Fprintf(&sb, "\tcustomSpecDesc_%d\t\"%s\"\n", i+1, desc)
				}
			}
		}

		// Point-buy slot emission. 15 slots for single-spec classes,
		// 15 × HasCustomSpec for multi-archetype ones. Slot index is
		// the canonical identifier in-game; we preserve it exactly so
		// round-trip edits don't scramble archetypes.
		slotCount := 15
		if char.HasCustomSpec > 1 {
			slotCount = 15 * char.HasCustomSpec
			if slotCount > 45 {
				slotCount = 45
			}
		}
		for i := 0; i < slotCount; i++ {
			if char.CustomSkills[i] != "" {
				fmt.Fprintf(&sb, "\tc_att_skill_%d\t%s\n", i, char.CustomSkills[i])
				if char.CustomNames[i] != "" {
					fmt.Fprintf(&sb, "\tc_att_names_%d\t\"%s\"\n", i, char.CustomNames[i])
				}
				if char.CustomRanks[i] != "" {
					fmt.Fprintf(&sb, "\tc_att_ranks_%d\t%s\n", i, char.CustomRanks[i])
				}
				if char.CustomDescs[i] != "" {
					fmt.Fprintf(&sb, "\tc_att_descs_%d\t\"%s\"\n", i, char.CustomDescs[i])
				}
			}
		}

		// Sorted iteration — same determinism reason as writeExtraFields.
		rankKeys := make([]string, 0, len(char.RankAttributes))
		for k := range char.RankAttributes {
			rankKeys = append(rankKeys, k)
		}
		sort.Strings(rankKeys)
		for _, k := range rankKeys {
			fmt.Fprintf(&sb, "\t%s\t\t%s\n", k, char.RankAttributes[k])
		}
	}

	writeExtraFields(&sb, extras)
	fmt.Fprintln(&sb, "}")

	// Weapon Info
	for i, wi := range char.WeaponOverrides {
		fmt.Fprintln(&sb)
		fmt.Fprintf(&sb, "WeaponInfo%d\n{\n", i)
		if wi.WeaponToReplace != "" {
			fmt.Fprintf(&sb, "\tWeaponToReplace\t\t%s\n", wi.WeaponToReplace)
		}
		if wi.WeaponBasedOff != "" {
			fmt.Fprintf(&sb, "\tWeaponBasedOff\t\t%s\n", wi.WeaponBasedOff)
		}
		if wi.NewWorldModel != "" {
			fmt.Fprintf(&sb, "\tNewWorldModel\t\t\"%s\"\n", wi.NewWorldModel)
		}
		if wi.NewViewModel != "" {
			fmt.Fprintf(&sb, "\tNewViewModel\t\t\"%s\"\n", wi.NewViewModel)
		}
		if wi.Icon != "" {
			fmt.Fprintf(&sb, "\tIcon\t\t\t\"%s\"\n", wi.Icon)
		}
		if wi.WeaponName != "" {
			fmt.Fprintf(&sb, "\tWeaponName\t\t\"%s\"\n", wi.WeaponName)
		}
		if wi.MuzzleEffect != "" {
			fmt.Fprintf(&sb, "\tMuzzleEffect\t\t\"%s\"\n", wi.MuzzleEffect)
		}
		if wi.AltMuzzleEffect != "" {
			fmt.Fprintf(&sb, "\tAltMuzzleEffect\t\t\"%s\"\n", wi.AltMuzzleEffect)
		}
		if wi.MissileEffect != "" {
			fmt.Fprintf(&sb, "\tMissileEffect\t\t\"%s\"\n", wi.MissileEffect)
		}
		if wi.AltMissileEffect != "" {
			fmt.Fprintf(&sb, "\tAltMissileEffect\t\"%s\"\n", wi.AltMissileEffect)
		}
		if wi.Missile3Effect != "" {
			fmt.Fprintf(&sb, "\tMissile3Effect\t\t\"%s\"\n", wi.Missile3Effect)
		}
		if wi.AltMissileEffect3 != "" {
			fmt.Fprintf(&sb, "\tAltMissileEffect3\t\"%s\"\n", wi.AltMissileEffect3)
		}
		if wi.PowerupShotEffect != "" {
			fmt.Fprintf(&sb, "\tPowerupShotEffect\t\"%s\"\n", wi.PowerupShotEffect)
		}
		if wi.PowerupShotEffect3 != "" {
			fmt.Fprintf(&sb, "\tPowerupShotEffect3\t\"%s\"\n", wi.PowerupShotEffect3)
		}
		if wi.FlashSound0 != "" {
			fmt.Fprintf(&sb, "\tFlashSound0\t\t\"%s\"\n", wi.FlashSound0)
		}
		if wi.AltFlashSound0 != "" {
			fmt.Fprintf(&sb, "\tAltFlashSound0\t\t\"%s\"\n", wi.AltFlashSound0)
		}
		if wi.ChargeSound != "" {
			fmt.Fprintf(&sb, "\tChargeSound\t\t\"%s\"\n", wi.ChargeSound)
		}
		if wi.AltChargeSound != "" {
			fmt.Fprintf(&sb, "\tAltChargeSound\t\t\"%s\"\n", wi.AltChargeSound)
		}
		if wi.PrimHitSound != "" {
			fmt.Fprintf(&sb, "\tPrimHitSound\t\t\"%s\"\n", wi.PrimHitSound)
		}
		if wi.AltHitSound != "" {
			fmt.Fprintf(&sb, "\tAltHitSound\t\t\"%s\"\n", wi.AltHitSound)
		}
		if wi.CustomAmmo > 0 {
			fmt.Fprintf(&sb, "\tcustomAmmo\t\t%d\n", wi.CustomAmmo)
		}
		if wi.ClipSize > 0 {
			fmt.Fprintf(&sb, "\tclipSize\t\t%d\n", wi.ClipSize)
		}
		if wi.ReloadTimeModifier > 0 {
			fmt.Fprintf(&sb, "\treloadTimeModifier\t%g\n", wi.ReloadTimeModifier)
		}
		writeExtraFields(&sb, wi.ExtraFields)
		fmt.Fprintln(&sb, "}")
	}

	// Force Info
	for i, fi := range char.ForceOverrides {
		fmt.Fprintln(&sb)
		fmt.Fprintf(&sb, "ForceInfo%d\n{\n", i)
		if fi.ForceToReplace != "" {
			fmt.Fprintf(&sb, "\tForceToReplace\t\t%s\n", fi.ForceToReplace)
		}
		if fi.Icon != "" {
			fmt.Fprintf(&sb, "\tIcon\t\t\t\"%s\"\n", fi.Icon)
		}
		if fi.ForcePowerName != "" {
			fmt.Fprintf(&sb, "\tForcePowerName\t\t\"%s\"\n", fi.ForcePowerName)
		}
		if fi.StartSound != "" {
			fmt.Fprintf(&sb, "\tStartSound\t\t\"%s\"\n", fi.StartSound)
		}
		if fi.LoopSound != "" {
			fmt.Fprintf(&sb, "\tLoopSound\t\t\"%s\"\n", fi.LoopSound)
		}
		writeExtraFields(&sb, fi.ExtraFields)
		fmt.Fprintln(&sb, "}")
	}

	fmt.Fprintln(&sb)
	if char.Description != "" {
		fmt.Fprintf(&sb, "description\t\"%s\"\n", char.Description)
	}

	return sb.String(), nil
}


func ParseMBCH(content string) (*MBCHCharacter, error) {
	tokens, err := Lex(content)
	if err != nil { return nil, err }
	doc := parseAST(tokens)
	
	char := NewMBCHCharacter()
	char.ctx = &sourceContext{doc: doc}
	
	for i := 0; i < len(doc.Nodes); i++ {
		if tok, ok := doc.Nodes[i].(*ASTToken); ok && tok.Type == TokenString && strings.ToLower(unquote(tok.Text)) == "description" {
			for j := i+1; j < len(doc.Nodes); j++ {
				if vTok, ok := doc.Nodes[j].(*ASTToken); ok {
					if vTok.Type == TokenWhitespace || vTok.Type == TokenComment {
						continue
					}
					if vTok.Type == TokenString {
						// Found the start of the description
						desc := vTok.Text
						if strings.HasPrefix(desc, "\"") && !strings.HasSuffix(desc, "\"") {
							// It's an unclosed quote, consume tokens until we find the closing quote
							for k := j + 1; k < len(doc.Nodes); k++ {
								if kTok, ok := doc.Nodes[k].(*ASTToken); ok {
									desc += kTok.Text
									if strings.Contains(kTok.Text, "\"") {
										break
									}
								}
							}
						}
						char.Description = unquote(desc)
						break
					}
				}
				if _, ok := doc.Nodes[j].(*ASTBlock); ok { break }
			}
			break
		}
	}

	seenClassInfo := false
	seenWeaponInfo := make(map[string]bool)
	seenForceInfo := make(map[string]bool)

	// Only TOP-LEVEL groups are effective: BG_SiegeGetValueGroup skips
	// non-matching group bodies by brace counting (bg_saga.c:1429-1465),
	// so a nested ClassInfo/WeaponInfo/ForceInfo is
	// dead text the engine never reads. Nested bodies stay in the
	// document verbatim and are never parsed into the model.
	for _, node := range doc.Nodes {
		b, ok := node.(*ASTBlock)
		if !ok || b.NameToken == nil {
			continue
		}
		name := strings.ToLower(b.NameToken.Text)
		switch {
		case name == "classinfo" && !seenClassInfo:
			seenClassInfo = true
			populateClassInfo(b, char)
		case strings.HasPrefix(name, "weaponinfo") && !seenWeaponInfo[name]:
			seenWeaponInfo[name] = true
			parseWeaponInfo(b, char, name)
		case strings.HasPrefix(name, "forceinfo") && !seenForceInfo[name]:
			seenForceInfo[name] = true
			parseForceInfo(b, char, name)
		}
	}
	diagnoseOverrideGaps(char)
	return char, nil
}

// diagnoseOverrideGaps records non-contiguous WeaponInfo<N>/ForceInfo<N>
// numbering. The engine consumes overrides through a linear scan seeded
// at WeaponInfo0 (bg_saga.c:2126: Com_sprintf(curValueGroup,
// "WeaponInfo%i", 0); i++ per hit — bg_saga.c:2250-2253) that stops at
// the FIRST missing index, so numbering is zero-based and must be
// contiguous; a gap orphans every later block. We diagnose instead of
// renumbering: the numbering is the file's identity and renumbering
// would silently change what the game loads.
func diagnoseOverrideGaps(char *MBCHCharacter) {
	// Filter out old gap diagnostics so they don't compound on re-generation
	var newDiags []string
	for _, d := range char.Diags {
		if !strings.Contains(d, "missing: the engine stops scanning at the first gap") {
			newDiags = append(newDiags, d)
		}
	}
	char.Diags = newDiags

	check := func(kind string, overrides []string) {
		seen := make(map[int]bool)
		max := -1
		for _, name := range overrides {
			idx := -1
			fmt.Sscanf(name, kind+"%d", &idx)
			if idx < 0 {
				continue
			}
			seen[idx] = true
			if idx > max {
				max = idx
			}
		}
		for i := range max {
			if !seen[i] {
				char.Diags = append(char.Diags, fmt.Sprintf(
					"%s%d missing: the engine stops scanning at the first gap and will ignore every later override block", kind, i))
			}
		}
	}
	wNames := make([]string, 0, len(char.WeaponOverrides))
	for _, wi := range char.WeaponOverrides {
		if wi.astName != "" {
			wNames = append(wNames, wi.astName)
		}
	}
	check("weaponinfo", wNames)
	fNames := make([]string, 0, len(char.ForceOverrides))
	for _, fi := range char.ForceOverrides {
		if fi.astName != "" {
			fNames = append(fNames, fi.astName)
		}
	}
	check("forceinfo", fNames)
}

func populateClassInfo(b *ASTBlock, char *MBCHCharacter) {
	char.ExtraFields = make(map[string]string)
	char.RankAttributes = make(map[string]string)
	typedKeys := make(map[string]bool)
	markTyped := func(k string) { typedKeys[strings.ToLower(k)] = true }
	markTyped("name"); markTyped("mbclass"); markTyped("model"); markTyped("skin")
	markTyped("uishader"); markTyped("soundset"); markTyped("weapons"); markTyped("attributes")
	markTyped("forcepowers"); markTyped("saberstyle"); markTyped("classflags")
	markTyped("maxhealth"); markTyped("maxarmor"); markTyped("forcepool"); markTyped("forceregen")
	markTyped("speed"); markTyped("apmultiplier"); markTyped("bpmultiplier"); markTyped("csmultiplier")
	markTyped("asmultiplier"); markTyped("saber1"); markTyped("saber2"); markTyped("sabercolor")
	markTyped("saber2color"); markTyped("classnumberlimit"); markTyped("respawncustomtime")
	markTyped("extralives"); markTyped("iscustombuild"); markTyped("mbpoints")
	markTyped("isonlyonespec"); markTyped("defaultspec"); markTyped("hascustomspec")
	// description is synced at file top level (SGPV reads it from the
	// file buffer, bg_saga.c:2383 — never inside ClassInfo). Excluding it
	// here keeps a stray in-block description from being duplicated into
	// the extras tail on save.
	markTyped("description")
	for i := 1; i <= 3; i++ {
		markTyped(fmt.Sprintf("customspecname_%d", i))
		markTyped(fmt.Sprintf("customspecicon_%d", i))
		markTyped(fmt.Sprintf("customspecdesc_%d", i))
	}
	for i := 0; i <= 45; i++ {
		markTyped(fmt.Sprintf("c_att_skill_%d", i))
		markTyped(fmt.Sprintf("c_att_names_%d", i))
		markTyped(fmt.Sprintf("c_att_ranks_%d", i))
		markTyped(fmt.Sprintf("c_att_descs_%d", i))
	}
	// RankAttributes are populated below and dynamically typed, so they don't need marking here yet.
	
	if val, ok := getFieldValueSGPV(b, "name"); ok { char.Name = val }
	if val, ok := getFieldValueSGPV(b, "mbclass"); ok { char.MBClass = val }
	if val, ok := getFieldValueSGPV(b, "model"); ok { char.Model = val }
	if val, ok := getFieldValueSGPV(b, "skin"); ok { char.Skin = val }
	if val, ok := getFieldValueSGPV(b, "uishader"); ok { char.UIShader = val }
	if val, ok := getFieldValueSGPV(b, "soundset"); ok { char.Soundset = val }
	if val, ok := getFieldValueSGPV(b, "weapons"); ok { char.Weapons = val }
	if val, ok := getFieldValueSGPV(b, "attributes"); ok { char.Attributes = val }
	if val, ok := getFieldValueSGPV(b, "forcepowers"); ok { char.ForcePowers = val }
	if val, ok := getFieldValueSGPV(b, "saberstyle"); ok { char.SaberStyle = val }
	if val, ok := getFieldValueSGPV(b, "classflags"); ok { char.ClassFlags = val }
	if val, ok := getFieldValueSGPV(b, "maxhealth"); ok { char.MaxHealth, _ = strconv.Atoi(val) }
	if val, ok := getFieldValueSGPV(b, "maxarmor"); ok { char.MaxArmor, _ = strconv.Atoi(val) }
	if val, ok := getFieldValueSGPV(b, "forcepool"); ok { char.ForcePool, _ = strconv.Atoi(val) }
	if val, ok := getFieldValueSGPV(b, "forceregen"); ok { char.ForceRegen, _ = strconv.ParseFloat(val, 64) }
	if val, ok := getFieldValueSGPV(b, "speed"); ok { char.Speed, _ = strconv.ParseFloat(val, 64) }
	if val, ok := getFieldValueSGPV(b, "apmultiplier"); ok { char.APMultiplier, _ = strconv.ParseFloat(val, 64) }
	if val, ok := getFieldValueSGPV(b, "bpmultiplier"); ok { char.BPMultiplier, _ = strconv.ParseFloat(val, 64) }
	if val, ok := getFieldValueSGPV(b, "csmultiplier"); ok { char.CSMultiplier, _ = strconv.ParseFloat(val, 64) }
	if val, ok := getFieldValueSGPV(b, "asmultiplier"); ok { char.ASMultiplier, _ = strconv.ParseFloat(val, 64) }
	if val, ok := getFieldValueSGPV(b, "saber1"); ok { char.Saber1 = val }
	if val, ok := getFieldValueSGPV(b, "saber2"); ok { char.Saber2 = val }
	if val, ok := getFieldValueSGPV(b, "sabercolor"); ok { char.SaberColor, _ = strconv.Atoi(val) }
	if val, ok := getFieldValueSGPV(b, "saber2color"); ok { char.Saber2Color, _ = strconv.Atoi(val) }
	if val, ok := getFieldValueSGPV(b, "classnumberlimit"); ok { char.ClassNumberLimit, _ = strconv.Atoi(val) }
	if val, ok := getFieldValueSGPV(b, "respawncustomtime"); ok { char.RespawnCustomTime, _ = strconv.Atoi(val) }
	if val, ok := getFieldValueSGPV(b, "extralives"); ok { char.ExtraLives, _ = strconv.Atoi(val) }
	if val, ok := getFieldValueSGPV(b, "iscustombuild"); ok { char.IsCustomBuild, _ = strconv.Atoi(val) }
	if val, ok := getFieldValueSGPV(b, "mbpoints"); ok { char.MBPoints, _ = strconv.Atoi(val) }
	if val, ok := getFieldValueSGPV(b, "isonlyonespec"); ok { char.IsOnlyOneSpec, _ = strconv.Atoi(val) }
	if val, ok := getFieldValueSGPV(b, "defaultspec"); ok { char.DefaultSpec, _ = strconv.Atoi(val) }
	if val, ok := getFieldValueSGPV(b, "hascustomspec"); ok { char.HasCustomSpec, _ = strconv.Atoi(val) }
	
	for i := 0; i < 3; i++ {
		suffix := fmt.Sprintf("_%d", i+1)
		if val, ok := getFieldValueSGPV(b, "customspecname"+suffix); ok { char.CustomSpecNames[i] = val }
		if val, ok := getFieldValueSGPV(b, "customspecicon"+suffix); ok { char.CustomSpecIcons[i] = val }
		if val, ok := getFieldValueSGPV(b, "customspecdesc"+suffix); ok { char.CustomSpecDescs[i] = val }
	}
	
	for i := 0; i < 45; i++ {
		suffix := fmt.Sprintf("_%d", i)
		if val, ok := getFieldValueSGPV(b, "c_att_skill"+suffix); ok { char.CustomSkills[i] = val }
		if val, ok := getFieldValueSGPV(b, "c_att_names"+suffix); ok { char.CustomNames[i] = val }
		if val, ok := getFieldValueSGPV(b, "c_att_ranks"+suffix); ok { char.CustomRanks[i] = val }
		if val, ok := getFieldValueSGPV(b, "c_att_descs"+suffix); ok { char.CustomDescs[i] = val }
	}
	
	// SGPV-accurate pairing: the first key of each line owns its value;
	// everything after the pair on the line is dead text (bg_saga.c:294-
	// 297 skips to the next newline), so value tokens are never misread
	// as keys.
	walkSGPVPairs(b.Children, func(_ int, key string, _ int, val string, hasVal bool) bool {
		lowerKey := strings.ToLower(key)
		if hasVal && !typedKeys[lowerKey] {
			if strings.HasPrefix(lowerKey, "rank_") {
				if _, exists := char.RankAttributes[key]; !exists {
					char.RankAttributes[key] = val
				}
			} else {
				if _, exists := char.ExtraFields[key]; !exists {
					char.ExtraFields[key] = val
				}
			}
		}
		return true
	})
	
	for k := range char.RankAttributes { markTyped(k) }
}

func parseWeaponInfo(b *ASTBlock, char *MBCHCharacter, astName string) {
	wi := WeaponInfo{ExtraFields: make(map[string]string), astName: astName}
	typedKeys := make(map[string]bool)
	markTyped := func(k string) { typedKeys[strings.ToLower(k)] = true }
	markTyped("weapontoreplace"); markTyped("weaponbasedoff"); markTyped("newworldmodel")
	markTyped("newviewmodel"); markTyped("icon"); markTyped("weaponname"); markTyped("muzzleeffect")
	markTyped("altmuzzleeffect"); markTyped("missileeffect"); markTyped("altmissileeffect")
	markTyped("missile3effect"); markTyped("altmissileeffect3"); markTyped("powerupshoteffect")
	markTyped("powerupshoteffect3"); markTyped("flashsound0"); markTyped("altflashsound0")
	markTyped("chargesound"); markTyped("altchargesound"); markTyped("primhitsound")
	markTyped("althitsound");
	markTyped("customammo"); markTyped("clipsize"); markTyped("reloadtimemodifier")
	
	if val, ok := getFieldValueSGPV(b, "weapontoreplace"); ok { wi.WeaponToReplace = val }
	if val, ok := getFieldValueSGPV(b, "weaponbasedoff"); ok { wi.WeaponBasedOff = val }
	if val, ok := getFieldValueSGPV(b, "newworldmodel"); ok { wi.NewWorldModel = val }
	if val, ok := getFieldValueSGPV(b, "newviewmodel"); ok { wi.NewViewModel = val }
	if val, ok := getFieldValueSGPV(b, "icon"); ok { wi.Icon = val }
	if val, ok := getFieldValueSGPV(b, "weaponname"); ok { wi.WeaponName = val }
	if val, ok := getFieldValueSGPV(b, "muzzleeffect"); ok { wi.MuzzleEffect = val }
	if val, ok := getFieldValueSGPV(b, "altmuzzleeffect"); ok { wi.AltMuzzleEffect = val }
	if val, ok := getFieldValueSGPV(b, "missileeffect"); ok { wi.MissileEffect = val }
	if val, ok := getFieldValueSGPV(b, "altmissileeffect"); ok { wi.AltMissileEffect = val }
	if val, ok := getFieldValueSGPV(b, "missile3effect"); ok { wi.Missile3Effect = val }
	if val, ok := getFieldValueSGPV(b, "altmissileeffect3"); ok { wi.AltMissileEffect3 = val }
	if val, ok := getFieldValueSGPV(b, "powerupshoteffect"); ok { wi.PowerupShotEffect = val }
	if val, ok := getFieldValueSGPV(b, "powerupshoteffect3"); ok { wi.PowerupShotEffect3 = val }
	if val, ok := getFieldValueSGPV(b, "flashsound0"); ok { wi.FlashSound0 = val }
	if val, ok := getFieldValueSGPV(b, "altflashsound0"); ok { wi.AltFlashSound0 = val }
	if val, ok := getFieldValueSGPV(b, "chargesound"); ok { wi.ChargeSound = val }
	if val, ok := getFieldValueSGPV(b, "altchargesound"); ok { wi.AltChargeSound = val }
	if val, ok := getFieldValueSGPV(b, "primhitsound"); ok { wi.PrimHitSound = val }
	if val, ok := getFieldValueSGPV(b, "althitsound"); ok { wi.AltHitSound = val }
	if val, ok := getFieldValueSGPV(b, "customammo"); ok { wi.CustomAmmo, _ = strconv.Atoi(val) }
	if val, ok := getFieldValueSGPV(b, "clipsize"); ok { wi.ClipSize, _ = strconv.Atoi(val) }
	if val, ok := getFieldValueSGPV(b, "reloadtimemodifier"); ok { wi.ReloadTimeModifier, _ = strconv.ParseFloat(val, 64) }
	
	// SGPV-accurate pairing (see populateClassInfo).
	walkSGPVPairs(b.Children, func(_ int, key string, _ int, val string, hasVal bool) bool {
		if hasVal && !typedKeys[strings.ToLower(key)] {
			if _, exists := wi.ExtraFields[key]; !exists {
				wi.ExtraFields[key] = val
			}
		}
		return true
	})
	char.WeaponOverrides = append(char.WeaponOverrides, wi)
}

func parseForceInfo(b *ASTBlock, char *MBCHCharacter, astName string) {
	fi := ForceInfo{ExtraFields: make(map[string]string), astName: astName}
	typedKeys := make(map[string]bool)
	markTyped := func(k string) { typedKeys[strings.ToLower(k)] = true }
	markTyped("forcetoreplace"); markTyped("icon"); markTyped("forcepowername"); markTyped("startsound"); markTyped("loopsound")

	if val, ok := getFieldValueSGPV(b, "forcetoreplace"); ok { fi.ForceToReplace = val }
	if val, ok := getFieldValueSGPV(b, "icon"); ok { fi.Icon = val }
	if val, ok := getFieldValueSGPV(b, "forcepowername"); ok { fi.ForcePowerName = val }
	if val, ok := getFieldValueSGPV(b, "startsound"); ok { fi.StartSound = val }
	if val, ok := getFieldValueSGPV(b, "loopsound"); ok { fi.LoopSound = val }

	// SGPV-accurate pairing (see populateClassInfo).
	walkSGPVPairs(b.Children, func(_ int, key string, _ int, val string, hasVal bool) bool {
		if hasVal && !typedKeys[strings.ToLower(key)] {
			if _, exists := fi.ExtraFields[key]; !exists {
				fi.ExtraFields[key] = val
			}
		}
		return true
	})
	char.ForceOverrides = append(char.ForceOverrides, fi)
}

