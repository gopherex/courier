# Admission, retries and retention

## Admission

Courier locks the project's configuration, loads preferences, validates and renders
every allowed channel, then inserts admission metadata and queue jobs in one
PostgreSQL transaction. If any allowed channel fails validation, the whole message
is rejected. A blocked channel does not require a provider or valid rendered template.
Wire-format validation still happens before preference filtering.

The response contains `accepted: true`, a `message_id` and `queued_channels`.
When every requested channel is blocked, the response is still `202`, with an empty
channel list and no jobs. There is no partial acceptance response.

The idempotency key is scoped to a project. Repeating the same request returns the
original acceptance, even if configuration changed. Different content returns
`409 idempotency_conflict`. Object key order is ignored; array order and JSON
number representations are significant. Rejected requests do not reserve a key.
Retry an uncertain HTTP outcome with the same key and unchanged body.

## Delivery

Workers claim jobs using renewable, fenced PostgreSQL leases. Delivery runs outside
the admission transaction. Defaults are four workers, 12 attempts, a 30-second
attempt timeout and a 90-second lease. Retry uses exponential backoff with full
jitter, from one second to one hour. Provider `Retry-After` is honored.

SMTP network errors and 4xx replies are retryable; 5xx replies are permanent.
HTTP 408, 429 and 5xx are retryable. Other HTTP failures are permanent.
Permanent failures and retry exhaustion enter the dead-letter queue.

Delivery is at least once. If the provider accepted a message but its response was
lost, retry can cause duplicates. SMTP Message-ID and content stay stable across
retries. Successful SMTP DATA acceptance remains success even if QUIT fails.
Provider acceptance is not proof of receipt or reading.

Before every attempt, workers recheck expiration and recipient preferences.
An expired job enters DLQ and cannot be replayed. A newly blocked channel is
skipped. Other channels are never substituted. Push TTL is capped at one hour or
the remaining expiration interval, whichever is shorter.

## Retention

Successful and skipped payloads are cleared atomically at completion; operational
rows remain for one hour. Dead payloads remain for seven days by default.
Original template data is not retained in jobs; rendered content and addresses are.

Deduplication metadata contains a fingerprint and acceptance, without message
content. Once all jobs leave pending/processing/DLQ, cleanup starts a full
seven-day deduplication window. Cleanup runs periodically; retention is an
eligibility threshold, not an exact deletion time. After metadata expires, a
reused key can create a new message.

See [configuration](../self-hosting/configuration.md) for overrides and
[dead-letter recovery](../guides/dead-letters.md).
