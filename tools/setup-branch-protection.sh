#!/usr/bin/env bash
#
# MBII Foundry — branch protection setup for `main`.
#
# Idempotent — safe to re-run. Configures the protection rules
# described in SECURITY.md via the GitHub API.
#
# Prerequisites:
#   - `gh` CLI authenticated as Frenzeh (not peterjamus or any other
#     account): `gh auth switch` or `gh auth login`
#   - SSH signing key already registered on Frenzeh's account as a
#     SIGNING key (not just an auth key): https://github.com/settings/keys
#
# What it configures:
#   - Direct pushes to main: blocked (PR-only)
#   - Required status checks: the CI job must pass
#   - Required reviews: CODEOWNERS (which is just @Frenzeh)
#   - Signed commits required
#   - Force pushes: blocked
#   - Deletions: blocked
#   - Linear history required (no merge commits; rebase/squash only)

set -euo pipefail

OWNER="Frenzeh"
REPO="mbii-foundry"
BRANCH="main"

# --- Verify gh auth points at Frenzeh ---
active=$(gh api user --jq .login 2>/dev/null || true)
if [[ "$active" != "$OWNER" ]]; then
    echo "ERROR: gh CLI is authed as '$active', expected '$OWNER'."
    echo "Run:  gh auth switch   # or: gh auth login"
    exit 1
fi
echo "✓ Authenticated as $active"

echo "Configuring branch protection on $OWNER/$REPO@$BRANCH..."

# Protection rules. See
# https://docs.github.com/en/rest/branches/branch-protection#update-branch-protection
gh api --method PUT \
    -H "Accept: application/vnd.github+json" \
    "/repos/$OWNER/$REPO/branches/$BRANCH/protection" \
    --input - <<'JSON'
{
  "required_status_checks": {
    "strict": true,
    "contexts": ["Build (ubuntu-latest)", "Build (macos-latest)", "Build (windows-latest)"]
  },
  "enforce_admins": true,
  "required_pull_request_reviews": {
    "dismiss_stale_reviews": true,
    "require_code_owner_reviews": true,
    "required_approving_review_count": 1
  },
  "restrictions": null,
  "required_linear_history": true,
  "allow_force_pushes": false,
  "allow_deletions": false,
  "block_creations": false,
  "required_conversation_resolution": true,
  "required_signatures": true,
  "lock_branch": false,
  "allow_fork_syncing": false
}
JSON

echo ""
echo "✓ Branch protection applied. Current rules:"
gh api "/repos/$OWNER/$REPO/branches/$BRANCH/protection" \
    --jq '{
        signed_commits: .required_signatures.enabled,
        linear_history: .required_linear_history.enabled,
        force_push: .allow_force_pushes.enabled,
        deletions: .allow_deletions.enabled,
        required_reviews: .required_pull_request_reviews.required_approving_review_count,
        codeowner_review: .required_pull_request_reviews.require_code_owner_reviews,
        required_checks: .required_status_checks.contexts
    }'
