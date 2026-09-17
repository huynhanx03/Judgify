package usecase

import (
	"context"
	"encoding/base64"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/huynhanx03/go-common/pkg/common/apperr"

	entity "github.com/huynhanx03/judgify/internal/cultivation/domain"
	platformmutation "github.com/huynhanx03/judgify/internal/platform/mutation"
	"github.com/huynhanx03/judgify/internal/ports"
)

type rankingProjectionRepositoryStub struct {
	page       entity.RankingProjectionPage
	pageErr    error
	lastQuery  ports.RankingProjectionQuery
	privacy    entity.RankingPrivacy
	privacyErr error
	update     entity.RankingPrivacyResult
	updateErr  error
}

func (repository *rankingProjectionRepositoryStub) Page(
	_ context.Context,
	query ports.RankingProjectionQuery,
) (entity.RankingProjectionPage, error) {
	repository.lastQuery = query
	return repository.page, repository.pageErr
}

func (repository *rankingProjectionRepositoryStub) Privacy(
	_ context.Context,
	_ uuid.UUID,
) (entity.RankingPrivacy, error) {
	return repository.privacy, repository.privacyErr
}

func (repository *rankingProjectionRepositoryStub) UpdatePrivacy(
	_ context.Context,
	_ entity.RankingPrivacyCommand,
) (entity.RankingPrivacyResult, error) {
	return repository.update, repository.updateErr
}

type rankingRankRepositoryStub struct {
	ports.RankRepository
	ranks []*entity.Rank
	err   error
}

func (repository rankingRankRepositoryStub) FindAll(context.Context) ([]*entity.Rank, error) {
	return repository.ranks, repository.err
}

type rankingLevelRepositoryStub struct {
	ports.LevelRepository
	levels []*entity.Level
	err    error
}

func (repository rankingLevelRepositoryStub) FindAll(context.Context) ([]*entity.Level, error) {
	return repository.levels, repository.err
}

type rankingJournalStub struct {
	command platformmutation.Command
	err     error
}

func (journal *rankingJournalStub) Execute(
	ctx context.Context,
	mutate func(context.Context) (platformmutation.Command, error),
) error {
	journal.command, journal.err = mutate(ctx)
	return journal.err
}

func TestRankingPageMapsPrivacySafeProjectionAndStableCursor(t *testing.T) {
	t.Parallel()

	updatedAt := time.Date(2026, time.July, 18, 12, 0, 0, 0, time.UTC)
	publicName := "alice"
	publicID := uuid.MustParse("00000000-0000-0000-0000-000000000101")
	anonymousID := uuid.MustParse("00000000-0000-0000-0000-000000000102")
	projection := &rankingProjectionRepositoryStub{page: entity.RankingProjectionPage{
		Entries: []entity.RankingProjectionEntry{
			{
				Position: 1, PublicID: publicID, DisplayName: &publicName,
				DisplayMode: "public", Rating: 1500, TotalExp: 9000,
				UpdatedAt: updatedAt,
			},
			{
				Position: 2, PublicID: anonymousID, DisplayMode: "anonymous",
				Rating: 1200, TotalExp: 5000, UpdatedAt: updatedAt.Add(time.Second),
			},
		},
		ProjectionRevision: 7,
		HasMore:            true,
	}}
	service, err := NewRankingService(
		projection,
		rankingRankRepositoryStub{ranks: []*entity.Rank{
			{Name: "Gold", MinRating: 1400},
			{Name: "Silver", MinRating: 1000},
		}},
		rankingLevelRepositoryStub{},
		&rankingJournalStub{},
	)
	require.NoError(t, err)

	response, err := service.Page(context.Background(), entity.RankingByRating, "", 2)
	require.NoError(t, err)
	require.Equal(t, entity.RankingByRating, projection.lastQuery.Kind)
	require.Equal(t, 2, projection.lastQuery.Limit)
	require.Nil(t, projection.lastQuery.Cursor)
	require.Len(t, response.Entries, 2)
	require.Equal(t, publicID, response.Entries[0].PublicID)
	require.Equal(t, "alice", *response.Entries[0].DisplayName)
	require.Equal(t, "Gold", response.Entries[0].RankTitle)
	require.Nil(t, response.Entries[1].DisplayName)
	require.Equal(t, "anonymous", response.Entries[1].DisplayMode)
	require.NotEmpty(t, response.NextCursor)

	cursor, err := decodeRankingCursor(response.NextCursor, entity.RankingByRating)
	require.NoError(t, err)
	require.Equal(t, int64(7), cursor.ProjectionRevision)
	require.Equal(t, int64(1200), cursor.Score)
	require.Equal(t, anonymousID, cursor.PublicID)
	require.Equal(t, updatedAt.Add(time.Second), cursor.UpdatedAt)
}

func TestRankingPageMapsProjectionRevisionConflict(t *testing.T) {
	t.Parallel()

	projection := &rankingProjectionRepositoryStub{pageErr: entity.ErrRankingRevisionChanged}
	service, err := NewRankingService(
		projection,
		rankingRankRepositoryStub{},
		rankingLevelRepositoryStub{},
		&rankingJournalStub{},
	)
	require.NoError(t, err)

	response, err := service.Page(context.Background(), entity.RankingByRating, "", 20)
	require.Nil(t, response)
	require.ErrorIs(t, err, entity.ErrRankingRevisionChanged)
	var appError *apperr.AppError
	require.ErrorAs(t, err, &appError)
	require.Equal(t, apperr.CodeConflict, appError.Code)
}

func TestRankingCursorRejectsKindMismatchAndUnknownFields(t *testing.T) {
	t.Parallel()

	encoded, err := encodeRankingCursor(entity.RankingCursor{
		Version:            1,
		Kind:               entity.RankingByRating,
		ProjectionRevision: 4,
		Score:              100,
		UpdatedAt:          time.Date(2026, time.July, 18, 12, 0, 0, 0, time.UTC),
		PublicID:           uuid.MustParse("00000000-0000-0000-0000-000000000201"),
	})
	require.NoError(t, err)
	_, err = decodeRankingCursor(encoded, entity.RankingByExperience)
	require.ErrorIs(t, err, entity.ErrInvalidRankingQuery)

	unknownField := base64.RawURLEncoding.EncodeToString([]byte(
		`{"v":1,"kind":"rating","revision":4,"score":100,"updated_at":"2026-07-18T12:00:00Z","public_id":"00000000-0000-0000-0000-000000000201","user_id":"private"}`,
	))
	_, err = decodeRankingCursor(unknownField, entity.RankingByRating)
	require.ErrorIs(t, err, entity.ErrInvalidRankingQuery)
}

func TestRankingPrivacyUpdateJournalsOnlyThePublicProjectionIdentity(t *testing.T) {
	t.Parallel()

	userID := uuid.MustParse("00000000-0000-0000-0000-000000000301")
	projection := &rankingProjectionRepositoryStub{update: entity.RankingPrivacyResult{
		Privacy: entity.RankingPrivacy{
			UserID:             userID,
			Visibility:         entity.RankingVisibilityAnonymous,
			Version:            3,
			ProjectionRevision: 9,
		},
		PreviousVisibility: entity.RankingVisibilityPublic,
	}}
	journal := &rankingJournalStub{}
	service, err := NewRankingService(
		projection,
		rankingRankRepositoryStub{},
		rankingLevelRepositoryStub{},
		journal,
	)
	require.NoError(t, err)

	response, err := service.UpdatePrivacy(context.Background(), entity.RankingPrivacyCommand{
		ID:              uuid.MustParse("00000000-0000-0000-0000-000000000302"),
		UserID:          userID,
		Visibility:      entity.RankingVisibilityAnonymous,
		ExpectedVersion: 2,
		Reason:          "user request",
	})
	require.NoError(t, err)
	require.Equal(t, "anonymous", response.Visibility)
	require.Equal(t, int64(9), response.ProjectionRevision)
	require.NoError(t, journal.err)
	require.Equal(t, userID.String(), journal.command.ResourceID)
	require.Equal(t, userID, journal.command.AggregateID)
	require.NotContains(t, journal.command.Payload, "username")
	require.NotContains(t, journal.command.Payload, "email")
}

func TestRankingPagePropagatesDecorationFailure(t *testing.T) {
	t.Parallel()

	decorationErr := errors.New("rank catalog unavailable")
	projection := &rankingProjectionRepositoryStub{page: entity.RankingProjectionPage{
		Entries: []entity.RankingProjectionEntry{{
			Position:    1,
			PublicID:    uuid.MustParse("00000000-0000-0000-0000-000000000401"),
			DisplayMode: "anonymous",
			Rating:      100,
			UpdatedAt:   time.Date(2026, time.July, 18, 12, 0, 0, 0, time.UTC),
		}},
	}}
	service, err := NewRankingService(
		projection,
		rankingRankRepositoryStub{err: decorationErr},
		rankingLevelRepositoryStub{},
		&rankingJournalStub{},
	)
	require.NoError(t, err)

	response, err := service.Page(context.Background(), entity.RankingByRating, "", 20)
	require.Nil(t, response)
	require.ErrorIs(t, err, decorationErr)
}
