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

-- name: AddSite :exec
INSERT INTO sites (domain, token)
VALUES (?, ?);

-- name: UpdateSiteToken :exec
UPDATE sites
SET token = ?
WHERE domain = ?;

-- name: AddBranch :exec
INSERT INTO branches (domain, branch, last_update, enable)
VALUES (?, ?, ?, ?);

-- name: UpdateBranch :exec
UPDATE branches
SET last_update = ?
WHERE domain = ?
  AND branch = ?;

-- name: SetBranchEnabled :exec
UPDATE branches
SET enable = ?
WHERE domain = ?
  AND branch = ?;
