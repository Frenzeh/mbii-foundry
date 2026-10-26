# MBII Foundry

A desktop editor for Movie Battles II text files, with visual forms, a live
Source panel, and asset lookup from your local game installation. Local editing
requires **no account or token**.

**[Download from Releases](https://github.com/Frenzeh/mbii-foundry/releases)**
· [User guide](USER_GUIDE.md) · [Build from source](go_module/README.md#run-from-source)

Foundry is **alpha software**. Review the proposed text before saving and test
edited content in your intended MBII engine version.

## Install

Choose a release with binaries and download the archive for your computer:

| Platform | Archive |
|---|---|
| macOS 13 or newer — Apple silicon and Intel | `mbii-foundry-macos-universal.zip` |
| Windows — x64 | `mbii-foundry-windows-amd64.zip` |
| Linux — x64 | `mbii-foundry-linux-amd64.tar.gz` |

Extract the archive. On macOS, open `MBII Foundry.app`. On Windows, run
`mbii-foundry.exe`; on Linux, run `./mbii-foundry` from the extracted folder.
Keep the bundled `data`, `definitions`, `schemas`, and `templates` folders
beside the Windows/Linux executable.

**macOS:** the app is ad-hoc signed and not notarized. That signature provides
no Apple publisher identity; Gatekeeper may require explicit approval on first
launch. See [release trust details](go_module/README.md#release-trust-boundary).

## Your first edit

1. **Choose local assets** in the first-run setup, or select **Continue without
   assets** to edit files without them:
   - **GameData** is your Jedi Academy folder containing both `base/` and
     `MBII/`. It supplies installed PK3 assets such as portraits, skins, and sounds.
   - **TextAssets** is an optional [`MBII/TextAssets`](https://github.com/MBII/TextAssets)
     checkout or loose-asset folder. Matching loose files override PK3 entries
     in Foundry's asset view; TextAssets alone usually lacks runtime images.
   You can change these machine-local paths later in `Edit -> Preferences`.
2. **Open a file** with `File -> Open File`, or start a character with
   `File -> New Character`.
3. **Edit the form** and watch the Source panel. For direct text edits, choose
   **Edit** in Source, then **Apply** once the draft parses successfully.
4. **Save and review.** `File -> Save` or `Save As` shows the current file beside
   the exact text to be written. Check that text, especially comments and unusual
   fields, before approving. Replacing an existing file requires a verified backup;
   a file changed after review is not overwritten.

For undo, recovery, asset diagnostics, and detailed saving behavior, see the
[user guide](USER_GUIDE.md). Recovery and backups require writable configuration
storage; restoring a recovery snapshot does not write the original file.

## What you can edit

| File | Editor |
|---|---|
| `.mbch` — characters | Identity, models, classes, weapons, force powers, attributes, and related character fields. |
| `.sab` — sabers | Selected saber fields, with visual blade controls for the first blade only. |
| `.veh` — vehicles | Selected identity, movement, armor, shield, and weapon fields. |
| `.siege` — siege definitions | Teams, rounds, classes, and related parsed fields. |
| `.mbtc` — team compositions | Separate `Tools -> Team Composer (.mbtc)` for one team with six class slots and per-class subclasses. |

The forms do **not** cover every engine field. SAB and VEH files can contain
multiple definitions; use the definition selector to switch between them.
Additional parsed fields remain available in Source. See the
[form limitations](USER_GUIDE.md#what-the-sab-and-veh-forms-cover).

Validation is an authoring aid, not proof that a file works in every MBII release.
`File -> Validate Folder` currently checks only `.mbch` parse errors and the
required `name`. Asset portraits are 2D images, not 3D model previews.

Foundry also offers [bulk editing](USER_GUIDE.md#11-bulk-edit-files) and
[modpack PK3/source-ZIP export](USER_GUIDE.md#10-track-and-export-modpacks).

## Updates and trust

Foundry checks GitHub releases for update notices. Automatic installation on
macOS/Linux requires an embedded publisher key and a valid signed update
manifest; missing or invalid trust data disables installation. Ordinary local
builds lack that key. Windows opens the release page instead.

Update verification authenticates Foundry release bytes, **not** Apple
notarization, Developer ID, Windows Authenticode, or operating-system trust.
See [update behavior and older-build upgrade instructions](USER_GUIDE.md#13-updates).

## Development

- [Build, run, and contribute](CONTRIBUTING.md)
- [Refresh embedded artwork](go_module/README.md#refresh-embedded-game-artwork)
- [Verify or refresh engine snapshots and enum catalogs](go_module/README.md#verify-engine-backed-metadata)
- [Configuration, credentials, and logs](USER_GUIDE.md#12-configuration-credentials-and-logs)

Apache License 2.0. See [LICENSE](LICENSE).
