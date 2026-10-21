# MBII Foundry Go module

This directory contains the Go 1.24/Fyne desktop application and its canonical
parsers. User installation and workflows are documented in
[`../README.md`](../README.md) and [`../USER_GUIDE.md`](../USER_GUIDE.md).

## Run from source

Install Go 1.24 or newer and the platform prerequisites from
[Fyne's setup guide](https://docs.fyne.io/started/). The repository's Bash
helpers build and launch from the correct directory:

```bash
git clone https://github.com/Frenzeh/mbii-foundry.git
cd mbii-foundry
./setup_mbii-foundry.sh
./run_mbii-foundry.sh
```

For the direct Go commands below, work from `go_module/`:

```bash
go run .
```

To create a repository-local executable:

```bash
go build -o mbii-foundry
./mbii-foundry
```

On native Windows:

```powershell
go build -ldflags="-H windowsgui" -o mbii-foundry.exe
.\mbii-foundry.exe
```

WSL builds a Linux application, not a native Windows executable.

Run from this directory so development builds can discover the repository's
`data/`, `definitions/`, `schemas/`, and `templates/` resources. Generated
executables are ignored build artifacts and must not be committed.

For a macOS application bundle, run `./build_app.sh` from the repository root
(or `../build_app.sh` from this directory). It creates `dist/MBII Foundry.app`
under the repository root, verifies both arm64 and x86_64 slices, stages the
complete bundle on the destination filesystem, and atomically publishes it,
restoring the prior bundle if publication fails. The default signature is
ad-hoc. Setting `FOUNDRY_CODESIGN_IDENTITY` selects a local signing identity;
the script does not notarize the bundle.

## Refresh embedded game artwork

Use the current installed public PK3s, with cloud placeholders downloaded first.
Stage extraction outside the working tree, from the repository root:

```bash
go -C tools/extract-icons run . \
  -pk3-dir "<GameData>/MBII" \
  -out "<scratch>/foundry-icons"
```

Use a fresh output directory. Extraction follows the engine's case-insensitive
archive ordering and fails on unreadable archives or selected images. Review the
output and merge its category directories into `go_module/assets/icons/`, backing
up changed files first. Do not replace the whole directory: base-game artwork
not present in the mod PK3s is still needed. New catalog aliases must follow
actual engine or menu bindings, not similarly named substitute pictures.

Rebuild afterward; these images are embedded, not loaded from the source tree.
From the repository root:

```bash
go -C go_module build -o mbii-foundry .
```

## Architecture

- `main.go` owns application lifecycle, menus, preferences, tabs, and editor
  routing.
- `parsers/` is the canonical parser and source-preserving AST implementation
  for MBCH, SAB, VEH, SIEGE, and MBTC content.
- `session_state.go`, `source_panel.go`, `save_review.go`, and
  `crash_recovery.go` implement document history, retained Source drafts,
  reviewed saving, backups, and recovery.
- `asset_browser.go`, `vfs.go`, and the resolver files provide a merged view of
  GameData PK3s and optional loose TextAssets.
- `bulk_editor.go`, `mbtc_composer.go`, `modpack_manager.go`, and
  `archive_export.go` contain batch, team, project, PK3, and source-export
  workflows.
- `safeio/` supplies atomic file publication.
- `updatemanifest/`, `cmd/signer/`, `updater.go`, and the platform installer
  files implement signed update manifests and platform-specific installation.

The application uses `os.UserConfigDir()/mbii-foundry`, not a configuration file
beside the executable. GitHub tokens use the OS native credential store and are
excluded from `config.json`.

## Focused checks

Run commands from this directory:

```bash
go test ./...
go vet ./...
go build ./...
```

The repository CI also checks formatting and builds its supported release
targets.

## Verify engine-backed metadata

The committed `testdata/engine_snapshot.json` is derived from an exact external
MBII engine revision. Verify it without modifying the snapshot:

```bash
MBII_ENGINE_SRC=/absolute/path/to/moviebattles \
  go test . -run '^TestExplicitEngineVerification$' -count=1
```

The checkout must be a Git repository whose `mb_defines.h`, `bg_public.h`,
`bg_weapons.h`, `q_shared.h`, `bg_saga.h`, and `bg_saga.c` bytes match its committed
`HEAD`. The generator accepts these at the checkout root, under `game/`, or under
`codemp/game/`. A native `cc` C preprocessor must be available. Missing inputs,
unsupported expressions, and source drift fail instead of guessing values.

To intentionally generate a candidate snapshot for review:

```bash
MBII_ENGINE_SRC=/absolute/path/to/moviebattles \
  go run ../tools/generate_snapshot.go \
  -snapshot testdata/engine_snapshot.json \
  -schema ../schemas/mbch_schema.json \
  -update-schemas
```

`-update-schemas` also regenerates the eight enum catalogs and the main schema's
`MBClass` constraint; omit it to generate only the snapshot. Generation records
the engine revision, six source hashes, source-default preprocessor configuration,
enum ranges and values, and parser-key inventory. Ten enum domains cover classes,
attributes, weapons, force powers, saber styles, holdables, special actions,
powerups, class flags, and held flags.

Conditional declarations use `mb_defines.h` defaults, without external includes
or deployed build flags. The snapshot does not prove a server's enabled classes,
parser acceptance of every catalog value, or gameplay behavior, and does not
certify definition prose. It contains metadata, not copied engine source.

## Release trust boundary

The release workflow requires matching Ed25519 publisher secrets and embeds the
public key into each release binary. A normal local `go build` leaves
`PublisherTrustedKey` empty, so automatic installation fails closed.

Manifest verification authenticates Foundry's release payload fields and bytes.
It does not provide Apple notarization, Apple Developer ID status, Windows
Authenticode identity, or an operating-system trust decision. Windows currently
opens the release page instead of replacing the running executable.
