package workers

import (
	"context"
	"log/slog"
	"runtime/debug"
)

// RunWithRecovery runs fn in a goroutine with panic recovery.
// If fn panics, the panic is logged and the goroutine exits cleanly.
func RunWithRecovery(ctx context.Context, workerName string, fn func(ctx context.Context)) {
	go func() {
		defer func() {
			if r := recover(); r != nil {
				trace := debug.Stack()
				slog.Error("Worker panic recovered", "worker", workerName, "panic", r, "stack", string(trace))
			}
		}()
		fn(ctx)
	}()
}
