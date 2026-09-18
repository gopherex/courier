---
slug: /
title: Courier
---

Courier delivers notifications for trusted applications over SMTP email, WebPush
and Firebase Cloud Messaging. A Go service owns the API, administration console
and delivery workers; PostgreSQL stores configuration and the durable queue.

Applications choose recipients and channels. They provide addresses and data for
every message. Courier applies preferences, validates strict schemas, renders
localized templates and queues all allowed deliveries in one transaction.

- [Run locally](quickstart.md) and send a registration email to Mailpit.
- [Configure the console](guides/admin.md), templates and provider credentials.
- Integrate through [Go](sdk/go.md), [TypeScript](sdk/typescript.md) or the [REST API](rest-api/overview.md).
- [Deploy](self-hosting/docker.md) with retries, dead letters and observability.

Courier does not manage identities, address books, campaigns, channel fallbacks
or a permanent notification history. IAM is not a runtime dependency. A `202`
response confirms durable admission, not delivery to a person.
