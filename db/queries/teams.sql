-- name: CreateTeam :one
INSERT INTO teams (name) VALUES ($1) RETURNING *;

-- name: GetTeam :one
SELECT * FROM teams WHERE id = $1;
