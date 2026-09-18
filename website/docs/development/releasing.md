# Releases

Run `make release` from a clean, committed `master` checkout after CI is green.
The interactive command proposes the first SDK version or a patch bump; enter an
explicit version if needed. `make release VERSION=0.1.0` preselects a version but
still asks for confirmation. `make release-plan` prints the proposal without
modifying files, creating tags or pushing.

The release helper fetches origin, requires local HEAD to match `origin/master`,
rejects existing tags and versions outside v0/v1, and keeps tags immutable. It
updates SDK/admin package versions, the local workspace dependency and OpenAPI
info.version, regenerates API output, runs checks, creates a conventional release
commit and an annotated tag, then pushes branch and tag atomically.

A `v*` tag triggers the release workflow. Full CI must pass before publication:

- Docker image at `ghcr.io/gopherex/courier:VERSION` and `latest` (Linux amd64).
- `@gopherex/courier-sdk@VERSION` at `npm.pkg.github.com` using GitHub Packages.
- A GitHub release containing OpenAPI and the packed SDK with SHA-256 checksums.

Go consumers use the root module tag; the Go SDK is part of that module. The
workflow uses `GITHUB_TOKEN` with scoped package/content permissions, with no
npmjs.org publication. Enable Actions/package creation in the repository or
organization. Private package installation requires a token with read access.

Do not delete or recreate published tags. For publication failure, inspect the
failed job before rerunning it: already-published package versions are immutable.
`make release` starts publication; it does not wait for or guarantee its completion.
Inspect the release workflow, artifacts and registry image after it finishes.

Documentation deploys independently to GitHub Pages on `master`. Pull requests
build the site and check links without deploying it.
