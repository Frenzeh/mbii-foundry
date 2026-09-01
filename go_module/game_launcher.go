package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
	"github.com/Frenzeh/mbii-foundry/parsers"
)

// LaunchInGameTest stages the character as a loose .mbch and boots OpenJK with fs_dirbeforepak 1.
func LaunchInGameTest(parent fyne.Window, char *parsers.MBCHCharacter, gamedataPath string) {
	if char == nil {
		dialog.ShowInformation("Test in MBII", "No active character loaded to test.", parent)
		return
	}
	if gamedataPath == "" {
		dialog.ShowInformation("Test in MBII", "GameData path is not configured. Please set it in Settings.", parent)
		return
	}

	// 1. Stage loose character file to GameData/MBII/ext_data/mb2/character/foundry_live_test.mbch
	targetDir := filepath.Join(gamedataPath, "MBII", "ext_data", "mb2", "character")
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		dialog.ShowError(fmt.Errorf("failed to create loose staging directory: %w", err), parent)
		return
	}

	targetFile := filepath.Join(targetDir, "foundry_live_test.mbch")
	mbchContent, err := parsers.GenerateMBCH(char)
	if err != nil {
		dialog.ShowError(fmt.Errorf("failed to generate character content: %w", err), parent)
		return
	}

	if err := os.WriteFile(targetFile, []byte(mbchContent), 0644); err != nil {
		dialog.ShowError(fmt.Errorf("failed to write test character: %w", err), parent)
		return
	}

	// 2. Locate OpenJK / Game binary
	binPath := findGameBinary(gamedataPath)
	if binPath == "" {
		showBinaryNotFoundDialog(parent, gamedataPath, targetFile)
		return
	}

	// 3. Build launch arguments with fs_dirbeforepak 1
	modelArg := "kyle/default"
	if char.Model != "" {
		skin := char.Skin
		if skin == "" {
			skin = "default"
		}
		modelArg = fmt.Sprintf("%s/%s", char.Model, skin)
	}

	args := []string{
		"+set", "fs_game", "MBII",
		"+set", "fs_dirbeforepak", "1",
		"+devmap", "mb2_dotf",
		"+model", modelArg,
	}

	cmd := exec.Command(binPath, args...)
	cmd.Dir = gamedataPath

	if err := cmd.Start(); err != nil {
		dialog.ShowError(fmt.Errorf("failed to launch game: %w", err), parent)
		return
	}

	showLaunchSuccessDialog(parent, targetFile, modelArg)
}

func findGameBinary(gamedata string) string {
	candidates := []string{}

	switch runtime.GOOS {
	case "darwin":
		candidates = append(candidates,
			"/Applications/OpenJK.app/Contents/MacOS/openjk.x86_64",
			"/Applications/Jedi Academy.app/Contents/MacOS/openjk.x86_64",
			filepath.Join(gamedata, "OpenJK.app", "Contents", "MacOS", "openjk.x86_64"),
			filepath.Join(gamedata, "openjk.x86_64"),
		)
	case "windows":
		candidates = append(candidates,
			filepath.Join(gamedata, "openjk.x86.exe"),
			filepath.Join(gamedata, "openjk.x64.exe"),
			filepath.Join(gamedata, "jamp.exe"),
			filepath.Join(gamedata, "MBII.exe"),
		)
	default: // linux
		candidates = append(candidates,
			filepath.Join(gamedata, "openjk.x86_64"),
			filepath.Join(gamedata, "openjk.i386"),
			filepath.Join(gamedata, "mbii.x86_64"),
		)
	}

	for _, c := range candidates {
		if info, err := os.Stat(c); err == nil && !info.IsDir() {
			return c
		}
	}
	return ""
}

func showLaunchSuccessDialog(parent fyne.Window, stagedFile, modelArg string) {
	infoText := `🚀 Game Launched Successfully!

Staged Character:
` + stagedFile + `

Engine Parameters:
+set fs_game MBII +set fs_dirbeforepak 1 +devmap mb2_dotf +model ` + modelArg + `

💡 How Fast In-Game Hot-Testing Works (fs_dirbeforepak 1):
OpenJK's 'fs_dirbeforepak 1' engine cvar instructs the filesystem to prioritize loose files in 'MBII/ext_data/mb2/character/' ahead of packed .pk3 archives.

Foundry stages your character directly into your loose GameData folder and launches the game engine into a local test arena ('mb2_dotf'), allowing instant testing in under 2 seconds without packing any PK3 archives!`

	msg := widget.NewLabel(infoText)
	msg.Wrapping = fyne.TextWrapWord

	dlg := dialog.NewCustom("Test in MBII (Live Hot Test)", "OK", container.NewPadded(msg), parent)
	dlg.Resize(fyne.NewSize(620, 360))
	dlg.Show()
}

func showBinaryNotFoundDialog(parent fyne.Window, gamedata, stagedFile string) {
	infoText := `Staged Character for In-Game Testing!

The character has been written as a loose file to:
` + stagedFile + `

To test manually in OpenJK / MBII, launch with:
+set fs_game MBII +set fs_dirbeforepak 1 +devmap mb2_dotf

💡 Why fs_dirbeforepak 1?
This engine cvar forces OpenJK to read the loose .mbch file directly from your MBII folder instead of needing to zip it into a .pk3 archive first!`

	msg := widget.NewLabel(infoText)
	msg.Wrapping = fyne.TextWrapWord

	dlg := dialog.NewCustom("Live Staging Complete (Manual Launch)", "Close", container.NewPadded(msg), parent)
	dlg.Resize(fyne.NewSize(580, 320))
	dlg.Show()
}
