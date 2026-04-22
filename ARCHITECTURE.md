# MBII Foundry Architecture

## Core Concepts

### Multi-Document Interface (MDI)
The application uses a `DocTabs` container in `main.go`.
*   **`App` struct**: Holds the central `docTabs` and a map `editors` linking tabs to their `Editor` instances.
*   **`Editor` Interface** (`common.go`): All editors (MBCH, SAB, VEH, Siege) must implement:
    *   `LoadFile(path)` / `SaveFile(path)`
    *   `GetContent()` (UI)
    *   `SetOnHover` (Link to InfoPanel)

### Editor Structure
Each editor (e.g., `MBCHEditor`) follows a standard pattern:
1.  **Data Struct**: Matches the file format (e.g., `MBCHCharacter`).
2.  **UI Components**: Fyne widgets (`Entry`, `Select`, `Check`).
3.  **Parsing Logic**: `parseContent` (Regex-based) and `WriteContent` (fmt.Fprintf).

## ⚠️ CRITICAL DEVELOPMENT RULES ⚠️

### 1. Regex & String Literals in Go
**THE PROBLEM:** The AI tools (`write_file`, `replace`) often corrupt Go string literals containing backslashes, especially regex patterns (e.g., `\s`, `\w`, `\d`).
*   Example failure: `regexp.MustCompile("(\w+)")` becomes `regexp.MustCompile("(\\w+)")` -> Compile Error "unknown escape".

**THE SOLUTION:**
*   **NEVER** edit `.go` files containing regex using standard tools if possible.
*   **ALWAYS** use a **Python Script** via `run_shell_command` to write or modify these files.
*   Use `chr(96)` for backticks in Python generation scripts to avoid shell conflicts.

### 2. File Parsing
*   Current parsers are **lossy**. They strip comments to parse data.
*   Future Goal: Non-destructive AST-based parsing to preserve user comments.

## Directory Structure
*   `go_module/`: Source code.
*   `definitions/`: Markdown files for the Info Panel.
*   `definitions/glossary/`: User-friendly explanations of fields.
