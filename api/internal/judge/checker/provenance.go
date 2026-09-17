package checker

import (
	"crypto/sha256"
	"embed"
	"encoding/binary"
	"fmt"
	"hash"
)

// implementationSources contains the exact trusted source compiled into both
// the worker and the capability inspector. Tests and this provenance helper
// are intentionally excluded: only code that participates in checker
// resolution, policy enforcement, input handling, and comparison is bound.
//
//go:embed checker.go exact.go token.go float.go
var implementationSources embed.FS

var implementationSourceFiles = map[Identity][]string{
	{Key: KeyExact, Version: VersionV1}: {"checker.go", "exact.go"},
	{Key: KeyToken, Version: VersionV1}: {"checker.go", "token.go"},
	{Key: KeyFloat, Version: VersionV1}: {"checker.go", "float.go"},
}

// ImplementationChecksum content-addresses the exact trusted Go source for
// one closed checker identity. The fixed file order and length-delimited
// encoding make the digest deterministic and immune to concatenation
// ambiguity.
func ImplementationChecksum(identity Identity) (string, bool) {
	files, supported := implementationSourceFiles[identity]
	if !supported || len(files) == 0 {
		return "", false
	}
	digest := sha256.New()
	writeImplementationPart(digest, []byte("judgify.checker.source.v1"))
	writeImplementationPart(digest, []byte(identity.Key))
	writeImplementationPart(digest, []byte(identity.Version))
	for _, name := range files {
		source, err := implementationSources.ReadFile(name)
		if err != nil || len(source) == 0 {
			return "", false
		}
		writeImplementationPart(digest, []byte(name))
		writeImplementationPart(digest, source)
	}
	return fmt.Sprintf("sha256:%x", digest.Sum(nil)), true
}

func writeImplementationPart(digest hash.Hash, value []byte) {
	var size [8]byte
	binary.BigEndian.PutUint64(size[:], uint64(len(value)))
	_, _ = digest.Write(size[:])
	_, _ = digest.Write(value)
}
