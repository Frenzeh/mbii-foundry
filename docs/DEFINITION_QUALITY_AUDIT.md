# Definition Quality Audit

Snapshot of what the Foundry's enum definitions actually contain right now. Run [`tools/audit-definitions.py`](../tools/audit-definitions.py) to regenerate.

## Summary as of 2026-04-22

**509 entries across 6 families — 0 rated "good".**

| Family | Entries | Good | Prose | Thin | Slop | Stub | Empty |
|---|---:|---:|---:|---:|---:|---:|---:|
| attributes   | 334 | 0 | 35 | 122 | 12 | 155 | 10 |
| weapons      | 51  | 0 |  3 |  27 |  0 |  21 |  0 |
| classes      | 25  | 0 |  1 |   0 |  0 |  24 |  0 |
| class_flags  | 18  | 0 | 10 |   6 |  2 |   0 |  0 |
| saber_styles | 8   | 0 |  6 |   0 |  0 |   2 |  0 |
| glossary     | 73  | 0 | 38 |  30 |  5 |   0 |  0 |

## What the ratings mean

- **good** — has either a markdown table or a `### Mechanics`-style section with concrete values. The kind of definition a balance designer can actually use.
- **prose** — has decent descriptive prose (≥200 chars) but no mechanical stats. Useful for a new player, thin for a designer.
- **thin** — has prose, but less than ~80 chars — a sentence or two, rarely enough.
- **slop** — matches a known AI-slop pattern: lazy suffixes like "X attribute.", marketing-speak ("powerful", "versatile", "essential"), lazy verbs ("Controls the X…", "Enables the X…").
- **stub** — the markdown file is our auto-generated stub (`*Stub — a human needs to document this.*`) or under 200 bytes of total content.
- **empty** — no description, no overview, no markdown file.

## Why "good" is zero

The bar for "good" includes mechanical data — multipliers, FP costs, cooldowns, hit-location tables, source references. Most of the current definitions are lore/flavor text ("Legendary Mandalorian iron plating") without actual numbers. A balance designer hovering `MB_ATT_BESKAR` in the Foundry app gets prose about what beskar *is*, not what it *does* to incoming damage.

Target for the next pass: get the most-edited attributes (top 30 by in-game use) to "good", then chip away at the long tail.

## Template

[`definitions/attributes/MB_ATT_BESKAR.md`](../definitions/attributes/MB_ATT_BESKAR.md) is the reference for what "good" looks like: damage-multiplier table per level, explicit exclusions ("blaster bolts not reduced"), direct source references (file + line).

Any PR that raises an entry's rating to "good" is a solid contribution — even one entry per PR is fine.

## Source ideas for extracting real stats

Per-attribute, the right move is to grep the MBII C source for the enum ID (`MB_ATT_*`):

```bash
grep -rn MB_ATT_<NAME> /path/to/moviebattles/game/
```

You'll typically find:
- A `#define` in a `.h` file with the multiplier/threshold constants
- Application logic in `g_combat.c`, `bg_pmove.c`, `w_saber.c`, `g_missile.c`, etc.
- Occasionally a table row in `bg_misc.c` or `bg_classes.h`

Translate those constants into the markdown table format shown in the Beskar template. If you don't have access to the source, the MBII Wiki and in-game testing are fallback sources — note whichever you used in a "Source:" footnote so future reviewers can verify.

## How contributors should use this doc

1. Run `python3 tools/audit-definitions.py --worst-first --limit 20` to get a personal to-do list.
2. Pick one enum you know from in-game experience.
3. Open its `.md` file via GitHub's web editor.
4. Follow the [Beskar template](../definitions/attributes/MB_ATT_BESKAR.md) structure.
5. Open a PR. Even one entry at a time is welcome.

See [`DEFINITIONS_GUIDE.md`](DEFINITIONS_GUIDE.md) for the full contribution workflow.
