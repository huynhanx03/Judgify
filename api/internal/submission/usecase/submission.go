package usecase

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/binary"
	"math"
	"time"

	"github.com/google/uuid"

	"github.com/huynhanx03/go-common/pkg/common/apperr"
	commondto "github.com/huynhanx03/go-common/pkg/dto"
	"github.com/huynhanx03/go-common/pkg/logger"
	"go.uber.org/zap"

	"github.com/huynhanx03/judgify/internal/ports"
	contract "github.com/huynhanx03/judgify/internal/submission/contract"
	entity "github.com/huynhanx03/judgify/internal/submission/domain"
	"github.com/huynhanx03/judgify/pkg/judging"
)

type submissionService struct {
	submissionRepo ports.SubmissionRepository
}

// NewSubmissionService creates the read-only submission service. Submission
// creation is owned exclusively by SubmissionCommandService and its durable
// transactional workflow.
func NewSubmissionService(submissionRepo ports.SubmissionRepository) ports.SubmissionService {
	return &submissionService{submissionRepo: submissionRepo}
}

func (s *submissionService) Get(ctx context.Context, userID, id uuid.UUID) (*contract.SubmissionResponse, error) {
	e, err := s.submissionRepo.GetOwned(ctx, id, userID)
	if err != nil {
		return nil, err
	}
	return ToSubmissionResponse(e), nil
}

func (s *submissionService) FindByProblemID(ctx context.Context, problemID uuid.UUID) ([]*contract.SubmissionSummaryResponse, error) {
	entities, err := s.submissionRepo.FindByProblemID(ctx, problemID)
	if err != nil {
		return nil, err
	}

	responses := make([]*contract.SubmissionSummaryResponse, len(entities))
	for i, e := range entities {
		responses[i] = ToSubmissionSummaryResponse(e)
	}
	return responses, nil
}

const (
	submissionCursorVersion = byte(1)
	submissionCursorBytes   = 1 + 8 + 16
)

func (s *submissionService) FindMyByProblem(
	ctx context.Context,
	userID uuid.UUID,
	req *contract.MySubmissionsRequest,
) (*contract.MySubmissionsPageResponse, error) {
	if s == nil || s.submissionRepo == nil || ctx == nil ||
		userID == uuid.Nil || req == nil || req.ProblemID == uuid.Nil {
		return nil, invalidMySubmissionPage()
	}
	limit := req.Limit
	if limit == 0 {
		limit = ports.DefaultMySubmissionPageLimit
	}
	query := ports.MySubmissionQuery{
		UserID: userID, ProblemID: req.ProblemID,
		ContestID: cloneUUIDPointer(req.ContestID), Limit: limit,
	}
	if req.Cursor != "" {
		after, err := decodeSubmissionCursor(req.Cursor)
		if err != nil {
			return nil, invalidMySubmissionPage()
		}
		query.After = &after
	}
	if err := query.Validate(); err != nil {
		return nil, invalidMySubmissionPage()
	}
	page, err := s.submissionRepo.FindMyByProblem(ctx, query)
	if err != nil {
		return nil, err
	}
	if len(page.Submissions) > limit ||
		(page.Next != nil && len(page.Submissions) == 0) {
		return nil, invalidMySubmissionPageProjection()
	}
	result := &contract.MySubmissionsPageResponse{
		Items: make([]*contract.SubmissionSummaryResponse, 0, len(page.Submissions)),
	}
	boundary := query.After
	for _, submission := range page.Submissions {
		if !validMySubmissionProjection(submission, query) {
			return nil, invalidMySubmissionPageProjection()
		}
		current := ports.SubmissionCursor{
			CreatedAt: submission.CreatedAt.UTC(),
			ID:        submission.ID,
		}
		if boundary != nil &&
			!submissionCursorStrictlyOlder(current, *boundary) {
			return nil, invalidMySubmissionPageProjection()
		}
		boundary = &current
		result.Items = append(
			result.Items,
			ToSubmissionSummaryResponse(submission),
		)
	}
	if page.Next != nil {
		last := page.Submissions[len(page.Submissions)-1]
		if page.Next.ID != last.ID ||
			!page.Next.CreatedAt.Equal(last.CreatedAt) {
			return nil, invalidMySubmissionPageProjection()
		}
		encoded, encodeErr := encodeSubmissionCursor(*page.Next)
		if encodeErr != nil {
			return nil, invalidMySubmissionPageProjection()
		}
		result.NextCursor = &encoded
		result.HasMore = true
	}
	logger.FromContext(ctx).Debug(
		"owner submission history listed",
		zap.String("user_id", userID.String()),
		zap.String("problem_id", req.ProblemID.String()),
		zap.String("contest_id", optionalUUIDString(req.ContestID)),
		zap.Int("count", len(result.Items)),
		zap.Bool("has_more", result.HasMore),
	)
	return result, nil
}

func validMySubmissionProjection(
	value *entity.Submission,
	query ports.MySubmissionQuery,
) bool {
	if value == nil || value.ID == uuid.Nil ||
		value.UserID != query.UserID || value.ProblemID != query.ProblemID ||
		value.CreatedAt.IsZero() || !entity.IsSubmissionStatus(value.Status) ||
		value.PassedCount < 0 || value.TotalCount < 0 ||
		value.PassedCount > value.TotalCount {
		return false
	}
	if query.ContestID != nil &&
		(value.ContestID == nil || *value.ContestID != *query.ContestID) {
		return false
	}
	_, supported := judging.ParseLanguage(value.Language)
	return supported
}

func submissionCursorStrictlyOlder(
	current ports.SubmissionCursor,
	boundary ports.SubmissionCursor,
) bool {
	if current.CreatedAt.Before(boundary.CreatedAt) {
		return true
	}
	return current.CreatedAt.Equal(boundary.CreatedAt) &&
		bytes.Compare(current.ID[:], boundary.ID[:]) < 0
}

func cloneUUIDPointer(value *uuid.UUID) *uuid.UUID {
	if value == nil {
		return nil
	}
	cloned := *value
	return &cloned
}

func optionalUUIDString(value *uuid.UUID) string {
	if value == nil {
		return ""
	}
	return value.String()
}

func encodeSubmissionCursor(cursor ports.SubmissionCursor) (string, error) {
	nanoseconds := cursor.CreatedAt.UTC().UnixNano()
	if cursor.ID == uuid.Nil || cursor.CreatedAt.IsZero() || nanoseconds <= 0 {
		return "", ports.ErrInvalidMySubmissionPage
	}
	payload := make([]byte, submissionCursorBytes)
	payload[0] = submissionCursorVersion
	binary.BigEndian.PutUint64(payload[1:9], uint64(nanoseconds))
	copy(payload[9:], cursor.ID[:])
	return base64.RawURLEncoding.EncodeToString(payload), nil
}

func decodeSubmissionCursor(raw string) (ports.SubmissionCursor, error) {
	expectedLength := base64.RawURLEncoding.EncodedLen(submissionCursorBytes)
	if len(raw) != expectedLength {
		return ports.SubmissionCursor{}, ports.ErrInvalidMySubmissionPage
	}
	payload, err := base64.RawURLEncoding.Strict().DecodeString(raw)
	if err != nil || len(payload) != submissionCursorBytes ||
		payload[0] != submissionCursorVersion ||
		base64.RawURLEncoding.EncodeToString(payload) != raw {
		return ports.SubmissionCursor{}, ports.ErrInvalidMySubmissionPage
	}
	nanoseconds := binary.BigEndian.Uint64(payload[1:9])
	if nanoseconds == 0 || nanoseconds > math.MaxInt64 {
		return ports.SubmissionCursor{}, ports.ErrInvalidMySubmissionPage
	}
	id, err := uuid.FromBytes(payload[9:])
	if err != nil || id == uuid.Nil {
		return ports.SubmissionCursor{}, ports.ErrInvalidMySubmissionPage
	}
	createdAt := time.Unix(0, int64(nanoseconds)).UTC()
	if createdAt.IsZero() {
		return ports.SubmissionCursor{}, ports.ErrInvalidMySubmissionPage
	}
	return ports.SubmissionCursor{CreatedAt: createdAt, ID: id}, nil
}

func invalidMySubmissionPage() error {
	return apperr.New(
		apperr.CodeBadRequest,
		"invalid submission history query",
		ports.ErrInvalidMySubmissionPage,
	)
}

func invalidMySubmissionPageProjection() error {
	return apperr.New(
		apperr.CodeInternalServer,
		"submission history is unavailable",
		ports.ErrInvalidMySubmissionPage,
	)
}

func (s *submissionService) FindAdmin(
	ctx context.Context,
	req *contract.AdminSubmissionFindRequest,
) (*commondto.Paginated[*contract.AdminSubmissionSummaryResponse], error) {
	query := entity.AdminSubmissionQuery{}
	if req != nil {
		if req.Pagination != nil {
			query.Page = req.Pagination.Page
			query.PageSize = req.Pagination.PageSize
		}
		if req.Status != nil {
			query.Status = *req.Status
		}
		if req.Language != nil {
			query.Language = *req.Language
		}
		query.ProblemID = req.ProblemID
		query.UserID = req.UserID
		query.ContestID = req.ContestID
	}

	normalized, err := entity.NormalizeAdminSubmissionQuery(query)
	if err != nil {
		return nil, apperr.New(
			apperr.CodeBadRequest,
			"invalid admin submission query",
			err,
		)
	}
	page, err := s.submissionRepo.FindAdmin(ctx, normalized)
	if err != nil {
		return nil, err
	}

	responses := make([]*contract.AdminSubmissionSummaryResponse, len(*page.Records))
	for index, submission := range *page.Records {
		responses[index] = ToAdminSubmissionSummaryResponse(submission)
	}
	return &commondto.Paginated[*contract.AdminSubmissionSummaryResponse]{
		Records:    &responses,
		Pagination: page.Pagination,
	}, nil
}

func (s *submissionService) GetAdmin(
	ctx context.Context,
	id uuid.UUID,
) (*contract.AdminSubmissionDetailResponse, error) {
	if id == uuid.Nil {
		return nil, apperr.New(apperr.CodeBadRequest, "invalid submission id", nil)
	}
	submission, err := s.submissionRepo.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	return ToAdminSubmissionDetailResponse(submission), nil
}
