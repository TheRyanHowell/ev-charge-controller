package testdb

import (
	"database/sql"
	"fmt"
	"testing"
)

const (
	defaultPlugName  = "Test"
	defaultNamespace = "default-ns"
	defaultMqttTopic = "test-topic"
	defaultUserEmail = "test@example.com"
)

const (
	UserID1 = "test-user"
	UserID2 = "u1"
	UserID3 = "u2"
)

const (
	PlugID1 = "test-plug"
	PlugID2 = "plug-1"
	PlugID3 = "plug-2"
)

// SeedDefaultUser inserts the default test user (idempotent).
func SeedDefaultUser(t *testing.T, db *sql.DB) {
	t.Helper()
	if err := InsertUser(db, DefaultUserID, defaultUserEmail, ""); err != nil {
		t.Fatalf("testdb.SeedDefaultUser: insert user: %v", err)
	}
}

// SeedDefaultPlug inserts the default test plug under the default user (idempotent).
// Requires SeedDefaultUser to have been called first.
func SeedDefaultPlug(t *testing.T, db *sql.DB) {
	t.Helper()
	if err := InsertPlug(db, DefaultPlugID, DefaultUserID, defaultPlugName, defaultNamespace, defaultMqttTopic); err != nil {
		t.Fatalf("testdb.SeedDefaultPlug: insert plug: %v", err)
	}
}

// SeedDefaultVehicles inserts rm1, rm1s, rm2 vehicle instances under the default user (idempotent).
// Requires SeedDefaultUser to have been called first.
func SeedDefaultVehicles(t *testing.T, db *sql.DB) {
	t.Helper()
	for _, modelID := range []string{"rm1", "rm1s", "rm2"} {
		name := fmt.Sprintf("Maeving %s", modelID)
		if err := InsertVehicle(db, modelID, DefaultUserID, modelID, name, 20, 80); err != nil {
			t.Fatalf("testdb.SeedDefaultVehicles: insert vehicle %s: %v", modelID, err)
		}
	}
}

// SeedFullTestDB seeds the default user, plug, and vehicles in one call.
func SeedFullTestDB(t *testing.T, db *sql.DB) {
	t.Helper()
	SeedDefaultUser(t, db)
	SeedDefaultPlug(t, db)
	SeedDefaultVehicles(t, db)
}

// SeedMultiUser seeds three users (test-user, u1, u2) each with a plug and vehicles.
func SeedMultiUser(t *testing.T, db *sql.DB) {
	t.Helper()
	seedUserWithPlugAndVehicles(t, db, UserID1, PlugID1, "test-plug-ns", "test-plug-topic")
	seedUserWithPlugAndVehicles(t, db, UserID2, PlugID2, "plug-ns-1", "plug-topic-1")
	seedUserWithPlugAndVehicles(t, db, UserID3, PlugID3, "plug-ns-2", "plug-topic-2")
}

func seedUserWithPlugAndVehicles(t *testing.T, db *sql.DB, userID, plugID, namespace, mqttTopic string) {
	t.Helper()
	email := userID + "@example.com"
	if err := InsertUser(db, userID, email, ""); err != nil {
		t.Fatalf("testdb.SeedMultiUser: insert user %s: %v", userID, err)
	}
	if err := InsertPlug(db, plugID, userID, "Plug "+userID, namespace, mqttTopic); err != nil {
		t.Fatalf("testdb.SeedMultiUser: insert plug %s: %v", plugID, err)
	}
	for _, modelID := range []string{"rm1", "rm1s", "rm2"} {
		name := fmt.Sprintf("Maeving %s (%s)", modelID, userID)
		vid := modelID + "-" + userID
		if err := InsertVehicle(db, vid, userID, modelID, name, 20, 80); err != nil {
			t.Fatalf("testdb.SeedMultiUser: insert vehicle %s: %v", vid, err)
		}
	}
}
