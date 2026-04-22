package parsers

import (
	"fmt"
	"strconv"
	"strings"
)

type SiegeObjective struct {
	Name          string // e.g. Objective1
	GoalName      string
	Final         int
	ObjDesc       string
	ObjGfx        string
	MapIcon       string
	LitMapIcon    string
	DoneMapIcon   string
	MapPos        string // "x y w h"? or "x y"
	MessageTeam1  string
	MessageTeam2  string
	SoundTeam1    string
	SoundTeam2    string
	Target        string
	ExtraFields   map[string]string
}

type SiegeTeam struct {
	Name               string // The key used in the file (e.g. "Imperials")
	TeamName           string // team1 or team2 (mapped from Teams block)
	UseTeam            string
	TeamIcon           string
	TeamColorOn        string
	TeamColorOff       string
	RequiredObjectives int
	Timed              int
	Attackers          int
	WonRound           string
	LostRound          string
	RoundOverSoundWon  string
	RoundOverSoundLost string
	RoundOverTarget    string
	Briefing           string
	FriendlyShader     string
	FlagShader         string
	Objectives         []SiegeObjective
	ExtraFields        map[string]string
}

type SiegeData struct {
	TeamsMap         map[string]string // team1 -> Name, team2 -> Name
	Team1            *SiegeTeam
	Team2            *SiegeTeam
	
	MissionName      string
	MapGraphic       string
	RadarTopLeft     string
	RadarBottomRight string
	MBModesAllowed   string
	RoundBeginTarget string
	HelpIcons        string // Raw block for now, or parsed?
	AutoMap          string // Raw block
	LevelshotDesc    string // Raw block
	ExtraFields      map[string]string
}

func NewSiegeData() *SiegeData {
	return &SiegeData{
		TeamsMap:    make(map[string]string),
		ExtraFields: make(map[string]string),
	}
}

// ParseSiege parses a .siege file content using a brace-counting tokenizer.
func ParseSiege(content string) (*SiegeData, error) {
	siege := NewSiegeData()
	
	// Pre-process: Remove comments
	cleanContent := stripComments(content)
	
	// Tokenize
	tokens := tokenize(cleanContent)
	
	i := 0
	for i < len(tokens) {
		key := tokens[i]
		i++
		
		if i >= len(tokens) { break }
		
		val := tokens[i]
		
		if val == "{" {
			// Block start
			blockContent, newIdx := extractBlock(tokens, i)
			i = newIdx
			
			processGlobalBlock(siege, key, blockContent)
		} else {
			// Key-Value pair
			i++ // consume value
			processGlobalField(siege, key, val)
		}
	}
	
	return siege, nil
}

func stripComments(content string) string {
	var sb strings.Builder
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		if idx := strings.Index(line, "//"); idx != -1 {
			line = line[:idx]
		}
		sb.WriteString(strings.TrimSpace(line) + "\n")
	}
	return sb.String()
}

// Simple tokenizer that splits by whitespace but treats quoted strings and {} as tokens
func tokenize(content string) []string {
	var tokens []string
	var currentToken strings.Builder
	inQuote := false
	
	for j := 0; j < len(content); j++ {
		c := content[j]
		
		if inQuote {
			if c == '"' {
				inQuote = false
				tokens = append(tokens, currentToken.String())
				currentToken.Reset()
			} else {
				currentToken.WriteByte(c)
			}
		} else {
			if c == '"' {
				if currentToken.Len() > 0 {
					tokens = append(tokens, currentToken.String())
					currentToken.Reset()
				}
				inQuote = true
			} else if c == '{' || c == '}' {
				if currentToken.Len() > 0 {
					tokens = append(tokens, currentToken.String())
					currentToken.Reset()
				}
				tokens = append(tokens, string(c))
			} else if c == ' ' || c == '\t' || c == '\n' || c == '\r' {
				if currentToken.Len() > 0 {
					tokens = append(tokens, currentToken.String())
					currentToken.Reset()
				}
			} else {
				currentToken.WriteByte(c)
			}
		}
	}
	if currentToken.Len() > 0 {
		tokens = append(tokens, currentToken.String())
	}
	return tokens
}

func extractBlock(tokens []string, startIdx int) ([]string, int) {
	// startIdx points to "{"
	depth := 1
	i := startIdx + 1
	blockStart := i
	
	for i < len(tokens) {
		if tokens[i] == "{" {
			depth++
		} else if tokens[i] == "}" {
			depth--
			if depth == 0 {
				return tokens[blockStart:i], i + 1
			}
		}
		i++
	}
	return []string{}, i
}

func processGlobalBlock(siege *SiegeData, key string, content []string) {
	if strings.ToLower(key) == "teams" {
		// Parse Teams block
		i := 0
		for i < len(content)-1 {
			k := content[i]
			v := content[i+1]
			siege.TeamsMap[strings.ToLower(k)] = v
			i += 2
		}
	} else if strings.ToLower(key) == "helpicons" {
		siege.HelpIcons = reconstructBlock(content)
	} else if strings.ToLower(key) == "automap" {
		siege.AutoMap = reconstructBlock(content)
	} else if strings.ToLower(key) == "levelshotdesc" {
		siege.LevelshotDesc = reconstructBlock(content)
	} else {
		// Check if this key matches a Team Name
		isTeam := false
		for _, teamName := range siege.TeamsMap {
			if teamName == key {
				isTeam = true
				team := parseTeamBlock(key, content)
				if siege.TeamsMap["team1"] == key {
					siege.Team1 = team
					team.TeamName = "team1"
				} else {
					siege.Team2 = team
					team.TeamName = "team2"
				}
				break
			}
		}
		if !isTeam {
			siege.ExtraFields[key] = reconstructBlock(content) // Store unknown blocks as string
		}
	}
}

func processGlobalField(siege *SiegeData, key, value string) {
	k := strings.ToLower(key)
	switch k {
	case "missionname": siege.MissionName = value
	case "mapgraphic": siege.MapGraphic = value
	case "radartopleft": siege.RadarTopLeft = value
	case "radarbottomright": siege.RadarBottomRight = value
	case "mbmodesallowed": siege.MBModesAllowed = value
	case "roundbegin_target": siege.RoundBeginTarget = value
	default: siege.ExtraFields[key] = value
	}
}

func parseTeamBlock(name string, content []string) *SiegeTeam {
	team := &SiegeTeam{Name: name, ExtraFields: make(map[string]string)}
	i := 0
	for i < len(content) {
		key := content[i]
		i++
		if i >= len(content) { break }
		
		if content[i] == "{" {
			// Nested Block (Objective)
			blockContent, newIdx := extractBlock(content, i)
			i = newIdx
			if strings.HasPrefix(strings.ToLower(key), "objective") {
				obj := parseObjective(key, blockContent)
				team.Objectives = append(team.Objectives, *obj)
			} else {
				team.ExtraFields[key] = reconstructBlock(blockContent)
			}
		} else {
			val := content[i]
			i++
			setTeamField(team, key, val)
		}
	}
	return team
}

func setTeamField(team *SiegeTeam, key, value string) {
	switch strings.ToLower(key) {
	case "useteam": team.UseTeam = value
	case "teamicon": team.TeamIcon = value
	case "teamcoloron": team.TeamColorOn = value
	case "teamcoloroff": team.TeamColorOff = value
	case "requiredobjectives": team.RequiredObjectives, _ = strconv.Atoi(value)
	case "timed": team.Timed, _ = strconv.Atoi(value)
	case "attackers": team.Attackers, _ = strconv.Atoi(value)
	case "wonround": team.WonRound = value
	case "lostround": team.LostRound = value
	case "roundover_sound_wewon": team.RoundOverSoundWon = value
	case "roundover_sound_welost": team.RoundOverSoundLost = value
	case "roundover_target": team.RoundOverTarget = value
	case "briefing": team.Briefing = value
	default: team.ExtraFields[key] = value
	}
}

func parseObjective(name string, content []string) *SiegeObjective {
	obj := &SiegeObjective{Name: name, ExtraFields: make(map[string]string)}
	i := 0
	for i < len(content) {
		key := content[i]
		i++
		if i >= len(content) { break }
		
		// Objectives usually don't have nested blocks, but if they do...
		if content[i] == "{" {
			blockContent, newIdx := extractBlock(content, i)
			i = newIdx
			obj.ExtraFields[key] = reconstructBlock(blockContent)
		} else {
			val := content[i]
			i++
			setObjectiveField(obj, key, val)
		}
	}
	return obj
}

func setObjectiveField(obj *SiegeObjective, key, value string) {
	switch strings.ToLower(key) {
	case "goalname": obj.GoalName = value
	case "final": obj.Final, _ = strconv.Atoi(value)
	case "objdesc": obj.ObjDesc = value
	case "objgfx": obj.ObjGfx = value
	case "mapicon": obj.MapIcon = value
	case "litmapicon": obj.LitMapIcon = value
	case "donemapicon": obj.DoneMapIcon = value
	case "mappos": obj.MapPos = value
	case "message_team1": obj.MessageTeam1 = value
	case "message_team2": obj.MessageTeam2 = value
	case "sound_team1": obj.SoundTeam1 = value
	case "sound_team2": obj.SoundTeam2 = value
	case "target": obj.Target = value
	default: obj.ExtraFields[key] = value
	}
}

func reconstructBlock(tokens []string) string {
	// Reconstructs a block string from tokens (simplified)
	// This loses formatting but preserves data
	var sb strings.Builder
	sb.WriteString("{\n")
	i := 0
	for i < len(tokens) {
		k := tokens[i]
		i++
		if i >= len(tokens) { break }
		if tokens[i] == "{" {
			// Nested
			nested, newIdx := extractBlock(tokens, i)
			i = newIdx
			sb.WriteString(fmt.Sprintf("\t%s %s\n", k, reconstructBlock(nested)))
		} else {
			v := tokens[i]
			i++
			sb.WriteString(fmt.Sprintf("\t%s \"%s\"\n", k, v))
		}
	}
	sb.WriteString("}")
	return sb.String()
}

func GenerateSiege(siege *SiegeData) (string, error) {
	var sb strings.Builder
	
	// Global Fields
	if siege.MissionName != "" { fmt.Fprintf(&sb, "missionname \"%s\"\n", siege.MissionName) }
	if siege.MapGraphic != "" { fmt.Fprintf(&sb, "mapgraphic \"%s\"\n", siege.MapGraphic) }
	if siege.RadarTopLeft != "" { fmt.Fprintf(&sb, "radartopleft \"%s\"\n", siege.RadarTopLeft) }
	if siege.RadarBottomRight != "" { fmt.Fprintf(&sb, "radarbottomright \"%s\"\n", siege.RadarBottomRight) }
	if siege.MBModesAllowed != "" { fmt.Fprintf(&sb, "MBModesAllowed \"%s\"\n", siege.MBModesAllowed) }
	if siege.RoundBeginTarget != "" { fmt.Fprintf(&sb, "roundbegin_target \"%s\"\n", siege.RoundBeginTarget) }
	
	for k, v := range siege.ExtraFields {
		if strings.HasPrefix(v, "{") {
			fmt.Fprintf(&sb, "%s\n%s\n", k, v) // Block
		} else {
			fmt.Fprintf(&sb, "%s \"%s\"\n", k, v)
		}
	}
	
	// Teams
	fmt.Fprintf(&sb, "\nTeams\n{\n")
	if siege.Team1 != nil { fmt.Fprintf(&sb, "\tteam1 %s\n", siege.Team1.Name) }
	if siege.Team2 != nil { fmt.Fprintf(&sb, "\tteam2 %s\n", siege.Team2.Name) }
	fmt.Fprintf(&sb, "}\n\n")
	
	// HelpIcons
	if siege.HelpIcons != "" { fmt.Fprintf(&sb, "HelpIcons\n%s\n\n", siege.HelpIcons) }
	if siege.LevelshotDesc != "" { fmt.Fprintf(&sb, "LevelshotDesc\n%s\n\n", siege.LevelshotDesc) }
	if siege.AutoMap != "" { fmt.Fprintf(&sb, "AutoMap\n%s\n\n", siege.AutoMap) }
	
	// Team Blocks
	if siege.Team1 != nil { generateTeamBlock(&sb, siege.Team1) }
	if siege.Team2 != nil { generateTeamBlock(&sb, siege.Team2) }
	
	return sb.String(), nil
}

func generateTeamBlock(sb *strings.Builder, team *SiegeTeam) {
	fmt.Fprintf(sb, "%s\n{\n", team.Name)
	fmt.Fprintf(sb, "\tRequiredObjectives %d\n", team.RequiredObjectives)
	fmt.Fprintf(sb, "\tTimed %d\n", team.Timed)
	fmt.Fprintf(sb, "\tattackers %d\n", team.Attackers)
	if team.UseTeam != "" { fmt.Fprintf(sb, "\tUseTeam \"%s\"\n", team.UseTeam) }
	if team.TeamIcon != "" { fmt.Fprintf(sb, "\tTeamIcon \"%s\"\n", team.TeamIcon) }
	if team.TeamColorOn != "" { fmt.Fprintf(sb, "\tTeamColorOn \"%s\"\n", team.TeamColorOn) }
	if team.TeamColorOff != "" { fmt.Fprintf(sb, "\tTeamColorOff \"%s\"\n", team.TeamColorOff) }
	if team.WonRound != "" { fmt.Fprintf(sb, "\twonround \"%s\"\n", team.WonRound) }
	if team.LostRound != "" { fmt.Fprintf(sb, "\tlostround \"%s\"\n", team.LostRound) }
	if team.Briefing != "" { fmt.Fprintf(sb, "\tbriefing \"%s\"\n", team.Briefing) }
	
	for _, obj := range team.Objectives {
		fmt.Fprintf(sb, "\n\t%s\n\t{\n", obj.Name)
		fmt.Fprintf(sb, "\t\tgoalname \"%s\"\n", obj.GoalName)
		fmt.Fprintf(sb, "\t\tfinal %d\n", obj.Final)
		if obj.ObjDesc != "" { fmt.Fprintf(sb, "\t\tobjdesc \"%s\"\n", obj.ObjDesc) }
		if obj.ObjGfx != "" { fmt.Fprintf(sb, "\t\tobjgfx \"%s\"\n", obj.ObjGfx) }
		if obj.MessageTeam1 != "" { fmt.Fprintf(sb, "\t\tmessage_team1 \"%s\"\n", obj.MessageTeam1) }
		if obj.MessageTeam2 != "" { fmt.Fprintf(sb, "\t\tmessage_team2 \"%s\"\n", obj.MessageTeam2) }
		// ... output other fields ...
		for k, v := range obj.ExtraFields {
			fmt.Fprintf(sb, "\t\t%s \"%s\"\n", k, v)
		}
		fmt.Fprintf(sb, "\t}\n")
	}
	
	for k, v := range team.ExtraFields {
		fmt.Fprintf(sb, "\t%s \"%s\"\n", k, v)
	}
	fmt.Fprintf(sb, "}\n\n")
}
