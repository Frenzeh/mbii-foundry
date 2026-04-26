# NewWorldModel

`NewWorldModel models/weapons2/foo/foo_w.glm`

> Third-person model — the weapon as seen on other players.

## What it does

Replaces the world-mesh shown on the player's hand from the third-person camera. This is the model other players (and the dropped-weapon pickup) render.

## Valid values

Path to a `.glm` or `.md3` weapon model under `models/weapons2/`.

## Notes

- Pair with `NewViewModel` to keep first-person and third-person consistent.
- Use `CorrectedWorldModel` only if the world-model needs separate origin/anchor adjustment.

---

`mbch` · `weapon` · `override` · `visuals`
