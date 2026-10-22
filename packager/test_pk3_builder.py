"""Synthetic stdlib regressions; run from the repo root with Python >= 3.7:

    python3 -m unittest packager.test_pk3_builder
"""

import json
import tempfile
import unittest
import zipfile
from pathlib import Path
from unittest.mock import patch

from packager import PK3Builder, PK3Structure, build_pk3, validate_package, validate_structure


class PK3BuilderTests(unittest.TestCase):
    ASSETS = {
        'characters': 'ext_data/mb2/character/example.mbch',
        'sabers': 'ext_data/sabers/example.sab',
        'siege_configs': 'ext_data/siege/example.siege',
        'team_configs': 'ext_data/mb2/teams/example.mbtc',
        'models': 'models/players/example/model.glm',
        'skins': 'models/players/example/model_default.skin',
        'textures': 'models/players/example/body.jpg',
        'sounds': 'sound/chars/example/misc/hello.wav',
        'icons': 'gfx/mb2/hud/example.png',
        'shaders': 'shaders/example.shader',
        'effects': 'effects/example/spark.efx',
        'other_files': 'notes.txt',
    }

    def setUp(self):
        self.temp = tempfile.TemporaryDirectory(prefix='foundry_packager_test_')
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name)
        self.output = self.root / 'export.pk3'

    def asset(self, relative_path, data=b'example asset'):
        path = self.root / 'assets' / relative_path
        path.parent.mkdir(parents=True, exist_ok=True)
        path.write_bytes(data)
        return str(path)

    def test_each_populated_asset_list_exports_bytes_and_truthful_manifest(self):
        for field, archive_path in self.ASSETS.items():
            with self.subTest(field=field):
                data = (field + ' contents').encode('ascii')
                source = self.asset(archive_path, data)
                structure = PK3Structure(name='example', **{field: [source]})
                self.assertEqual(validate_structure(structure), [])
                builder = PK3Builder([str(self.root / 'assets')])
                result = builder.build(structure, str(self.output), resolve_dependencies=False)
                self.assertTrue(result.success, result.errors)
                self.assertEqual(result.files_included, 2)
                self.assertEqual(result.output_path, str(self.output))
                self.assertEqual(result.total_size, self.output.stat().st_size)
                self.assertIsNone(builder.temp_dir)
                with zipfile.ZipFile(self.output) as archive:
                    self.assertEqual(set(archive.namelist()), {archive_path, 'manifest.json'})
                    self.assertEqual(archive.read(archive_path), data)
                    manifest = json.loads(archive.read('manifest.json'))
                    self.assertEqual(manifest['total_files'], 1)
                    self.assertEqual(manifest['total_size'], len(data))
                self.assertEqual(validate_package(str(self.output)), [])

    def test_all_asset_lists_combine_without_metadata(self):
        sources = {field: [self.asset(path)] for field, path in self.ASSETS.items()}
        result = PK3Builder([str(self.root / 'assets')]).build(
            PK3Structure(name='example', **sources), str(self.output),
            include_manifest=False, resolve_dependencies=False,
        )
        self.assertTrue(result.success, result.errors)
        with zipfile.ZipFile(self.output) as archive:
            self.assertEqual(set(archive.namelist()), set(self.ASSETS.values()))
        self.assertEqual(result.files_included, len(self.ASSETS))

    def test_search_roots_and_convenience_api_preserve_nested_paths(self):
        icon = 'gfx/mb2/hud/team/example.png'
        sound = 'sound/chars/example/misc/hello.mp3'
        self.asset(icon, b'icon')
        self.asset(sound, b'sound')
        result = build_pk3(
            'example', output_dir=str(self.root),
            search_paths=[str(self.root / 'assets')], icons=[icon], sounds=[sound],
        )
        self.assertTrue(result.success, result.errors)
        with zipfile.ZipFile(result.output_path) as archive:
            self.assertEqual(archive.read(icon), b'icon')
            self.assertEqual(archive.read(sound), b'sound')

    def test_model_and_sound_directories_keep_contents_and_prune_hidden_files(self):
        visible = {
            'models/players/example/model.glm': b'model',
            'models/players/example/model_default.skin': b'skin',
            'models/players/example/textures/body.tga': b'texture',
            'models/players/example/animation.cfg': b'config',
            'sound/chars/example/misc/hello.wav': b'sound',
        }
        for name, data in visible.items():
            self.asset(name, data)
        self.asset('models/players/example/.metadata', b'local metadata')
        self.asset('models/players/example/.cache/preview.png', b'local preview')
        structure = PK3Structure(
            name='example', models=['models/players/example'], sounds=['sound/chars/example'],
        )
        result = PK3Builder([str(self.root / 'assets')]).build(structure, str(self.output))
        self.assertTrue(result.success, result.errors)
        with zipfile.ZipFile(self.output) as archive:
            self.assertEqual(set(archive.namelist()), set(visible) | {'manifest.json'})
            for name, data in visible.items():
                self.assertEqual(archive.read(name), data)

    def test_character_and_saber_dependencies_merge_with_explicit_assets(self):
        character = self.asset(
            'ext_data/mb2/character/example.mbch',
            b'model "example"\nskin "default"\nuishader "gfx/mb2/hud/example"\nsoundset "example"\n',
        )
        saber = self.asset('ext_data/sabers/example.sab', b'saberModel "models/weapons/example/model.glm"\n')
        assets = {
            'models/players/example/model.glm': b'model',
            'models/players/example/model_default.skin': b'skin',
            'models/players/example/body.tga': b'texture',
            'models/weapons/example/model.glm': b'saber model',
            'sound/chars/example/hello.wav': b'sound',
            'gfx/mb2/hud/example.png': b'icon',
        }
        for name, data in assets.items():
            self.asset(name, data)
        structure = PK3Structure(
            name='example', characters=[character], sabers=[saber],
            models=['models/players/example'], icons=['gfx/mb2/hud/example.png'],
        )
        result = PK3Builder([str(self.root / 'assets')]).build(structure, str(self.output))
        self.assertTrue(result.success, result.errors)
        self.assertEqual(result.missing_assets, [])
        with zipfile.ZipFile(self.output) as archive:
            expected = set(assets) | {
                'ext_data/mb2/character/example.mbch', 'ext_data/sabers/example.sab', 'manifest.json',
            }
            self.assertEqual(set(archive.namelist()), expected)
            self.assertEqual(len(archive.namelist()), len(expected))
            for name, data in assets.items():
                self.assertEqual(archive.read(name), data)

    def test_missing_requested_assets_fail_without_replacing_existing_export(self):
        for field, archive_path in self.ASSETS.items():
            with self.subTest(field=field):
                self.output.write_bytes(b'previous export')
                missing = str(self.root / 'missing' / archive_path)
                structure = PK3Structure(name='example', **{field: [missing]})
                builder = PK3Builder()
                result = builder.build(structure, str(self.output), resolve_dependencies=False)
                self.assertFalse(result.success)
                self.assertTrue(any(missing in error for error in result.errors), result.errors)
                self.assertEqual(result.files_included, 0)
                self.assertEqual(result.output_path, '')
                self.assertEqual(self.output.read_bytes(), b'previous export')
                self.assertIsNone(builder.temp_dir)

    def test_one_missing_asset_does_not_publish_partial_export(self):
        icon = self.asset('gfx/mb2/hud/example.png')
        result = PK3Builder().build(
            PK3Structure(name='example', icons=[icon], effects=[str(self.root / 'missing.efx')]),
            str(self.output),
        )
        self.assertFalse(result.success)
        self.assertTrue(result.errors)
        self.assertFalse(self.output.exists())

    def test_missing_required_dependency_is_reported_not_success(self):
        character = self.asset('ext_data/mb2/character/example.mbch', b'model "missing_example"\n')
        builder = PK3Builder([str(self.root / 'assets')])
        result = builder.build(PK3Structure(name='example', characters=[character]), str(self.output))
        self.assertFalse(result.success)
        self.assertTrue(any('models/players/missing_example' in asset for asset in result.missing_assets))
        self.assertTrue(result.errors)
        self.assertFalse(self.output.exists())
        self.assertIsNone(builder.temp_dir)

    def test_empty_or_filtered_directory_never_exports_only_manifest(self):
        directory = self.root / 'assets' / 'models' / 'players' / 'empty'
        directory.mkdir(parents=True)
        for hidden in (False, True):
            with self.subTest(hidden=hidden):
                if hidden:
                    (directory / '.metadata').write_bytes(b'local metadata')
                builder = PK3Builder()
                result = builder.build(PK3Structure(name='example', models=[str(directory)]), str(self.output))
                self.assertFalse(result.success)
                self.assertTrue(result.errors)
                self.assertFalse(self.output.exists())
                self.assertIsNone(builder.temp_dir)
        result = PK3Builder().build(PK3Structure(name='example'), str(self.output), validate=False)
        self.assertFalse(result.success)
        self.assertFalse(self.output.exists())

    def test_loose_typed_assets_use_model_soundset_and_icon_locations(self):
        sources = {
            'skins': [self.asset('loose/player/model_default.skin')],
            'textures': [self.asset('loose/player/body.tga')],
            'sounds': [self.asset('loose/voice/hello.wav')],
            'icons': [self.asset('loose/icons/portrait.png')],
        }
        result = PK3Builder().build(PK3Structure(name='example', **sources), str(self.output))
        self.assertTrue(result.success, result.errors)
        with zipfile.ZipFile(self.output) as archive:
            self.assertEqual(set(archive.namelist()), {
                'models/players/player/model_default.skin', 'models/players/player/body.tga',
                'sound/chars/voice/hello.wav', 'gfx/mb2/hud/portrait.png', 'manifest.json',
            })

    def test_archive_paths_ignore_same_name_ancestors_outside_asset_root(self):
        project = self.root / 'work' / 'models' / 'project'
        shader = project / 'shaders' / 'custom.shader'
        texture = self.root / 'scripts' / 'textures' / 'player' / 'body.tga'
        loose_shader = self.root / 'models' / 'loose' / 'other.shader'
        notes = self.root / 'gfx' / 'notes.txt'
        for path in (shader, texture, loose_shader, notes):
            path.parent.mkdir(parents=True, exist_ok=True)
            path.write_bytes(path.name.encode('ascii'))
        structure = PK3Structure(
            name='example', other_files=[str(shader), str(notes)],
            textures=[str(texture)], shaders=[str(loose_shader)],
        )
        result = PK3Builder([str(project)]).build(structure, str(self.output))
        self.assertTrue(result.success, result.errors)
        with zipfile.ZipFile(self.output) as archive:
            self.assertEqual(set(archive.namelist()), {
                'shaders/custom.shader', 'models/players/player/body.tga',
                'shaders/other.shader', 'notes.txt', 'manifest.json',
            })
            self.assertEqual(archive.read('shaders/custom.shader'), b'custom.shader')

    def test_conflicting_loose_assets_fail_instead_of_choosing_arbitrary_winner(self):
        first = self.asset('first/example.shader', b'first shader')
        second = self.asset('second/example.shader', b'second shader')
        result = PK3Builder().build(
            PK3Structure(name='example', shaders=[first, second]), str(self.output),
        )
        self.assertFalse(result.success)
        self.assertTrue(any('Conflicting assets' in error for error in result.errors))
        self.assertFalse(self.output.exists())

    def test_write_failure_cleans_staging_and_preserves_previous_export(self):
        icon = self.asset('gfx/mb2/hud/example.png')
        self.output.write_bytes(b'previous export')
        builder = PK3Builder()
        with patch.object(zipfile.ZipFile, 'write', side_effect=OSError('ordinary write failure')):
            result = builder.build(PK3Structure(name='example', icons=[icon]), str(self.output))
        self.assertFalse(result.success)
        self.assertTrue(any('ordinary write failure' in error for error in result.errors))
        self.assertEqual(self.output.read_bytes(), b'previous export')
        self.assertIsNone(builder.temp_dir)
        self.assertEqual(list(self.root.glob('.fa_pk3_*')), [])


if __name__ == '__main__':
    unittest.main()
