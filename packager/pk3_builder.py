"""
PK3 Builder

Creates PK3 packages from FA Creator content with proper structure,
dependency resolution, and validation.
"""

import os
import json
import zipfile
import shutil
import tempfile
from datetime import datetime
from dataclasses import dataclass, field, asdict
from typing import List, Dict, Optional, Set, Tuple
from pathlib import Path

from .structure import (
    PK3Structure, AssetType, MBII_PATHS, ASSET_EXTENSIONS,
    get_asset_path, validate_structure, get_file_asset_type
)
from .asset_resolver import AssetResolver, AssetReference, find_missing_assets


@dataclass
class PackageManifest:
    """Manifest describing a PK3 package"""
    name: str
    version: str = "1.0.0"
    author: str = ""
    description: str = ""
    created: str = ""
    modified: str = ""

    # Content summary
    characters: List[str] = field(default_factory=list)
    sabers: List[str] = field(default_factory=list)
    models: List[str] = field(default_factory=list)
    total_files: int = 0
    total_size: int = 0

    # Build info
    build_tool: str = "FA Creator"
    build_version: str = "1.0.0"

    def to_json(self) -> str:
        """Convert to JSON string"""
        return json.dumps(asdict(self), indent=2)

    @classmethod
    def from_json(cls, json_str: str) -> 'PackageManifest':
        """Create from JSON string"""
        data = json.loads(json_str)
        return cls(**data)


@dataclass
class BuildResult:
    """Result of a PK3 build operation"""
    success: bool
    output_path: str = ""
    errors: List[str] = field(default_factory=list)
    warnings: List[str] = field(default_factory=list)
    files_included: int = 0
    total_size: int = 0
    missing_assets: List[str] = field(default_factory=list)


class PK3Builder:
    """
    Builds PK3 packages from FA Creator content.

    Handles dependency resolution, validation, and proper directory structure.
    """

    def __init__(self, search_paths: Optional[List[str]] = None):
        """
        Initialize the PK3 builder.

        Args:
            search_paths: Directories to search for assets (e.g., gamedata folders)
        """
        self.search_paths = search_paths or []
        self.resolver = AssetResolver(search_paths=self.search_paths)
        self.temp_dir: Optional[str] = None

    def add_search_path(self, path: str) -> None:
        """Add a directory to search for assets"""
        if path not in self.search_paths:
            self.search_paths.append(path)
            self.resolver.add_search_path(path)

    def build(
        self,
        structure: PK3Structure,
        output_path: str,
        include_manifest: bool = True,
        resolve_dependencies: bool = True,
        validate: bool = True
    ) -> BuildResult:
        """
        Build a PK3 package.

        Args:
            structure: The PK3Structure defining what to include
            output_path: Path for the output .pk3 file
            include_manifest: Whether to include a manifest.json
            resolve_dependencies: Whether to auto-resolve asset dependencies
            validate: Whether to validate before building

        Returns:
            BuildResult with success status and details
        """
        result = BuildResult(success=False)

        # Validate structure
        if validate:
            errors = validate_structure(structure)
            if errors:
                result.errors = errors
                return result

        # Create temp directory for staging
        self.temp_dir = tempfile.mkdtemp(prefix="fa_pk3_")

        try:
            files_to_include: List[Tuple[str, str]] = []  # (source, archive_path)

            # Process characters
            for char_path in structure.characters:
                if os.path.exists(char_path):
                    archive_path = f"{MBII_PATHS[AssetType.CHARACTER]}/{os.path.basename(char_path)}"
                    files_to_include.append((char_path, archive_path))

                    # Resolve dependencies
                    if resolve_dependencies:
                        refs = self.resolver.resolve_mbch_assets(char_path)
                        dep_files, missing = self._process_dependencies(refs)
                        files_to_include.extend(dep_files)
                        result.missing_assets.extend(missing)
                else:
                    result.warnings.append(f"Character file not found: {char_path}")

            # Process sabers
            for sab_path in structure.sabers:
                if os.path.exists(sab_path):
                    archive_path = f"{MBII_PATHS[AssetType.SABER]}/{os.path.basename(sab_path)}"
                    files_to_include.append((sab_path, archive_path))

                    # Resolve dependencies
                    if resolve_dependencies:
                        refs = self.resolver.resolve_sab_assets(sab_path)
                        dep_files, missing = self._process_dependencies(refs)
                        files_to_include.extend(dep_files)
                        result.missing_assets.extend(missing)
                else:
                    result.warnings.append(f"Saber file not found: {sab_path}")

            # Process siege configs
            for siege_path in structure.siege_configs:
                if os.path.exists(siege_path):
                    archive_path = f"{MBII_PATHS[AssetType.SIEGE]}/{os.path.basename(siege_path)}"
                    files_to_include.append((siege_path, archive_path))
                else:
                    result.warnings.append(f"Siege config not found: {siege_path}")

            # Process team configs
            for team_path in structure.team_configs:
                if os.path.exists(team_path):
                    archive_path = f"{MBII_PATHS[AssetType.TEAM_CONFIG]}/{os.path.basename(team_path)}"
                    files_to_include.append((team_path, archive_path))
                else:
                    result.warnings.append(f"Team config not found: {team_path}")

            # Process explicitly listed models
            for model_path in structure.models:
                model_files = self._gather_model_files(model_path)
                files_to_include.extend(model_files)

            # Process other files
            for file_path in structure.other_files:
                if os.path.exists(file_path):
                    # Try to determine correct archive path
                    archive_path = self._determine_archive_path(file_path)
                    files_to_include.append((file_path, archive_path))

            # Remove duplicates
            files_to_include = list(set(files_to_include))

            # Create manifest
            if include_manifest:
                manifest = self._create_manifest(structure, files_to_include)
                manifest_path = os.path.join(self.temp_dir, "manifest.json")
                with open(manifest_path, 'w') as f:
                    f.write(manifest.to_json())
                files_to_include.append((manifest_path, "manifest.json"))

            # Build the PK3
            self._create_pk3(output_path, files_to_include)

            # Calculate stats
            result.files_included = len(files_to_include)
            result.total_size = os.path.getsize(output_path)
            result.output_path = output_path
            result.success = True

        except Exception as e:
            result.errors.append(f"Build failed: {str(e)}")

        finally:
            # Cleanup temp directory
            if self.temp_dir and os.path.exists(self.temp_dir):
                shutil.rmtree(self.temp_dir)
                self.temp_dir = None

        return result

    def _process_dependencies(
        self,
        refs: List[AssetReference]
    ) -> Tuple[List[Tuple[str, str]], List[str]]:
        """
        Process asset references and return files to include.

        Returns:
            Tuple of (files_to_include, missing_assets)
        """
        files = []
        missing = []

        for ref in refs:
            if ref.found and ref.resolved_path:
                if os.path.isfile(ref.resolved_path):
                    files.append((ref.resolved_path, ref.path))
                elif os.path.isdir(ref.resolved_path):
                    # Include all files in directory
                    for root, dirs, filenames in os.walk(ref.resolved_path):
                        for filename in filenames:
                            full_path = os.path.join(root, filename)
                            rel_path = os.path.relpath(full_path, os.path.dirname(ref.resolved_path))
                            archive_path = os.path.join(os.path.dirname(ref.path), rel_path)
                            files.append((full_path, archive_path))
            elif ref.required:
                missing.append(f"{ref.asset_type}: {ref.name} ({ref.path})")

        return files, missing

    def _gather_model_files(self, model_path: str) -> List[Tuple[str, str]]:
        """Gather all files for a model"""
        files = []

        resolved = self.resolver.find_asset(model_path)
        if not resolved or not os.path.isdir(resolved):
            return files

        for root, dirs, filenames in os.walk(resolved):
            for filename in filenames:
                full_path = os.path.join(root, filename)
                rel_path = os.path.relpath(full_path, os.path.dirname(resolved))
                files.append((full_path, rel_path))

        return files

    def _determine_archive_path(self, file_path: str) -> str:
        """Determine the correct archive path for a file"""
        filename = os.path.basename(file_path)
        asset_type = get_file_asset_type(filename)

        if asset_type and asset_type in MBII_PATHS:
            return f"{MBII_PATHS[asset_type]}/{filename}"

        # Default to root
        return filename

    def _create_manifest(
        self,
        structure: PK3Structure,
        files: List[Tuple[str, str]]
    ) -> PackageManifest:
        """Create a manifest for the package"""
        now = datetime.now().isoformat()

        total_size = sum(
            os.path.getsize(f[0]) for f in files
            if os.path.exists(f[0])
        )

        return PackageManifest(
            name=structure.name,
            version=structure.version,
            author=structure.author,
            description=structure.description,
            created=now,
            modified=now,
            characters=[os.path.basename(c) for c in structure.characters],
            sabers=[os.path.basename(s) for s in structure.sabers],
            models=structure.models,
            total_files=len(files),
            total_size=total_size
        )

    def _create_pk3(self, output_path: str, files: List[Tuple[str, str]]) -> None:
        """Create the actual PK3 (ZIP) file"""
        # Ensure output directory exists
        os.makedirs(os.path.dirname(output_path) or '.', exist_ok=True)

        with zipfile.ZipFile(output_path, 'w', zipfile.ZIP_DEFLATED) as pk3:
            for source_path, archive_path in files:
                if os.path.exists(source_path):
                    # Normalize path separators for ZIP
                    archive_path = archive_path.replace('\\', '/')
                    pk3.write(source_path, archive_path)

    def validate_package(self, pk3_path: str) -> List[str]:
        """
        Validate an existing PK3 package.

        Args:
            pk3_path: Path to the PK3 file

        Returns:
            List of validation issues
        """
        issues = []

        if not os.path.exists(pk3_path):
            return [f"PK3 file not found: {pk3_path}"]

        try:
            with zipfile.ZipFile(pk3_path, 'r') as pk3:
                # Check for corrupted files
                bad_file = pk3.testzip()
                if bad_file:
                    issues.append(f"Corrupted file in archive: {bad_file}")

                # Check structure
                names = pk3.namelist()

                # Check for character files in correct location
                for name in names:
                    if name.endswith('.mbch'):
                        if not name.startswith('ext_data/mb2/character/'):
                            issues.append(f"Character file in wrong location: {name}")

                    if name.endswith('.sab'):
                        if not name.startswith('ext_data/sabers/'):
                            issues.append(f"Saber file in wrong location: {name}")

        except zipfile.BadZipFile:
            issues.append("Invalid PK3 file (not a valid ZIP archive)")

        return issues

    def clean_package(
        self,
        pk3_path: str,
        output_path: Optional[str] = None,
        remove_duplicates: bool = True,
        remove_empty: bool = True,
        optimize_textures: bool = False
    ) -> BuildResult:
        """
        Clean and optimize a PK3 package.

        Args:
            pk3_path: Path to the PK3 file
            output_path: Path for cleaned output (None = overwrite)
            remove_duplicates: Remove duplicate files
            remove_empty: Remove empty directories
            optimize_textures: Optimize texture sizes (future feature)

        Returns:
            BuildResult with operation details
        """
        result = BuildResult(success=False)

        if not os.path.exists(pk3_path):
            result.errors.append(f"PK3 file not found: {pk3_path}")
            return result

        output_path = output_path or pk3_path
        temp_output = pk3_path + ".tmp"

        try:
            seen_files: Set[str] = set()
            cleaned_files: List[Tuple[str, bytes]] = []

            with zipfile.ZipFile(pk3_path, 'r') as pk3:
                for name in pk3.namelist():
                    # Skip empty directory entries
                    if remove_empty and name.endswith('/'):
                        continue

                    # Skip duplicates
                    normalized_name = name.lower()
                    if remove_duplicates and normalized_name in seen_files:
                        result.warnings.append(f"Removed duplicate: {name}")
                        continue

                    seen_files.add(normalized_name)
                    data = pk3.read(name)
                    cleaned_files.append((name, data))

            # Write cleaned PK3
            with zipfile.ZipFile(temp_output, 'w', zipfile.ZIP_DEFLATED) as pk3:
                for name, data in cleaned_files:
                    pk3.writestr(name, data)

            # Replace original
            if output_path == pk3_path:
                os.replace(temp_output, pk3_path)
            else:
                shutil.move(temp_output, output_path)

            result.success = True
            result.output_path = output_path
            result.files_included = len(cleaned_files)
            result.total_size = os.path.getsize(output_path)

        except Exception as e:
            result.errors.append(f"Clean failed: {str(e)}")
            if os.path.exists(temp_output):
                os.remove(temp_output)

        return result


# Convenience functions

def build_pk3(
    name: str,
    characters: Optional[List[str]] = None,
    sabers: Optional[List[str]] = None,
    output_dir: str = ".",
    search_paths: Optional[List[str]] = None,
    **kwargs
) -> BuildResult:
    """
    Convenience function to build a PK3.

    Args:
        name: Package name
        characters: List of .mbch file paths
        sabers: List of .sab file paths
        output_dir: Output directory
        search_paths: Asset search directories
        **kwargs: Additional PK3Structure fields

    Returns:
        BuildResult
    """
    structure = PK3Structure(
        name=name,
        characters=characters or [],
        sabers=sabers or [],
        **kwargs
    )

    builder = PK3Builder(search_paths=search_paths)
    output_path = os.path.join(output_dir, f"{name}.pk3")

    return builder.build(structure, output_path)


def validate_package(pk3_path: str) -> List[str]:
    """Validate a PK3 package"""
    builder = PK3Builder()
    return builder.validate_package(pk3_path)


def clean_package(pk3_path: str, output_path: Optional[str] = None) -> BuildResult:
    """Clean a PK3 package"""
    builder = PK3Builder()
    return builder.clean_package(pk3_path, output_path)
