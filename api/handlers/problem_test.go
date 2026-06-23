package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// withDebugResponses enables debug responses for the duration of the test and
// resets the flag when it returns, preventing state leakage between tests.
func withDebugResponses(t *testing.T) {
	t.Helper()
	debugResponsesEnabled = true
	t.Cleanup(func() { debugResponsesEnabled = false })
}

func TestProblemJSON_NoDebugField(t *testing.T) {
	w := httptest.NewRecorder()
	problemJSON(w, http.StatusBadRequest, "about:blank#test", "Bad Request", "test detail")

	resp := w.Result()
	require.Equal(t, http.StatusBadRequest, resp.StatusCode)

	var pd ProblemDetails
	err := json.NewDecoder(resp.Body).Decode(&pd)
	require.NoError(t, err)

	assert.Equal(t, "about:blank#test", pd.Type)
	assert.Equal(t, "Bad Request", pd.Title)
	assert.Equal(t, http.StatusBadRequest, pd.Status)
	assert.Equal(t, "test detail", pd.Detail)
	assert.Empty(t, pd.Debug)
}

func TestProblemJSONDebug_IncludesDebugWhenEnabled(t *testing.T) {
	withDebugResponses(t)

	w := httptest.NewRecorder()
	problemJSONDebug(w, http.StatusInternalServerError, "about:blank#internal-error", "Internal Server Error", "something went wrong", "sqlite3: no such table: foo")

	resp := w.Result()
	require.Equal(t, http.StatusInternalServerError, resp.StatusCode)

	var pd ProblemDetails
	err := json.NewDecoder(resp.Body).Decode(&pd)
	require.NoError(t, err)

	assert.Equal(t, "sqlite3: no such table: foo", pd.Debug)
}

func TestProblemJSONDebug_OmitsDebugWhenDisabled(t *testing.T) {
	// debugResponsesEnabled is false by default; no setup needed.
	w := httptest.NewRecorder()
	problemJSONDebug(w, http.StatusInternalServerError, "about:blank#internal-error", "Internal Server Error", "something went wrong", "sqlite3: no such table: foo")

	resp := w.Result()
	require.Equal(t, http.StatusInternalServerError, resp.StatusCode)

	var pd ProblemDetails
	err := json.NewDecoder(resp.Body).Decode(&pd)
	require.NoError(t, err)

	assert.Empty(t, pd.Debug, "debug field must be omitted when debug responses are not enabled")
}

func TestProblemJSONDebug_DebugNotSerializedWhenEmpty(t *testing.T) {
	// debugResponsesEnabled is false; the "debug" key must not appear in the JSON.
	w := httptest.NewRecorder()
	problemJSONDebug(w, http.StatusInternalServerError, "about:blank#internal-error", "Internal Server Error", "something went wrong", "internal error")

	rawBody := w.Body.String()
	assert.NotContains(t, rawBody, "debug", "JSON response must not contain debug key when disabled")
	assert.Contains(t, rawBody, "something went wrong")
}

func TestEnableDebugResponses_SetsFlag(t *testing.T) {
	// Ensure the flag starts false (default).
	assert.False(t, debugResponsesEnabled)

	EnableDebugResponses()
	assert.True(t, debugResponsesEnabled)

	// Clean up so other tests are not affected.
	t.Cleanup(func() { debugResponsesEnabled = false })
}
