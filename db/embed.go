// Package dbassets embeds SQL migration files so they can be applied at runtime.
package dbassets

import "embed"

// Migrations holds the embedded SQL migration files applied by store.Migrate.
//
//go:embed migrations/*.sql
var Migrations embed.FS
