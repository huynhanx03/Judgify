package judge

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"errors"
	"sync"
	"time"

	"github.com/google/uuid"

	submissionports "github.com/huynhanx03/judgify/internal/ports"
	entity "github.com/huynhanx03/judgify/internal/submission/domain"
)

const (
	maximumSchedulerCapacity    = 4096
	maximumSchedulerConcurrency = 256
)

var (
	ErrSchedulerInvalid    = errors.New("judge scheduler: invalid operation")
	ErrSchedulerOverloaded = errors.New("judge scheduler: bounded queue is full")
	ErrSchedulerClosed     = errors.New("judge scheduler: admission is closed")
)

var schedulerLanes = [...]string{"contest", "regular", "maintenance"}

type FairSchedulerConfig struct {
	Capacity    int
	Concurrency int
	LaneWeights map[string]int
}

type scheduledLease struct {
	lease      submissionports.JobLease
	enqueuedAt time.Time
}

type leaseQueue struct {
	values []scheduledLease
	head   int
}

func (queue *leaseQueue) depth() int {
	if queue == nil {
		return 0
	}
	return len(queue.values) - queue.head
}

func (queue *leaseQueue) push(value scheduledLease) {
	queue.values = append(queue.values, value)
}

func (queue *leaseQueue) pop() scheduledLease {
	value := queue.values[queue.head]
	queue.values[queue.head] = scheduledLease{}
	queue.head++
	if queue.head == len(queue.values) {
		queue.values = queue.values[:0]
		queue.head = 0
	}
	return value
}

func (queue *leaseQueue) oldest() (time.Time, bool) {
	if queue.depth() == 0 {
		return time.Time{}, false
	}
	return queue.values[queue.head].enqueuedAt, true
}

type schedulerLeaseKey struct {
	jobID     uuid.UUID
	attemptID uuid.UUID
}

// FairScheduler is a bounded, process-local accelerator containing only
// leases already committed by PostgreSQL. It owns its worker goroutines;
// losing this entire structure is safe because lease expiry returns unfinished
// work to the durable reaper.
type FairScheduler struct {
	mu          sync.Mutex
	queues      [3]leaseQueue
	weights     [3]int
	credits     [3]int
	capacity    int
	concurrency int
	depth       int
	owned       map[schedulerLeaseKey]struct{}
	notify      chan struct{}
	done        chan struct{}
	closed      bool
	running     bool
	now         func() time.Time
	closeOnce   sync.Once
}

func NewFairScheduler(config FairSchedulerConfig) (*FairScheduler, error) {
	if config.Capacity <= 0 || config.Capacity > maximumSchedulerCapacity ||
		config.Concurrency <= 0 ||
		config.Concurrency > maximumSchedulerConcurrency ||
		config.Concurrency > config.Capacity {
		return nil, ErrSchedulerInvalid
	}
	weights := [3]int{60, 35, 5}
	known := make(map[string]struct{}, len(schedulerLanes))
	for index, lane := range schedulerLanes {
		known[lane] = struct{}{}
		if configured, exists := config.LaneWeights[lane]; exists {
			if configured <= 0 {
				return nil, ErrSchedulerInvalid
			}
			weights[index] = configured
		}
	}
	for lane := range config.LaneWeights {
		if _, exists := known[lane]; !exists {
			return nil, ErrSchedulerInvalid
		}
	}
	total := 0
	for _, weight := range weights {
		if weight > int(^uint(0)>>1)-total {
			return nil, ErrSchedulerInvalid
		}
		total += weight
	}
	return &FairScheduler{
		weights: weights, capacity: config.Capacity,
		concurrency: config.Concurrency,
		owned:       make(map[schedulerLeaseKey]struct{}, config.Capacity),
		notify:      make(chan struct{}, 1),
		done:        make(chan struct{}),
		now:         func() time.Time { return time.Now().UTC() },
	}, nil
}

// TryEnqueueBatch is an atomic, non-blocking ownership transfer. On success
// the scheduler owns each raw token and clears the caller's slice fields. On
// error it leaves the entire batch untouched.
func (scheduler *FairScheduler) TryEnqueueBatch(
	leases []submissionports.JobLease,
) error {
	if scheduler == nil {
		return ErrSchedulerInvalid
	}
	if len(leases) == 0 {
		return nil
	}
	batchKeys := make(map[schedulerLeaseKey]struct{}, len(leases))
	laneIndexes := make([]int, len(leases))
	for index := range leases {
		laneIndex, err := validateScheduledLease(leases[index])
		if err != nil {
			return err
		}
		key := schedulerLeaseKey{
			jobID: leases[index].Job.ID, attemptID: leases[index].Attempt.ID,
		}
		if _, duplicate := batchKeys[key]; duplicate {
			return ErrSchedulerInvalid
		}
		batchKeys[key] = struct{}{}
		laneIndexes[index] = laneIndex
	}
	scheduler.mu.Lock()
	if scheduler.closed {
		scheduler.mu.Unlock()
		return ErrSchedulerClosed
	}
	if len(leases) > scheduler.capacity-scheduler.depth {
		scheduler.mu.Unlock()
		return ErrSchedulerOverloaded
	}
	for key := range batchKeys {
		if _, duplicate := scheduler.owned[key]; duplicate {
			scheduler.mu.Unlock()
			return ErrSchedulerInvalid
		}
	}
	enqueuedAt := scheduler.now().UTC()
	for index := range leases {
		lease := leases[index]
		key := schedulerLeaseKey{
			jobID: lease.Job.ID, attemptID: lease.Attempt.ID,
		}
		scheduler.queues[laneIndexes[index]].push(scheduledLease{
			lease: lease, enqueuedAt: enqueuedAt,
		})
		scheduler.owned[key] = struct{}{}
		scheduler.depth++
		leases[index].LeaseToken = nil
	}
	scheduler.mu.Unlock()
	scheduler.signal()
	return nil
}

func validateScheduledLease(lease submissionports.JobLease) (int, error) {
	if lease.Job.ID == uuid.Nil || lease.Job.SubmissionID == uuid.Nil ||
		lease.Job.JudgeSpecID == uuid.Nil || lease.Job.Generation < 1 ||
		lease.Job.State != entity.JobLeased || lease.Attempt.ID == uuid.Nil ||
		lease.Attempt.JobID != lease.Job.ID ||
		lease.Attempt.SubmissionID != lease.Job.SubmissionID ||
		lease.Attempt.Generation != lease.Job.Generation ||
		lease.Attempt.JudgeSpecID != lease.Job.JudgeSpecID ||
		lease.Attempt.WorkerID == uuid.Nil ||
		lease.Attempt.WorkerBootID == uuid.Nil ||
		lease.JudgeSpec.ID != lease.Job.JudgeSpecID ||
		lease.JudgeSpec.SubmissionID != lease.Job.SubmissionID ||
		lease.JudgeSpec.Generation != lease.Job.Generation ||
		lease.LeaseOwner == "" || len(lease.LeaseOwner) > 128 ||
		len(lease.LeaseToken) != sha256.Size || lease.LeaseUntil.IsZero() ||
		lease.SubmissionVersion < 1 || lease.CreatedAt.IsZero() {
		return 0, ErrSchedulerInvalid
	}
	presentedHash := sha256.Sum256(lease.LeaseToken)
	if subtle.ConstantTimeCompare(
		presentedHash[:],
		lease.Attempt.LeaseTokenHash[:],
	) != 1 {
		return 0, ErrSchedulerInvalid
	}
	for index, lane := range schedulerLanes {
		if lease.Job.Lane == lane {
			return index, nil
		}
	}
	return 0, ErrSchedulerInvalid
}

func (scheduler *FairScheduler) signal() {
	select {
	case scheduler.notify <- struct{}{}:
	default:
	}
}

func (scheduler *FairScheduler) Remaining() int {
	if scheduler == nil {
		return 0
	}
	scheduler.mu.Lock()
	defer scheduler.mu.Unlock()
	return scheduler.capacity - scheduler.depth
}

func (scheduler *FairScheduler) Depth() int {
	if scheduler == nil {
		return 0
	}
	scheduler.mu.Lock()
	defer scheduler.mu.Unlock()
	return scheduler.depth
}

// FreeExecutionSlots reports physical local execution capacity, excluding
// leases already running while deliberately ignoring queued depth. JobService
// combines this value with Remaining(), so neither executor slots nor the
// bounded queue can be over-admitted.
func (scheduler *FairScheduler) FreeExecutionSlots() int {
	if scheduler == nil {
		return 0
	}
	scheduler.mu.Lock()
	defer scheduler.mu.Unlock()
	if scheduler.closed {
		return 0
	}
	active := len(scheduler.owned) - scheduler.depth
	if active < 0 || active >= scheduler.concurrency {
		return 0
	}
	return scheduler.concurrency - active
}

func (scheduler *FairScheduler) OldestAge(now time.Time) time.Duration {
	if scheduler == nil || now.IsZero() {
		return 0
	}
	scheduler.mu.Lock()
	defer scheduler.mu.Unlock()
	var oldest time.Time
	for index := range scheduler.queues {
		value, exists := scheduler.queues[index].oldest()
		if exists && (oldest.IsZero() || value.Before(oldest)) {
			oldest = value
		}
	}
	if oldest.IsZero() || now.Before(oldest) {
		return 0
	}
	return now.Sub(oldest)
}

type LeaseHandler func(context.Context, submissionports.JobLease)

// Run starts exactly Concurrency workers and blocks until ctx is cancelled.
// A handler call is synchronous ownership: the raw lease token is wiped as
// soon as the handler returns.
func (scheduler *FairScheduler) Run(ctx context.Context, handler LeaseHandler) error {
	if scheduler == nil || ctx == nil || handler == nil {
		return ErrSchedulerInvalid
	}
	scheduler.mu.Lock()
	if scheduler.running || scheduler.closed {
		scheduler.mu.Unlock()
		return ErrSchedulerClosed
	}
	scheduler.running = true
	scheduler.mu.Unlock()

	var workers sync.WaitGroup
	workers.Add(scheduler.concurrency)
	for range scheduler.concurrency {
		go func() {
			defer workers.Done()
			for {
				lease, err := scheduler.take(ctx)
				if err != nil {
					return
				}
				func() {
					defer scheduler.release(lease)
					defer wipeRawLeaseToken(&lease)
					handler(ctx, lease)
				}()
			}
		}()
	}
	workers.Wait()
	scheduler.Close()
	return nil
}

func (scheduler *FairScheduler) take(
	ctx context.Context,
) (submissionports.JobLease, error) {
	for {
		if err := ctx.Err(); err != nil {
			return submissionports.JobLease{}, err
		}
		scheduler.mu.Lock()
		if scheduler.closed {
			scheduler.mu.Unlock()
			return submissionports.JobLease{}, ErrSchedulerClosed
		}
		if scheduler.depth > 0 {
			lane := scheduler.nextLaneLocked()
			value := scheduler.queues[lane].pop()
			scheduler.depth--
			if scheduler.depth > 0 {
				scheduler.signal()
			}
			scheduler.mu.Unlock()
			return value.lease, nil
		}
		scheduler.mu.Unlock()
		select {
		case <-ctx.Done():
			return submissionports.JobLease{}, ctx.Err()
		case <-scheduler.done:
			return submissionports.JobLease{}, ErrSchedulerClosed
		case <-scheduler.notify:
		}
	}
}

func (scheduler *FairScheduler) release(lease submissionports.JobLease) {
	scheduler.mu.Lock()
	delete(scheduler.owned, schedulerLeaseKey{
		jobID: lease.Job.ID, attemptID: lease.Attempt.ID,
	})
	scheduler.mu.Unlock()
}

// nextLaneLocked implements smooth weighted deficit scheduling among only
// currently non-empty lanes. Empty lanes accrue no burst credit; all non-empty
// lanes receive their configured share and FIFO ordering within a lane.
func (scheduler *FairScheduler) nextLaneLocked() int {
	selected := -1
	activeWeight := 0
	for index := range scheduler.queues {
		if scheduler.queues[index].depth() == 0 {
			scheduler.credits[index] = 0
			continue
		}
		scheduler.credits[index] += scheduler.weights[index]
		activeWeight += scheduler.weights[index]
		if selected == -1 || scheduler.credits[index] > scheduler.credits[selected] {
			selected = index
		}
	}
	scheduler.credits[selected] -= activeWeight
	return selected
}

// Close rejects new work and securely forgets every queued lease. In-flight
// handlers are cancelled by the Run context and wipe their token on return.
func (scheduler *FairScheduler) Close() {
	if scheduler == nil {
		return
	}
	scheduler.closeOnce.Do(func() {
		scheduler.mu.Lock()
		scheduler.closed = true
		for laneIndex := range scheduler.queues {
			queue := &scheduler.queues[laneIndex]
			for queue.depth() > 0 {
				value := queue.pop()
				wipeRawLeaseToken(&value.lease)
			}
		}
		clear(scheduler.owned)
		scheduler.depth = 0
		scheduler.mu.Unlock()
		close(scheduler.done)
	})
}

func wipeRawLeaseToken(lease *submissionports.JobLease) {
	if lease == nil {
		return
	}
	for index := range lease.LeaseToken {
		lease.LeaseToken[index] = 0
	}
	lease.LeaseToken = nil
}

var _ interface {
	TryEnqueueBatch([]submissionports.JobLease) error
	Remaining() int
	Depth() int
	OldestAge(time.Time) time.Duration
} = (*FairScheduler)(nil)
