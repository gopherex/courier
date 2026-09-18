#!/usr/bin/env python3
"""Create a local-only development environment without replacing existing secrets."""
from pathlib import Path
import secrets
import base64

path = Path(".env")
with path.open("x") as output:
    output.write("DATABASE_URL=postgres://courier:courier@localhost:15432/courier?sslmode=disable\n")
    output.write("COURIER_DEVELOPMENT=true\nCOURIER_LISTEN=127.0.0.1:18080\nCOURIER_PUBLIC_URL=http://localhost:18080\n")
    output.write("COURIER_ADMIN_KEY=" + secrets.token_urlsafe(32) + "\n")
    output.write("COURIER_ENCRYPTION_KEY=" + base64.b64encode(secrets.token_bytes(32)).decode() + "\n")
path.chmod(0o600)
print("Created .env; the local administrator key is in COURIER_ADMIN_KEY.")
