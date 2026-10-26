package parsers

import (
	"fmt"
	"strconv"
	"strings"
)

type BladeInfo struct {
	Color  string
	Length float64
	Radius float64
}

type SaberData struct {
	ctx *sourceContext

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
	return ParseSABDefinition(content, 0)
}
func parseSaberBool(value string) bool {
	n, err := strconv.Atoi(value)
	return err == nil && n != 0
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
		for i := range saber.Blades {
			saber.Blades[i].Color = value
		}
	case "saberlength":
		if parsed, err := strconv.ParseFloat(value, 64); err == nil {
			for i := range saber.Blades {
				saber.Blades[i].Length = parsed
			}
		}
	case "saberradius":
		if parsed, err := strconv.ParseFloat(value, 64); err == nil {
			for i := range saber.Blades {
				saber.Blades[i].Radius = parsed
			}
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

	// Boolean flags are accumulated by the engine: each non-zero
	// occurrence sets a bit, while a later zero never clears it.
	case "nowallmarks":
		saber.NoWallMarks = saber.NoWallMarks || parseSaberBool(value)
	case "nodlight":
		saber.NoDlight = saber.NoDlight || parseSaberBool(value)
	case "noblade":
		saber.NoBlade = saber.NoBlade || parseSaberBool(value)
	case "noclashflare":
		saber.NoClashFlare = saber.NoClashFlare || parseSaberBool(value)
	case "nodismemberment":
		saber.NoDismemberment = saber.NoDismemberment || parseSaberBool(value)
	case "noidleeffect":
		saber.NoIdleEffect = saber.NoIdleEffect || parseSaberBool(value)
	case "alwaysblock":
		saber.AlwaysBlock = saber.AlwaysBlock || parseSaberBool(value)
	case "nomanualdeactivate":
		saber.NoManualDeactivate = saber.NoManualDeactivate || parseSaberBool(value)
	case "transitiondamage":
		saber.TransitionDamage = saber.TransitionDamage || parseSaberBool(value)
	case "notinopen":
		saber.NotInOpen = saber.NotInOpen || parseSaberBool(value)
	case "notinmp":
		saber.NotInMP = saber.NotInMP || parseSaberBool(value)
	case "nocartwheels":
		saber.NoCartwheels = saber.NoCartwheels || parseSaberBool(value)
	case "throwable":
		saber.Throwable = saber.Throwable || parseSaberBool(value)
	case "disarmable":
		saber.Disarmable = saber.Disarmable || parseSaberBool(value)
	case "blasterblocking":
		saber.BlasterBlocking = saber.BlasterBlocking || parseSaberBool(value)
	case "oninwater":
		saber.OnInWater = saber.OnInWater || parseSaberBool(value)
	case "bounceonwalls":
		saber.BounceOnWalls = saber.BounceOnWalls || parseSaberBool(value)
	case "twohanded":
		saber.TwoHanded = saber.TwoHanded || parseSaberBool(value)
	case "usegoreconfig":
		saber.UseGoreConfig = saber.UseGoreConfig || parseSaberBool(value)
	case "usegoreconfig2":
		saber.UseGoreConfig2 = saber.UseGoreConfig2 || parseSaberBool(value)
	case "nodismemberment2":
		saber.NoDismemberment2 = saber.NoDismemberment2 || parseSaberBool(value)
	case "nobladeeffects":
		saber.NoBladeEffects = saber.NoBladeEffects || parseSaberBool(value)
	case "nobladeeffects2":
		saber.NoBladeEffects2 = saber.NoBladeEffects2 || parseSaberBool(value)

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
	if saber.ctx != nil && saber.ctx.doc != nil {
		// Sync into a deep clone — the caller's retained parse baseline
		// stays pristine (see GenerateMBCH).
		doc := cloneASTDocument(saber.ctx.doc)
		syncSaberToAST(saber, doc.Nodes[saber.ctx.blockIndex].(*ASTBlock))
		return doc.String(), nil
	}
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
		first := saber.Blades[0]
		fmt.Fprintf(&sb, "\tsaberColor\t\t%s\n", first.Color)
		fmt.Fprintf(&sb, "\tsaberLength\t\t%s\n", strconv.FormatFloat(first.Length, 'f', -1, 64))
		fmt.Fprintf(&sb, "\tsaberRadius\t\t%s\n", strconv.FormatFloat(first.Radius, 'f', -1, 64))
		for i := 1; i < len(saber.Blades) && i < saber.NumBlades; i++ {
			blade := saber.Blades[i]
			if blade.Color != first.Color {
				fmt.Fprintf(&sb, "\tsaberColor%d\t\t%s\n", i+1, blade.Color)
			}
			if blade.Length != first.Length {
				fmt.Fprintf(&sb, "\tsaberLength%d\t\t%s\n", i+1, strconv.FormatFloat(blade.Length, 'f', -1, 64))
			}
			if blade.Radius != first.Radius {
				fmt.Fprintf(&sb, "\tsaberRadius%d\t\t%s\n", i+1, strconv.FormatFloat(blade.Radius, 'f', -1, 64))
			}
		}
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
		fmt.Fprintf(&sb, "\tmoveSpeedScale\t\t%s\n", strconv.FormatFloat(saber.MoveSpeedScale, 'f', -1, 64))
	}
	if saber.AnimSpeedScale != 1.0 {
		fmt.Fprintf(&sb, "\tanimSpeedScale\t\t%s\n", strconv.FormatFloat(saber.AnimSpeedScale, 'f', -1, 64))
	}
	if saber.DamageScale != 1.0 {
		fmt.Fprintf(&sb, "\tdamageScale\t\t%s\n", strconv.FormatFloat(saber.DamageScale, 'f', -1, 64))
	}
	if saber.KnockbackScale != 0.0 {
		fmt.Fprintf(&sb, "\tknockbackScale\t\t%s\n", strconv.FormatFloat(saber.KnockbackScale, 'f', -1, 64))
	}

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
	if saber.G2MarksShader != "" {
		fmt.Fprintf(&sb, "\tg2MarksShader\t\t\"%s\"\n", saber.G2MarksShader)
	}
	if saber.G2WeaponMarkShader != "" {
		fmt.Fprintf(&sb, "\tg2WeaponMarkShader\t\"%s\"\n", saber.G2WeaponMarkShader)
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
	if saber.UseGoreConfig2 {
		fmt.Fprintf(&sb, "\tuseGoreConfig2\t\t1\n")
	}
	if saber.NoDismemberment2 {
		fmt.Fprintf(&sb, "\tnoDismemberment2\t1\n")
	}
	if saber.NoBladeEffects {
		fmt.Fprintf(&sb, "\tnoBladeEffects\t\t1\n")
	}
	if saber.NoBladeEffects2 {
		fmt.Fprintf(&sb, "\tnoBladeEffects2\t\t1\n")
	}

	// Animation overrides
	if saber.SlapAnim != "" {
		fmt.Fprintf(&sb, "\tslapAnim\t\t%s\n", saber.SlapAnim)
	}
	if saber.ReadyAnim != "" {
		fmt.Fprintf(&sb, "\treadyAnim\t\t%s\n", saber.ReadyAnim)
	}
	if saber.JumpAtkUpMove != "" {
		fmt.Fprintf(&sb, "\tjumpAtkUpMove\t\t%s\n", saber.JumpAtkUpMove)
	}
	if saber.JumpAtkFwdMove != "" {
		fmt.Fprintf(&sb, "\tjumpAtkFwdMove\t\t%s\n", saber.JumpAtkFwdMove)
	}
	if saber.LungeAtkMove != "" {
		fmt.Fprintf(&sb, "\tlungeAtkMove\t\t%s\n", saber.LungeAtkMove)
	}

	writeExtraFields(&sb, saber.ExtraFields)

	fmt.Fprintln(&sb, "}")
	return sb.String(), nil
}
