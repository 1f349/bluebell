-- name: GetSiteByDomain :one
SELECT *
FROM sites
WHERE domain = ?
LIMIT 1;

-- name: GetLastUpdatedByDomainBranch :one
SELECT last_update
FROM branches
WHERE domain = ?
  AND branch = ?
  AND enable = true
LIMIT 1;

-- name: AddSiteDomain :exec
INSERT INTO sites (domain, token)
VALUES (?, ?);

-- name: SetDomainBranchEnabled :exec
UPDATE branches
SET enable = ?
WHERE domain = ?
  AND branch = ?;
