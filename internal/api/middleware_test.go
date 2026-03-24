package api

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/taigrr/jety"
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

func TestAuthDangerouslyAllowRootBypassesAuth(t *testing.T) {
	jety.Set("dangerously_allow_root", true)
	t.Cleanup(func() { jety.Set("dangerously_allow_root", false) })

	called := false
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})

	handler := Auth(inner, "FileServer")
	req := httptest.NewRequest(http.MethodGet, "/files/test", nil)
	// No Authorization header — should still pass with dangerously_allow_root.
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("dangerously_allow_root bypass returned %d, want 200", rec.Code)
	}
	if !called {
		t.Error("inner handler was not called despite dangerously_allow_root=true")
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
