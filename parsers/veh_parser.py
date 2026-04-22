"""
VEH (Vehicle) File Parser

Parses and generates MBII .veh vehicle configuration files.
Based on bg_vehicleLoad.c parsing logic.
"""

import re
from dataclasses import dataclass, field
from typing import List, Dict, Optional, Tuple
from enum import Enum


class VehicleType(Enum):
    """Vehicle types from vehicleType_t"""
    VH_NONE = 0
    VH_DEKA = 1      # Droidekas - walkers that strafe, have shields
    VH_WALKER = 2    # Vehicles you ride inside (AT-ST)
    VH_FIGHTER = 3   # Vehicles you fly inside (X-Wing, TIE)
    VH_SPEEDER = 4   # Vehicles you ride on (speeder, swoop)
    VH_ANIMAL = 5    # Animals you ride on (tauntaun)
    VH_FLIER = 6     # Flying animals


# Vehicle mode bitfield values
VEHICLE_MODES = {
    "VEHICLE_MODE_RUNNING": 1,
    "VEHICLE_MODE_WALKING": 2,
    "VEHICLE_MODE_BACKWARDS": 4,
    "VEHICLE_MODE_STILL": 8,
    "VEHICLE_MODE_TURNING": 16,
    "VEHICLE_MODE_CROUCHING": 32,
}


@dataclass
class TurretInfo:
    """Configuration for a vehicle turret"""
    weapon: str = ""
    delay: int = 0
    ammoMax: int = 0
    ammoRechargeMS: int = 0
    yawBone: str = ""
    pitchBone: str = ""
    yawAxis: int = 0
    pitchAxis: int = 0
    clampYawL: float = 0.0
    clampYawR: float = 0.0
    clampPitchU: float = 0.0
    clampPitchD: float = 0.0
    muzzle1: int = 0
    muzzle2: int = 0
    turnSpeed: float = 0.0
    ai: bool = False
    aiLead: bool = False
    aiRange: float = 0.0
    passengerNum: int = 0
    gunnerViewTag: str = ""


@dataclass
class VehicleInfo:
    """Complete vehicle configuration"""
    # Identity
    name: str = ""
    vehicleType: str = "VH_SPEEDER"

    # General properties
    numHands: int = 2
    lookPitch: float = 0.0
    lookYaw: float = 0.0
    length: float = 0.0
    width: float = 0.0
    height: float = 0.0
    centerOfGravity: List[float] = field(default_factory=lambda: [0.0, 0.0, 0.0])

    # Speed stats
    speedMax: float = 0.0
    turboSpeed: float = 0.0
    speedMin: float = 0.0
    speedIdle: float = 0.0
    accelIdle: float = 0.0
    acceleration: float = 0.0
    decelIdle: float = 0.0
    throttleSticks: bool = False
    strafePerc: float = 1.0

    # Handling
    bankingSpeed: float = 0.0
    rollLimit: float = 0.0
    pitchLimit: float = 0.0
    braking: float = 0.0
    mouseYaw: float = 0.0
    mousePitch: float = 0.0
    turningSpeed: float = 0.0
    turnWhenStopped: bool = False
    traction: float = 0.0
    friction: float = 0.0
    maxSlope: float = 0.0
    speedDependantTurning: bool = False

    # Durability
    mass: int = 0
    armor: int = 0
    shields: int = 0
    shieldRechargeMS: int = 0
    toughness: float = 1.0
    malfunctionArmorLevel: int = 0
    surfDestruction: bool = False
    health_front: int = 0
    health_back: int = 0
    health_right: int = 0
    health_left: int = 0

    # Model/visuals
    model: str = ""
    skin: str = ""
    g2radius: int = 0
    droidNPC: str = ""

    # UI
    radarIcon: str = ""
    dmgIndicFrame: str = ""
    dmgIndicShield: str = ""
    dmgIndicBackground: str = ""
    icon_front: str = ""
    icon_back: str = ""
    icon_right: str = ""
    icon_left: str = ""
    crosshairShader: str = ""
    shieldShader: str = ""

    # Sounds
    soundOn: str = ""
    soundOff: str = ""
    soundLoop: str = ""
    soundTakeOff: str = ""
    soundEngineStart: str = ""
    soundSpin: str = ""
    soundTurbo: str = ""
    soundHyper: str = ""
    soundLand: str = ""
    soundFlyBy: str = ""
    soundFlyBy2: str = ""
    soundShift1: str = ""
    soundShift2: str = ""
    soundShift3: str = ""
    soundShift4: str = ""

    # Effects
    riderAnim: str = ""
    exhaustFX: str = ""
    turboFX: str = ""
    turboStartFX: str = ""
    trailFX: str = ""
    impactFX: str = ""
    explodeFX: str = ""
    wakeFX: str = ""
    dmgFX: str = ""
    injureFX: str = ""
    noseFX: str = ""
    lwingFX: str = ""
    rwingFX: str = ""
    noFireball: bool = False

    # Weapons
    weap1: str = ""
    weap1Delay: int = 0
    weap1Link: int = 0
    weap1Aim: bool = False
    weap1AmmoMax: int = 0
    weap1AmmoRechargeMS: int = 0
    weap1SoundNoAmmo: str = ""

    weap2: str = ""
    weap2Delay: int = 0
    weap2Link: int = 0
    weap2Aim: bool = False
    weap2AmmoMax: int = 0
    weap2AmmoRechargeMS: int = 0
    weap2SoundNoAmmo: str = ""

    # Muzzles (1-10)
    weapMuzzles: Dict[int, str] = field(default_factory=dict)

    # Turrets
    turret1: TurretInfo = field(default_factory=TurretInfo)
    turret2: TurretInfo = field(default_factory=TurretInfo)

    # Flight/landing
    landingHeight: float = 0.0
    gravity: int = 800
    hoverHeight: float = 0.0
    hoverStrength: float = 0.0
    waterProof: bool = False
    bouyancy: float = 1.0

    # Fuel and turbo
    fuelMax: int = 0
    fuelRate: int = 0
    turboDuration: int = 0
    turboRecharge: int = 0

    # Detection
    visibility: int = 0
    loudness: int = 0

    # Explosion
    explosionRadius: float = 0.0
    explosionDamage: int = 0
    flammable: bool = False
    explosionDelay: int = 0

    # Passengers
    maxPassengers: int = 0
    hideRider: bool = False
    killRiderOnDeath: bool = False

    # MBII-specific
    MBFstopprimaryfiring: int = 0
    MBFstopaltfiring: int = 0
    MBFdisableshields: int = 0
    CantKnockoutShields: bool = False
    AllWeaponsDoDamageToArmor: bool = False
    AllWeaponsDoDamageToShields: bool = False
    ResistsMarking: bool = False
    SpeedMultiplierForRamDamage: float = 0.0
    RamDamage: int = 0
    VehicleScale: int = 100
    NoDamageWalls: bool = False
    GroundTrace: int = 0
    PlayerAsVehicleEject: bool = False

    # Camera (third person)
    cameraOverride: bool = False
    cameraRange: float = 80.0
    cameraVertOffset: float = 16.0
    cameraHorzOffset: float = 0.0
    cameraPitchOffset: float = 0.0
    cameraFOV: float = 80.0
    cameraAlpha: float = 1.0
    cameraPitchDependantVertOffset: bool = False

    # Camera (first person)
    firstPersonCameraMode: int = 0
    firstPersonCameraRange: float = 0.0
    firstPersonCameraVertOffset: float = 0.0
    firstPersonCameraHorzOffset: float = 0.0
    firstPersonCameraPitchOffset: float = 0.0
    firstPersonCameraFOV: float = 80.0
    firstPersonHideBones: List[str] = field(default_factory=list)

    # Extra fields
    extraFields: Dict[str, str] = field(default_factory=dict)


class VEHParser:
    """Parser for .veh vehicle files"""

    def __init__(self):
        self.errors: List[str] = []
        self.warnings: List[str] = []

    def parse(self, content: str) -> Tuple[VehicleInfo, List[str]]:
        """
        Parse .veh file content into VehicleInfo.

        Args:
            content: The raw file content

        Returns:
            Tuple of (VehicleInfo, list of errors)
        """
        self.errors = []
        self.warnings = []
        vehicle = VehicleInfo()

        # Remove comments
        lines = []
        for line in content.split('\n'):
            comment_idx = line.find('//')
            if comment_idx >= 0:
                line = line[:comment_idx]
            lines.append(line)
        content = '\n'.join(lines)

        # Find vehicle name and block
        match = re.search(r'(\w+)\s*\{([^}]+)\}', content, re.DOTALL)
        if not match:
            self.errors.append("Could not find vehicle definition block")
            return vehicle, self.errors

        vehicle.name = match.group(1)
        block = match.group(2)

        # Parse key-value pairs
        self._parse_block(block, vehicle)

        return vehicle, self.errors

    def _parse_block(self, block: str, vehicle: VehicleInfo):
        """Parse the vehicle definition block"""
        quoted_re = re.compile(r'(\w+)\s+"([^"]*)"')
        unquoted_re = re.compile(r'(\w+)\s+([^\s]+)')
        vector_re = re.compile(r'(\w+)\s+"?([0-9.\-]+)\s+([0-9.\-]+)\s+([0-9.\-]+)"?')

        for line in block.split('\n'):
            line = line.strip()
            if not line:
                continue

            key = None
            value = None

            # Try vector first
            m = vector_re.match(line)
            if m:
                key = m.group(1).lower()
                value = [float(m.group(2)), float(m.group(3)), float(m.group(4))]
                self._set_vector_field(vehicle, key, value)
                continue

            # Try quoted
            m = quoted_re.match(line)
            if m:
                key, value = m.group(1), m.group(2)
            else:
                m = unquoted_re.match(line)
                if m:
                    key, value = m.group(1), m.group(2)

            if key and value:
                self._set_field(vehicle, key.lower(), value)

    def _set_vector_field(self, vehicle: VehicleInfo, key: str, value: List[float]):
        """Set a vector field"""
        if key == "centerofgravity":
            vehicle.centerOfGravity = value

    def _set_field(self, vehicle: VehicleInfo, key: str, value: str):
        """Set a field on the vehicle"""
        # Type
        if key == "name":
            pass  # Already set from block header
        elif key == "type":
            vehicle.vehicleType = value.upper()

        # General
        elif key == "numhands":
            vehicle.numHands = int(value)
        elif key == "lookpitch":
            vehicle.lookPitch = float(value)
        elif key == "lookyaw":
            vehicle.lookYaw = float(value)
        elif key == "length":
            vehicle.length = float(value)
        elif key == "width":
            vehicle.width = float(value)
        elif key == "height":
            vehicle.height = float(value)

        # Speed
        elif key == "speedmax":
            vehicle.speedMax = float(value)
        elif key == "turbospeed":
            vehicle.turboSpeed = float(value)
        elif key == "speedmin":
            vehicle.speedMin = float(value)
        elif key == "speedidle":
            vehicle.speedIdle = float(value)
        elif key == "accelidle":
            vehicle.accelIdle = float(value)
        elif key == "acceleration":
            vehicle.acceleration = float(value)
        elif key == "decelidle":
            vehicle.decelIdle = float(value)
        elif key == "throttlesticks":
            vehicle.throttleSticks = value == "1"
        elif key == "strafeperc":
            vehicle.strafePerc = float(value)

        # Handling
        elif key == "bankingspeed":
            vehicle.bankingSpeed = float(value)
        elif key == "rolllimit":
            vehicle.rollLimit = float(value)
        elif key == "pitchlimit":
            vehicle.pitchLimit = float(value)
        elif key == "braking":
            vehicle.braking = float(value)
        elif key == "mouseyaw":
            vehicle.mouseYaw = float(value)
        elif key == "mousepitch":
            vehicle.mousePitch = float(value)
        elif key == "turningspeed":
            vehicle.turningSpeed = float(value)
        elif key == "turnwhenstopped":
            vehicle.turnWhenStopped = value == "1"
        elif key == "traction":
            vehicle.traction = float(value)
        elif key == "friction":
            vehicle.friction = float(value)
        elif key == "maxslope":
            vehicle.maxSlope = float(value)
        elif key == "speeddependantturning":
            vehicle.speedDependantTurning = value == "1"

        # Durability
        elif key == "mass":
            vehicle.mass = int(value)
        elif key == "armor":
            vehicle.armor = int(value)
        elif key == "shields":
            vehicle.shields = int(value)
        elif key == "shieldrechargems":
            vehicle.shieldRechargeMS = int(value)
        elif key == "toughness":
            vehicle.toughness = float(value)
        elif key == "malfunctionarmorlevel":
            vehicle.malfunctionArmorLevel = int(value)
        elif key == "surfdestruction":
            vehicle.surfDestruction = value == "1"
        elif key == "health_front":
            vehicle.health_front = int(value)
        elif key == "health_back":
            vehicle.health_back = int(value)
        elif key == "health_right":
            vehicle.health_right = int(value)
        elif key == "health_left":
            vehicle.health_left = int(value)

        # Model
        elif key == "model":
            vehicle.model = value
        elif key == "skin":
            vehicle.skin = value
        elif key == "g2radius":
            vehicle.g2radius = int(value)
        elif key == "droidnpc":
            vehicle.droidNPC = value

        # UI
        elif key == "radaricon":
            vehicle.radarIcon = value
        elif key == "dmgindicframe":
            vehicle.dmgIndicFrame = value
        elif key == "dmgindicshield":
            vehicle.dmgIndicShield = value
        elif key == "dmgindicbackground":
            vehicle.dmgIndicBackground = value
        elif key == "icon_front":
            vehicle.icon_front = value
        elif key == "icon_back":
            vehicle.icon_back = value
        elif key == "icon_right":
            vehicle.icon_right = value
        elif key == "icon_left":
            vehicle.icon_left = value
        elif key == "crosshairshader":
            vehicle.crosshairShader = value
        elif key == "shieldshader":
            vehicle.shieldShader = value

        # Sounds
        elif key == "soundon":
            vehicle.soundOn = value
        elif key == "soundoff":
            vehicle.soundOff = value
        elif key == "soundloop":
            vehicle.soundLoop = value
        elif key == "soundtakeoff":
            vehicle.soundTakeOff = value
        elif key == "soundenginestart":
            vehicle.soundEngineStart = value
        elif key == "soundspin":
            vehicle.soundSpin = value
        elif key == "soundturbo":
            vehicle.soundTurbo = value
        elif key == "soundhyper":
            vehicle.soundHyper = value
        elif key == "soundland":
            vehicle.soundLand = value
        elif key == "soundflyby":
            vehicle.soundFlyBy = value
        elif key == "soundflyby2":
            vehicle.soundFlyBy2 = value
        elif key.startswith("soundshift"):
            setattr(vehicle, key, value)

        # Effects
        elif key == "rideranim":
            vehicle.riderAnim = value
        elif key == "exhaustfx":
            vehicle.exhaustFX = value
        elif key == "turbofx":
            vehicle.turboFX = value
        elif key == "turbostartfx":
            vehicle.turboStartFX = value
        elif key == "trailfx":
            vehicle.trailFX = value
        elif key == "impactfx":
            vehicle.impactFX = value
        elif key == "explodefx":
            vehicle.explodeFX = value
        elif key == "wakefx":
            vehicle.wakeFX = value
        elif key == "dmgfx":
            vehicle.dmgFX = value
        elif key == "injurefx":
            vehicle.injureFX = value
        elif key == "nosefx":
            vehicle.noseFX = value
        elif key == "lwingfx":
            vehicle.lwingFX = value
        elif key == "rwingfx":
            vehicle.rwingFX = value
        elif key == "nofireball":
            vehicle.noFireball = value == "1"

        # Weapons
        elif key == "weap1":
            vehicle.weap1 = value
        elif key == "weap1delay":
            vehicle.weap1Delay = int(value)
        elif key == "weap1link":
            vehicle.weap1Link = int(value)
        elif key == "weap1aim":
            vehicle.weap1Aim = value == "1"
        elif key == "weap1ammomax":
            vehicle.weap1AmmoMax = int(value)
        elif key == "weap1ammorechargems":
            vehicle.weap1AmmoRechargeMS = int(value)
        elif key == "weap1soundnoammo":
            vehicle.weap1SoundNoAmmo = value
        elif key == "weap2":
            vehicle.weap2 = value
        elif key == "weap2delay":
            vehicle.weap2Delay = int(value)
        elif key == "weap2link":
            vehicle.weap2Link = int(value)
        elif key == "weap2aim":
            vehicle.weap2Aim = value == "1"
        elif key == "weap2ammomax":
            vehicle.weap2AmmoMax = int(value)
        elif key == "weap2ammorechargems":
            vehicle.weap2AmmoRechargeMS = int(value)
        elif key == "weap2soundnoammo":
            vehicle.weap2SoundNoAmmo = value

        # Muzzles
        elif key.startswith("weapmuzzle") and key[10:].isdigit():
            idx = int(key[10:])
            vehicle.weapMuzzles[idx] = value

        # Turret 1
        elif key.startswith("turret1"):
            self._set_turret_field(vehicle.turret1, key[7:], value)
        # Turret 2
        elif key.startswith("turret2"):
            self._set_turret_field(vehicle.turret2, key[7:], value)

        # Flight
        elif key == "landingheight":
            vehicle.landingHeight = float(value)
        elif key == "gravity":
            vehicle.gravity = int(value)
        elif key == "hoverheight":
            vehicle.hoverHeight = float(value)
        elif key == "hoverstrength":
            vehicle.hoverStrength = float(value)
        elif key == "waterproof":
            vehicle.waterProof = value == "1"
        elif key == "bouyancy":
            vehicle.bouyancy = float(value)

        # Fuel
        elif key == "fuelmax":
            vehicle.fuelMax = int(value)
        elif key == "fuelrate":
            vehicle.fuelRate = int(value)
        elif key == "turboduration":
            vehicle.turboDuration = int(value)
        elif key == "turborecharge":
            vehicle.turboRecharge = int(value)

        # Detection
        elif key == "visibility":
            vehicle.visibility = int(value)
        elif key == "loudness":
            vehicle.loudness = int(value)

        # Explosion
        elif key == "explosionradius":
            vehicle.explosionRadius = float(value)
        elif key == "explosiondamage":
            vehicle.explosionDamage = int(value)
        elif key == "flammable":
            vehicle.flammable = value == "1"
        elif key == "explosiondelay":
            vehicle.explosionDelay = int(value)

        # Passengers
        elif key == "maxpassengers":
            vehicle.maxPassengers = int(value)
        elif key == "hiderider":
            vehicle.hideRider = value == "1"
        elif key == "killriderondeath":
            vehicle.killRiderOnDeath = value == "1"

        # MBII specific
        elif key == "mbfstopprimaryfiring":
            vehicle.MBFstopprimaryfiring = int(value)
        elif key == "mbfstopaltfiring":
            vehicle.MBFstopaltfiring = int(value)
        elif key == "mbfdisableshields":
            vehicle.MBFdisableshields = int(value)
        elif key == "cantknockoutshields":
            vehicle.CantKnockoutShields = value == "1"
        elif key == "allweaponsdodamagetoarmor":
            vehicle.AllWeaponsDoDamageToArmor = value == "1"
        elif key == "allweaponsdodamagetoshields":
            vehicle.AllWeaponsDoDamageToShields = value == "1"
        elif key == "resistsmarking":
            vehicle.ResistsMarking = value == "1"
        elif key == "speedmultiplierforramdamage":
            vehicle.SpeedMultiplierForRamDamage = float(value)
        elif key == "ramdamage":
            vehicle.RamDamage = int(value)
        elif key == "vehiclescale":
            vehicle.VehicleScale = int(value)
        elif key == "nodamagewalls":
            vehicle.NoDamageWalls = value == "1"
        elif key == "groundtrace":
            vehicle.GroundTrace = int(value)
        elif key == "playerasvehicleeject":
            vehicle.PlayerAsVehicleEject = value == "1"

        # Camera third person
        elif key == "cameraoverride":
            vehicle.cameraOverride = value == "1"
        elif key == "camerarange":
            vehicle.cameraRange = float(value)
        elif key == "cameravertoffset":
            vehicle.cameraVertOffset = float(value)
        elif key == "camerahorzoffset":
            vehicle.cameraHorzOffset = float(value)
        elif key == "camerapitchoffset":
            vehicle.cameraPitchOffset = float(value)
        elif key == "camerafov":
            vehicle.cameraFOV = float(value)
        elif key == "cameraalpha":
            vehicle.cameraAlpha = float(value)
        elif key == "camerapitchdependantvertoffset":
            vehicle.cameraPitchDependantVertOffset = value == "1"

        # Camera first person
        elif key == "firstpersoncameramode":
            vehicle.firstPersonCameraMode = int(value)
        elif key == "firstpersoncamerarange":
            vehicle.firstPersonCameraRange = float(value)
        elif key == "firstpersoncameravertoffset":
            vehicle.firstPersonCameraVertOffset = float(value)
        elif key == "firstpersoncamerahorzoffset":
            vehicle.firstPersonCameraHorzOffset = float(value)
        elif key == "firstpersoncamerapitchoffset":
            vehicle.firstPersonCameraPitchOffset = float(value)
        elif key == "firstpersoncamerafov":
            vehicle.firstPersonCameraFOV = float(value)
        elif key.startswith("firstpersonhidebone"):
            vehicle.firstPersonHideBones.append(value)

        else:
            vehicle.extraFields[key] = value

    def _set_turret_field(self, turret: TurretInfo, key: str, value: str):
        """Set a turret field"""
        key = key.lower()
        if key == "weap":
            turret.weapon = value
        elif key == "delay":
            turret.delay = int(value)
        elif key == "ammomax":
            turret.ammoMax = int(value)
        elif key == "ammorechargems":
            turret.ammoRechargeMS = int(value)
        elif key == "yawbone":
            turret.yawBone = value
        elif key == "pitchbone":
            turret.pitchBone = value
        elif key == "yawaxis":
            turret.yawAxis = int(value)
        elif key == "pitchaxis":
            turret.pitchAxis = int(value)
        elif key == "clampyawl":
            turret.clampYawL = float(value)
        elif key == "clampyawr":
            turret.clampYawR = float(value)
        elif key == "clamppitchu":
            turret.clampPitchU = float(value)
        elif key == "clamppitchd":
            turret.clampPitchD = float(value)
        elif key == "muzzle1":
            turret.muzzle1 = int(value)
        elif key == "muzzle2":
            turret.muzzle2 = int(value)
        elif key == "turnspeed":
            turret.turnSpeed = float(value)
        elif key == "ai":
            turret.ai = value == "1"
        elif key == "ailead":
            turret.aiLead = value == "1"
        elif key == "airange":
            turret.aiRange = float(value)
        elif key == "passengernum":
            turret.passengerNum = int(value)
        elif key == "gunnerviewtag":
            turret.gunnerViewTag = value

    def generate(self, vehicle: VehicleInfo) -> str:
        """Generate .veh file content from VehicleInfo"""
        lines = [f"// {vehicle.name}", "", f"{vehicle.name}", "{"]

        # Type
        lines.append(f'\ttype\t\t\t{vehicle.vehicleType}')

        # Model
        if vehicle.model:
            lines.append(f'\tmodel\t\t\t"{vehicle.model}"')
        if vehicle.skin:
            lines.append(f'\tskin\t\t\t"{vehicle.skin}"')

        # Dimensions
        if vehicle.length:
            lines.append(f'\tlength\t\t\t{vehicle.length}')
        if vehicle.width:
            lines.append(f'\twidth\t\t\t{vehicle.width}')
        if vehicle.height:
            lines.append(f'\theight\t\t\t{vehicle.height}')

        # Speed
        if vehicle.speedMax:
            lines.append(f'\tspeedMax\t\t{vehicle.speedMax}')
        if vehicle.turboSpeed:
            lines.append(f'\tturboSpeed\t\t{vehicle.turboSpeed}')
        if vehicle.acceleration:
            lines.append(f'\tacceleration\t\t{vehicle.acceleration}')
        if vehicle.braking:
            lines.append(f'\tbraking\t\t\t{vehicle.braking}')

        # Durability
        if vehicle.armor:
            lines.append(f'\tarmor\t\t\t{vehicle.armor}')
        if vehicle.shields:
            lines.append(f'\tshields\t\t\t{vehicle.shields}')
        if vehicle.mass:
            lines.append(f'\tmass\t\t\t{vehicle.mass}')

        # Weapons
        if vehicle.weap1:
            lines.append(f'\tweap1\t\t\t{vehicle.weap1}')
            if vehicle.weap1Delay:
                lines.append(f'\tweap1Delay\t\t{vehicle.weap1Delay}')
            if vehicle.weap1AmmoMax:
                lines.append(f'\tweap1AmmoMax\t\t{vehicle.weap1AmmoMax}')

        if vehicle.weap2:
            lines.append(f'\tweap2\t\t\t{vehicle.weap2}')
            if vehicle.weap2Delay:
                lines.append(f'\tweap2Delay\t\t{vehicle.weap2Delay}')
            if vehicle.weap2AmmoMax:
                lines.append(f'\tweap2AmmoMax\t\t{vehicle.weap2AmmoMax}')

        # Effects
        if vehicle.exhaustFX:
            lines.append(f'\texhaustFX\t\t"{vehicle.exhaustFX}"')
        if vehicle.explodeFX:
            lines.append(f'\texplodeFX\t\t"{vehicle.explodeFX}"')

        # Sounds
        if vehicle.soundLoop:
            lines.append(f'\tsoundLoop\t\t"{vehicle.soundLoop}"')

        # Extra fields
        for key, value in vehicle.extraFields.items():
            if ' ' in str(value):
                lines.append(f'\t{key}\t\t\t"{value}"')
            else:
                lines.append(f'\t{key}\t\t\t{value}')

        lines.append("}")
        return '\n'.join(lines)

    def validate(self, vehicle: VehicleInfo) -> List[str]:
        """Validate a vehicle configuration"""
        errors = []

        if not vehicle.name:
            errors.append("Vehicle name is required")
        if not vehicle.model:
            errors.append("Vehicle model is required")
        if vehicle.armor <= 0:
            errors.append("Vehicle should have positive armor")

        return errors


def parse_veh(content: str) -> Tuple[VehicleInfo, List[str]]:
    """Convenience function to parse .veh content"""
    parser = VEHParser()
    return parser.parse(content)


def generate_veh(vehicle: VehicleInfo) -> str:
    """Convenience function to generate .veh content"""
    parser = VEHParser()
    return parser.generate(vehicle)


def validate_veh(vehicle: VehicleInfo) -> List[str]:
    """Convenience function to validate a vehicle"""
    parser = VEHParser()
    return parser.validate(vehicle)
