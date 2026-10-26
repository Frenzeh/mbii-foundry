# MBII Foundry — Current Architecture

## Runtime and entry points

The desktop application is a Go 1.27.3/Fyne 2.8.2 `main` package in
`go_module/`. `main.go` owns application lifecycle, configuration, menus,
activity navigation, document routing, and window coordination. Most desktop
components are files in that package, not separate `editors/`, `browser/`, or
`dialogs/` packages. macOS bundles target macOS 13.0; a deployment target is not
proof of execution on the oldest supported OS.

Four document editors implement the shared `Editor` contract from `common.go`:

| Format | Implementation | Document surface |
|--------|----------------|------------------|
| MBCH | `mbch_editor.go` | Character definitions, equipment, custom skills and ranks |
| SAB | `sab_editor.go` | Selected saber fields and first-blade controls |
| VEH | `veh_editor.go` | Selected vehicle fields |
| SIEGE | `siege_editor.go` | Siege configuration |

Documents normally use tabs, but document and Source pop-outs also exist; this
is not a single-window architecture. SAB and VEH editors retain the complete
parsed document while selecting one top-level definition for form editing.
`mbtc_composer.go` is a separate team-composer dialog, not a fifth tab editor.
`bulk_editor.go` handles reviewed batch changes. `modpack_manager.go` tracks
project folders; `archive_export.go` implements desktop PK3 and source ZIP export.

## Document ownership and persistence

- `parsers/` owns format models, parsing, source-preserving AST updates, and
  generation for MBCH, SAB, VEH, SIEGE, and MBTC. Forms expose selected fields,
  not exhaustive engine dictionaries. Save review remains necessary for exact
  text fidelity; parser acceptance is not engine or gameplay verification.
- Each editor owns its working model and a `DocumentSession`
  (`session_state.go`). The session tracks original bytes, saved baseline,
  coalesced undo/redo snapshots, selected definition, and recovery identity.
  Fresh unnamed defaults are anchored before user edits so undo can restore
  them without imposing the stricter user-facing Source Apply validation.
  Deferred commits run on the UI thread and carry a generation guard so a
  previously queued callback cannot consume a newer pending edit.
- `SourceDraftStore` is application-owned and keyed by editor identity. Docked
  and popped-out `SourcePanel` instances mirror that shared draft, including
  text, parse diagnostics, and edit/view mode. Switching tabs or disposing a
  mirror does not republish stale local text. Apply, explicit Revert, or
  confirmed discard consumes a draft; passive refresh does not. Revert refreshes
  both highlighted and editable surfaces from the form.
  Panels subscribe to document and draft changes rather than polling. Closing a
  panel removes both subscriptions without deleting the document's shared draft.
- MBCH custom skills use the canonical indexed `CustomSkills`, `CustomNames`,
  `CustomRanks`, and `CustomDescs` arrays. All case-insensitive `rank*` keys
  belong to `RankAttributes`, not a second copy in `ExtraFields`. Form bindings
  preserve numeric precision rather than rounding unrelated values during an
  edit. Configured-cost previews and defensive-pool/EHP displays are authoring
  aids with unverified runtime mechanics, not engine-parity results.
- `save_review.go` prepares the exact candidate without changing saved state,
  captures the destination, and rechecks both at approval. Publication uses a
  staged candidate, verified backup and recovery hold, and atomic no-replace
  operations before advancing the editor path/baseline. Errors distinguish
  failure before publication from published bytes whose finalization failed.
  Save As still reviews and adopts a different destination when its existing
  bytes happen to match the candidate. `safeio/` provides atomic file helpers.
- `crash_recovery.go` persists working source, selected definition, baseline,
  and unapplied draft separately from original documents. Periodic capture and
  the shared close/quit flow include detached editors. Quit reports actual
  recovery failures and requires explicit acceptance of unavailable recovery;
  it does not infer successful persistence merely from a configured directory.

## Assets, previews, and documentation

`vfs.go` indexes GameData PK3s and optional loose TextAssets into a logical-path
view; loose entries override matching PK3 entries. `asset_browser.go`, asset
pickers, and icon/portrait resolvers consume that view. Indexing and shader
prebuilding run in the background; UI publication uses the Fyne UI thread.

`shader_resolver.go` supplies still-image previews from supported
`map`/`clampmap`/`animmap` textures, not stage blending or engine rendering.
Only the winning VFS copy of each shader file is read. Duplicate shader names
use a deterministic **Foundry preview policy**: later PK3 source, then loose
files, then lexically later lowercase logical shader path wins; within a file,
the last supported declaration wins. This is not verified equivalence with an
engine's active profile or shader registration order.

Portraits and texture images are 2D previews. Presence diagnostics do not prove
engine acceptance. Embedded 3D rendering, audio playback, and drag-and-drop
opening/path assignment are not current architecture guarantees. See the
[User Guide](../USER_GUIDE.md) and application menus for actual interactions and
shortcuts, rather than a speculative shortcut table.

`info_panel.go` displays definitions and contextual help. Explicit documentation
clicks use sticky context independently of the hover preference. Custom activity,
class, asset, and equipment controls provide keyboard activation/focus cues;
visible color/channel labels supplement swatches. These affordances do not
constitute screen-reader verification.

## Catalogs and resource boundaries

Embedded defaults and installed `data/`, `definitions/`, `schemas/`, and
`templates/` resources support offline editing. Bundled resources are immutable
inputs, not refresh destinations. `data_loader.go` merges installed catalogs
and a valid user cache over defaults, retaining hidden/rank merge policy.

The manual attribute/weapon refresh fetches upstream `main` over HTTPS from
`https://raw.githubusercontent.com/Frenzeh/mbii-foundry/main/data`. Requests have
a 10-second timeout, a 4 MiB per-file limit, and no redirects. Both validated
catalogs are written together as **one atomic file**:
`os.UserCacheDir()/mbii-foundry/catalog.json` (not a `data/` subdirectory).
Missing or invalid cache falls back to installed/embedded data. Fetch and
persistence prepare a `CatalogUpdate` off-thread; `PublishCatalogUpdate` runs
on the UI thread. HTTPS transport is not authentication by the application
update manifest or by an independent catalog signature.

## Local configuration, credentials, and contribution

Preferences, recovery, backups, and operational state use
`os.UserConfigDir()/mbii-foundry`. Local editing does not need GitHub access.
`credentials.go`, `credential_access.go`, and `config_persistence.go` isolate
native-store access and protect active legacy plaintext until secure storage
succeeds. Explicit **Use saved credential** Connect can sanitize an eligible
regular retained pre-rebrand backup after a verified native read; it never
imports the backup's token. Proven migration provenance permits cleanup after
token replacement; older markerless backups require a matching active legacy
or verified token. Differing or unverifiable backups remain unchanged, with a
warning when plaintext is known; not every historical backup is auto-cleaned.

`github_manager.go` restricts authenticated clone/push to the connected user's
expected HTTPS `TextAssets` fork. Upstream pulls are credential-free and pinned
to `https://github.com/MovieBattles/TextAssets.git`. The submission dialog
honors its pull-request checkbox: push-only is valid, and a failed PR after a
successful push is reported as that partial outcome.

`updatemanifest/`, `updater.go`, platform installers, and `cmd/signer/` implement
the separate signed application-update boundary. Manifest authentication does
not establish notarization, OS publisher identity, or engine compatibility.

## Separate Python packaging API

The root `packager/` package is a supported Python API, separate from the Fyne
application and its Go parser/session state. It exports `PK3Builder`,
`PackageManifest`, `build_pk3`, `validate_package`, `clean_package`, asset
resolution helpers, and structure validation through `packager/__init__.py`.
It is not an embedded desktop backend, and desktop export should not be
assumed to call it.

For development dependencies and opt-in external-data checks, see
[`go_module/README.md`](../go_module/README.md). Historical Wails ideas remain
in [WAILS_MIGRATION.md](WAILS_MIGRATION.md); their proposed APIs and features
are not the implementation described here.
