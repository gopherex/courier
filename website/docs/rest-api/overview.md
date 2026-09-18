# REST API

[Download the OpenAPI contract](pathname:///courier/openapi.yaml) or inspect its
[source](https://github.com/gopherex/courier/blob/master/openapi/openapi.yaml).
OpenAPI 3.0.3 generates the Go server/client and TypeScript SDK. No separate
handwritten request/response types are maintained.

## Send a notification

Create `registration` with a required `name` field and an email provider first.
The service key is issued in the console. In this shell example, set
`COURIER_SERVICE_KEY` to that secret.

```bash
curl --fail-with-body http://localhost:18080/v1/messages \
  -H "Authorization: Bearer $COURIER_SERVICE_KEY" \
  -H 'Content-Type: application/json' \
  --data '{
    "notification_key":"registration",
    "idempotency_key":"registration:event-42",
    "recipient_id":"user-42",
    "locale":"en",
    "deliveries":[{
      "channel":"email","default_enabled":true,
      "email":{"address":"alex@example.test"},
      "data":{"name":"Alex"}
    }]
  }'
```

`202` returns `accepted`, `message_id` and `queued_channels`. It confirms durable
admission. Retry a lost response with the identical request and idempotency key.
See [delivery semantics](../concepts/delivery.md).

## Endpoints

| Method | Path | Authentication |
| --- | --- | --- |
| POST | `/v1/messages` | Service bearer key |
| GET, PUT | `/v1/recipients/{recipient_id}/preferences` | Service bearer key |
| POST | `/admin/session` | Administrator key in JSON body |
| DELETE | `/admin/session` | Session cookie |
| GET | `/admin/projects` | Session cookie |
| PUT | `/admin/projects/{project_id}` | Session cookie |
| GET, PUT | `/admin/projects/{project_id}/providers` | Session cookie |
| GET, POST | `/admin/projects/{project_id}/keys` | Session cookie |
| DELETE | `/admin/projects/{project_id}/keys/{key_id}` | Session cookie |
| POST | `/admin/projects/{project_id}/preview` | Session cookie |
| POST | `/admin/projects/{project_id}/test` | Session cookie |
| GET | `/admin/projects/{project_id}/dead-letters` | Session cookie |
| POST | `/admin/projects/{project_id}/dead-letters/{delivery_id}/replay` | Session cookie |

Service keys are project-scoped. The admin key does not work as a bearer key.
Login accepts `{"key":"..."}` and sets `courier_session`, HttpOnly, SameSite=Strict,
Secure outside development, scoped to `/admin`. Mutating admin requests reject
cross-origin requests. The login limit is ten attempts per minute shared by replicas.

Use trusted backends to call the service API; do not expose service keys to end users.
Preferences PUT and project PUT replace the full corresponding configuration.
