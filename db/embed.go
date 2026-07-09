// Package dbassets embeds SQL migration files so they can be applied at runtime.
package dbassets

import "embed"

//go:embed migrations/*.sql
var Migrations embed.FS
