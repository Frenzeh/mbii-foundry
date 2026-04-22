# MBII Foundry — User Guide

## Getting Started
1.  **Open a File:** Click the Folder icon or drag-and-drop an `.mbch` (Character), `.sab` (Saber), `.veh` (Vehicle), or `.siege` file.
2.  **Create New:** Click the `+` icon to start a fresh file.
3.  **Workspace:**
    *   **Center:** Your main editor tabs. You can have multiple files open.
    *   **Left:** Asset Browser. Browse game assets (`.pk3` contents). Double-click to insert paths into fields.
    *   **Right:** Info Panel. Shows details about the field you are currently editing.

## Features

### Asset Browser
*   **Search:** Type in the search bar to filter assets (e.g., "stormtrooper").
*   **Usage:** Double-clicking an asset (like a model `.glm` or icon image) will automatically fill the currently focused text field in the editor.

### Bulk Editor
*   Allows you to modify multiple files at once.
*   **Use Case:** "I want to change the `ClassNumberLimit` to 1 for ALL my Jedi characters."
*   Select the folder, choose the field, set the value, and click Apply.

### Info Panel & Glossary
*   The panel on the right explains what obscure fields like `classflags` or `uishader` actually do.
*   You can browse the full glossary by typing in the search box at the bottom of the panel.

## Tips
*   **Source View:** Check the "Source" tab in the Character Editor to see the raw text file being generated.
*   **Point Buy:** The "Point Buy" tab calculates the cost of your custom skills automatically.
