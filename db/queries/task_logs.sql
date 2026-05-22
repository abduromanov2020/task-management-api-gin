-- name: InsertTaskLog :exec
INSERT INTO task_logs (task_id, actor_id, action, payload)
VALUES ($1, $2, $3, $4);

-- name: ListTaskLogs :many
SELECT * FROM task_logs
WHERE task_id = $1
ORDER BY created_at DESC;
