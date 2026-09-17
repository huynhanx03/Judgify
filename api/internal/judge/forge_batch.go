package judge

import (
	"context"
	"errors"
	"fmt"

	"github.com/huynhanx03/go-common/pkg/mq/forge"
)

// forgeDispatch admits one decoded delivery and returns exactly one terminal
// application result. A non-nil result keeps the Forge offset uncommitted.
type forgeDispatch[T any] func(context.Context, T) (<-chan error, error)

type forgeApplicationCompletion struct {
	delivery forge.Delivery
	done     <-chan error
}

type permanentForgeApplicationError interface {
	error
	Permanent() bool
}

// consumeForgeBatch provides the worker's at-least-once boundary. Fetch only
// advances Forge's in-memory cursor; the durable offset is committed after
// every admitted task in the batch completes. Cancellation or failed
// admission rolls the cursor back, so the whole uncommitted tail is delivered
// again after restart.
func consumeForgeBatch[T any](
	ctx context.Context,
	consumer *forge.Consumer,
	limit int,
	decode func([]byte, any) error,
	dispatch forgeDispatch[T],
	onDecodeError func(forge.Delivery, error),
) (int, error) {
	if ctx == nil || consumer == nil || limit <= 0 || decode == nil || dispatch == nil {
		return 0, fmt.Errorf("consume Forge batch: invalid dependencies")
	}
	deliveries, err := consumer.FetchWait(
		ctx,
		limit,
		forge.DefaultFetchWaitInterval,
	)
	if err != nil || len(deliveries) == 0 {
		return 0, err
	}

	completions := make([]forgeApplicationCompletion, 0, len(deliveries))
	quarantine := make([]forge.Delivery, 0)
	for _, delivery := range deliveries {
		var payload T
		if err := decode(delivery.Value, &payload); err != nil {
			if onDecodeError != nil {
				onDecodeError(delivery, err)
			}
			quarantine = append(quarantine, delivery)
			continue
		}
		completed, err := dispatch(ctx, payload)
		if err != nil {
			return 0, rollbackForgeBatch(consumer, err)
		}
		if completed == nil {
			return 0, rollbackForgeBatch(consumer, errors.New("forge dispatch returned nil completion"))
		}
		completions = append(completions, forgeApplicationCompletion{
			delivery: delivery,
			done:     completed,
		})
	}

	var transient error
	for _, completion := range completions {
		select {
		case result, ok := <-completion.done:
			if !ok {
				transient = errors.Join(
					transient,
					errors.New("forge task closed without a result"),
				)
				continue
			}
			if result != nil {
				if isPermanentForgeApplicationError(result) {
					quarantine = append(quarantine, completion.delivery)
					continue
				}
				transient = errors.Join(transient, result)
			}
		case <-ctx.Done():
			return 0, rollbackForgeBatch(consumer, ctx.Err())
		}
	}
	if transient != nil {
		// Do not DLQ permanent records from a batch whose retryable tail must be
		// replayed. This avoids producing duplicate DLQ records on every retry.
		return 0, rollbackForgeBatch(consumer, transient)
	}
	if err := ctx.Err(); err != nil {
		return 0, rollbackForgeBatch(consumer, err)
	}
	for _, delivery := range quarantine {
		// The source offset advances only after every poison record is durably
		// fsynced to the configured DLQ.
		if err := consumer.NackDeliveryContext(ctx, delivery); err != nil {
			return 0, rollbackForgeBatch(
				consumer,
				fmt.Errorf("quarantine Forge delivery: %w", err),
			)
		}
	}
	nextOffset := deliveries[len(deliveries)-1].Offset + 1
	if err := consumer.CommitOffset(ctx, nextOffset); err != nil {
		return 0, rollbackForgeBatch(consumer, err)
	}
	return len(deliveries), nil
}

func isPermanentForgeApplicationError(err error) bool {
	var classified permanentForgeApplicationError
	return errors.As(err, &classified) && classified.Permanent()
}

func rollbackForgeBatch(consumer *forge.Consumer, cause error) error {
	return errors.Join(cause, consumer.Rollback())
}
