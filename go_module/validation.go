package main

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/Frenzeh/mbii-foundry/parsers"
)

// Per-block byte budgets. The MBCH file overall is capped at 16384
// bytes (R22.0.00); each named block has its own internal budget that
// matters for engine parsing. ClassInfo holds class metadata + most
// per-class fields and tends to be the largest. WeaponInfoN tops out
// at ~4096 — beyond that, animation override blocks risk truncation.
// ForceInfo blocks are smaller and mostly hold sound + icon + name.
// Per-key value text is capped at 2048 to avoid pathological inputs.
const (
	classInfoBudget    = 8192
	weaponInfoBudget   = 4096
	forceInfoBudget    = 2048
	keyValueByteBudget = 2048
)

// Validator checks character data for common errors
type Validator struct{}

func NewValidator() *Validator {
	return &Validator{}
}

func (v *Validator) ValidateCharacter(c *parsers.MBCHCharacter) []string {
	var issues []string

	// 1. Basic Integrity
	if c.Name == "" {
		issues = append(issues, "Name is required")
	}
	if c.MBClass == "" || c.MBClass == "MB_CLASS_NOCLASS" {
		issues = append(issues, "Class must be selected")
	}

	// 2. Force User Checks
	isForceUser := c.MBClass == "MB_CLASS_JEDI" || c.MBClass == "MB_CLASS_SITH"
	if isForceUser {
		if c.ForcePool <= 0 {
			issues = append(issues, "Force Users should have Force Pool > 0")
		}
		if !strings.Contains(c.ForcePowers, "FP_") {
			issues = append(issues, "Force Users usually have Force Powers")
		}
	}

	// 3. Weapon Checks
	if strings.Contains(c.Weapons, "WP_SABER") {
		if c.Saber1 == "" {
			issues = append(issues, "WP_SABER defined but Saber 1 is empty")
		}
		// Check for style
		if c.SaberStyle == "" {
			issues = append(issues, "Saber user has no Saber Style defined")
		}
	}

	if strings.Contains(c.SaberStyle, "SS_DUAL") {
		if c.Saber2 == "" {
			issues = append(issues, "Dual Saber style selected but Saber 2 is empty")
		}
	}

	// 4. Stat Sanity
	if c.MaxHealth > 500 && c.MBClass != "MB_CLASS_SBD" && c.MBClass != "MB_CLASS_DROIDEKA" && c.MBClass != "MB_CLASS_WOOKIE" {
		issues = append(issues, fmt.Sprintf("High HP (%d) for non-tank class", c.MaxHealth))
	}

	return issues
}

// ValidateBlockSizes scans the generated MBCH source for per-block
// budget overruns. Returns one issue per offending block. Source is
// the rendered text from GenerateMBCH (or the live source-panel
// content); we re-extract block bounds rather than rely on the parser
// state because the editor may have unsaved tweaks the parser hasn't
// re-ingested.
func (v *Validator) ValidateBlockSizes(source string) []string {
	var issues []string
	if source == "" {
		return issues
	}
	checkBlock := func(label string, body string, budget int) {
		n := len(body)
		if n > budget {
			issues = append(issues,
				fmt.Sprintf("%s exceeds per-block budget (%d/%d bytes)", label, n, budget))
		} else if n > (budget*9)/10 {
			issues = append(issues,
				fmt.Sprintf("%s near per-block budget (%d/%d bytes)", label, n, budget))
		}
	}

	// ClassInfo — single block.
	if m := regexp.MustCompile(`(?is)ClassInfo\s*\{([^}]+)\}`).FindStringSubmatch(source); len(m) > 1 {
		checkBlock("ClassInfo", m[1], classInfoBudget)
	}
	// WeaponInfoN — multiple, indexed.
	for _, m := range regexp.MustCompile(`(?is)WeaponInfo(\d+)\s*\{([^}]+)\}`).FindAllStringSubmatch(source, -1) {
		checkBlock(fmt.Sprintf("WeaponInfo%s", m[1]), m[2], weaponInfoBudget)
	}
	// ForceInfoN — multiple, indexed.
	for _, m := range regexp.MustCompile(`(?is)ForceInfo(\d+)\s*\{([^}]+)\}`).FindAllStringSubmatch(source, -1) {
		checkBlock(fmt.Sprintf("ForceInfo%s", m[1]), m[2], forceInfoBudget)
	}

	// Per-key value sanity — flag any individual key=value line whose
	// value exceeds the per-key cap. Catches accidental paste-bombs
	// (entire shader block dropped into a single field) before the
	// engine truncates them silently.
	for _, line := range strings.Split(source, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "//") {
			continue
		}
		// Skip block-delimiter lines.
		if trimmed == "{" || trimmed == "}" {
			continue
		}
		// "key value" — split on first whitespace.
		idx := strings.IndexAny(trimmed, " \t")
		if idx <= 0 {
			continue
		}
		key := trimmed[:idx]
		val := strings.TrimSpace(trimmed[idx:])
		if len(val) > keyValueByteBudget {
			issues = append(issues,
				fmt.Sprintf("Field %q value too long (%d/%d bytes)", key, len(val), keyValueByteBudget))
		}
	}
	return issues
}

func (v *Validator) ValidateSaber(s *parsers.SaberData) []string {
	var issues []string
	if s.Name == "" {
		issues = append(issues, "Saber name is required")
	}
	if s.SaberType == "" {
		issues = append(issues, "Saber type must be selected")
	}
	if s.NumBlades < 1 {
		issues = append(issues, "Must have at least 1 blade")
	}
	if len(s.Blades) == 0 {
		issues = append(issues, "No blade configuration found")
	}
	return issues
}
