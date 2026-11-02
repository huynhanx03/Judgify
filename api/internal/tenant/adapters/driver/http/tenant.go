package http

import (
	"errors"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/huynhanx03/go-common/pkg/common/apperr"
	"github.com/huynhanx03/go-common/pkg/common/http/response"

	"github.com/huynhanx03/judgify/internal/authctx"
	tenantdb "github.com/huynhanx03/judgify/internal/tenant/adapters/driven/db"
	"github.com/huynhanx03/judgify/internal/tenant/contract"
	"github.com/huynhanx03/judgify/internal/tenant/domain"
	"github.com/huynhanx03/judgify/internal/tenant/usecase"
)

type Handler struct{ service *usecase.Tenant }

func NewHandler(service *usecase.Tenant) *Handler { return &Handler{service: service} }

func (h *Handler) RegisterProtected(r *gin.RouterGroup) {
	if h == nil || h.service == nil || r == nil {
		return
	}
	group := r.Group("/tenants")
	group.Use(noStore)
	group.GET("", h.findForUser)
	group.POST("", h.create)
	group.PUT("/:tenantID/name", h.rename)
	group.POST("/:tenantID/invitations", h.invite)
	group.GET("/:tenantID/members", h.findMembersForOwner)
	group.POST("/:tenantID/transfer", h.transferOwnership)
	group.POST("/:tenantID/leave", h.leave)
	group.DELETE("/:tenantID/members/:userID", h.removeMember)
	r.Group("/tenant-invitations").Use(noStore).GET("", h.findPendingInvitationsForUser)
	r.Group("/tenant-invitations").Use(noStore).POST("/:invitationID/accept", h.acceptInvitation)
}

func (h *Handler) transferOwnership(c *gin.Context) {
	ownerID, ok := authctx.UserID(c.Request.Context())
	if !ok {
		h.fail(c, apperr.CodeUnauthorized, "unauthenticated", nil, nil)
		return
	}
	tenantID, err := uuid.Parse(c.Param("tenantID"))
	if err != nil || tenantID == uuid.Nil || tenantID.String() != c.Param("tenantID") {
		h.fail(c, apperr.CodeInvalidID, "invalid tenant ID", err, nil)
		return
	}
	var request struct {
		NextOwnerUserID uuid.UUID `json:"next_owner_user_id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&request); err != nil || request.NextOwnerUserID == uuid.Nil {
		h.fail(c, apperr.CodeValidationFailed, "invalid next owner", err, nil)
		return
	}
	if err := h.service.TransferOwnership(c.Request.Context(), tenantID, ownerID, request.NextOwnerUserID); err != nil {
		if errors.Is(err, tenantdb.ErrTenantOwnerRequired) {
			h.fail(c, apperr.CodeForbidden, "tenant owner required", err, nil)
			return
		}
		if errors.Is(err, domain.ErrOwnerCannotLeave) || errors.Is(err, domain.ErrMemberNotFound) {
			h.fail(c, apperr.CodeConflict, "ownership cannot be transferred", err, nil)
			return
		}
		h.fail(c, apperr.CodeInternalServer, "ownership transfer failed", err, nil)
		return
	}
	response.SuccessResponse(c, apperr.CodeSuccess, gin.H{"transferred": true})
}

func (h *Handler) findMembersForOwner(c *gin.Context) {
	ownerID, ok := authctx.UserID(c.Request.Context())
	if !ok {
		h.fail(c, apperr.CodeUnauthorized, "unauthenticated", nil, nil)
		return
	}
	tenantID, err := uuid.Parse(c.Param("tenantID"))
	if err != nil || tenantID == uuid.Nil || tenantID.String() != c.Param("tenantID") {
		h.fail(c, apperr.CodeInvalidID, "invalid tenant ID", err, nil)
		return
	}
	items, err := h.service.FindMembersForOwner(c.Request.Context(), tenantID, ownerID)
	if err != nil {
		if errors.Is(err, tenantdb.ErrTenantOwnerRequired) {
			h.fail(c, apperr.CodeForbidden, "tenant owner required", err, nil)
			return
		}
		h.fail(c, apperr.CodeInternalServer, "tenant member list failed", err, nil)
		return
	}
	result := make([]memberDTO, 0, len(items))
	for _, item := range items {
		result = append(result, memberDTO{UserID: item.UserID, Username: item.Username, Role: item.Role})
	}
	response.SuccessResponse(c, apperr.CodeSuccess, gin.H{"members": result})
}

func (h *Handler) leave(c *gin.Context) {
	userID, ok := authctx.UserID(c.Request.Context())
	if !ok {
		h.fail(c, apperr.CodeUnauthorized, "unauthenticated", nil, nil)
		return
	}
	tenantID, err := uuid.Parse(c.Param("tenantID"))
	if err != nil || tenantID == uuid.Nil || tenantID.String() != c.Param("tenantID") {
		h.fail(c, apperr.CodeInvalidID, "invalid tenant ID", err, nil)
		return
	}
	if err := h.service.Leave(c.Request.Context(), tenantID, userID); err != nil {
		if errors.Is(err, domain.ErrOwnerCannotLeave) || errors.Is(err, domain.ErrLastMembership) || errors.Is(err, domain.ErrMemberNotFound) {
			h.fail(c, apperr.CodeConflict, "tenant membership cannot be left", err, nil)
			return
		}
		h.fail(c, apperr.CodeInternalServer, "tenant leave failed", err, nil)
		return
	}
	response.SuccessResponse(c, apperr.CodeSuccess, gin.H{"left": true})
}

func (h *Handler) removeMember(c *gin.Context) {
	ownerID, ok := authctx.UserID(c.Request.Context())
	if !ok {
		h.fail(c, apperr.CodeUnauthorized, "unauthenticated", nil, nil)
		return
	}
	parseID := func(value string) (uuid.UUID, bool) {
		id, err := uuid.Parse(value)
		return id, err == nil && id != uuid.Nil && id.String() == value
	}
	tenantID, tenantOK := parseID(c.Param("tenantID"))
	memberID, memberOK := parseID(c.Param("userID"))
	if !tenantOK || !memberOK {
		h.fail(c, apperr.CodeInvalidID, "invalid tenant or user ID", nil, nil)
		return
	}
	if err := h.service.RemoveMember(c.Request.Context(), tenantID, ownerID, memberID); err != nil {
		if errors.Is(err, tenantdb.ErrTenantOwnerRequired) {
			h.fail(c, apperr.CodeForbidden, "tenant owner required", err, nil)
			return
		}
		if errors.Is(err, domain.ErrOwnerCannotLeave) || errors.Is(err, domain.ErrLastMembership) || errors.Is(err, domain.ErrMemberNotFound) {
			h.fail(c, apperr.CodeConflict, "member cannot be removed", err, nil)
			return
		}
		h.fail(c, apperr.CodeInternalServer, "tenant member removal failed", err, nil)
		return
	}
	response.SuccessResponse(c, apperr.CodeSuccess, gin.H{"removed": true})
}

func (h *Handler) invite(c *gin.Context) {
	userID, ok := authctx.UserID(c.Request.Context())
	if !ok {
		h.fail(c, apperr.CodeUnauthorized, "unauthenticated", nil, nil)
		return
	}
	tenantID, err := uuid.Parse(c.Param("tenantID"))
	if err != nil || tenantID == uuid.Nil || tenantID.String() != c.Param("tenantID") {
		h.fail(c, apperr.CodeInvalidID, "invalid tenant ID", err, nil)
		return
	}
	var request struct {
		Username string `json:"username" binding:"required,max=50"`
	}
	if err := c.ShouldBindJSON(&request); err != nil {
		h.fail(c, apperr.CodeValidationFailed, "invalid account username", err, nil)
		return
	}
	invitation, err := h.service.Invite(c.Request.Context(), tenantID, userID, request.Username)
	if err != nil {
		h.fail(c, apperr.CodeForbidden, "invitation could not be created", err, nil)
		return
	}
	response.SuccessResponse(c, apperr.CodeCreated, serializeInvitation(invitation))
}

func (h *Handler) findPendingInvitationsForUser(c *gin.Context) {
	userID, ok := authctx.UserID(c.Request.Context())
	if !ok {
		h.fail(c, apperr.CodeUnauthorized, "unauthenticated", nil, nil)
		return
	}
	items, err := h.service.FindPendingInvitationsForUser(c.Request.Context(), userID)
	if err != nil {
		h.fail(c, apperr.CodeInternalServer, "invitation list failed", err, nil)
		return
	}
	result := make([]invitationDTO, 0, len(items))
	for _, item := range items {
		result = append(result, serializeInvitation(item))
	}
	response.SuccessResponse(c, apperr.CodeSuccess, gin.H{"invitations": result})
}

func (h *Handler) acceptInvitation(c *gin.Context) {
	userID, ok := authctx.UserID(c.Request.Context())
	if !ok {
		h.fail(c, apperr.CodeUnauthorized, "unauthenticated", nil, nil)
		return
	}
	invitationID, err := uuid.Parse(c.Param("invitationID"))
	if err != nil || invitationID == uuid.Nil || invitationID.String() != c.Param("invitationID") {
		h.fail(c, apperr.CodeInvalidID, "invalid invitation ID", err, nil)
		return
	}
	view, err := h.service.AcceptInvitation(c.Request.Context(), invitationID, userID)
	if err != nil {
		h.fail(c, apperr.CodeConflict, "invitation cannot be accepted", err, nil)
		return
	}
	response.SuccessResponse(c, apperr.CodeSuccess, serializeView(view))
}

type invitationDTO struct {
	ID         uuid.UUID `json:"id"`
	TenantID   uuid.UUID `json:"tenant_id"`
	TenantName string    `json:"tenant_name"`
	Username   string    `json:"username"`
	ExpiresAt  time.Time `json:"expires_at"`
}

type memberDTO struct {
	UserID   uuid.UUID `json:"user_id"`
	Username string    `json:"username"`
	Role     string    `json:"role"`
}

func serializeInvitation(view contract.InvitationView) invitationDTO {
	return invitationDTO{ID: view.ID, TenantID: view.TenantID, TenantName: view.TenantName, Username: view.Username, ExpiresAt: view.ExpiresAt}
}

func (h *Handler) findForUser(c *gin.Context) {
	userID, ok := authctx.UserID(c.Request.Context())
	if !ok {
		h.fail(c, apperr.CodeUnauthorized, "unauthenticated", nil, nil)
		return
	}
	items, err := h.service.FindForUser(c.Request.Context(), userID)
	if err != nil {
		h.fail(c, apperr.CodeInternalServer, "tenant list failed", err, nil)
		return
	}
	response.SuccessResponse(c, apperr.CodeSuccess, gin.H{"tenants": serializeViews(items)})
}

func (h *Handler) create(c *gin.Context) {
	userID, ok := authctx.UserID(c.Request.Context())
	if !ok {
		h.fail(c, apperr.CodeUnauthorized, "unauthenticated", nil, nil)
		return
	}
	var request struct {
		Name string `json:"name" binding:"required,max=120"`
	}
	if err := c.ShouldBindJSON(&request); err != nil {
		h.fail(c, apperr.CodeValidationFailed, "invalid tenant name", err, nil)
		return
	}
	created, err := h.service.Create(c.Request.Context(), userID, request.Name)
	if err != nil {
		if errors.Is(err, domain.ErrInvalidTenantName) {
			h.fail(c, apperr.CodeValidationFailed, "invalid tenant name", err, nil)
			return
		}
		h.fail(c, apperr.CodeInternalServer, "tenant create failed", err, nil)
		return
	}
	response.SuccessResponse(c, apperr.CodeCreated, serializeView(created))
}

func (h *Handler) rename(c *gin.Context) {
	userID, ok := authctx.UserID(c.Request.Context())
	if !ok {
		h.fail(c, apperr.CodeUnauthorized, "unauthenticated", nil, nil)
		return
	}
	tenantID, err := uuid.Parse(c.Param("tenantID"))
	if err != nil || tenantID == uuid.Nil || tenantID.String() != c.Param("tenantID") {
		h.fail(c, apperr.CodeInvalidID, "invalid tenant ID", err, nil)
		return
	}
	var request struct {
		Name string `json:"name" binding:"required,max=120"`
	}
	if err := c.ShouldBindJSON(&request); err != nil {
		h.fail(c, apperr.CodeValidationFailed, "invalid tenant name", err, nil)
		return
	}
	updated, err := h.service.Rename(c.Request.Context(), tenantID, userID, request.Name)
	if err == nil {
		response.SuccessResponse(c, apperr.CodeSuccess, serializeView(updated))
		return
	}
	if errors.Is(err, domain.ErrRenameCooldown) {
		items, listErr := h.service.FindForUser(c.Request.Context(), userID)
		if listErr == nil {
			for _, item := range items {
				if item.Tenant.ID == tenantID {
					h.fail(c, apperr.CodeConflict, "tenant name change cooldown active", err, gin.H{"next_name_change_at": item.NextNameChangeAt})
					return
				}
			}
		}
		h.fail(c, apperr.CodeConflict, "tenant name change cooldown active", err, nil)
		return
	}
	if errors.Is(err, tenantdb.ErrTenantOwnerRequired) {
		h.fail(c, apperr.CodeForbidden, "tenant owner required", err, nil)
		return
	}
	h.fail(c, apperr.CodeInternalServer, "tenant rename failed", err, nil)
}

type tenantDTO struct {
	ID               uuid.UUID  `json:"id"`
	Name             string     `json:"name"`
	Slug             string     `json:"slug"`
	Role             string     `json:"role"`
	Status           string     `json:"status"`
	CanChangeName    bool       `json:"can_change_name"`
	NextNameChangeAt *time.Time `json:"next_name_change_at"`
}

func serializeViews(views []contract.TenantView) []tenantDTO {
	result := make([]tenantDTO, 0, len(views))
	for _, view := range views {
		result = append(result, serializeView(view))
	}
	return result
}

func serializeView(view contract.TenantView) tenantDTO {
	return tenantDTO{
		ID: view.Tenant.ID, Name: view.Tenant.Name, Slug: view.Tenant.Slug,
		Role: view.Role, Status: view.Tenant.Status,
		CanChangeName: view.CanChangeName, NextNameChangeAt: view.NextNameChangeAt,
	}
}

func noStore(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	c.Header("Pragma", "no-cache")
}

func (h *Handler) fail(c *gin.Context, code int, message string, cause error, details any) {
	noStore(c)
	err := apperr.New(code, message, cause).WithDetails(details)
	if cause != nil {
		_ = c.Error(cause)
	}
	c.Abort()
	response.ErrorResponse(c, code, err)
}
