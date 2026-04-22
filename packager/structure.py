"""
PK3 Structure Definitions

Defines the expected directory structure for MBII PK3 files and asset paths.
"""

from dataclasses import dataclass, field
from typing import Dict, List, Optional
from enum import Enum


class AssetType(Enum):
    """Types of assets in MBII"""
    CHARACTER = "character"      # .mbch files
    SABER = "saber"              # .sab files
    SIEGE = "siege"              # .siege files
    TEAM_CONFIG = "team_config"  # .mbtc files
    MODEL = "model"              # .glm files
    SKIN = "skin"                # .skin files
    TEXTURE = "texture"          # .tga/.jpg files
    SHADER = "shader"            # shader definitions
    SOUND = "sound"              # .wav/.mp3 files
    ICON = "icon"                # UI icons
    EFFECT = "effect"            # .efx files


# Standard MBII PK3 paths
MBII_PATHS = {
    # Character/Class definitions
    AssetType.CHARACTER: "ext_data/mb2/character",

    # Saber definitions
    AssetType.SABER: "ext_data/sabers",

    # Siege configurations
    AssetType.SIEGE: "ext_data/siege",

    # Team configurations
    AssetType.TEAM_CONFIG: "ext_data/mb2/teams",

    # Player models
    AssetType.MODEL: "models/players",

    # Skins (within model folder)
    AssetType.SKIN: "models/players/{model}",

    # Textures (within model folder)
    AssetType.TEXTURE: "models/players/{model}",

    # Shaders
    AssetType.SHADER: "shaders",

    # Character sounds
    AssetType.SOUND: "sound/chars/{soundset}",

    # HUD icons
    AssetType.ICON: "gfx/mb2/hud",

    # Effects
    AssetType.EFFECT: "effects",
}


# File extensions by asset type
ASSET_EXTENSIONS = {
    AssetType.CHARACTER: [".mbch"],
    AssetType.SABER: [".sab"],
    AssetType.SIEGE: [".siege"],
    AssetType.TEAM_CONFIG: [".mbtc"],
    AssetType.MODEL: [".glm", ".gla"],
    AssetType.SKIN: [".skin"],
    AssetType.TEXTURE: [".tga", ".jpg", ".png"],
    AssetType.SHADER: [".shader"],
    AssetType.SOUND: [".wav", ".mp3"],
    AssetType.ICON: [".tga", ".jpg", ".png"],
    AssetType.EFFECT: [".efx"],
}


@dataclass
class PK3Structure:
    """Represents the structure of a PK3 package"""
    name: str
    description: str = ""
    version: str = "1.0.0"
    author: str = ""

    # Assets organized by type
    characters: List[str] = field(default_factory=list)
    sabers: List[str] = field(default_factory=list)
    siege_configs: List[str] = field(default_factory=list)
    team_configs: List[str] = field(default_factory=list)

    # Dependent assets (auto-resolved)
    models: List[str] = field(default_factory=list)
    skins: List[str] = field(default_factory=list)
    textures: List[str] = field(default_factory=list)
    sounds: List[str] = field(default_factory=list)
    icons: List[str] = field(default_factory=list)
    shaders: List[str] = field(default_factory=list)
    effects: List[str] = field(default_factory=list)

    # Additional files
    other_files: List[str] = field(default_factory=list)

    def get_all_assets(self) -> Dict[AssetType, List[str]]:
        """Get all assets organized by type"""
        return {
            AssetType.CHARACTER: self.characters,
            AssetType.SABER: self.sabers,
            AssetType.SIEGE: self.siege_configs,
            AssetType.TEAM_CONFIG: self.team_configs,
            AssetType.MODEL: self.models,
            AssetType.SKIN: self.skins,
            AssetType.TEXTURE: self.textures,
            AssetType.SOUND: self.sounds,
            AssetType.ICON: self.icons,
            AssetType.SHADER: self.shaders,
            AssetType.EFFECT: self.effects,
        }

    def total_asset_count(self) -> int:
        """Get total number of assets"""
        return sum(len(assets) for assets in self.get_all_assets().values())


def get_asset_path(asset_type: AssetType, **kwargs) -> str:
    """
    Get the PK3 path for an asset type.

    Args:
        asset_type: The type of asset
        **kwargs: Format parameters (e.g., model='cultist', soundset='male')

    Returns:
        The formatted path string
    """
    path_template = MBII_PATHS.get(asset_type, "")
    if kwargs:
        return path_template.format(**kwargs)
    return path_template


def validate_structure(structure: PK3Structure) -> List[str]:
    """
    Validate a PK3 structure for common issues.

    Args:
        structure: The PK3Structure to validate

    Returns:
        List of validation errors (empty if valid)
    """
    errors = []

    # Check required fields
    if not structure.name:
        errors.append("Package name is required")

    if not structure.name.replace("_", "").replace("-", "").isalnum():
        errors.append("Package name should be alphanumeric (underscores/hyphens allowed)")

    # Check for content
    if structure.total_asset_count() == 0:
        errors.append("Package has no assets")

    # Validate character files have proper naming
    for char in structure.characters:
        if not char.endswith(".mbch"):
            errors.append(f"Character file should have .mbch extension: {char}")

    # Validate saber files
    for sab in structure.sabers:
        if not sab.endswith(".sab"):
            errors.append(f"Saber file should have .sab extension: {sab}")

    return errors


def get_file_asset_type(filename: str) -> Optional[AssetType]:
    """
    Determine the asset type from a filename.

    Args:
        filename: The filename to check

    Returns:
        The AssetType or None if unknown
    """
    filename_lower = filename.lower()

    for asset_type, extensions in ASSET_EXTENSIONS.items():
        for ext in extensions:
            if filename_lower.endswith(ext):
                return asset_type

    return None
