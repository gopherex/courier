# Operations and observability

| Endpoint | Purpose |
| --- | --- |
| `/healthz` | Process liveness |
| `/readyz` | PostgreSQL reachability |
| `/metrics` | Prometheus metrics |

Restrict metrics and database access to your operational network. Readiness does
not probe SMTP or push providers. Monitor provider outcomes separately.

| Metric | Useful signal |
| --- | --- |
| `courier_admissions_total` | Accepted, rejected and entirely skipped admissions |
| `courier_queue_jobs` | Pending, processing and dead row counts |
| `courier_queue_oldest_age_seconds` | Age of oldest row by queue status |
| `courier_delivery_attempts_total` | Attempt outcomes by channel |
| `courier_delivery_outcomes_total` | Terminal queue outcomes |

Start with alerts on dead jobs, growing pending age, increasing delivery failure
rate and readiness/metrics unavailability. Set thresholds from the time budget of
your registration or verification flow. Labels are bounded; no recipient IDs,
addresses or message content are used as metric labels.

JSON logs go to stdout. Optional OTLP/HTTP delivery spans use the configured
collector. Generated HTTP decoding traces are disabled because decoding failures
can contain input data. Raw provider errors, payloads and credentials are not
persisted in telemetry. Telemetry is not a delivery history.

Back up PostgreSQL and the encryption key. Monitor disk growth and cleanup lag.
After restoring an old database, previously delivered jobs may be present again;
at-least-once semantics still apply. Check provider connectivity with a controlled
test recipient before enabling production traffic.
