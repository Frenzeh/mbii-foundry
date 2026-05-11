package parsers

import (
	"fmt"
	"log"
	"regexp"
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

// WeaponInfo represents a weapon override block in an MBCH file
type WeaponInfo struct {
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

// ForceInfo represents a force power override block
type ForceInfo struct {
	ForceToReplace string
	Icon           string
	ForcePowerName string
	StartSound     string
	LoopSound      string
	ExtraFields    map[string]string
}

// MBCHCharacter represents the parsed data of a .mbch file
type MBCHCharacter struct {
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
	// name + icon describe the in-game tab. When HasCustomSpec > 1
	// each archetype gets its own 15-slot window in CustomSkills.
	// HasCustomSpec == 0 or 1 means "single spec" — treat CustomSkills
	// as one 15-slot build.
	HasCustomSpec   int
	CustomSpecNames [3]string
	CustomSpecIcons [3]string

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
func ParseMBCH(content string) (*MBCHCharacter, error) {
	char := NewMBCHCharacter()

	// Strip comments
	lines := []string{}
	for _, line := range strings.Split(content, "\n") {
		idx := strings.Index(line, "//")
		if idx >= 0 {
			line = line[:idx]
		}
		lines = append(lines, line)
	}
	cleanContent := strings.Join(lines, "\n")

	// Extract ClassInfo block
	re := regexp.MustCompile(`(?is)ClassInfo\s*\{([^}]+)\}`)
	match := re.FindStringSubmatch(cleanContent)
	if len(match) > 1 {
		log.Printf("DEBUG: Found ClassInfo Block. Length: %d\n", len(match[1]))
		parseClassInfo(match[1], char)
	} else {
		return nil, fmt.Errorf("no ClassInfo block found")
	}

	// Extract WeaponInfo blocks
	wiRe := regexp.MustCompile(`(?is)WeaponInfo\d+\s*\{([^}]+)\}`)
	wiMatches := wiRe.FindAllStringSubmatch(cleanContent, -1)
	for _, m := range wiMatches {
		parseWeaponInfo(m[1], char)
	}

	// Extract ForceInfo blocks
	fiRe := regexp.MustCompile(`(?is)ForceInfo\d+\s*\{([^}]+)\}`)
	fiMatches := fiRe.FindAllStringSubmatch(cleanContent, -1)
	for _, m := range fiMatches {
		parseForceInfo(m[1], char)
	}

	// Extract Description (outside ClassInfo usually)
	descRe := regexp.MustCompile(`(?is)description\s+\"([^\"]*)\"`)
	descMatch := descRe.FindStringSubmatch(cleanContent)
	if len(descMatch) > 1 {
		char.Description = descMatch[1]
	}

	return char, nil
}

func parseClassInfo(block string, char *MBCHCharacter) {
	for _, line := range strings.Split(block, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "//") {
			continue
		}

		parts := strings.Fields(line)
		if len(parts) < 2 {
			continue
		}

		key := parts[0]
		idx := strings.Index(line, key)
		valuePart := strings.TrimSpace(line[idx+len(key):])

		value := valuePart
		if strings.HasPrefix(value, "\"") && strings.HasSuffix(value, "\"") {
			if len(value) >= 2 {
				value = value[1 : len(value)-1]
			}
		}

		setField(char, key, value)
	}
}

func parseWeaponInfo(block string, char *MBCHCharacter) {
	wi := WeaponInfo{ExtraFields: make(map[string]string)}

	for _, line := range strings.Split(block, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "//") {
			continue
		}

		parts := strings.Fields(line)
		if len(parts) < 2 {
			continue
		}

		key := parts[0]
		idx := strings.Index(line, key)
		valuePart := strings.TrimSpace(line[idx+len(key):])

		value := valuePart
		if strings.HasPrefix(value, "\"") && strings.HasSuffix(value, "\"") {
			if len(value) >= 2 {
				value = value[1 : len(value)-1]
			}
		}

		switch strings.ToLower(key) {
		case "weapontoreplace":
			wi.WeaponToReplace = value
		case "weaponbasedoff":
			wi.WeaponBasedOff = value
		case "newworldmodel":
			wi.NewWorldModel = value
		case "newviewmodel":
			wi.NewViewModel = value
		case "icon":
			wi.Icon = value
		case "weaponname":
			wi.WeaponName = value
		case "muzzleeffect":
			wi.MuzzleEffect = value
		case "altmuzzleeffect":
			wi.AltMuzzleEffect = value
		case "missileeffect":
			wi.MissileEffect = value
		case "altmissileeffect":
			wi.AltMissileEffect = value
		case "missile3effect":
			wi.Missile3Effect = value
		case "altmissileeffect3":
			wi.AltMissileEffect3 = value
		case "powerupshoteffect":
			wi.PowerupShotEffect = value
		case "powerupshoteffect3":
			wi.PowerupShotEffect3 = value
		case "flashsound0":
			wi.FlashSound0 = value
		case "altflashsound0":
			wi.AltFlashSound0 = value
		case "chargesound":
			wi.ChargeSound = value
		case "altchargesound":
			wi.AltChargeSound = value
		case "primhitsound":
			wi.PrimHitSound = value
		case "althitsound":
			wi.AltHitSound = value
		case "customammo":
			wi.CustomAmmo, _ = strconv.Atoi(value)
		case "clipsize":
			wi.ClipSize, _ = strconv.Atoi(value)
		case "reloadtimemodifier":
			wi.ReloadTimeModifier, _ = strconv.ParseFloat(value, 64)
		default:
			wi.ExtraFields[key] = value
		}
	}
	char.WeaponOverrides = append(char.WeaponOverrides, wi)
}

func parseForceInfo(block string, char *MBCHCharacter) {
	fi := ForceInfo{ExtraFields: make(map[string]string)}

	for _, line := range strings.Split(block, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "//") {
			continue
		}

		parts := strings.Fields(line)
		if len(parts) < 2 {
			continue
		}

		key := parts[0]
		idx := strings.Index(line, key)
		valuePart := strings.TrimSpace(line[idx+len(key):])

		value := valuePart
		if strings.HasPrefix(value, "\"") && strings.HasSuffix(value, "\"") {
			if len(value) >= 2 {
				value = value[1 : len(value)-1]
			}
		}

		switch strings.ToLower(key) {
		case "forcetoreplace":
			fi.ForceToReplace = value
		case "icon":
			fi.Icon = value
		case "forcepowername":
			fi.ForcePowerName = value
		case "startsound":
			fi.StartSound = value
		case "loopsound":
			fi.LoopSound = value
		default:
			fi.ExtraFields[key] = value
		}
	}
	char.ForceOverrides = append(char.ForceOverrides, fi)
}

func setField(char *MBCHCharacter, key, value string) {
	// Point-buy slots — up to 45 (3 archetypes × 15). Legends FAs
	// like h3_CloneCom already emit c_att_skill_30+; legacy single-
	// archetype files stay inside 0-14.
	if strings.HasPrefix(key, "c_att_skill_") {
		if idx, err := strconv.Atoi(strings.TrimPrefix(key, "c_att_skill_")); err == nil && idx >= 0 && idx < 45 {
			char.CustomSkills[idx] = value
			return
		}
	}
	if strings.HasPrefix(key, "c_att_names_") {
		if idx, err := strconv.Atoi(strings.TrimPrefix(key, "c_att_names_")); err == nil && idx >= 0 && idx < 45 {
			char.CustomNames[idx] = value
			return
		}
	}
	if strings.HasPrefix(key, "c_att_ranks_") {
		if idx, err := strconv.Atoi(strings.TrimPrefix(key, "c_att_ranks_")); err == nil && idx >= 0 && idx < 45 {
			char.CustomRanks[idx] = value
			return
		}
	}
	// c_att_descs_N — per-slot description string seen in legends
	// (monkey-lizard uses it for "Retain momentum from first
	// Grapple" style hints that render under the purchase button).
	if strings.HasPrefix(key, "c_att_descs_") {
		if idx, err := strconv.Atoi(strings.TrimPrefix(key, "c_att_descs_")); err == nil && idx >= 0 && idx < 45 {
			char.CustomDescs[idx] = value
			return
		}
	}

	// Archetype system (customSpec).
	if strings.EqualFold(key, "hasCustomSpec") {
		if n, err := strconv.Atoi(value); err == nil {
			char.HasCustomSpec = n
		}
		return
	}
	if strings.HasPrefix(key, "customSpecName_") {
		if idx, err := strconv.Atoi(strings.TrimPrefix(key, "customSpecName_")); err == nil && idx >= 0 && idx < 3 {
			// Wiki uses 1-based indexing (customSpecName_1 is spec 1).
			// Normalize to 0-based internally by decrementing, but
			// guard against customSpecName_0 in case any file uses 0.
			if idx > 0 {
				idx--
			}
			char.CustomSpecNames[idx] = value
			return
		}
	}
	if strings.HasPrefix(key, "customSpecIcon_") {
		if idx, err := strconv.Atoi(strings.TrimPrefix(key, "customSpecIcon_")); err == nil && idx >= 0 && idx < 3 {
			if idx > 0 {
				idx--
			}
			char.CustomSpecIcons[idx] = value
			return
		}
	}

	// Handle Rank Attributes (e.g. rankHealth)
	if strings.HasPrefix(strings.ToLower(key), "rank") {
		char.RankAttributes[key] = value
		return
	}

	// Normalized to lowercase for robust matching
	switch strings.ToLower(key) {
	case "name":
		char.Name = value
	case "mbclass":
		char.MBClass = value
	case "model":
		char.Model = value
	case "skin":
		char.Skin = value
	case "uishader":
		char.UIShader = value
	case "soundset":
		char.Soundset = value
	case "weapons":
		char.Weapons = value
	case "attributes":
		char.Attributes = value
	case "forcepowers":
		char.ForcePowers = value
	case "saberstyle":
		char.SaberStyle = value
	case "classflags":
		char.ClassFlags = value
	case "maxhealth":
		char.MaxHealth, _ = strconv.Atoi(value)
	case "maxarmor":
		char.MaxArmor, _ = strconv.Atoi(value)
	case "forcepool":
		char.ForcePool, _ = strconv.Atoi(value)
	case "forceregen":
		char.ForceRegen, _ = strconv.ParseFloat(value, 64)
	case "speed":
		char.Speed, _ = strconv.ParseFloat(value, 64)
	case "basespeed":
		char.Speed, _ = strconv.ParseFloat(value, 64) // Alias for Speed
	case "apmultiplier":
		char.APMultiplier, _ = strconv.ParseFloat(value, 64)
	case "bpmultiplier":
		char.BPMultiplier, _ = strconv.ParseFloat(value, 64)
	case "csmultiplier":
		char.CSMultiplier, _ = strconv.ParseFloat(value, 64)
	case "asmultiplier":
		char.ASMultiplier, _ = strconv.ParseFloat(value, 64)
	case "saber1":
		char.Saber1 = value
	case "saber2":
		char.Saber2 = value
	case "sabercolor":
		char.SaberColor, _ = strconv.Atoi(value)
	case "saber2color":
		char.Saber2Color, _ = strconv.Atoi(value)
	case "classnumberlimit":
		char.ClassNumberLimit, _ = strconv.Atoi(value)
	case "respawncustomtime":
		char.RespawnCustomTime, _ = strconv.Atoi(value)
	case "extralives":
		char.ExtraLives, _ = strconv.Atoi(value)
	case "iscustombuild":
		// Parse the actual int — earlier code unconditionally set
		// IsCustomBuild=1 on any presence of the key, which made
		// every loaded MBCH show the Custom Build toggle as ON in
		// the Context tab regardless of the file's `iscustombuild 0`.
		char.IsCustomBuild, _ = strconv.Atoi(value)
	case "mbpoints":
		char.MBPoints, _ = strconv.Atoi(value)
	default:
		char.ExtraFields[key] = value
	}
}

// GenerateMBCH generates the string content for an MBCH file
func GenerateMBCH(char *MBCHCharacter) (string, error) {
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

		// Archetype ("customSpec") header. hasCustomSpec = number of
		// archetypes (1-3). When > 1, customSpecName_N/customSpecIcon_N
		// define each archetype's in-menu tab. Wiki uses 1-based
		// indexing (customSpecName_1 is the first spec) so we emit
		// with +1 offset and skip empty slots.
		if char.HasCustomSpec > 1 {
			fmt.Fprintf(&sb, "\thasCustomSpec\t\t%d\n", char.HasCustomSpec)
			for i := 0; i < char.HasCustomSpec && i < 3; i++ {
				if name := char.CustomSpecNames[i]; name != "" {
					fmt.Fprintf(&sb, "\tcustomSpecName_%d\t\"%s\"\n", i+1, name)
				}
				if icon := char.CustomSpecIcons[i]; icon != "" {
					fmt.Fprintf(&sb, "\tcustomSpecIcon_%d\t\"%s\"\n", i+1, icon)
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
		if wi.FlashSound0 != "" {
			fmt.Fprintf(&sb, "\tFlashSound0\t\t\"%s\"\n", wi.FlashSound0)
		}
		if wi.AltFlashSound0 != "" {
			fmt.Fprintf(&sb, "\tAltFlashSound0\t\t\"%s\"\n", wi.AltFlashSound0)
		}
		if wi.ChargeSound != "" {
			fmt.Fprintf(&sb, "\tChargeSound\t\t\"%s\"\n", wi.ChargeSound)
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
