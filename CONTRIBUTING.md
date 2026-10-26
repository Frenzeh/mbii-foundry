# Contributing to MBII Foundry

Contributions are welcome. Keep local content authoring separate from repository
development: using Foundry to edit your own files needs no GitHub account, while
submitting changes requires a normal branch and pull request.

## Prerequisites

- Go 1.27.3 or newer, matching `go_module/go.mod` and CI.
- Git.
- Fyne's native requirements for your platform:
  [docs.fyne.io/started](https://docs.fyne.io/started/).
- An external MBII engine checkout only when verifying or changing
  engine-backed metadata.

Do development builds on the target operating system. WSL produces a Linux
application, not a native Windows executable.

The macOS deployment target is 13.0. Use `build_app.sh` for a universal bundle;
it builds and checks both architectures with that target. A successful build
on a newer Mac is not runtime certification on macOS 13.

## Build and run

```bash
git clone https://github.com/Frenzeh/mbii-foundry.git
cd mbii-foundry/go_module
go run .
```

Or build a repository-local binary:

```bash
go build -o mbii-foundry
./mbii-foundry
```

Native Windows PowerShell:

```powershell
cd go_module
go build -ldflags="-H windowsgui" -o mbii-foundry.exe
.\mbii-foundry.exe
```

Run development binaries from `go_module/` so resource discovery can find the
repository's `data/`, `definitions/`, `schemas/`, and `templates/`. Do not
commit generated binaries.

For a local macOS bundle:

```bash
./build_app.sh
```

The script creates `dist/MBII Foundry.app`, not an application under
`/Applications`. It builds an arm64+x86_64 executable, stages and verifies the
complete bundle on the destination filesystem, and atomically replaces an
existing bundle in `dist/`. The default signature is ad-hoc. Setting
`FOUNDRY_CODESIGN_IDENTITY` selects a local signing identity, but the script
does not notarize.

## Repository map

```text
go_module/         Go/Fyne application, canonical parsers, tests, updater
data/              curated runtime enum metadata
definitions/       human-readable enum and field prose
schemas/           validation schemas
templates/         starter content
tools/             definition audit and engine snapshot generator
macos/             app-bundle metadata and icon
packager/          separate Python packaging library
```

Important application boundaries:

- `go_module/parsers/` owns parsing and source-preserving AST updates.
- `session_state.go`, `source_panel.go`, `save_review.go`, and
  `crash_recovery.go` own document state and safe user workflows.
- `safeio/` owns atomic publication primitives.
- `archive_export.go` is the shared PK3/source ZIP writer.
- `updatemanifest/`, `cmd/signer/`, and `update_installer*` share the update
  trust contract.

Reuse those paths rather than introducing a second parser, writer, credential
file, or archive implementation.

## Change workflow

1. Create a branch from the current default branch.
2. Make a focused change.
3. Add or adjust a regression test only when it protects observable behavior.
4. Run the checks relevant to the changed package.
5. Inspect generated files and repository status; do not include binaries,
   credentials, local paths, recovery snapshots, or private content.
6. Open a pull request describing behavior and verification.

### Core checks

From `go_module/`:

Use nonsynced scratch directories for temporary files and isolated
`HOME`/configuration/cache directories. Leave external-data opt-ins unset for
ordinary verification.

```bash
gofmt -w <changed-go-files>
go test ./...
go vet ./...
go build ./...
```

For UI changes, also launch the application and exercise the changed surface.
For file-format changes, use a synthetic or redistributable fixture and inspect
the exact Save review output. Do not use private game files as test fixtures.

External corpus tests and benchmarks require `MBII_TEST_REAL_DATA=1` and their
explicit asset-path input. They do not discover an installed game by default.
Engine verification has a separate opt-in, shown below.

### Python packaging API

The supported standard-library API is `packager.PK3Builder` with
`packager.PK3Structure`; `packager.build_pk3` is the convenience entrypoint.
From the repository root:

```bash
python3 -m unittest packager.test_pk3_builder
```

Every populated asset list is a required input. Missing requested files or
required dependencies fail the build without replacing an existing export.
Use configured search roots or game-relative paths to preserve custom
subdirectories. Loose files outside those roots use type-based destinations;
unrelated ancestor directory names do not determine archive layout.

The Python builder and desktop Go exporter are separate implementations.
Verify the one changed rather than treating the other's checks as coverage.

## Engine-backed metadata

`go_module/testdata/engine_snapshot.json` records the external engine revision,
source hashes, enum declaration ranges, enum numeric values, and parser-key
inventory used by drift checks. To verify the committed snapshot:

```bash
cd go_module
MBII_VERIFY_ENGINE=1 MBII_ENGINE_SRC=/absolute/path/to/moviebattles \
  go test . -run '^TestExplicitEngineVerification$' -count=1
```

Requirements for `MBII_ENGINE_SRC`:

- it names a Git checkout, not a copied header directory;
- the relevant `bg_public.h` and `bg_saga.c` match the checkout's committed
  `HEAD` bytes;
- the files may be at the root, under `game/`, or under `codemp/game/`.

Verification compares derived metadata with the committed snapshot and does not
rewrite it. To intentionally regenerate a candidate:

```bash
MBII_ENGINE_SRC=/absolute/path/to/moviebattles \
  go run ../tools/generate_snapshot.go \
  -snapshot testdata/engine_snapshot.json \
  -schema ../schemas/mbch_schema.json
```

Review the revision, hashes, enum values, parser-key changes, schema impact, and
runtime consumers. Snapshot parity does not verify descriptive prose, costs,
defaults, or gameplay mechanics.

## Definition prose

See [`docs/DEFINITIONS_GUIDE.md`](docs/DEFINITIONS_GUIDE.md) for the definition
format.

- Preserve exact enum or field spelling.
- Distinguish verified engine behavior, observed in-game behavior, wiki claims,
  and inference.
- Include the engine revision and source location when a statement was checked
  against source.
- Do not invent defaults, units, balance recommendations, synergies, or valid
  values.
- Do not treat `data/*.json`, schema prose, or existing generated stubs as
  engine evidence.
- A question mark or explicit unverified note is better than false precision.

The SAB and VEH forms expose selected fields only. Definition prose must not
turn that UI subset into a claim that unsupported types or fields are invalid.

Run the prose inventory from the repository root:

```bash
python3 tools/audit-definitions.py
```

The resulting quality report is a dated inventory, not engine verification.

## Local paths, configuration, and credentials

Never commit machine-local GameData, TextAssets, MD3View, document, or modpack
paths. Tests must use temporary directories.

The application stores preferences below
`os.UserConfigDir()/mbii-foundry`. GitHub tokens belong in the OS native
credential store and are excluded from `config.json`. Tests for credential
migration must use the repository's mock/injected store, never a real token or
interactive keychain.

Retained legacy backups are not proof of secure migration. Explicitly choosing
**Use saved credential** and connecting with a verified native-store token
permits conservative cleanup; unverifiable historical copies remain untouched
and may still contain plaintext. Loading preferences alone does not sanitize
them.

Contribution sync pulls the pinned public upstream without a token. Authenticated
pushes must target the expected HTTPS GitHub fork, including effective URL
rewrites and redirects. Pushing a branch is not consent to open a pull request;
PR creation requires a separate explicit action and must report its own outcome.

Catalog refreshes use HTTPS and validated data, but are not covered by the
signed application-update manifest. Their mutable cache is
`os.UserCacheDir()/mbii-foundry/catalog.json`; bundled resources stay read-only.

## Release and update trust

Tags matching `v*` start the release workflow. Before any platform build, the
workflow requires the tag to point at the current `main` commit, requires a
successful completed `main` CI run for that exact commit, and verifies the tag,
`AppVersion`, changelog heading, and macOS marketing version agree. It then:

- builds Linux amd64, Windows amd64, and a true macOS universal executable
  with checked macOS 13.0 deployment targets for both slices;
- requires the canonical Ed25519 public key in every release binary;
- exposes the private key only to one isolated signing job;
- proves the private and public keys match;
- signs manifests binding version, platform, architecture, length, and digest;
  and
- creates a draft GitHub release containing exactly three archives and three
  manifests. Publishing the inspected draft is a separate, explicit action.

The current macOS release workflow uses ad-hoc signing and does not notarize.
It does not import a Developer ID certificate or claim Apple publisher identity.
The project does not configure Authenticode for Windows, and the Windows
application opens the release page instead of replacing itself.

The Ed25519 publisher key is a long-lived update trust root, not an operating
system signing identity. Keep its private half outside the repository and logs,
restrict access, and maintain an offline recovery copy. After a public key ships
inside v0.16.0-alpha, loss or unplanned rotation of the private key strands
automatic updates; compromise permits forged manifests. Any rotation therefore
requires a planned transition release that trusts both old and new keys.

Never describe a release as notarized, Developer ID signed, Authenticode signed,
or authenticated by a publisher key unless the inspected release artifacts
provide the corresponding verifiable evidence.

## Reporting issues

Include:

- Foundry version and operating system;
- file format and a minimal redistributable example;
- exact steps and observed result;
- whether GameData and/or TextAssets was configured; and
- sanitized diagnostics when relevant.

The raw log is `mbii-foundry.log` in the operating-system temporary directory.
The in-app Debug Logs view redacts configured roots and known credentials before
display, but contributors must still inspect all diagnostic text before posting.

See [LICENSE](LICENSE) for contribution terms.
