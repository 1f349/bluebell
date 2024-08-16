// Code generated by sqlc. DO NOT EDIT.
// versions:
//   sqlc v1.25.0
// source: sites.sql

package database

import (
	"context"
)

const deleteDomain = `-- name: DeleteDomain :exec
UPDATE sites
SET enable = false
WHERE domain = ?
`

func (q *Queries) DeleteDomain(ctx context.Context, domain string) error {
	_, err := q.db.ExecContext(ctx, deleteDomain, domain)
	return err
}

const enableDomain = `-- name: EnableDomain :exec
INSERT INTO sites (slug, domain, token)
VALUES (?, ?, ?)
`

type EnableDomainParams struct {
	Slug   string `json:"slug"`
	Domain string `json:"domain"`
	Token  string `json:"token"`
}

func (q *Queries) EnableDomain(ctx context.Context, arg EnableDomainParams) error {
	_, err := q.db.ExecContext(ctx, enableDomain, arg.Slug, arg.Domain, arg.Token)
	return err
}

const getSiteByDomain = `-- name: GetSiteByDomain :one
SELECT id, slug, domain, token, enable
FROM sites
WHERE domain = ?
LIMIT 1
`

func (q *Queries) GetSiteByDomain(ctx context.Context, domain string) (Site, error) {
	row := q.db.QueryRowContext(ctx, getSiteByDomain, domain)
	var i Site
	err := row.Scan(
		&i.ID,
		&i.Slug,
		&i.Domain,
		&i.Token,
		&i.Enable,
	)
	return i, err
}

const getSiteBySlug = `-- name: GetSiteBySlug :one
SELECT id, slug, domain, token, enable
FROM sites
WHERE slug = ?
LIMIT 1
`

func (q *Queries) GetSiteBySlug(ctx context.Context, slug string) (Site, error) {
	row := q.db.QueryRowContext(ctx, getSiteBySlug, slug)
	var i Site
	err := row.Scan(
		&i.ID,
		&i.Slug,
		&i.Domain,
		&i.Token,
		&i.Enable,
	)
	return i, err
}