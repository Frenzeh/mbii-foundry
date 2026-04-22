package parsers

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

type VehicleData struct {
	Name        string
	Type        string // e.g. VH_SPEEDER
	Model       string
	Skin        string
	SpeedMax    float64
	TurboSpeed  float64
	Accel       float64
	Decel       float64
	StrafePerc  float64
	BankingSpeed float64
	RollLimit   float64
	PitchLimit  float64
	Braking     float64
	MouseYaw    float64
	MousePitch  float64
	CustomGravity float64
	
	Armor       int
	Shields     int
	
	Weapons     string // Comma/pipe separated
	
	ExtraFields map[string]string
}

func NewVehicleData() *VehicleData {
	return &VehicleData{
		Type: "VH_SPEEDER",
		ExtraFields: make(map[string]string),
	}
}

// ParseVEH parses the content of a VEH file
func ParseVEH(content string) (*VehicleData, error) {
	veh := NewVehicleData()
	
	lines := []string{}
	for _, line := range strings.Split(content, "\n") {
		idx := strings.Index(line, "//")
		if idx >= 0 { line = line[:idx] }
		lines = append(lines, line)
	}
	cleanContent := strings.Join(lines, "\n")

	// Extract Main Block: Name { ... }
	// Using simple regex for outer block is usually fine if file is Name { ... }
	// But robust brace counting is safer. 
	// For now, assuming standard VEH format Name { ... }
	re := regexp.MustCompile(`(?is)(\w+)\s*{\s*([^}]+)\s*}`)
	match := re.FindStringSubmatch(cleanContent)
	
	if len(match) > 2 {
		veh.Name = match[1]
		parseVehicleBlock(match[2], veh)
	} else {
		return nil, fmt.Errorf("no valid vehicle block found")
	}

	return veh, nil
}

func parseVehicleBlock(block string, veh *VehicleData) {
	for _, line := range strings.Split(block, "\n") {
		line = strings.TrimSpace(line)
		if line == "" { continue }

		parts := strings.Fields(line)
		if len(parts) < 2 { continue }
		
		key := parts[0]
		idx := strings.Index(line, key)
		valuePart := strings.TrimSpace(line[idx+len(key):])
		
		value := valuePart
		if strings.HasPrefix(value, "\"") && strings.HasSuffix(value, "\"") {
			if len(value) >= 2 {
				value = value[1 : len(value)-1]
			}
		}
		
		switch strings.ToLower(key) {
		case "name": veh.Name = value
		case "type": veh.Type = value
		case "model": veh.Model = value
		case "skin": veh.Skin = value
		case "speed": veh.SpeedMax, _ = strconv.ParseFloat(value, 64)
		case "turbospeed": veh.TurboSpeed, _ = strconv.ParseFloat(value, 64)
		case "accel": veh.Accel, _ = strconv.ParseFloat(value, 64)
		case "decel": veh.Decel, _ = strconv.ParseFloat(value, 64)
		case "strafeperc": veh.StrafePerc, _ = strconv.ParseFloat(value, 64)
		case "bankingspeed": veh.BankingSpeed, _ = strconv.ParseFloat(value, 64)
		case "rolllimit": veh.RollLimit, _ = strconv.ParseFloat(value, 64)
		case "pitchlimit": veh.PitchLimit, _ = strconv.ParseFloat(value, 64)
		case "braking": veh.Braking, _ = strconv.ParseFloat(value, 64)
		case "mouseyaw": veh.MouseYaw, _ = strconv.ParseFloat(value, 64)
		case "mousepitch": veh.MousePitch, _ = strconv.ParseFloat(value, 64)
		case "customgravity": veh.CustomGravity, _ = strconv.ParseFloat(value, 64)
		case "armor": veh.Armor, _ = strconv.Atoi(value)
		case "shields": veh.Shields, _ = strconv.Atoi(value)
		case "weapons": veh.Weapons = value
		default: veh.ExtraFields[key] = value
		}
	}
}

func GenerateVEH(veh *VehicleData) (string, error) {
	var sb strings.Builder
	fmt.Fprintf(&sb, "%s\n{\n", veh.Name)
	fmt.Fprintf(&sb, "\tname\t\t%s\n", veh.Name)
	fmt.Fprintf(&sb, "\ttype\t\t%s\n", veh.Type)
	if veh.Model != "" { fmt.Fprintf(&sb, "\tmodel\t\t%s\n", veh.Model) }
	if veh.Skin != "" { fmt.Fprintf(&sb, "\tskin\t\t%s\n", veh.Skin) }
	
	if veh.SpeedMax != 0 { fmt.Fprintf(&sb, "\tspeed\t\t%.1f\n", veh.SpeedMax) }
	if veh.TurboSpeed != 0 { fmt.Fprintf(&sb, "\tturboSpeed\t%.1f\n", veh.TurboSpeed) }
	if veh.Accel != 0 { fmt.Fprintf(&sb, "\taccel\t\t%.1f\n", veh.Accel) }
	if veh.Decel != 0 { fmt.Fprintf(&sb, "\tdecel\t\t%.1f\n", veh.Decel) }
	if veh.StrafePerc != 0 { fmt.Fprintf(&sb, "\tstrafePerc\t%.1f\n", veh.StrafePerc) }
	if veh.BankingSpeed != 0 { fmt.Fprintf(&sb, "\tbankingSpeed\t%.1f\n", veh.BankingSpeed) }
	if veh.RollLimit != 0 { fmt.Fprintf(&sb, "\trollLimit\t%.1f\n", veh.RollLimit) }
	if veh.PitchLimit != 0 { fmt.Fprintf(&sb, "\tpitchLimit\t%.1f\n", veh.PitchLimit) }
	if veh.Braking != 0 { fmt.Fprintf(&sb, "\tbraking\t\t%.1f\n", veh.Braking) }
	if veh.MouseYaw != 0 { fmt.Fprintf(&sb, "\tmouseYaw\t%.1f\n", veh.MouseYaw) }
	if veh.MousePitch != 0 { fmt.Fprintf(&sb, "\tmousePitch\t%.1f\n", veh.MousePitch) }
	if veh.CustomGravity != 0 { fmt.Fprintf(&sb, "\tcustomGravity\t%.1f\n", veh.CustomGravity) }
	
	if veh.Armor != 0 { fmt.Fprintf(&sb, "\tarmor\t\t%d\n", veh.Armor) }
	if veh.Shields != 0 { fmt.Fprintf(&sb, "\tshields\t\t%d\n", veh.Shields) }
	if veh.Weapons != "" { fmt.Fprintf(&sb, "\tweapons\t\t%s\n", veh.Weapons) }
	
	for k, v := range veh.ExtraFields {
		fmt.Fprintf(&sb, "\t%s\t\t%s\n", k, v)
	}
	fmt.Fprintln(&sb, "}")
	return sb.String(), nil
}