package parsers

import "fmt"
import "strconv"
import "strings"

func syncSaberToAST(saber *SaberData, block *ASTBlock) {
	if block.NameToken != nil {
		block.NameToken.Text = saber.Name
	}

	baseline := NewSaberData()
	readSaberBlockFields(baseline, block)

	lastFieldKey := func(keys ...string) string {
		wanted := make(map[string]bool, len(keys))
		for _, key := range keys {
			wanted[strings.ToLower(key)] = true
		}
		last := ""
		walkTokenPairs(block.Children, func(_ int, key string, _ int, _ string, hasVal bool) bool {
			if hasVal && wanted[strings.ToLower(key)] {
				last = key
			}
			return true
		})
		return last
	}

	setEffective := func(key, value string, quote bool) {
		lastValueIdx := -1
		walkTokenPairs(block.Children, func(_ int, candidate string, valueIdx int, _ string, hasVal bool) bool {
			if hasVal && strings.EqualFold(candidate, key) {
				lastValueIdx = valueIdx
			}
			return true
		})
		if lastValueIdx < 0 {
			setFieldValue(block, key, value, quote)
			return
		}
		valueText := value
		if quote {
			valueText = `"` + value + `"`
		} else if value == "" {
			valueText = `""`
		}
		block.Children[lastValueIdx].(*ASTToken).Text = valueText
	}

	syncStr := func(key, value, old, def string, quote bool) {
		if value == old {
			return
		}
		if value == def {
			removeField(block, key)
			return
		}
		effectiveKey := lastFieldKey(key)
		if effectiveKey == "" {
			effectiveKey = key
		}
		setEffective(effectiveKey, value, quote)
	}
	syncInt := func(key string, value, old int) {
		if value == old {
			return
		}
		effectiveKey := lastFieldKey(key)
		if effectiveKey == "" {
			effectiveKey = key
		}
		setEffective(effectiveKey, strconv.Itoa(value), false)
	}
	syncFloat := func(key string, value, old float64) {
		if value == old {
			return
		}
		effectiveKey := lastFieldKey(key)
		if effectiveKey == "" {
			effectiveKey = key
		}
		setEffective(effectiveKey, strconv.FormatFloat(value, 'f', -1, 64), false)
	}
	syncBool := func(key string, value, old bool) {
		if value == old {
			return
		}
		if !value {
			// Engine flag parsers OR every non-zero duplicate into the
			// destination bit. Removing every occurrence is the only
			// representation that cannot leave an earlier true alive.
			removeField(block, key)
			return
		}
		effectiveKey := lastFieldKey(key)
		if effectiveKey == "" {
			effectiveKey = key
		}
		setEffective(effectiveKey, "1", false)
	}

	syncStr("name", saber.FullName, baseline.FullName, "", true)
	syncStr("saberType", saber.SaberType, baseline.SaberType, "SABER_SINGLE", false)
	syncStr("saberModel", saber.SaberModel, baseline.SaberModel, "", true)
	syncStr("customSkin", saber.CustomSkin, baseline.CustomSkin, "", true)
	syncInt("numBlades", saber.NumBlades, baseline.NumBlades)

	baselineBlade := func(index int) BladeInfo {
		if index < len(baseline.Blades) {
			return baseline.Blades[index]
		}
		if len(baseline.Blades) > 0 {
			return baseline.Blades[0]
		}
		return BladeInfo{Color: "blue", Length: 32, Radius: 3}
	}
	syncBlade := func(base string, index int, value, old string) {
		if value == old {
			return
		}
		numbered := fmt.Sprintf("%s%d", base, index+1)
		if index == 0 {
			effectiveKey := lastFieldKey(base, numbered)
			// Updating a bare key would also alter every other blade.
			// Add a blade-1 override instead when multiple blades exist.
			if effectiveKey == "" {
				effectiveKey = base
			} else if strings.EqualFold(effectiveKey, base) && len(saber.Blades) > 1 {
				effectiveKey = numbered
			}
			setEffective(effectiveKey, value, false)
			return
		}
		setEffective(numbered, value, false)
	}
	for i, blade := range saber.Blades {
		old := baselineBlade(i)
		syncBlade("saberColor", i, blade.Color, old.Color)
		syncBlade("saberLength", i, strconv.FormatFloat(blade.Length, 'f', -1, 64), strconv.FormatFloat(old.Length, 'f', -1, 64))
		syncBlade("saberRadius", i, strconv.FormatFloat(blade.Radius, 'f', -1, 64), strconv.FormatFloat(old.Radius, 'f', -1, 64))
	}
	for n := len(saber.Blades) + 1; n <= 8; n++ {
		removeField(block, fmt.Sprintf("saberColor%d", n))
		removeField(block, fmt.Sprintf("saberLength%d", n))
		removeField(block, fmt.Sprintf("saberRadius%d", n))
	}

	syncStr("soundOn", saber.SoundOn, baseline.SoundOn, "", true)
	syncStr("soundOff", saber.SoundOff, baseline.SoundOff, "", true)
	syncStr("soundLoop", saber.SoundLoop, baseline.SoundLoop, "", true)
	syncStr("spinSound", saber.SpinSound, baseline.SpinSound, "", true)
	syncStr("swingSound1", saber.SwingSound1, baseline.SwingSound1, "", true)
	syncStr("swingSound2", saber.SwingSound2, baseline.SwingSound2, "", true)
	syncStr("swingSound3", saber.SwingSound3, baseline.SwingSound3, "", true)
	syncStr("fallSound1", saber.FallSound1, baseline.FallSound1, "", true)
	syncStr("fallSound2", saber.FallSound2, baseline.FallSound2, "", true)
	syncStr("fallSound3", saber.FallSound3, baseline.FallSound3, "", true)
	syncStr("hitSound1", saber.HitSound1, baseline.HitSound1, "", true)
	syncStr("hitSound2", saber.HitSound2, baseline.HitSound2, "", true)
	syncStr("hitSound3", saber.HitSound3, baseline.HitSound3, "", true)
	syncStr("blockSound1", saber.BlockSound1, baseline.BlockSound1, "", true)
	syncStr("blockSound2", saber.BlockSound2, baseline.BlockSound2, "", true)
	syncStr("blockSound3", saber.BlockSound3, baseline.BlockSound3, "", true)
	syncStr("bounceSound1", saber.BounceSound1, baseline.BounceSound1, "", true)
	syncStr("bounceSound2", saber.BounceSound2, baseline.BounceSound2, "", true)
	syncStr("bounceSound3", saber.BounceSound3, baseline.BounceSound3, "", true)

	syncStr("saberStyle", saber.SaberStyle, baseline.SaberStyle, "", false)
	syncStr("singleBladeStyle", saber.SingleBladeStyle, baseline.SingleBladeStyle, "", false)
	syncInt("maxChain", saber.MaxChain, baseline.MaxChain)
	syncInt("lockBonus", saber.LockBonus, baseline.LockBonus)
	syncInt("parryBonus", saber.ParryBonus, baseline.ParryBonus)
	syncInt("breakParryBonus", saber.BreakParryBonus, baseline.BreakParryBonus)
	syncInt("disarmBonus", saber.DisarmBonus, baseline.DisarmBonus)
	syncFloat("moveSpeedScale", saber.MoveSpeedScale, baseline.MoveSpeedScale)
	syncFloat("animSpeedScale", saber.AnimSpeedScale, baseline.AnimSpeedScale)
	syncFloat("damageScale", saber.DamageScale, baseline.DamageScale)
	syncFloat("knockbackScale", saber.KnockbackScale, baseline.KnockbackScale)

	syncInt("trailStyle", saber.TrailStyle, baseline.TrailStyle)
	syncStr("blockEffect", saber.BlockEffect, baseline.BlockEffect, "", true)
	syncStr("hitPersonEffect", saber.HitPersonEffect, baseline.HitPersonEffect, "", true)
	syncStr("bladeEffect", saber.BladeEffect, baseline.BladeEffect, "", true)
	syncStr("hitOtherEffect", saber.HitOtherEffect, baseline.HitOtherEffect, "", true)
	syncStr("g2MarksShader", saber.G2MarksShader, baseline.G2MarksShader, "", true)
	syncStr("g2WeaponMarkShader", saber.G2WeaponMarkShader, baseline.G2WeaponMarkShader, "", true)

	syncBool("noWallMarks", saber.NoWallMarks, baseline.NoWallMarks)
	syncBool("noDlight", saber.NoDlight, baseline.NoDlight)
	syncBool("noBlade", saber.NoBlade, baseline.NoBlade)
	syncBool("noClashFlare", saber.NoClashFlare, baseline.NoClashFlare)
	syncBool("noDismemberment", saber.NoDismemberment, baseline.NoDismemberment)
	syncBool("noIdleEffect", saber.NoIdleEffect, baseline.NoIdleEffect)
	syncBool("alwaysBlock", saber.AlwaysBlock, baseline.AlwaysBlock)
	syncBool("noManualDeactivate", saber.NoManualDeactivate, baseline.NoManualDeactivate)
	syncBool("transitionDamage", saber.TransitionDamage, baseline.TransitionDamage)
	syncBool("notinOpen", saber.NotInOpen, baseline.NotInOpen)
	syncBool("notInMP", saber.NotInMP, baseline.NotInMP)
	syncBool("noCartwheels", saber.NoCartwheels, baseline.NoCartwheels)
	syncBool("throwable", saber.Throwable, baseline.Throwable)
	syncBool("disarmable", saber.Disarmable, baseline.Disarmable)
	syncBool("blasterBlocking", saber.BlasterBlocking, baseline.BlasterBlocking)
	syncBool("onInWater", saber.OnInWater, baseline.OnInWater)
	syncBool("bounceOnWalls", saber.BounceOnWalls, baseline.BounceOnWalls)
	syncBool("twoHanded", saber.TwoHanded, baseline.TwoHanded)
	syncBool("useGoreConfig", saber.UseGoreConfig, baseline.UseGoreConfig)
	syncBool("useGoreConfig2", saber.UseGoreConfig2, baseline.UseGoreConfig2)
	syncBool("noDismemberment2", saber.NoDismemberment2, baseline.NoDismemberment2)
	syncBool("noBladeEffects", saber.NoBladeEffects, baseline.NoBladeEffects)
	syncBool("noBladeEffects2", saber.NoBladeEffects2, baseline.NoBladeEffects2)

	syncStr("slapAnim", saber.SlapAnim, baseline.SlapAnim, "", false)
	syncStr("readyAnim", saber.ReadyAnim, baseline.ReadyAnim, "", false)
	syncStr("jumpAtkUpMove", saber.JumpAtkUpMove, baseline.JumpAtkUpMove, "", false)
	syncStr("jumpAtkFwdMove", saber.JumpAtkFwdMove, baseline.JumpAtkFwdMove, "", false)
	syncStr("lungeAtkMove", saber.LungeAtkMove, baseline.LungeAtkMove, "", false)

	removeStaleExtrasSeq(block, saber.ExtraFields, saberFieldTyped)
	for _, key := range sortedKeys(saber.ExtraFields) {
		value := saber.ExtraFields[key]
		if old, ok := baseline.ExtraFields[key]; ok && old == value {
			continue
		}
		effectiveKey := lastFieldKey(key)
		if effectiveKey == "" {
			effectiveKey = key
		}
		setEffective(effectiveKey, value, strings.ContainsAny(value, " \t\n"))
	}
}

func readSaberBlockFields(saber *SaberData, block *ASTBlock) {
	walkTokenPairs(block.Children, func(_ int, key string, _ int, value string, hasValue bool) bool {
		if !hasValue {
			return true
		}
		if saberFieldTyped(key) {
			setSaberField(saber, strings.ToLower(key), value)
		} else {
			saber.ExtraFields[key] = value
		}
		return true
	})
}

// saberFieldTyped reports whether a .sab key is modeled natively by
// setSaberField (including numbered blade keys). Typed keys are never
// stale-removed during AST sync.
func saberFieldTyped(key string) bool {
	k := strings.ToLower(key)
	switch k {
	case "name", "sabertype", "sabermodel", "customskin", "numblades",
		"soundon", "soundoff", "soundloop", "spinsound",
		"swingsound1", "swingsound2", "swingsound3",
		"fallsound1", "fallsound2", "fallsound3",
		"hitsound1", "hitsound2", "hitsound3",
		"blocksound1", "blocksound2", "blocksound3",
		"bouncesound1", "bouncesound2", "bouncesound3",
		"saberstyle", "singlebladestyle",
		"maxchain", "lockbonus", "parrybonus", "breakparrybonus", "disarmbonus",
		"movespeedscale", "animspeedscale", "damagescale", "knockbackscale",
		"trailstyle", "blockeffect", "hitpersoneffect", "bladeeffect", "hitothereffect",
		"nowallmarks", "nodlight", "noblade", "noclashflare", "nodismemberment",
		"noidleeffect", "alwaysblock", "nomanualdeactivate", "transitiondamage",
		"notinopen", "notinmp", "nocartwheels", "throwable", "disarmable",
		"blasterblocking", "oninwater", "bounceonwalls", "twohanded",
		"usegoreconfig", "usegoreconfig2", "nodismemberment2",
		"nobladeeffects", "nobladeeffects2",
		"g2marksshader", "g2weaponmarkshader", "slapanim", "readyanim",
		"jumpatkupmove", "jumpatkfwdmove", "lungeatkmove":
		return true
	}
	for _, base := range []string{"sabercolor", "saberlength", "saberradius"} {
		if !strings.HasPrefix(k, base) {
			continue
		}
		suffix := strings.TrimPrefix(k, base)
		if suffix == "" {
			return true
		}
		if n, err := strconv.Atoi(suffix); err == nil && n >= 1 && n <= 8 {
			return true
		}
	}
	return false
}

func DefinitionNames(source string) ([]string, error) {
	tokens, err := Lex(source)
	if err != nil {
		return nil, err
	}
	doc := parseAST(tokens)
	var names []string
	for _, node := range doc.Nodes {
		if block, ok := node.(*ASTBlock); ok {
			if block.NameToken != nil {
				names = append(names, block.NameToken.Text)
			}
		}
	}
	return names, nil
}

func ParseSABDefinition(source string, index int) (*SaberData, error) {
	tokens, err := Lex(source)
	if err != nil {
		return nil, err
	}
	doc := parseAST(tokens)

	var block *ASTBlock
	var bIdx int
	currIdx := 0
	for i, node := range doc.Nodes {
		if b, ok := node.(*ASTBlock); ok {
			if currIdx == index {
				block = b
				bIdx = i
				break
			}
			currIdx++
		}
	}
	if block == nil {
		return nil, fmt.Errorf("saber definition at index %d not found", index)
	}

	saber := NewSaberData()
	if block.NameToken != nil {
		saber.Name = block.NameToken.Text
	}

	saber.ctx = &sourceContext{
		doc:        doc,
		blockIndex: bIdx,
	}

	// COM_Parse dispatches every key/value pair in source order.
	// Scalar handlers overwrite earlier duplicates; flag handlers in
	// setSaberField retain the engine's non-zero OR semantics.
	readSaberBlockFields(saber, block)

	return saber, nil
}
