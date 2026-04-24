# Wookiee Fury

`EAS_HI_FURY`

> Activates Wookiee berserk state.

## What it does

Triggers `Skill_Fury`, which sets the player's `FP_RAGE` flag and overrides animation timing for sped-up melee. Drains Rage continuously while active and grants knockback/damage bonuses through the Wookiee strength channel.

## Per level

(Cost is fixed at 35 Rage on activation; level scaling lives on `MB_ATT_WOOKIEE_FURY`.)

## Notes

- Cooldown: 3000 ms after the rage state ends.
- Stops automatically when Rage drains to 0 or the player toggles it off.
- Holding key uses `bCanStopWookieeFury` flag — release to deactivate cleanly.

---

`special` · `wookiee` · `melee` · `rage`

<!-- icon-suggestion: fury -->
