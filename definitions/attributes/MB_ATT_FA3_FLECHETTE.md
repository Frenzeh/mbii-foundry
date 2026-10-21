# FA3 Flechette Attribute

`MB_ATT_FA3_FLECHETTE`

> Current engine attribute identifier associated with `WP_FA3_FLECHETTE`.

## What it does

The current engine attribute-name table accepts `MB_ATT_FA3_FLECHETTE` (two consecutive `T`s). The older `MB_ATT_FA3_FLECHETE` spelling is not an accepted alias. Existing unknown tokens remain preserved by the editor; newly authored selections use the canonical identifier.

## Valid Values

- The engine's attribute parser accepts explicit levels from **0 through 255**.
- A bare attribute token defaults to level **3**; explicit **0** is distinct from omission.
- These are syntax bounds, not verified gameplay ranks. Per-level ammunition, spread, and firing-rate effects are not established by the enum/parser metadata.

Identifier and syntax evidence: `game/bg_saga.c`, attribute-name table and attribute-list parser, in the local engine revision `3108edc0f9934c1918012eff5712f9f5e790c96d`.

## Related

- `WP_FA3_FLECHETTE`
- `WP_FLECHETTE`

---

`attribute` · `shotgun` · `cqb`

<!-- icon-suggestion: w_icon_flechette -->
