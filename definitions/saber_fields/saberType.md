# Saber Type

`saberType`

> Selects the saber category stored in a SAB definition.

## Foundry support

The visual SAB form currently offers:

- `SABER_SINGLE`
- `SABER_STAFF`

This picker is a selected authoring surface, not an exhaustive list of constants
accepted by every engine revision. A different value already present in a
parsed file remains inspectable in Source; changing the picker replaces it with
one of the listed values.

New Foundry saber documents start with `SABER_SINGLE`. That is an application
default, not a verified statement about omitted-field behavior in the engine.

## Verification status

No exact engine revision or source range is recorded for the gameplay semantics
of this field. Verify additional constants and blade behavior against the
target engine checkout before documenting or relying on them.

---

`saber`
