package migrations

import "embed"

// Files contains the versioned SQL migration files used by the runtime migrator.
//
//go:embed *.sql
var Files embed.FS
