package main

import (
	"encoding/json"
	"os"
	"regexp"
	"sort"
	"strings"
)

// DiagnosticRoot represents a path to be replaced with a portable label.
type DiagnosticRoot struct {
	Path  string
	Label string
}

var diagnosticJSONSecret = regexp.MustCompile(`(?i)("(?:github[_-]?token|access[_-]?token|token|password|secret|authorization|api[_-]?key)"\s*:\s*)"(?:\\.|[^"\\])*"`)
var diagnosticURLCredential = regexp.MustCompile(`(?i)(https?://)[^/@\s]+@`)
var diagnosticAuthorization = regexp.MustCompile(`(?i)(\bauthorization\s*:\s*(?:bearer|token|basic)\s+)[^\s,;]+`)
var diagnosticNamedSecret = regexp.MustCompile(`(?i)(\b(?:github[_-]?token|access[_-]?token|api[_-]?key|password|authorization|secret)\b\s*=\s*)[^\s,;&]+`)
var diagnosticGitHubToken = regexp.MustCompile(`(?i)(?:ghp|gho|ghu|ghs|ghr|github_pat)_[a-z0-9_]+`)
var diagnosticJWT = regexp.MustCompile(`\beyJ[A-Za-z0-9_-]{8,}\.[A-Za-z0-9_-]{8,}\.[A-Za-z0-9_-]{8,}\b`)

// SanitizeDiagnosticText redacts sensitive data like credentials and replaces
// absolute paths with portable labels (e.g., <UserHomeDir>).
// It accepts a list of known secrets (like an arbitrary token value in memory) to redact.
func SanitizeDiagnosticText(text string, secrets []string, additionalRoots []DiagnosticRoot) string {
	output := text

	// Longest-first prevents a short known secret exposing the suffix of another.
	orderedSecrets := append([]string(nil), secrets...)
	sort.Slice(orderedSecrets, func(i, j int) bool { return len(orderedSecrets[i]) > len(orderedSecrets[j]) })
	for _, secret := range orderedSecrets {
		if secret == "" {
			continue
		}
		encoded, _ := json.Marshal(secret)
		output = strings.ReplaceAll(output, string(encoded[1:len(encoded)-1]), "<REDACTED>")
		output = strings.ReplaceAll(output, secret, "<REDACTED>")
	}
	output = diagnosticJSONSecret.ReplaceAllString(output, `${1}"<REDACTED>"`)
	output = diagnosticURLCredential.ReplaceAllString(output, `${1}<REDACTED>@`)
	output = diagnosticAuthorization.ReplaceAllString(output, `${1}<REDACTED>`)
	output = diagnosticNamedSecret.ReplaceAllString(output, `${1}<REDACTED>`)
	output = diagnosticGitHubToken.ReplaceAllString(output, `<REDACTED_TOKEN>`)
	output = diagnosticJWT.ReplaceAllString(output, `<REDACTED_TOKEN>`)

	var roots []DiagnosticRoot
	roots = append(roots, additionalRoots...)

	// Add standard paths
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		roots = append(roots, DiagnosticRoot{Path: home, Label: "<UserHomeDir>"})
	}
	if cfg, err := os.UserConfigDir(); err == nil && cfg != "" {
		roots = append(roots, DiagnosticRoot{Path: cfg, Label: "<UserConfigDir>"})
	}
	// Add current working directory
	if cwd, err := os.Getwd(); err == nil && cwd != "" {
		roots = append(roots, DiagnosticRoot{Path: cwd, Label: "<Cwd>"})
	}

	// More-specific roots win. Stable sorting also ensures caller-supplied labels
	// win over an equal standard root.
	sort.SliceStable(roots, func(i, j int) bool {
		return len(roots[i].Path) > len(roots[j].Path)
	})

	seenRoots := make(map[string]struct{}, len(roots))
	for _, r := range roots {
		rootPath := strings.TrimRight(r.Path, `/\`)
		if rootPath == "" || (len(rootPath) == 2 && rootPath[1] == ':') || r.Label == "" {
			continue
		}
		if _, seen := seenRoots[rootPath]; seen {
			continue
		}
		seenRoots[rootPath] = struct{}{}
		encodedPath, _ := json.Marshal(rootPath)
		encodedPathFragment := string(encodedPath[1 : len(encodedPath)-1])
		if encodedPathFragment != rootPath {
			encodedLabel, _ := json.Marshal(r.Label)
			output = replaceDiagnosticRoot(
				output,
				encodedPathFragment,
				string(encodedLabel[1:len(encodedLabel)-1]),
			)
		}
		output = replaceDiagnosticRoot(output, rootPath, r.Label)
	}

	return output
}

func replaceDiagnosticRoot(text, root, label string) string {
	if root == "" {
		return text
	}
	var output strings.Builder
	output.Grow(len(text))
	remainder := text
	for {
		index := strings.Index(remainder, root)
		if index < 0 {
			output.WriteString(remainder)
			return output.String()
		}
		beforeOK := index == 0 || diagnosticRootBoundary(remainder[index-1])
		after := index + len(root)
		afterOK := after == len(remainder) || diagnosticRootBoundary(remainder[after])
		if beforeOK && afterOK {
			output.WriteString(remainder[:index])
			output.WriteString(label)
			remainder = remainder[after:]
			continue
		}
		output.WriteString(remainder[:index+len(root)])
		remainder = remainder[after:]
	}
}

func diagnosticRootBoundary(value byte) bool {
	switch value {
	case '/', '\\', '"', '\'', '=', '(', ')', '[', ']', '{', '}', '<', '>', ':', ',', ';', '!', '?',
		' ', '\t', '\r', '\n':
		return true
	default:
		return false
	}
}
