-- name: GetSiteByDomain :one
SELECT *
FROM sites
WHERE domain = ?
LIMIT 1;
