# No Jetpack Cooldown

`CFL_NO_JETPACK_COOLDOWN`

> Removes the inter-burst cooldown on the jetpack.

## What it does

`g_items.c:1965` — normally, after a jetpack burst the player waits for a cooldown timer before firing again. When flagged, the check is skipped and the jetpack can be re-engaged immediately. Enables "skate" patterns and rapid-tap flight.

## Notes

- Only meaningful on Mandalorian-class (`isMando`) characters or others explicitly granted a jetpack.
- Companion flag `CFL_NO_FUEL_USE` makes the jetpack free (infinite Fuel); `CFL_NO_JETPACK_OVERHEAT` removes the overheat throttle.
- Elite Mando FA picks (Jango, Boba, Pre Vizsla) commonly carry this.

---

`mando` · `jetpack` · `mobility`
