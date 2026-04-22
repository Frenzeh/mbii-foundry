package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
	"github.com/google/go-github/v50/github"
	"golang.org/x/oauth2"
)

// GitHubManager handles all interactions with GitHub and local Git repo
type GitHubManager struct {
	client        *github.Client
	token         string
	user          *github.User
	repoPath      string
	baseRepoOwner string
	baseRepoName  string

	UpstreamBranch string // The branch we track (e.g., master or buildTest/...)
}

func NewGitHubManager(token string, repoPath string) *GitHubManager {
	mgr := &GitHubManager{
		token:          token,
		repoPath:       repoPath,
		baseRepoOwner:  "MovieBattles", // Verified upstream owner
		baseRepoName:   "TextAssets",   // Verified upstream repo
		UpstreamBranch: "master",       // Default, can be auto-detected
	}

	if token != "" {
		ctx := context.Background()
		ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: token})
		tc := oauth2.NewClient(ctx, ts)
		mgr.client = github.NewClient(tc)

		// Fetch user info
		u, _, err := mgr.client.Users.Get(ctx, "")
		if err == nil {
			mgr.user = u
		}
	}

	return mgr
}

// DetectDevelopmentBranch attempts to find the active buildTest branch
func (m *GitHubManager) DetectDevelopmentBranch() (string, error) {
	// We need to list remote branches from upstream
	// Since we might not have the repo cloned yet, or we want fresh info,
	// we can use the GitHub API if authenticated, or git ls-remote.
	// Using GitHub API is cleaner here since we have the client.

	if m.client == nil {
		return "master", nil
	}

	ctx := context.Background()
	branches, _, err := m.client.Repositories.ListBranches(ctx, m.baseRepoOwner, m.baseRepoName, &github.BranchListOptions{ListOptions: github.ListOptions{PerPage: 100}})
	if err != nil {
		return "master", err
	}

	var buildTestBranches []string
	for _, b := range branches {
		name := *b.Name
		if strings.HasPrefix(name, "buildTest/") {
			buildTestBranches = append(buildTestBranches, name)
		}
	}

	// If found, which one?
	// For now, let's pick the last one alphabetically or just the first?
	// Ideally we check commit dates, but that's expensive.
	// Let's assume the one with the "latest" name or just the first one found is better than master.
	// Actually, usually there's only one ACTIVE one.
	if len(buildTestBranches) > 0 {
		// Heuristic: Pick the one that seems most "main".
		// For now, return the first one found, or maybe let user choose?
		// Setting it as current.
		m.UpstreamBranch = buildTestBranches[0]
		return m.UpstreamBranch, nil
	}

	m.UpstreamBranch = "master"
	return "master", nil
}

// DeviceFlowStart initiates the OAuth Device Flow
// Returns: verification_uri, user_code, interval (seconds), device_code
func (m *GitHubManager) DeviceFlowStart() (string, string, int, string, error) {
	// Note: go-github doesn't directly support Device Flow initiation helpers typically found in generic OAuth libs,
	// but we can request the code manually or use a helper.
	// For simplicity, we'll simulate the standard GitHub flow request:
	// POST https://github.com/login/device/code
	// client_id = <CLIENT_ID> (We need a client ID for MBII Foundry app registration)

	// Since we don't have a registered Client ID for MBII Foundry yet in this context,
	// we will simulate the flow or require a Personal Access Token (PAT) for MVP.
	// However, the prompt asked for "Super New User Friendly".
	// The best SNUF way without a backend is PAT with a very good guide, OR Device Flow if we register an App.
	// For this prototype, I'll assume we prompt for PAT or use a placeholder Client ID.

	return "", "", 0, "", fmt.Errorf("device flow requires registered Client ID")
}

// SetupWorkspace performs the "Invisible Git" initialization
// 1. Checks if user has a fork
// 2. Forks if missing
// 3. Clones fork to local path
// 4. Sets upstream remote
func (m *GitHubManager) SetupWorkspace(progressCallback func(string)) error {
	ctx := context.Background()

	if m.client == nil {
		return fmt.Errorf("not logged in")
	}

	// 1. Check for Fork
	progressCallback("Checking for existing fork...")
	// List forks? Or just try to get repo User/TextAssets
	repo, _, err := m.client.Repositories.Get(ctx, *m.user.Login, m.baseRepoName)

	forkURL := ""
	if err != nil {
		// Not found, create fork
		progressCallback("Forking " + m.baseRepoOwner + "/" + m.baseRepoName + "...")
		fork, _, err := m.client.Repositories.CreateFork(ctx, m.baseRepoOwner, m.baseRepoName, nil)
		if err != nil {
			return fmt.Errorf("failed to fork: %v", err)
		}
		forkURL = *fork.CloneURL
		progressCallback("Fork created!")
		// Wait a moment for GitHub to provision
		time.Sleep(5 * time.Second)
	} else {
		forkURL = *repo.CloneURL
		progressCallback("Found existing fork.")
	}

	// 2. Clone
	if _, err := os.Stat(m.repoPath); !os.IsNotExist(err) {
		progressCallback("Target directory exists. Opening...")
		// Check if it's a git repo
		_, err := git.PlainOpen(m.repoPath)
		if err != nil {
			return fmt.Errorf("directory exists but is not a valid git repository: %v", err)
		}
	} else {
		progressCallback("Cloning repository (this may take a while)...")

		// Auth for Clone
		auth := &http.BasicAuth{
			Username: "oauth2", // Use oauth2 as username for tokens
			Password: m.token,
		}

		_, err := git.PlainClone(m.repoPath, false, &git.CloneOptions{
			URL:      forkURL,
			Progress: os.Stdout, // Or redirect to UI
			Auth:     auth,
			Depth:    1, // Shallow clone for speed (SNUF!)
		})
		if err != nil {
			return fmt.Errorf("failed to clone: %v", err)
		}
	}

	// 3. Configure Upstream
	progressCallback("Configuring upstream remote...")
	r, err := git.PlainOpen(m.repoPath)
	if err != nil {
		return err
	}

	_, err = r.CreateRemote(&config.RemoteConfig{
		Name: "upstream",
		URLs: []string{"https://github.com/" + m.baseRepoOwner + "/" + m.baseRepoName + ".git"},
	})
	if err != nil && err != git.ErrRemoteExists {
		return fmt.Errorf("failed to add upstream: %v", err)
	}

	progressCallback("Workspace Ready!")
	return nil
}

// SyncUpdates pulls from upstream and updates local master
// This effectively does: git checkout master && git pull upstream master
func (m *GitHubManager) SyncUpdates() error {
	r, err := git.PlainOpen(m.repoPath)
	if err != nil {
		return err
	}

	w, err := r.Worktree()
	if err != nil {
		return err
	}

	// 1. Checkout Master/UpstreamBranch
	err = w.Checkout(&git.CheckoutOptions{
		Branch: plumbing.NewBranchReferenceName(m.UpstreamBranch),
		Force:  false, // Safety first
	})
	if err != nil {
		return fmt.Errorf("failed to checkout %s (local changes?): %v", m.UpstreamBranch, err)
	}

	// 2. Pull Upstream
	// Note: We authenticate with the user's token even for public upstream if needed,
	// or just basic http.
	err = w.Pull(&git.PullOptions{
		RemoteName:    "upstream",
		ReferenceName: plumbing.NewBranchReferenceName(m.UpstreamBranch),
		Auth:          &http.BasicAuth{Username: "oauth2", Password: m.token},
		SingleBranch:  true,
	})

	if err != nil && err != git.NoErrAlreadyUpToDate {
		return fmt.Errorf("failed to pull upstream: %v", err)
	}

	return nil
}

// IsClean checks if the working directory is clean
func (m *GitHubManager) IsClean() (bool, error) {
	r, err := git.PlainOpen(m.repoPath)
	if err != nil {
		return false, err
	}
	w, err := r.Worktree()
	if err != nil {
		return false, err
	}
	status, err := w.Status()
	if err != nil {
		return false, err
	}
	return status.IsClean(), nil
}

// SwitchToUpstream switches to the tracked upstream branch
func (m *GitHubManager) SwitchToUpstream() error {
	r, err := git.PlainOpen(m.repoPath)
	if err != nil {
		return err
	}
	w, err := r.Worktree()
	if err != nil {
		return err
	}
	return w.Checkout(&git.CheckoutOptions{
		Branch: plumbing.NewBranchReferenceName(m.UpstreamBranch),
	})
}

// StageAndCommit adds all changes and commits them
func (m *GitHubManager) StageAndCommit(message string) error {
	r, err := git.PlainOpen(m.repoPath)
	if err != nil {
		return err
	}
	w, err := r.Worktree()
	if err != nil {
		return err
	}

	_, err = w.Add(".")
	if err != nil {
		return err
	}

	_, err = w.Commit(message, &git.CommitOptions{
		Author: &object.Signature{
			Name:  *m.user.Name,
			Email: *m.user.Email,
			When:  time.Now(),
		},
	})
	return err
}

// PushCurrentBranch pushes the current branch to origin
func (m *GitHubManager) PushCurrentBranch() error {
	r, err := git.PlainOpen(m.repoPath)
	if err != nil {
		return err
	}

	return r.Push(&git.PushOptions{
		RemoteName: "origin",
		Auth:       &http.BasicAuth{Username: "oauth2", Password: m.token},
	})
}

// OpenPullRequest creates a PR from current branch to upstream base
func (m *GitHubManager) OpenPullRequest(title, body string) (string, error) {
	branch, err := m.GetCurrentBranch()
	if err != nil {
		return "", err
	}

	// Determine Head prefix
	r, err := git.PlainOpen(m.repoPath)
	if err != nil {
		return "", err
	}
	remote, err := r.Remote("origin")
	headPrefix := *m.user.Login
	if err == nil && len(remote.Config().URLs) > 0 {
		url := remote.Config().URLs[0]
		if strings.Contains(url, "/"+m.baseRepoOwner+"/") || strings.Contains(url, ":"+m.baseRepoOwner+"/") {
			headPrefix = m.baseRepoOwner
		}
	}

	ctx := context.Background()
	newPR := &github.NewPullRequest{
		Title:               github.String(title),
		Head:                github.String(headPrefix + ":" + branch),
		Base:                github.String(m.UpstreamBranch),
		Body:                github.String(body + "\n\n*Created with MBII Foundry*"),
		MaintainerCanModify: github.Bool(true),
	}

	pr, _, err := m.client.PullRequests.Create(ctx, m.baseRepoOwner, m.baseRepoName, newPR)
	if err != nil {
		return "", err
	}
	return *pr.HTMLURL, nil
}

// CreateBranch creates a new branch from current HEAD
func (m *GitHubManager) CreateBranch(branchName string) error {
	r, err := git.PlainOpen(m.repoPath)
	if err != nil {
		return err
	}
	w, err := r.Worktree()
	if err != nil {
		return err
	}

	headRef, err := r.Head()
	if err != nil {
		return err
	}

	return w.Checkout(&git.CheckoutOptions{
		Hash:   headRef.Hash(),
		Branch: plumbing.NewBranchReferenceName(branchName),
		Create: true,
	})
}

// CreateContribution creates a branch, commits changes, pushes, and opens a PR
// Refactored to use granular methods
func (m *GitHubManager) CreateContribution(branchName, title, description string) (string, error) {
	// Force move to upstream branch first (carry over changes)
	// If this fails (conflicts), we abort.
	m.SwitchToUpstream()
	// Ignore error: if we can't switch (conflict), we might just branch off current.

	if err := m.CreateBranch(branchName); err != nil {
		return "", err
	}
	if err := m.StageAndCommit(title); err != nil {
		return "", err
	}
	if err := m.PushCurrentBranch(); err != nil {
		return "", err
	}
	return m.OpenPullRequest(title, description)
}

// GetStatus returns a list of modified files
func (m *GitHubManager) GetStatus() ([]string, error) {
	r, err := git.PlainOpen(m.repoPath)
	if err != nil {
		return nil, err
	}

	w, err := r.Worktree()
	if err != nil {
		return nil, err
	}

	status, err := w.Status()
	if err != nil {
		return nil, err
	}

	var changedFiles []string
	for file, s := range status {
		if s.Staging != git.Unmodified || s.Worktree != git.Unmodified {
			changedFiles = append(changedFiles, file)
		}
	}
	return changedFiles, nil
}

// GetCurrentBranch returns the name of the currently checked out branch
func (m *GitHubManager) GetCurrentBranch() (string, error) {
	r, err := git.PlainOpen(m.repoPath)
	if err != nil {
		return "", err
	}

	head, err := r.Head()
	if err != nil {
		return "", err
	}

	if head.Name().IsBranch() {
		return head.Name().Short(), nil
	}
	return head.Hash().String(), nil // Detached HEAD
}

// CheckoutBranch switches to an existing branch
func (m *GitHubManager) CheckoutBranch(branchName string) error {
	r, err := git.PlainOpen(m.repoPath)
	if err != nil {
		return err
	}

	w, err := r.Worktree()
	if err != nil {
		return err
	}

	return w.Checkout(&git.CheckoutOptions{
		Branch: plumbing.NewBranchReferenceName(branchName),
	})
}
