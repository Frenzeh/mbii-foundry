# Resource Regen Cap

`MB_ATT_RESOURCE_REGEN_CAP`

> Ceiling above which the class resource stops auto-regenerating.

## What it does

Caps the resource regen at a configured value. Once the pool reaches this value, regen pauses until usage drops it back below.

## Per level

- **Level 1** — low cap.
- **Level 2** — medium cap.
- **Level 3** — full pool cap.

## Notes

- 0 means cap to the configured maximum for the class's resource.
- Sibling of `MB_ATT_RESOURCE_REGEN_AMOUNT` and `MB_ATT_RESOURCE_REGEN_RATE`.

---

`regen` · `resource` · `passive` · `cap`

<!-- icon-suggestion: regen-cap -->
