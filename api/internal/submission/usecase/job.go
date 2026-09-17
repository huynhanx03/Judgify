package usecase

import (
	"context"
	"crypto/rand"
	"errors"
	"io"
	"math/big"
	"strings"
	"sync"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/huynhanx03/go-common/pkg/logger"

	"github.com/huynhanx03/judgify/internal/ports"
	entity "github.com/huynhanx03/judgify/internal/submission/domain"
)

var (
	ErrJobAdmissionClosed = errors.New("submission jobs: local admission closed")
	ErrJobServiceInvalid  = errors.New("submission jobs: invalid service operation")
)

type DurableJobRepository interface {
	Claim(context.Context, ports.JobClaimRequest) ([]ports.JobLease, error)
	HeartbeatWorkerBoot(context.Context, ports.WorkerBootHeartbeat) error
	Heartbeat(
		context.Context,
		ports.LeaseHeartbeat,
	) (acceptedLeaseUntil time.Time, err error)
	ReapExpired(context.Context, ports.ReapRequest) (ports.ReapResult, error)
	AcknowledgeCancellation(context.Context, ports.JobLease) error
	TransitionPhase(
		context.Context,
		ports.JobLease,
		int64,
		entity.Phase,
	) (entity.SubmissionState, error)
	RecordRetry(context.Context, ports.JobLease, ports.InfrastructureFailure) error
}

// LeaseSink is a bounded, process-local accelerator. TryEnqueueBatch is
// atomic: it accepts every lease or returns overload without blocking.
type LeaseSink interface {
	TryEnqueueBatch([]ports.JobLease) error
	Remaining() int
	Depth() int
	OldestAge(time.Time) time.Duration
}

type CapacitySnapshot struct {
	FreeSandboxes      int
	DiskAvailableBytes int64
}

type CapacityProbe interface {
	Snapshot(context.Context) (CapacitySnapshot, error)
}

type LeaseMetricOutcome string

const (
	LeaseMetricSuccess   LeaseMetricOutcome = "success"
	LeaseMetricRetry     LeaseMetricOutcome = "retry"
	LeaseMetricRejected  LeaseMetricOutcome = "rejected"
	LeaseMetricDead      LeaseMetricOutcome = "dead"
	LeaseMetricCancelled LeaseMetricOutcome = "cancelled"
	LeaseMetricError     LeaseMetricOutcome = "error"
)

// JobMetricsObserver is an optional transition observer. It receives only a
// closed outcome and aggregate count; lease/job/submission identities and
// failure text never cross this boundary.
type JobMetricsObserver interface {
	RecordLeaseEvent(context.Context, LeaseMetricOutcome, int)
}

type JobServiceConfig struct {
	WorkerID            uuid.UUID
	WorkerBootID        uuid.UUID
	LeaseOwner          string
	LeaseDuration       time.Duration
	HeartbeatFreshness  time.Duration
	GlobalRunningLimit  int
	PerUserRunningLimit int
	MinimumDiskBytes    int64
	MaximumLocalDepth   int
	MaximumOldestAge    time.Duration
	ReapLimit           int
	RetryInitial        time.Duration
	RetryMaximum        time.Duration
	Observer            JobMetricsObserver
}

type JobService struct {
	repository DurableJobRepository
	sink       LeaseSink
	capacity   CapacityProbe
	config     JobServiceConfig
	now        func() time.Time
	random     io.Reader
	claimMu    sync.Mutex
}

func NewJobService(
	repository DurableJobRepository,
	sink LeaseSink,
	capacity CapacityProbe,
	config JobServiceConfig,
) (*JobService, error) {
	if repository == nil || sink == nil || capacity == nil ||
		config.WorkerID == uuid.Nil || config.WorkerBootID == uuid.Nil ||
		config.LeaseOwner == "" || config.LeaseDuration <= 0 ||
		config.HeartbeatFreshness <= 0 || config.GlobalRunningLimit <= 0 ||
		config.PerUserRunningLimit <= 0 || config.MinimumDiskBytes < 0 ||
		config.MaximumLocalDepth <= 0 || config.MaximumOldestAge <= 0 ||
		config.ReapLimit <= 0 || config.RetryInitial <= 0 ||
		config.RetryMaximum < config.RetryInitial {
		return nil, ErrJobServiceInvalid
	}
	return &JobService{
		repository: repository, sink: sink, capacity: capacity,
		config: config, now: func() time.Time { return time.Now().UTC() },
		random: rand.Reader,
	}, nil
}

// ClaimAndSchedule applies local disk/depth/age pressure before asking
// PostgreSQL for work. PostgreSQL remains authoritative for compatibility,
// fairness bounds, and capacity.
func (service *JobService) ClaimAndSchedule(ctx context.Context) (int, error) {
	if service == nil || ctx == nil {
		return 0, ErrJobServiceInvalid
	}
	if !service.claimMu.TryLock() {
		return 0, ErrJobAdmissionClosed
	}
	defer service.claimMu.Unlock()
	now := service.now()
	localDepth := service.sink.Depth()
	if localDepth >= service.config.MaximumLocalDepth ||
		service.sink.OldestAge(now) >= service.config.MaximumOldestAge {
		return 0, ErrJobAdmissionClosed
	}
	snapshot, err := service.capacity.Snapshot(ctx)
	if err != nil {
		return 0, errors.Join(ErrJobAdmissionClosed, err)
	}
	capacity := minServiceInt(snapshot.FreeSandboxes, service.sink.Remaining())
	capacity = minServiceInt(capacity, service.config.MaximumLocalDepth-localDepth)
	if snapshot.DiskAvailableBytes < service.config.MinimumDiskBytes || capacity <= 0 {
		return 0, ErrJobAdmissionClosed
	}
	leases, err := service.repository.Claim(ctx, ports.JobClaimRequest{
		WorkerID: service.config.WorkerID, WorkerBootID: service.config.WorkerBootID,
		LeaseOwner: service.config.LeaseOwner, LeaseDuration: service.config.LeaseDuration,
		HeartbeatFreshness: service.config.HeartbeatFreshness, Capacity: capacity,
		GlobalRunningLimit:  service.config.GlobalRunningLimit,
		PerUserRunningLimit: service.config.PerUserRunningLimit,
	})
	if err != nil {
		service.recordLeaseEvent(ctx, LeaseMetricError, 1)
		return 0, err
	}
	if len(leases) == 0 {
		return 0, nil
	}
	if err := service.sink.TryEnqueueBatch(leases); err != nil {
		// The leases stay durable and safely return through expiry/reaping. The
		// local accelerator never blocks or invents an ownership transition.
		wipeLeaseTokens(leases)
		service.recordLeaseEvent(ctx, LeaseMetricRejected, len(leases))
		return 0, errors.Join(ErrJobAdmissionClosed, err)
	}
	service.recordLeaseEvent(ctx, LeaseMetricSuccess, len(leases))
	return len(leases), nil
}

func (service *JobService) HeartbeatWorkerBoot(ctx context.Context) error {
	if service == nil || ctx == nil {
		return ErrJobServiceInvalid
	}
	return service.repository.HeartbeatWorkerBoot(ctx, ports.WorkerBootHeartbeat{
		WorkerID:     service.config.WorkerID,
		WorkerBootID: service.config.WorkerBootID,
	})
}

func (service *JobService) Heartbeat(
	ctx context.Context,
	lease ports.JobLease,
) (time.Time, error) {
	if service == nil || ctx == nil || len(lease.LeaseToken) == 0 {
		return time.Time{}, ErrJobServiceInvalid
	}
	return service.repository.Heartbeat(ctx, ports.LeaseHeartbeat{
		JobID: lease.Job.ID, AttemptID: lease.Attempt.ID,
		WorkerID: service.config.WorkerID, WorkerBootID: service.config.WorkerBootID,
		LeaseOwner: service.config.LeaseOwner, LeaseToken: lease.LeaseToken,
		ExtendTo: service.now().Add(service.config.LeaseDuration),
	})
}

func (service *JobService) ReapExpired(ctx context.Context) (ports.ReapResult, error) {
	if service == nil || ctx == nil {
		return ports.ReapResult{}, ErrJobServiceInvalid
	}
	result, err := service.repository.ReapExpired(ctx, ports.ReapRequest{
		Limit:        service.config.ReapLimit,
		RetryInitial: service.config.RetryInitial,
		RetryMaximum: service.config.RetryMaximum,
	})
	if err != nil {
		service.recordLeaseEvent(ctx, LeaseMetricError, 1)
		return ports.ReapResult{}, err
	}
	service.recordLeaseEvent(ctx, LeaseMetricRetry, result.Promoted+result.Retried)
	service.recordLeaseEvent(ctx, LeaseMetricDead, result.Dead)
	service.recordLeaseEvent(ctx, LeaseMetricCancelled, result.Cancelled)
	return result, nil
}

// AcknowledgeCancellation durably closes a lease which has already been
// cancelled at the submission aggregate. This is intentionally distinct from
// retry: cancellation is a requested outcome, never an infrastructure fault.
func (service *JobService) AcknowledgeCancellation(
	ctx context.Context,
	lease ports.JobLease,
) error {
	if service == nil || ctx == nil || len(lease.LeaseToken) == 0 ||
		lease.Job.ID == uuid.Nil || lease.Attempt.ID == uuid.Nil {
		return ErrJobServiceInvalid
	}
	log := logger.FromContext(ctx)
	log.Debug("acknowledging cancelled judge lease")
	if err := service.repository.AcknowledgeCancellation(ctx, lease); err != nil {
		service.recordLeaseEvent(ctx, LeaseMetricError, 1)
		log.Error("cancelled judge lease acknowledgement failed", zap.Error(err))
		return err
	}
	service.recordLeaseEvent(ctx, LeaseMetricCancelled, 1)
	log.Info("cancelled judge lease acknowledged")
	return nil
}

// TransitionPhase advances both the durable attempt and submission aggregate
// behind the active lease fence. The returned aggregate version is the only
// version a caller may use for the next transition or finalization.
func (service *JobService) TransitionPhase(
	ctx context.Context,
	lease ports.JobLease,
	expectedSubmissionVersion int64,
	next entity.Phase,
) (entity.SubmissionState, error) {
	if service == nil || ctx == nil || len(lease.LeaseToken) == 0 ||
		expectedSubmissionVersion < 1 {
		return entity.SubmissionState{}, ErrJobServiceInvalid
	}
	log := logger.FromContext(ctx).With(
		zap.String("next_phase", string(next)),
	)
	log.Debug(
		"judge phase transition started",
		zap.Int64("expected_submission_version", expectedSubmissionVersion),
	)
	state, err := service.repository.TransitionPhase(
		ctx,
		lease,
		expectedSubmissionVersion,
		next,
	)
	if err != nil {
		log.Error("judge phase transition failed", zap.Error(err))
		return entity.SubmissionState{}, err
	}
	log.Info(
		"judge phase transition committed",
		zap.Int64("submission_version", state.AggregateVersion),
	)
	return state, nil
}

// RecordRetry turns a worker-observed infrastructure incident into one
// service-owned full-jitter retry schedule. Callers cannot choose RetryAt.
func (service *JobService) RecordRetry(
	ctx context.Context,
	lease ports.JobLease,
	incident ports.InfrastructureIncident,
) error {
	if service == nil || ctx == nil || len(lease.LeaseToken) == 0 ||
		lease.Attempt.AttemptNumber < 1 || !validInfrastructureIncident(incident) {
		return ErrJobServiceInvalid
	}
	delay, err := service.retryDelay(lease.Attempt.AttemptNumber)
	if err != nil {
		return errors.Join(ErrJobServiceInvalid, err)
	}
	failure := ports.InfrastructureFailure{
		Classification: incident.Classification,
		Code:           incident.Code,
		Summary:        incident.Summary,
		RetryAfter:     delay,
	}
	log := logger.FromContext(ctx).With(
		zap.String("failure_classification", incident.Classification),
		zap.String("failure_code", incident.Code),
	)
	log.Debug(
		"judge infrastructure retry scheduling started",
		zap.Uint32("attempt_number", lease.Attempt.AttemptNumber),
		zap.Duration("retry_delay", delay),
	)
	if err := service.repository.RecordRetry(ctx, lease, failure); err != nil {
		service.recordLeaseEvent(ctx, LeaseMetricError, 1)
		log.Error("judge infrastructure retry scheduling failed", zap.Error(err))
		return err
	}
	service.recordLeaseEvent(ctx, LeaseMetricRetry, 1)
	log.Warn(
		"judge infrastructure retry scheduled",
		zap.Duration("retry_after", failure.RetryAfter),
	)
	return nil
}

func (service *JobService) recordLeaseEvent(
	ctx context.Context,
	outcome LeaseMetricOutcome,
	count int,
) {
	if service == nil || service.config.Observer == nil || count <= 0 {
		return
	}
	defer func() { _ = recover() }()
	service.config.Observer.RecordLeaseEvent(ctx, outcome, count)
}

func (service *JobService) LeaseDuration() time.Duration {
	if service == nil {
		return 0
	}
	return service.config.LeaseDuration
}

// HeartbeatFreshness is the database fence used to reject a stale worker
// boot. Runtime composition uses it to prove the configured heartbeat cadence
// cannot age out between healthy heartbeats.
func (service *JobService) HeartbeatFreshness() time.Duration {
	if service == nil {
		return 0
	}
	return service.config.HeartbeatFreshness
}

func (service *JobService) retryDelay(attempt uint32) (time.Duration, error) {
	if service == nil || service.random == nil || attempt < 1 {
		return 0, ErrJobServiceInvalid
	}
	ceiling := service.config.RetryInitial
	for current := uint32(1); current < attempt && ceiling < service.config.RetryMaximum; current++ {
		if ceiling > service.config.RetryMaximum/2 {
			ceiling = service.config.RetryMaximum
		} else {
			ceiling *= 2
		}
	}
	if ceiling > service.config.RetryMaximum {
		ceiling = service.config.RetryMaximum
	}
	if ceiling < time.Nanosecond {
		return 0, ErrJobServiceInvalid
	}
	value, err := rand.Int(service.random, big.NewInt(int64(ceiling)))
	if err != nil {
		return 0, err
	}
	return time.Duration(value.Int64()) + time.Nanosecond, nil
}

func validInfrastructureIncident(incident ports.InfrastructureIncident) bool {
	return validJobServiceCode(incident.Classification) &&
		validJobServiceCode(incident.Code) &&
		validJobServiceText(incident.Summary, 16<<10)
}

func validJobServiceCode(value string) bool {
	if len(value) == 0 || len(value) > 64 || value[0] < 'a' || value[0] > 'z' {
		return false
	}
	for index := 1; index < len(value); index++ {
		character := value[index]
		if character >= 'a' && character <= 'z' ||
			character >= '0' && character <= '9' ||
			character == '.' || character == '_' || character == '-' {
			continue
		}
		return false
	}
	return true
}

func validJobServiceText(value string, maximum int) bool {
	if strings.TrimSpace(value) == "" || len(value) > maximum || !utf8.ValidString(value) {
		return false
	}
	for _, character := range value {
		if unicode.IsControl(character) || unicode.In(character, unicode.Cf) {
			return false
		}
	}
	return true
}

func minServiceInt(left, right int) int {
	if left < right {
		return left
	}
	return right
}

func wipeLeaseTokens(leases []ports.JobLease) {
	for index := range leases {
		for byteIndex := range leases[index].LeaseToken {
			leases[index].LeaseToken[byteIndex] = 0
		}
		leases[index].LeaseToken = nil
	}
}
