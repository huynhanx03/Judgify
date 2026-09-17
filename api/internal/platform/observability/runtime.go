package observability

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"reflect"
	"time"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	"go.uber.org/zap"

	commonmetrics "github.com/huynhanx03/go-common/pkg/common/observability"
	commonent "github.com/huynhanx03/go-common/pkg/database/ent"
	commonlogger "github.com/huynhanx03/go-common/pkg/logger"
)

const (
	maxCollectors    = 32
	collectorTimeout = 2 * time.Second
)

// Collector refreshes durable or process-local gauges immediately before a
// scrape. Implementations must be finite, honor ctx, and never start a hidden
// goroutine. Counters should be emitted at transition boundaries instead.
type Collector interface {
	Collect(context.Context) error
}

type CollectorFunc func(context.Context) error

func (collect CollectorFunc) Collect(ctx context.Context) error {
	if collect == nil {
		return ErrInvalidMetrics
	}
	return collect(ctx)
}

type registeredCollector struct {
	name      string
	collector Collector
}

// RegisterCollector installs one bounded process-local collector. Names are
// code-owned diagnostics only and are never exported as metric labels.
func (metrics *Metrics) RegisterCollector(name string, collector Collector) error {
	if metrics == nil || isNilInterface(collector) || !safeCollectorName(name) {
		return ErrInvalidMetrics
	}
	metrics.collectorMu.Lock()
	defer metrics.collectorMu.Unlock()
	if _, exists := metrics.collectorNames[name]; exists {
		return fmt.Errorf("%w: %s", ErrCollectorExists, name)
	}
	if len(metrics.collectors) >= maxCollectors {
		return ErrCollectorLimit
	}
	metrics.collectorNames[name] = struct{}{}
	metrics.collectors = append(metrics.collectors, registeredCollector{
		name:      name,
		collector: collector,
	})
	return nil
}

func isNilInterface(value any) bool {
	if value == nil {
		return true
	}
	reflected := reflect.ValueOf(value)
	switch reflected.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map,
		reflect.Pointer, reflect.Slice:
		return reflected.IsNil()
	default:
		return false
	}
}

// RegisterDatabasePool publishes the standard library's bounded pool snapshot
// from the already-owned pool. It performs no database statement and never
// takes ownership of Close.
func (metrics *Metrics) RegisterDatabasePool(pool *sql.DB) error {
	if pool == nil {
		return ErrInvalidMetrics
	}
	return metrics.RegisterCollector("database-pool", CollectorFunc(func(ctx context.Context) error {
		stats := pool.Stats()
		values := []struct {
			state string
			value float64
		}{
			{state: "open", value: float64(stats.OpenConnections)},
			{state: "in_use", value: float64(stats.InUse)},
			{state: "idle", value: float64(stats.Idle)},
			{state: "wait_count", value: float64(stats.WaitCount)},
		}
		var failures []error
		for _, value := range values {
			if err := metrics.Set(ctx, MetricDatabasePoolConnections, value.value, commonmetrics.Labels{
				"state": value.state,
			}); err != nil {
				failures = append(failures, err)
			}
		}
		return errors.Join(failures...)
	}))
}

// SQLPool resolves an Ent SQL driver without taking pool ownership.
func SQLPool(driver dialect.Driver) (*sql.DB, error) {
	unwrapped := commonent.UnwrapLogging(driver)
	sqlDriver, ok := unwrapped.(*entsql.Driver)
	if !ok || sqlDriver == nil || sqlDriver.DB() == nil {
		return nil, ErrInvalidMetrics
	}
	return sqlDriver.DB(), nil
}

func (metrics *Metrics) collectBeforeScrape(parent context.Context) {
	if metrics == nil || parent == nil || !metrics.scrapeMu.TryLock() {
		return
	}
	defer metrics.scrapeMu.Unlock()
	ctx, cancel := context.WithTimeout(parent, collectorTimeout)
	defer cancel()

	metrics.collectorMu.RLock()
	collectors := append([]registeredCollector(nil), metrics.collectors...)
	metrics.collectorMu.RUnlock()
	for _, registered := range collectors {
		if ctx.Err() != nil {
			return
		}
		collectorErr := runCollector(ctx, registered.collector)
		switch {
		case errors.Is(ctx.Err(), context.DeadlineExceeded):
			commonlogger.FromContext(context.WithoutCancel(ctx)).Warn(
				"metrics collector timed out",
				zap.String("collector", registered.name),
			)
			return
		case ctx.Err() != nil:
			// A disconnected scraper is expected request cancellation, not a
			// dependency incident.
			return
		case collectorErr != nil:
			commonlogger.FromContext(ctx).Warn(
				"metrics collector failed",
				zap.String("collector", registered.name),
				zap.String("error_type", fmt.Sprintf("%T", collectorErr)),
			)
		}
	}
}

func runCollector(ctx context.Context, collector Collector) (err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			err = fmt.Errorf("%w: collector panic", ErrInvalidMetrics)
		}
	}()
	return collector.Collect(ctx)
}

func safeCollectorName(name string) bool {
	if len(name) == 0 || len(name) > 64 {
		return false
	}
	for index := range len(name) {
		char := name[index]
		if (char >= 'a' && char <= 'z') ||
			(char >= '0' && char <= '9') ||
			char == '-' || char == '_' {
			continue
		}
		return false
	}
	return true
}
