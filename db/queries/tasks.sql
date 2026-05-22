-- name: CreateTask :one
INSERT INTO tasks (team_id, created_by, assignee_id, title, description, status, priority, due_date)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING *;

-- name: GetTaskByID :one
SELECT * FROM tasks WHERE id = $1;

-- name: GetTaskForUpdate :one
SELECT * FROM tasks WHERE id = $1 FOR UPDATE;

-- name: ListTasks :many
SELECT *, count(*) OVER () AS total
FROM tasks
WHERE team_id = @team_id
  AND (@status_filter::task_status IS NULL OR status = @status_filter)
  AND (@q::text = '' OR title ILIKE '%' || @q || '%')
ORDER BY created_at DESC
LIMIT @lim
OFFSET @off;

-- name: UpdateTask :one
UPDATE tasks
SET title       = $2,
    description = $3,
    status      = $4,
    priority    = $5,
    due_date    = $6,
    updated_at  = now()
WHERE id = $1
RETURNING *;

-- name: UpdateTaskAssignee :exec
UPDATE tasks
SET assignee_id = $2, updated_at = now()
WHERE id = $1;

-- name: DeleteTask :exec
DELETE FROM tasks WHERE id = $1;
