package workers

import (
	"context"
	"log/slog"
	"os"
	"testing"
	"time"
)

func TestRunWithRecovery_RecoveriesPanic(t *testing.T) {
	origWarn := slog.Default()
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelWarn}))
	defer slog.SetDefault(origWarn)
	slog.SetDefault(logger)

	done := make(chan struct{})
	func() {
		defer func() {
			close(done)
		}()
		RunWithRecovery(t.Context(), "test-worker", func(ctx context.Context) {
			panic("test panic")
		})
	}()

	<-done
}

func TestRunWithRecovery_NoPanic(t *testing.T) {
	done := make(chan struct{})
	RunWithRecovery(t.Context(), "test-worker", func(ctx context.Context) {
		close(done)
	})
	<-done
}

func TestRunWithRecovery_ContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan struct{})
	RunWithRecovery(ctx, "test-worker", func(ctx context.Context) {
		<-ctx.Done()
		close(done)
	})
	time.Sleep(50 * time.Millisecond)
	cancel()
	select {
	case <-done:
		// success
	case <-time.After(1 * time.Second):
		t.Fatal("goroutine did not exit after context cancellation")
	}
}
