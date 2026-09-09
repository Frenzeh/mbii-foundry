package parsers

import (
	"fmt"
	"sort"
	"strings"
)

func syncMBCHToAST(char *MBCHCharacter, doc *ASTDocument) {
	var classInfoBlock *ASTBlock
	for _, node := range doc.Nodes {
		if b, ok := node.(*ASTBlock); ok && b.NameToken != nil && strings.ToLower(b.NameToken.Text) == "classinfo" {
			classInfoBlock = b
			break
		}
	}

	if classInfoBlock == nil {
		classInfoBlock = &ASTBlock{
			NameToken:  &ASTToken{Type: TokenString, Text: "ClassInfo"},
			Preamble:   []ASTNode{&ASTToken{Type: TokenWhitespace, Text: "\n"}},
			OpenBrace:  &ASTToken{Type: TokenBraceOpen, Text: "{"},
			Children:   []ASTNode{&ASTToken{Type: TokenWhitespace, Text: "\n"}},
			CloseBrace: &ASTToken{Type: TokenBraceClose, Text: "}"},
		}
		doc.Nodes = append(doc.Nodes, classInfoBlock)
	}

	syncStr := func(block *ASTBlock, k, v, def string, quote bool) {
		_, found := getFieldValue(block, k)
		if v != def {
			setFieldValue(block, k, v, quote)
		} else if found {
			setFieldValue(block, k, v, quote)
		}
	}

	syncInt := func(block *ASTBlock, k string, v, def int) {
		_, found := getFieldValue(block, k)
		if v != def {
			setFieldValue(block, k, fmt.Sprintf("%d", v), false)
		} else if found {
			setFieldValue(block, k, fmt.Sprintf("%d", v), false)
		}
	}

	syncFloat := func(block *ASTBlock, k string, v, def float64) {
		_, found := getFieldValue(block, k)
		if v != def {
			setFieldValue(block, k, fmt.Sprintf("%g", v), false)
		} else if found {
			setFieldValue(block, k, fmt.Sprintf("%g", v), false)
		}
	}

	b := classInfoBlock
	syncStr(b, "name", char.Name, "", true)
	syncStr(b, "MBClass", char.MBClass, "", false)
	syncStr(b, "model", char.Model, "", true)
	syncStr(b, "skin", char.Skin, "", true)
	syncStr(b, "uishader", char.UIShader, "", true)
	syncStr(b, "soundset", char.Soundset, "", true)
	syncStr(b, "weapons", char.Weapons, "", false)
	syncStr(b, "attributes", char.Attributes, "", false)
	syncStr(b, "forcepowers", char.ForcePowers, "", false)
	syncStr(b, "saberstyle", char.SaberStyle, "", false)
	syncStr(b, "classflags", char.ClassFlags, "", false)
	syncInt(b, "maxhealth", char.MaxHealth, 100)
	syncInt(b, "maxarmor", char.MaxArmor, 0)
	syncInt(b, "forcepool", char.ForcePool, 0)
	syncFloat(b, "forceregen", char.ForceRegen, 1.0)
	syncFloat(b, "speed", char.Speed, 1.0)
	syncFloat(b, "APmultiplier", char.APMultiplier, 1.0)
	syncFloat(b, "BPmultiplier", char.BPMultiplier, 1.0)
	syncFloat(b, "CSmultiplier", char.CSMultiplier, 1.0)
	syncFloat(b, "ASmultiplier", char.ASMultiplier, 1.0)
	syncStr(b, "saber1", char.Saber1, "", false)
	syncStr(b, "saber2", char.Saber2, "", false)
	syncInt(b, "sabercolor", char.SaberColor, 0)
	syncInt(b, "saber2color", char.Saber2Color, 0)
	syncInt(b, "classNumberLimit", char.ClassNumberLimit, -1)
	syncInt(b, "respawnCustomTime", char.RespawnCustomTime, 0)
	syncInt(b, "extralives", char.ExtraLives, 0)
	syncInt(b, "iscustombuild", char.IsCustomBuild, 0)
	syncInt(b, "mbPoints", char.MBPoints, 0)
	syncInt(b, "isOnlyOneSpec", char.IsOnlyOneSpec, 0)
	syncInt(b, "defaultSpec", char.DefaultSpec, 0)

	syncInt(b, "hasCustomSpec", char.HasCustomSpec, 0)

	for i := 0; i < 3; i++ {
		suffix := fmt.Sprintf("_%d", i+1)
		syncStr(b, "customSpecName"+suffix, char.CustomSpecNames[i], "", true)
		syncStr(b, "customSpecIcon"+suffix, char.CustomSpecIcons[i], "", true)
		syncStr(b, "customSpecDesc"+suffix, char.CustomSpecDescs[i], "", true)
	}

	for i := 0; i < 45; i++ {
		suffix := fmt.Sprintf("_%d", i)
		syncStr(b, "c_att_skill"+suffix, char.CustomSkills[i], "", false)
		syncStr(b, "c_att_names"+suffix, char.CustomNames[i], "", true)
		syncStr(b, "c_att_ranks"+suffix, char.CustomRanks[i], "", false)
		syncStr(b, "c_att_descs"+suffix, char.CustomDescs[i], "", true)
	}

	// Rank attributes + ExtraFields — deletion-aware sync. The union of
	// both maps is the set of live untyped keys; an untyped key line
	// carrying a value that is neither typed nor live was deleted in the
	// model, and its line is removed from the document.
	liveExtras := make(map[string]string, len(char.ExtraFields)+len(char.RankAttributes))
	for k, v := range char.RankAttributes {
		liveExtras[k] = v
	}
	for k, v := range char.ExtraFields {
		if strings.ToLower(k) == "description" {
			// The description is a top-level key (SGPV reads it from the
			// file buffer, bg_saga.c:2383); it is never duplicated into
			// the ClassInfo extras tail.
			continue
		}
		liveExtras[k] = v
	}
	removeStaleExtras(b, liveExtras, mbchClassInfoTyped)
	for _, k := range sortedKeys(char.RankAttributes) {
		setFieldValue(b, k, char.RankAttributes[k], false)
	}
	for _, k := range sortedKeys(char.ExtraFields) {
		if strings.ToLower(k) == "description" {
			continue
		}
		v := char.ExtraFields[k]
		setFieldValue(b, k, v, strings.ContainsAny(v, " \t\n"))
	}

	// Weapon overrides — identity-keyed by block NAME captured at parse
	// time (wi.astName), never by slice index: files may carry
	// non-contiguous numbering, and renumbering would silently change
	// what the engine loads (its scan stops at the first gap,
	// bg_saga.c:2126-2253). Overrides deleted from the model have their
	// blocks removed; new overrides take indexes AFTER every surviving
	// block so deletions+additions stay gap-free.
	claimedW := make(map[string]bool, len(char.WeaponOverrides))
	for i := range char.WeaponOverrides {
		if astName := char.WeaponOverrides[i].astName; astName != "" {
			claimedW[astName] = true
		}
	}
	removeUnclaimedOverrideBlocks(doc, "weaponinfo", claimedW)
	nextW := maxTopLevelOverrideIndex(doc, "weaponinfo")
	for i := range char.WeaponOverrides {
		wi := &char.WeaponOverrides[i]
		astName := wi.astName
		if astName == "" {
			nextW++
			astName = fmt.Sprintf("weaponinfo%d", nextW)
		}
		wBlock := findTopLevelBlock(doc, astName)
		if wBlock == nil {
			wBlock = &ASTBlock{
				NameToken:  &ASTToken{Type: TokenString, Text: "WeaponInfo" + strings.TrimPrefix(astName, "weaponinfo")},
				Preamble:   []ASTNode{&ASTToken{Type: TokenWhitespace, Text: "\n\n"}},
				OpenBrace:  &ASTToken{Type: TokenBraceOpen, Text: "{"},
				Children:   []ASTNode{&ASTToken{Type: TokenWhitespace, Text: "\n"}},
				CloseBrace: &ASTToken{Type: TokenBraceClose, Text: "}"},
			}
			doc.Nodes = append(doc.Nodes, wBlock)
		}

		syncStr(wBlock, "WeaponToReplace", wi.WeaponToReplace, "", false)
		syncStr(wBlock, "WeaponBasedOff", wi.WeaponBasedOff, "", false)
		syncStr(wBlock, "NewWorldModel", wi.NewWorldModel, "", true)
		syncStr(wBlock, "NewViewModel", wi.NewViewModel, "", true)
		syncStr(wBlock, "Icon", wi.Icon, "", true)
		syncStr(wBlock, "WeaponName", wi.WeaponName, "", true)
		syncStr(wBlock, "MuzzleEffect", wi.MuzzleEffect, "", true)
		syncStr(wBlock, "AltMuzzleEffect", wi.AltMuzzleEffect, "", true)
		syncStr(wBlock, "MissileEffect", wi.MissileEffect, "", true)
		syncStr(wBlock, "AltMissileEffect", wi.AltMissileEffect, "", true)
		syncStr(wBlock, "Missile3Effect", wi.Missile3Effect, "", true)
		syncStr(wBlock, "AltMissileEffect3", wi.AltMissileEffect3, "", true)
		syncStr(wBlock, "PowerupShotEffect", wi.PowerupShotEffect, "", true)
		syncStr(wBlock, "PowerupShotEffect3", wi.PowerupShotEffect3, "", true)
		syncStr(wBlock, "FlashSound0", wi.FlashSound0, "", true)
		syncStr(wBlock, "AltFlashSound0", wi.AltFlashSound0, "", true)
		syncStr(wBlock, "ChargeSound", wi.ChargeSound, "", true)
		syncStr(wBlock, "AltChargeSound", wi.AltChargeSound, "", true)
		syncStr(wBlock, "PrimHitSound", wi.PrimHitSound, "", true)
		syncStr(wBlock, "AltHitSound", wi.AltHitSound, "", true)
		syncInt(wBlock, "customAmmo", wi.CustomAmmo, 0)
		syncInt(wBlock, "clipSize", wi.ClipSize, 0)
		syncFloat(wBlock, "reloadTimeModifier", wi.ReloadTimeModifier, 0)
		removeStaleExtras(wBlock, wi.ExtraFields, weaponInfoTyped)
		upsertExtras(wBlock, wi.ExtraFields)
	}

	// Force overrides — same identity scheme as weapon overrides.
	claimedF := make(map[string]bool, len(char.ForceOverrides))
	for i := range char.ForceOverrides {
		if astName := char.ForceOverrides[i].astName; astName != "" {
			claimedF[astName] = true
		}
	}
	removeUnclaimedOverrideBlocks(doc, "forceinfo", claimedF)
	nextF := maxTopLevelOverrideIndex(doc, "forceinfo")
	for i := range char.ForceOverrides {
		fi := &char.ForceOverrides[i]
		astName := fi.astName
		if astName == "" {
			nextF++
			astName = fmt.Sprintf("forceinfo%d", nextF)
		}
		fBlock := findTopLevelBlock(doc, astName)
		if fBlock == nil {
			fBlock = &ASTBlock{
				NameToken:  &ASTToken{Type: TokenString, Text: "ForceInfo" + strings.TrimPrefix(astName, "forceinfo")},
				Preamble:   []ASTNode{&ASTToken{Type: TokenWhitespace, Text: "\n\n"}},
				OpenBrace:  &ASTToken{Type: TokenBraceOpen, Text: "{"},
				Children:   []ASTNode{&ASTToken{Type: TokenWhitespace, Text: "\n"}},
				CloseBrace: &ASTToken{Type: TokenBraceClose, Text: "}"},
			}
			doc.Nodes = append(doc.Nodes, fBlock)
		}

		syncStr(fBlock, "ForceToReplace", fi.ForceToReplace, "", false)
		syncStr(fBlock, "Icon", fi.Icon, "", true)
		syncStr(fBlock, "ForcePowerName", fi.ForcePowerName, "", true)
		syncStr(fBlock, "StartSound", fi.StartSound, "", true)
		syncStr(fBlock, "LoopSound", fi.LoopSound, "", true)
		removeStaleExtras(fBlock, fi.ExtraFields, forceInfoTyped)
		upsertExtras(fBlock, fi.ExtraFields)
	}

	// Top-level description. With the SGPV lexer the value is a single
	// token (possibly spanning lines), so set/remove are exact token
	// operations — no orphaned leading quotes, no doubled quoting.
	if char.Description != "" {
		setDocDescription(doc, char.Description)
	} else {
		removeDocDescription(doc)
	}
}

// docDescriptionValueIdx returns the index of the value token that
// follows a top-level `description` key, or -1.
func docDescriptionValueIdx(doc *ASTDocument) int {
	for i := range doc.Nodes {
		tok, ok := doc.Nodes[i].(*ASTToken)
		if !ok || tok.Type != TokenString || strings.ToLower(unquote(tok.Text)) != "description" {
			continue
		}
		for j := i + 1; j < len(doc.Nodes); j++ {
			switch t := doc.Nodes[j].(type) {
			case *ASTToken:
				if t.Type == TokenWhitespace || t.Type == TokenComment {
					continue
				}
				if t.Type == TokenString {
					return j
				}
				return -1
			case *ASTBlock:
				return -1
			}
		}
		return -1
	}
	return -1
}

func setDocDescription(doc *ASTDocument, desc string) {
	// Raw quotes, never %q: SGPV treats backslash sequences literally,
	// so escapes would leak into the displayed text.
	quoted := fmt.Sprintf("\"%s\"", desc)
	if idx := docDescriptionValueIdx(doc); idx != -1 {
		if t, ok := doc.Nodes[idx].(*ASTToken); ok && t.Text != quoted {
			t.Text = quoted
		}
		return
	}
	doc.Nodes = append(doc.Nodes,
		&ASTToken{Type: TokenWhitespace, Text: "\n"},
		&ASTToken{Type: TokenString, Text: "description"},
		&ASTToken{Type: TokenWhitespace, Text: "\t"},
		&ASTToken{Type: TokenString, Text: quoted},
		&ASTToken{Type: TokenWhitespace, Text: "\n"},
	)
}

func removeDocDescription(doc *ASTDocument) {
	for i := range doc.Nodes {
		tok, ok := doc.Nodes[i].(*ASTToken)
		if !ok || tok.Type != TokenString || strings.ToLower(unquote(tok.Text)) != "description" {
			continue
		}
		if valIdx := docDescriptionValueIdx(doc); valIdx != -1 {
			removeFieldRange(&doc.Nodes, i, valIdx)
		}
		return
	}
}

// mbchClassInfoTyped reports whether a ClassInfo key is modeled natively
// (synced by the typed pass). Typed keys are never stale-removed and
// never written from the extras map.
func mbchClassInfoTyped(key string) bool {
	switch strings.ToLower(key) {
	case "name", "mbclass", "model", "skin", "uishader", "soundset",
		"weapons", "attributes", "forcepowers", "saberstyle", "classflags",
		"maxhealth", "maxarmor", "forcepool", "forceregen", "speed",
		"apmultiplier", "bpmultiplier", "csmultiplier", "asmultiplier",
		"saber1", "saber2", "sabercolor", "saber2color", "classnumberlimit",
		"respawncustomtime", "extralives", "iscustombuild", "mbpoints",
		"isonlyonespec", "defaultspec", "hascustomspec", "description":
		return true
	}
	k := strings.ToLower(key)
	for _, p := range []string{"customspecname_", "customspecicon_", "customspecdesc_",
		"c_att_skill_", "c_att_names_", "c_att_ranks_", "c_att_descs_"} {
		if strings.HasPrefix(k, p) {
			return true
		}
	}
	return false
}

func weaponInfoTyped(key string) bool {
	switch strings.ToLower(key) {
	case "weapontoreplace", "weaponbasedoff", "newworldmodel", "newviewmodel",
		"icon", "weaponname", "muzzleeffect", "altmuzzleeffect", "missileeffect",
		"altmissileeffect", "missile3effect", "altmissileeffect3", "powerupshoteffect",
		"powerupshoteffect3", "flashsound0", "altflashsound0", "chargesound",
		"altchargesound", "primhitsound", "althitsound", "customammo", "clipsize",
		"reloadtimemodifier":
		return true
	}
	return false
}

func forceInfoTyped(key string) bool {
	switch strings.ToLower(key) {
	case "forcetoreplace", "icon", "forcepowername", "startsound", "loopsound":
		return true
	}
	return false
}

func sortedKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// upsertExtras writes the extras map into the block in sorted key order
// (deterministic output), quoting values that need it.
func upsertExtras(block *ASTBlock, extras map[string]string) {
	for _, k := range sortedKeys(extras) {
		v := extras[k]
		setFieldValue(block, k, v, strings.ContainsAny(v, " \t\n"))
	}
}

// removeStaleExtras deletes lines for modeled-at-parse untyped keys that
// are no longer live in the model — the document half of deletion.
func removeStaleExtras(block *ASTBlock, live map[string]string, typed func(string) bool) {
	for {
		removed := false
		walkSGPVPairs(block.Children, func(keyIdx int, key string, valIdx int, _ string, hasVal bool) bool {
			if hasVal && !typed(key) {
				if _, ok := live[key]; !ok {
					removeFieldRange(&block.Children, keyIdx, valIdx)
					removed = true
					return false // indices shifted; restart the scan
				}
			}
			return true
		})
		if !removed {
			break
		}
	}
}

func findTopLevelBlock(doc *ASTDocument, lowerName string) *ASTBlock {
	for _, node := range doc.Nodes {
		if b, ok := node.(*ASTBlock); ok && b.NameToken != nil && strings.ToLower(b.NameToken.Text) == lowerName {
			return b
		}
	}
	return nil
}

// overrideIndex parses the numeric suffix of an override block name
// ("weaponinfo2" → 2); -1 when the name carries no valid number.
func overrideIndex(lowerName, kind string) int {
	n := -1
	fmt.Sscanf(lowerName, kind+"%d", &n)
	return n
}

func maxTopLevelOverrideIndex(doc *ASTDocument, kind string) int {
	max := -1
	for _, node := range doc.Nodes {
		b, ok := node.(*ASTBlock)
		if !ok || b.NameToken == nil {
			continue
		}
		name := strings.ToLower(b.NameToken.Text)
		if !strings.HasPrefix(name, kind) {
			continue
		}
		if n := overrideIndex(name, kind); n > max {
			max = n
		}
	}
	return max
}

// removeUnclaimedOverrideBlocks removes top-level override blocks whose
// identity no live override claims — the document half of deleting an
// override in the editor. Nested blocks are never touched: the engine's
// SGVG cannot see them (they are dead text), so they are preserved
// verbatim.
func removeUnclaimedOverrideBlocks(doc *ASTDocument, kind string, claimed map[string]bool) {
	out := doc.Nodes[:0]
	for _, node := range doc.Nodes {
		keep := true
		if b, ok := node.(*ASTBlock); ok && b.NameToken != nil {
			name := strings.ToLower(b.NameToken.Text)
			if strings.HasPrefix(name, kind) && overrideIndex(name, kind) >= 0 {
				keep = claimed[name]
			}
		}
		if keep {
			out = append(out, node)
		}
	}
	doc.Nodes = out
}
