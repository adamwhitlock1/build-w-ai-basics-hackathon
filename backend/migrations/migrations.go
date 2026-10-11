// Package migrations holds the SQL files that build Porchlight's database
// schema. They are embedded into the binary, so the server never needs the
// .sql files next to it at run time.
package migrations

import "embed"

// FS holds every .sql file in this directory. store.Open applies the ones
// the database hasn't seen yet, in filename order (001_, 002_, ...).
//
// The comment below is a compiler directive, not documentation: go:embed
// copies the matching files into the binary when it is built.
//
//go:embed *.sql
var FS embed.FS
