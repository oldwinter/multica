# Release runbook

## Normal release

Release from a reviewed commit on `main` by creating and pushing a new semantic
version tag such as `v0.18.4`. Upstream publishes from that tag push. A GitHub
UI dispatch is accepted only so the downstream auto-tagger can start a release
after it creates a tag with `GITHUB_TOKEN` (token-created tag pushes do not
retrigger workflows). Dispatching a branch still fails the tag-name check.

The canonical `multica-ai/multica` repository publishes its Homebrew formula to
`multica-ai/homebrew-tap`. Forks do not need that token: they publish CLI
archives, checksums, macOS/Linux/Windows Desktop installers, update metadata, and
container images to their own GitHub repository/package namespace, while the
Homebrew upload is skipped. Use a clearly downstream tag such as
`v0.18.4-oldwinter.1` so it remains distinct from upstream releases and is
classified as a prerelease.

On `oldwinter/multica`, every push to `main` runs `.github/workflows/auto-tag-main.yml`.
That workflow computes the next `vX.Y.Z-oldwinter.N` tag from the newest stable
`vX.Y.Z` ancestor, pushes it, and dispatches Release for that tag. Manual
`git tag` / `git push --tags` still works and still publishes through the tag
push event.

The Desktop jobs use the tag workflow's built-in `GITHUB_TOKEN` and the
`GITHUB_REPOSITORY` identity supplied by Actions. No personal access token is
required. Linux produces AppImage, deb, and rpm artifacts for x64 and arm64;
Windows produces NSIS installers for x64 and arm64; macOS produces DMG and ZIP
artifacts for Intel x64 and Apple Silicon arm64. These packages are unsigned,
and the macOS packages are not notarized. After the first blocked launch, users
must open **System Settings → Privacy & Security**, scroll to **Security**, click
**Open Anyway**, authenticate, and confirm **Open**. This creates an exception
for Multica without disabling Gatekeeper globally. The user-facing steps live in
the [Desktop documentation](../apps/docs/content/docs/desktop-app.mdx).
Automatic updates require a signed macOS app, so users of these temporary
unsigned builds must install each newer DMG manually.

Signed and notarized macOS releases can replace this temporary path after the
Apple certificate plus `APPLE_ID`, `APPLE_APP_SPECIFIC_PASSWORD`, and
`APPLE_TEAM_ID` secrets are wired into CI.

The verification job runs the Go tests and `govulncheck` before any publishing
job starts. The vulnerability scan is fail-closed by default.

## Emergency vulnerability-scan bypass

Use the bypass only when `govulncheck` itself or its live vulnerability database
is unavailable, or when maintainers have documented a confirmed false positive
that blocks an urgent release. Never use it to publish a release with an
unresolved reachable vulnerability.

1. Record the reason and maintainer approval in the release issue or pull
   request, and confirm no other release is in progress.
2. In **Settings → Secrets and variables → Actions → Variables**, set the
   repository variable `ALLOW_VULN_BYPASS_FOR_TAG` to the exact release tag,
   for example `v0.18.4`.
3. Re-run the failed Release workflow for that tag. A different tag, an empty
   value, or any typo keeps the scan enabled.
4. Confirm the verification log contains the explicit bypass warning and retain
   the workflow URL in the incident record.
5. Delete `ALLOW_VULN_BYPASS_FOR_TAG` immediately after the release run
   completes. The tag-scoped value prevents a concurrent release with another
   tag from inheriting the bypass.

Every Go binary retains its compiler version in the standard Go build metadata;
use `go version -m <binary>` when auditing a downloaded release artifact.
