package carbonintensity

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestGetCurrent_Success(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{
			"data": [{
				"from": "2024-01-01T12:00Z",
				"to": "2024-01-01T12:30Z",
				"intensity": {
					"forecast": 266,
					"actual": 263,
					"index": "moderate"
				}
			}]
		}`))
	}))
	defer server.Close()

	client := NewClient()
	client.SetBaseURL(server.URL)

	intensity, err := client.GetCurrent(context.Background())
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if intensity.Forecast != 266 {
		t.Errorf("expected forecast 266, got %d", intensity.Forecast)
	}
	if intensity.Actual != 263 {
		t.Errorf("expected actual 263, got %d", intensity.Actual)
	}
	if intensity.Index != "moderate" {
		t.Errorf("expected index 'moderate', got %s", intensity.Index)
	}
	if callCount != 1 {
		t.Errorf("expected 1 API call, got %d", callCount)
	}
}

func TestGetCurrent_CachesWithinHalfHour(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{
			"data": [{
				"from": "2024-01-01T12:00Z",
				"to": "2024-01-01T12:30Z",
				"intensity": {
					"forecast": 200,
					"actual": 195,
					"index": "low"
				}
			}]
		}`))
	}))
	defer server.Close()

	client := NewClient()
	client.SetBaseURL(server.URL)

	_, _ = client.GetCurrent(context.Background())
	firstCallCount := callCount
	_, _ = client.GetCurrent(context.Background())

	if callCount != firstCallCount {
		t.Errorf("expected cached response, got %d calls (expected %d)", callCount, firstCallCount)
	}
}

func TestGetCurrent_EmptyData(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"data": []}`))
	}))
	defer server.Close()

	client := NewClient()
	client.SetBaseURL(server.URL)

	_, err := client.GetCurrent(context.Background())
	if err == nil {
		t.Fatal("expected error for empty data, got nil")
	}
}

func TestGetCurrent_Non200(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer server.Close()

	client := NewClient()
	client.SetBaseURL(server.URL)

	_, err := client.GetCurrent(context.Background())
	if err == nil {
		t.Fatal("expected error for non-200 status, got nil")
	}
}

func TestGetCurrent_NetworkError(t *testing.T) {
	client := NewClient()
	client.SetBaseURL("http://localhost:59999")

	_, err := client.GetCurrent(context.Background())
	if err == nil {
		t.Fatal("expected error for network failure, got nil")
	}
}

func TestGetCurrent_Concurrent(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(50 * time.Millisecond)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{
			"data": [{
				"from": "2024-01-01T12:00Z",
				"to": "2024-01-01T12:30Z",
				"intensity": {
					"forecast": 200,
					"actual": 195,
					"index": "low"
				}
			}]
		}`))
	}))
	defer server.Close()

	client := NewClient()
	client.SetBaseURL(server.URL)

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = client.GetCurrent(context.Background())
		}()
	}
	wg.Wait()
}

func TestFetchCurrent_InvalidJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`not valid json`))
	}))
	defer server.Close()

	client := NewClient()
	client.SetBaseURL(server.URL)

	_, err := client.GetCurrent(context.Background())
	if err == nil {
		t.Fatal("expected error for invalid JSON, got nil")
	}
}

func TestFetchCurrent_ContextCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	client := NewClient()
	client.SetBaseURL("http://localhost:59999")

	_, err := client.GetCurrent(ctx)
	if err == nil {
		t.Fatal("expected error for cancelled context, got nil")
	}
}

func forecastJSON(buckets []struct{ from, to string; co2 int }) string {
	var sb strings.Builder
	sb.WriteString(`{"data":[`)
	for i, b := range buckets {
		if i > 0 {
			sb.WriteString(",")
		}
		sb.WriteString(`{"from":"` + b.from + `","to":"` + b.to + `","intensity":{"forecast":` + strconv.Itoa(b.co2) + `,"actual":0,"index":"low"}}`)
	}
	sb.WriteString(`]}`)
	return sb.String()
}

func TestGetForecast_ParsesAndSorts(t *testing.T) {
	// Buckets returned in reverse order - expects them sorted by From.
	payload := forecastJSON([]struct{ from, to string; co2 int }{
		{"2024-01-01T11:00Z", "2024-01-01T11:30Z", 200},
		{"2024-01-01T10:30Z", "2024-01-01T11:00Z", 300},
		{"2024-01-01T10:00Z", "2024-01-01T10:30Z", 400},
	})

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(payload))
	}))
	defer server.Close()

	client := NewClient()
	client.SetBaseURL(server.URL)

	from := time.Date(2024, 1, 1, 10, 0, 0, 0, time.UTC)
	to := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)
	buckets, err := client.GetForecast(context.Background(), from, to)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(buckets) != 3 {
		t.Fatalf("expected 3 buckets, got %d", len(buckets))
	}
	// Verify sorted order.
	if !buckets[0].From.Equal(time.Date(2024, 1, 1, 10, 0, 0, 0, time.UTC)) {
		t.Errorf("first bucket From = %v, want 10:00", buckets[0].From)
	}
	if buckets[0].ForecastGCo2 != 400 {
		t.Errorf("first bucket CO2 = %d, want 400", buckets[0].ForecastGCo2)
	}
	if buckets[2].ForecastGCo2 != 200 {
		t.Errorf("last bucket CO2 = %d, want 200", buckets[2].ForecastGCo2)
	}
}

func TestGetForecast_CachesWithinHalfHour(t *testing.T) {
	callCount := 0
	payload := forecastJSON([]struct{ from, to string; co2 int }{
		{"2024-01-01T10:00Z", "2024-01-01T10:30Z", 200},
	})

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(payload))
	}))
	defer server.Close()

	client := NewClient()
	client.SetBaseURL(server.URL)

	from := time.Date(2024, 1, 1, 10, 0, 0, 0, time.UTC)
	to := time.Date(2024, 1, 1, 10, 30, 0, 0, time.UTC)

	_, _ = client.GetForecast(context.Background(), from, to)
	firstCount := callCount
	_, _ = client.GetForecast(context.Background(), from, to)

	if callCount != firstCount {
		t.Errorf("expected cached response (1 API call), got %d calls", callCount)
	}
}

func TestGetForecast_SlicesToRange(t *testing.T) {
	payload := forecastJSON([]struct{ from, to string; co2 int }{
		{"2024-01-01T09:00Z", "2024-01-01T09:30Z", 100},
		{"2024-01-01T10:00Z", "2024-01-01T10:30Z", 200},
		{"2024-01-01T10:30Z", "2024-01-01T11:00Z", 300},
		{"2024-01-01T11:00Z", "2024-01-01T11:30Z", 400},
	})

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(payload))
	}))
	defer server.Close()

	client := NewClient()
	client.SetBaseURL(server.URL)

	from := time.Date(2024, 1, 1, 10, 0, 0, 0, time.UTC)
	to := time.Date(2024, 1, 1, 11, 0, 0, 0, time.UTC)

	buckets, err := client.GetForecast(context.Background(), from, to)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Should return only the two buckets in [10:00, 11:00).
	if len(buckets) != 2 {
		t.Fatalf("expected 2 buckets, got %d", len(buckets))
	}
	if buckets[0].ForecastGCo2 != 200 {
		t.Errorf("first bucket CO2 = %d, want 200", buckets[0].ForecastGCo2)
	}
	if buckets[1].ForecastGCo2 != 300 {
		t.Errorf("second bucket CO2 = %d, want 300", buckets[1].ForecastGCo2)
	}
}

func TestGetForecast_EmptyData(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"data":[]}`))
	}))
	defer server.Close()

	client := NewClient()
	client.SetBaseURL(server.URL)

	from := time.Date(2024, 1, 1, 10, 0, 0, 0, time.UTC)
	to := time.Date(2024, 1, 1, 11, 0, 0, 0, time.UTC)

	_, err := client.GetForecast(context.Background(), from, to)
	if err == nil {
		t.Fatal("expected error for empty forecast data, got nil")
	}
}

func TestGetForecast_Non200(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer server.Close()

	client := NewClient()
	client.SetBaseURL(server.URL)

	_, err := client.GetForecast(context.Background(), time.Now(), time.Now().Add(time.Hour))
	if err == nil {
		t.Fatal("expected error for non-200 status, got nil")
	}
}

func TestGetForecast_NoOverlappingBuckets(t *testing.T) {
	payload := forecastJSON([]struct{ from, to string; co2 int }{
		{"2024-01-01T08:00Z", "2024-01-01T08:30Z", 100},
	})

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(payload))
	}))
	defer server.Close()

	client := NewClient()
	client.SetBaseURL(server.URL)

	// Request a range that has no overlapping buckets.
	from := time.Date(2024, 1, 1, 10, 0, 0, 0, time.UTC)
	to := time.Date(2024, 1, 1, 11, 0, 0, 0, time.UTC)

	_, err := client.GetForecast(context.Background(), from, to)
	if err == nil {
		t.Fatal("expected error for no overlapping buckets, got nil")
	}
}

func TestSliceBuckets_EdgeCases(t *testing.T) {
	t0 := time.Date(2024, 1, 1, 10, 0, 0, 0, time.UTC)
	t1 := t0.Add(30 * time.Minute)
	t2 := t1.Add(30 * time.Minute)
	t3 := t2.Add(30 * time.Minute)

	buckets := []ForecastBucket{
		{From: t0, To: t1, ForecastGCo2: 100},
		{From: t1, To: t2, ForecastGCo2: 200},
		{From: t2, To: t3, ForecastGCo2: 300},
	}

	// Exact range match.
	got := sliceBuckets(buckets, t0, t3)
	if len(got) != 3 {
		t.Errorf("expected 3, got %d", len(got))
	}

	// Only middle bucket.
	got = sliceBuckets(buckets, t1, t2)
	if len(got) != 1 || got[0].ForecastGCo2 != 200 {
		t.Errorf("expected middle bucket only, got %+v", got)
	}

	// Empty range.
	got = sliceBuckets(buckets, t1, t1)
	if len(got) != 0 {
		t.Errorf("expected empty result for empty range, got %d", len(got))
	}

	// Range before all buckets.
	got = sliceBuckets(buckets, t0.Add(-2*time.Hour), t0)
	if len(got) != 0 {
		t.Errorf("expected empty result for range before all buckets, got %d", len(got))
	}
}

func TestNextHalfHourUTC(t *testing.T) {
	tests := []struct {
		name     string
		minute   int
		expected int
	}{
		{"10 minutes past", 10, 30},
		{"29 minutes past", 29, 30},
		{"30 minutes past", 30, 0},
		{"59 minutes past", 59, 0},
		{"exactly on hour", 0, 30},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			now := time.Date(2024, 1, 1, 14, tt.minute, 0, 0, time.UTC)
			next := nextHalfHourUTC(now)
			if next.Minute() != tt.expected {
				t.Errorf("expected minute %d, got %d", tt.expected, next.Minute())
			}
		})
	}
}
