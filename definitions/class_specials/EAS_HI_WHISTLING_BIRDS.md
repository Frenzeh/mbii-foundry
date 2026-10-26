# Whistling Birds

`EAS_HI_WHISTLING_BIRDS`

> Locks and fires a swarm of homing Whistling Birds.

## What it does

Calls `G_WhistlingBirdsLock`, which pings the local cone for valid targets and then unleashes a homing-projectile barrage. Each bird tracks an individual target. Designed for 1-vs-many cleanup against tightly-packed enemies.

## Notes

- Capacity (number of birds), lock window, and recharge live on `MB_ATT_WHISTLINGBIRD`.
- Cooldown is per-volley, not per-bird — lock and fire as one motion.
- Birds have limited tracking; line of sight at the lock moment matters more than aim during the volley.

---

`special` · `mandalorian` · `homing` · `barrage`

<!-- icon-suggestion: whistling-birds -->
