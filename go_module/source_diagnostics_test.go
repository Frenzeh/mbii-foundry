package main

import (
	"strings"
	"testing"

	"github.com/Frenzeh/mbii-foundry/parsers"
)

func TestSourceDiagnosticsSeparateStructuralErrorsFromPreservedWarnings(t *testing.T) {
	usePublicCatalogs(t)
	text := "ClassInfo\n{\n\tname test\n\tMBClass MB_CLASS_CUSTOM\n\tfutureField opaque\n}\n"
	diagnostics := NewSourceLanguage(nil).Analyze(text, SourceFormatMBCH).Diagnostics
	if !sourceHasDiagnostic(diagnostics, SourceWarning, "unknown value preserved for MBClass") {
		t.Fatalf("custom class warning missing: %+v", diagnostics)
	}
	if !sourceHasDiagnostic(diagnostics, SourceWarning, "unknown field preserved: futureField") {
		t.Fatalf("custom field warning missing: %+v", diagnostics)
	}
	for _, diagnostic := range diagnostics {
		if diagnostic.Severity == SourceError && strings.Contains(diagnostic.Message, "unknown") {
			t.Fatalf("unknown extension was made fatal: %+v", diagnostic)
		}
	}

	broken := "ClassInfo\n{\n\tname test\n\tMBClass MB_CLASS_SOLDIER\n"
	diagnostics = NewSourceLanguage(nil).Analyze(broken, SourceFormatMBCH).Diagnostics
	if !sourceHasDiagnostic(diagnostics, SourceError, "unclosed opening brace") {
		t.Fatalf("structural error missing: %+v", diagnostics)
	}
}

func TestSourceDiagnosticsEnforceEstablishedCapacityOnlyOnApplicableFormats(t *testing.T) {
	usePublicCatalogs(t)
	largeValue := strings.Repeat("x", parsers.PairedValueMaxPayload+1)
	mbch := "ClassInfo\n{\n\tname test\n\tMBClass MB_CLASS_SOLDIER\n\tfuture \"" + largeValue + "\"\n}\n"
	diagnostics := NewSourceLanguage(nil).Analyze(mbch, SourceFormatMBCH).Diagnostics
	if !sourceHasDiagnostic(diagnostics, SourceError, "paired value exceeds engine payload limit") {
		t.Fatalf("MBCH paired-value capacity error missing: %+v", diagnostics)
	}

	veh := "vehicle\n{\n\tname vehicle\n\ttype VH_SPEEDER\n\tnote \"" + largeValue + "\"\n}\n"
	diagnostics = NewSourceLanguage(nil).Analyze(veh, SourceFormatVEH).Diagnostics
	if sourceHasDiagnostic(diagnostics, SourceError, "paired value exceeds engine payload limit") {
		t.Fatalf("MBCH-only capacity was applied to VEH: %+v", diagnostics)
	}
}

func TestSourceDiagnosticsUseFormatParsersForRequiredStructure(t *testing.T) {
	tests := []struct {
		format SourceFormat
		text   string
		want   string
	}{
		{SourceFormatMBCH, "WeaponInfo0 { WeaponToReplace WP_BLASTER }", "missing ClassInfo"},
		{SourceFormatSiege, "Teams { team1 Rebels }\nRebels { attackers 1 }", "missing useTeam"},
		{SourceFormatMBTC, "Classes { class1 soldier }", "engine errors out"},
		{SourceFormat(".future"), "Thing { value 1 }", "unsupported source format"},
	}
	for _, test := range tests {
		diagnostics := NewSourceLanguage(nil).Analyze(test.text, test.format).Diagnostics
		if !sourceHasDiagnostic(diagnostics, SourceError, test.want) {
			t.Errorf("%s diagnostics missing %q: %+v", test.format, test.want, diagnostics)
		}
	}
}

func TestSourceDiagnosticsPreserveOriginalOffsetsForInsensitiveLookup(t *testing.T) {
	text := "İ prefix Target"
	got := sourceFindInsensitiveRange(text, "target")
	want := strings.Index(text, "Target")
	if got != (SourceRange{Start: want, End: want + len("Target")}) {
		t.Fatalf("case-insensitive range = %+v, want original byte range [%d,%d)", got, want, want+len("Target"))
	}
}

func TestSourceDiagnosticsReuseSABSemanticsForEachDefinition(t *testing.T) {
	text := "valid\n{\n\tnumBlades 1\n}\ninvalid\n{\n\tnumBlades 0\n}\n"
	diagnostics := NewSourceLanguage(nil).Analyze(text, SourceFormatSAB).Diagnostics
	wantStart := strings.LastIndex(text, "numBlades")
	for _, diagnostic := range diagnostics {
		if diagnostic.Severity == SourceError && strings.Contains(diagnostic.Message, "numBlades 0") {
			if diagnostic.Range != (SourceRange{Start: wantStart, End: wantStart + len("numBlades")}) {
				t.Fatalf("SAB semantic range = %+v, want [%d,%d)", diagnostic.Range, wantStart, wantStart+len("numBlades"))
			}
			return
		}
	}
	t.Fatalf("SAB semantic error missing: %+v", diagnostics)
}

func sourceHasDiagnostic(diagnostics []SourceDiagnostic, severity SourceSeverity, fragment string) bool {
	for _, diagnostic := range diagnostics {
		if diagnostic.Severity == severity && strings.Contains(diagnostic.Message, fragment) {
			return true
		}
	}
	return false
}
