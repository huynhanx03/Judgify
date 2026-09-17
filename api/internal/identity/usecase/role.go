package usecase

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/huynhanx03/go-common/pkg/common/apperr"
	"github.com/huynhanx03/go-common/pkg/common/tx"
	d "github.com/huynhanx03/go-common/pkg/dto"
	"github.com/huynhanx03/go-common/pkg/logger"
	"go.uber.org/zap"

	"github.com/huynhanx03/judgify/internal/authctx"
	"github.com/huynhanx03/judgify/internal/constant"
	dto "github.com/huynhanx03/judgify/internal/identity/contract"
	entity "github.com/huynhanx03/judgify/internal/identity/domain"
	"github.com/huynhanx03/judgify/internal/platform/audit"
	"github.com/huynhanx03/judgify/internal/platform/eventing"
	"github.com/huynhanx03/judgify/internal/ports"
)

type roleService struct {
	roleRepo     ports.RoleRepository
	authzService ports.AuthorizationService
	cacheService ports.CacheService
	txMgr        tx.Manager
	audit        audit.Appender
	outbox       eventing.OutboxAppender
	cache        ports.RoleCache
}

type RoleServiceOption func(*roleService)

func WithRoleLifecycle(auditAppender audit.Appender, outboxAppender eventing.OutboxAppender) RoleServiceOption {
	return func(service *roleService) {
		service.audit = auditAppender
		service.outbox = outboxAppender
	}
}

// NewRoleService creates a new RoleService instance.
func NewRoleService(
	roleRepo ports.RoleRepository,
	authzService ports.AuthorizationService,
	cacheService ports.CacheService,
	txMgr tx.Manager,
	cache ports.RoleCache,
	options ...RoleServiceOption,
) ports.RoleService {
	service := &roleService{
		roleRepo:     roleRepo,
		authzService: authzService,
		cacheService: cacheService,
		txMgr:        txMgr,
		cache:        cache,
	}
	for _, option := range options {
		if option != nil {
			option(service)
		}
	}
	return service
}

// FindAll retrieves all roles without pagination.
func (s *roleService) FindAll(ctx context.Context) ([]*dto.RoleResponse, error) {
	return s.cache.FindAll(ctx, func(ctx context.Context) ([]*dto.RoleResponse, error) {
		logger.FromContext(ctx).Debug("cache miss, loading roles from db")
		roles, err := s.roleRepo.FindAll(ctx)
		if err != nil {
			return nil, err
		}
		responses := make([]*dto.RoleResponse, len(roles))
		for i, r := range roles {
			responses[i] = ToRoleResponse(r)
		}
		return responses, nil
	})
}

// Find retrieves roles with pagination.
func (s *roleService) Find(ctx context.Context, opts *d.QueryOptions) (*d.Paginated[*dto.RoleResponse], error) {
	roles, err := s.roleRepo.Find(ctx, opts)
	if err != nil {
		return nil, err
	}

	if roles.Records == nil {
		return &d.Paginated[*dto.RoleResponse]{
			Records:    &[]*dto.RoleResponse{},
			Pagination: roles.Pagination,
		}, nil
	}

	entities := *roles.Records
	responses := make([]*dto.RoleResponse, len(entities))
	for i, role := range entities {
		responses[i] = ToRoleResponse(role)
	}

	return &d.Paginated[*dto.RoleResponse]{
		Records:    &responses,
		Pagination: roles.Pagination,
	}, nil
}

// Get retrieves a role by ID.
func (s *roleService) Get(ctx context.Context, id uuid.UUID) (*dto.RoleResponse, error) {
	return s.cache.Get(ctx, id, func(ctx context.Context) (*dto.RoleResponse, error) {
		logger.FromContext(ctx).Debug("cache miss, loading role from db", zap.String("role_id", id.String()))
		role, err := s.roleRepo.Get(ctx, id)
		if err != nil {
			return nil, err
		}
		return ToRoleResponse(role), nil
	})
}

func (s *roleService) Create(ctx context.Context, req *dto.CreateRoleRequest, requestHash [sha256.Size]byte) (*dto.RoleMutationReceipt, error) {
	if err := s.validateLifecycle(ctx); err != nil {
		return nil, err
	}
	if !validCreateRoleRequest(req, requestHash) {
		return nil, invalidRoleCommand("invalid role command")
	}

	const (
		action    = "create"
		eventType = "identity.role.metadata.created.v1"
	)
	actorID, sessionID, err := roleActor(ctx)
	if err != nil {
		return nil, err
	}
	receipt, found, err := s.loadReceipt(
		ctx,
		req.CommandID,
		eventType,
		action,
		uuid.Nil,
		1,
		requestHash,
		true,
	)
	if err != nil {
		return nil, err
	}
	if found {
		s.invalidateRole(ctx, receipt.Role.ID, receipt.Role.Key)
		return receipt, nil
	}

	role := ToRoleEntityFromCreate(req)
	var result *dto.RoleMutationReceipt
	err = s.txMgr.DoInTx(ctx, func(txCtx context.Context) error {
		if err := s.roleRepo.Create(txCtx, role); err != nil {
			return err
		}
		now := time.Now().UTC()
		response := *ToRoleResponse(role)
		if response.ID == uuid.Nil || response.Version != 1 ||
			!validRoleReceiptProjection(action, response) {
			return ports.ErrInvalidRoleCommand
		}
		auditID, err := s.audit.Append(txCtx, audit.Entry{
			ActorUserID:    &actorID,
			ActorSessionID: &sessionID,
			Action:         "identity.role.metadata.create",
			Resource:       "identity.role",
			ResourceID:     role.ID.String(),
			Reason:         req.Reason,
			Before:         map[string]any{},
			After:          roleAudit(&response),
			OccurredAt:     now,
		})
		if err != nil {
			return err
		}
		payload, err := json.Marshal(roleReceiptPayload{
			Action:      action,
			CommandID:   req.CommandID.String(),
			RequestHash: hex.EncodeToString(requestHash[:]),
			AuditID:     auditID.String(),
			Role:        response,
		})
		if err != nil {
			return err
		}
		if err := s.outbox.Append(txCtx, eventing.Event{
			ID:               req.CommandID,
			Type:             eventType,
			AggregateType:    "role",
			AggregateID:      role.ID,
			AggregateVersion: response.Version,
			Payload:          payload,
			OccurredAt:       now,
		}); err != nil {
			return err
		}
		result = &dto.RoleMutationReceipt{
			CommandID:   req.CommandID,
			EventID:     req.CommandID,
			Action:      action,
			Role:        response,
			CommittedAt: now,
		}
		return nil
	})
	if err != nil {
		receipt, found, replayErr := s.loadReceipt(
			ctx,
			req.CommandID,
			eventType,
			action,
			uuid.Nil,
			1,
			requestHash,
			true,
		)
		if replayErr != nil {
			return nil, replayErr
		}
		if found {
			s.invalidateRole(ctx, receipt.Role.ID, receipt.Role.Key)
			return receipt, nil
		}
		return nil, err
	}
	if result == nil {
		return nil, apperr.New(apperr.CodeInternalServer, "role lifecycle unavailable", nil)
	}
	s.invalidateRole(ctx, role.ID, role.Key)
	return result, nil
}

func (s *roleService) PreviewMetadata(ctx context.Context, id uuid.UUID, action string, req *dto.UpdateRoleRequest) (*dto.RoleMetadataPreview, error) {
	if err := s.validateLifecycle(ctx); err != nil {
		return nil, err
	}
	if err := validateRoleMetadataRequest(id, action, req, false); err != nil {
		return nil, err
	}
	role, err := s.roleRepo.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	if role.Version != req.ExpectedVersion {
		return nil, roleConflict(role.Version)
	}
	before := ToRoleResponse(role)
	after, err := roleMetadataAfter(role, action, req)
	if err != nil {
		return nil, err
	}
	return &dto.RoleMetadataPreview{
		Action:           action,
		RoleID:           id,
		ExpectedVersion:  role.Version,
		ResultingVersion: role.Version + 1,
		Before:           *before,
		After:            *after,
	}, nil
}

func (s *roleService) ApplyMetadata(
	ctx context.Context,
	id uuid.UUID,
	action string,
	req *dto.UpdateRoleRequest,
	requestHash [sha256.Size]byte,
) (*dto.RoleMutationReceipt, error) {
	if err := s.validateLifecycle(ctx); err != nil {
		return nil, err
	}
	if err := validateRoleMetadataRequest(id, action, req, true); err != nil {
		return nil, err
	}
	if requestHash == ([sha256.Size]byte{}) {
		return nil, invalidRoleCommand("invalid role command")
	}
	eventType := "identity.role.metadata.updated.v1"
	if action == "archive" {
		eventType = "identity.role.metadata.archived.v1"
	}
	actorID, sessionID, err := roleActor(ctx)
	if err != nil {
		return nil, err
	}
	receipt, found, err := s.loadReceipt(
		ctx,
		req.CommandID,
		eventType,
		action,
		id,
		req.ExpectedVersion+1,
		requestHash,
		true,
	)
	if err != nil {
		return nil, err
	}
	if found {
		s.invalidateRole(ctx, id, receipt.Role.Key)
		return receipt, nil
	}
	var result *dto.RoleMutationReceipt
	err = s.txMgr.DoInTx(ctx, func(txCtx context.Context) error {
		role, err := s.roleRepo.Get(txCtx, id)
		if err != nil {
			return err
		}
		if role.Version != req.ExpectedVersion {
			return roleConflict(role.Version)
		}
		before := *ToRoleResponse(role)
		after, err := roleMetadataAfter(role, action, req)
		if err != nil {
			return err
		}
		role.Name = after.Name
		role.Description = after.Description
		role.Status = entity.RoleStatus(after.Status)
		role.Version = after.Version
		if err := s.roleRepo.UpdateCAS(txCtx, role, req.ExpectedVersion); err != nil {
			return err
		}
		if action == "archive" {
			// Archiving the catalog row without deleting its p/g references would
			// leave the live runtime granting access and make the next snapshot
			// fail active-role validation. The nested authorization unit joins this
			// physical transaction and publishes only after the outer commit.
			if err := s.authzService.DeleteRole(txCtx, role.Key); err != nil {
				return err
			}
		}
		now := time.Now().UTC()
		auditID, err := s.audit.Append(txCtx, audit.Entry{
			ActorUserID:    &actorID,
			ActorSessionID: &sessionID,
			Action:         "identity.role.metadata." + action,
			Resource:       "identity.role",
			ResourceID:     id.String(),
			Reason:         req.Reason,
			Before:         roleAudit(&before),
			After:          roleAudit(after),
			OccurredAt:     now,
		})
		if err != nil {
			return err
		}
		payload, err := json.Marshal(roleReceiptPayload{
			Action:      action,
			CommandID:   req.CommandID.String(),
			RequestHash: hex.EncodeToString(requestHash[:]),
			AuditID:     auditID.String(),
			Role:        *after,
		})
		if err != nil {
			return err
		}
		if err := s.outbox.Append(txCtx, eventing.Event{
			ID:               req.CommandID,
			Type:             eventType,
			AggregateType:    "role",
			AggregateID:      id,
			AggregateVersion: after.Version,
			Payload:          payload,
			OccurredAt:       now,
		}); err != nil {
			return err
		}
		result = &dto.RoleMutationReceipt{
			CommandID:   req.CommandID,
			EventID:     req.CommandID,
			Action:      action,
			Role:        *after,
			CommittedAt: now,
		}
		return nil
	})
	if err != nil {
		receipt, found, replayErr := s.loadReceipt(
			ctx,
			req.CommandID,
			eventType,
			action,
			id,
			req.ExpectedVersion+1,
			requestHash,
			true,
		)
		if replayErr != nil {
			return nil, replayErr
		}
		if found {
			s.invalidateRole(ctx, id, receipt.Role.Key)
			return receipt, nil
		}
		if errors.Is(err, ports.ErrRoleVersionConflict) {
			return nil, s.currentRoleConflict(ctx, id)
		}
		return nil, err
	}
	if result == nil {
		return nil, apperr.New(apperr.CodeInternalServer, "role lifecycle unavailable", nil)
	}
	s.invalidateRole(ctx, id, result.Role.Key)
	return result, nil
}

type roleReceiptPayload struct {
	Action      string           `json:"action"`
	CommandID   string           `json:"command_id"`
	RequestHash string           `json:"request_hash"`
	AuditID     string           `json:"audit_id"`
	Role        dto.RoleResponse `json:"role"`
}

func (s *roleService) validateLifecycle(ctx context.Context) error {
	if s == nil || s.roleRepo == nil || s.authzService == nil ||
		s.cacheService == nil || s.txMgr == nil || s.audit == nil ||
		s.outbox == nil || s.cache == nil || ctx == nil || ctx.Err() != nil {
		return apperr.New(apperr.CodeInternalServer, "role lifecycle unavailable", nil)
	}
	return nil
}

func roleActor(ctx context.Context) (uuid.UUID, uuid.UUID, error) {
	actorID, actorOK := authctx.UserID(ctx)
	if !actorOK || actorID == uuid.Nil {
		return uuid.Nil, uuid.Nil, apperr.New(
			apperr.CodeUnauthorized,
			"authentication required",
			nil,
		)
	}
	sessionID, sessionOK := authctx.SessionID(ctx)
	if !sessionOK || sessionID == uuid.Nil {
		return uuid.Nil, uuid.Nil, apperr.New(
			apperr.CodeUnauthorized,
			"session required",
			nil,
		)
	}
	return actorID, sessionID, nil
}

func validCreateRoleRequest(
	req *dto.CreateRoleRequest,
	requestHash [sha256.Size]byte,
) bool {
	return req != nil && req.CommandID != uuid.Nil && req.ExpectedVersion == 1 &&
		requestHash != ([sha256.Size]byte{}) && validRoleKey(req.Key) && len(req.Key) <= 50 &&
		req.Key != protectedSuperadminRoleKey && req.Key != constant.DefaultUserRoleKey &&
		validRoleReason(req.Reason) &&
		strings.TrimSpace(req.Name) == req.Name && len(req.Name) >= 2 && len(req.Name) <= 50 &&
		strings.TrimSpace(req.Description) == req.Description && len(req.Description) <= 255
}

func validateRoleMetadataRequest(
	id uuid.UUID,
	action string,
	req *dto.UpdateRoleRequest,
	apply bool,
) error {
	if id == uuid.Nil || req == nil || req.ID != id || req.CommandID == uuid.Nil ||
		req.ExpectedVersion < 1 || !validRoleReason(req.Reason) ||
		(apply && (req.ConfirmationToken == "" || len(req.ConfirmationToken) > 2048)) ||
		(action != "update" && action != "archive") {
		return invalidRoleCommand("invalid role command")
	}
	if action == "update" && req.Name == nil && req.Description == nil {
		return invalidRoleCommand("role update is empty")
	}
	if action == "archive" && (req.Name != nil || req.Description != nil) {
		return invalidRoleCommand("invalid role archive command")
	}
	if req.Name != nil &&
		(strings.TrimSpace(*req.Name) != *req.Name || len(*req.Name) < 2 || len(*req.Name) > 50) {
		return invalidRoleCommand("invalid role name")
	}
	if req.Description != nil &&
		(strings.TrimSpace(*req.Description) != *req.Description || len(*req.Description) > 255) {
		return invalidRoleCommand("invalid role description")
	}
	return nil
}

func validRoleReason(reason string) bool {
	if !utf8.ValidString(reason) || strings.TrimSpace(reason) != reason ||
		len(reason) < 3 || len(reason) > 1024 {
		return false
	}
	for _, current := range reason {
		if unicode.IsControl(current) && current != '\n' && current != '\t' {
			return false
		}
	}
	return true
}

func roleMetadataAfter(
	role *entity.Role,
	action string,
	req *dto.UpdateRoleRequest,
) (*dto.RoleResponse, error) {
	if role == nil || role.ID == uuid.Nil || role.Version < 1 || role.IsSystem ||
		role.Key == protectedSuperadminRoleKey || role.Status != entity.RoleStatusActive {
		return nil, apperr.New(
			apperr.CodeConflict,
			"role cannot be changed",
			ports.ErrInvalidRoleCommand,
		)
	}
	after := *ToRoleResponse(role)
	after.Version++
	if action == "archive" {
		after.Status = string(entity.RoleStatusArchived)
		if !validRoleReceiptProjection(action, after) {
			return nil, invalidRoleCommand("invalid role projection")
		}
		return &after, nil
	}
	if req.Name != nil {
		after.Name = *req.Name
	}
	if req.Description != nil {
		after.Description = *req.Description
	}
	if after.Name == role.Name && after.Description == role.Description {
		return nil, invalidRoleCommand("role metadata is unchanged")
	}
	if !validRoleReceiptProjection(action, after) {
		return nil, invalidRoleCommand("invalid role projection")
	}
	return &after, nil
}

func invalidRoleCommand(message string) error {
	return apperr.New(apperr.CodeBadRequest, message, ports.ErrInvalidRoleCommand)
}

func roleConflict(version int64) error {
	errorValue := apperr.New(
		apperr.CodeConflict,
		"role changed; reload and preview again",
		ports.ErrRoleVersionConflict,
	)
	if version > 0 {
		return errorValue.WithDetails(map[string]any{"current_version": version})
	}
	return errorValue
}

func (s *roleService) currentRoleConflict(ctx context.Context, id uuid.UUID) error {
	role, err := s.roleRepo.Get(ctx, id)
	if err != nil || role == nil {
		return roleConflict(0)
	}
	return roleConflict(role.Version)
}

func roleAudit(role *dto.RoleResponse) map[string]any {
	if role == nil {
		return map[string]any{}
	}
	return map[string]any{
		"role_id":      role.ID.String(),
		"key":          role.Key,
		"name":         role.Name,
		"description":  role.Description,
		"is_system":    role.IsSystem,
		"is_protected": role.IsProtected,
		"status":       role.Status,
		"version":      role.Version,
	}
}

func (s *roleService) invalidateRole(ctx context.Context, id uuid.UUID, roleKey string) {
	if err := s.cacheService.InvalidatePermissionConfig(ctx); err != nil {
		logger.FromContext(ctx).Warn(
			"failed to invalidate permission config cache",
			zap.Error(err),
		)
	}
	if err := s.cache.Invalidate(ctx, id, roleKey); err != nil {
		logger.FromContext(ctx).Warn("failed to invalidate role metadata cache", zap.Error(err))
	}
}

func (s *roleService) loadReceipt(
	ctx context.Context,
	commandID uuid.UUID,
	eventType string,
	action string,
	roleID uuid.UUID,
	version int64,
	hash [sha256.Size]byte,
	replayed bool,
) (*dto.RoleMutationReceipt, bool, error) {
	receipt, found, err := s.roleRepo.LoadCommandReceipt(ctx, commandID)
	if err != nil || !found {
		return nil, found, err
	}
	var payload roleReceiptPayload
	unmarshalErr := json.Unmarshal(receipt.Payload, &payload)
	auditID, auditErr := uuid.Parse(payload.AuditID)
	if receipt.EventID != commandID || receipt.EventType != eventType ||
		receipt.AggregateType != "role" || receipt.AggregateID == uuid.Nil ||
		(roleID != uuid.Nil && receipt.AggregateID != roleID) ||
		receipt.AggregateVersion != version || receipt.OccurredAt.IsZero() ||
		unmarshalErr != nil || payload.CommandID != commandID.String() ||
		payload.RequestHash != hex.EncodeToString(hash[:]) || payload.Action != action ||
		auditErr != nil || auditID == uuid.Nil || payload.Role.ID != receipt.AggregateID ||
		payload.Role.Version != version || !validRoleReceiptProjection(action, payload.Role) {
		return nil, false, apperr.New(
			apperr.CodeConflict,
			"role command identity conflict",
			ports.ErrInvalidRoleCommand,
		)
	}
	return &dto.RoleMutationReceipt{
		CommandID:        commandID,
		EventID:          receipt.EventID,
		Action:           payload.Action,
		Role:             payload.Role,
		CommittedAt:      receipt.OccurredAt.UTC(),
		IdempotentReplay: replayed,
	}, true, nil
}

func validRoleReceiptProjection(action string, role dto.RoleResponse) bool {
	if role.ID == uuid.Nil || !validRoleKey(role.Key) || len(role.Key) > 50 ||
		strings.TrimSpace(role.Name) != role.Name || len(role.Name) < 2 || len(role.Name) > 50 ||
		strings.TrimSpace(role.Description) != role.Description || len(role.Description) > 255 ||
		role.IsSystem || role.IsProtected || role.Version < 1 {
		return false
	}
	if action == "archive" {
		return role.Status == string(entity.RoleStatusArchived)
	}
	return (action == "create" || action == "update") &&
		role.Status == string(entity.RoleStatusActive)
}

func (s *roleService) GetPolicies(ctx context.Context, id uuid.UUID) (*dto.RolePolicySnapshot, error) {
	role, err := s.roleRepo.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	return s.authzService.GetRolePolicies(ctx, role.Key)
}

func (s *roleService) SetPolicies(ctx context.Context, id uuid.UUID, req *dto.RolePoliciesUpdateRequest) (*dto.RolePolicySnapshot, error) {
	role, err := s.roleRepo.Get(ctx, id)
	if err != nil {
		return nil, err
	}

	var snapshot *dto.RolePolicySnapshot
	err = s.txMgr.DoInTx(ctx, func(ctx context.Context) error {
		var mutationErr error
		snapshot, mutationErr = s.authzService.SetRolePolicies(
			ctx,
			role.Key,
			req.ExpectedRevision,
			req.Reason,
			req.Rules,
		)
		return mutationErr
	})
	if err != nil {
		return nil, err
	}

	if err := s.cacheService.InvalidatePermissionConfig(ctx); err != nil {
		logger.FromContext(ctx).Warn("failed to invalidate permission config cache", zap.Error(err))
	}
	if err := s.cache.Invalidate(ctx, id, ""); err != nil {
		logger.FromContext(ctx).Warn("failed to invalidate role metadata cache", zap.Error(err))
	}

	return snapshot, nil
}
