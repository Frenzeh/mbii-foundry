"""
Asset Resolver

Resolves dependencies and finds required assets for MBII content.
Parses .mbch, .sab, and other files to find referenced models, sounds, etc.
"""

import os
import re
from dataclasses import dataclass, field
from typing import List, Dict, Set, Optional, Tuple
from pathlib import Path


@dataclass
class AssetReference:
    """Represents a reference to an asset"""
    asset_type: str
    name: str
    path: str
    source_file: str
    required: bool = True
    found: bool = False
    resolved_path: Optional[str] = None


@dataclass
class AssetResolver:
    """
    Resolves asset dependencies from MBII content files.

    Searches specified asset directories to find referenced files.
    """
    # Directories to search for assets
    search_paths: List[str] = field(default_factory=list)

    # Cache of found assets
    _asset_cache: Dict[str, str] = field(default_factory=dict)

    def add_search_path(self, path: str) -> None:
        """Add a directory to search for assets"""
        if path not in self.search_paths:
            self.search_paths.append(path)
            self._asset_cache.clear()  # Clear cache when paths change

    def find_asset(self, relative_path: str) -> Optional[str]:
        """
        Find an asset in the search paths.

        Args:
            relative_path: The relative path to the asset (e.g., 'models/players/cultist')

        Returns:
            The full path if found, None otherwise
        """
        # Check cache first
        if relative_path in self._asset_cache:
            return self._asset_cache[relative_path]

        # Search in all paths
        for search_path in self.search_paths:
            full_path = os.path.join(search_path, relative_path)
            if os.path.exists(full_path):
                self._asset_cache[relative_path] = full_path
                return full_path

        return None

    def resolve_mbch_assets(self, mbch_path: str) -> List[AssetReference]:
        """
        Parse an .mbch file and resolve all referenced assets.

        Args:
            mbch_path: Path to the .mbch file

        Returns:
            List of AssetReference objects
        """
        references = []

        try:
            with open(mbch_path, 'r', encoding='utf-8', errors='ignore') as f:
                content = f.read()
        except Exception as e:
            return references

        # Parse model reference
        model_match = re.search(r'model\s+"?([^"\s]+)"?', content, re.IGNORECASE)
        if model_match:
            model_name = model_match.group(1)
            ref = AssetReference(
                asset_type="model",
                name=model_name,
                path=f"models/players/{model_name}",
                source_file=mbch_path,
                required=True
            )
            ref.resolved_path = self.find_asset(ref.path)
            ref.found = ref.resolved_path is not None
            references.append(ref)

            # Also look for skins in the model folder
            skin_refs = self._resolve_model_skins(model_name, mbch_path)
            references.extend(skin_refs)

        # Parse skin reference
        skin_match = re.search(r'skin\s+"?([^"\s]+)"?', content, re.IGNORECASE)
        if skin_match and model_match:
            skin_name = skin_match.group(1)
            model_name = model_match.group(1)
            ref = AssetReference(
                asset_type="skin",
                name=skin_name,
                path=f"models/players/{model_name}/model_{skin_name}.skin",
                source_file=mbch_path,
                required=True
            )
            ref.resolved_path = self.find_asset(ref.path)
            ref.found = ref.resolved_path is not None
            references.append(ref)

        # Parse UI shader (icon)
        uishader_match = re.search(r'uishader\s+"?([^"\s]+)"?', content, re.IGNORECASE)
        if uishader_match:
            shader_path = uishader_match.group(1)
            # Try common extensions
            for ext in ['.tga', '.jpg', '.png', '']:
                ref = AssetReference(
                    asset_type="icon",
                    name=os.path.basename(shader_path),
                    path=f"{shader_path}{ext}" if not shader_path.endswith(('.tga', '.jpg', '.png')) else shader_path,
                    source_file=mbch_path,
                    required=False
                )
                ref.resolved_path = self.find_asset(ref.path)
                ref.found = ref.resolved_path is not None
                if ref.found:
                    references.append(ref)
                    break
            else:
                # Add unfound reference
                ref = AssetReference(
                    asset_type="icon",
                    name=os.path.basename(shader_path),
                    path=shader_path,
                    source_file=mbch_path,
                    required=False,
                    found=False
                )
                references.append(ref)

        # Parse soundset
        soundset_match = re.search(r'soundset\s+"?([^"\s]+)"?', content, re.IGNORECASE)
        if soundset_match:
            soundset = soundset_match.group(1)
            ref = AssetReference(
                asset_type="soundset",
                name=soundset,
                path=f"sound/chars/{soundset}",
                source_file=mbch_path,
                required=False
            )
            ref.resolved_path = self.find_asset(ref.path)
            ref.found = ref.resolved_path is not None
            references.append(ref)

        # Parse saber references
        for saber_field in ['saber1', 'saber2']:
            saber_match = re.search(rf'{saber_field}\s+"?([^"\s]+)"?', content, re.IGNORECASE)
            if saber_match:
                saber_name = saber_match.group(1)
                if saber_name and saber_name.lower() != 'none':
                    ref = AssetReference(
                        asset_type="saber",
                        name=saber_name,
                        path=f"ext_data/sabers/{saber_name}.sab",
                        source_file=mbch_path,
                        required=False
                    )
                    ref.resolved_path = self.find_asset(ref.path)
                    ref.found = ref.resolved_path is not None
                    references.append(ref)

        return references

    def _resolve_model_skins(self, model_name: str, source_file: str) -> List[AssetReference]:
        """Find all skin files for a model"""
        references = []
        model_path = f"models/players/{model_name}"

        resolved = self.find_asset(model_path)
        if resolved and os.path.isdir(resolved):
            for filename in os.listdir(resolved):
                if filename.endswith('.skin'):
                    ref = AssetReference(
                        asset_type="skin",
                        name=filename,
                        path=f"{model_path}/{filename}",
                        source_file=source_file,
                        required=False,
                        found=True,
                        resolved_path=os.path.join(resolved, filename)
                    )
                    references.append(ref)

        return references

    def resolve_sab_assets(self, sab_path: str) -> List[AssetReference]:
        """
        Parse a .sab file and resolve referenced assets.

        Args:
            sab_path: Path to the .sab file

        Returns:
            List of AssetReference objects
        """
        references = []

        try:
            with open(sab_path, 'r', encoding='utf-8', errors='ignore') as f:
                content = f.read()
        except Exception:
            return references

        # Parse saber model
        model_match = re.search(r'saberModel\s+"?([^"\s]+)"?', content, re.IGNORECASE)
        if model_match:
            model_path = model_match.group(1)
            ref = AssetReference(
                asset_type="saber_model",
                name=os.path.basename(model_path),
                path=model_path,
                source_file=sab_path,
                required=True
            )
            ref.resolved_path = self.find_asset(ref.path)
            ref.found = ref.resolved_path is not None
            references.append(ref)

        return references


def resolve_model_assets(model_path: str, search_paths: List[str]) -> List[AssetReference]:
    """
    Resolve all assets for a player model.

    Args:
        model_path: Path to model directory (e.g., 'models/players/cultist')
        search_paths: Directories to search

    Returns:
        List of found assets
    """
    resolver = AssetResolver(search_paths=search_paths)
    references = []

    resolved = resolver.find_asset(model_path)
    if not resolved or not os.path.isdir(resolved):
        return references

    # Find all files in the model directory
    for root, dirs, files in os.walk(resolved):
        for filename in files:
            full_path = os.path.join(root, filename)
            rel_path = os.path.relpath(full_path, os.path.dirname(resolved))

            # Determine asset type
            ext = os.path.splitext(filename)[1].lower()
            if ext == '.glm':
                asset_type = "model"
            elif ext == '.gla':
                asset_type = "animation"
            elif ext == '.skin':
                asset_type = "skin"
            elif ext in ['.tga', '.jpg', '.png']:
                asset_type = "texture"
            elif ext == '.shader':
                asset_type = "shader"
            else:
                asset_type = "other"

            ref = AssetReference(
                asset_type=asset_type,
                name=filename,
                path=rel_path,
                source_file=model_path,
                required=(ext in ['.glm', '.skin']),
                found=True,
                resolved_path=full_path
            )
            references.append(ref)

    return references


def resolve_sound_assets(soundset: str, search_paths: List[str]) -> List[AssetReference]:
    """
    Resolve all sound files for a soundset.

    Args:
        soundset: Name of the soundset
        search_paths: Directories to search

    Returns:
        List of found sound assets
    """
    resolver = AssetResolver(search_paths=search_paths)
    references = []

    sound_path = f"sound/chars/{soundset}"
    resolved = resolver.find_asset(sound_path)

    if not resolved or not os.path.isdir(resolved):
        return references

    for filename in os.listdir(resolved):
        if filename.endswith(('.wav', '.mp3')):
            ref = AssetReference(
                asset_type="sound",
                name=filename,
                path=f"{sound_path}/{filename}",
                source_file=soundset,
                required=False,
                found=True,
                resolved_path=os.path.join(resolved, filename)
            )
            references.append(ref)

    return references


def resolve_shader_assets(shader_name: str, search_paths: List[str]) -> List[AssetReference]:
    """
    Resolve shader file and its referenced textures.

    Args:
        shader_name: Name of the shader
        search_paths: Directories to search

    Returns:
        List of found shader/texture assets
    """
    resolver = AssetResolver(search_paths=search_paths)
    references = []

    # Search for shader files
    shader_path = f"shaders/{shader_name}.shader"
    resolved = resolver.find_asset(shader_path)

    if resolved:
        ref = AssetReference(
            asset_type="shader",
            name=shader_name,
            path=shader_path,
            source_file=shader_name,
            required=False,
            found=True,
            resolved_path=resolved
        )
        references.append(ref)

        # TODO: Parse shader file for texture references

    return references


def find_missing_assets(references: List[AssetReference]) -> List[AssetReference]:
    """
    Filter to only missing (unfound) assets.

    Args:
        references: List of asset references

    Returns:
        List of missing assets
    """
    return [ref for ref in references if not ref.found and ref.required]
