package app

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	dsadapter "github.com/ekaya-inc/dataclaw/internal/adapters/datasource"
	"github.com/ekaya-inc/dataclaw/internal/core"
	"github.com/ekaya-inc/dataclaw/internal/httpapi"
	mcpserver "github.com/ekaya-inc/dataclaw/internal/mcpserver"
	"github.com/ekaya-inc/dataclaw/internal/security"
	storepkg "github.com/ekaya-inc/dataclaw/internal/store"
	"github.com/ekaya-inc/dataclaw/migrations"
)

func TestBuildAdminMuxRouteMatrix(t *testing.T) {
	api, _ := newAppMuxTestAPI(t)
	auth := newTestAdminAuth(t)
	uiFS := fstest.MapFS{
		"index.html":    {Data: []byte("<!doctype html><html>spa</html>")},
		"assets/app.js": {Data: []byte("console.log('ready')")},
	}
	mux := BuildAdminMux(api, auth, uiFS)

	tests := []struct {
		name       string
		method     string
		path       string
		accept     string
		wantStatus int
		wantHeader string
	}{
		{name: "ping public", method: http.MethodGet, path: "/ping", wantStatus: http.StatusOK},
		{name: "api status requires admin session", method: http.MethodGet, path: "/api/status", wantStatus: http.StatusUnauthorized, wantHeader: "application/json"},
		{name: "html navigation redirects to signin", method: http.MethodGet, path: "/", accept: "text/html", wantStatus: http.StatusFound},
		{name: "signin shell public", method: http.MethodGet, path: "/signin", accept: "text/html", wantStatus: http.StatusOK, wantHeader: "text/html"},
		{name: "signin asset public", method: http.MethodGet, path: "/assets/app.js", wantStatus: http.StatusOK},
		{name: "bundle without capability code denied by bundle semantics", method: http.MethodGet, path: "/bundles/missing", wantStatus: http.StatusNotFound},
		{name: "mcp route excluded from admin ui fallback", method: http.MethodPost, path: "/mcp", wantStatus: http.StatusNotFound},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, "http://admin.local"+tc.path, nil)
			if tc.accept != "" {
				req.Header.Set("Accept", tc.accept)
			}
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, req)
			if rec.Code != tc.wantStatus {
				t.Fatalf("status = %d, want %d; location=%q body=%s", rec.Code, tc.wantStatus, rec.Header().Get("Location"), rec.Body.String())
			}
			if tc.path == "/" && rec.Header().Get("Location") != "/signin?next=%2F" {
				t.Fatalf("redirect location = %q", rec.Header().Get("Location"))
			}
			if tc.wantHeader != "" && !strings.Contains(rec.Header().Get("Content-Type"), tc.wantHeader) {
				t.Fatalf("Content-Type = %q, want %q", rec.Header().Get("Content-Type"), tc.wantHeader)
			}
		})
	}
}

func TestBuildMCPMuxFailClosedRouteMatrix(t *testing.T) {
	service, _ := newAppMuxTestService(t)
	mux := BuildMCPMux(mcpserver.New("test", service))

	tests := []struct {
		name       string
		method     string
		path       string
		wantStatus int
	}{
		{name: "mcp requires API key", method: http.MethodPost, path: "/mcp", wantStatus: http.StatusUnauthorized},
		{name: "mcp slash requires API key", method: http.MethodPost, path: "/mcp/", wantStatus: http.StatusUnauthorized},
		{name: "admin api not mounted", method: http.MethodGet, path: "/api/status", wantStatus: http.StatusNotFound},
		{name: "admin mutation route not mounted", method: http.MethodPost, path: "/api/datasource/test", wantStatus: http.StatusNotFound},
		{name: "root not mounted", method: http.MethodGet, path: "/", wantStatus: http.StatusNotFound},
		{name: "signin not mounted", method: http.MethodGet, path: "/signin", wantStatus: http.StatusNotFound},
		{name: "static assets not mounted", method: http.MethodGet, path: "/assets/app.js", wantStatus: http.StatusNotFound},
		{name: "bundles not mounted", method: http.MethodGet, path: "/bundles/agent?code=valid", wantStatus: http.StatusNotFound},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, "http://mcp.local"+tc.path, nil)
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, req)
			if rec.Code != tc.wantStatus {
				t.Fatalf("status = %d, want %d; body=%s", rec.Code, tc.wantStatus, rec.Body.String())
			}
		})
	}
}

func newAppMuxTestAPI(t *testing.T) (*httpapi.API, *core.Service) {
	t.Helper()
	service, _ := newAppMuxTestService(t)
	return httpapi.New(service), service
}

func newAppMuxTestService(t *testing.T) (*core.Service, *storepkg.Store) {
	t.Helper()
	ctx := context.Background()
	st, err := storepkg.Open(ctx, filepath.Join(t.TempDir(), "dataclaw.sqlite"), migrations.FS)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	secret, err := security.LoadOrCreateSecret(filepath.Join(t.TempDir(), "secret.key"))
	if err != nil {
		t.Fatalf("load secret: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	return core.NewWithBaseURLs(
		st,
		secret,
		"test",
		func() string { return "http://admin.local" },
		func() string { return "http://mcp.local" },
		dsadapter.NewFactory(dsadapter.DefaultRegistry()),
	), st
}
