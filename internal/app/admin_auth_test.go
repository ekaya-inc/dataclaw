package app

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestAdminAuthSigninSessionLogoutAndSignedCookie(t *testing.T) {
	auth := newTestAdminAuth(t)
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/auth/signin", auth.HandleSignIn)
	mux.HandleFunc("GET /api/auth/session", auth.HandleSession)
	mux.HandleFunc("POST /api/auth/logout", auth.HandleLogout)

	badOrigin := httptest.NewRequest(http.MethodPost, "http://dataclaw.local/api/auth/signin", strings.NewReader(`{"password":"secret"}`))
	badOrigin.Header.Set("Origin", "http://evil.local")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, badOrigin)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("cross-origin signin status = %d, want %d", rec.Code, http.StatusForbidden)
	}

	signin := httptest.NewRequest(http.MethodPost, "http://dataclaw.local/api/auth/signin", strings.NewReader(`{"password":"secret"}`))
	signin.Header.Set("Origin", "http://dataclaw.local")
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, signin)
	if rec.Code != http.StatusOK {
		t.Fatalf("signin status = %d body=%s", rec.Code, rec.Body.String())
	}
	cookie := rec.Result().Cookies()[0]
	if cookie.Name != adminSessionCookieName || !cookie.HttpOnly || cookie.SameSite != http.SameSiteLaxMode {
		t.Fatalf("unexpected cookie: %#v", cookie)
	}
	if cookie.Expires.IsZero() || !cookie.Expires.After(time.Unix(100, 0)) {
		t.Fatalf("expected bounded expiry after login time, got %#v", cookie.Expires)
	}
	if !strings.Contains(rec.Body.String(), `"authenticated":true`) || strings.Contains(rec.Body.String(), "secret") {
		t.Fatalf("signin body leaked or missed auth state: %s", rec.Body.String())
	}

	session := httptest.NewRequest(http.MethodGet, "http://dataclaw.local/api/auth/session", nil)
	session.AddCookie(cookie)
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, session)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"authenticated":true`) {
		t.Fatalf("session response = %d %s", rec.Code, rec.Body.String())
	}

	tampered := *cookie
	tampered.Value += "x"
	session = httptest.NewRequest(http.MethodGet, "http://dataclaw.local/api/auth/session", nil)
	session.AddCookie(&tampered)
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, session)
	if rec.Code != http.StatusUnauthorized || sessionCookieFrom(rec).MaxAge != -1 {
		t.Fatalf("tampered session response = %d cookies=%v body=%s", rec.Code, rec.Result().Cookies(), rec.Body.String())
	}

	logout := httptest.NewRequest(http.MethodPost, "http://dataclaw.local/api/auth/logout", nil)
	logout.AddCookie(cookie)
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, logout)
	if rec.Code != http.StatusOK {
		t.Fatalf("logout status = %d body=%s", rec.Code, rec.Body.String())
	}
	cleared := sessionCookieFrom(rec)
	if cleared.MaxAge != -1 {
		t.Fatalf("expected clearing cookie, got %#v", cleared)
	}
}

func TestAdminAuthRejectsExpiredUnsupportedAndInsecureCookies(t *testing.T) {
	auth := newTestAdminAuth(t)
	tests := []struct {
		name    string
		session adminSessionPayload
	}{
		{name: "expired", session: adminSessionPayload{Version: 1, Issued: 1, Expires: 99, SID: "sid", CSRF: "csrf-token"}},
		{name: "unsupported version", session: adminSessionPayload{Version: 2, Issued: 1, Expires: 200, SID: "sid", CSRF: "csrf-token"}},
		{name: "missing csrf", session: adminSessionPayload{Version: 1, Issued: 1, Expires: 200, SID: "sid"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			value, err := auth.signSession(tc.session)
			if err != nil {
				t.Fatalf("sign session: %v", err)
			}
			req := httptest.NewRequest(http.MethodGet, "http://dataclaw.local/api/auth/session", nil)
			req.AddCookie(&http.Cookie{Name: adminSessionCookieName, Value: value})
			rec := httptest.NewRecorder()
			auth.HandleSession(rec, req)
			if rec.Code != http.StatusUnauthorized || sessionCookieFrom(rec).MaxAge != -1 {
				t.Fatalf("session response = %d cookies=%v body=%s", rec.Code, rec.Result().Cookies(), rec.Body.String())
			}
		})
	}
}

func TestAdminAuthRememberUsesLongTTLAndHTTPSCookieSecureFlag(t *testing.T) {
	auth := newTestAdminAuth(t, AdminAuthOptions{AdminBaseURL: "https://dataclaw.local"})
	req := httptest.NewRequest(http.MethodPost, "https://dataclaw.local/api/auth/signin", strings.NewReader(`{"password":"secret","remember":true}`))
	req.Header.Set("Origin", "https://dataclaw.local")
	rec := httptest.NewRecorder()

	auth.HandleSignIn(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("signin status = %d body=%s", rec.Code, rec.Body.String())
	}
	cookie := sessionCookieFrom(rec)
	if cookie == nil || !cookie.Secure {
		t.Fatalf("expected secure session cookie, got %#v", cookie)
	}
	wantExpiry := time.Unix(100, 0).UTC().Add(2 * time.Hour)
	if !cookie.Expires.Equal(wantExpiry) {
		t.Fatalf("remember cookie expiry = %s, want %s", cookie.Expires, wantExpiry)
	}
}

func TestAdminAuthMiddlewareRequiresSessionAndSameOriginForUnsafeAPI(t *testing.T) {
	auth := newTestAdminAuth(t)
	encoded, err := auth.signSession(adminSessionPayload{Version: 1, Issued: 50, Expires: 200, SID: "sid", CSRF: "csrf-token"})
	if err != nil {
		t.Fatalf("sign session: %v", err)
	}
	protected := auth.RequireAdmin(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusNoContent) }))

	missing := httptest.NewRequest(http.MethodGet, "http://dataclaw.local/api/status", nil)
	missing.Header.Set("Accept", "application/json")
	rec := httptest.NewRecorder()
	protected.ServeHTTP(rec, missing)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("missing session status = %d", rec.Code)
	}

	htmlMissing := httptest.NewRequest(http.MethodGet, "http://dataclaw.local/", nil)
	htmlMissing.Header.Set("Accept", "text/html")
	rec = httptest.NewRecorder()
	protected.ServeHTTP(rec, htmlMissing)
	if rec.Code != http.StatusFound || rec.Header().Get("Location") != "/signin?next=%2F" {
		t.Fatalf("html redirect = %d location=%q", rec.Code, rec.Header().Get("Location"))
	}

	missingOrigin := httptest.NewRequest(http.MethodPost, "http://dataclaw.local/api/datasource", nil)
	missingOrigin.AddCookie(&http.Cookie{Name: adminSessionCookieName, Value: encoded})
	rec = httptest.NewRecorder()
	protected.ServeHTTP(rec, missingOrigin)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("missing origin status = %d", rec.Code)
	}

	crossOrigin := httptest.NewRequest(http.MethodPost, "http://dataclaw.local/api/datasource", nil)
	crossOrigin.Header.Set("Origin", "http://evil.local")
	crossOrigin.AddCookie(&http.Cookie{Name: adminSessionCookieName, Value: encoded})
	rec = httptest.NewRecorder()
	protected.ServeHTTP(rec, crossOrigin)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("cross origin status = %d", rec.Code)
	}

	allowed := httptest.NewRequest(http.MethodPost, "http://dataclaw.local/api/datasource", nil)
	allowed.Header.Set("Origin", "http://dataclaw.local")
	allowed.AddCookie(&http.Cookie{Name: adminSessionCookieName, Value: encoded})
	rec = httptest.NewRecorder()
	protected.ServeHTTP(rec, allowed)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("missing csrf status = %d body=%s", rec.Code, rec.Body.String())
	}

	badCSRF := httptest.NewRequest(http.MethodPost, "http://dataclaw.local/api/datasource", nil)
	badCSRF.Header.Set("Origin", "http://dataclaw.local")
	badCSRF.Header.Set("X-CSRF-Token", "wrong")
	badCSRF.AddCookie(&http.Cookie{Name: adminSessionCookieName, Value: encoded})
	rec = httptest.NewRecorder()
	protected.ServeHTTP(rec, badCSRF)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("bad csrf status = %d body=%s", rec.Code, rec.Body.String())
	}

	allowed = httptest.NewRequest(http.MethodPost, "http://dataclaw.local/api/datasource", nil)
	allowed.Header.Set("Origin", "http://dataclaw.local")
	allowed.Header.Set("X-CSRF-Token", "csrf-token")
	allowed.AddCookie(&http.Cookie{Name: adminSessionCookieName, Value: encoded})
	rec = httptest.NewRecorder()
	protected.ServeHTTP(rec, allowed)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("allowed status = %d body=%s", rec.Code, rec.Body.String())
	}

	refererAllowed := httptest.NewRequest(http.MethodDelete, "http://dataclaw.local/api/datasource", nil)
	refererAllowed.Header.Set("Referer", "http://dataclaw.local/datasource")
	refererAllowed.Header.Set("X-CSRF-Token", "csrf-token")
	refererAllowed.AddCookie(&http.Cookie{Name: adminSessionCookieName, Value: encoded})
	rec = httptest.NewRecorder()
	protected.ServeHTTP(rec, refererAllowed)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("referer allowed status = %d body=%s", rec.Code, rec.Body.String())
	}
}

func newTestAdminAuth(t *testing.T, overrides ...AdminAuthOptions) *AdminAuth {
	t.Helper()
	opts := AdminAuthOptions{
		Password:       "secret",
		Secret:         []byte("0123456789abcdef0123456789abcdef"),
		AdminBaseURL:   "http://dataclaw.local",
		SessionTTL:     time.Hour,
		LongSessionTTL: 2 * time.Hour,
		Now:            func() time.Time { return time.Unix(100, 0).UTC() },
		Random:         strings.NewReader("0123456789abcdef0123456789abcdef"),
	}
	if len(overrides) > 0 {
		override := overrides[0]
		if override.Password != "" {
			opts.Password = override.Password
		}
		if len(override.Secret) > 0 {
			opts.Secret = override.Secret
		}
		if override.AdminBaseURL != "" {
			opts.AdminBaseURL = override.AdminBaseURL
		}
		if override.SessionTTL != 0 {
			opts.SessionTTL = override.SessionTTL
		}
		if override.LongSessionTTL != 0 {
			opts.LongSessionTTL = override.LongSessionTTL
		}
		if override.Now != nil {
			opts.Now = override.Now
		}
		if override.Random != nil {
			opts.Random = override.Random
		}
	}
	auth, err := NewAdminAuth(opts)
	if err != nil {
		t.Fatalf("NewAdminAuth: %v", err)
	}
	return auth
}

func sessionCookieFrom(rec *httptest.ResponseRecorder) *http.Cookie {
	for _, cookie := range rec.Result().Cookies() {
		if cookie.Name == adminSessionCookieName {
			return cookie
		}
	}
	return nil
}
