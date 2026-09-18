# Development and tests

```bash
make node-deps docs-deps
make check          # lint, vet, unit/race tests, generated drift, SDK/UI build
make integration    # real PostgreSQL and SMTP, race detector; Docker required
make docs-build     # Docusaurus build, strict link checks
make build
make dev-env dev-up migrate
make run            # leave running in another terminal
# Install Chromium once, then run against the local stack.
yarn playwright install chromium
make browser-test
```

`CHROMIUM_PATH` can select an existing Chromium executable. Browser tests create
isolated test projects and send only to the configured local Mailpit sink. They
exercise project setup, schema editing, templates, preview escaping, SMTP setup,
test send, saved configuration, navigation, language switching and mobile layout.

Integration coverage includes concurrent deduplication, transactional rejection,
late preference blocks, project-scoped service authentication/revocation, live
provider updates, payload cleanup, retry exhaustion, expiration and replay.
Session lifecycle, cross-instance login limits and the post-completion
deduplication retention window also run against PostgreSQL. Release tooling tests
exercise the interactive bump, cancellation and tag-recreation paths against
temporary local Git repositories and bare remotes.
Additional unit tests cover strict schemas, localization/HTML escaping, integer
precision, SMTP success boundaries and encrypted WebPush requests.

`make generate` regenerates Go and TypeScript API types and SQL query models.
Edit OpenAPI/SQL inputs, never generated output. `make generate-check` detects
changed, deleted and newly generated files. Generate migrations with
`make migration NAME=description` against Docker PostgreSQL.

Go uses `GOWORK=off`. Lint runs the pinned Makefile executable over the whole module,
including integration-tagged files. Failed Docker prerequisites fail tests instead
of silently skipping them. Documentation dependencies use their own lockfile in
`website/`, following the IAM repository structure.

See [release workflow](releasing.md) for publication.
