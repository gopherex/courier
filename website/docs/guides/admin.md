# Administrator console

Sign in with the runtime administrator key. It is exchanged for an HttpOnly session
cookie, valid for eight hours. Logout revokes the session in PostgreSQL.

Use the project selector to scope navigation:

| Section | Operations |
| --- | --- |
| Notifications | Create keys, enable/disable them, edit schema and translated templates |
| Providers | Configure SMTP, WebPush or FCM credentials |
| Service access | Issue named project keys, copy a new secret once, revoke keys |
| Dead letters | Inspect failed payloads and replay eligible deliveries |
| Project settings | Change translated project names and default locale |

The notification editor has separate Settings, Data schema, Templates and Test
delivery tabs. Switching these editor tabs preserves input. **Save configuration**
applies the entire notification configuration. Preview and test delivery use the
saved configuration, not unsubmitted form input. Save before changing project or
leaving the editor.

The interface supports Russian and English. Template locales are independent of
the console language. Provider forms replace the complete configuration; stored
secrets are never returned to the browser. Keep credentials available when rotating
a connection. A configured status means settings exist, not that a connection
probe or a delivery has succeeded.

See [templates](templates.md), [providers](providers.md) and [dead letters](dead-letters.md).
