package main

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

func TestDiagnosticJSONRedactsEscapedCredentials(t *testing.T) {
	input := `{"github_token":"fixture\"secret-tail","message":"decode failed","password":"second-fixture"}`
	got := SanitizeDiagnosticText(input, nil, nil)
	var fields map[string]string
	if err := json.Unmarshal([]byte(got), &fields); err != nil {
		t.Fatal("sanitized diagnostic JSON is invalid")
	}
	if strings.Contains(got, "secret-tail") || strings.Contains(got, "second-fixture") {
		t.Fatal("escaped credential material leaked")
	}
	if fields["message"] != "decode failed" {
		t.Fatal("useful nonsensitive diagnostic was lost")
	}
}

func TestDiagnosticKnownSecretRedactsJSONEscapedAndOverlappingForms(t *testing.T) {
	secret := "fixture-secret-long\"with\\escapes"
	input, err := json.Marshal(map[string]string{
		"detail": secret,
		"status": "decode failed",
	})
	if err != nil {
		t.Fatal(err)
	}
	got := SanitizeDiagnosticText(string(input), []string{"fixture-secret", secret}, nil)
	if strings.Contains(got, "fixture-secret") || strings.Contains(got, "with") || strings.Contains(got, "escapes") {
		t.Fatal("JSON-escaped known secret leaked")
	}
	var decoded map[string]string
	if err := json.Unmarshal([]byte(got), &decoded); err != nil {
		t.Fatal("known-secret redaction corrupted JSON")
	}
	if decoded["detail"] != "<REDACTED>" || decoded["status"] != "decode failed" {
		t.Fatal("known-secret redaction lost useful diagnostic context")
	}
}

func TestDiagnosticSecretsAndRootPrecedence(t *testing.T) {
	input := "auth=fixture-secret-long and fixture-secret; https://fixture-user:fixture-pass@example.invalid/release; path=/fixture/root/assets/models/mira/icon.png"
	got := SanitizeDiagnosticText(input, []string{"fixture-secret", "fixture-secret-long"}, []DiagnosticRoot{
		{Path: "/fixture/root", Label: "<root>"},
		{Path: "/fixture/root/assets", Label: "<assets>"},
	})
	for _, private := range []string{"fixture-secret", "-long", "fixture-user", "fixture-pass", "/fixture/root"} {
		if strings.Contains(got, private) {
			t.Fatal("private diagnostic content leaked")
		}
	}
	if !strings.Contains(got, "example.invalid/release") || !strings.Contains(got, "<assets>/models/mira/icon.png") {
		t.Fatal("sanitization lost the useful release endpoint or source-relative asset identity")
	}
}

func TestDiagnosticExportRedactsJSONEncodedRoots(t *testing.T) {
	windowsRoot := `C:\Users\fixture-private\Assets`
	macRoot := `/Users/fixture-private/Assets "custom"`
	encoded, err := json.Marshal(map[string]string{
		"windows": windowsRoot + `\models\icon.png`,
		"mac":     macRoot + "/models/icon.png",
	})
	if err != nil {
		t.Fatal(err)
	}
	got := SanitizeDiagnosticText(string(encoded), nil, []DiagnosticRoot{
		{Path: windowsRoot, Label: "<GameData>"},
		{Path: macRoot, Label: "<TextAssets>"},
	})
	if strings.Contains(got, "fixture-private") {
		t.Fatal("JSON-encoded root identity leaked")
	}
	var decoded map[string]string
	if err := json.Unmarshal([]byte(got), &decoded); err != nil {
		t.Fatal("encoded root redaction corrupted exported JSON")
	}
	if decoded["windows"] != `<GameData>\models\icon.png` || decoded["mac"] != "<TextAssets>/models/icon.png" {
		t.Fatal("source-relative asset identities were not preserved")
	}
}

func TestDiagnosticTokenFormatsAreRedactedWithoutLosingContext(t *testing.T) {
	githubToken := "ghp_fixtureTOKEN123"
	jwt := "eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiJmaXh0dXJlIn0.signature12345"
	input := "request failed: Authorization: Bearer bearer-fixture; access_token=query-fixture&state=kept; github=" +
		githubToken + "; session=" + jwt
	got := SanitizeDiagnosticText(input, nil, nil)
	for _, private := range []string{"bearer-fixture", "query-fixture", githubToken, jwt} {
		if strings.Contains(got, private) {
			t.Fatalf("token material leaked: %q", private)
		}
	}
	for _, useful := range []string{"request failed", "Authorization: Bearer", "state=kept", "github=", "session="} {
		if !strings.Contains(got, useful) {
			t.Fatalf("useful diagnostic context was removed: %q", useful)
		}
	}
}

func TestDiagnosticRootReplacementHonorsPathBoundaries(t *testing.T) {
	input := "private=/fixture/root/assets/model.glm public=/fixture/rooted/readme.txt words=token budget"
	got := SanitizeDiagnosticText(input, nil, []DiagnosticRoot{{Path: "/fixture/root/", Label: "<Project>"}})
	if strings.Contains(got, "private=/fixture/root/") {
		t.Fatal("private operator root was not redacted")
	}
	if !strings.Contains(got, "private=<Project>/assets/model.glm") {
		t.Fatal("root redaction lost the useful source-relative path")
	}
	if !strings.Contains(got, "public=/fixture/rooted/readme.txt") || !strings.Contains(got, "words=token budget") {
		t.Fatal("sanitizer over-redacted a root prefix or ordinary token wording")
	}
}

func TestDiagnosticCallerRootLabelWinsForEqualStandardRoot(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	// UserHomeDir reads HOME on Unix; provide the actual returned value so this
	// remains portable if the platform uses a different source.
	if current, err := os.UserHomeDir(); err == nil {
		home = current
	}
	got := SanitizeDiagnosticText(home+"/project/file.mbch", nil, []DiagnosticRoot{{Path: home, Label: "<ProjectRoot>"}})
	if got != "<ProjectRoot>/project/file.mbch" {
		t.Fatalf("caller label did not take precedence: %q", got)
	}
}
