#!/usr/bin/env python3
"""
MBCH Parser - Parse and write MBII Full Authentic character class files (.mbch)

This parser reads/writes .mbch files following the format parsed by
BG_SiegeParseClassFile() in bg_saga.c

Usage:
    parser = MBCHParser()
    character = parser.parse_file("path/to/file.mbch")
    print(character.name, character.weapons)

    # Modify and write back
    character.maxhealth = 150
    parser.write_file(character, "path/to/output.mbch")
"""

import re
import json
from pathlib import Path
from dataclasses import dataclass, field
from typing import Optional, List, Dict, Any, Union


@dataclass
class WeaponInfo:
    """Weapon override block data - comprehensive from bg_saga.c ParseWeaponOverrides()"""
    index: int = 0

    # Required fields
    WeaponToReplace: str = ""      # WP_* weapon type to replace
    WeaponBasedOff: str = ""       # WP_* base weapon behavior
    NewWorldModel: str = ""         # World model path
    NewViewModel: str = ""          # First-person view model path
    Icon: str = ""                  # HUD icon shader path

    # Optional models
    NewHandsModel: str = ""
    NewBarrelModel: str = ""
    CorrectedWorldModel: str = ""
    missileModel: str = ""
    altMissileModel: str = ""

    # Fire control
    altFireEnabled: int = 1
    primFireEnabled: int = 1
    clipSize: int = 0
    isMinigun: int = 0
    customAmmo: str = ""

    # Hitscan configuration
    primHitscanShot: str = ""
    primHitscanTracer: str = ""
    altHitscanShot: str = ""
    altHitscanTracer: str = ""

    # Effects - Primary
    MuzzleEffect: str = ""
    missileEffect: str = ""
    missileMissEffect: str = ""
    missileMissEffectEnhanced: str = ""
    missileHitHumanEffect: str = ""
    missileHitDroidEffect: str = ""

    # Effects - Alt fire
    AltMuzzleEffect: str = ""
    AltmissileEffect: str = ""

    # Charging effects
    ChargeEffect: str = ""

    # Sounds - Primary (0-3)
    FlashSound0: str = ""
    FlashSound1: str = ""
    FlashSound2: str = ""
    FlashSound3: str = ""

    # Sounds - Alt fire (0-3)
    AltFlashSound0: str = ""
    AltFlashSound1: str = ""
    AltFlashSound2: str = ""
    AltFlashSound3: str = ""

    # Other sounds
    ChargeSound: str = ""
    SelectSound: str = ""
    missileSound: str = ""

    # Modifiers
    ReloadTimeModifier: float = 1.0
    damageMod: float = 1.0
    velocityMod: float = 1.0
    rateMod: float = 1.0

    # Force point block modifiers
    FPBlockMinMult: float = 1.0
    FPBlockMaxMult: float = 1.0
    FPNoBlockMinMult: float = 1.0
    FPNoBlockMaxMult: float = 1.0
    FPChargeMult: float = 1.0
    FPMult: float = 1.0

    # Animation overrides
    hasAnimOverrides: int = 0
    animReady: str = ""
    animAttack: str = ""
    animCharge: str = ""
    typeCharge: str = ""

    # Legacy compatibility
    weapon: str = ""               # Old field name for WeaponToReplace
    WeaponModel: str = ""          # Old field name for NewWorldModel
    ProjectileEffect: str = ""     # Old field name for missileEffect


@dataclass
class ForceInfo:
    """Force power override block data - comprehensive from bg_saga.c ParseForceOverrides()"""
    index: int = 0

    # Required fields
    ForceToReplace: str = ""       # FP_* force power to replace
    Icon: str = ""                  # Force power icon shader
    ForcePowerName: str = ""       # Display name

    # Hand shaders (primary effect)
    HandShader: str = ""           # Base hand effect shader
    HandShaderRed: int = 0         # RGB color red (0-255)
    HandShaderGreen: int = 0       # RGB color green (0-255)
    HandShaderBlue: int = 0        # RGB color blue (0-255)

    # Hand shaders (secondary/alt effect)
    HandShader2: str = ""
    HandShader2Red: int = 0
    HandShader2Green: int = 0
    HandShader2Blue: int = 0

    # Effects
    ConeEffect: str = ""           # Area of effect cone

    # Sounds
    StartSound: str = ""           # Sound when power activates
    LoopSound: str = ""            # Continuous sound during power
    HitSound: str = ""             # Sound on hit/impact

    # Legacy compatibility
    forcepower: str = ""           # Old field name for ForceToReplace
    CastSound: str = ""            # Old field name for StartSound
    CastEffect: str = ""           # Old field name for ConeEffect
    HitEffect: str = ""            # Old field name
    LoopEffect: str = ""           # Old field name
    HandEffect: str = ""           # Old field name for HandShader


@dataclass
class MBCHCharacter:
    """Full MBCH character definition - comprehensive from bg_saga.c BG_SiegeParseClassFile()"""

    # === IDENTITY & BASIC INFO ===
    name: str = ""
    MBClass: str = "MB_CLASS_NOCLASS"
    siegeTeam: int = 0             # Team assignment

    # === MODEL & APPEARANCE (with variant support _1 through _20) ===
    model: str = ""
    skin: str = "default"
    uishader: str = ""             # Icon for class selection UI
    uiglobal: str = ""             # Global UI shader
    modelscale: float = 1.0
    soundset: str = ""

    # Custom RGB colors
    customred: int = 0             # RGB red (0-255)
    customgreen: int = 0           # RGB green (0-255)
    customblue: int = 0            # RGB blue (0-255)
    userRGB: int = 0               # Enable user RGB customization

    # UI overlay shaders (left/center/right)
    uioverlay_l: str = ""
    uioverlay_c: str = ""
    uioverlay_r: str = ""

    # === SABER CONFIGURATION ===
    saber1: str = ""
    saber2: str = ""
    sabercolor: int = 0            # 0-8 color index
    saber2color: int = 0
    saberstyle: str = ""           # SS_FAST|SS_MEDIUM|SS_STRONG etc
    saberStance: int = 0           # Default stance animation
    saberDamage: float = 1.0

    # === EQUIPMENT & LOADOUT ===
    weapons: str = ""              # WP_SABER|WP_MELEE|WP_BLASTER etc
    holdables: str = ""            # HI_JETPACK,3|HI_BINOCULARS,1 etc
    powerups: str = ""             # PW_* powerups
    attributes: str = ""           # MB_ATT_* attributes
    forcepowers: str = ""          # FP_PUSH,3|FP_PULL,3 etc
    classflags: str = ""           # CFL_HASQ3|CFL_HEAVYMELEE etc

    # === STATS ===
    maxhealth: int = 100
    maxarmor: int = 0
    starthealth: int = 0           # Starting health (if different from max)
    startarmor: int = 0            # Starting armor (if different from max)
    forcepool: int = 0
    forceregen: float = 1.0
    speed: float = 1.0
    baseSpeed: float = 1.0         # Base movement speed multiplier

    # === MULTIPLIERS ===
    APmultiplier: float = 1.0      # Action Points multiplier
    BPmultiplier: float = 1.0      # Block Points multiplier
    CSmultiplier: float = 1.0      # Combo Speed multiplier
    ASmultiplier: float = 1.0      # Attack Speed multiplier
    FPmultiplier: float = 1.0      # Force Points multiplier

    # === DAMAGE MODIFIERS ===
    damageTaken: float = 1.0
    damageGiven: float = 1.0
    rateOfFire: float = 1.0
    rateOfFire_Melee: float = 1.0  # Melee-specific fire rate
    knockbackTaken: float = 1.0
    knockbackGiven: float = 1.0
    skillTimerMod: float = 1.0     # Ability cooldown modifier
    hackRate: float = 1.0          # Objective hacking speed

    # === MELEE CONFIGURATION ===
    meleeknockback: float = 1.0
    meleeMoves: str = ""           # Available melee combos
    disableGunBash: int = 0        # Disable gun bash attack

    # === HEALTH REGENERATION ===
    healthRegenAmount: int = 0
    healthRegenTime: int = 0       # Interval in ms
    healthRegenDelay: int = 0      # Delay before regen starts
    healthRegenMax: int = 0        # Max health to regen to

    # === ARMOR REGENERATION ===
    armourRegenAmount: int = 0
    armourRegenTime: int = 0
    armourRegenDelay: int = 0
    armourRegenMax: int = 0

    # === RESOURCE REGENERATION ===
    resourceRegenAmount: int = 0
    resourceRegenTime: int = 0
    resourceRegenDelay: int = 0
    resourceRegenMax: int = 0

    # === BLOCK POINT REGENERATION ===
    blockRegenAmount: int = 0
    blockRegenTime: int = 0
    blockRegenDelay: int = 0
    blockRegenMax: int = 0

    # === RANK PROGRESSION (comma-separated values per rank) ===
    rankHealth: str = ""
    rankArmor: str = ""
    rankAP: str = ""
    rankBP: str = ""
    rankCS: str = ""
    rankAS: str = ""
    rankFP: str = ""
    rankForcePool: str = ""
    rankSpeed: str = ""
    rankSaberDamage: str = ""
    rankDamageTaken: str = ""
    rankDamageGiven: str = ""
    rankKnockbackTaken: str = ""
    rankKnockbackGiven: str = ""
    rankMeleeKnockback: str = ""
    rankHealthRegenAmount: str = ""
    rankHealthRegenTime: str = ""
    rankHealthRegenDelay: str = ""
    rankHealthRegenMax: str = ""
    rankArmourRegenAmount: str = ""
    rankArmourRegenTime: str = ""
    rankArmourRegenDelay: str = ""
    rankArmourRegenMax: str = ""
    rankResourceRegenAmount: str = ""
    rankResourceRegenTime: str = ""
    rankResourceRegenDelay: str = ""
    rankResourceRegenMax: str = ""
    rankBlockRegenAmount: str = ""
    rankBlockRegenTime: str = ""
    rankBlockRegenDelay: str = ""
    rankBlockRegenMax: str = ""

    # === ANIMATION OVERRIDES ===
    runForward: str = ""
    runBackward: str = ""
    runLeft: str = ""
    runRight: str = ""
    walkForward: str = ""
    walkBackward: str = ""
    walkLeft: str = ""
    walkRight: str = ""
    deathAnim: str = ""
    stunAnim: str = ""
    tauntAnim: str = ""
    flourishAnim: str = ""
    gloatAnim: str = ""
    idleAnim: str = ""
    bowAnim: str = ""
    meditateAnim: str = ""
    saberStanceAnim: str = ""

    # === SOUND OVERRIDES ===
    bargeSoundOverride: str = ""
    rageSoundOverride: str = ""

    # === JETPACK CUSTOMIZATION ===
    jetpackThrustEffect: str = ""
    jetpackIdleEffect: str = ""
    jetpackJetOffset: str = ""
    jetpackJet2Offset: str = ""
    jetpackNoJet2: int = 0
    jetpackFuel: int = 100
    jetpackFuelRegen: float = 1.0

    # === SPECIAL ABILITIES (EAS_* ability slots) ===
    special1: str = ""
    special2: str = ""
    special3: str = ""
    special4: str = ""
    resource: str = ""             # Resource type for abilities

    # === CLASS LIMITS ===
    classNumberLimit: int = -1
    respawnWait: int = 0           # Respawn delay
    respawnCustomTime: int = 0
    extralives: int = 0

    # === MISC ===
    humanoidSkeleton: str = ""     # Custom skeleton
    customveh: str = ""            # Custom vehicle
    headSwapModel: str = ""        # Head swap model
    headSwapSkin: str = ""         # Head swap skin

    # === CUSTOM BUILD SYSTEM ===
    isCustomBuild: int = 0
    mbPoints: int = 0              # Available build points

    # Custom build skill slots (0-14)
    c_att_skills: Dict[int, str] = field(default_factory=dict)
    c_att_names: Dict[int, str] = field(default_factory=dict)
    c_att_ranks: Dict[int, str] = field(default_factory=dict)
    c_att_descs: Dict[int, str] = field(default_factory=dict)

    # === CUSTOM SPEC SYSTEM ===
    hasCustomSpec: int = 0
    isOnlyOneSpec: int = 0
    defaultSpec: int = 0

    # Custom spec tabs (1-3)
    customSpecName_1: str = ""
    customSpecIcon_1: str = ""
    customSpecDesc_1: str = ""
    customSpecName_2: str = ""
    customSpecIcon_2: str = ""
    customSpecDesc_2: str = ""
    customSpecName_3: str = ""
    customSpecIcon_3: str = ""
    customSpecDesc_3: str = ""

    # === PER-WEAPON FLAGS (WP_*Flags) ===
    # These override class flags for specific weapons
    weapon_flags: Dict[str, str] = field(default_factory=dict)

    # === MODEL VARIANTS (model_1 through model_20) ===
    model_variants: Dict[int, Dict[str, str]] = field(default_factory=dict)

    # === WEAPON/FORCE OVERRIDE BLOCKS ===
    weapon_infos: List[WeaponInfo] = field(default_factory=list)
    force_infos: List[ForceInfo] = field(default_factory=list)

    # === DESCRIPTION ===
    description: str = ""

    # === RAW STORAGE FOR UNKNOWN FIELDS ===
    extra_fields: Dict[str, Any] = field(default_factory=dict)


class MBCHParser:
    """Parser for MBII .mbch character class files"""

    # Fields that are floats
    FLOAT_FIELDS = {
        # Model/appearance
        'modelscale',
        # Stats
        'forceregen', 'speed', 'baseSpeed',
        # Multipliers
        'APmultiplier', 'BPmultiplier', 'CSmultiplier', 'ASmultiplier', 'FPmultiplier',
        # Damage modifiers
        'damageTaken', 'damageGiven', 'rateOfFire', 'rateOfFire_Melee',
        'knockbackTaken', 'knockbackGiven', 'skillTimerMod', 'hackRate',
        # Melee
        'meleeknockback',
        # Saber
        'saberDamage',
        # Jetpack
        'jetpackFuelRegen',
        # WeaponInfo floats
        'ReloadTimeModifier', 'damageMod', 'velocityMod', 'rateMod',
        'FPBlockMinMult', 'FPBlockMaxMult', 'FPNoBlockMinMult', 'FPNoBlockMaxMult',
        'FPChargeMult', 'FPMult', 'MissileLight'
    }

    # Fields that are integers
    INT_FIELDS = {
        # Basic info
        'siegeTeam',
        # RGB colors
        'customred', 'customgreen', 'customblue', 'userRGB',
        # Stats
        'maxhealth', 'maxarmor', 'starthealth', 'startarmor', 'forcepool',
        # Saber
        'sabercolor', 'saber2color', 'saberStance',
        # Melee
        'disableGunBash',
        # Regen - health
        'healthRegenAmount', 'healthRegenTime', 'healthRegenDelay', 'healthRegenMax',
        # Regen - armor
        'armourRegenAmount', 'armourRegenTime', 'armourRegenDelay', 'armourRegenMax',
        # Regen - resource
        'resourceRegenAmount', 'resourceRegenTime', 'resourceRegenDelay', 'resourceRegenMax',
        # Regen - block
        'blockRegenAmount', 'blockRegenTime', 'blockRegenDelay', 'blockRegenMax',
        # Class limits
        'classNumberLimit', 'respawnWait', 'respawnCustomTime', 'extralives',
        # Jetpack
        'jetpackNoJet2', 'jetpackFuel',
        # Custom build
        'isCustomBuild', 'mbPoints',
        # Custom spec
        'hasCustomSpec', 'isOnlyOneSpec', 'defaultSpec',
        # WeaponInfo ints
        'altFireEnabled', 'primFireEnabled', 'clipSize', 'isMinigun', 'hasAnimOverrides',
        # ForceInfo ints
        'HandShaderRed', 'HandShaderGreen', 'HandShaderBlue',
        'HandShader2Red', 'HandShader2Green', 'HandShader2Blue'
    }

    def __init__(self, schema_path: Optional[str] = None):
        """Initialize parser with optional schema for validation"""
        self.schema = None
        if schema_path:
            self.load_schema(schema_path)

    def load_schema(self, schema_path: str) -> None:
        """Load JSON schema for validation"""
        with open(schema_path, 'r') as f:
            self.schema = json.load(f)

    def parse_file(self, filepath: str) -> MBCHCharacter:
        """Parse an .mbch file and return MBCHCharacter object"""
        with open(filepath, 'r', encoding='utf-8', errors='replace') as f:
            content = f.read()
        return self.parse_content(content)

    def parse_content(self, content: str) -> MBCHCharacter:
        """Parse .mbch content string and return MBCHCharacter object"""
        char = MBCHCharacter()

        # Remove single-line comments
        lines = []
        for line in content.split('\n'):
            # Handle // comments
            comment_pos = line.find('//')
            if comment_pos >= 0:
                line = line[:comment_pos]
            lines.append(line)
        content = '\n'.join(lines)

        # Parse ClassInfo block
        class_info_match = re.search(r'ClassInfo\s*\{([^}]+)\}', content, re.DOTALL | re.IGNORECASE)
        if class_info_match:
            self._parse_class_info(class_info_match.group(1), char)

        # Parse WeaponInfo blocks (WeaponInfo0, WeaponInfo1, etc.)
        for match in re.finditer(r'WeaponInfo(\d+)\s*\{([^}]+)\}', content, re.DOTALL | re.IGNORECASE):
            index = int(match.group(1))
            weapon_info = self._parse_weapon_info(match.group(2), index)
            char.weapon_infos.append(weapon_info)

        # Parse ForceInfo blocks
        for match in re.finditer(r'ForceInfo(\d+)\s*\{([^}]+)\}', content, re.DOTALL | re.IGNORECASE):
            index = int(match.group(1))
            force_info = self._parse_force_info(match.group(2), index)
            char.force_infos.append(force_info)

        # Parse description (outside of blocks)
        desc_match = re.search(r'[Dd]escription\s+"([^"]*)"', content)
        if desc_match:
            char.description = desc_match.group(1)

        return char

    def _parse_class_info(self, block_content: str, char: MBCHCharacter) -> None:
        """Parse ClassInfo block content into character object"""
        # Parse key-value pairs
        # Format: key "value" or key value

        for line in block_content.split('\n'):
            line = line.strip()
            if not line:
                continue

            # Match: key "quoted value" or key unquoted_value
            quoted_match = re.match(r'(\w+)\s+"([^"]*)"', line)
            unquoted_match = re.match(r'(\w+)\s+([^\s]+)', line)

            if quoted_match:
                key, value = quoted_match.groups()
            elif unquoted_match:
                key, value = unquoted_match.groups()
            else:
                continue

            # Set value on character object
            self._set_char_field(char, key, value)

    def _set_char_field(self, char: MBCHCharacter, key: str, value: str) -> None:
        """Set a field on the character object, handling type conversion"""

        # Handle custom build skill fields (c_att_skill_0 through c_att_skill_14)
        skill_match = re.match(r'c_att_skill_(\d+)', key)
        if skill_match:
            char.c_att_skills[int(skill_match.group(1))] = value
            return

        name_match = re.match(r'c_att_names_(\d+)', key)
        if name_match:
            char.c_att_names[int(name_match.group(1))] = value
            return

        rank_match = re.match(r'c_att_ranks_(\d+)', key)
        if rank_match:
            char.c_att_ranks[int(rank_match.group(1))] = value
            return

        desc_match = re.match(r'c_att_descs_(\d+)', key)
        if desc_match:
            char.c_att_descs[int(desc_match.group(1))] = value
            return

        # Handle WP_*Flags (per-weapon class flags)
        if key.startswith('WP_') and key.endswith('Flags'):
            char.weapon_flags[key] = value
            return

        # Handle model variants (model_1, skin_1, etc. - up to 20)
        variant_match = re.match(
            r'(model|skin|uishader|saber1|saber2|sabercolor|saber2color|soundset|customred|customgreen|customblue|saberStance)_(\d+)',
            key
        )
        if variant_match:
            field_type = variant_match.group(1)
            variant_num = int(variant_match.group(2))
            if variant_num not in char.model_variants:
                char.model_variants[variant_num] = {}
            char.model_variants[variant_num][field_type] = value
            return

        # Handle customSpec fields (customSpecName_1, customSpecIcon_2, etc.)
        spec_match = re.match(r'(customSpecName|customSpecIcon|customSpecDesc)_(\d+)', key)
        if spec_match:
            # These are regular attributes on the class
            full_key = f"{spec_match.group(1)}_{spec_match.group(2)}"
            if hasattr(char, full_key):
                setattr(char, full_key, value)
            return

        # Check if field exists on character
        if hasattr(char, key):
            if key in self.FLOAT_FIELDS:
                try:
                    setattr(char, key, float(value))
                except ValueError:
                    setattr(char, key, value)  # Keep as string if conversion fails
            elif key in self.INT_FIELDS:
                try:
                    setattr(char, key, int(value))
                except ValueError:
                    setattr(char, key, value)  # Keep as string if conversion fails
            else:
                setattr(char, key, value)
        else:
            # Store unknown fields
            char.extra_fields[key] = value

    def _parse_weapon_info(self, block_content: str, index: int) -> WeaponInfo:
        """Parse WeaponInfo block content"""
        info = WeaponInfo(index=index)

        # WeaponInfo float fields
        wi_float_fields = {
            'ReloadTimeModifier', 'damageMod', 'velocityMod', 'rateMod',
            'FPBlockMinMult', 'FPBlockMaxMult', 'FPNoBlockMinMult', 'FPNoBlockMaxMult',
            'FPChargeMult', 'FPMult', 'MissileLight'
        }
        # WeaponInfo int fields
        wi_int_fields = {
            'altFireEnabled', 'primFireEnabled', 'clipSize', 'isMinigun', 'hasAnimOverrides'
        }

        for line in block_content.split('\n'):
            line = line.strip()
            if not line:
                continue

            quoted_match = re.match(r'(\w+)\s+"([^"]*)"', line)
            unquoted_match = re.match(r'(\w+)\s+([^\s]+)', line)

            if quoted_match:
                key, value = quoted_match.groups()
            elif unquoted_match:
                key, value = unquoted_match.groups()
            else:
                continue

            if hasattr(info, key):
                if key in wi_float_fields:
                    try:
                        setattr(info, key, float(value))
                    except ValueError:
                        setattr(info, key, value)
                elif key in wi_int_fields:
                    try:
                        setattr(info, key, int(value))
                    except ValueError:
                        setattr(info, key, value)
                else:
                    setattr(info, key, value)

        return info

    def _parse_force_info(self, block_content: str, index: int) -> ForceInfo:
        """Parse ForceInfo block content"""
        info = ForceInfo(index=index)

        # ForceInfo int fields (RGB colors)
        fi_int_fields = {
            'HandShaderRed', 'HandShaderGreen', 'HandShaderBlue',
            'HandShader2Red', 'HandShader2Green', 'HandShader2Blue'
        }

        for line in block_content.split('\n'):
            line = line.strip()
            if not line:
                continue

            quoted_match = re.match(r'(\w+)\s+"([^"]*)"', line)
            unquoted_match = re.match(r'(\w+)\s+([^\s]+)', line)

            if quoted_match:
                key, value = quoted_match.groups()
            elif unquoted_match:
                key, value = unquoted_match.groups()
            else:
                continue

            if hasattr(info, key):
                if key in fi_int_fields:
                    try:
                        setattr(info, key, int(value))
                    except ValueError:
                        setattr(info, key, value)
                else:
                    setattr(info, key, value)

        return info

    def write_file(self, char: MBCHCharacter, filepath: str) -> None:
        """Write MBCHCharacter object to .mbch file"""
        content = self.to_mbch_string(char)
        with open(filepath, 'w', encoding='utf-8') as f:
            f.write(content)

    def to_mbch_string(self, char: MBCHCharacter) -> str:
        """Convert MBCHCharacter object to .mbch format string"""
        lines = []

        # Comment header
        lines.append(f"// {char.name}")
        lines.append("")

        # ClassInfo block
        lines.append("ClassInfo")
        lines.append("{")

        # Basic info
        lines.append(f'\tname\t\t\t"{char.name}"')
        if char.MBClass:
            lines.append(f'\tMBClass\t\t\t{char.MBClass}')

        # Model/appearance
        if char.model:
            lines.append(f'\tmodel\t\t\t"{char.model}"')
        if char.skin:
            lines.append(f'\tskin\t\t\t"{char.skin}"')
        if char.uishader:
            lines.append(f'\tuishader\t\t"{char.uishader}"')
        if char.modelscale != 1.0:
            lines.append(f'\tmodelscale\t\t{char.modelscale}')
        if char.soundset:
            lines.append(f'\tsoundset\t\t"{char.soundset}"')

        # Equipment
        if char.weapons:
            lines.append(f'\tweapons\t\t\t{char.weapons}')
        if char.attributes:
            lines.append(f'\tattributes\t\t{char.attributes}')
        if char.forcepowers:
            lines.append(f'\tforcepowers\t\t{char.forcepowers}')
        if char.saberstyle:
            lines.append(f'\tsaberstyle\t\t{char.saberstyle}')
        if char.classflags:
            lines.append(f'\tclassflags\t\t{char.classflags}')

        # Stats
        lines.append(f'\tmaxhealth\t\t{char.maxhealth}')
        if char.maxarmor > 0:
            lines.append(f'\tmaxarmor\t\t{char.maxarmor}')
        if char.forcepool > 0:
            lines.append(f'\tforcepool\t\t{char.forcepool}')
        if char.forceregen != 1.0:
            lines.append(f'\tforceregen\t\t{char.forceregen}')
        if char.speed != 1.0:
            lines.append(f'\tspeed\t\t\t{char.speed}')

        # Multipliers (only if not default 1.0)
        if char.APmultiplier != 1.0:
            lines.append(f'\tAPmultiplier\t\t{char.APmultiplier}')
        if char.BPmultiplier != 1.0:
            lines.append(f'\tBPmultiplier\t\t{char.BPmultiplier}')
        if char.CSmultiplier != 1.0:
            lines.append(f'\tCSmultiplier\t\t{char.CSmultiplier}')
        if char.ASmultiplier != 1.0:
            lines.append(f'\tASmultiplier\t\t{char.ASmultiplier}')
        if char.FPmultiplier != 1.0:
            lines.append(f'\tFPmultiplier\t\t{char.FPmultiplier}')

        # Damage modifiers
        if char.damageTaken != 1.0:
            lines.append(f'\tdamageTaken\t\t{char.damageTaken}')
        if char.damageGiven != 1.0:
            lines.append(f'\tdamageGiven\t\t{char.damageGiven}')
        if char.rateOfFire != 1.0:
            lines.append(f'\trateOfFire\t\t{char.rateOfFire}')
        if char.knockbackTaken != 1.0:
            lines.append(f'\tknockbackTaken\t\t{char.knockbackTaken}')
        if char.knockbackGiven != 1.0:
            lines.append(f'\tknockbackGiven\t\t{char.knockbackGiven}')

        # Saber config
        if char.saber1:
            lines.append(f'\tsaber1\t\t\t{char.saber1}')
        if char.saber2:
            lines.append(f'\tsaber2\t\t\t{char.saber2}')
        if char.sabercolor:
            lines.append(f'\tsabercolor\t\t{char.sabercolor}')
        if char.saber2color:
            lines.append(f'\tsaber2color\t\t{char.saber2color}')
        if char.saberDamage != 1.0:
            lines.append(f'\tsaberDamage\t\t{char.saberDamage}')

        # Class limits
        if char.classNumberLimit != -1:
            lines.append(f'\tclassNumberLimit\t{char.classNumberLimit}')
        if char.respawnCustomTime > 0:
            lines.append(f'\trespawnCustomTime\t{char.respawnCustomTime}')
        if char.extralives > 0:
            lines.append(f'\textralives\t\t{char.extralives}')

        # Health regen
        if char.healthRegenAmount > 0:
            lines.append(f'\thealthRegenAmount\t{char.healthRegenAmount}')
            lines.append(f'\thealthRegenTime\t\t{char.healthRegenTime}')
            lines.append(f'\thealthRegenDelay\t{char.healthRegenDelay}')
            if char.healthRegenMax > 0:
                lines.append(f'\thealthRegenMax\t\t{char.healthRegenMax}')

        # Armor regen
        if char.armourRegenAmount > 0:
            lines.append(f'\tarmourRegenAmount\t{char.armourRegenAmount}')
            lines.append(f'\tarmourRegenTime\t\t{char.armourRegenTime}')
            lines.append(f'\tarmourRegenDelay\t{char.armourRegenDelay}')
            if char.armourRegenMax > 0:
                lines.append(f'\tarmourRegenMax\t\t{char.armourRegenMax}')

        # Rank stats
        if char.rankHealth:
            lines.append(f'\trankHealth\t\t{char.rankHealth}')
        if char.rankArmor:
            lines.append(f'\trankArmor\t\t{char.rankArmor}')
        if char.rankAP:
            lines.append(f'\trankAP\t\t\t{char.rankAP}')
        if char.rankBP:
            lines.append(f'\trankBP\t\t\t{char.rankBP}')

        # Special abilities
        if char.special1:
            lines.append(f'\tspecial1\t\t{char.special1}')
        if char.special2:
            lines.append(f'\tspecial2\t\t{char.special2}')
        if char.special3:
            lines.append(f'\tspecial3\t\t{char.special3}')
        if char.special4:
            lines.append(f'\tspecial4\t\t{char.special4}')

        # Custom build system
        if char.isCustomBuild:
            lines.append(f'\tisCustomBuild\t\t{char.isCustomBuild}')
            lines.append(f'\tmbPoints\t\t{char.mbPoints}')

            # Custom skill slots
            for i in sorted(char.c_att_skills.keys()):
                lines.append(f'\tc_att_skill_{i}\t\t{char.c_att_skills[i]}')
            for i in sorted(char.c_att_names.keys()):
                lines.append(f'\tc_att_names_{i}\t\t"{char.c_att_names[i]}"')
            for i in sorted(char.c_att_ranks.keys()):
                lines.append(f'\tc_att_ranks_{i}\t\t{char.c_att_ranks[i]}')
            for i in sorted(char.c_att_descs.keys()):
                lines.append(f'\tc_att_descs_{i}\t\t"{char.c_att_descs[i]}"')

        # Model variants
        for variant_num in sorted(char.model_variants.keys()):
            variant = char.model_variants[variant_num]
            if 'model' in variant:
                lines.append(f'\tmodel_{variant_num}\t\t"{variant["model"]}"')
            if 'skin' in variant:
                lines.append(f'\tskin_{variant_num}\t\t"{variant["skin"]}"')
            if 'uishader' in variant:
                lines.append(f'\tuishader_{variant_num}\t\t"{variant["uishader"]}"')
            if 'saber1' in variant:
                lines.append(f'\tsaber1_{variant_num}\t\t{variant["saber1"]}')
            if 'saber2' in variant:
                lines.append(f'\tsaber2_{variant_num}\t\t{variant["saber2"]}')
            if 'sabercolor' in variant:
                lines.append(f'\tsabercolor_{variant_num}\t\t{variant["sabercolor"]}')

        # Extra unknown fields
        for key, value in char.extra_fields.items():
            if isinstance(value, str) and ' ' in value:
                lines.append(f'\t{key}\t\t\t"{value}"')
            else:
                lines.append(f'\t{key}\t\t\t{value}')

        lines.append("}")
        lines.append("")

        # WeaponInfo blocks
        for weapon_info in char.weapon_infos:
            lines.append(f"WeaponInfo{weapon_info.index}")
            lines.append("{")
            if weapon_info.weapon:
                lines.append(f'\tweapon\t\t\t{weapon_info.weapon}')
            if weapon_info.WeaponModel:
                lines.append(f'\tWeaponModel\t\t"{weapon_info.WeaponModel}"')
            if weapon_info.MuzzleEffect:
                lines.append(f'\tMuzzleEffect\t\t"{weapon_info.MuzzleEffect}"')
            if weapon_info.ProjectileEffect:
                lines.append(f'\tProjectileEffect\t"{weapon_info.ProjectileEffect}"')
            if weapon_info.FlashSound:
                lines.append(f'\tFlashSound\t\t"{weapon_info.FlashSound}"')
            # Add other fields as needed
            lines.append("}")
            lines.append("")

        # ForceInfo blocks
        for force_info in char.force_infos:
            lines.append(f"ForceInfo{force_info.index}")
            lines.append("{")
            if force_info.forcepower:
                lines.append(f'\tforcepower\t\t{force_info.forcepower}')
            if force_info.CastEffect:
                lines.append(f'\tCastEffect\t\t"{force_info.CastEffect}"')
            if force_info.HitEffect:
                lines.append(f'\tHitEffect\t\t"{force_info.HitEffect}"')
            if force_info.CastSound:
                lines.append(f'\tCastSound\t\t"{force_info.CastSound}"')
            lines.append("}")
            lines.append("")

        # Description
        if char.description:
            lines.append(f'description "{char.description}"')

        return '\n'.join(lines)

    def validate(self, char: MBCHCharacter) -> List[str]:
        """Validate character against schema, return list of errors"""
        errors = []

        # Required fields
        if not char.name:
            errors.append("Missing required field: name")
        if not char.MBClass:
            errors.append("Missing required field: MBClass")

        # Validate MBClass enum — shape-check only. We accept any
        # MB_CLASS_* prefix so files authored against extended builds
        # still parse without this module needing a curated list.
        if char.MBClass and not char.MBClass.startswith("MB_CLASS_"):
            errors.append(f"Invalid MBClass: {char.MBClass}")

        # Validate weapon format
        if char.weapons:
            for weapon in char.weapons.split('|'):
                if not weapon.startswith('WP_'):
                    errors.append(f"Invalid weapon format: {weapon}")

        # Validate numeric ranges
        if char.maxhealth < 1 or char.maxhealth > 9999:
            errors.append(f"maxhealth out of range (1-9999): {char.maxhealth}")
        if char.maxarmor < 0 or char.maxarmor > 9999:
            errors.append(f"maxarmor out of range (0-9999): {char.maxarmor}")
        if char.sabercolor < 0 or char.sabercolor > 8:
            errors.append(f"sabercolor out of range (0-8): {char.sabercolor}")

        return errors

    def to_dict(self, char: MBCHCharacter) -> Dict[str, Any]:
        """Convert MBCHCharacter to dictionary for JSON export"""
        return {
            'ClassInfo': {
                'name': char.name,
                'MBClass': char.MBClass,
                'model': char.model,
                'skin': char.skin,
                'uishader': char.uishader,
                'modelscale': char.modelscale,
                'weapons': char.weapons,
                'attributes': char.attributes,
                'forcepowers': char.forcepowers,
                'saberstyle': char.saberstyle,
                'classflags': char.classflags,
                'maxhealth': char.maxhealth,
                'maxarmor': char.maxarmor,
                'forcepool': char.forcepool,
                'forceregen': char.forceregen,
                'speed': char.speed,
                'APmultiplier': char.APmultiplier,
                'BPmultiplier': char.BPmultiplier,
                'CSmultiplier': char.CSmultiplier,
                'ASmultiplier': char.ASmultiplier,
                'saber1': char.saber1,
                'saber2': char.saber2,
                'sabercolor': char.sabercolor,
                'classNumberLimit': char.classNumberLimit,
                'respawnCustomTime': char.respawnCustomTime,
                'isCustomBuild': char.isCustomBuild,
                'mbPoints': char.mbPoints,
                'extra_fields': char.extra_fields
            },
            'WeaponInfo': [
                {
                    'index': wi.index,
                    'weapon': wi.weapon,
                    'WeaponModel': wi.WeaponModel,
                    'MuzzleEffect': wi.MuzzleEffect,
                    'ProjectileEffect': wi.ProjectileEffect
                }
                for wi in char.weapon_infos
            ],
            'ForceInfo': [
                {
                    'index': fi.index,
                    'forcepower': fi.forcepower,
                    'CastEffect': fi.CastEffect,
                    'HitEffect': fi.HitEffect
                }
                for fi in char.force_infos
            ],
            'description': char.description
        }

    def from_dict(self, data: Dict[str, Any]) -> MBCHCharacter:
        """Create MBCHCharacter from dictionary"""
        char = MBCHCharacter()

        if 'ClassInfo' in data:
            ci = data['ClassInfo']
            for key, value in ci.items():
                if key == 'extra_fields':
                    char.extra_fields = value
                elif hasattr(char, key):
                    setattr(char, key, value)

        if 'WeaponInfo' in data:
            for wi_data in data['WeaponInfo']:
                wi = WeaponInfo()
                for key, value in wi_data.items():
                    if hasattr(wi, key):
                        setattr(wi, key, value)
                char.weapon_infos.append(wi)

        if 'ForceInfo' in data:
            for fi_data in data['ForceInfo']:
                fi = ForceInfo()
                for key, value in fi_data.items():
                    if hasattr(fi, key):
                        setattr(fi, key, value)
                char.force_infos.append(fi)

        if 'description' in data:
            char.description = data['description']

        return char


def main():
    """CLI interface for MBCH parser"""
    import sys

    if len(sys.argv) < 2:
        print("Usage: mbch_parser.py <file.mbch> [--json] [--validate]")
        sys.exit(1)

    filepath = sys.argv[1]
    parser = MBCHParser()

    try:
        char = parser.parse_file(filepath)

        if '--validate' in sys.argv:
            errors = parser.validate(char)
            if errors:
                print("Validation errors:")
                for err in errors:
                    print(f"  - {err}")
                sys.exit(1)
            else:
                print("Validation passed!")

        if '--json' in sys.argv:
            import json
            print(json.dumps(parser.to_dict(char), indent=2))
        else:
            print(f"Name: {char.name}")
            print(f"Class: {char.MBClass}")
            print(f"Model: {char.model}")
            print(f"Health: {char.maxhealth}")
            print(f"Armor: {char.maxarmor}")
            print(f"Weapons: {char.weapons}")
            print(f"Custom Build: {bool(char.isCustomBuild)}")
            if char.weapon_infos:
                print(f"Weapon Overrides: {len(char.weapon_infos)}")
            if char.description:
                print(f"Description: {char.description[:50]}...")

    except Exception as e:
        print(f"Error parsing file: {e}")
        sys.exit(1)


if __name__ == "__main__":
    main()
