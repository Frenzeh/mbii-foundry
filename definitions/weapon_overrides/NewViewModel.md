# NewViewModel

`NewViewModel models/weapons2/foo/foo.md3`

> First-person model — the weapon as the user sees it.

## What it does

Replaces the view-mesh rendered in first-person (the "in-hand" model). Has no effect on what other players see.

## Valid values

Path to a `.md3` (or `.glm`) view-model under `models/weapons2/`.

## Notes

- Pair with `NewWorldModel` to keep silhouettes consistent.
- `NewHandsModel` and `NewBarrelModel` allow finer control over composite rigs.

---

`mbch` · `weapon` · `override` · `visuals`
