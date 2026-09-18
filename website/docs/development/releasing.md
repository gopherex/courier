# Releases

Run `make release` from a clean, committed checkout after CI is green. The
interactive Makefile recipe follows IAM's release menu:

```text
1) bump version
2) recreate last tag (vX.Y.Z) on HEAD   [force]
3) cancel
```

**Bump version** opens a second menu: major, minor or patch. The base is the latest
local `vMAJOR.MINOR.PATCH` tag, sorted numerically; without tags it is `0.0.0`.
The first major/minor/patch choices therefore produce `1.0.0`, `0.1.0` or `0.0.1`.
Versions above major 1 are rejected because they require a new Go module path.

The recipe prints the actions and requires the exact confirmation `yes`. It
updates the SDK version and the admin workspace's SDK dependency, runs Yarn,
commits the changes, creates an annotated tag and pushes HEAD followed by the tag.
It does not fetch tags, require a specific branch, demand that HEAD already match
origin, or run the test suite locally. Run checks and fetch tags beforehand when
needed. The OpenAPI info version describes the API contract independently of the
SDK release version.

**Recreate last tag** requires an existing local release tag and an SDK version
matching it. After confirmation, it deletes the tag from origin if present,
recreates the local annotated tag on HEAD and force-pushes it. This changes the
meaning of an existing version and is intended for local development releases,
not immutable production versions. Cancellation makes no changes.

A `v*` tag triggers the release workflow. Full CI must pass before publication:

- Docker image at `ghcr.io/gopherex/courier:VERSION` and `latest` (Linux amd64).
- `@gopherex/courier-sdk@VERSION` at `npm.pkg.github.com` using GitHub Packages.
- A GitHub release containing OpenAPI and the packed SDK with SHA-256 checksums.

Go consumers use the root module tag; the Go SDK is part of that module. The
workflow uses `GITHUB_TOKEN` with scoped package/content permissions, with no
npmjs.org publication. Enable Actions/package creation in the repository or
organization. Private package installation requires a token with read access.

For a recreated tag, the workflow attempts to delete the existing GitHub npm
package version before publishing it again, as in IAM. Configure the optional
`PACKAGES_TOKEN` secret with package deletion permissions for this operation.
Without it, a first publication still works; republishing an existing npm version
fails. The normal `GITHUB_TOKEN` cannot delete package versions. An existing
GitHub Release is updated and its assets are replaced; Docker tags are rebuilt.
For production versions, use a new version instead of recreating the tag.
`make release` starts publication; it does not wait for or guarantee its completion.
Inspect the release workflow, artifacts and registry image after it finishes.

Documentation deploys independently to GitHub Pages on `master`. Pull requests
build the site and check links without deploying it.
