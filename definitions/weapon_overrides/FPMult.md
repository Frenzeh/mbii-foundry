# Force Multipliers (FPMult / FPChargeMult / FPBlock*)

`FPMult 1.5`
`FPChargeMult 1.0`
`FPBlockMinMult 0.8`
`FPBlockMaxMult 1.2`
`FPNoBlockMinMult 0.5`
`FPNoBlockMaxMult 0.9`

> Force-pool cost multipliers for force-related interactions with this weapon.

## What they do

- **`FPMult`** — base FP cost when interacting with force powers.
- **`FPChargeMult`** — multiplier on charged-fire FP cost.
- **`FPBlockMinMult` / `FPBlockMaxMult`** — FP cost range when the user blocks shots from this weapon.
- **`FPNoBlockMinMult` / `FPNoBlockMaxMult`** — FP cost range when an enemy fails to block shots from this weapon.

## Valid values

Float multipliers, typically 0..3.

## Notes

- Lower mins reduce force drain on hits, making the weapon "force-friendly."
- Higher maxes punish defenders more for each successful block / failed block.
- The min/max range is sampled per shot — exact draw depends on engine roll logic.

---

`mbch` · `weapon` · `override` · `force`
