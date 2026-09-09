package parsers

import (
	"fmt"
	"strings"
)

// Siege canonical AST sync.
//
// syncSiegeToAST reconciles the entire SiegeData model with the
// parse-time document: typed top-level scalars, scalar extras, raw
// blocks (HelpIcons / AutoMap / LevelshotDesc / unknown extras), the
// Teams block, team blocks (typed fields, extra keys, objectives).
// Nothing is cleared as a workaround and no model state is dropped:
//
//   - identity is the parse-time block NAME (astName), so renames
//     update the document in place and deletions remove exactly the
//     claimed blocks;
//   - raw blocks and nested extras are compared in canonical
//     reconstructBlock form, so a no-op save stays byte-exact while a
//     real edit re-renders only the changed block;
//   - stale lines/blocks whose model state was deleted are removed.
func syncSiegeToAST(siege *SiegeData, doc *ASTDocument) {
	setTopField := func(key, val string, useQuotes bool) {
		keyLower := strings.ToLower(key)
		valStr := val
		if useQuotes {
			valStr = fmt.Sprintf("\"%s\"", val)
		}

		found := false
		for i := 0; i < len(doc.Nodes); i++ {
			if tok, ok := doc.Nodes[i].(*ASTToken); ok && tok.Type == TokenString && strings.ToLower(unquote(tok.Text)) == keyLower {
				for j := i + 1; j < len(doc.Nodes); j++ {
					if vTok, ok := doc.Nodes[j].(*ASTToken); ok && vTok.Type == TokenString {
						vTok.Text = valStr
						found = true
						break
					}
					if _, ok := doc.Nodes[j].(*ASTBlock); ok {
						break
					}
				}
				if found {
					break
				}
			}
		}

		if !found {
			doc.Nodes = append([]ASTNode{
				&ASTToken{Type: TokenString, Text: key},
				&ASTToken{Type: TokenWhitespace, Text: " "},
				&ASTToken{Type: TokenString, Text: valStr},
				&ASTToken{Type: TokenWhitespace, Text: "\n"},
			}, doc.Nodes...)
		}
	}

	getTopField := func(key string) (string, bool) {
		keyLower := strings.ToLower(key)
		for i := 0; i < len(doc.Nodes); i++ {
			if tok, ok := doc.Nodes[i].(*ASTToken); ok && tok.Type == TokenString && strings.ToLower(unquote(tok.Text)) == keyLower {
				for j := i + 1; j < len(doc.Nodes); j++ {
					if vTok, ok := doc.Nodes[j].(*ASTToken); ok && vTok.Type == TokenString {
						return unquote(vTok.Text), true
					}
					if _, ok := doc.Nodes[j].(*ASTBlock); ok {
						break
					}
				}
			}
		}
		return "", false
	}

	syncStr := func(k, v, def string, quote bool) {
		_, found := getTopField(k)
		if v != def {
			setTopField(k, v, quote)
		} else if found {
			setTopField(k, v, quote)
		}
	}

	// 1) Typed top-level scalars.
	syncStr("missionname", siege.MissionName, "", true)
	syncStr("mapgraphic", siege.MapGraphic, "", true)
	syncStr("radartopleft", siege.RadarTopLeft, "", true)
	syncStr("radarbottomright", siege.RadarBottomRight, "", true)
	syncStr("MBModesAllowed", siege.MBModesAllowed, "", true)
	syncStr("roundbegin_target", siege.RoundBeginTarget, "", true)

	// 2) Scalar extras — stale removal + upsert. Block-valued extras are
	// canonicalized by the raw-block pass below, so they are invisible
	// here (their keys read as bare keys before a nested block).
	scalarExtras := make(map[string]string)
	for k, v := range siege.ExtraFields {
		if !strings.HasPrefix(v, "{") {
			scalarExtras[k] = v
		}
	}
	for {
		removed := false
		walkSGPVPairs(doc.Nodes, func(keyIdx int, key string, valIdx int, _ string, hasVal bool) bool {
			if hasVal && !siegeTopTyped(key) {
				if _, live := scalarExtras[key]; !live {
					removeFieldRange(&doc.Nodes, keyIdx, valIdx)
					removed = true
					return false
				}
			}
			return true
		})
		if !removed {
			break
		}
	}
	for _, k := range sortedKeys(scalarExtras) {
		v := scalarExtras[k]
		setTopField(k, v, strings.ContainsAny(v, " \t\n"))
	}

	// 3) Raw blocks — HelpIcons / AutoMap / LevelshotDesc / block-valued
	// extras. Compare canonical renders; re-render children only when
	// the model changed; remove on model deletion.
	liveTeams := map[string]bool{}
	for _, team := range []*SiegeTeam{siege.Team1, siege.Team2} {
		if team != nil {
			liveTeams[strings.ToLower(team.astName)] = true
		}
	}
	rawSeen := map[string]bool{}
	handleRaw := func(lower, raw string) {
		if rawSeen[lower] {
			return
		}
		rawSeen[lower] = true
		block := findTopLevelBlock(doc, lower)
		if raw == "" {
			removeTopLevelNamed(doc, lower)
			return
		}
		if block == nil {
			appendRawBlock(doc, lower, raw)
			return
		}
		if canonicalBlockRaw(block.Children) != raw {
			block.Children = renderRawChildren(raw)
		}
	}
	handleRaw("helpicons", siege.HelpIcons)
	handleRaw("automap", siege.AutoMap)
	handleRaw("levelshotdesc", siege.LevelshotDesc)
	for _, k := range sortedKeys(siege.ExtraFields) {
		if v := siege.ExtraFields[k]; strings.HasPrefix(v, "{") {
			handleRaw(strings.ToLower(k), v)
		}
	}

	// 4) Teams block — team1/team2 references mirror the live teams.
	teamsBlock := findTopLevelBlock(doc, "teams")
	if teamsBlock == nil && (siege.Team1 != nil || siege.Team2 != nil) {
		teamsBlock = &ASTBlock{
			NameToken:  &ASTToken{Type: TokenString, Text: "Teams"},
			Preamble:   []ASTNode{&ASTToken{Type: TokenWhitespace, Text: "\n"}},
			OpenBrace:  &ASTToken{Type: TokenBraceOpen, Text: "{"},
			Children:   []ASTNode{&ASTToken{Type: TokenWhitespace, Text: "\n"}},
			CloseBrace: &ASTToken{Type: TokenBraceClose, Text: "}"},
		}
		doc.Nodes = append(doc.Nodes, teamsBlock)
	}
	if teamsBlock != nil {
		for _, slot := range []struct {
			key  string
			team *SiegeTeam
		}{{"team1", siege.Team1}, {"team2", siege.Team2}} {
			if slot.team != nil {
				setFieldValue(teamsBlock, slot.key, slot.team.Name, false)
			} else {
				removeField(teamsBlock, slot.key)
			}
		}
	}

	// 5) Team blocks — GC unclaimed blocks, then canonical sync per team.
	removeUnclaimedTopLevelBlocks(doc, liveTeams, rawSeen)
	for _, team := range []*SiegeTeam{siege.Team1, siege.Team2} {
		if team == nil {
			continue
		}
		if team.astName == "" {
			team.astName = team.Name
		}
		teamBlock := findTopLevelBlock(doc, strings.ToLower(team.astName))
		if teamBlock == nil {
			teamBlock = &ASTBlock{
				NameToken:  &ASTToken{Type: TokenString, Text: team.Name},
				Preamble:   []ASTNode{&ASTToken{Type: TokenWhitespace, Text: "\n"}},
				OpenBrace:  &ASTToken{Type: TokenBraceOpen, Text: "{"},
				Children:   []ASTNode{&ASTToken{Type: TokenWhitespace, Text: "\n"}},
				CloseBrace: &ASTToken{Type: TokenBraceClose, Text: "}"},
			}
			doc.Nodes = append(doc.Nodes, teamBlock)
		}
		if teamBlock.NameToken != nil && teamBlock.NameToken.Text != team.Name {
			teamBlock.NameToken.Text = team.Name // rename in place
		}
		syncSiegeTeamToAST(team, teamBlock)
	}
}

// syncSiegeTeamToAST reconciles one team block: typed fields, nested
// raw blocks, scalar extras and objectives.
func syncSiegeTeamToAST(team *SiegeTeam, block *ASTBlock) {
	syncStr := func(k, v, def string, quote bool) {
		_, found := getFieldValue(block, k)
		if v != def {
			setFieldValue(block, k, v, quote)
		} else if found {
			setFieldValue(block, k, v, quote)
		}
	}
	syncInt := func(k string, v, def int) {
		_, found := getFieldValue(block, k)
		if v != def {
			setFieldValue(block, k, fmt.Sprintf("%d", v), false)
		} else if found {
			setFieldValue(block, k, fmt.Sprintf("%d", v), false)
		}
	}

	syncInt("RequiredObjectives", team.RequiredObjectives, 0)
	syncInt("Timed", team.Timed, 0)
	syncInt("attackers", team.Attackers, 0)
	syncStr("UseTeam", team.UseTeam, "", true)
	syncStr("TeamIcon", team.TeamIcon, "", true)
	syncStr("TeamColorOn", team.TeamColorOn, "", true)
	syncStr("TeamColorOff", team.TeamColorOff, "", true)
	syncStr("wonround", team.WonRound, "", true)
	syncStr("lostround", team.LostRound, "", true)
	syncStr("roundover_sound_wewon", team.RoundOverSoundWon, "", true)
	syncStr("roundover_sound_welost", team.RoundOverSoundLost, "", true)
	syncStr("roundover_target", team.RoundOverTarget, "", true)
	syncStr("briefing", team.Briefing, "", true)

	// Nested raw blocks (block-valued team extras): canonical compare,
	// re-render on change, remove on deletion.
	nestedRaw := map[string]string{}
	for k, v := range team.ExtraFields {
		if strings.HasPrefix(v, "{") {
			nestedRaw[strings.ToLower(k)] = v
		}
	}
	for _, lower := range sortedKeys(nestedRaw) {
		child := findNestedBlock(block, lower)
		if child == nil {
			block.Children = append(block.Children, renderRawNodes(nestedRaw[lower])...)
			continue
		}
		if canonicalBlockRaw(child.Children) != nestedRaw[lower] {
			child.Children = renderRawChildren(nestedRaw[lower])
		}
	}

	// Scalar extras.
	scalarExtras := make(map[string]string)
	for k, v := range team.ExtraFields {
		if !strings.HasPrefix(v, "{") {
			scalarExtras[k] = v
		}
	}
	removeStaleExtras(block, scalarExtras, siegeTeamTyped)
	upsertExtras(block, scalarExtras)

	// Objectives — identity by parse-time block name.
	claimed := map[string]bool{}
	for i := range team.Objectives {
		obj := &team.Objectives[i]
		if obj.astName == "" {
			obj.astName = obj.Name
		}
		claimed[strings.ToLower(obj.astName)] = true
	}
	pruneUnclaimedNestedObjectives(block, claimed)
	next := maxNestedObjectiveIndex(block)
	for i := range team.Objectives {
		obj := &team.Objectives[i]
		objBlock := findNestedBlock(block, strings.ToLower(obj.astName))
		if objBlock == nil {
			if obj.astName == "" {
				next++
				obj.astName = fmt.Sprintf("Objective%d", next)
			}
			objBlock = &ASTBlock{
				NameToken:  &ASTToken{Type: TokenString, Text: obj.astName},
				Preamble:   []ASTNode{&ASTToken{Type: TokenWhitespace, Text: "\n\t"}},
				OpenBrace:  &ASTToken{Type: TokenBraceOpen, Text: "{"},
				Children:   []ASTNode{&ASTToken{Type: TokenWhitespace, Text: "\n\t\t"}},
				CloseBrace: &ASTToken{Type: TokenBraceClose, Text: "}"},
			}
			block.Children = append(block.Children, objBlock)
		}
		if objBlock.NameToken != nil && objBlock.NameToken.Text != obj.Name {
			objBlock.NameToken.Text = obj.Name // rename in place
		}
		syncSiegeObjectiveToAST(obj, objBlock)
	}
}

func syncSiegeObjectiveToAST(obj *SiegeObjective, block *ASTBlock) {
	syncStr := func(k, v, def string, quote bool) {
		_, found := getFieldValue(block, k)
		if v != def {
			setFieldValue(block, k, v, quote)
		} else if found {
			setFieldValue(block, k, v, quote)
		}
	}
	syncInt := func(k string, v, def int) {
		_, found := getFieldValue(block, k)
		if v != def {
			setFieldValue(block, k, fmt.Sprintf("%d", v), false)
		} else if found {
			setFieldValue(block, k, fmt.Sprintf("%d", v), false)
		}
	}

	syncStr("goalname", obj.GoalName, "", true)
	syncInt("final", obj.Final, 0)
	syncStr("objdesc", obj.ObjDesc, "", true)
	syncStr("objgfx", obj.ObjGfx, "", true)
	syncStr("mapicon", obj.MapIcon, "", true)
	syncStr("litmapicon", obj.LitMapIcon, "", true)
	syncStr("donemapicon", obj.DoneMapIcon, "", true)
	syncStr("mappos", obj.MapPos, "", true)
	syncStr("message_team1", obj.MessageTeam1, "", true)
	syncStr("message_team2", obj.MessageTeam2, "", true)
	syncStr("sound_team1", obj.SoundTeam1, "", true)
	syncStr("sound_team2", obj.SoundTeam2, "", true)
	syncStr("target", obj.Target, "", true)

	removeStaleExtras(block, obj.ExtraFields, siegeObjectiveTyped)
	upsertExtras(block, obj.ExtraFields)
}

// siegeTopTyped reports top-level keys modeled natively by the scalar
// sync or structural passes. They are never stale-removed.
func siegeTopTyped(key string) bool {
	switch strings.ToLower(key) {
	case "missionname", "mapgraphic", "radartopleft", "radarbottomright",
		"mbmodesallowed", "roundbegin_target", "teams":
		return true
	}
	return false
}

// siegeTeamTyped reports team-block keys modeled natively; nested
// non-objective blocks and block-valued extras are handled separately.
func siegeTeamTyped(key string) bool {
	switch strings.ToLower(key) {
	case "useteam", "teamicon", "teamcoloron", "teamcoloroff",
		"requiredobjectives", "timed", "attackers", "wonround", "lostround",
		"roundover_sound_wewon", "roundover_sound_welost", "roundover_target",
		"briefing", "friendlyshader", "flagshader":
		return true
	}
	return false
}

// siegeObjectiveTyped reports objective keys modeled natively.
func siegeObjectiveTyped(key string) bool {
	switch strings.ToLower(key) {
	case "goalname", "final", "objdesc", "objgfx", "mapicon", "litmapicon",
		"donemapicon", "mappos", "message_team1", "message_team2",
		"sound_team1", "sound_team2", "target":
		return true
	}
	return false
}

func findNestedBlock(block *ASTBlock, lowerName string) *ASTBlock {
	for _, child := range block.Children {
		if b, ok := child.(*ASTBlock); ok && b.NameToken != nil && strings.ToLower(b.NameToken.Text) == lowerName {
			return b
		}
	}
	return nil
}

// canonicalBlockRaw renders a block's children in the same canonical
// form reconstructBlock produces for parse-side model values, so model
// and document can be compared without formatting noise.
func canonicalBlockRaw(children []ASTNode) string {
	var walk func(nodes []ASTNode)
	tokens := []string{}
	walk = func(nodes []ASTNode) {
		for _, n := range nodes {
			switch t := n.(type) {
			case *ASTToken:
				if t.Type == TokenString {
					tokens = append(tokens, unquote(t.Text))
				}
			case *ASTBlock:
				if t.NameToken != nil {
					tokens = append(tokens, t.NameToken.Text)
				}
				tokens = append(tokens, "{")
				walk(t.Children)
				tokens = append(tokens, "}")
			}
		}
	}
	walk(children)
	return reconstructBlock(tokens)
}

func appendRawBlock(doc *ASTDocument, lower, raw string) {
	block := &ASTBlock{
		NameToken:  &ASTToken{Type: TokenString, Text: lower},
		Preamble:   []ASTNode{&ASTToken{Type: TokenWhitespace, Text: "\n"}},
		OpenBrace:  &ASTToken{Type: TokenBraceOpen, Text: "{"},
		Children:   renderRawChildren(raw),
		CloseBrace: &ASTToken{Type: TokenBraceClose, Text: "}"},
	}
	doc.Nodes = append(doc.Nodes, block)
}

// removeTopLevelNamed removes a top-level block together with its key
// token when one directly precedes (only whitespace/comments between).
func removeTopLevelNamed(doc *ASTDocument, lower string) {
	for i := range doc.Nodes {
		b, ok := doc.Nodes[i].(*ASTBlock)
		if !ok || b.NameToken == nil || strings.ToLower(b.NameToken.Text) != lower {
			continue
		}
		start := i
		for j := start - 1; j >= 0; j-- {
			t, ok := doc.Nodes[j].(*ASTToken)
			if !ok {
				break
			}
			if t.Type == TokenWhitespace || t.Type == TokenComment {
				start = j
				continue
			}
			if t.Type == TokenString {
				start = j // the key token
			}
			break
		}
		doc.Nodes = append(doc.Nodes[:start], doc.Nodes[i+1:]...)
		return
	}
}

// renderRawChildren parses a raw block text (reconstructBlock form,
// with outer braces) into fresh child nodes.
func renderRawChildren(raw string) []ASTNode {
	return renderRawNodes(raw)
}

func renderRawNodes(raw string) []ASTNode {
	inner := strings.TrimSpace(raw)
	inner = strings.TrimPrefix(inner, "{")
	inner = strings.TrimSuffix(inner, "}")
	tokens, err := Lex(inner)
	if err != nil {
		return nil
	}
	sub := parseAST(tokens)
	return sub.Nodes
}

func removeUnclaimedTopLevelBlocks(doc *ASTDocument, claimedTeams, rawSeen map[string]bool) {
	out := doc.Nodes[:0]
	for _, node := range doc.Nodes {
		keep := true
		if b, ok := node.(*ASTBlock); ok && b.NameToken != nil {
			lower := strings.ToLower(b.NameToken.Text)
			if lower != "teams" && !rawSeen[lower] && !claimedTeams[lower] {
				keep = false
			}
		}
		if keep {
			out = append(out, node)
		}
	}
	doc.Nodes = out
}

// pruneUnclaimedNestedObjectives removes nested blocks that carry an
// Objective<N> style name no live objective claims.
func pruneUnclaimedNestedObjectives(block *ASTBlock, claimed map[string]bool) {
	out := block.Children[:0]
	for _, child := range block.Children {
		keep := true
		if b, ok := child.(*ASTBlock); ok && b.NameToken != nil {
			lower := strings.ToLower(b.NameToken.Text)
			if strings.HasPrefix(lower, "objective") && !claimed[lower] {
				keep = false
			}
		}
		if keep {
			out = append(out, child)
		}
	}
	block.Children = out
}

func maxNestedObjectiveIndex(block *ASTBlock) int {
	max := 0
	for _, child := range block.Children {
		b, ok := child.(*ASTBlock)
		if !ok || b.NameToken == nil {
			continue
		}
		lower := strings.ToLower(b.NameToken.Text)
		if !strings.HasPrefix(lower, "objective") {
			continue
		}
		n := 0
		fmt.Sscanf(lower, "objective%d", &n)
		if n > max {
			max = n
		}
	}
	return max
}
