package tasmota

import (
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestCircuitBreaker_AllowsRequestsWhenClosed(t *testing.T) {
	cb := NewCircuitBreaker(CircuitBreakerConfig{
		FailureThreshold: 3,
		ResetTimeout:     time.Second,
	})

	assert.False(t, cb.ShouldBlock())
}

func TestCircuitBreaker_RecordsSuccess(t *testing.T) {
	cb := NewCircuitBreaker(CircuitBreakerConfig{
		FailureThreshold: 3,
		ResetTimeout:     time.Second,
	})

	cb.Success()
	assert.False(t, cb.ShouldBlock())
}

func TestCircuitBreaker_OpensAfterThresholdFailures(t *testing.T) {
	cb := NewCircuitBreaker(CircuitBreakerConfig{
		FailureThreshold: 3,
		ResetTimeout:     time.Second,
	})

	cb.Failure()
	cb.Failure()
	assert.False(t, cb.ShouldBlock())

	cb.Failure()
	assert.True(t, cb.ShouldBlock())
}

func TestCircuitBreaker_RejectsRequestsWhenOpen(t *testing.T) {
	cb := NewCircuitBreaker(CircuitBreakerConfig{
		FailureThreshold: 3,
		ResetTimeout:     time.Second,
	})

	cb.Failure()
	cb.Failure()
	cb.Failure()

	assert.True(t, cb.ShouldBlock())
}

func TestCircuitBreaker_TransitionsToHalfOpenAfterTimeout(t *testing.T) {
	cb := NewCircuitBreaker(CircuitBreakerConfig{
		FailureThreshold: 3,
		ResetTimeout:     50 * time.Millisecond,
	})

	cb.Failure()
	cb.Failure()
	cb.Failure()
	assert.True(t, cb.ShouldBlock())

	time.Sleep(60 * time.Millisecond)

	// Should allow a test request in half-open state
	assert.False(t, cb.ShouldBlock())
}

func TestCircuitBreaker_ClosesOnSuccessInHalfOpen(t *testing.T) {
	cb := NewCircuitBreaker(CircuitBreakerConfig{
		FailureThreshold: 3,
		ResetTimeout:     50 * time.Millisecond,
	})

	cb.Failure()
	cb.Failure()
	cb.Failure()
	time.Sleep(60 * time.Millisecond)

	// Trigger half-open transition
	cb.ShouldBlock()
	cb.Success()

	// Should be closed again
	assert.False(t, cb.ShouldBlock())
}

func TestCircuitBreaker_ReopensOnFailureInHalfOpen(t *testing.T) {
	cb := NewCircuitBreaker(CircuitBreakerConfig{
		FailureThreshold: 3,
		ResetTimeout:     50 * time.Millisecond,
	})

	cb.Failure()
	cb.Failure()
	cb.Failure()
	time.Sleep(60 * time.Millisecond)

	// Trigger half-open transition, then fail
	cb.ShouldBlock()
	cb.Failure()

	// Should be open again
	assert.True(t, cb.ShouldBlock())
}

func TestCircuitBreaker_ConcurrentAccess(t *testing.T) {
	cb := NewCircuitBreaker(CircuitBreakerConfig{
		FailureThreshold: 100,
		ResetTimeout:     time.Second,
	})

	done := make(chan struct{})
	for i := 0; i < 50; i++ {
		go func() {
			for range 100 {
				cb.Failure()
				cb.ShouldBlock()
				select {
				case <-done:
					return
				default:
				}
			}
		}()
	}
	close(done)
}

func TestErrCircuitOpen(t *testing.T) {
	assert.Error(t, ErrCircuitOpen)
	assert.Contains(t, ErrCircuitOpen.Error(), "circuit breaker")
}

func TestCircuitBreaker_State(t *testing.T) {
	cb := NewCircuitBreaker(CircuitBreakerConfig{
		FailureThreshold: 2,
		ResetTimeout:     50 * time.Millisecond,
	})

	assert.Equal(t, CircuitClosed, cb.State())

	cb.Failure()
	cb.Failure()
	assert.Equal(t, CircuitOpen, cb.State())

	time.Sleep(60 * time.Millisecond)
	// State() is a pure query: it must NOT transition on read, even after the
	// reset window elapses. The transition happens in ShouldBlock.
	assert.Equal(t, CircuitOpen, cb.State())

	assert.False(t, cb.ShouldBlock()) // admits the probe → half-open
	assert.Equal(t, CircuitHalfOpen, cb.State())
}

// State() must be idempotent: repeated reads after the reset window elapses must
// not promote the circuit (regression test for the CQS-violating getter).
func TestCircuitBreaker_State_PureNoSideEffect(t *testing.T) {
	cb := NewCircuitBreaker(CircuitBreakerConfig{
		FailureThreshold: 2,
		ResetTimeout:     20 * time.Millisecond,
	})
	cb.Failure()
	cb.Failure()
	time.Sleep(30 * time.Millisecond)

	for range 5 {
		assert.Equal(t, CircuitOpen, cb.State(), "State() must not transition on read")
	}
	assert.True(t, cb.IsOpen())
}

// After the reset window, exactly ONE concurrent caller may be admitted as the
// half-open probe; all others must be blocked (single-flight invariant).
func TestCircuitBreaker_HalfOpen_AdmitsExactlyOneProbe(t *testing.T) {
	cb := NewCircuitBreaker(CircuitBreakerConfig{
		FailureThreshold: 1,
		ResetTimeout:     30 * time.Millisecond,
	})
	cb.Failure() // open
	time.Sleep(40 * time.Millisecond)

	const goroutines = 64
	var admitted atomic.Int64
	var wg sync.WaitGroup
	start := make(chan struct{})
	wg.Add(goroutines)
	for range goroutines {
		go func() {
			defer wg.Done()
			<-start
			if !cb.ShouldBlock() {
				admitted.Add(1)
			}
		}()
	}
	close(start)
	wg.Wait()

	assert.Equal(t, int64(1), admitted.Load(), "exactly one probe must be admitted")
	assert.Equal(t, CircuitHalfOpen, cb.State())
}

func TestCircuitBreaker_FailureCountResetOnSuccess(t *testing.T) {
	cb := NewCircuitBreaker(CircuitBreakerConfig{
		FailureThreshold: 3,
		ResetTimeout:     time.Second,
	})

	cb.Failure()
	cb.Failure()
	cb.Success()

	// After success, failures should be reset
	cb.Failure()
	cb.Failure()
	assert.False(t, cb.ShouldBlock())
}

func TestCircuitBreaker_ShouldBlockAtomic(t *testing.T) {
	cb := NewCircuitBreaker(CircuitBreakerConfig{
		FailureThreshold: 3,
		ResetTimeout:     time.Second,
	})

	var allowed int64
	for range 100 {
		if cb.ShouldBlock() {
			atomic.AddInt64(&allowed, 1)
		}
	}
	assert.Equal(t, int64(0), allowed)
}

func TestCircuitBreaker_IsOpen(t *testing.T) {
	cb := NewCircuitBreaker(CircuitBreakerConfig{
		FailureThreshold: 2,
		ResetTimeout:     time.Second,
	})

	assert.False(t, cb.IsOpen())

	cb.Failure()
	cb.Failure()
	assert.True(t, cb.IsOpen())
}

func TestCircuitBreaker_ErrOpen(t *testing.T) {
	cb := NewCircuitBreaker(CircuitBreakerConfig{
		FailureThreshold: 1,
		ResetTimeout:     time.Second,
	})

	cb.Failure()
	err := cb.ErrOpen()
	assert.Error(t, err)
	assert.ErrorIs(t, err, ErrCircuitOpen)
}

func TestCircuitBreaker_ErrOpen_NotOpen(t *testing.T) {
	cb := NewCircuitBreaker(CircuitBreakerConfig{
		FailureThreshold: 3,
		ResetTimeout:     time.Second,
	})

	err := cb.ErrOpen()
	assert.NoError(t, err)
}

func TestCircuitBreaker_DefaultConfig(t *testing.T) {
	cb := NewCircuitBreaker(CircuitBreakerConfig{})
	assert.NotNil(t, cb)
}

func TestCircuitBreaker_WrapError(t *testing.T) {
	cb := NewCircuitBreaker(CircuitBreakerConfig{
		FailureThreshold: 1,
		ResetTimeout:     time.Second,
	})

	cb.Failure()
	wrapped := cb.Wrap(errors.New("connection refused"))
	assert.Error(t, wrapped)
	assert.ErrorIs(t, wrapped, ErrCircuitOpen)
}

func TestCircuitBreaker_Wrap_Closed(t *testing.T) {
	cb := NewCircuitBreaker(CircuitBreakerConfig{
		FailureThreshold: 3,
		ResetTimeout:     time.Second,
	})

	// Circuit is closed, Wrap should return nil
	wrapped := cb.Wrap(errors.New("connection refused"))
	assert.NoError(t, wrapped)
}

func TestCircuitBreaker_ShouldBlock_HalfOpen_AllowsOne(t *testing.T) {
	cb := NewCircuitBreaker(CircuitBreakerConfig{
		FailureThreshold: 3,
		ResetTimeout:     50 * time.Millisecond,
	})

	// Open the circuit
	cb.Failure()
	cb.Failure()
	cb.Failure()
	assert.True(t, cb.ShouldBlock())

	// Wait for half-open
	time.Sleep(60 * time.Millisecond)

	// First call should allow (transitions to half-open, then takes test request)
	assert.False(t, cb.ShouldBlock())

	// The probe is now in flight (half-open). A failed probe reopens the
	// circuit and re-arms the reset window, so further requests are blocked.
	cb.Failure()
	assert.True(t, cb.ShouldBlock())
}

func TestCircuitBreaker_Failure_InHalfOpen(t *testing.T) {
	cb := NewCircuitBreaker(CircuitBreakerConfig{
		FailureThreshold: 3,
		ResetTimeout:     50 * time.Millisecond,
	})

	cb.Failure()
	cb.Failure()
	cb.Failure()
	time.Sleep(60 * time.Millisecond)

	// Trigger half-open
	cb.ShouldBlock()

	// Record failure in half-open state
	cb.Failure()

	// Should be open again
	assert.Equal(t, CircuitOpen, cb.State())
}

func TestCircuitState_String(t *testing.T) {
	tests := []struct {
		state CircuitState
		want  string
	}{
		{CircuitClosed, "closed"},
		{CircuitOpen, "open"},
		{CircuitHalfOpen, "half-open"},
		{CircuitState(99), "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.state.String())
		})
	}
}

func TestCircuitBreaker_TimeoutElapsed_ZeroLastFailure(t *testing.T) {
	cb := NewCircuitBreaker(CircuitBreakerConfig{
		FailureThreshold: 3,
		ResetTimeout:     time.Second,
	})

	// lastFailureTime is zero; timeoutElapsedLocked should return false
	// We can't call timeoutElapsedLocked directly, but we can verify
	// that ShouldBlock in open state without any failures doesn't transition.
	// Since we can't set state to open without failures, we use a fake clock.
	now := time.Now()
	cb.now = func() time.Time { return now }

	// After failures, lastFailureTime is set
	cb.Failure()
	cb.Failure()
	cb.Failure()
	assert.Equal(t, CircuitOpen, cb.State())

	// With fake clock, time hasn't elapsed
	assert.True(t, cb.ShouldBlock())
}

func TestCircuitBreaker_HalfOpen_LivenessGuard(t *testing.T) {
	// When half-open probe is lost (timeout elapses again), admit a fresh probe
	var clock time.Time
	cb := NewCircuitBreaker(CircuitBreakerConfig{
		FailureThreshold: 1,
		ResetTimeout:     time.Second,
	})
	cb.now = func() time.Time { return clock }

	// Open the circuit
	clock = time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	cb.Failure()
	assert.Equal(t, CircuitOpen, cb.State())

	// Advance past reset timeout -> half-open
	clock = clock.Add(2 * time.Second)
	assert.False(t, cb.ShouldBlock()) // admits probe, transitions to half-open
	assert.Equal(t, CircuitHalfOpen, cb.State())

	// While still within window, second caller is blocked
	assert.True(t, cb.ShouldBlock())

	// Advance past reset timeout again -> liveness guard admits fresh probe
	clock = clock.Add(2 * time.Second)
	assert.False(t, cb.ShouldBlock()) // liveness guard: admits new probe
}

func TestCircuitBreaker_ShouldBlock_DefaultState(t *testing.T) {
	// Test the default case (unknown state) - should return false
	cb := &CircuitBreaker{
		state:        CircuitState(99), // unknown state
		threshold:    3,
		resetTimeout: time.Second,
		now:          time.Now,
	}

	// Unknown state should not block
	assert.False(t, cb.ShouldBlock())
}

func TestCircuitBreaker_Failure_BelowThreshold(t *testing.T) {
	cb := NewCircuitBreaker(CircuitBreakerConfig{
		FailureThreshold: 5,
		ResetTimeout:     time.Second,
	})

	// Failures below threshold should not open
	cb.Failure()
	cb.Failure()
	cb.Failure()
	assert.Equal(t, CircuitClosed, cb.State())
	assert.False(t, cb.ShouldBlock())
}

func TestCircuitBreaker_TransitionLocked_NoOpOnSameState(t *testing.T) {
	cb := NewCircuitBreaker(CircuitBreakerConfig{
		FailureThreshold: 3,
		ResetTimeout:     time.Second,
	})

	// transitionLocked is private, but we can verify that calling
	// Success() when already closed doesn't cause issues
	cb.Success()
	cb.Success()
	assert.Equal(t, CircuitClosed, cb.State())
	assert.False(t, cb.ShouldBlock())
}

func TestCircuitBreaker_ZeroConfigDefaults(t *testing.T) {
	cb := NewCircuitBreaker(CircuitBreakerConfig{})

	// Should use defaults
	assert.Equal(t, CircuitClosed, cb.State())
	assert.NotNil(t, cb.now)

	// Default threshold is 5
	for range 4 {
		cb.Failure()
		assert.Equal(t, CircuitClosed, cb.State())
	}
	cb.Failure()
	assert.Equal(t, CircuitOpen, cb.State())
}
