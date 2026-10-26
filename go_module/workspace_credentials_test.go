package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
)

// Observe the real wizard's successful publication without a production test hook.
// Close returns only after the test window has finished its own teardown.
type workspaceCredentialTestWindow struct {
	fyne.Window
	closed chan struct{}
}

func (w *workspaceCredentialTestWindow) Close() {
	w.Window.Close()
	close(w.closed)
}

type workspaceCredentialTestApp struct {
	fyne.App
	window fyne.Window
	closed chan struct{}
}

func (a *workspaceCredentialTestApp) NewWindow(title string) fyne.Window {
	a.window = &workspaceCredentialTestWindow{Window: a.App.NewWindow(title), closed: a.closed}
	return a.window
}

func TestWorkspaceSetupCredentialOwnership(t *testing.T) {
	const initialToken = "synthetic-workspace-original-token"
	const replacementToken = "synthetic-workspace-replacement-token"
	const initialLogin = "fixture-contributor"
	const replacementLogin = "replacement-contributor"
	const upstreamBranch = "buildTest/workspace-fixture"
	for _, tc := range []struct {
		name  string
		token string
		login string
	}{
		{"replacement", replacementToken, replacementLogin},
		{"removal", "", ""},
		{"unchanged", initialToken, initialLogin},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// These existing hook/transport helpers are process-global; keep this test serial.
			auditCredentialHome(t)
			ui := test.NewApp()
			t.Cleanup(ui.Quit)
			observedUI := &workspaceCredentialTestApp{App: ui, closed: make(chan struct{})}
			root := t.TempDir()
			targetPath := filepath.Join(root, "TextAssets")
			previousPath := filepath.Join(root, "previous-workspace")
			repo, err := git.PlainInit(targetPath, false)
			if err != nil {
				t.Fatal(err)
			}
			forkURL := "https://github.com/" + initialLogin + "/TextAssets.git"
			if _, err := repo.CreateRemote(&config.RemoteConfig{Name: "origin", URLs: []string{forkURL}}); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(targetPath, "local-draft.txt"), []byte("keep local edits"), 0600); err != nil {
				t.Fatal(err)
			}
			store := &testCredentialStore{token: initialToken}
			installTestCredentialStore(t, store)

			branchStarted, releaseBranch := make(chan struct{}), make(chan struct{})
			release := sync.OnceFunc(func() { close(releaseBranch) })
			branchCalls := 0
			lastBranchAuthorization := ""
			auditUseCatalogTransport(t, func(req *http.Request) (*http.Response, error) {
				if req.Method != http.MethodGet || req.URL.Scheme != "https" || req.URL.Host != "api.github.com" {
					return nil, fmt.Errorf("unexpected workspace request: %s %s", req.Method, req.URL)
				}
				authorization := req.Header.Get("Authorization")
				login := ""
				switch authorization {
				case "Bearer " + initialToken:
					login = initialLogin
				case "Bearer " + replacementToken:
					login = replacementLogin
				default:
					return nil, fmt.Errorf("workspace request used an unexpected synthetic credential")
				}
				var body string
				switch req.URL.Path {
				case "/user":
					body = fmt.Sprintf(`{"login":%q,"id":123}`, login)
				case "/repos/" + initialLogin + "/TextAssets":
					body = fmt.Sprintf(`{"clone_url":%q}`, forkURL)
				case "/repos/MovieBattles/TextAssets/branches":
					branchCalls++
					lastBranchAuthorization = authorization
					if branchCalls == 1 {
						// SetupWorkspace has finished; publication must wait for this response.
						close(branchStarted)
						<-releaseBranch
					}
					body = fmt.Sprintf(`[{"name":%q}]`, upstreamBranch)
				default:
					return nil, fmt.Errorf("unexpected workspace API path: %s", req.URL.Path)
				}
				return &http.Response{
					StatusCode: http.StatusOK,
					Header:     http.Header{"Content-Type": []string{"application/json"}},
					Body:       io.NopCloser(strings.NewReader(body)),
					Request:    req,
				}, nil
			})

			a := &App{
				fyneApp:    observedUI,
				mainWindow: ui.NewWindow("Workspace credential fixture"),
				configPath: filepath.Join(root, "config.json"),
				config:     AppConfig{GitHubToken: initialToken, TextAssetsPath: previousPath},
			}
			a.refreshGitHubConnection()
			a.showWorkspaceSetupWizard()
			entryObject := auditAssetFind(observedUI.window.Content(), func(object fyne.CanvasObject) bool {
				_, ok := object.(*widget.Entry)
				return ok
			})
			if entryObject == nil {
				t.Fatal("workspace destination entry is missing")
			}
			entryObject.(*widget.Entry).SetText(targetPath)
			initialize := findButtonByText(observedUI.window.Content(), "Initialize Workspace")
			if initialize == nil {
				t.Fatal("workspace initialization action is missing")
			}
			t.Cleanup(func() {
				release()
				select {
				case <-observedUI.closed:
				case <-time.After(5 * time.Second):
					t.Error("workspace worker did not reach successful publication")
				}
			})
			test.Tap(initialize)
			select {
			case <-branchStarted:
			case <-time.After(5 * time.Second):
				t.Fatal("workspace setup did not reach the completion barrier")
			}
			if tc.token != initialToken {
				if err := awaitCredentialAction(t, func(done func(error)) { a.storeCredential(tc.token, done) }); err != nil {
					t.Fatal(err)
				}
			}
			release()
			select {
			case <-observedUI.closed:
			case <-time.After(5 * time.Second):
				t.Fatal("workspace setup did not publish its result")
			}

			if a.config.TextAssetsPath != targetPath {
				t.Fatalf("completed workspace path = %q, want %q", a.config.TextAssetsPath, targetPath)
			}
			var saved AppConfig
			data, err := os.ReadFile(a.configPath)
			if err != nil {
				t.Fatal(err)
			}
			if err := json.Unmarshal(data, &saved); err != nil {
				t.Fatal(err)
			}
			if saved.TextAssetsPath != targetPath {
				t.Fatalf("persisted workspace path = %q, want %q", saved.TextAssetsPath, targetPath)
			}
			if a.config.GitHubToken != tc.token || store.token != tc.token {
				t.Fatal("workspace completion replaced the current credential")
			}
			if tc.token == "" {
				if a.githubManager != nil {
					t.Fatal("workspace completion resurrected a manager after credential removal")
				}
				return
			}
			manager := a.githubManager
			if manager == nil {
				t.Fatal("workspace completion lost the current GitHub connection")
			}
			if clean, err := manager.IsClean(); err != nil || clean {
				t.Fatalf("published manager cannot see the new workspace's local draft: clean=%v, error=%v", clean, err)
			}
			if manager.UpstreamBranch != upstreamBranch {
				t.Fatalf("published manager lost the detected branch: %q", manager.UpstreamBranch)
			}
			if branch, err := manager.DetectDevelopmentBranch(); err != nil || branch != upstreamBranch {
				t.Fatalf("published connection cannot query upstream: branch=%q, error=%v", branch, err)
			}
			if lastBranchAuthorization != "Bearer "+tc.token {
				t.Fatal("published connection still authenticates with the setup-time credential")
			}
			if err := manager.ensureUser(); err != nil {
				t.Fatal(err)
			}
			if manager.user.GetLogin() != tc.login {
				t.Fatal("published connection retained the setup-time account identity")
			}
			err = manager.configureWorkspaceRemotes(repo)
			if tc.login != initialLogin {
				if err == nil {
					t.Fatal("replacement account accepted the previous account's fork")
				}
			} else if err != nil {
				t.Fatalf("unchanged account cannot use its initialized workspace: %v", err)
			}
		})
	}
}
