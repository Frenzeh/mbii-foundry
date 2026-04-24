# Thermal Detonator (Alias)

`MB_ATT_THERMAL`

> Thermal detonator alias attribute. Defensive alias of `MB_ATT_THERMALS`.

## What it does

Defensive alias / duplicate of `MB_ATT_THERMALS` — both appear in the `weaponAttributeIDs` allow-list in `hidden_content.go`. The canonical thermal-detonator attribute is `MB_ATT_THERMALS` (which maps to `WP_REAL_TD`); `MB_ATT_THERMAL` is kept as a defensive alias for older content references.

## Per level

- **Level 1** — Same as `MB_ATT_THERMALS` level 1.
- **Level 2** — Same as `MB_ATT_THERMALS` level 2.
- **Level 3** — Same as `MB_ATT_THERMALS` level 3.

## Notes

- Prefer `MB_ATT_THERMALS` (plural) for new content.
- Both alias to `WP_REAL_TD` (the canon-accurate thermal detonator).
- See `MB_ATT_BASE_TD` for the basic-issue `WP_THERMAL` alternative.

---

`grenade` · `thermal` · `alias`

<!-- icon-suggestion: icon_stats_thermal -->
