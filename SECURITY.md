# Security

If you've found a security issue in MBII Foundry — a way the app could be tricked into writing outside the configured paths, a crash triggered by a malformed file, anything exploitable in the network code — please don't open a public GitHub issue.

Email **elitewarriors@protonmail.com** with a description, minimal repro steps, and the commit SHA or release version. I'll acknowledge within a few days.

---

## Note on access

Only Frenzy pushes to this repo. Every commit on `main` is signed with Frenzy's SSH signing key — look for the green **Verified** badge on GitHub. If you ever see an unverified commit authored by "Frenzy", please report it via the email above.

## Release and update trust

v0.16.0-alpha establishes the first long-lived Ed25519 update trust root.
Release binaries embed only its public half. One isolated release job receives
the private half, verifies the keypair, and signs manifests that bind the
release tag, platform, architecture, archive length, and SHA-256 digest.

The private publisher key is kept outside the repository and logs, with
restricted access and an offline recovery copy. Losing it without a planned
transition prevents installed builds from authenticating later updates;
compromise allows forged update manifests. Key rotation must ship as a
transition signed by the old key rather than silently replacing the trust root.

The v0.16.0-alpha macOS archive is intentionally ad-hoc signed and unnotarized.
That signature checks bundle structure but provides no Apple publisher identity.
Windows artifacts are not Authenticode signed. A valid Foundry manifest is an
application update-integrity statement, not an Apple, Microsoft, or operating
system trust verdict.
