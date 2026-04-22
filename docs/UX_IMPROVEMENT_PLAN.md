# MBII Foundry — UX Improvement Plan

## Vision
Transform MBII Foundry from a technical tool requiring knowledge of enum syntax (`MB_ATT_PISTOL,3|MB_ATT_RIFLE,2`) into an intuitive visual editor where users click toggles, adjust sliders, and see rich explanations for every option.

---

## Phase 1: Rich Documentation Schema

### 1.1 Enhance JSON Schemas with Deep Documentation
Every enum value needs:
- **Short name**: Display label (e.g., "Force Push")
- **Description**: 1-2 sentence summary
- **Detailed explanation**: Per-level breakdown of what each rank does
- **Tips**: Gameplay/design tips for FA authors
- **Related**: Links to related attributes
- **Category tags**: For filtering/grouping

**Example enhanced attribute:**
```json
{
  "id": "MB_ATT_PUSH",
  "name": "Force Push",
  "icon": "force_push.png",
  "category": "force_powers",
  "alignment": "light",
  "description": "Push enemies away with the Force",
  "detailed": {
    "overview": "Force Push is a core Light Side power that knocks back enemies. Essential for defensive Jedi builds.",
    "ranks": {
      "1": {
        "name": "Basic Push",
        "fpCost": 20,
        "effect": "Light knockback, minimal damage. Interrupts enemy attacks.",
        "tip": "Good for creating space, not for damage."
      },
      "2": {
        "name": "Powerful Push",
        "fpCost": 30,
        "effect": "Strong knockback, can knock enemies off ledges. Moderate damage on wall impact.",
        "tip": "Effective near environmental hazards."
      },
      "3": {
        "name": "Force Wave",
        "fpCost": 50,
        "effect": "360° push wave, knockdown on hit. Can affect multiple targets.",
        "tip": "Use when surrounded. Pairs well with Saber Throw."
      }
    }
  },
  "related": ["MB_ATT_PULL", "MB_ATT_FORCEPOOL"],
  "tags": ["force", "light_side", "defensive", "crowd_control"]
}
```

### 1.2 Documentation Categories to Create

| Category | # Items | Priority | Notes |
|----------|---------|----------|-------|
| **Attributes (MB_ATT_*)** | 150+ | HIGH | Core of character building |
| **Weapons (WP_*)** | 58 | HIGH | Most commonly edited |
| **Force Powers (FP_*)** | 24 | HIGH | Complex, needs per-level docs |
| **Saber Styles (SS_*)** | 9 | MEDIUM | Simple but need combat tips |
| **Classes (MB_CLASS_*)** | 26 | MEDIUM | Need team/era context |
| **Class Flags (CFL_*)** | 24 | MEDIUM | Technical, need clear explanations |
| **Holdables (HI_*)** | 45+ | MEDIUM | Equipment/abilities |
| **MBCH Fields** | 300+ | LOW | Many are self-explanatory |
| **SAB Fields** | 100+ | LOW | Saber-specific properties |
| **VEH Fields** | 100+ | LOW | Vehicle properties |

### 1.3 Research Sources for Documentation
1. **MBII Wiki** - Official documentation
2. **bg_public.h / bg_classes.h** - Source code comments
3. **MBII Discord/Forums** - Community knowledge
4. **In-game testing** - Verify actual behavior
5. **AI analysis of codebase** - Extract patterns and relationships

---

## Phase 2: Intuitive Editing Widgets

### 2.1 Widget Types Needed

#### A. Toggle Grid (for multi-select enums)
```
┌─ Force Powers ─────────────────────────────────────┐
│ [✓] Push      [1][2][●3]  ⓘ "Push enemies..."     │
│ [✓] Pull      [●1][2][3]  ⓘ "Pull enemies..."     │
│ [ ] Speed     [1][2][3]   ⓘ "Move faster..."      │
│ [ ] Grip      [1][2][3]   ⓘ "Choke enemies..."    │
└────────────────────────────────────────────────────┘
```
- Checkbox to enable/disable
- Level buttons (only show when enabled)
- Info icon shows tooltip on hover
- Compact grid layout by category

#### B. Flag Picker (for bitfield flags)
```
┌─ Class Flags ──────────────────────────────────────┐
│ ○ CFL_HASQ3           "Quake 3 movement physics"  │
│ ● CFL_HEAVYMELEE      "Enhanced melee damage"     │
│ ○ CFL_REALTD          "Real thermal detonators"   │
│ ● CFL_BPFREEJUMPS     "Free jump BP cost"         │
└────────────────────────────────────────────────────┘
```
- Simple toggle buttons
- Inline description
- Organized by function

#### C. Dropdown with Preview (for single-select)
```
┌─ Base Class ───────────────────────────────────────┐
│ [▼ MB_CLASS_JEDI                              ]    │
│ ┌─────────────────────────────────────────────┐    │
│ │ 🗡️ Jedi Knight                              │    │
│ │ Team: Heroes | Era: Any | Force User        │    │
│ │ Light side warrior with lightsaber combat   │    │
│ └─────────────────────────────────────────────┘    │
└────────────────────────────────────────────────────┘
```
- Dropdown selection
- Rich preview panel below
- Shows related info (team, era, abilities)

#### D. Numeric Slider with Labels
```
┌─ Damage Multiplier ────────────────────────────────┐
│ [0.5]━━━━━━●━━━━━━━━━━[2.0]   Current: 1.2        │
│ "Character deals 20% more damage than normal"      │
└────────────────────────────────────────────────────┘
```
- Visual slider
- Real-time value display
- Contextual description updates

#### E. Asset Picker with Preview
```
┌─ Character Model ──────────────────────────────────┐
│ [models/players/jedi_hm/model.glm      ] [Browse] │
│ ┌─────────────────────────────────────────────┐    │
│ │         [3D Preview or Icon]                │    │
│ │         jedi_hm                             │    │
│ │         Skins: default, red, blue           │    │
│ └─────────────────────────────────────────────┘    │
└────────────────────────────────────────────────────┘
```
- Text field + browse button
- Preview panel (icon or model thumbnail)
- Available skins listed

### 2.2 Widget Implementation Order
1. **Toggle Grid** - Most impactful (attributes, weapons, force powers)
2. **Flag Picker** - Needed for classflags, weapon flags
3. **Dropdown with Preview** - Classes, saber types
4. **Numeric Slider** - Multipliers, stats
5. **Asset Picker** - Models, sounds, effects

---

## Phase 3: Editor Layout Redesign

### 3.1 Character Editor (MBCH) Sections

```
┌────────────────────────────────────────────────────────────────┐
│ [Identity] [Combat] [Force] [Equipment] [Stats] [Advanced]    │
├────────────────────────────────────────────────────────────────┤
│                                                                │
│  ┌─ Identity ───────────────────────────────────────────────┐  │
│  │ Name: [_________________________]                        │  │
│  │ Class: [▼ MB_CLASS_JEDI         ] [Preview Panel]       │  │
│  │ Model: [models/players/...      ] [Browse] [👁]         │  │
│  │ Skin:  [▼ default               ]                       │  │
│  └──────────────────────────────────────────────────────────┘  │
│                                                                │
│  ┌─ Combat Equipment ───────────────────────────────────────┐  │
│  │ [Toggle Grid: Weapons by Category]                       │  │
│  │ ┌─ Pistols ─┐ ┌─ Rifles ─┐ ┌─ Heavy ─┐ ┌─ Grenades ─┐  │  │
│  │ │ [✓] P1   │ │ [✓] R1   │ │ [ ] Conc│ │ [✓] Therm  │  │  │
│  │ │ [ ] P2   │ │ [ ] R2   │ │ [ ] PLX │ │ [ ] Frag   │  │  │
│  │ └──────────┘ └──────────┘ └─────────┘ └────────────┘  │  │
│  └──────────────────────────────────────────────────────────┘  │
│                                                                │
└────────────────────────────────────────────────────────────────┘
```

### 3.2 Tab Organization

| Tab | Contents | Widgets Used |
|-----|----------|--------------|
| **Identity** | Name, class, model, skin, UI | Dropdowns, asset picker |
| **Combat** | Weapons, grenades, melee | Toggle grids |
| **Force** | Force powers, saber styles | Toggle grids with levels |
| **Equipment** | Holdables, gadgets, abilities | Toggle grids |
| **Stats** | Health, armor, multipliers | Sliders, numeric inputs |
| **Advanced** | Flags, overrides, custom specs | Flag picker, raw editor |

---

## Phase 4: Implementation Roadmap

### Sprint 1: Documentation Foundation (Week 1-2)
- [ ] Define JSON schema format for rich documentation
- [ ] Create template for attribute documentation
- [ ] Document 50 most common attributes with full details
- [ ] Build documentation loader in Go

### Sprint 2: Core Widgets (Week 2-3)
- [ ] Implement ToggleGrid widget
- [ ] Implement FlagPicker widget
- [ ] Implement RichDropdown widget
- [ ] Add tooltip system

### Sprint 3: Editor Integration (Week 3-4)
- [ ] Refactor MBCH editor to use new widgets
- [ ] Add tabbed section layout
- [ ] Connect widgets to data model
- [ ] Test save/load functionality

### Sprint 4: Polish & Complete Docs (Week 4+)
- [ ] Complete documentation for all enums
- [ ] Add SAB editor improvements
- [ ] Add VEH editor improvements
- [ ] User testing and refinement

---

## Technical Decisions

### Documentation Storage
- **Option A**: Embedded in Go code (current approach)
  - Pros: No external files, always available
  - Cons: Hard to update, bloats binary

- **Option B**: External JSON files loaded at runtime
  - Pros: Easy to update, can be community-contributed
  - Cons: Need to bundle/find files

- **Recommendation**: **Option B** - JSON files in `schemas/docs/` directory
  - Can ship with app bundle
  - Easy for community to contribute
  - AI can help generate/update

### Tooltip Implementation
- Fyne doesn't have native tooltips
- Options:
  1. Info button that shows popup (current)
  2. Custom hover detection with overlay
  3. Status bar shows description on hover

- **Recommendation**: Hybrid approach
  - Short description inline
  - Info button for detailed popup
  - Status bar for focused item

---

## Next Steps

1. **Approve this plan** - Does this direction make sense?
2. **Prioritize**: Which area needs work first?
3. **Documentation sprint**: Use AI to generate initial docs
4. **Widget prototype**: Build ToggleGrid first as proof of concept

---

## Questions for You

1. Should documentation be embedded or external JSON?
2. Which editor section is most painful currently?
3. Do you want category icons/colors?
4. Should we support keyboard navigation?
5. Any specific attributes that desperately need explanation?
