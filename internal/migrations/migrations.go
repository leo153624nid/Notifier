// Package migrations embeds the SQL migration files applied to the
// notifier database. Files are versioned and consumed by cmd/migrate
// via golang-migrate (see Makefile: migrate-up / migrate-down).
package migrations

import "embed"

//go:embed *.sql
var FS embed.FS
