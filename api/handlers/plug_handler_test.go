package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"ev-charge-controller/api/internal"
	"ev-charge-controller/api/models"
	"ev-charge-controller/api/repository"
	"ev-charge-controller/api/services"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	testPlugHandlerUserID = "test-user-plug-handler"
)

func setupPlugHandlerTest(t *testing.T) (*PlugHandler, string) {
	t.Helper()
	db := setupHandlerTestDB(t)
	t.Cleanup(func() { db.Close() })

	_, err := db.Exec(`INSERT OR IGNORE INTO users (id, email, password_hash) VALUES (?, ?, ?)`,
		testPlugHandlerUserID, "plug-handler-test@example.com", "")
	require.NoError(t, err)

	plugRepo := repository.NewPlugRepository(db)
	svc := services.NewMqttProvisioningService(plugRepo, nil, &internal.Config{MQTTExternalIP: "test.local", MQTTExternalPort: "1883"})
	handler := NewPlugHandler(svc)
	return handler, testPlugHandlerUserID
}

func plugHandlerReqWithUser(method, path string, body []byte, userID string) *http.Request {
	var req *http.Request
	if body != nil {
		req = httptest.NewRequest(method, path, bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
	} else {
		req = httptest.NewRequest(method, path, nil)
	}
	return req.WithContext(internal.WithUserID(req.Context(), userID))
}

func TestPlugHandler_List_Empty(t *testing.T) {
	handler, userID := setupPlugHandlerTest(t)

	req := plugHandlerReqWithUser(http.MethodGet, "/api/plugs", nil, userID)
	rr := httptest.NewRecorder()
	handler.List(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	var plugs []models.Plug
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&plugs))
	assert.Empty(t, plugs)
}

func TestPlugHandler_Create(t *testing.T) {
	handler, userID := setupPlugHandlerTest(t)

	body, _ := json.Marshal(map[string]string{"name": "Living Room", "mqttTopic": "my-plug"})
	req := plugHandlerReqWithUser(http.MethodPost, "/api/plugs", body, userID)
	rr := httptest.NewRecorder()
	handler.Create(rr, req)

	assert.Equal(t, http.StatusCreated, rr.Code)
	var resp struct{ Plug models.Plug `json:"plug"` }
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&resp))
	assert.Equal(t, "Living Room", resp.Plug.Name)
	assert.NotEmpty(t, resp.Plug.Namespace, "namespace should be generated at creation")
	assert.True(t, strings.HasPrefix(resp.Plug.Namespace, "ns-"), "namespace=%s", resp.Plug.Namespace)
	assert.Len(t, resp.Plug.MqttTopic, 8, "topic should be random hex")
}

func TestPlugHandler_Create_MissingName(t *testing.T) {
	handler, userID := setupPlugHandlerTest(t)

	body, _ := json.Marshal(map[string]string{"mqttTopic": "my-plug"})
	req := plugHandlerReqWithUser(http.MethodPost, "/api/plugs", body, userID)
	rr := httptest.NewRecorder()
	handler.Create(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestPlugHandler_Create_NoAuth(t *testing.T) {
	handler, _ := setupPlugHandlerTest(t)

	body, _ := json.Marshal(map[string]string{"name": "plug", "mqttTopic": "topic"})
	req := httptest.NewRequest(http.MethodPost, "/api/plugs", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	handler.Create(rr, req)

	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}

func TestPlugHandler_Create_InvalidBody(t *testing.T) {
	handler, userID := setupPlugHandlerTest(t)

	req := plugHandlerReqWithUser(http.MethodPost, "/api/plugs", []byte("invalid"), userID)
	rr := httptest.NewRecorder()
	handler.Create(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestPlugHandler_Update(t *testing.T) {
	handler, userID := setupPlugHandlerTest(t)

	// Create a plug first
	body, _ := json.Marshal(map[string]string{"name": "Old Name", "mqttTopic": "topic"})
	req := plugHandlerReqWithUser(http.MethodPost, "/api/plugs", body, userID)
	rr := httptest.NewRecorder()
	handler.Create(rr, req)
	require.Equal(t, http.StatusCreated, rr.Code)
	var resp struct{ Plug models.Plug `json:"plug"` }
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&resp))
	plugID := resp.Plug.ID

	// Update it
	newName := "New Name"
	updateBody, _ := json.Marshal(map[string]string{"name": newName})
	req = plugHandlerReqWithUser(http.MethodPatch, "/api/plugs/"+plugID, updateBody, userID)
	req.SetPathValue("id", plugID)
	rr = httptest.NewRecorder()
	handler.Update(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	var updated models.Plug
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&updated))
	assert.Equal(t, newName, updated.Name)
}

func TestPlugHandler_Update_NotFound(t *testing.T) {
	handler, userID := setupPlugHandlerTest(t)

	newName := "New Name"
	body, _ := json.Marshal(map[string]string{"name": newName})
	req := plugHandlerReqWithUser(http.MethodPatch, "/api/plugs/nonexistent", body, userID)
	req.SetPathValue("id", "nonexistent")
	rr := httptest.NewRecorder()
	handler.Update(rr, req)

	assert.Equal(t, http.StatusNotFound, rr.Code)
}

func TestPlugHandler_Update_InvalidID(t *testing.T) {
	handler, userID := setupPlugHandlerTest(t)

	body, _ := json.Marshal(map[string]string{"name": "name"})
	req := plugHandlerReqWithUser(http.MethodPatch, "/api/plugs/", body, userID)
	req.SetPathValue("id", "")
	rr := httptest.NewRecorder()
	handler.Update(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestPlugHandler_Delete(t *testing.T) {
	handler, userID := setupPlugHandlerTest(t)

	// Create a plug first
	body, _ := json.Marshal(map[string]string{"name": "To Delete", "mqttTopic": "del-topic"})
	req := plugHandlerReqWithUser(http.MethodPost, "/api/plugs", body, userID)
	rr := httptest.NewRecorder()
	handler.Create(rr, req)
	require.Equal(t, http.StatusCreated, rr.Code)
	var resp struct{ Plug models.Plug `json:"plug"` }
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&resp))
	plugID := resp.Plug.ID

	// Delete it
	req = plugHandlerReqWithUser(http.MethodDelete, "/api/plugs/"+plugID, nil, userID)
	req.SetPathValue("id", plugID)
	rr = httptest.NewRecorder()
	handler.Delete(rr, req)

	assert.Equal(t, http.StatusNoContent, rr.Code)
}

func TestPlugHandler_Delete_NotFound(t *testing.T) {
	handler, userID := setupPlugHandlerTest(t)

	req := plugHandlerReqWithUser(http.MethodDelete, "/api/plugs/nonexistent", nil, userID)
	req.SetPathValue("id", "nonexistent")
	rr := httptest.NewRecorder()
	handler.Delete(rr, req)

	assert.Equal(t, http.StatusNotFound, rr.Code)
}

func TestPlugHandler_List_AfterCreate(t *testing.T) {
	handler, userID := setupPlugHandlerTest(t)

	// Create two plugs
	for i, name := range []string{"Plug A", "Plug B"} {
		topic := []string{"plug-a", "plug-b"}[i]
		body, _ := json.Marshal(map[string]string{"name": name, "mqttTopic": topic})
		req := plugHandlerReqWithUser(http.MethodPost, "/api/plugs", body, userID)
		rr := httptest.NewRecorder()
		handler.Create(rr, req)
		require.Equal(t, http.StatusCreated, rr.Code)
	}

	// List
	req := plugHandlerReqWithUser(http.MethodGet, "/api/plugs", nil, userID)
	rr := httptest.NewRecorder()
	handler.List(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	var plugs []models.Plug
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&plugs))
	assert.Len(t, plugs, 2)
}

func TestPlugHandler_GetByID(t *testing.T) {
	handler, userID := setupPlugHandlerTest(t)

	body, _ := json.Marshal(map[string]string{"name": "Garage", "mqttTopic": "garage"})
	req := plugHandlerReqWithUser(http.MethodPost, "/api/plugs", body, userID)
	rr := httptest.NewRecorder()
	handler.Create(rr, req)
	require.Equal(t, http.StatusCreated, rr.Code)
	var resp struct{ Plug models.Plug `json:"plug"` }
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&resp))
	plugID := resp.Plug.ID

	req = plugHandlerReqWithUser(http.MethodGet, "/api/plugs/"+plugID, nil, userID)
	req.SetPathValue("id", plugID)
	rr = httptest.NewRecorder()
	handler.GetByID(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	var got models.Plug
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&got))
	assert.Equal(t, "Garage", got.Name)
}

func TestPlugHandler_GetByID_NotFound(t *testing.T) {
	handler, userID := setupPlugHandlerTest(t)

	req := plugHandlerReqWithUser(http.MethodGet, "/api/plugs/nonexistent", nil, userID)
	req.SetPathValue("id", "nonexistent")
	rr := httptest.NewRecorder()
	handler.GetByID(rr, req)

	assert.Equal(t, http.StatusNotFound, rr.Code)
}

func TestPlugHandler_ConfigureDevice_ManualPath(t *testing.T) {
	handler, userID := setupPlugHandlerTest(t)

	// Create plug
	body, _ := json.Marshal(map[string]string{"name": "Kitchen", "mqttTopic": "kitchen"})
	req := plugHandlerReqWithUser(http.MethodPost, "/api/plugs", body, userID)
	rr := httptest.NewRecorder()
	handler.Create(rr, req)
	require.Equal(t, http.StatusCreated, rr.Code)
	var resp struct{ Plug models.Plug `json:"plug"` }
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&resp))
	plugID := resp.Plug.ID

	// Configure without tasmotaIP (manual path)
	req = plugHandlerReqWithUser(http.MethodPost, "/api/plugs/"+plugID+"/configure", []byte("{}"), userID)
	req.SetPathValue("id", plugID)
	rr = httptest.NewRecorder()
	handler.ConfigureDevice(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	var configureResp map[string]string
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&configureResp))
	assert.NotEmpty(t, configureResp["consoleCommands"])
	assert.Contains(t, configureResp["consoleCommands"], "Backlog")
}

func TestPlugHandler_ConfigureDevice_NotFound(t *testing.T) {
	handler, userID := setupPlugHandlerTest(t)

	req := plugHandlerReqWithUser(http.MethodPost, "/api/plugs/nonexistent/configure", []byte("{}"), userID)
	req.SetPathValue("id", "nonexistent")
	rr := httptest.NewRecorder()
	handler.ConfigureDevice(rr, req)

	assert.Equal(t, http.StatusNotFound, rr.Code)
}

func TestPlugHandler_List_Unauthorized(t *testing.T) {
	handler, _ := setupPlugHandlerTest(t)

	req := httptest.NewRequest(http.MethodGet, "/api/plugs", nil)
	rr := httptest.NewRecorder()
	handler.List(rr, req)

	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}

func TestPlugHandler_List_DBError(t *testing.T) {
	db := setupHandlerTestDB(t)

	_, err := db.Exec(`INSERT OR IGNORE INTO users (id, email, password_hash) VALUES (?, ?, ?)`,
		testPlugHandlerUserID, "plug-handler-test@example.com", "")
	require.NoError(t, err)

	plugRepo := repository.NewPlugRepository(db)
	svc := services.NewMqttProvisioningService(plugRepo, nil, &internal.Config{MQTTExternalIP: "test.local", MQTTExternalPort: "1883"})
	handler := NewPlugHandler(svc)

	db.Close()

	req := plugHandlerReqWithUser(http.MethodGet, "/api/plugs", nil, testPlugHandlerUserID)
	rr := httptest.NewRecorder()
	handler.List(rr, req)

	assert.Equal(t, http.StatusInternalServerError, rr.Code)
}

func TestPlugHandler_GetByID_InvalidID(t *testing.T) {
	handler, userID := setupPlugHandlerTest(t)

	req := plugHandlerReqWithUser(http.MethodGet, "/api/plugs/", nil, userID)
	req.SetPathValue("id", "")
	rr := httptest.NewRecorder()
	handler.GetByID(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestPlugHandler_GetByID_Unauthorized(t *testing.T) {
	handler, _ := setupPlugHandlerTest(t)

	req := httptest.NewRequest(http.MethodGet, "/api/plugs/test-plug", nil)
	req.SetPathValue("id", "test-plug")
	rr := httptest.NewRecorder()
	handler.GetByID(rr, req)

	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}

func TestPlugHandler_Delete_InvalidID(t *testing.T) {
	handler, userID := setupPlugHandlerTest(t)

	req := plugHandlerReqWithUser(http.MethodDelete, "/api/plugs/", nil, userID)
	req.SetPathValue("id", "")
	rr := httptest.NewRecorder()
	handler.Delete(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestPlugHandler_Delete_Unauthorized(t *testing.T) {
	handler, _ := setupPlugHandlerTest(t)

	req := httptest.NewRequest(http.MethodDelete, "/api/plugs/test-plug", nil)
	req.SetPathValue("id", "test-plug")
	rr := httptest.NewRecorder()
	handler.Delete(rr, req)

	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}

func TestPlugHandler_ConfigureDevice_InvalidID(t *testing.T) {
	handler, userID := setupPlugHandlerTest(t)

	req := plugHandlerReqWithUser(http.MethodPost, "/api/plugs//configure", []byte("{}"), userID)
	req.SetPathValue("id", "")
	rr := httptest.NewRecorder()
	handler.ConfigureDevice(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestPlugHandler_ConfigureDevice_Unauthorized(t *testing.T) {
	handler, _ := setupPlugHandlerTest(t)

	req := httptest.NewRequest(http.MethodPost, "/api/plugs/test-plug/configure", bytes.NewReader([]byte("{}")))
	req.Header.Set("Content-Type", "application/json")
	req.SetPathValue("id", "test-plug")
	rr := httptest.NewRecorder()
	handler.ConfigureDevice(rr, req)

	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}

func TestPlugHandler_Update_Unauthorized(t *testing.T) {
	handler, _ := setupPlugHandlerTest(t)

	body, _ := json.Marshal(map[string]string{"name": "New Name"})
	req := httptest.NewRequest(http.MethodPatch, "/api/plugs/test-plug", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.SetPathValue("id", "test-plug")
	rr := httptest.NewRecorder()
	handler.Update(rr, req)

	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}

func TestPlugHandler_Update_InvalidBody(t *testing.T) {
	handler, userID := setupPlugHandlerTest(t)

	req := plugHandlerReqWithUser(http.MethodPatch, "/api/plugs/test-plug", []byte("invalid"), userID)
	req.SetPathValue("id", "test-plug")
	rr := httptest.NewRecorder()
	handler.Update(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestPlugHandler_Create_ServiceError(t *testing.T) {
	db := setupHandlerTestDB(t)

	_, err := db.Exec(`INSERT OR IGNORE INTO users (id, email, password_hash) VALUES (?, ?, ?)`,
		testPlugHandlerUserID, "plug-handler-test@example.com", "")
	require.NoError(t, err)

	plugRepo := repository.NewPlugRepository(db)
	svc := services.NewMqttProvisioningService(plugRepo, nil, &internal.Config{MQTTExternalIP: "test.local", MQTTExternalPort: "1883"})
	handler := NewPlugHandler(svc)

	db.Close()

	body, _ := json.Marshal(map[string]string{"name": "Test Plug", "mqttTopic": "topic"})
	req := plugHandlerReqWithUser(http.MethodPost, "/api/plugs", body, testPlugHandlerUserID)
	rr := httptest.NewRecorder()
	handler.Create(rr, req)

	assert.Equal(t, http.StatusInternalServerError, rr.Code)
}

func TestPlugHandler_ConfigureDevice_InvalidBody(t *testing.T) {
	handler, userID := setupPlugHandlerTest(t)

	req := plugHandlerReqWithUser(http.MethodPost, "/api/plugs/test-plug/configure", []byte("invalid json"), userID)
	req.SetPathValue("id", "test-plug")
	rr := httptest.NewRecorder()
	handler.ConfigureDevice(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestPlugHandler_Create_WithType(t *testing.T) {
	handler, userID := setupPlugHandlerTest(t)

	body, _ := json.Marshal(map[string]string{"name": "Maint Plug", "type": "maintenance"})
	req := plugHandlerReqWithUser(http.MethodPost, "/api/plugs", body, userID)
	rr := httptest.NewRecorder()
	handler.Create(rr, req)

	assert.Equal(t, http.StatusCreated, rr.Code)
	var resp struct {
		Plug models.Plug `json:"plug"`
	}
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&resp))
	assert.Equal(t, "maintenance", resp.Plug.Type)
}

func TestPlugHandler_Create_InvalidType(t *testing.T) {
	handler, userID := setupPlugHandlerTest(t)

	body, _ := json.Marshal(map[string]string{"name": "Plug", "type": "unknown"})
	req := plugHandlerReqWithUser(http.MethodPost, "/api/plugs", body, userID)
	rr := httptest.NewRecorder()
	handler.Create(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestPlugHandler_TogglePower_NoMQTT(t *testing.T) {
	handler, userID := setupPlugHandlerTest(t)

	// Create a maintenance plug first.
	createBody, _ := json.Marshal(map[string]string{"name": "12V", "type": "maintenance"})
	createReq := plugHandlerReqWithUser(http.MethodPost, "/api/plugs", createBody, userID)
	createRR := httptest.NewRecorder()
	handler.Create(createRR, createReq)
	require.Equal(t, http.StatusCreated, createRR.Code)
	var createResp struct {
		Plug models.Plug `json:"plug"`
	}
	require.NoError(t, json.NewDecoder(createRR.Body).Decode(&createResp))

	// Toggle without MQTT wired in returns 503.
	toggleBody, _ := json.Marshal(map[string]bool{"on": true})
	req := plugHandlerReqWithUser(http.MethodPatch, "/api/plugs/"+createResp.Plug.ID+"/power", toggleBody, userID)
	req.SetPathValue("id", createResp.Plug.ID)
	rr := httptest.NewRecorder()
	handler.TogglePower(rr, req)

	assert.Equal(t, http.StatusServiceUnavailable, rr.Code)
}

func TestPlugHandler_TogglePower_ChargingPlugRejected(t *testing.T) {
	handler, userID := setupPlugHandlerTest(t)

	// Create a charging plug (default type).
	createBody, _ := json.Marshal(map[string]string{"name": "EV Charger"})
	createReq := plugHandlerReqWithUser(http.MethodPost, "/api/plugs", createBody, userID)
	createRR := httptest.NewRecorder()
	handler.Create(createRR, createReq)
	require.Equal(t, http.StatusCreated, createRR.Code)
	var createResp struct {
		Plug models.Plug `json:"plug"`
	}
	require.NoError(t, json.NewDecoder(createRR.Body).Decode(&createResp))

	// Toggle should return 400 for charging plugs.
	toggleBody, _ := json.Marshal(map[string]bool{"on": true})
	req := plugHandlerReqWithUser(http.MethodPatch, "/api/plugs/"+createResp.Plug.ID+"/power", toggleBody, userID)
	req.SetPathValue("id", createResp.Plug.ID)
	rr := httptest.NewRecorder()
	handler.TogglePower(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}
