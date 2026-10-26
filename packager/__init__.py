"""
FA Creator Packager Module

Provides PK3 packaging, validation, and asset management for MBII content.
Supports .mbch (characters), .sab (sabers), .siege, and .mbtc files.
"""

from .pk3_builder import (
    PK3Builder,
    PackageManifest,
    AssetReference,
    build_pk3,
    validate_package,
    clean_package
)

from .asset_resolver import (
    AssetResolver,
    resolve_model_assets,
    resolve_sound_assets,
    resolve_shader_assets,
    find_missing_assets
)

from .structure import (
    PK3Structure,
    MBII_PATHS,
    get_asset_path,
    validate_structure
)

__all__ = [
    # PK3 Builder
    'PK3Builder',
    'PackageManifest',
    'AssetReference',
    'build_pk3',
    'validate_package',
    'clean_package',
    # Asset Resolver
    'AssetResolver',
    'resolve_model_assets',
    'resolve_sound_assets',
    'resolve_shader_assets',
    'find_missing_assets',
    # Structure
    'PK3Structure',
    'MBII_PATHS',
    'get_asset_path',
    'validate_structure'
]
