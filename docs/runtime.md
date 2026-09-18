# Runtime behavior

## Admission

The authenticated service key determines the project. A request contains one to
three distinct channels, a notification key and a required idempotency key.
Targets and data belong to individual channels. Only active notification keys
are admitted; clients do not register keys or send templates.

An admission transaction locks the project configuration against concurrent
updates, loads recipient preferences once, checks the allowed channels, renders
all their templates and inserts the deduplication result and jobs atomically.
A failure rolls back every insert. Blocked channels do not require provider or
template preparation. When all are blocked, the result is accepted with an empty
`queued_channels` list. PostgreSQL failures never bypass preference checks.

Project-scoped advisory locks serialize concurrent uses of an idempotency key.
The fingerprint is SHA-256 of the generated request's JSON with recursively sorted
object keys. Array order and JSON numeric representation are significant. Missing
optional values and explicit empty values are distinct. The original accepted
result is returned before consulting current configuration. Different content
returns `409`; rejected requests are not retained and can be corrected and retried.

Addresses, rendered content, selected locale, creation time and expiration are
frozen in each job. The original dynamic template data is not retained. Provider
settings are loaded and decrypted on every attempt. Provider updates apply to
queued jobs; changes to templates do not.

## Delivery and retention

The pg-outbox relay runs one delivery per batch. Defaults: four workers, 12
attempts, 30-second attempt timeout and 90-second renewable lease. Retry delay is
exponential with full jitter, starting at one second and capped at one hour.
Provider `Retry-After` is honored; HTTP 408, 429 and 5xx are temporary. Other
HTTP failures and SMTP 5xx are permanent. SMTP network/4xx failures retry.
Permanent errors and exhausted retries enter the dead-letter queue.

Before every attempt Courier rechecks preferences and expiration. A new block
ends the job as skipped. Initially excluded channels are never recreated.
Expiration cancels in-progress network activity and prohibits subsequent attempts;
a provider may already have accepted a request before cancellation. Expired jobs
enter DLQ and cannot be replayed. Administrative replay locks and checks the dead
row in the same transaction as requeue; it preserves the frozen payload.

A successful SMTP DATA reply is the success boundary, regardless of QUIT outcome.
Message-ID, date and MIME boundary stay stable across retries. An uncertain outcome
is retried and may produce a duplicate. Provider acceptance does not mean the
person received/read the notification. Push TTL is at most one hour and is reduced
to the remaining notification lifetime when `expires_at` is supplied.

Successful/skipped payloads and headers are cleared atomically by the relay;
operational rows are cleaned after one hour. Dead payloads remain for seven days
by default. Deduplication metadata has no addresses or content. After every job has
left pending/processing/DLQ, cleanup starts a full deduplication retention window
(seven days by default). Cleanup runs periodically; retention is an eligibility
threshold, not an exact deletion deadline.

Runtime overrides: `COURIER_WORKERS`, `COURIER_MAX_ATTEMPTS`,
`COURIER_ATTEMPT_TIMEOUT`, `COURIER_LEASE_DURATION`,
`COURIER_DEDUP_RETENTION`, `COURIER_DEAD_RETENTION`. Request bodies are limited
to 1 MiB, individual rendered fields to 256 KiB. Defaults are operational settings,
not additional delivery guarantees.

## Schemas and localization

The Go schemapb runtime is authoritative. Every nested schema and definition must
be strict. Coercion, defaults, computed fields, normalization, immutable/default
fields and conditional field gates are rejected. CEL validation uses a cost limit
of 10,000. Integers are decoded without passing through float64. The browser uses
the same pinned schemapb implementation for editor feedback; the full JSON editor
supports advanced nested schemas and constraints beyond the basic field editor.

Locale selection is exact BCP-47 tag, then base language, then project default.
Configured tags must use canonical spelling. Every channel needs a complete default
locale template. Email subject/text/HTML are selected together. Go text/template
and html/template both use missingkey=error. Preview and test delivery use saved
configuration and the same renderer as normal delivery.

## Administrator console

The console uses a compact dark layout with persistent project navigation.
The project selector scopes notifications, providers, service access and dead
letters. Notification keys open in a split list/editor view; settings, data
schema, localized templates and test delivery are separate editor tabs. Switching
these tabs preserves form input. The save action applies the entire notification
configuration; preview and test delivery use the saved configuration.

Provider tables show configuration status; service key tables show issuance
timestamps and revocation actions.
The interface supports Russian and English, keyboard focus indicators and narrow
screens. System fonts are used without external font requests.

## Authentication and secrets

Service keys are random, rotatable and scoped to one project, with sending and
recipient-preference permissions. Only SHA-256 digests are stored. Revocation
applies to subsequent requests. The administrator master key is separate and comes
from runtime configuration. Login is limited to ten attempts per minute across all
replicas. Sessions are random, stored as digests in PostgreSQL, last eight hours
and are revoked on logout. Cookies are HttpOnly, SameSite=Strict and Secure outside
development. Cross-origin admin mutations are rejected.

Providers are encrypted using AES-256-GCM with fresh nonces and project/channel
bound authenticated data. The 32-byte encryption key is supplied as base64 through
`COURIER_ENCRYPTION_KEY`; back it up separately from PostgreSQL. Replacing this key
alone does not re-encrypt existing rows. Automated encryption-key rotation is not
implemented. Provider credential rotation is available by replacing the connection
settings in the admin UI. Full provider secrets are never returned by read APIs.

## Operations

Apply migrations through the explicit migration command before serving traffic.
Readiness checks database reachability; startup also verifies the schema. SIGTERM
cancels the relay and drains HTTP. Lease fencing prevents a stale worker from
completing a job claimed by another replica; external side effects remain
at-least-once.

Metrics include `courier_admissions_total` (accepted, rejected or entirely skipped),
`courier_queue_jobs`, `courier_queue_oldest_age_seconds`,
`courier_delivery_attempts_total` and `courier_delivery_outcomes_total`. Labels are
bounded channel/status/result values, never recipient or message identifiers.
Useful alerts: dead jobs > 0, increasing age of pending jobs, rising failed-attempt
rate and unavailable readiness/metrics. OTel delivery spans are exported only when
an OTLP endpoint is configured. Raw provider error bodies are neither persisted
nor logged. Generated HTTP decoder traces are disabled because validation errors
can include input values.

SMTP is verified against a local sink, without sending external mail. Real WebPush
and FCM delivery requires valid project credentials/subscriptions and must be
verified on the intended providers before production use.
