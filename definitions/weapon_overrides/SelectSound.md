# SelectSound

`SelectSound sound/weapons/foo/select.wav`

> Sound played when the weapon is drawn (selected from inventory).

## What it does

Replaces the engine-default "weapon ready" sound. Plays once when the weapon is brought up.

## Valid values

Path to a `.wav` or `.mp3` under `sound/`.

## Notes

- `ChargeSound` is the looping charge-up sound — different concept.
- `FlashSound0..3` covers per-shot fire sounds.

---

`mbch` · `weapon` · `override` · `sound`
