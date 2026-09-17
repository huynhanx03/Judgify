package audit

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"

	entsql "entgo.io/ent/dialect/sql"

	"github.com/huynhanx03/go-common/pkg/common/apperr"
	"github.com/huynhanx03/go-common/pkg/correlation"
	"github.com/huynhanx03/go-common/pkg/logger"

	internalent "github.com/huynhanx03/judgify/internal/ent"
	"github.com/huynhanx03/judgify/internal/ent/generate"
	generatedaudit "github.com/huynhanx03/judgify/internal/ent/generate/auditlog"
	"github.com/huynhanx03/judgify/internal/ent/generate/predicate"
	"github.com/huynhanx03/judgify/internal/platform/internal/boundedjson"
)

const (
	DefaultSearchLimit = 25
	MaximumSearchLimit = 100

	maxAuditCursorBytes = 64
	cursorVersion       = byte(1)
	cursorBinaryBytes   = 1 + 8 + 4 + 16

	// PostgreSQL's finite timestamp range is [4713 BC, 294277 AD). Keeping
	// cursors inside that interval prevents a forged keyset value from reaching
	// the driver only to fail as an out-of-range timestamptz parameter.
	minimumAuditTimestampUnixSeconds          = int64(-210866803200)
	maximumAuditTimestampUnixSecondsExclusive = int64(9224318016000)
)

var (
	ErrInvalidQuery       = errors.New("audit: invalid query")
	ErrInvalidSearch      = errors.New("audit: invalid search request")
	ErrUnsafeStoredRecord = errors.New("audit: unsafe stored record")
)

// SearchRequest contains only the public, exact-match audit filters. HTTP
// parsing owns unknown and duplicate query-key rejection before this value is
// constructed.
type SearchRequest struct {
	ActorUserID   string
	Action        string
	Resource      string
	ResourceID    string
	CorrelationID string
	From          string
	To            string
	Cursor        string
	Limit         string
}

// Cursor identifies the last row of a descending (occurred_at, id) page.
// Both fields are required together; callers never construct SQL ordering.
type Cursor struct {
	OccurredAt time.Time
	ID         uuid.UUID
}

// Filter is the closed persistence query contract. Time windows are
// [From, To), and Cursor always means rows strictly older than its tuple.
type Filter struct {
	ActorUserID   *uuid.UUID
	Action        string
	Resource      string
	ResourceID    string
	CorrelationID string
	From          *time.Time
	To            *time.Time
	Cursor        Cursor
	Limit         int
}

// Item is the minimum administrative audit projection. Actor/internal session
// identifiers and storage bookkeeping timestamps never leave this boundary.
// A target_session_id inside a document remains auditable because it identifies
// the affected resource rather than the actor's authentication context.
type Item struct {
	ID            uuid.UUID      `json:"id"`
	ActorUserID   *uuid.UUID     `json:"actor_user_id,omitempty"`
	Action        string         `json:"action"`
	Resource      string         `json:"resource"`
	ResourceID    string         `json:"resource_id"`
	Reason        string         `json:"reason"`
	Before        map[string]any `json:"before"`
	After         map[string]any `json:"after"`
	CorrelationID string         `json:"correlation_id"`
	OccurredAt    time.Time      `json:"occurred_at"`
}

// Page is a bounded keyset page. NextCursor is emitted only when HasMore is
// true and points at the last returned item.
type Page struct {
	Items      []Item `json:"items"`
	NextCursor string `json:"next_cursor,omitempty"`
	HasMore    bool   `json:"has_more"`
}

// Searcher is the read capability consumed by the HTTP adapter.
type Searcher interface {
	Search(context.Context, Filter) (*Page, error)
}

// Query reads immutable audit rows through a fixed field projection and
// defensively canonicalizes/redacts their JSON documents before release.
type Query struct {
	client   *internalent.EntClient
	redacted map[string]struct{}
}

func NewQuery(client *internalent.EntClient) (*Query, error) {
	if client == nil || client.Client() == nil {
		return nil, ErrInvalidQuery
	}
	redacted := make(map[string]struct{}, len(defaultRedactedKeys))
	for _, key := range defaultRedactedKeys {
		redacted[normalizeKey(key)] = struct{}{}
	}
	return &Query{client: client, redacted: redacted}, nil
}

// ParseSearchRequest strictly parses public strings so malformed UUIDs,
// timestamps, limits, or cursors can never silently degrade into a broad
// unfiltered search.
func ParseSearchRequest(request SearchRequest) (Filter, error) {
	filter := Filter{Limit: DefaultSearchLimit}
	var err error
	if request.ActorUserID != "" {
		value, parseErr := uuid.Parse(request.ActorUserID)
		if parseErr != nil || value == uuid.Nil || value.String() != strings.ToLower(request.ActorUserID) {
			return Filter{}, ErrInvalidSearch
		}
		filter.ActorUserID = &value
	}
	if request.Action != "" && !validIdentifier(request.Action, maxAuditActionBytes) {
		return Filter{}, ErrInvalidSearch
	}
	filter.Action = request.Action
	if request.Resource != "" && !validIdentifier(request.Resource, maxAuditResourceBytes) {
		return Filter{}, ErrInvalidSearch
	}
	filter.Resource = request.Resource
	if request.ResourceID != "" && !validText(request.ResourceID, maxAuditResourceID, false) {
		return Filter{}, ErrInvalidSearch
	}
	filter.ResourceID = request.ResourceID
	if request.CorrelationID != "" {
		if correlation.Validate(request.CorrelationID) != nil {
			return Filter{}, ErrInvalidSearch
		}
		filter.CorrelationID = request.CorrelationID
	}
	if request.From != "" {
		filter.From, err = parseSearchTime(request.From)
		if err != nil {
			return Filter{}, ErrInvalidSearch
		}
	}
	if request.To != "" {
		filter.To, err = parseSearchTime(request.To)
		if err != nil {
			return Filter{}, ErrInvalidSearch
		}
	}
	if request.Limit != "" {
		filter.Limit, err = strconv.Atoi(request.Limit)
		if err != nil {
			return Filter{}, ErrInvalidSearch
		}
	}
	if request.Cursor != "" {
		filter.Cursor, err = decodeCursor(request.Cursor)
		if err != nil {
			return Filter{}, ErrInvalidSearch
		}
	}
	if err := filter.Validate(); err != nil {
		return Filter{}, ErrInvalidSearch
	}
	return filter, nil
}

func (filter Filter) Validate() error {
	if filter.Limit < 1 || filter.Limit > MaximumSearchLimit {
		return ErrInvalidSearch
	}
	if filter.ActorUserID != nil && *filter.ActorUserID == uuid.Nil {
		return ErrInvalidSearch
	}
	if filter.Action != "" && !validIdentifier(filter.Action, maxAuditActionBytes) {
		return ErrInvalidSearch
	}
	if filter.Resource != "" && !validIdentifier(filter.Resource, maxAuditResourceBytes) {
		return ErrInvalidSearch
	}
	if filter.ResourceID != "" && !validText(filter.ResourceID, maxAuditResourceID, false) {
		return ErrInvalidSearch
	}
	if filter.CorrelationID != "" && correlation.Validate(filter.CorrelationID) != nil {
		return ErrInvalidSearch
	}
	if (filter.From != nil && filter.From.IsZero()) || (filter.To != nil && filter.To.IsZero()) {
		return ErrInvalidSearch
	}
	if (filter.From != nil && !validAuditTimestamp(*filter.From)) ||
		(filter.To != nil && !validAuditTimestamp(*filter.To)) {
		return ErrInvalidSearch
	}
	if filter.From != nil && filter.To != nil && !filter.From.Before(*filter.To) {
		return ErrInvalidSearch
	}
	cursorPresent := filter.Cursor.ID != uuid.Nil || !filter.Cursor.OccurredAt.IsZero()
	if cursorPresent && (filter.Cursor.ID == uuid.Nil || !validAuditTimestamp(filter.Cursor.OccurredAt)) {
		return ErrInvalidSearch
	}
	if cursorPresent {
		if filter.From != nil && filter.Cursor.OccurredAt.Before(*filter.From) {
			return ErrInvalidSearch
		}
		if filter.To != nil && !filter.Cursor.OccurredAt.Before(*filter.To) {
			return ErrInvalidSearch
		}
	}
	return nil
}

func (query *Query) Search(ctx context.Context, filter Filter) (*Page, error) {
	if query == nil || query.client == nil || query.redacted == nil || ctx == nil {
		return nil, ErrInvalidQuery
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := filter.Validate(); err != nil {
		return nil, err
	}

	predicates := searchPredicates(filter)
	databaseQuery := query.client.DB(ctx).AuditLog.Query().
		Where(predicates...).
		Order(
			generatedaudit.ByOccurredAt(entsql.OrderDesc()),
			generatedaudit.ByID(entsql.OrderDesc()),
		).
		Limit(filter.Limit + 1)
	databaseQuery.Select(
		generatedaudit.FieldID,
		generatedaudit.FieldActorUserID,
		generatedaudit.FieldAction,
		generatedaudit.FieldResource,
		generatedaudit.FieldResourceID,
		generatedaudit.FieldReason,
		generatedaudit.FieldBeforeData,
		generatedaudit.FieldAfterData,
		generatedaudit.FieldCorrelationID,
		generatedaudit.FieldOccurredAt,
	)
	records, err := databaseQuery.All(ctx)
	if err != nil {
		return nil, apperr.New(apperr.CodeInternalServer, "audit unavailable", fmt.Errorf("query audit records: %w", err))
	}

	hasMore := len(records) > filter.Limit
	if hasMore {
		records = records[:filter.Limit]
	}
	items := make([]Item, 0, len(records))
	for _, record := range records {
		item, convertErr := query.safeItem(record)
		if convertErr != nil {
			return nil, apperr.New(apperr.CodeInternalServer, "audit unavailable", convertErr)
		}
		items = append(items, item)
	}

	page := &Page{Items: items, HasMore: hasMore}
	if hasMore && len(items) > 0 {
		page.NextCursor, err = encodeCursor(Cursor{
			OccurredAt: items[len(items)-1].OccurredAt,
			ID:         items[len(items)-1].ID,
		})
		if err != nil {
			return nil, apperr.New(apperr.CodeInternalServer, "audit unavailable", err)
		}
	}
	logger.FromContext(ctx).Debug(
		"audit search completed",
		zap.Int("result_count", len(items)),
		zap.Bool("has_more", page.HasMore),
		zap.Bool("filter_actor", filter.ActorUserID != nil),
		zap.Bool("filter_action", filter.Action != ""),
		zap.Bool("filter_resource", filter.Resource != ""),
		zap.Bool("filter_resource_id", filter.ResourceID != ""),
		zap.Bool("filter_correlation", filter.CorrelationID != ""),
		zap.Bool("filter_from", filter.From != nil),
		zap.Bool("filter_to", filter.To != nil),
		zap.Bool("filter_cursor", filter.Cursor.ID != uuid.Nil),
	)
	return page, nil
}

func searchPredicates(filter Filter) []predicate.AuditLog {
	predicates := make([]predicate.AuditLog, 0, 9)
	if filter.ActorUserID != nil {
		predicates = append(predicates, generatedaudit.ActorUserIDEQ(*filter.ActorUserID))
	}
	if filter.Action != "" {
		predicates = append(predicates, generatedaudit.ActionEQ(filter.Action))
	}
	if filter.Resource != "" {
		predicates = append(predicates, generatedaudit.ResourceEQ(filter.Resource))
	}
	if filter.ResourceID != "" {
		predicates = append(predicates, generatedaudit.ResourceIDEQ(filter.ResourceID))
	}
	if filter.CorrelationID != "" {
		predicates = append(predicates, generatedaudit.CorrelationIDEQ(filter.CorrelationID))
	}
	if filter.From != nil {
		predicates = append(predicates, generatedaudit.OccurredAtGTE(filter.From.UTC()))
	}
	if filter.To != nil {
		predicates = append(predicates, generatedaudit.OccurredAtLT(filter.To.UTC()))
	}
	if filter.Cursor.ID != uuid.Nil {
		occurredAt := filter.Cursor.OccurredAt.UTC()
		predicates = append(predicates, generatedaudit.Or(
			generatedaudit.OccurredAtLT(occurredAt),
			generatedaudit.And(
				generatedaudit.OccurredAtEQ(occurredAt),
				generatedaudit.IDLT(filter.Cursor.ID),
			),
		))
	}
	return predicates
}

func (query *Query) safeItem(record *generate.AuditLog) (Item, error) {
	if record == nil || record.ID == uuid.Nil || record.OccurredAt.IsZero() ||
		!validIdentifier(record.Action, maxAuditActionBytes) ||
		!validIdentifier(record.Resource, maxAuditResourceBytes) ||
		!validText(record.ResourceID, maxAuditResourceID, true) ||
		!validText(record.Reason, maxAuditReasonBytes, false) ||
		correlation.Validate(record.CorrelationID) != nil ||
		(record.ActorUserID != nil && *record.ActorUserID == uuid.Nil) {
		return Item{}, ErrUnsafeStoredRecord
	}
	before, err := query.safeDocument(record.BeforeData)
	if err != nil {
		return Item{}, ErrUnsafeStoredRecord
	}
	after, err := query.safeDocument(record.AfterData)
	if err != nil {
		return Item{}, ErrUnsafeStoredRecord
	}
	return Item{
		ID:            record.ID,
		ActorUserID:   cloneUUID(record.ActorUserID),
		Action:        strings.Clone(record.Action),
		Resource:      strings.Clone(record.Resource),
		ResourceID:    strings.Clone(record.ResourceID),
		Reason:        strings.Clone(record.Reason),
		Before:        before,
		After:         after,
		CorrelationID: strings.Clone(record.CorrelationID),
		OccurredAt:    record.OccurredAt.UTC(),
	}, nil
}

func (query *Query) safeDocument(raw json.RawMessage) (map[string]any, error) {
	limits := boundedjson.Limits{
		MaxBytes: maxAuditDocumentBytes,
		MaxDepth: maxAuditDepth,
		MaxNodes: maxAuditNodes,
	}
	canonical, err := boundedjson.CanonicalObject(raw, limits)
	if err != nil {
		return nil, err
	}
	decoded := make(map[string]any)
	decoder := json.NewDecoder(bytes.NewReader(canonical))
	decoder.UseNumber()
	if err := decoder.Decode(&decoded); err != nil {
		return nil, err
	}
	redacted, err := boundedjson.MarshalMap(decoded, limits, func(key string) bool {
		_, sensitive := query.redacted[normalizeKey(key)]
		return sensitive
	})
	if err != nil {
		return nil, err
	}
	result := make(map[string]any)
	decoder = json.NewDecoder(bytes.NewReader(redacted))
	decoder.UseNumber()
	if err := decoder.Decode(&result); err != nil {
		return nil, err
	}
	return result, nil
}

func parseSearchTime(value string) (*time.Time, error) {
	parsed, err := time.Parse(time.RFC3339Nano, value)
	if err != nil || parsed.IsZero() || value != strings.TrimSpace(value) {
		return nil, ErrInvalidSearch
	}
	parsed = parsed.UTC()
	return &parsed, nil
}

func encodeCursor(cursor Cursor) (string, error) {
	if cursor.ID == uuid.Nil || !validAuditTimestamp(cursor.OccurredAt) {
		return "", ErrInvalidSearch
	}
	payload := make([]byte, cursorBinaryBytes)
	payload[0] = cursorVersion
	binary.BigEndian.PutUint64(payload[1:9], uint64(cursor.OccurredAt.Unix()))
	binary.BigEndian.PutUint32(payload[9:13], uint32(cursor.OccurredAt.Nanosecond()))
	copy(payload[13:], cursor.ID[:])
	return base64.RawURLEncoding.EncodeToString(payload), nil
}

func decodeCursor(value string) (Cursor, error) {
	if len(value) == 0 || len(value) > maxAuditCursorBytes || value != strings.TrimSpace(value) {
		return Cursor{}, ErrInvalidSearch
	}
	payload, err := base64.RawURLEncoding.Strict().DecodeString(value)
	if err != nil || len(payload) != cursorBinaryBytes || payload[0] != cursorVersion {
		return Cursor{}, ErrInvalidSearch
	}
	nanosecond := binary.BigEndian.Uint32(payload[9:13])
	if nanosecond >= uint32(time.Second) {
		return Cursor{}, ErrInvalidSearch
	}
	id, err := uuid.FromBytes(payload[13:])
	if err != nil || id == uuid.Nil {
		return Cursor{}, ErrInvalidSearch
	}
	cursor := Cursor{
		OccurredAt: time.Unix(int64(binary.BigEndian.Uint64(payload[1:9])), int64(nanosecond)).UTC(),
		ID:         id,
	}
	if !validAuditTimestamp(cursor.OccurredAt) {
		return Cursor{}, ErrInvalidSearch
	}
	return cursor, nil
}

func validAuditTimestamp(value time.Time) bool {
	seconds := value.Unix()
	return !value.IsZero() &&
		seconds >= minimumAuditTimestampUnixSeconds &&
		seconds < maximumAuditTimestampUnixSecondsExclusive
}
