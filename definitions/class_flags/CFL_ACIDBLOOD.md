# Acid Blood

`CFL_ACIDBLOOD`

> Damages nearby attackers with a poison splash when the carrier is hit / killed.

## What it does

Two code paths in `g_combat.c`:
- **On damage taken** — when flagged, nearby non-flagged enemies are dosed with acid (poison-like DoT), skipping enemies who are already poisoned or who themselves carry the flag (mutual immunity).
- **On death / explosion** — the acid-splash radius is triggered as part of the death effects chain.

## Notes

- Immunity: carriers of the flag do not acid-dose each other.
- Does not stack — enemies with `nAcidDosesLeft > 0` are skipped.
- Classic uses: Trandoshan, Noghri, Xenomorph-style characters.

---

`defense` · `dot` · `alien`
