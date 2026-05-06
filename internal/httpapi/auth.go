package httpapi

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"
)

const (
	adminSessionCookie = "dataclaw_admin_session"
	adminSessionTTL    = 12 * time.Hour
	csrfHeader         = "X-CSRF-Token"
)

type AdminAuthOptions struct {
	Password      string
	SessionKey    []byte
	AllowedOrigin string
	CookieSecure  bool
}

type adminAuth struct {
	password      string
	signer        sessionSigner
	allowedOrigin string
	cookieSecure  bool
}

type sessionSigner struct{ key []byte }

func newAdminAuth(opts AdminAuthOptions) *adminAuth {
	key := append([]byte(nil), opts.SessionKey...)
	if len(key) == 0 {
		key = make([]byte, 32)
		_, _ = rand.Read(key)
	}
	return &adminAuth{password: opts.Password, signer: sessionSigner{key: key}, allowedOrigin: strings.TrimRight(opts.AllowedOrigin, "/"), cookieSecure: opts.CookieSecure}
}

func (a *adminAuth) handleSignin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, response{Error: "method not allowed"})
		return
	}
	var req struct {
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, response{Error: "invalid request body"})
		return
	}
	if a.password == "" || subtle.ConstantTimeCompare([]byte(req.Password), []byte(a.password)) != 1 {
		writeJSON(w, http.StatusUnauthorized, response{Error: "invalid credentials"})
		return
	}
	expires := time.Now().Add(adminSessionTTL)
	token := a.signer.sign(expires)
	http.SetCookie(w, &http.Cookie{Name: adminSessionCookie, Value: token, Path: "/", Expires: expires, MaxAge: int(time.Until(expires).Seconds()), HttpOnly: true, SameSite: http.SameSiteLaxMode, Secure: a.cookieSecure})
	writeJSON(w, http.StatusOK, response{Success: true, Data: map[string]any{"authenticated": true, "csrf_token": a.signer.csrfToken(token), "expires_at": expires.UTC().Format(time.RFC3339)}})
}

func (a *adminAuth) handleLogout(w http.ResponseWriter, _ *http.Request) {
	http.SetCookie(w, &http.Cookie{Name: adminSessionCookie, Value: "", Path: "/", MaxAge: -1, HttpOnly: true, SameSite: http.SameSiteLaxMode, Secure: a.cookieSecure})
	writeJSON(w, http.StatusOK, response{Success: true, Data: map[string]any{"authenticated": false}})
}

func (a *adminAuth) handleSession(w http.ResponseWriter, r *http.Request) {
	token, ok := a.sessionToken(r)
	if !ok {
		writeJSON(w, http.StatusOK, response{Success: true, Data: map[string]any{"authenticated": false}})
		return
	}
	writeJSON(w, http.StatusOK, response{Success: true, Data: map[string]any{"authenticated": true, "csrf_token": a.signer.csrfToken(token)}})
}

func (a *adminAuth) requireSession(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token, ok := a.sessionToken(r)
		if !ok {
			writeJSON(w, http.StatusUnauthorized, response{Error: "admin authentication required"})
			return
		}
		if isUnsafeMethod(r.Method) {
			if !a.sameOrigin(r) {
				writeJSON(w, http.StatusForbidden, response{Error: "origin not allowed"})
				return
			}
			if !hmac.Equal([]byte(r.Header.Get(csrfHeader)), []byte(a.signer.csrfToken(token))) {
				writeJSON(w, http.StatusForbidden, response{Error: "invalid csrf token"})
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

func (a *adminAuth) sessionToken(r *http.Request) (string, bool) {
	cookie, err := r.Cookie(adminSessionCookie)
	if err != nil || cookie.Value == "" {
		return "", false
	}
	if err := a.signer.verify(cookie.Value, time.Now()); err != nil {
		return "", false
	}
	return cookie.Value, true
}

func (a *adminAuth) sameOrigin(r *http.Request) bool {
	origin := strings.TrimRight(r.Header.Get("Origin"), "/")
	if origin == "" {
		return false
	}
	if a.allowedOrigin != "" {
		return origin == a.allowedOrigin
	}
	return origin == "http://"+r.Host || origin == "https://"+r.Host
}

func isUnsafeMethod(method string) bool {
	switch method {
	case http.MethodGet, http.MethodHead, http.MethodOptions, http.MethodTrace:
		return false
	default:
		return true
	}
}

func (s sessionSigner) sign(expires time.Time) string {
	payload := []byte(expires.UTC().Format(time.RFC3339Nano))
	encoded := base64.RawURLEncoding.EncodeToString(payload)
	mac := hmac.New(sha256.New, s.key)
	_, _ = mac.Write([]byte(encoded))
	return encoded + "." + hex.EncodeToString(mac.Sum(nil))
}

func (s sessionSigner) verify(token string, now time.Time) error {
	encoded, sig, ok := strings.Cut(token, ".")
	if !ok || encoded == "" || sig == "" {
		return errors.New("invalid session")
	}
	mac := hmac.New(sha256.New, s.key)
	_, _ = mac.Write([]byte(encoded))
	want := hex.EncodeToString(mac.Sum(nil))
	if !hmac.Equal([]byte(sig), []byte(want)) {
		return errors.New("invalid session signature")
	}
	raw, err := base64.RawURLEncoding.DecodeString(encoded)
	if err != nil {
		return err
	}
	expires, err := time.Parse(time.RFC3339Nano, string(raw))
	if err != nil {
		return err
	}
	if !expires.After(now) {
		return errors.New("expired session")
	}
	return nil
}

func (s sessionSigner) csrfToken(session string) string {
	mac := hmac.New(sha256.New, s.key)
	_, _ = mac.Write([]byte("csrf:" + session))
	return hex.EncodeToString(mac.Sum(nil))
}
