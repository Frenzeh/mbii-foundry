package main

import (
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime/debug"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/dialog"
)

var LogFile *os.File

func InitLogger() {
	tempDir := os.TempDir()
	if tempDir == "" {
		fmt.Fprintln(os.Stderr, "Failed to open log file: temporary directory is unavailable")
		return
	}
	if err := initLoggerAt(filepath.Join(tempDir, "mbii-foundry.log")); err != nil {
		fmt.Fprintln(os.Stderr, "Failed to open log file:", err)
	}
}

func initLoggerAt(logPath string) error {
	if logPath == "" {
		return fmt.Errorf("log path is empty")
	}
	if info, err := os.Lstat(logPath); err == nil {
		if !info.Mode().IsRegular() {
			return fmt.Errorf("log path is not a regular file: %s", logPath)
		}
	} else if !os.IsNotExist(err) {
		return err
	}

	f, err := os.OpenFile(logPath, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0600)
	if err != nil {
		return err
	}
	closeWithError := func(err error) error {
		_ = f.Close()
		return err
	}
	pathInfo, err := os.Lstat(logPath)
	if err != nil {
		return closeWithError(err)
	}
	fileInfo, err := f.Stat()
	if err != nil {
		return closeWithError(err)
	}
	if !pathInfo.Mode().IsRegular() || !os.SameFile(pathInfo, fileInfo) {
		return closeWithError(fmt.Errorf("log path changed while opening: %s", logPath))
	}
	if err := f.Chmod(0600); err != nil {
		return closeWithError(err)
	}

	previous := LogFile
	LogFile = f
	log.SetOutput(f)
	if previous != nil {
		_ = previous.Close()
	}
	log.Println("------------------------------------------------")
	log.Printf("MBII Foundry Started at %s", time.Now().Format(time.RFC3339))
	return nil
}

func LogInfo(format string, v ...interface{}) {
	msg := fmt.Sprintf("[INFO] "+format, v...)
	log.Println(msg)
	fmt.Println(msg)
}

func LogError(format string, v ...interface{}) {
	msg := fmt.Sprintf("[ERROR] "+format, v...)
	log.Println(msg)
	fmt.Println(msg)
}

func ShowError(err error, win fyne.Window) {
	if err == nil {
		return
	}
	LogError("UI Error: %v", err)
	dialog.ShowError(err, win)
}

func SafeExecute(fn func(), win fyne.Window) {
	defer func() {
		if r := recover(); r != nil {
			stack := string(debug.Stack())
			LogError("PANIC: %v\nStack: %s", r, stack)
			msg := fmt.Sprintf("An unexpected error occurred:\n%v\n\nSee mbii-foundry.log for details.", r)
			dialog.ShowError(errors.New(msg), win)
		}
	}()
	fn()
}
