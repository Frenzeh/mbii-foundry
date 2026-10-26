package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"reflect"
	"strings"
	"testing"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	githttp "github.com/go-git/go-git/v5/plumbing/transport/http"
	"github.com/go-git/go-git/v5/storage/memory"
	"github.com/google/go-github/v50/github"
)

const (
	auditContributionToken = "synthetic-contribution-token"
	auditContributionFork  = "https://github.com/fixture-contributor/TextAssets.git"
	auditContributionPR    = "https://github.com/MovieBattles/TextAssets/pull/123"
)

func auditContributionManager() *GitHubManager {
	manager := NewGitHubManager(auditContributionToken, "")
	manager.user = &github.User{Login: github.String("fixture-contributor"), ID: github.Int64(123)}
	return manager
}

func TestContributionCloneDestinationPolicy(t *testing.T) {
	for _, tt := range []struct {
		name string
		url  string
		ok   bool
	}{
		{"canonical fork", auditContributionFork, true},
		{"equivalent GitHub spelling", "https://GITHUB.COM/Fixture-Contributor/textassets", true},
		{"trailing slash", auditContributionFork + "/", true},
		{"different origin", "https://example.invalid/fixture-contributor/TextAssets.git", false},
		{"different owner", "https://github.com/other-contributor/TextAssets.git", false},
		{"different repository", "https://github.com/fixture-contributor/OtherAssets.git", false},
		{"public upstream is not the fork", "https://github.com/MovieBattles/TextAssets.git", false},
		{"unencrypted", "http://github.com/fixture-contributor/TextAssets.git", false},
		{"SSH", "git@github.com:fixture-contributor/TextAssets.git", false},
		{"explicit port", "https://github.com:8443/fixture-contributor/TextAssets.git", false},
		{"embedded credential", "https://fixture-user:synthetic-url-password@github.com/fixture-contributor/TextAssets.git", false},
		{"query", auditContributionFork + "?view=1", false},
		{"fragment", auditContributionFork + "#main", false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			options, err := auditContributionManager().forkCloneOptions(tt.url)
			if !tt.ok {
				if err == nil || options != nil {
					t.Fatal("untrusted clone destination was accepted")
				}
				if strings.Contains(err.Error(), "synthetic-url-password") {
					t.Fatal("URL rejection disclosed the embedded fixture credential")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if options.URL != auditContributionFork {
				t.Fatalf("clone URL = %q, want canonical fork", options.URL)
			}
			auth, ok := options.Auth.(*githttp.BasicAuth)
			if !ok || auth.Username != "oauth2" || auth.Password != auditContributionToken {
				t.Fatal("trusted clone did not retain the supplied synthetic credential")
			}
		})
	}
}

func TestContributionPushDestinationPolicy(t *testing.T) {
	branch := plumbing.NewBranchReferenceName("contrib/fixture")
	for _, tt := range []struct {
		name string
		urls []string
		ok   bool
	}{
		{"fork", []string{auditContributionFork}, true},
		{"equivalent fork", []string{"https://github.com/Fixture-Contributor/TextAssets"}, true},
		{"same fetch and push destination", []string{auditContributionFork, auditContributionFork}, true},
		{"no destination", nil, false},
		{"different push destination", []string{auditContributionFork, "https://example.invalid/fixture/TextAssets.git"}, false},
		{"different fork", []string{"https://github.com/other-contributor/TextAssets.git"}, false},
		{"upstream", []string{"https://github.com/MovieBattles/TextAssets.git"}, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			options, err := auditContributionManager().forkPushOptions(&config.RemoteConfig{Name: "origin", URLs: tt.urls}, branch)
			if !tt.ok {
				if err == nil || options != nil {
					t.Fatal("untrusted push destination was accepted")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if options.RemoteURL != auditContributionFork || options.RemoteName != "origin" {
				t.Fatalf("push is not pinned to the verified fork: %+v", options)
			}
			wantRefs := []config.RefSpec{"refs/heads/contrib/fixture:refs/heads/contrib/fixture"}
			if !reflect.DeepEqual(options.RefSpecs, wantRefs) {
				t.Fatalf("push refs = %v, want only the checked-out branch", options.RefSpecs)
			}
			auth, ok := options.Auth.(*githttp.BasicAuth)
			if !ok || auth.Password != auditContributionToken {
				t.Fatal("trusted push did not retain the supplied synthetic credential")
			}
		})
	}
	if _, err := auditContributionManager().forkPushOptions(&config.RemoteConfig{Name: "origin", URLs: []string{auditContributionFork}}, plumbing.HEAD); err == nil {
		t.Fatal("detached HEAD was accepted as a current branch")
	}
}

func TestContributionWorkspaceEffectiveRemotePolicy(t *testing.T) {
	origin := "[remote \"origin\"]\nurl = " + auditContributionFork + "\n"
	for _, tt := range []struct {
		name   string
		config string
		ok     bool
	}{
		{"trusted workspace", origin, true},
		{"untrusted pushurl", origin + "pushurl = https://example.invalid/fixture/TextAssets.git\n", false},
		{"untrusted rewrite", origin + "[url \"https://example.invalid/\"]\ninsteadOf = https://github.com/\n", false},
		{"untrusted upstream", origin + "[remote \"upstream\"]\nurl = https://example.invalid/MovieBattles/TextAssets.git\n", false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			r, err := git.Init(memory.NewStorage(), nil)
			if err != nil {
				t.Fatal(err)
			}
			cfg := config.NewConfig()
			if err := cfg.Unmarshal([]byte(tt.config)); err != nil {
				t.Fatal(err)
			}
			if err := r.SetConfig(cfg); err != nil {
				t.Fatal(err)
			}
			manager := auditContributionManager()
			err = manager.configureWorkspaceRemotes(r)
			if (err == nil) != tt.ok {
				t.Fatalf("workspace acceptance = %v, want %v: %v", err == nil, tt.ok, err)
			}
			if !tt.ok {
				return
			}
			upstream, err := r.Remote("upstream")
			if err != nil || !reflect.DeepEqual(upstream.Config().URLs, []string{"https://github.com/MovieBattles/TextAssets.git"}) {
				t.Fatalf("workspace did not configure the pinned public upstream: %v", err)
			}
			remote, err := r.Remote("origin")
			if err != nil {
				t.Fatal(err)
			}
			options, err := manager.forkPushOptions(remote.Config(), plumbing.NewBranchReferenceName("contrib/fixture"))
			if err != nil {
				t.Fatal(err)
			}
			cfg.Remotes["origin"].URLs = []string{"https://example.invalid/changed/TextAssets.git"}
			if err := r.SetConfig(cfg); err != nil {
				t.Fatal(err)
			}
			if options.RemoteURL != auditContributionFork {
				t.Fatal("prepared push destination changed with mutable repository configuration")
			}
		})
	}
}

func TestContributionPublicPullOmitsCredential(t *testing.T) {
	for _, token := range []string{"", auditContributionToken} {
		manager := NewGitHubManager(token, "")
		manager.UpstreamBranch = "buildTest/fixture"
		options := manager.upstreamPullOptions()
		if options.RemoteURL != "https://github.com/MovieBattles/TextAssets.git" || options.RemoteName != "upstream" {
			t.Fatal("public pull is not pinned to the official upstream")
		}
		if options.Auth != nil {
			t.Fatal("public pull attached a credential")
		}
		if options.ReferenceName != plumbing.NewBranchReferenceName("buildTest/fixture") {
			t.Fatal("public pull lost the selected development branch")
		}
	}
}

func TestContributionRedirectPolicy(t *testing.T) {
	check := auditContributionManager().client.Client().CheckRedirect
	if check == nil {
		t.Fatal("authenticated API client has no redirect policy")
	}
	for _, tt := range []struct {
		name   string
		origin string
		target string
		ok     bool
	}{
		{"same Git origin", auditContributionFork, auditContributionFork + "/info/refs", true},
		{"same API origin", "https://api.github.com/user", "https://api.github.com/user/123", true},
		{"other Git origin", auditContributionFork, "https://example.invalid/fixture/TextAssets.git", false},
		{"other API origin", "https://api.github.com/user", "https://github.com/user", false},
		{"subdomain", auditContributionFork, "https://api.github.com/fixture/TextAssets.git", false},
		{"downgrade", auditContributionFork, "http://github.com/fixture-contributor/TextAssets.git", false},
		{"userinfo", auditContributionFork, "https://fixture:synthetic-password@github.com/fixture-contributor/TextAssets.git", false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			origin, err := http.NewRequest(http.MethodGet, tt.origin, nil)
			if err != nil {
				t.Fatal(err)
			}
			target, err := http.NewRequest(http.MethodGet, tt.target, nil)
			if err != nil {
				t.Fatal(err)
			}
			if err := check(target, []*http.Request{origin}); (err == nil) != tt.ok {
				t.Fatalf("redirect acceptance = %v, want %v: %v", err == nil, tt.ok, err)
			}
		})
	}
	origin, err := http.NewRequest(http.MethodGet, auditContributionFork, nil)
	if err != nil {
		t.Fatal(err)
	}
	via := make([]*http.Request, 10)
	for i := range via {
		via[i] = origin
	}
	if err := check(origin, via); err == nil {
		t.Fatal("redirect chain limit was not enforced")
	}
}

type auditContributionSubmitter struct {
	calls         []string
	failAt        string
	failure       error
	commitTitle   string
	prTitle       string
	prDescription string
}

func (s *auditContributionSubmitter) step(name string) error {
	s.calls = append(s.calls, name)
	if name == s.failAt {
		return s.failure
	}
	return nil
}

func (s *auditContributionSubmitter) SwitchToUpstream() error { return s.step("upstream") }
func (s *auditContributionSubmitter) CreateBranch(branch string) error {
	return s.step("branch:" + branch)
}
func (s *auditContributionSubmitter) StageAndCommit(title string) error {
	s.commitTitle = title
	return s.step("commit")
}
func (s *auditContributionSubmitter) PushCurrentBranch() error { return s.step("push") }
func (s *auditContributionSubmitter) OpenPullRequest(title, description string) (string, error) {
	s.prTitle, s.prDescription = title, description
	if err := s.step("pr"); err != nil {
		return "", err
	}
	return auditContributionPR, nil
}

func TestContributionSubmissionConsent(t *testing.T) {
	for _, branch := range []string{"", "contrib/fixture"} {
		for _, createPR := range []bool{false, true} {
			t.Run(fmt.Sprintf("branch=%q/pr=%v", branch, createPR), func(t *testing.T) {
				publisher := &auditContributionSubmitter{}
				message, err := submitContribution(publisher, branch, "Fixture title", "Fixture description", createPR)
				if err != nil {
					t.Fatal(err)
				}
				var want []string
				if branch != "" {
					want = append(want, "upstream", "branch:"+branch)
				}
				want = append(want, "commit", "push")
				if createPR {
					want = append(want, "pr")
				}
				if !reflect.DeepEqual(publisher.calls, want) {
					t.Fatalf("publication calls = %v, want %v", publisher.calls, want)
				}
				if publisher.commitTitle != "Fixture title" || (createPR && (publisher.prTitle != "Fixture title" || publisher.prDescription != "Fixture description")) {
					t.Fatal("captured submission fields were not retained")
				}
				if strings.Contains(message, auditContributionPR) != createPR {
					t.Fatalf("completion message does not match PR consent: %q", message)
				}
				if !createPR && !strings.Contains(message, "No pull request was requested") {
					t.Fatalf("push-only completion is not explicit: %q", message)
				}
			})
		}
	}
}

func TestContributionNewBranchRetainsCurrentBaseOnCheckoutConflict(t *testing.T) {
	publisher := &auditContributionSubmitter{failAt: "upstream", failure: errors.New("synthetic checkout conflict")}
	message, err := submitContribution(publisher, "contrib/fixture", "Fixture title", "Fixture description", false)
	if err != nil || message == "" {
		t.Fatalf("existing current-base workflow was lost: message=%q err=%v", message, err)
	}
	want := []string{"upstream", "branch:contrib/fixture", "commit", "push"}
	if !reflect.DeepEqual(publisher.calls, want) {
		t.Fatalf("fallback calls = %v, want %v", publisher.calls, want)
	}
}

func TestContributionSubmissionFailuresAreReported(t *testing.T) {
	failure := errors.New("synthetic publication failure")
	for _, tt := range []struct {
		name   string
		branch string
		failAt string
		calls  []string
	}{
		{"branch", "contrib/fixture", "branch:contrib/fixture", []string{"upstream", "branch:contrib/fixture"}},
		{"commit", "", "commit", []string{"commit"}},
		{"push", "", "push", []string{"commit", "push"}},
		{"current branch PR", "", "pr", []string{"commit", "push", "pr"}},
		{"new branch PR", "contrib/fixture", "pr", []string{"upstream", "branch:contrib/fixture", "commit", "push", "pr"}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			publisher := &auditContributionSubmitter{failAt: tt.failAt, failure: failure}
			message, err := submitContribution(publisher, tt.branch, "Fixture title", "Fixture description", true)
			if !errors.Is(err, failure) || message != "" {
				t.Fatalf("publication failure became success: message=%q err=%v", message, err)
			}
			if !reflect.DeepEqual(publisher.calls, tt.calls) {
				t.Fatalf("calls after failure = %v, want %v", publisher.calls, tt.calls)
			}
			if tt.failAt == "pr" && !strings.Contains(err.Error(), "changes were pushed") {
				t.Fatal("PR failure did not disclose that the push had already completed")
			}
		})
	}
}

func TestContributionPullRequestExistingAndFailure(t *testing.T) {
	existing := fmt.Sprintf(`[{"state":"open","html_url":%q,"head":{"ref":"contrib/fixture","user":{"login":"fixture-contributor"}},"base":{"ref":"master"}}]`, auditContributionPR)
	created := fmt.Sprintf(`{"html_url":%q}`, auditContributionPR)
	for _, tt := range []struct {
		name         string
		listStatus   int
		listBody     string
		createStatus int
		createBody   string
		wantCalls    []string
		wantError    bool
	}{
		{"verified existing PR", 200, existing, 0, "", []string{"GET"}, false},
		{"new PR", 200, "[]", 201, created, []string{"GET", "POST"}, false},
		{"unrelated PR is not existing", 200, strings.ReplaceAll(existing, `"master"`, `"other-base"`), 201, created, []string{"GET", "POST"}, false},
		{"lookup failure", 403, `{"message":"fixture lookup denied"}`, 0, "", []string{"GET"}, true},
		{"validation failure", 200, "[]", 422, `{"message":"fixture validation failed"}`, []string{"GET", "POST"}, true},
		{"permission failure", 200, "[]", 403, `{"message":"fixture publication denied"}`, []string{"GET", "POST"}, true},
		{"missing created URL", 200, "[]", 201, "{}", []string{"GET", "POST"}, true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var calls []string
			manager := auditContributionManager()
			manager.client = github.NewClient(&http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				calls = append(calls, req.Method)
				if req.URL.Scheme != "https" || req.URL.Host != "api.github.com" || req.URL.Path != "/repos/MovieBattles/TextAssets/pulls" {
					t.Fatalf("unexpected API destination: %s", req.URL)
				}
				status, body := tt.listStatus, tt.listBody
				switch req.Method {
				case http.MethodGet:
					query := req.URL.Query()
					if query.Get("state") != "open" || query.Get("head") != "fixture-contributor:contrib/fixture" || query.Get("base") != "master" {
						t.Fatalf("existing-PR query lost branch identity: %s", req.URL.RawQuery)
					}
				case http.MethodPost:
					if tt.createStatus == 0 {
						t.Fatal("PR creation was attempted without a successful lookup requiring creation")
					}
					var payload struct {
						Title string `json:"title"`
						Head  string `json:"head"`
						Base  string `json:"base"`
						Body  string `json:"body"`
					}
					if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
						t.Fatal(err)
					}
					if payload.Title != "Fixture title" || payload.Head != "fixture-contributor:contrib/fixture" || payload.Base != "master" || !strings.HasPrefix(payload.Body, "Fixture description\n\n") {
						t.Fatalf("PR fields changed: %+v", payload)
					}
					status, body = tt.createStatus, tt.createBody
				default:
					t.Fatalf("unexpected API method %s", req.Method)
				}
				return &http.Response{
					StatusCode: status,
					Status:     fmt.Sprintf("%d %s", status, http.StatusText(status)),
					Header:     http.Header{"Content-Type": []string{"application/json"}},
					Body:       io.NopCloser(strings.NewReader(body)),
					Request:    req,
				}, nil
			})})
			url, err := manager.openPullRequest("contrib/fixture", "Fixture title", "Fixture description")
			if (err != nil) != tt.wantError {
				t.Fatalf("PR result = %q, %v; want error %v", url, err, tt.wantError)
			}
			if tt.wantError && url != "" {
				t.Fatal("failed PR operation returned a success URL")
			}
			if !tt.wantError && url != auditContributionPR {
				t.Fatalf("PR URL = %q, want verified result", url)
			}
			if !reflect.DeepEqual(calls, tt.wantCalls) {
				t.Fatalf("API calls = %v, want %v", calls, tt.wantCalls)
			}
		})
	}
}
