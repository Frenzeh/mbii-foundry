package parsers

import "fmt"
import "strings"

func syncVehicleToAST(veh *VehicleData, block *ASTBlock) {
	if block.NameToken != nil {
		block.NameToken.Text = veh.Name
	}

	syncStr := func(k, v, def string, quote bool) {
		_, found := getFieldValue(block, k)
		if v != def {
			setFieldValue(block, k, v, quote)
		} else if found {
			setFieldValue(block, k, v, quote)
		}
	}

	syncStr("name", veh.Name, "", false)
	syncStr("type", veh.Type, "VH_SPEEDER", false)
	syncStr("model", veh.Model, "", false)
	syncStr("skin", veh.Skin, "", false)

	syncFloat := func(k string, v, def float64) {
		_, found := getFieldValue(block, k)
		if v != def {
			setFieldValue(block, k, fmt.Sprintf("%.1f", v), false)
		} else if found {
			setFieldValue(block, k, fmt.Sprintf("%.1f", v), false)
		}
	}

	syncFloat("speedMax", veh.SpeedMax, 0)
	syncFloat("turboSpeed", veh.TurboSpeed, 0)
	syncFloat("acceleration", veh.Accel, 0)
	syncFloat("decelIdle", veh.Decel, 0)
	syncFloat("strafePerc", veh.StrafePerc, 0)
	syncFloat("bankingSpeed", veh.BankingSpeed, 0)
	syncFloat("rollLimit", veh.RollLimit, 0)
	syncFloat("pitchLimit", veh.PitchLimit, 0)
	syncFloat("braking", veh.Braking, 0)
	syncFloat("mouseYaw", veh.MouseYaw, 0)
	syncFloat("mousePitch", veh.MousePitch, 0)
	syncFloat("customGravity", veh.CustomGravity, 0)

	syncInt := func(k string, v, def int) {
		_, found := getFieldValue(block, k)
		if v != def {
			setFieldValue(block, k, fmt.Sprintf("%d", v), false)
		} else if found {
			setFieldValue(block, k, fmt.Sprintf("%d", v), false)
		}
	}

	syncInt("armor", veh.Armor, 0)
	syncInt("shields", veh.Shields, 0)

	syncStr("weapons", veh.Weapons, "", false)

	// ExtraFields — deletion-aware sync: keys the editor removed from
	// the map have their lines removed; live keys are upserted.
	removeStaleExtrasSeq(block, veh.ExtraFields, vehicleFieldTyped)
	upsertExtras(block, veh.ExtraFields)
}

// vehicleFieldTyped reports whether a .veh key is modeled natively by
// parseVehicleBlock. Typed keys are never stale-removed during sync.
func vehicleFieldTyped(key string) bool {
	switch strings.ToLower(key) {
	case "name", "type", "model", "skin",
		"speedmax", "turbospeed", "acceleration", "decelidle", "strafeperc",
		"bankingspeed", "rolllimit", "pitchlimit", "braking",
		"mouseyaw", "mousepitch", "customgravity",
		"armor", "shields", "weapons":
		return true
	}
	return false
}

func ParseVEHDefinition(source string, index int) (*VehicleData, error) {
	tokens, err := Lex(source)
	if err != nil {
		return nil, err
	}
	doc := parseAST(tokens)

	var block *ASTBlock
	var bIdx int
	currIdx := 0
	for i, node := range doc.Nodes {
		if b, ok := node.(*ASTBlock); ok {
			if currIdx == index {
				block = b
				bIdx = i
				break
			}
			currIdx++
		}
	}
	if block == nil {
		return nil, fmt.Errorf("vehicle definition at index %d not found", index)
	}

	veh := NewVehicleData()
	if block.NameToken != nil {
		veh.Name = block.NameToken.Text
	}

	veh.ctx = &sourceContext{
		doc:        doc,
		blockIndex: bIdx,
	}

	// Field parse from token pairs, not line-splitting: quoted values
	// may span lines (one token carries the newlines) and a line-split
	// would truncate them mid-value. First occurrence of a key wins
	// (matches the previous reversed-line parser and the engine's
	// first-effective reading).
	seen := make(map[string]bool)
	walkTokenPairs(block.Children, func(_ int, key string, _ int, val string, hasVal bool) bool {
		lower := strings.ToLower(key)
		if hasVal && !seen[lower] {
			seen[lower] = true
			setVehicleField(veh, key, val)
		}
		return true
	})
	return veh, nil
}
