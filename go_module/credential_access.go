package main

import (
	"fmt"
	"net/url"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
)

// Tests replace these only in isolated processes; normal runs use the native store.
var readNativeCredential = LoadGitHubToken
var writeNativeCredential = SaveGitHubToken

// runCredentialWork is entered on the UI thread. Native unlock prompts must never
// hold the editor's event loop; one in-flight operation prevents repeated prompts.
func (a *App) runCredentialWork(work func() (string, error), done func(string, error)) {
	if a.credentialBusy {
		done("", fmt.Errorf("an OS credential operation is still pending; local editing remains available"))
		return
	}
	a.credentialBusy = true
	a.credentialWarning = "Waiting for the OS credential store. You can keep editing locally."
	go func() {
		token, err := work()
		fyne.Do(func() {
			a.credentialBusy = false
			if err != nil {
				a.credentialWarning = "The OS credential store is locked or unavailable. Unlock it and retry Connect; local editing remains available."
			} else {
				a.credentialWarning = ""
			}
			done(token, err)
		})
	}()
}

func (a *App) refreshGitHubConnection() {
	a.githubManager = nil
	if a.config.GitHubToken != "" {
		a.githubManager = NewGitHubManager(a.config.GitHubToken, a.config.TextAssetsPath)
	}
}

func (a *App) loadCredentials(done func(error)) {
	pending := a.legacyTokenPending
	a.runCredentialWork(func() (string, error) {
		if pending != "" {
			return migrateLegacyGitHubToken(pending, readNativeCredential, writeNativeCredential)
		}
		return readNativeCredential()
	}, func(token string, err error) {
		if err != nil {
			done(err)
			return
		}
		a.config.GitHubToken = token
		if pending != "" && pending == a.legacyTokenPending {
			a.legacyCredentialSecured = true
			if err := a.persistConfig(); err != nil {
				a.credentialWarning = "Credential secured, but the original configuration could not be updated. Retry Connect before changing or deleting the credential."
				done(err)
				return
			}
		}
		a.refreshGitHubConnection()
		done(nil)
	})
}

func (a *App) storeCredential(token string, done func(error)) {
	// Commit removal of legacy plaintext before replacement/deletion, otherwise
	// a future retry or restart could resurrect the obsolete credential.
	if a.legacyTokenPending != "" {
		a.loadCredentials(func(err error) {
			if err != nil { done(err); return }
			a.storeCredential(token, done)
		})
		return
	}
	a.runCredentialWork(func() (string, error) {
		return token, writeNativeCredential(token)
	}, func(stored string, err error) {
		if err == nil {
			a.config.GitHubToken = stored
			a.refreshGitHubConnection()
		}
		done(err)
	})
}

// showCredentialConnect uses a separate, non-modal window. Even an unanswered OS
// unlock request leaves the main editor available and never asks for a token again
// when the operator can use the one already stored securely.
func (a *App) showCredentialConnect(continueAction func()) {
	win := a.fyneApp.NewWindow("Connect GitHub — optional contribution access")
	closed := false
	win.SetOnClosed(func() { closed = true })
	status := widget.NewLabel("Use your saved credential, or store a new token. Local editing never needs an account.")
	status.Wrapping = fyne.TextWrapWord
	entry := NewPasswordInputEntry()
	entry.SetPlaceHolder("New token (only for replacement or first-time setup)")
	finish := func(err error) {
		if closed { return }
		if err != nil {
			status.SetText(a.credentialWarning + "\n" + err.Error())
			return
		}
		entry.SetText("")
		if a.config.GitHubToken == "" {
			status.SetText("No credential is stored. Add a token to contribute, or close this window and keep editing locally.")
			return
		}
		status.SetText("Secure credential ready. Local asset paths and documents are unchanged.")
		if continueAction != nil {
			win.Close()
			continueAction()
		}
	}
	useSaved := widget.NewButton("Use saved credential", func() {
		status.SetText("Waiting for the OS credential store. The main editor remains available.")
		a.loadCredentials(finish)
	})
	useSaved.Importance = widget.HighImportance
	store := widget.NewButton("Store new token", func() {
		if entry.Text == "" { status.SetText("Enter a new token, or choose Use saved credential."); return }
		status.SetText("Storing securely. The main editor remains available.")
		a.storeCredential(entry.Text, finish)
	})
	remove := widget.NewButton("Remove stored credential", func() {
		dialog.ShowConfirm("Remove GitHub access?", "Remove only Foundry's stored credential? Local files and projects are unchanged.", func(yes bool) {
			if !yes { return }
			status.SetText("Removing the stored credential. The main editor remains available.")
			a.storeCredential("", finish)
		}, win)
	})
	getToken := widget.NewButton("Create a GitHub token", func() {
		if address, err := url.Parse("https://github.com/settings/tokens"); err == nil { a.fyneApp.OpenURL(address) }
	})
	win.SetContent(container.NewPadded(container.NewVBox(status, useSaved, widget.NewSeparator(), entry,
		container.NewGridWithColumns(2, store, getToken), remove)))
	win.Resize(fyne.NewSize(620, 330))
	win.Show()
}
