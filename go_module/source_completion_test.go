package main

import (
	"strconv"
	"strings"
	"testing"
)

func TestSourceCompletionUsesAvailableClassesButRecognizesLegacyClasses(t *testing.T) {
	usePublicCatalogs(t)
	hiddenClassIDs = publicHiddenClassIDs()
	language := NewSourceLanguage(nil)
	text := "ClassInfo\n{\n\tname test\n\tMBClass MB_CLASS_\n}\n"
	caret := strings.Index(text, "MB_CLASS_") + len("MB_CLASS_")
	items := language.Complete(text, caret, SourceFormatMBCH)
	if !sourceHasCompletion(items, "MB_CLASS_SOLDIER") {
		t.Fatal("available class was not suggested")
	}
	for _, hidden := range []string{"MB_CLASS_ENFORCER", "MB_CLASS_SPY", "MB_CLASS_ASTROMECH"} {
		if sourceHasCompletion(items, hidden) {
			t.Fatalf("hidden class %s was suggested", hidden)
		}
	}

	authored := "ClassInfo\n{\n\tname test\n\tMBClass MB_CLASS_SPY\n}\n"
	for _, diagnostic := range language.Analyze(authored, SourceFormatMBCH).Diagnostics {
		if strings.Contains(diagnostic.Message, "unknown value") && strings.Contains(diagnostic.Message, "MB_CLASS_SPY") {
			t.Fatalf("known legacy class was incorrectly warned as unknown: %+v", diagnostic)
		}
	}
}

func TestSourceCompletionScopesFieldsAndReplacesOnlyListOrRankFragment(t *testing.T) {
	usePublicCatalogs(t)
	language := NewSourceLanguage(nil)
	text := "ClassInfo\n{\n\tMB\n\tattributes \"MB_ATT_STAMINA,3|MB_ATT_PIS\"\n}\n"
	keyCaret := strings.Index(text, "\tMB\n") + len("\tMB")
	keys := language.Complete(text, keyCaret, SourceFormatMBCH)
	if !sourceHasCompletion(keys, "MBClass") {
		t.Fatalf("ClassInfo key completion missing MBClass: %+v", keys)
	}
	if sourceHasCompletion(keys, "WeaponToReplace") {
		t.Fatal("WeaponInfo field leaked into ClassInfo completion")
	}

	valueCaret := strings.Index(text, "MB_ATT_PIS") + len("MB_ATT_PIS")
	values := language.Complete(text, valueCaret, SourceFormatMBCH)
	pistol, ok := sourceCompletion(values, "MB_ATT_PISTOL")
	if !ok {
		t.Fatalf("attribute completion missing MB_ATT_PISTOL: %+v", values)
	}
	if got := text[pistol.Replace.Start:pistol.Replace.End]; got != "MB_ATT_PIS" {
		t.Fatalf("attribute completion replaces %q, want current pipe fragment", got)
	}
	if pistol.InsertText != "MB_ATT_PISTOL" {
		t.Fatalf("quoted list inserted %q", pistol.InsertText)
	}

	rankText := "ClassInfo\n{\n\tattributes \"MB_ATT_PISTOL,\"\n}\n"
	rankCaret := strings.Index(rankText, `,"`) + 1
	ranks := language.Complete(rankText, rankCaret, SourceFormatMBCH)
	if !sourceHasCompletion(ranks, "2") || !sourceHasCompletion(ranks, "3") {
		t.Fatalf("catalog-backed rank completion missing levels: %+v", ranks)
	}
}

func TestSourceCompletionHandlesQuotedAndUnquotedAssetTokens(t *testing.T) {
	usePublicCatalogs(t)
	application, _ := syntheticSourceAssetApp(t)
	language := NewSourceLanguage(application.sourceAssetSuggestions)
	tests := []struct {
		name   string
		format SourceFormat
		text   string
		prefix string
		label  string
		want   string
	}{
		{
			name: "quoted player shorthand", format: SourceFormatMBCH,
			text: "ClassInfo\n{\n\tmodel \"reb\"\n}\n", prefix: "reb", label: "rebel",
			want: "ClassInfo\n{\n\tmodel \"rebel\"\n}\n",
		},
		{
			name: "unquoted player shorthand", format: SourceFormatMBCH,
			text: "ClassInfo\n{\n\tmodel reb\n}\n", prefix: "reb", label: "rebel",
			want: "ClassInfo\n{\n\tmodel \"rebel\"\n}\n",
		},
		{
			name: "vehicle retains logical path", format: SourceFormatVEH,
			text: "smoke\n{\n\tmodel \"models/vehicles/a\"\n}\n", prefix: "models/vehicles/a",
			label: "models/vehicles/alpha.glm",
			want:  "smoke\n{\n\tmodel \"models/vehicles/alpha.glm\"\n}\n",
		},
		{
			name: "caret after closing quote", format: SourceFormatMBCH,
			text: "WeaponInfo0\n{\n\tIcon \"gfx/hud/dir\"\n}\n", prefix: "gfx/hud/dir\"",
			label: "gfx/hud/direct",
			want:  "WeaponInfo0\n{\n\tIcon \"gfx/hud/direct\"\n}\n",
		},
		{
			name: "caret before opening quote", format: SourceFormatMBCH,
			text: "WeaponInfo0\n{\n\tIcon \"gfx/hud/old\"\n}\n", prefix: "Icon ",
			label: "gfx/hud/direct",
			want:  "WeaponInfo0\n{\n\tIcon \"gfx/hud/direct\"\n}\n",
		},
		{
			name: "empty quoted value", format: SourceFormatMBCH,
			text: "WeaponInfo0\n{\n\tIcon \"\"\n}\n", prefix: "Icon \"",
			label: "gfx/hud/direct",
			want:  "WeaponInfo0\n{\n\tIcon \"gfx/hud/direct\"\n}\n",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			caret := strings.Index(test.text, test.prefix) + len(test.prefix)
			items := language.Complete(test.text, caret, test.format)
			item, ok := sourceCompletion(items, test.label)
			if !ok {
				t.Fatalf("completion missing %q: %+v", test.label, items)
			}
			got := test.text[:item.Replace.Start] + item.InsertText + test.text[item.Replace.End:]
			if got != test.want {
				t.Fatalf("completion changed the wrong source bytes:\ngot:  %q\nwant: %q", got, test.want)
			}
		})
	}
	trailing := "WeaponInfo0\n{\n\tIcon \"gfx/hud/dir\" \n}\n"
	caret := strings.Index(trailing, "\" \n") + 2
	if items := language.Complete(trailing, caret, SourceFormatMBCH); len(items) != 0 {
		t.Fatalf("completion must not append a second value after whitespace: %+v", items)
	}
}

func TestSourceCompletionAppliesAttributeSemanticsToExplicitCustomSkills(t *testing.T) {
	usePublicCatalogs(t)
	language := NewSourceLanguage(nil)
	for index := 0; index <= 14; index++ {
		field := "c_att_skill_" + strconv.Itoa(index)
		text := "ClassInfo\n{\n\t" + field + " MB_ATT_PIS\n}\n"
		caret := strings.Index(text, "MB_ATT_PIS") + len("MB_ATT_PIS")
		items := language.Complete(text, caret, SourceFormatMBCH)
		if !sourceHasCompletion(items, "MB_ATT_PISTOL") {
			t.Fatalf("%s missing attribute completion: %+v", field, items)
		}
		if sourceHasCompletion(items, "2") {
			t.Fatalf("%s was confused with a ranked attribute field: %+v", field, items)
		}
	}
}

func TestSourceCompletionCoversOtherSupportedScopes(t *testing.T) {
	usePublicCatalogs(t)
	language := NewSourceLanguage(nil)
	tests := []struct {
		format SourceFormat
		text   string
		prefix string
		want   string
	}{
		{SourceFormatSAB, "my_saber\n{\n\tsaberSt\n}\n", "saberSt", "saberStyle"},
		{SourceFormatVEH, "my_vehicle\n{\n\ttype VH_\n}\n", "VH_", "VH_SPEEDER"},
		{SourceFormatSiege, "Teams { team1 Rebels }\nRebels\n{\n\tObjective1\n\t{\n\t\tmap\n\t}\n}\n", "map", "mapicon"},
		{SourceFormatMBTC, "Classes\n{\n\tclass1 MB_CLASS_\n}\n", "MB_CLASS_", "MB_CLASS_SOLDIER"},
	}
	for _, test := range tests {
		caret := strings.LastIndex(test.text, test.prefix) + len(test.prefix)
		if items := language.Complete(test.text, caret, test.format); !sourceHasCompletion(items, test.want) {
			t.Errorf("%s completion missing %q: %+v", test.format, test.want, items)
		}
	}
}

func sourceHasCompletion(items []SourceCompletion, label string) bool {
	_, ok := sourceCompletion(items, label)
	return ok
}

func sourceCompletion(items []SourceCompletion, label string) (SourceCompletion, bool) {
	for _, item := range items {
		if item.Label == label {
			return item, true
		}
	}
	return SourceCompletion{}, false
}
