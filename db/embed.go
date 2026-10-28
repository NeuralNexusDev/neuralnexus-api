// Package db embeds the SQL schema file of each database, so tests and tools apply the schema production has.
package db

import "embed"

// FS holds neuralnexus.sql, bee_name_generator.sql, events.sql, pet_pictures.sql, twitch.sql and archive.sql.
//
//go:embed *.sql
var FS embed.FS
