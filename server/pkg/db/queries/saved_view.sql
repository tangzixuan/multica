-- name: CountSavedViews :one
SELECT COUNT(*)::int AS count FROM saved_view
WHERE workspace_id = $1 AND page = $2
  AND (sqlc.narg('project_id')::uuid IS NULL OR project_id = sqlc.narg('project_id'));

-- name: ListSavedViews :many
-- V1: all views visible to all workspace members. Future: add
-- (shared = true OR creator_id = @user_id) filter for private views.
SELECT * FROM saved_view
WHERE workspace_id = $1 AND page = $2
  AND (sqlc.narg('project_id')::uuid IS NULL OR project_id = sqlc.narg('project_id'))
ORDER BY position ASC, created_at ASC;

-- name: GetSavedView :one
SELECT * FROM saved_view
WHERE id = $1 AND workspace_id = $2;

-- name: CreateSavedView :one
INSERT INTO saved_view (workspace_id, creator_id, name, page, project_id, filters, display, position, shared, is_default)
VALUES ($1, $2, $3, $4, sqlc.narg('project_id'), $5, $6, $7, $8, $9)
RETURNING *;

-- name: UpdateSavedView :one
UPDATE saved_view SET
    name = COALESCE(sqlc.narg('name'), name),
    filters = COALESCE(sqlc.narg('filters'), filters),
    display = COALESCE(sqlc.narg('display'), display),
    shared = COALESCE(sqlc.narg('shared'), shared),
    updated_at = now()
WHERE id = $1 AND workspace_id = $2
RETURNING *;

-- name: DeleteSavedView :exec
DELETE FROM saved_view WHERE id = $1 AND workspace_id = $2;

-- name: UpdateSavedViewPosition :exec
UPDATE saved_view SET position = $1, updated_at = now()
WHERE id = $2 AND workspace_id = $3;

-- name: GetMaxSavedViewPosition :one
SELECT COALESCE(MAX(position), 0)::float8 AS max_position
FROM saved_view
WHERE workspace_id = $1 AND page = $2
  AND (sqlc.narg('project_id')::uuid IS NULL OR project_id = sqlc.narg('project_id'));

-- name: EnsureDefaultViews :exec
-- Lazy-create default views for a page. Called on first access.
-- ON CONFLICT DO NOTHING ensures idempotency.
INSERT INTO saved_view (workspace_id, name, page, project_id, filters, display, position, shared, is_default)
VALUES ($1, $2, $3, sqlc.narg('project_id'), $4, $5, $6, true, $7)
ON CONFLICT DO NOTHING;
