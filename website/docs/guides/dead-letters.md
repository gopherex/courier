# Recover failed deliveries

Open **Dead letters** for a project to inspect terminal failures, attempt count,
error code, completion time and the frozen payload. Rows are paginated by UUID,
100 at a time. This is temporary failure storage, not notification history.

1. Inspect the safe error code and channel. Check provider configuration and metrics.
2. Correct provider credentials or connectivity as appropriate.
3. Replay the delivery from the console or the administrative replay endpoint.
4. Observe delivery outcome metrics and the actual provider/test inbox.

Replay uses the same rendered payload and addresses with current provider settings.
It resets the queue attempt state; it does not re-render templates. Preferences and
expiration are checked again. Replay can duplicate a previous uncertain delivery.
Expired payloads cannot be replayed, and removed DLQ rows cannot be recovered.

Default DLQ retention is seven days. Alert before failures age out. Avoid copying
payloads into logs or issue trackers: they can contain addresses and message content.
