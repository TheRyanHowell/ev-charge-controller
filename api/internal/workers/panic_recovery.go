package workers

import (
	"context"
	"log/slog"
	"runtime/debug"
	"time"
)

// workerRestartDelay is the pause before restarting a worker whose run
// panicked, preventing a hot crash loop when the panic recurs immediately.
// Overridable in tests.
var workerRestartDelay = 5 * time.Second

// RunWithRecovery runs fn in a goroutine with panic recovery. If fn panics,
// the panic is logged and fn is restarted after workerRestartDelay - a
// monitoring worker that dies permanently would silently stop enforcing
// charge targets. Restarts stop once ctx is cancelled or fn returns normally.
// The returned channel is closed when the goroutine (including any restarts)
// has fully exited. Because fn may be invoked multiple times, it must not
// contain run-once side effects (e.g. WaitGroup.Done) - callers wait on the
// returned channel instead.
func RunWithRecovery(ctx context.Context, workerName string, fn func(ctx context.Context)) <-chan struct{} {
	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			if !runOnceWithRecovery(ctx, workerName, fn) {
				return
			}
			select {
			case <-ctx.Done():
				return
			case <-time.After(workerRestartDelay):
				slog.Info("Worker restarting after panic", "worker", workerName)
			}
		}
	}()
	return done
}

// runOnceWithRecovery invokes fn, recovering and logging any panic.
// Returns true when fn panicked (the caller should restart it).
func runOnceWithRecovery(ctx context.Context, workerName string, fn func(ctx context.Context)) (panicked bool) {
	defer func() {
		if r := recover(); r != nil {
			panicked = true
			slog.Error("Worker panic recovered", "worker", workerName, "panic", r, "stack", string(debug.Stack()))
		}
	}()
	fn(ctx)
	return false
}
