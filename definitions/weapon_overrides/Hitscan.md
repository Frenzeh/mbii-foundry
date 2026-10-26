# Hitscan / Disruptor / Gore Effects

`primHitscanShot   effects/foo/hit.efx`
`altHitscanShot    effects/foo/hit_alt.efx`
`primHitscanTracer effects/foo/tracer.efx`
`altHitscanTracer  effects/foo/tracer_alt.efx`
`primGore          effects/blood/spurt.efx`
`altGore           effects/blood/spurt_heavy.efx`
`disruptorBeam1    effects/disruptor/beam.efx`
`disruptorBeam2    effects/disruptor/beam_zoomed.efx`

> Hit / tracer / beam effect overrides for hitscan and disruptor weapons.

## What they do

- **`*HitscanShot`** — impact effect on hit point for hitscan weapons.
- **`*HitscanTracer`** — visible bullet tracer trail.
- **`*Gore`** — flesh-impact effect (blood spurts, lightsaber severs).
- **`disruptorBeam1` / `disruptorBeam2`** — sniper beam effect (un-zoomed and zoomed states).

## Notes

- Hitscan effects only fire on hitscan weapons; missile weapons use `MissileEffect`.
- Gore effects only render if engine gore settings allow.

---

`mbch` · `weapon` · `override` · `effect`
