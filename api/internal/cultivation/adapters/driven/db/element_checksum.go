package db

import (
	"github.com/google/uuid"

	problemcontract "github.com/huynhanx03/judgify/internal/problem/contract"
)

const elementRevisionDomain = "judgify.element-revision.v1"

func elementRevisionChecksum(id uuid.UUID, revision int64, name, code, description string) [32]byte {
	encoder := problemcontract.NewChecksumEncoder(elementRevisionDomain)
	encoder.UUID(id)
	encoder.Int64(revision)
	encoder.String(name)
	encoder.String(code)
	encoder.String(description)
	return encoder.Sum()
}
