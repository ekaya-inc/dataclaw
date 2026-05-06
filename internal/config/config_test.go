package config

import (
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestNormalizeBindAddrPreservesExplicitExposure(t *testing.T) {
	cases := map[string]string{
		"":             DefaultBindAddr,
		"127.0.0.1":    "127.0.0.1",
		"localhost":    DefaultBindAddr,
		"0.0.0.0":      "0.0.0.0",
		"192.168.1.10": "192.168.1.10",
	}
	for input, want := range cases {
		if got := normalizeBindAddr(input); got != want {
			t.Fatalf("normalizeBindAddr(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestParseLogLevel(t *testing.T) {
	cases := []struct {
		in   string
		want slog.Level
	}{
		{"", DefaultLogLevel},
		{"debug", slog.LevelDebug},
		{"DEBUG", slog.LevelDebug},
		{" info ", slog.LevelInfo},
		{"warn", slog.LevelWarn},
		{"warning", slog.LevelWarn},
		{"error", slog.LevelError},
	}
	for _, tc := range cases {
		got, err := parseLogLevel(tc.in)
		if err != nil {
			t.Fatalf("parseLogLevel(%q) returned error: %v", tc.in, err)
		}
		if got != tc.want {
			t.Fatalf("parseLogLevel(%q) = %v, want %v", tc.in, got, tc.want)
		}
	}
	if _, err := parseLogLevel("chatty"); err == nil {
		t.Fatal("parseLogLevel(\"chatty\") returned no error")
	}
}

func TestLoadUsesDefaults(t *testing.T) {
	home := t.TempDir()
	setLoadEnv(t, map[string]string{"HOME": home})

	cfg, err := Load("1.2.3")
	if err != nil {
		t.Fatalf("Load(): %v", err)
	}

	wantDataDir := filepath.Join(home, ".dataclaw")
	if cfg.BindAddr != DefaultAdminBindAddr || cfg.Admin.BindAddr != DefaultAdminBindAddr {
		t.Fatalf("admin bind = legacy %q nested %q, want %q", cfg.BindAddr, cfg.Admin.BindAddr, DefaultAdminBindAddr)
	}
	if cfg.Port != DefaultAdminPort || cfg.Admin.Port != DefaultAdminPort {
		t.Fatalf("admin port = legacy %d nested %d, want %d", cfg.Port, cfg.Admin.Port, DefaultAdminPort)
	}
	if cfg.MCP.BindAddr != DefaultMCPBindAddr || cfg.MCP.Port != DefaultMCPPort {
		t.Fatalf("mcp listener = %s:%d, want %s:%d", cfg.MCP.BindAddr, cfg.MCP.Port, DefaultMCPBindAddr, DefaultMCPPort)
	}
	if cfg.Admin.Password != DefaultAdminPassword || !cfg.Admin.PasswordDefaulted {
		t.Fatalf("admin password default state = %q/%v", cfg.Admin.Password, cfg.Admin.PasswordDefaulted)
	}
	if cfg.Admin.SessionTTL != DefaultAdminSessionTTL || cfg.Admin.SessionLongTTL != DefaultAdminSessionLongTTL {
		t.Fatalf("session TTLs = %s/%s", cfg.Admin.SessionTTL, cfg.Admin.SessionLongTTL)
	}
	if cfg.DataDir != wantDataDir {
		t.Fatalf("DataDir = %q, want %q", cfg.DataDir, wantDataDir)
	}
	if cfg.SQLitePath != filepath.Join(wantDataDir, "dataclaw.sqlite") {
		t.Fatalf("SQLitePath = %q", cfg.SQLitePath)
	}
	if cfg.SecretPath != filepath.Join(wantDataDir, "secret.key") {
		t.Fatalf("SecretPath = %q", cfg.SecretPath)
	}
	if cfg.Version != "1.2.3" {
		t.Fatalf("Version = %q, want %q", cfg.Version, "1.2.3")
	}
	if cfg.LogLevel != DefaultLogLevel {
		t.Fatalf("LogLevel = %v, want %v", cfg.LogLevel, DefaultLogLevel)
	}
}

func TestLoadUsesJSONConfigAndEnvOverrides(t *testing.T) {
	dataDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dataDir, "config.json"), []byte(`{
		"config_version": 1,
		"admin": {
			"bind_addr": "0.0.0.0",
			"port": 19001,
			"advertised_host": "admin.example.com",
			"password": "from-file",
			"session_ttl": "2h",
			"session_long_ttl": "48h"
		},
		"mcp": {
			"bind_addr": "127.0.0.1",
			"port": 19002,
			"advertised_host": "mcp.example.com"
		},
		"log_level": "warn"
	}`), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	setLoadEnv(t, map[string]string{
		"HOME":                             t.TempDir(),
		"DATACLAW_DATA_DIR":                dataDir,
		"DATACLAW_ADMIN_PORT":              "19111",
		"DATACLAW_ADMIN_PASSWORD":          "from-env",
		"DATACLAW_MCP_ADVERTISED_BASE_URL": "https://mcp.public.example.com/",
		"DATACLAW_ADMIN_SESSION_LONG_TTL":  "72h",
		"DATACLAW_ADMIN_TLS":               "true",
		"DATACLAW_MCP_TLS_CERT_FILE":       filepath.Join(dataDir, "mcp.crt"),
		"DATACLAW_MCP_TLS_KEY_FILE":        filepath.Join(dataDir, "mcp.key"),
	})

	cfg, err := Load("dev")
	if err != nil {
		t.Fatalf("Load(): %v", err)
	}
	if cfg.Admin.BindAddr != "0.0.0.0" || cfg.Admin.Port != 19111 {
		t.Fatalf("admin listener = %s:%d", cfg.Admin.BindAddr, cfg.Admin.Port)
	}
	if cfg.Admin.Password != "from-env" || cfg.Admin.PasswordDefaulted {
		t.Fatalf("admin password/defaulted = %q/%v", cfg.Admin.Password, cfg.Admin.PasswordDefaulted)
	}
	if cfg.Admin.SessionTTL != 2*time.Hour || cfg.Admin.SessionLongTTL != 72*time.Hour {
		t.Fatalf("session TTLs = %s/%s", cfg.Admin.SessionTTL, cfg.Admin.SessionLongTTL)
	}
	if cfg.MCP.Port != 19002 || cfg.MCP.AdvertisedBaseURL != "https://mcp.public.example.com" || !cfg.MCP.ServesTLS() {
		t.Fatalf("mcp config = %#v", cfg.MCP)
	}
	if got := cfg.AdminBaseURL(19111); got != "https://admin.example.com:19111" {
		t.Fatalf("AdminBaseURL() = %q", got)
	}
	if got := cfg.MCPBaseURL(19002); got != "https://mcp.public.example.com" {
		t.Fatalf("MCPBaseURL() = %q", got)
	}
	if cfg.LogLevel != slog.LevelWarn {
		t.Fatalf("LogLevel = %v, want %v", cfg.LogLevel, slog.LevelWarn)
	}
}

func TestLoadUsesLegacyEnvAliasesForAdminOnly(t *testing.T) {
	customDir := filepath.Join(t.TempDir(), "custom")
	setLoadEnv(t, map[string]string{
		"HOME":                 t.TempDir(),
		"DATACLAW_BIND_ADDR":   " 0.0.0.0 ",
		"DATACLAW_PORT":        " 19001 ",
		"DATACLAW_DATA_DIR":    " " + customDir + " ",
		"DATACLAW_DB_PATH":     " " + filepath.Join(customDir, "db.sqlite") + " ",
		"DATACLAW_SECRET_PATH": " " + filepath.Join(customDir, "secret.key") + " ",
		"DATACLAW_LOG_LEVEL":   " WARN ",
	})

	cfg, err := Load("dev")
	if err != nil {
		t.Fatalf("Load(): %v", err)
	}

	if cfg.BindAddr != "0.0.0.0" || cfg.Admin.BindAddr != "0.0.0.0" {
		t.Fatalf("admin bind = legacy %q nested %q, want explicit 0.0.0.0", cfg.BindAddr, cfg.Admin.BindAddr)
	}
	if cfg.MCP.BindAddr != DefaultMCPBindAddr || cfg.MCP.Port != DefaultMCPPort {
		t.Fatalf("legacy aliases should not affect MCP, got %#v", cfg.MCP)
	}
	if cfg.Port != 19001 || cfg.Admin.Port != 19001 {
		t.Fatalf("admin port = legacy %d nested %d", cfg.Port, cfg.Admin.Port)
	}
	if len(cfg.Warnings) != 2 || !strings.Contains(strings.Join(cfg.Warnings, "\n"), "DATACLAW_BIND_ADDR is deprecated") {
		t.Fatalf("warnings = %#v", cfg.Warnings)
	}
	if cfg.DataDir != customDir {
		t.Fatalf("DataDir = %q, want %q", cfg.DataDir, customDir)
	}
	if cfg.SQLitePath != filepath.Join(customDir, "db.sqlite") {
		t.Fatalf("SQLitePath = %q", cfg.SQLitePath)
	}
	if cfg.SecretPath != filepath.Join(customDir, "secret.key") {
		t.Fatalf("SecretPath = %q", cfg.SecretPath)
	}
	if cfg.LogLevel != slog.LevelWarn {
		t.Fatalf("LogLevel = %v, want %v", cfg.LogLevel, slog.LevelWarn)
	}
}

func TestLoadRejectsInvalidPort(t *testing.T) {
	setLoadEnv(t, map[string]string{
		"HOME":               t.TempDir(),
		"DATACLAW_PORT":      "not-a-number",
		"DATACLAW_LOG_LEVEL": "",
	})

	_, err := Load("test")
	if err == nil {
		t.Fatal("Load() returned nil error for invalid DATACLAW_PORT")
	}
	if !strings.Contains(err.Error(), "parse DATACLAW_PORT") {
		t.Fatalf("Load() error = %v, want parse DATACLAW_PORT error", err)
	}
}

func TestLoadRejectsInvalidLogLevel(t *testing.T) {
	setLoadEnv(t, map[string]string{
		"HOME":               t.TempDir(),
		"DATACLAW_PORT":      "",
		"DATACLAW_LOG_LEVEL": "chatty",
	})

	_, err := Load("test")
	if err == nil {
		t.Fatal("Load() returned nil error for invalid DATACLAW_LOG_LEVEL")
	}
	if !strings.Contains(err.Error(), "invalid DATACLAW_LOG_LEVEL") {
		t.Fatalf("Load() error = %v, want invalid DATACLAW_LOG_LEVEL error", err)
	}
}

func TestLoadRejectsInvalidSecurityConfig(t *testing.T) {
	cases := []struct {
		name string
		env  map[string]string
		want string
	}{
		{"empty admin password", map[string]string{"DATACLAW_ADMIN_PASSWORD": ""}, "DATACLAW_ADMIN_PASSWORD must not be empty"},
		{"invalid admin base url", map[string]string{"DATACLAW_ADMIN_ADVERTISED_BASE_URL": "ftp://admin.example.com"}, "admin.advertised_base_url must use http or https"},
		{"advertised host with port", map[string]string{"DATACLAW_MCP_ADVERTISED_HOST": "mcp.example.com:18791"}, "mcp.advertised_host must not include a port"},
		{"partial tls pair", map[string]string{"DATACLAW_ADMIN_TLS_CERT_FILE": "cert.pem"}, "admin TLS cert/key must be configured together"},
		{"too long session", map[string]string{"DATACLAW_ADMIN_SESSION_LONG_TTL": "2161h"}, "admin.session_long_ttl must be positive"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			values := map[string]string{"HOME": t.TempDir()}
			for key, value := range tc.env {
				values[key] = value
			}
			setLoadEnv(t, values)
			_, err := Load("test")
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("Load() error = %v, want substring %q", err, tc.want)
			}
		})
	}
}

func TestConfigBaseURLHelpers(t *testing.T) {
	cfg := &Config{Admin: AdminConfig{ListenerConfig: ListenerConfig{BindAddr: "0.0.0.0"}}, MCP: ListenerConfig{BindAddr: "::"}}

	if got := cfg.BaseURL(18790); got != "http://127.0.0.1:18790" {
		t.Fatalf("BaseURL() = %q, want loopback-safe advertised URL", got)
	}
	if got := cfg.UIBaseURL(18790); got != "http://127.0.0.1:18790" {
		t.Fatalf("UIBaseURL() = %q, want loopback-safe advertised URL", got)
	}
	if got := cfg.MCPBaseURL(18791); got != "http://127.0.0.1:18791" {
		t.Fatalf("MCPBaseURL() = %q, want loopback-safe advertised URL", got)
	}

	cfg.Admin.BindAddr = "127.0.0.1"
	cfg.Admin.AdvertisedHost = "admin.local"
	cfg.MCP = ListenerConfig{BindAddr: "127.0.0.1", AdvertisedBaseURL: "https://mcp.example.com"}
	if got := cfg.UIBaseURL(18790); got != "http://admin.local:18790" {
		t.Fatalf("UIBaseURL(advertised host) = %q", got)
	}
	if got := cfg.MCPBaseURL(18791); got != "https://mcp.example.com" {
		t.Fatalf("MCPBaseURL(advertised base) = %q", got)
	}
}

func setLoadEnv(t *testing.T, values map[string]string) {
	t.Helper()
	for _, key := range []string{
		"HOME",
		"DATACLAW_CONFIG_PATH",
		"DATACLAW_BIND_ADDR",
		"DATACLAW_PORT",
		"DATACLAW_DATA_DIR",
		"DATACLAW_DB_PATH",
		"DATACLAW_SECRET_PATH",
		"DATACLAW_LOG_LEVEL",
		"DATACLAW_ADMIN_BIND_ADDR",
		"DATACLAW_ADMIN_PORT",
		"DATACLAW_ADMIN_ADVERTISED_HOST",
		"DATACLAW_ADMIN_ADVERTISED_BASE_URL",
		"DATACLAW_ADMIN_TLS",
		"DATACLAW_ADMIN_TLS_CERT_FILE",
		"DATACLAW_ADMIN_TLS_KEY_FILE",
		"DATACLAW_ADMIN_PASSWORD",
		"DATACLAW_ADMIN_SESSION_TTL",
		"DATACLAW_ADMIN_SESSION_LONG_TTL",
		"DATACLAW_MCP_BIND_ADDR",
		"DATACLAW_MCP_PORT",
		"DATACLAW_MCP_ADVERTISED_HOST",
		"DATACLAW_MCP_ADVERTISED_BASE_URL",
		"DATACLAW_MCP_TLS",
		"DATACLAW_MCP_TLS_CERT_FILE",
		"DATACLAW_MCP_TLS_KEY_FILE",
	} {
		old, hadOld := os.LookupEnv(key)
		if err := os.Unsetenv(key); err != nil {
			t.Fatalf("unset %s: %v", key, err)
		}
		if value, ok := values[key]; ok {
			if err := os.Setenv(key, value); err != nil {
				t.Fatalf("set %s: %v", key, err)
			}
		}
		t.Cleanup(func() {
			if hadOld {
				_ = os.Setenv(key, old)
			} else {
				_ = os.Unsetenv(key)
			}
		})
	}
}
