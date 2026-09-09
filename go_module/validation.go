package main

import (
	"fmt"

	"github.com/Frenzeh/mbii-foundry/parsers"
)

// Validator checks character data for common errors
type Validator struct{}

func NewValidator() *Validator {
	return &Validator{}
}

func (v *Validator) ValidateCharacter(c *parsers.MBCHCharacter) []string {
	if c == nil {
		return []string{"Character is required"}
	}

	var issues []string
	if c.Name == "" {
		issues = append(issues, "Name is required")
	}
	if c.MBClass == "" || c.MBClass == "MB_CLASS_NOCLASS" {
		issues = append(issues, "Class must be selected")
	}

	if c.HasCustomSpec < 0 || c.HasCustomSpec > parsers.PointbuyMaxArchetypes {
		issues = append(issues, fmt.Sprintf(
			"hasCustomSpec exceeds engine range (got %d; maximum %d archetypes)",
			c.HasCustomSpec, parsers.PointbuyMaxArchetypes,
		))
	}
	activeArchetypes := c.HasCustomSpec
	if activeArchetypes < 2 {
		activeArchetypes = 1
	}
	if activeArchetypes > parsers.PointbuyMaxArchetypes {
		activeArchetypes = parsers.PointbuyMaxArchetypes
	}
	activeSlots := activeArchetypes * parsers.PointbuySlotsPerArchetype
	for i := activeSlots; i < parsers.PointbuyMaxTotalSlots; i++ {
		if c.CustomSkills[i] != "" || c.CustomNames[i] != "" ||
			c.CustomRanks[i] != "" || c.CustomDescs[i] != "" {
			issues = append(issues, fmt.Sprintf(
				"custom point-buy slot %d is outside the active engine range (maximum %d slots per archetype, %d total)",
				i, parsers.PointbuySlotsPerArchetype, parsers.PointbuyMaxTotalSlots,
			))
		}
	}

	source, err := parsers.GenerateMBCH(c)
	if err != nil {
		return append(issues, fmt.Sprintf("GenerateMBCH failed: %v", err))
	}
	return append(issues, v.ValidateBlockSizes(source)...)
}

func (v *Validator) ValidateBlockSizes(source string) []string {
	assessment, err := parsers.AssessMBCHSourceBuffers(source)
	if err != nil {
		return []string{fmt.Sprintf("AssessMBCHSourceBuffers failed: %v", err)}
	}

	issues := make([]string, 0, len(assessment.Diagnostics)+5)
	for _, diagnostic := range assessment.Diagnostics {
		issues = append(issues, fmt.Sprintf("Parser diagnostic: %s", diagnostic))
	}
	if assessment.TotalFileBytes >= parsers.MBCHMaxFileBytes {
		issues = append(issues, fmt.Sprintf(
			"File exceeds absolute engine capacity (%d/%d payload bytes; capacity %d includes the terminator)",
			assessment.TotalFileBytes, parsers.MBCHMaxFileBytes-1, parsers.MBCHMaxFileBytes,
		))
	}
	if assessment.ClassInfoBytes > parsers.ClassInfoMaxPayload {
		issues = append(issues, fmt.Sprintf(
			"ClassInfo exceeds engine payload limit (%d/%d bytes; buffer %d)",
			assessment.ClassInfoBytes, parsers.ClassInfoMaxPayload, parsers.ClassInfoMaxBytes,
		))
	}
	for i, size := range assessment.WeaponInfoBytes {
		if size > parsers.WeaponInfoMaxPayload {
			issues = append(issues, fmt.Sprintf(
				"WeaponInfo[%d] exceeds engine payload limit (%d/%d bytes; buffer %d)",
				i, size, parsers.WeaponInfoMaxPayload, parsers.WeaponInfoMaxBytes,
			))
		}
	}
	for i, size := range assessment.ForceInfoBytes {
		if size > parsers.ForceInfoMaxPayload {
			issues = append(issues, fmt.Sprintf(
				"ForceInfo[%d] exceeds engine payload limit (%d/%d bytes; buffer %d)",
				i, size, parsers.ForceInfoMaxPayload, parsers.ForceInfoMaxBytes,
			))
		}
	}
	if assessment.MaxPairedValueBytes > parsers.PairedValueMaxPayload {
		issues = append(issues, fmt.Sprintf(
			"Paired value exceeds engine payload limit (%d/%d bytes; buffer %d)",
			assessment.MaxPairedValueBytes, parsers.PairedValueMaxPayload, parsers.PairedValueMaxBytes,
		))
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

func (v *Validator) ValidateVehicle(veh *parsers.VehicleData) []string {
	var issues []string
	if veh.Name == "" {
		issues = append(issues, "Vehicle name is required")
	}
	if veh.Type == "" {
		issues = append(issues, "Vehicle type must be selected")
	}
	return issues
}

func (v *Validator) ValidateSiege(s *parsers.SiegeData) []string {
	var issues []string
	if s.Team1 == nil && s.Team2 == nil {
		issues = append(issues, "Siege requires at least one team")
	}

	checkTeam := func(team *parsers.SiegeTeam) {
		if team == nil {
			return
		}
		if team.UseTeam == "" {
			issues = append(issues, fmt.Sprintf("Team '%s' is missing 'UseTeam' (faction)", team.Name))
		}
		hasFinal := false
		for _, obj := range team.Objectives {
			if obj.Final != 0 {
				hasFinal = true
			}
		}
		if team.Attackers != 0 && !hasFinal && len(team.Objectives) > 0 {
			issues = append(issues, fmt.Sprintf("Attacking team '%s' has objectives but no 'final 1' objective", team.Name))
		}
	}
	checkTeam(s.Team1)
	checkTeam(s.Team2)

	return issues
}
