package api

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAuthPublicRoutesPassThrough(t *testing.T) {
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	for _, route := range []string{"GetCertificate", "PutNKey"} {
		t.Run(route, func(t *testing.T) {
			handler := Auth(inner, route)
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)
			if rec.Code != http.StatusOK {
				t.Errorf("public route %s returned %d, want 200", route, rec.Code)
			}
		})
	}
}

func TestAuthNoTokenReturns401(t *testing.T) {
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	handler := Auth(inner, "Cook")
	req := httptest.NewRequest(http.MethodPost, "/cook", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("no-token request returned %d, want 401", rec.Code)
	}
}

func TestAuthBadTokenReturns403(t *testing.T) {
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	handler := Auth(inner, "Cook")
	req := httptest.NewRequest(http.MethodPost, "/cook", nil)
	req.Header.Set("Authorization", "invalid-token-data")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	// Should be 403 (forbidden) since the token is invalid
	if rec.Code != http.StatusForbidden {
		t.Errorf("bad-token request returned %d, want 403", rec.Code)
	}
}

func TestLoggerWrapsHandler(t *testing.T) {
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTeapot)
	})

	handler := Logger(inner, "TestRoute")
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusTeapot {
		t.Errorf("Logger wrapper changed status code: got %d, want 418", rec.Code)
	}
}
