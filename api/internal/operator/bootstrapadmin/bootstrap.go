package bootstrapadmin

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/huynhanx03/go-common/pkg/correlation"
	commonent "github.com/huynhanx03/go-common/pkg/database/ent"
	commonpassword "github.com/huynhanx03/go-common/pkg/security/password"

	identityconstant "github.com/huynhanx03/judgify/internal/constant"
	"github.com/huynhanx03/judgify/internal/ent/generate"
	"github.com/huynhanx03/judgify/internal/ent/generate/attributedefinition"
	"github.com/huynhanx03/judgify/internal/ent/generate/attributedefinitionrevision"
	"github.com/huynhanx03/judgify/internal/ent/generate/role"
	"github.com/huynhanx03/judgify/internal/ent/generate/user"
	"github.com/huynhanx03/judgify/internal/platform/audit"
	platformtx "github.com/huynhanx03/judgify/internal/platform/transaction"
	identityports "github.com/huynhanx03/judgify/internal/ports"
)

const (
	protectedRoleKey       = identityconstant.ProtectedSuperadminRoleKey
	defaultStudentRoleKey  = identityconstant.DefaultUserRoleKey
	authorizationStateID   = "default"
	authorizationRevision  = int64(1)
	profileSchemaRevision  = int64(1)
	passwordCredentialType = "password"
	passwordHashKey        = "hash"
)

var ErrAlreadyInitialized = errors.New("bootstrap-admin: database is already initialized or inconsistent")

// Bootstrap creates the only initial privileged account in a fresh,
// migration-owned database. It never repairs, overwrites, or merges state:
// any existing authorization data causes the whole transaction to roll back.
func Bootstrap(ctx context.Context, client *generate.Client, input Input) (uuid.UUID, error) {
	defer input.Clear()
	if client == nil || ctx == nil || !validUsername(input.Username) || !validPassword(input.Password) {
		return uuid.Nil, ErrInvalidInput
	}

	passwordHasher, err := commonpassword.New(commonpassword.DefaultPolicy())
	if err != nil {
		return uuid.Nil, fmt.Errorf("bootstrap-admin: configure password hasher: %w", err)
	}
	passwordHash, err := passwordHasher.Hash(ctx, input.Password)
	if err != nil {
		return uuid.Nil, fmt.Errorf("bootstrap-admin: hash password: %w", err)
	}

	ctx = correlation.EnsureContext(ctx)
	var administratorID uuid.UUID
	err = platformtx.NewEntUnitOfWork(client).Within(ctx, func(txCtx context.Context) error {
		txClient, txErr := platformtx.Client(txCtx, nil)
		if txErr != nil {
			return txErr
		}
		state, stateErr := readDatabaseState(commonent.SkipSoftDelete(txCtx), txClient)
		if stateErr != nil {
			return stateErr
		}
		if stateErr = validateFreshState(state); stateErr != nil {
			return stateErr
		}

		if len(state.roles) == 0 {
			if createErr := createStudentRole(txCtx, txClient); createErr != nil {
				return createErr
			}
		}
		if createErr := createProtectedRole(txCtx, txClient); createErr != nil {
			return createErr
		}
		if createErr := createRequiredSystemAttributes(txCtx, txClient); createErr != nil {
			return createErr
		}
		if createErr := createProfileSchemaState(txCtx, txClient); createErr != nil {
			return createErr
		}

		var createErr error
		administratorID, createErr = uuid.NewV7()
		if createErr != nil {
			return fmt.Errorf("bootstrap-admin: allocate administrator ID: %w", createErr)
		}
		if createErr = txClient.User.Create().
			SetID(administratorID).
			SetUsername(input.Username).
			SetStatus(user.StatusActive).
			SetSecurityVersion(1).
			Exec(txCtx); createErr != nil {
			return fmt.Errorf("bootstrap-admin: create administrator: %w", createErr)
		}
		if createErr = txClient.Credential.Create().
			SetUserID(administratorID).
			SetType(passwordCredentialType).
			SetCredentialData(map[string]any{passwordHashKey: passwordHash}).
			SetVersion(1).
			SetActive(true).
			Exec(txCtx); createErr != nil {
			return fmt.Errorf("bootstrap-admin: create password credential: %w", createErr)
		}

		if createErr = txClient.CasbinRule.Create().
			SetPtype("g").
			SetV0("user:" + administratorID.String()).
			SetV1("role:" + protectedRoleKey).
			Exec(txCtx); createErr != nil {
			return fmt.Errorf("bootstrap-admin: assign administrator role: %w", createErr)
		}
		if createErr = txClient.CasbinRule.Create().
			SetPtype("p").
			SetV0("role:" + protectedRoleKey).
			SetV1("*").
			SetV2("*").
			Exec(txCtx); createErr != nil {
			return fmt.Errorf("bootstrap-admin: create administrator policy: %w", createErr)
		}
		if createErr = txClient.AuthorizationState.Create().
			SetID(authorizationStateID).
			SetDesiredRevision(authorizationRevision).
			SetUpdatedBy(administratorID).
			Exec(txCtx); createErr != nil {
			return fmt.Errorf("bootstrap-admin: create authorization state: %w", createErr)
		}

		_, createErr = audit.NewRepository().Append(txCtx, audit.Entry{
			ActorUserID: &administratorID,
			Action:      "bootstrap_admin.initialize",
			Resource:    "authorization",
			ResourceID:  authorizationStateID,
			Reason:      "initial production administrator bootstrap",
			Before:      map[string]any{},
			After: map[string]any{
				"administrator_user_id":   administratorID.String(),
				"authorization_revision":  authorizationRevision,
				"roles":                   []string{protectedRoleKey, defaultStudentRoleKey},
				"system_attributes":       systemAttributeKeys(),
				"profile_schema_revision": profileSchemaRevision,
			},
		})
		if createErr != nil {
			return fmt.Errorf("bootstrap-admin: append audit record: %w", createErr)
		}
		return nil
	})
	if err != nil {
		return uuid.Nil, err
	}
	return administratorID, nil
}

func createProfileSchemaState(ctx context.Context, client *generate.Client) error {
	stateID, err := uuid.NewV7()
	if err != nil {
		return fmt.Errorf("bootstrap-admin: allocate profile schema state: %w", err)
	}
	if err := client.ProfileSchemaState.Create().
		SetID(stateID).
		SetKey(identityports.ProfileSchemaStateKey).
		SetRevision(profileSchemaRevision).
		Exec(ctx); err != nil {
		return fmt.Errorf("bootstrap-admin: create profile schema state: %w", err)
	}
	return nil
}

func createRequiredSystemAttributes(ctx context.Context, client *generate.Client) error {
	for _, spec := range identityconstant.RequiredSystemAttributeCatalog() {
		definitionID, err := uuid.NewV7()
		if err != nil {
			return fmt.Errorf("bootstrap-admin: allocate system attribute %s: %w", spec.Key, err)
		}
		revisionID, err := uuid.NewV7()
		if err != nil {
			return fmt.Errorf("bootstrap-admin: allocate system attribute revision %s: %w", spec.Key, err)
		}
		if err = client.AttributeDefinition.Create().
			SetID(definitionID).
			SetKey(spec.Key).
			SetKind(attributedefinition.KindSystem).
			SetRequiredOnOnboarding(true).
			SetStatus(attributedefinition.StatusActive).
			SetVersion(1).
			Exec(ctx); err != nil {
			return fmt.Errorf("bootstrap-admin: create system attribute %s: %w", spec.Key, err)
		}
		if err = client.AttributeDefinitionRevision.Create().
			SetID(revisionID).
			SetDefinitionID(definitionID).
			SetRevision(1).
			SetDataType(attributedefinitionrevision.DataType(spec.DataType)).
			SetLabel(spec.Label).
			SetDescription(spec.Description).
			SetVisibility(attributedefinitionrevision.VisibilityPrivate).
			SetUserEditable(true).
			SetDisplayOrder(spec.DisplayOrder).
			SetValidation([]byte(spec.Validation)).
			SetCreatedAt(time.Now().UTC()).
			Exec(ctx); err != nil {
			return fmt.Errorf("bootstrap-admin: create system attribute revision %s: %w", spec.Key, err)
		}
		if err = client.AttributeDefinition.UpdateOneID(definitionID).
			SetDraftRevisionID(revisionID).
			SetActiveRevisionID(revisionID).
			Exec(ctx); err != nil {
			return fmt.Errorf("bootstrap-admin: activate system attribute %s: %w", spec.Key, err)
		}
	}
	return nil
}

func systemAttributeKeys() []string {
	catalog := identityconstant.RequiredSystemAttributeCatalog()
	keys := make([]string, len(catalog))
	for index, spec := range catalog {
		keys[index] = spec.Key
	}
	return keys
}

type databaseState struct {
	authorizationStates  int
	users                int
	credentials          int
	casbinRules          int
	audits               int
	attributeDefinitions int
	attributeRevisions   int
	attributeValues      int
	profileSchemaStates  int
	roles                []roleState
}

type roleState struct {
	key     string
	system  bool
	active  bool
	version int64
	deleted bool
}

func readDatabaseState(ctx context.Context, client *generate.Client) (databaseState, error) {
	if client == nil {
		return databaseState{}, ErrInvalidInput
	}
	state := databaseState{}
	var err error
	if state.authorizationStates, err = client.AuthorizationState.Query().Count(ctx); err != nil {
		return databaseState{}, fmt.Errorf("bootstrap-admin: inspect authorization state: %w", err)
	}
	if state.authorizationStates != 0 {
		return databaseState{}, ErrAlreadyInitialized
	}
	if state.users, err = client.User.Query().Count(ctx); err != nil {
		return databaseState{}, fmt.Errorf("bootstrap-admin: inspect users: %w", err)
	}
	if state.users != 0 {
		return databaseState{}, ErrAlreadyInitialized
	}
	if state.credentials, err = client.Credential.Query().Count(ctx); err != nil {
		return databaseState{}, fmt.Errorf("bootstrap-admin: inspect credentials: %w", err)
	}
	if state.credentials != 0 {
		return databaseState{}, ErrAlreadyInitialized
	}
	if state.casbinRules, err = client.CasbinRule.Query().Count(ctx); err != nil {
		return databaseState{}, fmt.Errorf("bootstrap-admin: inspect authorization policies: %w", err)
	}
	if state.casbinRules != 0 {
		return databaseState{}, ErrAlreadyInitialized
	}
	if state.audits, err = client.AuditLog.Query().Count(ctx); err != nil {
		return databaseState{}, fmt.Errorf("bootstrap-admin: inspect audit records: %w", err)
	}
	if state.audits != 0 {
		return databaseState{}, ErrAlreadyInitialized
	}
	if state.attributeDefinitions, err = client.AttributeDefinition.Query().Count(ctx); err != nil {
		return databaseState{}, fmt.Errorf("bootstrap-admin: inspect attribute definitions: %w", err)
	}
	if state.attributeDefinitions != 0 {
		return databaseState{}, ErrAlreadyInitialized
	}
	if state.attributeRevisions, err = client.AttributeDefinitionRevision.Query().Count(ctx); err != nil {
		return databaseState{}, fmt.Errorf("bootstrap-admin: inspect attribute revisions: %w", err)
	}
	if state.attributeRevisions != 0 {
		return databaseState{}, ErrAlreadyInitialized
	}
	if state.attributeValues, err = client.UserAttributeValue.Query().Count(ctx); err != nil {
		return databaseState{}, fmt.Errorf("bootstrap-admin: inspect attribute values: %w", err)
	}
	if state.attributeValues != 0 {
		return databaseState{}, ErrAlreadyInitialized
	}
	if state.profileSchemaStates, err = client.ProfileSchemaState.Query().Count(ctx); err != nil {
		return databaseState{}, fmt.Errorf("bootstrap-admin: inspect profile schema state: %w", err)
	}
	if state.profileSchemaStates != 0 {
		return databaseState{}, ErrAlreadyInitialized
	}
	records, err := client.Role.Query().All(ctx)
	if err != nil {
		return databaseState{}, fmt.Errorf("bootstrap-admin: inspect roles: %w", err)
	}
	state.roles = make([]roleState, 0, len(records))
	for _, record := range records {
		if record == nil {
			return databaseState{}, ErrAlreadyInitialized
		}
		state.roles = append(state.roles, roleState{
			key:     record.Key,
			system:  record.IsSystem,
			active:  record.Status == role.StatusActive,
			version: record.Version,
			deleted: record.DeletedAt != nil,
		})
	}
	return state, nil
}

func validateFreshState(state databaseState) error {
	if state.authorizationStates != 0 || state.users != 0 || state.credentials != 0 ||
		state.casbinRules != 0 || state.audits != 0 ||
		state.attributeDefinitions != 0 || state.attributeRevisions != 0 ||
		state.attributeValues != 0 || state.profileSchemaStates != 0 {
		return ErrAlreadyInitialized
	}
	if len(state.roles) == 0 {
		return nil
	}
	if len(state.roles) != 1 {
		return ErrAlreadyInitialized
	}
	student := state.roles[0]
	if student.key != defaultStudentRoleKey || !student.system || !student.active || student.version != 1 || student.deleted {
		return ErrAlreadyInitialized
	}
	return nil
}

func createStudentRole(ctx context.Context, client *generate.Client) error {
	if err := client.Role.Create().
		SetKey(defaultStudentRoleKey).
		SetName("Student").
		SetDescription("Default learner role").
		SetIsSystem(true).
		SetStatus(role.StatusActive).
		SetVersion(1).
		Exec(ctx); err != nil {
		return fmt.Errorf("bootstrap-admin: create default student role: %w", err)
	}
	return nil
}

func createProtectedRole(ctx context.Context, client *generate.Client) error {
	if err := client.Role.Create().
		SetKey(protectedRoleKey).
		SetName("Super Administrator").
		SetDescription("Protected break-glass administrator role").
		SetIsSystem(true).
		SetStatus(role.StatusActive).
		SetVersion(1).
		Exec(ctx); err != nil {
		return fmt.Errorf("bootstrap-admin: create protected administrator role: %w", err)
	}
	return nil
}
