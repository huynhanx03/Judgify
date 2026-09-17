package db

import (
	"bytes"
	"context"
	stdsql "database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"

	internalent "github.com/huynhanx03/judgify/internal/ent"
	"github.com/huynhanx03/judgify/internal/ent/generate"
	"github.com/huynhanx03/judgify/internal/identity/attributevalue"
	entity "github.com/huynhanx03/judgify/internal/identity/domain"
	"github.com/huynhanx03/judgify/internal/ports"
)

type PublicProfileRepository struct {
	client *internalent.EntClient
}

type publicProfileRawQuerier interface {
	QueryContext(context.Context, string, ...any) (*stdsql.Rows, error)
}

func NewPublicProfileRepository(
	client *internalent.EntClient,
) (*PublicProfileRepository, error) {
	if client == nil || client.Client() == nil {
		return nil, ports.ErrInvalidPublicProfile
	}
	return &PublicProfileRepository{client: client}, nil
}

var _ ports.PublicProfileRepository = (*PublicProfileRepository)(nil)

// GetPublicByUsername runs inside the caller's read snapshot. The account
// gate and active-revision value query remain separate so a profile with no
// public values is still a valid public profile instead of disappearing in an
// inner join.
func (repository *PublicProfileRepository) GetPublicByUsername(
	ctx context.Context,
	username string,
) (*entity.PublicProfile, error) {
	if repository == nil || repository.client == nil || ctx == nil ||
		ctx.Err() != nil || username == "" || strings.TrimSpace(username) != username {
		return nil, ports.ErrInvalidPublicProfile
	}

	profile := &entity.PublicProfile{Username: username}
	err := scanSinglePublicProfileRow(ctx, repository.client.DB(ctx).User, `
		SELECT id, username, created_at
		FROM users
		WHERE username = $1
		  AND status = 'active'
		  AND profile_visibility = 'public'
		  AND deleted_at IS NULL`, []any{username},
		&profile.UserID,
		&profile.Username,
		&profile.JoinedAt,
	)
	if errors.Is(err, stdsql.ErrNoRows) {
		return nil, ports.ErrPublicProfileNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("read public profile account: %w", err)
	}
	if profile.UserID == uuid.Nil || profile.Username != username ||
		profile.JoinedAt.IsZero() {
		return nil, ports.ErrInvalidPublicProfile
	}

	rows, err := repository.client.DB(ctx).UserAttributeValue.QueryContext(ctx, `
		SELECT definition.key, revision.data_type, revision.label,
		       COALESCE(revision.description, ''), revision.validation,
		       value.canonical_value
		FROM user_attribute_values AS value
		JOIN attribute_definitions AS definition
		  ON definition.id = value.attribute_id
		 AND definition.status = 'active'
		 AND definition.deleted_at IS NULL
		 AND definition.active_revision_id = value.definition_revision_id
		JOIN attribute_definition_revisions AS revision
		  ON revision.id = definition.active_revision_id
		 AND revision.definition_id = definition.id
		 AND revision.visibility = 'public'
		WHERE value.user_id = $1
		  AND value.deleted_at IS NULL
		ORDER BY revision.display_order, definition.key, definition.id
		LIMIT $2`, profile.UserID, entity.MaximumPublicProfileAttributes+1)
	if err != nil {
		return nil, fmt.Errorf("read public profile attributes: %w", err)
	}
	defer func() { _ = rows.Close() }()

	profile.Attributes = make([]entity.PublicProfileAttribute, 0)
	for rows.Next() {
		var attribute entity.PublicProfileAttribute
		var validation json.RawMessage
		if err := rows.Scan(
			&attribute.Key,
			&attribute.DataType,
			&attribute.Label,
			&attribute.Description,
			&validation,
			&attribute.Value,
		); err != nil {
			return nil, fmt.Errorf("scan public profile attribute: %w", err)
		}
		if len(profile.Attributes) == entity.MaximumPublicProfileAttributes ||
			!validPublicProfileAttribute(attribute, validation) {
			return nil, ports.ErrInvalidPublicProfile
		}
		canonical, err := attributevalue.Canonicalize(
			attribute.Value,
			entity.AttributeDefinitionRevision{
				DataType:    attribute.DataType,
				Label:       attribute.Label,
				Description: attribute.Description,
				Visibility:  entity.AttributeVisibilityPublic,
				Validation:  validation,
			},
		)
		if err != nil || !bytes.Equal(canonical, attribute.Value) {
			return nil, ports.ErrInvalidPublicProfile
		}
		attribute.Value = append(json.RawMessage(nil), canonical...)
		profile.Attributes = append(profile.Attributes, attribute)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate public profile attributes: %w", err)
	}
	return profile, nil
}

func (repository *PublicProfileRepository) GetPrivacy(
	ctx context.Context,
	userID uuid.UUID,
) (*entity.ProfilePrivacyState, error) {
	if repository == nil || repository.client == nil || ctx == nil ||
		ctx.Err() != nil || userID == uuid.Nil {
		return nil, ports.ErrInvalidProfilePrivacyCommand
	}
	state := &entity.ProfilePrivacyState{UserID: userID}
	err := scanSinglePublicProfileRow(ctx, repository.client.DB(ctx).User, `
		SELECT id, profile_visibility, profile_visibility_version,
		       profile_visibility_updated_at
		FROM users
		WHERE id = $1 AND status = 'active' AND deleted_at IS NULL`, []any{userID},
		&state.UserID,
		&state.Visibility,
		&state.Version,
		&state.UpdatedAt,
	)
	if errors.Is(err, stdsql.ErrNoRows) {
		return nil, ports.ErrPublicProfileNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("read profile privacy: %w", err)
	}
	if !state.Valid() || state.UserID != userID {
		return nil, ports.ErrInvalidProfilePrivacyCommand
	}
	return state, nil
}

func (repository *PublicProfileRepository) UpdatePrivacyCAS(
	ctx context.Context,
	userID uuid.UUID,
	expectedVersion int64,
	visibility entity.ProfileVisibility,
) (*entity.ProfilePrivacyState, error) {
	if repository == nil || repository.client == nil || ctx == nil ||
		ctx.Err() != nil || userID == uuid.Nil || expectedVersion < 1 ||
		!visibility.Valid() {
		return nil, ports.ErrInvalidProfilePrivacyCommand
	}
	state := &entity.ProfilePrivacyState{UserID: userID}
	err := scanSinglePublicProfileRow(ctx, repository.client.DB(ctx).User, `
		UPDATE users
		SET profile_visibility = $3,
		    profile_visibility_version = profile_visibility_version + 1,
		    profile_visibility_updated_at = CURRENT_TIMESTAMP,
		    updated_at = CURRENT_TIMESTAMP
		WHERE id = $1
		  AND profile_visibility_version = $2
		  AND profile_visibility <> $3
		  AND status = 'active'
		  AND deleted_at IS NULL
		RETURNING id, profile_visibility, profile_visibility_version,
		          profile_visibility_updated_at`,
		[]any{userID, expectedVersion, visibility},
		&state.UserID,
		&state.Visibility,
		&state.Version,
		&state.UpdatedAt,
	)
	if errors.Is(err, stdsql.ErrNoRows) {
		return nil, ports.ErrProfilePrivacyVersionConflict
	}
	if err != nil {
		return nil, fmt.Errorf("update profile privacy: %w", err)
	}
	if !state.Valid() || state.UserID != userID ||
		state.Visibility != visibility || state.Version != expectedVersion+1 {
		return nil, ports.ErrInvalidProfilePrivacyCommand
	}
	return state, nil
}

func (repository *PublicProfileRepository) LoadPrivacyCommandReceipt(
	ctx context.Context,
	commandID uuid.UUID,
) (ports.ProfilePrivacyCommandReceipt, bool, error) {
	if repository == nil || repository.client == nil || ctx == nil ||
		ctx.Err() != nil || commandID == uuid.Nil {
		return ports.ProfilePrivacyCommandReceipt{}, false,
			ports.ErrInvalidProfilePrivacyCommand
	}
	record, err := repository.client.DB(ctx).OutboxEvent.Get(ctx, commandID)
	if generate.IsNotFound(err) {
		return ports.ProfilePrivacyCommandReceipt{}, false, nil
	}
	if err != nil {
		return ports.ProfilePrivacyCommandReceipt{}, false,
			fmt.Errorf("read profile privacy receipt: %w", err)
	}
	if record == nil {
		return ports.ProfilePrivacyCommandReceipt{}, false,
			ports.ErrInvalidProfilePrivacyCommand
	}
	return ports.ProfilePrivacyCommandReceipt{
		EventID:          record.ID,
		EventType:        record.EventType,
		AggregateType:    record.AggregateType,
		AggregateID:      record.AggregateID,
		AggregateVersion: record.AggregateVersion,
		Payload:          append(json.RawMessage(nil), record.Payload...),
		OccurredAt:       record.OccurredAt.UTC(),
	}, true, nil
}

func validPublicProfileAttribute(
	attribute entity.PublicProfileAttribute,
	validation json.RawMessage,
) bool {
	return attribute.Key != "" && strings.TrimSpace(attribute.Key) == attribute.Key &&
		len(attribute.Key) <= 64 &&
		attribute.DataType.Valid() && attribute.Label != "" &&
		strings.TrimSpace(attribute.Label) == attribute.Label &&
		len(attribute.Label) <= 128 && len(attribute.Description) <= 1_024 &&
		len(attribute.Value) > 0 &&
		len(attribute.Value) <= attributevalue.MaxCanonicalBytes &&
		json.Valid(attribute.Value) &&
		len(validation) > 0 && json.Valid(validation)
}

func scanSinglePublicProfileRow(
	ctx context.Context,
	querier publicProfileRawQuerier,
	statement string,
	arguments []any,
	destinations ...any,
) error {
	if ctx == nil || querier == nil || statement == "" || len(destinations) == 0 {
		return ports.ErrInvalidPublicProfile
	}
	rows, err := querier.QueryContext(ctx, statement, arguments...)
	if err != nil {
		return err
	}
	defer func() { _ = rows.Close() }()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return err
		}
		return stdsql.ErrNoRows
	}
	if err := rows.Scan(destinations...); err != nil {
		return err
	}
	if rows.Next() {
		return ports.ErrInvalidPublicProfile
	}
	return rows.Err()
}
