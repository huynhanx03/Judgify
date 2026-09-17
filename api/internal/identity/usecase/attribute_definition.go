package usecase

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/huynhanx03/go-common/pkg/common/apperr"
	"github.com/huynhanx03/go-common/pkg/common/tx"
	"github.com/huynhanx03/go-common/pkg/correlation"
	d "github.com/huynhanx03/go-common/pkg/dto"
	"github.com/huynhanx03/go-common/pkg/logger"
	"go.uber.org/zap"

	"github.com/huynhanx03/judgify/internal/authctx"
	"github.com/huynhanx03/judgify/internal/constant"
	"github.com/huynhanx03/judgify/internal/identity/attributemigration"
	"github.com/huynhanx03/judgify/internal/identity/attributevalue"
	dto "github.com/huynhanx03/judgify/internal/identity/contract"
	entity "github.com/huynhanx03/judgify/internal/identity/domain"
	"github.com/huynhanx03/judgify/internal/platform/audit"
	"github.com/huynhanx03/judgify/internal/platform/eventing"
	"github.com/huynhanx03/judgify/internal/platform/operation"
	"github.com/huynhanx03/judgify/internal/ports"
)

var attributeDefinitionKeyPattern = regexp.MustCompile(`^[a-z][a-z0-9_.-]{1,49}$`)

var errAttributeLifecyclePreviewChanged = errors.New(
	"attribute lifecycle preview changed during commit",
)

type attributeDefinitionService struct {
	attrDefRepo        ports.AttributeDefinitionRepository
	cache              ports.AttributeDefinitionCache
	txMgr              tx.Manager
	audit              audit.Appender
	outbox             eventing.OutboxAppender
	migration          attributemigration.CoordinatorRepository
	operations         *operation.Starter
	migrationBatchSize int
	profileSchema      ports.ProfileSchemaRepository
}

type AttributeDefinitionServiceOption func(*attributeDefinitionService)

// AttributeDefinitionLifecycle is the complete composition-root contract for
// the profile-schema service. HTTP and workflows consume the narrower public
// service port; the operation executor additionally consumes Activator. The
// constructor returns both capabilities explicitly so wiring is compile-time
// checked instead of relying on a runtime type assertion.
type AttributeDefinitionLifecycle interface {
	ports.AttributeDefinitionService
	attributemigration.Activator
}

func WithAttributeDefinitionLifecycle(
	txMgr tx.Manager,
	auditAppender audit.Appender,
	outbox eventing.OutboxAppender,
) AttributeDefinitionServiceOption {
	return func(service *attributeDefinitionService) {
		service.txMgr = txMgr
		service.audit = auditAppender
		service.outbox = outbox
	}
}

func WithAttributeDefinitionMigration(
	repository attributemigration.CoordinatorRepository,
	starter *operation.Starter,
	batchSize int,
) AttributeDefinitionServiceOption {
	return func(service *attributeDefinitionService) {
		service.migration = repository
		service.operations = starter
		service.migrationBatchSize = batchSize
	}
}

func WithAttributeDefinitionProfileSchema(
	repository ports.ProfileSchemaRepository,
) AttributeDefinitionServiceOption {
	return func(service *attributeDefinitionService) {
		service.profileSchema = repository
	}
}

// NewAttributeDefinitionService creates a new AttributeDefinitionService instance.
func NewAttributeDefinitionService(
	attrDefRepo ports.AttributeDefinitionRepository,
	cache ports.AttributeDefinitionCache,
	options ...AttributeDefinitionServiceOption,
) AttributeDefinitionLifecycle {
	service := &attributeDefinitionService{
		attrDefRepo: attrDefRepo,
		cache:       cache,
	}
	for _, option := range options {
		if option != nil {
			option(service)
		}
	}
	return service
}

// Find retrieves attribute definitions with pagination.
func (s *attributeDefinitionService) Find(ctx context.Context, opts *d.QueryOptions) (*d.Paginated[*dto.AttributeDefinitionResponse], error) {
	result, err := s.attrDefRepo.Find(ctx, opts)
	if err != nil {
		return nil, err
	}

	if result.Records == nil {
		return &d.Paginated[*dto.AttributeDefinitionResponse]{
			Records:    &[]*dto.AttributeDefinitionResponse{},
			Pagination: result.Pagination,
		}, nil
	}

	entities := *result.Records
	responses := make([]*dto.AttributeDefinitionResponse, len(entities))
	for i, e := range entities {
		responses[i] = ToAttributeDefinitionResponse(e)
	}

	return &d.Paginated[*dto.AttributeDefinitionResponse]{
		Records:    &responses,
		Pagination: result.Pagination,
	}, nil
}

// Get retrieves an attribute definition by ID.
func (s *attributeDefinitionService) Get(ctx context.Context, id uuid.UUID) (*dto.AttributeDefinitionResponse, error) {
	attrDef, err := s.cache.Get(ctx, id, func(ctx context.Context) (*entity.AttributeDefinition, error) {
		return s.attrDefRepo.Get(ctx, id)
	})
	if err != nil {
		return nil, err
	}

	return ToAttributeDefinitionResponse(attrDef), nil
}

func (s *attributeDefinitionService) ListRevisions(
	ctx context.Context,
	id uuid.UUID,
	cursor string,
	limit int,
) (*dto.AttributeDefinitionRevisionPage, error) {
	if s == nil || s.attrDefRepo == nil || ctx == nil || ctx.Err() != nil ||
		id == uuid.Nil || len(cursor) > 32 || limit < 0 || limit > 100 {
		return nil, apperr.New(apperr.CodeBadRequest, "invalid attribute revision query", nil)
	}
	before := int64(0)
	if cursor != "" {
		parsed, err := strconv.ParseInt(cursor, 10, 64)
		if err != nil || parsed < 2 {
			return nil, apperr.New(apperr.CodeBadRequest, "invalid attribute revision cursor", err)
		}
		before = parsed
	}
	if limit == 0 {
		limit = 25
	}
	if _, err := s.attrDefRepo.Get(ctx, id); err != nil {
		return nil, err
	}
	revisions, err := s.attrDefRepo.ListRevisions(ctx, id, before, limit+1)
	if err != nil {
		return nil, err
	}
	count := min(len(revisions), limit)
	page := &dto.AttributeDefinitionRevisionPage{
		Records: make([]*dto.AttributeDefinitionRevisionResponse, 0, count),
	}
	for index := 0; index < count; index++ {
		response := ToAttributeDefinitionRevisionResponse(revisions[index])
		if response == nil || response.DefinitionID != id || response.Revision < 1 {
			return nil, apperr.New(apperr.CodeInternalServer, "invalid attribute revision projection", nil)
		}
		page.Records = append(page.Records, response)
	}
	if len(revisions) > limit {
		page.NextCursor = strconv.FormatInt(revisions[limit-1].Revision, 10)
	}
	return page, nil
}

// Create creates a new attribute definition.
func (s *attributeDefinitionService) Create(ctx context.Context, req *dto.CreateAttributeDefinitionRequest) (*dto.AttributeDefinitionResponse, error) {
	if err := s.validateMutationContext(ctx); err != nil {
		return nil, err
	}
	if req == nil || req.CommandID == uuid.Nil {
		return nil, apperr.New(apperr.CodeBadRequest, "invalid attribute definition command", nil)
	}
	req.Key = strings.ToLower(strings.TrimSpace(req.Key))
	req.Label = strings.TrimSpace(req.Label)
	req.Description = strings.TrimSpace(req.Description)
	req.Reason = strings.TrimSpace(req.Reason)
	actorID, actorOK := authctx.UserID(ctx)
	sessionID, sessionOK := authctx.SessionID(ctx)
	validation, err := canonicalAttributeValidation(req.Validation)
	if !actorOK || !sessionOK {
		return nil, apperr.New(apperr.CodeUnauthorized, "authentication required", nil)
	}
	if err != nil || !attributeDefinitionKeyPattern.MatchString(req.Key) || req.Reason == "" {
		return nil, apperr.New(apperr.CodeBadRequest, "invalid attribute definition command", err)
	}
	if err := validateRequiredOnboardingRevision(
		req.RequiredOnOnboarding,
		entity.AttributeVisibility(req.Visibility),
		req.UserEditable,
	); err != nil {
		return nil, apperr.New(apperr.CodeBadRequest, "invalid onboarding attribute revision", err)
	}
	req.Validation = validation
	requestHash, err := attributeDefinitionCommandHash(
		"identity.attribute_definition.create.v1",
		struct {
			CommandID            uuid.UUID       `json:"command_id"`
			Reason               string          `json:"reason"`
			Key                  string          `json:"key"`
			DataType             string          `json:"data_type"`
			Label                string          `json:"label"`
			Description          string          `json:"description"`
			Visibility           string          `json:"visibility"`
			UserEditable         bool            `json:"user_editable"`
			DisplayOrder         int             `json:"display_order"`
			RequiredOnOnboarding bool            `json:"required_on_onboarding"`
			Validation           json.RawMessage `json:"validation"`
		}{
			CommandID:            req.CommandID,
			Reason:               req.Reason,
			Key:                  req.Key,
			DataType:             req.DataType,
			Label:                req.Label,
			Description:          req.Description,
			Visibility:           req.Visibility,
			UserEditable:         req.UserEditable,
			DisplayOrder:         req.DisplayOrder,
			RequiredOnOnboarding: req.RequiredOnOnboarding,
			Validation:           validation,
		},
	)
	if err != nil {
		return nil, apperr.New(apperr.CodeBadRequest, "invalid attribute definition command", err)
	}
	ctx = correlation.EnsureContext(ctx)
	if replayed, found, replayErr := s.loadAttributeDefinitionMutationReceipt(
		ctx,
		"create",
		req.CommandID,
		uuid.Nil,
		1,
		requestHash,
		req.Key,
	); replayErr != nil || found {
		return replayed, replayErr
	}
	attrDef := ToAttributeDefinitionEntityFromCreate(req)
	var revision *entity.AttributeDefinitionRevision
	err = s.txMgr.DoInTx(ctx, func(txCtx context.Context) error {
		fence, err := s.profileSchema.LockExclusive(txCtx)
		if err != nil {
			return err
		}
		if err := s.attrDefRepo.Create(txCtx, attrDef); err != nil {
			return err
		}
		revisionID, err := uuid.NewV7()
		if err != nil {
			return fmt.Errorf("create attribute revision identity: %w", err)
		}
		now := time.Now().UTC()
		revision = &entity.AttributeDefinitionRevision{
			ID:           revisionID,
			DefinitionID: attrDef.ID,
			Revision:     1,
			DataType:     entity.AttributeValueType(req.DataType),
			Label:        strings.TrimSpace(req.Label),
			Description:  strings.TrimSpace(req.Description),
			Visibility:   entity.AttributeVisibility(req.Visibility),
			UserEditable: req.UserEditable,
			DisplayOrder: req.DisplayOrder,
			Validation:   validation,
			CreatedBy:    &actorID,
			CreatedAt:    now,
		}
		if err := validateAttributeRevision(revision); err != nil {
			return err
		}
		if err := s.attrDefRepo.CreateRevision(txCtx, revision); err != nil {
			return err
		}
		if err := s.attrDefRepo.SetInitialRevision(txCtx, attrDef.ID, revision.ID); err != nil {
			return err
		}
		attrDef.DraftRevisionID = &revision.ID
		attrDef.ActiveRevisionID = &revision.ID
		attrDef.DraftRevision = revision
		attrDef.ActiveRevision = revision
		if req.RequiredOnOnboarding {
			if _, err := s.profileSchema.Advance(txCtx, fence); err != nil {
				return err
			}
		}
		auditID, err := s.audit.Append(txCtx, audit.Entry{
			ActorUserID:    &actorID,
			ActorSessionID: &sessionID,
			Action:         "identity.attribute_definition.create",
			Resource:       "identity.attribute_definition",
			ResourceID:     attrDef.ID.String(),
			Reason:         strings.TrimSpace(req.Reason),
			Before:         map[string]any{},
			After:          attributeDefinitionAudit(attrDef, revision),
			OccurredAt:     now,
		})
		if err != nil {
			return err
		}
		payload, err := json.Marshal(map[string]any{
			"command_id":    req.CommandID.String(),
			"audit_id":      auditID.String(),
			"definition_id": attrDef.ID.String(),
			"revision_id":   revision.ID.String(),
			"version":       int64(1),
			"action":        "create",
			"request_hash":  hex.EncodeToString(requestHash[:]),
			"definition":    ToAttributeDefinitionResponse(attrDef),
		})
		if err != nil {
			return err
		}
		return s.outbox.Append(txCtx, eventing.Event{
			ID:               req.CommandID,
			Type:             "identity.attribute_definition.created.v1",
			AggregateType:    "attribute_definition",
			AggregateID:      attrDef.ID,
			AggregateVersion: 1,
			Payload:          payload,
			OccurredAt:       now,
		})
	})
	if err != nil {
		if replayed, found, replayErr := s.loadAttributeDefinitionMutationReceipt(
			ctx,
			"create",
			req.CommandID,
			uuid.Nil,
			1,
			requestHash,
			req.Key,
		); replayErr != nil || found {
			return replayed, replayErr
		}
		return nil, err
	}

	logger.FromContext(ctx).Info("attribute definition created", zap.String("attribute_definition_id", attrDef.ID.String()))
	return ToAttributeDefinitionResponse(attrDef), nil
}

// Update updates an existing attribute definition.
func (s *attributeDefinitionService) Update(ctx context.Context, id uuid.UUID, req *dto.UpdateAttributeDefinitionRequest) (*dto.AttributeDefinitionResponse, error) {
	if err := s.validateMutationContext(ctx); err != nil {
		return nil, err
	}
	if req == nil || req.CommandID == uuid.Nil || req.ExpectedVersion < 1 || strings.TrimSpace(req.Reason) == "" {
		return nil, apperr.New(apperr.CodeBadRequest, "invalid attribute definition command", nil)
	}
	req.Reason = strings.TrimSpace(req.Reason)
	normalizeOptionalAttributeString(req.Label)
	normalizeOptionalAttributeString(req.Description)
	actorID, actorOK := authctx.UserID(ctx)
	sessionID, sessionOK := authctx.SessionID(ctx)
	if !actorOK || !sessionOK {
		return nil, apperr.New(apperr.CodeUnauthorized, "authentication required", nil)
	}
	ctx = correlation.EnsureContext(ctx)
	var err error
	requestValidation := append(json.RawMessage(nil), req.Validation...)
	if len(requestValidation) > 0 {
		requestValidation, err = canonicalAttributeValidation(requestValidation)
		if err != nil {
			return nil, apperr.New(apperr.CodeBadRequest, "invalid attribute validation", err)
		}
		req.Validation = requestValidation
	}
	requestHash, err := attributeDefinitionCommandHash(
		"identity.attribute_definition.draft.v1",
		struct {
			DefinitionID         uuid.UUID       `json:"definition_id"`
			CommandID            uuid.UUID       `json:"command_id"`
			ExpectedVersion      int64           `json:"expected_version"`
			Reason               string          `json:"reason"`
			DataType             *string         `json:"data_type,omitempty"`
			Label                *string         `json:"label,omitempty"`
			Description          *string         `json:"description,omitempty"`
			Visibility           *string         `json:"visibility,omitempty"`
			UserEditable         *bool           `json:"user_editable,omitempty"`
			DisplayOrder         *int            `json:"display_order,omitempty"`
			RequiredOnOnboarding *bool           `json:"required_on_onboarding,omitempty"`
			Validation           json.RawMessage `json:"validation,omitempty"`
		}{
			DefinitionID:         id,
			CommandID:            req.CommandID,
			ExpectedVersion:      req.ExpectedVersion,
			Reason:               req.Reason,
			DataType:             req.DataType,
			Label:                req.Label,
			Description:          req.Description,
			Visibility:           req.Visibility,
			UserEditable:         req.UserEditable,
			DisplayOrder:         req.DisplayOrder,
			RequiredOnOnboarding: req.RequiredOnOnboarding,
			Validation:           requestValidation,
		},
	)
	if err != nil {
		return nil, apperr.New(apperr.CodeBadRequest, "invalid attribute definition command", err)
	}
	if replayed, found, replayErr := s.loadAttributeDefinitionMutationReceipt(
		ctx,
		"draft",
		req.CommandID,
		id,
		req.ExpectedVersion+1,
		requestHash,
		"",
	); replayErr != nil || found {
		return replayed, replayErr
	}
	attrDef, err := s.attrDefRepo.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	if attrDef.Version != req.ExpectedVersion || attrDef.DraftRevisionID == nil {
		return nil, attributeVersionConflict(attrDef.Version)
	}
	if attrDef.Kind == "system" && req.RequiredOnOnboarding != nil &&
		!*req.RequiredOnOnboarding {
		return nil, apperr.New(
			apperr.CodeConflict,
			"code-required system attribute cannot be disabled for onboarding",
			nil,
		)
	}
	base, err := s.attrDefRepo.GetRevision(ctx, *attrDef.DraftRevisionID)
	if err != nil {
		return nil, err
	}
	if base == nil || base.DefinitionID != id {
		return nil, apperr.New(apperr.CodeInternalServer, "invalid attribute revision", nil)
	}
	next := *base
	next.ID, err = uuid.NewV7()
	if err != nil {
		return nil, err
	}
	latest, err := s.attrDefRepo.ListRevisions(ctx, id, 0, 1)
	if err != nil {
		return nil, err
	}
	if len(latest) != 1 || latest[0] == nil || latest[0].Revision < 1 {
		return nil, apperr.New(apperr.CodeInternalServer, "invalid attribute revision history", nil)
	}
	next.Revision = latest[0].Revision + 1
	next.CreatedBy = &actorID
	next.CreatedAt = time.Now().UTC()
	if req.DataType != nil {
		next.DataType = entity.AttributeValueType(*req.DataType)
	}
	if req.Label != nil {
		next.Label = strings.TrimSpace(*req.Label)
	}
	if req.Description != nil {
		next.Description = strings.TrimSpace(*req.Description)
	}
	if req.Visibility != nil {
		next.Visibility = entity.AttributeVisibility(*req.Visibility)
	}
	if req.UserEditable != nil {
		next.UserEditable = *req.UserEditable
	}
	if req.DisplayOrder != nil {
		next.DisplayOrder = *req.DisplayOrder
	}
	if len(req.Validation) > 0 {
		next.Validation, err = canonicalAttributeValidation(req.Validation)
		if err != nil {
			return nil, apperr.New(apperr.CodeBadRequest, "invalid attribute validation", err)
		}
	}
	if sameAttributeRevisionContent(base, &next) &&
		(req.RequiredOnOnboarding == nil ||
			*req.RequiredOnOnboarding == attrDef.RequiredOnOnboarding) {
		return nil, apperr.New(
			apperr.CodeBadRequest,
			"attribute definition command has no changes",
			nil,
		)
	}
	if err := validateAttributeRevision(&next); err != nil {
		return nil, apperr.New(apperr.CodeBadRequest, "invalid attribute revision", err)
	}
	requiredOnOnboarding := attrDef.RequiredOnOnboarding
	if req.RequiredOnOnboarding != nil {
		requiredOnOnboarding = *req.RequiredOnOnboarding
	}
	if err := validateRequiredOnboardingRevision(
		requiredOnOnboarding,
		next.Visibility,
		next.UserEditable,
	); err != nil {
		return nil, apperr.New(apperr.CodeBadRequest, "invalid onboarding attribute revision", err)
	}
	if requiredOnOnboarding && (attrDef.ActiveRevision == nil ||
		validateRequiredOnboardingRevision(
			true,
			attrDef.ActiveRevision.Visibility,
			attrDef.ActiveRevision.UserEditable,
		) != nil) {
		return nil, apperr.New(
			apperr.CodeConflict,
			"active revision is not eligible for required onboarding",
			nil,
		)
	}
	var updated *entity.AttributeDefinition
	err = s.txMgr.DoInTx(ctx, func(txCtx context.Context) error {
		fence, fenceErr := s.profileSchema.LockExclusive(txCtx)
		if fenceErr != nil {
			return fenceErr
		}
		if s.migration != nil {
			activeMigration, migrationErr := s.migration.ActiveMigrationTarget(txCtx, attrDef)
			if migrationErr != nil {
				return migrationErr
			}
			if activeMigration != nil {
				return attributemigration.ErrMigrationActive
			}
		}
		if err := s.attrDefRepo.CreateRevision(txCtx, &next); err != nil {
			return err
		}
		var err error
		updated, err = s.attrDefRepo.SetDraftRevision(
			txCtx,
			id,
			req.ExpectedVersion,
			&next,
			req.RequiredOnOnboarding,
		)
		if err != nil {
			return err
		}
		updated.DraftRevision = &next
		updated.ActiveRevision = attrDef.ActiveRevision
		if req.RequiredOnOnboarding != nil &&
			*req.RequiredOnOnboarding != attrDef.RequiredOnOnboarding {
			if _, err := s.profileSchema.Advance(txCtx, fence); err != nil {
				return err
			}
		}
		now := time.Now().UTC()
		auditID, err := s.audit.Append(txCtx, audit.Entry{
			ActorUserID:    &actorID,
			ActorSessionID: &sessionID,
			Action:         "identity.attribute_definition.draft",
			Resource:       "identity.attribute_definition",
			ResourceID:     id.String(),
			Reason:         strings.TrimSpace(req.Reason),
			Before:         attributeDefinitionAudit(attrDef, base),
			After:          attributeDefinitionAudit(updated, &next),
			OccurredAt:     now,
		})
		if err != nil {
			return err
		}
		payload, err := json.Marshal(map[string]any{
			"command_id":        req.CommandID.String(),
			"audit_id":          auditID.String(),
			"definition_id":     id.String(),
			"revision_id":       next.ID.String(),
			"expected_version":  req.ExpectedVersion,
			"resulting_version": req.ExpectedVersion + 1,
			"action":            "draft",
			"request_hash":      hex.EncodeToString(requestHash[:]),
			"definition":        ToAttributeDefinitionResponse(updated),
		})
		if err != nil {
			return err
		}
		return s.outbox.Append(txCtx, eventing.Event{
			ID:               req.CommandID,
			Type:             "identity.attribute_definition.draft_created.v1",
			AggregateType:    "attribute_definition",
			AggregateID:      id,
			AggregateVersion: req.ExpectedVersion + 1,
			Payload:          payload,
			OccurredAt:       now,
		})
	})
	if err != nil {
		if replayed, found, replayErr := s.loadAttributeDefinitionMutationReceipt(
			ctx,
			"draft",
			req.CommandID,
			id,
			req.ExpectedVersion+1,
			requestHash,
			"",
		); replayErr != nil || found {
			return replayed, replayErr
		}
		if errors.Is(err, ports.ErrAttributeDefinitionVersionConflict) {
			if current, loadErr := s.attrDefRepo.Get(ctx, id); loadErr == nil && current != nil {
				return nil, attributeVersionConflict(current.Version)
			}
			return nil, attributeVersionConflict(req.ExpectedVersion)
		}
		if errors.Is(err, attributemigration.ErrMigrationActive) {
			return nil, apperr.New(
				apperr.CodeConflict,
				"attribute migration is active",
				err,
			).WithDetails(map[string]any{"reason": "migration_active"})
		}
		return nil, err
	}

	// The key is immutable; invalidate both stable cache views.
	if err := s.cache.Invalidate(ctx, id, updated.Key); err != nil {
		return nil, err
	}

	logger.FromContext(ctx).Info("attribute definition draft created", zap.String("attribute_definition_id", updated.ID.String()), zap.Int64("version", updated.Version))
	return ToAttributeDefinitionResponse(updated), nil
}

func (s *attributeDefinitionService) validateMutationContext(ctx context.Context) error {
	if s == nil || s.attrDefRepo == nil || s.txMgr == nil || s.audit == nil ||
		s.outbox == nil || s.profileSchema == nil || ctx == nil || ctx.Err() != nil {
		return apperr.New(apperr.CodeInternalServer, "attribute definition lifecycle unavailable", nil)
	}
	return nil
}

func canonicalAttributeValidation(value json.RawMessage) (json.RawMessage, error) {
	if len(value) == 0 || len(value) > 8<<10 || !json.Valid(value) {
		return nil, fmt.Errorf("invalid attribute validation document")
	}
	decoder := json.NewDecoder(bytes.NewReader(value))
	decoder.UseNumber()
	var document map[string]any
	if err := decoder.Decode(&document); err != nil || document == nil {
		return nil, fmt.Errorf("attribute validation must be an object")
	}
	canonical, err := json.Marshal(document)
	if err != nil || len(canonical) > 8<<10 {
		return nil, fmt.Errorf("invalid attribute validation document")
	}
	return canonical, nil
}

func validateAttributeRevision(revision *entity.AttributeDefinitionRevision) error {
	if revision == nil || revision.ID == uuid.Nil || revision.DefinitionID == uuid.Nil ||
		revision.Revision < 1 || !revision.DataType.Valid() ||
		!revision.Visibility.Valid() || strings.TrimSpace(revision.Label) != revision.Label ||
		len(revision.Label) < 2 || len(revision.Label) > 128 ||
		revision.DisplayOrder < 0 || revision.DisplayOrder > 10000 ||
		len(revision.Description) > 1024 || len(revision.Validation) == 0 ||
		!json.Valid(revision.Validation) || revision.CreatedAt.IsZero() ||
		attributevalue.ValidateRevision(*revision) != nil {
		return fmt.Errorf("invalid attribute definition revision")
	}
	return nil
}

func validateRequiredOnboardingRevision(
	required bool,
	visibility entity.AttributeVisibility,
	userEditable bool,
) error {
	if required && visibility == entity.AttributeVisibilityAdmin {
		return fmt.Errorf("required onboarding attribute cannot be admin-only")
	}
	if required && !userEditable {
		return fmt.Errorf("required onboarding attribute must be user editable")
	}
	return nil
}

func sameAttributeRevisionContent(
	left *entity.AttributeDefinitionRevision,
	right *entity.AttributeDefinitionRevision,
) bool {
	return left != nil && right != nil &&
		left.DataType == right.DataType && left.Label == right.Label &&
		left.Description == right.Description &&
		left.Visibility == right.Visibility &&
		left.UserEditable == right.UserEditable &&
		left.DisplayOrder == right.DisplayOrder &&
		bytes.Equal(left.Validation, right.Validation)
}

func attributeDefinitionAudit(
	definition *entity.AttributeDefinition,
	revision *entity.AttributeDefinitionRevision,
) map[string]any {
	if definition == nil || revision == nil {
		return map[string]any{}
	}
	return map[string]any{
		"definition_id":          definition.ID.String(),
		"key":                    definition.Key,
		"status":                 definition.Status,
		"version":                definition.Version,
		"revision_id":            revision.ID.String(),
		"revision":               revision.Revision,
		"data_type":              string(revision.DataType),
		"visibility":             string(revision.Visibility),
		"user_editable":          revision.UserEditable,
		"display_order":          revision.DisplayOrder,
		"required_on_onboarding": definition.RequiredOnOnboarding,
	}
}

func attributeVersionConflict(current int64) error {
	return apperr.New(
		apperr.CodeConflict,
		"attribute definition changed; reload and try again",
		ports.ErrAttributeDefinitionVersionConflict,
	).WithDetails(map[string]any{"current_version": current})
}

func (s *attributeDefinitionService) PreviewLifecycle(
	ctx context.Context,
	command ports.PreviewAttributeLifecycleCommand,
) (ports.AttributeLifecyclePreview, error) {
	if s == nil || s.attrDefRepo == nil || ctx == nil || ctx.Err() != nil ||
		!command.Action.Valid() || command.DefinitionID == uuid.Nil ||
		command.ExpectedVersion < 1 ||
		(command.Action == ports.AttributeLifecycleActivate) !=
			(command.CandidateRevisionID != uuid.Nil) {
		return ports.AttributeLifecyclePreview{}, apperr.New(
			apperr.CodeBadRequest,
			"invalid attribute lifecycle command",
			nil,
		)
	}
	definition, err := s.attrDefRepo.Get(ctx, command.DefinitionID)
	if err != nil {
		return ports.AttributeLifecyclePreview{}, err
	}
	if definition == nil || definition.Version != command.ExpectedVersion ||
		definition.Status != "active" || definition.ActiveRevisionID == nil ||
		definition.ActiveRevision == nil {
		current := command.ExpectedVersion
		if definition != nil {
			current = definition.Version
		}
		return ports.AttributeLifecyclePreview{}, attributeVersionConflict(current)
	}
	preview := ports.AttributeLifecyclePreview{
		Action:           command.Action,
		DefinitionID:     command.DefinitionID,
		ExpectedVersion:  command.ExpectedVersion,
		ResultingVersion: command.ExpectedVersion + 1,
		CurrentRevision:  ToAttributeDefinitionRevisionResponse(definition.ActiveRevision),
	}
	valueCount, err := s.attrDefRepo.CountValues(
		ctx,
		definition.ID,
		*definition.ActiveRevisionID,
	)
	if err != nil {
		return ports.AttributeLifecyclePreview{}, err
	}
	preview.SourceValueCount = valueCount
	if command.Action == ports.AttributeLifecycleArchive {
		if definition.Kind == "system" || definition.RequiredOnOnboarding {
			return ports.AttributeLifecyclePreview{}, apperr.New(
				apperr.CodeConflict,
				"protected attribute definition cannot be archived",
				nil,
			)
		}
		return preview, nil
	}
	candidate, err := s.attrDefRepo.GetRevision(ctx, command.CandidateRevisionID)
	if err != nil {
		return ports.AttributeLifecyclePreview{}, err
	}
	if candidate == nil || validateAttributeRevision(candidate) != nil ||
		candidate.DefinitionID != definition.ID ||
		candidate.ID == *definition.ActiveRevisionID {
		return ports.AttributeLifecyclePreview{}, apperr.New(
			apperr.CodeConflict,
			"candidate attribute revision is not activatable",
			nil,
		)
	}
	if err := validateRequiredOnboardingRevision(
		definition.RequiredOnOnboarding,
		candidate.Visibility,
		candidate.UserEditable,
	); err != nil {
		return ports.AttributeLifecyclePreview{}, apperr.New(
			apperr.CodeConflict,
			"candidate revision is not eligible for required onboarding",
			err,
		)
	}
	if definition.Kind == "system" {
		spec, protected := constant.RequiredSystemAttribute(definition.Key)
		expectedValidation, validationErr := canonicalAttributeValidation(
			json.RawMessage(spec.Validation),
		)
		if !protected || validationErr != nil ||
			candidate.DataType != entity.AttributeValueType(spec.DataType) ||
			!bytes.Equal(candidate.Validation, expectedValidation) {
			return ports.AttributeLifecyclePreview{}, apperr.New(
				apperr.CodeConflict,
				"system attribute type and validation are code-owned",
				nil,
			)
		}
	}
	preview.CandidateRevision = ToAttributeDefinitionRevisionResponse(candidate)
	targetValueCount, err := s.attrDefRepo.CountValues(
		ctx,
		definition.ID,
		candidate.ID,
	)
	if err != nil {
		return ports.AttributeLifecyclePreview{}, err
	}
	preview.TargetValueCount = targetValueCount
	// Values are keyed by revision ID. Even a schema-compatible revision needs
	// a shadow copy before the active pointer moves, otherwise existing profile
	// values would disappear from ordinary reads. A retained target also needs
	// bounded cleanup even when the current source is empty, otherwise stale
	// historical rows could become visible again.
	preview.RequiresMigration = valueCount > 0 || targetValueCount > 0
	return preview, nil
}

func (s *attributeDefinitionService) ApplyLifecycle(
	ctx context.Context,
	command ports.ApplyAttributeLifecycleCommand,
) (*dto.AttributeLifecycleReceipt, error) {
	if err := s.validateMutationContext(ctx); err != nil {
		return nil, err
	}
	command.Reason = strings.TrimSpace(command.Reason)
	if !command.Action.Valid() || command.DefinitionID == uuid.Nil ||
		command.CommandID == uuid.Nil || command.ExpectedVersion < 1 ||
		command.RequestHash == ([sha256.Size]byte{}) ||
		command.ReviewedPreviewHash == ([sha256.Size]byte{}) || command.Reason == "" ||
		(command.Action == ports.AttributeLifecycleActivate) !=
			(command.CandidateRevisionID != uuid.Nil) {
		return nil, apperr.New(apperr.CodeBadRequest, "invalid attribute lifecycle command", nil)
	}
	actorID, actorOK := authctx.UserID(ctx)
	sessionID, sessionOK := authctx.SessionID(ctx)
	if !actorOK || !sessionOK {
		return nil, apperr.New(apperr.CodeUnauthorized, "authentication required", nil)
	}
	ctx = correlation.EnsureContext(ctx)
	if replayed, found, err := s.loadAttributeLifecycleReceipt(ctx, command, true); err != nil || found {
		return replayed, err
	}
	preview, err := s.PreviewLifecycle(ctx, ports.PreviewAttributeLifecycleCommand{
		Action:              command.Action,
		DefinitionID:        command.DefinitionID,
		CandidateRevisionID: command.CandidateRevisionID,
		ExpectedVersion:     command.ExpectedVersion,
	})
	if err != nil {
		return nil, err
	}
	previewHash, err := ports.HashAttributeLifecyclePreview(preview)
	if err != nil || !bytes.Equal(previewHash[:], command.ReviewedPreviewHash[:]) {
		return nil, apperr.New(
			apperr.CodeConflict,
			"attribute lifecycle preview changed; review again",
			err,
		).WithDetails(map[string]any{"reason": "preview_changed"})
	}
	if preview.RequiresMigration {
		return s.scheduleAttributeMigration(
			ctx,
			command,
			actorID,
			sessionID,
			preview,
		)
	}
	var (
		updated *entity.AttributeDefinition
		receipt *dto.AttributeLifecycleReceipt
	)
	err = s.txMgr.DoInTx(ctx, func(txCtx context.Context) error {
		var commitErr error
		receipt, updated, commitErr = s.commitAttributeLifecycle(
			txCtx,
			command,
			actorID,
			sessionID,
			&preview,
		)
		return commitErr
	})
	if err != nil {
		if replayed, found, replayErr := s.loadAttributeLifecycleReceipt(ctx, command, true); replayErr != nil || found {
			return replayed, replayErr
		}
		if errors.Is(err, ports.ErrAttributeDefinitionVersionConflict) {
			if current, loadErr := s.attrDefRepo.Get(ctx, command.DefinitionID); loadErr == nil && current != nil {
				return nil, attributeVersionConflict(current.Version)
			}
			return nil, attributeVersionConflict(command.ExpectedVersion)
		}
		if errors.Is(err, errAttributeLifecyclePreviewChanged) {
			return nil, apperr.New(
				apperr.CodeConflict,
				"attribute lifecycle preview changed; review again",
				err,
			).WithDetails(map[string]any{"reason": "preview_changed"})
		}
		if errors.Is(err, attributemigration.ErrMigrationActive) {
			return nil, apperr.New(
				apperr.CodeConflict,
				"attribute migration is active",
				err,
			).WithDetails(map[string]any{"reason": "migration_active"})
		}
		return nil, err
	}
	key := ""
	if updated != nil {
		key = updated.Key
	}
	if err := s.cache.Invalidate(ctx, command.DefinitionID, key); err != nil {
		return nil, err
	}
	logger.FromContext(ctx).Info(
		"attribute definition lifecycle committed",
		zap.String("attribute_definition_id", command.DefinitionID.String()),
		zap.String("action", string(command.Action)),
		zap.Int64("version", command.ExpectedVersion+1),
	)
	return receipt, nil
}

func (s *attributeDefinitionService) scheduleAttributeMigration(
	ctx context.Context,
	command ports.ApplyAttributeLifecycleCommand,
	actorID uuid.UUID,
	sessionID uuid.UUID,
	reviewed ports.AttributeLifecyclePreview,
) (*dto.AttributeLifecycleReceipt, error) {
	if s == nil || s.migration == nil || s.operations == nil ||
		s.migrationBatchSize < 1 || s.migrationBatchSize > operation.MaxBatchSize ||
		!reviewed.RequiresMigration || reviewed.DefinitionID != command.DefinitionID ||
		reviewed.ExpectedVersion != command.ExpectedVersion ||
		reviewed.CandidateRevision == nil ||
		reviewed.CandidateRevision.ID != command.CandidateRevisionID {
		return nil, apperr.New(
			apperr.CodeInternalServer,
			"attribute migration is unavailable",
			nil,
		)
	}
	var (
		definition *entity.AttributeDefinition
		started    operation.CreateResult
		prepared   attributemigration.Request
	)
	err := s.txMgr.DoInTx(ctx, func(txCtx context.Context) error {
		var loadErr error
		definition, loadErr = s.attrDefRepo.Get(txCtx, command.DefinitionID)
		if loadErr != nil {
			return loadErr
		}
		if definition == nil || definition.Version != command.ExpectedVersion ||
			definition.ActiveRevisionID == nil || definition.ActiveRevision == nil ||
			command.Action != ports.AttributeLifecycleActivate {
			return ports.ErrAttributeDefinitionVersionConflict
		}
		var prepareErr error
		prepared, prepareErr = attributemigration.Prepare(
			txCtx,
			s.migration,
			attributemigration.Request{
				OperationID:         command.CommandID,
				CommandID:           command.CommandID,
				DefinitionID:        command.DefinitionID,
				ExpectedVersion:     command.ExpectedVersion,
				SourceRevisionID:    *definition.ActiveRevisionID,
				CandidateRevisionID: command.CandidateRevisionID,
				ActorUserID:         actorID,
				ActorSessionID:      sessionID,
				Reason:              command.Reason,
				ConfirmationHash:    hex.EncodeToString(command.RequestHash[:]),
			},
		)
		if prepareErr != nil {
			return prepareErr
		}
		if prepared.SourceCount != reviewed.SourceValueCount ||
			prepared.TargetCount != reviewed.TargetValueCount {
			// The confirmation digest binds the operator to exact affected-row
			// counts. Prepare owns the authoritative definition lock and frozen
			// UUID cutoffs, so it must close the final preview/commit race rather
			// than silently scheduling a differently-sized operation.
			return errAttributeLifecyclePreviewChanged
		}
		encoded, requestHash, encodeErr := operation.EncodeRequest(prepared)
		if encodeErr != nil {
			return encodeErr
		}
		checkpoint, encodeErr := json.Marshal(attributemigration.Checkpoint{})
		if encodeErr != nil {
			return encodeErr
		}
		session := sessionID
		started, prepareErr = s.operations.StartInTransaction(
			txCtx,
			operation.StartRequest{
				Operation: operation.CreateCommand{
					ID:             prepared.OperationID,
					Kind:           attributemigration.OperationKind,
					IdempotencyKey: command.CommandID,
					RequestHash:    requestHash,
					Request:        encoded,
					Total:          prepared.TargetCount + prepared.SourceCount,
					BatchSize:      s.migrationBatchSize,
					Checkpoint:     checkpoint,
					ActorUserID:    actorID,
					ActorSessionID: &session,
					Reason:         command.Reason,
					CorrelationID:  correlation.FromContext(txCtx),
				},
				Audit: audit.Entry{
					Action:   "identity.attribute_definition.migration.start",
					Resource: "identity.attribute_migration",
					Before:   map[string]any{},
					After: map[string]any{
						"definition_id":         command.DefinitionID.String(),
						"source_revision_id":    prepared.SourceRevisionID.String(),
						"candidate_revision_id": prepared.CandidateRevisionID.String(),
						"source_value_count":    prepared.SourceCount,
						"target_value_count":    prepared.TargetCount,
					},
				},
			},
		)
		if prepareErr != nil {
			return prepareErr
		}
		if started.Operation.ID != prepared.OperationID {
			return operation.ErrConflict
		}
		if started.Replay {
			return nil
		}
		payload, encodeErr := json.Marshal(map[string]any{
			"command_id":            command.CommandID.String(),
			"operation_id":          prepared.OperationID.String(),
			"definition_id":         prepared.DefinitionID.String(),
			"expected_version":      prepared.ExpectedVersion,
			"source_revision_id":    prepared.SourceRevisionID.String(),
			"candidate_revision_id": prepared.CandidateRevisionID.String(),
			"source_value_count":    prepared.SourceCount,
			"target_value_count":    prepared.TargetCount,
		})
		if encodeErr != nil {
			return encodeErr
		}
		return s.outbox.Append(txCtx, eventing.Event{
			ID:               attributeMigrationStartedEventID(command.CommandID),
			Type:             "identity.attribute_definition.migration_started.v1",
			AggregateType:    "attribute_migration",
			AggregateID:      prepared.OperationID,
			AggregateVersion: 1,
			Payload:          payload,
			OccurredAt:       started.Operation.CreatedAt.UTC(),
		})
	})
	if err != nil {
		if replayed, found, replayErr := s.loadAttributeLifecycleReceipt(ctx, command, true); replayErr != nil || found {
			return replayed, replayErr
		}
		switch {
		case errors.Is(err, errAttributeLifecyclePreviewChanged):
			return nil, apperr.New(
				apperr.CodeConflict,
				"attribute lifecycle preview changed; review again",
				err,
			).WithDetails(map[string]any{"reason": "preview_changed"})
		case errors.Is(err, ports.ErrAttributeDefinitionVersionConflict),
			errors.Is(err, attributemigration.ErrDefinitionDrift),
			errors.Is(err, attributemigration.ErrMigrationActive),
			errors.Is(err, operation.ErrConflict):
			return nil, apperr.New(
				apperr.CodeConflict,
				"attribute migration conflicts with current state",
				err,
			).WithDetails(map[string]any{
				"definition_id": command.DefinitionID.String(),
				"reason":        "migration_conflict",
			})
		default:
			return nil, err
		}
	}
	projected, err := operation.Project(started.Operation)
	if err != nil || definition == nil || definition.ActiveRevision == nil {
		return nil, apperr.New(
			apperr.CodeInternalServer,
			"attribute migration returned an invalid operation",
			err,
		)
	}
	logger.FromContext(ctx).Info(
		"attribute definition migration scheduled",
		zap.String("attribute_definition_id", command.DefinitionID.String()),
		zap.String("operation_id", projected.ID.String()),
		zap.Int64("source_value_count", prepared.SourceCount),
		zap.Int64("target_value_count", prepared.TargetCount),
		zap.Bool("replayed", started.Replay),
	)
	return &dto.AttributeLifecycleReceipt{
		Status:           dto.AttributeLifecycleMigrationScheduled,
		Action:           string(command.Action),
		CommandID:        command.CommandID,
		DefinitionID:     command.DefinitionID,
		Version:          definition.Version,
		Definition:       ToAttributeDefinitionResponse(definition),
		ActiveRevision:   ToAttributeDefinitionRevisionResponse(definition.ActiveRevision),
		Operation:        &projected,
		RecordedAt:       projected.CreatedAt.UTC(),
		IdempotentReplay: started.Replay,
	}, nil
}

func (s *attributeDefinitionService) commitAttributeLifecycle(
	ctx context.Context,
	command ports.ApplyAttributeLifecycleCommand,
	actorID uuid.UUID,
	sessionID uuid.UUID,
	reviewed *ports.AttributeLifecyclePreview,
) (*dto.AttributeLifecycleReceipt, *entity.AttributeDefinition, error) {
	fence, err := s.profileSchema.LockExclusive(ctx)
	if err != nil {
		return nil, nil, err
	}
	before, err := s.attrDefRepo.Get(ctx, command.DefinitionID)
	if err != nil {
		return nil, nil, err
	}
	if before == nil || before.Version != command.ExpectedVersion ||
		before.ActiveRevisionID == nil || before.ActiveRevision == nil {
		return nil, nil, ports.ErrAttributeDefinitionVersionConflict
	}
	if command.Action == ports.AttributeLifecycleArchive && before.RequiredOnOnboarding {
		return nil, nil, ports.ErrAttributeDefinitionVersionConflict
	}
	migrationActivation := false
	if s.migration != nil {
		migration, migrationErr := s.migration.ActiveMigrationTarget(ctx, before)
		if migrationErr != nil {
			return nil, nil, migrationErr
		}
		if migration != nil && (command.Action != ports.AttributeLifecycleActivate ||
			migration.OperationID != command.CommandID ||
			migration.CandidateRevisionID != command.CandidateRevisionID ||
			migration.ExpectedVersion != command.ExpectedVersion) {
			return nil, nil, attributemigration.ErrMigrationActive
		}
		migrationActivation = migration != nil
	}
	var active *entity.AttributeDefinitionRevision
	if command.Action == ports.AttributeLifecycleActivate {
		active, err = s.attrDefRepo.GetRevision(ctx, command.CandidateRevisionID)
		if err != nil {
			return nil, nil, err
		}
		if active == nil || active.DefinitionID != command.DefinitionID {
			return nil, nil, ports.ErrAttributeDefinitionVersionConflict
		}
		if err := validateRequiredOnboardingRevision(
			before.RequiredOnOnboarding,
			active.Visibility,
			active.UserEditable,
		); err != nil {
			return nil, nil, ports.ErrAttributeDefinitionVersionConflict
		}
	}
	var updated *entity.AttributeDefinition
	if command.Action == ports.AttributeLifecycleActivate {
		updated, err = s.attrDefRepo.ActivateRevision(
			ctx,
			command.DefinitionID,
			command.ExpectedVersion,
			*before.ActiveRevisionID,
			active,
			!migrationActivation,
		)
	} else {
		updated, err = s.attrDefRepo.Archive(
			ctx,
			command.DefinitionID,
			command.ExpectedVersion,
		)
	}
	if err != nil {
		return nil, nil, err
	}
	if updated == nil || updated.Version != command.ExpectedVersion+1 {
		return nil, nil, ports.ErrAttributeDefinitionVersionConflict
	}
	if reviewed != nil {
		if reviewed.Action != command.Action ||
			reviewed.DefinitionID != command.DefinitionID ||
			reviewed.ExpectedVersion != command.ExpectedVersion ||
			reviewed.RequiresMigration {
			return nil, nil, errAttributeLifecyclePreviewChanged
		}
		sourceCount, countErr := s.attrDefRepo.CountValues(
			ctx,
			command.DefinitionID,
			*before.ActiveRevisionID,
		)
		if countErr != nil {
			return nil, nil, countErr
		}
		if sourceCount != reviewed.SourceValueCount {
			return nil, nil, errAttributeLifecyclePreviewChanged
		}
		if command.Action == ports.AttributeLifecycleActivate {
			targetCount, countErr := s.attrDefRepo.CountValues(
				ctx,
				command.DefinitionID,
				command.CandidateRevisionID,
			)
			if countErr != nil {
				return nil, nil, countErr
			}
			if targetCount != reviewed.TargetValueCount {
				return nil, nil, errAttributeLifecyclePreviewChanged
			}
		}
	}
	if active != nil {
		updated.ActiveRevisionID = &active.ID
		updated.DraftRevisionID = &active.ID
		updated.ActiveRevision = active
		updated.DraftRevision = active
	} else {
		updated.ActiveRevision = before.ActiveRevision
		updated.DraftRevision = before.DraftRevision
	}
	if command.Action == ports.AttributeLifecycleActivate && before.RequiredOnOnboarding {
		if _, err := s.profileSchema.Advance(ctx, fence); err != nil {
			return nil, nil, err
		}
	}
	now := time.Now().UTC()
	auditID, err := s.audit.Append(ctx, audit.Entry{
		ActorUserID:    &actorID,
		ActorSessionID: &sessionID,
		Action:         "identity.attribute_definition." + string(command.Action),
		Resource:       "identity.attribute_definition",
		ResourceID:     command.DefinitionID.String(),
		Reason:         command.Reason,
		Before:         attributeDefinitionAudit(before, before.ActiveRevision),
		After:          attributeDefinitionAudit(updated, activeOrCurrent(active, before.ActiveRevision)),
		OccurredAt:     now,
	})
	if err != nil {
		return nil, nil, err
	}
	eventID := command.CommandID
	receipt := &dto.AttributeLifecycleReceipt{
		Status:         dto.AttributeLifecycleCommitted,
		Action:         string(command.Action),
		CommandID:      command.CommandID,
		EventID:        &eventID,
		DefinitionID:   command.DefinitionID,
		Version:        updated.Version,
		Definition:     ToAttributeDefinitionResponse(updated),
		ActiveRevision: ToAttributeDefinitionRevisionResponse(active),
		RecordedAt:     now,
	}
	payload, err := json.Marshal(attributeLifecycleReceiptPayload{
		Status:              receipt.Status,
		Action:              receipt.Action,
		DefinitionID:        receipt.DefinitionID.String(),
		CandidateRevisionID: canonicalOptionalAttributeUUID(command.CandidateRevisionID),
		ExpectedVersion:     command.ExpectedVersion,
		ResultingVersion:    receipt.Version,
		CommandID:           command.CommandID.String(),
		RequestHash:         hex.EncodeToString(command.RequestHash[:]),
		AuditID:             auditID.String(),
		Definition:          receipt.Definition,
		ActiveRevision:      receipt.ActiveRevision,
	})
	if err != nil {
		return nil, nil, err
	}
	err = s.outbox.Append(ctx, eventing.Event{
		ID:               command.CommandID,
		Type:             "identity.attribute_definition." + string(command.Action) + "d.v1",
		AggregateType:    "attribute_definition",
		AggregateID:      command.DefinitionID,
		AggregateVersion: updated.Version,
		Payload:          payload,
		OccurredAt:       now,
	})
	if err != nil {
		return nil, nil, err
	}
	return receipt, updated, nil
}

// ActivateMigratedRevision is invoked only by the typed operation executor in
// its final transaction, after VerifyFinal has locked and proven the entire
// source/candidate relation.
func (s *attributeDefinitionService) ActivateMigratedRevision(
	ctx context.Context,
	request attributemigration.Request,
) error {
	if s == nil || s.attrDefRepo == nil || s.audit == nil || s.outbox == nil ||
		s.profileSchema == nil || attributemigration.ValidateRequest(request) != nil {
		return attributemigration.ErrInvalidMigration
	}
	hashBytes, err := hex.DecodeString(request.ConfirmationHash)
	if err != nil || len(hashBytes) != sha256.Size {
		return attributemigration.ErrInvalidMigration
	}
	var requestHash [sha256.Size]byte
	copy(requestHash[:], hashBytes)
	command := ports.ApplyAttributeLifecycleCommand{
		Action:              ports.AttributeLifecycleActivate,
		DefinitionID:        request.DefinitionID,
		CandidateRevisionID: request.CandidateRevisionID,
		ExpectedVersion:     request.ExpectedVersion,
		CommandID:           request.CommandID,
		RequestHash:         requestHash,
		Reason:              request.Reason,
	}
	_, updated, err := s.commitAttributeLifecycle(
		ctx,
		command,
		request.ActorUserID,
		request.ActorSessionID,
		nil,
	)
	if err != nil {
		if errors.Is(err, ports.ErrAttributeDefinitionVersionConflict) {
			return attributemigration.ErrDefinitionDrift
		}
		return err
	}
	if updated == nil {
		return attributemigration.ErrDefinitionDrift
	}
	definitionID := request.DefinitionID
	definitionKey := updated.Key
	if err := s.cache.Invalidate(ctx, definitionID, definitionKey); err != nil {
		return err
	}
	logger.FromContext(ctx).Info(
		"attribute definition migration activated",
		zap.String("attribute_definition_id", definitionID.String()),
		zap.String("operation_id", request.OperationID.String()),
		zap.Int64("version", request.ExpectedVersion+1),
	)
	return nil
}

func attributeMigrationStartedEventID(commandID uuid.UUID) uuid.UUID {
	return uuid.NewSHA1(
		uuid.NameSpaceURL,
		[]byte("judgify/identity/attribute-definition/migration-started/"+commandID.String()),
	)
}

type attributeLifecycleReceiptPayload struct {
	Status              string                                   `json:"status"`
	Action              string                                   `json:"action"`
	DefinitionID        string                                   `json:"definition_id"`
	CandidateRevisionID string                                   `json:"candidate_revision_id,omitempty"`
	ExpectedVersion     int64                                    `json:"expected_version"`
	ResultingVersion    int64                                    `json:"resulting_version"`
	CommandID           string                                   `json:"command_id"`
	RequestHash         string                                   `json:"request_hash"`
	AuditID             string                                   `json:"audit_id"`
	Definition          *dto.AttributeDefinitionResponse         `json:"definition"`
	ActiveRevision      *dto.AttributeDefinitionRevisionResponse `json:"active_revision,omitempty"`
}

func (s *attributeDefinitionService) loadAttributeLifecycleReceipt(
	ctx context.Context,
	command ports.ApplyAttributeLifecycleCommand,
	replayed bool,
) (*dto.AttributeLifecycleReceipt, bool, error) {
	stored, found, err := s.attrDefRepo.LoadCommandReceipt(ctx, command.CommandID)
	if err != nil || !found {
		return nil, found, err
	}
	expectedType := "identity.attribute_definition." + string(command.Action) + "d.v1"
	var payload attributeLifecycleReceiptPayload
	if stored.EventID != command.CommandID || stored.EventType != expectedType ||
		stored.AggregateID != command.DefinitionID ||
		stored.AggregateVersion != command.ExpectedVersion+1 || stored.OccurredAt.IsZero() ||
		json.Unmarshal(stored.Payload, &payload) != nil ||
		payload.Status != dto.AttributeLifecycleCommitted ||
		payload.Action != string(command.Action) ||
		payload.DefinitionID != command.DefinitionID.String() ||
		payload.CandidateRevisionID != canonicalOptionalAttributeUUID(command.CandidateRevisionID) ||
		payload.ExpectedVersion != command.ExpectedVersion ||
		payload.ResultingVersion != stored.AggregateVersion ||
		payload.CommandID != command.CommandID.String() ||
		payload.RequestHash != hex.EncodeToString(command.RequestHash[:]) ||
		!canonicalAttributeReceiptUUID(payload.AuditID) || payload.Definition == nil ||
		payload.Definition.ID != command.DefinitionID ||
		payload.Definition.Version != stored.AggregateVersion {
		return nil, false, attributeVersionConflict(stored.AggregateVersion)
	}
	if command.Action == ports.AttributeLifecycleActivate {
		if payload.ActiveRevision == nil ||
			payload.ActiveRevision.ID != command.CandidateRevisionID {
			return nil, false, attributeVersionConflict(stored.AggregateVersion)
		}
	} else if payload.ActiveRevision != nil || payload.Definition.Status != "archived" {
		return nil, false, attributeVersionConflict(stored.AggregateVersion)
	}
	eventID := stored.EventID
	return &dto.AttributeLifecycleReceipt{
		Status:           dto.AttributeLifecycleCommitted,
		Action:           payload.Action,
		CommandID:        command.CommandID,
		EventID:          &eventID,
		DefinitionID:     command.DefinitionID,
		Version:          stored.AggregateVersion,
		Definition:       payload.Definition,
		ActiveRevision:   payload.ActiveRevision,
		RecordedAt:       stored.OccurredAt.UTC(),
		IdempotentReplay: replayed,
	}, true, nil
}

func activeOrCurrent(
	active *entity.AttributeDefinitionRevision,
	current *entity.AttributeDefinitionRevision,
) *entity.AttributeDefinitionRevision {
	if active != nil {
		return active
	}
	return current
}

func canonicalOptionalAttributeUUID(value uuid.UUID) string {
	if value == uuid.Nil {
		return ""
	}
	return value.String()
}

func canonicalAttributeReceiptUUID(value string) bool {
	id, err := uuid.Parse(value)
	return err == nil && id != uuid.Nil && id.String() == value
}

type attributeDefinitionMutationReceiptPayload struct {
	Action      string                           `json:"action"`
	CommandID   string                           `json:"command_id"`
	RequestHash string                           `json:"request_hash"`
	Definition  *dto.AttributeDefinitionResponse `json:"definition"`
}

func (s *attributeDefinitionService) loadAttributeDefinitionMutationReceipt(
	ctx context.Context,
	action string,
	commandID uuid.UUID,
	definitionID uuid.UUID,
	aggregateVersion int64,
	requestHash [sha256.Size]byte,
	expectedKey string,
) (*dto.AttributeDefinitionResponse, bool, error) {
	stored, found, err := s.attrDefRepo.LoadCommandReceipt(ctx, commandID)
	if err != nil || !found {
		return nil, found, err
	}
	eventType := "identity.attribute_definition.created.v1"
	if action == "draft" {
		eventType = "identity.attribute_definition.draft_created.v1"
	}
	var payload attributeDefinitionMutationReceiptPayload
	if (action != "create" && action != "draft") ||
		stored.EventID != commandID || stored.EventType != eventType ||
		stored.AggregateVersion != aggregateVersion || stored.OccurredAt.IsZero() ||
		json.Unmarshal(stored.Payload, &payload) != nil || payload.Action != action ||
		payload.CommandID != commandID.String() ||
		payload.RequestHash != hex.EncodeToString(requestHash[:]) ||
		payload.Definition == nil || payload.Definition.ID != stored.AggregateID ||
		payload.Definition.Version != aggregateVersion ||
		(definitionID != uuid.Nil && stored.AggregateID != definitionID) ||
		(expectedKey != "" && payload.Definition.Key != expectedKey) {
		return nil, false, attributeVersionConflict(stored.AggregateVersion)
	}
	return payload.Definition, true, nil
}

func attributeDefinitionCommandHash(
	domain string,
	document any,
) ([sha256.Size]byte, error) {
	if strings.TrimSpace(domain) != domain || domain == "" || document == nil {
		return [sha256.Size]byte{}, fmt.Errorf("invalid attribute command hash")
	}
	payload, err := json.Marshal(document)
	if err != nil || len(payload) == 0 || len(payload) > 64<<10 {
		return [sha256.Size]byte{}, fmt.Errorf("invalid attribute command hash")
	}
	framed := make([]byte, 0, len(domain)+1+len(payload))
	framed = append(framed, domain...)
	framed = append(framed, 0)
	framed = append(framed, payload...)
	return sha256.Sum256(framed), nil
}

func normalizeOptionalAttributeString(value *string) {
	if value != nil {
		*value = strings.TrimSpace(*value)
	}
}
