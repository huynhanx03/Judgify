package http

import (
	"context"
	"crypto/sha256"
	"net/netip"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/huynhanx03/go-common/pkg/common/http/handler"
	"github.com/huynhanx03/go-common/pkg/common/http/middlewares"
	commonrequest "github.com/huynhanx03/go-common/pkg/common/http/request"

	"github.com/huynhanx03/judgify/internal/material/viewer"
	"github.com/huynhanx03/judgify/pkg/permissions"
)

const materialAnonymousViewerDomain = "judgify/material/anonymous-viewer/v1"

// MaterialHandlerGroup contains all material-related handlers.
type MaterialHandlerGroup struct {
	CategoryHandler MaterialCategoryHandler
	MaterialHandler MaterialHandler
}

// RegisterPublic registers public material routes.
func (h *MaterialHandlerGroup) RegisterPublic(r *gin.RouterGroup) {
	categories := r.Group("/material-categories")
	{
		categories.GET("", handler.WrapNoReq(h.CategoryHandler.FindAll))
	}

	materials := r.Group("/materials")
	{
		materials.POST("/search", handler.Wrap(h.MaterialHandler.Search))
		materials.GET("/slug/:slug", handler.Wrap(h.MaterialHandler.ResolveSlug))
		materials.GET("/slug/:slug/related", handler.Wrap(h.MaterialHandler.Related))
		materials.POST("/:id/views", captureAnonymousMaterialViewer(), handler.Wrap(h.MaterialHandler.RecordView))
	}
}

func captureAnonymousMaterialViewer() gin.HandlerFunc {
	return func(c *gin.Context) {
		address, err := netip.ParseAddr(strings.TrimSpace(c.ClientIP()))
		if err == nil {
			digest := sha256.New()
			_, _ = digest.Write([]byte(materialAnonymousViewerDomain))
			_, _ = digest.Write([]byte{0})
			_, _ = digest.Write([]byte(address.Unmap().String()))
			_, _ = digest.Write([]byte{0})
			_, _ = digest.Write([]byte(c.Request.UserAgent()))
			var fingerprint [sha256.Size]byte
			copy(fingerprint[:], digest.Sum(nil))
			c.Request = c.Request.WithContext(
				viewer.WithAnonymousFingerprint(c.Request.Context(), fingerprint),
			)
		}
		c.Next()
	}
}

// RegisterProtected registers protected material routes.
func (h *MaterialHandlerGroup) RegisterProtected(r *gin.RouterGroup, permChecker *middlewares.AuthorizationChecker) {
	categories := r.Group("/material-categories")
	{
		categories.POST("/find", permChecker.RequirePermission(permissions.ResourceKeyMaterialCategory, permissions.PermissionScopeRead), handler.Wrap(h.CategoryHandler.Find))
		categories.GET("/:id", permChecker.RequirePermission(permissions.ResourceKeyMaterialCategory, permissions.PermissionScopeRead), handler.Wrap(h.CategoryHandler.Get))
		categories.POST("", permChecker.RequirePermission(permissions.ResourceKeyMaterialCategory, permissions.PermissionScopeCreate), handler.Wrap(h.CategoryHandler.Create))
		categories.PUT("/:id", permChecker.RequirePermission(permissions.ResourceKeyMaterialCategory, permissions.PermissionScopeUpdate), handler.Wrap(h.CategoryHandler.Update))
		categories.DELETE("/:id", permChecker.RequirePermission(permissions.ResourceKeyMaterialCategory, permissions.PermissionScopeDelete), handler.Wrap(h.CategoryHandler.Delete))
	}

	adminMaterials := r.Group("/admin/materials")
	{
		adminMaterials.POST("/search", permChecker.RequirePermission(permissions.ResourceKeyMaterial, permissions.PermissionScopeRead), handler.Wrap(h.MaterialHandler.SearchAdmin))
		adminMaterials.GET("/:id", permChecker.RequirePermission(permissions.ResourceKeyMaterial, permissions.PermissionScopeRead), handler.Wrap(h.MaterialHandler.GetAdmin))
		adminMaterials.GET("/:id/revisions", permChecker.RequirePermission(permissions.ResourceKeyMaterial, permissions.PermissionScopeRead), handler.Wrap(h.MaterialHandler.ListRevisions))
		adminMaterials.GET("/:id/revisions/:revision_id", permChecker.RequirePermission(permissions.ResourceKeyMaterial, permissions.PermissionScopeRead), handler.Wrap(h.MaterialHandler.GetRevision))
		adminMaterials.POST("/drafts", permChecker.RequirePermission(permissions.ResourceKeyMaterial, permissions.PermissionScopeCreate), materialCommandNoStore(), captureMaterialCommandID(), handler.Wrap(h.MaterialHandler.CreateDraft))
		adminMaterials.PUT("/:id/draft", permChecker.RequirePermission(permissions.ResourceKeyMaterial, permissions.PermissionScopeUpdate), materialCommandNoStore(), captureMaterialCommandID(), handler.Wrap(h.MaterialHandler.SaveDraft))
		adminMaterials.POST("/:id/submit-review/preview", permChecker.RequirePermission(permissions.ResourceKeyMaterial, permissions.PermissionScopeUpdate), materialCommandNoStore(), captureMaterialCommandID(), handler.Wrap(h.MaterialHandler.PreviewSubmitReview))
		adminMaterials.POST("/:id/submit-review", permChecker.RequirePermission(permissions.ResourceKeyMaterial, permissions.PermissionScopeUpdate), materialCommandNoStore(), captureMaterialCommandID(), handler.Wrap(h.MaterialHandler.ApplySubmitReview))
		adminMaterials.POST("/:id/return-to-draft/preview", permChecker.RequirePermission(permissions.ResourceKeyMaterial, permissions.PermissionScopeManage), materialCommandNoStore(), captureMaterialCommandID(), handler.Wrap(h.MaterialHandler.PreviewReturnToDraft))
		adminMaterials.POST("/:id/return-to-draft", permChecker.RequirePermission(permissions.ResourceKeyMaterial, permissions.PermissionScopeManage), materialCommandNoStore(), captureMaterialCommandID(), handler.Wrap(h.MaterialHandler.ApplyReturnToDraft))
		adminMaterials.POST("/:id/publish/preview", permChecker.RequirePermission(permissions.ResourceKeyMaterial, permissions.PermissionScopePublish), materialCommandNoStore(), captureMaterialCommandID(), handler.Wrap(h.MaterialHandler.PreviewPublish))
		adminMaterials.POST("/:id/publish", permChecker.RequirePermission(permissions.ResourceKeyMaterial, permissions.PermissionScopePublish), materialCommandNoStore(), captureMaterialCommandID(), handler.Wrap(h.MaterialHandler.ApplyPublish))
		adminMaterials.POST("/:id/archive/preview", permChecker.RequirePermission(permissions.ResourceKeyMaterial, permissions.PermissionScopeArchive), materialCommandNoStore(), captureMaterialCommandID(), handler.Wrap(h.MaterialHandler.PreviewArchive))
		adminMaterials.POST("/:id/archive", permChecker.RequirePermission(permissions.ResourceKeyMaterial, permissions.PermissionScopeArchive), materialCommandNoStore(), captureMaterialCommandID(), handler.Wrap(h.MaterialHandler.ApplyArchive))
		adminMaterials.POST("/:id/rollback/preview", permChecker.RequirePermission(permissions.ResourceKeyMaterial, permissions.PermissionScopeRollback), materialCommandNoStore(), captureMaterialCommandID(), handler.Wrap(h.MaterialHandler.PreviewRollback))
		adminMaterials.POST("/:id/rollback", permChecker.RequirePermission(permissions.ResourceKeyMaterial, permissions.PermissionScopeRollback), materialCommandNoStore(), captureMaterialCommandID(), handler.Wrap(h.MaterialHandler.ApplyRollback))
		adminMaterials.POST("/:id/rename", permChecker.RequirePermission(permissions.ResourceKeyMaterial, permissions.PermissionScopeUpdate), materialCommandNoStore(), captureMaterialCommandID(), handler.Wrap(h.MaterialHandler.Rename))
	}

}

type materialCommandIDContextKey struct{}

func captureMaterialCommandID() gin.HandlerFunc {
	return func(c *gin.Context) {
		commandID, err := commonrequest.ParseCanonicalUUIDIdempotencyKey(
			c.Request.Header,
		)
		if err == nil {
			c.Request = c.Request.WithContext(context.WithValue(
				c.Request.Context(),
				materialCommandIDContextKey{},
				commandID,
			))
		}
		c.Next()
	}
}

func materialCommandNoStore() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Cache-Control", "no-store")
		c.Header("Pragma", "no-cache")
		c.Header("Expires", "0")
		c.Next()
	}
}
