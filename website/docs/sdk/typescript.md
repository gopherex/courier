# TypeScript SDK

`@gopherex/courier-sdk` is generated from the same contract as the Go server.
Releases publish to **GitHub Packages**, not npmjs.org. After a release, configure
the scope in your consuming project's `.npmrc`:

```ini
@gopherex:registry=https://npm.pkg.github.com
//npm.pkg.github.com/:_authToken=${NODE_AUTH_TOKEN}
```

Use a token with package read access and install the desired published version.
Before the first release, run `make node-deps build-sdk` in the checkout and use the
workspace package or a local packed tarball.

```ts
import { send } from '@gopherex/courier-sdk';

const { data } = await send({
  baseUrl: 'https://courier.example',
  auth: serviceKey,
  throwOnError: true,
  body: {
    notification_key: 'registration',
    idempotency_key: 'registration:event-42',
    locale: 'en',
    deliveries: [{
      channel: 'email', default_enabled: true,
      email: { address: 'alex@example.test' },
      data: { name: 'Alex' },
    }],
  },
});
console.log(data.message_id);
```

Call from a trusted backend. Never bundle `serviceKey` into a public application.
Per-call options support separate projects without mutating a shared global client.
Generated types include `SendRequest`, `DeliveryInput`, `Preferences` and `Problem`.
With `throwOnError: true`, failures throw; otherwise inspect `error` and `response`.
Admin calls use session cookies; the Courier console sets `credentials: 'same-origin'`.
