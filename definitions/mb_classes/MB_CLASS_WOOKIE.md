# Wookiee

`MB_CLASS_WOOKIE`

> Rebel melee juggernaut — massive HP, ignores knockback, trades shots for slaps.

## Role

The Rebel brawler. Enormous scaling HP pool (`WOOKIEE_HEALTH_0`=125, up to 450 at Wookiee Health 3), `MM_PK_KATA` melee moveset, Rage resource for Fury + Barge. Secondary-fire melee is the Wookiee Slap (not the standard punch). Wookiees take 2x damage from fire — the one reliable soft-counter. Chewbacca is the canonical default.

## Default loadout

- **Base health** — 125 (up to 450 with Wookiee Health 3)
- **Base speed** — 187.5
- **Armor pool** — `NO_ARMOR_POOL`
- **Melee moves** — Punch + Kick + Kata (`MM_PK_KATA`)
- **Scale** — 1.15x (big hitbox)
- **Resource** — Rage (fuels Fury)
- **Abilities (CS1-4)** — none, none, none, none
- **Abilities (CS5-8)** — `EAS_HI_FURY`, `EAS_HI_BARGE`, none, none
- **Forcecfg** — `forcecfg/red/wookiee`
- **Model** — `chewbacca`

## Signature mechanics

- **Secondary-fire melee = Wookiee Slap**. Uppercut melee is gated on explicit grant — FA designers must add `MM_ALL` or similar if the full roster is wanted.
- **2x fire damage** — the intended counter-play window for a Wookiee.
- **`MB_ATT_WOOKIE_STRENGTH`** is the melee-identity attribute. Rank 1 adds damage; Rank 2 grants Heavy Melee speed + knockdown/kick immunity; Rank 3 is lethal (one-hit-kill capable), immune to Grip (except Crush), Lightning stun, and Push/Pull knockback, and shoves enemies aside by running into them.
- **Wookiee Strength 2** gives 1.15x movement speed while holding melee; Strength 3 gives 1.20x.

## Counters

- Flamethrower / fire grenades (double damage).
- Sustained ranged fire from outside slap range.
- Droidekas (the Wookiee can't easily close on a shielded/firing Deka).

---

`rebel` · `melee` · `tank` · `brawler`

<!-- icon-suggestion: wook -->
