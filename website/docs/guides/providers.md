# Delivery providers

Provider settings are project-scoped and encrypted with AES-256-GCM. Reads expose
only configured channels, never secrets. Updating a provider affects waiting jobs
and later retries. The sender never falls back to another channel.

## SMTP

Set host, port, TLS mode, sender address and optional username/password.
`starttls` upgrades the SMTP connection; `tls` uses implicit TLS. `plain` is allowed
only with `COURIER_DEVELOPMENT=true`. TLS uses certificate verification and a
minimum of TLS 1.2. Courier supports authenticated SMTP using PLAIN after TLS.

For the local quickstart, use `127.0.0.1:11025`, `plain` and
`courier@example.test`. Inside the Compose network Mailpit is `mailpit:1025`.
Choose a sender accepted by your production SMTP provider; domain verification and
SPF/DKIM setup are external to Courier.

## WebPush

Supply a matching P-256 VAPID public/private key pair (base64url, without padding)
and a contact subject beginning with `mailto:` or `https:`. Requests include the
client subscription's HTTPS endpoint, `p256dh` and `auth`. Subscriptions remain
owned by the application. Courier encrypts payloads and signs VAPID requests.

## Firebase Cloud Messaging

Supply the Firebase project ID and a service-account JSON document with permission
to send messages. The credential uses Google's OAuth token endpoint. Every request
includes an FCM device token. Courier uses the FCM HTTP v1 API.

SMTP is covered by a local Mailpit integration. WebPush encryption and VAPID are
covered by an HTTP fixture. Real WebPush/FCM delivery still requires verification
against your credentials, subscriptions and devices; configured status alone does
not establish delivery readiness.
