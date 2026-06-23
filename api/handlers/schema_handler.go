package handlers

import (
	"log/slog"
	"net/http"
	"reflect"

	"ev-charge-controller/api/models"
)

// SchemaHandler exports API response schemas for consumption by frontend tools.
type SchemaHandler struct{}

// FieldDef describes a single struct field for schema generation.
type FieldDef struct {
	Name     string      `json:"name"`
	Type     string      `json:"type"`
	Optional bool        `json:"optional"`
	Nullable bool        `json:"nullable"`
	Tags     map[string]string `json:"tags"`
}

// TypeSchema describes a complete Go type (e.g., Vehicle, ChargeSession).
type TypeSchema struct {
	Name   string     `json:"name"`
	Fields []FieldDef `json:"fields"`
}

// SchemaResponse is the root response for /api/schemas.
type SchemaResponse struct {
	Schemas map[string]TypeSchema `json:"schemas"`
}

// NewSchemaHandler creates a new schema handler.
func NewSchemaHandler() *SchemaHandler {
	return &SchemaHandler{}
}

// Get returns all API schemas.
func (h *SchemaHandler) Get(w http.ResponseWriter, r *http.Request) {
	schemas := SchemaResponse{
		Schemas: make(map[string]TypeSchema),
	}

	// Export all model types
	schemas.Schemas["Vehicle"] = h.extractSchema((*models.Vehicle)(nil))
	schemas.Schemas["ChargeSession"] = h.extractSchema((*models.ChargeSession)(nil))
	schemas.Schemas["ChargeSessionView"] = h.extractSchema((*models.ChargeSessionView)(nil))
	schemas.Schemas["Schedule"] = h.extractSchema((*models.Schedule)(nil))
	schemas.Schemas["PowerReading"] = h.extractSchema((*models.PowerReading)(nil))
	schemas.Schemas["SOCSnapshot"] = h.extractSchema((*models.SOCSnapshot)(nil))

	if err := writeJSON(w, http.StatusOK, schemas); err != nil {
		slog.Error("error encoding schemas", "err", err)
	}
}

// extractSchema uses reflection to extract struct definition as TypeSchema.
func (h *SchemaHandler) extractSchema(ptr interface{}) TypeSchema {
	if ptr == nil {
		return TypeSchema{}
	}

	t := reflect.TypeOf(ptr)
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}

	schema := TypeSchema{
		Name:   t.Name(),
		Fields: make([]FieldDef, 0),
	}

	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		if !field.IsExported() {
			continue // Skip private fields
		}

		fieldDef := FieldDef{
			Name: field.Name,
			Type: h.typeString(field.Type),
			Tags: extractJSONTag(field),
		}

		// Determine if field is optional (pointer or struct tag)
		if field.Type.Kind() == reflect.Ptr {
			fieldDef.Optional = true
			fieldDef.Nullable = true
		}

		// Check for explicit nullable/optional in JSON tag
		jsonTag := field.Tag.Get("json")
		if jsonTag != "" && jsonTag != "-" {
			fieldDef.Tags["json"] = jsonTag
			if jsonTag[len(jsonTag)-1:] == "?" {
				fieldDef.Optional = true
			}
		}

		schema.Fields = append(schema.Fields, fieldDef)
	}

	return schema
}

// typeString returns a human-readable type name for reflection types.
func (h *SchemaHandler) typeString(t reflect.Type) string {
	if t.Kind() == reflect.Ptr {
		return "*" + h.typeString(t.Elem())
	}

	switch t.Kind() {
	case reflect.String:
		return "string"
	case reflect.Float64, reflect.Float32:
		return "float64"
	case reflect.Int, reflect.Int64, reflect.Int32:
		return "int"
	case reflect.Bool:
		return "bool"
	case reflect.Struct:
		if t.PkgPath() == "time" && t.Name() == "Time" {
			return "time.Time"
		}
		return t.Name()
	default:
		return t.Kind().String()
	}
}

// extractJSONTag parses the JSON struct tag and returns key-value pairs.
func extractJSONTag(field reflect.StructField) map[string]string {
	tags := make(map[string]string)
	jsonTag := field.Tag.Get("json")
	if jsonTag != "" && jsonTag != "-" {
		tags["json"] = jsonTag
	}
	return tags
}
