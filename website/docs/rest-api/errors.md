# Errors

Failures use a JSON object with `code` and `message`. Stable codes omit addresses,
credentials, template data and raw provider errors. Use `code` for program logic.

| Status | Codes and meaning |
| --- | --- |
| 400 | `invalid_request`: malformed JSON or invalid wire shape |
| 401 | `unauthorized`: invalid/revoked key or expired session |
| 403 | `origin_rejected`: rejected cross-origin admin mutation |
| 404 | `project_not_found`, `delivery_not_found`: missing scoped resource |
| 409 | `idempotency_conflict`: key reused with another body; `expired`: replay no longer allowed; `delivery_not_dead`: no longer in DLQ |
| 422 | `notification_unavailable`, `channel_unavailable`, `provider_missing`, `duplicate_channel`, `invalid_target`, `invalid_data`, `invalid_schema`, `invalid_template`, `invalid_configuration`, `invalid_provider`, `duplicate_preference`, `expired` |
| 429 | `rate_limited`: administrator login attempt limit |
| 500 | `internal_error`: unexpected internal failure; `provider_decryption_failed`: configured credentials cannot be decrypted |

A rejected request is not durably accepted. Correct invalid input before retrying.
For a timeout or connection loss, the outcome can be uncertain: retry the same body
with the same idempotency key. Provider failures after admission are asynchronous;
inspect dead letters and operational metrics rather than expecting another HTTP
response or a delivery callback.
