# Recipient preferences

`recipient_id` is an optional opaque identifier supplied by the application.
Courier does not resolve it through IAM and does not store a user-to-address map.
The caller still includes the email, subscription or device token on every send.

Preferences are scoped to project, recipient, notification key and channel:

```json
{"rules":[{"notification_key":"registration","channel":"email","enabled":false}]}
```

A matching rule overrides the delivery's `default_enabled`. Without a matching
rule, or without `recipient_id`, Courier uses that default. Only explicitly
requested channels can be sent; enabling a preference never adds a new channel.

`PUT /v1/recipients/{recipient_id}/preferences` replaces the complete rule list.
An empty list removes all overrides. `GET` returns an empty list for a recipient
without preferences. Both operations require a project service key.

Preferences are evaluated at admission and again before each attempt. A later
block cancels pending delivery; a later enable does not recreate jobs that were
excluded during admission. The application owns authorization for end users;
these endpoints are intended for trusted backend services.
