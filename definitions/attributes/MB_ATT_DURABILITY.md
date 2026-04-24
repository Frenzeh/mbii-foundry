# Durability

`MB_ATT_DURABILITY`

> Generic damage-resistance buff — older sibling of `MB_ATT_DMG_TAKEN_MULTIPLIER`.

## What it does

Reduces incoming damage by a percentage. Legacy attribute kept for class-script compatibility — newer builds prefer `MB_ATT_DMG_TAKEN_MULTIPLIER` for explicit numerical control.

## Per level

- **Level 1** — small reduction.
- **Level 2** — medium reduction.
- **Level 3** — large reduction.

## Notes

- Stacks multiplicatively with `MB_ATT_DMG_TAKEN_MULTIPLIER` if both are set.
- Does not affect saber or environmental damage independently — those have their own pipes.
- For modern class definitions prefer the `_MULTIPLIER` variant.

---

`defense` · `damage-resistance` · `general`

<!-- icon-suggestion: durability -->
