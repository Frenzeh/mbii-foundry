# MBII Foundry Go module

This directory contains the Go 1.24/Fyne desktop application and its canonical
parsers. User installation and workflows are documented in
[`../README.md`](../README.md) and [`../USER_GUIDE.md`](../USER_GUIDE.md).

## Run from source

Install Go 1.24 or newer and the platform prerequisites from
[Fyne's setup guide](https://docs.fyne.io/started/), then:

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

Run from this directory so development builds can discover the repository's
`data/`, `definitions/`, `schemas/`, and `templates/` resources. Generated
executables are ignored build artifacts and must not be committed.

For a macOS application bundle, run `../build_app.sh` from either the repository
root or this directory's parent. It creates
`../dist/MBII Foundry.app`, verifies both arm64 and x86_64 slices, stages the
complete bundle on the destination filesystem, and atomically publishes it.
The default signature is ad-hoc and the script does not notarize the bundle.

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

The checkout must be a Git repository whose relevant `bg_public.h` and
`bg_saga.c` bytes match its committed `HEAD`. The generator accepts those files
at the checkout root, under `game/`, or under `codemp/game/`. A mismatch fails
instead of rewriting committed metadata.

To intentionally generate a candidate snapshot for review:

```bash
MBII_ENGINE_SRC=/absolute/path/to/moviebattles \
  go run ../tools/generate_snapshot.go \
  -snapshot testdata/engine_snapshot.json \
  -schema ../schemas/mbch_schema.json
```

Generation records the engine revision, source hashes, enum ranges, enum
numeric values, and parser-key inventory. It does not copy C source text and
does not certify definition prose or gameplay behavior.

## Release trust boundary

The release workflow requires matching Ed25519 publisher secrets and embeds the
public key into each release binary. A normal local `go build` leaves
`PublisherTrustedKey` empty, so automatic installation fails closed.

Manifest verification authenticates Foundry's release payload fields and bytes.
It does not provide Apple notarization, Apple Developer ID status, Windows
Authenticode identity, or an operating-system trust decision. Windows currently
opens the release page instead of replacing the running executable.
