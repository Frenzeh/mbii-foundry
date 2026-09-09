package parsers

import (
	"reflect"
	"strings"
	"testing"
)

func TestSABEngineDuplicateSemanticsAndTransitions(t *testing.T) {
	input := `// keep header
saber_a {
	noWallMarks 2 // any non-zero occurrence sets the engine flag
	NOWALLMARKS 0
	g2MarksShader "gfx/old"
	// keep between duplicates
	G2MARKSSHADER "gfx/effective"
	readyAnim BOTH_READY_OLD
	readyAnim BOTH_READY_EFFECTIVE
	unknownField "keep me"
}
`

	saber, err := ParseSAB(input)
	if err != nil {
		t.Fatal(err)
	}
	if !saber.NoWallMarks {
		t.Fatal("non-zero boolean occurrence followed by zero parsed false; engine OR semantics require true")
	}
	if saber.G2MarksShader != "gfx/effective" || saber.ReadyAnim != "BOTH_READY_EFFECTIVE" {
		t.Fatalf("scalar duplicates did not use the engine's last value: shader=%q ready=%q", saber.G2MarksShader, saber.ReadyAnim)
	}
	noOp, err := GenerateSAB(saber)
	if err != nil {
		t.Fatal(err)
	}
	if noOp != input {
		t.Fatalf("no-op save changed bytes:\n--- want ---\n%s\n--- got ---\n%s", input, noOp)
	}

	saber.NoWallMarks = false
	saber.G2MarksShader = "gfx/edited"
	saber.ReadyAnim = "BOTH_READY_EDITED"
	output, err := GenerateSAB(saber)
	if err != nil {
		t.Fatal(err)
	}
	lower := strings.ToLower(output)
	if strings.Contains(lower, "nowallmarks") {
		t.Fatalf("clearing a true flag left a duplicate that can revive it:\n%s", output)
	}
	if !strings.Contains(output, `g2MarksShader "gfx/old"`) || !strings.Contains(output, `G2MARKSSHADER "gfx/edited"`) {
		t.Fatalf("shader edit did not preserve the shadowed line and change only the effective line:\n%s", output)
	}
	if !strings.Contains(output, "readyAnim BOTH_READY_OLD") || !strings.Contains(output, "readyAnim BOTH_READY_EDITED") {
		t.Fatalf("animation edit did not change only the effective line:\n%s", output)
	}
	if !strings.Contains(output, "// keep between duplicates") || !strings.Contains(output, `unknownField "keep me"`) {
		t.Fatalf("modeled edits lost comments or unknown fields:\n%s", output)
	}

	roundTrip, err := ParseSAB(output)
	if err != nil {
		t.Fatal(err)
	}
	if roundTrip.NoWallMarks || roundTrip.G2MarksShader != "gfx/edited" || roundTrip.ReadyAnim != "BOTH_READY_EDITED" {
		t.Fatalf("edited output did not round-trip: flag=%v shader=%q ready=%q", roundTrip.NoWallMarks, roundTrip.G2MarksShader, roundTrip.ReadyAnim)
	}

	saber.G2MarksShader = ""
	cleared, err := GenerateSAB(saber)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(strings.ToLower(cleared), "g2marksshader") {
		t.Fatalf("clearing a scalar left a duplicate that can revive it:\n%s", cleared)
	}
}

func completeSaberData() *SaberData {
	return &SaberData{
		Name:               "complete_saber",
		FullName:           "Complete Saber",
		SaberType:          "SABER_STAFF",
		SaberModel:         "models/weapons2/saber/model.glm",
		CustomSkin:         "models/weapons2/saber/model_red.skin",
		NumBlades:          3,
		Blades:             []BladeInfo{{Color: "orange", Length: 31.125, Radius: 2.75}, {Color: "blue", Length: 24.5, Radius: 1.25}, {Color: "green", Length: 40.75, Radius: 4.125}},
		SoundOn:            "sound/on.wav",
		SoundOff:           "sound/off.wav",
		SoundLoop:          "sound/loop.wav",
		SpinSound:          "sound/spin.wav",
		SwingSound1:        "sound/swing1.wav",
		SwingSound2:        "sound/swing2.wav",
		SwingSound3:        "sound/swing3.wav",
		FallSound1:         "sound/fall1.wav",
		FallSound2:         "sound/fall2.wav",
		FallSound3:         "sound/fall3.wav",
		HitSound1:          "sound/hit1.wav",
		HitSound2:          "sound/hit2.wav",
		HitSound3:          "sound/hit3.wav",
		BlockSound1:        "sound/block1.wav",
		BlockSound2:        "sound/block2.wav",
		BlockSound3:        "sound/block3.wav",
		BounceSound1:       "sound/bounce1.wav",
		BounceSound2:       "sound/bounce2.wav",
		BounceSound3:       "sound/bounce3.wav",
		SaberStyle:         "SS_STRONG",
		SingleBladeStyle:   "SS_FAST",
		MaxChain:           7,
		LockBonus:          2,
		ParryBonus:         3,
		BreakParryBonus:    4,
		DisarmBonus:        5,
		MoveSpeedScale:     1.125,
		AnimSpeedScale:     0.875,
		DamageScale:        1.375,
		KnockbackScale:     0.625,
		TrailStyle:         2,
		BlockEffect:        "effects/block",
		HitPersonEffect:    "effects/person",
		BladeEffect:        "effects/blade",
		HitOtherEffect:     "effects/other",
		NoWallMarks:        true,
		NoDlight:           true,
		NoBlade:            true,
		NoClashFlare:       true,
		NoDismemberment:    true,
		NoIdleEffect:       true,
		AlwaysBlock:        true,
		NoManualDeactivate: true,
		TransitionDamage:   true,
		NotInOpen:          true,
		NotInMP:            true,
		NoCartwheels:       true,
		Throwable:          true,
		Disarmable:         true,
		BlasterBlocking:    true,
		OnInWater:          true,
		BounceOnWalls:      true,
		TwoHanded:          true,
		UseGoreConfig:      true,
		UseGoreConfig2:     true,
		NoDismemberment2:   true,
		NoBladeEffects:     true,
		NoBladeEffects2:    true,
		G2MarksShader:      "gfx/damage/mark",
		G2WeaponMarkShader: "gfx/damage/weaponmark",
		SlapAnim:           "BOTH_SLAP",
		ReadyAnim:          "BOTH_READY",
		JumpAtkUpMove:      "LS_JUMP_UP",
		JumpAtkFwdMove:     "LS_JUMP_FWD",
		LungeAtkMove:       "LS_LUNGE",
		ExtraFields:        map[string]string{"customUnknown": "preserved value"},
		SaberFlagMap:       map[string]bool{},
	}
}

func TestSABFreshGenerationRoundTripsEveryModeledField(t *testing.T) {
	want := completeSaberData()

	generated, err := GenerateSAB(want)
	if err != nil {
		t.Fatal(err)
	}
	got, err := ParseSAB(generated)
	if err != nil {
		t.Fatal(err)
	}
	got.ctx = nil
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("fresh generation lost modeled data:\nwant: %#v\n got: %#v\nsource:\n%s", want, got, generated)
	}
}

func TestSABASTSyncRoundTripsEveryModeledScalar(t *testing.T) {
	source, err := GenerateSAB(completeSaberData())
	if err != nil {
		t.Fatal(err)
	}
	want, err := ParseSAB(source)
	if err != nil {
		t.Fatal(err)
	}

	value := reflect.ValueOf(want).Elem()
	valueType := value.Type()
	for i := range value.NumField() {
		field := value.Field(i)
		name := valueType.Field(i).Name
		if !field.CanSet() || name == "NumBlades" || name == "Blades" || name == "ExtraFields" || name == "SaberFlagMap" {
			continue
		}
		switch field.Kind() {
		case reflect.String:
			field.SetString(field.String() + "_EDITED")
		case reflect.Int:
			field.SetInt(field.Int() + 1)
		case reflect.Float64:
			field.SetFloat(field.Float() + 0.125)
		case reflect.Bool:
			field.SetBool(!field.Bool())
		default:
			t.Fatalf("modeled field %s has an unhandled transition kind %s", name, field.Kind())
		}
	}
	for i := range want.Blades {
		want.Blades[i].Color += "_edited"
		want.Blades[i].Length += float64(i+1) * 0.125
		want.Blades[i].Radius += float64(i+1) * 0.25
	}
	want.ExtraFields["customUnknown"] = "edited value"

	output, err := GenerateSAB(want)
	if err != nil {
		t.Fatal(err)
	}
	got, err := ParseSAB(output)
	if err != nil {
		t.Fatal(err)
	}
	want.ctx = nil
	got.ctx = nil
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("AST sync lost a modeled transition:\nwant: %#v\n got: %#v\nsource:\n%s", want, got, output)
	}
}

func TestSABSecondaryFlagsAndEditorFieldsGenerateFresh(t *testing.T) {
	saber := NewSaberData()
	saber.Name = "fresh"
	saber.UseGoreConfig2 = true
	saber.NoDismemberment2 = true
	saber.NoBladeEffects = true
	saber.NoBladeEffects2 = true
	saber.G2MarksShader = "gfx/marks"
	saber.G2WeaponMarkShader = "gfx/weaponmarks"
	saber.SlapAnim = "BOTH_SLAP"
	saber.ReadyAnim = "BOTH_READY"
	saber.JumpAtkUpMove = "LS_UP"
	saber.JumpAtkFwdMove = "LS_FWD"
	saber.LungeAtkMove = "LS_LUNGE"

	generated, err := GenerateSAB(saber)
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{
		"useGoreConfig2", "noDismemberment2", "noBladeEffects", "noBladeEffects2",
		"g2MarksShader", "g2WeaponMarkShader", "slapAnim", "readyAnim",
		"jumpAtkUpMove", "jumpAtkFwdMove", "lungeAtkMove",
	} {
		if !strings.Contains(generated, key) {
			t.Errorf("fresh generator omitted %s:\n%s", key, generated)
		}
	}
}
