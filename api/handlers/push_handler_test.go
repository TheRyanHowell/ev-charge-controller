package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"ev-charge-controller/api/database"
	"ev-charge-controller/api/internal"
	"ev-charge-controller/api/models"
	"ev-charge-controller/api/repository"
	"ev-charge-controller/api/services"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupPushHandlerTestDB(t *testing.T) *repository.PushSubscriptionRepository {
	db, err := database.Init(":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() })
	_, err = db.Exec(`INSERT OR IGNORE INTO users (id, email, password_hash) VALUES ('test-user', 'push@test.com', '')`)
	require.NoError(t, err)
	return repository.NewPushSubscriptionRepository(db)
}

type errorPushSubscriptionRepo struct {
	err error
}

func (e *errorPushSubscriptionRepo) Upsert(_ context.Context, _ *models.PushSubscription) error {
	return e.err
}

func (e *errorPushSubscriptionRepo) RemoveByEndpoint(_ context.Context, _ string) error {
	return e.err
}

func (e *errorPushSubscriptionRepo) GetAll(_ context.Context) ([]models.PushSubscription, error) {
	return nil, e.err
}

func TestPushHandler_Subscribe_Success(t *testing.T) {
	repo := setupPushHandlerTestDB(t)

	handler := NewPushHandler(services.NewPushService(repo, "", "", nil))

	reqBody := map[string]string{
		"endpoint":  "https://fcm.googleapis.com/fcm/send/test123",
		"p256dhKey": "key123",
		"authKey":   "auth456",
	}
	body, _ := json.Marshal(reqBody)

	req, _ := http.NewRequest(http.MethodPost, "/api/push-subscriptions", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(internal.WithUserID(req.Context(), "test-user"))
	rr := httptest.NewRecorder()

	handler.Subscribe(rr, req)

	assert.Equal(t, http.StatusCreated, rr.Code)

	var sub models.PushSubscription
	err := json.NewDecoder(rr.Body).Decode(&sub)
	require.NoError(t, err)
	assert.Equal(t, "https://fcm.googleapis.com/fcm/send/test123", sub.Endpoint)
}

func TestPushHandler_Subscribe_BadRequest(t *testing.T) {
	repo := setupPushHandlerTestDB(t)

	handler := NewPushHandler(services.NewPushService(repo, "", "", nil))

	reqBody := map[string]string{
		"endpoint": "https://fcm.googleapis.com/fcm/send/test",
	}
	body, _ := json.Marshal(reqBody)

	req, _ := http.NewRequest(http.MethodPost, "/api/push-subscriptions", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	handler.Subscribe(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestPushHandler_Subscribe_RejectsNonHTTPSEndpoint(t *testing.T) {
	repo := setupPushHandlerTestDB(t)
	handler := NewPushHandler(services.NewPushService(repo, "", "", nil))

	// SSRF guard: the server later POSTs to this URL, so non-https / internal
	// targets must be rejected.
	for _, endpoint := range []string{
		"http://fcm.googleapis.com/fcm/send/x", // not https
		"http://169.254.169.254/latest/meta",   // link-local metadata
		"file:///etc/passwd",                   // non-http scheme
		"not a url",                            // unparseable
	} {
		body, _ := json.Marshal(map[string]string{
			"endpoint": endpoint, "p256dhKey": "k", "authKey": "a",
		})
		req, _ := http.NewRequest(http.MethodPost, "/api/push-subscriptions", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()

		handler.Subscribe(rr, req)
		assert.Equal(t, http.StatusBadRequest, rr.Code, "endpoint %q must be rejected", endpoint)
	}
}

func TestPushHandler_Subscribe_RejectsNonJSONContentType(t *testing.T) {
	repo := setupPushHandlerTestDB(t)
	handler := NewPushHandler(services.NewPushService(repo, "", "", nil))

	body, _ := json.Marshal(map[string]string{
		"endpoint": "https://fcm.googleapis.com/fcm/send/x", "p256dhKey": "k", "authKey": "a",
	})
	req, _ := http.NewRequest(http.MethodPost, "/api/push-subscriptions", bytes.NewReader(body))
	req.Header.Set("Content-Type", "text/plain")
	rr := httptest.NewRecorder()

	handler.Subscribe(rr, req)
	assert.Equal(t, http.StatusUnsupportedMediaType, rr.Code)
}

func TestPushHandler_Subscribe_RejectsUnknownField(t *testing.T) {
	repo := setupPushHandlerTestDB(t)
	handler := NewPushHandler(services.NewPushService(repo, "", "", nil))

	// A typo'd / unexpected field must be a 400, not silently dropped.
	req, _ := http.NewRequest(http.MethodPost, "/api/push-subscriptions",
		bytes.NewReader([]byte(`{"endpoint":"https://x.example/y","p256dhKey":"k","authKey":"a","extra":1}`)))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	handler.Subscribe(rr, req)
	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestPushHandler_Subscribe_DBError(t *testing.T) {
	repo := &errorPushSubscriptionRepo{err: errors.New("db error")}

	handler := NewPushHandler(services.NewPushService(repo, "", "", nil))

	reqBody := map[string]string{
		"endpoint":  "https://fcm.googleapis.com/fcm/send/test123",
		"p256dhKey": "key123",
		"authKey":   "auth456",
	}
	body, _ := json.Marshal(reqBody)

	req, _ := http.NewRequest(http.MethodPost, "/api/push-subscriptions", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	handler.Subscribe(rr, req)

	assert.Equal(t, http.StatusInternalServerError, rr.Code)
}

func TestPushHandler_Unsubscribe_Success(t *testing.T) {
	repo := setupPushHandlerTestDB(t)

	handler := NewPushHandler(services.NewPushService(repo, "", "", nil))

	testUserID := "test-user"
	require.NoError(t, repo.Upsert(context.Background(), &models.PushSubscription{
		ID:        "sub-1",
		UserID:    &testUserID,
		Endpoint:  "https://fcm.googleapis.com/fcm/send/test123",
		P256dhKey: "key123",
		AuthKey:   "auth456",
	}))

	reqBody := map[string]string{
		"endpoint": "https://fcm.googleapis.com/fcm/send/test123",
	}
	body, _ := json.Marshal(reqBody)

	req, _ := http.NewRequest(http.MethodDelete, "/api/push-subscriptions", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	handler.Unsubscribe(rr, req)

	assert.Equal(t, http.StatusNoContent, rr.Code)

	all, _ := repo.GetAll(context.Background())
	assert.Empty(t, all)
}

func TestPushHandler_Unsubscribe_MissingEndpoint(t *testing.T) {
	repo := setupPushHandlerTestDB(t)

	handler := NewPushHandler(services.NewPushService(repo, "", "", nil))

	reqBody := map[string]string{}
	body, _ := json.Marshal(reqBody)

	req, _ := http.NewRequest(http.MethodDelete, "/api/push-subscriptions", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	handler.Unsubscribe(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestPushHandler_Unsubscribe_DBError(t *testing.T) {
	repo := &errorPushSubscriptionRepo{err: errors.New("db error")}

	handler := NewPushHandler(services.NewPushService(repo, "", "", nil))

	reqBody := map[string]string{
		"endpoint": "https://fcm.googleapis.com/fcm/send/test123",
	}
	body, _ := json.Marshal(reqBody)

	req, _ := http.NewRequest(http.MethodDelete, "/api/push-subscriptions", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	handler.Unsubscribe(rr, req)

	assert.Equal(t, http.StatusInternalServerError, rr.Code)
}
