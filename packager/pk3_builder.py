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

        All populated asset lists are required inputs. Missing explicit assets or
        required resolved dependencies fail the build without replacing an existing
        output. Requested game-relative paths and files under configured search
        roots keep their root-relative paths. Other loose files use typed
        locations: models/skins/textures use their model directory and sounds
        their soundset directory; ancestor directory names are not interpreted.

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

        try:
            # Stage generated metadata only after structure validation.
            self.temp_dir = tempfile.mkdtemp(prefix="fa_pk3_")
            files_to_include: List[Tuple[str, str]] = []  # (source, archive_path)

            # Every populated typed list participates in the same build manifest.
            for asset_type, paths in structure.get_all_assets().items():
                for asset_path in paths:
                    files_to_include.extend(self._gather_asset_files(asset_path, asset_type))

                    if resolve_dependencies and asset_type in (AssetType.CHARACTER, AssetType.SABER):
                        source_path = self._resolve_source(asset_path)
                        if asset_type == AssetType.CHARACTER:
                            refs = self.resolver.resolve_mbch_assets(source_path)
                        else:
                            refs = self.resolver.resolve_sab_assets(source_path)
                        dep_files, missing = self._process_dependencies(refs)
                        files_to_include.extend(dep_files)
                        result.missing_assets.extend(missing)

            for file_path in structure.other_files:
                files_to_include.extend(self._gather_asset_files(file_path))

            if result.missing_assets:
                result.errors.extend(f"Required asset not found: {asset}" for asset in result.missing_assets)
                return result

            files_to_include = self._validate_files(files_to_include, output_path, include_manifest)
            if not files_to_include:
                result.errors.append("No asset files to include")
                return result

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
                try:
                    shutil.rmtree(self.temp_dir)
                    self.temp_dir = None
                except OSError as error:
                    result.success = False
                    result.errors.append(f"Cleanup failed for {self.temp_dir}: {error}")

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
                if os.path.isdir(ref.resolved_path):
                    files.extend(self._gather_directory(ref.resolved_path, ref.path))
                elif os.path.isfile(ref.resolved_path):
                    files.append((ref.resolved_path, ref.path))
                else:
                    raise FileNotFoundError(f"Resolved asset not found: {ref.resolved_path}")
            elif ref.required:
                missing.append(f"{ref.asset_type}: {ref.name} ({ref.path})")

        return files, missing

    def _resolve_source(self, asset_path: str) -> str:
        """Accept explicit local paths or game-relative paths in search roots."""
        if os.path.exists(asset_path):
            return os.path.abspath(asset_path)
        resolved = self.resolver.find_asset(asset_path)
        if resolved:
            return os.path.abspath(resolved)
        raise FileNotFoundError(f"Asset not found: {asset_path}")

    def _gather_asset_files(
        self, asset_path: str, asset_type: Optional[AssetType] = None
    ) -> List[Tuple[str, str]]:
        source = self._resolve_source(asset_path)
        definitions = (AssetType.CHARACTER, AssetType.SABER, AssetType.SIEGE, AssetType.TEAM_CONFIG)
        archive_path = None
        if (asset_type or get_file_asset_type(source)) not in definitions:
            archive_path = self._root_relative_path(asset_path, source)
        if archive_path is None:
            archive_path = self._determine_archive_path(source, asset_type)
        if os.path.isdir(source):
            return self._gather_directory(source, archive_path)
        return [(source, archive_path)]

    def _gather_directory(self, source: str, archive_path: str) -> List[Tuple[str, str]]:
        """Keep directory contents, pruning hidden descendants like the desktop exporter."""
        self._check_source(source)
        files = []
        for root, dirs, filenames in os.walk(source, onerror=self._raise_walk_error):
            dirs[:] = sorted(d for d in dirs if not d.startswith('.'))
            for directory in dirs:
                self._check_source(os.path.join(root, directory))
            for filename in sorted(filenames):
                if filename.startswith('.'):
                    continue
                full_path = os.path.join(root, filename)
                rel_path = os.path.relpath(full_path, source).replace(os.sep, '/')
                files.append((full_path, f"{archive_path}/{rel_path}"))
        if not files:
            raise ValueError(f"No asset files found in directory: {source}")
        return files

    @staticmethod
    def _raise_walk_error(error: OSError) -> None:
        raise error

    def _root_relative_path(self, requested: str, source: str) -> Optional[str]:
        """Return the game-relative path from the requested value or matching search root."""
        if not os.path.isabs(requested) and not os.path.exists(requested):
            return Path(os.path.normpath(requested)).as_posix()
        real_source = os.path.realpath(source)
        matches = []
        for search_path in self.search_paths:
            root = os.path.realpath(search_path)
            same_drive = os.path.splitdrive(root)[0].lower() == os.path.splitdrive(real_source)[0].lower()
            if same_drive and real_source != root and os.path.commonpath((root, real_source)) == root:
                matches.append(os.path.relpath(real_source, root))
        # Nested roots: the deepest configured root is the most specific asset root.
        return Path(min(matches, key=len)).as_posix() if matches else None

    @staticmethod
    def _check_source(source: str) -> None:
        # Check selected entries, not ancestors outside the selected asset tree
        # (ordinary platform temp roots may themselves be symlink aliases).
        if Path(source).is_symlink():
            raise ValueError(f"Symlink not allowed: {source}")

    def _gather_model_files(self, model_path: str) -> List[Tuple[str, str]]:
        """Gather a model directory or an explicitly selected model file."""
        return self._gather_asset_files(model_path, AssetType.MODEL)

    def _determine_archive_path(
        self, file_path: str, asset_type: Optional[AssetType] = None
    ) -> str:
        """Infer a typed location for a loose file outside every configured search root."""
        asset_type = asset_type or get_file_asset_type(file_path)
        if asset_type in (AssetType.CHARACTER, AssetType.SABER, AssetType.SIEGE, AssetType.TEAM_CONFIG):
            return f"{MBII_PATHS[asset_type]}/{os.path.basename(file_path)}"

        filename = os.path.basename(file_path)
        parent = os.path.basename(os.path.dirname(file_path))
        if os.path.isdir(file_path) and asset_type in (AssetType.MODEL, AssetType.SKIN, AssetType.TEXTURE):
            return get_asset_path(AssetType.SKIN, model=filename)
        if os.path.isdir(file_path) and asset_type == AssetType.SOUND:
            return get_asset_path(asset_type, soundset=filename)
        if asset_type in (AssetType.MODEL, AssetType.SKIN, AssetType.TEXTURE):
            if not parent:
                raise ValueError(f"Cannot infer model directory for asset: {file_path}")
            return f"{get_asset_path(AssetType.SKIN, model=parent)}/{filename}"
        if asset_type == AssetType.SOUND:
            if not parent:
                raise ValueError(f"Cannot infer soundset directory for asset: {file_path}")
            return f"{get_asset_path(asset_type, soundset=parent)}/{filename}"
        if asset_type:
            return f"{MBII_PATHS[asset_type]}/{filename}"
        return filename

    def _validate_files(
        self, files: List[Tuple[str, str]], output_path: str, include_manifest: bool
    ) -> List[Tuple[str, str]]:
        """Validate the complete source/archive mapping before writing any output."""
        entries: Dict[str, Tuple[str, str]] = {}
        for source, archive_path in files:
            self._check_source(source)
            if not os.path.isfile(source):
                raise FileNotFoundError(f"Asset is not a regular file: {source}")
            if os.path.realpath(source) == os.path.realpath(output_path) or (
                os.path.exists(output_path) and os.path.samefile(source, output_path)
            ):
                raise ValueError(f"Cannot include output file as an asset: {source}")
            archive_path = archive_path.replace('\\', '/')
            parts = archive_path.split('/')
            if (
                any(part in ('', '.', '..') or part.startswith('.') for part in parts)
                or any(':' in part for part in parts)
                or any(ord(char) < 32 or ord(char) == 127 for char in archive_path)
            ):
                raise ValueError(f"Invalid archive path: {archive_path}")
            key = archive_path.casefold()
            if include_manifest and key == 'manifest.json':
                raise ValueError("Asset conflicts with generated manifest.json")
            if key in entries:
                previous_source, previous_path = entries[key]
                if previous_path != archive_path or not os.path.samefile(previous_source, source):
                    raise ValueError(f"Conflicting assets for archive path: {archive_path}")
            else:
                entries[key] = (source, archive_path)
        return sorted(entries.values(), key=lambda entry: entry[1])

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

        # Publish only a fully written archive, preserving an existing export on failure.
        descriptor, staged_path = tempfile.mkstemp(
            prefix=".fa_pk3_", suffix=".tmp", dir=os.path.dirname(output_path) or '.'
        )
        try:
            with os.fdopen(descriptor, 'wb') as output:
                with zipfile.ZipFile(output, 'w', zipfile.ZIP_DEFLATED) as pk3:
                    for source_path, archive_path in files:
                        self._check_source(source_path)
                        if not os.path.isfile(source_path):
                            raise FileNotFoundError(f"Asset is not a regular file: {source_path}")
                        pk3.write(source_path, archive_path)
            os.replace(staged_path, output_path)
        finally:
            if os.path.exists(staged_path):
                os.remove(staged_path)

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
