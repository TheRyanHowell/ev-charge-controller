package workers

import (
	"context"
	"log/slog"
	"os"
	"sync/atomic"
	"testing"
	"time"
)

func TestRunWithRecovery_RecoveriesPanic(t *testing.T) {
	origWarn := slog.Default()
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelWarn}))
	defer slog.SetDefault(origWarn)
	slog.SetDefault(logger)

	ctx, cancel := context.WithCancel(t.Context())
	done := RunWithRecovery(ctx, "test-worker", func(ctx context.Context) {
		panic("test panic")
	})

	// Join the worker goroutine before the test returns so it can't race a
	// later test's workerRestartDelay override.
	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("worker did not exit after context cancellation")
	}
}

func TestRunWithRecovery_NoPanic(t *testing.T) {
	done := make(chan struct{})
	RunWithRecovery(t.Context(), "test-worker", func(ctx context.Context) {
		close(done)
	})
	<-done
}

func TestRunWithRecovery_RestartsAfterPanic(t *testing.T) {
	origDelay := workerRestartDelay
	workerRestartDelay = 10 * time.Millisecond
	defer func() { workerRestartDelay = origDelay }()

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	var calls atomic.Int32
	restarted := make(chan struct{})
	done := RunWithRecovery(ctx, "test-worker", func(ctx context.Context) {
		if calls.Add(1) == 1 {
			panic("first run panics")
		}
		close(restarted)
		<-ctx.Done()
	})

	select {
	case <-restarted:
		// success: the worker ran again after the recovered panic
	case <-time.After(2 * time.Second):
		t.Fatal("worker was not restarted after a recovered panic")
	}

	// Join the worker goroutine before restoring workerRestartDelay.
	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("worker did not exit after context cancellation")
	}
}

func TestRunWithRecovery_NoRestartAfterContextCancel(t *testing.T) {
	origDelay := workerRestartDelay
	workerRestartDelay = 50 * time.Millisecond
	defer func() { workerRestartDelay = origDelay }()

	ctx, cancel := context.WithCancel(t.Context())

	var calls atomic.Int32
	done := RunWithRecovery(ctx, "test-worker", func(ctx context.Context) {
		calls.Add(1)
		panic("always panics")
	})

	// Cancel during the restart backoff: the worker must exit instead of restarting.
	time.Sleep(10 * time.Millisecond)
	cancel()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("worker did not exit after context cancellation")
	}
	if got := calls.Load(); got != 1 {
		t.Fatalf("expected exactly 1 run before cancellation, got %d", got)
	}
}

func TestRunWithRecovery_DoneClosesOnNormalExit(t *testing.T) {
	done := RunWithRecovery(t.Context(), "test-worker", func(ctx context.Context) {})
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("done channel not closed after worker returned")
	}
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
