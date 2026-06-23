package main

import (
	"context"
	"database/sql"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"ev-charge-controller/api/carbonintensity"
	"ev-charge-controller/api/chargeestimate"
	"ev-charge-controller/api/database"
	"ev-charge-controller/api/handlers"
	"ev-charge-controller/api/internal"
	"ev-charge-controller/api/internal/workers"
	"ev-charge-controller/api/middleware"
	mqttclient "ev-charge-controller/api/mqtt"
	"ev-charge-controller/api/repository"
	"ev-charge-controller/api/services"
	"ev-charge-controller/api/tasmota"
)

func main() {
	cfg := internal.LoadConfig()
	if err := cfg.Validate(); err != nil {
		slog.Error("Configuration validation failed", "err", err)
		os.Exit(1)
	}
	internal.InitLogger(internal.LoggerConfig{Level: slog.LevelWarn})
	if cfg.IsDev() {
		handlers.EnableDebugResponses()
	}

	db, err := database.Init(cfg.DBPath)
	if err != nil {
		slog.Error("Failed to initialize database", "err", err)
		os.Exit(1)
	}

	ctx, cancel := context.WithCancel(context.Background())
	// sysCtx carries a system-principal marker so repository helpers can
	// distinguish authorised cross-user background access from a handler that
	// accidentally bypassed auth middleware (see internal.WarnIfMissingPrincipal).
	sysCtx := internal.WithSystemContext(ctx)
	svcs := initServices(sysCtx, db, cfg)

	mqttClient, lwtManager, dynsecMgr, mqttCtrl := startMQTTClient(sysCtx, cfg, db, svcs.chargeService)
	if dynsecMgr != nil {
		svcs.mqttProvSvc.SetDynsec(dynsecMgr)
	}
	if mqttCtrl != nil {
		svcs.chargeService.SetPlugController(mqttCtrl)
		svcs.plugHandler.SetPlugController(mqttCtrl, repository.NewPlugRepository(db))
	}

	var wg sync.WaitGroup
	startWorkers(sysCtx, &wg, svcs)

	slog.Info("Server starting", "port", cfg.Port)

	mux := http.NewServeMux()
	authMW := middleware.RequireAuth(svcs.authService)
	registerRoutes(mux, db, svcs, authMW)

	bodyLimited := middleware.BodyLimitMiddleware(mux)
	corsHandler := middleware.CorsHandler(bodyLimited)
	requestIDHandler := middleware.RequestIDMiddleware(corsHandler)
	addr := ":" + cfg.Port
	server := NewServer(addr, requestIDHandler)

	go func() {
		sig := <-waitForShutdownSignal()
		slog.Info("Shutting down", "signal", sig)

		// 1. Stop accepting new HTTP connections, drain existing ones
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer shutdownCancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			slog.Error("HTTP server shutdown error", "err", err)
		}

		// 2. Cancel worker context so tickers stop
		cancel()

		// 3. Disconnect MQTT client and cancel pending LWT debounce timers
		if mqttClient != nil {
			mqttClient.Disconnect(context.Background())
		}
		if lwtManager != nil {
			lwtManager.CancelAll()
		}

		// 4. Wait for workers to finish current iteration
		wg.Wait()
		slog.Info("All workers stopped")

		// 5. Shut down service (drain SOC worker channel)
		svcs.chargeService.Shutdown()

		// 6. Close database
		if err := db.Close(); err != nil {
			slog.Error("Database close error", "err", err)
		}
		slog.Info("Server shut down complete")
	}()

	slog.Info("Server listening", "addr", addr)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		slog.Error("Server failed", "err", err)
		os.Exit(1)
	}
}

func NewServer(addr string, handler http.Handler) *http.Server {
	return &http.Server{
		Addr:           addr,
		Handler:        handler,
		ReadTimeout:    10 * time.Second,
		WriteTimeout:   30 * time.Second,
		IdleTimeout:    120 * time.Second,
		MaxHeaderBytes: middleware.MaxHeaderBytes,
	}
}

	type serverServices struct {
		authHandler        *handlers.AuthHandler
		mqttProvSvc        *services.MqttProvisioningService
		plugHandler        *handlers.PlugHandler
		vehicleHandler     *handlers.VehicleHandler
		vehicleStatsHdl    *handlers.VehicleStatsHandler
		chargeHandler      *handlers.ChargeSessionHandler
		scheduleHandler    *handlers.ScheduleHandler
		pushHandler        *handlers.PushHandler
		powerReadingsHdl   *handlers.PowerReadingsHandler
		socSnapshotsHdl    *handlers.SOCSnapshotsHandler
		historyHandler     *handlers.HistoryHandler
		carbonIntensityHdl *handlers.CarbonIntensityHandler
		schemaHandler      *handlers.SchemaHandler
		tariffHandler      *handlers.TariffHandler
		resetHandler       *handlers.ResetHandler
		chargeService      *services.ChargeSessionService
		scheduleService    *services.ScheduleService
		authService        *services.AuthService
	}

func initServices(ctx context.Context, db *sql.DB, cfg *internal.Config) *serverServices {
	chargeRepo := repository.NewChargeSessionRepository(db)
	vehicleRepo := repository.NewVehicleRepository(db)
	vehicleModelRepo := repository.NewVehicleModelRepository(db)
	scheduleRepo := repository.NewScheduleRepository(db)
	userRepo := repository.NewUserRepository(db)
	tokenRepo := repository.NewRefreshTokenRepository(db)
	plugRepo := repository.NewPlugRepository(db)

	var pushService *services.PushService
	var pushHandler *handlers.PushHandler
	if cfg.PushEnabled() {
		pushRepo := repository.NewPushSubscriptionRepository(db)
		pushService = services.NewPushService(pushRepo, cfg.VapidPublicKey, cfg.VapidPrivateKey, &http.Client{})
		pushHandler = handlers.NewPushHandler(pushService)
		slog.Info("Push notifications enabled")
	}

	tariffRepo := repository.NewTariffRepository(db)
	tariffService := services.NewTariffService(tariffRepo)

	carbonIntensityClient := carbonintensity.NewClient()
	// plugCtrl is nil until MQTT connects; wired in via chargeService.SetPlugController after startMQTTClient.
	chargeService := services.NewChargeSessionService(ctx, chargeRepo, vehicleRepo, plugRepo, nil, carbonIntensityClient, pushService)
	// Tariff provider lets session completion freeze a time-weighted cost on the session.
	chargeService.SetTariffProvider(tariffService)
	vehicleService := services.NewVehicleService(vehicleRepo, vehicleModelRepo, chargeRepo, chargeService.Locker())
	scheduleService := services.NewScheduleService(scheduleRepo, plugRepo, vehicleRepo, chargeService)
	scheduleService.SetCarbonAwareDeps(carbonIntensityClient, chargeestimate.EstimateMinutes, chargeService.Notifier())
	chartDataService := services.NewChartDataService(chargeRepo)
	historyService := services.NewHistoryService(chargeRepo, chargeRepo)
	authService := services.NewAuthService(userRepo, tokenRepo, cfg.JWTSecret)
	mqttProvSvc := services.NewMqttProvisioningService(plugRepo, nil, cfg)

	vehicleHandler := handlers.NewVehicleHandler(vehicleService)
	vehicleStatsSvc := services.NewVehicleStatsServiceWithRepos(vehicleRepo, chargeRepo)
	vehicleStatsHdl := handlers.NewVehicleStatsHandler(vehicleStatsSvc)
	chargeHandler := handlers.NewChargeSessionHandler(chargeService)
	scheduleHandler := handlers.NewScheduleHandler(scheduleService)
	powerReadingsHdl := handlers.NewPowerReadingsHandler(chartDataService)
	socSnapshotsHdl := handlers.NewSOCSnapshotsHandler(chartDataService)
	historyHandler := handlers.NewHistoryHandler(historyService)
	carbonIntensityHdl := handlers.NewCarbonIntensityHandler(carbonIntensityClient)
	schemaHandler := handlers.NewSchemaHandler()
	authHandler := handlers.NewAuthHandler(authService)
	plugHandler := handlers.NewPlugHandler(mqttProvSvc)
	tariffHandler := handlers.NewTariffHandler(tariffService)

	// Reset handler (dev only) - resets DB + mock-tasmota to seed state
	var resetHandler *handlers.ResetHandler
	if cfg.IsDev() {
		tasmotaURLs := []string{"http://mock-tasmota:8081", "http://mock-tasmota-2:8082", "http://mock-tasmota-3:8083"}
		seedSvc := services.NewSeedService(db, tasmotaURLs)
		resetHandler = handlers.NewResetHandler(seedSvc)
	}

	return &serverServices{
		authHandler:        authHandler,
		mqttProvSvc:        mqttProvSvc,
		plugHandler:        plugHandler,
		vehicleHandler:     vehicleHandler,
		vehicleStatsHdl:    vehicleStatsHdl,
		chargeHandler:      chargeHandler,
		scheduleHandler:    scheduleHandler,
		pushHandler:        pushHandler,
		powerReadingsHdl:   powerReadingsHdl,
		socSnapshotsHdl:    socSnapshotsHdl,
		historyHandler:     historyHandler,
		carbonIntensityHdl: carbonIntensityHdl,
		schemaHandler:      schemaHandler,
		tariffHandler:      tariffHandler,
		resetHandler:       resetHandler,
		chargeService:      chargeService,
		scheduleService:    scheduleService,
		authService:        authService,
	}
}

func startWorkers(ctx context.Context, wg *sync.WaitGroup, svcs *serverServices) {
	wg.Add(3)
	workers.RunWithRecovery(ctx, "energy-poller", func(ctx context.Context) {
		defer wg.Done()
		workers.NewEnergyPoller(svcs.chargeService).Start(ctx)
	})
	workers.RunWithRecovery(ctx, "auto-stop-checker", func(ctx context.Context) {
		defer wg.Done()
		workers.NewAutoStopChecker(svcs.chargeService).Start(ctx)
	})
	workers.RunWithRecovery(ctx, "schedule-activator", func(ctx context.Context) {
		defer wg.Done()
		workers.NewScheduleActivator(svcs.scheduleService).Start(ctx)
	})
}

func waitForShutdownSignal() chan os.Signal {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	return sigChan
}


// startMQTTClient creates and connects an MQTT client if a broker URL is configured.
// Returns (nil, nil, nil, nil) when MQTT_BROKER_URL is empty so callers must nil-check.
func startMQTTClient(ctx context.Context, cfg *internal.Config, db *sql.DB, chargeService *services.ChargeSessionService) (*mqttclient.Client, *mqttclient.LWTManager, *mqttclient.DynsecManager, internal.PlugController) {
	if cfg.MQTTBrokerURL == "" {
		slog.Warn("mqtt: MQTT_BROKER_URL not set - MQTT disabled")
		return nil, nil, nil, nil
	}
	plugRepo := repository.NewPlugRepository(db)
	chargeRepo := repository.NewChargeSessionRepository(db)
	plugCache := mqttclient.NewPlugCache(plugRepo)
	energyHandler := func(handlerCtx context.Context, plugID string, energy *tasmota.EnergyData) {
		chargeService.HandleSensorMessage(handlerCtx, plugID, energy)
	}
	lwtManager := mqttclient.NewLWTManager(plugRepo, chargeRepo, chargeService, chargeService, nil)
	dispatcher := mqttclient.NewDispatcher(plugCache, energyHandler, lwtManager, plugRepo)
	client, err := mqttclient.NewClient(ctx, mqttclient.ClientConfig{
		BrokerURL: cfg.MQTTBrokerURL,
		Username:  cfg.MQTTUsername,
		Password:  cfg.MQTTPassword,
	}, dispatcher)
	if err != nil {
		slog.Error("mqtt: failed to create client", "err", err)
		return nil, nil, nil, nil
	}

	dynsecMgr := mqttclient.NewDynsecManager(client.ConnectionManager())
	publisher := mqttclient.NewPublisher(client.ConnectionManager(), mqttclient.NewRepoPlugLookup(plugRepo))
	lwtManager.SetInitializer(services.NewPlugInitializerService(plugRepo, publisher))
	mqttCtrl := mqttclient.NewController(dispatcher, publisher)

	awaitCtx, awaitCancel := context.WithTimeout(ctx, 15*time.Second)
	defer awaitCancel()
	if err := client.AwaitConnection(awaitCtx); err != nil {
		slog.Warn("mqtt: timed out waiting for initial connection; dynsec setup skipped", "err", err)
	} else if cfg.MQTTUsername != "" {
		dynsecCtx, dynsecCancel := context.WithTimeout(ctx, 15*time.Second)
		if err := dynsecMgr.EnsureAPIAccess(dynsecCtx, cfg.MQTTUsername); err != nil {
			slog.Warn("mqtt: failed to ensure API dynsec access", "err", err)
		}
		dynsecCancel()
	}

	slog.Info("mqtt: client started", "broker", cfg.MQTTBrokerURL)
	return client, lwtManager, dynsecMgr, mqttCtrl
}

func registerRoutes(mux *http.ServeMux, db *sql.DB, svcs *serverServices, authMW func(http.Handler) http.Handler) {
	// protect wraps a handler with the auth middleware.
	protect := func(h http.HandlerFunc) http.HandlerFunc {
		return authMW(h).ServeHTTP
	}

	// Public - no authentication required.
	mux.HandleFunc("GET /health", handlers.HealthHandler(handlers.NewDBHealthChecker(db)))
	mux.HandleFunc("GET /api/schemas", svcs.schemaHandler.Get)
	mux.HandleFunc("POST /api/auth/register", svcs.authHandler.Register)
	mux.HandleFunc("POST /api/auth/login", svcs.authHandler.Login)
	mux.HandleFunc("POST /api/auth/refresh", svcs.authHandler.Refresh)
	mux.HandleFunc("POST /api/auth/logout", svcs.authHandler.Logout)

	// Protected - JWT required.
	mux.HandleFunc("GET /api/auth/me", protect(svcs.authHandler.Me))
	mux.HandleFunc("GET /api/vehicle-models", protect(svcs.vehicleHandler.ListModels))
	mux.HandleFunc("GET /api/vehicles", protect(svcs.vehicleHandler.List))
	mux.HandleFunc("POST /api/vehicles", protect(svcs.vehicleHandler.Create))
	mux.HandleFunc("GET /api/vehicles/{id}", protect(func(w http.ResponseWriter, r *http.Request) {
		svcs.vehicleHandler.GetByID(w, r, r.PathValue("id"))
	}))
	mux.HandleFunc("PATCH /api/vehicles/{id}", protect(func(w http.ResponseWriter, r *http.Request) {
		svcs.vehicleHandler.Patch(w, r, r.PathValue("id"))
	}))
	mux.HandleFunc("DELETE /api/vehicles/{id}", protect(svcs.vehicleHandler.Delete))
	mux.HandleFunc("GET /api/vehicles/{id}/stats", protect(func(w http.ResponseWriter, r *http.Request) {
		svcs.vehicleStatsHdl.GetStats(w, r, r.PathValue("id"))
	}))
	mux.HandleFunc("GET /api/vehicles/stats", protect(svcs.vehicleStatsHdl.GetAllStats))
	mux.HandleFunc("GET /api/tariff-settings", protect(svcs.tariffHandler.Get))
	mux.HandleFunc("PUT /api/tariff-settings", protect(svcs.tariffHandler.Put))
	mux.HandleFunc("POST /api/charge-sessions", protect(svcs.chargeHandler.Start))
	mux.HandleFunc("PATCH /api/charge-sessions", protect(svcs.chargeHandler.UpdateTarget))
	mux.HandleFunc("DELETE /api/charge-sessions/{id}", protect(func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		svcs.chargeHandler.Delete(w, r, id)
	}))
	mux.HandleFunc("GET /api/plugs", protect(svcs.plugHandler.List))
	mux.HandleFunc("POST /api/plugs", protect(svcs.plugHandler.Create))
	mux.HandleFunc("GET /api/plugs/{id}", protect(svcs.plugHandler.GetByID))
	mux.HandleFunc("PATCH /api/plugs/{id}", protect(svcs.plugHandler.Update))
	mux.HandleFunc("DELETE /api/plugs/{id}", protect(svcs.plugHandler.Delete))
	mux.HandleFunc("GET /api/plugs/{id}/schedule", protect(svcs.scheduleHandler.GetByPlug))
	mux.HandleFunc("PATCH /api/plugs/{id}/schedule", protect(svcs.scheduleHandler.UpsertByPlug))
	mux.HandleFunc("PATCH /api/plugs/{id}/power", protect(svcs.plugHandler.TogglePower))
	mux.HandleFunc("POST /api/plugs/{id}/configure", protect(svcs.plugHandler.ConfigureDevice))
	mux.HandleFunc("GET /api/charge-sessions", protect(svcs.chargeHandler.GetActive))
	mux.HandleFunc("GET /api/power-readings", protect(svcs.powerReadingsHdl.GetReadings))
	mux.HandleFunc("GET /api/soc-snapshots", protect(svcs.socSnapshotsHdl.GetSnapshots))
	mux.HandleFunc("GET /api/history", protect(svcs.historyHandler.Get))
	mux.HandleFunc("GET /api/carbon-intensity", protect(svcs.carbonIntensityHdl.GetCurrent))

	if svcs.pushHandler != nil {
		mux.HandleFunc("POST /api/push-subscriptions", protect(svcs.pushHandler.Subscribe))
		mux.HandleFunc("DELETE /api/push-subscriptions", protect(svcs.pushHandler.Unsubscribe))
	}

	// Dev-only: reset endpoint (unauthenticated, guarded by cfg.IsDev())
	if svcs.resetHandler != nil {
		mux.HandleFunc("POST /api/reset", svcs.resetHandler.Reset)
	}
}
