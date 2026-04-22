package parsers

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

type BladeInfo struct {
	Color  string
	Length float64
	Radius float64
}

type SaberData struct {
	Name               string
	FullName           string
	SaberType          string
	SaberModel         string
	CustomSkin         string
	NumBlades          int
	Blades             []BladeInfo
	SoundOn            string
	SoundOff           string
	SoundLoop          string
	SpinSound          string
	SwingSound1        string
	SwingSound2        string
	SwingSound3        string
	FallSound1         string
	FallSound2         string
	FallSound3         string
	HitSound1          string
	HitSound2          string
	HitSound3          string
	BlockSound1        string
	BlockSound2        string
	BlockSound3        string
	BounceSound1       string
	BounceSound2       string
	BounceSound3       string
	SaberStyle         string
	SingleBladeStyle   string
	MaxChain           int
	LockBonus          int
	ParryBonus         int
	BreakParryBonus    int
	DisarmBonus        int
	MoveSpeedScale     float64
	AnimSpeedScale     float64
	DamageScale        float64
	KnockbackScale     float64
	TrailStyle         int
	BlockEffect        string
	HitPersonEffect    string
	BladeEffect        string
	HitOtherEffect     string
	NoWallMarks        bool
	NoDlight           bool
	NoBlade            bool
	NoClashFlare       bool
	NoDismemberment    bool
	NoIdleEffect       bool
	AlwaysBlock        bool
	NoManualDeactivate bool
	TransitionDamage   bool
	NotInOpen          bool
	NotInMP            bool
	NoCartwheels       bool
	Throwable          bool
	Disarmable         bool
	BlasterBlocking    bool
	OnInWater          bool
	BounceOnWalls      bool
	TwoHanded          bool
	UseGoreConfig      bool
	UseGoreConfig2     bool
	NoDismemberment2   bool
	NoBladeEffects     bool
	NoBladeEffects2    bool
	G2MarksShader      string
	G2WeaponMarkShader string
	SlapAnim           string
	ReadyAnim          string
	JumpAtkUpMove      string
	JumpAtkFwdMove     string
	LungeAtkMove       string
	ExtraFields        map[string]string
	SaberFlagMap       map[string]bool
}

func NewSaberData() *SaberData {
	return &SaberData{
		SaberType:      "SABER_SINGLE",
		NumBlades:      1,
		MoveSpeedScale: 1.0,
		AnimSpeedScale: 1.0,
		DamageScale:    1.0,
		Blades:         []BladeInfo{{Color: "blue", Length: 32.0, Radius: 3.0}},
		ExtraFields:    make(map[string]string),
		SaberFlagMap:   make(map[string]bool),
	}
}

// ParseSAB parses the content of a SAB file
func ParseSAB(content string) (*SaberData, error) {
	saber := NewSaberData()

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

	// Extract Main Block: Name { ... }
	re := regexp.MustCompile(`(?is)(\w+)\s*\{([^}]+)\}`)
	match := re.FindStringSubmatch(cleanContent)

	if len(match) > 2 {
		saber.Name = match[1]
		parseSaberBlock(match[2], saber)
	} else {
		return nil, fmt.Errorf("no valid saber block found")
	}

	return saber, nil
}

func parseSaberBlock(block string, saber *SaberData) {
	for _, line := range strings.Split(block, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// robust parsing
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

		setSaberField(saber, strings.ToLower(key), value)
	}
}

func setSaberField(saber *SaberData, key, value string) {
	switch key {
	case "name":
		saber.FullName = value
	case "sabertype":
		saber.SaberType = strings.ToUpper(value)
	case "sabermodel":
		saber.SaberModel = value
	case "customskin":
		saber.CustomSkin = value
	case "numblades":
		saber.NumBlades, _ = strconv.Atoi(value)
		for len(saber.Blades) < saber.NumBlades {
			saber.Blades = append(saber.Blades, BladeInfo{Color: "blue", Length: 32.0, Radius: 3.0})
		}
	case "sabercolor":
		if len(saber.Blades) > 0 {
			saber.Blades[0].Color = value
		}
	case "saberlength":
		if len(saber.Blades) > 0 {
			saber.Blades[0].Length, _ = strconv.ParseFloat(value, 64)
		}
	case "saberradius":
		if len(saber.Blades) > 0 {
			saber.Blades[0].Radius, _ = strconv.ParseFloat(value, 64)
		}

	// Handle numbered blades
	case "sabercolor1", "sabercolor2", "sabercolor3", "sabercolor4", "sabercolor5", "sabercolor6", "sabercolor7", "sabercolor8":
		idx, _ := strconv.Atoi(key[10:])
		idx--
		for len(saber.Blades) <= idx {
			saber.Blades = append(saber.Blades, BladeInfo{Color: "blue", Length: 32.0, Radius: 3.0})
		}
		if idx >= 0 && idx < len(saber.Blades) {
			saber.Blades[idx].Color = value
		}
	case "saberlength1", "saberlength2", "saberlength3", "saberlength4", "saberlength5", "saberlength6", "saberlength7", "saberlength8":
		idx, _ := strconv.Atoi(key[11:])
		idx--
		for len(saber.Blades) <= idx {
			saber.Blades = append(saber.Blades, BladeInfo{Color: "blue", Length: 32.0, Radius: 3.0})
		}
		if idx >= 0 && idx < len(saber.Blades) {
			saber.Blades[idx].Length, _ = strconv.ParseFloat(value, 64)
		}
	case "saberradius1", "saberradius2", "saberradius3", "saberradius4", "saberradius5", "saberradius6", "saberradius7", "saberradius8":
		idx, _ := strconv.Atoi(key[11:])
		idx--
		for len(saber.Blades) <= idx {
			saber.Blades = append(saber.Blades, BladeInfo{Color: "blue", Length: 32.0, Radius: 3.0})
		}
		if idx >= 0 && idx < len(saber.Blades) {
			saber.Blades[idx].Radius, _ = strconv.ParseFloat(value, 64)
		}

	case "soundon":
		saber.SoundOn = value
	case "soundoff":
		saber.SoundOff = value
	case "soundloop":
		saber.SoundLoop = value
	case "spinsound":
		saber.SpinSound = value
	case "swingsound1":
		saber.SwingSound1 = value
	case "swingsound2":
		saber.SwingSound2 = value
	case "swingsound3":
		saber.SwingSound3 = value
	case "fallsound1":
		saber.FallSound1 = value
	case "fallsound2":
		saber.FallSound2 = value
	case "fallsound3":
		saber.FallSound3 = value
	case "hitsound1":
		saber.HitSound1 = value
	case "hitsound2":
		saber.HitSound2 = value
	case "hitsound3":
		saber.HitSound3 = value
	case "blocksound1":
		saber.BlockSound1 = value
	case "blocksound2":
		saber.BlockSound2 = value
	case "blocksound3":
		saber.BlockSound3 = value
	case "bouncesound1":
		saber.BounceSound1 = value
	case "bouncesound2":
		saber.BounceSound2 = value
	case "bouncesound3":
		saber.BounceSound3 = value

	case "saberstyle":
		saber.SaberStyle = value
	case "singlebladestyle":
		saber.SingleBladeStyle = value
	case "maxchain":
		saber.MaxChain, _ = strconv.Atoi(value)
	case "lockbonus":
		saber.LockBonus, _ = strconv.Atoi(value)
	case "parrybonus":
		saber.ParryBonus, _ = strconv.Atoi(value)
	case "breakparrybonus":
		saber.BreakParryBonus, _ = strconv.Atoi(value)
	case "disarmbonus":
		saber.DisarmBonus, _ = strconv.Atoi(value)
	case "movespeedscale":
		saber.MoveSpeedScale, _ = strconv.ParseFloat(value, 64)
	case "animspeedscale":
		saber.AnimSpeedScale, _ = strconv.ParseFloat(value, 64)
	case "damagescale":
		saber.DamageScale, _ = strconv.ParseFloat(value, 64)
	case "knockbackscale":
		saber.KnockbackScale, _ = strconv.ParseFloat(value, 64)
	case "trailstyle":
		saber.TrailStyle, _ = strconv.Atoi(value)
	case "blockeffect":
		saber.BlockEffect = value
	case "hitpersoneffect":
		saber.HitPersonEffect = value
	case "bladeeffect":
		saber.BladeEffect = value
	case "hitothereffect":
		saber.HitOtherEffect = value

	// Boolean Flags
	case "nowallmarks":
		saber.NoWallMarks = value == "1"
	case "nodlight":
		saber.NoDlight = value == "1"
	case "noblade":
		saber.NoBlade = value == "1"
	case "noclashflare":
		saber.NoClashFlare = value == "1"
	case "nodismemberment":
		saber.NoDismemberment = value == "1"
	case "noidleeffect":
		saber.NoIdleEffect = value == "1"
	case "alwaysblock":
		saber.AlwaysBlock = value == "1"
	case "nomanualdeactivate":
		saber.NoManualDeactivate = value == "1"
	case "transitiondamage":
		saber.TransitionDamage = value == "1"
	case "notinopen":
		saber.NotInOpen = value == "1"
	case "notinmp":
		saber.NotInMP = value == "1"
	case "nocartwheels":
		saber.NoCartwheels = value == "1"
	case "throwable":
		saber.Throwable = value == "1"
	case "disarmable":
		saber.Disarmable = value == "1"
	case "blasterblocking":
		saber.BlasterBlocking = value == "1"
	case "oninwater":
		saber.OnInWater = value == "1"
	case "bounceonwalls":
		saber.BounceOnWalls = value == "1"
	case "twohanded":
		saber.TwoHanded = value == "1"
	case "usegoreconfig":
		saber.UseGoreConfig = value == "1"
	case "usegoreconfig2":
		saber.UseGoreConfig2 = value == "1"
	case "nodismemberment2":
		saber.NoDismemberment2 = value == "1"
	case "nobladeeffects":
		saber.NoBladeEffects = value == "1"
	case "nobladeeffects2":
		saber.NoBladeEffects2 = value == "1"

	case "g2marksshader":
		saber.G2MarksShader = value
	case "g2weaponmarkshader":
		saber.G2WeaponMarkShader = value
	case "slapanim":
		saber.SlapAnim = value
	case "readyanim":
		saber.ReadyAnim = value
	case "jumpatkupmove":
		saber.JumpAtkUpMove = value
	case "jumpatkfwdmove":
		saber.JumpAtkFwdMove = value
	case "lungeatkmove":
		saber.LungeAtkMove = value

	default:
		saber.ExtraFields[key] = value
	}
}

func GenerateSAB(saber *SaberData) (string, error) {
	var sb strings.Builder

	fmt.Fprintf(&sb, "%s\n{\n", saber.Name)
	if saber.FullName != "" {
		fmt.Fprintf(&sb, "\tname\t\t\t\"%s\"\n", saber.FullName)
	}
	fmt.Fprintf(&sb, "\tsaberType\t\t%s\n", saber.SaberType)
	if saber.SaberModel != "" {
		fmt.Fprintf(&sb, "\tsaberModel\t\t\"%s\"\n", saber.SaberModel)
	}
	if saber.CustomSkin != "" {
		fmt.Fprintf(&sb, "\tcustomSkin\t\t\"%s\"\n", saber.CustomSkin)
	}
	if saber.NumBlades > 1 {
		fmt.Fprintf(&sb, "\tnumBlades\t\t%d\n", saber.NumBlades)
	}

	if len(saber.Blades) > 0 {
		fmt.Fprintf(&sb, "\tsaberColor\t\t%s\n", saber.Blades[0].Color)
		fmt.Fprintf(&sb, "\tsaberLength\t\t%.1f\n", saber.Blades[0].Length)
		fmt.Fprintf(&sb, "\tsaberRadius\t\t%.1f\n", saber.Blades[0].Radius)
	}

	// Sounds
	if saber.SoundOn != "" {
		fmt.Fprintf(&sb, "\tsoundOn\t\t\t\"%s\"\n", saber.SoundOn)
	}
	if saber.SoundOff != "" {
		fmt.Fprintf(&sb, "\tsoundOff\t\t\"%s\"\n", saber.SoundOff)
	}
	if saber.SoundLoop != "" {
		fmt.Fprintf(&sb, "\tsoundLoop\t\t\"%s\"\n", saber.SoundLoop)
	}
	if saber.SpinSound != "" {
		fmt.Fprintf(&sb, "\tspinSound\t\t\"%s\"\n", saber.SpinSound)
	}
	if saber.SwingSound1 != "" {
		fmt.Fprintf(&sb, "\tswingSound1\t\t\"%s\"\n", saber.SwingSound1)
	}
	if saber.SwingSound2 != "" {
		fmt.Fprintf(&sb, "\tswingSound2\t\t\"%s\"\n", saber.SwingSound2)
	}
	if saber.SwingSound3 != "" {
		fmt.Fprintf(&sb, "\tswingSound3\t\t\"%s\"\n", saber.SwingSound3)
	}
	if saber.FallSound1 != "" {
		fmt.Fprintf(&sb, "\tfallSound1\t\t\"%s\"\n", saber.FallSound1)
	}
	if saber.FallSound2 != "" {
		fmt.Fprintf(&sb, "\tfallSound2\t\t\"%s\"\n", saber.FallSound2)
	}
	if saber.FallSound3 != "" {
		fmt.Fprintf(&sb, "\tfallSound3\t\t\"%s\"\n", saber.FallSound3)
	}
	if saber.HitSound1 != "" {
		fmt.Fprintf(&sb, "\thitSound1\t\t\"%s\"\n", saber.HitSound1)
	}
	if saber.HitSound2 != "" {
		fmt.Fprintf(&sb, "\thitSound2\t\t\"%s\"\n", saber.HitSound2)
	}
	if saber.HitSound3 != "" {
		fmt.Fprintf(&sb, "\thitSound3\t\t\"%s\"\n", saber.HitSound3)
	}
	if saber.BlockSound1 != "" {
		fmt.Fprintf(&sb, "\tblockSound1\t\t\"%s\"\n", saber.BlockSound1)
	}
	if saber.BlockSound2 != "" {
		fmt.Fprintf(&sb, "\tblockSound2\t\t\"%s\"\n", saber.BlockSound2)
	}
	if saber.BlockSound3 != "" {
		fmt.Fprintf(&sb, "\tblockSound3\t\t\"%s\"\n", saber.BlockSound3)
	}
	if saber.BounceSound1 != "" {
		fmt.Fprintf(&sb, "\tbounceSound1\t\t\"%s\"\n", saber.BounceSound1)
	}
	if saber.BounceSound2 != "" {
		fmt.Fprintf(&sb, "\tbounceSound2\t\t\"%s\"\n", saber.BounceSound2)
	}
	if saber.BounceSound3 != "" {
		fmt.Fprintf(&sb, "\tbounceSound3\t\t\"%s\"\n", saber.BounceSound3)
	}

	// Combat
	if saber.SaberStyle != "" {
		fmt.Fprintf(&sb, "\tsaberStyle\t\t%s\n", saber.SaberStyle)
	}
	if saber.SingleBladeStyle != "" {
		fmt.Fprintf(&sb, "\tsingleBladeStyle\t%s\n", saber.SingleBladeStyle)
	}
	if saber.MaxChain > 0 {
		fmt.Fprintf(&sb, "\tmaxChain\t\t%d\n", saber.MaxChain)
	}
	if saber.LockBonus > 0 {
		fmt.Fprintf(&sb, "\tlockBonus\t\t%d\n", saber.LockBonus)
	}
	if saber.ParryBonus > 0 {
		fmt.Fprintf(&sb, "\tparryBonus\t\t%d\n", saber.ParryBonus)
	}
	if saber.BreakParryBonus > 0 {
		fmt.Fprintf(&sb, "\tbreakParryBonus\t\t%d\n", saber.BreakParryBonus)
	}
	if saber.DisarmBonus > 0 {
		fmt.Fprintf(&sb, "\tdisarmBonus\t\t%d\n", saber.DisarmBonus)
	}
	if saber.MoveSpeedScale != 1.0 {
		fmt.Fprintf(&sb, "\tmoveSpeedScale\t\t%.2f\n", saber.MoveSpeedScale)
	}
	if saber.AnimSpeedScale != 1.0 {
		fmt.Fprintf(&sb, "\tanimSpeedScale\t\t%.2f\n", saber.AnimSpeedScale)
	}
	if saber.DamageScale != 1.0 {
		fmt.Fprintf(&sb, "\tdamageScale\t\t%.2f\n", saber.DamageScale)
	}
	if saber.KnockbackScale != 0.0 {
		fmt.Fprintf(&sb, "\tknockbackScale\t\t%.2f\n", saber.KnockbackScale)
	} // 0.0 default?

	// Effects
	if saber.TrailStyle > 0 {
		fmt.Fprintf(&sb, "\ttrailStyle\t\t%d\n", saber.TrailStyle)
	}
	if saber.BlockEffect != "" {
		fmt.Fprintf(&sb, "\tblockEffect\t\t\"%s\"\n", saber.BlockEffect)
	}
	if saber.HitPersonEffect != "" {
		fmt.Fprintf(&sb, "\thitPersonEffect\t\t\"%s\"\n", saber.HitPersonEffect)
	}
	if saber.BladeEffect != "" {
		fmt.Fprintf(&sb, "\tbladeEffect\t\t\"%s\"\n", saber.BladeEffect)
	}
	if saber.HitOtherEffect != "" {
		fmt.Fprintf(&sb, "\thitOtherEffect\t\t\"%s\"\n", saber.HitOtherEffect)
	}

	// Boolean Flags
	if saber.NoWallMarks {
		fmt.Fprintf(&sb, "\tnoWallMarks\t\t1\n")
	}
	if saber.NoDlight {
		fmt.Fprintf(&sb, "\tnoDlight\t\t1\n")
	}
	if saber.NoBlade {
		fmt.Fprintf(&sb, "\tnoBlade\t\t\t1\n")
	}
	if saber.NoClashFlare {
		fmt.Fprintf(&sb, "\tnoClashFlare\t\t1\n")
	}
	if saber.NoDismemberment {
		fmt.Fprintf(&sb, "\tnoDismemberment\t\t1\n")
	}
	if saber.NoIdleEffect {
		fmt.Fprintf(&sb, "\tnoIdleEffect\t\t1\n")
	}
	if saber.AlwaysBlock {
		fmt.Fprintf(&sb, "\talwaysBlock\t\t1\n")
	}
	if saber.NoManualDeactivate {
		fmt.Fprintf(&sb, "\tnoManualDeactivate\t1\n")
	}
	if saber.TransitionDamage {
		fmt.Fprintf(&sb, "\ttransitionDamage\t1\n")
	}
	if saber.NotInOpen {
		fmt.Fprintf(&sb, "\tnotinOpen\t\t1\n")
	}
	if saber.NotInMP {
		fmt.Fprintf(&sb, "\tnotInMP\t\t\t1\n")
	}
	if saber.NoCartwheels {
		fmt.Fprintf(&sb, "\tnoCartwheels\t\t1\n")
	}
	if saber.Throwable {
		fmt.Fprintf(&sb, "\tthrowable\t\t1\n")
	}
	if saber.Disarmable {
		fmt.Fprintf(&sb, "\tdisarmable\t\t1\n")
	}
	if saber.BlasterBlocking {
		fmt.Fprintf(&sb, "\tblasterBlocking\t\t1\n")
	}
	if saber.OnInWater {
		fmt.Fprintf(&sb, "\tonInWater\t\t1\n")
	}
	if saber.BounceOnWalls {
		fmt.Fprintf(&sb, "\tbounceOnWalls\t\t1\n")
	}
	if saber.TwoHanded {
		fmt.Fprintf(&sb, "\ttwoHanded\t\t1\n")
	}
	if saber.UseGoreConfig {
		fmt.Fprintf(&sb, "\tuseGoreConfig\t\t1\n")
	}

	writeExtraFields(&sb, saber.ExtraFields)

	fmt.Fprintln(&sb, "}")
	return sb.String(), nil
}
