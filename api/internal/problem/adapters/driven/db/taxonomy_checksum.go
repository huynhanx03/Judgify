package db

import (
	"bytes"
	"sort"

	"github.com/google/uuid"

	problemcontract "github.com/huynhanx03/judgify/internal/problem/contract"
)

const (
	difficultyRevisionDomain = "judgify.difficulty-revision.v1"
	tagRevisionDomain        = "judgify.tag-revision.v1"
)

type elementRevisionFact struct {
	ID       uuid.UUID
	Revision int64
}

func difficultyRevisionChecksum(
	id uuid.UUID,
	revision int64,
	name string,
	level int,
	experience int64,
	description string,
) [32]byte {
	encoder := problemcontract.NewChecksumEncoder(difficultyRevisionDomain)
	encoder.UUID(id)
	encoder.Int64(revision)
	encoder.String(name)
	encoder.Int64(int64(level))
	encoder.Int64(experience)
	encoder.String(description)
	return encoder.Sum()
}

func tagRevisionChecksum(id uuid.UUID, revision int64, name string, elements []elementRevisionFact) [32]byte {
	canonical := append([]elementRevisionFact(nil), elements...)
	sort.Slice(canonical, func(i, j int) bool {
		return bytes.Compare(canonical[i].ID[:], canonical[j].ID[:]) < 0
	})
	encoder := problemcontract.NewChecksumEncoder(tagRevisionDomain)
	encoder.UUID(id)
	encoder.Int64(revision)
	encoder.String(name)
	encoder.Int64(int64(len(canonical)))
	for _, element := range canonical {
		encoder.UUID(element.ID)
		encoder.Int64(element.Revision)
	}
	return encoder.Sum()
}
