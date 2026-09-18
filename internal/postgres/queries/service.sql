-- name: GetProject :one
SELECT config FROM projects WHERE id = @id;
-- name: LockProject :one
SELECT config FROM projects WHERE id = @id FOR SHARE;
-- name: ListProjects :many
SELECT id, config FROM projects ORDER BY id;
-- name: PutProject :exec
INSERT INTO projects(id,config) VALUES (@id,@config) ON CONFLICT(id) DO UPDATE SET config=EXCLUDED.config,updated_at=now();
-- name: GetProvider :one
SELECT ciphertext FROM providers WHERE project_id=@project_id AND channel=@channel;
-- name: ListProviders :many
SELECT channel FROM providers WHERE project_id=@project_id ORDER BY channel;
-- name: PutProvider :exec
INSERT INTO providers(project_id,channel,ciphertext) VALUES (@project_id,@channel,@ciphertext)
ON CONFLICT(project_id,channel) DO UPDATE SET ciphertext=EXCLUDED.ciphertext;
-- name: Authenticate :one
SELECT project_id FROM api_keys WHERE digest=@digest;
-- name: IssueKey :exec
INSERT INTO api_keys(id,project_id,name,digest) VALUES (@id,@project_id,@name,@digest);
-- name: ListKeys :many
SELECT id,name,created_at FROM api_keys WHERE project_id=@project_id ORDER BY created_at,id;
-- name: RevokeKey :exec
DELETE FROM api_keys WHERE id=@id AND project_id=@project_id;
-- name: GetPreferences :one
SELECT rules FROM preferences WHERE project_id=@project_id AND recipient_id=@recipient_id;
-- name: PutPreferences :exec
INSERT INTO preferences(project_id,recipient_id,rules) VALUES (@project_id,@recipient_id,@rules)
ON CONFLICT(project_id,recipient_id) DO UPDATE SET rules=EXCLUDED.rules;
-- name: LockAdmission :exec
SELECT pg_advisory_xact_lock(hashtextextended(@lock_key::text, 0));
-- name: GetAdmission :one
SELECT fingerprint,result FROM admissions WHERE project_id=@project_id AND idempotency_key=@idempotency_key;
-- name: PutAdmission :exec
INSERT INTO admissions(project_id,idempotency_key,fingerprint,result,retain_until)
VALUES (@project_id,@idempotency_key,@fingerprint,@result,@retain_until);
-- name: CleanupAdmissions :exec
DELETE FROM admissions WHERE completed_at IS NOT NULL AND retain_until < now()
AND NOT EXISTS (SELECT 1 FROM outbox_messages WHERE partition_key=admissions.result->>'message_id' AND status IN ('pending','processing','dead'));
-- name: AddSession :exec
INSERT INTO admin_sessions(digest,expires_at) VALUES (@digest,@expires_at);
-- name: CheckSession :one
SELECT digest FROM admin_sessions WHERE digest=@digest AND expires_at>now();
-- name: DeleteSession :exec
DELETE FROM admin_sessions WHERE digest=@digest;
-- name: LoginAttempt :one
INSERT INTO login_limits(bucket,attempts,reset_at) VALUES (@bucket,1,now()+interval '1 minute')
ON CONFLICT(bucket) DO UPDATE SET attempts=CASE WHEN login_limits.reset_at<=now() THEN 1 ELSE login_limits.attempts+1 END,
reset_at=CASE WHEN login_limits.reset_at<=now() THEN now()+interval '1 minute' ELSE login_limits.reset_at END
RETURNING attempts;
-- name: CleanupSessions :exec
DELETE FROM admin_sessions WHERE expires_at<now();
-- name: CleanupLoginLimits :exec
DELETE FROM login_limits WHERE reset_at<now();
-- name: ListDead :many
SELECT id,payload,attempts,last_error,finished_at FROM outbox_messages
WHERE topic=@topic AND status='dead' AND id > @after::uuid ORDER BY id LIMIT 100;
-- name: LockDead :one
SELECT id,payload FROM outbox_messages WHERE id=@id AND topic=@topic AND status='dead' FOR UPDATE;
-- name: QueueStats :many
SELECT status,count(*)::bigint AS total,COALESCE(EXTRACT(EPOCH FROM now()-min(created_at)),0)::float8 AS oldest_seconds
FROM outbox_messages WHERE status IN ('pending','processing','dead') GROUP BY status;

-- name: LockProjectWrite :one
SELECT config FROM projects WHERE id=@id FOR UPDATE;

-- name: CompleteAdmissions :exec
UPDATE admissions SET completed_at=now(),retain_until=@retain_until
WHERE completed_at IS NULL AND NOT EXISTS (
 SELECT 1 FROM outbox_messages WHERE partition_key=admissions.result->>'message_id'
 AND status IN ('pending','processing','dead')
);
