# Releasing the Go SDK

Checklist for cutting a new release of `github.com/declaw-ai/declaw-go`.
The public repo is a clean mirror of `declaw/go-sdk/` in the monorepo,
synced via `git subtree split` (GH Actions sync is inactive — manual only).

## Prerequisites

- Push rights on `declaw-ai/declaw-go` and the `go-sdk` remote configured:
  `git remote add go-sdk git@github.com:declaw-ai/declaw-go.git`

## Steps

1. **Update changelog**

   Add the new version block at the top of `CHANGELOG.md`
   ([Keep a Changelog](https://keepachangelog.com/) format — `## [vX.Y.Z]`,
   `### Added` / `### Changed` / `### Fixed`). Go semver rules apply:
   `v1.x` is a stability promise, `v2+` requires a module-path change —
   stay on `v0.x` until that commitment is made deliberately.

2. **Verify**

   ```bash
   cd declaw/go-sdk && go build ./... && go vet ./... && go test ./...
   ```

3. **Sync the public mirror** (from repo root)

   ```bash
   git subtree split --prefix=declaw/go-sdk -b go-sdk-release
   git push go-sdk go-sdk-release:main --force
   git branch -D go-sdk-release
   ```

4. **Tag on the public repo**

   ```bash
   git ls-remote go-sdk main          # note the synced HEAD sha
   # tag via the GitHub UI/API or a public-repo clone:
   gh api repos/declaw-ai/declaw-go/git/refs -f ref=refs/tags/vX.Y.Z -f sha=<synced-head-sha>
   ```

5. **Verify the module resolves**

   ```bash
   GOPROXY=proxy.golang.org go list -m github.com/declaw-ai/declaw-go@vX.Y.Z
   ```

6. **Back in the monorepo**: commit the changelog as
   `release(go-sdk): vX.Y.Z`, push `main`, and update the version table in
   `CLAUDE.md` + the docs capability matrix if this is part of a release train.

## Note on the CLI coupling

The CLI's public `go.mod` pins a published go-sdk version. When releasing
both, release the **go-sdk first**, then point the CLI's public `go.mod` at
the new tag during the CLI sync (see `cli/RELEASING.md`).
