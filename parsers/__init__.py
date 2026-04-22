"""
FA Creator Parsers Package

Provides parsing and file management for MBII Full Authentic content files:
- .mbch - Character class definitions
- .sab  - Saber configurations
- .veh  - Vehicle definitions
- .siege - Siege mode class configs (uses mbch_parser)
"""

from .mbch_parser import (
    MBCHParser,
    MBCHCharacter,
    WeaponInfo,
    ForceInfo
)

from .sab_parser import (
    SABParser,
    SaberInfo,
    BladeInfo,
    SaberType,
    SaberColor,
    SABER_FLAGS,
    SABER_STYLES,
    parse_sab,
    generate_sab,
    validate_sab
)

from .veh_parser import (
    VEHParser,
    VehicleInfo,
    TurretInfo,
    VehicleType,
    VEHICLE_MODES,
    parse_veh,
    generate_veh,
    validate_veh
)

from .file_manager import (
    MBCHFileManager,
    get_manager,
    load_file,
    save_file,
    load_template,
    export_json,
    load_json
)

__all__ = [
    # MBCH Parser
    'MBCHParser',
    'MBCHCharacter',
    'WeaponInfo',
    'ForceInfo',
    # SAB Parser
    'SABParser',
    'SaberInfo',
    'BladeInfo',
    'SaberType',
    'SaberColor',
    'SABER_FLAGS',
    'SABER_STYLES',
    'parse_sab',
    'generate_sab',
    'validate_sab',
    # VEH Parser
    'VEHParser',
    'VehicleInfo',
    'TurretInfo',
    'VehicleType',
    'VEHICLE_MODES',
    'parse_veh',
    'generate_veh',
    'validate_veh',
    # File manager
    'MBCHFileManager',
    'get_manager',
    'load_file',
    'save_file',
    'load_template',
    'export_json',
    'load_json'
]
