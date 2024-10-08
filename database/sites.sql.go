// Code generated by sqlc. DO NOT EDIT.
// versions:
//   sqlc v1.25.0
// source: sites.sql

package database

import (
	"context"
)

const getSiteByDomain = `-- name: GetSiteByDomain :one
SELECT id, domain, token
FROM sites
WHERE domain = ?
LIMIT 1
`

func (q *Queries) GetSiteByDomain(ctx context.Context, domain string) (Site, error) {
	row := q.db.QueryRowContext(ctx, getSiteByDomain, domain)
	var i Site
	err := row.Scan(&i.ID, &i.Domain, &i.Token)
	return i, err
}
