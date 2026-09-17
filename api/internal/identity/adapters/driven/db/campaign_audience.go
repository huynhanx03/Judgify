package db

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	dbent "github.com/huynhanx03/judgify/internal/ent"
	platformtx "github.com/huynhanx03/judgify/internal/platform/transaction"
	"github.com/huynhanx03/judgify/internal/ports"
)

const (
	campaignAudienceExplicit = "explicit_users"
	campaignAudienceRole     = "role"
	campaignAudienceAll      = "all_active"
)

type CampaignAudienceRepository struct {
	client *dbent.EntClient
}

func NewCampaignAudienceRepository(client *dbent.EntClient) (*CampaignAudienceRepository, error) {
	if client == nil || client.Client() == nil {
		return nil, ports.ErrInvalidCampaignAudience
	}
	return &CampaignAudienceRepository{client: client}, nil
}

var _ ports.CampaignAudienceParticipant = (*CampaignAudienceRepository)(nil)

func (repository *CampaignAudienceRepository) SnapshotCampaignAudience(
	ctx context.Context,
	selector ports.CampaignAudienceSelector,
	limit int,
	at time.Time,
) (ports.CampaignAudienceSnapshot, error) {
	if repository == nil || repository.client == nil || ctx == nil ||
		limit < 1 || at.IsZero() || !validCampaignAudienceSelector(selector) {
		return ports.CampaignAudienceSnapshot{}, ports.ErrInvalidCampaignAudience
	}
	executor := repository.client.DB(ctx).User
	var (
		rows *sql.Rows
		err  error
	)
	switch selector.Kind {
	case campaignAudienceExplicit:
		encodedIDs, encodeErr := json.Marshal(uuidStrings(selector.UserIDs))
		if encodeErr != nil {
			return ports.CampaignAudienceSnapshot{}, ports.ErrInvalidCampaignAudience
		}
		rows, err = executor.QueryContext(ctx, `
			WITH requested(id) AS (
				SELECT value::uuid
				FROM jsonb_array_elements_text($1::jsonb) value
			)
			SELECT account.id, account.username
			FROM users account
			JOIN requested ON requested.id = account.id
			WHERE account.status = 'active'
				AND account.deleted_at IS NULL
			ORDER BY account.id
			LIMIT $2`, encodedIDs, limit+1)
	case campaignAudienceRole:
		rows, err = executor.QueryContext(ctx, `
			WITH RECURSIVE selected_role(subject) AS (
				SELECT 'role:' || key
				FROM roles
				WHERE id = $1 AND deleted_at IS NULL
			),
			member_roles(subject) AS (
				SELECT subject FROM selected_role
				UNION
				SELECT rule.v0
				FROM casbin_rules rule
				JOIN member_roles parent ON rule.ptype = 'g' AND rule.v1 = parent.subject
				WHERE rule.v0 LIKE 'role:%'
			)
			SELECT DISTINCT account.id, account.username
			FROM users account
			JOIN casbin_rules membership
				ON membership.ptype = 'g'
				AND membership.v0 = 'user:' || account.id::text
			JOIN member_roles allowed ON allowed.subject = membership.v1
			WHERE account.status = 'active' AND account.deleted_at IS NULL
			ORDER BY account.id
			LIMIT $2`, *selector.RoleID, limit+1)
	case campaignAudienceAll:
		rows, err = executor.QueryContext(ctx, `
			SELECT id, username
			FROM users
			WHERE status = 'active' AND deleted_at IS NULL
			ORDER BY id
			LIMIT $1`, limit+1)
	default:
		return ports.CampaignAudienceSnapshot{}, ports.ErrInvalidCampaignAudience
	}
	if err != nil {
		return ports.CampaignAudienceSnapshot{}, fmt.Errorf("snapshot campaign audience: %w", err)
	}
	defer func() { _ = rows.Close() }()
	members := make([]ports.CampaignAudienceMember, 0, min(limit, 128))
	for rows.Next() {
		var member ports.CampaignAudienceMember
		if err := rows.Scan(&member.UserID, &member.Username); err != nil {
			return ports.CampaignAudienceSnapshot{}, fmt.Errorf("scan campaign audience: %w", err)
		}
		members = append(members, member)
	}
	if err := rows.Err(); err != nil {
		return ports.CampaignAudienceSnapshot{}, fmt.Errorf("read campaign audience: %w", err)
	}
	if selector.Kind == campaignAudienceRole && len(members) == 0 {
		var exists bool
		existsRows, err := executor.QueryContext(ctx, `
			SELECT EXISTS (
				SELECT 1 FROM roles WHERE id = $1 AND deleted_at IS NULL
			)`, *selector.RoleID)
		if err != nil {
			return ports.CampaignAudienceSnapshot{}, fmt.Errorf("verify campaign role: %w", err)
		}
		if !existsRows.Next() {
			closeErr := existsRows.Close()
			if closeErr != nil {
				return ports.CampaignAudienceSnapshot{}, fmt.Errorf("verify campaign role: %w", closeErr)
			}
			return ports.CampaignAudienceSnapshot{}, fmt.Errorf("verify campaign role: %w", sql.ErrNoRows)
		}
		if err := existsRows.Scan(&exists); err != nil {
			_ = existsRows.Close()
			return ports.CampaignAudienceSnapshot{}, fmt.Errorf("verify campaign role: %w", err)
		}
		if err := existsRows.Close(); err != nil {
			return ports.CampaignAudienceSnapshot{}, fmt.Errorf("verify campaign role: %w", err)
		}
		if !exists {
			return ports.CampaignAudienceSnapshot{}, ports.ErrCampaignRoleNotFound
		}
	}
	if len(members) > limit {
		return ports.CampaignAudienceSnapshot{}, ports.IdentityErrCampaignAudienceTooLarge
	}
	return ports.CampaignAudienceSnapshot{Members: members, At: at.UTC()}, nil
}

func (repository *CampaignAudienceRepository) CheckCampaignRecipient(
	ctx context.Context,
	userID uuid.UUID,
	audienceKind string,
	roleID *uuid.UUID,
) (ports.CampaignRecipientEligibility, error) {
	if repository == nil || repository.client == nil || ctx == nil ||
		userID == uuid.Nil || !validCampaignAudienceShape(audienceKind, roleID) {
		return ports.CampaignRecipientEligibility{}, ports.ErrInvalidCampaignAudience
	}
	executor := repository.client.DB(ctx).User
	rows, err := executor.QueryContext(ctx, `
		SELECT status, deleted_at
		FROM users
		WHERE id = $1
		FOR UPDATE`, userID)
	if err != nil {
		return ports.CampaignRecipientEligibility{}, fmt.Errorf(
			"lock campaign recipient account: %w", err,
		)
	}
	if !rows.Next() {
		readErr := rows.Err()
		_ = rows.Close()
		if readErr != nil {
			return ports.CampaignRecipientEligibility{}, fmt.Errorf(
				"read campaign recipient account: %w", readErr,
			)
		}
		return ports.CampaignRecipientEligibility{
			SkipCode: "inactive_account",
		}, nil
	}
	var (
		status    string
		deletedAt sql.NullTime
	)
	if err := rows.Scan(&status, &deletedAt); err != nil {
		_ = rows.Close()
		return ports.CampaignRecipientEligibility{}, fmt.Errorf(
			"scan campaign recipient account: %w", err,
		)
	}
	if rows.Next() {
		_ = rows.Close()
		return ports.CampaignRecipientEligibility{}, fmt.Errorf(
			"campaign recipient account query returned multiple rows",
		)
	}
	if err := rows.Close(); err != nil {
		return ports.CampaignRecipientEligibility{}, fmt.Errorf(
			"close campaign recipient account: %w", err,
		)
	}
	if status != "active" || deletedAt.Valid {
		return ports.CampaignRecipientEligibility{
			SkipCode: "inactive_account",
		}, nil
	}
	if audienceKind == campaignAudienceRole {
		var eligible bool
		rows, err := executor.QueryContext(ctx, `
			WITH RECURSIVE selected_role(subject) AS (
				SELECT 'role:' || key FROM roles
				WHERE id = $2 AND deleted_at IS NULL
			),
			member_roles(subject) AS (
				SELECT subject FROM selected_role
				UNION
				SELECT rule.v0
				FROM casbin_rules rule
				JOIN member_roles parent ON rule.ptype = 'g' AND rule.v1 = parent.subject
				WHERE rule.v0 LIKE 'role:%'
			)
			SELECT EXISTS (
				SELECT 1
				FROM casbin_rules membership
				JOIN member_roles allowed ON allowed.subject = membership.v1
				WHERE membership.ptype = 'g'
					AND membership.v0 = 'user:' || $1::text
			)`, userID, *roleID)
		if err != nil {
			return ports.CampaignRecipientEligibility{}, fmt.Errorf("check campaign role eligibility: %w", err)
		}
		if !rows.Next() {
			_ = rows.Close()
			return ports.CampaignRecipientEligibility{}, fmt.Errorf(
				"check campaign role eligibility: %w", sql.ErrNoRows,
			)
		}
		if err := rows.Scan(&eligible); err != nil {
			_ = rows.Close()
			return ports.CampaignRecipientEligibility{}, fmt.Errorf("check campaign role eligibility: %w", err)
		}
		if err := rows.Close(); err != nil {
			return ports.CampaignRecipientEligibility{}, fmt.Errorf("check campaign role eligibility: %w", err)
		}
		if !eligible {
			return ports.CampaignRecipientEligibility{SkipCode: "inactive_or_role_removed"}, nil
		}
		return ports.CampaignRecipientEligibility{Eligible: true}, nil
	}
	return ports.CampaignRecipientEligibility{Eligible: true}, nil
}

func (repository *CampaignAudienceRepository) RequireRecentReauthentication(
	ctx context.Context,
	userID uuid.UUID,
	sessionID uuid.UUID,
	recentAfter time.Time,
) error {
	if repository == nil || repository.client == nil || ctx == nil ||
		userID == uuid.Nil || sessionID == uuid.Nil || recentAfter.IsZero() {
		return ports.ErrInvalidCampaignAudience
	}
	if _, err := platformtx.Client(ctx, nil); err != nil {
		return err
	}
	rows, err := repository.client.DB(ctx).AuthSession.QueryContext(ctx, `
		SELECT last_reauthenticated_at
		FROM auth_sessions
		WHERE id = $1
			AND user_id = $2
			AND status = 'active'
			AND idle_expires_at > clock_timestamp()
			AND absolute_expires_at > clock_timestamp()
		FOR UPDATE`, sessionID, userID)
	if err != nil {
		return fmt.Errorf("lock campaign actor session: %w", err)
	}
	defer func() { _ = rows.Close() }()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return fmt.Errorf("read campaign actor session: %w", err)
		}
		return ports.ErrCampaignRecentAuthenticationRequired
	}
	var last time.Time
	if err := rows.Scan(&last); err != nil {
		return fmt.Errorf("scan campaign actor session: %w", err)
	}
	if rows.Next() {
		return fmt.Errorf("campaign actor session query returned multiple rows")
	}
	if last.UTC().Before(recentAfter.UTC()) {
		return ports.ErrCampaignRecentAuthenticationRequired
	}
	return nil
}

func validCampaignAudienceSelector(selector ports.CampaignAudienceSelector) bool {
	if !validCampaignAudienceShape(selector.Kind, selector.RoleID) {
		return false
	}
	switch selector.Kind {
	case campaignAudienceExplicit:
		if len(selector.UserIDs) == 0 {
			return false
		}
		for index, id := range selector.UserIDs {
			if id == uuid.Nil || (index > 0 && strings.Compare(
				selector.UserIDs[index-1].String(), id.String(),
			) >= 0) {
				return false
			}
		}
		return true
	default:
		return len(selector.UserIDs) == 0
	}
}

func validCampaignAudienceShape(kind string, roleID *uuid.UUID) bool {
	switch kind {
	case campaignAudienceRole:
		return roleID != nil && *roleID != uuid.Nil
	case campaignAudienceExplicit, campaignAudienceAll:
		return roleID == nil
	default:
		return false
	}
}

func uuidStrings(values []uuid.UUID) []string {
	result := make([]string, len(values))
	for index, value := range values {
		result[index] = value.String()
	}
	return result
}
