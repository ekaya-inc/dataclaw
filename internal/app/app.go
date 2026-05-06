package app

import (
	"context"
	"fmt"
	"io/fs"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	dsadapter "github.com/ekaya-inc/dataclaw/internal/adapters/datasource"
	_ "github.com/ekaya-inc/dataclaw/internal/adapters/datasource/mssql"
	_ "github.com/ekaya-inc/dataclaw/internal/adapters/datasource/postgres"
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
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: cfg.LogLevel})))
	for _, warning := range cfg.Warnings {
		slog.Warn(warning)
	}
	if cfg.Admin.PasswordDefaulted {
		slog.Warn("admin password default is active; set DATACLAW_ADMIN_PASSWORD or admin.password in config.json")
	}
	if cfg.Admin.TLS && !cfg.Admin.ServesTLS() {
		slog.Info("admin listener advertises https for reverse proxy; serving plain HTTP because cert/key files are not configured")
	}
	if cfg.MCP.TLS && !cfg.MCP.ServesTLS() {
		slog.Info("mcp listener advertises https for reverse proxy; serving plain HTTP because cert/key files are not configured")
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
	adminLn, mcpLn, err := runtime.ListenPair(
		runtime.ListenerRequest{Name: "admin", BindAddr: cfg.Admin.BindAddr, Port: cfg.Admin.Port},
		runtime.ListenerRequest{Name: "mcp", BindAddr: cfg.MCP.BindAddr, Port: cfg.MCP.Port},
		100,
	)
	if err != nil {
		return err
	}
	adminBaseURL := cfg.AdminBaseURL(adminLn.Port)
	mcpBaseURL := cfg.MCPBaseURL(mcpLn.Port)
	service := core.NewWithBaseURLs(
		store,
		secret,
		version,
		func() string { return adminBaseURL },
		func() string { return mcpBaseURL },
		dsadapter.NewFactory(dsadapter.DefaultRegistry()),
	)
	defer service.Close()
	api := httpapi.New(service)
	mcpSrv := mcpserver.New(version, service)
	adminAuth, err := NewAdminAuth(AdminAuthOptions{
		Password:       cfg.Admin.Password,
		Secret:         secret,
		AdminBaseURL:   adminBaseURL,
		SessionTTL:     cfg.Admin.SessionTTL,
		LongSessionTTL: cfg.Admin.SessionLongTTL,
	})
	if err != nil {
		_ = adminLn.Listener.Close()
		_ = mcpLn.Listener.Close()
		return err
	}

	uiFS, err := uifs.Load()
	if err != nil {
		_ = adminLn.Listener.Close()
		_ = mcpLn.Listener.Close()
		return fmt.Errorf("load ui: %w", err)
	}
	adminMux := BuildAdminMux(api, adminAuth, uiFS)
	mcpMux := BuildMCPMux(mcpSrv)
	adminServer := &http.Server{Handler: logRequests(adminMux)}
	mcpServer := &http.Server{Handler: logRequests(mcpMux)}
	errCh := make(chan error, 2)
	slog.Info(
		"starting dataclaw",
		"admin_base_url", adminBaseURL,
		"mcp_url", mcpBaseURL+"/mcp",
		"sqlite", cfg.SQLitePath,
		"ui_source", uifs.Source(),
	)
	slog.Info("dataclaw", "version", version, "open", adminBaseURL)
	shutdownDone := make(chan struct{})
	go func() {
		sig := make(chan os.Signal, 1)
		signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
		defer signal.Stop(sig)
		<-sig
		shutdownServers(adminServer, mcpServer)
		close(shutdownDone)
	}()
	go serveHTTPServer("admin", adminServer, adminLn.Listener, cfg.Admin.ListenerConfig, errCh)
	go serveHTTPServer("mcp", mcpServer, mcpLn.Listener, cfg.MCP, errCh)

	select {
	case err := <-errCh:
		shutdownServers(adminServer, mcpServer)
		if err != nil {
			return err
		}
		return nil
	case <-shutdownDone:
		return nil
	}
}

func serveHTTPServer(name string, server *http.Server, ln net.Listener, listenerCfg config.ListenerConfig, errCh chan<- error) {
	var err error
	if listenerCfg.ServesTLS() {
		err = server.ServeTLS(ln, listenerCfg.TLSCertFile, listenerCfg.TLSKeyFile)
	} else {
		err = server.Serve(ln)
	}
	if err != nil && err != http.ErrServerClosed {
		errCh <- fmt.Errorf("%s server: %w", name, err)
		return
	}
	errCh <- nil
}

func shutdownServers(servers ...*http.Server) {
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	for _, server := range servers {
		if server != nil {
			_ = server.Shutdown(shutdownCtx)
		}
	}
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
