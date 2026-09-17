package usecase

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"sort"
	"time"

	"github.com/google/uuid"
	"github.com/huynhanx03/go-common/pkg/common/apperr"

	contract "github.com/huynhanx03/judgify/internal/cultivation/contract"
	entity "github.com/huynhanx03/judgify/internal/cultivation/domain"
	platformmutation "github.com/huynhanx03/judgify/internal/platform/mutation"
	"github.com/huynhanx03/judgify/internal/ports"
)

const (
	maximumRankingPageSize    = 100
	defaultRankingPageSize    = 20
	maximumRankingCursorBytes = 768
)

type rankingService struct {
	projection ports.RankingProjectionRepository
	rankRepo   ports.RankRepository
	levelRepo  ports.LevelRepository
	journal    platformmutation.Executor
}

func NewRankingService(
	projection ports.RankingProjectionRepository,
	rankRepo ports.RankRepository,
	levelRepo ports.LevelRepository,
	journal platformmutation.Executor,
) (ports.RankingService, error) {
	if projection == nil || rankRepo == nil || levelRepo == nil || journal == nil {
		return nil, entity.ErrInvalidRankingQuery
	}
	return &rankingService{
		projection: projection, rankRepo: rankRepo,
		levelRepo: levelRepo, journal: journal,
	}, nil
}

func (service *rankingService) Page(
	ctx context.Context,
	kind entity.RankingKind,
	encodedCursor string,
	limit int,
) (*contract.RankingPageResponse, error) {
	if service == nil || ctx == nil || !kind.Valid() {
		return nil, invalidRankingQuery(entity.ErrInvalidRankingQuery)
	}
	if limit == 0 {
		limit = defaultRankingPageSize
	}
	if limit < 1 || limit > maximumRankingPageSize {
		return nil, invalidRankingQuery(entity.ErrInvalidRankingQuery)
	}
	cursor, err := decodeRankingCursor(encodedCursor, kind)
	if err != nil {
		return nil, invalidRankingQuery(err)
	}
	page, err := service.projection.Page(ctx, ports.RankingProjectionQuery{
		Kind: kind, Cursor: cursor, Limit: limit,
	})
	if err != nil {
		if errors.Is(err, entity.ErrRankingRevisionChanged) {
			return nil, apperr.New(apperr.CodeConflict, "ranking changed; restart pagination", err)
		}
		if errors.Is(err, entity.ErrInvalidRankingQuery) {
			return nil, invalidRankingQuery(err)
		}
		return nil, err
	}
	entries := make([]*contract.RankingEntry, len(page.Entries))
	for index, projected := range page.Entries {
		entries[index] = &contract.RankingEntry{
			Rank: projected.Position, PublicID: projected.PublicID,
			DisplayName: projected.DisplayName, DisplayMode: projected.DisplayMode,
			Rating: projected.Rating, TotalExp: projected.TotalExp,
		}
	}
	if err := service.decorate(ctx, kind, entries); err != nil {
		return nil, err
	}
	response := &contract.RankingPageResponse{
		Entries: entries, ProjectionRevision: page.ProjectionRevision,
	}
	if page.HasMore && len(page.Entries) > 0 {
		last := page.Entries[len(page.Entries)-1]
		response.NextCursor, err = encodeRankingCursor(entity.RankingCursor{
			Version: 1, Kind: kind,
			ProjectionRevision: page.ProjectionRevision,
			Score:              last.Score(kind), UpdatedAt: last.UpdatedAt,
			PublicID: last.PublicID,
		})
		if err != nil {
			return nil, err
		}
	}
	return response, nil
}

func (service *rankingService) Privacy(
	ctx context.Context,
	userID uuid.UUID,
) (*contract.RankingPrivacyResponse, error) {
	if service == nil || ctx == nil || userID == uuid.Nil {
		return nil, invalidRankingQuery(entity.ErrInvalidRankingQuery)
	}
	privacy, err := service.projection.Privacy(ctx, userID)
	if err != nil {
		return nil, mapRankingPrivacyError(err)
	}
	return &contract.RankingPrivacyResponse{
		Visibility: string(privacy.Visibility), Version: privacy.Version,
		ProjectionRevision: privacy.ProjectionRevision,
	}, nil
}

func (service *rankingService) UpdatePrivacy(
	ctx context.Context,
	command entity.RankingPrivacyCommand,
) (*contract.RankingPrivacyResponse, error) {
	if service == nil || ctx == nil {
		return nil, invalidRankingQuery(entity.ErrInvalidRankingQuery)
	}
	command, err := entity.NormalizeRankingPrivacyCommand(command)
	if err != nil {
		return nil, invalidRankingQuery(err)
	}
	var result entity.RankingPrivacyResult
	err = service.journal.Execute(ctx, func(txCtx context.Context) (platformmutation.Command, error) {
		var updateErr error
		result, updateErr = service.projection.UpdatePrivacy(txCtx, command)
		if updateErr != nil {
			return platformmutation.Command{}, updateErr
		}
		if result.Replayed {
			return platformmutation.Command{Noop: true}, nil
		}
		return platformmutation.Command{
			EventID:    command.ID,
			Action:     "cultivation.ranking_privacy.updated.v1",
			Resource:   "cultivation_ranking_privacy",
			ResourceID: command.UserID.String(), Reason: command.Reason,
			Before: map[string]any{
				"visibility": result.PreviousVisibility,
				"version":    result.Privacy.Version - 1,
			},
			After: map[string]any{
				"visibility":          result.Privacy.Visibility,
				"version":             result.Privacy.Version,
				"projection_revision": result.Privacy.ProjectionRevision,
			},
			EventType:        "cultivation.ranking_privacy.updated.v1",
			AggregateType:    "cultivation_ranking_privacy",
			AggregateID:      command.UserID,
			AggregateVersion: result.Privacy.Version,
			Payload: map[string]any{
				"user_id":             command.UserID.String(),
				"visibility":          result.Privacy.Visibility,
				"privacy_version":     result.Privacy.Version,
				"projection_revision": result.Privacy.ProjectionRevision,
			},
		}, nil
	})
	if err != nil {
		return nil, mapRankingPrivacyError(err)
	}
	return &contract.RankingPrivacyResponse{
		Visibility:         string(result.Privacy.Visibility),
		Version:            result.Privacy.Version,
		ProjectionRevision: result.Privacy.ProjectionRevision,
		Replayed:           result.Replayed,
	}, nil
}

func mapRankingPrivacyError(err error) error {
	switch {
	case errors.Is(err, entity.ErrInvalidRankingQuery):
		return invalidRankingQuery(err)
	case errors.Is(err, entity.ErrRankingPrivacyConflict):
		return apperr.New(apperr.CodeConflict, "ranking privacy changed; refresh and retry", err)
	case errors.Is(err, entity.ErrRankingProjectionNotFound):
		return apperr.New(apperr.CodeNotFound, "ranking projection not found", err)
	default:
		return err
	}
}

func (service *rankingService) decorate(ctx context.Context, kind entity.RankingKind, entries []*contract.RankingEntry) error {
	if kind == entity.RankingByRating {
		ranks, err := service.rankRepo.FindAll(ctx)
		if err != nil {
			return err
		}
		sort.Slice(ranks, func(i, j int) bool { return ranks[i].MinRating > ranks[j].MinRating })
		for _, entry := range entries {
			for _, rank := range ranks {
				if rank.MinRating <= entry.Rating {
					entry.RankTitle = rank.Name
					break
				}
			}
		}
		return nil
	}
	levels, err := service.levelRepo.FindAll(ctx)
	if err != nil {
		return err
	}
	sort.Slice(levels, func(i, j int) bool { return levels[i].MinExp > levels[j].MinExp })
	for _, entry := range entries {
		for index, level := range levels {
			if level.MinExp <= entry.TotalExp {
				entry.LevelName = level.Name
				entry.Level = len(levels) - index
				break
			}
		}
	}
	return nil
}

type rankingCursorDocument struct {
	Version            int                `json:"v"`
	Kind               entity.RankingKind `json:"kind"`
	ProjectionRevision int64              `json:"revision"`
	Score              int64              `json:"score"`
	UpdatedAt          time.Time          `json:"updated_at"`
	PublicID           uuid.UUID          `json:"public_id"`
}

func encodeRankingCursor(cursor entity.RankingCursor) (string, error) {
	if !validRankingCursor(cursor, cursor.Kind) {
		return "", entity.ErrInvalidRankingQuery
	}
	encoded, err := json.Marshal(rankingCursorDocument{
		Version: cursor.Version, Kind: cursor.Kind,
		ProjectionRevision: cursor.ProjectionRevision,
		Score:              cursor.Score, UpdatedAt: cursor.UpdatedAt.UTC(), PublicID: cursor.PublicID,
	})
	if err != nil || len(encoded) > maximumRankingCursorBytes {
		return "", entity.ErrInvalidRankingQuery
	}
	return base64.RawURLEncoding.EncodeToString(encoded), nil
}

func decodeRankingCursor(value string, kind entity.RankingKind) (*entity.RankingCursor, error) {
	if value == "" {
		return nil, nil
	}
	if len(value) > base64.RawURLEncoding.EncodedLen(maximumRankingCursorBytes) {
		return nil, entity.ErrInvalidRankingQuery
	}
	decoded, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil || len(decoded) == 0 || len(decoded) > maximumRankingCursorBytes {
		return nil, entity.ErrInvalidRankingQuery
	}
	var document rankingCursorDocument
	decoder := json.NewDecoder(bytes.NewReader(decoded))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&document); err != nil {
		return nil, entity.ErrInvalidRankingQuery
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return nil, entity.ErrInvalidRankingQuery
	}
	cursor := &entity.RankingCursor{
		Version: document.Version, Kind: document.Kind,
		ProjectionRevision: document.ProjectionRevision,
		Score:              document.Score, UpdatedAt: document.UpdatedAt.UTC(),
		PublicID: document.PublicID,
	}
	if !validRankingCursor(*cursor, kind) {
		return nil, entity.ErrInvalidRankingQuery
	}
	return cursor, nil
}

func validRankingCursor(cursor entity.RankingCursor, kind entity.RankingKind) bool {
	return cursor.Version == 1 && cursor.Kind == kind && kind.Valid() &&
		cursor.ProjectionRevision >= 0 && cursor.Score >= 0 &&
		!cursor.UpdatedAt.IsZero() && cursor.PublicID != uuid.Nil
}

func invalidRankingQuery(err error) error {
	return apperr.New(apperr.CodeBadRequest, "invalid ranking query", err)
}
