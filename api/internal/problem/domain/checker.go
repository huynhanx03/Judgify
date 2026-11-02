package domain

import "github.com/google/uuid"

const (
	CheckerKeyExact = "exact"
	CheckerKeyToken = "token"
	CheckerKeyFloat = "float"
)

type CheckerConfigKind string

const (
	CheckerConfigNone           CheckerConfigKind = "none"
	CheckerConfigFloatTolerance CheckerConfigKind = "float_tolerance"
)

// CheckerProfile identifies one versioned, trusted checker contract.
type CheckerProfile struct {
	ID                     uuid.UUID
	Key                    string
	Version                string
	ImplementationChecksum string
	ConfigKind             CheckerConfigKind
	Active                 bool
}
