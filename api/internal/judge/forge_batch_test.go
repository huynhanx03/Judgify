package judge

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/huynhanx03/go-common/pkg/mq/forge"
)

func TestConsumeForgeBatchCommitsOnlyAfterCompletion(t *testing.T) {
	broker, producer, consumer := newForgeBatchFixture(t)
	defer func() { _ = broker.Close() }()
	if err := producer.SendContext(context.Background(), nil, []byte("job"), nil, forge.AckFsync); err != nil {
		t.Fatalf("produce: %v", err)
	}

	completed := make(chan error, 1)
	result := make(chan error, 1)
	go func() {
		_, err := consumeForgeBatch(
			context.Background(),
			consumer,
			1,
			func(value []byte, target any) error { *target.(*string) = string(value); return nil },
			func(context.Context, string) (<-chan error, error) { return completed, nil },
			nil,
		)
		result <- err
	}()

	select {
	case err := <-result:
		t.Fatalf("batch returned before side effects completed: %v", err)
	case <-time.After(20 * time.Millisecond):
	}
	if got := consumer.CommittedOffset(); got != 0 {
		t.Fatalf("committed offset while task is running = %d, want 0", got)
	}
	completed <- nil
	close(completed)
	if err := <-result; err != nil {
		t.Fatalf("consume batch: %v", err)
	}
	if got := consumer.CommittedOffset(); got != 1 {
		t.Fatalf("committed offset after completion = %d, want 1", got)
	}
}

func TestConsumeForgeBatchRollsBackCanceledAdmission(t *testing.T) {
	broker, producer, consumer := newForgeBatchFixture(t)
	defer func() { _ = broker.Close() }()
	if err := producer.SendContext(context.Background(), nil, []byte("job"), nil, forge.AckFsync); err != nil {
		t.Fatalf("produce: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	_, err := consumeForgeBatch(
		ctx,
		consumer,
		1,
		func(value []byte, target any) error { *target.(*string) = string(value); return nil },
		func(context.Context, string) (<-chan error, error) {
			// Cancellation here occurs after Fetch advanced the in-memory
			// cursor but before the record was admitted.
			cancel()
			return nil, context.Canceled
		},
		nil,
	)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("consume canceled batch = %v, want context.Canceled", err)
	}
	if got := consumer.CommittedOffset(); got != 0 {
		t.Fatalf("committed offset after cancellation = %d, want 0", got)
	}
	if got := consumer.Offset(); got != 0 {
		t.Fatalf("fetch cursor after cancellation = %d, want rollback to 0", got)
	}
}

func TestConsumeForgeBatchRollsBackFailedApplicationResult(t *testing.T) {
	broker, producer, consumer := newForgeBatchFixture(t)
	defer func() { _ = broker.Close() }()
	if err := producer.SendContext(context.Background(), nil, []byte("job"), nil, forge.AckFsync); err != nil {
		t.Fatalf("produce: %v", err)
	}

	processingErr := errors.New("lease lost")
	completed := make(chan error, 1)
	completed <- processingErr
	close(completed)
	_, err := consumeForgeBatch(
		context.Background(),
		consumer,
		1,
		func(value []byte, target any) error { *target.(*string) = string(value); return nil },
		func(context.Context, string) (<-chan error, error) { return completed, nil },
		nil,
	)
	if !errors.Is(err, processingErr) {
		t.Fatalf("consume failed task = %v, want processing error", err)
	}
	if got := consumer.CommittedOffset(); got != 0 {
		t.Fatalf("committed offset after failed task = %d, want 0", got)
	}
	if got := consumer.Offset(); got != 0 {
		t.Fatalf("fetch cursor after failed task = %d, want rollback to 0", got)
	}
}

func TestConsumeForgeBatchDurablyQuarantinesMalformedDeliveryBeforeCommit(t *testing.T) {
	broker, err := forge.NewBroker(t.TempDir())
	if err != nil {
		t.Fatalf("open Forge: %v", err)
	}
	defer func() { _ = broker.Close() }()
	producer, err := broker.NewProducer("jobs")
	if err != nil {
		t.Fatalf("open producer: %v", err)
	}
	consumer, _, err := broker.NewDLQConsumer("workers", "jobs")
	if err != nil {
		t.Fatalf("open DLQ consumer: %v", err)
	}
	if err := producer.SendContext(context.Background(), nil, []byte("malformed"), nil, forge.AckFsync); err != nil {
		t.Fatalf("produce: %v", err)
	}

	dispatched := false
	consumed, err := consumeForgeBatch(
		context.Background(),
		consumer,
		1,
		func([]byte, any) error { return errors.New("decode") },
		func(context.Context, string) (<-chan error, error) {
			dispatched = true
			return nil, nil
		},
		nil,
	)
	if err != nil {
		t.Fatalf("consume malformed batch: %v", err)
	}
	if consumed != 1 || dispatched {
		t.Fatalf("consume = %d, dispatched=%v; want quarantined delivery", consumed, dispatched)
	}
	if got := consumer.CommittedOffset(); got != 1 {
		t.Fatalf("committed offset = %d, want 1 after durable DLQ write", got)
	}

	dlq, err := broker.NewConsumer("dlq-audit", "jobs.dlq")
	if err != nil {
		t.Fatalf("open DLQ reader: %v", err)
	}
	records, err := dlq.Fetch(context.Background(), 1)
	if err != nil {
		t.Fatalf("fetch DLQ: %v", err)
	}
	if len(records) != 1 || string(records[0].Value) != "malformed" {
		t.Fatalf("DLQ records = %+v, want malformed delivery", records)
	}
}

func TestConsumeForgeBatchDurablyQuarantinesPermanentApplicationFailure(t *testing.T) {
	broker, err := forge.NewBroker(t.TempDir())
	if err != nil {
		t.Fatalf("open Forge: %v", err)
	}
	defer func() { _ = broker.Close() }()
	producer, err := broker.NewProducer("jobs")
	if err != nil {
		t.Fatalf("open producer: %v", err)
	}
	consumer, _, err := broker.NewDLQConsumer("workers", "jobs")
	if err != nil {
		t.Fatalf("open DLQ consumer: %v", err)
	}
	if err := producer.SendContext(context.Background(), nil, []byte("valid-json"), nil, forge.AckFsync); err != nil {
		t.Fatalf("produce: %v", err)
	}

	completed := make(chan error, 1)
	completed <- permanentForgeTestError{}
	close(completed)
	consumed, err := consumeForgeBatch(
		context.Background(),
		consumer,
		1,
		func(value []byte, target any) error { *target.(*string) = string(value); return nil },
		func(context.Context, string) (<-chan error, error) { return completed, nil },
		nil,
	)
	if err != nil || consumed != 1 || consumer.CommittedOffset() != 1 {
		t.Fatalf("consume permanent failure = (%d, %v, offset=%d)", consumed, err, consumer.CommittedOffset())
	}
	dlq, err := broker.NewConsumer("dlq-audit", "jobs.dlq")
	if err != nil {
		t.Fatalf("open DLQ reader: %v", err)
	}
	deliveries, err := dlq.Fetch(context.Background(), 1)
	if err != nil || len(deliveries) != 1 || string(deliveries[0].Value) != "valid-json" {
		t.Fatalf("DLQ deliveries = %+v, %v", deliveries, err)
	}
}

type permanentForgeTestError struct{}

func (permanentForgeTestError) Error() string   { return "permanent" }
func (permanentForgeTestError) Permanent() bool { return true }

func newForgeBatchFixture(t *testing.T) (*forge.Broker, *forge.Producer, *forge.Consumer) {
	t.Helper()
	broker, err := forge.NewBroker(t.TempDir())
	if err != nil {
		t.Fatalf("open Forge: %v", err)
	}
	producer, err := broker.NewProducer("jobs")
	if err != nil {
		_ = broker.Close()
		t.Fatalf("open producer: %v", err)
	}
	consumer, err := broker.NewConsumer("workers", "jobs")
	if err != nil {
		_ = broker.Close()
		t.Fatalf("open consumer: %v", err)
	}
	return broker, producer, consumer
}
