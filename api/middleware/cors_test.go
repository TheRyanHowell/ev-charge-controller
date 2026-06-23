package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCorsHandler(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()

	CorsHandler(next).ServeHTTP(w, req)

	if w.Header().Get("Access-Control-Allow-Origin") != "http://localhost:3000" {
		t.Errorf("Expected Access-Control-Allow-Origin to be http://localhost:3000, got %s", w.Header().Get("Access-Control-Allow-Origin"))
	}

	if w.Header().Get("Access-Control-Allow-Methods") != "GET, POST, DELETE, PATCH, OPTIONS" {
		t.Errorf("Expected Access-Control-Allow-Methods, got %s", w.Header().Get("Access-Control-Allow-Methods"))
	}

	if w.Header().Get("Access-Control-Allow-Headers") != "Content-Type" {
		t.Errorf("Expected Access-Control-Allow-Headers to be Content-Type, got %s", w.Header().Get("Access-Control-Allow-Headers"))
	}
}

func TestCorsHandlerCustomOrigin(t *testing.T) {
	t.Setenv("CORS_ORIGIN", "http://example.com:3000")

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()

	CorsHandler(next).ServeHTTP(w, req)

	if w.Header().Get("Access-Control-Allow-Origin") != "http://example.com:3000" {
		t.Errorf("Expected custom origin, got %s", w.Header().Get("Access-Control-Allow-Origin"))
	}
}

func TestGetCorsOriginDefault(t *testing.T) {
	t.Setenv("CORS_ORIGIN", "")
	origin := GetCorsOrigin()
	if origin != defaultCorsOrigin {
		t.Errorf("Expected default origin %s, got %s", defaultCorsOrigin, origin)
	}
}

func TestGetCorsOriginCustom(t *testing.T) {
	t.Setenv("CORS_ORIGIN", "http://custom.example.com")
	origin := GetCorsOrigin()
	if origin != "http://custom.example.com" {
		t.Errorf("Expected http://custom.example.com, got %s", origin)
	}
}

func TestCorsHandlerOptionsPreflight(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("Handler should not be called for OPTIONS request")
	})

	req := httptest.NewRequest(http.MethodOptions, "/test", nil)
	w := httptest.NewRecorder()

	CorsHandler(next).ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected OPTIONS request to return 200 OK, got %d", w.Code)
	}
}

func TestCorsHandlerMuxOptionsPreflight(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/test", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	handler := CorsHandler(mux)

	req := httptest.NewRequest(http.MethodOptions, "/api/test", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected OPTIONS to return 200 (not 405 from mux), got %d", w.Code)
	}

	if w.Header().Get("Access-Control-Allow-Origin") != "http://localhost:3000" {
		t.Errorf("Expected CORS origin header, got %s", w.Header().Get("Access-Control-Allow-Origin"))
	}
}

func TestCorsHandlerPassthrough(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/test", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	handler := CorsHandler(mux)

	req := httptest.NewRequest(http.MethodGet, "/api/test", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected GET to pass through to mux, got %d", w.Code)
	}
}

func TestCorsHandlerMuxCustomOrigin(t *testing.T) {
	t.Setenv("CORS_ORIGIN", "http://192.168.1.100:3000")

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/test", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	handler := CorsHandler(mux)

	req := httptest.NewRequest(http.MethodOptions, "/api/test", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Header().Get("Access-Control-Allow-Origin") != "http://192.168.1.100:3000" {
		t.Errorf("Expected custom origin, got %s", w.Header().Get("Access-Control-Allow-Origin"))
	}
}

func TestCorsHandler_RejectsDisallowedOrigin(t *testing.T) {
	t.Setenv("CORS_ORIGIN", "http://localhost:3000")

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("Handler should not be called for disallowed origin")
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Origin", "http://evil.com")
	w := httptest.NewRecorder()

	CorsHandler(next).ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("Expected 403 for disallowed origin, got %d", w.Code)
	}
}

func TestCorsHandler_AllowsMatchingOrigin(t *testing.T) {
	t.Setenv("CORS_ORIGIN", "http://localhost:3000")

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Origin", "http://localhost:3000")
	w := httptest.NewRecorder()

	CorsHandler(next).ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected 200 for matching origin, got %d", w.Code)
	}
	if w.Header().Get("Access-Control-Allow-Origin") != "http://localhost:3000" {
		t.Errorf("Expected matching origin in header, got %s", w.Header().Get("Access-Control-Allow-Origin"))
	}
}

func TestCorsHandler_NoOriginHeader(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	// No Origin header set
	w := httptest.NewRecorder()

	CorsHandler(next).ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected 200 for no origin header, got %d", w.Code)
	}
	if w.Header().Get("Access-Control-Allow-Origin") != "http://localhost:3000" {
		t.Errorf("Expected default origin in header, got %s", w.Header().Get("Access-Control-Allow-Origin"))
	}
}
