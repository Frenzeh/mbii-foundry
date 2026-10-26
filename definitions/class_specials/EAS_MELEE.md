# Slap (Melee)

`EAS_MELEE`

> Performs a melee slap with the current weapon.

## What it does

Quick, low-damage melee swing that works regardless of weapon held. Used for staggering nearby enemies or breaking saber locks at point-blank. Cost is animation time, not a resource.

## Notes

- Animation lookup respects per-saber `slapAnim` overrides — sabers can repurpose this for hilt-bash or push gestures.
- Damage and stagger scale with `MB_ATT_GUNBASH` for blaster classes.
- Does not require any special weapon; available on every class that has the binding.

---

`special` · `melee` · `stagger`

<!-- icon-suggestion: melee -->
