package db

import (
	"bytes"
	"context"
	stdsql "database/sql"
	"errors"

	"github.com/google/uuid"

	commonEnt "github.com/huynhanx03/go-common/pkg/database/ent"

	dbEnt "github.com/huynhanx03/judgify/internal/ent"

	"github.com/huynhanx03/judgify/internal/ent/generate"
	"github.com/huynhanx03/judgify/internal/ent/generate/attributedefinition"
	"github.com/huynhanx03/judgify/internal/ent/generate/attributedefinitionrevision"
	"github.com/huynhanx03/judgify/internal/ent/generate/userattributevalue"
	"github.com/huynhanx03/judgify/internal/identity/attributemigration"
	"github.com/huynhanx03/judgify/internal/identity/attributevalue"
	entity "github.com/huynhanx03/judgify/internal/identity/domain"
	platformtx "github.com/huynhanx03/judgify/internal/platform/transaction"
	"github.com/huynhanx03/judgify/internal/ports"
)

const (
	userAttrValueRepoName      = "User Attribute Value"
	userAttributeValueLockSeed = 7415201
	maximumProfileAttributes   = 256
)

type UserAttributeValueRepository struct {
	client *dbEnt.EntClient
}

var _ ports.RevisionedUserAttributeValueWriter = (*UserAttributeValueRepository)(nil)
var _ ports.ActiveProfileAttributeValueReader = (*UserAttributeValueRepository)(nil)

func NewUserAttributeValueRepository(client *dbEnt.EntClient) ports.UserAttributeValueRepository {
	return &UserAttributeValueRepository{client: client}
}

func (r *UserAttributeValueRepository) ListActiveProfileAttributes(
	ctx context.Context,
	userID uuid.UUID,
) ([]ports.ActiveProfileAttributeValue, error) {
	if r == nil || r.client == nil || userID == uuid.Nil {
		return nil, ports.ErrInvalidUserAttributeValue
	}
	rows, err := r.client.DB(ctx).UserAttributeValue.QueryContext(ctx, `
		SELECT definition.id, revision.id,
		       definition.key, definition.kind, definition.required_on_onboarding,
		       revision.data_type, revision.label, revision.description,
		       revision.visibility, revision.user_editable, revision.display_order,
		       revision.validation,
		       value.id, value.source_revision_id, value.canonical_value,
		       value.source_value_version, value.version
		FROM attribute_definitions AS definition
		JOIN attribute_definition_revisions AS revision
		  ON revision.id = definition.active_revision_id
		 AND revision.definition_id = definition.id
		LEFT JOIN user_attribute_values AS value
		  ON value.user_id = $1
		 AND value.attribute_id = definition.id
		 AND value.definition_revision_id = definition.active_revision_id
		 AND value.deleted_at IS NULL
		WHERE definition.deleted_at IS NULL
		  AND definition.status = 'active'
		  AND revision.visibility <> 'admin'
		ORDER BY revision.display_order, definition.key, definition.id
		LIMIT 257`, userID)
	if err != nil {
		return nil, commonEnt.MapEntError(err, userAttrValueRepoName)
	}
	defer func() { _ = rows.Close() }()
	result := make([]ports.ActiveProfileAttributeValue, 0)
	for rows.Next() {
		var valueID uuid.NullUUID
		var sourceRevisionID uuid.NullUUID
		var canonical []byte
		var description stdsql.NullString
		var sourceValueVersion stdsql.NullInt64
		var valueVersion stdsql.NullInt64
		attribute := ports.ActiveProfileAttributeValue{}
		if err := rows.Scan(
			&attribute.DefinitionID,
			&attribute.DefinitionRevisionID,
			&attribute.Key,
			&attribute.Kind,
			&attribute.RequiredOnOnboarding,
			&attribute.DataType,
			&attribute.Label,
			&description,
			&attribute.Visibility,
			&attribute.UserEditable,
			&attribute.DisplayOrder,
			&attribute.Validation,
			&valueID,
			&sourceRevisionID,
			&canonical,
			&sourceValueVersion,
			&valueVersion,
		); err != nil {
			return nil, commonEnt.MapEntError(err, userAttrValueRepoName)
		}
		attribute.HasValue = valueID.Valid
		if description.Valid {
			attribute.Description = description.String
		}
		if attribute.HasValue {
			if valueID.UUID == uuid.Nil || valueID.UUID.Version() != 7 ||
				!sourceRevisionID.Valid || sourceRevisionID.UUID == uuid.Nil ||
				sourceRevisionID.UUID.Version() != 7 ||
				!sourceValueVersion.Valid || !valueVersion.Valid ||
				sourceValueVersion.Int64 < 1 ||
				valueVersion.Int64 < 1 ||
				(sourceRevisionID.UUID == attribute.DefinitionRevisionID &&
					sourceValueVersion.Int64 != valueVersion.Int64) ||
				len(canonical) == 0 {
				return nil, ports.ErrInvalidUserAttributeValue
			}
			attribute.CanonicalValue = append([]byte(nil), canonical...)
			attribute.ValueVersion = valueVersion.Int64
		} else if sourceRevisionID.Valid || sourceValueVersion.Valid ||
			valueVersion.Valid || len(canonical) != 0 {
			return nil, ports.ErrInvalidUserAttributeValue
		}
		if attribute.DefinitionID == uuid.Nil ||
			attribute.DefinitionRevisionID == uuid.Nil ||
			attribute.Key == "" ||
			!attribute.DataType.Valid() ||
			!attribute.Visibility.Valid() ||
			attribute.Visibility == entity.AttributeVisibilityAdmin ||
			len(attribute.Validation) == 0 {
			return nil, ports.ErrInvalidUserAttributeValue
		}
		result = append(result, attribute)
		if len(result) > maximumProfileAttributes {
			return nil, ports.ErrInvalidUserAttributeValue
		}
	}
	if err := rows.Err(); err != nil {
		return nil, commonEnt.MapEntError(err, userAttrValueRepoName)
	}
	return result, nil
}

func (r *UserAttributeValueRepository) ApplyActiveMutation(
	ctx context.Context,
	mutation ports.RevisionedUserAttributeValueMutation,
) (*entity.UserAttributeValue, error) {
	return r.applyActiveMutation(ctx, mutation)
}

func (r *UserAttributeValueRepository) applyActiveMutation(
	ctx context.Context,
	mutation ports.RevisionedUserAttributeValueMutation,
) (*entity.UserAttributeValue, error) {
	if r == nil || r.client == nil || ctx == nil || ctx.Err() != nil ||
		mutation.UserID == uuid.Nil || mutation.DefinitionID == uuid.Nil ||
		mutation.ExpectedDefinitionRevisionID == uuid.Nil ||
		mutation.ExpectedValueVersion < 0 ||
		mutation.Unset == (len(mutation.CanonicalValue) > 0) {
		return nil, ports.ErrInvalidUserAttributeValue
	}
	migrations := NewAttributeValueMigrationRepository(r.client)
	client, err := migrationsClient(ctx, r.client)
	if err != nil {
		return nil, ports.ErrUserAttributeValueWriterUnavailable
	}
	definitionRecord, err := client.AttributeDefinition.Query().Where(
		attributedefinition.ID(mutation.DefinitionID),
		attributedefinition.StatusEQ(attributedefinition.StatusActive),
	).ForShare().Only(ctx)
	if err != nil || definitionRecord == nil || definitionRecord.ActiveRevisionID == nil {
		if generate.IsNotFound(err) || definitionRecord == nil || definitionRecord.ActiveRevisionID == nil {
			return nil, ports.ErrUserAttributeDefinitionRevisionConflict
		}
		return nil, commonEnt.MapEntError(err, userAttrValueRepoName)
	}
	activeRevisionID := *definitionRecord.ActiveRevisionID
	if mutation.ExpectedDefinitionRevisionID != activeRevisionID {
		return nil, ports.ErrUserAttributeDefinitionRevisionConflict
	}
	revisionRecord, err := client.AttributeDefinitionRevision.Query().Where(
		attributedefinitionrevision.ID(activeRevisionID),
		attributedefinitionrevision.DefinitionID(mutation.DefinitionID),
	).Only(ctx)
	if err != nil || revisionRecord == nil {
		if generate.IsNotFound(err) || revisionRecord == nil {
			return nil, ports.ErrUserAttributeDefinitionRevisionConflict
		}
		return nil, commonEnt.MapEntError(err, userAttrValueRepoName)
	}
	activeRevision := attributeMigrationRevision(revisionRecord)
	definition := &entity.AttributeDefinition{
		ID: definitionRecord.ID, Status: definitionRecord.Status.String(),
		Version: definitionRecord.Version, ActiveRevisionID: &activeRevisionID,
		ActiveRevision: &activeRevision,
	}
	if _, err := client.UserAttributeValue.ExecContext(
		ctx,
		`SELECT pg_advisory_xact_lock(hashtextextended($1, $2))`,
		mutation.UserID.String()+":"+mutation.DefinitionID.String(),
		userAttributeValueLockSeed,
	); err != nil {
		return nil, commonEnt.MapEntError(err, userAttrValueRepoName)
	}

	source, err := client.UserAttributeValue.Query().Where(
		userattributevalue.UserID(mutation.UserID),
		userattributevalue.AttributeID(mutation.DefinitionID),
		userattributevalue.DefinitionRevisionID(activeRevisionID),
	).ForUpdate().Only(ctx)
	if err != nil && !generate.IsNotFound(err) {
		return nil, commonEnt.MapEntError(err, userAttrValueRepoName)
	}
	sourceMissing := generate.IsNotFound(err)
	if !sourceMissing {
		if err := validatePersistedActiveUserAttributeValue(
			source,
			mutation.UserID,
			mutation.DefinitionID,
			activeRevision,
		); err != nil {
			return nil, err
		}
	}

	if mutation.Unset {
		target, targetErr := migrations.ActiveMigrationTarget(ctx, definition)
		if targetErr != nil {
			return nil, mapRevisionedUserAttributeWriteError(targetErr)
		}
		if target != nil {
			return nil, ports.ErrUserAttributeMigrationActive
		}
		if sourceMissing {
			if mutation.ExpectedValueVersion != 0 {
				return nil, ports.ErrUserAttributeValueConflict
			}
			return nil, nil
		}
		if source == nil || mutation.ExpectedValueVersion < 1 ||
			mutation.ExpectedValueVersion != source.Version {
			return nil, ports.ErrUserAttributeValueConflict
		}
		// Retained historical revisions are immutable evidence, not fallback
		// values. Any later activation targeting one of those rows is forced
		// through bounded migration because target_count is non-zero; target
		// cleanup then removes the row when this active source is absent before
		// the pointer can move. Deleting every historical revision here would be
		// an unbounded user-request write and is intentionally avoided.
		deleteErr := client.UserAttributeValue.DeleteOneID(source.ID).Where(
			userattributevalue.UserID(mutation.UserID),
			userattributevalue.AttributeID(mutation.DefinitionID),
			userattributevalue.DefinitionRevisionID(activeRevisionID),
			userattributevalue.SourceRevisionID(source.SourceRevisionID),
			userattributevalue.Version(source.Version),
		).Exec(ctx)
		if generate.IsNotFound(deleteErr) {
			return nil, ports.ErrUserAttributeValueConflict
		}
		if deleteErr != nil {
			return nil, commonEnt.MapEntError(deleteErr, userAttrValueRepoName)
		}
		return nil, nil
	}

	canonical, err := attributevalue.Canonicalize(mutation.CanonicalValue, activeRevision)
	if err == nil && !bytes.Equal(canonical, mutation.CanonicalValue) {
		err = attributevalue.ErrInvalidValue
	}
	if err != nil {
		return nil, err
	}
	legacyValue, err := attributevalue.LegacyProjection(canonical, activeRevision.DataType)
	if err != nil {
		return nil, err
	}

	var persisted *generate.UserAttributeValue
	if sourceMissing {
		if mutation.ExpectedValueVersion != 0 {
			return nil, ports.ErrUserAttributeValueConflict
		}
		id, idErr := uuid.NewV7()
		if idErr != nil {
			return nil, ports.ErrUserAttributeValueWriterUnavailable
		}
		persisted, err = client.UserAttributeValue.Create().
			SetID(id).
			SetUserID(mutation.UserID).
			SetAttributeID(mutation.DefinitionID).
			SetDefinitionRevisionID(activeRevisionID).
			SetSourceRevisionID(activeRevisionID).
			SetCanonicalValue(canonical).
			SetValue(legacyValue).
			SetSourceValueVersion(1).
			SetVersion(1).
			Save(ctx)
	} else {
		if source == nil || mutation.ExpectedValueVersion < 1 ||
			mutation.ExpectedValueVersion != source.Version {
			return nil, ports.ErrUserAttributeValueConflict
		}
		if source.Value == legacyValue && bytes.Equal(source.CanonicalValue, canonical) {
			persisted = source
		} else {
			next := source.Version + 1
			persisted, err = client.UserAttributeValue.UpdateOneID(source.ID).
				Where(userattributevalue.Version(source.Version)).
				SetSourceRevisionID(activeRevisionID).
				SetCanonicalValue(canonical).
				SetValue(legacyValue).
				SetSourceValueVersion(next).
				SetVersion(next).
				Save(ctx)
			if generate.IsNotFound(err) {
				return nil, ports.ErrUserAttributeValueConflict
			}
		}
	}
	if err != nil {
		return nil, commonEnt.MapEntError(err, userAttrValueRepoName)
	}
	target, err := migrations.ActiveMigrationTarget(ctx, definition)
	if err != nil {
		return nil, mapRevisionedUserAttributeWriteError(err)
	}
	if target != nil {
		candidate, err := client.AttributeDefinitionRevision.Query().
			Where(attributedefinitionrevision.ID(target.CandidateRevisionID)).
			Only(ctx)
		if err != nil || candidate == nil || candidate.DefinitionID != mutation.DefinitionID {
			return nil, ports.ErrUserAttributeMigrationConflict
		}
		candidateRevision := attributeMigrationRevision(candidate)
		candidateCanonical, err := attributevalue.CanonicalizeLegacy(
			legacyValue,
			candidateRevision,
		)
		if err != nil {
			// Do not let the source and candidate diverge. The surrounding
			// transaction rolls back this source mutation as well.
			return nil, ports.ErrUserAttributeMigrationConflict
		}
		candidateLegacyValue, err := attributevalue.LegacyProjection(
			candidateCanonical,
			candidateRevision.DataType,
		)
		if err != nil {
			return nil, ports.ErrUserAttributeMigrationConflict
		}
		if err := migrations.PutShadow(ctx, attributemigration.ShadowValue{
			UserID: mutation.UserID, DefinitionID: mutation.DefinitionID,
			SourceRevisionID:    target.SourceRevisionID,
			CandidateRevisionID: target.CandidateRevisionID,
			CanonicalValue:      candidateCanonical, LegacyValue: candidateLegacyValue,
			SourceValueVersion: persisted.Version,
		}); err != nil {
			if errors.Is(err, attributemigration.ErrValueDrift) {
				return nil, ports.ErrUserAttributeMigrationConflict
			}
			return nil, mapRevisionedUserAttributeWriteError(err)
		}
	}
	return ToUserAttributeValueEntity(persisted), nil
}

func migrationsClient(ctx context.Context, client *dbEnt.EntClient) (*generate.Client, error) {
	if client == nil || client.Client() == nil {
		return nil, ports.ErrUserAttributeValueWriterUnavailable
	}
	transactional, err := platformtx.Client(ctx, nil)
	if err != nil {
		return nil, err
	}
	return transactional, nil
}

func validatePersistedActiveUserAttributeValue(
	record *generate.UserAttributeValue,
	userID uuid.UUID,
	definitionID uuid.UUID,
	revision entity.AttributeDefinitionRevision,
) error {
	// An active row can still carry the immutable provenance of the revision
	// from which a lifecycle migration derived it. Only a self-originated row
	// couples source_value_version to its own optimistic version; the first
	// ordinary edit resets both provenance fields to the active revision below.
	if record == nil || record.ID == uuid.Nil || record.ID.Version() != 7 ||
		record.UserID != userID || record.AttributeID != definitionID ||
		record.DefinitionRevisionID != revision.ID ||
		record.SourceRevisionID == uuid.Nil || record.SourceRevisionID.Version() != 7 ||
		record.Version < 1 || record.SourceValueVersion < 1 ||
		(record.SourceRevisionID == revision.ID &&
			record.SourceValueVersion != record.Version) ||
		len(record.CanonicalValue) == 0 {
		return ports.ErrInvalidUserAttributeValue
	}
	canonical, err := attributevalue.Canonicalize(record.CanonicalValue, revision)
	if err != nil || !bytes.Equal(canonical, record.CanonicalValue) {
		return ports.ErrInvalidUserAttributeValue
	}
	legacy, err := attributevalue.LegacyProjection(canonical, revision.DataType)
	if err != nil || legacy != record.Value {
		return ports.ErrInvalidUserAttributeValue
	}
	return nil
}

func mapRevisionedUserAttributeWriteError(err error) error {
	if errors.Is(err, attributemigration.ErrValueDrift) {
		return ports.ErrUserAttributeValueConflict
	}
	if errors.Is(err, attributemigration.ErrDefinitionDrift) {
		return ports.ErrUserAttributeDefinitionRevisionConflict
	}
	return err
}
