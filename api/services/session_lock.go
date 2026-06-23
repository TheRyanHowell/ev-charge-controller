package services

import "sync"

// sessionLock serialises all mutations to the single active charge session. It
// is shared by pointer across ChargeSessionService and its lifecycle and
// monitoring sub-services so that a status check and the write that depends on
// it cannot interleave with a concurrent Stop / StartSession.
//
// Discipline: only fast, in-process work and the session's own DB check-then-act
// belong inside the locked region. Long or external I/O (Tasmota / carbon
// intensity HTTP calls) must be performed outside it so a slow dependency never
// stalls every lifecycle operation. Giving the lock a named type (rather than a
// bare *sync.Mutex passed around) makes that shared ownership explicit.
type sessionLock struct {
	mu sync.Mutex
}

func newSessionLock() *sessionLock { return &sessionLock{} }

// Lock acquires the session lock.
func (l *sessionLock) Lock() { l.mu.Lock() }

// Unlock releases the session lock.
func (l *sessionLock) Unlock() { l.mu.Unlock() }
