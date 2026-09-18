# Go SDK

The module exports generated wire types and client in `pkg/api`; `pkg/sdk.New`
configures project bearer authentication. Pin a published module version for
production; before the first release use a local checkout or commit version.

```go
import (
    "context"
    "github.com/go-faster/jx"
    "github.com/gopherex/courier/pkg/api"
    "github.com/gopherex/courier/pkg/sdk"
)

func notify(ctx context.Context, endpoint, key string) (*api.Accepted, error) {
    client, err := sdk.New(endpoint, key)
    if err != nil { return nil, err }
    return client.Send(ctx, &api.SendRequest{
        NotificationKey: "registration",
        IdempotencyKey: "registration:event-42",
        Locale: api.NewOptString("en"),
        Deliveries: []api.DeliveryInput{{
            Channel: api.ChannelEmail,
            DefaultEnabled: true,
            Email: api.NewOptEmailTarget(api.EmailTarget{Address: "alex@example.test"}),
            Data: api.Data{"name": jx.Raw(`"Alex"`)},
        }},
    })
}
```

The returned acceptance is not a delivery receipt. Reuse the event's idempotency key
on retries. Do not generate a new key for each network attempt. The client accepts
generated `api.ClientOption` values for HTTP transport and other client settings.
Administrator operations require a session and are not authenticated by this helper.
