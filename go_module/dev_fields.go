package main

import (
	"sort"

	"fyne.io/fyne/v2/widget"
)

// DevField mirrors the runtime-relevant slice of a schema entry marked
// `"dev": true`. The schema (schemas/mbch_schema.json) is the canonical
// source of truth — this Go-side list exists because Foundry does not
// embed the schema at build time (it stays alongside the source as
// documentation-of-record). TestDevFieldDriftAgainstSchema, in
// dev_fields_test.go, fails the build if the two ever go out of sync,
// so it is safe to treat this list as authoritative at runtime.
type DevField struct {
	// Key is the exact MBCH field name as parsed by bg_saga.c —
	// case matters, since BG_SiegeGetPairedValue does a stricmp
	// lookup but the writer must round-trip the original casing.
	Key string
	// Label is the short human-facing form-row label.
	Label string
	// Description is the tooltip + entry placeholder text.
	Description string
}

// devFieldsRegistry is every ClassInfo-level field marked `"dev": true`
// in the schema, alphabetically sorted for stable form-row ordering.
// Editing this list without also editing schemas/mbch_schema.json
// (or vice-versa) is caught by TestDevFieldDriftAgainstSchema — the
// schema is the source of truth, this is the runtime mirror.
var devFieldsRegistry = []DevField{
	{Key: "AB_FLightningFlags", Label: "AB_FLightningFlags", Description: "Force-lightning ability flag bitmask"},
	{Key: "AB_PDartFlags", Label: "AB_PDartFlags", Description: "Poison-dart flag bitmask"},
	{Key: "AB_RocketFlags", Label: "AB_RocketFlags", Description: "Mandalorian rocket flag bitmask"},
	{Key: "AB_TDartFlags", Label: "AB_TDartFlags", Description: "Tracking-dart flag bitmask"},
	{Key: "AB_WBirdsFlags", Label: "AB_WBirdsFlags", Description: "Whistling-birds flag bitmask"},
	{Key: "AB_WLaserFlags", Label: "AB_WLaserFlags", Description: "Wrist-laser flag bitmask (e.g. 'HELD_ALTRELOAD')"},
	{Key: "forceblocking", Label: "forceblocking", Description: "Force-blocking attribute (parsed in bg_saberLoad.c)"},
	{Key: "jetpackJet2Angles", Label: "jetpackJet2Angles", Description: "Secondary jet rotation 'pitch, yaw, roll'"},
	{Key: "jetpackJet2Offset", Label: "jetpackJet2Offset", Description: "Secondary jet position offset 'x, y, z'"},
	{Key: "jetpackJet2Tag", Label: "jetpackJet2Tag", Description: "Model tag for secondary jet (e.g. *jet2)"},
	{Key: "jetpackJetAngles", Label: "jetpackJetAngles", Description: "Primary jet rotation 'pitch, yaw, roll'"},
	{Key: "jetpackJetOffset", Label: "jetpackJetOffset", Description: "Primary jet position offset 'x, y, z'"},
	{Key: "jetpackJetTag", Label: "jetpackJetTag", Description: "Model tag for primary jet (e.g. *jet1)"},
	{Key: "saberDamageStyle", Label: "saberDamageStyle", Description: "Per-style saber damage overrides (consumed by BG_TranslateSaberDamageStyles)"},
}

// devFieldKeysSorted returns the list as a sorted []string for diff
// comparisons (e.g. against the schema in the drift test).
func devFieldKeysSorted() []string {
	keys := make([]string, len(devFieldsRegistry))
	for i, df := range devFieldsRegistry {
		keys[i] = df.Key
	}
	sort.Strings(keys)
	return keys
}

// buildDevFieldsCard renders the registry as a single labelled Card
// containing a vertical form of <label, entry> pairs. The entries
// read/write through the editor's character.ExtraFields map so dev
// values round-trip via the writer's writeExtraFields alphabetical
// tail (no special-case wiring needed — these are not promoted to
// dedicated struct fields).
//
// The card is appended to e.devSurfaces so applyDeveloperVisibility
// can flip it together with any other dev-flagged UI; the editor
// hides it by default and only shows it when AppConfig.ShowDeveloperFields
// is on (View → Show Developer Fields).
func (e *MBCHEditor) buildDevFieldsCard() *widget.Card {
	items := make([]*widget.FormItem, 0, len(devFieldsRegistry))
	for _, df := range devFieldsRegistry {
		df := df // capture per-iteration
		entry := NewInputEntry()
		entry.SetPlaceHolder(df.Description)
		entry.OnChanged = func(s string) {
			if e.character == nil {
				return
			}
			if e.character.ExtraFields == nil {
				e.character.ExtraFields = make(map[string]string)
			}
			if s == "" {
				delete(e.character.ExtraFields, df.Key)
			} else {
				e.character.ExtraFields[df.Key] = s
			}
			e.markDirty()
		}
		item := widget.NewFormItem(df.Label, entry)
		item.HintText = df.Description
		items = append(items, item)
		e.devFieldEntries[df.Key] = entry
	}
	form := widget.NewForm(items...)
	return widget.NewCard(
		"Advanced (Developer) Fields",
		"Engine-internal fields (bitmasks, jetpack rig, saber damage style). Hidden by default; toggle via View → Show Developer Fields.",
		form,
	)
}

// populateDevFieldsFromCharacter pushes ExtraFields values into the
// dev-field entries during file load + revert. The standard loading-
// flag suppresses the OnChanged → markDirty path so the editor opens
// clean.
func (e *MBCHEditor) populateDevFieldsFromCharacter() {
	if e.character == nil {
		return
	}
	for key, entry := range e.devFieldEntries {
		if entry == nil {
			continue
		}
		entry.SetText(e.character.ExtraFields[key])
	}
}
