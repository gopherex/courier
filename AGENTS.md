# Courier agent instructions

## Scope and decisions

- Courier is a notification delivery service. Read `docs/runtime.md` for implemented behavior and operational defaults.
- Read `docs/idea.md` for the agreed product and architecture requirements.
  PostgreSQL is the storage and durable queue. Initial channels are SMTP email,
  WebPush, and FCM. Ask about unresolved choices before implementing them.
- Use IAM as the reference for OpenAPI-first development, code generation,
  and SDKs. Courier has its own notification contract. Generate API types,
  server interfaces, and public Go and TypeScript SDKs from OpenAPI.
- Use pgxpool, pgtx, and sqld for PostgreSQL. Generate database row types,
  query parameters, and results from SQL. Do not hand-write duplicate API or
  database contract types. Internal implementation structures, such as workers,
  provider clients, and process configuration, may be ordinary handwritten code.
- Use Komeet's current PostgreSQL implementation as the reference for
  transactions, migrations, and telemetry. See `docs/implementation-references.md`.
- Build the admin UI from scratch. Do not copy IAM's admin UI or assume its
  application layout is the required Courier layout.
- Add packages and dependencies when required by an agreed implementation;
  do not create placeholder services or speculative abstractions.

## Discovery and local work

- Inspect the worktree before changes and preserve unrelated user edits.
- Prefer codebase-memory-mcp for code discovery: `search_graph`, `trace_path`,
  `get_code_snippet`, `query_graph`, and `get_architecture`. Index this
  repository first if it is not indexed. Verify results refer to Courier.
- Use `rg` for configuration, documentation, literal strings, or when the
  graph cannot answer the question.
- On environments with `/home/yaroher/.codex/RTK.md`, read and follow it.
  RTK is a local command wrapper, not a repository or CI dependency.

## Implementation and verification

- Use the Makefile as the canonical source of development commands.
- Run `make check` before completing code or tooling changes. Report which
  checks passed, failed, or were skipped; do not describe skips as tests.
- Run appropriate additional checks when the change requires them.
- Use `make fmt` and `make tidy` to resolve formatting or module drift.
- Keep Go commands isolated from parent workspaces with `GOWORK=off`.
- Keep the linter version pinned in the Makefile and invoke that executable
  both locally and in CI. Lint the whole module, not only changed lines.
- Never suppress command failures to make checks pass. Any lint suppression
  must name the linter and explain why it is necessary.
- When introducing the first Go package, remove the empty-package skip in
  the Makefile and update README so missing packages fail checks normally.
- Once code generation exists, edit its source inputs and regenerate;
  do not hand-edit generated output.
- Keep credentials, local configuration, tool binaries, and build artifacts
  out of version control. Commit examples with safe placeholder values.

## Documentation

- User documentation lives in `website/docs` (Docusaurus, GitHub Pages). Keep
  README concise and link to the relevant guides. Run `make docs-build` after
  documentation edits; broken links fail the build.
- Shared SVG branding lives in `.github/assets`; keep copies in `web/public`
  and `website/static/img` consistent.

- Update documentation alongside behavior and tooling changes. Describe
  current behavior explicitly, without conversational references or edit logs.
- Keep README commands consistent with the Makefile and CI.
- If the repository is bound to YouTrack, follow the `youtrack-tasks` skill:
  project documentation belongs in its knowledge base, organized by section;
  update dependent articles and link related issues and articles both ways.

## Commits

- Use Conventional Commits: `type(scope): summary`, with an imperative,
  lowercase subject of at most 72 characters and no trailing period.
- Types: feat, fix, refactor, chore, docs, test, perf, build, ci, style.
- Never add `Co-Authored-By` or other AI attribution.
- Resolve YouTrack bindings from `~/.config/youtrack/index.json`; never guess
  a project key. In a bound repository use the `youtrack-tasks` skill and add
  the applicable issue IDs in a `Refs:` trailer.
