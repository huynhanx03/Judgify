package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	materialDTO "github.com/huynhanx03/judgify/internal/material/core/dto"
	"github.com/huynhanx03/judgify/internal/material/core/entity"
	materialMocks "github.com/huynhanx03/judgify/internal/material/mocks"
	"github.com/huynhanx03/judgify/pkg/common/apperr"
	"github.com/huynhanx03/judgify/pkg/common/http/response"
	d "github.com/huynhanx03/judgify/pkg/dto"
)

func newMaterialTestEnv(t *testing.T) (
	*materialService,
	*materialMocks.MockMaterialRepository,
) {
	t.Helper()
	ctrl := gomock.NewController(t)

	repo := materialMocks.NewMockMaterialRepository(ctrl)
	svc := NewMaterialService(repo).(*materialService)
	return svc, repo
}

// --- calculateEstimatedReadTime ---

func TestCalculateEstimatedReadTime(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		expected int
	}{
		{"empty", "", 0},
		{"short", "hello", 1},
		{"medium", string(make([]byte, 500)), 1},
		{"long", string(make([]byte, 2000)), 2},
		{"very long", string(make([]byte, 5000)), 5},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, calculateEstimatedReadTime(tt.content))
		})
	}
}

// --- Find ---

func TestFindMaterials_Success(t *testing.T) {
	svc, repo := newMaterialTestEnv(t)
	ctx := context.Background()

	records := []*entity.Material{
		{ID: 1, Title: "Go Basics", ViewCount: 10},
		{ID: 2, Title: "Advanced Go", ViewCount: 5},
	}
	repo.EXPECT().Find(ctx, gomock.Any()).Return(&d.Paginated[*entity.Material]{
		Records:    &records,
		Pagination: d.CalculatePagination(1, 10, 2),
	}, nil)

	resp, err := svc.Find(ctx, &d.QueryOptions{})

	assert.NoError(t, err)
	assert.Len(t, *resp.Records, 2)
}

func TestFindMaterials_RepoError(t *testing.T) {
	svc, repo := newMaterialTestEnv(t)
	ctx := context.Background()

	repo.EXPECT().Find(ctx, gomock.Any()).Return(nil, apperr.New(response.CodeDatabaseError, "db error", nil))

	resp, err := svc.Find(ctx, &d.QueryOptions{})

	assert.Nil(t, resp)
	assert.Error(t, err)
}

// --- Get ---

func TestGetMaterial_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	repo := materialMocks.NewMockMaterialRepository(ctrl)
	svc := NewMaterialService(repo).(*materialService)
	ctx := context.Background()

	repo.EXPECT().Get(ctx, 1).Return(&entity.Material{
		ID: 1, Title: "Go Basics", ViewCount: 10,
	}, nil)

	// IncrementViewCount runs in a goroutine — allow but don't enforce
	repo.EXPECT().IncrementViewCount(gomock.Any(), 1).Return(nil).AnyTimes()

	resp, err := svc.Get(ctx, 1)

	assert.NoError(t, err)
	assert.Equal(t, 1, resp.ID)
	assert.Equal(t, 11, resp.ViewCount) // viewCount + 1
}

func TestGetMaterial_NotFound(t *testing.T) {
	svc, repo := newMaterialTestEnv(t)
	ctx := context.Background()

	repo.EXPECT().Get(ctx, 999).Return(nil, apperr.New(response.CodeNotFound, "not found", nil))

	resp, err := svc.Get(ctx, 999)

	assert.Nil(t, resp)
	assert.Error(t, err)
}

// --- Create ---

func TestCreateMaterial_Success(t *testing.T) {
	svc, repo := newMaterialTestEnv(t)
	ctx := context.Background()

	repo.EXPECT().Create(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, e *entity.Material) error {
			e.ID = 1
			return nil
		},
	)

	repo.EXPECT().AddTags(gomock.Any(), 1, []int{10, 20}).Return(nil)

	repo.EXPECT().Get(gomock.Any(), 1).Return(&entity.Material{
		ID:       1,
		Title:    "Go Basics",
		Status:   "draft",
		ViewCount: 0,
	}, nil)

	resp, err := svc.Create(ctx, 1, &materialDTO.CreateMaterialRequest{
		Title:       "Go Basics",
		Content:     "some content",
		TagIDs:      []int{10, 20},
	})

	assert.NoError(t, err)
	assert.Equal(t, "Go Basics", resp.Title)
}

func TestCreateMaterial_WithoutTags(t *testing.T) {
	svc, repo := newMaterialTestEnv(t)
	ctx := context.Background()

	repo.EXPECT().Create(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, e *entity.Material) error {
			e.ID = 2
			return nil
		},
	)

	// AddTags should NOT be called when TagIDs is empty

	repo.EXPECT().Get(gomock.Any(), 2).Return(&entity.Material{
		ID:    2,
		Title: "Simple Material",
	}, nil)

	resp, err := svc.Create(ctx, 1, &materialDTO.CreateMaterialRequest{
		Title:   "Simple Material",
		Content: "hello",
	})

	assert.NoError(t, err)
	assert.Equal(t, "Simple Material", resp.Title)
}

func TestCreateMaterial_RepoCreateError(t *testing.T) {
	svc, repo := newMaterialTestEnv(t)
	ctx := context.Background()

	repo.EXPECT().Create(gomock.Any(), gomock.Any()).Return(apperr.New(response.CodeDatabaseError, "db error", nil))

	resp, err := svc.Create(ctx, 1, &materialDTO.CreateMaterialRequest{
		Title: "Fail",
	})

	assert.Nil(t, resp)
	assert.Error(t, err)
}

func TestCreateMaterial_AddTagsError(t *testing.T) {
	svc, repo := newMaterialTestEnv(t)
	ctx := context.Background()

	repo.EXPECT().Create(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, e *entity.Material) error {
			e.ID = 1
			return nil
		},
	)

	repo.EXPECT().AddTags(gomock.Any(), 1, []int{99}).Return(apperr.New(response.CodeDatabaseError, "tag error", nil))

	resp, err := svc.Create(ctx, 1, &materialDTO.CreateMaterialRequest{
		Title:  "Fail Tags",
		TagIDs: []int{99},
	})

	assert.Nil(t, resp)
	assert.Error(t, err)
}

// --- Update ---

func TestUpdateMaterial_Success(t *testing.T) {
	svc, repo := newMaterialTestEnv(t)
	ctx := context.Background()

	repo.EXPECT().Get(ctx, 1).Return(&entity.Material{
		ID:       1,
		Title:    "Go Basics",
		Content:  "old content",
		ViewCount: 10,
	}, nil)

	repo.EXPECT().Update(gomock.Any(), gomock.Any()).Return(nil)

	repo.EXPECT().Get(gomock.Any(), 1).Return(&entity.Material{
		ID:       1,
		Title:    "Go Advanced",
		Content:  "new content",
		ViewCount: 10,
	}, nil)

	newTitle := "Go Advanced"
	newContent := "new content"
	resp, err := svc.Update(ctx, 1, &materialDTO.UpdateMaterialRequest{
		Title:   &newTitle,
		Content: &newContent,
	})

	assert.NoError(t, err)
	assert.Equal(t, "Go Advanced", resp.Title)
}

func TestUpdateMaterial_NotFound(t *testing.T) {
	svc, repo := newMaterialTestEnv(t)
	ctx := context.Background()

	repo.EXPECT().Get(ctx, 999).Return(nil, apperr.New(response.CodeNotFound, "not found", nil))

	resp, err := svc.Update(ctx, 999, &materialDTO.UpdateMaterialRequest{})

	assert.Nil(t, resp)
	assert.Error(t, err)
}

func TestUpdateMaterial_WithTagsReplace(t *testing.T) {
	svc, repo := newMaterialTestEnv(t)
	ctx := context.Background()

	repo.EXPECT().Get(ctx, 1).Return(&entity.Material{
		ID:    1,
		Title: "Go Basics",
	}, nil)

	repo.EXPECT().Update(gomock.Any(), gomock.Any()).Return(nil)

	newTags := []int{30, 40}
	repo.EXPECT().RemoveAllTags(gomock.Any(), 1).Return(nil)
	repo.EXPECT().AddTags(gomock.Any(), 1, newTags).Return(nil)

	repo.EXPECT().Get(gomock.Any(), 1).Return(&entity.Material{
		ID:    1,
		Title: "Go Basics",
	}, nil)

	resp, err := svc.Update(ctx, 1, &materialDTO.UpdateMaterialRequest{
		TagIDs: &newTags,
	})

	assert.NoError(t, err)
	assert.NotNil(t, resp)
}

func TestUpdateMaterial_RemoveAllTags(t *testing.T) {
	svc, repo := newMaterialTestEnv(t)
	ctx := context.Background()

	repo.EXPECT().Get(ctx, 1).Return(&entity.Material{
		ID:    1,
		Title: "Go Basics",
	}, nil)

	repo.EXPECT().Update(gomock.Any(), gomock.Any()).Return(nil)

	emptyTags := []int{}
	repo.EXPECT().RemoveAllTags(gomock.Any(), 1).Return(nil)
	// AddTags should NOT be called when tag list is empty

	repo.EXPECT().Get(gomock.Any(), 1).Return(&entity.Material{
		ID:    1,
		Title: "Go Basics",
	}, nil)

	resp, err := svc.Update(ctx, 1, &materialDTO.UpdateMaterialRequest{
		TagIDs: &emptyTags,
	})

	assert.NoError(t, err)
	assert.NotNil(t, resp)
}

func TestUpdateMaterial_RepoUpdateError(t *testing.T) {
	svc, repo := newMaterialTestEnv(t)
	ctx := context.Background()

	repo.EXPECT().Get(ctx, 1).Return(&entity.Material{
		ID:    1,
		Title: "Go Basics",
	}, nil)

	repo.EXPECT().Update(gomock.Any(), gomock.Any()).Return(apperr.New(response.CodeDatabaseError, "db error", nil))

	resp, err := svc.Update(ctx, 1, &materialDTO.UpdateMaterialRequest{})

	assert.Nil(t, resp)
	assert.Error(t, err)
}

// --- Delete ---

func TestDeleteMaterial_Success(t *testing.T) {
	svc, repo := newMaterialTestEnv(t)
	ctx := context.Background()

	repo.EXPECT().Delete(ctx, 1).Return(nil)

	err := svc.Delete(ctx, 1)
	assert.NoError(t, err)
}

func TestDeleteMaterial_Error(t *testing.T) {
	svc, repo := newMaterialTestEnv(t)
	ctx := context.Background()

	repo.EXPECT().Delete(ctx, 999).Return(apperr.New(response.CodeNotFound, "not found", nil))

	err := svc.Delete(ctx, 999)
	assert.Error(t, err)
}
