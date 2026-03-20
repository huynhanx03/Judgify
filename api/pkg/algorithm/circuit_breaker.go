package algorithm

import (
	"errors"
	"sync"
	"time"
)

// Circuit breaker states.
const (
	StateClosed   = "closed"
	StateOpen     = "open"
	StateHalfOpen = "half_open"
)

// Circuit breaker defaults.
const (
	defaultFailureThreshold = 5
	defaultSuccessThreshold = 2
	defaultOpenTimeout      = 30 * time.Second
)

// Sentinel errors.
var (
	ErrCircuitOpen = errors.New("circuit breaker is open")
)

// CircuitBreaker implements the circuit breaker pattern.
// States: Closed → Open → HalfOpen → Closed (or back to Open).
type CircuitBreaker struct {
	mu               sync.Mutex
	state            string
	failureCount     int
	successCount     int
	failureThreshold int
	successThreshold int
	openTimeout      time.Duration
	lastFailure      int64
	onStateChange    func(from, to string)
	now              func() int64
}

// CircuitBreakerOption configures a CircuitBreaker.
type CircuitBreakerOption func(*CircuitBreaker)

// WithFailureThreshold sets consecutive failures needed to open the circuit.
func WithFailureThreshold(n int) CircuitBreakerOption {
	return func(cb *CircuitBreaker) {
		if n > 0 {
			cb.failureThreshold = n
		}
	}
}

// WithSuccessThreshold sets consecutive successes in half-open to close the circuit.
func WithSuccessThreshold(n int) CircuitBreakerOption {
	return func(cb *CircuitBreaker) {
		if n > 0 {
			cb.successThreshold = n
		}
	}
}

// WithOpenTimeout sets how long the circuit stays open before transitioning to half-open.
func WithOpenTimeout(d time.Duration) CircuitBreakerOption {
	return func(cb *CircuitBreaker) {
		if d > 0 {
			cb.openTimeout = d
		}
	}
}

// WithOnStateChange sets a callback invoked on state transitions.
func WithOnStateChange(fn func(from, to string)) CircuitBreakerOption {
	return func(cb *CircuitBreaker) {
		cb.onStateChange = fn
	}
}

// WithBreakerClock overrides the time source (unix nanoseconds).
func WithBreakerClock(fn func() int64) CircuitBreakerOption {
	return func(cb *CircuitBreaker) {
		if fn != nil {
			cb.now = fn
		}
	}
}

// NewCircuitBreaker creates a circuit breaker with the given options.
func NewCircuitBreaker(opts ...CircuitBreakerOption) *CircuitBreaker {
	cb := &CircuitBreaker{
		state:            StateClosed,
		failureThreshold: defaultFailureThreshold,
		successThreshold: defaultSuccessThreshold,
		openTimeout:      defaultOpenTimeout,
		now:              func() int64 { return time.Now().UnixNano() },
	}
	for _, opt := range opts {
		opt(cb)
	}
	return cb
}

// Allow checks if a request is permitted through the circuit breaker.
// Returns ErrCircuitOpen if the circuit is open.
func (cb *CircuitBreaker) Allow() error {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	switch cb.state {
	case StateClosed:
		return nil
	case StateOpen:
		if cb.now()-cb.lastFailure >= int64(cb.openTimeout) {
			cb.transition(StateHalfOpen)
			return nil
		}
		return ErrCircuitOpen
	case StateHalfOpen:
		return nil
	default:
		return nil
	}
}

// RecordSuccess records a successful operation.
func (cb *CircuitBreaker) RecordSuccess() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	switch cb.state {
	case StateClosed:
		cb.failureCount = 0
	case StateHalfOpen:
		cb.successCount++
		if cb.successCount >= cb.successThreshold {
			cb.transition(StateClosed)
		}
	}
}

// RecordFailure records a failed operation.
func (cb *CircuitBreaker) RecordFailure() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.lastFailure = cb.now()

	switch cb.state {
	case StateClosed:
		cb.failureCount++
		if cb.failureCount >= cb.failureThreshold {
			cb.transition(StateOpen)
		}
	case StateHalfOpen:
		cb.transition(StateOpen)
	}
}

// State returns the current circuit breaker state.
func (cb *CircuitBreaker) State() string {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	return cb.state
}

// Reset returns the circuit breaker to the closed state.
func (cb *CircuitBreaker) Reset() {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.transition(StateClosed)
}

// Counts returns current failure and success counters.
func (cb *CircuitBreaker) Counts() (failures, successes int) {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	return cb.failureCount, cb.successCount
}

// transition moves to a new state and fires the callback.
// Caller must hold cb.mu.
func (cb *CircuitBreaker) transition(to string) {
	from := cb.state
	cb.state = to
	cb.failureCount = 0
	cb.successCount = 0

	if cb.onStateChange != nil && from != to {
		cb.onStateChange(from, to)
	}
}
