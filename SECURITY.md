# Security & Access Model

MBII Foundry is single-maintainer by design. Every commit on `main` must be authored and signed by **Frenzeh**. No exceptions — not for collaborators, not for automation, not for emergency fixes.

This document describes the layers that enforce it and how to report issues.

## Access layers (defense in depth)

1. **Push access** — GitHub repo-level. `Frenzeh/mbii-foundry` is a personal repo with no added collaborators. Only the `Frenzeh` account can push directly. All external contributions go through **forks + pull requests**; never direct pushes.
2. **Branch protection on `main`** — GitHub setting. Direct pushes are blocked; merges must land via PR with CODEOWNERS review, linear history, passing CI, and a signed merge commit. Force-pushes are disabled. See [`tools/setup-branch-protection.sh`](tools/setup-branch-protection.sh) for the exact API calls that configure this.
3. **CODEOWNERS** — [`.github/CODEOWNERS`](.github/CODEOWNERS) assigns `* @Frenzeh`. With "Require review from Code Owners" enabled in branch protection, every PR to main needs Frenzeh's explicit approval regardless of who opens it.
4. **Signed commits** — every commit on main is cryptographically signed with Frenzy's SSH signing key. GitHub displays a green "Verified" badge for valid signatures; unsigned or wrongly-signed commits are rejected at merge time. Users can audit a commit's authenticity via `git log --show-signature`.
5. **Client-side pre-push hook** — [`tools/git-hooks/pre-push`](tools/git-hooks/pre-push) rejects a push if the local git identity doesn't match Frenzy or if the remote isn't the canonical Frenzeh URL. Defense-in-depth catch for honest mistakes (e.g. a dev session pushing under the wrong Git config). Not a security boundary — can be bypassed with `--no-verify` — but useful.

## One-time setup (maintainer)

All four items below are already done for the live repo. Documented here so that state is reproducible if the repo is ever re-hosted.

### 1. Enable SSH commit signing locally

```bash
cd mbii-foundry
git config user.signingkey ~/.ssh/id_ed25519_github_frenzy.pub
git config gpg.format ssh
git config commit.gpgsign true
git config tag.gpgsign true
```

Scoped to this repo via `git config` (no `--global`) so other projects aren't affected.

### 2. Register the SSH key as a GitHub *signing* key (not just auth)

GitHub treats SSH auth keys and SSH signing keys as separate concepts, even when they're the same key file. Add the public key at:

<https://github.com/settings/ssh/new> → **Key type: Signing Key**

Without this, `git log --show-signature` shows "signed but key not found" on GitHub's UI.

### 3. Install the client-side pre-push guard

```bash
cd mbii-foundry
./tools/git-hooks/install.sh
```

One-liner that points this repo at `tools/git-hooks/` for its hooks path.

### 4. Configure branch protection on main

```bash
gh auth login   # as Frenzeh, not peterjamus
./tools/setup-branch-protection.sh
```

Requires the `Frenzeh` account's `gh` CLI auth. Idempotent — safe to re-run.

## Reporting vulnerabilities

If you've found a security issue in MBII Foundry — a way the app could be tricked into writing files outside the user's gamedata folder, a crash from a malformed `.mbch`, an auth bypass in any network code — do not open a public GitHub issue.

Instead, email **elitewarriors@protonmail.com** with:
- A description of the issue
- Minimal repro steps or a proof-of-concept file
- The commit SHA or release version where you observed it

I'll acknowledge within a few days and credit you in the fix commit unless you prefer otherwise.

## What "Verified" means on GitHub

When you browse commits on this repo, each commit should show a green **Verified** badge. If you see:
- **Verified** — commit was signed by Frenzeh's registered signing key. Trustworthy.
- **Unverified** — commit is authored by Frenzy but not signed, OR signed by a key not registered for signing. Treat with suspicion.
- **No badge** — historical commit from before signing was enabled. Will gradually age out as main advances.

If you ever see an unverified commit on main that isn't explained, please report it (see above).
