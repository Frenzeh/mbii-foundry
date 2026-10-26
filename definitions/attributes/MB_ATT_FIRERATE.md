# Fire Rate

`MB_ATT_FIRERATE`

> Class-level fire rate buff (legacy / per-class).

## What it does

Reduces the inter-shot delay across the user's weapons. Older sibling of `MB_ATT_ROF_MULTIPLIER` — kept for class-script compatibility. Used on rapid-fire builds (Clone Trooper, ARC, T-21 gunners).

## Per level

- **Level 1** — slight increase (~10% faster).
- **Level 2** — moderate increase.
- **Level 3** — significant — auto-fire weapons feel fully cyclic.

## Notes

- Stacks multiplicatively with `MB_ATT_ROF_MULTIPLIER` if both are set.
- Distinct from per-weapon `rateOfFire` field.
- Wookiees and SBDs ignore Firerate on their melee mode.

---

`offense` · `firerate` · `weapons`

<!-- icon-suggestion: firerate -->
