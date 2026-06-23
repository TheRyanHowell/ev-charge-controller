package workers

import (
	"context"
	"log/slog"
	"time"
)

// RunTickerWorker runs fn on every interval tick until ctx is cancelled.
// A buffered channel prevents overlapping ticks: if fn is still running when
// the next tick fires, that tick is silently dropped rather than queued.
func RunTickerWorker(ctx context.Context, interval time.Duration, name string, fn func(ctx context.Context)) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	running := make(chan struct{}, 1)
	slog.Info(name+" started", "interval", interval)

	for {
		select {
		case <-ctx.Done():
			slog.Info(name + " stopped")
			return
		case <-ticker.C:
			select {
			case running <- struct{}{}:
				fn(ctx)
				<-running
			default:
				slog.Debug(name + ": skipping tick, previous tick still running")
			}
		}
	}
}
