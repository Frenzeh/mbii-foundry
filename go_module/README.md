# MBII Foundry Go module

This directory contains the Go 1.27.3/Fyne 2.8.2 desktop application and its canonical
parsers. User installation and workflows are documented in
[`../README.md`](../README.md) and [`../USER_GUIDE.md`](../USER_GUIDE.md).

## Run from source

Install Go 1.27.3 or newer and the platform prerequisites from
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
restoring the prior bundle if publication fails. Both slices and the bundle
metadata target macOS 13.0. This target is not proof of a runtime test on macOS
13. The default signature is ad-hoc. Setting `FOUNDRY_CODESIGN_IDENTITY` selects
a local signing identity; the script does not notarize the bundle.

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

The desktop components above are mostly files in one Go `main` package.
MBCH/SAB/VEH/SIEGE are document editors; MBTC is a composer dialog. Document and
Source pop-outs share application-managed session/draft state, not independent
copies of a saved document. Revert must refresh both Source surfaces, and
history restoration must accept fresh unnamed defaults without weakening Source
Apply validation. Custom skill arrays and `RankAttributes` are canonical MBCH
owners, not duplicate `ExtraFields` projections. Numeric form binding must not
round unrelated values. Configured-cost and EHP previews do not establish engine
mechanics. See [`../docs/ARCHITECTURE.md`](../docs/ARCHITECTURE.md) for ownership
and persistence boundaries.

The application uses `os.UserConfigDir()/mbii-foundry`, not a configuration file
beside the executable. GitHub tokens use the OS native credential store and are
excluded from newly persisted `config.json`; failed secure migration preserves
the active legacy plaintext. Explicit **Use saved credential** Connect can clean
an eligible regular retained pre-rebrand backup after verified native access.
Recorded migration provenance allows cleanup after token replacement; older
markerless backups require a matching active legacy or verified token.
Differing/unverifiable backups stay unchanged, with known-plaintext warnings;
backup tokens are never imported into the native store.

Catalog refresh writes one atomic
`os.UserCacheDir()/mbii-foundry/catalog.json`, never bundled resources or a cache
`data/` directory. Missing/invalid cache falls back to installed/embedded data.
The bounded HTTPS upstream-main fetch disallows redirects and has neither
application-update-manifest authentication nor an independent catalog signature.
Shader precedence is deterministic Foundry still-image preview policy, not
verified engine/profile equivalence.

Contribution clone/push accepts the connected account's expected HTTPS
`TextAssets` fork. Upstream pulls are pinned to
`https://github.com/MovieBattles/TextAssets.git` and credential-free. The PR
checkbox controls whether submission requests a PR; push-only is supported,
and PR failure after a push is reported as a partial outcome.

The supported root `packager/` Python API is separate from desktop export and
the Go parser/session implementation; see its exported API in
`../packager/__init__.py`.

## Focused checks

Run commands with isolated preferences, caches, temporary files, and no live
asset trees. This macOS/Linux shell example explicitly uses `/tmp`, outside the
checkout; confirm the chosen scratch root is nonsynced on your machine:

```bash
scratch="$(mktemp -d /tmp/foundry-checks.XXXXXX)"
mkdir -p "$scratch/home" "$scratch/tmp" "$scratch/config" "$scratch/cache"
export HOME="$scratch/home" TMPDIR="$scratch/tmp"
export XDG_CONFIG_HOME="$scratch/config" XDG_CACHE_HOME="$scratch/cache"
unset MBII_TEST_REAL_DATA MBII_VERIFY_ENGINE MBII_TEXTASSETS \
  MBII_GAMEDATA_PATH MBII_TEXTASSETS_PATH MBII_ENGINE_SRC
go test ./...
go vet ./...
go build ./...
```

On native Windows, isolate `USERPROFILE`, `APPDATA`, `LOCALAPPDATA`, `TEMP`,
and `TMP` as well; setting only `HOME` and `TMPDIR` is insufficient.

Mutating headless editor tests use `useEditorTestApp`; its fixture-owned clock
keeps session commits serial. Session timing tests use `useSessionTestClock`
and advance history boundaries explicitly. Fyne's headless `fyne.Do` executes
inline, so a real background timer does not provide UI-thread serialization.

The repository CI also checks formatting and builds its supported release
targets.

Ordinary runs skip external TextAssets/asset and engine-checkout verification.
Synthetic HOME/TMPDIR does **not** isolate an OS keychain: use the injected
credential-store tests, never real tokens or real native-store operations.
These commands are developer instructions, not a claim that native UI flows,
screen readers, oldest supported OS, or engine parity have been verified.

External data is explicit opt-in. Parser integration tests require both
`MBII_TEST_REAL_DATA=1` and `MBII_TEXTASSETS=/absolute/path/to/TextAssets`.
The asset benchmark additionally uses its own explicit roots:

```bash
MBII_TEST_REAL_DATA=1 \
MBII_GAMEDATA_PATH=/absolute/path/to/GameData \
MBII_TEXTASSETS_PATH=/absolute/path/to/TextAssets \
  go test . -run '^$' -bench BenchmarkVFS -benchmem
```

`-short` always skips external-data coverage. A path alone does not opt in.
Select only assets you have explicitly authorized for the run; do not point
routine checks at a live installation or private checkout.

## Verify engine-backed metadata

The committed `testdata/engine_snapshot.json` is derived from an exact external
MBII engine revision. Verify it without modifying the snapshot:

```bash
MBII_VERIFY_ENGINE=1 MBII_ENGINE_SRC=/absolute/path/to/moviebattles \
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
