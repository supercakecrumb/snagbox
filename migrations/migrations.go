// Package migrations embeds the goose SQL migration files so they can be
// applied from a single compiled binary without shipping loose .sql files.
package migrations

import "embed"

// FS contains every *.sql migration file in this directory.
//
//go:embed *.sql
var FS embed.FS
