// Package migrations предоставляет встроенные SQL-миграции для auth и server.
package migrations

import "embed"

// Files содержит все .sql файлы из этой директории.
//go:embed *.sql
var Files embed.FS
