package database

import (
	"database/sql"
	"embed"
	"fmt"
	"io/fs"
	"log/slog"
	"strings"

	migrate "github.com/golang-migrate/migrate/v4"
	sqlite3driver "github.com/golang-migrate/migrate/v4/database/sqlite3"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"github.com/google/uuid"
	_ "github.com/mattn/go-sqlite3"
)

//go:embed migrations/*.sql
var migrationFS embed.FS

//go:embed seed.sql
var seedSQL string

// Open creates a new SQLite database connection.
func Open(dbPath string) (*sql.DB, error) {
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, err
	}

	// SQLite (both file-based and in-memory) must use a single connection.
	// File-based: prevents "database is locked" from concurrent writers.
	// In-memory: required to persist schema across connections.
	db.SetMaxOpenConns(1)

	return db, nil
}

// ApplyPragmas configures SQLite pragmas (WAL mode, foreign keys).
func ApplyPragmas(db *sql.DB) error {
	// Enable WAL mode for better concurrent access
	if _, werr := db.Exec("PRAGMA journal_mode=WAL"); werr != nil {
		slog.Warn("failed to enable WAL mode", "error", werr)
	}
	if _, err := db.Exec("PRAGMA foreign_keys=ON"); err != nil {
		return fmt.Errorf("failed to enable foreign keys: %w", err)
	}
	return nil
}

// RunMigrations applies all pending schema migrations to the database.
func RunMigrations(db *sql.DB) error {
	subFS, err := fs.Sub(migrationFS, "migrations")
	if err != nil {
		return fmt.Errorf("failed to access migrations: %w", err)
	}

	src, err := iofs.New(subFS, ".")
	if err != nil {
		return fmt.Errorf("failed to create migration source: %w", err)
	}

	driver, err := sqlite3driver.WithInstance(db, &sqlite3driver.Config{
		MigrationsTable: "schema_migrations",
	})
	if err != nil {
		return fmt.Errorf("failed to create migrate driver: %w", err)
	}

	m, err := migrate.NewWithInstance("iofs", src, "sqlite3", driver)
	if err != nil {
		return fmt.Errorf("failed to create migrator: %w", err)
	}

	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		return fmt.Errorf("migration failed: %w", err)
	}

	return nil
}

// Seed inserts default vehicles into the database.
func Seed(db *sql.DB) error {
	if _, err := db.Exec(seedSQL); err != nil {
		return fmt.Errorf("failed to seed database: %w", err)
	}
	return nil
}

// Init opens the database, applies pragmas, runs migrations, and seeds the
// vehicle catalog. Seeding uses INSERT OR IGNORE, so re-running on an existing
// database only backfills catalog models added since the first seed.
func Init(dbPath string) (*sql.DB, error) {
	db, err := Open(dbPath)
	if err != nil {
		return nil, err
	}

	if err := ApplyPragmas(db); err != nil {
		db.Close()
		return nil, err
	}

	if err := RunMigrations(db); err != nil {
		db.Close()
		return nil, err
	}

	if err := Seed(db); err != nil {
		db.Close()
		return nil, err
	}

	return db, nil
}

// BackfillBootstrap seeds the bootstrap user and default plug when no users exist,
// then back-fills user_id/plug_id FK columns on all pre-existing rows.
// Safe to call multiple times; no-op if users table is non-empty.
func BackfillBootstrap(db *sql.DB, bootstrapEmail string) error {
	if bootstrapEmail == "" {
		bootstrapEmail = "admin@localhost"
	}

	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("BackfillBootstrap begin: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck

	var userCount int
	if err := tx.QueryRow("SELECT COUNT(*) FROM users").Scan(&userCount); err != nil {
		return fmt.Errorf("BackfillBootstrap count users: %w", err)
	}
	if userCount > 0 {
		return nil
	}

	userID := uuid.New().String()
	if _, err := tx.Exec(
		`INSERT INTO users (id, email, password_hash, created_at) VALUES (?, ?, '', CURRENT_TIMESTAMP)`,
		userID, bootstrapEmail,
	); err != nil {
		return fmt.Errorf("BackfillBootstrap insert user: %w", err)
	}

	namespace := strings.ReplaceAll(uuid.New().String(), "-", "")[:8]
	plugID := uuid.New().String()
	if _, err := tx.Exec(
		`INSERT INTO plugs (id, user_id, name, namespace, created_at) VALUES (?, ?, 'Default Plug', ?, CURRENT_TIMESTAMP)`,
		plugID, userID, namespace,
	); err != nil {
		return fmt.Errorf("BackfillBootstrap insert plug: %w", err)
	}

	// Create one vehicle instance per catalog model for the bootstrap admin.
	modelRows, err := tx.Query(`SELECT id, name, capacity_kwh FROM vehicle_models ORDER BY id`)
	if err != nil {
		return fmt.Errorf("BackfillBootstrap list models: %w", err)
	}
	type modelRow struct {
		id, name    string
		capacityKwh float64
	}
	var catalogModels []modelRow
	for modelRows.Next() {
		var m modelRow
		if err := modelRows.Scan(&m.id, &m.name, &m.capacityKwh); err != nil {
			modelRows.Close()
			return fmt.Errorf("BackfillBootstrap scan model: %w", err)
		}
		catalogModels = append(catalogModels, m)
	}
	modelRows.Close()
	if err := modelRows.Err(); err != nil {
		return fmt.Errorf("BackfillBootstrap models err: %w", err)
	}

	// The default plug is a charging plug, so it must be assigned to the first
	// battery-capable vehicle - the battery-less generic model sorts first.
	var firstVehicleID string
	for _, m := range catalogModels {
		vid := uuid.New().String()
		if firstVehicleID == "" && m.capacityKwh > 0 {
			firstVehicleID = vid
		}
		if _, err := tx.Exec(
			`INSERT INTO vehicles (id, user_id, model_id, name, current_percent, target_percent, created_at)
			 VALUES (?, ?, ?, ?, 20.0, 80.0, CURRENT_TIMESTAMP)`,
			vid, userID, m.id, m.name,
		); err != nil {
			return fmt.Errorf("BackfillBootstrap insert vehicle: %w", err)
		}
	}

	if firstVehicleID != "" {
		if _, err := tx.Exec(`UPDATE plugs SET vehicle_id = ? WHERE id = ?`, firstVehicleID, plugID); err != nil {
			return fmt.Errorf("BackfillBootstrap set plug vehicle: %w", err)
		}
	}

	updates := []struct {
		query string
		args  []any
	}{
		{`UPDATE push_subscriptions SET user_id = ?`, []any{userID}},
		{`UPDATE charge_sessions SET user_id = ?, plug_id = ?`, []any{userID, plugID}},
		{`UPDATE schedules SET user_id = ?, plug_id = ?`, []any{userID, plugID}},
	}
	for _, u := range updates {
		if _, err := tx.Exec(u.query, u.args...); err != nil {
			return fmt.Errorf("BackfillBootstrap back-fill: %w", err)
		}
	}

	slog.Info("Bootstrap back-fill complete", "email", bootstrapEmail, "plugNamespace", namespace)
	return tx.Commit()
}

// SetupTestDB creates an in-memory database with the schema.
// When withSeed is true, seed data is also inserted.
// Use this in tests instead of inline CREATE TABLE statements.
func SetupTestDB(withSeed bool) (*sql.DB, error) {
	db, err := Open(":memory:")
	if err != nil {
		return nil, err
	}

	if err := ApplyPragmas(db); err != nil {
		db.Close()
		return nil, err
	}

	if err := RunMigrations(db); err != nil {
		db.Close()
		return nil, err
	}

	if withSeed {
		if err := Seed(db); err != nil {
			db.Close()
			return nil, err
		}
	}

	return db, nil
}
