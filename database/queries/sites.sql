-- name: GetSiteBySlug :one
SELECT *
FROM sites
WHERE slug = ?
LIMIT 1;

-- name: GetSiteByDomain :one
SELECT *
FROM sites
WHERE domain = ?
LIMIT 1;

-- name: EnableDomain :exec
INSERT INTO sites (slug, domain, token)
VALUES (?, ?, ?);

-- name: DeleteDomain :exec
UPDATE sites
SET enable = false
WHERE domain = ?;
