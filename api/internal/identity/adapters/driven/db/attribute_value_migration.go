package db

import (
	"bytes"
	"context"
	stdsql "database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/uuid"

	entsql "entgo.io/ent/dialect/sql"

	dbEnt "github.com/huynhanx03/judgify/internal/ent"
	"github.com/huynhanx03/judgify/internal/ent/generate"
	"github.com/huynhanx03/judgify/internal/ent/generate/attributedefinition"
	"github.com/huynhanx03/judgify/internal/ent/generate/attributedefinitionrevision"
	"github.com/huynhanx03/judgify/internal/ent/generate/profileschemastate"
	"github.com/huynhanx03/judgify/internal/ent/generate/userattributevalue"
	"github.com/huynhanx03/judgify/internal/identity/attributemigration"
	"github.com/huynhanx03/judgify/internal/identity/attributevalue"
	entity "github.com/huynhanx03/judgify/internal/identity/domain"
	platformtx "github.com/huynhanx03/judgify/internal/platform/transaction"
	"github.com/huynhanx03/judgify/internal/ports"
)

// AttributeValueMigrationRepository persists only the revisioned-value side
// of an activation operation. The lifecycle workflow remains the sole owner
// of the audited definition pointer transition.
type AttributeValueMigrationRepository struct {
	client *dbEnt.EntClient
}

func NewAttributeValueMigrationRepository(client *dbEnt.EntClient) *AttributeValueMigrationRepository {
	return &AttributeValueMigrationRepository{client: client}
}

var _ attributemigration.Repository = (*AttributeValueMigrationRepository)(nil)
var _ attributemigration.CoordinatorRepository = (*AttributeValueMigrationRepository)(nil)

// Prepare captures a UUIDv7 high-water mark while holding the definition
// write lock. Profile writes must obtain the corresponding shared fence before
// mutating values; that is what makes the later final verification a short
// definition lock rather than an unbounded row-lock sweep.
func (repository *AttributeValueMigrationRepository) Prepare(
	ctx context.Context,
	request attributemigration.Request,
) (attributemigration.Request, error) {
	client, err := repository.transactionalClient(ctx)
	if err != nil || !validMigrationIntent(request) {
		return attributemigration.Request{}, attributemigration.ErrInvalidMigration
	}
	definition, err := client.AttributeDefinition.Query().
		Where(
			attributedefinition.ID(request.DefinitionID),
			attributedefinition.Version(request.ExpectedVersion),
			attributedefinition.StatusEQ(attributedefinition.StatusActive),
			attributedefinition.ActiveRevisionID(request.SourceRevisionID),
		).
		ForUpdate().
		Only(ctx)
	if err != nil {
		return attributemigration.Request{}, migrationDefinitionError(err)
	}
	if definition == nil {
		return attributemigration.Request{}, attributemigration.ErrDefinitionDrift
	}
	candidate, err := client.AttributeDefinitionRevision.Query().
		Where(attributedefinitionrevision.ID(request.CandidateRevisionID)).
		Only(ctx)
	if err != nil || candidate == nil || candidate.DefinitionID != request.DefinitionID {
		return attributemigration.Request{}, attributemigration.ErrDefinitionDrift
	}
	active, err := activeAttributeMigrationRequests(
		ctx,
		client,
		request.DefinitionID,
		request.SourceRevisionID,
	)
	if err != nil {
		return attributemigration.Request{}, err
	}
	if len(active) > 0 {
		if len(active) != 1 || !sameAttributeMigrationIntent(active[0], request) {
			return attributemigration.Request{}, attributemigration.ErrMigrationActive
		}
		return active[0], nil
	}
	target := client.UserAttributeValue.Query().Where(
		userattributevalue.AttributeID(request.DefinitionID),
		userattributevalue.DefinitionRevisionID(request.CandidateRevisionID),
	)
	request.TargetCount, request.TargetCutoff, err = freezeAttributeValueSet(ctx, target)
	if err != nil {
		return attributemigration.Request{}, err
	}
	source := client.UserAttributeValue.Query().Where(
		userattributevalue.AttributeID(request.DefinitionID),
		userattributevalue.DefinitionRevisionID(request.SourceRevisionID),
	)
	request.SourceCount, request.SourceCutoff, err = freezeAttributeValueSet(ctx, source)
	if err != nil {
		return attributemigration.Request{}, err
	}
	if request.TargetCount == 0 && request.SourceCount == 0 {
		return attributemigration.Request{}, attributemigration.ErrInvalidMigration
	}
	return request, nil
}

func freezeAttributeValueSet(
	ctx context.Context,
	query *generate.UserAttributeValueQuery,
) (int64, uuid.UUID, error) {
	if query == nil {
		return 0, uuid.Nil, attributemigration.ErrInvalidMigration
	}
	total, err := query.Clone().Count(ctx)
	if err != nil {
		return 0, uuid.Nil, err
	}
	if total == 0 {
		return 0, uuid.Nil, nil
	}
	cutoff, err := query.Clone().
		Order(userattributevalue.ByID(entsql.OrderDesc())).
		First(ctx)
	if err != nil || cutoff == nil || cutoff.ID == uuid.Nil || cutoff.ID.Version() != 7 {
		return 0, uuid.Nil, attributemigration.ErrValueDrift
	}
	return int64(total), cutoff.ID, nil
}

// CleanupTargetBatch removes only stale generations from a retained target.
// Each returned ID is processed under row locks in source-then-target order,
// matching the live profile writer and avoiding lock inversion. A valid
// concurrent dual-write is retained; every other target row is soft-deleted.
func (repository *AttributeValueMigrationRepository) CleanupTargetBatch(
	ctx context.Context,
	request attributemigration.Request,
	checkpoint attributemigration.Checkpoint,
	limit int,
) (attributemigration.CleanupBatch, error) {
	client, err := repository.transactionalClient(ctx)
	if err != nil || !validMigrationRequest(request) ||
		checkpoint.Phase != attributemigration.PhaseCleanupTarget ||
		limit < 1 || request.TargetCount < 1 {
		return attributemigration.CleanupBatch{}, attributemigration.ErrInvalidMigration
	}
	query := client.UserAttributeValue.Query().Where(
		userattributevalue.AttributeID(request.DefinitionID),
		userattributevalue.DefinitionRevisionID(request.CandidateRevisionID),
		userattributevalue.IDLTE(request.TargetCutoff),
	)
	if checkpoint.AfterTargetValueID != uuid.Nil {
		query.Where(userattributevalue.IDGT(checkpoint.AfterTargetValueID))
	}
	records, err := query.Order(userattributevalue.ByID()).Limit(limit + 1).All(ctx)
	if err != nil {
		return attributemigration.CleanupBatch{}, err
	}
	done := len(records) <= limit
	if len(records) > limit {
		records = records[:limit]
	}
	candidate, err := client.AttributeDefinitionRevision.Query().
		Where(attributedefinitionrevision.ID(request.CandidateRevisionID)).
		Only(ctx)
	if err != nil || candidate == nil || candidate.DefinitionID != request.DefinitionID {
		return attributemigration.CleanupBatch{}, attributemigration.ErrDefinitionDrift
	}
	candidateRevision := attributeMigrationRevision(candidate)
	sourceRevisionRecord, err := client.AttributeDefinitionRevision.Query().
		Where(
			attributedefinitionrevision.ID(request.SourceRevisionID),
			attributedefinitionrevision.DefinitionID(request.DefinitionID),
		).
		Only(ctx)
	if err != nil || sourceRevisionRecord == nil {
		return attributemigration.CleanupBatch{}, attributemigration.ErrDefinitionDrift
	}
	sourceRevision := attributeMigrationRevision(sourceRevisionRecord)
	ids := make([]uuid.UUID, 0, len(records))
	for _, snapshot := range records {
		if snapshot == nil || snapshot.ID == uuid.Nil {
			return attributemigration.CleanupBatch{}, attributemigration.ErrValueDrift
		}
		source, sourceErr := client.UserAttributeValue.Query().Where(
			userattributevalue.UserID(snapshot.UserID),
			userattributevalue.AttributeID(request.DefinitionID),
			userattributevalue.DefinitionRevisionID(request.SourceRevisionID),
		).ForUpdate().Only(ctx)
		if sourceErr != nil && !generate.IsNotFound(sourceErr) {
			return attributemigration.CleanupBatch{}, sourceErr
		}
		target, targetErr := client.UserAttributeValue.Query().Where(
			userattributevalue.ID(snapshot.ID),
			userattributevalue.AttributeID(request.DefinitionID),
			userattributevalue.DefinitionRevisionID(request.CandidateRevisionID),
		).ForUpdate().Only(ctx)
		if targetErr != nil || target == nil {
			return attributemigration.CleanupBatch{}, attributemigration.ErrValueDrift
		}
		keep := false
		if sourceErr == nil && source != nil &&
			target.SourceRevisionID == request.SourceRevisionID &&
			target.SourceValueVersion == source.Version &&
			len(target.CanonicalValue) > 0 {
			expected, canonicalErr := canonicalAttributeMigrationSource(
				source,
				sourceRevision,
				candidateRevision,
			)
			expectedLegacy := ""
			if canonicalErr == nil {
				expectedLegacy, canonicalErr = attributevalue.LegacyProjection(
					expected,
					candidateRevision.DataType,
				)
			}
			keep = canonicalErr == nil &&
				target.Value == expectedLegacy &&
				bytes.Equal(expected, target.CanonicalValue)
		}
		if !keep {
			if deleteErr := client.UserAttributeValue.DeleteOneID(target.ID).Exec(ctx); deleteErr != nil {
				return attributemigration.CleanupBatch{}, deleteErr
			}
		}
		ids = append(ids, target.ID)
	}
	return attributemigration.CleanupBatch{ValueIDs: ids, Done: done}, nil
}

func canonicalAttributeMigrationSource(
	source *generate.UserAttributeValue,
	sourceRevision entity.AttributeDefinitionRevision,
	candidate entity.AttributeDefinitionRevision,
) (json.RawMessage, error) {
	if err := validateAttributeMigrationSourceRecord(source, sourceRevision); err != nil {
		return nil, err
	}
	canonical, err := attributevalue.Canonicalize(source.CanonicalValue, sourceRevision)
	if err != nil || !bytes.Equal(canonical, source.CanonicalValue) {
		return nil, attributemigration.ErrValueDrift
	}
	legacy, err := attributevalue.LegacyProjection(canonical, sourceRevision.DataType)
	if err != nil || legacy != source.Value {
		return nil, attributemigration.ErrValueDrift
	}
	return attributevalue.CanonicalizeLegacy(legacy, candidate)
}

func (repository *AttributeValueMigrationRepository) LoadBatch(
	ctx context.Context,
	request attributemigration.Request,
	checkpoint attributemigration.Checkpoint,
	limit int,
) (attributemigration.Batch, error) {
	client, err := repository.transactionalClient(ctx)
	if err != nil || !validMigrationRequest(request) || limit < 1 {
		return attributemigration.Batch{}, attributemigration.ErrInvalidMigration
	}
	candidate, err := client.AttributeDefinitionRevision.Query().
		Where(attributedefinitionrevision.ID(request.CandidateRevisionID)).
		Only(ctx)
	if err != nil || candidate == nil || candidate.DefinitionID != request.DefinitionID {
		return attributemigration.Batch{}, attributemigration.ErrDefinitionDrift
	}
	sourceRevision, err := client.AttributeDefinitionRevision.Query().
		Where(
			attributedefinitionrevision.ID(request.SourceRevisionID),
			attributedefinitionrevision.DefinitionID(request.DefinitionID),
		).
		Only(ctx)
	if err != nil || sourceRevision == nil {
		return attributemigration.Batch{}, attributemigration.ErrDefinitionDrift
	}
	query := client.UserAttributeValue.Query().Where(
		userattributevalue.AttributeID(request.DefinitionID),
		userattributevalue.DefinitionRevisionID(request.SourceRevisionID),
		userattributevalue.IDLTE(request.SourceCutoff),
	)
	if checkpoint.AfterSourceValueID != uuid.Nil {
		query.Where(userattributevalue.IDGT(checkpoint.AfterSourceValueID))
	}
	records, err := query.Order(userattributevalue.ByID()).Limit(limit).ForUpdate().All(ctx)
	if err != nil {
		return attributemigration.Batch{}, err
	}
	values := make([]attributemigration.SourceValue, 0, len(records))
	for _, record := range records {
		if err := validateAttributeMigrationSourceRecord(
			record,
			attributeMigrationRevision(sourceRevision),
		); err != nil {
			return attributemigration.Batch{}, err
		}
		values = append(values, attributemigration.SourceValue{
			ID: record.ID, UserID: record.UserID, DefinitionID: record.AttributeID,
			RevisionID:     record.DefinitionRevisionID,
			CanonicalValue: append([]byte(nil), record.CanonicalValue...),
			LegacyValue:    record.Value, Version: record.Version,
		})
	}
	return attributemigration.Batch{
		Source:    attributeMigrationRevision(sourceRevision),
		Candidate: attributeMigrationRevision(candidate),
		Values:    values,
		Done:      len(values) < limit,
	}, nil
}

// PutShadow is source-version monotonic. A concurrent dual write that has
// already produced a newer candidate wins; a same-version different value is
// a hard drift rather than an implicit last-writer-wins overwrite.
func (repository *AttributeValueMigrationRepository) PutShadow(
	ctx context.Context,
	shadow attributemigration.ShadowValue,
) error {
	client, err := repository.transactionalClient(ctx)
	if err != nil || shadow.UserID == uuid.Nil || shadow.DefinitionID == uuid.Nil ||
		shadow.SourceRevisionID == uuid.Nil ||
		shadow.CandidateRevisionID == uuid.Nil || shadow.SourceValueVersion < 1 ||
		shadow.SourceRevisionID == shadow.CandidateRevisionID ||
		len(shadow.CanonicalValue) == 0 {
		return attributemigration.ErrInvalidMigration
	}
	candidate, err := client.AttributeDefinitionRevision.Query().
		Where(attributedefinitionrevision.ID(shadow.CandidateRevisionID)).
		Only(ctx)
	if err != nil || candidate == nil || candidate.DefinitionID != shadow.DefinitionID {
		return attributemigration.ErrDefinitionDrift
	}
	candidateRevision := attributeMigrationRevision(candidate)
	canonical, err := attributevalue.Canonicalize(
		shadow.CanonicalValue,
		candidateRevision,
	)
	if err != nil || !bytes.Equal(canonical, shadow.CanonicalValue) {
		return attributemigration.ErrValueDrift
	}
	legacy, err := attributevalue.LegacyProjection(
		canonical,
		candidateRevision.DataType,
	)
	if err != nil || legacy != shadow.LegacyValue {
		return attributemigration.ErrValueDrift
	}
	existing, err := client.UserAttributeValue.Query().Where(
		userattributevalue.UserID(shadow.UserID),
		userattributevalue.AttributeID(shadow.DefinitionID),
		userattributevalue.DefinitionRevisionID(shadow.CandidateRevisionID),
	).ForUpdate().Only(ctx)
	if err == nil && existing != nil {
		if err := validateAttributeMigrationShadowRecord(
			existing,
			shadow,
			candidateRevision,
		); err != nil {
			return err
		}
		switch {
		case existing.SourceRevisionID == shadow.SourceRevisionID &&
			existing.SourceValueVersion > shadow.SourceValueVersion:
			return nil
		case existing.SourceRevisionID == shadow.SourceRevisionID &&
			existing.SourceValueVersion == shadow.SourceValueVersion &&
			existing.Value == shadow.LegacyValue &&
			bytes.Equal(existing.CanonicalValue, shadow.CanonicalValue):
			return nil
		case existing.SourceRevisionID == shadow.SourceRevisionID &&
			existing.SourceValueVersion == shadow.SourceValueVersion:
			return attributemigration.ErrValueDrift
		}
		_, err = client.UserAttributeValue.UpdateOneID(existing.ID).
			Where(userattributevalue.Version(existing.Version)).
			SetSourceRevisionID(shadow.SourceRevisionID).
			SetCanonicalValue(append([]byte(nil), shadow.CanonicalValue...)).
			SetValue(shadow.LegacyValue).
			SetSourceValueVersion(shadow.SourceValueVersion).
			SetVersion(existing.Version + 1).
			Save(ctx)
		if generate.IsNotFound(err) {
			return attributemigration.ErrValueDrift
		}
		return err
	}
	if !generate.IsNotFound(err) {
		return err
	}
	_, err = client.UserAttributeValue.Create().
		SetUserID(shadow.UserID).
		SetAttributeID(shadow.DefinitionID).
		SetDefinitionRevisionID(shadow.CandidateRevisionID).
		SetSourceRevisionID(shadow.SourceRevisionID).
		SetCanonicalValue(append([]byte(nil), shadow.CanonicalValue...)).
		SetValue(shadow.LegacyValue).
		SetSourceValueVersion(shadow.SourceValueVersion).
		Save(ctx)
	return err
}

func validateAttributeMigrationSourceRecord(
	record *generate.UserAttributeValue,
	revision entity.AttributeDefinitionRevision,
) error {
	// Chained activation is valid: a B row selected as the source for B -> C may
	// itself retain A -> B provenance. Self-originated rows alone require the
	// source version to equal their optimistic row version.
	if record == nil || record.ID == uuid.Nil || record.ID.Version() != 7 ||
		record.UserID == uuid.Nil || record.AttributeID != revision.DefinitionID ||
		record.DefinitionRevisionID != revision.ID ||
		record.SourceRevisionID == uuid.Nil || record.SourceRevisionID.Version() != 7 ||
		record.Version < 1 || record.SourceValueVersion < 1 ||
		(record.SourceRevisionID == revision.ID &&
			record.SourceValueVersion != record.Version) ||
		len(record.CanonicalValue) == 0 {
		return attributemigration.ErrValueDrift
	}
	canonical, err := attributevalue.Canonicalize(record.CanonicalValue, revision)
	if err != nil || !bytes.Equal(canonical, record.CanonicalValue) {
		return attributemigration.ErrValueDrift
	}
	legacy, err := attributevalue.LegacyProjection(canonical, revision.DataType)
	if err != nil || legacy != record.Value {
		return attributemigration.ErrValueDrift
	}
	return nil
}

func validateAttributeMigrationShadowRecord(
	record *generate.UserAttributeValue,
	shadow attributemigration.ShadowValue,
	candidate entity.AttributeDefinitionRevision,
) error {
	if record == nil || record.ID == uuid.Nil || record.ID.Version() != 7 ||
		record.UserID != shadow.UserID || record.AttributeID != shadow.DefinitionID ||
		record.DefinitionRevisionID != shadow.CandidateRevisionID ||
		record.SourceRevisionID == uuid.Nil || record.SourceRevisionID.Version() != 7 ||
		record.SourceValueVersion < 1 ||
		record.Version < 1 || len(record.CanonicalValue) == 0 {
		return attributemigration.ErrValueDrift
	}
	canonical, err := attributevalue.Canonicalize(record.CanonicalValue, candidate)
	if err != nil || !bytes.Equal(canonical, record.CanonicalValue) {
		return attributemigration.ErrValueDrift
	}
	legacy, err := attributevalue.LegacyProjection(canonical, candidate.DataType)
	if err != nil || legacy != record.Value {
		return attributemigration.ErrValueDrift
	}
	return nil
}

// VerifyFinal rechecks the complete source/candidate relation with a bounded
// aggregate query while the definition is row-locked. It sees writes after
// the UUIDv7 cutoff too, which must have been mirrored by the dual-write
// helper before this switch can succeed.
func (repository *AttributeValueMigrationRepository) VerifyFinal(
	ctx context.Context,
	request attributemigration.Request,
) error {
	client, err := repository.transactionalClient(ctx)
	if err != nil || !validMigrationRequest(request) {
		return attributemigration.ErrInvalidMigration
	}
	// Lock order is schema fence -> definition -> values everywhere. The
	// activator will reacquire this row in the same transaction before it
	// advances the public onboarding catalog revision.
	schemaState, err := client.ProfileSchemaState.Query().
		Where(profileschemastate.Key(ports.ProfileSchemaStateKey)).
		ForUpdate().
		Only(ctx)
	if err != nil || schemaState == nil || schemaState.Revision < 1 {
		return attributemigration.ErrDefinitionDrift
	}
	definition, err := client.AttributeDefinition.Query().Where(
		attributedefinition.ID(request.DefinitionID),
		attributedefinition.Version(request.ExpectedVersion),
		attributedefinition.StatusEQ(attributedefinition.StatusActive),
		attributedefinition.ActiveRevisionID(request.SourceRevisionID),
	).ForUpdate().Only(ctx)
	if err != nil || definition == nil {
		return migrationDefinitionError(err)
	}
	rows, err := client.UserAttributeValue.QueryContext(ctx, `
		SELECT COUNT(*)
		FROM user_attribute_values AS source
		WHERE source.attribute_id = $1
		  AND source.definition_revision_id = $2
		  AND source.deleted_at IS NULL
		  AND NOT EXISTS (
			SELECT 1
			FROM user_attribute_values AS candidate
			WHERE candidate.user_id = source.user_id
			  AND candidate.attribute_id = source.attribute_id
			  AND candidate.definition_revision_id = $3
			  AND candidate.deleted_at IS NULL
			  AND candidate.source_revision_id = $2
			  AND candidate.source_value_version = source.version
			  AND candidate.canonical_value IS NOT NULL
		  )`, request.DefinitionID, request.SourceRevisionID, request.CandidateRevisionID)
	if err != nil {
		return err
	}
	defer func() { _ = rows.Close() }()
	if !rows.Next() {
		if rowErr := rows.Err(); rowErr != nil {
			return rowErr
		}
		return attributemigration.ErrValueDrift
	}
	var drift int64
	if err := rows.Scan(&drift); err != nil || rows.Next() || rows.Err() != nil {
		return attributemigration.ErrValueDrift
	}
	if drift != 0 {
		return attributemigration.ErrValueDrift
	}
	return nil
}

// LockActiveDefinitionForValueWrite is the row-lock fence required by profile
// writes while a migration may be activating. Callers hold this shared lock,
// then update the active value and call PutShadow for the candidate selected
// by their active migration command before committing.
func (repository *AttributeValueMigrationRepository) LockActiveDefinitionForValueWrite(
	ctx context.Context,
	definitionID uuid.UUID,
) (*entity.AttributeDefinition, error) {
	client, err := repository.transactionalClient(ctx)
	if err != nil || definitionID == uuid.Nil {
		return nil, attributemigration.ErrInvalidMigration
	}
	definition, err := client.AttributeDefinition.Query().
		Where(
			attributedefinition.ID(definitionID),
			attributedefinition.StatusEQ(attributedefinition.StatusActive),
		).
		ForShare().
		Only(ctx)
	if err != nil {
		return nil, migrationDefinitionError(err)
	}
	if definition == nil || definition.ActiveRevisionID == nil {
		return nil, attributemigration.ErrDefinitionDrift
	}
	active, err := client.AttributeDefinitionRevision.Query().
		Where(attributedefinitionrevision.ID(*definition.ActiveRevisionID)).
		Only(ctx)
	if err != nil || active == nil || active.DefinitionID != definition.ID {
		return nil, attributemigration.ErrDefinitionDrift
	}
	activeRevision := attributeMigrationRevision(active)
	return &entity.AttributeDefinition{
		ID: definition.ID, Status: definition.Status.String(), Version: definition.Version,
		ActiveRevisionID: cloneAttributeMigrationUUID(definition.ActiveRevisionID),
		ActiveRevision:   &activeRevision,
	}, nil
}

// ActiveMigrationTarget returns the sole candidate currently responsible for
// dual writes from this active source revision. Operation state is intentionally
// read through the generic durable-operation table: the domain owns only its
// typed request, while pending/running/paused are the generic non-terminal
// states that may still resume and later activate.
func (repository *AttributeValueMigrationRepository) ActiveMigrationTarget(
	ctx context.Context,
	definition *entity.AttributeDefinition,
) (*attributemigration.Request, error) {
	client, err := repository.transactionalClient(ctx)
	if err != nil || definition == nil || definition.ID == uuid.Nil ||
		definition.ActiveRevisionID == nil || definition.ActiveRevision == nil {
		return nil, attributemigration.ErrInvalidMigration
	}
	active, err := activeAttributeMigrationRequests(
		ctx,
		client,
		definition.ID,
		*definition.ActiveRevisionID,
	)
	if err != nil {
		return nil, err
	}
	if len(active) == 0 {
		return nil, nil
	}
	if len(active) != 1 {
		return nil, attributemigration.ErrDefinitionDrift
	}
	target := active[0]
	return &target, nil
}

func activeAttributeMigrationRequests(
	ctx context.Context,
	client *generate.Client,
	definitionID uuid.UUID,
	sourceRevisionID uuid.UUID,
) ([]attributemigration.Request, error) {
	if client == nil || definitionID == uuid.Nil || sourceRevisionID == uuid.Nil {
		return nil, attributemigration.ErrInvalidMigration
	}
	rows, err := client.QueryContext(ctx, `
		SELECT request_payload
		FROM operations
		WHERE kind = $1
		  AND status IN ('pending', 'running', 'paused')
		  AND request_payload->>'definition_id' = $2
		  AND request_payload->>'source_revision_id' = $3
		ORDER BY created_at DESC, id DESC
		LIMIT 2`,
		attributemigration.OperationKind,
		definitionID.String(),
		sourceRevisionID.String(),
	)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	requests := make([]attributemigration.Request, 0, 1)
	for rows.Next() {
		var payload json.RawMessage
		if err := rows.Scan(&payload); err != nil {
			return nil, err
		}
		var request attributemigration.Request
		if json.Unmarshal(payload, &request) != nil ||
			attributemigration.ValidateRequest(request) != nil ||
			request.DefinitionID != definitionID ||
			request.SourceRevisionID != sourceRevisionID {
			return nil, attributemigration.ErrDefinitionDrift
		}
		requests = append(requests, request)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return requests, nil
}

func sameAttributeMigrationIntent(
	stored attributemigration.Request,
	requested attributemigration.Request,
) bool {
	return stored.OperationID == requested.OperationID &&
		stored.CommandID == requested.CommandID &&
		stored.DefinitionID == requested.DefinitionID &&
		stored.ExpectedVersion == requested.ExpectedVersion &&
		stored.SourceRevisionID == requested.SourceRevisionID &&
		stored.CandidateRevisionID == requested.CandidateRevisionID &&
		stored.ActorUserID == requested.ActorUserID &&
		stored.ActorSessionID == requested.ActorSessionID &&
		stored.Reason == requested.Reason &&
		stored.ConfirmationHash == requested.ConfirmationHash
}

func (repository *AttributeValueMigrationRepository) transactionalClient(ctx context.Context) (*generate.Client, error) {
	if repository == nil || repository.client == nil || repository.client.Client() == nil || ctx == nil || ctx.Err() != nil {
		return nil, attributemigration.ErrInvalidMigration
	}
	client, err := platformtx.Client(ctx, nil)
	if err != nil {
		return nil, err
	}
	return client, nil
}

func validMigrationIntent(request attributemigration.Request) bool {
	return attributemigration.ValidateIntent(request) == nil
}

func validMigrationRequest(request attributemigration.Request) bool {
	return attributemigration.ValidateRequest(request) == nil
}

func migrationDefinitionError(err error) error {
	if err == nil || generate.IsNotFound(err) || errors.Is(err, stdsql.ErrNoRows) {
		return attributemigration.ErrDefinitionDrift
	}
	return fmt.Errorf("attribute migration definition: %w", err)
}

func cloneAttributeMigrationUUID(value *uuid.UUID) *uuid.UUID {
	if value == nil {
		return nil
	}
	cloned := *value
	return &cloned
}

func attributeMigrationRevision(
	record *generate.AttributeDefinitionRevision,
) entity.AttributeDefinitionRevision {
	if record == nil {
		return entity.AttributeDefinitionRevision{}
	}
	return entity.AttributeDefinitionRevision{
		ID:           record.ID,
		DefinitionID: record.DefinitionID,
		Revision:     record.Revision,
		DataType:     entity.AttributeValueType(record.DataType),
		Label:        record.Label,
		Description:  record.Description,
		Visibility:   entity.AttributeVisibility(record.Visibility),
		UserEditable: record.UserEditable,
		DisplayOrder: record.DisplayOrder,
		Validation:   append([]byte(nil), record.Validation...),
	}
}
