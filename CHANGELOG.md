# Changelog

All notable changes to MBII Foundry are documented here. Format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/). Versions follow
[Semantic Versioning](https://semver.org/) where practical, with alpha/beta
pre-release suffixes until the project stabilizes.

## [Unreleased]

### Added
- CI workflow (build + vet + gofmt + test across Linux/macOS/Windows)
- Release workflow (triggered on `v*` tags, builds + publishes binaries)
- Issue templates (bug, feature, definition error) + PR template
- Dependabot config for Go modules and GitHub Actions
- Self-contained parser round-trip tests
- README install badges + "Where do I find .mbch files to edit?" section

### Fixed
- `go vet` warning in `logger.go` (non-constant format string in panic handler)
- `gofmt` applied across the entire Go codebase

### Changed
- `TestParseRealFiles` now skips cleanly when a TextAssets checkout isn't
  available (was previously relying on hardcoded relative paths)

---

## [0.1.0-alpha] — 2026-04-22

Initial public alpha.

### Added
- Visual editor for `.mbch`, `.sab`, `.veh`, `.siege` files (Fyne GUI)
- Curated enum metadata: 278 attributes, 53 weapons, 25 classes, 8 saber
  styles, 18 class flags, force powers, holdables
- Per-enum markdown documentation (371 polished, 164 stubs pending)
- Asset browser for PK3 contents
- Legends point-buy simulator
- Modpack packager
- macOS `.app` bundle build script
- Optional Holocron AI integration (pairs with a local backend if present)

### Known Issues
- 30.7% of enum definitions are AI-generated stubs awaiting human prose
  (see `docs/DEFINITIONS_GUIDE.md`)
- No prebuilt binaries in this release — build from source
