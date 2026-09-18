# Docker deployment

Build locally:

```bash
docker build -t courier:local .
```

The image contains a statically compiled Go service and built console assets,
runs as non-root and listens on port 8080. It needs PostgreSQL and runtime secrets.
After a release, the image is available as `ghcr.io/gopherex/courier:VERSION`.
Pin a version or digest rather than using `latest` in production.

Create a private environment file with the [required variables](configuration.md).
The database URL must be reachable from the container; `localhost` inside a
container is not the host. Set `COURIER_PUBLIC_URL` to your HTTPS origin and
`COURIER_DEVELOPMENT=false`.

```bash
# Apply migrations before starting the service.
docker run --rm --env-file /secure/courier.env courier:local migrate
# Put an HTTPS reverse proxy in front of the loopback listener.
docker run -d --name courier --restart unless-stopped \
  --env-file /secure/courier.env -p 127.0.0.1:8080:8080 courier:local
```

The repository's Compose file starts development PostgreSQL and Mailpit only;
`make run` starts Courier on the host. Production has no dependency on Mailpit.

Multiple service replicas share PostgreSQL sessions, admission locks and leased
jobs. Apply migrations as a single deployment job before rolling out replicas.
Migrations include the imported pg-outbox schema: do not independently apply the
library's migrations to the same database. Back up before schema upgrades.

SIGTERM cancels workers and drains HTTP. For a binary deployment, ship `web/dist`
with the binary and set `COURIER_ADMIN_DIR` if it is elsewhere. Assets are served
from disk, not embedded in the executable.
