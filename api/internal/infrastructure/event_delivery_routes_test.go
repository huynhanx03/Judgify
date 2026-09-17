package infrastructure

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	commonlogger "github.com/huynhanx03/go-common/pkg/logger"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"

	commonoutbox "github.com/huynhanx03/go-common/pkg/mq/outbox"

	materialentity "github.com/huynhanx03/judgify/internal/material/domain"
)

func TestStrictJSONRouteEnrichesConsumerLogsWithEventIdentity(t *testing.T) {
	t.Parallel()

	type payload struct {
		Value string `json:"value"`
	}
	eventID := uuid.MustParse("00000000-0000-7000-8000-000000000001")
	core, logs := observer.New(zap.DebugLevel)
	ctx := commonlogger.WithContext(context.Background(), zap.New(core))
	route := strictJSONRoute(
		"account.changed.v1",
		"projection",
		1024,
		func(message commonoutbox.Message, _ payload) error {
			if message.ID != eventID.String() {
				return ErrInvalidEventDelivery
			}
			return nil
		},
		func(ctx context.Context, _ payload) error {
			commonlogger.FromContext(ctx).Info("event consumed")
			return nil
		},
		nil,
	)
	encoded, err := json.Marshal(payload{Value: "safe"})
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if err := route.Handle(ctx, commonoutbox.Message{
		ID: eventID.String(), Payload: encoded,
	}); err != nil {
		t.Fatalf("Handle: %v", err)
	}
	entries := logs.FilterMessage("event consumed").All()
	if len(entries) != 1 || entries[0].ContextMap()["event_id"] != eventID.String() {
		t.Fatalf("event log entries = %+v", entries)
	}
}

func TestValidateEventEnvelopeBindsIdentityAggregateVersionCIDAndTime(t *testing.T) {
	t.Parallel()

	eventID := uuid.MustParse("00000000-0000-7000-8000-000000000001")
	aggregateID := uuid.MustParse("00000000-0000-7000-8000-000000000002")
	occurredAt := time.Date(2026, time.July, 30, 12, 0, 0, 123456789, time.UTC)
	cid := "018f5fbe-3d77-7e10-8fd1-8b6f1ca32e17"
	message := commonoutbox.Message{
		ID:          eventID.String(),
		Destination: "account.changed.v1",
		Key:         []byte(aggregateID.String()),
		Metadata: commonoutbox.Metadata{
			CorrelationID: cid,
			Attributes: []commonoutbox.Attribute{
				{Key: "aggregate_type", Value: "account"},
				{Key: "aggregate_version", Value: "7"},
				{
					Key:   "occurred_at",
					Value: occurredAt.Truncate(time.Microsecond).Format(time.RFC3339Nano),
				},
			},
		},
	}
	expected := eventEnvelopeExpectation{
		EventID:          eventID,
		Destination:      "account.changed.v1",
		AggregateType:    "account",
		AggregateID:      aggregateID,
		AggregateVersion: 7,
		CorrelationID:    cid,
		OccurredAt:       occurredAt,
	}
	if err := validateEventEnvelope(message, expected); err != nil {
		t.Fatalf("validateEventEnvelope: %v", err)
	}

	message.Metadata.Attributes = append(
		message.Metadata.Attributes,
		commonoutbox.Attribute{Key: "aggregate_version", Value: "7"},
	)
	if !errors.Is(validateEventEnvelope(message, expected), ErrInvalidEventDelivery) {
		t.Fatal("duplicate aggregate version was accepted")
	}
}

func TestStrictJSONRouteRejectsUnknownFieldsBeforeConsumer(t *testing.T) {
	t.Parallel()

	type payload struct {
		ID uuid.UUID `json:"id"`
	}
	consumed := false
	route := strictJSONRoute(
		"account.changed.v1",
		"projection",
		1024,
		func(commonoutbox.Message, payload) error { return nil },
		func(context.Context, payload) error {
			consumed = true
			return nil
		},
		nil,
	)
	encoded, err := json.Marshal(map[string]any{
		"id":         uuid.New(),
		"unexpected": true,
	})
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	err = route.Handle(context.Background(), commonoutbox.Message{Payload: encoded})
	var failure *commonoutbox.PublishError
	if !errors.As(err, &failure) || failure.Code != "invalid_event_payload" ||
		failure.Retryable || consumed {
		t.Fatalf("route failure = %#v, consumed=%t", err, consumed)
	}
}

func TestMaterialViewEnvelopeUsesDeterministicReceiptIdentity(t *testing.T) {
	t.Parallel()

	receiptID := uuid.MustParse("00000000-0000-7000-8000-000000000001")
	materialID := uuid.MustParse("00000000-0000-7000-8000-000000000002")
	eventID := uuid.NewSHA1(
		uuid.NameSpaceURL,
		[]byte(materialentity.MaterialViewRecordedV1+"\x00"+receiptID.String()),
	)
	message := commonoutbox.Message{
		ID:          eventID.String(),
		Destination: materialentity.MaterialViewRecordedV1,
		Key:         []byte(receiptID.String()),
		Metadata: commonoutbox.Metadata{
			CorrelationID: "018f5fbe-3d77-7e10-8fd1-8b6f1ca32e17",
			Attributes: []commonoutbox.Attribute{
				{Key: "aggregate_type", Value: "material_view"},
				{Key: "aggregate_version", Value: "1"},
				{Key: "occurred_at", Value: "2026-07-30T12:00:00Z"},
			},
		},
	}
	event := materialentity.DailyViewRecorded{
		ReceiptID:  receiptID,
		MaterialID: materialID,
		ViewDay:    time.Date(2026, time.July, 30, 0, 0, 0, 0, time.UTC),
	}
	if err := validateMaterialViewEvent(message, event); err != nil {
		t.Fatalf("validateMaterialViewEvent: %v", err)
	}
	message.ID = uuid.NewString()
	if !errors.Is(validateMaterialViewEvent(message, event), ErrInvalidEventDelivery) {
		t.Fatal("mismatched deterministic event ID was accepted")
	}
}

func TestIdentitySecurityRevocationTargetsExactSessionOrSubject(t *testing.T) {
	t.Parallel()

	userID := uuid.MustParse("00000000-0000-7000-8000-000000000021")
	sessionID := uuid.MustParse("00000000-0000-7000-8000-000000000022")
	revoker := &recordingConnectionRevoker{}
	if err := revokeIdentitySecurityConnections(
		context.Background(),
		revoker,
		securityNotificationDefinition{
			EventType:  "identity.session.revoked.v1",
			Revocation: securityConnectionRevocationSession,
		},
		identitySecurityEvent{TargetSessionID: sessionID.String()},
	); err != nil {
		t.Fatalf("revoke session: %v", err)
	}
	if len(revoker.sessions) != 1 || revoker.sessions[0] != sessionID ||
		len(revoker.subjects) != 0 {
		t.Fatalf("session revocations = %v, subjects = %v", revoker.sessions, revoker.subjects)
	}

	if err := revokeIdentitySecurityConnections(
		context.Background(),
		revoker,
		securityNotificationDefinition{
			EventType:  "identity.sessions.revoked_all.v1",
			Revocation: securityConnectionRevocationSubject,
		},
		identitySecurityEvent{UserID: userID.String()},
	); err != nil {
		t.Fatalf("revoke subject: %v", err)
	}
	if len(revoker.subjects) != 1 || revoker.subjects[0] != userID {
		t.Fatalf("subject revocations = %v", revoker.subjects)
	}
}

func TestIdentitySecurityRevocationFailureRemainsRetryable(t *testing.T) {
	t.Parallel()

	revoker := &recordingConnectionRevoker{err: errors.New("hub unavailable")}
	err := revokeIdentitySecurityConnections(
		context.Background(),
		revoker,
		securityNotificationDefinition{
			EventType:  "identity.password.changed.v1",
			Revocation: securityConnectionRevocationSubject,
		},
		identitySecurityEvent{UserID: uuid.NewString()},
	)
	var failure *commonoutbox.PublishError
	if !errors.As(err, &failure) || !failure.Retryable ||
		failure.Code != "realtime_revocation_failed" {
		t.Fatalf("revocation failure = %#v", err)
	}
}

type recordingConnectionRevoker struct {
	subjects []uuid.UUID
	sessions []uuid.UUID
	err      error
}

func (revoker *recordingConnectionRevoker) RevokeAllAuthenticated(context.Context) (int, error) {
	return 0, revoker.err
}

func (revoker *recordingConnectionRevoker) RevokeSubject(
	_ context.Context,
	userID uuid.UUID,
) (int, error) {
	revoker.subjects = append(revoker.subjects, userID)
	return 1, revoker.err
}

func (revoker *recordingConnectionRevoker) RevokeSession(
	_ context.Context,
	sessionID uuid.UUID,
) (int, error) {
	revoker.sessions = append(revoker.sessions, sessionID)
	return 1, revoker.err
}
