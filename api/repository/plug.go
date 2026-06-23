package repository

import (
	"context"
	"database/sql"
	"errors"

	"ev-charge-controller/api/models"

	"github.com/google/uuid"
)

type PlugRepository struct {
	db *sql.DB
}

func NewPlugRepository(db *sql.DB) *PlugRepository {
	return &PlugRepository{db: db}
}

func (r *PlugRepository) Create(ctx context.Context, plug *models.Plug) error {
	plug.ID = uuid.New().String()
	tls := 0
	if plug.TLS {
		tls = 1
	}
	online := 0
	if plug.Online {
		online = 1
	}
	plugType := plug.Type
	if plugType == "" {
		plugType = models.PlugTypeCharging
	}
	powerOn := 0
	if plug.PowerOn {
		powerOn = 1
	}
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO plugs (id, user_id, name, namespace, mqtt_topic, tls, online, type, power_on, vehicle_id, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP)`,
		plug.ID, plug.UserID, plug.Name, plug.Namespace, plug.MqttTopic, tls, online, plugType, powerOn,
		toNullString(plug.VehicleID),
	)
	return err
}

func (r *PlugRepository) FindByID(ctx context.Context, id string) (*models.Plug, error) {
	var p models.Plug
	var tls, online, initialized, powerOn int
	err := r.db.QueryRowContext(ctx,
		`SELECT id, user_id, name, namespace, mqtt_topic, tls, online, initialized, type, power_on, last_seen,
		        last_offline_notified_at, vehicle_id, created_at
		 FROM plugs WHERE id = ?`, id,
	).Scan(
		&p.ID, &p.UserID, &p.Name, &p.Namespace, &p.MqttTopic, &tls, &online, &initialized, &p.Type, &powerOn,
		newNullTime(&p.LastSeen), newNullTime(&p.LastOfflineNotifiedAt),
		newNullString(&p.VehicleID), &p.CreatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	p.TLS = tls == 1
	p.Online = online == 1
	p.Initialized = initialized == 1
	p.PowerOn = powerOn == 1
	return &p, nil
}

// NamespaceAndSlug returns the namespace and mqtt_topic for a plug by ID.
// Satisfies the mqtt.plugLookup interface.
func (r *PlugRepository) NamespaceAndSlug(ctx context.Context, plugID string) (namespace, slug string, err error) {
	plug, err := r.FindByID(ctx, plugID)
	if err != nil {
		return "", "", err
	}
	if plug == nil {
		return "", "", nil
	}
	return plug.Namespace, plug.MqttTopic, nil
}

func (r *PlugRepository) FindByNamespaceAndSlug(ctx context.Context, namespace, slug string) (*models.Plug, error) {
	var p models.Plug
	var tls, online, initialized, powerOn int
	err := r.db.QueryRowContext(ctx,
		`SELECT id, user_id, name, namespace, mqtt_topic, tls, online, initialized, type, power_on, last_seen,
		        last_offline_notified_at, vehicle_id, created_at
		 FROM plugs WHERE namespace = ? AND mqtt_topic = ?`, namespace, slug,
	).Scan(
		&p.ID, &p.UserID, &p.Name, &p.Namespace, &p.MqttTopic, &tls, &online, &initialized, &p.Type, &powerOn,
		newNullTime(&p.LastSeen), newNullTime(&p.LastOfflineNotifiedAt),
		newNullString(&p.VehicleID), &p.CreatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	p.TLS = tls == 1
	p.Online = online == 1
	p.Initialized = initialized == 1
	p.PowerOn = powerOn == 1
	return &p, nil
}

func (r *PlugRepository) List(ctx context.Context, userID string) ([]models.Plug, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, user_id, name, namespace, mqtt_topic, tls, online, initialized, type, power_on, last_seen,
		        last_offline_notified_at, vehicle_id, created_at
		 FROM plugs WHERE user_id = ? ORDER BY created_at`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var plugs []models.Plug
	for rows.Next() {
		var p models.Plug
		var tls, online, initialized, powerOn int
		if err := rows.Scan(
			&p.ID, &p.UserID, &p.Name, &p.Namespace, &p.MqttTopic, &tls, &online, &initialized, &p.Type, &powerOn,
			newNullTime(&p.LastSeen), newNullTime(&p.LastOfflineNotifiedAt),
			newNullString(&p.VehicleID), &p.CreatedAt,
		); err != nil {
			return nil, err
		}
		p.TLS = tls == 1
		p.Online = online == 1
		p.Initialized = initialized == 1
		p.PowerOn = powerOn == 1
		plugs = append(plugs, p)
	}
	return plugs, rows.Err()
}

func (r *PlugRepository) Update(ctx context.Context, plug *models.Plug) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE plugs SET name = ?, namespace = ?, vehicle_id = ? WHERE id = ? AND user_id = ?`,
		plug.Name, plug.Namespace, toNullString(plug.VehicleID), plug.ID, plug.UserID,
	)
	return err
}

func (r *PlugRepository) Delete(ctx context.Context, id, userID string) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM plugs WHERE id = ? AND user_id = ?`, id, userID)
	return err
}

// SetInitialized marks a plug as initialized after first-time MQTT configuration.
func (r *PlugRepository) SetInitialized(ctx context.Context, plugID string) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE plugs SET initialized = 1 WHERE id = ?`, plugID,
	)
	return err
}

// SetOnline updates a plug's online status and last_seen timestamp.
func (r *PlugRepository) SetOnline(ctx context.Context, plugID string, online bool) error {
	onlineInt := 0
	if online {
		onlineInt = 1
	}
	_, err := r.db.ExecContext(ctx,
		`UPDATE plugs SET online = ?, last_seen = CURRENT_TIMESTAMP WHERE id = ?`,
		onlineInt, plugID,
	)
	return err
}

// UpdateLastOfflineNotifiedAt sets the last_offline_notified_at column to now.
func (r *PlugRepository) UpdateLastOfflineNotifiedAt(ctx context.Context, plugID string) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE plugs SET last_offline_notified_at = CURRENT_TIMESTAMP WHERE id = ?`, plugID,
	)
	return err
}

// SetPowerState updates the cached relay power state for a plug.
func (r *PlugRepository) SetPowerState(ctx context.Context, plugID string, on bool) error {
	powerOn := 0
	if on {
		powerOn = 1
	}
	_, err := r.db.ExecContext(ctx,
		`UPDATE plugs SET power_on = ? WHERE id = ?`, powerOn, plugID,
	)
	return err
}

func (r *PlugRepository) ListNamespacesByUserID(ctx context.Context, userID string) ([]string, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT namespace FROM plugs WHERE user_id = ? ORDER BY created_at`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ns []string
	for rows.Next() {
		var n string
		if err := rows.Scan(&n); err != nil {
			return nil, err
		}
		ns = append(ns, n)
	}
	return ns, rows.Err()
}
