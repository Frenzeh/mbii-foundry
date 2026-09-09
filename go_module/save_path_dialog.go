package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/storage"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// ShowSavePathDialog chooses a destination without opening it. Fyne's file-save
// dialog supplies an already-open writer, which can truncate an existing file
// before an editor has generated or validated its replacement.
func ShowSavePathDialog(parent fyne.Window, title, defaultPath, extension string, save func(string) error) {
	directory, name := filepath.Split(defaultPath)
	if directory == "" {
		directory, _ = os.UserHomeDir()
		if directory == "" {
			directory, _ = os.Getwd()
		}
	}
	if name == "" {
		name = "Untitled" + extension
	}

	directoryEntry := widget.NewEntry()
	directoryEntry.SetText(filepath.Clean(directory))
	nameEntry := widget.NewEntry()
	nameEntry.SetText(name)
	message := widget.NewLabel("")
	message.Wrapping = fyne.TextWrapWord

	browse := widget.NewButtonWithIcon("Choose folder", theme.FolderOpenIcon(), func() {
		picker := dialog.NewFolderOpen(func(uri fyne.ListableURI, err error) {
			if err != nil {
				message.SetText(err.Error())
				return
			}
			if uri != nil {
				directoryEntry.SetText(uri.Path())
			}
		}, parent)
		if location, err := storage.ListerForURI(storage.NewFileURI(directoryEntry.Text)); err == nil {
			picker.SetLocation(location)
		}
		picker.Show()
	})
	var chooser *dialog.CustomDialog
	saveButton := widget.NewButtonWithIcon("Save", theme.DocumentSaveIcon(), func() {
		path, err := resolveSaveDestination(directoryEntry.Text, nameEntry.Text, extension)
		if err != nil {
			message.SetText(err.Error())
			return
		}
		commit := func() {
			if err := save(path); err != nil {
				message.SetText(err.Error())
				return
			}
			chooser.Hide()
		}
		info, err := os.Lstat(path)
		if err == nil {
			if !info.Mode().IsRegular() {
				message.SetText("Choose a regular file destination, not a directory or symbolic link.")
				return
			}
			dialog.ShowConfirm("Replace existing file?", fmt.Sprintf("Replace %s only after the new content has been validated and written successfully?", filepath.Base(path)), func(confirmed bool) {
				if confirmed {
					commit()
				}
			}, parent)
			return
		}
		if !os.IsNotExist(err) {
			message.SetText(err.Error())
			return
		}
		commit()
	})
	saveButton.Importance = widget.HighImportance
	content := container.NewVBox(
		widget.NewLabel("The existing file is not opened or changed while choosing a destination."),
		widget.NewForm(
			widget.NewFormItem("Folder", container.NewBorder(nil, nil, nil, browse, directoryEntry)),
			widget.NewFormItem("File name", nameEntry),
		),
		message,
		container.NewHBox(saveButton),
	)
	chooser = dialog.NewCustom(title, "Cancel", content, parent)
	chooser.Resize(fyne.NewSize(680, 260))
	chooser.Show()
	parent.Canvas().Focus(nameEntry)
}

func resolveSaveDestination(directory, name, extension string) (string, error) {
	directory = strings.TrimSpace(directory)
	name = strings.TrimSpace(name)
	if directory == "" {
		return "", fmt.Errorf("choose a destination folder")
	}
	if name == "" || name == "." || name == ".." || strings.ContainsAny(name, `/\\`) {
		return "", fmt.Errorf("enter a file name without directory separators")
	}
	if extension != "" {
		if !strings.HasPrefix(extension, ".") {
			extension = "." + extension
		}
		if filepath.Ext(name) == "" {
			name += extension
		} else if !strings.EqualFold(filepath.Ext(name), extension) {
			return "", fmt.Errorf("file name must end in %s", extension)
		}
	}
	absolute, err := filepath.Abs(directory)
	if err != nil {
		return "", err
	}
	info, err := os.Stat(absolute)
	if err != nil {
		return "", fmt.Errorf("destination folder: %w", err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("destination folder is not a directory")
	}
	return filepath.Join(absolute, name), nil
}
