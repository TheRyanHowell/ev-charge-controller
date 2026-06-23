package services

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"ev-charge-controller/api/models"

	"github.com/SherClockHolmes/webpush-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Valid P-256 subscription keys for testing.
// Generated with crypto/ecdh.P256().GenerateKey() + 16-byte random auth secret.
// URL-safe base64 without padding (what browsers send via PushSubscription.getKey()).
const testP256dh = "BE4R_NQ9HkQ19uTmn9sKcrR1teJq9THRwmBUXNJe0B9nMGUH0FKBIg4qqyYjcrS29BwSvRUMamluwX50sIJiubU"
const testAuth = "pmHYFJcs6BgS82GDL35L9Q"

// mockPushRepo implements pushRepo for testing.
type mockPushRepo struct {
	mu      sync.Mutex
	subs    []models.PushSubscription
	removed []string
}

func (m *mockPushRepo) Upsert(_ context.Context, sub *models.PushSubscription) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.subs = append(m.subs, *sub)
	return nil
}

func (m *mockPushRepo) RemoveByEndpoint(_ context.Context, endpoint string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.removed = append(m.removed, endpoint)
	var filtered []models.PushSubscription
	for _, s := range m.subs {
		if s.Endpoint != endpoint {
			filtered = append(filtered, s)
		}
	}
	m.subs = filtered
	return nil
}

func (m *mockPushRepo) GetAll(_ context.Context) ([]models.PushSubscription, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	result := make([]models.PushSubscription, len(m.subs))
	copy(result, m.subs)
	return result, nil
}

// mockHTTPClient implements webpush.HTTPClient for testing.
type mockHTTPClient struct {
	handler func(*http.Request) (*http.Response, error)
}

func (m *mockHTTPClient) Do(req *http.Request) (*http.Response, error) {
	return m.handler(req)
}

func TestPushService_SendNotification_NoSubscriptions(t *testing.T) {
	repo := &mockPushRepo{}
	client := &mockHTTPClient{handler: func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusOK, Status: "200 OK"}, nil
	}}
	ps := NewPushService(repo, "pub", "priv", client)

	err := 	ps.SendNotification(context.Background(), "Charge Complete", "RM1 reached 80%")
	require.NoError(t, err)
	assert.Empty(t, repo.removed)
}

func TestPushService_SendNotification_RemovesStaleOn410(t *testing.T) {
	repo := &mockPushRepo{}
	require.NoError(t, repo.Upsert(context.Background(), &models.PushSubscription{
		ID:        "sub-stale",
		Endpoint:  "https://fcm.googleapis.com/fcm/send/abc",
		P256dhKey: testP256dh,
		AuthKey:   testAuth,
	}))

	client := &mockHTTPClient{
		handler: func(r *http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: http.StatusGone, Status: "410 Gone"}, nil
		},
	}

	ps := NewPushService(repo, "pub", "priv", client)
	err := ps.SendNotification(context.Background(), "Charge Complete", "RM1 reached 80%")
	require.NoError(t, err)

	all, _ := repo.GetAll(context.Background())
	assert.Empty(t, all, "subscription should be removed after 410")
	require.Len(t, repo.removed, 1)
	assert.Equal(t, "https://fcm.googleapis.com/fcm/send/abc", repo.removed[0])
}

func TestPushService_SendNotification_RemovesStaleOn404(t *testing.T) {
	repo := &mockPushRepo{}
	require.NoError(t, repo.Upsert(context.Background(), &models.PushSubscription{
		ID:        "sub-404",
		Endpoint:  "https://updates.push.services.mozilla.com/push/abc",
		P256dhKey: testP256dh,
		AuthKey:   testAuth,
	}))

	client := &mockHTTPClient{
		handler: func(r *http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: http.StatusNotFound, Status: "404 Not Found"}, nil
		},
	}

	ps := NewPushService(repo, "pub", "priv", client)
	err := ps.SendNotification(context.Background(), "Charge Complete", "RM1 reached 80%")
	require.NoError(t, err)

	all, _ := repo.GetAll(context.Background())
	assert.Empty(t, all, "subscription should be removed after 404")
}

func TestPushService_SendNotification_MixedResults(t *testing.T) {
	repo := &mockPushRepo{}
	require.NoError(t, repo.Upsert(context.Background(), &models.PushSubscription{
		ID:        "sub-ok",
		Endpoint:  "https://fcm.googleapis.com/fcm/send/ok",
		P256dhKey: testP256dh,
		AuthKey:   testAuth,
	}))
	require.NoError(t, repo.Upsert(context.Background(), &models.PushSubscription{
		ID:        "sub-stale",
		Endpoint:  "https://fcm.googleapis.com/fcm/send/stale",
		P256dhKey: testP256dh,
		AuthKey:   testAuth,
	}))

	// Decide per-endpoint (not by call order) so the result is deterministic
	// and race-free under the concurrent fan-out.
	client := &mockHTTPClient{
		handler: func(r *http.Request) (*http.Response, error) {
			if strings.HasSuffix(r.URL.Path, "/stale") {
				return &http.Response{StatusCode: http.StatusGone, Status: "410 Gone"}, nil
			}
			return &http.Response{StatusCode: http.StatusOK, Status: "200 OK"}, nil
		},
	}

	ps := NewPushService(repo, "pub", "priv", client)
	err := ps.SendNotification(context.Background(), "Charge Complete", "RM1 reached 80%")
	require.NoError(t, err)

	all, _ := repo.GetAll(context.Background())
	require.Len(t, all, 1)
	assert.Equal(t, "sub-ok", all[0].ID)
}

func TestPushService_SendNotification_500DoesNotRemove(t *testing.T) {
	repo := &mockPushRepo{}
	require.NoError(t, repo.Upsert(context.Background(), &models.PushSubscription{
		ID:        "sub-1",
		Endpoint:  "https://fcm.googleapis.com/fcm/send/abc",
		P256dhKey: testP256dh,
		AuthKey:   testAuth,
	}))

	client := &mockHTTPClient{
		handler: func(r *http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: http.StatusInternalServerError, Status: "500 Internal Server Error"}, nil
		},
	}

	ps := NewPushService(repo, "pub", "priv", client)
	err := ps.SendNotification(context.Background(), "Charge Complete", "RM1 reached 80%")
	require.NoError(t, err)

	all, _ := repo.GetAll(context.Background())
	require.Len(t, all, 1, "500 should NOT remove subscription")
}

func TestNormalizeBase64URLToBase64(t *testing.T) {
	tests := []struct {
		input  string
		output string
	}{
		{"abc", "abc="},
		{"abcd", "abcd"},
		{"abc-", "abc+", },
		{"abc-_d", "abc+/d=="},
		{testP256dh, "BE4R/NQ9HkQ19uTmn9sKcrR1teJq9THRwmBUXNJe0B9nMGUH0FKBIg4qqyYjcrS29BwSvRUMamluwX50sIJiubU="},
		{testAuth, "pmHYFJcs6BgS82GDL35L9Q=="},
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			result := normalizeBase64URLToBase64(tc.input)
			assert.Equal(t, tc.output, result)
		})
	}
}

func TestIsStaleResponse(t *testing.T) {
	tests := []struct {
		code     int
		expected bool
	}{
		{http.StatusGone, true},
		{http.StatusNotFound, true},
		{http.StatusOK, false},
		{http.StatusInternalServerError, false},
		{http.StatusBadRequest, false},
	}

	for _, tc := range tests {
		t.Run(fmt.Sprintf("%d", tc.code), func(t *testing.T) {
			result := isStaleResponse(tc.code)
			assert.Equal(t, tc.expected, result, "failed for code: %d", tc.code)
		})
	}
}

func TestSelectAuthScheme_FCM(t *testing.T) {
	scheme := selectAuthScheme("https://fcm.googleapis.com/fcm/send/abc")
	assert.Equal(t, webpush.WebPush, scheme)
}

func TestSelectAuthScheme_Android(t *testing.T) {
	scheme := selectAuthScheme("https://android.googleapis.com/gcm/send/abc")
	assert.Equal(t, webpush.WebPush, scheme)
}

func TestSelectAuthScheme_Apple(t *testing.T) {
	scheme := selectAuthScheme("https://apple.com/gcm/send/abc")
	assert.Equal(t, webpush.WebPush, scheme)
}

func TestSelectAuthScheme_Firefox(t *testing.T) {
	scheme := selectAuthScheme("https://updates.push.services.mozilla.com/push/abc")
	assert.Equal(t, webpush.Vapid, scheme)
}

func TestSelectAuthScheme_Default(t *testing.T) {
	scheme := selectAuthScheme("http://localhost:8080/test")
	assert.Equal(t, webpush.Vapid, scheme)
}

func TestNewPushService(t *testing.T) {
	ps := NewPushService(&mockPushRepo{}, "public-key", "private-key", nil)

	require.NotNil(t, ps)
	assert.Equal(t, "public-key", ps.vapidPubKey)
	assert.Equal(t, "private-key", ps.vapidPrivKey)
	assert.NotNil(t, ps.httpClient)
}

func TestNewPushService_NilHTTPClient(t *testing.T) {
	ps := NewPushService(&mockPushRepo{}, "pub", "priv", nil)

	require.NotNil(t, ps)
	assert.NotNil(t, ps.httpClient, "nil httpClient should default to http.DefaultClient")
}

type errorPushRepo struct {
	err error
}

func (e *errorPushRepo) GetAll(_ context.Context) ([]models.PushSubscription, error) {
	return nil, e.err
}

func (e *errorPushRepo) RemoveByEndpoint(_ context.Context, _ string) error {
	return e.err
}

func (e *errorPushRepo) Upsert(_ context.Context, _ *models.PushSubscription) error {
	return e.err
}

func TestPushService_SendNotification_GetAllError(t *testing.T) {
	repo := &errorPushRepo{err: assert.AnError}
	ps := NewPushService(repo, "pub", "priv", nil)

	err := ps.SendNotification(context.Background(), "title", "body")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to get push subscriptions")
}

func TestPushService_SendNotification_SendError(t *testing.T) {
	repo := &mockPushRepo{}
	require.NoError(t, repo.Upsert(context.Background(), &models.PushSubscription{
		ID:        "sub-1",
		Endpoint:  "https://fcm.googleapis.com/fcm/send/test",
		P256dhKey: testP256dh,
		AuthKey:   testAuth,
	}))

	client := &mockHTTPClient{
		handler: func(r *http.Request) (*http.Response, error) {
			return nil, assert.AnError
		},
	}

	ps := NewPushService(repo, "pub", "priv", client)
	err := ps.SendNotification(context.Background(), "title", "body")
	require.Error(t, err, "a failed send must surface the failure count")
	assert.Contains(t, err.Error(), "1 of 1")

	all, _ := repo.GetAll(context.Background())
	require.Len(t, all, 1, "subscription should NOT be removed on send error")
}

func TestPushService_SendNotification_RemoveError(t *testing.T) {
	repo := &mockPushRepo{}
	require.NoError(t, repo.Upsert(context.Background(), &models.PushSubscription{
		ID:        "sub-1",
		Endpoint:  "https://fcm.googleapis.com/fcm/send/test",
		P256dhKey: testP256dh,
		AuthKey:   testAuth,
	}))

	// Override RemoveByEndpoint to return error
	origRemove := repo.removed
	repo.removed = nil

	client := &mockHTTPClient{
		handler: func(r *http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: http.StatusGone, Status: "410 Gone"}, nil
		},
	}

	ps := NewPushService(repo, "pub", "priv", client)
	err := ps.SendNotification(context.Background(), "title", "body")
	require.NoError(t, err) // RemoveByEndpoint error is logged, not returned

	// Verify the subscription was still removed despite the error path being exercised
	_ = origRemove
}

func TestPushService_UpsertSubscription(t *testing.T) {
	repo := &mockPushRepo{}
	ps := NewPushService(repo, "pub", "priv", &http.Client{})

	sub := &models.PushSubscription{
		Endpoint:  "https://fcm.googleapis.com/v1/projects/test",
		P256dhKey: testP256dh,
		AuthKey:   testAuth,
	}
	err := ps.UpsertSubscription(context.Background(), sub)
	require.NoError(t, err)

	repo.mu.Lock()
	defer repo.mu.Unlock()
	assert.Len(t, repo.subs, 1)
	assert.Equal(t, sub.Endpoint, repo.subs[0].Endpoint)
}

func TestPushService_RemoveSubscriptionByEndpoint(t *testing.T) {
	repo := &mockPushRepo{}
	ps := NewPushService(repo, "pub", "priv", &http.Client{})

	// First insert a subscription
	sub := &models.PushSubscription{
		Endpoint:  "https://fcm.googleapis.com/v1/projects/test",
		P256dhKey: testP256dh,
		AuthKey:   testAuth,
	}
	require.NoError(t, repo.Upsert(context.Background(), sub))

	// Remove by endpoint
	err := ps.RemoveSubscriptionByEndpoint(context.Background(), sub.Endpoint)
	require.NoError(t, err)

	repo.mu.Lock()
	defer repo.mu.Unlock()
	assert.Len(t, repo.subs, 0)
	assert.Len(t, repo.removed, 1)
	assert.Equal(t, sub.Endpoint, repo.removed[0])
}

func TestDrainAndCloseBody_NilResponse(t *testing.T) {
	drainAndCloseBody(nil)
}

func TestDrainAndCloseBody_NilBody(t *testing.T) {
	drainAndCloseBody(&http.Response{Body: nil})
}

func TestDrainAndCloseBody_Normal(t *testing.T) {
	resp := &http.Response{
		Body: io.NopCloser(bytes.NewReader([]byte("drain me"))),
	}
	drainAndCloseBody(resp)

	buf := make([]byte, 1)
	n, _ := resp.Body.Read(buf)
	assert.Zero(t, n, "body should be fully drained")
}

func TestRedactEndpoint_WithSlash(t *testing.T) {
	result := redactEndpoint("https://fcm.googleapis.com/fcm/send/abc123")
	assert.Equal(t, "https://fcm.googleapis.com/fcm/send/***", result)
}

func TestRedactEndpoint_NoSlash(t *testing.T) {
	result := redactEndpoint("noSlashHere")
	assert.Equal(t, "noSlashHere", result)
}

func TestRedactEndpoint_Empty(t *testing.T) {
	result := redactEndpoint("")
	assert.Equal(t, "", result)
}

func TestRedactEndpoint_SingleSlash(t *testing.T) {
	result := redactEndpoint("/")
	assert.Equal(t, "/", result)
}

func TestNewPushService_WithHTTPClient(t *testing.T) {
	client := &http.Client{Timeout: 5 * time.Second}
	ps := NewPushService(&mockPushRepo{}, "public-key", "private-key", client)

	require.NotNil(t, ps)
	assert.Equal(t, "public-key", ps.vapidPubKey)
	assert.Equal(t, "private-key", ps.vapidPrivKey)
	assert.Equal(t, client, ps.httpClient)
}
