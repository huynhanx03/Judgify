package usecase

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"github.com/huynhanx03/go-common/pkg/common/apperr"
	d "github.com/huynhanx03/go-common/pkg/dto"

	materialDTO "github.com/huynhanx03/judgify/internal/material/contract"
	entity "github.com/huynhanx03/judgify/internal/material/domain"
	"github.com/huynhanx03/judgify/internal/testutil"
	materialMocks "github.com/huynhanx03/judgify/internal/testutil/mocks"
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

// --- Public/admin reads ---

func TestFindAdminMaterials_Success(t *testing.T) {
	svc, repo := newMaterialTestEnv(t)
	ctx := context.Background()

	records := []*entity.Material{
		{ID: testutil.ID(1), Title: "Go Basics", ViewCount: 10},
		{ID: testutil.ID(2), Title: "Advanced Go", ViewCount: 5},
	}
	repo.EXPECT().Find(ctx, gomock.Any()).Return(&d.Paginated[*entity.Material]{
		Records:    &records,
		Pagination: d.CalculatePagination(1, 10, 2),
	}, nil)

	resp, err := svc.FindAdmin(ctx, &d.QueryOptions{})

	assert.NoError(t, err)
	assert.Len(t, *resp.Records, 2)
}

func TestFindAdminMaterials_RepoError(t *testing.T) {
	svc, repo := newMaterialTestEnv(t)
	ctx := context.Background()

	repo.EXPECT().Find(ctx, gomock.Any()).Return(nil, apperr.New(apperr.CodeDatabaseError, "db error", nil))

	resp, err := svc.FindAdmin(ctx, &d.QueryOptions{})

	assert.Nil(t, resp)
	assert.Error(t, err)
}

func TestFindPublicMaterials_UsesRestrictedRepository(t *testing.T) {
	svc, repo := newMaterialTestEnv(t)
	ctx := context.Background()
	records := []*entity.Material{
		{ID: testutil.ID(1), Title: "Published public", Status: "published", Visibility: "public"},
	}
	options := &d.QueryOptions{}
	repo.EXPECT().FindPublic(ctx, options).Return(&d.Paginated[*entity.Material]{
		Records:    &records,
		Pagination: d.CalculatePagination(1, 10, 1),
	}, nil)

	resp, err := svc.FindPublic(ctx, options)

	assert.NoError(t, err)
	assert.Len(t, *resp.Records, 1)
	assert.Equal(t, "published", (*resp.Records)[0].Status)
	assert.Equal(t, "public", (*resp.Records)[0].Visibility)
}

// --- Get ---

func TestGetMaterial_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	repo := materialMocks.NewMockMaterialRepository(ctrl)
	svc := NewMaterialService(repo).(*materialService)
	ctx := context.Background()

	repo.EXPECT().GetPublic(ctx, testutil.ID(1)).Return(&entity.Material{
		ID: testutil.ID(1), Title: "Go Basics", ViewCount: 10,
	}, nil)

	// IncrementViewCount runs in a goroutine — allow but don't enforce
	repo.EXPECT().IncrementViewCount(gomock.Any(), testutil.ID(1)).Return(nil).AnyTimes()

	resp, err := svc.GetPublic(ctx, testutil.ID(1))

	assert.NoError(t, err)
	assert.Equal(t, testutil.ID(1), resp.ID)
	assert.Equal(t, int64(11), resp.ViewCount) // viewCount + 1
}

func TestGetMaterial_NotFound(t *testing.T) {
	svc, repo := newMaterialTestEnv(t)
	ctx := context.Background()

	repo.EXPECT().GetPublic(ctx, testutil.ID(999)).Return(nil, apperr.New(apperr.CodeNotFound, "not found", nil))

	resp, err := svc.GetPublic(ctx, testutil.ID(999))

	assert.Nil(t, resp)
	assert.Error(t, err)
}

func TestGetAdminMaterial_IncludesDraftWithoutIncrementingViews(t *testing.T) {
	svc, repo := newMaterialTestEnv(t)
	ctx := context.Background()
	draft := &entity.Material{
		ID:         testutil.ID(2),
		Title:      "Draft",
		Status:     "draft",
		Visibility: "group",
		ViewCount:  7,
	}
	repo.EXPECT().Get(ctx, draft.ID).Return(draft, nil)

	resp, err := svc.GetAdmin(ctx, draft.ID)

	assert.NoError(t, err)
	assert.Equal(t, "draft", resp.Status)
	assert.Equal(t, "group", resp.Visibility)
	assert.Equal(t, int64(7), resp.ViewCount)
}

// --- Create ---

func TestCreateMaterial_Success(t *testing.T) {
	svc, repo := newMaterialTestEnv(t)
	ctx := context.Background()

	repo.EXPECT().Create(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, e *entity.Material) error {
			e.ID = testutil.ID(1)
			return nil
		},
	)

	repo.EXPECT().AddTags(gomock.Any(), testutil.ID(1), []uuid.UUID{testutil.ID(10), testutil.ID(20)}).Return(nil)

	repo.EXPECT().Get(gomock.Any(), testutil.ID(1)).Return(&entity.Material{
		ID:        testutil.ID(1),
		Title:     "Go Basics",
		Status:    "draft",
		ViewCount: 0,
	}, nil)

	resp, err := svc.Create(ctx, testutil.ID(1), &materialDTO.CreateMaterialCommand{
		Title:   "Go Basics",
		Content: "some content",
		TagIDs:  []uuid.UUID{testutil.ID(10), testutil.ID(20)},
	})

	assert.NoError(t, err)
	assert.Equal(t, "Go Basics", resp.Title)
}

func TestCreateMaterial_WithoutTags(t *testing.T) {
	svc, repo := newMaterialTestEnv(t)
	ctx := context.Background()

	repo.EXPECT().Create(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, e *entity.Material) error {
			e.ID = testutil.ID(2)
			return nil
		},
	)

	// AddTags should NOT be called when TagIDs is empty

	repo.EXPECT().Get(gomock.Any(), testutil.ID(2)).Return(&entity.Material{
		ID:    testutil.ID(2),
		Title: "Simple Material",
	}, nil)

	resp, err := svc.Create(ctx, testutil.ID(1), &materialDTO.CreateMaterialCommand{
		Title:   "Simple Material",
		Content: "hello",
	})

	assert.NoError(t, err)
	assert.Equal(t, "Simple Material", resp.Title)
}

func TestCreateMaterial_RepoCreateError(t *testing.T) {
	svc, repo := newMaterialTestEnv(t)
	ctx := context.Background()

	repo.EXPECT().Create(gomock.Any(), gomock.Any()).Return(apperr.New(apperr.CodeDatabaseError, "db error", nil))

	resp, err := svc.Create(ctx, testutil.ID(1), &materialDTO.CreateMaterialCommand{
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
			e.ID = testutil.ID(1)
			return nil
		},
	)

	repo.EXPECT().AddTags(gomock.Any(), testutil.ID(1), []uuid.UUID{testutil.ID(99)}).Return(apperr.New(apperr.CodeDatabaseError, "tag error", nil))

	resp, err := svc.Create(ctx, testutil.ID(1), &materialDTO.CreateMaterialCommand{
		Title:  "Fail Tags",
		TagIDs: []uuid.UUID{testutil.ID(99)},
	})

	assert.Nil(t, resp)
	assert.Error(t, err)
}

// --- Update ---

func TestUpdateMaterial_Success(t *testing.T) {
	svc, repo := newMaterialTestEnv(t)
	ctx := context.Background()

	repo.EXPECT().Get(ctx, testutil.ID(1)).Return(&entity.Material{
		ID:        testutil.ID(1),
		Title:     "Go Basics",
		Content:   "old content",
		ViewCount: 10,
	}, nil)

	repo.EXPECT().Update(gomock.Any(), gomock.Any()).Return(nil)

	repo.EXPECT().Get(gomock.Any(), testutil.ID(1)).Return(&entity.Material{
		ID:        testutil.ID(1),
		Title:     "Go Advanced",
		Content:   "new content",
		ViewCount: 10,
	}, nil)

	newTitle := "Go Advanced"
	newContent := "new content"
	resp, err := svc.Update(ctx, testutil.ID(1), &materialDTO.UpdateMaterialCommand{
		Title:   &newTitle,
		Content: &newContent,
	})

	assert.NoError(t, err)
	assert.Equal(t, "Go Advanced", resp.Title)
}

func TestUpdateMaterial_NotFound(t *testing.T) {
	svc, repo := newMaterialTestEnv(t)
	ctx := context.Background()

	repo.EXPECT().Get(ctx, testutil.ID(999)).Return(nil, apperr.New(apperr.CodeNotFound, "not found", nil))

	resp, err := svc.Update(ctx, testutil.ID(999), &materialDTO.UpdateMaterialCommand{})

	assert.Nil(t, resp)
	assert.Error(t, err)
}

func TestUpdateMaterial_WithTagsReplace(t *testing.T) {
	svc, repo := newMaterialTestEnv(t)
	ctx := context.Background()

	repo.EXPECT().Get(ctx, testutil.ID(1)).Return(&entity.Material{
		ID:    testutil.ID(1),
		Title: "Go Basics",
	}, nil)

	repo.EXPECT().Update(gomock.Any(), gomock.Any()).Return(nil)

	newTags := []uuid.UUID{testutil.ID(30), testutil.ID(40)}
	repo.EXPECT().RemoveAllTags(gomock.Any(), testutil.ID(1)).Return(nil)
	repo.EXPECT().AddTags(gomock.Any(), testutil.ID(1), newTags).Return(nil)

	repo.EXPECT().Get(gomock.Any(), testutil.ID(1)).Return(&entity.Material{
		ID:    testutil.ID(1),
		Title: "Go Basics",
	}, nil)

	resp, err := svc.Update(ctx, testutil.ID(1), &materialDTO.UpdateMaterialCommand{
		TagIDs: &newTags,
	})

	assert.NoError(t, err)
	assert.NotNil(t, resp)
}

func TestUpdateMaterial_RemoveAllTags(t *testing.T) {
	svc, repo := newMaterialTestEnv(t)
	ctx := context.Background()

	repo.EXPECT().Get(ctx, testutil.ID(1)).Return(&entity.Material{
		ID:    testutil.ID(1),
		Title: "Go Basics",
	}, nil)

	repo.EXPECT().Update(gomock.Any(), gomock.Any()).Return(nil)

	emptyTags := []uuid.UUID{}
	repo.EXPECT().RemoveAllTags(gomock.Any(), testutil.ID(1)).Return(nil)
	// AddTags should NOT be called when tag list is empty

	repo.EXPECT().Get(gomock.Any(), testutil.ID(1)).Return(&entity.Material{
		ID:    testutil.ID(1),
		Title: "Go Basics",
	}, nil)

	resp, err := svc.Update(ctx, testutil.ID(1), &materialDTO.UpdateMaterialCommand{
		TagIDs: &emptyTags,
	})

	assert.NoError(t, err)
	assert.NotNil(t, resp)
}

func TestUpdateMaterial_RepoUpdateError(t *testing.T) {
	svc, repo := newMaterialTestEnv(t)
	ctx := context.Background()

	repo.EXPECT().Get(ctx, testutil.ID(1)).Return(&entity.Material{
		ID:    testutil.ID(1),
		Title: "Go Basics",
	}, nil)

	repo.EXPECT().Update(gomock.Any(), gomock.Any()).Return(apperr.New(apperr.CodeDatabaseError, "db error", nil))

	resp, err := svc.Update(ctx, testutil.ID(1), &materialDTO.UpdateMaterialCommand{})

	assert.Nil(t, resp)
	assert.Error(t, err)
}

// --- Delete ---

func TestDeleteMaterial_Success(t *testing.T) {
	svc, repo := newMaterialTestEnv(t)
	ctx := context.Background()

	repo.EXPECT().Delete(ctx, testutil.ID(1)).Return(nil)

	err := svc.Delete(ctx, testutil.ID(1))
	assert.NoError(t, err)
}

func TestDeleteMaterial_Error(t *testing.T) {
	svc, repo := newMaterialTestEnv(t)
	ctx := context.Background()

	repo.EXPECT().Delete(ctx, testutil.ID(999)).Return(apperr.New(apperr.CodeNotFound, "not found", nil))

	err := svc.Delete(ctx, testutil.ID(999))
	assert.Error(t, err)
}
