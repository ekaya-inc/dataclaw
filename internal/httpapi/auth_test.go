package httpapi

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestAdminAuthSessionAndCSRFFlow(t *testing.T) {
	api := newTestAPI(t)
	mux := http.NewServeMux()
	api.RegisterAdmin(mux, AdminAuthOptions{Password: "secret", SessionKey: []byte("01234567890123456789012345678901"), AllowedOrigin: "http://admin.local"})

	unauth := httptest.NewRecorder()
	mux.ServeHTTP(unauth, httptest.NewRequest(http.MethodGet, "/api/status", nil))
	if unauth.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated status = %d, want %d", unauth.Code, http.StatusUnauthorized)
	}

	bad := httptest.NewRecorder()
	mux.ServeHTTP(bad, httptest.NewRequest(http.MethodPost, "/api/auth/signin", strings.NewReader(`{"password":"bad"}`)))
	if bad.Code != http.StatusUnauthorized {
		t.Fatalf("bad signin status = %d, want %d", bad.Code, http.StatusUnauthorized)
	}

	signin := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/auth/signin", strings.NewReader(`{"password":"secret"}`))
	mux.ServeHTTP(signin, req)
	if signin.Code != http.StatusOK {
		t.Fatalf("signin status = %d, want %d: %s", signin.Code, http.StatusOK, signin.Body.String())
	}
	cookie := sessionCookie(t, signin)
	if !cookie.HttpOnly || cookie.SameSite != http.SameSiteLaxMode {
		t.Fatalf("session cookie security flags = HttpOnly:%v SameSite:%v", cookie.HttpOnly, cookie.SameSite)
	}
	csrf := csrfFromBody(t, signin.Body.Bytes())

	session := httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/api/auth/session", nil)
	req.AddCookie(cookie)
	mux.ServeHTTP(session, req)
	if session.Code != http.StatusOK || !strings.Contains(session.Body.String(), `"authenticated":true`) {
		t.Fatalf("session response = %d %s", session.Code, session.Body.String())
	}

	missingCSRF := httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/api/queries/validate", strings.NewReader(`{"sql_query":"SELECT 1"}`))
	req.AddCookie(cookie)
	req.Header.Set("Origin", "http://admin.local")
	mux.ServeHTTP(missingCSRF, req)
	if missingCSRF.Code != http.StatusForbidden {
		t.Fatalf("missing csrf status = %d, want %d", missingCSRF.Code, http.StatusForbidden)
	}

	badOrigin := httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/api/queries/validate", bytes.NewBufferString(`{"sql_query":"SELECT 1"}`))
	req.AddCookie(cookie)
	req.Header.Set("Origin", "http://evil.local")
	req.Header.Set(csrfHeader, csrf)
	mux.ServeHTTP(badOrigin, req)
	if badOrigin.Code != http.StatusForbidden {
		t.Fatalf("bad origin status = %d, want %d", badOrigin.Code, http.StatusForbidden)
	}

	ok := httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/api/queries/validate", bytes.NewBufferString(`{"sql_query":"SELECT 1"}`))
	req.AddCookie(cookie)
	req.Header.Set("Origin", "http://admin.local")
	req.Header.Set(csrfHeader, csrf)
	mux.ServeHTTP(ok, req)
	if ok.Code != http.StatusOK {
		t.Fatalf("csrf protected request status = %d, want %d: %s", ok.Code, http.StatusOK, ok.Body.String())
	}

	logout := httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/api/auth/logout", nil)
	mux.ServeHTTP(logout, req)
	if logout.Code != http.StatusOK || sessionCookie(t, logout).MaxAge != -1 {
		t.Fatalf("logout response = %d cookies=%v", logout.Code, logout.Result().Cookies())
	}
}

func TestAdminRouteMatrixSeparatesPublicAuthAndProtectedAPI(t *testing.T) {
	api := newTestAPI(t)
	mux := http.NewServeMux()
	api.RegisterAdmin(mux, AdminAuthOptions{Password: "secret", SessionKey: []byte("01234567890123456789012345678901")})

	tests := []struct {
		name string
		meth string
		path string
		want int
	}{
		{"ping remains public", http.MethodGet, "/ping", http.StatusOK},
		{"signin route public", http.MethodPost, "/api/auth/signin", http.StatusBadRequest},
		{"session route public", http.MethodGet, "/api/auth/session", http.StatusOK},
		{"api protected", http.MethodGet, "/api/status", http.StatusUnauthorized},
		{"mcp not on admin mux", http.MethodPost, "/mcp", http.StatusNotFound},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, httptest.NewRequest(tc.meth, tc.path, nil))
			if rec.Code != tc.want {
				t.Fatalf("status = %d, want %d", rec.Code, tc.want)
			}
		})
	}
}

func sessionCookie(t *testing.T, rec *httptest.ResponseRecorder) *http.Cookie {
	t.Helper()
	for _, c := range rec.Result().Cookies() {
		if c.Name == adminSessionCookie {
			return c
		}
	}
	t.Fatalf("missing %s cookie", adminSessionCookie)
	return nil
}

func csrfFromBody(t *testing.T, raw []byte) string {
	t.Helper()
	var payload struct {
		Data struct {
			CSRFToken string `json:"csrf_token"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatalf("unmarshal signin: %v", err)
	}
	if payload.Data.CSRFToken == "" {
		t.Fatal("missing csrf_token")
	}
	return payload.Data.CSRFToken
}
