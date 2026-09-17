package http

import (
	"encoding/json"

	"github.com/google/uuid"
)

type registerProfileValueRequest struct {
	DefinitionID         uuid.UUID       `json:"definition_id" validate:"required"`
	DefinitionRevisionID uuid.UUID       `json:"definition_revision_id" validate:"required"`
	Value                json.RawMessage `json:"value" validate:"required"`
}
type registerRequest struct {
	Username              string                        `json:"username" validate:"required,min=3,max=50,alphanum"`
	Password              string                        `json:"password" validate:"required,min=8,max=256"`
	RecoveryEmail         string                        `json:"recovery_email" validate:"required,email,max=320"`
	OfferID               uuid.UUID                     `json:"offer_id" validate:"required"`
	OfferSequence         int64                         `json:"offer_sequence" validate:"required,min=1"`
	CatalogRevisionID     uuid.UUID                     `json:"catalog_revision_id" validate:"required"`
	CatalogChecksum       string                        `json:"catalog_checksum" validate:"required,len=64,hexadecimal"`
	OfferChecksum         string                        `json:"offer_checksum" validate:"required,len=64,hexadecimal"`
	RootBoneID            uuid.UUID                     `json:"root_bone_id" validate:"required"`
	TalentIDs             []uuid.UUID                   `json:"talent_ids" validate:"required,len=3"`
	ProfileSchemaRevision int64                         `json:"profile_schema_revision" validate:"required,gt=0"`
	ProfileSchemaChecksum string                        `json:"profile_schema_checksum" validate:"required,len=64,hexadecimal"`
	ProfileValues         []registerProfileValueRequest `json:"profile_values" validate:"required,min=1,max=128,dive"`
	PreSessionID          uuid.UUID
	IdempotencyKey        uuid.UUID
	UserAgent             string
	IPHash                []byte
}
