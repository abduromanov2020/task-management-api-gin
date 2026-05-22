-- name: AcquireIdempotency :one
-- Atomically claim a fresh idempotency slot, or reclaim a stale (expired-lease)
-- in-flight row, in a single round-trip.
--   acquired=true  → caller owns the lease, proceed to do the work
--   acquired=false + status_code != 0 → completed previously, replay it
--   no row returned → another caller holds an unexpired lease (409)
INSERT INTO idempotency_keys
    (user_id, idempotency_key, request_hash, lease_expires_at)
VALUES ($1, $2, $3, now() + interval '30 seconds')
ON CONFLICT (user_id, idempotency_key) DO UPDATE
SET request_hash     = EXCLUDED.request_hash,
    lease_expires_at = EXCLUDED.lease_expires_at,
    status_code      = 0,
    response_body    = NULL
WHERE (idempotency_keys.status_code = 0
       AND idempotency_keys.lease_expires_at < now())
   OR (idempotency_keys.status_code != 0
       AND idempotency_keys.created_at < now() - interval '24 hours')
RETURNING (xmax = 0) AS acquired,
          status_code, response_body, request_hash;

-- name: GetIdempotency :one
-- Used after an Acquire returns no row, to fetch the completed/in-flight record.
SELECT status_code, response_body, request_hash, lease_expires_at
FROM idempotency_keys
WHERE user_id = $1 AND idempotency_key = $2;

-- name: CompleteIdempotency :exec
UPDATE idempotency_keys
SET status_code = $3, response_body = $4
WHERE user_id = $1 AND idempotency_key = $2;
