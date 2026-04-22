#!/usr/bin/env bash
# One-liner to activate the in-repo git hooks for this clone.
# Re-run whenever the hooks directory changes to pick up new hooks.

set -e
cd "$(dirname "$0")/../.."
git config core.hooksPath tools/git-hooks
chmod +x tools/git-hooks/pre-push
echo "✓ Git hooks installed. pre-push guard is active."
echo "  To disable temporarily: git push --no-verify"
echo "  To disable permanently: git config --unset core.hooksPath"
