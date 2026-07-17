package services

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"ev-charge-controller/api/internal"
	"ev-charge-controller/api/models"

	"github.com/SherClockHolmes/webpush-go"
)

// pushFanOutConcurrency bounds how many subscription sends run in parallel.
const pushFanOutConcurrency = 8

// pushSendMaxAttempts bounds delivery attempts per subscription. Transient
// failures (network errors, 429, 5xx) are retried so a brief blip doesn't
// lose a notification; permanent rejections are never retried.
const pushSendMaxAttempts = 3

// pushSendRetryBaseDelay is the delay before the first retry; it doubles on
// each subsequent attempt (2s, 4s), keeping total retry time within the
// notifier's send timeout.
const pushSendRetryBaseDelay = 2 * time.Second

type PushPayload struct {
	Title     string `json:"title"`
	Body      string `json:"body"`
	Vibration []int  `json:"vibration"`
}

type PushService struct {
	repo         internal.PushSubscriptionRepo
	vapidPubKey  string
	vapidPrivKey string
	httpClient   webpush.HTTPClient
	// retryBaseDelay is the first retry backoff; overridable in tests.
	retryBaseDelay time.Duration
}

func NewPushService(repo internal.PushSubscriptionRepo, publicKey, privateKey string, httpClient webpush.HTTPClient) *PushService {
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	return &PushService{
		repo:           repo,
		vapidPubKey:    publicKey,
		vapidPrivKey:   privateKey,
		httpClient:     httpClient,
		retryBaseDelay: pushSendRetryBaseDelay,
	}
}

// pushTTLSeconds is the time-to-live for push messages in seconds.
// FCM will hold the message for this duration if the device is offline.
// 24 hours is the maximum allowed by FCM.
const pushTTLSeconds = 86400

// SendNotification sends a push notification to all registered subscriptions.
// Removes stale subscriptions (410 Gone, 404 Not Found) per Web Push spec.
func (ps *PushService) SendNotification(ctx context.Context, title, body string) error {
	subs, err := ps.repo.GetAll(ctx)
	if err != nil {
		return fmt.Errorf("failed to get push subscriptions: %w", err)
	}

	slog.Info("SendNotification", "title", title, "body", body, "subscriptions", len(subs))

	if len(subs) == 0 {
		slog.Info("No subscriptions to send notification to")
		return nil
	}

	payload, err := json.Marshal(PushPayload{
		Title:     title,
		Body:      body,
		Vibration: []int{300, 100, 300, 100, 300, 100, 300, 100, 600, 200},
	})
	if err != nil {
		return fmt.Errorf("failed to marshal push payload: %w", err)
	}

	// Fan out across a bounded worker pool so a large subscription list doesn't
	// open one connection per subscriber, while still parallelising slow sends.
	sem := make(chan struct{}, pushFanOutConcurrency)
	var wg sync.WaitGroup
	var mu sync.Mutex
	var failures int

	for _, sub := range subs {
		wg.Add(1)
		sem <- struct{}{}
		go func(sub models.PushSubscription) {
			defer wg.Done()
			defer func() { <-sem }()
			if sendErr := ps.sendToSubscription(ctx, sub, payload); sendErr != nil {
				slog.Error("Send error for subscription", "id", sub.ID, "err", sendErr)
				mu.Lock()
				failures++
				mu.Unlock()
			}
		}(sub)
	}
	wg.Wait()

	if failures > 0 {
		return fmt.Errorf("failed to send push notification to %d of %d subscriptions", failures, len(subs))
	}
	return nil
}

// sendToSubscription delivers a single payload to one subscription, retrying
// transient failures with exponential backoff. The subscription is removed if
// the push service reports it stale (410 Gone / 404).
func (ps *PushService) sendToSubscription(ctx context.Context, sub models.PushSubscription, payload []byte) error {
	authScheme := selectAuthScheme(sub.Endpoint)

	opts := &webpush.Options{
		AuthScheme:      authScheme,
		HTTPClient:      ps.httpClient,
		TTL:             pushTTLSeconds,
		Urgency:         webpush.UrgencyHigh,
		VAPIDPublicKey:  ps.vapidPubKey,
		VAPIDPrivateKey: ps.vapidPrivKey,
	}

	wpSub := &webpush.Subscription{
		Endpoint: sub.Endpoint,
		Keys: webpush.Keys{
			Auth:   normalizeBase64URLToBase64(sub.AuthKey),
			P256dh: normalizeBase64URLToBase64(sub.P256dhKey),
		},
	}

	slog.Info("Sending to subscription", "id", sub.ID, "endpoint", redactEndpoint(sub.Endpoint), "auth", authScheme)

	var lastErr error
	delay := ps.retryBaseDelay
	for attempt := 1; attempt <= pushSendMaxAttempts; attempt++ {
		if attempt > 1 {
			select {
			case <-ctx.Done():
				return fmt.Errorf("push send cancelled after %d attempts: %w", attempt-1, lastErr)
			case <-time.After(delay):
			}
			delay *= 2
		}

		retryable, err := ps.attemptSend(ctx, sub, wpSub, opts, payload)
		if err == nil || !retryable {
			return err
		}
		lastErr = err
		slog.Warn("Transient push send failure, will retry", "id", sub.ID, "attempt", attempt, "err", err)
	}
	return fmt.Errorf("push send failed after %d attempts: %w", pushSendMaxAttempts, lastErr)
}

// attemptSend performs one delivery attempt and classifies the outcome.
// retryable is true only for failures worth another attempt: transport
// errors, 429 rate limiting, and 5xx. Stale subscriptions (410/404) are
// pruned and reported as success; other 4xx are permanent rejections.
func (ps *PushService) attemptSend(ctx context.Context, sub models.PushSubscription, wpSub *webpush.Subscription, opts *webpush.Options, payload []byte) (retryable bool, err error) {
	resp, err := webpush.SendNotificationWithContext(ctx, payload, wpSub, opts)
	if err != nil {
		// Context cancellation is terminal; transport errors are retryable.
		return ctx.Err() == nil, err
	}
	defer drainAndCloseBody(resp)

	slog.Info("Response for subscription", "id", sub.ID, "statusCode", resp.StatusCode, "status", resp.Status)

	switch {
	case isStaleResponse(resp.StatusCode):
		slog.Info("Removing stale subscription", "id", sub.ID, "statusCode", resp.StatusCode, "status", resp.Status)
		if removeErr := ps.repo.RemoveByEndpoint(ctx, sub.Endpoint); removeErr != nil {
			slog.Error("Failed to remove stale subscription", "id", sub.ID, "err", removeErr)
		}
		return false, nil
	case resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= http.StatusInternalServerError:
		return true, fmt.Errorf("push service returned %s", resp.Status)
	case resp.StatusCode >= http.StatusBadRequest:
		return false, fmt.Errorf("push service rejected notification: %s", resp.Status)
	default:
		return false, nil
	}
}

// drainAndCloseBody fully reads and closes a response body so the underlying
// keep-alive connection to the push service can be reused.
func drainAndCloseBody(resp *http.Response) {
	if resp == nil || resp.Body == nil {
		return
	}
	_, _ = io.Copy(io.Discard, resp.Body)
	_ = resp.Body.Close()
}

// redactEndpoint hides sensitive path segments from the endpoint URL for logging.
func redactEndpoint(endpoint string) string {
	if idx := strings.LastIndex(endpoint, "/"); idx > 0 {
		return endpoint[:idx] + "/***"
	}
	return endpoint
}

// selectAuthScheme picks the correct VAPID auth scheme based on the push service
// endpoint. Chrome/FCM/Apple use the modern "webpush" scheme (RFC 8292), while
// Firefox uses the original "vapid" scheme.
func selectAuthScheme(endpoint string) webpush.AuthScheme {
	if strings.Contains(endpoint, "android.googleapis.com") ||
		strings.Contains(endpoint, "fcm.googleapis.com") ||
		strings.Contains(endpoint, "apple.com") {
		return webpush.WebPush
	}
	return webpush.Vapid
}

// normalizeBase64URLToBase64 converts URL-safe base64 (without padding) to
// standard base64 (with padding). Browsers send URL-safe base64 without padding,
// but webpush-go expects standard base64 with padding.
func normalizeBase64URLToBase64(s string) string {
	s = strings.ReplaceAll(s, "-", "+")
	s = strings.ReplaceAll(s, "_", "/")
	switch len(s) % 4 {
	case 2:
		s += "=="
	case 3:
		s += "="
	}
	return s
}

func isStaleResponse(statusCode int) bool {
	return statusCode == http.StatusGone || statusCode == http.StatusNotFound
}

// UpsertSubscription saves or updates a push subscription.
func (s *PushService) UpsertSubscription(ctx context.Context, sub *models.PushSubscription) error {
	return s.repo.Upsert(ctx, sub)
}

// RemoveSubscriptionByEndpoint removes a push subscription by endpoint.
func (s *PushService) RemoveSubscriptionByEndpoint(ctx context.Context, endpoint string) error {
	return s.repo.RemoveByEndpoint(ctx, endpoint)
}
