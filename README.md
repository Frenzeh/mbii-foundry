# MBII Foundry

[![CI](https://github.com/Frenzeh/mbii-foundry/actions/workflows/ci.yml/badge.svg)](https://github.com/Frenzeh/mbii-foundry/actions/workflows/ci.yml)
[![License: Apache 2.0](https://img.shields.io/badge/license-Apache%202.0-blue.svg)](LICENSE)
[![Go version](https://img.shields.io/github/go-mod/go-version/Frenzeh/mbii-foundry?filename=go_module%2Fgo.mod)](go_module/go.mod)
[![Release](https://img.shields.io/github/v/release/Frenzeh/mbii-foundry?include_prereleases&label=release)](https://github.com/Frenzeh/mbii-foundry/releases)

MBII Foundry is a standalone desktop editor for Movie Battles II text content.
It provides visual forms, a live source view, asset lookup, validation, and
packaging without requiring an account or an online service for local editing.

**Status:** alpha. Review the exact proposed output before saving and test
authored content in the intended MBII engine revision.

## Supported content

| File type | Current editor surface |
|---|---|
| `.mbch` | Character identity, models and portraits, classes, weapons, force powers, attributes, overrides, custom skills, point-buy, and developer fields. |
| `.sab` | Selected identity, type, first-blade, sound, combat, effect, flag, and animation fields. The form offers `SABER_SINGLE` and `SABER_STAFF`; other parsed keys remain available in Source. |
| `.veh` | Selected identity, movement, armor, shield, and weapon fields. The form offers `VH_SPEEDER`, `VH_ANIMAL`, `VH_WALKER`, and `VH_FIGHTER`; other parsed keys remain available in Source. |
| `.siege` | Siege teams, rounds, classes, and related parsed fields. |

SAB and VEH files may contain multiple top-level definitions. Foundry exposes a
definition selector and keeps sibling definitions in the document. The visual
forms do not claim to expose every engine field.

Other tools include:

- **Source panel:** live would-save text, an editable draft mode, parse
  diagnostics, Apply, Revert, Copy, and a pop-out mirror.
- **Undo/redo:** form edits, valid Source applies, JSON imports, templates, and
  definition switches participate in document history.
- **Save review:** Save and Save As show the on-disk file beside the exact
  candidate. A replacement requires a verified backup and checked publication;
  a file changed after review is not overwritten.
- **Crash recovery:** dirty `.mbch`, `.sab`, `.veh`, and `.siege` documents,
  including unapplied Source drafts, are snapshotted under the app configuration
  directory. Restoring a snapshot opens working state and never writes the
  original until you explicitly save.
- **Asset browser and diagnostics:** a merged view of installed PK3s and optional
  loose TextAssets. Character model/skin badges distinguish found, missing,
  unreadable, and fallback results and show the winning source.
- **Team Composer:** creates one siege team per `.mbtc` file, with six contiguous
  class slots and per-class subclass lists.
- **Modpacks:** tracks ordinary folders, scaffolds common MBII paths, previews an
  export manifest, builds PK3s, and exports a source ZIP. Removing a project from
  Foundry does not delete its folder.

Validation is a focused authoring aid, not proof that content is accepted by
every MBII release. `File -> Validate Folder` currently scans `.mbch` files for
parse failures and the engine-required `name`.

## Install

### Release archives

When a release provides binaries, download the matching asset from the
[Releases page](https://github.com/Frenzeh/mbii-foundry/releases):

- macOS: `mbii-foundry-macos-universal.zip` contains `MBII Foundry.app`
  with both arm64 and x86_64 executable slices.
- Windows: `mbii-foundry-windows-amd64.zip`.
- Linux: `mbii-foundry-linux-amd64.tar.gz`.

The release workflow does **not** perform Apple notarization. A macOS bundle is
Developer ID signed only when release credentials are configured; otherwise it
is ad-hoc signed and Gatekeeper may require explicit approval on first launch.
Do not treat an intact ad-hoc signature as publisher identity.

### Build from source

Install Go 1.24 or newer. Fyne also needs the platform dependencies documented
in [Fyne's setup guide](https://docs.fyne.io/started/).

```bash
git clone https://github.com/Frenzeh/mbii-foundry.git
cd mbii-foundry
./setup_mbii-foundry.sh
./run_mbii-foundry.sh
```

The helper scripts require Bash. On native Windows PowerShell:

```powershell
cd go_module
go build -ldflags="-H windowsgui" -o mbii-foundry.exe
.\mbii-foundry.exe
```

WSL builds a Linux application; it is not a substitute for a native Windows
build.

On macOS, `./build_app.sh` creates a universal
`dist/MBII Foundry.app`. It stages and verifies the complete bundle before an
atomic publish into `dist/`, and restores the prior bundle if publication fails.
The default local signature is ad-hoc. Setting `FOUNDRY_CODESIGN_IDENTITY`
selects a local signing identity but still does not notarize the bundle.

## Local setup: GameData and TextAssets

The first-run wizard is for **local asset roots**, not GitHub setup:

1. **GameData** is the Jedi Academy runtime directory containing both `base/`
   and `MBII/`. It supplies installed PK3 assets and is the recommended root for
   portraits, skins, sounds, shaders, and in-game testing.
2. **TextAssets** is an optional checkout or loose-asset tree. It is indexed
   after GameData, so matching loose files override installed PK3 entries in
   Foundry's merged view. A TextAssets checkout alone usually does not contain
   all runtime images.
3. You may continue without either root and edit local files. Configure paths
   later in `Edit -> Preferences`.

Paths are machine-local. Use the folder pickers or paste native paths, and
configure each computer separately; do not copy another machine's absolute
paths into shared project files.

### Local editing versus contributing

Opening, editing, validating, saving, recovering, and packaging local files
requires no GitHub account or token. The app does perform a cached GitHub release
check for update notices; `Help -> Check for Updates` forces a fresh check.

To edit official text assets and contribute them upstream, clone
[`MBII/TextAssets`](https://github.com/MBII/TextAssets), configure that checkout
as TextAssets, and use the repository's normal branch and pull-request workflow.
Foundry's optional contribution connection is separate from Local Setup.
GitHub tokens are stored in the operating system's native credential store, not
in `config.json`. A legacy plaintext token is retained until migration to native
storage can be written and read back successfully.

## Configuration, backups, and logs

Foundry uses the operating system's user configuration directory with an
`mbii-foundry` child:

- macOS: normally `~/Library/Application Support/mbii-foundry/`
- Windows: normally `%AppData%\mbii-foundry\`
- Linux: normally `$XDG_CONFIG_HOME/mbii-foundry/` or
  `~/.config/mbii-foundry/`

This directory holds preferences, recent files, favorites, project metadata,
update-check cache, backups, and crash-recovery snapshots. Existing
`mbii-fa-creator` configuration is copied to the new location on first use and
left in place as a safety copy. If configuration storage is unavailable, local
file editing remains available but preferences, favorites, backups, and recovery
cannot be persisted.

The log is `mbii-foundry.log` in the operating system temporary directory. The
in-app Debug Logs view redacts configured roots and known credentials before
display, but still review any diagnostic text before sharing it.

## Bulk editing, teams, and export

- The **Bulk Edit** activity supports parsed top-level keys in `.mbch`, `.sab`,
  `.veh`, `.siege`, and `.mbtc`; extension matching is case-insensitive.
  **Add File…** adds one supported file, while **Add Folder…** performs an
  uncapped recursive scan. Folder scans skip symlinks without following them,
  canonical paths are de-duplicated, and unsupported, duplicate, or symlinked
  paths plus read/parse failures remain visible by exact path in the scrollable
  load report. Newly added files are selected by default; use **Select All**,
  **Select None**, **Remove Selected**, or **Clear Batch** to adjust the in-app
  batch without deleting source files.
- Enter a case-insensitive field key and value, then run **Preview** and inspect
  every current-to-new row before **Apply to Selected**. Changing the key, value,
  or selection invalidates the preview. Apply refuses stale files, creates
  backups, and rolls back already-written files if a later write fails without
  overwriting newer external content.
- `Tools -> Team Composer (.mbtc)` edits one team: required `name`, optional
  `TimePeriod`, `EUAllowed`, `FriendlyShader`, six contiguous class slots, and
  per-class subclasses. `ClassesAllowed` is retained as a legacy field, not
  presented as current engine behavior. Class-reference checks use the merged
  VFS and are warnings when unavailable or incomplete.
- Modpack PK3/source exports omit hidden descendants and the output archive
  itself, reject symlinks and unsafe or duplicate archive paths, refuse empty
  output, and publish through an atomic writer. Preview Manifest shows the
  archive paths before a PK3 build.

## Updates and platform limits

Foundry checks the latest GitHub release in the background and caches the result
for six hours. Automatic installation is available only when the running build
contains a configured Ed25519 publisher key and the downloaded manifest binds
the requested version, platform, architecture, byte length, and SHA-256 digest.
Missing or invalid trust data disables auto-install rather than accepting an
unsigned payload.

- macOS and Linux have automatic installers with rollback behavior.
- Windows opens the release page instead of replacing the running executable;
  the project has no Authenticode publisher identity to verify.
- A verified update manifest is Foundry release integrity. It is not Apple
  notarization, Developer ID proof, Windows Authenticode, or an operating-system
  trust-store verdict.
- Ordinary local `go build` and the default `build_app.sh` invocation do not
  embed a publisher update key, so their auto-install path is disabled.

## Engine metadata verification

The committed engine snapshot records its source revision and file hashes. To
verify it against an exact local engine checkout:

```bash
cd go_module
MBII_ENGINE_SRC=/absolute/path/to/moviebattles \
  go test . -run '^TestExplicitEngineVerification$' -count=1
```

`MBII_ENGINE_SRC` must be a Git checkout whose relevant `bg_public.h` and
`bg_saga.c` bytes match its committed `HEAD`; the verifier checks the committed
snapshot rather than silently regenerating it. This is the engine-parity check
for snapshot-backed enum IDs and parser-key inventory, not a blanket
certification of every definition's prose or gameplay semantics.

## Repository layout

```text
mbii-foundry/
├── go_module/        Go/Fyne application and canonical parsers
├── data/             curated enum metadata
├── definitions/      per-enum and per-field prose
├── schemas/          validation schemas
├── templates/        starter content
├── tools/            audits and engine snapshot generator
├── macos/            app-bundle resources
└── packager/         Python PK3 packaging library
```

See [USER_GUIDE.md](USER_GUIDE.md) for the editing workflow and
[CONTRIBUTING.md](CONTRIBUTING.md) for contributor checks.

## License

Apache License 2.0. See [LICENSE](LICENSE).
