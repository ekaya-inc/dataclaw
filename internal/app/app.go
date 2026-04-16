package app

import (
	"context"
	"fmt"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/ekaya-inc/dataclaw/internal/config"
	"github.com/ekaya-inc/dataclaw/internal/core"
	"github.com/ekaya-inc/dataclaw/internal/httpapi"
	mcpserver "github.com/ekaya-inc/dataclaw/internal/mcpserver"
	"github.com/ekaya-inc/dataclaw/internal/runtime"
	"github.com/ekaya-inc/dataclaw/internal/security"
	storepkg "github.com/ekaya-inc/dataclaw/internal/store"
	"github.com/ekaya-inc/dataclaw/internal/uifs"
	"github.com/ekaya-inc/dataclaw/migrations"
)

func Run(version string) error {
	cfg, err := config.Load(version)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(cfg.DataDir, 0o755); err != nil {
		return err
	}
	secret, err := security.LoadOrCreateSecret(cfg.SecretPath)
	if err != nil {
		return err
	}
	ctx := context.Background()
	store, err := storepkg.Open(ctx, cfg.SQLitePath, migrations.FS)
	if err != nil {
		return err
	}
	defer store.Close()
	ln, port, err := runtime.ListenIncrement(cfg.BindAddr, cfg.Port, 100)
	if err != nil {
		return err
	}
	baseURL := cfg.UIBaseURL(port)
	service := core.New(store, secret, version, func() string { return baseURL })
	defer service.Close()
	api := httpapi.New(service)
	mcpSrv := mcpserver.New(version, service)
	mux := http.NewServeMux()
	api.Register(mux)
	mux.Handle("/mcp", mcpSrv.Handler())
	uiFS, err := uifs.Load()
	if err != nil {
		return fmt.Errorf("load ui: %w", err)
	}
	registerUIRoutes(mux, uiFS)
	server := &http.Server{Handler: logRequests(mux)}
	slog.Info("starting dataclaw", "base_url", baseURL, "mcp_url", baseURL+"/mcp", "sqlite", cfg.SQLitePath, "ui_source", uifs.Source())
	shutdownDone := make(chan struct{})
	go func() {
		sig := make(chan os.Signal, 1)
		signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
		defer signal.Stop(sig)
		<-sig
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdownCtx)
		close(shutdownDone)
	}()
	err = server.Serve(ln)
	if err != nil && err != http.ErrServerClosed {
		return err
	}
	select {
	case <-shutdownDone:
	default:
	}
	return nil
}

func registerUIRoutes(mux *http.ServeMux, uiFS fs.FS) {
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
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
		indexHTML, err := fs.ReadFile(uiFS, "index.html")
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err != nil {
			http.Error(w, "index.html missing", http.StatusInternalServerError)
			return
		}
		_, _ = w.Write(indexHTML)
	})
}

func logRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		started := time.Now()
		next.ServeHTTP(w, r)
		slog.Debug("request", "method", r.Method, "path", r.URL.Path, "duration_ms", time.Since(started).Milliseconds())
	})
}
