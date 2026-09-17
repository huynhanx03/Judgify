package profile

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/huynhanx03/go-common/pkg/common/apperr"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	identitydto "github.com/huynhanx03/judgify/internal/identity/contract"
	identitymocks "github.com/huynhanx03/judgify/internal/testutil/mocks"
)

type profileSourceFunc func(context.Context, uuid.UUID) (Projection, error)

func (load profileSourceFunc) Load(ctx context.Context, userID uuid.UUID) (Projection, error) {
	return load(ctx, userID)
}

type profileTransactionMarker struct{}

type profileMarkerTransaction struct{}

func (profileMarkerTransaction) DoInTx(
	ctx context.Context,
	callback func(context.Context) error,
) error {
	return callback(context.WithValue(ctx, profileTransactionMarker{}, true))
}

func TestQueryBuildsCompatibleProfileFromOneTransactionProjection(t *testing.T) {
	controller := gomock.NewController(t)
	users := identitymocks.NewMockUserService(controller)
	userID := uuid.New()
	projection := validProjection()
	users.EXPECT().GetProfile(gomock.Any(), userID).
		DoAndReturn(func(ctx context.Context, _ uuid.UUID) (*identitydto.ProfileAttrs, error) {
			require.True(t, profileTransactionMarked(ctx))
			return &identitydto.ProfileAttrs{
				ID: userID, Username: "judge-user", FirstName: "Judge", LastName: "User",
				Gender: 1, Birthday: "2000-01-01", JoinedAt: "2026-07-22T04:00:00Z",
			}, nil
		})
	source := profileSourceFunc(func(ctx context.Context, requested uuid.UUID) (Projection, error) {
		require.True(t, profileTransactionMarked(ctx))
		require.Equal(t, userID, requested)
		return projection, nil
	})
	query, err := NewQuery(users, source, profileMarkerTransaction{})
	require.NoError(t, err)

	response, err := query.Get(context.Background(), userID)

	require.NoError(t, err)
	require.Equal(t, userID, response.ID)
	require.Equal(t, "judge-user", response.Username)
	require.Equal(t, int64(50), response.Cultivation.TotalExp)
	require.Equal(t, "Novice", response.Cultivation.Level.Name)
	require.Equal(t, "Advanced", response.Cultivation.Level.NextName)
	require.InDelta(t, 50, response.Cultivation.Level.Progress, 0.001)
	require.Equal(t, int64(50), response.Cultivation.Level.ExpToNext)
	require.Equal(t, "Silver", response.Cultivation.Rank.Name)
	require.Equal(t, "Gold", response.Cultivation.Rank.NextName)
	require.InDelta(t, 50, response.Cultivation.Rank.Progress, 0.001)
	require.Equal(t, 50, response.Cultivation.Rank.RatingToNext)
	require.NotNil(t, response.Cultivation.RootBone)
	require.Equal(t, "Dragon Bone", response.Cultivation.RootBone.Name)
	require.Len(t, response.Cultivation.Talents, 3)
	require.Len(t, response.Cultivation.Elements, 1)
	require.Len(t, response.ProblemStats.ByDifficulty, 1)
	require.Len(t, response.ProblemStats.ByTag, 1)
	// The query sorts clones; a caller-owned source projection remains untouched.
	require.Equal(t, "Advanced", projection.Levels[0].Name)
}

func TestQueryReturnsStorageFailureInsteadOfFabricatingEmptyProfile(t *testing.T) {
	controller := gomock.NewController(t)
	users := identitymocks.NewMockUserService(controller)
	userID := uuid.New()
	storageErr := errors.New("database unavailable")
	users.EXPECT().GetProfile(gomock.Any(), userID).Return(
		&identitydto.ProfileAttrs{ID: userID, Username: "judge-user"},
		nil,
	)
	query, err := NewQuery(
		users,
		profileSourceFunc(func(context.Context, uuid.UUID) (Projection, error) {
			return Projection{}, storageErr
		}),
		profileMarkerTransaction{},
	)
	require.NoError(t, err)

	response, err := query.Get(context.Background(), userID)

	require.Nil(t, response)
	require.ErrorIs(t, err, storageErr)
	requireProfileAppError(t, err, apperr.CodeInternalServer, "profile unavailable")
}

func TestQueryRejectsUnboundedAlternativeSource(t *testing.T) {
	controller := gomock.NewController(t)
	users := identitymocks.NewMockUserService(controller)
	userID := uuid.New()
	users.EXPECT().GetProfile(gomock.Any(), userID).Return(
		&identitydto.ProfileAttrs{ID: userID, Username: "judge-user"},
		nil,
	)
	query, err := NewQuery(
		users,
		profileSourceFunc(func(context.Context, uuid.UUID) (Projection, error) {
			return Projection{RewardProfile: RewardProfile{Traits: make([]Trait, MaximumTraits+1)}}, nil
		}),
		profileMarkerTransaction{},
	)
	require.NoError(t, err)

	response, err := query.Get(context.Background(), userID)

	require.Nil(t, response)
	require.ErrorIs(t, err, ErrInvalidProfileProjection)
	requireProfileAppError(t, err, apperr.CodeInternalServer, "profile unavailable")
}

func TestQueryRejectsMismatchedIdentityProjection(t *testing.T) {
	controller := gomock.NewController(t)
	users := identitymocks.NewMockUserService(controller)
	userID := uuid.New()
	users.EXPECT().GetProfile(gomock.Any(), userID).Return(
		&identitydto.ProfileAttrs{ID: uuid.New(), Username: "wrong-user"},
		nil,
	)
	query, err := NewQuery(
		users,
		profileSourceFunc(func(context.Context, uuid.UUID) (Projection, error) {
			return validProjection(), nil
		}),
		profileMarkerTransaction{},
	)
	require.NoError(t, err)

	response, err := query.Get(context.Background(), userID)

	require.Nil(t, response)
	requireProfileAppError(t, err, apperr.CodeInternalServer, "profile unavailable")
}

func validProjection() Projection {
	return Projection{
		Stats: Stats{TotalExp: 50, Rating: 150, TotalSubmissions: 12, AcceptedCount: 8},
		RewardProfile: RewardProfile{
			ID: uuid.New(), Revision: 1,
			Checksum: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
			Source:   "onboarding", EffectiveAt: time.Date(2026, time.July, 22, 4, 0, 0, 0, time.UTC),
			Traits: []Trait{
				validProfileTrait("root_bone", "root_bone", 0, "Dragon Bone"),
				validProfileTrait("talent", "talent", 0, "Focus"),
				validProfileTrait("talent", "talent", 1, "Clarity"),
				validProfileTrait("talent", "talent", 2, "Resolve"),
			},
		},
		Elements: []ElementExp{{Code: "fire", Name: "Fire", Exp: 25}},
		Levels: []Level{
			{Name: "Advanced", MinExp: 100},
			{Name: "Novice", MinExp: 0},
		},
		Ranks: []Rank{
			{Name: "Gold", MinRating: 200},
			{Name: "Bronze", MinRating: 0},
			{Name: "Silver", MinRating: 100},
		},
		DifficultyStats: []DifficultyStat{{Name: "Easy", Level: 1, SolvedCount: 4}},
		TagStats: []TagStat{{
			Name: "Arrays", SolvedCount: 3,
			Elements: []ElementBrief{{Code: "fire", Name: "Fire"}},
		}},
	}
}

func validProfileTrait(kind, slot string, slotOrder int, name string) Trait {
	bonus := int64(1)
	return Trait{
		ID: uuid.New(), Code: strings.ToLower(name), Type: kind, Name: name,
		RarityCode: "earth", RarityName: "Earth", Slot: slot, SlotOrder: slotOrder,
		EffectRevisionID: uuid.New(), EffectRevision: 1,
		EffectChecksum:    "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
		EffectEffectiveAt: time.Date(2026, time.July, 22, 4, 0, 0, 0, time.UTC),
		Effect:            TraitEffect{Version: 1, Kind: "exp_bonus", Scope: "all", FlatBonus: &bonus},
	}
}

func profileTransactionMarked(ctx context.Context) bool {
	marked, _ := ctx.Value(profileTransactionMarker{}).(bool)
	return marked
}

func requireProfileAppError(t *testing.T, err error, code int, message string) {
	t.Helper()
	var applicationError *apperr.AppError
	require.ErrorAs(t, err, &applicationError)
	require.Equal(t, code, applicationError.Code)
	require.Equal(t, message, applicationError.Message)
}
