package carbonintensity

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"sync"
	"time"
)

// CarbonIntensity represents the current carbon intensity reading.
type CarbonIntensity struct {
	Forecast int    `json:"forecast"` // gCO2/kWh forecast
	Actual   int    `json:"actual"`   // gCO2/kWh actual
	Index    string `json:"index"`    // very low, low, moderate, high, very high
}

// CarbonIntensityResponse is the API response from carbonintensity.org.uk.
type CarbonIntensityResponse struct {
	Data []struct {
		From      string          `json:"from"`
		To        string          `json:"to"`
		Intensity CarbonIntensity `json:"intensity"`
	} `json:"data"`
}

// ForecastBucket represents a 30-minute forecast window.
type ForecastBucket struct {
	From         time.Time
	To           time.Time
	ForecastGCo2 int
}

// cacheEntry holds a cached intensity value and its expiration time.
type cacheEntry struct {
	value     *CarbonIntensity
	expiresAt time.Time
}

// forecastCacheEntry holds the cached 48h forecast.
type forecastCacheEntry struct {
	buckets   []ForecastBucket
	expiresAt time.Time
}

// nowFunc returns the current time. Overridable for testing.
var nowFunc = time.Now

// timestampLayout is the format used by the carbon intensity API.
const timestampLayout = "2006-01-02T15:04Z"

// Client is a client for the UK Carbon Intensity API.
type Client struct {
	httpClient      *http.Client
	baseURL         string
	mu              sync.RWMutex
	cached          *cacheEntry
	forecastCached  *forecastCacheEntry
}

// NewClient creates a new Carbon Intensity API client with caching.
func NewClient() *Client {
	return &Client{
		httpClient: &http.Client{Timeout: CarbonIntensityHttpTimeout},
		baseURL:    CarbonIntensityBaseURL,
	}
}

// SetBaseURL sets the base URL. Used for testing.
func (c *Client) SetBaseURL(url string) {
	c.baseURL = url
}

// CarbonIntensityHttpTimeout is the HTTP timeout for API requests.
const CarbonIntensityHttpTimeout = 5 * time.Second

// CarbonIntensityBaseURL is the base URL for the Carbon Intensity API.
const CarbonIntensityBaseURL = "https://api.carbonintensity.org.uk"

// GetCurrent returns the current carbon intensity for GB.
// Uses an in-memory cache with a 30-minute TTL to align with the API's update cycle.
func (c *Client) GetCurrent(ctx context.Context) (*CarbonIntensity, error) {
	c.mu.RLock()
	if c.cached != nil && nowFunc().UTC().Before(c.cached.expiresAt) {
		val := c.cached.value
		c.mu.RUnlock()
		return val, nil
	}
	c.mu.RUnlock()

	intensity, err := c.fetchCurrent(ctx)
	if err != nil {
		return nil, err
	}

	c.mu.Lock()
	c.cached = &cacheEntry{
		value:     intensity,
		expiresAt: nextHalfHourUTC(nowFunc()),
	}
	c.mu.Unlock()

	return intensity, nil
}

// fetchCurrent performs the actual API fetch (no caching).
func (c *Client) fetchCurrent(ctx context.Context) (*CarbonIntensity, error) {
	reqURL := fmt.Sprintf("%s/intensity", c.baseURL)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create carbon intensity request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch carbon intensity: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("carbon intensity API returned status %d", resp.StatusCode)
	}

	var apiResp CarbonIntensityResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return nil, fmt.Errorf("failed to parse carbon intensity response: %w", err)
	}

	if len(apiResp.Data) == 0 {
		return nil, fmt.Errorf("carbon intensity API returned empty data")
	}

	return &apiResp.Data[0].Intensity, nil
}

// GetForecast returns forecast buckets for the GB grid over the requested time range.
// The full 48h horizon is fetched once per half-hour and cached; the result is sliced
// to [from, to]. Returns an error if the API call fails or returns no data.
func (c *Client) GetForecast(ctx context.Context, from, to time.Time) ([]ForecastBucket, error) {
	c.mu.RLock()
	if c.forecastCached != nil && nowFunc().UTC().Before(c.forecastCached.expiresAt) {
		buckets := sliceBuckets(c.forecastCached.buckets, from, to)
		c.mu.RUnlock()
		if len(buckets) == 0 {
			return nil, fmt.Errorf("carbon forecast: no buckets in range [%v, %v]", from, to)
		}
		return buckets, nil
	}
	c.mu.RUnlock()

	buckets, err := c.fetchForecast(ctx)
	if err != nil {
		return nil, err
	}

	c.mu.Lock()
	c.forecastCached = &forecastCacheEntry{
		buckets:   buckets,
		expiresAt: nextHalfHourUTC(nowFunc()),
	}
	c.mu.Unlock()

	result := sliceBuckets(buckets, from, to)
	if len(result) == 0 {
		return nil, fmt.Errorf("carbon forecast: no buckets in range [%v, %v]", from, to)
	}
	return result, nil
}

// fetchForecast performs the 48h forecast API fetch (no caching).
func (c *Client) fetchForecast(ctx context.Context) ([]ForecastBucket, error) {
	fromStr := nowFunc().UTC().Format(timestampLayout)
	reqURL := fmt.Sprintf("%s/intensity/%s/fw48h", c.baseURL, fromStr)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create forecast request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch carbon forecast: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("carbon forecast API returned status %d", resp.StatusCode)
	}

	var apiResp CarbonIntensityResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return nil, fmt.Errorf("failed to parse carbon forecast response: %w", err)
	}

	if len(apiResp.Data) == 0 {
		return nil, fmt.Errorf("carbon forecast API returned empty data")
	}

	buckets := make([]ForecastBucket, 0, len(apiResp.Data))
	for _, d := range apiResp.Data {
		from, err := time.Parse(timestampLayout, d.From)
		if err != nil {
			return nil, fmt.Errorf("failed to parse forecast bucket from %q: %w", d.From, err)
		}
		to, err := time.Parse(timestampLayout, d.To)
		if err != nil {
			return nil, fmt.Errorf("failed to parse forecast bucket to %q: %w", d.To, err)
		}
		buckets = append(buckets, ForecastBucket{
			From:         from.UTC(),
			To:           to.UTC(),
			ForecastGCo2: d.Intensity.Forecast,
		})
	}

	sort.Slice(buckets, func(i, j int) bool {
		return buckets[i].From.Before(buckets[j].From)
	})

	return buckets, nil
}

// sliceBuckets returns the subset of buckets that overlap [from, to).
func sliceBuckets(buckets []ForecastBucket, from, to time.Time) []ForecastBucket {
	fromUTC := from.UTC()
	toUTC := to.UTC()
	var out []ForecastBucket
	for _, b := range buckets {
		if b.To.After(fromUTC) && b.From.Before(toUTC) {
			out = append(out, b)
		}
	}
	return out
}

// nextHalfHourUTC returns the next :00 or :30 UTC boundary.
func nextHalfHourUTC(now time.Time) time.Time {
	now = now.UTC()
	next := time.Date(now.Year(), now.Month(), now.Day(), now.Hour(), 0, 0, 0, time.UTC)
	if now.Minute() < 30 {
		next = next.Add(30 * time.Minute)
	} else {
		next = next.Add(time.Hour)
	}
	return next
}
