package eventing

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/google/uuid"

	"entgo.io/ent/dialect/sql"

	commonent "github.com/huynhanx03/go-common/pkg/database/ent"
	commonoutbox "github.com/huynhanx03/go-common/pkg/mq/outbox"

	"github.com/huynhanx03/judgify/internal/ent/generate"
	generatedoutbox "github.com/huynhanx03/judgify/internal/ent/generate/outboxevent"
)

const (
	maximumClaimLimit        = 100
	maximumOwnerBytes        = 256
	maximumLeaseBytes        = 256
	maximumCodeBytes         = 64
	maximumDestinationBytes  = 128
	maximumClaimDestinations = 128
	maximumReferenceSize     = 256
	maximumLeaseDuration     = 24 * time.Hour
	maximumRetryDelay        = 30 * 24 * time.Hour
)

var ErrInvalidStoreOperation = errors.New("eventing store: invalid operation")

// ReplayDeadRequest is an operator-supplied compare-and-swap fence. A replay
// resets the delivery-attempt budget for one retained dead event only when the
// caller observed its current attempt count.
type ReplayDeadRequest struct {
	EventID          uuid.UUID
	ExpectedAttempts uint32
}

// Store adapts Judgify's Ent outbox table to go-common's transport-neutral
// leased relay contract. Every claim is one PostgreSQL transaction using
// FOR UPDATE SKIP LOCKED, so concurrent worker processes cannot own the same
// row and a failed claim never returns a partially committed batch.
type Store struct {
	client *generate.Client
	clock  func() time.Time
	token  func() string
}

func NewStore(client *generate.Client) (*Store, error) {
	if client == nil {
		return nil, ErrInvalidStoreOperation
	}
	return &Store{
		client: client,
		clock:  func() time.Time { return time.Now().UTC() },
		token:  func() string { return uuid.NewString() },
	}, nil
}

var _ commonoutbox.Store = (*Store)(nil)

func (store *Store) Claim(
	ctx context.Context,
	request commonoutbox.ClaimRequest,
) ([]commonoutbox.ClaimedMessage, error) {
	if store == nil || store.client == nil || store.clock == nil || store.token == nil || ctx == nil ||
		request.Limit <= 0 || request.Limit > maximumClaimLimit ||
		request.LeaseDuration <= 0 || request.LeaseDuration > maximumLeaseDuration ||
		!validStoreText(request.Owner, maximumOwnerBytes, false) ||
		!validClaimDestinations(request.Destinations) {
		return nil, ErrInvalidStoreOperation
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	now := store.clock().UTC()
	leasedUntil := now.Add(request.LeaseDuration)
	if now.IsZero() || !leasedUntil.After(now) {
		return nil, ErrInvalidStoreOperation
	}
	claimed := make([]commonoutbox.ClaimedMessage, 0, request.Limit)
	err := commonent.WithTx(ctx, store.client.Tx, func(transaction *generate.Tx) error {
		client := transaction.Client()
		query := client.OutboxEvent.Query().
			Where(generatedoutbox.Or(
				generatedoutbox.And(
					generatedoutbox.StateEQ(generatedoutbox.StatePending),
					generatedoutbox.NextAttemptAtLTE(now),
				),
				generatedoutbox.And(
					generatedoutbox.StateEQ(generatedoutbox.StateLeased),
					generatedoutbox.LeasedUntilLTE(now),
				),
			))
		if len(request.Destinations) > 0 {
			query = query.Where(generatedoutbox.EventTypeIn(request.Destinations...))
		}
		records, err := query.
			Order(
				generatedoutbox.ByNextAttemptAt(),
				generatedoutbox.ByOccurredAt(),
				generatedoutbox.ByID(),
			).
			Limit(request.Limit).
			Modify(func(selector *sql.Selector) {
				selector.ForUpdate(sql.WithLockAction(sql.SkipLocked))
			}).
			All(ctx)
		if err != nil {
			return fmt.Errorf("select outbox claims: %w", err)
		}

		batch := make([]commonoutbox.ClaimedMessage, 0, len(records))
		for _, record := range records {
			leaseToken := store.token()
			if !validStoreText(leaseToken, maximumLeaseBytes, false) {
				return ErrInvalidStoreOperation
			}
			attempt := record.AttemptCount + 1
			if attempt == 0 {
				return ErrInvalidStoreOperation
			}
			updated, err := client.OutboxEvent.Update().
				Where(
					generatedoutbox.IDEQ(record.ID),
					generatedoutbox.StateEQ(record.State),
					generatedoutbox.AttemptCountEQ(record.AttemptCount),
				).
				SetState(generatedoutbox.StateLeased).
				SetAttemptCount(attempt).
				SetLeaseOwner(request.Owner).
				SetLeaseToken(leaseToken).
				SetLeasedUntil(leasedUntil).
				SetLastAttemptAt(now).
				Save(ctx)
			if err != nil {
				return fmt.Errorf("lease outbox event: %w", err)
			}
			if updated != 1 {
				return fmt.Errorf("%w: claim fence changed", commonoutbox.ErrStoreContract)
			}
			batch = append(batch, commonoutbox.ClaimedMessage{
				Lease: commonoutbox.Lease{
					MessageID: record.ID.String(),
					Token:     leaseToken,
					Attempt:   attempt,
				},
				Message: messageFromRecord(record),
			})
		}
		claimed = batch
		return nil
	})
	if err != nil {
		return nil, err
	}
	return claimed, nil
}

func validClaimDestinations(destinations []string) bool {
	if len(destinations) > maximumClaimDestinations {
		return false
	}
	seen := make(map[string]struct{}, len(destinations))
	for _, destination := range destinations {
		if !validStoreText(destination, maximumDestinationBytes, false) {
			return false
		}
		if _, exists := seen[destination]; exists {
			return false
		}
		seen[destination] = struct{}{}
	}
	return true
}

func (store *Store) MarkPublished(
	ctx context.Context,
	lease commonoutbox.Lease,
	ack commonoutbox.PublishAck,
) error {
	if !validStoreText(ack.Reference, maximumReferenceSize, true) {
		ack.Reference = ""
	}
	return store.finalize(ctx, lease, func(update *generate.OutboxEventUpdate, now time.Time) {
		update.SetState(generatedoutbox.StatePublished).
			SetPublishedAt(now).
			SetNillablePublishReference(optionalString(ack.Reference)).
			ClearLastErrorCode().
			SetAmbiguous(false)
	})
}

func (store *Store) MarkRetry(
	ctx context.Context,
	lease commonoutbox.Lease,
	disposition commonoutbox.RetryDisposition,
) error {
	if disposition.Delay < 0 || disposition.Delay > maximumRetryDelay ||
		!validStoreText(disposition.Code, maximumCodeBytes, false) {
		return ErrInvalidStoreOperation
	}
	return store.finalize(ctx, lease, func(update *generate.OutboxEventUpdate, now time.Time) {
		update.SetState(generatedoutbox.StatePending).
			SetNextAttemptAt(now.Add(disposition.Delay)).
			SetLastErrorCode(disposition.Code).
			SetAmbiguous(disposition.Ambiguous)
	})
}

func (store *Store) MarkDead(
	ctx context.Context,
	lease commonoutbox.Lease,
	disposition commonoutbox.DeadDisposition,
) error {
	if !validStoreText(disposition.Code, maximumCodeBytes, false) {
		return ErrInvalidStoreOperation
	}
	return store.finalize(ctx, lease, func(update *generate.OutboxEventUpdate, now time.Time) {
		update.SetState(generatedoutbox.StateDead).
			SetDeadAt(now).
			SetLastErrorCode(disposition.Code).
			SetAmbiguous(disposition.Ambiguous)
	})
}

// ReplayDead atomically returns one retained dead event to pending state. It
// clears terminal and lease state and begins a fresh bounded attempt cycle.
// The expected-attempt fence prevents two operators or automation loops from
// replaying the same observed failure concurrently.
func (store *Store) ReplayDead(
	ctx context.Context,
	request ReplayDeadRequest,
) (bool, error) {
	if store == nil || store.client == nil || store.clock == nil || ctx == nil ||
		request.EventID == uuid.Nil || request.ExpectedAttempts == 0 {
		return false, ErrInvalidStoreOperation
	}
	if err := ctx.Err(); err != nil {
		return false, err
	}
	now := store.clock().UTC()
	if now.IsZero() {
		return false, ErrInvalidStoreOperation
	}
	updated, err := store.client.OutboxEvent.Update().
		Where(
			generatedoutbox.IDEQ(request.EventID),
			generatedoutbox.StateEQ(generatedoutbox.StateDead),
			generatedoutbox.AttemptCountEQ(request.ExpectedAttempts),
		).
		SetState(generatedoutbox.StatePending).
		SetAttemptCount(0).
		SetNextAttemptAt(now).
		SetAmbiguous(false).
		ClearLeaseOwner().
		ClearLeaseToken().
		ClearLeasedUntil().
		ClearLastAttemptAt().
		ClearPublishedAt().
		ClearDeadAt().
		ClearPublishReference().
		ClearLastErrorCode().
		Save(ctx)
	if err != nil {
		return false, fmt.Errorf("replay dead outbox event: %w", err)
	}
	return updated == 1, nil
}

func (store *Store) finalize(
	ctx context.Context,
	lease commonoutbox.Lease,
	transition func(*generate.OutboxEventUpdate, time.Time),
) error {
	if store == nil || store.client == nil || store.clock == nil || ctx == nil || transition == nil ||
		!validStoreText(lease.MessageID, maximumLeaseBytes, false) ||
		!validStoreText(lease.Token, maximumLeaseBytes, false) || lease.Attempt == 0 {
		return ErrInvalidStoreOperation
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	messageID, err := uuid.Parse(lease.MessageID)
	if err != nil || messageID == uuid.Nil {
		return ErrInvalidStoreOperation
	}
	now := store.clock().UTC()
	if now.IsZero() {
		return ErrInvalidStoreOperation
	}
	update := store.client.OutboxEvent.Update().Where(
		generatedoutbox.IDEQ(messageID),
		generatedoutbox.StateEQ(generatedoutbox.StateLeased),
		generatedoutbox.LeaseTokenEQ(lease.Token),
		generatedoutbox.AttemptCountEQ(lease.Attempt),
	)
	transition(update, now)
	updated, err := update.
		ClearLeaseOwner().
		ClearLeaseToken().
		ClearLeasedUntil().
		Save(ctx)
	if err != nil {
		return fmt.Errorf("finalize outbox event: %w", err)
	}
	if updated != 1 {
		return commonoutbox.ErrLeaseLost
	}
	return nil
}

func messageFromRecord(record *generate.OutboxEvent) commonoutbox.Message {
	if record == nil {
		return commonoutbox.Message{}
	}
	return commonoutbox.Message{
		ID:          record.ID.String(),
		Destination: record.EventType,
		Key:         []byte(record.AggregateID.String()),
		Payload:     append([]byte(nil), record.Payload...),
		Metadata: commonoutbox.Metadata{
			CorrelationID: record.CorrelationID,
			Attributes: []commonoutbox.Attribute{
				{Key: "aggregate_type", Value: record.AggregateType},
				{Key: "aggregate_version", Value: strconv.FormatInt(record.AggregateVersion, 10)},
				{Key: "occurred_at", Value: record.OccurredAt.UTC().Format(time.RFC3339Nano)},
			},
		},
	}
}

func optionalString(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}

func validStoreText(value string, maximum int, allowEmpty bool) bool {
	if (!allowEmpty && value == "") || len(value) > maximum || !utf8.ValidString(value) {
		return false
	}
	for _, character := range value {
		if unicode.IsControl(character) || unicode.In(character, unicode.Cf) {
			return false
		}
	}
	return !strings.ContainsRune(value, '\x00')
}
