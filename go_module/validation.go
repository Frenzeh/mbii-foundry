package main

import (
	"fmt"
	"strings"
	
	"github.com/mbii-holocron/fa_creator/parsers"
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
