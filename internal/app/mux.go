package app

import (
	"io/fs"
	"net/http"
	"strings"

	"github.com/ekaya-inc/dataclaw/internal/httpapi"
	mcpserver "github.com/ekaya-inc/dataclaw/internal/mcpserver"
)

func BuildAdminMux(api *httpapi.API, auth *AdminAuth, uiFS fs.FS) http.Handler {
	apiMux := http.NewServeMux()
	api.Register(apiMux)

	mux := http.NewServeMux()
	mux.Handle("GET /ping", apiMux)
	mux.Handle("HEAD /ping", apiMux)
	mux.Handle("GET /bundles/", apiMux)
	mux.HandleFunc("POST /api/auth/signin", auth.HandleSignIn)
	mux.HandleFunc("POST /api/auth/logout", auth.HandleLogout)
	mux.HandleFunc("GET /api/auth/session", auth.HandleSession)
	mux.Handle("/api/", auth.RequireAdmin(apiMux))
	mux.Handle("GET /signin", auth.RedirectIfAuthenticated(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		serveUIIndex(w, uiFS)
	})))
	registerPublicUIAssetRoutes(mux, uiFS)
	mux.Handle("/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/mcp") {
			http.NotFound(w, r)
			return
		}
		auth.RequireAdmin(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			serveUIRoute(w, r, uiFS)
		})).ServeHTTP(w, r)
	}))
	return mux
}

func BuildMCPMux(mcpSrv *mcpserver.Server) *http.ServeMux {
	mux := http.NewServeMux()
	mux.Handle("/mcp", mcpSrv.Handler())
	mux.Handle("/mcp/", mcpSrv.Handler())
	return mux
}

func registerPublicUIAssetRoutes(mux *http.ServeMux, uiFS fs.FS) {
	for _, prefix := range []string{"/assets/", "/icons/"} {
		prefix := prefix
		mux.Handle("GET "+prefix, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			serveUIRoute(w, r, uiFS)
		}))
	}
	for _, path := range []string{"/favicon.ico", "/manifest.webmanifest", "/logo.svg"} {
		path := path
		mux.Handle("GET "+path, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			serveUIRoute(w, r, uiFS)
		}))
	}
}

func serveUIRoute(w http.ResponseWriter, r *http.Request, uiFS fs.FS) {
	if strings.HasPrefix(r.URL.Path, "/api/") || strings.HasPrefix(r.URL.Path, "/mcp") {
		http.NotFound(w, r)
		return
	}
	fileServer := http.FileServer(http.FS(uiFS))
	path := strings.TrimPrefix(r.URL.Path, "/")
	if path == "" {
		path = "index.html"
	}
	if _, err := fs.Stat(uiFS, path); err == nil {
		fileServer.ServeHTTP(w, r)
		return
	}
	serveUIIndex(w, uiFS)
}

func serveUIIndex(w http.ResponseWriter, uiFS fs.FS) {
	indexHTML, err := fs.ReadFile(uiFS, "index.html")
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err != nil {
		http.Error(w, "index.html missing", http.StatusInternalServerError)
		return
	}
	_, _ = w.Write(indexHTML)
}
