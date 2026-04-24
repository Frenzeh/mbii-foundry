# No Locational Damage

`CFL_NOLOCATIONALDAMAGE`

> Disables hit-location multipliers (no headshot bonus, no limb reduction).

## What it does

`g_combat.c:4956` — the hit-location resolver returns early if this flag is set, skipping the headshot / limb-shot damage adjustment. The player takes flat body damage regardless of where they were hit.

## Notes

- Droid-flagged classes (`classData[].isDroid`) get this behavior automatically via a separate `isDroid` early-return; the flag is for non-droid characters who need the same "no weak points" treatment (heavily armored juggernauts, bosses, certain NPC types).
- Saber damage ignores hit-location anyway, so the flag has no effect on saber damage.
- NPC stats file can set an equivalent `noLocationalDamage` at NPC-spawn time.

---

`defense` · `tank` · `hit-model`
