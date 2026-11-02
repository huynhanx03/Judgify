package contract

import (
	"crypto/sha256"
	"encoding/binary"
	"hash"

	"github.com/google/uuid"
)

// ChecksumEncoder writes the canonical binary checksum preimage shared by
// immutable taxonomy revisions. Every primitive is length-prefixed to make
// concatenated tuples unambiguous.
type ChecksumEncoder struct{ hash hash.Hash }

// NewChecksumEncoder starts an independent SHA-256 preimage with a domain
// separator. Callers must use a versioned domain separator.
func NewChecksumEncoder(domain string) *ChecksumEncoder {
	e := &ChecksumEncoder{hash: sha256.New()}
	e.Bytes([]byte(domain))
	return e
}

// Bytes appends an unsigned-big-endian byte length followed by exact bytes.
func (e *ChecksumEncoder) Bytes(value []byte) {
	var length [8]byte
	binary.BigEndian.PutUint64(length[:], uint64(len(value)))
	_, _ = e.hash.Write(length[:])
	_, _ = e.hash.Write(value)
}

// String appends the supplied UTF-8 bytes exactly as supplied.
func (e *ChecksumEncoder) String(value string) { e.Bytes([]byte(value)) }

// UUID appends raw UUID bytes and never its formatted textual representation.
func (e *ChecksumEncoder) UUID(value uuid.UUID) { e.Bytes(value[:]) }

// Int64 appends a fixed-width, big-endian two's-complement integer.
func (e *ChecksumEncoder) Int64(value int64) {
	var encoded [8]byte
	binary.BigEndian.PutUint64(encoded[:], uint64(value))
	e.Bytes(encoded[:])
}

// Sum returns the 32-byte SHA-256 digest.
func (e *ChecksumEncoder) Sum() [sha256.Size]byte {
	var result [sha256.Size]byte
	copy(result[:], e.hash.Sum(nil))
	return result
}
