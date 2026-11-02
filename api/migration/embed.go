// Package migration embeds the checked-in SQL migration chain for the
// operator command.
package migration

import "embed"

// Embedded contains the versioned SQL files and their Atlas checksum.
//
//go:embed *.sql atlas.sum
var Embedded embed.FS
