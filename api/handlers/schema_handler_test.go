package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"ev-charge-controller/api/models"
)

func TestSchemaHandler_Get(t *testing.T) {
	handler := NewSchemaHandler()
	req := httptest.NewRequest(http.MethodGet, "/api/schemas", nil)
	w := httptest.NewRecorder()

	handler.Get(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	var resp SchemaResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	// Verify all expected schemas are present
	expectedSchemas := []string{"Vehicle", "ChargeSession", "ChargeSessionView", "Schedule", "PowerReading", "SOCSnapshot"}
	for _, name := range expectedSchemas {
		if _, ok := resp.Schemas[name]; !ok {
			t.Errorf("missing schema: %s", name)
		}
	}

	// Verify Vehicle schema has expected fields
	vehicleSchema := resp.Schemas["Vehicle"]
	if vehicleSchema.Name != "Vehicle" {
		t.Errorf("expected schema name 'Vehicle', got %q", vehicleSchema.Name)
	}

	// Check for specific Vehicle fields
	fieldNames := make(map[string]bool)
	for _, field := range vehicleSchema.Fields {
		fieldNames[field.Name] = true
	}

	expectedFields := []string{"ID", "Name", "CapacityKwh", "ChargerOutputW", "ChargingEfficiency"}
	for _, fieldName := range expectedFields {
		if !fieldNames[fieldName] {
			t.Errorf("missing field in Vehicle schema: %s", fieldName)
		}
	}

	// Verify ChargeSession schema
	chargeSessionSchema := resp.Schemas["ChargeSession"]
	chargeSessionFields := make(map[string]FieldDef)
	for _, field := range chargeSessionSchema.Fields {
		chargeSessionFields[field.Name] = field
	}

	// EndKwh should be nullable
	if endKwhField, ok := chargeSessionFields["EndKwh"]; ok {
		if !endKwhField.Nullable {
			t.Error("EndKwh should be marked as nullable")
		}
		if !endKwhField.Optional {
			t.Error("EndKwh should be marked as optional")
		}
	} else {
		t.Error("missing EndKwh field in ChargeSession schema")
	}
}

func TestExtractSchema(t *testing.T) {
	handler := NewSchemaHandler()
	schema := handler.extractSchema((*models.Vehicle)(nil))

	if schema.Name != "Vehicle" {
		t.Errorf("expected name 'Vehicle', got %q", schema.Name)
	}

	if len(schema.Fields) == 0 {
		t.Error("expected fields to be extracted")
	}

	// Check that fields have types
	for _, field := range schema.Fields {
		if field.Type == "" {
			t.Errorf("field %s has empty type", field.Name)
		}
		if field.Name == "" {
			t.Error("found field with empty name")
		}
	}
}

func TestTypeString(t *testing.T) {
	handler := NewSchemaHandler()

	tests := []struct {
		name     string
		ptr      interface{}
		expected string
	}{
		{"Vehicle struct", (*models.Vehicle)(nil), "Vehicle"},
		{"Schedule struct", (*models.Schedule)(nil), "Schedule"},
		{"ChargeSession struct", (*models.ChargeSession)(nil), "ChargeSession"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			schema := handler.extractSchema(tt.ptr)
			if schema.Name != tt.expected {
				t.Errorf("expected name %q, got %q", tt.expected, schema.Name)
			}
		})
	}
}
