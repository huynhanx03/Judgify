package observability

import (
	"context"
	"database/sql"
	"errors"
	"sort"
	"strconv"
	"strings"
	"time"

	commonmetrics "github.com/huynhanx03/go-common/pkg/common/observability"

	contestcontract "github.com/huynhanx03/judgify/internal/contest/contract"
	submissioncontract "github.com/huynhanx03/judgify/internal/submission/contract"
)

var judgeMetricStates = []string{"ready", "leased", "completed", "cancelled", "dead"}
var outboxMetricStates = []string{"pending", "leased", "published", "dead"}
var securityEmailMetricStates = []string{"pending", "leased", "sent", "dead", "cancelled"}
var campaignMetricStates = []string{"scheduled", "running", "paused", "completed", "cancelled", "failed"}
var operationMetricStates = []string{"scheduled", "running", "paused", "completed", "cancelled", "failed"}
var contestProjectionClasses = []string{"standing", "content", "notification", "lifecycle"}

// RegisterDurableAPICollectors installs finite PostgreSQL snapshots for the
// durable queues and projections owned by the API role.
func (metrics *Metrics) RegisterDurableAPICollectors(
	pool *sql.DB,
	operationKinds []string,
) error {
	registrations := []func() error{
		func() error { return metrics.RegisterJudgeQueueCollector(pool) },
		func() error { return metrics.registerOutboxCollector(pool) },
		func() error { return metrics.registerSecurityEmailCollector(pool) },
		func() error { return metrics.registerCampaignCollector(pool) },
		func() error { return metrics.registerOperationCollector(pool, operationKinds) },
		func() error { return metrics.registerContestCollector(pool) },
	}
	for _, register := range registrations {
		if err := register(); err != nil {
			return err
		}
	}
	return nil
}

// RegisterJudgeQueueCollector is shared by API and worker metric registries;
// both observe the same durable source of truth through their existing pool.
func (metrics *Metrics) RegisterJudgeQueueCollector(pool *sql.DB) error {
	if pool == nil {
		return ErrInvalidMetrics
	}
	return metrics.RegisterCollector("judge-queue", CollectorFunc(func(ctx context.Context) error {
		rows, err := pool.QueryContext(ctx, `
			SELECT state, COUNT(*)
			FROM judge_jobs
			GROUP BY state`)
		if err != nil {
			return err
		}
		counts := make(map[string]float64, len(judgeMetricStates))
		for rows.Next() {
			var state string
			var count int64
			if err := rows.Scan(&state, &count); err != nil {
				_ = rows.Close()
				return err
			}
			switch state {
			case "ready", "retry_wait":
				counts["ready"] += float64(count)
			case "leased":
				counts["leased"] += float64(count)
			case "done":
				counts["completed"] += float64(count)
			case "cancelled", "dead":
				counts[state] += float64(count)
			}
		}
		if err := rows.Err(); err != nil {
			_ = rows.Close()
			return err
		}
		if err := rows.Close(); err != nil {
			return err
		}
		var oldest float64
		if err := pool.QueryRowContext(ctx, `
			SELECT COALESCE(MAX(GREATEST(EXTRACT(EPOCH FROM (CURRENT_TIMESTAMP - available_at)), 0)), 0)::double precision
			FROM judge_jobs
			WHERE state IN ('ready', 'retry_wait') AND available_at <= CURRENT_TIMESTAMP`).Scan(&oldest); err != nil {
			return err
		}
		var failures []error
		for _, state := range judgeMetricStates {
			failures = appendMetricError(failures, metrics.Set(ctx, MetricJudgeJobs, counts[state], commonmetrics.Labels{"state": state}))
		}
		failures = appendMetricError(failures, metrics.Set(ctx, MetricJudgeOldestReady, oldest, nil))
		return errors.Join(failures...)
	}))
}

// RegisterSolveRewardDeliveryCollectors installs the worker-owned durable
// boundary snapshots. Queries are fixed to the code-owned solve destination;
// no configured topic, group, event, or aggregate identity becomes a label.
func (metrics *Metrics) RegisterSolveRewardDeliveryCollectors(
	pool *sql.DB,
) error {
	if pool == nil {
		return ErrInvalidMetrics
	}
	registrations := []struct {
		name    string
		collect Collector
	}{
		{
			name: "solve-reward-outbox-state",
			collect: CollectorFunc(func(ctx context.Context) error {
				rows, err := pool.QueryContext(ctx, `
					SELECT state, COUNT(*)
					FROM outbox_events
					WHERE event_type = $1
					GROUP BY state`,
					submissioncontract.EventProblemSolveStatusChangedV1,
				)
				if err != nil {
					return err
				}
				counts := make(map[string]float64, len(outboxMetricStates))
				for rows.Next() {
					var state string
					var count int64
					if err := rows.Scan(&state, &count); err != nil {
						_ = rows.Close()
						return err
					}
					counts[state] = float64(count)
				}
				if err := rows.Err(); err != nil {
					_ = rows.Close()
					return err
				}
				if err := rows.Close(); err != nil {
					return err
				}
				var oldest float64
				if err := pool.QueryRowContext(ctx, `
					SELECT COALESCE(MAX(GREATEST(EXTRACT(EPOCH FROM (
						CURRENT_TIMESTAMP - occurred_at
					)), 0)), 0)::double precision
					FROM outbox_events
					WHERE event_type = $1
					  AND state = 'pending'
					  AND next_attempt_at <= CURRENT_TIMESTAMP`,
					submissioncontract.EventProblemSolveStatusChangedV1,
				).Scan(&oldest); err != nil {
					return err
				}
				var failures []error
				for _, state := range outboxMetricStates {
					failures = appendMetricError(failures, metrics.Set(
						ctx,
						MetricOutboxEvents,
						counts[state],
						commonmetrics.Labels{"state": state},
					))
				}
				failures = appendMetricError(failures, metrics.Set(
					ctx,
					MetricOutboxOldestPending,
					oldest,
					nil,
				))
				return errors.Join(failures...)
			}),
		},
		{
			name: "solve-reward-reconciliation-lag",
			collect: CollectorFunc(func(ctx context.Context) error {
				var lag float64
				if err := pool.QueryRowContext(ctx, `
					SELECT COALESCE(MAX(GREATEST(EXTRACT(EPOCH FROM (
						CURRENT_TIMESTAMP - transition.occurred_at
					)), 0)), 0)::double precision
					FROM user_problem_solve_transitions AS transition
					WHERE NOT EXISTS (
						SELECT 1
						FROM solve_reward_processed_transitions AS processed
						WHERE processed.solve_id = transition.solve_id
						  AND processed.aggregate_version = transition.to_version
					)`,
				).Scan(&lag); err != nil {
					return err
				}
				return metrics.Set(ctx, MetricOutboxReplayLag, lag, nil)
			}),
		},
	}
	for _, registration := range registrations {
		if err := metrics.RegisterCollector(
			registration.name,
			registration.collect,
		); err != nil {
			return err
		}
	}
	return nil
}

func (metrics *Metrics) registerOutboxCollector(pool *sql.DB) error {
	if pool == nil {
		return ErrInvalidMetrics
	}
	return metrics.RegisterCollector("outbox-state", CollectorFunc(func(ctx context.Context) error {
		counts, err := groupedCounts(ctx, pool, `SELECT state, COUNT(*) FROM outbox_events GROUP BY state`)
		if err != nil {
			return err
		}
		var oldest float64
		if err := pool.QueryRowContext(ctx, `
			SELECT COALESCE(MAX(GREATEST(EXTRACT(EPOCH FROM (CURRENT_TIMESTAMP - occurred_at)), 0)), 0)::double precision
			FROM outbox_events
			WHERE state = 'pending' AND next_attempt_at <= CURRENT_TIMESTAMP`).Scan(&oldest); err != nil {
			return err
		}
		var failures []error
		for _, state := range outboxMetricStates {
			failures = appendMetricError(failures, metrics.Set(ctx, MetricOutboxEvents, counts[state], commonmetrics.Labels{"state": state}))
		}
		failures = appendMetricError(failures, metrics.Set(ctx, MetricOutboxOldestPending, oldest, nil))
		return errors.Join(failures...)
	}))
}

func (metrics *Metrics) registerSecurityEmailCollector(pool *sql.DB) error {
	if pool == nil {
		return ErrInvalidMetrics
	}
	return metrics.RegisterCollector("security-email-state", CollectorFunc(func(ctx context.Context) error {
		stored, err := groupedCounts(ctx, pool, `SELECT status, COUNT(*) FROM security_email_deliveries GROUP BY status`)
		if err != nil {
			return err
		}
		counts := map[string]float64{
			"pending":   stored["ready"],
			"leased":    stored["leased"],
			"sent":      stored["sent"],
			"dead":      stored["dead"],
			"cancelled": stored["cancelled"],
		}
		var oldest float64
		if err := pool.QueryRowContext(ctx, `
			SELECT COALESCE(MAX(GREATEST(EXTRACT(EPOCH FROM (CURRENT_TIMESTAMP - created_at)), 0)), 0)::double precision
			FROM security_email_deliveries
			WHERE status = 'ready' AND next_attempt_at <= CURRENT_TIMESTAMP`).Scan(&oldest); err != nil {
			return err
		}
		var failures []error
		for _, state := range securityEmailMetricStates {
			failures = appendMetricError(failures, metrics.Set(ctx, MetricSecurityEmailJobs, counts[state], commonmetrics.Labels{"state": state}))
		}
		failures = appendMetricError(failures, metrics.Set(ctx, MetricSecurityEmailOldestPending, oldest, nil))
		return errors.Join(failures...)
	}))
}

func (metrics *Metrics) registerCampaignCollector(pool *sql.DB) error {
	if pool == nil {
		return ErrInvalidMetrics
	}
	return metrics.RegisterCollector("notification-campaign-state", CollectorFunc(func(ctx context.Context) error {
		counts, err := groupedCounts(ctx, pool, `
			SELECT CASE
				WHEN operation.status = 'paused' THEN 'paused'
				WHEN campaign.status = 'sending' THEN 'running'
				ELSE campaign.status
			END AS metric_state, COUNT(*)
			FROM notification_campaigns AS campaign
			LEFT JOIN operations AS operation ON operation.id = campaign.operation_id
			WHERE campaign.status <> 'draft'
			GROUP BY metric_state`)
		if err != nil {
			return err
		}
		var failures []error
		for _, state := range campaignMetricStates {
			failures = appendMetricError(failures, metrics.Set(ctx, MetricCampaignOperations, counts[state], commonmetrics.Labels{"state": state}))
		}
		return errors.Join(failures...)
	}))
}

func (metrics *Metrics) registerOperationCollector(pool *sql.DB, operationKinds []string) error {
	if pool == nil || len(operationKinds) == 0 || len(operationKinds) > 31 {
		return ErrInvalidMetrics
	}
	allowed := make(map[string]struct{}, len(operationKinds))
	frozenKinds := make([]string, 0, len(operationKinds))
	for _, kind := range operationKinds {
		if _, duplicate := allowed[kind]; duplicate || kind == "other" || !safeOperationKind(kind) {
			return ErrInvalidMetrics
		}
		allowed[kind] = struct{}{}
		frozenKinds = append(frozenKinds, kind)
	}
	sort.Strings(frozenKinds)
	placeholders := make([]string, len(frozenKinds))
	arguments := make([]any, len(frozenKinds))
	for index, kind := range frozenKinds {
		placeholders[index] = "$" + strconv.Itoa(index+1)
		arguments[index] = kind
	}
	// Fold both dimensions inside PostgreSQL. The result is bounded by
	// (registered kinds + other) * six states even if legacy rows contain
	// arbitrary kind or status values.
	query := `
		SELECT
			CASE WHEN kind IN (` + strings.Join(placeholders, ", ") + `) THEN kind ELSE 'other' END AS metric_kind,
			CASE WHEN status = 'pending' THEN 'scheduled' ELSE status END AS metric_state,
			COUNT(*)
		FROM operations
		WHERE status IN ('pending', 'running', 'paused', 'completed', 'cancelled', 'failed')
		GROUP BY 1, 2`
	frozenKinds = append(frozenKinds, "other")
	return metrics.RegisterCollector("operation-state", CollectorFunc(func(ctx context.Context) error {
		rows, err := pool.QueryContext(ctx, query, arguments...)
		if err != nil {
			return err
		}
		counts := make(map[string]map[string]float64, len(frozenKinds))
		for rows.Next() {
			var kind, state string
			var count int64
			if err := rows.Scan(&kind, &state, &count); err != nil {
				_ = rows.Close()
				return err
			}
			if _, known := allowed[kind]; !known {
				kind = "other"
			}
			if state == "pending" {
				state = "scheduled"
			}
			if !contains(operationMetricStates, state) {
				continue
			}
			if counts[kind] == nil {
				counts[kind] = make(map[string]float64, len(operationMetricStates))
			}
			counts[kind][state] += float64(count)
		}
		if err := rows.Err(); err != nil {
			_ = rows.Close()
			return err
		}
		if err := rows.Close(); err != nil {
			return err
		}
		var failures []error
		for _, kind := range frozenKinds {
			for _, state := range operationMetricStates {
				failures = appendMetricError(failures, metrics.Set(ctx, MetricOperations, counts[kind][state], commonmetrics.Labels{
					"kind": kind, "state": state,
				}))
			}
		}
		return errors.Join(failures...)
	}))
}

func safeOperationKind(kind string) bool {
	if len(kind) == 0 || len(kind) > 128 || kind[0] < 'a' || kind[0] > 'z' {
		return false
	}
	for index := 1; index < len(kind); index++ {
		character := kind[index]
		if (character >= 'a' && character <= 'z') ||
			(character >= '0' && character <= '9') ||
			character == '.' || character == '_' || character == '-' {
			continue
		}
		return false
	}
	return true
}

func (metrics *Metrics) registerContestCollector(pool *sql.DB) error {
	if pool == nil {
		return ErrInvalidMetrics
	}
	return metrics.RegisterCollector("contest-projection-state", CollectorFunc(func(ctx context.Context) error {
		lag := make(map[string]float64, len(contestProjectionClasses))
		rows, err := pool.QueryContext(ctx, `
			SELECT CASE
				WHEN event_type = $1 THEN 'standing'
				WHEN event_type IN (
					$2, $3, $4, $5, $6, $7
				) THEN 'content'
				WHEN event_type IN (
					$8, $9, $10, $11
				) THEN 'notification'
				WHEN event_type = $12 THEN 'lifecycle'
			END AS projection,
			MAX(GREATEST(EXTRACT(EPOCH FROM (CURRENT_TIMESTAMP - occurred_at)), 0))::double precision
			FROM outbox_events
			WHERE state IN ('pending', 'leased') AND event_type IN (
				$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12
			)
			GROUP BY projection`,
			submissioncontract.EventSubmissionJudgedV1,
			contestcontract.EventAnnouncementPublishedV1,
			contestcontract.EventAnnouncementWithdrawnV1,
			contestcontract.EventClarificationSubmittedV1,
			contestcontract.EventClarificationAnsweredV1,
			contestcontract.EventClarificationClosedV1,
			contestcontract.EventClarificationWithdrawnV1,
			contestcontract.EventRegistrationChangedV1,
			contestcontract.EventRatingCalculatedV1,
			contestcontract.EventRatingSkippedV1,
			contestcontract.EventRatingProjectionActivatedV1,
			contestcontract.EventLifecycleChangedV1,
		)
		if err != nil {
			return err
		}
		for rows.Next() {
			var projection string
			var seconds float64
			if err := rows.Scan(&projection, &seconds); err != nil {
				_ = rows.Close()
				return err
			}
			if contains(contestProjectionClasses, projection) {
				lag[projection] = seconds
			}
		}
		if err := rows.Err(); err != nil {
			_ = rows.Close()
			return err
		}
		if err := rows.Close(); err != nil {
			return err
		}
		var lifecycleLag float64
		if err := pool.QueryRowContext(ctx, `
			SELECT COALESCE(MAX(GREATEST(EXTRACT(EPOCH FROM (
				CURRENT_TIMESTAMP - CASE WHEN status = 'upcoming' THEN start_time ELSE end_time END
			)), 0)), 0)::double precision
			FROM contests
			WHERE deleted_at IS NULL AND (
				(status = 'upcoming' AND start_time <= CURRENT_TIMESTAMP) OR
				(status = 'running' AND end_time <= CURRENT_TIMESTAMP)
			)`).Scan(&lifecycleLag); err != nil {
			return err
		}
		if lifecycleLag > lag["lifecycle"] {
			lag["lifecycle"] = lifecycleLag
		}
		var ratingLag float64
		if err := pool.QueryRowContext(ctx, `
			SELECT COALESCE(MAX(GREATEST(EXTRACT(EPOCH FROM (CURRENT_TIMESTAMP - COALESCE(ended_at, end_time))), 0)), 0)::double precision
			FROM contests
			WHERE deleted_at IS NULL AND status = 'ended' AND rated = true AND rating_completed_at IS NULL`).Scan(&ratingLag); err != nil {
			return err
		}
		var failures []error
		for _, projection := range contestProjectionClasses {
			failures = appendMetricError(failures, metrics.Set(ctx, MetricContestProjectionLag, lag[projection], commonmetrics.Labels{"projection": projection}))
		}
		failures = appendMetricError(failures, metrics.Set(ctx, MetricContestRatingLag, ratingLag, nil))
		return errors.Join(failures...)
	}))
}

func groupedCounts(ctx context.Context, pool *sql.DB, query string) (map[string]float64, error) {
	rows, err := pool.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	counts := make(map[string]float64)
	for rows.Next() {
		var state string
		var count int64
		if err := rows.Scan(&state, &count); err != nil {
			return nil, err
		}
		counts[state] = float64(count)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return counts, rows.Close()
}

func appendMetricError(failures []error, err error) []error {
	if err != nil {
		return append(failures, err)
	}
	return failures
}

func contains(values []string, value string) bool {
	for _, candidate := range values {
		if candidate == value {
			return true
		}
	}
	return false
}

func ageSeconds(now time.Time, since time.Time) float64 {
	if since.IsZero() || now.Before(since) {
		return 0
	}
	return now.Sub(since).Seconds()
}
