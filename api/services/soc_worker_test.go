package services

import (
	"context"
	"sync"
	"testing"
	"time"

	"ev-charge-controller/api/models"
	"ev-charge-controller/api/tasmota"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockSOCProcessor struct {
	mu         sync.Mutex
	calls      []socRequest
	snapshots  []*models.SOCSnapshot
	lastBlends []float64
}

func (m *mockSOCProcessor) ProcessSOC(ctx context.Context, req socRequest) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.calls = append(m.calls, req)
	m.snapshots = append(m.snapshots, &models.SOCSnapshot{
		SessionID: req.sessionID,
		SocPercent: 75.0,
	})
	m.lastBlends = append(m.lastBlends, req.startKwh+1.0)
	return nil
}

func (m *mockSOCProcessor) GetCalls() []socRequest {
	m.mu.Lock()
	defer m.mu.Unlock()
	result := make([]socRequest, len(m.calls))
	copy(result, m.calls)
	return result
}

func (m *mockSOCProcessor) GetSnapshots() []*models.SOCSnapshot {
	m.mu.Lock()
	defer m.mu.Unlock()
	result := make([]*models.SOCSnapshot, len(m.snapshots))
	copy(result, m.snapshots)
	return result
}

func (m *mockSOCProcessor) GetLastBlends() []float64 {
	m.mu.Lock()
	defer m.mu.Unlock()
	result := make([]float64, len(m.lastBlends))
	copy(result, m.lastBlends)
	return result
}

func TestSOCWorker_ProcessRequests(t *testing.T) {
	processor := &mockSOCProcessor{}
	worker := NewSOCWorker(processor)

	// Send a request before starting - should not panic
	req1 := socRequest{
		sessionID:     "session-1",
		vehicleID:     "vehicle-1",
		startKwh:      20.0,
		startTotalKwh: 100.0,
		targetKwh:     60.0,
		energy: &tasmota.EnergyData{
			Total:   101.0,
			Power:   3000,
			Voltage: 230,
			Current: 13,
		},
	}

	worker.Send(req1)

	// Start the worker
	ctx, cancel := context.WithCancel(context.Background())
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		worker.Start(ctx)
	}()

	// Give the worker time to process
	time.Sleep(50 * time.Millisecond)

	// Send another request
	req2 := socRequest{
		sessionID: "session-2",
		vehicleID: "vehicle-2",
		startKwh:  30.0,
		energy: &tasmota.EnergyData{
			Total:   200.0,
			Power:   5000,
			Voltage: 240,
			Current: 20,
		},
	}
	worker.Send(req2)

	// Give the worker time to process
	time.Sleep(50 * time.Millisecond)

	// Cancel context to stop the worker
	cancel()
	worker.Shutdown()
	wg.Wait()

	// Verify both requests were processed
	calls := processor.GetCalls()
	require.Len(t, calls, 2)
	assert.Equal(t, "session-1", calls[0].sessionID)
	assert.Equal(t, "session-2", calls[1].sessionID)
}

func TestSOCWorker_DropWhenFull(t *testing.T) {
	// Create a processor that blocks to fill the channel
	blockingProcessor := &mockSOCProcessor{}
	worker := NewSOCWorker(blockingProcessor)

	// Fill the channel to capacity
	for i := 0; i < socWorkerBufferSize; i++ {
		worker.Send(socRequest{
			sessionID: string(rune('a' + i%26)),
			energy: &tasmota.EnergyData{
				Total: 100.0,
			},
		})
	}

	// Next send should be dropped (return false)
	dropped := worker.Send(socRequest{
		sessionID: "should-be-dropped",
		energy: &tasmota.EnergyData{
			Total: 100.0,
		},
	})
	assert.False(t, dropped, "Send should return false when channel is full")

	// Shutdown without processing (channel will drain on close)
	worker.Shutdown()
}

func TestSOCWorker_ShutdownWithoutStart(t *testing.T) {
	processor := &mockSOCProcessor{}
	worker := NewSOCWorker(processor)

	// Send a request
	worker.Send(socRequest{
		sessionID: "test",
		energy: &tasmota.EnergyData{
			Total: 100.0,
		},
	})

	// Shutdown without starting - should not panic
	worker.Shutdown()
}
