package db

import (
	"context"
	"crypto/subtle"
	"errors"

	"github.com/google/uuid"

	commonEnt "github.com/huynhanx03/go-common/pkg/database/ent"
	d "github.com/huynhanx03/go-common/pkg/dto"

	dbEnt "github.com/huynhanx03/judgify/internal/ent"

	"entgo.io/ent/dialect/sql"

	"github.com/huynhanx03/judgify/internal/ent/generate"
	"github.com/huynhanx03/judgify/internal/ent/generate/credential"
	"github.com/huynhanx03/judgify/internal/ent/generate/user"
	entity "github.com/huynhanx03/judgify/internal/identity/domain"
	platformtx "github.com/huynhanx03/judgify/internal/platform/transaction"
	"github.com/huynhanx03/judgify/internal/ports"
)

const (
	credentialRepoName     = "Credential"
	credentialTypePassword = "password"
	credentialHashKey      = "hash"
)

type CredentialRepository struct {
	client *dbEnt.EntClient
}

func NewCredentialRepository(client *dbEnt.EntClient) ports.CredentialRepository {
	return &CredentialRepository{client: client}
}

func (r *CredentialRepository) Find(ctx context.Context, opts *d.QueryOptions) (*d.Paginated[*entity.Credential], error) {
	client := r.client.DB(ctx)

	query := client.Credential.Query()
	query.Where(func(s *sql.Selector) {
		commonEnt.ApplyQueryFilters(opts, s, credentialQueryColumns...)
	})

	total, err := query.Clone().Count(ctx)
	if err != nil {
		return nil, commonEnt.MapEntError(err, credentialRepoName)
	}

	query.Where(func(s *sql.Selector) {
		commonEnt.ApplyQueryWindow(opts, s, credentialQueryColumns...)
	})

	records, err := query.All(ctx)
	if err != nil {
		return nil, commonEnt.MapEntError(err, credentialRepoName)
	}

	entities := make([]*entity.Credential, len(records))
	for i, record := range records {
		entities[i] = ToCredentialEntity(record)
	}

	paginationOpts := &d.PaginationOptions{}
	if opts != nil && opts.Pagination != nil {
		paginationOpts = opts.Pagination
	} else {
		paginationOpts.SetDefaults()
	}

	meta := d.CalculatePagination(
		paginationOpts.Page,
		paginationOpts.PageSize,
		int64(total),
	)

	return &d.Paginated[*entity.Credential]{
		Records:    &entities,
		Pagination: meta,
	}, nil
}

func (r *CredentialRepository) Get(ctx context.Context, id uuid.UUID) (*entity.Credential, error) {
	record, err := r.client.DB(ctx).Credential.Get(ctx, id)
	if err != nil {
		return nil, commonEnt.MapEntError(err, credentialRepoName)
	}
	return ToCredentialEntity(record), nil
}

func (r *CredentialRepository) GetByUserID(ctx context.Context, userID uuid.UUID, credType string) (*entity.Credential, error) {
	record, err := r.client.DB(ctx).Credential.Query().
		Where(
			credential.UserID(userID),
			credential.Type(credType),
			credential.ActiveEQ(true),
			credential.DeletedAtIsNil(),
		).
		Only(ctx)
	if err != nil {
		return nil, commonEnt.MapEntError(err, credentialRepoName)
	}
	return ToCredentialEntity(record), nil
}

func (r *CredentialRepository) Create(ctx context.Context, e *entity.Credential) error {
	create := BuildCreateCredential(r.client.DB(ctx), e)
	record, err := create.Save(ctx)
	if err != nil {
		return commonEnt.MapEntError(err, credentialRepoName)
	}

	if created := ToCredentialEntity(record); created != nil {
		*e = *created
	}
	return nil
}

func (r *CredentialRepository) Update(ctx context.Context, e *entity.Credential) error {
	update := BuildUpdateCredential(r.client.DB(ctx), e)
	record, err := update.Save(ctx)
	if err != nil {
		return commonEnt.MapEntError(err, credentialRepoName)
	}
	e.UpdatedAt = record.UpdatedAt
	return nil
}

func (r *CredentialRepository) RotatePassword(
	ctx context.Context,
	rotation ports.PasswordCredentialRotation,
) (uint64, error) {
	if r == nil || r.client == nil || ctx == nil || ctx.Err() != nil ||
		rotation.UserID == uuid.Nil || rotation.CredentialID == uuid.Nil ||
		rotation.ExpectedHash == "" || rotation.Replacement == "" {
		return 0, ports.ErrCredentialAuthorityLost
	}
	if _, err := platformtx.Client(ctx, nil); err != nil {
		return 0, errors.Join(ports.ErrCredentialAuthorityLost, platformtx.ErrTransactionRequired)
	}
	client := r.client.DB(ctx)
	owner, err := client.User.Query().Where(
		user.ID(rotation.UserID),
		user.StatusEQ(user.StatusActive),
		user.DeletedAtIsNil(),
	).Select(user.FieldID, user.FieldSecurityVersion).ForUpdate().Only(ctx)
	if err != nil {
		if generate.IsNotFound(err) {
			return 0, ports.ErrCredentialAuthorityLost
		}
		return 0, commonEnt.MapEntError(err, credentialRepoName)
	}
	if owner.SecurityVersion < 1 {
		return 0, ports.ErrCredentialAuthorityLost
	}
	record, err := client.Credential.Query().Where(
		credential.ID(rotation.CredentialID),
		credential.UserID(rotation.UserID),
		credential.Type(credentialTypePassword),
		credential.ActiveEQ(true),
		credential.DeletedAtIsNil(),
	).Select(
		credential.FieldID,
		credential.FieldCredentialData,
		credential.FieldVersion,
	).ForUpdate().Only(ctx)
	if err != nil {
		if generate.IsNotFound(err) {
			return 0, ports.ErrCredentialAuthorityLost
		}
		return 0, commonEnt.MapEntError(err, credentialRepoName)
	}
	currentHash, ok := record.CredentialData[credentialHashKey].(string)
	if !ok || currentHash == "" ||
		subtle.ConstantTimeCompare([]byte(currentHash), []byte(rotation.ExpectedHash)) != 1 {
		return 0, ports.ErrCredentialConflict
	}
	if record.Version < 1 {
		return 0, ports.ErrCredentialAuthorityLost
	}
	replacementData := make(map[string]any, len(record.CredentialData))
	for key, value := range record.CredentialData {
		replacementData[key] = value
	}
	replacementData[credentialHashKey] = rotation.Replacement
	updatedCredential, err := client.Credential.UpdateOneID(record.ID).
		Where(credential.VersionEQ(record.Version), credential.ActiveEQ(true), credential.DeletedAtIsNil()).
		SetCredentialData(replacementData).
		SetVersion(record.Version + 1).
		Save(ctx)
	if err != nil {
		return 0, commonEnt.MapEntError(err, credentialRepoName)
	}
	if updatedCredential == nil || updatedCredential.Version != record.Version+1 {
		return 0, ports.ErrCredentialConflict
	}
	securityVersion := owner.SecurityVersion + 1
	updatedOwner, err := client.User.UpdateOneID(owner.ID).
		Where(user.SecurityVersionEQ(owner.SecurityVersion), user.StatusEQ(user.StatusActive), user.DeletedAtIsNil()).
		SetSecurityVersion(securityVersion).
		Save(ctx)
	if err != nil {
		return 0, commonEnt.MapEntError(err, credentialRepoName)
	}
	if updatedOwner == nil || updatedOwner.SecurityVersion != securityVersion {
		return 0, ports.ErrCredentialConflict
	}
	return uint64(securityVersion), nil
}

func (r *CredentialRepository) Delete(ctx context.Context, id uuid.UUID) error {
	if err := r.client.DB(ctx).Credential.DeleteOneID(id).Exec(ctx); err != nil {
		return commonEnt.MapEntError(err, credentialRepoName)
	}
	return nil
}

func (r *CredentialRepository) Exists(ctx context.Context, id uuid.UUID) (bool, error) {
	exists, err := r.client.DB(ctx).Credential.Query().Where(credential.ID(id)).Exist(ctx)

	if err != nil {
		return false, commonEnt.MapEntError(err, credentialRepoName)
	}

	return exists, nil
}
