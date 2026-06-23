package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"ev-charge-controller/api/database"
	"ev-charge-controller/api/internal"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInitServices(t *testing.T) {
	db, err := database.Init(":memory:")
	require.NoError(t, err)
	defer db.Close()

	t.Setenv("VAPID_PUBLIC_KEY", "")
	t.Setenv("VAPID_PRIVATE_KEY", "")

	cfg := internal.LoadConfig()
	svcs := initServices(context.Background(), db, cfg)

	require.NotNil(t, svcs)
	assert.NotNil(t, svcs.vehicleHandler)
	assert.NotNil(t, svcs.chargeHandler)
	assert.NotNil(t, svcs.scheduleHandler)
	assert.NotNil(t, svcs.chargeService)
	assert.NotNil(t, svcs.scheduleService)
	assert.Nil(t, svcs.pushHandler)
}

func TestInitServices_WithPush(t *testing.T) {
	db, err := database.Init(":memory:")
	require.NoError(t, err)
	defer db.Close()

	t.Setenv("VAPID_PUBLIC_KEY", "test-pub")
	t.Setenv("VAPID_PRIVATE_KEY", "test-priv")

	cfg := internal.LoadConfig()
	svcs := initServices(context.Background(), db, cfg)

	require.NotNil(t, svcs)
	assert.NotNil(t, svcs.pushHandler)
	assert.NotNil(t, svcs.chargeService)
}

func TestStartWorkers_NoPanic(t *testing.T) {
	db, err := database.Init(":memory:")
	require.NoError(t, err)
	defer db.Close()

	cfg := internal.LoadConfig()
	svcs := initServices(context.Background(), db, cfg)

	ctx, cancel := context.WithCancel(t.Context())

	var wg sync.WaitGroup
	startWorkers(ctx, &wg, svcs)

	time.Sleep(100 * time.Millisecond)
	cancel()
	wg.Wait()
}

func TestRegisterRoutes(t *testing.T) {
	db, err := database.Init(":memory:")
	require.NoError(t, err)
	defer db.Close()

	cfg := internal.LoadConfig()
	svcs := initServices(context.Background(), db, cfg)

	mux := http.NewServeMux()
	identityMW := func(h http.Handler) http.Handler { return h }
	registerRoutes(mux, db, svcs, identityMW)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestRegisterRoutes_WithPush(t *testing.T) {
	db, err := database.Init(":memory:")
	require.NoError(t, err)
	defer db.Close()

	t.Setenv("VAPID_PUBLIC_KEY", "test-pub")
	t.Setenv("VAPID_PRIVATE_KEY", "test-priv")

	cfg := internal.LoadConfig()
	svcs := initServices(context.Background(), db, cfg)
	require.NotNil(t, svcs.pushHandler)

	mux := http.NewServeMux()
	identityMW := func(h http.Handler) http.Handler { return h }
	registerRoutes(mux, db, svcs, identityMW)

	req := httptest.NewRequest(http.MethodPost, "/api/push-subscriptions", strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	assert.NotEqual(t, http.StatusNotFound, w.Code)
}

func TestWaitForShutdownSignal(t *testing.T) {
	sigChan := waitForShutdownSignal()

	// Verify channel is created and non-nil
	assert.NotNil(t, sigChan)

	// The channel should be buffered with capacity 1
	assert.Equal(t, 1, cap(sigChan))
}

func TestNewServer_Timeouts(t *testing.T) {
	handler := http.NewServeMux()
	server := NewServer(":8080", handler)

	require.NotNil(t, server)
	assert.Equal(t, ":8080", server.Addr)
	assert.Equal(t, 10*time.Second, server.ReadTimeout)
	assert.Equal(t, 30*time.Second, server.WriteTimeout)
	assert.Equal(t, 120*time.Second, server.IdleTimeout)
}

func TestNewServer_HandlesRequests(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /test", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	server := NewServer(":0", mux)
	ts := httptest.NewServer(server.Handler)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/test")
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}
