# Model Scale Multiplier

`MB_ATT_MODELSCALE_MULTIPLIER`

> Multiplier on the player model scale.

## What it does

Scales the player model and hitbox uniformly. Smaller values produce harder-to-hit characters (Yoda-flavor); larger values make giant boss characters. Stacks on top of any `modelscale` field set in the MBCH.

## Notes

- Hitbox tracks the model scale — small models really are harder to hit, large ones easier.
- Default 1.0; common Yoda-class values around 0.6–0.7.
- Pairs with `MB_ATT_BASESPEED` to keep movement feel consistent at unusual sizes.

---

`multiplier` · `model` · `hitbox`

<!-- icon-suggestion: scale -->
