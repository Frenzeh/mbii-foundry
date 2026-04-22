package main

import (
	"errors"
	"fmt"
	"log"
	"os"
	"runtime/debug"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/dialog"
)

var LogFile *os.File

func InitLogger() {
	// Use platform-appropriate temp directory (works on Windows, macOS, Linux)
	logPath := os.TempDir() + string(os.PathSeparator) + "mbii-foundry.log"

	f, err := os.OpenFile(logPath, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		fmt.Println("Failed to open log file:", err)
		return
	}
	LogFile = f
	log.SetOutput(f)
	log.Println("------------------------------------------------")
	log.Printf("MBII Foundry Started at %s", time.Now().Format(time.RFC3339))
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
