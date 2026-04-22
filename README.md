```
███╗   ███╗██████╗ ██╗██╗    ███████╗ ██████╗ ██╗   ██╗███╗   ██╗██████╗ ██████╗ ██╗   ██╗
████╗ ████║██╔══██╗██║██║    ██╔════╝██╔═══██╗██║   ██║████╗  ██║██╔══██╗██╔══██╗╚██╗ ██╔╝
██╔████╔██║██████╔╝██║██║    █████╗  ██║   ██║██║   ██║██╔██╗ ██║██║  ██║██████╔╝ ╚████╔╝
██║╚██╔╝██║██╔══██╗██║██║    ██╔══╝  ██║   ██║██║   ██║██║╚██╗██║██║  ██║██╔══██╗  ╚██╔╝
██║ ╚═╝ ██║██████╔╝██║██║    ██║     ╚██████╔╝╚██████╔╝██║ ╚████║██████╔╝██║  ██║   ██║
╚═╝     ╚═╝╚═════╝ ╚═╝╚═╝    ╚═╝      ╚═════╝  ╚═════╝ ╚═╝  ╚═══╝╚═════╝ ╚═╝  ╚═╝   ╚═╝

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

There are two install paths. **Most people want the Easy Path.** Only use "From Source" if you're on the bleeding edge or there's no prebuilt release for your platform yet.

### Easy Path — prebuilt download (not yet available)

1. Open the [Releases page](https://github.com/Frenzeh/mbii-foundry/releases).
2. Find the latest release → scroll to **Assets**.
3. Download the file for your OS:
   - macOS → `MBII.Foundry.app.zip`  → unzip → drag to **Applications**.
   - Windows → `mbii-foundry-windows.zip` → unzip anywhere → double-click `mbii-foundry.exe`.
   - Linux → `mbii-foundry-linux.tar.gz` → extract → `./mbii-foundry`.
4. Launch. First run will ask for your **MBII gamedata path** (the folder with the `MBII` subfolder inside your Jedi Academy install). Point it there.

> **Status note:** MBII Foundry is alpha; no prebuilt Releases exist yet. Until one does, use "From Source" below.

### From Source — for alpha testers and contributors

You need **Go 1.21 or newer** installed. Go is free and fast to set up.

**Install Go (one-time):**
- **macOS:** `brew install go` (or download from [go.dev/dl](https://go.dev/dl/))
- **Windows:** download the MSI from [go.dev/dl](https://go.dev/dl/) and run it
- **Linux (Ubuntu/Debian):** `sudo apt install golang` (Fedora/Arch users already know the drill)

Verify: open a terminal and run `go version`. If it prints a version, you're set.

**Build and launch MBII Foundry:**

```bash
git clone https://github.com/Frenzeh/mbii-foundry.git
cd mbii-foundry
./setup_mbii-foundry.sh      # builds the app (first time only, takes ~1 minute)
./run_mbii-foundry.sh        # launches the app
```

On **Windows**, use the same commands from a Git Bash or WSL terminal. If you're in PowerShell/cmd, run `cd go_module && go build -o mbii-foundry.exe` then double-click `mbii-foundry.exe`.

**Want a double-clickable Mac app?** After `setup_mbii-foundry.sh`, run `./build_app.sh`. You'll get `MBII Foundry.app` you can drag into Applications.

### Update to the latest version

Pull and rebuild:

```bash
cd mbii-foundry
git pull
./setup_mbii-foundry.sh     # rebuilds the binary with the new code
```

If you're editing your own branch, `git pull` won't work — `git fetch && git merge origin/main` instead. Ask an existing contributor if you're unsure.

### Getting help

- App crashes on launch? Check the log at `go_module/mbii-foundry.log` (or `mbii-foundry.log` next to the binary).
- Build failed? Re-read the Go version message (`go version` must print 1.21+) and file an [issue](https://github.com/Frenzeh/mbii-foundry/issues) with the error output.
- Gamedata path? You're looking for the folder that contains `base/` and `MBII/` subfolders — that's your Jedi Academy `GameData` install directory.

## Contributing

Issues and PRs welcome. Priority areas during alpha:

- **Writing / fixing enum documentation** — the highest-leverage contribution for anyone who isn't a programmer. Many `definitions/<category>/*.md` files are AI-generated stubs or placeholders. Editing them directly on GitHub (pencil icon → Propose changes) works; no git clone needed. Full walkthrough: [`docs/DEFINITIONS_GUIDE.md`](docs/DEFINITIONS_GUIDE.md).
- **Bug reports** — especially anything that crashes the app or produces a `.mbch` that MBII won't load.
- **UI polish** — Fyne tooltips, layout tweaks, keyboard shortcuts.
- **Code contributions** — see [`CONTRIBUTING.md`](CONTRIBUTING.md) for the developer guide.

Some MBII enums (`MB_ATT_*`, `WP_*`, etc.) are curated from the game's source — when new ones land, a maintainer regenerates the stubs and the community polishes the prose.

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
