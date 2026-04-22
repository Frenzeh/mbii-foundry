#!/usr/bin/env python3
"""
File Manager - Comprehensive file operations for MBCH character files

Provides:
- Save/Load .mbch files
- Export/Import JSON format
- Template management
- Recent files tracking
- Backup system
- Batch operations
"""

import os
import json
import shutil
from pathlib import Path
from datetime import datetime
from typing import Optional, List, Dict, Any, Tuple
from dataclasses import asdict

from .mbch_parser import MBCHParser, MBCHCharacter, WeaponInfo, ForceInfo


class MBCHFileManager:
    """Manages file operations for MBCH character files"""

    # Default paths relative to fa_creator directory
    DEFAULT_TEMPLATE_DIR = "templates"
    DEFAULT_OUTPUT_DIR = "output"
    DEFAULT_BACKUP_DIR = "backups"
    RECENT_FILES_PATH = "config/recent_files.json"
    MAX_RECENT_FILES = 20
    MAX_BACKUPS_PER_FILE = 5

    def __init__(self, base_path: Optional[str] = None):
        """
        Initialize file manager.

        Args:
            base_path: Base directory for FA Creator. If None, uses script directory.
        """
        if base_path:
            self.base_path = Path(base_path)
        else:
            self.base_path = Path(__file__).parent.parent

        self.parser = MBCHParser()
        self._ensure_directories()

    def _ensure_directories(self) -> None:
        """Create required directories if they don't exist"""
        for dir_name in [self.DEFAULT_TEMPLATE_DIR, self.DEFAULT_OUTPUT_DIR,
                         self.DEFAULT_BACKUP_DIR, "config"]:
            (self.base_path / dir_name).mkdir(parents=True, exist_ok=True)

    # ==================== LOAD OPERATIONS ====================

    def load_file(self, file_path: str) -> Tuple[MBCHCharacter, List[str]]:
        """
        Load an MBCH file.

        Args:
            file_path: Path to .mbch file

        Returns:
            Tuple of (MBCHCharacter, list of warnings/errors)
        """
        path = Path(file_path)
        if not path.exists():
            raise FileNotFoundError(f"File not found: {file_path}")

        if not path.suffix.lower() == '.mbch':
            raise ValueError(f"Invalid file type: {path.suffix}. Expected .mbch")

        char = self.parser.parse_file(str(path))
        warnings = self.parser.validate(char)

        # Add to recent files
        self._add_to_recent(str(path.absolute()))

        return char, warnings

    def load_template(self, template_name: str) -> MBCHCharacter:
        """
        Load a character template.

        Args:
            template_name: Name of template (without extension) or full path

        Returns:
            MBCHCharacter from template
        """
        # Check if it's a path or just a name
        if os.path.sep in template_name or template_name.endswith('.mbch'):
            template_path = Path(template_name)
        else:
            template_path = self.base_path / self.DEFAULT_TEMPLATE_DIR / f"{template_name}_template.mbch"

        if not template_path.exists():
            # Try without _template suffix
            template_path = self.base_path / self.DEFAULT_TEMPLATE_DIR / f"{template_name}.mbch"

        if not template_path.exists():
            raise FileNotFoundError(f"Template not found: {template_name}")

        char = self.parser.parse_file(str(template_path))
        # Reset name for new character
        char.name = f"New_{template_name.title()}"

        return char

    def load_json(self, file_path: str) -> MBCHCharacter:
        """
        Load character from JSON format.

        Args:
            file_path: Path to .json file

        Returns:
            MBCHCharacter from JSON
        """
        with open(file_path, 'r', encoding='utf-8') as f:
            data = json.load(f)

        return self.parser.from_dict(data)

    # ==================== SAVE OPERATIONS ====================

    def save_file(self, char: MBCHCharacter, file_path: str,
                  create_backup: bool = True,
                  validate: bool = True) -> Tuple[bool, List[str]]:
        """
        Save character to MBCH file.

        Args:
            char: Character to save
            file_path: Destination path
            create_backup: Whether to backup existing file
            validate: Whether to validate before saving

        Returns:
            Tuple of (success, list of warnings/errors)
        """
        path = Path(file_path)
        messages = []

        # Validate if requested
        if validate:
            errors = self.parser.validate(char)
            if errors:
                messages.extend([f"Warning: {e}" for e in errors])

        # Create backup if file exists
        if create_backup and path.exists():
            backup_path = self._create_backup(str(path))
            messages.append(f"Backup created: {backup_path}")

        # Ensure parent directory exists
        path.parent.mkdir(parents=True, exist_ok=True)

        # Write file
        try:
            self.parser.write_file(char, str(path))
            messages.append(f"Saved: {path}")
            self._add_to_recent(str(path.absolute()))
            return True, messages
        except Exception as e:
            messages.append(f"Error saving: {e}")
            return False, messages

    def save_as_template(self, char: MBCHCharacter, template_name: str) -> str:
        """
        Save character as a template.

        Args:
            char: Character to save as template
            template_name: Name for the template

        Returns:
            Path to saved template
        """
        # Sanitize template name
        safe_name = "".join(c for c in template_name if c.isalnum() or c in "._- ")
        template_path = self.base_path / self.DEFAULT_TEMPLATE_DIR / f"{safe_name}_template.mbch"

        self.parser.write_file(char, str(template_path))
        return str(template_path)

    def export_json(self, char: MBCHCharacter, file_path: str,
                    pretty: bool = True) -> str:
        """
        Export character to JSON format.

        Args:
            char: Character to export
            file_path: Destination path
            pretty: Whether to format with indentation

        Returns:
            Path to exported file
        """
        path = Path(file_path)
        if not path.suffix.lower() == '.json':
            path = path.with_suffix('.json')

        data = self.parser.to_dict(char)

        with open(path, 'w', encoding='utf-8') as f:
            if pretty:
                json.dump(data, f, indent=2)
            else:
                json.dump(data, f)

        return str(path)

    # ==================== BACKUP OPERATIONS ====================

    def _create_backup(self, file_path: str) -> str:
        """
        Create a backup of a file.

        Args:
            file_path: Path to file to backup

        Returns:
            Path to backup file
        """
        path = Path(file_path)
        if not path.exists():
            raise FileNotFoundError(f"Cannot backup: {file_path}")

        # Create backup filename with timestamp
        timestamp = datetime.now().strftime("%Y%m%d_%H%M%S")
        backup_name = f"{path.stem}_{timestamp}{path.suffix}"
        backup_path = self.base_path / self.DEFAULT_BACKUP_DIR / backup_name

        shutil.copy2(path, backup_path)

        # Clean old backups for this file
        self._cleanup_old_backups(path.stem)

        return str(backup_path)

    def _cleanup_old_backups(self, file_stem: str) -> None:
        """Remove old backups exceeding MAX_BACKUPS_PER_FILE"""
        backup_dir = self.base_path / self.DEFAULT_BACKUP_DIR
        pattern = f"{file_stem}_*.mbch"

        backups = sorted(backup_dir.glob(pattern), key=lambda p: p.stat().st_mtime)

        # Remove oldest backups if exceeding limit
        while len(backups) > self.MAX_BACKUPS_PER_FILE:
            oldest = backups.pop(0)
            oldest.unlink()

    def list_backups(self, file_name: Optional[str] = None) -> List[Dict[str, Any]]:
        """
        List available backups.

        Args:
            file_name: Optional filter by original filename

        Returns:
            List of backup info dicts
        """
        backup_dir = self.base_path / self.DEFAULT_BACKUP_DIR
        pattern = f"{file_name}_*.mbch" if file_name else "*.mbch"

        backups = []
        for backup_path in backup_dir.glob(pattern):
            stat = backup_path.stat()
            backups.append({
                'path': str(backup_path),
                'name': backup_path.name,
                'size': stat.st_size,
                'modified': datetime.fromtimestamp(stat.st_mtime).isoformat(),
                'original': backup_path.stem.rsplit('_', 2)[0]  # Remove timestamp
            })

        return sorted(backups, key=lambda x: x['modified'], reverse=True)

    def restore_backup(self, backup_path: str, destination: str) -> str:
        """
        Restore a backup file.

        Args:
            backup_path: Path to backup file
            destination: Where to restore to

        Returns:
            Path to restored file
        """
        if not Path(backup_path).exists():
            raise FileNotFoundError(f"Backup not found: {backup_path}")

        # Backup current if it exists
        if Path(destination).exists():
            self._create_backup(destination)

        shutil.copy2(backup_path, destination)
        return destination

    # ==================== RECENT FILES ====================

    def _get_recent_files_path(self) -> Path:
        """Get path to recent files config"""
        return self.base_path / self.RECENT_FILES_PATH

    def _add_to_recent(self, file_path: str) -> None:
        """Add a file to recent files list"""
        recent = self.get_recent_files()

        # Remove if already in list
        recent = [f for f in recent if f['path'] != file_path]

        # Add to front
        recent.insert(0, {
            'path': file_path,
            'name': Path(file_path).name,
            'accessed': datetime.now().isoformat()
        })

        # Trim to max
        recent = recent[:self.MAX_RECENT_FILES]

        # Save
        config_path = self._get_recent_files_path()
        config_path.parent.mkdir(parents=True, exist_ok=True)
        with open(config_path, 'w') as f:
            json.dump(recent, f, indent=2)

    def get_recent_files(self) -> List[Dict[str, str]]:
        """
        Get list of recently accessed files.

        Returns:
            List of recent file info dicts
        """
        config_path = self._get_recent_files_path()
        if not config_path.exists():
            return []

        try:
            with open(config_path, 'r') as f:
                recent = json.load(f)
            # Filter out files that no longer exist
            return [f for f in recent if Path(f['path']).exists()]
        except (json.JSONDecodeError, KeyError):
            return []

    def clear_recent_files(self) -> None:
        """Clear the recent files list"""
        config_path = self._get_recent_files_path()
        if config_path.exists():
            config_path.unlink()

    # ==================== TEMPLATE OPERATIONS ====================

    def list_templates(self) -> List[Dict[str, Any]]:
        """
        List available templates.

        Returns:
            List of template info dicts
        """
        template_dir = self.base_path / self.DEFAULT_TEMPLATE_DIR
        templates = []

        for template_path in template_dir.glob("*.mbch"):
            try:
                char = self.parser.parse_file(str(template_path))
                templates.append({
                    'path': str(template_path),
                    'name': template_path.stem.replace('_template', ''),
                    'filename': template_path.name,
                    'class': char.MBClass,
                    'description': char.description[:100] if char.description else ""
                })
            except Exception as e:
                templates.append({
                    'path': str(template_path),
                    'name': template_path.stem,
                    'filename': template_path.name,
                    'error': str(e)
                })

        return templates

    def delete_template(self, template_name: str) -> bool:
        """
        Delete a template.

        Args:
            template_name: Name of template to delete

        Returns:
            True if deleted
        """
        template_path = self.base_path / self.DEFAULT_TEMPLATE_DIR / f"{template_name}_template.mbch"
        if not template_path.exists():
            template_path = self.base_path / self.DEFAULT_TEMPLATE_DIR / f"{template_name}.mbch"

        if template_path.exists():
            # Backup before deleting
            self._create_backup(str(template_path))
            template_path.unlink()
            return True
        return False

    # ==================== BATCH OPERATIONS ====================

    def batch_validate(self, directory: str, recursive: bool = True) -> Dict[str, List[str]]:
        """
        Validate all MBCH files in a directory.

        Args:
            directory: Directory to scan
            recursive: Whether to search subdirectories

        Returns:
            Dict of {filepath: [errors]} for files with errors
        """
        path = Path(directory)
        pattern = "**/*.mbch" if recursive else "*.mbch"

        results = {}
        for mbch_file in path.glob(pattern):
            try:
                char = self.parser.parse_file(str(mbch_file))
                errors = self.parser.validate(char)
                if errors:
                    results[str(mbch_file)] = errors
            except Exception as e:
                results[str(mbch_file)] = [f"Parse error: {e}"]

        return results

    def batch_export_json(self, directory: str, output_dir: str,
                          recursive: bool = True) -> List[str]:
        """
        Export all MBCH files in a directory to JSON.

        Args:
            directory: Source directory
            output_dir: Output directory
            recursive: Whether to search subdirectories

        Returns:
            List of exported file paths
        """
        path = Path(directory)
        out_path = Path(output_dir)
        out_path.mkdir(parents=True, exist_ok=True)

        pattern = "**/*.mbch" if recursive else "*.mbch"
        exported = []

        for mbch_file in path.glob(pattern):
            try:
                char = self.parser.parse_file(str(mbch_file))
                json_path = out_path / f"{mbch_file.stem}.json"
                self.export_json(char, str(json_path))
                exported.append(str(json_path))
            except Exception as e:
                print(f"Error exporting {mbch_file}: {e}")

        return exported

    # ==================== UTILITY METHODS ====================

    def get_file_info(self, file_path: str) -> Dict[str, Any]:
        """
        Get detailed information about an MBCH file.

        Args:
            file_path: Path to file

        Returns:
            Dict with file info and character summary
        """
        path = Path(file_path)
        if not path.exists():
            raise FileNotFoundError(f"File not found: {file_path}")

        stat = path.stat()
        char = self.parser.parse_file(str(path))
        errors = self.parser.validate(char)

        return {
            'path': str(path.absolute()),
            'name': path.name,
            'size': stat.st_size,
            'modified': datetime.fromtimestamp(stat.st_mtime).isoformat(),
            'character': {
                'name': char.name,
                'class': char.MBClass,
                'model': char.model,
                'health': char.maxhealth,
                'armor': char.maxarmor,
                'weapons': char.weapons,
                'description': char.description[:200] if char.description else ""
            },
            'validation_errors': errors,
            'is_valid': len(errors) == 0,
            'weapon_overrides': len(char.weapon_infos),
            'force_overrides': len(char.force_infos),
            'extra_fields': len(char.extra_fields)
        }

    def compare_files(self, file1: str, file2: str) -> Dict[str, Any]:
        """
        Compare two MBCH files.

        Args:
            file1: Path to first file
            file2: Path to second file

        Returns:
            Dict with differences
        """
        char1 = self.parser.parse_file(file1)
        char2 = self.parser.parse_file(file2)

        dict1 = self.parser.to_dict(char1)
        dict2 = self.parser.to_dict(char2)

        differences = {
            'file1': file1,
            'file2': file2,
            'changes': []
        }

        def compare_dicts(d1, d2, prefix=""):
            for key in set(list(d1.keys()) + list(d2.keys())):
                k1 = d1.get(key)
                k2 = d2.get(key)
                full_key = f"{prefix}.{key}" if prefix else key

                if k1 != k2:
                    if isinstance(k1, dict) and isinstance(k2, dict):
                        compare_dicts(k1, k2, full_key)
                    else:
                        differences['changes'].append({
                            'field': full_key,
                            'file1_value': k1,
                            'file2_value': k2
                        })

        compare_dicts(dict1, dict2)
        return differences


# Module-level convenience functions
_default_manager: Optional[MBCHFileManager] = None


def get_manager(base_path: Optional[str] = None) -> MBCHFileManager:
    """Get or create the default file manager"""
    global _default_manager
    if _default_manager is None or base_path is not None:
        _default_manager = MBCHFileManager(base_path)
    return _default_manager


def load_file(file_path: str) -> Tuple[MBCHCharacter, List[str]]:
    """Load an MBCH file"""
    return get_manager().load_file(file_path)


def save_file(char: MBCHCharacter, file_path: str, **kwargs) -> Tuple[bool, List[str]]:
    """Save an MBCH file"""
    return get_manager().save_file(char, file_path, **kwargs)


def load_template(template_name: str) -> MBCHCharacter:
    """Load a template"""
    return get_manager().load_template(template_name)


def export_json(char: MBCHCharacter, file_path: str) -> str:
    """Export to JSON"""
    return get_manager().export_json(char, file_path)


def load_json(file_path: str) -> MBCHCharacter:
    """Load from JSON"""
    return get_manager().load_json(file_path)


if __name__ == "__main__":
    import sys

    if len(sys.argv) < 2:
        print("Usage: file_manager.py <command> [args]")
        print("Commands:")
        print("  info <file.mbch>           - Show file information")
        print("  validate <directory>       - Validate all files in directory")
        print("  templates                  - List available templates")
        print("  recent                     - Show recent files")
        print("  compare <file1> <file2>    - Compare two files")
        sys.exit(1)

    manager = MBCHFileManager()
    command = sys.argv[1]

    if command == "info" and len(sys.argv) > 2:
        info = manager.get_file_info(sys.argv[2])
        print(json.dumps(info, indent=2))

    elif command == "validate" and len(sys.argv) > 2:
        results = manager.batch_validate(sys.argv[2])
        if results:
            print("Validation errors found:")
            for path, errors in results.items():
                print(f"\n{path}:")
                for err in errors:
                    print(f"  - {err}")
        else:
            print("All files validated successfully!")

    elif command == "templates":
        templates = manager.list_templates()
        for t in templates:
            print(f"{t['name']}: {t.get('class', 'N/A')} - {t.get('description', '')[:50]}")

    elif command == "recent":
        recent = manager.get_recent_files()
        for r in recent:
            print(f"{r['name']}: {r['path']}")

    elif command == "compare" and len(sys.argv) > 3:
        diff = manager.compare_files(sys.argv[2], sys.argv[3])
        print(json.dumps(diff, indent=2))

    else:
        print(f"Unknown command or missing arguments: {command}")
        sys.exit(1)
