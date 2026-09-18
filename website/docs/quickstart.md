# Quickstart

Requirements: Go 1.26.2+, Node 22.23.1+, Yarn 1.22.22, Python 3, GNU Make,
a C compiler and Docker with Compose. Go commands run with `GOWORK=off`.

```bash
git clone https://github.com/gopherex/courier.git
cd courier
make node-deps build
make dev-env dev-up migrate
make run
```

`make dev-env` creates a local `.env` with random secrets and refuses to overwrite
an existing file. PostgreSQL uses localhost:15432, SMTP localhost:11025 and
Mailpit's inbox localhost:18025. These ports bind to loopback.

1. Open http://localhost:18080 and enter `COURIER_ADMIN_KEY` from `.env`.
2. Create a project, for example `accounts`, with default locale `en`.
3. Open **Providers → SMTP**. Set host `127.0.0.1`, port `11025`, TLS `plain`,
   sender `courier@example.test`; leave credentials empty. Save.
4. Add notification key `registration`. In **Data schema**, add a required
   `string` field named `name`.
5. In **Templates**, use subject `Welcome, {{.name}}` and text `Hello, {{.name}}`.
   Save the configuration.
6. Open **Test delivery**, enter sample data `{"name":"Alex"}` and target
   `{"address":"alex@example.test"}`. Preview, then send.
7. Read the message at http://localhost:18025.

Plain SMTP is allowed only in explicit development mode. Do not reuse the local
database credentials or development flags in production.

For application integration, issue a project key under **Service access** and
follow the [API example](rest-api/overview.md). The administrator key must not be
used as a service bearer token.

Stop infrastructure with `docker compose down`; its named volume preserves the
database. `make dev-up` starts it again. The local server uses the built files in
`web/dist`; rebuild with `make build-web` after UI changes.
