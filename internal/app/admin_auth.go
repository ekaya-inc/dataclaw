package app

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	adminSessionCookieName = "dataclaw_admin_session"
	adminSessionPurpose    = "dataclaw-admin-session-v1"
	defaultAdminPassword   = "admin"
	defaultAdminSessionTTL = 12 * time.Hour
	defaultAdminLongTTL    = 30 * 24 * time.Hour
	maxAdminLongTTL        = 90 * 24 * time.Hour
)

type AdminAuthOptions struct {
	Password       string
	Secret         []byte
	AdminBaseURL   string
	SessionTTL     time.Duration
	LongSessionTTL time.Duration
	Now            func() time.Time
	Random         io.Reader
}

type AdminAuth struct {
	password       string
	key            []byte
	adminOrigin    string
	cookieSecure   bool
	sessionTTL     time.Duration
	longSessionTTL time.Duration
	now            func() time.Time
	random         io.Reader
}

type adminSessionPayload struct {
	Version int    `json:"v"`
	Issued  int64  `json:"iat"`
	Expires int64  `json:"exp"`
	SID     string `json:"sid"`
	Long    bool   `json:"long,omitempty"`
	CSRF    string `json:"csrf"`
}

type adminAuthResponse struct {
	Success bool   `json:"success"`
	Data    any    `json:"data,omitempty"`
	Error   string `json:"error,omitempty"`
}

func NewAdminAuth(opts AdminAuthOptions) (*AdminAuth, error) {
	if opts.Password == "" {
		return nil, errors.New("admin password is required")
	}
	if len(opts.Secret) == 0 {
		return nil, errors.New("admin session secret is required")
	}
	sessionTTL := opts.SessionTTL
	if sessionTTL == 0 {
		sessionTTL = defaultAdminSessionTTL
	}
	if sessionTTL <= 0 {
		return nil, errors.New("admin session ttl must be positive")
	}
	longSessionTTL := opts.LongSessionTTL
	if longSessionTTL == 0 {
		longSessionTTL = defaultAdminLongTTL
	}
	if longSessionTTL <= 0 {
		return nil, errors.New("admin long session ttl must be positive")
	}
	if longSessionTTL > maxAdminLongTTL {
		return nil, fmt.Errorf("admin long session ttl must be <= %s", maxAdminLongTTL)
	}
	if longSessionTTL < sessionTTL {
		return nil, errors.New("admin long session ttl must be >= admin session ttl")
	}
	parsedBaseURL, err := url.Parse(opts.AdminBaseURL)
	if err != nil || parsedBaseURL.Scheme == "" || parsedBaseURL.Host == "" {
		return nil, fmt.Errorf("admin base url must be absolute: %q", opts.AdminBaseURL)
	}
	if parsedBaseURL.Scheme != "http" && parsedBaseURL.Scheme != "https" {
		return nil, fmt.Errorf("admin base url must use http or https: %q", opts.AdminBaseURL)
	}
	now := opts.Now
	if now == nil {
		now = func() time.Time { return time.Now().UTC() }
	}
	random := opts.Random
	if random == nil {
		random = rand.Reader
	}
	return &AdminAuth{
		password:       opts.Password,
		key:            deriveAdminSessionKey(opts.Secret),
		adminOrigin:    (&url.URL{Scheme: parsedBaseURL.Scheme, Host: parsedBaseURL.Host}).String(),
		cookieSecure:   parsedBaseURL.Scheme == "https",
		sessionTTL:     sessionTTL,
		longSessionTTL: longSessionTTL,
		now:            now,
		random:         random,
	}, nil
}

func (a *AdminAuth) HandleSignIn(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		writeAdminAuthJSON(w, http.StatusMethodNotAllowed, adminAuthResponse{Success: false, Error: "method not allowed"})
		return
	}
	if !a.requestOriginAllowed(r) {
		writeAdminAuthJSON(w, http.StatusForbidden, adminAuthResponse{Success: false, Error: "forbidden"})
		return
	}
	var payload struct {
		Password string `json:"password"`
		Remember bool   `json:"remember"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&payload); err != nil {
		writeAdminAuthJSON(w, http.StatusBadRequest, adminAuthResponse{Success: false, Error: "invalid json"})
		return
	}
	if !a.passwordMatches(payload.Password) {
		writeAdminAuthJSON(w, http.StatusUnauthorized, adminAuthResponse{Success: false, Error: "unauthorized"})
		return
	}
	session, err := a.newSession(payload.Remember)
	if err != nil {
		writeAdminAuthJSON(w, http.StatusInternalServerError, adminAuthResponse{Success: false, Error: "create session"})
		return
	}
	value, err := a.signSession(session)
	if err != nil {
		writeAdminAuthJSON(w, http.StatusInternalServerError, adminAuthResponse{Success: false, Error: "create session"})
		return
	}
	http.SetCookie(w, a.sessionCookie(value, time.Unix(session.Expires, 0).UTC()))
	writeAdminAuthJSON(w, http.StatusOK, adminAuthResponse{Success: true, Data: map[string]any{
		"authenticated": true,
		"csrf_token":    session.CSRF,
		"expires_at":    time.Unix(session.Expires, 0).UTC().Format(time.RFC3339),
	}})
}

func (a *AdminAuth) HandleLogout(w http.ResponseWriter, _ *http.Request) {
	http.SetCookie(w, a.expiredCookie())
	writeAdminAuthJSON(w, http.StatusOK, adminAuthResponse{Success: true, Data: map[string]any{"authenticated": false}})
}

func (a *AdminAuth) HandleSession(w http.ResponseWriter, r *http.Request) {
	session, ok := a.Session(r)
	if !ok {
		http.SetCookie(w, a.expiredCookie())
		writeAdminAuthJSON(w, http.StatusUnauthorized, adminAuthResponse{Success: false, Error: "unauthorized"})
		return
	}
	writeAdminAuthJSON(w, http.StatusOK, adminAuthResponse{Success: true, Data: map[string]any{
		"authenticated": true,
		"csrf_token":    session.CSRF,
		"expires_at":    time.Unix(session.Expires, 0).UTC().Format(time.RFC3339),
	}})
}

func (a *AdminAuth) RequireAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		session, ok := a.Session(r)
		if !ok {
			http.SetCookie(w, a.expiredCookie())
			if strings.HasPrefix(r.URL.Path, "/api/") || !wantsHTML(r) {
				writeAdminAuthJSON(w, http.StatusUnauthorized, adminAuthResponse{Success: false, Error: "unauthorized"})
				return
			}
			nextPath := r.URL.RequestURI()
			if nextPath == "" {
				nextPath = "/"
			}
			http.Redirect(w, r, "/signin?next="+url.QueryEscape(nextPath), http.StatusFound)
			return
		}
		if isUnsafeMethod(r.Method) && strings.HasPrefix(r.URL.Path, "/api/") {
			if !a.requestOriginAllowed(r) {
				writeAdminAuthJSON(w, http.StatusForbidden, adminAuthResponse{Success: false, Error: "forbidden"})
				return
			}
			if subtle.ConstantTimeCompare([]byte(r.Header.Get("X-CSRF-Token")), []byte(session.CSRF)) != 1 {
				writeAdminAuthJSON(w, http.StatusForbidden, adminAuthResponse{Success: false, Error: "csrf token required"})
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

func (a *AdminAuth) RedirectIfAuthenticated(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, ok := a.Session(r); ok {
			http.Redirect(w, r, "/", http.StatusFound)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (a *AdminAuth) Session(r *http.Request) (adminSessionPayload, bool) {
	cookie, err := r.Cookie(adminSessionCookieName)
	if err != nil {
		return adminSessionPayload{}, false
	}
	session, err := a.verifySession(cookie.Value)
	if err != nil {
		return adminSessionPayload{}, false
	}
	if session.Version != 1 || session.Expires <= a.now().Unix() {
		return adminSessionPayload{}, false
	}
	return session, true
}

func (a *AdminAuth) passwordMatches(provided string) bool {
	want := sha256.Sum256([]byte(a.password))
	got := sha256.Sum256([]byte(provided))
	return subtle.ConstantTimeCompare(want[:], got[:]) == 1
}

func (a *AdminAuth) newSession(remember bool) (adminSessionPayload, error) {
	now := a.now().UTC()
	ttl := a.sessionTTL
	if remember {
		ttl = a.longSessionTTL
	}
	nonce := make([]byte, 16)
	if _, err := io.ReadFull(a.random, nonce); err != nil {
		return adminSessionPayload{}, err
	}
	csrf := make([]byte, 16)
	if _, err := io.ReadFull(a.random, csrf); err != nil {
		return adminSessionPayload{}, err
	}
	return adminSessionPayload{
		Version: 1,
		Issued:  now.Unix(),
		Expires: now.Add(ttl).Unix(),
		SID:     base64.RawURLEncoding.EncodeToString(nonce),
		Long:    remember,
		CSRF:    base64.RawURLEncoding.EncodeToString(csrf),
	}, nil
}

func (a *AdminAuth) signSession(session adminSessionPayload) (string, error) {
	payload, err := json.Marshal(session)
	if err != nil {
		return "", err
	}
	encodedPayload := base64.RawURLEncoding.EncodeToString(payload)
	signature := a.sessionSignature(encodedPayload)
	return encodedPayload + "." + signature, nil
}

func (a *AdminAuth) verifySession(value string) (adminSessionPayload, error) {
	encodedPayload, encodedSignature, ok := strings.Cut(value, ".")
	if !ok || encodedPayload == "" || encodedSignature == "" {
		return adminSessionPayload{}, errors.New("malformed session")
	}
	want := a.sessionSignature(encodedPayload)
	if subtle.ConstantTimeCompare([]byte(want), []byte(encodedSignature)) != 1 {
		return adminSessionPayload{}, errors.New("invalid session signature")
	}
	payload, err := base64.RawURLEncoding.DecodeString(encodedPayload)
	if err != nil {
		return adminSessionPayload{}, err
	}
	var session adminSessionPayload
	if err := json.Unmarshal(payload, &session); err != nil {
		return adminSessionPayload{}, err
	}
	return session, nil
}

func (a *AdminAuth) sessionSignature(encodedPayload string) string {
	mac := hmac.New(sha256.New, a.key)
	_, _ = mac.Write([]byte(encodedPayload))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

func (a *AdminAuth) sessionCookie(value string, expires time.Time) *http.Cookie {
	return &http.Cookie{
		Name:     adminSessionCookieName,
		Value:    value,
		Path:     "/",
		Expires:  expires,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   a.cookieSecure,
	}
}

func (a *AdminAuth) expiredCookie() *http.Cookie {
	return &http.Cookie{
		Name:     adminSessionCookieName,
		Value:    "",
		Path:     "/",
		Expires:  time.Unix(0, 0).UTC(),
		MaxAge:   -1,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   a.cookieSecure,
	}
}

func (a *AdminAuth) requestOriginAllowed(r *http.Request) bool {
	if !isUnsafeMethod(r.Method) {
		return true
	}
	if origin := strings.TrimSpace(r.Header.Get("Origin")); origin != "" {
		return sameOrigin(origin, a.adminOrigin)
	}
	if referer := strings.TrimSpace(r.Header.Get("Referer")); referer != "" {
		return sameOrigin(referer, a.adminOrigin)
	}
	return false
}

func deriveAdminSessionKey(secret []byte) []byte {
	mac := hmac.New(sha256.New, secret)
	_, _ = mac.Write([]byte(adminSessionPurpose))
	return mac.Sum(nil)
}

func sameOrigin(candidate, expectedOrigin string) bool {
	parsed, err := url.Parse(candidate)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return false
	}
	return (&url.URL{Scheme: parsed.Scheme, Host: parsed.Host}).String() == expectedOrigin
}

func isUnsafeMethod(method string) bool {
	switch method {
	case http.MethodGet, http.MethodHead, http.MethodOptions, http.MethodTrace:
		return false
	default:
		return true
	}
}

func wantsHTML(r *http.Request) bool {
	accept := r.Header.Get("Accept")
	return accept == "" || strings.Contains(accept, "text/html") || strings.Contains(accept, "*/*")
}

func writeAdminAuthJSON(w http.ResponseWriter, status int, payload adminAuthResponse) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}
