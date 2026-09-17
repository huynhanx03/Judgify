package db

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"

	casbinmodel "github.com/casbin/casbin/v3/model"
	entadapter "github.com/casbin/ent-adapter"
	adapterent "github.com/casbin/ent-adapter/ent"
	"github.com/google/uuid"
	authcasbin "github.com/huynhanx03/go-common/pkg/security/authorization/casbin"

	dbent "github.com/huynhanx03/judgify/internal/ent"
	"github.com/huynhanx03/judgify/internal/ent/generate"
	"github.com/huynhanx03/judgify/internal/ent/generate/authorizationstate"
	"github.com/huynhanx03/judgify/internal/ent/generate/casbinrule"
	"github.com/huynhanx03/judgify/internal/ent/generate/role"
	"github.com/huynhanx03/judgify/internal/ent/generate/user"
	platformtx "github.com/huynhanx03/judgify/internal/platform/transaction"
	"github.com/huynhanx03/judgify/internal/ports"
)

const (
	authorizationStateID                 = "default"
	maxAuthorizationPolicyTypeBytes      = 64
	maxAuthorizationPolicyValueBytes     = 512
	maxAuthorizationRules                = 100_000
	maxAuthorizationRulesPerMutation     = 8_192
	maxAuthorizationReferenceLookupItems = 10_000
	maxAuthorizationRoleKeyBytes         = 64
)

// AuthorizationPolicyRepository persists canonical Casbin rules and the
// single-tenant desired-revision fence. It deliberately owns no connection or
// schema lifecycle and never delegates writes to Casbin AutoSave.
type AuthorizationPolicyRepository struct {
	client *dbent.EntClient
}

// NewAuthorizationPolicyRepository creates a transaction-aware authorization
// policy repository over the application's already-migrated Ent client.
func NewAuthorizationPolicyRepository(client *dbent.EntClient) ports.AuthorizationPolicyRepository {
	return &AuthorizationPolicyRepository{client: client}
}

func (repository *AuthorizationPolicyRepository) EnsureState(
	ctx context.Context,
	initialRevision int64,
) error {
	if !repository.validContext(ctx) || initialRevision <= 0 {
		return ports.ErrInvalidAuthorizationPolicy
	}
	err := repository.client.DB(ctx).AuthorizationState.Create().
		SetID(authorizationStateID).
		SetDesiredRevision(initialRevision).
		OnConflictColumns(authorizationstate.FieldID).
		DoNothing().
		Exec(ctx)
	return authorizationPersistenceError("ensure state", err)
}

// LoadRules uses the pinned Ent adapter for the committed startup snapshot.
// The local adapter patch disables Schema.Create; writes remain exclusively in
// this repository so they can join Judgify's transaction and revision fence.
func (repository *AuthorizationPolicyRepository) LoadRules(
	ctx context.Context,
	modelText string,
) ([]authcasbin.PolicyRule, error) {
	if !repository.validContext(ctx) || strings.TrimSpace(modelText) == "" {
		return nil, ports.ErrInvalidAuthorizationPolicy
	}
	driver, ok := repository.client.DialectDriver()
	if !ok {
		return nil, ports.ErrInvalidAuthorizationPolicy
	}
	parsed, err := casbinmodel.NewModelFromString(modelText)
	if err != nil {
		return nil, authorizationPersistenceError("parse policy model", err)
	}
	adapterClient := adapterent.NewClient(adapterent.Driver(driver))
	loader, err := entadapter.NewAdapterWithClient(
		adapterClient,
		entadapter.SkipSchemaCreate(),
		entadapter.WithContext(ctx),
		entadapter.WithPolicyLimit(maxAuthorizationRules),
	)
	if err != nil {
		return nil, authorizationPersistenceError("initialize policy adapter", err)
	}
	if err := loader.LoadPolicy(parsed); err != nil {
		return nil, authorizationPersistenceError("load rules through Ent adapter", err)
	}
	return authorizationRulesFromModel(parsed)
}

func (repository *AuthorizationPolicyRepository) State(
	ctx context.Context,
) (ports.AuthorizationState, error) {
	if !repository.validContext(ctx) {
		return ports.AuthorizationState{}, ports.ErrInvalidAuthorizationPolicy
	}
	record, err := repository.client.DB(ctx).AuthorizationState.Query().
		Where(authorizationstate.ID(authorizationStateID)).
		Only(ctx)
	if err != nil {
		return ports.AuthorizationState{}, authorizationStateReadError(err)
	}
	return authorizationStateFromRecord(record)
}

func (repository *AuthorizationPolicyRepository) LockState(
	ctx context.Context,
) (ports.AuthorizationState, error) {
	if !repository.validContext(ctx) {
		return ports.AuthorizationState{}, ports.ErrInvalidAuthorizationPolicy
	}
	// DB(ctx) intentionally allows ordinary reads outside a transaction. A row
	// lock does not, so resolve the mandatory platform scope before issuing SQL.
	if _, err := platformtx.Client(ctx, nil); err != nil {
		return ports.AuthorizationState{}, err
	}
	record, err := repository.client.DB(ctx).AuthorizationState.Query().
		Where(authorizationstate.ID(authorizationStateID)).
		ForUpdate().
		Only(ctx)
	if err != nil {
		return ports.AuthorizationState{}, authorizationStateReadError(err)
	}
	return authorizationStateFromRecord(record)
}

func (repository *AuthorizationPolicyRepository) ListRules(
	ctx context.Context,
) ([]authcasbin.PolicyRule, error) {
	if !repository.validContext(ctx) {
		return nil, ports.ErrInvalidAuthorizationPolicy
	}
	records, err := repository.client.DB(ctx).CasbinRule.Query().
		Order(
			casbinrule.ByPtype(),
			casbinrule.ByV0(),
			casbinrule.ByV1(),
			casbinrule.ByV2(),
			casbinrule.ByV3(),
			casbinrule.ByV4(),
			casbinrule.ByV5(),
		).
		Limit(maxAuthorizationRules + 1).
		All(ctx)
	if err != nil {
		return nil, authorizationPersistenceError("list rules", err)
	}
	if len(records) > maxAuthorizationRules {
		return nil, ports.ErrInvalidAuthorizationPolicy
	}

	rules := make([]authcasbin.PolicyRule, len(records))
	seen := make(map[authcasbin.PolicyRule]struct{}, len(records))
	for index, record := range records {
		rule, conversionErr := authorizationPolicyRuleFromRecord(record)
		if conversionErr != nil {
			return nil, conversionErr
		}
		if _, duplicate := seen[rule]; duplicate {
			return nil, ports.ErrInvalidAuthorizationPolicy
		}
		seen[rule] = struct{}{}
		rules[index] = rule
	}
	sortAuthorizationPolicyRules(rules)
	return rules, nil
}

func (repository *AuthorizationPolicyRepository) ReplaceRolePolicies(
	ctx context.Context,
	roleSubject string,
	rules []authcasbin.PolicyRule,
) error {
	if !repository.validContext(ctx) {
		return ports.ErrInvalidAuthorizationPolicy
	}
	normalized, err := normalizeRolePolicyRules(roleSubject, rules)
	if err != nil {
		return err
	}
	client := repository.client.DB(ctx)
	if _, err = client.CasbinRule.Delete().Where(
		casbinrule.PtypeEQ("p"),
		casbinrule.V0EQ(roleSubject),
	).Exec(ctx); err != nil {
		return authorizationPersistenceError("replace role policies", err)
	}
	return createAuthorizationPolicyRules(ctx, client, normalized)
}

func (repository *AuthorizationPolicyRepository) ReplaceUserRoles(
	ctx context.Context,
	userSubject string,
	roleSubjects []string,
) error {
	if !repository.validContext(ctx) {
		return ports.ErrInvalidAuthorizationPolicy
	}
	normalized, err := normalizeUserRoleRules(userSubject, roleSubjects)
	if err != nil {
		return err
	}
	client := repository.client.DB(ctx)
	if _, err = client.CasbinRule.Delete().Where(
		casbinrule.PtypeEQ("g"),
		casbinrule.V0EQ(userSubject),
	).Exec(ctx); err != nil {
		return authorizationPersistenceError("replace user roles", err)
	}
	return createAuthorizationPolicyRules(ctx, client, normalized)
}

func (repository *AuthorizationPolicyRepository) DeleteRoleReferences(
	ctx context.Context,
	roleSubject string,
) error {
	if !repository.validContext(ctx) || !validRoleSubject(roleSubject) {
		return ports.ErrInvalidAuthorizationPolicy
	}
	_, err := repository.client.DB(ctx).CasbinRule.Delete().Where(
		casbinrule.Or(
			casbinrule.And(
				casbinrule.PtypeEQ("p"),
				casbinrule.V0EQ(roleSubject),
			),
			casbinrule.And(
				casbinrule.PtypeEQ("g"),
				casbinrule.Or(
					casbinrule.V0EQ(roleSubject),
					casbinrule.V1EQ(roleSubject),
				),
			),
		),
	).Exec(ctx)
	return authorizationPersistenceError("delete role references", err)
}

func (repository *AuthorizationPolicyRepository) DeleteUserReferences(
	ctx context.Context,
	userSubject string,
) error {
	if !repository.validContext(ctx) || !validUserSubject(userSubject) {
		return ports.ErrInvalidAuthorizationPolicy
	}
	_, err := repository.client.DB(ctx).CasbinRule.Delete().Where(
		casbinrule.PtypeEQ("g"),
		casbinrule.V0EQ(userSubject),
	).Exec(ctx)
	return authorizationPersistenceError("delete user references", err)
}

func (repository *AuthorizationPolicyRepository) AdvanceRevision(
	ctx context.Context,
	expected, next int64,
	updatedBy *uuid.UUID,
) error {
	if !repository.validContext(ctx) || expected <= 0 || next <= expected || next != expected+1 ||
		(updatedBy != nil && *updatedBy == uuid.Nil) {
		return ports.ErrInvalidAuthorizationPolicy
	}
	update := repository.client.DB(ctx).AuthorizationState.Update().
		Where(
			authorizationstate.ID(authorizationStateID),
			authorizationstate.DesiredRevisionEQ(expected),
		).
		SetDesiredRevision(next)
	if updatedBy == nil {
		update.ClearUpdatedBy()
	} else {
		update.SetUpdatedBy(*updatedBy)
	}
	affected, err := update.Save(ctx)
	if err != nil {
		return authorizationPersistenceError("advance revision", err)
	}
	switch affected {
	case 1:
		return nil
	case 0:
		return ports.ErrAuthorizationRevisionConflict
	default:
		return fmt.Errorf("authorization policy persistence: revision update affected %d rows", affected)
	}
}

func (repository *AuthorizationPolicyRepository) ActiveRoleKeys(
	ctx context.Context,
	keys []string,
) (map[string]bool, error) {
	if !repository.validContext(ctx) {
		return nil, ports.ErrInvalidAuthorizationPolicy
	}
	normalized, err := normalizeRoleKeyLookup(keys)
	if err != nil {
		return nil, err
	}
	active := make(map[string]bool, len(normalized))
	if len(normalized) == 0 {
		return active, nil
	}
	var rows []string
	err = repository.client.DB(ctx).Role.Query().
		Where(
			role.KeyIn(normalized...),
			role.StatusEQ(role.StatusActive),
			role.DeletedAtIsNil(),
		).
		Order(role.ByKey()).
		Select(role.FieldKey).
		Scan(ctx, &rows)
	if err != nil {
		return nil, authorizationPersistenceError("list active roles", err)
	}
	for _, key := range rows {
		active[key] = true
	}
	return active, nil
}

func (repository *AuthorizationPolicyRepository) ActiveUserIDs(
	ctx context.Context,
	ids []uuid.UUID,
) (map[uuid.UUID]bool, error) {
	return repository.authorizationUserIDs(
		ctx,
		ids,
		"list active users",
		user.StatusActive,
	)
}

func (repository *AuthorizationPolicyRepository) AuthorizationEligibleUserIDs(
	ctx context.Context,
	ids []uuid.UUID,
) (map[uuid.UUID]bool, error) {
	return repository.authorizationUserIDs(
		ctx,
		ids,
		"list authorization-eligible users",
		user.StatusActive,
		user.StatusInvited,
		user.StatusSuspended,
	)
}

func (repository *AuthorizationPolicyRepository) authorizationUserIDs(
	ctx context.Context,
	ids []uuid.UUID,
	operation string,
	statuses ...user.Status,
) (map[uuid.UUID]bool, error) {
	if !repository.validContext(ctx) {
		return nil, ports.ErrInvalidAuthorizationPolicy
	}
	normalized, err := normalizeUserIDLookup(ids)
	if err != nil {
		return nil, err
	}
	if len(statuses) == 0 {
		return nil, ports.ErrInvalidAuthorizationPolicy
	}
	matched := make(map[uuid.UUID]bool, len(normalized))
	if len(normalized) == 0 {
		return matched, nil
	}
	rows, err := repository.client.DB(ctx).User.Query().
		Where(
			user.IDIn(normalized...),
			user.StatusIn(statuses...),
			user.DeletedAtIsNil(),
		).
		Order(user.ByID()).
		IDs(ctx)
	if err != nil {
		return nil, authorizationPersistenceError(operation, err)
	}
	for _, id := range rows {
		matched[id] = true
	}
	return matched, nil
}

func (repository *AuthorizationPolicyRepository) LoadCommandReceipt(
	ctx context.Context,
	commandID uuid.UUID,
) (ports.AuthorizationCommandReceipt, bool, error) {
	if !repository.validContext(ctx) || commandID == uuid.Nil {
		return ports.AuthorizationCommandReceipt{}, false, ports.ErrInvalidAuthorizationPolicy
	}
	record, err := repository.client.DB(ctx).OutboxEvent.Get(ctx, commandID)
	if generate.IsNotFound(err) {
		return ports.AuthorizationCommandReceipt{}, false, nil
	}
	if err != nil {
		return ports.AuthorizationCommandReceipt{}, false,
			authorizationPersistenceError("load command receipt", err)
	}
	if record == nil {
		return ports.AuthorizationCommandReceipt{}, false, ports.ErrInvalidAuthorizationPolicy
	}
	return ports.AuthorizationCommandReceipt{
		EventID:          record.ID,
		EventType:        record.EventType,
		AggregateID:      record.AggregateID,
		AggregateVersion: record.AggregateVersion,
		Payload:          append([]byte(nil), record.Payload...),
		OccurredAt:       record.OccurredAt.UTC(),
	}, true, nil
}

func (repository *AuthorizationPolicyRepository) validContext(ctx context.Context) bool {
	return repository != nil && repository.client != nil && ctx != nil
}

func authorizationStateReadError(err error) error {
	if generate.IsNotFound(err) {
		return ports.ErrAuthorizationStateNotInitialized
	}
	return authorizationPersistenceError("read state", err)
}

func authorizationStateFromRecord(
	record *generate.AuthorizationState,
) (ports.AuthorizationState, error) {
	if record == nil || record.ID != authorizationStateID || record.DesiredRevision <= 0 ||
		(record.UpdatedBy != nil && *record.UpdatedBy == uuid.Nil) {
		return ports.AuthorizationState{}, ports.ErrInvalidAuthorizationPolicy
	}
	state := ports.AuthorizationState{DesiredRevision: record.DesiredRevision}
	if record.UpdatedBy != nil {
		updatedBy := *record.UpdatedBy
		state.UpdatedBy = &updatedBy
	}
	return state, nil
}

func authorizationPolicyRuleFromRecord(
	record *generate.CasbinRule,
) (authcasbin.PolicyRule, error) {
	if record == nil {
		return authcasbin.PolicyRule{}, ports.ErrInvalidAuthorizationPolicy
	}
	rule := authcasbin.PolicyRule{
		Type: record.Ptype,
		V0:   record.V0,
		V1:   record.V1,
		V2:   record.V2,
		V3:   record.V3,
		V4:   record.V4,
		V5:   record.V5,
	}
	if !validCanonicalAuthorizationRule(rule) {
		return authcasbin.PolicyRule{}, ports.ErrInvalidAuthorizationPolicy
	}
	return rule, nil
}

func authorizationRulesFromModel(parsed casbinmodel.Model) ([]authcasbin.PolicyRule, error) {
	rules := make([]authcasbin.PolicyRule, 0)
	seen := make(map[authcasbin.PolicyRule]struct{})
	for _, section := range []string{"g", "p"} {
		assertions := parsed[section]
		names := make([]string, 0, len(assertions))
		for name := range assertions {
			names = append(names, name)
		}
		slices.Sort(names)
		for _, name := range names {
			for _, values := range assertions[name].Policy {
				if len(values) == 0 || len(values) > 6 || len(rules) >= maxAuthorizationRules {
					return nil, ports.ErrInvalidAuthorizationPolicy
				}
				rule := authcasbin.PolicyRule{Type: name}
				destinations := []*string{&rule.V0, &rule.V1, &rule.V2, &rule.V3, &rule.V4, &rule.V5}
				for index, value := range values {
					*destinations[index] = value
				}
				if !validCanonicalAuthorizationRule(rule) {
					return nil, ports.ErrInvalidAuthorizationPolicy
				}
				if _, duplicate := seen[rule]; duplicate {
					return nil, ports.ErrInvalidAuthorizationPolicy
				}
				seen[rule] = struct{}{}
				rules = append(rules, rule)
			}
		}
	}
	sortAuthorizationPolicyRules(rules)
	return rules, nil
}

func normalizeRolePolicyRules(
	roleSubject string,
	rules []authcasbin.PolicyRule,
) ([]authcasbin.PolicyRule, error) {
	if !validRoleSubject(roleSubject) || len(rules) > maxAuthorizationRulesPerMutation {
		return nil, ports.ErrInvalidAuthorizationPolicy
	}
	normalized := append([]authcasbin.PolicyRule(nil), rules...)
	seen := make(map[authcasbin.PolicyRule]struct{}, len(normalized))
	for _, rule := range normalized {
		if rule.Type != "p" || rule.V0 != roleSubject ||
			!validCanonicalAuthorizationRule(rule) {
			return nil, ports.ErrInvalidAuthorizationPolicy
		}
		if _, duplicate := seen[rule]; duplicate {
			return nil, ports.ErrInvalidAuthorizationPolicy
		}
		seen[rule] = struct{}{}
	}
	sortAuthorizationPolicyRules(normalized)
	return normalized, nil
}

func normalizeUserRoleRules(
	userSubject string,
	roleSubjects []string,
) ([]authcasbin.PolicyRule, error) {
	if !validUserSubject(userSubject) || len(roleSubjects) > maxAuthorizationRulesPerMutation {
		return nil, ports.ErrInvalidAuthorizationPolicy
	}
	normalizedSubjects := append([]string(nil), roleSubjects...)
	seen := make(map[string]struct{}, len(normalizedSubjects))
	for _, roleSubject := range normalizedSubjects {
		if !validRoleSubject(roleSubject) {
			return nil, ports.ErrInvalidAuthorizationPolicy
		}
		if _, duplicate := seen[roleSubject]; duplicate {
			return nil, ports.ErrInvalidAuthorizationPolicy
		}
		seen[roleSubject] = struct{}{}
	}
	slices.Sort(normalizedSubjects)
	rules := make([]authcasbin.PolicyRule, len(normalizedSubjects))
	for index, roleSubject := range normalizedSubjects {
		rules[index] = authcasbin.PolicyRule{
			Type: "g",
			V0:   userSubject,
			V1:   roleSubject,
		}
	}
	return rules, nil
}

func validCanonicalAuthorizationRule(rule authcasbin.PolicyRule) bool {
	if len(rule.Type) == 0 || len(rule.Type) > maxAuthorizationPolicyTypeBytes {
		return false
	}
	switch rule.Type {
	case "p":
		return validRoleSubject(rule.V0) &&
			validAuthorizationPolicyValue(rule.V1) &&
			validAuthorizationPolicyValue(rule.V2) &&
			rule.V3 == "" && rule.V4 == "" && rule.V5 == ""
	case "g":
		return validUserSubject(rule.V0) && validRoleSubject(rule.V1) &&
			rule.V2 == "" && rule.V3 == "" && rule.V4 == "" && rule.V5 == ""
	default:
		return false
	}
}

func validAuthorizationPolicyValue(value string) bool {
	if value == "" || len(value) > maxAuthorizationPolicyValueBytes ||
		!utf8.ValidString(value) || strings.TrimSpace(value) != value {
		return false
	}
	for _, character := range value {
		if unicode.IsControl(character) {
			return false
		}
	}
	return true
}

func validRoleSubject(value string) bool {
	if !strings.HasPrefix(value, "role:") {
		return false
	}
	return validAuthorizationRoleKey(strings.TrimPrefix(value, "role:"))
}

func validAuthorizationRoleKey(value string) bool {
	if value == "" || len(value) > maxAuthorizationRoleKeyBytes {
		return false
	}
	for index, character := range value {
		if character >= 'a' && character <= 'z' ||
			index > 0 && character >= '0' && character <= '9' ||
			index > 0 && (character == '_' || character == '-') {
			continue
		}
		return false
	}
	return true
}

func validUserSubject(value string) bool {
	if !strings.HasPrefix(value, "user:") {
		return false
	}
	rawID := strings.TrimPrefix(value, "user:")
	id, err := uuid.Parse(rawID)
	return err == nil && id != uuid.Nil && id.String() == rawID
}

func sortAuthorizationPolicyRules(rules []authcasbin.PolicyRule) {
	slices.SortFunc(rules, func(left, right authcasbin.PolicyRule) int {
		leftValues := [...]string{left.Type, left.V0, left.V1, left.V2, left.V3, left.V4, left.V5}
		rightValues := [...]string{right.Type, right.V0, right.V1, right.V2, right.V3, right.V4, right.V5}
		for index := range leftValues {
			if compared := strings.Compare(leftValues[index], rightValues[index]); compared != 0 {
				return compared
			}
		}
		return 0
	})
}

func createAuthorizationPolicyRules(
	ctx context.Context,
	client *generate.Client,
	rules []authcasbin.PolicyRule,
) error {
	if len(rules) == 0 {
		return nil
	}
	builders := make([]*generate.CasbinRuleCreate, len(rules))
	for index, rule := range rules {
		builders[index] = client.CasbinRule.Create().
			SetPtype(rule.Type).
			SetV0(rule.V0).
			SetV1(rule.V1).
			SetV2(rule.V2).
			SetV3(rule.V3).
			SetV4(rule.V4).
			SetV5(rule.V5)
	}
	_, err := client.CasbinRule.CreateBulk(builders...).Save(ctx)
	return authorizationPersistenceError("bulk create rules", err)
}

func normalizeRoleKeyLookup(keys []string) ([]string, error) {
	if len(keys) > maxAuthorizationReferenceLookupItems {
		return nil, ports.ErrInvalidAuthorizationPolicy
	}
	seen := make(map[string]struct{}, len(keys))
	normalized := make([]string, 0, len(keys))
	for _, key := range keys {
		if !validAuthorizationRoleKey(key) {
			return nil, ports.ErrInvalidAuthorizationPolicy
		}
		if _, duplicate := seen[key]; duplicate {
			continue
		}
		seen[key] = struct{}{}
		normalized = append(normalized, key)
	}
	slices.Sort(normalized)
	return normalized, nil
}

func normalizeUserIDLookup(ids []uuid.UUID) ([]uuid.UUID, error) {
	if len(ids) > maxAuthorizationReferenceLookupItems {
		return nil, ports.ErrInvalidAuthorizationPolicy
	}
	seen := make(map[uuid.UUID]struct{}, len(ids))
	normalized := make([]uuid.UUID, 0, len(ids))
	for _, id := range ids {
		if id == uuid.Nil {
			return nil, ports.ErrInvalidAuthorizationPolicy
		}
		if _, duplicate := seen[id]; duplicate {
			continue
		}
		seen[id] = struct{}{}
		normalized = append(normalized, id)
	}
	slices.SortFunc(normalized, func(left, right uuid.UUID) int {
		return strings.Compare(left.String(), right.String())
	})
	return normalized, nil
}

func authorizationPersistenceError(operation string, err error) error {
	if err == nil {
		return nil
	}
	// operation is a closed, code-owned label. No policy values or user
	// subjects are ever interpolated into repository diagnostics, and Error does
	// not render driver text that could contain rejected constraint values.
	return &authorizationPersistenceFailure{operation: operation, cause: err}
}

type authorizationPersistenceFailure struct {
	operation string
	cause     error
}

func (failure *authorizationPersistenceFailure) Error() string {
	return "authorization policy persistence " + failure.operation + " failed"
}

func (failure *authorizationPersistenceFailure) Unwrap() error {
	return failure.cause
}

var _ ports.AuthorizationPolicyRepository = (*AuthorizationPolicyRepository)(nil)
