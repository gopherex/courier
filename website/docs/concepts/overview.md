# Projects and notification keys

A project owns notification configuration, provider credentials, service keys and
recipient preferences. Service bearer keys determine the project: send requests
cannot select a different project. The global administrator manages all projects.

A notification key such as `registration` is defined by the administrator before
applications send it. Clients do not register keys at startup. Disabled or unknown
keys reject new messages. Each notification has translated names/descriptions and
one to three distinct channels (`email`, `webpush`, `fcm`).

Each channel has its own strict data schema and localized templates. Clients send
separate data and addresses per channel. There are no inline templates and no
automatic fallback from one channel to another.

The admin project update replaces the complete configuration. Saving immediately
affects new admissions. Existing jobs retain their rendered content and addresses;
provider credentials are looked up again for every delivery attempt. Disabling a
key stops new admissions, but does not cancel already admitted jobs.

See [delivery semantics](delivery.md), [preferences](preferences.md) and
[template configuration](../guides/templates.md).
