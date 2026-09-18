-- sqld:up
CREATE TABLE "public"."admin_sessions" (
  "digest" bytea NOT NULL,
  "expires_at" timestamptz NOT NULL,
  PRIMARY KEY ("digest")
);
CREATE TABLE "public"."admissions" (
  "project_id" text NOT NULL,
  "idempotency_key" text NOT NULL,
  "fingerprint" bytea NOT NULL,
  "result" jsonb NOT NULL,
  "retain_until" timestamptz NOT NULL,
  PRIMARY KEY ("project_id", "idempotency_key")
);
CREATE TABLE "public"."api_keys" (
  "id" uuid NOT NULL,
  "project_id" text NOT NULL,
  "name" text NOT NULL,
  "digest" bytea NOT NULL,
  "created_at" timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY ("id")
);
CREATE TABLE "public"."login_limits" (
  "bucket" text NOT NULL,
  "attempts" int4 NOT NULL,
  "reset_at" timestamptz NOT NULL,
  PRIMARY KEY ("bucket")
);
CREATE TABLE "public"."outbox_messages" (
  "id" uuid NOT NULL DEFAULT gen_random_uuid(),
  "topic" text NOT NULL,
  "partition_key" text,
  "payload" bytea NOT NULL,
  "headers" jsonb NOT NULL DEFAULT '{}'::jsonb,
  "content_type" text,
  "message_type" text,
  "status" text NOT NULL DEFAULT 'pending'::text,
  "attempts" int4 NOT NULL DEFAULT 0,
  "max_attempts" int4,
  "locked_by" text,
  "locked_until" timestamptz,
  "next_attempt_at" timestamptz NOT NULL DEFAULT now(),
  "last_error" text,
  "created_at" timestamptz NOT NULL DEFAULT now(),
  "published_at" timestamptz,
  "lease_token" uuid,
  "finished_at" timestamptz,
  PRIMARY KEY ("id")
);
CREATE TABLE "public"."preferences" (
  "project_id" text NOT NULL,
  "recipient_id" text NOT NULL,
  "rules" jsonb NOT NULL,
  PRIMARY KEY ("project_id", "recipient_id")
);
CREATE TABLE "public"."projects" (
  "id" text NOT NULL,
  "config" jsonb NOT NULL,
  "updated_at" timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY ("id")
);
CREATE TABLE "public"."providers" (
  "project_id" text NOT NULL,
  "channel" text NOT NULL,
  "ciphertext" bytea NOT NULL,
  PRIMARY KEY ("project_id", "channel")
);
ALTER TABLE "public"."api_keys" ADD CONSTRAINT "api_keys_digest_key" UNIQUE ("digest");
ALTER TABLE "public"."outbox_messages" ADD CONSTRAINT "outbox_messages_status_check" CHECK (status = ANY (ARRAY['pending'::text, 'processing'::text, 'published'::text, 'dead'::text, 'discarded'::text]));
ALTER TABLE "public"."providers" ADD CONSTRAINT "providers_channel_check" CHECK (channel = ANY (ARRAY['email'::text, 'webpush'::text, 'fcm'::text]));
CREATE INDEX "admissions_expiry_idx" ON "public"."admissions" ("retain_until");
CREATE INDEX "outbox_messages_claim_idx" ON "public"."outbox_messages" ("next_attempt_at", "created_at") WHERE (status = ANY (ARRAY['pending'::text, 'processing'::text]));
CREATE INDEX "outbox_messages_cleanup_idx" ON "public"."outbox_messages" ("published_at") WHERE (status = 'published'::text);
CREATE INDEX "outbox_messages_finished_idx" ON "public"."outbox_messages" ("finished_at") WHERE (status = ANY (ARRAY['dead'::text, 'discarded'::text]));
CREATE INDEX "outbox_messages_partition_idx" ON "public"."outbox_messages" ("partition_key", "created_at") WHERE (partition_key IS NOT NULL);
ALTER TABLE "public"."admissions" ADD CONSTRAINT "admissions_project_id_fkey" FOREIGN KEY ("project_id") REFERENCES "public"."projects" ("id");
ALTER TABLE "public"."api_keys" ADD CONSTRAINT "api_keys_project_id_fkey" FOREIGN KEY ("project_id") REFERENCES "public"."projects" ("id");
ALTER TABLE "public"."preferences" ADD CONSTRAINT "preferences_project_id_fkey" FOREIGN KEY ("project_id") REFERENCES "public"."projects" ("id");
ALTER TABLE "public"."providers" ADD CONSTRAINT "providers_project_id_fkey" FOREIGN KEY ("project_id") REFERENCES "public"."projects" ("id");
CREATE FUNCTION "public"."outbox_messages_notify"() RETURNS trigger LANGUAGE plpgsql VOLATILE PARALLEL UNSAFE AS $$
BEGIN
    PERFORM pg_notify('outbox_messages', '');
    RETURN NULL;
END;
$$;
CREATE TRIGGER "outbox_messages_notify_trg" AFTER INSERT ON "public"."outbox_messages" FOR EACH STATEMENT EXECUTE FUNCTION "public"."outbox_messages_notify"();

-- sqld:down
DROP TRIGGER "outbox_messages_notify_trg" ON "public"."outbox_messages";
DROP FUNCTION "public"."outbox_messages_notify"();
ALTER TABLE "public"."providers" DROP CONSTRAINT "providers_project_id_fkey";
ALTER TABLE "public"."preferences" DROP CONSTRAINT "preferences_project_id_fkey";
ALTER TABLE "public"."api_keys" DROP CONSTRAINT "api_keys_project_id_fkey";
ALTER TABLE "public"."admissions" DROP CONSTRAINT "admissions_project_id_fkey";
DROP INDEX "public"."outbox_messages_partition_idx";
DROP INDEX "public"."outbox_messages_finished_idx";
DROP INDEX "public"."outbox_messages_cleanup_idx";
DROP INDEX "public"."outbox_messages_claim_idx";
DROP INDEX "public"."admissions_expiry_idx";
ALTER TABLE "public"."providers" DROP CONSTRAINT "providers_channel_check";
ALTER TABLE "public"."outbox_messages" DROP CONSTRAINT "outbox_messages_status_check";
ALTER TABLE "public"."api_keys" DROP CONSTRAINT "api_keys_digest_key";
DROP TABLE "public"."providers";
DROP TABLE "public"."projects";
DROP TABLE "public"."preferences";
DROP TABLE "public"."outbox_messages";
DROP TABLE "public"."login_limits";
DROP TABLE "public"."api_keys";
DROP TABLE "public"."admissions";
DROP TABLE "public"."admin_sessions";
