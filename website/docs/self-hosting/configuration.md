# Configuration

Courier reads environment variables. The process does not load `.env` itself;
local Make targets source it. Pass variables through your orchestrator or container.

| Variable | Default | Meaning |
| --- | --- | --- |
| `DATABASE_URL` | required | PostgreSQL connection string |
| `COURIER_ADMIN_KEY` | required | Administrator secret, at least 32 characters |
| `COURIER_ENCRYPTION_KEY` | required | Base64 of exactly 32 random bytes |
| `COURIER_PUBLIC_URL` | `http://localhost:8080` | Exact origin without path/trailing slash; HTTPS required in production |
| `COURIER_LISTEN` | `:8080` | HTTP listener |
| `COURIER_ADMIN_DIR` | `web/dist` | Built console assets directory |
| `COURIER_DEVELOPMENT` | false | Only literal `true` permits HTTP cookies and plain SMTP |
| `COURIER_WORKERS` | `4` | Workers per replica, range 1–64 |
| `COURIER_MAX_ATTEMPTS` | `12` | Attempts before dead-lettering, range 1–100 |
| `COURIER_ATTEMPT_TIMEOUT` | `30s` | Provider attempt timeout |
| `COURIER_LEASE_DURATION` | `90s` | Renewable lease; must exceed attempt timeout |
| `COURIER_DEDUP_RETENTION` | `168h` | Deduplication retention after completion |
| `COURIER_DEAD_RETENTION` | `168h` | Dead-letter payload retention |
| `OTEL_EXPORTER_OTLP_ENDPOINT` | unset | Optional OTLP/HTTP endpoint |
| `OTEL_EXPORTER_OTLP_TRACES_ENDPOINT` | unset | Optional trace-specific OTLP/HTTP endpoint |

Durations use Go syntax and must be at least one second. PostgreSQL 16 is exercised
by CI and the local stack. Keep the database on a private network and enable TLS
according to your deployment.

Generate secrets with `openssl rand -hex 32` for the administrator key and
`openssl rand -base64 32` for the encryption key. Back up the encryption key
separately from PostgreSQL. Replacing it alone makes saved provider credentials
unreadable; automated encryption-key rotation is not implemented. Provider
credentials themselves can be rotated through the console.
