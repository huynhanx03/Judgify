package di

import (
	"github.com/huynhanx03/judgify/global"
	materialHttp "github.com/huynhanx03/judgify/internal/material/adapters/driver/http"
	"github.com/huynhanx03/judgify/internal/material/adapters/driven/db"
	"github.com/huynhanx03/judgify/internal/material/core/service"
	"github.com/huynhanx03/judgify/internal/material/ports"
)

// MaterialContainer holds all dependencies for the material domain.
type MaterialContainer struct {
	MaterialHandlerGroup *materialHttp.MaterialHandlerGroup
	CategoryRepo         ports.MaterialCategoryRepository
	MaterialRepo         ports.MaterialRepository
	CategoryService      ports.MaterialCategoryService
	MaterialService      ports.MaterialService
}

// NewMaterialContainer creates a new MaterialContainer.
func NewMaterialContainer() *MaterialContainer {
	client := global.EntClient

	// Repositories
	categoryRepo := db.NewMaterialCategoryRepository(client)
	materialRepo := db.NewMaterialRepository(client)

	// Services
	categoryService := service.NewMaterialCategoryService(categoryRepo)
	materialService := service.NewMaterialService(materialRepo)

	// Handlers
	handlerGroup := &materialHttp.MaterialHandlerGroup{
		CategoryHandler: materialHttp.NewMaterialCategoryHandler(categoryService),
		MaterialHandler: materialHttp.NewMaterialHandler(materialService),
	}

	return &MaterialContainer{
		MaterialHandlerGroup: handlerGroup,
		CategoryRepo:         categoryRepo,
		MaterialRepo:         materialRepo,
		CategoryService:      categoryService,
		MaterialService:      materialService,
	}
}
