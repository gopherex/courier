CREATE TABLE projects (
 id text PRIMARY KEY,
 config jsonb NOT NULL,
 updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE providers (
 project_id text NOT NULL REFERENCES projects(id),
 channel text NOT NULL CHECK (channel IN ('email','webpush','fcm')),
 ciphertext bytea NOT NULL,
 PRIMARY KEY (project_id, channel)
);
CREATE TABLE api_keys (
 id uuid PRIMARY KEY,
 project_id text NOT NULL REFERENCES projects(id),
 name text NOT NULL,
 digest bytea NOT NULL UNIQUE,
 created_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE preferences (
 project_id text NOT NULL REFERENCES projects(id),
 recipient_id text NOT NULL,
 rules jsonb NOT NULL,
 PRIMARY KEY (project_id, recipient_id)
);
CREATE TABLE admissions (
 project_id text NOT NULL REFERENCES projects(id),
 idempotency_key text NOT NULL,
 fingerprint bytea NOT NULL,
 result jsonb NOT NULL,
 retain_until timestamptz NOT NULL,
 completed_at timestamptz,
 PRIMARY KEY (project_id, idempotency_key)
);
CREATE INDEX admissions_expiry_idx ON admissions(retain_until);
CREATE TABLE admin_sessions (
 digest bytea PRIMARY KEY,
 expires_at timestamptz NOT NULL
);
CREATE TABLE login_limits (
 bucket text PRIMARY KEY,
 attempts integer NOT NULL,
 reset_at timestamptz NOT NULL
);
