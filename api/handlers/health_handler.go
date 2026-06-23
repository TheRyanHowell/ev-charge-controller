package handlers

import (
	"context"
	"database/sql"
	"log/slog"
	"net/http"
)

type HealthChecker interface {
	Ping(ctx context.Context) error
}

type dbHealthChecker struct {
	db *sql.DB
}

func (d *dbHealthChecker) Ping(ctx context.Context) error {
	return d.db.PingContext(ctx)
}

// NewDBHealthChecker creates a HealthChecker that wraps an *sql.DB.
func NewDBHealthChecker(db *sql.DB) HealthChecker {
	return &dbHealthChecker{db: db}
}

// HealthHandler returns a health check response that verifies database connectivity.
func HealthHandler(checker HealthChecker) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := checker.Ping(r.Context()); err != nil {
			slog.Error("health check: database ping failed", append(logReq(r), "err", err)...)
			problemJSONDebug(w, http.StatusServiceUnavailable, "about:blank#db-unavailable",
				"Service Unavailable", "Database health check failed", err.Error())
			return
		}

		_ = writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	}
}
