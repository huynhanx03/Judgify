package db

import (
	"context"
	"time"

	"github.com/google/uuid"

	commonEnt "github.com/huynhanx03/go-common/pkg/database/ent"
	d "github.com/huynhanx03/go-common/pkg/dto"

	dbEnt "github.com/huynhanx03/judgify/internal/ent"

	"entgo.io/ent/dialect/sql"

	"github.com/huynhanx03/judgify/internal/ent/generate"
	"github.com/huynhanx03/judgify/internal/ent/generate/authsession"
	"github.com/huynhanx03/judgify/internal/ent/generate/credential"
	"github.com/huynhanx03/judgify/internal/ent/generate/federatedidentity"
	"github.com/huynhanx03/judgify/internal/ent/generate/user"
	entity "github.com/huynhanx03/judgify/internal/identity/domain"
	platformtx "github.com/huynhanx03/judgify/internal/platform/transaction"
	"github.com/huynhanx03/judgify/internal/ports"
)

const fedIdentityRepoName = "Federated Identity"

type FederatedIdentityRepository struct {
	client *dbEnt.EntClient
}

func NewFederatedIdentityRepository(client *dbEnt.EntClient) ports.FederatedIdentityRepository {
	return &FederatedIdentityRepository{client: client}
}

func (r *FederatedIdentityRepository) Find(ctx context.Context, opts *d.QueryOptions) (*d.Paginated[*entity.FederatedIdentity], error) {
	client := r.client.DB(ctx)

	query := client.FederatedIdentity.Query()
	query.Where(func(s *sql.Selector) {
		commonEnt.ApplyQueryFilters(opts, s, federatedIdentityQueryColumns...)
	})

	total, err := query.Clone().Count(ctx)
	if err != nil {
		return nil, commonEnt.MapEntError(err, fedIdentityRepoName)
	}

	query.Where(func(s *sql.Selector) {
		commonEnt.ApplyQueryWindow(opts, s, federatedIdentityQueryColumns...)
	})

	records, err := query.All(ctx)
	if err != nil {
		return nil, commonEnt.MapEntError(err, fedIdentityRepoName)
	}

	entities := make([]*entity.FederatedIdentity, len(records))
	for i, record := range records {
		entities[i] = ToFederatedIdentityEntity(record)
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

	return &d.Paginated[*entity.FederatedIdentity]{
		Records:    &entities,
		Pagination: meta,
	}, nil
}

func (r *FederatedIdentityRepository) Get(ctx context.Context, id uuid.UUID) (*entity.FederatedIdentity, error) {
	record, err := r.client.DB(ctx).FederatedIdentity.Get(ctx, id)
	if err != nil {
		return nil, commonEnt.MapEntError(err, fedIdentityRepoName)
	}
	return ToFederatedIdentityEntity(record), nil
}

// GetByProviderAndSubject finds the identity by the provider's immutable subject.
func (r *FederatedIdentityRepository) GetByProviderAndSubject(ctx context.Context, provider, providerSubject string) (*entity.FederatedIdentity, error) {
	record, err := r.client.DB(ctx).FederatedIdentity.Query().
		Where(
			federatedidentity.Provider(provider),
			federatedidentity.ProviderSubject(providerSubject),
		).Only(ctx)
	if err != nil {
		return nil, commonEnt.MapEntError(err, fedIdentityRepoName)
	}
	return ToFederatedIdentityEntity(record), nil
}

func (r *FederatedIdentityRepository) GetByUserAndProvider(ctx context.Context, userID uuid.UUID, provider string) (*entity.FederatedIdentity, error) {
	record, err := r.client.DB(ctx).FederatedIdentity.Query().Where(
		federatedidentity.UserID(userID),
		federatedidentity.Provider(provider),
		federatedidentity.DeletedAtIsNil(),
	).Only(ctx)
	if err != nil {
		if generate.IsNotFound(err) {
			return nil, ports.ErrFederatedIdentityNotFound
		}
		return nil, commonEnt.MapEntError(err, fedIdentityRepoName)
	}
	return ToFederatedIdentityEntity(record), nil
}

func (r *FederatedIdentityRepository) ListByUser(ctx context.Context, userID uuid.UUID) ([]*entity.FederatedIdentity, error) {
	records, err := r.client.DB(ctx).FederatedIdentity.Query().Where(
		federatedidentity.UserID(userID),
		federatedidentity.DeletedAtIsNil(),
	).Order(federatedidentity.ByProvider(), federatedidentity.ByID()).All(ctx)
	if err != nil {
		return nil, commonEnt.MapEntError(err, fedIdentityRepoName)
	}
	result := make([]*entity.FederatedIdentity, len(records))
	for index, record := range records {
		result[index] = ToFederatedIdentityEntity(record)
	}
	return result, nil
}

func (r *FederatedIdentityRepository) Create(ctx context.Context, e *entity.FederatedIdentity) error {
	create := BuildCreateFederatedIdentity(r.client.DB(ctx), e)
	record, err := create.Save(ctx)
	if err != nil {
		return commonEnt.MapEntError(err, fedIdentityRepoName)
	}

	if created := ToFederatedIdentityEntity(record); created != nil {
		*e = *created
	}
	return nil
}

func (r *FederatedIdentityRepository) RecordAuthentication(ctx context.Context, e *entity.FederatedIdentity, at time.Time) error {
	if r == nil || r.client == nil || ctx == nil || e == nil || e.ID == uuid.Nil ||
		e.UserID == uuid.Nil || e.Provider == "" || at.IsZero() {
		return ports.ErrFederatedIdentityNotFound
	}
	record, err := r.client.DB(ctx).FederatedIdentity.UpdateOneID(e.ID).Where(
		federatedidentity.UserID(e.UserID),
		federatedidentity.Provider(e.Provider),
		federatedidentity.DeletedAtIsNil(),
	).SetNillableDisplayName(e.DisplayName).
		SetNillableAvatarURL(e.AvatarURL).
		SetLastAuthenticatedAt(at.UTC()).
		Save(ctx)
	if err != nil {
		if generate.IsNotFound(err) {
			return ports.ErrFederatedIdentityNotFound
		}
		return commonEnt.MapEntError(err, fedIdentityRepoName)
	}
	*e = *ToFederatedIdentityEntity(record)
	return nil
}

func (r *FederatedIdentityRepository) Unlink(
	ctx context.Context,
	userID, sessionID uuid.UUID,
	provider string,
	at time.Time,
) (*entity.FederatedIdentity, error) {
	if r == nil || r.client == nil || ctx == nil || userID == uuid.Nil ||
		sessionID == uuid.Nil || provider == "" || at.IsZero() {
		return nil, ports.ErrFederatedIdentityNotFound
	}
	if _, err := platformtx.Client(ctx, nil); err != nil {
		return nil, err
	}
	client := r.client.DB(ctx)
	owner, err := client.User.Query().Where(
		user.ID(userID), user.StatusEQ(user.StatusActive), user.DeletedAtIsNil(),
	).ForUpdate().Only(ctx)
	if err != nil {
		if generate.IsNotFound(err) {
			return nil, ports.ErrFederatedIdentityNotFound
		}
		return nil, commonEnt.MapEntError(err, fedIdentityRepoName)
	}
	session, err := client.AuthSession.Query().Where(
		authsession.ID(sessionID), authsession.UserID(userID),
		authsession.StatusEQ(authsession.StatusActive),
	).ForUpdate().Only(ctx)
	if err != nil {
		if generate.IsNotFound(err) {
			return nil, ports.ErrFederatedIdentityNotFound
		}
		return nil, commonEnt.MapEntError(err, fedIdentityRepoName)
	}
	if !session.IdleExpiresAt.After(at) || !session.AbsoluteExpiresAt.After(at) ||
		session.UserSecurityVersion != owner.SecurityVersion {
		return nil, ports.ErrFederatedIdentityNotFound
	}
	identities, err := client.FederatedIdentity.Query().Where(
		federatedidentity.UserID(userID), federatedidentity.DeletedAtIsNil(),
	).Order(federatedidentity.ByProvider(), federatedidentity.ByID()).ForUpdate().All(ctx)
	if err != nil {
		return nil, commonEnt.MapEntError(err, fedIdentityRepoName)
	}
	var target *generate.FederatedIdentity
	for _, identity := range identities {
		if identity.Provider == provider {
			target = identity
			break
		}
	}
	if target == nil {
		return nil, ports.ErrFederatedIdentityNotFound
	}
	passwordExists, err := client.Credential.Query().Where(
		credential.UserID(userID), credential.TypeEQ("password"),
		credential.ActiveEQ(true), credential.DeletedAtIsNil(),
	).ForShare().Exist(ctx)
	if err != nil {
		return nil, commonEnt.MapEntError(err, fedIdentityRepoName)
	}
	if !passwordExists && len(identities) <= 1 {
		return nil, ports.ErrLastAuthenticationMethod
	}
	if err := client.FederatedIdentity.DeleteOneID(target.ID).Exec(commonEnt.SkipSoftDelete(ctx)); err != nil {
		return nil, commonEnt.MapEntError(err, fedIdentityRepoName)
	}
	return ToFederatedIdentityEntity(target), nil
}

func (r *FederatedIdentityRepository) Delete(ctx context.Context, id uuid.UUID) error {
	if err := r.client.DB(ctx).FederatedIdentity.DeleteOneID(id).Exec(ctx); err != nil {
		return commonEnt.MapEntError(err, fedIdentityRepoName)
	}
	return nil
}

func (r *FederatedIdentityRepository) Exists(ctx context.Context, id uuid.UUID) (bool, error) {
	exists, err := r.client.DB(ctx).FederatedIdentity.Query().Where(federatedidentity.ID(id)).Exist(ctx)

	if err != nil {
		return false, commonEnt.MapEntError(err, fedIdentityRepoName)
	}

	return exists, nil
}
