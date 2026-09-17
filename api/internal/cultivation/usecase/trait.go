package usecase

import (
	"context"
	"errors"
	"strings"

	"github.com/google/uuid"
	"github.com/huynhanx03/go-common/pkg/common/apperr"
	d "github.com/huynhanx03/go-common/pkg/dto"
	"github.com/huynhanx03/go-common/pkg/logger"
	"go.uber.org/zap"

	contract "github.com/huynhanx03/judgify/internal/cultivation/contract"
	entity "github.com/huynhanx03/judgify/internal/cultivation/domain"
	platformmutation "github.com/huynhanx03/judgify/internal/platform/mutation"
	"github.com/huynhanx03/judgify/internal/ports"
)

type traitService struct {
	traitRepo ports.TraitRepository
	catalog   ports.TraitCatalogInvalidator
	cache     ports.TraitCache
	journal   platformmutation.Executor
	effects   ports.TraitEffectRepository
}

// NewTraitService creates a new TraitService instance.
func NewTraitService(
	traitRepo ports.TraitRepository,
	catalog ports.TraitCatalogInvalidator,
	cache ports.TraitCache,
	effects ports.TraitEffectRepository,
	journals ...platformmutation.Executor,
) ports.TraitService {
	var journal platformmutation.Executor
	if len(journals) > 0 {
		journal = journals[0]
	}
	return &traitService{
		traitRepo: traitRepo,
		catalog:   catalog,
		cache:     cache,
		journal:   journal,
		effects:   effects,
	}
}

func (s *traitService) Find(ctx context.Context, opts *d.QueryOptions) (*d.Paginated[*contract.TraitResponse], error) {
	result, err := s.traitRepo.Find(ctx, opts)
	if err != nil {
		return nil, err
	}

	if result.Records == nil {
		return &d.Paginated[*contract.TraitResponse]{Records: &[]*contract.TraitResponse{}, Pagination: result.Pagination}, nil
	}

	entities := *result.Records
	responses := make([]*contract.TraitResponse, len(entities))
	for i, e := range entities {
		responses[i] = ToTraitResponse(e)
	}

	return &d.Paginated[*contract.TraitResponse]{Records: &responses, Pagination: result.Pagination}, nil
}

func (s *traitService) FindAll(ctx context.Context) ([]*contract.TraitResponse, error) {
	return s.cache.FindAll(ctx, func(ctx context.Context) ([]*contract.TraitResponse, error) {
		logger.FromContext(ctx).Debug("cache miss, loading traits from db")
		entities, err := s.traitRepo.FindAll(ctx)
		if err != nil {
			return nil, err
		}

		responses := make([]*contract.TraitResponse, len(entities))
		for i, e := range entities {
			responses[i] = ToTraitResponse(e)
		}
		return responses, nil
	})
}

func (s *traitService) Get(ctx context.Context, id uuid.UUID) (*contract.TraitResponse, error) {
	return s.cache.Get(ctx, id, func(ctx context.Context) (*contract.TraitResponse, error) {
		logger.FromContext(ctx).Debug("cache miss, loading trait from db", zap.String("trait_id", id.String()))
		e, err := s.traitRepo.Get(ctx, id)
		if err != nil {
			return nil, err
		}
		return ToTraitResponse(e), nil
	})
}

func (s *traitService) Create(ctx context.Context, command *contract.CreateTraitCommand) (*contract.TraitResponse, error) {
	if command == nil || s.effects == nil || !validTraitMutationReason(command.Reason) {
		return nil, invalidTraitMutation(entity.ErrInvalidTraitEffect)
	}
	command.Reason = strings.TrimSpace(command.Reason)
	effect, err := entity.NormalizeTraitEffectSpec(command.Effect)
	if err != nil {
		return nil, invalidTraitMutation(err)
	}
	e := ToTraitEntityFromCreate(command)
	var full *entity.Trait
	err = executeCultivationMutation(s.journal, ctx, func(txCtx context.Context) (platformmutation.Command, error) {
		if err := s.effects.LockTimeline(txCtx); err != nil {
			return platformmutation.Command{}, err
		}
		if err := s.traitRepo.Create(txCtx, e); err != nil {
			return platformmutation.Command{}, err
		}
		published, err := s.effects.Publish(txCtx, e.ID, e.Version, effect)
		if err != nil {
			return platformmutation.Command{}, err
		}
		e.EffectRevision = &published
		full, err = s.traitRepo.Get(txCtx, e.ID)
		if err != nil {
			return platformmutation.Command{}, err
		}
		if err := s.refreshCatalog(txCtx); err != nil {
			return platformmutation.Command{}, err
		}
		return traitMutationCommand(
			"cultivation.trait.created.v1", e.ID, e.Version, command.Reason,
			map[string]any{}, traitDocument(full),
		), nil
	})
	if err != nil {
		return nil, mapTraitMutationError(err)
	}
	if err := s.cache.Invalidate(ctx, e.ID); err != nil {
		return nil, err
	}
	logger.FromContext(ctx).Info("trait created", zap.String("trait_id", e.ID.String()))
	return ToTraitResponse(full), nil
}

func (s *traitService) Update(ctx context.Context, id uuid.UUID, command *contract.UpdateTraitCommand) (*contract.TraitResponse, error) {
	if command == nil || s.effects == nil || command.ExpectedVersion < 1 ||
		!validTraitMutationReason(command.Reason) {
		return nil, invalidTraitMutation(entity.ErrInvalidTraitEffect)
	}
	command.Reason = strings.TrimSpace(command.Reason)
	var e *entity.Trait
	var full *entity.Trait
	err := executeCultivationMutation(s.journal, ctx, func(txCtx context.Context) (platformmutation.Command, error) {
		if err := s.effects.LockTimeline(txCtx); err != nil {
			return platformmutation.Command{}, err
		}
		var err error
		e, err = s.traitRepo.Get(txCtx, id)
		if err != nil {
			return platformmutation.Command{}, err
		}
		if e.Version != command.ExpectedVersion {
			return platformmutation.Command{}, entity.ErrTraitConflict
		}
		before := traitDocument(e)
		if e.EffectRevision == nil || !e.EffectRevision.Valid() {
			return platformmutation.Command{}, entity.ErrInvalidTraitEffect
		}
		effect := e.EffectRevision.Effect
		if command.Type != nil {
			e.Type = *command.Type
		}
		if command.Name != nil {
			e.Name = *command.Name
		}
		if command.RarityID != nil {
			e.RarityID = *command.RarityID
		}
		if command.Description != nil {
			e.Description = *command.Description
		}
		if command.DisplayOrder != nil {
			e.DisplayOrder = *command.DisplayOrder
		}
		if command.Active != nil {
			e.Active = *command.Active
		}
		if command.Effect != nil {
			effect, err = entity.NormalizeTraitEffectSpec(*command.Effect)
			if err != nil {
				return platformmutation.Command{}, err
			}
		}
		if err := s.traitRepo.Update(txCtx, e, command.ExpectedVersion); err != nil {
			return platformmutation.Command{}, err
		}
		published, err := s.effects.Publish(txCtx, e.ID, e.Version, effect)
		if err != nil {
			return platformmutation.Command{}, err
		}
		e.EffectRevision = &published
		full, err = s.traitRepo.Get(txCtx, id)
		if err != nil {
			return platformmutation.Command{}, err
		}
		if err := s.refreshCatalog(txCtx); err != nil {
			return platformmutation.Command{}, err
		}
		return traitMutationCommand(
			"cultivation.trait.updated.v1", id, full.Version, command.Reason,
			before, traitDocument(full),
		), nil
	})
	if err != nil {
		return nil, mapTraitMutationError(err)
	}
	if err := s.cache.Invalidate(ctx, id); err != nil {
		return nil, err
	}
	logger.FromContext(ctx).Info("trait updated", zap.String("trait_id", e.ID.String()))
	return ToTraitResponse(full), nil
}

func (s *traitService) Delete(
	ctx context.Context,
	id uuid.UUID,
	expectedVersion int64,
	reason string,
) error {
	if s.effects == nil || expectedVersion < 1 || !validTraitMutationReason(reason) {
		return invalidTraitMutation(entity.ErrInvalidTraitEffect)
	}
	reason = strings.TrimSpace(reason)
	err := executeCultivationMutation(s.journal, ctx, func(txCtx context.Context) (platformmutation.Command, error) {
		if err := s.effects.LockTimeline(txCtx); err != nil {
			return platformmutation.Command{}, err
		}
		stored, err := s.traitRepo.Get(txCtx, id)
		if err != nil {
			return platformmutation.Command{}, err
		}
		if stored.Version != expectedVersion || !stored.Active {
			return platformmutation.Command{}, entity.ErrTraitConflict
		}
		if stored.EffectRevision == nil || !stored.EffectRevision.Valid() {
			return platformmutation.Command{}, entity.ErrInvalidTraitEffect
		}
		before := traitDocument(stored)
		stored.Active = false
		if err := s.traitRepo.Update(txCtx, stored, expectedVersion); err != nil {
			return platformmutation.Command{}, err
		}
		published, err := s.effects.Publish(
			txCtx, stored.ID, stored.Version, stored.EffectRevision.Effect,
		)
		if err != nil {
			return platformmutation.Command{}, err
		}
		stored.EffectRevision = &published
		if err := s.refreshCatalog(txCtx); err != nil {
			return platformmutation.Command{}, err
		}
		return traitMutationCommand(
			"cultivation.trait.archived.v1", id, stored.Version, reason,
			before, traitDocument(stored),
		), nil
	})
	if err != nil {
		return mapTraitMutationError(err)
	}
	if err := s.cache.Invalidate(ctx, id); err != nil {
		return err
	}
	logger.FromContext(ctx).Info("trait archived", zap.String("trait_id", id.String()))
	return nil
}

func traitMutationCommand(
	action string,
	id uuid.UUID,
	version int64,
	reason string,
	before map[string]any,
	after map[string]any,
) platformmutation.Command {
	command := cultivationCommand(
		action, "cultivation_trait", id, version, before, after,
	)
	command.Reason = reason
	return command
}

func invalidTraitMutation(err error) error {
	if errors.Is(err, entity.ErrTraitConflict) {
		return apperr.New(apperr.CodeConflict, "trait changed; reload and try again", err)
	}
	return apperr.New(apperr.CodeBadRequest, "invalid trait mutation", err)
}

func mapTraitMutationError(err error) error {
	if errors.Is(err, entity.ErrTraitConflict) {
		return invalidTraitMutation(err)
	}
	if errors.Is(err, entity.ErrInvalidTraitEffect) {
		return invalidTraitMutation(err)
	}
	return err
}

func validTraitMutationReason(value string) bool {
	value = strings.TrimSpace(value)
	return len(value) >= 3 && len(value) <= 1024
}

func (s *traitService) refreshCatalog(ctx context.Context) error {
	if s.catalog == nil {
		return entity.ErrTraitOfferUnavailable
	}
	return s.catalog.InvalidatePool(ctx)
}
