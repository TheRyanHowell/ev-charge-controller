package services

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"ev-charge-controller/api/tasmota"
)

// socWorkerBufferSize is the capacity of the buffered channel for SOC snapshot requests.
const socWorkerBufferSize = 20

// socRequest holds data needed to asynchronously store an SOC snapshot.
type socRequest struct {
	sessionID      string
	vehicleID      string
	startKwh       float64
	startTotalKwh  float64
	targetKwh      float64
	createdAt      time.Time
	startedAt      *time.Time
	lastBlendedKwh *float64
	energy         *tasmota.EnergyData
}

// SOCProcessor handles the actual SOC calculation and persistence.
type SOCProcessor interface {
	ProcessSOC(ctx context.Context, req socRequest) error
}

// SOCWorker processes SOC snapshot requests asynchronously, offloading
// expensive calculations and database writes from the polling hot path.
type SOCWorker struct {
	processor SOCProcessor
	ch        chan socRequest
	done      chan struct{}
	closeMu   sync.Mutex
	closed    bool
}

// NewSOCWorker creates a new SOCWorker with the given processor.
func NewSOCWorker(processor SOCProcessor) *SOCWorker {
	return &SOCWorker{
		processor: processor,
		ch:        make(chan socRequest, socWorkerBufferSize),
		done:      make(chan struct{}),
	}
}

// Start begins processing SOC requests. Blocks until the channel is closed
// or the context is cancelled.
func (w *SOCWorker) Start(ctx context.Context) {
	defer close(w.done)
	for {
		select {
		case <-ctx.Done():
			return
		case req, ok := <-w.ch:
			if !ok {
				return
			}
			reqCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
			if err := w.processor.ProcessSOC(reqCtx, req); err != nil {
				slog.Error("Error processing SOC snapshot", "sessionID", req.sessionID, "err", err)
			}
			cancel()
		}
	}
}

// Send attempts to send a SOC request to the worker.
// Returns false if the channel is full and the request is dropped.
func (w *SOCWorker) Send(req socRequest) bool {
	select {
	case w.ch <- req:
		return true
	default:
		slog.Warn("SOC worker channel full, dropping snapshot", "sessionID", req.sessionID)
		return false
	}
}

// Shutdown closes the worker channel and waits for the worker to finish.
// Safe to call multiple times. Safe to call without ever starting the worker.
func (w *SOCWorker) Shutdown() {
	w.closeMu.Lock()
	defer w.closeMu.Unlock()
	if w.closed {
		return
	}
	w.closed = true
	close(w.ch)
	select {
	case <-w.done:
		slog.Info("SOCWorker shut down")
	default:
		slog.Info("SOCWorker shut down (worker was not started)")
	}
}
