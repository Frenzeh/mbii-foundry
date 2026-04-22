```
███╗   ███╗ ██████╗ ██╗██╗    ███████╗ ██████╗ ██╗   ██╗███╗   ██╗██████╗ ██████╗ ██╗   ██╗
████╗ ████║██╔════╝ ██║██║    ██╔════╝██╔═══██╗██║   ██║████╗  ██║██╔══██╗██╔══██╗╚██╗ ██╔╝
██╔████╔██║██║  ███╗██║██║    █████╗  ██║   ██║██║   ██║██╔██╗ ██║██║  ██║██████╔╝ ╚████╔╝
██║╚██╔╝██║██║   ██║██║██║    ██╔══╝  ██║   ██║██║   ██║██║╚██╗██║██║  ██║██╔══██╗  ╚██╔╝
██║ ╚═╝ ██║╚██████╔╝██║██║    ██║     ╚██████╔╝╚██████╔╝██║ ╚████║██████╔╝██║  ██║   ██║
╚═╝     ╚═╝ ╚═════╝ ╚═╝╚═╝    ╚═╝      ╚═════╝  ╚═════╝ ╚═╝  ╚═══╝╚═════╝ ╚═╝  ╚═╝   ╚═╝

                    W H E R E   M O V I E   B A T T L E S   I I   I S   F O R G E D
```

> A visual content editor for **Movie Battles II**. Shape classes, sabers, and vehicles without wrestling raw config syntax.

**Status:** Alpha — works end-to-end for `.mbch` editing, other formats rolling in. Rough edges expected; feedback welcome.

---

## What it does

MBII Foundry is a standalone desktop app that replaces hand-editing MBII's text-based content files. Open a class file, tick boxes for weapons and force powers, slide attribute levels — save back to a clean, valid file every time.

| File type | Editor | Notes |
|---|---|---|
| `.mbch` | Character / class | Primary focus. Full attribute grid, force-power picker, weapon selection, class flags, Legends point-buy simulator. |
| `.sab`  | Saber config | Hilt, blade, style, stats. |
| `.veh`  | Vehicle | Base stats, weapons, flags. |
| `.siege` | Siege / FA config | Round rules, class rosters. |

Plus:

- **Asset browser** — peek at models/textures/sounds directly from PK3s.
- **Modpack packager** — bundle your edits into a loadable pk3.
- **Validation** — prevents the common "typo in enum → crash on map load" class of bug.

## Who it's for

- **Content creators** building custom classes and modpacks.
- **Balance designers** iterating on FA attribute costs and class power levels.
- **Players** who want to tweak loadouts without learning the file format.

You do not need to be a programmer. You do not need any AI assistant or backend service. The app works on its own.

## Install

Binaries for Mac, Windows, and Linux are published under [Releases](https://github.com/Frenzeh/mbii-foundry/releases) when available. During alpha, you're likely building from source — see below.

## Build from source

Requires **Go 1.21+**. Fyne handles the native GUI, so no extra GUI toolkit install.

```bash
git clone https://github.com/Frenzeh/mbii-foundry.git
cd mbii-foundry/go_module
go build -o fa_creator        # fa_creator.exe on Windows
./fa_creator
```

macOS app bundle (with local code signing):

```bash
cd mbii-foundry
./build_app.sh
# Output: "FA Creator.app"
```

Details and caveats in [`USER_GUIDE.md`](USER_GUIDE.md).

## Contributing

Issues and PRs welcome. Priority areas during alpha:

- **Documentation prose** — many enum definitions under `definitions/<category>/*.md` are stubs waiting for descriptive text. These feed the in-app info panel and are the single highest-leverage contribution for a non-coder.
- **Bug reports** — especially anything that crashes the app or produces a `.mbch` that MBII won't load.
- **UI polish** — Fyne tooltips, layout tweaks, keyboard shortcuts.

Read [`CONTRIBUTING.md`](CONTRIBUTING.md) before opening a PR. Some MBII enums (`MB_ATT_*`, `WP_*`, etc.) are curated from the game's source — when new ones land, a maintainer regenerates the stubs and the community polishes the prose.

## Layout

```
mbii-foundry/
├── go_module/        Fyne GUI application (Go source)
├── data/             Curated enum metadata (JSON)
├── definitions/      Per-enum markdown documentation
│   ├── attributes/
│   ├── weapons/
│   ├── mb_classes/
│   ├── saber_styles/
│   └── …
├── schemas/          JSON Schemas for file validation
├── templates/        Starter files for new creations
├── parsers/          File-format parsers (shared library)
├── macos/            .app bundle resources
└── packager/         PK3 packaging
```

## License

Apache License 2.0. See [`LICENSE`](LICENSE).

## Acknowledgements

- **Fyne** — the Go-native GUI toolkit doing the heavy visual lifting.
- **The MBII dev team** — for building the game this tool serves.
- Community testers and documentation contributors.

---

*May the Force be with your builds.*
