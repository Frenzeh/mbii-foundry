"""
SAB (Saber) File Parser

Parses and generates MBII .sab saber configuration files.
Based on bg_saberLoad.c parsing logic.
"""

import re
from dataclasses import dataclass, field
from typing import List, Dict, Optional, Tuple
from enum import Enum


class SaberType(Enum):
    """Saber types from saberType_t"""
    SABER_NONE = 0
    SABER_SINGLE = 1
    SABER_STAFF = 2
    SABER_DAGGER = 3
    SABER_BROAD = 4
    SABER_PRONG = 5
    SABER_ARC = 6
    SABER_SAI = 7
    SABER_CLAW = 8
    SABER_LANCE = 9
    SABER_STAR = 10
    SABER_TRIDENT = 11
    SABER_SITH_SWORD = 12


class SaberColor(Enum):
    """Saber colors from saber_colors_t"""
    RED = 0
    ORANGE = 1
    YELLOW = 2
    GREEN = 3
    BLUE = 4
    PURPLE = 5
    SILVER = 6  # Also called "white" in some contexts
    RANDOM = 7


# Saber style enum values
SABER_STYLES = {
    "SS_NONE": 0,
    "SS_FAST": 1,
    "SS_MEDIUM": 2,
    "SS_STRONG": 3,
    "SS_DESANN": 4,
    "SS_TAVION": 5,
    "SS_DUAL": 6,
    "SS_STAFF": 7,
}

# Saber flags (SFL_*)
SABER_FLAGS = {
    "forceBlocking": 1 << 0,      # SFL_FORCE_BLOCK - resist force while blocking
    "notThrowable": 1 << 1,       # SFL_NOT_THROWABLE
    "notDisarmable": 1 << 2,      # SFL_NOT_DISARMABLE
    "notActiveBlocking": 1 << 3,  # SFL_NOT_ACTIVE_BLOCKING
    "twoHanded": 1 << 4,          # SFL_TWO_HANDED
    "singleBladeThrowable": 1 << 5,  # SFL_SINGLE_BLADE_THROWABLE
    "returnDamage": 1 << 6,       # SFL_RETURN_DAMAGE
    "onInWater": 1 << 7,          # SFL_ON_IN_WATER
    "bounceOnWalls": 1 << 8,      # SFL_BOUNCE_ON_WALLS
    "boltToWrist": 1 << 9,        # SFL_BOLT_TO_WRIST
    "noPullAttack": 1 << 10,      # SFL_NO_PULL_ATTACK
    "noBackAttack": 1 << 11,      # SFL_NO_BACK_ATTACK
    "noStabDown": 1 << 12,        # SFL_NO_STABDOWN
    "noWallRuns": 1 << 13,        # SFL_NO_WALL_RUNS
    "noWallFlips": 1 << 14,       # SFL_NO_WALL_FLIPS
    "noWallGrab": 1 << 15,        # SFL_NO_WALL_GRAB
    "noRolls": 1 << 16,           # SFL_NO_ROLLS
    "noFlips": 1 << 17,           # SFL_NO_FLIPS
    "noCartwheels": 1 << 18,      # SFL_NO_CARTWHEELS
    "noKicks": 1 << 19,           # SFL_NO_KICKS
    "noMirrorAttacks": 1 << 20,   # SFL_NO_MIRROR_ATTACKS
    "noRollStab": 1 << 21,        # SFL_NO_ROLL_STAB
    "unstable": 1 << 22,          # SFL_IS_UNSTABLE - Kylo Ren blade
    "noBlasterBlock": 1 << 23,    # SFL_NO_BLASTER_BLOCK
    "darksaber": 1 << 24,         # SFL_IS_DARKSABER
}


@dataclass
class BladeInfo:
    """Information for a single blade"""
    color: str = "blue"
    length: float = 32.0
    radius: float = 3.0


@dataclass
class SaberInfo:
    """Complete saber configuration"""
    # Identity
    name: str = ""
    fullName: str = "lightsaber"

    # Type and model
    saberType: str = "SABER_SINGLE"
    saberModel: str = "models/weapons2/saber_reborn/saber_w.glm"
    customSkin: str = ""
    numBlades: int = 1

    # Blade properties (up to 8 blades)
    blades: List[BladeInfo] = field(default_factory=lambda: [BladeInfo()])
    bladeStyle2Start: int = 0  # First blade to use secondary style

    # Sounds
    soundOn: str = ""
    soundOff: str = ""
    soundLoop: str = ""
    spinSound: str = ""
    swingSounds: List[str] = field(default_factory=list)  # 1-3 swing sounds

    # Primary blade sounds
    hitSounds: List[str] = field(default_factory=list)
    blockSounds: List[str] = field(default_factory=list)
    bounceSounds: List[str] = field(default_factory=list)

    # Secondary blade sounds
    hit2Sounds: List[str] = field(default_factory=list)
    block2Sounds: List[str] = field(default_factory=list)
    bounce2Sounds: List[str] = field(default_factory=list)

    # Combat style
    saberStyle: str = ""
    saberStyleLearned: int = 0
    saberStyleForbidden: int = 0
    singleBladeStyle: str = "SS_NONE"
    maxChain: int = 0
    forceRestrict: int = 0
    lockBonus: int = 0
    parryBonus: int = 0
    breakParryBonus: int = 0
    breakParryBonus2: int = 0
    disarmBonus: int = 0
    disarmBonus2: int = 0

    # Speed and damage
    moveSpeedScale: float = 1.0
    animSpeedScale: float = 1.0
    damageScale: float = 1.0
    damageScale2: float = 1.0
    knockbackScale: float = 0.0
    knockbackScale2: float = 0.0

    # Splash damage (primary)
    splashRadius: float = 0.0
    splashDamage: int = 0
    splashKnockback: float = 0.0

    # Splash damage (secondary)
    splashRadius2: float = 0.0
    splashDamage2: int = 0
    splashKnockback2: float = 0.0

    # Visual effects (primary)
    trailStyle: int = 0
    g2MarksShader: str = ""
    g2WeaponMarkShader: str = ""
    blockEffect: str = ""
    hitPersonEffect: str = ""
    hitOtherEffect: str = ""
    bladeEffect: str = ""

    # Visual effects (secondary)
    trailStyle2: int = 0
    g2MarksShader2: str = ""
    g2WeaponMarkShader2: str = ""
    blockEffect2: str = ""
    hitPersonEffect2: str = ""
    hitOtherEffect2: str = ""
    bladeEffect2: str = ""

    # Animations
    readyAnim: int = -1
    drawAnim: int = -1
    putawayAnim: int = -1
    tauntAnim: int = -1
    bowAnim: int = -1
    meditateAnim: int = -1
    flourishAnim: int = -1
    gloatAnim: int = -1
    slapAnim: int = -1
    readyAnimOnlyTorso: bool = False

    # Special moves
    kataMove: str = ""
    lungeAtkMove: str = ""
    jumpAtkUpMove: str = ""
    jumpAtkFwdMove: str = ""
    jumpAtkBackMove: str = ""
    jumpAtkRightMove: str = ""
    jumpAtkLeftMove: str = ""

    # Behavior flags (primary blade)
    noWallMarks: bool = False
    noDlight: bool = False
    noBlade: bool = False
    noClashFlare: bool = False
    noDismemberment: bool = False
    noIdleEffect: bool = False
    alwaysBlock: bool = False
    noManualDeactivate: bool = False
    transitionDamage: bool = False
    disabledBladeIsHot: bool = False
    noBladeCortosisReaction: bool = False

    # Behavior flags (secondary blade)
    noWallMarks2: bool = False
    noDlight2: bool = False
    noBlade2: bool = False
    noClashFlare2: bool = False
    noDismemberment2: bool = False
    noIdleEffect2: bool = False
    alwaysBlock2: bool = False
    noManualDeactivate2: bool = False
    transitionDamage2: bool = False
    disabledBladeIsHot2: bool = False
    noBladeCortosisReaction2: bool = False

    # Saber-level flags
    saberFlags: Dict[str, bool] = field(default_factory=dict)

    # Misc
    notInMP: bool = False
    isOpenMode: bool = False

    # Extra fields not explicitly defined
    extraFields: Dict[str, str] = field(default_factory=dict)


class SABParser:
    """Parser for .sab saber files"""

    def __init__(self):
        self.errors: List[str] = []
        self.warnings: List[str] = []

    def parse(self, content: str) -> Tuple[SaberInfo, List[str]]:
        """
        Parse .sab file content into SaberInfo.

        Args:
            content: The raw file content

        Returns:
            Tuple of (SaberInfo, list of errors)
        """
        self.errors = []
        self.warnings = []
        saber = SaberInfo()

        # Remove comments
        lines = []
        for line in content.split('\n'):
            comment_idx = line.find('//')
            if comment_idx >= 0:
                line = line[:comment_idx]
            lines.append(line)
        content = '\n'.join(lines)

        # Find saber name and block
        # Format: sabername { ... }
        match = re.search(r'(\w+)\s*\{([^}]+)\}', content, re.DOTALL)
        if not match:
            self.errors.append("Could not find saber definition block")
            return saber, self.errors

        saber.name = match.group(1)
        block = match.group(2)

        # Parse key-value pairs
        self._parse_block(block, saber)

        return saber, self.errors

    def _parse_block(self, block: str, saber: SaberInfo):
        """Parse the saber definition block"""
        # Match quoted and unquoted values
        quoted_re = re.compile(r'(\w+)\s+"([^"]*)"')
        unquoted_re = re.compile(r'(\w+)\s+([^\s]+)')

        for line in block.split('\n'):
            line = line.strip()
            if not line:
                continue

            key = None
            value = None

            # Try quoted first
            m = quoted_re.match(line)
            if m:
                key, value = m.group(1), m.group(2)
            else:
                m = unquoted_re.match(line)
                if m:
                    key, value = m.group(1), m.group(2)

            if key and value:
                self._set_field(saber, key.lower(), value)

    def _set_field(self, saber: SaberInfo, key: str, value: str):
        """Set a field on the saber"""
        # Name and type
        if key == "name":
            saber.fullName = value
        elif key == "sabertype":
            saber.saberType = value.upper()
        elif key == "sabermodel":
            saber.saberModel = value
        elif key == "customskin":
            saber.customSkin = value
        elif key == "numblades":
            saber.numBlades = int(value)
            # Ensure we have enough blade entries
            while len(saber.blades) < saber.numBlades:
                saber.blades.append(BladeInfo())

        # Blade colors (saberColor, saberColor1-8)
        elif key == "sabercolor":
            for blade in saber.blades:
                blade.color = value
        elif key.startswith("sabercolor") and key[10:].isdigit():
            idx = int(key[10:]) - 1
            if 0 <= idx < len(saber.blades):
                saber.blades[idx].color = value

        # Blade lengths (saberLength, saberLength1-8)
        elif key == "saberlength":
            length = float(value)
            for blade in saber.blades:
                blade.length = length
        elif key.startswith("saberlength") and key[11:].isdigit():
            idx = int(key[11:]) - 1
            if 0 <= idx < len(saber.blades):
                saber.blades[idx].length = float(value)

        # Blade radius (saberRadius, saberRadius1-8)
        elif key == "saberradius":
            radius = float(value)
            for blade in saber.blades:
                blade.radius = radius
        elif key.startswith("saberradius") and key[11:].isdigit():
            idx = int(key[11:]) - 1
            if 0 <= idx < len(saber.blades):
                saber.blades[idx].radius = float(value)

        # Blade style division
        elif key == "bladestyle2start":
            saber.bladeStyle2Start = int(value)

        # Sounds
        elif key == "soundon":
            saber.soundOn = value
        elif key == "soundoff":
            saber.soundOff = value
        elif key == "soundloop":
            saber.soundLoop = value
        elif key == "spinsound":
            saber.spinSound = value
        elif key.startswith("swingsound"):
            saber.swingSounds.append(value)
        elif key.startswith("hitsound") and not key.startswith("hit2sound"):
            saber.hitSounds.append(value)
        elif key.startswith("blocksound") and not key.startswith("block2sound"):
            saber.blockSounds.append(value)
        elif key.startswith("bouncesound") and not key.startswith("bounce2sound"):
            saber.bounceSounds.append(value)
        elif key.startswith("hit2sound"):
            saber.hit2Sounds.append(value)
        elif key.startswith("block2sound"):
            saber.block2Sounds.append(value)
        elif key.startswith("bounce2sound"):
            saber.bounce2Sounds.append(value)

        # Combat style
        elif key == "saberstyle":
            saber.saberStyle = value
        elif key == "singlebladestyle":
            saber.singleBladeStyle = value
        elif key == "maxchain":
            saber.maxChain = int(value)
        elif key == "lockbonus":
            saber.lockBonus = int(value)
        elif key == "parrybonus":
            saber.parryBonus = int(value)
        elif key == "breakparrybonus":
            saber.breakParryBonus = int(value)
        elif key == "breakparrybonus2":
            saber.breakParryBonus2 = int(value)
        elif key == "disarmbonus":
            saber.disarmBonus = int(value)
        elif key == "disarmbonus2":
            saber.disarmBonus2 = int(value)

        # Speed and damage
        elif key == "movespeedscale":
            saber.moveSpeedScale = float(value)
        elif key == "animspeedscale":
            saber.animSpeedScale = float(value)
        elif key == "damagescale":
            saber.damageScale = float(value)
        elif key == "damagescale2":
            saber.damageScale2 = float(value)
        elif key == "knockbackscale":
            saber.knockbackScale = float(value)
        elif key == "knockbackscale2":
            saber.knockbackScale2 = float(value)

        # Splash damage
        elif key == "splashradius":
            saber.splashRadius = float(value)
        elif key == "splashdamage":
            saber.splashDamage = int(value)
        elif key == "splashknockback":
            saber.splashKnockback = float(value)
        elif key == "splashradius2":
            saber.splashRadius2 = float(value)
        elif key == "splashdamage2":
            saber.splashDamage2 = int(value)
        elif key == "splashknockback2":
            saber.splashKnockback2 = float(value)

        # Visual effects
        elif key == "trailstyle":
            saber.trailStyle = int(value)
        elif key == "trailstyle2":
            saber.trailStyle2 = int(value)
        elif key == "g2marksshader":
            saber.g2MarksShader = value
        elif key == "g2marksshader2":
            saber.g2MarksShader2 = value
        elif key == "blockeffect":
            saber.blockEffect = value
        elif key == "blockeffect2":
            saber.blockEffect2 = value
        elif key == "hitpersoneffect":
            saber.hitPersonEffect = value
        elif key == "hitpersoneffect2":
            saber.hitPersonEffect2 = value
        elif key == "hitothereffect":
            saber.hitOtherEffect = value
        elif key == "bladeeffect":
            saber.bladeEffect = value
        elif key == "bladeeffect2":
            saber.bladeEffect2 = value

        # Behavior flags
        elif key == "nowallmarks":
            saber.noWallMarks = value == "1"
        elif key == "nodlight":
            saber.noDlight = value == "1"
        elif key == "noblade":
            saber.noBlade = value == "1"
        elif key == "noclashflare":
            saber.noClashFlare = value == "1"
        elif key == "nodismemberment":
            saber.noDismemberment = value == "1"
        elif key == "noidleeffect":
            saber.noIdleEffect = value == "1"
        elif key == "alwaysblock":
            saber.alwaysBlock = value == "1"
        elif key == "nomanualdeactivate":
            saber.noManualDeactivate = value == "1"
        elif key == "transitiondamage":
            saber.transitionDamage = value == "1"

        # Saber-level flags
        elif key in SABER_FLAGS:
            saber.saberFlags[key] = value == "1"

        # Special moves
        elif key == "katamove":
            saber.kataMove = value
        elif key == "lungeatkMove":
            saber.lungeAtkMove = value
        elif key.endswith("move") and "jump" in key:
            setattr(saber, key, value)

        # Misc
        elif key == "notinmp":
            saber.notInMP = value == "1"
        elif key == "isopenmode":
            saber.isOpenMode = value == "1"

        else:
            # Store unknown fields
            saber.extraFields[key] = value

    def generate(self, saber: SaberInfo) -> str:
        """
        Generate .sab file content from SaberInfo.

        Args:
            saber: The saber configuration

        Returns:
            The generated file content
        """
        lines = [f"// {saber.fullName}", "", f"{saber.name}", "{"]

        # Core properties
        lines.append(f'\tname\t\t\t"{saber.fullName}"')
        lines.append(f'\tsaberType\t\t{saber.saberType}')

        if saber.saberModel:
            lines.append(f'\tsaberModel\t\t"{saber.saberModel}"')
        if saber.customSkin:
            lines.append(f'\tcustomSkin\t\t"{saber.customSkin}"')
        if saber.numBlades > 1:
            lines.append(f'\tnumBlades\t\t{saber.numBlades}')

        # Blade properties
        if saber.blades:
            # Check if all blades have same color
            first_color = saber.blades[0].color
            all_same_color = all(b.color == first_color for b in saber.blades)

            if all_same_color:
                lines.append(f'\tsaberColor\t\t{first_color}')
            else:
                for i, blade in enumerate(saber.blades):
                    lines.append(f'\tsaberColor{i+1}\t\t{blade.color}')

            # Lengths
            first_length = saber.blades[0].length
            all_same_length = all(b.length == first_length for b in saber.blades)

            if all_same_length and first_length != 32.0:
                lines.append(f'\tsaberLength\t\t{first_length}')
            elif not all_same_length:
                for i, blade in enumerate(saber.blades):
                    if blade.length != 32.0:
                        lines.append(f'\tsaberLength{i+1}\t\t{blade.length}')

            # Radius
            first_radius = saber.blades[0].radius
            all_same_radius = all(b.radius == first_radius for b in saber.blades)

            if all_same_radius and first_radius != 3.0:
                lines.append(f'\tsaberRadius\t\t{first_radius}')
            elif not all_same_radius:
                for i, blade in enumerate(saber.blades):
                    if blade.radius != 3.0:
                        lines.append(f'\tsaberRadius{i+1}\t\t{blade.radius}')

        # Style
        if saber.saberStyle:
            lines.append(f'\tsaberStyle\t\t{saber.saberStyle}')

        # Speed/damage modifiers
        if saber.moveSpeedScale != 1.0:
            lines.append(f'\tmoveSpeedScale\t\t{saber.moveSpeedScale}')
        if saber.animSpeedScale != 1.0:
            lines.append(f'\tanimSpeedScale\t\t{saber.animSpeedScale}')
        if saber.damageScale != 1.0:
            lines.append(f'\tdamageScale\t\t{saber.damageScale}')
        if saber.knockbackScale != 0.0:
            lines.append(f'\tknockbackScale\t\t{saber.knockbackScale}')

        # Combat bonuses
        if saber.lockBonus != 0:
            lines.append(f'\tlockBonus\t\t{saber.lockBonus}')
        if saber.parryBonus != 0:
            lines.append(f'\tparryBonus\t\t{saber.parryBonus}')
        if saber.breakParryBonus != 0:
            lines.append(f'\tbreakParryBonus\t\t{saber.breakParryBonus}')
        if saber.disarmBonus != 0:
            lines.append(f'\tdisarmBonus\t\t{saber.disarmBonus}')

        # Sounds
        if saber.soundOn:
            lines.append(f'\tsoundOn\t\t\t"{saber.soundOn}"')
        if saber.soundOff:
            lines.append(f'\tsoundOff\t\t"{saber.soundOff}"')
        if saber.soundLoop:
            lines.append(f'\tsoundLoop\t\t"{saber.soundLoop}"')

        for i, sound in enumerate(saber.swingSounds[:3]):
            lines.append(f'\tswingSound{i+1}\t\t"{sound}"')
        for i, sound in enumerate(saber.hitSounds[:3]):
            lines.append(f'\thitSound{i+1}\t\t"{sound}"')
        for i, sound in enumerate(saber.blockSounds[:3]):
            lines.append(f'\tblockSound{i+1}\t\t"{sound}"')

        # Visual effects
        if saber.trailStyle != 0:
            lines.append(f'\ttrailStyle\t\t{saber.trailStyle}')
        if saber.blockEffect:
            lines.append(f'\tblockEffect\t\t"{saber.blockEffect}"')
        if saber.hitPersonEffect:
            lines.append(f'\thitPersonEffect\t\t"{saber.hitPersonEffect}"')
        if saber.bladeEffect:
            lines.append(f'\tbladeEffect\t\t"{saber.bladeEffect}"')

        # Behavior flags
        bool_flags = [
            ("noWallMarks", saber.noWallMarks),
            ("noDlight", saber.noDlight),
            ("noBlade", saber.noBlade),
            ("noClashFlare", saber.noClashFlare),
            ("noDismemberment", saber.noDismemberment),
            ("noIdleEffect", saber.noIdleEffect),
            ("alwaysBlock", saber.alwaysBlock),
            ("noManualDeactivate", saber.noManualDeactivate),
            ("transitionDamage", saber.transitionDamage),
        ]

        for name, val in bool_flags:
            if val:
                lines.append(f'\t{name}\t\t\t1')

        # Saber-level flags
        for flag_name, enabled in saber.saberFlags.items():
            if enabled:
                lines.append(f'\t{flag_name}\t\t\t1')

        # Extra fields
        for key, value in saber.extraFields.items():
            if ' ' in value:
                lines.append(f'\t{key}\t\t\t"{value}"')
            else:
                lines.append(f'\t{key}\t\t\t{value}')

        lines.append("}")

        return '\n'.join(lines)

    def validate(self, saber: SaberInfo) -> List[str]:
        """
        Validate a saber configuration.

        Args:
            saber: The saber to validate

        Returns:
            List of validation errors
        """
        errors = []

        if not saber.name:
            errors.append("Saber name is required")

        if not saber.saberModel:
            errors.append("Saber model is required")

        if saber.numBlades < 1 or saber.numBlades > 8:
            errors.append("Number of blades must be between 1 and 8")

        if len(saber.blades) < saber.numBlades:
            errors.append(f"Not enough blade definitions ({len(saber.blades)}) for numBlades ({saber.numBlades})")

        for i, blade in enumerate(saber.blades):
            if blade.length < 4.0:
                errors.append(f"Blade {i+1} length must be at least 4.0")
            if blade.radius <= 0:
                errors.append(f"Blade {i+1} radius must be positive")

        # Validate sounds come in sets of 3
        if saber.swingSounds and len(saber.swingSounds) != 3:
            self.warnings.append("Swing sounds should have exactly 3 variants")
        if saber.hitSounds and len(saber.hitSounds) != 3:
            self.warnings.append("Hit sounds should have exactly 3 variants")
        if saber.blockSounds and len(saber.blockSounds) != 3:
            self.warnings.append("Block sounds should have exactly 3 variants")

        return errors


def parse_sab(content: str) -> Tuple[SaberInfo, List[str]]:
    """Convenience function to parse .sab content"""
    parser = SABParser()
    return parser.parse(content)


def generate_sab(saber: SaberInfo) -> str:
    """Convenience function to generate .sab content"""
    parser = SABParser()
    return parser.generate(saber)


def validate_sab(saber: SaberInfo) -> List[str]:
    """Convenience function to validate a saber"""
    parser = SABParser()
    return parser.validate(saber)
