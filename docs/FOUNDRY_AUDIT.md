# Foundry audit

**Date:** 2026-09-07. **Source baseline:** `010c1c8` (`Fix portrait rendering and source-aware asset caches`), application version `0.15.0-alpha`.

This is the second-pass audit of the standalone Go/Fyne Foundry editor. It supersedes the preliminary local reports `~/tmp/foundry-independent-audit.md` and `~/tmp/foundry-parser-consumer-evidence.md`. This pass changes documentation only: the findings below are not implemented fixes.

## Executive assessment

Foundry already has useful character editing, Source editing, asset browsing, variants, character comparison, and point-buy tooling. The next investment should be **document safety and trustworthy diagnostics**, not a toolkit rewrite or another parallel set of editors.

The most consequential findings were reproduced against the actual Go implementations:

- A rejected oversized MBCH save truncates the existing destination before reporting the size error; a configured backup survives in the exercised case.
- SAB and VEH parse/generate cycles retain only the first definition in a two-definition document.
- Source Apply changes the form but leaves it clean; retargeting the Source panel discards unapplied text.
- PK3 building includes files beneath a hidden directory and follows a file symlink outside the chosen project.
- The macOS updater's Go ZIP-extraction fallback permits a `../` member to escape its extraction directory.

**Immediate author precautions:** work on copies or version-controlled files; compare original and regenerated text before replacing a shared asset; explicitly save applied Source edits; do not round-trip multi-definition SAB/VEH originals; export only an intentionally clean, symlink-free staging folder. Keep the prior app bundle when installing updates. These precautions reduce exposure, not repair the underlying contracts.

## Evidence and limits

Evidence labels used throughout:

- **[P] Parser exercise:** an isolated program imported the checkout's canonical `go_module/parsers` package and called its real parse/generate functions.
- **[R] Runtime-method exercise:** an external Go overlay added temporary probes to the production package without changing repository source. Fyne's headless test application exercised the actual editor, Source panel, PK3 builder, and macOS ZIP fallback. Probe success means the reported observation was reproduced, not that Foundry is safe.
- **[S] Source inspection:** concrete implementation paths were read; no corresponding end-to-end user interaction is claimed.
- **[V] Native visual evidence:** production Fyne windows/canvases captured during the preceding portrait verification, reused here for visual observations.
- **[INFERENCE]** A consequence or risk derived from inspected behavior but not directly exercised.

This is representative coverage across the areas below, not an exhaustive review of every function. Execution was on **macbook** only. No game was launched, no update was downloaded or installed during this audit, and no Windows/Linux runtime, screen reader, full-dataset performance run, power-loss simulation, or network attack was performed. **office-pc** remained offline and was not probed. Runtime assets, sync state, and credentials were not modified.

The preceding portrait fix had already passed `go test ./...`, focused portrait race tests, `go vet ./...`, and an ARM64 build. Those results are historical verification of that fix, not a new full-suite audit gate. This documentation pass ran the isolated probes below instead of rerunning project-wide validation.

## Coverage map

| Area | Existing capability and representative path | Evidence / remaining concern |
|---|---|---|
| Open/create/save/export | Editor interface, format-specific LoadFile/SaveFile, app Save/Save As, PK3-origin Save As | [S/R] [editor persistence](../go_module/mbch_editor.go#L1268-L1379), [dispatch](../go_module/main.go#L1904-L1993). Destructive validation order and inconsistent writer completion. |
| Document fidelity | MBCH extra-field maps, deterministic generators, SAB/VEH/SIEGE parsers | [P/S] [MBCH](../go_module/parsers/mbch_parser.go#L199-L271), [SAB](../go_module/parsers/sab_parser.go#L107-L161), [VEH](../go_module/parsers/veh_parser.go#L45-L86), [SIEGE](../go_module/parsers/siege_parser.go#L75-L186). These are object serializers, not lossless documents. |
| Source / unsaved / recovery / bulk | Editable Source, dirty-tab and quit prompts, recent files, MBCH backups, bulk fields | [R/S] [Source state](../go_module/source_panel.go#L229-L262), [Apply](../go_module/source_panel.go#L428-L479), [backups](../go_module/common.go#L146-L231), [bulk](../go_module/bulk_editor.go#L86-L131). No app-wide command history or crash-draft recovery was found in the inspected Go paths. |
| FA domain / loadouts | Attributes, overrides, custom skills, point-buy simulator and budget, defensive matrix | [R/S] [budget](../go_module/mbch_pointbuy_sim.go#L377-L468), [defensive matrix](../go_module/stats_dr_calculator.go#L48-L115), [validation](../go_module/validation.go#L32-L140). Engine-revision grounding and a lives-field mismatch need attention. |
| Schema / definition drift | Built-in data plus external metadata, rich definitions, developer-field drift check | [S] [merge](../go_module/data_loader.go#L38-L133), [limited drift test](../go_module/dev_fields_test.go#L23-L65). The existing check does not certify all schema types, enum levels, costs, or engine semantics. |
| Teams / modpacks / export | MBTC composer, SIEGE team form, project tracking, PK3 builder | [R/S] [MBTC](../go_module/mbtc_composer.go#L178-L255), [projects](../go_module/modpack_manager.go#L205-L235), [PK3](../go_module/main.go#L2434-L2486). Boundary checks and truthful completion reporting are missing in specific paths. |
| Asset provenance / caches | Ordered VFS, loose-file overlay, shader resolution, source-aware portrait caching | [V/S] [VFS](../go_module/vfs.go#L52-L185), [portrait regressions](../go_module/portrait_loading_test.go). Portrait defect is resolved; selected runtime profile and scan/decode diagnostics remain improvement areas. |
| Layout / responsiveness | Profile sections, model/skin gallery, Source panel and pop-out | [V/S] [gallery](../go_module/widget_model_gallery.go#L227-L360), [refresh work](../go_module/mbch_editor.go#L303-L347). Squeezed gallery search was visible; full-data latency was not measured. |
| Keyboard / accessibility | Platform-aware file/validate/tab shortcuts, ordinary Fyne controls | [S] [shortcuts](../go_module/main.go#L1182-L1239), [class cards](../go_module/class_icon_picker.go#L152-L242), [clickable cells](../go_module/holdable_grid.go#L334-L370). Custom card focus/keyboard contracts need completion; no screen-reader result is claimed. |
| Setup / portability | Initial path wizard, skip mode, preferences, config migration, bundled runtime data | [S] [wizard](../go_module/main.go#L2541-L2667), [migration](../go_module/config_dir.go#L24-L97), [bundle script](../build_app.sh#L24-L75). Build requirements, rollback, migration completion, and path diagnostics need coherent guidance. |
| Secrets / updates / archive safety | HTTPS release fetching, temporary download, platform installers | [R/S] [download](../go_module/update_installer.go#L55-L116), [Mac install/extract](../go_module/update_installer_darwin.go#L33-L191). Extraction containment, release authentication, and credential storage require hardening. |
| Tests / maintainability / onboarding | Cross-platform CI, portable MBCH and portrait regressions, optional real-asset tests, contribution docs | [S] [CI](../.github/workflows/ci.yml#L12-L58), [parser tests](../go_module/parsers/mbch_roundtrip_test.go), [definition audit](DEFINITION_QUALITY_AUDIT.md). Add contract-focused regressions for demonstrated losses rather than a coverage-percentage target. |

## Priority findings

Priorities reflect author data loss and security boundaries first. **High** means address before relying on the affected workflow for important originals or distributing trusted updates. It is not a claim that every ordinary edit fails.

### F1 — High: validation occurs after destination truncation

**[R/S]** `MBCHEditor.SaveFile` calls `os.Create` before `SaveToWriter`; the latter rejects generated output above 8,192 bytes. The actual production-method probe loaded a 69-byte MBCH, set a 9,000-byte description, and saved to the same isolated path:

```text
error=file exceeds 8192 character limit (9118 chars) - reduce attributes or remove overrides
original_bytes=69 remaining_bytes=0 backups=1
```

This is an exercised rejection in the real saver, not merely an illustration of `os.Create`. The probe configured a FileManager: one backup remained. The call to `CreateBackup` ignores its returned error, so backup success is not a save precondition. SAB, VEH, and SIEGE SaveFile also truncate first and defer Close without checking its result. Their complete failure paths were inspected, not fault-injected.

**[INFERENCE]** Write/close failures can leave incomplete files or report success too early. A power interruption's exact outcome depends on filesystem and timing; this audit does not claim every crash produces a zero-byte file.

**Recommended contract:** finish generation and applicable validation before opening the destination; stage output on the destination filesystem, check write/close (and durability operations appropriate to the supported platforms), then replace only after success. Preserve an existing original on every failed save. Apply that contract to Save As and exporters too. Distinguish backup failure and offer a deliberate decision instead of silently proceeding. Backups need source-path identity and collision-resistant names: current names use only basename plus second-resolution time.

Sources: [MBCH saver](../go_module/mbch_editor.go#L1333-L1379), [SAB saver](../go_module/sab_editor.go#L476-L503), [VEH saver](../go_module/veh_editor.go#L240-L267), [SIEGE saver](../go_module/siege_editor.go#L281-L308), [backup naming](../go_module/common.go#L146-L165).

### F2 — High: successful parsing can discard other definitions

**[P/S]** ParseSAB and ParseVEH use `FindStringSubmatch` for one outer block and return one object. Their generators write that one object. In isolated two-definition inputs, both parsers returned nil errors and both generators omitted the second definition.

```text
SAB parse_error=nil first=saber_a retained_second=false
VEH parse_error=nil first=veh_a retained_second=false
```

**Recommended contract:** treat a file as a document containing definitions. Select and edit one definition without deleting its siblings, comments, or unmodeled content. Until that contract exists, detect unsupported multiplicity and refuse destructive overwrite, with an explicit diagnostic. Do not silently reinterpret a whole-file operation as a single-object conversion.

The parsers also remove comments and regenerate order/default formatting. Deterministic sorted extra fields are valuable for stable comparisons but are not preservation of original ordering, duplicate keys, or comments. Unknown fields inside modeled blocks have some preservation; unknown top-level blocks are not generally retained by the MBCH representation.

Sources: [SAB first match](../go_module/parsers/sab_parser.go#L107-L132), [SAB generator](../go_module/parsers/sab_parser.go#L360-L390), [VEH first match](../go_module/parsers/veh_parser.go#L45-L73), [VEH generator](../go_module/parsers/veh_parser.go#L143-L215), [MBCH extra fields](../go_module/parsers/mbch_parser.go#L12-L34).

### F3 — High: Source drafts bypass the document dirty contract

**[R/S]** In the actual SourcePanel and MBCHEditor methods:

```text
before_apply source_dirty=true editor_dirty=false
after_apply name="After" source_dirty=false editor_dirty=false path_restored=true
after_retarget draft_retained=false source_dirty=false
```

Apply correctly restores the original path after temporary-file loading. However, LoadFile marks the editor clean and Apply does not mark the changed form dirty afterward. SetActiveEditor clears the panel's draft state and replaces its content. The latter probe retargeted the same editor; switching away has the same reset path. Tab close and application quit prompts consult editor IsDirty, not unapplied Source text.

**[INFERENCE]** A user can apply a change and close without the expected save warning, or lose unapplied Source text when the active editor changes. The probe exercised state transitions, not an OS-level quit dialog.

**Recommended contract:** preserve per-document Source drafts, prompt or retain them across switches, make Apply one undoable document mutation, and have close/quit/recovery consult both form and draft changes. Keep the successful original-path restoration. Native text-entry undo and Source Revert are not substitutes for document-level undo of class picks, overrides, bulk changes, or Apply.

Sources: [Source retarget](../go_module/source_panel.go#L229-L246), [Apply](../go_module/source_panel.go#L428-L479), [LoadFile clean state](../go_module/mbch_editor.go#L1312-L1329), [tab close](../go_module/main.go#L1576-L1589), [quit guard](../go_module/main.go#L565-L590).

### F4 — High: PK3 export crosses the intended project boundary

**[R/S]** A synthetic project contained `.git/config` and a file symlink `linked.txt` to a sibling marker outside the project. Calling the real buildPK3 and reopening its ZIP produced:

```text
buildPK3 exported_entries=[.git/config linked.txt]
```

The walker returns for directories before its hidden-file filter, so it does not prune hidden subtrees. `os.Open` follows the file symlink. This was a confined local experiment using synthetic markers; no real repository metadata or secret was exported.

**Recommended contract:** preview the export manifest; prune excluded subtrees; reject or explicitly resolve symlinks under a documented containment policy; exclude the output itself; validate the destination before creating it; check ZIP writer Close and file completion errors. The present implementation defers ZIP Close and announces success before checking archive finalization. An output located within the source tree is also unguarded **[S]**, but recursive self-inclusion was not exercised.

Source: [PK3 builder](../go_module/main.go#L2434-L2486).

### F5 — High, conditional update path: ZIP fallback escapes extraction root

**[R/S]** The macOS Go fallback joins `destDir` with archive member names without containment validation. Calling `unzipGo` with a ZIP containing `../escaped.txt` wrote the marker next to the extraction directory and returned nil:

```text
unzipGo error=<nil> escaped_destination=true
```

Everything stayed inside an isolated temporary test root. The full updater normally tries `/usr/bin/unzip` first and only invokes this fallback on error. This probe demonstrates the fallback vulnerability; it does not claim the system extractor was exploited or that an official release is malicious.

**Recommended contract:** validate every member against the extraction root before writing, with explicit absolute-path, traversal, symlink, and resource-limit policy. Do not fall back into a partially extracted untrusted tree. Validate the completed application and trust metadata before replacement; retain rollback until the new app is verified.

The download path accepts a release-provided filename and URL, streams without an enforced byte limit, and does not independently authenticate the artifact. Mac installation strips quarantine, attempts ad-hoc signing while ignoring its result, then removes the backup before checking the new executable/relaunch. These are inspected hardening gaps, not additional exploited chains.

**Trust distinction:** HTTPS authenticates transport to the requested server. A checksum obtained from the same release can detect corruption but is not independent authentication against compromise of that release channel. A signed manifest verified against a trusted key, and appropriate OS publisher signing/notarization where supported, address a different trust requirement. Ad-hoc signing is not publisher identity. Preserve platform trust checks rather than treating quarantine removal as verification.

Sources: [ZIP fallback](../go_module/update_installer_darwin.go#L156-L191), [fallback dispatch and replacement](../go_module/update_installer_darwin.go#L33-L120), [download](../go_module/update_installer.go#L55-L116), [release packaging](../.github/workflows/release.yml#L88-L108).

### F6 — High for fidelity: parser acceptance is not engine compatibility

**[P/S]** MBCH, SAB, VEH, and SIEGE strip `//` from each line before parsing, without retaining source text. SIEGE's tokenizer can then absorb structural text into a broken quoted value; the exercised fixture lost its objective and second team while returning no parse error. An incomplete SIEGE document also returned nil error.

**Important consumer correction:** MBCH is read by `SGPV` / `BG_SiegeGetPairedValue` in `<sync-root>/mbii/moviebattles/game/bg_saga.c`, not a generic `COM_ParseExt` contract. Inspected SGPV lines 270–300 stop quoted values at `//` too and do not implement backslash-escaped quotes. Therefore `description "Known as \"Ghost\""` and quoted `//` examples must not be advertised as valid MBCH syntax that only Foundry mishandles. The real problem is unsupported/ambiguous text being accepted, normalized, or structurally lost without an adequate diagnostic. No SAB/VEH engine lexical-equivalence claim is made here.

A separate **placement mismatch** is directly demonstrated:

```text
ClassInfo
{
 name "Example"
 description "Inside"
}
description "Outside"
```

Foundry chooses `Inside`, loses `Outside`, and emits two `description` keys (one extra field inside ClassInfo and one top-level). The engine's top-level SGPV lookup skips subgroups (lines 243–263); BG_SiegeParseClassFile queries top-level description at lines 2382–2384 before extracting ClassInfo. This is not merely a choice between two equally valid top-level descriptions.

An empty extra-field value is emitted as a bare key. **ExtraFields means unmodeled by Foundry, not unknown to the engine.** For example, the local engine queries `userRGB` under its custom-RGBA feature flag at line 2508; Foundry can store it in ExtraFields. Engine paired-value lookup skips whitespace across newlines, so a bare recognized key can consume following text as its value. Do not assume such entries are harmlessly ignored.

**Recommended contract:** preserve the original document and source locations, define field placement and lexical rules against the actual target consumer, and diagnose unsupported quotes/comments/empty values/unbalanced groups before allowing destructive normalization. A greedier description regex or generic C-style escape support would not establish that contract. Do not promise byte-exact round trips from another regex patch.

Sources: [MBCH parse](../go_module/parsers/mbch_parser.go#L199-L271), [extra-field emission](../go_module/parsers/mbch_parser.go#L17-L34), [description emission](../go_module/parsers/mbch_parser.go#L817-L823), [SIEGE tokenize/extract](../go_module/parsers/siege_parser.go#L111-L186). Engine sources are local, intentionally referenced with portable placeholders rather than personal filesystem links.

### F7 — Medium: bulk/team/project workflows overstate completion

**[S]** Bulk edit uses case-sensitive prefix matching, not an exact parsed key, and writes raw entered values directly. Failed files disappear from the success count without per-file errors, preview, or rollback. A field absent from the document is not added; a longer key sharing its prefix can be changed. Source: [bulk implementation](../go_module/bulk_editor.go#L86-L131).

**[S]** MBTC parsing switches teams on any line containing `imperial` before comment filtering, fills slots sequentially rather than from the key's index, and does not clear previous roster slots when parsing another file. Save ignores the result of os.WriteFile and displays “Saved.” These are inspected behaviors; engine-level roster acceptance was not exercised. Source: [MBTC parse/save](../go_module/mbtc_composer.go#L178-L255).

**[S]** Modpack creation ignores directory-creation errors before announcing success. Its Share callback opens a save dialog but does not write the archive. VEH/SIEGE Validate return empty results, and their JSON import/export methods return nil without performing work. Their presence is not evidence those operations are implemented. Sources: [project creation](../go_module/modpack_manager.go#L205-L235), [Share](../go_module/modpack_manager.go#L350-L364), [VEH methods](../go_module/veh_editor.go#L315-L317), [SIEGE methods](../go_module/siege_editor.go#L311-L313).

**Recommended contract:** preview the exact affected files/fields, validate before writes, report per-file outcomes, and permit recovery from partial batches. Hide or explicitly label unavailable actions until real implementation exists. Validate team references and packaged paths against the selected project before distribution.

### F8 — Medium: engine limits and derived stats lack a single contract

**[R/S]** SaveToWriter caps the entire MBCH at 8,192 bytes; Source/validation code describes a 16,384-byte file limit with separate block limits. The inspected engine header `bg_saga.h` lines 165–168 defines file 16,384, ClassInfo 8,192, paired-value buffer 2,048, and WeaponInfo 4,096; the file reader rejects `len >= SIEGE_CLASS_FILE_LEN`. Exact usable content budgets also depend on delimiters and terminators. The 9,000-byte description in F1 is deliberately oversized test data, **not** a claim that it is a valid engine description.

The buffer gauge approximates some block lengths, whereas ValidateBlockSizes re-extracts blocks with regex. The advisory character validator checks selected relationships, not all enum validity, level bounds, or conflicts, and main Save does not call it as a universal gate.

**[R]** The defensive matrix probe set the parsed model's `ExtraLives=2` without a `reinforcements` extra field and reported `displayed_total_lives=1`. The calculator reads `ExtraFields["reinforcements"]` instead of ExtraLives. The inspected engine reads `extralives` for extra respawns and UI display at bg_saga.c lines 2600–2602. The mismatch propagates into character comparison's lives/EHP display. Conditional damage-reduction estimates should also be labeled estimates, not promised as exact live-game outcomes.

**Recommended contract:** centralize revision-aware limits and their inclusive/exclusive boundaries; derive exact serialized byte counts from the same document used for save. Tie class/ability conflicts, costs, and statistical formulas to cited engine versions, with explicit unavailable/approximate states. Extend existing point-buy and definition tooling rather than introducing duplicate budget controls.

Sources: [save limit](../go_module/mbch_editor.go#L1333-L1346), [validation](../go_module/validation.go#L85-L140), [gauge approximation](../go_module/buffer_gauge.go#L33-L83), [lives computation](../go_module/stats_dr_calculator.go#L48-L67), [comparison](../go_module/character_diff.go#L41-L70), [point-buy budget](../go_module/mbch_pointbuy_sim.go#L405-L450).

## Assets and native UI assessment

### Portrait loading is resolved, not pending

Commit `010c1c8` corrected generic raster-decoder routing, colliding resource identities, root/source-sensitive caches, and late VFS readiness refresh for the affected portrait surfaces. The runtime icon cache is VFS-owned and invalidated on refresh; embedded-only misses do not poison later runtime lookup. Portrait fallback is deliberate rather than an arbitrary texture-map iteration result.

**[V]** Production Fyne captures showed distinct atton/default and mira/default Profile portraits, multiple sith_assassin skin cards, selection of acolyte2 updating the Profile, a missing-model silhouette, and the model gallery. A readiness transition from an initially unindexed VFS refreshed the open Profile and variants. Pixel comparisons were performed during that preceding fix. This verifies portraits and those surfaces, not 3D model rendering or every image resource in the application.

Evidence filenames under `~/tmp/foundry-portrait-evidence/`: `native-profile-loading.png`, `native-profile-atton.png`, `native-profile-mira.png`, `native-skin-variants.png`, `native-skin-picker-multiple.png`, `native-selected-acolyte2.png`, `native-missing-portrait.png`, `native-model-gallery.png`. These are local evidence, not shipped documentation assets.

The gallery capture intentionally used a **two-model filtered VFS subset**. It is valid layout/portrait evidence, not a full-catalog performance test. Description color-toolbar PNG resources still produced unrelated generic TGA-decoder errors during the prior native run; their producer is [newColorDotResource / NewQ3ColorToolbar](../go_module/desc_generator.go). Do not mark all image-loading issues resolved on the strength of the portrait fix.

Only `/Applications/MBII Foundry.app` adopted the verified signed build, with the prior bundle preserved. The repository-local `go_module/mbii-foundry` executable was deliberately left old. A normal, sandboxed installed-app startup loaded the bundled data and exited cleanly; network denial meant no updater success was tested. Its receipt is `~/tmp/foundry-commit-adoption-receipt.md`. This audit did not rebuild or reinstall either executable.

### Concrete usability improvements

- **[V/S] Give gallery search flexible width.** In `native-model-gallery.png`, the entry is visibly squeezed beside the faction control. The current HBox uses the entry's minimum width. Keep search prominent and usable at the tested modal size; then verify at narrow windows and larger UI scale. This recommendation is not a guessed performance issue.
- **[V] Keep a compact character summary visible while editing.** Retain identity, model/skin, class, health/armor, and dirty/validation status near the active work area rather than requiring repeated scrolling between Profile sections. Existing content is useful; reduce vertical travel instead of hiding it.
- **[V/S] Make variants easier to compare side by side.** Preserve the working picker and selection callback, with clear selected/fallback labels and adjacent previews. Do not infer missing active-class styling from captures whose synthetic character did not necessarily have a selected class.
- **[S] Improve discoverability of existing comparison and Source tools.** `character_diff.go` already compares two MBCHs' stats, weapons, and attributes. Source already has highlighted view, edit/apply/revert, copy, and pop-out. Add an original-versus-would-save review that exposes normalization and dropped content; that is a different contract from the existing character comparison.
- **[S] Expose provenance and actionable asset status.** Show winning loose/PK3 source, fallback reason, active search roots/profile, and scan/decode errors. VFS scans several development/profile directories and overlays TextAssets last; this is not automatically the same as a particular game's active search path. Corrupt PK3/open failures can currently be skipped silently while Refresh returns nil. A silhouette alone cannot distinguish absent art, decode failure, or indexing in progress.
- **[S] Finish keyboard contracts for custom cards.** classCard and clickableCell implement pointer activation but no Focusable/TypedKey contract. Retain existing platform shortcuts and standard controls; add tab/arrow navigation, Enter/Space activation, visible focus and text alternatives where custom drawing is used. Verify focus restoration around modals and Source switching. Screen-reader support remains unverified.

### Responsiveness: measure before prescribing

**[S]** Gallery filtering reconstructs cards and loads their images on each change. MBCH markDirty rebuilds defensive-matrix widgets and asset-health badges; Source has a 500 ms fallback ticker that regenerates text. The ticker has no panel disposal signal in its inspected lifetime. These are concrete candidates for profiling, not proof of a measured hang.

**Recommended validation:** time cold/warm gallery opening, typing/filtering, switching characters, and Source updates on a representative complete asset set. Keep work proportional to visible/changed content, cancel obsolete work, and give long scans progress and cancellation. Introduce debouncing, incremental updates, or virtualization only where measurements justify them. Do not prescribe an arbitrary 150 ms delay, extrapolate from the two-model capture, or replace Fyne solely on this evidence.

Sources: [gallery rebuild](../go_module/widget_model_gallery.go#L254-L315), [dirty refresh](../go_module/mbch_editor.go#L303-L347), [Source ticker](../go_module/source_panel.go#L83-L106).

## Setup, packaging, and portability

**[S]** The local first-run wizard accepts GameData and optional TextAssets and has a Skip path; configuring a GitHub token is not required for local editing. The separate contribution-workspace wizard requires a token. Make that distinction explicit so a new author is not sent through a repository-fork workflow just to browse assets.

The current configuration is in the OS user-config directory, e.g. `~/Library/Application Support/mbii-foundry/config.json` on macbook, not a legacy dotfile. Config paths are machine-local; document roles using **macbook**, **main-pc**, **office-pc**, **nas**, and `<sync-root>` instead of copying another account's absolute paths. Show detected and configured roots and why a path is rejected. A failed config-directory copy can leave a partial new directory that a later startup treats as already migrated **[S/INFERENCE]**; migration needs a completion/rollback contract, not just an existence check.

`go.mod` and CI require **Go 1.24**. The README previously said 1.21; this documentation pass corrects it. The actual log is `mbii-foundry.log` beneath `os.TempDir()`, not necessarily next to the binary. Fyne still requires platform-native build dependencies; Go alone is not a complete cross-platform desktop toolchain.

`build_app.sh` **builds successfully before removing the installed bundle**. Therefore the draft claim that a compilation failure deletes the old app was wrong. After a successful build, it does remove the bundle and populate the replacement in place; later copying/packaging/signing failures lack a preserved rollback. Signing failure is tolerated. Stage and validate the entire bundle first, preserve the prior installation, and keep source-build, local raw-binary, and installed-bundle launch instructions distinct. No script or installer was changed by this audit.

The release workflow names its Mac artifact `macos-universal` but the inspected build is a single ordinary `go build`, with no architecture merge. **[S/INFERENCE]** The label alone is not proof of a universal executable. Verify artifact architectures and bundle metadata before advertising portability. Likewise, do not infer current public release availability from README prose; this audit did not query release hosting.

Sources: [initial setup](../go_module/main.go#L2541-L2667), [workspace wizard](../go_module/workspace_wizard.go#L14-L18), [config load](../go_module/main.go#L1995-L2048), [migration](../go_module/config_dir.go#L24-L97), [module requirement](../go_module/go.mod#L3-L7), [logger](../go_module/logger.go#L17-L30), [bundle script](../build_app.sh#L24-L75), [release build](../.github/workflows/release.yml#L80-L108).

## Credentials and safe diagnostics

**[S]** GitHubToken is a JSON config field, and saveConfig serializes configuration using os.WriteFile with requested mode `0644`. Actual permissions depend on existing file mode and umask; no live credential value was inspected or copied into this report. Prefer the platform credential store, keep ordinary preferences separate, and handle credential/config persistence errors. Where file storage is unavoidable, use restrictive permissions and explicit migration handling.

Logs include source paths and parser content metadata. Provide a sanitized diagnostic export with selected roots expressed portably; never ask contributors to paste an entire token-bearing config or private absolute-path log. A public report should use `<Foundry bundle ID>` rather than a personal bundle identifier. Archive scope and updater trust requirements are covered by F4/F5 rather than a blanket claim that all ZIP/PK3 handling is safe.

Sources: [token field](../go_module/main.go#L145-L148), [config save](../go_module/main.go#L2334-L2337), [logger](../go_module/logger.go#L17-L40).

## Tests, maintainability, and documentation

The existing CI builds, vets, formats, and tests on three OS families. Portable tests already defend important MBCH custom-spec/variant behavior, shader lookup, VFS suggestions, and portrait rendering/cache boundaries. Preserve that value. The developer-field test checks only the schema's dev-key set; do not describe it as full schema/engine parity.

Real-asset tests can skip without the external checkout. One Legends round-trip test still uses a hardcoded machine-specific root; another supports `MBII_TEXTASSETS`. Prefer portable synthetic boundary fixtures and an explicit opt-in integration-root convention. The inspected parser test files focus on MBCH; absence of dedicated SAB/VEH/SIEGE fixtures is a gap, not a measured claim of zero function coverage.

Prioritize regression tests that would fail on the demonstrated defects: rejected-save original preservation; Source Apply/switch/close draft state; multiple-definition retention; unsupported-lexeme diagnostics with original preservation; archive traversal and symlink/export exclusions; late writer failures; and field-to-engine stats semantics. Add generated parser inputs/fuzzing for nesting, duplicate keys, comment boundaries and malformed input where it exercises a real contract. Do not replace this with snapshot wording tests, checks that merely assert “did not panic,” or an arbitrary coverage percentage.

Keep canonical parsing in `go_module/parsers`, reuse a coherent document/save contract across callers, and avoid a second convention beside existing helpers. Definition content needs engine references, tested conditions, and revision labels; the [definition-quality audit](DEFINITION_QUALITY_AUDIT.md) is a dated inventory, not a current guarantee about every loaded definition. README, USER_GUIDE, module docs, build scripts, and release metadata should agree on the actual workflow and limitations. This pass corrects the root README's safety overclaims, Go version, log location, and direct-install warning; it does not claim every historical document has been reconciled.

## Reproduction reference

The local helpers used for this pass are under `~/tmp/foundry-audit-probes/`, outside the repository. They are audit evidence, not permanent tests. From `go_module`, the executed commands were:

```sh
go run ~/tmp/foundry-audit-probes/main.go
go test -overlay ~/tmp/foundry-audit-probes/overlay.json -run '^TestAuditProbe' -count=1 -v .
```

The local overlay JSON maps an otherwise nonexistent package test filename to the external helper; it contains machine-local paths and is intentionally not committed. To recreate elsewhere, adapt the temporary overlay paths and import the module returned by `go list -m`; do not copy a private absolute path. All filesystem probes must use a fresh temporary root with synthetic data.

Small parser repros can also be reconstructed directly from these inputs:

```text
# ParseSAB -> GenerateSAB
saber_a
{
 name "Alpha"
}
saber_b
{
 name "Beta"
}

# ParseVEH -> GenerateVEH
veh_a
{
 type VH_SPEEDER
}
veh_b
{
 type VH_FIGHTER
}
```

The `#` headings above are explanatory separators, not part of the input. Run each format separately. Both return nil parse errors and omit the second definition on generation. The description-placement fixture is in F6.

SIEGE structural-loss input used with ParseSiege -> GenerateSiege:

```text
Teams
{
 team1 RedTeam
 team2 BlueTeam
}
RedTeam
{
 RequiredObjectives 1
 briefing "Destroy generator // do not fail"
 Objective1
 {
  goalname "Generator"
  final 1
 }
}
BlueTeam
{
 briefing "Defend"
}
```

Observed: nil parser error, no retained Objective1 or BlueTeam in generated output. This fixture tests rejection/preservation of an ambiguous lexical boundary; it is **not** asserted to be engine-valid quoted-comment syntax. `Teams { team1 RedTeam` followed by `RedTeam { briefing "unterminated` also returned nil parse error.

The Source/saver probe uses NewMBCHEditor, a temporary FileManager, NewSourcePanel, LoadFile, SaveFile, applyEdits, and SetActiveEditor, not a reimplementation. Fyne's default test theme lacked a bold-monospace font on the first harness run; selecting the standard Fyne theme allowed the production-method probes to run. That harness failure is not reported as a production startup defect. Final isolated state/archive probes passed while recording the defects above. Locale and duplicate-library warnings were observed; they were not suppressed.

## Recommended order of work

1. **Protect originals and drafts:** F1–F3, with acceptance tests that exercise failure and switching paths. Preserve content rather than silently dropping unsupported input.
2. **Constrain archive and update boundaries:** F4/F5 before trusted packaging/update reliance, including complete-write checks, trust verification, and retained rollback.
3. **Make parser/domain diagnostics honest:** F6–F8; target actual engine consumers, field placement, exact size boundaries, and existing budget/stat tools.
4. **Finish batch/team workflow contracts:** per-file results, recoverable batches, meaningful validation, and no false-success placeholders.
5. **Polish the proven native surface:** flexible gallery search, compact summary, clearer variants/provenance, save review, keyboard focus, then measured full-dataset responsiveness.

Acceptance for a future fix is a demonstrated changed behavior, not this document's priority label. Portraits remain resolved at `010c1c8`; all new findings in this audit remain open until separately implemented and verified.
