package db

import (
	"context"
	"crypto/subtle"
	stdsql "database/sql"
	stdjson "encoding/json"
	"errors"
	"fmt"
	"time"

	entsql "entgo.io/ent/dialect/sql"
	"github.com/google/uuid"

	"github.com/huynhanx03/go-common/pkg/encoding/json"

	entity "github.com/huynhanx03/judgify/internal/cultivation/domain"
	dbent "github.com/huynhanx03/judgify/internal/ent"
	"github.com/huynhanx03/judgify/internal/ports"
)

const (
	maximumStoredRewardRuleBytes = 16 << 10
	maximumRewardRulePageSize    = 100
)

type RewardRuleRepository struct {
	client *dbent.EntClient
}

func NewRewardRuleRepository(client *dbent.EntClient) (*RewardRuleRepository, error) {
	if client == nil || client.Client() == nil {
		return nil, entity.ErrInvalidRewardRule
	}
	return &RewardRuleRepository{client: client}, nil
}

var _ ports.RewardRuleRepository = (*RewardRuleRepository)(nil)

// InstallDefaultRewardRule verifies or installs the code-owned initial rule.
// It never mutates an existing immutable revision with different bytes.
func InstallDefaultRewardRule(ctx context.Context, client *dbent.EntClient) error {
	if ctx == nil || client == nil || ctx.Err() != nil {
		return entity.ErrInvalidRewardRule
	}
	driver, ok := client.DialectDriver()
	if !ok {
		return entity.ErrInvalidRewardRule
	}
	sqlDriver, ok := driver.(*entsql.Driver)
	if !ok || sqlDriver.DB() == nil {
		return entity.ErrInvalidRewardRule
	}
	rule := entity.DefaultRewardRuleRevision()
	configuration, err := stdjson.Marshal(rule.Configuration)
	if err != nil {
		return fmt.Errorf("encode default reward rule: %w", err)
	}
	_, err = sqlDriver.DB().ExecContext(ctx, `
		INSERT INTO reward_rule_revisions (
			id, rule_key, revision, algorithm_version, configuration,
			checksum, request_checksum, effective_from, created_at
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,CURRENT_TIMESTAMP)
		ON CONFLICT (rule_key, revision) DO NOTHING`,
		rule.ID, rule.RuleKey, rule.Revision, rule.AlgorithmVersion,
		configuration, rule.Checksum[:], rule.RequestChecksum[:], rule.EffectiveFrom,
	)
	if err != nil {
		return fmt.Errorf("install default reward rule: %w", err)
	}
	loaded, err := loadRewardRuleByID(ctx, sqlDriver.DB(), rule.ID)
	if err != nil {
		return fmt.Errorf("verify default reward rule: %w", err)
	}
	if loaded.Checksum != rule.Checksum || loaded.RequestChecksum != rule.RequestChecksum || !loaded.Valid() {
		return entity.ErrInvalidRewardRule
	}
	return nil
}

func (repository *RewardRuleRepository) Current(
	ctx context.Context,
	at time.Time,
) (entity.RewardRuleRevision, error) {
	if repository == nil || repository.client == nil || ctx == nil {
		return entity.RewardRuleRevision{}, entity.ErrInvalidRewardRule
	}
	if at.IsZero() {
		at = time.Now().UTC()
	}
	return loadEffectiveRewardRule(ctx, repository.client.DB(ctx).RewardRuleRevision, at)
}

func (repository *RewardRuleRepository) List(
	ctx context.Context,
	cursor *int64,
	limit int,
) (ports.RewardRulePage, error) {
	if repository == nil || repository.client == nil || ctx == nil ||
		limit < 1 || limit > maximumRewardRulePageSize ||
		(cursor != nil && *cursor < 1) {
		return ports.RewardRulePage{}, entity.ErrInvalidRewardRule
	}
	rows, err := repository.client.DB(ctx).RewardRuleRevision.QueryContext(ctx, `
		SELECT id, rule_key, revision, algorithm_version, configuration,
			checksum, request_checksum, effective_from
		FROM reward_rule_revisions
		WHERE rule_key = $1 AND ($2::bigint IS NULL OR revision < $2)
		ORDER BY revision DESC
		LIMIT $3`, entity.DefaultRewardRuleKey, cursor, limit+1)
	if err != nil {
		return ports.RewardRulePage{}, fmt.Errorf("list reward rule revisions: %w", err)
	}
	defer func() { _ = rows.Close() }()
	rules := make([]entity.RewardRuleRevision, 0, limit+1)
	for rows.Next() {
		rule, scanErr := scanRewardRule(rows.Scan)
		if scanErr != nil {
			return ports.RewardRulePage{}, scanErr
		}
		rules = append(rules, rule)
	}
	if err := rows.Err(); err != nil {
		return ports.RewardRulePage{}, fmt.Errorf("iterate reward rule revisions: %w", err)
	}
	page := ports.RewardRulePage{Rules: rules}
	if len(rules) > limit {
		page.Rules = rules[:limit]
		value := page.Rules[len(page.Rules)-1].Revision
		page.NextCursor = &value
	}
	return page, nil
}

func (repository *RewardRuleRepository) Publish(
	ctx context.Context,
	command entity.RewardRulePublicationCommand,
) (entity.RewardRulePublicationResult, error) {
	if repository == nil || repository.client == nil || ctx == nil {
		return entity.RewardRulePublicationResult{}, entity.ErrInvalidRewardRule
	}
	command, err := entity.NormalizeRewardRulePublicationCommand(command)
	if err != nil {
		return entity.RewardRulePublicationResult{}, err
	}
	executor := repository.client.DB(ctx).RewardRuleRevision
	requestChecksum := entity.RewardRulePublicationRequestChecksum(command)
	if result, found, replayErr := readRewardRuleReplay(ctx, executor, command.ID, requestChecksum); replayErr != nil || found {
		return result, replayErr
	}
	if err := acquireRewardTimelineLock(ctx, executor); err != nil {
		return entity.RewardRulePublicationResult{}, fmt.Errorf("lock reward timeline: %w", err)
	}
	if result, found, replayErr := readRewardRuleReplay(ctx, executor, command.ID, requestChecksum); replayErr != nil || found {
		return result, replayErr
	}
	current, err := loadLatestRewardRuleForUpdate(ctx, executor)
	if err != nil {
		return entity.RewardRulePublicationResult{}, err
	}
	preview, err := entity.NewRewardRulePublicationPreview(current, command.Configuration)
	if err != nil || current.Revision != command.ExpectedRevision ||
		subtle.ConstantTimeCompare(preview.Checksum[:], command.PreviewChecksum[:]) != 1 {
		return entity.RewardRulePublicationResult{}, entity.ErrRewardRuleConflict
	}
	now, err := rewardTimelineNow(ctx, executor)
	if err != nil {
		return entity.RewardRulePublicationResult{}, fmt.Errorf("read reward publication time: %w", err)
	}
	rule := entity.RewardRuleRevision{
		ID: command.ID, RuleKey: entity.DefaultRewardRuleKey,
		Revision:         current.Revision + 1,
		AlgorithmVersion: command.AlgorithmVersion,
		Configuration:    command.Configuration,
		RequestChecksum:  requestChecksum,
		EffectiveFrom:    now.UTC(),
	}
	rule.Checksum, err = rule.CalculateChecksum()
	if err != nil || !rule.Valid() {
		return entity.RewardRulePublicationResult{}, entity.ErrInvalidRewardRule
	}
	configuration, err := stdjson.Marshal(rule.Configuration)
	if err != nil {
		return entity.RewardRulePublicationResult{}, fmt.Errorf("encode reward rule publication: %w", err)
	}
	_, err = executor.ExecContext(ctx, `
		INSERT INTO reward_rule_revisions (
			id, rule_key, revision, algorithm_version, configuration,
			checksum, request_checksum, effective_from, created_at
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$8)`,
		rule.ID, rule.RuleKey, rule.Revision, rule.AlgorithmVersion,
		configuration, rule.Checksum[:], rule.RequestChecksum[:], rule.EffectiveFrom,
	)
	if err != nil {
		return entity.RewardRulePublicationResult{}, fmt.Errorf("publish reward rule revision: %w", err)
	}
	return entity.RewardRulePublicationResult{Rule: rule}, nil
}

type rewardRuleQueryer interface {
	QueryContext(context.Context, string, ...any) (*stdsql.Rows, error)
}

func loadEffectiveRewardRule(ctx context.Context, queryer rewardRuleQueryer, at time.Time) (entity.RewardRuleRevision, error) {
	if ctx == nil || queryer == nil || at.IsZero() {
		return entity.RewardRuleRevision{}, entity.ErrInvalidRewardRule
	}
	rule, err := queryRewardRule(ctx, queryer, `
		SELECT id, rule_key, revision, algorithm_version, configuration,
			checksum, request_checksum, effective_from
		FROM reward_rule_revisions
		WHERE rule_key = $1 AND effective_from <= $2
		ORDER BY effective_from DESC, revision DESC
		LIMIT 1`, entity.DefaultRewardRuleKey, at.UTC())
	if errors.Is(err, stdsql.ErrNoRows) {
		return entity.RewardRuleRevision{}, entity.ErrInvalidRewardRule
	}
	return rule, err
}

func loadLatestRewardRuleForUpdate(ctx context.Context, queryer rewardRuleQueryer) (entity.RewardRuleRevision, error) {
	// reward_rule_revisions is append-only; rows can never be updated or
	// deleted. acquireRewardTimelineLock (pg_advisory_xact_lock) taken by the
	// caller already serialises all writers, so FOR UPDATE is both unprivileged
	// and redundant.
	rule, err := queryRewardRule(ctx, queryer, `
		SELECT id, rule_key, revision, algorithm_version, configuration,
			checksum, request_checksum, effective_from
		FROM reward_rule_revisions
		WHERE rule_key = $1
		ORDER BY revision DESC
		LIMIT 1`, entity.DefaultRewardRuleKey)
	if errors.Is(err, stdsql.ErrNoRows) {
		return entity.RewardRuleRevision{}, entity.ErrInvalidRewardRule
	}
	return rule, err
}

func loadRewardRuleByID(ctx context.Context, queryer rewardRuleQueryer, id uuid.UUID) (entity.RewardRuleRevision, error) {
	return queryRewardRule(ctx, queryer, `
		SELECT id, rule_key, revision, algorithm_version, configuration,
			checksum, request_checksum, effective_from
		FROM reward_rule_revisions WHERE id = $1`, id)
}

func queryRewardRule(ctx context.Context, queryer rewardRuleQueryer, statement string, arguments ...any) (entity.RewardRuleRevision, error) {
	if ctx == nil || queryer == nil {
		return entity.RewardRuleRevision{}, entity.ErrInvalidRewardRule
	}
	return scanRewardRule(querySingleRow(ctx, queryer, statement, arguments...).Scan)
}

func scanRewardRule(scan func(...any) error) (entity.RewardRuleRevision, error) {
	var rule entity.RewardRuleRevision
	var configuration, checksum, requestChecksum []byte
	if err := scan(
		&rule.ID, &rule.RuleKey, &rule.Revision, &rule.AlgorithmVersion,
		&configuration, &checksum, &requestChecksum, &rule.EffectiveFrom,
	); err != nil {
		return entity.RewardRuleRevision{}, err
	}
	if len(checksum) != 32 || len(requestChecksum) != 32 ||
		len(configuration) == 0 || len(configuration) > maximumStoredRewardRuleBytes {
		return entity.RewardRuleRevision{}, entity.ErrInvalidRewardRule
	}
	if err := json.UnmarshalStrictLimit(configuration, &rule.Configuration, maximumStoredRewardRuleBytes); err != nil {
		return entity.RewardRuleRevision{}, entity.ErrInvalidRewardRule
	}
	copy(rule.Checksum[:], checksum)
	copy(rule.RequestChecksum[:], requestChecksum)
	rule.EffectiveFrom = rule.EffectiveFrom.UTC()
	if !rule.Valid() {
		return entity.RewardRuleRevision{}, entity.ErrInvalidRewardRule
	}
	return rule, nil
}

func readRewardRuleReplay(
	ctx context.Context,
	queryer rewardRuleQueryer,
	id uuid.UUID,
	requestChecksum [32]byte,
) (entity.RewardRulePublicationResult, bool, error) {
	rule, err := loadRewardRuleByID(ctx, queryer, id)
	if errors.Is(err, stdsql.ErrNoRows) {
		return entity.RewardRulePublicationResult{}, false, nil
	}
	if err != nil {
		return entity.RewardRulePublicationResult{}, false, err
	}
	if subtle.ConstantTimeCompare(rule.RequestChecksum[:], requestChecksum[:]) != 1 {
		return entity.RewardRulePublicationResult{}, false, entity.ErrRewardRuleConflict
	}
	return entity.RewardRulePublicationResult{Rule: rule, Replayed: true}, true, nil
}
