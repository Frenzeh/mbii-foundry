package main

import (
	"context"
	"errors"
	"fmt"
	nethttp "net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	gitclient "github.com/go-git/go-git/v5/plumbing/transport/client"
	githttp "github.com/go-git/go-git/v5/plumbing/transport/http"
	"github.com/google/go-github/v50/github"
	"golang.org/x/oauth2"
)

const (
	contributionUpstreamOwner = "MovieBattles"
	contributionRepository    = "TextAssets"
	contributionUpstreamURL   = "https://github.com/MovieBattles/TextAssets.git"
)

func init() {
	// Install once, before workers start: go-git's protocol registry is global.
	gitclient.InstallProtocol("https", githttp.NewClient(&nethttp.Client{
		Transport:     nethttp.DefaultTransport,
		CheckRedirect: checkContributionRedirect,
	}))
}

// GitHubManager handles all interactions with GitHub and local Git repo
type GitHubManager struct {
	client   *github.Client
	token    string
	user     *github.User
	repoPath string

	UpstreamBranch string // The branch we track (e.g., master or buildTest/...)
}

func NewGitHubManager(token string, repoPath string) *GitHubManager {
	mgr := &GitHubManager{
		token:          token,
		repoPath:       repoPath,
		UpstreamBranch: "master",
	}

	if token != "" {
		ctx := context.Background()
		ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: token})
		tc := oauth2.NewClient(ctx, ts)
		tc.Timeout = 30 * time.Second
		tc.CheckRedirect = checkContributionRedirect
		mgr.client = github.NewClient(tc)
	}

	return mgr
}

// checkContributionRedirect also covers OAuth requests: that transport reapplies
// credentials on each request, so net/http's header stripping is not sufficient.
func checkContributionRedirect(req *nethttp.Request, via []*nethttp.Request) error {
	if len(via) == 0 || len(via) >= 10 {
		return fmt.Errorf("contribution redirect limit reached")
	}
	origin := via[0].URL
	if req.URL.Scheme != "https" || origin.Scheme != "https" ||
		!strings.EqualFold(req.URL.Host, origin.Host) || req.URL.User != nil {
		return fmt.Errorf("refusing contribution redirect outside the original HTTPS origin")
	}
	return nil
}

func validateContributionURL(raw, expected string) error {
	u, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("invalid contribution repository URL")
	}
	want, err := url.Parse(expected)
	if err != nil {
		return fmt.Errorf("invalid expected contribution repository URL")
	}
	path := strings.TrimSuffix(strings.TrimSuffix(u.Path, "/"), ".git")
	wantPath := strings.TrimSuffix(want.Path, ".git")
	if u.Scheme != "https" || !strings.EqualFold(u.Host, "github.com") ||
		u.User != nil || u.Opaque != "" || u.RawPath != "" ||
		u.RawQuery != "" || u.ForceQuery || u.Fragment != "" ||
		!strings.EqualFold(path, wantPath) {
		return fmt.Errorf("repository must use the expected HTTPS GitHub destination")
	}
	return nil
}

func validateContributionRemote(remote *config.RemoteConfig, expected string) error {
	if remote == nil || len(remote.URLs) == 0 {
		return fmt.Errorf("workspace remote has no repository URL")
	}
	// go-git includes pushurl entries and applies insteadOf rules in this list.
	for _, raw := range remote.URLs {
		if err := validateContributionURL(raw, expected); err != nil {
			return fmt.Errorf("untrusted workspace remote %q: %w", remote.Name, err)
		}
	}
	return nil
}

func (m *GitHubManager) forkURL() (string, error) {
	login := m.user.GetLogin()
	if login == "" || strings.Trim(login, "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789-") != "" {
		return "", fmt.Errorf("GitHub did not return a valid fork owner")
	}
	return "https://github.com/" + login + "/" + contributionRepository + ".git", nil
}

func (m *GitHubManager) forkCloneOptions(raw string) (*git.CloneOptions, error) {
	expected, err := m.forkURL()
	if err != nil {
		return nil, err
	}
	if err := validateContributionURL(raw, expected); err != nil {
		return nil, fmt.Errorf("untrusted fork clone URL: %w", err)
	}
	return &git.CloneOptions{
		URL:      expected,
		Progress: os.Stdout,
		Auth:     &githttp.BasicAuth{Username: "oauth2", Password: m.token},
		Depth:    1,
	}, nil
}

func (m *GitHubManager) forkPushOptions(remote *config.RemoteConfig, branch plumbing.ReferenceName) (*git.PushOptions, error) {
	expected, err := m.forkURL()
	if err != nil {
		return nil, err
	}
	if err := validateContributionRemote(remote, expected); err != nil {
		return nil, err
	}
	if !branch.IsBranch() || branch.Validate() != nil {
		return nil, fmt.Errorf("checkout a local branch before pushing")
	}
	return &git.PushOptions{
		RemoteName: "origin",
		RemoteURL:  expected,
		RefSpecs:   []config.RefSpec{config.RefSpec(branch.String() + ":" + branch.String())},
		Auth:       &githttp.BasicAuth{Username: "oauth2", Password: m.token},
	}, nil
}

func (m *GitHubManager) upstreamPullOptions() *git.PullOptions {
	return &git.PullOptions{
		RemoteName:    "upstream",
		RemoteURL:     contributionUpstreamURL,
		ReferenceName: plumbing.NewBranchReferenceName(m.UpstreamBranch),
		SingleBranch:  true,
	}
}

// ensureUser runs only inside an explicitly requested contribution operation.
// Constructing the local editor or restoring a credential never performs network I/O.
func (m *GitHubManager) ensureUser() error {
	if m.user != nil {
		return nil
	}
	if m.client == nil {
		return fmt.Errorf("connect a stored GitHub credential before contributing")
	}
	user, _, err := m.client.Users.Get(context.Background(), "")
	if err != nil {
		return fmt.Errorf("could not verify GitHub account: %w", err)
	}
	if user == nil || user.GetLogin() == "" || user.GetID() == 0 {
		return fmt.Errorf("GitHub did not return a valid account identity")
	}
	m.user = user
	return nil
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
	branches, _, err := m.client.Repositories.ListBranches(ctx, contributionUpstreamOwner, contributionRepository, &github.BranchListOptions{ListOptions: github.ListOptions{PerPage: 100}})
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

// SetupWorkspace checks or creates the user's fork, clones it, and pins upstream.
func (m *GitHubManager) SetupWorkspace(progressCallback func(string)) error {
	ctx := context.Background()
	if err := m.ensureUser(); err != nil {
		return err
	}

	progressCallback("Checking for existing fork...")
	repo, _, err := m.client.Repositories.Get(ctx, m.user.GetLogin(), contributionRepository)
	if err != nil {
		var apiErr *github.ErrorResponse
		if !errors.As(err, &apiErr) || apiErr.Response == nil || apiErr.Response.StatusCode != nethttp.StatusNotFound {
			return fmt.Errorf("failed to check for fork: %w", err)
		}
		progressCallback("Forking " + contributionUpstreamOwner + "/" + contributionRepository + "...")
		repo, _, err = m.client.Repositories.CreateFork(ctx, contributionUpstreamOwner, contributionRepository, nil)
		var accepted *github.AcceptedError
		if err != nil && !errors.As(err, &accepted) {
			return fmt.Errorf("failed to fork: %w", err)
		}
		progressCallback("Fork created!")
		time.Sleep(5 * time.Second)
	} else {
		progressCallback("Found existing fork.")
	}
	cloneOptions, err := m.forkCloneOptions(repo.GetCloneURL())
	if err != nil {
		return err
	}

	var r *git.Repository
	if _, err := os.Stat(m.repoPath); err == nil {
		progressCallback("Target directory exists. Opening...")
		r, err = git.PlainOpen(m.repoPath)
		if err != nil {
			return fmt.Errorf("directory exists but is not a valid git repository: %w", err)
		}
	} else if os.IsNotExist(err) {
		progressCallback("Cloning repository (this may take a while)...")
		r, err = git.PlainClone(m.repoPath, false, cloneOptions)
		if err != nil {
			return fmt.Errorf("failed to clone: %w", err)
		}
	} else {
		return err
	}

	progressCallback("Configuring upstream remote...")
	if err := m.configureWorkspaceRemotes(r); err != nil {
		return err
	}
	progressCallback("Workspace Ready!")
	return nil
}

func (m *GitHubManager) configureWorkspaceRemotes(r *git.Repository) error {
	expected, err := m.forkURL()
	if err != nil {
		return err
	}
	origin, err := r.Remote("origin")
	if err != nil {
		return fmt.Errorf("workspace needs an origin pointing to your GitHub fork: %w", err)
	}
	if err := validateContributionRemote(origin.Config(), expected); err != nil {
		return err
	}
	upstream, err := r.Remote("upstream")
	if err == nil {
		return validateContributionRemote(upstream.Config(), contributionUpstreamURL)
	}
	if !errors.Is(err, git.ErrRemoteNotFound) {
		return err
	}
	_, err = r.CreateRemote(&config.RemoteConfig{
		Name: "upstream",
		URLs: []string{contributionUpstreamURL},
	})
	if err != nil {
		return fmt.Errorf("failed to add upstream: %w", err)
	}
	return nil
}

// SyncUpdates pulls from upstream and updates local master
// This effectively does: git checkout master && git pull upstream master
func (m *GitHubManager) SyncUpdates() error {
	r, err := git.PlainOpen(m.repoPath)
	if err != nil {
		return err
	}
	upstream, err := r.Remote("upstream")
	if err != nil {
		return err
	}
	if err := validateContributionRemote(upstream.Config(), contributionUpstreamURL); err != nil {
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

	// Pin the public source independently of mutable repository configuration.
	// No stored credential is needed or attached to this pull.
	err = w.Pull(m.upstreamPullOptions())

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
	if err := m.ensureUser(); err != nil {
		return err
	}
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

	name, email := m.user.GetName(), m.user.GetEmail()
	if name == "" {
		name = m.user.GetLogin()
	}
	if email == "" {
		email = fmt.Sprintf("%d+%s@users.noreply.github.com", m.user.GetID(), m.user.GetLogin())
	}
	_, err = w.Commit(message, &git.CommitOptions{
		Author: &object.Signature{
			Name:  name,
			Email: email,
			When:  time.Now(),
		},
	})
	return err
}

// PushCurrentBranch pushes only the checked-out branch to the verified user's fork.
func (m *GitHubManager) PushCurrentBranch() error {
	if err := m.ensureUser(); err != nil {
		return err
	}
	r, err := git.PlainOpen(m.repoPath)
	if err != nil {
		return err
	}
	remote, err := r.Remote("origin")
	if err != nil {
		return err
	}
	head, err := r.Head()
	if err != nil {
		return err
	}
	options, err := m.forkPushOptions(remote.Config(), head.Name())
	if err != nil {
		return err
	}
	// Use this validated remote snapshot and explicit URL; r.Push would re-read
	// repository configuration between validation and transport creation.
	return remote.Push(options)
}

// OpenPullRequest returns an existing matching open PR or creates one.
func (m *GitHubManager) OpenPullRequest(title, body string) (string, error) {
	if err := m.ensureUser(); err != nil {
		return "", err
	}
	branch, err := m.GetCurrentBranch()
	if err != nil {
		return "", err
	}
	return m.openPullRequest(branch, title, body)
}

func (m *GitHubManager) openPullRequest(branch, title, body string) (string, error) {
	ctx := context.Background()
	head := m.user.GetLogin() + ":" + branch
	existing, _, err := m.client.PullRequests.List(ctx, contributionUpstreamOwner, contributionRepository, &github.PullRequestListOptions{
		State: "open",
		Head:  head,
		Base:  m.UpstreamBranch,
	})
	if err != nil {
		return "", fmt.Errorf("could not check for an existing pull request: %w", err)
	}
	for _, pr := range existing {
		if pr != nil && pr.GetState() == "open" && pr.Head != nil && pr.Base != nil &&
			strings.EqualFold(pr.Head.User.GetLogin(), m.user.GetLogin()) &&
			pr.Head.GetRef() == branch && pr.Base.GetRef() == m.UpstreamBranch {
			if pr.GetHTMLURL() == "" {
				return "", fmt.Errorf("GitHub returned an existing pull request without a URL")
			}
			return pr.GetHTMLURL(), nil
		}
	}
	newPR := &github.NewPullRequest{
		Title:               github.String(title),
		Head:                github.String(head),
		Base:                github.String(m.UpstreamBranch),
		Body:                github.String(body + "\n\n*Created with MBII Foundry*"),
		MaintainerCanModify: github.Bool(true),
	}
	pr, _, err := m.client.PullRequests.Create(ctx, contributionUpstreamOwner, contributionRepository, newPR)
	if err != nil {
		return "", fmt.Errorf("could not create pull request: %w", err)
	}
	if pr.GetHTMLURL() == "" {
		return "", fmt.Errorf("GitHub returned a pull request without a URL")
	}
	return pr.GetHTMLURL(), nil
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
