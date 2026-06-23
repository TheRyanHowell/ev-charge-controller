package tasmota

import (
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"
)

// CircuitState represents the state of the circuit breaker.
type CircuitState int

const (
	// CircuitClosed allows requests through normally.
	CircuitClosed CircuitState = iota
	// CircuitOpen rejects requests immediately.
	CircuitOpen
	// CircuitHalfOpen allows a single test request through.
	CircuitHalfOpen
)

// String returns a human-readable name for the state (used in transition logs).
func (s CircuitState) String() string {
	switch s {
	case CircuitClosed:
		return "closed"
	case CircuitOpen:
		return "open"
	case CircuitHalfOpen:
		return "half-open"
	default:
		return "unknown"
	}
}

// CircuitBreakerConfig holds configuration for the circuit breaker.
type CircuitBreakerConfig struct {
	// FailureThreshold is the number of consecutive failures before opening.
	FailureThreshold int
	// ResetTimeout is the duration to wait before transitioning from open to half-open.
	ResetTimeout time.Duration
}

// Default circuit breaker tuning, used when a config field is left zero.
const (
	defaultFailureThreshold = 5
	defaultResetTimeout     = 30 * time.Second
)

// ErrCircuitOpen is returned when the circuit breaker is open and rejecting requests.
var ErrCircuitOpen = errors.New("circuit breaker is open")

// CircuitBreaker prevents cascading failures by stopping requests to a
// failing dependency after a threshold of consecutive failures.
//
// States:
//   - Closed: requests pass through normally. Failures increment a counter;
//     a success resets it.
//   - Open: requests are rejected immediately. After ResetTimeout, the next
//     request is promoted to a single half-open probe.
//   - Half-open: exactly one probe request is admitted. Success closes the
//     circuit; failure reopens it.
//
// Concurrency model: all state lives behind mu. ShouldBlock is the single
// request-path gate - it performs the Open→HalfOpen transition itself and
// admits exactly one probe per reset window (single-flight). State is a pure
// query (CQS): it never mutates, so callers can observe state without side
// effects. The clock is injectable (now) so timeout behaviour is testable
// without sleeps.
type CircuitBreaker struct {
	mu              sync.Mutex
	state           CircuitState
	failures        int
	threshold       int
	resetTimeout    time.Duration
	lastFailureTime time.Time
	now             func() time.Time
}

// NewCircuitBreaker creates a new circuit breaker with the given config.
// Zero-valued config fields fall back to defaultFailureThreshold / defaultResetTimeout.
func NewCircuitBreaker(cfg CircuitBreakerConfig) *CircuitBreaker {
	threshold := cfg.FailureThreshold
	if threshold <= 0 {
		threshold = defaultFailureThreshold
	}
	timeout := cfg.ResetTimeout
	if timeout <= 0 {
		timeout = defaultResetTimeout
	}

	return &CircuitBreaker{
		state:        CircuitClosed,
		threshold:    threshold,
		resetTimeout: timeout,
		now:          time.Now,
	}
}

// State returns the current circuit state. This is a pure query: it never
// mutates state (no read-time transitions). State transitions happen only in
// ShouldBlock, Success, and Failure.
func (cb *CircuitBreaker) State() CircuitState {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	return cb.state
}

// ShouldBlock reports whether a request must be rejected (true = block).
// It is the single gate the request path calls. When the circuit is open and
// the reset window has elapsed, it promotes to half-open and admits the calling
// goroutine as the one probe, re-arming the clock so concurrent callers are
// blocked until the probe resolves via Success or Failure.
func (cb *CircuitBreaker) ShouldBlock() bool {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	switch cb.state {
	case CircuitClosed:
		return false
	case CircuitOpen:
		if !cb.timeoutElapsedLocked() {
			return true
		}
		// Reset window elapsed: promote to half-open and admit THIS caller as
		// the single probe. Re-arming lastFailureTime makes the transition
		// self-arming - concurrent callers see half-open within the window and
		// are blocked, guaranteeing exactly one probe.
		cb.transitionLocked(CircuitHalfOpen)
		cb.lastFailureTime = cb.now()
		return false
	case CircuitHalfOpen:
		// A probe is in flight. Block other callers until it resolves - unless
		// the window elapsed again, in which case the probe was lost and we
		// admit a fresh one (liveness guard).
		if !cb.timeoutElapsedLocked() {
			return true
		}
		cb.lastFailureTime = cb.now()
		return false
	default:
		return false
	}
}

// Success records a successful request, closing the circuit and resetting the
// failure counter.
func (cb *CircuitBreaker) Success() {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.transitionLocked(CircuitClosed)
	cb.failures = 0
}

// Failure records a failed request. A failure during a half-open probe reopens
// the circuit immediately; otherwise it increments the counter and opens the
// circuit once the threshold is reached.
func (cb *CircuitBreaker) Failure() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	if cb.state == CircuitHalfOpen {
		cb.transitionLocked(CircuitOpen)
		cb.lastFailureTime = cb.now()
		return
	}

	cb.failures++
	if cb.failures >= cb.threshold {
		cb.transitionLocked(CircuitOpen)
		cb.lastFailureTime = cb.now()
	}
}

// IsOpen returns true if the circuit is currently open (rejecting requests).
// Pure query; does not transition.
func (cb *CircuitBreaker) IsOpen() bool {
	return cb.State() == CircuitOpen
}

// ErrOpen returns ErrCircuitOpen if the circuit is open, nil otherwise.
func (cb *CircuitBreaker) ErrOpen() error {
	if cb.IsOpen() {
		return ErrCircuitOpen
	}
	return nil
}

// Wrap wraps an error with circuit breaker context. If the circuit is open,
// returns a wrapped ErrCircuitOpen. Otherwise returns nil.
func (cb *CircuitBreaker) Wrap(err error) error {
	if cb.IsOpen() {
		return fmt.Errorf("%w: %w", ErrCircuitOpen, err)
	}
	return nil
}

// transitionLocked sets the state and logs the transition. Caller must hold mu.
func (cb *CircuitBreaker) transitionLocked(to CircuitState) {
	if cb.state == to {
		return
	}
	slog.Info("[CircuitBreaker] state transition", "from", cb.state.String(), "to", to.String())
	cb.state = to
}

// timeoutElapsedLocked reports whether the reset window has elapsed since the
// last failure. Caller must hold mu.
func (cb *CircuitBreaker) timeoutElapsedLocked() bool {
	if cb.lastFailureTime.IsZero() {
		return false
	}
	return cb.now().Sub(cb.lastFailureTime) >= cb.resetTimeout
}
