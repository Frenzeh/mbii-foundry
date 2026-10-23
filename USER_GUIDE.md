# MBII Foundry User Guide

## 1. Start locally

Foundry does not need an account or token to edit files.

On first launch, **Local Setup** offers two independent asset roots:

- **GameData:** your Jedi Academy runtime folder containing `base/` and
  `MBII/`. Use this for installed PK3 assets, portraits, sounds, shaders, and
  live testing.
- **TextAssets:** an optional `MBII/TextAssets` checkout or compatible loose
  asset tree. Loose TextAssets entries take precedence over matching PK3 entries
  in Foundry's asset view.

TextAssets is not a replacement for GameData: the repository normally lacks many
runtime images. You can also choose **Continue without assets** and configure
both paths later in `Edit -> Preferences`.

These paths are local to one computer. Choose them again on another machine
rather than sharing copied absolute paths. GitHub contribution setup is a
separate optional workflow.

## 2. Open and navigate documents

Use `File -> Open File` or the folder toolbar button. Foundry routes `.mbch`,
`.sab`, `.veh`, and `.siege` files by extension. The application does not
currently advertise drag-and-drop file opening.

`File -> New Character` and the usual New shortcut create a character document.
Other file types are opened from existing files. Multiple files can stay open in
tabs; an asterisk in the tab title means the form differs from the last
successful save.

SAB and VEH files can contain multiple top-level definitions. Use the
**Definition** selector above the editor to choose the one shown in the form.
Switching definitions is undoable and does not intentionally discard sibling
blocks.

### What the SAB and VEH forms cover

The SAB form exposes selected identity, type, first-blade, sound, combat,
effects, flags, and animation controls. Its type picker contains
`SABER_SINGLE` and `SABER_STAFF`; only the first blade has visual blade controls.
Additional parsed fields and blades remain visible in Source.

The VEH form exposes selected identity, model/skin, speed, turbo, acceleration,
deceleration, strafing, braking, armor, shields, and weapons. Its type picker
contains `VH_SPEEDER`, `VH_ANIMAL`, `VH_WALKER`, and `VH_FIGHTER`. Other parsed
vehicle keys remain visible in Source.

These are selected editing surfaces, not exhaustive engine dictionaries.

Numeric form fields retain their supported floating-point precision; opening a
file or editing an unrelated field should not round its SAB/VEH values. In MBCH,
the Custom Skills and Point Buy views edit the same indexed skill/name/rank/
description slots. Rank modifiers (`rankHealth`, `rankArmor`, and other `rank*`
keys) have one canonical value rather than competing copies in advanced fields.
Review Source before saving changes to unusual or repeated keys.

In character files, **Attributes** controls abilities and their levels;
**Inventory** controls starting weapons and items. **Flags** lists passive class
traits separately, with the technical identifier beneath each readable name.
Enforcer, Spy, and Astromech are excluded from available class choices;
Astrodroid remains available. Opening a file that already uses a hidden class
does not renumber or replace its stored identifier.

Configured rank-cost totals are authoring previews, not verified purchase or
specialization mechanics. Defensive pool displays show configured values;
energy, explosive, and melee EHP remain **Unverified**, without inferred
class/attribute damage-reduction bonuses.

## 3. Use the form and Source together

Use the **Guided / Advanced** controls above the document tabs to choose your
editing style. The choice is remembered. Both presentations share one document
and one pending Source draft; switching modes does not apply, reformat, or
discard text. Guided continues to show the last explicitly applied form state.

In Guided mode, the right-hand **Source** panel displays the current form's exact
would-save text, or its pending draft. Use its collapse/expand control to hide
or restore it, or `View -> Pop Out Source Panel` for a separate window.

- **Copy** copies the currently displayed source.
- **Edit** changes to a plain-text editor and pauses form-driven replacement of
  that text.
- **Apply** is enabled only when the active format parser accepts the draft. A
  successful Apply updates the form and becomes an undoable step.
- **Revert** explicitly discards the shared draft and refreshes both highlighted
  and editable text from the form. Continuing to type starts from that restored
  text, not the discarded draft.
- Switching tabs, toggling View/Edit, or using a pop-out does not discard an
  unapplied draft. Drafts and their parse diagnostics are document-specific;
  docked and popped-out Source views mirror the same draft. Closing one mirror
  does not overwrite newer edits from another.

If you press Save while a Source draft exists, Foundry asks whether to apply it
before beginning save review. A parse error leaves the draft available for
correction. Canceling leaves both the document and draft unchanged.

### Advanced text editing

Advanced provides a full-width editable source view for `.mbch`, `.sab`, `.veh`,
and `.siege` documents, with line numbers and syntax colours. Document tabs,
pending drafts, cursor position, and local text history stay separate per file.
The team composer remains a separate tool.

- **Complete** offers context-aware field, flag, enum, and asset suggestions.
  `Ctrl-Space` also requests completion where the operating system permits it;
  choose a suggestion explicitly to change the source.
- **Find/replace** supports case-sensitive matching, previous/next matches,
  individual replacements, and Replace All. `Command/Ctrl-F` opens Find;
  `Command/Ctrl-H` opens Replace.
- `Tab` / `Shift-Tab` indent or dedent; `Command/Ctrl-/` toggles line comments.
  Enter keeps the current indentation. These are undoable text edits, not an
  automatic formatter.
- Line/column diagnostics distinguish **errors** from **warnings**. Select a
  diagnostic to locate it; matching-bracket markers help inspect structure.
  Unknown custom fields are retained rather than silently removed.
- **Apply to Guided** explicitly updates the form from a valid draft. Errors
  keep the draft editable and block Apply; switching to Guided does not bypass
  this guard. Save and Save As still use the reviewed-save workflow below.
- Choose a syntax preset or custom token/background/selection colours in
  `Edit -> Preferences`. Syntax colours are independent of the application
  theme; custom colours use `#RRGGBB` or `#RRGGBBAA`.
- **Insert Asset** uses configured game/loose assets and inserts a portable
  logical reference at the retained selection. Compatible image/shader choices
  offer a still-image preview; this is not a 3D or engine-rendering preview.

Diagnostics check known syntax and format rules, not every engine behavior.
A clean editor is **not** a guarantee against runtime crashes; test the result
in the intended MBII engine version.

## 4. Undo, redo, and close safely

Use `Edit -> Undo` / `Edit -> Redo`.

- macOS: `Command-Z`, `Command-Shift-Z`
- Windows/Linux: `Ctrl-Z`, `Ctrl-Shift-Z` or `Ctrl-Y`

History covers form edits, valid Source applies, JSON imports, template changes,
and multi-definition selection. Text-entry edits are grouped into short
coalescing windows, and a document keeps up to 100 snapshots.

Fresh unnamed documents keep their initial defaults as the history baseline.
Undo can return to those defaults even before a name is entered; this internal
restoration does not relax the validation required by Source Apply.

Closing a dirty tab or detached document asks before discarding it. Application
Quit and the main window's close button share a guard covering tabbed and
detached documents, including unapplied Source drafts. Quit attempts to persist
recovery and reports whether that succeeded. If persistence or recovery cleanup
fails, continuing requires explicit acceptance that recovery is not guaranteed.

## 5. Review and save

`File -> Save` and `File -> Save As` do not write immediately:

1. If a Source draft exists, decide whether to apply it.
2. Foundry prepares the exact candidate without advancing saved state.
3. The **Save Changes** dialog shows **Current file** and
   **Will be written**, including byte counts and validation warnings.
4. Select **Save** to approve those exact bytes.

For an existing file, Foundry stages and syncs the candidate, creates and
verifies an exact backup, then atomically moves the reviewed file to a
same-directory recovery hold. It verifies that hold before publishing with a
platform no-replace operation. The hold is removed only after publication and
a final backup proof. If another writer wins the publication window, its file
is preserved and the error identifies the retained hold and backup paths.
An interruption or cleanup failure can likewise leave those recovery artifacts
for manual inspection. A failure before publication leaves the destination
unchanged. A failure after publication says explicitly that the candidate is on
disk and names the recovery paths; the editor keeps its prior saved baseline
and dirty state so the document must be reviewed again before retrying.

Save review is intentionally exact text, not a claim that all normalization is
harmless. Read the proposed side when preserving comments, unusual keys, and
multi-definition files matters.

Save As to a different existing file still requires review even if that file
already contains byte-identical candidate text. A successful save adopts the
chosen destination as the document path and saved baseline. Cancel or failure
keeps the destination chooser available for correction/retry instead of
reporting a successful save.

## 6. Recover an interrupted session

Every 15 seconds, Foundry attempts to snapshot dirty `.mbch`, `.sab`, `.veh`,
and `.siege` working state and any Source draft into the operating-system
configuration directory. Detached documents are included. It also captures a
final pass during ordinary shutdown and checks the persistence result.

After an interrupted session, **Recover Unsaved Work** lists safe snapshots.
Choose Restore to reopen the working state. Restoration never writes the
original file; use the normal reviewed Save flow when ready. Declining recovery
removes the recovery entries. A clean save or confirmed discard removes that
document's entry.

Recovery depends on writable configuration storage, not merely a configured
path. If startup reports that the configuration directory is unavailable,
local editing still works but recovery, backups, preferences, and favorites
cannot be persisted. A later write failure is not a successful recovery save;
read the quit guard's result before choosing to continue.

## 7. Browse assets and read diagnostics

The **Library/Assets** activity indexes:

1. PK3s under GameData (`base`, `MBII`, test/release/profile directories in the
   application's scan order), and
2. optional loose TextAssets, indexed afterward as overrides.

Search and browse logical game paths. Double-clicking a compatible asset in a
picker inserts its logical path into the focused field.

For character models and skins, the small diagnostic button beside the field
reports one of these conditions:

- found in the merged VFS,
- model or skin absent,
- found but unreadable or undecodable, or
- the requested portrait failed and a named fallback was selected.

Open the button for the source PK3/loose path, failure stage, fallback path, and
available suggestions. A green model/skin result verifies indexed file presence;
it does not render the 3D model or prove the engine accepts the whole character.

### Portrait behavior

Profile and skin portraits refresh after background indexing. Foundry tries an
explicit `uishader`, then model/skin portrait names, and may use the model's
default portrait where the applicable preview allows it. A silhouette means no
portrait was resolved. These are 2D images, not previews of mesh, materials,
animations, or tint.

Shader still-image lookup uses a deterministic **Foundry preview policy**:
later PK3 sources win, loose files win over PK3s, and then the lexically later
lowercase logical shader path wins; within a file, the last supported
declaration wins. Foundry selects a supported texture, not the shader's stage
blending or animation. This policy is not verified engine/profile equivalence.

### Keyboard and documentation access

Use Tab to focus custom activity, class, asset, and equipment controls, then
Enter or Space to activate the focused control. Focus cues and visible color/
RGB-channel labels supplement hover and swatches; these are not a claim of
screen-reader certification. Explicit documentation clicks still open/update
the Info panel when **Show info tooltips on hover** is disabled in Preferences.

## 8. Validate

`File -> Validate` (default shortcut `Command-R` or `Ctrl-R`) validates the
active editor's currently supported constraints. Save review displays warnings
but may still allow an explicit save.

`File -> Validate Folder` recursively scans `.mbch` files, skipping hidden
subdirectories. It reports read/parse errors and a missing required `name`. It
does not validate SAB, VEH, SIEGE, or MBTC files, and it is not a complete
engine simulation.

## 9. Compose an MBTC team

Open `Tools -> Team Composer (.mbtc)`.

One `.mbtc` file represents one siege team. The composer provides:

- required `name`;
- optional `TimePeriod`, `EUAllowed`, and `FriendlyShader`;
- `class1` through `class6`, which must be contiguous because the engine stops at
  the first empty class slot; and
- a separate ordered subclass list for each class slot, up to 41 entries.

`ClassesAllowed` is displayed only as a preserved legacy integer. It is not
described as a current engine control.

When GameData or TextAssets supplies character files, the composer checks class
references by file stem and parsed character name. Missing or unavailable
reference indexing is reported as a warning. Required structure, integer fields,
class contiguity, subclass limits, and engine file-size limits are blocking.

Saving an opened MBTC refuses to overwrite a destination that changed since it
was opened. Save As also guards creation/replacement races and uses backups when
replacing an existing regular file.

## 10. Track and export modpacks

Open the **Modpacks** activity.

- **New** creates common `ext_data`, map, shader, model, and HUD directories
  under the chosen project folder, then records the project.
- **Import** records an existing non-empty folder.
- **Open in Editor** changes the default open directory; it does not import or
  rewrite every file in the project.
- **Preview Manifest** lists archive member paths.
- **Build PK3** creates a `.pk3`.
- **Share / Export Source** creates a ZIP from the same source tree.
- **Remove** removes only Foundry's project-list entry. Files on disk stay in
  place and can be imported again.

PK3 and source export skip hidden descendants and the output file itself. They
reject symlinks, path traversal, absolute or non-canonical member paths, and
case/Unicode-normalization aliases. Empty projects do not overwrite an existing
archive. Output is staged and published atomically.

## 11. Bulk edit files

Open **Bulk Edit**, then:

1. Choose **Add File…** for one `.mbch`, `.sab`, `.veh`, `.siege`, or `.mbtc`
   file, or choose **Add Folder…** for an uncapped recursive scan. Extension
   matching is case-insensitive.
2. Review the scrollable load report. Unsupported files, duplicates, skipped
   symlinks, and read or parse failures remain listed with their exact paths.
   Folder scans never follow symlinks. Successfully parsed files are added and
   selected.
3. Adjust the batch with **Select All**, **Select None**, **Remove Selected**,
   or **Clear Batch**. Removing or clearing an entry does not delete its file.
4. Enter a field key (matched case-insensitively) and a new value.
5. Choose **Preview** and inspect every current-to-new row. A missing key is
   shown as an addition with the parser-selected scope.
6. Choose **Apply to Selected** only after the complete preview looks correct.

Changing the key, value, or selection invalidates the preview. Apply refuses a
file changed since Preview, backs up originals, and rolls back every published
candidate—including a failing current file—if any publication or final proof
fails. If external content changes after Apply begins, rollback does not
overwrite that newer content; the result points to the retained backup.

## 12. Configuration, credentials, and logs

Preferences and operational state live in the OS user configuration directory:

- macOS: normally `~/Library/Application Support/mbii-foundry/`
- Windows: normally `%AppData%\mbii-foundry\`
- Linux: normally `$XDG_CONFIG_HOME/mbii-foundry/` or
  `~/.config/mbii-foundry/`

The directory contains `config.json`, recent files, favorites, backup and
recovery data, modpack metadata, and the update cache. Foundry migrates the old
`mbii-fa-creator` directory by copying it and leaves the old directory intact.

The manually refreshed attribute/weapon catalog is separate:
`os.UserCacheDir()/mbii-foundry/catalog.json`, one atomically replaced cache
file, not a `data/` directory. Bundled resources are read-only; missing or invalid
cache falls back to installed/embedded data. Refresh fetches upstream `main`
over HTTPS with a 10-second timeout, a 4 MiB per-file bound, and no redirects.
It is not authenticated by the application-update manifest or an independent
catalog signature.

Local editing needs no GitHub credential. If you opt into contribution features,
**Connect / manage GitHub access** uses the operating system's native credential
store. The token is excluded from newly persisted `config.json`. An active
legacy plaintext token stays protected until native storage succeeds and its
read-back is verified; failure does not erase that source.

Only an explicit **Use saved credential** Connect, after a verified native read,
cleans an eligible regular retained pre-rebrand configuration backup. Recorded
migration provenance permits cleanup even after token replacement. Older
markerless backups are eligible only when their token matches the active legacy
or verified token. Differing or unverifiable backups remain unchanged; known
plaintext produces a warning for manual review. Foundry never migrates a backup
token into the native store, and does not automatically clean every historical
backup.

### Optional GitHub contribution

Workspace clone/push requires the connected account's expected HTTPS
`TextAssets` fork on GitHub. Upstream pulls are pinned to
`https://github.com/MovieBattles/TextAssets.git` and carry no GitHub credential;
they do not trust an arbitrary local upstream URL. In the submission dialog,
the pull-request checkbox is honored: leave it unchecked for push-only. If a
push succeeds but opening the requested PR fails, Foundry reports that partial
outcome instead of claiming the PR exists.

The raw log is `mbii-foundry.log` under the operating-system temporary
directory. `Help -> Debug Logs` redacts configured roots and known credentials
before display. Review all text again before sharing it.

## 13. Updates

Startup uses a six-hour cached GitHub release check;
`Help -> Check for Updates` forces a fresh request. Alpha and beta builds may
follow newer prereleases or stable releases, while stable builds ignore
prereleases. Drafts and releases with inconsistent tag/prerelease metadata are
not offered.

Automatic installation requires the long-lived publisher public key embedded
at release-build time and a valid Ed25519 manifest matching the requested
version, platform, architecture, byte length, and SHA-256 digest. Missing or
invalid trust data fails closed.

v0.16.0-alpha is the first trusted-key build. v0.15.0-alpha cannot discover or
authenticate it, so that upgrade must be downloaded manually from the Releases
page. Once v0.16.0-alpha is installed, later releases signed by the same
publisher key can use the authenticated installer.

- macOS and Linux implement automatic replacement with rollback.
- Windows opens the release page; no Authenticode publisher identity is
  configured for an in-place updater.
- Ordinary local builds do not embed the release publisher key.
- Manifest verification is not Apple notarization, Developer ID verification,
  Windows Authenticode, or an OS trust-store judgment. The v0.16.0-alpha macOS
  archive is intentionally ad-hoc signed and unnotarized.
